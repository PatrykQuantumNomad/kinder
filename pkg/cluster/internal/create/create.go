/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package create provides cluster creation logic and coordinates creation actions.
package create

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"al.essio.dev/pkg/shellescape"
	"golang.org/x/sync/errgroup"

	"sigs.k8s.io/kind/pkg/cluster/internal/delete"
	"sigs.k8s.io/kind/pkg/cluster/internal/providers"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/apis/config/encoding"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/doctor"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	configaction "sigs.k8s.io/kind/pkg/cluster/internal/create/actions/config"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installcertmanager"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installcni"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installnvidiagpu"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installcorednstuning"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installdashboard"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installenvoygw"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installlocalregistry"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installmetallb"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installmetricsserver"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installstorage"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/kubeadminit"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/kubeadmjoin"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/loadbalancer"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/waitforready"
	"sigs.k8s.io/kind/pkg/cluster/internal/kubeconfig"
)

const (
	// Typical host name max limit is 64 characters (https://linux.die.net/man/2/sethostname)
	// We append -control-plane (14 characters) to the cluster name on the control plane container
	clusterNameMax = 50
)

// ClusterOptions holds cluster creation options
type ClusterOptions struct {
	Config       *config.Cluster
	NameOverride string // overrides config.Name
	// NodeImage overrides the nodes' images in Config if non-zero
	NodeImage      string
	Retain         bool
	WaitForReady   time.Duration
	KubeconfigPath string
	// see https://github.com/kubernetes-sigs/kind/issues/324
	StopBeforeSettingUpKubernetes bool // if false kind should setup kubernetes after creating nodes
	// Options to control output
	DisplayUsage      bool
	DisplaySalutation bool
	AirGapped         bool
}

// addonResult captures the outcome of running a single addon action.
type addonResult struct {
	name     string
	enabled  bool
	err      error
	duration time.Duration // install duration (zero for disabled addons)
}

// AddonEntry pairs an addon's display name, enabled flag, and action for
// the registry-driven installation loop.
type AddonEntry struct {
	Name    string
	Enabled bool
	Action  actions.Action
}

// runAddonTimed executes a single addon action and returns the result with timing.
// Disabled addons are skipped immediately (zero duration).
func runAddonTimed(actionsCtx *actions.ActionContext, name string, enabled bool, a actions.Action) addonResult {
	if !enabled {
		actionsCtx.Logger.V(0).Infof(" * Skipping %s (disabled in config)\n", name)
		return addonResult{name: name, enabled: false}
	}
	start := time.Now()
	err := a.Execute(actionsCtx)
	dur := time.Since(start)
	if err != nil {
		actionsCtx.Logger.Warnf("Addon %s failed to install (cluster still usable): %v", name, err)
	}
	return addonResult{name: name, enabled: true, err: err, duration: dur}
}

// parallelActionContext returns a copy of ac suitable for parallel goroutine use.
// It replaces the shared Status with a per-goroutine no-op status to avoid
// racing on Status.status (which is not concurrent-safe). The nodesOnce
// cache (sync.OnceValues) is shared safely across goroutines via the shared Provider.
func parallelActionContext(ac *actions.ActionContext) *actions.ActionContext {
	return actions.NewActionContext(
		ac.Context,
		ac.Logger,
		cli.StatusForLogger(log.NoopLogger{}),
		ac.Provider,
		ac.Config,
	)
}

// Cluster creates a cluster
func Cluster(logger log.Logger, p providers.Provider, opts *ClusterOptions) error {
	// validate provider first
	if err := validateProvider(p); err != nil {
		return err
	}

	// default / process options (namely config)
	if err := fixupOptions(opts); err != nil {
		return err
	}

	// Check if the cluster name already exists
	if err := alreadyExists(p, opts.Config.Name); err != nil {
		return err
	}

	// warn if cluster name might typically be too long
	if len(opts.Config.Name) > clusterNameMax {
		logger.Warnf("cluster name %q is probably too long, this might not work properly on some systems", opts.Config.Name)
	}

	// then validate
	if err := opts.Config.Validate(); err != nil {
		return err
	}

	// setup a status object to show progress to the user
	status := cli.StatusForLogger(logger)

	// we're going to start creating now, tell the user
	logger.V(0).Infof("Creating cluster %q ...\n", opts.Config.Name)

	// Inject containerd config_path for local registry (must be before Provision).
	// containerd certs.d hot-reload requires config_path to be set in config.toml
	// at node creation time — it cannot be injected post-provisioning.
	if opts.Config.Addons.LocalRegistry {
		opts.Config.ContainerdConfigPatches = append(
			opts.Config.ContainerdConfigPatches,
			`[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "/etc/containerd/certs.d"
`,
		)
	}

	// Apply safe mitigations before provisioning.
	// Tier-1 only: env vars, cluster config adjustments. Never calls sudo.
	// Errors are informational -- log and continue to provisioning.
	if errs := doctor.ApplySafeMitigations(logger); len(errs) > 0 {
		for _, err := range errs {
			logger.Warnf("Mitigation warning: %v", err)
		}
	}

	// Create node containers implementing defined config Nodes
	if err := p.Provision(status, opts.Config); err != nil {
		// In case of errors nodes are deleted (except if retain is explicitly set)
		if !opts.Retain {
			_ = delete.Cluster(logger, p, opts.Config.Name, opts.KubeconfigPath)
		}
		return err
	}

	// TODO(bentheelder): make this controllable from the command line?
	actionsToRun := []actions.Action{
		loadbalancer.NewAction(), // setup external loadbalancer
		configaction.NewAction(), // setup kubeadm config
	}
	if !opts.StopBeforeSettingUpKubernetes {
		actionsToRun = append(actionsToRun,
			kubeadminit.NewAction(opts.Config), // run kubeadm init
		)
		// this step might be skipped, but is next after init
		if !opts.Config.Networking.DisableDefaultCNI {
			actionsToRun = append(actionsToRun,
				installcni.NewAction(), // install CNI
			)
		}
		// add remaining steps
		actionsToRun = append(actionsToRun,
			installstorage.NewAction(),                // install StorageClass
			kubeadmjoin.NewAction(),                   // run kubeadm join
			waitforready.NewAction(opts.WaitForReady), // wait for cluster readiness
		)
	}

	// run all actions
	actionsContext := actions.NewActionContext(context.Background(), logger, status, p, opts.Config)
	for _, action := range actionsToRun {
		if err := action.Execute(actionsContext); err != nil {
			if !opts.Retain {
				_ = delete.Cluster(logger, p, opts.Config.Name, opts.KubeconfigPath)
			}
			return err
		}
	}

	// skip the rest if we're not setting up kubernetes
	if opts.StopBeforeSettingUpKubernetes {
		return nil
	}

	// try exporting kubeconfig with backoff for locking failures
	// TODO: factor out into a public errors API w/ backoff handling?
	// for now this is easier than coming up with a good API
	var err error
	for _, b := range []time.Duration{0, time.Millisecond, time.Millisecond * 50, time.Millisecond * 100} {
		time.Sleep(b)
		if err = kubeconfig.Export(p, opts.Config.Name, opts.KubeconfigPath, true); err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	// --- Addon actions (warn-and-continue) ---

	// Dependency conflict check (per user decision: warn and continue)
	if !opts.Config.Addons.MetalLB && opts.Config.Addons.EnvoyGateway {
		logger.Warn("MetalLB is disabled but Envoy Gateway is enabled. Envoy Gateway proxy services will not receive LoadBalancer IPs.")
	}

	// Addon dependency DAG:
	// Wave 1 (parallel, SetLimit(3)): Local Registry, MetalLB, Metrics Server,
	//   CoreDNS Tuning, Dashboard, Cert Manager
	//   All Wave 1 addons are independent -- no inter-addon dependencies.
	// Wave 2 (sequential, after Wave 1): Envoy Gateway
	//   EnvoyGateway depends on MetalLB: MetalLB must assign LoadBalancer IPs
	//   before Envoy Gateway proxy services receive external IPs.
	// Wave boundary is explicit: errgroup.Wait() separates Wave 1 from Wave 2.

	wave1 := []AddonEntry{
		{"Local Registry", opts.Config.Addons.LocalRegistry, installlocalregistry.NewAction()},
		{"MetalLB", opts.Config.Addons.MetalLB, installmetallb.NewAction()},
		{"Metrics Server", opts.Config.Addons.MetricsServer, installmetricsserver.NewAction()},
		{"CoreDNS Tuning", opts.Config.Addons.CoreDNSTuning, installcorednstuning.NewAction()},
		{"Dashboard", opts.Config.Addons.Dashboard, installdashboard.NewAction()},
		{"Cert Manager", opts.Config.Addons.CertManager, installcertmanager.NewAction()},
		{"NVIDIA GPU", opts.Config.Addons.NvidiaGPU, installnvidiagpu.NewAction()},
	}

	wave2 := []AddonEntry{
		{"Envoy Gateway", opts.Config.Addons.EnvoyGateway, installenvoygw.NewAction()},
	}

	// Pre-allocate results for deterministic summary ordering.
	wave1Results := make([]addonResult, len(wave1))
	addonResults := make([]addonResult, 0, len(wave1)+len(wave2))

	// Wave 1: run independent addons in parallel (up to 3 concurrent).
	g, _ := errgroup.WithContext(actionsContext.Context)
	g.SetLimit(3)

	var mu sync.Mutex
	for i, addon := range wave1 {
		i, addon := i, addon
		g.Go(func() error {
			pCtx := parallelActionContext(actionsContext)
			res := runAddonTimed(pCtx, addon.Name, addon.Enabled, addon.Action)
			mu.Lock()
			wave1Results[i] = res
			mu.Unlock()
			return nil // warn-and-continue: never propagate addon error to errgroup
		})
	}
	if err := g.Wait(); err != nil {
		if !opts.Retain {
			_ = delete.Cluster(logger, p, opts.Config.Name, opts.KubeconfigPath)
		}
		return err
	}
	addonResults = append(addonResults, wave1Results...)

	// Wave 2: Envoy Gateway depends on MetalLB (Wave 1 must complete first).
	for _, addon := range wave2 {
		res := runAddonTimed(actionsContext, addon.Name, addon.Enabled, addon.Action)
		addonResults = append(addonResults, res)
	}

	// Platform warning for MetalLB (FOUND-05)
	if opts.Config.Addons.MetalLB {
		logMetalLBPlatformWarning(logger)
	}

	// Addon summary
	logAddonSummary(logger, addonResults)

	// optionally display usage
	if opts.DisplayUsage {
		logUsage(logger, opts.Config.Name, opts.KubeconfigPath)
	}
	// optionally give the user a friendly salutation
	if opts.DisplaySalutation {
		logger.V(0).Info("")
		logSalutation(logger)
	}
	return nil
}

// alreadyExists returns an error if the cluster name already exists
// or if we had an error checking
func alreadyExists(p providers.Provider, name string) error {
	n, err := p.ListNodes(name)
	if err != nil {
		return err
	}
	if len(n) != 0 {
		return errors.Errorf("node(s) already exist for a cluster with the name %q", name)
	}
	return nil
}

func logUsage(logger log.Logger, name, explicitKubeconfigPath string) {
	// construct a sample command for interacting with the cluster
	kctx := kubeconfig.ContextForCluster(name)
	sampleCommand := fmt.Sprintf("kubectl cluster-info --context %s", kctx)
	if explicitKubeconfigPath != "" {
		// explicit path, include this
		sampleCommand += " --kubeconfig " + shellescape.Quote(explicitKubeconfigPath)
	}
	logger.V(0).Infof(`Set kubectl context to "%s"`, kctx)
	logger.V(0).Infof("You can now use your cluster with:\n\n" + sampleCommand)
}

func logSalutation(logger log.Logger) {
	salutations := []string{
		"Have a nice day! 👋",
		"Thanks for using kinder! 😊",
		"Not sure what to do next? 😅  Check out https://kind.sigs.k8s.io/docs/user/quick-start/",
		"Have a question, bug, or feature request? Let us know! https://kind.sigs.k8s.io/#community 🙂",
	}
	s := salutations[rand.Intn(len(salutations))]
	logger.V(0).Info(s)
}

func fixupOptions(opts *ClusterOptions) error {
	// do post processing for options
	// first ensure we at least have a default cluster config
	if opts.Config == nil {
		cfg, err := encoding.Load("")
		if err != nil {
			return err
		}
		opts.Config = cfg
	}

	if opts.NameOverride != "" {
		opts.Config.Name = opts.NameOverride
	}

	// if NodeImage was set, override the image on nodes that do not have an explicit image
	if opts.NodeImage != "" {
		// Apply image override only to nodes without an explicit per-node image.
		// Nodes with ExplicitImage=true had their image set in the config file and
		// should not be overridden by the --image flag.
		// TODO(fabrizio pandini): this should be reconsidered when implementing
		//     https://github.com/kubernetes-sigs/kind/issues/133
		for i := range opts.Config.Nodes {
			if !opts.Config.Nodes[i].ExplicitImage {
				opts.Config.Nodes[i].Image = opts.NodeImage
			}
		}
	}

	if opts.AirGapped {
		opts.Config.AirGapped = true
	}

	return nil
}

func validateProvider(p providers.Provider) error {
	info, err := p.Info()
	if err != nil {
		return err
	}
	if info.Rootless {
		if !info.Cgroup2 {
			return errors.New("running kind with rootless provider requires cgroup v2, see https://kind.sigs.k8s.io/docs/user/rootless/")
		}
		if !info.SupportsMemoryLimit || !info.SupportsPidsLimit || !info.SupportsCPUShares {
			return errors.New("running kind with rootless provider requires setting systemd property \"Delegate=yes\", see https://kind.sigs.k8s.io/docs/user/rootless/")
		}
	}
	return nil
}

// logMetalLBPlatformWarning prints a warning on macOS/Windows that MetalLB
// LoadBalancer IPs are not reachable from the host.
func logMetalLBPlatformWarning(logger log.Logger) {
	switch runtime.GOOS {
	case "darwin", "windows":
		logger.Warnf(
			"On %s, MetalLB LoadBalancer IPs are not directly reachable from the host.\n"+
				"   Use kubectl port-forward to access LoadBalancer services:\n"+
				"   kubectl port-forward svc/<service-name> <local-port>:<service-port>",
			runtime.GOOS,
		)
	}
}

// logAddonSummary prints a scannable summary of addon installation results.
func logAddonSummary(logger log.Logger, results []addonResult) {
	logger.V(0).Info("")
	logger.V(0).Info("Addons:")
	for _, r := range results {
		switch {
		case !r.enabled:
			logger.V(0).Infof(" * %-20s skipped (disabled)", r.name)
		case r.err != nil:
			logger.V(0).Infof(" * %-20s FAILED: %v (%.1fs)", r.name, r.err, r.duration.Seconds())
		default:
			logger.V(0).Infof(" * %-20s installed (%.1fs)", r.name, r.duration.Seconds())
		}
	}
}
