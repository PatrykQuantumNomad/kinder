---
gsd_state_version: 1.0
milestone: v2.1
milestone_name: Known Issues & Proactive Diagnostics
status: active
stopped_at: null
last_updated: "2026-03-06"
last_activity: "2026-03-06 — Milestone v2.1 started"
progress:
  total_phases: 0
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-06)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Defining requirements for v2.1

## Current Position

Phase: Not started (defining requirements)
Plan: —
Status: Defining requirements
Last activity: 2026-03-06 — Milestone v2.1 started

## Performance Metrics

**Velocity:**
- v1.0: 12 plans, 8 phases, 1 day
- v1.1: 8 plans, 6 phases, 2 days
- v1.2: 4 phases, 1 day
- v1.3: 8 plans, 6 phases, ~5 hours
- v1.4: 13 plans, 5 phases, 2 days
- v1.5: 7 plans, 5 phases, 1 day
- v2.0: 7 plans, 3 phases, 2 days

## Accumulated Context

### Decisions

- v1.0: Fork kind, addons as creation actions, on-by-default opt-out, go:embed for manifests
- v1.1: Astro + Starlight, kinder-site/ dir, dark-only mode, npm for CI
- v1.2: Kinder logo from modified kind robot, favicon.ico over SVG, llms.txt for GEO
- v1.3: common/ provider dedup, local registry addon, cert-manager addon, CLI diagnostic tools
- v1.4: Context in struct (not param), wave-based parallel (not DAG), sync.OnceValues for Nodes(), errgroup.SetLimit(3), flagpole/switch/json.NewEncoder pattern, CreateWithAddonProfile with 4 presets
- v1.5: Core/optional addon grouping, Symptom/Cause/Fix troubleshooting pattern, tutorial structure (overview/prerequisites/steps/cleanup), ci profile = MetricsServer + CertManager only
- v2.0: GoReleaser pipeline, goreleaser-action@v7, Homebrew cask via homebrew-kinder tap, NVIDIA GPU addon opt-in (boolValOptIn), NvidiaGPU package-level vars for test injection

### Pending Todos

None.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-03-06
Stopped at: null
Resume file: None
