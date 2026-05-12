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

package doctor

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// makeNodeEntries is a test helper to build nodeEntry slices.
func makeNodeEntries(entries []nodeEntry) []nodeEntry { return entries }

// newTestClusterNodeSkewCheck builds a clusterNodeSkewCheck backed by a
// static list of nodeEntries — no real container runtime required.
func newTestClusterNodeSkewCheck(entries []nodeEntry, listErr error) *clusterNodeSkewCheck {
	return &clusterNodeSkewCheck{
		listNodes: func() ([]nodeEntry, error) {
			return entries, listErr
		},
	}
}

func TestClusterNodeSkewCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newTestClusterNodeSkewCheck(nil, nil)
	if check.Name() != "cluster-node-skew" {
		t.Errorf("Name() = %q, want %q", check.Name(), "cluster-node-skew")
	}
	if check.Category() != "Cluster" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Cluster")
	}
	if check.Platforms() != nil {
		t.Errorf("Platforms() = %v, want nil", check.Platforms())
	}
}

func TestClusterNodeSkew_NoCluster(t *testing.T) {
	t.Parallel()
	check := newTestClusterNodeSkewCheck(nil, nil)
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "skip" {
		t.Errorf("Status = %q, want %q", r.Status, "skip")
	}
	if !strings.Contains(r.Message, "no cluster found") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "no cluster found")
	}
}

func TestClusterNodeSkew_AllSameVersion_NoViolation(t *testing.T) {
	t.Parallel()
	entries := makeNodeEntries([]nodeEntry{
		{Name: "cp", Role: "control-plane", Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
		{Name: "w1", Role: "worker", Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
		{Name: "w2", Role: "worker", Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
	})
	check := newTestClusterNodeSkewCheck(entries, nil)
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	r := results[0]
	if r.Status != "ok" {
		t.Errorf("Status = %q, want %q; Message=%q Reason=%q", r.Status, "ok", r.Message, r.Reason)
	}
}

func TestClusterNodeSkew_WorkerFourMinorsBehind_Warn(t *testing.T) {
	t.Parallel()
	entries := makeNodeEntries([]nodeEntry{
		{Name: "cp", Role: "control-plane", Version: "v1.31.0", Image: "kindest/node:v1.31.0"},
		{Name: "w1", Role: "worker", Version: "v1.27.0", Image: "kindest/node:v1.27.0"},
	})
	check := newTestClusterNodeSkewCheck(entries, nil)
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q; Message=%q", r.Status, "warn", r.Message)
	}
	if !strings.Contains(r.Message, "w1") {
		t.Errorf("Message = %q, want to contain node name %q", r.Message, "w1")
	}
	// Should mention skew violation
	if !strings.Contains(r.Message, "skew") && !strings.Contains(r.Reason, "skew") {
		t.Errorf("Result should mention skew; Message=%q Reason=%q", r.Message, r.Reason)
	}
}

func TestClusterNodeSkew_HAControlPlaneVersionMismatch_Warn(t *testing.T) {
	t.Parallel()
	entries := makeNodeEntries([]nodeEntry{
		{Name: "cp1", Role: "control-plane", Version: "v1.31.0", Image: "kindest/node:v1.31.0"},
		{Name: "cp2", Role: "control-plane", Version: "v1.30.0", Image: "kindest/node:v1.30.0"},
		{Name: "cp3", Role: "control-plane", Version: "v1.31.0", Image: "kindest/node:v1.31.0"},
		{Name: "w1", Role: "worker", Version: "v1.31.0", Image: "kindest/node:v1.31.0"},
	})
	check := newTestClusterNodeSkewCheck(entries, nil)
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q; Message=%q", r.Status, "warn", r.Message)
	}
	// Should mention the mismatched control-plane node
	if !strings.Contains(r.Message, "cp2") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "cp2")
	}
}

func TestClusterNodeSkew_KubeVersionReadFails_Warn(t *testing.T) {
	t.Parallel()
	// Simulate listNodes returning an entry with empty version (read failure)
	entries := makeNodeEntries([]nodeEntry{
		{Name: "cp", Role: "control-plane", Version: "", Image: "kindest/node:v1.31.0", VersionErr: errors.New("cat: /kind/version: No such file")},
		{Name: "w1", Role: "worker", Version: "v1.31.0", Image: "kindest/node:v1.31.0"},
	})
	check := newTestClusterNodeSkewCheck(entries, nil)
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q; Message=%q", r.Status, "warn", r.Message)
	}
	if !strings.Contains(r.Message, "cp") {
		t.Errorf("Message = %q, want to contain node name %q", r.Message, "cp")
	}
}

func TestClusterNodeSkew_MultipleViolations_SingleResult(t *testing.T) {
	t.Parallel()
	entries := makeNodeEntries([]nodeEntry{
		{Name: "cp", Role: "control-plane", Version: "v1.31.0", Image: "kindest/node:v1.31.0"},
		{Name: "w1", Role: "worker", Version: "v1.27.0", Image: "kindest/node:v1.27.0"},
		{Name: "w2", Role: "worker", Version: "v1.26.0", Image: "kindest/node:v1.26.0"},
	})
	check := newTestClusterNodeSkewCheck(entries, nil)
	results := check.Run()
	// All violations should be in a single result (table format)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q", r.Status, "warn")
	}
	// Both violating workers should appear in the message
	if !strings.Contains(r.Message, "w1") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "w1")
	}
	if !strings.Contains(r.Message, "w2") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "w2")
	}
}

func TestClusterNodeSkew_ConfigDrift_Warn(t *testing.T) {
	t.Parallel()
	// Image tag says v1.31.0 but live version is v1.30.5 (drift)
	entries := makeNodeEntries([]nodeEntry{
		{Name: "cp", Role: "control-plane", Version: "v1.31.0", Image: "kindest/node:v1.30.5"},
	})
	check := newTestClusterNodeSkewCheck(entries, nil)
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q; Message=%q", r.Status, "warn", r.Message)
	}
	if !strings.Contains(r.Message, "drift") && !strings.Contains(r.Reason, "drift") {
		t.Errorf("Result should mention drift; Message=%q Reason=%q", r.Message, r.Reason)
	}
}

func TestClusterNodeSkew_NoConfigDrift_OK(t *testing.T) {
	t.Parallel()
	// Image tag and live version match
	entries := makeNodeEntries([]nodeEntry{
		{Name: "cp", Role: "control-plane", Version: "v1.31.0", Image: "kindest/node:v1.31.0"},
	})
	check := newTestClusterNodeSkewCheck(entries, nil)
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "ok" {
		t.Errorf("Status = %q, want %q; Message=%q Reason=%q", r.Status, "ok", r.Message, r.Reason)
	}
}

// TestClusterNodeSkew_NonDefaultClusterName_Discovered verifies that
// clusterNodeSkewCheck works with any cluster name when listNodes is injected
// (regression coverage for the presence-only filter refactor).
func TestClusterNodeSkew_NonDefaultClusterName_Discovered(t *testing.T) {
	t.Parallel()
	// Simulate a cluster named "verify47" with non-default name containers.
	entries := makeNodeEntries([]nodeEntry{
		{Name: "verify47-control-plane", Role: "control-plane", Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
		{Name: "verify47-worker", Role: "worker", Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
	})
	check := newTestClusterNodeSkewCheck(entries, nil)
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "ok" {
		t.Errorf("Status = %q, want %q (Message=%q Reason=%q)", r.Status, "ok", r.Message, r.Reason)
	}
}

// TestClusterNodeSkew_RealListFilter_NoValuePin verifies that clusterFilter()
// returns a presence-only label filter (no "=kind" suffix). This test fails
// until the GREEN refactor extracts clusterFilter() and removes the =kind pin.
func TestClusterNodeSkew_RealListFilter_NoValuePin(t *testing.T) {
	t.Parallel()
	// clusterFilter() must exist (factored out in GREEN) and must NOT contain =kind.
	args := clusterFilter()
	found := false
	for _, a := range args {
		if a == "label=io.x-k8s.kind.cluster" {
			found = true
		}
		if a == "label=io.x-k8s.kind.cluster=kind" {
			t.Errorf("clusterFilter() returned the pinned =kind filter — breaks non-default cluster names")
		}
	}
	if !found {
		t.Errorf("clusterFilter() args %v do not contain the presence-only label filter %q", args, "label=io.x-k8s.kind.cluster")
	}
}

func TestImageTagVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		image   string
		wantVer string
		wantErr bool
	}{
		{"kindest/node:v1.31.2", "v1.31.2", false},
		{"kindest/node:v1.28.0", "v1.28.0", false},
		{"registry.k8s.io/kindest/node:v1.30.0@sha256:abc", "v1.30.0", false},
		{"kindest/node:latest", "", true},
		{"kindest/node", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			t.Parallel()
			got, err := imageTagVersion(tt.image)
			if (err != nil) != tt.wantErr {
				t.Errorf("imageTagVersion(%q) error = %v, wantErr %v", tt.image, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantVer {
				t.Errorf("imageTagVersion(%q) = %q, want %q", tt.image, got, tt.wantVer)
			}
		})
	}
}

// TestClusterNodeSkew_ExternalLoadBalancer_NotWarned is the SC1 runtime gate
// for Phase 57 DIAG-05. It asserts that an external-load-balancer entry with
// no live version (because realListNodes skipped the cat /kind/version exec
// for the LB role) does NOT trigger the version-readable warning at Run()
// lines 171-178, and is correctly skipped by all subsequent checks (CP
// consistency, worker skew, config drift).
func TestClusterNodeSkew_ExternalLoadBalancer_NotWarned(t *testing.T) {
	t.Parallel()
	// 3-CP HA cluster + 1 worker + 1 external-load-balancer container.
	// Post-fix invariant: realListNodes returns an LB entry with VersionErr == nil
	// (the cat /kind/version exec is skipped for LB role), so Run() must NOT
	// emit a version-readable warning for the LB container name. SC1 of Phase 57.
	entries := makeNodeEntries([]nodeEntry{
		{Name: "kind-control-plane", Role: "control-plane", Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
		{Name: "kind-control-plane2", Role: "control-plane", Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
		{Name: "kind-control-plane3", Role: "control-plane", Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
		{Name: "kind-worker", Role: "worker", Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
		{Name: "kind-external-load-balancer", Role: "external-load-balancer", Version: "", Image: "kindest/haproxy:v20230510-486859a6"},
	})
	check := newTestClusterNodeSkewCheck(entries, nil)
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	r := results[0]
	if r.Status != "ok" {
		t.Errorf("Status = %q, want %q (Message=%q Reason=%q)", r.Status, "ok", r.Message, r.Reason)
	}
	if strings.Contains(r.Message, "kind-external-load-balancer") {
		t.Errorf("Message must not contain the LB container name; got %q", r.Message)
	}
	if r.Reason != "" {
		t.Errorf("Reason should be empty for an all-healthy HA cluster + LB; got %q", r.Reason)
	}
}

// TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource is a
// source-level invariant proving the DIAG-05 inline guard landed in
// clusterskew.go. It guards against future regressions where the inline guard
// is removed without also removing this test, and provides a deterministic
// RED gate for Task 1 (pre-implementation) that the runtime test alone cannot.
func TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource(t *testing.T) {
	t.Parallel()
	// Source-level invariant: realListNodes MUST reference
	// constants.ExternalLoadBalancerNodeRoleValue. This guards against future
	// regressions where the inline guard is removed without removing this test.
	// The token check uses an "uncommented" filter so prose in header comments
	// (e.g. doc comments mentioning the constant) cannot satisfy the gate.
	data, err := os.ReadFile("clusterskew.go")
	if err != nil {
		t.Fatalf("read clusterskew.go: %v", err)
	}
	// Strip // line comments cheaply by line-prefix; the inline guard is code,
	// not a comment, so an uncommented occurrence must exist.
	nonComment := 0
	for _, line := range strings.Split(string(data), "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "//") {
			continue
		}
		if strings.Contains(trim, "constants.ExternalLoadBalancerNodeRoleValue") {
			nonComment++
		}
	}
	if nonComment == 0 {
		t.Errorf("clusterskew.go does not reference constants.ExternalLoadBalancerNodeRoleValue in non-comment source; DIAG-05 inline guard missing")
	}
}
