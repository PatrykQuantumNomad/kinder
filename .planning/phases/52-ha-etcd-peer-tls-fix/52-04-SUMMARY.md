---
phase: 52-ha-etcd-peer-tls-fix
plan: "04"
subsystem: doctor
tags: [doctor, ha, resume-strategy, label-inspection, cluster]
dependency_graph:
  requires: [52-01, 52-02, 52-03]
  provides:
    - haResumeStrategyCheck implementing Check interface
    - newHAResumeStrategyCheck() registered as "ha-resume-strategy" in allChecks Cluster category
  affects:
    - pkg/internal/doctor/check.go (allChecks registry: 25 → 26)
    - pkg/internal/doctor/check_test.go (CountIs25→CountIs26, new IncludesHAResumeStrategy)
    - pkg/internal/doctor/gpu_test.go (TestAllChecks_RegisteredOrder: 25→26)
    - pkg/internal/doctor/socket_test.go (TestAllChecks_Registry: 25→26)
tech_stack:
  added: []
  patterns:
    - Struct-field function injection (lister, inspector) for test isolation (mirrors clusterResumeReadinessCheck pattern)
    - Separate listKinderCPContainersByCluster helper returning map[clusterName][]containerName for multi-cluster detection
    - Two-call inspect pattern for label value + presence (avoids fragile cross-runtime conditional templates)
    - Import pkg/cluster/constants directly (zero imports, no cycle risk)
key_files:
  created:
    - pkg/internal/doctor/resumestrategy.go
    - pkg/internal/doctor/resumestrategy_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/check_test.go
    - pkg/internal/doctor/gpu_test.go
    - pkg/internal/doctor/socket_test.go
decisions:
  - "listKinderCPContainersByCluster is a NEW helper, not a reuse of realListCPNodes (see Why New Lister section)"
  - "pkg/cluster/constants imported directly — zero imports, no cycle possible"
  - "Two-call inspect approach (value then PRESENT/ABSENT) chosen over single conditional template for cross-runtime portability"
  - "Check does NOT call ProbeIPAM; verified by DoesNotRunProbe meta test"
  - "Mixed-label case is fail; legacy (absent label) and cert-regen are both warn — this matches CONTEXT.md D-locks"
metrics:
  duration_minutes: 12
  completed: "2026-05-10"
  tasks_completed: 2
  files_created: 2
  files_modified: 4
---

# Phase 52 Plan 04: HA Resume Strategy Doctor Check Summary

New `ha-resume-strategy` doctor check in Cluster category reads the resume-strategy label from all CP containers in a kinder HA cluster and emits 8 structured verdicts; allChecks registry advanced from 25 to 26 closing Phase 52.

## Verbatim Reason Strings (user-facing doctor output)

These are the locked Reason strings used in each verdict branch, exactly as they appear in the implementation:

### Verdict 2 — cert-regen (all CPs consistently labeled cert-regen)
```
Cluster will regenerate etcd peer certs on each resume (~40-60s overhead). Switch to ip-pinned by deleting and recreating the cluster on a runtime that supports IP pinning.
```

### Verdict 3 — cert-regen (legacy) (all CPs have absent label, cluster predates v2.4)
```
Cluster was created before v2.4 IP-pinning was available. Delete and recreate to opt into the faster ip-pinned resume path.
```

### Verdict 4 — mixed (CPs disagree on resume-strategy label — corruption)
```
Control-plane nodes disagree on resume-strategy (<cp_summary>). This indicates manual label tampering or partial migration; cluster state may be inconsistent. Delete and recreate.
```
where `<cp_summary>` is e.g. `cp1+cp3=ip-pinned, cp2=cert-regen` (sorted, grouped by value).

### Verdict 7 — indeterminate (inspect on any CP returns error)
```
Could not read resume-strategy label from one or more control-plane nodes: <cp>: <error>. Re-run after the runtime daemon recovers.
```

### Verdict 8 — multiple clusters present
```
Reason: "check kubectl context to select the correct cluster before running doctor"
```

## Why a New Lister Was Added (Not realListCPNodes)

`realListCPNodes` (in `resumereadiness.go`) flattens all CP containers across ALL kinder clusters into a single sorted `[]string`. This design assumes a single cluster per host.

`haResumeStrategyCheck` requires a `map[clusterName][]containerName` to implement Verdict 8 (warn when multiple clusters detected). Flattening would make it impossible to count distinct clusters or report which clusters are present.

A new helper `listKinderCPContainersByCluster` was added to `resumestrategy.go` with a comment explaining why the new helper was needed. It does NOT replace `realListCPNodes` — both coexist, serving different shapes.

## Eight Verdict Branches (all confirmed present and passing)

| # | Verdict | Status | Test |
|---|---------|--------|------|
| 1 | ok — all ip-pinned | ok | TestHAResumeStrategyCheck_AllIPPinned |
| 2 | warn — cert-regen | warn | TestHAResumeStrategyCheck_AllCertRegen |
| 3 | warn — legacy (absent label) | warn | TestHAResumeStrategyCheck_LegacyNoLabel |
| 4 | fail — mixed labels | fail | TestHAResumeStrategyCheck_Mixed |
| 5 | skip — single CP | skip | TestHAResumeStrategyCheck_SingleCP |
| 6 | skip — no cluster | skip | TestHAResumeStrategyCheck_NoCluster |
| 7 | warn — indeterminate (inspect error) | warn | TestHAResumeStrategyCheck_InspectFails |
| 8 | warn — multiple clusters | warn | TestHAResumeStrategyCheck_MultipleClustersDetected |

Meta tests (invariants): TestHAResumeStrategyCheck_CategoryAndName, TestHAResumeStrategyCheck_DoesNotRunProbe.

**Total: 10 tests in resumestrategy_test.go — all passing.**

## Final allChecks Count

| State | Count | Change |
|-------|-------|--------|
| Before Phase 52 | 24 | baseline |
| After Plan 52-01 (ipam-probe) | 25 | +1 |
| After Plan 52-04 (ha-resume-strategy) | 26 | +1 |

Verified via:
```
awk '/^var allChecks = \[\]Check{/,/^}/' pkg/internal/doctor/check.go \
  | grep -E '^\s+new.+\(\),' | wc -l
```
Output: `26`

## Count Test Rename Confirmation

- `TestAllChecks_CountIs25` (introduced in Plan 52-01) has been **renamed** to `TestAllChecks_CountIs26`.
- The asserted constant was bumped from `25` to `26`.
- The previous test name `TestAllChecks_CountIs25` **no longer appears anywhere** in the codebase (confirmed via `grep -rn "CountIs25" pkg/internal/doctor/` — no output).

All three count-asserting tests updated:
1. `check_test.go` — `TestAllChecks_CountIs26` (renamed + bumped)
2. `socket_test.go` — `TestAllChecks_Registry` (count 25→26, ha-resume-strategy added to expected slice)
3. `gpu_test.go` — `TestAllChecks_RegisteredOrder` (count 25→26, ha-resume-strategy added to expected slice)

## Files Created / Modified

| File | Change |
|------|--------|
| `pkg/internal/doctor/resumestrategy.go` | NEW — haResumeStrategyCheck, newHAResumeStrategyCheck, listKinderCPContainersByCluster, inspectContainerLabel, sortedKeys |
| `pkg/internal/doctor/resumestrategy_test.go` | NEW — 10 tests: 8 verdict branches + 2 meta tests |
| `pkg/internal/doctor/check.go` | Cluster category comment updated; newHAResumeStrategyCheck() appended after newClusterResumeReadinessCheck() |
| `pkg/internal/doctor/check_test.go` | TestAllChecks_CountIs25→CountIs26 (bumped to 26); TestAllChecks_IncludesHAResumeStrategy added |
| `pkg/internal/doctor/gpu_test.go` | TestAllChecks_RegisteredOrder: count 25→26, ha-resume-strategy added |
| `pkg/internal/doctor/socket_test.go` | TestAllChecks_Registry: count 25→26, ha-resume-strategy added |

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| Task 1 | 55b75533 | feat(52-04): implement haResumeStrategyCheck + tests (Task 1) |
| Task 2 | 8eb9dfd1 | feat(52-04): register ha-resume-strategy in allChecks; rename count test to CountIs26 (Task 2) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] LegacyNoLabel test failed due to case mismatch**
- **Found during:** Task 1 GREEN (first test run)
- **Issue:** Plan's locked Reason text starts with "Cluster was created before v2.4..." — "Delete" is capitalized. The test assertion used `strings.Contains(r.Reason, "delete")` (lowercase), which failed.
- **Fix:** Changed test assertion to normalize the reason to lowercase before comparison: `reasonLower := strings.ToLower(r.Reason)` then check `reasonLower` — allows any casing in the implementation while correctly verifying semantic content.
- **Files modified:** `pkg/internal/doctor/resumestrategy_test.go`
- **Commit:** 55b75533

## Known Stubs

None. The check is fully implemented. It reads actual container labels via `docker/podman inspect`. No production data is mocked or deferred.

## Threat Surface Scan

This plan adds label-value inspection and renders them in `Result.Message`/`Result.Reason`. Mitigations from the plan's threat model:

| Threat | Mitigation Status |
|--------|------------------|
| T-52-04-01: label values rendered to terminal | ACCEPTED — labels are kinder-internal namespaced values (ip-pinned, cert-regen); not user secrets |
| T-52-04-02: user manually mutates labels to mixed values | MITIGATED — mixed-label case detected as fail with explicit corruption reason directing user to delete+recreate |
| T-52-04-03: inspect on stopped runtime daemon | MITIGATED — inspect errors produce warn (indeterminate) not fail; doctor exits non-zero only on fail verdict |

No new network endpoints, auth paths, or schema changes introduced.

## Phase 52 Closure

Both new checks added in Phase 52 are now registered and discoverable:

| Check | Category | Plan | Status |
|-------|----------|------|--------|
| `ipam-probe` | Network | 52-01 | Registered (baseline 24 → 25) |
| `ha-resume-strategy` | Cluster | 52-04 | Registered (25 → 26) |

Phase 52 is complete: etcd peer TLS cert fix, IP pinning at create time, reactive drift detection + cert-regen at resume, and two new doctor checks exposing system state.

## Self-Check: PASSED

Files exist:
- FOUND: pkg/internal/doctor/resumestrategy.go
- FOUND: pkg/internal/doctor/resumestrategy_test.go

Commits exist:
- FOUND: 55b75533 (Task 1)
- FOUND: 8eb9dfd1 (Task 2)

Tests pass:
- `go test ./pkg/internal/doctor/... -count=1`: PASS (all 10 new tests + full suite)
- `go vet ./...`: PASS
- `go build ./...`: PASS
- `grep -rn "CountIs25" pkg/internal/doctor/`: empty (old test name gone)
- `awk ... | wc -l`: 26 (allChecks count confirmed)
- No new go.mod/go.sum changes
