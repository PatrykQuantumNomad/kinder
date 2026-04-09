# Phase 43: Air-Gapped Cluster Creation - Research

**Researched:** 2026-04-09
**Domain:** Go CLI, container runtime image inspection, offline cluster provisioning
**Confidence:** HIGH

## Summary

Phase 43 adds a `--air-gapped` flag to `kinder create cluster` so that users working in networks without internet access can create clusters reliably. The two key behaviours are: (1) fail fast with a complete missing-image list instead of hanging on failed pulls, and (2) warn users proactively before they switch to offline mode so they know what to pre-load.

The implementation divides cleanly into four concern areas: (a) propagating an `AirGapped bool` option through `ClusterOptions` → `Provider.Provision`, (b) replacing the retry-pull logic in all three provider `ensureNodeImages` functions with a presence-only check that accumulates missing images, (c) computing the full set of required images (node images + LB image + each enabled addon's images) in a single shared utility, and (d) adding a `kinder doctor` check (`offline-readiness`) that compares that same image list against local presence.

The existing `working-offline.md` page already covers the manual "docker save / docker load" workflow. Phase 43 extends it with a kinder-native two-mode guide (pre-create bake via privileged container commit vs post-create load via `kinder load images`).

**Primary recommendation:** Add `AirGapped bool` to `ClusterOptions`, add `RequiredAddonImages(cfg)` to `pkg/cluster/internal/providers/common/images.go`, have each provider's `ensureNodeImages` branch on the flag, and register a new `offline-readiness` doctor check.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `sigs.k8s.io/kind/pkg/exec` | existing | Run container runtime CLI commands | Used by all existing provider/doctor code |
| `sigs.k8s.io/kind/pkg/errors` | existing | Error aggregation and wrapping | `errors.NewAggregate` already used across codebase |
| `sigs.k8s.io/kind/pkg/internal/sets` | existing | Deduplicate image lists | Used by `RequiredNodeImages` |
| `text/tabwriter` | stdlib | Aligned table output for missing-image list | Used by clusterskew doctor check |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/internal/cli` | existing | `cli.Status` spinner for UX | Same as current ensureNodeImages pattern |
| `sigs.k8s.io/kind/pkg/log` | existing | Logger passed through from command | Consistent with existing providers |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Shared `RequiredAddonImages` utility | Hardcode in each provider | Utility keeps image list in one place; providers are thin |
| Accumulate missing images before returning error | Return first missing image | Accumulate is better UX: user fixes everything in one pass |

## Architecture Patterns

### Recommended Project Structure

New/changed files for Phase 43:

```
pkg/
├── cmd/kind/create/cluster/createcluster.go        # add --air-gapped flag → CreateWithAirGapped
├── cluster/
│   ├── createoption.go                              # add CreateWithAirGapped option
│   └── internal/
│       ├── create/
│       │   └── create.go                            # pass AirGapped through ClusterOptions → Provision
│       └── providers/
│           ├── provider.go                          # add AirGapped bool to ProvisionOptions (or pass through cfg)
│           ├── common/
│           │   └── images.go                        # add RequiredAddonImages(cfg) function
│           ├── docker/
│           │   └── images.go                        # branch ensureNodeImages on air-gapped mode
│           ├── podman/
│           │   └── images.go                        # branch ensureNodeImages on air-gapped mode
│           └── nerdctl/
│               └── images.go                        # branch ensureNodeImages on air-gapped mode
└── internal/
    └── doctor/
        └── offlinereadiness.go                      # new: offline-readiness check
site/content/docs/user/
└── working-offline.md                               # extend: add two-mode workflow section
```

### Pattern 1: Propagating AirGapped Through the Stack

**What:** The flag flows from the CLI flag through `CreateOption` → `ClusterOptions.AirGapped` → `config.Cluster.AirGapped` OR via a `ProvisionOptions` struct added to the `Provider` interface.

**Recommended approach:** Add `AirGapped bool` directly to `config.Cluster` (the internal config type) so that `Provision(status, cfg)` already carries the flag. This avoids changing the `Provider` interface signature, which would require updating all three providers simultaneously for unrelated reasons.

Alternatively, add to `ClusterOptions` only and thread it into `Provision` via a wrapper — but since `config.Cluster` is already the carrier for all other cluster-wide settings (`Addons`, `Networking`), adding it there is the most consistent pattern.

**Example pattern (based on existing `Addons` field in `config.Cluster`):**
```go
// pkg/internal/apis/config/types.go
type Cluster struct {
    // ... existing fields ...
    AirGapped bool  // disables image pulls; cluster creation fails fast if images are missing
}
```

```go
// pkg/cluster/createoption.go
func CreateWithAirGapped(airGapped bool) CreateOption {
    return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
        o.AirGapped = airGapped
        return nil
    })
}
```

```go
// pkg/cluster/internal/create/create.go — in fixupOptions or directly
opts.Config.AirGapped = opts.AirGapped
```

### Pattern 2: Fast-Fail ensureNodeImages in Air-Gapped Mode

**What:** Each provider's `ensureNodeImages` currently calls `pullIfNotPresent` which retries up to 4 times. In air-gapped mode, skip the pull entirely and accumulate missing images.

**When to use:** Whenever `cfg.AirGapped == true`.

**Key insight:** The check for local presence is already inside `pullIfNotPresent` — it calls `<runtime> inspect --type=image <image>`. In air-gapped mode we reuse that same inspect call but skip the pull path.

```go
// pkg/cluster/internal/providers/docker/images.go — air-gapped variant
func ensureNodeImages(logger log.Logger, status *cli.Status, cfg *config.Cluster) error {
    if cfg.AirGapped {
        return checkImagesPresent(cfg)
    }
    // ... existing retry-pull logic ...
}

func checkImagesPresent(cfg *config.Cluster) error {
    allRequired := requiredAllImages(cfg)
    var missing []string
    for _, image := range allRequired.List() {
        cmd := exec.Command("docker", "inspect", "--type=image", image)
        if err := cmd.Run(); err != nil {
            missing = append(missing, image)
        }
    }
    if len(missing) > 0 {
        return formatMissingImagesError(missing)
    }
    return nil
}
```

**Note:** The podman provider uses `podman inspect --type=image` and the nerdctl provider uses `<binaryName> inspect --type=image` — the same pattern already exists in `pullIfNotPresent` in each respective file.

### Pattern 3: RequiredAddonImages Utility

**What:** A new `RequiredAddonImages(cfg *config.Cluster) sets.String` function in `pkg/cluster/internal/providers/common/images.go` that returns the full set of images required beyond the node image (LB image + each enabled addon's images).

**Why:** The LB image is in `loadbalancer.Image` const. Addon images are hardcoded in their manifests. A single source-of-truth function allows both the provider fast-fail check AND the doctor offline-readiness check to use the same list without duplication.

**Complete image list as of the current codebase:**

| Image | Source | Addon |
|-------|--------|-------|
| `docker.io/kindest/haproxy:v20260131-7181c60a` | `loadbalancer.Image` const | LB (multi-CP only) |
| `registry:2` | `localregistry.registryImage` const | LocalRegistry addon |
| `quay.io/metallb/controller:v0.15.3` | metallb-native.yaml | MetalLB addon |
| `quay.io/metallb/speaker:v0.15.3` | metallb-native.yaml | MetalLB addon |
| `registry.k8s.io/metrics-server/metrics-server:v0.8.1` | components.yaml | MetricsServer addon |
| `quay.io/jetstack/cert-manager-cainjector:v1.16.3` | cert-manager.yaml | CertManager addon |
| `quay.io/jetstack/cert-manager-controller:v1.16.3` | cert-manager.yaml | CertManager addon |
| `quay.io/jetstack/cert-manager-webhook:v1.16.3` | cert-manager.yaml | CertManager addon |
| `docker.io/envoyproxy/ratelimit:ae4cee11` | install.yaml | EnvoyGateway addon |
| `envoyproxy/gateway:v1.3.1` | install.yaml | EnvoyGateway addon |
| `ghcr.io/headlamp-k8s/headlamp:v0.40.1` | headlamp.yaml | Dashboard addon |
| `nvcr.io/nvidia/k8s-device-plugin:v0.17.1` | nvidia-device-plugin.yaml | NvidiaGPU addon |

**Challenge:** Addon image references live inside embedded YAML manifests. The cleanest approach is to define a Go constant or `var` for each addon's image list alongside the manifest embedding, then reference those from `RequiredAddonImages`. This avoids YAML parsing at runtime and keeps image references as first-class Go symbols.

```go
// Example: pkg/cluster/internal/create/actions/installmetallb/metallb.go
// Add alongside the existing //go:embed directive:
var Images = []string{
    "quay.io/metallb/controller:v0.15.3",
    "quay.io/metallb/speaker:v0.15.3",
}
```

```go
// pkg/cluster/internal/providers/common/images.go
func RequiredAddonImages(cfg *config.Cluster) sets.String {
    images := sets.NewString()
    // LB image is always needed when cluster has multiple control planes
    // (cannot be determined without full config; include it always for simplicity,
    // or check config.ClusterHasImplicitLoadBalancer)
    images.Insert(loadbalancer.Image)
    if cfg.Addons.LocalRegistry {
        images.Insert("registry:2")  // or import the const from installlocalregistry
    }
    if cfg.Addons.MetalLB {
        images.Insert(installmetallb.Images...)
    }
    // ... etc for each addon ...
    return images
}
```

**Import cycle warning:** `common/images.go` already imports `config`. The new function will also need to import the addon action packages. Check for import cycles: addon packages import `actions` (not `common`). `common` does NOT currently import any addon package. Adding imports of addon packages from `common` would create an import cycle IF those addon packages import `common`.

Looking at the code: `installmetallb` imports `actions`, `nodeutils`, `errors` — NOT `common`. So adding `var Images` in `installmetallb` and importing `installmetallb` from `common/images.go` would create a cycle only if `common` imports `installmetallb` AND `installmetallb` imports `common`. Currently `installmetallb` does NOT import `common`, so this is safe.

**Alternative (safer if import cycles are a concern):** Define addon image lists in a separate `pkg/cluster/internal/create/images/` package that has no upstream dependencies, and import from both `common/images.go` and the addon action packages.

### Pattern 4: Addon Image Warning (Non–Air-Gapped Mode)

**What:** When `--air-gapped` is NOT set, warn users before creation about images that will be pulled for enabled addons.

**Where:** In `Cluster()` in `pkg/cluster/internal/create/create.go`, immediately after `fixupOptions` and before `p.Provision`, log the list of addon images that will be pulled.

```go
// Before Provision, if NOT air-gapped:
if !opts.Config.AirGapped && hasEnabledAddons(opts.Config.Addons) {
    addonImages := common.RequiredAddonImages(opts.Config)
    logger.V(0).Info("NOTE: The following addon images will be pulled during cluster creation.")
    logger.V(0).Info("      Pre-load these images to use --air-gapped mode:")
    for _, img := range addonImages.List() {
        logger.V(0).Infof("  %s", img)
    }
}
```

### Pattern 5: Doctor offline-readiness Check

**What:** New `pkg/internal/doctor/offlinereadiness.go` implementing the `Check` interface. Lists which required images are absent from the local image store.

**Challenge:** The doctor package cannot import `pkg/cluster/internal/providers/common` (import cycle: `doctor` is imported by `create.go` in `pkg/cluster/internal/create/create.go`). Therefore the doctor check must either:
1. Re-implement the image list inline (fragile — duplicates the list), OR
2. Move the canonical image list to a `pkg/internal/airgap` package that has no upstream dependencies and can be imported by both `common/images.go` and `doctor/offlinereadiness.go`.

**Recommended approach:** Create `pkg/internal/airgap/images.go` with the canonical image constants and the `RequiredImages(cfg)` function. This package imports only `config` (internal types) and `sets`, avoiding cycles.

**Pattern based on existing clusterskew check:**
```go
// pkg/internal/doctor/offlinereadiness.go
type offlineReadinessCheck struct {
    inspectImage func(image string) bool  // injected for testing
}

func newOfflineReadinessCheck() Check {
    return &offlineReadinessCheck{
        inspectImage: realInspectImage,
    }
}

func (c *offlineReadinessCheck) Name() string       { return "offline-readiness" }
func (c *offlineReadinessCheck) Category() string    { return "Offline" }
func (c *offlineReadinessCheck) Platforms() []string { return nil }

func (c *offlineReadinessCheck) Run() []Result {
    // Use a hardcoded list of ALL possible images (all addons enabled)
    // or accept a config — but doctor.RunAllChecks() takes no config.
    // Must list ALL possible images (node image is user-specified, so skip it
    // and focus on infra/addon images that are version-pinned).
    ...
}
```

**Key constraint:** `doctor.RunAllChecks()` takes no arguments. The check cannot be parameterised on user config. Therefore the offline-readiness check should check for ALL known addon images (treating every addon as potentially enabled), and report which are absent. Users can ignore absent images for addons they don't intend to enable.

**Detection command:** Same as pullIfNotPresent:
```
<runtime> inspect --type=image <image>
```
Return code 0 = present, non-zero = absent. This is identical to the approach in all three `pullIfNotPresent` implementations.

**Output format (table, consistent with clusterskew):**
```
MISSING IMAGE                                 REQUIRED BY
----------------------------------            -----------
quay.io/metallb/controller:v0.15.3           MetalLB addon
quay.io/metallb/speaker:v0.15.3              MetalLB addon
```

### Anti-Patterns to Avoid

- **Parsing embedded YAML at runtime to extract image references:** Fragile, adds YAML parsing dependency, and breaks if manifest format changes. Use Go constants instead.
- **Changing the `Provider.Provision` signature:** Would require updating all three providers and the interface in one diff; the `cfg.AirGapped` field approach is less disruptive.
- **Silently skipping pulls in air-gapped mode:** Must fail fast and loudly with the complete missing-image list. A silent skip would let the cluster creation proceed until containerd tries to pull the image inside the node and fails with a confusing network error.
- **Checking only node images in air-gapped mode:** Addon images also need to be pre-loaded. The failure mode for missing addon images is much harder to diagnose (addon install hangs or fails with Kubernetes-level errors, not a clear Docker pull error).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Image presence detection | Custom HTTP registry ping | `<runtime> inspect --type=image` | Already used in all three `pullIfNotPresent` implementations; works for all runtimes |
| Error aggregation | Manual `strings.Builder` loop | `errors.NewAggregate` or simple `[]string` + formatted error | Consistent with existing validate.go pattern |
| Set deduplication | `map[string]bool` manual | `sets.NewString()` | Already imported by `common/images.go` |
| Table formatting | Manual padding | `text/tabwriter` | Used by clusterskew.go for exactly this purpose |

## Common Pitfalls

### Pitfall 1: Import Cycle Between doctor and providers/common
**What goes wrong:** Adding `RequiredAddonImages` to `providers/common/images.go` and then importing it from `doctor` creates a cycle because `create.go` already imports `doctor` (`doctor.ApplySafeMitigations`).
**Why it happens:** The dependency graph is: `cmd` → `cluster` → `create` → `doctor`. Adding `doctor` → `common` would close a cycle.
**How to avoid:** Keep the canonical image list in a new `pkg/internal/airgap/` package (or `pkg/cluster/internal/create/addonimages/`), imported by both `doctor` and `common` but itself importing nothing from `create`.
**Warning signs:** `go build` fails with "import cycle not allowed".

### Pitfall 2: Missing LB Image in Air-Gapped Check
**What goes wrong:** The LB container is only created for multi-control-plane clusters (`config.ClusterHasImplicitLoadBalancer`). If the air-gapped check doesn't account for it, a 3-CP cluster will fail at container creation time with a confusing error.
**Why it happens:** `RequiredNodeImages` in `common/images.go` only covers node images, not the LB image from `loadbalancer.Image`.
**How to avoid:** `RequiredAllImages` (the new combined function) must include `loadbalancer.Image` when the config has multiple control planes.

### Pitfall 3: Doctor Check Runs Without Runtime
**What goes wrong:** If no container runtime is present, `inspect` will fail and the check will report all images as "missing", which is misleading.
**Why it happens:** The doctor check is designed to run pre-flight before any cluster creation.
**How to avoid:** Mirror the existing `containerRuntimeCheck` pattern — detect available runtime first; if none found, return `skip` result rather than listing all images as missing.

### Pitfall 4: Addon Image List Drift
**What goes wrong:** When addon manifests are upgraded (e.g. MetalLB bumped from v0.15.3 to v0.16.0), the Go image constants are not updated, so air-gapped users get confusing "image not found" errors inside the node.
**Why it happens:** Two sources of truth: the YAML manifests and the Go constants.
**How to avoid:** Add a unit test that parses the embedded manifest YAML and compares the image references against the Go constants. This test fails when manifests are updated without updating constants, making drift visible at CI time.

### Pitfall 5: nerdctl `inspect` Returns Different Error for Missing Image
**What goes wrong:** `nerdctl inspect --type=image` may return a different exit code or error format than `docker inspect --type=image`.
**Why it happens:** nerdctl does not guarantee 100% Docker CLI compatibility.
**How to avoid:** The existing `pullIfNotPresent` in each provider already handles this — it just checks `cmd.Run() != nil` (any error = image absent). Reuse the same logic, don't add runtime-specific error code matching.

## Code Examples

Verified patterns from existing codebase:

### Image Presence Check (Docker)
```go
// Source: pkg/cluster/internal/providers/docker/images.go:56-62
cmd := exec.Command("docker", "inspect", "--type=image", image)
if err := cmd.Run(); err == nil {
    logger.V(1).Infof("Image: %s present locally", image)
    return false, nil
}
```

### Image Presence Check (Nerdctl)
```go
// Source: pkg/cluster/internal/providers/nerdctl/images.go:55-62
cmd := exec.Command(binaryName, "inspect", "--type=image", image)
if err := cmd.Run(); err == nil {
    logger.V(1).Infof("Image: %s present locally", image)
    return false, nil
}
```

### Image Presence Check (Podman)
```go
// Source: pkg/cluster/internal/providers/podman/images.go:55-62
cmd := exec.Command("podman", "inspect", "--type=image", image)
if err := cmd.Run(); err == nil {
    logger.V(1).Infof("Image: %s present locally", image)
    return false, nil
}
```

### Doctor Check Pattern (Category, injectable deps)
```go
// Source: pkg/internal/doctor/runtime.go:27-38
type containerRuntimeCheck struct {
    lookPath func(string) (string, error)
    execCmd  func(name string, args ...string) exec.Cmd
}
func newContainerRuntimeCheck() Check {
    return &containerRuntimeCheck{
        lookPath: osexec.LookPath,
        execCmd:  exec.Command,
    }
}
func (c *containerRuntimeCheck) Name() string       { return "container-runtime" }
func (c *containerRuntimeCheck) Category() string    { return "Runtime" }
func (c *containerRuntimeCheck) Platforms() []string { return nil }
```

### Doctor Check Registration
```go
// Source: pkg/internal/doctor/check.go:53-82
var allChecks = []Check{
    // ...
    // Category: Cluster (Phase 42)
    newClusterNodeSkewCheck(),
    // Add new check here:
    // newOfflineReadinessCheck(),
}
```

### CreateOption Pattern
```go
// Source: pkg/cluster/createoption.go:78-84
func CreateWithRetain(retain bool) CreateOption {
    return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
        o.Retain = retain
        return nil
    })
}
```

### RequiredNodeImages (existing utility to extend)
```go
// Source: pkg/cluster/internal/providers/common/images.go:27-33
func RequiredNodeImages(cfg *config.Cluster) sets.String {
    images := sets.NewString()
    for _, node := range cfg.Nodes {
        images.Insert(node.Image)
    }
    return images
}
```

### Table Output via tabwriter
```go
// Source: pkg/internal/doctor/clusterskew.go:314-324
func formatSkewTable(violations []skewViolation) string {
    var buf bytes.Buffer
    w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "NODE\tDETAIL")
    fmt.Fprintln(w, "----\t------")
    for _, v := range violations {
        fmt.Fprintf(w, "%s\t%s\n", v.node, v.detail)
    }
    w.Flush()
    return strings.TrimRight(buf.String(), "\n")
}
```

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|------------------|--------|
| Users must manually track and pre-load all images | `--air-gapped` fails fast with full list | User can pre-load everything in one pass |
| working-offline.md covers node image only | Phase 43 extends doc with addon image workflow | Users no longer discover addon image requirements at creation time |

**Existing docs:** `/site/content/docs/user/working-offline.md` already documents the node image offline workflow. Phase 43 extends it, not replaces it.

## Open Questions

1. **Where to define the canonical addon image list to avoid import cycles?**
   - What we know: `doctor` imports nothing from `providers/common`; `create.go` imports `doctor`; `providers` are only imported from `create.go`.
   - What's unclear: Whether `pkg/internal/airgap/` is the right package name, or whether to use a simpler approach of defining `var AddonImages []string` in each addon action package and importing them from `common`.
   - Recommendation: Prototype the import graph before committing to a package name. The safest option is `pkg/internal/airgap/images.go` since `pkg/internal/` packages have no internal domain logic. Verify with `go build ./...` after any new import.

2. **Does `kinder doctor` offline-readiness check need the user's cluster config?**
   - What we know: `RunAllChecks()` takes no arguments; all current checks are config-independent.
   - What's unclear: Whether to check ALL known addon images (safe but noisy for minimal-profile users) or to require a `--config` flag on `kinder doctor`.
   - Recommendation: Check ALL known infra images unconditionally; in the output, label which addon each image belongs to so users can determine relevance. A `--config` flag on doctor is out of scope for this phase.

3. **Should the non–air-gapped addon image warning be printed before or after node image pull?**
   - What we know: `ensureNodeImages` is called at the start of `Provision`. The warning would be most useful before any work starts.
   - What's unclear: Whether printing before `Provision` clutters the create output.
   - Recommendation: Print before `p.Provision()` is called in `create.go`, at `logger.V(0)` level, so it's always visible unless verbosity is reduced.

4. **Does `kinder load images` (the `--air-gapped` post-create workflow) need changes?**
   - What we know: `kinder load docker-image` exists at `pkg/cmd/kind/load/docker-image/docker-image.go` and already loads images from host into cluster nodes. It uses `docker save` internally and therefore only supports Docker on the host side.
   - What's unclear: Whether the documentation should clarify that `kinder load docker-image` is the post-create path, and whether podman/nerdctl users need a different command.
   - Recommendation: Document the existing `kinder load docker-image` command in the offline workflow section; note the Docker host-side requirement for post-create loading.

## Sources

### Primary (HIGH confidence)
- Direct codebase inspection — all findings verified against actual source files
  - `pkg/cluster/internal/providers/docker/images.go` — image presence and pull logic
  - `pkg/cluster/internal/providers/podman/images.go` — same pattern for podman
  - `pkg/cluster/internal/providers/nerdctl/images.go` — same pattern for nerdctl
  - `pkg/cluster/internal/providers/common/images.go` — `RequiredNodeImages` utility
  - `pkg/cluster/internal/create/create.go` — `ClusterOptions`, `Cluster()`, addon wave execution
  - `pkg/cluster/createoption.go` — `CreateOption` pattern
  - `pkg/internal/doctor/check.go` — `Check` interface and `allChecks` registry
  - `pkg/internal/doctor/clusterskew.go` — reference for new doctor check pattern
  - `pkg/internal/doctor/format.go` — `FormatHumanReadable`, table output
  - `pkg/internal/apis/config/types.go` — `Cluster`, `Addons` struct
  - `pkg/cluster/internal/loadbalancer/const.go` — `Image = "docker.io/kindest/haproxy:v20260131-7181c60a"`
  - `pkg/cluster/internal/create/actions/installmetallb/manifests/metallb-native.yaml` — image refs
  - `pkg/cluster/internal/create/actions/installmetricsserver/manifests/components.yaml` — image refs
  - `pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml` — image refs
  - `pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml` — image refs
  - `pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml` — image refs
  - `pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go` — `registryImage = "registry:2"`
  - `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-device-plugin.yaml` — image refs
  - `site/content/docs/user/working-offline.md` — existing offline doc to extend

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries are already present in the codebase
- Architecture: HIGH — all patterns verified by reading existing code in each provider and doctor package
- Pitfalls: HIGH — import cycle risk confirmed by tracing actual import graph; other pitfalls are based on direct code reading

**Research date:** 2026-04-09
**Valid until:** 2026-05-09 (stable codebase; addon manifest versions may change)
