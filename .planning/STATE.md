# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-03)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 28 — Parallel Addon Execution (v1.4)

## Current Position

Phase: 28 of 29 (Parallel Addon Execution)
Plan: 1 of 2 complete — Plan 02 next
Status: Phase 28 Plan 01 complete; sync.OnceValues cache + golang.org/x/sync dep added; ready for Plan 02
Last activity: 2026-03-03 — Plan 28-01 complete (race-free Nodes() cache with sync.OnceValues)

Progress: [█████████████░░░░░░░] 65% (v1.0-v1.3 complete; v1.4 phases 25-28 in progress)

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
- [Phase 27 Plan 01]: FakeNode.nextCmd returns &FakeCmd{} (not nil) when queue exhausted so callers can always call .Run() safely
- [Phase 27 Plan 01]: NewTestContext uses NoopLogger with StatusForLogger to avoid spinner setup complexity
- [Phase 27 Plan 01]: CommandContext accepts nil context pointer since FakeNode ignores it (test-only usage)
- [Phase 27 Plan 02]: errContains for certmanager wait-loop failures uses deployment name substring (e.g. cert-manager-cainjector); name is in the error via Wrapf
- [Phase 27 Plan 02]: dashboard success test feeds base64.StdEncoding.EncodeToString([]byte("test-token")) as FakeCmd.Output to exercise SetStdout/decode path
- [Phase 27 Plan 03]: TestExecute_InfoError always runs without Docker; Provider.Info() is called before any exec.Command invocations in localregistry.Execute()
- [Phase 27 Plan 03]: Docker-dependent tests use t.Skip guard via exec.Command("docker","version").Run() (sigs.k8s.io/kind/pkg/exec, not os/exec)
- [Phase 27 Plan 03]: FakeProvider does not implement fmt.Stringer so binaryName defaults to "docker"; aligns with Docker skip guard
- [Phase 28 Plan 01]: sync.OnceValues used over RWMutex-based cachedData: eliminates TOCTOU race, single-call guarantee
- [Phase 28 Plan 01]: golang.org/x/sync v0.19.0 added via go.mod edit + go mod download (not go get + go mod tidy; tidy removes unused deps before Plan 02 imports errgroup)

### Pending Todos

None.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-03-03
Stopped at: Phase 28 Plan 01 complete; sync.OnceValues + x/sync dep done; ready for Plan 02 (errgroup wave execution)
Resume file: None
