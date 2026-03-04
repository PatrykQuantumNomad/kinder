---
gsd_state_version: 1.0
milestone: v1.5
milestone_name: Website Use Cases & Documentation
status: executing
stopped_at: Completed 32-01 (CLI Reference plan 01)
last_updated: "2026-03-04T15:36:00Z"
last_activity: 2026-03-04 — Phase 32 plan 01 complete (profile comparison, JSON output, troubleshooting pages)
progress:
  total_phases: 5
  completed_phases: 3
  total_plans: 7
  completed_plans: 4
  percent: 57
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-04)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v1.5 — Phase 33: Tutorials

## Current Position

Phase: 32 of 34 (CLI Reference)
Plan: 1 of 1 in current phase
Status: Phase complete — ready for Phase 33
Last activity: 2026-03-04 — Phase 32 complete (profile comparison, JSON output, troubleshooting pages)

Progress: [████████░░] 80%

## Performance Metrics

**Velocity:**
- v1.0: 12 plans, 8 phases, 1 day
- v1.1: 8 plans, 6 phases, 2 days
- v1.2: 4 phases, 1 day
- v1.3: 8 plans, 6 phases, ~5 hours
- v1.4: 13 plans, 5 phases, 2 days

**v1.5 target:** 7 plans, 5 phases

## Accumulated Context

### Decisions

- v1.0: Fork kind, addons as creation actions, on-by-default opt-out, go:embed for manifests
- v1.1: Astro + Starlight, kinder-site/ dir, dark-only mode, npm for CI
- v1.2: Kinder logo from modified kind robot, favicon.ico over SVG, llms.txt for GEO
- v1.3: common/ provider dedup, local registry addon, cert-manager addon, CLI diagnostic tools
- v1.4: Context in struct (not param), wave-based parallel (not DAG), sync.OnceValues for Nodes(), errgroup.SetLimit(3), flagpole/switch/json.NewEncoder pattern, CreateWithAddonProfile with 4 presets
- [Phase 30-foundation-fixes]: Group addons as core (MetalLB, Metrics Server, CoreDNS) vs optional (Envoy Gateway, Headlamp, Local Registry, cert-manager) consistently across all three pages
- [Phase 30-foundation-fixes]: Sidebar groups Addons, Guides, CLI Reference are all collapsed by default
- [Phase 30-foundation-fixes]: Placeholder pages use Starlight Coming soon callout — minimal content, no stub sections
- [Phase 31-addon-page-depth]: Practical examples + Troubleshooting pattern established for addon docs: show working YAML with expected output, follow symptom/cause/fix format
- [Phase 31-addon-page-depth]: Troubleshooting sections use Symptom/Cause/Fix structure for scannable problem resolution
- [Phase 31-addon-page-depth]: cert-manager ClusterIssuer distinction emphasized via :::note callout as single most common error
- [Phase 32-cli-reference]: kinder env outputs a single JSON object (not array) — documented with :::note to prevent .[] misuse
- [Phase 32-cli-reference]: container-runtime check name (fallback) vs actual runtime name (docker/podman/nerdctl) distinction documented with detection order
- [Phase 32-cli-reference]: Commands without JSON support listed explicitly (get kubeconfig, create cluster, delete cluster)

### Pending Todos

None.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-03-04T15:36:00Z
Stopped at: Completed 32-01 (CLI Reference plan 01)
Resume file: None
