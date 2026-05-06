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
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/log"
)

// ---------------------------------------------------------------------------
// Helpers: build a minimal real bundle on disk for Restore tests
// ---------------------------------------------------------------------------

// buildTestBundle creates a minimal valid .tar.gz bundle in root/clusterName/
// with the given snap name and metadata, and returns the store.
func buildTestBundle(t *testing.T, root, clusterName, snapName string, meta *Metadata) *SnapshotStore {
	t.Helper()

	store, err := NewStore(root, clusterName)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	tmpDir := t.TempDir()

	// Write minimal component files.
	etcdPath := tmpDir + "/etcd.snap"
	if err := os.WriteFile(etcdPath, []byte("fake-etcd"), 0600); err != nil {
		t.Fatal(err)
	}
	imagesPath := tmpDir + "/images.tar"
	if err := os.WriteFile(imagesPath, []byte("fake-images"), 0600); err != nil {
		t.Fatal(err)
	}
	pvsPath := tmpDir + "/pvs.tar"
	if err := os.WriteFile(pvsPath, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	configPath := tmpDir + "/kind-config.yaml"
	if err := os.WriteFile(configPath, []byte("kind: Cluster"), 0600); err != nil {
		t.Fatal(err)
	}

	comps := []Component{
		{Name: EntryEtcd, Path: etcdPath},
		{Name: EntryImages, Path: imagesPath},
		{Name: EntryPVs, Path: pvsPath},
		{Name: EntryConfig, Path: configPath},
	}

	archivePath := store.Path(snapName)
	_, err = WriteBundle(context.Background(), archivePath, comps, meta)
	if err != nil {
		t.Fatalf("WriteBundle: %v", err)
	}
	return store
}

// defaultSnapMeta returns a sensible Metadata for tests.
func defaultSnapMeta(clusterName, snapName string) *Metadata {
	return &Metadata{
		SchemaVersion: MetadataVersion,
		Name:          snapName,
		ClusterName:   clusterName,
		CreatedAt:     time.Now().UTC(),
		K8sVersion:    "v1.31.2",
		NodeImage:     "kindest/node:v1.31.2",
		Topology:      TopologyInfo{ControlPlaneCount: 1, WorkerCount: 0},
		AddonVersions: map[string]string{},
	}
}

// buildTestRestoreOpts constructs a RestoreOptions with all fakes wired in.
// recorder tracks the call order.
func buildTestRestoreOpts(
	t *testing.T,
	recorder *callRecorder,
	root string,
	clusterName string,
	snapName string,
	liveMeta *Metadata, // live cluster metadata returned by captureTopo/addon fakes
) RestoreOptions {
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
	verifyBundleFn := func(archivePath string) error {
		recorder.record("verifyBundle")
		return nil
	}
	diskFn := func(_ string, _ int64) error {
		recorder.record("diskCheck")
		return nil
	}
	etcdReachableFn := func(_ context.Context, _ nodes.Node) error {
		recorder.record("etcdReachable")
		return nil
	}
	captureLiveTopoFn := func(_ context.Context, _ []nodes.Node, _ ClassifyFn, _ string) (TopologyInfo, string, string, error) {
		recorder.record("captureTopo")
		if liveMeta != nil {
			return liveMeta.Topology, liveMeta.K8sVersion, liveMeta.NodeImage, nil
		}
		return TopologyInfo{ControlPlaneCount: 1}, "v1.31.2", "kindest/node:v1.31.2", nil
	}
	captureAddonFn := func(_ context.Context, _ nodes.Node) (map[string]string, error) {
		recorder.record("captureAddon")
		if liveMeta != nil {
			return liveMeta.AddonVersions, nil
		}
		return map[string]string{}, nil
	}
	restoreEtcdFn := func(_ context.Context, _ EtcdRestoreOptions) error {
		recorder.record("restoreEtcd")
		return nil
	}
	restoreImagesFn := func(_ context.Context, _ []nodes.Node, _ string, _ ImportArchiveFn) error {
		recorder.record("restoreImages")
		return nil
	}
	restorePVsFn := func(_ context.Context, _ []nodes.Node, _ string) error {
		recorder.record("restorePVs")
		return nil
	}

	return RestoreOptions{
		ClusterName: clusterName,
		SnapName:    snapName,
		Root:        root,
		Logger:      log.NoopLogger{},
		Context:     context.Background(),
		nodeFetcherFn:     func(_ string) ([]nodes.Node, error) { return []nodes.Node{cp, worker}, nil },
		pauseFn:           pauseFn,
		resumeFn:          resumeFn,
		classifyFn:        classifyFn,
		verifyBundleFn:    verifyBundleFn,
		diskSpaceFn:       diskFn,
		etcdReachableFn:   etcdReachableFn,
		captureLiveTopoFn: captureLiveTopoFn,
		captureAddonFn:    captureAddonFn,
		restoreEtcdFn:     restoreEtcdFn,
		restoreImagesFn:   restoreImagesFn,
		restorePVsFn:      restorePVsFn,
	}
}

// ---------------------------------------------------------------------------
// Restore orchestrator tests
// ---------------------------------------------------------------------------

func TestRestoreSnapshotNotFound(t *testing.T) {
	root := t.TempDir()
	recorder := &callRecorder{}

	opts := buildTestRestoreOpts(t, recorder, root, "test-cluster", "nonexistent-snap", nil)

	_, err := Restore(opts)
	if err == nil {
		t.Fatal("expected error for nonexistent snapshot, got nil")
	}
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Errorf("expected ErrSnapshotNotFound, got: %v", err)
	}

	for _, call := range recorder.sequence() {
		if call == "pauseFn" {
			t.Error("pauseFn was called despite snapshot not found")
		}
	}
}

func TestRestoreCorruptArchive(t *testing.T) {
	root := t.TempDir()
	clusterName := "test-cluster"
	snapName := "test-snap"
	recorder := &callRecorder{}

	buildTestBundle(t, root, clusterName, snapName, defaultSnapMeta(clusterName, snapName))

	opts := buildTestRestoreOpts(t, recorder, root, clusterName, snapName, nil)
	// Override verifyBundle to return corrupt error.
	opts.verifyBundleFn = func(_ string) error {
		return ErrCorruptArchive
	}

	_, err := Restore(opts)
	if err == nil {
		t.Fatal("expected error for corrupt archive, got nil")
	}
	if !errors.Is(err, ErrCorruptArchive) {
		t.Errorf("expected ErrCorruptArchive, got: %v", err)
	}

	for _, call := range recorder.sequence() {
		if call == "pauseFn" {
			t.Error("pauseFn was called despite corrupt archive pre-check failing")
		}
	}
}

func TestRestoreInsufficientDisk(t *testing.T) {
	root := t.TempDir()
	clusterName := "test-cluster"
	snapName := "test-snap"
	recorder := &callRecorder{}

	buildTestBundle(t, root, clusterName, snapName, defaultSnapMeta(clusterName, snapName))

	opts := buildTestRestoreOpts(t, recorder, root, clusterName, snapName, nil)
	opts.diskSpaceFn = func(_ string, _ int64) error {
		return ErrInsufficientDiskSpace
	}

	_, err := Restore(opts)
	if err == nil {
		t.Fatal("expected error for insufficient disk, got nil")
	}
	if !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Errorf("expected ErrInsufficientDiskSpace, got: %v", err)
	}

	for _, call := range recorder.sequence() {
		if call == "pauseFn" {
			t.Error("pauseFn was called despite disk pre-check failing")
		}
	}
}

func TestRestoreClusterNotRunning(t *testing.T) {
	root := t.TempDir()
	clusterName := "test-cluster"
	snapName := "test-snap"
	recorder := &callRecorder{}

	buildTestBundle(t, root, clusterName, snapName, defaultSnapMeta(clusterName, snapName))

	opts := buildTestRestoreOpts(t, recorder, root, clusterName, snapName, nil)
	opts.etcdReachableFn = func(_ context.Context, _ nodes.Node) error {
		return ErrClusterNotRunning
	}

	_, err := Restore(opts)
	if err == nil {
		t.Fatal("expected error for cluster not running, got nil")
	}
	if !errors.Is(err, ErrClusterNotRunning) {
		t.Errorf("expected ErrClusterNotRunning, got: %v", err)
	}

	for _, call := range recorder.sequence() {
		if call == "pauseFn" {
			t.Error("pauseFn was called despite cluster not running pre-check failing")
		}
	}
}

func TestRestoreK8sVersionMismatch(t *testing.T) {
	root := t.TempDir()
	clusterName := "test-cluster"
	snapName := "test-snap"
	recorder := &callRecorder{}

	snapMeta := defaultSnapMeta(clusterName, snapName)
	snapMeta.K8sVersion = "v1.31.2"
	buildTestBundle(t, root, clusterName, snapName, snapMeta)

	// Live cluster reports a different K8s version.
	liveMeta := defaultSnapMeta(clusterName, snapName)
	liveMeta.K8sVersion = "v1.32.0"
	opts := buildTestRestoreOpts(t, recorder, root, clusterName, snapName, liveMeta)

	_, err := Restore(opts)
	if err == nil {
		t.Fatal("expected error for k8s version mismatch, got nil")
	}
	if !errors.Is(err, ErrCompatK8sMismatch) {
		t.Errorf("expected ErrCompatK8sMismatch, got: %v", err)
	}

	for _, call := range recorder.sequence() {
		if call == "pauseFn" {
			t.Error("pauseFn was called despite k8s version mismatch")
		}
	}
}

func TestRestoreTopologyMismatch(t *testing.T) {
	root := t.TempDir()
	clusterName := "test-cluster"
	snapName := "test-snap"
	recorder := &callRecorder{}

	snapMeta := defaultSnapMeta(clusterName, snapName)
	snapMeta.Topology = TopologyInfo{ControlPlaneCount: 1, WorkerCount: 0}
	buildTestBundle(t, root, clusterName, snapName, snapMeta)

	// Live cluster has 3 CPs.
	liveMeta := defaultSnapMeta(clusterName, snapName)
	liveMeta.Topology = TopologyInfo{ControlPlaneCount: 3, WorkerCount: 0}
	opts := buildTestRestoreOpts(t, recorder, root, clusterName, snapName, liveMeta)

	_, err := Restore(opts)
	if err == nil {
		t.Fatal("expected error for topology mismatch, got nil")
	}
	if !errors.Is(err, ErrCompatTopologyMismatch) {
		t.Errorf("expected ErrCompatTopologyMismatch, got: %v", err)
	}

	for _, call := range recorder.sequence() {
		if call == "pauseFn" {
			t.Error("pauseFn was called despite topology mismatch")
		}
	}
}

func TestRestoreAddonMismatch(t *testing.T) {
	root := t.TempDir()
	clusterName := "test-cluster"
	snapName := "test-snap"
	recorder := &callRecorder{}

	snapMeta := defaultSnapMeta(clusterName, snapName)
	snapMeta.AddonVersions = map[string]string{"certManager": "1.12.0"}
	buildTestBundle(t, root, clusterName, snapName, snapMeta)

	// Live cluster has a different certManager version.
	liveMeta := defaultSnapMeta(clusterName, snapName)
	liveMeta.AddonVersions = map[string]string{"certManager": "1.13.0"}
	opts := buildTestRestoreOpts(t, recorder, root, clusterName, snapName, liveMeta)

	_, err := Restore(opts)
	if err == nil {
		t.Fatal("expected error for addon mismatch, got nil")
	}
	if !errors.Is(err, ErrCompatAddonMismatch) {
		t.Errorf("expected ErrCompatAddonMismatch, got: %v", err)
	}

	for _, call := range recorder.sequence() {
		if call == "pauseFn" {
			t.Error("pauseFn was called despite addon mismatch")
		}
	}
}

func TestRestoreHappyPath(t *testing.T) {
	root := t.TempDir()
	clusterName := "test-cluster"
	snapName := "test-snap"
	recorder := &callRecorder{}

	buildTestBundle(t, root, clusterName, snapName, defaultSnapMeta(clusterName, snapName))

	opts := buildTestRestoreOpts(t, recorder, root, clusterName, snapName, nil)

	result, err := Restore(opts)
	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.SnapName != snapName {
		t.Errorf("expected SnapName=%q, got %q", snapName, result.SnapName)
	}

	seq := recorder.sequence()
	t.Logf("call sequence: %v", seq)

	positions := map[string]int{}
	for i, name := range seq {
		if _, exists := positions[name]; !exists {
			positions[name] = i
		}
	}

	// Pre-flight must ALL precede pauseFn.
	preFlightCalls := []string{"verifyBundle", "diskCheck", "etcdReachable", "captureTopo", "captureAddon"}
	for _, pf := range preFlightCalls {
		pfPos, pfOk := positions[pf]
		if !pfOk {
			t.Errorf("pre-flight call %q was never made", pf)
			continue
		}
		pausePos, pauseOk := positions["pauseFn"]
		if !pauseOk {
			t.Errorf("pauseFn was never called")
			continue
		}
		if pfPos >= pausePos {
			t.Errorf("pre-flight call %q (pos %d) must precede pauseFn (pos %d)", pf, pfPos, pausePos)
		}
	}

	// Restore operations must follow pauseFn.
	restoreOps := []string{"restoreEtcd", "restoreImages", "restorePVs"}
	for _, op := range restoreOps {
		opPos, opOk := positions[op]
		if !opOk {
			t.Errorf("restore op %q was never called", op)
			continue
		}
		pausePos := positions["pauseFn"]
		if opPos <= pausePos {
			t.Errorf("restore op %q (pos %d) must come after pauseFn (pos %d)", op, opPos, pausePos)
		}
	}

	// resumeFn must come last (after all restore ops).
	resumePos, resumeOk := positions["resumeFn"]
	if !resumeOk {
		t.Error("resumeFn was never called")
	}
	for _, op := range restoreOps {
		opPos := positions[op]
		if opPos >= resumePos {
			t.Errorf("restore op %q (pos %d) must precede resumeFn (pos %d)", op, opPos, resumePos)
		}
	}
}

func TestRestorePostPauseFailure_ErrorMentionsRecoveryHint(t *testing.T) {
	root := t.TempDir()
	clusterName := "test-cluster"
	snapName := "test-snap"
	recorder := &callRecorder{}

	buildTestBundle(t, root, clusterName, snapName, defaultSnapMeta(clusterName, snapName))

	opts := buildTestRestoreOpts(t, recorder, root, clusterName, snapName, nil)
	// Make RestoreEtcd fail AFTER pause.
	opts.restoreEtcdFn = func(_ context.Context, _ EtcdRestoreOptions) error {
		recorder.record("restoreEtcd")
		return errors.New("etcd restore simulated failure")
	}

	_, err := Restore(opts)
	if err == nil {
		t.Fatal("expected error from post-pause RestoreEtcd failure, got nil")
	}

	errMsg := err.Error()
	t.Logf("error: %s", errMsg)

	// Error message MUST mention recovery hint.
	if !strings.Contains(errMsg, "kinder resume") {
		t.Errorf("error does not mention 'kinder resume': %q", errMsg)
	}
	if !strings.Contains(strings.ToLower(errMsg), "inconsistent") &&
		!strings.Contains(strings.ToLower(errMsg), "paused") {
		t.Errorf("error does not mention cluster inconsistent/paused state: %q", errMsg)
	}

	// resumeFn must NOT have been called (no auto-rollback per CONTEXT.md).
	for _, call := range recorder.sequence() {
		if call == "resumeFn" {
			t.Error("resumeFn was called after post-pause failure (violates no-rollback policy)")
		}
	}
}
