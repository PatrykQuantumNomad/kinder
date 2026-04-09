---
phase: 43-air-gapped-cluster-creation
plan: "03"
subsystem: doctor
tags: [doctor, offline, air-gapped, images, documentation]
dependency_graph:
  requires: ["43-01"]
  provides: ["offline-readiness doctor check", "working-offline.md addon workflow"]
  affects: ["pkg/internal/doctor", "site/content/docs/user/working-offline.md"]
tech_stack:
  added: []
  patterns: ["injectable-dependency check pattern (inspectImage/lookPath)", "tabwriter table output"]
key_files:
  created:
    - pkg/internal/doctor/offlinereadiness.go
    - pkg/internal/doctor/offlinereadiness_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/socket_test.go
    - pkg/internal/doctor/gpu_test.go
    - site/content/docs/user/working-offline.md
decisions:
  - "Image list defined inline in offlinereadiness.go — no import from pkg/cluster/internal (avoids import cycle)"
  - "lookPath checked first to detect runtime before inspecting images; skip if none found"
  - "tabwriter used for missing-image table, consistent with clusterskew.go pattern"
  - "Registry count tests (socket_test.go + gpu_test.go) updated 19→20 as part of Rule 1 auto-fix"
metrics:
  duration: "~7 minutes"
  completed: "2026-04-09T12:24:26Z"
  tasks_completed: 2
  files_modified: 6
requirements: [AIRGAP-05, AIRGAP-06]
---

# Phase 43 Plan 03: Doctor Offline-Readiness Check and Working-Offline Docs Summary

**One-liner:** Offline-readiness doctor check with injectable image inspection reports missing addon images by name, plus two-mode offline workflow documented in working-offline.md.

## Tasks Completed

| # | Name | Commit | Files |
|---|------|--------|-------|
| 1 | Create offline-readiness doctor check with tests | f2e84747 | offlinereadiness.go, offlinereadiness_test.go, check.go, socket_test.go, gpu_test.go |
| 2 | Extend working-offline.md with two-mode addon image workflow | ed996183 | working-offline.md |

## What Was Built

### Task 1: offline-readiness doctor check

- `offlineReadinessCheck` struct with `inspectImage func(string) bool` and `lookPath func(string)(string,error)` injected fields
- `allAddonImages` slice: 12 entries covering Load Balancer (HA), Local Registry, MetalLB (2), Metrics Server, Cert Manager (3), Envoy Gateway (2), Dashboard, NVIDIA GPU
- Image tags verified against actual embedded manifests and const files before hardcoding
- `Run()` logic: skip if no runtime, ok if all present, warn with tabwriter table if any absent
- `realInspectImage`: detects first available runtime (docker/podman/nerdctl), runs `<rt> inspect --type=image <image>`
- Registered as `newOfflineReadinessCheck()` in `allChecks` under "Offline" category
- 5 tests: AllPresent (ok), SomeAbsent (warn + MetalLB label), AllAbsent (warn + count), NoRuntime (skip), CountMatchesExpected (len==12)

### Task 2: working-offline.md addon workflow

Added "Addon images" section with:
- How to check which images are needed (`kinder doctor` + offline-readiness check)
- How to pre-load images (docker pull/save/load workflow with MetalLB example)
- How to create in air-gapped mode (`kinder create cluster --air-gapped`)
- Two-mode offline workflow table:
  - Mode 1: Pre-create image baking (recommended, uses --air-gapped)
  - Mode 2: Post-create image loading (kinder load docker-image)
- Note about Docker requirement for Mode 2

All existing documentation preserved. Link references at bottom of file intact.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated registry count assertions in existing tests**
- **Found during:** Task 1 — `go test` after registering new check
- **Issue:** `TestAllChecks_Registry` (socket_test.go:163) and `TestAllChecks_RegisteredOrder` (gpu_test.go:290) both asserted `len(AllChecks()) == 19`; adding the new check made them fail with 20
- **Fix:** Updated count from 19→20 and appended `{"offline-readiness", "Offline"}` to expected slice in both tests
- **Files modified:** pkg/internal/doctor/socket_test.go, pkg/internal/doctor/gpu_test.go
- **Commit:** f2e84747 (included in Task 1 commit)

## Known Stubs

None — the check uses real `realInspectImage` with container runtime CLI; no hardcoded empty values flow to UI rendering.

## Threat Flags

None — the new files do not introduce network endpoints, auth paths, file access patterns, or schema changes at trust boundaries. The check is read-only (runs container inspect commands).

## Self-Check: PASSED

- [x] `pkg/internal/doctor/offlinereadiness.go` exists on disk
- [x] `pkg/internal/doctor/offlinereadiness_test.go` exists on disk
- [x] `git log --oneline --all --grep="43-03"` returns 2 commits (f2e84747, ed996183)
- [x] `go build ./...` succeeds
- [x] `go test ./pkg/internal/doctor/...` passes (ok, 0.350s)
