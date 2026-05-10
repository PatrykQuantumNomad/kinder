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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
	kindippin "sigs.k8s.io/kind/pkg/internal/ippin"
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

// ============================================================================
// HA resume strategy tests (Plan 52-03)
// ============================================================================
//
// NOTE: These tests swap kindippin.IppinCmder (for ReadIPAMState / docker cp)
// AND defaultCmder (for start, inspect, network, kubeadm, mv).
// MUST NOT use t.Parallel() — they mutate shared package-level globals.

// withIPPinCmderFn swaps kindippin.IppinCmder for the duration of the test.
// MUST NOT be used with t.Parallel().
func withIPPinCmderFn(t *testing.T, fn kindippin.Cmder) {
	t.Helper()
	prev := kindippin.IppinCmder
	kindippin.IppinCmder = fn
	t.Cleanup(func() { kindippin.IppinCmder = prev })
}

// withCertRegenSleeper swaps certRegenSleeper for the duration of the test.
func withCertRegenSleeper(t *testing.T, fn func(time.Duration)) {
	t.Helper()
	prev := certRegenSleeper
	certRegenSleeper = fn
	t.Cleanup(func() { certRegenSleeper = prev })
}

// withResumeBinaryName swaps resumeBinaryName for the duration of the test.
func withResumeBinaryName(t *testing.T, name string) {
	t.Helper()
	prev := resumeBinaryName
	resumeBinaryName = func() string { return name }
	t.Cleanup(func() { resumeBinaryName = prev })
}

// haTestCall records a single CLI invocation.
type haTestCall struct {
	name string
	args []string
}

func (c haTestCall) joined() string {
	return strings.Join(append([]string{c.name}, c.args...), " ")
}

// haTestCmder is a flexible Cmder for HA strategy tests. It dispatches on the
// docker subcommand and returns scripted responses.
type haTestCmder struct {
	mu    sync.Mutex
	calls []haTestCall

	// containerStates maps container name → state string ("exited", "running").
	// Used for `inspect --format {{.State.Status}} <name>`.
	containerStates map[string]string

	// strategyLabel maps container name → strategy label value.
	// Used for `inspect --format '{{index .Config.Labels "io.x-k8s.kinder.resume-strategy"}}' <name>`.
	strategyLabel map[string]string

	// currentIPs maps container name → current IP (after start).
	// Used for `inspect --format {{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}} <name>`.
	currentIPs map[string]string

	// startErrors maps container → error to return from `start`.
	startErrors map[string]error

	// networkDisconnectErrors maps container → error for `network disconnect`.
	networkDisconnectErrors map[string]error

	// networkConnectErrors maps container → error for `network connect`.
	networkConnectErrors map[string]error

	// kubeadmRenewErrors maps container → error for `kubeadm certs renew etcd-peer`.
	kubeadmRenewErrors map[string]error

	// mvErrors: if set, error returned on the first `mv` call.
	mvErrors map[string]error
}

func (h *haTestCmder) cmder() Cmder {
	return func(name string, args ...string) exec.Cmd {
		h.mu.Lock()
		argsCopy := make([]string, len(args))
		copy(argsCopy, args)
		h.calls = append(h.calls, haTestCall{name: name, args: argsCopy})
		h.mu.Unlock()

		if len(args) == 0 {
			return &fakeCmd{err: fmt.Errorf("no args")}
		}

		switch args[0] {
		case "start":
			container := args[len(args)-1]
			err := h.startErrors[container]
			return &fakeCmd{err: err}

		case "inspect":
			container := args[len(args)-1]
			// Determine which format is being requested.
			formatIdx := -1
			for i, a := range args {
				if a == "--format" && i+1 < len(args) {
					formatIdx = i + 1
					break
				}
			}
			format := ""
			if formatIdx >= 0 {
				format = args[formatIdx]
			}
			if strings.Contains(format, "State.Status") {
				state := h.containerStates[container]
				if state == "" {
					state = "exited"
				}
				return &fakeCmd{stdout: state + "\n"}
			}
			if strings.Contains(format, "Config.Labels") || strings.Contains(format, "resume-strategy") {
				label := h.strategyLabel[container]
				return &fakeCmd{stdout: label + "\n"}
			}
			if strings.Contains(format, "NetworkSettings.Networks") || strings.Contains(format, "IPAddress") {
				ip := h.currentIPs[container]
				return &fakeCmd{stdout: ip + "\n"}
			}
			// Unknown inspect format
			return &fakeCmd{stdout: "\n"}

		case "network":
			if len(args) < 2 {
				return &fakeCmd{err: fmt.Errorf("network: too few args")}
			}
			container := args[len(args)-1]
			switch args[1] {
			case "disconnect":
				err := h.networkDisconnectErrors[container]
				return &fakeCmd{err: err}
			case "connect":
				err := h.networkConnectErrors[container]
				return &fakeCmd{err: err}
			}
			return &fakeCmd{}

		case "kubeadm":
			// kubeadm certs renew etcd-peer — called via node.Command, so args
			// start with "certs" (the binary is the node name, not "kubeadm").
			// Actually node.Command("kubeadm", ...) routes through defaultCmder
			// as: defaultCmder("kubeadm", "certs", "renew", "etcd-peer")
			// But wait — fakeNode.Command calls defaultCmder(c, a...) where c="kubeadm".
			// So name="kubeadm" and args=["certs", "renew", "etcd-peer"].
			// The container name for error lookup must be known from context.
			// Since fakeNode.Command doesn't pass node name to defaultCmder, we
			// need to track which node is being processed via call ordering.
			// Simpler approach: record the call and return a canned error based
			// on the call index (or use a global error map keyed by call index).
			// For now, use a call-count approach embedded in the test.
			return &fakeCmd{}

		case "mv":
			return &fakeCmd{}
		}

		return &fakeCmd{err: fmt.Errorf("haTestCmder: unhandled subcommand %q (args=%v)", args[0], args)}
	}
}

func (h *haTestCmder) startCalls() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for _, c := range h.calls {
		if len(c.args) > 0 && c.args[0] == "start" {
			out = append(out, c.args[len(c.args)-1])
		}
	}
	return out
}

func (h *haTestCmder) networkCalls() []haTestCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []haTestCall
	for _, c := range h.calls {
		if len(c.args) > 0 && c.args[0] == "network" {
			out = append(out, c)
		}
	}
	return out
}

func (h *haTestCmder) allCalls() []haTestCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]haTestCall, len(h.calls))
	copy(out, h.calls)
	return out
}

// writeIPAMStateForHA writes per-CP ipam-state.json files to tmpDir, and
// configures ippinCmder to succeed on docker cp (file is already in tmpDir).
// Returns a configured ippinCmderFake.
func writeIPAMStateForHA(t *testing.T, tmpDir string, cpNetworks, cpIPs map[string]string) *ippinCmderFake {
	t.Helper()
	// Build responses in cp-sorted order. The exact order depends on ClassifyNodes.
	// We pre-write all files and return a cmder that always succeeds on cp.
	for container, ip := range cpIPs {
		network := cpNetworks[container]
		state := kindippin.IPAMState{Network: network, IPv4: ip}
		data, err := json.Marshal(state)
		if err != nil {
			t.Fatalf("writeIPAMStateForHA: %v", err)
		}
		hostPath := filepath.Join(tmpDir, container+"-ipam.json")
		if err := os.WriteFile(hostPath, data, 0o600); err != nil {
			t.Fatalf("writeIPAMStateForHA: %v", err)
		}
	}
	// Return a cmder that always succeeds on docker cp (file already written).
	// Number of responses = number of cp nodes (one cp per ReadIPAMState call).
	responses := make([]ippinCmdResp, len(cpIPs))
	return &ippinCmderFake{responses: responses}
}

// ---- Tests ------------------------------------------------------------------

// TestResume_HAWithIPPinned_ReconnectsBeforeCPStart: 3 CPs labeled ip-pinned.
// Asserts: disconnect+connect per CP BEFORE the first `docker start` on a CP.
// NetworkName comes from IPAMState.Network (per-CP).
// MUST NOT call t.Parallel().
func TestResume_HAWithIPPinned_ReconnectsBeforeCPStart(t *testing.T) {
	withResumeBinaryName(t, "docker")
	withCertRegenSleeper(t, func(time.Duration) {})
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {})
	withReadinessProber(t, alwaysReadyProber())

	tmpDir := t.TempDir()
	cpNetworks := map[string]string{"cp1": "kind-net-1", "cp2": "kind-net-2", "cp3": "kind-net-3"}
	cpIPs := map[string]string{"cp1": "172.18.0.5", "cp2": "172.18.0.6", "cp3": "172.18.0.7"}
	ippinFk := writeIPAMStateForHA(t, tmpDir, cpNetworks, cpIPs)
	withIPPinCmderFn(t, ippinFk.cmder)

	tc := &haTestCmder{
		containerStates: pausedInspectMap("cp1", "cp2", "cp3"),
		strategyLabel:   map[string]string{"cp1": StrategyIPPinned, "cp2": StrategyIPPinned, "cp3": StrategyIPPinned},
		currentIPs:      cpIPs,
	}
	withCmder(t, tc.cmder())

	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
	}
	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}
	// Override tmpDir in IPDriftDetected. Since IPDriftDetected is called by
	// the cert-regen path (not ip-pinned), this is not critical here.
	// applyPinnedIPsBeforeCPStart calls ReadIPAMState with os.TempDir() —
	// we wrote files there via ippinFk which routes docker cp.

	res, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify: for each CP, disconnect+connect appeared BEFORE docker start.
	allCalls := tc.allCalls()
	cpNames := []string{"cp1", "cp2", "cp3"}
	for _, cpName := range cpNames {
		disconnectIdx := -1
		connectIdx := -1
		startIdx := -1
		for i, c := range allCalls {
			joined := c.joined()
			if strings.Contains(joined, "network disconnect") && strings.Contains(joined, cpName) {
				disconnectIdx = i
			}
			if strings.Contains(joined, "network connect") && strings.Contains(joined, cpName) {
				connectIdx = i
			}
			if len(c.args) > 0 && c.args[0] == "start" && strings.Contains(joined, cpName) {
				startIdx = i
			}
		}
		if disconnectIdx < 0 {
			t.Errorf("no network disconnect for %s", cpName)
		}
		if connectIdx < 0 {
			t.Errorf("no network connect for %s", cpName)
		}
		if startIdx < 0 {
			t.Errorf("no start for %s", cpName)
		}
		if disconnectIdx >= startIdx {
			t.Errorf("%s: disconnect (idx=%d) must appear BEFORE start (idx=%d)", cpName, disconnectIdx, startIdx)
		}
		if connectIdx >= startIdx {
			t.Errorf("%s: connect (idx=%d) must appear BEFORE start (idx=%d)", cpName, connectIdx, startIdx)
		}
	}

	// Verify networkName per CP comes from IPAMState.Network.
	netCalls := tc.networkCalls()
	for _, cpName := range cpNames {
		expectedNet := cpNetworks[cpName]
		for _, c := range netCalls {
			if strings.Contains(c.joined(), cpName) {
				if !strings.Contains(c.joined(), expectedNet) {
					t.Errorf("%s: expected networkName %q in call %q", cpName, expectedNet, c.joined())
				}
			}
		}
	}

	// Verify --ip flag with recorded IP in the connect call.
	for _, cpName := range cpNames {
		recordedIP := cpIPs[cpName]
		foundConnect := false
		for _, c := range netCalls {
			if c.args[1] == "connect" && strings.Contains(c.joined(), cpName) {
				foundConnect = true
				if !strings.Contains(c.joined(), "--ip") || !strings.Contains(c.joined(), recordedIP) {
					t.Errorf("%s: connect call should have --ip %s: %q", cpName, recordedIP, c.joined())
				}
			}
		}
		if !foundConnect {
			t.Errorf("no network connect call for %s", cpName)
		}
	}
}

// TestResume_HAIPPin_ReadIPAMStateFailureHalts: W2 Option A.
// cp2's docker cp fails → Resume halts with structured diagnostic.
// cp3 NOT processed. Phase 2 (docker start on CPs) NOT reached.
// MUST NOT call t.Parallel().
func TestResume_HAIPPin_ReadIPAMStateFailureHalts(t *testing.T) {
	withResumeBinaryName(t, "docker")
	withCertRegenSleeper(t, func(time.Duration) {})
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {})
	withReadinessProber(t, alwaysReadyProber())

	tmpDir := t.TempDir()
	// cp1 has valid ipam-state.json; cp2 docker cp fails.
	state := kindippin.IPAMState{Network: "kind", IPv4: "172.18.0.5"}
	data, _ := json.Marshal(state)
	_ = os.WriteFile(filepath.Join(tmpDir, "cp1-ipam.json"), data, 0o600)

	// IppinCmder: call 0 = cp1 docker cp succeeds, call 1 = cp2 docker cp fails.
	ippinFk := &ippinCmderFake{
		responses: []ippinCmdResp{
			{},                                             // cp1 docker cp: success
			{err: fmt.Errorf("No such file or directory")}, // cp2 docker cp: failure
		},
	}
	withIPPinCmderFn(t, ippinFk.cmder)

	tc := &haTestCmder{
		containerStates: pausedInspectMap("cp1", "cp2", "cp3"),
		strategyLabel:   map[string]string{"cp1": StrategyIPPinned, "cp2": StrategyIPPinned, "cp3": StrategyIPPinned},
	}
	withCmder(t, tc.cmder())

	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
	}
	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}

	_, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err == nil {
		t.Fatal("expected error for ReadIPAMState failure on cp2, got nil")
	}

	// Error must match W2 Option A diagnostic pattern.
	if !strings.Contains(err.Error(), "ip-pin resume halted") {
		t.Errorf("error should contain 'ip-pin resume halted': %v", err)
	}
	if !strings.Contains(err.Error(), "cp2") {
		t.Errorf("error should mention cp2: %v", err)
	}
	if !strings.Contains(err.Error(), "Cluster state is undefined") {
		t.Errorf("error should contain 'Cluster state is undefined': %v", err)
	}

	// cp3 must NOT have disconnect/connect.
	allCalls := tc.allCalls()
	for _, c := range allCalls {
		if len(c.args) > 0 && c.args[0] == "network" && strings.Contains(c.joined(), "cp3") {
			t.Errorf("cp3 must NOT be touched after cp2 failure; got: %v", c.joined())
		}
	}

	// docker start on CPs must NOT have been called.
	starts := tc.startCalls()
	for _, s := range starts {
		if strings.HasPrefix(s, "cp") {
			t.Errorf("docker start on CP %s must NOT have been called after ip-pin halt", s)
		}
	}
}

// TestResume_HAWithCertRegen_NoIPDrift_SkipsRegen: 3 CPs labeled cert-regen;
// all CPs report same IP before and after start. No kubeadm calls.
// MUST NOT call t.Parallel().
func TestResume_HAWithCertRegen_NoIPDrift_SkipsRegen(t *testing.T) {
	withResumeBinaryName(t, "docker")
	withCertRegenSleeper(t, func(time.Duration) {})
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {})
	withReadinessProber(t, alwaysReadyProber())

	tmpDir := t.TempDir()
	cpIPs := map[string]string{"cp1": "172.18.0.5", "cp2": "172.18.0.6", "cp3": "172.18.0.7"}
	// Write ipam-state.json files with same IPs.
	for cpName, ip := range cpIPs {
		state := kindippin.IPAMState{Network: "kind", IPv4: ip}
		data, _ := json.Marshal(state)
		_ = os.WriteFile(filepath.Join(tmpDir, cpName+"-ipam.json"), data, 0o600)
	}
	// IppinCmder: 3 docker cp calls succeed (one per CP for IPDriftDetected).
	ippinFk := &ippinCmderFake{
		responses: []ippinCmdResp{{}, {}, {}},
	}
	withIPPinCmderFn(t, ippinFk.cmder)

	tc := &haTestCmder{
		containerStates: pausedInspectMap("cp1", "cp2", "cp3"),
		strategyLabel:   map[string]string{"cp1": StrategyCertRegen, "cp2": StrategyCertRegen, "cp3": StrategyCertRegen},
		currentIPs:      cpIPs, // same IPs as recorded → no drift
	}
	withCmder(t, tc.cmder())

	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
	}
	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}

	_, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No kubeadm calls expected (no drift).
	for _, c := range tc.allCalls() {
		if c.name == "kubeadm" {
			t.Errorf("kubeadm must NOT be called when no drift: %v", c.joined())
		}
	}
}

// TestResume_HAWithCertRegen_IPDrift_RunsWholesaleRegen: 3 CPs labeled cert-regen;
// cp1 reports drift. kubeadm renew must run on ALL 3 CPs (wholesale).
// MUST NOT call t.Parallel().
func TestResume_HAWithCertRegen_IPDrift_RunsWholesaleRegen(t *testing.T) {
	withResumeBinaryName(t, "docker")
	withCertRegenSleeper(t, func(time.Duration) {})
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {})
	withReadinessProber(t, alwaysReadyProber())

	tmpDir := t.TempDir()
	recordedIPs := map[string]string{"cp1": "172.18.0.5", "cp2": "172.18.0.6", "cp3": "172.18.0.7"}
	// Write ipam-state.json files with recorded IPs.
	for cpName, ip := range recordedIPs {
		state := kindippin.IPAMState{Network: "kind", IPv4: ip}
		data, _ := json.Marshal(state)
		_ = os.WriteFile(filepath.Join(tmpDir, cpName+"-ipam.json"), data, 0o600)
	}
	// IppinCmder: 3 docker cp calls succeed.
	ippinFk := &ippinCmderFake{
		responses: []ippinCmdResp{{}, {}, {}},
	}
	withIPPinCmderFn(t, ippinFk.cmder)

	tc := &haTestCmder{
		containerStates: pausedInspectMap("cp1", "cp2", "cp3"),
		strategyLabel:   map[string]string{"cp1": StrategyCertRegen, "cp2": StrategyCertRegen, "cp3": StrategyCertRegen},
		// cp1 reports a DIFFERENT IP → drift detected on first CP → wholesale regen.
		currentIPs: map[string]string{"cp1": "172.18.0.99", "cp2": "172.18.0.6", "cp3": "172.18.0.7"},
	}
	withCmder(t, tc.cmder())

	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
	}
	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}

	_, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// kubeadm certs renew etcd-peer must have been called on all 3 CPs.
	kubeadmCount := 0
	for _, c := range tc.allCalls() {
		if c.name == "kubeadm" {
			kubeadmCount++
		}
	}
	if kubeadmCount != 3 {
		t.Errorf("expected kubeadm certs renew on all 3 CPs, got %d calls; calls=%v",
			kubeadmCount, joinedCallsHA(tc.allCalls()))
	}
}

// TestResume_HALegacyNoLabel_RunsWholesaleRegen: 3 CPs with NO resume-strategy
// label (legacy). Treated as cert-regen. Wholesale regen runs (legacy=drift always).
// MUST NOT call t.Parallel().
func TestResume_HALegacyNoLabel_RunsWholesaleRegen(t *testing.T) {
	withResumeBinaryName(t, "docker")
	withCertRegenSleeper(t, func(time.Duration) {})
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {})
	withReadinessProber(t, alwaysReadyProber())

	tmpDir := t.TempDir()
	// IppinCmder: 3 docker cp calls fail with "No such file" (legacy = no ipam-state.json).
	ippinFk := &ippinCmderFake{
		responses: []ippinCmdResp{
			{err: fmt.Errorf("No such file or directory")},
			{err: fmt.Errorf("No such file or directory")},
			{err: fmt.Errorf("No such file or directory")},
		},
	}
	withIPPinCmderFn(t, ippinFk.cmder)
	_ = tmpDir

	tc := &haTestCmder{
		containerStates: pausedInspectMap("cp1", "cp2", "cp3"),
		// No strategy label set (legacy).
		strategyLabel: map[string]string{},
	}
	withCmder(t, tc.cmder())

	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
	}
	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}

	_, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// kubeadm certs renew should have been called on all 3 CPs (legacy = drift=true always).
	kubeadmCount := 0
	for _, c := range tc.allCalls() {
		if c.name == "kubeadm" {
			kubeadmCount++
		}
	}
	if kubeadmCount != 3 {
		t.Errorf("expected kubeadm calls on all 3 CPs for legacy cluster, got %d; calls=%v",
			kubeadmCount, joinedCallsHA(tc.allCalls()))
	}
}

// TestResume_SingleCP_NoStrategyDispatch: 1 CP, no label. Strategy dispatch
// is bypassed entirely (no probe, no inspect of /kind/ipam-state.json, no kubeadm).
// MUST NOT call t.Parallel().
func TestResume_SingleCP_NoStrategyDispatch(t *testing.T) {
	withResumeBinaryName(t, "docker")
	withCertRegenSleeper(t, func(time.Duration) {})
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {})
	withReadinessProber(t, alwaysReadyProber())

	cpCallCount := 0
	withIPPinCmderFn(t, func(name string, args ...string) exec.Cmd {
		cpCallCount++
		return &fakeCmd{err: fmt.Errorf("ippinCmder: should not be called on single-CP")}
	})

	tc := &haTestCmder{
		containerStates: pausedInspectMap("cp1"),
		strategyLabel:   map[string]string{},
	}
	withCmder(t, tc.cmder())

	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
	}
	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}

	_, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cpCallCount > 0 {
		t.Errorf("IppinCmder (docker cp for ReadIPAMState) must NOT be called on single-CP cluster")
	}
	for _, c := range tc.allCalls() {
		if c.name == "kubeadm" {
			t.Errorf("kubeadm must NOT be called on single-CP cluster; got: %v", c.joined())
		}
		if len(c.args) > 0 && c.args[0] == "network" {
			t.Errorf("network op must NOT be called on single-CP cluster; got: %v", c.joined())
		}
	}
}

// TestResume_HAIPPin_DisconnectFailsHaltsResume: disconnect on cp2 fails.
// Resume returns aggregated error; cp3 NOT reconnected; readiness gate NOT entered.
// MUST NOT call t.Parallel().
func TestResume_HAIPPin_DisconnectFailsHaltsResume(t *testing.T) {
	withResumeBinaryName(t, "docker")
	withCertRegenSleeper(t, func(time.Duration) {})
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {})

	probeCalled := false
	withReadinessProber(t, func(_ context.Context, _ nodes.Node, _ time.Time) error {
		probeCalled = true
		return nil
	})

	tmpDir := t.TempDir()
	for _, cp := range []string{"cp1", "cp2", "cp3"} {
		state := kindippin.IPAMState{Network: "kind", IPv4: "172.18.0.5"}
		data, _ := json.Marshal(state)
		_ = os.WriteFile(filepath.Join(tmpDir, cp+"-ipam.json"), data, 0o600)
	}
	ippinFk := &ippinCmderFake{
		responses: []ippinCmdResp{{}, {}, {}},
	}
	withIPPinCmderFn(t, ippinFk.cmder)

	tc := &haTestCmder{
		containerStates:         pausedInspectMap("cp1", "cp2", "cp3"),
		strategyLabel:           map[string]string{"cp1": StrategyIPPinned, "cp2": StrategyIPPinned, "cp3": StrategyIPPinned},
		networkDisconnectErrors: map[string]error{"cp2": fmt.Errorf("disconnect: permission denied")},
	}
	withCmder(t, tc.cmder())

	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
	}
	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}

	_, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err == nil {
		t.Fatal("expected error for cp2 disconnect failure, got nil")
	}

	// cp3 must NOT be reconnected.
	for _, c := range tc.networkCalls() {
		if strings.Contains(c.joined(), "cp3") {
			t.Errorf("cp3 must NOT be reconnected after cp2 disconnect failure: %v", c.joined())
		}
	}

	// Readiness gate must NOT be entered.
	if probeCalled {
		t.Error("readiness gate must NOT be entered after ip-pin failure")
	}
}

// TestResume_HACertRegen_RegenFailsHaltsResume: regen on cp2 fails.
// Resume returns wrapped error directing user to delete+recreate.
// Readiness gate skipped.
// MUST NOT call t.Parallel().
func TestResume_HACertRegen_RegenFailsHaltsResume(t *testing.T) {
	withResumeBinaryName(t, "docker")
	withCertRegenSleeper(t, func(time.Duration) {})
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {})

	probeCalled := false
	withReadinessProber(t, func(_ context.Context, _ nodes.Node, _ time.Time) error {
		probeCalled = true
		return nil
	})

	tmpDir := t.TempDir()
	// Legacy: no ipam-state.json → always drift → runs regen.
	ippinFk := &ippinCmderFake{
		responses: []ippinCmdResp{
			{err: fmt.Errorf("No such file or directory")},
			{err: fmt.Errorf("No such file or directory")},
			{err: fmt.Errorf("No such file or directory")},
		},
	}
	withIPPinCmderFn(t, ippinFk.cmder)
	_ = tmpDir

	// The kubeadm renew on cp2 will fail. Since fakeNode.Command routes through
	// defaultCmder (which is tc.cmder()), we need tc.cmder() to fail on the
	// 4th kubeadm call (cp2's kubeadm = after cp1's 3 calls: renew+mv+mv).
	kubeadmCallCount := 0
	tc := &haTestCmder{
		containerStates: pausedInspectMap("cp1", "cp2", "cp3"),
		strategyLabel:   map[string]string{},
	}
	// We'll use a custom cmder that wraps tc.cmder() to inject the kubeadm error.
	baseCmder := tc.cmder()
	kubeadmErrCmder := func(name string, args ...string) exec.Cmd {
		if name == "kubeadm" {
			kubeadmCallCount++
			if kubeadmCallCount == 2 { // second kubeadm call = cp2
				return &fakeCmd{err: fmt.Errorf("kubeadm: cert renew failed on cp2")}
			}
		}
		return baseCmder(name, args...)
	}
	withCmder(t, kubeadmErrCmder)

	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
	}
	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}

	_, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err == nil {
		t.Fatal("expected error for cert regen failure on cp2, got nil")
	}
	if !strings.Contains(err.Error(), "delete and recreate") {
		t.Errorf("error should direct user to delete+recreate: %v", err)
	}
	if probeCalled {
		t.Error("readiness gate must NOT be entered after cert-regen failure")
	}
}

// TestResume_HAIPPin_NerdctlPath: when binaryName=="nerdctl", the resume-strategy
// is downgraded to cert-regen regardless of the label value (defense in depth).
// MUST NOT call t.Parallel().
func TestResume_HAIPPin_NerdctlPath(t *testing.T) {
	withResumeBinaryName(t, "nerdctl")
	withCertRegenSleeper(t, func(time.Duration) {})
	withResumeReadinessHook(t, func(_, _ string, _ log.Logger) {})
	withReadinessProber(t, alwaysReadyProber())

	tmpDir := t.TempDir()
	// Legacy (no ipam-state.json) → drift = true → regen runs.
	ippinFk := &ippinCmderFake{
		responses: []ippinCmdResp{
			{err: fmt.Errorf("No such file or directory")},
			{err: fmt.Errorf("No such file or directory")},
			{err: fmt.Errorf("No such file or directory")},
		},
	}
	withIPPinCmderFn(t, ippinFk.cmder)
	_ = tmpDir

	tc := &haTestCmder{
		containerStates: pausedInspectMap("cp1", "cp2", "cp3"),
		// Label says ip-pinned but nerdctl should downgrade to cert-regen.
		strategyLabel: map[string]string{"cp1": StrategyIPPinned, "cp2": StrategyIPPinned, "cp3": StrategyIPPinned},
	}
	withCmder(t, tc.cmder())

	all := []nodes.Node{
		&fakeNode{name: "cp1", role: "control-plane"},
		&fakeNode{name: "cp2", role: "control-plane"},
		&fakeNode{name: "cp3", role: "control-plane"},
	}
	prov := &fakeProvider{
		clusters: []string{"k"},
		nodesMap: map[string][]nodes.Node{"k": all},
	}

	_, err := Resume(ResumeOptions{
		ClusterName: "k",
		Provider:    prov,
		Logger:      noopLogger{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must NOT have called network disconnect/connect (ip-pinned path).
	for _, c := range tc.networkCalls() {
		t.Errorf("nerdctl path must NOT call network ops; got: %v", c.joined())
	}
	// Should have called kubeadm (cert-regen path after downgrade).
	kubeadmCount := 0
	for _, c := range tc.allCalls() {
		if c.name == "kubeadm" {
			kubeadmCount++
		}
	}
	if kubeadmCount == 0 {
		t.Error("nerdctl path should fall back to cert-regen; expected kubeadm calls")
	}
}

// joinedCallsHA formats haTestCall slice for debug output.
func joinedCallsHA(calls []haTestCall) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.joined()
	}
	return out
}
