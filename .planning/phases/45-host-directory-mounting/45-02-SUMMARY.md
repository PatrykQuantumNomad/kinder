---
phase: 45-host-directory-mounting
plan: "02"
subsystem: doctor
tags: [doctor, mounts, diagnostics, macos, file-sharing]
requires: []
provides: [host-mount-path check, docker-desktop-file-sharing check]
affects: [pkg/internal/doctor]
tech-stack:
  added: []
  patterns: [injected-deps check pattern, isPathCovered prefix-safe helper]
key-files:
  created:
    - pkg/internal/doctor/hostmount.go
    - pkg/internal/doctor/hostmount_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/socket_test.go
    - pkg/internal/doctor/gpu_test.go
key-decisions:
  - newHostMountPathCheck uses injected getMountPaths and statPath for full test isolation
  - dockerDesktopFileSharingCheck falls back to defaultFileSharingDirs when settings-store.json is absent or malformed
  - isPathCovered uses HasPrefix(path, dir+"/") guard to prevent /Userspace matching /Users
  - Both checks skip gracefully returning single skip result when getMountPaths returns nil or empty
duration: "2m 11s"
completed: "2026-04-09T16:38:15Z"
---

# Phase 45 Plan 02: Host Mount Doctor Checks Summary

Two new kinder doctor checks — `host-mount-path` (all platforms) and `docker-desktop-file-sharing` (macOS only) — added to the 23-entry allChecks registry with injected-dep test isolation.

## Performance

- Duration: 2m 11s
- Start: 2026-04-09T16:36:04Z
- End: 2026-04-09T16:38:15Z
- Tasks completed: 2/2
- Files created: 2
- Files modified: 3

## Accomplishments

- Created `hostmount.go` with `hostMountPathCheck` (fail + mkdir fix when path missing, warn when inaccessible, skip when unconfigured) and `dockerDesktopFileSharingCheck` (reads settings-store.json, falls back to macOS defaults, warns when mount path not covered by file sharing)
- Created `hostmount_test.go` with 17 test cases across 3 test functions (TestHostMountPathCheck 6, TestDockerDesktopFileSharingCheck 6, TestIsPathCovered 5)
- Registered both checks in `allChecks` under `// Category: Mounts (Phase 45)`
- Updated AllChecks count assertions from 21 to 23 in `socket_test.go` and `gpu_test.go`

## Task Commits

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Add hostMountPathCheck and dockerDesktopFileSharingCheck | 823f7014 |
| 2 | Register mount checks and add unit tests | dfdd78b2 |

## Files Created

- `/Users/patrykattc/work/git/kinder/pkg/internal/doctor/hostmount.go` — both check implementations + isPathCovered helper
- `/Users/patrykattc/work/git/kinder/pkg/internal/doctor/hostmount_test.go` — 17 unit tests across 3 functions

## Files Modified

- `pkg/internal/doctor/check.go` — added newHostMountPathCheck() and newDockerDesktopFileSharingCheck() to allChecks
- `pkg/internal/doctor/socket_test.go` — AllChecks count 21 -> 23, added 2 new expected entries
- `pkg/internal/doctor/gpu_test.go` — AllChecks count 21 -> 23, added 2 new expected entries

## Decisions Made

1. **Injected getMountPaths returns nil by default** — production constructor wires a real config reader in a later phase; for now defaults to nil so both checks skip gracefully with no configured paths.
2. **Default file-sharing dirs fallback** — when settings-store.json is absent (non-macOS CI, fresh Docker Desktop install), the check falls back to Docker Desktop's documented defaults (/Users, /Volumes, /private, /tmp, /var/folders) rather than erroring out.
3. **isPathCovered uses dir+"/" separator** — prevents /Users matching /Userspace/data; exact equality also accepted so /Users itself is covered.

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

- `newHostMountPathCheck()` and `newDockerDesktopFileSharingCheck()` wire `getMountPaths: func() []string { return nil }` as the production implementation. This means both checks always skip in production until a future plan (Phase 45 plan 03 or cluster creation integration) wires the real config reader. The stub is intentional and documented — no user-visible regression since these are new checks.

## Threat Flags

None — no new network endpoints, auth paths, or file writes introduced. `settings-store.json` is read-only.

## Self-Check: PASSED

- [x] `pkg/internal/doctor/hostmount.go` exists on disk
- [x] `pkg/internal/doctor/hostmount_test.go` exists on disk
- [x] Commit 823f7014 exists (feat(45-02): add hostMountPathCheck and dockerDesktopFileSharingCheck)
- [x] Commit dfdd78b2 exists (feat(45-02): register mount checks and add unit tests)
- [x] `go test ./pkg/internal/doctor/...` passes
- [x] `go vet ./pkg/internal/doctor/...` clean
- [x] AllChecks count is 23
