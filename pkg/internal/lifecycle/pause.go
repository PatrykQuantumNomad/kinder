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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// PauseOptions controls a single Pause invocation.
type PauseOptions struct {
	// ClusterName is the resolved cluster name. Callers should run the args
	// through ResolveClusterName before populating this field; Pause itself
	// rejects an empty value.
	ClusterName string
	// Timeout is the per-container graceful stop timeout. Defaults to 30s
	// when zero.
	Timeout time.Duration
	// Logger receives V(0) per-node progress lines and snapshot warnings.
	// Defaults to log.NoopLogger{} when nil.
	Logger log.Logger
	// Provider is the live container runtime provider used to enumerate
	// cluster nodes. Tests can swap the package-level pauseNodeFetcher to
	// avoid building a provider; production callers always pass a real one.
	Provider *cluster.Provider
}

// NodeResult records the outcome of stopping a single node.
type NodeResult struct {
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	Success  bool    `json:"success"`
	Error    string  `json:"error,omitempty"`
	Duration float64 `json:"durationSeconds"`
}

// PauseResult is the structured return value of Pause(). It is also the JSON
// shape emitted by `kinder pause --json`.
type PauseResult struct {
	Cluster       string       `json:"cluster"`
	State         string       `json:"state"`
	AlreadyPaused bool         `json:"alreadyPaused,omitempty"`
	Nodes         []NodeResult `json:"nodes"`
	Duration      float64      `json:"durationSeconds"`
}

// pauseSnapshot is the on-disk schema of /kind/pause-snapshot.json written
// inside the bootstrap CP container of HA clusters. Plan 47-04's resume
// readiness check reads this file back to compare leader identity across the
// pause/resume gap.
type pauseSnapshot struct {
	LeaderID  string    `json:"leaderID"`
	PauseTime time.Time `json:"pauseTime"`
}

// nodeFetcher is the minimum surface Pause needs from a cluster provider.
// *cluster.Provider satisfies this interface structurally.
type nodeFetcher interface {
	ListNodes(name string) ([]nodes.Node, error)
}

// pauseNodeFetcher is overridden by tests via withFetcher to avoid wiring a
// real *cluster.Provider into unit tests. When nil (production), Pause
// constructs the fetcher from the supplied opts.Provider.
var pauseNodeFetcher nodeFetcher

// pauseBinaryName is the test-injection point for the container runtime
// binary name. In production it forwards to ProviderBinaryName.
var pauseBinaryName = ProviderBinaryName

// Pause stops every container in the named cluster in quorum-safe order
// (workers → control-plane → external-load-balancer). The function is
// best-effort: a failure on one node does not abort the rest, and all errors
// are aggregated into the returned error. PauseResult is always returned
// non-nil when ClusterName is valid so callers can render structured output
// even on partial failure.
//
// On HA clusters (≥2 control-plane nodes) Pause additionally captures a
// pre-pause snapshot to /kind/pause-snapshot.json inside the bootstrap CP
// container; snapshot capture is itself best-effort and never fails the pause.
//
// Pause is idempotent: if every node is already in a stopped state,
// AlreadyPaused is set on the returned PauseResult and no docker stop calls
// are issued.
func Pause(opts PauseOptions) (*PauseResult, error) {
	t0 := time.Now()
	if opts.ClusterName == "" {
		return nil, errors.New("Pause: ClusterName is empty")
	}
	if opts.Logger == nil {
		opts.Logger = log.NoopLogger{}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}

	fetcher := pauseNodeFetcher
	if fetcher == nil {
		if opts.Provider == nil {
			return nil, errors.New("Pause: Provider is required when no test fetcher is installed")
		}
		fetcher = opts.Provider
	}

	allNodes, err := fetcher.ListNodes(opts.ClusterName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list nodes for cluster %q", opts.ClusterName)
	}
	if len(allNodes) == 0 {
		return nil, errors.Errorf("cluster %q has no nodes", opts.ClusterName)
	}

	binaryName := pauseBinaryName()
	if binaryName == "" {
		return nil, errors.New("Pause: no container runtime detected (docker, podman, nerdctl)")
	}

	// Idempotency: if the cluster already reports as Paused, no-op.
	if status := ClusterStatus(binaryName, allNodes); status == "Paused" {
		opts.Logger.Warn(fmt.Sprintf("cluster %q is already paused; no action taken", opts.ClusterName))
		return &PauseResult{
			Cluster:       opts.ClusterName,
			State:         "paused",
			AlreadyPaused: true,
			Nodes:         []NodeResult{},
			Duration:      time.Since(t0).Seconds(),
		}, nil
	}

	cp, workers, lb, err := ClassifyNodes(allNodes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to classify nodes")
	}

	// HA snapshot capture: only when ≥2 control-plane nodes exist. Best-effort
	// — any failure is logged and the pause proceeds.
	if len(cp) >= 2 {
		captureHASnapshot(opts.Logger, cp[0])
	}

	// Build ordered stop list: workers → CP → LB.
	type nodeWithRole struct {
		node nodes.Node
		role string
	}
	toStop := make([]nodeWithRole, 0, len(allNodes))
	for _, w := range workers {
		toStop = append(toStop, nodeWithRole{node: w, role: "worker"})
	}
	for _, c := range cp {
		toStop = append(toStop, nodeWithRole{node: c, role: "control-plane"})
	}
	if lb != nil {
		toStop = append(toStop, nodeWithRole{node: lb, role: "external-load-balancer"})
	}

	results := make([]NodeResult, 0, len(toStop))
	var errs []error
	for _, nr := range toStop {
		start := time.Now()
		cmd := defaultCmder(
			binaryName,
			"stop",
			fmt.Sprintf("--time=%d", int(opts.Timeout.Seconds())),
			nr.node.String(),
		)
		runErr := cmd.Run()
		dur := time.Since(start).Seconds()
		res := NodeResult{
			Name:     nr.node.String(),
			Role:     nr.role,
			Success:  runErr == nil,
			Duration: dur,
		}
		if runErr != nil {
			res.Error = runErr.Error()
			errs = append(errs, errors.Wrapf(runErr, "failed to stop %s", nr.node.String()))
			opts.Logger.V(0).Infof(" x %s (role: %s, %.1fs) — %v", nr.node.String(), nr.role, dur, runErr)
		} else {
			opts.Logger.V(0).Infof(" - %s stopped (role: %s, %.1fs)", nr.node.String(), nr.role, dur)
		}
		results = append(results, res)
	}

	result := &PauseResult{
		Cluster:  opts.ClusterName,
		State:    "paused",
		Nodes:    results,
		Duration: time.Since(t0).Seconds(),
	}
	if len(errs) > 0 {
		return result, errors.NewAggregate(errs)
	}
	return result, nil
}

// captureHASnapshot writes /kind/pause-snapshot.json inside the bootstrap CP
// container. All failures are logged as warnings — snapshot capture must
// never fail the pause.
func captureHASnapshot(logger log.Logger, bootstrap nodes.Node) {
	leaderID, err := readEtcdLeaderID(bootstrap)
	if err != nil {
		logger.Warnf("failed to capture etcd leader id for HA pause snapshot (continuing): %v", err)
		// Still write a snapshot with an empty leader id so plan 04's resume
		// readiness check has a timestamp to anchor against.
		leaderID = ""
	}
	snap := pauseSnapshot{
		LeaderID:  leaderID,
		PauseTime: time.Now().UTC(),
	}
	payload, mErr := json.Marshal(snap)
	if mErr != nil {
		logger.Warnf("failed to marshal pause snapshot (continuing): %v", mErr)
		return
	}
	// Use sh -c with a heredoc to avoid shell-quoting hell around the JSON
	// payload (which contains its own double quotes).
	script := fmt.Sprintf("cat > /kind/pause-snapshot.json <<'PAUSE_SNAP_EOF'\n%s\nPAUSE_SNAP_EOF\n", string(payload))
	writeCmd := bootstrap.Command("sh", "-c", script)
	if wErr := writeCmd.Run(); wErr != nil {
		logger.Warnf("failed to write /kind/pause-snapshot.json on bootstrap CP (continuing): %v", wErr)
	}
}

// readEtcdLeaderID discovers the current etcd leader's member id by running
// etcdctl inside the etcd static-pod container via crictl exec. This approach
// works on real kinder clusters where etcdctl ships only inside the etcd
// container image (registry.k8s.io/etcd:VERSION), not in the kindest/node
// rootfs. crictl is available on kindest/node (used by the container runtime).
//
// Best-effort: any failure (crictl missing, no etcd container, etcdctl error,
// parse error) returns ("", err) so captureHASnapshot can log a warning and
// still write the snapshot with an empty leaderID.
func readEtcdLeaderID(bootstrap nodes.Node) (string, error) {
	// 1. Discover the running etcd static-pod container id via crictl.
	idOut, err := exec.OutputLines(bootstrap.Command("crictl", "ps", "--name", "etcd", "-q"))
	if err != nil {
		return "", errors.Wrap(err, "crictl ps for etcd container failed")
	}
	var etcdContainerID string
	for _, line := range idOut {
		if id := strings.TrimSpace(line); id != "" {
			etcdContainerID = id
			break
		}
	}
	if etcdContainerID == "" {
		return "", errors.New("etcd container not running on bootstrap CP")
	}

	// 2. Run etcdctl endpoint status inside the etcd container via crictl exec.
	// The etcd container has etcdctl on its PATH and /etc/kubernetes/pki/etcd/
	// bind-mounted by kubelet — the same cert paths as in the kindest/node rootfs.
	etcdctlArgs := []string{
		"--cacert=/etc/kubernetes/pki/etcd/ca.crt",
		"--cert=/etc/kubernetes/pki/etcd/peer.crt",
		"--key=/etc/kubernetes/pki/etcd/peer.key",
		"--endpoints=https://127.0.0.1:2379",
		"endpoint", "status", "--cluster", "--write-out=json",
	}
	cmdArgs := append([]string{"crictl", "exec", etcdContainerID, "etcdctl"}, etcdctlArgs...)
	out, err := exec.OutputLines(bootstrap.Command(cmdArgs[0], cmdArgs[1:]...))
	if err != nil {
		return "", errors.Wrap(err, "etcdctl endpoint status via crictl exec failed")
	}
	joined := strings.Join(out, "")
	if joined == "" {
		return "", errors.New("empty etcdctl output")
	}
	return parseEtcdLeader(joined)
}

// parseEtcdLeader extracts the leader member id from etcdctl's
// `endpoint status --cluster --write-out=json` output. The output is a JSON
// array; the leader is reported per-endpoint under Status.leader (uint64).
//
// We only need a single leader id (etcd reports one elected leader at a time
// across the cluster); the first non-zero value wins. Returns "" with no
// error if the array is empty or no leader was reported.
func parseEtcdLeader(rawJSON string) (string, error) {
	var entries []map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &entries); err != nil {
		return "", errors.Wrapf(err, "failed to parse etcdctl json")
	}
	for _, e := range entries {
		statusRaw, ok := e["Status"]
		if !ok {
			continue
		}
		status, ok := statusRaw.(map[string]interface{})
		if !ok {
			continue
		}
		switch v := status["leader"].(type) {
		case float64:
			if v != 0 {
				return fmt.Sprintf("%d", uint64(v)), nil
			}
		case string:
			if v != "" && v != "0" {
				return v, nil
			}
		}
	}
	return "", nil
}
