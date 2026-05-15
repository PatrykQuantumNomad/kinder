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

	"sigs.k8s.io/kind/pkg/cluster/loadbalancer"
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

// lbCmderState scripts the cmder behavior for one test. Phase 57.2: rewritten
// from the Phase 57.1 docker-network-probe shape to the ip-family-label shape.
// `sawIPFamilyInspect` replaces the old `sawNetworkInspect` / `sawIPv6Inspect`
// tripwires; `ipFamilyStdout` is the canned label value returned by the fake
// `docker inspect --format ... Config.Labels` call.
type lbCmderState struct {
	// ipFamilyStdout is the value the fake label-inspect call returns.
	// Tests set "ipv4\n", "ipv6\n", "dual\n", or "" (label absent).
	ipFamilyStdout string
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
	// sawIPFamilyInspect is set true when the fake cmder sees the host-side
	// `inspect --format ... Config.Labels ... ip-family ...` call issued by
	// loadbalancer.ClusterIPFamily. The Phase 57.2 replacement for the
	// removed sawNetworkInspect / sawIPv6Inspect tripwires.
	sawIPFamilyInspect bool
	// stdinByCall captures the stdin payload (LDS/CDS YAML) handed to
	// each `cp /dev/stdin <dest>` invocation routed through fakeNode.
	// Lets SC5 assert at the lifecycle layer that the bytes flowing to
	// the LB node are non-empty AND contain a CP container name (the
	// regression-detection signal: cds.yaml must NOT be "resources: []").
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

// newLBCmder dispatches on docker subcommand shape. Phase 57.2 dispatch cases:
//   - args[0]=="inspect" AND args contains "Config.Labels" AND "ip-family"
//     → label inspect (Phase 57.2 ClusterIPFamily). Sets sawIPFamilyInspect=true,
//     returns ipFamilyStdout. This replaces the Phase 57.1
//     NetworkSettings.Networks + network inspect EnableIPv6 dispatch pair.
//   - binary=="mkdir" → silent success (nodeutils.WriteFile mkdir before cp).
//   - binary=="cp" → captures stdin payload, advances attempt on error.
//   - binary=="sh" with args[0]=="-c" → final atomic-swap step; advances attempt.
//
// Phase 57.2: routes loadbalancer.ClusterIPFamily through a swapped cmder.
// Implementation detail: ClusterIPFamily uses its own package-level cmder
// (`defaultClusterIPFamilyCmder` in pkg/cluster/loadbalancer), distinct from
// lifecycle's `defaultCmder`. The lifecycle integration test swaps BOTH so
// the helper sees this fake too — done in each test via the loadbalancer
// helper withClusterIPFamilyCmder.
func newLBCmder(s *lbCmderState) Cmder {
	return func(binary string, args ...string) exec.Cmd {
		s.calls++
		joined := strings.Join(args, " ")

		// Case 1 (Phase 57.2): `docker inspect --format {{index .Config.Labels
		// "io.x-k8s.kinder.ip-family"}} <lb>` issued by ClusterIPFamily.
		// Binary is "docker"/"podman", args[0]=="inspect".
		if len(args) >= 2 && args[0] == "inspect" &&
			strings.Contains(joined, "Config.Labels") &&
			strings.Contains(joined, "ip-family") {
			s.sawIPFamilyInspect = true
			return &lbFakeCmd{state: s, slot: -1, stdoutS: s.ipFamilyStdout}
		}
		// Case 2a: `mkdir -p <dir>` issued by nodeutils.WriteFile before cp.
		// fakeNode routes through defaultCmder("mkdir", "-p", dir), so binary="mkdir".
		if binary == "mkdir" {
			return &lbFakeCmd{state: s, slot: -1}
		}
		// Case 2b: `cp /dev/stdin <dest>` issued by nodeutils.WriteFile.
		if binary == "cp" {
			idx := s.attempt
			var err error
			if idx < len(s.attemptErrs) {
				err = s.attemptErrs[idx]
			}
			slot := len(s.stdinByCall)
			s.stdinByCall = append(s.stdinByCall, "")
			if err != nil {
				s.attempt++
			}
			return &lbFakeCmd{state: s, slot: slot, err: err}
		}
		// Case 2c: `sh -c chmod && mv && mv` issued by WriteDynamicConfig.
		if binary == "sh" && len(args) >= 1 && args[0] == "-c" {
			idx := s.attempt
			var err error
			if idx < len(s.attemptErrs) {
				err = s.attemptErrs[idx]
			}
			s.attempt++
			return &lbFakeCmd{state: s, slot: -1, err: err}
		}
		return &lbFakeCmd{state: s, slot: -1, err: fmt.Errorf("unexpected call: %s %v", binary, args)}
	}
}

// withClusterIPFamilyCmderInLifecycle swaps the loadbalancer package's
// ClusterIPFamily cmder via the exported test hook
// loadbalancer.SetClusterIPFamilyCmderForTest, so the lifecycle tests can
// drive ClusterIPFamily via a single newLBCmder fixture. The exported hook
// exists because Go's package-private access prevents cross-package var swap.
func withClusterIPFamilyCmderInLifecycle(t *testing.T, c Cmder) {
	t.Helper()
	prev := loadbalancer.SetClusterIPFamilyCmderForTest(func(name string, args ...string) exec.Cmd {
		return c(name, args...)
	})
	t.Cleanup(func() { loadbalancer.SetClusterIPFamilyCmderForTest(prev) })
}

// TestReapplyLBConfig_NoLBNoOp (SC3): when lb is nil, zero docker calls.
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_NoLBNoOp(t *testing.T) {
	s := &lbCmderState{ipFamilyStdout: "ipv4\n"}
	withCmder(t, newLBCmder(s))
	withClusterIPFamilyCmderInLifecycle(t, newLBCmder(s))

	cps := fakeCPs("kinder-control-plane")
	err := reapplyLBConfig("docker", nil, cps, noopLogger{})
	if err != nil {
		t.Fatalf("expected nil error for no-LB case, got: %v", err)
	}
	if s.calls != 0 {
		t.Errorf("expected zero docker calls when lb=nil, got %d", s.calls)
	}
}

// TestReapplyLBConfig_HappyIPv4 (SC5 + Phase 57.2 label-driven IPv4):
// 3-CP HA cluster, ip-family label = ipv4; all writes succeed first try.
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_HappyIPv4(t *testing.T) {
	withLBRetryBackoff(t, 0)

	s := &lbCmderState{ipFamilyStdout: "ipv4\n"}
	cmder := newLBCmder(s)
	withCmder(t, cmder)
	withClusterIPFamilyCmderInLifecycle(t, cmder)

	lb := &fakeNode{name: "kinder-lb", role: "external-load-balancer"}
	cps := fakeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	err := reapplyLBConfig("docker", lb, cps, noopLogger{})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !s.sawIPFamilyInspect {
		t.Error("expected sawIPFamilyInspect=true (ClusterIPFamily must read the label)")
	}
	if s.attempt != 1 {
		t.Errorf("expected succeeded on first attempt (attempt=1), got attempt=%d", s.attempt)
	}

	// SC5: cds/lds content non-empty at the lifecycle integration layer.
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

// TestReapplyLBConfig_IPv6FromLabel (Phase 57.2 — renamed from IPv6Detection):
// IPv6 ip-family label drives the IPv6 listener (write succeeds).
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_IPv6FromLabel(t *testing.T) {
	withLBRetryBackoff(t, 0)

	s := &lbCmderState{ipFamilyStdout: "ipv6\n"}
	cmder := newLBCmder(s)
	withCmder(t, cmder)
	withClusterIPFamilyCmderInLifecycle(t, cmder)

	lb := &fakeNode{name: "kinder-lb", role: "external-load-balancer"}
	cps := fakeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	err := reapplyLBConfig("docker", lb, cps, noopLogger{})
	if err != nil {
		t.Fatalf("expected nil error for IPv6 path, got: %v", err)
	}
	if !s.sawIPFamilyInspect {
		t.Error("expected sawIPFamilyInspect=true")
	}
}

// TestReapplyLBConfig_DualFromLabel (Phase 57.2): dual ip-family label
// collapses to IPv6=true (write succeeds). Symmetric with IPv6FromLabel.
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_DualFromLabel(t *testing.T) {
	withLBRetryBackoff(t, 0)

	s := &lbCmderState{ipFamilyStdout: "dual\n"}
	cmder := newLBCmder(s)
	withCmder(t, cmder)
	withClusterIPFamilyCmderInLifecycle(t, cmder)

	lb := &fakeNode{name: "kinder-lb", role: "external-load-balancer"}
	cps := fakeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	err := reapplyLBConfig("docker", lb, cps, noopLogger{})
	if err != nil {
		t.Fatalf("expected nil error for dual path, got: %v", err)
	}
	if !s.sawIPFamilyInspect {
		t.Error("expected sawIPFamilyInspect=true")
	}
}

// TestReapplyLBConfig_LabelAbsent_FailsLoud (Phase 57.2): label missing on
// LB container → ClusterIPFamily returns error → reapplyLBConfig propagates,
// returns error containing "delete and recreate the cluster"; no write is
// attempted (attempt stays 0).
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_LabelAbsent_FailsLoud(t *testing.T) {
	withLBRetryBackoff(t, 0)

	s := &lbCmderState{ipFamilyStdout: ""} // label absent
	cmder := newLBCmder(s)
	withCmder(t, cmder)
	withClusterIPFamilyCmderInLifecycle(t, cmder)

	lb := &fakeNode{name: "kinder-lb", role: "external-load-balancer"}
	cps := fakeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	err := reapplyLBConfig("docker", lb, cps, noopLogger{})
	if err == nil {
		t.Fatal("expected non-nil error when ip-family label is absent, got nil")
	}
	if !strings.Contains(err.Error(), "delete and recreate the cluster") {
		t.Errorf("error message does not contain 'delete and recreate the cluster'; got: %v", err)
	}
	if s.attempt != 0 {
		t.Errorf("no WriteDynamicConfig attempts should be made on label-missing failure; got attempt=%d", s.attempt)
	}
}

// TestReapplyLBConfig_RetrySuccessOnAttempt2: transient cp error on attempt 1,
// success on attempt 2.
// MUST NOT call t.Parallel().
func TestReapplyLBConfig_RetrySuccessOnAttempt2(t *testing.T) {
	withLBRetryBackoff(t, 0)

	s := &lbCmderState{
		ipFamilyStdout: "ipv4\n",
		attemptErrs:    []error{fmt.Errorf("transient cp error")},
	}
	cmder := newLBCmder(s)
	withCmder(t, cmder)
	withClusterIPFamilyCmderInLifecycle(t, cmder)

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
		ipFamilyStdout: "ipv4\n",
		attemptErrs: []error{
			fmt.Errorf("e1"),
			fmt.Errorf("e2"),
			fmt.Errorf("e3"),
		},
	}
	cmder := newLBCmder(s)
	withCmder(t, cmder)
	withClusterIPFamilyCmderInLifecycle(t, cmder)

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
