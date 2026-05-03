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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// --- Pause-specific test fakes ---

// recordingCmder records every invocation made through it. Each call returns
// a fakeCmd whose error and stdout are taken from the provided lookup function
// so individual tests can program per-container behavior.
type recordingCmder struct {
	mu    sync.Mutex
	calls []recordedCall
	// lookup returns the (stdout, err) pair for a given (name, args) call.
	// If lookup is nil the call always succeeds with empty stdout.
	lookup func(name string, args []string) (string, error)
}

type recordedCall struct {
	name string
	args []string
}

func (r *recordingCmder) cmder() Cmder {
	return func(name string, args ...string) exec.Cmd {
		r.mu.Lock()
		argsCopy := make([]string, len(args))
		copy(argsCopy, args)
		r.calls = append(r.calls, recordedCall{name: name, args: argsCopy})
		r.mu.Unlock()
		stdout := ""
		var err error
		if r.lookup != nil {
			stdout, err = r.lookup(name, args)
		}
		return &fakeCmd{stdout: stdout, err: err}
	}
}

func (r *recordingCmder) snapshot() []recordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// stopCallNames returns the container name (last arg) for every recorded
// `stop` invocation, in invocation order.
func stopCallNames(calls []recordedCall) []string {
	out := []string{}
	for _, c := range calls {
		if len(c.args) >= 1 && c.args[0] == "stop" {
			out = append(out, c.args[len(c.args)-1])
		}
	}
	return out
}

// indexOf returns the first index of name in s or -1 if not present.
func indexOf(s []string, name string) int {
	for i, v := range s {
		if v == name {
			return i
		}
	}
	return -1
}

// nodeLister-style stub used by Pause to discover cluster contents without a
// real provider. Pause receives a *cluster.Provider in production; tests use
// the package-private nodeFetcher injection point so we can avoid building a
// fake *cluster.Provider (which would need a real internal provider).
type fakePauseNodeFetcher struct {
	nodes []nodes.Node
	err   error
}

func (f *fakePauseNodeFetcher) ListNodes(_ string) ([]nodes.Node, error) {
	return f.nodes, f.err
}

// commandRecordingNode wraps fakeNode so that calls to Command() are routed
// through the package-level defaultCmder (which tests can swap). This is what
// lets us assert that the bootstrap CP node received a snapshot-write
// invocation.
type commandRecordingNode struct {
	fakeNode
}

func (n *commandRecordingNode) Command(c string, a ...string) exec.Cmd {
	return defaultCmder(c, a...)
}
func (n *commandRecordingNode) CommandContext(_ context.Context, c string, a ...string) exec.Cmd {
	return defaultCmder(c, a...)
}

// withFetcher swaps the package-level pauseNodeFetcher for the duration of t.
func withFetcher(t *testing.T, f nodeFetcher) {
	t.Helper()
	prev := pauseNodeFetcher
	pauseNodeFetcher = f
	t.Cleanup(func() { pauseNodeFetcher = prev })
}

// withBinaryName swaps the package-level pauseBinaryName for the duration of t.
func withBinaryName(t *testing.T, name string) {
	t.Helper()
	prev := pauseBinaryName
	pauseBinaryName = func() string { return name }
	t.Cleanup(func() { pauseBinaryName = prev })
}

// stateLookupCmder returns a Cmder lookup that programs `inspect ... <name>`
// stdout per container name from `states`, and treats any other call (stop,
// exec, sh) as success. snapshotErr (when non-nil) is returned for any
// etcdctl call to simulate a failed snapshot.
func stateLookupCmder(states map[string]string) func(_ string, args []string) (string, error) {
	return func(_ string, args []string) (string, error) {
		// inspect --format {{.State.Status}} <name>
		if len(args) >= 1 && args[0] == "inspect" {
			name := args[len(args)-1]
			if s, ok := states[name]; ok {
				return s + "\n", nil
			}
			return "running\n", nil
		}
		return "", nil
	}
}

// --- Pause behavior tests ---

// TestPause_OrderWorkersBeforeCP: 2 workers + 2 CP, capture order of `docker
// stop` invocations → all worker names appear before any CP name.
func TestPause_OrderWorkersBeforeCP(t *testing.T) {
	withBinaryName(t, "docker")
	withFetcher(t, &fakePauseNodeFetcher{
		nodes: []nodes.Node{
			&fakeNode{name: "kind-control-plane", role: "control-plane"},
			&fakeNode{name: "kind-control-plane2", role: "control-plane"},
			&fakeNode{name: "kind-worker", role: "worker"},
			&fakeNode{name: "kind-worker2", role: "worker"},
		},
	})
	rec := &recordingCmder{lookup: stateLookupCmder(map[string]string{
		"kind-control-plane":  "running",
		"kind-control-plane2": "running",
		"kind-worker":         "running",
		"kind-worker2":        "running",
	})}
	withCmder(t, rec.cmder())

	_, err := Pause(PauseOptions{
		ClusterName: "kind",
		Logger:      log.NoopLogger{},
	})
	if err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}

	stops := stopCallNames(rec.snapshot())
	wIdx := indexOf(stops, "kind-worker")
	w2Idx := indexOf(stops, "kind-worker2")
	cpIdx := indexOf(stops, "kind-control-plane")
	cp2Idx := indexOf(stops, "kind-control-plane2")
	for _, idx := range []int{wIdx, w2Idx, cpIdx, cp2Idx} {
		if idx < 0 {
			t.Fatalf("missing stop call for one of nodes; got order: %v", stops)
		}
	}
	maxWorker := wIdx
	if w2Idx > maxWorker {
		maxWorker = w2Idx
	}
	minCP := cpIdx
	if cp2Idx < minCP {
		minCP = cp2Idx
	}
	if maxWorker >= minCP {
		t.Errorf("workers must stop before CP; order: %v (last worker idx=%d, first cp idx=%d)", stops, maxWorker, minCP)
	}
}

// TestPause_OrderCPBeforeLB_HA: 3 CP + 1 LB → all CP stop calls appear before
// LB stop call.
func TestPause_OrderCPBeforeLB_HA(t *testing.T) {
	withBinaryName(t, "docker")
	withFetcher(t, &fakePauseNodeFetcher{
		nodes: []nodes.Node{
			&commandRecordingNode{fakeNode: fakeNode{name: "cp1", role: "control-plane"}},
			&commandRecordingNode{fakeNode: fakeNode{name: "cp2", role: "control-plane"}},
			&commandRecordingNode{fakeNode: fakeNode{name: "cp3", role: "control-plane"}},
			&fakeNode{name: "lb", role: "external-load-balancer"},
		},
	})
	rec := &recordingCmder{lookup: stateLookupCmder(map[string]string{
		"cp1": "running", "cp2": "running", "cp3": "running", "lb": "running",
	})}
	withCmder(t, rec.cmder())

	_, err := Pause(PauseOptions{
		ClusterName: "kind",
		Logger:      log.NoopLogger{},
	})
	if err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}

	stops := stopCallNames(rec.snapshot())
	lbIdx := indexOf(stops, "lb")
	if lbIdx < 0 {
		t.Fatalf("no stop call for lb; got: %v", stops)
	}
	for _, name := range []string{"cp1", "cp2", "cp3"} {
		idx := indexOf(stops, name)
		if idx < 0 {
			t.Fatalf("no stop call for %s; got: %v", name, stops)
		}
		if idx >= lbIdx {
			t.Errorf("CP %s (idx=%d) must stop before lb (idx=%d); order: %v", name, idx, lbIdx, stops)
		}
	}
}

// TestPause_OrderSingleNode: 1 CP only → exactly one stop call for that node.
func TestPause_OrderSingleNode(t *testing.T) {
	withBinaryName(t, "docker")
	withFetcher(t, &fakePauseNodeFetcher{
		nodes: []nodes.Node{
			&fakeNode{name: "kind-control-plane", role: "control-plane"},
		},
	})
	rec := &recordingCmder{lookup: stateLookupCmder(map[string]string{
		"kind-control-plane": "running",
	})}
	withCmder(t, rec.cmder())

	_, err := Pause(PauseOptions{
		ClusterName: "kind",
		Logger:      log.NoopLogger{},
	})
	if err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}

	stops := stopCallNames(rec.snapshot())
	if len(stops) != 1 || stops[0] != "kind-control-plane" {
		t.Errorf("expected exactly one stop call for kind-control-plane; got %v", stops)
	}
}

// TestPause_TimeoutFlag: PauseOptions{Timeout: 45s} → recorded `docker stop
// --time=45 ...` (NOT `--time=30`).
func TestPause_TimeoutFlag(t *testing.T) {
	withBinaryName(t, "docker")
	withFetcher(t, &fakePauseNodeFetcher{
		nodes: []nodes.Node{
			&fakeNode{name: "n1", role: "control-plane"},
		},
	})
	rec := &recordingCmder{lookup: stateLookupCmder(map[string]string{"n1": "running"})}
	withCmder(t, rec.cmder())

	_, err := Pause(PauseOptions{
		ClusterName: "kind",
		Timeout:     45 * time.Second,
		Logger:      log.NoopLogger{},
	})
	if err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}

	found := false
	for _, c := range rec.snapshot() {
		if len(c.args) >= 1 && c.args[0] == "stop" {
			for _, a := range c.args {
				if a == "--time=45" {
					found = true
				}
				if a == "--time=30" {
					t.Errorf("expected --time=45, but found default --time=30; args: %v", c.args)
				}
			}
		}
	}
	if !found {
		t.Errorf("did not find --time=45 in any stop call; calls: %v", rec.snapshot())
	}
}

// TestPause_BestEffortContinuesOnFailure: 3 nodes, fake Cmder fails for node 2
// → all 3 stop calls attempted; PauseResult.Nodes has Success=false for node 2
// with non-empty Error; Pause returns aggregated error but Result is still
// populated for all 3 nodes.
func TestPause_BestEffortContinuesOnFailure(t *testing.T) {
	withBinaryName(t, "docker")
	withFetcher(t, &fakePauseNodeFetcher{
		nodes: []nodes.Node{
			&fakeNode{name: "n1", role: "worker"},
			&fakeNode{name: "n2", role: "worker"},
			&fakeNode{name: "n3", role: "worker"},
		},
	})
	rec := &recordingCmder{lookup: func(_ string, args []string) (string, error) {
		if args[0] == "inspect" {
			return "running\n", nil
		}
		if args[0] == "stop" {
			name := args[len(args)-1]
			if name == "n2" {
				return "", fmt.Errorf("simulated stop failure for n2")
			}
		}
		return "", nil
	}}
	withCmder(t, rec.cmder())

	result, err := Pause(PauseOptions{
		ClusterName: "kind",
		Logger:      log.NoopLogger{},
	})
	if err == nil {
		t.Fatalf("expected aggregated error from Pause, got nil")
	}
	if result == nil {
		t.Fatalf("expected non-nil PauseResult even on partial failure")
	}
	if len(result.Nodes) != 3 {
		t.Fatalf("expected 3 NodeResults, got %d", len(result.Nodes))
	}
	stops := stopCallNames(rec.snapshot())
	for _, name := range []string{"n1", "n2", "n3"} {
		if indexOf(stops, name) < 0 {
			t.Errorf("expected stop call for %s; got %v", name, stops)
		}
	}
	for _, nr := range result.Nodes {
		if nr.Name == "n2" {
			if nr.Success {
				t.Errorf("expected Success=false for n2; got true")
			}
			if nr.Error == "" {
				t.Errorf("expected non-empty Error for n2")
			}
		} else {
			if !nr.Success {
				t.Errorf("expected Success=true for %s; got false (err=%q)", nr.Name, nr.Error)
			}
		}
	}
}

// TestPause_IdempotentNoOp: ContainerState reports all nodes "exited" →
// returns PauseResult with State="paused", AlreadyPaused=true; emits NO docker
// stop calls.
func TestPause_IdempotentNoOp(t *testing.T) {
	withBinaryName(t, "docker")
	withFetcher(t, &fakePauseNodeFetcher{
		nodes: []nodes.Node{
			&fakeNode{name: "n1", role: "worker"},
			&fakeNode{name: "n2", role: "control-plane"},
		},
	})
	rec := &recordingCmder{lookup: stateLookupCmder(map[string]string{
		"n1": "exited", "n2": "exited",
	})}
	withCmder(t, rec.cmder())

	result, err := Pause(PauseOptions{
		ClusterName: "kind",
		Logger:      log.NoopLogger{},
	})
	if err != nil {
		t.Fatalf("idempotent no-op should not error; got %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil PauseResult")
	}
	if !result.AlreadyPaused {
		t.Errorf("expected AlreadyPaused=true; got false")
	}
	if result.State != "paused" {
		t.Errorf("expected State=\"paused\"; got %q", result.State)
	}
	stops := stopCallNames(rec.snapshot())
	if len(stops) != 0 {
		t.Errorf("expected zero stop calls on idempotent no-op; got %v", stops)
	}
}

// TestPause_ResolveClusterError: empty ClusterName returns error; no stop
// calls emitted. (This validates the input-validation guard rather than the
// resolver itself, which is unit-tested separately.)
func TestPause_ResolveClusterError(t *testing.T) {
	withBinaryName(t, "docker")
	rec := &recordingCmder{lookup: stateLookupCmder(nil)}
	withCmder(t, rec.cmder())

	_, err := Pause(PauseOptions{
		ClusterName: "",
		Logger:      log.NoopLogger{},
	})
	if err == nil {
		t.Fatalf("expected error for empty ClusterName")
	}
	stops := stopCallNames(rec.snapshot())
	if len(stops) != 0 {
		t.Errorf("expected zero stop calls when ClusterName empty; got %v", stops)
	}
}

// TestPause_HASnapshotCaptured: 3 CP cluster → bootstrap CP node receives a
// Command("sh", "-c", "...> /kind/pause-snapshot.json") call; snapshot JSON
// contains keys "leaderID" and "pauseTime" (RFC3339).
func TestPause_HASnapshotCaptured(t *testing.T) {
	withBinaryName(t, "docker")
	bootstrap := &commandRecordingNode{fakeNode: fakeNode{name: "cp1", role: "control-plane"}}
	withFetcher(t, &fakePauseNodeFetcher{
		nodes: []nodes.Node{
			bootstrap,
			&commandRecordingNode{fakeNode: fakeNode{name: "cp2", role: "control-plane"}},
			&commandRecordingNode{fakeNode: fakeNode{name: "cp3", role: "control-plane"}},
		},
	})

	// Program the etcdctl invocation to return a JSON list with a leader id.
	etcdctlOutput := `[{"Endpoint":"https://127.0.0.1:2379","Status":{"header":{"member_id":1234},"leader":1234}}]`
	rec := &recordingCmder{lookup: func(_ string, args []string) (string, error) {
		if args[0] == "inspect" {
			return "running\n", nil
		}
		// any path ending in etcdctl OR any args containing endpoint/status
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "etcdctl") || (len(args) >= 2 && args[0] == "endpoint" && args[1] == "status") {
			return etcdctlOutput + "\n", nil
		}
		return "", nil
	}}
	withCmder(t, rec.cmder())

	_, err := Pause(PauseOptions{
		ClusterName: "kind",
		Logger:      log.NoopLogger{},
	})
	if err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}

	// Find the snapshot-write call: args contain "sh" command with payload referencing
	// /kind/pause-snapshot.json.
	foundWrite := false
	var writePayload string
	for _, c := range rec.snapshot() {
		if c.name == "sh" || strings.HasSuffix(c.name, "/sh") {
			joined := strings.Join(c.args, " ")
			if strings.Contains(joined, "/kind/pause-snapshot.json") {
				foundWrite = true
				writePayload = joined
			}
		}
	}
	if !foundWrite {
		t.Fatalf("expected a sh -c invocation writing /kind/pause-snapshot.json; calls: %#v", rec.snapshot())
	}
	if !strings.Contains(writePayload, "leaderID") {
		t.Errorf("snapshot payload missing leaderID key; got: %s", writePayload)
	}
	if !strings.Contains(writePayload, "pauseTime") {
		t.Errorf("snapshot payload missing pauseTime key; got: %s", writePayload)
	}
}

// TestPause_SingleCPNoSnapshotAttempt: 1 CP cluster → no snapshot Command call
// (HA-only behavior).
func TestPause_SingleCPNoSnapshotAttempt(t *testing.T) {
	withBinaryName(t, "docker")
	withFetcher(t, &fakePauseNodeFetcher{
		nodes: []nodes.Node{
			&commandRecordingNode{fakeNode: fakeNode{name: "cp1", role: "control-plane"}},
		},
	})
	rec := &recordingCmder{lookup: stateLookupCmder(map[string]string{"cp1": "running"})}
	withCmder(t, rec.cmder())

	_, err := Pause(PauseOptions{
		ClusterName: "kind",
		Logger:      log.NoopLogger{},
	})
	if err != nil {
		t.Fatalf("Pause returned error: %v", err)
	}
	for _, c := range rec.snapshot() {
		joined := strings.Join(c.args, " ")
		if strings.Contains(joined, "/kind/pause-snapshot.json") {
			t.Errorf("expected NO snapshot write on single-CP cluster; saw call: %v %v", c.name, c.args)
		}
		if strings.Contains(joined, "etcdctl") {
			t.Errorf("expected NO etcdctl invocation on single-CP cluster; saw call: %v %v", c.name, c.args)
		}
	}
}

// TestPause_SnapshotFailureContinues: HA cluster but etcdctl fake errors →
// pause still succeeds (snapshot is best-effort); warning emitted to logger.
func TestPause_SnapshotFailureContinues(t *testing.T) {
	withBinaryName(t, "docker")
	withFetcher(t, &fakePauseNodeFetcher{
		nodes: []nodes.Node{
			&commandRecordingNode{fakeNode: fakeNode{name: "cp1", role: "control-plane"}},
			&commandRecordingNode{fakeNode: fakeNode{name: "cp2", role: "control-plane"}},
		},
	})
	rec := &recordingCmder{lookup: func(_ string, args []string) (string, error) {
		if args[0] == "inspect" {
			return "running\n", nil
		}
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "etcdctl") || (len(args) >= 2 && args[0] == "endpoint" && args[1] == "status") {
			return "", fmt.Errorf("simulated etcdctl failure")
		}
		return "", nil
	}}
	withCmder(t, rec.cmder())

	logger := &warnRecordingLogger{}
	result, err := Pause(PauseOptions{
		ClusterName: "kind",
		Logger:      logger,
	})
	if err != nil {
		t.Fatalf("snapshot failure should not fail Pause; got err=%v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if !logger.warned {
		t.Errorf("expected a warning log entry for snapshot failure; saw none")
	}
}

// TestPauseResult_JSONSchema: marshal a populated PauseResult → JSON has
// top-level keys cluster, state, nodes (array), durationSeconds; each node has
// name, role, success, durationSeconds.
func TestPauseResult_JSONSchema(t *testing.T) {
	res := PauseResult{
		Cluster:  "kind",
		State:    "paused",
		Duration: 1.23,
		Nodes: []NodeResult{
			{Name: "n1", Role: "worker", Success: true, Duration: 0.5},
			{Name: "n2", Role: "control-plane", Success: false, Error: "boom", Duration: 0.7},
		},
	}
	bytes, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(bytes, &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, string(bytes))
	}
	for _, k := range []string{"cluster", "state", "nodes", "durationSeconds"} {
		if _, ok := got[k]; !ok {
			t.Errorf("top-level missing key %q (got %v)", k, mapKeys2(got))
		}
	}
	rawNodes, ok := got["nodes"].([]interface{})
	if !ok {
		t.Fatalf("nodes is not array: %T", got["nodes"])
	}
	if len(rawNodes) != 2 {
		t.Fatalf("nodes len = %d, want 2", len(rawNodes))
	}
	n0, ok := rawNodes[0].(map[string]interface{})
	if !ok {
		t.Fatalf("nodes[0] is not object")
	}
	for _, k := range []string{"name", "role", "success", "durationSeconds"} {
		if _, ok := n0[k]; !ok {
			t.Errorf("nodes[0] missing key %q (got %v)", k, mapKeys2(n0))
		}
	}
}

// --- Test-helper types ---

// warnRecordingLogger is a minimal log.Logger that records whether Warn or
// Warnf were ever called.
type warnRecordingLogger struct {
	warned bool
}

var _ log.Logger = (*warnRecordingLogger)(nil)

func (l *warnRecordingLogger) Warn(_ string)                              { l.warned = true }
func (l *warnRecordingLogger) Warnf(_ string, _ ...interface{})           { l.warned = true }
func (l *warnRecordingLogger) Error(_ string)                             {}
func (l *warnRecordingLogger) Errorf(_ string, _ ...interface{})          {}
func (l *warnRecordingLogger) V(_ log.Level) log.InfoLogger               { return log.NoopInfoLogger{} }

// mapKeys2 mirrors the helper from state_test.go for diagnostic messages
// (named differently to avoid collision with the existing helper if any).
func mapKeys2(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// silence "imported and not used" if io is unreferenced once helpers move.
var _ = io.Discard
