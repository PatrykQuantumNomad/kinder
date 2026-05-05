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
	"errors"
	"strings"
	"testing"
)

// fakeReadinessOpts builds a clusterResumeReadinessCheck with all dependencies
// injected. Any nil function leaves the production default in place.
type fakeReadinessOpts struct {
	cpNodeNames []string
	binaryName  string
	listErr     error
	// execResults maps a string-joined "container|cmd args..." key to (lines, err).
	execResults  map[string]fakeExecLines
	snapshotID   string
	snapshotOK   bool
}

type fakeExecLines struct {
	lines []string
	err   error
}

// newFakeResumeReadinessCheck constructs a clusterResumeReadinessCheck wired to
// fake dependencies. Tests build the opts struct describing the cluster shape,
// what etcdctl returns, and what the snapshot file says.
func newFakeResumeReadinessCheck(opts fakeReadinessOpts) *clusterResumeReadinessCheck {
	return &clusterResumeReadinessCheck{
		listClusterNodes: func() ([]string, string, error) {
			if opts.listErr != nil {
				return nil, "", opts.listErr
			}
			binary := opts.binaryName
			if binary == "" && len(opts.cpNodeNames) > 0 {
				binary = "docker"
			}
			return opts.cpNodeNames, binary, nil
		},
		execInContainer: func(_ string, container string, cmd ...string) ([]string, error) {
			key := container + "|" + strings.Join(cmd, " ")
			if r, ok := opts.execResults[key]; ok {
				return r.lines, r.err
			}
			return nil, errors.New("fake exec: no result for key " + key)
		},
		readSnapshot: func(_ string, _ string) (string, bool) {
			return opts.snapshotID, opts.snapshotOK
		},
	}
}

func TestClusterResumeReadiness_Metadata(t *testing.T) {
	t.Parallel()
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{})
	if c.Name() != "cluster-resume-readiness" {
		t.Errorf("Name() = %q, want %q", c.Name(), "cluster-resume-readiness")
	}
	if c.Category() != "Cluster" {
		t.Errorf("Category() = %q, want %q", c.Category(), "Cluster")
	}
	if c.Platforms() != nil {
		t.Errorf("Platforms() = %v, want nil", c.Platforms())
	}
}

func TestClusterResumeReadiness_NoCluster_Skip(t *testing.T) {
	t.Parallel()
	// No CP nodes detected (empty slice) → skip "no kind cluster detected"
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		cpNodeNames: nil,
		binaryName:  "docker",
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "skip" {
		t.Errorf("Status = %q, want %q", r.Status, "skip")
	}
	if !strings.Contains(r.Message, "no kind cluster") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "no kind cluster")
	}
}

func TestClusterResumeReadiness_ListError_Skip(t *testing.T) {
	t.Parallel()
	// listClusterNodes returns an error → skip "no kind cluster detected"
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		listErr: errors.New("docker ps failed"),
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "skip" {
		t.Errorf("Status = %q, want %q", results[0].Status, "skip")
	}
}

func TestClusterResumeReadiness_SingleCP_Skip(t *testing.T) {
	t.Parallel()
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		cpNodeNames: []string{"kind-control-plane"},
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "skip" {
		t.Errorf("Status = %q, want %q (Message=%q)", r.Status, "skip", r.Message)
	}
	if !strings.Contains(r.Message, "single-control-plane") {
		t.Errorf("Message = %q, want to mention single-control-plane", r.Message)
	}
}

// TestClusterResumeReadiness_CrictlMissing_Skip: crictl ps errors (e.g. exit
// 127 — crictl not on PATH) → skip with "crictl unavailable" message.
func TestClusterResumeReadiness_CrictlMissing_Skip(t *testing.T) {
	t.Parallel()
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		cpNodeNames: []string{"cp1", "cp2", "cp3"},
		execResults: map[string]fakeExecLines{
			"cp1|crictl ps --name etcd -q": {err: errors.New("exit 127")},
		},
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "skip" {
		t.Errorf("Status = %q, want %q", r.Status, "skip")
	}
	if !strings.Contains(r.Message, "crictl unavailable") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "crictl unavailable")
	}
}

// TestClusterResumeReadiness_NoEtcdContainer_Skip: crictl ps succeeds but
// returns empty output (no running etcd container) → skip with message
// mentioning etcd container not running.
func TestClusterResumeReadiness_NoEtcdContainer_Skip(t *testing.T) {
	t.Parallel()
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		cpNodeNames: []string{"cp1", "cp2", "cp3"},
		execResults: map[string]fakeExecLines{
			"cp1|crictl ps --name etcd -q": {lines: []string{}},
		},
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "skip" {
		t.Errorf("Status = %q, want %q", r.Status, "skip")
	}
	if !strings.Contains(r.Message, "etcd container not running") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "etcd container not running")
	}
}

// healthyEtcdJSON returns a JSON array reporting all members healthy.
func healthyEtcdJSON(n int) string {
	parts := []string{}
	for i := 0; i < n; i++ {
		parts = append(parts, `{"endpoint":"https://127.0.0.1:2379","health":true,"took":"1ms"}`)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// statusEtcdJSON returns a JSON array reporting each member's leader id.
// All entries report the same leader (consensus). leader is given as a numeric
// string; it is encoded as a uint64 number in the JSON.
func statusEtcdJSON(leader string, n int) string {
	parts := []string{}
	for i := 0; i < n; i++ {
		parts = append(parts,
			`{"Endpoint":"https://127.0.0.1:2379","Status":{"header":{"member_id":1},"leader":`+leader+`}}`,
		)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func TestClusterResumeReadiness_HealthyHA_OK(t *testing.T) {
	t.Parallel()
	const leader = "12345"
	const etcdContainerID = "etcd-container-id-abc"
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		cpNodeNames: []string{"cp1", "cp2", "cp3"},
		execResults: map[string]fakeExecLines{
			"cp1|crictl ps --name etcd -q": {lines: []string{etcdContainerID}},
			"cp1|crictl exec " + etcdContainerID + " etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint health --cluster --write-out=json": {
				lines: []string{healthyEtcdJSON(3)},
			},
			"cp1|crictl exec " + etcdContainerID + " etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint status --cluster --write-out=json": {
				lines: []string{statusEtcdJSON(leader, 3)},
			},
		},
		// No snapshot present — should still report ok
		snapshotOK: false,
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d (%v)", len(results), results)
	}
	r := results[0]
	if r.Status != "ok" {
		t.Errorf("Status = %q, want %q (Message=%q Reason=%q)", r.Status, "ok", r.Message, r.Reason)
	}
	if !strings.Contains(r.Message, "3/3") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "3/3")
	}
}

func TestClusterResumeReadiness_UnhealthyMember_Warn(t *testing.T) {
	t.Parallel()
	const etcdContainerID = "etcd-container-id-abc"
	mixed := `[` +
		`{"endpoint":"https://127.0.0.1:2379","health":true,"took":"1ms"},` +
		`{"endpoint":"https://10.0.0.2:2379","health":true,"took":"1ms"},` +
		`{"endpoint":"https://10.0.0.3:2379","health":false,"error":"connection refused"}` +
		`]`
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		cpNodeNames: []string{"cp1", "cp2", "cp3"},
		execResults: map[string]fakeExecLines{
			"cp1|crictl ps --name etcd -q": {lines: []string{etcdContainerID}},
			"cp1|crictl exec " + etcdContainerID + " etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint health --cluster --write-out=json": {
				lines: []string{mixed},
			},
		},
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q (Message=%q Reason=%q)", r.Status, "warn", r.Message, r.Reason)
	}
	// Warn never escalates to fail (CONTEXT.md "warn and continue").
	if r.Status == "fail" {
		t.Error("Status must never be fail per CONTEXT.md warn-and-continue")
	}
	if !strings.Contains(r.Reason, "unhealthy") {
		t.Errorf("Reason = %q, want to contain %q", r.Reason, "unhealthy")
	}
}

func TestClusterResumeReadiness_AllUnhealthy_Warn(t *testing.T) {
	t.Parallel()
	const etcdContainerID = "etcd-container-id-abc"
	allBad := `[` +
		`{"endpoint":"https://127.0.0.1:2379","health":false,"error":"refused"},` +
		`{"endpoint":"https://10.0.0.2:2379","health":false,"error":"refused"},` +
		`{"endpoint":"https://10.0.0.3:2379","health":false,"error":"refused"}` +
		`]`
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		cpNodeNames: []string{"cp1", "cp2", "cp3"},
		execResults: map[string]fakeExecLines{
			"cp1|crictl ps --name etcd -q": {lines: []string{etcdContainerID}},
			"cp1|crictl exec " + etcdContainerID + " etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint health --cluster --write-out=json": {
				lines: []string{allBad},
			},
		},
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q", r.Status, "warn")
	}
	if !strings.Contains(r.Reason, "no healthy etcd members") {
		t.Errorf("Reason = %q, want to contain %q", r.Reason, "no healthy etcd members")
	}
}

func TestClusterResumeReadiness_StaleSnapshot_Warn(t *testing.T) {
	t.Parallel()
	const currentLeader = "99999"
	const staleLeader = "11111"
	const etcdContainerID = "etcd-container-id-abc"
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		cpNodeNames: []string{"cp1", "cp2", "cp3"},
		execResults: map[string]fakeExecLines{
			"cp1|crictl ps --name etcd -q": {lines: []string{etcdContainerID}},
			"cp1|crictl exec " + etcdContainerID + " etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint health --cluster --write-out=json": {
				lines: []string{healthyEtcdJSON(3)},
			},
			"cp1|crictl exec " + etcdContainerID + " etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint status --cluster --write-out=json": {
				lines: []string{statusEtcdJSON(currentLeader, 3)},
			},
		},
		snapshotID: staleLeader,
		snapshotOK: true,
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q (Message=%q Reason=%q)", r.Status, "warn", r.Message, r.Reason)
	}
	if !strings.Contains(r.Reason, "leader") {
		t.Errorf("Reason = %q, want to mention leader change", r.Reason)
	}
}

func TestClusterResumeReadiness_FreshSnapshot_OK(t *testing.T) {
	t.Parallel()
	const leader = "55555"
	const etcdContainerID = "etcd-container-id-abc"
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		cpNodeNames: []string{"cp1", "cp2", "cp3"},
		execResults: map[string]fakeExecLines{
			"cp1|crictl ps --name etcd -q": {lines: []string{etcdContainerID}},
			"cp1|crictl exec " + etcdContainerID + " etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint health --cluster --write-out=json": {
				lines: []string{healthyEtcdJSON(3)},
			},
			"cp1|crictl exec " + etcdContainerID + " etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint status --cluster --write-out=json": {
				lines: []string{statusEtcdJSON(leader, 3)},
			},
		},
		snapshotID: leader,
		snapshotOK: true,
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "ok" {
		t.Errorf("Status = %q, want %q (Message=%q Reason=%q)", results[0].Status, "ok", results[0].Message, results[0].Reason)
	}
}

func TestClusterResumeReadiness_NoSnapshot_OK(t *testing.T) {
	t.Parallel()
	const leader = "77777"
	const etcdContainerID = "etcd-container-id-abc"
	c := newFakeResumeReadinessCheck(fakeReadinessOpts{
		cpNodeNames: []string{"cp1", "cp2", "cp3"},
		execResults: map[string]fakeExecLines{
			"cp1|crictl ps --name etcd -q": {lines: []string{etcdContainerID}},
			"cp1|crictl exec " + etcdContainerID + " etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint health --cluster --write-out=json": {
				lines: []string{healthyEtcdJSON(3)},
			},
			"cp1|crictl exec " + etcdContainerID + " etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint status --cluster --write-out=json": {
				lines: []string{statusEtcdJSON(leader, 3)},
			},
		},
		// snapshotOK: false → snapshot file missing
		snapshotOK: false,
	})
	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "ok" {
		t.Errorf("Status = %q, want %q (without snapshot, healthy etcd is still ok)", results[0].Status, "ok")
	}
}

func TestRegistry_ContainsResumeReadiness(t *testing.T) {
	t.Parallel()
	checks := AllChecks()
	found := false
	for _, c := range checks {
		if c.Name() == "cluster-resume-readiness" {
			if c.Category() != "Cluster" {
				t.Errorf("cluster-resume-readiness category = %q, want %q", c.Category(), "Cluster")
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("AllChecks() does not contain cluster-resume-readiness")
	}
}
