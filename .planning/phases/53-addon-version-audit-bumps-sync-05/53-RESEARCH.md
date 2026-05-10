# Phase 53: Addon Version Audit, Bumps & SYNC-05 - Research

**Researched:** 2026-05-10
**Domain:** Kubernetes addon manifest version bumps + Docker Hub gated default-image probe
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### UAT depth per addon

- **local-path-provisioner v0.0.36** — Full smoke + StorageClass review:
  - Cluster create + addon pods Ready + image digest matches v0.0.36
  - `kubectl create pvc` → verify dynamic provisioning binds, pod mounts the volume
  - Verify default StorageClass annotations, reclaim policy unchanged, host-path permissions correct
- **Headlamp v0.42.0** — Token auth smoke (live), per roadmap mandate:
  - Cluster create → kinder prints SA token
  - `kubectl auth can-i --token=<X> get pods` returns yes
  - `curl` Headlamp UI with token returns 200
- **cert-manager v1.20.2** — ClusterIssuer + cert smoke + UID verification:
  - Create self-signed ClusterIssuer, request a Certificate, verify it issues
  - Verify cert-manager pods run as UID 65532 (the upstream default change — note: CONTEXT.md decision bullet had a typo "65632"; REQUIREMENTS.md ADDON-03 and verified upstream release notes say 65532)
- **Envoy Gateway v1.7.2** — Install + HTTPRoute traffic (roadmap-mandated):
  - Cluster create, EG pods Ready, Gateway resource accepted (status: Programmed)
  - Create HTTPRoute, send curl traffic end-to-end through the gateway, verify 200

#### Hold/abort criteria (per addon)

- **Headlamp hold trigger** — ANY of:
  - `kubectl auth can-i --token=<printed> get pods` fails or errors
  - Headlamp UI rejects the printed token (login screen loop)
  - Token generation/printing mechanism changes (rbac.yaml shape, SA name, namespace) in a way that breaks existing kinder docs
  - On hold: stay at v0.40.1, document hold + reason in 53-02 commit + CHANGELOG
- **EG hold trigger** — ANY of:
  - End-to-end HTTPRoute curl returns non-200 or hangs
  - The `eg-gateway-helm-certgen` job name changed in v1.7.2 install.yaml without a documented migration
  - Breaking field change in Gateway API CRDs that affects existing kinder users' HTTPRoutes
  - On hold: stay at v1.3.1, document hold + reason in 53-04 commit + CHANGELOG
- **cert-manager hold trigger** — ANY of:
  - Self-signed ClusterIssuer fails to issue a Certificate on a fresh kinder cluster
  - The UID 65532 change breaks any persistent volume / mounted-secret scenarios in existing kinder addons
  - On hold: stay at v1.16.3, document hold + reason in 53-03 commit + CHANGELOG
- **Phase-level hold policy** — Hold the addon, finish the phase:
  - Sub-plan documents the hold + reason
  - Remaining sub-plans (cert-manager, MetalLB hold-verify, Metrics hold-verify, offlinereadiness consolidation) all proceed
  - Phase ships with hold disclosed in CHANGELOG and release notes

#### EG bump strategy (53-04)

- **Single jump v1.3.1 → v1.7.2** — One commit, one HTTPRoute smoke test. (User overrode the "two-phase 1.3→1.5→1.7" recommendation from Cross-Phase Concerns.) Hold criteria above are the safety net.
- **Gateway API CRD pinning** — Match EG's bundled CRDs (whatever EG v1.7.2 install.yaml ships). Trust upstream. No separate explicit pin in addon source. (Researcher confirmed: EG v1.7.2 install.yaml ships Gateway API CRDs at bundle-version `v1.4.1`.)
- **`eg-gateway-helm-certgen` job name verification** — Pre-bump in research phase. **VERIFIED below: name is unchanged in v1.7.2.**

#### SYNC-05 (53-00)

- **If probe confirms publication** (`kindest/node:v1.36.x` exists on Docker Hub with valid manifest digest):
  - Bump `pkg/apis/config/defaults/image.go` constant
  - Update CI integration test matrix to include 1.36 (or replace the floor)
  - Update website examples + CLI reference + tutorials to reference K8s 1.36 where "latest" is implied
- **If probe is INCONCLUSIVE** (image not yet published):
  - 53-00 records INCONCLUSIVE status, no source change for default image
  - Addons (53-01 through 53-07) all proceed normally
  - SYNC-05 deferred to v2.5 or re-run later — re-runnable status preserved

(Researcher pre-ran the probe today: `count=0` for `?name=v1.36`. SYNC-05 will be INCONCLUSIVE at execute time unless kind ships v0.32.0 between now and execution. See `## State of the Art` and the SYNC-05 section in `## Code Examples`.)

#### Breaking-change disclosure

- **Three-tier disclosure** for cert-manager UID 65532, rotationPolicy: Always default change, EG major bump, CRD changes, and (if SYNC-05 fires) default image bump:
  - **CHANGELOG.md** — Per-addon entries
  - **Addon docs (website)** — Breaking-change callout box on each affected addon page
  - **v2.4 release notes (GitHub release body)** — Dedicated section for upgraders
- **CHANGELOG cadence** — Both: stub atomic + final consolidate:
  - Each sub-plan commit (53-01, 53-02, …) adds a stub CHANGELOG line atomically in the same commit
  - 53-07 (offlinereadiness consolidation) also reformats/consolidates the stubs into a polished v2.4 entry

### Claude's Discretion

- Hold-verification depth for MetalLB v0.15.3 and Metrics Server v0.8.1 (53-05, 53-06) — area not selected for discussion; planner picks between "upstream version check only" vs "re-validate functionality on a fresh cluster" based on what existing tests cover.
- Specific test names, file paths, and where to wire new test cases.
- Exact CHANGELOG entry wording (subject to project convention).
- How to structure the website "breaking-change callout" component (existing pattern, if any, takes precedence).

### Deferred Ideas (OUT OF SCOPE)

- Hold-verification depth for the two holds (MetalLB v0.15.3, Metrics Server v0.8.1) — left to planner discretion this phase; could be revisited if a hold-verify regression surfaces.
- Two-phase EG bump (v1.3 → v1.5 → v1.7) as a fallback if single-jump smoke fails — captured here so the planner knows it's a documented retreat path, not a new capability.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ADDON-01 | local-path-provisioner v0.0.36 (CVE GHSA-7fxv-8wr2-mfc4 fix); embedded busybox pin retained | Section "Standard Stack — local-path-provisioner"; Pitfall LPP-01 (busybox pin loss); upstream manifest verified to use unpinned `busybox` (so kinder vendored override `busybox:1.37.0` MUST be re-applied after copy) |
| ADDON-02 | Headlamp v0.42.0 with mandatory plan-time token-print flow verification (hold at v0.40.1 if broken) | Section "Headlamp v0.42.0 upstream changes"; CONTEXT hold criteria; verified via upstream release notes + ghcr image presence |
| ADDON-03 | cert-manager v1.20.2 with `--server-side`; UID change (1000→65532); ClusterIssuer smoke | Section "cert-manager v1.20.2 upstream changes"; v1.20.2 manifest size = 992 KB (still > 256 KB) confirmed; UID change verified in v1.20.0 release notes; rotationPolicy: Always default change verified in v1.18.0 release notes |
| ADDON-04 | Envoy Gateway v1.7.2 (jumps two majors); HTTPRoute UAT; certgen job name verification; Gateway API CRD version lock | Section "Envoy Gateway v1.7.2 upstream changes"; **certgen job name VERIFIED unchanged**; bundled Gateway API CRDs at v1.4.1 (was v1.2.1) |
| ADDON-05 | All bumped image refs propagated to `pkg/internal/doctor/offlinereadiness.go` `allAddonImages`; `TestAllAddonImages_CountMatchesExpected` updated | Section "offlinereadiness.go consolidation"; current count = 14; verified post-bump count remains 14 (same number of images per addon — only tags change) |
| SYNC-05 | Default `kindest/node` to K8s 1.36.x conditional on Docker Hub publication | Section "SYNC-05 (53-00) — Docker Hub probe"; researcher pre-ran probe — currently `count=0`, INCONCLUSIVE expected |

</phase_requirements>

## Summary

This is a low-novelty, high-discipline phase: copy upstream addon manifests of known-locked versions into the corresponding `pkg/cluster/internal/create/actions/install*/manifests/*.yaml` files, update the matching `Images` slice in each `*.go` action, then consolidate `offlinereadiness.go`. All four addon-bump plans (53-01..53-04) follow the SAME mechanical pattern; the only variation is the per-addon UAT/hold criteria. Three meaningful unknowns existed coming into research: (1) was `eg-gateway-helm-certgen` renamed in EG v1.7.2 — **VERIFIED no, name unchanged**; (2) does cert-manager v1.20.2 still need `--server-side` — **VERIFIED yes (manifest is 992 KB, well above the 256 KB annotation limit)**; (3) is `kindest/node:v1.36.x` published — **NO as of probe today (2026-05-10), expect INCONCLUSIVE**.

The work is dominated by manifest hygiene risks documented in PITFALLS.md items 6–12 (CRD annotation limits, webhook startup races, CRD/version-lock between EG and Gateway API, busybox pin loss). The pattern set by Phase 51 plan 51-04 (Docker Hub two-step probe with INCONCLUSIVE re-runnable exit) IS the canonical SYNC pattern — clone it for 53-00.

**Primary recommendation:** Mirror Phase 51 plan layout exactly. Each addon-bump plan gets one TDD RED→GREEN cycle that pins the new version string in a `Images` test, then updates the manifest YAML and the `Images` slice atomically. Run the unit test suite after each plan; do NOT batch. The offlinereadiness count test (`TestAllAddonImages_CountMatchesExpected`) stays at 14 — only the image tags shift. The `--server-side` apply call must NOT regress in any cert-manager bump (currently locked decision; verify retention in 53-03).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Addon manifest YAML (embedded) | go:embed at build time | — | Compiled into the kinder binary; deterministic across machines |
| Addon Images slice (Go) | Package-level `var Images []string` per `installX` package | `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` (mirrors all packages' Images) | Single source of truth per addon, consolidated for offline-readiness check |
| Default node image constant | `pkg/apis/config/defaults/image.go` `Image` const | Tested by `image_test.go` (created in Phase 51 plan 51-04 RED step; expected to exist if SYNC-05 ever fired) | Compile-time pinned; `kinder create cluster` reads this when no `--image` is given |
| Addon install action | `pkg/cluster/internal/create/actions/installX/X.go` `Execute(ctx)` | Test in `installX/X_test.go` using `testutil.FakeNode`/`FakeCmd` | Each addon owns its own action with a private test harness; no cross-addon coupling |
| K8s 1.36 IPVS guard | `pkg/internal/apis/config/validate.go` validateConfig | `validate_test.go` | Compile-time validator; fires on cluster create, not on the default-image bump |
| Docker Hub probe (SYNC-05) | Bash + curl in plan execution (no Go code) | Plan SUMMARY.md as the audit trail | Probe is one-shot at plan time; not a runtime concern of kinder itself |
| CHANGELOG entries | `kinder-site/src/content/docs/changelog.md` (Astro Starlight markdown) | Per-addon docs at `kinder-site/src/content/docs/addons/*.md` | Site renders the changelog as a docs page; per-addon page hosts the breaking-change callout |
| Breaking-change callout | `:::caution[Title]` Starlight aside, embedded inline in addon docs | — | Existing pattern (e.g. `metallb.md:173 :::caution[Rootless Podman limitation]`) |

## Standard Stack

### Core (already in use; do NOT introduce alternatives)

| Library / Tool | Version | Purpose | Why Standard |
|----------------|---------|---------|--------------|
| go:embed | stdlib | Embed addon YAML manifests at build time | Already the project pattern — every `install*` action uses it |
| `kubectl apply -f -` (via stdin pipe to control-plane container) | n/a | Apply addon manifests | The action pattern: `node.CommandContext(ctx, "kubectl", "...", "apply", "-f", "-").SetStdin(strings.NewReader(manifest))` |
| `kubectl apply --server-side` | n/a | For manifests > 256 KB (cert-manager, Envoy Gateway) | Required when the YAML exceeds the last-applied-configuration annotation limit |
| `kubectl wait` for Deployments / Jobs / GatewayClass | n/a | Gate addon readiness | Established pattern in every `install*` action |
| `testutil.FakeNode` + `testutil.FakeCmd` | internal | Mock control-plane node command queue for unit tests | Established pattern; see `installdashboard/dashboard_test.go` for the canonical layout |
| `curl` + `jq` | shell | Docker Hub probe (SYNC-05) | Mirrors Phase 51 plan 51-04 Task 1 exactly |
| `:::caution[Title]` (Astro Starlight aside) | site | Breaking-change callout in addon docs | Already used (metallb, coredns, metrics-server, local-path, host-directory-mounting, hpa) |

### Versions to Land

| Addon | Current | Target | Verified | Source |
|-------|---------|--------|----------|--------|
| local-path-provisioner | v0.0.35 | **v0.0.36** | [VERIFIED: github.com/rancher/local-path-provisioner/releases/tag/v0.0.36] | Release notes confirm GHSA-7fxv-8wr2-mfc4 fix |
| Headlamp | v0.40.1 | **v0.42.0** | [VERIFIED: ghcr.io/headlamp-k8s/headlamp:v0.42.0 published] | Release notes (no breaking auth changes), image confirmed published |
| cert-manager | v1.16.3 | **v1.20.2** | [VERIFIED: github.com/cert-manager/cert-manager/releases/tag/v1.20.2] | Manifest downloaded (992 KB, 13623 lines); 3 images (cainjector, controller, webhook) |
| Envoy Gateway | v1.3.1 | **v1.7.2** | [VERIFIED: install.yaml downloaded from upstream] | Manifest downloaded (3.3 MB, 52834 lines); image: `envoyproxy/gateway:v1.7.2`; ratelimit image bumped to `docker.io/envoyproxy/ratelimit:05c08d03`; bundled Gateway API CRDs at bundle-version v1.4.1 |
| MetalLB | v0.15.3 | v0.15.3 (HOLD) | [VERIFIED: github.com/metallb/metallb/releases — latest is v0.15.3, 2024-12-04] | No newer release exists; hold confirmed |
| Metrics Server | v0.8.1 | v0.8.1 (HOLD) | [VERIFIED: github.com/kubernetes-sigs/metrics-server/releases — latest is v0.8.1, 2026-01-29] | No newer release exists; hold confirmed |
| kindest/node default | v1.35.1 | v1.36.x (CONDITIONAL) | [VERIFIED: probe at 2026-05-10 returns count=0] | Docker Hub `?name=v1.36` returns empty; SYNC-05 will be INCONCLUSIVE at execute time |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Bumping in 7 sequential plans | One mega-plan that bumps all addons | Ambiguous failure attribution (PITFALLS item 5); locked NO by ROADMAP cross-phase concerns |
| `kubectl apply` (client-side) for cert-manager / EG | `kubectl apply --server-side` | Client-side fails on > 256 KB manifests; locked YES (already current pattern) |
| EG two-phase bump (v1.3→v1.5→v1.7) | Single-jump v1.3.1→v1.7.2 | User locked single-jump; two-phase is documented deferred fallback |
| Pin Gateway API CRDs separately | Trust EG-bundled CRDs | User locked: trust upstream bundle (v1.4.1 in EG v1.7.2) |

**Installation (no new dependencies):**
```bash
# Zero new Go module dependencies. All work is manifest YAML + Images slice updates.
go test ./pkg/internal/doctor/... -run TestAllAddonImages
go test ./pkg/cluster/internal/create/actions/... -race
```

**Version verification commands** (run during execution):
```bash
# Docker Hub probe for SYNC-05 (mirror of plan 51-04 Task 1):
curl -s 'https://hub.docker.com/v2/repositories/kindest/node/tags/?page_size=100&name=v1.36' | \
  jq -r '.results[] | "\(.name)\t\(.digest // .images[0].digest // "no-digest")"' | sort -V

# Latest version checks:
curl -s 'https://api.github.com/repos/rancher/local-path-provisioner/releases/latest' | jq -r '.tag_name'
curl -s 'https://api.github.com/repos/headlamp-k8s/headlamp/releases/latest' | jq -r '.tag_name'
curl -s 'https://api.github.com/repos/cert-manager/cert-manager/releases/latest' | jq -r '.tag_name'
curl -s 'https://api.github.com/repos/envoyproxy/gateway/releases/latest' | jq -r '.tag_name'
curl -s 'https://api.github.com/repos/metallb/metallb/releases/latest' | jq -r '.tag_name'
curl -s 'https://api.github.com/repos/kubernetes-sigs/metrics-server/releases/latest' | jq -r '.tag_name'
```

## Architecture Patterns

### System Architecture Diagram

```
                     Phase 53 sub-plan flow (sequential, NOT parallel)
                     ────────────────────────────────────────────────

  53-00 SYNC-05 probe ──> Outcome A (image found)        Outcome B (count=0)
       │                  ──────────────────────         ─────────────────────
       │                  Capture TAG + DIGEST           Write INCONCLUSIVE summary
       │                  (cont. in 53-00 same plan)     STOP this sub-plan
       │                                                 (other plans continue)
       │
       ▼
  53-01 local-path v0.0.36
       │   ┌── Update manifests/local-path-storage.yaml (preserve busybox:1.37.0 + is-default-class annotation)
       │   ├── Update Images slice in localpathprovisioner.go
       │   ├── Atomic CHANGELOG stub
       │   └── go test ./pkg/cluster/.../installlocalpath/... -race
       ▼
  53-02 Headlamp v0.42.0           [HOLD on token failure → stay v0.40.1]
       │   Same pattern + token-print smoke
       ▼
  53-03 cert-manager v1.20.2       [HOLD on ClusterIssuer failure → stay v1.16.3]
       │   Same pattern + verify --server-side preserved + UID 65532 verification
       ▼
  53-04 Envoy Gateway v1.7.2       [HOLD on HTTPRoute failure → stay v1.3.1]
       │   Same pattern + HTTPRoute smoke + bundled CRDs at v1.4.1
       ▼
  53-05 MetalLB hold-verify (no source change if confirmed)
       ▼
  53-06 Metrics Server hold-verify (no source change if confirmed)
       ▼
  53-07 offlinereadiness consolidation
           ├── Update allAddonImages slice with bumped tags (count stays at 14)
           ├── Re-run TestAllAddonImages_CountMatchesExpected (must pass)
           └── Consolidate per-plan CHANGELOG stubs into a polished v2.4 section


  Probe details (53-00, mirrors plan 51-04 Task 1):
  ────────────────────────────────────────────────
  Step 1 (existence):  curl https://hub.docker.com/v2/repositories/kindest/node/tags/?page_size=100&name=v1.36
  Step 2 (digest):     curl -H "Authorization: Bearer <token>" https://registry-1.docker.io/v2/kindest/node/manifests/<TAG>
                       (or: docker pull && docker inspect to read RepoDigests)
```

### Component Responsibilities

| Layer | Files | What changes per plan |
|-------|-------|------------------------|
| Embedded addon manifest | `pkg/cluster/internal/create/actions/install{localpath,dashboard,certmanager,envoygw,metallb,metricsserver}/manifests/*.yaml` | YAML body replaced with new upstream version |
| Addon `Images` slice | `pkg/cluster/internal/create/actions/install{...}/{addon}.go` | Tag strings updated to match manifest |
| Action implementation | Same `.go` file | NO changes expected (apply-and-wait pattern is stable); only changes if upstream renames a Job/Deployment/Namespace |
| Action unit test | `installX/X_test.go` | New version string assertions if any |
| Offline-readiness | `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` | Image tag strings updated; count stays 14 |
| Offline-readiness test | `pkg/internal/doctor/offlinereadiness_test.go` `TestAllAddonImages_CountMatchesExpected` | NO change needed unless an addon adds/removes an image (verified: none do for these versions) |
| Default node image | `pkg/apis/config/defaults/image.go` `Image` constant | Updated ONLY if 53-00 returns Outcome A |
| Default image test | `pkg/apis/config/defaults/image_test.go` (created in 51-04 if Outcome A; otherwise still absent) | RED→GREEN per plan 51-04 pattern if 53-00 returns Outcome A |
| Addon docs | `kinder-site/src/content/docs/addons/*.md` (line 8 area: `kinder installs X **vY.Y.Y**`) | Update version string + add `:::caution[Breaking change]` for cert-manager and EG |
| Changelog | `kinder-site/src/content/docs/changelog.md` (top-level H2 = milestone) | Per-plan stub; consolidated into v2.4 section in 53-07 |

### Recommended File Map

```
pkg/
├── apis/config/defaults/
│   ├── image.go                                       [SYNC-05: const Image — bump if Outcome A]
│   └── image_test.go                                  [SYNC-05: created in plan 51-04 RED if Outcome A; absent today]
├── cluster/internal/create/actions/
│   ├── installlocalpath/
│   │   ├── localpathprovisioner.go                    [53-01: Images slice → v0.0.36]
│   │   ├── localpathprovisioner_test.go               [53-01: assertions if any]
│   │   └── manifests/local-path-storage.yaml          [53-01: replace YAML, RE-PIN busybox:1.37.0, RE-ADD is-default-class annotation]
│   ├── installdashboard/
│   │   ├── dashboard.go                               [53-02: Images slice → v0.42.0]
│   │   ├── dashboard_test.go                          [53-02: token-print test still passes]
│   │   └── manifests/headlamp.yaml                    [53-02: image: ghcr.io/headlamp-k8s/headlamp:v0.42.0; verify SA/Secret/Service/Deployment shape unchanged]
│   ├── installcertmanager/
│   │   ├── certmanager.go                             [53-03: Images slice → v1.20.2; verify --server-side preserved]
│   │   ├── certmanager_test.go                        [53-03: --server-side flag assertion]
│   │   └── manifests/cert-manager.yaml                [53-03: replace with upstream v1.20.2 (992 KB)]
│   ├── installenvoygw/
│   │   ├── envoygw.go                                 [53-04: Images slice → gateway:v1.7.2 + ratelimit:05c08d03]
│   │   ├── envoygw_test.go                            [53-04: certgen job-name assertion still valid]
│   │   └── manifests/install.yaml                     [53-04: replace with upstream v1.7.2 install.yaml (3.3 MB)]
│   ├── installmetallb/                                [53-05: hold — no source change]
│   └── installmetricsserver/                          [53-06: hold — no source change]
└── internal/doctor/
    ├── offlinereadiness.go                            [53-07: allAddonImages tag updates; count stays 14]
    └── offlinereadiness_test.go                       [53-07: TestAllAddonImages_CountMatchesExpected unchanged at 14]

kinder-site/src/content/docs/
├── addons/
│   ├── local-path-provisioner.md                      [53-01: bump version line; mention security fix]
│   ├── headlamp.md                                    [53-02: bump version line]
│   ├── cert-manager.md                                [53-03: bump version + :::caution[Breaking change in v1.20] for UID + rotationPolicy]
│   ├── envoy-gateway.md                               [53-04: bump version + :::caution[Major upgrade] for v1.3→v1.7 + Gateway API CRD bump v1.2.1→v1.4.1]
│   ├── metallb.md                                     [53-05: no change unless hold-verify reveals new info]
│   └── metrics-server.md                              [53-06: no change unless hold-verify reveals new info]
├── changelog.md                                       [53-07: consolidated v2.4 section; per-plan stub commits already in main]
└── guides/                                            [SYNC-05 only: examples that pin v1.35.1 as default → bump to v1.36.x if Outcome A]
```

### Pattern 1: Per-addon TDD bump cycle (RED → GREEN)

**What:** Each addon-bump plan adds one assertion test pinning the new version, then updates the manifest + Images slice + offlinereadiness entry to make it pass.
**When to use:** Every addon bump (53-01 through 53-04). The hold-verify plans (53-05, 53-06) skip the GREEN step if the version check confirms no upstream change.
**Example (matches plan 51-01 / 51-04 cadence):**
```go
// Source: pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go (extended)
// RED commit: tests/<addon>: pin Headlamp Images to v0.42.0
func TestImagesPinsHeadlampV042(t *testing.T) {
    t.Parallel()
    const want = "ghcr.io/headlamp-k8s/headlamp:v0.42.0"
    found := false
    for _, img := range Images {
        if img == want {
            found = true
        }
    }
    if !found {
        t.Errorf("Images = %v, want to contain %q", Images, want)
    }
}
```
After RED commit, the test fails. GREEN commit: update `Images` slice to `["ghcr.io/headlamp-k8s/headlamp:v0.42.0"]` AND update `manifests/headlamp.yaml` line 62 (`image:`) to match. Both files in the SAME GREEN commit.

### Pattern 2: SYNC-05 Docker Hub two-step probe (mirror of plan 51-04 Task 1)

**What:** A pre-flight probe that gates source-code changes on upstream image availability.
**When to use:** Plan 53-00 only. The pattern is bash + `curl` + `jq` — no Go code.
**Example (verbatim from plan 51-04 Task 1, working today):**
```bash
# Step 1 (existence):
curl -s 'https://hub.docker.com/v2/repositories/kindest/node/tags/?page_size=100&name=v1.36' | \
  jq -r '.results[] | "\(.name)\t\(.digest // .images[0].digest // "no-digest")"' | \
  sort -V

# Outcome A: at least one v1.36.x tag — pick highest patch, capture digest, proceed to Outcome A flow
# Outcome B: empty results — write INCONCLUSIVE summary and STOP this sub-plan

# Step 2 (digest, if Outcome A):
TAG=v1.36.X   # from step 1
curl -s -H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" \
  -H "Authorization: Bearer $(curl -s 'https://auth.docker.io/token?service=registry.docker.io&scope=repository:kindest/node:pull' | jq -r .token)" \
  "https://registry-1.docker.io/v2/kindest/node/manifests/$TAG" | sha256sum
# OR: docker pull kindest/node:$TAG && docker inspect kindest/node:$TAG --format '{{index .RepoDigests 0}}'
```

### Pattern 3: Server-side apply for large manifests

**What:** `kubectl apply --server-side` for any manifest exceeding 256 KB.
**When to use:** cert-manager (992 KB), Envoy Gateway (3.3 MB). Already the locked pattern.
**Example (current, do NOT regress):**
```go
// Source: pkg/cluster/internal/create/actions/installcertmanager/certmanager.go:76-80
if err := node.CommandContext(ctx.Context,
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "--server-side", "-f", "-",
).SetStdin(strings.NewReader(certManagerManifest)).Run(); err != nil {
    return errors.Wrap(err, "failed to apply cert-manager manifest")
}
```
**Verification step in 53-03:** grep the action file for `--server-side` AFTER editing — if absent, the bump is broken.

### Pattern 4: Vendored manifest preservation overrides

**What:** Some upstream manifests are slightly modified for kinder. Those modifications MUST be preserved when copying a new upstream version.
**When to use:** local-path-provisioner specifically.
**Specific overrides (53-01) — VERIFIED today:**
- Upstream `deploy/local-path-storage.yaml` uses unpinned `image: docker.io/library/busybox` (in helperPod.yaml + helper-image flag); kinder's vendored manifest pins to `busybox:1.37.0` in BOTH places (lines 107 + 169 of current manifest).
- Upstream StorageClass has no annotations; kinder's vendored manifest adds `storageclass.kubernetes.io/is-default-class: "true"` (line 128 of current manifest).
- Upstream may set `--debug` flag inconsistently; kinder's manifest sets it explicitly. Verify after copy.

**Verification:** After replacing the YAML, diff against the previous file; the only differences should be:
1. Image tag in container `image:` (rancher/local-path-provisioner:v0.0.35 → v0.0.36)
2. Any upstream improvements (e.g., new RBAC rules) — accept these
3. Re-applied kinder overrides (busybox pin, is-default-class annotation)

### Anti-Patterns to Avoid

- **Skipping `--server-side` for cert-manager / EG:** Hits 256 KB annotation limit; warn-and-continue path fails silently. PITFALLS item 6. Always verify the flag is present after editing.
- **Bumping EG without bumping bundled Gateway API CRDs:** EG v1.3.1 ships Gateway API v1.2.1; v1.7.2 ships v1.4.1. The CRDs are IN the install.yaml — replacing the whole file handles this automatically. Do NOT split out CRDs into a separate file.
- **Bumping local-path-provisioner without re-pinning busybox:** Air-gap users get `ErrImagePull` at PVC-provision time. PITFALLS item 12.
- **Forgetting `is-default-class: "true"` annotation on local-path StorageClass:** Cluster ships with no default StorageClass; PVCs without explicit class go Pending forever.
- **Adding new test cases to `TestAllAddonImages_CountMatchesExpected` count:** The count is 14 today and stays 14 — none of these bumps add/remove an image per addon. Don't bump the constant unless you actually add a new image.
- **Updating offlinereadiness in each addon plan:** CONTEXT locks consolidation to 53-07. Per-plan offlinereadiness edits would create merge conflicts and ambiguous test failures. Each plan touches its own `Images` slice; 53-07 mirrors them all into `allAddonImages`.
- **Single jump for EG with no hold criteria:** User chose single-jump; the hold criteria (HTTPRoute curl, certgen job rename, CRD breaking change) ARE the safety net. Plan 53-04 must run all three checks BEFORE marking the bump committed.
- **Treating CONTEXT.md typo "65632" as truth:** REQUIREMENTS.md ADDON-03 and the upstream cert-manager v1.20.0 release notes both say 65532. Use 65532 in plans, tests, and CHANGELOG.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Docker Hub image existence check | Custom HTTP client + retry logic | `curl` + `jq` per plan 51-04 Task 1 | One-shot pre-flight; not a runtime concern; precedent established |
| Manifest large-resource apply | Vendored kubectl alternative | `kubectl apply --server-side` over node CommandContext | Already standard; works, tested, locked decision |
| Container-image presence detection | Custom registry client | `realInspectImage` already in offlinereadiness.go (uses `docker/podman/nerdctl inspect`) | Existing helper; no need for new code |
| Helm chart rendering | Add helm to kinder | `kubectl apply -f <embedded-yaml>` | Helm explicitly forbidden by PROJECT.md decision; embedded YAML is the project pattern |
| Cert-manager UID change detection | grep manifest YAML | `kubectl get pod -n cert-manager <pod> -o jsonpath='{.spec.containers[0].securityContext.runAsUser}'` after install | Manifest defers UID to image's USER directive; only the running pod tells you the truth |
| Re-implementing K8s 1.36 IPVS guard | Add new guard | Already implemented in `validate.go` (Phase 51) | Existing — no work needed |
| Breaking-change callout component | Custom Astro component | `:::caution[Title]` Starlight aside | Already used 6+ times across docs; site renderer handles it |

**Key insight:** Every problem this phase touches has an established kinder/Phase-51 pattern. The phase IS the pattern, repeated 7 times. Novelty is zero — discipline is everything.

## Runtime State Inventory

This phase is NOT a rename/refactor. Image tags change but no string identifiers move (no namespace renames, no SA renames, no CRD group renames). The runtime-state inventory is therefore narrow:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — addon image bumps don't move K8s data. PVC backing dirs (`/opt/local-path-provisioner`) keep working across local-path bumps. | None |
| Live service config | None — kinder addons are in-cluster K8s resources, not external services. | None |
| OS-registered state | None — no Task Scheduler, launchd, or systemd registrations involved | None |
| Secrets/env vars | The `kinder-dashboard-token` Secret is in-cluster only; not affected by image bump. | None — verify post-Headlamp-bump that the existing `kubernetes.io/service-account-token` Secret pattern still produces a usable token |
| Build artifacts | go:embed compiles manifests into the binary. After any manifest edit, `go build ./...` MUST be re-run. The `bin/kinder` binary is stale until rebuilt. | Each plan ends with `go build ./...` to refresh; integration tests (when added) must use rebuilt binary, not PATH |

**The canonical question:** *After every file in the repo is updated, what runtime systems still have the old string cached, stored, or registered?*

**Answer for this phase:** Existing kinder clusters created with the old addon versions keep running their old addons. There is NO upgrade path for existing clusters (per PROJECT.md decision: no `kinder upgrade`). Users recreate clusters to get new addon versions. This is by design — document it in the v2.4 release notes.

## Common Pitfalls

### Pitfall LPP-01: local-path-provisioner busybox pin lost on manifest copy
**What goes wrong:** Upstream v0.0.36 manifest uses unpinned `busybox` (no tag). If the planner copies the upstream YAML verbatim, the air-gap `allAddonImages` list (which expects `busybox:1.37.0`) goes stale and `realInspectImage` returns false even though `busybox` (latest) is loaded.
**Why it happens:** Upstream manifest is built for online use; kinder is air-gap-aware.
**How to avoid:** After replacing the YAML, run `grep -nE "image:.*busybox|--helper-image" manifests/local-path-storage.yaml` and verify TWO occurrences both pin to `busybox:1.37.0`. Add an assertion test: `TestLocalPathManifestPinsBusyboxVersion` greps the embedded string for `busybox:1.37.0`.
**Warning signs:** PVC stuck Pending after cluster create; `kubectl describe pod` on the helper pod shows `ErrImagePull` for `busybox:latest`.

### Pitfall LPP-02: local-path StorageClass loses is-default-class annotation
**What goes wrong:** Upstream manifest does not set `storageclass.kubernetes.io/is-default-class: "true"`. Without this annotation, PVCs without an explicit `storageClassName` go Pending. kinder's whole inner-loop story relies on `local-path` being default.
**Why it happens:** Same as LPP-01 — upstream optimizes for explicit-class usage; kinder optimizes for "just works".
**How to avoid:** After replacing the YAML, grep for `is-default-class` and verify it's `"true"`. Add an assertion test: `TestLocalPathStorageClassIsDefault` parses the manifest and asserts the annotation.
**Warning signs:** `kubectl get sc` shows `local-path (default)` is missing the `(default)` marker; PVCs from existing kinder docs/tutorials hang in Pending.

### Pitfall HEAD-01: Headlamp token-print regresses on `-in-cluster` arg removal
**What goes wrong:** Upstream Headlamp release notes for v0.42.0 do not mention removing `-in-cluster` or `-plugins-dir`, but a major version could drop them. If they're removed, the deployment template's `args:` block no longer wires the SA token bootstrap, and the printed token won't authenticate.
**Why it happens:** Helm chart values change between versions.
**How to avoid:** After replacing `manifests/headlamp.yaml`, grep for `-in-cluster` and verify it's still present. Also verify `targetPort: 4466` (the container port) is unchanged — port renames break the kubectl port-forward UX printed in the action.
**Warning signs:** `kubectl logs -n kube-system deployment/headlamp` shows "auth required" even when token is supplied; or port-forward command in the success message points to a closed port.

### Pitfall CERT-01 (research/PITFALLS item 6 reaffirmed): cert-manager `--server-side` flag silently dropped
**What goes wrong:** During the v1.16.3→v1.20.2 manifest update, the `--server-side` flag in `certmanager.go:76-80` is preserved by code-edit habit, but a careless refactor could drop it. cert-manager v1.20.2 manifest is 992 KB (4× bigger than the 256 KB annotation limit). Without `--server-side`, addon install fails silently if the action wraps with errors.Wrap (which is the case).
**Why it happens:** "tidy up" PRs sometimes consolidate the `apply` call.
**How to avoid:** Add a guard test in `certmanager_test.go`: `TestExecuteUsesServerSideApply` greps the captured FakeCmd args for `"--server-side"`. Make it a hard test, not a smoke test.
**Warning signs:** `kubectl get crd | grep cert-manager` shows old version after create; cert-manager pods CrashLoopBackOff with `v1alpha2: no kind is registered`.

### Pitfall CERT-02 (research/PITFALLS item 7 reaffirmed): cert-manager webhook startup race
**What goes wrong:** v1.20.2 webhook validation logic may be slower than v1.16.3. The current 300s timeout in `certmanager.go:99` should be enough but verify on a fresh local cluster before committing.
**How to avoid:** Existing wait pattern handles this; just verify timeout adequacy in 53-03 smoke. Do not lower the timeout.

### Pitfall CERT-03: cert-manager UID change breaks PVC ownership in air-gap setups
**What goes wrong:** UID 1000→65532 means any PVC pre-populated with files owned by UID 1000 becomes unreadable to the new cert-manager pod. cert-manager itself doesn't use PVCs, BUT if any sidecar / operator integrating with cert-manager mounts shared volumes, the UID flip can break them.
**Why it happens:** Default container user changed in v1.20.0 (verified upstream).
**How to avoid:** In 53-03 UAT, run `kubectl get pod -n cert-manager <pod> -o jsonpath='{.spec.containers[0].securityContext.runAsUser}'` and verify 65532. Document in CHANGELOG and addon doc with `:::caution[Breaking change in v1.20]`. CONTEXT-locked.
**Warning signs:** Pre-existing kinder users running custom cert-manager-adjacent integrations report file-permission errors after rebuild.

### Pitfall CERT-04: rotationPolicy: Always default flip silently re-issues certs
**What goes wrong:** v1.18.0 changed the default `Certificate.Spec.PrivateKey.RotationPolicy` from `Never` to `Always`. cert-manager v1.20.2 makes this GA-and-mandatory (cannot disable). Any kinder user with a long-running Certificate that doesn't explicitly set `rotationPolicy: Never` will see a NEW private key issued on next renewal. This is a runtime behavior change.
**Why it happens:** v1.18 release notes verified.
**How to avoid:** Disclose in CHANGELOG and the cert-manager addon doc `:::caution[Breaking change in v1.20]`. There is no code change in kinder — it's a user-facing behavior note.

### Pitfall EG-01 (research/PITFALLS item 8 reaffirmed): Gateway API CRD/EG version lock — REPLACED by full install.yaml swap
**What goes wrong:** EG v1.3.1 bundles Gateway API CRDs at v1.2.1; EG v1.7.2 bundles v1.4.1. If only the gateway image is bumped without replacing CRDs, HTTPRoutes fail admission with "no kind registered."
**Why it happens:** Two coupled-but-separate concerns in the same YAML.
**How to avoid:** **Replace the WHOLE `install.yaml` file as a unit.** EG ships everything (CRDs, RBAC, Job, Deployment) in the install.yaml. Do NOT split. Verified: kinder's current `installenvoygw/manifests/install.yaml` is the upstream EG install.yaml in full. Just download v1.7.2 and replace.
**Verification command:** `grep "gateway.networking.k8s.io/bundle-version" manifests/install.yaml | sort -u` should show `v1.4.1` (was `v1.2.1`).

### Pitfall EG-02 (load-bearing): certgen Job rename verification
**What goes wrong:** kinder's `installenvoygw/envoygw.go` waits on `job/eg-gateway-helm-certgen` by hardcoded name. If upstream renamed it in v1.7.2, the wait would time out.
**Why it happens:** Helm chart template renames are common in major bumps.
**How to avoid:** **VERIFIED today — name is unchanged in v1.7.2.** Researcher pulled `https://github.com/envoyproxy/gateway/releases/download/v1.7.2/install.yaml` (3.3 MB, 52834 lines) and confirmed:
```
line 52743:  # Source: gateway-helm/templates/certgen.yaml
line 52744:  apiVersion: batch/v1
line 52745:  kind: Job
line 52746:  metadata:
line 52747:    name: eg-gateway-helm-certgen
line 52748:    namespace: 'envoy-gateway-system'
```
Same name, same namespace as v1.3.1. NO code change needed in `envoygw.go` for the certgen wait. Plan 53-04 still adds an assertion test that the embedded manifest contains `name: eg-gateway-helm-certgen`.
**Warning signs:** `kubectl wait --for=condition=Complete job/eg-gateway-helm-certgen` times out after 120s. (Won't happen in v1.7.2 per verification.)

### Pitfall EG-03: ratelimit image tag change in v1.7.2
**What goes wrong:** kinder's `installenvoygw/envoygw.go` `Images = []string{"docker.io/envoyproxy/ratelimit:ae4cee11", "envoyproxy/gateway:v1.3.1"}`. The ratelimit tag is a content-addressable hash that bumps with EG version. v1.7.2 ships `docker.io/envoyproxy/ratelimit:05c08d03` (verified from upstream install.yaml line 52162).
**Why it happens:** Ratelimit is built from a different repo and pinned by hash in EG's chart values.
**How to avoid:** Update BOTH images in the `Images` slice and the offlinereadiness `allAddonImages` list. Plan 53-04 must update both.
**Warning signs:** Air-gap users get `ErrImagePull` for the old `ae4cee11` tag.

### Pitfall ALL-01: bin/kinder stale after manifest changes
**What goes wrong:** Manifests are go:embedded. Edits to YAML files are NOT reflected in `bin/kinder` until `go build ./...` re-runs. If a smoke test runs against a stale binary, it tests the OLD manifest.
**How to avoid:** Each plan's verify step ends with `go build ./...` BEFORE any cluster-create smoke test. Phase 58 documents this with Pitfall 23 ("stale binary trap"); the same applies here for any per-plan local smoke.

### Pitfall ALL-02: offlinereadiness count test brittleness
**What goes wrong:** `TestAllAddonImages_CountMatchesExpected` uses `const expected = 14`. If the planner accidentally adds or removes an image entry while updating tags, the test fails — but the failure mode is "look at the new count and update the constant," which can mask a real bug.
**How to avoid:** Verify the count BEFORE editing — current is 14. After bumps, verify the slice still has exactly 14 entries (1 LB + 1 registry + 2 metallb + 1 metrics + 3 cert-mgr + 2 EG + 1 dashboard + 1 GPU + 2 local-path = 14). If any of these changes, update the test constant deliberately and document the why in the plan SUMMARY.

## Code Examples

Verified patterns from the existing codebase:

### Addon install action (apply-and-wait)
```go
// Source: pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go
//go:embed manifests/local-path-storage.yaml
var localPathManifest string

var Images = []string{
    "docker.io/rancher/local-path-provisioner:v0.0.35",  // <-- 53-01 bumps to v0.0.36
    "docker.io/library/busybox:1.37.0",
}

// Apply the embedded local-path-provisioner manifest via kubectl
if err := node.CommandContext(ctx.Context, "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "-f", "-",
).SetStdin(strings.NewReader(localPathManifest)).Run(); err != nil {
    return errors.Wrap(err, "failed to apply local-path-provisioner manifest")
}
```

### Server-side apply for large manifests (cert-manager / EG)
```go
// Source: pkg/cluster/internal/create/actions/installcertmanager/certmanager.go:76-80
// --server-side is REQUIRED: cert-manager CRDs exceed the 256 KB last-applied-configuration
// annotation limit imposed by standard kubectl apply.
if err := node.CommandContext(ctx.Context, "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "--server-side", "-f", "-",
).SetStdin(strings.NewReader(certManagerManifest)).Run(); err != nil {
    return errors.Wrap(err, "failed to apply cert-manager manifest")
}
```

### Wait for Job completion (Envoy Gateway certgen)
```go
// Source: pkg/cluster/internal/create/actions/installenvoygw/envoygw.go:85-95
// Step 2: Wait for certgen Job to complete (creates TLS Secrets for the controller)
// Job name "eg-gateway-helm-certgen" verified for v1.3.1; re-verified for v1.7.2 in research
if err := node.CommandContext(ctx.Context, "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "wait",
    "--namespace=envoy-gateway-system",
    "--for=condition=Complete",
    "job/eg-gateway-helm-certgen",
    "--timeout=120s",
).Run(); err != nil {
    return errors.Wrap(err, "Envoy Gateway certgen job did not complete")
}
```

### Headlamp token print (read SA token Secret)
```go
// Source: pkg/cluster/internal/create/actions/installdashboard/dashboard.go:88-105
var tokenBuf bytes.Buffer
if err := node.CommandContext(ctx.Context, "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "get", "secret", "kinder-dashboard-token",
    "--namespace=kube-system",
    "-o", "jsonpath={.data.token}",
).SetStdout(&tokenBuf).Run(); err != nil {
    return errors.Wrap(err, "failed to read dashboard token from secret")
}

// Decode base64 in Go to avoid shell compatibility issues
tokenBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(tokenBuf.String()))
```

### offlinereadiness.go canonical image list (current state)
```go
// Source: pkg/internal/doctor/offlinereadiness.go:49-73
// Total: 14 — count test pins this.
var allAddonImages = []addonImage{
    {"docker.io/envoyproxy/envoy:v1.36.2", "Load Balancer (HA)"},
    {"registry:2", "Local Registry"},
    {"quay.io/metallb/controller:v0.15.3", "MetalLB"},
    {"quay.io/metallb/speaker:v0.15.3", "MetalLB"},
    {"registry.k8s.io/metrics-server/metrics-server:v0.8.1", "Metrics Server"},
    {"quay.io/jetstack/cert-manager-cainjector:v1.16.3", "Cert Manager"},   // → v1.20.2 in 53-07
    {"quay.io/jetstack/cert-manager-controller:v1.16.3", "Cert Manager"},   // → v1.20.2
    {"quay.io/jetstack/cert-manager-webhook:v1.16.3", "Cert Manager"},      // → v1.20.2
    {"docker.io/envoyproxy/ratelimit:ae4cee11", "Envoy Gateway"},           // → ratelimit:05c08d03
    {"envoyproxy/gateway:v1.3.1", "Envoy Gateway"},                         // → v1.7.2
    {"ghcr.io/headlamp-k8s/headlamp:v0.40.1", "Dashboard"},                 // → v0.42.0
    {"nvcr.io/nvidia/k8s-device-plugin:v0.17.1", "NVIDIA GPU"},
    {"docker.io/rancher/local-path-provisioner:v0.0.35", "Local Path Provisioner"},  // → v0.0.36
    {"docker.io/library/busybox:1.37.0", "Local Path Provisioner"},
}
```

### SYNC-05 Outcome B INCONCLUSIVE summary template (mirror of 51-04-SUMMARY.md)
```markdown
## INCONCLUSIVE

Plan 53-00 cannot proceed past the gating probe: no `kindest/node:v1.36.x` image is
published on Docker Hub as of <today's date>. Latest available is `kindest/node:v1.35.1`
(kind v0.31.0 default, published 2026-02-14). Re-run this plan after kind ships
v0.32.0 (or whatever release publishes the v1.36 image), or after a manual
`kind build node-image` is uploaded.

SC6 / SYNC-05 (default node image is K8s 1.36.x) remains DEFERRED.

### Probe output
GET https://hub.docker.com/v2/repositories/kindest/node/tags/?page_size=100&name=v1.36
HTTP/2 200
{"count":0,"next":null,"previous":null,"results":[]}

Phase 53 sub-plans 53-01 through 53-07 proceed normally; SYNC-05 does NOT block addon work.
```

### Default image constant (current)
```go
// Source: pkg/apis/config/defaults/image.go:21
const Image = "kindest/node:v1.35.1@sha256:05d7bcdefbda08b4e038f644c4df690cdac3fba8b06f8289f30e10026720a1ab"
// 53-00 Outcome A: bumped to kindest/node:v1.36.x@sha256:<digest> (TDD RED→GREEN per plan 51-04 pattern)
// 53-00 Outcome B: unchanged
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact on This Phase |
|--------------|------------------|--------------|----------------------|
| Manual kubectl apply per addon | Embedded YAML + go:embed + apply pipe | v1.0 (Phase 1-8) | Established pattern; do not deviate |
| Client-side kubectl apply | `--server-side` for >256 KB | v1.3 (Phase 23 cert-manager) | Locked decision; do not regress |
| HAProxy LB | Envoy LB | v2.3 (Phase 51) | LB image (`envoyproxy/envoy:v1.36.2`) is in offlinereadiness.go; unrelated to this phase but listed in `allAddonImages` |
| Per-addon offlinereadiness updates | Consolidated 53-07 step | v2.4 (this phase) | Locked decision in CONTEXT — no per-plan offlinereadiness edits |
| Kubernetes 1.35 default | Kubernetes 1.36 (conditional) | v2.4 (this phase) | SYNC-05 — gated on Docker Hub probe |
| cert-manager UID 1000 | cert-manager UID 65532 | v1.20.0 (upstream) | Disclose in CHANGELOG + addon docs `:::caution` |
| cert-manager rotationPolicy: Never default | rotationPolicy: Always (mandatory) | v1.20.0 (upstream, GA) | Disclose in CHANGELOG + addon docs `:::caution` |
| Envoy Gateway Gateway API v1.2.1 | Gateway API v1.4.1 | v1.7.2 (upstream EG) | Bundled in install.yaml; no separate work |

**Deprecated/outdated for this phase:**
- The PITFALLS item 8 recommendation to staged-bump EG (1.3→1.5→1.7) is OVERRIDDEN by user CONTEXT decision. Single-jump with hold criteria is the new path.
- Two-phase fallback is documented as deferred in CONTEXT — not active.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `Images` slice pattern in each `installX/X.go` will not require structural changes for any of the four bumps | Standard Stack | LOW — verified all four upstream manifests; only image tags change |
| A2 | The cert-manager manifest size of 992 KB will not shrink below 256 KB before the planner executes | Pitfall CERT-01 | NEGLIGIBLE — this would be an upstream regression, not a forward concern |
| A3 | `ghcr.io/headlamp-k8s/headlamp:v0.42.0` will remain pullable when the executor runs (image is published per the GitHub container packages page) | Standard Stack — Headlamp | LOW — repo organization moved (`headlamp-k8s` → `kubernetes-sigs`) but image path stayed |
| A4 | kind v0.32.0 may publish kindest/node:v1.36.x between research date (2026-05-10) and execute date | SYNC-05 | LOW — if it publishes, SYNC-05 fires Outcome A; CONTEXT covers both branches |
| A5 | Existing `realInspectImage` helper in offlinereadiness.go correctly handles the new image tags | Don't Hand-Roll | NEGLIGIBLE — uses `<runtime> inspect --type=image <ref>`; ref-format-agnostic |

**No `[ASSUMED]` claims appear in user-facing recommendations.** All version numbers, manifest sizes, and job names are verified above.

## Open Questions

1. **Should hold-verify plans (53-05, 53-06) include a fresh-cluster smoke or just a version-grep check?**
   - What we know: CONTEXT explicitly leaves this to planner discretion ("upstream version check only" vs "re-validate functionality on a fresh cluster").
   - What's unclear: Whether existing test coverage for MetalLB and Metrics Server is sufficient to skip a new smoke.
   - Recommendation: Planner picks "upstream version check only" — `curl -s https://api.github.com/repos/.../releases/latest | jq .tag_name` verifies still-latest. If returned tag matches the pinned version, no source change. If a newer version exists, that's a deviation requiring a new bump plan or an updated hold rationale. Live cluster smoke adds time without new signal — these addons haven't changed.

2. **Does cert-manager v1.20.2 require a longer wait timeout for cert-manager-webhook deployment?**
   - What we know: Current timeout is 300s; cert-manager v1.20.0 release notes do not mention startup-time changes.
   - What's unclear: Whether new validation logic in v1.20 webhook adds startup latency.
   - Recommendation: Plan 53-03 keeps the 300s timeout. If smoke fails on first run, the planner increases it. Don't pre-emptively double it.

3. **Should the K8s 1.36 IPVS guard test cases be expanded once SYNC-05 fires?**
   - What we know: `validate_test.go` already covers `kindest/node:v1.36.0` IPVS rejection.
   - What's unclear: Whether bumping the default constant to v1.36.x triggers a default-path test gap.
   - Recommendation: If 53-00 is Outcome A, the planner's GREEN commit should add `TestDefaultImageRejectsIPVS` covering "default image with IPVS proxy rejects at validate" — quick assertion, prevents regression. If Outcome B, no test needed.

4. **Where exactly do the per-plan CHANGELOG stubs live before 53-07 consolidation?**
   - What we know: CONTEXT locks "stub atomic + final consolidate."
   - What's unclear: Whether the stub goes in `kinder-site/src/content/docs/changelog.md` directly (and gets reformatted in 53-07) or in a holding scratch file.
   - Recommendation: Stub goes directly in `changelog.md` under an `## Unreleased` (or `## v2.4 — Hardening (in progress)`) H2 — line item per addon plan. 53-07 reformats into final v2.4 section structure (matching v2.3 format). No scratch file. Atomic = "the bump commit and its CHANGELOG line ship together."

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `curl` | SYNC-05 probe (53-00); upstream-version checks (53-05, 53-06) | ✓ (assumed; macOS default) | n/a | None — required |
| `jq` | SYNC-05 probe parsing | ✓ (project assumes; used in plan 51-04) | n/a | `python3 -c "import json"` if jq missing |
| `docker` | (Optional) Pull-and-inspect alternative for Step 2 of SYNC-05 probe | likely ✓ | n/a | Use registry-1.docker.io HTTP API directly (the curl path) |
| `go 1.24+` | Compile + test the bumped manifests | ✓ (project standard) | per `go.mod` | None |
| Live K8s cluster (optional) | If planner chooses fresh-cluster smoke for hold-verify or for cert-manager UID check | varies | n/a | Unit tests already cover the action logic; live smoke is enhancement only |
| kind v0.32.0 (`kindest/node:v1.36.x`) | SYNC-05 Outcome A | ✗ (probe today: count=0) | n/a | Outcome B path: write INCONCLUSIVE summary, defer SYNC-05 |

**Missing dependencies with no fallback:** None blocking. The only "missing" is `kindest/node:v1.36.x`, which IS the gate SYNC-05 is designed to handle — its absence is not a blocker, it's an Outcome B signal.

**Missing dependencies with fallback:** `jq` → python3; `docker pull` for Step 2 → curl manifest API.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `t.Parallel()` |
| Config file | none (Go convention) |
| Quick run command | `go test ./pkg/cluster/internal/create/actions/installX/... -run TestExecute -v` (per addon) |
| Full suite command | `go test ./pkg/cluster/internal/create/actions/... ./pkg/internal/doctor/... -race` |
| Phase gate | Full suite green before `/gsd-verify-work` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ADDON-01 | local-path-provisioner v0.0.36 image pinned in `Images` slice | unit | `go test ./pkg/cluster/internal/create/actions/installlocalpath/... -run TestImagesPinsV0036 -v` | ❌ Wave 0 (RED commit creates) |
| ADDON-01 | busybox:1.37.0 still pinned in vendored manifest | unit | `go test ./pkg/cluster/internal/create/actions/installlocalpath/... -run TestManifestPinsBusybox -v` | ❌ Wave 0 |
| ADDON-01 | local-path StorageClass has is-default-class annotation | unit | `go test ./pkg/cluster/internal/create/actions/installlocalpath/... -run TestStorageClassIsDefault -v` | ❌ Wave 0 |
| ADDON-02 | Headlamp v0.42.0 image pinned in `Images` slice | unit | `go test ./pkg/cluster/internal/create/actions/installdashboard/... -run TestImagesPinsHeadlampV042 -v` | ❌ Wave 0 |
| ADDON-02 | Token-print flow still works (FakeCmd queue test) | unit | `go test ./pkg/cluster/internal/create/actions/installdashboard/... -run TestExecute -v` | ✅ exists at `dashboard_test.go` |
| ADDON-02 | Live token smoke (`kubectl auth can-i` + curl) | manual | one-shot during plan; document in 53-02 SUMMARY | manual-only (CONTEXT-locked) |
| ADDON-03 | cert-manager v1.20.2 images pinned (3 of them) | unit | `go test ./pkg/cluster/internal/create/actions/installcertmanager/... -run TestImagesPinsV1202 -v` | ❌ Wave 0 |
| ADDON-03 | `--server-side` flag preserved in apply call | unit | `go test ./pkg/cluster/internal/create/actions/installcertmanager/... -run TestExecuteUsesServerSideApply -v` | ❌ Wave 0 (extension of existing TestExecute) |
| ADDON-03 | Live UID 65532 verification on running pod | manual | `kubectl get pod ... -o jsonpath` in 53-03 SUMMARY | manual-only (CONTEXT-locked) |
| ADDON-04 | EG v1.7.2 + ratelimit:05c08d03 images pinned | unit | `go test ./pkg/cluster/internal/create/actions/installenvoygw/... -run TestImagesPinsEGV172 -v` | ❌ Wave 0 |
| ADDON-04 | certgen Job name still `eg-gateway-helm-certgen` in embedded YAML | unit | `go test ./pkg/cluster/internal/create/actions/installenvoygw/... -run TestManifestContainsCertgenJobName -v` | ❌ Wave 0 |
| ADDON-04 | Bundled Gateway API CRD bundle-version is v1.4.1 | unit | `go test ./pkg/cluster/internal/create/actions/installenvoygw/... -run TestManifestPinsGatewayAPIBundleV141 -v` | ❌ Wave 0 |
| ADDON-04 | Live HTTPRoute end-to-end smoke | manual | curl through gateway in 53-04 SUMMARY | manual-only (CONTEXT-locked) |
| ADDON-05 | offlinereadiness `allAddonImages` reflects all bumps; count=14 | unit | `go test ./pkg/internal/doctor/... -run TestAllAddonImages` | ✅ exists at `offlinereadiness_test.go:118` |
| SYNC-05 | (if Outcome A) default image pinned to v1.36.x with sha256 | unit | `go test ./pkg/apis/config/defaults/... -run TestDefaultImageIsKubernetes136 -v` | ❌ Wave 0 (created by plan 51-04 pattern in 53-00 GREEN) |

### Sampling Rate

- **Per task commit:** `go test ./pkg/cluster/internal/create/actions/installX/... -race` (the addon under edit)
- **Per wave merge (= per sub-plan close):** `go test ./pkg/cluster/internal/create/actions/... ./pkg/internal/doctor/... -race`
- **Phase gate (before 53-07 merge):** Full suite + `go build ./...` + a clean `go vet ./...`

### Wave 0 Gaps

- [ ] `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go` — extend with `TestImagesPinsV0036`, `TestManifestPinsBusybox`, `TestStorageClassIsDefault`
- [ ] `pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go` — extend with `TestImagesPinsHeadlampV042`
- [ ] `pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go` — extend with `TestImagesPinsV1202`, `TestExecuteUsesServerSideApply` (the latter greps captured FakeCmd args for `--server-side`)
- [ ] `pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go` — extend with `TestImagesPinsEGV172`, `TestManifestContainsCertgenJobName`, `TestManifestPinsGatewayAPIBundleV141`
- [ ] `pkg/apis/config/defaults/image_test.go` — created in plan 53-00 GREEN if Outcome A, with `TestDefaultImageIsKubernetes136` per plan 51-04 RED template

*(All gaps are RED-commit responsibilities of each plan, not pre-Wave-0 work — they ship in the same commit-pair as their GREEN counterparts.)*

## Sources

### Primary (HIGH confidence)
- **Existing codebase (verified in this session):**
  - `pkg/internal/doctor/offlinereadiness.go` lines 33-73 (`allAddonImages` slice, comment header "Total: 14")
  - `pkg/internal/doctor/offlinereadiness_test.go` line 122 (`const expected = 14` in `TestAllAddonImages_CountMatchesExpected`)
  - `pkg/apis/config/defaults/image.go` line 21 (current default image constant)
  - `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go` (Images slice, action structure)
  - `pkg/cluster/internal/create/actions/installdashboard/dashboard.go` (token-print flow)
  - `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go` (--server-side apply)
  - `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` (certgen wait pattern)
  - `pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml` (vendored manifest shape)
  - `pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` (busybox pin + is-default-class annotation)
- **Past phase plans (read in this session):**
  - `.planning/phases/51-upstream-sync-k8s-1-36/51-04-default-node-image-bump-PLAN.md` (canonical SYNC probe pattern)
  - `.planning/phases/51-upstream-sync-k8s-1-36/51-04-SUMMARY.md` (INCONCLUSIVE template)
- **Upstream artifacts downloaded and verified in this session:**
  - `https://github.com/envoyproxy/gateway/releases/download/v1.7.2/install.yaml` (3.3 MB; certgen Job name + bundled CRD version verified)
  - `https://github.com/cert-manager/cert-manager/releases/download/v1.20.2/cert-manager.yaml` (992 KB; --server-side requirement confirmed; 3 unique images)
  - `https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.36/deploy/local-path-storage.yaml` (verified: upstream uses unpinned busybox, no default-class annotation — both kinder overrides MUST be re-applied)
- **Docker Hub probe executed in this session:**
  - `https://hub.docker.com/v2/repositories/kindest/node/tags/?page_size=100&name=v1.36` returned `count=0` (HTTP 200, authoritative)

### Secondary (MEDIUM confidence)
- **GitHub release notes (web-fetched in this session):**
  - cert-manager v1.20.0 release page — confirmed UID 1000→65532 + rotationPolicy: Always default change (was beta in v1.18, GA in v1.20)
  - Headlamp v0.42.0 release page — no breaking auth changes
  - local-path-provisioner v0.0.36 — confirmed GHSA-7fxv-8wr2-mfc4 fix
  - GitHub container package page for `headlamp-k8s/headlamp` — v0.42.0 published "3 days ago"
  - MetalLB releases — v0.15.3 (2024-12-04) is still latest
  - metrics-server releases — v0.8.1 (2026-01-29) is still latest
- **Existing PITFALLS.md research:**
  - Items 5–12 (per-addon hazards) were re-read and are referenced inline in the Pitfalls section above

### Tertiary (LOW confidence)
- None. All claims in this RESEARCH.md are tied to either an in-repo file, a verified upstream artifact, or a verified web source.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every addon version verified against upstream; existing patterns are stable
- Architecture: HIGH — pattern is reused (1 addon = 1 manifest YAML + 1 Images slice + 1 wait pattern); plan 51 sequence is the template
- Pitfalls: HIGH — research/PITFALLS.md items 6–12 cover the hazards; researcher verified the EG certgen job name and Gateway API bundle version directly from upstream
- SYNC-05 outcome: HIGH for Outcome B (probe today is count=0, authoritative); LOW for Outcome A (depends on kind v0.32.0 publishing)
- Hold-verify (53-05/53-06): MEDIUM — depends on planner's discretion choice; both sub-options are documented

**Research date:** 2026-05-10
**Valid until:** 2026-05-24 (14 days for fast-moving addon ecosystem; re-verify versions before merging plan if execute date slips past this)
