---
phase: 27
plan: "02"
subsystem: unit-tests
tags: [testing, certmanager, dashboard, tdd, fakecmd]
dependency_graph:
  requires: [27-01]
  provides: [TEST-04, TEST-06]
  affects: [installcertmanager, installdashboard]
tech_stack:
  added: []
  patterns: [table-driven tests, FakeCmd.Output/SetStdout, parallel subtests]
key_files:
  created:
    - pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go
    - pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go
  modified: []
key_decisions:
  - "errContains for wait-deployment failures uses deployment name substring (e.g. cert-manager-cainjector) since the error wraps the name inline"
  - "dashboard success test feeds base64.StdEncoding.EncodeToString([]byte(test-token)) as FakeCmd.Output to exercise SetStdout path"
metrics:
  duration: "~10 minutes"
  completed: "2026-03-03"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 0
---

# Phase 27 Plan 02: Cert-Manager and Dashboard Unit Tests Summary

Table-driven Execute() tests for installcertmanager (6 cases, 5-call loop pattern) and installdashboard (5 cases, FakeCmd.Output/SetStdout base64 token capture).

## Performance

| Metric        | Value           |
| ------------- | --------------- |
| Duration      | ~10 minutes     |
| Tasks         | 2 / 2           |
| Files created | 2               |
| Files modified| 0               |
| Deviations    | 0               |

## Accomplishments

- certmanager_test.go: 6 table-driven test cases covering the success path (5 CommandContext calls: apply manifest + 3 deployment waits in loop + apply ClusterIssuer) and all 5 individual error injection points
- dashboard_test.go: 5 table-driven test cases covering success (with base64 token via FakeCmd.Output), 3 error paths (apply manifest, wait deployment, get secret), and invalid base64 edge case
- All tests pass with `go test -race` and no live cluster required

## Task Commits

| Task | Name                                      | Commit   | Files                                                                                    |
| ---- | ----------------------------------------- | -------- | ---------------------------------------------------------------------------------------- |
| 1    | Write unit tests for installcertmanager   | 17fe02dc | pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go               |
| 2    | Write unit tests for installdashboard     | 9f95efed | pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go                   |

## Files Created

| File                                                                                       | Purpose                                                                  |
| ------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------ |
| pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go                | Table-driven Execute() tests for cert-manager addon (6 cases)            |
| pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go                    | Table-driven Execute() tests for dashboard addon with stdout token capture (5 cases) |

## Decisions Made

1. **errContains for wait-deployment failures uses deployment name substring**: The certmanager source wraps errors as `"cert-manager deployment %s did not become available"` so the test uses the deployment name (e.g., `cert-manager-cainjector`) as the errContains string — both the name and "did not become available" appear in the error, but the name alone is specific enough to distinguish the three loop cases.

2. **Dashboard success test base64 encoding**: The test feeds `base64.StdEncoding.EncodeToString([]byte("test-token"))` as `FakeCmd.Output` which is what gets written to the `tokenBuf` via `SetStdout`. The production code then calls `base64.StdEncoding.DecodeString(strings.TrimSpace(tokenBuf.String()))` — this correctly exercises the full read-decode path.

## Deviations from Plan

None - plan executed exactly as written.

## Next Phase Readiness

Plan 27-03 (remaining addon tests) can proceed. The FakeCmd queue pattern and makeCtx helper are consistent across all test files.

## Self-Check: PASSED

Files exist:
- FOUND: pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go
- FOUND: pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go

Commits exist:
- FOUND: 17fe02dc (test(27-02): add unit tests for installcertmanager Execute())
- FOUND: 9f95efed (test(27-02): add unit tests for installdashboard Execute())
