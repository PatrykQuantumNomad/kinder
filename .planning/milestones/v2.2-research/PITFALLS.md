# Domain Pitfalls: v2.2 Cluster Capabilities

**Domain:** Offline/air-gapped clusters, local storage provisioning, host directory mounting, and multi-version node support — added to an existing kind fork (kinder)
**Researched:** 2026-04-08
**Confidence:** HIGH for image loading mechanics, local-path-provisioner offline failure, StorageClass collision, mount propagation macOS, and kubeadm per-node patch limits (verified against kind issue tracker, rancher/local-path-provisioner issues, and kinder codebase); MEDIUM for pre-baking approaches and multi-version addon compatibility (synthesized from kind design docs and community sources); LOW for kubeadm v1beta4 migration timing specifics (single source, needs monitoring)

---

## Critical Pitfalls

### Pitfall 1: local-path-provisioner Helper Pod Pulls busybox at Runtime — Breaks Offline Mode

**What goes wrong:**
The local-path-provisioner uses a separate helper pod (configured in its ConfigMap) to create and delete PV directories on the node. By default this helper pod pulls `busybox` from Docker Hub at PVC create/delete time with `imagePullPolicy: Always`. In an offline cluster, every PVC creation will hang with the helper pod stuck in `ImagePullBackOff`. The provisioner deployment itself may be healthy (image pre-loaded), but storage provisioning is broken. This is a two-image problem: both the provisioner image AND the busybox helper image must be offline-available.

**Why it happens:**
The busybox dependency is embedded in the provisioner's ConfigMap (`config.json`), not its Deployment manifest. Developers pre-loading only the provisioner image miss the helper image entirely. The ConfigMap `helperPod.yaml` key specifies the image, and the default is `busybox:latest` with `imagePullPolicy: Always` — the worst combination for offline use.

**Consequences:**
All PVC operations silently fail. StatefulSets remain Pending indefinitely. The provisioner pod shows `Running` so no obvious error surface. Users discover the issue only when a workload tries to claim storage.

**Prevention:**
- When pre-baking images into the kinder node image for offline use, include ALL images required by every addon at runtime — not just the addon controller itself. For local-path-provisioner this means: the provisioner image AND `busybox` (or a configurable substitute).
- The provisioner ConfigMap must set `imagePullPolicy: IfNotPresent` for the helper pod. Do not use the upstream manifest verbatim; patch it before embedding in `go:embed`.
- Allow the helper image to be configurable via kinder's addon config so users in restricted environments can point to a mirrored image.
- When implementing the `kinder load images` command, document that users must load the helper image explicitly, not just the provisioner.

**Detection:**
- PVC stuck in `Pending` state while provisioner pod is `Running`
- Helper pod `create-pvc-XXXXX` in `kube-system` showing `ImagePullBackOff`
- `kubectl describe pod create-pvc-XXXXX -n kube-system` shows `Failed to pull image "busybox:latest": ...no such host`

**Phase to address:** The local-path-provisioner addon phase. The go:embed manifest must be patched before embedding, not applied verbatim. Do this before any integration testing of the feature.

---

### Pitfall 2: ctr images import Fails on Multi-Platform Images from Docker with Containerd Image Store Enabled

**What goes wrong:**
The existing `kinder load docker-image` command calls `docker save` then pipes through `ctr --namespace=k8s.io images import --all-platforms --digests --snapshotter=...`. This works when Docker uses its classic image store. When Docker Desktop 27+ has "Use containerd for pulling and storing images" enabled (now the default in many versions), `docker save` produces a tar with attestation manifests and multi-platform index references pointing to blobs that were not included in the export. `ctr import` then fails with:

```
ctr: content digest sha256:<hash>: not found
```

The image was exported from Docker, looks valid, but containerd's strict digest validation rejects it.

**Why it happens:**
Docker's new containerd image store exports the full OCI manifest list including attestation layers and all-platform references. The `ctr import --all-platforms` flag attempts to import every referenced platform, but the export only included the host platform's actual layers. The digest references for other platforms exist in the manifest but their content blobs are absent.

**Consequences:**
`kinder load images` fails for standard images (nginx, busybox) that exist in multi-platform form. This is the primary failure mode for the new load-images command and affects all users on Docker Desktop 27+.

**Prevention:**
- The `kinder load images` command should attempt `--local` mode for the import (containerd 2.x flag that relaxes blob availability requirements) as a fallback when `--all-platforms` fails.
- Alternatively, use `--platform linux/amd64` (or current host platform) instead of `--all-platforms` for the import step. The `--all-platforms` flag in `ctr import` is not useful here — kind nodes only need the current platform's image.
- Look at the existing `getSnapshotter()` + `parseSnapshotter()` logic in `pkg/cluster/nodeutils/util.go` which already handles config versions 2 and 3; the import command must be similarly version-aware.
- Document that Docker Desktop users should disable the containerd image store if they experience load failures, as a known workaround.

**Detection:**
- `kinder load images` fails with `content digest ... not found`
- `docker info | grep "Storage Driver"` shows `overlay2` but `docker info | grep "containerd"` shows containerd-backed store
- Issue occurs on Docker Desktop 27+ and Docker Engine with `"features": {"containerd-snapshotter": true}` in daemon.json

**Phase to address:** The `kinder load images` command implementation phase. The fallback import strategy must be implemented before the command is declared complete. Do not rely solely on `--all-platforms` without a fallback.

---

### Pitfall 3: StorageClass Name Collision — local-path-provisioner vs Existing "standard" StorageClass

**What goes wrong:**
Kinder already installs a `StorageClass` named `standard` via the `installstorage` action (see `pkg/cluster/internal/create/actions/installstorage/storage.go` — it reads `/kind/manifests/default-storage.yaml` from the node image, which defines a StorageClass named `standard` with `storageclass.kubernetes.io/is-default-class: "true"`). Adding local-path-provisioner as an addon installs ANOTHER StorageClass, also annotated as the default. The result is two default StorageClasses. Kubernetes behavior with two default StorageClasses is undefined: some versions pick the first found, others fail PVC binding, others add the annotation to newly created StorageClasses automatically. Users get confusing errors like `more than one default StorageClass found` depending on their Kubernetes version.

**Why it happens:**
The existing `installstorage` action is kind's built-in action and runs unconditionally (it is not one of kinder's opt-out addons). When local-path-provisioner is added as a kinder addon, it ships with its own default-annotated StorageClass. Neither action is aware of the other.

**Consequences:**
PVC binding becomes non-deterministic. On Kubernetes 1.26+, the admission webhook enforces at most one default StorageClass per cluster and starts blocking new ones. On older versions, both StorageClasses exist silently and some PVCs bind to the old host-path provisioner (which doesn't actually provision volumes dynamically) rather than local-path-provisioner.

**Prevention:**
- The local-path-provisioner addon action must check for and remove the `standard` StorageClass installed by `installstorage` before applying its own manifest, OR patch the local-path-provisioner's StorageClass to not be annotated as default and instead rename the existing `standard` StorageClass's provisioner.
- Preferred approach: the local-path-provisioner addon replaces `installstorage` entirely. Since kinder addons are opt-out, the local-path-provisioner addon should be enabled by default and the `installstorage` action should be skipped when local-path-provisioner is enabled. This requires a check in `create.go` where both actions are registered.
- The go:embed manifest for local-path-provisioner should use the name `standard` for its StorageClass for backward compatibility, or provide a migration that patches the existing `standard` StorageClass to use the new provisioner.

**Detection:**
- `kubectl get storageclass` shows two entries both annotated `(default)`
- PVCs bind inconsistently or fail with `no persistent volumes available`
- New PVCs go to `host-path` provisioner instead of `rancher.io/local-path` provisioner

**Phase to address:** Local-path-provisioner addon phase. Define the StorageClass ownership model before writing the action — decide upfront whether local-path-provisioner replaces or coexists with the existing storage action.

---

### Pitfall 4: Mount Propagation Breaks Silently on macOS/Windows — HostToContainer and Bidirectional Are No-ops

**What goes wrong:**
When kinder implements host-to-pod directory mounting via config, the natural implementation is to use `ExtraMounts` on the node with `propagation: HostToContainer`. This works on Linux (native Docker). On macOS and Windows, Docker Desktop runs inside a Linux VM — the host filesystem and the container filesystem are in different kernel namespaces. Mount propagation requires the two kernels to share mount events, which is impossible across the VM boundary. The `propagation` field is silently ignored; the host directory still appears in the container (Docker Desktop syncs it) but new sub-mounts created after start do not propagate. The cluster appears to create successfully, but the mount semantics are wrong — and the issue is invisible because `kinder create cluster` reports success.

**Why it happens:**
Docker Desktop emulates bind mounts by synchronizing filesystem state (via virtioFS or gRPC FUSE) rather than true kernel mount propagation. The `propagation` field in the container spec is passed to Docker but Docker Desktop silently drops or ignores it. Kind's own documentation warns about this (PR #2974) but the fix was a documentation update, not a runtime error.

**Consequences:**
If kinder's host directory mounting feature defaults to `propagation: HostToContainer`, all macOS/Windows users get silently broken mount semantics. Workloads requiring dynamic sub-mounts (like container runtime sockets, FUSE filesystems) fail unpredictably. Worse: the cluster creates without error so users assume the feature works.

**Prevention:**
- When implementing the v1alpha4 config for host directory mounting, use `propagation: None` (the default) unless the user explicitly requests otherwise.
- Add a validation step: if the user specifies `propagation: HostToContainer` or `propagation: Bidirectional`, emit a warning on non-Linux hosts (check `runtime.GOOS`) that mount propagation is not supported on this platform and the mount will be created without propagation.
- Do NOT silently downgrade the propagation setting — warn explicitly and proceed, so users understand what is happening.
- This validation should be integrated into kinder's existing platform detection code (similar to the macOS MetalLB warning).

**Detection:**
- No error during `kinder create cluster` on macOS
- `docker exec -it <node> ls <mountpath>` shows contents but new sub-mounts don't appear
- User confusion when hostPath volumes work for static files but fail for dynamic mount scenarios

**Phase to address:** Host directory mounting phase. The platform check must be implemented before the feature is marked complete — it is not optional.

---

## Moderate Pitfalls

### Pitfall 5: Pre-baking Images into a Node Image Requires a Privileged Build Container — Standard Dockerfile FROM Cannot Do It

**What goes wrong:**
The intuitive approach to pre-baking addon images for offline use is to write a Dockerfile that uses `kindest/node:vX.Y.Z` as the base and runs `ctr images import` during the build. This fails because `ctr` requires the containerd daemon running (`/run/containerd/containerd.sock`), which is not available during a standard `docker build`. The Dockerfile approach cannot start containerd, so `ctr` commands fail with `connection refused`.

**Why it happens:**
Containerd is a daemon that must be running to import images into its content store. `docker build` runs each layer in a rootfs snapshot without daemon services. Kind's own `imageimporter.go` (in `pkg/build/nodeimage/`) handles this by launching containerd inside a privileged build container via `bash -c "nohup containerd > /dev/null 2>&1 &"` and waiting for the socket before running `ctr import`. This is NOT possible in a standard multi-stage Dockerfile.

**Consequences:**
If `kinder build node-image --with-images` is implemented naively via Dockerfile layers, the build silently fails to import images or fails with `containerd.sock` errors. The resulting image is created but the pre-baked images are absent — discoverable only at cluster creation time when the offline environment cannot pull them.

**Prevention:**
- Follow kind's existing approach in `pkg/build/nodeimage/`: use a privileged build container (not a Dockerfile layer) that starts containerd, waits for the socket, imports images via `ctr`, then stops containerd and commits the container as an image.
- The `kinder build node-image` command (if extended to support `--with-images`) must use `docker create` + `docker start --privileged` + `containerd start` + `ctr import` + `docker commit` — not `docker build`.
- Alternative: implement `kinder load images` as a post-creation command that loads images into a running cluster (simpler than pre-baking), and recommend pre-baking only for true airgap scenarios where the cluster has no post-creation network access.
- If pre-baking is required, add a step that verifies images are actually present in the node image's containerd store (`ctr --namespace=k8s.io images list`) before declaring the build successful.

**Detection:**
- Build completes without error but `docker run --rm kindest-custom ctr --namespace=k8s.io images list` shows no addon images
- Cluster creation succeeds but addon pods show `ImagePullBackOff` immediately

**Phase to address:** Offline image pre-baking phase. Validate the build approach with a test that runs the built image and lists containerd images before any integration testing.

---

### Pitfall 6: kubeadm KubeletConfiguration Patches Apply Cluster-Wide, Not Per-Node — v1alpha4 Per-Node Patches Have a Hidden Limit

**What goes wrong:**
When implementing multi-version node support, a natural instinct is to use `kubeadmConfigPatches` at the node level in v1alpha4 to configure each node's kubelet for its specific Kubernetes version. However, kubeadm reads `KubeletConfiguration` only from the first control-plane node (during `kubeadm init`) and broadcasts it to all nodes via a ConfigMap. Worker node `kubeadm join` ignores `KubeletConfiguration` in the per-node config. This means node-level kubelet patches in `kubeadmConfigPatches` are silently dropped for all non-init-node nodes.

**Why it happens:**
This is a known upstream kubeadm design limitation (kubernetes/kubeadm#1682). Kubeadm treats `KubeletConfiguration` as cluster-wide even though the kubelet technically supports per-node config. The kind codebase's `kubeadmjoin/join.go` reads the join config from `/kind/kubeadm.conf` which is generated per-node — but the `KubeletConfiguration` within it is ignored by kubeadm join.

**Consequences:**
Users configuring multi-version clusters with per-node kubelet tuning (e.g., different `featureGates` per node) will find their worker node patches silently discarded. This is not a kinder bug but users will report it as one. The issue is particularly relevant for multi-version testing where different Kubernetes versions may require different kubelet flags.

**Prevention:**
- Document clearly in kinder's multi-version feature: `KubeletConfiguration` patches only apply to the first control-plane node. Worker-node kubelet configuration must be changed post-creation via `kubectl edit cm kubelet-config -n kube-system` or node-level kubelet restarts.
- `InitConfiguration` and `ClusterConfiguration` patches DO apply correctly per-node via kubeadm patches; the limitation is specific to `KubeletConfiguration`.
- For multi-version node support, the primary configuration needed is the per-node `image` field (different `kindest/node:vX.Y.Z` images) — which DOES work correctly in v1alpha4 and is the right mechanism.

**Detection:**
- Worker node kubelet feature gates don't match what was specified in `kubeadmConfigPatches`
- `kubectl get cm kubelet-config -n kube-system -o yaml` shows control-plane config, not worker config

**Phase to address:** Multi-version cluster phase. Mention in user-facing docs and any generated config examples that node-level `KubeletConfiguration` patches are ignored on worker nodes.

---

### Pitfall 7: kubeadm Configuration API Version Mismatch in Multi-Version Clusters — v1beta3 vs v1beta4 Diverge Across Node Image Versions

**What goes wrong:**
Kind currently generates kubeadm config in `v1beta3` format for all nodes. Kubernetes v1.31+ ships kubeadm that supports (and prefers) `v1beta4`. Kubernetes v1.36+ will remove `v1beta3` support. When a multi-version cluster has a control-plane at v1.36+ and workers at v1.33, the control-plane's kubeadm will reject the `v1beta3` join config and warn (or fail) with:

```
WARNING: Ignored YAML document with GroupVersionKind kubeadm.k8s.io/v1beta3, Kind=JoinConfiguration
```

In practice today this is a warning, not a hard failure, but it becomes a hard failure when v1beta3 is removed.

**Why it happens:**
Kind's `kubeadminit` and `kubeadmjoin` actions generate a single kubeadm config file format regardless of the node's Kubernetes version. When joining a node that runs a newer kubeadm binary that has deprecated v1beta3, the generated config format may not match what that specific kubeadm version expects. The mismatch is per-node because different node images ship different kubeadm binaries.

**Consequences:**
Multi-version clusters spanning the v1beta3/v1beta4 boundary (roughly v1.31/v1.36) will generate warnings today and will fail entirely post-v1.36. This is a forward-compatibility time bomb.

**Prevention:**
- The kubeadm config generation must be version-aware per node. Query the node's kubeadm version (`kubeadm version -o short` inside the node container) and select the appropriate `apiVersion` for the generated config.
- For v2.2 scope (multi-version clusters, not necessarily spanning the v1beta4 boundary), document the known versions where this transition occurs and add a validation warning if the requested node version combination would span the boundary.
- This is the same problem kind upstream faces — monitor the kind project's releases for how they handle the v1beta4 migration.

**Detection:**
- `kinder create cluster` logs contain `WARNING: Ignored YAML document...`
- Cluster creation fails on Kubernetes 1.36+ nodes with `unknown apiVersion: kubeadm.k8s.io/v1beta3`

**Phase to address:** Multi-version cluster phase. Add a version-range validation that warns users when their requested node version combination includes a node at or above the v1beta4-required threshold.

---

### Pitfall 8: Addons Applied During Cluster Creation Are Not Offline-Safe by Default — Addon Manifests Reference Images That Must Be Pre-Loaded

**What goes wrong:**
Kinder's addon manifests are embedded via `go:embed` and applied via `kubectl apply` during cluster creation — the manifests themselves are offline-capable. However, the IMAGES referenced in those manifests are not pre-loaded into the node. In a live cluster, Kubernetes pulls them from the internet. In an offline cluster, every addon pod fails with `ImagePullBackOff`. The current addon list references several external registries:
- MetalLB: `quay.io/metallb/...`
- Envoy Gateway: `docker.io/envoyproxy/...`
- Metrics Server: `registry.k8s.io/metrics-server/...`
- Headlamp: `ghcr.io/headlamp-k8s/...`
- cert-manager: `quay.io/jetstack/...`
- local-path-provisioner: `docker.io/rancher/...`
- local-path-provisioner helper: `docker.io/library/busybox`

An offline cluster needs ALL of these pre-loaded into every node's containerd store.

**Why it happens:**
The project decision to apply addons at runtime via kubectl (rather than baking them into the node image) was correct for the online use case and keeps the node image small. But it creates a gap for offline: manifest application succeeds (embedded YAML) while pod scheduling fails (missing images).

**Consequences:**
Offline cluster creation appears to succeed (`kinder create cluster` exits 0) but the cluster is non-functional — all addon pods remain in `ImagePullBackOff`. The user has no indication this happened unless they run `kubectl get pods -A` manually.

**Prevention:**
- The `kinder load images` command should accept a `--from-addons` flag (or equivalent) that automatically resolves the current set of addon images from their embedded manifests and loads them all in one operation.
- The offline mode should add a post-addon-apply readiness check that inspects pod status and distinguishes `ImagePullBackOff` (missing images, offline failure) from `CrashLoopBackOff` (config issues) — and surfaces the former as a clear "did you run `kinder load images` before creating this offline cluster?" message.
- Document a canonical "offline setup" workflow: (1) `kinder export addon-images addon-images.tar`, (2) transfer to airgap machine, (3) `kinder load images addon-images.tar`, (4) `kinder create cluster --offline`.

**Detection:**
- All addon pods in `ImagePullBackOff` immediately after cluster creation
- `kubectl describe pod <addon-pod> | grep "Failed to pull image"` shows external registry URLs
- Cluster creation exits 0 but `kinder doctor` post-creation check fails

**Phase to address:** Offline/air-gapped phase. The image export/load workflow must be designed before implementation begins, as it affects both the `kinder load images` command design and the offline cluster creation flow.

---

### Pitfall 9: Per-Node Version Skew Constraints Are Hard to Validate Upfront — Version Range Errors Surface Only at kubeadm join Time

**What goes wrong:**
Kubernetes enforces that `kubelet` version may be at most N minor versions older than the API server (N=3 since Kubernetes 1.28, N=2 before that). When a user configures a multi-version cluster in v1alpha4 with a control-plane at v1.34 and a worker at v1.30, the combination exceeds the allowed skew. Kinder proceeds with cluster creation, provisioning node containers with the requested images, until `kubeadm join` fails with an inscrutable error about version mismatch deep in the kubeadm logs. The user may see a generic "failed to join node with kubeadm" error with no clear indication that the node image versions are incompatible.

**Why it happens:**
Kinder has no pre-flight version skew validation. The per-node `image` field is treated as an opaque string. Version extraction from image tags (e.g., `kindest/node:v1.30.0`) is possible but not implemented. The error surfaces inside `kubeadmjoin.go`'s `runKubeadmJoin()` which wraps the kubeadm output but the relevant version skew error is buried in `--v=6` output.

**Consequences:**
Users get a confusing failure partway through cluster creation. The control-plane nodes are provisioned, addons may have started, but workers fail to join. Cleanup is needed. The error message is not user-friendly.

**Prevention:**
- Parse node image tags to extract the Kubernetes version (the `vX.Y.Z` component from `kindest/node:vX.Y.Z` or `kindernode:vX.Y.Z`). This is reasonable to do at config validation time.
- In the config validation step, compute the maximum version across all control-plane nodes and the minimum version across all worker nodes. If the delta exceeds 3 minor versions (or 2 for Kubernetes < 1.28), fail fast with a clear error: `worker node v1.30 is 4 minor versions behind control-plane v1.34; maximum allowed skew is 3`.
- If the image tag does not follow the `vX.Y.Z` pattern (custom images), skip the validation and proceed — don't block on unparseable versions.
- Add a `kinder validate config` command (or wire into `kinder create cluster` preflight) that runs this check before provisioning any containers.

**Detection:**
- `kinder create cluster` fails with "failed to join node with kubeadm"
- `--v=6` logs show kubeadm version skew errors

**Phase to address:** Multi-version cluster phase. Add version-skew validation before implementing the full cluster creation flow.

---

## Minor Pitfalls

### Pitfall 10: local-path-provisioner Doesn't Clean Up PVs When Nodes Are Deleted — Orphaned PV References Block Future Clusters

**What goes wrong:**
When a kinder cluster is deleted and recreated, any PersistentVolume references to the deleted node's local path directory remain in etcd until the provisioner's cleanup helper pod runs. Since the cluster is deleted, the helper pod never runs. If the same cluster name and node names are reused (the common case for `kinder create cluster`), the next cluster's provisioner finds stale PV references and can enter a confused state.

**Prevention:**
- This is a local-path-provisioner limitation, not a kinder bug. Document it.
- `kinder delete cluster` should include a step that invokes provisioner cleanup before destroying nodes — or at minimum document that users should run `kubectl delete pvc --all -A` before deleting a cluster that used local-path-provisioner.
- The data directory on the host (default: `/tmp/kinder-data` or similar) persists after cluster deletion. Document this and consider adding a `--clean-storage` flag to `kinder delete cluster`.

**Phase to address:** Local-path-provisioner addon phase.

---

### Pitfall 11: load images Command Targets All Nodes by Default — Slow for Multi-Node Clusters

**What goes wrong:**
The existing `kinder load docker-image` command loads images to ALL nodes in the cluster by default. For a 5-node multi-version test cluster, loading 10 addon images means 50 `ctr import` operations. Each is sequential per-node (concurrent across nodes). For large addon images (Envoy Gateway is ~200MB), this can take several minutes. The `--nodes` flag allows targeting specific nodes, but users must know which nodes need which images.

**Prevention:**
- The `kinder load images --from-addons` workflow should automatically target only nodes that are missing the image (the existing smart-load logic in the load command already does this — keep it).
- Document expected load time for the full addon image set so users don't think the command is hung.
- For multi-version clusters, some addon images may not be compatible with older node versions — document which addons support which Kubernetes versions to avoid loading images that will never be used.

**Phase to address:** `kinder load images` command phase.

---

### Pitfall 12: ExtraMounts Host Path Must Exist Before Cluster Creation — No Auto-Creation

**What goes wrong:**
When implementing host directory mounting via v1alpha4 config, if the user specifies a `hostPath` that doesn't exist on the host, the Docker/Podman container creation will fail with a bind mount error. Kind does not create the host directory automatically. The error is a provider-level container creation failure, not a kinder validation error — the message is obscure.

**Prevention:**
- During cluster creation, after config parsing but before node provisioning, validate that all `hostPath` values in the user's mount config exist on the host. If not, fail with a clear message: `hostPath "/data/myapp" does not exist; create it before running kinder create cluster`.
- Add this check to kinder's existing preflight chain (similar to how kinder doctor validates prerequisites).

**Phase to address:** Host directory mounting phase.

---

### Pitfall 13: kinder load images Hardcodes docker save — Fails with Podman and Nerdctl Providers

**What goes wrong:**
The existing `kinder load docker-image` command calls `exec.Command("docker", "save", ...)` directly. Kinder supports three container runtimes (docker, podman, nerdctl). For podman and nerdctl users, `docker save` does not exist or is not the correct command. The `load docker-image` command silently uses the wrong tool.

**Prevention:**
- The `kinder load images` command (new) and any image export functionality must use the provider-abstracted exec mechanism, not hardcoded `docker` calls. Check how the existing common provider package handles runtime-specific commands and follow the same pattern.
- The `docker-image.go` implementation in the existing load command already has this flaw — the new `kinder load images` command must not inherit it. Use `runtime.GetDefault(logger)` to get the current provider and call the appropriate save command.

**Phase to address:** `kinder load images` command phase. Verify provider compatibility before marking complete.

---

## Cross-Feature Integration Pitfalls

### Integration Pitfall 1: Offline Mode + local-path-provisioner Creates a Two-Phase Image Dependency

**What goes wrong:**
An offline cluster using local-path-provisioner requires images pre-loaded BEFORE cluster creation. But kinder's `kinder load images` command loads images into a RUNNING cluster. The offline cluster cannot pull images to start provisioner pods, but `kinder load images` needs a running cluster to import images into node containers.

**Resolution approach:**
The implementation must support TWO modes:
1. **Pre-create load** (`kinder build node-image --with-images` or equivalent): bake images into the node image before cluster creation — for true airgap where no connectivity exists at any point
2. **Post-create load** (`kinder load images`): load images into a running cluster — for "load from tarball" scenarios where the machine has the images but no registry

Mode 2 is simpler to implement and covers most cases. Mode 1 requires the privileged build approach (Pitfall 5). Document both clearly and address in the correct phases.

**Phase to address:** Design this two-mode approach as a documented decision before starting implementation.

---

### Integration Pitfall 2: Multi-Version Clusters + Addons — Some Addons Require Minimum Kubernetes Version

**What goes wrong:**
Kinder installs addons assuming a single Kubernetes version across all nodes. In a multi-version cluster where the control-plane is at v1.28 but a worker is at v1.24, some addons may fail. Notably:
- MetalLB dropped endpoint support (v1.21+), requires EndpointSlices — fine for 1.24+
- Envoy Gateway 1.x requires Kubernetes 1.25+ for Gateway API v1 CRDs
- cert-manager v1.16+ requires Kubernetes 1.22+
- Metrics Server with `--kubelet-insecure-tls` works across all tested versions

The issue is that addons are applied to the control-plane but serve the entire cluster including older worker nodes.

**Prevention:**
- For v2.2, multi-version clusters are primarily for Kubernetes version skew testing — users testing upgrade paths from v1.28 to v1.31. The addon minimum versions should not be a practical blocker for this use case (all tested versions are recent enough).
- Add a note in documentation that multi-version clusters are validated with node version ranges within the supported 3-minor-version skew window, and addon compatibility outside that window is not tested.

**Phase to address:** Multi-version cluster phase documentation.

---

### Integration Pitfall 3: Host Directory Mounting + local-path-provisioner — Don't Collide on /tmp

**What goes wrong:**
local-path-provisioner defaults to storing PV data in `/opt/local-path-provisioner` inside the node container (which maps to a path on the host Docker filesystem layer). If a user also configures an `extraMount` that binds a host directory to `/opt/local-path-provisioner`, the mount shadows the provisioner's storage directory and PV operations fail or become non-isolated between clusters.

**Prevention:**
- Document that `/opt/local-path-provisioner` is the reserved path for local-path-provisioner and should not be used as a target for user ExtraMounts.
- If kinder's host-mounting config generates a path on the same node, validate that user-specified mount targets do not overlap with known addon paths.

**Phase to address:** Can be addressed in either the host-mounting or local-path-provisioner phase, whichever comes first.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| local-path-provisioner addon | busybox helper image not pre-loaded (Pitfall 1) | Patch ConfigMap's `imagePullPolicy` to `IfNotPresent` before go:embed |
| local-path-provisioner addon | StorageClass collision with existing `standard` SC (Pitfall 3) | Define ownership model first; likely replace installstorage when addon is enabled |
| kinder load images command | Multi-platform ctr import failure on Docker Desktop 27+ (Pitfall 2) | Implement `--local` fallback or platform-specific import |
| kinder load images command | Hardcoded `docker save` breaks podman/nerdctl (Pitfall 13) | Use provider abstraction from `runtime.GetDefault()` |
| Offline image pre-baking | ctr import needs running containerd daemon (Pitfall 5) | Use privileged container commit, not Dockerfile |
| Offline image pre-baking | All addon images must be loaded, not just controller images (Pitfall 8) | Implement `--from-addons` image resolution |
| Host directory mounting | Propagation breaks silently on macOS/Windows (Pitfall 4) | Emit platform warning; default to `propagation: None` |
| Host directory mounting | Missing host directory gives obscure error (Pitfall 12) | Pre-flight validation of all hostPath values |
| Multi-version clusters | Version skew exceeds limit — surfaces at kubeadm join (Pitfall 9) | Config validation before provisioning |
| Multi-version clusters | KubeletConfiguration patches silently ignored on workers (Pitfall 6) | Document clearly; it's an upstream kubeadm limitation |
| Multi-version clusters | kubeadm v1beta3/v1beta4 boundary (Pitfall 7) | Version-aware config generation; warn on boundary-spanning configs |
| Offline + local-path-provisioner | Two-phase image dependency (Integration Pitfall 1) | Design pre-create vs post-create load modes explicitly |

---

## Sources

- kind issue tracker: image loading failures ([#3795](https://github.com/kubernetes-sigs/kind/issues/3795), [#3996](https://github.com/kubernetes-sigs/kind/issues/3996), [#2402](https://github.com/kubernetes-sigs/kind/issues/2402)) — HIGH confidence
- kind issue tracker: mount propagation macOS ([#2576](https://github.com/kubernetes-sigs/kind/issues/2576), [#2400](https://github.com/kubernetes-sigs/kind/issues/2400), [#2700](https://github.com/kubernetes-sigs/kind/issues/2700)) — HIGH confidence
- kind issue tracker: local-path-provisioner ([#2697](https://github.com/kubernetes-sigs/kind/issues/2697), [#3191](https://github.com/kubernetes-sigs/kind/issues/3191)) — HIGH confidence
- kind issue tracker: per-node kubeadm patches ([#1424](https://github.com/kubernetes-sigs/kind/issues/1424)) — HIGH confidence (kubeadm upstream limitation confirmed)
- rancher/local-path-provisioner: air-gap busybox ([k3s-io/k3s#1908](https://github.com/k3s-io/k3s/issues/1908), [k3s-io/k3s#2391](https://github.com/k3s-io/k3s/issues/2391)) — HIGH confidence
- kind offline documentation: https://kind.sigs.k8s.io/docs/user/working-offline/ — HIGH confidence
- kind known issues: https://kind.sigs.k8s.io/docs/user/known-issues/ — HIGH confidence
- kinder codebase: `pkg/cluster/nodeutils/util.go` LoadImageArchive + getSnapshotter (verified locally) — HIGH confidence
- kinder codebase: `pkg/cluster/internal/create/actions/installstorage/storage.go` (verified locally) — HIGH confidence
- kinder codebase: `pkg/build/nodeimage/imageimporter.go` privileged containerd start approach (verified locally) — HIGH confidence
- Kubernetes version skew policy: https://kubernetes.io/releases/version-skew-policy/ — HIGH confidence
- kubeadm v1beta4 blog post (Kubernetes v1.31): https://kubernetes.io/blog/2024/08/23/kubernetes-1-31-kubeadm-v1beta4/ — MEDIUM confidence (v1beta3 removal timeline is "1.34 or later", exact version TBD)
- iximiuz.com imagePullPolicy pitfall: https://iximiuz.com/en/posts/kubernetes-kind-load-docker-image/ — MEDIUM confidence (single source, but consistent with kind docs)
- DEV Community local-path limitations: https://dev.to/frosnerd/addressing-the-limitations-of-local-path-provisioner-in-kubernetes-3g12 — MEDIUM confidence
