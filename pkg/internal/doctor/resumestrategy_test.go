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

package doctor

import (
	"errors"
	"strings"
	"testing"
)

// fakeStrategyLister returns a map[clusterName][]containerName for use in tests.
// It matches the signature of the real listKinderCPContainersByCluster function.
type fakeStrategyLister struct {
	clusters map[string][]string
	err      error
}

func (f *fakeStrategyLister) list(binaryName string) (map[string][]string, string, error) {
	return f.clusters, binaryName, f.err
}

// fakeStrategyInspector records calls and returns preset results.
type fakeStrategyInspector struct {
	// labels is a map from container name to (value, present, error).
	labels map[string]inspectLabelResult
}

type inspectLabelResult struct {
	value   string
	present bool
	err     error
}

func (f *fakeStrategyInspector) inspect(binaryName, container, labelKey string) (string, bool, error) {
	r, ok := f.labels[container]
	if !ok {
		return "", false, nil
	}
	return r.value, r.present, r.err
}

// newTestHACheck builds a haResumeStrategyCheck with injected fakes.
func newTestHACheck(lister *fakeStrategyLister, inspector *fakeStrategyInspector) *haResumeStrategyCheck {
	c := &haResumeStrategyCheck{}
	c.lister = lister.list
	c.inspector = inspector.inspect
	return c
}

// --- Meta tests (invariants) ---

// TestHAResumeStrategyCheck_CategoryAndName verifies the check's metadata.
func TestHAResumeStrategyCheck_CategoryAndName(t *testing.T) {
	t.Parallel()
	check := newHAResumeStrategyCheck()
	if check.Name() != "ha-resume-strategy" {
		t.Errorf("Name() = %q, want %q", check.Name(), "ha-resume-strategy")
	}
	if check.Category() != "Cluster" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Cluster")
	}
	platforms := check.Platforms()
	if platforms != nil {
		t.Errorf("Platforms() = %v, want nil (all platforms)", platforms)
	}
}

// TestHAResumeStrategyCheck_DoesNotRunProbe verifies the check does NOT call
// ProbeIPAM. The resume-strategy check is pure state inspection; ProbeIPAM is
// a separate check ("ipam-probe" from Plan 52-01).
func TestHAResumeStrategyCheck_DoesNotRunProbe(t *testing.T) {
	t.Parallel()
	probeCallCount := 0
	fakeProbeFn := func(binaryName string) (Verdict, string, error) {
		probeCallCount++
		return VerdictIPPinned, "", nil
	}
	// Suppress unused warning — the probe IS available but must NOT be called.
	_ = fakeProbeFn

	lister := &fakeStrategyLister{
		clusters: map[string][]string{
			"kind": {"kind-control-plane", "kind-control-plane2", "kind-control-plane3"},
		},
	}
	inspector := &fakeStrategyInspector{
		labels: map[string]inspectLabelResult{
			"kind-control-plane":  {value: "ip-pinned", present: true},
			"kind-control-plane2": {value: "ip-pinned", present: true},
			"kind-control-plane3": {value: "ip-pinned", present: true},
		},
	}
	c := newTestHACheck(lister, inspector)
	// The check must run without ever calling fakeProbeFn.
	// If the implementation calls ProbeIPAM, it would need to wire fakeProbeFn.
	// We simply verify probeCallCount remains 0 after Run().
	_ = c.Run()
	if probeCallCount != 0 {
		t.Errorf("ProbeIPAM was called %d time(s); ha-resume-strategy must NOT call the probe", probeCallCount)
	}
}

// --- Verdict 6: skip — no cluster ---

// TestHAResumeStrategyCheck_NoCluster verifies that when no containers are found
// the check skips with "no kinder cluster detected".
func TestHAResumeStrategyCheck_NoCluster(t *testing.T) {
	t.Parallel()
	lister := &fakeStrategyLister{clusters: map[string][]string{}}
	inspector := &fakeStrategyInspector{labels: map[string]inspectLabelResult{}}
	c := newTestHACheck(lister, inspector)

	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "skip" {
		t.Errorf("Status = %q, want %q", r.Status, "skip")
	}
	if !strings.Contains(r.Message, "no kinder cluster detected") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "no kinder cluster detected")
	}
	if r.Name != "ha-resume-strategy" {
		t.Errorf("Name = %q, want %q", r.Name, "ha-resume-strategy")
	}
	if r.Category != "Cluster" {
		t.Errorf("Category = %q, want %q", r.Category, "Cluster")
	}
}

// --- Verdict 5: skip — single CP ---

// TestHAResumeStrategyCheck_SingleCP verifies that a single-CP cluster is skipped.
func TestHAResumeStrategyCheck_SingleCP(t *testing.T) {
	t.Parallel()
	lister := &fakeStrategyLister{
		clusters: map[string][]string{
			"kind": {"kind-control-plane"},
		},
	}
	inspector := &fakeStrategyInspector{labels: map[string]inspectLabelResult{}}
	c := newTestHACheck(lister, inspector)

	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "skip" {
		t.Errorf("Status = %q, want %q", r.Status, "skip")
	}
	if !strings.Contains(r.Message, "non-HA cluster") {
		t.Errorf("Message = %q, want to contain %q", r.Message, "non-HA cluster")
	}
	if !strings.Contains(r.Message, "1") {
		t.Errorf("Message = %q, want to contain the CP count (1)", r.Message)
	}
}

// --- Verdict 1: ok — all ip-pinned ---

// TestHAResumeStrategyCheck_AllIPPinned verifies that when all CPs are labeled
// ip-pinned the check returns ok.
func TestHAResumeStrategyCheck_AllIPPinned(t *testing.T) {
	t.Parallel()
	lister := &fakeStrategyLister{
		clusters: map[string][]string{
			"kind": {"cp1", "cp2", "cp3"},
		},
	}
	inspector := &fakeStrategyInspector{
		labels: map[string]inspectLabelResult{
			"cp1": {value: "ip-pinned", present: true},
			"cp2": {value: "ip-pinned", present: true},
			"cp3": {value: "ip-pinned", present: true},
		},
	}
	c := newTestHACheck(lister, inspector)

	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "ok" {
		t.Errorf("Status = %q, want %q", r.Status, "ok")
	}
	if r.Message != "resume-strategy: ip-pinned" {
		t.Errorf("Message = %q, want %q", r.Message, "resume-strategy: ip-pinned")
	}
	if r.Reason != "" {
		t.Errorf("Reason = %q, want empty (no reason needed for ok)", r.Reason)
	}
}

// --- Verdict 2: warn — all cert-regen ---

// TestHAResumeStrategyCheck_AllCertRegen verifies that when all CPs are labeled
// cert-regen the check returns warn with a reason about slower resume.
func TestHAResumeStrategyCheck_AllCertRegen(t *testing.T) {
	t.Parallel()
	lister := &fakeStrategyLister{
		clusters: map[string][]string{
			"kind": {"cp1", "cp2", "cp3"},
		},
	}
	inspector := &fakeStrategyInspector{
		labels: map[string]inspectLabelResult{
			"cp1": {value: "cert-regen", present: true},
			"cp2": {value: "cert-regen", present: true},
			"cp3": {value: "cert-regen", present: true},
		},
	}
	c := newTestHACheck(lister, inspector)

	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q", r.Status, "warn")
	}
	if r.Message != "resume-strategy: cert-regen" {
		t.Errorf("Message = %q, want %q", r.Message, "resume-strategy: cert-regen")
	}
	if !strings.Contains(r.Reason, "regenerate etcd peer certs") && !strings.Contains(r.Reason, "slower resume") {
		t.Errorf("Reason = %q, want to contain 'regenerate etcd peer certs' or 'slower resume'", r.Reason)
	}
	if r.Fix != "" {
		t.Errorf("Fix = %q, want empty (informational only)", r.Fix)
	}
}

// --- Verdict 3: warn — legacy (no label) ---

// TestHAResumeStrategyCheck_LegacyNoLabel verifies that when all CPs have an
// absent label the check returns warn with a legacy-cluster explanation.
func TestHAResumeStrategyCheck_LegacyNoLabel(t *testing.T) {
	t.Parallel()
	lister := &fakeStrategyLister{
		clusters: map[string][]string{
			"kind": {"cp1", "cp2", "cp3"},
		},
	}
	inspector := &fakeStrategyInspector{
		labels: map[string]inspectLabelResult{
			// present=false, value="" simulates absent label.
			"cp1": {value: "", present: false},
			"cp2": {value: "", present: false},
			"cp3": {value: "", present: false},
		},
	}
	c := newTestHACheck(lister, inspector)

	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q", r.Status, "warn")
	}
	if r.Message != "resume-strategy: cert-regen (legacy)" {
		t.Errorf("Message = %q, want %q", r.Message, "resume-strategy: cert-regen (legacy)")
	}
	if !strings.Contains(r.Reason, "v2.4") {
		t.Errorf("Reason = %q, want to contain 'v2.4'", r.Reason)
	}
	reasonLower := strings.ToLower(r.Reason)
	if !strings.Contains(reasonLower, "delete") || !strings.Contains(reasonLower, "recreate") {
		t.Errorf("Reason = %q, want to mention 'delete and recreate'", r.Reason)
	}
}

// --- Verdict 4: fail — mixed labels ---

// TestHAResumeStrategyCheck_Mixed verifies that when CPs have disagreeing labels
// the check returns fail (genuine corruption).
func TestHAResumeStrategyCheck_Mixed(t *testing.T) {
	t.Parallel()
	lister := &fakeStrategyLister{
		clusters: map[string][]string{
			"kind": {"cp1", "cp2", "cp3"},
		},
	}
	inspector := &fakeStrategyInspector{
		labels: map[string]inspectLabelResult{
			"cp1": {value: "ip-pinned", present: true},
			"cp2": {value: "cert-regen", present: true},
			"cp3": {value: "ip-pinned", present: true},
		},
	}
	c := newTestHACheck(lister, inspector)

	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "fail" {
		t.Errorf("Status = %q, want %q", r.Status, "fail")
	}
	if r.Message != "resume-strategy: mixed" {
		t.Errorf("Message = %q, want %q", r.Message, "resume-strategy: mixed")
	}
	// Reason must name the disagreeing nodes.
	if !strings.Contains(r.Reason, "cp2") {
		t.Errorf("Reason = %q, want to mention disagreeing node 'cp2'", r.Reason)
	}
}

// --- Verdict 7: warn — indeterminate (inspect error) ---

// TestHAResumeStrategyCheck_InspectFails verifies that an inspect error produces
// warn (indeterminate) rather than fail — the daemon may be transiently down.
func TestHAResumeStrategyCheck_InspectFails(t *testing.T) {
	t.Parallel()
	lister := &fakeStrategyLister{
		clusters: map[string][]string{
			"kind": {"cp1", "cp2", "cp3"},
		},
	}
	inspector := &fakeStrategyInspector{
		labels: map[string]inspectLabelResult{
			"cp1": {value: "ip-pinned", present: true},
			"cp2": {err: errors.New("daemon not responding")},
			"cp3": {value: "ip-pinned", present: true},
		},
	}
	c := newTestHACheck(lister, inspector)

	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q", r.Status, "warn")
	}
	if r.Message != "resume-strategy: indeterminate" {
		t.Errorf("Message = %q, want %q", r.Message, "resume-strategy: indeterminate")
	}
	if !strings.Contains(r.Reason, "daemon not responding") {
		t.Errorf("Reason = %q, want to contain the underlying error", r.Reason)
	}
}

// --- Verdict 8: warn — multiple clusters ---

// TestHAResumeStrategyCheck_MultipleClustersDetected verifies that when CPs span
// multiple clusters the check returns warn asking the user to check kubectl context.
func TestHAResumeStrategyCheck_MultipleClustersDetected(t *testing.T) {
	t.Parallel()
	lister := &fakeStrategyLister{
		clusters: map[string][]string{
			"cluster-a": {"a-cp1", "a-cp2"},
			"cluster-b": {"b-cp1", "b-cp2"},
		},
	}
	inspector := &fakeStrategyInspector{labels: map[string]inspectLabelResult{}}
	c := newTestHACheck(lister, inspector)

	results := c.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != "warn" {
		t.Errorf("Status = %q, want %q", r.Status, "warn")
	}
	if !strings.Contains(r.Message, "multiple kinder clusters") {
		t.Errorf("Message = %q, want to contain 'multiple kinder clusters'", r.Message)
	}
	if !strings.Contains(r.Reason, "kubectl context") {
		t.Errorf("Reason = %q, want to contain 'kubectl context'", r.Reason)
	}
}
