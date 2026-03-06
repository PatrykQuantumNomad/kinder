---
phase: 39-docker-and-tool-configuration-checks
plan: 01
subsystem: infra
tags: [docker, disk-space, daemon-json, snap, diagnostics, unix-statfs]

# Dependency graph
requires:
  - phase: 38-check-infrastructure-and-interface
    provides: Check interface, allChecks registry, fakeCmd test helpers
provides:
  - diskSpaceCheck with build-tagged statfsFreeBytes (linux/darwin)
  - daemonJSONCheck with multi-candidate path search
  - dockerSnapCheck with symlink resolution
  - allChecks expanded to 8 checks in Runtime/Docker/Tools/GPU order
affects: [39-02-PLAN, create-flow-mitigations]

# Tech tracking
tech-stack:
  added: [golang.org/x/sys/unix (promoted from indirect to direct)]
  patterns: [build-tagged platform abstractions, injectable readFile/readDiskFree deps]

key-files:
  created:
    - pkg/internal/doctor/disk.go
    - pkg/internal/doctor/disk_unix.go
    - pkg/internal/doctor/disk_other.go
    - pkg/internal/doctor/disk_test.go
    - pkg/internal/doctor/daemon.go
    - pkg/internal/doctor/daemon_test.go
    - pkg/internal/doctor/snap.go
    - pkg/internal/doctor/snap_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/gpu_test.go
    - go.mod

key-decisions:
  - "diskSpaceCheck uses build-tagged statfsFreeBytes with int64 cast for macOS/Linux Bsize portability"
  - "daemonJSONCheck searches 6 candidate paths including Windows ProgramData when GOOS is windows"
  - "dockerSnapCheck uses filepath.EvalSymlinks for symlink-transparent snap detection"

patterns-established:
  - "Build-tag pattern: _unix.go (linux||darwin) + _other.go (!linux&&!darwin) for platform-specific syscalls"
  - "Injectable function deps: readDiskFree, readFile, evalSymlinks for testability without mocking packages"

requirements-completed: [DOCK-01, DOCK-02, DOCK-03]

# Metrics
duration: 4min
completed: 2026-03-06
---

# Phase 39 Plan 01: Docker and Tool Configuration Checks Summary

**Disk space, daemon.json init flag, and snap Docker detection checks with build-tagged statfs and injectable deps**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-06T15:37:12Z
- **Completed:** 2026-03-06T15:41:12Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments
- diskSpaceCheck warns at <5GB and fails at <2GB using Docker's data root path from `docker info`
- daemonJSONCheck searches 6+ candidate paths and detects init:true which breaks kind nodes
- dockerSnapCheck resolves symlinks to detect /snap/ installations and suggests TMPDIR workaround
- allChecks registry expanded from 5 to 8 checks in Runtime/Docker/Tools/GPU category order
- golang.org/x/sys promoted from indirect to direct dependency for unix.Statfs

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement disk space check with build-tagged Statfs helper**
   - `8021af21` (test) - Failing tests for disk space check thresholds and fallback behavior
   - `6fc2b57e` (feat) - Disk space check implementation with statfs helpers and go.mod update
2. **Task 2: Implement daemon.json and Docker snap checks, register all three**
   - `06fdeba0` (test) - Failing tests for daemon.json and snap checks
   - `4e12e049` (feat) - daemon.go, snap.go implementation and allChecks registry update

## Files Created/Modified
- `pkg/internal/doctor/disk.go` - diskSpaceCheck with Docker data root detection and threshold logic
- `pkg/internal/doctor/disk_unix.go` - statfsFreeBytes using golang.org/x/sys/unix (linux/darwin build tag)
- `pkg/internal/doctor/disk_other.go` - statfsFreeBytes stub for unsupported platforms
- `pkg/internal/doctor/disk_test.go` - 7 table-driven tests for all threshold and fallback cases
- `pkg/internal/doctor/daemon.go` - daemonJSONCheck with 6+ candidate path search for init:true
- `pkg/internal/doctor/daemon_test.go` - 7 table-driven tests for daemon.json detection scenarios
- `pkg/internal/doctor/snap.go` - dockerSnapCheck with symlink resolution for snap detection
- `pkg/internal/doctor/snap_test.go` - 4 table-driven tests for snap detection and fallback
- `pkg/internal/doctor/check.go` - allChecks expanded to 8 entries with Docker category
- `pkg/internal/doctor/gpu_test.go` - TestAllChecks_RegisteredOrder updated for 8 checks
- `go.mod` - golang.org/x/sys promoted from indirect to direct

## Decisions Made
- Used build-tagged statfsFreeBytes with int64 cast for macOS uint32 vs Linux int64 Bsize portability
- daemonJSONCheck searches 6 candidate paths including Windows ProgramData when GOOS is windows
- dockerSnapCheck uses filepath.EvalSymlinks for transparent snap detection through symlinks
- All three checks use injectable function deps (readDiskFree, readFile, evalSymlinks) matching established pattern

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 3 Docker checks implemented and tested, ready for plan 39-02 (tool version checks)
- allChecks registry ready to accept 2 more checks from plan 39-02 (will go from 8 to 10)
- Full test suite passes with no regressions

## Self-Check: PASSED

All 9 key files verified on disk. All 4 commit hashes verified in git log.

---
*Phase: 39-docker-and-tool-configuration-checks*
*Completed: 2026-03-06*
