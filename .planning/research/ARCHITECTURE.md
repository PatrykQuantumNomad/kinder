# Architecture Research

**Domain:** Distribution pipeline (GoReleaser + GitHub Releases + Homebrew tap) + NVIDIA GPU addon for kinder Go CLI
**Researched:** 2026-03-04
**Confidence:** HIGH — existing code read directly; GoReleaser config from official docs; NVIDIA from official repo

---

## System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                  DISTRIBUTION LAYER (new)                           │
│                                                                     │
│  .goreleaser.yaml         .github/workflows/release.yml            │
│  (builds + archives +     (existing: tag-triggered,                │
│   checksums + brews)       softprops/action-gh-release)            │
│         │                           │                              │
│         └─────────────┬─────────────┘                              │
│                       │                                            │
│         ┌─────────────▼─────────────┐                              │
│         │     github.com/releases   │   ← cross-platform binaries  │
│         │     (kinder-linux-amd64   │     + sha256sums             │
│         │      kinder-darwin-arm64  │                              │
│         │      kinder-windows-amd64 │                              │
│         │      ...)                 │                              │
│         └─────────────┬─────────────┘                              │
│                       │ GoReleaser writes formula                  │
│         ┌─────────────▼─────────────┐                              │
│         │ homebrew-kinder (tap repo) │   ← Formula/kinder.rb       │
│         └───────────────────────────┘                              │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                  ADDON LAYER (existing, extended)                   │
│                                                                     │
│  pkg/cluster/internal/create/create.go                             │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │                  Wave 1 (errgroup, limit 3)                 │    │
│  │  LocalRegistry  MetalLB  MetricsServer  CoreDNSTuning      │    │
│  │  Dashboard      CertManager                                │    │
│  └───────────────────────────┬────────────────────────────────┘    │
│                              │ Wait                                │
│  ┌───────────────────────────▼────────────────────────────────┐    │
│  │               Wave 2 (sequential)                          │    │
│  │  EnvoyGateway     [NvidiaGPU — new, Wave 1 candidate]      │    │
│  └────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  Each addon: pkg/cluster/internal/create/actions/<name>/<name>.go  │
│              go:embed manifests/*.yaml                             │
│              Execute(*ActionContext) error                         │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                  BUILD LAYER (existing, extended)                   │
│                                                                     │
│  Makefile                 hack/release/build/cross.sh              │
│  (local build/install)    (cross-compile 5 targets)                │
│                                                                     │
│  pkg/internal/kindversion/version.go                               │
│  (gitCommit + versionCore injected via -ldflags at build time)     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Component Responsibilities

| Component | Responsibility | New / Modified |
|-----------|----------------|----------------|
| `.goreleaser.yaml` | Cross-compile all targets, package archives, compute checksums, publish GitHub Release, push Homebrew formula | NEW |
| `.github/workflows/release.yml` | Tag-triggered release workflow that invokes GoReleaser | MODIFIED (replace current shell-based release) |
| `github.com/<owner>/homebrew-kinder` | Separate tap repository holding `Formula/kinder.rb` | NEW repo |
| `pkg/cluster/internal/create/actions/installnvidiagpu/` | NVIDIA GPU addon package: embeds DaemonSet manifest, executes device plugin install + RuntimeClass | NEW package |
| `pkg/internal/apis/config/types.go` (internal) | Add `NvidiaGPU bool` field to `Addons` struct | MODIFIED |
| `pkg/apis/config/v1alpha4/types.go` (public) | Add `NvidiaGPU *bool` field to `Addons` struct | MODIFIED |
| `pkg/cluster/internal/create/create.go` | Register NvidiaGPU addon in wave1 slice | MODIFIED |

---

## Distribution Pipeline: GoReleaser Integration

### How GoReleaser maps to the existing build

The existing `Makefile` and `hack/release/build/cross.sh` already produce the correct binaries. GoReleaser replaces `cross.sh` for release builds. The `Makefile`'s `KIND_BUILD_LD_FLAGS` pattern (injecting `gitCommit` and `gitCommitCount` via `-X`) must be replicated in `.goreleaser.yaml`:

```yaml
# .goreleaser.yaml
version: 2

project_name: kinder

builds:
  - binary: kinder
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -trimpath
      - -buildid=
      - -w
      - -X sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .Commit }}
      - -X sigs.k8s.io/kind/pkg/internal/kindversion.gitCommitCount={{ .Env.COMMIT_COUNT }}
    mod_timestamp: "{{ .CommitTimestamp }}"
```

Critical: the `-X` flags must reference `pkg/internal/kindversion.*` — the package that currently holds the build-time injected vars. The existing `Makefile` uses `KIND_VERSION_PKG:=sigs.k8s.io/kind/pkg/internal/kindversion` as the target package path.

### Archive and checksum configuration

```yaml
archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "kinder-{{ .Os }}-{{ .Arch }}"

checksum:
  name_template: "checksums.txt"
  algorithm: sha256
```

The existing `cross.sh` names binaries `kinder-${GOOS}-${GOARCH}` and appends `.sha256sum` files. GoReleaser's `name_template` should match this convention to avoid breaking any users who reference the binary URL pattern.

### Homebrew tap configuration

GoReleaser's `brews` section generates a Ruby formula and pushes it to a separate tap repository (`homebrew-kinder`). The tap repo must exist before first release.

```yaml
brews:
  - name: kinder
    repository:
      owner: "{{ .Env.HOMEBREW_TAP_OWNER }}"
      name: homebrew-kinder
      token: "{{ .Env.HOMEBREW_TOKEN }}"
    commit_author:
      name: goreleaserbot
      email: goreleaser@kinder
    commit_msg_template: "Brew formula update for {{ .ProjectName }} version {{ .Tag }}"
    directory: Formula
    homepage: "https://kinder.sigs.k8s.io"
    description: "batteries-included local Kubernetes clusters (kind fork)"
    license: "Apache-2.0"
    install: |
      bin.install "kinder"
    test: |
      system "#{bin}/kinder", "version"
```

### GitHub Actions workflow change

The existing `release.yml` uses `softprops/action-gh-release` directly. Replace with `goreleaser/goreleaser-action`:

```yaml
# .github/workflows/release.yml — new structure
name: Release
on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # GoReleaser needs full git history for changelog

      - name: Read Go version
        id: go-version
        run: echo "version=$(cat .go-version)" >> "$GITHUB_OUTPUT"

      - uses: actions/setup-go@v5
        with:
          go-version: ${{ steps.go-version.outputs.version }}

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TOKEN: ${{ secrets.HOMEBREW_TOKEN }}
          HOMEBREW_TAP_OWNER: ${{ github.repository_owner }}
```

`HOMEBREW_TOKEN` must be a GitHub PAT with `contents: write` permission scoped to the tap repository. It cannot be the default `GITHUB_TOKEN` (which is scoped to the current repo only).

### New tap repository structure

```
homebrew-kinder/
└── Formula/
    └── kinder.rb        ← generated and updated by GoReleaser on each release
```

GoReleaser generates the formula with the correct download URLs, SHA256 hashes, and OS-specific `on_macos`/`on_linux` blocks automatically.

---

## NVIDIA GPU Addon: Integration with Existing Architecture

### The GPU passthrough problem on kind

kind nodes are Docker/Podman containers. To expose NVIDIA GPUs to pods running inside kind nodes, two pre-conditions must hold on the host:

1. `nvidia-container-toolkit` is installed and configured as the containerd runtime
2. The toolkit is configured with `accept-nvidia-visible-devices-as-volume-mounts=true` (the nvkind approach)

**Without these host pre-conditions, the addon cannot function.** The addon must detect their presence and fail gracefully if absent. This is the central architectural constraint distinguishing GPU from all other addons.

### What the NvidiaGPU addon does at cluster creation time

The addon has no provisioning-time component (no `ContainerdConfigPatches` injection is needed — the host's nvidia-container-toolkit configures this before cluster creation). All steps execute post-cluster-creation via kubectl, exactly like every other addon:

1. Apply NVIDIA device plugin DaemonSet (`manifests/nvidia-device-plugin.yaml`, embedded via `go:embed`)
2. Apply NVIDIA RuntimeClass (`manifests/runtime-class.yaml`, embedded)
3. Wait for DaemonSet to be available on GPU-capable nodes (with tolerance for `NoSchedule` taint on non-GPU nodes)

The DaemonSet manifest is the static YAML from `https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.17.x/deployments/static/nvidia-device-plugin.yml` — pinned version, embedded at build time.

### Package structure (mirrors existing addons exactly)

```
pkg/cluster/internal/create/actions/installnvidiagpu/
├── nvidiagpu.go              ← Execute(*ActionContext) error
└── manifests/
    ├── nvidia-device-plugin.yaml   ← embedded DaemonSet
    └── runtime-class.yaml          ← RuntimeClass "nvidia"
```

`nvidiagpu.go` structure:

```go
package installnvidiagpu

import (
    _ "embed"
    "strings"

    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/nodeutils"
    "sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/nvidia-device-plugin.yaml
var devicePluginManifest string

//go:embed manifests/runtime-class.yaml
var runtimeClassManifest string

type action struct{}

func NewAction() actions.Action { return &action{} }

func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing NVIDIA GPU support")
    defer ctx.Status.End(false)

    allNodes, err := ctx.Nodes()
    // ... get control plane node ...

    // Step 1: Apply RuntimeClass "nvidia"
    if err := node.CommandContext(ctx.Context, "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(runtimeClassManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply NVIDIA RuntimeClass")
    }

    // Step 2: Apply device plugin DaemonSet
    if err := node.CommandContext(ctx.Context, "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(devicePluginManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply NVIDIA device plugin")
    }

    // Step 3: Wait — only for nodes that have GPUs (DaemonSet may have 0 desired pods on GPU-free nodes)
    // Use kubectl rollout status with a short timeout; tolerate failure (warn-and-continue)
    _ = node.CommandContext(ctx.Context, "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "rollout", "status", "daemonset/nvidia-device-plugin-daemonset",
        "-n", "kube-system", "--timeout=60s",
    ).Run()

    ctx.Status.End(true)
    return nil
}
```

### Config API changes (three files must change together)

**1. Public API: `pkg/apis/config/v1alpha4/types.go`**

```go
type Addons struct {
    // ... existing fields unchanged ...
    // NvidiaGPU enables NVIDIA device plugin for GPU workloads.
    // Requires nvidia-container-toolkit on the host.
    // +optional (default: false — opt-in, unlike other addons)
    NvidiaGPU *bool `yaml:"nvidiaGPU,omitempty" json:"nvidiaGPU,omitempty"`
}
```

**2. Internal API: `pkg/internal/apis/config/types.go`**

```go
type Addons struct {
    // ... existing fields unchanged ...
    NvidiaGPU bool
}
```

**3. Conversion: `pkg/internal/apis/config/convert_v1alpha4.go`**

The conversion function that maps `*bool` (public) to `bool` (internal) must handle `NvidiaGPU`. The existing pattern: `nil` → `defaultValue`, `&true` → `true`, `&false` → `false`.

Unlike other addons (default `true`), `NvidiaGPU` defaults to `false` — it is opt-in because it requires host-level prerequisites.

**4. `pkg/apis/config/v1alpha4/default.go`**

Must set `NvidiaGPU` default. Since it's opt-in, the default is `false`, and the default function sets it only if unset:

```go
// NvidiaGPU defaults to false (opt-in: requires host nvidia-container-toolkit)
if cfg.Addons.NvidiaGPU == nil {
    cfg.Addons.NvidiaGPU = boolPointer(false)
}
```

### Wave placement in create.go

`NvidiaGPU` belongs in Wave 1 (parallel, independent). It does not depend on MetalLB and does not conflict with other Wave 1 addons.

```go
// create.go — wave1 slice addition
wave1 := []AddonEntry{
    {"Local Registry", opts.Config.Addons.LocalRegistry, installlocalregistry.NewAction()},
    {"MetalLB",        opts.Config.Addons.MetalLB,        installmetallb.NewAction()},
    {"Metrics Server", opts.Config.Addons.MetricsServer,  installmetricsserver.NewAction()},
    {"CoreDNS Tuning", opts.Config.Addons.CoreDNSTuning,  installcorednstuning.NewAction()},
    {"Dashboard",      opts.Config.Addons.Dashboard,      installdashboard.NewAction()},
    {"Cert Manager",   opts.Config.Addons.CertManager,    installcertmanager.NewAction()},
    {"NVIDIA GPU",     opts.Config.Addons.NvidiaGPU,      installnvidiagpu.NewAction()}, // NEW
}
```

---

## Recommended Project Structure (delta from current)

```
kinder/
├── .goreleaser.yaml                   ← NEW: GoReleaser config (replaces cross.sh in CI)
│
├── .github/
│   └── workflows/
│       └── release.yml                ← MODIFIED: goreleaser-action replaces shell build
│
└── pkg/
    ├── apis/config/v1alpha4/
    │   ├── types.go                   ← MODIFIED: NvidiaGPU *bool in Addons
    │   └── default.go                 ← MODIFIED: NvidiaGPU default = false
    │
    ├── internal/apis/config/
    │   ├── types.go                   ← MODIFIED: NvidiaGPU bool in Addons
    │   └── convert_v1alpha4.go        ← MODIFIED: convert NvidiaGPU field
    │
    └── cluster/internal/create/
        ├── create.go                  ← MODIFIED: add NvidiaGPU to wave1
        └── actions/
            └── installnvidiagpu/      ← NEW package
                ├── nvidiagpu.go       ← Execute() using CommandContext pattern
                └── manifests/
                    ├── nvidia-device-plugin.yaml  ← embedded DaemonSet manifest
                    └── runtime-class.yaml         ← embedded RuntimeClass

(separate GitHub repo)
homebrew-kinder/
└── Formula/
    └── kinder.rb                      ← generated by GoReleaser on tag push
```

### Structure Rationale

- **`.goreleaser.yaml` at repo root:** GoReleaser's required location; consistent with all Go CLI projects using GoReleaser.
- **`installnvidiagpu/` mirrors all existing addon packages exactly:** Same `Execute(*ActionContext) error` interface, same `go:embed manifests/*.yaml` pattern, same package naming convention (`install` + noun).
- **Two embedded YAML files instead of one:** RuntimeClass and DaemonSet serve different purposes and may need independent updates. Keeps manifests small and independently auditable.
- **NvidiaGPU defaults `false`:** Host prerequisites (nvidia-container-toolkit) are not universally present. Silent failure when enabled-by-default would confuse users on GPU-free machines.

---

## Architectural Patterns

### Pattern 1: GoReleaser replaces `hack/release/build/cross.sh` for CI releases

**What:** GoReleaser orchestrates the same cross-compilation (GOOS/GOARCH matrix) that `cross.sh` does via `xargs -P`, but adds archive packaging, SHA256 checksums, GitHub Release creation, and Homebrew formula push in a single tool.

**When to use:** Tag push triggers `release.yml`. Local builds still use `make build`. `cross.sh` can remain for manual release testing but is no longer the CI path.

**Trade-offs:** GoReleaser adds a tool dependency in CI. The `.goreleaser.yaml` must stay in sync with the ldflags in `Makefile`. The existing `cross.sh` output naming (`kinder-${GOOS}-${GOARCH}`) must be matched in the archive `name_template`.

### Pattern 2: Opt-in addon with pre-condition check

**What:** NvidiaGPU addon defaults to `false` (unlike all other addons which default to `true`). The addon's `Execute()` does not validate host GPU presence — it applies manifests and relies on the device plugin DaemonSet to surface GPU-specific errors (the DaemonSet won't schedule pods on non-GPU nodes, which is correct behavior).

**When to use:** Any addon that requires host-level configuration outside kinder's control.

**Trade-offs:** Users must explicitly set `addons.nvidiaGPU: true` in their config YAML or pass a flag. Slightly more friction, but avoids confusing errors on GPU-free machines.

### Pattern 3: Pinned manifest version via `go:embed`

**What:** The device plugin DaemonSet YAML is fetched once at development time, pinned at a specific upstream version, and embedded in the Go binary. No runtime fetching.

**When to use:** All addons. Follows the established pattern used by MetalLB, Envoy Gateway, cert-manager, Dashboard, Local Registry, Metrics Server.

**Trade-offs:** Manifest updates require a kinder release. This is intentional — it prevents silent breakage from upstream manifest changes.

---

## Data Flow

### GoReleaser Release Flow

```
git push tag v0.5.0
    ↓
.github/workflows/release.yml triggers
    ↓
goreleaser/goreleaser-action@v6
    ↓
.goreleaser.yaml: builds section
    → go build (5 GOOS/GOARCH targets in parallel)
    → output: bin/kinder-linux-amd64, bin/kinder-darwin-arm64, ...
    ↓
archives section
    → tar.gz (unix) / zip (windows) per target
    ↓
checksum section
    → checksums.txt (sha256 of all archives)
    ↓
release section (GitHub)
    → creates GitHub Release at tag v0.5.0
    → uploads all archives + checksums.txt as assets
    ↓
brews section
    → generates Formula/kinder.rb with correct download URLs + SHA256
    → commits to homebrew-kinder repo via HOMEBREW_TOKEN
```

### NVIDIA GPU Addon Flow (within cluster creation)

```
kinder create cluster (with addons.nvidiaGPU: true in config)
    ↓
create.go: wave1 goroutine pool
    ↓
installnvidiagpu.Execute(*ActionContext)
    ↓
node.CommandContext(ctx, "kubectl", "apply", "-f", "-")
    └── stdin: runtimeClassManifest (go:embed)
    → creates RuntimeClass "nvidia" in cluster
    ↓
node.CommandContext(ctx, "kubectl", "apply", "-f", "-")
    └── stdin: devicePluginManifest (go:embed)
    → creates DaemonSet "nvidia-device-plugin-daemonset" in kube-system
    ↓
node.CommandContext(ctx, "kubectl", "rollout", "status", ...)
    → waits up to 60s (warn-and-continue on timeout)
    ↓
addonResult{name: "NVIDIA GPU", enabled: true, err: nil/err}
    → collected by wave1 mutex
    ↓
logAddonSummary() includes "NVIDIA GPU installed (Xs)" or "NVIDIA GPU FAILED: ..."
```

### User Install Flow (after Homebrew tap is live)

```
User runs: brew tap <owner>/kinder
    ↓
Homebrew fetches Formula/kinder.rb from homebrew-kinder repo
    ↓
brew install kinder
    ↓
Homebrew downloads kinder-darwin-arm64.tar.gz (or amd64) from GitHub Releases
    → verifies sha256 against checksums in formula
    → extracts and installs kinder binary to $(brew --prefix)/bin/kinder
```

---

## Integration Points

### New vs. Modified Components (explicit)

| Component | New / Modified | What Changes |
|-----------|---------------|--------------|
| `.goreleaser.yaml` | NEW | GoReleaser config for builds, archives, checksums, GitHub Release, Homebrew formula |
| `.github/workflows/release.yml` | MODIFIED | Replace `softprops/action-gh-release` + `cross.sh` with `goreleaser/goreleaser-action@v6` |
| `pkg/apis/config/v1alpha4/types.go` | MODIFIED | `NvidiaGPU *bool` field added to `Addons` struct |
| `pkg/apis/config/v1alpha4/default.go` | MODIFIED | NvidiaGPU default set to `false` |
| `pkg/internal/apis/config/types.go` | MODIFIED | `NvidiaGPU bool` field added to internal `Addons` struct |
| `pkg/internal/apis/config/convert_v1alpha4.go` | MODIFIED | NvidiaGPU conversion from `*bool` to `bool` |
| `pkg/cluster/internal/create/create.go` | MODIFIED | `NvidiaGPU` entry added to `wave1` slice |
| `pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu.go` | NEW | Addon Execute() using CommandContext, two manifests |
| `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-device-plugin.yaml` | NEW | Pinned device plugin DaemonSet YAML (embedded) |
| `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/runtime-class.yaml` | NEW | RuntimeClass "nvidia" YAML (embedded) |
| `homebrew-kinder` (separate GitHub repo) | NEW repo | Tap repository; `Formula/kinder.rb` generated by GoReleaser |
| GitHub repo secrets | NEW | `HOMEBREW_TOKEN` PAT with `contents: write` on homebrew-kinder |

### Boundaries that do NOT change

| Boundary | Reason |
|----------|--------|
| `ActionContext` interface | NvidiaGPU addon uses the existing interface exactly (CommandContext, Nodes(), Logger, Config, Status) |
| Provider abstraction | Docker/Podman/nerdctl providers unchanged; GPU addon uses kubectl-in-node pattern like all other addons |
| Wave system | NvidiaGPU fits in Wave 1 (independent); no new wave needed |
| Config API versioning | `v1alpha4` remains the single version; adding a new `*bool` field with `omitempty` is backward-compatible |
| Binary naming convention | GoReleaser `name_template` matches existing `kinder-${GOOS}-${GOARCH}` pattern from `cross.sh` |

---

## Build Order Recommendation

Dependencies determine order. Each step is independently verifiable.

```
Step 1: GoReleaser setup (no code changes, independent)
    1a. Install goreleaser locally (brew install goreleaser or go install)
    1b. Create .goreleaser.yaml with builds + archives + checksum sections
    1c. goreleaser check (validates config without building)
    1d. goreleaser release --snapshot --clean (local test build)
    1e. Verify ldflags: built binary `kinder version` shows correct commit hash

Step 2: Homebrew tap repo (independent of code changes)
    2a. Create github.com/<owner>/homebrew-kinder repository
    2b. Create Formula/ directory with placeholder
    2c. Add HOMEBREW_TOKEN secret to kinder repo (PAT with contents:write on homebrew-kinder)
    2d. Add brews section to .goreleaser.yaml
    2e. goreleaser check (re-validate)

Step 3: Update release workflow (depends on Steps 1 and 2)
    3a. Replace release.yml content with goreleaser-action workflow
    3b. git push tag vX.Y.Z-test to validate end-to-end
    3c. Verify GitHub Release created, binaries attached, formula committed to tap

Step 4: NvidiaGPU addon — config API layer (depends on nothing else)
    4a. Add NvidiaGPU *bool to pkg/apis/config/v1alpha4/types.go
    4b. Add NvidiaGPU default (false) to pkg/apis/config/v1alpha4/default.go
    4c. Add NvidiaGPU bool to pkg/internal/apis/config/types.go
    4d. Update convert_v1alpha4.go conversion function
    4e. go build ./... (must compile — all config consumers see new field as zero value)
    4f. go test ./pkg/internal/apis/config/...

Step 5: NvidiaGPU addon — action package (depends on Step 4)
    5a. Fetch pinned device plugin YAML from NVIDIA/k8s-device-plugin repo
    5b. Create runtime-class.yaml (RuntimeClass "nvidia")
    5c. Create pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu.go
    5d. go build ./pkg/cluster/internal/create/actions/installnvidiagpu/...

Step 6: Wire addon into create.go (depends on Steps 4 and 5)
    6a. Add import for installnvidiagpu package in create.go
    6b. Add AddonEntry to wave1 slice
    6c. go build ./...
    6d. Manual test: kinder create cluster (GPU addon skipped — defaults false)
    6e. Manual test on GPU-equipped Linux host: kinder create cluster with nvidiaGPU: true
```

---

## Anti-Patterns

### Anti-Pattern 1: Defaulting NvidiaGPU to true

**What people do:** Add `NvidiaGPU` to the Addons struct with default `true` to match all other addons.

**Why it's wrong:** On machines without NVIDIA drivers and `nvidia-container-toolkit`, the device plugin DaemonSet will enter `CrashLoopBackOff`. This makes `kinder create cluster` fail or produce confusing output for the vast majority of users who don't have GPUs.

**Do this instead:** Default `NvidiaGPU` to `false`. Users with GPU setups opt-in via `addons.nvidiaGPU: true` in config or a `--addon nvidia-gpu` flag (if a flag-based addon toggle is added). Document the host prerequisites clearly.

### Anti-Pattern 2: Diverging the ldflags between Makefile and .goreleaser.yaml

**What people do:** Copy the `go build` command from `cross.sh` into `.goreleaser.yaml` without checking the `-X` package path. The `Makefile` injects version vars at `pkg/internal/kindversion.*` but `.goreleaser.yaml` is written with `pkg/cmd/kind/version.*`.

**Why it's wrong:** Release binaries show empty version strings or wrong commit hashes. Users see `kinder v` or `kind v0.0.0` instead of the real version.

**Do this instead:** Explicitly verify by running `goreleaser release --snapshot --clean` and checking `./dist/kinder_linux_amd64_v1/kinder version` output before merging the workflow change.

### Anti-Pattern 3: Using GITHUB_TOKEN for the Homebrew tap push

**What people do:** Pass `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` as the token for both the GitHub Release creation and the Homebrew formula push to the tap repo.

**Why it's wrong:** `GITHUB_TOKEN` is scoped to the triggering repository only. GoReleaser cannot push to `homebrew-kinder` (a different repo) with this token and will fail with a 403 or "no permission" error.

**Do this instead:** Create a PAT (classic or fine-grained) with `contents: write` permission on `homebrew-kinder`, store it as `HOMEBREW_TOKEN` secret in the kinder repo, and reference it in the brews `token` field of `.goreleaser.yaml`.

### Anti-Pattern 4: Applying the GPU DaemonSet with `kubectl apply` (standard) when the manifest exceeds annotation limits

**What people do:** Use `kubectl apply -f -` without `--server-side` for the device plugin manifest.

**Why it's wrong:** Large YAML manifests (> 256 KB) fail with standard `apply` due to the `last-applied-configuration` annotation size limit. The existing `installenvoygw` addon already encountered this and explicitly uses `--server-side`. The NVIDIA device plugin manifest is smaller but this pattern must be checked.

**Do this instead:** Check manifest size before embedding. If > ~100 KB, use `--server-side`. For the device plugin's static manifest (which is small), standard apply is fine, but add a comment documenting the size check.

### Anti-Pattern 5: Embedding the "latest" device plugin manifest URL

**What people do:** Instead of embedding a pinned manifest, use `kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/main/deployments/static/nvidia-device-plugin.yml` (pulling from main branch at runtime).

**Why it's wrong:** Runtime manifest fetching requires internet access in the kind node container and ties kinder to whatever NVIDIA ships at that moment. This breaks offline use and creates reproducibility issues.

**Do this instead:** Pin the manifest to a specific version tag (e.g., `v0.17.1`), embed it via `go:embed`, and update it deliberately with a kinder release when the device plugin has a significant new version.

---

## Scaling Considerations

| Concern | Now (no distribution) | After GoReleaser | At 10k+ users |
|---------|----------------------|-----------------|---------------|
| Binary distribution | Source-only, `make install` | Pre-built binaries via GitHub Releases + Homebrew | No change needed — GitHub Releases scales |
| Addon manifest updates | Edit .yaml + rebuild | Same + kinder release | Same |
| GPU adoption | No GPU addon | Opt-in GPU addon | Consider GPU-specific node image if demand grows |
| Release cadence | Manual `cross.sh` + manual GitHub Release | Fully automated on tag push | No change needed |

---

## Sources

- Kinder codebase, direct read — `pkg/cluster/internal/create/create.go`, `actions/action.go`, all addon packages, `pkg/apis/config/v1alpha4/types.go`, `pkg/internal/apis/config/types.go`, `Makefile`, `hack/release/build/cross.sh`, `.github/workflows/release.yml` — HIGH confidence
- GoReleaser Homebrew Formulas docs: [goreleaser.com/customization/homebrew_formulas/](https://goreleaser.com/customization/homebrew_formulas/) — HIGH confidence
- GoReleaser GitHub Actions docs: [goreleaser.com/ci/actions/](https://goreleaser.com/ci/actions/) — HIGH confidence
- GoReleaser Go builds docs: [goreleaser.com/customization/builds/go/](https://goreleaser.com/customization/builds/go/) — HIGH confidence
- NVIDIA k8s-device-plugin: [github.com/NVIDIA/k8s-device-plugin](https://github.com/NVIDIA/k8s-device-plugin) — HIGH confidence
- nvkind (NVIDIA's kind+GPU tool): [github.com/NVIDIA/nvkind](https://github.com/NVIDIA/nvkind) — MEDIUM confidence (verify GPU passthrough mechanism before implementation)
- NVIDIA GPU Operator docs: [docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/index.html) — MEDIUM confidence (GPU Operator is heavier than device-plugin-only approach)
- GoReleaser homebrew token discussion: [github.com/orgs/goreleaser/discussions/4926](https://github.com/orgs/goreleaser/discussions/4926) — MEDIUM confidence

---
*Architecture research for: kinder distribution pipeline (GoReleaser + GitHub Releases + Homebrew) + NVIDIA GPU addon*
*Researched: 2026-03-04*
