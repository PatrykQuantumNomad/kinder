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

//go:build integration

package doctor

import (
	"testing"
	"time"
)

// TestDecodeIntegration_EveryCatalogPatternMatchable asserts that every entry in
// Catalog has at least one representative fixture line that triggers a match.
//
// Each subtest feeds a single injected log line through RunDecode and checks that
// the catalog pattern fires (or that the KUB-01/KUB-02 overlap exception applies).
//
// Orphan check: every Catalog entry must have a fixture — this test fails loudly
// when a new Catalog entry is added without a corresponding fixture line.
// Stale check: no fixture line may reference a non-existent Catalog ID.
//
// Run with: go test -tags integration -race ./pkg/internal/doctor/... -count=1
func TestDecodeIntegration_EveryCatalogPatternMatchable(t *testing.T) {
	// Map of pattern.ID -> fixture line. Update when adding/removing
	// catalog entries; the orphan-check assertion at the bottom will fail
	// loudly if a Catalog entry has no corresponding fixture.
	fixtures := map[string]string{
		"KUB-01": "kubelet[123]: failed to watch /var/log: too many open files",
		"KUB-02": "failed to create fsnotify watcher: too many open files",
		// KUB-03..KUB-05 and all remaining entries deliberately omitted —
		// RED gate: this incomplete map causes the test to fail.
	}

	for _, pat := range Catalog {
		line, ok := fixtures[pat.ID]
		if !ok {
			t.Errorf("Catalog entry %q has no fixture line; add one to the test", pat.ID)
			continue
		}
		pat := pat // capture
		t.Run(pat.ID, func(t *testing.T) {
			origLogs, origEvents := dockerLogsFn, k8sEventsFn
			t.Cleanup(func() { dockerLogsFn = origLogs; k8sEventsFn = origEvents })

			dockerLogsFn = func(_, node, _ string) ([]string, error) {
				if node != "integ-cp" {
					return nil, nil
				}
				return []string{line}, nil
			}
			k8sEventsFn = func(_, _, _ string, _ bool) ([]string, error) {
				return nil, nil
			}

			result, err := RunDecode(DecodeOptions{
				Cluster:    "integ-test",
				BinaryName: "docker",
				CPNodeName: "integ-cp",
				AllNodes:   []string{"integ-cp"},
				Since:      30 * time.Minute,
			})
			if err != nil {
				t.Fatalf("RunDecode: %v", err)
			}

			gotIDs := make([]string, 0, len(result.Matches))
			for _, m := range result.Matches {
				gotIDs = append(gotIDs, m.Pattern.ID)
			}

			if len(result.Matches) == 0 {
				t.Fatalf("expected at least one match for %q with line %q, got none", pat.ID, line)
			}

			found := false
			for _, id := range gotIDs {
				if id == pat.ID {
					found = true
					break
				}
			}
			if !found {
				// Allow KUB-01/KUB-02 cross-match: both patterns fire on
				// "too many open files" lines; KUB-01 (first-match-wins) also
				// covers KUB-02 fixture by covering the same root-cause string.
				if (pat.ID == "KUB-01" || pat.ID == "KUB-02") && containsAny(gotIDs, "KUB-01", "KUB-02") {
					return
				}
				t.Errorf("fixture for %q did not produce a match for %q; got matches: %v", pat.ID, pat.ID, gotIDs)
			}
		})
	}

	// Orphan check: every Catalog entry must have a fixture.
	for _, pat := range Catalog {
		if _, ok := fixtures[pat.ID]; !ok {
			t.Errorf("orphan: Catalog entry %q has no fixture line; update the test", pat.ID)
		}
	}
	// Stale check: no fixture may reference a non-existent Catalog ID.
	catalogIDs := make(map[string]bool, len(Catalog))
	for _, pat := range Catalog {
		catalogIDs[pat.ID] = true
	}
	for id := range fixtures {
		if !catalogIDs[id] {
			t.Errorf("stale fixture: %q is not in Catalog", id)
		}
	}
}

// containsAny returns true when slice contains any of the want values.
func containsAny(slice []string, want ...string) bool {
	for _, s := range slice {
		for _, w := range want {
			if s == w {
				return true
			}
		}
	}
	return false
}
