---
phase: 40-kernel-security-and-platform-specific-checks
plan: 02
subsystem: diagnostics
tags: [apparmor, selinux, firewalld, security, linux, doctor]

# Dependency graph
requires:
  - phase: 38-check-infrastructure-and-interface
    provides: "Check interface, Result type, fakeReadFile/fakeExecCmd test helpers"
provides:
  - "apparmorCheck detecting AppArmor enabled state with moby/moby#7512 warning"
  - "selinuxCheck detecting SELinux enforcing on Fedora with /dev/dma_heap warning"
  - "firewalldCheck detecting nftables backend with iptables fix suggestion"
affects: [40-03-wsl2-rootfs-registry-wiring, 41-network-integration-website]

# Tech tracking
tech-stack:
  added: []
  patterns: [readFile-only dependency injection for security checks, multi-file readFile for SELinux distro detection]

key-files:
  created:
    - pkg/internal/doctor/apparmor.go
    - pkg/internal/doctor/apparmor_test.go
    - pkg/internal/doctor/selinux.go
    - pkg/internal/doctor/selinux_test.go
    - pkg/internal/doctor/firewalld.go
    - pkg/internal/doctor/firewalld_test.go
  modified: []

key-decisions:
  - "AppArmor and SELinux checks are completely independent (not mutually exclusive) per LSM stacking since kernel 5.1"
  - "SELinux warns only on Fedora (ID=fedora in os-release) since /dev/dma_heap denials are Fedora-specific"
  - "Firewalld defaults to nftables when FirewallBackend line absent (Fedora 32+ default)"
  - "isFedora returns false on os-release read error (err on safe side, don't false-positive)"

patterns-established:
  - "readFile-only checks: apparmorCheck and selinuxCheck use only readFile dep (no execCmd needed)"
  - "Multi-dep checks: firewalldCheck combines readFile + lookPath + execCmd for three-step detection"
  - "Default-value parsing: when config line absent, use platform-documented default (nftables since Fedora 32)"

requirements-completed: [KERN-02, KERN-03, PLAT-01]

# Metrics
duration: 3min
completed: 2026-03-06
---

# Phase 40 Plan 02: AppArmor, SELinux, and Firewalld Checks Summary

**Three security and platform checks detecting AppArmor profile conflicts, SELinux enforcing mode on Fedora, and firewalld nftables backend breaking Docker networking**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-06T17:18:01Z
- **Completed:** 2026-03-06T17:21:00Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- AppArmor check detects enabled state and warns about moby/moby#7512 stale profile issue with aa-remove-unknown fix
- SELinux check detects enforcing mode and warns only on Fedora about /dev/dma_heap denials with setenforce fix
- Firewalld check detects nftables backend through three-step detection (installed -> running -> config parse) with iptables fix
- All three checks are fully independent and use consistent readFile dependency injection pattern
- 14 table-driven test cases covering all behavior combinations

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement AppArmor and SELinux checks with tests**
   - `3e91e57b` (test) - failing tests for AppArmor and SELinux
   - `8df241d5` (feat) - AppArmor and SELinux implementations
2. **Task 2: Implement firewalld nftables backend check with tests**
   - `fa351170` (test) - failing tests for firewalld backend
   - `ca1739c8` (feat) - firewalld implementation

_Note: TDD tasks have two commits each (test -> feat). No refactor needed._

## Files Created/Modified
- `pkg/internal/doctor/apparmor.go` - AppArmor enabled detection via /sys/module/apparmor/parameters/enabled
- `pkg/internal/doctor/apparmor_test.go` - 3 test cases: enabled (warn), disabled (ok), not-loaded (ok)
- `pkg/internal/doctor/selinux.go` - SELinux enforcing detection with isFedora() helper
- `pkg/internal/doctor/selinux_test.go` - 5 test cases: enforcing+Fedora (warn), enforcing+Ubuntu (ok), permissive (ok), not-available (ok), unreadable os-release (ok)
- `pkg/internal/doctor/firewalld.go` - Firewalld nftables backend detection via lookPath + firewall-cmd --state + config parse
- `pkg/internal/doctor/firewalld_test.go` - 6 test cases: not-installed, not-running, nftables (warn), iptables (ok), unreadable config (ok), missing line defaults nftables (warn)

## Decisions Made
- AppArmor and SELinux checks are completely independent (both can run, no if/else) per LSM stacking since kernel 5.1
- SELinux warns only on Fedora (checks ID=fedora in /etc/os-release case-insensitively); returns ok on non-Fedora enforcing
- isFedora() returns false on os-release read error to avoid false-positive warnings
- Firewalld defaults to nftables when FirewallBackend config line is absent (Fedora 32+ default behavior)
- AppArmor check does NOT run aa-status (requires root); only reads sysfs enabled parameter

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- All three checks implemented and tested, ready for allChecks registry wiring in Plan 40-03
- No checks registered in allChecks yet (Plan 03 handles registration)
- Checks follow established patterns: readFile dep injection, table-driven tests, fakeReadFile/fakeExecCmd helpers

## Self-Check: PASSED

All 7 files verified present. All 4 commit hashes verified in git log.

---
*Phase: 40-kernel-security-and-platform-specific-checks*
*Completed: 2026-03-06*
