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

// create.go — top-level Create orchestrator.
//
// Ordering per CONTEXT.md locked decision:
//   1. captureTopology + captureAddonVersions WHILE running (need kubectl)
//   2. captureEtcd WHILE running (etcd must be running)
//   3. lifecycle.Pause — defer lifecycle.Resume on function exit (always resumes)
//   4. captureImages (post-pause; cluster containers are running but kubelet/etcd off)
//   5. capturePVs (post-pause)
//   6. bundle.WriteBundle
//   7. (deferred) lifecycle.Resume
//
// The defer-Resume guarantee intentionally differs from Restore's "no rollback"
// policy: Create is READ-ONLY at the cluster level (no etcd/PV mutation), so
// always resuming is safe and user-friendly. Leaving a cluster paused because
// a read-only capture failed is unnecessary disruption.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/log"
)

// defaultDiskSpaceMinBytes is the minimum free disk space threshold used by
// Create when no better estimate is available. 8 GiB covers a typical kind
// cluster with images (~5 GB) plus etcd snapshot and PV headroom.
const defaultDiskSpaceMinBytes = 8 * 1024 * 1024 * 1024 // 8 GiB

// Package-level injection points. Tests overwrite these via CreateOptions
// injection fields; production callers leave them nil to use the defaults.
// NOTE: these are vars rather than constants so tests can override them
// via the options struct; the pattern matches lifecycle/pause.go:51.

// CreateOptions configures a single Create invocation. Unexported fields are
// injection points for unit tests; production callers leave them nil/zero.
type CreateOptions struct {
	// ClusterName is the kinder cluster to snapshot. Required.
	ClusterName string
	// SnapName is the snapshot name. If empty, a name is auto-generated as
	// "snap-YYYYMMDD-HHMMSS" (CONTEXT.md locked default).
	SnapName string
	// Root is the snapshot store root directory. Defaults to DefaultRoot().
	Root string
	// Logger receives progress lines. Defaults to log.NoopLogger{}.
	Logger log.Logger
	// Provider is the cluster provider used to list nodes. Required in
	// production; tests inject nodeFetcherFn instead.
	Provider interface {
		ListNodes(name string) ([]nodes.Node, error)
	}
	// Context is passed to capture functions. Defaults to context.Background().
	Context context.Context

	// --- injection points (used by tests, not by production callers) ---

	// nodeFetcherFn replaces Provider.ListNodes in tests.
	nodeFetcherFn func(clusterName string) ([]nodes.Node, error)

	// diskSpaceFn replaces EnsureDiskSpace in tests.
	diskSpaceFn func(path string, required int64) error

	// pauseFn replaces lifecycle.Pause in tests.
	pauseFn func(lifecycle.PauseOptions) (*lifecycle.PauseResult, error)

	// resumeFn replaces lifecycle.Resume in tests.
	resumeFn func(lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error)

	// classifyFn replaces lifecycle.ClassifyNodes in tests.
	classifyFn ClassifyFn

	// captureEtcd replaces CaptureEtcd in tests.
	captureEtcd func(ctx context.Context, cp nodes.Node, dstPath string) (string, error)

	// captureImages replaces CaptureImages in tests.
	captureImages func(ctx context.Context, cp nodes.Node, dstPath string) (string, error)

	// capturePVs replaces CapturePVs in tests.
	capturePVs func(ctx context.Context, allNodes []nodes.Node, dstPath string) (string, error)

	// captureTopo replaces CaptureTopology in tests.
	captureTopo func(ctx context.Context, allNodes []nodes.Node, classify ClassifyFn, providerBin string) (TopologyInfo, string, string, error)

	// captureAddon replaces CaptureAddonVersions in tests.
	captureAddon func(ctx context.Context, cp nodes.Node) (map[string]string, error)

	// writeBundleFn replaces WriteBundle in tests.
	writeBundleFn func(ctx context.Context, destPath string, comps []Component, meta *Metadata) (string, error)
}

// CreateResult is the structured return value of Create.
type CreateResult struct {
	// SnapName is the snapshot name (may be auto-generated).
	SnapName string
	// Path is the absolute path of the written .tar.gz archive.
	Path string
	// Size is the size of the .tar.gz file in bytes.
	Size int64
	// Metadata is the metadata written into the bundle.
	Metadata *Metadata
	// DurationS is the wall-clock duration of Create in seconds.
	DurationS float64
}

// Create orchestrates the full snapshot capture flow:
//
//  1. Resolve defaults (context, snap name, root, logger, inject fns).
//  2. List + classify cluster nodes.
//  3. Disk-space pre-check.
//  4. captureTopology + captureAddonVersions WHILE running.
//  5. captureEtcd WHILE running.
//  6. Pause (defer Resume on all exit paths).
//  7. captureImages + capturePVs (post-pause).
//  8. WriteBundle.
//  9. (deferred) Resume.
func Create(opts CreateOptions) (*CreateResult, error) {
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
			return nil, fmt.Errorf("create snapshot: resolve root: %w", err)
		}
		opts.Root = r
	}
	if opts.SnapName == "" {
		opts.SnapName = "snap-" + time.Now().UTC().Format("20060102-150405")
	}

	// Wire injection defaults.
	listNodesFn := opts.nodeFetcherFn
	if listNodesFn == nil {
		if opts.Provider == nil {
			return nil, fmt.Errorf("create snapshot: Provider is required")
		}
		listNodesFn = opts.Provider.ListNodes
	}
	diskFn := opts.diskSpaceFn
	if diskFn == nil {
		diskFn = EnsureDiskSpace
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
	etcdFn := opts.captureEtcd
	if etcdFn == nil {
		etcdFn = CaptureEtcd
	}
	imagesFn := opts.captureImages
	if imagesFn == nil {
		imagesFn = CaptureImages
	}
	pvsFn := opts.capturePVs
	if pvsFn == nil {
		pvsFn = CapturePVs
	}
	topoFn := opts.captureTopo
	if topoFn == nil {
		topoFn = CaptureTopology
	}
	addonFn := opts.captureAddon
	if addonFn == nil {
		addonFn = CaptureAddonVersions
	}
	wbFn := opts.writeBundleFn
	if wbFn == nil {
		wbFn = WriteBundle
	}

	clusterName := opts.ClusterName
	snapName := opts.SnapName
	ctx := opts.Context

	// --- Step 1: List nodes ---
	allNodes, err := listNodesFn(clusterName)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: list nodes for %q: %w", clusterName, err)
	}
	if len(allNodes) == 0 {
		return nil, fmt.Errorf("create snapshot: cluster %q has no nodes", clusterName)
	}

	// --- Step 2: Create store + get snapshot directory ---
	store, err := NewStore(opts.Root, clusterName)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: new store: %w", err)
	}
	snapDir := store.SnapshotRoot()

	// --- Step 3: Disk-space pre-check ---
	// Threshold: max(defaultDiskSpaceMinBytes, 500 MB) — we use a fixed 8 GiB
	// because we cannot estimate image size without listing them first
	// (chicken-and-egg). The plan explicitly allows this approach.
	threshold := int64(defaultDiskSpaceMinBytes)
	if threshold < 500*1024*1024 {
		threshold = 500 * 1024 * 1024
	}
	if err := diskFn(snapDir, threshold); err != nil {
		return nil, fmt.Errorf("create snapshot: disk pre-check: %w", err)
	}

	// --- Step 4: Capture topology + addon versions WHILE running ---
	// These need kubectl against the live apiserver.
	topo, k8sVersion, nodeImage, err := topoFn(ctx, allNodes, classifyFn, "")
	if err != nil {
		return nil, fmt.Errorf("create snapshot: capture topology: %w", err)
	}

	// Get bootstrap CP node for single-node operations.
	cp, _, _, err := classifyFn(allNodes)
	if err != nil || len(cp) == 0 {
		return nil, fmt.Errorf("create snapshot: classify nodes: %w", err)
	}
	bootstrap := cp[0]

	addonVersions, err := addonFn(ctx, bootstrap)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: capture addon versions: %w", err)
	}

	// --- Step 5: Create temp directory for captured components ---
	tmpDir, err := os.MkdirTemp("", "kinder-snap-*")
	if err != nil {
		return nil, fmt.Errorf("create snapshot: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// --- Step 6: captureEtcd WHILE running (etcd must be running pre-pause) ---
	etcdDst := filepath.Join(tmpDir, EntryEtcd)
	etcdDigest, err := etcdFn(ctx, bootstrap, etcdDst)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: capture etcd: %w", err)
	}

	// --- Step 7: Pause (DEFER Resume on all exit paths) ---
	// Create is read-only at the cluster level (no etcd/PV mutation), so always
	// resuming is safe and user-friendly — contrast with Restore's "no rollback."
	pauseResult, err := pauseFn(lifecycle.PauseOptions{
		ClusterName: clusterName,
		Logger:      opts.Logger,
	})
	if err != nil && pauseResult == nil {
		// Fatal pause failure (cluster not found, etc.) — cannot proceed.
		return nil, fmt.Errorf("create snapshot: pause cluster: %w", err)
	}
	// Non-nil pauseResult means the call succeeded or was idempotent (already paused).
	// Resume must run on ALL subsequent exit paths.
	var resumeErr error
	defer func() {
		_, resumeErr = resumeFn(lifecycle.ResumeOptions{
			ClusterName: clusterName,
			Logger:      opts.Logger,
			Provider:    opts.Provider,
			Context:     ctx,
		})
		if resumeErr != nil {
			opts.Logger.Warnf("create snapshot: resume cluster %q after capture: %v", clusterName, resumeErr)
		}
	}()

	// --- Step 8: captureImages (post-pause) ---
	imagesDst := filepath.Join(tmpDir, EntryImages)
	imagesDigest, err := imagesFn(ctx, bootstrap, imagesDst)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: capture images: %w", err)
	}

	// --- Step 9: capturePVs (post-pause) ---
	pvsDst := filepath.Join(tmpDir, EntryPVs)
	pvsDigest, err := pvsFn(ctx, allNodes, pvsDst)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: capture pvs: %w", err)
	}

	// --- Step 10: Reconstruct kind config (pure function) ---
	configBytes, err := ReconstructKindConfig(topo, k8sVersion, nodeImage, addonVersions)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: reconstruct kind config: %w", err)
	}
	configDst := filepath.Join(tmpDir, EntryConfig)
	if err := os.WriteFile(configDst, configBytes, 0600); err != nil {
		return nil, fmt.Errorf("create snapshot: write kind config: %w", err)
	}

	// --- Step 11: Build metadata ---
	meta := &Metadata{
		SchemaVersion: MetadataVersion,
		Name:          snapName,
		ClusterName:   clusterName,
		CreatedAt:     time.Now().UTC(),
		K8sVersion:    k8sVersion,
		NodeImage:     nodeImage,
		Topology:      topo,
		AddonVersions: addonVersions,
		EtcdDigest:    etcdDigest,
		ImagesDigest:  imagesDigest,
		PVsDigest:     pvsDigest,
	}

	// --- Step 12: WriteBundle ---
	archivePath := store.Path(snapName)
	comps := []Component{
		{Name: EntryEtcd, Path: etcdDst},
		{Name: EntryImages, Path: imagesDst},
		{Name: EntryPVs, Path: pvsDst},
		{Name: EntryConfig, Path: configDst},
	}
	archiveDigest, err := wbFn(ctx, archivePath, comps, meta)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: write bundle: %w", err)
	}
	meta.ArchiveDigest = archiveDigest

	// --- Step 13: Stat the archive for size ---
	fi, err := os.Stat(archivePath)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: stat archive: %w", err)
	}

	return &CreateResult{
		SnapName:  snapName,
		Path:      archivePath,
		Size:      fi.Size(),
		Metadata:  meta,
		DurationS: time.Since(t0).Seconds(),
	}, nil
}
