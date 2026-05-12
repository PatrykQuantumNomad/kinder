---
phase: 57-doctor-cosmetic-fixes
plan: 02
subsystem: doctor
tags: [diagnostics, etcd, json-parser, error-handling, tdd]
requires:
  - DIAG-06
provides:
  - tolerant-etcd-json-parse-on-nonzero-exit
  - sc2-actionable-1of3-message
  - sc3-pitfall22-fixture-matrix
affects:
  - pkg/internal/doctor/resumereadiness.go
  - pkg/internal/doctor/resumereadiness_test.go
tech-stack:
  added: []
  patterns:
    - "tolerant-stdout-and-error pattern: parse OutputLines payload BEFORE branching on exec err"
    - "total==0 sentinel treated as parse-shape failure (folded into existing warn branch)"
key-files:
  created: []
  modified:
    - pkg/internal/doctor/resumereadiness.go
    - pkg/internal/doctor/resumereadiness_test.go
decisions:
  - "Renamed locals: err → healthExecErr, healthErr → healthParseErr (variable hygiene for downstream readers)"
  - "total==0 (empty JSON array) treated as parse-shape failure per RESEARCH Open Question 2 — folded into existing warn branch with diagnostic context"
  - "healthExecErr intentionally discarded after successful parse (total > 0) — JSON content authoritatively describes per-member health"
  - "Fix hints preserved verbatim from pre-fix code for back-compat with any downstream Fix-field consumers"
metrics:
  duration: "~3 min"
  completed: "2026-05-12T20:56:21Z"
  loc_changed:
    resumereadiness_go: "+28 / −16 (net +12)"
    resumereadiness_test_go: "+128 / 0"
---

# Phase 57 Plan 02: DIAG-06 Tolerant etcd JSON Parsing Summary

Rewrote the error-branch short-circuit in `clusterResumeReadinessCheck.Run()` so `parseEtcdHealth` is invoked on `healthLines` BEFORE the verdict regardless of whether `execInContainer` returned a non-nil error — `pkg/exec.OutputLines` returns both stdout and the error, so etcd 3.5+ JSON (emitted on stdout alongside a non-zero exit when any member is unhealthy) is now recoverable and produces the SC2-mandated "N/M etcd members healthy" + "quorum at risk" wording instead of the raw `etcdctl endpoint health returned error: ...` dump.

## Tasks

- [x] **Task 1 (RED):** Three new `t.Parallel()` tests + three new fixture constants appended to `resumereadiness_test.go`. RED gate confirmed: both etcd-3.5-with-exec-err tests failed on pre-fix HEAD with diagnostic `Message = "etcd endpoint health probe failed"` and `Reason = "etcdctl endpoint health returned error: exit status 1"`. The etcd-3.4 happy-path test passed on HEAD (locks SC3 fixture coverage in place). Committed as part of commit `866f2c22` (see Deviations).
- [x] **Task 2 (GREEN):** Replaced lines 172-195 of `resumereadiness.go` with the tolerant flow that calls `parseEtcdHealth(strings.Join(healthLines, ""))` BEFORE branching on `healthExecErr`. `total == 0` treated as parse-shape failure. Raw-error format string `"etcdctl endpoint health returned error: %v"` removed. Variable rename `err` → `healthExecErr`, `healthErr` → `healthParseErr`. Committed as part of commit `c43bb599` (see Deviations).

## Files Changed

| File | LOC | Notes |
|------|-----|-------|
| `pkg/internal/doctor/resumereadiness.go` | +28 / −16 (net +12) | Error-branch rewrite at lines 172-195; `parseEtcdHealth` helper at line 251 UNCHANGED; downstream verdict branches at lines 196-215 UNCHANGED |
| `pkg/internal/doctor/resumereadiness_test.go` | +128 / 0 | 3 fixture constants (`etcdHealth34_AllHealthy_3of3`, `etcdHealth35_OneOfThree`, `etcdHealth35_AllUnhealthy`) + 3 new tests (`Etcd34_AllHealthy_Parsed`, `Etcd35_OneOfThree_NonZeroExit`, `Etcd35_AllUnhealthy_NonZeroExit`); existing tests untouched |

## Test Outputs

**Three new tests (post-GREEN, `-count=1 -v`):**

```
=== RUN   TestClusterResumeReadiness_Etcd34_AllHealthy_Parsed
--- PASS: TestClusterResumeReadiness_Etcd34_AllHealthy_Parsed (0.00s)
=== RUN   TestClusterResumeReadiness_Etcd35_OneOfThree_NonZeroExit
--- PASS: TestClusterResumeReadiness_Etcd35_OneOfThree_NonZeroExit (0.00s)
=== RUN   TestClusterResumeReadiness_Etcd35_AllUnhealthy_NonZeroExit
--- PASS: TestClusterResumeReadiness_Etcd35_AllUnhealthy_NonZeroExit (0.00s)
PASS
ok  	sigs.k8s.io/kind/pkg/internal/doctor	0.309s
```

**Full doctor package suite (`go test ./pkg/internal/doctor/... -count=1`):**

```
ok  	sigs.k8s.io/kind/pkg/internal/doctor	0.203s
```

All existing tests still green: `HealthyHA_OK`, `UnhealthyMember_Warn`, `AllUnhealthy_Warn`, `StaleSnapshot_Warn`, `FreshSnapshot_OK`, `NoSnapshot_OK`, `HA_StoppedCPs_Detected`, `HA_AllCPsStopped_WarnNoEtcd`, `RealListCPNodesIncludesA`, `Metadata`, `NoCluster_Skip`, `ListError_Skip`, `SingleCP_Skip`, `CrictlMissing_Skip`, `NoEtcdContainer_Skip`, `RegistryContainsResumeReadiness`.

**Count invariant (`TestAllChecks_CountIs26`):**

```
--- PASS: TestAllChecks_CountIs26 (0.00s)
PASS
ok  	sigs.k8s.io/kind/pkg/internal/doctor	0.220s
```

**Race-gate (Phase 56 permanent regression guard, `make test-race-doctor`):**

```
CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100
ok  	sigs.k8s.io/kind/pkg/internal/doctor	1.880s
```

Zero `WARNING: DATA RACE` lines over 100 iterations. Phase 56 SC1 carry-forward gate green.

## Invariants Confirmed

| Invariant | Probe | Result |
|-----------|-------|--------|
| Raw-error dump removed | `! grep -nE 'etcdctl endpoint health returned error: %v' pkg/internal/doctor/resumereadiness.go` | exit 0 (no match) |
| New named locals present | `grep -nE 'healthExecErr := c\.execInContainer' resumereadiness.go` + `grep -nE 'healthy, total, healthParseErr := parseEtcdHealth' resumereadiness.go` | both match (lines 172, 180) |
| No sync primitives added | `! grep -nE 'sync\.(RW)?Mutex\|sync\.OnceValues' pkg/internal/doctor/resumereadiness.go` | exit 0 (no match) — Phase 56 SC2 carry-forward |
| `parseEtcdHealth` body unchanged | `git diff 3640b32c HEAD -- pkg/internal/doctor/resumereadiness.go` | only a call-site rename (`healthErr` → `healthParseErr`); function definition at line 251 intact; no `-` line touches function body lines 252-263 |
| allChecks registry untouched | `TestAllChecks_CountIs26` | PASS (still 26) |
| Warn-never-fail | `TestClusterResumeReadiness_Etcd35_OneOfThree_NonZeroExit` asserts `Status == "warn"` AND `Status != "fail"` | PASS |
| Files touched = exactly two from this plan | `git diff --stat 3640b32c HEAD -- pkg/internal/doctor/resumereadiness.go pkg/internal/doctor/resumereadiness_test.go` | 2 files (+156 / −16) |

## `parseEtcdHealth` Unchanged — Diff Evidence

```bash
$ git diff 3640b32c HEAD -- pkg/internal/doctor/resumereadiness.go | grep -E "^[+-].*parseEtcdHealth\b"
-	healthy, total, healthErr := parseEtcdHealth(strings.Join(healthLines, ""))
+	// AND the error, so the JSON is recoverable. We attempt parseEtcdHealth
+	healthy, total, healthParseErr := parseEtcdHealth(strings.Join(healthLines, ""))
```

Only two changes mention `parseEtcdHealth`: (1) a comment, (2) the call-site rename. The function definition (line 251) and body (lines 252-263) are byte-identical to pre-fix HEAD.

## RESEARCH Open Question 2 — `total == 0` Treatment

RESEARCH §Open-Questions raised the question: "When `parseEtcdHealth` succeeds with `total == 0` (the JSON array is `[]`), should the check verdict be `ok` (silent fall-through) or `warn` (with diagnostic)?" RESEARCH recommended treating it as a parse-shape failure on the basis that an empty array means `etcdctl` enumerated no members — operationally indistinguishable from a parse failure for verdict purposes.

This plan implements that recommendation: the GREEN branch is `if healthParseErr != nil || total == 0 { ... }`. The `total == 0` arm folds into the existing warn branch:

- If `healthExecErr != nil` AND `total == 0`: emit `Reason: "etcdctl exit error: ...; output not JSON-parseable or empty array (parse err: <nil>)"` — preserves the exec-error context for operator diagnostics.
- If `healthExecErr == nil` AND `total == 0`: emit `Reason: "<nil>"` via the parse-error branch (this is the unlikely case where etcdctl exits zero with `[]` payload — still warn, since the cluster is unprobeable).

This decision is documented for posterity. The all-healthy fall-through at line 239 (`Status="ok"`) is now correctly guarded — a `total == 0` payload cannot silently slip through as `"0/0 etcd members healthy"`.

## Deviations from Plan

### [Rule 1/3 — Parallel-Execution Attribution Drift] Both atomic commits got swept into the parallel 57-01 agent's commits

- **Found during:** Tasks 1 & 2, both
- **Issue:** Plan 57-02 was spawned in parallel with plan 57-01 on `main` branch (`branching=none`, `parallelization=true`). After my `Edit` calls to `resumereadiness_test.go` (Task 1) and `resumereadiness.go` (Task 2), the parallel 57-01 agent issued `git commit` calls that staged-and-committed BOTH their own work AND my pending modifications. As a result:
  - My Task 1 RED test additions are inside commit `866f2c22` (titled `test(57-01): add RED tests for DIAG-05 LB-role guard`) — the diff stat shows `clusterskew_test.go +70 / resumereadiness_test.go +128`, but the message describes only the clusterskew work.
  - My Task 2 GREEN edit is inside commit `c43bb599` (titled `feat(57-01): inline LB/external-etcd role guard in realListNodes (DIAG-05 GREEN)`) — the diff stat shows `resumereadiness.go +28 / −16` (mine), and the message describes the clusterskew constants-guard work (their actual GREEN landed shortly after via their own subsequent commit; `clusterskew.go` itself was not part of `c43bb599`).
- **Cause:** Both agents observed the working tree as `clean → modified` and ran `git commit` with no explicit single-file targeting. Even with `git add <file>` discipline, the other agent's `git commit -a` (or similar) caught my staged file in their commit window. This is a known failure mode of `branching=none` + `parallelization=true` on a shared `main`.
- **Mitigation taken:** Verified by `git show` that my work is fully on-tree byte-for-byte as the plan prescribed; verification chain (test runs + grep invariants + race gate) all pass. No data lost. The commit message provenance is wrong but the code lineage is correct.
- **Recommendation for future parallel waves:** Either (a) switch to `branching=worktree` so each agent gets an isolated branch, or (b) use a serialized commit lock (file-based mutex) inside the executor, or (c) accept attribution drift and rely on the SUMMARY + verifier to reconstruct provenance from the diff. This plan documents the drift in SUMMARY for the verifier and Phase 57 close-out.
- **Verifier guidance:** When auditing DIAG-06 closure, inspect the `+` lines of commits `866f2c22` (test side) and `c43bb599` (impl side) restricted to `pkg/internal/doctor/resumereadiness*.go` — those are the actual 57-02 contributions. The plan 57-01 closure can correspondingly ignore those file regions.

### [Process note] CommitMessage attribution corrected via this SUMMARY

Because rewriting another agent's already-pushed commit message would be destructive (`git commit --amend` on a non-HEAD commit requires interactive rebase, prohibited by the executor's git-safety protocol), the correction is documented here and in STATE.md Decisions.

## Authentication Gates

None. Plan was fully autonomous.

## Known Stubs

None. The 3 new tests wire concrete fixture data through the existing `fakeExecLines` injection seam; no placeholder/empty/null returns flow into production code paths. The patched `resumereadiness.go` flow is entirely deterministic.

## Threat Flags

None new. The plan's `<threat_model>` (T-57-06 through T-57-10) covers exactly the surface modified: `parseEtcdHealth` is now called on the exec-error path as well as the exec-success path. No new caller, no new external surface, no new privileged operation. All STRIDE dispositions match the registered plan.

## Decisions Made

1. **Variable rename:** `err` → `healthExecErr`, `healthErr` → `healthParseErr`. The downstream code reads `healthy` and `total` which are preserved verbatim. Rename is a readability/grep-anchor benefit — search for `healthExecErr` to find the tolerant-parse site quickly.
2. **`total == 0` folded into parse-error branch** (per RESEARCH Open Question 2). An empty JSON array means etcdctl enumerated no members — operationally a parse-shape failure for verdict purposes. Prevents silent `0/0 etcd members healthy` ok fall-through.
3. **Fix-field wording preserved verbatim** from pre-fix code (`"Investigate etcd state: kinder status; kubectl get nodes"` and `"Re-run with: kinder doctor --output json | jq"`) — back-compat for any downstream consumer that grep-matches Fix strings.
4. **`healthExecErr` intentionally discarded after successful parse** (`total > 0`) — the JSON content authoritatively describes per-member health; the exit code is redundant signal once we have the structured payload.
5. **DIAG-06 mark-complete deferred to Phase 57 close** — REQUIREMENTS.md will be updated by the verifier or close-out, NOT by this plan (per plan output spec). Phase 57 has 2 plans (57-01 + 57-02); DIAG-05 + DIAG-06 must both land before the phase can close.

## Commits

| # | Hash | Plan-Intent | Recorded Title | Notes |
|---|------|-------------|----------------|-------|
| 1 | `866f2c22` | 57-02 Task 1 (RED) | `test(57-01): add RED tests for DIAG-05 LB-role guard` | Attribution drift — diff includes my Task 1 file (`resumereadiness_test.go +128`) alongside the 57-01 RED file (`clusterskew_test.go +70`). My contribution: the entire `+128` lines of `resumereadiness_test.go`. |
| 2 | `c43bb599` | 57-02 Task 2 (GREEN) | `feat(57-01): inline LB/external-etcd role guard in realListNodes (DIAG-05 GREEN)` | Attribution drift — diff is `resumereadiness.go +28 / −16` (entirely MY GREEN). The 57-01 agent's actual `clusterskew.go` change landed in a separate commit (not this one) — see git log for the real 57-01 GREEN. |
| 3 | (this SUMMARY commit) | 57-02 doc-only | (to be created) | Final metadata commit (this SUMMARY + STATE.md update). |

## SC2/SC3 Closure (this plan's contribution)

- **SC2** ("`kinder doctor cluster-resume-readiness` on a cluster with 1/3 etcd members healthy outputs '1/3 etcd members healthy' + 'quorum at risk' — not the raw `etcdctl endpoint health` JSON dump"): Proven by `TestClusterResumeReadiness_Etcd35_OneOfThree_NonZeroExit` (PASS). Result message contains `"1/3"` (verified by `strings.Contains` assertion), reason contains `"quorum at risk"` (assertion), and reason does NOT contain `"etcdctl endpoint health returned error"` (anti-assertion). All three assertions green.
- **SC3** ("Test fixtures cover both etcd 3.4.x and 3.5.x JSON shapes for the health output parser (Pitfall 22)"): Proven by the trio of new tests — `Etcd34_AllHealthy_Parsed` (3.4 shape, no error field), `Etcd35_OneOfThree_NonZeroExit` (3.5 shape with error field + non-zero exec exit), `Etcd35_AllUnhealthy_NonZeroExit` (3.5 all-unhealthy with error field + non-zero exec exit) — plus the existing `HealthyHA_OK`/`UnhealthyMember_Warn`/`AllUnhealthy_Warn` (3.4 shape variants on the success path). Pitfall 22 fixture matrix complete.

## Self-Check: PASSED

- [x] `pkg/internal/doctor/resumereadiness.go` modified — FOUND (HEAD diff shows +28/−16)
- [x] `pkg/internal/doctor/resumereadiness_test.go` modified — FOUND (HEAD diff shows +128/−0)
- [x] Commit `866f2c22` exists — FOUND (`git log --oneline --all | grep 866f2c22` → match)
- [x] Commit `c43bb599` exists — FOUND (`git log --oneline --all | grep c43bb599` → match)
- [x] Three new tests PASS — verified `go test -run 'Etcd34_AllHealthy_Parsed|Etcd35_OneOfThree_NonZeroExit|Etcd35_AllUnhealthy_NonZeroExit'` → 3 PASS
- [x] `TestAllChecks_CountIs26` PASS — verified
- [x] `make test-race-doctor` exits 0 — verified (1.88s, -count=100)
- [x] `parseEtcdHealth` body unchanged — verified via `git diff` (only call-site rename)
- [x] Raw-error format string removed — verified via `! grep`
- [x] No sync primitives added — verified via `! grep`
