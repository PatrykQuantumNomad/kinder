# Technology Stack

**Project:** kinder — Milestone 3 (Code quality, testing, parallel addons, JSON output, addon registry)
**Scope:** Stack additions/changes for: go.mod version bump, golang.org/x/sys update, context.Context support, unit tests for addon actions, parallel addon installation, structured JSON output, addon registry pattern.
**Researched:** 2026-03-03
**Overall confidence:** HIGH for all items below (verified against official sources and live package versions)

---

## Executive Summary: What Actually Changes in go.mod

**New external dependency: ONE** — `golang.org/x/sync` for `errgroup` (parallel addon install).

The `golang.org/x/sys` version update is a maintenance upgrade (v0.6.0 → v0.41.0). The `go` minimum version directive bumps from `go 1.21.0` to `go 1.23.0` (already used by the `hack/tools` module; aligns with the build toolchain's effective minimum). The `toolchain` directive is already present at `go1.26.0` and stays unchanged.

All other improvements — context support, unit tests, JSON output, addon registry — use only the standard library plus existing in-repo packages.

---

## Recommended Stack Changes

### 1. Go Module Minimum Version — go 1.21.0 → go 1.23.0

| Item | Current | Recommended |
|------|---------|-------------|
| `go` directive | `go 1.21.0` | `go 1.23.0` |
| `toolchain` directive | `toolchain go1.26.0` | `toolchain go1.26.0` (unchanged) |

**Why `go 1.23.0` (not 1.24 or 1.25):**

- The `hack/tools` module (`hack/tools/go.mod`) already declares `go 1.23`. Bumping the main module to 1.23 brings it into alignment without adding friction.
- Go 1.23 introduced `iter` package (range-over-func), but more relevantly for this milestone it stabilised iterator-based APIs in `slices`/`maps` that are used in common helpers. More practically, 1.23 is the LTS-stable choice with no known regressions relevant to CLI tools as of 2026-03.
- Go 1.24 (released February 2025) adds `math/rand.Seed()` as a no-op, which finalises the rand.Intn auto-seeding cleanup already flagged in the codebase comment. However, the rand simplification in `logSalutation()` only requires 1.20+ (global auto-seed), so the 1.23 bump already enables that cleanup.
- Go 1.24 also adds `testing.T.Context()` — useful for test context management — but it is NOT required for the unit tests in scope (which test pure functions like `patchCorefile`, `carvePoolFromSubnet`, and manifest generation).
- Go 1.25 (released August 2025) adds `sync.WaitGroup.Go()` and `testing/synctest` — neither is required here.
- **Decision: go 1.23.0** — minimum useful bump, aligned with hack/tools, avoids pulling in unnecessary Go version constraints on downstream consumers.

**Unlocked by the bump:**

| Feature | Since | Use in this milestone |
|---------|-------|----------------------|
| `rand.Intn()` without explicit seeding | 1.20 | Replace `rand.New(rand.NewSource(...))` in `logSalutation()` |
| `slices.Contains`, `slices.Sort` | 1.21 | Cleaner addon name deduplication in registry |
| `maps.Keys` | 1.21 | Addon registry key enumeration |
| `iter` package | 1.23 | Available if needed for addon iteration patterns |

**go.mod diff:**

```diff
-go 1.21.0
+go 1.23.0

 toolchain go1.26.0
```

**Confidence:** HIGH — verified against Go release notes and hack/tools/go.mod in this repo.

---

### 2. golang.org/x/sys — v0.6.0 → v0.41.0

| Item | Current | Recommended |
|------|---------|-------------|
| `golang.org/x/sys` | `v0.6.0` (indirect) | `v0.41.0` (indirect) |

**Why this update is safe:**

- `golang.org/x/sys` is an `// indirect` dependency in the main `go.mod`. Nothing in the kinder codebase imports it directly — it is pulled in transitively by `github.com/mattn/go-isatty v0.0.20`.
- The one known backward-incompatible change in the v0.6→v0.41 range is in `v0.23.0`: removal of Linux-specific `ETHTOOL_FLAG_*` constants that track kernel-6.10 enum promotions. Kinder does not use the `unix` or `windows` subpackages of `x/sys` directly, so this does not affect it.
- `go-isatty v0.0.20` declares its own minimum `x/sys` requirement; `go mod tidy` will resolve the correct minimum. Running `go get golang.org/x/sys@v0.41.0 && go mod tidy` is the correct update path.
- Staying on `v0.6.0` is a security hygiene risk — 35 point releases of a low-level OS interaction package have accumulated over 3 years.

**Update command:**

```bash
go get golang.org/x/sys@v0.41.0
go mod tidy
```

**Confidence:** HIGH — version confirmed via https://pkg.go.dev/golang.org/x/sys?tab=versions (v0.41.0 released 2026-02-08). Breaking change risk assessed from https://github.com/golang/go/issues/68766.

---

### 3. golang.org/x/sync — NEW DEPENDENCY (v0.19.0)

| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `golang.org/x/sync/errgroup` | `v0.19.0` | Parallel addon installation with error propagation | Standard Go extended-library approach for bounded concurrent goroutines with error collection |

**Why `errgroup` and not `sync.WaitGroup`:**

- `errgroup.Group` collects the first non-nil error and cancels sibling goroutines via a shared `context.Context`. This is exactly what parallel addon install needs: if one addon fails, others should still complete (warn-and-continue semantics), but the error must propagate.
- `errgroup.SetLimit(n)` provides bounded concurrency — important because all addons call `kubectl` commands on the same control-plane node, and unlimited parallelism would flood the node's API server.
- `sync.WaitGroup` alone cannot propagate errors without additional `chan error` boilerplate; `errgroup` is the idiomatic choice in 2025/2026 Go.

**Key API surface used:**

```go
import "golang.org/x/sync/errgroup"

g, ctx := errgroup.WithContext(parentCtx)
g.SetLimit(3) // at most 3 addons installing concurrently

for _, addon := range addons {
    addon := addon // loop capture (required pre-Go 1.22; harmless after)
    g.Go(func() error {
        return addon.Execute(actionsCtx)
    })
}

if err := g.Wait(); err != nil {
    // first non-nil error — but addon system uses warn-and-continue
    // so errors are captured in addonResult, not returned from g.Go
}
```

**Warn-and-continue design:** Addon actions must NOT return errors from `g.Go` goroutines for the parallel case (because errgroup cancels peers on first error). Instead, each goroutine captures its error into an `addonResult` and always returns `nil` — preserving the existing warn-and-continue semantics while gaining parallelism.

**Add to go.mod:**

```bash
go get golang.org/x/sync@v0.19.0
```

**Confidence:** HIGH — version confirmed via https://pkg.go.dev/golang.org/x/sync?tab=versions (v0.19.0 released 2025-12-04). errgroup API confirmed via https://pkg.go.dev/golang.org/x/sync/errgroup.

---

### 4. Unit Tests for Addon Actions — Standard Library Only

**No new test dependencies.** The existing pattern across the codebase uses only `testing` from the standard library with table-driven tests. This is the correct pattern for kinder.

| What | Pattern | Example in codebase |
|------|---------|---------------------|
| Pure function testing | Table-driven `t.Run` tests | `subnet_test.go`, `corefile_test.go` |
| Logger stubbing | Custom `testLogger` struct implementing `log.Logger` | `create_addon_test.go` |
| Exec command stubbing | Function-level extraction of pure logic from exec calls | `subnet_test.go` (tests `parseSubnetFromJSON` not `detectSubnet`) |

**Why NOT to add testify:**

- The existing test files (`subnet_test.go`, `corefile_test.go`, `create_addon_test.go`, `waitforready_test.go`) all use zero external test dependencies. testify's `assert.Equal` is convenient but adds a dependency and changes the repo's zero-external-test-dep posture.
- The test patterns in the existing addon tests are idiomatic Go: `if got != want { t.Errorf(...) }`. New tests should match this pattern.
- `hack/tools/go.mod` does include `github.com/stretchr/testify v1.10.0` as a transitive dependency of golangci-lint, but it is not used directly in the main module test files.

**What the new addon tests should test (pure functions extracted from Execute):**

| Addon | Extractable pure function | Test focus |
|-------|--------------------------|------------|
| `installmetallb` | `carvePoolFromSubnet`, `parseSubnetFromJSON` | Already tested. Extend with edge cases. |
| `installcorednstuning` | `patchCorefile`, `indentCorefile` | Already tested. |
| `installcertmanager` | Manifest embedding verification | New: ensure embedded YAML is valid, non-empty |
| `installenvoygw` | Manifest embedding verification | New: same pattern |
| `installlocalregistry` | Hosts.toml template generation | New: extract template rendering |
| `installmetricsserver` | Manifest embedding verification | New |
| `installdashboard` | Manifest embedding verification | New |

**Key testing pattern for addon actions:**

The `Execute(ctx *ActionContext)` method cannot be unit tested directly because it calls `node.Command(...)` (which runs docker/kubectl). The correct approach is:

1. Extract the pure computation from `Execute` into unexported helper functions (like `patchCorefile` already does)
2. Write table-driven tests against those helpers
3. `Execute` itself is integration-tested by the existing integration test suite

**Confidence:** HIGH — based on direct reading of existing test files and the established project pattern.

---

### 5. context.Context Support for Blocking Operations

**No new dependencies.** `context` is a standard library package available since Go 1.7.

**What needs context threading:**

| Operation | Current | With context |
|-----------|---------|-------------|
| `Action.Execute(ctx *ActionContext)` | No cancellation | Add `context.Context` to `ActionContext` struct |
| `node.Command(...)` calls | Uses `Cmd` interface (no context) | Use `node.CommandContext(goCtx, ...)` where `CommandContext` already exists on `Cmder` interface |
| `waitforready.tryUntil(...)` | Polls until deadline, no external cancel | Pass `ctx context.Context`; select on `ctx.Done()` in poll loop |
| `Cluster(...)` in `create.go` | No context param | Add `ctx context.Context` as first parameter |

**Design decision: Minimal surface change**

Do NOT add `context.Context` to the `actions.Action` interface as a parameter change. That would require updating every `Execute` implementation immediately and break the interface contract. Instead:

- Add a `Ctx context.Context` field to `ActionContext` struct (already used for caching with the `cachedData` mutex)
- Each action accesses `ctx.Ctx` when it needs to pass context to `node.CommandContext`
- The cobra command layer creates `context.Background()` and threads it down; optionally wires to `signal.NotifyContext` for Ctrl-C cancellation

**Signal handling pattern (optional, can be phase-specific):**

```go
// In cobra RunE:
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
```

This is the idiomatic pattern for CLI tools that need graceful cancellation. The `os/signal` and `syscall` packages are both in the standard library.

**Confidence:** HIGH — pattern is consistent with Go standard library docs and the existing `CommandContext` support already present in `exec/local.go` and `exec/types.go`.

---

### 6. Structured JSON Output for CLI Commands

**No new dependencies.** Uses `encoding/json` from the standard library.

**Pattern for `--output` flag:**

```go
// In each command's options struct:
type flagpole struct {
    Output string // "text" | "json"
}

// Register with cobra:
flags.StringVar(&flagp.Output, "output", "text", `output format: "text" or "json"`)
```

**JSON output structs for `kinder get clusters`:**

```go
type ClusterInfo struct {
    Name     string `json:"name"`
    Provider string `json:"provider,omitempty"`
    Nodes    int    `json:"nodes,omitempty"`
}
```

**Render function pattern:**

```go
func renderOutput(w io.Writer, format string, data interface{}) error {
    switch format {
    case "json":
        enc := json.NewEncoder(w)
        enc.SetIndent("", "  ")
        return enc.Encode(data)
    default:
        // existing text rendering
    }
}
```

**Commands to add JSON output to:**

| Command | JSON struct fields |
|---------|--------------------|
| `kinder get clusters` | `[{"name": "kind"}]` |
| `kinder get nodes` | `[{"name": "...", "role": "...", "status": "..."}]` |
| `kinder env` | `{"KUBECONFIG": "...", "KIND_CLUSTER_NAME": "..."}` |
| `kinder doctor` | `{"checks": [{"name": "...", "status": "ok|warn|fail", "detail": "..."}]}` |

**Why NOT to add a third-party output library:**

- `encoding/json` with `SetIndent` produces standard, machine-parseable output identical to what `kubectl -o json` produces.
- Libraries like `go-pretty` or `tablewriter` are for human-readable table output — not needed for JSON.
- The GitHub CLI's `AddJSONFlags` pattern (jq expression filtering) is overkill for a local dev tool.

**Confidence:** HIGH — `encoding/json` is the standard, verified Go approach. No external library needed.

---

### 7. Addon Registry Pattern

**No new dependencies.** Implemented using standard Go interfaces and maps.

**Design: Map-based registry with ordered execution**

The existing addon system in `create.go` is a hardcoded sequence of `runAddon()` calls. The registry pattern replaces this with a declarative map.

**Core types:**

```go
// pkg/cluster/internal/create/actions/registry.go

// AddonFactory creates a new Action for an addon given the cluster config.
type AddonFactory func(cfg *config.Cluster) actions.Action

// AddonDescriptor describes a registered addon.
type AddonDescriptor struct {
    Name    string       // human-readable name for logs
    Enabled func(*config.AddonsConfig) bool  // predicate from config
    Factory AddonFactory
    Order   int          // lower = runs first; addons with equal Order run in parallel
}

// Registry holds all registered addons.
var Registry []AddonDescriptor
```

**Registration (replaces hardcoded runAddon calls):**

```go
// At package init time in each addon's package — or registered centrally in create.go
// Central registration is preferred: keeps create.go the single source of ordering truth.

var defaultAddons = []AddonDescriptor{
    {Name: "Local Registry", Order: 10, Enabled: func(a *config.AddonsConfig) bool { return a.LocalRegistry }, Factory: func(_ *config.Cluster) actions.Action { return installlocalregistry.NewAction() }},
    {Name: "MetalLB",        Order: 20, Enabled: func(a *config.AddonsConfig) bool { return a.MetalLB },        Factory: func(_ *config.Cluster) actions.Action { return installmetallb.NewAction() }},
    // ...
}
```

**Why this approach over `init()` auto-registration:**

- `init()` based plugin registries (where each addon package calls `registry.Register(...)` in its `init()`) require blank import side effects (`_ "pkg/addon/metallb"`) which are fragile and hard to discover.
- Central registration in `create.go` (or a dedicated `addons.go` file) is more readable, testable, and avoids init-order surprises.
- The `Order int` field enables the parallel install group: addons with the same `Order` value run concurrently via `errgroup`; different `Order` values run in sequence.

**Benefit for cluster presets:**

```go
// A "minimal" preset omits heavy addons:
type ClusterPreset struct {
    Name    string
    Addons  []string  // addon names to enable
    Config  *config.Cluster
}

var Presets = map[string]ClusterPreset{
    "minimal": {Name: "minimal", Addons: []string{}},
    "standard": {Name: "standard", Addons: []string{"MetalLB", "Metrics Server", "CoreDNS Tuning"}},
    "full":    {Name: "full", Addons: nil}, // nil = all enabled
}
```

**Confidence:** HIGH — pattern is standard Go interface design; the `Order`-based parallel grouping aligns with `errgroup.SetLimit` approach above.

---

### 8. golangci-lint — v1.62.2 → v2.10.1 (hack/tools module only)

| Item | Current | Recommended |
|------|---------|-------------|
| golangci-lint (in hack/tools/go.mod) | `v1.62.2` | `v2.10.1` |
| Config file (hack/tools/.golangci.yml) | v1 format | Migrate to v2 format |

**Why update:**

- golangci-lint v2 was released in March 2025. v1.x is in maintenance mode. The current `v1.62.2` is from late 2024.
- v2 has breaking config changes: `linters.disable-all` is replaced by `linters.default: none`, `exportloopref` is removed (fixed in Go 1.22+), `gosimple`/`stylecheck` merged into `staticcheck`.
- Run `golangci-lint migrate` to auto-convert `hack/tools/.golangci.yml` from v1 to v2 format.
- This only affects `hack/tools/go.mod`, not the main `go.mod`. The linter is a dev tool, not a runtime or test dependency.

**Linter config change required:**

Remove `exportloopref` from enabled linters (it is removed in v2; the underlying loop variable capture bug was fixed in Go 1.22 and the linter was retired). Add `copyloopvar` if targeting Go < 1.22 users (not needed here since we're at 1.23 minimum).

**Confidence:** MEDIUM — golangci-lint v2 confirmed at https://github.com/golangci/golangci-lint/releases (v2.10.1 released 2026-02-17). Migration guide confirmed at https://golangci-lint.run/docs/product/migration-guide/. Marking MEDIUM because the config migration requires validation after running `golangci-lint migrate`.

---

## Dependency Delta Summary

```
go.mod changes:
  go 1.21.0 → go 1.23.0
  golang.org/x/sys v0.6.0 → v0.41.0   (indirect, bump for security hygiene)
  golang.org/x/sync v0.19.0            (NEW — errgroup for parallel addon install)

hack/tools/go.mod changes:
  golangci-lint v1.62.2 → v2.10.1
  hack/tools/.golangci.yml: run golangci-lint migrate
```

---

## What NOT to Add

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `github.com/stretchr/testify` (main module) | Zero external test deps is the established project posture; stdlib `testing` is sufficient for table-driven tests of pure functions | Standard library `testing` package |
| `k8s.io/client-go` | All kubectl calls run inside kind node containers via `node.Command("kubectl", ...)` — no in-process k8s client needed | Existing `exec.Cmd` pattern |
| `gomock` / `mockery` | The testable addon pattern extracts pure functions; there is nothing to mock at the unit test level | Extract pure functions, test them directly |
| `sync.WaitGroup` (for parallel addons) | No built-in error propagation; requires `chan error` boilerplate | `golang.org/x/sync/errgroup` |
| `github.com/pkg/errors` (already in go.mod but deprecated by authors) | Already present as `sigs.k8s.io/kind/pkg/errors`; do not add new direct uses of `github.com/pkg/errors` from external code | Use `fmt.Errorf("...: %w", err)` for new code |
| `go-pretty` / `tablewriter` for JSON output | JSON rendering is one `encoding/json` call; third-party formatting libraries add dependencies with no benefit | `encoding/json` with `SetIndent` |
| Plugin system using `plugin.Open` (Go plugins via .so files) | Requires CGO, not available in static builds, platform-limited | Interface-based registry with compile-time registration |

---

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `golang.org/x/sync v0.19.0` | `go 1.23.0` | x/sync v0.19 requires go 1.22+; confirmed compatible |
| `golang.org/x/sys v0.41.0` | `go 1.23.0` | x/sys v0.41 requires go 1.23+; exact match with our bump |
| `golangci-lint v2.10.1` | `go 1.23` (hack/tools) | v2 requires go 1.22+; compatible with hack/tools go 1.23 |
| `github.com/stretchr/testify v1.11.1` | `go 1.23.0` | Latest testify (if ever added to main module); not required now |

---

## Integration Points

```
go.mod
  go 1.23.0                              (was 1.21.0)
  golang.org/x/sync v0.19.0             (NEW)
  golang.org/x/sys v0.41.0              (updated from v0.6.0, indirect)

pkg/cluster/internal/create/actions/action.go
  ActionContext.Ctx context.Context      (new field)

pkg/cluster/internal/create/actions/registry.go (NEW FILE)
  AddonDescriptor struct
  AddonFactory type
  defaultAddons []AddonDescriptor

pkg/cluster/internal/create/create.go
  Cluster(ctx context.Context, ...) error  (ctx added as first param)
  logSalutation(): rand.Intn() without explicit seeding (1.20+ cleanup)
  parallel addon loop using errgroup + defaultAddons registry

pkg/cmd/kind/get/clusters/clusters.go
  --output flag ("text"|"json")
  JSON rendering via encoding/json

pkg/cmd/kind/get/nodes/nodes.go
  --output flag ("text"|"json")

pkg/cmd/kind/env/env.go
  --output flag (text is default; json emits key-value as JSON object)

pkg/cmd/kind/doctor/doctor.go
  --output flag with CheckResult JSON struct

hack/tools/go.mod
  golangci-lint v2.10.1                  (from v1.62.2)

hack/tools/.golangci.yml
  Migrated to v2 config format (run: golangci-lint migrate)
  Remove: exportloopref (removed in v2)
```

---

## Sources

- Go 1.23 release notes: https://go.dev/doc/go1.23 (HIGH confidence)
- Go 1.24 release notes (rand.Seed no-op, T.Context): https://go.dev/doc/go1.24 (HIGH confidence)
- Go 1.25 release notes (sync.WaitGroup.Go, testing/synctest): https://go.dev/doc/go1.25 (HIGH confidence)
- golang.org/x/sys versions: https://pkg.go.dev/golang.org/x/sys?tab=versions — v0.41.0 released 2026-02-08 (HIGH confidence)
- golang.org/x/sync versions: https://pkg.go.dev/golang.org/x/sync?tab=versions — v0.19.0 released 2025-12-04 (HIGH confidence)
- errgroup API: https://pkg.go.dev/golang.org/x/sync/errgroup (HIGH confidence)
- x/sys backwards-incompatible change v0.23.0: https://github.com/golang/go/issues/68766 (HIGH confidence)
- golangci-lint v2.10.1: https://github.com/golangci/golangci-lint/releases (HIGH confidence)
- golangci-lint v2 migration guide: https://golangci-lint.run/docs/product/migration-guide/ (HIGH confidence)
- rand auto-seed since Go 1.20: https://go.dev/blog/randv2 (HIGH confidence)
- Existing codebase: direct file reads of go.mod, hack/tools/go.mod, create.go, action.go, exec/local.go, exec/types.go, and all addon test files (HIGH confidence)

---
*Stack research for: kinder — code quality, testing, parallel addons, JSON output, addon registry*
*Researched: 2026-03-03*
