# Feature Research

**Domain:** Go CLI distribution pipeline (GoReleaser + GitHub Releases + Homebrew tap) and NVIDIA GPU addon for kind-based Kubernetes tool
**Researched:** 2026-03-04
**Confidence:** HIGH for GoReleaser/Homebrew (official docs + verified patterns); MEDIUM for GPU addon (Linux-only platform constraint confirmed; kind-specific approach synthesized from community sources)

---

## Context and Scope

This research covers the **new features** for the v2.0 milestone only. The existing kinder codebase already ships:
- `kinder create cluster` with 7 addons, profiles, JSON output, wave-based parallel install
- `kinder env`, `kinder doctor`, `kinder get clusters/nodes`
- Website at kinder.patrykgolabek.dev
- Release pipeline via `hack/release/build/cross.sh` + `softprops/action-gh-release@v2`

v2.0 adds three independent feature areas:

1. **GoReleaser** — replace the hand-rolled `cross.sh` script with GoReleaser for cross-platform binary builds, checksums, changelogs, and archive packaging
2. **Homebrew tap** — `brew install patrykquantumnomad/kinder/kinder` installation path via a `homebrew-kinder` tap repository
3. **NVIDIA GPU addon** — `--addons nvidia` (or profile) that enables CUDA/GPU workloads in a kind cluster on Linux

These three areas have different dependency relationships: GoReleaser is a prerequisite for the Homebrew tap (taps pull binaries from GitHub Releases). The GPU addon is fully independent of distribution.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist when they discover the project. Missing these = product feels incomplete for its stated scope.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Pre-built binaries on GitHub Releases | Any Go CLI published to GitHub is expected to have pre-built binaries; developers don't want to install Go toolchain just to try a tool | LOW | Cross-compilation is already working via `cross.sh`; GoReleaser automates and formalizes it |
| SHA-256 checksums for each binary | Security-conscious users verify downloads; `sha256sum -c` is the standard; GitHub Releases without checksums feel unprofessional | LOW | GoReleaser generates `checksums.txt` automatically from all archives |
| Platform coverage: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 | Same platforms as `cross.sh` already builds; must not regress | LOW | Already in `cross.sh`; map to `goos`/`goarch` in `.goreleaser.yaml` |
| Automated changelog generation | Every GitHub Release without a changelog just says "v2.0.0" — users want to know what changed | LOW | GoReleaser generates changelog from git commits; configurable to filter by prefix |
| `brew install` installation path | macOS and Linux developers expect Homebrew installation for CLI tools; `make install` is a barrier to adoption | MEDIUM | Requires a separate `homebrew-kinder` repository and a `GH_TAP_TOKEN` secret |
| GPU workload support (`nvidia.com/gpu` resource) | AI/ML developers running local experiments expect a way to test GPU-scheduled pods locally without cloud; kinder claims "batteries included" | HIGH | Linux-only; host prerequisites required (NVIDIA drivers, container toolkit); see GPU section |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| `brew install patrykquantumnomad/kinder/kinder` (tap, not core) | One command install instead of `make install`; lowers friction for new users; only kinder-specific tool with this path — kind requires manual download or brew from core | MEDIUM | Uses `homebrew_casks` section in `.goreleaser.yaml` (v2.10+ feature); publishes to separate `homebrew-kinder` repo |
| Reproducible builds (`-trimpath`, `-buildid=`, `mod_timestamp`) | kinder already does this in `Makefile`; GoReleaser carries this forward with `ldflags` preserving the same version metadata via `-X` flags | LOW | Map existing `KIND_BUILD_LD_FLAGS` to GoReleaser `ldflags`; use `{{ .CommitTimestamp }}` for `mod_timestamp` |
| Version metadata baked in (`kinder version` shows real tag) | Kind-based forks that just build from source often have empty or wrong version strings; GoReleaser injects `{{.Version}}`, `{{.Commit}}`, `{{.Date}}` | LOW | Extend existing `KIND_VERSION_PKG` vars; GoReleaser templates map 1:1 to Makefile's `-X` flags |
| NVIDIA GPU addon as a first-class kinder addon | No kind-based tool has a GPU addon; minikube has GPU support via driver abstraction; kinder's addon model makes this a natural extension; useful for local ML/AI workload testing | HIGH | Linux-only; requires host NVIDIA stack; kinder emits a clear error/warning on unsupported platforms |
| GPU time-slicing support via device plugin ConfigMap | `nvidia.com/gpu: 1` on each pod is too coarse; time-slicing lets multiple pods share one GPU; useful for local testing when a developer has only one GPU | HIGH | Requires NVIDIA k8s-device-plugin ConfigMap with `sharing.timeSlicing.resources`; adds complexity but is the differentiator from just "install the device plugin" |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Publish to `homebrew-core` (not a tap) | "I don't want to `brew tap` first" | Homebrew core requires: stable tarballs, build from source, 30-day review cycle, project maturity checks; kinder is a fork with a non-standard module path (`sigs.k8s.io/kind`); it would be rejected | Custom tap is the correct path for tools not ready for core; `brew tap && brew install` is two commands, not a meaningful barrier |
| Windows binaries distributed via Chocolatey/Winget | Complete platform parity feels professional | kind/kinder has limited Windows support (Docker Desktop only, no native containers); the added maintenance of Chocolatey package manifests outweighs the benefit for a local K8s tool | Ship Windows binaries in GitHub Releases; let users download manually; add Chocolatey/Winget only after Windows feedback confirms demand |
| NVIDIA GPU operator (full operator stack) instead of device plugin | GPU Operator is the "production" approach; feels more complete | GPU Operator is NOT supported on kind (confirmed: kind is not on NVIDIA's supported platform list); it requires privileged daemonsets that conflict with kind's isolation; it installs NVIDIA drivers into containers — unnecessary when the host already has drivers | Use the standalone NVIDIA k8s-device-plugin (daemonset only); this is what works with kind and is what community guides use |
| macOS GPU support (Apple M-series MPS/Metal) | Apple Silicon is common among developers | CUDA is a Linux/Windows technology; NVIDIA GPUs don't exist on Apple Silicon Macs; Apple's MPS is not the same as CUDA and has no Kubernetes device plugin | Document clearly: GPU addon is Linux+NVIDIA only; add `kinder doctor` check for GPU prerequisites |
| GPU support via Docker Desktop GPU mode | Seems simpler than configuring NVIDIA container toolkit | Docker Desktop's GPU passthrough is experimental, only on Windows WSL2, and doesn't expose GPUs to Kubernetes via the device plugin API | Use native Linux containerd NVIDIA runtime configuration; this is the only path that reliably works with kind |
| Automated Homebrew cask notarization | macOS Gatekeeper blocks unsigned binaries | Apple's notarization requires: Apple Developer account ($99/year), build on macOS CI runners (expensive), entitlements configuration | Add a `caveats` message in the cask explaining how to remove quarantine: `sudo xattr -d com.apple.quarantine /usr/local/bin/kinder`; this is what most OSS tools do |
| Multi-architecture Docker images for kinder itself | OCI images feel modern | kinder IS a local Docker tool — publishing kinder as a Docker image is circular and confusing; users run kinder on their host, not inside a container | Skip Docker images for the kinder binary; this is not a server-side tool |

---

## Feature Dependencies

```
GoReleaser Setup
    requires──> .goreleaser.yaml config file (new)
    requires──> GitHub Actions workflow updated (replace cross.sh call with goreleaser-action)
    requires──> GITHUB_TOKEN with contents:write (already present in existing release.yml)
    enables──> Homebrew tap publishing (casks need binaries in GitHub Releases)
    enables──> Reproducible binary naming (goreleaser archive naming used by tap formula)
    replaces──> hack/release/build/cross.sh (manual parallel xargs build)
    replaces──> softprops/action-gh-release@v2 (goreleaser-action handles releases)

Homebrew Tap
    requires──> GoReleaser setup (tap formula references release archives by URL)
    requires──> homebrew-kinder repository (github.com/PatrykQuantumNomad/homebrew-kinder)
    requires──> GH_TAP_TOKEN secret (PAT with repo scope on homebrew-kinder repo — GITHUB_TOKEN only has access to current repo)
    requires──> homebrew_casks section in .goreleaser.yaml (v2.10+ feature)
    enables──> brew install patrykquantumnomad/kinder/kinder

NVIDIA GPU Addon
    requires──> containerdConfigPatches in kind cluster config (existing kinder mechanism — already used for local registry)
    requires──> NVIDIA container runtime configured on HOST before kinder create cluster (host prerequisite, not kinder's responsibility)
    requires──> GPU-capable Linux host (NVIDIA drivers, nvidia-container-toolkit, set accept-nvidia-visible-devices-as-volume-mounts=true)
    requires──> k8s-device-plugin DaemonSet manifest (embedded via go:embed, same pattern as all other addons)
    requires──> New Addons field in config (same pattern as LocalRegistry, CertManager additions in v1.3)
    optionally──> time-slicing ConfigMap (separate embedded manifest, conditionally applied)
    conflicts──> macOS/Windows hosts (emit clear error from addon, checked in Execute())
    no dependency on──> GoReleaser or Homebrew tap (fully independent)

kinder doctor GPU check
    requires──> NVIDIA GPU addon to exist (adds GPU-specific checks)
    enhances──> GPU addon UX (pre-flight validation with actionable error messages)
    no dependency on──> GoReleaser or Homebrew tap
```

### Dependency Notes

- **GoReleaser before Homebrew:** The Homebrew cask formula references GitHub Release archive URLs. GoReleaser must be set up and produce a tagged release before the tap formula can be tested.
- **GPU addon is independent.** It can be developed, merged, and shipped entirely separately from the distribution pipeline. The addon uses the identical pattern to LocalRegistry and CertManager from v1.3.
- **homebrew-kinder repository must exist before any release.** GoReleaser will fail to push the cask update if the tap repository does not exist. Create the `homebrew-kinder` repo and the `GH_TAP_TOKEN` secret before tagging v2.0.0.
- **GH_TAP_TOKEN is different from GITHUB_TOKEN.** The `GITHUB_TOKEN` generated by GitHub Actions only has write access to the repository where the workflow runs. Publishing to a separate `homebrew-kinder` repo requires a Personal Access Token with `repo` scope, stored as a repository secret.

---

## MVP Definition

### Launch With (v2.0)

- [ ] **GoReleaser config (`.goreleaser.yaml`)** — replaces `cross.sh`; produces archives for all 5 platforms; injects version metadata; generates `checksums.txt`; auto-generates changelog; updates GitHub Release
- [ ] **Updated release.yml** — replace `cross.sh` + `softprops/action-gh-release` with `goreleaser/goreleaser-action@v7`; same trigger (push tag `v*`); requires `contents: write` permission (already present)
- [ ] **Homebrew tap repository** (`PatrykQuantumNomad/homebrew-kinder`) — created before tagging; contains initial stub or GoReleaser-managed formula
- [ ] **`homebrew_casks` section in `.goreleaser.yaml`** — publishes cask to homebrew-kinder on every tagged release via `GH_TAP_TOKEN`
- [ ] **NVIDIA GPU addon** — new addon following exact same pattern as existing addons; guarded by `cfg.Addons.NVIDIA` bool; emits clear error on non-Linux or missing prerequisites; embeds device plugin DaemonSet manifest via `go:embed`
- [ ] **`kinder doctor` GPU check** — adds GPU prerequisite checks when GPU addon is enabled: NVIDIA device detected, nvidia-container-toolkit installed, runtime configured

### Add After Validation (v2.x)

- [ ] **GPU time-slicing ConfigMap** — `nvidia.com/gpu` time-slicing for multi-tenant GPU sharing; adds complexity; defer until GPU addon basic flow is validated
- [ ] **Shell completions in Homebrew cask** — bash/zsh/fish completions installable via `brew`; GoReleaser supports this in `homebrew_casks.completions`; nice-to-have, not blocking
- [ ] **Cosign signing for binaries** — supply chain security; requires `id-token: write` permission and Cosign setup; emerging best practice; add when users request it

### Future Consideration (v3+)

- [ ] **Chocolatey/Winget packages** — Windows package manager distribution; defer until Windows adoption data justifies maintenance cost
- [ ] **Homebrew core submission** — requires project maturity, stable tarballs, build-from-source support; kinder's module path (`sigs.k8s.io/kind`) and fork status complicate core submission; defer indefinitely
- [ ] **Snap/Flatpak for Linux** — alternative Linux distribution; Homebrew on Linux covers most developers; defer
- [ ] **AMD GPU addon (ROCm)** — different device plugin, different runtime config; parallel to NVIDIA addon but much smaller user base; defer

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| GoReleaser setup | HIGH (unlocks all distribution) | LOW (config file + workflow update) | P1 |
| Homebrew tap (`brew install`) | HIGH (biggest adoption barrier removed) | MEDIUM (second repo + PAT secret) | P1 |
| NVIDIA GPU addon | HIGH (unique capability, no competitor) | HIGH (Linux-only, host prerequisites, platform detection) | P1 |
| kinder doctor GPU checks | MEDIUM (UX improvement) | LOW (extend existing doctor pattern) | P1 |
| Reproducible builds via GoReleaser | MEDIUM (professional release quality) | LOW (ldflags already in Makefile, map to GoReleaser) | P1 |
| GPU time-slicing support | MEDIUM (differentiator for multi-pod testing) | MEDIUM (ConfigMap conditional apply) | P2 |
| Shell completions in tap | LOW (convenience) | LOW (GoReleaser feature, one config block) | P2 |
| Cosign binary signing | LOW (most OSS tools don't do this yet) | MEDIUM (workflow changes, key management) | P3 |
| Windows Chocolatey/Winget | LOW (limited kind support on Windows) | HIGH (separate manifest files, publishing process) | P3 |

**Priority key:**
- P1: Must have for v2.0 launch
- P2: Should have, add in v2.x
- P3: Nice to have, future consideration

---

## Area-Specific Details

### Area 1: GoReleaser Distribution

#### What GoReleaser replaces

The existing `hack/release/build/cross.sh` script uses `xargs -P` for parallel builds, `shasum -a 256` for checksums, and outputs bare binaries named `kinder-${GOOS}-${GOARCH}`. The `release.yml` then calls `softprops/action-gh-release@v2` to upload.

GoReleaser produces:
- Archives (`.tar.gz` for Linux/macOS, `.zip` for Windows) containing the binary
- `checksums.txt` with SHA-256 hashes for all archives
- Auto-generated changelog from git commits
- GitHub Release update (replacing the `softprops` action)
- Homebrew cask update (replacing manual formula editing)

#### Key `.goreleaser.yaml` sections

```yaml
version: 2

before:
  hooks:
    - go mod download

builds:
  - id: kinder
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
    # Preserve existing version metadata injection
    ldflags:
      - -trimpath
      - -buildid=
      - -w
      - -X sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .Commit }}
      - -X sigs.k8s.io/kind/pkg/internal/kindversion.gitCommitCount={{ .Env.COMMIT_COUNT }}
    mod_timestamp: "{{ .CommitTimestamp }}"

archives:
  - name_template: "kinder_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        formats: [zip]

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude:
      - "^chore:"
      - "^Merge"

homebrew_casks:
  - name: kinder
    description: "Batteries-included local Kubernetes clusters"
    homepage: "https://kinder.patrykgolabek.dev"
    repository:
      owner: PatrykQuantumNomad
      name: homebrew-kinder
      token: "{{ .Env.GH_TAP_TOKEN }}"
```

#### GitHub Actions workflow change

Replace existing steps in `.github/workflows/release.yml`:

```yaml
# REMOVE:
# - name: Build cross-platform binaries
#   run: hack/release/build/cross.sh
# - name: Create GitHub Release
#   uses: softprops/action-gh-release@v2

# ADD:
- uses: goreleaser/goreleaser-action@v7
  with:
    distribution: goreleaser
    version: "~> v2"
    args: release --clean
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    GH_TAP_TOKEN: ${{ secrets.GH_TAP_TOKEN }}
    COMMIT_COUNT: # see note below
```

**COMMIT_COUNT note:** The existing Makefile computes `COMMIT_COUNT` via `git describe --tags | rev | cut -d- -f2 | rev`. GoReleaser does not have a built-in template for this. Options: (1) compute it in a prior workflow step and export as `$GITHUB_ENV`; (2) drop `gitCommitCount` from ldflags in v2.0 (it's not user-visible); (3) use a `before.hooks` script. Option 2 is simplest for now — `gitCommitCount` was never exposed in `kinder version` output.

#### Confidence: HIGH
GoReleaser is the de facto standard for Go CLI distribution. The configuration maps directly from the existing Makefile. Official docs at goreleaser.com confirmed current behavior. GoReleaser v2.14.1 is the current stable version.

---

### Area 2: Homebrew Tap

#### Repository structure

The `homebrew-kinder` repository (at `github.com/PatrykQuantumNomad/homebrew-kinder`) must exist and be public. GoReleaser will manage the cask file content — no manual formula writing required.

```
homebrew-kinder/
  Casks/
    kinder.rb          # generated and updated by GoReleaser on each release
  README.md            # explains: brew tap patrykquantumnomad/kinder && brew install kinder
```

#### User-facing install commands

```bash
brew tap patrykquantumnomad/kinder
brew install kinder

# Or in one command:
brew install patrykquantumnomad/kinder/kinder
```

#### PAT token requirement

The `GITHUB_TOKEN` in the release workflow only has access to `PatrykQuantumNomad/kinder`. To push cask updates to `PatrykQuantumNomad/homebrew-kinder`, a Personal Access Token with `repo` scope on that repository is required. Steps:

1. Create PAT at github.com/settings/tokens (classic token, `repo` scope, or fine-grained with Contents:write on homebrew-kinder)
2. Add as `GH_TAP_TOKEN` secret in the `kinder` repository settings
3. Reference in `.goreleaser.yaml` and release.yml as shown above

#### Cask vs Formula distinction

GoReleaser v2.10+ introduced `homebrew_casks` as the correct approach (the old `brews`/`homebrew_formulas` section is deprecated). Casks are the right format for binary-only tools (no need to build from source). Formulae require building from source and are for Homebrew core submission. Since kinder ships pre-built binaries and uses a custom tap, `homebrew_casks` is correct.

#### macOS quarantine

macOS will quarantine unsigned binaries downloaded via Homebrew. Add a `caveats` message:

```yaml
homebrew_casks:
  - caveats: |
      If macOS blocks kinder due to Gatekeeper, run:
        sudo xattr -d com.apple.quarantine /usr/local/bin/kinder
```

This is the standard pattern used by kubectl, kind, k3d, and most unsigned OSS tools.

#### Confidence: HIGH
Official GoReleaser docs confirmed. Pattern verified against goreleaser's own homebrew-tap. PAT requirement confirmed in GoReleaser discussions. Homebrew cask vs formula distinction confirmed in Homebrew docs.

---

### Area 3: NVIDIA GPU Addon

#### Platform constraint (critical)

**NVIDIA GPU support is Linux-only.** This is a hard technical constraint:

- NVIDIA GPU Operator does NOT support kind clusters (confirmed: kind is absent from NVIDIA's official supported platforms list; Ubuntu, RHEL, OpenShift only)
- macOS has no NVIDIA GPUs (Apple Silicon uses MPS, not CUDA)
- Windows requires WSL2 for GPU passthrough with Docker, which does not expose GPUs to kind's containerd runtime

**Recommended approach:** Use the standalone NVIDIA k8s-device-plugin DaemonSet (NOT the GPU Operator). This is the approach that community guides confirm works with kind.

#### Host prerequisites (outside kinder's control)

The following must exist on the Linux host BEFORE `kinder create cluster --addons nvidia`:

1. NVIDIA GPU driver installed (e.g., `nvidia-driver-535`)
2. NVIDIA Container Toolkit installed (`nvidia-container-toolkit` package)
3. Docker/containerd configured with NVIDIA runtime: `nvidia-ctk runtime configure --runtime=containerd --set-as-default && systemctl restart containerd`
4. `accept-nvidia-visible-devices-as-volume-mounts = true` in `/etc/nvidia-container-runtime/config.toml`

Kinder cannot automate these. The `kinder doctor` command is the right place to check for them.

#### Kind cluster config requirements

The GPU addon must apply a `containerdConfigPatches` to inject the NVIDIA runtime into the kind node's containerd config. This is the same mechanism already used for the local registry addon:

```toml
# ContainerdConfigPatch applied during kinder create cluster (before Provision)
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes."nvidia"]
  runtime_type = "io.containerd.runc.v2"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes."nvidia".options]
  BinaryName = "/usr/bin/nvidia-container-runtime"
```

Additionally, `extraMounts` on each worker node to expose the NVIDIA device:

```yaml
extraMounts:
  - hostPath: /dev/null
    containerPath: /var/run/nvidia-container-devices/all
```

**Implementation pattern:** The GPU addon's `Execute()` method is structurally different from other addons — the `containerdConfigPatches` and `extraMounts` must be set during cluster config construction (in `create.go` before `p.Provision()`), not after. This is the same constraint documented for the local registry addon's `ContainerdConfigPatches` field. The GPU addon must hook into cluster config setup, not just post-provision manifest apply.

#### Post-provision manifest application

After cluster provisioning, the GPU addon applies the NVIDIA k8s-device-plugin DaemonSet:

```yaml
# Embedded via go:embed: manifests/nvidia-device-plugin.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: nvidia-device-plugin-daemonset
  namespace: kube-system
spec:
  selector:
    matchLabels:
      name: nvidia-device-plugin-ds
  template:
    spec:
      tolerations:
        - key: nvidia.com/gpu
          operator: Exists
          effect: NoSchedule
      containers:
        - name: nvidia-device-plugin-ctr
          image: nvcr.io/nvidia/k8s/cuda-sample:vectoradd-cuda10.2
          # pinned version: k8s-device-plugin v0.17.1
```

#### Platform detection in Execute()

```go
func (a *action) Execute(ctx *actions.ActionContext) error {
    if runtime.GOOS != "linux" {
        ctx.Logger.V(0).Infof("WARNING: NVIDIA GPU addon requires Linux; skipping on %s", runtime.GOOS)
        return nil
    }
    // proceed with device plugin apply
}
```

Note: `runtime.GOOS` checks the HOST OS (where kinder runs), which is correct — the kind nodes inherit the host's container runtime.

#### kinder doctor GPU checks

Add GPU-specific checks to `kinder doctor` (only evaluated when GPU addon is enabled or when `--check gpu` is explicitly requested):

| Check | Command | Pass Condition |
|-------|---------|----------------|
| NVIDIA GPU present | `nvidia-smi` exits 0 | GPU hardware detected |
| Container toolkit installed | `nvidia-ctk --version` exits 0 | Package installed |
| Containerd runtime configured | `nvidia-ctk runtime check` | nvidia runtime registered |
| Volume mounts flag set | Read `/etc/nvidia-container-runtime/config.toml` | `accept-nvidia-visible-devices-as-volume-mounts = true` |

#### Existing addon pattern (reference)

The local registry addon (`installlocalregistry`) is the closest analogy because it also requires:
- `ContainerdConfigPatches` applied before `Provision()` (registry mirrors config)
- Post-provision manifest apply (registry DaemonSet)
- `kinder doctor` check integration

The GPU addon follows the same two-phase pattern.

#### Addon config field

Add to `pkg/apis/config/v1alpha4/types.go` (same pattern as `LocalRegistry`, `CertManager`):

```go
type Addons struct {
    // existing fields...
    // Nvidia enables the NVIDIA GPU addon (Linux only).
    // +optional
    Nvidia *bool `json:"nvidia,omitempty"`
}
```

And to `pkg/internal/apis/config/types.go`:

```go
type Addons struct {
    // existing fields...
    Nvidia bool
}
```

#### Confidence: MEDIUM (HIGH for device plugin approach, MEDIUM for containerdConfigPatches specifics)

The device plugin approach (not GPU Operator) is confirmed by multiple community guides and by the fact that kind is not on NVIDIA's official platform support list. The specific TOML patch content for containerd is synthesized from community gists, RKE2 docs, and the NVIDIA container toolkit configure command output — not from an official kind+GPU guide (no official guide exists). The `extraMounts` pattern is confirmed from the kind+CAPI+GPU gist. The `ContainerdConfigPatches` mechanism within kinder is HIGH confidence (directly used in production for local registry).

---

## Competitor Feature Analysis

| Feature | kind (upstream) | minikube | k3d | kinder (v1.5) | kinder (v2.0 target) |
|---------|----------------|----------|-----|---------------|----------------------|
| Pre-built binaries | Yes (GitHub Releases) | Yes | Yes | No (source only) | Yes (GoReleaser) |
| Homebrew install | `brew install kind` (core) | `brew install minikube` (core) | `brew install k3d` (core) | None | `brew install patrykquantumnomad/kinder/kinder` (tap) |
| Checksums | Yes | Yes | Yes | Yes (sha256sum) | Yes (GoReleaser checksums.txt) |
| Changelog | GitHub auto | GitHub auto | GitHub auto | GitHub auto | GoReleaser changelog with commit filtering |
| NVIDIA GPU addon | No | Yes (via driver abstraction) | No | No | Yes (device plugin, Linux only) |
| GPU time-slicing | No | Partial | No | No | Yes (v2.x) |

**Homebrew core vs tap:** kind, minikube, and k3d are all in `homebrew-core` because they're mature, stable, and build from source. A custom tap is the correct path for kinder at this stage — it gets to `brew install` without the Homebrew core review process. The tap approach also allows faster iteration (no Homebrew maintainer review needed for updates).

---

## Sources

- GoReleaser GitHub Actions integration: https://goreleaser.com/ci/actions/ (HIGH confidence)
- GoReleaser Go build configuration: https://goreleaser.com/customization/builds/go/ (HIGH confidence)
- GoReleaser homebrew_casks (v2.10+): https://goreleaser.com/customization/homebrew_casks/ (HIGH confidence)
- GoReleaser v2.10 announcement: https://goreleaser.com/blog/goreleaser-v2.10/ (HIGH confidence)
- GoReleaser current version v2.14.1: https://goreleaser.com/install/ (HIGH confidence)
- Homebrew tap documentation: https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap (HIGH confidence)
- NVIDIA GPU Operator supported platforms: https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/platform-support.html (HIGH confidence) — kind is NOT listed
- NVIDIA GPU Operator getting started: https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/getting-started.html (HIGH confidence)
- NVIDIA k8s-device-plugin repository: https://github.com/NVIDIA/k8s-device-plugin (HIGH confidence)
- NVIDIA Container Toolkit install guide: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html (HIGH confidence)
- Kind + CAPI + GPU gist (containerdConfigPatches approach): https://gist.github.com/mproffitt/a828c074b09bbf65dae184790baacb41 (MEDIUM confidence — community source)
- GPU on Kubernetes practical guide: https://www.jimangel.io/posts/nvidia-rtx-gpu-kubernetes-setup/ (MEDIUM confidence — community verified)
- GoReleaser homebrew token discussion: https://github.com/orgs/goreleaser/discussions/4926 (HIGH confidence — GH_TAP_TOKEN pattern confirmed)
- Kinder codebase direct analysis: `hack/release/build/cross.sh`, `.github/workflows/release.yml`, `Makefile`, `go.mod`, `pkg/cluster/internal/create/actions/installlocalregistry/` (HIGH confidence — primary source)

---

*Feature research for: kinder v2.0 — distribution pipeline and NVIDIA GPU addon*
*Researched: 2026-03-04*
