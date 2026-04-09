---
phase: 45-host-directory-mounting
plan: "01"
subsystem: cluster-create
tags: [validation, preflight, mounts, propagation, darwin, windows]
requires: []
provides: [validateExtraMounts, logMountPropagationPlatformWarning]
affects: [pkg/cluster/internal/create/create.go]
tech-stack:
  added: []
  patterns: [pre-flight validation before Provision(), warn-and-continue pattern]
key-files:
  created:
    - pkg/cluster/internal/create/create_mount_test.go
  modified:
    - pkg/cluster/internal/create/create.go
key-decisions:
  - validateExtraMounts called after Config.Validate() and before Provision() — ensures host paths checked before any container is created
  - logMountPropagationPlatformWarning uses return-after-first-match to warn once, not per mount — mirrors logMetalLBPlatformWarning pattern
  - Relative paths resolved via filepath.Abs before os.Stat — avoids false negatives for callers with CWD-relative paths
  - Test file reuses existing testLogger from create_addon_test.go (same package) — avoids duplicate type declarations
duration: "2 minutes"
completed: "2026-04-09T16:38:00Z"
---

# Phase 45 Plan 01: Host Path Pre-flight Validation and Propagation Warning Summary

Pre-flight host-path existence validation and Docker Desktop propagation warning wired into cluster creation before any containers are provisioned.

## Performance

- Duration: ~2 minutes
- Start: 2026-04-09T16:35:45Z
- End: 2026-04-09T16:38:00Z
- Tasks completed: 2/2
- Files created: 1
- Files modified: 1

## Accomplishments

- Added `validateExtraMounts(cfg *config.Cluster) error` to `create.go`: iterates all node ExtraMounts, resolves relative paths via `filepath.Abs`, calls `os.Stat` to verify host path existence before any container is created; returns a structured error identifying which node and mount index failed.
- Added `logMountPropagationPlatformWarning(logger log.Logger, cfg *config.Cluster)` to `create.go`: on darwin/windows, emits a single warning when any extraMount specifies a non-None propagation mode, informing users that Docker Desktop silently ignores it.
- Wired both calls in `Cluster()` between `opts.Config.Validate()` and the `cli.StatusForLogger` / `p.Provision()` block — guarantees pre-flight runs before container creation starts.
- Created `create_mount_test.go` with 11 test cases (6 for `validateExtraMounts`, 5 for `logMountPropagationPlatformWarning`); all pass.

## Task Commits

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Add validateExtraMounts and logMountPropagationPlatformWarning to create.go | e0244b26 |
| 2 | Add unit tests for both functions | 6d66c926 |

## Files Created

- `pkg/cluster/internal/create/create_mount_test.go` — unit tests for validateExtraMounts (6 cases) and logMountPropagationPlatformWarning (5 cases)

## Files Modified

- `pkg/cluster/internal/create/create.go` — added `os` and `path/filepath` imports; added `validateExtraMounts` and `logMountPropagationPlatformWarning` functions; wired both calls in `Cluster()` between `Validate()` and `Provision()`

## Decisions Made

1. **validateExtraMounts called between Validate() and Provision()**: Ensures all pre-flight checks (config validity + host path existence) complete before any containers are created, giving a clean rollback-free failure mode.
2. **warn-once propagation warning**: `logMountPropagationPlatformWarning` returns after the first non-None mount detected — consistent with the `logMetalLBPlatformWarning` pattern already in the file. Avoids log spam when multiple mounts have propagation set.
3. **Relative path resolution before stat**: `filepath.Abs` is called when `filepath.IsAbs` returns false, so paths like `.` or `./data` work correctly regardless of caller CWD.
4. **Reuse existing testLogger**: The `testLogger` struct with `lines []string` is already declared in `create_addon_test.go` (same package). The new test file references `l.lines` directly without redeclaring the type — deviation from the plan's suggestion to define a new minimal logger, but correct for the codebase.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] testLogger already declared in create_addon_test.go**
- **Found during:** Task 2
- **Issue:** The plan instructed creating a new `testLogger` struct, but one already existed in `create_addon_test.go` (same package) with `lines []string` field. Redeclaring it caused a compile error.
- **Fix:** Removed the duplicate struct from `create_mount_test.go` and used `l.lines` (matching the existing field name) for all warn-count assertions.
- **Files modified:** `pkg/cluster/internal/create/create_mount_test.go`
- **Commit:** 6d66c926

## Issues Encountered

None beyond the testLogger name collision (auto-fixed above).

## Next Phase Readiness

- Phase 45 Plan 02 can proceed: `validateExtraMounts` and `logMountPropagationPlatformWarning` are committed and tested.
- The integration path (calling `Cluster()` with a missing host path) will surface the new error in end-to-end tests.

## Self-Check: PASSED

Files exist on disk:
- FOUND: pkg/cluster/internal/create/create.go
- FOUND: pkg/cluster/internal/create/create_mount_test.go

Commits exist in git log:
- FOUND: e0244b26 (feat(45-01): add validateExtraMounts and logMountPropagationPlatformWarning)
- FOUND: 6d66c926 (test(45-01): add unit tests for validateExtraMounts and logMountPropagationPlatformWarning)
