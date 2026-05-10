---
phase: 52-ha-etcd-peer-tls-fix
plan: "02"
subsystem: lifecycle/ippin + docker-provider + podman-provider
tags: [ipam, ip-pin, docker, podman, lifecycle, resume-strategy, ha]
dependency_graph:
  requires: [52-01]
  provides:
    - RecordAndPinHAControlPlane(binaryName, networkName, cpContainers, logger) error
    - ReadIPAMState(binaryName, container, tmpDir) (*IPAMState, error)
    - IPAMState{Network, IPv4} in pkg/internal/ippin
    - ResumeStrategyLabel/StrategyIPPinned/StrategyCertRegen in pkg/cluster/constants
    - /kind/ipam-state.json written on each pinned CP container at create time
    - resume-strategy label on CP containers at docker/podman run time
  affects:
    - pkg/internal/ippin/ippin.go (NEW — implementation)
    - pkg/internal/lifecycle/ippin.go (MODIFIED — thin facade)
    - pkg/internal/lifecycle/ippin_test.go (MODIFIED — updated to use ippin vars)
    - pkg/cluster/constants/constants.go (MODIFIED — 3 strategy constants added)
    - pkg/cluster/internal/providers/docker/provider.go (MODIFIED — Provision wired)
    - pkg/cluster/internal/providers/docker/create.go (MODIFIED — planCreation strategy param)
    - pkg/cluster/internal/providers/docker/provider_ippin_test.go (NEW — full-strength tests)
    - pkg/cluster/internal/providers/podman/provider.go (MODIFIED — Provision wired)
    - pkg/cluster/internal/providers/podman/provision.go (MODIFIED — planCreation strategy param)
    - pkg/cluster/internal/providers/podman/provider_ippin_test.go (NEW — full-strength tests)
tech_stack:
  added: []
  patterns:
    - Package-level var injection for probe and RecordAndPin (matches ipamProbeCmder in doctor)
    - Neutral package pkg/internal/ippin to break lifecycle→cluster→docker import cycle
    - Strategy label injected before image arg in runArgsForNode output (minimal-change pattern)
    - Closure-capture of strategy string in planCreation for per-CP label injection
key_files:
  created:
    - pkg/internal/ippin/ippin.go
    - pkg/cluster/internal/providers/docker/provider_ippin_test.go
    - pkg/cluster/internal/providers/podman/provider_ippin_test.go
  modified:
    - pkg/internal/lifecycle/ippin.go
    - pkg/internal/lifecycle/ippin_test.go
    - pkg/cluster/constants/constants.go
    - pkg/cluster/internal/providers/docker/provider.go
    - pkg/cluster/internal/providers/docker/create.go
    - pkg/cluster/internal/providers/podman/provider.go
    - pkg/cluster/internal/providers/podman/provision.go
decisions:
  - "Import cycle (lifecycle→cluster→docker) prevented direct lifecycle import from providers; implementation moved to pkg/internal/ippin; lifecycle/ippin.go is a thin facade delegating to ippin package"
  - "Strategy constants moved to pkg/cluster/constants (already imported by both providers); lifecycle re-uses via imported alias"
  - "Label injection point: insert --label before the image (last element) in args slice returned by runArgsForNode — minimal change, no signature change to runArgsForNode"
  - "Single-CP threshold in Provision (not in lifecycle helper) — gate is len(cpContainerNamesForConfig(cfg)) >= 2"
  - "Probe runs once per Provision; result cached in strategy local var for both label injection AND post-Provision hook"
  - "TestProvisionAttachesStrategyLabel_Podman is full-strength: asserts probe→label-injection→post-Provision hook (not smoke-only); B3 disposition honored"
  - "nerdctl provider intentionally untouched; no resume-strategy label on nerdctl containers; Plan 52-03 treats no-label as legacy=cert-regen"
  - "Connect-failure followed by recovery-failure returns wrapped error containing BOTH causes (W1 disposition); bootstrap CP left disconnected; Provision fails fast"
metrics:
  duration_minutes: 35
  completed: "2026-05-10"
  tasks_completed: 2
  files_created: 3
  files_modified: 7
---

# Phase 52 Plan 02: IP-Pin Create-Time Wiring Summary

IP-pin create-time wiring implemented across docker and podman providers; each HA CP container receives /kind/ipam-state.json and a run-time resume-strategy label; cycle break via pkg/internal/ippin neutral package.

## Architecture: Import Cycle Resolution

`pkg/internal/lifecycle` imports `sigs.k8s.io/kind/pkg/cluster` (via `pause.go` PauseOptions.Provider field), which transitively imports `pkg/cluster/internal/providers/docker`. This made `docker/provider.go → lifecycle` a cycle.

**Resolution**: The implementation was split into two layers:

| Layer | Package | Role |
|-------|---------|------|
| Implementation | `pkg/internal/ippin` | Full logic (no cycle: doesn't import cluster) |
| Facade | `pkg/internal/lifecycle/ippin.go` | Thin wrapper delegating to `pkg/internal/ippin` |

Both `docker/provider.go` and `podman/provider.go` import `pkg/internal/ippin` directly. `pkg/internal/lifecycle` re-exports the API via wrapper functions. Plan 52-03 can use either the lifecycle facade or `pkg/internal/ippin` directly.

**Strategy constants location**: `pkg/cluster/constants` (already imported by both providers). The three constants (`ResumeStrategyLabel`, `StrategyIPPinned`, `StrategyCertRegen`) live there so providers and lifecycle can both consume them without a cycle.

## Wiring Point: docker/create.go and podman/provision.go

**Choice**: Closure-capture of `strategy` string with insertion before-image in the CP node arg slice.

The label is inserted inside the CP-role closure in `planCreation`:
```go
// After runArgsForNode builds args (image is always last):
if strategy != "" {
    labelArgs := []string{"--label",
        fmt.Sprintf("%s=%s", constants.ResumeStrategyLabel, strategy)}
    args = append(args[:len(args)-1], append(labelArgs, args[len(args)-1])...)
}
```

`planCreation` signature changed from `planCreation(cfg, networkName)` to `planCreation(cfg, networkName, strategy string)`. Workers and LBs receive no label.

## Single-CP Threshold Logic

The gate lives in **`Provision`**, not in the lifecycle helper:

```go
cpNames := cpContainerNamesForConfig(cfg)
if len(cpNames) >= 2 {
    // probe once
}
// ... later:
if len(cpNames) >= 2 && strategy == constants.StrategyIPPinned {
    provisionRecordAndPinFn(...)
}
```

`RecordAndPinHAControlPlane` in `pkg/internal/ippin` also has an internal guard (`len(cpContainers) <= 1 → return nil`) for safety. Single-CP Provision calls `planCreation` with `strategy=""` — zero overhead.

## Podman Wiring (Parity Confirmation)

Podman wiring is **identical to docker**:
- `cpContainerNamesForConfig(cfg)` — same helper in `podman/provider.go`
- `provisionProbeIPAMFn = doctor.ProbeIPAM` — called with `"podman"` binary
- `provisionRecordAndPinFn = kindippin.RecordAndPinHAControlPlane` — same function
- `planCreation` in `provision.go` has the same `strategy string` parameter and identical label injection

`TestProvisionAttachesStrategyLabel_Podman` is **full-strength** (B3 honored): it asserts:
1. Probe invoked with binaryName=`"podman"`
2. Label injection: `planCreation(cfg, "kind", StrategyIPPinned)` returns 4 funcs (3 CP + 1 LB)
3. Post-Provision hook: `provisionRecordAndPinFn` receives binary=`"podman"`, network=`"kind"`, and all 3 CP container names

This is NOT a smoke-only assertion (per B3 disposition: UAT-3 supplements but does not replace this unit coverage).

## nerdctl Provider: Deliberately Untouched

`pkg/cluster/internal/providers/nerdctl/` was not modified. nerdctl containers are created without a resume-strategy label and without `/kind/ipam-state.json`. At resume time (Plan 52-03), the bootstrap CP has no label → legacy branch → cert-regen. This is correct: nerdctl `network connect` is not implemented (RESEARCH PIT-4).

## Connect-Failure / Recovery-Failure Error Format (W1)

```
"failed to pin IP 172.18.0.5 for cp1: connect-ip error: <original>; recovery-connect error: <recovery>"
```

Both causes are present in the returned error. The bootstrap CP container is left disconnected; Provision fails fast. Subsequent CPs in the slice are NOT processed (consistent with disconnect-failure halt).

When recovery **succeeds** (IP not pinned but baseline connectivity restored), the error message is:
```
"failed to pin IP 172.18.0.5 for cp1 (recovery reconnect succeeded without --ip, but pin failed): <original>"
```

## Files Created / Modified

| File | Change |
|------|--------|
| `pkg/internal/ippin/ippin.go` | NEW — RecordAndPinHAControlPlane, ReadIPAMState, IPAMState, Cmder type, IppinCmder/ProbeIPAMFn vars |
| `pkg/internal/lifecycle/ippin.go` | MODIFIED — now thin facade delegating to pkg/internal/ippin |
| `pkg/internal/lifecycle/ippin_test.go` | MODIFIED — tests swap ippin.IppinCmder/ippin.ProbeIPAMFn via swapIPPin* helpers |
| `pkg/cluster/constants/constants.go` | MODIFIED — ResumeStrategyLabel, StrategyIPPinned, StrategyCertRegen added |
| `pkg/cluster/internal/providers/docker/provider.go` | MODIFIED — cpContainerNamesForConfig, provisionProbeIPAMFn, provisionRecordAndPinFn, Provision wired |
| `pkg/cluster/internal/providers/docker/create.go` | MODIFIED — planCreation signature + CP label injection |
| `pkg/cluster/internal/providers/docker/provider_ippin_test.go` | NEW — 5 tests (full-strength B3) |
| `pkg/cluster/internal/providers/podman/provider.go` | MODIFIED — identical wiring to docker |
| `pkg/cluster/internal/providers/podman/provision.go` | MODIFIED — identical planCreation change to docker |
| `pkg/cluster/internal/providers/podman/provider_ippin_test.go` | NEW — 4 tests (full-strength B3, parity) |

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| Task 1 | d9cf003b | feat(52-02): implement ippin lifecycle module + tests (Task 1) |
| Task 2 | b1d07103 | feat(52-02): wire create-time hook into docker+podman providers (Task 2) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 4 - Architecture] Import cycle discovered; implementation moved to pkg/internal/ippin**
- **Found during:** Task 2 GREEN (planning the provider import)
- **Issue:** `pkg/internal/lifecycle` transitively imports `pkg/cluster/internal/providers/docker` via `pause.go` → `pkg/cluster`. If docker imported `lifecycle`, a cycle would result.
- **Fix:** Created `pkg/internal/ippin` as a neutral package with the actual implementation. `lifecycle/ippin.go` became a thin facade. The plan's "if cycle detected, move constants to neutral package" guidance was extended to the full implementation (moving just constants would not have broken the cycle since `RecordAndPinHAControlPlane` still needed to be in a non-cycling location). Strategy constants were added to `pkg/cluster/constants` as the plan specified.
- **Files modified:** `pkg/internal/ippin/ippin.go` (new), `pkg/internal/lifecycle/ippin.go` (refactored), `pkg/internal/lifecycle/ippin_test.go` (updated)
- **Commit:** b1d07103 (Task 2)

## Known Stubs

None. The create-time wiring is fully implemented. Plan 52-03 owns the consume-side (resume-time) which calls `ReadIPAMState` and `ReapplyPinnedIPs` — those are NOT stubs, they're the next plan's responsibility.

## Threat Surface Scan

No new network endpoints or auth paths. The following mitigations from the plan's threat model were implemented:

| Threat | Mitigation Status |
|--------|------------------|
| T-52-02-01: IP injection (inspect → --ip) | MITIGATED — `net.ParseIP(rawIP) == nil` returns error before any CLI reuse |
| T-52-02-02: partial pinning half-state | MITIGATED — disconnect failure halts the loop; Provision fails fast |
| T-52-02-03: /kind/ipam-state.json world-readable | MITIGATED — `chmod 0600` in the `sh -c` script at write time |
| T-52-02-04: label collision | ACCEPTED — `io.x-k8s.kinder.*` namespace matches existing convention |
| T-52-02-05: podman behavioral divergence | MITIGATED — per-runtime ProbeIPAM verdict gates the ip-pin path |
| T-52-02-06: connect-failure + recovery-failure DoS | ACCEPTED — hard error with both causes returned; Provision fails fast (W1) |

## Self-Check: PASSED

Files exist:
- FOUND: pkg/internal/ippin/ippin.go
- FOUND: pkg/internal/lifecycle/ippin.go
- FOUND: pkg/internal/lifecycle/ippin_test.go
- FOUND: pkg/cluster/internal/providers/docker/provider_ippin_test.go
- FOUND: pkg/cluster/internal/providers/podman/provider_ippin_test.go

Commits exist:
- FOUND: d9cf003b (Task 1)
- FOUND: b1d07103 (Task 2)

Tests pass:
- `go test ./pkg/internal/lifecycle/... ./pkg/cluster/internal/providers/docker/... ./pkg/cluster/internal/providers/podman/... -count=1 -race`: PASS
- `go vet ./...`: PASS
- `go build ./...`: PASS (no import cycle)
- No new go.mod/go.sum changes
