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
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// CheckCompatibility tests
// ---------------------------------------------------------------------------

func TestCompatHappyPath(t *testing.T) {
	snap := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1, WorkerCount: 2, HasLoadBalancer: false},
		AddonVersions: map[string]string{"certManager": "1.12.0"},
	}
	live := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1, WorkerCount: 2, HasLoadBalancer: false},
		AddonVersions: map[string]string{"certManager": "1.12.0"},
	}
	if err := CheckCompatibility(snap, live); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestCompatK8sMismatch(t *testing.T) {
	snap := &Metadata{
		K8sVersion:    "v1.32.0",
		Topology:      TopologyInfo{ControlPlaneCount: 1},
		AddonVersions: map[string]string{},
	}
	live := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1},
		AddonVersions: map[string]string{},
	}
	err := CheckCompatibility(snap, live)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrCompatK8sMismatch) {
		t.Errorf("expected ErrCompatK8sMismatch, got: %v", err)
	}
}

func TestCompatTopologyMismatch(t *testing.T) {
	snap := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 3, WorkerCount: 0, HasLoadBalancer: true},
		AddonVersions: map[string]string{},
	}
	live := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1, WorkerCount: 2, HasLoadBalancer: false},
		AddonVersions: map[string]string{},
	}
	err := CheckCompatibility(snap, live)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrCompatTopologyMismatch) {
		t.Errorf("expected ErrCompatTopologyMismatch, got: %v", err)
	}
}

func TestCompatAddonExtra(t *testing.T) {
	snap := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1},
		AddonVersions: map[string]string{},
	}
	live := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1},
		AddonVersions: map[string]string{"metalLB": "v0.14.3"},
	}
	err := CheckCompatibility(snap, live)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrCompatAddonMismatch) {
		t.Errorf("expected ErrCompatAddonMismatch, got: %v", err)
	}
}

func TestCompatAddonVersionDrift(t *testing.T) {
	snap := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1},
		AddonVersions: map[string]string{"certManager": "1.12.0"},
	}
	live := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1},
		AddonVersions: map[string]string{"certManager": "1.13.0"},
	}
	err := CheckCompatibility(snap, live)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrCompatAddonMismatch) {
		t.Errorf("expected ErrCompatAddonMismatch, got: %v", err)
	}
}

func TestCompatMultipleViolations(t *testing.T) {
	snap := &Metadata{
		K8sVersion:    "v1.32.0",
		Topology:      TopologyInfo{ControlPlaneCount: 3, WorkerCount: 0, HasLoadBalancer: true},
		AddonVersions: map[string]string{"certManager": "1.12.0"},
	}
	live := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1, WorkerCount: 2, HasLoadBalancer: false},
		AddonVersions: map[string]string{"certManager": "1.13.0"},
	}
	err := CheckCompatibility(snap, live)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrCompatK8sMismatch) {
		t.Errorf("expected ErrCompatK8sMismatch in aggregate, got: %v", err)
	}
	if !errors.Is(err, ErrCompatTopologyMismatch) {
		t.Errorf("expected ErrCompatTopologyMismatch in aggregate, got: %v", err)
	}
	if !errors.Is(err, ErrCompatAddonMismatch) {
		t.Errorf("expected ErrCompatAddonMismatch in aggregate, got: %v", err)
	}
}

func TestCompatEmptyAddonsEqualNil(t *testing.T) {
	snap := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1},
		AddonVersions: nil,
	}
	live := &Metadata{
		K8sVersion:    "v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1},
		AddonVersions: map[string]string{},
	}
	if err := CheckCompatibility(snap, live); err != nil {
		t.Fatalf("expected nil for nil vs empty map addons, got: %v", err)
	}
}
