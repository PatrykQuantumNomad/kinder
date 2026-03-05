---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Distribution & GPU Support
status: executing
stopped_at: Completed 37-01-PLAN.md
last_updated: "2026-03-05T00:26:46.845Z"
last_activity: "2026-03-04 — Phase 37 Plan 02 complete: NVIDIA doctor checks (nvidia-driver, nvidia-container-toolkit, nvidia-docker-runtime) gated on Linux"
progress:
  total_phases: 3
  completed_phases: 2
  total_plans: 7
  completed_plans: 6
  percent: 96
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-04)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 37 — NVIDIA GPU Addon

## Current Position

Phase: 37 of 37 (NVIDIA GPU Addon) — In progress, Plan 2 of 3 complete
Plan: 2 of 3 in Phase 37 complete
Status: In progress
Last activity: 2026-03-04 — Phase 37 Plan 02 complete: NVIDIA doctor checks (nvidia-driver, nvidia-container-toolkit, nvidia-docker-runtime) gated on Linux

Progress: [██████████] 96%

## Performance Metrics

**Velocity:**
- v1.0: 12 plans, 8 phases, 1 day
- v1.1: 8 plans, 6 phases, 2 days
- v1.2: 4 phases, 1 day
- v1.3: 8 plans, 6 phases, ~5 hours
- v1.4: 13 plans, 5 phases, 2 days
- v1.5: 7 plans, 5 phases, 1 day

## Accumulated Context

### Decisions

- v1.0: Fork kind, addons as creation actions, on-by-default opt-out, go:embed for manifests
- v1.1: Astro + Starlight, kinder-site/ dir, dark-only mode, npm for CI
- v1.2: Kinder logo from modified kind robot, favicon.ico over SVG, llms.txt for GEO
- v1.3: common/ provider dedup, local registry addon, cert-manager addon, CLI diagnostic tools
- v1.4: Context in struct (not param), wave-based parallel (not DAG), sync.OnceValues for Nodes(), errgroup.SetLimit(3), flagpole/switch/json.NewEncoder pattern, CreateWithAddonProfile with 4 presets
- v1.5: Core/optional addon grouping, Symptom/Cause/Fix troubleshooting pattern, tutorial structure (overview/prerequisites/steps/cleanup), ci profile = MetricsServer + CertManager only
- v2.0 roadmap: SITE-01 merged into Phase 36 (Homebrew), SITE-02 merged into Phase 37 (GPU addon) — no standalone website phase needed; GPU addon independent of distribution pipeline
- Phase 35-01: -trimpath is a go build compiler flag (use flags:), not a linker flag (not ldflags:); gitCommitCount safely omitted from GoReleaser ldflags
- Phase 35-02: goreleaser-action@v7 replaces cross.sh + softprops atomically; go-version-file: .go-version simplifies version read; push-latest-cli.sh disabled (upstream kind GCS script not used by fork)
- Phase 36-02: Direct users to GitHub Releases page for downloads (no hardcoded versioned URLs that go stale); Homebrew section placed before binary download section as preferred macOS method
- Phase 36-01: homebrew_casks (not deprecated brews:), skip_upload:auto for pre-release safety, Casks/ directory (not Formula/), HOMEBREW_TAP_TOKEN fine-grained PAT scoped to homebrew-kinder for cross-repo push
- [Phase 37-02]: Use 'warn' (not 'fail') for all missing NVIDIA components — GPU is optional so doctor must not exit 1 on non-GPU Linux machines
- [Phase 37-02]: Ok-case formatter prints message only when non-empty — driver version shows for nvidia-driver ok, no trailing colon for docker/kubectl
- [Phase 37]: NvidiaGPU uses boolValOptIn (nil=false) unlike all other addons which use boolVal (nil=true) — GPU is opt-in
- [Phase 37]: currentOS and checkPrerequisites are package-level vars for test injection without build tags in installnvidiagpu

### Pending Todos

None.

### Blockers/Concerns

- Phase 37 (GPU): GPU Operator vs standalone device plugin decision must be resolved during planning (research flag: examine nvkind source). ContainerdConfigPatches vs post-provision-only also unresolved.
- Phase 37 (GPU): End-to-end validation requires Linux host with real NVIDIA GPU — plan accordingly.
- Phase 35 (GoReleaser): RESOLVED — Phase 35 complete. GoReleaser pipeline operational, goreleaser check passes, snapshot validated.
- Phase 36 (Homebrew): RESOLVED — Phase 36 complete. HOMEBREW_TAP_TOKEN PAT created and stored. Tap repo exists. Cask config wired. End-to-end verification requires a real tagged release.

## Session Continuity

Last session: 2026-03-05T00:26:46.843Z
Stopped at: Completed 37-01-PLAN.md
Resume file: None
