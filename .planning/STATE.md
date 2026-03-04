# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-03)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 26 — Architecture (v1.4)

## Current Position

Phase: 26 of 29 (Architecture)
Plan: 2 of 2 in current phase — PHASE COMPLETE
Status: Phase 26 complete; ready for Phase 27
Last activity: 2026-03-04 — Plan 26-02 complete (CommandContext propagation + context-aware waitforready)

Progress: [██████████░░░░░░░░░░] 50% (v1.0-v1.3 complete; v1.4 phases 25-26 done)

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
- [Phase 25 Plan 01]: go directive settled at 1.24.0 (toolchain go1.26 enforces minimum; go 1.23 reverts on every tidy)
- [Phase 25 Plan 01]: rand.Int31() used in buildcontext.go (math/rand v1 has no Int32; v2 not yet adopted)
- [Phase 25 Plan 03]: pkg/internal/kindversion/ is the canonical location for all CLI version constants; Makefile linker -X flags and hack/release/create.sh VERSION_FILE both updated to this path
- [Phase 25 Plan 02]: golangci-lint v2 typecheck is always-active; remove from linters.enable or get fatal config error
- [Phase 25 Plan 02]: errcheck for deferred Close calls suppressed with //nolint:errcheck (cannot meaningfully handle in defer context)
- [Phase 25 Plan 02]: var-naming exclusions added for established package names (common, errors, log, version) to avoid mass rename refactor
- [Phase 25 Plan 04]: SHA-256 replaces SHA-1 for subnet generation; subnet values change but clusters are transient so no impact
- [Phase 25 Plan 04]: ErrNoNodeProviderDetected rename is technically breaking API but kinder is not consumed as a library
- [Phase 25 Plan 04]: Dashboard token at V(1); other dashboard output (header, URL, instructions) remains at V(0) for user visibility
- [Phase 26 Plan 01]: context.Background() at create.go call site — signal-wired context deferred to future phase
- [Phase 26 Plan 01]: AddonEntry defined in create.go (not action.go) to avoid import cycle risk
- [Phase 26 Plan 02]: Host-side exec.Command calls in installlocalregistry intentionally unchanged (not Node interface)
- [Phase 26 Plan 02]: tryUntil uses select on ctx.Done() with 500ms fallback for immediate cancellation without busy-loop

### Pending Todos

None.

### Blockers/Concerns

- [Phase 28 entry]: Validate MetalLB/EnvoyGateway runtime dependency empirically before parallelizing
- [Phase 28 entry]: Confirm cli.Status goroutine safety by reading pkg/internal/cli/status.go before Phase 28

## Session Continuity

Last session: 2026-03-04
Stopped at: Phase 26 complete (both plans done); ready for Phase 27 (Unit Tests)
Resume file: None
