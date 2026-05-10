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
	"io"
	"regexp"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/exec"
)

// setIPAMProbeCmder swaps the package-level ipamProbeCmder for testing.
// Tests that call this MUST NOT use t.Parallel() because ipamProbeCmder is a
// package-level global and concurrent mutation would cause a data race.
// Callers must restore the original in a defer.
func setIPAMProbeCmder(fn func(name string, args ...string) exec.Cmd) func() {
	orig := ipamProbeCmder
	ipamProbeCmder = fn
	return func() { ipamProbeCmder = orig }
}

// fakeIPAMCmd is a fake exec.Cmd that returns canned output and an optional error.
type fakeIPAMCmd struct {
	output string
	err    error
	stdout io.Writer
}

func (f *fakeIPAMCmd) Run() error {
	if f.stdout != nil && f.output != "" {
		_, _ = io.WriteString(f.stdout, f.output)
	}
	return f.err
}
func (f *fakeIPAMCmd) SetEnv(...string) exec.Cmd      { return f }
func (f *fakeIPAMCmd) SetStdin(io.Reader) exec.Cmd    { return f }
func (f *fakeIPAMCmd) SetStdout(w io.Writer) exec.Cmd { f.stdout = w; return f }
func (f *fakeIPAMCmd) SetStderr(io.Writer) exec.Cmd   { return f }

var _ exec.Cmd = &fakeIPAMCmd{}

// seqResponse is a single canned response in a sequenced fake cmder.
type seqResponse struct {
	output string
	err    error
}

// seqCmder builds a cmder that returns canned responses in order.
// If invoked more times than responses provided, it panics to catch test bugs.
func seqCmder(responses []seqResponse) func(name string, args ...string) exec.Cmd {
	i := 0
	return func(name string, args ...string) exec.Cmd {
		if i >= len(responses) {
			panic("seqCmder: more calls than expected responses; last call: " + name + " " + strings.Join(args, " "))
		}
		r := responses[i]
		i++
		return &fakeIPAMCmd{output: r.output, err: r.err}
	}
}

// recordingCmder records every invocation and delegates to a fallback cmder.
type recordingCmder struct {
	calls    []string
	delegate func(name string, args ...string) exec.Cmd
}

func (r *recordingCmder) cmder(name string, args ...string) exec.Cmd {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	return r.delegate(name, args...)
}

// TestIPAMProbe_NerdctlShortCircuit verifies that binaryName="nerdctl" returns
// VerdictUnsupported immediately, without calling any container command.
// Not parallel: uses package-level ipamProbeCmder global.
func TestIPAMProbe_NerdctlShortCircuit(t *testing.T) {
	restore := setIPAMProbeCmder(func(name string, args ...string) exec.Cmd {
		t.Errorf("ProbeIPAM called runtime command on nerdctl: %s %v", name, args)
		return &fakeIPAMCmd{}
	})
	defer restore()

	verdict, reason, err := ProbeIPAM("nerdctl")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if verdict != VerdictUnsupported {
		t.Errorf("expected VerdictUnsupported, got %q", verdict)
	}
	if !strings.Contains(reason, "nerdctl") {
		t.Errorf("expected reason to mention nerdctl, got %q", reason)
	}
}

// TestIPAMProbe_DockerHappyPath scripts the 8-step happy path and asserts VerdictIPPinned.
// Not parallel: uses package-level ipamProbeCmder global.
func TestIPAMProbe_DockerHappyPath(t *testing.T) {
	// Steps: network create, run, inspect(IP), disconnect, connect --ip, stop, start, inspect(IP), cleanup(rm -f, network rm)
	restore := setIPAMProbeCmder(seqCmder([]seqResponse{
		{output: "", err: nil},              // 1. network create
		{output: "", err: nil},              // 2. run
		{output: "172.200.0.5\n", err: nil}, // 3. inspect IP
		{output: "", err: nil},              // 4. network disconnect
		{output: "", err: nil},              // 5. network connect --ip
		{output: "", err: nil},              // 6. stop
		{output: "", err: nil},              // 7. start
		{output: "172.200.0.5\n", err: nil}, // 8. inspect IP (same)
		{output: "", err: nil},              // cleanup: rm -f
		{output: "", err: nil},              // cleanup: network rm
	}))
	defer restore()

	verdict, reason, err := ProbeIPAM("docker")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if verdict != VerdictIPPinned {
		t.Errorf("expected VerdictIPPinned, got %q (reason: %s)", verdict, reason)
	}
	if reason != "" {
		t.Errorf("expected empty reason on happy path, got %q", reason)
	}
}

// TestIPAMProbe_DockerIPChanged verifies VerdictCertRegen when post-start IP differs.
// Not parallel: uses package-level ipamProbeCmder global.
func TestIPAMProbe_DockerIPChanged(t *testing.T) {
	restore := setIPAMProbeCmder(seqCmder([]seqResponse{
		{output: "", err: nil},              // 1. network create
		{output: "", err: nil},              // 2. run
		{output: "172.200.0.5\n", err: nil}, // 3. inspect IP
		{output: "", err: nil},              // 4. disconnect
		{output: "", err: nil},              // 5. connect --ip
		{output: "", err: nil},              // 6. stop
		{output: "", err: nil},              // 7. start
		{output: "172.200.0.7\n", err: nil}, // 8. inspect IP (changed!)
		{output: "", err: nil},              // cleanup: rm -f
		{output: "", err: nil},              // cleanup: network rm
	}))
	defer restore()

	verdict, reason, err := ProbeIPAM("docker")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if verdict != VerdictCertRegen {
		t.Errorf("expected VerdictCertRegen, got %q", verdict)
	}
	if !strings.Contains(reason, "IP reassigned") {
		t.Errorf("expected reason to mention IP reassignment, got %q", reason)
	}
}

// TestIPAMProbe_DockerNetworkConnectFails verifies VerdictCertRegen when --ip connect fails.
// Not parallel: uses package-level ipamProbeCmder global.
func TestIPAMProbe_DockerNetworkConnectFails(t *testing.T) {
	connectErr := errors.New("network connect: address not available")
	restore := setIPAMProbeCmder(seqCmder([]seqResponse{
		{output: "", err: nil},              // 1. network create
		{output: "", err: nil},              // 2. run
		{output: "172.200.0.5\n", err: nil}, // 3. inspect IP
		{output: "", err: nil},              // 4. disconnect
		{output: "", err: connectErr},       // 5. connect --ip FAILS
		{output: "", err: nil},              // cleanup: rm -f
		{output: "", err: nil},              // cleanup: network rm
	}))
	defer restore()

	verdict, reason, err := ProbeIPAM("docker")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if verdict != VerdictCertRegen {
		t.Errorf("expected VerdictCertRegen, got %q", verdict)
	}
	if !strings.Contains(reason, "network connect") {
		t.Errorf("expected reason to mention network connect, got %q", reason)
	}
}

// TestIPAMProbe_PermissionDenied verifies soft fallback when network create fails.
// Not parallel: uses package-level ipamProbeCmder global.
func TestIPAMProbe_PermissionDenied(t *testing.T) {
	createErr := errors.New("permission denied")
	restore := setIPAMProbeCmder(seqCmder([]seqResponse{
		{output: "", err: createErr}, // 1. network create FAILS
		{output: "", err: nil},       // cleanup: rm -f
		{output: "", err: nil},       // cleanup: network rm
	}))
	defer restore()

	verdict, reason, err := ProbeIPAM("docker")
	if err != nil {
		t.Fatalf("expected nil error (soft fallback), got %v", err)
	}
	if verdict != VerdictCertRegen {
		t.Errorf("expected VerdictCertRegen, got %q", verdict)
	}
	if reason == "" {
		t.Errorf("expected non-empty reason on runtime error")
	}
}

// TestIPAMProbe_CleanupOnEarlyFailure verifies network rm IS issued even when run fails.
// Not parallel: uses package-level ipamProbeCmder global.
func TestIPAMProbe_CleanupOnEarlyFailure(t *testing.T) {
	runErr := errors.New("image not found")

	rec := &recordingCmder{
		delegate: seqCmder([]seqResponse{
			{output: "", err: nil},    // 1. network create (succeeds)
			{output: "", err: runErr}, // 2. run FAILS
			{output: "", err: nil},    // cleanup: rm -f
			{output: "", err: nil},    // cleanup: network rm
		}),
	}

	restore := setIPAMProbeCmder(rec.cmder)
	defer restore()

	verdict, _, err := ProbeIPAM("docker")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if verdict != VerdictCertRegen {
		t.Errorf("expected VerdictCertRegen, got %q", verdict)
	}

	// Verify cleanup: both rm -f and network rm were issued.
	networkRMSeen := false
	rmSeen := false
	for _, c := range rec.calls {
		if strings.Contains(c, "network rm") {
			networkRMSeen = true
		}
		if strings.Contains(c, "rm -f") {
			rmSeen = true
		}
	}
	if !networkRMSeen {
		t.Errorf("expected 'network rm' cleanup call, calls were: %v", rec.calls)
	}
	if !rmSeen {
		t.Errorf("expected 'rm -f' cleanup call, calls were: %v", rec.calls)
	}
}

// TestIPAMProbeCheck_DoctorOutput verifies verdict → Result translation.
// Parallel-safe: uses struct injection (no global).
func TestIPAMProbeCheck_DoctorOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		verdict         Verdict
		probeReason     string
		probeErr        error
		expectedStatus  string
		messageContains string
	}{
		{
			name:            "ip-pinned → ok",
			verdict:         VerdictIPPinned,
			expectedStatus:  "ok",
			messageContains: "ip-pinned",
		},
		{
			name:            "cert-regen → warn",
			verdict:         VerdictCertRegen,
			expectedStatus:  "warn",
			messageContains: "cert-regen",
		},
		{
			name:            "unsupported → warn",
			verdict:         VerdictUnsupported,
			expectedStatus:  "warn",
			messageContains: "unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			check := &ipamProbeCheck{
				probeFunc: func(binaryName string) (Verdict, string, error) {
					return tt.verdict, tt.probeReason, tt.probeErr
				},
				detectRuntime: func() string { return "docker" },
			}

			results := check.Run()
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			r := results[0]
			if r.Status != tt.expectedStatus {
				t.Errorf("expected status %q, got %q", tt.expectedStatus, r.Status)
			}
			if !strings.Contains(strings.ToLower(r.Message), tt.messageContains) {
				t.Errorf("expected message to contain %q, got %q", tt.messageContains, r.Message)
			}
		})
	}
}

// TestIPAMProbeCheck_NoRuntime verifies skip result when no runtime is detected.
// Parallel-safe: uses struct injection (no global).
func TestIPAMProbeCheck_NoRuntime(t *testing.T) {
	t.Parallel()

	check := &ipamProbeCheck{
		probeFunc:     func(string) (Verdict, string, error) { return VerdictIPPinned, "", nil },
		detectRuntime: func() string { return "" },
	}
	results := check.Run()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "skip" {
		t.Errorf("expected skip, got %q", results[0].Status)
	}
}

// TestIPAMProbe_UsesNonOverlappingSubnet asserts the probe network create call
// carries --subnet=172.200.0.0/24 (non-overlapping with kind's default 172.18.0.0/16).
// This test pins the magic value so any future change trips the test.
// Not parallel: uses package-level ipamProbeCmder global.
func TestIPAMProbe_UsesNonOverlappingSubnet(t *testing.T) {
	var networkCreateArgs string

	restore := setIPAMProbeCmder(func(name string, args ...string) exec.Cmd {
		call := name + " " + strings.Join(args, " ")
		if strings.Contains(call, "network create") {
			networkCreateArgs = call
			// Fail immediately after capturing args so probe exits early.
			return &fakeIPAMCmd{err: errors.New("abort after capture")}
		}
		// Cleanup calls succeed silently.
		return &fakeIPAMCmd{err: nil}
	})
	defer restore()

	ProbeIPAM("docker") //nolint:errcheck

	if !strings.Contains(networkCreateArgs, "--subnet=172.200.0.0/24") {
		t.Errorf("expected network create to use --subnet=172.200.0.0/24, got: %q", networkCreateArgs)
	}
	// Verify it does NOT use the kind default 172.18.0.0/16.
	if strings.Contains(networkCreateArgs, "172.18.") {
		t.Errorf("probe must not use kind's default subnet 172.18.0.0/16, got: %q", networkCreateArgs)
	}
}

// TestIPAMProbe_NetworkAndContainerNamesAreUnique verifies probe network and container
// names include a timestamp/PID suffix matching kinder-ipam-probe-[a-zA-Z0-9_-]+.
// Not parallel: uses package-level ipamProbeCmder global.
func TestIPAMProbe_NetworkAndContainerNamesAreUnique(t *testing.T) {
	uniqueNameRE := regexp.MustCompile(`kinder-ipam-probe-[a-zA-Z0-9_-]+`)

	var capturedNetworkName, capturedContainerName string
	callCount := 0

	restore := setIPAMProbeCmder(func(name string, args ...string) exec.Cmd {
		callCount++
		switch callCount {
		case 1: // network create — succeed and capture the network name arg
			for _, arg := range args {
				if uniqueNameRE.MatchString(arg) {
					capturedNetworkName = arg
				}
			}
			return &fakeIPAMCmd{err: nil}
		case 2: // run — capture --name value then abort
			for i, arg := range args {
				if arg == "--name" && i+1 < len(args) {
					capturedContainerName = args[i+1]
				}
			}
			return &fakeIPAMCmd{err: errors.New("abort after capture")}
		default: // cleanup calls (rm -f, network rm) succeed silently
			return &fakeIPAMCmd{err: nil}
		}
	})
	defer restore()

	ProbeIPAM("docker") //nolint:errcheck

	if capturedNetworkName == "" || !uniqueNameRE.MatchString(capturedNetworkName) {
		t.Errorf("probe network name %q does not match kinder-ipam-probe-[a-zA-Z0-9_-]+", capturedNetworkName)
	}
	if capturedContainerName == "" || !uniqueNameRE.MatchString(capturedContainerName) {
		t.Errorf("probe container name %q does not match kinder-ipam-probe-[a-zA-Z0-9_-]+", capturedContainerName)
	}
}
