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

package loadbalancer

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
)

// testNode is a minimal nodes.Node implementation for ClusterIPFamily tests.
// Mirrors the testNode pattern in pkg/cluster/internal/loadbalancer/config_test.go.
// ClusterIPFamily reads the ip-family label via host-side `docker inspect`, so
// no inside-container Command routing is exercised by these tests — the host-
// side cmder is swapped via withClusterIPFamilyCmder.
type cipTestNode struct {
	name string
}

var _ nodes.Node = (*cipTestNode)(nil)

func (n *cipTestNode) String() string               { return n.name }
func (n *cipTestNode) Role() (string, error)        { return "external-load-balancer", nil }
func (n *cipTestNode) IP() (string, string, error)  { return "", "", nil }
func (n *cipTestNode) SerialLogs(_ io.Writer) error { return nil }
func (n *cipTestNode) Command(_ string, _ ...string) exec.Cmd {
	return &cipTestCmd{err: fmt.Errorf("cipTestNode.Command should not be called")}
}
func (n *cipTestNode) CommandContext(_ context.Context, _ string, _ ...string) exec.Cmd {
	return &cipTestCmd{err: fmt.Errorf("cipTestNode.CommandContext should not be called")}
}

// cipState scripts the fake cmder behavior for one test.
type cipState struct {
	// labelStdout is the stdout that the fake `inspect --format ... Config.Labels`
	// call returns. Tests set this to "ipv4\n", "ipv6\n", "dual\n", "", or
	// "bogus\n" to drive ClusterIPFamily's switch.
	labelStdout string
	// runErr is the error the fake cmd.Run() returns. Non-nil simulates docker
	// inspect failure (binary missing, container not found, etc.).
	runErr error
	// sawLabelInspect is set when the fake cmder receives an `inspect --format
	// ... Config.Labels ...` call — confirms ClusterIPFamily reads the label.
	sawLabelInspect bool
	// sawNetworkInspect is the regression tripwire: set when the fake cmder
	// receives any `network inspect` call. ClusterIPFamily MUST NEVER call
	// `docker network inspect` (that was the Phase 57.1 bug); tests assert
	// this stays false.
	sawNetworkInspect bool
	// calls counts every cmder invocation, for tests that want to verify a
	// single call was made.
	calls int
}

// cipTestCmd implements exec.Cmd. It records stdin (unused here), writes
// stdoutS to the SetStdout writer when Run() is called, and returns err.
type cipTestCmd struct {
	stdoutS string
	stdoutW io.Writer
	err     error
}

var _ exec.Cmd = (*cipTestCmd)(nil)

func (c *cipTestCmd) Run() error {
	if c.stdoutW != nil && c.stdoutS != "" {
		_, _ = c.stdoutW.Write([]byte(c.stdoutS))
	}
	return c.err
}
func (c *cipTestCmd) SetEnv(_ ...string) exec.Cmd    { return c }
func (c *cipTestCmd) SetStdin(_ io.Reader) exec.Cmd  { return c }
func (c *cipTestCmd) SetStdout(w io.Writer) exec.Cmd { c.stdoutW = w; return c }
func (c *cipTestCmd) SetStderr(_ io.Writer) exec.Cmd { return c }

// newCIPCmder returns a fake cmder that dispatches on the `docker inspect`
// vs `docker network inspect` shape. The "label" path returns labelStdout
// and sets sawLabelInspect=true; the "network inspect" path sets the tripwire
// sawNetworkInspect=true and ALWAYS returns an error (the helper should never
// reach this branch — if it does, the test fails on the network-inspect side
// AS WELL as the tripwire assertion).
func newCIPCmder(s *cipState) func(binary string, args ...string) exec.Cmd {
	return func(_ string, args ...string) exec.Cmd {
		s.calls++
		joined := strings.Join(args, " ")
		// Label inspect: `inspect --format {{index .Config.Labels "..."}} <node>`
		if len(args) >= 2 && args[0] == "inspect" && strings.Contains(joined, "Config.Labels") {
			s.sawLabelInspect = true
			return &cipTestCmd{stdoutS: s.labelStdout, err: s.runErr}
		}
		// Tripwire: helper must NEVER call `docker network inspect`.
		if len(args) >= 2 && args[0] == "network" && args[1] == "inspect" {
			s.sawNetworkInspect = true
			return &cipTestCmd{err: fmt.Errorf("ClusterIPFamily must NOT call docker network inspect (Phase 57.2 regression tripwire)")}
		}
		return &cipTestCmd{err: fmt.Errorf("unexpected cmder call: %v", args)}
	}
}

// withClusterIPFamilyCmder swaps the package-level defaultClusterIPFamilyCmder
// for the duration of one test. MUST NOT call t.Parallel() in tests that use
// this helper — they mutate a package global.
func withClusterIPFamilyCmder(t *testing.T, cmder func(string, ...string) exec.Cmd) {
	t.Helper()
	prev := defaultClusterIPFamilyCmder
	defaultClusterIPFamilyCmder = cmder
	t.Cleanup(func() { defaultClusterIPFamilyCmder = prev })
}

// TestClusterIPFamily_LabelMatrix is the 6-fixture matrix (per Phase 57.2 CONTEXT.md):
// {ipv4, ipv6, dual} x {macOS-style dual-stack docker network, linux-style IPv4-only
// network}. The "network type" dimension exists to document intent — but the helper
// MUST NEVER inspect the docker network at all (the Phase 57.1 bug). All 6 fixtures
// assert sawNetworkInspect == false.
//
// MUST NOT call t.Parallel() — mutates package-level defaultClusterIPFamilyCmder.
func TestClusterIPFamily_LabelMatrix(t *testing.T) {
	cases := []struct {
		name        string
		labelValue  string
		networkType string // documentation-only; does not affect the helper
		wantIPv6    bool
	}{
		{"ipv4-on-macOS-dual-stack-net", "ipv4\n", "dual-stack", false},
		{"ipv4-on-linux-ipv4-only-net", "ipv4\n", "ipv4-only", false},
		{"ipv6-on-macOS-dual-stack-net", "ipv6\n", "dual-stack", true},
		{"ipv6-on-linux-ipv4-only-net", "ipv6\n", "ipv4-only", true},
		{"dual-on-macOS-dual-stack-net", "dual\n", "dual-stack", true},
		{"dual-on-linux-ipv4-only-net", "dual\n", "ipv4-only", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := &cipState{labelStdout: tc.labelValue}
			withClusterIPFamilyCmder(t, newCIPCmder(s))

			node := &cipTestNode{name: "kinder-external-load-balancer"}
			got, err := ClusterIPFamily("docker", node)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantIPv6 {
				t.Errorf("ClusterIPFamily = %v, want %v (label=%q)", got, tc.wantIPv6, tc.labelValue)
			}
			if !s.sawLabelInspect {
				t.Error("expected sawLabelInspect=true (helper must read the label)")
			}
			if s.sawNetworkInspect {
				t.Errorf("CRITICAL: ClusterIPFamily called docker network inspect (Phase 57.1 regression — Phase 57.2 reverts this)")
			}
		})
	}
}

// TestClusterIPFamily_MacOSDualStackNetwork_IPv4Cluster is the named regression
// test for the bug-of-record (Phase 58 UAT test_09 failure, 2026-05-13). The
// scenario: an IPv4 cluster (config.IPFamily="" or "ipv4") running on macOS
// Docker Desktop's `kind` network, which is dual-stack by default (the docker
// network has `EnableIPv6=true`). Phase 57.1's resume-time IPv6 probe inspected
// the docker network and mis-classified the cluster as IPv6, causing the resumed LB listener to
// render `address: "::"` (IPv6-only) → host kubectl IPv4 port-mapping → TLS EOF.
//
// Phase 57.2 fix: ClusterIPFamily reads the cluster-authoritative
// `io.x-k8s.kinder.ip-family` label, NOT the docker network state. This test
// asserts the helper returns false (IPv4) for an IPv4-labeled cluster AND does
// NOT invoke `docker network inspect` at all.
//
// MUST NOT call t.Parallel().
func TestClusterIPFamily_MacOSDualStackNetwork_IPv4Cluster(t *testing.T) {
	s := &cipState{labelStdout: "ipv4\n"} // cluster IS IPv4-only
	withClusterIPFamilyCmder(t, newCIPCmder(s))

	node := &cipTestNode{name: "kinder-external-load-balancer"}
	got, err := ClusterIPFamily("docker", node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != false {
		t.Errorf("ClusterIPFamily = %v, want false (cluster labeled ipv4)", got)
	}
	if s.sawNetworkInspect {
		t.Fatal("CRITICAL REGRESSION: ClusterIPFamily called docker network inspect — Phase 57.1 bug has returned. Phase 57.2's whole point is that resume must NEVER probe the docker network for IPv6 mode.")
	}
}

// TestClusterIPFamily_LabelAbsent_FailsLoud: when the ip-family label is missing
// (label query returns empty stdout), ClusterIPFamily must fail loudly with a
// "delete and recreate the cluster" message — there is no fallback for
// pre-Phase-57.2 clusters per CONTEXT.md decision.
//
// MUST NOT call t.Parallel().
func TestClusterIPFamily_LabelAbsent_FailsLoud(t *testing.T) {
	s := &cipState{labelStdout: ""} // label missing
	withClusterIPFamilyCmder(t, newCIPCmder(s))

	node := &cipTestNode{name: "kinder-external-load-balancer"}
	got, err := ClusterIPFamily("docker", node)
	if err == nil {
		t.Fatal("expected non-nil error when label is absent, got nil")
	}
	if got != false {
		t.Errorf("ClusterIPFamily first return = %v, want false on error", got)
	}
	if !strings.Contains(err.Error(), "delete and recreate the cluster") {
		t.Errorf("error message does not contain 'delete and recreate the cluster'; got: %v", err)
	}
}

// TestClusterIPFamily_LabelUnrecognized_FailsLoud: when the ip-family label
// has an unrecognized value (not ipv4/ipv6/dual), ClusterIPFamily fails loudly.
// The error message must include the offending value for debuggability AND the
// "delete and recreate" guidance.
//
// MUST NOT call t.Parallel().
func TestClusterIPFamily_LabelUnrecognized_FailsLoud(t *testing.T) {
	s := &cipState{labelStdout: "bogus\n"}
	withClusterIPFamilyCmder(t, newCIPCmder(s))

	node := &cipTestNode{name: "kinder-external-load-balancer"}
	_, err := ClusterIPFamily("docker", node)
	if err == nil {
		t.Fatal("expected non-nil error when label is unrecognized, got nil")
	}
	if !strings.Contains(err.Error(), "delete and recreate the cluster") {
		t.Errorf("error message does not contain 'delete and recreate the cluster'; got: %v", err)
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error message does not contain the offending value 'bogus'; got: %v", err)
	}
}

// TestClusterIPFamily_DockerInspectFails_PropagatesError: when the underlying
// docker inspect command itself fails (binary missing, container gone, etc.),
// ClusterIPFamily wraps and returns the error with diagnostic context.
//
// MUST NOT call t.Parallel().
func TestClusterIPFamily_DockerInspectFails_PropagatesError(t *testing.T) {
	s := &cipState{
		labelStdout: "",
		runErr:      fmt.Errorf("docker inspect: no such container: kinder-external-load-balancer"),
	}
	withClusterIPFamilyCmder(t, newCIPCmder(s))

	node := &cipTestNode{name: "kinder-external-load-balancer"}
	_, err := ClusterIPFamily("docker", node)
	if err == nil {
		t.Fatal("expected non-nil error when docker inspect fails, got nil")
	}
	// Error must mention the binary name (or "docker inspect") and the node name
	// for diagnostic value.
	msg := err.Error()
	if !strings.Contains(msg, "docker") {
		t.Errorf("error should mention 'docker' (or the binary name); got: %v", err)
	}
	if !strings.Contains(msg, "kinder-external-load-balancer") {
		t.Errorf("error should mention node name 'kinder-external-load-balancer'; got: %v", err)
	}
}
