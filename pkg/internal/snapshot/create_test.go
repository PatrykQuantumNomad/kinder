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
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/log"
)

// ---------------------------------------------------------------------------
// Fake implementations for Create orchestrator tests
// ---------------------------------------------------------------------------

// callRecorder records the ordered sequence of function calls. It is safe for
// concurrent use (though the orchestrator is sequential; the mutex is defensive).
type callRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *callRecorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name)
}

func (r *callRecorder) sequence() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]string, len(r.calls))
	copy(cp, r.calls)
	return cp
}

// fakeNodeFetcher is a test-double nodeFetcher for Create/Restore tests.
type fakeNodeFetcher struct {
	nodes []nodes.Node
	err   error
}

func (f *fakeNodeFetcher) ListNodes(_ string) ([]nodes.Node, error) {
	return f.nodes, f.err
}

// createFakeNode is a minimal nodes.Node for Create tests that satisfies the
// full nodes.Node interface. Uses snapFakeNode from etcdrestore_test.go under
// the hood by building a no-op lookup node.
func newCreateFakeNode(name, role string) *snapFakeNode {
	return &snapFakeNode{name: name, role: role}
}

// buildTestCreateOpts constructs a CreateOptions with all fakes wired in.
// recorder tracks the call order. Callers can override individual fns.
func buildTestCreateOpts(
	t *testing.T,
	recorder *callRecorder,
	root string,
	clusterName string,
	snapName string,
) CreateOptions {
	t.Helper()

	cp := newCreateFakeNode(clusterName+"-control-plane", "control-plane")
	worker := newCreateFakeNode(clusterName+"-worker", "worker")

	pauseFn := func(opts lifecycle.PauseOptions) (*lifecycle.PauseResult, error) {
		recorder.record("pauseFn")
		return &lifecycle.PauseResult{Cluster: opts.ClusterName, State: "paused"}, nil
	}
	resumeFn := func(opts lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error) {
		recorder.record("resumeFn")
		return &lifecycle.ResumeResult{Cluster: opts.ClusterName, State: "resumed"}, nil
	}
	classifyFn := func(all []nodes.Node) (cpNodes, workers []nodes.Node, lb nodes.Node, err error) {
		return []nodes.Node{cp}, []nodes.Node{worker}, nil, nil
	}
	captureEtcd := func(_ context.Context, _ nodes.Node, dstPath string) (string, error) {
		recorder.record("captureEtcd")
		if err := os.WriteFile(dstPath, []byte("fake-etcd-snap"), 0600); err != nil {
			return "", err
		}
		return "deadbeef", nil
	}
	captureImages := func(_ context.Context, _ nodes.Node, dstPath string) (string, error) {
		recorder.record("captureImages")
		if err := os.WriteFile(dstPath, []byte("fake-images-tar"), 0600); err != nil {
			return "", err
		}
		return "cafebabe", nil
	}
	capturePVs := func(_ context.Context, _ []nodes.Node, dstPath string) (string, error) {
		recorder.record("capturePVs")
		if err := os.WriteFile(dstPath, []byte(""), 0600); err != nil {
			return "", err
		}
		return "", nil
	}
	captureTopo := func(_ context.Context, _ []nodes.Node, _ ClassifyFn, _ string) (TopologyInfo, string, string, error) {
		recorder.record("captureTopo")
		return TopologyInfo{ControlPlaneCount: 1, WorkerCount: 1}, "v1.31.2", "kindest/node:v1.31.2", nil
	}
	captureAddon := func(_ context.Context, _ nodes.Node) (map[string]string, error) {
		recorder.record("captureAddon")
		return map[string]string{}, nil
	}

	return CreateOptions{
		ClusterName: clusterName,
		SnapName:    snapName,
		Root:        root,
		Logger:      log.NoopLogger{},
		Context:     context.Background(),
		nodeFetcherFn: func(_ string) ([]nodes.Node, error) {
			return []nodes.Node{cp, worker}, nil
		},
		pauseFn:      pauseFn,
		resumeFn:     resumeFn,
		classifyFn:   classifyFn,
		captureEtcd:  captureEtcd,
		captureImages: captureImages,
		capturePVs:   capturePVs,
		captureTopo:  captureTopo,
		captureAddon: captureAddon,
	}
}

// ---------------------------------------------------------------------------
// Create orchestrator tests
// ---------------------------------------------------------------------------

func TestCreateHappyPath(t *testing.T) {
	root := t.TempDir()
	recorder := &callRecorder{}

	opts := buildTestCreateOpts(t, recorder, root, "test-cluster", "my-snap")

	result, err := Create(opts)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.SnapName != "my-snap" {
		t.Errorf("expected SnapName=my-snap, got %q", result.SnapName)
	}

	// Verify call sequence: topology + addon BEFORE etcd BEFORE pause BEFORE images+PVs
	seq := recorder.sequence()
	t.Logf("call sequence: %v", seq)

	positions := map[string]int{}
	for i, name := range seq {
		positions[name] = i
	}

	// Required ordering constraints per CONTEXT.md:
	// captureTopo, captureAddon, captureEtcd MUST all precede pauseFn
	// captureImages and capturePVs MUST come after pauseFn
	// resumeFn MUST come last
	required := []struct {
		before, after string
	}{
		{"captureTopo", "pauseFn"},
		{"captureAddon", "pauseFn"},
		{"captureEtcd", "pauseFn"},
		{"pauseFn", "captureImages"},
		{"pauseFn", "capturePVs"},
		{"captureImages", "resumeFn"},
		{"capturePVs", "resumeFn"},
	}
	for _, r := range required {
		bPos, bOk := positions[r.before]
		aPos, aOk := positions[r.after]
		if !bOk {
			t.Errorf("call %q was never made", r.before)
			continue
		}
		if !aOk {
			t.Errorf("call %q was never made", r.after)
			continue
		}
		if bPos >= aPos {
			t.Errorf("expected %q (pos %d) before %q (pos %d)", r.before, bPos, r.after, aPos)
		}
	}
}

func TestCreateAutoNamesSnapshot(t *testing.T) {
	root := t.TempDir()
	recorder := &callRecorder{}

	opts := buildTestCreateOpts(t, recorder, root, "test-cluster", "") // empty snap name

	result, err := Create(opts)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	// Auto-generated name must match ^snap-\d{8}-\d{6}$
	re := regexp.MustCompile(`^snap-\d{8}-\d{6}$`)
	if !re.MatchString(result.SnapName) {
		t.Errorf("auto-generated SnapName %q does not match %s", result.SnapName, re.String())
	}
}

func TestCreateRespectsExplicitName(t *testing.T) {
	root := t.TempDir()
	recorder := &callRecorder{}

	opts := buildTestCreateOpts(t, recorder, root, "test-cluster", "my-explicit-snap")

	result, err := Create(opts)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.SnapName != "my-explicit-snap" {
		t.Errorf("expected SnapName=my-explicit-snap, got %q", result.SnapName)
	}
}

func TestCreateInsufficientDisk(t *testing.T) {
	root := t.TempDir()
	recorder := &callRecorder{}

	opts := buildTestCreateOpts(t, recorder, root, "test-cluster", "snap-disk-fail")
	// Override disk-space check to fail.
	opts.diskSpaceFn = func(_ string, _ int64) error {
		return ErrInsufficientDiskSpace
	}

	_, err := Create(opts)
	if err == nil {
		t.Fatal("expected error from insufficient disk, got nil")
	}
	if !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Errorf("expected ErrInsufficientDiskSpace, got: %v", err)
	}

	// pauseFn must NOT have been called.
	for _, call := range recorder.sequence() {
		if call == "pauseFn" {
			t.Error("pauseFn was called despite disk pre-check failing")
		}
	}
}

func TestCreateEtcdCaptureFails(t *testing.T) {
	root := t.TempDir()
	recorder := &callRecorder{}

	opts := buildTestCreateOpts(t, recorder, root, "test-cluster", "snap-etcd-fail")
	opts.captureEtcd = func(_ context.Context, _ nodes.Node, _ string) (string, error) {
		recorder.record("captureEtcd")
		return "", errors.New("etcd capture error")
	}

	_, err := Create(opts)
	if err == nil {
		t.Fatal("expected error from etcd capture failure, got nil")
	}
	if !strings.Contains(err.Error(), "capture etcd") {
		t.Errorf("expected error to mention 'capture etcd', got: %v", err)
	}

	// pauseFn must NOT have been called.
	for _, call := range recorder.sequence() {
		if call == "pauseFn" {
			t.Error("pauseFn was called despite etcd pre-pause capture failing")
		}
	}
}

func TestCreateImagesCaptureFailsAfterPause_StillResumes(t *testing.T) {
	root := t.TempDir()
	recorder := &callRecorder{}

	opts := buildTestCreateOpts(t, recorder, root, "test-cluster", "snap-images-fail")
	opts.captureImages = func(_ context.Context, _ nodes.Node, _ string) (string, error) {
		recorder.record("captureImages")
		return "", errors.New("images capture error")
	}

	_, err := Create(opts)
	if err == nil {
		t.Fatal("expected error from images capture failure, got nil")
	}
	if !strings.Contains(err.Error(), "capture images") {
		t.Errorf("expected error to mention 'capture images', got: %v", err)
	}

	// resumeFn MUST have been called (defer guarantees it even after post-pause failure).
	resumeCalled := false
	for _, call := range recorder.sequence() {
		if call == "resumeFn" {
			resumeCalled = true
		}
	}
	if !resumeCalled {
		t.Error("resumeFn was NOT called after images capture failure (defer-resume broken)")
	}
}

func TestCreateBundleWriteFails(t *testing.T) {
	root := t.TempDir()
	recorder := &callRecorder{}

	opts := buildTestCreateOpts(t, recorder, root, "test-cluster", "snap-bundle-fail")
	// Override bundle write to fail.
	opts.writeBundleFn = func(_ context.Context, _ string, _ []Component, _ *Metadata) (string, error) {
		return "", errors.New("bundle write error")
	}

	_, err := Create(opts)
	if err == nil {
		t.Fatal("expected error from bundle write failure, got nil")
	}

	// resumeFn MUST have been called (defer guarantees it).
	resumeCalled := false
	for _, call := range recorder.sequence() {
		if call == "resumeFn" {
			resumeCalled = true
		}
	}
	if !resumeCalled {
		t.Error("resumeFn was NOT called after bundle write failure (defer-resume broken)")
	}
}
