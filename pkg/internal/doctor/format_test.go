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
	"strings"
	"testing"
)

func TestFormatHumanReadable_MixedStatuses(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Name: "container-runtime", Category: "Runtime", Status: "ok", Message: "Container runtime detected (docker)"},
		{Name: "kubectl", Category: "Tools", Status: "ok", Message: "kubectl found"},
		{Name: "disk-space", Category: "Tools", Status: "warn", Message: "Low disk space", Reason: "Less than 5GB available", Fix: "Free disk space"},
		{Name: "nvidia-driver", Category: "GPU", Status: "fail", Message: "nvidia-smi not found", Reason: "NVIDIA GPU addon requires the NVIDIA driver", Fix: "Install drivers: https://www.nvidia.com/drivers"},
		{Name: "inotify", Category: "Kernel", Status: "skip", Message: "(linux only)"},
	}

	var buf bytes.Buffer
	FormatHumanReadable(&buf, results)
	output := buf.String()

	// Check category headers
	if !strings.Contains(output, "=== Runtime ===") {
		t.Error("missing Runtime category header")
	}
	if !strings.Contains(output, "=== Tools ===") {
		t.Error("missing Tools category header")
	}
	if !strings.Contains(output, "=== GPU ===") {
		t.Error("missing GPU category header")
	}
	if !strings.Contains(output, "=== Kernel ===") {
		t.Error("missing Kernel category header")
	}

	// Check Unicode icons
	if !strings.Contains(output, "\u2713") { // checkmark
		t.Error("missing checkmark icon for ok status")
	}
	if !strings.Contains(output, "\u26A0") { // warning
		t.Error("missing warning icon for warn status")
	}
	if !strings.Contains(output, "\u2717") { // x mark
		t.Error("missing x mark icon for fail status")
	}
	if !strings.Contains(output, "\u2298") { // null/skip
		t.Error("missing null icon for skip status")
	}

	// Check 2-space indent
	if !strings.Contains(output, "  \u2713") {
		t.Error("ok results should be 2-space indented")
	}

	// Check separator
	if !strings.Contains(output, "\u2500\u2500\u2500") { // horizontal line
		t.Error("missing horizontal line separator")
	}

	// Check summary
	if !strings.Contains(output, "5 checks:") {
		t.Error("missing or incorrect total in summary")
	}
	if !strings.Contains(output, "2 ok") {
		t.Error("missing or incorrect ok count in summary")
	}
	if !strings.Contains(output, "1 warning") {
		t.Error("missing or incorrect warning count in summary")
	}
	if !strings.Contains(output, "1 failed") {
		t.Error("missing or incorrect fail count in summary")
	}
	if !strings.Contains(output, "1 skipped") {
		t.Error("missing or incorrect skip count in summary")
	}
}

func TestFormatHumanReadable_CategoryOrderPreserved(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Name: "check-a", Category: "Zebra", Status: "ok", Message: "a"},
		{Name: "check-b", Category: "Alpha", Status: "ok", Message: "b"},
		{Name: "check-c", Category: "Zebra", Status: "ok", Message: "c"},
	}

	var buf bytes.Buffer
	FormatHumanReadable(&buf, results)
	output := buf.String()

	// Zebra should appear before Alpha because it was first-seen first
	zebraIdx := strings.Index(output, "=== Zebra ===")
	alphaIdx := strings.Index(output, "=== Alpha ===")
	if zebraIdx < 0 || alphaIdx < 0 {
		t.Fatal("missing category headers")
	}
	if zebraIdx > alphaIdx {
		t.Error("categories should preserve first-seen order, Zebra should appear before Alpha")
	}
}

func TestFormatHumanReadable_WarnFailShowReasonAndFix(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Name: "test-warn", Category: "Test", Status: "warn", Message: "problem", Reason: "this is why", Fix: "do this"},
		{Name: "test-fail", Category: "Test", Status: "fail", Message: "critical", Reason: "very bad", Fix: "fix it"},
	}

	var buf bytes.Buffer
	FormatHumanReadable(&buf, results)
	output := buf.String()

	if !strings.Contains(output, "this is why") {
		t.Error("warn result should show reason")
	}
	if !strings.Contains(output, "\u2192 do this") { // arrow + fix
		t.Error("warn result should show fix with arrow prefix")
	}
	if !strings.Contains(output, "very bad") {
		t.Error("fail result should show reason")
	}
	if !strings.Contains(output, "\u2192 fix it") {
		t.Error("fail result should show fix with arrow prefix")
	}
}

func TestFormatHumanReadable_SkipShowsPlatformTag(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Name: "inotify", Category: "Kernel", Status: "skip", Message: "(linux only)"},
	}

	var buf bytes.Buffer
	FormatHumanReadable(&buf, results)
	output := buf.String()

	if !strings.Contains(output, "inotify") {
		t.Error("skip result should show check name")
	}
	if !strings.Contains(output, "(linux only)") {
		t.Error("skip result should show platform tag")
	}
}

func TestFormatJSON_Envelope(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Name: "a", Category: "Cat1", Status: "ok", Message: "good"},
		{Name: "b", Category: "Cat2", Status: "warn", Message: "meh", Reason: "r", Fix: "f"},
		{Name: "c", Category: "Cat1", Status: "fail", Message: "bad", Reason: "r2", Fix: "f2"},
		{Name: "d", Category: "Cat2", Status: "skip", Message: "(linux only)"},
	}

	envelope := FormatJSON(results)

	// Check "checks" key exists and has correct length
	checks, ok := envelope["checks"]
	if !ok {
		t.Fatal("envelope missing 'checks' key")
	}
	checksSlice, ok := checks.([]Result)
	if !ok {
		t.Fatal("'checks' is not []Result")
	}
	if len(checksSlice) != 4 {
		t.Errorf("expected 4 checks, got %d", len(checksSlice))
	}

	// Check "summary" key exists with correct counts
	summary, ok := envelope["summary"]
	if !ok {
		t.Fatal("envelope missing 'summary' key")
	}
	summaryMap, ok := summary.(map[string]int)
	if !ok {
		t.Fatal("'summary' is not map[string]int")
	}
	if summaryMap["total"] != 4 {
		t.Errorf("summary total = %d, want 4", summaryMap["total"])
	}
	if summaryMap["ok"] != 1 {
		t.Errorf("summary ok = %d, want 1", summaryMap["ok"])
	}
	if summaryMap["warn"] != 1 {
		t.Errorf("summary warn = %d, want 1", summaryMap["warn"])
	}
	if summaryMap["fail"] != 1 {
		t.Errorf("summary fail = %d, want 1", summaryMap["fail"])
	}
	if summaryMap["skip"] != 1 {
		t.Errorf("summary skip = %d, want 1", summaryMap["skip"])
	}
}

func TestFormatJSON_AllFieldsPresent(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Name: "test", Category: "Cat", Status: "warn", Message: "msg", Reason: "why", Fix: "how"},
	}

	envelope := FormatJSON(results)
	checks := envelope["checks"].([]Result)
	r := checks[0]

	if r.Name != "test" {
		t.Errorf("Name = %q, want 'test'", r.Name)
	}
	if r.Category != "Cat" {
		t.Errorf("Category = %q, want 'Cat'", r.Category)
	}
	if r.Status != "warn" {
		t.Errorf("Status = %q, want 'warn'", r.Status)
	}
	if r.Message != "msg" {
		t.Errorf("Message = %q, want 'msg'", r.Message)
	}
	if r.Reason != "why" {
		t.Errorf("Reason = %q, want 'why'", r.Reason)
	}
	if r.Fix != "how" {
		t.Errorf("Fix = %q, want 'how'", r.Fix)
	}
}
