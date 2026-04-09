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
- 🚧 **v2.2 Cluster Capabilities** - Phases 42-46 (in progress)

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

### 🚧 v2.2 Cluster Capabilities (In Progress)

**Milestone Goal:** Add four cluster capabilities — multi-version per-node Kubernetes, offline/air-gapped cluster creation, local-path-provisioner dynamic storage, host-directory mounting — plus a `kinder load images` utility that ties the offline and multi-version workflows together. All features integrate using packages already in `go.mod` with zero new dependencies.

**Phase Numbering Note:** Decimal phases (e.g., 42.1) indicate urgent insertions created via `/gsd-insert-phase`.

## Phase Details

### Phase 42: Multi-Version Node Validation
**Goal**: Users can configure per-node Kubernetes versions and kinder validates version-skew correctness at config parse time instead of surfacing cryptic kubeadm errors after provisioning begins
**Depends on**: Phase 41
**Requirements**: MVER-01, MVER-02, MVER-03, MVER-04, MVER-05
**Success Criteria** (what must be TRUE):
  1. Running `kinder create cluster` with a per-node `image:` field and a global `--image` flag preserves the per-node image (the `--image` flag no longer overrides explicit node images)
  2. A config with workers more than 3 minor versions behind the control-plane is rejected before any containers are created, with an error message stating the violating node and version delta
  3. A config with HA control-plane nodes at different versions is rejected at config validation time with a clear explanation
  4. `kinder doctor` run against a running multi-version cluster reports a warning when version-skew policy is violated
  5. `kinder get nodes` output includes a column showing the Kubernetes version installed on each node
**Plans:** 2/2 plans complete
Plans:
- [x] 42-01-PLAN.md — ExplicitImage sentinel, fixupOptions fix, version-skew config validation
- [x] 42-02-PLAN.md — Doctor cluster-skew check, get nodes VERSION/IMAGE/SKEW columns

### Phase 43: Air-Gapped Cluster Creation
**Goal**: Users can create a fully functional cluster without internet access by passing `--air-gapped`, and kinder fails immediately with a complete list of missing images rather than hanging on failed pulls
**Depends on**: Phase 42
**Requirements**: AIRGAP-01, AIRGAP-02, AIRGAP-03, AIRGAP-04, AIRGAP-05, AIRGAP-06
**Success Criteria** (what must be TRUE):
  1. `kinder create cluster --air-gapped` on a machine with pre-loaded images creates the cluster without any network calls for image pulls across all three providers (docker, podman, nerdctl)
  2. `kinder create cluster --air-gapped` on a machine with missing images exits immediately with a human-readable list of every image that must be pre-loaded, instead of timing out or producing Docker pull errors
  3. Running `kinder create cluster` (without `--air-gapped`) prints a warning listing all addon images that will be pulled, so users know what to pre-load before switching to offline mode
  4. `kinder doctor` run before cluster creation lists which required images are absent from the local image store, serving as a pre-flight offline readiness check
  5. The two-mode offline workflow (pre-create image baking via privileged container commit vs. post-create load via `kinder load images`) is documented and reachable from the website
**Plans:** 3/3 plans complete
Plans:
- [x] 43-01-PLAN.md — AirGapped flag plumbing, addon image constants, RequiredAddonImages utility
- [x] 43-02-PLAN.md — Provider air-gapped fast-fail, non-air-gapped addon image warning
- [x] 43-03-PLAN.md — Doctor offline-readiness check, working-offline.md documentation
**UI hint**: yes

### Phase 44: Local-Path-Provisioner Addon
**Goal**: Users get automatic dynamic PVC provisioning out of the box via local-path-provisioner as a default addon, with `local-path` as the only default StorageClass and no StorageClass collision
**Depends on**: Phase 43
**Requirements**: STOR-01, STOR-02, STOR-03, STOR-04, STOR-05, STOR-06
**Success Criteria** (what must be TRUE):
  1. Running `kinder create cluster` installs local-path-provisioner v0.0.35 and `local-path` is the only default StorageClass (the legacy `standard` StorageClass from `installstorage` is absent)
  2. A PVC with `storageClassName: local-path` (or no storageClassName) transitions to `Bound` automatically in both single-node and multi-node clusters without any manual operator action
  3. Setting `addons.localPath: false` in the cluster config skips the addon, and `installstorage` installs the legacy `standard` StorageClass instead (exact same behavior as pre-v2.2)
  4. `kinder doctor` on a running cluster warns when local-path-provisioner is below v0.0.34 (CVE-2025-62878 threshold)
  5. The embedded manifest uses `busybox:1.37.0` with `imagePullPolicy: IfNotPresent`, ensuring PVC operations work correctly in air-gapped clusters where `busybox:latest` cannot be pulled
**Plans**:
- [x] 44-01-PLAN.md — LocalPath config pipeline (5-location), installlocalpath action package, installstorage gate, wave1 registration, RequiredAddonImages registration
- [x] 44-02-PLAN.md — Unit tests (FakeNode), offlinereadiness +2 images (total 14), images_test LocalPath coverage
- [x] 44-03-PLAN.md — Doctor CVE-2025-62878 check: warns when local-path-provisioner < v0.0.34

### Phase 45: Host-Directory Mounting
**Goal**: Users can mount host directories into cluster nodes with clear pre-flight validation, explicit platform warnings, and documented guidance for wiring mounts through to pods via hostPath PVs
**Depends on**: Phase 44
**Requirements**: MOUNT-01, MOUNT-02, MOUNT-03, MOUNT-04
**Success Criteria** (what must be TRUE):
  1. Running `kinder create cluster` with an `extraMounts` entry pointing to a non-existent host path exits before any containers are created, with an error message identifying the missing path
  2. Specifying `propagation: HostToContainer` or `propagation: Bidirectional` on macOS or Windows emits a visible warning during cluster creation explaining that propagation is unsupported on Docker Desktop and defaults to `None`
  3. `kinder doctor` on macOS checks that a configured host mount path exists and that Docker Desktop file sharing is enabled for that path, reporting actionable guidance when either check fails
  4. The website documents the two-hop mount pattern (host directory → node extraMount → pod hostPath PV) with a complete example YAML showing host dir mounted as a PV-backed volume
**Plans**: TBD
**UI hint**: yes

### Phase 46: kinder load images Command
**Goal**: Users can load one or more local images into all nodes of a running cluster with a single command that works across all three providers and handles Docker Desktop 27+ containerd image store compatibility
**Depends on**: Phase 43
**Requirements**: LOAD-01, LOAD-02, LOAD-03, LOAD-04
**Success Criteria** (what must be TRUE):
  1. `kinder load images <image> [<image>...]` loads the specified images into every node of the target cluster using the correct provider's image save mechanism (not hardcoded to `docker save`)
  2. The command works identically with docker, podman, and nerdctl providers, using each provider's native image export
  3. On Docker Desktop 27+ with the containerd image store enabled, `kinder load images` successfully imports multi-platform images without the `content digest: not found` error (fallback strategy applied automatically)
  4. Re-running `kinder load images` with an image already present on all nodes completes without re-importing, reporting that the image was skipped as already present
**Plans**: TBD

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
| 42. Multi-Version Node Validation | v2.2 | 2/2 | Complete   | 2026-04-08 |
| 43. Air-Gapped Cluster Creation | v2.2 | 3/3 | Complete   | 2026-04-09 |
| 44. Local-Path-Provisioner Addon | v2.2 | 3/3 | Complete   | 2026-04-09 |
| 45. Host-Directory Mounting | v2.2 | 0/TBD | Not started | - |
| 46. kinder load images Command | v2.2 | 0/TBD | Not started | - |
