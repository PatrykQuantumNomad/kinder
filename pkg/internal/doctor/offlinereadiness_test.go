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
	"fmt"
	"strings"
	"testing"
)

// TestOfflineReadiness_AllPresent verifies that the check returns a single "ok"
// result when all addon images are present locally.
func TestOfflineReadiness_AllPresent(t *testing.T) {
	t.Parallel()
	check := &offlineReadinessCheck{
		inspectImage: func(image string) bool { return true },
		lookPath:     func(s string) (string, error) { return "/usr/bin/" + s, nil },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	r := results[0]
	if r.Status != "ok" {
		t.Errorf("Status = %q, want %q; Message=%q", r.Status, "ok", r.Message)
	}
	if !strings.Contains(r.Message, "all") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "all")
	}
}

// TestOfflineReadiness_SomeAbsent verifies that the check returns a "warn" result
// containing addon name labels when some images are absent.
func TestOfflineReadiness_SomeAbsent(t *testing.T) {
	t.Parallel()
	// Return false only for MetalLB images.
	check := &offlineReadinessCheck{
		inspectImage: func(image string) bool {
			return !strings.Contains(image, "metallb")
		},
		lookPath: func(s string) (string, error) { return "/usr/bin/" + s, nil },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q; Message=%q", r.Status, "warn", r.Message)
	}
	if !strings.Contains(r.Message, "MetalLB") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "MetalLB")
	}
	// Should not mention addons whose images are all present.
	if strings.Contains(r.Message, "Dashboard") {
		t.Errorf("Message = %q, should not contain %q (Dashboard images are present)", r.Message, "Dashboard")
	}
}

// TestOfflineReadiness_AllAbsent verifies that when every addon image is missing
// the warn message reports the full count.
func TestOfflineReadiness_AllAbsent(t *testing.T) {
	t.Parallel()
	check := &offlineReadinessCheck{
		inspectImage: func(image string) bool { return false },
		lookPath:     func(s string) (string, error) { return "/usr/bin/" + s, nil },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q; Message=%q", r.Status, "warn", r.Message)
	}
	// Message should mention the total count of addon images.
	totalStr := fmt.Sprintf("%d of %d", len(allAddonImages), len(allAddonImages))
	if !strings.Contains(r.Message, totalStr) {
		t.Errorf("Message = %q, want to contain %q", r.Message, totalStr)
	}
}

// TestOfflineReadiness_NoRuntime verifies that the check skips gracefully when
// no container runtime binary is available.
func TestOfflineReadiness_NoRuntime(t *testing.T) {
	t.Parallel()
	check := &offlineReadinessCheck{
		inspectImage: func(image string) bool { return false },
		lookPath:     func(s string) (string, error) { return "", fmt.Errorf("not found: %s", s) },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(results), results)
	}
	r := results[0]
	if r.Status != "skip" {
		t.Errorf("Status = %q, want %q; Message=%q", r.Status, "skip", r.Message)
	}
	if !strings.Contains(r.Message, "no container runtime") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "no container runtime")
	}
}

// TestAllAddonImages_CountMatchesExpected verifies the canonical image list has
// exactly 14 entries (one per known addon image across all addons).
func TestAllAddonImages_CountMatchesExpected(t *testing.T) {
	t.Parallel()
	const expected = 14
	if len(allAddonImages) != expected {
		t.Errorf("len(allAddonImages) = %d, want %d", len(allAddonImages), expected)
	}
}

// TestOfflineReadinessIncludesEnvoyImage verifies that the offline readiness check
// lists the Envoy image (not HAProxy) for the Load Balancer (HA) addon.
func TestOfflineReadinessIncludesEnvoyImage(t *testing.T) {
	t.Parallel()
	const envoyImage = "docker.io/envoyproxy/envoy:v1.36.2"
	foundEnvoy := false
	for _, entry := range allAddonImages {
		if strings.Contains(entry.Image, "kindest/haproxy") {
			t.Errorf("allAddonImages contains kindest/haproxy image %q; expected Envoy LB image instead", entry.Image)
		}
		if entry.Image == envoyImage {
			foundEnvoy = true
		}
	}
	if !foundEnvoy {
		t.Errorf("allAddonImages does not contain Envoy LB image %q", envoyImage)
	}
}

// TestAllAddonImages_TagsMatchActions verifies that allAddonImages mirrors the
// delivered image tags from each addon's install action (the authoritative source of
// truth).  This test is the phase-53 consolidation gate: it will fail (RED) until
// offlinereadiness.go is updated to reflect all Phase 53 bumps, and will pass (GREEN)
// once the bumped tags are in place.
//
// Expected delivered state after Phase 53:
//   - local-path-provisioner: v0.0.36 (53-01)
//   - Headlamp: v0.42.0 (53-02 Path A)
//   - cert-manager: v1.20.2 (53-03 Path A)
//   - Envoy Gateway: v1.7.2 + ratelimit:05c08d03 (53-04 Path A)
//   - MetalLB: v0.15.3 (53-05 held)
//   - Metrics Server: v0.8.1 (53-06 held)
func TestAllAddonImages_TagsMatchActions(t *testing.T) {
	t.Parallel()

	// requiredImages is the canonical set of tags that allAddonImages MUST contain
	// after Phase 53.  Each entry maps to the deliverable from the corresponding plan.
	requiredImages := []struct {
		image string
		plan  string
	}{
		// 53-01: local-path-provisioner bumped v0.0.35 → v0.0.36
		{"docker.io/rancher/local-path-provisioner:v0.0.36", "53-01"},
		// 53-02: Headlamp bumped v0.40.1 → v0.42.0
		{"ghcr.io/headlamp-k8s/headlamp:v0.42.0", "53-02"},
		// 53-03: cert-manager bumped v1.16.3 → v1.20.2 (all three components)
		{"quay.io/jetstack/cert-manager-cainjector:v1.20.2", "53-03"},
		{"quay.io/jetstack/cert-manager-controller:v1.20.2", "53-03"},
		{"quay.io/jetstack/cert-manager-webhook:v1.20.2", "53-03"},
		// 53-04: Envoy Gateway bumped v1.3.1 → v1.7.2; ratelimit ae4cee11 → 05c08d03
		{"envoyproxy/gateway:v1.7.2", "53-04"},
		{"docker.io/envoyproxy/ratelimit:05c08d03", "53-04"},
		// 53-05: MetalLB held at v0.15.3
		{"quay.io/metallb/controller:v0.15.3", "53-05"},
		{"quay.io/metallb/speaker:v0.15.3", "53-05"},
		// 53-06: Metrics Server held at v0.8.1
		{"registry.k8s.io/metrics-server/metrics-server:v0.8.1", "53-06"},
	}

	// Build a set of all images in allAddonImages for O(1) lookup.
	present := make(map[string]bool, len(allAddonImages))
	for _, entry := range allAddonImages {
		present[entry.Image] = true
	}

	for _, want := range requiredImages {
		if !present[want.image] {
			t.Errorf("allAddonImages is missing %q (required by plan %s)", want.image, want.plan)
		}
	}

	// Also verify that old tags for bumped addons are NOT present (stale entries would
	// cause kinder doctor offline-readiness to check against outdated image refs).
	staleImages := []struct {
		image string
		plan  string
	}{
		{"docker.io/rancher/local-path-provisioner:v0.0.35", "53-01 old tag"},
		{"ghcr.io/headlamp-k8s/headlamp:v0.40.1", "53-02 old tag"},
		{"quay.io/jetstack/cert-manager-cainjector:v1.16.3", "53-03 old tag"},
		{"quay.io/jetstack/cert-manager-controller:v1.16.3", "53-03 old tag"},
		{"quay.io/jetstack/cert-manager-webhook:v1.16.3", "53-03 old tag"},
		{"envoyproxy/gateway:v1.3.1", "53-04 old tag"},
		{"docker.io/envoyproxy/ratelimit:ae4cee11", "53-04 old tag"},
	}

	for _, stale := range staleImages {
		if present[stale.image] {
			t.Errorf("allAddonImages still contains stale tag %q (%s — should have been bumped in Phase 53)", stale.image, stale.plan)
		}
	}
}
