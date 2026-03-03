---
phase: 19-bug-fixes
plan: "02"
subsystem: cluster
tags: [sort, provider, bug-fix, strict-weak-ordering, defaultName]

# Dependency graph
requires: []
provides:
  - ListInternalNodes resolves empty name to default cluster name via defaultName()
  - sortNetworkInspectEntries uses strict weak ordering (iLen != jLen guard)
  - Unit tests proving both fixes
affects: [phase-20-provider-deduplication]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "All Provider methods that accept a cluster name must wrap with defaultName() before passing to internal provider"
    - "sort.Slice comparators must use != guard (not >-only) when using fallback secondary key to maintain strict weak ordering"

key-files:
  created:
    - pkg/cluster/provider_test.go
  modified:
    - pkg/cluster/provider.go
    - pkg/cluster/internal/providers/docker/network.go
    - pkg/cluster/internal/providers/docker/network_test.go

key-decisions:
  - "BUG-03 fix is one-line: wrap defaultName(name) in ListInternalNodes; matches every other method on Provider"
  - "BUG-04 fix uses iLen != jLen guard rather than separate else branch — clearer intent, minimal diff"
  - "TDD RED for BUG-04 required starting slice with the higher-ID network first to expose the violation; 2-element sort with fewer-containers-first input coincidentally produced correct output with buggy code"

patterns-established:
  - "sort.Slice with primary/secondary key: guard primary key with != comparison, return secondary only in else branch"

# Metrics
duration: 4min
completed: 2026-03-03
---

# Phase 19 Plan 02: Bug Fixes (BUG-03, BUG-04) Summary

**ListInternalNodes now resolves empty cluster name via defaultName() and sortNetworkInspectEntries uses iLen != jLen strict weak ordering guard with ID tiebreaker**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-03T12:42:58Z
- **Completed:** 2026-03-03T12:47:00Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- BUG-03: ListInternalNodes no longer returns empty results when called with an empty cluster name; it now calls `p.provider.ListNodes(defaultName(name))` matching all other Provider methods
- BUG-04: sortNetworkInspectEntries comparator now satisfies strict weak ordering — the `iLen != jLen` guard prevents both `less(i,j)` and `less(j,i)` from returning true simultaneously, ending undefined sort behavior
- Both fixes covered by table-driven TDD unit tests; the BUG-04 determinism test runs 100 repeated sorts to confirm stability

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix BUG-03 — ListInternalNodes missing defaultName() call** - `5a321168` (fix)
2. **Task 2: Fix BUG-04 — network sort comparator strict weak ordering** - `499beb9f` (fix)

**Plan metadata:** (docs commit — see below)

_Note: TDD tasks include RED (test written) and GREEN (fix applied) in same commit per plan instruction_

## Files Created/Modified

- `pkg/cluster/provider.go` - Added `defaultName(name)` wrap in `ListInternalNodes` (line 231)
- `pkg/cluster/provider_test.go` - New file: `TestListInternalNodes_DefaultName` with mock provider and table-driven cases for empty/non-empty name
- `pkg/cluster/internal/providers/docker/network.go` - Replaced sort comparator with `iLen != jLen` guard pattern
- `pkg/cluster/internal/providers/docker/network_test.go` - Added "more containers wins over lower ID" case and `Test_sortNetworkInspectEntries_Deterministic` (100 runs)

## Decisions Made

- BUG-03 fix is a single-character change (wrap with `defaultName()`); no refactoring needed
- BUG-04 TDD required careful input ordering: the bug only manifests when the network with MORE containers has a HIGHER ID and appears FIRST in the input slice — the shorter-ID-first input coincidentally produces correct output with the buggy comparator
- Used `reflect.DeepEqual` directly in the determinism test (not assert helper) to match the `sort.Slice` repeated-run pattern without needing the assert package dependency on the new test

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

During RED phase for BUG-04, the initial test input had the lower-ID network (`aaaa`, 0 containers) first and the higher-ID network (`zzzz`, 1 container) second. Go's pdqsort happens to produce the correct result in that configuration even with the buggy comparator. The test was corrected to place the higher-ID network first, which exposes the strict weak ordering violation and causes the expected test failure.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- BUG-03 and BUG-04 are fully resolved; both have regression-proof unit tests
- Phase 19 is complete (plan 01 fixed BUG-01 and BUG-02, plan 02 fixed BUG-03 and BUG-04)
- Ready for Phase 20 (Provider Deduplication) — see STATE.md blocker regarding ProviderBehavior interface design

---
*Phase: 19-bug-fixes*
*Completed: 2026-03-03*

## Self-Check: PASSED

- FOUND: pkg/cluster/provider.go
- FOUND: pkg/cluster/provider_test.go
- FOUND: pkg/cluster/internal/providers/docker/network.go
- FOUND: pkg/cluster/internal/providers/docker/network_test.go
- FOUND: .planning/phases/19-bug-fixes/19-02-SUMMARY.md
- FOUND commit 5a321168: fix(19-02): add defaultName() call in ListInternalNodes
- FOUND commit 499beb9f: fix(19-02): use strict weak ordering in network sort comparator
- VERIFIED: provider.go line 231 has `p.provider.ListNodes(defaultName(name))`
- VERIFIED: network.go has `iLen != jLen` guard at line 212
