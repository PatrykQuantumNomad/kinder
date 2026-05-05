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

package doctor

import (
	"encoding/json"
	"fmt"
	osexec "os/exec"
	"sort"
	"strings"

	"sigs.k8s.io/kind/pkg/exec"
)

// clusterResumeReadinessCheck (LIFE-04, phase 47 plan 04) inspects an HA kinder
// cluster after pause/resume and warns when etcd quorum is at risk or the
// elected leader has rotated since pause. The check is HA-only (single-CP
// clusters skip), and per CONTEXT.md "warn and continue" semantics it never
// reports `fail` — only `ok`, `warn`, or `skip`. The same check is invoked
// inline from lifecycle.Resume between CP-start and worker-start so HA users
// see warnings automatically without running `kinder doctor` separately.
type clusterResumeReadinessCheck struct {
	// listClusterNodes returns the sorted control-plane container names plus the
	// detected runtime binary ("docker"/"podman"/"nerdctl"). Returning an empty
	// slice or an error is treated as "no kind cluster detected" → skip.
	listClusterNodes func() (cpNodeNames []string, binaryName string, err error)
	// execInContainer runs `<binaryName> exec <container> <cmd...>` and returns
	// stdout split into lines. Used for `which etcdctl` probing and the two
	// etcdctl invocations (endpoint health, endpoint status).
	execInContainer func(binaryName, container string, cmd ...string) ([]string, error)
	// readSnapshot returns the leader id captured at pause time from
	// /kind/pause-snapshot.json, plus a presence bool. ok=false means the
	// file is absent or unparseable — treated as "no prior leader info"
	// (snapshot is optional context, not required).
	readSnapshot func(binaryName, container string) (leaderID string, ok bool)
}

// newClusterResumeReadinessCheck returns the production check wired to real
// container CLI calls. The snapshot reader, etcdctl runner, and CP node lister
// are all real-implementation closures defined below.
func newClusterResumeReadinessCheck() Check {
	return &clusterResumeReadinessCheck{
		listClusterNodes: realListCPNodes,
		execInContainer:  realExecInContainer,
		readSnapshot:     realReadPauseSnapshot,
	}
}

// NewClusterResumeReadinessCheck is the exported constructor used by callers
// outside the doctor package — specifically, lifecycle.Resume invokes this
// inline on HA clusters during plan 47-04 so warnings flow through the same
// code path as `kinder doctor`.
func NewClusterResumeReadinessCheck() Check {
	return newClusterResumeReadinessCheck()
}

func (c *clusterResumeReadinessCheck) Name() string       { return "cluster-resume-readiness" }
func (c *clusterResumeReadinessCheck) Category() string    { return "Cluster" }
func (c *clusterResumeReadinessCheck) Platforms() []string { return nil }

// etcdctlAuthArgs is the shared cert/endpoint argument tuple used by both
// etcdctl invocations (endpoint health, endpoint status). Kept as a package
// var so the snapshot capture in lifecycle/pause.go and this check stay in
// sync if cert paths ever move.
var etcdctlAuthArgs = []string{
	"--cacert=/etc/kubernetes/pki/etcd/ca.crt",
	"--cert=/etc/kubernetes/pki/etcd/peer.crt",
	"--key=/etc/kubernetes/pki/etcd/peer.key",
	"--endpoints=https://127.0.0.1:2379",
}

// Run executes the readiness check on a best-effort basis. Disposition matrix:
//
//	skip — no cluster detected, single-CP cluster, or etcdctl unavailable
//	warn — at least one etcd member unhealthy, no healthy members, OR
//	       elected leader changed since pause (snapshot mismatch)
//	ok   — all etcd members healthy AND (no snapshot OR snapshot leader matches
//	       current leader)
//	fail — never (warn-and-continue per CONTEXT.md)
func (c *clusterResumeReadinessCheck) Run() []Result {
	cpNodeNames, binaryName, err := c.listClusterNodes()
	if err != nil || len(cpNodeNames) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "no kind cluster detected",
		}}
	}
	if len(cpNodeNames) <= 1 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "single-control-plane cluster; HA check not applicable",
		}}
	}

	bootstrap := cpNodeNames[0]

	// 1. Probe etcdctl availability inside the bootstrap CP container.
	if _, err := c.execInContainer(binaryName, bootstrap, "which", "etcdctl"); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "etcdctl unavailable inside container",
		}}
	}

	// 2. Run `etcdctl endpoint health --cluster` to count healthy/unhealthy members.
	healthArgs := append([]string{"/usr/local/bin/etcdctl"}, etcdctlAuthArgs...)
	healthArgs = append(healthArgs, "endpoint", "health", "--cluster", "--write-out=json")
	healthLines, err := c.execInContainer(binaryName, bootstrap, healthArgs...)
	if err != nil {
		// etcdctl ran (which succeeded) but health probe failed — quorum
		// likely lost. Warn (never fail).
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "etcd endpoint health probe failed",
			Reason:   fmt.Sprintf("etcdctl endpoint health returned error: %v", err),
			Fix:      "Investigate etcd state: kinder status; kubectl get nodes",
		}}
	}
	healthy, total, healthErr := parseEtcdHealth(strings.Join(healthLines, ""))
	if healthErr != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "could not parse etcd health output",
			Reason:   healthErr.Error(),
			Fix:      "Re-run with: kinder doctor --output json | jq",
		}}
	}
	if healthy == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("0/%d etcd members healthy", total),
			Reason:   "no healthy etcd members reachable; quorum lost",
			Fix:      "Investigate etcd state: kinder status; kubectl get pods -n kube-system",
		}}
	}
	if healthy < total {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("%d/%d etcd members healthy", healthy, total),
			Reason:   fmt.Sprintf("%d unhealthy etcd member(s) — quorum at risk", total-healthy),
			Fix:      "Investigate etcd state: kinder status; kubectl get pods -n kube-system",
		}}
	}

	// 3. All members healthy. Check snapshot freshness when present.
	snapLeader, snapOK := c.readSnapshot(binaryName, bootstrap)
	if snapOK && snapLeader != "" {
		statusArgs := append([]string{"/usr/local/bin/etcdctl"}, etcdctlAuthArgs...)
		statusArgs = append(statusArgs, "endpoint", "status", "--cluster", "--write-out=json")
		statusLines, statusErr := c.execInContainer(binaryName, bootstrap, statusArgs...)
		if statusErr == nil {
			if currentLeader, parseErr := parseEtcdStatusLeader(strings.Join(statusLines, "")); parseErr == nil && currentLeader != "" {
				if currentLeader != snapLeader {
					return []Result{{
						Name:     c.Name(),
						Category: c.Category(),
						Status:   "warn",
						Message:  "etcd leader changed since pause",
						Reason:   fmt.Sprintf("leader id rotated; previous=%s, current=%s", snapLeader, currentLeader),
						Fix:      "Verify cluster health: kubectl get nodes; kinder status",
					}}
				}
			}
		}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  fmt.Sprintf("%d/%d etcd members healthy", healthy, total),
	}}
}

// parseEtcdHealth parses the JSON array emitted by
// `etcdctl endpoint health --cluster --write-out=json`. The schema per entry is
// {"endpoint":"...", "health":bool, "took":"...", "error":"..."}.
// Returns (healthyCount, totalCount, err). An empty array returns (0, 0, nil).
func parseEtcdHealth(rawJSON string) (healthy, total int, err error) {
	var entries []map[string]interface{}
	if uErr := json.Unmarshal([]byte(rawJSON), &entries); uErr != nil {
		return 0, 0, fmt.Errorf("etcd health JSON parse: %w", uErr)
	}
	for _, e := range entries {
		total++
		if h, ok := e["health"].(bool); ok && h {
			healthy++
		}
	}
	return healthy, total, nil
}

// parseEtcdStatusLeader extracts the consensus leader member id from
// `etcdctl endpoint status --cluster --write-out=json`. Each entry has a
// "Status" object with "leader" (uint64 in JSON). The first non-zero leader
// wins (etcd has a single elected leader at a time across the cluster).
// Returns "" with no error when the array is empty or no leader is reported.
func parseEtcdStatusLeader(rawJSON string) (string, error) {
	var entries []map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &entries); err != nil {
		return "", fmt.Errorf("etcd status JSON parse: %w", err)
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

// realListCPNodes discovers control-plane containers in the default kind
// cluster using the same low-level CLI pattern as clusterskew.go (avoids
// importing the cluster package, which would create an import cycle).
//
// Returns the sorted CP container names, the detected runtime binary, and
// nil/error. An empty slice with no error is the "no cluster found" sentinel
// (treated as skip by the check).
func realListCPNodes() ([]string, string, error) {
	var binaryName string
	for _, rt := range []string{"docker", "podman", "nerdctl"} {
		if _, err := osexec.LookPath(rt); err == nil {
			binaryName = rt
			break
		}
	}
	if binaryName == "" {
		return nil, "", nil
	}
	// List all kind cluster containers, filtering by role label.
	lines, err := exec.OutputLines(exec.Command(
		binaryName, "ps",
		"--filter", "label=io.x-k8s.kind.cluster",
		"--format", `{{.Names}}|{{.Label "io.x-k8s.kind.role"}}`,
	))
	if err != nil {
		return nil, binaryName, err
	}
	var cps []string
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 2)
		if len(parts) != 2 {
			continue
		}
		name, role := parts[0], parts[1]
		if role == "control-plane" && name != "" {
			cps = append(cps, name)
		}
	}
	sort.Strings(cps)
	return cps, binaryName, nil
}

// realExecInContainer runs `<binaryName> exec <container> <cmd...>` and
// returns stdout split into lines (matches the production pattern used by
// clusterskew.go realListNodes).
func realExecInContainer(binaryName, container string, cmd ...string) ([]string, error) {
	args := append([]string{"exec", container}, cmd...)
	return exec.OutputLines(exec.Command(binaryName, args...))
}

// realReadPauseSnapshot reads /kind/pause-snapshot.json from inside the named
// container and returns the captured leader id plus a presence bool. Any
// error (file missing, parse failure, empty leader) returns ("", false) so
// the check treats the snapshot as absent — which is the explicit "snapshot
// is optional context" semantic from plan 47-02.
func realReadPauseSnapshot(binaryName, container string) (string, bool) {
	lines, err := exec.OutputLines(exec.Command(binaryName, "exec", container, "cat", "/kind/pause-snapshot.json"))
	if err != nil || len(lines) == 0 {
		return "", false
	}
	var snap struct {
		LeaderID string `json:"leaderID"`
	}
	if err := json.Unmarshal([]byte(strings.Join(lines, "")), &snap); err != nil {
		return "", false
	}
	if snap.LeaderID == "" {
		return "", false
	}
	return snap.LeaderID, true
}
