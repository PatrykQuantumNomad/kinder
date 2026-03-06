---
phase: 38-check-infrastructure-and-interface
plan: 02
subsystem: infra
tags: [go-interface, doctor, diagnostics, check-migration, deps-struct, tdd]

# Dependency graph
requires:
  - phase: 38-check-infrastructure-and-interface
    provides: Check interface, Result type, AllChecks() registry, FormatHumanReadable(), FormatJSON(), ExitCodeFromResults(), SafeMitigation skeleton
provides:
  - containerRuntimeCheck with deps struct detecting docker/podman/nerdctl
  - kubectlCheck with deps struct detecting kubectl availability
  - nvidiaDriverCheck, nvidiaContainerToolkitCheck, nvidiaDockerRuntimeCheck with deps struct
  - AllChecks() populated with 5 checks in Runtime/Tools/GPU category order
  - doctor.go CLI refactored to delegate entirely to pkg/internal/doctor/
affects: [39-docker-checks, 40-kernel-checks, 41-create-flow-integration]

# Tech tracking
tech-stack:
  added: []
  patterns: [deps struct injection for check testability, inline fakeCmd test doubles, table-driven check tests with t.Parallel()]

key-files:
  created:
    - pkg/internal/doctor/runtime.go
    - pkg/internal/doctor/runtime_test.go
    - pkg/internal/doctor/tools.go
    - pkg/internal/doctor/tools_test.go
    - pkg/internal/doctor/gpu.go
    - pkg/internal/doctor/gpu_test.go
    - pkg/internal/doctor/testhelpers_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/cmd/kind/doctor/doctor.go

key-decisions:
  - "Inline fakeCmd test doubles in testhelpers_test.go instead of importing testutil from pkg/cluster/internal/"
  - "nvidiaDockerRuntimeCheck returns skip when docker binary not found (not fail/warn)"
  - "containerRuntimeCheck falls back from 'version' to '-v' subcommand matching original checkBinary() behavior"

patterns-established:
  - "Deps struct: each check has lookPath + execCmd fields for injectable testing"
  - "fakeExecResult map: tests map command strings to canned output/error pairs"
  - "All check tests use t.Parallel() with table-driven subtests"

requirements-completed: [INFRA-06, INFRA-02, INFRA-03]

# Metrics
duration: 3min
completed: 2026-03-06
---

# Phase 38 Plan 02: Check Migration Summary

**5 existing doctor checks migrated to Check interface with deps struct injection and doctor.go refactored from 298 to 74 lines**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-06T14:28:48Z
- **Completed:** 2026-03-06T14:32:32Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- All 3 existing check groups (container runtime, kubectl, NVIDIA GPU) migrated to Check interface as 5 individual checks
- AllChecks() registry populated with checks in Runtime/Tools/GPU category order
- doctor.go CLI layer reduced from 298 lines to 74 lines, delegating to doctor.RunAllChecks(), FormatHumanReadable(), FormatJSON(), ExitCodeFromResults()
- nvidiaDockerRuntimeCheck skips gracefully when Docker binary not found (returns skip instead of confusing warn)
- 20+ new tests with inline fakeCmd doubles and table-driven parallel execution
- Full project compiles, go vet clean, all tests pass (no regressions)

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate 3 existing checks to Check interface with deps struct** - `208de9f5` (feat) [TDD]
2. **Task 2: Refactor doctor.go CLI to delegate to pkg/internal/doctor/** - `7f2f92d0` (refactor)

_Note: TDD task -- tests written first (RED), then implementation (GREEN), committed together._

## Files Created/Modified
- `pkg/internal/doctor/runtime.go` - containerRuntimeCheck detecting docker/podman/nerdctl with deps struct
- `pkg/internal/doctor/runtime_test.go` - 5 test scenarios for container runtime detection
- `pkg/internal/doctor/tools.go` - kubectlCheck verifying kubectl availability with deps struct
- `pkg/internal/doctor/tools_test.go` - 3 test scenarios for kubectl detection
- `pkg/internal/doctor/gpu.go` - nvidiaDriverCheck, nvidiaContainerToolkitCheck, nvidiaDockerRuntimeCheck with deps struct
- `pkg/internal/doctor/gpu_test.go` - 10 test scenarios for NVIDIA GPU checks plus registry order test
- `pkg/internal/doctor/testhelpers_test.go` - fakeCmd implementing exec.Cmd, fakeExecResult map helper
- `pkg/internal/doctor/check.go` - AllChecks() populated with 5 checks
- `pkg/cmd/kind/doctor/doctor.go` - Refactored CLI delegating to pkg/internal/doctor/

## Decisions Made
- Created inline fakeCmd test doubles in testhelpers_test.go rather than importing testutil from pkg/cluster/internal/ to avoid creating cross-package test dependency
- nvidiaDockerRuntimeCheck returns skip status (not warn) when docker binary is not found, following research recommendation to avoid confusing non-Docker users
- containerRuntimeCheck preserves the existing checkBinary() fallback from "version" to "-v" subcommand for runtime detection

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 38 complete: Check interface, registry, formatters, mitigations skeleton, and all existing checks migrated
- doctor.go is a thin CLI layer -- adding new checks in Phases 39-40 requires only creating new check files in pkg/internal/doctor/ and adding to AllChecks()
- deps struct pattern established for all future checks to follow
- Ready for Phase 39 (Docker and Tool Configuration Checks)

## Self-Check: PASSED

- All 9 created/modified files verified on disk
- Commit 208de9f5 verified in git log
- Commit 7f2f92d0 verified in git log
- All tests passing (go test ./pkg/internal/doctor/... clean)
- go build ./... clean
- go vet ./... clean

---
*Phase: 38-check-infrastructure-and-interface*
*Completed: 2026-03-06*
