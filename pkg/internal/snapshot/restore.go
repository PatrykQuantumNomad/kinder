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

// restore.go — top-level Restore orchestrator.
//
// Ordering per CONTEXT.md "atomicity = pre-flight + fail-fast, no rollback":
//
//  PRE-FLIGHT (all before ANY mutation):
//    PF1. VerifyBundle: sha256 of archive must match sidecar.
//    PF2. Disk space: need ~2x archive size.
//    PF3. Etcd reachability: sanity probe via crictl ps --name etcd -q.
//    PF4. Capture LIVE topology + addon versions.
//    PF5. CheckCompatibility(snap, live) — hard-fail K8s/topology/addon.
//
//  MUTATION (no rollback on failure):
//    M1. Extract components to host tempdir.
//    M2. Pause cluster.
//    M3. RestoreEtcd (Plan 03).
//    M4. RestoreImages (Plan 03).
//    M5. RestorePVs (Plan 03).
//    M6. Resume cluster.
//
// IMPORTANT: unlike Create, Restore does NOT defer Resume on post-pause
// failure. On failure after Pause, the error is wrapped with a recovery hint
// instructing the user to run `kinder resume <cluster>` manually. This matches
// CONTEXT.md "no automatic rollback."

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/log"
)

// ErrClusterNotRunning is returned during Restore pre-flight when the etcd
// container is not reachable on the bootstrap CP node. This typically means
// the cluster is stopped/paused and must be resumed before restore can run.
var ErrClusterNotRunning = errors.New("cluster etcd is not reachable (cluster may be paused or stopped)")

// RestoreOptions configures a single Restore invocation.
type RestoreOptions struct {
	// ClusterName is the kinder cluster to restore into. Required.
	ClusterName string
	// SnapName is the name of the snapshot to restore. Required.
	SnapName string
	// Root is the snapshot store root directory. Defaults to DefaultRoot().
	Root string
	// SkipImages skips image restore (future-proofing; default false).
	SkipImages bool
	// Logger receives progress lines. Defaults to log.NoopLogger{}.
	Logger log.Logger
	// Provider is the cluster provider used to list nodes. Required in
	// production; tests inject nodeFetcherFn instead.
	Provider interface {
		ListNodes(name string) ([]nodes.Node, error)
	}
	// Context is passed to restore functions. Defaults to context.Background().
	Context context.Context

	// --- injection points (used by tests, not by production callers) ---

	// nodeFetcherFn replaces Provider.ListNodes in tests.
	nodeFetcherFn func(clusterName string) ([]nodes.Node, error)

	// pauseFn replaces lifecycle.Pause in tests.
	pauseFn func(lifecycle.PauseOptions) (*lifecycle.PauseResult, error)

	// resumeFn replaces lifecycle.Resume in tests.
	resumeFn func(lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error)

	// classifyFn replaces lifecycle.ClassifyNodes in tests.
	classifyFn ClassifyFn

	// verifyBundleFn replaces VerifyBundle in tests.
	verifyBundleFn func(archivePath string) error

	// diskSpaceFn replaces EnsureDiskSpace in tests.
	diskSpaceFn func(path string, required int64) error

	// etcdReachableFn probes etcd reachability on the bootstrap CP. Tests inject
	// a fake to control the cluster-running pre-flight check.
	etcdReachableFn func(ctx context.Context, bootstrap nodes.Node) error

	// captureLiveTopoFn replaces CaptureTopology for the live-state capture.
	captureLiveTopoFn func(ctx context.Context, allNodes []nodes.Node, classify ClassifyFn, providerBin string) (TopologyInfo, string, string, error)

	// captureAddonFn replaces CaptureAddonVersions for the live-state capture.
	captureAddonFn func(ctx context.Context, cp nodes.Node) (map[string]string, error)

	// restoreEtcdFn replaces RestoreEtcd in tests.
	restoreEtcdFn func(ctx context.Context, opts EtcdRestoreOptions) error

	// restoreImagesFn replaces RestoreImages in tests.
	restoreImagesFn func(ctx context.Context, allNodes []nodes.Node, imagesTarHostPath string, importFn ImportArchiveFn) error

	// restorePVsFn replaces RestorePVs in tests.
	restorePVsFn func(ctx context.Context, allNodes []nodes.Node, pvsTarHostPath string) error
}

// RestoreResult is the structured return value of Restore.
type RestoreResult struct {
	// SnapName is the snapshot that was restored.
	SnapName string
	// DurationS is the wall-clock duration of Restore in seconds.
	DurationS float64
	// Metadata is the snapshot metadata that was restored.
	Metadata *Metadata
}

// probeEtcdReachable is the default implementation of etcdReachableFn.
// It runs `crictl ps --name etcd -q` on the bootstrap CP node and verifies
// that at least one container ID is returned. An empty result means etcd is
// not running — the cluster may be paused or not started.
func probeEtcdReachable(ctx context.Context, bootstrap nodes.Node) error {
	idLines, err := exec.OutputLines(bootstrap.CommandContext(ctx, "crictl", "ps", "--name", "etcd", "-q"))
	if err != nil {
		return fmt.Errorf("%w: crictl ps failed: %v", ErrClusterNotRunning, err)
	}
	for _, line := range idLines {
		if id := strings.TrimSpace(line); id != "" {
			return nil // etcd is running
		}
	}
	return fmt.Errorf("%w: no etcd container found on bootstrap CP (run `kinder resume %s` first)",
		ErrClusterNotRunning, bootstrap.String())
}

// Restore orchestrates the full cluster restore flow:
//
//  PRE-FLIGHT (all before ANY mutation):
//    PF1. Open bundle / validate snapshot exists.
//    PF2. VerifyBundle sha256.
//    PF3. Disk-space check (2x archive size).
//    PF4. List + classify cluster nodes.
//    PF5. Probe etcd reachability.
//    PF6. Capture live topology + addon versions.
//    PF7. CheckCompatibility(snap, live).
//
//  MUTATION (no rollback):
//    M1. Extract components to tempdir.
//    M2. Pause cluster.
//    M3. RestoreEtcd.
//    M4. RestoreImages.
//    M5. RestorePVs.
//    M6. Resume cluster.
func Restore(opts RestoreOptions) (*RestoreResult, error) {
	t0 := time.Now()

	// --- Defaults ---
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	if opts.Logger == nil {
		opts.Logger = log.NoopLogger{}
	}
	if opts.Root == "" {
		r, err := DefaultRoot()
		if err != nil {
			return nil, fmt.Errorf("restore snapshot: resolve root: %w", err)
		}
		opts.Root = r
	}

	// Wire injection defaults.
	listNodesFn := opts.nodeFetcherFn
	if listNodesFn == nil {
		if opts.Provider == nil {
			return nil, fmt.Errorf("restore snapshot: Provider is required")
		}
		listNodesFn = opts.Provider.ListNodes
	}
	pauseFn := opts.pauseFn
	if pauseFn == nil {
		pauseFn = lifecycle.Pause
	}
	resumeFn := opts.resumeFn
	if resumeFn == nil {
		resumeFn = lifecycle.Resume
	}
	classifyFn := opts.classifyFn
	if classifyFn == nil {
		classifyFn = lifecycle.ClassifyNodes
	}
	verifyFn := opts.verifyBundleFn
	if verifyFn == nil {
		verifyFn = VerifyBundle
	}
	diskFn := opts.diskSpaceFn
	if diskFn == nil {
		diskFn = EnsureDiskSpace
	}
	etcdReachFn := opts.etcdReachableFn
	if etcdReachFn == nil {
		etcdReachFn = probeEtcdReachable
	}
	capTopoFn := opts.captureLiveTopoFn
	if capTopoFn == nil {
		capTopoFn = CaptureTopology
	}
	capAddonFn := opts.captureAddonFn
	if capAddonFn == nil {
		capAddonFn = CaptureAddonVersions
	}
	restEtcdFn := opts.restoreEtcdFn
	if restEtcdFn == nil {
		restEtcdFn = func(ctx context.Context, ropts EtcdRestoreOptions) error {
			return RestoreEtcd(ctx, ropts)
		}
	}
	restImagesFn := opts.restoreImagesFn
	if restImagesFn == nil {
		restImagesFn = RestoreImages
	}
	restPVsFn := opts.restorePVsFn
	if restPVsFn == nil {
		restPVsFn = RestorePVs
	}

	clusterName := opts.ClusterName
	snapName := opts.SnapName
	ctx := opts.Context

	// === PRE-FLIGHT (no mutations below this point) ===

	// PF1. Open bundle — validates snapshot exists (returns ErrSnapshotNotFound if absent).
	store, err := NewStore(opts.Root, clusterName)
	if err != nil {
		return nil, fmt.Errorf("restore snapshot: new store: %w", err)
	}

	bundleReader, info, err := store.Open(ctx, snapName)
	if err != nil {
		return nil, fmt.Errorf("restore snapshot: open snapshot %q: %w", snapName, err)
	}
	defer bundleReader.Close()

	snapMeta := bundleReader.Metadata()
	if snapMeta == nil {
		return nil, fmt.Errorf("restore snapshot: snapshot %q has no readable metadata", snapName)
	}

	// PF2. Verify bundle sha256 integrity.
	if err := verifyFn(info.Path); err != nil {
		return nil, fmt.Errorf("restore snapshot: verify bundle: %w", err)
	}

	// PF3. Disk-space check: need ~2x archive size.
	required := info.Size * 2
	if required < 500*1024*1024 { // minimum 500 MB
		required = 500 * 1024 * 1024
	}
	if err := diskFn(store.SnapshotRoot(), required); err != nil {
		return nil, fmt.Errorf("restore snapshot: disk pre-check: %w", err)
	}

	// PF4. List + classify cluster nodes.
	allNodes, err := listNodesFn(clusterName)
	if err != nil {
		return nil, fmt.Errorf("restore snapshot: list nodes for %q: %w", clusterName, err)
	}
	if len(allNodes) == 0 {
		return nil, fmt.Errorf("restore snapshot: cluster %q has no nodes", clusterName)
	}
	cp, _, _, err := classifyFn(allNodes)
	if err != nil || len(cp) == 0 {
		return nil, fmt.Errorf("restore snapshot: classify nodes: %w", err)
	}
	bootstrap := cp[0]

	// PF5. Probe etcd reachability (ensures cluster is running before we pause it).
	if err := etcdReachFn(ctx, bootstrap); err != nil {
		return nil, fmt.Errorf("restore snapshot: etcd reachability check: %w", err)
	}

	// PF6. Capture live topology + addon versions for compatibility comparison.
	liveTopo, liveK8sVersion, liveNodeImage, err := capTopoFn(ctx, allNodes, classifyFn, "")
	if err != nil {
		return nil, fmt.Errorf("restore snapshot: capture live topology: %w", err)
	}
	liveAddons, err := capAddonFn(ctx, bootstrap)
	if err != nil {
		return nil, fmt.Errorf("restore snapshot: capture live addon versions: %w", err)
	}

	liveMeta := &Metadata{
		K8sVersion:    liveK8sVersion,
		NodeImage:     liveNodeImage,
		Topology:      liveTopo,
		AddonVersions: liveAddons,
	}

	// PF7. Compatibility check — hard-fail on K8s/topology/addon mismatches.
	if err := CheckCompatibility(snapMeta, liveMeta); err != nil {
		return nil, fmt.Errorf("restore snapshot: compatibility check failed: %w", err)
	}

	// === MUTATION PHASE (no rollback on failure) ===
	// From this point, any failure is wrapped with a recovery hint per CONTEXT.md.
	// Do NOT defer Resume — leave the cluster paused so the user can investigate.

	// M1. Extract components from bundle to host tempdir.
	tmpDir, err := os.MkdirTemp("", "kinder-restore-*")
	if err != nil {
		return nil, fmt.Errorf("restore snapshot: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	etcdPath := filepath.Join(tmpDir, EntryEtcd)
	imagesPath := filepath.Join(tmpDir, EntryImages)
	pvsPath := filepath.Join(tmpDir, EntryPVs)

	if err := extractEntry(bundleReader, EntryEtcd, etcdPath); err != nil {
		return nil, fmt.Errorf("restore snapshot: extract etcd snapshot: %w", err)
	}
	if err := extractEntry(bundleReader, EntryImages, imagesPath); err != nil {
		return nil, fmt.Errorf("restore snapshot: extract images archive: %w", err)
	}
	if err := extractEntry(bundleReader, EntryPVs, pvsPath); err != nil {
		return nil, fmt.Errorf("restore snapshot: extract pvs archive: %w", err)
	}

	// wrapMutationError wraps post-pause errors with the recovery hint per CONTEXT.md.
	wrapMutationError := func(err error) error {
		if err == nil {
			return nil
		}
		hint := fmt.Sprintf(
			"cluster %q is in a paused/inconsistent state; "+
				"run `kinder resume %s` to restart, then re-attempt restore or recreate cluster",
			clusterName, clusterName,
		)
		return fmt.Errorf("%w\n\n*** Recovery hint: %s ***", err, hint)
	}

	// M2. Pause cluster.
	pauseResult, err := pauseFn(lifecycle.PauseOptions{
		ClusterName: clusterName,
		Logger:      opts.Logger,
	})
	if err != nil && pauseResult == nil {
		return nil, fmt.Errorf("restore snapshot: pause cluster: %w", err)
	}

	// M3. RestoreEtcd.
	if err := restEtcdFn(ctx, EtcdRestoreOptions{
		CPs:              cp,
		SnapshotHostPath: etcdPath,
		ProviderBin:      "",
	}); err != nil {
		return nil, wrapMutationError(fmt.Errorf("restore snapshot: restore etcd: %w", err))
	}

	// M4. RestoreImages.
	if !opts.SkipImages {
		if err := restImagesFn(ctx, allNodes, imagesPath, nil); err != nil {
			return nil, wrapMutationError(fmt.Errorf("restore snapshot: restore images: %w", err))
		}
	}

	// M5. RestorePVs.
	if err := restPVsFn(ctx, allNodes, pvsPath); err != nil {
		return nil, wrapMutationError(fmt.Errorf("restore snapshot: restore pvs: %w", err))
	}

	// M6. Resume cluster (quorum-safe restart + readiness gate).
	if _, err := resumeFn(lifecycle.ResumeOptions{
		ClusterName: clusterName,
		Logger:      opts.Logger,
		Provider:    opts.Provider,
		Context:     ctx,
	}); err != nil {
		return nil, wrapMutationError(fmt.Errorf("restore snapshot: resume cluster: %w", err))
	}

	return &RestoreResult{
		SnapName:  snapName,
		DurationS: time.Since(t0).Seconds(),
		Metadata:  snapMeta,
	}, nil
}

// extractEntry copies the named bundle entry to the destination path on disk.
func extractEntry(br BundleReader, entryName, dstPath string) error {
	rc, err := br.Open(entryName)
	if err != nil {
		return fmt.Errorf("open bundle entry %q: %w", entryName, err)
	}
	defer rc.Close()

	f, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create extract dest %q: %w", dstPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, rc); err != nil {
		return fmt.Errorf("extract %q to %q: %w", entryName, dstPath, err)
	}
	return nil
}
