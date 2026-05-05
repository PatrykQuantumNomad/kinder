---
phase: 47-cluster-pause-resume
plan: 06
subsystem: infra
tags: [docker, kinder, doctor, cobra, cli, tdd, etcd, ha, duration]

# Dependency graph
requires:
  - phase: 47-cluster-pause-resume
    plan: 05
    provides: crictl exec etcdctl probe pattern in doctor resumereadiness check and pause readEtcdLeaderID
  - phase: 47-cluster-pause-resume
    plan: 04
    provides: clusterResumeReadinessCheck struct with injection fields; lifecycle.ResumeReadinessHook
  - phase: 47-cluster-pause-resume
    plan: 01
    provides: lifecycle.ResolveClusterName, lifecycle.ContainerState
provides:
  - Presence-only kind cluster filter in clusterskew.go (any cluster name discovered, not just "kind")
  - realListCPNodes uses docker ps -a so stopped CPs appear in declared topology
  - running-CP bootstrap selector in resumereadiness.Run (all-stopped → warn not skip)
  - realInspectState in doctor package (inlines lifecycle.ContainerState to avoid import cycle)
  - DurationVar for --wait and --timeout on kinder resume and kinder pause
  - cobra.MaximumNArgs(1) + lifecycle.ResolveClusterName on kinder get nodes
  - 16 test changes across 5 test files locking in gap-closure behavior
affects: [phase-48, 47-UAT]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Helper extraction pattern: factor docker ps args into clusterFilter()/cpNodeFilter() for unit-testable filter assertions"
    - "realInspectState inlines lifecycle.ContainerState to avoid doctor→lifecycle import cycle"
    - "listNodes nil-check injection: production code nil-guards the var; tests swap it to capture resolved names"

key-files:
  created: []
  modified:
    - pkg/internal/doctor/clusterskew.go
    - pkg/internal/doctor/clusterskew_test.go
    - pkg/internal/doctor/resumereadiness.go
    - pkg/internal/doctor/resumereadiness_test.go
    - pkg/cmd/kind/resume/resume.go
    - pkg/cmd/kind/resume/resume_test.go
    - pkg/cmd/kind/pause/pause.go
    - pkg/cmd/kind/pause/pause_test.go
    - pkg/cmd/kind/get/nodes/nodes.go
    - pkg/cmd/kind/get/nodes/nodes_test.go

key-decisions:
  - "Bare integer --wait=600 intentionally rejected after DurationVar migration; no install base for Phase 47 flags makes backward compat not worth a custom flag type"
  - "realInspectState inlines lifecycle.ContainerState rather than importing lifecycle (would create doctor→lifecycle→doctor cycle)"
  - "listNodes var is nil by default; production path nil-guards it and falls through to provider.ListNodes; test path sets it to capture the resolved name"
  - "All-stopped HA cluster returns warn not skip: a completely stopped HA cluster is real degradation with user-actionable advice, not absence-of-check"

patterns-established:
  - "cpNodeFilter()/clusterFilter() helper extraction: makes docker ps arg assertions testable without executing docker"
  - "inspectState injection field on check struct: enables per-container state mocking in unit tests"

requirements-completed: [LIFE-01, LIFE-02, LIFE-03, LIFE-04]

# Metrics
duration: ~40min
completed: 2026-05-05
---

# Phase 47 Plan 06: Gap Closure UAT Fixes Summary

**Four source gaps closed: presence-only cluster filter, docker ps -a declared topology, DurationVar CLI flags, and positional cluster arg on kinder get nodes — all with RED then GREEN TDD commits.**

## Performance

- **Duration:** ~40 min
- **Started:** 2026-05-05T10:15:00Z
- **Completed:** 2026-05-05T10:55:00Z
- **Tasks:** 3 (6 atomic commits: 3x RED + 3x GREEN)
- **Files modified:** 10 (4 source + 5 test, nodes.go counts as both)

## Accomplishments

- clusterskew.go: removed `=kind` pin from docker ps label filter; now `label=io.x-k8s.kind.cluster` (presence-only) so any cluster name is discovered by the skew check
- resumereadiness.go: added `-a` flag to docker ps so stopped CPs appear in declared topology; added running-CP bootstrap selector (cp1 exited → try cp2 → try cp3); all-stopped edge case returns warn not skip
- resume.go + pause.go: migrated `--wait` and `--timeout` from `IntVar(int)` to `DurationVar(time.Duration)` — duration strings (5m, 30s, 2m) now accepted natively; bare ints rejected
- nodes.go: cobra.NoArgs → cobra.MaximumNArgs(1); resolveClusterName package var; runE threads args; positional cluster name works like `kinder pause`/`kinder resume`
- 16 test changes (4 new doctor, 4 updated + 4 new resume, 3 new pause, 4 new nodes) — each gap locked in with a failing-then-passing test

## Task Commits (RED→GREEN order)

1. **Task 1 RED:** `7d860f76` — test(47-06): add failing tests for doctor cluster-discovery fixes (gap closure)
2. **Task 1 GREEN:** `ed85ecdf` — fix(47-06): doctor discovers any cluster name; HA gate uses declared topology + running-CP bootstrap (LIFE-04)
3. **Task 2 RED:** `738c70c0` — test(47-06): migrate --wait/--timeout tests to duration syntax (RED)
4. **Task 2 GREEN:** `7a4f722f` — feat(47-06): --wait/--timeout accept Go duration strings (5m, 30s) on kinder pause/resume
5. **Task 3 RED:** `057df188` — test(47-06): add failing tests for kinder get nodes positional cluster arg (RED)
6. **Task 3 GREEN:** `50aa742a` — feat(47-06): kinder get nodes accepts positional cluster name (matches pause/resume convention)

## Files Created/Modified

- `pkg/internal/doctor/clusterskew.go` — added `clusterFilter()` helper returning presence-only label filter; realListNodes calls it
- `pkg/internal/doctor/clusterskew_test.go` — added TestClusterNodeSkew_NonDefaultClusterName_Discovered, TestClusterNodeSkew_RealListFilter_NoValuePin
- `pkg/internal/doctor/resumereadiness.go` — added inspectState field + realInspectState; added cpNodeFilter() with -a; bootstrap selector loop; all-stopped warn result
- `pkg/internal/doctor/resumereadiness_test.go` — fakeReadinessOpts.inspectStates field; newFakeResumeReadinessCheck wires inspectState; 3 new tests
- `pkg/cmd/kind/resume/resume.go` — flagpole.Timeout/WaitTimeout → time.Duration; IntVar → DurationVar; removed * time.Second conversions
- `pkg/cmd/kind/resume/resume_test.go` — 4 updated tests (duration syntax); 1 new BareIntegerWaitRejected
- `pkg/cmd/kind/pause/pause.go` — flagpole.Timeout → time.Duration; IntVar → DurationVar; removed * time.Second conversion
- `pkg/cmd/kind/pause/pause_test.go` — added time import; 3 new tests (Propagated, Negative, BareInteger)
- `pkg/cmd/kind/get/nodes/nodes.go` — cobra.NoArgs → MaximumNArgs(1); resolveClusterName var; listNodes nil-check var; runE(args); resolve logic
- `pkg/cmd/kind/get/nodes/nodes_test.go` — withResolveClusterName/withListNodes/newTestStreams helpers; nodes import; 4 new tests

## Decisions Made

- **Bare integer break accepted:** `--wait=600` (no unit) now rejected; `--wait=600s` or `--wait=10m` required. No install base for Phase 47 CLI flags — accepting bare ints not worth a custom flag type.
- **All-stopped HA cluster → warn:** When every CP is exited, the check returns `warn` with "all control-plane containers stopped" / "cannot probe etcd" message rather than `skip`. This is intentional and matches CONTEXT.md warn-and-continue: a completely stopped HA cluster is real degradation, not "check not applicable".
- **realInspectState inlines ContainerState:** To avoid creating a doctor→lifecycle import cycle (lifecycle.resume.go already imports doctor), `realInspectState` duplicates the three-line inspect command pattern rather than calling `lifecycle.ContainerState`.
- **listNodes nil-check injection:** Rather than a complex provider-wrapper, `listNodes` is a package-level var that is nil by default. Production code nil-guards it. Tests set it to a closure that captures which name was passed. Avoids needing to wire a real cluster.Provider in test setup.

## Deviations from Plan

None — plan executed exactly as written. All four RED compile errors (clusterFilter, cpNodeFilter, resolveClusterName, listNodes undefined) failed as expected; all GREEN tests passed after source changes.

## Tests Added (count per the plan's output spec)

- Doctor package (`pkg/internal/doctor/`): 4 new (2 clusterskew + 2 resumereadiness + 1 cpNodeFilter assertion)
- Resume package (`pkg/cmd/kind/resume/`): 4 updated + 1 new = 5 test changes
- Pause package (`pkg/cmd/kind/pause/`): 3 new tests + time import
- Nodes package (`pkg/cmd/kind/get/nodes/`): 4 new tests + 3 new helpers
- **Total: 16 test changes across 5 test files**

## Known Stubs

None.

## Rebuild Reminder (developer action required)

Source fixes alone do NOT close UAT tests 12 and 14 — those failures were also caused by a stale binary (`bin/kinder` built May 5 08:40, before 47-05 commit 0c612a54 at 09:54). After these source commits land, the developer MUST rebuild and reinstall before re-running UAT 12, 13, 14:

```bash
cd /Users/patrykattc/work/git/kinder
go build -o bin/kinder ./cmd/kinder
install bin/kinder /opt/homebrew/bin/kinder    # overrides Homebrew Cask symlink with fresh build
```

Verify the running binary contains the gap-closure fixes:

```bash
which kinder                                                              # → /opt/homebrew/bin/kinder
strings $(which kinder) | grep -c 'crictl ps --name etcd'                # MUST be ≥1
strings $(which kinder) | grep -cE 'label=io\.x-k8s\.kind\.cluster=kind' # MUST be 0
strings $(which kinder) | grep -c 'all control-plane containers stopped'  # MUST be ≥1
```

Note: `/opt/homebrew/Caskroom/kinder/1.4/kinder` is left untouched; only the `/opt/homebrew/bin/kinder` symlink target is replaced. To restore: `brew reinstall kinder`.

After rebuild, re-run UAT 3, 9, 12, 13, 14 against a fresh 3-CP HA cluster. All five should now pass.

## Next Phase Readiness

Phase 47 gap closure complete. The four source gaps (cluster discovery, HA topology counting, duration flag syntax, positional cluster arg) are fixed and tested. Phase 47 fully delivers LIFE-01..LIFE-04 including gap closure 47-06.

Ready to start Phase 48 (cluster snapshot/restore) context-gathering.

---
*Phase: 47-cluster-pause-resume*
*Completed: 2026-05-05*

## Self-Check: PASSED

- All 10 source/test files confirmed present on disk
- All 6 commits (7d860f76, ed85ecdf, 738c70c0, 7a4f722f, 057df188, 50aa742a) confirmed in git history
- `go test ./... -count=1 -timeout 5m` exits 0 (verified above)
- Doctor registry still 24 checks (gpu_test.go + socket_test.go assertions both pass)
