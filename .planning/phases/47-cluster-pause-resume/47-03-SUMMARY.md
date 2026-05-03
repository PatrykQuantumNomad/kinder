---
phase: 47-cluster-pause-resume
plan: 03
subsystem: cluster-lifecycle
tags: [cobra, docker-start, kubectl, k8s-1.24-fallback, readiness-gate, json-output, tdd]

# Dependency graph
requires:
  - phase: 47-01
    provides: lifecycle.ContainerState, lifecycle.ClusterStatus, lifecycle.ClassifyNodes, lifecycle.ProviderBinaryName, lifecycle.ResolveClusterName, defaultCmder, NodeResult (via plan 47-02 pause.go), nodeFetcher (via plan 47-02 pause.go), pkg/cmd/kind/resume scaffold
  - phase: 47-02
    provides: NodeResult struct, nodeFetcher interface (both shared with Resume in same package)
provides:
  - lifecycle.Resume (orchestration entry point) and lifecycle.WaitForNodesReady (reusable readiness probe)
  - kinder resume [name] command with --timeout, --wait, --json flags wired to lifecycle.Resume
  - Test injection points resumeFn (cmd) + defaultReadinessProber (lifecycle) for unit tests without Docker
  - Quorum-safe start ordering (LB → CP → workers, reverse of pause)
  - Best-effort failure semantics (continues on per-node start failure, aggregated error at end)
  - Idempotency for already-running clusters (warning + exit 0; readiness probe skipped)
affects: [47-04-PLAN doctor readiness check, 47-05-PLAN docs]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Package-level ReadinessProber injection (defaultReadinessProber) for testable readiness gate without spinning K8s
    - Test-injection function-vars in cmd package (resumeFn, resolveClusterName) so the command's runE can be unit-tested without a real cluster
    - Best-effort start ordering with aggregated error reporting (matches plan 47-02 pause's best-effort stop pattern)
    - Readiness probe queries ALL nodes (no --selector) — diverges from create's waitforready which only watches control-plane

key-files:
  created:
    - pkg/internal/lifecycle/resume.go
    - pkg/internal/lifecycle/resume_test.go
    - pkg/cmd/kind/resume/resume_test.go
  modified:
    - pkg/cmd/kind/resume/resume.go

key-decisions:
  - "Readiness probe queries ALL nodes (kubectl get nodes without --selector); unlike create's waitforready which uses --selector=control-plane because workers may not exist yet during create. For resume, every container exists and every node must report Ready."
  - "K8s 1.24 selector fallback (control-plane vs master label) preserved in WaitForNodesReady for completeness even though Resume doesn't use a selector — keeps the helper reusable by future callers that DO want to filter by role."
  - "Skip readiness probe when any container start failed: no point waiting for a known-incomplete cluster; aggregated start errors are returned directly."
  - "Idempotent fast-path (cluster already Running) emits a Warn log line and returns AlreadyRunning=true; zero docker-start calls, zero readiness probes, exit 0."
  - "Define ReadinessProber as a package-level var (defaultReadinessProber) instead of an opts field to keep ResumeOptions stable; future callers (plan 47-04 doctor) can swap globally during their inline check."
  - "NodeResult and nodeFetcher are shared with pause.go (same package); resume.go references them by name without redeclaration. Wave-2 ordering: pause.go landed first via plan 47-02, so my resume.go imports the existing types directly."
  - "Per-node text output emitted by lifecycle.Resume via opts.Logger.V(0).Infof; the cmd-level runE only emits the final 'Cluster resumed. Total time: X.Xs' line — no duplication."

patterns-established:
  - "Pattern: Wave-2 plan files share types defined in the same package (NodeResult, nodeFetcher). The plan that lands first owns the declaration; later plans reference by name."
  - "Pattern: Readiness gates are injectable via package-level function vars so unit tests don't need a real Kubernetes API server."

# Metrics
duration: ~25m (TDD cycles for both tasks; near-zero deviations because plan 47-01/47-02 had cleared the path)
completed: 2026-05-03
---

# Phase 47 Plan 03: kinder resume Summary

**Quorum-safe resume body with readiness gate, idempotency, best-effort failure handling, and structured JSON output — completes the user-facing pause/resume pair (LIFE-02 and resume-side LIFE-03).**

## Performance

- **Duration:** ~25m elapsed (mostly TDD cycles; one minor compile-only blocker resolved)
- **Started:** 2026-05-03T19:55:00Z (approx, when this executor began)
- **Completed:** 2026-05-03T20:02:51Z
- **Tasks:** 2
- **Files modified:** 4 (3 created, 1 modified)

## Accomplishments

- New `lifecycle.Resume` orchestration: start LB → CP → workers, best-effort sequential, aggregated errors.
- New `lifecycle.WaitForNodesReady` reusable readiness gate (queries ALL nodes, K8s 1.24 selector fallback preserved for callers that want it).
- Idempotent no-op when cluster already running (warning + exit 0; zero docker-start, zero readiness probes).
- Skip readiness probe when any start failed (no point waiting for a known-incomplete cluster).
- `kinder resume` command body wired (replaced "not yet implemented" stub from plan 47-01).
- Flags: `--timeout` (graceful start, default 30s), `--wait` (readiness gate, default 300s), `--json` (structured output).
- Negative flag values rejected with clear error messages before any orchestration runs.
- 14 lifecycle.Resume unit tests + 10 kinder-resume cmd tests, all passing.
- Zero new module dependencies (go.mod and go.sum untouched).

## Task Commits

Each task was committed atomically (RED → GREEN TDD):

1. **Task 1 RED: lifecycle.Resume failing tests** — `cc6fa301` (test)
2. **Task 1 GREEN: lifecycle.Resume implementation** — `50c686aa` (feat)
3. **Task 2 RED: kinder resume cmd failing tests** — `87d55f21` (test)
4. **Task 2 GREEN: kinder resume RunE implementation** — `3d8cf7d1` (feat)

## Files Created/Modified

- `pkg/internal/lifecycle/resume.go` — exports `Resume(opts) (*ResumeResult, error)`, `ResumeOptions`, `ResumeResult`, `WaitForNodesReady`, `ReadinessProber` type. Internal: `defaultReadinessProber` package var (test-injection), `resumeBinaryName` package var (test-injection), `classifyRole`/`aggregateErrors`/`tryUntil` helpers.
- `pkg/internal/lifecycle/resume_test.go` — 14 unit tests covering ordering (LB→CP→workers, CP→workers no LB, single-node), best-effort failure aggregation, idempotency (zero start calls + zero probe calls), readiness gate success/timeout, no-readiness-on-partial-start-failure, probe-receives-bootstrap, validation errors (empty cluster, nil provider, empty name), JSON schema (with and without alreadyRunning).
- `pkg/cmd/kind/resume/resume.go` — RunE body replaces the plan-01 stub. Reads flags, validates, resolves cluster name (via test-swappable `resolveClusterName`), constructs provider, calls `resumeFn` (= lifecycle.Resume), renders text or JSON output.
- `pkg/cmd/kind/resume/resume_test.go` — 10 tests: JSON output schema, text output ("Cluster resumed. Total time:" summary line), AlreadyRunning exit 0, no-args auto-detect, readiness-timeout exit non-zero (JSON still emitted), --wait propagation, --timeout propagation, negative flag rejection (both --wait and --timeout), resolveClusterName error path.

## Exported Symbols (for plans 47-04, 47-05)

From `pkg/internal/lifecycle` (this plan adds):

```go
type ResumeOptions struct {
    ClusterName  string
    StartTimeout time.Duration  // default 30s
    WaitTimeout  time.Duration  // default 5m
    Logger       log.Logger     // default log.NoopLogger{}
    Provider     nodeFetcher    // required (interface satisfied by *cluster.Provider)
    Context      context.Context // default context.Background()
}

type ResumeResult struct {
    Cluster          string       `json:"cluster"`
    State            string       `json:"state"`            // "resumed"
    AlreadyRunning   bool         `json:"alreadyRunning,omitempty"`
    Nodes            []NodeResult `json:"nodes"`            // shared with PauseResult
    ReadinessSeconds float64      `json:"readinessSeconds"`
    Duration         float64      `json:"durationSeconds"`
}

type ReadinessProber func(ctx context.Context, bootstrap nodes.Node, deadline time.Time) error

func Resume(opts ResumeOptions) (*ResumeResult, error)
func WaitForNodesReady(ctx context.Context, bootstrap nodes.Node, deadline time.Time) error
```

`*cluster.Provider` satisfies `nodeFetcher` structurally — pass it directly.

## Test Injection Points

- **Lifecycle package:** `defaultReadinessProber ReadinessProber` (package var) — tests swap via `withReadinessProber(t, fakeFn)` to skip the real kubectl probe. `resumeBinaryName func() string` (package var) — defaults to `ProviderBinaryName`; tests can swap to inject a known runtime name.
- **Cmd package:** `resumeFn func(opts) (*ResumeResult, error)` (package var, default `lifecycle.Resume`) — tests substitute a fake to assert opts propagation, AlreadyRunning behavior, partial-failure rendering. `resolveClusterName func(args) (string, error)` (package var, default wraps `lifecycle.ResolveClusterName` with a real provider) — tests substitute a closure that returns a fixed name so they don't need a real `*cluster.Provider`.

## JSON Output Schema (`kinder resume --json`)

```json
{
  "cluster": "kind",
  "state": "resumed",
  "alreadyRunning": false,
  "nodes": [
    {"name": "kind-control-plane", "role": "control-plane", "success": true, "durationSeconds": 0.7},
    {"name": "kind-worker", "role": "worker", "success": true, "durationSeconds": 0.5}
  ],
  "readinessSeconds": 3.5,
  "durationSeconds": 5.2
}
```

`alreadyRunning` is omitted when false (omitempty); `error` per node is omitted when empty.

## Decisions Made

See frontmatter `key-decisions` for the full list. Highlights:

- **Readiness probe queries ALL nodes:** Drops the `--selector=` filter that create's `waitforready.go` uses. Rationale: during `create`, workers may not exist yet, so create can only wait for control-plane. During `resume`, every container has been started, so every node must report Ready before the user can run kubectl. This matches the plan's `<interfaces>` block recommendation.
- **K8s 1.24 selector fallback retained but unused:** The version-aware `node-role.kubernetes.io/control-plane` ↔ `node-role.kubernetes.io/master` swap from `waitforready.go:76-89` is preserved in `WaitForNodesReady` (kube version is read and parsed) for any future caller that wants to filter by role. The unused branch is a small future-proofing cost.
- **Skip readiness on partial start failure:** If even one container failed to start, the readiness probe will never succeed (the failed node's status is missing or NotReady). Returning aggregated start errors immediately is faster, less noisy, and gives the user actionable output.
- **Per-node logging from lifecycle, summary from cmd:** `lifecycle.Resume` emits one `V(0).Infof` line per node (✓ or ✗ with role and duration); the cmd RunE emits only the single final "Cluster resumed. Total time: X.Xs" line. Matches the pattern plan 47-02 established for pause and avoids duplication.
- **Idempotent fast-path checks ClusterStatus before classification:** Avoids any node iteration when nothing needs to be done. Mirrors the same fast-path pattern in `lifecycle.Pause` from plan 47-02.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Plan's `files_modified` paths use the legacy `pkg/cluster/internal/lifecycle/` location**

- **Found during:** Initial context review (orchestrator pre-flagged this).
- **Issue:** Plan 47-03 frontmatter lists `pkg/cluster/internal/lifecycle/resume.go` and `pkg/cluster/internal/lifecycle/resume_test.go`. Plan 47-01's executor relocated the package to `pkg/internal/lifecycle/` because Go's internal-package rule blocks `pkg/cmd/kind/...` from importing under `pkg/cluster/internal/...`. The orchestrator's spawn message also flagged this.
- **Fix:** Wrote the files at the corrected location `pkg/internal/lifecycle/resume.go` and `pkg/internal/lifecycle/resume_test.go`. Used `import "sigs.k8s.io/kind/pkg/internal/lifecycle"` from the cmd package.
- **Verification:** `go build ./...` passes; the import path matches what plan 47-02's `pause.go` uses.
- **Committed in:** `50c686aa` and `cc6fa301`.

**2. [Rule 3 - Blocking] Avoided redeclaring `NodeResult` and `nodeFetcher` (already defined in pause.go)**

- **Found during:** First `go build` after writing initial resume.go draft.
- **Issue:** I initially defined `NodeResult` struct and `nodeFetcher` interface in resume.go, intending to be the file that owns the shared types. But plan 47-02's `pause.go` (committed concurrently in Wave 2) already defines both. Redeclaration caused `nodeFetcher redeclared` and `NodeResult redeclared` build errors.
- **Fix:** Removed both declarations from `resume.go`; left a single comment pointing at `pause.go` as the source of truth. `Resume()` references `NodeResult` and `nodeFetcher` by name (same package) without import.
- **Verification:** `go build ./...` clean. All resume tests still pass (they were already using `NodeResult` and the fake provider satisfies `nodeFetcher`).
- **Committed in:** `50c686aa` (the resume.go GREEN commit had the fix baked in).
- **Note for future plans:** Wave-2 plans that share package-level types must check what already exists in the package before declaring. The plan that lands first owns the declaration; the second plan references by name.

---

**Total deviations:** 2 auto-fixed (both Rule 3 - Blocking, both pre-flagged or trivially detected). No scope creep, no architectural changes, no new dependencies.

## Issues Encountered

None beyond the two deviations above. Plan 47-02's GREEN cycle completed during my initial context-load, which actually made my job easier (NodeResult was already defined; I just used it).

## TDD Gate Compliance

- **Task 1 RED:** `cc6fa301` (test) — confirmed package fails to compile without resume.go.
- **Task 1 GREEN:** `50c686aa` (feat) — 14 TestResume cases pass.
- **Task 2 RED:** `87d55f21` (test) — confirmed cmd package fails to compile without resumeFn / resolveClusterName package vars.
- **Task 2 GREEN:** `3d8cf7d1` (feat) — 10 TestResumeCmd cases pass.
- No REFACTOR commits needed (initial GREEN was clean).

## User Setup Required

None — no external service configuration required. `kinder resume` is fully self-contained against any kinder-managed cluster.

## Next Phase Readiness for Plan 47-04 (Doctor Readiness Check)

Implicit contract:
- `lifecycle.Resume` calls `defaultReadinessProber(ctx, bootstrap, deadline)` (which defaults to `lifecycle.WaitForNodesReady`).
- Plan 47-04 will inject an additional doctor-style check INVOCATION before the workers start by either (a) wrapping `defaultReadinessProber` in a chained probe, or (b) adding a hook in `Resume` between the start loop and the readiness gate. Either approach can be implemented without modifying `Resume`'s public signature.
- The plan-01 lifecycle helpers (`ClassifyNodes`, `ContainerState`) are also available for plan 47-04 to use.
- HA pause snapshot (`/kind/pause-snapshot.json` written by plan 47-02's `Pause`) is the data source plan 47-04's doctor check will read back to compare leader identity across the pause/resume gap.

## Self-Check: PASSED

All four created/modified files verified present on disk.
All four task commits verified present in git log.

```
FOUND: pkg/internal/lifecycle/resume.go
FOUND: pkg/internal/lifecycle/resume_test.go
FOUND: pkg/cmd/kind/resume/resume.go (modified, plan-01 stub replaced)
FOUND: pkg/cmd/kind/resume/resume_test.go
FOUND commit: cc6fa301 (Task 1 RED)
FOUND commit: 50c686aa (Task 1 GREEN)
FOUND commit: 87d55f21 (Task 2 RED)
FOUND commit: 3d8cf7d1 (Task 2 GREEN)
```

---
*Phase: 47-cluster-pause-resume*
*Plan: 03*
*Completed: 2026-05-03*
