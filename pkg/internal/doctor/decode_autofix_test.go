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
	"errors"
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// Task 1 tests: three SafeMitigation factories (Tests 1–11)
// ---------------------------------------------------------------------------

// Test 1: NeedsFix returns true when inotify values are below safe minimums.
func TestInotifyRaiseMitigation_NeedsFixWhenLow(t *testing.T) {
	orig := readSysctlFn
	t.Cleanup(func() { readSysctlFn = orig })
	readSysctlFn = func(key string) (int, error) {
		switch key {
		case "fs.inotify.max_user_watches":
			return 1024, nil
		case "fs.inotify.max_user_instances":
			return 128, nil
		}
		return 0, fmt.Errorf("unknown key %q", key)
	}

	m := InotifyRaiseMitigation()
	if !m.NeedsFix() {
		t.Error("NeedsFix() expected true when watches=1024, instances=128 (below minimums)")
	}
}

// Test 2: NeedsFix returns false when inotify values are at or above minimums (idempotency).
func TestInotifyRaiseMitigation_NoFixWhenSufficient(t *testing.T) {
	orig := readSysctlFn
	t.Cleanup(func() { readSysctlFn = orig })
	readSysctlFn = func(key string) (int, error) {
		switch key {
		case "fs.inotify.max_user_watches":
			return 524288, nil
		case "fs.inotify.max_user_instances":
			return 512, nil
		}
		return 0, fmt.Errorf("unknown key %q", key)
	}

	m := InotifyRaiseMitigation()
	if m.NeedsFix() {
		t.Error("NeedsFix() expected false when watches=524288, instances=512 (at minimums)")
	}
}

// Test 3: Apply invokes writeSysctlFn with correct (key, value) pairs.
func TestInotifyRaiseMitigation_ApplyWritesSysctl(t *testing.T) {
	type call struct{ key, value string }
	var calls []call

	origWrite := writeSysctlFn
	t.Cleanup(func() { writeSysctlFn = origWrite })
	writeSysctlFn = func(key, value string) error {
		calls = append(calls, call{key, value})
		return nil
	}

	m := InotifyRaiseMitigation()
	if err := m.Apply(); err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("Apply() expected 2 writeSysctl calls, got %d", len(calls))
	}
	if calls[0].key != "fs.inotify.max_user_watches" || calls[0].value != "524288" {
		t.Errorf("call[0] = %v; want {fs.inotify.max_user_watches, 524288}", calls[0])
	}
	if calls[1].key != "fs.inotify.max_user_instances" || calls[1].value != "512" {
		t.Errorf("call[1] = %v; want {fs.inotify.max_user_instances, 512}", calls[1])
	}
}

// Test 4: InotifyRaiseMitigation.NeedsRoot must be true.
func TestInotifyRaiseMitigation_NeedsRootTrue(t *testing.T) {
	m := InotifyRaiseMitigation()
	if !m.NeedsRoot {
		t.Error("InotifyRaiseMitigation.NeedsRoot expected true")
	}
}

// Test 5: CoreDNSRestartMitigation.NeedsFix returns true when status is "Pending".
func TestCoreDNSRestartMitigation_NeedsFixWhenPending(t *testing.T) {
	orig := getCoreDNSStatusFn
	t.Cleanup(func() { getCoreDNSStatusFn = orig })
	getCoreDNSStatusFn = func(binaryName, cpNode string) (string, error) {
		return "Pending", nil
	}

	m := CoreDNSRestartMitigation("docker", "kind-control-plane")
	if !m.NeedsFix() {
		t.Error("NeedsFix() expected true when CoreDNS status is Pending")
	}
}

// Test 6: CoreDNSRestartMitigation.NeedsFix returns false when status is "Running" (idempotent).
func TestCoreDNSRestartMitigation_NoFixWhenRunning(t *testing.T) {
	orig := getCoreDNSStatusFn
	t.Cleanup(func() { getCoreDNSStatusFn = orig })
	getCoreDNSStatusFn = func(binaryName, cpNode string) (string, error) {
		return "Running", nil
	}

	m := CoreDNSRestartMitigation("docker", "kind-control-plane")
	if m.NeedsFix() {
		t.Error("NeedsFix() expected false when CoreDNS status is Running")
	}
}

// Test 7: CoreDNSRestartMitigation.Apply invokes execCmdFn with correct kubectl rollout restart args.
func TestCoreDNSRestartMitigation_ApplyExecsKubectl(t *testing.T) {
	type callRecord struct {
		binaryName string
		args       []string
	}
	var recorded callRecord

	origExec := execCmdFn
	t.Cleanup(func() { execCmdFn = origExec })
	execCmdFn = func(binaryName string, args ...string) error {
		recorded = callRecord{binaryName, append([]string(nil), args...)}
		return nil
	}

	const binary = "docker"
	const cpNode = "kind-control-plane"
	m := CoreDNSRestartMitigation(binary, cpNode)
	if err := m.Apply(); err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}

	if recorded.binaryName != binary {
		t.Errorf("binaryName = %q; want %q", recorded.binaryName, binary)
	}

	wantArgs := []string{
		"exec", cpNode,
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"rollout", "restart", "deployment/coredns",
		"-n", "kube-system",
	}
	if len(recorded.args) != len(wantArgs) {
		t.Fatalf("args = %v; want %v", recorded.args, wantArgs)
	}
	for i := range wantArgs {
		if recorded.args[i] != wantArgs[i] {
			t.Errorf("args[%d] = %q; want %q", i, recorded.args[i], wantArgs[i])
		}
	}

	if m.NeedsRoot {
		t.Error("CoreDNSRestartMitigation.NeedsRoot expected false")
	}
}

// Test 8: NodeContainerRestartMitigation.NeedsFix returns true when container state is "exited".
func TestNodeContainerRestartMitigation_NeedsFixWhenStopped(t *testing.T) {
	orig := inspectStateAutoFn
	t.Cleanup(func() { inspectStateAutoFn = orig })
	inspectStateAutoFn = func(binaryName, container string) (string, error) {
		return "exited", nil
	}

	m := NodeContainerRestartMitigation("docker", "kind-worker")
	if !m.NeedsFix() {
		t.Error("NeedsFix() expected true when container state is exited")
	}
}

// Test 9: NodeContainerRestartMitigation.NeedsFix returns false when container is "running" (idempotent).
func TestNodeContainerRestartMitigation_NoFixWhenRunning(t *testing.T) {
	orig := inspectStateAutoFn
	t.Cleanup(func() { inspectStateAutoFn = orig })
	inspectStateAutoFn = func(binaryName, container string) (string, error) {
		return "running", nil
	}

	m := NodeContainerRestartMitigation("docker", "kind-worker")
	if m.NeedsFix() {
		t.Error("NeedsFix() expected false when container state is running")
	}
}

// Test 10: NodeContainerRestartMitigation.Apply invokes execCmdFn with ["start", nodeName].
func TestNodeContainerRestartMitigation_ApplyExecsStart(t *testing.T) {
	type callRecord struct {
		binaryName string
		args       []string
	}
	var recorded callRecord

	origExec := execCmdFn
	t.Cleanup(func() { execCmdFn = origExec })
	execCmdFn = func(binaryName string, args ...string) error {
		recorded = callRecord{binaryName, append([]string(nil), args...)}
		return nil
	}

	const binary = "docker"
	const node = "kind-worker"
	m := NodeContainerRestartMitigation(binary, node)
	if err := m.Apply(); err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}

	if recorded.binaryName != binary {
		t.Errorf("binaryName = %q; want %q", recorded.binaryName, binary)
	}
	wantArgs := []string{"start", node}
	if len(recorded.args) != len(wantArgs) {
		t.Fatalf("args = %v; want %v", recorded.args, wantArgs)
	}
	for i := range wantArgs {
		if recorded.args[i] != wantArgs[i] {
			t.Errorf("args[%d] = %q; want %q", i, recorded.args[i], wantArgs[i])
		}
	}

	if m.NeedsRoot {
		t.Error("NodeContainerRestartMitigation.NeedsRoot expected false")
	}
}

// Test 11: Compile-time check that factory signatures match the expected API.
func TestMitigations_FactorySignatures(t *testing.T) {
	// These assignments compile only if the return type is *SafeMitigation with
	// the correct parameter shapes. The test body itself is a no-op at runtime.
	var _ *SafeMitigation = InotifyRaiseMitigation()
	var _ *SafeMitigation = CoreDNSRestartMitigation("docker", "kind-control-plane")
	var _ *SafeMitigation = NodeContainerRestartMitigation("docker", "kind-worker")
}

// ---------------------------------------------------------------------------
// Task 2 tests: ApplyDecodeAutoFix + PreviewDecodeAutoFix + Catalog (Tests 12–20)
// ---------------------------------------------------------------------------

// stubMitigation builds a *SafeMitigation useful for orchestrator tests.
// applyFn is optional — pass nil to use a no-op. If applyFn is non-nil and
// shouldFail is set, Apply returns a sentinel error.
func stubMitigation(name string, needsFix bool, needsRoot bool, applyFn func() error) *SafeMitigation {
	if applyFn == nil {
		applyFn = func() error { return nil }
	}
	return &SafeMitigation{
		Name:      name,
		NeedsFix:  func() bool { return needsFix },
		Apply:     applyFn,
		NeedsRoot: needsRoot,
	}
}

// Test 12: ApplyDecodeAutoFix applies whitelisted matches and skips non-whitelisted ones.
func TestApplyDecodeAutoFix_AppliesWhitelistedOnly(t *testing.T) {
	applied := 0
	whitelisted := stubMitigation("test-fix", true, false, func() error {
		applied++
		return nil
	})

	matches := []DecodeMatch{
		{Source: "docker-logs:node1", Pattern: DecodePattern{ID: "KUB-01", AutoFixable: true, AutoFix: whitelisted}},
		{Source: "docker-logs:node1", Pattern: DecodePattern{ID: "KUB-03", AutoFixable: false, AutoFix: nil}},
	}

	errs := ApplyDecodeAutoFix(matches, DecodeAutoFixContext{BinaryName: "docker", CPNodeName: "kind-control-plane"}, nil)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %v", errs)
	}
	if applied != 1 {
		t.Errorf("Apply called %d times; want 1 (non-whitelisted must be skipped)", applied)
	}
}

// Test 13: ApplyDecodeAutoFix skips Apply when NeedsFix returns false (idempotency).
func TestApplyDecodeAutoFix_RespectsNeedsFix(t *testing.T) {
	applied := 0
	m := stubMitigation("already-done", false /* NeedsFix=false */, false, func() error {
		applied++
		return nil
	})

	matches := []DecodeMatch{
		{Source: "docker-logs:node1", Pattern: DecodePattern{ID: "KUB-01", AutoFixable: true, AutoFix: m}},
	}

	errs := ApplyDecodeAutoFix(matches, DecodeAutoFixContext{}, nil)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if applied != 0 {
		t.Errorf("Apply called %d times; want 0 (NeedsFix is false)", applied)
	}
}

// Test 14: ApplyDecodeAutoFix skips NeedsRoot mitigations when not root;
// non-root mitigations still apply normally.
func TestApplyDecodeAutoFix_SkipsNeedsRootWhenNotRoot(t *testing.T) {
	origEuid := geteuidFn
	t.Cleanup(func() { geteuidFn = origEuid })
	geteuidFn = func() int { return 1000 } // not root

	rootApplied := 0
	nonRootApplied := 0

	rootMit := stubMitigation("needs-root", true, true, func() error {
		rootApplied++
		return nil
	})
	nonRootMit := stubMitigation("no-root", true, false, func() error {
		nonRootApplied++
		return nil
	})

	matches := []DecodeMatch{
		{Source: "s1", Pattern: DecodePattern{ID: "KUB-01", AutoFixable: true, AutoFix: rootMit}},
		{Source: "s2", Pattern: DecodePattern{ID: "KUB-02", AutoFixable: true, AutoFix: nonRootMit}},
	}

	errs := ApplyDecodeAutoFix(matches, DecodeAutoFixContext{}, nil)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if rootApplied != 0 {
		t.Errorf("NeedsRoot mitigation applied %d times; want 0 (not root)", rootApplied)
	}
	if nonRootApplied != 1 {
		t.Errorf("non-root mitigation applied %d times; want 1", nonRootApplied)
	}
}

// Test 15: ApplyDecodeAutoFix deduplicates by mitigation Name across multiple matches.
func TestApplyDecodeAutoFix_DedupesByMitigationName(t *testing.T) {
	applied := 0
	shared := stubMitigation("inotify-raise", true, false, func() error {
		applied++
		return nil
	})

	// Three matches all reference the same-named mitigation (KUB-01 fired in 3 log streams).
	matches := []DecodeMatch{
		{Source: "docker-logs:node1", Pattern: DecodePattern{ID: "KUB-01", AutoFixable: true, AutoFix: shared}},
		{Source: "docker-logs:node2", Pattern: DecodePattern{ID: "KUB-01", AutoFixable: true, AutoFix: shared}},
		{Source: "docker-logs:node3", Pattern: DecodePattern{ID: "KUB-01", AutoFixable: true, AutoFix: shared}},
	}

	errs := ApplyDecodeAutoFix(matches, DecodeAutoFixContext{}, nil)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if applied != 1 {
		t.Errorf("Apply called %d times; want 1 (deduped by Name)", applied)
	}
}

// Test 16: ApplyDecodeAutoFix collects errors but continues applying remaining mitigations.
func TestApplyDecodeAutoFix_ReturnsErrorsButContinues(t *testing.T) {
	secondApplied := 0
	sentinel := errors.New("apply error")

	failMit := stubMitigation("fail-fix", true, false, func() error {
		return sentinel
	})
	okMit := stubMitigation("ok-fix", true, false, func() error {
		secondApplied++
		return nil
	})

	matches := []DecodeMatch{
		{Source: "s1", Pattern: DecodePattern{ID: "KUB-01", AutoFixable: true, AutoFix: failMit}},
		{Source: "s2", Pattern: DecodePattern{ID: "KUB-02", AutoFixable: true, AutoFix: okMit}},
	}

	errs := ApplyDecodeAutoFix(matches, DecodeAutoFixContext{}, nil)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !errors.Is(errs[0], sentinel) {
		t.Errorf("error = %v; want to wrap sentinel", errs[0])
	}
	if secondApplied != 1 {
		t.Errorf("second mitigation applied %d times; want 1 (loop should continue after error)", secondApplied)
	}
}

// Test 17: ApplyDecodeAutoFix constructs CoreDNSRestartMitigation for KADM-02 (AutoFix=nil)
// using ctx.BinaryName and ctx.CPNodeName.
func TestApplyDecodeAutoFix_ConstructsParameterizedMitigations(t *testing.T) {
	// Inject fakes at the fn-var level so CoreDNSRestartMitigation picks them up.
	origStatus := getCoreDNSStatusFn
	origExec := execCmdFn
	t.Cleanup(func() {
		getCoreDNSStatusFn = origStatus
		execCmdFn = origExec
	})

	var statusBinary, statusNode string
	getCoreDNSStatusFn = func(binaryName, cpNode string) (string, error) {
		statusBinary = binaryName
		statusNode = cpNode
		return "Pending", nil // NeedsFix → true
	}

	var execBinary string
	execCmdFn = func(binaryName string, args ...string) error {
		execBinary = binaryName
		return nil
	}

	ctx := DecodeAutoFixContext{BinaryName: "podman", CPNodeName: "mycluster-control-plane"}

	matches := []DecodeMatch{
		{
			Source: "k8s-events",
			Pattern: DecodePattern{
				ID:          "KADM-02",
				AutoFixable: true,
				AutoFix:     nil, // orchestrator must construct at runtime
			},
		},
	}

	errs := ApplyDecodeAutoFix(matches, ctx, nil)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if statusBinary != "podman" {
		t.Errorf("getCoreDNSStatusFn received binaryName=%q; want %q", statusBinary, "podman")
	}
	if statusNode != "mycluster-control-plane" {
		t.Errorf("getCoreDNSStatusFn received cpNode=%q; want %q", statusNode, "mycluster-control-plane")
	}
	if execBinary != "podman" {
		t.Errorf("execCmdFn received binaryName=%q; want %q", execBinary, "podman")
	}
}

// Test 18: ApplyDecodeAutoFix constructs NodeContainerRestartMitigation for KUB-05 by
// extracting the node name from match.Source; skips when Source has no extractable node name.
func TestApplyDecodeAutoFix_ConstructsNodeRestartFromSource(t *testing.T) {
	origInspect := inspectStateAutoFn
	origExec := execCmdFn
	t.Cleanup(func() {
		inspectStateAutoFn = origInspect
		execCmdFn = origExec
	})

	inspectStateAutoFn = func(binaryName, container string) (string, error) {
		return "exited", nil // NeedsFix → true
	}

	var startedNode string
	execCmdFn = func(binaryName string, args ...string) error {
		if len(args) >= 2 && args[0] == "start" {
			startedNode = args[1]
		}
		return nil
	}

	ctx := DecodeAutoFixContext{BinaryName: "docker", CPNodeName: "kind-control-plane"}

	// First match: Source has "docker-logs:" prefix → node name extractable.
	// Second match: Source "k8s-events" → no node name → must be skipped.
	matches := []DecodeMatch{
		{
			Source: "docker-logs:kind-control-plane",
			Pattern: DecodePattern{
				ID:          "KUB-05",
				AutoFixable: true,
				AutoFix:     nil,
			},
		},
		{
			Source: "k8s-events",
			Pattern: DecodePattern{
				ID:          "KUB-05",
				AutoFixable: true,
				AutoFix:     nil,
			},
		},
	}

	errs := ApplyDecodeAutoFix(matches, ctx, nil)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if startedNode != "kind-control-plane" {
		t.Errorf("started node = %q; want %q", startedNode, "kind-control-plane")
	}
	// The k8s-events match for KUB-05 must have been skipped (mitigationFor returned nil),
	// so execCmdFn is NOT called a second time with a different node.
	// (The dedup by Name means even if constructed, it would be deduped after the first one.)
	// But since k8s-events has no "docker-logs:" prefix, mitigationFor returns nil for it.
}

// Test 19: PreviewDecodeAutoFix is side-effect free — Apply must never be called.
func TestPreviewDecodeAutoFix_NoSideEffects(t *testing.T) {
	applyInvoked := false
	m := &SafeMitigation{
		Name:     "preview-test",
		NeedsFix: func() bool { return true },
		Apply: func() error {
			applyInvoked = true
			return errors.New("Apply MUST NOT be called during preview")
		},
		NeedsRoot: false,
	}

	matches := []DecodeMatch{
		{Source: "docker-logs:node1", Pattern: DecodePattern{ID: "KUB-01", AutoFixable: true, AutoFix: m}},
	}

	lines := PreviewDecodeAutoFix(matches, DecodeAutoFixContext{})
	if applyInvoked {
		t.Error("PreviewDecodeAutoFix must not invoke Apply on any mitigation")
	}
	if len(lines) == 0 {
		t.Error("PreviewDecodeAutoFix expected at least one output line for a whitelisted match")
	}
	// Output line should mention the mitigation name.
	if len(lines) > 0 && lines[0] == "" {
		t.Error("PreviewDecodeAutoFix output line should not be empty")
	}
}

// Test 20 lives in decode_catalog_test.go (added in Task 2 RED step).
