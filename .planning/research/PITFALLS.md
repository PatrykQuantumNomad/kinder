# Pitfalls Research

**Domain:** Go CLI tool distribution pipeline — GoReleaser + GitHub Releases + Homebrew tap + NVIDIA GPU addon for a kind fork
**Researched:** 2026-03-04
**Confidence:** HIGH (GoReleaser/Homebrew pitfalls sourced from official docs and verified issue tracker; GPU pitfalls sourced from NVIDIA docs, kind community, and official Kubernetes device plugin docs)

---

## Critical Pitfalls

### Pitfall 1: GoReleaser gomod.proxy Broken by Fork's Module Path

**What goes wrong:**
Kinder's `go.mod` declares `module sigs.k8s.io/kind` — the upstream module path, not `github.com/PatrykQuantumNomad/kinder`. GoReleaser's `gomod.proxy` feature works by fetching `yourmodule@v{tag}` from the Go module proxy (`proxy.golang.org`). If `gomod.proxy: true` is set in `.goreleaser.yaml`, GoReleaser will attempt to resolve `sigs.k8s.io/kind@vX.Y.Z` from the proxy — which returns upstream kind, not kinder. The resulting binary will be built from upstream kind source, not the fork. The binary is silently wrong: it compiles and passes checksums, but is the wrong program.

Even if `gomod.proxy` is left at its default (`false`), the mismatch between module path and repository URL confuses `goreleaser check` and the git validation pipe. GoReleaser reads the git remote URL and compares it against the module path to detect configuration drift — a mismatch triggers a warning that can fail the pipeline with non-obvious error messages.

**Why it happens:**
The fork kept the upstream module path for compatibility (all import paths work, no 28K LOC churn). GoReleaser documentation assumes the module path matches the GitHub repository URL. Most Go projects satisfy this assumption; forks that preserve the upstream module path do not.

**How to avoid:**
- Set `gomod.proxy: false` explicitly in `.goreleaser.yaml` (this is the default but must be declared to prevent accidental activation in CI templates)
- Disable module path validation: set `gomod.proxy` to `false` and add `skip: validate` only for the git-remote check if GoReleaser raises a false-positive; do NOT skip `validate` globally
- Use `builds[*].env` to set `GONOSUMCHECK=sigs.k8s.io/kind` so the Go toolchain doesn't try to verify the module against the sum database under the wrong identity
- Test with `goreleaser build --snapshot --clean` locally before any CI run; this catches module path issues without touching GitHub

**Warning signs:**
- `goreleaser check` warns about "module path does not match VCS remote"
- A locally built `goreleaser` binary reports `sigs.k8s.io/kind v1.x` in `version -m` output instead of the kinder version
- `go version -m ./bin/kinder` shows a `build` path resolving to `sigs.k8s.io/kind` at a hash that exists in the upstream repo

**Phase to address:** Phase 1 (GoReleaser setup) — verify `gomod.proxy: false` is explicit, run `goreleaser build --snapshot --clean` before enabling any Homebrew or release steps

---

### Pitfall 2: GoReleaser Ignores the Existing cross.sh Script — ldflags Version Variables Are Lost

**What goes wrong:**
Kinder already has a working cross-compile script at `hack/release/build/cross.sh` and a `release.yml` workflow. The Makefile builds with custom ldflags: `-X=$(KIND_VERSION_PKG).gitCommit=$(COMMIT) -X=$(KIND_VERSION_PKG).gitCommitCount=$(COMMIT_COUNT)`. GoReleaser, when configured naively, applies its own default ldflags (`-s -w`) and does not automatically replicate the Makefile's `-X` flags. The result: released binaries have no embedded git commit metadata. `kinder version` returns empty version info. The `kindversion` package fields (`gitCommit`, `gitCommitCount`) are blank strings at runtime.

Separately, the existing `cross.sh` script produces binaries named `kinder-${GOOS}-${GOARCH}` (e.g., `kinder-linux-amd64`). GoReleaser by default names archives as `kinder_v1.0.0_linux_amd64.tar.gz`. If the Homebrew formula references the archive name and the binary inside, a name mismatch breaks `brew install`.

**Why it happens:**
Two independent build systems (Makefile + GoReleaser) define the same set of output binaries differently. Developers add GoReleaser assuming it will read the Makefile targets. GoReleaser does not read Makefiles — it reads `.goreleaser.yaml` only. The ldflags template syntax differs between make and GoReleaser (`$(COMMIT)` vs `{{ .FullCommit }}`).

**How to avoid:**
- In `.goreleaser.yaml`, explicitly replicate all Makefile ldflags using GoReleaser template variables:
  ```yaml
  builds:
    - binary: kinder
      ldflags:
        - -trimpath
        - -buildid=
        - -w
        - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .FullCommit }}
        - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommitCount={{ .Env.COMMIT_COUNT }}
  ```
  Note: `COMMIT_COUNT` is a `git describe`-derived value; either compute it in the CI workflow and expose it as an env var, or replicate the logic in GoReleaser using a `before.hooks` step
- Retire `cross.sh` for release builds once GoReleaser is validated; do not run both in parallel — parallel builds produce duplicate artifacts that overwrite each other on the GitHub Release
- Run `./bin/kinder version` after `goreleaser build --snapshot` and verify the git commit and count fields are populated

**Warning signs:**
- `kinder version` outputs empty `gitCommit` after a GoReleaser build
- Two workflows both upload to the same GitHub Release tag and one fails with "asset already exists" (422 error from GitHub API)
- Checksums in `checksums.txt` cover different file lists between the cross.sh release and the GoReleaser release

**Phase to address:** Phase 1 (GoReleaser setup) — audit the Makefile ldflags before writing `.goreleaser.yaml`; verify version output before enabling the Homebrew tap

---

### Pitfall 3: Homebrew Tap PAT Scope — GITHUB_TOKEN Cannot Push to a Separate Repository

**What goes wrong:**
The GitHub Actions default `GITHUB_TOKEN` has `contents: write` permission only for the repository that contains the workflow (the kinder repo). The Homebrew tap is a separate repository (e.g., `github.com/PatrykQuantumNomad/homebrew-kinder`). When GoReleaser attempts to commit the generated cask file to the tap repository, it uses `GITHUB_TOKEN` and gets a 403 — the token has no permissions on the tap repo. The release workflow succeeds, binaries are uploaded, but the Homebrew tap is silently not updated.

The silent failure mode is documented in GoReleaser: "if an error happens, it'll log it to the release output, but will not fail the pipeline." Users run `brew install patrykquantumnomad/kinder/kinder` after a release and get the previous version.

**Why it happens:**
Developers follow the GoReleaser GitHub Actions example which uses `${{ secrets.GITHUB_TOKEN }}`. This token works for the release itself (binary upload to GitHub Releases) but not for cross-repository writes. The documentation note about the separate token is easy to overlook because the release succeeds without error.

**How to avoid:**
- Create a GitHub Personal Access Token (PAT) with `repo` scope for the tap repository; store it as `secrets.HOMEBREW_TAP_TOKEN` in the kinder repository settings
- In `.goreleaser.yaml` homebrew_casks section, set `token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"`
- In the workflow, pass the PAT: `env: HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}`
- After a test release, verify the tap repository has a new commit: `gh api repos/PatrykQuantumNomad/homebrew-kinder/commits --jq '.[0].commit.message'`
- Do NOT use the GitHub App token approach unless the GitHub App is installed on both the kinder and tap repositories

**Warning signs:**
- Release workflow completes with green status but the tap repo has no new commits
- GoReleaser log contains "level=warn msg=brew: could not push cask" — this warning does not fail the pipeline
- `brew upgrade kinder` after a release still shows the old version

**Phase to address:** Phase 2 (Homebrew tap) — create the PAT and add the secret BEFORE writing the `.goreleaser.yaml` tap configuration; test with a pre-release tag to catch the silent failure

---

### Pitfall 4: Homebrew Formula vs. Cask Confusion — Using Deprecated brews Instead of homebrew_casks

**What goes wrong:**
GoReleaser has two Homebrew publishing mechanisms: `brews` (deprecated since v2.10, removed in v3) and `homebrew_casks` (current). Tutorials and Stack Overflow answers from 2023-2024 use `brews`. A developer copying an example into `.goreleaser.yaml` will use the deprecated `brews` section. The release works with current GoReleaser v2 versions but produces a Homebrew Formula (not a Cask). Homebrew Formulas for pre-compiled binaries are not accepted in `homebrew-core` and are semantically incorrect for binary-only distributions. More critically, when GoReleaser v3 is released (date TBD), the `brews` section will be silently ignored or will error, breaking the pipeline.

The second confusion: `brew install patrykquantumnomad/kinder/kinder` works for both Formulas and Casks, but `brew install --cask kinder` only works for Casks. Users who install via `--cask` with a Formula-based tap get an error that looks like kinder does not exist.

**Why it happens:**
The migration from `brews` to `homebrew_casks` happened in GoReleaser v2.10 (February 2025). Most tutorials predate this. The old `brews` section still works in GoReleaser v2.x, so there is no immediate error — just a deprecation warning that developers dismiss.

**How to avoid:**
- Use `homebrew_casks:` in `.goreleaser.yaml`, NOT `brews:`
- The minimal `homebrew_casks` configuration for kinder:
  ```yaml
  homebrew_casks:
    - name: kinder
      repository:
        owner: PatrykQuantumNomad
        name: homebrew-kinder
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
      description: "Batteries-included local Kubernetes clusters (kind fork)"
      homepage: "https://github.com/PatrykQuantumNomad/kinder"
  ```
- Run `goreleaser check` and verify no deprecation warnings about `brews`
- Document the install method as `brew install patrykquantumnomad/kinder/kinder` (without `--cask`), which works for both; or document `brew tap patrykquantumnomad/kinder && brew install kinder`

**Warning signs:**
- `.goreleaser.yaml` contains a `brews:` key — rename to `homebrew_casks:`
- `goreleaser check` output contains "brews is deprecated, use homebrew_casks instead"
- `brew install --cask kinder` fails with "No Cask with this name exists" even though the tap is configured

**Phase to address:** Phase 2 (Homebrew tap) — start with `homebrew_casks` from day one; do not port from a `brews` example

---

### Pitfall 5: GitHub Release Re-runs Fail With "Asset Already Exists" (422)

**What goes wrong:**
When a GoReleaser release workflow fails partway through (e.g., after uploading binaries but before updating the Homebrew tap), re-running the workflow on the same tag produces a 422 error from the GitHub API: "asset already exists". GitHub does not allow overwriting release assets. GoReleaser has no automatic recovery for partially uploaded releases. The developer must manually delete all assets from the GitHub Release page, then re-run. If they re-run without deleting, every upload attempt fails, but the Homebrew cask update (which runs after uploads) is skipped — leaving an inconsistent state where the release has partial assets but no Homebrew update.

A related failure: releasing a `v1.0.0-rc1` tag by mistake and then releasing `v1.0.0` with a corrected changelog — GoReleaser treats these as separate releases, which is correct. But if the developer deletes `v1.0.0-rc1` from GitHub without deleting the git tag locally and on the remote, the next `goreleaser release` run on `v1.0.0` may fail the git dirty-state check because the tag exists but has no associated release.

**Why it happens:**
CI pipelines fail for non-release reasons (network timeout, Homebrew push failure, etc.). Re-triggering the workflow re-runs `goreleaser release` with the same tag. The `--clean` flag cleans the `dist/` directory but does not clean GitHub Release assets.

**How to avoid:**
- Add `release: extra_files: []` and `release: mode: replace` in `.goreleaser.yaml` to allow asset replacement on re-run (GoReleaser supports `mode: replace` since v2 to overwrite existing assets)
- Alternatively: use a draft release workflow — GoReleaser creates a draft, CI validates, then promotes to published; a failed draft can be deleted and re-run without the "asset exists" error
- Add `make goreleaser-dry-run` target that runs `goreleaser release --snapshot --skip=publish --clean` for local testing before tagging
- Document the recovery procedure: "if the release pipeline fails, delete all assets from the GitHub Release before re-running"

**Warning signs:**
- CI logs show "422 Validation Failed: asset already_exists" from the GitHub API
- `dist/` directory exists from a previous run when `goreleaser release` starts (should always run with `--clean`)
- GitHub Release page shows some platform binaries but not others (partial upload)

**Phase to address:** Phase 1 (GoReleaser setup) — configure `release: mode: replace` or the draft release workflow during initial setup; document recovery in the project README

---

### Pitfall 6: fetch-depth: 0 Missing — GoReleaser Cannot Compute Changelog or Tags

**What goes wrong:**
GoReleaser requires the full git history to compute the changelog (`git log v0.9.9..v1.0.0`) and to resolve the previous tag for the `gitCommitCount` ldflag (`git describe --tags`). The kinder release workflow already uses `fetch-depth: 0` in `actions/checkout@v4`. If this is ever changed (e.g., a developer "optimizes" the workflow by setting `fetch-depth: 1` to speed up checkout), GoReleaser fails with a cryptic error: `git: unknown flag --tags` or the changelog is empty. The `gitCommitCount` ldflag also computes as `0` because `git describe` cannot find any tag in a shallow clone.

This is an existing risk in the current `release.yml` — the fetch-depth is correct today but fragile against future edits.

**Why it happens:**
`fetch-depth: 1` is the recommended optimization for most CI pipelines. A developer unfamiliar with GoReleaser's requirements will apply it as part of a "CI speed improvement" task. The GoReleaser error is not obvious: it produces an empty changelog (not a build failure) because GoReleaser treats a missing previous tag as "initial release" and generates an empty diff.

**How to avoid:**
- Add a comment in `release.yml` directly above the `fetch-depth: 0` line:
  ```yaml
  # REQUIRED for GoReleaser: full history needed for changelog and git describe
  # DO NOT change to fetch-depth: 1 — this silently breaks version embedding
  fetch-depth: 0
  ```
- Add a `goreleaser check` step that runs after checkout and validates the git configuration
- In the GoReleaser changelog configuration, set `use: github` to use the GitHub API for changelog generation as a fallback if git history is shallow — this provides a safety net

**Warning signs:**
- Release binary has `gitCommitCount: 0` even when the repo has many commits since the last tag
- GitHub Release changelog section is empty or only shows "Initial commit"
- `git describe --tags` in the CI environment returns an error about no tags found

**Phase to address:** Phase 1 (GoReleaser setup) — add the protective comment during initial setup; validate with `goreleaser build --snapshot --clean` in CI

---

### Pitfall 7: NVIDIA GPU Passthrough — Host Driver Must Match Container Runtime Configuration

**What goes wrong:**
The kinder GPU addon configures kind nodes to pass through host GPUs via Docker's `--gpus=all` flag. This requires three independent components to be correctly configured on the host at the time `kinder create cluster` is run:
1. NVIDIA driver installed on the host (`nvidia-smi` works)
2. NVIDIA Container Toolkit installed (`nvidia-ctk` binary present, `/etc/docker/daemon.json` configured with `"runtime": "nvidia"`)
3. containerd configured with the NVIDIA runtime class (required by the GPU operator inside the cluster)

Kinder can detect (2) and (3) but cannot install them. If `kinder create cluster --addons gpu` is run on a host where only the driver is installed but the Container Toolkit is not configured, Docker creates the kind node containers without GPU access. The containers start successfully. The GPU operator is then installed inside the cluster and the device plugin DaemonSet starts — but no GPU devices are visible inside the node containers. The device plugin reports 0 GPUs. The failure mode looks like a Kubernetes configuration problem, not a host configuration problem, because the cluster itself is healthy and the device plugin is running.

**Why it happens:**
The NVIDIA Container Toolkit's `nvidia-ctk runtime configure --runtime=docker` step modifies `/etc/docker/daemon.json` and requires a Docker daemon restart. Users skip this step, expecting Docker GPU support to be automatic (it was not automatic before nvidia-container-toolkit v1.13). The kinder addon cannot distinguish between "no GPU on this machine" and "GPU present but Container Toolkit not configured."

**How to avoid:**
- In the GPU addon's `Execute()` method, add a pre-flight check before creating nodes:
  ```go
  // Check that the host docker daemon has the nvidia runtime configured
  out, err := exec.Output(exec.Command("docker", "info", "--format", "{{json .Runtimes}}"))
  // parse JSON and verify "nvidia" key exists
  ```
- If the check fails, return a user-actionable error: "NVIDIA Container Toolkit is not configured as a Docker runtime. Run: nvidia-ctk runtime configure --runtime=docker && systemctl restart docker"
- Distinguish between the pre-flight check (addon refuses to proceed) and the GPU operator installation (best-effort, can be rerun)
- Document the host requirements prominently in the GPU addon documentation: driver version, Container Toolkit version, Docker daemon restart requirement

**Warning signs:**
- `kubectl describe node | grep nvidia.com/gpu` shows `nvidia.com/gpu: 0` after cluster creation
- NVIDIA Device Plugin pod logs: "No devices found" or "nvml: Driver/library version mismatch"
- `docker run --rm --gpus all nvidia/cuda:12.0-base nvidia-smi` fails on the host while `kinder create cluster` succeeds — these are independent

**Phase to address:** Phase 3 (GPU addon implementation) — implement the pre-flight check before writing the addon's kubectl manifest application logic; the pre-flight check is more important than the operator configuration

---

### Pitfall 8: GPU Operator Driver Mode — Installing Drivers Inside kind Nodes Fails

**What goes wrong:**
The NVIDIA GPU Operator can operate in two modes: "driver mode" (installs NVIDIA drivers inside cluster nodes) and "driverless mode" (uses host drivers). Kind nodes are Docker containers. Installing NVIDIA drivers inside a Docker container requires a privileged container with access to the host kernel module interface — which kind's standard node image does not provide. The driver installer pod will start, attempt to install the driver, and hang indefinitely waiting for kernel module access that it cannot have.

The correct mode for kind is "driverless" (`driver.enabled=false`), which uses the drivers already on the host. Developers copy GPU operator installation examples from production Kubernetes documentation (which default to driver mode) without realizing kind requires a different operator configuration.

**Why it happens:**
The NVIDIA GPU Operator's default Helm chart configuration enables the driver installer (`driver.enabled=true`). Every NVIDIA tutorial for production clusters uses this default. The kind-specific configuration (`driver.enabled=false`) is only documented in community guides, not the official GPU Operator documentation. The failure mode — the driver installer pod hanging — is not obviously caused by running in a Docker container.

**How to avoid:**
- Hardcode `driver.enabled=false` in the GPU addon's Helm values; do not expose this as a user-configurable option (it should never be `true` for kind-based clusters)
- The correct Helm invocation for the GPU operator in kinder:
  ```bash
  helm upgrade --install gpu-operator nvidia/gpu-operator \
    --namespace gpu-operator --create-namespace \
    --set driver.enabled=false \
    --set toolkit.enabled=true \
    --set devicePlugin.enabled=true \
    --set migManager.enabled=false
  ```
- After installation, add a validation step: wait for `nvidia.com/gpu` to appear as an allocatable resource on the node before marking the addon as complete
- Document explicitly: "kinder's GPU addon uses driverless mode — NVIDIA drivers must be installed on the host before creating the cluster"

**Warning signs:**
- A pod named `nvidia-driver-daemonset-*` is in `Init:OOMKilled` or `Init:CrashLoopBackOff` after cluster creation
- GPU operator namespace has a pod stuck in `Pending` with events about "operation not permitted" or "module is already loaded"
- `kubectl -n gpu-operator describe pod nvidia-driver-daemonset-*` shows failed `insmod` or `modprobe` commands

**Phase to address:** Phase 3 (GPU addon implementation) — set `driver.enabled=false` in the embedded Helm values from the start; add the validation wait loop before declaring success

---

### Pitfall 9: cgroup v2 and GPU Device Access — NVIDIA Runtime Needs Explicit Configuration

**What goes wrong:**
Modern Linux distributions (Ubuntu 22.04+, Fedora 31+, Debian 11+) use cgroup v2 by default. The NVIDIA Container Toolkit before v1.8.0 did not support cgroup v2, causing containers to fail to access GPU devices even when drivers and the runtime are correctly installed. Post-v1.8.0, cgroup v2 is supported, but it requires an additional configuration step: the NVIDIA container runtime config (`/etc/nvidia-container-runtime/config.toml` or `/etc/cdi/`) must have the device visibility settings aligned with how kind nodes are created.

Kind nodes use `--privileged` Docker containers (for kubeadm, networking setup, etc.). Privileged containers have different cgroup device rules than non-privileged containers. If `accept-nvidia-visible-devices-envvar-when-unprivileged` is set to `false` (a security best practice per NVIDIA's 2025 security bulletin), GPU devices are not visible inside privileged containers that request them via the `NVIDIA_VISIBLE_DEVICES` environment variable alone — they must be requested via CDI (Container Device Interface) or volume mounts.

**Why it happens:**
The NVIDIA Container Toolkit security defaults changed in 2024-2025 in response to CVE findings. Configurations that worked in 2023 (setting `NVIDIA_VISIBLE_DEVICES=all` on a container) may not work on updated systems. Kind's node container creation does not set CDI device annotations — it uses the older `--gpus=all` Docker flag, which relies on the NVIDIA Docker runtime, not CDI.

**How to avoid:**
- In the GPU addon pre-flight check, verify the NVIDIA Container Toolkit version: require v1.13+ (CDI support stable)
- Check that `accept-nvidia-visible-devices-as-volume-mounts = true` in `/etc/nvidia-container-runtime/config.toml` when using the volume-mounts device list strategy; alternatively verify that the host is using CDI mode
- When creating kind cluster nodes for GPU support, add `extraMounts` to expose the NVIDIA device files explicitly:
  ```yaml
  nodes:
    - role: control-plane
      extraMounts:
        - hostPath: /dev/null
          containerPath: /var/run/nvidia-container-devices/all
  ```
  This is required when the runtime uses the volume-mounts strategy instead of the envvar strategy
- Add a CI test that runs on a GPU host with the newest available Ubuntu LTS and verifies GPU access end-to-end

**Warning signs:**
- `docker run --gpus all nvidia/cuda:12.0-base nvidia-smi` works on the host, but `kubectl exec -it test-gpu-pod -- nvidia-smi` inside the cluster fails
- NVIDIA container toolkit log (`/var/log/nvidia-container-toolkit.log`) shows "no devices found for container" despite `NVIDIA_VISIBLE_DEVICES=all` being set
- NVIDIA toolkit config has `accept-nvidia-visible-devices-envvar-when-unprivileged = false` and the kind node container is not using CDI annotations

**Phase to address:** Phase 3 (GPU addon implementation) — document cgroup v2 + security config requirements; implement the pre-flight check to detect incompatible toolkit configurations before creating the cluster

---

### Pitfall 10: Binary Name "kinder" Has No Homebrew Core Conflict But Tap Name Matters

**What goes wrong:**
`kinder` is not a formula in `homebrew-core`, so there is no immediate conflict. However, the Homebrew tap name and the cask name must follow specific conventions. If the tap repository is named `homebrew-kinder`, the tap is referenced as `patrykquantumnomad/kinder`. If the cask inside that tap is also named `kinder`, then installation is `brew install patrykquantumnomad/kinder/kinder`. This double-naming is awkward in documentation and easy to mistype.

More critically: GoReleaser defaults the cask `name` to the project name (from `project_name` in `.goreleaser.yaml`). If `project_name: kinder` is set, the cask file is `kinder.rb`. If `project_name` is left unset, GoReleaser infers the project name from the git repository name. If the repository is `kinder` (not the same as the module path `sigs.k8s.io/kind`), GoReleaser uses `kinder` as the project name — which is correct, but this inference must be verified, because getting it wrong produces a cask named after the wrong string.

**Why it happens:**
The go.mod module path is `sigs.k8s.io/kind`, the git repository is `kinder`, and GoReleaser must choose one as the canonical project name. GoReleaser uses the git repository name, not the module path — which is the right choice here, but the developer may not realize GoReleaser ignores go.mod for naming.

**How to avoid:**
- Set `project_name: kinder` explicitly in `.goreleaser.yaml` — never rely on inference when the module path and repository name differ
- Name the tap repository `homebrew-kinder` (convention: prefix `homebrew-`) and document the install command as `brew tap patrykquantumnomad/kinder && brew install kinder` (after tapping, the short name works)
- Verify the generated cask file has the correct binary name in the `binary` field and it matches the actual executable name inside the archive
- Run `brew install --cask patrykquantumnomad/kinder/kinder` on macOS after the first release to confirm end-to-end

**Warning signs:**
- The cask `.rb` file generated by GoReleaser has `cask "kind"` instead of `cask "kinder"` (happens if `project_name` is inferred from the module path)
- `brew install kinder` works but installs from the wrong cask (a future conflict if another `kinder` cask appears)
- The cask file's `binary` field points to a file that does not exist in the archive (name mismatch between archive binary and `binary` directive)

**Phase to address:** Phase 2 (Homebrew tap) — set `project_name` explicitly before generating any release; validate the generated `.rb` cask file before publishing

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Copy `.goreleaser.yaml` from an upstream kind project | Faster setup | Module path mismatch silently builds wrong binary; ldflags don't match kinder's version package | Never — start from scratch, reference kind as documentation only |
| Use `brews:` instead of `homebrew_casks:` | Works in GoReleaser v2 today | Breaks in v3; produces Formula not Cask; incompatible with `brew install --cask` | Never for new projects — use `homebrew_casks:` |
| Expose `driver.enabled` as a user config option in GPU addon | Flexible | Users who set `driver.enabled=true` get hanging pods with no clear error | Never for kind-based clusters — always set `driver.enabled=false` |
| Skip pre-flight GPU host check | Simpler addon implementation | Users get a healthy cluster with 0 GPUs and no actionable error | Never — the pre-flight check is the most important part of the GPU addon |
| Reuse `GITHUB_TOKEN` for Homebrew tap pushes | No extra secret management | Silent tap update failures after every release | Never — Homebrew tap requires a dedicated PAT |
| Run GoReleaser in parallel with the existing `cross.sh` script | Transition period safety | Duplicate release assets cause 422 errors on GitHub; checksums mismatch | Never — retire cross.sh before enabling GoReleaser release mode |

---

## Integration Gotchas

Common mistakes when connecting the new features to the existing system.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| GoReleaser + go.mod module path | Setting `gomod.proxy: true` — fetches wrong module | Set `gomod.proxy: false` explicitly; skip module path validation |
| GoReleaser + Makefile ldflags | Omitting `-X` flags for gitCommit/gitCommitCount | Replicate all `-X` flags in `builds.ldflags` using GoReleaser templates |
| GoReleaser + cross.sh | Running both in the same release workflow | Retire cross.sh when GoReleaser takes over; never upload the same binary from two workflows |
| Homebrew tap + GITHUB_TOKEN | Using the default token for cross-repo push | Create a PAT with `repo` scope; set as `HOMEBREW_TAP_TOKEN` secret |
| Homebrew `brews` vs `homebrew_casks` | Using deprecated `brews:` from old tutorials | Use `homebrew_casks:` — the only non-deprecated option since GoReleaser v2.10 |
| GPU addon + host toolkit | Assuming Docker's `--gpus=all` implies the toolkit is configured | Run pre-flight check: `docker info --format {{json .Runtimes}}` must contain `nvidia` |
| GPU operator + driver mode | Copying production GPU operator Helm values | Set `driver.enabled=false` hardcoded for kind clusters; never expose to users |
| GPU addon + cgroup v2 | Assuming envvar-based device visibility works everywhere | Verify `accept-nvidia-visible-devices-as-volume-mounts` or use CDI; add `extraMounts` for device files |
| Homebrew cask + binary name | Relying on GoReleaser to infer project name from module path | Set `project_name: kinder` explicitly; verify cask `.rb` content after first release |
| GitHub Release + re-run | Re-running workflow without deleting existing assets | Set `release.mode: replace` in GoReleaser config or use draft releases |

---

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Unlimited parallel GoReleaser platform builds | One build fails, partial assets uploaded, checksums incomplete | GoReleaser handles this internally; do not parallelize builds with external scripts | Immediately — do not parallelize externally |
| GPU addon sequential kubectl apply for all operator components | Cluster creation takes 5+ minutes waiting for GPU operator readiness | Apply the operator asynchronously; check readiness in a separate verify step or skip waiting in fast mode | With every cluster creation on GPU-enabled machines |
| Homebrew cask URL using raw GitHub Release download | Homebrew caches the URL; if assets are replaced (mode: replace), the SHA256 mismatch breaks installs | Never replace released assets that Homebrew has cached; use a new version tag instead | After any `mode: replace` operation on a version Homebrew has already cached |
| GPU pre-flight check making Docker API calls at addon init time | Slows down non-GPU cluster creation by running docker info unnecessarily | Run GPU checks only when the GPU addon is enabled in config | When the GPU addon is always checked even for non-GPU clusters |

---

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Signing GoReleaser releases without verifying the signing key is correct | Malicious binary shipped with valid signature | Use a consistent GPG key stored as a GitHub Secret; verify the public key is published and matches before the first release |
| Exposing `HOMEBREW_TAP_TOKEN` (full `repo` scope PAT) with no expiration | Long-lived token can be used to push malicious code to tap | Set the PAT to expire in 90 days; rotate before expiry; consider using a GitHub App with limited scope instead |
| GPU addon running kubectl with cluster-admin and no timeout | A hung kubectl apply blocks the creation goroutine indefinitely | Always pass `--timeout 120s` to kubectl apply calls in addon actions |
| Publishing checksums without signing | Checksums file can be replaced without detection | Enable GoReleaser's cosign integration or GPG signing of `checksums.txt`; publish the public key in README |
| NVIDIA Container Toolkit with `accept-nvidia-visible-devices-envvar-when-unprivileged = true` | All pods on GPU nodes can claim GPUs without the device plugin (security bypass, CVE-2024-0132) | Require `accept-nvidia-visible-devices-envvar-when-unprivileged = false` in the pre-flight check; document the security requirement |

---

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **GoReleaser binary built:** Binary exists in `dist/` — but also `./dist/kinder_linux_amd64_v1/kinder version` shows correct `gitCommit` and `gitCommitCount`, AND `gomod.proxy: false` is explicit in `.goreleaser.yaml`, AND `goreleaser check` shows no deprecation warnings
- [ ] **GitHub Release created:** Release exists at the tag URL — but also all 5 platform binaries are present (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64), AND `checksums.txt` covers all of them, AND the old `cross.sh` workflow is disabled/retired
- [ ] **Homebrew tap configured:** `.goreleaser.yaml` has `homebrew_casks:` — but also the tap repository exists, the `HOMEBREW_TAP_TOKEN` secret is set, the cask `.rb` file was committed to the tap repo after the last release, AND `brew install patrykquantumnomad/kinder/kinder` works on macOS
- [ ] **GPU addon installed:** GPU operator pods are running — but also `kubectl get nodes -o json | jq '.items[].status.allocatable["nvidia.com/gpu"]'` returns a non-zero value, AND a test pod requesting `nvidia.com/gpu: 1` runs and `nvidia-smi` succeeds inside it, AND the pre-flight check is implemented and tested with a machine that has no GPU
- [ ] **GPU addon pre-flight check:** Check code exists — but also it returns a clear, actionable error message when the NVIDIA Container Toolkit is absent, AND it does NOT block cluster creation when the GPU addon is disabled, AND it handles the case where Docker is not running (separate error from "toolkit not installed")
- [ ] **Homebrew install documented:** README has install instructions — but also the instructions distinguish between `brew tap` + `brew install` (works for CLI users), AND the cask name in the instructions matches the actual cask name in the tap, AND the instructions are verified after the first real release (not just the draft/snapshot)

---

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| GoReleaser built wrong binary (gomod.proxy fetched upstream kind) | MEDIUM | Delete all release assets and the GitHub Release; add `gomod.proxy: false` to `.goreleaser.yaml`; re-tag and re-release |
| Homebrew tap silently not updated (PAT issue) | LOW | Add `HOMEBREW_TAP_TOKEN` secret; re-run the release workflow (GoReleaser will push the cask again if `mode: replace` is set); verify with `gh api repos/PatrykQuantumNomad/homebrew-kinder/commits` |
| GitHub Release 422 "asset already exists" on re-run | LOW | Go to GitHub Release page; delete all existing assets manually; re-run the workflow; or set `release.mode: replace` going forward |
| GPU addon: cluster created but 0 GPUs visible | MEDIUM | Check host: `docker run --rm --gpus all nvidia/cuda nvidia-smi`; if host works, check node container: `docker exec -it kinder-control-plane nvidia-smi`; if node fails, the Container Toolkit is not configured for privileged containers |
| GPU operator driver installer pods hanging | LOW | Delete the GPU operator namespace; reinstall with `--set driver.enabled=false`; the addon should embed this value, so this indicates the addon was misconfigured |
| brews deprecation breaks pipeline in GoReleaser v3 | MEDIUM | Rename `brews:` to `homebrew_casks:` in `.goreleaser.yaml`; adjust any fields that changed between the two sections; run `goreleaser check` to validate |
| Cask SHA256 mismatch after replacing release assets | MEDIUM | Never replace released assets — create a new patch version instead; if already replaced, delete the cask from the tap and re-push with the correct SHA256 from the new assets |

---

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| gomod.proxy fetches wrong module | Phase 1: GoReleaser setup | `./dist/kinder_*/kinder version` shows kinder gitCommit, not upstream kind |
| ldflags version vars lost | Phase 1: GoReleaser setup | `kinder version` output matches git tag and commit |
| fetch-depth: 0 removed | Phase 1: GoReleaser setup | Protective comment added; `goreleaser build --snapshot` passes in CI |
| GitHub Release 422 on re-run | Phase 1: GoReleaser setup | `release.mode: replace` set; draft release tested |
| Homebrew PAT missing | Phase 2: Homebrew tap | Tap repo has new commit after test release; verified with `gh api` |
| brews vs homebrew_casks | Phase 2: Homebrew tap | `goreleaser check` shows no deprecation warnings; `--cask` install works |
| Binary name / project name inference | Phase 2: Homebrew tap | Cask file has `cask "kinder"` and correct binary name |
| GPU host toolkit not configured | Phase 3: GPU addon | Pre-flight check returns actionable error on a machine without the toolkit |
| GPU operator driver mode enabled | Phase 3: GPU addon | `driver.enabled=false` hardcoded; hang scenario cannot occur |
| cgroup v2 device visibility | Phase 3: GPU addon | End-to-end test with `nvidia-smi` inside a pod succeeds on Ubuntu 22.04+ |

---

## Fork-Specific Pitfalls

Issues unique to kinder's status as a fork of `sigs.k8s.io/kind`.

### Pitfall F1: GoReleaser Infers Wrong Project Identity From Module Path

Kinder's `go.mod` module path is `sigs.k8s.io/kind`. GoReleaser reads this for metadata but uses the git repository name for the project name. If `project_name` is not explicitly set, GoReleaser may use `kind` (the last segment of the module path) instead of `kinder` (the repository name). This produces:
- Cask file named `kind.rb` instead of `kinder.rb`
- Archives named `kind_v1.0.0_linux_amd64.tar.gz` instead of `kinder_v1.0.0_linux_amd64.tar.gz`
- Homebrew conflicts with the upstream `kind` formula if both are installed

**Prevention:** Set `project_name: kinder` explicitly as the first line of `.goreleaser.yaml`.

**Phase to address:** Phase 1 (GoReleaser setup) — set `project_name` before any other configuration.

### Pitfall F2: Releasing Under the Upstream Module Path Confuses go install Users

Users familiar with `go install sigs.k8s.io/kind@latest` may try `go install sigs.k8s.io/kind@latest` and get upstream kind, not kinder. There is no way to fix this without changing the module path. Kinder should NOT be installable via `go install sigs.k8s.io/kind@latest` — this would install the wrong binary. The only supported installation methods should be `brew install` (via Homebrew tap) and direct binary download from GitHub Releases.

**Prevention:** Document clearly that `go install sigs.k8s.io/kind@latest` installs upstream kind. The README install section should lead with `brew install` and the direct download link, with `go install github.com/PatrykQuantumNomad/kinder@latest` (using the GitHub URL, not the module path) as a developer option if the module path is ever updated.

**Phase to address:** Phase 2 (documentation) — update README install instructions before publishing the Homebrew tap.

---

## Sources

- GoReleaser official documentation — Building Go modules: https://goreleaser.com/cookbooks/build-go-modules/ (HIGH confidence)
- GoReleaser official documentation — Go build configuration: https://goreleaser.com/customization/builds/go/ (HIGH confidence)
- GoReleaser official documentation — Homebrew Casks: https://goreleaser.com/customization/homebrew_casks/ (HIGH confidence)
- GoReleaser official documentation — Homebrew Formulas (deprecated): https://goreleaser.com/deprecations/ (HIGH confidence)
- GoReleaser official documentation — GitHub Actions CI: https://goreleaser.com/ci/actions/ (HIGH confidence)
- GoReleaser official documentation — Release configuration: https://goreleaser.com/customization/release/ (HIGH confidence)
- GoReleaser v2.10 announcement (homebrew_casks introduction, brews deprecation): https://goreleaser.com/blog/goreleaser-v2.10/ (HIGH confidence)
- GoReleaser GitHub issue #557 — asset override on re-run: https://github.com/goreleaser/goreleaser/issues/557 (HIGH confidence)
- GoReleaser GitHub issue #3148 — duplicate draft releases: https://github.com/goreleaser/goreleaser/issues/3148 (HIGH confidence)
- GoReleaser GitHub issue #2833 — gomod.proxy fails with templated main: https://github.com/goreleaser/goreleaser/issues/2833 (HIGH confidence)
- GoReleaser GitHub discussion #4926 — Homebrew tokens: https://github.com/orgs/goreleaser/discussions/4926 (HIGH confidence)
- NVIDIA Container Toolkit official docs — Installation guide: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html (HIGH confidence)
- NVIDIA GPU Operator official docs — Getting started: https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/getting-started.html (HIGH confidence)
- NVIDIA security bulletin Jan 2025 (ACCEPT_NVIDIA_VISIBLE_DEVICES_ENVVAR_WHEN_UNPRIVILEGED): https://nvidia.custhelp.com/app/answers/detail/a_id/5599 (HIGH confidence)
- NVIDIA Container Toolkit GitHub issue #189 — cgroup v2 rootless mode: https://github.com/NVIDIA/nvidia-container-toolkit/issues/189 (HIGH confidence)
- NVIDIA GPU Operator GitHub issue #700 — unprivileged container GPU bypass: https://github.com/NVIDIA/gpu-operator/issues/700 (HIGH confidence)
- Jacob Tomlinson — Quick hack: Adding GPU support to kind (community reference): https://jacobtomlinson.dev/posts/2022/quick-hack-adding-gpu-support-to-kind/ (MEDIUM confidence — community guide, 2022)
- SeineAI nvidia-kind-deploy (community toolkit for kind + GPU operator): https://github.com/SeineAI/nvidia-kind-deploy (MEDIUM confidence — community project, undated)
- Kubernetes official docs — Schedule GPUs: https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/ (HIGH confidence)
- Homebrew official docs — How to Create and Maintain a Tap: https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap (HIGH confidence)
- Homebrew official docs — Cask Cookbook: https://docs.brew.sh/Cask-Cookbook (HIGH confidence)
- Kinder codebase direct analysis: `hack/release/build/cross.sh`, `.github/workflows/release.yml`, `Makefile`, `go.mod`, `pkg/internal/kindversion/` — (HIGH confidence — primary sources)

---
*Pitfalls research for: kinder — distribution pipeline (GoReleaser + GitHub Releases + Homebrew tap) + NVIDIA GPU addon*
*Researched: 2026-03-04*
