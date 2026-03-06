---
phase: 39-docker-and-tool-configuration-checks
plan: 02
subsystem: infra
tags: [kubectl, version-skew, docker-socket, permissions, diagnostics, semver]

# Dependency graph
requires:
  - phase: 39-docker-and-tool-configuration-checks
    plan: 01
    provides: allChecks registry with 8 checks, fakeCmd test helpers, Check interface
provides:
  - kubectlVersionSkewCheck with semver parsing and +/-1 minor tolerance
  - dockerSocketCheck with case-insensitive permission denied detection
  - allChecks registry complete at 10 checks in Runtime/Docker/Tools/GPU order
affects: [create-flow-mitigations]

# Tech tracking
tech-stack:
  added: []
  patterns: [kubectl version JSON parsing, case-insensitive error output matching]

key-files:
  created:
    - pkg/internal/doctor/versionskew.go
    - pkg/internal/doctor/versionskew_test.go
    - pkg/internal/doctor/socket.go
    - pkg/internal/doctor/socket_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/gpu_test.go

key-decisions:
  - "referenceK8sMinor = 31 as package constant, matching default kind node image version"
  - "kubectl version skew tolerance is +/-1 minor (matching Kubernetes upstream skew policy)"
  - "dockerSocketCheck is linux-only since macOS Docker Desktop manages socket permissions automatically"
  - "Non-permission docker info failures return ok (daemon-not-running handled by container-runtime check)"

patterns-established:
  - "JSON command output parsing: exec.CombinedOutputLines -> strings.Join -> json.Unmarshal"
  - "Case-insensitive error matching: strings.ToLower on joined output lines"

requirements-completed: [DOCK-04, DOCK-05]

# Metrics
duration: 3min
completed: 2026-03-06
---

# Phase 39 Plan 02: kubectl Version Skew and Docker Socket Checks Summary

**kubectl semver skew detection against reference K8s minor version and Docker socket permission denied detection with usermod fix**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-06T15:45:12Z
- **Completed:** 2026-03-06T15:48:12Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- kubectlVersionSkewCheck parses `kubectl version --client -o json`, compares minor version against referenceK8sMinor (31), warns when skew exceeds +/-1
- dockerSocketCheck runs `docker info`, detects "permission denied" case-insensitively, suggests `usermod -aG docker $USER` fix
- allChecks registry complete at 10 checks: Runtime(1), Docker(4), Tools(2), GPU(3)
- Full project test suite passes with zero regressions

## Task Commits

Each task was committed atomically (TDD: test -> feat):

1. **Task 1: Implement kubectl version skew check**
   - `4678d2b1` (test) - Failing tests for kubectl version skew check
   - `d71b5965` (feat) - kubectl version skew check implementation
2. **Task 2: Implement Docker socket permission check and register all Phase 39 checks**
   - `5d07fc1a` (test) - Failing tests for docker socket and registry checks
   - `793dc9f8` (feat) - Docker socket check implementation, allChecks updated to 10

## Files Created/Modified
- `pkg/internal/doctor/versionskew.go` - kubectlVersionSkewCheck with semver parsing and diff comparison
- `pkg/internal/doctor/versionskew_test.go` - 8 table-driven tests for skip/ok/warn/parse-error scenarios
- `pkg/internal/doctor/socket.go` - dockerSocketCheck with case-insensitive permission denied detection
- `pkg/internal/doctor/socket_test.go` - 5 table-driven tests for socket permission scenarios + 10-check registry test
- `pkg/internal/doctor/check.go` - allChecks expanded from 8 to 10 with docker-socket and kubectl-version-skew
- `pkg/internal/doctor/gpu_test.go` - TestAllChecks_RegisteredOrder updated from 8 to 10 checks

## Decisions Made
- referenceK8sMinor = 31 as package constant, with comment to update when bumping kind node image
- kubectl version skew tolerance is +/-1 minor, matching Kubernetes upstream version skew policy
- dockerSocketCheck is linux-only since macOS Docker Desktop manages socket permissions automatically
- Non-permission docker info failures return ok status (daemon-not-running already handled by container-runtime check)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed daemon-json check name in registry test**
- **Found during:** Task 2 (registry test)
- **Issue:** Plan specified "daemon-json" but actual check name is "daemon-json-init"
- **Fix:** Updated test expectation to match actual check name
- **Files modified:** pkg/internal/doctor/socket_test.go
- **Verification:** All tests pass
- **Committed in:** 793dc9f8 (Task 2 GREEN commit)

**2. [Rule 3 - Blocking] Updated existing TestAllChecks_RegisteredOrder for 10 checks**
- **Found during:** Task 2 (full test suite)
- **Issue:** Existing test in gpu_test.go expected 8 checks, now there are 10
- **Fix:** Updated count and expected list to include docker-socket and kubectl-version-skew
- **Files modified:** pkg/internal/doctor/gpu_test.go
- **Verification:** All tests pass
- **Committed in:** 793dc9f8 (Task 2 GREEN commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes necessary for test correctness. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 39 complete: all 10 diagnostic checks implemented and tested
- allChecks registry ready for create-flow mitigations integration (Phase 41)
- Full test suite passes with zero regressions

## Self-Check: PASSED

All 6 key files verified on disk. All 4 commit hashes verified in git log. allChecks has exactly 10 entries.

---
*Phase: 39-docker-and-tool-configuration-checks*
*Completed: 2026-03-06*
