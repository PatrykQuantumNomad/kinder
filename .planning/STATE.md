# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-03)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 29 — CLI Features (v1.4)

## Current Position

Phase: 29 of 29 (CLI Features) — COMPLETE
Plan: 2 of 2 complete — Phase 29 done
Status: Phase 29 complete; JSON output for all read commands + addon profile presets implemented; v1.4 milestone complete
Last activity: 2026-03-04 — Plan 29-02 complete (--output json for get nodes, --profile flag for create cluster, CreateWithAddonProfile)

Progress: [████████████████████] 100% (v1.0-v1.4 all phases complete)

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
- [Phase 28 Plan 02]: errgroup.WithContext + SetLimit(3) for Wave 1 parallel addon dispatch; Wave 2 (EnvoyGateway) sequential after g.Wait()
- [Phase 28 Plan 02]: parallelActionContext() creates per-goroutine ActionContext with no-op Status to avoid cli.Status.status write race
- [Phase 28 Plan 02]: wave1Results pre-allocated by index (not append) for deterministic summary ordering from concurrent goroutines
- [Phase 29 Plan 01]: Format validation switch before output branch: validate flags.Output early and return error for unknown formats, then branch on json vs default
- [Phase 29 Plan 01]: doctor checkResult struct exported with JSON tags: allows clean JSON serialization separate from internal result struct
- [Phase 29 Plan 01]: nil slice initialized to empty for clusters JSON: avoids JSON null output, emits [] for zero clusters
- [Phase 29 Plan 01]: hasFail/hasWarn computed before output branch in doctor: exit codes apply regardless of output format
- [Phase 29 Plan 02]: JSON branch fires before human-readable empty-node checks so --output json always returns valid JSON array (empty or populated)
- [Phase 29 Plan 02]: CreateWithAddonProfile nil-guards o.Config by loading default config; avoids nil dereference when no --config flag given
- [Phase 29 Plan 02]: --profile wired after withConfig in provider.Create() so profile addons override any config-file addon settings
- [Phase 29 Plan 02]: Empty --profile is a strict no-op; default cluster creation behavior is fully preserved

### Pending Todos

None.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-03-04
Stopped at: Phase 29 Plan 02 complete; JSON output for get nodes + addon profile presets done; Phase 29 complete; v1.4 milestone complete
Resume file: None
