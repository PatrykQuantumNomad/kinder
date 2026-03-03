# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-03)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v1.3 Harden & Extend — Phase 22: Local Registry Addon

## Current Position

Phase: 22 of 24 (Local Registry Addon)
Plan: 1 of 1 in current phase (phase complete)
Status: Phase 22 complete
Last activity: 2026-03-03 — Phase 22 Plan 01 complete (local registry action + create.go wiring)

Progress: [█████░░░░░] 33% (v1.3 — 4/6 phases complete, phase 22 1/1 done)

## Performance Metrics

**Velocity (v1.0):** 12 plans, 8 phases
**Velocity (v1.1):** 8 plans, 6 phases, 2 days
**Velocity (v1.2):** 4 phases, 1 day

**Phase 19, Plan 01:** 2 tasks, 8 files modified, ~25 min, 2026-03-03
**Phase 19, Plan 02:** 2 tasks, 4 files modified, ~4 min, 2026-03-03
**Phase 20, Plan 01:** 2 tasks, 6 files modified + 3 deleted, ~15 min, 2026-03-03
**Phase 20, Plan 02:** 2 tasks, 6 created + 4 deleted, ~15 min, 2026-03-03
**Phase 21, Plan 01:** 2 tasks, 5 modified + 1 created, ~10 min, 2026-03-03
**Phase 22, Plan 01:** 2 tasks, 2 created + 1 modified, ~2 min, 2026-03-03

## Accumulated Context

### Decisions

- v1.0: Fork kind, addons as creation actions, on-by-default opt-out, go:embed for manifests
- v1.1: Astro + Starlight, kinder-site/ dir, dark-only mode, npm for CI
- v1.2: Kinder logo from modified kind robot, favicon.ico over SVG, llms.txt for GEO
- v1.3: Extract shared provider code to common/, local registry as addon, cert-manager alongside Envoy Gateway
- v1.3 Phase 19-01: Release port listeners immediately in generatePortMappings loops (not deferred); return truncation error from extractTarball instead of silent break
- v1.3 Phase 19-02: All Provider methods accepting cluster name must wrap with defaultName(); sort.Slice primary/secondary key comparators must use != guard (not >-only) for strict weak ordering
- v1.3 Phase 20-01: Use exported Node struct with BinaryName string field (not interface/callback) for provider dispatch; keep nodeCmd unexported; go.mod to go 1.21.0 with toolchain go1.26.0
- v1.3 Phase 20-02: CreateContainer takes binaryName as first parameter to support both docker and nerdctl from a single common function; podman keeps its own generatePortMappings (lowercase protocol, :0 strip); docker and nerdctl provision.go deleted in favour of create.go files calling common helpers
- v1.3 Phase 21-01: LocalRegistry and CertManager both default to true (on-by-default opt-out, consistent with existing addon pattern); plain bool in internal types, *bool in v1alpha4 public API (matching MetalLB/Dashboard pattern)
- v1.3 Phase 22-01: registry:2 (not :3); ContainerdConfigPatches injected in create.go before p.Provision() (cannot be done post-provisioning); ALL nodes patched with hosts.toml (not just control-plane); Podman rootless warn-and-continue; idempotent container ops via inspect-before-create/connect

### Pending Todos

None.

### Blockers/Concerns

- Phase 23 (cert-manager): Confirm true/false default before phase begins — research recommends false (opt-in) to keep cluster creation fast; this is a product decision

## Session Continuity

Last session: 2026-03-03
Stopped at: Phase 22 Plan 01 complete — local registry addon phase done
Resume file: None
