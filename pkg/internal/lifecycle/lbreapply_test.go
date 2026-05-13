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

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
)

// withLBRetryBackoff swaps the package-level defaultLBRetryBackoff for the
// duration of a test. Mirrors withCertRegenSleeper in resume_test.go.
// Required by retry-success and retry-exhausted tests so they don't sleep
// real seconds. Plan 02 Task 2 defines `defaultLBRetryBackoff` in
// lbreapply.go; this helper is the test-side knob.
func withLBRetryBackoff(t *testing.T, d time.Duration) {
	t.Helper()
	prev := defaultLBRetryBackoff
	defaultLBRetryBackoff = d
	t.Cleanup(func() { defaultLBRetryBackoff = prev })
}

// fakeCPs builds a slice of fakeNodes with role="control-plane".
func fakeCPs(names ...string) []nodes.Node {
	out := make([]nodes.Node, 0, len(names))
	for _, n := range names {
		out = append(out, &fakeNode{name: n, role: "control-plane"})
	}
	return out
}

// lbCmderState scripts the cmder behavior for one test.
type lbCmderState struct {
	ipv6Stdout  string // "true\n" or "false\n"
	networkName string // first key in networks JSON
	// attemptErrs scripts the error returned by each WriteDynamicConfig
	// *attempt*. An "attempt" is one full triple (WriteFile LDS, WriteFile
	// CDS, sh -c chmod+mv). For simplicity, this cmder fails the FIRST
	// command of an attempt when attemptErrs[currentAttempt] != nil; the
	// helper short-circuits and the next attempt begins (retry loop).
	attemptErrs []error
	// attempt is the 0-based index of the current WriteDynamicConfig
	// attempt. Incremented when an attempt begins.
	attempt int
	// calls counts ALL invocations regardless of attempt.
	calls int
	// sawNetworkInspect / sawIPv6Inspect track docker inspect calls so
	// tests can assert IPv6 discovery happened.
	sawNetworkInspect, sawIPv6Inspect bool
	// stdinByCall captures the stdin payload (LDS/CDS YAML) handed to
	// each `cp /dev/stdin <dest>` invocation routed through fakeNode.
	// Lets SC5 assert at the lifecycle layer that the bytes flowing to
	// the LB node are non-empty AND contain a CP container name (the
	// regression-detection signal: cds.yaml must NOT be "resources: []").
	// RESEARCH §Pattern 4 confirms nodeutils.WriteFile uses
	// `cp /dev/stdin <dest>` with `SetStdin(...)`; capturing stdin in
	// this cmder is the right hook point.
	stdinByCall []string
}

// lbFakeCmd captures the stdin payload routed through SetStdin so the
// cmder can record the LDS/CDS YAML bytes that nodeutils.WriteFile
// streams via `cp /dev/stdin <dest>`. Cannot reuse the package-level
// fakeCmd because its SetStdin is a no-op (state_test.go:67).
type lbFakeCmd struct {
	state   *lbCmderState
	slot    int // index in state.stdinByCall to write to
	stdoutS string
	stdoutW io.Writer
	in      io.Reader
	err     error
}

var _ exec.Cmd = (*lbFakeCmd)(nil)

func (c *lbFakeCmd) Run() error {
	if c.in != nil && c.slot >= 0 {
		b, _ := io.ReadAll(c.in)
		// ensure slice has room
		for len(c.state.stdinByCall) <= c.slot {
			c.state.stdinByCall = append(c.state.stdinByCall, "")
		}
		c.state.stdinByCall[c.slot] = string(b)
	}
	if c.stdoutW != nil && c.stdoutS != "" {
		_, _ = c.stdoutW.Write([]byte(c.stdoutS))
	}
	return c.err
}
func (c *lbFakeCmd) SetEnv(_ ...string) exec.Cmd    { return c }
func (c *lbFakeCmd) SetStdin(r io.Reader) exec.Cmd  { c.in = r; return c }
func (c *lbFakeCmd) SetStdout(w io.Writer) exec.Cmd { c.stdoutW = w; return c }
func (c *lbFakeCmd) SetStderr(_ io.Writer) exec.Cmd { return c }

func newLBCmder(s *lbCmderState) Cmder {
	return func(binary string, args ...string) exec.Cmd {
		s.calls++
		joined := strings.Join(args, " ")
		// Case 1: docker inspect <lb> --format ...Networks → JSON
		if len(args) >= 4 && args[0] == "inspect" &&
			strings.Contains(joined, "NetworkSettings.Networks") {
			s.sawNetworkInspect = true
			return &lbFakeCmd{state: s, slot: -1, stdoutS: fmt.Sprintf(`{%q:{}}`, s.networkName)}
		}
		// Case 2: docker network inspect <name> --format ...EnableIPv6
		if len(args) >= 3 && args[0] == "network" && args[1] == "inspect" &&
			strings.Contains(joined, "EnableIPv6") {
			s.sawIPv6Inspect = true
			return &lbFakeCmd{state: s, slot: -1, stdoutS: s.ipv6Stdout}
		}
		// Case 3a: mkdir -p (issued by nodeutils.WriteFile before cp).
		// Always succeeds silently; does NOT advance attempt or capture stdin.
		if len(args) > 0 && args[0] == "mkdir" {
			return &lbFakeCmd{state: s, slot: -1}
		}
		// Case 3b: any WriteDynamicConfig path-cmd. Inspect the args[0]
		// — `cp` (WriteFile) or `sh` (chmod+mv). Both flow through
		// node.Command(...) which routes here via fakeNode.Command.
		//
		// We count the first cp call as "start of attempt"; if
		// attemptErrs[attempt] != nil, return that error and advance.
		if len(args) > 0 && (args[0] == "cp" || (args[0] == "sh" && len(args) >= 2 && args[1] == "-c")) {
			idx := s.attempt
			var err error
			if idx < len(s.attemptErrs) {
				err = s.attemptErrs[idx]
			}
			// Allocate a stdinByCall slot for cp calls so Run() can record
			// the YAML payload. sh calls don't carry payload — slot = -1.
			slot := -1
			if args[0] == "cp" {
				slot = len(s.stdinByCall)
				s.stdinByCall = append(s.stdinByCall, "")
			}
			// Advance attempt on the FINAL command of an attempt — the
			// `sh -c chmod && mv && mv`. WriteFile (cp) calls don't
			// advance the counter.
			if args[0] == "sh" {
				s.attempt++
			}
			// Also advance on a cp error so the next attempt starts cleanly.
			if args[0] == "cp" && err != nil {
				s.attempt++
			}
			return &lbFakeCmd{state: s, slot: slot, err: err}
		}
		return &lbFakeCmd{state: s, slot: -1, err: fmt.Errorf("unexpected call: %s %v", binary, args)}
	}
}

// TestReapplyLBConfig_NoLBNoOp (SC3): when lb is nil, zero docker calls.
// MUST NOT call t.Parallel() — mutates package-level defaultCmder and defaultLBRetryBackoff.
func TestReapplyLBConfig_NoLBNoOp(t *testing.T) {
	s := &lbCmderState{
		ipv6Stdout:  "false\n",
		networkName: "kind",
	}
	withCmder(t, newLBCmder(s))

	cps := fakeCPs("kinder-control-plane")
	err := reapplyLBConfig("docker", nil, cps, noopLogger{})
	if err != nil {
		t.Fatalf("expected nil error for no-LB case, got: %v", err)
	}
	if s.calls != 0 {
		t.Errorf("expected zero docker calls when lb=nil, got %d", s.calls)
	}
}

// TestReapplyLBConfig_HappyIPv4 (SC5 + IPv4 detection): 3-CP HA cluster,
// IPv6 disabled; all writes succeed on first attempt.
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_HappyIPv4(t *testing.T) {
	withLBRetryBackoff(t, 0)

	s := &lbCmderState{
		ipv6Stdout:  "false\n",
		networkName: "kind",
	}
	withCmder(t, newLBCmder(s))

	lb := &fakeNode{name: "kinder-lb", role: "external-load-balancer"}
	cps := fakeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	err := reapplyLBConfig("docker", lb, cps, noopLogger{})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !s.sawNetworkInspect {
		t.Error("expected sawNetworkInspect=true (docker inspect LB networks)")
	}
	if !s.sawIPv6Inspect {
		t.Error("expected sawIPv6Inspect=true (docker network inspect EnableIPv6)")
	}
	if s.attempt != 1 {
		t.Errorf("expected succeeded on first attempt (attempt=1), got attempt=%d", s.attempt)
	}

	// SC5: cds/lds content non-empty at the lifecycle integration layer.
	// The cmder captured every `cp /dev/stdin <dest>` stdin payload as the
	// helper streamed YAML to the LB node via nodeutils.WriteFile. If any
	// capture is empty OR none contain a CP container name, the cds.yaml
	// resources-empty regression (Phase 47↔51) has snuck back in.
	if len(s.stdinByCall) == 0 {
		t.Fatal("expected WriteFile stdin captures from WriteDynamicConfig, got 0")
	}
	var sawCPName bool
	for i, payload := range s.stdinByCall {
		if payload == "" {
			t.Errorf("captured empty payload at slot %d — cds/lds must NOT be resources: []", i)
		}
		if strings.Contains(payload, "kinder-control-plane") {
			sawCPName = true
		}
	}
	if !sawCPName {
		t.Error("no captured payload contained a CP container name; cds.yaml regression possible")
	}
}

// TestReapplyLBConfig_IPv6Detection: same as HappyIPv4 but IPv6 enabled.
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_IPv6Detection(t *testing.T) {
	withLBRetryBackoff(t, 0)

	s := &lbCmderState{
		ipv6Stdout:  "true\n",
		networkName: "kind",
	}
	withCmder(t, newLBCmder(s))

	lb := &fakeNode{name: "kinder-lb", role: "external-load-balancer"}
	cps := fakeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	err := reapplyLBConfig("docker", lb, cps, noopLogger{})
	if err != nil {
		t.Fatalf("expected nil error for IPv6 path, got: %v", err)
	}
	if !s.sawNetworkInspect {
		t.Error("expected sawNetworkInspect=true")
	}
	if !s.sawIPv6Inspect {
		t.Error("expected sawIPv6Inspect=true")
	}
}

// TestReapplyLBConfig_RetrySuccessOnAttempt2: transient cp error on attempt 1,
// success on attempt 2.
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_RetrySuccessOnAttempt2(t *testing.T) {
	withLBRetryBackoff(t, 0)

	s := &lbCmderState{
		ipv6Stdout:  "false\n",
		networkName: "kind",
		attemptErrs: []error{fmt.Errorf("transient cp error")},
	}
	withCmder(t, newLBCmder(s))

	lb := &fakeNode{name: "kinder-lb", role: "external-load-balancer"}
	cps := fakeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	err := reapplyLBConfig("docker", lb, cps, noopLogger{})
	if err != nil {
		t.Fatalf("expected nil error (retry succeeded), got: %v", err)
	}
	if s.attempt != 2 {
		t.Errorf("expected attempt=2 (one retry; succeeded on second), got attempt=%d", s.attempt)
	}
}

// TestReapplyLBConfig_RetryExhausted: all 3 attempts fail, hard-fail with
// "after 3 attempts" + last error preserved.
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_RetryExhausted(t *testing.T) {
	withLBRetryBackoff(t, 0)

	s := &lbCmderState{
		ipv6Stdout:  "false\n",
		networkName: "kind",
		attemptErrs: []error{
			fmt.Errorf("e1"),
			fmt.Errorf("e2"),
			fmt.Errorf("e3"),
		},
	}
	withCmder(t, newLBCmder(s))

	lb := &fakeNode{name: "kinder-lb", role: "external-load-balancer"}
	cps := fakeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	start := time.Now()
	err := reapplyLBConfig("docker", lb, cps, noopLogger{})
	dur := time.Since(start)

	if err == nil {
		t.Fatal("expected non-nil error after 3 exhausted attempts, got nil")
	}
	if !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("error should contain 'after 3 attempts': %v", err)
	}
	if !strings.Contains(err.Error(), "e3") {
		t.Errorf("error should contain last error 'e3': %v", err)
	}
	if dur >= 100*time.Millisecond {
		t.Errorf("retry exhausted test took %v; expected <100ms (backoff must be injectable to 0)", dur)
	}
	if s.attempt != 3 {
		t.Errorf("expected attempt=3 (all 3 exhausted), got attempt=%d", s.attempt)
	}
}

// TestReapplyLBConfig_IPv6DiscoveryFailureDefaultsToIPv4: docker inspect error
// during IPv6 discovery → graceful fallback to IPv4, write path still succeeds.
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_IPv6DiscoveryFailureDefaultsToIPv4(t *testing.T) {
	withLBRetryBackoff(t, 0)

	s := &lbCmderState{
		ipv6Stdout:  "false\n",
		networkName: "kind",
	}
	// Custom cmder: docker inspect LB networks returns error (IPv6 discovery fails).
	// All other commands (mkdir, cp, sh) proceed via the standard lbCmderState logic.
	withCmder(t, func(binary string, args ...string) exec.Cmd {
		if len(args) >= 4 && args[0] == "inspect" &&
			strings.Contains(strings.Join(args, " "), "NetworkSettings.Networks") {
			s.calls++
			s.sawNetworkInspect = true
			return &lbFakeCmd{state: s, slot: -1, err: fmt.Errorf("docker inspect: container not found")}
		}
		return newLBCmder(s)(binary, args...)
	})

	lb := &fakeNode{name: "kinder-lb", role: "external-load-balancer"}
	cps := fakeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	err := reapplyLBConfig("docker", lb, cps, noopLogger{})
	if err != nil {
		t.Fatalf("expected nil error (graceful IPv6 fallback), got: %v", err)
	}
	if s.attempt != 1 {
		t.Errorf("expected attempt=1 (write succeeded despite IPv6 discovery error), got attempt=%d", s.attempt)
	}
}
