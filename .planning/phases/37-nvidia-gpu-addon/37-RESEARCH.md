# Phase 37: NVIDIA GPU Addon - Research

**Researched:** 2026-03-04
**Domain:** NVIDIA GPU device plugin, Kubernetes RuntimeClass, Go embed, pre-flight host checks, Docker runtime configuration
**Confidence:** HIGH (core patterns), MEDIUM (exact daemon.json structure), LOW (kind-specific GPU node mounting)

## Summary

Phase 37 adds an opt-in NVIDIA GPU addon to kinder that enables GPU workloads on Linux hosts with NVIDIA hardware. The approach is device plugin only: embed the NVIDIA k8s-device-plugin DaemonSet manifest and a RuntimeClass "nvidia" manifest, apply them via `kubectl`, and guard behind a pre-flight check that validates the host has NVIDIA drivers, nvidia-container-toolkit, and Docker configured with the nvidia runtime as default. Non-Linux platforms get an informational skip message, not an error. The `kinder doctor` command gains three new NVIDIA-specific checks.

The key architectural decision already encoded in requirements is: device plugin only (not GPU Operator, not NVIDIA Operator full stack). This is correct — kind runs Kubernetes inside Docker containers, and those containers cannot load kernel modules, so host drivers are the only viable path. The device plugin approach (embed + apply) matches exactly what every other kinder addon does: `//go:embed` a vendored YAML manifest and apply it via `kubectl apply -f -` on the control-plane node.

The NvidiaGPU field in v1alpha4 is opt-in (`*bool` defaulting to false), which is different from all other existing addons (which default to true). This is correct: NVIDIA GPU hardware is not universally available, making opt-out the wrong default.

**Primary recommendation:** Follow the existing addon pattern exactly — `//go:embed` two vendored manifests (device plugin DaemonSet + RuntimeClass), run the pre-flight check in `Execute()` before applying, and extend `kinder doctor` with three new result items.

## Standard Stack

### Core
| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| NVIDIA k8s-device-plugin | v0.17.1 | DaemonSet that exposes `nvidia.com/gpu` resources to kubelet | Official NVIDIA plugin; device-plugin-only approach avoids GPU Operator complexity incompatible with kind |
| Kubernetes RuntimeClass | node.k8s.io/v1 | Named runtime "nvidia" with handler "nvidia" | Required so pods can target the NVIDIA container runtime without making it the sole cluster default |
| `//go:embed` | Go stdlib | Bundle YAML manifests at build time | Already the pattern used by every kinder addon (metallb, cert-manager, envoy-gateway) |
| `os/exec.LookPath` | Go stdlib | Check for nvidia-smi, nvidia-ctk binaries on host | Already used in `kinder doctor` for binary presence checks |
| `runtime.GOOS` | Go stdlib | Platform guard (Linux-only) | Already used in `create.go` for MetalLB platform warning |

### Supporting
| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| nvidia-container-toolkit | >= 1.7.0 | Host-side bridge between Docker and NVIDIA drivers | Must be installed on host before kinder can expose GPUs to kind nodes |
| `docker info --format '{{json .Runtimes}}'` | Docker CLI | Check whether nvidia runtime is registered | Used in doctor check to detect missing runtime config |
| `nvidia-smi --query-gpu=driver_version --format=csv,noheader` | Host binary | Get driver version string | Used in doctor check to extract driver version for display |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Device plugin DaemonSet | GPU Operator (Helm) | GPU Operator manages drivers + toolkit + plugin stack; incompatible with kind (nodes are containers, no kernel module access) — explicitly out of scope |
| Embedding v0.17.1 manifest | Fetching at runtime | Embedding avoids network dependency at cluster creation; matches all other addons |
| `docker info` JSON parsing | Parsing `/etc/docker/daemon.json` | `docker info` is the live state; daemon.json may exist but Docker may not have been restarted yet |

**Installation (host prerequisites, not kinder code):**
```bash
# On Ubuntu/Debian Linux host with NVIDIA GPU
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
  sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
  sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
sudo nvidia-ctk runtime configure --runtime=docker --set-as-default
sudo systemctl restart docker
```

## Architecture Patterns

### Recommended Project Structure
```
pkg/cluster/internal/create/actions/installnvidiagpu/
├── nvidiagpu.go             # action: pre-flight + apply manifests
├── nvidiagpu_test.go        # unit tests using testutil.FakeNode
└── manifests/
    ├── nvidia-device-plugin.yaml   # embedded DaemonSet (v0.17.1)
    └── nvidia-runtimeclass.yaml    # embedded RuntimeClass "nvidia"
```

```
pkg/cmd/kind/doctor/
└── doctor.go                # extend with 3 new NVIDIA check functions
```

```
pkg/apis/config/v1alpha4/
└── types.go                 # add NvidiaGPU *bool to Addons struct

pkg/internal/apis/config/
├── types.go                 # add NvidiaGPU bool to internal Addons struct
├── convert_v1alpha4.go      # convert NvidiaGPU field (opt-in: false default)
├── default_test.go          # add test for NvidiaGPU defaults to false
└── zz_generated.deepcopy.go # add NvidiaGPU *bool deepcopy block

kinder-site/src/content/docs/addons/
└── nvidia-gpu.md            # documentation page (SITE-02)
```

### Pattern 1: Addon Action (Execute + Embed)
**What:** An `actions.Action` implementation that uses `//go:embed` to bundle manifests and applies them via `kubectl apply -f -` piped through `strings.NewReader`.
**When to use:** All kinder addons follow this pattern.
**Example:**
```go
// Source: pkg/cluster/internal/create/actions/installcertmanager/certmanager.go (project codebase)
package installnvidiagpu

import (
    _ "embed"
    "fmt"
    "runtime"
    "strings"

    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/nodeutils"
    "sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/nvidia-device-plugin.yaml
var nvidiaDevicePluginManifest string

//go:embed manifests/nvidia-runtimeclass.yaml
var nvidiaRuntimeClassManifest string

type action struct{}

func NewAction() actions.Action { return &action{} }

func (a *action) Execute(ctx *actions.ActionContext) error {
    // GPU-04: Linux-only guard
    if runtime.GOOS != "linux" {
        ctx.Logger.V(0).Infof(
            "NVIDIA GPU addon is Linux-only; skipping on %s (no GPUs will be available)",
            runtime.GOOS,
        )
        return nil
    }

    ctx.Status.Start("Installing NVIDIA GPU addon")
    defer ctx.Status.End(false)

    // GPU-06: Pre-flight check before applying manifests
    if err := checkHostPrerequisites(); err != nil {
        return err // fast-fail with actionable message
    }

    allNodes, err := ctx.Nodes()
    if err != nil {
        return errors.Wrap(err, "failed to list cluster nodes")
    }
    controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)
    if err != nil || len(controlPlanes) == 0 {
        return errors.New("no control plane nodes found")
    }
    node := controlPlanes[0]

    // GPU-02: Apply RuntimeClass first
    if err := node.CommandContext(ctx.Context, "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-").
        SetStdin(strings.NewReader(nvidiaRuntimeClassManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply nvidia RuntimeClass")
    }

    // GPU-01: Apply device plugin DaemonSet
    if err := node.CommandContext(ctx.Context, "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-").
        SetStdin(strings.NewReader(nvidiaDevicePluginManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply nvidia-device-plugin DaemonSet")
    }

    ctx.Status.End(true)
    return nil
}
```

### Pattern 2: Pre-flight Host Check (GPU-06)
**What:** Run host-side commands before applying Kubernetes manifests. Return a structured error with the exact fix command.
**When to use:** When the addon requires host-side prerequisites that are not guaranteed to be present.
**Example:**
```go
// Source: research-derived from existing doctor.go pattern in this project
import osexec "os/exec"

func checkHostPrerequisites() error {
    // Check 1: NVIDIA driver
    if _, err := osexec.LookPath("nvidia-smi"); err != nil {
        return fmt.Errorf(
            "NVIDIA GPU addon: nvidia-smi not found — install NVIDIA drivers first.\n" +
            "  See: https://docs.nvidia.com/datacenter/tesla/driver-installation-guide/",
        )
    }

    // Check 2: nvidia-container-toolkit
    if _, err := osexec.LookPath("nvidia-ctk"); err != nil {
        return fmt.Errorf(
            "NVIDIA GPU addon: nvidia-ctk not found — install the NVIDIA Container Toolkit.\n" +
            "  Run: sudo apt-get install -y nvidia-container-toolkit",
        )
    }

    // Check 3: nvidia runtime configured in Docker
    if !nvidiaRuntimeInDocker() {
        return fmt.Errorf(
            "NVIDIA GPU addon: nvidia runtime not configured as Docker default.\n" +
            "  Run: sudo nvidia-ctk runtime configure --runtime=docker --set-as-default && sudo systemctl restart docker",
        )
    }

    return nil
}

func nvidiaRuntimeInDocker() bool {
    out, err := exec.OutputLines(exec.Command("docker", "info", "--format", "{{range $k, $v := .Runtimes}}{{$k}} {{end}}"))
    if err != nil || len(out) == 0 {
        return false
    }
    return strings.Contains(out[0], "nvidia")
}
```

### Pattern 3: Config API Extension (opt-in field)
**What:** Add `NvidiaGPU *bool` to the v1alpha4 public Addons struct (nil = false, not-enabled by default).
**When to use:** GPU is opt-in; most users won't have GPU hardware. All existing fields default to true; this is the first false-default addon.
**Example:**
```go
// Source: pkg/apis/config/v1alpha4/types.go (project codebase)
type Addons struct {
    // existing fields...
    MetalLB *bool `yaml:"metalLB,omitempty" json:"metalLB,omitempty"`
    // ... etc ...

    // NvidiaGPU enables the NVIDIA device plugin DaemonSet and RuntimeClass "nvidia".
    // Linux-only. Requires nvidia-container-toolkit configured as the Docker default runtime.
    // +optional (default: false -- opt-in, GPU hardware not universally available)
    NvidiaGPU *bool `yaml:"nvidiaGPU,omitempty" json:"nvidiaGPU,omitempty"`
}
```

Internal config type (no pointer, plain bool):
```go
// Source: pkg/internal/apis/config/types.go
type Addons struct {
    MetalLB       bool
    // ...
    NvidiaGPU     bool  // false = disabled (opt-in)
}
```

Conversion (opt-in = false when nil):
```go
// Source: pkg/internal/apis/config/convert_v1alpha4.go
boolValOptIn := func(b *bool) bool {
    if b == nil {
        return false  // different from existing boolVal which defaults true
    }
    return *b
}

out.Addons = Addons{
    // existing...
    NvidiaGPU: boolValOptIn(in.Addons.NvidiaGPU),
}
```

DeepCopy addition in `zz_generated.deepcopy.go`:
```go
if in.NvidiaGPU != nil {
    in, out := &in.NvidiaGPU, &out.NvidiaGPU
    *out = new(bool)
    **out = **in
}
```

### Pattern 4: Doctor Checks (GPU-05)
**What:** Extend the existing `runE` function in `doctor.go` with three new NVIDIA-specific result items, gated on Linux OS.
**When to use:** Non-Linux hosts should not fail doctor for missing NVIDIA tools.
**Example:**
```go
// Source: research-derived from existing doctor.go pattern in this project
// In runE() after existing kubectl check:
if runtime.GOOS == "linux" {
    results = append(results, checkNvidiaDriver())
    results = append(results, checkNvidiaContainerToolkit())
    results = append(results, checkNvidiaDockerRuntime())
}

func checkNvidiaDriver() result {
    if _, err := osexec.LookPath("nvidia-smi"); err != nil {
        return result{
            name:    "nvidia-driver",
            status:  "warn",
            message: "nvidia-smi not found — NVIDIA GPU addon will not work. Install drivers: https://docs.nvidia.com/datacenter/tesla/driver-installation-guide/",
        }
    }
    // Get driver version
    lines, err := exec.OutputLines(exec.Command("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader"))
    version := "unknown"
    if err == nil && len(lines) > 0 {
        version = strings.TrimSpace(lines[0])
    }
    return result{name: "nvidia-driver", status: "ok", message: "driver version " + version}
}

func checkNvidiaContainerToolkit() result {
    if _, err := osexec.LookPath("nvidia-ctk"); err != nil {
        return result{
            name:    "nvidia-container-toolkit",
            status:  "warn",
            message: "nvidia-ctk not found — install: sudo apt-get install -y nvidia-container-toolkit",
        }
    }
    return result{name: "nvidia-container-toolkit", status: "ok"}
}

func checkNvidiaDockerRuntime() result {
    lines, err := exec.OutputLines(exec.Command("docker", "info", "--format",
        "{{range $k, $v := .Runtimes}}{{$k}} {{end}}"))
    if err != nil || len(lines) == 0 || !strings.Contains(lines[0], "nvidia") {
        return result{
            name:    "nvidia-docker-runtime",
            status:  "warn",
            message: "nvidia runtime not in Docker — run: sudo nvidia-ctk runtime configure --runtime=docker --set-as-default && sudo systemctl restart docker",
        }
    }
    return result{name: "nvidia-docker-runtime", status: "ok"}
}
```

### Pattern 5: Wave Integration
**What:** Add NVIDIA GPU addon to create.go wave1 list alongside other parallel addons.
**When to use:** NVIDIA GPU addon has no inter-addon dependencies; it can run in parallel with others.
**Example:**
```go
// Source: pkg/cluster/internal/create/create.go (project codebase)
wave1 := []AddonEntry{
    // existing entries...
    {"NVIDIA GPU", opts.Config.Addons.NvidiaGPU, installnvidiagpu.NewAction()},
}
```

### Anti-Patterns to Avoid
- **Fetching manifests at runtime:** Don't `http.Get` the device plugin manifest during cluster creation; embed it at build time. Network outages should not block cluster creation.
- **Making nvidia the default Docker runtime globally:** The addon does NOT modify the host's Docker daemon. That is a host prerequisite the user must set up. The pre-flight check validates it and returns an actionable error.
- **Applying manifests before pre-flight:** Never apply the DaemonSet then discover GPU isn't available. Pre-flight runs first; if it fails, `Execute()` returns immediately before touching the cluster.
- **Panicking on non-Linux:** Return `nil` after logging the informational skip message — never return an error that would abort cluster creation on macOS/Windows.
- **Using `runtime.GOOS == "linux"` to guard deepcopy/config:** The config API changes are platform-agnostic; only the `Execute()` guard and the doctor checks are OS-conditional.
- **Treating `NvidiaGPU *bool` the same as other addons in defaults:** Do NOT call `boolPtrTrue(&obj.Addons.NvidiaGPU)` in `SetDefaultsCluster`. The nil value means disabled (false default), unlike all existing addons which default to true.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| GPU device exposure in Kubernetes | Custom device plugin | NVIDIA k8s-device-plugin v0.17.1 DaemonSet | Handles NVML library, device enumeration, cgroup configuration, health monitoring, and MIG support |
| Container runtime GPU injection | Custom container hook | nvidia-container-toolkit + host configuration | Handles CUDA library injection, device cgroup isolation, and capability assignment |
| GPU Operator full stack | Custom CRD/controller | Not applicable (out of scope) | GPU Operator requires kernel module loading; kind nodes are Docker containers |
| RuntimeClass nginx-style handler | Custom webhook | Standard `node.k8s.io/v1 RuntimeClass` with `handler: nvidia` | Two lines of YAML; no code needed |

**Key insight:** The GPU exposure stack (driver → nvidia-container-runtime → containerd/Docker → kubelet → device plugin → pod) must be assembled from official NVIDIA components. Any custom implementation will miss edge cases like MIG, health monitoring, and CUDA version compatibility.

## Common Pitfalls

### Pitfall 1: Pre-flight Runs Too Late
**What goes wrong:** Cluster creation succeeds, but GPU pods stay in `Pending` because the device plugin can't see any GPUs, or they fail after minutes with opaque errors.
**Why it happens:** Applying the device plugin manifest with no pre-flight check means errors surface only after a 10-minute cluster creation.
**How to avoid:** Run `checkHostPrerequisites()` as the first statement in `Execute()`, before any `ctx.Nodes()` or `kubectl` calls. Return a structured error with the exact fix command.
**Warning signs:** Requirements GPU-06 is not satisfied — any code path that reaches `kubectl apply` before checking for nvidia-smi, nvidia-ctk, and docker nvidia runtime.

### Pitfall 2: Wrong Default for NvidiaGPU
**What goes wrong:** If `SetDefaultsCluster` calls `boolPtrTrue(&obj.Addons.NvidiaGPU)` (the same pattern as all other addons), every cluster creation attempt will try to install the GPU addon, fail the pre-flight check on non-GPU hosts, and print warnings.
**Why it happens:** The existing pattern in `default.go` defaults all addons to true; GPU is opt-in.
**How to avoid:** Do NOT add `boolPtrTrue(&obj.Addons.NvidiaGPU)` to `SetDefaultsCluster`. Leave nil = disabled. The conversion function uses a separate `boolValOptIn` helper that returns false for nil.
**Warning signs:** Any test cluster creation on a non-GPU machine prints a GPU-related warning.

### Pitfall 3: macOS/Windows Returns Error Instead of Info
**What goes wrong:** `kinder create cluster --config gpu-cluster.yaml` fails on macOS with exit code 1, breaking user workflow.
**Why it happens:** GPU-04 says skip with informational message; if the code returns an error, the addon framework logs "FAILED" and the user is confused.
**How to avoid:** In `Execute()`, after the `runtime.GOOS != "linux"` check, call `ctx.Logger.V(0).Infof(...)` and return `nil` (not an error).
**Warning signs:** Test on `darwin` shows addon summary line "NVIDIA GPU   FAILED" instead of "NVIDIA GPU   skipped".

### Pitfall 4: Doctor Checks Run on Non-Linux Hosts
**What goes wrong:** `kinder doctor` on macOS fails with "nvidia-smi not found [FAIL]", misleading non-GPU users.
**Why it happens:** Adding NVIDIA checks unconditionally to `runE()`.
**How to avoid:** Gate the three NVIDIA checks behind `if runtime.GOOS == "linux"`. Non-Linux hosts should not see any NVIDIA result lines.
**Warning signs:** Running `kinder doctor` on macOS shows any nvidia-related output.

### Pitfall 5: DeepCopy Not Updated
**What goes wrong:** Compile error or subtle nil pointer bug when the internal config round-trips through deepcopy.
**Why it happens:** `zz_generated.deepcopy.go` is auto-generated but in this project the comment says "DO NOT EDIT" but the project does not run codegen automatically; it must be updated manually to match the new `NvidiaGPU *bool` field.
**How to avoid:** Add the `NvidiaGPU *bool` deepcopy block to `zz_generated.deepcopy.go` in the same commit that adds the field to `types.go`. Follow the exact pattern of every other `*bool` field (MetalLB, EnvoyGateway, etc.).
**Warning signs:** `go build ./...` succeeds but `go vet ./...` complains about missing deepcopy, or test failures in config round-trip tests.

### Pitfall 6: Device Plugin Sees No GPUs Despite Correct Setup
**What goes wrong:** Device plugin DaemonSet is running, but `kubectl describe node` shows `nvidia.com/gpu: 0`.
**Why it happens:** The host Docker daemon's nvidia runtime is configured but `accept-nvidia-visible-devices-as-volume-mounts` is not set to true in `/etc/nvidia-container-runtime/config.toml`. This setting is required specifically for kind (where kind nodes are Docker containers that themselves run containerd).
**Why it happens root cause:** kind cluster nodes are Docker containers. The nvidia-container-runtime injects GPU access via environment variables by default. For nested containerization (Docker → kind node container → pod), the volume-mounts strategy must be used instead.
**How to avoid:** Document clearly in the prerequisites that users must run:
  ```bash
  sudo nvidia-ctk config --set accept-nvidia-visible-devices-as-volume-mounts=true --in-place
  sudo systemctl restart docker
  ```
**Warning signs:** `kubectl describe node | grep nvidia.com/gpu` shows `nvidia.com/gpu: 0`; device plugin pod logs show "0 GPUs found".

### Pitfall 7: RuntimeClass Handler Mismatch
**What goes wrong:** Pod with `runtimeClassName: nvidia` fails to schedule with "RuntimeClass not found" or stays in Pending.
**Why it happens:** RuntimeClass name is "nvidia" but handler must also be "nvidia" — handler must match the runtime name registered in Docker/containerd.
**How to avoid:** The embedded RuntimeClass manifest must have both `metadata.name: nvidia` and `handler: nvidia`.
**Warning signs:** `kubectl get runtimeclass nvidia -o yaml` shows `handler: runc` instead of `handler: nvidia`.

## Code Examples

### NVIDIA Device Plugin DaemonSet Manifest (to embed)
```yaml
# Source: https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.17.1/deployments/static/nvidia-device-plugin.yml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: nvidia-device-plugin-daemonset
  namespace: kube-system
spec:
  selector:
    matchLabels:
      name: nvidia-device-plugin-ds
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        name: nvidia-device-plugin-ds
    spec:
      tolerations:
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
      priorityClassName: "system-node-critical"
      containers:
      - image: nvcr.io/nvidia/k8s-device-plugin:v0.17.1
        name: nvidia-device-plugin-ctr
        env:
          - name: FAIL_ON_INIT_ERROR
            value: "false"
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
        volumeMounts:
        - name: device-plugin
          mountPath: /var/lib/kubelet/device-plugins
      volumes:
      - name: device-plugin
        hostPath:
          path: /var/lib/kubelet/device-plugins
```

### RuntimeClass Manifest (to embed)
```yaml
# Source: https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/ + NVIDIA nvkind README
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: nvidia
handler: nvidia
```

### Test Pod to Verify GPU Works
```yaml
# Usage example for documentation and validation
apiVersion: v1
kind: Pod
metadata:
  name: gpu-smoke-test
spec:
  runtimeClassName: nvidia
  restartPolicy: Never
  containers:
  - name: cuda
    image: nvcr.io/nvidia/cuda:12.4.1-base-ubuntu22.04
    command: ["nvidia-smi"]
    resources:
      limits:
        nvidia.com/gpu: 1
```

### kinder cluster config (GPU enabled)
```yaml
# Source: requirements GPU-03
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  nvidiaGPU: true
```

### Docker daemon.json after nvidia-ctk configure
```json
{
  "default-runtime": "nvidia",
  "runtimes": {
    "nvidia": {
      "path": "/usr/bin/nvidia-container-runtime",
      "runtimeArgs": []
    }
  }
}
```

### Check nvidia runtime in Docker (for pre-flight and doctor)
```bash
# Command to verify nvidia runtime is present
docker info --format '{{range $k, $v := .Runtimes}}{{$k}} {{end}}'
# Expected output (contains "nvidia"): nvidia runc
```

### Get NVIDIA driver version for doctor output
```bash
nvidia-smi --query-gpu=driver_version --format=csv,noheader
# Example output: 535.104.05
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| GPU Operator (full stack) | Device plugin only (for kind) | Always for kind | GPU Operator requires kernel modules; kind nodes are containers |
| `--gpus all` Docker flag | nvidia-container-toolkit + default runtime | ~2020 | Enables kubelet-level GPU allocation via device plugin |
| `nvcr.io/nvidia/k8s-device-plugin:v0.14.x` | `v0.17.1` (Jan 2025) | 2025 | Newer versions support CDI, FAIL_ON_INIT_ERROR=false |
| RuntimeClass not required | RuntimeClass "nvidia" required when not default | Kubernetes 1.14+ | Without RuntimeClass, pods must rely on default runtime being nvidia |

**Deprecated/outdated:**
- `nvidia-docker2` package: Superseded by nvidia-container-toolkit; the old `nvidia-docker` wrapper is no longer required. Use `nvidia-ctk runtime configure` instead.
- `NVIDIA_VISIBLE_DEVICES=all` environment variable approach: Replaced by volume-mounts strategy for nested containers (required for kind). Set `accept-nvidia-visible-devices-as-volume-mounts=true`.

## Open Questions

1. **`accept-nvidia-visible-devices-as-volume-mounts` pre-flight check**
   - What we know: This setting in `/etc/nvidia-container-runtime/config.toml` is required for GPU to appear in kind nodes. The nvkind project explicitly requires it.
   - What's unclear: Whether the pre-flight check should verify this file directly (fragile path) or rely on the doctor check only (not a hard error).
   - Recommendation: Add this to the doctor check as a "warn" (not "fail"), and add a clear note in the documentation prerequisites. The pre-flight in `Execute()` can skip checking this file — the symptom of missing this setting is 0 GPUs visible (not a startup error), so a runtime check would not help anyway. Instead, the troubleshooting section in docs should cover "0 GPUs visible" as a known failure mode.

2. **Device plugin version pinning**
   - What we know: v0.17.1 static manifest is available at a versioned URL. The main branch tracks `v0.18.0` in the manifest.
   - What's unclear: Whether to pin at v0.17.1 (verified working) or v0.17.4/v0.18.0 (latest).
   - Recommendation: Embed v0.17.1 (the version verified in research). The file is vendored at build time; upgrading is a single file swap + version bump.

3. **No-GOOS-tag for pre-flight functions**
   - What we know: `checkHostPrerequisites()` uses `osexec.LookPath` which works on all platforms but would always fail on non-Linux (no nvidia-smi on macOS).
   - What's unclear: Whether to use build tags to compile out the Linux-only functions, or gate with `runtime.GOOS` at runtime.
   - Recommendation: Gate with `runtime.GOOS` at runtime (same as existing MetalLB platform warning pattern in `create.go`). Simpler than build tags, and the binary size difference is negligible.

4. **Kinder-site sidebar update**
   - What we know: `astro.config.mjs` manually lists all addon pages; adding `nvidia-gpu.md` requires adding `{ slug: 'addons/nvidia-gpu' }` to the sidebar.
   - What's unclear: Nothing; this is straightforward.
   - Recommendation: Add the sidebar entry in the same commit as the docs page.

## Validation Architecture

No `.planning/config.json` found (key absent = treat nyquist_validation as enabled).

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none (go test ./...) |
| Quick run command | `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/... ./pkg/cmd/kind/doctor/... ./pkg/internal/apis/config/... ./pkg/apis/config/v1alpha4/...` |
| Full suite command | `go test ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| GPU-01 | DaemonSet applied via kubectl apply | unit (FakeNode queue) | `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/...` | Wave 0 |
| GPU-02 | RuntimeClass applied before DaemonSet | unit (FakeNode call order) | `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/...` | Wave 0 |
| GPU-03 | NvidiaGPU *bool field, nil=false in conversion | unit | `go test ./pkg/internal/apis/config/...` | Wave 0 |
| GPU-04 | Non-Linux platforms return nil, log info message | unit (runtime.GOOS mock impossible; test with GOOS=linux skip gate) | `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/...` | Wave 0 |
| GPU-05 | Doctor checks: driver, toolkit, docker runtime | unit (mock LookPath via interface OR skip on non-Linux CI) | `go test ./pkg/cmd/kind/doctor/...` | Wave 0 |
| GPU-06 | Pre-flight error before kubectl apply | unit (FakeNode expects 0 calls when pre-flight fails) | `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/...` | Wave 0 |
| SITE-02 | Documentation page exists with required sections | manual | N/A | Wave 0 |

**Note on GPU-04 testing:** `runtime.GOOS` cannot be mocked at test time. Tests should verify the action executes the Linux path (apply manifests) by running on Linux CI. The non-Linux behavior is verifiable by code review of the guard condition. A table-driven test can inject a `goos string` parameter via a testable function signature if the planner chooses that approach.

### Sampling Rate
- **Per task commit:** `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/... ./pkg/internal/apis/config/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go test ./...` green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu_test.go` — covers GPU-01, GPU-02, GPU-04, GPU-06
- [ ] `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-device-plugin.yaml` — embedded manifest (GPU-01)
- [ ] `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-runtimeclass.yaml` — embedded manifest (GPU-02)
- [ ] Doctor test additions for GPU-05 checks — add to existing `pkg/cmd/kind/doctor/doctor_test.go` (may not exist yet; create)
- [ ] `pkg/internal/apis/config/default_test.go` addition — test NvidiaGPU=false when nil (GPU-03)

## Sources

### Primary (HIGH confidence)
- Project codebase (`pkg/cluster/internal/create/actions/`) — existing addon patterns (embed, FakeNode, action interface, wave integration)
- Project codebase (`pkg/cmd/kind/doctor/doctor.go`) — existing doctor check pattern (LookPath, OutputLines, result struct)
- Project codebase (`pkg/apis/config/v1alpha4/types.go`, `convert_v1alpha4.go`, `zz_generated.deepcopy.go`) — config API extension pattern
- `https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.17.1/deployments/static/nvidia-device-plugin.yml` — DaemonSet manifest (confirmed fetched)

### Secondary (MEDIUM confidence)
- `https://github.com/NVIDIA/nvkind` — exact nvidia-ctk commands required for kind GPU support: `--set-as-default`, `--cdi.enabled`, `accept-nvidia-visible-devices-as-volume-mounts=true`
- `https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html` — installation commands and `nvidia-ctk runtime configure --runtime=docker` pattern
- `https://k3d.io/v5.8.3/usage/advanced/cuda/` — RuntimeClass manifest structure for similar in-Docker Kubernetes (k3d analogue)
- `https://github.com/NVIDIA/k8s-device-plugin` — version v0.17.1 latest stable static manifest, `FAIL_ON_INIT_ERROR=false` flag

### Tertiary (LOW confidence)
- WebSearch results on `docker info --format '{{range $k, $v := .Runtimes}}{{$k}} {{end}}'` for runtime detection — verified against existing `checkBinary` pattern but not against official Docker docs
- `accept-nvidia-visible-devices-as-volume-mounts` requirement for kind specifically — sourced from nvkind README and community gist, not official NVIDIA+kind documentation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — device plugin DaemonSet URL verified, manifest content retrieved, existing addon pattern is well-established in this codebase
- Architecture: HIGH — maps exactly to existing addon pattern (embed + FakeNode tests + wave integration + config API)
- Pitfalls: MEDIUM — wrong default (HIGH confidence it matters), accept-nvidia-visible-devices (MEDIUM, sourced from nvkind + community), RuntimeClass handler (HIGH, basic Kubernetes fact)
- Doctor checks: HIGH — maps directly to existing doctor.go pattern; NVIDIA check commands verified against official NVIDIA docs

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (device plugin version stable; check for v0.18.x release before final implementation)
