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

// TestCatalogCount verifies the SC2 floor: at least 15 entries must ship.
func TestCatalogCount(t *testing.T) {
	if len(Catalog) < 15 {
		t.Errorf("Catalog must have at least 15 entries; got %d", len(Catalog))
	}
}

// TestCatalogAllScopes verifies DIAG-02: every DecodeScope value appears at
// least once in Catalog. Failing this means one of the five required categories
// (kubelet, kubeadm, containerd, docker, addon-startup) has no coverage.
func TestCatalogAllScopes(t *testing.T) {
	required := []DecodeScope{
		ScopeKubelet,
		ScopeKubeadm,
		ScopeContainerd,
		ScopeDocker,
		ScopeAddon,
	}
	present := make(map[DecodeScope]bool)
	for _, p := range Catalog {
		present[p.Scope] = true
	}
	for _, s := range required {
		if !present[s] {
			t.Errorf("scope %q has no entries in Catalog", s)
		}
	}
}

// TestCatalogFieldsPopulated verifies DIAG-03 / SC3: every entry must have a
// non-empty ID, Match, Explanation, and Fix. DocLink may be empty (some
// patterns have no authoritative link; the renderer must tolerate that).
func TestCatalogFieldsPopulated(t *testing.T) {
	for i, p := range Catalog {
		if p.ID == "" {
			t.Errorf("Catalog[%d]: empty ID", i)
		}
		if p.Match == "" {
			t.Errorf("Catalog[%d] (%s): empty Match", i, p.ID)
		}
		if p.Explanation == "" {
			t.Errorf("Catalog[%d] (%s): empty Explanation", i, p.ID)
		}
		if p.Fix == "" {
			t.Errorf("Catalog[%d] (%s): empty Fix", i, p.ID)
		}
	}
}

// TestCatalogIDsUnique verifies that all catalog entry IDs are pairwise unique.
// Duplicate IDs would cause auto-fix targeting confusion in Plan 50-04.
func TestCatalogIDsUnique(t *testing.T) {
	seen := make(map[string]int) // ID -> first index
	for i, p := range Catalog {
		if prev, dup := seen[p.ID]; dup {
			t.Errorf("Catalog[%d]: duplicate ID %q (first seen at index %d)", i, p.ID, prev)
		}
		seen[p.ID] = i
	}
}

// TestCatalog_AutoFixableEntries verifies that exactly four catalog entries have
// AutoFixable=true: KUB-01, KUB-02, KADM-02, KUB-05. KUB-01 and KUB-02 must have
// non-nil AutoFix pointers (parameterless inotify mitigation). KADM-02 and KUB-05
// must have AutoFix=nil (parameterized at runtime by the orchestrator).
// All other entries must have AutoFixable=false and AutoFix=nil.
func TestCatalog_AutoFixableEntries(t *testing.T) {
	autoFixableIDs := map[string]bool{
		"KUB-01":  true,
		"KUB-02":  true,
		"KADM-02": true,
		"KUB-05":  true,
	}
	// KUB-01 and KUB-02 have embedded AutoFix pointers (parameterless inotify factory).
	// KADM-02 and KUB-05 have AutoFix=nil (constructed from ctx at runtime).
	autoFixPointerIDs := map[string]bool{
		"KUB-01": true,
		"KUB-02": true,
	}

	for _, p := range Catalog {
		if autoFixableIDs[p.ID] {
			if !p.AutoFixable {
				t.Errorf("Catalog entry %s: AutoFixable=false, want true", p.ID)
			}
			if autoFixPointerIDs[p.ID] && p.AutoFix == nil {
				t.Errorf("Catalog entry %s: AutoFix=nil, want non-nil pointer (parameterless factory)", p.ID)
			}
			if !autoFixPointerIDs[p.ID] && p.AutoFix != nil {
				t.Errorf("Catalog entry %s: AutoFix non-nil, want nil (parameterized at runtime)", p.ID)
			}
		} else {
			if p.AutoFixable {
				t.Errorf("Catalog entry %s: AutoFixable=true, want false (not in whitelist)", p.ID)
			}
			if p.AutoFix != nil {
				t.Errorf("Catalog entry %s: AutoFix non-nil, want nil (AutoFixable=false)", p.ID)
			}
		}
	}

	// Verify exactly 4 entries have AutoFixable=true.
	count := 0
	for _, p := range Catalog {
		if p.AutoFixable {
			count++
		}
	}
	if count != 4 {
		t.Errorf("expected exactly 4 AutoFixable=true entries, got %d", count)
	}
}

// TestCatalogMatchesKnownLines verifies five concrete fixture lines (one per
// scope) each match exactly one Catalog entry via matchLines.
// Lines taken verbatim from RESEARCH §"Pattern Catalog Seed".
func TestCatalogMatchesKnownLines(t *testing.T) {
	cases := []struct {
		desc        string
		line        string
		wantScope   DecodeScope
		wantMatches int
	}{
		{
			desc:        "KUB-01/KUB-02 inotify — kubelet scope",
			line:        "failed to create fsnotify watcher: too many open files",
			wantScope:   ScopeKubelet,
			wantMatches: 1,
		},
		{
			desc:        "KADM-01 CRI not running — kubeadm scope",
			line:        "[ERROR CRI]: container runtime is not running",
			wantScope:   ScopeKubeadm,
			wantMatches: 1,
		},
		{
			desc:        "CTD-01 image not found — containerd scope",
			line:        `failed to pull image "nginx:bogus": not found`,
			wantScope:   ScopeContainerd,
			wantMatches: 1,
		},
		{
			desc:        "DOCK-01 no space left — docker scope",
			line:        "no space left on device",
			wantScope:   ScopeDocker,
			wantMatches: 1,
		},
		{
			desc:        "ADDON-02 configmap not found — addon scope",
			line:        `MountVolume.SetUp failed for volume "x" : configmap "foo" not found`,
			wantScope:   ScopeAddon,
			wantMatches: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := matchLines([]string{tc.line}, Catalog, "test")
			if len(got) != tc.wantMatches {
				t.Fatalf("line %q: expected %d match(es), got %d", tc.line, tc.wantMatches, len(got))
			}
			if len(got) > 0 && got[0].Pattern.Scope != tc.wantScope {
				t.Errorf("line %q: expected scope %q, got %q", tc.line, tc.wantScope, got[0].Pattern.Scope)
			}
		})
	}
}
