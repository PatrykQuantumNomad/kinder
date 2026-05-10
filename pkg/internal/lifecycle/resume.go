/*
Copyright 2026 The Kubernetes Authors.

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

package lifecycle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/doctor"
	"sigs.k8s.io/kind/pkg/internal/version"
	"sigs.k8s.io/kind/pkg/log"
)

// nodeFetcher and NodeResult are declared in pause.go (plan 47-02). Resume
// reuses both so PauseResult and ResumeResult share the per-node JSON shape.

// ResumeOptions configures a single Resume invocation.
type ResumeOptions struct {
	// ClusterName is the kinder cluster to resume (required).
	ClusterName string
	// StartTimeout is the per-container graceful start timeout. Defaults to 30s.
	StartTimeout time.Duration
	// WaitTimeout is the readiness gate timeout. Defaults to 5m.
	WaitTimeout time.Duration
	// Logger receives per-node lines and warnings. Required to be non-nil.
	Logger log.Logger
	// Provider is the cluster provider used to discover nodes. Required.
	Provider nodeFetcher
	// Context allows callers to cancel the in-flight Resume. Defaults to
	// context.Background().
	Context context.Context
}

// ResumeResult is the structured outcome of a Resume call. It is the JSON shape
// emitted by `kinder resume --json`.
type ResumeResult struct {
	Cluster          string       `json:"cluster"`
	State            string       `json:"state"`
	AlreadyRunning   bool         `json:"alreadyRunning,omitempty"`
	Nodes            []NodeResult `json:"nodes"`
	ReadinessSeconds float64      `json:"readinessSeconds"`
	Duration         float64      `json:"durationSeconds"`
}

// ReadinessProber is the function signature used by Resume to wait for the
// Kubernetes API to report all nodes Ready. The production implementation is
// realReadinessProbe; tests swap in a fake via the package-level
// defaultReadinessProber to avoid spinning a real cluster.
type ReadinessProber func(ctx context.Context, bootstrap nodes.Node, deadline time.Time) error

// defaultReadinessProber is the production readiness probe; tests swap it via
// withReadinessProber.
var defaultReadinessProber ReadinessProber = WaitForNodesReady

// resumeBinaryName allows tests to swap the runtime auto-detect (docker/
// podman/nerdctl) without touching $PATH. Production callers reach
// ProviderBinaryName directly.
var resumeBinaryName = ProviderBinaryName

// ResumeReadinessHook is invoked AFTER all CP containers have started but
// BEFORE worker containers start, ONLY on HA clusters (≥2 control-plane
// nodes). The hook MUST NEVER block resume — warnings flow through opts.Logger
// only, and the hook returns no error. The default impl runs the
// `cluster-resume-readiness` doctor check (LIFE-04, plan 47-04). Tests
// inject a fake to assert invocation order/args. Per CONTEXT.md decision
// "warn and continue".
var ResumeReadinessHook = defaultResumeReadinessHook

// defaultResumeReadinessHook runs the doctor.NewClusterResumeReadinessCheck
// inline and forwards its results to opts.Logger. Warnings appear at V(0),
// skips at V(2), oks at V(1). Errors are intentionally swallowed — the hook
// never affects Resume's exit code.
func defaultResumeReadinessHook(_ string, _ string, logger log.Logger) {
	check := doctor.NewClusterResumeReadinessCheck()
	for _, r := range check.Run() {
		switch r.Status {
		case "warn", "fail":
			// "fail" is documented as never-emitted by this check, but log it
			// the same way as warn just in case future code paths add it.
			logger.Warnf("⚠ %s: %s — %s", r.Name, r.Message, r.Reason)
		case "skip":
			logger.V(2).Infof(" • %s: %s (skipped)", r.Name, r.Message)
		case "ok":
			logger.V(1).Infof(" ✓ %s: %s", r.Name, r.Message)
		}
	}
}

// readResumeStrategy inspects the bootstrap CP's labels to read the
// resume-strategy label value (ResumeStrategyLabel).
// Returns the label value, or "" (empty string) for legacy clusters where the
// label is absent (no-label = legacy = cert-regen-forever per CONTEXT.md).
func readResumeStrategy(binaryName string, cpNodes []nodes.Node) string {
	if len(cpNodes) == 0 {
		return ""
	}
	bootstrap := cpNodes[0].String()
	lines, err := exec.OutputLines(defaultCmder(binaryName,
		"inspect",
		"--format", `{{index .Config.Labels "`+ResumeStrategyLabel+`"}}`,
		bootstrap,
	))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines, ""))
}

// applyPinnedIPsBeforeCPStart reconnects each stopped CP container to its
// kind network with the pinned IP recorded in /kind/ipam-state.json.
//
// The networkName for each CP is sourced from IPAMState.Network — the value
// Plan 52-02 persisted at create time. This avoids the multi-network
// ambiguity (W4): a CP container attached to multiple networks would make a
// separate docker inspect call ambiguous, while the JSON file unambiguously
// names the kind network the CP must be reconnected to.
//
// W2 Option A: if ReadIPAMState fails for ANY CP (file missing, unreadable,
// or unparseable), Resume HALTS with a structured diagnostic error. There is
// NO soft-skip and NO per-CP fallback to cert-regen mid-resume. The
// cluster-level ip-pinned label is a contract that every CP carries a valid
// /kind/ipam-state.json; its absence or corruption indicates cluster damage.
func applyPinnedIPsBeforeCPStart(binaryName string, cpNodes []nodes.Node, logger log.Logger) error {
	tmpDir := os.TempDir()
	for _, cpNode := range cpNodes {
		container := cpNode.String()

		// W2 Option A: ReadIPAMState failure on ANY CP halts Resume.
		state, err := ReadIPAMState(binaryName, container, tmpDir)
		if err != nil {
			return errors.Errorf(
				"ip-pin resume halted: failed to read /kind/ipam-state.json on %s: %v. Cluster state is undefined — delete and recreate the cluster.",
				container, err,
			)
		}

		networkName := state.Network

		// Disconnect from network (container must be in exited state at this point).
		if discErr := defaultCmder(binaryName, "network", "disconnect", networkName, container).Run(); discErr != nil {
			return errors.Wrapf(discErr,
				"ip-pin resume halted: failed to disconnect %s from network %s. Cluster state is undefined — delete and recreate the cluster.",
				container, networkName,
			)
		}

		// Reconnect with the pinned IP (T-52-03-04: container is stopped → safe).
		if connErr := defaultCmder(binaryName, "network", "connect",
			"--ip", state.IPv4, networkName, container,
		).Run(); connErr != nil {
			return errors.Wrapf(connErr,
				"ip-pin resume halted: failed to reconnect %s with --ip %s on network %s. Cluster state is undefined — delete and recreate the cluster.",
				container, state.IPv4, networkName,
			)
		}

		logger.V(1).Infof(" ✓ pinned IP %s for %s on network %s", state.IPv4, container, networkName)
	}
	return nil
}

// Resume starts every container in a paused kinder cluster in quorum-safe
// order (external-load-balancer → control-plane → workers) and waits for all
// nodes to report Ready via the Kubernetes API. Best-effort: if a single
// container fails to start, Resume continues with the remaining containers
// and returns an aggregated error at the end.
//
// If the cluster is already running, Resume returns immediately with
// ResumeResult.AlreadyRunning=true and skips the readiness probe.
func Resume(opts ResumeOptions) (*ResumeResult, error) {
	if opts.ClusterName == "" {
		return nil, errors.New("Resume: ClusterName is required")
	}
	if opts.Provider == nil {
		return nil, errors.New("Resume: Provider is required")
	}
	if opts.Logger == nil {
		opts.Logger = log.NoopLogger{}
	}
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	if opts.StartTimeout <= 0 {
		opts.StartTimeout = 30 * time.Second
	}
	if opts.WaitTimeout <= 0 {
		opts.WaitTimeout = 5 * time.Minute
	}

	t0 := time.Now()

	allNodes, err := opts.Provider.ListNodes(opts.ClusterName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list nodes for cluster %q", opts.ClusterName)
	}
	if len(allNodes) == 0 {
		return nil, errors.Errorf("no nodes found for cluster %q", opts.ClusterName)
	}

	binaryName := resumeBinaryName()
	if binaryName == "" {
		return nil, errors.New("Resume: no container runtime detected (docker/podman/nerdctl)")
	}

	// Idempotency: if every node is already running, log a warning and exit
	// without touching the cluster.
	if ClusterStatus(binaryName, allNodes) == "Running" {
		opts.Logger.Warnf("cluster %q is already running; no action taken", opts.ClusterName)
		return &ResumeResult{
			Cluster:        opts.ClusterName,
			State:          "resumed",
			AlreadyRunning: true,
			Nodes:          []NodeResult{},
			Duration:       time.Since(t0).Seconds(),
		}, nil
	}

	cp, workers, lb, err := ClassifyNodes(allNodes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to classify nodes")
	}
	if len(cp) == 0 {
		return nil, errors.Errorf("cluster %q has no control-plane nodes", opts.ClusterName)
	}

	// Phase 0: HA resume strategy detection. Single-CP/non-HA: skip entirely
	// (zero overhead per D-locked decision D-zero-overhead-single-CP).
	//
	// strategy is one of:
	//   StrategyIPPinned  ("ip-pinned")  — reconnect with --ip before CP start
	//   StrategyCertRegen ("cert-regen") — reactive wholesale regen after start
	//   ""                               — legacy (absent label); treat as cert-regen
	//
	// readResumeStrategy reads the bootstrap CP's label via docker inspect.
	var strategy string
	if len(cp) >= 2 {
		strategy = readResumeStrategy(binaryName, cp)
		// Defensive: nerdctl cannot pin IPs (RESEARCH PIT-4). If label says
		// ip-pinned but runtime is nerdctl, downgrade to cert-regen.
		if strategy == StrategyIPPinned && filepath.Base(binaryName) == "nerdctl" {
			opts.Logger.Warnf("nerdctl detected; downgrading resume-strategy from ip-pinned to cert-regen")
			strategy = StrategyCertRegen
		}
	}

	// Five-phase start order (reverse of pause, with HA hooks inserted):
	//   Phase 1: LB (if present)
	//   Phase 1.5 (HA + ip-pinned): reconnect each CP with --ip before start
	//   Phase 2: control-plane nodes
	//   Phase 3 (HA only, post-CP / pre-workers): inline cluster-resume-readiness
	//   Phase 4: workers
	//   Phase 4.5 (HA + cert-regen/legacy): reactive wholesale cert-regen
	//
	// Each phase appends to results/startErrs via startNodes(). The inline
	// readiness hook is called only when both:
	//   (a) the cluster is HA (≥2 CPs), AND
	//   (b) every prior start succeeded (len(startErrs)==0)
	// matching CONTEXT.md "warn and continue" + "no point probing a known-
	// incomplete cluster".
	results := make([]NodeResult, 0, len(allNodes))
	startErrs := []error{}

	startNodes := func(group []nodes.Node) {
		for _, node := range group {
			// Honor cancellation between starts.
			select {
			case <-opts.Context.Done():
				startErrs = append(startErrs, opts.Context.Err())
				return
			default:
			}

			role := classifyRole(node)
			nodeStart := time.Now()
			cmd := defaultCmder(binaryName, "start", node.String())
			runErr := cmd.Run()
			nodeDuration := time.Since(nodeStart).Seconds()

			nr := NodeResult{
				Name:     node.String(),
				Role:     role,
				Success:  runErr == nil,
				Duration: nodeDuration,
			}
			if runErr != nil {
				nr.Error = runErr.Error()
				startErrs = append(startErrs, errors.Wrapf(runErr, "failed to start %s", node.String()))
				opts.Logger.V(0).Infof(" • ✗ %s (role=%s, %.1fs): %v", node.String(), role, nodeDuration, runErr)
			} else {
				opts.Logger.V(0).Infof(" • ✓ %s (role=%s, %.1fs)", node.String(), role, nodeDuration)
			}
			results = append(results, nr)
		}
	}

	// Phase 1: LB
	if lb != nil {
		startNodes([]nodes.Node{lb})
	}

	// Phase 1.5: ip-pinned pre-start reconnect.
	// CPs are in `exited` state here — network mutation is legal (T-52-03-04).
	// networkName is sourced per-CP from IPAMState.Network (W4: avoids
	// multi-network ambiguity). ReadIPAMState failure on ANY CP halts Resume
	// (W2 Option A: no soft-skip, no per-CP cert-regen fallback).
	if strategy == StrategyIPPinned {
		if err := applyPinnedIPsBeforeCPStart(binaryName, cp, opts.Logger); err != nil {
			res := &ResumeResult{
				Cluster:  opts.ClusterName,
				State:    "resumed",
				Nodes:    results,
				Duration: time.Since(t0).Seconds(),
			}
			startErrs = append(startErrs, err)
			return res, aggregateErrors(startErrs)
		}
	}

	// Phase 2: control-plane
	startNodes(cp)
	// Phase 3: inline readiness hook (HA only, no prior failures, hook installed)
	if len(cp) >= 2 && len(startErrs) == 0 && ResumeReadinessHook != nil {
		ResumeReadinessHook(binaryName, cp[0].String(), opts.Logger)
	}
	// Phase 4: workers
	startNodes(workers)

	// Phase 4.5: cert-regen / legacy path (REACTIVE — only on drift).
	// Runs AFTER all containers are started (kubeadm needs API server reachable,
	// per RESEARCH PIT-5). Wholesale across all CPs (RESEARCH PIT-6).
	if (strategy == StrategyCertRegen || strategy == "") && len(cp) >= 2 && len(startErrs) == 0 {
		// Reactive: check drift on each CP. If ANY CP drifted, regen all (wholesale).
		anyDrift := false
		for _, c := range cp {
			drifted, _, _, driftErr := IPDriftDetected(binaryName, c.String(), os.TempDir())
			if driftErr != nil {
				// Log warning and treat as drift — better to regen unnecessarily
				// than to leave stale certs in place.
				opts.Logger.Warnf("IPDriftDetected error on %s (treating as drift): %v", c.String(), driftErr)
				drifted = true
			}
			if drifted {
				anyDrift = true
				break
			}
		}
		if anyDrift {
			if regenErr := RegenerateEtcdPeerCertsWholesale(cp, opts.Logger); regenErr != nil {
				res := &ResumeResult{
					Cluster:  opts.ClusterName,
					State:    "resumed",
					Nodes:    results,
					Duration: time.Since(t0).Seconds(),
				}
				return res, errors.Wrap(regenErr, "HA resume cert-regen failed: delete and recreate the cluster")
			}
		} else {
			opts.Logger.V(1).Infof("etcd peer IPs unchanged; cert-regen skipped")
		}
	}

	res := &ResumeResult{
		Cluster: opts.ClusterName,
		State:   "resumed",
		Nodes:   results,
	}

	// If any node failed to start, skip readiness probe (per plan: no point
	// waiting for an incomplete cluster).
	if len(startErrs) > 0 {
		res.Duration = time.Since(t0).Seconds()
		return res, aggregateErrors(startErrs)
	}

	// Readiness gate: poll kubectl inside bootstrap CP until every node reports
	// Ready or the WaitTimeout deadline passes.
	bootstrap := cp[0]
	readyT0 := time.Now()
	probeErr := defaultReadinessProber(opts.Context, bootstrap, readyT0.Add(opts.WaitTimeout))
	res.ReadinessSeconds = time.Since(readyT0).Seconds()
	res.Duration = time.Since(t0).Seconds()

	if probeErr != nil {
		opts.Logger.Warnf("timed out waiting for nodes Ready: %v", probeErr)
		return res, errors.Wrap(probeErr, "timed out waiting for nodes Ready")
	}

	return res, nil
}

// classifyRole returns the node's role string with a safe fallback when the
// underlying nodes.Node.Role() call errors (e.g. the container is mid-
// transition). The fallback "unknown" matches the convention used by
// pkg/cmd/kind/status.
func classifyRole(n nodes.Node) string {
	role, err := n.Role()
	if err != nil || role == "" {
		return "unknown"
	}
	return role
}

// aggregateErrors joins multiple errors into a single error suitable for
// returning to a CLI caller. Returns nil for an empty slice.
func aggregateErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	}
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, e.Error())
	}
	return errors.Errorf("multiple errors during resume: %s", strings.Join(parts, "; "))
}

// WaitForNodesReady polls kubectl inside `bootstrap` until every node in the
// cluster reports Ready or the deadline passes. Returns nil on success, an
// error on timeout or context cancellation.
//
// Unlike pkg/cluster/internal/create/actions/waitforready (which only waits
// for control-plane during create — workers may not exist yet), Resume waits
// for ALL nodes because every container has been started.
//
// Note: the K8s 1.24 selector fallback (control-plane vs master label) is
// preserved here for consistency with create's waitforready, but is unused
// because Resume queries all nodes without a selector. The fallback is kept
// so any future caller that wants to filter by role can copy the same logic.
func WaitForNodesReady(ctx context.Context, bootstrap nodes.Node, deadline time.Time) error {
	rawVersion, err := nodeutils.KubeVersion(bootstrap)
	if err != nil {
		return errors.Wrap(err, "failed to read Kubernetes version from bootstrap node")
	}
	if _, err := version.ParseSemantic(rawVersion); err != nil {
		return errors.Wrap(err, "failed to parse Kubernetes version")
	}

	if !tryUntil(ctx, deadline, func() bool {
		cmd := bootstrap.CommandContext(ctx,
			"kubectl",
			"--kubeconfig=/etc/kubernetes/admin.conf",
			"get",
			"nodes",
			"-o=jsonpath='{.items..status.conditions[-1:].status}'",
		)
		lines, err := exec.OutputLines(cmd)
		if err != nil || len(lines) == 0 {
			return false
		}
		// kubectl returns a string per node ("True True True" for 3 ready nodes).
		statuses := strings.Fields(lines[0])
		if len(statuses) == 0 {
			return false
		}
		for _, s := range statuses {
			if !strings.Contains(s, "True") {
				return false
			}
		}
		return true
	}) {
		return errors.New("deadline exceeded")
	}
	return nil
}

// tryUntil calls try() in a loop until it returns true or the deadline passes.
// Returns true if try() ever returned true; false on deadline or context
// cancellation. Adapted from pkg/cluster/internal/create/actions/waitforready.
func tryUntil(ctx context.Context, until time.Time, try func() bool) bool {
	for until.After(time.Now()) {
		if try() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(500 * time.Millisecond):
		}
	}
	return false
}

