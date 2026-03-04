---
phase: "27"
plan: "03"
subsystem: unit-tests
tags: [testing, localregistry, docker-skip, tdd]
requires: [27-01]
provides: [TEST-03]
affects: [pkg/cluster/internal/create/actions/installlocalregistry]
tech-stack:
  added: []
  patterns: [docker-skip-guard, fake-node, table-driven-tests]
key-files:
  created:
    - pkg/cluster/internal/create/actions/installlocalregistry/localregistry_test.go
  modified: []
key-decisions:
  - "TestExecute_InfoError always runs without Docker because Provider.Info() is called before any exec.Command invocations"
  - "Docker-dependent tests use t.Skip('requires Docker daemon') via exec.Command('docker','version').Run() guard"
  - "TestExecute_NodePatchingErrors sub-tests are sequential (not parallel) to avoid concurrent mutation of the shared kind-registry container"
  - "FakeProvider intentionally does not implement fmt.Stringer so binaryName defaults to 'docker', matching the Docker skip guard"
duration: "~10 minutes"
completed: "2026-03-03"
---

# Phase 27 Plan 03: Local Registry Unit Tests Summary

Unit tests for installlocalregistry Execute() with honest handling of the mixed host/node execution model: TestExecute_InfoError runs unconditionally; Docker-dependent tests skip cleanly when Docker is unavailable.

## Performance

- Duration: ~10 minutes
- Tasks completed: 1/1
- Files created: 1
- Files modified: 0
- Deviations: None

## Accomplishments

### Task 1: Write unit tests for installlocalregistry Execute()

Created `localregistry_test.go` with three test functions covering the three distinct execution paths:

1. **TestExecute_InfoError** (always runs) — Verifies that `Provider.Info()` failure propagates as `"failed to get provider info"`. This code path executes before any host-side `exec.Command` calls, so no Docker daemon is needed. Runs in parallel.

2. **TestExecute_FullPath** (Docker-guarded) — Tests the full Execute() flow including all four host-side Docker commands (inspect, run, network inspect, network connect) and verifies the FakeNode records at least 3 node-side calls (mkdir, tee, kubectl apply). Skips when Docker unavailable.

3. **TestExecute_NodePatchingErrors** (Docker-guarded, table-driven) — Tests that node-side errors propagate with descriptive messages. Two cases: mkdir fails ("failed to create certs.d dir") and tee fails ("failed to write hosts.toml"). Sequential sub-tests to avoid concurrent container mutation.

## Task Commits

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Unit tests for localregistry Execute() | ab3beb35 | localregistry_test.go |

## Files Created / Modified

**Created:**
- `pkg/cluster/internal/create/actions/installlocalregistry/localregistry_test.go` — 164 lines, 3 test functions

## Decisions Made

1. **TestExecute_InfoError always runs without Docker** — After reading localregistry.go, confirmed that `ctx.Provider.Info()` is called at line 73, before ANY `exec.Command(binaryName, ...)` calls. This means the Info error path is genuinely Docker-free and t.Skip is not needed.

2. **Docker skip guard uses `sigs.k8s.io/kind/pkg/exec`** — The project uses a custom exec package (not `os/exec`). The `dockerAvailable()` helper calls `exec.Command("docker", "version").Run()` using this package for consistency.

3. **Sequential sub-tests for NodePatchingErrors** — The host-side steps create/manage the `kind-registry` container. Running sub-tests in parallel would cause race conditions on the shared container. Sub-tests run sequentially; cleanup is in a single `t.Cleanup` on the parent.

4. **FakeProvider does not implement fmt.Stringer** — The Execute() method does `ctx.Provider.(fmt.Stringer)` to detect binary name. When the type assertion fails (as with FakeProvider), it defaults to "docker". This is correct behavior for tests that use the Docker skip guard.

## Deviations from Plan

None — plan executed exactly as written. The "honest approach" (TestExecute_InfoError always runs; Docker-dependent tests skip cleanly) was implemented as specified.

## Verification Results

```
=== RUN   TestExecute_InfoError
--- PASS: TestExecute_InfoError (0.00s)
=== RUN   TestExecute_FullPath
--- PASS: TestExecute_FullPath (0.34s)
=== RUN   TestExecute_NodePatchingErrors
    --- PASS: TestExecute_NodePatchingErrors/mkdir_fails_on_first_node (0.34s)
    --- PASS: TestExecute_NodePatchingErrors/tee_fails_on_first_node (0.05s)
--- PASS: TestExecute_NodePatchingErrors (0.67s)
PASS
ok  sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installlocalregistry 1.267s
```

Full actions suite with race detection:
```
go test -race ./pkg/cluster/internal/create/actions/... -count=1
# All packages pass; no data races reported
```

## Next Phase Readiness

- Phase 27 complete: all 3 plans done (27-01 testutil + metricsserver/envoygw, 27-02 certmanager/dashboard, 27-03 localregistry)
- Phase 28 (Parallel Execution) is unblocked; depends on Phase 26 + Phase 27
- Blocker noted: validate MetalLB/EnvoyGateway runtime dependency before parallelizing (from Phase 28 entry in STATE.md)

## Self-Check: PASSED

- [x] `pkg/cluster/internal/create/actions/installlocalregistry/localregistry_test.go` exists
- [x] Commit ab3beb35 exists in git log
- [x] `go test ./pkg/cluster/internal/create/actions/installlocalregistry/ -count=1 -v` passes
- [x] `go test -race ./pkg/cluster/internal/create/actions/... -count=1` passes
- [x] `unset KUBECONFIG && go test ./pkg/cluster/internal/create/actions/... -count=1` passes
