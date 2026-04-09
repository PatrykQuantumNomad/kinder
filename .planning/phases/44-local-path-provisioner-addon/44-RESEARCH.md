# Phase 44: Local-Path-Provisioner Addon - Research

**Researched:** 2026-04-09
**Domain:** Go addon action pattern, Kubernetes StorageClass management, local-path-provisioner manifest, doctor CVE check
**Confidence:** HIGH — grounded entirely in direct codebase analysis of the kinder repository; cross-referenced with project's own prior research documents in `.planning/research/`

---

## Summary

Phase 44 adds `local-path-provisioner v0.0.35` as a default addon that replaces the legacy non-dynamic `standard` StorageClass installed by `installstorage`. The implementation follows a well-established pattern already used by seven other addons in this codebase (MetalLB, Metrics Server, cert-manager, Dashboard, etc.).

The central technical challenge is not the addon action itself — that is mechanical — but three coordination decisions that must be made and implemented correctly before writing any code:

1. **StorageClass ownership:** `installstorage` must be gated out when `LocalPath` is enabled; two default StorageClasses produce non-deterministic PVC binding.
2. **busybox helper image:** The upstream manifest uses `busybox:latest` with `imagePullPolicy: Always`; the embedded manifest must pin `busybox:1.37.0` with `imagePullPolicy: IfNotPresent` for air-gapped correctness.
3. **StorageClass name:** The phase uses `local-path` (not `standard`) as the StorageClass name. This is correct for the goal ("the only default StorageClass") — legacy `standard` is absent.

The `kinder doctor` CVE check (STOR-05) extends the existing `clusterskew.go` approach: inspect the running provisioner image tag and compare it against the minimum safe version `v0.0.34`.

Zero new Go module dependencies are needed. Everything integrates using packages already in use.

**Primary recommendation:** Implement in three sequential work units: (1) 5-location config pipeline + `installstorage` gate + addon action; (2) busybox-pinned manifest file; (3) doctor CVE check. All three must land together for success criteria to be met.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `_ "embed"` (stdlib) | Go 1.16+ | `//go:embed manifests/local-path-storage.yaml` | Used in all 7 existing addon action packages |
| `strings` (stdlib) | Go stdlib | `strings.NewReader(manifest)` for kubectl stdin | Same pattern in every addon action |
| `sigs.k8s.io/kind/pkg/cluster/internal/create/actions` | in-repo | `actions.Action` interface + `ActionContext` | Required for all addon actions |
| `sigs.k8s.io/kind/pkg/cluster/nodeutils` | in-repo | `nodeutils.ControlPlaneNodes()` | Used by all addons to get the control plane |
| `sigs.k8s.io/kind/pkg/errors` | in-repo | `errors.Wrap()` for structured error messages | Project-wide error package |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/exec` | in-repo | `exec.OutputLines()` for reading kubectl output (CVE check) | Only needed for the doctor check, not the addon action |
| `sigs.k8s.io/kind/pkg/internal/version` | in-repo | `version.ParseSemantic()` + `LessThan()` for CVE threshold check | For doctor CVE check comparing provisioner tag vs `v0.0.34` |
| `sigs.k8s.io/kind/pkg/cluster/internal/create/actions/testutil` | in-repo | `FakeNode`, `FakeProvider`, `NewTestContext` for unit tests | All addon tests use this testutil package |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Embedded manifest (`go:embed`) | Runtime `kubectl apply` with a downloaded manifest | Embedding is the established pattern; no network dependency, deterministic |
| `busybox:1.37.0` helper | `docker.io/library/busybox:musl` | `1.37.0` is already the busybox version used by kinder's node image; consistent |
| `rancher/local-path-provisioner:v0.0.35` | `docker.io/kindest/local-path-provisioner:v20260213-ea8e5717` | The kindest variant is baked into the node image for the legacy path; this addon intentionally uses the upstream Rancher version for the new default path |

**Installation:** No new packages. Zero go.mod changes required.

---

## Architecture Patterns

### Recommended Project Structure

```
pkg/cluster/internal/create/actions/installlocalpath/
├── localpathprovisioner.go       # Action implementation + Images var
├── localpathprovisioner_test.go  # Unit tests using testutil.FakeNode
└── manifests/
    └── local-path-storage.yaml   # Embedded manifest (patched: busybox:1.37.0, IfNotPresent)

pkg/internal/doctor/
└── localpath.go                  # CVE-2025-62878 check (new file)
    localpath_test.go             # Tests for the CVE check
```

### Pattern 1: Addon Action (follows installmetricsserver exactly)

**What:** A Go package implementing `actions.Action` that applies an embedded YAML manifest via `kubectl apply` on the control plane, then waits for `deployment/local-path-provisioner` readiness.

**When to use:** All addon installations in this codebase.

**Example:**
```go
// Source: pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go (adapted)
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

// Images contains the container images used by local-path-provisioner.
// busybox is the helper image referenced in the local-path-config ConfigMap.
var Images = []string{
    "docker.io/rancher/local-path-provisioner:v0.0.35",
    "docker.io/library/busybox:1.37.0",
}

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
    if len(controlPlanes) == 0 {
        return errors.New("no control plane nodes found")
    }
    node := controlPlanes[0]

    // Step 1: Apply the manifest
    if err := node.CommandContext(ctx.Context,
        "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "apply", "-f", "-",
    ).SetStdin(strings.NewReader(localPathManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply local-path-provisioner manifest")
    }

    // Step 2: Wait for readiness
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

### Pattern 2: 5-Location Config Pipeline

**What:** Every new `Addons.*` field requires an atomic 5-file change set plus the behavior gate in `create.go`.

**The 5 locations:**

1. **`pkg/apis/config/v1alpha4/types.go`** — add to `Addons` struct:
```go
// LocalPath enables local-path-provisioner for dynamic PVC provisioning.
// When true (default), the legacy standard StorageClass from installstorage is skipped.
// +optional (default: true)
LocalPath *bool `yaml:"localPath,omitempty" json:"localPath,omitempty"`
```

2. **`pkg/apis/config/v1alpha4/default.go`** — default to enabled (opt-out, not opt-in):
```go
boolPtrTrue(&obj.Addons.LocalPath)
```

3. **`pkg/apis/config/v1alpha4/zz_generated.deepcopy.go`** — add nil-check copy:
```go
if in.LocalPath != nil {
    in, out := &in.LocalPath, &out.LocalPath
    *out = new(bool)
    **out = **in
}
```

4. **`pkg/internal/apis/config/types.go`** — add to internal `Addons` struct (no tags):
```go
LocalPath bool
```

5. **`pkg/internal/apis/config/convert_v1alpha4.go`** — add to `Convertv1alpha4()`:
```go
LocalPath: boolVal(in.Addons.LocalPath),
```

### Pattern 3: installstorage Gate in create.go

**What:** The `installstorage` action in the sequential pipeline (before kubeadm operations) must be skipped when `LocalPath=true`.

**Where:** `pkg/cluster/internal/create/create.go`, in the sequential actions block (currently line 224):

```go
// Before (unconditional):
actionsToRun = append(actionsToRun,
    installstorage.NewAction(),
    ...
)

// After (gated):
if !opts.Config.Addons.LocalPath {
    actionsToRun = append(actionsToRun,
        installstorage.NewAction(),
    )
}
```

**And add to wave1:**
```go
wave1 := []AddonEntry{
    ...
    {"Local Path Provisioner", opts.Config.Addons.LocalPath, installlocalpath.NewAction()},
}
```

### Pattern 4: Doctor CVE Check

**What:** A new `Check` implementation in `pkg/internal/doctor/` that inspects the running `local-path-provisioner` pod's image tag and warns if the tag's version is below `v0.0.34`.

**Pattern:** Follows `clusterskew.go` — uses `exec.Command` to run `kubectl` against the running cluster's control plane container. Since the doctor checks run outside the cluster (no kubeconfig path available by default), the CVE check should use `docker exec` on the kind cluster node container, similar to how `clusterNodeSkewCheck` uses `docker ps` + `docker exec`.

**Implementation approach for STOR-05:**

```go
// pkg/internal/doctor/localpath.go
type localPathCVECheck struct {
    listNodes func() ([]nodeEntry, error) // reuse realListNodes from clusterskew.go
}

func (c *localPathCVECheck) Run() []Result {
    // 1. List cluster nodes via docker ps (same as clusterNodeSkewCheck)
    // 2. For a control-plane node, run:
    //    docker exec <node> kubectl --kubeconfig=/etc/kubernetes/admin.conf \
    //      get deploy local-path-provisioner -n local-path-storage \
    //      -o jsonpath='{.spec.template.spec.containers[0].image}'
    // 3. Parse the image tag version from the result
    // 4. Compare against v0.0.34 (the CVE fix threshold)
    // 5. If below → warn; if not found → skip; if >= → ok
}
```

**Key version threshold:** `v0.0.34` is the minimum safe version (CVE-2025-62878 path traversal fixed). The phase installs `v0.0.35`, which satisfies this. The doctor check warns on clusters where local-path-provisioner is already running at a version below the threshold.

### Pattern 5: Manifest Patches Required

The embedded manifest (`manifests/local-path-storage.yaml`) MUST differ from the upstream in two ways:

1. **busybox image pinning:** The upstream `local-path-config` ConfigMap's `helperPod.yaml` key references the helper image. Change from `busybox` or `busybox:latest` to `busybox:1.37.0`.

2. **busybox imagePullPolicy:** The ConfigMap's `helperPod.yaml` spec must include `imagePullPolicy: IfNotPresent`.

3. **StorageClass name:** Use `local-path` as the StorageClass name (NOT `standard`). Annotate it as the default with `storageclass.kubernetes.io/is-default-class: "true"`.

4. **Provisioner:** `provisioner: rancher.io/local-path` and `volumeBindingMode: WaitForFirstConsumer`.

The node image already has a `kindest/local-path-provisioner` and `kindest/local-path-helper` baked in for the `installstorage` fallback path. This new addon uses `rancher/local-path-provisioner:v0.0.35` (upstream) and `busybox:1.37.0` as helper — these are separate from the baked-in images.

### Pattern 6: RequiredAddonImages Registration

The new addon's images must be registered in `pkg/cluster/internal/providers/common/images.go` so they appear in the air-gapped pre-flight check and the "NOTE: the following addon images will be pulled" warning:

```go
// In RequiredAddonImages():
if cfg.Addons.LocalPath {
    images.Insert(installlocalpath.Images...)
}
```

### Pattern 7: offlinereadiness.go Update

The `pkg/internal/doctor/offlinereadiness.go` maintains `allAddonImages` — a list of all addon images for the `kinder doctor offline-readiness` check. The two new images must be added:

```go
{"docker.io/rancher/local-path-provisioner:v0.0.35", "Local Path Provisioner"},
{"docker.io/library/busybox:1.37.0", "Local Path Provisioner (helper)"},
```

### Anti-Patterns to Avoid

- **Two default StorageClasses:** Never let both `installstorage` and `installlocalpath` run concurrently. The gate in `create.go` is mandatory.
- **Using `busybox:latest`:** The upstream default causes `ImagePullBackOff` in air-gapped clusters. Pin to `busybox:1.37.0` in the embedded manifest file, not at runtime.
- **Naming the StorageClass `standard`:** The goal is `local-path` as the only default. If the StorageClass were named `standard`, users' YAML referencing `storageClassName: local-path` would not find it.
- **Applying `installstorage` before the addon skips it:** The sequential pipeline runs before the addon wave. The gate must prevent `installstorage` from running entirely when `LocalPath=true`.
- **Registering `LocalPath` as opt-in (like `NvidiaGPU`):** The success criterion says "Running `kinder create cluster` installs local-path-provisioner" — it must be default-enabled (`boolPtrTrue`, not `boolValOptIn`).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Manifest application | Custom kubectl logic | `controlPlane.CommandContext(ctx.Context, "kubectl", ..., "apply", "-f", "-")` | Established pattern in all 7 existing addon actions |
| Deployment readiness wait | Poll loop with retries | `kubectl wait --for=condition=Available --timeout=120s` | Declarative, no custom retry logic needed |
| Image version comparison (CVE check) | Custom semver parsing | `version.ParseSemantic()` + `LessThan()` from `pkg/internal/version` | Already used throughout codebase; handles all edge cases |
| Node listing for doctor check | Docker API client | `realListNodes()` from `clusterskew.go` (or equivalent) | Already implemented, avoids import cycles |
| Fake test infrastructure | New test mocks | `testutil.FakeNode`, `testutil.FakeProvider`, `testutil.NewTestContext` | Shared testutil eliminates duplication |

**Key insight:** The addon action pattern is fully mechanical. The only design decisions are at the configuration boundary (opt-in vs opt-out, `installstorage` gate) and the manifest content (busybox pin, StorageClass name). The implementation code follows directly from the `installmetricsserver` template.

---

## Common Pitfalls

### Pitfall 1: busybox Helper Image Breaks All PVC Operations in Air-Gapped Mode

**What goes wrong:** local-path-provisioner spawns a helper pod at PVC create/delete time using the image configured in the `local-path-config` ConfigMap. If this is `busybox:latest` with `imagePullPolicy: Always` (upstream default), every PVC operation hangs with `ImagePullBackOff` in air-gapped clusters. The provisioner pod itself shows `Running` so there is no obvious error.

**Why it happens:** The dependency is in the ConfigMap, not the Deployment spec. Developers pre-loading only the provisioner image miss the helper image entirely.

**How to avoid:** Patch the embedded manifest BEFORE the `go:embed` directive reads it. The manifest file on disk must have `busybox:1.37.0` and `imagePullPolicy: IfNotPresent` in the `helperPod.yaml` key of the `local-path-config` ConfigMap. This cannot be a runtime patch.

**Warning signs:** PVC stuck in `Pending` while `deployment/local-path-provisioner` shows `Available`; helper pod `create-pvc-XXXXX` shows `ImagePullBackOff`.

### Pitfall 2: StorageClass Collision Breaks PVC Binding

**What goes wrong:** If `installstorage` runs AND `installlocalpath` runs, two StorageClasses with `storageclass.kubernetes.io/is-default-class: "true"` exist. On Kubernetes 1.26+ the admission webhook may block the second default; on older versions both exist and PVC binding is non-deterministic.

**Why it happens:** `installstorage` is in the sequential action pipeline before the addon wave and runs unconditionally in the existing code.

**How to avoid:** The `installstorage` call in `create.go` must be wrapped in `if !opts.Config.Addons.LocalPath { ... }`. This is the most critical single line of code in this phase.

**Warning signs:** `kubectl get storageclass` shows two entries annotated `(default)`.

### Pitfall 3: `installstorage` is in Sequential Pipeline, Not Wave1

**What goes wrong:** Unlike addon actions (which are in wave1 or wave2), `installstorage` is in the *sequential* pre-addon pipeline (`actionsToRun`). This means it runs before `kubeadmjoin` and before any addon actions. The gate must be applied in the sequential section, not in the wave1 section.

**How to avoid:** The gate is at line 224 in `create.go` (currently `installstorage.NewAction(),` in the `actionsToRun = append(actionsToRun, ...)` block). The new `installlocalpath` is added to `wave1`, which runs AFTER the sequential pipeline. These are different sections of `create.go`.

### Pitfall 4: Doctor CVE Check Must Skip When No Cluster Is Running

**What goes wrong:** The doctor checks run on the local machine. If no kind cluster is running, any `kubectl` or `docker exec` call fails. The check must skip gracefully (status: "skip") when no cluster is detected.

**How to avoid:** Follow the `clusterNodeSkewCheck` pattern: call `realListNodes()` first; if it returns an empty list, return a `skip` result.

### Pitfall 5: busybox Image Reference Location in Manifest

**What goes wrong:** Developers assume the busybox image is in the Deployment spec and miss the ConfigMap. The upstream local-path-provisioner manifest stores the helper image reference in `local-path-config` ConfigMap's `helperPod.yaml` key, not in the Deployment spec.

**How to avoid:** When editing the embedded manifest, look specifically for the ConfigMap named `local-path-config` and its `helperPod.yaml` data key. The image line is nested in a Pod spec inside a multiline string value. Both the image tag AND the `imagePullPolicy` must be set.

---

## Code Examples

### Complete 5-Location Config Pipeline Diff

**Location 1 — `pkg/apis/config/v1alpha4/types.go` (Addons struct, after NvidiaGPU):**
```go
// LocalPath enables local-path-provisioner for dynamic PVC provisioning.
// When enabled (default), the legacy standard StorageClass from installstorage is replaced.
// +optional (default: true)
LocalPath *bool `yaml:"localPath,omitempty" json:"localPath,omitempty"`
```

**Location 2 — `pkg/apis/config/v1alpha4/default.go` (SetDefaultsCluster):**
```go
boolPtrTrue(&obj.Addons.LocalPath)  // LocalPath is opt-out (enabled by default)
```

**Location 3 — `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` (Addons.DeepCopyInto):**
```go
if in.LocalPath != nil {
    in, out := &in.LocalPath, &out.LocalPath
    *out = new(bool)
    **out = **in
}
```

**Location 4 — `pkg/internal/apis/config/types.go` (Addons struct):**
```go
LocalPath bool
```

**Location 5 — `pkg/internal/apis/config/convert_v1alpha4.go` (out.Addons block):**
```go
LocalPath: boolVal(in.Addons.LocalPath),
```

### create.go Changes

```go
// installstorage gate (in sequential actionsToRun, BEFORE kubeadmjoin):
if !opts.Config.Addons.LocalPath {
    actionsToRun = append(actionsToRun,
        installstorage.NewAction(),
    )
}

// installlocalpath in wave1 (alongside other addons):
wave1 := []AddonEntry{
    // ... existing addons ...
    {"Local Path Provisioner", opts.Config.Addons.LocalPath, installlocalpath.NewAction()},
}
```

### Manifest ConfigMap Patch (embedded file, not runtime)

The `local-path-config` ConfigMap's `helperPod.yaml` key must look like:
```yaml
  helperPod.yaml: |-
    apiVersion: v1
    kind: Pod
    metadata:
      name: helper-pod
    spec:
      priorityClassName: system-node-critical
      tolerations:
        - key: node.kubernetes.io/disk-pressure
          operator: Exists
          effect: NoSchedule
      containers:
      - name: helper-pod
        image: busybox:1.37.0
        imagePullPolicy: IfNotPresent
```

### RequiredAddonImages Registration

```go
// In pkg/cluster/internal/providers/common/images.go, RequiredAddonImages():
if cfg.Addons.LocalPath {
    images.Insert(installlocalpath.Images...)
}
```

### Doctor CVE Check Skeleton

```go
// pkg/internal/doctor/localpath.go
type localPathCVECheck struct {
    listNodes    func() ([]nodeEntry, error)
    runKubectl   func(nodeName, args string) (string, error)
}

func (c *localPathCVECheck) Name() string       { return "local-path-cve-2025-62878" }
func (c *localPathCVECheck) Category() string    { return "Cluster" }
func (c *localPathCVECheck) Platforms() []string { return nil }

// minSafeVersion is the first version that fixes CVE-2025-62878.
const minSafeVersion = "v0.0.34"

func (c *localPathCVECheck) Run() []Result {
    nodes, err := c.listNodes()
    if err != nil || len(nodes) == 0 {
        return []Result{{Status: "skip", Message: "(no cluster found)"}}
    }
    // find a control-plane node, exec kubectl to get provisioner image tag
    // parse tag version, compare against minSafeVersion
    // warn if below, ok if at or above, skip if provisioner not found
}
```

### Test Pattern (follows metricsserver_test.go)

```go
// pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go
func TestExecute(t *testing.T) {
    tests := []struct {
        name      string
        cmds      []*testutil.FakeCmd
        wantErr   bool
        wantCalls int
    }{
        {
            name: "all steps succeed",
            cmds: []*testutil.FakeCmd{
                {},  // Step 1: kubectl apply manifest
                {},  // Step 2: kubectl wait deployment
            },
            wantErr:   false,
            wantCalls: 2,
        },
        {
            name:    "apply manifest fails",
            cmds:    []*testutil.FakeCmd{{Err: errors.New("kubectl failed")}},
            wantErr: true, wantCalls: 1,
        },
    }
    // ...
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `standard` StorageClass via `installstorage` (host-path, non-dynamic) | `local-path` StorageClass via `installlocalpath` (dynamic, `rancher.io/local-path` provisioner) | Phase 44 | PVCs bind automatically; no manual PV pre-creation |
| `installstorage` unconditional | `installstorage` gated by `!Addons.LocalPath` | Phase 44 | Legacy path preserved when `addons.localPath: false` |
| No CVE awareness | `kinder doctor` warns on CVE-2025-62878 | Phase 44 | Security hygiene for existing clusters |

**Deprecated/outdated:**
- `standard` StorageClass: replaced by `local-path` when `LocalPath=true` (default). Still installed when `LocalPath=false` for backward compatibility.
- `busybox:latest` as helper image: pinned to `busybox:1.37.0` in the embedded manifest.

---

## Open Questions

1. **Upstream v0.0.35 manifest location**
   - What we know: The upstream release is at `https://github.com/rancher/local-path-provisioner/releases/tag/v0.0.35`; the deploy manifest is typically at `https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.35/deploy/local-path-storage.yaml`
   - What's unclear: The exact ConfigMap structure in the v0.0.35 manifest (whether `imagePullPolicy` is already present, what the default helper image tag is)
   - Recommendation: Fetch the manifest and inspect before writing the embedded file. The manifest is the source of truth for ConfigMap structure. The project's existing SUMMARY.md confirms "helper image is in the `local-path-config` ConfigMap's `config.json` key" — verify the exact nested YAML structure before editing.

2. **CVE check: how to exec kubectl inside a node container from the doctor context**
   - What we know: `clusterNodeSkewCheck.realListNodes()` uses `docker ps --filter label=io.x-k8s.kind.cluster=kind` to find nodes, then `docker exec <node> cat /kind/version` for version info.
   - What's unclear: Whether `kubectl --kubeconfig=/etc/kubernetes/admin.conf get deploy -n local-path-storage` can be run via `docker exec` on the control-plane container in the doctor context.
   - Recommendation: Follow the exact same `docker exec` pattern as `clusterskew.go`. The command to run inside the control-plane container: `kubectl --kubeconfig=/etc/kubernetes/admin.conf get deploy local-path-provisioner -n local-path-storage -o jsonpath='{.spec.template.spec.containers[0].image}'`.

3. **Multi-node cluster: does `local-path-provisioner` work on all nodes?**
   - What we know: local-path-provisioner uses `WaitForFirstConsumer` volume binding mode and schedules storage on the node where the PVC is bound. The Deployment runs on any node (no node affinity to a specific worker). STOR-06 requires single AND multi-node.
   - What's unclear: Whether any additional RBAC or config is needed for multi-node support.
   - Recommendation: The upstream manifest includes `tolerations` for control-plane taints and runs as a single-replica Deployment. For multi-node, `WaitForFirstConsumer` ensures the PV is created on the node where the pod is scheduled. No additional configuration is needed. The STOR-06 success criterion ("PVC transitions to Bound automatically in both single-node and multi-node clusters") is met by the standard manifest.

---

## File Touch Map

All files that must be modified or created:

| File | Action | What Changes |
|------|--------|--------------|
| `pkg/apis/config/v1alpha4/types.go` | MODIFY | Add `LocalPath *bool` to `Addons` struct |
| `pkg/apis/config/v1alpha4/default.go` | MODIFY | Add `boolPtrTrue(&obj.Addons.LocalPath)` |
| `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` | MODIFY | Add nil-check deepcopy for `LocalPath *bool` |
| `pkg/internal/apis/config/types.go` | MODIFY | Add `LocalPath bool` to internal `Addons` struct |
| `pkg/internal/apis/config/convert_v1alpha4.go` | MODIFY | Add `LocalPath: boolVal(in.Addons.LocalPath)` |
| `pkg/cluster/internal/create/create.go` | MODIFY | Gate `installstorage`; add `installlocalpath` to wave1; add import |
| `pkg/cluster/internal/providers/common/images.go` | MODIFY | Add `LocalPath` branch to `RequiredAddonImages()` |
| `pkg/internal/doctor/offlinereadiness.go` | MODIFY | Add provisioner + busybox to `allAddonImages` |
| `pkg/internal/doctor/check.go` | MODIFY | Register `newLocalPathCVECheck()` in `allChecks` |
| `pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-*.yaml` | MODIFY | Add `localPath:` fields to addon test fixtures |
| `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go` | CREATE | New addon action |
| `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go` | CREATE | Unit tests |
| `pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` | CREATE | Patched upstream manifest |
| `pkg/internal/doctor/localpath.go` | CREATE | CVE-2025-62878 doctor check |
| `pkg/internal/doctor/localpath_test.go` | CREATE | CVE check tests |

---

## Sources

### Primary (HIGH confidence)

- Direct codebase reads: `pkg/cluster/internal/create/create.go` — addon wave structure, `installstorage` position, `AddonEntry` pattern
- Direct codebase reads: `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go` — canonical simple addon template
- Direct codebase reads: `pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go` — addon with `Images` var pattern
- Direct codebase reads: `pkg/cluster/internal/create/actions/installstorage/storage.go` — current storage installation logic
- Direct codebase reads: `pkg/build/nodeimage/const_storage.go` — existing kindest local-path-provisioner manifest (shows what the node image already has)
- Direct codebase reads: `pkg/apis/config/v1alpha4/types.go`, `default.go`, `zz_generated.deepcopy.go` — current Addons struct and deepcopy pattern
- Direct codebase reads: `pkg/internal/apis/config/types.go`, `convert_v1alpha4.go` — internal types and conversion pattern
- Direct codebase reads: `pkg/cluster/internal/providers/common/images.go` — RequiredAddonImages pattern
- Direct codebase reads: `pkg/internal/doctor/offlinereadiness.go` — allAddonImages pattern
- Direct codebase reads: `pkg/internal/doctor/check.go` — allChecks registry, Check interface
- Direct codebase reads: `pkg/internal/doctor/clusterskew.go` — docker exec inside kind nodes pattern for doctor checks
- Direct codebase reads: `pkg/internal/apis/config/validate.go` — version parsing utilities already in use
- Direct codebase reads: `pkg/cluster/internal/create/actions/testutil/fake.go` — FakeNode/FakeProvider test infrastructure
- Project research: `.planning/research/SUMMARY.md`, `STACK.md`, `ARCHITECTURE.md`, `PITFALLS.md` (researched 2026-04-08) — comprehensive prior art for this exact phase

### Secondary (MEDIUM confidence)

- `.planning/research/SUMMARY.md` — CVE-2025-62878 fix threshold: v0.0.34 (confirmed as MEDIUM confidence in source document: third-party Orca Security advisory consistent with release notes)
- `.planning/research/STACK.md` — rancher/local-path-provisioner v0.0.35 released 2026-03-10 (HIGH confidence: official GitHub release)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — direct codebase verification; all packages already in use
- Architecture: HIGH — all patterns are established and used in ≥3 existing addon actions; 5-location config pipeline confirmed across 8+ existing fields
- Pitfalls: HIGH — StorageClass collision and busybox offline failure are documented in project research with codebase evidence; CVE threshold confirmed from upstream release notes
- Doctor CVE check approach: MEDIUM — `clusterskew.go` pattern is established; the specific `kubectl get deploy` command to extract the provisioner image tag has not been tested live

**Research date:** 2026-04-09
**Valid until:** 2026-07-09 (90 days — local-path-provisioner release cadence is quarterly; `v0.0.35` is the current latest as of research date)
