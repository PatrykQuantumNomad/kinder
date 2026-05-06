---
phase: 48-cluster-snapshot-restore
plan: 04
subsystem: snapshot
tags: [lifecycle, pause, resume, etcd, tdd, orchestration, compatibility]

# Dependency graph
requires:
  - phase: 48-01
    provides: "Metadata schema, WriteBundle/VerifyBundle, SnapshotStore, prune policies"
  - phase: 48-02
    provides: "CaptureEtcd, CaptureImages, CapturePVs, CaptureTopology, CaptureAddonVersions, ClassifyFn"
  - phase: 48-03
    provides: "RestoreEtcd, RestoreImages, RestorePVs"
  - phase: 47
    provides: "lifecycle.Pause, lifecycle.Resume, lifecycle.ClassifyNodes, lifecycle.ResolveClusterName"
provides:
  - "snapshot.Create(CreateOptions) (*CreateResult, error) — full orchestrated snapshot capture"
  - "snapshot.Restore(RestoreOptions) (*RestoreResult, error) — full orchestrated restore with pre-flight gauntlet"
  - "CheckCompatibility(snap, live *Metadata) error — aggregated 3-dimension compat check"
  - "EnsureDiskSpace(path string, required int64) error — syscall.Statfs wrapper"
  - "ErrCompatK8sMismatch, ErrCompatTopologyMismatch, ErrCompatAddonMismatch sentinel errors"
  - "ErrClusterNotRunning sentinel error for Restore pre-flight"
  - "ErrInsufficientDiskSpace sentinel error"
affects: [48-05, 48-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Injection via unexported Options struct fields (pauseFn, resumeFn, classifyFn, captureFns) — matches lifecycle package Phase 47 test injection pattern"
    - "defer-Resume on Create (read-only capture; always safe to resume even on failure)"
    - "No-defer-Resume on Restore (mutation path; recovery hint error per CONTEXT.md no-rollback)"
    - "aggregated compat errors via kinderrors.NewAggregate — user sees all violations at once"
    - "ensureFromStatfs pure inner function — testable without real filesystem state"
    - "callRecorder for ordering assertions in TDD tests"

key-files:
  created:
    - pkg/internal/snapshot/compat.go
    - pkg/internal/snapshot/compat_test.go
    - pkg/internal/snapshot/diskspace.go
    - pkg/internal/snapshot/diskspace_test.go
    - pkg/internal/snapshot/create.go
    - pkg/internal/snapshot/create_test.go
    - pkg/internal/snapshot/restore.go
    - pkg/internal/snapshot/restore_test.go
  modified: []

key-decisions:
  - "Create defers Resume on all exit paths (read-only capture; no cluster mutation → always safe to resume)"
  - "Restore does NOT defer Resume on post-pause failure (no-rollback per CONTEXT.md locked decision)"
  - "Post-pause restore errors wrapped with recovery hint: 'run kinder resume <cluster> to restart'"
  - "CheckCompatibility aggregates ALL three dimension violations in one error (K8s + topology + addon)"
  - "Restore pre-flight captures live topology+addon via CaptureTopology/CaptureAddonVersions (requires running cluster)"
  - "EnsureDiskSpace has pure inner function ensureFromStatfs for portable testing via synthetic Statfs_t"
  - "ErrClusterNotRunning added as new sentinel for etcd reachability pre-flight check"
  - "Create disk-space threshold fixed at 8GiB (cannot estimate image size pre-capture — chicken-and-egg)"

patterns-established:
  - "Pre-flight all-before-mutation ordering enforced via test callRecorder assertions"
  - "Injection points in Options structs (unexported fields) with nil-defaults to production functions"

requirements-completed: [LIFE-05, LIFE-06]

# Metrics
duration: ~35min
completed: 2026-05-06
---

# Phase 48 Plan 04: Orchestrators (Create + Restore) Summary

**Create orchestrator (etcd-while-running → Pause → images+PVs → bundle → defer-Resume) and Restore orchestrator (full pre-flight gauntlet → Pause → etcd+images+PVs → Resume) composing Plans 01-03 primitives with lifecycle.Pause/Resume from Phase 47**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-05-06T13:00:00Z
- **Completed:** 2026-05-06T13:35:00Z
- **Tasks:** 3
- **Files created:** 8

## Accomplishments

- `snapshot.Create` orchestrates: topology+addons (while running) → etcd snapshot (while running) → Pause → images → PVs → WriteBundle → (deferred) Resume. Defer-Resume guarantees the cluster is always resumed even on intermediate capture failure.
- `snapshot.Restore` runs ALL 5 pre-flight checks before the first Pause call: VerifyBundle, disk-space (2x archive), etcd reachability, live topology+addon capture, CheckCompatibility. Post-pause failures include recovery hint error message with no auto-rollback.
- Three sentinel compat errors aggregated via kinderrors.NewAggregate so all violations surface in one error message instead of fix-rerun-discover loops.
- `EnsureDiskSpace` uses `syscall.Statfs` with a pure `ensureFromStatfs` inner function for portable testing.
- 27 new tests pass with `-race` (7 Create, 9 Restore, 7 compat, 4 diskspace) across all failure modes and ordering assertions.

## Task Commits

Each task was committed atomically:

1. **Task 1: Compatibility checks + disk-space pre-check** - `98f78649` (feat)
2. **Task 2: snapshot.Create orchestrator** - `d96f03bf` (feat)
3. **Task 3: snapshot.Restore orchestrator** - `a9cd681a` (feat)

**Plan metadata:** _(docs commit follows)_

## Files Created

- `pkg/internal/snapshot/compat.go` — CheckCompatibility with 3 sentinel errors + addonsEqual helper
- `pkg/internal/snapshot/compat_test.go` — 7 tests: happy path, K8s mismatch, topology mismatch, addon extra, addon drift, multiple violations, nil-vs-empty-map
- `pkg/internal/snapshot/diskspace.go` — EnsureDiskSpace (syscall.Statfs) + ensureFromStatfs pure inner fn
- `pkg/internal/snapshot/diskspace_test.go` — 4 tests: sufficient, missing path, synthetic insufficient, sentinel
- `pkg/internal/snapshot/create.go` — Create orchestrator with CreateOptions injection fields
- `pkg/internal/snapshot/create_test.go` — 7 tests: happy path (ordering), auto-name, explicit name, insufficient disk, pre-pause etcd failure, post-pause images failure (resume guaranteed), bundle write failure (resume guaranteed)
- `pkg/internal/snapshot/restore.go` — Restore orchestrator with RestoreOptions injection fields, probeEtcdReachable, extractEntry helper
- `pkg/internal/snapshot/restore_test.go` — 9 tests: snapshot not found, corrupt archive, insufficient disk, cluster not running, K8s mismatch, topology mismatch, addon mismatch, happy path (ordering), post-pause failure with recovery hint + no-auto-resume

## Decisions Made

- **defer-Resume on Create, NOT on Restore.** Create is read-only at the cluster level so resuming is always correct on failure. Restore mutates etcd/images/PVs — leaving the cluster paused after failure lets the user investigate before deciding to retry. CONTEXT.md locked decision honored.
- **Recovery hint error pattern.** Post-pause Restore failures wrap the underlying error with: `"cluster is in a paused/inconsistent state; run kinder resume <cluster> to restart, then re-attempt restore or recreate cluster"`. Test asserts both `kinder resume` and `inconsistent` appear in the error string.
- **Aggregated compat errors.** All three mismatches can appear simultaneously (K8s version + topology + addon). Using kinderrors.NewAggregate allows errors.Is() to drill into wrapped aggregate for each sentinel independently — confirmed in TestCompatMultipleViolations.
- **ErrClusterNotRunning sentinel.** New error added to pre-flight to cover the case where Restore is called on a stopped/paused cluster. Message includes a recovery hint to run `kinder resume` first.
- **Create disk threshold fixed at 8 GiB.** Cannot estimate image size before listing them (chicken-and-egg). The 8 GiB constant matches a typical kind cluster with full containerd image cache.
- **etcdReachableFn injection in RestoreOptions.** The etcd reachability probe runs crictl ps --name etcd -q which requires a real node. Injecting it via etcdReachableFn enables TestRestoreClusterNotRunning without a real cluster.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `execOutputLines` undefined in restore.go**
- **Found during:** Task 3 (snapshot.Restore implementation)
- **Issue:** Called `execOutputLines(...)` which doesn't exist in the snapshot package; etcd.go uses `exec.OutputLines(...)` from `sigs.k8s.io/kind/pkg/exec`
- **Fix:** Added `exec` import and used `exec.OutputLines(...)` directly (same pattern as etcd.go)
- **Files modified:** `pkg/internal/snapshot/restore.go`
- **Verification:** `go build ./pkg/internal/snapshot/...` passes
- **Committed in:** a9cd681a (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — trivial build error)
**Impact on plan:** No scope change; one-line fix to align with existing package pattern.

## Issues Encountered

None beyond the auto-fixed build error above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plan 48-05 (CLI): `kinder snapshot create/restore/list/show/prune` commands can now call `snapshot.Create` and `snapshot.Restore` directly. The Options structs accept Provider, Logger, and Context from cobra command context.
- The three compat sentinel errors are exported and testable via errors.Is for CLI error-message formatting in Plan 48-05.
- Plan 48-06 (integration tests): All injection points in CreateOptions/RestoreOptions enable table-driven integration tests on real clusters without needing to re-inject lifecycle behavior.

---
*Phase: 48-cluster-snapshot-restore*
*Completed: 2026-05-06*
