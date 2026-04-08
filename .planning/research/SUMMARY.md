# Project Research Summary

**Project:** kinder — v2.2 Cluster Capabilities (offline clusters, local-path-provisioner, host-dir mounts, multi-version nodes)
**Domain:** Kubernetes local development tooling — kind fork with extended cluster management capabilities
**Researched:** 2026-04-08
**Confidence:** HIGH — all four features grounded in direct codebase analysis plus official kind, Kubernetes, and Rancher documentation

## Executive Summary

Kinder v2.2 adds four cluster capabilities to an existing kind fork: offline/air-gapped cluster creation, local-path-provisioner dynamic storage, host-to-pod directory mounting, and multi-version per-node Kubernetes. The central research finding is that **zero new Go module dependencies are required** — every feature integrates using packages already present in the codebase. The data models for three of the four features are already partially or fully implemented in the type system; the work is validation logic, new actions, targeted bug fixes, and documentation rather than greenfield infrastructure.

The recommended build order prioritizes low-risk, isolated changes first. Multi-version node validation is the least risky starting point (a single new function in `validate.go`). Local-path-provisioner and host-directory mounting are medium-complexity but well-contained within the established addon action pattern. Offline/air-gapped is the most complex feature — it touches all three container runtime providers, requires auditing every addon manifest for image pull policies, and demands a clearly designed two-mode loading workflow (pre-create bake vs. post-create load) — and should be treated as a focused sub-milestone rather than bundled with the other three.

The dominant risk across all four features is the interaction surface between them: offline mode amplifies every image-dependency, the local-path-provisioner busybox helper creates a hidden two-image dependency for offline use, the `--image` flag has a confirmed bug that destroys per-node version configs, and two default StorageClasses will coexist silently if the `installstorage` gate is not wired correctly. Each pitfall has a clear, low-cost prevention strategy, but they must be addressed before rather than after integration testing.

---

## Key Findings

### Recommended Stack

No new dependencies are required. Every feature uses packages already in `go.mod`: `sigs.k8s.io/kind/pkg/exec` for shelling out to containerd/docker, `pkg/cluster/nodeutils` for `LoadImageArchive`, `pkg/internal/version` for semver parsing, `encoding/json` for docker inspect output, `_ "embed"` for the local-path-provisioner manifest (pattern already used in 7 addons), and the existing `cobra` command structure for new subcommands. The one rejected pattern that recurs is pulling in Go libraries for OCI operations — all image operations go through the Docker/containerd CLI, which is already the project's established pattern.

**Core technologies (all pre-existing in go.mod):**
- `sigs.k8s.io/kind/pkg/exec` — shell out to `docker save`, `ctr images import`, `docker commit` for offline image operations; same pattern as existing load commands
- `sigs.k8s.io/kind/pkg/cluster/nodeutils.LoadImageArchive` — pre-existing bottleneck function for loading images into nodes; `kinder load images` is a thin wrapper
- `sigs.k8s.io/kind/pkg/internal/version.ParseSemantic` + `LessThan` — version skew validation for multi-version nodes; no external semver library needed
- `_ "embed"` (stdlib) — `//go:embed manifests/local-path-storage.yaml` for the local-path-provisioner manifest, identical to cert-manager and MetalLB patterns
- `rancher/local-path-provisioner:v0.0.35` — latest upstream release (2026-03-10); includes fix for CVE-2025-62878 (CVSS 10.0 path traversal, fixed in v0.0.34); embed as YAML manifest, not as a Go dependency

**Critical manifest note:** Pin the embedded busybox helper to `busybox:1.37.0` and patch `imagePullPolicy: IfNotPresent` in the ConfigMap before embedding. The upstream manifest uses unpinned `busybox:latest` with `imagePullPolicy: Always` — the primary failure mode for offline storage provisioning.

### Expected Features

**Must have (table stakes):**
- `--air-gapped` flag on `kinder create cluster` that errors immediately on missing images instead of retrying pulls — without explicit opt-in there is no way to distinguish intentional offline from network failure
- Per-node `image:` field in v1alpha4 config that is NOT overridden by the global `--image` flag — confirmed bug in `fixupOptions()` that destroys multi-version cluster configs today
- Version-skew validation before provisioning — without it, invalid configurations surface as cryptic `kubeadm join` failures after node containers are already provisioned
- `local-path` as the default StorageClass replacing the legacy non-dynamic `standard` StorageClass — dynamic PVC provisioning is the expected behavior for any stateful workload
- `ExtraMounts` host-path existence validation pre-flight — missing host directory gives an obscure Docker container creation error rather than a clear kinder message

**Should have (differentiators):**
- `kinder doctor` offline readiness check — lists which required images are missing before a multi-minute cluster create fails
- `kinder doctor` CVE-2025-62878 check — warns if local-path-provisioner version in a running cluster is below v0.0.34
- Platform warning when `propagation: HostToContainer` or `Bidirectional` is specified on macOS/Windows — silently broken on Docker Desktop; emit an explicit warning and default to `propagation: None`
- `kinder get nodes` output extended with per-node K8s version column — makes multi-version cluster topology visible at a glance
- `--node-image role=image` per-node CLI flag for ad-hoc multi-version without a config file

**Defer to v2.3+:**
- `kinder images pull/load` commands (enumerate and pull all addon images in one shot) — high value but scope-creep for v2.2; per-addon `kinder load docker-image` is sufficient for initial release
- `--profile upgrade-test` preset — useful after core multi-version works and is stable
- Offline profile preset (`--profile offline`) — depends on offline feature being complete and well-tested first
- Auto-PV creation from `extraMounts.createPV` — the documentation-plus-doctor approach is sufficient for v2.2; auto-PV adds type system complexity for limited immediate gain

**Anti-features (explicitly excluded):**
- Bundling addon images into the kinder binary — binary would become multi-GB and break Homebrew distribution
- Auto-detecting internet availability and switching modes silently — unreliable on VPNs/proxies and causes confusing silent behavior
- Auto-rolling upgrades via multi-version clusters — out of scope; kubeadm covers this already
- Allowing HA control-plane version skew > 1 minor version — kubeadm refuses it; kinder must validate and reject upfront

### Architecture Approach

All four features follow the established config pipeline pattern: a 5-location change set (v1alpha4 types, defaults, deepcopy, internal types, conversion) followed by behavior gating in `create.go`. New addon actions live in dedicated packages under `pkg/cluster/internal/create/actions/` and use `//go:embed` for manifests plus `kubectl apply` via the control plane node's command runner — identical to MetalLB, Metrics Server, and cert-manager. The air-gapped feature is the architectural outlier: it introduces a `ProvisionOptions` struct on the `Provider` interface to carry a runtime-only flag without polluting the serialized cluster config YAML (same precedent as `--retain`, `--wait`).

**Major components and integration points:**
1. **`pkg/cluster/internal/providers/*/images.go` (3 files)** — air-gapped gate added to `ensureNodeImages()`; changes from retry-pull to fail-fast-with-clear-error when `AirGapped=true`; uses new `ProvisionOptions` struct on `provider.go` interface
2. **`pkg/cluster/internal/create/actions/installlocalpath/`** (new package) — local-path-provisioner addon action following `installmetricsserver` template exactly; `//go:embed manifests/local-path-storage.yaml` with patched `imagePullPolicy: IfNotPresent` and pinned busybox
3. **`pkg/cluster/internal/create/create.go`** — `installstorage` gated out when `LocalPath=true`; `installlocalpath` added to wave1; `AirGapped` flag threaded to provider via `ProvisionOptions`; `installhostmounts` added to sequential pipeline after `waitforready` (deferred to v2.3 if auto-PV is out of scope)
4. **`pkg/internal/apis/config/validate.go`** — new `validateNodeVersionSkew()` function; parses Kubernetes version from node image tags; validates all control-plane nodes at same version; workers within 3-minor-version skew of control-plane
5. **`fixupOptions()` in `create.go`** — targeted one-line bug fix: only override `node.Image` when the field is empty, preserving explicit per-node image specifications when `--image` flag is set globally

**5-location config pipeline checklist (required for every new config field):**
Every new field must atomically touch: `v1alpha4/types.go` → `v1alpha4/default.go` → `v1alpha4/zz_generated.deepcopy.go` → `internal/apis/config/types.go` → `internal/apis/config/convert_v1alpha4.go`. Partial state is a runtime bug. The `Addons.LocalPath *bool` field is the only new config field in v2.2 scope (auto-PV `Mount.CreatePV` deferred to v2.3).

### Critical Pitfalls

1. **busybox helper image not pre-loaded breaks all PVC operations in offline clusters** — the local-path-provisioner uses a busybox helper pod at PVC create/delete time; the upstream manifest uses `imagePullPolicy: Always` for `busybox:latest`. The provisioner pod shows `Running` but PVCs never bind. Patch the embedded manifest to `IfNotPresent` and pin to `busybox:1.37.0` before the `go:embed`. This must happen in the manifest file; it cannot be patched at runtime.

2. **`ctr images import --all-platforms` fails on Docker Desktop 27+ with the containerd image store enabled** — Docker Desktop 27+ exports multi-platform manifests with attestation layers; `ctr import` rejects them with `content digest: not found`. Implement a fallback: attempt `--local` mode (containerd 2.x) or drop `--all-platforms` in favour of the current host platform only. Do not ship `kinder load images` without this fallback tested against Docker Desktop 27+.

3. **Two default StorageClasses if `installstorage` is not gated out** — `installstorage` installs `standard` (annotated as default) unconditionally. `local-path-provisioner` also installs a default-annotated StorageClass. On Kubernetes 1.26+ the admission webhook blocks the second default; on older versions both exist and PVC binding is non-deterministic. Fix: add a conditional in `create.go` that skips `installstorage` when `LocalPath=true`. Define this ownership model before writing the addon action.

4. **`--image` flag overrides per-node images, destroying multi-version configs** — `fixupOptions()` currently iterates all nodes and sets every node's image to the flag value. Fix: only override nodes where `node.Image == ""`. This is a one-line targeted fix but must land before any multi-version testing begins.

5. **Mount propagation silently broken on macOS/Windows** — Docker Desktop uses virtioFS/gRPC FUSE and cannot propagate kernel mount events across the VM boundary. `propagation: HostToContainer` and `Bidirectional` are silently dropped with no error. Default to `propagation: None`; emit an explicit platform warning (check `runtime.GOOS`) when a non-None propagation is requested on non-Linux hosts.

---

## Implications for Roadmap

Based on research, the four features decompose into a five-phase build that respects dependencies, isolates risk, and addresses critical pitfalls at the correct layer.

### Phase 1: Multi-Version Node Validation

**Rationale:** Lowest code risk of any feature in this milestone. Isolated to a single new function in `validate.go` with no new files and no full config pipeline changes. Also fixes the confirmed `--image` flag override bug which is a prerequisite for any multi-version testing or accurate CI use. Delivers immediate, standalone value.

**Delivers:** Version-skew validation at config parse time with clear error messages instead of cryptic kubeadm join failures; `--image` flag bug fix preserving per-node image specifications; optional `kinder doctor` version-skew check; optional `kinder get nodes` K8s version column extension.

**Addresses:** FEATURES.md table-stakes — per-node image support working correctly, version-skew validation, clear preflight error surface. FEATURES.md differentiators — `kinder doctor` skew check, extended node output.

**Avoids:** Pitfall 9 (version skew surfaces at kubeadm join — caught at validation instead); Pitfall 6 (document KubeletConfiguration patch limitation for workers — upstream kubeadm design, not a kinder bug); Pitfall 7 (add warning for v1beta3/v1beta4 boundary if node versions span Kubernetes 1.31+).

**Research flag:** Standard patterns; skip `/gsd-research-phase`. Semver parsing uses `pkg/internal/version` already in use throughout the codebase; validation logic is straightforward.

### Phase 2: Air-Gapped Cluster Creation

**Rationale:** Second because it touches the widest surface (all three provider implementations) and must be stable before local-path-provisioner references it for its offline image warning. The `ProvisionOptions` struct introduced here establishes the correct architecture for runtime-only provider flags.

**Delivers:** `--air-gapped` CLI flag on `kinder create cluster`; fail-fast behavior in all three providers when required images are absent; addon image warning output listing all images that need pre-loading before creation; `kinder doctor` offline readiness check; documented two-mode design (pre-create bake via privileged container commit vs. post-create load via `kinder load docker-image`).

**Addresses:** FEATURES.md table-stakes — explicit offline opt-in, clear error surfacing. FEATURES.md anti-features — no bundled images, no auto-detection.

**Avoids:** Pitfall 8 (addon manifests reference images not present — warn with full image list at creation time); Pitfall 5 (image pre-baking requires privileged container commit, not Dockerfile — design this decision explicitly before implementation); Integration Pitfall 1 (two-phase image dependency — document pre-create vs. post-create load modes before writing any code).

**Research flag:** Needs `/gsd-research-phase` during planning. The `Provider` interface change (`ProvisionOptions`) must be reviewed for backward compatibility with test infrastructure that mocks `Provider`. The two-mode image loading architectural decision must be documented and agreed before code is written.

### Phase 3: Local-Path-Provisioner Addon

**Rationale:** Third because the air-gapped warning in phase 2 will include busybox and the provisioner image — making the interaction explicit before the addon is written. The addon itself follows a known pattern but the StorageClass ownership decision must be made upfront to avoid the collision pitfall.

**Delivers:** `installlocalpath` addon action package; patched manifest with `imagePullPolicy: IfNotPresent` and pinned `busybox:1.37.0`; `installstorage` gate in `create.go` skipped when `LocalPath=true`; `local-path` as the default StorageClass; full 5-location config pipeline for `Addons.LocalPath *bool`; `kinder doctor` CVE-2025-62878 check; documentation of `/opt/local-path-provisioner` as reserved path and PV orphan behavior on cluster delete.

**Addresses:** FEATURES.md table-stakes — dynamic PVC provisioning, `local-path` as default StorageClass, predictable node storage path, single and multi-node topology support. FEATURES.md differentiators — configurable base path via config, doctor CVE check.

**Avoids:** Pitfall 1 (busybox helper image — patch before `go:embed`, not at runtime); Pitfall 3 (StorageClass collision — gate out `installstorage`, define ownership model first); Integration Pitfall 3 (document `/opt/local-path-provisioner` as reserved; validate user mounts do not shadow it).

**Research flag:** Standard patterns; skip `/gsd-research-phase`. The addon action pattern is identical to existing addons. The StorageClass ownership model is answered by research: `local-path-provisioner` replaces `installstorage` when enabled.

### Phase 4: Host-Directory Mounting

**Rationale:** Fourth because the ExtraMounts type system already works end-to-end; the remaining work is pre-flight validation, doctor checks, platform warnings, and documentation. Benefits from local-path-provisioner being installed (phase 3) since the most powerful mounting pattern is: host dir → node extraMount → local-path-provisioner configurable `nodePath` as PV backing.

**Delivers:** Pre-flight `hostPath` existence validation before node provisioning; platform warning for non-None propagation modes on macOS/Windows; two `kinder doctor` checks (path exists, Docker Desktop file sharing on macOS); documentation and example YAML for the two-hop mount pattern (host → node extraMount → pod hostPath PV).

**Addresses:** FEATURES.md table-stakes — end-to-end ExtraMounts working with clear errors, two-layer pattern documented. FEATURES.md differentiators — doctor path/sharing checks, propagation mode guidance.

**Avoids:** Pitfall 4 (propagation silent on macOS/Windows — platform warning via `runtime.GOOS` check + default to `propagation: None`); Pitfall 12 (missing host directory obscure error — pre-flight `os.Stat` validation with clear message).

**Research flag:** Standard patterns; skip `/gsd-research-phase`. ExtraMounts infrastructure is complete; work is validation and documentation. Auto-PV (`CreatePV` field on `Mount`) is explicitly deferred to v2.3 to keep scope contained and avoid adding complexity to the `Mount` type in this milestone.

### Phase 5: kinder load images Command

**Rationale:** Last because it is the utility that supports offline and multi-version workflows. Building it last means the consumer features (offline in phase 2, local-path in phase 3) are stable and their image requirements are known. The multi-platform Docker Desktop containerd store pitfall is the hardest technical problem in this milestone and should not block the four primary features.

**Delivers:** `kinder load images` subcommand; provider-abstracted image saving (no hardcoded `docker save`; uses `runtime.GetDefault()` provider); fallback import strategy for Docker Desktop 27+ containerd image store (`--local` flag or drop `--all-platforms`); documentation of expected load times for multi-node clusters.

**Addresses:** FEATURES.md differentiators — `kinder images load` wrapping the per-image loop; supports offline workflow completion; supports multi-node cluster image pre-seeding.

**Avoids:** Pitfall 2 (multi-platform ctr import failure on Docker Desktop 27+ — implement and test `--local` fallback before declaring complete); Pitfall 11 (slow load for multi-node — leverage existing smart-load that skips already-present images; document expected times); Pitfall 13 (hardcoded `docker save` breaking podman/nerdctl — use provider abstraction from the start).

**Research flag:** Needs `/gsd-research-phase` during planning. The containerd 2.x `--local` flag availability and exact version requirements need verification against the Docker Desktop 27+ version matrix before implementation begins. The exact `getSnapshotter()` version-awareness strategy in `pkg/cluster/nodeutils/util.go` needs review for extension.

### Phase Ordering Rationale

- Phase 1 first: fixes a confirmed regression bug (`--image` override) and adds pure validation with zero risk of breaking existing clusters. The fix is a prerequisite for any multi-version testing in all later phases.
- Phase 2 before phase 3: the air-gapped image warning list in `create.go` must enumerate addon images including those from local-path-provisioner. Writing that warning while the addon is being built in the same phase creates merge risk and ordering ambiguity.
- Phase 3 before phase 4: the most powerful host-mounting use case (host dir as PV backing via configurable `nodePath`) depends on local-path-provisioner being installed and its storage path being settable.
- Phase 5 last: it is a support utility; all consumer features work without it (using raw `kinder load docker-image` per image) and the Docker Desktop 27+ compatibility problem is the most uncertain technical challenge in the milestone.
- Phases 3 and 4 could be merged if schedule pressure exists; their config pipeline changes touch different structs (`Addons` vs `Mount`) and have no code conflicts.

### Research Flags

Phases needing deeper research during planning:
- **Phase 2 (Air-Gapped):** The `ProvisionOptions`/`Provider` interface change is a breaking change to the internal provider interface. Needs review of all three provider implementations and test infrastructure that mocks `Provider`. Also: the pre-create bake vs. post-create load architectural decision must be documented before any code is written.
- **Phase 5 (kinder load images):** The Docker Desktop 27+ containerd image store compatibility problem (Pitfall 2) needs a test matrix. The exact `--local` flag availability in the installed containerd 2.x version must be verified before implementation.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Multi-Version Validation):** `validate.go` pattern is established; `pkg/internal/version` semver utilities are documented and in active use.
- **Phase 3 (Local-Path-Provisioner):** Addon action pattern is identical to `installmetricsserver`; 5-location config pipeline is documented and mechanical.
- **Phase 4 (Host-Directory Mounting):** ExtraMounts infrastructure is complete; doctor check pattern is established. Only new work is validation logic and documentation.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Direct `go.mod` verification; confirmed zero new dependencies; all packages in active use in the codebase |
| Features | HIGH | Verified against kind official docs, K8s version-skew policy, Rancher release notes; MEDIUM for offline UX tradeoffs (multiple sources but some community-only) |
| Architecture | HIGH | Direct codebase analysis of all relevant files; config pipeline pattern confirmed across 5+ existing fields; provider pattern confirmed in all three providers |
| Pitfalls | HIGH | Critical pitfalls 1-4 verified against kind issue tracker and official docs; MEDIUM for Pitfall 7 (v1beta4 removal timeline — single source, exact Kubernetes version TBD) |

**Overall confidence:** HIGH

### Gaps to Address

- **kubeadm v1beta3/v1beta4 removal timeline (Pitfall 7):** Research identifies that v1beta3 support will be removed "in Kubernetes 1.34 or later." The exact version is not confirmed. During Phase 1 planning, add a validation warning for node version combinations that include a node at Kubernetes 1.31+ but treat this as a soft warning rather than a hard error until the upstream removal version is confirmed.
- **Docker Desktop 27+ containerd store `--local` flag availability:** Research confirms the failure mode (Pitfall 2) and proposes `--local` as the fix, but the exact flag name and the minimum containerd 2.x minor version that supports it need verification against a live Docker Desktop 27+ environment before Phase 5 implementation begins.
- **`installstorage` StorageClass name assumption:** `installstorage` reads from `/kind/manifests/default-storage.yaml` inside the node image (falls back to a hardcoded StorageClass). If a custom node image ships a different StorageClass name, the gate logic in `create.go` may skip `installstorage` but leave a ghost StorageClass. During Phase 3, validate the StorageClass name against the current default node image before relying on it in the gate.
- **busybox helper image location in local-path-provisioner manifest:** Research confirms the helper image is in the `local-path-config` ConfigMap's `config.json` key, not the Deployment spec. The pinned `busybox:1.37.0` substitution must be applied to the ConfigMap key. Verify the exact ConfigMap structure against the v0.0.35 manifest before writing the embedded manifest patch.

---

## Sources

### Primary (HIGH confidence)
- Kinder codebase direct reads: `pkg/build/nodeimage/imageimporter.go`, `buildcontext.go`, `const_storage.go`, `pkg/cmd/kind/load/docker-image/docker-image.go`, `pkg/cluster/nodeutils/util.go`, `pkg/cluster/internal/create/create.go`, `pkg/apis/config/v1alpha4/types.go`, `pkg/internal/apis/config/convert_v1alpha4.go`, `pkg/cluster/internal/providers/docker/images.go`, `pkg/cluster/internal/create/actions/installstorage/storage.go` — direct source of truth for all architecture findings
- [Kind Working Offline docs](https://kind.sigs.k8s.io/docs/user/working-offline/) — confirmed pre-built node image + `kind load` approach
- [Kind Configuration docs](https://kind.sigs.k8s.io/docs/user/configuration/) — confirmed `Node.Image` per-node field + `ExtraMounts`
- [Kind Local Registry](https://kind.sigs.k8s.io/docs/user/local-registry/) — registry mirror pattern
- [Kind known issues](https://kind.sigs.k8s.io/docs/user/known-issues/) — mount propagation macOS limitation
- [Kubernetes Version Skew Policy](https://kubernetes.io/releases/version-skew-policy/) — kubelet up to 3 minor versions older than kube-apiserver (since K8s 1.28)
- [rancher/local-path-provisioner v0.0.35 release](https://github.com/rancher/local-path-provisioner/releases/tag/v0.0.35) — latest release 2026-03-10, CVE-2025-62878 fix confirmed
- Kind issue tracker: #3795, #3996, #2402 (image loading failures); #2576, #2400, #2700 (mount propagation macOS); #2697, #3191 (local-path-provisioner); #1424 (per-node kubeadm patches — upstream limitation)

### Secondary (MEDIUM confidence)
- [CVE-2025-62878 advisory — Orca Security](https://orca.security/resources/blog/cve-2025-62878-rancher-local-path-provisioner/) — CVSS 10.0, path traversal fixed in v0.0.34; third-party source but consistent with upstream release notes
- [iximiuz.com — KIND: How I Wasted a Day Loading Local Docker Images](https://iximiuz.com/en/posts/kubernetes-kind-load-docker-image/) — imagePullPolicy pitfall; single source but consistent with kind docs
- [maelvls.dev — Pull-through Docker registry on Kind clusters](https://maelvls.dev/docker-proxy-registry-kind/) — registry mirror pattern for offline use
- [DEV.to — Addressing Limitations of Local Path Provisioner](https://dev.to/frosnerd/addressing-the-limitations-of-local-path-provisioner-in-kubernetes-3g12) — PV orphan and node-local storage limitations
- [blog.andygol.co.ua — Getting access to host filesystem for PV in Kind](https://blog.andygol.co.ua/en/2025/04/05/host-fs-to-backup-pv-in-kind/) — two-layer mount pattern
- k3s-io/k3s#1908, k3s-io/k3s#2391 — busybox helper air-gap failure in local-path-provisioner (behavior consistent with official docs)
- [minikube local-path-provisioner tutorial](https://minikube.sigs.k8s.io/docs/tutorials/local_path_provisioner/) — provisioner behavior in a similar local K8s tool

### Tertiary (LOW confidence)
- [Kubernetes 1.31 blog — kubeadm v1beta4](https://kubernetes.io/blog/2024/08/23/kubernetes-1-31-kubeadm-v1beta4/) — v1beta3 removal timeline stated as "1.34 or later"; exact version unconfirmed, needs monitoring against upstream kind releases

---
*Research completed: 2026-04-08*
*Ready for roadmap: yes*
