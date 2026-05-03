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
	"strings"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// noopLogger is a minimal log.Logger for tests that don't care about output.
type noopLogger struct{}

var _ log.Logger = noopLogger{}

func (noopLogger) Warn(string)                     {}
func (noopLogger) Warnf(string, ...interface{})    {}
func (noopLogger) Error(string)                    {}
func (noopLogger) Errorf(string, ...interface{})   {}
func (noopLogger) V(log.Level) log.InfoLogger      { return noopInfoLogger{} }

type noopInfoLogger struct{}

func (noopInfoLogger) Info(string)                  {}
func (noopInfoLogger) Infof(string, ...interface{}) {}
func (noopInfoLogger) Enabled() bool                { return false }

// fakeProvider implements ClusterListerWithNodes (the subset of *cluster.Provider
// Resume actually uses): List() and ListNodes(name).
type fakeProvider struct {
	clusters []string
	nodesMap map[string][]nodes.Node
	listErr  error
}

func (f *fakeProvider) List() ([]string, error) {
	return f.clusters, f.listErr
}

func (f *fakeProvider) ListNodes(name string) ([]nodes.Node, error) {
	ns, ok := f.nodesMap[name]
	if !ok {
		return nil, nil
	}
	return ns, nil
}

// startCallRecorder produces a Cmder that records every container name passed
// to a `start` invocation. inspect calls fall through to a static lookup so the
// idempotency check can still execute.
type startCallRecorder struct {
	mu          sync.Mutex
	startOrder  []string
	startErrs   map[string]error // container -> err to return from Run
	inspectMap  map[string]string // container -> state to return ("running", "exited", ...)
	inspectErrs map[string]error
}

func (r *startCallRecorder) Cmder() Cmder {
	return func(name string, args ...string) exec.Cmd {
		// args[0] is the subcommand for docker/podman/nerdctl
		if len(args) == 0 {
			return &fakeCmd{err: fmt.Errorf("no args")}
		}
		switch args[0] {
		case "start":
			// `<bin> start <container>` — record the container name.
			container := args[len(args)-1]
			r.mu.Lock()
			r.startOrder = append(r.startOrder, container)
			err := r.startErrs[container]
			r.mu.Unlock()
			return &fakeCmd{err: err}
		case "inspect":
			// `<bin> inspect --format {{.State.Status}} <container>`
			container := args[len(args)-1]
			r.mu.Lock()
			state, ok := r.inspectMap[container]
			err := r.inspectErrs[container]
			r.mu.Unlock()
			if err != nil {
				return &fakeCmd{err: err}
			}
			if !ok {
				state = "exited"
			}
			return &fakeCmd{stdout: state + "\n"}
		}
		return &fakeCmd{err: fmt.Errorf("unhandled subcommand %q", args[0])}
	}
}

// withReadinessProber swaps the package readiness prober for the duration of
// the test and restores it on cleanup.
func withReadinessProber(t *testing.T, p ReadinessProber) {
	t.Helper()
	prev := defaultReadinessProber
	defaultReadinessProber = p
	t.Cleanup(func() { defaultReadinessProber = prev })
}

// makeRunningInspectMap returns an inspect map that reports every node as
// "exited" (so ClusterStatus returns Paused, ensuring Resume proceeds with
// start calls instead of taking the idempotent fast-path).
func pausedInspectMap(names ...string) map[string]string {
	m := make(map[string]string, len(names))
	for _, n := range names {
		m[n] = "exited"
	}
	return m
}

// runningInspectMap returns an inspect map with every node "running".
func runningInspectMap(names ...string) map[string]string {
	m := make(map[string]string, len(names))
	for _, n := range names {
		m[n] = "running"
	}
	return m
}

// alwaysReadyProber pretends every readiness probe call succeeds immediately.
func alwaysReadyProber() ReadinessProber {
	return func(_ context.Context, _ nodes.Node, _ time.Time) error {
		return nil
	}
}

// neverReadyProber pretends the readiness gate always times out.
func neverReadyProber() ReadinessProber {
	return func(ctx context.Context, _ nodes.Node, deadline time.Time) error {
		// honor the deadline so tests don't hang
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(deadline)):
		}
		return fmt.Errorf("timed out waiting for nodes Ready after %s", time.Until(deadline))
	}
}

// --- Resume tests ---

func TestResume_OrderLBBeforeCP_HA(t *testing.T) {
	all := []nodes.Node{
		&fakeNode{name: "w1", role: "worker"},
		&fakeNode{name: "w2", role: "worker"},
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
		&fakeNode{name: "lb", role: "external-load-balancer"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("w1", "w2", "cp1", "cp2", "cp3", "lb"),
	}
	withCmder(t, rec.Cmder())
	withReadinessProber(t, alwaysReadyProber())

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	res, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.AlreadyRunning {
		t.Fatalf("unexpected idempotent result: %#v", res)
	}
	// LB must be first.
	if len(rec.startOrder) == 0 || rec.startOrder[0] != "lb" {
		t.Fatalf("expected lb to start first, got order: %v", rec.startOrder)
	}
	// All CPs must come before any worker.
	cpSeen := 0
	workerSeen := 0
	for _, name := range rec.startOrder[1:] {
		switch {
		case strings.HasPrefix(name, "cp"):
			if workerSeen > 0 {
				t.Fatalf("CP %s started after a worker; order: %v", name, rec.startOrder)
			}
			cpSeen++
		case strings.HasPrefix(name, "w"):
			workerSeen++
		}
	}
	if cpSeen != 3 || workerSeen != 2 {
		t.Errorf("expected 3 CPs + 2 workers after lb; got cp=%d w=%d (order=%v)", cpSeen, workerSeen, rec.startOrder)
	}
}

func TestResume_OrderCPBeforeWorkers(t *testing.T) {
	all := []nodes.Node{
		&fakeNode{name: "w1", role: "worker"},
		&fakeNode{name: "w2", role: "worker"},
		&fakeNode{name: "cp1", role: "control-plane"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("w1", "w2", "cp1"),
	}
	withCmder(t, rec.Cmder())
	withReadinessProber(t, alwaysReadyProber())

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	if _, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.startOrder) != 3 {
		t.Fatalf("expected 3 start calls, got %v", rec.startOrder)
	}
	if rec.startOrder[0] != "cp1" {
		t.Errorf("CP must start first, got order: %v", rec.startOrder)
	}
}

func TestResume_OrderSingleNode(t *testing.T) {
	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("cp1"),
	}
	withCmder(t, rec.Cmder())
	withReadinessProber(t, alwaysReadyProber())

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	if _, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.startOrder) != 1 || rec.startOrder[0] != "cp1" {
		t.Errorf("expected single start of cp1, got %v", rec.startOrder)
	}
}

func TestResume_BestEffortContinuesOnFailure(t *testing.T) {
	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "w1", role: "worker"},
		&fakeNode{name: "w2", role: "worker"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("cp1", "w1", "w2"),
		startErrs:  map[string]error{"w1": fmt.Errorf("docker: simulated start failure")},
	}
	withCmder(t, rec.Cmder())
	withReadinessProber(t, alwaysReadyProber())

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	res, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err == nil {
		t.Fatalf("expected aggregated start error, got nil; result=%#v", res)
	}
	if len(rec.startOrder) != 3 {
		t.Errorf("expected 3 start attempts even after failure, got %v", rec.startOrder)
	}
	if res == nil || len(res.Nodes) != 3 {
		t.Fatalf("expected ResumeResult with 3 node entries, got %#v", res)
	}
	successCount, failCount := 0, 0
	for _, n := range res.Nodes {
		if n.Success {
			successCount++
		} else {
			failCount++
			if n.Name == "w1" && n.Error == "" {
				t.Errorf("expected error message on failed node w1, got empty")
			}
		}
	}
	if successCount != 2 || failCount != 1 {
		t.Errorf("expected 2 success / 1 fail, got %d/%d", successCount, failCount)
	}
}

func TestResume_IdempotentNoOp(t *testing.T) {
	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "w1", role: "worker"},
	}
	rec := &startCallRecorder{
		inspectMap: runningInspectMap("cp1", "w1"),
	}
	withCmder(t, rec.Cmder())

	probeCalls := 0
	withReadinessProber(t, func(_ context.Context, _ nodes.Node, _ time.Time) error {
		probeCalls++
		return nil
	})

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	res, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.AlreadyRunning {
		t.Fatalf("expected AlreadyRunning result, got %#v", res)
	}
	if len(rec.startOrder) != 0 {
		t.Errorf("expected zero start calls on idempotent path, got %v", rec.startOrder)
	}
	if probeCalls != 0 {
		t.Errorf("expected zero readiness probe calls, got %d", probeCalls)
	}
}

func TestResume_ReadinessGate_AllReady(t *testing.T) {
	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("cp1"),
	}
	withCmder(t, rec.Cmder())

	probeCalled := false
	withReadinessProber(t, func(_ context.Context, _ nodes.Node, _ time.Time) error {
		probeCalled = true
		return nil
	})

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	res, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !probeCalled {
		t.Fatal("expected readiness prober to be called")
	}
	if res.ReadinessSeconds < 0 {
		t.Errorf("readinessSeconds should be >= 0, got %v", res.ReadinessSeconds)
	}
}

func TestResume_ReadinessGate_Timeout(t *testing.T) {
	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("cp1"),
	}
	withCmder(t, rec.Cmder())
	withReadinessProber(t, neverReadyProber())

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	res, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
		WaitTimeout: 100 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected timeout error, got nil; result=%#v", res)
	}
	if !strings.Contains(err.Error(), "timed out waiting for nodes Ready") {
		t.Errorf("expected timeout error message, got: %v", err)
	}
}

func TestResume_NoReadinessOnPartialStartFailure(t *testing.T) {
	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "w1", role: "worker"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("cp1", "w1"),
		startErrs:  map[string]error{"w1": fmt.Errorf("simulated failure")},
	}
	withCmder(t, rec.Cmder())

	probeCalls := 0
	withReadinessProber(t, func(_ context.Context, _ nodes.Node, _ time.Time) error {
		probeCalls++
		return nil
	})

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	if _, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	}); err == nil {
		t.Fatal("expected aggregated start error, got nil")
	}
	if probeCalls != 0 {
		t.Errorf("expected zero readiness probe calls when starts failed, got %d", probeCalls)
	}
}

func TestResume_ReadinessProbeReceivesBootstrap(t *testing.T) {
	all := []nodes.Node{
		&fakeNode{name: "cp-bootstrap", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "w1", role: "worker"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("cp-bootstrap", "cp2", "w1"),
	}
	withCmder(t, rec.Cmder())

	var seen string
	withReadinessProber(t, func(_ context.Context, bootstrap nodes.Node, _ time.Time) error {
		seen = bootstrap.String()
		return nil
	})

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	if _, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// nodeutils.ControlPlaneNodes sorts deterministically — bootstrap is index 0.
	if seen == "" {
		t.Fatal("readiness probe was never called with a bootstrap node")
	}
	// Either CP can be index 0 depending on sort; both are acceptable as long
	// as the probe receives a control-plane node (not a worker, not the LB).
	if seen != "cp-bootstrap" && seen != "cp2" {
		t.Errorf("readiness probe got node %q; expected a control-plane node", seen)
	}
}

func TestResume_EmptyCluster_Errors(t *testing.T) {
	rec := &startCallRecorder{}
	withCmder(t, rec.Cmder())
	withReadinessProber(t, alwaysReadyProber())

	prov := &fakeProvider{
		clusters: []string{},
		nodesMap: map[string][]nodes.Node{},
	}
	if _, err := Resume(ResumeOptions{
		ClusterName: "missing",
		Provider:    prov,
		Logger:      noopLogger{},
	}); err == nil {
		t.Fatal("expected error for missing/empty cluster, got nil")
	}
}

func TestResume_NilProvider_Errors(t *testing.T) {
	if _, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    nil,
	}); err == nil {
		t.Fatal("expected error for nil provider, got nil")
	}
}

func TestResume_EmptyClusterName_Errors(t *testing.T) {
	prov := &fakeProvider{nodesMap: map[string][]nodes.Node{}}
	if _, err := Resume(ResumeOptions{
		ClusterName: "",
		Provider:    prov,
	}); err == nil {
		t.Fatal("expected error for empty cluster name, got nil")
	}
}

func TestResumeResult_JSONSchema(t *testing.T) {
	r := &ResumeResult{
		Cluster:          "k",
		State:            "resumed",
		Nodes:            []NodeResult{{Name: "cp1", Role: "control-plane", Success: true, Duration: 1.5}},
		ReadinessSeconds: 2.5,
		Duration:         5.0,
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	got := string(b)
	for _, key := range []string{`"cluster"`, `"state"`, `"nodes"`, `"readinessSeconds"`, `"durationSeconds"`, `"name"`, `"role"`, `"success"`, `"durationSeconds"`} {
		if !strings.Contains(got, key) {
			t.Errorf("expected key %s in JSON %s", key, got)
		}
	}
	// alreadyRunning should be omitted when false
	if strings.Contains(got, `"alreadyRunning"`) {
		t.Errorf("alreadyRunning should be omitted when false, got %s", got)
	}
}

func TestResumeResult_JSONSchema_AlreadyRunning(t *testing.T) {
	r := &ResumeResult{
		Cluster:        "k",
		State:          "resumed",
		AlreadyRunning: true,
		Nodes:          []NodeResult{},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if !strings.Contains(string(b), `"alreadyRunning":true`) {
		t.Errorf("expected alreadyRunning:true in JSON %s", string(b))
	}
}

// --- Inline cluster-resume-readiness hook tests (plan 47-04) ---

// recordedHookInvocation captures one call to ResumeReadinessHook so tests can
// assert it ran (or didn't), with what args, in the right order vs. start calls.
type recordedHookInvocation struct {
	binaryName  string
	bootstrapCP string
	calledAfter []string // snapshot of startOrder at time of call
}

// withResumeReadinessHook swaps the package-level hook for the duration of the
// test and restores it on cleanup. Used by tests to assert invocation; the
// production default is exercised by TestResume_InlineReadinessHook_DefaultIsRealCheck.
func withResumeReadinessHook(t *testing.T, h func(binaryName, bootstrapCP string, logger log.Logger)) {
	t.Helper()
	prev := ResumeReadinessHook
	ResumeReadinessHook = h
	t.Cleanup(func() { ResumeReadinessHook = prev })
}

func TestResume_InlineReadinessHook_HA(t *testing.T) {
	// HA cluster: hook MUST be invoked exactly once between CP start and worker start,
	// receiving the bootstrap CP container name.
	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
		&fakeNode{name: "w1", role: "worker"},
		&fakeNode{name: "lb", role: "external-load-balancer"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("cp1", "cp2", "cp3", "w1", "lb"),
	}
	withCmder(t, rec.Cmder())
	withReadinessProber(t, alwaysReadyProber())

	var invocations []recordedHookInvocation
	withResumeReadinessHook(t, func(binaryName, bootstrap string, _ log.Logger) {
		// snapshot startOrder at hook-time
		rec.mu.Lock()
		snap := append([]string(nil), rec.startOrder...)
		rec.mu.Unlock()
		invocations = append(invocations, recordedHookInvocation{
			binaryName:  binaryName,
			bootstrapCP: bootstrap,
			calledAfter: snap,
		})
	})

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	if _, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(invocations) != 1 {
		t.Fatalf("expected exactly 1 hook invocation, got %d", len(invocations))
	}
	inv := invocations[0]
	if inv.bootstrapCP != "cp1" && inv.bootstrapCP != "cp2" && inv.bootstrapCP != "cp3" {
		t.Errorf("hook bootstrap = %q; expected one of the CP names", inv.bootstrapCP)
	}
	// Must be called after lb + all 3 CPs but before any worker.
	cpStarted := 0
	for _, name := range inv.calledAfter {
		if strings.HasPrefix(name, "cp") {
			cpStarted++
		}
		if strings.HasPrefix(name, "w") {
			t.Errorf("worker %q started before hook ran; calledAfter=%v", name, inv.calledAfter)
		}
	}
	if cpStarted != 3 {
		t.Errorf("expected all 3 CPs started before hook, got %d (calledAfter=%v)", cpStarted, inv.calledAfter)
	}
}

func TestResume_InlineReadinessHook_SingleCP_Skipped(t *testing.T) {
	// Single-CP cluster: hook MUST NOT be called.
	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "w1", role: "worker"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("cp1", "w1"),
	}
	withCmder(t, rec.Cmder())
	withReadinessProber(t, alwaysReadyProber())

	called := false
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {
		called = true
	})

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	if _, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("expected hook NOT to be called on single-CP cluster")
	}
}

func TestResume_InlineReadinessHook_WarnDoesNotBlock(t *testing.T) {
	// HA cluster, hook simulates a doctor warn (logs, but no error). Resume must
	// proceed: workers start, readiness probe runs, no error returned, exit code
	// unchanged. Per CONTEXT.md "warn and continue".
	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "w1", role: "worker"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("cp1", "cp2", "w1"),
	}
	withCmder(t, rec.Cmder())
	withReadinessProber(t, alwaysReadyProber())

	withResumeReadinessHook(t, func(_, _ string, logger log.Logger) {
		// emulate warn — logger receives it; hook returns no error
		logger.Warnf("simulated etcd quorum at risk")
	})

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	res, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err != nil {
		t.Fatalf("warn must not block resume; got error: %v", err)
	}
	if res == nil || len(res.Nodes) != 3 {
		t.Fatalf("expected 3 NodeResults, got %#v", res)
	}
	for _, n := range res.Nodes {
		if !n.Success {
			t.Errorf("expected all nodes to succeed despite hook warn; got node %s success=%v", n.Name, n.Success)
		}
	}
}

func TestResume_InlineReadinessHook_DefaultIsRealCheck(t *testing.T) {
	// The package-level ResumeReadinessHook MUST default to a non-nil function
	// so the production code path actually runs the doctor check. We don't call
	// it (would require a real cluster) — just assert it's wired.
	if ResumeReadinessHook == nil {
		t.Fatal("ResumeReadinessHook should default to a non-nil function (defaultResumeReadinessHook)")
	}
}

func TestResume_InlineReadinessHook_SkippedOnPartialStartFailure(t *testing.T) {
	// HA cluster but a CP fails to start → hook MUST NOT be called (no point
	// probing etcd quorum when containers didn't come up).
	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "w1", role: "worker"},
	}
	rec := &startCallRecorder{
		inspectMap: pausedInspectMap("cp1", "cp2", "w1"),
		startErrs:  map[string]error{"cp2": fmt.Errorf("simulated CP failure")},
	}
	withCmder(t, rec.Cmder())
	withReadinessProber(t, alwaysReadyProber())

	called := false
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {
		called = true
	})

	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	if _, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	}); err == nil {
		t.Fatal("expected aggregated start error, got nil")
	}
	if called {
		t.Error("hook MUST NOT be called when a CP failed to start")
	}
}
