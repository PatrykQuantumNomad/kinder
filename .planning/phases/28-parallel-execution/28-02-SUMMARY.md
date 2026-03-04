---
phase: 28-parallel-execution
plan: "02"
subsystem: infra
tags: [concurrency, errgroup, wave-execution, parallel, race-free, timing]
dependency_graph:
  requires:
    - phase: 28-01
      provides: race-free-nodes-cache via sync.OnceValues, golang.org/x/sync v0.19.0 in go.mod
  provides:
    - wave-based-parallel-addon-execution
    - per-addon-install-duration-in-summary
    - addon-dependency-DAG-docs
    - Makefile-test-race-target
  affects:
    - pkg/cluster/internal/create/create.go
    - pkg/cluster/internal/create/create_addon_test.go
    - Makefile
tech-stack:
  added: []
  patterns:
    - "errgroup.WithContext + SetLimit(3) for bounded parallel goroutine dispatch"
    - "per-goroutine no-op Status to avoid cli.Status concurrency race"
    - "pre-allocated result slice by index for deterministic ordering from concurrent goroutines"
    - "warn-and-continue: goroutine returns nil to errgroup, error captured in addonResult"
    - "wave boundary: explicit errgroup.Wait() separates Wave 1 from Wave 2"
key-files:
  created: []
  modified:
    - pkg/cluster/internal/create/create.go
    - pkg/cluster/internal/create/create_addon_test.go
    - Makefile
key-decisions:
  - "Wave 1 (6 addons) run concurrently via errgroup with SetLimit(3); Wave 2 (EnvoyGateway) runs sequentially after g.Wait()"
  - "parallelActionContext() creates per-goroutine ActionContext with no-op Status to avoid cli.Status.status write race"
  - "wave1Results pre-allocated by index (not append) to guarantee deterministic summary ordering regardless of goroutine scheduling"
  - "runAddonTimed is package-level (not closure) for direct test access and clear separation of concerns"
  - "Addon dependency DAG documented in source comments with explicit wave boundary rationale"
metrics:
  duration: "~15 minutes"
  completed: "2026-03-03"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 3
---

# Phase 28 Plan 02: Wave-Based Parallel Addon Execution Summary

**errgroup.WithContext + SetLimit(3) dispatches 6 independent Wave 1 addons concurrently; EnvoyGateway runs in Wave 2 after g.Wait(); per-addon timing printed in post-creation summary**

## Performance

- **Duration:** ~15 minutes
- **Started:** 2026-03-03T00:00:00Z
- **Completed:** 2026-03-03
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Wave 1 (Local Registry, MetalLB, Metrics Server, CoreDNS Tuning, Dashboard, Cert Manager) run concurrently with errgroup.WithContext + SetLimit(3), reducing cluster creation time
- Wave 2 (EnvoyGateway) runs sequentially after Wave 1 completes via explicit g.Wait() boundary
- Addon dependency DAG documented in source code comments with wave boundaries and rationale
- addonResult.duration field records install time via time.Now()/time.Since(); logAddonSummary prints "X.Ys" for installed/failed addons
- cli.Status race eliminated via parallelActionContext() creating per-goroutine no-op Status for Wave 1 goroutines
- Makefile test-race target added for CGO_ENABLED=1 go test -race

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement wave-based parallel addon execution with DAG docs and timing** - `ce7cf1ec` (feat)
2. **Task 2: Update addon tests for duration field and add Makefile test-race target** - `c2a57161` (test)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `pkg/cluster/internal/create/create.go` - Added errgroup+sync imports, duration field on addonResult, runAddonTimed(), parallelActionContext(), wave1/wave2 slices replacing addonRegistry, DAG comments, updated logAddonSummary with timing
- `pkg/cluster/internal/create/create_addon_test.go` - Added time/context/actions/config/cli imports, updated existing test cases with duration assertions, 3 new test cases, TestRunAddonTimed_DisabledSkips
- `Makefile` - Added test-race target (CGO_ENABLED=1 go test -race), updated .PHONY line

## Decisions Made

- **errgroup.WithContext with SetLimit(3):** Bounds concurrency to avoid overwhelming the Kubernetes API or node containers. Three concurrent addon installs is a reasonable limit balancing parallelism vs. resource contention.
- **parallelActionContext with no-op Status:** The cli.Status struct is not concurrent-safe (Status.status string accessed without locks). Creating per-goroutine contexts with no-op Status avoids the race without requiring mutex-wrapping the Status type.
- **Index-based result assignment (not append):** `wave1Results[i] = res` under mutex guarantees deterministic ordering. Using append under a mutex would still work but the code expresses intent more clearly with pre-allocation.
- **Wave 2 uses original actionsContext:** EnvoyGateway runs sequentially so the real Status (with spinner) is safe to use; it also runs after Wave 1 so the Nodes() cache is already warm.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - the implementation compiled and passed all tests on the first attempt.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 28 is now complete (both plans done)
- Wave-based parallel execution is in production-ready shape with no data races
- The test-race Makefile target enables ongoing race detection in CI
- Phase 29 (final phase) can proceed

## Self-Check: PASSED

All files verified:
- FOUND: pkg/cluster/internal/create/create.go
- FOUND: pkg/cluster/internal/create/create_addon_test.go
- FOUND: Makefile
- FOUND: .planning/phases/28-parallel-execution/28-02-SUMMARY.md

All commits verified:
- FOUND: ce7cf1ec (feat(28-02): implement wave-based parallel addon execution with DAG docs and timing)
- FOUND: c2a57161 (test(28-02): update addon tests for duration field and add Makefile test-race target)

---
*Phase: 28-parallel-execution*
*Completed: 2026-03-03*
