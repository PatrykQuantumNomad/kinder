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
