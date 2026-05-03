# Requirements: Kinder v2.3 Inner Loop

**Defined:** 2026-05-03
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Milestone Goal:** Make daily iteration on a kinder cluster as fast as creating one — pause/resume to reclaim laptop resources, snapshot/restore for instant clean state, hot-reload for code changes, and runtime error decoding extending the v2.1 doctor framework. Includes a sync phase for kind upstream's HAProxy→Envoy LB transition and K8s 1.36 default.

## v2.3 Requirements

Requirements for the Inner Loop milestone. Each maps to exactly one phase.

### LIFE — Cluster Lifecycle

- [x] **LIFE-01**: User can pause a running cluster via `kinder pause [name]`, freeing CPU/RAM without losing state
- [x] **LIFE-02**: User can resume a paused cluster via `kinder resume [name]`; pods, PVs, and node state are preserved
- [x] **LIFE-03**: Pause-resume orchestrates control-plane and worker stop/start in correct order to preserve etcd quorum on HA clusters
- [x] **LIFE-04**: Doctor pre-flight check `cluster-resume-readiness` runs before resume on HA clusters and warns if etcd quorum is at risk
- [ ] **LIFE-05**: User can snapshot a cluster via `kinder snapshot create [snap-name]`; snapshot captures etcd state, loaded container images, and local-path-provisioner PV contents
- [ ] **LIFE-06**: User can restore a cluster from a snapshot via `kinder snapshot restore [snap-name]`; restore refuses if Kubernetes version in snapshot mismatches current cluster
- [ ] **LIFE-07**: User can list, inspect (size, age, k8s version), and prune snapshots via `kinder snapshot list`, `kinder snapshot show [snap-name]`, `kinder snapshot prune`
- [ ] **LIFE-08**: Snapshot metadata records cluster Kubernetes version, addon versions, and image-bundle digest for air-gap reproducibility

### DEV — Inner Loop

- [ ] **DEV-01**: User can run `kinder dev --watch <dir> --target <deployment>` to watch a directory and trigger a build-load-rollout pipeline on file changes
- [ ] **DEV-02**: `kinder dev` builds an image from the watched directory (Dockerfile-based) and imports it via the existing `kinder load images` pipeline
- [ ] **DEV-03**: After successful image load, `kinder dev` rolls the target Deployment via `kubectl rollout restart` and waits for ready
- [ ] **DEV-04**: `kinder dev` debounces rapid file changes (configurable, default 500ms) and shows build/load/rollout timing per cycle
- [ ] **DEV-05**: `kinder dev` supports `--poll` flag for fsnotify-unfriendly environments (Docker Desktop volume mounts on macOS); polls watched directory at configurable interval

### DIAG — Runtime Diagnostics (extends v2.1 doctor framework)

- [ ] **DIAG-01**: User can run `kinder doctor decode` to scan recent docker logs and `kubectl get events` for known error patterns and print plain-English explanations with suggested fixes
- [ ] **DIAG-02**: Decoder ships with at least 15 cataloged error patterns covering kubelet, kubeadm, containerd, docker, and addon-startup failures (sourced from kind issue tracker, v2.1 checks, and CI failures)
- [ ] **DIAG-03**: Each decoded error includes: pattern matched, plain-English explanation, suggested fix, link to docs/issue (where applicable)
- [ ] **DIAG-04**: `kinder doctor decode --auto-fix` applies known-safe remediations automatically (whitelist only — never destructive)

### SYNC — Upstream Sync & K8s 1.36

- [ ] **SYNC-01**: Adopt kind PR #4127 — replace HAProxy load-balancer with Envoy in HA cluster setups; drop `kindest/haproxy` image dependency
- [ ] **SYNC-02**: Default `kindest/node` image bumped to a Kubernetes 1.36.x patch release (latest stable at ship time, ≥1.36.4 to avoid kind #4131 regression)
- [ ] **SYNC-03**: Cluster config validation rejects `kubeProxyMode: ipvs` on K8s 1.36+ with a clear error message pointing to the iptables migration path
- [ ] **SYNC-04**: Website ships a "What's new in K8s 1.36" recipe page demonstrating User Namespaces (GA) and In-Place Pod Resize (GA) on a kinder cluster

## v2.4+ Requirements (Deferred)

Tracked but not in current roadmap.

### Tech Debt (deferred from CONCERNS audit)

- **DEBT-01**: Provider de-duplication — extract docker/podman/nerdctl shared code parameterized by binary
- **DEBT-02**: context.Context cancellation through `cluster.Create()` and node-provisioning blocking ops
- **DEBT-03**: kubeadm v1beta4 generator alongside v1beta3 (pick by node K8s version)

### Addon Bumps

- **ADDON-01**: cert-manager v1.16.3 → v1.20.x (UID/GID 1000→65532 audit required)
- **ADDON-02**: Envoy Gateway v1.3.1 → v1.7.x (staged 1.3 → 1.5 → 1.7)
- **ADDON-03**: Headlamp v0.40.1 → v0.41.x (MCP server for AI assistants)

### Lifecycle Expansion

- **LIFE-09**: Pause/resume support for podman provider
- **LIFE-10**: Pause/resume support for nerdctl provider
- **LIFE-11**: Snapshot remote-storage backend (S3/GCS) for team sharing

## Out of Scope

Explicitly excluded from kinder. Documented to prevent scope creep.

| Feature | Reason |
|---|---|
| Cilium-by-default CNI swap | Kernel/Docker Desktop pain; blast radius too large for default; opt-in addon path remains available |
| Tiltfile-style DSL for `kinder dev` | Compete-with-Tilt loses; stay imperative-CLI minimal |
| Velero bundle for backup/restore | Overkill for laptops; lightweight `kinder snapshot` covers actual dev pain |
| Web UI for cluster management | Headlamp covers it (already shipped); UX treadmill |
| Apple Containerization runtime support | Surface unstable; revisit in 6-12 months |
| `kinder fleet` multi-cluster command | Different milestone theme; vind/k3d coverage exists |
| `kinder observe` LGTM stack preset | Different milestone theme; resource-heavy; opt-in path via addons remains |
| `kinder gpu ollama` AI runtime preset | Different milestone theme; revisit if user demand surfaces |
| `kinder gitops` Argo/Flux preset | Niche; defer until requested |
| `kinder runner` GitHub Actions runner preset | Niche; defer until requested |

## Traceability

Which phases cover which requirements. Updated during roadmap creation by gsd-roadmapper.

| Requirement | Phase | Status |
|-------------|-------|--------|
| LIFE-01 | 47 | Complete (47-02) |
| LIFE-02 | 47 | Complete (47-03) |
| LIFE-03 | 47 | Complete (47-02 pause-side + 47-03 resume-side) |
| LIFE-04 | 47 | Complete (47-04) |
| LIFE-05 | 48 | Pending |
| LIFE-06 | 48 | Pending |
| LIFE-07 | 48 | Pending |
| LIFE-08 | 48 | Pending |
| DEV-01 | 49 | Pending |
| DEV-02 | 49 | Pending |
| DEV-03 | 49 | Pending |
| DEV-04 | 49 | Pending |
| DEV-05 | 49 | Pending |
| DIAG-01 | 50 | Pending |
| DIAG-02 | 50 | Pending |
| DIAG-03 | 50 | Pending |
| DIAG-04 | 50 | Pending |
| SYNC-01 | 51 | Pending |
| SYNC-02 | 51 | Pending |
| SYNC-03 | 51 | Pending |
| SYNC-04 | 51 | Pending |

**Coverage:**
- v2.3 requirements: 21 total
- Mapped to phases: 21
- Unmapped: 0 ✓

---
*Requirements defined: 2026-05-03*
*Last updated: 2026-05-03 — traceability mapped by gsd-roadmapper (phases 47-51)*
