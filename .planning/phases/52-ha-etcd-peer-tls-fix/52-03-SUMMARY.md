---
phase: 52-ha-etcd-peer-tls-fix
plan: "03"
subsystem: lifecycle/certregen + resume
tags: [ha, etcd, cert-regen, ip-pin, resume, drift-detection, lifecycle]
dependency_graph:
  requires: [52-02]
  provides:
    - IPDriftDetected(binaryName, container, tmpDir string) (bool, string, string, error)
    - RegenerateEtcdPeerCertsWholesale(cpNodes []nodes.Node, logger log.Logger) error
    - applyPinnedIPsBeforeCPStart (unexported, in resume.go Phase 1.5)
    - readResumeStrategy (unexported, in resume.go Phase 0)
    - StrategyIPPinned / StrategyCertRegen / ResumeStrategyLabel re-exported in lifecycle/ippin.go
  affects:
    - pkg/internal/lifecycle/certregen.go (NEW)
    - pkg/internal/lifecycle/certregen_test.go (NEW)
    - pkg/internal/lifecycle/resume.go (MODIFIED — Phase 0 + 1.5 + 4.5 hooks)
    - pkg/internal/lifecycle/resume_test.go (MODIFIED — 9 new HA strategy tests)
    - pkg/internal/lifecycle/ippin.go (MODIFIED — strategy constant re-exports)
tech_stack:
  added: []
  patterns:
    - certRegenSleeper package-level var injection (avoids 45s real sleeps in tests)
    - IPDriftDetected reads /kind/ipam-state.json via ReadIPAMState then inspects via defaultCmder
    - applyPinnedIPsBeforeCPStart sources networkName from IPAMState.Network (W4 fix)
    - Strategy dispatch gated by len(cp) >= 2 (D-zero-overhead-single-CP)
    - Legacy = absent label = cert-regen-forever per CONTEXT.md
key_files:
  created:
    - pkg/internal/lifecycle/certregen.go
    - pkg/internal/lifecycle/certregen_test.go
  modified:
    - pkg/internal/lifecycle/resume.go
    - pkg/internal/lifecycle/resume_test.go
    - pkg/internal/lifecycle/ippin.go
decisions:
  - "applyPinnedIPsBeforeCPStart uses os.TempDir() as tmpDir for ReadIPAMState staging; tests pre-write files there with t.Cleanup removal"
  - "certRegenSleeper package-level injection prevents 45s+ real sleeps in unit tests (same pattern as ipamProbeCmder in doctor)"
  - "IPDriftDetected signature takes tmpDir to decouple from os.TempDir() at test boundary"
  - "haTestCmder dispatches on name first (kubeadm, mv) then on args[0] (start, inspect, network) — covers node.Command() routing through defaultCmder"
  - "Strategy constants re-exported as typed consts in lifecycle/ippin.go so resume.go uses unqualified StrategyIPPinned without constants. qualifier"
metrics:
  duration_minutes: 11
  completed: "2026-05-10"
  tasks_completed: 2
  files_created: 2
  files_modified: 3
---

# Phase 52 Plan 03: certregen module + HA resume strategy wiring Summary

Reactive drift detection and wholesale etcd peer cert-regen implemented in certregen.go; resume.go wired with Phase 1.5 (ip-pin reconnect before CP start) and Phase 4.5 (cert-regen after workers start); W2 Option A hard-halt on ReadIPAMState failure enforced; networkName sourced from IPAMState.Network (W4).

## Insertion Points in resume.go

| Hook | Line Range | Description |
|------|-----------|-------------|
| Phase 0 (strategy detection) | lines 248-266 | `readResumeStrategy` called after `ClassifyNodes`, gated `len(cp) >= 2`. Nerdctl downgrade to cert-regen at line 261. |
| Phase 1.5 (ip-pinned reconnect) | lines 322-338 | `applyPinnedIPsBeforeCPStart` called between LB start (Phase 1) and CP start (Phase 2). CPs are in exited state here (T-52-03-04). |
| Phase 4.5 (cert-regen / legacy) | lines 349-373 | Reactive drift loop + `RegenerateEtcdPeerCertsWholesale` called after workers start (Phase 4), before readiness gate. |

## W4 Confirmation: networkName from IPAMState.Network

`applyPinnedIPsBeforeCPStart` does NOT call `docker inspect` to derive the network name. It reads `state.Network` from the `IPAMState` struct returned by `ReadIPAMState`. This is the value Plan 52-02 persisted into `/kind/ipam-state.json` at create time — unambiguously naming the kind network for each CP, regardless of how many networks the container is attached to.

Relevant code in `pkg/internal/lifecycle/resume.go` (lines ~152-155):
```go
state, err := ReadIPAMState(binaryName, container, tmpDir)
// ...
networkName := state.Network
```

## W2 Naming Confirmation: Unqualified Calls in resume.go

Both `IPDriftDetected` and `RegenerateEtcdPeerCertsWholesale` are called unqualified from `resume.go` because `certregen.go` is in the same package (`package lifecycle`). No `certregen.` qualifier exists anywhere in `resume.go`.

Grep gate result:
```
grep -v '^#' pkg/internal/lifecycle/resume.go | grep -c 'certregen\.' → 0
```

## W2 Option A Confirmation: Hard-Halt on ReadIPAMState Failure

`applyPinnedIPsBeforeCPStart` has NO soft-skip path. On any error from `ReadIPAMState` (file missing, parse fail, unreadable), the function immediately returns a structured diagnostic error. No per-CP fallback to cert-regen mid-resume exists. The test `TestResume_HAIPPin_ReadIPAMStateFailureHalts` verifies: (a) cp3 is NOT processed, (b) docker start on CPs is NOT called, (c) readiness gate is NOT entered.

## Verbatim Halt-and-Diagnostic Error Strings

### W2 Option A (ip-pin ReadIPAMState failure):
```
"ip-pin resume halted: failed to read /kind/ipam-state.json on %s: %v. Cluster state is undefined — delete and recreate the cluster."
```

### Pin reconnect disconnect failure:
```
"ip-pin resume halted: failed to disconnect %s from network %s. Cluster state is undefined — delete and recreate the cluster."
```

### Pin reconnect connect failure:
```
"ip-pin resume halted: failed to reconnect %s with --ip %s on network %s. Cluster state is undefined — delete and recreate the cluster."
```

### Cert-regen kubeadm failure:
```
"etcd peer cert regen failed on %s: kubeadm certs renew error. Cluster state is undefined — delete and recreate the cluster"
```

### Cert-regen mv manifest-out failure:
```
"etcd peer cert regen failed on %s: failed to move etcd manifest out. Cluster state is undefined — delete and recreate the cluster"
```

### Cert-regen mv manifest-back failure:
```
"etcd peer cert regen failed on %s: failed to restore etcd manifest. Cluster state is undefined — delete and recreate the cluster"
```

### Resume-level cert-regen wrap:
```
"HA resume cert-regen failed: delete and recreate the cluster: <inner error>"
```

## Static Pod Cycle Approach

Sleep-based timing is used (no crictl polling):
1. `certRegenSleeper(kubeletFileCheckFrequency + staticPodCycleSafetyMargin)` = 25s after mv-out
2. `certRegenSleeper(staticPodRecreationWait)` = 20s after mv-back

`certRegenSleeper` is a package-level `var` (initialized to `time.Sleep`) so tests inject a no-op. No crictl polling was introduced. This matches CONTEXT.md and RESEARCH guidance.

## Strategy Constants Re-export

The three strategy constants (`ResumeStrategyLabel`, `StrategyIPPinned`, `StrategyCertRegen`) are defined in `pkg/cluster/constants` (Plan 52-02) and re-exported as typed `const` values in `pkg/internal/lifecycle/ippin.go`:

```go
const (
    ResumeStrategyLabel = constants.ResumeStrategyLabel
    StrategyIPPinned    = constants.StrategyIPPinned
    StrategyCertRegen   = constants.StrategyCertRegen
)
```

This allows `resume.go` to use `StrategyIPPinned` (not `constants.StrategyIPPinned`) throughout, keeping the call sites clean and consistent with the W2 naming requirement.

## Files Created / Modified

| File | Change |
|------|--------|
| `pkg/internal/lifecycle/certregen.go` | NEW — certRegenSleeper, IPDriftDetected, RegenerateEtcdPeerCertsWholesale, timing constants |
| `pkg/internal/lifecycle/certregen_test.go` | NEW — 8 tests: drift detection (3) + wholesale regen (5 incl. sleep injection) |
| `pkg/internal/lifecycle/resume.go` | MODIFIED — readResumeStrategy, applyPinnedIPsBeforeCPStart, Phase 0/1.5/4.5 hooks, filepath+os imports |
| `pkg/internal/lifecycle/resume_test.go` | MODIFIED — 9 new HA strategy tests, haTestCmder infra, writeIPAMStateToOSTempDir helper |
| `pkg/internal/lifecycle/ippin.go` | MODIFIED — strategy constant re-exports added |

## Commits

| Task | Phase | Commit | Description |
|------|-------|--------|-------------|
| Task 1 | RED | e1f2cce8 | test(52-03): add failing tests for certregen module |
| Task 1 | GREEN | cab777cc | feat(52-03): implement IPDriftDetected + RegenerateEtcdPeerCertsWholesale |
| Task 2 | RED | 6b881d7b | test(52-03): add failing HA strategy tests for resume.go |
| Task 2 | GREEN | c38bbdf1 | feat(52-03): wire applyHAResumeStrategy into Resume() Phase 1.5 + Phase 4.5 |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Strategy constants not directly accessible in lifecycle package namespace**
- **Found during:** Task 2 GREEN (planning resume.go code)
- **Issue:** The plan specifies unqualified `StrategyIPPinned` etc. in `resume.go`, but these constants live in `pkg/cluster/constants`. Without re-exports, `resume.go` would need `constants.StrategyIPPinned` qualifiers.
- **Fix:** Added `const ResumeStrategyLabel`, `StrategyIPPinned`, `StrategyCertRegen` re-exports to `pkg/internal/lifecycle/ippin.go` — consistent with the existing `IPAMState` type alias already there.
- **Files modified:** `pkg/internal/lifecycle/ippin.go`
- **Commit:** 6b881d7b (Task 2 RED — added before tests ran)

**2. [Rule 1 - Bug] haTestCmder dispatch on args[0] missed node.Command("kubeadm", ...) calls**
- **Found during:** Task 2 GREEN (first test run)
- **Issue:** `haTestCmder.cmder()` switched on `args[0]` for all commands. But `fakeNode.Command("kubeadm", "certs", ...)` routes through `defaultCmder("kubeadm", "certs", ...)` — `name="kubeadm"` and `args[0]="certs"`. The switch returned "unhandled subcommand certs".
- **Fix:** Added `switch name { case "kubeadm": ... case "mv": ... }` before the `switch args[0]` block.
- **Files modified:** `pkg/internal/lifecycle/resume_test.go`
- **Commit:** c38bbdf1 (Task 2 GREEN)

**3. [Rule 1 - Bug] Tests pre-wrote ipam-state.json to t.TempDir() but applyPinnedIPsBeforeCPStart uses os.TempDir()**
- **Found during:** Task 2 GREEN (first test run — "no such file" errors)
- **Issue:** The original test helper wrote files to `t.TempDir()`. `applyPinnedIPsBeforeCPStart` and `IPDriftDetected` call `ReadIPAMState(..., os.TempDir())`, which looks for files at `os.TempDir()/<container>-ipam.json`. Test files were at a different path.
- **Fix:** Replaced `writeIPAMStateForHA(t, tmpDir, ...)` with `writeIPAMStateToOSTempDir(t, ...)` that writes to `os.TempDir()` and registers `t.Cleanup` removals.
- **Files modified:** `pkg/internal/lifecycle/resume_test.go`
- **Commit:** c38bbdf1 (Task 2 GREEN)

## Known Stubs

None. Both code paths (ip-pin and cert-regen) are fully wired. The cert-regen path depends on `kubeadm certs renew etcd-peer` running inside a live CP container — this is exercised in UAT-2/UAT-3 (52-VALIDATION.md), not in unit tests (no real cluster available).

## Threat Surface Scan

The following mitigations from the plan's threat model were implemented:

| Threat | Mitigation Status |
|--------|------------------|
| T-52-03-01: malformed recorded IP → --ip injection | MITIGATED — `net.ParseIP(rawIP) == nil` check in `IPDriftDetected` before using IP |
| T-52-03-02: cert-regen mid-fail leaves cluster broken | ACCEPTED — halt + diagnostic error; user directed to delete+recreate |
| T-52-03-03: cert-regen sleeps block resume 45s×N | MITIGATED — reactive trigger; common case (no drift) skips entirely; certRegenSleeper is injectable |
| T-52-03-04: network disconnect on running container | MITIGATED — Phase 1.5 runs strictly after LB start (Phase 1) and before CP start (Phase 2); CPs are in exited state |
| T-52-03-05: partial wholesale regen mid-failure | MITIGATED — first failure halts the loop; structured error directs user to delete+recreate |
| T-52-03-06: tampered /kind/ipam-state.json networkName | MITIGATED — if tampered network name causes `network connect` to fail loudly, Resume halts with wrapped error |
| T-52-03-07: missing /kind/ipam-state.json on ip-pinned CP | MITIGATED — W2 Option A: HARD HALT with structured diagnostic; TestResume_HAIPPin_ReadIPAMStateFailureHalts verifies |

## Self-Check: PASSED

Files exist:
- FOUND: pkg/internal/lifecycle/certregen.go
- FOUND: pkg/internal/lifecycle/certregen_test.go

Commits exist:
- FOUND: e1f2cce8 (Task 1 RED)
- FOUND: cab777cc (Task 1 GREEN)
- FOUND: 6b881d7b (Task 2 RED)
- FOUND: c38bbdf1 (Task 2 GREEN)

Tests pass:
- `go test ./pkg/internal/lifecycle/... -count=1 -race`: PASS
- `go vet ./...`: PASS
- `go build ./...`: PASS
- `grep -c 'certregen\.' resume.go`: 0 (W2 gate)
- No new go.mod/go.sum changes
