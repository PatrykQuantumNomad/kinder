---
phase: 19-bug-fixes
plan: 01
subsystem: infra
tags: [port-mapping, tar-extraction, file-descriptors, docker, nerdctl, podman]

# Dependency graph
requires: []
provides:
  - "Fixed generatePortMappings in docker/nerdctl/podman: port listeners released per-iteration, not deferred to function return"
  - "Fixed extractTarball: truncated tar archives return descriptive error instead of silently succeeding"
affects: [cluster-creation, node-image-build]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Release resources immediately after use in loops — do not use defer inside loops"
    - "Return descriptive error messages for truncation vs generic I/O errors"

key-files:
  created:
    - pkg/cluster/internal/providers/podman/provision_test.go
  modified:
    - pkg/cluster/internal/providers/docker/provision.go
    - pkg/cluster/internal/providers/docker/provision_test.go
    - pkg/cluster/internal/providers/nerdctl/provision.go
    - pkg/cluster/internal/providers/nerdctl/provision_test.go
    - pkg/cluster/internal/providers/podman/provision.go
    - pkg/build/nodeimage/internal/kube/tar.go
    - pkg/build/nodeimage/internal/kube/tar_test.go

key-decisions:
  - "Release port listeners immediately after building the --publish arg, not via defer — prevents FD accumulation with many port mappings"
  - "Handle both io.EOF and io.ErrUnexpectedEOF from io.CopyN as truncation errors — tar.Reader returns ErrUnexpectedEOF for truncated bodies"
  - "Truncation error message includes 'truncat' keyword to distinguish from generic I/O failures"

patterns-established:
  - "Pattern: Never use defer inside a loop for resource cleanup — call the cleanup function immediately after use"
  - "Pattern: Return error-containing 'truncat' for archive truncation to enable diagnostic distinction from other I/O errors"

# Metrics
duration: 25min
completed: 2026-03-03
---

# Phase 19 Plan 01: Bug Fixes Summary

**Port listener defer-in-loop FD leak fixed across docker/nerdctl/podman providers; tar extraction now returns truncation error instead of silently stopping mid-archive**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-03-03T00:00:00Z
- **Completed:** 2026-03-03
- **Tasks:** 2
- **Files modified:** 7 modified, 1 created

## Accomplishments

- Removed `defer releaseHostPortFn()` from `generatePortMappings` loops in all 3 providers (docker, nerdctl, podman) — listeners are now released immediately after the port number is captured
- Fixed `extractTarball` to return `"archive truncated: unexpected EOF while extracting <name>"` instead of silently breaking the loop when `io.CopyN` encounters EOF mid-body
- Added comprehensive `Test_generatePortMappings` tests to docker, nerdctl, and podman provision_test.go files
- Created `TestExtractTarball_Truncated` with `writeTruncatedTarGz` fixture helper to prove the truncation error is returned

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix BUG-01 — defer-in-loop port leak in generatePortMappings across all 3 providers** - `31277eb1` (fix)
2. **Task 2: Fix BUG-02 — silent tar truncation in extractTarball** - `7f08faae` (fix)

**Plan metadata:** (to be added)

_Note: TDD tasks had combined RED+GREEN commits since the bug was a structural resource leak (not functional failure), making RED tests structurally pass before the fix._

## Files Created/Modified

- `pkg/cluster/internal/providers/docker/provision.go` - Removed `defer` from releaseHostPortFn, call immediately after append
- `pkg/cluster/internal/providers/docker/provision_test.go` - Added Test_generatePortMappings with 5 table-driven subtests
- `pkg/cluster/internal/providers/nerdctl/provision.go` - Same defer-to-immediate fix
- `pkg/cluster/internal/providers/nerdctl/provision_test.go` - Added Test_generatePortMappings with 4 subtests
- `pkg/cluster/internal/providers/podman/provision.go` - Same defer-to-immediate fix
- `pkg/cluster/internal/providers/podman/provision_test.go` - Created new file with 6 subtests including port release verification
- `pkg/build/nodeimage/internal/kube/tar.go` - Changed io.EOF/io.ErrUnexpectedEOF handling to return truncation error
- `pkg/build/nodeimage/internal/kube/tar_test.go` - Added TestExtractTarball_Truncated and writeTruncatedTarGz helper

## Decisions Made

- Used `io.ErrUnexpectedEOF` handling alongside `io.EOF` in tar.go because `tar.Reader` wraps partial reads into `ErrUnexpectedEOF` rather than bare `io.EOF`
- Created separate `writeTruncatedTarGz` helper (vs the existing `writeTarGz`) to build a deliberately malformed archive with mismatched header size and body length
- The "port release timing" test (verifying ports can be rebound immediately after function return) works correctly with the fix and serves as a regression guard

## Deviations from Plan

None - plan executed exactly as written, with one clarification:

The BUG-02 test discovery revealed that `io.CopyN` with `tar.Reader` returns `io.ErrUnexpectedEOF` (not `io.EOF`) for truncated bodies. The plan described handling `io.EOF` at `CopyN`, but the actual error is `io.ErrUnexpectedEOF`. The fix handles both cases (both map to truncation error), which is more robust. This was discovered during the RED phase run and handled inline.

## Issues Encountered

- **BUG-02 test fixture design:** Initial approach (gzip-level truncation at 30%) caused errors at the gzip/tar-reader level (`unexpected EOF` at `tr.Next()`), not at `io.CopyN`. This meant the test would pass even without the fix. Switched to a tar-level truncation strategy: write a header declaring 4096-byte body but only write 5 bytes, then gzip the raw (invalid) tar. This correctly triggers `io.CopyN` to return `io.ErrUnexpectedEOF`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Both bugs fixed with tests; provider and kube package test suites pass cleanly
- No regressions: `go test ./pkg/... -short -tags=nointegration` exits 0
- Ready for next plan in phase 19

---
*Phase: 19-bug-fixes*
*Completed: 2026-03-03*

## Self-Check: PASSED

- docker/provision.go: FOUND
- nerdctl/provision.go: FOUND
- podman/provision.go: FOUND
- podman/provision_test.go: FOUND
- tar.go: FOUND
- SUMMARY.md: FOUND
- Commit 31277eb1: FOUND
- Commit 7f08faae: FOUND
