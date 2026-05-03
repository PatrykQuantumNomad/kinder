# Architecture Patterns: v2.2 Cluster Capabilities

**Domain:** Offline/air-gapped clusters, local-path-provisioner addon, host-to-pod directory mounting, multi-version per-node Kubernetes
**Researched:** 2026-04-08
**Overall confidence:** HIGH (direct codebase analysis + verified against official kind/rancher docs)

---

## Scope

This document answers: how do the four new v2.2 features integrate with kinder's existing architecture? For each feature it identifies:
- Which existing files require modification
- What new files/packages are needed
- Exact data flow through the config pipeline
- Component boundaries

---

## Feature 1: Offline / Air-Gapped Clusters

### What it means

Air-gapped cluster creation skips all network image pulls. The user is responsible for having node images already loaded into the local container runtime (via `docker load`, `kinder load image-archive`, etc.). During `kinder create cluster`, the `ensureNodeImages` step in each provider's `Provision()` must be skipped or converted to a presence-check-only operation.

### Where image pulling happens (current)

Every provider's `Provision()` calls `ensureNodeImages()` before creating containers:

```
pkg/cluster/internal/providers/docker/images.go   → ensureNodeImages() → pullIfNotPresent()
pkg/cluster/internal/providers/podman/images.go   → ensureNodeImages() → pullIfNotPresent()
pkg/cluster/internal/providers/nerdctl/images.go  → ensureNodeImages() → pullIfNotPresent()
```

`pullIfNotPresent()` calls `docker inspect --type=image` first. If the image IS already present locally, it returns without pulling. This means air-gapped works today IF images are pre-loaded — the issue is that if an image is missing, the function tries to pull it (and retries 4 times) instead of returning a clear error.

### Required architecture change

Add an `AirGapped bool` field to `ClusterOptions` in `pkg/cluster/internal/create/create.go`. When `AirGapped = true`, each provider's `ensureNodeImages()` must verify that all required images are present locally and return an error immediately if any is missing, without attempting to pull.

### Config pipeline touch points

Since air-gapped is a creation-time behavioral flag (not a persistent cluster property), it does NOT need to flow through the v1alpha4 config YAML. It is a CLI flag only, matching the precedent of `--retain`, `--wait`, `--image`.

**Approach A (recommended):** Add `AirGapped bool` to `ClusterOptions` struct only. Pass it into the provider via a modified `Provision()` signature or a new `ProvisionOptions` struct.

The current `Provider` interface in `pkg/cluster/internal/providers/provider.go` defines:
```go
Provision(status *cli.Status, cfg *config.Cluster) error
```

The cleanest extension is to add a `ProvisionOptions` struct that wraps the existing arguments and carries the flag:

```go
// pkg/cluster/internal/providers/provider.go
type ProvisionOptions struct {
    AirGapped bool
}

// Provider interface gains ProvisionWithOptions or Provision gains a third param.
// Preferred: new method to avoid breaking existing call sites:
ProvisionWithOptions(status *cli.Status, cfg *config.Cluster, opts ProvisionOptions) error
```

Alternatively, since all three providers already implement the same `ensureNodeImages()` pattern, a simpler approach is to gate the pull in each provider's `Provision()` using a field checked from `cfg` — but that would pollute the cluster config with a runtime flag. The `ProvisionOptions` approach is cleaner.

**Files modified:**
- `pkg/cluster/internal/providers/provider.go` — add `ProvisionOptions` struct, extend Provider interface
- `pkg/cluster/internal/providers/docker/images.go` — `ensureNodeImages()` gains `airGapped bool` param; skip pull and error if image absent
- `pkg/cluster/internal/providers/podman/images.go` — same
- `pkg/cluster/internal/providers/nerdctl/images.go` — same
- `pkg/cluster/internal/providers/docker/provider.go` — `Provision()` passes flag to `ensureNodeImages()`
- `pkg/cluster/internal/providers/podman/provider.go` — same
- `pkg/cluster/internal/providers/nerdctl/provider.go` — same
- `pkg/cluster/internal/create/create.go` — `ClusterOptions` gains `AirGapped bool`; passes to `p.Provision()`
- `pkg/cluster/createoption.go` — add `CreateWithAirGapped(bool)` option function
- `pkg/cmd/kind/create/cluster/createcluster.go` — add `--air-gapped` flag to cobra command

### Interaction with addon images

When `AirGapped = true`, addon manifests that reference images from public registries (MetalLB, Metrics Server, Cert Manager, etc.) will fail to pull those images at pod startup time. The addon actions themselves just apply YAML — they do not pull images. The user must pre-load addon images into the cluster nodes using `kinder load docker-image` or `kinder load image-archive` before or immediately after cluster creation.

Recommended UX: when `AirGapped = true`, print a warning listing all addon images that will need to be pre-loaded. No action system changes needed; the warning lives in `create.go` after the addon profile is resolved.

### Existing `load` commands are already correct

`pkg/cmd/kind/load/docker-image/docker-image.go` and `pkg/cmd/kind/load/image-archive/image-archive.go` both use `nodeutils.LoadImageArchive()` which runs `ctr --namespace=k8s.io images import` inside the node container. This is exactly the right mechanism for pre-baking images. No modifications needed to the load commands for the air-gapped feature.

---

## Feature 2: Local-Path-Provisioner Addon

### What it does

[rancher/local-path-provisioner](https://github.com/rancher/local-path-provisioner) installs a StorageClass named `local-path` that dynamically provisions `hostPath`-backed PersistentVolumes on the node where the pod is scheduled. The provisioner uses a helper pod running `busybox` to set up/tear down directories.

**Images required:**
- `rancher/local-path-provisioner:v0.0.35` (or current release) — the provisioner deployment
- `busybox` — the helper pod (referenced in the `local-path-config` ConfigMap)

**Namespace:** `local-path-storage`

**Default storage path on nodes:** `/opt/local-path-provisioner`

### Integration pattern

This is a pure addon action following the exact same pattern as MetalLB, Metrics Server, and Dashboard. No new infrastructure is needed.

**New package:**
```
pkg/cluster/internal/create/actions/installlocalpath/
    localpathprovisioner.go
    localpathprovisioner_test.go
    manifests/
        local-path-storage.yaml   (embedded via go:embed)
```

The `Execute()` method:
1. Gets control plane node via `nodeutils.ControlPlaneNodes()`
2. Applies `local-path-storage.yaml` via `kubectl --kubeconfig=/etc/kubernetes/admin.conf apply -f -`
3. Waits for `deployment/local-path-provisioner` in `local-path-storage` namespace
4. Optionally patches to set `local-path` as the default StorageClass (replaces the existing `standard` from `installstorage`)

### StorageClass conflict

The existing `installstorage` action installs a `StorageClass` named `standard` (annotated as default). Local-path-provisioner installs `local-path`. If both run, two default StorageClasses exist, which Kubernetes does not reject but which creates ambiguous PVC binding behavior.

**Resolution:** `LocalPath` addon is mutually exclusive with `installstorage`. When `LocalPath = true`:
- Skip `installstorage.NewAction()` in `create.go`'s sequential action list
- Set `local-path` as the default StorageClass in the manifest or via a post-apply patch

The existing `installstorage` action is invoked unconditionally in the sequential pipeline at line `installstorage.NewAction()` in `create.go`. Add a conditional:

```go
// In create.go sequential actions:
if !opts.Config.Addons.LocalPath {
    actionsToRun = append(actionsToRun, installstorage.NewAction())
}
```

### Config pipeline touch points (5 locations)

**1. `pkg/apis/config/v1alpha4/types.go`** — add to `Addons` struct:
```go
LocalPath *bool `yaml:"localPath,omitempty" json:"localPath,omitempty"`
```

**2. `pkg/apis/config/v1alpha4/default.go`** — default to enabled (consistent with other addons):
```go
boolPtrTrue(&obj.Addons.LocalPath)
```

**3. `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go`** — add nil-check copy for `LocalPath *bool` (same pattern as existing `MetalLB`, `NvidiaGPU`, etc.)

**4. `pkg/internal/apis/config/types.go`** — add to internal `Addons` struct:
```go
LocalPath bool
```

**5. `pkg/internal/apis/config/convert_v1alpha4.go`** — add to `Convertv1alpha4()`:
```go
LocalPath: boolVal(in.Addons.LocalPath),
```

**6. `pkg/cluster/internal/create/create.go`** — add to wave1 addons and gate `installstorage`:
```go
wave1 := []AddonEntry{
    ...
    {"Local Path Provisioner", opts.Config.Addons.LocalPath, installlocalpath.NewAction()},
}
// and conditionally skip installstorage (see above)
```

### Air-gapped interaction

For air-gapped clusters, `busybox` and `rancher/local-path-provisioner` must be pre-loaded into each node. The addon action itself does not need modification; the air-gapped warning in `create.go` should include these images when `LocalPath = true`.

---

## Feature 3: Host-to-Pod Directory Mounting

### What this means

Users want to mount a directory from the Docker host machine (macOS/Linux running kinder) into pods running inside the cluster. This is a two-hop mount:

```
Host machine directory
    → ExtraMount into node container (Docker bind mount)
    → hostPath PersistentVolume referencing the mounted path inside the node
    → Pod volume mount
```

### Existing support (what already works)

The config API already fully supports the node-container side:

- `v1alpha4.Node.ExtraMounts []Mount` is already defined in `pkg/apis/config/v1alpha4/types.go`
- `common.GenerateMountBindings()` in `pkg/cluster/internal/providers/common/provision.go` converts `[]Mount` to Docker bind mount flags
- `planCreation()` in each provider's `create.go` calls `GenerateMountBindings(node.ExtraMounts...)` and appends to container args

A user can today write:
```yaml
nodes:
- role: control-plane
  extraMounts:
  - hostPath: /home/user/data
    containerPath: /data
    propagation: HostToContainer
```

This mounts `/home/user/data` from the Docker host into the node container at `/data`. Inside the node, that path is visible to kubelet.

### What is missing

There is no automatic wiring between an `extraMount` and a Kubernetes PersistentVolume/PersistentVolumeClaim. The user must manually create a PV referencing the `containerPath` after cluster creation.

**Feature request is likely:** auto-create PVs for each `extraMount` that has `createPV: true` (or similar annotation), so the host path is immediately usable by pods without manual PV YAML.

### Recommended architecture: new config field + post-init action

Add an optional `CreatePV` boolean to the `Mount` type:

**`pkg/apis/config/v1alpha4/types.go`:**
```go
type Mount struct {
    ContainerPath string           `yaml:"containerPath,omitempty"`
    HostPath      string           `yaml:"hostPath,omitempty"`
    Readonly      bool             `yaml:"readOnly,omitempty"`
    SelinuxRelabel bool            `yaml:"selinuxRelabel,omitempty"`
    Propagation   MountPropagation `yaml:"propagation,omitempty"`
    // CreatePV, if true, automatically creates a hostPath PersistentVolume
    // backed by this mount's containerPath, accessible cluster-wide.
    // +optional (default: false)
    CreatePV bool `yaml:"createPV,omitempty" json:"createPV,omitempty"`
    // PVName is the name for the auto-created PersistentVolume.
    // Defaults to "kinder-pv-<sanitized-containerPath>".
    // +optional
    PVName string `yaml:"pvName,omitempty" json:"pvName,omitempty"`
    // PVCapacity is the storage capacity for the auto-created PV, e.g. "10Gi".
    // Defaults to "10Gi".
    // +optional
    PVCapacity string `yaml:"pvCapacity,omitempty" json:"pvCapacity,omitempty"`
}
```

### New action: installhostmounts

```
pkg/cluster/internal/create/actions/installhostmounts/
    hostmounts.go
    hostmounts_test.go
```

`Execute()` iterates over `ctx.Config.Nodes`, finds all `ExtraMounts` with `CreatePV = true`, and creates PersistentVolume manifests via kubectl. The PV uses `hostPath` pointing to `mount.ContainerPath` (the path inside the node container, which kubelet can access). The PV is not tied to a specific node name — it uses a `nodeAffinity` matching the node where the mount exists.

**Node-to-PV mapping:** The action must correlate the `config.Node` entry to the actual running container name. The existing pattern in `config/config.go` uses `common.MakeNodeNamer("")` and suffix matching — use the same approach.

### Data flow for this feature

```
User config YAML
    extraMounts[i].createPV = true
         |
         v
v1alpha4.Mount.CreatePV parsed
         |
         v
Convertv1alpha4: internal Mount gains CreatePV, PVName, PVCapacity fields
         |
         v
provider.Provision() → container created with bind mount (existing, unchanged)
         |
         v
installhostmounts.Execute() [NEW action in sequential pipeline]
    → for each node with extraMounts[i].CreatePV=true:
        → generate PV YAML with hostPath=mount.ContainerPath, nodeAffinity targeting this node
        → kubectl apply -f - on control plane node
```

**Position in action pipeline:** After `installstorage` (or `installlocalpath`) and before `kubeadmjoin`. The PVs reference node hostPaths that exist once the node containers are running, but before Kubernetes has finished joining workers. Actually, PVs can be created anytime after kubeadm init — place this action after `waitforready` to ensure the API server is accepting resources.

**Files modified:**
- `pkg/apis/config/v1alpha4/types.go` — add `CreatePV`, `PVName`, `PVCapacity` to `Mount`
- `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` — regenerate (string fields, no pointer complexity)
- `pkg/internal/apis/config/types.go` — add same fields to internal `Mount`
- `pkg/internal/apis/config/convert_v1alpha4.go` — `convertv1alpha4Mount()` copies new fields
- `pkg/cluster/internal/create/create.go` — add `installhostmounts.NewAction()` to sequential pipeline (after `waitforready`)

**Files created:**
- `pkg/cluster/internal/create/actions/installhostmounts/hostmounts.go`
- `pkg/cluster/internal/create/actions/installhostmounts/hostmounts_test.go`

### Alternative (simpler) approach

If the auto-PV feature is out of scope for this milestone, the existing `extraMounts` support is sufficient. Document the two-hop mount pattern and provide example YAML showing the PV creation that users must do manually. No code changes needed for this simpler variant.

---

## Feature 4: Multi-Version Per-Node Kubernetes Clusters

### What this means

Each node in the cluster can run a different Kubernetes version by specifying a different node image. Example:

```yaml
nodes:
- role: control-plane
  image: kindest/node:v1.30.0@sha256:...
- role: worker
  image: kindest/node:v1.29.0@sha256:...  # one minor version behind
```

### Existing support (what already works)

Per-node image specification is **already fully supported** by the existing config API and provider layer:

1. `v1alpha4.Node.Image string` — each node has its own image field
2. `planCreation()` in each provider iterates `cfg.Nodes` and uses `node.Image` directly as the container image
3. `common.RequiredNodeImages(cfg)` collects all unique images for the pre-pull step
4. `SetDefaultsNode()` assigns the default image only when `node.Image == ""`

The `--image` CLI flag overrides ALL nodes globally (via `fixupOptions` in `create.go`), but the config file supports per-node images natively.

### What is missing

**Kubeadm version skew enforcement.** When nodes have different Kubernetes versions, kubeadm enforces version skew policy:
- Workers can be at most 3 minor versions behind the control plane
- All control planes must be at the same version (kubeadm does not support mixed-version control planes)

Currently `kubeadminit/init.go` reads the Kubernetes version from the bootstrap control plane node's `/kind/version` file and uses it for all kubeadm operations. For worker nodes with different versions, `kubeadmjoin/join.go` generates join config using the same control plane's version — this is correct behavior (the join config references the control plane's version, not the worker's).

**The actual gap:** Validation. The `Cluster.Validate()` in `pkg/internal/apis/config/validate.go` does not check for version skew compatibility across nodes. A user could accidentally configure an invalid version combination and get a cryptic kubeadm error.

### Recommended architecture change: version skew validation

Add version skew validation in `Cluster.Validate()` after all node images are set:

```go
// In pkg/internal/apis/config/validate.go, Cluster.Validate():
if err := validateNodeVersionSkew(c.Nodes); err != nil {
    errs = append(errs, err)
}
```

`validateNodeVersionSkew()` would:
1. Find all control plane nodes — all must have the same image (same version)
2. Find all worker nodes — their images must be at most 3 minor versions behind the control plane version

**Challenge:** The image string (e.g., `kindest/node:v1.30.0@sha256:abc`) must be parsed for the version. The version is embedded in the tag. The validation should be best-effort: if the image is a non-standard image (e.g., a custom build), skip version skew validation with a warning rather than failing.

**Files modified:**
- `pkg/internal/apis/config/validate.go` — add `validateNodeVersionSkew()` function
- `pkg/internal/version/` — possibly extend the existing version parsing utilities to handle node image tag formats

### Config action change for per-node versions

`config/config.go`'s `getKubeadmConfig()` already reads the Kubernetes version from each individual node via `nodeutils.KubeVersion(node)`. This means each node generates its own kubeadm config using its own version. This is already correct.

The `kubeadm.ConfigData.KubernetesVersion` is set per-node in the loop at the bottom of `getKubeadmConfig()`. No changes needed here.

### Data flow for multi-version

```
User config YAML
    nodes[0].image = "kindest/node:v1.30.0"   # control plane
    nodes[1].image = "kindest/node:v1.29.0"   # worker
         |
         v
Cluster.Validate() → validateNodeVersionSkew() [NEW]
    → parse versions from image tags
    → assert all control planes same version
    → assert workers within 3-minor-version skew
         |
         v
provider.Provision()
    → ensureNodeImages() pulls both images (or verifies locally if air-gapped)
    → planCreation() creates node[0] container with v1.30 image, node[1] with v1.29 image
         |
         v
configaction.Execute()
    → for each node: nodeutils.KubeVersion(node) reads /kind/version from THAT node's container
    → generates kubeadm config with that node's own K8s version
         |
         v
kubeadminit.Execute()
    → reads version from bootstrap control plane (v1.30)
    → runs kubeadm init with v1.30 config
         |
         v
kubeadmjoin.Execute()
    → worker node runs kubeadm join
    → worker uses its own kubelet binary (v1.29 from its image)
    → control plane API is v1.30 — within allowed skew
```

### New CLI flag: --node-image

Consider adding a `--node-image role=image` flag for ad-hoc per-node image override without a config file:

```
kinder create cluster --node-image control-plane=kindest/node:v1.30.0 --node-image worker=kindest/node:v1.29.0
```

This is additive to the existing `--image` (global override). Implementation in `createcluster.go` + `createoption.go`. Medium complexity, reasonable scope for the milestone.

---

## Component Boundary Summary

| Component | File | New / Modified | Feature |
|-----------|------|----------------|---------|
| `ProvisionOptions` struct | `pkg/cluster/internal/providers/provider.go` | MODIFIED (new struct) | Air-gapped |
| `ensureNodeImages()` air-gap gate | `pkg/cluster/internal/providers/docker/images.go` | MODIFIED | Air-gapped |
| `ensureNodeImages()` air-gap gate | `pkg/cluster/internal/providers/podman/images.go` | MODIFIED | Air-gapped |
| `ensureNodeImages()` air-gap gate | `pkg/cluster/internal/providers/nerdctl/images.go` | MODIFIED | Air-gapped |
| `ClusterOptions.AirGapped` | `pkg/cluster/internal/create/create.go` | MODIFIED | Air-gapped |
| `CreateWithAirGapped()` | `pkg/cluster/createoption.go` | MODIFIED (new func) | Air-gapped |
| `--air-gapped` CLI flag | `pkg/cmd/kind/create/cluster/createcluster.go` | MODIFIED | Air-gapped |
| `installlocalpath` package | `pkg/cluster/internal/create/actions/installlocalpath/` | NEW | Local-path-provisioner |
| `Addons.LocalPath` (v1alpha4) | `pkg/apis/config/v1alpha4/types.go` | MODIFIED | Local-path-provisioner |
| `Addons.LocalPath` default | `pkg/apis/config/v1alpha4/default.go` | MODIFIED | Local-path-provisioner |
| `Addons.LocalPath` deepcopy | `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` | MODIFIED | Local-path-provisioner |
| `Addons.LocalPath` (internal) | `pkg/internal/apis/config/types.go` | MODIFIED | Local-path-provisioner |
| `Addons.LocalPath` conversion | `pkg/internal/apis/config/convert_v1alpha4.go` | MODIFIED | Local-path-provisioner |
| `installstorage` gate in pipeline | `pkg/cluster/internal/create/create.go` | MODIFIED | Local-path-provisioner |
| `installlocalpath` in wave1 | `pkg/cluster/internal/create/create.go` | MODIFIED | Local-path-provisioner |
| `Mount.CreatePV` fields (v1alpha4) | `pkg/apis/config/v1alpha4/types.go` | MODIFIED | Host-to-pod mounts |
| `Mount.CreatePV` deepcopy | `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` | MODIFIED | Host-to-pod mounts |
| `Mount.CreatePV` fields (internal) | `pkg/internal/apis/config/types.go` | MODIFIED | Host-to-pod mounts |
| `convertv1alpha4Mount()` | `pkg/internal/apis/config/convert_v1alpha4.go` | MODIFIED | Host-to-pod mounts |
| `installhostmounts` package | `pkg/cluster/internal/create/actions/installhostmounts/` | NEW | Host-to-pod mounts |
| `installhostmounts` in pipeline | `pkg/cluster/internal/create/create.go` | MODIFIED | Host-to-pod mounts |
| `validateNodeVersionSkew()` | `pkg/internal/apis/config/validate.go` | MODIFIED (new func) | Multi-version nodes |

---

## Data Flow: Config Pipeline for New Fields

Every new config field touches 5 locations. The pattern, established by all existing fields:

```
User writes YAML
    → v1alpha4/types.go (external API struct)
    → v1alpha4/default.go (SetDefaultsCluster / SetDefaultsNode)
    → v1alpha4/zz_generated.deepcopy.go (DeepCopy for the pointer/field)
    → internal/apis/config/types.go (internal struct, no serialization tags)
    → internal/apis/config/convert_v1alpha4.go (Convertv1alpha4 + convertv1alpha4Node/Mount)
```

The 6th location is `create.go` where the config field gates or drives behavior.

---

## Suggested Build Order

Dependencies between the 4 features determine build order:

**Phase 1: Multi-version per-node validation (no dependencies)**
- Touches only `validate.go` (new function, no new files needed)
- Prerequisite for: air-gapped (need to validate per-node images exist)
- Config pipeline: validation only, no new fields
- Risk: LOW — isolated to one function in validate.go

**Phase 2: Air-gapped clusters**
- Depends on: provider layer familiarity (same files as phase 1 context)
- Touches: all three provider `images.go` files, `provider.go` interface, `create.go`, `createoption.go`, CLI
- Risk: MEDIUM — touches all three providers, must not break existing image pull behavior

**Phase 3: Local-path-provisioner addon**
- Depends on: none (standalone addon, but benefits from air-gapped being done so the busybox image warning can be included)
- Touches: full 5-location config pipeline + new action package + `create.go` wave1 + `installstorage` gate
- Risk: LOW for the action itself; MEDIUM for the `installstorage` mutual exclusion logic

**Phase 4: Host-to-pod directory mounting (auto-PV)**
- Depends on: local-path-provisioner (provides the StorageClass that PVs will use)
- Touches: `Mount` type in both v1alpha4 and internal config + new action package
- Risk: MEDIUM — `Mount` is used in the provider layer; adding fields must not break existing mount binding

---

## Patterns to Follow

### Pattern 1: New Addon Action (local-path-provisioner)

Follows the `installmetricsserver` template exactly:

```go
package installlocalpath

import (
    _ "embed"
    "strings"

    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/nodeutils"
    "sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/local-path-storage.yaml
var localPathManifest string

type action struct{}

func NewAction() actions.Action { return &action{} }

func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing Local Path Provisioner")
    defer ctx.Status.End(false)

    allNodes, err := ctx.Nodes()
    if err != nil {
        return errors.Wrap(err, "failed to list cluster nodes")
    }
    controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)
    if err != nil {
        return errors.Wrap(err, "failed to find control plane nodes")
    }
    node := controlPlanes[0]

    if err := node.CommandContext(ctx.Context,
        "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "apply", "-f", "-",
    ).SetStdin(strings.NewReader(localPathManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply local-path-provisioner manifest")
    }

    if err := node.CommandContext(ctx.Context,
        "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait", "--namespace=local-path-storage",
        "--for=condition=Available", "deployment/local-path-provisioner",
        "--timeout=120s",
    ).Run(); err != nil {
        return errors.Wrap(err, "local-path-provisioner did not become available")
    }

    ctx.Status.End(true)
    return nil
}
```

### Pattern 2: Provider-Level Feature Flag via ProvisionOptions

Rather than adding fields to `config.Cluster` for runtime-only flags:

```go
// provider.go
type ProvisionOptions struct {
    AirGapped bool
}

// Usage in create.go:
opts := providers.ProvisionOptions{AirGapped: clusterOpts.AirGapped}
if err := p.ProvisionWithOptions(status, opts.Config, opts); err != nil { ... }
```

Each provider's `ensureNodeImages()` receives this flag:

```go
func ensureNodeImages(logger log.Logger, status *cli.Status, cfg *config.Cluster, airGapped bool) error {
    for _, image := range common.RequiredNodeImages(cfg).List() {
        friendlyImageName, image := sanitizeImage(image)
        cmd := exec.Command("docker", "inspect", "--type=image", image)
        if err := cmd.Run(); err == nil {
            continue // image present, skip
        }
        if airGapped {
            return errors.Errorf("air-gapped mode: image %q not present locally; pre-load with: kinder load docker-image %s", friendlyImageName, image)
        }
        // existing pull logic
        status.Start(fmt.Sprintf("Ensuring node image (%s)", friendlyImageName))
        if _, err := pullIfNotPresent(logger, image, 4); err != nil {
            status.End(false)
            return err
        }
    }
    return nil
}
```

### Pattern 3: Config Pipeline Extension (5-location checklist)

Every new config field must touch all 5 locations atomically in the same commit to avoid partial-state breakage. Checklist:

- [ ] `pkg/apis/config/v1alpha4/types.go` — external struct field with yaml/json tags
- [ ] `pkg/apis/config/v1alpha4/default.go` — default value (or omit if zero-value is correct default)
- [ ] `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` — deepcopy for pointer fields
- [ ] `pkg/internal/apis/config/types.go` — internal struct field (no tags)
- [ ] `pkg/internal/apis/config/convert_v1alpha4.go` — conversion function

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Putting Air-Gapped in the Cluster Config YAML

Air-gapped is a creation-time flag, not a persistent cluster property. Adding it to `v1alpha4.Cluster` would mean cluster configs are non-portable (a config with `airGapped: true` would fail on a machine that hasn't pre-loaded images). Keep it as a CLI flag only, like `--retain`.

### Anti-Pattern 2: Modifying kubeadm config generation for multi-version

The `config/config.go` action already reads each node's version from its own `/kind/version` file and generates per-node kubeadm configs. Do not add version inference or override logic at this layer — it already works correctly.

### Anti-Pattern 3: Installing both `standard` and `local-path` StorageClasses as default

Two default StorageClasses cause ambiguous PVC binding. The `installstorage` action must be skipped when `LocalPath` addon is enabled. Do not attempt to "merge" them or use annotation patching to manage default/non-default status post-install.

### Anti-Pattern 4: Hardcoding busybox image in installlocalpath

The `local-path-config` ConfigMap in the manifest controls the helper pod image. For air-gapped compatibility, the manifest should reference the image with `imagePullPolicy: IfNotPresent` (the default in the official manifest). Do not hardcode a different image or add image-override logic in the action — let the user handle it by pre-loading busybox before creation.

### Anti-Pattern 5: Creating PVs before cluster is fully ready

The `installhostmounts` action must run after `waitforready`. The Kubernetes API server must be accepting resources. Placing this action in the sequential pipeline before `waitforready` risks `kubectl apply` failures during control plane startup.

---

## Scalability Considerations

| Concern | Impact | Notes |
|---------|--------|-------|
| Air-gapped + many addon images | User burden | Provide `kinder get addon-images` subcommand in a future milestone to list all images that need pre-loading |
| local-path-provisioner on multi-node clusters | Works correctly | The provisioner schedules on the node where the PVC is bound; no cluster-wide storage distribution |
| Multi-version + air-gapped | Both flags active simultaneously | Must pre-load multiple different `kindest/node` images; warning should list each image by node role |
| ExtraMounts on workers + auto-PV | Correct, needs care | PV must have `nodeAffinity` matching the specific worker node; otherwise a pod on a different node will fail to mount |

---

## Sources

- Direct codebase analysis of `pkg/cluster/internal/create/create.go` (action pipeline, wave structure, ClusterOptions) — HIGH confidence
- Direct codebase analysis of `pkg/cluster/internal/providers/docker/images.go`, `podman/images.go`, `nerdctl/images.go` (pullIfNotPresent pattern) — HIGH confidence
- Direct codebase analysis of `pkg/cluster/internal/providers/common/provision.go` (GenerateMountBindings, ExtraMounts) — HIGH confidence
- Direct codebase analysis of `pkg/cluster/internal/create/actions/` (all existing addon actions for pattern reference) — HIGH confidence
- Direct codebase analysis of `pkg/apis/config/v1alpha4/types.go`, `default.go`, `pkg/internal/apis/config/convert_v1alpha4.go` (config pipeline) — HIGH confidence
- Direct codebase analysis of `pkg/cluster/internal/create/actions/config/config.go` (per-node version reading via KubeVersion) — HIGH confidence
- [kind offline docs](https://kind.sigs.k8s.io/docs/user/working-offline/) — image pre-loading mechanism — HIGH confidence
- [kind configuration docs](https://kind.sigs.k8s.io/docs/user/configuration/) — per-node image and extraMounts support confirmed — HIGH confidence
- [rancher/local-path-provisioner README](https://github.com/rancher/local-path-provisioner) — images, namespace, ConfigMap structure — HIGH confidence (verified via official repo)
- [k3s air-gap + local-path busybox issue](https://github.com/k3s-io/k3s/issues/1908) — confirms busybox is a required pre-load for air-gapped local-path-provisioner — MEDIUM confidence (community issue, behavior consistent with official docs)
