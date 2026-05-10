---
phase: 52-ha-etcd-peer-tls-fix
plan: "01"
subsystem: doctor
tags: [ipam, probe, docker, podman, nerdctl, doctor, network]
dependency_graph:
  requires: []
  provides:
    - ProbeIPAM(binaryName string) (Verdict, string, error)
    - VerdictIPPinned / VerdictCertRegen / VerdictUnsupported
    - newIPAMProbeCheck() registered as "ipam-probe" in allChecks Network category
  affects:
    - pkg/internal/doctor/check.go (allChecks registry: 24 → 25)
    - Plans 52-02 (ip-pin path) and 52-03 (cert-regen path) consume ProbeIPAM
tech_stack:
  added: []
  patterns:
    - Package-level var cmder injection for ProbeIPAM (mirrors resumereadiness.go)
    - Struct-field probeFunc injection for ipamProbeCheck (mirrors clusterResumeReadinessCheck)
    - seqCmder / recordingCmder test helpers (new, shared within ipamprobe_test.go)
key_files:
  created:
    - pkg/internal/doctor/ipamprobe.go
    - pkg/internal/doctor/ipamprobe_test.go
  modified:
    - pkg/internal/doctor/check.go
    - pkg/internal/doctor/check_test.go
    - pkg/internal/doctor/gpu_test.go
    - pkg/internal/doctor/socket_test.go
decisions:
  - "Tests that swap the package-level ipamProbeCmder global must NOT use t.Parallel() — documented in test file comment to prevent future regressions"
  - "Used seqCmder with exact call counts per test branch rather than a key-based map — catches unexpected extra calls via panic"
  - "Existing TestAllChecks_Registry (socket_test.go) and TestAllChecks_RegisteredOrder (gpu_test.go) updated alongside the new TestAllChecks_CountIs25 — all three count tests must stay in sync"
metrics:
  duration_minutes: 8
  completed: "2026-05-10"
  tasks_completed: 2
  files_created: 2
  files_modified: 4
---

# Phase 52 Plan 01: IPAM Probe Doctor Check Summary

IPAM feasibility probe and ipam-probe doctor check implemented using 8-step stop/start lifecycle simulation; nerdctl hard-coded to unsupported; allChecks registry advanced from 24 to 25.

## Exported API (consumed by Plans 52-02 and 52-03)

```go
// Package: sigs.k8s.io/kind/pkg/internal/doctor

// Verdict is the IPAM probe outcome.
type Verdict string

const (
    VerdictIPPinned    Verdict = "ip-pinned"   // runtime supports --ip, IP survives stop/start
    VerdictCertRegen   Verdict = "cert-regen"  // IP pinning unavailable; use cert-regen fallback
    VerdictUnsupported Verdict = "unsupported" // runtime lacks network connect (nerdctl)
)

// ProbeIPAM runs the full stop→start lifecycle simulation.
// Returns: (verdict, humanReason, error).
// error is always nil — runtime failures produce VerdictCertRegen with a non-empty reason.
// nerdctl short-circuits to VerdictUnsupported without any container operations.
func ProbeIPAM(binaryName string) (Verdict, string, error)

// newIPAMProbeCheck returns the doctor Check registered as "ipam-probe" / "Network".
// Exported as NewIPAMProbeCheck() is NOT needed — the check is self-registering via allChecks.
func newIPAMProbeCheck() Check
```

## allChecks Baseline Count

| State | Count | Source |
|-------|-------|--------|
| Before plan 52-01 | 24 | Verified via `awk ... | grep -E 'new.+\(\),' | wc -l` on branch before Task 2 |
| After plan 52-01 | 25 | newIPAMProbeCheck() appended after newSubnetClashCheck() in Network category |

Plan 52-04 will update TestAllChecks_CountIs25 → TestAllChecks_CountIs26 when the resume-strategy check is added.

## Probe Behavioral Contract

| Runtime | Short-circuit? | Verdict | Condition |
|---------|---------------|---------|-----------|
| nerdctl | YES — before any ops | VerdictUnsupported | filepath.Base(binary) == "nerdctl" |
| docker/podman | NO — full 8 steps | VerdictIPPinned | post-start IP == pre-stop IP |
| docker/podman | NO — full 8 steps | VerdictCertRegen | post-start IP != pre-stop IP |
| docker/podman | NO — fails at step N | VerdictCertRegen | any command error; reason describes step |

## Files Created / Modified

| File | Change |
|------|--------|
| `pkg/internal/doctor/ipamprobe.go` | NEW — Verdict type, ProbeIPAM, ipamProbeCmder global, ipamProbeCheck, detectRuntimeBinary |
| `pkg/internal/doctor/ipamprobe_test.go` | NEW — 10 tests covering all branches, cleanup, subnet, uniqueness, doctor output |
| `pkg/internal/doctor/check.go` | Network category comment updated; newIPAMProbeCheck() appended |
| `pkg/internal/doctor/check_test.go` | Added TestAllChecks_IncludesIPAMProbe, TestAllChecks_CountIs25 |
| `pkg/internal/doctor/gpu_test.go` | TestAllChecks_RegisteredOrder: count 24→25, ipam-probe added |
| `pkg/internal/doctor/socket_test.go` | TestAllChecks_Registry: count 24→25, ipam-probe added |

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| Task 1 | bb31049e | feat(52-01): implement IPAM probe doctor check (Task 1) |
| Task 2 | 143c4588 | feat(52-01): register ipam-probe in allChecks; pin count to 25 (Task 2) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Parallel tests sharing package-level global caused data race**
- **Found during:** Task 1 GREEN (test run)
- **Issue:** Tests using `setIPAMProbeCmder` all called `t.Parallel()`, causing concurrent mutations of the `ipamProbeCmder` package-level var and spurious panics from seqCmder
- **Fix:** Removed `t.Parallel()` from all tests that use `setIPAMProbeCmder`. Tests using struct injection (`ipamProbeCheck.probeFunc`) remain parallel-safe and keep `t.Parallel()`. Added explanation comment in test file.
- **Files modified:** `pkg/internal/doctor/ipamprobe_test.go`
- **Commit:** bb31049e (same task, included in GREEN)

**2. [Rule 2 - Missing Critical] Two pre-existing count tests needed updating**
- **Found during:** Task 2 RED (TestAllChecks_Registry in socket_test.go and TestAllChecks_RegisteredOrder in gpu_test.go asserted count==24 and would fail after registration)
- **Fix:** Updated both tests to count==25 and added ipam-probe to their expected-order lists. This is required for correctness — leaving them at 24 would break the build after Task 2 registration.
- **Files modified:** `pkg/internal/doctor/gpu_test.go`, `pkg/internal/doctor/socket_test.go`
- **Commit:** 143c4588 (Task 2 commit)

**3. [Rule 1 - Bug] TestIPAMProbe_NetworkAndContainerNamesAreUnique first version aborted on network create**
- **Found during:** Task 1 GREEN (first test run)
- **Issue:** Custom cmder returned error on every call; probe returned before reaching `run`, so container name was never captured
- **Fix:** Rewrote cmder to let network create succeed (capture network name), fail on run (capture container name from `--name` arg), then allow cleanup calls
- **Files modified:** `pkg/internal/doctor/ipamprobe_test.go`
- **Commit:** bb31049e (same task)

## Known Stubs

None. The probe is fully wired. No production callers wired yet (Plans 52-02 and 52-03 do that) — this is by design per the plan's "Output" section.

## Threat Surface Scan

No new network endpoints, auth paths, or schema changes introduced. The IPAM probe creates and destroys ephemeral scratch containers and networks — all mitigations listed in the plan's threat model are implemented:

| Threat | Mitigation Status |
|--------|------------------|
| T-52-01-01: IP injection via inspect output | MITIGATED — `net.ParseIP(ip)` validation before --ip |
| T-52-01-02: resource leak on probe failure | MITIGATED — deferred best-effort cleanup (two independent `_ =` calls) |
| T-52-01-03: probe net on shared subnet | ACCEPTED — 172.200.0.0/24 documented as non-overlapping; pinned in test |
| T-52-01-04: probe hang | MITIGATED — `sleep 30` bound in container; each CLI call returns within seconds |

## Self-Check: PASSED

Files exist:
- FOUND: pkg/internal/doctor/ipamprobe.go
- FOUND: pkg/internal/doctor/ipamprobe_test.go

Commits exist:
- FOUND: bb31049e (Task 1)
- FOUND: 143c4588 (Task 2)

Tests pass:
- `go test ./pkg/internal/doctor/... -count=1`: PASS
- `go vet ./...`: PASS
- `go build ./...`: PASS
- `go test ./pkg/internal/doctor/... -count=1 -race`: PASS
- No new go.mod/go.sum changes
