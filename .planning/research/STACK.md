# Technology Stack

**Project:** kinder — Next milestone (Distribution pipeline + NVIDIA GPU addon)
**Scope:** GoReleaser cross-platform builds, GitHub Releases, Homebrew custom tap, NVIDIA GPU Operator addon
**Researched:** 2026-03-04
**Overall confidence:** HIGH for distribution pipeline (all versions live-verified via GitHub API). MEDIUM for NVIDIA GPU addon in kind context (GPU Operator not officially validated for kind; pattern established via community tools).

---

## Executive Summary: What Actually Changes

This milestone adds two orthogonal concerns:

1. **Distribution pipeline** — replaces the hand-rolled `hack/release/build/cross.sh` + `softprops/action-gh-release@v2` workflow with GoReleaser. No Go module changes. No new Go dependencies. Pure tooling and workflow change.

2. **NVIDIA GPU addon** — adds an 8th addon (`installnvidiagpu`) using the same go:embed + kubectl apply pattern as the existing 7 addons. The embedded manifests install NVIDIA GPU Operator v25.10.1 via Helm (or kubectl-applied pre-rendered manifests). The Go code change is minimal; the complexity is operational (host prerequisites for real GPU access).

**go.mod does not change for this milestone.** GoReleaser is a binary tool, not a Go library dependency.

---

## Recommended Stack

### Core Technologies — Distribution Pipeline

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| GoReleaser OSS | v2.14.1 | Cross-platform binary builds, GitHub Release creation, Homebrew tap automation | Industry standard for Go CLI tools; single `.goreleaser.yaml` replaces `cross.sh` + `softprops` action; built-in checksum generation; active development (released 2026-02-25) |
| goreleaser-action | v7.0.0 | GitHub Actions integration for GoReleaser | Official action; v7 defaults to GoReleaser v2; released 2026-02-21 |
| GitHub Releases | (platform) | Binary artifact hosting | Free, native to the existing `softprops/action-gh-release@v2` workflow being replaced; GoReleaser publishes here directly |
| homebrew-tap repository | (new repo) | Custom Homebrew tap hosting Homebrew Casks for kinder | Required by GoReleaser `homebrew_casks` feature; must be named `homebrew-kinder` under `PatrykQuantumNomad` org |

### Core Technologies — NVIDIA GPU Addon

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| NVIDIA GPU Operator | v25.10.1 (Helm chart) | Manages GPU driver, container toolkit, device plugin lifecycle in Kubernetes | Full-stack GPU automation; single Helm install handles driver detection, device plugin, monitoring; latest stable release (2025-12-04) |
| NVIDIA k8s-device-plugin | v0.18.2 | Exposes GPU resources to Kubernetes scheduler | Alternative to full GPU Operator for pre-driver environments; lighter weight; latest release (2026-01-23) |

**GPU addon implementation decision:** Use GPU Operator (not device plugin alone) because kinder's value proposition is "batteries included" — the Operator handles the full stack. Implement with `driver.enabled=false` mode so the addon works when the host already has NVIDIA drivers installed (the common case for developer machines with NVIDIA GPUs).

### Supporting Tools

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| GitHub Personal Access Token (PAT) | — | Authorize GoReleaser to push to `homebrew-kinder` tap repo | Required — `GITHUB_TOKEN` from the release workflow cannot write to a different repository; needs `contents: write` scope on the tap repo |
| Helm v3 | latest stable | Render NVIDIA GPU Operator manifests for embedding | Use during addon development to generate static manifests to embed; kinder cluster creation does NOT run Helm at runtime |

---

## Installation — Distribution Pipeline

### 1. Install GoReleaser locally (development only)

```bash
# macOS via Homebrew
brew install goreleaser

# Or via Go install (not recommended — installs main branch)
# Instead, download the binary from GitHub Releases
```

### 2. Create the `.goreleaser.yaml` at repo root

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: kinder
    main: .
    binary: kinder
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
      - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{.Commit}}
      - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommitCount={{.Env.COMMIT_COUNT}}
    mod_timestamp: "{{.CommitTimestamp}}"

archives:
  - id: kinder
    name_template: >-
      {{- .ProjectName }}_
      {{- title .Os }}_
      {{- if eq .Arch "amd64" }}x86_64
      {{- else if eq .Arch "arm64" }}arm64
      {{- else }}{{ .Arch }}{{ end }}
    format_overrides:
      - goos: windows
        formats:
          - zip
    files:
      - LICENSE

checksum:
  name_template: "{{ .ProjectName }}_{{ .Version }}_checksums.txt"
  algorithm: sha256

release:
  github:
    owner: PatrykQuantumNomad
    name: kinder
  draft: false
  generate_release_notes: true

homebrew_casks:
  - name: kinder
    repository:
      owner: PatrykQuantumNomad
      name: homebrew-kinder
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: "https://github.com/PatrykQuantumNomad/kinder"
    description: "Batteries-included local Kubernetes clusters (fork of kind)"
    license: Apache-2.0
    conflicts:
      - formula: kinder

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
```

**Why `homebrew_casks` not `brews`:** The `brews` section generates Homebrew Formulas, which are semantically intended for source builds. GoReleaser deprecated `brews` since v2.10 in favor of `homebrew_casks`, which correctly represents pre-compiled binary distribution. The `brews` section will be removed in GoReleaser v3.

**Why `ldflags` use the existing version package path:** The Makefile already injects `gitCommit` and `gitCommitCount` via `-X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit=...`. GoReleaser must target the same variables. The `versionCore` constant (`0.4.1`) stays in the source; GoReleaser sets only the build-time metadata fields.

**Note on `COMMIT_COUNT`:** The Makefile derives `COMMIT_COUNT` via `git describe --tags | rev | cut -d- -f2 | rev`. GoReleaser does not have a built-in template for this. Options: (a) pre-compute in a before hook and write to a file, (b) use `{{.Env.COMMIT_COUNT}}` and set it in the CI workflow before calling GoReleaser, or (c) simplify: leave `gitCommitCount` empty in release builds (it only appears in pre-release versions). **Recommended:** Option (c) — leave it empty. Release tags have `versionPreRelease = ""` so `gitCommitCount` is unused in the version string for tagged releases.

### 3. Create the GitHub Actions release workflow

Replace `.github/workflows/release.yml` entirely:

```yaml
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
          fetch-depth: 0  # required — GoReleaser uses git history for changelog

      - uses: actions/setup-go@v5
        with:
          go-version-file: .go-version

      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

### 4. Create the Homebrew tap repository

1. Create a new GitHub repository: `PatrykQuantumNomad/homebrew-kinder`
2. Initialize with a `Casks/` directory
3. Create a GitHub PAT with `contents: write` scope on this repo
4. Add the PAT as `HOMEBREW_TAP_TOKEN` in the kinder repo's Actions secrets

Users install via:
```bash
brew tap PatrykQuantumNomad/kinder
brew install --cask kinder
```

### 5. NVIDIA GPU addon — embedded manifest approach

The addon follows the identical pattern to the existing 7 addons:

```go
// pkg/cluster/internal/create/actions/installnvidiagpu/action.go
//go:embed manifests/gpu-operator.yaml
var gpuOperatorManifest []byte
```

The `gpu-operator.yaml` is a pre-rendered Helm manifest (generated offline with `helm template`) for GPU Operator v25.10.1 with `driver.enabled=false`. This avoids requiring Helm at runtime (consistent with kinder's design — all addon manifests are embedded, not fetched at runtime).

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| GoReleaser v2.14.1 | Keep existing `cross.sh` + `softprops/action-gh-release@v2` | If Homebrew tap is NOT needed and binary naming is already correct. The existing script produces valid binaries. GoReleaser's added value is Homebrew Cask automation + checksum file + structured changelog. |
| `homebrew_casks` section | `brews` section (deprecated) | Never for new configurations — `brews` is deprecated since v2.10 and will be removed in v3 |
| GPU Operator v25.10.1 with `driver.enabled=false` | k8s-device-plugin v0.18.2 alone (DaemonSet) | When users want minimum footprint and ALREADY have nvidia-container-toolkit configured on the host. The device plugin alone is simpler (one DaemonSet) but requires the host container toolkit to be configured, which the Operator handles automatically. |
| Pre-rendered Helm manifests embedded via go:embed | Runtime Helm install from the addon `Execute()` method | Never — kinder does not and should not have Helm as a runtime dependency. All existing addons use embedded manifests. |
| goreleaser-action@v7 | softprops/action-gh-release@v2 (existing) | Keep `softprops` only if NOT using GoReleaser. They are mutually exclusive — GoReleaser handles the release creation itself. |

---

## What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| GoReleaser Pro | OSS version has all needed features (cross-platform builds, Homebrew Cask, GitHub Releases). Pro adds Docker manifests, signing, Kubernetes manifests — none needed here. | GoReleaser OSS v2.14.1 |
| `nfpm` (deb/rpm/apk packaging) | kinder targets developers on macOS/Linux who use Homebrew or direct binary download; system package manager support adds significant complexity for minimal gain at this stage | Homebrew Cask + GitHub binary download |
| Runtime Helm dependency in kinder | All existing addons use embedded manifests (`go:embed`). Adding `helm install` at runtime breaks this design, requires Helm on PATH, and makes addon behavior unpredictable on user machines without Helm | Pre-render with `helm template` offline; embed the output |
| NVIDIA driver installation inside the addon | Installing NVIDIA drivers requires root, reboots, and kernel module loading — none of which are achievable inside a container/addon context. Driver installation is a HOST-level concern. | Document as prerequisite; use `driver.enabled=false` in GPU Operator config |
| `brews` section in `.goreleaser.yaml` | Deprecated since v2.10; generates incorrect Homebrew Formula instead of Cask; will be removed in v3 | `homebrew_casks` section |
| Docker image builds in GoReleaser | kinder is a CLI tool that orchestrates Docker, not a Docker image itself. GoReleaser Docker manifest support is for containerized apps. | Skip — no `dockers:` section in `.goreleaser.yaml` |
| `--output json` for `goreleaser release` in CI | Not needed; GoReleaser's human-readable output is sufficient for CI logs | Default output |

---

## Version Compatibility

| Component | Version | Compatible With | Notes |
|-----------|---------|-----------------|-------|
| GoReleaser OSS | v2.14.1 | Go 1.24 (kinder's Go version) | GoReleaser builds Go binaries regardless of the Go version used to build GoReleaser itself; compatible |
| goreleaser-action | v7.0.0 | GoReleaser v2.x | v7 defaults to `~> v2`; released 2026-02-21; replaces v6 |
| NVIDIA GPU Operator | v25.10.1 | Kubernetes v1.29–v1.34 | Kind uses containerd; GPU Operator v25.10.1 added containerd v2.0 support; minimum K8s v1.29 |
| NVIDIA k8s-device-plugin | v0.18.2 | Kubernetes v1.28+ | Used as a component inside the GPU Operator; not installed separately |
| homebrew_casks section | GoReleaser v2.10+ | GoReleaser v2.14.1 | Introduced v2.10; stable in v2.14.1 |
| `softprops/action-gh-release@v2` | (REMOVED) | — | Replaced entirely by GoReleaser; keep in existing workflows only until GoReleaser migration is complete |

---

## Integration Points — Existing Build System

### `.goreleaser.yaml` replaces `hack/release/build/cross.sh`

The existing script produces these binaries:
```
bin/kinder-windows-amd64
bin/kinder-darwin-amd64
bin/kinder-darwin-arm64
bin/kinder-linux-amd64
bin/kinder-linux-arm64
```

GoReleaser produces archives (`.tar.gz` / `.zip`) instead of raw binaries in `dist/`, which is the correct format for Homebrew Cask and GitHub Releases. The existing Makefile `build` and `install` targets are UNAFFECTED — they continue to work for local development.

### Makefile additions (minimal)

```makefile
# Validate goreleaser config locally (does not build or release)
goreleaser-check:
	goreleaser check

# Dry-run release (builds binaries, does not publish)
goreleaser-snapshot:
	goreleaser release --snapshot --clean
```

Add these as `.PHONY` targets. Do NOT modify the existing `build`, `install`, `test`, or `kind` targets.

### `.github/workflows/release.yml` — full replacement

The existing workflow calls `hack/release/build/cross.sh` then `softprops/action-gh-release@v2`. Replace it entirely with the goreleaser-action workflow shown above. Keep `hack/release/build/cross.sh` in the repository (do not delete it) until the GoReleaser workflow is validated in production.

### GPU addon — new files only

```
pkg/cluster/internal/create/actions/installnvidiagpu/
  action.go                 # Execute() method — same pattern as installcertmanager
  manifests/
    gpu-operator.yaml       # Pre-rendered helm template output for GPU Operator v25.10.1
```

The addon is registered in `create.go` alongside the existing 7 addons (or in the addon registry slice introduced in v1.4). The v1alpha4 config API gains a `NvidiaGPU *bool` field (same `*bool` pattern as all other addons).

---

## GPU Operator Helm Template Command (for manifest pre-rendering)

Run this once to generate the embedded manifest. NOT run at kinder runtime.

```bash
# Add NVIDIA Helm repo
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia
helm repo update

# Pre-render with driver disabled (host has drivers), toolkit enabled, device plugin enabled
helm template gpu-operator nvidia/gpu-operator \
  --version v25.10.1 \
  --namespace gpu-operator \
  --create-namespace \
  --set driver.enabled=false \
  --set toolkit.enabled=true \
  --set devicePlugin.enabled=true \
  --set migManager.enabled=false \
  --set nfd.enabled=true \
  > pkg/cluster/internal/create/actions/installnvidiagpu/manifests/gpu-operator.yaml
```

**Why `driver.enabled=false`:** On a developer machine with an NVIDIA GPU, the NVIDIA driver is pre-installed on the host. The kind node container inherits GPU access from the host via the NVIDIA Container Runtime. Installing the driver again inside the cluster is unnecessary and prone to failure in containerized environments. The GPU Operator detects pre-installed drivers and skips the driver pod automatically even with `driver.enabled=true`, but setting it explicitly `false` avoids the detection delay and the init container overhead.

**Why `nfd.enabled=true`:** Node Feature Discovery labels nodes with `nvidia.com/gpu=true` which gates GPU workloads. This is required for the device plugin to correctly expose GPUs as schedulable resources.

**Caveat — no real GPU required for addon installation:** The GPU Operator will install and remain in a `Pending`/`NotReady` state on nodes without NVIDIA hardware. The addon succeeds (manifests applied) but GPU workloads will not run. This is acceptable — the same behavior as MetalLB on non-Docker networks. Document this clearly.

---

## Sources

- GoReleaser OSS latest release: `github.com/goreleaser/goreleaser` releases API — v2.14.1, released 2026-02-25 (HIGH confidence — live-verified via GitHub API)
- goreleaser-action latest: `github.com/goreleaser/goreleaser-action` releases API — v7.0.0, released 2026-02-21 (HIGH confidence — live-verified)
- NVIDIA GPU Operator latest: `github.com/NVIDIA/gpu-operator` releases API — v25.10.1, released 2025-12-04 (HIGH confidence — live-verified)
- NVIDIA k8s-device-plugin latest: `github.com/NVIDIA/k8s-device-plugin` releases API — v0.18.2, released 2026-01-23 (HIGH confidence — live-verified)
- NVIDIA Container Toolkit latest: `github.com/NVIDIA/nvidia-container-toolkit` — v1.18.2, released 2026-01-23 (HIGH confidence — live-verified)
- GoReleaser `homebrew_casks` documentation: https://goreleaser.com/customization/homebrew_casks/ — confirmed as replacement for deprecated `brews` (HIGH confidence)
- GoReleaser deprecations: https://goreleaser.com/deprecations/ — `brews` deprecated since v2.10 (HIGH confidence)
- GoReleaser GitHub Actions documentation: https://goreleaser.com/ci/actions/ — workflow structure confirmed (HIGH confidence)
- NVIDIA GPU Operator getting started: https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/getting-started.html — Helm values, `driver.enabled=false` semantics (HIGH confidence)
- GPU Operator + kind compatibility: SeineAI/nvidia-kind-deploy community reference (MEDIUM confidence — community pattern, not official NVIDIA validation)
- GoReleaser v2.14 announcement: https://goreleaser.com/blog/goreleaser-v2.14/ — `homebrew_casks` improvements confirmed for 2026-02-21 (HIGH confidence)
- Homebrew tap structure: https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap — `Casks/` directory convention confirmed (HIGH confidence)
- Kinder codebase: direct reads of `Makefile`, `.github/workflows/release.yml`, `hack/release/build/cross.sh`, `pkg/internal/kindversion/version.go`, `go.mod` (HIGH confidence — primary source)

---
*Stack research for: kinder — distribution pipeline (GoReleaser + GitHub Releases + Homebrew tap) and NVIDIA GPU addon*
*Researched: 2026-03-04*
