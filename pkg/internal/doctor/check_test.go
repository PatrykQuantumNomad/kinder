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
	"runtime"
	"testing"
)

// mockCheck is a test-only Check implementation.
type mockCheck struct {
	name      string
	category  string
	platforms []string
	results   []Result
}

func (m *mockCheck) Name() string        { return m.name }
func (m *mockCheck) Category() string     { return m.category }
func (m *mockCheck) Platforms() []string  { return m.platforms }
func (m *mockCheck) Run() []Result        { return m.results }

func TestAllChecks_ReturnsNonNilSlice(t *testing.T) {
	t.Parallel()
	checks := AllChecks()
	if checks == nil {
		t.Fatal("AllChecks() returned nil, expected non-nil slice")
	}
}

func TestRunAllChecks_PlatformSkip(t *testing.T) {
	t.Parallel()

	// Determine a platform that is NOT the current OS.
	nonCurrentPlatform := "linux"
	if runtime.GOOS == "linux" {
		nonCurrentPlatform = "windows"
	}

	original := allChecks
	defer func() { allChecks = original }()

	allChecks = []Check{
		&mockCheck{
			name:      "platform-specific",
			category:  "Test",
			platforms: []string{nonCurrentPlatform},
			results: []Result{{
				Name:     "platform-specific",
				Category: "Test",
				Status:   "ok",
				Message:  "should not appear",
			}},
		},
	}

	results := RunAllChecks()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "skip" {
		t.Errorf("expected skip status, got %q", results[0].Status)
	}
	if results[0].Name != "platform-specific" {
		t.Errorf("expected name 'platform-specific', got %q", results[0].Name)
	}
}

func TestRunAllChecks_NilPlatformsRunsOnAll(t *testing.T) {
	t.Parallel()

	original := allChecks
	defer func() { allChecks = original }()

	allChecks = []Check{
		&mockCheck{
			name:      "universal",
			category:  "Test",
			platforms: nil,
			results: []Result{{
				Name:     "universal",
				Category: "Test",
				Status:   "ok",
				Message:  "runs everywhere",
			}},
		},
	}

	results := RunAllChecks()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "ok" {
		t.Errorf("expected ok status, got %q", results[0].Status)
	}
	if results[0].Message != "runs everywhere" {
		t.Errorf("expected message 'runs everywhere', got %q", results[0].Message)
	}
}

func TestRunAllChecks_MultipleResultsPreserved(t *testing.T) {
	t.Parallel()

	original := allChecks
	defer func() { allChecks = original }()

	allChecks = []Check{
		&mockCheck{
			name:     "multi-check",
			category: "Test",
			results: []Result{
				{Name: "sub-a", Category: "Test", Status: "ok", Message: "first"},
				{Name: "sub-b", Category: "Test", Status: "warn", Message: "second"},
				{Name: "sub-c", Category: "Test", Status: "fail", Message: "third"},
			},
		},
	}

	results := RunAllChecks()
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Name != "sub-a" || results[1].Name != "sub-b" || results[2].Name != "sub-c" {
		t.Errorf("results not preserved in order: %v", results)
	}
}

func TestExitCodeFromResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		results  []Result
		expected int
	}{
		{
			name:     "all ok",
			results:  []Result{{Status: "ok"}, {Status: "ok"}},
			expected: 0,
		},
		{
			name:     "all skip",
			results:  []Result{{Status: "skip"}, {Status: "skip"}},
			expected: 0,
		},
		{
			name:     "mix ok and skip",
			results:  []Result{{Status: "ok"}, {Status: "skip"}},
			expected: 0,
		},
		{
			name:     "any fail",
			results:  []Result{{Status: "ok"}, {Status: "fail"}, {Status: "warn"}},
			expected: 1,
		},
		{
			name:     "any warn no fail",
			results:  []Result{{Status: "ok"}, {Status: "warn"}},
			expected: 2,
		},
		{
			name:     "fail takes priority over warn",
			results:  []Result{{Status: "warn"}, {Status: "fail"}},
			expected: 1,
		},
		{
			name:     "empty results",
			results:  nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExitCodeFromResults(tt.results)
			if got != tt.expected {
				t.Errorf("ExitCodeFromResults() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestPlatformSkipMessage_SinglePlatform(t *testing.T) {
	t.Parallel()
	got := platformSkipMessage([]string{"linux"})
	expected := "(linux only)"
	if got != expected {
		t.Errorf("platformSkipMessage([linux]) = %q, want %q", got, expected)
	}
}

func TestPlatformSkipMessage_MultiplePlatforms(t *testing.T) {
	t.Parallel()
	got := platformSkipMessage([]string{"linux", "darwin"})
	expected := "(linux/darwin only)"
	if got != expected {
		t.Errorf("platformSkipMessage([linux,darwin]) = %q, want %q", got, expected)
	}
}
