# Phase 53: Addon Version Audit, Bumps & SYNC-05 - Context

**Gathered:** 2026-05-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Bump 4 addons to versions locked by REQUIREMENTS.md (local-path-provisioner v0.0.36, Headlamp v0.42.0, cert-manager v1.20.2, Envoy Gateway v1.7.2), verify 2 holds (MetalLB v0.15.3, Metrics Server v0.8.1), conditionally bump default node image to K8s 1.36 (SYNC-05), and consolidate `pkg/internal/doctor/offlinereadiness.go` to reflect new image references.

Sub-plan order is locked sequential per ROADMAP.md cross-phase concerns: 53-00 (SYNC-05 probe) → 53-01 (local-path) → 53-02 (Headlamp) → 53-03 (cert-manager) → 53-04 (Envoy Gateway) → 53-05 (MetalLB hold) → 53-06 (Metrics Server hold) → 53-07 (offlinereadiness consolidation). Parallel execution explicitly forbidden — ambiguous cross-bump failures are undiagnosable.

</domain>

<decisions>
## Implementation Decisions

### UAT depth per addon

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
  - Verify cert-manager pods run as UID 65632 (the upstream default change)
- **Envoy Gateway v1.7.2** — Install + HTTPRoute traffic (roadmap-mandated):
  - Cluster create, EG pods Ready, Gateway resource accepted (status: Programmed)
  - Create HTTPRoute, send curl traffic end-to-end through the gateway, verify 200

### Hold/abort criteria (per addon)

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

### EG bump strategy (53-04)

- **Single jump v1.3.1 → v1.7.2** — One commit, one HTTPRoute smoke test. (User overrode the "two-phase 1.3→1.5→1.7" recommendation from Cross-Phase Concerns.) Hold criteria above are the safety net.
- **Gateway API CRD pinning** — Match EG's bundled CRDs (whatever EG v1.7.2 install.yaml ships). Trust upstream. No separate explicit pin in addon source.
- **`eg-gateway-helm-certgen` job name verification** — Pre-bump in research phase. The phase researcher pulls v1.7.2 install.yaml, greps for the job name, and reports BEFORE the planner writes 53-04. Avoids surprises during execution.

### SYNC-05 (53-00)

- **If probe confirms publication** (`kindest/node:v1.36.x` exists on Docker Hub with valid manifest digest):
  - Bump `pkg/apis/config/defaults/image.go` constant
  - Update CI integration test matrix to include 1.36 (or replace the floor)
  - Update website examples + CLI reference + tutorials to reference K8s 1.36 where "latest" is implied
- **If probe is INCONCLUSIVE** (image not yet published):
  - 53-00 records INCONCLUSIVE status, no source change for default image
  - Addons (53-01 through 53-07) all proceed normally
  - SYNC-05 deferred to v2.5 or re-run later — re-runnable status preserved

### Breaking-change disclosure

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

</decisions>

<specifics>
## Specific Ideas

- Headlamp UAT must include BOTH `kubectl auth can-i` AND a `curl` against the UI — "token flow" means both layers, not just RBAC validity.
- EG single-jump is the user's call despite Cross-Phase Concerns' recommendation; the hold criteria (HTTPRoute fail, job rename, CRD breaking change) are the explicit safety net that compensates.
- `eg-gateway-helm-certgen` job name verification belongs in the research phase, not planning or execution — surface unknowns as early as possible.
- SYNC-05 INCONCLUSIVE must NOT block addon work — addons ship even without the default image bump.

</specifics>

<deferred>
## Deferred Ideas

- Hold-verification depth for the two holds (MetalLB v0.15.3, Metrics Server v0.8.1) — left to planner discretion this phase; could be revisited if a hold-verify regression surfaces.
- Two-phase EG bump (v1.3 → v1.5 → v1.7) as a fallback if single-jump smoke fails — captured here so the planner knows it's a documented retreat path, not a new capability.

</deferred>

---

*Phase: 53-addon-version-audit-bumps-sync-05*
*Context gathered: 2026-05-10*
