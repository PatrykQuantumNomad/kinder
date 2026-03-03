# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-03)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 25 — Foundation (v1.4)

## Current Position

Phase: 25 of 29 (Foundation)
Plan: 0 of ? in current phase
Status: Ready to plan
Last activity: 2026-03-03 — v1.4 roadmap created (phases 25-29)

Progress: [████████░░░░░░░░░░░░] 40% (v1.0-v1.3 complete; v1.4 not started)

## Performance Metrics

**Velocity:**
- v1.0: 12 plans, 8 phases, 1 day
- v1.1: 8 plans, 6 phases, 2 days
- v1.2: 4 phases, 1 day
- v1.3: 8 plans, 6 phases, ~5 hours
- v1.4: TBD

## Accumulated Context

### Decisions

- v1.0: Fork kind, addons as creation actions, on-by-default opt-out, go:embed for manifests
- v1.1: Astro + Starlight, kinder-site/ dir, dark-only mode, npm for CI
- v1.2: Kinder logo from modified kind robot, favicon.ico over SVG, llms.txt for GEO
- v1.3: common/ provider dedup, local registry addon, cert-manager addon, CLI diagnostic tools
- [v1.4 entry]: Context in struct (not function param) — deliberate trade-off for minimal call-site churn; document in code
- [v1.4 entry]: Wave-based parallel not full DAG — 7 addons with shallow deps; DAG adds 200+ lines for zero benefit
- [v1.4 entry]: Linker -X flags must be updated when version pkg moves to pkg/internal/kindversion/

### Pending Todos

None.

### Blockers/Concerns

- [Phase 26 entry]: Context in struct design decision must be confirmed before Phase 26 planning begins
- [Phase 28 entry]: Validate MetalLB/EnvoyGateway runtime dependency empirically before parallelizing
- [Phase 28 entry]: Confirm cli.Status goroutine safety by reading pkg/internal/cli/status.go before Phase 28

## Session Continuity

Last session: 2026-03-03
Stopped at: v1.4 roadmap created; ready to plan Phase 25
Resume file: None
