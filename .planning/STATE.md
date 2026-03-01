# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-01)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 2 — MetalLB

## Current Position

Phase: 2 of 7 (MetalLB)
Plan: 1 of 2 in current phase
Status: In progress
Last activity: 2026-03-01 — Completed 02-01: subnet detection and IP pool carving (15 tests, all pass).

Progress: [███░░░░░░░] 21%

## Performance Metrics

**Velocity:**
- Total plans completed: 3
- Average duration: 2 minutes
- Total execution time: 0.11 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation | 2 | 4 min | 2 min |
| 02-metallb | 1 | 2 min | 2 min |

**Recent Trend:**
- Last 5 plans: 01-01 (2m), 01-02 (2m), 02-01 (2m)
- Trend: Stable

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Research]: Use Headlamp v0.40.1 instead of kubernetes/dashboard (archived Jan 2026, Helm dependency)
- [Research]: Embed all addon manifests at build time using go:embed (offline-capable, no external tools)
- [Research]: MetalLB must be fully ready before Envoy Gateway actions run (hard ordering dependency)
- [Research]: CoreDNS patching uses read-modify-write in Go, not kubectl patch (Corefile is a string blob)
- [01-01]: *bool for v1alpha4 Addons fields — nil means not-set, defaults to true; avoids Go bool zero-value ambiguity after YAML decode
- [01-01]: Internal config uses plain bool — conversion is the single point where nil *bool => true translation happens
- [01-01]: Binary renamed via Makefile + cobra Use field only — module path stays sigs.k8s.io/kind
- [01-02]: Addon loop uses a closure (runAddon) rather than a top-level function — keeps actionsContext and addonResults in natural scope
- [01-02]: Platform warning fires after all addon runs, before summary — groups warning visually with addon results
- [01-02]: Salutation updated "kind" to "kinder" — URLs left pointing to kind docs until kinder docs exist
- [02-01]: carvePoolFromSubnet uses broadcast-address arithmetic — computes broadcast, then sets last octet to .200-.250; handles /16, /24, /20 automatically
- [02-01]: parseSubnetFromJSON branches on providerName=="podman" only — Docker and Nerdctl share IPAM.Config schema so no third branch needed

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 4]: CoreDNS Corefile merge-patch strategy needs validation — string blob in ConfigMap, not structured YAML
- [Phase 5]: Confirm standard vs. experimental Gateway API CRD channel; measure binary size of ~3,000-line Envoy Gateway manifest
- [Phase 6]: Headlamp v0.40.1 static manifest URLs need verification before embedding
- [Phase 2]: Podman rootless MetalLB viability (L2 speaker + raw sockets) needs testing during implementation

## Session Continuity

Last session: 2026-03-01
Stopped at: Completed 02-01-PLAN.md — subnet detection and IP pool carving (subnet.go + subnet_test.go, 15 tests all pass). Ready for 02-02.
Resume file: None
