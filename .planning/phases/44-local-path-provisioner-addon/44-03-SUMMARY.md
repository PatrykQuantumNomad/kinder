---
phase: 44
plan: 03
subsystem: doctor
tags: [cve, local-path-provisioner, doctor-check, security]
requires: ["44-01"]
provides: ["localPathCVECheck registered in allChecks"]
affects: ["pkg/internal/doctor/check.go", "pkg/internal/doctor/localpath.go"]
tech-stack:
  added: []
  patterns: ["injectable dependency for testing", "semver comparison via version.ParseSemantic", "container exec kubectl pattern"]
key-files:
  created:
    - pkg/internal/doctor/localpath.go
    - pkg/internal/doctor/localpath_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/socket_test.go
    - pkg/internal/doctor/gpu_test.go
key-decisions:
  - "CVE threshold is v0.0.34 (first patched release); exact threshold returns ok (not vulnerable)"
  - "realGetProvisionerVersion uses container exec kubectl inside kind control-plane — same pattern as realListNodes in clusterskew.go to avoid import cycles"
  - "No new Go module dependencies — uses sigs.k8s.io/kind/pkg/exec and pkg/internal/version already in go.mod"
  - "Registry count tests in socket_test.go and gpu_test.go updated from 20 to 21 (handled by parallel 44-02 executor)"
metrics:
  duration: "2m"
  started: "2026-04-09T13:23:51Z"
  completed: "2026-04-09T13:26:37Z"
  tasks_completed: 2
  files_created: 2
  files_modified: 1
---

# Phase 44 Plan 03: Doctor CVE-2025-62878 Check Summary

**One-liner:** CVE-2025-62878 doctor check warns when local-path-provisioner in a running cluster is below v0.0.34 using injectable container-exec-kubectl version detection.

## Accomplishments

1. **Task 1 — localPathCVECheck implementation** (`afb95837`)
   - New `pkg/internal/doctor/localpath.go` with `localPathCVECheck` struct
   - `realGetProvisionerVersion` detects runtime, finds kind control-plane container via `label=io.x-k8s.kind.role=control-plane`, execs `kubectl get deployment local-path-provisioner -n local-path-storage -o jsonpath=...` inside the container
   - `Run()` compares tag against `v0.0.34` threshold using `version.ParseSemantic`
   - Handles all edge cases: no runtime, no cluster, not installed, unparseable version, no-v-prefix
   - Registered in `allChecks` under Cluster category immediately after `newClusterNodeSkewCheck()`

2. **Task 2 — Unit tests** (`ca641be0`)
   - 7 test scenarios in `localpath_test.go` using injected `getProvisionerVersion`
   - All tests pass; zero new dependencies

## Task Commits

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | localPathCVECheck + check.go registration | `afb95837` | localpath.go, check.go |
| 2 | 7 unit tests (localpath_test.go) | `ca641be0` | localpath_test.go |

## Files Created/Modified

| File | Action | Description |
|------|--------|-------------|
| `pkg/internal/doctor/localpath.go` | Created | CVE check struct, real version probe, Run logic |
| `pkg/internal/doctor/localpath_test.go` | Created | 7 injected-dependency test scenarios |
| `pkg/internal/doctor/check.go` | Modified | Added `newLocalPathCVECheck()` to allChecks |
| `pkg/internal/doctor/socket_test.go` | Modified (by 44-02) | Registry count 20 -> 21, added local-path-cve entry |
| `pkg/internal/doctor/gpu_test.go` | Modified (by 44-02) | Registry count 20 -> 21, added local-path-cve entry |

## Decisions Made

- **CVE threshold semantics:** v0.0.34 returns "ok" (it is the fix version). Only strictly less-than triggers "warn".
- **Version probe approach:** Container exec kubectl inside kind control-plane node avoids import cycles with `pkg/cluster/internal/`. Follows the same pattern as `realListNodes` in `clusterskew.go`.
- **No-v-prefix handling:** Tags like `"0.0.35"` get `"v"` prepended before `version.ParseSemantic`, matching real-world image tags that may omit the prefix.
- **Skip vs warn on "latest":** Non-semver tags (like `"latest"`) return "warn" with "unparseable" message rather than "skip" to surface the anomaly.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Registry count tests hardcoded to 20**
- **Found during:** Task 2 test run
- **Issue:** `TestAllChecks_Registry` (socket_test.go) and `TestAllChecks_RegisteredOrder` (gpu_test.go) both asserted `len(checks) == 20`. Adding `newLocalPathCVECheck()` to allChecks made count 21, breaking both tests.
- **Fix:** Updated both to expect 21 and added `{"local-path-cve", "Cluster"}` to expected order slice.
- **Note:** The parallel 44-02 executor committed these fixes (commit `6f899191`) before Task 2 was committed. No duplicate work needed.
- **Files modified:** `pkg/internal/doctor/socket_test.go`, `pkg/internal/doctor/gpu_test.go`
- **Commit:** `6f899191` (by 44-02 parallel executor)

## Verification Results

| Check | Result |
|-------|--------|
| `go build ./...` | PASS |
| `go test ./pkg/internal/doctor/... -count=1` | PASS (21 checks, all tests) |
| `go vet ./pkg/internal/doctor/...` | PASS |
| `grep localPathCVECheck localpath.go` | FOUND |
| `grep newLocalPathCVECheck check.go` | FOUND |
| `grep CVE-2025-62878 localpath.go` | FOUND (4 occurrences) |
| 7 TestLocalPathCVE_* tests pass | PASS |

## Known Stubs

None — the check is fully wired. `realGetProvisionerVersion` performs actual runtime detection and container exec. Tests inject a fake version function to avoid requiring a live Docker daemon.

## Threat Flags

None — this plan adds a read-only diagnostic check. It executes `kubectl get` inside an existing container; no new write paths, auth paths, or schema changes introduced.

## Self-Check: PASSED

- `pkg/internal/doctor/localpath.go` — FOUND
- `pkg/internal/doctor/localpath_test.go` — FOUND
- `pkg/internal/doctor/check.go` contains `newLocalPathCVECheck` — FOUND
- Commit `afb95837` (feat(44-03)) — FOUND
- Commit `ca641be0` (test(44-03)) — FOUND
