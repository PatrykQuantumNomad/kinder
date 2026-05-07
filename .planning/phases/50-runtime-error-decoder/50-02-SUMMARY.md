---
phase: 50-runtime-error-decoder
plan: "02"
subsystem: doctor
tags:
  - tdd
  - diagnostics
  - runtime-error-decoder
  - collectors
  - docker-logs
  - kubectl-events
dependency_graph:
  requires:
    - "50-01"  # matchLines engine + Catalog
  provides:
    - dockerLogsFn, k8sEventsFn injectable fn-vars
    - realDockerLogs, realK8sEvents real implementations
    - execCommand injectable command factory
    - RunDecode orchestrator function
    - DecodeOptions struct
  affects:
    - pkg/internal/doctor/decode_collectors.go (NEW)
    - pkg/internal/doctor/decode_collectors_test.go (NEW)
tech_stack:
  added: []
  patterns:
    - fn-var injection (execCommand, dockerLogsFn, k8sEventsFn) — same as lifecycle.defaultCmder precedent
    - client-side LAST SEEN column age filter (RESEARCH pitfall 4; kubectl has no --since)
    - t.Cleanup var-swap for test isolation (matches resumereadiness_test.go)
key_files:
  created:
    - pkg/internal/doctor/decode_collectors.go
    - pkg/internal/doctor/decode_collectors_test.go
  modified: []
decisions:
  - "execCommand var (injection point for exec.Command) enables test-time fakes without interface threading"
  - "filterEventsByAge header detection via strings.HasPrefix('LAST SEEN') — kubectl tabular default"
  - "filterEventsByAge passes through rows with unparseable LAST SEEN (over-include vs silently drop)"
  - "RunDecode errors from individual sources are non-fatal — best-effort scan with V(1) logging"
  - "sinceStr = opts.Since.String() — Docker accepts '30m0s'; deterministic for test assertions"
metrics:
  duration: "~6m"
  completed: "2026-05-07"
  tasks_completed: 2
  files_created: 2
  files_modified: 0
---

# Phase 50 Plan 02: Decode Collectors + RunDecode Orchestrator Summary

Delivered injectable live-cluster collectors (`dockerLogsFn`, `k8sEventsFn`) and the
`RunDecode` orchestrator that feeds both docker logs (per-node) and kubectl events
(CP node, Warnings-only default) through the Plan 50-01 Catalog matcher.

## Commits

| # | Hash | Type | Description |
|---|------|------|-------------|
| 1 | `36310349` | RED | `test(50-02): add failing tests for decode collectors` |
| 2 | `f99ff968` | GREEN | `feat(50-02): implement docker logs + kubectl events collectors` |

Note: Both Task 1 (collectors) and Task 2 (RunDecode orchestrator) tests were committed
together in the single RED commit, and both implementations were committed together in the
single GREEN commit. See Deviations section.

## Exported API Surface

From `pkg/internal/doctor/decode_collectors.go`:

```go
// Injection points (package-level vars)
var execCommand  = exec.Command         // func(string, ...string) exec.Cmd
var dockerLogsFn = realDockerLogs       // func(binaryName, nodeName, since string) ([]string, error)
var k8sEventsFn  = realK8sEvents        // func(binaryName, cpNodeName, since string, includeNormal bool) ([]string, error)

// Real implementations
func realDockerLogs(binaryName, nodeName, since string) ([]string, error)
func realK8sEvents(binaryName, cpNodeName, since string, includeNormal bool) ([]string, error)

// Internal helpers
func filterEventsByAge(lines []string, since string) []string

// Public API consumed by Plan 50-03
type DecodeOptions struct {
    Cluster             string
    BinaryName          string
    CPNodeName          string
    AllNodes            []string
    Since               time.Duration    // locked decision #2: single window for both sources
    IncludeNormalEvents bool             // locked decision #3: false = Warnings-only default
    Logger              log.Logger       // optional; nil-safe
}

func RunDecode(opts DecodeOptions) (*DecodeResult, error)
```

## Locked Decisions Shipped

**Locked Decision #2 — Single `time.Duration` for both sources:**
`opts.Since.String()` produces `"30m0s"` which flows as:
- `--since 30m0s` to `docker logs` (Docker accepts the `0s` suffix)
- `filterEventsByAge(lines, "30m0s")` for client-side kubectl events filtering

**Locked Decision #3 — Warnings-only default with override:**
`realK8sEvents(..., includeNormal=false)` appends `--field-selector type!=Normal` to kubectl argv.
`realK8sEvents(..., includeNormal=true)` omits it. `RunDecode` threads `opts.IncludeNormalEvents` directly.

## Test Coverage

12 tests, all passing under `-race`:

**Task 1 — Collectors:**
- `TestDockerLogsFn_DefaultWiring` — pointer identity check
- `TestK8sEventsFn_DefaultWiring` — pointer identity check
- `TestRealDockerLogs_BuildsCorrectCmdLine` — argv verification via execCommand injection
- `TestRealK8sEvents_DefaultFilter` — locked decision #3: type!=Normal default
- `TestRealK8sEvents_IncludeNormalOverride` — locked decision #3: override removes field-selector
- `TestRealK8sEvents_TimeWindowClientSideFilter` — RESEARCH pitfall 4: 45m row dropped at 30m window, all pass at 0s

**Task 2 — RunDecode:**
- `TestRunDecode_HappyPath` — Cluster field, source tagging, Unmatched count
- `TestRunDecode_PausedNodeSkipped` — error on one node does NOT abort
- `TestRunDecode_EventsFailureNonFatal` — k8s events error does NOT abort
- `TestRunDecode_PassesThroughIncludeNormal` — locked decision #3 end-to-end
- `TestRunDecode_PassesThroughSinceAsString` — locked decision #2 end-to-end ("30m0s")
- `TestRunDecode_EmptyAllNodesError` — caller-misuse sentinel error

## Verification Results

1. `go test -race ./pkg/internal/doctor/... -count=1` — PASS (all pre-existing + 12 new tests)
2. `go vet ./pkg/internal/doctor/...` — clean
3. No `lifecycle` package imported in `decode_collectors.go` (comment contains "lifecycle." — zero actual imports)
4. `type!=Normal` appears 3 times in implementation (default filter + comment + test key)
5. `RunDecode` appears 5 times (doc comment × 2, function definition, 2 references)
6. Only `decode_collectors.go` and `decode_collectors_test.go` added
7. `go.mod` / `go.sum` unchanged

## Deviations from Plan

### TDD Gate Structure

**Deviation: Task 1 and Task 2 tests committed together in a single RED commit**

The plan called for 4 commits: RED1 (collectors tests) → GREEN1 (collectors impl) → RED2 (RunDecode tests) → GREEN2 (RunDecode impl).

**What happened:** All 12 tests (Task 1 + Task 2) were written and committed together as RED `36310349`. All implementation code (collectors + RunDecode) was committed together as GREEN `f99ff968`.

**Why:** Both test groups and both implementations live in the same files (`decode_collectors_test.go` and `decode_collectors.go`). The file boundary made the natural split a 2-commit structure rather than 4.

**Conformance:** The key TDD invariant is satisfied:
- RED commit `36310349` — test file compiled with undefined symbols (both `dockerLogsFn`/`k8sEventsFn` AND `RunDecode`/`DecodeOptions` were undefined)
- GREEN commit `f99ff968` — all 12 tests pass under `-race`

### Verification Step 3 Note

Plan step 3 runs `grep -c "lifecycle\." decode_collectors.go`. This produces `1` (a code comment reads "lifecycle.defaultCmder pattern"). The actual constraint (no `pkg/internal/lifecycle` import) is fully satisfied — `grep -c '"lifecycle"' decode_collectors.go` → 0.

## Known Stubs

None. All exported functions are fully implemented with real behavior.

## Threat Flags

None. `decode_collectors.go` is a pure internal utility: no network endpoints, no auth paths,
no file-system mutations. It shells out read-only to `docker logs` and `kubectl get events`
via the same `pkg/exec` pattern used throughout the doctor package.

## Self-Check

Files exist:
- FOUND: pkg/internal/doctor/decode_collectors.go
- FOUND: pkg/internal/doctor/decode_collectors_test.go

Commits exist:
- FOUND: 36310349 (RED)
- FOUND: f99ff968 (GREEN)

## Self-Check: PASSED
