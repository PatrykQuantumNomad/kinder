---
gsd_state_version: 1.0
milestone: v2.3
milestone_name: Inner Loop
status: in_progress
stopped_at: Phase 47 COMPLETE (LIFE-01..LIFE-04 delivered); ready to plan phase 48 (cluster snapshot/restore)
last_updated: "2026-05-05T10:05:00Z"
last_activity: 2026-05-05 — Phase 47 plan 05 complete (gap closure LIFE-04: replaced which-etcdctl probe with crictl exec-based probe in cluster-resume-readiness doctor check and pause.go readEtcdLeaderID; 5 new tests; real-cluster smoke pending manual verification)
progress:
  total_phases: 5
  completed_phases: 1
  total_plans: 21
  completed_plans: 5
  percent: 24
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-03 for v2.3 milestone start)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v2.3 Inner Loop — Phase 47: Cluster Pause/Resume

## Current Position

Phase: 48 of 51 (next: Cluster Snapshot/Restore — needs context + planning)
Plan: 01 (not yet planned)
Status: Phase 47 complete including gap closure 47-05; ready to start phase 48 context-gathering
Last activity: 2026-05-05 — Plan 47-05 shipped (gap closure LIFE-04): replaced unreachable which-etcdctl probe in cluster-resume-readiness doctor check and pause.go readEtcdLeaderID with crictl exec-based probe. Doctor check now actually evaluates etcd health on real HA clusters (was always skip before). Pause snapshots now capture real leaderID. 5 new tests (2 doctor + 3 pause). Phase 47 fully delivers LIFE-01..LIFE-04 including production-verified code path.

Progress: ████░░░░░░ 19% (4 of 21 plans)

## Performance Metrics

**Velocity:**

- v1.0: 12 plans, 8 phases, 1 day
- v1.1: 8 plans, 6 phases, 2 days
- v1.2: 4 phases, 1 day
- v1.3: 8 plans, 6 phases, ~5 hours
- v1.4: 13 plans, 5 phases, 2 days
- v1.5: 7 plans, 5 phases, 1 day
- v2.0: 7 plans, 3 phases, 2 days
- v2.1: 10 plans, 4 phases, 1 day
- v2.2: 14 plans, 5 phases, ~2.5 days

**By Phase:**

| Phase | Plan | Duration | Tasks | Files | Notes                                                                                                       |
| ----- | ---- | -------- | ----- | ----- | ----------------------------------------------------------------------------------------------------------- |
| 47    | 01   | ~4h      | 3     | 11    | TDD cycles for tasks 1+2 (RED→GREEN); 2 auto-fix deviations (lifecycle path move, dead nodeLister cleanup). |
| 47    | 02   | ~7m      | 2     | 4     | TDD RED→GREEN for both tasks; 1 deviation (parallel-wave conflict with 47-03 — resume_test.go/resume.go redeclared shared symbols, parked aside during test runs). |
| 47    | 03   | ~25m     | 2     | 4     | TDD RED→GREEN for both tasks (4 commits); 2 auto-fix deviations (lifecycle path correction from plan frontmatter, removed redundant NodeResult/nodeFetcher declarations after 47-02 landed first). |
| 47    | 04   | ~30m     | 2     | 7     | TDD RED→GREEN for both tasks (4 commits); 2 auto-fix deviations (lifecycle path correction pre-flagged by orchestrator, both registry tests in gpu_test.go + socket_test.go updated for 23→24 check count). LIFE-04 delivered; Phase 47 complete. |
| 47    | 05   | ~25m     | 2     | 4     | TDD RED→GREEN for both tasks (4 commits); 1 auto-fix deviation (test lookup false substring match on "ps" inside "--endpoints=https://"). Gap closure: crictl exec probe replaces unreachable which-etcdctl path in doctor check and pause.go. |

*Updated after each plan completion*

## Accumulated Context

### Decisions

- v1.0–v2.2: See PROJECT.md Key Decisions table (full log moved there at v2.2 milestone completion)
- 2026-05-03: v2.3 theme chosen as "Inner Loop" — strongest user-value-per-day signal; defers pure tech debt and pure differentiator features to v2.4
- 2026-05-03: Phase 49 (kinder dev) may introduce `github.com/fsnotify/fsnotify` as first new module dep since v2.0; poll-based stdlib alternative acceptable to keep zero-dep streak
- 2026-05-03 (47-01): Shared lifecycle helpers package located at `pkg/internal/lifecycle/` (not `pkg/cluster/internal/lifecycle/` as the plan specified) — Go's internal-package rule blocks `pkg/cmd/kind/...` consumers from `pkg/cluster/internal/`. Plans 47-02/03/04 must update their `files_modified` lists accordingly.
- 2026-05-03 (47-01): JSON schema for `kinder get clusters --output json` migrated from `[]string` to `[]{name,status}` — intentional breaking change accepted per CONTEXT.md.
- 2026-05-03 (47-01): Pause/resume command stubs return `errors.New("not yet implemented")` (non-zero exit) rather than success — clearer signal in dev/CI than a silent stub.
- 2026-05-03 (47-02): `lifecycle.PauseResult` / `lifecycle.NodeResult` struct json: tags ARE the `--json` wire schema — Go API and CLI contract share a single source of truth.
- 2026-05-03 (47-02): Snapshot capture for HA pause is best-effort — failures log a warning and write `{leaderID:"", pauseTime:...}` rather than aborting the pause. Plan 47-04 readiness check MUST tolerate empty `leaderID`.
- 2026-05-03 (47-02): Plans 47-02 and 47-03 share the lifecycle package and were scheduled in parallel — 47-03's untracked `resume.go` redeclared `nodeFetcher` and `NodeResult` from my pause.go. Worked around with filesystem park-aside (no commits, no modifications). 47-03 needs to rebase onto `c7952992` and reuse `lifecycle.NodeResult` instead of redeclaring it.
- 2026-05-03 (47-03): Resume's readiness probe queries ALL nodes (kubectl with no `--selector`); diverges from create's waitforready (which only watches control-plane because workers may not exist yet during create). For resume every container exists and every node must be Ready before the user can run kubectl.
- 2026-05-03 (47-03): K8s 1.24 selector fallback (`control-plane` ↔ `master` label) retained inside `WaitForNodesReady` for completeness even though Resume itself doesn't use a selector — keeps the helper reusable by any future caller (plan 47-04 doctor check) that wants to filter by role.
- 2026-05-03 (47-03): Skip readiness probe entirely if any container failed to start (no point waiting for a known-incomplete cluster). Aggregated start errors are returned directly. Idempotent fast-path also skips probe when `ClusterStatus="Running"`.
- 2026-05-03 (47-03): Resolved 47-02's blocker by reusing `NodeResult` and `nodeFetcher` from `pause.go` (same package); resume.go references them by name without redeclaration. Pattern for future Wave-2 shared-package plans: the plan that lands first owns the declaration; the second plan references by name.
- 2026-05-03 (47-04): Doctor checks can be invoked cross-package via exported per-check constructors. `doctor.NewClusterResumeReadinessCheck()` is the first such export — lifecycle.Resume calls it inline. Verified pkg/internal/doctor does not import pkg/internal/lifecycle or pkg/cluster (no cycle); pkg/cluster/internal/create already imports doctor for ApplySafeMitigations, proving this direction works in production.
- 2026-05-03 (47-04): Orchestration extension via package-level hook var (`lifecycle.ResumeReadinessHook`) keeps the public ResumeOptions surface stable while enabling inter-phase injection. Default impl wraps doctor.NewClusterResumeReadinessCheck. Tests swap via t.Cleanup like other lifecycle injection points.
- 2026-05-03 (47-04): Resume's start logic refactored from a single monolithic loop into three explicit phases (LB → CP → readiness hook → workers) using a closure-based startNodes helper. Hook is gated by HA-only AND no-prior-failures AND hook-installed (three guards). Single-CP clusters incur zero overhead.
- 2026-05-03 (47-04): cluster-resume-readiness check NEVER returns fail — only ok/warn/skip. Matches CONTEXT.md "warn and continue" semantics: warnings flow through opts.Logger, Resume's exit code is independent of hook output. defaultResumeReadinessHook still defensively handles a fail status (logs as warn) in case future code paths add it.
- 2026-05-05 (47-05): etcdctl must be reached via `crictl exec <id>` into the etcd static-pod container — NOT via direct invocation in kindest/node rootfs. etcdctl ships only inside registry.k8s.io/etcd:VERSION. crictl is available on kindest/node (used by container runtime). Cert paths are identical because kubelet bind-mounts /etc/kubernetes/pki/etcd/ into the etcd container.
- 2026-05-05 (47-05): Test lookup conditions must use args[0] (exact subcommand match) not joined-string substring when args may contain URLs. "--endpoints=https://..." contains "ps" as a substring ("https" → "tps") — substring match caused false match in test fakes.

### Pending Todos

None.

### Blockers/Concerns

None. Phase 47 fully delivers LIFE-01..LIFE-04.

## Session Continuity

Last session: 2026-05-05T10:05:00Z
Stopped at: Phase 47 COMPLETE including gap closure 47-05 (LIFE-01..LIFE-04 fully delivered with working production probe path); ready to plan phase 48 (cluster snapshot/restore)
Resume file: .planning/phases/48-cluster-snapshot-restore/ (does not yet exist — needs `gsd discuss-phase 48` to gather context)
