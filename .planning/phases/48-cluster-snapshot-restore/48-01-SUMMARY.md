---
phase: 48-cluster-snapshot-restore
plan: "01"
subsystem: snapshot
tags: [snapshot, bundle, tar-gz, sha256, prune, store, tdd]
dependency_graph:
  requires: []
  provides:
    - pkg/internal/snapshot (Metadata, BundleReader, SnapshotStore, prune policies)
  affects:
    - Phase 48 Plan 02 (capture — calls WriteBundle)
    - Phase 48 Plan 03 (restore — calls OpenBundle / VerifyBundle)
    - Phase 48 Plan 04 (orchestrators — calls SnapshotStore.List, PrunePlan)
    - Phase 48 Plan 05 (CLI commands — calls NewStore, List, Delete)
tech_stack:
  added: []
  patterns:
    - Single-pass streaming sha256 via io.MultiWriter(file, sha256.New()) — no file re-read
    - stdlib-only snapshot layer (archive/tar, compress/gzip, crypto/sha256, encoding/hex)
    - Typed error sentinels (ErrCorruptArchive, ErrMissingSidecar, ErrSnapshotNotFound)
    - Pure prune-policy functions operating on []Info (no I/O, no side effects)
    - TDD RED-GREEN per task (6 conventional commits: 3 test + 3 feat)
key_files:
  created:
    - pkg/internal/snapshot/doc.go
    - pkg/internal/snapshot/metadata.go
    - pkg/internal/snapshot/metadata_test.go
    - pkg/internal/snapshot/bundle.go
    - pkg/internal/snapshot/bundle_test.go
    - pkg/internal/snapshot/store.go
    - pkg/internal/snapshot/store_test.go
    - pkg/internal/snapshot/prune.go
    - pkg/internal/snapshot/prune_test.go
  modified: []
decisions:
  - "ArchiveDigest is left empty inside the tarred metadata.json; sidecar .sha256 is the single source of truth for the archive digest. Callers that cache Metadata in memory may populate ArchiveDigest from WriteBundle's return value."
  - "ErrMissingSidecar is a distinct sentinel from ErrCorruptArchive — missing sidecar is an operational error (write interrupted), not a data-integrity failure."
  - "SnapshotStore.List performs full VerifyBundle (re-hash) for accurate Status. Fast-path (Status='unknown' without re-hash) deferred to Plan 05 CLI command via StatusFast/StatusFull mode distinction documented inline."
  - "bundleReader loads entire archive into memory on OpenBundle — acceptable for restore path because large entries (images.tar, pvs.tar) are extracted to temp files anyway; avoids seeking on non-seekable gzip streams."
  - "PrunePlan uses UNION semantics: a snapshot is deleted if ANY active policy marks it. Zero-value Policy fields are inactive (0 = no deletions for that field)."
  - "MkdirAll with 0700 for storage root and per-cluster directory — etcd snapshots contain Secrets and must not be world-readable."
metrics:
  duration: "~7m"
  completed: "2026-05-06"
  tasks: 3
  tdd_commits: 6
---

# Phase 48 Plan 01: Snapshot Package Foundation Summary

Delivered `pkg/internal/snapshot/` — a stdlib-only foundational package providing metadata schema, single-pass tar.gz bundle writing with streaming SHA-256, on-disk snapshot store backed by `~/.kinder/snapshots/<cluster>/`, and pure prune-policy filters. Zero imports from sigs.k8s.io/kind/pkg/cluster or lifecycle packages.

## What Was Built

### metadata.go

Metadata struct with all LIFE-08 fields:

- `SchemaVersion` ("1") enforced on unmarshal — forward-compat alarm for future schema changes
- `K8sVersion`, `NodeImage`, `TopologyInfo` (ControlPlaneCount, WorkerCount, HasLoadBalancer)
- `AddonVersions map[string]string` (empty map = no addons; no nil/missing distinction)
- Per-component digests: `EtcdDigest`, `ImagesDigest`, `PVsDigest`, `ConfigDigest`, `ArchiveDigest`
- 5 entry-name constants: `EntryEtcd`, `EntryImages`, `EntryPVs`, `EntryConfig`, `EntryMetadata`
- `MarshalMetadata` / `UnmarshalMetadata` thin wrappers over stdlib `json.MarshalIndent`

### bundle.go

Single-pass streaming bundle writer and reader:

- `WriteBundle(ctx, destPath, comps, meta) -> (archiveDigest, error)`: streams components through `gzip.NewWriter -> tar.NewWriter` with `io.MultiWriter(file, sha256.New())` capturing digest without re-reading the file. Sidecar written as sha256sum convention (`<hex>  <basename>.tar.gz\n`).
- `OpenBundle(path) -> BundleReader`: decompresses archive into in-memory entry map for random-access Open() by entry name.
- `VerifyBundle(path) error`: re-hashes archive, compares against sidecar. Returns `ErrCorruptArchive` on mismatch, `ErrMissingSidecar` if sidecar absent.
- `BundleReader` interface: `Metadata()`, `Open(entryName) io.ReadCloser`, `Close()`
- `BundleEntries` ordered slice for consistent archive layout

### store.go

On-disk snapshot store:

- `DefaultRoot() (string, error)`: `~/.kinder/snapshots` via `os.UserHomeDir`
- `NewStore(root, clusterName) (*SnapshotStore, error)`: `mkdir -p <root>/<cluster>` with 0700
- `Info` struct: Name, ClusterName, Path, Size (archive + sidecar), CreatedAt (mtime), Metadata, Status
- `List(ctx)`: glob `*.tar.gz`, stat, OpenBundle for metadata, VerifyBundle for status, sort newest-first
- `Open(ctx, name)`: returns `BundleReader + *Info`; `ErrSnapshotNotFound` if missing
- `Delete(ctx, name)`: removes both `.tar.gz` + `.sha256`; `ErrSnapshotNotFound` if not present
- `Path(name) string`: `<root>/<cluster>/<name>.tar.gz`

### prune.go

Pure prune-policy functions (no I/O, no side effects):

- `KeepLast(infos, n)`: delete everything past the first n items
- `OlderThan(infos, d, now)`: delete items where `now.Sub(CreatedAt) > d`
- `MaxSize(infos, max)`: delete oldest-first until `sum(Size) <= max`
- `PrunePlan(infos, policy, now)`: UNION composition — snapshot deleted if matched by ANY active policy

## TDD Gate Compliance

All 3 tasks followed strict RED-GREEN TDD:

| Gate  | Task 1 Commit | Task 2 Commit | Task 3 Commit |
|-------|---------------|---------------|---------------|
| RED   | `59e3ead2`    | `692525c8`    | `e9d4a266`    |
| GREEN | `5bc803bd`    | `5ac3c2c7`    | `7e08262b`    |

## Test Coverage

17 tests across 4 test files, all with `t.Parallel()`:

- `TestMetadataRoundTrip` (3 sub-cases: full, minimal, missing schemaVersion)
- `TestBundleRoundTrip`, `TestBundleCorruptionDetected`, `TestBundleMissingSidecar`, `TestBundleEmptyPVs`, `TestBundleContextCancellation`
- `TestStoreList`, `TestStoreOpenMissing`, `TestStoreDelete`, `TestStoreOpen`
- `TestKeepLast`, `TestKeepLastExact`, `TestKeepLastZero`, `TestOlderThan`, `TestMaxSize`, `TestMaxSizeNoDeletion`, `TestPrunePolicies` (6 sub-cases)

All pass: `go test -race ./pkg/internal/snapshot/...` — no races detected.

## Deviations from Plan

### Auto-added Tests (Rule 2)

**1. [Rule 2 - Missing] TestBundleContextCancellation**
- Added beyond the plan's 4 bundle tests to verify context.Context cancellation before write start
- Context cancellation is an API contract per the plan's requirement that all APIs accept context.Context
- No implementation change needed (WriteBundle already checked ctx.Err())

**2. [Rule 2 - Missing] TestKeepLastExact, TestKeepLastZero**
- Added beyond the plan's TestKeepLast to cover boundary cases (n == len, n == 0)
- Boundary tests are correctness requirements for pure functions

**3. [Rule 2 - Missing] TestMaxSizeNoDeletion, TestStoreOpen**
- TestMaxSizeNoDeletion: verifies the "no deletions when total <= max" early-exit path
- TestStoreOpen: verifies the Open() happy path returns working BundleReader

### Implementation Decision (Claude's Discretion)

**ArchiveDigest inside tar is empty.** Per plan note: including ArchiveDigest in metadata.json that is itself tarred is recursive. Resolution: ArchiveDigest inside the tar is always ""; the sidecar is the single source of truth. Documented inline in WriteBundle and store.go.

**bundleReader is in-memory.** OpenBundle reads all entries into a `map[string][]byte`. For Plan 03 restore this is acceptable because images.tar and pvs.tar are extracted to temp files (not held in RAM). Avoids complexity of maintaining a seekable gzip reader.

## Threat Flags

None. This plan creates no network endpoints, no auth paths, and no new trust boundaries. File access is restricted to `~/.kinder/snapshots/` with 0700 mode.

## Known Stubs

None. All exported APIs are fully implemented and tested.

## Self-Check: PASSED

Files created and confirmed present:
- pkg/internal/snapshot/doc.go
- pkg/internal/snapshot/metadata.go
- pkg/internal/snapshot/metadata_test.go
- pkg/internal/snapshot/bundle.go
- pkg/internal/snapshot/bundle_test.go
- pkg/internal/snapshot/store.go
- pkg/internal/snapshot/store_test.go
- pkg/internal/snapshot/prune.go
- pkg/internal/snapshot/prune_test.go

Commits confirmed in git log:
- 59e3ead2 (RED: metadata test)
- 5bc803bd (GREEN: metadata)
- 692525c8 (RED: bundle test)
- 5ac3c2c7 (GREEN: bundle)
- e9d4a266 (RED: store+prune test)
- 7e08262b (GREEN: store+prune)

All 17 tests pass: `go test -race ./pkg/internal/snapshot/...`
`go vet ./pkg/internal/snapshot/...` — clean
No cluster imports: `grep -r '"sigs.k8s.io/kind/pkg/cluster"' pkg/internal/snapshot/` — empty
