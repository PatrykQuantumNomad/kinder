# Requirements: Kinder v2.2 Cluster Capabilities

**Defined:** 2026-04-08
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## v2.2 Requirements

Requirements for v2.2 Cluster Capabilities. Each maps to roadmap phases.

### Multi-Version Clusters

- [x] **MVER-01**: `--image` flag only overrides nodes without an explicit image in config (fix fixupOptions bug)
- [x] **MVER-02**: Config validation rejects invalid version-skew combinations before provisioning (control-plane same version, workers within 3-minor of control-plane)
- [x] **MVER-03**: Clear error messages surface version-skew violations at config parse time instead of cryptic kubeadm join failures
- [ ] **MVER-04**: `kinder doctor` detects version-skew issues in running clusters
- [ ] **MVER-05**: `kinder get nodes` output includes per-node Kubernetes version column

### Air-Gapped Clusters

- [ ] **AIRGAP-01**: `--air-gapped` flag on `kinder create cluster` enables offline mode
- [ ] **AIRGAP-02**: Air-gapped mode fails fast with clear error listing missing images instead of retrying pulls
- [ ] **AIRGAP-03**: All three providers (docker/podman/nerdctl) support air-gapped mode via ProvisionOptions
- [ ] **AIRGAP-04**: Addon image warning output lists all required images that need pre-loading before creation
- [ ] **AIRGAP-05**: `kinder doctor` offline readiness check lists which required images are missing
- [ ] **AIRGAP-06**: Two-mode design documented (pre-create bake via privileged container commit vs. post-create load)

### Local Storage (local-path-provisioner)

- [ ] **STOR-01**: local-path-provisioner v0.0.35 installed as a default addon via `installlocalpath` action
- [ ] **STOR-02**: `local-path` is the default StorageClass; `installstorage` gated out when LocalPath enabled
- [ ] **STOR-03**: Embedded manifest patches busybox to `busybox:1.37.0` with `imagePullPolicy: IfNotPresent`
- [ ] **STOR-04**: `Addons.LocalPath *bool` field added to v1alpha4 config (full 5-location pipeline)
- [ ] **STOR-05**: `kinder doctor` CVE-2025-62878 check warns if running cluster has vulnerable local-path-provisioner
- [ ] **STOR-06**: PVCs dynamically provision and bind automatically in single and multi-node clusters

### Host Directory Mounting

- [ ] **MOUNT-01**: Pre-flight validation checks host path existence before node provisioning with clear error message
- [ ] **MOUNT-02**: Platform warning emitted when non-None propagation mode specified on macOS/Windows
- [ ] **MOUNT-03**: `kinder doctor` checks host path exists and Docker Desktop file sharing is configured (macOS)
- [ ] **MOUNT-04**: Documentation covers two-hop mount pattern (host → node extraMount → pod hostPath PV)

### Load Images Command

- [ ] **LOAD-01**: `kinder load images` subcommand loads one or more images into all cluster nodes
- [ ] **LOAD-02**: Provider-abstracted image saving (works with docker/podman/nerdctl, not hardcoded to docker save)
- [ ] **LOAD-03**: Fallback import strategy for Docker Desktop 27+ containerd image store compatibility
- [ ] **LOAD-04**: Smart-load skips images already present on nodes

## v2.3+ Requirements

Deferred to future release. Tracked but not in current roadmap.

### Auto-PV from ExtraMounts

- **MOUNT-05**: `Mount.CreatePV bool` field auto-creates PersistentVolume from extraMount paths
- **MOUNT-06**: `Mount.PVName string` and `Mount.PVCapacity string` configure auto-created PVs

### Offline Presets

- **AIRGAP-07**: `--profile offline` preset combines air-gapped flag with pre-baked image config
- **AIRGAP-08**: `kinder build bake-images` command pre-bakes addon images into custom node image

### Multi-Version Presets

- **MVER-06**: `--profile upgrade-test` preset creates a mixed-version cluster for upgrade testing

## Out of Scope

| Feature | Reason |
|---------|--------|
| Bundling addon images into kinder binary | Binary would become multi-GB; breaks Homebrew distribution |
| Auto-detecting internet availability | Unreliable on VPNs/proxies; causes confusing silent behavior |
| Auto-rolling upgrades via multi-version clusters | Out of scope; kubeadm covers this already |
| HA control-plane version skew > 1 minor | kubeadm refuses it; kinder must validate and reject |
| OCI Go libraries for image operations | All image operations go through Docker/containerd CLI (established pattern) |
| Auto-PV from extraMounts (v2.2) | Type system complexity for limited gain; defer to v2.3 |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| MVER-01 | Phase 42 | Complete |
| MVER-02 | Phase 42 | Complete |
| MVER-03 | Phase 42 | Complete |
| MVER-04 | Phase 42 | Pending |
| MVER-05 | Phase 42 | Pending |
| AIRGAP-01 | Phase 43 | Pending |
| AIRGAP-02 | Phase 43 | Pending |
| AIRGAP-03 | Phase 43 | Pending |
| AIRGAP-04 | Phase 43 | Pending |
| AIRGAP-05 | Phase 43 | Pending |
| AIRGAP-06 | Phase 43 | Pending |
| STOR-01 | Phase 44 | Pending |
| STOR-02 | Phase 44 | Pending |
| STOR-03 | Phase 44 | Pending |
| STOR-04 | Phase 44 | Pending |
| STOR-05 | Phase 44 | Pending |
| STOR-06 | Phase 44 | Pending |
| MOUNT-01 | Phase 45 | Pending |
| MOUNT-02 | Phase 45 | Pending |
| MOUNT-03 | Phase 45 | Pending |
| MOUNT-04 | Phase 45 | Pending |
| LOAD-01 | Phase 46 | Pending |
| LOAD-02 | Phase 46 | Pending |
| LOAD-03 | Phase 46 | Pending |
| LOAD-04 | Phase 46 | Pending |

**Coverage:**
- v2.2 requirements: 25 total
- Mapped to phases: 25
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-08*
*Last updated: 2026-04-08 after roadmap creation*
