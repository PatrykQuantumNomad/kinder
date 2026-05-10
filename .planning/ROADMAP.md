# Roadmap: Kinder

## Milestones

- SHIPPED **v1.0 Batteries Included** - Phases 1-8 (shipped 2026-03-01)
- SHIPPED **v1.1 Kinder Website** - Phases 9-14 (shipped 2026-03-02)
- SHIPPED **v1.2 Branding & Polish** - Phases 15-18 (shipped 2026-03-02)
- SHIPPED **v1.3 Harden & Extend** - Phases 19-24 (shipped 2026-03-03)
- SHIPPED **v1.4 Code Quality & Features** - Phases 25-29 (shipped 2026-03-04)
- SHIPPED **v1.5 Website Use Cases & Documentation** - Phases 30-34 (shipped 2026-03-04)
- SHIPPED **v2.0 Distribution & GPU Support** - Phases 35-37 (shipped 2026-03-05)
- SHIPPED **v2.1 Known Issues & Proactive Diagnostics** - Phases 38-41 (shipped 2026-03-06)
- SHIPPED **v2.2 Cluster Capabilities** - Phases 42-46 (shipped 2026-04-10)
- SHIPPED **v2.3 Inner Loop** - Phases 47-51 (shipped 2026-05-07)
- IN PROGRESS **v2.4 Hardening** - Phases 52-58 (started 2026-05-09)

## Phases

<details>
<summary>SHIPPED v1.0 Batteries Included (Phases 1-8) - SHIPPED 2026-03-01</summary>

See `.planning/milestones/v1.0-ROADMAP.md` for full phase details.

Phases 1-8: Foundation, MetalLB, Metrics Server, CoreDNS Tuning, Envoy Gateway, Dashboard, Integration Testing, Gap Closure.

</details>

<details>
<summary>SHIPPED v1.1 Kinder Website (Phases 9-14) - SHIPPED 2026-03-02</summary>

See `.planning/milestones/v1.1-ROADMAP.md` for full phase details.

Phases 9-14: Scaffold & Deploy Pipeline, Dark Theme, Documentation Content, Landing Page, Assets & Identity, Polish & Validation.

</details>

<details>
<summary>SHIPPED v1.2 Branding & Polish (Phases 15-18) - SHIPPED 2026-03-02</summary>

Phases 15-18: Logo, SEO, Docs Rewrite, Dark Theme Enforcement.

</details>

<details>
<summary>SHIPPED v1.3 Harden & Extend (Phases 19-24) - SHIPPED 2026-03-03</summary>

Phases 19-24: Bug Fixes, Provider Code Deduplication, Config Type Additions, Local Registry Addon, Cert-Manager Addon, CLI Diagnostic Tools.

</details>

<details>
<summary>SHIPPED v1.4 Code Quality & Features (Phases 25-29) - SHIPPED 2026-03-04</summary>

See `.planning/milestones/v1.4-ROADMAP.md` for full phase details.

Phases 25-29: Foundation (Go 1.24, golangci-lint v2, layer fix), Architecture (context.Context, addon registry), Unit Tests (FakeNode/FakeCmd test infra), Parallel Execution (wave-based errgroup), CLI Features (JSON output, profile presets).

</details>

<details>
<summary>SHIPPED v1.5 Website Use Cases & Documentation (Phases 30-34) - SHIPPED 2026-03-04</summary>

See `.planning/milestones/v1.5-ROADMAP.md` for full phase details.

Phases 30-34: Foundation Fixes, Addon Page Depth, CLI Reference, Tutorials, Verification & Polish.

</details>

<details>
<summary>SHIPPED v2.0 Distribution & GPU Support (Phases 35-37) - SHIPPED 2026-03-05</summary>

Phases 35-37: GoReleaser Foundation, Homebrew Tap, NVIDIA GPU Addon.

</details>

<details>
<summary>SHIPPED v2.1 Known Issues & Proactive Diagnostics (Phases 38-41) - SHIPPED 2026-03-06</summary>

See `.planning/milestones/v2.1-ROADMAP.md` for full phase details.

Phases 38-41: Check Infrastructure, Docker & Tool Checks, Kernel & Platform Checks, Network/Create-Flow/Website.

</details>

<details>
<summary>SHIPPED v2.2 Cluster Capabilities (Phases 42-46) - SHIPPED 2026-04-10</summary>

See `.planning/milestones/v2.2-ROADMAP.md` for full phase details.

Phases 42-46: Multi-Version Node Validation, Air-Gapped Cluster Creation, Local-Path-Provisioner Addon, Host-Directory Mounting, `kinder load images` Command. Doctor registry expanded from 18 to 23 checks. Zero new Go module dependencies.

</details>

<details>
<summary>SHIPPED v2.3 Inner Loop (Phases 47-51) - SHIPPED 2026-05-07</summary>

See `.planning/milestones/v2.3-ROADMAP.md` for full phase details.

Phases 47-51: Cluster Pause/Resume, Cluster Snapshot/Restore, Inner-Loop Hot Reload (`kinder dev`), Runtime Error Decoder (`kinder doctor decode` with 16-pattern catalog), Upstream Sync (HAProxy→Envoy LB across docker/podman/nerdctl + IPVS-on-1.36+ guard + K8s 1.36 website recipe). SYNC-02 (default node image bump to K8s 1.36.x) DEFERRED — `kindest/node:v1.36.x` not yet on Docker Hub.

</details>

### v2.4 Hardening (In Progress)

**Milestone Goal:** Close v2.3 tech debt, bring all addons to current stable, fix the HA pause/resume etcd-TLS architectural gap, and unblock distribution UX via macOS ad-hoc signing and a Windows PR-CI build step.

- [ ] **Phase 52: HA Etcd Peer-TLS Fix** - IP-pin HA control-plane containers so Docker IPAM reassignment cannot break peer connectivity on resume
- [ ] **Phase 53: Addon Version Audit, Bumps & SYNC-05** - Audit all 7 addons, execute security and version bumps, conditionally re-run SYNC-05 node image bump
- [ ] **Phase 54: macOS Ad-Hoc Code Signing** - Sign darwin/amd64 and darwin/arm64 GoReleaser artifacts via `codesign --force --sign -` on a macOS runner
- [ ] **Phase 55: Windows PR-CI Build Step** - Add blocking `GOOS=windows go build ./...` cross-compile step to PR CI
- [ ] **Phase 56: DEBT-04 Doctor Test Race Fix** - Eliminate `allChecks` global mutation under `t.Parallel()` via scoped `runChecks(checks []Check)` helper
- [ ] **Phase 57: Doctor Cosmetic Fixes** - Fix cluster-node-skew LB false-positive and cluster-resume-readiness JSON reason text
- [ ] **Phase 58: Live UAT Closure for Phase 47 + 51** - Run and record live smoke tests against rebuilt v2.4 binary for both deferred UAT items

## Phase Details

### Phase 52: HA Etcd Peer-TLS Fix
**Goal**: HA clusters resume cleanly after `kinder pause` + `kinder resume` regardless of whether Docker IPAM reassigns container IPs
**Depends on**: Nothing (first v2.4 phase; highest blast radius — work in isolation)
**Requirements**: LIFE-09
**Success Criteria** (what must be TRUE):
  1. `kinder resume` on a 3-CP HA cluster returns all control-plane nodes to Ready state even when Docker assigns different IPs than those recorded in etcd peer certs
  2. A fresh `kinder create cluster --config ha.yaml` followed by `kinder pause` + `kinder resume` passes `etcdctl endpoint health --cluster` with all members healthy
  3. Single-CP clusters incur zero overhead — no cert or network operations fire for non-HA topologies
  4. If Docker IPAM feasibility probe (Plan 52-01 Task 1) succeeds, the fix uses IP pinning via `docker network connect --ip`; if infeasible, the fallback cert-regen approach is documented and implemented instead
**Plans**: 4 plans

Plans:
- [x] 52-01-PLAN.md — IPAM feasibility probe + doctor `ipam-probe` check (Roadmap pre-flight gate; Task 1 IS the probe)
- [x] 52-02-PLAN.md — IP-pin module + create-time hook in docker provider (records IP, writes /kind/ipam-state.json, sets resume-strategy label)
- [ ] 52-03-PLAN.md — Cert-regen fallback module + Resume() dispatch (pre-CP-start IP reapply for ip-pinned; post-start reactive wholesale regen for cert-regen/legacy)
- [ ] 52-04-PLAN.md — `ha-resume-strategy` doctor check + count test bump to 26

**RISK NOTE**: This phase has the highest blast radius of any v2.4 item. Discuss with `/gsd:discuss-phase 52` before planning. Task 1 of the first plan MUST be the Docker IPAM feasibility probe — no code is written until the probe result is known. See PITFALLS research items 1-4.

### Phase 53: Addon Version Audit, Bumps & SYNC-05
**Goal**: All 7 addons are at verified current-stable versions (or documented holds), the security fix for local-path-provisioner is closed, and the SYNC-05 node image bump executes if Docker Hub has published `kindest/node:v1.36.x`
**Depends on**: Phase 52
**Requirements**: ADDON-01, ADDON-02, ADDON-03, ADDON-04, ADDON-05, SYNC-05
**Success Criteria** (what must be TRUE):
  1. `kinder create cluster` installs local-path-provisioner v0.0.36 (GHSA-7fxv-8wr2-mfc4 security fix closed); `kinder doctor offline-readiness` does not warn on a fresh default cluster
  2. `kinder create cluster` installs Headlamp v0.42.0; the printed ServiceAccount token authenticates successfully against the Headlamp UI (or a documented hold at v0.40.1 is in place with explanation)
  3. `kinder create cluster` installs cert-manager v1.20.2 with `--server-side` apply; `kubectl get crd certificates.cert-manager.io -o jsonpath='{.spec.versions[0].name}'` returns the v1.20.2 API version; self-signed ClusterIssuer issues a certificate with the new UID (65532)
  4. `kinder create cluster` installs Envoy Gateway v1.7.2; an HTTPRoute routes traffic end-to-end; the `eg-gateway-helm-certgen` job name is verified in the v1.7.2 install.yaml before commit
  5. `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` and `TestAllAddonImages_CountMatchesExpected` reflect all bumped image references; `go test ./pkg/internal/doctor/... -run TestAllAddonImages` passes
  6. If Docker Hub two-step probe (existence + manifest digest) confirms `kindest/node:v1.36.x` is published, the default image constant in `pkg/apis/config/defaults/image.go` is updated and `kinder create cluster` with no `--image` flag uses K8s 1.36; otherwise SYNC-05 halts INCONCLUSIVE with re-runnable status

**Sub-plan order (sequential — NOT parallel, see cross-phase concerns):**
- 53-00: SYNC-05 probe — Docker Hub two-step probe (existence + manifest digest); conditional gate before any source change
- 53-01: local-path-provisioner v0.0.35 → v0.0.36 (security fix; CVE threshold update in doctor)
- 53-02: Headlamp v0.40.1 → v0.42.0 (token flow pre-check mandatory before writing bump)
- 53-03: cert-manager v1.16.3 → v1.20.2 (--server-side flag verified; ClusterIssuer smoke with UID 65532)
- 53-04: Envoy Gateway v1.3.1 → v1.7.2 (HTTPRoute UAT; job name verification; Gateway API CRD companion bump)
- 53-05: MetalLB hold verification (confirm v0.15.3 still latest; no file changes if confirmed)
- 53-06: Metrics Server hold verification (confirm v0.8.1 still latest; no file changes if confirmed)
- 53-07: offlinereadiness.go consolidation (single commit updating allAddonImages after all bumps; count test updated)
**Plans**: TBD (7-8 sub-plans as listed above)

**NOTES ON REQUIREMENTS vs RESEARCH DIVERGENCE**: REQUIREMENTS.md (the locked scope) specifies cert-manager v1.20.2 and Envoy Gateway v1.7.2. The research SUMMARY.md recommended holding EG at v1.3.1 and bumping cert-manager only to v1.16.5, but these recommendations were superseded when REQUIREMENTS.md was finalized. The v1.7.2 EG bump requires dedicated HTTPRoute UAT (Plan 53-04) and companion Gateway API CRD version audit. The v1.20.2 cert-manager bump requires disclosure of the `rotationPolicy: Always` default change and UID change (1000→65532) in CHANGELOG. See PITFALLS items 6-12 for per-addon hazards.

### Phase 54: macOS Ad-Hoc Code Signing
**Goal**: darwin/amd64 and darwin/arm64 GoReleaser artifacts are ad-hoc signed so Apple Silicon AMFI no longer kills the binary on first run
**Depends on**: Phase 52 (no code dependency — release pipeline only; can begin after Phase 52 to maintain sequential clarity)
**Requirements**: DIST-01
**Success Criteria** (what must be TRUE):
  1. `codesign -vvv dist/kinder_darwin_amd64_v1/kinder` returns `satisfies its Designated Requirement` in CI after a snapshot build
  2. `codesign -vvv dist/kinder_darwin_arm64/kinder` returns `satisfies its Designated Requirement` in CI after a snapshot build (both architectures verified independently)
  3. Release notes and install guide explicitly state: "ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`"
  4. The sign step is the LAST operation on each binary before archiving — no post-sign strip, UPX, or binary copy invalidates the Mach-O signature block
**Plans**: TBD (1-2 plans: GoReleaser config split + CI runner change + release notes)

### Phase 55: Windows PR-CI Build Step
**Goal**: Every PR is cross-compiled for `windows/amd64` on `ubuntu-latest`, preventing silent Windows compilation regressions
**Depends on**: Phase 52 (no code dependency — CI-only change)
**Requirements**: DIST-02
**Success Criteria** (what must be TRUE):
  1. A new `.github/workflows/build-check.yml` job runs `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` on every PR and fails the check if the build fails
  2. `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` is verified locally before the CI YAML is written (cgo transitive dependency probe — Pitfall 18)
  3. The Windows build job is blocking (failure fails the PR check) per DIST-02 requirement
**Plans**: TBD (1 plan)

### Phase 56: DEBT-04 Doctor Test Race Fix
**Goal**: `go test -race ./pkg/internal/doctor/... -count=100` passes with zero data races; the production `RunAllChecks` read path remains lock-free
**Depends on**: Phase 53 (addon bumps touch offlinereadiness.go in the same package; DEBT-04 fix must land on a stable baseline)
**Requirements**: DEBT-04
**Success Criteria** (what must be TRUE):
  1. `go test -race ./pkg/internal/doctor/... -count=100` reports zero races (100-run threshold to catch timing-dependent races — Pitfall 20)
  2. Production `check.go` does NOT add `sync.RWMutex` to the `allChecks` read path; the fix is confined to test scope via `runChecks(checks []Check)` parameter injection
  3. `kinder doctor` command timing is unchanged — no serialization regression introduced
**Plans**: TBD (1 plan)

**MUST PRECEDE Phase 57**: Both DEBT-04 (Phase 56) and doctor cosmetic fixes (Phase 57) touch `pkg/internal/doctor/`. Phase 56 must land first to give Phase 57 a mutex-free, race-clean baseline.

### Phase 57: Doctor Cosmetic Fixes
**Goal**: `kinder doctor` produces no false-positive version-skew warnings on HA clusters, and `cluster-resume-readiness` outputs actionable member-count text instead of raw etcdctl JSON
**Depends on**: Phase 56 (same package — race fix must land first)
**Requirements**: DIAG-05, DIAG-06
**Success Criteria** (what must be TRUE):
  1. `kinder doctor cluster-node-skew` on a 3-CP HA cluster with an external-load-balancer container produces no version-skew warning for the LB container; it only warns on genuine CP/worker skew
  2. `kinder doctor cluster-resume-readiness` on a cluster with 1/3 etcd members healthy outputs "1/3 etcd members healthy, quorum at risk" (or equivalent actionable text) — not the raw `etcdctl endpoint health` JSON dump
  3. Test fixtures cover both etcd 3.4.x and 3.5.x JSON shapes for the health output parser (Pitfall 22)
**Plans**: TBD (2 plans: 57-01 cluster-node-skew LB guard; 57-02 cluster-resume-readiness JSON parsing)

### Phase 58: Live UAT Closure for Phase 47 + 51
**Goal**: Both carry-forward UAT items from v2.3 are formally closed with live evidence recorded against the final v2.4 binary
**Depends on**: Phase 57 (must run against the FINAL v2.4 binary — all bumps + signing + IP-pinning + cosmetics complete; see Pitfall 23)
**Requirements**: UAT-01, UAT-02
**Success Criteria** (what must be TRUE):
  1. `./bin/kinder version` confirms the v2.4 build hash before any UAT run begins — smoke never runs against a stale PATH binary
  2. Phase 47 UAT: `scripts/uat-47-ha-smoke.sh` runs against a 3-CP + 2-worker + 1-LB cluster; verifies pause (workers→CP→LB ordering), resume (LB→CP→workers ordering), and `kubectl get nodes` returns all nodes Ready; `.planning/phases/47-cluster-pause-resume/47-UAT.md` status fields updated from `issue` to `pass`
  3. Phase 51 UAT: `docker ps` confirms `envoyproxy/envoy` (not `kindest/haproxy`) as the LB container on the HA cluster; `kinder create cluster --config <ipvs+1.36-config>` is rejected at validate with migration URL in the error message; K8s 1.36 guide page renders with its sidebar entry; `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md` created with full evidence
  4. Both UAT scripts reference `./bin/kinder` (not `kinder` from PATH) to guarantee evidence corresponds to the rebuilt binary
**Plans**: TBD (2 plans: 58-01 Phase 47 HA smoke; 58-02 Phase 51 Envoy LB + IPVS + guide)

---

## Phase Ordering Rationale

The ordering adopts the suggested sequence from the research synthesis with one clarification about Phases 54 and 55:

**Dependency chain:**
```
Phase 52 (etcd peer-TLS / IP pinning)  [highest blast radius; isolated]
  → Phase 53 (addon bumps + SYNC-05)   [sequential sub-plans; offlinereadiness.go final]
    → Phase 56 (DEBT-04 race fix)       [must precede Phase 57; same package]
      → Phase 57 (doctor cosmetics)     [depends on race-clean baseline]
        → Phase 58 (live UAT closure)   [MUST run against final v2.4 binary]

Phase 54 (macOS signing)   — independent of source code; starts after Phase 52
Phase 55 (Windows CI)      — independent of source code; starts after Phase 52
```

**Why Phase 52 is first:** PITFALLS research item 1 flags cert/network operations on running containers as catastrophic (quorum loss, data corruption). Isolating Phase 52 as the first deliverable gives all subsequent testing a working HA resume path. Any other ordering risks running addon bump integration tests (Phase 53) against a broken HA cluster.

**Why Phase 56 precedes Phase 57:** Both touch `pkg/internal/doctor/`. DEBT-04's `runChecks(checks []Check)` refactor must land before the cosmetic fixes add new test cases, so those new tests inherit the race-clean infrastructure rather than layering on top of a known-racy baseline.

**Why Phase 58 is last:** Pitfall 23 (stale binary trap) is the definitive gate. Live UAT against a binary that does not include all v2.4 changes (bumps, signing, IP pinning, cosmetics) produces misleading evidence. Phase 58 plans must hard-fail if `./bin/kinder version` does not match the expected v2.4 build hash.

**Why Phases 54 and 55 follow Phase 52 (not Phase 57):** macOS signing and Windows CI are release-pipeline changes with zero Go source dependencies. They can be interleaved with Phases 53-57 once the highest-risk work (Phase 52) is complete. The ordering shown reflects the intent to keep the critical path (52→53→56→57→58) clean; Phases 54 and 55 can execute in any window where the Phase 53-57 critical path is blocked on review or testing.

---

## Cross-Phase Concerns

**Sequential constraints (must NOT be parallelized):**

1. **Phase 56 before Phase 57** — DEBT-04 fix and doctor cosmetics both modify `pkg/internal/doctor/`. DEBT-04 refactors `allChecks` access; cosmetic fixes add new test cases. Cosmetic tests must inherit the race-clean `runChecks` infrastructure.

2. **Phase 53 sub-plans are strictly sequential** — Each addon bump updates `offlinereadiness.go` (or defers to plan 53-07). An ambiguous test failure across multiple simultaneous bumps is undiagnosable. Order: 53-00 (SYNC-05 probe) → 53-01 (local-path) → 53-02 (Headlamp) → 53-03 (cert-manager) → 53-04 (Envoy Gateway) → 53-05 (MetalLB hold) → 53-06 (Metrics Server hold) → 53-07 (offlinereadiness.go consolidation).

3. **Phase 58 is last** — Runs against the FINAL v2.4 binary after ALL other phases complete. Any UAT run before Phases 52-57 are all merged produces evidence that does not represent the shipped release.

**High-risk items requiring pre-plan discussion:**

- Phase 52: Docker IPAM feasibility probe must be Plan 52-01 Task 1; no source code until probe result known. Recommend `/gsd:discuss-phase 52` before planning.
- Phase 53-02 (Headlamp): Token flow verification must precede writing the bump plan. If `kubectl auth can-i --token=<printed> get pods` fails, hold at v0.40.1.
- Phase 53-04 (Envoy Gateway v1.7.2): Requires companion Gateway API CRD version audit. Verify `eg-gateway-helm-certgen` job name in v1.7.2 install.yaml. Two-phase bump (1.3→1.5, validate, then 1.5→1.7) is safer than a single jump.

---

## Progress

**Execution Order:**
Phases execute in numeric order. Decimal phases (inserted via `/gsd-insert-phase`) run between their surrounding integers.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8. v1.0 phases | v1.0 | 12/12 | Complete | 2026-03-01 |
| 9-14. v1.1 phases | v1.1 | 8/8 | Complete | 2026-03-02 |
| 15-18. v1.2 phases | v1.2 | 4/4 | Complete | 2026-03-02 |
| 19-24. v1.3 phases | v1.3 | 8/8 | Complete | 2026-03-03 |
| 25-29. v1.4 phases | v1.4 | 13/13 | Complete | 2026-03-04 |
| 30-34. v1.5 phases | v1.5 | 7/7 | Complete | 2026-03-04 |
| 35-37. v2.0 phases | v2.0 | 7/7 | Complete | 2026-03-05 |
| 38-41. v2.1 phases | v2.1 | 10/10 | Complete | 2026-03-06 |
| 42-46. v2.2 phases | v2.2 | 14/14 | Complete | 2026-04-10 |
| 47-51. v2.3 phases | v2.3 | 25/25 | Complete (SYNC-02 deferred) | 2026-05-07 |
| 52. HA Etcd Peer-TLS Fix | v2.4 | 2/4 | In Progress|  |
| 53. Addon Version Audit, Bumps & SYNC-05 | v2.4 | 0/TBD | Not started | - |
| 54. macOS Ad-Hoc Code Signing | v2.4 | 0/TBD | Not started | - |
| 55. Windows PR-CI Build Step | v2.4 | 0/TBD | Not started | - |
| 56. DEBT-04 Doctor Test Race Fix | v2.4 | 0/TBD | Not started | - |
| 57. Doctor Cosmetic Fixes | v2.4 | 0/TBD | Not started | - |
| 58. Live UAT Closure for Phase 47 + 51 | v2.4 | 0/TBD | Not started | - |
