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
	"testing"
)

// localPatterns is a test-local pattern slice; intentionally NOT referencing
// the global Catalog so Task 1 tests remain independent of Task 2 catalog churn.
var localPatterns = []DecodePattern{
	{
		ID:          "TEST-01",
		Scope:       ScopeKubelet,
		Match:       "too many open files",
		Explanation: "Inotify watch limit exhausted",
		Fix:         "sudo sysctl fs.inotify.max_user_watches=524288",
		DocLink:     "https://kind.sigs.k8s.io/docs/user/known-issues/",
	},
	{
		ID:          "TEST-02",
		Scope:       ScopeKubelet,
		Match:       "regex:error adding pid \\d+ to cgroups",
		Explanation: "Cgroup v2 hierarchy conflict",
		Fix:         "Wait for node readiness",
		DocLink:     "",
	},
}

func TestMatchLines_SubstringHit(t *testing.T) {
	lines := []string{"failed to create fsnotify watcher: too many open files"}
	got := matchLines(lines, localPatterns[:1], "fixture")
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].Source != "fixture" {
		t.Errorf("expected Source=fixture, got %q", got[0].Source)
	}
	if got[0].Line != lines[0] {
		t.Errorf("expected Line=%q, got %q", lines[0], got[0].Line)
	}
	if got[0].Pattern.ID != "TEST-01" {
		t.Errorf("expected Pattern.ID=TEST-01, got %q", got[0].Pattern.ID)
	}
}

func TestMatchLines_NoMatch(t *testing.T) {
	lines := []string{"normal log output"}
	got := matchLines(lines, localPatterns[:1], "fixture")
	if len(got) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(got))
	}
}

func TestMatchLines_RegexHit(t *testing.T) {
	lines := []string{"error adding pid 1234 to cgroups"}
	got := matchLines(lines, localPatterns[1:2], "fixture")
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0].Pattern.ID != "TEST-02" {
		t.Errorf("expected Pattern.ID=TEST-02, got %q", got[0].Pattern.ID)
	}
}

func TestMatchLines_FirstMatchWins(t *testing.T) {
	// Line contains substring from both TEST-01 and TEST-02 won't apply here,
	// but we test two overlapping substring patterns where the first wins.
	overlapPatterns := []DecodePattern{
		{
			ID:          "FIRST-01",
			Scope:       ScopeKubelet,
			Match:       "too many open files",
			Explanation: "First pattern",
			Fix:         "fix1",
		},
		{
			ID:          "SECOND-01",
			Scope:       ScopeKubelet,
			Match:       "open files",
			Explanation: "Second pattern — subset of FIRST-01 match",
			Fix:         "fix2",
		},
	}
	lines := []string{"failed to create fsnotify watcher: too many open files"}
	got := matchLines(lines, overlapPatterns, "fixture")
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 match (first-match-wins), got %d", len(got))
	}
	if got[0].Pattern.ID != "FIRST-01" {
		t.Errorf("expected FIRST-01 to win, got %q", got[0].Pattern.ID)
	}
}

func TestMatchLines_PreservesAllFields(t *testing.T) {
	lines := []string{"failed to create fsnotify watcher: too many open files"}
	got := matchLines(lines, localPatterns[:1], "fixture")
	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	p := got[0].Pattern
	if p.ID != "TEST-01" {
		t.Errorf("ID mismatch: got %q", p.ID)
	}
	if p.Scope != ScopeKubelet {
		t.Errorf("Scope mismatch: got %q", p.Scope)
	}
	if p.Match != "too many open files" {
		t.Errorf("Match mismatch: got %q", p.Match)
	}
	if p.Explanation == "" {
		t.Error("Explanation must be non-empty")
	}
	if p.Fix == "" {
		t.Error("Fix must be non-empty")
	}
	if p.DocLink == "" {
		t.Error("DocLink must be non-empty for TEST-01")
	}
}

func TestMatchLines_EmptyInputs(t *testing.T) {
	// nil lines
	got := matchLines(nil, localPatterns, "fixture")
	if got == nil {
		t.Fatal("matchLines(nil lines) must return non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 matches for nil lines, got %d", len(got))
	}

	// empty lines
	got = matchLines([]string{}, localPatterns, "fixture")
	if got == nil {
		t.Fatal("matchLines(empty lines) must return non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 matches for empty lines, got %d", len(got))
	}

	// nil patterns
	got = matchLines([]string{"some line"}, nil, "fixture")
	if got == nil {
		t.Fatal("matchLines(nil patterns) must return non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 matches for nil patterns, got %d", len(got))
	}

	// empty patterns
	got = matchLines([]string{"some line"}, []DecodePattern{}, "fixture")
	if got == nil {
		t.Fatal("matchLines(empty patterns) must return non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 matches for empty patterns, got %d", len(got))
	}
}
