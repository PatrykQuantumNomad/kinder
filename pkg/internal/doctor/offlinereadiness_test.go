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
