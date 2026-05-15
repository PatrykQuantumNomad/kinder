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

// testNode is a minimal nodes.Node for unit testing WriteDynamicConfig.
// It records every Command(...) invocation so tests can assert against
// the exact shell-exec'd atomic-swap line, and lets per-call errors be
// scripted via the errs slice.
type testNode struct {
	name string
	// calls records each (cmd, args) invocation, in order. Includes the
	// nodeutils.WriteFile mkdir + cp calls AND the final "sh -c chmod && mv && mv" call.
	calls []testCall
	// errs scripts the error returned by the Nth Command(...).Run().
	// If len(errs) < calls executed, missing entries default to nil.
	errs []error
	// stdinByCall captures payloads written to .SetStdin(...) per call (so
	// tests can assert the LDS/CDS YAML content).
	stdinByCall []string
}

type testCall struct {
	cmd  string
	args []string
}

var _ nodes.Node = (*testNode)(nil)

func (n *testNode) String() string                                              { return n.name }
func (n *testNode) Role() (string, error)                                       { return "external-load-balancer", nil }
func (n *testNode) IP() (string, string, error)                                 { return "", "", nil }
func (n *testNode) SerialLogs(_ io.Writer) error                                { return nil }
func (n *testNode) Command(c string, a ...string) exec.Cmd                     { return n.newCmd(c, a) }
func (n *testNode) CommandContext(_ context.Context, c string, a ...string) exec.Cmd { return n.newCmd(c, a) }

func (n *testNode) newCmd(c string, args []string) exec.Cmd {
	idx := len(n.calls)
	n.calls = append(n.calls, testCall{cmd: c, args: append([]string{}, args...)})
	var err error
	if idx < len(n.errs) {
		err = n.errs[idx]
	}
	return &testCmd{node: n, idx: idx, err: err}
}

type testCmd struct {
	node *testNode
	idx  int
	err  error
	in   io.Reader
}

var _ exec.Cmd = (*testCmd)(nil)

func (c *testCmd) Run() error {
	if c.in != nil {
		b, _ := io.ReadAll(c.in)
		for len(c.node.stdinByCall) <= c.idx {
			c.node.stdinByCall = append(c.node.stdinByCall, "")
		}
		c.node.stdinByCall[c.idx] = string(b)
	}
	return c.err
}
func (c *testCmd) SetEnv(_ ...string) exec.Cmd    { return c }
func (c *testCmd) SetStdin(r io.Reader) exec.Cmd  { c.in = r; return c }
func (c *testCmd) SetStdout(_ io.Writer) exec.Cmd { return c }
func (c *testCmd) SetStderr(_ io.Writer) exec.Cmd { return c }

func makeCPs(names ...string) []nodes.Node {
	out := make([]nodes.Node, 0, len(names))
	for _, n := range names {
		out = append(out, &testNode{name: n})
	}
	return out
}

func TestImageConstantIsEnvoy(t *testing.T) {
	t.Parallel()
	const want = "docker.io/envoyproxy/envoy:v1.36.2"
	if Image != want {
		t.Errorf("Image = %q, want %q", Image, want)
	}
}

func TestProxyConfigPathConstants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"ProxyConfigPath", ProxyConfigPath, "/home/envoy/envoy.yaml"},
		{"ProxyConfigPathCDS", ProxyConfigPathCDS, "/home/envoy/cds.yaml"},
		{"ProxyConfigPathLDS", ProxyConfigPathLDS, "/home/envoy/lds.yaml"},
		{"ProxyConfigDir", ProxyConfigDir, "/home/envoy"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestConfigLDSRendersControlPlanePort(t *testing.T) {
	t.Parallel()
	out, err := Config(&ConfigData{ControlPlanePort: 6443, IPv6: false}, ProxyLDSConfigTemplate)
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if !strings.Contains(out, "port_value: 6443") {
		t.Errorf("output does not contain 'port_value: 6443'; got:\n%s", out)
	}
	if !strings.Contains(out, `"0.0.0.0"`) {
		t.Errorf("output does not contain '\"0.0.0.0\"' for IPv4; got:\n%s", out)
	}
}

func TestConfigLDSIPv6(t *testing.T) {
	t.Parallel()
	out, err := Config(&ConfigData{ControlPlanePort: 6443, IPv6: true}, ProxyLDSConfigTemplate)
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if !strings.Contains(out, `"::"`) {
		t.Errorf("output does not contain '\"::\"' for IPv6; got:\n%s", out)
	}
}

// TestConfigLDSIPv6IncludesIPv4Compat (Phase 57.2): the IPv6 listener template
// must render `ipv4_compat: true` under socket_address so the `::` listener
// accepts IPv4-mapped peers. Without this, macOS Docker Desktop's IPv4
// port-mapping cannot reach an IPv6-only listener (host kubectl gets TLS EOF).
func TestConfigLDSIPv6IncludesIPv4Compat(t *testing.T) {
	t.Parallel()
	out, err := Config(&ConfigData{ControlPlanePort: 6443, IPv6: true}, ProxyLDSConfigTemplate)
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if !strings.Contains(out, "ipv4_compat: true") {
		t.Errorf("IPv6 LDS output does not contain 'ipv4_compat: true'; got:\n%s", out)
	}
}

// TestConfigLDSDualStackIncludesIPv4Compat (Phase 57.2): the ClusterIPFamily
// helper collapses `dual` to IPv6=true, so the dual-stack render goes through
// the same template branch as IPv6. Assert ipv4_compat is present so future
// readers see the dual-stack intent symmetric with IPv6.
func TestConfigLDSDualStackIncludesIPv4Compat(t *testing.T) {
	t.Parallel()
	// ConfigData has only one bool (IPv6); dual-stack collapses to IPv6=true.
	out, err := Config(&ConfigData{ControlPlanePort: 6443, IPv6: true}, ProxyLDSConfigTemplate)
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if !strings.Contains(out, "ipv4_compat: true") {
		t.Errorf("dual-stack LDS output does not contain 'ipv4_compat: true'; got:\n%s", out)
	}
}

// TestConfigLDSIPv4DoesNotIncludeIPv4Compat (Phase 57.2): the IPv4-only listener
// binds to "0.0.0.0" and must NOT render `ipv4_compat` — it is a meaningless
// field on an IPv4 socket and would be surprising in the rendered YAML.
func TestConfigLDSIPv4DoesNotIncludeIPv4Compat(t *testing.T) {
	t.Parallel()
	out, err := Config(&ConfigData{ControlPlanePort: 6443, IPv6: false}, ProxyLDSConfigTemplate)
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if strings.Contains(out, "ipv4_compat") {
		t.Errorf("IPv4 LDS output unexpectedly contains 'ipv4_compat'; got:\n%s", out)
	}
}

func TestConfigCDSRendersBackendServers(t *testing.T) {
	t.Parallel()
	data := &ConfigData{
		BackendServers: map[string]string{
			"cp-1": "172.18.0.4:6443",
			"cp-2": "172.18.0.5:6443",
		},
	}
	out, err := Config(data, ProxyCDSConfigTemplate)
	if err != nil {
		t.Fatalf("Config() error: %v", err)
	}
	if !strings.Contains(out, "172.18.0.4") {
		t.Errorf("output does not contain '172.18.0.4'; got:\n%s", out)
	}
	if !strings.Contains(out, "172.18.0.5") {
		t.Errorf("output does not contain '172.18.0.5'; got:\n%s", out)
	}
	if !strings.Contains(out, "health_checks") {
		t.Errorf("output does not contain 'health_checks' block; got:\n%s", out)
	}
}

func TestGenerateBootstrapCommandShape(t *testing.T) {
	t.Parallel()
	args := GenerateBootstrapCommand("my-cluster", "my-cluster-external-load-balancer")
	if len(args) != 3 {
		t.Fatalf("len(args) = %d, want 3; args = %v", len(args), args)
	}
	if args[0] != "bash" {
		t.Errorf("args[0] = %q, want %q", args[0], "bash")
	}
	if args[1] != "-c" {
		t.Errorf("args[1] = %q, want %q", args[1], "-c")
	}
	cmd := args[2]
	for _, substr := range []string{
		"mkdir -p /home/envoy",
		"/home/envoy/envoy.yaml",
		"/home/envoy/cds.yaml",
		"/home/envoy/lds.yaml",
		"while true; do envoy -c",
	} {
		if !strings.Contains(cmd, substr) {
			t.Errorf("args[2] does not contain %q; got:\n%s", substr, cmd)
		}
	}
}

// nodeutils.WriteFile issues TWO node.Command calls per file:
//   1. mkdir -p <dir>
//   2. cp /dev/stdin <dest>   (with stdin payload)
//
// So for WriteDynamicConfig the call sequence is:
//   idx 0: mkdir for LDS tmp
//   idx 1: cp  /dev/stdin → lds.yaml.tmp  (stdin = LDS YAML)
//   idx 2: mkdir for CDS tmp
//   idx 3: cp  /dev/stdin → cds.yaml.tmp  (stdin = CDS YAML)
//   idx 4: sh -c "chmod 666 ... && mv ... && mv ..."

// TestWriteDynamicConfig_HappyIPv4 verifies that WriteDynamicConfig renders
// IPv4 LDS (0.0.0.0 bind) and CDS (V4_PREFERRED dns_lookup_family), writes
// both tmp files via nodeutils.WriteFile, and issues the atomic-swap sh -c
// command that mirrors create-time loadbalancer.go:106-110.
func TestWriteDynamicConfig_HappyIPv4(t *testing.T) {
	lb := &testNode{name: "kinder-lb"}
	cpNodes := makeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	err := WriteDynamicConfig(lb, cpNodes, false)
	if err != nil {
		t.Fatalf("WriteDynamicConfig returned unexpected error: %v", err)
	}

	// Expect 5 calls: mkdir+cp for LDS, mkdir+cp for CDS, sh -c
	if len(lb.calls) < 5 {
		t.Fatalf("expected at least 5 node.Command calls, got %d; calls=%v", len(lb.calls), lb.calls)
	}

	// Last call must be sh -c with the atomic-swap command
	lastCall := lb.calls[len(lb.calls)-1]
	if lastCall.cmd != "sh" {
		t.Errorf("last call cmd = %q, want %q", lastCall.cmd, "sh")
	}
	if len(lastCall.args) < 2 || lastCall.args[0] != "-c" {
		t.Errorf("last call args = %v, want [\"-c\", <cmd>]", lastCall.args)
	}
	shCmd := lastCall.args[1]
	for _, substr := range []string{"chmod 666", "mv", "lds.yaml", "cds.yaml"} {
		if !strings.Contains(shCmd, substr) {
			t.Errorf("sh -c arg does not contain %q; got:\n%s", substr, shCmd)
		}
	}

	// LDS stdin (idx 1 cp call) must bind to 0.0.0.0 (IPv4) not ::
	ldsStdin := lb.stdinByCall[1]
	if !strings.Contains(ldsStdin, `"0.0.0.0"`) {
		t.Errorf("LDS stdin does not contain '\"0.0.0.0\"' for IPv4; got:\n%s", ldsStdin)
	}
	if strings.Contains(ldsStdin, `"::"`) {
		t.Errorf("LDS stdin unexpectedly contains '\"::\"'; got:\n%s", ldsStdin)
	}

	// CDS stdin (idx 3 cp call) must use V4_PREFERRED not AUTO
	cdsStdin := lb.stdinByCall[3]
	if !strings.Contains(cdsStdin, "V4_PREFERRED") {
		t.Errorf("CDS stdin does not contain 'V4_PREFERRED'; got:\n%s", cdsStdin)
	}
	if strings.Contains(cdsStdin, "AUTO") {
		t.Errorf("CDS stdin unexpectedly contains 'AUTO'; got:\n%s", cdsStdin)
	}

	// CDS stdin must contain all 3 CP container names
	for _, cp := range []string{"kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3"} {
		if !strings.Contains(cdsStdin, cp) {
			t.Errorf("CDS stdin does not contain CP name %q; got:\n%s", cp, cdsStdin)
		}
	}
}

// TestWriteDynamicConfig_HappyIPv6 verifies that WriteDynamicConfig renders
// IPv6 LDS (:: bind) and CDS (AUTO dns_lookup_family) when ipv6=true.
func TestWriteDynamicConfig_HappyIPv6(t *testing.T) {
	lb := &testNode{name: "kinder-lb"}
	cpNodes := makeCPs("kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3")

	err := WriteDynamicConfig(lb, cpNodes, true)
	if err != nil {
		t.Fatalf("WriteDynamicConfig returned unexpected error: %v", err)
	}

	if len(lb.calls) < 5 {
		t.Fatalf("expected at least 5 node.Command calls, got %d", len(lb.calls))
	}

	// LDS stdin must bind to :: (IPv6) not 0.0.0.0
	ldsStdin := lb.stdinByCall[1]
	if !strings.Contains(ldsStdin, `"::"`) {
		t.Errorf("LDS stdin does not contain '\"::\"' for IPv6; got:\n%s", ldsStdin)
	}
	if strings.Contains(ldsStdin, `"0.0.0.0"`) {
		t.Errorf("LDS stdin unexpectedly contains '\"0.0.0.0\"'; got:\n%s", ldsStdin)
	}

	// CDS stdin must use AUTO not V4_PREFERRED
	cdsStdin := lb.stdinByCall[3]
	if !strings.Contains(cdsStdin, "AUTO") {
		t.Errorf("CDS stdin does not contain 'AUTO'; got:\n%s", cdsStdin)
	}
	if strings.Contains(cdsStdin, "V4_PREFERRED") {
		t.Errorf("CDS stdin unexpectedly contains 'V4_PREFERRED'; got:\n%s", cdsStdin)
	}

	// CDS stdin must still contain all 3 CP container names
	for _, cp := range []string{"kinder-control-plane", "kinder-control-plane2", "kinder-control-plane3"} {
		if !strings.Contains(cdsStdin, cp) {
			t.Errorf("CDS stdin does not contain CP name %q; got:\n%s", cp, cdsStdin)
		}
	}
}

// TestWriteDynamicConfig_WriteFileError verifies that when the first
// node.Command (mkdir for LDS tmp dir) returns an error, WriteDynamicConfig
// returns a wrapped error containing "failed to copy LDS config to load
// balancer node" and the original error text, and does NOT proceed to the
// atomic-swap step.
func TestWriteDynamicConfig_WriteFileError(t *testing.T) {
	lb := &testNode{
		name: "kinder-lb",
		errs: []error{fmt.Errorf("disk full")},
	}

	err := WriteDynamicConfig(lb, makeCPs("cp1", "cp2", "cp3"), false)
	if err == nil {
		t.Fatal("expected non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to copy LDS config to load balancer node") {
		t.Errorf("error does not contain expected wrapper text; got: %v", err)
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error does not contain original error text 'disk full'; got: %v", err)
	}
	// Function must have returned early — only 1 call made (the failing mkdir)
	if len(lb.calls) != 1 {
		t.Errorf("expected 1 node.Command call (early return after mkdir error), got %d; calls=%v", len(lb.calls), lb.calls)
	}
}

// TestWriteDynamicConfig_MvError verifies that when the sh -c atomic-swap
// command (5th node.Command call) returns an error, WriteDynamicConfig
// returns a wrapped error containing "failed to reload Envoy load balancer
// config" and the original error text.
func TestWriteDynamicConfig_MvError(t *testing.T) {
	lb := &testNode{
		name: "kinder-lb",
		// errs indices: 0=mkdir LDS, 1=cp LDS, 2=mkdir CDS, 3=cp CDS, 4=sh -c
		errs: []error{nil, nil, nil, nil, fmt.Errorf("permission denied")},
	}

	err := WriteDynamicConfig(lb, makeCPs("cp1", "cp2", "cp3"), false)
	if err == nil {
		t.Fatal("expected non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to reload Envoy load balancer config") {
		t.Errorf("error does not contain expected wrapper text; got: %v", err)
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error does not contain original error text 'permission denied'; got: %v", err)
	}
	// All 5 calls must have been made (function reached sh -c before failing)
	if len(lb.calls) != 5 {
		t.Errorf("expected 5 node.Command calls, got %d; calls=%v", len(lb.calls), lb.calls)
	}
}
