---
phase: 47-cluster-pause-resume
plan: 04
subsystem: cluster-lifecycle
tags: [doctor, etcdctl, etcd-quorum, ha, cobra, json, tdd]

# Dependency graph
requires:
  - phase: 47-01
    provides: lifecycle.ProviderBinaryName, lifecycle.ClassifyNodes, lifecycle.ContainerState, lifecycle.ClusterStatus, defaultCmder injection
  - phase: 47-02
    provides: /kind/pause-snapshot.json schema (leaderID + pauseTime), lifecycle.NodeResult shared type
  - phase: 47-03
    provides: lifecycle.Resume orchestration loop, lifecycle.ResumeOptions, lifecycle.ResumeResult, lifecycle.WaitForNodesReady, defaultReadinessProber injection point, three-phase ordering surface (LB → CP → workers)
provides:
  - doctor.NewClusterResumeReadinessCheck() exported constructor (cross-package callable from lifecycle)
  - cluster-resume-readiness Check registered in doctor.allChecks (24th check)
  - Inline lifecycle.ResumeReadinessHook package-var invoked between CP-start and worker-start on HA clusters only
  - Three-phase Resume ordering refactor (LB → CP → readiness check → workers)
  - Snapshot-aware leader rotation detection (compares /kind/pause-snapshot.json leaderID to current etcd leader)
  - HA-only quorum probe with graceful skip when etcdctl missing or single-CP cluster
affects: [47-05-PLAN docs (must document doctor check + inline resume behavior)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Doctor check with three injectable dependencies (listClusterNodes, execInContainer, readSnapshot) — extends the clusterskew injection pattern from one fn to three for richer fakes
    - Cross-package check invocation via exported constructor (NewClusterResumeReadinessCheck) — first time the doctor package exports a per-check constructor for inline use
    - Package-level hook var (ResumeReadinessHook) for orchestration extension without changing ResumeOptions surface
    - Three-phase explicit start loop (closure-based startNodes helper) replaces single-loop iteration when an inter-phase hook is needed
    - Warn-and-continue semantics preserved end-to-end: hook returns no error, results flow through opts.Logger only, Resume's exit code is independent of hook output

key-files:
  created:
    - pkg/internal/doctor/resumereadiness.go
    - pkg/internal/doctor/resumereadiness_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/gpu_test.go
    - pkg/internal/doctor/socket_test.go
    - pkg/internal/lifecycle/resume.go
    - pkg/internal/lifecycle/resume_test.go

key-decisions:
  - "Exported NewClusterResumeReadinessCheck() so lifecycle.Resume can instantiate the check inline without re-implementing the etcd probe (avoids duplication; single source of truth for the readiness logic)"
  - "Lifecycle imports doctor (pkg/internal/lifecycle → pkg/internal/doctor) — no cycle because doctor does not import lifecycle/cluster (verified via grep before introducing the import)"
  - "Refactored Resume's single start-loop into three explicit phases (LB → CP → hook → workers) using a closure helper (startNodes) — keeps results/startErrs aggregation in one place while adding a clear inter-phase extension point"
  - "Hook is gated by len(cp) >= 2 AND len(startErrs) == 0 AND ResumeReadinessHook != nil — single-CP and partial-failure paths both bypass the hook with zero overhead, matching CONTEXT.md decision and the existing readiness-gate skip-on-failure rule"
  - "Hook signature is (binaryName, bootstrapCP, logger) instead of (opts ResumeOptions) — keeps the hook decoupled from ResumeOptions internals; tests can install a hook without constructing a full ResumeOptions"
  - "doctor.fail status is documented as never-emitted by this check (warn-and-continue) but defaultResumeReadinessHook still handles fail by logging it like warn — defensive in case future code paths add it"
  - "Updated TWO registry tests (gpu_test.go and socket_test.go) when bumping check count 23→24; both have full ordered-name assertions and would silently break otherwise"

patterns-established:
  - "Pattern: doctor check with multiple injectable closures (3 deps here vs 1 in clusterskew) — when a check needs richer test isolation, expand to per-collaborator closures rather than monolithic fakes"
  - "Pattern: cross-package doctor check invocation — NewClusterResumeReadinessCheck() exported constructor lets lifecycle/orchestration code call doctor checks inline without import cycles"
  - "Pattern: orchestration-extension via package-level hook var (ResumeReadinessHook) — keeps the public ResumeOptions surface stable while letting future plans (or tests) inject behavior between fixed phases"

# Metrics
duration: ~30 min
completed: 2026-05-03
---

# Phase 47 Plan 04: Cluster Resume Readiness Doctor Check Summary

**HA-aware `cluster-resume-readiness` doctor check (LIFE-04) with inline invocation in `kinder resume` between CP-start and worker-start; warns on etcd quorum risk or leader rotation since pause but never blocks resume — completes Phase 47 (LIFE-01..LIFE-04).**

## Performance

- **Duration:** ~30 min elapsed (TDD RED→GREEN cycles for both tasks; near-zero rework — plan 47-01/02/03 had cleared the import path and shared-type questions)
- **Started:** 2026-05-03T20:03Z (approx, when this executor began)
- **Completed:** 2026-05-03T20:14Z
- **Tasks:** 2 (both TDD)
- **Files modified:** 7 (2 created, 5 modified)
- **New module deps:** 0

## Accomplishments

- New `pkg/internal/doctor/resumereadiness.go` implementing the `Check` interface with HA-only quorum probing via `etcdctl endpoint health --cluster --write-out=json` and snapshot-leader comparison via `etcdctl endpoint status --cluster --write-out=json`.
- Doctor registry (`pkg/internal/doctor/check.go`) now contains 24 checks (was 23 after v2.2 phase 45); the new `cluster-resume-readiness` check sits in the Cluster category alongside `cluster-node-skew` and `local-path-cve`.
- Exported constructor `doctor.NewClusterResumeReadinessCheck()` so `lifecycle.Resume` can instantiate the check inline without import-cycle risk.
- `lifecycle.ResumeReadinessHook` package-level var (defaulting to `defaultResumeReadinessHook`) is invoked AFTER all CP containers are started but BEFORE workers start, ONLY on HA clusters with no prior start failures.
- Three-phase Resume ordering refactor: `LB → CP → readiness hook → workers` (was a single monolithic loop in plan 47-03). Aggregation logic (results/startErrs) preserved via closure helper.
- Five dispositions covered (skip×3, ok×1, warn×1 in the matrix; 11 unit tests at the doctor layer + 5 at the lifecycle layer cover all branches):
  - `skip` — no cluster, single-CP, etcdctl missing
  - `ok` — all etcd members healthy + (no snapshot OR snapshot leader matches)
  - `warn` — unhealthy member, no healthy members, leader rotation since pause, parse failure
  - `fail` — never (CONTEXT.md "warn and continue")
- Inline hook is gated three ways (HA, no failures, hook installed) so single-CP clusters incur zero overhead.
- Both registry tests (`gpu_test.go::TestAllChecks_RegisteredOrder`, `socket_test.go::TestAllChecks_Registry`) updated to expect 24 entries with the new check at position 21 (Cluster category).
- Zero new module dependencies; `go.mod` and `go.sum` untouched.

## Task Commits

Each task was committed atomically (TDD RED → GREEN):

1. **Task 1 RED: doctor check failing tests (11 cases)** — `26f87340` (test)
2. **Task 1 GREEN: clusterResumeReadinessCheck + registry registration + registry-test updates** — `8d5504c8` (feat)
3. **Task 2 RED: ResumeReadinessHook failing tests (5 cases)** — `3c62244b` (test)
4. **Task 2 GREEN: defaultResumeReadinessHook + three-phase Resume refactor** — `9e4bf512` (feat)

## Files Created/Modified

- `pkg/internal/doctor/resumereadiness.go` — new. `clusterResumeReadinessCheck` struct with three injectable closures (`listClusterNodes`, `execInContainer`, `readSnapshot`); `Run()` decision tree; `parseEtcdHealth` and `parseEtcdStatusLeader` JSON parsers; `realListCPNodes` / `realExecInContainer` / `realReadPauseSnapshot` production deps; `NewClusterResumeReadinessCheck()` exported constructor.
- `pkg/internal/doctor/resumereadiness_test.go` — new. 11 unit tests using fake closures (no docker dependency): metadata, no-cluster skip (×2 — empty + list-error), single-CP skip, etcdctl-missing skip, healthy-HA ok, unhealthy-member warn, all-unhealthy warn, stale-snapshot warn, fresh-snapshot ok, no-snapshot ok, registry-contains assertion.
- `pkg/internal/doctor/check.go` — modified. Added `newClusterResumeReadinessCheck()` to `allChecks` after `newLocalPathCVECheck()` in the Cluster category; comment updated to "Cluster (Phase 42 + 47)".
- `pkg/internal/doctor/gpu_test.go` — modified. `TestAllChecks_RegisteredOrder` now expects 24 entries with `cluster-resume-readiness` after `local-path-cve`.
- `pkg/internal/doctor/socket_test.go` — modified. `TestAllChecks_Registry` mirror update (24 entries; same insertion point).
- `pkg/internal/lifecycle/resume.go` — modified. Added `import "sigs.k8s.io/kind/pkg/internal/doctor"`; declared `ResumeReadinessHook` package var + `defaultResumeReadinessHook` function; refactored start logic into three phases (LB / CP / hook / workers) via a closure-based `startNodes` helper.
- `pkg/internal/lifecycle/resume_test.go` — modified. Added 5 new tests under "Inline cluster-resume-readiness hook tests (plan 47-04)": `_HA` (invocation timing/args), `_SingleCP_Skipped`, `_WarnDoesNotBlock`, `_DefaultIsRealCheck`, `_SkippedOnPartialStartFailure`. All 14 pre-existing plan-03 Resume tests still pass.

## Exported Symbols (for plan 47-05 docs)

```go
// pkg/internal/doctor (new export, plan 47-04)
func NewClusterResumeReadinessCheck() Check  // HA quorum probe; never returns fail
```

```go
// pkg/internal/lifecycle (new export, plan 47-04)
var ResumeReadinessHook = defaultResumeReadinessHook  // hook signature: func(binaryName, bootstrapCP string, logger log.Logger)
```

## Doctor Check Behavior (cluster-resume-readiness)

**Disposition matrix:**

| Condition                                                            | Status | Message                                                | Reason                                            |
| -------------------------------------------------------------------- | ------ | ------------------------------------------------------ | ------------------------------------------------- |
| No kind cluster detected (empty CP list or list error)               | skip   | "no kind cluster detected"                              | —                                                 |
| Single-control-plane cluster (`len(cp) == 1`)                        | skip   | "single-control-plane cluster; HA check not applicable" | —                                                 |
| etcdctl absent inside bootstrap CP container                         | skip   | "etcdctl unavailable inside container"                  | —                                                 |
| `etcdctl endpoint health` failed                                     | warn   | "etcd endpoint health probe failed"                    | "etcdctl endpoint health returned error: <err>"  |
| Health JSON parse failed                                             | warn   | "could not parse etcd health output"                   | "<parse error>"                                  |
| 0 of N members healthy                                               | warn   | "0/N etcd members healthy"                              | "no healthy etcd members reachable; quorum lost" |
| Some-but-not-all members healthy                                     | warn   | "K/N etcd members healthy"                              | "(N-K) unhealthy etcd member(s) — quorum at risk" |
| All members healthy + snapshot leader != current leader              | warn   | "etcd leader changed since pause"                      | "leader id rotated; previous=<id>, current=<id>" |
| All members healthy + (no snapshot OR snapshot leader == current)    | ok     | "K/K etcd members healthy"                              | —                                                 |
| (no condition)                                                       | fail   | NEVER                                                   | (CONTEXT.md "warn and continue")                  |

**HA gate:** `len(cpNodeNames) > 1` (i.e., ≥2 control-plane nodes). Single-CP clusters always skip.

**Snapshot tolerance:** A missing or unparseable `/kind/pause-snapshot.json` is treated as "snapshot absent" and never demotes an `ok` to `warn`. An empty `leaderID` (which lifecycle.Pause writes when its etcdctl probe fails — see plan 47-02 SUMMARY) is also tolerated: `realReadPauseSnapshot` returns `("", false)` so the staleness comparison is skipped.

**etcdctl path probing:** The check tries `/usr/local/bin/etcdctl` directly (the path in all recent kindest/node images). If `which etcdctl` fails inside the container, the check skips with "etcdctl unavailable inside container" — matches RESEARCH.md Pitfall 3 (graceful degradation).

## Inline Hook Behavior (lifecycle.Resume)

The new three-phase Resume ordering and gating logic:

```text
Phase 1: LB         (always; starts the external load balancer if present)
Phase 2: CP         (always; starts every control-plane container in nodeutils.ControlPlaneNodes order)
Phase 3: HOOK       (HA only AND no prior failures AND ResumeReadinessHook != nil)
Phase 4: WORKERS    (always; starts every worker container)
Phase 5 (existing): READINESS PROBE  (HA-or-not; skipped on partial failure as before)
```

**Hook gating** (all three must be true):
1. `len(cp) >= 2` — single-CP clusters skip (zero overhead)
2. `len(startErrs) == 0` — no point probing quorum if a CP container failed to start
3. `ResumeReadinessHook != nil` — defensive against tests setting hook to nil

**Hook signature:** `func(binaryName, bootstrapCP string, logger log.Logger)`. The hook returns no error — warnings flow through `logger` only and Resume's exit code is independent of hook output.

**Default impl** (`defaultResumeReadinessHook`):
- Calls `doctor.NewClusterResumeReadinessCheck().Run()`
- Forwards results to logger by status: `warn`/`fail` → `logger.Warnf("⚠ %s: %s — %s", ...)`, `skip` → `logger.V(2).Infof(...)`, `ok` → `logger.V(1).Infof(...)`
- Defensive `fail` handling included even though the check never emits fail

## Test Coverage

**Doctor layer (`pkg/internal/doctor/resumereadiness_test.go`):** 11 tests, all passing
- Metadata (Name/Category/Platforms)
- skip × 3 (no cluster, list error, single CP, etcdctl missing — 4 actually; counts as "skip" disposition)
- ok × 3 (healthy HA, fresh snapshot match, no snapshot)
- warn × 3 (unhealthy member, all unhealthy, stale snapshot)
- Registry assertion (TestRegistry_ContainsResumeReadiness)

**Lifecycle layer (`pkg/internal/lifecycle/resume_test.go` additions):** 5 new tests, all passing
- `TestResume_InlineReadinessHook_HA` — exactly 1 invocation, after all 3 CPs, before any worker
- `TestResume_InlineReadinessHook_SingleCP_Skipped` — hook NEVER called on 1-CP cluster
- `TestResume_InlineReadinessHook_WarnDoesNotBlock` — hook warn does not abort Resume; all NodeResults still success; no error returned
- `TestResume_InlineReadinessHook_DefaultIsRealCheck` — package-var defaults to a non-nil function (production wiring)
- `TestResume_InlineReadinessHook_SkippedOnPartialStartFailure` — hook NOT called when a CP fails to start (gating clause works)

**All 14 plan-03 Resume tests still pass** — no regression.

## Decisions Made

See frontmatter `key-decisions` for the full list. Highlights:

- **Cross-package doctor invocation:** Exported `NewClusterResumeReadinessCheck()` so the lifecycle package can call the doctor check directly. Verified before doing this that `pkg/internal/doctor/*` does not import any `pkg/internal/lifecycle` or `pkg/cluster` symbol — no cycle risk. (Note: `pkg/cluster/internal/create/create.go` already imports doctor for `ApplySafeMitigations`, proving this direction works in production.)
- **Three-phase refactor over conditional check inside loop:** Could have left the single-loop start in resume.go and put a "what role is the next node?" branch inside the loop. Three explicit phases are far clearer to read, easier to test (the test simply snapshots `startOrder` at hook-call time and asserts on it), and naturally express the gating condition.
- **Hook signature decoupled from ResumeOptions:** Took `(binaryName, bootstrapCP, logger)` instead of `(opts ResumeOptions)`. Tests don't have to construct a full options struct to install a hook, and future hooks (or alternative checks) can swap in trivially.
- **Updated BOTH registry tests:** The doctor package has two distinct registry-shape tests — `TestAllChecks_RegisteredOrder` in gpu_test.go and `TestAllChecks_Registry` in socket_test.go. Both have full ordered-name assertions; missing either would have caused `go test ./pkg/internal/doctor/...` to fail. The fix is mechanical (bump 23→24, insert the new entry after `local-path-cve`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Plan referenced legacy lifecycle path**

- **Found during:** Initial context review (orchestrator pre-flagged this in the spawn message).
- **Issue:** Plan 47-04 frontmatter `files_modified` lists `pkg/cluster/internal/lifecycle/resume.go` and `pkg/cluster/internal/lifecycle/resume_test.go`. Plan 47-01's executor relocated the package to `pkg/internal/lifecycle/` because Go's internal-package rule blocks `pkg/cmd/kind/...` from importing under `pkg/cluster/internal/`. The orchestrator's spawn message and STATE.md both flagged this.
- **Fix:** Modified the actual files at `pkg/internal/lifecycle/resume.go` and `pkg/internal/lifecycle/resume_test.go`. The doctor files (`pkg/internal/doctor/...`) were unchanged from plan as the doctor package was always at the corrected path.
- **Verification:** `go build ./...` clean.
- **Committed in:** `9e4bf512`.

**2. [Rule 2 - Missing critical] Both registry-shape tests had to be updated, not just one**

- **Found during:** Task 1 GREEN test run (initial check.go modification triggered failures in two tests, not one).
- **Issue:** Plan only mentioned updating the doctor's `allChecks` slice. The registry has two ordered-shape tests (one in gpu_test.go from Phase 40, one in socket_test.go from Phase 39) that BOTH iterate the slice and assert names + categories at every index. Bumping the slice to 24 without updating both tests would have left the gpu_test.go test failing.
- **Fix:** Updated both `TestAllChecks_RegisteredOrder` and `TestAllChecks_Registry` to expect 24 entries and to include `{"cluster-resume-readiness", "Cluster"}` at the right insertion point (index 21, after `local-path-cve`). Both tests now pass.
- **Files modified:** `pkg/internal/doctor/gpu_test.go`, `pkg/internal/doctor/socket_test.go`.
- **Verification:** `go test ./pkg/internal/doctor/...` clean.
- **Committed in:** `8d5504c8` (combined with the rest of Task 1 GREEN).

---

**Total deviations:** 2 auto-fixed (1 blocking-path correction pre-flagged by orchestrator, 1 test-suite update plan didn't anticipate). No scope creep, no architectural changes, no new dependencies, no plan content changed.

## Issues Encountered

None beyond the two deviations above. Both TDD cycles completed first try after the fixes.

## TDD Gate Compliance

- **Task 1 RED:** `26f87340` (test) — `go test` fails at compile-time (`undefined: clusterResumeReadinessCheck`).
- **Task 1 GREEN:** `8d5504c8` (feat) — 11 TestClusterResumeReadiness cases + TestRegistry_ContainsResumeReadiness all pass.
- **Task 2 RED:** `3c62244b` (test) — `go test` fails at compile-time (`undefined: ResumeReadinessHook`).
- **Task 2 GREEN:** `9e4bf512` (feat) — 5 TestResume_InlineReadinessHook cases pass; ALL 14 prior plan-03 Resume tests still pass.
- No REFACTOR commits needed; both initial GREENs were clean.

## User Setup Required

None — no external service configuration required. The check runs against any HA kinder cluster as soon as the binary is rebuilt (`go build ./cmd/kind/`). Single-CP clusters automatically skip the check (no setup required).

## Next Phase Readiness

**Phase 47 is FUNCTIONALLY COMPLETE.** All four LIFE-* requirements delivered:

- **LIFE-01** (`kinder status` + Status column on get clusters/nodes) — plan 47-01
- **LIFE-02** (`kinder pause` quorum-safe + HA snapshot capture) — plan 47-02
- **LIFE-03** (`kinder resume` quorum-safe + readiness gate) — plan 47-03
- **LIFE-04** (`cluster-resume-readiness` doctor check + inline invocation) — plan 47-04 (this plan)

**Plan 47-05 (docs)** must document:
- New `kinder doctor` check `cluster-resume-readiness` (HA-only; warn-and-continue; what it probes)
- The inline invocation in `kinder resume` (transparent to users; appears in the resume output as a warn line if quorum is at risk)
- The `/kind/pause-snapshot.json` schema (now stable across plans 02 and 04)
- The exported `doctor.NewClusterResumeReadinessCheck()` constructor and `lifecycle.ResumeReadinessHook` package-var (low-priority since these are internal — only matters if external consumers/tooling want to build on top)
- Updated three-phase Resume ordering (LB → CP → readiness check → workers)

## Threat Flags

None. The new check exercises only operations already trusted by the existing kinder code path:
- `etcdctl endpoint health --cluster` and `etcdctl endpoint status --cluster` use the same peer cert paths under `/etc/kubernetes/pki/etcd/` that `lifecycle.Pause` (plan 47-02) and every other kind etcd interaction already use.
- `/kind/pause-snapshot.json` was created at trust boundary by `lifecycle.Pause` and is read inside the same container via `<runtime> exec` — no new file IO at host trust boundary, no secret extraction.
- The exported `doctor.NewClusterResumeReadinessCheck()` constructor returns a check that runs identical commands to the production `kinder doctor` invocation; no new privilege escalation surface.

## Self-Check: PASSED

All claimed files verified present on disk:

- `pkg/internal/doctor/resumereadiness.go` — present
- `pkg/internal/doctor/resumereadiness_test.go` — present
- `pkg/internal/doctor/check.go` — modified (registers `newClusterResumeReadinessCheck()`)
- `pkg/internal/doctor/gpu_test.go` — modified (registry test updated to 24)
- `pkg/internal/doctor/socket_test.go` — modified (registry test updated to 24)
- `pkg/internal/lifecycle/resume.go` — modified (ResumeReadinessHook + three-phase refactor)
- `pkg/internal/lifecycle/resume_test.go` — modified (5 new TestResume_InlineReadinessHook cases)

All claimed commits verified in git log:

- `26f87340` test(47-04): add failing tests for cluster-resume-readiness doctor check
- `8d5504c8` feat(47-04): implement cluster-resume-readiness doctor check (LIFE-04)
- `3c62244b` test(47-04): add failing tests for inline ResumeReadinessHook in lifecycle.Resume
- `9e4bf512` feat(47-04): wire inline cluster-resume-readiness hook into lifecycle.Resume

Plan-level verification block re-run: `go test ./pkg/internal/doctor/... ./pkg/internal/lifecycle/...` ok; `go build ./...` clean; `go vet ./...` clean; whole-repo `go test ./...` all packages ok.

---
*Phase: 47-cluster-pause-resume*
*Plan: 04 (final plan in phase 47)*
*Completed: 2026-05-03*
