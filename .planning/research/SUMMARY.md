# Project Research Summary

**Project:** kinder — Milestone 3 (code quality, testing, parallel addons, JSON output, addon registry)
**Domain:** Go CLI tool — Kubernetes local development environment (kind fork)
**Researched:** 2026-03-03
**Confidence:** HIGH

## Executive Summary

kinder is a fork of `sigs.k8s.io/kind` that extends the base kind CLI with 7 additional addons (MetalLB, Envoy Gateway, cert-manager, local registry, metrics server, dashboard, CoreDNS tuning) and enhanced cluster management. Milestone 3 is a code quality and capability milestone rather than a net-new product build. Experts build this type of incremental improvement by working inside-out: shore up test coverage and fix architectural violations before adding new user-facing features, because the safest order for this specific codebase is unit tests first, then JSON output, then cluster presets, then parallel addon installation, and finally the addon registry pattern (if pursued at all).

The recommended approach has minimal new dependencies. A single new external package (`golang.org/x/sync v0.19.0` for `errgroup`) and a Go minimum version bump from 1.21 to 1.23 are the only go.mod changes needed. All other improvements — context propagation, JSON output, addon registry, unit tests — use the standard library plus existing in-repo interfaces. The architecture has one hard layer violation (`pkg/cluster/provider.go` importing from the CLI layer `pkg/cmd/kind/version`) which must be fixed by moving version constants to a new `pkg/internal/kindversion/` package before any other refactoring touches `provider.go`.

The key risks are: (1) context propagation done wrong by embedding `context.Context` in `ActionContext` instead of passing it as a function parameter — the official Go guidance prohibits struct embedding; (2) naive full parallelism of addons creating timing-dependent race conditions between MetalLB and Envoy Gateway; (3) JSON output contaminated by logger output on stdout. Each risk has a clear prevention strategy. None requires a heroic recovery path — they are avoidable with upfront design choices and the right implementation order.

---

## Key Findings

### Recommended Stack

The go.mod needs two targeted changes: bump the `go` directive from `1.21.0` to `1.23.0` (aligning with `hack/tools/go.mod` which already requires 1.23, and unlocking `slices`/`maps` stdlib helpers), and add `golang.org/x/sync v0.19.0` as the sole new direct dependency. The existing `golang.org/x/sys v0.6.0` (indirect) should be updated to `v0.41.0` for security hygiene — 35 point releases accumulated over 3 years. The `golangci-lint` in `hack/tools/go.mod` should be updated from `v1.62.2` to `v2.10.1` which requires running `golangci-lint migrate` to convert the config format and removing the retired `exportloopref` linter.

**Core technologies:**
- `go 1.23.0` minimum — aligns with `hack/tools`, unlocks `slices.Contains`/`maps.Keys`, enables rand cleanup; do NOT go to 1.24 or 1.25 (no features required)
- `golang.org/x/sync/errgroup v0.19.0` — only new external dependency; provides bounded parallel goroutines with error collection; `errgroup.SetLimit(3)` prevents flooding the API server
- `encoding/json` (stdlib) — all JSON output; no third-party output library needed
- `context` (stdlib) — context propagation through existing `CommandContext` infrastructure already present in `exec/local.go` and `common/node.go`
- `testing` (stdlib) — unit tests; no `testify` to preserve the zero-external-test-dep posture already established in the codebase
- `golangci-lint v2.10.1` — dev tooling only; requires config migration; not a runtime dependency

**Explicit exclusions (do not add):**
- `github.com/stretchr/testify` in main module — existing tests use zero external test deps; table-driven stdlib `testing` is sufficient
- `gomock`/`mockery` — the pure function extraction pattern makes mocking unnecessary at unit test level
- `go-pretty`/`tablewriter` for JSON — `encoding/json` with `SetIndent` is the entire solution
- Go `plugin.Open` plugin system — requires CGO, incompatible with static builds
- `sync.WaitGroup` alone for parallel addons — no built-in error propagation; use `errgroup` instead

### Expected Features

**Must have (table stakes):**
- JSON output for `kinder env` and `kinder doctor` — CI scripts pipe to `jq`; this is the kubectl convention; lowest complexity, highest CI value
- Unit tests for addon `Execute()` paths — prerequisite for the parallel refactor; without these, any concurrent change is unverifiable
- Cluster presets via `--profile` flag (minimal, full, gateway, ci) — removes the requirement to write a full YAML config for the 90% use case; ~50 lines of code, zero architecture change

**Should have (competitive):**
- Wave-based parallel addon installation — cuts cluster creation time by 40-50 seconds by running independent addons concurrently; no existing kind-based tool does this; requires unit tests in place first
- JSON output for `kinder get clusters` and `kinder get nodes` — lower priority than env/doctor but completes the machine-readable surface; uses identical pattern
- Addon registry pattern (centralized `[]AddonEntry` slice in `create.go`) — reduces a new-addon addition from 4-place edit to 2-place; not full self-registration (which requires a config API breaking change)

**Defer (v2+):**
- Full addon self-registration via `init()` — requires changing the config API `Addons` struct from named bool fields to `map[string]bool` which is a breaking YAML format change; only valuable when addon count exceeds ~10
- `--addons` CLI flag (comma-separated override) — complement to presets; depends on registry pattern being settled first
- Plugin system for external addons — requires CGO, ABI stability, versioning; fundamentally incompatible with static builds
- YAML output (`--output=yaml`) — adds gopkg.in/yaml.v2 dependency for minimal benefit; `jq` + `yq` covers the use case
- Async (non-blocking) addon install — MetalLB must be ready before Envoy Gateway needs LoadBalancer IPs; async install violates dependencies silently

### Architecture Approach

The codebase has one clear structural violation and six well-scoped improvements. The violation is `pkg/cluster/provider.go` importing from `pkg/cmd/kind/version/` — the library layer must not depend on the CLI layer. The fix is a new `pkg/internal/kindversion/` package that receives the version functions, with the CLI version package becoming a thin re-export wrapper. The six improvements are: version package move (fix), context propagation (ActionContext gets `context.Context`), addon registry (centralized slice), parallel execution (`errgroup`-based wave execution), JSON output (post-execution summary struct), and unit tests (pure function extraction + `fakeNode{}` pattern). Each step is independently buildable and verifiable.

**Major components:**
1. `pkg/internal/kindversion/` (NEW) — version string logic moved from CLI layer; resolves the layer violation; must be the first change committed
2. `pkg/cluster/internal/create/actions/addon_registry.go` (NEW) — `AddonEntry` type + `Registry()` function; replaces 7 hard-coded `runAddon()` calls; lives in the `actions/` package to own addon imports without creating a cycle
3. `pkg/cluster/internal/create/create.go` (MODIFIED) — accepts `context.Context`, uses registry loop, goroutine fan-out for parallel phase, JSON output branch at end
4. `pkg/cluster/internal/create/actions/action.go` (MODIFIED) — adds `Context context.Context` field to `ActionContext` struct (pragmatic minimal-churn approach; see gap note on context design decision)
5. All addon `Execute()` methods (MODIFIED) — replace `node.Command(...)` with `node.CommandContext(ctx.Context, ...)` using existing `CommandContext` infrastructure already in `exec/local.go`
6. New `_test.go` files for `installenvoygw`, `installlocalregistry`, `installcertmanager`, `installmetricsserver`, `installdashboard` — pure function extraction pattern matching existing `corefile_test.go` and `subnet_test.go`

**Patterns to follow:**
- Extract-and-test: split `Execute()` into (a) pure logic with no I/O and (b) kubectl orchestration; test (a) directly; `Execute()` itself is an integration concern
- `AddonEntry` registry: a `[]AddonEntry` slice in `create.go` (or `addon_registry.go` in same package) replaces hard-coded `runAddon` calls; adding an addon = add one entry
- `CommandContext` over `Command`: all blocking kubectl calls in addon `Execute()` must use `node.CommandContext(ctx.Context, ...)`, not `node.Command(...)`

**Anti-patterns to avoid:**
- Testing `Execute()` directly without a real cluster — results in nil pointer panics or tests leaking into real clusters
- Sharing `cli.Status` across goroutines — not concurrency-safe; use `ctx.Logger` in parallel addon code
- Removing dependency order comments when adding the registry — ordering represents implicit runtime dependencies that future maintainers must understand
- Exposing `context.Context` in the public API prematurely (`provider.go:Create()` should pass `context.Background()` internally; a future `CreateWithContext` is the additive path)

### Critical Pitfalls

1. **Context in struct vs. function parameter** — architecture recommends adding `Context context.Context` to `ActionContext` struct for minimal call-site churn; official Go guidance ("Do not store Contexts inside a struct type") recommends against it. Decision required before Phase 2 begins. The struct-embedding approach is acceptable if documented as a deliberate trade-off. Never use `context.Background()` or `context.TODO()` as placeholders inside `Execute()` methods after the migration.

2. **Naive full addon parallelism breaks MetalLB/EnvoyGateway ordering** — EnvoyGateway depends on MetalLB's `IPAddressPool` at runtime; `ActionContext.Nodes()` has a TOCTOU race (check-then-set, not atomic — fix with `sync.Once`); `cli.Status` spinner is not goroutine-safe. Prevention: document the dependency DAG before writing any goroutine, implement wave-based groups (not full parallelism), run `go test -race ./...` in CI.

3. **JSON stdout contamination** — when `--output json` is active, logger output must route to `os.Stderr` not `os.Stdout`; encoding items in a loop produces JSON Lines, not a JSON array (encode the full slice once); spinner carriage returns corrupt JSON. Prevention: enforce logger-to-stderr at the command entry point, write a golden-file test piping through `jq empty`, define JSON struct with tags before implementation.

4. **Import cycle in registry refactoring** — if `addon_registry.go` is placed in a new sub-package of `actions/`, it imports each addon package, which each import the `actions/` package for `ActionContext`, creating a cycle. Prevention: keep the registry in `pkg/cluster/internal/create/actions/addon_registry.go` (same package as `action.go`).

5. **go.mod bump leaving comment-gated dead code** — `create.go` has a comment noting that `rand.New(rand.NewSource(...))` can be simplified once the minimum version reaches 1.20+. After the 1.23 bump this code must be cleaned up in the same commit. Run `grep -rn "rand.NewSource\|1.17\|1.20" pkg/` after the bump.

---

## Implications for Roadmap

The implementation order is dictated by compile-time dependencies and safety constraints between improvements. Each phase is independently verifiable with `go build ./... && go test ./...`.

### Phase 1: Foundation — go.mod, Dependencies, Layer Violation Fix
**Rationale:** The layer violation in `provider.go` and the dependency updates are non-behavioral changes that must precede any architectural work. Doing them first creates a clean baseline. Combining a package move with a behavioral change in one commit is a bisect nightmare — never do it.
**Delivers:** Clean dependency baseline; `pkg/internal/kindversion/` package; `provider.go` no longer imports CLI layer; `rand.Intn()` cleanup after 1.23 bump; updated `golang.org/x/sys v0.41.0`; `golangci-lint v2.10.1` with migrated config
**Addresses:** go.mod minimum version alignment; security hygiene for indirect deps; layer violation fix
**Avoids:** Pitfall 1 (go.mod bump with leftover comment-gated code), Pitfall 2 (dependency cascade — update one dep at a time, not `go get -u ./...`), Pitfall 8 (package move breaks upstream sync — update `-X` linker flags for `pkg/internal/kindversion.*` vars immediately)

### Phase 2: Architecture — context.Context + Addon Registry
**Rationale:** Context propagation and the addon registry are structural changes to `ActionContext` and `create.go` that all subsequent phases depend on. Context must exist before parallel execution. The registry must exist before the parallel loop can be written. These two are independent of each other but both must precede Phases 3 and 4.
**Delivers:** `ActionContext` with `Context context.Context` field; `go get golang.org/x/sync@v0.19.0`; `addon_registry.go` with `AddonEntry` type and `Registry()` function; `create.go` using registry loop; all addon `Execute()` methods using `node.CommandContext(ctx.Context, ...)`; `waitforready.go` with context-aware `tryUntil` loop
**Uses:** `golang.org/x/sync/errgroup` (added to go.mod); standard library `context`
**Implements:** Addon registry component; context propagation data flow
**Avoids:** Pitfall 3 (context design decision must be made at Phase 2 entry, not discovered mid-phase), Pitfall 5 (import cycle — keep registry in `actions/` package, not a sub-package)

### Phase 3: Unit Tests for Addon Actions
**Rationale:** Unit tests must be in place before parallel execution is introduced. Tests are the only verification that the sequential-to-concurrent refactor introduces no regression. The `fakeNode{}` + `fakeCmd{}` infrastructure should be built once in a `testutil` package and reused across all addon test files.
**Delivers:** `testutil` package with `FakeNode`, `FakeCmd`; test files for `installenvoygw`, `installlocalregistry`, `installcertmanager`, `installmetricsserver`, `installdashboard`; pure function extraction from addon `Execute()` methods where logic is non-trivial; `go test ./pkg/cluster/internal/create/actions/...` passes without `$KUBECONFIG` set
**Addresses:** Unit test coverage for addon Execute paths (P1 feature)
**Avoids:** Pitfall 4 (fakeNode not catching real bugs — `FakeCmd` must verify `SetStdin` was called for `kubectl apply` commands; tests must not leak into real clusters)

### Phase 4: Parallel Addon Installation
**Rationale:** Depends on Phase 2 (registry + context) and Phase 3 (unit tests). The dependency DAG must be documented before any goroutine is written. The `sync.Once` fix for `ActionContext.Nodes()` cache TOCTOU race must be applied before goroutines are introduced.
**Delivers:** Wave-based parallel addon execution in `create.go` using `errgroup`; `sync.Once` nodes cache fix; `go test -race ./...` clean; 40-50 second reduction in full cluster creation time; `errgroup.SetLimit(3)` bounding concurrency; `Status.Start/End` removed from addon Execute methods in favor of `ctx.Logger`
**Uses:** `golang.org/x/sync/errgroup` with `SetLimit`
**Avoids:** Pitfall 6 (race conditions — document dependency DAG first, implement wave groups, then run `-race`; never run `go test -race` only after the feature is "done")

### Phase 5: User-Facing Features — JSON Output + Cluster Presets
**Rationale:** JSON output and cluster presets are independent of the architectural work (Phases 1-4) and could technically be done earlier, but shipping them after the architecture is solid reduces rework risk. JSON output for `kinder create cluster` depends on `addonResults` being available from Phase 4.
**Delivers:** `--output json` flag on `kinder get clusters`, `kinder get nodes`, `kinder env`, `kinder doctor`; `--output json` on `kinder create cluster` with per-addon result JSON; `--profile` flag with `minimal`, `full`, `gateway`, `ci` presets defined in `pkg/cluster/presets.go`; golden-file tests verifying `--output json | jq empty` exits 0; logger routing to `os.Stderr` enforced in JSON mode
**Addresses:** JSON output for env/doctor (P1 feature), cluster presets (P1 feature), JSON for get commands (P2 feature)
**Avoids:** Pitfall 7 (stdout contamination — stream routing rule enforced at command entry point; encode full array once, not per-item in loop)

### Phase Ordering Rationale

- Phase 1 before everything else: the layer violation and dependency updates create a clean merge point; combining behavioral changes with package moves is a bisect nightmare
- Phase 2 (architecture) before Phase 3 (tests): the `CommandContext` call signature change touches every addon file; tests written against the old signature would need immediate rewriting
- Phase 3 (tests) before Phase 4 (parallel): parallel execution without test coverage is unverifiable for regression; the `sync.Once` nodes cache fix is easiest to validate with test infrastructure in place
- Phase 5 (features) last: JSON output for `kinder create cluster` needs `addonResults` from Phase 4; presets are independent but ship cleanly alongside JSON as a DX-focused release increment

### Research Flags

Phases likely needing deeper research during planning:

- **Phase 2 (context design):** The struct-embedding vs. function-parameter choice is explicitly in tension between the architecture doc (recommends struct for minimal churn) and pitfalls doc (warns against it per official Go guidance). This is a team decision that must be made before Phase 2 planning begins to avoid mid-phase rework.
- **Phase 4 (MetalLB/EnvoyGateway runtime dependency):** The research documents that EnvoyGateway depends on MetalLB at runtime (not install time), but this was not empirically verified. Before parallelizing, validate: create a cluster with MetalLB disabled and EnvoyGateway enabled and confirm whether the Gateway resource gets a LoadBalancer IP.
- **Phase 4 (`cli.Status` goroutine safety):** PITFALLS.md identifies `Status.Start/End` as not goroutine-safe based on code analysis. Verify directly by reading `pkg/internal/cli/status.go` before Phase 4 begins.

Phases with standard patterns (skip research-phase):

- **Phase 1:** All changes are mechanical — version bumps, one package move, grep for dead code. Well-documented in Go toolchain docs.
- **Phase 3:** fakeNode pattern is standard Go; existing `corefile_test.go` and `subnet_test.go` are the direct templates.
- **Phase 5:** JSON output (~50 lines per command) and presets (~50 lines total) are proven patterns from kubectl and GitHub CLI. No architectural unknowns.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All versions verified against official package registries; go.mod and hack/tools/go.mod read directly from codebase; errgroup API confirmed at pkg.go.dev; x/sys compatibility with go 1.23 confirmed |
| Features | HIGH | Codebase analyzed directly; competitor tools (minikube, k3d, skaffold, helmfile) checked; feature prioritization from first-party code analysis; safest order validated against compile-time dependency graph |
| Architecture | HIGH | Every integration point identified from direct file reads; existing `CommandContext` infrastructure confirmed present in `exec/local.go` and `common/node.go`; import cycle risk assessed; 7-step build order specified with verification gates |
| Pitfalls | HIGH | Critical pitfalls sourced from official Go docs (context-and-structs blog), Go toolchain directive docs, cert-manager webhook docs; TOCTOU race in `ActionContext.Nodes()` identified from direct code read of `action.go` |

**Overall confidence:** HIGH

### Gaps to Address

- **Context in struct decision:** The architecture research (pragmatic: struct embedding) and pitfalls research (principled: no context in structs) disagree. This is a team decision, not a research gap — but it must be resolved before Phase 2 planning begins.
- **EnvoyGateway/MetalLB runtime dependency:** Documented as a dependency assumption from code reading; not empirically verified. Validate manually before Phase 4 parallelizes these two addons.
- **`cli.Status` goroutine safety:** Identified as not goroutine-safe from behavioral analysis. Confirm by reading `pkg/internal/cli/status.go` directly before Phase 4.
- **golangci-lint v2 config migration:** STACK.md rates this MEDIUM confidence because the config migration requires validation after running `golangci-lint migrate`. Budget time for this in Phase 1 rather than discovering issues mid-phase.
- **Build system `-X` linker flags:** Moving version vars to `pkg/internal/kindversion/` requires updating `-X` linker flags in `Makefile`/`hack/build.sh`. Inventory all `-X` flags before the move to avoid producing a binary where `kinder version` shows empty version strings.

---

## Sources

### Primary (HIGH confidence)
- Kinder codebase direct analysis — `create.go`, `action.go`, `exec/types.go`, `exec/local.go`, `common/node.go`, `provider.go`, `go.mod`, `hack/tools/go.mod`, all addon `*_test.go` files
- Go 1.23 release notes: https://go.dev/doc/go1.23
- Go 1.24 release notes (T.Context, rand cleanup): https://go.dev/doc/go1.24
- Go 1.25 release notes: https://go.dev/doc/go1.25
- golang.org/x/sys v0.41.0: https://pkg.go.dev/golang.org/x/sys?tab=versions
- golang.org/x/sync v0.19.0 + errgroup API: https://pkg.go.dev/golang.org/x/sync/errgroup
- Go toolchains reference: https://go.dev/doc/toolchain
- Contexts and structs — official Go blog: https://go.dev/blog/context-and-structs
- Go data race detector: https://go.dev/doc/articles/race_detector
- golangci-lint v2.10.1 releases: https://github.com/golangci/golangci-lint/releases
- golangci-lint v2 migration guide: https://golangci-lint.run/docs/product/migration-guide/
- cert-manager webhook readiness: https://cert-manager.io/docs/concepts/webhook/
- k8s.io/utils/exec/testing FakeExec (pattern reference): https://pkg.go.dev/k8s.io/utils/exec/testing
- rand auto-seed since Go 1.20: https://go.dev/blog/randv2
- x/sys backward-incompatible change v0.23.0: https://github.com/golang/go/issues/68766

### Secondary (MEDIUM confidence)
- minikube addon system architecture (DeepWiki) — sequential execution confirmed; no parallel addon install
- minikube profile system (DeepWiki) — file-based profiles, not addon presets
- k3d configuration system (DeepWiki) — no named presets confirmed
- helmfile dependency ordering (DeepWiki) — DAG + wave pattern for parallel execution
- Skaffold profiles documentation (official) — overlay pattern
- Go import cycles and solutions (Jogendra blog)
- cmd/go Issue #62409 — real-world CI breakage from toolchain directive after update
- Go context library anti-patterns (Medium) — context retrofitting patterns

---
*Research completed: 2026-03-03*
*Ready for roadmap: yes*
