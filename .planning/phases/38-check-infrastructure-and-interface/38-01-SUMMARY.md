---
phase: 38-check-infrastructure-and-interface
plan: 01
subsystem: infra
tags: [go-interface, doctor, diagnostics, json, unicode-output, mitigation]

# Dependency graph
requires:
  - phase: 37-nvidia-gpu-addon
    provides: existing doctor checks (container-runtime, kubectl, NVIDIA GPU) to migrate in Plan 02
provides:
  - Check interface with Name(), Category(), Platforms(), Run() methods
  - Result type with 6 JSON-tagged fields
  - AllChecks() explicit registry and RunAllChecks() with platform filtering
  - ExitCodeFromResults() exit code computation
  - FormatHumanReadable() category-grouped Unicode output formatter
  - FormatJSON() envelope formatter with checks array and summary object
  - SafeMitigation struct with NeedsFix/Apply/NeedsRoot fields
  - ApplySafeMitigations() entry point (skeleton)
affects: [38-02 check migration, 39-docker-checks, 40-kernel-checks, 41-create-flow-integration]

# Tech tracking
tech-stack:
  added: []
  patterns: [Check interface with 4 methods, Result struct with 6 JSON-tagged fields, explicit registry slice, centralized platform filtering, first-seen category grouping]

key-files:
  created:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/check_test.go
    - pkg/internal/doctor/format.go
    - pkg/internal/doctor/format_test.go
    - pkg/internal/doctor/mitigations.go
    - pkg/internal/doctor/mitigations_test.go
  modified: []

key-decisions:
  - "Empty allChecks registry initialized as []Check{} (not nil) for non-nil guarantee"
  - "FormatJSON returns map[string]interface{} for flexible JSON serialization"
  - "ApplySafeMitigations early-returns on non-Linux platforms"

patterns-established:
  - "Check interface: Name()/Category()/Platforms()/Run() - all future checks implement this"
  - "Result struct: 6 fields with JSON tags - canonical output format for all checks"
  - "Platform filtering: centralized in RunAllChecks(), checks never check runtime.GOOS themselves"
  - "Category grouping: first-seen order from registry, not alphabetical"
  - "Exit codes: 0 for ok/skip, 1 for fail, 2 for warn-only"

# Metrics
duration: 3min
completed: 2026-03-06
---

# Phase 38 Plan 01: Check Infrastructure Summary

**Check interface, Result type, registry with platform filtering, Unicode output formatter, JSON envelope formatter, and SafeMitigation skeleton in pkg/internal/doctor/**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-06T14:20:51Z
- **Completed:** 2026-03-06T14:23:52Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Check interface defined with 4 methods (Name, Category, Platforms, Run) providing the contract for all future diagnostic checks
- Result type with 6 JSON-tagged fields matching the user-specified output format
- AllChecks() registry and RunAllChecks() with centralized platform filtering that emits skip results for non-matching platforms
- FormatHumanReadable() producing category-grouped output with Unicode icons (checkmark, x, warning, null), 2-space indent, separator line, and summary counts
- FormatJSON() producing envelope with checks array and summary object with total/ok/warn/fail/skip counts
- SafeMitigation struct with NeedsFix/Apply/NeedsRoot fields and ApplySafeMitigations() skeleton
- ExitCodeFromResults: 0 for ok/skip, 1 for fail, 2 for warn-only
- 15 unit tests all passing with t.Parallel()

## Task Commits

Each task was committed atomically:

1. **Task 1: Check interface, Result type, registry, platform filtering** - `4c2d9887` (feat)
2. **Task 2: Output formatters and SafeMitigation skeleton** - `e88308a2` (feat)

_Note: TDD tasks -- tests written first (RED), then implementation (GREEN), committed together._

## Files Created/Modified
- `pkg/internal/doctor/check.go` - Check interface, Result type, AllChecks() registry, RunAllChecks() with platform filter, ExitCodeFromResults()
- `pkg/internal/doctor/check_test.go` - Tests for registry, platform filtering, exit codes, platformSkipMessage
- `pkg/internal/doctor/format.go` - FormatHumanReadable() and FormatJSON() output formatters
- `pkg/internal/doctor/format_test.go` - Tests for human-readable output, category ordering, JSON envelope
- `pkg/internal/doctor/mitigations.go` - SafeMitigation struct, SafeMitigations(), ApplySafeMitigations()
- `pkg/internal/doctor/mitigations_test.go` - Tests for mitigation skeleton

## Decisions Made
- Initialized allChecks as `[]Check{}` (not nil) to guarantee AllChecks() always returns non-nil slice
- FormatJSON returns `map[string]interface{}` rather than a typed struct for flexibility in JSON serialization
- ApplySafeMitigations() early-returns nil on non-Linux platforms since all mitigations target Linux

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- pkg/internal/doctor/ package ready for Plan 02 to migrate 3 existing checks (container-runtime, kubectl, NVIDIA GPU) and refactor doctor.go CLI layer
- Check interface and Result type provide the contract for all Phases 38-41 check implementations
- FormatHumanReadable and FormatJSON ready to consume results from migrated checks

## Self-Check: PASSED

- All 6 created files verified on disk
- Commit 4c2d9887 verified in git log
- Commit e88308a2 verified in git log
- 15/15 tests passing
- go vet clean
- go build ./... clean

---
*Phase: 38-check-infrastructure-and-interface*
*Completed: 2026-03-06*
