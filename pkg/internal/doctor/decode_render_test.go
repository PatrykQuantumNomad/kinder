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
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// fixtureMatch builds a DecodeMatch for tests.
func fixtureMatch(id string, scope DecodeScope, explanation, fix, docLink, source, line string) DecodeMatch {
	return DecodeMatch{
		Source: source,
		Line:   line,
		Pattern: DecodePattern{
			ID:          id,
			Scope:       scope,
			Explanation: explanation,
			Fix:         fix,
			DocLink:     docLink,
		},
	}
}

// TestFormatDecodeHumanReadable_ShowsAllSC3Fields asserts every SC3-required
// field appears in the rendered output for a single match.
func TestFormatDecodeHumanReadable_ShowsAllSC3Fields(t *testing.T) {
	t.Parallel()

	m := fixtureMatch(
		"KUB-01",
		ScopeKubelet,
		"Inotify watch limit exhausted",
		"sudo sysctl fs.inotify.max_user_watches=524288",
		"https://kind.sigs.k8s.io/docs/user/known-issues/",
		"docker-logs:kind-control-plane",
		"failed to create fsnotify watcher: too many open files",
	)
	result := &DecodeResult{
		Cluster:   "kind",
		Matches:   []DecodeMatch{m},
		Unmatched: 5,
	}

	var buf bytes.Buffer
	FormatDecodeHumanReadable(&buf, result)
	out := buf.String()

	for _, want := range []string{
		"KUB-01",
		"kubelet",
		"Inotify watch limit exhausted",
		"sudo sysctl fs.inotify.max_user_watches=524288",
		"https://kind.sigs.k8s.io/docs/user/known-issues/",
		"docker-logs:kind-control-plane",
		"failed to create fsnotify watcher",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

// TestFormatDecodeHumanReadable_HandlesEmptyDocLink ensures a match with no
// DocLink renders without crashing and without printing an empty "Docs:" line.
func TestFormatDecodeHumanReadable_HandlesEmptyDocLink(t *testing.T) {
	t.Parallel()

	m := fixtureMatch("DOCK-01", ScopeDocker, "Disk full", "docker system prune", "", "docker-logs:node1", "no space left on device")
	result := &DecodeResult{Cluster: "kind", Matches: []DecodeMatch{m}}

	var buf bytes.Buffer
	FormatDecodeHumanReadable(&buf, result)
	out := buf.String()

	// Must not contain an empty "Docs:  " or "Docs: " line followed by nothing.
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Docs:" {
			t.Errorf("found empty Docs: line in output:\n%s", out)
		}
	}
	// Must not crash (implicit: test would panic otherwise).
}

// TestFormatDecodeHumanReadable_HeaderAndSummary asserts the output contains a
// header with the cluster name and a summary line with totals.
func TestFormatDecodeHumanReadable_HeaderAndSummary(t *testing.T) {
	t.Parallel()

	m := fixtureMatch("KUB-01", ScopeKubelet, "Inotify exhausted", "sysctl ...", "", "docker-logs:cp", "too many open files")
	result := &DecodeResult{
		Cluster:   "my-cluster",
		Matches:   []DecodeMatch{m},
		Unmatched: 10,
	}

	var buf bytes.Buffer
	FormatDecodeHumanReadable(&buf, result)
	out := buf.String()

	if !strings.Contains(out, "my-cluster") {
		t.Errorf("header missing cluster name %q\noutput:\n%s", "my-cluster", out)
	}
	// Summary should contain both a match count and lines scanned.
	if !strings.Contains(out, "1") {
		t.Errorf("summary missing match count in:\n%s", out)
	}
	// Total lines = matches + unmatched = 11.
	if !strings.Contains(out, "11") {
		t.Errorf("summary missing total lines count (11) in:\n%s", out)
	}
}

// TestFormatDecodeHumanReadable_GroupsByScope verifies that matches sharing a
// scope are grouped under a single scope heading.
func TestFormatDecodeHumanReadable_GroupsByScope(t *testing.T) {
	t.Parallel()

	result := &DecodeResult{
		Cluster: "kind",
		Matches: []DecodeMatch{
			fixtureMatch("KUB-01", ScopeKubelet, "e1", "f1", "", "src1", "line1"),
			fixtureMatch("KUB-02", ScopeKubelet, "e2", "f2", "", "src2", "line2"),
			fixtureMatch("DOCK-01", ScopeDocker, "e3", "f3", "", "src3", "line3"),
		},
	}

	var buf bytes.Buffer
	FormatDecodeHumanReadable(&buf, result)
	out := buf.String()

	// "kubelet" heading appears exactly once.
	kubeletCount := strings.Count(out, "kubelet")
	if kubeletCount != 1 {
		t.Errorf("expected kubelet heading to appear 1 time, got %d\noutput:\n%s", kubeletCount, out)
	}
	// "docker" heading appears exactly once.
	dockerCount := strings.Count(out, "docker")
	if dockerCount != 1 {
		t.Errorf("expected docker heading to appear 1 time, got %d\noutput:\n%s", dockerCount, out)
	}
}

// TestFormatDecodeJSON_ShapeMatchesResult asserts the JSON envelope has the
// required top-level keys and that all SC3 fields are present per match.
func TestFormatDecodeJSON_ShapeMatchesResult(t *testing.T) {
	t.Parallel()

	m := fixtureMatch(
		"KUB-01", ScopeKubelet,
		"Inotify exhausted",
		"sudo sysctl ...",
		"https://kind.sigs.k8s.io/",
		"docker-logs:kind-control-plane",
		"too many open files",
	)
	result := &DecodeResult{
		Cluster:   "kind",
		Matches:   []DecodeMatch{m},
		Unmatched: 7,
	}

	envelope := FormatDecodeJSON(result)

	// Serialize + deserialize to verify JSON-marshallable shape.
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	// Top-level keys.
	for _, key := range []string{"cluster", "matches", "unmatched", "summary"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("envelope missing top-level key %q; keys: %v", key, keysOf(decoded))
		}
	}

	// matches is a non-empty array.
	matchesRaw, ok := decoded["matches"].([]interface{})
	if !ok || len(matchesRaw) != 1 {
		t.Fatalf("matches: want []interface{} len=1, got %T %v", decoded["matches"], decoded["matches"])
	}

	// Each match has all SC3 fields.
	firstMatch, ok := matchesRaw[0].(map[string]interface{})
	if !ok {
		t.Fatalf("matches[0] is not a map: %T", matchesRaw[0])
	}
	for _, key := range []string{"pattern_id", "scope", "explanation", "fix", "doc_link", "source", "line"} {
		if _, ok := firstMatch[key]; !ok {
			t.Errorf("match missing key %q; keys: %v", key, keysOf(firstMatch))
		}
	}

	// summary has total_matches.
	summary, ok := decoded["summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("summary is not a map: %T", decoded["summary"])
	}
	if _, ok := summary["total_matches"]; !ok {
		t.Errorf("summary missing total_matches; keys: %v", keysOf(summary))
	}
	if _, ok := summary["total_lines"]; !ok {
		t.Errorf("summary missing total_lines; keys: %v", keysOf(summary))
	}
	if _, ok := summary["by_scope"]; !ok {
		t.Errorf("summary missing by_scope; keys: %v", keysOf(summary))
	}
}

// TestFormatDecodeHumanReadable_NoMatchesMessage checks empty-result messages.
func TestFormatDecodeHumanReadable_NoMatchesMessage(t *testing.T) {
	t.Parallel()

	// Case 1: no matches but some unmatched lines.
	result1 := &DecodeResult{Cluster: "kind", Matches: nil, Unmatched: 42}
	var buf1 bytes.Buffer
	FormatDecodeHumanReadable(&buf1, result1)
	out1 := buf1.String()
	if !strings.Contains(out1, "No known patterns matched") {
		t.Errorf("case1: expected 'No known patterns matched' in:\n%s", out1)
	}
	if !strings.Contains(out1, "42") {
		t.Errorf("case1: expected line count 42 in:\n%s", out1)
	}

	// Case 2: empty cluster, no matches, no unmatched.
	result2 := &DecodeResult{Cluster: "", Matches: nil, Unmatched: 0}
	var buf2 bytes.Buffer
	FormatDecodeHumanReadable(&buf2, result2)
	out2 := buf2.String()
	if !strings.Contains(out2, "No logs or events to scan") {
		t.Errorf("case2: expected 'No logs or events to scan' in:\n%s", out2)
	}
}

// keysOf returns the keys of a map[string]interface{} for test error messages.
func keysOf(m map[string]interface{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
