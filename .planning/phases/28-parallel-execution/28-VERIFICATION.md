---
phase: 28-parallel-execution
verified: 2026-03-03T00:00:00Z
status: passed
score: 4/4 must-haves verified
re_verification: false
---

# Phase 28: Parallel Addon Execution Verification Report

**Phase Goal:** Independent addons install concurrently in waves during cluster creation, reducing total creation time, with the Nodes() cache race fixed and per-addon durations printed in the summary
**Verified:** 2026-03-03
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth                                                                                           | Status     | Evidence                                                                                                |
| --- | ----------------------------------------------------------------------------------------------- | ---------- | ------------------------------------------------------------------------------------------------------- |
| 1   | Addon dependency DAG documented in source with MetalLB before EnvoyGateway; wave comments present | VERIFIED | create.go lines 239-246: multi-line DAG comment with Wave 1 (6 addons) / Wave 2 (EnvoyGateway) rationale; wave boundary noted as explicit errgroup.Wait() |
| 2   | ActionContext.Nodes() uses sync.OnceValues; go test -race is clean                               | VERIFIED | action.go: cachedData removed (grep count=0), nodesOnce field of type func()([]nodes.Node,error) initialized via sync.OnceValues; CGO_ENABLED=1 go test -race ./... PASS, no races |
| 3   | Independent addons run via errgroup.WithContext + SetLimit(3) in Wave 1; Wave 2 runs sequentially after | VERIFIED | create.go lines 266-293: errgroup.WithContext, g.SetLimit(3), 6-goroutine Wave 1 loop, explicit g.Wait() before Wave 2 sequential loop |
| 4   | Post-creation summary prints per-addon install duration                                          | VERIFIED | create.go logAddonSummary (lines 410-423): "installed (%.1fs)" and "FAILED: %v (%.1fs)" format strings; addonResult.duration field populated by runAddonTimed via time.Since(start) |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact                                                                      | Expected                                              | Status   | Details                                                                                      |
| ----------------------------------------------------------------------------- | ----------------------------------------------------- | -------- | -------------------------------------------------------------------------------------------- |
| `pkg/cluster/internal/create/actions/action.go`                               | Race-free Nodes() cache using sync.OnceValues          | VERIFIED | nodesOnce field present; sync.OnceValues used (3 occurrences); cachedData fully removed (0 occurrences); Nodes() is a one-liner `return ac.nodesOnce()` |
| `pkg/cluster/internal/create/actions/action_test.go`                          | Concurrent race test for Nodes()                      | VERIFIED | TestNodes_ConcurrentAccess (10 goroutines) and TestNodes_CachesResult both present and passing |
| `go.mod`                                                                      | golang.org/x/sync dependency                          | VERIFIED | `golang.org/x/sync v0.19.0` in direct require block                                         |
| `pkg/cluster/internal/create/create.go`                                       | Wave-based parallel addon execution with errgroup, timing, DAG docs | VERIFIED | errgroup.WithContext, g.SetLimit(3), wave1/wave2 slices, DAG comments, duration field, logAddonSummary with "%.1fs" formatting |
| `pkg/cluster/internal/create/create_addon_test.go`                            | Updated tests for duration field in addonResult       | VERIFIED | 8 test cases in TestLogAddonSummary including "installed 3.0s", "FAILED 2.0s", "12.3s", "5.7s"; TestRunAddonTimed_DisabledSkips present |
| `Makefile`                                                                    | test-race target for CGO_ENABLED=1 race testing       | VERIFIED | test-race target at line 88-89; in .PHONY at line 118                                       |

### Key Link Verification

| From                                              | To                          | Via                                                | Status   | Details                                                                      |
| ------------------------------------------------- | --------------------------- | -------------------------------------------------- | -------- | ---------------------------------------------------------------------------- |
| `pkg/cluster/internal/create/create.go`           | `golang.org/x/sync/errgroup` | errgroup.WithContext + SetLimit(3)                 | WIRED    | Line 29 import; line 266 `errgroup.WithContext`; line 267 `g.SetLimit(3)`   |
| `pkg/cluster/internal/create/create.go`           | `logAddonSummary`           | addonResult.duration printed as seconds in summary | WIRED    | Line 301 `logAddonSummary(logger, addonResults)`; lines 418/420 `duration.Seconds()` |
| `pkg/cluster/internal/create/actions/action.go`   | `sync.OnceValues`           | nodesOnce field replaces cachedData struct         | WIRED    | Line 46 field declaration; line 63 `sync.OnceValues(...)` in NewActionContext; line 72 `return ac.nodesOnce()` |
| `pkg/cluster/internal/create/actions/action_test.go` | `action.go`              | 10 concurrent goroutines calling Nodes()           | WIRED    | Lines 40-53: for range 10 loop with goroutines each calling ctx.Nodes()     |

### Requirements Coverage

| Requirement | Source Plan | Description                              | Status    | Evidence                                                                                   |
| ----------- | ----------- | ---------------------------------------- | --------- | ------------------------------------------------------------------------------------------ |
| PARA-01     | 28-02       | Dependency DAG documented                | SATISFIED | create.go lines 239-246: DAG comment block; MetalLB->EnvoyGateway dependency explicitly stated; wave boundaries in code comments |
| PARA-02     | 28-01       | Race-free Nodes() cache                  | SATISFIED | sync.OnceValues replaces cachedData; `CGO_ENABLED=1 go test -race ./...` clean; TestNodes_ConcurrentAccess passes under race detector |
| PARA-03     | 28-02       | Parallel addon execution via errgroup    | SATISFIED | errgroup.WithContext + g.SetLimit(3) + 6 goroutines in Wave 1; g.Wait() boundary before Wave 2 |
| PARA-04     | 28-02       | Per-addon timing in summary              | SATISFIED | addonResult.duration field; runAddonTimed records time.Since(start); logAddonSummary prints "installed (%.1fs)" / "FAILED: ... (%.1fs)" |

### Anti-Patterns Found

| File                                                            | Line | Pattern                       | Severity | Impact                                                       |
| --------------------------------------------------------------- | ---- | ----------------------------- | -------- | ------------------------------------------------------------ |
| `pkg/cluster/internal/create/create.go`                         | 179  | TODO(bentheelder)             | Info     | Pre-existing TODO from upstream kind codebase; not introduced by this phase; no impact on goal |
| `pkg/cluster/internal/create/create.go`                         | 219  | TODO: factor out              | Info     | Pre-existing TODO from upstream kind codebase; not introduced by this phase; no impact on goal |
| `pkg/cluster/internal/create/create.go`                         | 369  | TODO(fabrizio pandini)        | Info     | Pre-existing TODO from upstream kind codebase; not introduced by this phase; no impact on goal |

No blockers or warnings. All TODOs are pre-existing upstream comments with named authors, not introduced by this phase and not affecting correctness.

### Human Verification Required

None. All success criteria are verifiable programmatically.

### Gaps Summary

No gaps. All four success criteria from the ROADMAP.md are met:

1. **PARA-01 (DAG documented):** The addon dependency DAG is present in source code comments in create.go (lines 239-246), naming all Wave 1 addons as independent and stating EnvoyGateway's MetalLB dependency explicitly. Wave boundaries are visible in code comments at lines 265, 289, and the errgroup.Wait() call at line 281.

2. **PARA-02 (Race-free Nodes()):** ActionContext.Nodes() uses sync.OnceValues (the cachedData struct and its RWMutex are fully removed). TestNodes_ConcurrentAccess verifies 10 goroutines can call Nodes() concurrently. `CGO_ENABLED=1 go test -race ./pkg/cluster/internal/create/... -count=1` passes cleanly with no data races detected.

3. **PARA-03 (Parallel via errgroup SetLimit(3)):** Wave 1 dispatches 6 addon goroutines via errgroup.WithContext + g.SetLimit(3). Wave 2 runs EnvoyGateway sequentially after g.Wait(). The parallelActionContext() helper creates per-goroutine ActionContexts with no-op Status to prevent cli.Status.status write races.

4. **PARA-04 (Per-addon timing in summary):** addonResult.duration is recorded by runAddonTimed() using time.Now()/time.Since(). logAddonSummary() prints "installed (X.Ys)" for successful addons and "FAILED: err (X.Ys)" for failed addons. Tests in create_addon_test.go verify "3.0s", "2.0s", "12.3s", and "5.7s" appear in output.

---

_Verified: 2026-03-03T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
