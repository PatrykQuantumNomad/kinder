---
phase: 41-network-create-flow-integration-and-website
plan: 01
subsystem: diagnostics
tags: [docker, networking, subnet, cidr, netip, cross-platform]

# Dependency graph
requires:
  - phase: 40-kernel-security-platform-checks
    provides: "allChecks registry with 17 checks and test infrastructure"
provides:
  - "subnetClashCheck detecting Docker/host subnet overlaps"
  - "normalizeAbbreviatedCIDR for macOS route abbreviation expansion"
  - "Cross-platform route parsing (ip route show / netstat -rn)"
  - "allChecks registry updated to 18 checks with Network category"
affects: [41-02-create-flow-integration, 41-03-website-known-issues]

# Tech tracking
tech-stack:
  added: []
  patterns: [injectable-getRoutesFunc-for-testability, netip-Prefix-Overlaps-for-CIDR-comparison]

key-files:
  created:
    - pkg/internal/doctor/subnet.go
    - pkg/internal/doctor/subnet_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/gpu_test.go
    - pkg/internal/doctor/socket_test.go

key-decisions:
  - "getRoutesFunc injectable dependency for testing route parsing without system calls"
  - "Self-referential Docker routes (exact prefix match) skipped to avoid false positives"
  - "IPv4 only -- IPv6 ULA clashes are extremely rare and not worth the complexity"
  - "normalizeAbbreviatedCIDR returns false for 4-octet host routes (no CIDR) to avoid false matches"

patterns-established:
  - "Injectable route function pattern: getRoutesFunc func() []string for OS-specific route parsing testability"

requirements-completed: [PLAT-03]

# Metrics
duration: 3min
completed: 2026-03-06
---

# Phase 41 Plan 01: Subnet Clash Detection Summary

**Docker network subnet clash detection via netip.Prefix.Overlaps with cross-platform route parsing and macOS CIDR abbreviation normalization**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-06T18:04:16Z
- **Completed:** 2026-03-06T18:07:58Z
- **Tasks:** 3 (RED, GREEN, registry)
- **Files modified:** 5

## Accomplishments
- subnetClashCheck detects Docker subnet / host route overlaps on both Linux and macOS
- normalizeAbbreviatedCIDR handles all macOS netstat -rn abbreviated destination formats (1/2/3-octet)
- Self-referential Docker routes (Docker's own route in host table) correctly skipped to prevent false positives
- allChecks registry updated to 18 checks with new Network category

## Task Commits

Each task was committed atomically:

1. **Task 1: RED - Failing tests** - `4b7fa49a` (test)
2. **Task 2: GREEN - Implementation** - `1597098f` (feat)
3. **Task 3: Registry wiring** - `f8152798` (feat)

## Files Created/Modified
- `pkg/internal/doctor/subnet.go` - Subnet clash check with cross-platform route parsing and CIDR normalization
- `pkg/internal/doctor/subnet_test.go` - 19 test cases: 9 for normalizeAbbreviatedCIDR, 3 for metadata, 7 for Run()
- `pkg/internal/doctor/check.go` - Added newSubnetClashCheck() as 18th check under Network category
- `pkg/internal/doctor/gpu_test.go` - Updated allChecks count from 17 to 18 with network-subnet entry
- `pkg/internal/doctor/socket_test.go` - Updated allChecks count from 17 to 18 with network-subnet entry

## Decisions Made
- Used injectable `getRoutesFunc func() []string` instead of injectable execCmd for route testing, providing cleaner test isolation
- Self-referential Docker routes detected by exact `netip.Prefix` equality (same subnet = Docker's own route)
- IPv4 only -- IPv6 ULA subnet clashes are extremely rare in practice
- normalizeAbbreviatedCIDR rejects 4-octet bare IPs (host routes) to avoid false CIDR matches

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated existing allChecks registry count tests**
- **Found during:** Task 3 (registry wiring)
- **Issue:** gpu_test.go and socket_test.go both hardcoded `want 17` for AllChecks() count
- **Fix:** Updated both to `want 18` and added `{"network-subnet", "Network"}` to expected order tables
- **Files modified:** pkg/internal/doctor/gpu_test.go, pkg/internal/doctor/socket_test.go
- **Verification:** `go test ./pkg/internal/doctor/... -count=1` all pass
- **Committed in:** f8152798 (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary update to existing tests reflecting new check count. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Subnet clash check is registered and ready for ApplySafeMitigations integration (41-02)
- All 18 diagnostic checks are now in the allChecks registry
- Network category established for potential future networking checks

## Self-Check: PASSED

All 3 created/modified files verified on disk. All 3 commit hashes verified in git log.

---
*Phase: 41-network-create-flow-integration-and-website*
*Completed: 2026-03-06*
