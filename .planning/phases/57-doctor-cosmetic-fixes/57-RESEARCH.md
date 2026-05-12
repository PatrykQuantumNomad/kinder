# Phase 57: Doctor Cosmetic Fixes - Research

**Researched:** 2026-05-12
**Domain:** Go testing in `pkg/internal/doctor`; container role filtering (LB exclusion in HA clusters); defensive JSON parsing of `etcdctl endpoint health --write-out=json`
**Confidence:** HIGH — both bug sites located in source with exact line ranges; etcd JSON schema verified against upstream `etcd-io/etcd` source for both 3.4 and 3.5 release branches; race-free test infrastructure (Phase 56) already in place

---

## 1. Goal Restated

`kinder doctor` produces no false-positive version-skew warnings on HA clusters that include an `external-load-balancer` container, and `cluster-resume-readiness` outputs actionable member-count text (e.g. `"1/3 etcd members healthy, quorum at risk"`) instead of the raw `etcdctl endpoint health` error string. Both fixes are localized inside `pkg/internal/doctor/`. Two plans expected: **57-01** patches `clusterskew.go` (`realListNodes` LB role guard); **57-02** patches `resumereadiness.go` (parse JSON from `OutputLines` stdout even when the underlying command exits non-zero).

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DIAG-05 | `cluster-node-skew` doctor check skips containers with role `external-load-balancer` (no `/kind/version` file present); `ListInternalNodes`-style role filter applied | `clusterskew.go` `realListNodes` already reads the `io.x-k8s.kind.role` label per container (line 99); the role string for LB containers is `external-load-balancer` (verified in `pkg/cluster/constants/constants.go:57` — `ExternalLoadBalancerNodeRoleValue`). LB containers (haproxy) have no `/kind/version`, so the `docker exec ... cat /kind/version` at line 113 fails with a non-nil `VersionErr` that the `Run()` method at lines 171-178 currently converts into a `warn`. `nodeutils.InternalNodes` (`pkg/cluster/nodeutils/roles.go:47`) is the canonical filter (worker + control-plane only) — but it lives in the `cluster` package which cannot be imported from `doctor` (import-cycle comment at `clusterskew.go:67`). Therefore the fix is an inline role guard inside `realListNodes` before the `cat /kind/version` exec, matching the inline pattern already used. |
| DIAG-06 | `cluster-resume-readiness` doctor check parses `etcdctl endpoint health --write-out json` output and reports actionable reason text instead of dumping raw etcdctl error | `resumereadiness.go` already has a working JSON parser (`parseEtcdHealth`, line 251) that handles `[{endpoint, health, took}]`. The bug is at lines 172-184: when `c.execInContainer(...)` returns a non-nil `err` (etcd 3.5+ exits non-zero when ANY member is unhealthy), the code short-circuits with `Reason: fmt.Sprintf("etcdctl endpoint health returned error: %v", err)` — dumping the combined exec error string. `pkg/exec.OutputLines` (`helpers.go:78`) returns BOTH stdout-lines AND the error, so `healthLines` already contains the JSON; the fix is to attempt `parseEtcdHealth` on the captured lines even when `err != nil` and synthesize the actionable text from the parsed result, falling back to the current generic warn only if the parse itself fails. Upstream `etcd-io/etcd` source confirms the JSON schema is `endpoint`/`health`/`took`/`error,omitempty` (lowercase tags) in both `release-3.4` and `release-3.5` of `etcdctl/ctlv3/command/ep_command.go` — the schemas are identical, so a single tolerant parser path covers both. |

---

## 2. Codebase Map (exact file paths and line ranges)

### 2a. cluster-node-skew (DIAG-05)

| Path | Lines | Role |
|------|-------|------|
| `pkg/internal/doctor/clusterskew.go` | 50-54 | `newClusterNodeSkewCheck()` — wires `realListNodes` |
| `pkg/internal/doctor/clusterskew.go` | 67-131 | `realListNodes` — discovers containers, reads role + image + `/kind/version` per container — **the LB false-positive originates here** at the inner loop (lines 89-130) when `cat /kind/version` is exec'd against an LB container |
| `pkg/internal/doctor/clusterskew.go` | 99 | exact `inspect` line that reads `io.x-k8s.kind.role` label — `role` variable already in scope by line 105 |
| `pkg/internal/doctor/clusterskew.go` | 113-120 | `cat /kind/version` exec — guard inserts BEFORE this block when `role == "external-load-balancer"` |
| `pkg/internal/doctor/clusterskew.go` | 148-282 | `Run()` — consumes `nodeEntry` slice; processes `VersionErr` at lines 171-178 → emits the false `warn` for LB |
| `pkg/internal/doctor/clusterskew_test.go` | 30-36 | `newTestClusterNodeSkewCheck(entries, listErr)` — injection point — tests build `[]nodeEntry` directly, **bypassing `realListNodes`** entirely |
| `pkg/internal/doctor/clusterskew_test.go` | 38-288 | Existing tests; all use `t.Parallel()`, all use the static-entries injection; no test currently asserts LB-role behavior |

### 2b. cluster-resume-readiness (DIAG-06)

| Path | Lines | Role |
|------|-------|------|
| `pkg/internal/doctor/resumereadiness.go` | 60-66 | `newClusterResumeReadinessCheck()` — wires `realExecInContainer` etc. |
| `pkg/internal/doctor/resumereadiness.go` | 100-245 | `Run()` — the bug branch is lines 172-184 |
| `pkg/internal/doctor/resumereadiness.go` | **172-184** | **THE BUG** — when `c.execInContainer` returns err, Reason dumps raw `%v` error |
| `pkg/internal/doctor/resumereadiness.go` | 185-215 | Healthy path: `parseEtcdHealth` is called on stdout; already produces `"N/M etcd members healthy"` and `"X unhealthy etcd member(s) — quorum at risk"` text — this is the format SC2 wants |
| `pkg/internal/doctor/resumereadiness.go` | 251-263 | `parseEtcdHealth` — uses `[]map[string]interface{}` (already case-tolerant for `"health"` key) |
| `pkg/internal/doctor/resumereadiness_test.go` | 27-79 | `fakeReadinessOpts` + `newFakeResumeReadinessCheck` — full injection surface (listNodes, execInContainer, inspectState, snapshot) |
| `pkg/internal/doctor/resumereadiness_test.go` | 195-215 | `healthyEtcdJSON(n)` / `statusEtcdJSON(leader, n)` fixture helpers |
| `pkg/internal/doctor/resumereadiness_test.go` | 248-310 | `TestClusterResumeReadiness_UnhealthyMember_Warn` / `_AllUnhealthy_Warn` — existing fixtures for the no-error path; will need a **new sibling** where `execInContainer` returns the same JSON AND a non-nil err to exercise the fixed parser |
| `pkg/exec/helpers.go` | 78-87 | `OutputLines(cmd Cmd) (lines []string, err error)` — returns BOTH stdout-lines AND err. **This is why partial JSON is recoverable on non-zero exit.** |

### 2c. Shared infra (Phase 56 baseline)

| Path | Lines | Role |
|------|-------|------|
| `pkg/internal/doctor/check.go` | 51-91 | `allChecks` registry — 26 entries; both checks already registered (`newClusterNodeSkewCheck` line 82, `newClusterResumeReadinessCheck` line 84) |
| `pkg/internal/doctor/check.go` | 122-149 | `RunAllChecks()` + `runChecks(checks []Check) []Result` — Phase 56's race-free helper |
| `pkg/internal/doctor/check_test.go` | 229-240 | `TestAllChecks_CountIs26` — the count-pin test. **Verified live: `go test -run TestAllChecks_CountIs26 ./pkg/internal/doctor/` passes; current count = 26.** Phase 57 must NOT add new checks (both DIAG-05 and DIAG-06 are modifications), so count stays 26. |
| `Makefile` | 87-92 | `test-race-doctor` target — `CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100` |
| `.github/workflows/race-check.yml` | — | PR regression guard — runs `test-race-doctor` on every PR |

---

## 3. DIAG-05 Implementation Sketch (Plan 57-01)

**Locked decision (Pitfall 21 + ARCHITECTURE.md §8a):** Inline LB-role guard inside `realListNodes` is the canonical fix. Cannot use `nodeutils.InternalNodes` directly because of the import cycle (`cluster` package imports `doctor`).

**Proposed change to `pkg/internal/doctor/clusterskew.go` (~5 LOC):**

After the `inspect` block parses `role` and `image` (around line 109), insert before the `cat /kind/version` exec block (current line 113):

```go
// External load-balancer containers (haproxy/envoy) have no /kind/version;
// they are not Kubernetes nodes and must be excluded from version-skew
// comparisons. Matches nodeutils.InternalNodes filter semantics (which
// cannot be imported here due to a cycle — see realListNodes comment).
if role == constants.ExternalLoadBalancerNodeRoleValue {
    entries = append(entries, nodeEntry{Name: name, Role: role, Image: image})
    continue
}
```

The `Run()` method already handles this correctly downstream — it iterates `cpEntries` (role==control-plane) and workers (role==worker) explicitly; LB entries with empty `Version`/`VersionErr` are now harmless because the `VersionErr != nil` short-circuit at lines 171-178 is never triggered for LB.

**Import addition:** add `"sigs.k8s.io/kind/pkg/cluster/constants"` to `clusterskew.go`. Verified zero-cycle: `pkg/cluster/constants` is a leaf package (only stdlib imports — see file inspection). The HA-resume-strategy check (`resumestrategy.go`, plan 52-04) already imports it from the doctor package — established precedent.

**Defensive role guard (recommended):** Also skip the `external-etcd` role (`constants.ExternalEtcdNodeRoleValue`) for the same reason — it's another non-Kubernetes node type. Marked "not yet implemented" in `constants.go:62`, but adding the guard now costs nothing and prevents future regression if it ever ships.

**Test fixture (`clusterskew_test.go`):** Add a new `t.Parallel()` test using the existing `newTestClusterNodeSkewCheck(entries, nil)` injection — entries:
```go
{Name: "kind-control-plane",    Role: "control-plane",        Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
{Name: "kind-control-plane2",   Role: "control-plane",        Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
{Name: "kind-control-plane3",   Role: "control-plane",        Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
{Name: "kind-worker",           Role: "worker",               Version: "v1.31.2", Image: "kindest/node:v1.31.2"},
{Name: "kind-external-load-balancer", Role: "external-load-balancer", Version: "", Image: "kindest/haproxy:v..." , VersionErr: errors.New("cat: /kind/version: No such file or directory")},
```
Assert `Status == "ok"` AND `Message` does NOT contain `"kind-external-load-balancer"`. This test exercises the existing `Run()` injection (does not exercise `realListNodes` directly).

**Optional second test:** Exercise `realListNodes` via a refactor that takes the role-filter decision as a strategy parameter (or — simpler — leave `realListNodes` untested as today; the static-entries injection in `Run()` covers the regression). Recommendation: skip the second test — the bug is purely in `realListNodes`'s `cat /kind/version` step, which is a side effect; the regression test at the `Run()` level proves the fix because the new `realListNodes` behavior is to NOT produce a `VersionErr` entry for LB.

---

## 4. DIAG-06 Implementation Sketch (Plan 57-02)

**Locked decision (Pitfall 22 + FEATURES.md §"cluster-resume-readiness Reason Text Parsing"):** Parse JSON from `OutputLines` stdout even when the underlying exec returns a non-zero exit status. The `parseEtcdHealth` function already produces the actionable text; the fix is in the call-site error branch.

**Proposed change to `pkg/internal/doctor/resumereadiness.go` (~15 LOC):**

Replace lines 172-184 (the `if err != nil` short-circuit on `c.execInContainer`) with a tolerant flow that:

1. Captures BOTH `healthLines` and `err`.
2. Always attempts `parseEtcdHealth(strings.Join(healthLines, ""))`.
3. If parse succeeds (returns `(healthy, total, nil)` with `total > 0`): fall through into the existing healthy/unhealthy/zero-healthy branches at lines 196-215. These ALREADY emit `"%d/%d etcd members healthy"` + `"%d unhealthy etcd member(s) — quorum at risk"` — exactly what SC2 requires.
4. If parse fails AND `err != nil`: return the existing generic-warn result (preserves today's safety net).
5. If parse fails AND `err == nil`: return today's `"could not parse etcd health output"` warn (no change).

Concrete diff shape:

```go
healthLines, healthErrExec := c.execInContainer(binaryName, bootstrap, healthArgs...)
healthy, total, parseErr := parseEtcdHealth(strings.Join(healthLines, ""))

if parseErr != nil {
    // Parse failed. If exec also failed, surface the original error context.
    if healthErrExec != nil {
        return []Result{{
            Name:     c.Name(),
            Category: c.Category(),
            Status:   "warn",
            Message:  "etcd endpoint health probe failed",
            Reason:   fmt.Sprintf("etcdctl exit error: %v; output not JSON-parseable: %v", healthErrExec, parseErr),
            Fix:      "Investigate etcd state: kinder status; kubectl get pods -n kube-system",
        }}
    }
    return []Result{{
        Name:     c.Name(),
        Category: c.Category(),
        Status:   "warn",
        Message:  "could not parse etcd health output",
        Reason:   parseErr.Error(),
        Fix:      "Re-run with: kinder doctor --output json | jq",
    }}
}

// Parse succeeded. Fall through to the existing healthy/unhealthy/zero branches
// using healthy/total — they already produce "N/M etcd members healthy" + quorum text.
if healthy == 0 { ... }
if healthy < total { ... }
```

The existing line 196-215 block then runs unchanged.

**Quorum boundary clarification:** the existing code's verdict matrix is already correct per CONTEXT.md "warn and continue":

| Total | Healthy | Quorum check `healthy > total/2` | Current verdict | Keep? |
|-------|---------|----------------------------------|-----------------|-------|
| 3 | 3 | OK | ok ("3/3 etcd members healthy") | yes |
| 3 | 2 | OK | warn ("2/3 etcd members healthy", reason "1 unhealthy etcd member(s) — quorum at risk") | yes — but reason is technically inaccurate (2/3 = quorum intact). **Cosmetic improvement (optional):** distinguish "quorum intact" vs "quorum at risk" by computing `healthy > total/2`. SC2 example text is "1/3 healthy, quorum at risk" — the 2/3 case is not specified by SC2. **Recommendation: keep current wording or improve in a follow-up; SC2 only mandates the 1/3 case.** |
| 3 | 1 | LOST | warn ("1/3 etcd members healthy", reason "2 unhealthy etcd member(s) — quorum at risk") | yes — matches SC2 verbatim modulo word order |
| 3 | 0 | LOST | warn ("0/3 etcd members healthy", reason "no healthy etcd members reachable; quorum lost") | yes |

**The phase-spec note "fail when zero healthy" is NOT supported by CONTEXT.md** — `resumereadiness.go:96-99` (and existing test `TestClusterResumeReadiness_AllUnhealthy_Warn`) explicitly require **never fail** ("warn-and-continue per CONTEXT.md"). Keep this invariant; do not promote zero-healthy to fail.

**Recommended Result/Verdict constants:** The doctor package uses string constants for Status: `"ok"`, `"warn"`, `"fail"`, `"skip"` (defined inline at `check.go:42-49`). No typed enum exists; keep using string literals matching existing code.

**Test fixtures (resumereadiness_test.go):** add three new `t.Parallel()` tests using the existing `fakeReadinessOpts` + `fakeExecLines` injection.

| Test name | Setup | Assertion |
|-----------|-------|-----------|
| `TestClusterResumeReadiness_UnhealthyMember_NonZeroExit_Warn` | `fakeExecLines{lines: []string{mixed1of3JSON}, err: errors.New("exit status 1")}` (etcd 3.5 unhealthy-exit shape) | `Status == "warn"`, `Message` contains `"1/3"`, `Reason` contains `"quorum at risk"` (NOT `"exit status 1"` raw dump) |
| `TestClusterResumeReadiness_HealthOutputContains3_4Shape_Parsed` | `fakeExecLines{lines: []string{healthyEtcdJSON34Shape(3)}, err: nil}` — etcd 3.4 JSON shape (lowercase tags, no `error` field) | `Status == "ok"`, `Message` contains `"3/3"` |
| `TestClusterResumeReadiness_HealthOutputContains3_5ShapeWithError_Parsed` | `fakeExecLines{lines: []string{mixedJSONWithErrorField}, err: errors.New("exit 1")}` — etcd 3.5 mixed shape with `"error":"context deadline exceeded"` in unhealthy entries | `Status == "warn"`, `Message` contains `"2/3"` (or "1/3" depending on fixture), `Reason` contains `"quorum at risk"` |

**Fixture content for both etcd versions** (Pitfall 22 coverage; both are valid etcd JSON per upstream source):

```go
// etcd 3.4.x: error field omitted (omitempty) when entry healthy
const etcdHealth3_4_AllHealthy = `[
  {"endpoint":"https://127.0.0.1:2379","health":true,"took":"1.2ms"},
  {"endpoint":"https://10.0.0.2:2379","health":true,"took":"1.5ms"},
  {"endpoint":"https://10.0.0.3:2379","health":true,"took":"1.1ms"}
]`

// etcd 3.5.x: error field present on unhealthy entries
const etcdHealth3_5_OneOfThree = `[
  {"endpoint":"https://127.0.0.1:2379","health":true,"took":"1.2ms"},
  {"endpoint":"https://10.0.0.2:2379","health":false,"took":"5s","error":"context deadline exceeded"},
  {"endpoint":"https://10.0.0.3:2379","health":false,"took":"5s","error":"connection refused"}
]`
```

Both shapes parse cleanly through the existing `parseEtcdHealth` because it uses `[]map[string]interface{}` and only reads the `"health"` key. **Pitfall 22's claim of capitalized JSON keys is not supported by upstream source for either 3.4 or 3.5** — the `epHealth` struct in both `release-3.4` and `release-3.5` of `etcd-io/etcd` uses lowercase JSON tags (`endpoint`/`health`/`took`/`error`). Pitfall 22's defensive intent stands, but the implementation does NOT require a custom UnmarshalJSON — `[]map[string]interface{}` is already case-sensitive on the lowercase keys, which is correct.

---

## 5. Pitfall 22 Fixture Coverage Matrix

| Fixture | etcd version | Member count | Healthy | Exec err | Expected Status | Expected Message contains | Expected Reason contains |
|---------|--------------|-------------:|--------:|---------|-----------------|---------------------------|--------------------------|
| `etcdHealth3_4_AllHealthy` | 3.4.x | 3 | 3 | nil | `ok` | `3/3 etcd members healthy` | (empty) |
| `etcdHealth3_4_OneOfThree` (no error field; takes=0s on unhealthy) | 3.4.x | 3 | 1 | nil OR `exit 1` | `warn` | `1/3` | `quorum at risk` |
| `etcdHealth3_5_AllHealthy` (lowercase + no error field) | 3.5.x | 3 | 3 | nil | `ok` | `3/3 etcd members healthy` | (empty) |
| `etcdHealth3_5_OneOfThree` (with `"error":"context deadline exceeded"`) | 3.5.x | 3 | 1 | `exit 1` (non-zero from etcdctl) | `warn` | `1/3` | `quorum at risk` |
| `etcdHealth3_5_AllUnhealthy` | 3.5.x | 3 | 0 | `exit 1` | `warn` | `0/3` | `no healthy etcd members` OR `quorum lost` |
| Empty JSON array `[]` | both | 0 | 0 | nil | `warn` (current behavior — `total == 0` falls through to the `healthy < total` branch which is false, then hits the `all healthy` branch which returns ok with "0/0"). **Open Question:** is this acceptable, or should `total == 0` be its own warn? Recommendation: add explicit guard `if total == 0 { return warn "could not enumerate etcd members" }`. |
| Malformed JSON (e.g. `"not json"`) | n/a | n/a | n/a | nil OR exit-err | `warn` | `could not parse etcd health output` (when exec succeeded) OR `etcdctl exit error: ...; output not JSON-parseable: ...` (when exec failed) | — |

**Minimum required for SC3:** rows 1, 3, 4 (covers 3.4 healthy, 3.5 healthy, 3.5 degraded-with-error-field). Rows 2, 5, 6, 7 are recommended for completeness.

---

## 6. Wave + Dependency Recommendation

**Recommendation: 57-01 and 57-02 run in PARALLEL (same wave).**

Rationale:
- **57-01 touches:** `pkg/internal/doctor/clusterskew.go` + `pkg/internal/doctor/clusterskew_test.go` only.
- **57-02 touches:** `pkg/internal/doctor/resumereadiness.go` + `pkg/internal/doctor/resumereadiness_test.go` only.
- **Shared file risk:** none. The two checks are independent — `clusterNodeSkewCheck` and `clusterResumeReadinessCheck` do not share helpers, do not share global state, do not share constants. Both consume `pkg/cluster/constants` separately (57-01 adds the import; 57-02 already uses it via shared etcdctl arg patterns).
- **Count test (`TestAllChecks_CountIs26`):** neither plan adds a new entry to `allChecks`. The count test is shared, but both plans should leave it unchanged. Verified live: count is currently 26.
- **Test infrastructure (`runChecks` helper from Phase 56):** both plans inherit it; neither plan changes `check.go`. No write conflict.
- **Race test (`make test-race-doctor`):** both plans MUST pass it as a per-plan gate (Phase 56 SC1 is a permanent regression gate). Each plan's new tests use the existing static-injection (`newTestClusterNodeSkewCheck` / `newFakeResumeReadinessCheck`) which the Phase 56 audit confirmed parallel-safe; no `t.Parallel()` discipline issue.

If the planner prefers strict sequencing for safety, the order is irrelevant — pick either. The roadmap says "Plans: TBD (2 plans expected: 57-01 cluster-node-skew LB guard; 57-02 cluster-resume-readiness JSON parsing)" with no implied ordering.

**Confidence: HIGH** that parallel is safe.

---

## 7. must_haves derived from phase goal

Each must_have is tagged with the SC it satisfies. The planner should propagate these verbatim into the plan must_haves lists.

**Plan 57-01 (DIAG-05 — cluster-node-skew LB guard):**

- [M1, SC1] `realListNodes` in `pkg/internal/doctor/clusterskew.go` MUST skip the `cat /kind/version` exec for containers whose `io.x-k8s.kind.role` label equals `constants.ExternalLoadBalancerNodeRoleValue`.
- [M2, SC1] LB-role entries appended to the returned `[]nodeEntry` slice MUST have `VersionErr == nil` (so `Run()` does not emit a false-positive warn at `clusterskew.go:171-178`).
- [M3, SC1] A new `t.Parallel()` test in `clusterskew_test.go` MUST exercise a mixed entry slice (3 CP + 1 worker + 1 LB) via `newTestClusterNodeSkewCheck` and assert `Status == "ok"` AND `Message` does NOT contain the LB container name.
- [M4, SC3-adjacent] The patch MUST NOT introduce any new entry into `allChecks` (count test remains pinned at 26 — `TestAllChecks_CountIs26`).
- [M5, race gate] `make test-race-doctor` MUST exit 0 after the patch (`CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100`).
- [M6, optional but recommended] Apply the same guard for `constants.ExternalEtcdNodeRoleValue` so future external-etcd containers do not regress this fix.

**Plan 57-02 (DIAG-06 — cluster-resume-readiness JSON parsing):**

- [M1, SC2] In `Run()` of `clusterResumeReadinessCheck`, the path at `resumereadiness.go:172-184` MUST attempt `parseEtcdHealth(strings.Join(healthLines, ""))` BEFORE short-circuiting on `err`. When parse succeeds (total > 0), the result MUST fall through into the existing healthy/unhealthy/zero branches that emit `"%d/%d etcd members healthy"` and `"%d unhealthy etcd member(s) — quorum at risk"`.
- [M2, SC2] When the etcd 3.5 unhealthy-member case produces stdout JSON AND a non-zero exit from `etcdctl`, the resulting `Result.Message` MUST contain `"N/M etcd members healthy"` (e.g. `"1/3 etcd members healthy"`) — NOT `"etcd endpoint health probe failed"` and NOT the raw `etcdctl exit error: ...` string.
- [M3, SC3] `resumereadiness_test.go` MUST include AT LEAST three new tests covering:
  - etcd 3.4 healthy JSON (no `error` field on entries, exec err == nil) → `ok`, `"3/3"`
  - etcd 3.5 mixed JSON with `"error":"..."` field on unhealthy entries, exec err != nil → `warn`, `"1/3"`, reason `"quorum at risk"`
  - etcd 3.5 all-unhealthy JSON (3 entries with `"error":...`, exec err != nil) → `warn`, `"0/3"`, reason mentions `"no healthy etcd members"` OR `"quorum lost"`
- [M4, SC2 invariant] `Status` MUST NEVER be `"fail"` (CONTEXT.md "warn and continue per resumereadiness.go:96-99" — also enforced by existing tests).
- [M5, count test] The patch MUST NOT introduce any new entry into `allChecks` (count test remains pinned at 26).
- [M6, race gate] `make test-race-doctor` MUST exit 0 after the patch.
- [M7, defensive parsing] The `parseEtcdHealth` parser MUST remain case-sensitive on the lowercase `"health"` JSON key (matches upstream etcd 3.4/3.5 `epHealth` struct tags); do NOT add a custom UnmarshalJSON path unless a real reproduction of capitalized keys surfaces (current upstream source contradicts Pitfall 22's claim).

---

## 8. Open Questions

1. **Quorum-intact wording at 2/3 healthy.** SC2's example mandates `"1/3 healthy, quorum at risk"`. The current code at `resumereadiness.go:206-215` emits `"2/3 etcd members healthy"` + reason `"1 unhealthy etcd member(s) — quorum at risk"`. Strictly, 2/3 is quorum **intact** (since `2 > 3/2 = 1`). Should the wording be improved to distinguish "quorum intact" (2/3) vs "quorum at risk" (1/3) vs "quorum lost" (0/3)? FEATURES.md §"Reason Text Parsing" suggests three distinct strings; SC2 only requires the at-risk case. **Recommendation:** lock in the three-tier wording in 57-02 since it costs ~5 LOC, improves user signal, and is described in research already. Risk: changing the existing `TestClusterResumeReadiness_UnhealthyMember_Warn` assertion (line 277 `Reason contains "unhealthy"`) — the new wording must still match.

2. **Empty JSON array `[]` handling.** If `etcdctl` returns `[]` (zero members enumerated) and `total == 0`, the current code flows past both the `healthy == 0` (false: `0 == 0` is true → warn "0/0 etcd members healthy") and `healthy < total` (false) branches into the final "all healthy" branch. **Need to confirm** whether `healthy == 0 && total == 0` should be a distinct warn ("could not enumerate etcd members") or fall through to ok. Recommendation: add explicit guard before the `healthy == 0` check: `if total == 0 { warn "etcd reported zero members; cannot assess quorum" }`. Low-cost.

3. **Should DIAG-05 also exclude `external-etcd` role?** `pkg/cluster/constants/constants.go:62` defines `ExternalEtcdNodeRoleValue = "external-etcd"` but is marked "not yet implemented". Adding the guard is defensive and free. **Recommendation:** include it (M6 above). If the planner disagrees, it's a one-line removal — no semantic change today.

4. **Should `realListNodes` itself be unit-tested for the LB skip?** Currently `realListNodes` is untested directly because tests inject via `listNodes` closure. Adding direct coverage requires a `cmder`-style injection for `exec.Command`. **Recommendation:** skip — the static-entries test at the `Run()` level adequately proves the regression is fixed. Adding `cmder` injection is out of scope for a cosmetic fix and would expand the diff substantially.

5. **`Fix` field wording.** The current Fix field on the bug path (line 182) says `"Investigate etcd state: kinder status; kubectl get nodes"`. For the new parsed-result warn branches, the Fix should likely be the existing line 203/213 wording: `"Investigate etcd state: kinder status; kubectl get pods -n kube-system"`. No genuine ambiguity here — just confirm 57-02 uses the existing wording.

---

## 9. Risks / Gotchas

| Risk | Mitigation |
|------|------------|
| **`TestAllChecks_CountIs26` regression** | Neither plan adds new checks. If a planner accidentally introduces `newSomethingCheck()` into `allChecks`, the count test fails. Verified live count = 26. Recommend: both plan SC blocks include "AllChecks count remains 26" as an explicit hold-point. |
| **`make test-race-doctor` regression (Phase 56 SC1 permanent gate)** | Both new tests use the existing parallel-safe injection (`newTestClusterNodeSkewCheck` / `newFakeResumeReadinessCheck`). Neither plan touches `check.go`, neither plan mutates `allChecks`. Each plan's verify step MUST run `make test-race-doctor` and assert exit 0. Per Phase 56 STATE.md decisions, no new globals may be introduced; if one is needed, it must NOT have a `t.Parallel()` consumer. |
| **`t.Parallel()` discipline** | Phase 56's pattern: parallel tests pass local `[]Check{...}` to `runChecks(...)` rather than mutating `allChecks`. Neither 57-01 nor 57-02 needs `runChecks` (their tests inject at the check-struct level, not at the registry level). No package-level globals are introduced by either plan. |
| **Import cycle from `pkg/cluster/constants`** | Verified zero-risk: `pkg/cluster/constants/constants.go` is a leaf (zero imports of kinder-internal packages). `resumestrategy.go` already imports it from `doctor`. |
| **Existing `TestClusterResumeReadiness_UnhealthyMember_Warn` assertion drift** | Existing test (line 277) asserts `Reason contains "unhealthy"`. If 57-02 reworks reason wording (Open Question 1), this test must still pass. Choose wording that contains both `"unhealthy"` (for back-compat with current test) AND `"quorum"` (for SC2). Existing wording `"%d unhealthy etcd member(s) — quorum at risk"` already does both. |
| **Reason field length** | The new tolerant parser may produce longer reason text on degraded clusters; ensure no consumer truncates at <120 chars. Current `format.go` in the doctor package wraps appropriately (existing pattern — no change needed). |
| **`OutputLines` returning partial buffer on error** | Confirmed: `pkg/exec/helpers.go:78-87` returns `lines` AND `err`. The buffer is scanned regardless of `err`, so partial JSON from etcdctl stdout is recoverable. This is the foundation of the fix. |
| **`etcdctl --cluster` exit semantics** | etcd 3.5+ etcdctl exits non-zero when `--cluster` is set and ANY member is unhealthy, but still writes JSON to stdout for all members. This is the exact failure mode this phase fixes. Verified by Pitfall 22 description + upstream source review. |
| **No live cluster needed for tests** | All fixtures are string constants; both plans verifiable with `go test ./pkg/internal/doctor/...`. No `kinder create cluster` smoke required for unit verification. SC1 mentions "3-CP HA cluster" — interpret as the test assertion, not a live UAT requirement. Live verification is deferred to Phase 58 (HA UAT smoke). |

---

## 10. Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (Go 1.26.3 — see `go.mod`) |
| Config file | none (Go default `go test`) |
| Quick run command | `go test ./pkg/internal/doctor/ -run '57'` (after adding `57` substring to new test names) — or `go test ./pkg/internal/doctor/ -run 'ClusterNodeSkew\|ClusterResumeReadiness' -count=1` |
| Full suite command | `go test ./pkg/internal/doctor/...` |
| Race gate (permanent, from Phase 56) | `make test-race-doctor` → `CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DIAG-05 | LB role entries are skipped — no version-skew warn for LB | unit | `go test ./pkg/internal/doctor/ -run TestClusterNodeSkew_LBContainer -count=1` | ❌ new — `clusterskew_test.go` |
| DIAG-06 | etcd 3.4 healthy → `ok 3/3` | unit | `go test ./pkg/internal/doctor/ -run TestClusterResumeReadiness_Etcd34_AllHealthy -count=1` | ❌ new — `resumereadiness_test.go` |
| DIAG-06 | etcd 3.5 1/3 healthy + exec err → `warn 1/3 quorum at risk` | unit | `go test ./pkg/internal/doctor/ -run TestClusterResumeReadiness_Etcd35_OneOfThree_NonZeroExit -count=1` | ❌ new |
| DIAG-06 | etcd 3.5 0/3 healthy → `warn 0/3 quorum lost` | unit | `go test ./pkg/internal/doctor/ -run TestClusterResumeReadiness_Etcd35_AllUnhealthy_NonZeroExit -count=1` | ❌ new |
| Phase 56 carry-forward | race-free under `-race -count=100` | regression gate | `make test-race-doctor` | ✅ exists |
| Phase 52 carry-forward | `TestAllChecks_CountIs26` stays green | invariant | `go test ./pkg/internal/doctor/ -run TestAllChecks_CountIs26 -count=1` | ✅ exists |

### Sampling Rate
- **Per task commit:** `go test ./pkg/internal/doctor/ -run '<new-test-pattern>' -count=1` (sub-2-second)
- **Per plan merge:** `go test ./pkg/internal/doctor/...` + `make test-race-doctor`
- **Phase gate:** Full suite green + race gate green before `/gsd-verify-work`

### Wave 0 Gaps
- None. All test infrastructure (fakes, fixture helpers, count test, race gate) is in place from prior phases. Each plan adds new test functions to existing test files; no new test files, no new fixtures dirs needed.

---

## 11. Project Constraints (from CLAUDE.md)

No CLAUDE.md present at repo root (verified). The applicable constraints derive from STATE.md decisions and Phase 56 outcomes:

- **No `sync` primitives in production read paths** (Phase 56 SC2 — production `RunAllChecks` remains lock-free). Phase 57 has zero need for sync — keep it lock-free.
- **No new package-level globals in test files** (Phase 56 STATE.md decision — globals interact badly with `t.Parallel()`; use struct-field injection instead). Both 57-01 and 57-02 use existing struct-field injection patterns.
- **`make test-race-doctor` is a permanent regression gate** (Phase 56 SC1) — every Phase 57 plan must pass it.
- **Count test discipline** (52-01 STATE decision) — `TestAllChecks_CountIs26` must be updated only when checks are added/removed. Phase 57 is modify-only; count stays 26.

---

## 12. Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All builds/tests | ✓ | go1.26.3 darwin/arm64 (per Phase 56 probe) | — |
| `go test -race` (CGO) | `make test-race-doctor` | ✓ | bundled with Go | — |
| Live HA cluster (3-CP) | SC1 live verification (optional — UAT) | n/a | — | Unit tests with injected fakes cover the regression; live UAT deferred to Phase 58 |
| etcd binary | n/a (only used inside container) | n/a | — | Test fixtures are JSON strings; no real etcd needed |

No new external dependencies introduced by Phase 57.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Discover cluster containers and read per-container metadata | `realListNodes` (clusterskew.go) / `realListCPNodes` (resumereadiness.go) — both inline in `pkg/internal/doctor` | — | Cannot import `pkg/cluster` from `pkg/internal/doctor` (import cycle). Both checks reach the runtime via `pkg/exec` directly. |
| Filter Kubernetes nodes from non-K8s nodes (LB/external-etcd) | `realListNodes` inline role guard (NEW in 57-01) | `nodeutils.InternalNodes` is the canonical filter but unreachable from doctor — must mirror inline | Same import-cycle constraint as above. Constants imported from `pkg/cluster/constants` (leaf package). |
| Parse `etcdctl endpoint health --write-out=json` | `parseEtcdHealth` (resumereadiness.go:251) — already exists; unchanged by 57-02 | — | Pure function; `[]map[string]interface{}` is case-correct for the lowercase JSON tags in both etcd 3.4 and 3.5 |
| Decide pass/warn/fail from healthy-count | `Run()` branches at resumereadiness.go:196-215 — already correct; 57-02 only changes the call-site to reach these branches even when exec exits non-zero | — | Verdict matrix locked by CONTEXT.md: warn-and-continue, never fail |
| Race-free test execution | `runChecks(checks []Check)` helper in check.go (Phase 56) — Phase 57 inherits, does not change | — | Both 57-01 and 57-02 tests inject at the check-struct field level, never at the `allChecks` package-global level |
| Race regression CI gate | `make test-race-doctor` + `.github/workflows/race-check.yml` (Phase 56) — Phase 57 must pass | — | Permanent gate per Phase 56 SC1 |

---

## Standard Stack (no new dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/json` | stdlib | Parse etcdctl JSON output | Already used in `resumereadiness.go`; correctly handles lowercase tags |
| `sigs.k8s.io/kind/pkg/exec` | in-repo | Run container exec commands with stdout+err return | Already used; `OutputLines` returns BOTH lines AND err, enabling the partial-parse fix |
| `sigs.k8s.io/kind/pkg/cluster/constants` | in-repo (leaf) | Role string constants (`ExternalLoadBalancerNodeRoleValue` etc.) | Leaf package — no import cycle from doctor; already used by `resumestrategy.go` |
| `testing` (stdlib) | stdlib | Test framework | Existing; all tests use `t.Parallel()` correctly via struct injection |

No external dependencies added by Phase 57.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| etcdctl JSON unmarshalling | A custom UnmarshalJSON for case-insensitivity | The existing `[]map[string]interface{}` parser at `parseEtcdHealth:251` | Upstream etcd source (both 3.4 and 3.5) uses lowercase JSON tags — case-folding is not needed. Pitfall 22's defensive intent is already satisfied by the map-based parser. |
| LB filtering via container-name regex | Suffix matching like `strings.HasSuffix(name, "-lb")` | The `io.x-k8s.kind.role` label (already read at clusterskew.go:99) | Cluster names can be anything (Phase 47-06 already removed the `=kind` value pin); name-based heuristics break. The role label is canonical. |
| New Result/Verdict type | A typed enum for pass/warn/fail | Existing string constants `"ok"`, `"warn"`, `"fail"`, `"skip"` per `check.go:42-49` | Adding a typed enum would touch every check in `allChecks` (26 entries) — out of scope for a cosmetic fix. |
| Test-side cluster mocks | A real docker/containerd container fake | Existing `newFakeResumeReadinessCheck` / `newTestClusterNodeSkewCheck` injection points | The injection seams already exist; both checks have factored-out closures for every runtime call. |

---

## Common Pitfalls

### Pitfall 22 (this phase): etcdctl JSON shape variance

**Status:** Re-examined this session. Upstream source (etcd `release-3.4` and `release-3.5` branches, `etcdctl/ctlv3/command/ep_command.go`, `epHealth` struct) confirms **lowercase JSON tags in both versions**. Pitfall 22's "capitalized keys in some builds" claim is **not supported by upstream source**. The fixture matrix in §5 still distinguishes 3.4 (no error field on healthy entries) from 3.5 (error field present on unhealthy entries — `omitempty`) — both shapes parse cleanly through `[]map[string]interface{}`.

**The actual bug is NOT a shape variance** — it is the non-zero exit handling at `resumereadiness.go:172-184`. The JSON parser is fine; the bug is that the parser is never called when etcdctl exits non-zero.

### Pitfall 21 (this phase): LB container in skew calculation

**Status:** Confirmed. LB containers (haproxy) have no `/kind/version`. `realListNodes` blindly execs against every container matching the kind cluster label. Fix is the inline role guard. Single source of truth for the role string is `pkg/cluster/constants` (already in tree).

### Pitfall 20 (Phase 56 hangover): no mutex in production read path

**Status:** Hardened by Phase 56. Phase 57 must not introduce a `sync.Mutex` — verified neither fix needs one.

### Pitfall 23 (Phase 58 gate): stale binary trap

**Status:** Phase 58 concern only; Phase 57 is code+unit-tests, no live UAT. The binary built after 57 merges is what Phase 58 will smoke.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong | Source |
|---|-------|---------|---------------|--------|
| A1 | etcd 3.4 and 3.5 both use lowercase JSON tags `endpoint`/`health`/`took`/`error` for `etcdctl endpoint health --write-out=json` | §4, §5, §"Don't Hand-Roll" | If etcd 3.6+ or a vendor build emits capitalized keys, the parser silently returns `health:false` for all members (exact Pitfall 22 scenario). Mitigation: keep the `[]map[string]interface{}` approach which can be extended to case-fold without struct change. | [CITED: etcd-io/etcd `release-3.4` and `release-3.5` `etcdctl/ctlv3/command/ep_command.go` `epHealth` struct — verified via WebFetch this session] |
| A2 | `pkg/exec.OutputLines` returns both stdout-lines AND err on non-zero exit | §4, §"Don't Hand-Roll" | If `OutputLines` clears the buffer on error, partial-parse is impossible and the fix becomes more invasive (would need direct `exec.Cmd` usage). | [VERIFIED: `pkg/exec/helpers.go:78-87` — read this session; buffer is scanned regardless of `err`] |
| A3 | `pkg/cluster/constants` is import-safe from `pkg/internal/doctor` (no cycle) | §3, §10 | If a future commit makes `constants` import any doctor-adjacent package, the build breaks. | [VERIFIED: `resumestrategy.go` (Phase 52-04) already imports `pkg/cluster/constants` from the doctor package; STATE.md entry 2026-05-10 (52-04) documents "zero-import package, no cycle"] |
| A4 | Current `allChecks` count is 26 | §2c, §11 | If count is wrong, `TestAllChecks_CountIs26` fails immediately. | [VERIFIED: `go test -run TestAllChecks_CountIs26 ./pkg/internal/doctor/` exits 0 in 0.355s this session] |
| A5 | etcd 3.5's `etcdctl endpoint health --cluster` exits non-zero when any member is unhealthy, BUT still writes the JSON array to stdout for all members | §1, §4, §9 | If etcd 3.5 actually fails before writing stdout (e.g. on auth error per upstream issue #13144), then `healthLines` is empty and the parser fails → falls through to the existing generic-warn branch. This is the correct behavior — not a regression. | [CITED: etcd-io/etcd issue #9532 + #13144; PR #9540 — JSON output was specifically wired through both healthy and unhealthy code paths] |
| A6 | The 2/3 healthy case ("quorum intact" wording) is out of SC2's strict scope but improving it is low-cost | §4, §8 Open Q1 | If the planner decides the 2/3 wording is locked at "quorum at risk" (current code), SC2 still passes — the only normative case is 1/3. | [ASSUMED] |
| A7 | Live HA cluster verification for SC1/SC2 is deferred to Phase 58 (UAT) | §10, §11 | If the verifier interprets SC1's "3-CP HA cluster" as a live-UAT requirement for Phase 57 itself, plans need a smoke script. Recommendation: confirm with ROADMAP — ROADMAP §"Why Phase 58 is last" explicitly notes UAT runs after 57 against the final binary, so SC1/SC2 here are unit-test assertions. | [VERIFIED: ROADMAP.md lines 209-212 + 242 — Phase 58 is the UAT phase, runs against final binary; Phase 57 is code+unit-tests] |

**Items needing user confirmation before plan execution:**
- A6 (quorum-intact wording at 2/3) — Open Question 1.

---

## Sources

### Primary (HIGH confidence)
- `pkg/internal/doctor/clusterskew.go` (read in full this session) — DIAG-05 site
- `pkg/internal/doctor/clusterskew_test.go` (read in full this session) — existing test pattern + injection seam
- `pkg/internal/doctor/resumereadiness.go` (read in full this session) — DIAG-06 site
- `pkg/internal/doctor/resumereadiness_test.go` (read in full this session) — existing fixtures + injection seam
- `pkg/internal/doctor/check.go` (read this session) — Phase 56 race-free helper + `allChecks` registry
- `pkg/internal/doctor/check_test.go` (read this session) — count test pinning at 26
- `pkg/cluster/constants/constants.go` (read this session) — role string constants, leaf import
- `pkg/cluster/nodeutils/roles.go` (read this session) — canonical `InternalNodes` filter (reference only, not importable from doctor)
- `pkg/exec/helpers.go:78-87` (read this session) — `OutputLines` behavior on non-zero exit
- `Makefile:85-92` (read this session) — `test-race-doctor` target
- `.planning/STATE.md` (read this session) — Phase 56 close decisions, count-test history
- `.planning/research/SUMMARY.md` (read this session) — Phase 57 plan structure consensus
- `.planning/research/ARCHITECTURE.md §8a, §8b` (read this session) — exact fix descriptions for both checks
- `.planning/research/PITFALLS.md §Pitfall 21, §Pitfall 22` (read this session) — failure modes + warning signs
- `.planning/research/FEATURES.md §"Doctor Cosmetic Fixes"` (read this session) — target behavior + JSON field stability claim
- Live: `go test -run TestAllChecks_CountIs26 ./pkg/internal/doctor/` — exits 0; count = 26

### Secondary (MEDIUM-HIGH confidence — official source verified)
- etcd-io/etcd `release-3.4/etcdctl/ctlv3/command/ep_command.go` — `epHealth` struct: `Ep`/`Health`/`Took`/`Error` with lowercase JSON tags + `omitempty` on Error — confirms 3.4 schema
- etcd-io/etcd `release-3.5/etcdctl/ctlv3/command/ep_command.go` — identical schema to 3.4 — confirms 3.5 schema
- etcd-io/etcd PR #9540 — historical: JSON output added; example shows lowercase tags

### Tertiary (LOW confidence — for context only)
- WebSearch results on etcdctl JSON output (multiple results converge on lowercase tags; no credible source for capitalized keys)

---

## Metadata

**Confidence breakdown:**
- Codebase map: HIGH — all line ranges read directly this session
- DIAG-05 fix: HIGH — single inline role guard; constants already in repo; test seam exists
- DIAG-06 fix: HIGH — parser already exists; bug is one call-site error-branch; `OutputLines` semantics verified
- Pitfall 22 fixture coverage: HIGH — etcd source confirmed schemas are identical (lowercase tags + omitempty error field) across 3.4 and 3.5
- Wave/parallel safety: HIGH — files are disjoint; count test invariant unchanged; race gate inherited
- Open questions: LOW for Q1 (wording aesthetics), HIGH for Q2 (empty array case)

**Research date:** 2026-05-12
**Valid until:** 2026-06-11 (30 days — etcd schema is stable; bug sites are stable code)
