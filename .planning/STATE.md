# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-03)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v1.3 Harden & Extend — Phase 19: Bug Fixes

## Current Position

Phase: 19 of 24 (Bug Fixes)
Plan: 1 of TBD in current phase
Status: In progress
Last activity: 2026-03-03 — Completed plan 01 (BUG-01, BUG-02 fixes)

Progress: [█░░░░░░░░░] ~4% (v1.3)

## Performance Metrics

**Velocity (v1.0):** 12 plans, 8 phases
**Velocity (v1.1):** 8 plans, 6 phases, 2 days
**Velocity (v1.2):** 4 phases, 1 day

**Phase 19, Plan 01:** 2 tasks, 8 files modified, ~25 min, 2026-03-03

## Accumulated Context

### Decisions

- v1.0: Fork kind, addons as creation actions, on-by-default opt-out, go:embed for manifests
- v1.1: Astro + Starlight, kinder-site/ dir, dark-only mode, npm for CI
- v1.2: Kinder logo from modified kind robot, favicon.ico over SVG, llms.txt for GEO
- v1.3: Extract shared provider code to common/, local registry as addon, cert-manager alongside Envoy Gateway
- v1.3 Phase 19-01: Release port listeners immediately in generatePortMappings loops (not deferred); return truncation error from extractTarball instead of silent break

### Pending Todos

None.

### Blockers/Concerns

- Phase 20 (Provider Deduplication): ProviderBehavior interface design (callback vs interface for port formatting/volume args) must be decided before writing any shared code — see SUMMARY.md Phase 2 research flag
- Phase 22 (Local Registry): Verify --network kind + container name DNS resolution works in Podman rootless before committing to implementation — see SUMMARY.md Phase 4 research flag
- Phase 23 (cert-manager): Confirm true/false default before phase begins — research recommends false (opt-in) to keep cluster creation fast; this is a product decision

## Session Continuity

Last session: 2026-03-03
Stopped at: Completed 19-01-PLAN.md (BUG-01 port leak, BUG-02 tar truncation)
Resume file: None
