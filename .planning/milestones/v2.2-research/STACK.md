# Technology Stack

**Project:** kinder -- v2.2 Cluster Capabilities (offline clusters, local-path-provisioner, host-dir mounts, multi-version nodes)
**Researched:** 2026-04-08
**Overall confidence:** HIGH -- All four features use Go stdlib + already-present dependencies. No new external dependencies required.

---

## Executive Summary: What Actually Changes

This milestone adds four capabilities to kinder. After thorough codebase analysis and dependency verification, the finding is the same as the previous milestone: **zero new go.mod dependencies are required**. Every feature integrates using packages already present in the codebase.

| Feature | Stack Impact |
|---------|-------------|
| Offline/air-gapped clusters (`kinder build bake-images` + `kinder load images`) | New subcommand using existing `exec`, `fs`, `errors` packages + existing `containerdImporter` pattern |
| local-path-provisioner addon | New addon action using `//go:embed` + `kubectl apply` pattern (identical to MetalLB, cert-manager) |
| Host-to-pod directory mounting via v1alpha4 | Zero new dependencies -- `ExtraMounts []Mount` already exists on `Node`; feature is a v1alpha4 config field + kinder doctor check |
| Multi-version per-node Kubernetes | Zero new dependencies -- `Node.Image` already accepts any `kindest/node` image; feature is validation + UX in create command |

---

## Recommended Stack

### Existing Packages -- Reuse for All Four Features

| Package | Already In go.mod | Purpose in New Features |
|---------|-------------------|------------------------|
| `sigs.k8s.io/kind/pkg/exec` | YES | Shell out to `docker save`, `docker load`, `ctr images import` for image baking and `kinder load images` |
| `sigs.k8s.io/kind/pkg/fs` | YES | `fs.TempDir` for staging image tarballs during `kinder load images` (same as existing `load docker-image`) |
| `sigs.k8s.io/kind/pkg/errors` | YES | `errors.UntilErrorConcurrent` for parallel node image loading (same pattern as existing load subcommand) |
| `sigs.k8s.io/kind/pkg/build/nodeimage` (internal) | YES | `containerdImporter` in `imageimporter.go` -- the exact mechanism for baking images into node images |
| `sigs.k8s.io/kind/pkg/cluster/nodeutils` | YES | `LoadImageArchive` for `kinder load images` (already used by `load docker-image` and `load image-archive`) |
| `sigs.k8s.io/kind/pkg/internal/version` | YES | `ParseSemantic`, `LessThan` for per-node version skew validation |
| `encoding/json` (stdlib) | YES | Parse `docker image inspect` output for image ID extraction |
| `os` (stdlib) | YES | File I/O for image tarballs |
| `_ "embed"` (stdlib) | YES (used in 7 addons) | `//go:embed manifests/local-path-storage.yaml` for local-path-provisioner manifest |

### Feature 1: Offline/Air-Gapped Clusters

**Approach:** Two-part implementation.

**Part A -- `kinder build bake-images IMAGE [IMAGE...]` subcommand:**
Pre-bakes additional images into a `kindest/node` base image using `docker run` + `ctr images import` + `docker commit`. This is the exact pattern already in `pkg/build/nodeimage/buildcontext.go` (`prePullImagesAndWriteManifests` + `containerdImporter`). The new command wraps that mechanism for ad-hoc image baking.

Implementation path:
1. New `pkg/cmd/kind/build/bakeimages/` package (sibling of existing `nodeimage/`)
2. Uses `containerdImporter` from `pkg/build/nodeimage/imageimporter.go` (may need to be made exportable or replicated)
3. Flow: `docker run <base-image>` → start containerd → `ctr import` each image tar → `docker commit` → output new image tag

**Part B -- `kinder load images IMAGE [IMAGE...]` subcommand (runtime loading):**
Loads images from host Docker daemon into running cluster nodes. This is identical to the existing `load docker-image` command in `pkg/cmd/kind/load/docker-image/`. The only addition is a convenience alias `kinder load images` that resolves multiple image names, calls `docker save`, then pipes to `nodeutils.LoadImageArchive` on each node.

No new dependencies. The existing `load docker-image` code is the blueprint.

**Part C -- Containerd offline mode config:**
For true air-gapped operation, containerd must be configured not to attempt pulls (`imagePullPolicy: Never` is pod-level, but containerd-level requires no additional code change -- it is enforced by the node image having all images pre-baked and Kubernetes `imagePullPolicy: IfNotPresent`). The existing `ContainerdConfigPatches` mechanism handles any required containerd registry mirror configuration.

**Part D -- `kinder doctor` checks for offline readiness:**
Uses existing `exec` + `crictl`/`ctr` pattern from `pkg/cluster/nodeutils/util.go` to verify that required images are present on nodes before cluster creation.

### Feature 2: local-path-provisioner Addon

**Important codebase finding:** The `kindest/node` base image already ships a `local-path-provisioner` variant as its storage driver. `pkg/build/nodeimage/const_storage.go` defines `docker.io/kindest/local-path-provisioner:v20260213-ea8e5717` and `docker.io/kindest/local-path-helper:v20260131-7181c60a`. The existing `installstorage` action in `create.go` applies this at cluster creation from the manifest baked into the node image.

**What the new addon adds:** An opt-in `LocalPathProvisioner` addon backed by upstream `rancher/local-path-provisioner:v0.0.35`, as opposed to the kindest-flavored version baked into the node image. This is useful when the user wants the canonical Rancher distribution on a cluster that was not built with `kinder build node-image`.

Implementation:
1. New `pkg/cluster/internal/create/actions/installlocalpath/` package
2. `//go:embed manifests/local-path-storage.yaml` embedding the upstream manifest
3. Manifest sources `rancher/local-path-provisioner:v0.0.35` and `busybox` as helper
4. Follows identical pattern to `installcertmanager`, `installmetallb`: `kubectl apply -f -` via `controlPlane.Command`
5. Add `LocalPathProvisioner *bool` to `v1alpha4.Addons` struct (default opt-in: `true` using `boolVal`)
6. Add `LocalPathProvisioner bool` to internal `config.Addons`
7. Wire into wave 1 in `create.go` (independent of other addons -- no dependency)

**Manifest version:** `rancher/local-path-provisioner:v0.0.35` (released 2026-03-10). This is the latest upstream release and includes the fix for CVE-2025-62878.

**No new external dependencies.** The manifest is `//go:embed` static content. `kubectl apply` is called inside the node container via `controlPlane.Command`, same as every other addon.

### Feature 3: Host-to-Pod Directory Mounting via v1alpha4

**Codebase finding:** This feature is already partially implemented. The `v1alpha4.Node.ExtraMounts []Mount` field exists, is fully parsed, and is wired through `docker create` args via `common.GenerateMountBindings`. The `Mount` struct has `HostPath`, `ContainerPath`, `Readonly`, `SelinuxRelabel`, `Propagation` fields. The docker provider already calls `filepath.Abs` to resolve relative host paths.

**What is NOT yet present:** The `ExtraMounts` field is on the Node (i.e., it mounts into the Docker container that IS the node). To mount a host directory into a pod running INSIDE the node, the user must:
1. Mount the host path into the node container via `Node.ExtraMounts` (already works)
2. Define a `hostPath` PersistentVolume or pod `volumes.hostPath` that references the container path

**Stack impact:** No new packages needed. The implementation is:
1. Documentation and examples showing the two-step pattern (existing `ExtraMounts` on node + pod `hostPath` volume referencing the container path)
2. Optionally: a new `v1alpha4.Cluster.HostMounts []HostMount` field at the cluster level that auto-propagates a mount to ALL nodes (for the common case where the user wants the same directory accessible from any pod). This uses the same `Mount` struct and same docker args generation.
3. A `kinder doctor` check validating that `ExtraMounts.HostPath` paths exist on the host (uses `os.Stat` -- stdlib only)

**No new dependencies.** Pure struct field additions + existing path handling.

### Feature 4: Multi-Version Per-Node Kubernetes

**Codebase finding:** This feature is already supported at the data model level. `v1alpha4.Node.Image` accepts any image string, and different nodes can have different `kindest/node` images at different versions. The `fixupOptions` function in `create.go` only overrides all node images when `--image` flag is set globally; per-node images in config are preserved.

**What is NOT yet present:**
1. Validation that warns when nodes have different Kubernetes versions (currently silent)
2. UX helper to resolve `kindest/node:vX.Y.Z` to the SHA-pinned digest for a given version
3. `kinder doctor` check that validates version skew between control plane and worker nodes does not exceed +/- 2 minor versions (Kubernetes version skew policy)

**Stack impact:**
- Version comparison uses existing `pkg/internal/version.ParseSemantic` + `LessThan` / `AtLeast` -- no new dependency
- SHA digest resolution uses `exec.Command("docker", "image", "inspect", "-f", "{{.RepoDigests}}", image)` -- no new dependency
- A `--image` flag per-node CLI override is additive to existing cobra command structure

**No new dependencies.** The feature is validation logic + UX over what already works in the data model.

---

## go.mod Changes Required

**None.** All four features are implementable with the current dependency set.

```
# No changes to go.mod required for this milestone.
# All packages used are already present:
# - golang.org/x/sync v0.19.0  (errgroup for parallel node loading)
# - sigs.k8s.io/yaml v1.4.0    (config parsing)
# - github.com/spf13/cobra v1.8.0 (new subcommands)
# - github.com/pelletier/go-toml v1.9.5 (already used in nodeutils.go)
# - go:embed (stdlib, Go 1.16+; already used in 7 addon actions)
```

---

## Rejected Dependencies

| Evaluated | Decision | Reason |
|-----------|----------|--------|
| `github.com/rancher/local-path-provisioner` (as a Go library) | REJECTED | Only need the YAML manifest, not the Go API. `//go:embed` the static manifest. |
| `github.com/opencontainers/image-spec` | REJECTED | `docker/buildx/internal/container/docker/archive.go` already handles OCI tar parsing via stdlib `archive/tar`; existing pattern in `GetArchiveTags` is sufficient |
| `github.com/google/go-containerregistry` | REJECTED | Needed if pulling from registries without Docker CLI. All offline operations go through `docker save`/`ctr import`, both of which are shelled out. No need for a Go OCI library. |
| `github.com/Masterminds/semver` | REJECTED | `pkg/internal/version.ParseSemantic` + `LessThan` handles all version comparison needs for per-node version skew validation |
| Any OCI registry client library | REJECTED | Offline feature explicitly avoids registry interactions; `ctr content fetch` is the containerd CLI path used during node image build only |

---

## Integration Points with Existing Architecture

### 1. `kinder build bake-images` integrates with `pkg/build/nodeimage/`

The `containerdImporter` in `imageimporter.go` (lines 26-93) is the exact mechanism needed:
- `Prepare()` -- starts containerd in build container
- `Pull(image, platform)` -- pulls via `ctr content fetch`
- `LoadCommand()` -- pipes tar via `ctr images import`
- `Tag(src, target)` -- retags via `ctr images tag`

The bake-images command wraps this: spin up a `docker run` container from the base node image, call `containerdImporter`, then `docker commit`.

### 2. `kinder load images` integrates with `pkg/cmd/kind/load/`

`nodeutils.LoadImageArchive(node, reader)` is already the bottleneck function. The new `kinder load images` command is a thin wrapper: collect image names, `docker save` into a tempdir tar, open the file, call `LoadImageArchive` on each selected node concurrently via `errors.UntilErrorConcurrent`. This is identical to what `load docker-image` does (lines 173-193 in `docker-image.go`).

### 3. local-path-provisioner addon integrates with wave 1 in `create.go`

New `AddonEntry{"Local Path Provisioner", opts.Config.Addons.LocalPathProvisioner, installlocalpath.NewAction()}` is added to `wave1` slice. No wave dependency -- local-path-provisioner is independent of MetalLB, Envoy, cert-manager.

Coordination note: The `installstorage` action already installs a storage class from the node image manifest. The new addon either replaces this (if `LocalPathProvisioner: true`) or coexists. Recommended: make the new addon replace `installstorage` when enabled, to avoid duplicate `standard` StorageClass. This requires a config check in `installstorage` -- no new package.

### 4. Host mounts extend `v1alpha4.Cluster` without touching the docker provider

A new `HostMounts []Mount` field at `Cluster` level (not `Node` level) is auto-propagated in `fixupOptions` or `Convertv1alpha4`:
```go
// In convertv1alpha4Node, after existing ExtraMounts conversion:
out.ExtraMounts = append(out.ExtraMounts, clusterLevelHostMounts...)
```
The docker provider's existing `runArgsForNode` → `common.GenerateMountBindings` handles the rest. Zero provider code changes.

### 5. Multi-version validation fits in `config.Validate()` or `fixupOptions()`

The internal `config.Cluster` already has `Nodes[]Node` with `Node.Image`. A new `validateNodeVersionSkew()` function calls `nodeutils.KubeVersion` semantics (parses `kindest/node:vX.Y.Z` tag) and checks skew. Uses `version.ParseSemantic` already in use throughout. No structural changes to the create flow.

---

## Platform Compatibility

| Feature | Linux | macOS | Windows | Notes |
|---------|-------|-------|---------|-------|
| `kinder build bake-images` | YES | YES | NO | Requires `docker run` + `ctr`; Windows has no containerd in build container |
| `kinder load images` | YES | YES | YES | Pure Docker CLI + node exec; same as existing `load docker-image` |
| Host-to-pod directory mounts | YES | LIMITED | NO | macOS requires host path in Docker Desktop file sharing; Windows named pipes differ |
| Multi-version per-node | YES | YES | YES | Data model only; cross-platform |
| local-path-provisioner addon | YES | YES | YES | kubectl apply inside node; cross-platform |

---

## Manifest Version for local-path-provisioner

| Component | Version | Image | Source |
|-----------|---------|-------|--------|
| local-path-provisioner | v0.0.35 | `rancher/local-path-provisioner:v0.0.35` | [GitHub release 2026-03-10](https://github.com/rancher/local-path-provisioner/releases/tag/v0.0.35) |
| helper pod | latest | `busybox` (no pinned tag in upstream manifest) | Embedded in manifest |

**Note on pinning busybox:** The upstream manifest uses unpinned `busybox`. For kinder's embedded manifest, pin to `busybox:1.37.0` or use `docker.io/library/busybox:musl` to avoid latest-tag surprises. This is a manifest edit, not a Go dependency change.

**Note on CVE-2025-62878:** This path traversal vulnerability was fixed in v0.0.34. Using v0.0.35 (latest, released after the CVE fix) is the correct choice.

---

## Sources

- Kinder codebase direct reads: `pkg/build/nodeimage/imageimporter.go`, `buildcontext.go`, `const_storage.go`, `pkg/cmd/kind/load/docker-image/docker-image.go`, `pkg/cluster/nodeutils/util.go`, `pkg/cluster/internal/create/create.go`, `pkg/apis/config/v1alpha4/types.go`, `pkg/internal/apis/config/convert_v1alpha4.go` (HIGH confidence -- primary source)
- Kind offline docs: https://kind.sigs.k8s.io/docs/user/working-offline/ -- confirmed pre-built node image + `kind load` approach (HIGH confidence -- official kind docs)
- Kind configuration docs: https://kind.sigs.k8s.io/docs/user/configuration/ -- confirmed `Node.Image` per-node field + `ExtraMounts` (HIGH confidence -- official kind docs)
- local-path-provisioner v0.0.35 release: https://github.com/rancher/local-path-provisioner/releases/tag/v0.0.35 -- released 2026-03-10 (HIGH confidence -- official GitHub release)
- CVE-2025-62878 advisory: https://orca.security/resources/blog/cve-2025-62878-rancher-local-path-provisioner/ -- fixed in v0.0.34 (MEDIUM confidence -- third-party security report)
- go.mod current state: confirmed `github.com/pelletier/go-toml v1.9.5` (TOML parsing, used in `nodeutils.go`), `golang.org/x/sync v0.19.0` (errgroup), `github.com/spf13/cobra v1.8.0` (subcommands) -- all present (HIGH confidence -- direct file read)

---
*Stack research for: kinder v2.2 -- offline clusters, local-path-provisioner, host mounts, multi-version nodes*
*Researched: 2026-04-08*
