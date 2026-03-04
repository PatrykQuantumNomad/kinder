# Project Research Summary

**Project:** kinder — v2.0 milestone (Distribution pipeline + NVIDIA GPU addon)
**Domain:** Go CLI tool distribution (GoReleaser + GitHub Releases + Homebrew tap) + NVIDIA GPU Kubernetes addon
**Researched:** 2026-03-04
**Confidence:** HIGH for distribution pipeline; MEDIUM for NVIDIA GPU addon in kind context

## Executive Summary

Kinder v2.0 adds two orthogonal capabilities to an existing, working Go CLI tool: a professional distribution pipeline and an NVIDIA GPU addon. The distribution pipeline replaces a hand-rolled `cross.sh` + `softprops/action-gh-release` workflow with GoReleaser (v2.14.1), which handles cross-platform binary builds, SHA-256 checksums, automated changelogs, GitHub Release creation, and Homebrew Cask publishing to a custom tap — all from a single `.goreleaser.yaml`. The recommended approach is well-documented and carries HIGH confidence: GoReleaser is the de facto standard for Go CLI distribution, the configuration maps directly from the existing Makefile, and all versions are live-verified. The one structural complexity is kinder's fork status: `go.mod` declares `module sigs.k8s.io/kind` (upstream path), not the GitHub repository URL, which requires explicit `gomod.proxy: false` and `project_name: kinder` in the GoReleaser config to prevent silent wrong-binary builds.

The NVIDIA GPU addon adds an 8th addon (`installnvidiagpu`) following the identical `go:embed` + `kubectl apply` pattern as the existing 7 addons. The critical distinction from all other addons is that GPU support is Linux-only, opt-in by default (not on by default like the others), and depends on host-level prerequisites (NVIDIA drivers + nvidia-container-toolkit) that kinder cannot install. The recommended approach is to use the standalone NVIDIA k8s-device-plugin DaemonSet (NOT the full GPU Operator), because kind is not on NVIDIA's official GPU Operator supported platforms list. The addon must include a pre-flight check for host toolkit configuration — this is the most important part of the implementation and the most common failure mode.

The primary risk across both workstreams is silent failure: GoReleaser can build the wrong binary (upstream kind instead of kinder) if `gomod.proxy` is not explicitly disabled; the Homebrew tap can silently stop updating if the wrong token is used; the GPU addon can install successfully while exposing 0 GPU resources if host prerequisites are missing. The mitigation pattern is consistent across all three risks: verify outputs explicitly (run `kinder version` after GoReleaser builds, check tap repo commits via `gh api`, test GPU allocation with a real workload) rather than trusting that workflow exit 0 means success.

---

## Key Findings

### Recommended Stack

The distribution pipeline requires no Go code changes — it is purely tooling. GoReleaser OSS v2.14.1 (released 2026-02-25) replaces `hack/release/build/cross.sh` for CI release builds while the existing Makefile `build`/`install` targets remain unchanged for local development. The `goreleaser-action@v7` GitHub Action replaces `softprops/action-gh-release@v2` in the release workflow. A new `homebrew-kinder` repository under `PatrykQuantumNomad` must be created before any tagged release, and a GitHub PAT (not the default `GITHUB_TOKEN`) with `contents: write` scope on that repo must be added as a secret before the release workflow is updated.

The NVIDIA GPU addon embeds pre-rendered manifests via `go:embed`, identical to all other addons. The NVIDIA k8s-device-plugin v0.18.2 (a DaemonSet) is the correct component — not the GPU Operator — because kind is not an officially supported GPU Operator platform. Helm is used only during development to pre-render manifests offline; kinder itself has no Helm runtime dependency.

**Core technologies:**
- GoReleaser OSS v2.14.1: cross-platform builds, GitHub Release, Homebrew Cask — industry standard; single config replaces `cross.sh` + `softprops`
- goreleaser-action v7.0.0: GitHub Actions integration for GoReleaser — official action, defaults to GoReleaser v2
- `homebrew-kinder` tap repo: separate GitHub repository holding GoReleaser-generated Cask file — required for `brew install`
- NVIDIA k8s-device-plugin v0.18.2: Kubernetes DaemonSet that exposes GPU resources to the scheduler — the only approach confirmed to work with kind
- GitHub PAT (HOMEBREW_TAP_TOKEN): separate from GITHUB_TOKEN — mandatory for cross-repo tap pushes; GITHUB_TOKEN is scoped to the triggering repo only

### Expected Features

**Must have (table stakes for v2.0):**
- Pre-built binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 — users should not need to install Go toolchain to try kinder
- SHA-256 checksums for all binaries — GoReleaser generates `checksums.txt` automatically
- `brew install patrykquantumnomad/kinder/kinder` installation path — the biggest single adoption barrier to remove
- Automated changelog generation — GoReleaser generates from git commits with configurable prefix filtering
- NVIDIA GPU addon (`addons.nvidiaGPU: true`) — enables `nvidia.com/gpu` resource in kind clusters on Linux; Linux-only, opt-in
- `kinder doctor` GPU pre-flight checks — validates NVIDIA driver, container toolkit, and runtime configuration before cluster creation

**Should have (competitive, add in v2.x):**
- GPU time-slicing ConfigMap — multiple pods sharing one GPU via `sharing.timeSlicing.resources`; defer until basic GPU flow is validated
- Shell completions in Homebrew Cask — bash/zsh/fish completions via GoReleaser `homebrew_casks.completions` config; nice-to-have
- Cosign binary signing — supply chain security; emerging best practice, add when users request it

**Defer (v3+):**
- Chocolatey/Winget for Windows — limited kind Windows support; defer until adoption data justifies maintenance
- Homebrew core submission — requires project maturity, build-from-source, 30-day review; fork status and non-standard module path complicate this indefinitely
- AMD GPU addon (ROCm) — much smaller user base than NVIDIA; defer

### Architecture Approach

The distribution pipeline is a workflow-level change with no Go source modifications: `.goreleaser.yaml` replaces `cross.sh` as the CI release artifact producer; `release.yml` replaces `softprops/action-gh-release@v2` with `goreleaser-action@v7`. The NVIDIA GPU addon is a minimal Go source addition: a new package `pkg/cluster/internal/create/actions/installnvidiagpu/` with one Go file and two embedded YAML manifests, plus config API field additions to four existing files (`v1alpha4/types.go`, `v1alpha4/default.go`, `internal/config/types.go`, `convert_v1alpha4.go`) and a one-line registration entry in `create.go`. The addon belongs in Wave 1 (parallel execution alongside other addons) and defaults to `false` — unlike all other addons which default to `true`.

**Major components:**
1. `.goreleaser.yaml` (NEW) — defines all build targets, archive formats, checksums, GitHub Release, and Homebrew Cask publishing; must replicate Makefile ldflags exactly using GoReleaser template variables; `project_name: kinder` must be explicit to avoid inference from module path
2. `installnvidiagpu/` package (NEW) — `Execute(*ActionContext)` applies two embedded manifests (RuntimeClass "nvidia" + device plugin DaemonSet) via `kubectl apply`; pre-flight check validates host toolkit before proceeding; defaults to `false` unlike other addons
3. Config API changes (MODIFIED, 4 files) — adds `NvidiaGPU *bool` (public v1alpha4) / `NvidiaGPU bool` (internal) with `false` default; follows identical `*bool` conversion pattern as all other addons
4. `homebrew-kinder` repository (NEW repo) — separate GitHub repo; GoReleaser pushes updated `Casks/kinder.rb` on every tagged release via HOMEBREW_TAP_TOKEN

### Critical Pitfalls

1. **GoReleaser `gomod.proxy` builds upstream kind instead of kinder** — the fork's `go.mod` declares `module sigs.k8s.io/kind`; if `gomod.proxy: true` is set, GoReleaser fetches upstream kind source from the Go module proxy and produces the wrong binary. Prevention: set `gomod.proxy: false` explicitly and `project_name: kinder` explicitly in `.goreleaser.yaml`; verify by running `kinder version` on the built binary; verify with `goreleaser build --snapshot --clean` before enabling release mode.

2. **GITHUB_TOKEN cannot push to the Homebrew tap repo** — the default Actions token is scoped to the triggering repository only; GoReleaser logs a warning but does not fail the pipeline, so the tap stops updating without any visible CI failure. Prevention: create a PAT with `contents: write` on `homebrew-kinder`, store as `HOMEBREW_TAP_TOKEN`, verify after every release with `gh api repos/PatrykQuantumNomad/homebrew-kinder/commits`.

3. **ldflags version metadata not replicated in GoReleaser** — the Makefile injects `gitCommit` and `gitCommitCount` via `-X` flags; GoReleaser's default ldflags omit these, resulting in empty version strings in release binaries. Prevention: explicitly replicate all `-X` flags in `.goreleaser.yaml` using `{{ .FullCommit }}`; verify with `kinder version` after snapshot build; simplest approach is to drop `gitCommitCount` from release builds (it is not user-visible in tagged releases).

4. **GPU addon installs successfully but reports 0 GPUs** — if `nvidia-container-toolkit` is installed but not configured as the Docker runtime, kind nodes start without GPU access; the device plugin DaemonSet runs and reports 0 GPUs with no obvious cluster-level error. Prevention: implement pre-flight check in `Execute()` that verifies `docker info --format {{json .Runtimes}}` contains the `nvidia` key before proceeding; return an actionable error message with the exact command needed to fix it.

5. **`brews:` (deprecated) instead of `homebrew_casks:`** — deprecated since GoReleaser v2.10, removed in v3; produces a Homebrew Formula (wrong format for binary-only distribution) instead of a Cask. Prevention: use `homebrew_casks:` from day one; run `goreleaser check` to verify no deprecation warnings; never copy from tutorials predating GoReleaser v2.10.

---

## Implications for Roadmap

Both workstreams can proceed largely in parallel. The dependency chain is: GoReleaser must be configured and validated before the Homebrew tap can be tested end-to-end (the tap formula references GitHub Release archive URLs). The GPU addon has no dependency on the distribution pipeline and can be developed entirely in parallel with Phases 1 and 2.

### Phase 1: GoReleaser Foundation

**Rationale:** The distribution pipeline is a prerequisite for Homebrew tap publishing. GoReleaser must be configured, validated locally with `--snapshot`, and the release workflow must be updated before any tagged release is pushed. This phase has the highest risk from the fork-specific `gomod.proxy` pitfall and the ldflags replication requirement — both must be caught before any public release. The existing `cross.sh` + `softprops` workflow must be retired in the same commit that enables GoReleaser to prevent duplicate asset uploads (422 errors from GitHub API).

**Delivers:** `.goreleaser.yaml` with cross-platform builds, SHA-256 checksums, automated changelog; updated `release.yml` replacing `cross.sh` + `softprops`; two new Makefile targets (`goreleaser-check`, `goreleaser-snapshot`); validated binary with correct `kinder version` output showing real git commit.

**Addresses:** Pre-built binaries (table stakes), SHA-256 checksums (table stakes), reproducible builds with version metadata.

**Avoids:** Pitfall 1 (gomod.proxy wrong binary — explicit `false`), Pitfall 3 (ldflags version vars — replicate all `-X` flags), Pitfall 5 (GitHub Release 422 on re-run — `release.mode: replace`), fetch-depth: 0 comment to prevent future breakage.

**Research flag:** Standard patterns — GoReleaser is well-documented; no additional research needed. Fork-specific `gomod.proxy` issue is fully understood from PITFALLS.md research.

### Phase 2: Homebrew Tap

**Rationale:** Depends on Phase 1 (Homebrew Cask formula references GitHub Release archive URLs by hash; GoReleaser must produce at least one tagged release before the tap formula can be generated and tested). The tap repository must exist and HOMEBREW_TAP_TOKEN must be configured before any release tag is pushed — GoReleaser silently fails to push the cask if the tap repo does not exist.

**Delivers:** `homebrew-kinder` repository created with `Casks/` directory; `homebrew_casks:` section added to `.goreleaser.yaml` (using correct non-deprecated key); HOMEBREW_TAP_TOKEN secret configured; `brew install patrykquantumnomad/kinder/kinder` working on macOS after first real tagged release.

**Addresses:** `brew install` table-stakes feature; removes the biggest single adoption barrier; macOS Gatekeeper quarantine documented via `caveats` message.

**Avoids:** Pitfall 2 (GITHUB_TOKEN PAT scope — dedicated HOMEBREW_TAP_TOKEN PAT), Pitfall 4 (brews vs homebrew_casks — use only `homebrew_casks:`), Pitfall 10 (binary name inference — explicit `project_name: kinder`).

**Research flag:** Standard patterns — Homebrew tap structure and GoReleaser `homebrew_casks` are well-documented in official sources.

### Phase 3: NVIDIA GPU Addon

**Rationale:** Fully independent of the distribution pipeline; can be developed in parallel with Phases 1 and 2 or sequentially afterward. It is the highest-complexity addition in this milestone: config API changes in 4 files, a new addon package with two embedded manifests, and a pre-flight check implementation. The GPU-on-kind approach is community-documented but not officially supported by NVIDIA, so certain implementation details (ContainerdConfigPatches vs extraMounts, GPU Operator vs device plugin) require resolution via nvkind source examination before writing the addon.

**Delivers:** `installnvidiagpu/` package with embedded RuntimeClass and device plugin DaemonSet manifests; `NvidiaGPU *bool` config API field (opt-in, defaults `false`); Wave 1 registration in `create.go`; `kinder doctor` GPU pre-flight checks with actionable error messages; clear documentation of Linux-only constraint and host prerequisites.

**Addresses:** NVIDIA GPU addon (unique capability, no equivalent in kind-based tools), `kinder doctor` GPU checks (UX prerequisite for GPU users).

**Avoids:** Pitfall 7 (host toolkit pre-flight check before cluster creation), Pitfall 8 (driver.enabled=false hardcoded — GPU Operator path if chosen), Pitfall 9 (cgroup v2 + accept-nvidia-visible-devices-as-volume-mounts config requirement).

**Research flag:** Needs deeper research during phase planning. Specific areas requiring resolution before implementation: (a) GPU Operator vs standalone device plugin — STACK.md and FEATURES.md disagree on the correct approach; nvkind source code is the tiebreaker; (b) whether `ContainerdConfigPatches` is needed in the kind cluster config for GPU — FEATURES.md says yes, ARCHITECTURE.md says no; (c) end-to-end validation requires a Linux host with a real NVIDIA GPU.

### Phase Ordering Rationale

- **GoReleaser before Homebrew:** The Homebrew Cask formula references GitHub Release archive URLs by hash. GoReleaser must produce at least one tagged release before the tap formula can be generated, tested, and verified end-to-end.
- **GPU addon is parallel:** No dependency on distribution pipeline phases. Can be code-reviewed and merged at any point. It does not affect or conflict with the release pipeline.
- **Pre-flight checks in Phase 3, not a separate phase:** GPU `kinder doctor` checks are tightly coupled to the addon — they share precondition knowledge and must be implemented together to avoid shipping an addon without actionable error messages.
- **Retire cross.sh in Phase 1:** Running GoReleaser and `cross.sh` in parallel would produce duplicate release assets causing 422 errors from the GitHub API. The retirement must happen in the same PR that enables GoReleaser release mode.

### Research Flags

Phases needing deeper research during planning:
- **Phase 3 (GPU addon):** The GPU Operator vs device-plugin decision must be resolved by examining nvkind source code. The `ContainerdConfigPatches` TOML content discrepancy between FEATURES.md and ARCHITECTURE.md must be resolved before implementation begins. End-to-end GPU validation requires dedicated Linux hardware with an NVIDIA GPU.

Phases with standard patterns (skip research-phase):
- **Phase 1 (GoReleaser):** Fully documented in official GoReleaser docs; all fork-specific pitfalls identified and addressed; all versions live-verified via GitHub API.
- **Phase 2 (Homebrew tap):** Standard GoReleaser `homebrew_casks` pattern; well-documented in official GoReleaser and Homebrew docs.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All versions live-verified via GitHub API; GoReleaser v2.14.1, goreleaser-action v7.0.0, k8s-device-plugin v0.18.2 confirmed; go.mod analyzed directly from codebase |
| Features | HIGH for distribution; MEDIUM for GPU | GoReleaser/Homebrew features fully confirmed from official docs; GPU addon approach (device plugin vs GPU Operator) confirmed from NVIDIA's platform support page — kind is explicitly absent; specific GPU Operator vs device plugin recommendation differs between STACK.md and FEATURES.md |
| Architecture | HIGH for distribution; MEDIUM for GPU | Distribution architecture reads directly from existing kinder codebase; GPU addon architecture synthesized from community guides — the `ContainerdConfigPatches` approach is unverified end-to-end on a real kind+GPU setup; nvkind source not examined |
| Pitfalls | HIGH | GoReleaser pitfalls sourced from official docs and verified issue tracker; GPU pitfalls from NVIDIA security bulletins, official container toolkit docs, and Kubernetes GPU scheduling docs |

**Overall confidence:** HIGH for the distribution pipeline milestone; MEDIUM for GPU addon implementation specifics.

### Gaps to Address

- **GPU Operator vs standalone device plugin (direct contradiction between research files):** STACK.md recommends GPU Operator v25.10.1 with `driver.enabled=false`. FEATURES.md explicitly recommends the standalone k8s-device-plugin DaemonSet and states the GPU Operator is NOT supported on kind. FEATURES.md reasoning is stronger (cites NVIDIA's official platform support list). Resolve in Phase 3 planning by examining nvkind (NVIDIA's own kind+GPU tool) source code to determine which approach NVIDIA itself uses.

- **ContainerdConfigPatches vs no-op for GPU addon (direct contradiction between research files):** FEATURES.md says the GPU addon must apply a `containerdConfigPatches` TOML patch to inject the NVIDIA runtime into the kind node's containerd config (required before `Provision()`). ARCHITECTURE.md says the addon is purely post-provision with no config patches needed (the host nvidia-container-toolkit handles this). This is a critical implementation detail that determines the two-phase vs one-phase addon structure. Resolve before Phase 3 begins.

- **COMMIT_COUNT in GoReleaser:** The existing Makefile derives `COMMIT_COUNT` via `git describe --tags`; GoReleaser has no built-in template for this. Both STACK.md and FEATURES.md recommend dropping `gitCommitCount` from release binary ldflags (it only appears in pre-release version strings, not tagged releases). Validate this decision is acceptable before Phase 1 implementation.

- **GPU end-to-end validation gap:** No research source provides a tested, working kind+NVIDIA configuration from 2025 or later. The nvkind tool (NVIDIA's own implementation at `github.com/NVIDIA/nvkind`) is the authoritative reference and must be examined before writing the addon.

---

## Sources

### Primary (HIGH confidence)
- GoReleaser official docs (goreleaser.com/customization/, ci/actions/, deprecations/) — all GoReleaser configuration, `homebrew_casks` feature, `brews` deprecation
- goreleaser/goreleaser GitHub API — v2.14.1 release (2026-02-25) confirmed
- goreleaser/goreleaser-action GitHub API — v7.0.0 release (2026-02-21) confirmed
- NVIDIA/gpu-operator GitHub API — v25.10.1 release (2025-12-04) confirmed
- NVIDIA/k8s-device-plugin GitHub API — v0.18.2 release (2026-01-23) confirmed
- NVIDIA GPU Operator platform support docs — confirmed kind is NOT listed as a supported platform
- NVIDIA Container Toolkit install guide — host prerequisites, cgroup v2 requirements
- NVIDIA security bulletin (Jan 2025) — `accept-nvidia-visible-devices-envvar-when-unprivileged` CVE-2024-0132
- Homebrew docs (docs.brew.sh) — tap structure, Cask vs Formula distinction, Cask Cookbook
- Kubernetes docs — scheduling GPUs guide
- Kinder codebase (direct read) — Makefile, release.yml, cross.sh, kindversion package, all 7 addon packages, config API files (v1alpha4/types.go, internal/apis/config/types.go)
- GoReleaser issue tracker — asset override on re-run (#557), gomod.proxy issues (#2833), Homebrew tokens discussion (#4926)

### Secondary (MEDIUM confidence)
- NVIDIA/nvkind — NVIDIA's own kind+GPU tool; confirms the approach but source not read directly
- SeineAI/nvidia-kind-deploy — community toolkit for kind + GPU operator; undated
- Kind + CAPI + GPU community gist (mproffitt) — `containerdConfigPatches` and `extraMounts` approach for kind+GPU
- Jim Angel blog post — NVIDIA GPU on Kubernetes practical guide (community-verified)
- Jacob Tomlinson blog post — Adding GPU support to kind (2022; may be outdated for modern nvidia-container-toolkit security defaults)

### Tertiary (LOW confidence)
- Stack Overflow answers on GoReleaser `brews` vs `homebrew_casks` — predated the v2.10 migration; treat as historical context only

---
*Research completed: 2026-03-04*
*Ready for roadmap: yes*
