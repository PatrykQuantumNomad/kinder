---
phase: 27-unit-tests
plan: "01"
subsystem: testing
tags: [unit-tests, testutil, fakes, tdd, metricsserver, envoygw]
requires: []
provides: [testutil-package, metricsserver-tests, envoygw-tests]
affects: [installmetricsserver, installenvoygw, future-addon-tests]
tech-stack:
  added: [testutil package]
  patterns: [table-driven tests, TDD red-green, FakeNode/FakeCmd pattern]
key-files:
  created:
    - pkg/cluster/internal/create/actions/testutil/fake.go
    - pkg/cluster/internal/create/actions/testutil/fake_test.go
    - pkg/cluster/internal/create/actions/installmetricsserver/metricsserver_test.go
    - pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go
  modified: []
key-decisions:
  - FakeNode.nextCmd returns &FakeCmd{} (not nil) when queue exhausted so callers can always call .Run() safely
  - FakeProvider implements full providers.Provider interface with no-op stubs for unused methods
  - NewTestContext helper uses NoopLogger with StatusForLogger to avoid spinner setup complexity
  - CommandContext accepts nil context pointer since FakeNode ignores it (test-only usage)
duration: ~2 minutes
completed: 2026-03-04
---

# Phase 27 Plan 01: Testutil and Addon Tests Summary

Shared test infrastructure (FakeNode, FakeCmd, FakeProvider) plus unit tests for installmetricsserver (2 CommandContext calls) and installenvoygw (5 CommandContext calls) — all tests pass with -race, no live cluster required.

## Performance

- Tasks completed: 2 / 2
- Duration: ~2 minutes
- Test cases added: 16 (7 testutil self-tests + 3 metricsserver + 6 envoygw)
- Files created: 4

## Accomplishments

- Created `pkg/cluster/internal/create/actions/testutil/` package with three types
  - `FakeCmd` implements `exec.Cmd`: writes `Output` to stdout on `Run()`, returns `Err`
  - `FakeNode` implements `nodes.Node`: returns queued `FakeCmds` in order, records all `CommandContext` calls in `Calls [][]string`
  - `FakeProvider` implements `providers.Provider`: returns configured `Nodes` and `Info`
  - `NewFakeControlPlane` constructor sets role to `constants.ControlPlaneNodeRoleValue`
  - `NewTestContext` helper builds `ActionContext` with `NoopLogger` and `context.Background()`
- Written 7 self-tests in `fake_test.go` covering all FakeNode and FakeCmd behaviors
- Written 3 table-driven tests in `metricsserver_test.go`: success, apply-fails, wait-fails
- Written 6 table-driven tests in `envoygw_test.go`: success + one failure per CommandContext call (5 steps)
- All tests run with `-race` with no data races detected

## Task Commits

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 (RED) | Failing testutil tests | 3cbddc12 | testutil/fake_test.go |
| 1 (GREEN) | testutil implementation | 7903bd02 | testutil/fake.go |
| 2 | metricsserver + envoygw tests | cea007e5 | metricsserver_test.go, envoygw_test.go |

## Files Created

| File | Purpose |
|------|---------|
| `pkg/cluster/internal/create/actions/testutil/fake.go` | FakeNode, FakeCmd, FakeProvider, NewFakeControlPlane, NewTestContext |
| `pkg/cluster/internal/create/actions/testutil/fake_test.go` | Self-tests verifying fake behavior |
| `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver_test.go` | 3 table-driven Execute() tests |
| `pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go` | 6 table-driven Execute() tests |

## Decisions Made

1. **FakeNode returns non-nil default on queue exhaustion** — `nextCmd()` returns `&FakeCmd{}` (nil error, no output) rather than `nil` so callers can always safely chain `.SetStdin().Run()` without panicking.
2. **CommandContext accepts nil context** — Since FakeNode ignores the context argument entirely, passing `nil` from tests is safe and avoids boilerplate `context.Background()` at every call site.
3. **NewTestContext uses NoopLogger** — Avoids any terminal/spinner detection logic; `StatusForLogger(NoopLogger{})` creates a plain Status that logs to nowhere, suitable for unit tests.
4. **FakeProvider stubs all methods** — Full `providers.Provider` interface implementation with no-op stubs ensures the type satisfies the interface without requiring test setup for unused methods.

## Deviations from Plan

None - plan executed exactly as written.

## Next Phase Readiness

The testutil package is ready to be consumed by all remaining addon test plans (27-02 through 27-07). Each subsequent plan can:
- Import `sigs.k8s.io/kind/pkg/cluster/internal/create/actions/testutil`
- Use `testutil.NewFakeControlPlane(name, cmds)` to create a wired node
- Use `testutil.FakeProvider{Nodes: ..., InfoResp: ...}` to wrap the node
- Use `testutil.NewTestContext(provider)` to get an ActionContext

## Self-Check

- [x] testutil/fake.go exists and compiles
- [x] testutil/fake_test.go exists and passes
- [x] metricsserver_test.go exists and passes (3 test cases)
- [x] envoygw_test.go exists and passes (6 test cases)
- [x] Commits 3cbddc12, 7903bd02, cea007e5 all exist
- [x] `go test ./pkg/cluster/internal/create/actions/... -count=1` all pass
- [x] No data races with -race flag

## Self-Check: PASSED
