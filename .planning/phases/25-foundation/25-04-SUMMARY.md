---
phase: 25-foundation
plan: 04
subsystem: infra
tags: [crypto, sha256, subnet, permissions, logging, error-naming, go-conventions]

# Dependency graph
requires:
  - phase: 25-foundation
    provides: 25-01 through 25-03 code quality fixes (go version, linting, layer violations)
provides:
  - SHA-256 subnet generation in docker/podman/nerdctl providers
  - 0755 log directory permissions in provider.go
  - Dashboard token logged at V(1) (sensitive output)
  - ErrNoNodeProviderDetected following Go error naming convention
  - subnet_test.go using stdlib strings.Contains (no custom helper)
affects: [26-context-refactor, any code referencing ErrNoNodeProviderDetected]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Go error variable naming: Err prefix for exported error vars"
    - "Log verbosity: sensitive values (tokens) at V(1), user-facing output at V(0)"
    - "Prefer stdlib functions over custom helpers in tests"
    - "Use stronger hash algorithms (SHA-256 over SHA-1) even for non-cryptographic uses"

key-files:
  created: []
  modified:
    - pkg/cluster/internal/providers/docker/network.go
    - pkg/cluster/internal/providers/podman/network.go
    - pkg/cluster/internal/providers/nerdctl/network.go
    - pkg/cluster/provider.go
    - pkg/cluster/internal/create/actions/installdashboard/dashboard.go
    - pkg/cluster/internal/create/actions/installmetallb/subnet_test.go

key-decisions:
  - "SHA-256 replaces SHA-1 for subnet generation; subnet values will change for existing clusters but clusters are transient so this is acceptable"
  - "ErrNoNodeProviderDetected is technically a breaking API change for external callers but kinder is not consumed as a library"
  - "Dashboard token at V(1): token is sensitive, should not appear in default output"

patterns-established:
  - "Pattern: Go error naming — exported error vars use Err prefix"
  - "Pattern: Sensitive values in logs use V(1) or higher"

requirements-completed: [FOUND-06, FOUND-07, FOUND-08, FOUND-09, FOUND-10]

# Metrics
duration: 2min
completed: 2026-03-03
---

# Phase 25 Plan 04: Code Quality Fixes Summary

**Five targeted mechanical fixes: SHA-256 subnet hashing across docker/podman/nerdctl providers, 0755 log directory permissions, dashboard token at V(1), ErrNoNodeProviderDetected Go naming, and strings.Contains replacing custom test helper.**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-03T23:50:42Z
- **Completed:** 2026-03-03T23:52:22Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Replaced SHA-1 with SHA-256 in all three provider network.go files (docker, podman, nerdctl) for subnet generation
- Fixed log directory permissions from os.ModePerm (0777) to 0755 in provider.go; dashboard token now logged at V(1) for sensitive output control
- Renamed NoNodeProviderDetectedError to ErrNoNodeProviderDetected (Go convention) and removed custom contains/containsStr helpers from subnet_test.go in favor of strings.Contains

## Task Commits

Each task was committed atomically:

1. **Task 1: SHA-256 subnet generation + log dir permissions + dashboard token log level** - `2e8c3459` (fix)
2. **Task 2: Rename error var to Go convention + remove redundant test helper** - `46b55031` (fix)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `pkg/cluster/internal/providers/docker/network.go` - crypto/sha1 -> crypto/sha256, sha1.New() -> sha256.New()
- `pkg/cluster/internal/providers/podman/network.go` - crypto/sha1 -> crypto/sha256, sha1.New() -> sha256.New()
- `pkg/cluster/internal/providers/nerdctl/network.go` - crypto/sha1 -> crypto/sha256, sha1.New() -> sha256.New()
- `pkg/cluster/provider.go` - os.ModePerm -> 0755 for MkdirAll; NoNodeProviderDetectedError -> ErrNoNodeProviderDetected (declaration + usage + comment)
- `pkg/cluster/internal/create/actions/installdashboard/dashboard.go` - token line V(0) -> V(1)
- `pkg/cluster/internal/create/actions/installmetallb/subnet_test.go` - added "strings" import; replaced contains() calls with strings.Contains(); deleted contains and containsStr helper functions

## Decisions Made

- SHA-256 produces different subnet values than SHA-1, but clusters are transient so existing cluster subnets are not a concern
- ErrNoNodeProviderDetected is a breaking exported API change; acceptable because kinder is not consumed as a library
- Token line specifically moved to V(1); other dashboard output lines (header, port-forward command, URL) remain at V(0) for user-facing visibility

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All five code quality findings (FOUND-06 through FOUND-10) from the Phase 25 codebase review are resolved
- Phase 25 Foundation is complete (all 4 plans executed: go version, linting, layer violation, code quality fixes)
- Ready for Phase 26 (context-in-struct refactor) once verifier marks phase 25 complete

## Self-Check: PASSED

- SUMMARY.md: FOUND
- Commit 2e8c3459 (Task 1): FOUND
- Commit 46b55031 (Task 2): FOUND

---
*Phase: 25-foundation*
*Completed: 2026-03-03*
