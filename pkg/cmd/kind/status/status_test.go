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

package status

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestStatusJSON_SchemaShape verifies that the JSON renderer produces a top-level
// object with cluster name and a nodes array of {name, role, state, version} objects.
func TestStatusJSON_SchemaShape(t *testing.T) {
	infos := []nodeStatusInfo{
		{Name: "kind-control-plane", Role: "control-plane", State: "running", Version: "v1.31.2"},
		{Name: "kind-worker", Role: "worker", State: "exited", Version: "v1.31.2"},
	}
	var buf bytes.Buffer
	if err := renderJSON(&buf, "kind", infos); err != nil {
		t.Fatalf("renderJSON returned error: %v", err)
	}

	// Decode into a generic map to validate the wire shape, not just the struct.
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	if got["cluster"] != "kind" {
		t.Errorf("cluster field = %v, want %q", got["cluster"], "kind")
	}
	rawNodes, ok := got["nodes"].([]interface{})
	if !ok {
		t.Fatalf("nodes field is not an array: %T", got["nodes"])
	}
	if len(rawNodes) != 2 {
		t.Fatalf("nodes count = %d, want 2", len(rawNodes))
	}
	first, ok := rawNodes[0].(map[string]interface{})
	if !ok {
		t.Fatalf("nodes[0] is not an object: %T", rawNodes[0])
	}
	for _, key := range []string{"name", "role", "state", "version"} {
		if _, present := first[key]; !present {
			t.Errorf("nodes[0] missing required field %q (got keys: %v)", key, mapKeys(first))
		}
	}
}

// TestStatusText_TabwriterColumns verifies the text renderer emits a header
// row with NAME, ROLE, STATE, K8S-VERSION columns.
func TestStatusText_TabwriterColumns(t *testing.T) {
	infos := []nodeStatusInfo{
		{Name: "kind-control-plane", Role: "control-plane", State: "running", Version: "v1.31.2"},
	}
	var buf bytes.Buffer
	if err := renderText(&buf, infos); err != nil {
		t.Fatalf("renderText returned error: %v", err)
	}
	out := buf.String()
	for _, col := range []string{"NAME", "ROLE", "STATE", "K8S-VERSION"} {
		if !strings.Contains(out, col) {
			t.Errorf("text output missing %q header column. Output:\n%s", col, out)
		}
	}
	// And the data row should appear.
	if !strings.Contains(out, "kind-control-plane") {
		t.Errorf("text output missing node name. Output:\n%s", out)
	}
	if !strings.Contains(out, "running") {
		t.Errorf("text output missing state value. Output:\n%s", out)
	}
}

// mapKeys returns the keys of a map[string]interface{} as a slice for diagnostics.
func mapKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
