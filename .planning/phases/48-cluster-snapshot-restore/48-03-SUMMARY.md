---
phase: 48-cluster-snapshot-restore
plan: 03
subsystem: infra
tags: [etcd, restore, images, pvs, snapshot, kind, kubernetes, tdd]

# Dependency graph
requires:
  - phase: 48-01
    provides: bundle.go (OpenBundle), metadata.go (Metadata struct, entry constants)
  - phase: 48-02
    provides: pvs.go CapturePVs nested-tar layout (nodeName/local-path-provisioner.tar entries)
provides:
  - RestoreEtcd: HA-safe etcd restore with fresh cluster token, manifest bracketing, data-dir atomic swap
  - RestoreImages: thin wrapper over nodeutils.LoadImageArchiveWithFallback with 0-byte gate
  - RestorePVs: per-node nested tar dispatch to /opt/local-path-provisioner, 0-byte no-op
  - EtcdRestoreOptions struct: CPs, SnapshotHostPath, Token, ProviderBin
  - ImportArchiveFn type: injectable function for testing image import
affects:
  - 48-04 (plan 04 orchestrates these three primitives inside Restore flow)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Injectable function type (ImportArchiveFn) as test seam to avoid shelling out to containerd"
    - "Package-level var (etcdManifestSettleDelay) for test-injectable sleep duration"
    - "snapFakeNode/snapFakeCmd with stdin capture for asserting exact node command sequences"
    - "deferred manifest restore pattern: mv aside at start, defer mv back on all exit paths"
    - "errors.AggregateConcurrent for HA parallel per-CP restore"

key-files:
  created:
    - pkg/internal/snapshot/etcdrestore.go
    - pkg/internal/snapshot/etcdrestore_test.go
    - pkg/internal/snapshot/imagesrestore.go
    - pkg/internal/snapshot/imagesrestore_test.go
    - pkg/internal/snapshot/pvsrestore.go
    - pkg/internal/snapshot/pvsrestore_test.go
  modified: []

key-decisions:
  - "etcdctl invoked directly on kindest/node PATH (not via crictl exec), consistent with RESEARCH OQ-6 finding"
  - "HA token generated once before goroutine fan-out; all CPs share identical token and initial-cluster string"
  - "Manifest restore via defer (not explicit call) to guarantee rollback on all error paths including panic"
  - "etcdManifestSettleDelay is a package-level var (not a parameter) to keep EtcdRestoreOptions clean while allowing test injection"
  - "RestorePVs reads all inner bytes before piping to SetStdin (vs streaming) to avoid tar reader position issues with concurrent reads"
  - "Unknown nodes in pvs.tar: warn to stderr, skip, no error — tolerates renamed/removed nodes post-snapshot"

patterns-established:
  - "Restore primitive pattern: 0-byte gate → parse/dispatch → aggregate errors (matches Plan 02 capture pattern)"

# Metrics
duration: 11min
completed: 2026-05-06
---

# Phase 48 Plan 03: Restore Sources Summary

**Three restore primitives — RestoreEtcd (HA-safe, manifest-bracketed, data-dir atomic swap), RestoreImages (nodeutils.LoadImageArchiveWithFallback wrapper), RestorePVs (per-node nested-tar dispatch) — TDD'd with FakeNode stdin capture**

## Performance

- **Duration:** 11 min
- **Started:** 2026-05-06T12:46:54Z
- **Completed:** 2026-05-06T12:57:46Z
- **Tasks:** 2
- **Files modified:** 6 created

## Accomplishments

- RestoreEtcd handles single-CP and 3-CP HA: manifest moved aside, etcdctl snapshot restore, atomic data-dir swap with rollback, deferred manifest restore
- HA safety: token generated once outside goroutine fan-out → all 3 etcdctl invocations use identical `--initial-cluster-token` and `--initial-cluster` (RESEARCH Pitfall 5)
- RestoreImages: thin wrapper around `nodeutils.LoadImageArchiveWithFallback` (no reimplementation); 0-byte gate; ImportArchiveFn injection for tests
- RestorePVs: iterates Plan 02's nested outer tar (`<nodeName>/local-path-provisioner.tar`), dispatches inner tar bytes via `tar -xf - -C /opt` SetStdin, 0-byte no-op, unknown nodes warn-and-skip
- All functions accept context.Context and return aggregated errors via errors.NewAggregate / errors.AggregateConcurrent
- 9 unit tests passing with race detector; zero circular imports to lifecycle or doctor packages

## Task Commits

TDD (RED → GREEN) per task:

1. **Task 1 RED: TestRestoreEtcd (5 tests)** - `d4605d49` (test)
2. **Task 1 GREEN: RestoreEtcd implementation** - `467db58e` (feat)
3. **Task 2 RED: TestRestoreImages + TestRestorePVs (7 tests)** - `94c49d79` (test)
4. **Task 2 GREEN: RestoreImages + RestorePVs implementation** - `0ede8bac` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified

- `pkg/internal/snapshot/etcdrestore.go` - RestoreEtcd, EtcdRestoreOptions, buildInitialCluster, restoreSingleCP, etcdctlRestore
- `pkg/internal/snapshot/etcdrestore_test.go` - 5 tests: sequence, HA same-token, etcdctl failure rollback, data-swap failure rollback, IP lookup; snapFakeNode/snapFakeCmd fakes
- `pkg/internal/snapshot/imagesrestore.go` - RestoreImages, ImportArchiveFn type, defaultImportArchive
- `pkg/internal/snapshot/imagesrestore_test.go` - 3 tests: injected fn called, empty-file gate, error propagation
- `pkg/internal/snapshot/pvsrestore.go` - RestorePVs, newBytesReader
- `pkg/internal/snapshot/pvsrestore_test.go` - 4 tests: empty file, single-node data, unknown node ignored, untar failure aggregated

## Decisions Made

- **etcdctl on node PATH**: Plan says to verify during implementation; test fakes bypass real binary; production assumption is etcdctl is on kindest/node PATH per RESEARCH OQ-6. No fallback to `crictl run` was implemented (RESEARCH Pitfall 4 resolution is deferred to Plan 04 if a real cluster test reveals etcdctl absent).
- **etcdManifestSettleDelay var**: kept as package-level rather than EtcdRestoreOptions field to avoid polluting the public API for a test-only concern.
- **RestorePVs stdin approach**: reads all inner bytes into memory before `SetStdin` (not streaming) because `tar.Reader` position advances with each `tr.Next()` call; buffering is safe at the scales kinder targets.

## Deviations from Plan

None — all 5 RestoreEtcd tests and all 7 RestoreImages/RestorePVs tests implemented exactly as specified in the plan. The `withZeroSettleDelay` helper was added to avoid 5-second sleeps in tests (not a deviation, it's a test-ergonomics addition within scope of the test task).

## Issues Encountered

- **Predicate ambiguity in TestRestoreEtcdSingleCP_Sequence**: initial `hasArg` predicates matched both `mv manifest aside` and `mv manifest back` because both contain the same two path arguments. Fixed by using directional arg checks (`c.args[0]` / `c.args[1]`) rather than `hasArg` (order-independent).
- **Deferred call ordering**: `mv manifest back` runs via `defer` AFTER `return nil` fires, so the ordering check must be `rm tmp snap → mv manifest back` (not the reverse). Test updated accordingly.

## Next Phase Readiness

- Plan 04 can import RestoreEtcd, RestoreImages, RestorePVs directly from `pkg/internal/snapshot`
- Plan 04 must call `lifecycle.Pause` before RestoreEtcd and `lifecycle.Resume` after; these primitives assume CP containers are running
- etcdManifestSettleDelay (5s default) may need tuning based on real cluster testing in Plan 04

---
*Phase: 48-cluster-snapshot-restore*
*Completed: 2026-05-06*

## Self-Check: PASSED

Files verified present:
- pkg/internal/snapshot/etcdrestore.go: FOUND
- pkg/internal/snapshot/etcdrestore_test.go: FOUND
- pkg/internal/snapshot/imagesrestore.go: FOUND
- pkg/internal/snapshot/imagesrestore_test.go: FOUND
- pkg/internal/snapshot/pvsrestore.go: FOUND
- pkg/internal/snapshot/pvsrestore_test.go: FOUND

Commits verified present:
- d4605d49: FOUND (test RED etcdrestore)
- 467db58e: FOUND (feat GREEN etcdrestore)
- 94c49d79: FOUND (test RED images+pvs)
- 0ede8bac: FOUND (feat GREEN images+pvs)
