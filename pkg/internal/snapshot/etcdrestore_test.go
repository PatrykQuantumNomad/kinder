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

package snapshot

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
)

// ---------------------------------------------------------------------------
// Test-only node/cmd fakes for the snapshot package
// ---------------------------------------------------------------------------

// snapFakeNode is a nodes.Node whose Command/CommandContext calls are routed
// through a per-node lookup function so each test can program exact responses.
type snapFakeNode struct {
	name   string
	role   string
	mu     sync.Mutex
	calls  []snapCall // recorded in call order
	lookup func(name string, args []string) (string, error)
}

type snapCall struct {
	name string
	args []string
	// stdin holds the bytes passed via SetStdin, captured lazily when Run() is called.
	stdin []byte
}

var _ nodes.Node = (*snapFakeNode)(nil)

func (n *snapFakeNode) String() string                    { return n.name }
func (n *snapFakeNode) Role() (string, error)             { return n.role, nil }
func (n *snapFakeNode) IP() (string, string, error)       { return "", "", nil }
func (n *snapFakeNode) SerialLogs(_ io.Writer) error      { return nil }

func (n *snapFakeNode) Command(c string, a ...string) exec.Cmd {
	return n.CommandContext(context.Background(), c, a...)
}

func (n *snapFakeNode) CommandContext(_ context.Context, c string, a ...string) exec.Cmd {
	argsCopy := make([]string, len(a))
	copy(argsCopy, a)
	return &snapFakeCmd{
		node: n,
		name: c,
		args: argsCopy,
	}
}

func (n *snapFakeNode) recordCall(c snapCall) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, c)
}

func (n *snapFakeNode) snapshot() []snapCall {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]snapCall, len(n.calls))
	copy(out, n.calls)
	return out
}

// snapFakeCmd implements exec.Cmd and captures stdin bytes before delegating
// to the node's lookup for the return value.
type snapFakeCmd struct {
	node     *snapFakeNode
	name     string
	args     []string
	stdinBuf []byte
	stdoutW  io.Writer
}

var _ exec.Cmd = (*snapFakeCmd)(nil)

func (c *snapFakeCmd) Run() error {
	call := snapCall{name: c.name, args: c.args, stdin: c.stdinBuf}
	c.node.recordCall(call)
	stdout := ""
	var err error
	if c.node.lookup != nil {
		stdout, err = c.node.lookup(c.name, c.args)
	}
	if c.stdoutW != nil && stdout != "" {
		_, _ = io.WriteString(c.stdoutW, stdout)
	}
	return err
}

func (c *snapFakeCmd) SetEnv(_ ...string) exec.Cmd    { return c }
func (c *snapFakeCmd) SetStderr(_ io.Writer) exec.Cmd { return c }
func (c *snapFakeCmd) SetStdout(w io.Writer) exec.Cmd  { c.stdoutW = w; return c }
func (c *snapFakeCmd) SetStdin(r io.Reader) exec.Cmd {
	if r != nil {
		data, _ := io.ReadAll(r)
		c.stdinBuf = data
	}
	return c
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// callNames returns the binary name of each recorded call in order.
func callNames(calls []snapCall) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.name
	}
	return out
}

// findCallIdx returns the index of the first call matching predicate, or -1.
func findCallIdx(calls []snapCall, pred func(snapCall) bool) int {
	for i, c := range calls {
		if pred(c) {
			return i
		}
	}
	return -1
}

// hasArg reports whether the call's args contain the exact string s.
func hasArg(c snapCall, s string) bool {
	for _, a := range c.args {
		if a == s {
			return true
		}
	}
	return false
}

// hasArgPrefix reports whether any arg has the given prefix.
func hasArgPrefix(c snapCall, prefix string) bool {
	for _, a := range c.args {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

// withZeroSettleDelay replaces etcdManifestSettleDelay with 0 for the duration
// of the test so tests don't spend 5 seconds per CP waiting for kubelet.
func withZeroSettleDelay(t *testing.T) {
	t.Helper()
	prev := etcdManifestSettleDelay
	etcdManifestSettleDelay = 0
	t.Cleanup(func() { etcdManifestSettleDelay = prev })
}

// writeSnapFile creates a temp file with content and returns its path.
func writeSnapFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "etcd-*.snap")
	if err != nil {
		t.Fatalf("create temp snap file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write snap file: %v", err)
	}
	f.Close()
	return f.Name()
}

// ---------------------------------------------------------------------------
// TestRestoreEtcdSingleCP_Sequence
// ---------------------------------------------------------------------------

// TestRestoreEtcdSingleCP_Sequence verifies that RestoreEtcd on a single
// control-plane node issues commands in the exact expected order:
//  1. mv manifest aside
//  2. cp snap to node
//  3. etcdctl snapshot restore
//  4. mv /var/lib/etcd → /var/lib/etcd.kinder-old
//  5. mv /var/lib/etcd-restored → /var/lib/etcd
//  6. mv manifest back
//  7. rm tmp snap
func TestRestoreEtcdSingleCP_Sequence(t *testing.T) {
	withZeroSettleDelay(t)
	snapPath := writeSnapFile(t, "fake-etcd-snapshot")

	cp := &snapFakeNode{
		name: "kind-control-plane",
		role: "control-plane",
		lookup: func(name string, args []string) (string, error) {
			return "", nil // all commands succeed
		},
	}

	opts := EtcdRestoreOptions{
		CPs:              []nodes.Node{cp},
		SnapshotHostPath: snapPath,
		ProviderBin:      "docker",
	}
	if err := RestoreEtcd(context.Background(), opts); err != nil {
		t.Fatalf("RestoreEtcd returned unexpected error: %v", err)
	}

	calls := cp.snapshot()
	if len(calls) == 0 {
		t.Fatal("expected commands to be recorded; got none")
	}

	// Verify exact ordering by finding sentinel calls
	mvManifestAside := findCallIdx(calls, func(c snapCall) bool {
		// args: ["/etc/kubernetes/manifests/etcd.yaml", "/tmp/etcd-manifest.yaml.kinder-snap"]
		return c.name == "mv" &&
			len(c.args) == 2 &&
			c.args[0] == "/etc/kubernetes/manifests/etcd.yaml" &&
			c.args[1] == "/tmp/etcd-manifest.yaml.kinder-snap"
	})
	cpSnap := findCallIdx(calls, func(c snapCall) bool {
		return c.name == "cp" &&
			hasArg(c, "/tmp/kinder-restore.snap")
	})
	etcdctlRestore := findCallIdx(calls, func(c snapCall) bool {
		return c.name == "etcdctl" &&
			hasArg(c, "snapshot") && hasArg(c, "restore")
	})
	mvOld := findCallIdx(calls, func(c snapCall) bool {
		return c.name == "mv" &&
			len(c.args) == 2 &&
			c.args[0] == "/var/lib/etcd" &&
			c.args[1] == "/var/lib/etcd.kinder-old"
	})
	mvRestored := findCallIdx(calls, func(c snapCall) bool {
		return c.name == "mv" &&
			len(c.args) == 2 &&
			c.args[0] == "/var/lib/etcd-restored" &&
			c.args[1] == "/var/lib/etcd"
	})
	mvManifestBack := findCallIdx(calls, func(c snapCall) bool {
		// args: ["/tmp/etcd-manifest.yaml.kinder-snap", "/etc/kubernetes/manifests/etcd.yaml"]
		return c.name == "mv" &&
			len(c.args) == 2 &&
			c.args[0] == "/tmp/etcd-manifest.yaml.kinder-snap" &&
			c.args[1] == "/etc/kubernetes/manifests/etcd.yaml"
	})
	rmTmp := findCallIdx(calls, func(c snapCall) bool {
		return c.name == "rm" && hasArg(c, "/tmp/kinder-restore.snap")
	})

	for name, idx := range map[string]int{
		"mv manifest aside": mvManifestAside,
		"cp snap":           cpSnap,
		"etcdctl restore":   etcdctlRestore,
		"mv etcd→old":       mvOld,
		"mv restored→etcd":  mvRestored,
		"mv manifest back":  mvManifestBack,
		"rm tmp snap":       rmTmp,
	} {
		if idx < 0 {
			t.Errorf("expected call %q not found in recorded calls: %v", name, calls)
		}
	}

	// Ordering constraints.
	// Note: manifest-back is a deferred call that fires AFTER the function
	// returns, so it comes AFTER rm tmp snap in call order.
	type orderCheck struct {
		before    string
		beforeIdx int
		after     string
		afterIdx  int
	}
	checks := []orderCheck{
		{"mv manifest aside", mvManifestAside, "cp snap", cpSnap},
		{"cp snap", cpSnap, "etcdctl restore", etcdctlRestore},
		{"etcdctl restore", etcdctlRestore, "mv etcd→old", mvOld},
		{"mv etcd→old", mvOld, "mv restored→etcd", mvRestored},
		{"mv restored→etcd", mvRestored, "rm tmp snap", rmTmp},
		{"rm tmp snap", rmTmp, "mv manifest back", mvManifestBack},
	}
	for _, oc := range checks {
		if oc.beforeIdx >= 0 && oc.afterIdx >= 0 && oc.beforeIdx >= oc.afterIdx {
			t.Errorf("expected %q (idx=%d) before %q (idx=%d)", oc.before, oc.beforeIdx, oc.after, oc.afterIdx)
		}
	}

	// Single CP: --initial-cluster should be "kind-control-plane=https://127.0.0.1:2380"
	if etcdctlRestore >= 0 {
		c := calls[etcdctlRestore]
		if !hasArgPrefix(c, "--initial-cluster=") {
			t.Errorf("etcdctl restore missing --initial-cluster flag; args: %v", c.args)
		}
		if !hasArgPrefix(c, "--initial-advertise-peer-urls=") {
			t.Errorf("etcdctl restore missing --initial-advertise-peer-urls flag; args: %v", c.args)
		}
		if !hasArgPrefix(c, "--initial-cluster-token=") {
			t.Errorf("etcdctl restore missing --initial-cluster-token flag; args: %v", c.args)
		}
	}
}

// ---------------------------------------------------------------------------
// TestRestoreEtcdHA_SameToken
// ---------------------------------------------------------------------------

// TestRestoreEtcdHA_SameToken verifies that when restoring 3 control-plane
// nodes, all 3 etcdctl invocations use the IDENTICAL --initial-cluster-token
// value AND the identical --initial-cluster membership string.
func TestRestoreEtcdHA_SameToken(t *testing.T) {
	withZeroSettleDelay(t)
	snapPath := writeSnapFile(t, "fake-etcd-snapshot-ha")

	// For HA, the implementation needs container IPs via "docker inspect".
	// We simulate that via node.Command("docker", "inspect", ...) by providing
	// a lookup that returns a fake IP per node name.
	ipFor := map[string]string{
		"cp1": "172.18.0.2",
		"cp2": "172.18.0.3",
		"cp3": "172.18.0.4",
	}

	makeLookup := func(nodeName string) func(string, []string) (string, error) {
		return func(name string, args []string) (string, error) {
			// inspect call returns IP
			if strings.Contains(strings.Join(args, " "), "inspect") {
				return ipFor[nodeName] + "\n", nil
			}
			return "", nil
		}
	}

	cp1 := &snapFakeNode{name: "cp1", role: "control-plane", lookup: makeLookup("cp1")}
	cp2 := &snapFakeNode{name: "cp2", role: "control-plane", lookup: makeLookup("cp2")}
	cp3 := &snapFakeNode{name: "cp3", role: "control-plane", lookup: makeLookup("cp3")}

	opts := EtcdRestoreOptions{
		CPs:              []nodes.Node{cp1, cp2, cp3},
		SnapshotHostPath: snapPath,
		ProviderBin:      "docker",
	}
	if err := RestoreEtcd(context.Background(), opts); err != nil {
		t.Fatalf("RestoreEtcd HA returned unexpected error: %v", err)
	}

	// Extract etcdctl restore calls from each CP.
	getEtcdctlArgs := func(n *snapFakeNode) []string {
		for _, c := range n.snapshot() {
			if c.name == "etcdctl" && hasArg(c, "snapshot") && hasArg(c, "restore") {
				return c.args
			}
		}
		return nil
	}

	args1 := getEtcdctlArgs(cp1)
	args2 := getEtcdctlArgs(cp2)
	args3 := getEtcdctlArgs(cp3)

	for i, args := range [][]string{args1, args2, args3} {
		if args == nil {
			t.Fatalf("cp%d: no etcdctl snapshot restore call recorded", i+1)
		}
	}

	// Extract --initial-cluster-token from each
	getFlag := func(args []string, prefix string) string {
		for _, a := range args {
			if strings.HasPrefix(a, prefix) {
				return strings.TrimPrefix(a, prefix)
			}
		}
		return ""
	}

	token1 := getFlag(args1, "--initial-cluster-token=")
	token2 := getFlag(args2, "--initial-cluster-token=")
	token3 := getFlag(args3, "--initial-cluster-token=")

	if token1 == "" {
		t.Error("cp1: missing --initial-cluster-token")
	}
	if token1 != token2 || token1 != token3 {
		t.Errorf("HA: cluster tokens differ: cp1=%q cp2=%q cp3=%q (all must be identical)", token1, token2, token3)
	}

	// Extract --initial-cluster from each; all must be identical
	cluster1 := getFlag(args1, "--initial-cluster=")
	cluster2 := getFlag(args2, "--initial-cluster=")
	cluster3 := getFlag(args3, "--initial-cluster=")

	if cluster1 == "" {
		t.Error("cp1: missing --initial-cluster")
	}
	if cluster1 != cluster2 || cluster1 != cluster3 {
		t.Errorf("HA: initial-cluster strings differ: cp1=%q cp2=%q cp3=%q", cluster1, cluster2, cluster3)
	}
}

// ---------------------------------------------------------------------------
// TestRestoreEtcdSnapshotRestoreFails_ManifestRestored
// ---------------------------------------------------------------------------

// TestRestoreEtcdSnapshotRestoreFails_ManifestRestored verifies that when
// etcdctl exits non-zero, the manifest is moved BACK (rollback) and an
// error mentioning etcdctl is returned.
func TestRestoreEtcdSnapshotRestoreFails_ManifestRestored(t *testing.T) {
	withZeroSettleDelay(t)
	snapPath := writeSnapFile(t, "fake-snap")

	cp := &snapFakeNode{
		name: "kind-control-plane",
		role: "control-plane",
		lookup: func(name string, args []string) (string, error) {
			if name == "etcdctl" {
				return "", fmt.Errorf("exit status 1: etcdctl snapshot restore failed")
			}
			return "", nil
		},
	}

	opts := EtcdRestoreOptions{
		CPs:              []nodes.Node{cp},
		SnapshotHostPath: snapPath,
		ProviderBin:      "docker",
	}
	err := RestoreEtcd(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error from RestoreEtcd when etcdctl fails; got nil")
	}
	if !strings.Contains(err.Error(), "etcdctl") && !strings.Contains(err.Error(), "restore") {
		t.Errorf("error should mention etcdctl or restore; got: %v", err)
	}

	calls := cp.snapshot()
	// Manifest should have been moved back (defer runs: mv kinder-snap → etcd.yaml)
	mvBack := findCallIdx(calls, func(c snapCall) bool {
		return c.name == "mv" &&
			len(c.args) == 2 &&
			c.args[0] == "/tmp/etcd-manifest.yaml.kinder-snap" &&
			c.args[1] == "/etc/kubernetes/manifests/etcd.yaml"
	})
	if mvBack < 0 {
		t.Errorf("manifest was NOT moved back after etcdctl failure; calls: %v", calls)
	}
}

// ---------------------------------------------------------------------------
// TestRestoreEtcdDataSwapFails_OldDataRestored
// ---------------------------------------------------------------------------

// TestRestoreEtcdDataSwapFails_OldDataRestored verifies that when the second
// mv (mv /var/lib/etcd-restored /var/lib/etcd) fails, the rollback
// (mv /var/lib/etcd.kinder-old /var/lib/etcd) is attempted and an error
// is returned.
func TestRestoreEtcdDataSwapFails_OldDataRestored(t *testing.T) {
	withZeroSettleDelay(t)
	snapPath := writeSnapFile(t, "fake-snap")

	cp := &snapFakeNode{
		name: "kind-control-plane",
		role: "control-plane",
		lookup: func(name string, args []string) (string, error) {
			// Fail the second mv in the data-dir swap sequence:
			// "mv /var/lib/etcd-restored /var/lib/etcd"
			if name == "mv" &&
				len(args) == 2 &&
				args[0] == "/var/lib/etcd-restored" &&
				args[1] == "/var/lib/etcd" {
				return "", fmt.Errorf("simulated mv failure")
			}
			return "", nil
		},
	}

	opts := EtcdRestoreOptions{
		CPs:              []nodes.Node{cp},
		SnapshotHostPath: snapPath,
		ProviderBin:      "docker",
	}
	err := RestoreEtcd(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error when data-dir swap fails; got nil")
	}

	calls := cp.snapshot()
	// Rollback: mv /var/lib/etcd.kinder-old /var/lib/etcd must have been attempted
	rollback := findCallIdx(calls, func(c snapCall) bool {
		return c.name == "mv" &&
			len(c.args) == 2 &&
			c.args[0] == "/var/lib/etcd.kinder-old" &&
			c.args[1] == "/var/lib/etcd"
	})
	if rollback < 0 {
		t.Errorf("expected rollback mv (/var/lib/etcd.kinder-old -> /var/lib/etcd) was not called; calls: %v", calls)
	}
}

// ---------------------------------------------------------------------------
// TestRestoreEtcdContainerIPLookup
// ---------------------------------------------------------------------------

// TestRestoreEtcdContainerIPLookup verifies that when two CP nodes exist,
// the container IP returned by docker inspect is used in
// --initial-advertise-peer-urls.
func TestRestoreEtcdContainerIPLookup(t *testing.T) {
	withZeroSettleDelay(t)
	snapPath := writeSnapFile(t, "fake-snap")

	const fakeIP = "172.18.0.5"

	makeLookup := func(ip string) func(string, []string) (string, error) {
		return func(name string, args []string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "inspect") {
				return ip + "\n", nil
			}
			return "", nil
		}
	}

	cp1 := &snapFakeNode{name: "cp1", role: "control-plane", lookup: makeLookup(fakeIP)}
	cp2 := &snapFakeNode{name: "cp2", role: "control-plane", lookup: makeLookup("172.18.0.6")}

	opts := EtcdRestoreOptions{
		CPs:              []nodes.Node{cp1, cp2},
		SnapshotHostPath: snapPath,
		ProviderBin:      "docker",
	}
	if err := RestoreEtcd(context.Background(), opts); err != nil {
		t.Fatalf("RestoreEtcd returned error: %v", err)
	}

	// cp1 should have used fakeIP in --initial-advertise-peer-urls
	for _, c := range cp1.snapshot() {
		if c.name == "etcdctl" && hasArg(c, "snapshot") && hasArg(c, "restore") {
			peerURLs := ""
			for _, a := range c.args {
				if strings.HasPrefix(a, "--initial-advertise-peer-urls=") {
					peerURLs = a
					break
				}
			}
			if !strings.Contains(peerURLs, fakeIP) {
				t.Errorf("cp1 --initial-advertise-peer-urls should contain %q; got %q", fakeIP, peerURLs)
			}
			return
		}
	}
	t.Error("no etcdctl snapshot restore call found for cp1")
}
