---
phase: 08-gap-closure
plan: "01"
subsystem: config-defaulting
tags: [bug-fix, addons, defaulting, tdd, gap-closure]
requires: []
provides: [FOUND-04]
affects: [pkg/internal/apis/config/default.go, pkg/cluster/internal/create/create.go, pkg/internal/apis/config/default_test.go]
tech-stack:
  added: []
  patterns: [tdd-red-green, config-defaulting]
key-files:
  created:
    - pkg/internal/apis/config/default_test.go
  modified:
    - pkg/internal/apis/config/default.go
    - pkg/cluster/internal/create/create.go
key-decisions:
  - "All-false addon guard removed from SetDefaultsCluster — encoding.Load('') path is the single correct defaulting point for library usage"
  - "Redundant SetDefaultsCluster call removed from fixupOptions in create.go — opts.Config is already defaulted when encoding.Load('') runs for nil config"
  - "load.go line 40 SetDefaultsCluster call preserved — this is the correct single point where empty-path defaults apply"
duration: 1 min
completed: 2026-03-01T20:45:10Z
---

# Phase 8 Plan 01: All-False Addon Guard Fix Summary

**One-liner:** Removed the all-false addon guard from `SetDefaultsCluster` and redundant `SetDefaultsCluster` call from `fixupOptions`, ensuring explicit user opt-out of all five addons is respected, verified by TDD unit tests.

## Performance

- Duration: ~1 minute
- Started: 2026-03-01T20:44:03Z
- Completed: 2026-03-01T20:45:10Z
- Tasks completed: 1 of 1
- Files created: 1
- Files modified: 2

## Accomplishments

- Removed 9-line all-false addon guard block from `SetDefaultsCluster` in `default.go` that silently overrode explicit user config when all five addons were set to false
- Removed redundant `config.SetDefaultsCluster(opts.Config)` call and its comment from `fixupOptions` in `create.go` — the call was unnecessary because `encoding.Load("")` already calls `SetDefaultsCluster` for nil configs
- Created `default_test.go` with two test functions proving the fix: `TestSetDefaultsCluster_AddonFields` (all-false and zero-value preserved) and `TestSetDefaultsCluster_NonAddonDefaults` (Name, Nodes, IPFamily still default correctly)
- Full test suite (`go test ./...`) passes with zero failures after changes

## Task Commits

| Task | Phase | Description | Commit |
|------|-------|-------------|--------|
| 1 | RED | Add failing tests for all-false addon guard | 6a5d8183 |
| 1 | GREEN | Remove all-false addon guard and redundant SetDefaultsCluster call | 1574486a |

## Files Created

| File | Description |
|------|-------------|
| pkg/internal/apis/config/default_test.go | Unit tests for SetDefaultsCluster addon and non-addon defaulting behavior |

## Files Modified

| File | Change |
|------|--------|
| pkg/internal/apis/config/default.go | Removed all-false addon guard (9 lines) from SetDefaultsCluster |
| pkg/cluster/internal/create/create.go | Removed redundant config.SetDefaultsCluster(opts.Config) call (3 lines) from fixupOptions |

## Decisions Made

1. **Single defaulting point is load.go:40** — `encoding.Load("")` calls `SetDefaultsCluster` once for the nil-config case. `fixupOptions` in `create.go` called it again after already receiving a defaulted config, making the second call both redundant and harmful (it could silently override user intent). Removing the redundant call preserves the correct flow.

2. **No replacement guard needed** — The plan initially suggested the guard was a "safety net for library usage." However, the correct approach for library usage is to call `encoding.Load("")` or `SetDefaultsCluster` explicitly once. Silently overriding user intent is wrong behavior; the guard was a workaround masking the real issue (the redundant call in `fixupOptions`).

3. **load.go line 40 preserved** — The `config.SetDefaultsCluster(out)` call inside `encoding.Load("")` for the empty-path case is correct and must remain. It handles the case where callers use `encoding.Load("")` to get a default config without addons being overridden.

## Deviations from Plan

None — plan executed exactly as written. TDD RED-GREEN cycle followed. No architectural changes required.

## Issues Encountered

None. The two deletions were straightforward once the existing import of `config` in `create.go` was verified to still be needed (it is, for `config.Cluster` type reference on line 59).

## Verification Results

- `go test ./pkg/internal/apis/config/... -run TestSetDefaultsCluster -v` — PASS (both test functions, all subtests)
- `go test ./...` — PASS (zero failures across all packages)
- `grep -n "Default all addons" pkg/internal/apis/config/default.go` — NOT FOUND (guard removed)
- `grep -n "SetDefaultsCluster" pkg/cluster/internal/create/create.go` — NOT FOUND (redundant call removed)
- `grep -n "SetDefaultsCluster" pkg/internal/apis/config/encoding/load.go` — line 40 still present (preserved)

## Self-Check

- [x] `pkg/internal/apis/config/default_test.go` created — VERIFIED
- [x] `pkg/internal/apis/config/default.go` all-false guard removed — VERIFIED
- [x] `pkg/cluster/internal/create/create.go` redundant call removed — VERIFIED
- [x] `pkg/internal/apis/config/encoding/load.go` line 40 preserved — VERIFIED
- [x] Commit 6a5d8183 (RED) exists — VERIFIED
- [x] Commit 1574486a (GREEN) exists — VERIFIED
- [x] `go test ./...` passes — VERIFIED

## Self-Check: PASSED

## Next Phase Readiness

This is Plan 1 of 2 in Phase 8 (Gap Closure). Plan 2 (kubectl context targeting in integration script) was already completed (commit 0382e8d7). Both gap closure items satisfying FOUND-04 are now complete. The v1 milestone is fully reached with all 8 phases complete.
