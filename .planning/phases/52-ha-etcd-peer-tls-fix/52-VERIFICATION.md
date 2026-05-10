---
phase: 52-ha-etcd-peer-tls-fix
verified: 2026-05-10T14:00:00Z
status: human_needed
score: 2/4 must-haves verified (2 require live cluster)
overrides_applied: 0
gaps:
  - truth: "go test -race passes cleanly for all Phase 52 packages"
    status: partial
    reason: >
      pkg/internal/doctor fails the race detector with count>=2. Race is
      between parallel allChecks WRITERS in check_test.go
      (TestRunAllChecks_PlatformSkip, TestRunAllChecks_NilPlatformsRunsOnAll,
      TestRunAllChecks_MultipleResultsPreserved -- all use t.Parallel() while
      writing the allChecks package global) and parallel allChecks READERS
      added by Phase 52 (TestAllChecks_IncludesHAResumeStrategy at
      check_test.go:255, TestAllChecks_CountIs26 at check_test.go:243).
      The root cause pre-existed Phase 52 (socket_test.go TestAllChecks_Registry
      also reads AllChecks() in parallel, dating back to pre-52 commits). Phase 52
      added two new parallel readers that made the race reliably triggerable with
      -count=3. SUMMARY incorrectly claimed "-race: PASS" based on a lucky
      single run. All other packages (lifecycle, ippin, docker, podman) pass
      cleanly with -count=3 -race.
    artifacts:
      - path: "pkg/internal/doctor/check_test.go"
        issue: >
          TestRunAllChecks_PlatformSkip (line 46), TestRunAllChecks_NilPlatformsRunsOnAll
          (line 84), TestRunAllChecks_MultipleResultsPreserved (line 116) all call
          t.Parallel() and then write the package-level allChecks var. Phase 52 added
          TestAllChecks_IncludesHAResumeStrategy (line 253) and TestAllChecks_CountIs26
          (line 242) which also call t.Parallel() and read allChecks via AllChecks().
          The race detector fires at check_test.go:255 vs check_test.go:57/121.
    missing:
      - >
        Remove t.Parallel() from the three allChecks-mutating tests (PlatformSkip,
        NilPlatforms, MultipleResultsPreserved) OR protect allChecks with a sync.RWMutex
        in check.go. Removing t.Parallel() from the mutating tests is the minimal fix --
        they still run sequentially with the full suite. The pattern was already correctly
        applied in ipamprobe_test.go (the ipamProbeCmder mutating tests do NOT use
        t.Parallel()). The allChecks mutating tests should follow the same rule.
human_verification:
  - test: "kinder resume on 3-CP HA cluster restores all control-plane nodes to Ready state after Docker IPAM reassigns IPs"
    expected: >
      After 'kinder pause <cluster>' + 'kinder resume <cluster>' on a 3-CP Docker HA cluster:
      all three control-plane nodes show Ready in 'kubectl get nodes'; 'etcdctl endpoint health
      --cluster' shows all members healthy; if IP pinning succeeded (VerdictIPPinned), each CP
      IP is unchanged and no cert-regen ran; if cert-regen was used, the resume log shows
      "Regenerating etcd peer cert on <node> (N/M)" and etcd is still healthy.
    why_human: >
      Must-have 1 and 2 require a live Docker daemon, a real kinder HA cluster, and actual
      IPAM reassignment behavior that cannot be replicated by unit tests with fakeCmder/fakeNode.
      The code paths for ip-pin reconnect (Phase 1.5) and cert-regen (Phase 4.5) are fully
      implemented and wired in resume.go but end-to-end correctness can only be confirmed
      against a real cluster. This maps to UAT-2 in 52-VALIDATION.md.
  - test: "fresh 'kinder create cluster --config ha.yaml' + pause + resume passes etcdctl endpoint health --cluster"
    expected: >
      All etcd members report 'health checked' with no errors. The /kind/ipam-state.json file
      is present on each CP container (if Docker supported --ip pinning) and contains the
      correct IPv4 and network name. OR if VerdictCertRegen was returned, no
      /kind/ipam-state.json is present and resume log shows cert-regen progress.
    why_human: >
      Must-have 2 requires a fresh cluster creation, pause, and resume cycle -- not
      reproducible statically. The create-time hook (RecordAndPinHAControlPlane) and the
      resume-time dispatch (applyPinnedIPsBeforeCPStart / RegenerateEtcdPeerCertsWholesale)
      are both fully implemented but their interaction with the real Docker IPAM needs
      live verification. This maps to UAT-1 + UAT-2 in 52-VALIDATION.md.
---

# Phase 52: HA Etcd Peer-TLS Fix Verification Report

**Phase Goal:** HA clusters resume cleanly after `kinder pause` + `kinder resume` regardless of whether Docker IPAM reassigns container IPs
**Verified:** 2026-05-10T14:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `kinder resume` on a 3-CP HA cluster returns all control-plane nodes to Ready state even when Docker assigns different IPs than those recorded in etcd peer certs | ? HUMAN NEEDED | ip-pin (Phase 1.5) and cert-regen (Phase 4.5) hooks fully wired in resume.go lines 322-380; correctness requires live cluster run |
| 2 | Fresh `kinder create cluster --config ha.yaml` + `kinder pause` + `kinder resume` passes `etcdctl endpoint health --cluster` with all members healthy | ? HUMAN NEEDED | Create-time hook (RecordAndPinHAControlPlane) wired in docker/provider.go:148 and podman/provider.go:150; resume-time dispatch wired; requires live run to confirm |
| 3 | Single-CP clusters incur zero overhead — no cert or network operations fire for non-HA topologies | VERIFIED | Three independent guards: docker/provider.go:115 `if len(cpNames) >= 2`, ippin/ippin.go:94 `if len(cpContainers) <= 1 { return nil }`, resume.go:257 `if len(cp) >= 2`; all three gate on HA. `planCreation` called with `strategy=""` for single-CP (zero label, zero probe). |
| 4 | If Docker IPAM probe succeeds, the fix uses IP pinning via `docker network connect --ip`; if infeasible, the cert-regen fallback is documented and implemented | VERIFIED | ProbeIPAM() in ipamprobe.go returns VerdictIPPinned / VerdictCertRegen / VerdictUnsupported; Provision gates on `verdict == VerdictIPPinned` (docker/provider.go:118, podman/provider.go:123); cert-regen path fully implemented in certregen.go with RegenerateEtcdPeerCertsWholesale(); both paths wired into resume.go Phase 1.5 and 4.5; nerdctl explicitly hardcoded to VerdictUnsupported (ipamprobe.go:77) |

**Score:** 2/4 truths fully verified statically; 2/4 need live cluster

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/internal/doctor/ipamprobe.go` | IPAM probe + doctor check | VERIFIED | ProbeIPAM(), Verdict type, VerdictIPPinned/CertRegen/Unsupported, newIPAMProbeCheck() |
| `pkg/internal/doctor/ipamprobe_test.go` | 10 tests covering all probe branches | VERIFIED | All tests pass (no race in ipamprobe_test.go itself) |
| `pkg/internal/ippin/ippin.go` | RecordAndPinHAControlPlane, ReadIPAMState, IPAMState | VERIFIED | Full implementation; import-cycle-free neutral package |
| `pkg/internal/lifecycle/ippin.go` | Thin facade delegating to ippin package | VERIFIED | Re-exports RecordAndPinHAControlPlane, ReadIPAMState, IPAMState, strategy constants |
| `pkg/internal/lifecycle/certregen.go` | IPDriftDetected, RegenerateEtcdPeerCertsWholesale | VERIFIED | Both functions implemented; certRegenSleeper injectable; timing constants present |
| `pkg/internal/lifecycle/certregen_test.go` | 8 tests | VERIFIED | All tests pass |
| `pkg/internal/lifecycle/resume.go` | Phase 0 + 1.5 + 4.5 hooks in Resume() | VERIFIED | readResumeStrategy (line 114), applyPinnedIPsBeforeCPStart (line 144), Phase 4.5 cert-regen loop (lines 352-381); all three hooks gated on len(cp) >= 2 |
| `pkg/internal/lifecycle/resume_test.go` | 9 new HA strategy tests | VERIFIED | Tests pass with -race -count=3 (lifecycle package is clean) |
| `pkg/internal/doctor/resumestrategy.go` | haResumeStrategyCheck with 8 verdict branches | VERIFIED | All 8 verdicts implemented; registered as "ha-resume-strategy" in Cluster category |
| `pkg/internal/doctor/resumestrategy_test.go` | 10 tests (8 verdicts + 2 meta) | VERIFIED | All tests pass |
| `pkg/cluster/internal/providers/docker/provider.go` | RecordAndPinHAControlPlane wiring in Provision | VERIFIED | provisionProbeIPAMFn, provisionRecordAndPinFn, cpContainerNamesForConfig, len(cpNames) >= 2 gate at lines 115 and 148 |
| `pkg/cluster/internal/providers/docker/create.go` | planCreation strategy param + CP label injection | VERIFIED | Label inserted before image arg when strategy != "" (lines 122-127) |
| `pkg/cluster/internal/providers/podman/provider.go` | Identical wiring to docker | VERIFIED | provisionProbeIPAMFn("podman"), provisionRecordAndPinFn, same gate pattern at lines 120 and 150 |
| `pkg/cluster/internal/providers/podman/provision.go` | planCreation strategy param + CP label injection | VERIFIED | Identical to docker pattern |
| `pkg/internal/doctor/check.go` | allChecks count = 26 | VERIFIED | `awk ... grep -E new.+() | wc -l` = 26; newIPAMProbeCheck() at line 80; newHAResumeStrategyCheck() at line 85 |
| `pkg/cluster/constants/constants.go` | ResumeStrategyLabel, StrategyIPPinned, StrategyCertRegen | VERIFIED | All three constants present; consumed by both providers and re-exported by lifecycle/ippin.go |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `docker/provider.go::Provision` | `doctor.ProbeIPAM` | `provisionProbeIPAMFn` | WIRED | Lines 116-120; probe called with "docker"; verdict gates strategy |
| `docker/provider.go::Provision` | `ippin.RecordAndPinHAControlPlane` | `provisionRecordAndPinFn` | WIRED | Lines 148-151; only called when `len(cpNames) >= 2 && strategy == StrategyIPPinned` |
| `docker/create.go::planCreation` | container label injection | `strategy` closure capture | WIRED | Lines 122-127; `--label io.x-k8s.kinder.resume-strategy=<strategy>` injected before image arg on CP nodes |
| `podman/provider.go::Provision` | `doctor.ProbeIPAM` | `provisionProbeIPAMFn` | WIRED | Lines 121-125; identical to docker |
| `podman/provider.go::Provision` | `ippin.RecordAndPinHAControlPlane` | `provisionRecordAndPinFn` | WIRED | Lines 150-153; identical to docker |
| `resume.go::Resume` | `readResumeStrategy` | Phase 0 block (line 257) | WIRED | Gated on `len(cp) >= 2`; reads bootstrap CP label via docker inspect |
| `resume.go::Resume` | `applyPinnedIPsBeforeCPStart` | Phase 1.5 (line 327) | WIRED | Called only when `strategy == StrategyIPPinned`; before startNodes(cp) |
| `resume.go::Resume` | `IPDriftDetected` + `RegenerateEtcdPeerCertsWholesale` | Phase 4.5 (lines 352-380) | WIRED | Called when `strategy == StrategyCertRegen || strategy == ""` and `len(cp) >= 2`; reactive: only regen if drift detected |
| `certregen.go::IPDriftDetected` | `ReadIPAMState` | same package (lifecycle) | WIRED | Line 69; reads /kind/ipam-state.json via ippin facade |
| `certregen.go::IPDriftDetected` | `defaultCmder` (docker inspect) | line 89 | WIRED | Inspects current container IP via runtime CLI |
| `ippin.go::RecordAndPinHAControlPlane` | `ProbeIPAMFn` | package-level var (line 98) | WIRED | ProbeIPAMFn = doctor.ProbeIPAM at production; verdict gates IP pin sequence |
| `ippin.go::pinContainer` | `/kind/ipam-state.json` | `docker exec sh -c 'cat >'` (line 143) | WIRED | JSON written with chmod 0600; network disconnect+connect --ip sequence follows |
| `check.go::allChecks` | `ipamprobe.go::newIPAMProbeCheck` | registry entry (line 80) | WIRED | Network category; count advanced from 24 to 25 |
| `check.go::allChecks` | `resumestrategy.go::newHAResumeStrategyCheck` | registry entry (line 85) | WIRED | Cluster category; count advanced from 25 to 26 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `resume.go::applyPinnedIPsBeforeCPStart` | `state.Network`, `state.IPv4` | `ReadIPAMState` → `docker cp` → JSON parse | YES — reads JSON persisted at create time from live container | FLOWING |
| `resume.go` Phase 4.5 | `drifted bool` | `IPDriftDetected` → `ReadIPAMState` + `docker inspect` | YES — compares recorded vs live IP | FLOWING |
| `certregen.go::RegenerateEtcdPeerCertsWholesale` | node commands | `node.Command("kubeadm", ...)` + `node.Command("mv", ...)` | YES — runs real commands inside CP containers | FLOWING (live cluster only) |
| `resumestrategy.go::haResumeStrategyCheck` | `labelValues` | `inspectContainerLabel` → `docker inspect` | YES — reads actual container labels | FLOWING (live cluster only) |
| `ipamprobe.go::ProbeIPAM` | `originalIP`, `postIP` | `docker inspect` + real stop/start cycle | YES — real container operations | FLOWING (live cluster only) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| allChecks count is 26 | `awk '/^var allChecks/,/^}/' check.go \| grep -E '^\s+new.+\(\),' \| wc -l` | 26 | PASS |
| ipam-probe registered in Network category | `go test ./pkg/internal/doctor/... -run TestAllChecks_IncludesIPAMProbe -count=1` | PASS | PASS |
| ha-resume-strategy registered in Cluster category | `go test ./pkg/internal/doctor/... -run TestAllChecks_IncludesHAResumeStrategy -count=1` | PASS | PASS |
| Single-CP guard present in ippin.go | `grep -n "len(cpContainers) <= 1" ippin.go` | line 94 | PASS |
| Single-CP guard present in docker/provider.go | `grep -n "len(cpNames) >= 2" docker/provider.go` | lines 115, 148 | PASS |
| No certregen. qualifier in resume.go (W2) | `grep -c 'certregen\.' pkg/internal/lifecycle/resume.go` | 0 | PASS |
| nerdctl short-circuit before any container ops | ipamprobe.go:77 `filepath.Base(binaryName) == "nerdctl"` returns VerdictUnsupported | VERIFIED | PASS |
| doctor package race (-count=1 single run) | `go test ./pkg/internal/doctor/... -count=1 -race` | ok | PASS (intermittent) |
| doctor package race (-count=3) | `go test ./pkg/internal/doctor/... -count=3 -race` | FAIL | FAIL — data race detected |
| lifecycle package race (-count=3) | `go test ./pkg/internal/lifecycle/... -count=3 -race` | ok | PASS |
| Build clean | `go build ./...` | (no output) | PASS |
| Vet clean | `go vet ./pkg/internal/...` | (no output) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| LIFE-09 (HA etcd peer TLS fix) | 52-01 through 52-04 | HA clusters resume cleanly after IP reassignment | PARTIALLY SATISFIED | Full code implementation verified; live cluster outcomes require human UAT (must-haves 1 and 2) |

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `pkg/internal/doctor/check_test.go` lines 46, 84, 116 | `t.Parallel()` + direct `allChecks = []Check{...}` write (no mutex) | WARNING | Races against Phase-52-added parallel readers `TestAllChecks_CountIs26` (line 242) and `TestAllChecks_IncludesHAResumeStrategy` (line 253); `-race -count=3` fails reliably; `-count=1` is intermittently clean (SUMMARY claim of "PASS" was a lucky run) |

No TODOs, placeholders, or empty implementations found in any Phase 52 production files. Stub classification checked: `return nil` in `RecordAndPinHAControlPlane` for single-CP is a correct early-return, not a stub (the D-locked zero-overhead guard). `certRegenSleeper` default to `time.Sleep` is production-correct initialization, not a stub.

### Human Verification Required

#### 1. HA Pause/Resume — Docker (must-haves 1 and 2)

**Test:** Create a 3-CP HA kinder cluster on Docker. Verify each CP has `/kind/ipam-state.json` and label `io.x-k8s.kinder.resume-strategy=ip-pinned` (or `cert-regen` if Docker IPAM probe failed). Run `kinder pause <cluster>` then `kinder resume <cluster>`.
**Expected:** All three control-plane nodes return to Ready state. `kubectl get nodes` shows all Ready. `kubectl exec -n kube-system <etcd-pod> -- etcdctl endpoint health --cluster` shows all members healthy. If ip-pinned: container IPs unchanged, no cert-regen log lines. If cert-regen: resume log shows "Regenerating etcd peer cert on <node> (N/M)" for each CP; etcd still healthy.
**Why human:** Requires live Docker daemon + real kinder HA cluster + actual IPAM behavior. Unit tests use fakeCmder/fakeNode. Maps to UAT-2 in 52-VALIDATION.md.

#### 2. HA Pause/Resume — etcd peer cert SAN content (UAT-1)

**Test:** On any CP node of a live HA cluster, run: `docker exec <cp> openssl x509 -in /etc/kubernetes/pki/etcd/peer.crt -text -noout | grep -A2 'Subject Alternative'`. Cross-check the IPs in the SAN list against `docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' <cp>`.
**Expected:** The container IP from `docker inspect` appears in the SAN list (alongside 127.0.0.1 and ::1). This validates that `kubeadm certs renew etcd-peer` targets the correct IPs for cert-regen to be effective.
**Why human:** Cannot read a real etcd peer cert from a unit test environment. Foundational to cert-regen correctness. RESEARCH Open Question 3 was resolved MEDIUM confidence; this UAT confirms it. Maps to UAT-1 in 52-VALIDATION.md.

### Gaps Summary

**One gap found (WARNING severity) — data race in `pkg/internal/doctor` test suite:**

Three pre-existing tests in `check_test.go` mutate the `allChecks` package-level variable while calling `t.Parallel()`:
- `TestRunAllChecks_PlatformSkip` (writes `allChecks` at line 57)
- `TestRunAllChecks_NilPlatformsRunsOnAll` (writes `allChecks` at line 89)
- `TestRunAllChecks_MultipleResultsPreserved` (writes `allChecks` at line 121)

Phase 52 added two new parallel readers that made the race reliably triggerable:
- `TestAllChecks_IncludesHAResumeStrategy` (line 253, reads via `AllChecks()`)
- `TestAllChecks_CountIs26` (line 242, reads via `AllChecks()`)

The race detector fires reliably with `-count=3 -race` (cross-goroutine read/write of `allChecks` slice header). With `-count=1`, the race is intermittent — the SUMMARY's claim of "PASS" was based on a lucky single run. All other Phase 52 packages (lifecycle, ippin, docker, podman) pass `-count=3 -race` cleanly.

**Fix:** Remove `t.Parallel()` from the three `allChecks`-mutating tests. This pattern was already correctly applied in `ipamprobe_test.go` (the `ipamProbeCmder`-mutating tests do NOT use `t.Parallel()`, with an explanatory comment). The same convention should be applied to the `allChecks` mutating tests in `check_test.go`. This is a low-risk change to test-only code that does not affect production behavior.

**Two must-haves require human verification (must-haves 1 and 2):** The code paths are fully implemented and wired — `applyPinnedIPsBeforeCPStart` at Phase 1.5 and `RegenerateEtcdPeerCertsWholesale` at Phase 4.5 are both reachable, substantive, and data-flowing. Live cluster testing per 52-VALIDATION.md UAT-1 and UAT-2 is required to confirm end-to-end behavior. These were explicitly pre-declared as human-needed in the phase context.

---

_Verified: 2026-05-10T14:00:00Z_
_Verifier: Claude (gsd-verifier)_
