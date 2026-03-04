---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Distribution & GPU Support
status: in_progress
stopped_at: "Completed 35-01-PLAN.md"
last_updated: "2026-03-04T21:05:44.672Z"
last_activity: 2026-03-04 — Phase 35 Plan 01 complete: .goreleaser.yaml and Makefile targets
progress:
  total_phases: 3
  completed_phases: 0
  total_plans: 7
  completed_plans: 1
  percent: 14
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-04)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 35 — GoReleaser Foundation

## Current Position

Phase: 35 of 37 (GoReleaser Foundation)
Plan: 1 of 2 in current phase (Plan 01 complete)
Status: In progress
Last activity: 2026-03-04 — Phase 35-01 complete: .goreleaser.yaml created, Makefile targets added, snapshot build validated

Progress: [█░░░░░░░░░] 14%

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

### Pending Todos

None.

### Blockers/Concerns

- Phase 37 (GPU): GPU Operator vs standalone device plugin decision must be resolved during planning (research flag: examine nvkind source). ContainerdConfigPatches vs post-provision-only also unresolved.
- Phase 37 (GPU): End-to-end validation requires Linux host with real NVIDIA GPU — plan accordingly.
- Phase 35 (GoReleaser): RESOLVED — `gomod.proxy: false` and `project_name: kinder` confirmed set in .goreleaser.yaml; snapshot build validated.
- Phase 36 (Homebrew): HOMEBREW_TAP_TOKEN PAT must be created and stored as repo secret before any tagged release; GITHUB_TOKEN cannot push cross-repo.

## Session Continuity

Last session: 2026-03-04T21:04:51Z
Stopped at: Completed 35-01-PLAN.md
Resume file: None
