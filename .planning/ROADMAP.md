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
- ACTIVE **v2.3 Inner Loop** - Phases 47-51 (in progress)

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

<details open>
<summary>ACTIVE v2.3 Inner Loop (Phases 47-51) - IN PROGRESS</summary>

**Milestone Goal:** Make daily iteration on a kinder cluster as fast as creating one — pause/resume to reclaim laptop resources, snapshot/restore for instant clean state, hot-reload for code changes, runtime error decoding extending the v2.1 doctor framework, and an upstream sync to adopt kind's HAProxy→Envoy LB transition with K8s 1.36 as the new default.

### Phase 47: Cluster Pause/Resume
**Goal**: Users can pause and resume a kinder cluster to reclaim laptop resources without losing any cluster state
**Depends on**: Phase 46 (v2.2 complete)
**Requirements**: LIFE-01, LIFE-02, LIFE-03, LIFE-04
**Success Criteria** (what must be TRUE):
  1. User runs `kinder pause [name]` and all cluster containers stop; CPU and RAM drop to near-zero on the host
  2. User runs `kinder resume [name]` and the cluster becomes fully operational; pods, PVs, and services are in the same state as before pause
  3. On a multi-control-plane cluster, pause/resume orchestrates container stop/start in quorum-safe order (workers before control-plane nodes on pause; reverse on resume)
  4. Before resuming an HA cluster, `kinder doctor` emits a `cluster-resume-readiness` warning if etcd quorum is at risk
**Plans**: 4 plans
- [x] 47-01-PLAN.md — Cluster status surface: container-state helpers, `kinder status [name]`, Status column on `kinder get clusters` (JSON schema migration), real container state on `kinder get nodes`, register pause/resume stub commands in root.go
- [x] 47-02-PLAN.md — `kinder pause`: quorum-safe stop (workers→CP→LB), best-effort errors, idempotent no-op, `--timeout`/`--json` flags, HA pre-pause etcd snapshot to `/kind/pause-snapshot.json`
- [x] 47-03-PLAN.md — `kinder resume`: quorum-safe start (LB→CP→workers), best-effort errors, idempotent no-op, `--wait`/`--timeout`/`--json` flags, all-nodes-Ready gate via kubectl with K8s 1.24 selector fallback
- [ ] 47-04-PLAN.md — `cluster-resume-readiness` doctor check: registered in v2.1 doctor catalog, HA-only with skip on single-CP, warn-and-continue on unhealthy etcd members, gracefully skip when etcdctl missing, inline invocation between CP-start and worker-start in `lifecycle.Resume`

### Phase 48: Cluster Snapshot/Restore
**Goal**: Users can capture a complete cluster state as a named snapshot and restore it in seconds, enabling instant reset between development cycles
**Depends on**: Phase 47
**Requirements**: LIFE-05, LIFE-06, LIFE-07, LIFE-08
**Success Criteria** (what must be TRUE):
  1. User runs `kinder snapshot create [snap-name]` and a snapshot archive is created that captures etcd state, all loaded container images, and local-path-provisioner PV contents
  2. User runs `kinder snapshot restore [snap-name]` and the cluster returns to the captured state; the command refuses with a clear error if the snapshot's Kubernetes version differs from the current cluster
  3. User can run `kinder snapshot list` to see all snapshots, `kinder snapshot show [snap-name]` to inspect size, age, k8s version, and image digest, and `kinder snapshot prune` to delete old snapshots
  4. Each snapshot's metadata records the cluster Kubernetes version, addon versions, and image-bundle digest for air-gap reproducibility
**Plans**: TBD

### Phase 49: Inner-Loop Hot Reload (`kinder dev`)
**Goal**: Users can iterate on application code inside a kinder cluster with a single command that watches for file changes and completes a full build-load-rollout cycle automatically
**Depends on**: Phase 48
**Requirements**: DEV-01, DEV-02, DEV-03, DEV-04, DEV-05
**Success Criteria** (what must be TRUE):
  1. User runs `kinder dev --watch <dir> --target <deployment>` and the command enters watch mode; saving a file in the watched directory triggers a build-load-rollout cycle automatically
  2. Each cycle builds a Docker image from the watched directory, imports it via the existing `kinder load images` pipeline, and rolls the target Deployment via `kubectl rollout restart`; timing for each step is printed per cycle
  3. Rapid file saves within the configurable debounce window (default 500ms) trigger only one cycle, not one per save
  4. On Docker Desktop for macOS where fsnotify events are unreliable, user can pass `--poll` to switch to a polling-based watcher at a configurable interval
**Plans**: TBD
**UI hint**: yes

### Phase 50: Runtime Error Decoder
**Goal**: Users can decode cryptic runtime errors from running clusters into plain-English explanations with actionable fixes, extending the v2.1 doctor framework into post-create diagnostics
**Depends on**: Phase 47
**Requirements**: DIAG-01, DIAG-02, DIAG-03, DIAG-04
**Success Criteria** (what must be TRUE):
  1. User runs `kinder doctor decode` and the command scans recent docker logs and `kubectl get events`, matches known error patterns, and prints plain-English explanations with suggested fixes
  2. The decoder recognizes at least 15 cataloged error patterns covering kubelet, kubeadm, containerd, docker, and addon-startup failures
  3. Each matched error shows: the pattern that matched, a plain-English explanation, the suggested fix, and a link to documentation or a known issue where applicable
  4. User runs `kinder doctor decode --auto-fix` and the command applies only whitelisted, non-destructive remediations automatically; no destructive action is ever taken without explicit user confirmation
**Plans**: TBD

### Phase 51: Upstream Sync & K8s 1.36
**Goal**: Kinder adopts kind's HAProxy-to-Envoy LB transition, ships K8s 1.36 as the default node image, and protects users from the silent IPVS removal breakage introduced in 1.36
**Depends on**: Phase 50
**Requirements**: SYNC-01, SYNC-02, SYNC-03, SYNC-04
**Success Criteria** (what must be TRUE):
  1. HA clusters created with kinder use Envoy as the load-balancer container instead of HAProxy; `kindest/haproxy` is no longer pulled
  2. Running `kinder create cluster` without an explicit `image:` field provisions a K8s 1.36.x node (latest stable patch at ship time, >=1.36.4)
  3. A cluster config with `kubeProxyMode: ipvs` is rejected at validation time with a clear error message pointing to the iptables migration path when the node version is 1.36 or higher
  4. The kinder website has a "What's new in K8s 1.36" recipe page with working examples demonstrating User Namespaces (GA) and In-Place Pod Resize (GA) on a kinder cluster
**Plans**: TBD
**UI hint**: yes

</details>

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
| 47. Cluster Pause/Resume | v2.3 | 0/4 | Planned | - |
| 48. Cluster Snapshot/Restore | v2.3 | 0/TBD | Not started | - |
| 49. Inner-Loop Hot Reload | v2.3 | 0/TBD | Not started | - |
| 50. Runtime Error Decoder | v2.3 | 0/TBD | Not started | - |
| 51. Upstream Sync & K8s 1.36 | v2.3 | 0/TBD | Not started | - |
