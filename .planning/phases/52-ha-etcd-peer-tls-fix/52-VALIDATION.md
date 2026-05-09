# Phase 52: HA Etcd Peer-TLS Fix — Validation Map

**Derived from:** 52-RESEARCH.md `## Validation Architecture` + per-plan `must_haves`.
**Purpose:** Map every observable truth in every plan to a concrete validation harness so Nyquist's "every truth must have an automated check" rule is satisfied. Manual UAT rows are explicitly flagged where automation is impossible at plan time.

**Doctor count baseline (verified live in `pkg/internal/doctor/check.go`):**
| Stage | Count | Test name |
|-------|-------|-----------|
| Pre-Phase-52 | 24 | (no count test yet) |
| After Plan 52-01 | 25 | `TestAllChecks_CountIs25` |
| After Plan 52-04 | 26 | `TestAllChecks_CountIs26` (renamed from CountIs25) |

---

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (`testing` package), existing kinder test patterns |
| Config file | None |
| Quick run command | `go test ./pkg/internal/lifecycle/... ./pkg/internal/doctor/... -count=1` |
| Race-mode command | `go test ./pkg/internal/lifecycle/... ./pkg/internal/doctor/... -count=1 -race` |
| Build gate | `go build ./...` |
| Static analysis | `go vet ./...` |
| Full suite | `go test ./pkg/internal/... ./pkg/cluster/...` |

---

## Plan 52-01 — IPAM Probe + Doctor Registration

| Truth (from 52-01 must_haves) | Test Type | Harness | Automated Command |
|-------------------------------|-----------|---------|-------------------|
| Automated probe determines per-runtime whether `network connect --ip` survives stop/start with order inversion | unit | `pkg/internal/doctor/ipamprobe_test.go::TestIPAMProbe_DockerHappyPath` + `TestIPAMProbe_DockerIPChanged` | `go test ./pkg/internal/doctor/... -run TestIPAMProbe -v -count=1` |
| Probe verdict is one of three: ip-pinned / cert-regen / cert-regen+warn | unit | `TestIPAMProbe_NerdctlShortCircuit`, `TestIPAMProbe_DockerNetworkConnectFails`, `TestIPAMProbe_PermissionDenied` | `go test ./pkg/internal/doctor/... -run TestIPAMProbe -v -count=1` |
| Probe registered as `ipam-probe` in Network category | unit | `TestAllChecks_IncludesIPAMProbe` | `go test ./pkg/internal/doctor/... -run TestAllChecks -v -count=1` |
| Probe creates and tears down only an ephemeral scratch network; no artifacts on either pass or fail | unit | `TestIPAMProbe_CleanupOnEarlyFailure` (asserts deferred cleanup commands invoked) | `go test ./pkg/internal/doctor/... -run TestIPAMProbe_CleanupOnEarlyFailure -v -count=1` |
| nerdctl is hard-coded to cert-regen verdict (no container ops) | unit | `TestIPAMProbe_NerdctlShortCircuit` (fakeCmder fails the test if invoked) | `go test ./pkg/internal/doctor/... -run TestIPAMProbe_NerdctlShortCircuit -v -count=1` |
| Probe does NOT create a kind cluster; uses non-overlapping subnet 172.200.0.0/24 | unit | `TestIPAMProbe_UsesNonOverlappingSubnet` | `go test ./pkg/internal/doctor/... -run TestIPAMProbe_UsesNonOverlappingSubnet -v -count=1` |
| Doctor count test pinned to 25 after this plan (baseline 24 + probe = 25) | unit | `TestAllChecks_CountIs25` | `go test ./pkg/internal/doctor/... -run TestAllChecks_CountIs25 -v -count=1` |

**Key links covered:** `check.go → ipamprobe.go` (verified by `TestAllChecks_IncludesIPAMProbe`); `ipamprobe.go → exec.Command` (verified implicitly by all probe tests via fakeCmder injection).

---

## Plan 52-02 — IP-Pin Recording + Provider Wiring

| Truth (from 52-02 must_haves) | Test Type | Harness | Automated Command |
|-------------------------------|-----------|---------|-------------------|
| Each CP container has `/kind/ipam-state.json` written with assigned IPv4 + network name | unit | `TestRecordAndPinHAControlPlane_VerdictIPPinned_HappyPath` (asserts `sh -c 'cat > /kind/ipam-state.json'` invocation per CP) | `go test ./pkg/internal/lifecycle/... -run TestRecordAndPinHAControlPlane -v -count=1` |
| Each CP container reconnected with `network connect --ip <recorded-ip>` | unit | `TestRecordAndPinHAControlPlane_VerdictIPPinned_HappyPath` (asserts disconnect + connect-with-ip per CP in order) | `go test ./pkg/internal/lifecycle/... -run TestRecordAndPinHAControlPlane -v -count=1` |
| Each CP container labeled `io.x-k8s.kinder.resume-strategy=ip-pinned` at `docker run` time | unit | `TestProvisionAttachesStrategyLabel_Docker` (`pkg/cluster/internal/providers/docker/provider_ippin_test.go`) + `TestProvisionAttachesStrategyLabel_Podman` (`pkg/cluster/internal/providers/podman/provider_ippin_test.go`) | `go test ./pkg/cluster/internal/providers/docker/... ./pkg/cluster/internal/providers/podman/... -run TestProvisionAttachesStrategyLabel -v -count=1` |
| Single-CP clusters skip all overhead (no probe, no extra docker calls, no label, no state file) | unit | `TestRecordAndPinHAControlPlane_SingleCPSkip` + `TestProvisionSingleCP_NoProbe` | `go test ./pkg/internal/lifecycle/... ./pkg/cluster/internal/providers/docker/... ./pkg/cluster/internal/providers/podman/... -run "SingleCP" -v -count=1` |
| nerdctl: containers NOT pinned, NOT labeled `ip-pinned`; resume-strategy label set to `cert-regen` instead | unit | `TestRecordAndPinHAControlPlane_NerdctlVerdict_NoReconnect` (nerdctl provider remains unchanged — its containers never enter the ip-pin branch because nerdctl's `Provision` does not call `RecordAndPinHAControlPlane`; defaults to no label = legacy = cert-regen at resume) | `go test ./pkg/internal/lifecycle/... -run TestRecordAndPinHAControlPlane_NerdctlVerdict_NoReconnect -v -count=1` |
| Probe runtime error → fall back to cert-regen labeling + warn log | unit | `TestRecordAndPinHAControlPlane_ProbeRuntimeError` + provider-level `TestProvisionLogsWarnOnProbeError` | `go test ./pkg/internal/lifecycle/... ./pkg/cluster/internal/providers/docker/... -run "ProbeRuntimeError|ProvisionLogsWarn" -v -count=1` |
| IP from `docker inspect` validated before passing to `--ip` (V5 input validation) | unit | `TestRecordAndPinHAControlPlane_InspectMalformedIP` | `go test ./pkg/internal/lifecycle/... -run TestRecordAndPinHAControlPlane_InspectMalformedIP -v -count=1` |
| Disconnect failure halts the loop (no half-pinning); connect-failure recovery double-failure also halts | unit | `TestRecordAndPinHAControlPlane_DisconnectFailureIsFatal` + `TestRecordAndPinHAControlPlane_ConnectFailureRecoveryFails` (W1 fix — see 52-02 Task 1 behavior block) | `go test ./pkg/internal/lifecycle/... -run "TestRecordAndPinHAControlPlane_DisconnectFailureIsFatal\|TestRecordAndPinHAControlPlane_ConnectFailureRecoveryFails" -v -count=1` |
| `ReadIPAMState` round-trips the JSON via `docker cp` | unit | `TestRecordAndPinHAControlPlane_ReadbackJSON` + `TestIPAMState_RoundTrip` | `go test ./pkg/internal/lifecycle/... -run "TestIPAMState_RoundTrip|ReadbackJSON" -v -count=1` |
| `go build ./...` succeeds (no import cycle introduced) | build | `go build ./...` | `go build ./...` |

**Key links covered:** docker `provider.go::Provision → RecordAndPinHAControlPlane` (verified by `TestProvisionAttachesStrategyLabel_Docker`); podman `provider.go::Provision → RecordAndPinHAControlPlane` (verified by `TestProvisionAttachesStrategyLabel_Podman`); `RecordAndPinHAControlPlane → doctor.ProbeIPAM` (verified by injected `probeIPAMFn`); `RecordAndPinHAControlPlane → /kind/ipam-state.json` (verified by `TestRecordAndPinHAControlPlane_VerdictIPPinned_HappyPath` asserting the `sh -c 'cat > /kind/ipam-state.json'` command).

---

## Plan 52-03 — Resume-Time Orchestration (IP-Pin + Cert-Regen)

| Truth (from 52-03 must_haves) | Test Type | Harness | Automated Command |
|-------------------------------|-----------|---------|-------------------|
| HA + ip-pinned: BEFORE `docker start` on each CP, recorded IP from `/kind/ipam-state.json` is reapplied via disconnect+connect while container is still `exited` | unit | `TestResume_HAWithIPPinned_ReconnectsBeforeCPStart` | `go test ./pkg/internal/lifecycle/... -run TestResume_HAWithIPPinned -v -count=1 -race` |
| HA + cert-regen (or absent label = legacy): AFTER all containers started, `kubeadm certs renew etcd-peer` runs on every CP and etcd static pod is cycled wholesale | unit | `TestResume_HAWithCertRegen_IPDrift_RunsWholesaleRegen` + `TestResume_HALegacyNoLabel_RunsWholesaleRegen` | `go test ./pkg/internal/lifecycle/... -run "HAWithCertRegen|HALegacyNoLabel" -v -count=1 -race` |
| Cert-regen is REACTIVE — runs only when current inspect IP differs from recorded value (or recording absent) | unit | `TestResume_HAWithCertRegen_NoIPDrift_SkipsRegen` + `TestIPDriftDetected_NoDrift` + `TestIPDriftDetected_Drift` + `TestIPDriftDetected_LegacyNoFile` | `go test ./pkg/internal/lifecycle/... -run "TestIPDriftDetected\|HAWithCertRegen_NoIPDrift" -v -count=1` |
| Cert-regen is WHOLESALE across all CP nodes — never partial | unit | `TestRegenerateEtcdPeerCertsWholesale_HappyPath` (asserts full 5-step sequence × N CPs, not interleaved) | `go test ./pkg/internal/lifecycle/... -run TestRegenerateEtcdPeerCertsWholesale_HappyPath -v -count=1` |
| Cert-regen mid-resume failure halts with structured diagnostic; no auto-recovery | unit | `TestRegenerateEtcdPeerCertsWholesale_RenewFailureHalts` + `TestResume_HACertRegen_RegenFailsHaltsResume` | `go test ./pkg/internal/lifecycle/... -run "RegenFailure\|RegenFailsHalts" -v -count=1` |
| Single-CP and non-HA clusters unaffected (Resume control flow does not enter either branch) | unit | `TestResume_SingleCP_NoStrategyDispatch` (NEW) + existing `TestResume_SingleCP_HappyPath` continues to pass | `go test ./pkg/internal/lifecycle/... -run "SingleCP" -v -count=1` |
| Cert-regen branch shows single-line spinner per CP node matching existing UX | unit | `TestRegenerateEtcdPeerCertsWholesale_LoggerProgress` | `go test ./pkg/internal/lifecycle/... -run TestRegenerateEtcdPeerCertsWholesale_LoggerProgress -v -count=1` |
| nerdctl with stale `ip-pinned` label downgrades to cert-regen at resume time (defense in depth) | unit | `TestResume_HAIPPin_NerdctlPath` | `go test ./pkg/internal/lifecycle/... -run TestResume_HAIPPin_NerdctlPath -v -count=1` |
| Pin-failure halts Resume (no readiness gate, no further CPs reconnected) | unit | `TestResume_HAIPPin_DisconnectFailsHaltsResume` | `go test ./pkg/internal/lifecycle/... -run TestResume_HAIPPin_DisconnectFailsHaltsResume -v -count=1` |
| ReadIPAMState failure on a CP carrying ip-pinned label HALTS Resume (W2 Option A — no soft skip) | unit | `TestResume_HAIPPin_ReadIPAMStateFailureHalts` (W2 fix — see 52-03 Task 2 behavior block) | `go test ./pkg/internal/lifecycle/... -run TestResume_HAIPPin_ReadIPAMStateFailureHalts -v -count=1` |
| Sleep is injectable so tests do not block 45 s | unit | `TestRegenerateEtcdPeerCertsWholesale_SleepIsInjectable` | `go test ./pkg/internal/lifecycle/... -run TestRegenerateEtcdPeerCertsWholesale_SleepIsInjectable -v -count=1` |

**Key links covered:** `resume.go → ippin.go::ReadIPAMState` (verified by `TestResume_HAWithIPPinned_ReconnectsBeforeCPStart`); `resume.go → certregen.go::RegenerateEtcdPeerCertsWholesale` (verified by `TestResume_HAWithCertRegen_IPDrift_RunsWholesaleRegen`); `certregen.go → kubeadm certs renew + static pod cycle` (verified by `TestRegenerateEtcdPeerCertsWholesale_HappyPath`'s exact-sequence assertion).

---

## Plan 52-04 — Resume-Strategy Doctor Check

| Truth (from 52-04 must_haves) | Test Type | Harness | Automated Command |
|-------------------------------|-----------|---------|-------------------|
| `kinder doctor` lists `ha-resume-strategy` in Cluster category | unit | `TestAllChecks_IncludesHAResumeStrategy` + `TestHAResumeStrategyCheck_CategoryAndName` | `go test ./pkg/internal/doctor/... -run "TestAllChecks_IncludesHAResumeStrategy\|CategoryAndName" -v -count=1` |
| Single-CP cluster (or no cluster): skip with clear message | unit | `TestHAResumeStrategyCheck_SingleCP` + `TestHAResumeStrategyCheck_NoCluster` | `go test ./pkg/internal/doctor/... -run "TestHAResumeStrategyCheck_SingleCP\|TestHAResumeStrategyCheck_NoCluster" -v -count=1` |
| All CPs labeled ip-pinned → status=ok | unit | `TestHAResumeStrategyCheck_AllIPPinned` | `go test ./pkg/internal/doctor/... -run TestHAResumeStrategyCheck_AllIPPinned -v -count=1` |
| All CPs labeled cert-regen → status=warn with reason | unit | `TestHAResumeStrategyCheck_AllCertRegen` | `go test ./pkg/internal/doctor/... -run TestHAResumeStrategyCheck_AllCertRegen -v -count=1` |
| Label absent on any CP (legacy) → status=warn with "(legacy)" message | unit | `TestHAResumeStrategyCheck_LegacyNoLabel` | `go test ./pkg/internal/doctor/... -run TestHAResumeStrategyCheck_LegacyNoLabel -v -count=1` |
| Mixed labels across CPs → status=fail (corruption signal) | unit | `TestHAResumeStrategyCheck_Mixed` | `go test ./pkg/internal/doctor/... -run TestHAResumeStrategyCheck_Mixed -v -count=1` |
| Inspect failure → status=warn (not fail; transient) | unit | `TestHAResumeStrategyCheck_InspectFails` | `go test ./pkg/internal/doctor/... -run TestHAResumeStrategyCheck_InspectFails -v -count=1` |
| Multiple kinder clusters present → status=warn with "check kubectl context" reason | unit | `TestHAResumeStrategyCheck_MultipleClustersDetected` | `go test ./pkg/internal/doctor/... -run TestHAResumeStrategyCheck_MultipleClustersDetected -v -count=1` |
| Doctor count test pinned to 26 (baseline 24 + probe + ha-resume-strategy = 26) | unit | `TestAllChecks_CountIs26` (renamed from `TestAllChecks_CountIs25`) | `go test ./pkg/internal/doctor/... -run TestAllChecks_CountIs26 -v -count=1` |
| Check does NOT invoke `ProbeIPAM` (pure inspection) | unit | `TestHAResumeStrategyCheck_DoesNotRunProbe` | `go test ./pkg/internal/doctor/... -run TestHAResumeStrategyCheck_DoesNotRunProbe -v -count=1` |

**Key links covered:** `check.go → resumestrategy.go` (verified by `TestAllChecks_IncludesHAResumeStrategy`); `resumestrategy.go → docker inspect labels` (verified by all branch tests via `resumeStrategyInspector` injection).

---

## Phase-Wide Acceptance Tests

| Acceptance Truth | Test Type | Harness | Automated Command |
|------------------|-----------|---------|-------------------|
| `go build ./...` succeeds with all four plans landed | build | full module build | `go build ./...` |
| `go vet ./...` clean | static | vet | `go vet ./...` |
| Full unit suite passes with `-race` | unit | aggregate | `go test ./pkg/internal/lifecycle/... ./pkg/internal/doctor/... ./pkg/cluster/internal/providers/docker/... ./pkg/cluster/internal/providers/podman/... -count=1 -race` |
| LIFE-09 requirement coverage: every plan's `requirements:` field lists `LIFE-09` and at least one truth per plan ties back to it | static | `gsd-sdk query frontmatter.validate` per plan | `for f in .planning/phases/52-ha-etcd-peer-tls-fix/52-0?-PLAN.md; do gsd-sdk query frontmatter.validate "$f" --schema plan; done` |

---

## Manual UATs (Cannot Be Automated at Plan Time)

These are recorded here because the verification requires either (a) a live container runtime in a state the unit suite cannot create, or (b) an existing kinder HA cluster, neither of which is guaranteed during plan execution. Each UAT MUST be executed by the human reviewer (or by an executor with `kinder` + a running runtime) before the phase is considered shipped.

### UAT-1 — etcd peer cert SAN content (resolves RESEARCH Open Question 3)

**Why this is manual:** Plan 52-01's probe runs on a scratch container with no kubeadm install. There is no in-suite way to read a real etcd peer cert at plan time. RESEARCH Open Question 3 (resolved with MEDIUM confidence based on kubeadm docs) is foundational to the cert-regen path's correctness. If the SAN list embeds pod-CIDR IPs instead of container IPs, cert-regen does not fix peer TLS connectivity.

**Steps:**
1. Create a fresh kinder HA cluster on Docker (any kindest/node version supported by this kinder build):
   ```bash
   kinder create cluster --config <path-to-3-cp-ha-config>.yaml
   ```
2. On any CP node, read the etcd peer cert SAN list:
   ```bash
   docker exec <cp-container-name> openssl x509 \
     -in /etc/kubernetes/pki/etcd/peer.crt -text -noout \
     | grep -A2 'Subject Alternative'
   ```
3. Capture the output. Cross-check the IPs in the SAN list against:
   ```bash
   docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' <cp-container-name>
   ```

**PASS criteria:** the container IP from `docker inspect` appears in the SAN list (alongside `127.0.0.1`, `::1`, and possibly the node's hostname).

**FAIL criteria:** the SAN list contains pod-CIDR IPs (e.g., `10.244.x.x`) and not the container IP. If FAIL: revise Plan 52-03 — the cert-regen path's correctness is invalidated and a different mitigation is required.

**Recording:** append the captured SAN output and the verdict to `52-04-SUMMARY.md` (or to a phase-level UAT log in `RETROSPECTIVE.md`).

### UAT-2 — Live HA pause/resume on Docker (end-to-end)

**Why this is manual:** Unit tests use fakeCmder/fakeNode and cannot exercise the real Docker IPAM behavior that motivated this phase.

**Steps:**
1. Create a 3-CP HA cluster on Docker with this kinder build.
2. Verify each CP has `/kind/ipam-state.json` and label `io.x-k8s.kinder.resume-strategy=ip-pinned`:
   ```bash
   for c in <cp1> <cp2> <cp3>; do docker exec $c cat /kind/ipam-state.json; done
   docker inspect --format '{{index .Config.Labels "io.x-k8s.kinder.resume-strategy"}}' <cp1>
   ```
3. `kinder pause <cluster>` then `kinder resume <cluster>`.
4. Verify each CP's IP is unchanged across the pause/resume cycle.
5. Verify etcd cluster health: `kubectl exec -n kube-system <etcd-pod> -- etcdctl endpoint health --cluster`.

**PASS criteria:** all 3 etcd members healthy after resume; container IPs unchanged.

**FAIL criteria:** any etcd member unhealthy or any IP changed despite the ip-pinned label. If FAIL on Docker: regression in Plan 52-02 or 52-03 — revise.

### UAT-3 — Live HA pause/resume on Podman (end-to-end)

**Why this is manual:** Plan 52-02 wires podman provider for IP pinning, but plan-time validation of podman is limited (CLI flag confirmed; live container cycle not verified). Per CONTEXT.md "each runtime probed independently," podman MUST PASS the probe and exercise the ip-pin path. UAT-3 is a SUPPLEMENT to the unit-level `TestProvisionAttachesStrategyLabel_Podman` test (which is required, full-strength — see 52-02 Task 2). It is NOT a substitute.

**Steps:** Same as UAT-2 but using `podman` runtime.

**PASS criteria:** podman cluster behaves identically to docker — ip-pinned label set, IPs preserved across resume, etcd healthy.

**FAIL criteria:** if podman probe FAILS, log the verdict + reason and confirm the cluster transparently fell through to cert-regen labels (label = `cert-regen`, no `/kind/ipam-state.json` file). This is acceptable behavior. The only true FAIL is silent breakage (e.g., probe returned ip-pinned but reconnect failed at resume time without a downgrade).

### UAT-4 — Live HA pause/resume on nerdctl (end-to-end fallback path)

**Why this is manual:** nerdctl is the explicit cert-regen-always runtime per RESEARCH; this UAT verifies the fallback path actually works on a real nerdctl install.

**Steps:**
1. Install nerdctl + containerd (developer environment).
2. Create a 3-CP HA cluster with `nerdctl` runtime.
3. Verify each CP has label `io.x-k8s.kinder.resume-strategy=cert-regen` and NO `/kind/ipam-state.json`.
4. `kinder pause` + `kinder resume`.
5. Observe single-line spinner "Regenerating etcd peer cert on <node> (N/M)" during resume.
6. Verify etcd health post-resume.

**PASS criteria:** etcd healthy after resume; cert-regen ran wholesale; no IP-pin code path executed (no `network connect --ip` invocations in any kinder log).

**FAIL criteria:** kinder attempts `nerdctl network connect --ip` (would fail with "unknown command"); etcd unhealthy after resume.

---

## Coverage Audit

Every truth in every plan's `must_haves` block has a row in this document. Every key link has at least one test row that exercises it. Every foundational assumption that cannot be unit-verified is listed under Manual UATs with explicit PASS/FAIL criteria.

If during execution any plan adds a truth or removes one, this VALIDATION.md MUST be updated in the same commit so the map stays in sync.
