---
phase: 45-host-directory-mounting
plan: "04"
subsystem: doctor
tags: [mount-checks, config-flag, gap-closure, SC-3]
dependency_graph:
  requires: [45-02]
  provides: [MOUNT-03]
  affects: [pkg/cmd/kind/doctor, pkg/internal/doctor]
tech_stack:
  added: []
  patterns: [dependency-injection, interface-type-assertion, tdd-red-green]
key_files:
  created:
    - path: pkg/cmd/kind/doctor/doctor_test.go
      purpose: Tests for extractMountPaths deduplication logic
  modified:
    - path: pkg/internal/doctor/check.go
      purpose: Added mountPathConfigurable interface and SetMountPaths exported function
    - path: pkg/internal/doctor/hostmount.go
      purpose: Added setMountPaths method to both mount check types
    - path: pkg/cmd/kind/doctor/doctor.go
      purpose: Added --config flag, extractMountPaths helper, and runE wiring
    - path: pkg/internal/doctor/hostmount_test.go
      purpose: Added TestSetMountPaths, TestHostMountPathCheck_SetMountPaths, TestDockerDesktopFileSharingCheck_SetMountPaths
decisions:
  - "mountPathConfigurable interface is unexported (internal to doctor package); SetMountPaths is exported for cmd layer"
  - "extractMountPaths deduplicates by seen map to handle multi-node configs with shared host paths"
  - "SetMountPaths(nil) restores skip behavior for backward compatibility without --config"
  - "encoding.Load already handles empty path (returns default cluster); doctor skips SetMountPaths when Config is empty string"
metrics:
  duration: 2m
  completed_date: "2026-04-09"
  tasks_completed: 2
  files_modified: 5
  commits: 4
---

# Phase 45 Plan 04: SC-3 Gap Closure — Wire ExtraMounts into Doctor Mount Checks Summary

**One-liner:** `kinder doctor --config cluster.yaml` extracts ExtraMounts host paths via mountPathConfigurable interface and injects them into both mount checks before RunAllChecks, closing SC-3.

## Tasks Completed

| # | Name | Commit | Status |
|---|------|--------|--------|
| 1 (RED) | Failing tests for setMountPaths and SetMountPaths | 4d10a054 | PASS |
| 1 (GREEN) | mountPathConfigurable interface and SetMountPaths | 8dea75a8 | PASS |
| 2 (RED) | Failing tests for extractMountPaths | 6f4ce7b8 | PASS |
| 2 (GREEN) | --config flag, extractMountPaths, runE wiring | b1257689 | PASS |

## What Was Built

### Task 1: mountPathConfigurable interface + SetMountPaths

`pkg/internal/doctor/hostmount.go` gained `setMountPaths(paths []string)` methods on both `hostMountPathCheck` and `dockerDesktopFileSharingCheck`. The method replaces the `getMountPaths` closure, so subsequent `Run()` calls iterate the supplied paths instead of returning a skip result.

`pkg/internal/doctor/check.go` gained:
- `mountPathConfigurable` (unexported interface): `setMountPaths([]string)`
- `SetMountPaths(paths []string)` (exported): iterates `allChecks`, type-asserts each to `mountPathConfigurable`, calls `setMountPaths` on matches

### Task 2: --config flag and ExtraMounts wiring

`pkg/cmd/kind/doctor/doctor.go` gained:
- `Config string` field on `flagpole`
- `--config` cobra flag registered with descriptive usage string
- `extractMountPaths(cfg *config.Cluster) []string` helper: walks all nodes' ExtraMounts, deduplicates by host path, returns nil if none
- `runE` updated: when `flags.Config != ""`, calls `encoding.Load(flags.Config)` then `doctor.SetMountPaths(paths)` before `doctor.RunAllChecks()`

## Verification Results

```
go test ./pkg/internal/doctor/... -v -count=1   PASS (all tests)
go test ./pkg/cmd/kind/doctor/... -v -count=1   PASS (all tests)
go vet ./pkg/internal/doctor/... ./pkg/cmd/kind/doctor/...   CLEAN
go build ./...   BUILD OK
```

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None. Both mount checks now receive real paths from --config and execute their full logic.

## Threat Flags

No new network endpoints, auth paths, file access patterns, or schema changes beyond what the threat model covers (T-45-04-01 through T-45-04-03 all accepted).

## Self-Check: PASSED

- `pkg/cmd/kind/doctor/doctor_test.go` exists on disk: FOUND
- `pkg/internal/doctor/check.go` (modified, SetMountPaths present): FOUND
- `git log --oneline --all --grep="45-04"` returns 4 commits: FOUND
