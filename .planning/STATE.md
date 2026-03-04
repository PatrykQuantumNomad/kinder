---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Distribution & GPU Support
status: in_progress
stopped_at: "Completed 36-02-PLAN.md"
last_updated: "2026-03-04T23:15:00Z"
last_activity: 2026-03-04 — Completed 36-02: Homebrew install instructions and updated download URLs on installation page
progress:
  total_phases: 3
  completed_phases: 1
  total_plans: 7
  completed_plans: 4
  percent: 57
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-04)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 36 — Homebrew Tap

## Current Position

Phase: 36 of 37 (Homebrew Tap) — Phase 35 complete
Plan: 2 of 2 in current phase complete (36-01 pending PAT creation checkpoint, 36-02 complete)
Status: In progress
Last activity: 2026-03-04 — Completed 36-02: Homebrew install instructions and updated download URLs on installation page

Progress: [█████░░░░░] 57%

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

### Pending Todos

None.

### Blockers/Concerns

- Phase 37 (GPU): GPU Operator vs standalone device plugin decision must be resolved during planning (research flag: examine nvkind source). ContainerdConfigPatches vs post-provision-only also unresolved.
- Phase 37 (GPU): End-to-end validation requires Linux host with real NVIDIA GPU — plan accordingly.
- Phase 35 (GoReleaser): RESOLVED — Phase 35 complete. GoReleaser pipeline operational, goreleaser check passes, snapshot validated.
- Phase 36 (Homebrew): HOMEBREW_TAP_TOKEN PAT must be created and stored as repo secret before any tagged release; GITHUB_TOKEN cannot push cross-repo.

## Session Continuity

Last session: 2026-03-04T23:15:00Z
Stopped at: Completed 36-02-PLAN.md
Resume file: None
