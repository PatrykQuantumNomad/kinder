# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-03)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v1.3 Harden & Extend — Phase 24: CLI Commands (complete)

## Current Position

Phase: 24 of 24 (CLI Commands — COMPLETE)
Plan: 1 of 1 in current phase (phase complete)
Status: Phase 24 complete — v1.3 milestone complete
Last activity: 2026-03-03 — Phase 24 Plan 01 complete (kinder env + kinder doctor CLI commands, Provider.Name())

Progress: [██████████] 100% (v1.3 — 6/6 phases complete, phase 24 1/1 done)

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
**Phase 23, Plan 01:** 2 tasks, 2 created + 1 modified, ~10 min, 2026-03-03
**Phase 24, Plan 01:** 2 tasks, 2 created + 2 modified, ~15 min, 2026-03-03

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
- v1.3 Phase 23-01: cert-manager v1.16.3 (v1.17.6 not yet released); --server-side apply required for 986KB manifest; webhook readiness gate — wait for all 3 deployments (300s) before applying ClusterIssuer to prevent "no endpoints available" errors
- v1.3 Phase 24-01: Provider.Name() uses fmt.Stringer type assertion; kinder env reads env vars first then DetectNodeProvider() (zero runtime calls); os.Exit(1|2) in RunE for structured exit codes (Cobra always exits 1 for non-nil error); output keys: KINDER_PROVIDER, KIND_CLUSTER_NAME, KUBECONFIG

### Pending Todos

None.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-03-03
Stopped at: Phase 24 Plan 01 complete — v1.3 milestone complete (kinder env + kinder doctor)
Resume file: None
