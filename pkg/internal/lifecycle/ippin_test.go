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

package lifecycle

// NOTE: Tests in this file that swap the package-level probeIPAMFn or
// ippinCmder globals in pkg/internal/ippin MUST NOT use t.Parallel() because
// they mutate shared package state. Only pure data-model tests
// (TestIPAMState_RoundTrip) are safe to parallelize.
//
// Import-cycle note: this file tests the lifecycle facade
// (RecordAndPinHAControlPlane, ReadIPAMState), which delegate to
// pkg/internal/ippin. The test swaps ippin.ProbeIPAMFn and ippin.IppinCmder
// directly, then calls the facade function to confirm delegation works.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/exec"
	kindippin "sigs.k8s.io/kind/pkg/internal/ippin"
	"sigs.k8s.io/kind/pkg/internal/doctor"
	"sigs.k8s.io/kind/pkg/log"
)

// ---- ippinFakeCmd -----------------------------------------------------------

// ippinFakeCmd implements exec.Cmd. When SetStdout is called the output string
// is written to the provided writer (exec.OutputLines calls SetStdout → Run).
type ippinFakeCmd struct {
	output  string
	err     error
	stdoutW io.Writer
}

var _ exec.Cmd = (*ippinFakeCmd)(nil)

func (f *ippinFakeCmd) Run() error {
	if f.stdoutW != nil && f.output != "" {
		_, _ = io.WriteString(f.stdoutW, f.output)
	}
	return f.err
}
func (f *ippinFakeCmd) SetEnv(_ ...string) exec.Cmd    { return f }
func (f *ippinFakeCmd) SetStdin(_ io.Reader) exec.Cmd  { return f }
func (f *ippinFakeCmd) SetStdout(w io.Writer) exec.Cmd { f.stdoutW = w; return f }
func (f *ippinFakeCmd) SetStderr(_ io.Writer) exec.Cmd { return f }

// ---- ippinCmderFake ---------------------------------------------------------

type ippinCmdRecord struct {
	name string
	args []string
}

type ippinCmdResp struct {
	output string
	err    error
}

// ippinCmderFake records every call and returns pre-scripted responses.
type ippinCmderFake struct {
	calls     []ippinCmdRecord
	responses []ippinCmdResp
}

func (f *ippinCmderFake) cmder(name string, args ...string) exec.Cmd {
	idx := len(f.calls)
	f.calls = append(f.calls, ippinCmdRecord{name: name, args: args})
	if idx >= len(f.responses) {
		return &ippinFakeCmd{}
	}
	r := f.responses[idx]
	return &ippinFakeCmd{output: r.output, err: r.err}
}

func (f *ippinCmderFake) callStr(i int) string {
	if i >= len(f.calls) {
		return "<no call>"
	}
	parts := append([]string{f.calls[i].name}, f.calls[i].args...)
	return strings.Join(parts, " ")
}

// ---- ippinTestLogger --------------------------------------------------------

type ippinTestLogger struct {
	warns []string
}

func (l *ippinTestLogger) Warn(msg string)                   { l.warns = append(l.warns, msg) }
func (l *ippinTestLogger) Warnf(f string, a ...interface{})  { l.warns = append(l.warns, fmt.Sprintf(f, a...)) }
func (l *ippinTestLogger) Error(msg string)                  {}
func (l *ippinTestLogger) Errorf(f string, a ...interface{}) {}
func (l *ippinTestLogger) V(_ log.Level) log.InfoLogger      { return log.NoopInfoLogger{} }

var _ log.Logger = (*ippinTestLogger)(nil)

// ---- swapIPPinFns helpers ---------------------------------------------------

// swapIPPinCmder replaces ippin.IppinCmder for the duration of the test.
// MUST NOT be used with t.Parallel().
func swapIPPinCmder(t *testing.T, fc *ippinCmderFake) {
	t.Helper()
	prev := kindippin.IppinCmder
	kindippin.IppinCmder = fc.cmder
	t.Cleanup(func() { kindippin.IppinCmder = prev })
}

// swapIPPinProbe replaces ippin.ProbeIPAMFn for the duration of the test.
// MUST NOT be used with t.Parallel().
func swapIPPinProbe(t *testing.T, fn func(string) (doctor.Verdict, string, error)) {
	t.Helper()
	prev := kindippin.ProbeIPAMFn
	kindippin.ProbeIPAMFn = fn
	t.Cleanup(func() { kindippin.ProbeIPAMFn = prev })
}

// ---- Tests ------------------------------------------------------------------

// TestRecordAndPinHAControlPlane_SingleCPSkip: 1 container → no-op.
func TestRecordAndPinHAControlPlane_SingleCPSkip(t *testing.T) {
	fc := &ippinCmderFake{}
	swapIPPinCmder(t, fc)
	swapIPPinProbe(t, func(_ string) (doctor.Verdict, string, error) {
		t.Error("probe must NOT be called for single-CP")
		return doctor.VerdictCertRegen, "", nil
	})

	err := RecordAndPinHAControlPlane("docker", "kind", []string{"cp1"}, log.NoopLogger{})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(fc.calls) != 0 {
		t.Errorf("expected 0 commands, got %d", len(fc.calls))
	}
}

// TestRecordAndPinHAControlPlane_VerdictIPPinned_HappyPath: 3 CPs, VerdictIPPinned.
// Per CP: inspect → exec sh -c (file write) → disconnect → connect --ip.
func TestRecordAndPinHAControlPlane_VerdictIPPinned_HappyPath(t *testing.T) {
	containers := []string{"cp1", "cp2", "cp3"}
	ips := []string{"172.18.0.5", "172.18.0.6", "172.18.0.7"}

	var responses []ippinCmdResp
	for i := range containers {
		responses = append(responses,
			ippinCmdResp{output: ips[i] + "\n"},
			ippinCmdResp{},
			ippinCmdResp{},
			ippinCmdResp{},
		)
	}
	fc := &ippinCmderFake{responses: responses}
	swapIPPinCmder(t, fc)
	swapIPPinProbe(t, func(_ string) (doctor.Verdict, string, error) {
		return doctor.VerdictIPPinned, "", nil
	})

	err := RecordAndPinHAControlPlane("docker", "kind", containers, log.NoopLogger{})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(fc.calls) != 12 {
		t.Fatalf("expected 12 calls (4 per CP × 3), got %d", len(fc.calls))
	}

	for i, c := range containers {
		base := i * 4
		s := fc.callStr(base)
		if !strings.Contains(s, "inspect") || !strings.Contains(s, c) {
			t.Errorf("call[%d] should be inspect for %s, got: %s", base, c, s)
		}
		s = fc.callStr(base + 1)
		if !strings.Contains(s, "exec") || !strings.Contains(s, c) || !strings.Contains(s, "ipam-state.json") {
			t.Errorf("call[%d] should be exec with ipam-state.json for %s, got: %s", base+1, c, s)
		}
		s = fc.callStr(base + 2)
		if !strings.Contains(s, "disconnect") || !strings.Contains(s, c) {
			t.Errorf("call[%d] should be disconnect for %s, got: %s", base+2, c, s)
		}
		s = fc.callStr(base + 3)
		if !strings.Contains(s, "connect") || !strings.Contains(s, "--ip") || !strings.Contains(s, ips[i]) {
			t.Errorf("call[%d] should be connect --ip %s for %s, got: %s", base+3, ips[i], c, s)
		}
	}

	// No label-mutation
	for j, call := range fc.calls {
		joined := strings.Join(append([]string{call.name}, call.args...), " ")
		if strings.Contains(joined, "label-add") || strings.Contains(joined, "container update") {
			t.Errorf("call[%d] must not attempt label mutation: %s", j, joined)
		}
	}

	// File-write payload for cp1 must contain JSON keys
	s1 := fc.callStr(1)
	if !strings.Contains(s1, "\"network\"") || !strings.Contains(s1, "\"ipv4\"") {
		t.Errorf("file-write payload should contain JSON keys: %s", s1)
	}
}

// TestRecordAndPinHAControlPlane_VerdictCertRegen_NoReconnect: VerdictCertRegen
// → zero commands, nil error.
func TestRecordAndPinHAControlPlane_VerdictCertRegen_NoReconnect(t *testing.T) {
	fc := &ippinCmderFake{}
	swapIPPinCmder(t, fc)
	swapIPPinProbe(t, func(_ string) (doctor.Verdict, string, error) {
		return doctor.VerdictCertRegen, "docker too old", nil
	})

	err := RecordAndPinHAControlPlane("docker", "kind", []string{"cp1", "cp2", "cp3"}, log.NoopLogger{})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(fc.calls) != 0 {
		t.Errorf("expected 0 commands on VerdictCertRegen, got %d", len(fc.calls))
	}
}

// TestRecordAndPinHAControlPlane_NerdctlVerdict_NoReconnect: VerdictUnsupported
// → zero commands, nil error.
func TestRecordAndPinHAControlPlane_NerdctlVerdict_NoReconnect(t *testing.T) {
	fc := &ippinCmderFake{}
	swapIPPinCmder(t, fc)
	swapIPPinProbe(t, func(_ string) (doctor.Verdict, string, error) {
		return doctor.VerdictUnsupported, "nerdctl network connect is not implemented", nil
	})

	err := RecordAndPinHAControlPlane("nerdctl", "kind", []string{"cp1", "cp2"}, log.NoopLogger{})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(fc.calls) != 0 {
		t.Errorf("expected 0 commands on VerdictUnsupported, got %d", len(fc.calls))
	}
}

// TestRecordAndPinHAControlPlane_InspectMalformedIP: inspect returns
// "not-an-ip" → error containing "invalid IP from docker inspect".
func TestRecordAndPinHAControlPlane_InspectMalformedIP(t *testing.T) {
	fc := &ippinCmderFake{responses: []ippinCmdResp{{output: "not-an-ip\n"}}}
	swapIPPinCmder(t, fc)
	swapIPPinProbe(t, func(_ string) (doctor.Verdict, string, error) {
		return doctor.VerdictIPPinned, "", nil
	})

	err := RecordAndPinHAControlPlane("docker", "kind", []string{"cp1", "cp2"}, log.NoopLogger{})
	if err == nil {
		t.Fatal("expected error for malformed IP, got nil")
	}
	if !strings.Contains(err.Error(), "invalid IP from docker inspect") {
		t.Errorf("expected 'invalid IP from docker inspect' in error, got: %v", err)
	}
}

// TestRecordAndPinHAControlPlane_DisconnectFailureIsFatal: disconnect fails →
// error returned, cp2 NOT processed (3 calls only).
func TestRecordAndPinHAControlPlane_DisconnectFailureIsFatal(t *testing.T) {
	responses := []ippinCmdResp{
		{output: "172.18.0.5\n"},
		{},
		{err: fmt.Errorf("disconnect: permission denied")},
	}
	fc := &ippinCmderFake{responses: responses}
	swapIPPinCmder(t, fc)
	swapIPPinProbe(t, func(_ string) (doctor.Verdict, string, error) {
		return doctor.VerdictIPPinned, "", nil
	})

	err := RecordAndPinHAControlPlane("docker", "kind", []string{"cp1", "cp2"}, log.NoopLogger{})
	if err == nil {
		t.Fatal("expected error for disconnect failure, got nil")
	}
	if len(fc.calls) != 3 {
		t.Errorf("expected 3 calls (cp1 only), got %d", len(fc.calls))
	}
}

// TestRecordAndPinHAControlPlane_ConnectFailureRecoveryFails: W1 disposition.
// Both connect --ip and recovery fail. Error contains both causes; 5 calls only.
func TestRecordAndPinHAControlPlane_ConnectFailureRecoveryFails(t *testing.T) {
	connectErr := fmt.Errorf("connect --ip: address already in use")
	recoveryErr := fmt.Errorf("connect (recovery): network unreachable")

	responses := []ippinCmdResp{
		{output: "172.18.0.5\n"},
		{},
		{},
		{err: connectErr},
		{err: recoveryErr},
	}
	fc := &ippinCmderFake{responses: responses}
	swapIPPinCmder(t, fc)
	swapIPPinProbe(t, func(_ string) (doctor.Verdict, string, error) {
		return doctor.VerdictIPPinned, "", nil
	})

	err := RecordAndPinHAControlPlane("docker", "kind", []string{"cp1", "cp2"}, log.NoopLogger{})
	if err == nil {
		t.Fatal("expected wrapped hard error, got nil")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, connectErr.Error()) {
		t.Errorf("error must contain connect-ip failure: %v", err)
	}
	if !strings.Contains(errStr, recoveryErr.Error()) {
		t.Errorf("error must contain recovery failure: %v", err)
	}
	if len(fc.calls) != 5 {
		t.Errorf("expected 5 calls, got %d", len(fc.calls))
	}
	c3 := strings.Join(append([]string{fc.calls[3].name}, fc.calls[3].args...), " ")
	c4 := strings.Join(append([]string{fc.calls[4].name}, fc.calls[4].args...), " ")
	if !strings.Contains(c3, "--ip") {
		t.Errorf("call[3] should be connect --ip, got: %s", c3)
	}
	if strings.Contains(c4, "--ip") {
		t.Errorf("call[4] (recovery) should NOT have --ip, got: %s", c4)
	}
}

// TestRecordAndPinHAControlPlane_ConnectFailureRecoverySucceeds: connect --ip
// fails; recovery succeeds; still returns hard error.
func TestRecordAndPinHAControlPlane_ConnectFailureRecoverySucceeds(t *testing.T) {
	connectErr := fmt.Errorf("connect --ip: verdict drift")

	responses := []ippinCmdResp{
		{output: "172.18.0.5\n"},
		{},
		{},
		{err: connectErr},
		{},
	}
	fc := &ippinCmderFake{responses: responses}
	swapIPPinCmder(t, fc)
	swapIPPinProbe(t, func(_ string) (doctor.Verdict, string, error) {
		return doctor.VerdictIPPinned, "", nil
	})

	err := RecordAndPinHAControlPlane("docker", "kind", []string{"cp1", "cp2"}, log.NoopLogger{})
	if err == nil {
		t.Fatal("expected hard error even when recovery succeeds, got nil")
	}
	if !strings.Contains(err.Error(), connectErr.Error()) {
		t.Errorf("error should contain connect-ip failure: %v", err)
	}
	if len(fc.calls) != 5 {
		t.Errorf("expected 5 calls, got %d", len(fc.calls))
	}
}

// TestRecordAndPinHAControlPlane_ReadbackJSON: ReadIPAMState reads pre-placed
// JSON file (fake cp is a no-op, file already exists).
func TestRecordAndPinHAControlPlane_ReadbackJSON(t *testing.T) {
	state := IPAMState{Network: "kind", IPv4: "172.18.0.5"}
	payload, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	tmpDir := t.TempDir()
	hostPath := tmpDir + "/cp1-ipam.json"
	if err := os.WriteFile(hostPath, payload, 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	fc := &ippinCmderFake{responses: []ippinCmdResp{{}}}
	swapIPPinCmder(t, fc)

	got, err := ReadIPAMState("docker", "cp1", tmpDir)
	if err != nil {
		t.Fatalf("ReadIPAMState: %v", err)
	}
	if got.Network != state.Network {
		t.Errorf("Network: got %q, want %q", got.Network, state.Network)
	}
	if got.IPv4 != state.IPv4 {
		t.Errorf("IPv4: got %q, want %q", got.IPv4, state.IPv4)
	}
}

// TestIPAMState_RoundTrip: JSON marshal/unmarshal preserves all fields.
func TestIPAMState_RoundTrip(t *testing.T) {
	t.Parallel()
	orig := IPAMState{Network: "kind-net", IPv4: "10.96.0.1"}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got IPAMState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != orig {
		t.Errorf("round-trip: got %+v, want %+v", got, orig)
	}
}

// TestRecordAndPinHAControlPlane_ProbeRuntimeError: probe returns VerdictCertRegen
// with reason → Warn logged, nil returned, zero commands.
func TestRecordAndPinHAControlPlane_ProbeRuntimeError(t *testing.T) {
	fc := &ippinCmderFake{}
	swapIPPinCmder(t, fc)
	swapIPPinProbe(t, func(_ string) (doctor.Verdict, string, error) {
		return doctor.VerdictCertRegen, "probe runtime error: permission denied", nil
	})

	tl := &ippinTestLogger{}
	err := RecordAndPinHAControlPlane("docker", "kind", []string{"cp1", "cp2"}, tl)
	if err != nil {
		t.Fatalf("expected nil (warn-and-proceed), got %v", err)
	}
	if len(tl.warns) == 0 {
		t.Error("expected at least one Warn log for probe error, got none")
	}
	if len(fc.calls) != 0 {
		t.Errorf("expected 0 commands, got %d", len(fc.calls))
	}
}
