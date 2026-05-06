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

package snapshot

import (
	"strings"
	"testing"
)

// TestReconstructSingleCP: 1 CP, 0 workers, no LB → 1 node entry.
func TestReconstructSingleCP(t *testing.T) {
	topo := TopologyInfo{ControlPlaneCount: 1, WorkerCount: 0, HasLoadBalancer: false}
	out, err := ReconstructKindConfig(topo, "v1.31.2", "kindest/node:v1.31.2", nil)
	if err != nil {
		t.Fatalf("ReconstructKindConfig returned error: %v", err)
	}
	yaml := string(out)

	// Should have exactly 1 control-plane entry
	count := strings.Count(yaml, "role: control-plane")
	if count != 1 {
		t.Errorf("expected 1 control-plane role entry, got %d; yaml:\n%s", count, yaml)
	}
	// Should have 0 worker entries
	workerCount := strings.Count(yaml, "role: worker")
	if workerCount != 0 {
		t.Errorf("expected 0 worker role entries, got %d; yaml:\n%s", workerCount, yaml)
	}
	// Should contain the node image
	if !strings.Contains(yaml, "kindest/node:v1.31.2") {
		t.Errorf("expected node image in output; yaml:\n%s", yaml)
	}
	// Should contain kind/apiVersion
	if !strings.Contains(yaml, "kind: Cluster") {
		t.Errorf("expected 'kind: Cluster' in output; yaml:\n%s", yaml)
	}
	if !strings.Contains(yaml, "apiVersion: kind.x-k8s.io/v1alpha4") {
		t.Errorf("expected apiVersion in output; yaml:\n%s", yaml)
	}
}

// TestReconstructHA: 3 CP + 2 workers + LB → 5 node entries (LB implicit via comment).
func TestReconstructHA(t *testing.T) {
	topo := TopologyInfo{ControlPlaneCount: 3, WorkerCount: 2, HasLoadBalancer: true}
	out, err := ReconstructKindConfig(topo, "v1.31.2", "kindest/node:v1.31.2", nil)
	if err != nil {
		t.Fatalf("ReconstructKindConfig returned error: %v", err)
	}
	yaml := string(out)

	// 3 CP + 2 workers = 5 node entries
	cpCount := strings.Count(yaml, "role: control-plane")
	workerCount := strings.Count(yaml, "role: worker")
	if cpCount != 3 {
		t.Errorf("expected 3 control-plane entries, got %d; yaml:\n%s", cpCount, yaml)
	}
	if workerCount != 2 {
		t.Errorf("expected 2 worker entries, got %d; yaml:\n%s", workerCount, yaml)
	}
	// LB should NOT have an explicit role: load-balancer entry (kind v1alpha4 doesn't have it)
	if strings.Contains(yaml, "role: load-balancer") {
		t.Errorf("unexpected 'role: load-balancer' in output (not a valid kind v1alpha4 role); yaml:\n%s", yaml)
	}
	// But LB topology should be documented via comment
	if !strings.Contains(yaml, "load balancer") && !strings.Contains(yaml, "load-balancer") {
		t.Errorf("expected load balancer mention in comment for HA topology; yaml:\n%s", yaml)
	}
}

// TestReconstructHasNotice: output contains the "NOT included" comment.
func TestReconstructHasNotice(t *testing.T) {
	topo := TopologyInfo{ControlPlaneCount: 1, WorkerCount: 0, HasLoadBalancer: false}
	out, err := ReconstructKindConfig(topo, "v1.31.2", "kindest/node:v1.31.2", nil)
	if err != nil {
		t.Fatalf("ReconstructKindConfig returned error: %v", err)
	}
	yaml := string(out)

	if !strings.Contains(yaml, "NOT") || !strings.Contains(yaml, "included") {
		t.Errorf("expected 'NOT included' notice in output; yaml:\n%s", yaml)
	}
}
