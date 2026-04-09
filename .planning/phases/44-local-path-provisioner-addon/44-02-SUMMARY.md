---
phase: 44
plan: 02
subsystem: testing
tags: [unit-tests, doctor, offline-readiness, local-path-provisioner]
requires: ["44-01"]
provides: ["installlocalpath unit tests", "offlinereadiness LocalPath entries", "images_test LocalPath coverage"]
affects: ["pkg/internal/doctor", "pkg/cluster/internal/providers/common"]
tech-stack:
  added: []
  patterns: ["FakeNode/FakeCmd table-driven tests", "go test parallel subtests"]
key-files:
  created:
    - pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go
  modified:
    - pkg/internal/doctor/offlinereadiness.go
    - pkg/internal/doctor/offlinereadiness_test.go
    - pkg/cluster/internal/providers/common/images_test.go
    - pkg/internal/doctor/gpu_test.go
    - pkg/internal/doctor/socket_test.go
key-decisions:
  - "Used docker.io/ prefix for offlinereadiness entries to match how localpathprovisioner.go declares Images var"
  - "TestAllChecks count tests updated to 21 (not in plan) because feat(44-03) added local-path-cve check out of order"
duration: ~15min
completed: 2026-04-09
---

# Phase 44 Plan 02: Unit Tests and Offline-Readiness for Local-Path-Provisioner Summary

Unit tests added for installlocalpath using FakeNode/FakeCmd pattern, offlinereadiness updated to track 14 addon images including local-path-provisioner and busybox, and images_test extended with LocalPath coverage.

## Duration

- Start: 2026-04-09T13:10:00Z
- End: 2026-04-09T13:25:51Z
- Duration: ~16 minutes

## Tasks Completed

2/2 tasks completed (plus 1 auto-fix deviation).

## Accomplishments

1. Created `localpathprovisioner_test.go` — table-driven `TestExecute` with success/apply-fail/wait-fail cases using FakeNode/FakeCmd infrastructure matching `installmetricsserver` pattern. `TestImages` confirms exactly 2 image entries.
2. Updated `offlinereadiness.go` — added 2 new entries (local-path-provisioner v0.0.35 and busybox:1.37.0 with docker.io/ prefix), updated header comment total from 12 to 14.
3. Updated `offlinereadiness_test.go` — changed `TestAllAddonImages_CountMatchesExpected` expected value from 12 to 14.
4. Updated `images_test.go` — added `installlocalpath` import, added `LocalPath: true` to `TestRequiredAddonImages_AllEnabled` with `want.Insert(installlocalpath.Images...)`, added new `TestRequiredAddonImages_LocalPathOnly` test.
5. Auto-fixed `gpu_test.go` and `socket_test.go` — count tests for `AllChecks()` were expecting 20 but `feat(44-03)` (run out of order) had already added `newLocalPathCVECheck()` making the actual count 21.

## Task Commits

| Task | Commit | Description |
|------|--------|-------------|
| Task 1 | 489d0f3e | test(44-02): add installlocalpath unit tests |
| Task 2 | c8438604 | feat(44-02): update offlinereadiness and images_test for LocalPath |
| Deviation fix | 6f899191 | fix(44-02): update AllChecks registry count tests to 21 |

## Files Created

- `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go` (125 lines)

## Files Modified

- `pkg/internal/doctor/offlinereadiness.go` — added 2 image entries, updated comment total
- `pkg/internal/doctor/offlinereadiness_test.go` — updated expected count 12 → 14
- `pkg/cluster/internal/providers/common/images_test.go` — added import, LocalPath: true in AllEnabled, new LocalPathOnly test
- `pkg/internal/doctor/gpu_test.go` — updated count 20 → 21, added local-path-cve entry (deviation fix)
- `pkg/internal/doctor/socket_test.go` — updated count 20 → 21, added local-path-cve entry (deviation fix)

## Decisions Made

1. `docker.io/` prefix for offlinereadiness entries — consistent with how `localpathprovisioner.go` declares the `Images` var and how other entries in `allAddonImages` use fully-qualified references.
2. AllEnabled test places `installlocalpath.Images` between `installdashboard` and `installnvidiagpu` — follows registry order in `images.go`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] AllChecks count tests expected 20 but actual count is 21**
- **Found during:** Task 2 verification (`go test ./pkg/internal/doctor/...`)
- **Issue:** `feat(44-03)` commit (executed out of plan order) added `newLocalPathCVECheck()` to the `allChecks` registry, bringing the total from 20 to 21. `TestAllChecks_RegisteredOrder` (gpu_test.go) and `TestAllChecks_Registry` (socket_test.go) still asserted count=20 and did not include the `local-path-cve` entry in their ordered expected slice.
- **Fix:** Updated both tests to expect 21 entries and added `{"local-path-cve", "Cluster"}` between `cluster-node-skew` and `offline-readiness` in the ordered expected slice.
- **Files modified:** `pkg/internal/doctor/gpu_test.go`, `pkg/internal/doctor/socket_test.go`
- **Commit:** 6f899191

## Verification Results

All plan success criteria met:

- [x] `go test ./pkg/cluster/internal/create/actions/installlocalpath/...` — PASS
- [x] `go test ./pkg/internal/doctor/...` — PASS (14 addon images, 21 checks)
- [x] `go test ./pkg/cluster/internal/providers/common/...` — PASS (LocalPath coverage)
- [x] `go build ./...` — PASS
- [x] `go vet ./...` — PASS

## Self-Check: PASSED

- [x] `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go` — EXISTS
- [x] Commit 489d0f3e — EXISTS (test(44-02): add installlocalpath unit tests)
- [x] Commit c8438604 — EXISTS (feat(44-02): update offlinereadiness and images_test for LocalPath)
- [x] Commit 6f899191 — EXISTS (fix(44-02): update AllChecks registry count tests to 21)
- [x] `grep 'local-path-provisioner' pkg/internal/doctor/offlinereadiness.go` — FOUND
- [x] `grep 'busybox:1.37.0' pkg/internal/doctor/offlinereadiness.go` — FOUND
- [x] `grep 'LocalPath' pkg/cluster/internal/providers/common/images_test.go` — FOUND
