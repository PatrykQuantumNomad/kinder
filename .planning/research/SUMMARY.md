# Project Research Summary

**Project:** kinder v2.4 Hardening
**Domain:** Brownfield maintenance milestone — batteries-included kind fork (Go CLI, Kubernetes addon manager)
**Researched:** 2026-05-09
**Confidence:** HIGH

---

## Executive Summary

kinder v2.4 is a locked-scope hardening milestone. v2.3 shipped 2026-05-07 and left three carry-forward items (SYNC-02 node image gated on Docker Hub, Phase 47 + 51 live UAT unclosed) plus known technical debt (DEBT-04 race). The v2.4 scope adds macOS ad-hoc signing, a Windows cross-compile CI gate, and an HA etcd peer-TLS fix for pause/resume. Research across all four domains confirms the scope is well-bounded, patterns are established in the codebase, and no new architectural primitives are required. Execution risk is concentrated in two areas: the etcd peer-TLS approach (where researchers diverged) and the Envoy Gateway version target (where researchers disagreed sharply).

The recommended approach is minimal-invasive throughout: fix the etcd TLS issue via IP pinning at resume time rather than cert regeneration (lower blast radius, matching the k3d precedent), hold Envoy Gateway at v1.3.1 with documented EOL acceptance (v1.4+ requires dedicated HTTPRoute UAT outside the hardening scope), bump cert-manager to v1.16.5 only (safe patch of the current EOL line; skip the major jump to v1.20.2 for this milestone), and bump Headlamp to v0.42.0 with one specific pre-commit verification. All other addon bumps are either holds (MetalLB v0.15.3, Metrics Server v0.8.1) or security-mandatory bumps (local-path-provisioner v0.0.36).

Key risks: (1) etcd peer-TLS — whichever approach is chosen must gate on all CP containers being stopped before any cert or network operation; partial-state transition causes split-brain. (2) Addon bumps must be executed one-per-plan, not batched — a single ambiguous test failure across 7 simultaneous bumps is undiagnosable. (3) macOS ad-hoc signing requires a macOS CI runner for the release job; the current ubuntu-latest runner cannot produce a signed darwin binary. (4) DEBT-04 must be fixed at test scope only — adding a mutex to the production RunAllChecks read path would serialize all doctor checks.

---

## Key Findings

### Recommended Stack

kinder v2.4 introduces no new language-level or infrastructure dependencies. The Go build toolchain, GoReleaser v2.x, and the existing go:embed addon manifest pipeline are unchanged. All changes are confined to: version string constants in addon packages, a new resume.go phase for etcd TLS, minor changes to two doctor check files, release pipeline YAML for signing, and a new GHA workflow file for Windows CI.

**Addon version decisions (synthesized):**

| Addon | From | To | Action | Rationale |
|-------|------|----|--------|-----------|
| cert-manager | v1.16.3 | v1.16.5 | Bump | Safe CVE/security patch; v1.20.2 jump carries rotationPolicy breaking default — defer to v2.5 |
| Envoy Gateway | v1.3.1 | v1.3.1 | HOLD | 4 breaking changes in v1.4; requires dedicated HTTPRoute UAT outside hardening scope |
| Headlamp | v0.40.1 | v0.42.0 | Bump | Verify token flow before commit (2 days old at research time) |
| MetalLB | v0.15.3 | v0.15.3 | Hold | Latest; no v0.16 exists |
| Metrics Server | v0.8.1 | v0.8.1 | Hold | Latest stable |
| local-path-provisioner | v0.0.35 | v0.0.36 | Bump | HIGH priority — GHSA-7fxv-8wr2-mfc4 security fix |
| registry:2 | floating | floating | Hold | registry:3 explicitly out of scope |

**GoReleaser signing:** Split build approach — separate kinder-darwin build entry with post_hooks running codesign --force --sign -. Requires macos-latest CI runner for release job. GoReleaser v2.x (current ~v2.15); no version bump needed.

**Windows CI:** ubuntu-24.04 runner with GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./... plus go vet. New file .github/workflows/build-check.yml. Informational (non-blocking) for v2.4.

### Expected Features

**Must have (table stakes):**
- Addon version audit + bump for all 7 addons — users expect a maintained tool to track upstream; local-path-provisioner is a security fix
- macOS ad-hoc signing — resolves AMFI: has no CMS blob? kernel kill on Apple Silicon for cross-compiled release binaries
- Windows PR-CI build gate — prevents silent Windows compilation regressions on every PR
- etcd peer-TLS fix for HA pause/resume — v2.3 delivered pause/resume; silent etcd quorum failure on resume is a regression
- Phase 47 + 51 live UAT closure — v2.3 carry-forward; must be formally closed before v2.4 ships
- cluster-node-skew LB false-positive fix — Envoy LB container has no /kind/version; spurious doctor warn on every HA cluster
- cluster-resume-readiness structured output — "1/3 etcd members healthy, quorum at risk" instead of raw JSON blob

**Should have (competitive):**
- DEBT-04 race fix — go test -race ./... clean; quality signal visible to contributors
- cert-manager rotationPolicy disclosure in CHANGELOG — proactive breaking-change communication for v1.17+ default
- Conditional etcd-TLS (only when IP changes) — zero overhead when IPs are stable

**Defer to v2.5+:**
- Envoy Gateway bump to v1.4+ (breaking changes require dedicated HTTPRoute UAT)
- cert-manager bump to v1.20.2 (rotationPolicy: Always breaking default requires validation)
- Windows runtime test suite
- Apple notarization (paid Developer ID cert, out of scope)

**Anti-features (must NOT enter v2.4):**
- macOS notarization (Apple Developer cert, paid) — explicit out of scope
- Helm-based addon manager — violates go:embed static manifest approach
- kinder upgrade in-place addon bump on existing clusters — create-destroy model is kinder's architecture
- Unconditional etcd cert regen on every resume — 5-15s overhead, partial-failure risk
- Strict-Windows-build CI failure block before Windows is a supported runtime — informational-only for v2.4
- registry:3 — deprecated storage drivers; kind ecosystem is on v2

### Architecture Approach

All v2.4 changes slot into existing extension points with no new architectural primitives. The addon pipeline (per-package var Images + go:embed manifest + offlinereadiness.go inline duplicate) requires a mandatory dual-update on every image string change — this is the highest-frequency mechanical error source. The resume lifecycle already has a three-phase start order (LB → CP → readiness hook → workers) with a Phase 3 window that is the exact insertion point for etcd TLS work. The doctor check registry (allChecks global, 24 checks, pinned count test) will require count updates only if a new check is added (not the case in v2.4).

**Key integration points per feature:**

1. Addon bumps: pkg/cluster/internal/create/actions/install<Addon>/<addon>.go (var Images) + manifests/<file>.yaml + pkg/internal/doctor/offlinereadiness.go (allAddonImages). Three files per addon; offlinereadiness_test.go pins count at 14 — will fail on count mismatch.
2. etcd peer-TLS: pkg/internal/lifecycle/resume.go only — new function in Phase 3 window before readiness hook.
3. DEBT-04: pkg/internal/doctor/check.go + check_test.go + socket_test.go — fix at test scope, not production read path.
4. macOS signing: .goreleaser.yaml only — no Go source changes.
5. Windows CI: .github/workflows/build-check.yml only — new file.
6. Doctor cosmetics: pkg/internal/doctor/clusterskew.go (LB role guard, ~5 lines) + pkg/internal/doctor/resumereadiness.go (JSON parsing refactor).

**Parallel-wave conflicts:** DEBT-04 and cosmetic doctor fixes both touch pkg/internal/doctor/; must be sequential (DEBT-04 first). Addon bumps and offlinereadiness.go update must be sequential (bumps first, then single follow-up commit).

### Critical Pitfalls

1. **Etcd regen while any CP is running causes quorum loss** — gate ALL cert/network operations on all-stopped state. Never regen live. Recovery cost: HIGH.

2. **Partial SAN coverage = split-brain** — all CP peer certs must be regenerated (or IPs pinned) in a single atomic pass before any CP node starts. One-node-at-a-time regen is worse than no regen.

3. **Batching all 7 addon bumps in one plan makes failures undiagnosable** — one plan per addon (53-01 through 53-07), one make test gate between each.

4. **cert-manager CRD annotation limit silently fails without --server-side** — both v1.16.5 and v1.20.2 manifests exceed 256 KB. Flag is a locked decision; verify it is present and add integration test asserting CRD version post-install.

5. **GoReleaser signing fan-out misses one architecture** — darwin/amd64 and darwin/arm64 must each be verified with codesign -vvv in CI. Sign must be the LAST operation before packaging.

6. **DEBT-04 over-broad mutex serializes doctor run** — allChecks global is written only in tests, never in production after init. Fix belongs entirely in test scope. Adding sync.RWMutex to production RunAllChecks read path serializes all doctor checks.

7. **kubeadm certs subcommand is not version-stable** — if cert regen approach is chosen over IP pinning, do not rely on kubeadm; implement via Go crypto/x509 or openssl exec.

---

## Divergences Synthesized

### Divergence 1: Envoy Gateway Version Target

STACK.md recommended bumping to v1.7.2 (v1.3.1 two major versions EOL; v1.6 EOLs 2026-05-13). FEATURES.md recommended holding at v1.3.1 (v1.4 has 4 breaking changes: readiness port 19003, JSON access log default, xDS snapshot behavior, extension manager error handling; hardening milestone is not the place to absorb them). PITFALLS.md confirmed EG CRD version is locked to Gateway API CRDs version — cannot bump EG without bumping companion CRDs simultaneously.

**Recommendation: HOLD at v1.3.1 with documented EOL acceptance.**

The hardening milestone's primary mandate is "do not regress." Absorbing 4 breaking changes from v1.3 to v1.4 without dedicated HTTPRoute end-to-end UAT violates that mandate. kinder's use case (embedded GatewayClass + single HTTPRoute for demo) is not externally exposed — EOL security posture is acceptable for one maintenance milestone.

CHANGELOG for v2.4 must state: "Envoy Gateway remains at v1.3.1 (EOL); v1.4+ bump planned for v2.5 with dedicated HTTPRoute UAT."

Options surfaced for REQUIREMENTS decision:
- (a) Hold at v1.3.1 + document EOL acceptance — RECOMMENDED
- (b) Bump to v1.4 only — requires Gateway API CRD version audit + readiness probe port update + access log format change; 3 coordinated changes on a hardening milestone; not recommended
- (c) Bump to v1.7.2 with HTTPRoute UAT plan — only viable if v2.4 scope is expanded; scope is locked; reject

### Divergence 2: Etcd Peer-TLS Approach

ARCHITECTURE.md recommended kubeadm certs renew etcd-peer via docker exec, inserted in resume.go Phase 3 between CP start and readiness hook. FEATURES.md recommended IP pinning via docker network connect --ip (k3d model — prevents problem rather than recovering; 5-15s overhead avoided). PITFALLS.md flagged both: kubeadm not version-stable (Pitfall 4); partial SAN coverage = split-brain (Pitfall 2); CA key extraction risk (Pitfall 3); IP pinning Docker IPAM feasibility is MEDIUM confidence.

**Recommendation: IP pinning is preferred; cert regen is the fallback.**

kinder's locked decision history favors preventing problems over recovering from them. IP pinning matches the k3d precedent and produces zero overhead on the common case.

Implementation: (1) kinder create cluster HA mode: record {containerName: assignedIP} in cluster state after container creation. (2) kinder resume: before starting CP containers, call docker network connect --ip <stored-ip> <network> <container> for each CP. (3) No cert operations required.

**Critical gap:** Can docker network connect --ip reliably restore a stored IP after container stop (not remove)? Feasibility must be verified empirically in Phase 52 plan Task 1 before any code is written. If Docker IPAM has reassigned the IP, this approach fails.

Fallback cert regen constraints (if IP pinning proves infeasible):
- Gate entirely on all-stopped state (Pitfall 1)
- Generate all peer certs in single atomic pass (Pitfall 2)
- Extract CA key via docker cp from stopped container only (Pitfall 3)
- Do NOT use kubeadm; use Go crypto/x509 or openssl exec (Pitfall 4)

### Divergence 3: cert-manager Version Target

STACK.md flagged both v1.16.5 (safe minimal patch, v1.16 EOL 2025-06-10) and v1.20.2 (current stable). FEATURES.md flagged that v1.18+ introduces breaking default rotationPolicy: Always and recommended v1.17 as a lower-risk intermediate step.

**Recommendation: Bump to v1.16.5 only for v2.4.**

v1.16.5 is a pure CVE/security-dependency patch with no API changes and no behavioral changes for kinder's embedded ClusterIssuer use case. v1.17+ introduces user-visible behavioral changes (rotationPolicy: Always, UID/GID change to 65532/65532) that require CHANGELOG disclosure. That disclosure work is appropriate for v2.5, not a hardening milestone.

CHANGELOG requirement: Note v1.16 EOL 2025-06-10; v2.5 will target v1.20.x with full breaking-change disclosure.

### Divergence 4: Headlamp Version + Token Flow Confidence

STACK.md recommended v0.42.0 (released 2026-05-07, no breaking changes per release notes). FEATURES.md had lower confidence on token flow stability — v0.41 changed ServiceAccount token bootstrap flow; v0.42 may have continued that change.

**Recommendation: Bump to v0.42.0 with one mandatory pre-commit verification.**

Before writing the bump plan, verify: does kubectl create token against the v0.42.0 Headlamp ServiceAccount produce a valid token? Run kubectl auth can-i --token=<printed> get pods -n kube-system. If the token is empty or auth fails, update the token-print step or remove it and update the success message. If the check reveals a regression, hold at v0.40.1 and document.

---

## Implications for Roadmap

### Phase 52: HA Etcd Peer-TLS Fix
**Rationale:** Highest blast radius of any v2.4 item. PITFALLS.md explicitly flags as "first phase or standalone phase with zero other concurrent changes." Closing it first means all subsequent testing is done against a working HA resume path.
**Delivers:** Reliable kinder resume for HA clusters; no false etcd quorum failures after Docker IP reassignment.
**Implements:** IP pinning via Docker network reconnect in pkg/internal/lifecycle/resume.go (preferred), or conditional cert regen fallback.
**Avoids:** ETCD-TLS Pitfalls 1-4 (quorum loss, split-brain, CA key extraction, kubeadm version-stability).
**Research flag:** NEEDS validation — Docker IPAM static IP feasibility must be verified as first task of the plan before any code is written.

### Phase 53: Addon Version Audit + Bumps (+ SYNC-02 conditional)
**Rationale:** One plan per addon prevents ambiguous test failures. SYNC-02 belongs here as sub-plan 53-00 because it shares the offlinereadiness.go dual-update pattern with addon bumps.
**Delivers:** All 7 addons at verified current versions; local-path-provisioner security fix closed; SYNC-02 promoted if Docker Hub probe passes.
**Sub-plan order (lowest risk to highest):**
- 53-00: SYNC-02 probe + conditional node image bump (two-step probe: existence + manifest digest)
- 53-01: local-path-provisioner v0.0.35 → v0.0.36 (security fix; update CVE threshold in doctor)
- 53-02: MetalLB HOLD verification (confirm v0.15.3 still latest; no file changes)
- 53-03: Metrics Server HOLD verification (confirm v0.8.1 still latest)
- 53-04: Headlamp v0.40.1 → v0.42.0 (token flow pre-check mandatory)
- 53-05: cert-manager v1.16.3 → v1.16.5 (verify --server-side flag; CRD version assertion)
- 53-06: Envoy Gateway HOLD at v1.3.1 (update EOL documentation; no image changes)
- 53-07: offlinereadiness.go consolidation (single commit updating allAddonImages after all bumps)
**Avoids:** Pitfalls 5-14 (batch bumps, annotation limit, webhook race, EG CRD lock, MetalLB legacy format, Headlamp token, Metrics Server flag, local-path air-gap, SYNC-02 incomplete push, stale version strings).
**Research flag:** No additional research needed for holds. Headlamp token flow and SYNC-02 probe are plan-time verifications.

### Phase 54: macOS Ad-Hoc Code Signing
**Rationale:** Release pipeline change only — no Go source changes. Independent of all other phases.
**Delivers:** darwin/amd64 and darwin/arm64 binaries signed with codesign --sign - in GoReleaser release job. Resolves Apple Silicon AMFI kernel kill for cross-compiled binaries.
**Implements:** Split build entries in .goreleaser.yaml. macos-latest release runner.
**Avoids:** Pitfalls 15 (symbol strip after sign invalidates signature), 16 (fan-out misses one arch), 17 (ad-hoc conflated with notarization in docs — communication requirement).
**Research flag:** Standard GoReleaser pattern; no additional research needed.

### Phase 55: Windows PR-CI Build Step
**Rationale:** Infrastructure change only — new GHA workflow file. Independent of all other phases. Informational (non-blocking) for v2.4.
**Delivers:** Every PR cross-compiled for windows/amd64 on ubuntu-24.04. Prevents silent Windows compilation regressions.
**Implements:** .github/workflows/build-check.yml — new file with windows-build job (go build + go vet under GOOS=windows).
**Avoids:** Pitfalls 18 (cgo transitive deps), 19 (path separator failures masked by build-only check).
**Research flag:** Standard GHA pattern. Run cross-compile probe locally before writing CI YAML.

### Phase 56: DEBT-04 Race Fix
**Rationale:** Low-complexity, high-confidence fix. Placed after addon bumps so stable test infrastructure is in place before doctor cosmetic fixes (Phase 57) land — both touch pkg/internal/doctor/.
**Delivers:** go test -race ./pkg/internal/doctor/... -count=100 passes with zero races. Clean contributor CI.
**Implements:** Test-scope save/restore pattern for allChecks in check_test.go and socket_test.go. Production check.go unchanged or minimally changed (unexported runChecks(checks []Check) if parameter-injection approach chosen).
**Avoids:** Pitfall 20 (mutex held during check execution serializes doctor run — do NOT add mutex to production read path).
**Research flag:** Standard Go concurrency fix; no additional research needed.

### Phase 57: Doctor Cosmetic Fixes
**Rationale:** Depends on DEBT-04 (Phase 56) being complete because both fixes touch pkg/internal/doctor/ files.
**Delivers:** No false-positive version-skew warn on HA clusters. Actionable "N/M etcd members healthy" output replacing raw JSON.
**Sub-plans:**
- 57-01: cluster-node-skew LB role guard (clusterskew.go ~5 lines + test case)
- 57-02: cluster-resume-readiness JSON parsing (resumereadiness.go refactor + test fixtures for etcd 3.4.x and 3.5.x JSON shapes)
**Avoids:** Pitfalls 21 (LB node in skew calculation), 22 (etcdctl JSON shape variance).
**Research flag:** No additional research needed. etcdctl JSON fields confirmed stable across etcd 3.4-3.6.

### Phase 58: Phase 47 + 51 Live UAT Closure
**Rationale:** Must run against the final v2.4 binary — not before Phases 52-57 are complete. PITFALLS.md Pitfall 23 is definitive: run make build and confirm ./bin/kinder version shows the v2.4 build hash before any UAT script. Never run smoke against PATH kinder.
**Delivers:**
- scripts/uat-47-ha-smoke.sh — shell script: create 3-CP HA cluster, pause, verify containers stopped, resume, verify kubectl get nodes returns Ready on current binary
- .planning/phases/47-cluster-pause-resume/47-UAT.md — status fields updated from issue to pass
- .planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md — new file with Envoy LB image probe, IPVS CLI rejection, K8s 1.36 guide spot-check evidence
**Avoids:** Pitfall 23 (stale binary, recording drift).
**Research flag:** No additional research needed. Existing 47-UAT.md format is the established evidence template.

### Phase Ordering Rationale

The ordering above synthesizes three competing suggestions from research:
- PITFALLS.md said etcd-TLS first (Phase 52) — adopted; matches blast-radius logic
- ARCHITECTURE.md said DEBT-04 → bumps → cosmetics → etcd-TLS → SYNC-02 → UAT → signing+CI — partially adopted; swapped etcd-TLS to first; moved DEBT-04 after bumps to respect the pkg/internal/doctor/ sequential conflict
- FEATURES.md said UAT closure first (close v2.3 carry-forward before opening new fronts) — rejected for UAT specifically (UAT must verify the hardened binary); accepted as principle by treating SYNC-02 as Phase 53's first sub-plan

Dependency chain:
```
Phase 52 (etcd-TLS)
  → Phase 53 (addon bumps + SYNC-02)
    → Phase 56 (DEBT-04)
      → Phase 57 (doctor cosmetics)
        → Phase 58 (UAT closure, on final binary)

Phase 54 (macOS signing)  — independent after Phase 52
Phase 55 (Windows CI)     — independent after Phase 52
```

### Velocity Estimate

Reference milestones: v2.1 (4 phases / 10 plans / 1 day); v2.3 (5 phases / 25 plans / 5 days).

v2.4 estimate: 7 phases / ~20 plans / 3-4 days. Phase 53 alone accounts for 7-8 sub-plans. Scope is closer to v2.3 than v2.1 due to etcd TLS complexity and addon count, but fewer unknown-unknowns than v2.3 which introduced pause/resume from scratch. The etcd IP pinning feasibility verification could extend Phase 52 by 0.5-1 day if Docker IPAM requires the fallback cert-regen approach.

### Research Flags

**Needs validation before plan execution:**
- Phase 52: Docker IPAM static IP feasibility — run verification before writing any code
- Phase 53-04 (Headlamp): Token flow verification — verify before writing bump plan
- Phase 53-00 (SYNC-02): Docker Hub two-step probe — run before any file changes

**Standard patterns (skip additional research):**
- Phase 54 (macOS signing): GoReleaser post_hooks pattern established; STACK.md has exact YAML
- Phase 55 (Windows CI): GHA cross-compile pattern is standard; STACK.md has exact YAML
- Phase 56 (DEBT-04): Go test-scope concurrency fix is standard; pattern verified
- Phase 57 (doctor cosmetics): Code locations verified by ARCHITECTURE.md; exact fix described
- Phase 58 (UAT): Existing 47-UAT.md template; no new patterns needed

---

## Decisions Needed

These go/no-go decisions must be recorded in REQUIREMENTS before any plan execution begins:

1. **Envoy Gateway: Hold at v1.3.1 or bump?** Recommended: Hold with documented EOL acceptance. If stakeholder requires a bump, commit to v1.4 only (not v1.7) and extend scope to include a dedicated HTTPRoute UAT sub-plan.

2. **Etcd peer-TLS: IP pinning or cert regen?** Recommended: IP pinning (Docker network connect). Requires feasibility verification in Phase 52 plan Task 1. If infeasible, fall back to cert regen via Go crypto/x509 (not kubeadm). Decision must be locked before Phase 52 plan is written.

3. **cert-manager: v1.16.5 only or also v1.20.2?** Recommended: v1.16.5 only for v2.4. Document v1.20.2 target for v2.5. If schedule permits after all other phases close, v1.20.2 can be added as Phase 53-05b but must include CHANGELOG disclosure for rotationPolicy: Always.

4. **Headlamp v0.42.0: include or hold?** Recommended: Include with mandatory token-flow pre-check. If pre-check reveals token flow regression, hold at v0.40.1 and document.

5. **macOS signing CI runner: macos-latest or document-and-defer?** Recommended: macos-latest for release job (~$0.80/release on GHA — acceptable). Option C (document xattr -d com.apple.quarantine in release notes, zero CI cost) is the fallback.

6. **Windows CI: informational or blocking?** Recommended: Informational (non-blocking, continue-on-error: true) for v2.4. Upgrade to blocking in v2.5 once path-separator audit is complete.

7. **SYNC-02: Include in v2.4 or remain externally gated?** As of 2026-05-09, Docker Hub shows kindest/node latest is v1.35.1 (not v1.36.x). Include Phase 53-00 as a conditional plan that halts INCONCLUSIVE if the probe fails. Do not force a deadline on Docker Hub.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All addon versions fetched from upstream GitHub releases; GoReleaser docs fetched from goreleaser.com; kubeadm docs from kubernetes.io. Single gap: Headlamp v0.42.0 token flow (2 days old at research time) |
| Features | HIGH | Official cert-manager, MetalLB, Envoy Gateway docs; k3d IPAM precedent verified; etcd JSON fields confirmed stable 3.4-3.6 |
| Architecture | HIGH | All integration points verified against actual source files. Integration map is authoritative for file locations and conflict detection |
| Pitfalls | HIGH | Grounded in v2.3 audit record and direct codebase review. CA key extraction risk and kubeadm version-stability caveat are critical for Phase 52 |

**Overall confidence: HIGH**

### Gaps to Address

- **Docker IPAM static IP feasibility:** PITFALLS.md rates IP pinning as MEDIUM confidence. Must be verified empirically in Phase 52 plan Task 1. Failure mode is well-defined (fall back to cert regen); gap does not block planning.

- **Headlamp v0.42.0 token flow:** 2 days old at research time. Resolve with a 5-minute manual verification before writing Phase 53-04 plan.

- **Envoy Gateway CRD version for v1.4+:** Not fully researched because FEATURES.md recommends holding. If the REQUIREMENTS decision changes to bump, a targeted research task is needed to identify the exact Gateway API CRD version required by EG v1.4 and any HTTPRoute behavioral changes.

---

## Sources

### Primary (HIGH confidence)
- cert-manager GitHub releases (github.com/cert-manager/cert-manager/releases) — v1.16.5 date, v1.20.2 date, EOL status
- cert-manager supported releases page (cert-manager.io/docs/releases/) — v1.16 EOL 2025-06-10, v1.20 current stable
- Envoy Gateway releases (github.com/envoyproxy/gateway/releases) — v1.7.2 date, v1.6.7 date
- Envoy Gateway compatibility matrix (gateway.envoyproxy.io/news/releases/matrix/) — v1.7 K8s 1.32-1.35 requirement
- Envoy Gateway v1.4 announcement (gateway.envoyproxy.io/news/releases/v1.4/) — 4 breaking changes documented
- Headlamp releases (github.com/headlamp-k8s/headlamp/releases) — v0.42.0 date 2026-05-07
- MetalLB release notes (metallb.universe.tf/release-notes/) — v0.15.3 latest; no v0.16
- metrics-server releases (github.com/kubernetes-sigs/metrics-server/releases) — v0.8.1 latest
- local-path-provisioner releases (github.com/rancher/local-path-provisioner/releases) — v0.0.36 GHSA-7fxv-8wr2-mfc4
- GoReleaser build hooks docs (goreleaser.com/customization/builds/hooks/) — hook schema, template vars
- golang/go#42684 (github.com/golang/go/issues/42684) — codesign -s - fixes arm64 macOS AMFI kill
- kubeadm certs renew docs (kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm-certs/) — subcommand confirmed
- Go race detector (go.dev/doc/articles/race_detector) — t.Parallel + global mutation pattern
- k3d IPAM feature request (github.com/k3d-io/k3d/issues/550) — implemented v4.4.5; IP pinning precedent
- etcd cluster status tutorial (etcd.io/docs/v3.5/tutorials/how-to-check-cluster-status/) — JSON field names
- v2.3 Milestone Audit (.planning/milestones/v2.3-MILESTONE-AUDIT.md) — deferred items, tech debt
- Codebase Architecture (.planning/codebase/ARCHITECTURE.md) — verified source of integration point truth
- Direct source code inspection (all pkg/internal/doctor/, pkg/internal/lifecycle/, pkg/cluster/internal/create/actions/ files)

### Secondary (MEDIUM confidence)
- docker network connect docs (docs.docker.com/reference/cli/docker/network/connect/) — --ip flag behavior; feasibility for kinder's IPAM setup requires empirical testing
- Headlamp v0.42.0 token flow — 2 days old at research; verify before plan execution

### Tertiary (LOW confidence — needs validation)
- kindest/node v1.36.x Docker Hub availability — not published as of 2026-05-09 (probe shows v1.35.1); SYNC-02 remains externally gated

---

*Research completed: 2026-05-09*
*Ready for roadmap: yes — pending 7 locked decisions listed in "Decisions Needed" section*
