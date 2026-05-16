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
- IN PROGRESS **v2.4 Hardening** - Phases 52-58 + 57.1/57.2/57.3/57.4 (started 2026-05-09)

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

- [x] **Phase 52: HA Etcd Peer-TLS Fix** - IP-pin HA control-plane containers so Docker IPAM reassignment cannot break peer connectivity on resume (completed 2026-05-10; live UAT carries forward to Phase 58)
- [x] **Phase 53: Addon Version Audit, Bumps & SYNC-05** - Audit all 7 addons, execute security and version bumps, conditionally re-run SYNC-05 node image bump (completed 2026-05-12; 4 bumps + 2 holds + 1 INCONCLUSIVE SYNC-05 probe + offlinereadiness consolidation + SC wording gap closure)
- [x] **Phase 54: macOS Ad-Hoc Code Signing** - Sign darwin/amd64 and darwin/arm64 GoReleaser artifacts via `codesign --force --sign -` on a macOS runner (completed 2026-05-12; SC4 sign-as-last-op invariant established in 54-01, snapshot-verify CI gate + 3-file SC3 disclosure + PROJECT.md Key Decisions row landed in 54-02; CI run 25746519788 green — both darwin binaries verified `satisfies its Designated Requirement`)
- [x] **Phase 55: Windows PR-CI Build Step** - Add blocking `GOOS=windows go build ./...` cross-compile step to PR CI (completed 2026-05-12; `.github/workflows/build-check.yml` on `ubuntu-24.04` runs `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` on every PR to `main` + workflow_dispatch; SC3 satisfied at workflow level — merge-level branch protection deferred to future CI-policy phase per RESEARCH; CI run 25750801764 green in 32s; verifier 3/3 passed)
- [x] **Phase 56: DEBT-04 Doctor Test Race Fix** - Eliminate `allChecks` global mutation under `t.Parallel()` via scoped `runChecks(checks []Check)` helper (completed 2026-05-12; `runChecks(checks []Check) []Result` helper extracted in `pkg/internal/doctor/check.go` with `RunAllChecks()` now a 1-line delegate; three racing parallel tests in `check_test.go` rewritten to use local `[]Check` slices; `Makefile` `test-race-doctor` target + `.github/workflows/race-check.yml` CI gate added; `CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100` exits 0 in 2.662s with zero DATA RACE; verifier 3/3 passed)
- [x] **Phase 57: Doctor Cosmetic Fixes** - Fix cluster-node-skew LB false-positive and cluster-resume-readiness JSON reason text (completed 2026-05-12; inline `external-load-balancer` + `external-etcd` role guard in `realListNodes` at `clusterskew.go:111-126` eliminates the false-positive version-skew warning; tolerant flow in `resumereadiness.go:172-207` calls `parseEtcdHealth` BEFORE the verdict so etcd 3.5+ non-zero exits still surface `"N/M etcd members healthy"` + `"quorum at risk"`; raw `"etcdctl endpoint health returned error: %v"` dump removed; Pitfall 22 fixture matrix covers etcd 3.4 + 3.5 JSON shapes; `make test-race-doctor` over `-count=100` green in 2.661s; verifier 3/3 passed)
- [x] **Phase 57.1: Phase 47 Resume re-applies Envoy LB cds/lds config (INSERTED)** - Fix Phase 47↔51 regression: `lifecycle.Resume` never re-applies the Envoy LB dynamic xDS config after the LB container restarts. The container's hardcoded entrypoint resets `/home/envoy/{cds,lds}.yaml` to `resources: []` on every start, so after pause+resume the LB has zero upstreams and host kubectl gets EOF — discovered during Phase 58 live UAT test_09 (2026-05-13) (completed 2026-05-13)
- [x] **Phase 57.2: Fix `discoverLBIPv6` — derive IPv6 mode from cluster IPFamily, not docker network EnableIPv6 (INSERTED)** - Replaced docker-network `EnableIPv6` probe with cluster-authoritative `io.x-k8s.kinder.ip-family={ipv4\|ipv6\|dual}` label stamped at create-time on all containers (LB+CP+worker) across docker/podman/nerdctl; added `loadbalancer.ClusterIPFamily(binary, node)` helper as single source of truth on resume; deleted `discoverLBIPv6` entirely; added `ipv4_compat: true` to IPv6/dual listener template branch (completed 2026-05-16; verifier 6/6 PASSED; live UAT on macOS Docker Desktop with IPv6-enabled `kind` network: IPv4 cluster pause/resume listener invariance verified — lds.yaml stays `address: "0.0.0.0"` pre+post resume and host `kubectl get nodes` returns 5/5 Ready, closing the Phase 58 UAT test_09 IPv4 regression of record; IPv6 cluster listener also invariant — lds.yaml stays `"::"` + `ipv4_compat: true` pre+post resume; SC4 macOS DD caveat documented in 57.2-02-UAT.md — Docker Desktop creates only `[::1]:port->6443/tcp` for IPv6 clusters so `ipv4_compat` is architecturally defensive but not exercised by host kubectl on this topology; IPv6 HA cluster cert-regen recovery surfaced as a separate downstream Phase 52 finding → filed as Phase 57.3)
- [ ] **Phase 57.3: HA cluster cert-regen recovery (INSERTED 2026-05-16; SCOPE EXPANDED 2026-05-16)** - After Phase 52's cert-regen fires on ANY HA cluster (IPv4, IPv6, or dual-stack), kube-apiservers crash-loop because they cannot TLS-handshake to etcd on `127.0.0.1:2379` / `[::1]:2379` (`authentication handshake failed: context deadline exceeded` → `Error creating leases: error creating storage factory: context deadline exceeded` → fatal exit). Originally filed as IPv6-only after Phase 57.2 Plan 02 test_11 (2026-05-16 11:00 UTC); scope EXPANDED on 2026-05-16 13:00 UTC after Phase 58 Plan 01 live UAT reproduced byte-identical fatal pattern on a vanilla IPv4 HA cluster (`uat-58-01`). Likely root cause: regenerated `etcd/server.crt` SAN list / regenerated peer-cert trust chain doesn't allow apiserver to re-handshake within its 20s startup budget after cert-regen — on ALL stacks, not just IPv6. Phase 52 was never UAT'd against any HA cluster post-cert-regen because Phase 52 UAT exercised the ip-pin path (IPAM probe succeeded then). **Blocks Phase 58.**
- [ ] **Phase 57.4: IPAM probe regression — `inspect returned invalid IP` (INSERTED 2026-05-16)** - `pkg/internal/doctor/ipamprobe.go:131` emits `probe runtime error: inspect returned invalid IP "invalid IP"` (literal string, with space — confirmed via `%q` quoting). `docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}'` on the alpine probe container is returning the unexpected value `"invalid IP"` on the host running this UAT (macOS Docker Desktop, 2026-05-16). Consequence: every HA cluster on this host routes to cert-regen strategy at create time (`io.x-k8s.kinder.resume-strategy=cert-regen` label) instead of the preferred ip-pin path; `/kind/ipam-state.json` is never written, so resume falls through `IPDriftDetected` to the (broken) cert-regen path. Was NOT seen during Phase 52 UAT — likely a Docker Desktop / kernel update introduced the quirk, OR a latent format-template bug when the probe container ends up on >1 network. **Blocks Phase 58 jointly with 57.3.**
- [ ] **Phase 58: Live UAT Closure for Phase 47 + 51** - Run and record live smoke tests against rebuilt v2.4 binary for both deferred UAT items. (BLOCKED 2026-05-16 on Phase 57.3 + 57.4 after Plan 01 live UAT caught real upstream defects)

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
- [x] 52-03-PLAN.md — Cert-regen fallback module + Resume() dispatch (pre-CP-start IP reapply for ip-pinned; post-start reactive wholesale regen for cert-regen/legacy)
- [x] 52-04-PLAN.md — `ha-resume-strategy` doctor check + count test bump to 26

**RISK NOTE**: This phase has the highest blast radius of any v2.4 item. Discuss with `/gsd:discuss-phase 52` before planning. Task 1 of the first plan MUST be the Docker IPAM feasibility probe — no code is written until the probe result is known. See PITFALLS research items 1-4.

### Phase 53: Addon Version Audit, Bumps & SYNC-05
**Goal**: All 7 addons are at verified current-stable versions (or documented holds), the security fix for local-path-provisioner is closed, and the SYNC-05 node image bump executes if Docker Hub has published `kindest/node:v1.36.x`
**Depends on**: Phase 52
**Requirements**: ADDON-01, ADDON-02, ADDON-03, ADDON-04, ADDON-05, SYNC-05
**Success Criteria** (what must be TRUE):
  1. `kinder create cluster` installs local-path-provisioner v0.0.36 (GHSA-7fxv-8wr2-mfc4 security fix closed); all 14 bumped/held addon tags are present on the cluster node's containerd image store (verified via `crictl images` on the control-plane node after `kinder create cluster`). NOTE: `kinder doctor offline-readiness` measures HOST docker pre-pull readiness for `--air-gapped` mode (not cluster-node store), so warns on a fresh default cluster by design; the air-gapped semantics are documented in 53-07-SUMMARY.md.
  2. `kinder create cluster` installs Headlamp v0.42.0; the printed ServiceAccount token authenticates successfully against the Headlamp UI (or a documented hold at v0.40.1 is in place with explanation)
  3. `kinder create cluster` installs cert-manager v1.20.2 with `--server-side` apply; `kubectl get crd certificates.cert-manager.io -o jsonpath='{.spec.versions[0].name}'` returns the v1.20.2 API version; self-signed ClusterIssuer issues a certificate from pods enforced non-root via pod-level `runAsNonRoot: true` + distroless image USER nonroot directive (functional UID 65532; upstream v1.20.2 does not pin `runAsUser: 65532` in the manifest — kubelet `runAsNonRoot: true` enforcement is the actual security guarantee per 53-03-SUMMARY.md UID Deviation section).
  4. `kinder create cluster` installs Envoy Gateway v1.7.2; an HTTPRoute routes traffic end-to-end; the `eg-gateway-helm-certgen` job name is verified in the v1.7.2 install.yaml before commit
  5. `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` and `TestAllAddonImages_CountMatchesExpected` reflect all bumped image references; `go test ./pkg/internal/doctor/... -run TestAllAddonImages` passes
  6. If Docker Hub two-step probe (existence + manifest digest) confirms `kindest/node:v1.36.x` is published, the default image constant in `pkg/apis/config/defaults/image.go` is updated and `kinder create cluster` with no `--image` flag uses K8s 1.36; otherwise SYNC-05 halts INCONCLUSIVE with re-runnable status

**Sub-plan order (sequential — NOT parallel, see cross-phase concerns):**
- 53-00: SYNC-05 probe — Docker Hub two-step probe (existence + manifest digest); conditional gate before any source change
- 53-01: local-path-provisioner v0.0.35 → v0.0.36 (security fix; CVE threshold update in doctor)
- 53-02: Headlamp v0.40.1 → v0.42.0 (token flow pre-check mandatory before writing bump) [COMPLETE — Path A, UAT-2 passed, ADDON-02 delivered]
- 53-03: cert-manager v1.16.3 → v1.20.2 (--server-side flag verified; ClusterIssuer smoke with UID 65532)
- 53-04: Envoy Gateway v1.3.1 → v1.7.2 (HTTPRoute UAT; job name verification; Gateway API CRD companion bump)
- 53-05: MetalLB hold verification (confirm v0.15.3 still latest; no file changes if confirmed)
- 53-06: Metrics Server hold verification (confirm v0.8.1 still latest; no file changes if confirmed)
- 53-07: offlinereadiness.go consolidation (single commit updating allAddonImages after all bumps; count test updated)
- 53-08: SC wording revision (gap closure for SC1 second clause + SC3 third clause — pure ROADMAP.md doc fix per developer decision 2026-05-12; no code changes)
**Plans**: 9 sub-plans (53-00 through 53-08; 53-08 is gap closure for SC wording — pure docs)

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
**Plans**: 2 plans
- [x] 54-01-goreleaser-darwin-signing-PLAN.md — Add darwin-gated codesign post-hook to .goreleaser.yaml builds[] + -s ldflags + switch release.yml to macos-latest runner (SC4 + prerequisite plumbing for SC1/SC2)
- [x] 54-02-snapshot-verify-and-docs-PLAN.md — Add .github/workflows/macos-sign-verify.yml snapshot+verify CI gate + SC3 wording across installation.md + changelog.md + release-notes-v2.4-draft.md + PROJECT.md Key Decisions row (SC1+SC2+SC3)

### Phase 55: Windows PR-CI Build Step
**Goal**: Every PR is cross-compiled for `windows/amd64` on `ubuntu-latest`, preventing silent Windows compilation regressions
**Depends on**: Phase 52 (no code dependency — CI-only change)
**Requirements**: DIST-02
**Success Criteria** (what must be TRUE):
  1. A new `.github/workflows/build-check.yml` job runs `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` on every PR and fails the check if the build fails
  2. `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` is verified locally before the CI YAML is written (cgo transitive dependency probe — Pitfall 18)
  3. The Windows build job is blocking (failure fails the PR check) per DIST-02 requirement
**Plans**: 1 plan
- [x] 55-01-windows-build-check-workflow-PLAN.md — Add `.github/workflows/build-check.yml` (ubuntu-24.04, SHA-pinned actions, env-block cross-compile) + green workflow_dispatch run 25750801764 (Task 3 merge-level protection deferred per RESEARCH)

### Phase 56: DEBT-04 Doctor Test Race Fix
**Goal**: `go test -race ./pkg/internal/doctor/... -count=100` passes with zero data races; the production `RunAllChecks` read path remains lock-free
**Depends on**: Phase 53 (addon bumps touch offlinereadiness.go in the same package; DEBT-04 fix must land on a stable baseline)
**Requirements**: DEBT-04
**Success Criteria** (what must be TRUE):
  1. `go test -race ./pkg/internal/doctor/... -count=100` reports zero races (100-run threshold to catch timing-dependent races — Pitfall 20)
  2. Production `check.go` does NOT add `sync.RWMutex` to the `allChecks` read path; the fix is confined to test scope via `runChecks(checks []Check)` parameter injection
  3. `kinder doctor` command timing is unchanged — no serialization regression introduced
**Plans**: 1 plan
- [x] 56-01-PLAN.md — Extract `runChecks(checks []Check) []Result` helper + rewrite 3 racing parallel tests + Makefile `test-race-doctor` target + `.github/workflows/race-check.yml` PR regression guard

**MUST PRECEDE Phase 57**: Both DEBT-04 (Phase 56) and doctor cosmetic fixes (Phase 57) touch `pkg/internal/doctor/`. Phase 56 must land first to give Phase 57 a mutex-free, race-clean baseline.

### Phase 57: Doctor Cosmetic Fixes
**Goal**: `kinder doctor` produces no false-positive version-skew warnings on HA clusters, and `cluster-resume-readiness` outputs actionable member-count text instead of raw etcdctl JSON
**Depends on**: Phase 56 (same package — race fix must land first)
**Requirements**: DIAG-05, DIAG-06
**Success Criteria** (what must be TRUE):
  1. `kinder doctor cluster-node-skew` on a 3-CP HA cluster with an external-load-balancer container produces no version-skew warning for the LB container; it only warns on genuine CP/worker skew
  2. `kinder doctor cluster-resume-readiness` on a cluster with 1/3 etcd members healthy outputs "1/3 etcd members healthy, quorum at risk" (or equivalent actionable text) — not the raw `etcdctl endpoint health` JSON dump
  3. Test fixtures cover both etcd 3.4.x and 3.5.x JSON shapes for the health output parser (Pitfall 22)
**Plans**: 2 plans
- [x] 57-01-PLAN.md — DIAG-05 cluster-node-skew inline LB/external-etcd role guard in realListNodes (clusterskew.go) + regression test (SC1) — SUMMARY: 57-01-SUMMARY.md
- [x] 57-02-PLAN.md — DIAG-06 cluster-resume-readiness tolerant JSON parsing in Run() error-branch (resumereadiness.go) + Pitfall 22 fixture matrix etcd 3.4/3.5 (SC2 + SC3) — SUMMARY: 57-02-SUMMARY.md

### Phase 57.1: Phase 47 Resume re-applies Envoy LB cds/lds config (INSERTED)

**Goal**: `kinder resume` on an HA cluster restores Envoy LB connectivity to all control-plane apiservers, so `kubectl` through the LB succeeds within the documented `--wait` window
**Depends on**: Phase 57 (clean baseline; same lifecycle package as 47/52)
**Requirements**: To be derived during planning (likely a new LIFE-10 requirement, or appended scope to LIFE-09)
**Why urgent (discovered 2026-05-13 during Phase 58 UAT test_09)**:
  - `pkg/internal/lifecycle/resume.go` starts the LB container in the documented quorum-safe ordering but **never re-applies the LB upstream config** after the LB restart
  - The Envoy LB container's entrypoint hardcodes `echo -en 'resources: []' > /home/envoy/cds.yaml && echo -en 'resources: []' > /home/envoy/lds.yaml` on every start
  - Result: post-resume the LB has zero clusters and zero listeners; host `kubectl` gets `EOF`; the apiservers themselves heal internally (verified `curl https://localhost:6443/healthz` returns `ok` from within the CP container)
  - This gap was masked in the May 7 v2.3 47-UAT because test 9 failed earlier at `strconv.ParseInt` (the `--wait` IntVar bug, fixed in 47-06 commit 7a4f722f); v2.4 Phase 58 exposed the next layer of the onion
  - Git archaeology: 47-03 commit `50c686aa` implemented Resume **before** Phase 51 swapped HAProxy→Envoy; 51-01 commit `4267886a` added the Envoy atomic-swap mechanism but only to the **create-time** action; 52-03 commit `c38bbdf1` added the HA strategy dispatch but didn't add LB reapply

**Success Criteria** (what must be TRUE):
  1. After `kinder pause` + `kinder resume --wait 5m` on a 3-CP + 2-worker + 1-LB cluster, host `kubectl --context kind-<cluster> get nodes` succeeds within the wait window (no `EOF` from the LB)
  2. After resume, `docker exec <lb> cat /home/envoy/cds.yaml` contains the 3 CP container names as upstream backends (NOT `resources: []`); `lds.yaml` contains the :6443 listener (NOT `resources: []`)
  3. Single-CP clusters (no `external-load-balancer` container present) incur zero overhead — the new code path is no-op when `nodeutils.ExternalLoadBalancerNode(allNodes)` returns nil
  4. The fix mirrors the create-time atomic-swap path (template render → `nodeutils.WriteFile` to `.tmp` → `mv` swap → Envoy file-poll picks it up); no SIGHUP, no container restart of the LB, no `docker cp` from host
  5. New regression test in `pkg/internal/lifecycle/` exercises the LB reapply path against a `FakeNode` LB and asserts the post-resume cds/lds content is non-empty
  6. Phase 58 UAT script `hack/uat-47-ha-smoke.sh` test_09 passes after this fix lands (the existing script is the regression gate — no further script changes required)

**Plans**: 2 plans (sequential — Wave 1 → Wave 2 — to avoid the 57-01 parallel-cwd commit-contamination lesson)

Plans:
- [x] 57.1-01-extract-helper-PLAN.md — Wave 1: Extract `WriteDynamicConfig(node nodes.Node, cps []nodes.Node, ipv6 bool) error` into `pkg/cluster/internal/loadbalancer/config.go`; refactor create-time `Execute()` to delegate; add 4 helper-level unit tests (happy IPv4, happy IPv6, WriteFile err, mv err)
- [x] 57.1-02-resume-wire-PLAN.md — Wave 2 (depends on 01): Wire helper into `Resume()` at Phase 1.25; add `reapplyLBConfig` + IPv6 discovery (docker network inspect) + retry-3x-with-1s-backoff in new `pkg/internal/lifecycle/lbreapply.go`; add 6 lifecycle tests (no-LB no-op, happy IPv4, IPv6 detect, retry success on attempt 2, retry exhausted, IPv6 discovery fallback)

**RISK NOTE (SUPERSEDED by 57.1-CONTEXT.md D-lock 1)**: The earlier guidance ("AFTER cert-regen/ip-pin finishes") was caution, not a derived constraint. CONTEXT.md locks the insertion at Phase 1.25 (after LB start, BEFORE Phase 1.5 ip-pin / Phase 2 CP start). Rationale: cds/lds encode CP container *names*, not IPs; ip-pin and cert-regen are CP-side concerns; Envoy file-polls and tolerates not-yet-running upstream backends. Symmetric with create-time which writes cds/lds while CPs are still starting.

### Phase 57.2: Fix `discoverLBIPv6` — derive IPv6 mode from cluster IPFamily, not docker network EnableIPv6 (INSERTED)

**Goal**: `kinder resume` on a vanilla IPv4 HA cluster renders the LB listener at `address: "0.0.0.0"` (IPv4) — not `"::"` (IPv6-only) — so host `kubectl` on the docker port-mapping reaches the apiserver after resume, on macOS Docker Desktop's default dual-stack kind network.
**Depends on**: Phase 57.1 (same lifecycle package; fixes a bug introduced by 57.1)
**Requirements**: To be derived during planning (likely a new LIFE-11 requirement, or appended scope to whatever 57.1 wrote)
**Why urgent (discovered 2026-05-13 during Phase 58 UAT test_09)**:
  - `pkg/internal/lifecycle/lbreapply.go:59-90` (`discoverLBIPv6`) probes `docker network inspect --format '{{.EnableIPv6}}'`. On macOS Docker Desktop the `kind` network is dual-stack by default (`EnableIPv6=true`), regardless of cluster IPFamily.
  - For a vanilla IPv4 cluster (no `ipFamily` in spec), the create-time path at `pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go:70-71` correctly sets `ipv6 := ctx.Config.Networking.IPFamily == config.IPv6Family || == config.DualStackFamily` → IPv4 mode → listener `address: "0.0.0.0"`.
  - On resume, `discoverLBIPv6` returns `true` for the same cluster → re-renders listener `address: "::"`. Envoy defaults `socket_address.ipv4_compat: false`, so the listener becomes IPv6-only.
  - Forensic evidence (cluster `uat-58-01` left running 2026-05-13): TCP from host's port-mapping reaches Envoy but TLS returns EOF; from inside the `kind` network `openssl s_client -connect 172.19.0.2:6443` → `Connection refused`; `[fc00:f853:ccd:e793::2]:6443` → TLS OK with `subject=CN=kube-apiserver`.
  - The two paths must agree on the meaning of "IPv6". Authoritative source is the cluster spec, not the docker network's capability flag.

**Success Criteria** (what must be TRUE):
  1. After `kinder pause` + `kinder resume --wait 5m` on a vanilla IPv4 3-CP + 2-worker + 1-LB cluster, `docker exec <lb> cat /home/envoy/lds.yaml | grep -E 'address:\s*"0\.0\.0\.0"'` returns a match (IPv4 listener); `address: "::"` does NOT appear in the listener block.
  2. After resume on a vanilla IPv4 cluster, host `kubectl --context kind-<cluster> get nodes` succeeds within the `--wait` window (validates SC1 end-to-end; closes the 58-UAT test_09 regression).
  3. After resume on an explicit IPv6 or DualStack cluster (`networking.ipFamily: ipv6` or `dual`), `docker exec <lb> cat /home/envoy/lds.yaml | grep -E 'address:\s*"::"'` returns a match — IPv6 mode is preserved when the cluster genuinely wants it.
  4. `discoverLBIPv6` no longer references `docker network inspect ... EnableIPv6`; the IPv6 decision is derived from a cluster-authoritative source (candidates per STATE Blockers: first CP's primary network IP family; a label set by create-time IP-pin module; a config file inside a CP container — to be locked during `/gsd:discuss-phase 57.2`).
  5. Phase 57.1's existing 6 lifecycle tests still pass; Phase 57.2 adds at least one regression test that asserts the resume-time IPv6 flag matches the create-time IPv6 flag for both IPv4 and dual-stack clusters (using FakeNode/FakeCmd test infra to avoid live docker dependency).
  6. Phase 58 UAT script `hack/uat-47-ha-smoke.sh` test_09 passes against the post-57.2 binary on macOS Docker Desktop (the failed pre-57.2 log at `hack/uat-47-ha-smoke.log.pre-57.2` is the comparison baseline).

**Plans**: 2 plans

Plans:
- [x] 57.2-01-PLAN.md — Atomic code fix (autonomous): RED tests + GREEN implementation of ip-family label injection across docker/podman/nerdctl (LB+CP+worker); ClusterIPFamily helper in pkg/cluster/loadbalancer/; ipv4_compat: true on the IPv6/dual listener template branch; resume calls helper; delete discoverLBIPv6 entirely
- [x] 57.2-02-PLAN.md — Live IPv6 UAT on macOS Docker Desktop (developer-driven; requires Docker Desktop IPv6 enabled): IPv4 + IPv6 cluster create/pause/resume listener invariance + host kubectl succeeds + ip-family label present on every container

**Details**:
The fix-shape options to discuss (each has tradeoffs; the right choice depends on existing labels/state already written during create-time):

  - **Option A — Inspect first CP's primary network entry**: `docker inspect <cp1> --format '{{json .NetworkSettings.Networks}}'` and check whether the network entry has a non-empty `IPAddress` (IPv4) vs only `GlobalIPv6Address` (IPv6-only). Self-contained; no schema change. Risk: dual-stack clusters have BOTH; need to align on whether the LB listener for dual-stack should be `"::"` (matches create-time) or `"0.0.0.0"` (broader macOS Docker Desktop compatibility).
  - **Option B — Read a label written by create-time**: Phase 52's IP-pin module already writes labels (`io.x-k8s.kinder.resume-strategy`); extend that with `io.x-k8s.kinder.ip-family={ipv4|ipv6|dual}` so resume reads the authoritative cluster decision directly. Requires create-time edit; cleaner for the long term.
  - **Option C — Parse a config file inside a CP container**: Read `/kind/kinder-cluster-config.yaml` (or wherever the cluster spec is persisted) via `docker exec <cp1> cat`. Most authoritative but slowest and most fragile (path/format coupling).

Forensic state available for the planner:
  - Failed UAT log: `hack/uat-47-ha-smoke.log.pre-57.2` (untracked)
  - Live cluster `uat-58-01` left running on host (3-CP + 2-worker + 1-LB; cert-regen strategy; on dual-stack `kind` network) — exercise candidate fixes against this cluster before committing.

### Phase 57.3: HA cluster cert-regen recovery (INSERTED 2026-05-16; SCOPE EXPANDED 2026-05-16)

**Goal**: After `kinder pause` + `kinder resume` on ANY HA cluster (IPv4, IPv6, or dual-stack), every `kube-apiserver` process successfully completes its TLS handshake to etcd on its loopback within 30s of apiserver start, and all CP nodes report Ready within the `--wait 10m` window.
**Depends on**: Phase 52 (the cert-regen module being fixed). **Blocks Phase 58.**
**Requirements**: To be derived during planning (likely a new LIFE-12 requirement, or appended scope to LIFE-09).
**Why urgent**:

The defect was originally filed as IPv6-only after Phase 57.2 Plan 02 test_11 on 2026-05-16 11:00 UTC. **Phase 58 Plan 01 live UAT on 2026-05-16 13:00 UTC then reproduced byte-identical fatal pattern on a vanilla IPv4 HA cluster** (`uat-58-01`, no `ipFamily` in spec), so the scope is NOT IPv6-specific — Phase 52's cert-regen recovery is broken on ALL HA stacks.

**Forensic evidence (IPv4 — discovered 2026-05-16 13:00 UTC on `uat-58-01` cluster, 3-CP + 2-worker + 1-LB)**:
  - `kube-apiserver` post-cert-regen container on cp1: ATTEMPT=8, state=Exited (crash loop). NOT listening on `:6443` anywhere (`curl -k https://127.0.0.1:6443/healthz` from inside cp1 → `Could not connect to server`).
  - `etcd` IS listening on `127.0.0.1:2379` + `172.19.0.3:2379` + peer `172.19.0.3:2380` + metrics `127.0.0.1:2381`.
  - `crictl logs` of the post-cert-regen kube-apiserver container shows: `grpc: addrConn.createTransport failed to connect to {Addr: "127.0.0.1:2379"}` → `transport: authentication handshake failed: context deadline exceeded` → `F instance.go:233] Error creating leases: error creating storage factory: context deadline exceeded` after ~20s → fatal exit → crash loop.
  - Cluster has labels `io.x-k8s.kinder.ip-family=ipv4` (Phase 57.2 label correct), `io.x-k8s.kinder.resume-strategy=cert-regen` (routed via Phase 57.4 IPAM probe regression).
  - `/kind/ipam-state.json` does NOT exist on any CP (cert-regen strategy at create time, no ip-pin state written).
  - Reference: `.planning/phases/58-live-uat-closure-for-phase-47-51/uat-logs/2026-05-16-uat-58-01-cert-regen-failure.txt` (88 lines).

**Forensic evidence (IPv6 — discovered 2026-05-16 11:00 UTC on `uat-572-ipv6` cluster)**: Identical fatal pattern except `Addr: "[::1]:2379"` and ports `[::1]:2379` + `[fc00:...:a]:2379`. Reference: `.planning/phases/57.2-fix-discoverlbipv6-derive-from-cluster-ipfamily/uat-logs/ipv6-diag.txt` + `57.2-02-UAT.md` "Forensic Deep-Dive" section.

This is architecturally upstream of Phase 57.2's LB listener fix (LB lds.yaml + cds.yaml + container labels are correct on both clusters). The failure is INSIDE the CP container in BOTH cases.

**Why never caught before**: Phase 52's UAT exercised the **ip-pin path** (IPAM probe succeeded then). The cert-regen path was implemented as a documented fallback (Phase 52 SC4) but the actual recovery behavior was only ever unit-tested with `FakeNode`/`FakeCmd`, never live. Phase 57.4's IPAM probe regression (also filed 2026-05-16) now routes every HA cluster to cert-regen, exposing this latent defect.

**Likely fix areas** (to be locked during `/gsd:discuss-phase 57.3`):
  - **Option A — etcd server.crt SAN list / trust chain**: Phase 52's cert-regen may be regenerating `etcd/server.crt` or `etcd/peer.crt` with a SAN list or trust-chain configuration that doesn't allow apiserver→etcd TLS to re-establish after cert rotation. Verify via `openssl x509 -in /etc/kubernetes/pki/etcd/server.crt -noout -text` on cp1 after cert-regen — compare to pre-pause cert.
  - **Option B — etcd peer re-coordination timing**: Phase 52 regenerates peer certs sequentially on all 3 CPs. The cluster may fail to re-elect a leader / restore quorum before the apiserver's 20s startup timeout. Add an etcd-readiness gate AFTER cert-regen on all 3 CPs and BEFORE letting the apiserver restart.
  - **Option C — apiserver static-pod manifest stale-config**: The apiserver static-pod manifest may reference cert paths or fingerprints that no longer match after cert-regen. Verify the manifest is re-rendered (or kubelet re-reads it) between cert-regen and apiserver restart.

**Success Criteria** (what must be TRUE):
  1. After `kinder pause` + `kinder resume --wait 10m` on a vanilla IPv4 HA cluster (3-CP + 2-worker + 1-LB), `docker exec <cp1> sh -c 'curl -k --max-time 5 https://127.0.0.1:6443/healthz'` returns `ok` (apiserver bound and serving) within the wait window. Verified specifically against the cert-regen strategy path (post-57.4 the default may flip back to ip-pin; test must force cert-regen for this SC).
  2. After resume on a vanilla IPv4 HA cluster, host `kubectl --context kind-<cluster> get nodes` returns all 5 nodes (3 CP + 2 worker) in `Ready` state within the `--wait 10m` window.
  3. After resume on an explicit IPv6 HA cluster (`networking.ipFamily: ipv6`), `docker exec <cp1> sh -c 'curl -k --max-time 5 https://127.0.0.1:6443/healthz'` returns `ok` within the wait window (parity with SC1 on the IPv6 stack).
  4. New regression test asserts the regenerated etcd cert chain allows apiserver→etcd TLS handshake to succeed (using FakeNode/FakeCmd + a real openssl x509 verify on the regenerated cert) — covers both IPv4 and IPv6 loopback SANs.
  5. Phase 58 UAT script `hack/uat-47-ha-smoke.sh` test_09 passes against the post-57.3+57.4 binary on macOS Docker Desktop. Phase 57.2 UAT script `hack/uat-57.2-ipv6-listener.sh` test_11 also passes (IPv6 parity).

**Plans**: 2 plans (`/gsd:plan-phase 57.3` 2026-05-16):
- [ ] 57.3-01-PLAN.md — Diagnostic capture from uat-58-01; widen RegenerateEtcdPeerCertsWholesale cert scope; active etcd + apiserver health-gates replace static sleep; IPv6/dual-stack loopback addressing; post-pass `kubeadm certs check-expiration` verify; diagnostic dump on hard-fail; `--strategy=<auto|ip-pin|cert-regen>` cobra flag wired through ResumeOptions; unit-test matrix; ROADMAP/REQUIREMENTS doc updates
- [ ] 57.3-02-PLAN.md — Live UAT `hack/uat-57.3-cert-regen.sh` covering IPv4 + IPv6 + dual-stack HA fixtures; 57.3-02-UAT.md evidence + verdict

**Forensic state available for the planner**:
  - IPv4 failure (this expansion): `.planning/phases/58-live-uat-closure-for-phase-47-51/uat-logs/2026-05-16-uat-58-01-cert-regen-failure.txt` (full forensic snapshot) + `2026-05-16-uat-58-01-fail.log` (script's tee output)
  - IPv6 failure (original filing): `.planning/phases/57.2-fix-discoverlbipv6-derive-from-cluster-ipfamily/uat-logs/ipv6-resume.log` + `ipv6-kubectl.log` + `ipv6-diag.txt`
  - Phase 57.2 verification of the LB-layer correctness: `.planning/phases/57.2-fix-discoverlbipv6-derive-from-cluster-ipfamily/57.2-VERIFICATION.md`
  - Live cluster left up for planner forensic access: `uat-58-01` (IPv4) on the host as of 2026-05-16 13:11 UTC (`./bin/kinder delete cluster --name uat-58-01` when no longer needed)

### Phase 57.4: IPAM probe regression — `inspect returned invalid IP` (INSERTED 2026-05-16)

**Goal**: `kinder doctor ipam-probe` (and the equivalent in-process check at HA cluster create time) returns `VerdictIPPinned` on a macOS Docker Desktop host with a normal docker daemon, NOT `VerdictCertRegen` with the bogus error string `inspect returned invalid IP "invalid IP"`.
**Depends on**: Phase 52 (the IPAM probe module being fixed). **Blocks Phase 58.** Sibling to Phase 57.3 (also a Phase 52 downstream defect; either can land first; both must land before Phase 58 re-runs).
**Requirements**: To be derived during planning (likely a new LIFE-13 requirement, or appended scope to LIFE-09).
**Why urgent (discovered 2026-05-16 13:00 UTC during Phase 58 Plan 01 live UAT)**:
  - At HA cluster create time, kinder prints the line: `HA cluster will use cert-regen resume strategy: probe runtime error: inspect returned invalid IP "invalid IP"` — the `originalIP` value being passed back is the literal string `"invalid IP"` with a space (confirmed by the `%q` formatter in `pkg/internal/doctor/ipamprobe.go:131` quoting it as `"invalid IP"`).
  - Consequence: every HA cluster on this host today routes to cert-regen strategy at create time. The downstream Phase 57.3 defect (broken cert-regen recovery) is hit on 100% of HA clusters on this host.
  - The probe code path: spawn an alpine probe container, connect it to a temporary probe network, run `docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' <probe>`, then `net.ParseIP(originalIP)`. The ParseIP rejection is the trigger; the upstream question is why `docker inspect` returned `"invalid IP"`.
  - Phase 52 UAT didn't see this — the regression is new (Docker Desktop update, kernel update, or a network-namespace state introduced by Phase 57.2's label-injection changes that didn't exist at Phase 52 UAT time).
  - Reference: `.planning/phases/58-live-uat-closure-for-phase-47-51/uat-logs/2026-05-16-uat-58-01-cert-regen-failure.txt` § "IPAM probe source" + `2026-05-16-uat-58-01-fail.log` line containing `probe runtime error: inspect returned invalid IP`.

**Likely fix areas** (to be locked during `/gsd:discuss-phase 57.4`):
  - **Option A — investigate WHY `docker inspect` returns `"invalid IP"`**: Run the inspect command manually on a probe container today and see what comes back. Is the probe container ending up on multiple networks (resulting in concatenated IPs)? Is Docker Desktop returning a placeholder for some network state? This is the cheap, high-information-value path.
  - **Option B — make the probe robust to multi-network inspect**: Change the format template to `'{{(index .NetworkSettings.Networks "<probeNet>").IPAddress}}'` so it asks for the specific probe network's IP, not "all networks concatenated". This avoids the underlying confusion regardless of cause.
  - **Option C — fall back to ip-pin path heuristically**: If the inspect-based probe is unreliable, replace it with a `docker network connect --ip` dry-run attempt directly. The probe's purpose is feasibility detection, not IP capture; the IP captured here isn't actually USED for anything (the probe container is discarded).

**Success Criteria** (what must be TRUE):
  1. On a macOS Docker Desktop host (2026-05-16+ versions) with the default `kind` network, `./bin/kinder doctor ipam-probe` reports `VerdictIPPinned` (not `VerdictCertRegen` with `inspect returned invalid IP`).
  2. `kinder create cluster --config <ha.yaml>` on the same host emits `HA cluster will use ip-pin resume strategy` (NOT `cert-regen ... probe runtime error`); `/kind/ipam-state.json` IS written on every CP after create.
  3. New regression test fixture captures the failing `docker inspect` output verbatim (whatever it actually returns today — the literal `"invalid IP"` string or a multi-IP concatenation) and asserts the probe code handles it correctly (either repairs the format template, or detects the bad value and returns a clean fallback Verdict).
  4. Phase 58 UAT script `hack/uat-47-ha-smoke.sh` test_09 passes against the post-57.3+57.4 binary (joint SC with Phase 57.3).

**Plans**: TBD (1-2 plans expected; `/gsd:discuss-phase 57.4` to investigate the docker inspect output first; then `/gsd:plan-phase 57.4`).

**Forensic state available for the planner**:
  - Failed UAT cluster left up: `uat-58-01` (3-CP + 2-worker + 1-LB, IPv4, on `kind` network, labels `io.x-k8s.kinder.resume-strategy=cert-regen`, `/kind/ipam-state.json` absent on all CPs)
  - Forensic snapshot: `.planning/phases/58-live-uat-closure-for-phase-47-51/uat-logs/2026-05-16-uat-58-01-cert-regen-failure.txt`
  - Source code: `pkg/internal/doctor/ipamprobe.go:115-135` (probe + ParseIP rejection at line 131)
  - Commit history: `bb31049e feat(52-01): implement IPAM probe doctor check` (no later edits — this is the original Phase 52 implementation that's now failing)

### Phase 58: Live UAT Closure for Phase 47 + 51
**Goal**: Both carry-forward UAT items from v2.3 are formally closed with live evidence recorded against the final v2.4 binary
**Depends on**: Phase 57.2 (LB IPv6-detection fix — VERIFIED CLOSED 2026-05-16), Phase 57.1 (LB reapply fix), Phase 57 (must run against the FINAL v2.4 binary — all bumps + signing + IP-pinning + cosmetics complete; see Pitfall 23). **NEWLY BLOCKED 2026-05-16 on Phase 57.3 + Phase 57.4** after Plan 01 live UAT caught two upstream Phase 52 defects. Phase 58 IS NOT IPv6-vs-IPv4 specific anymore — the defects affect both stacks.
**Requirements**: UAT-01, UAT-02
**Plan 01 status (2026-05-16)**: PAUSED at live-UAT checkpoint. Script (`hack/uat-47-ha-smoke.sh`) committed and clean at HEAD (`5bd9e673`, `74b90199`, `696c2cc3`). Live run on `uat-58-01` cluster: Pitfall-23 freshness gate ✅, test_03 ✅, test_09 ❌ (real upstream defect — see Phase 57.3 + 57.4). 47-UAT.md rows NOT flipped; canonical log NOT committed. Re-run after Phase 57.3 + 57.4 land.
**Success Criteria** (what must be TRUE):
  1. `./bin/kinder version` confirms the v2.4 build hash before any UAT run begins — smoke never runs against a stale PATH binary
  2. Phase 47 UAT: `scripts/uat-47-ha-smoke.sh` runs against a 3-CP + 2-worker + 1-LB cluster; verifies pause (workers→CP→LB ordering), resume (LB→CP→workers ordering), and `kubectl get nodes` returns all nodes Ready; `.planning/phases/47-cluster-pause-resume/47-UAT.md` status fields updated from `issue` to `pass`
  3. Phase 51 UAT: `docker ps` confirms `envoyproxy/envoy` (not `kindest/haproxy`) as the LB container on the HA cluster; `kinder create cluster --config <ipvs+1.36-config>` is rejected at validate with migration URL in the error message; K8s 1.36 guide page renders with its sidebar entry; `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md` created with full evidence
  4. Both UAT scripts reference `./bin/kinder` (not `kinder` from PATH) to guarantee evidence corresponds to the rebuilt binary
**Plans**: 2 plans

Plans:
- [ ] 58-01-ha-smoke-PLAN.md — Phase 47 HA pause/resume live UAT against rebuilt v2.4 binary; flips 47-UAT.md tests 3/9/12/13/14 from issue to pass
- [ ] 58-02-envoy-ipvs-guide-PLAN.md — Phase 51 Envoy LB + IPVS-1.36 reject + K8s 1.36 guide re-verification against rebuilt v2.4 binary; augments 51-UAT.md with v2.4 evidence section

---

## Phase Ordering Rationale

The ordering adopts the suggested sequence from the research synthesis with one clarification about Phases 54 and 55:

**Dependency chain:**
```
Phase 52 (etcd peer-TLS / IP pinning)  [highest blast radius; isolated]
  → Phase 53 (addon bumps + SYNC-05)   [sequential sub-plans; offlinereadiness.go final]
    → Phase 56 (DEBT-04 race fix)       [must precede Phase 57; same package]
      → Phase 57 (doctor cosmetics)     [depends on race-clean baseline]
        → Phase 57.1 (LB reapply fix)   [INSERTED 2026-05-13; UAT test_09 regression]
          → Phase 57.2 (LB IPv6 detect) [INSERTED 2026-05-13; surfaced by Phase 58 UAT; fixes a 57.1 sub-regression]
            → Phase 57.3 (HA cert-regen recovery) [INSERTED 2026-05-16 as IPv6-only;
                                                    SCOPE EXPANDED 2026-05-16 to all HA stacks
                                                    after Phase 58 Plan 01 reproduced on IPv4]
            → Phase 57.4 (IPAM probe regression) [INSERTED 2026-05-16; surfaced by Phase 58 Plan 01;
                                                  sibling of 57.3 — either can land first, both block 58]
              → Phase 58 (live UAT closure) [MUST run against final v2.4 binary;
                                              blocked on Phase 57.3 + 57.4 as of 2026-05-16 13:00 UTC]

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
| 52. HA Etcd Peer-TLS Fix | v2.4 | 4/4 | Complete (UAT→Phase 58) | 2026-05-10 |
| 53. Addon Version Audit, Bumps & SYNC-05 | v2.4 | 9/9 | Complete | 2026-05-12 |
| 54. macOS Ad-Hoc Code Signing | v2.4 | 2/2 | Complete | 2026-05-12 |
| 55. Windows PR-CI Build Step | v2.4 | 1/1 | Complete | 2026-05-12 |
| 56. DEBT-04 Doctor Test Race Fix | v2.4 | 1/1 | Complete | 2026-05-12 |
| 57. Doctor Cosmetic Fixes | v2.4 | 2/2 | Complete | 2026-05-12 |
| 57.1. Phase 47 Resume re-applies Envoy LB cds/lds config (INSERTED) | v2.4 | 2/2 | Complete (regression filed → 57.2) | 2026-05-13 |
| 57.2. Fix `discoverLBIPv6` (INSERTED) | v2.4 | 2/2 | Complete (IPv6 cert-regen finding filed → 57.3) | 2026-05-16 |
| 57.3. HA cluster cert-regen recovery (INSERTED; SCOPE EXPANDED) | v2.4 | 0/? | Pending `/gsd:discuss-phase 57.3` (now covers IPv4 + IPv6 + dual) | - |
| 57.4. IPAM probe regression (INSERTED) | v2.4 | 0/? | Pending `/gsd:discuss-phase 57.4` | - |
| 58. Live UAT Closure for Phase 47 + 51 | v2.4 | 0/2 | BLOCKED on Phase 57.3 + 57.4 (Plan 01 paused at live-UAT) | - |
