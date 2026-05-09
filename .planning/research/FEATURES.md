# Feature Research

**Domain:** Local Kubernetes dev-tool hardening milestone (batteries-included kind fork)
**Researched:** 2026-05-09
**Confidence:** HIGH (cert-manager, MetalLB, Envoy Gateway: official docs + release notes; signing: official Apple/GoReleaser docs; etcd TLS: kubeadm docs + comparable tool behavior; doctor cosmetics: etcd API + kind project)

---

## Scope

This file covers only the **new user-visible behavior in v2.4**. Existing features (7 addons, 4 lifecycle commands, 23+ doctor checks, GoReleaser distribution) are shipped and not re-researched.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features that, if missing or broken, make the milestone feel incomplete or regressive.

| Feature | Why Expected | Complexity | Dependency | Notes |
|---------|--------------|------------|------------|-------|
| Addon version bumps (all 7) with release note callout | Users expect a maintained tool to track upstream. Any stale addon that ships a known CVE is a support problem. | LOW | Existing embedded manifest pipeline (`go:embed`) | cert-manager 1.17/1.18 have user-visible behavior changes (see below). Others are low-risk incremental bumps. Release note callout required for cert-manager; optional for patch bumps. |
| cert-manager rotationPolicy=Always default (1.18) disclosed | cert-manager 1.18 changes `Certificate.Spec.PrivateKey.rotationPolicy` default from `Never` to `Always`. Existing clusters that upgrade will see new private keys generated on next renewal — silent breakage if user relies on key stability. | LOW | cert-manager addon already shipped | Must appear in CHANGELOG and release notes. kinder users only hit this on `kinder create cluster` with fresh clusters — existing clusters are not mutated — so impact is low unless user also runs `kinder upgrade` (anti-feature; see below). |
| macOS Gatekeeper friction eliminated for Homebrew Cask users | Users who `brew install kinder-cask` expect the binary to run without a quarantine dialog or manual `xattr -d` step. This is now table-stakes for any CLI tool distributed via Homebrew. | LOW | GoReleaser already produces darwin binaries | Ad-hoc signing (`codesign --sign -`) via GoReleaser post_hooks. No Apple Developer account required. Homebrew itself applies ad-hoc signatures automatically in some paths but not reliably for Casks. |
| Windows PR-CI build gating (internal-facing, user-impact: no Windows regression) | Users on Windows amd64 expect the GoReleaser-produced `kinder_windows_amd64.zip` to actually run. A CI build step ensures no PR silently breaks Windows compilation. | LOW | GoReleaser already produces windows-amd64; pure-Go build (CGO_ENABLED=0) | Pure GOOS=windows go build ./... step on every PR, not a test runner. No Windows-specific test suite needed in v2.4. |
| etcd peer-TLS regen on HA pause/resume (no user-visible breakage) | HA clusters with 3 control-plane nodes use etcd peer certs with IP SANs. When Docker reassigns container IPs after `kinder pause` / `kinder resume`, etcd peers cannot authenticate each other. Users expect `kinder resume` to produce a working cluster — silent etcd failure is a regression against the v2.3 pause/resume feature. | MEDIUM | `kinder pause`/`kinder resume` shipped in v2.3 | Two valid approaches: (a) conditional regen — detect IP change, regenerate certs; (b) IP pinning — reserve static IPs on the cluster's Docker network so they never change. IP pinning is simpler and has fewer failure modes (see detailed notes below). |
| Phase 47 + 51 live UAT closed with documented evidence | v2.3 shipped pause/resume and K8s 1.36 sync but marked UAT pending. v2.4 must close these as verified deliverables. | LOW | Pause/resume (Ph47) and Envoy LB migration (Ph51) both shipped | See UAT closure patterns below. A shell script with `//go:build manual` tag or a recorded command log is the minimum; asciinema is a differentiator. |
| cluster-node-skew check no longer warns on LB containers | Envoy LB (previously HAProxy) containers are not Kubernetes nodes — they do not carry a Kubernetes version. A version-skew warning on an LB container is a false positive that erodes trust in all doctor output. | LOW | cluster-node-skew check shipped in v2.2; Envoy LB migration shipped in v2.3 | Fix: filter nodes by role — skip containers whose name/role identifies them as LB, not a control-plane or worker. |
| cluster-resume-readiness shows actionable text, not raw etcdctl JSON | Current behavior exposes raw `etcdctl endpoint health --write-out json` output. Users need "1/3 members healthy, quorum at risk" not a JSON blob. | LOW | cluster-resume-readiness check shipped in v2.3 | `etcdctl endpoint health --write-out json` returns `[{"endpoint":..., "health": bool, "took":...}]` per member. Parse: count `health:true` vs total, synthesize "N/M healthy" string. |

### Differentiators (Competitive Advantage)

Features that make kinder stand out versus kind, k3d, or minikube in this maintenance cycle.

| Feature | Value Proposition | Complexity | Dependency | Notes |
|---------|-------------------|------------|------------|-------|
| Conditional etcd-TLS regen (IP-change-only) | Regen only when IP actually changes means zero overhead on the common case (pause on same host, resume immediately). Avoids unnecessary cert churn on fast workstations. | MEDIUM | HA pause/resume (v2.3), etcd cert infrastructure | Requires reading current peer IP from the running container, comparing to IP in the existing cert SAN, and only triggering `kubeadm init phase certs etcd-peer` if mismatched. Neither k3d nor kind handle this automatically — k3d IPAM was the fix, not cert regen. |
| cert-manager 1.18 rotationPolicy disclosure in release notes | Proactively surfacing a breaking default change (rotationPolicy: Always) builds user trust. Comparable tools rarely call this out explicitly. | LOW | cert-manager addon | One paragraph in CHANGELOG + website release notes page. |
| Structured etcd health parsing in doctor | "2/3 members healthy, quorum intact" vs "1/3 members healthy, quorum at risk" vs "cluster unavailable" is genuinely useful — not available in kind, k3d, or kubeadm out of the box without piping etcdctl into jq. | LOW | cluster-resume-readiness (v2.3) | Parse JSON array from `etcdctl endpoint health --write-out json`, count health:true, derive quorum state (need >N/2 healthy). |
| IP pinning via Docker network static IP reservation | Simpler than cert regen: reserve the same IP for each HA node across pause/resume cycles. Docker named networks support `--ip` flag. k3d solved their IPAM problem this way (implemented in v4.4.5). kinder could use the same pattern. | MEDIUM | HA cluster creation (v2.3), Docker provider | Store assigned IPs in kinder cluster state file on `create`. On `resume`, use `docker network connect --ip <stored-ip>` to restore. Requires docker network to survive pause (it does — `docker pause` stops containers, not networks). |
| DEBT-04 race fix under -race | Fixing `allChecks` global mutation under `t.Parallel()` means kinder's own test suite passes `go test -race ./...` cleanly. This is a quality signal visible to contributors on CI. | LOW | Doctor check test suite (v2.1) | `allChecks` is a package-level var mutated in `TestMain` or per-test setup. Fix: copy slice per test, or initialize once in `TestMain` and treat as read-only. No user-visible behavior change. |

### Anti-Features (Do Not Build in v2.4)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| `kinder upgrade` to apply new addon versions to existing clusters | Users naturally ask: "can I get cert-manager 1.18 without recreating?" | Upgrading addon CRDs in-place (cert-manager 1.18's rotationPolicy change, cert GC) is risky without a full migration path. Kind itself has no upgrade path; kinder's model is create-destroy. In-place upgrade would require CRD diffing, rollback logic, and compatibility matrix — out of scope for a hardening milestone and architecturally complex. | Document that addon bumps require `kinder delete cluster && kinder create cluster`. New clusters get the bumped version automatically. |
| Apple notarization (full Developer ID signing) | Users and articles suggest notarization for "fully signed" macOS experience | Requires Apple Developer account ($99/yr), provisioning profile, and must run on macOS CI (not linux runners). GoReleaser Pro supports it; the free tier does not. Ad-hoc signing removes the quarantine friction for Homebrew users at zero cost. Notarization is a v3.x consideration if distribution grows to non-Homebrew channels (direct download, pkg installer). | Ad-hoc signing via `codesign --sign -` in GoReleaser post_hooks for darwin builds. Covers the Gatekeeper quarantine problem for Homebrew Cask. |
| Windows test runner in CI (full test suite on windows-latest) | Logical extension of adding a Windows build step | Windows containers behave differently; kind/kinder's container provider relies on Docker/Podman Linux semantics. A full Windows test suite would require Docker Desktop Windows licensing, runner costs, and significant test scaffold work. The v2.4 goal is preventing Windows compilation breakage on PRs, not validating Windows runtime behavior. | `GOOS=windows go build ./...` on linux runner with CGO_ENABLED=0. Validates compilation; runtime validation deferred. |
| Etcd TLS regen on every resume (unconditional) | Simplest code path — always regen, never worry about stale certs | Unconditional regen requires running `kubeadm` inside the container on every resume, adding 5–15 seconds to resume time. Also risks cert state divergence if regen fails midway. The right default is conditional (only if IP changed) or IP pinning (so IPs never change). | IP pinning (preferred) or conditional regen (detect SAN mismatch before regenerating). |
| Envoy Gateway bump to 1.7 in v2.4 | Latest stable is 1.7.2 (April 2026) | 1.3→1.4 has 4 breaking changes: readiness port change (19003), access log format (JSON not text), xDS snapshot behavior, extension manager error handling. 1.4→1.7 likely has additional changes not fully researched. Bumping Envoy Gateway requires re-embedding the manifest, validating xDS config structure, and testing the readiness probe. This is a medium-risk addon bump that belongs in v2.5 with dedicated UAT, not lumped into a hardening milestone. | Hold at 1.3.1 for v2.4. Document the hold with rationale. Bump in a dedicated v2.5 phase with UAT. |
| Headlamp v0.41 bump in v2.4 | v0.41 adds rollback, multi-cluster logout | Minor UX feature additions; no security relevance. v0.40.1 is stable and current. Addon bumps should be batched where low-risk. Headlamp has no breaking changes between 0.40 and 0.41 per release notes. Low risk — include as part of routine bump. | Include in routine addon audit bump. Flag as low-risk. |

---

## Feature Dependencies

```
macOS ad-hoc signing
    └──requires──> GoReleaser darwin build targets (v2.0)
    └──requires──> GoReleaser post_hooks capability (GoReleaser docs confirmed)

etcd peer-TLS regen / IP pinning
    └──requires──> kinder pause/resume (v2.3 Phase 47)
    └──requires──> HA cluster creation with 3+ control-plane nodes (v2.3)
    └──uses──> Docker network static IP assignment (Docker API)

Phase 47 + 51 UAT closure
    └──requires──> pause/resume (Phase 47, v2.3) — code already shipped
    └──requires──> Envoy LB migration (Phase 51, v2.3) — code already shipped

cluster-node-skew fix
    └──requires──> cluster-node-skew check (v2.2)
    └──requires──> Envoy LB migration (Phase 51 — LB container is now Envoy, not HAProxy)

cluster-resume-readiness reason text
    └──requires──> cluster-resume-readiness check (v2.3)
    └──uses──> etcdctl endpoint health JSON output format

DEBT-04 race fix
    └──requires──> allChecks doctor test infrastructure (v2.1)

Addon bumps (cert-manager, MetalLB, Headlamp, Metrics Server, local-path-provisioner)
    └──requires──> go:embed manifest pipeline (v1.0+)
    └──cert-manager 1.18 bump──enables──> rotationPolicy:Always disclosure

Windows PR-CI build step
    └──requires──> GoReleaser windows-amd64 target (v2.0)
    └──enables──> PR-time compilation gating (new in v2.4)
```

### Dependency Notes

- **etcd peer-TLS regen requires pause/resume**: The architectural gap only surfaces when a cluster is paused, the host loses the Docker network state or reassigns IPs, and then resumed. Single-node clusters are unaffected (single etcd member, no peer TLS).
- **cluster-node-skew fix requires Envoy LB migration**: Pre-v2.3, the LB container was HAProxy; post-v2.3 it is Envoy. The false-positive warning occurs because the check inspects all containers in the cluster network, including the LB container, which has no Kubernetes version label. The fix is the same regardless of LB type — filter by node role.
- **Windows CI step does not require a Windows runner**: `GOOS=windows go build ./...` compiles cross-platform from a Linux runner. CGO_ENABLED=0 (already required for kind-style builds) makes this trivially portable.

---

## Addon Bump Assessment

### cert-manager 1.16.3 → 1.17.x / 1.18.x

**1.16 → 1.17 breaking changes (LOW user impact):**
- RSA keys 3072+ bits now use SHA-384/SHA-512. Broad support; unlikely to affect kinder users with default cert configs.
- Structured logging replaces unstructured logs. Only breaks log-scraping pipelines.
- `ValidateCAA` feature gate deprecated (removed in 1.18). kinder does not set this gate.

**1.16 → 1.18 breaking changes (MEDIUM user impact, disclosure required):**
- `Certificate.Spec.PrivateKey.rotationPolicy` default changed from `Never` to `Always`. On fresh cluster creation, new certificates will rotate private keys on renewal by default. Users who expected key stability (e.g., pinning public key fingerprints in config) are affected.
- `Certificate.Spec.RevisionHistoryLimit` now defaults to 1. Stale CertificateRequest resources are GC'd automatically on upgrade — not relevant to fresh kinder clusters.
- ACME HTTP01 Ingress PathType changed to `Exact` (was `ImplementationSpecific`). Affects ACME users with ingress-nginx 1.8.0–1.12.5 (bug rejecting dots in paths). kinder's default cert-manager use case is self-signed certs, not ACME — low impact.
- No CRD migrations required between these versions.

**Recommendation:** Bump to 1.17 for v2.4 (lower risk). Disclose rotationPolicy change in CHANGELOG. 1.18 can follow in v2.5 once ACME PathType compat is validated.

### MetalLB 0.15.3 → latest

**Assessment:** MetalLB 0.15.x has no breaking changes for L2 mode between 0.14 and 0.15. The 0.15 series introduced BGP improvements and minor security hardening. kinder uses L2 mode only. The embedded manifest should remain compatible.

**Recommendation:** Audit current stable (check releases page for 0.15.4+). If only patch releases, bump is LOW risk. No user callout needed.

### Envoy Gateway 1.3.1 → 1.4.x

**1.3 → 1.4 breaking changes (MEDIUM impact, do NOT include in v2.4):**
- Readiness port changed to 19003 (dedicated listener). Affects any health-check configuration targeting the old port.
- Default access log format changed from text to JSON. Log-scraping integrations break.
- xDS snapshot behavior changed: translation errors now skip the update instead of replacing with empty.
- These changes require dedicated UAT of the Gateway API flow (HTTPRoute end-to-end). Not appropriate for a hardening milestone.

**Recommendation:** Hold Envoy Gateway at 1.3.1 for v2.4. Flag for v2.5 with dedicated HTTPRoute UAT.

### Headlamp 0.40.1 → 0.41.x

**Assessment:** 0.41.0 adds rollback for Deployments/DaemonSets/StatefulSets, multi-cluster user logout, cascading delete of Pods on Job deletion. No breaking changes. Manifest format is unchanged.

**Recommendation:** LOW risk bump. Include in v2.4 addon audit.

### local-path-provisioner 0.0.35 → 0.0.36

**Assessment:** 0.0.36 (released May 8, 2024) adds a high-severity HelperPod template injection vulnerability fix (validation rejecting privileged containers and hostPath volumes). Also updates Go to 1.26.2 and qualifies image references.

**Recommendation:** HIGH priority bump. Security fix. Update CVE threshold in doctor check from 0.0.34 to 0.0.35 (fix version for CVE-2025-62878). 0.0.36 is the new safe floor.

### Metrics Server

Check current stable at kubernetes-sigs/metrics-server. 0.8.x series is expected; bump is typically LOW risk (no CRD, no API surface changes for kinder users).

---

## etcd Peer-TLS: Implementation Approach Detail

### The Problem

When Docker stops containers (`docker pause` → `docker stop` → `docker start`), the container's IP on the cluster network may change. etcd peer certificates contain IP SANs (Subject Alternative Names) recording the original IP. When IPs change, mutual TLS authentication between etcd peers fails silently — the cluster appears to resume but API calls fail intermittently or hang.

### Comparable Tool Behavior

| Tool | Approach | User-visible? |
|------|----------|---------------|
| k3d | IPAM: reserves static IPs per node on the k3d Docker network (implemented v4.4.5, was Issue #550). Avoids TLS regen entirely. | Transparent |
| k3s | Does NOT handle IP changes automatically. Known failure mode: "K3s w/ etcd fails to restart after IP Change". Requires manual `--cluster-reset`. | Breaking — user must intervene |
| kubeadm | Manual: `kubeadm init phase certs etcd-peer` after deleting cert files. Skips regen if cert files exist. | Requires operator action |
| k0s | `etcd.peerAddress` config; does not auto-detect IP changes | Requires config update |

**k3d's IPAM approach is the right model for kinder.** It prevents the problem rather than requiring cert regen recovery. The implementation stores IP assignments in the kinder state file on cluster creation and passes `--ip <stored-ip>` to `docker network connect` on resume. The Docker named network survives container pause/stop/start — only IP assignment is lost when the container is removed.

### Recommended Approach: IP Pinning

1. On `kinder create cluster` (HA mode): after containers are created and IP is known, record `{containerName: ip}` in cluster state.
2. On `kinder resume`: before starting containers, reconnect each container to the cluster network with the stored static IP using `docker network connect --ip <ip> <network> <container>`.
3. On `kinder delete cluster`: state file is removed; no cleanup needed.

**Complexity: MEDIUM** (requires state file schema extension, Docker network reconnect logic, and ordering: network reconnect must happen before etcd start).

**Fallback approach: Conditional cert regen** (if IP pinning proves unreliable across providers):
1. After resume, read current container IP.
2. Read SAN IPs from existing peer cert (`openssl x509 -noout -text -in /etc/kubernetes/pki/etcd/peer.crt`).
3. If IP not in SAN list, delete cert files and run `kubeadm init phase certs etcd-peer`.
4. Restart etcd pod.

**Complexity: HIGH** (requires openssl parsing, kubeadm exec inside container, etcd restart sequencing, and must handle partial failure across 3+ nodes).

---

## macOS Ad-Hoc Signing: Implementation Detail

### What Gatekeeper Does

- **Unsigned binary (downloaded, not via Homebrew)**: Gatekeeper blocks with "Apple cannot check it for malicious software". User must `xattr -d com.apple.quarantine ./kinder` or right-click > Open.
- **Ad-hoc signed binary**: Passes Gatekeeper's quarantine check for locally trusted contexts. Does NOT establish developer identity or pass Apple's notarization scan. Sufficient for Homebrew Cask distribution.
- **Notarized binary**: Full Apple Developer ID + notarization ticket. Required for direct .dmg/.pkg distribution. NOT required for Homebrew formula/cask.

### Homebrew Behavior

Homebrew applies `codesign --sign -` automatically to some artifacts but NOT reliably for all Cask binaries on Apple Silicon. The ad-hoc signature must be embedded at release time to guarantee consistent behavior.

### GoReleaser Implementation

GoReleaser supports per-build `post_hooks` with template variables including `.Path` and `.Target`. The correct implementation:

```yaml
builds:
  - id: kinder-darwin
    goos: [darwin]
    goarch: [amd64, arm64]
    hooks:
      post:
        - cmd: codesign --sign - --force --preserve-metadata=entitlements,requirements,flags "{{ .Path }}"
          env:
            - CGO_ENABLED=0
```

This runs only for darwin targets (post_hooks are per-build artifact). The `--force` flag re-signs if already signed. The `--preserve-metadata` flags avoid breaking any existing metadata.

**No Apple Developer account required.** No Info.plist embedding needed for CLI binaries (Info.plist is for app bundles). The GoReleaser docs confirm codesign project support via `post_hooks`.

**Complexity: LOW.** Single config addition to `.goreleaser.yaml`, runs on macOS CI runner or can run cross-platform via `rcodesign` (Rust tool). However, `codesign` binary is macOS-only, so this hook must only run on darwin targets and requires a macOS CI runner or conditional execution.

---

## UAT Closure: Patterns and Deliverables

### What "UAT Closed" Means in Go CLI Tools

For maintenance milestones in similar-scope Go CLI tools (kind, k3d, kubectl-style tools):

| Evidence Type | Cost | Durability | Common Use |
|---------------|------|------------|------------|
| Shell script with `//go:build manual` tag | LOW | HIGH (rerunnable) | Most common in Go CLI tools |
| Recorded asciinema session | LOW | MEDIUM (visual, not executable) | Documentation / README demos |
| CI smoke job (integration test) | MEDIUM-HIGH | HIGH (automated, runs on PRs) | k3d, minikube use this |
| Written runbook (markdown checklist) | LOW | LOW (manual steps, drift risk) | Acceptable for one-off closure |

**Recommended for kinder v2.4:**
- Phase 47 (pause/resume): Shell script `hack/uat-pause-resume.sh` with `//go:build ignore` (not `manual` — `ignore` is more idiomatic for non-compilable scripts). Steps: create HA cluster, pause, verify containers stopped, resume, verify `kubectl get nodes` returns Ready. Run manually by developer; output log committed as evidence.
- Phase 51 (K8s 1.36 / Envoy LB): Shell script `hack/uat-envoy-lb.sh`. Steps: create cluster, deploy HTTPRoute, verify ingress works, verify IPVS rejection message, verify Envoy container (not HAProxy) in `docker ps`.

These scripts satisfy "live verification" without requiring a full CI integration test suite (which would need a Docker-in-Docker runner).

---

## Doctor Cosmetic Fixes: Exact Behavior Targets

### cluster-node-skew False Positive on LB Containers

**Current behavior:** The check compares Kubernetes node versions across all cluster members. The Envoy LB container (a plain haproxy/envoy container, not a kubelet node) has no Kubernetes version label. The check may warn about version skew because it cannot compare the LB container's "version" against control-plane nodes.

**Target behavior:** Skip containers whose `com.docker.compose.service` label or container name suffix matches `-lb` or whose role is `external-load-balancer`. Only compare containers with role `control-plane` and `worker`.

**User-visible message (before fix):** Warning about version skew or unrecognized node type.
**User-visible message (after fix):** No warning for LB containers. If genuine skew exists (CP vs worker), warn correctly.

### cluster-resume-readiness Reason Text Parsing

**Current behavior:** On partial etcd failure, the check passes the raw etcdctl JSON to the reason field. Example raw output:
```json
[{"endpoint":"https://127.0.0.1:2379","health":true,"took":"2ms"},
 {"endpoint":"https://127.0.0.2:2379","health":false,"took":"0s"},
 {"endpoint":"https://127.0.0.3:2379","health":false,"took":"0s"}]
```

**Target behavior:** Parse this JSON array, count `health:true` vs total, derive quorum state:
- All healthy: `"3/3 etcd members healthy"` (pass)
- Quorum intact: `"2/3 etcd members healthy, quorum intact"` (warn)
- Quorum at risk: `"1/3 etcd members healthy, quorum at risk — run kinder doctor --auto-fix"` (fail)
- All down: `"0/3 etcd members healthy — cluster unavailable"` (fail)

**etcdctl JSON fields confirmed:** `endpoint` (string), `health` (bool), `took` (string). These are stable across etcd 3.4–3.6.

---

## DEBT-04: Race Fix

**User-visible angle:** None. This is purely internal.

**What breaks:** `allChecks` is a package-level slice in `pkg/internal/doctor/` that gets appended to in multiple test functions running under `t.Parallel()`. The data race causes non-deterministic test failures under `go test -race`. Not observable by end users; only affects contributor CI and `-race` runs.

**Fix pattern:** Initialize `allChecks` once in `TestMain` (read-only after init) and pass it as a parameter to each sub-test, or use `t.Setenv`-style isolation per test. Standard Go concurrency fix; no architectural change.

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Addon bumps (cert-manager 1.17, Headlamp 0.41, local-path-provisioner 0.0.36, MetalLB latest) | MEDIUM | LOW | P1 |
| local-path-provisioner 0.0.36 (security fix) | HIGH | LOW | P1 |
| macOS ad-hoc signing | MEDIUM | LOW | P1 |
| cluster-node-skew LB false-positive fix | MEDIUM | LOW | P1 |
| cluster-resume-readiness reason text | MEDIUM | LOW | P1 |
| Phase 47 UAT closure (shell script + log) | HIGH (trust/quality) | LOW | P1 |
| Phase 51 UAT closure (shell script + log) | HIGH (trust/quality) | LOW | P1 |
| Windows PR-CI build step | LOW (user) / HIGH (dev) | LOW | P1 |
| DEBT-04 race fix | LOW (user) / MEDIUM (dev) | LOW | P1 |
| HA etcd peer-TLS / IP pinning | HIGH (HA user correctness) | MEDIUM | P2 |
| cert-manager 1.18 bump (vs 1.17) | LOW (incremental) | LOW | P2 |
| Envoy Gateway bump to 1.4+ | MEDIUM | MEDIUM | P3 (v2.5) |
| Full Windows test suite | LOW | HIGH | P3 (v2.5+) |
| Apple notarization | LOW | HIGH | P3 (v3.x) |
| `kinder upgrade` in-place addon update | MEDIUM | VERY HIGH | ANTI-FEATURE |

---

## Competitor Feature Analysis (Comparable Tools)

| Feature | kind | k3d | minikube | kinder v2.4 |
|---------|------|-----|----------|-------------|
| Addon version tracking | None (bare cluster) | Packaged addons but no auditing UI | Addons via addons enable; versions bundled | Explicit version audit + bump each release |
| macOS binary signing | Unsigned (quarantine on direct download) | Ad-hoc signed via GoReleaser post-hooks (confirmed) | Notarized (Apple Developer ID) | Ad-hoc sign in v2.4; notarization deferred |
| HA etcd IP stability on pause/resume | No pause command | IPAM-based static IPs (v4.4.5+) | Not applicable (VM-based, VirtualBox/Docker Driver) | IP pinning (v2.4 target) |
| Doctor/diagnostic checks | Known Issues page only | None | `minikube status` + basic checks | 23+ structured checks with auto-fix |
| etcd health human-readable | None | None | None | Parsed "N/M healthy" output (v2.4) |

---

## Sources

- cert-manager 1.17 upgrade guide: https://cert-manager.io/docs/releases/upgrading/upgrading-1.16-1.17/
- cert-manager 1.18 release notes: https://cert-manager.io/docs/releases/release-notes/release-notes-1.18/
- MetalLB release notes: https://metallb.universe.tf/release-notes/
- Envoy Gateway v1.4 announcement: https://gateway.envoyproxy.io/news/releases/v1.4/
- local-path-provisioner releases: https://github.com/rancher/local-path-provisioner/releases
- Headlamp 0.41.0 release: https://github.com/kubernetes-sigs/headlamp/releases/tag/v0.41.0
- GoReleaser build hooks docs: https://goreleaser.com/customization/builds/hooks/
- GoReleaser notarize docs: https://goreleaser.com/customization/sign/notarize/
- macOS CLI signing guide: https://tuist.dev/blog/2024/12/31/signing-macos-clis
- k3d IPAM feature request (implemented v4.4.5): https://github.com/k3d-io/k3d/issues/550
- k3s etcd IP change failures: https://forums.rancher.com/t/k3s-w-etcd-fails-to-restart-after-ip-change/42496
- etcd cluster status tutorial: https://etcd.io/docs/v3.5/tutorials/how-to-check-cluster-status/
- Kubernetes cert management with kubeadm: https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/
- Homebrew no-quarantine behavior: https://news.ycombinator.com/item?id=45907259

---

*Feature research for: kinder v2.4 Hardening milestone*
*Researched: 2026-05-09*
