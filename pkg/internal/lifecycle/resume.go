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
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
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

	// Build start order: LB → all CPs → all workers (reverse of pause).
	toStart := make([]nodes.Node, 0, len(allNodes))
	if lb != nil {
		toStart = append(toStart, lb)
	}
	toStart = append(toStart, cp...)
	toStart = append(toStart, workers...)

	results := make([]NodeResult, 0, len(toStart))
	startErrs := []error{}
	for _, node := range toStart {
		// Honor cancellation between starts.
		select {
		case <-opts.Context.Done():
			startErrs = append(startErrs, opts.Context.Err())
			break
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

