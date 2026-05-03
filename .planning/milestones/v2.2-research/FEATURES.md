# Feature Landscape: v2.2 Cluster Capabilities

**Domain:** Kubernetes local development tooling — offline/air-gapped clusters, local-path-provisioner storage, host directory mounting, multi-version (per-node K8s version) clusters
**Researched:** 2026-04-08
**Overall Confidence:** HIGH for mechanics (verified via kind docs, rancher/local-path-provisioner source, Kubernetes version-skew policy); MEDIUM for implementation approach tradeoffs (verified with multiple sources)

---

## Context and Scope

This research covers four discrete capabilities being added to kinder v2.2. Each is treated as an independent feature domain. Existing kinder infrastructure relevant to all four:

- `v1alpha4` config with `ContainerdConfigPatches []string` (cluster-wide) and `ExtraMounts []Mount` (per-node) — both are raw passthrough to kind's node provisioner and are already in the type system
- `Addons` struct with `*bool` fields — the addon registration/disable pattern is established
- `LocalRegistry` addon — already creates `kind-registry` container and wires `containerd certs.d` on all nodes
- `installstorage` action — already applies a default StorageClass from `/kind/manifests/default-storage.yaml` or a hardcoded fallback
- Wave-based parallel addon execution — new addons slot into wave 1 (parallel, no dependencies) or wave 2 (sequential, after MetalLB)
- `kinder doctor` with 18 checks — new checks can be added to existing check registry

---

## Feature 1: Offline / Air-Gapped Clusters

### What "Air-Gapped" Means for a kind-based Tool

A kind-based cluster is air-gapped when the node containers cannot reach public container registries (docker.io, registry.k8s.io, quay.io, ghcr.io) during cluster creation or addon installation. This requires:

1. The `kindest/node` image (and HAProxy image for HA) must be pre-loaded into the Docker daemon before `kinder create cluster`
2. All addon images (MetalLB, Envoy Gateway, cert-manager, Metrics Server, etc.) must be pre-seeded into cluster nodes via `kind load` OR routed through a local registry mirror that has been pre-populated
3. containerd on each node must be configured to NOT fall back to upstream registries, or to route through a local mirror

### Table Stakes

Features users expect if kinder claims "air-gapped support." Without these, the feature does not work at all.

| Feature | Why Expected | Complexity | Existing Dependency |
|---------|--------------|------------|---------------------|
| **`--offline` flag on `kinder create cluster`** that skips all network-dependent addon installs and instead expects images to already be present | Without a flag, there is no way to distinguish "cluster that happens to have no internet" from "intentionally offline cluster." Users need explicit opt-in | MEDIUM | Reads `Addons` config; must skip HTTP-based installs and substitute local image refs |
| **`kinder images list`** (or `kinder images export`) subcommand that prints/saves all images required for a fully functional cluster at the current addon versions | Users cannot guess which image tags are needed for each addon version. Production air-gap workflows require a complete, versioned bill of materials | MEDIUM | Must enumerate image refs from all addon manifests (currently embedded as YAML strings in each action package) |
| **`kind load docker-image` passthrough or `kinder load` command** that pre-seeds images into all cluster nodes | kind already provides `kind load docker-image` and `kind load image-archive`; kinder must not remove this capability | LOW | Kind's existing load commands are preserved; kinder is a fork, not a wrapper |
| **containerd registry mirror configuration via `ContainerdConfigPatches`** that routes pulls through a local mirror | Already possible via v1alpha4 config — this is a documentation and UX concern, not a new code feature. Must be tested and documented | LOW | `ContainerdConfigPatches` field already exists in v1alpha4 types |
| **`imagePullPolicy: IfNotPresent` enforced on all addon manifests** when offline mode is active | If any addon manifest uses the default (`Always` for `:latest` tags), it will fail even after pre-seeding. This is the #1 pitfall with `kind load` | MEDIUM | All addon manifests must be audited and rendered with correct pull policy |

### Differentiators

Features that make kinder's air-gapped experience meaningfully better than raw kind.

| Feature | Value Proposition | Complexity | Existing Dependency |
|---------|-------------------|------------|---------------------|
| **`kinder images pull`** — single command to pull all addon images to the local Docker daemon (for later transfer) | Eliminates the need for users to manually identify and pull a dozen images across registries. Batteries-included parity | MEDIUM | Requires extracting image refs from embedded manifests |
| **`kinder images load`** — single command to load all pre-pulled addon images from the local Docker daemon into all cluster nodes | `kind load docker-image` must be called once per image per cluster; `kinder images load` wraps this loop | LOW | Delegates to `kind load` internally |
| **Offline profile preset (`--profile offline`)** — creates a minimal cluster with only offline-compatible addons enabled by default | CI/CD pipelines and air-gapped enterprise environments need a turnkey preset. Complements existing `--profile minimal/full/gateway/ci` | MEDIUM | Extends existing `--profile` flag handling |
| **Doctor check: offline readiness** — `kinder doctor` checks if required node and addon images are available locally before cluster creation | Catch missing images before a multi-minute failed cluster create. Shows which images are missing and how to pull them | MEDIUM | Extends existing doctor check infrastructure |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Bundling all addon images into the `kinder` binary or a companion tarball** | Binary size becomes enormous (multi-GB). Breaks distribution via Homebrew. Images must be version-matched to each K8s release | Print a manifest file users can pass to `docker pull`. Keep kinder lean |
| **Attempting to auto-detect whether internet is available and switching modes** | Unreliable (VPNs, proxies, split-horizon DNS). Silent mode switches cause confusing behavior | Require explicit `--offline` opt-in |
| **Custom registry server embedded in kinder** | The LocalRegistry addon already ships `registry:2`. Running a second registry inside kinder is redundant and creates port conflicts | Reuse the existing LocalRegistry addon, pre-populated |

### Complexity Assessment

**Overall: HIGH.** The mechanical changes (flags, image enumeration, pre-seeding loop) are individually medium-complexity. The high complexity comes from:
- Every addon action must be audited for hardcoded image refs and pull policies
- The image manifest must stay in sync as addon versions are bumped
- Testing requires a truly network-isolated environment (not just "Docker with VPN")

### Dependencies on Existing Kinder Features

- **LocalRegistry addon** — the local registry container can serve as the mirror endpoint; offline mode can route all addon image pulls through `localhost:5001`
- **`ContainerdConfigPatches`** — already supports injecting registry mirror config into all node containers at provisioning time
- **Addon `*bool` disable flags** — offline mode may disable addons that have no offline-safe equivalent
- **`--profile` flag** — offline profile builds on this infrastructure

---

## Feature 2: Local-Path-Provisioner Storage

### What Local-Path-Provisioner Is

Rancher's `local-path-provisioner` (v0.0.35 as of research date) dynamically provisions `hostPath` or `local` PVs on the node where the pod is scheduled. It is the default StorageClass in K3s and is widely used in local dev clusters. It requires only a single controller pod and zero cluster-level storage infrastructure.

Default storage path on nodes: `/opt/local-path-provisioner`

Kinder currently installs a basic `StorageClass` with provisioner `kubernetes.io/host-path` via `installstorage`. This is the legacy kind default — it does NOT provide dynamic provisioning. Users who create a PVC get a PV only if a matching static PV exists; there is no controller to create PVs on demand.

**Key difference:** local-path-provisioner provides true dynamic provisioning. A user creates a PVC and a PV is automatically created on the scheduled node.

### Table Stakes

| Feature | Why Expected | Complexity | Existing Dependency |
|---------|--------------|------------|---------------------|
| **Dynamic PVC provisioning works out of the box** — create a PVC, get a bound PV immediately | Users expect `kubectl apply -f my-pvc.yaml` to result in a `Bound` PVC. The current `kubernetes.io/host-path` provisioner requires manually pre-creating PVs | LOW | Replace or augment `installstorage` action |
| **`local-path` is the default StorageClass** | No annotation hunting. `kubectl get storageclass` shows `local-path (default)`. Matches K3s behavior — the de facto standard for local dev | LOW | Set `storageclass.kubernetes.io/is-default-class: "true"` annotation on the local-path StorageClass |
| **PVs are created in a stable, predictable path on the node** | Users running StatefulSets need to know where data lands. Default `/opt/local-path-provisioner` is well-known and documented | LOW | Configurable via ConfigMap |
| **Works with single-node and multi-node kind clusters** | Kinder supports multi-node configs. Provisioner must deploy and function on all topologies | LOW | local-path-provisioner DaemonSet (or Deployment) handles this automatically |
| **Volumes survive pod restarts** (on same node) | Core expectation of any persistent storage | LOW | Inherent to `hostPath` volumes |

### Differentiators

| Feature | Value Proposition | Complexity | Existing Dependency |
|---------|-------------------|------------|---------------------|
| **Configurable base path via kinder config** — `addons.localPathProvisioner.nodePath: /my/custom/path` | Teams with specific directory layouts (e.g., host directories via `extraMounts`) need to control where PV data lands | LOW | Add optional field to `Addons` struct |
| **`WaitForFirstConsumer` volume binding mode** | Prevents PVs from being pre-bound to a node before the pod is scheduled, enabling better multi-node scheduling. K8s best practice for node-local storage | LOW | Set in StorageClass definition |
| **Doctor check for CVE-2025-62878** — warn if local-path-provisioner version < v0.0.34 (path traversal, CVSS 10.0) | This critical vulnerability was disclosed January 2026 and affects all versions < 0.0.34. A doctor check provides active security hygiene | LOW | Extends doctor check infrastructure; check provisioner pod image tag |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Presenting local-path-provisioner as production-ready shared storage** | Volumes are node-local: if a node is removed, PVs become orphaned and pods become unschedulable. No capacity limits are enforced | Document clearly: "for local dev only; not HA, not replicated" |
| **Making local-path-provisioner the only StorageClass** | Remove existing `kubernetes.io/host-path` StorageClass entirely | Keep the legacy `standard` StorageClass as a non-default fallback for backward compatibility with existing kinder clusters and CI configs |
| **Adding a second provisioner (e.g., Longhorn, OpenEBS)** for "real" HA storage | Massive complexity; multi-GB images; not a local dev tool concern | Out of scope for v2.2. Recommend Longhorn separately for users who need it |

### Complexity Assessment

**Overall: LOW-MEDIUM.** The provisioner itself installs via a single manifest. The complexity is:
- Auditing the existing `installstorage` action: does it conflict with local-path-provisioner's default StorageClass?
- Ensuring the provisioner image is available in offline mode (if `--offline` flag is implemented)
- The `Addons` struct field addition and ConfigMap customization

### Dependencies on Existing Kinder Features

- **`installstorage` action** — must be modified or replaced; the legacy `standard` StorageClass must coexist without being the default
- **`Addons` struct** — add `LocalPathProvisioner *bool` and optional config subfield
- **Offline feature** (if concurrent) — provisioner image must be in the image manifest

---

## Feature 3: Host Directory Mounting to Pods

### How Host Directory Mounting Works in Kind

There are two layers:

1. **`extraMounts` in kind node config** — mounts a host directory into the kind node container. This makes the host path available inside the node container at `containerPath`. This is already supported in kinder's `v1alpha4` Node type via `ExtraMounts []Mount`.

2. **`hostPath` volumes in pod specs** — once a path is available inside the node, pods can reference it as a `hostPath` volume. The pod spec references the `containerPath` from step 1.

The key insight: there are TWO layers of mounting. Host → kind node container (via `extraMounts`), and kind node container → pod (via `hostPath` volume). Users routinely forget the first layer.

### What "Feature" Means for Kinder

The `ExtraMounts` field already exists in the type system. The work for v2.2 is:
- Making the feature discoverable and documented (UX: `kinder create cluster --help` or config examples)
- Ensuring the doctor command validates mount configuration (paths exist, permissions are correct)
- Optionally: a higher-level config shortcut that wires both layers automatically

### Table Stakes

| Feature | Why Expected | Complexity | Existing Dependency |
|---------|--------------|------------|---------------------|
| **`extraMounts` in node config works end-to-end** — configure a host dir, create a `hostPath` PV pointing to the `containerPath`, create a PVC, use in a pod | Users doing data science, ML, or code-sharing workflows need local data in pods. This is the primary use case | LOW | `ExtraMounts []Mount` already in v1alpha4 types; wiring is in kind's Provision path |
| **Documentation/example: two-layer mount setup** | The two-layer pattern is the #1 confusion point. Users try to use host paths directly in PVs without the `extraMounts` step | LOW | Docs/examples, not code |
| **Custom StorageClass with `reclaimPolicy: Retain`** for host-path-backed PVs | Kind's default `local-path` StorageClass deletes PV contents on PVC deletion. For host-mounted data, users want `Retain` to avoid data loss | LOW | Requires user-created custom StorageClass; kinder can ship an example |

### Differentiators

| Feature | Value Proposition | Complexity | Existing Dependency |
|---------|-------------------|------------|---------------------|
| **`kinder doctor` check: `extraMounts` host path exists** — fail if `hostPath` in node config does not exist or is not readable | Catches "path does not exist" before a 2-minute cluster create fails with an opaque error | LOW | Extends doctor infrastructure; reads node config from cluster config file |
| **`kinder doctor` check: Docker Desktop file sharing** — warn on macOS/Windows if a configured `hostPath` is not in Docker Desktop's allowed sharing list | "If you are using Docker for Mac or Windows check that the hostPath is included in Preferences > Resources > File Sharing" — official kind docs. This is the #2 cause of `extraMounts` failures | MEDIUM | Requires parsing Docker Desktop settings; platform-gated to `darwin`/`windows` |
| **Propagation mode documented** — `None`, `HostToContainer`, `Bidirectional` options with use-case guidance | Users enabling hot-reload workflows (e.g., mounting source code) need `Bidirectional`; users mounting data need `None`. Getting this wrong causes silent breakage | LOW | `Propagation` field already in `Mount` type |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Auto-configuring `hostPath` PVs and PVCs from `extraMounts` config** | Kinder's job is cluster creation, not workload management. PV/PVC lifecycle management is the user's responsibility | Provide example manifests in documentation; do not create PVs automatically |
| **Bypassing the two-layer approach with direct Docker socket mounts** | Gives pods access to the host Docker socket — a container escape vector. Inappropriate for a batteries-included dev tool | Document securely via PV/PVC + hostPath pattern |
| **SELinux relabeling (`selinuxRelabel: true`) as a default** | Modifying file labels on the host filesystem has permanent, sometimes irreversible effects | Keep `selinuxRelabel: false` as default; document as opt-in for SELinux-enabled hosts |

### Complexity Assessment

**Overall: LOW.** The underlying machinery (`ExtraMounts`) already exists in the type system and is handled by kind's provisioner. The work is:
- Two doctor checks (path existence, Docker Desktop sharing)
- Documentation and examples
- Potentially: a higher-level UX shortcut (config field) — this is the only medium-complexity item

### Dependencies on Existing Kinder Features

- **`ExtraMounts []Mount` in `v1alpha4` Node type** — already exists; no type changes needed unless adding shortcuts
- **`kinder doctor` infrastructure** — two new checks slot into existing check registry
- **local-path-provisioner** (Feature 2) — if local-path-provisioner is set to a configurable `nodePath`, that path can overlap with an `extraMount` `containerPath`, creating a powerful pattern: host dir → node mount point → dynamic PV provisioner base path

---

## Feature 4: Multi-Version (Per-Node Kubernetes Version) Clusters

### How Per-Node Versions Work in Kind

Each kind node is a Docker container running `kindest/node:<version>`. The `Node.Image` field in v1alpha4 config can be set per-node to different versions. This is already in kinder's type system.

The Kubernetes version-skew policy (as of K8s v1.28+) allows:
- **kubelet**: up to 3 minor versions older than kube-apiserver
- **kube-apiserver** (HA): maximum 1 minor version skew between instances
- **kube-controller-manager, kube-scheduler**: must not be newer than kube-apiserver; can be up to 1 minor version older

**Practical implication:** In a kinder cluster, you can have a control-plane at v1.32 and workers at v1.29, v1.30, v1.31, or v1.32 — and this is a supported, tested configuration.

**Primary use cases:**
1. Testing Kubernetes operator/controller compatibility across versions during an upgrade simulation
2. CI testing a Helm chart against multiple K8s minor versions simultaneously
3. Testing the upgrade path: bring up an old-version cluster, then simulate node-by-node upgrade

### Table Stakes

| Feature | Why Expected | Complexity | Existing Dependency |
|---------|--------------|------------|---------------------|
| **Per-node `image:` field in v1alpha4 config works correctly** — control-plane at v1.32, workers at v1.30 creates a functioning cluster | This is the core capability. Without it the feature does not exist | LOW | `Node.Image` already in v1alpha4 type; kind handles provisioning; kinder must not override per-node images |
| **`kinder create cluster` does NOT override per-node images when `--image` flag is set** | The current `fixupOptions` in `create.go` overrides ALL node images when `--image` is specified: `for i := range opts.Config.Nodes { opts.Config.Nodes[i].Image = opts.NodeImage }`. This destroys per-node version configs | LOW | **Bug to fix in `fixupOptions`**: `--image` flag should only override nodes that do not already have an explicit `image:` field set |
| **Version-skew validation** — error if worker version is more than 3 minor versions behind control-plane | Creating a cluster that violates Kubernetes version-skew policy will fail at kubeadm join with cryptic errors | MEDIUM | Add validation in `config.Validate()` or `fixupOptions`; parse version from image tag |
| **`kinder get nodes --output json` includes per-node K8s version** | Users inspecting a multi-version cluster need to see which nodes are at which version | LOW | Extend `kinder env` / `kinder get nodes` output to include node image version |

### Differentiators

| Feature | Value Proposition | Complexity | Existing Dependency |
|---------|-------------------|------------|---------------------|
| **`--profile upgrade-test` preset** — control-plane at current-1, one worker at current-2, one worker at current-1, ready for upgrade simulation | The most common multi-version use case is upgrade testing. A preset eliminates manual version-skew arithmetic | MEDIUM | Extends `--profile` flag; must resolve "current" version from default node image at runtime |
| **`kinder doctor` check: version-skew policy compliance** — warn if configured node versions violate 3-minor-version skew | Catches misconfiguration before cluster create. Provides specific guidance (e.g., "worker v1.28 is 4 minor versions behind control-plane v1.32; max skew is 3") | LOW | Extends doctor check infrastructure; parses version from image tags in cluster config |
| **`kinder get nodes` shows `NODE_K8S_VERSION` column** | In a multi-version cluster, `kubectl get nodes` shows VERSION but this is the kubelet version. A dedicated kinder output column makes the image-level version explicit | LOW | Extends existing `get nodes` output |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Allowing control-plane version skew > 1 minor version in HA clusters** | Kubernetes prohibits this; kubeadm will refuse to join. Kinder must enforce, not merely document, this constraint | Validate in `config.Validate()` for HA (>1 control-plane) configs |
| **Automating rolling upgrades** (upgrade node images in a running cluster) | This requires draining nodes, updating containerd images, running kubeadm upgrade — a complex orchestration that is already covered by `kubeadm` docs and upgrade tooling | Document that multi-version clusters are for testing initial skew scenarios, not for running live upgrades via kinder |
| **Supporting version skew in addon installation** | Addon manifests (MetalLB, cert-manager, etc.) are tested against a single K8s version. Installing addons across a skewed cluster may hit API version mismatches | Document: addon installation runs against the control-plane API version; multi-version is primarily a node/workload concern |

### Complexity Assessment

**Overall: LOW-MEDIUM.** The type system already supports this. The work is:
- Fix the `--image` flag override bug in `fixupOptions` (LOW — targeted one-liner fix)
- Add version-skew validation (MEDIUM — requires parsing semver from image tag strings like `kindest/node:v1.31.0@sha256:...`)
- Extend `kinder get nodes` / `kinder env` output (LOW)
- Optional: `--profile upgrade-test` preset (MEDIUM)

The hardest part is robust semver parsing from image tag strings, which may include digest suffixes.

### Dependencies on Existing Kinder Features

- **`Node.Image` field in v1alpha4 type** — already exists; no type changes needed
- **`fixupOptions` in `create.go`** — must be modified to preserve explicit per-node images when `--image` flag is set
- **`config.Validate()` path** — add version-skew validation here
- **`kinder get nodes` / `kinder env`** — extend output

---

## Cross-Feature Dependency Map

```
Feature 1 (Offline)
  └── depends on → LocalRegistry addon (registry mirror endpoint)
  └── depends on → Feature 2 (local-path-provisioner image must be in offline manifest)
  └── depends on → ContainerdConfigPatches (registry mirror wiring)

Feature 2 (local-path-provisioner)
  └── depends on → installstorage action (must coexist or replace)
  └── enables → Feature 3 (if nodePath == extraMount containerPath)

Feature 3 (Host Directory Mounting)
  └── depends on → ExtraMounts in v1alpha4 Node type (already exists)
  └── enhances → Feature 2 (host dir as backing storage for PVCs)

Feature 4 (Multi-Version)
  └── depends on → Node.Image in v1alpha4 Node type (already exists)
  └── conflicts with → current --image flag behavior (bug to fix)
```

---

## Feature Dependencies on Existing Addon System

| Existing Component | Feature 1 | Feature 2 | Feature 3 | Feature 4 |
|-------------------|-----------|-----------|-----------|-----------|
| `Addons` struct `*bool` fields | Add `LocalPathProvisioner *bool` | Add field | No change | No change |
| `installstorage` action | Must skip if offline | Replace/augment | No change | No change |
| `installlocalregistry` action | Reuse as mirror endpoint | Image must be in offline manifest | No change | No change |
| `ContainerdConfigPatches` | Use for registry mirror config | No change | No change | No change |
| `ExtraMounts []Mount` on Node | No change | Enables PV backing | Core mechanism | No change |
| `Node.Image` field | Must handle in image manifest | No change | No change | Core mechanism |
| `fixupOptions` in create.go | No change | No change | No change | Bug fix required |
| `config.Validate()` | No change | No change | No change | Add skew validation |
| `kinder doctor` infrastructure | Add readiness check | Add CVE check | Add path/sharing checks | Add skew check |

---

## MVP Recommendation

Prioritize for v2.2 implementation in this order:

1. **Feature 4 (Multi-Version)** — Lowest code risk. Fix the `--image` override bug and add skew validation. High value for CI/testing workflows. No new addon infrastructure needed.

2. **Feature 2 (local-path-provisioner)** — Medium complexity but standalone. Replaces the legacy `kubernetes.io/host-path` StorageClass with a real dynamic provisioner. Directly improves the batteries-included experience for users with stateful workloads.

3. **Feature 3 (Host Directory Mounting)** — Mostly documentation and two doctor checks. The type system already works. Low risk to ship alongside Feature 2.

4. **Feature 1 (Offline/Air-Gapped)** — Highest complexity. Requires auditing all addon manifests, maintaining an image manifest, and testing in truly isolated environments. Recommend as a standalone sub-milestone rather than bundled with the other three.

**Defer:**
- `--profile upgrade-test` preset: useful but not blocking; ship after core multi-version works
- `kinder images pull/load` commands: high value but scope-creep risk for v2.2; consider v2.3
- Offline profile preset: depends on Feature 1 being complete and stable

---

## Sources

- [kind Working Offline](https://kind.sigs.k8s.io/docs/user/working-offline/) — HIGH confidence
- [kind Configuration (extraMounts, node images)](https://kind.sigs.k8s.io/docs/user/configuration/) — HIGH confidence
- [kind Local Registry](https://kind.sigs.k8s.io/docs/user/local-registry/) — HIGH confidence
- [rancher/local-path-provisioner GitHub](https://github.com/rancher/local-path-provisioner) — HIGH confidence
- [Addressing Limitations of Local Path Provisioner (DEV.to)](https://dev.to/frosnerd/addressing-the-limitations-of-local-path-provisioner-in-kubernetes-3g12) — MEDIUM confidence
- [CVE-2025-62878 (Orca Security)](https://orca.security/resources/blog/cve-2025-62878-rancher-local-path-provisioner/) — HIGH confidence (CVSS 10.0, fixed in v0.0.34)
- [Kubernetes Version Skew Policy](https://kubernetes.io/releases/version-skew-policy/) — HIGH confidence
- [kind Persistent Volumes — Duffie Cooley](https://mauilion.dev/posts/kind-pvc/) — MEDIUM confidence
- [Getting access to host filesystem for PV in Kind (andygol.co.ua)](https://blog.andygol.co.ua/en/2025/04/05/host-fs-to-backup-pv-in-kind/) — MEDIUM confidence
- [KIND: How I Wasted a Day Loading Local Docker Images (iximiuz.com)](https://iximiuz.com/en/posts/kubernetes-kind-load-docker-image/) — MEDIUM confidence
- [Pull-through Docker registry on Kind clusters (maelvls.dev)](https://maelvls.dev/docker-proxy-registry-kind/) — MEDIUM confidence
- [Using Local Path Provisioner — Minikube](https://minikube.sigs.k8s.io/docs/tutorials/local_path_provisioner/) — MEDIUM confidence
- Kinder source code: `pkg/apis/config/v1alpha4/types.go`, `pkg/cluster/internal/create/create.go`, `pkg/cluster/internal/create/actions/installstorage/storage.go`, `pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go` — HIGH confidence (direct code review)
