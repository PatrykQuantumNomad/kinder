---
phase: 40-kernel-security-and-platform-specific-checks
plan: 01
subsystem: diagnostics
tags: [inotify, kernel, sysctl, uname, proc, build-tags, cgroup]

# Dependency graph
requires:
  - phase: 38-doctor-framework-and-runtime-checks
    provides: Check interface, Result struct, allChecks registry pattern
provides:
  - inotifyCheck for /proc/sys/fs/inotify/ limit detection
  - kernelVersionCheck for cgroup namespace kernel version requirement
  - parseKernelVersion helper for Linux release string parsing
  - fakeReadFile test helper for injectable file reading
affects: [40-03-check-registration, 41-create-flow-integration]

# Tech tracking
tech-stack:
  added: []
  patterns: [readFile dependency injection for proc files, build-tagged linux/other split, unix.Uname injection]

key-files:
  created:
    - pkg/internal/doctor/inotify.go
    - pkg/internal/doctor/inotify_test.go
    - pkg/internal/doctor/kernel_linux.go
    - pkg/internal/doctor/kernel_other.go
    - pkg/internal/doctor/kernel_linux_test.go
  modified: []

key-decisions:
  - "inotifyCheck returns multiple results when both limits are low, single ok when both sufficient"
  - "kernelVersionCheck uses status fail (not warn) for < 4.6 since cgroup namespace is a hard blocker"
  - "kernel_other.go stub returns nil from Run() matching platform filtering pattern"
  - "fakeReadFile test helper defined in inotify_test.go, reusable across doctor package tests"

patterns-established:
  - "readFile injection: struct field func(string)([]byte,error) with os.ReadFile default for /proc/ reads"
  - "build-tagged pair: kernel_linux.go + kernel_other.go matching disk_unix.go + disk_other.go pattern"
  - "uname injection: struct field func(*unix.Utsname)error with unix.Uname default"

requirements-completed: [KERN-01, KERN-04]

# Metrics
duration: 4min
completed: 2026-03-06
---

# Phase 40 Plan 01: Inotify Limits and Kernel Version Checks Summary

**inotify limits check warns on low max_user_watches/instances with sysctl fix commands; kernel version check fails on < 4.6 for cgroup namespace support**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-06T17:16:59Z
- **Completed:** 2026-03-06T17:21:07Z
- **Tasks:** 2
- **Files created:** 5

## Accomplishments
- inotifyCheck reads /proc/sys/fs/inotify/ limits and warns when watches < 524288 or instances < 512 with exact sysctl fix commands
- kernelVersionCheck parses kernel release strings (handles -generic, -azure, -WSL2 suffixes) and fails when kernel < 4.6
- kernel_other.go stub compiles on macOS/Windows without unix import, matching established build-tag pattern
- fakeReadFile test helper enables dependency-injected /proc file testing across the doctor package

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement inotify limits check with tests**
   - `c799641b` test(40-01): add failing tests for inotify limits check
   - `2f181806` feat(40-01): implement inotify limits check
2. **Task 2: Implement kernel version check with build tags and tests**
   - `4d50cb4d` test(40-01): add failing tests for kernel version check
   - `49b3e873` feat(40-01): implement kernel version check with build tags

_TDD tasks have RED (test) and GREEN (feat) commits._

## Files Created/Modified
- `pkg/internal/doctor/inotify.go` - inotifyCheck struct with readFile dep, warns on low inotify limits
- `pkg/internal/doctor/inotify_test.go` - Table-driven tests for inotify thresholds and fakeReadFile helper
- `pkg/internal/doctor/kernel_linux.go` - kernelVersionCheck with unix.Uname dep, parseKernelVersion helper
- `pkg/internal/doctor/kernel_other.go` - Stub kernelVersionCheck for non-Linux compilation
- `pkg/internal/doctor/kernel_linux_test.go` - Tests for kernel version parsing and threshold comparison (linux build-tagged)

## Decisions Made
- inotifyCheck returns multiple results when both limits are low (one per problematic value with specific fix), single ok when both sufficient
- kernelVersionCheck uses status "fail" (not "warn") for kernels below 4.6, since cgroup namespace support is a hard blocker for kind
- kernel_other.go stub returns nil from Run() -- never called due to Platforms() filtering, but must compile
- fakeReadFile test helper defined in inotify_test.go and reusable by other tests in the doctor package (like firewalld_test.go)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Both checks compile and pass tests but are NOT yet registered in allChecks (Plan 03 handles registration)
- kernel_linux_test.go is linux build-tagged so tests only run on Linux CI; macOS verification uses go build ./...
- fakeReadFile helper is available for use by other check tests (already used by firewalld_test.go from 40-02)

## Self-Check: PASSED

All 5 created files verified present. All 4 commit hashes verified in git log.

---
*Phase: 40-kernel-security-and-platform-specific-checks*
*Completed: 2026-03-06*
