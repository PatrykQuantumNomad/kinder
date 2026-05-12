---
phase: 56-debt-04-doctor-test-race-fix
plan: "01"
subsystem: doctor
tags: [race-fix, refactor, test, ci, debt]
dependency_graph:
  requires: []
  provides: [runChecks-helper, race-free-doctor-tests, test-race-doctor-target, race-check-ci]
  affects: [pkg/internal/doctor/check.go, pkg/internal/doctor/check_test.go, Makefile, .github/workflows/race-check.yml]
tech_stack:
  added: []
  patterns: [parameter-injection, unexported-helper, path-gated-ci-workflow]
key_files:
  created:
    - .github/workflows/race-check.yml
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/check_test.go
    - Makefile
decisions:
  - "Fix is parameter-injection (not synchronization): runChecks(checks []Check) helper; no sync primitives; allChecks global remains a plain init-time var"
  - "TestSetMountPaths in hostmount_test.go left untouched (exercises public SetMountPaths API; out-of-scope guard held)"
  - "race-check.yml path-gated to pkg/internal/doctor/** to bound CI cost; 10-minute timeout per T-56-02 threat mitigation"
metrics:
  duration: "~2.5 min"
  completed: "2026-05-12T18:36:52Z"
---

# Phase 56 Plan 01: DEBT-04 Doctor Test Race Fix Summary

Eliminated the `allChecks` t.Parallel() data race in `pkg/internal/doctor/check_test.go` via parameter-injection refactor: new unexported `runChecks(checks []Check) []Result` helper; three racing parallel tests call it with local slices instead of mutating the package global; 100-iteration race gate and CI regression guard added.

## Files Changed

| File | Change | +/- Lines |
|------|--------|-----------|
| `pkg/internal/doctor/check.go` | Added `runChecks` helper; `RunAllChecks` → one-line delegate | +19, -3 |
| `pkg/internal/doctor/check_test.go` | Three tests: removed save/restore/allChecks= pattern; call `runChecks(checks)` | +6, -15 |
| `Makefile` | Added `test-race-doctor` target + `.PHONY` entry | +3, -1 |
| `.github/workflows/race-check.yml` | New file: PR regression guard (path-gated, ubuntu-24.04, SHA-pinned, CGO_ENABLED=1) | +48 |

## Race Detector Results

**Before fix:** `CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100` would produce `WARNING: DATA RACE` on the `allChecks` global when the three parallel tests ran concurrently.

**After fix (100-iteration run — authoritative SC1 gate):**

```
    --- PASS: TestInotifyCheck_Run/values_above_threshold (0.00s)
    --- PASS: TestInotifyCheck_Run/both_files_unreadable (0.00s)
    --- PASS: TestInotifyCheck_Run/watches_unreadable_but_instances_ok (0.00s)
    --- PASS: TestInotifyCheck_Run/watches_too_low (0.00s)
    --- PASS: TestInotifyCheck_Run/both_too_low (0.00s)
    --- PASS: TestInotifyCheck_Run/instances_too_low (0.00s)
PASS
ok  	sigs.k8s.io/kind/pkg/internal/doctor	2.57s
```

**Verdict:** PASS — zero `WARNING: DATA RACE` lines. Elapsed: ~2.57s for 100 iterations.

## Success Criteria Results

| Gate | Command | Result |
|------|---------|--------|
| SC1 | `CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100` | PASS — zero races, exit 0 |
| SC2 | `! grep -nE 'sync\.(RW)?Mutex\|sync\.OnceValues' pkg/internal/doctor/check.go` | PASS — no matches |
| SC3 | `git diff HEAD~3 HEAD -- pkg/internal/doctor/check.go \| grep -cE '^\+.*sync\.'` | PASS — count=0 |

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | `9d57b54b` | `refactor(56-01): extract runChecks helper; make RunAllChecks a one-line delegate` |
| 2 | `b797b729` | `fix(56-01): rewrite 3 racing parallel tests to call runChecks with local slices` |
| 3 | `ee9b0af0` | `ci(56-01): add test-race-doctor Makefile target and race-check.yml CI workflow` |

## Deviations from Plan

None — plan executed exactly as written. All three tasks followed the verbatim actions from the plan's `<action>` blocks.

## Out-of-Scope Guards Held

- `pkg/internal/doctor/hostmount_test.go::TestSetMountPaths`: untouched. This test is intentionally non-parallel and exercises the public `SetMountPaths` API. Refactoring it would change the public surface — explicitly locked out of scope per the plan.
- Existing `test-race` Makefile target (scoped to `./pkg/cluster/internal/create/...`): unchanged. The new `test-race-doctor` target is purely additive.
- `allChecks` package-level `var` declaration in `check.go`: left as a plain init-time var with no synchronization primitives added (SC2 guard).

## REQUIREMENTS.md socket_test.go Doc Drift Note

REQUIREMENTS.md (line 33) references `pkg/internal/doctor/socket_test.go` as a race site alongside `check_test.go`. This is documentation drift. Per 56-RESEARCH.md Open Question 1, the actual mutation sites (`allChecks =` writes) are exclusively in `check_test.go`. The `socket_test.go` is a read-only race victim — it reads `allChecks` via `RunAllChecks()` concurrently while `check_test.go` writes to the global. No REQUIREMENTS.md edit was performed beyond the DEBT-04 mark-complete (the drift is cosmetic and the description's intent is satisfied by this fix). Future docs-cleanup phases may revise the wording.

## Known Stubs

None — all code is wired end-to-end. `runChecks` returns real check results; `RunAllChecks` delegates to it with the real `allChecks` registry.

## Threat Flags

None — this plan introduces no new network endpoints, auth paths, file access patterns, or schema changes. The only new surface is `.github/workflows/race-check.yml` which is path-gated and SHA-pinned (T-56-04 mitigated; T-56-02 mitigated via 10-minute timeout and path filter).

## Self-Check: PASSED

- `pkg/internal/doctor/check.go` — modified, confirmed present
- `pkg/internal/doctor/check_test.go` — modified, confirmed present
- `Makefile` — modified, confirmed present
- `.github/workflows/race-check.yml` — created, confirmed present
- Commit `9d57b54b` — confirmed in git log
- Commit `b797b729` — confirmed in git log
- Commit `ee9b0af0` — confirmed in git log
- SC1/SC2/SC3 all green — verified above
