# Roadmap: Kinder

## Milestones

- ✅ **v1.0 Batteries Included** - Phases 1-8 (shipped 2026-03-01)
- ✅ **v1.1 Kinder Website** - Phases 9-14 (shipped 2026-03-02)
- ✅ **v1.2 Branding & Polish** - Phases 15-18 (shipped 2026-03-02)
- 🚧 **v1.3 Harden & Extend** - Phases 19-24 (in progress)

---

<details>
<summary>✅ v1.0 Batteries Included (Phases 1-8) - SHIPPED 2026-03-01</summary>

Phases 1-8 delivered the forked kinder binary with 5 default addons (MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, Headlamp) all individually disableable via config. 65 files, ~1,950 LOC Go.

</details>

<details>
<summary>✅ v1.1 Kinder Website (Phases 9-14) - SHIPPED 2026-03-02</summary>

Phases 9-14 delivered the Astro/Starlight documentation site at kinder.patrykgolabek.dev with dark terminal aesthetic, landing page, 10 documentation pages, and GitHub Pages deployment. 878 LOC.

</details>

<details>
<summary>✅ v1.2 Branding & Polish (Phases 15-18) - SHIPPED 2026-03-02</summary>

Phases 15-18 delivered kinder logo (SVG/PNG/favicon/OG), SEO (llms.txt, JSON-LD, meta tags), README rewrite, and dark-only theme enforcement.

</details>

---

### 🚧 v1.3 Harden & Extend (In Progress)

**Milestone Goal:** Fix critical bugs, reduce provider code duplication, and add local registry, cert-manager addons and CLI diagnostic tools.

## Phases

- [ ] **Phase 19: Bug Fixes** - Fix four correctness bugs across all providers before any new work
- [ ] **Phase 20: Provider Code Deduplication** - Extract shared docker/podman/nerdctl code to common/ package
- [ ] **Phase 21: Config Type Additions** - Add LocalRegistry and CertManager fields to v1alpha4 and wire through config pipeline
- [ ] **Phase 22: Local Registry Addon** - Batteries-included local registry at localhost:5001 on the kind network
- [ ] **Phase 23: cert-manager Addon** - Embedded cert-manager with self-signed ClusterIssuer ready immediately
- [ ] **Phase 24: CLI Commands** - kinder env and kinder doctor diagnostic commands

## Phase Details

### Phase 19: Bug Fixes
**Goal**: All four correctness bugs are eliminated across every provider before any refactoring or new feature work begins
**Depends on**: Nothing (first v1.3 phase)
**Requirements**: BUG-01, BUG-02, BUG-03, BUG-04
**Success Criteria** (what must be TRUE):
  1. Port listeners acquired in generatePortMappings are released at iteration end, not at function return — no port leak under early exit
  2. Tar extraction returns an error on truncated files instead of silently stopping mid-archive
  3. ListInternalNodes resolves an empty cluster name to the default cluster name consistently across all three providers
  4. Network sort comparator satisfies strict weak ordering — identical networks compare equal, sort is deterministic
**Plans**: TBD

### Phase 20: Provider Code Deduplication
**Goal**: Shared docker/podman/nerdctl logic lives in one common/ package; per-provider files are deleted; all three providers compile and pass their test suites unchanged
**Depends on**: Phase 19
**Requirements**: PROV-01, PROV-02, PROV-03
**Success Criteria** (what must be TRUE):
  1. common/node.go exists with a shared Node struct parameterized by binaryName; docker/node.go, nerdctl/node.go, and podman/node.go are deleted
  2. common/provision.go exists with shared generateMountBindings, generatePortMappings, and createContainer; docker/provision.go and nerdctl/provision.go are deleted
  3. go.mod minimum directive is go 1.21.0 with toolchain go1.26.0 and the build passes with no new external dependencies
  4. go build ./... and go test ./... both pass identically before and after extraction — no behavior change across any provider
**Plans**: TBD

### Phase 21: Config Type Additions
**Goal**: LocalRegistry and CertManager addon fields exist in the public v1alpha4 API and are wired through all five config pipeline locations so action packages can reference them at compile time
**Depends on**: Phase 20
**Requirements**: CFG-01, CFG-02, CFG-03
**Success Criteria** (what must be TRUE):
  1. v1alpha4 Addons struct has LocalRegistry *bool and CertManager *bool fields, both defaulting to true when nil
  2. Internal config types, conversion, and defaults all reflect the new fields — the five-location pipeline is complete
  3. A cluster config with addons.localRegistry: false and addons.certManager: false parses and validates without error
**Plans**: TBD

### Phase 22: Local Registry Addon
**Goal**: Users get a working local container registry at localhost:5001 by default, accessible from all cluster nodes, with dev tool discovery support — all without any manual setup
**Depends on**: Phase 21
**Requirements**: REG-01, REG-02, REG-03, REG-04
**Success Criteria** (what must be TRUE):
  1. A registry:2 container named kind-registry is created on the kind network during cluster creation and survives cluster restarts
  2. docker push localhost:5001/myimage:latest succeeds from the host and the image is pullable from inside a pod on any cluster node
  3. The kube-public/local-registry-hosting ConfigMap exists in the cluster and contains the correct registry endpoint for Tilt, Skaffold, and other dev tools
  4. Setting addons.localRegistry: false in cluster config skips registry creation entirely — kinder create cluster completes without the registry container
**Plans**: TBD

### Phase 23: cert-manager Addon
**Goal**: Users get a fully ready cert-manager installation with a self-signed ClusterIssuer so Certificate resources work immediately after cluster creation — no manual cert-manager setup required
**Depends on**: Phase 22
**Requirements**: CERT-01, CERT-02, CERT-03, CERT-04
**Success Criteria** (what must be TRUE):
  1. The embedded cert-manager v1.17.6 manifest is applied during cluster creation and all three components (cert-manager, cert-manager-webhook, cert-manager-cainjector) reach Available status before the cluster is reported ready
  2. A self-signed ClusterIssuer named selfsigned-issuer exists in the cluster immediately after creation — kubectl apply of a Certificate resource referencing it succeeds without error
  3. Setting addons.certManager: false in cluster config skips cert-manager installation entirely — cluster creation is not delayed by webhook readiness waits
**Plans**: TBD

### Phase 24: CLI Commands
**Goal**: Users have two diagnostic commands — kinder env for machine-readable cluster environment info and kinder doctor for prerequisite checking — both following the existing Cobra command patterns
**Depends on**: Phase 21
**Requirements**: CLI-01, CLI-02, CLI-03, CLI-04
**Success Criteria** (what must be TRUE):
  1. kinder env prints the active provider, cluster name, and kubeconfig path in key=value format to stdout — eval $(kinder env) succeeds in bash
  2. kinder env writes all warnings and errors to stderr only, leaving stdout clean for machine consumption
  3. kinder doctor checks binary prerequisites (docker/podman/nerdctl, kubectl) and prints an actionable fix message for each missing or misconfigured binary
  4. kinder doctor exits 0 when all checks pass, exits 1 on any hard failure, and exits 2 on warnings — scripts can distinguish warning from failure
**Plans**: TBD

## Progress

**Execution Order:** 19 → 20 → 21 → 22 → 23 → 24 (Phase 24 depends only on Phase 21, can run after Phase 21)

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8. Batteries Included | v1.0 | 12/12 | Complete | 2026-03-01 |
| 9-14. Kinder Website | v1.1 | 8/8 | Complete | 2026-03-02 |
| 15-18. Branding & Polish | v1.2 | 4/4 | Complete | 2026-03-02 |
| 19. Bug Fixes | v1.3 | 0/TBD | Not started | - |
| 20. Provider Code Deduplication | v1.3 | 0/TBD | Not started | - |
| 21. Config Type Additions | v1.3 | 0/TBD | Not started | - |
| 22. Local Registry Addon | v1.3 | 0/TBD | Not started | - |
| 23. cert-manager Addon | v1.3 | 0/TBD | Not started | - |
| 24. CLI Commands | v1.3 | 0/TBD | Not started | - |

---
*Roadmap created: 2026-03-03 — v1.3 Harden & Extend roadmap added*
*Previous milestones: see archived phase details above*
