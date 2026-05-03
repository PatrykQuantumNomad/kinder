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

package clusters

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestClustersJSON_NewSchema verifies that --output json emits an array of
// {name, status} objects, NOT a bare array of cluster-name strings.
//
// This is the schema migration documented in the phase 47 CONTEXT.md
// (D-04, "Cluster list integration: add Status column"), and is an intentional
// breaking change accepted by the user. See pitfall #6 in 47-RESEARCH.md.
func TestClustersJSON_NewSchema(t *testing.T) {
	infos := []clusterInfo{
		{Name: "kind", Status: "Running"},
		{Name: "dev", Status: "Paused"},
	}
	var buf bytes.Buffer
	if err := renderJSON(&buf, infos); err != nil {
		t.Fatalf("renderJSON error: %v", err)
	}

	// Decode generically so we assert the wire shape, not just the struct.
	var got []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not a JSON array of objects: %v\n%s", err, buf.String())
	}
	if len(got) != 2 {
		t.Fatalf("array len = %d, want 2", len(got))
	}
	for i, item := range got {
		if _, present := item["name"]; !present {
			t.Errorf("item[%d] missing required field %q (got keys: %v)", i, "name", mapKeys(item))
		}
		if _, present := item["status"]; !present {
			t.Errorf("item[%d] missing required field %q (got keys: %v)", i, "status", mapKeys(item))
		}
	}
}

// TestClustersJSON_EmptyArray verifies that an empty cluster list still encodes
// as `[]` (not `null`), matching the existing pre-migration behavior for empty
// arrays.
func TestClustersJSON_EmptyArray(t *testing.T) {
	var buf bytes.Buffer
	if err := renderJSON(&buf, nil); err != nil {
		t.Fatalf("renderJSON error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != "[]" {
		t.Errorf("empty render = %q, want %q", out, "[]")
	}
}

// TestClustersText_StatusColumn verifies the tabwriter output includes a
// STATUS column with values from the {Running, Paused, Error} vocabulary.
func TestClustersText_StatusColumn(t *testing.T) {
	infos := []clusterInfo{
		{Name: "kind", Status: "Running"},
		{Name: "dev", Status: "Paused"},
	}
	var buf bytes.Buffer
	if err := renderText(&buf, infos); err != nil {
		t.Fatalf("renderText error: %v", err)
	}
	out := buf.String()
	for _, col := range []string{"NAME", "STATUS"} {
		if !strings.Contains(out, col) {
			t.Errorf("text output missing header %q. Output:\n%s", col, out)
		}
	}
	for _, val := range []string{"kind", "Running", "dev", "Paused"} {
		if !strings.Contains(out, val) {
			t.Errorf("text output missing value %q. Output:\n%s", val, out)
		}
	}
}

func mapKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
