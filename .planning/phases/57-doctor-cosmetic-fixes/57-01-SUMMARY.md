---
phase: 57-doctor-cosmetic-fixes
plan: 01
subsystem: doctor
tags: [diagnostics, cluster-node-skew, external-load-balancer, external-etcd, role-guard, ha, golang]

# Dependency graph
requires:
  - phase: 52-life-09
    provides: "pkg/cluster/constants importable from pkg/internal/doctor (no cycle) — established by resumestrategy.go"
  - phase: 56-debt-04
    provides: "Race-clean baseline in pkg/internal/doctor (runChecks helper, parameter-injection pattern); permanent race gate make test-race-doctor over -count=100"
provides:
  - "DIAG-05 cosmetic fix: cluster-node-skew no longer emits a false-positive version-skew warning for external-load-balancer (haproxy/envoy) containers on HA clusters"
  - "Defensive guard for external-etcd role (future-proofing per RESEARCH M6; the upstream role is `not yet implemented` but costless to include)"
  - "New regression test TestClusterNodeSkew_ExternalLoadBalancer_NotWarned (3-CP + worker + LB fixture, asserts Status=ok, LB name absent from Message)"
  - "Source-level invariant test TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource (guards against silent guard removal in future refactors)"
affects: [phase-57-02, phase-57-verifier, phase-58-uat]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Inline role guard pattern in realListNodes (avoids nodeutils.InternalNodes import cycle by comparing role-strings against pkg/cluster/constants directly)"
    - "Source-invariant test pattern (os.ReadFile + non-comment grep) as a deterministic RED gate when runtime semantics alone are insufficient to fail pre-implementation"

key-files:
  created: []
  modified:
    - pkg/internal/doctor/clusterskew.go (+18 lines: constants import + 13-line role-guard stanza in realListNodes)
    - pkg/internal/doctor/clusterskew_test.go (+70 lines: 2 new tests + "os" import)

key-decisions:
  - "DIAG-05 fixed inline in realListNodes (NOT via a new nodeutils.InternalNodes import) to avoid the documented import cycle: cluster/internal/create imports doctor, so doctor cannot import nodeutils — see realListNodes header comment lines 65-66."
  - "External-etcd role guard included in the same `||` branch as LB defensively per RESEARCH M6, despite the upstream role being `not yet implemented` per constants.go:62. Cost is zero (one extra string comparison), benefit is future-proofing if external-etcd lands upstream."
  - "Source-invariant test added as a second RED gate because the runtime test alone passes even without the implementation (an LB entry with VersionErr=nil constructed by hand bypasses the realListNodes bug entirely — only the source-invariant test provides a deterministic pre-implementation RED)."

patterns-established:
  - "Cross-plan staging discipline: when running in parallel with another executor in the same working tree, use `git commit --only <path>` rather than `git add` followed by `git commit`, because the parallel agent can race the staging index between calls (see Deviations section)."

# Metrics
duration: 3m 16s
completed: 2026-05-12
---

# Phase 57 Plan 01: DIAG-05 External-Load-Balancer Skew False-Positive Fix Summary

**Inline LB/external-etcd role guard in `realListNodes` skips the `cat /kind/version` exec for non-Kubernetes containers, eliminating the cosmetic version-skew false-positive on HA clusters; +2 tests assert both the runtime behavior and the source-level invariant.**

## Performance

- **Duration:** 3m 16s
- **Started:** 2026-05-12T20:53:01Z
- **Completed:** 2026-05-12T20:56:17Z
- **Tasks:** 2 (RED + GREEN; TDD plan-level cycle)
- **Files modified:** 2 (clusterskew.go +18 LOC; clusterskew_test.go +70 LOC; total net +88 LOC)

## Accomplishments

- **DIAG-05 fixed:** External-load-balancer entries no longer trigger the version-readable warn at Run() lines 171-178; haproxy/envoy containers no longer appear in the cluster-node-skew output as "could not read version" violations.
- **Defensive external-etcd guard added:** Same `||` branch handles `ExternalEtcdNodeRoleValue` per RESEARCH M6 future-proofing — zero behavior change today (role not yet implemented upstream), zero-cost insurance for tomorrow.
- **Source invariant test added:** TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource reads clusterskew.go and asserts a non-commented reference to `constants.ExternalLoadBalancerNodeRoleValue` — guards against silent guard-removal regressions in future refactors.
- **Phase 56 race gate preserved:** `make test-race-doctor` still exits 0 with zero DATA RACE over -count=100 (2.645s); no `sync` primitive added; allChecks registry untouched.

## Task Commits

Each task was committed atomically (with the unusual cross-plan contamination documented under Deviations):

1. **Task 1 (RED): write failing regression test** — `866f2c22` `test(57-01): add RED tests for DIAG-05 LB-role guard`
   - Adds TestClusterNodeSkew_ExternalLoadBalancer_NotWarned + TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource
   - Adds `"os"` import to clusterskew_test.go
   - NOTE: this commit also unexpectedly contained 57-02's resumereadiness_test.go +128 LOC due to a parallel-executor staging race (see Deviations).
2. **Task 2 (GREEN): implement inline LB/external-etcd role guard** — first attempted as `c43bb599`, which was contaminated to contain ONLY 57-02's resumereadiness.go +44 LOC (the actual clusterskew.go change was un-staged out by the parallel agent between `git add` and `git commit`). Recovered immediately by a third commit `33544309` `feat(57-01): apply DIAG-05 LB-role guard to clusterskew.go` using `git commit --only` for deterministic content.

**Effective commit map for 57-01 work:**
- `866f2c22` — clusterskew_test.go (+70 LOC) — Task 1 RED + Task 2 source-invariant gate
- `33544309` — clusterskew.go (+18 LOC) — Task 2 GREEN actual source change

**Effective commit map for 57-02 work that landed under 57-01 hashes (cross-plan contamination):**
- `866f2c22` — resumereadiness_test.go +128 LOC (57-02's test fixtures)
- `c43bb599` — resumereadiness.go +44 LOC −16 LOC (57-02's source changes)

**Plan metadata commit:** (this SUMMARY + STATE.md row + final commit, hash assigned after this Write step.)

## Files Created/Modified

- `pkg/internal/doctor/clusterskew.go` (modified, +18 lines net)
  - Adds `"sigs.k8s.io/kind/pkg/cluster/constants"` import (line 26) in the kind subtree, alphabetically before `"sigs.k8s.io/kind/pkg/exec"`.
  - Adds inline role guard inside `realListNodes` (lines 111-126), between the inspect block (closes at line 109) and the existing `// Get live Kubernetes version from /kind/version inside the container.` comment (now at line 128). The guard:
    - Compares `role` against both `constants.ExternalLoadBalancerNodeRoleValue` and `constants.ExternalEtcdNodeRoleValue`
    - On match: appends a `nodeEntry{Name, Role, Image}` (Version="" VersionErr=nil) and `continue`s
    - The empty `Version` short-circuits the config-drift loop at line 247-249; `Role != "control-plane"` skips the CP-consistency loop; `Role != "worker"` skips the worker-skew loop; `VersionErr == nil` skips the version-readable warn
- `pkg/internal/doctor/clusterskew_test.go` (modified, +70 lines net)
  - Adds `"os"` to imports (line 21, alphabetical between `"errors"` and `"strings"`)
  - Appends `TestClusterNodeSkew_ExternalLoadBalancer_NotWarned` (lines 297-329): 3-CP HA + 1 worker + 1 LB entry slice via newTestClusterNodeSkewCheck; asserts Status="ok", LB name absent from Message, Reason empty.
  - Appends `TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource` (lines 332-362): reads clusterskew.go, strips `//` line-prefix comments, counts non-comment occurrences of `constants.ExternalLoadBalancerNodeRoleValue`, asserts >= 1.

## Decisions Made

- **Inline-guard approach (not nodeutils.InternalNodes import).** The realListNodes header comment (lines 64-66) explicitly documents that doctor cannot import the cluster package because of a cycle. Inline role-string comparison against the leaf `pkg/cluster/constants` is the established workaround (precedent: 52-04 resumestrategy.go).
- **Defensive external-etcd guard.** Constants.go:62 says `// WARNING: this node type is not yet implemented!`. The guard still includes it because (a) cost is one extra string comparison, (b) when/if external-etcd lands upstream the guard prevents a future regression that would otherwise require re-opening DIAG-05.
- **Two-test RED gate.** A single runtime test is insufficient as a RED gate because newTestClusterNodeSkewCheck takes statically-constructed entries; a hand-built `nodeEntry{Role: "external-load-balancer", VersionErr: nil}` already produces Status=ok pre-implementation. The source-invariant test reads clusterskew.go directly and is the only deterministic RED gate. The plan called this out and the second test was added as designed.
- **No edits to check.go / allChecks registry.** DIAG-05 is a behavior-change inside an existing check, not a new check. TestAllChecks_CountIs26 invariant preserved.
- **DIAG-05 checkbox in REQUIREMENTS.md NOT flipped.** Per plan output section: deferred to Phase 57 close after both 57-01 and 57-02 land.

## Deviations from Plan

### Cross-plan staging contamination (Rule 3 — Blocking; auto-recovered)

**[Rule 3 - Blocking] Parallel-executor index race produced two contaminated commits**

- **Found during:** Both Task 1 and Task 2.
- **Issue:** Plan 57-01 and Plan 57-02 are running in parallel in the **same working tree** (the orchestrator's `parallelization=true` config does NOT use git worktrees per the config note in this plan's executor context). The 57-02 executor agent was actively writing to `pkg/internal/doctor/resumereadiness_test.go` and `pkg/internal/doctor/resumereadiness.go` between my `git add` and `git commit` calls. Two concrete failure modes were observed:
    1. **Task 1 commit (866f2c22) absorbed 57-02's resumereadiness_test.go +128 LOC.** After `git add pkg/internal/doctor/clusterskew_test.go`, `git status --short` correctly showed only clusterskew_test.go as staged. By the time `git commit -m ...` ran, the 57-02 agent had also staged its resumereadiness_test.go changes, and they were swept into my commit.
    2. **Task 2 first attempt (c43bb599) contained ONLY 57-02's resumereadiness.go +44 LOC −16 LOC and ZERO of my clusterskew.go changes.** Same race in the other direction: my staged clusterskew.go was un-staged out of the index by the 57-02 agent (likely a `git restore --staged` or `git reset HEAD` in their flow) between my add and commit; the commit caught only what the 57-02 agent had freshly staged.
- **Fix:** A third commit (`33544309`) re-staged ONLY clusterskew.go using `git commit --only pkg/internal/doctor/clusterskew.go -m ...`, which bypasses the shared index entirely and commits the named paths from the working tree directly. This produced a deterministic 18-line clusterskew.go-only commit. The commit message explicitly documents the contamination so the verifier and future spelunkers understand the misleading commit log.
- **Files modified:** Only the planned files (clusterskew.go + clusterskew_test.go) when counting 57-01's intended work. 57-02's work (resumereadiness.go + resumereadiness_test.go) is preserved in 866f2c22 and c43bb599 — the parallel executor's data is not lost, just misattributed by commit message.
- **Verification:** Post-recovery `git diff HEAD~3..HEAD --stat` shows the union of 57-01 and 57-02 work; `git show 33544309 --stat` shows only clusterskew.go +18. All SC gates green at HEAD.
- **Committed in:** 866f2c22 (Task 1 RED, +clusterskew_test.go and accidentally +resumereadiness_test.go), c43bb599 (Task 2 first attempt — only contains 57-02's resumereadiness.go, NOT my Task 2 change), 33544309 (Task 2 corrective — clusterskew.go only).

**Note for verifier:** The plan's success criterion "Files touched are exactly two" is satisfied at the WORKING-TREE / FILE-LEVEL (only clusterskew.go and clusterskew_test.go contain my edits), but the commit log shows 4 files modified across the 57-01 commit range due to the parallel-executor index race. The 57-02 SUMMARY (separate document) should mirror this disclosure. Re-attributing the commits via rebase/cherry-pick was deliberately NOT attempted per the destructive-git-prohibition (never rewrite history).

---

**Total deviations:** 1 auto-fixed (Rule 3 — Blocking, recovered via `git commit --only`)
**Impact on plan:** Final HEAD state is correct; all SC gates green. The commit log is misleading but the verifier can reconcile by reading commit messages. Pattern established for future parallel runs: prefer `git commit --only <path>` over `git add` + `git commit` whenever a sibling executor is active in the same working tree.

## Issues Encountered

- See Deviations section above. No other issues.

## Verification (SC Outputs)

### SC1 — TestClusterNodeSkew_ExternalLoadBalancer_NotWarned

```
=== RUN   TestClusterNodeSkew_ExternalLoadBalancer_NotWarned
=== PAUSE TestClusterNodeSkew_ExternalLoadBalancer_NotWarned
=== CONT  TestClusterNodeSkew_ExternalLoadBalancer_NotWarned
--- PASS: TestClusterNodeSkew_ExternalLoadBalancer_NotWarned (0.00s)
PASS
ok  	sigs.k8s.io/kind/pkg/internal/doctor	0.206s
```

### Source invariant — TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource

```
=== RUN   TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource
=== PAUSE TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource
=== CONT  TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource
--- PASS: TestClusterNodeSkew_realListNodes_LBRoleGuard_PresentInSource (0.00s)
PASS
ok  	sigs.k8s.io/kind/pkg/internal/doctor	0.206s
```

### Count invariant — TestAllChecks_CountIs26

```
=== RUN   TestAllChecks_CountIs26
=== PAUSE TestAllChecks_CountIs26
=== CONT  TestAllChecks_CountIs26
--- PASS: TestAllChecks_CountIs26 (0.00s)
PASS
ok  	sigs.k8s.io/kind/pkg/internal/doctor	0.209s
```

### Race gate (Phase 56 permanent regression guard) — make test-race-doctor

```
CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100
ok  	sigs.k8s.io/kind/pkg/internal/doctor	2.653s
```

- Exit code: 0
- Zero "WARNING: DATA RACE" lines
- 100 iterations green
- Elapsed: 2.653s

### Build + vet

```
go vet ./...       → exit 0
go build ./...     → exit 0
```

### Full doctor suite

```
ok  	sigs.k8s.io/kind/pkg/internal/doctor	0.210s
```

## RESEARCH M6 Note

Per `.planning/phases/57-doctor-cosmetic-fixes/57-RESEARCH.md` mitigation M6 (defensive external-etcd guard for future-proofing), the inline guard at clusterskew.go:118-119 includes BOTH `constants.ExternalLoadBalancerNodeRoleValue` and `constants.ExternalEtcdNodeRoleValue` in a single `||` branch. The constants.go:62 doc-comment notes that external-etcd is "not yet implemented" upstream — but the guard cost is one extra string comparison per container per invocation, which is negligible, and the future-proofing eliminates a class of regression that would otherwise require re-opening DIAG-05 if external-etcd ever lands.

## Threat Flags

None — DIAG-05 is a localized cosmetic fix to a diagnostic check. No new network surface, auth path, file access pattern, or schema change introduced. The new `pkg/cluster/constants` import is a leaf data package containing only string literals (no init side-effects, no behavior).

## Known Stubs

None.

## Next Phase Readiness

- **57-02 (DIAG-06 cluster-resume-readiness raw JSON dump)**: Running in parallel with this plan. The parallel-executor staging race documented in Deviations affects both plans symmetrically; 57-02's source/test changes are preserved in 866f2c22 and c43bb599 (mis-attributed to 57-01 commit hashes). The 57-02 executor's SUMMARY should disclose the inverse contamination.
- **Phase 57 verifier**: When verifying SC1 of the phase ROADMAP, run the two new tests directly + `make test-race-doctor` + `TestAllChecks_CountIs26`. Do NOT use git log to attribute file changes to plans — the commit log is contaminated. Use `git diff 3640b32c..HEAD --stat` to see the union of both plans' work, and the file-level disposition (clusterskew.* belongs to 57-01, resumereadiness.* belongs to 57-02).
- **REQUIREMENTS.md DIAG-05 checkbox flip**: DEFERRED to Phase 57 close per plan output section.

## Self-Check: PASSED

Per `<self_check>` step — verify all claimed files exist and all claimed commits exist:

```
[FOUND] pkg/internal/doctor/clusterskew.go
[FOUND] pkg/internal/doctor/clusterskew_test.go
[FOUND] 866f2c22 test(57-01): add RED tests for DIAG-05 LB-role guard
[FOUND] c43bb599 feat(57-01): inline LB/external-etcd role guard in realListNodes (DIAG-05 GREEN)
[FOUND] 33544309 feat(57-01): apply DIAG-05 LB-role guard to clusterskew.go
```

All artifacts present; SUMMARY claims are verifiable from git log + filesystem.

---
*Phase: 57-doctor-cosmetic-fixes*
*Plan: 01*
*Completed: 2026-05-12*
