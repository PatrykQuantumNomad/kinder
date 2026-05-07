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
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestDecodeRender_AllScopes_SC3FieldsPresent verifies that FormatDecodeHumanReadable
// and FormatDecodeJSON include all four SC3-mandated fields for each match:
//   - Pattern ID (e.g. "KUB-01")
//   - Explanation (plain-English description)
//   - Fix (remediation one-liner)
//   - DocLink (optional URL — checked only when non-empty)
//
// The test builds a synthetic DecodeResult with one match per scope (kubelet,
// kubeadm, containerd, docker, addon) so the renderer is exercised across the
// full scope surface.
//
// Run with: go test -tags integration -race ./pkg/internal/doctor/... -count=1
func TestDecodeRender_AllScopes_SC3FieldsPresent(t *testing.T) {
	result := &DecodeResult{
		Cluster: "integ-test",
		Matches: []DecodeMatch{
			{
				Source: "docker-logs:cp",
				Line:   "fixture line 1",
				Pattern: DecodePattern{
					ID:          "KUB-01",
					Scope:       ScopeKubelet,
					Explanation: "exp-kub",
					Fix:         "fix-kub",
					DocLink:     "https://example.com/kub",
				},
			},
			{
				Source: "docker-logs:cp",
				Line:   "fixture line 2",
				Pattern: DecodePattern{
					ID:          "KADM-02",
					Scope:       ScopeKubeadm,
					Explanation: "exp-kadm",
					Fix:         "fix-kadm",
					DocLink:     "", // deliberately empty to test omission is correct
				},
			},
			{
				Source: "k8s-events",
				Line:   "fixture line 3",
				Pattern: DecodePattern{
					ID:          "CTD-01",
					Scope:       ScopeContainerd,
					Explanation: "exp-ctd",
					Fix:         "fix-ctd",
					DocLink:     "https://example.com/ctd",
				},
			},
			{
				Source: "docker-logs:cp",
				Line:   "fixture line 4",
				Pattern: DecodePattern{
					ID:          "DOCK-01",
					Scope:       ScopeDocker,
					Explanation: "exp-dock",
					Fix:         "fix-dock",
					DocLink:     "https://example.com/dock",
				},
			},
			{
				Source: "k8s-events",
				Line:   "fixture line 5",
				Pattern: DecodePattern{
					ID:          "ADDON-01",
					Scope:       ScopeAddon,
					Explanation: "exp-addon",
					Fix:         "fix-addon",
					DocLink:     "https://example.com/addon",
				},
			},
		},
		Unmatched: 7,
	}

	// -------------------------------------------------------------------------
	// Human-readable output: assert all SC3 fields appear for each match.
	// -------------------------------------------------------------------------
	var humanBuf bytes.Buffer
	FormatDecodeHumanReadable(&humanBuf, result)
	humanOut := humanBuf.String()

	for _, m := range result.Matches {
		if !strings.Contains(humanOut, m.Pattern.ID) {
			t.Errorf("human output missing pattern ID %q", m.Pattern.ID)
		}
		if !strings.Contains(humanOut, m.Pattern.Explanation) {
			t.Errorf("human output missing explanation for %q", m.Pattern.ID)
		}
		if !strings.Contains(humanOut, m.Pattern.Fix) {
			t.Errorf("human output missing fix for %q", m.Pattern.ID)
		}
		if m.Pattern.DocLink != "" && !strings.Contains(humanOut, m.Pattern.DocLink) {
			t.Errorf("human output missing doc link for %q", m.Pattern.ID)
		}
	}

	// -------------------------------------------------------------------------
	// JSON output: assert all SC3 fields are present on each match object.
	// -------------------------------------------------------------------------
	envelope := FormatDecodeJSON(result)
	b, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	rawMatches, ok := parsed["matches"].([]interface{})
	if !ok || len(rawMatches) != 5 {
		t.Fatalf("JSON envelope matches: want 5, got %d (value=%v)", len(rawMatches), parsed["matches"])
	}
	requiredKeys := []string{"pattern_id", "scope", "explanation", "fix", "source", "line"}
	for i, raw := range rawMatches {
		m, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("match %d is not a JSON object: %T", i, raw)
		}
		for _, key := range requiredKeys {
			if _, has := m[key]; !has {
				t.Errorf("match %d JSON missing key %q (got keys: %v)", i, key, mapKeys(m))
			}
		}
		// doc_link key must be present (may be empty string for KADM-02).
		if _, has := m["doc_link"]; !has {
			t.Errorf("match %d JSON missing key \"doc_link\" (got keys: %v)", i, mapKeys(m))
		}
	}
}

// mapKeys returns the key names of a map for diagnostic messages.
// Named mapKeys to avoid collision with the keysOf helper in decode_render_test.go.
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
