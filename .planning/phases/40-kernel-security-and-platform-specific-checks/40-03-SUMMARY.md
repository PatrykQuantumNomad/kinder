---
phase: 40-kernel-security-and-platform-specific-checks
plan: 03
subsystem: diagnostics
tags: [wsl2, cgroup, btrfs, rootfs, docker, registry, platform-detection]

# Dependency graph
requires:
  - phase: 40-01
    provides: inotifyCheck and kernelVersionCheck constructors
  - phase: 40-02
    provides: apparmorCheck, selinuxCheck, and firewalldCheck constructors
provides:
  - WSL2 multi-signal detection with cgroup v2 controller verification
  - Rootfs device check detecting BTRFS storage driver and backing filesystem
  - Complete allChecks registry with 17 checks across 7 categories
affects: [phase-41-create-flow-integration, phase-41-website-known-issues]

# Tech tracking
tech-stack:
  added: []
  patterns: [multi-signal platform detection, injectable os.Stat dependency]

key-files:
  created:
    - pkg/internal/doctor/wsl2.go
    - pkg/internal/doctor/wsl2_test.go
    - pkg/internal/doctor/rootfs.go
    - pkg/internal/doctor/rootfs_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/gpu_test.go
    - pkg/internal/doctor/socket_test.go

key-decisions:
  - "WSL2 detection requires two signals to prevent Azure VM false positives"
  - "Cgroup v2 check verifies cpu, memory, pids controllers when WSL2 confirmed"
  - "Rootfs check queries both Docker storage driver and DriverStatus for BTRFS"
  - "allChecks registry ordered: Runtime, Docker, Tools, GPU, Kernel, Security, Platform"

patterns-established:
  - "Multi-signal detection: require corroborating evidence before platform-specific checks"
  - "Injectable os.Stat for file-existence checks in struct deps"

requirements-completed: [PLAT-02, PLAT-04]

# Metrics
duration: 3min
completed: 2026-03-06
---

# Phase 40 Plan 03: WSL2 Rootfs and Registry Wiring Summary

**WSL2 multi-signal detection with cgroup v2 verification, BTRFS rootfs detection, and allChecks registry expanded from 10 to 17 checks**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-06T17:26:01Z
- **Completed:** 2026-03-06T17:29:30Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- WSL2 detection using multi-signal approach (microsoft in /proc/version + WSL_DISTRO_NAME or WSLInterop) that avoids Azure VM false positives
- Cgroup v2 controller verification (cpu, memory, pids) when WSL2 is confirmed, with fix link to spurin/wsl-cgroupsv2
- Rootfs device check detecting BTRFS as Docker storage driver or backing filesystem
- allChecks registry wired with all 7 Phase 40 checks (inotify, kernel, apparmor, selinux, firewalld, wsl2, rootfs)

## Task Commits

Each task was committed atomically:

1. **Task 1: WSL2 multi-signal detection and cgroup v2 check**
   - `92af1777` test(40-03): add failing tests for WSL2 multi-signal detection and cgroup v2 check
   - `df6e3d61` feat(40-03): implement WSL2 multi-signal detection and cgroup v2 check

2. **Task 2: Rootfs device check and allChecks registry wiring**
   - `a2189738` test(40-03): add failing tests for rootfs device BTRFS detection
   - `a9e25bfd` feat(40-03): implement rootfs device check and wire all 7 Phase 40 checks into registry

_Note: TDD tasks have RED (test) and GREEN (feat) commits._

## Files Created/Modified
- `pkg/internal/doctor/wsl2.go` - WSL2 multi-signal detection and cgroup v2 controller check
- `pkg/internal/doctor/wsl2_test.go` - 10 table-driven tests for WSL2 detection scenarios
- `pkg/internal/doctor/rootfs.go` - BTRFS storage driver and backing filesystem detection
- `pkg/internal/doctor/rootfs_test.go` - 5 table-driven tests for rootfs device check
- `pkg/internal/doctor/check.go` - allChecks registry expanded from 10 to 17 entries
- `pkg/internal/doctor/gpu_test.go` - TestAllChecks_RegisteredOrder updated for 17 checks
- `pkg/internal/doctor/socket_test.go` - TestAllChecks_Registry updated for 17 checks

## Decisions Made
- WSL2 detection requires two signals (microsoft in /proc/version PLUS WSL_DISTRO_NAME or WSLInterop) to prevent Azure VM false positives
- Cgroup v2 controller check verifies cpu, memory, pids as required controllers for kind nodes
- WSLInterop-late file supported for WSL 4.1.4+ compatibility
- Rootfs check queries both Docker .Driver and .DriverStatus for comprehensive BTRFS detection
- allChecks registry ordered by category: Runtime(1), Docker(4), Tools(2), GPU(3), Kernel(2), Security(2), Platform(3)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated TestAllChecks_Registry in socket_test.go**
- **Found during:** Task 2 (registry wiring)
- **Issue:** socket_test.go contained a duplicate registry order test (TestAllChecks_Registry) that also asserted 10 checks, causing test failure
- **Fix:** Updated the test to expect 17 checks with all 7 new entries in correct order
- **Files modified:** pkg/internal/doctor/socket_test.go
- **Verification:** Full test suite passes
- **Committed in:** a9e25bfd (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to keep test suite green. No scope creep.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 40 complete: all 7 kernel, security, and platform checks implemented and registered
- allChecks registry has 17 checks across 7 categories, ready for Phase 41 create-flow integration
- Phase 41 can wire ApplySafeMitigations into create flow and build website Known Issues page

## Self-Check: PASSED

All 7 files verified present. All 4 commits verified in git log.

---
*Phase: 40-kernel-security-and-platform-specific-checks*
*Completed: 2026-03-06*
