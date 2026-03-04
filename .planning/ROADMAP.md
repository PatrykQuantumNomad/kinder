# Roadmap: Kinder

## Milestones

- SHIPPED **v1.0 Batteries Included** - Phases 1-8 (shipped 2026-03-01)
- SHIPPED **v1.1 Kinder Website** - Phases 9-14 (shipped 2026-03-02)
- SHIPPED **v1.2 Branding & Polish** - Phases 15-18 (shipped 2026-03-02)
- SHIPPED **v1.3 Harden & Extend** - Phases 19-24 (shipped 2026-03-03)
- SHIPPED **v1.4 Code Quality & Features** - Phases 25-29 (shipped 2026-03-04)
- SHIPPED **v1.5 Website Use Cases & Documentation** - Phases 30-34 (shipped 2026-03-04)
- ACTIVE **v2.0 Distribution & GPU Support** - Phases 35-37 (in progress)

## Phases

<details>
<summary>SHIPPED v1.0 Batteries Included (Phases 1-8) - SHIPPED 2026-03-01</summary>

See `.planning/milestones/v1.0-ROADMAP.md` for full phase details.

Phases 1-8: Foundation, MetalLB, Metrics Server, CoreDNS Tuning, Envoy Gateway, Dashboard, Integration Testing, Gap Closure.

</details>

<details>
<summary>SHIPPED v1.1 Kinder Website (Phases 9-14) - SHIPPED 2026-03-02</summary>

See `.planning/milestones/v1.1-ROADMAP.md` for full phase details.

Phases 9-14: Scaffold & Deploy Pipeline, Dark Theme, Documentation Content, Landing Page, Assets & Identity, Polish & Validation.

</details>

<details>
<summary>SHIPPED v1.2 Branding & Polish (Phases 15-18) - SHIPPED 2026-03-02</summary>

Phases 15-18: Logo, SEO, Docs Rewrite, Dark Theme Enforcement.

</details>

<details>
<summary>SHIPPED v1.3 Harden & Extend (Phases 19-24) - SHIPPED 2026-03-03</summary>

Phases 19-24: Bug Fixes, Provider Code Deduplication, Config Type Additions, Local Registry Addon, Cert-Manager Addon, CLI Diagnostic Tools.

</details>

<details>
<summary>SHIPPED v1.4 Code Quality & Features (Phases 25-29) - SHIPPED 2026-03-04</summary>

See `.planning/milestones/v1.4-ROADMAP.md` for full phase details.

Phases 25-29: Foundation (Go 1.24, golangci-lint v2, layer fix), Architecture (context.Context, addon registry), Unit Tests (FakeNode/FakeCmd test infra), Parallel Execution (wave-based errgroup), CLI Features (JSON output, profile presets).

</details>

<details>
<summary>SHIPPED v1.5 Website Use Cases & Documentation (Phases 30-34) - SHIPPED 2026-03-04</summary>

See `.planning/milestones/v1.5-ROADMAP.md` for full phase details.

Phases 30-34: Foundation Fixes, Addon Page Depth, CLI Reference, Tutorials, Verification & Polish.

</details>

### ACTIVE v2.0 Distribution & GPU Support (In Progress)

**Milestone Goal:** Make kinder installable via Homebrew with pre-built binaries from GitHub Releases, and add full NVIDIA GPU stack as a new addon.

## Phase Details

### Phase 35: GoReleaser Foundation
**Goal**: Users can download pre-built kinder binaries for all platforms from GitHub Releases, produced by an automated pipeline that replaces cross.sh
**Depends on**: Phase 34 (v1.5 complete)
**Requirements**: REL-01, REL-02, REL-03, REL-04, REL-05, REL-06
**Plans:** 2/2 plans executed — COMPLETE
**Success Criteria** (what must be TRUE):
  1. Running `goreleaser build --snapshot --clean` locally produces correct kinder binaries for linux/darwin amd64+arm64 and windows/amd64 — not upstream kind
  2. `kinder version` on a snapshot binary shows the real git commit hash, not empty or "unknown"
  3. A tagged release on GitHub creates a Release page with five platform archives, a checksums.txt, and an auto-generated changelog — without any manual intervention
  4. The GitHub Actions release workflow uses goreleaser-action replacing cross.sh and softprops; the old cross.sh is retired in the same commit
  5. `goreleaser check` passes with zero errors and zero deprecation warnings

Plans:
- [x] 35-01-PLAN.md — GoReleaser config and local snapshot validation
- [x] 35-02-PLAN.md — Release workflow migration and cross.sh retirement

### Phase 36: Homebrew Tap
**Goal**: macOS users can install kinder with a single `brew install` command from a maintained custom tap, without needing a Go toolchain
**Depends on**: Phase 35 (GoReleaser must produce at least one tagged release before tap formula can reference archive hashes)
**Requirements**: BREW-01, BREW-02, BREW-03, SITE-01
**Plans:** 1/2 plans executed
**Success Criteria** (what must be TRUE):
  1. `brew install patrykquantumnomad/kinder/kinder` succeeds on macOS and produces a working kinder binary
  2. After a new tagged release, the `Casks/kinder.rb` file in the homebrew-kinder tap repo is updated automatically — verifiable via `gh api repos/PatrykQuantumNomad/homebrew-kinder/commits`
  3. The kinder installation page at kinder.patrykgolabek.dev shows Homebrew install instructions alongside make install

Plans:
- [ ] 36-01-PLAN.md — Tap repo creation, GoReleaser cask config, and PAT secret setup
- [x] 36-02-PLAN.md — Installation page update with Homebrew instructions and corrected download URLs

### Phase 37: NVIDIA GPU Addon
**Goal**: Users on Linux with NVIDIA GPUs can create a kind cluster that exposes GPU resources to pods via a single config field, with actionable pre-flight error messages if prerequisites are missing
**Depends on**: Phase 34 (v1.5 complete — independent of distribution pipeline, can proceed in parallel with 35-36)
**Requirements**: GPU-01, GPU-02, GPU-03, GPU-04, GPU-05, GPU-06, SITE-02
**Success Criteria** (what must be TRUE):
  1. A pod requesting `nvidia.com/gpu: 1` is scheduled and runs successfully on a kinder cluster created with `addons.nvidiaGPU: true` on a Linux host with NVIDIA drivers installed
  2. `kinder create cluster --config gpu-cluster.yaml` on macOS or Windows prints a clear informational message that the GPU addon is Linux-only and skips without failing cluster creation
  3. `kinder doctor` reports the NVIDIA driver version, container toolkit presence, and whether the nvidia runtime is configured in Docker — with actionable fix commands for any missing prerequisite
  4. Running `kinder create cluster` with `addons.nvidiaGPU: true` but missing nvidia-container-toolkit configured as the Docker runtime fails fast with an error message telling the user exactly which command to run to fix it — not after 10 minutes of cluster creation
  5. The GPU addon documentation page at kinder.patrykgolabek.dev covers prerequisites, config field, usage example, and a troubleshooting section for the 0-GPUs-allocated failure mode
**Plans**: TBD

Plans:
- [ ] 37-01: Config API extension and installnvidiagpu package skeleton
- [ ] 37-02: Embedded manifests, pre-flight checks, and kinder doctor integration
- [ ] 37-03: GPU documentation page and end-to-end validation

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8. v1.0 phases | v1.0 | 12/12 | Complete | 2026-03-01 |
| 9-14. v1.1 phases | v1.1 | 8/8 | Complete | 2026-03-02 |
| 15-18. v1.2 phases | v1.2 | 4/4 | Complete | 2026-03-02 |
| 19-24. v1.3 phases | v1.3 | 8/8 | Complete | 2026-03-03 |
| 25-29. v1.4 phases | v1.4 | 13/13 | Complete | 2026-03-04 |
| 30-34. v1.5 phases | v1.5 | 7/7 | Complete | 2026-03-04 |
| 35. GoReleaser Foundation | v2.0 | 2/2 | Complete | 2026-03-04 |
| 36. Homebrew Tap | v2.0 | 1/2 | In progress | - |
| 37. NVIDIA GPU Addon | v2.0 | 0/3 | Not started | - |
