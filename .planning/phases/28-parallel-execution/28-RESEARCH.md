# Phase 28: Parallel Execution - Research

**Researched:** 2026-03-03
**Domain:** Go concurrency — errgroup, sync.Once, wave-based parallel addon execution, timing instrumentation
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PARA-01 | Addon dependency DAG documented | 7 addons have exactly 1 dependency edge: MetalLB → EnvoyGateway; all others are independent; wave boundaries expressed as code comments in the addonRegistry slice |
| PARA-02 | sync.Once fix for ActionContext.Nodes() cache TOCTOU race | Current cachedData uses RWMutex check-then-set which allows concurrent goroutines to both see nil and both call ListNodes; replace with sync.Once or sync.OnceValues (Go 1.21+) |
| PARA-03 | Wave-based parallel addon execution via errgroup with SetLimit(3) | golang.org/x/sync v0.19.0 errgroup.Group with SetLimit(3) and WithContext; requires `go get golang.org/x/sync` |
| PARA-04 | Per-addon install duration printed in summary | Add `duration time.Duration` to addonResult; record `time.Since(start)` in runAddon; update logAddonSummary format string |
</phase_requirements>

---

## Summary

Phase 28 implements wave-based parallel addon execution in `kinder create cluster`. The current implementation runs all seven addons sequentially in a single loop via `runAddon`. The new implementation splits addons into two waves: Wave 1 (independent addons: Local Registry, MetalLB, Metrics Server, CoreDNS Tuning, Dashboard, Cert Manager) runs with up to 3 concurrent goroutines; Wave 2 (EnvoyGateway, which depends on MetalLB being ready) runs after Wave 1 completes. This follows the explicit project decision from REQUIREMENTS.md that rules out full DAG-based parallelism as over-engineered for 7 addons.

The implementation requires three coordinated changes: (1) replacing the TOCTOU-prone RWMutex cache in `ActionContext.Nodes()` with `sync.Once`/`sync.OnceValues` so concurrent goroutines safely share the cached node list; (2) adding `golang.org/x/sync` as a module dependency for `errgroup.Group.SetLimit(3)` — this package is not currently in go.mod; (3) extending the `addonResult` struct with a `duration` field and updating `logAddonSummary` to print per-addon install times. The `cli.Status` spinner is not concurrency-safe for parallel use; each parallel wave must use mutex-guarded or no-op status output.

The dependency DAG is shallow and fixed: only EnvoyGateway depends on MetalLB (MetalLB must provide LoadBalancer IPs before Envoy Gateway proxy services work). All other addons are independent. The project explicitly ruled out full DAG cycle detection as 7 addons with shallow deps do not justify 200+ extra lines.

**Primary recommendation:** Use `errgroup.WithContext` + `SetLimit(3)` for Wave 1; run Wave 2 sequentially after `g.Wait()`; fix Nodes() TOCTOU race with `sync.OnceValues`; add `duration time.Duration` field to `addonResult` and `time.Since` timing in `runAddon`.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `sync` | stdlib (Go 1.24+) | `sync.Once`, `sync.OnceValues` for cache fix | Standard library; `OnceValues` is Go 1.21+ and available at Go 1.24 minimum |
| `time` | stdlib | `time.Now()`, `time.Since()` for per-addon timing | Already imported in create.go |
| `golang.org/x/sync/errgroup` | v0.19.0 | `errgroup.Group` with `SetLimit(3)` for bounded parallel goroutines | Official Go extended library; the only correct primitive for fan-out with error collection |
| `context` | stdlib | Already used; `errgroup.WithContext` provides cancel-on-first-error | Already imported |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sync.Mutex` | stdlib | Protect `cli.Status` spinner calls during parallel execution | Status spinner is not documented as concurrent-safe |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `errgroup.Group` | `sync.WaitGroup` + channel | errgroup is cleaner: error collection built-in, SetLimit avoids semaphore boilerplate |
| `errgroup.Group` | Manual goroutines + `errors.Join` (Go 1.20+) | errgroup abstracts Add/Done counting, handles first-error, and provides SetLimit |
| `sync.OnceValues` | RWMutex check-then-set (current) | RWMutex pattern has TOCTOU: both goroutines see nil, both call ListNodes |
| `sync.OnceValues` | `sync.Once` + struct wrapper | `OnceValues` (Go 1.21+) is cleaner: returns `([]nodes.Node, error)` without a struct |

**Installation:**
```bash
go get golang.org/x/sync@v0.19.0
```

This adds `golang.org/x/sync` to go.mod and go.sum. It is the only new external dependency for this phase.

---

## Architecture Patterns

### Recommended File Changes

```
pkg/cluster/internal/create/
├── create.go              # Wave orchestration, addonResult.duration, logAddonSummary update
└── actions/
    └── action.go          # Replace cachedData RWMutex with sync.OnceValues
```

### Pattern 1: sync.OnceValues for Nodes() Cache (PARA-02)

**What:** Replace the current TOCTOU-prone RWMutex check-then-set pattern in `ActionContext.Nodes()` with `sync.OnceValues`. The race: two goroutines both call `getNodes()`, both see nil, both proceed to call `ListNodes`, both call `setNodes` — the second write is redundant but harmless. However, `go test -race` will detect that `nodes` is written by two goroutines without proper happens-before synchronization even with the RWMutex because the check (RLock, read nil, RUnlock) and the write (Lock, write, Unlock) are not atomic together.

**Current broken code (action.go):**
```go
// TOCTOU: two goroutines can both pass the nil check before either calls setNodes
func (ac *ActionContext) Nodes() ([]nodes.Node, error) {
    cachedNodes := ac.cache.getNodes()
    if cachedNodes != nil {
        return cachedNodes, nil
    }
    n, err := ac.Provider.ListNodes(ac.Config.Name)
    if err != nil {
        return nil, err
    }
    ac.cache.setNodes(n)
    return n, nil
}
```

**Fixed code using sync.OnceValues (Go 1.21+):**
```go
// Source: https://pkg.go.dev/sync#OnceValues
// Source: Go 1.21 release notes

type cachedData struct {
    once  sync.Once
    nodes []nodes.Node
    err   error
}

// newCachedData must be called with the provider/config ready.
// Or: use sync.OnceValues at ActionContext construction time.
```

**Preferred approach — store the result function on ActionContext:**
```go
// Source: https://pkg.go.dev/sync#OnceValues
// In NewActionContext:
ac.nodesOnce = sync.OnceValues(func() ([]nodes.Node, error) {
    return provider.ListNodes(cfg.Name)
})

// In Nodes():
func (ac *ActionContext) Nodes() ([]nodes.Node, error) {
    return ac.nodesOnce()
}
```

The `nodesOnce` field is a `func() ([]nodes.Node, error)` that caches the result of the first call. `sync.OnceValues` guarantees exactly one call to the wrapped function even under concurrent load. Subsequent calls return the cached result. Error is also cached — if `ListNodes` fails, subsequent calls return the same error (acceptable: the cluster must be listable for addons to proceed).

**ActionContext struct change:**
```go
type ActionContext struct {
    Context   context.Context
    Logger    log.Logger
    Status    *cli.Status
    Config    *config.Cluster
    Provider  providers.Provider
    nodesOnce func() ([]nodes.Node, error) // replaces cache *cachedData
}
```

**When to use:** Always. `sync.OnceValues` is the correct primitive for lazy-initialized, error-returning, concurrent-safe cache.

### Pattern 2: Wave-Based Parallel Execution (PARA-03)

**What:** Split the sequential addon loop into two waves using `errgroup.Group`. Wave 1 contains independent addons (no inter-addon dependencies). Wave 2 contains EnvoyGateway (depends on MetalLB from Wave 1).

**Dependency DAG (PARA-01):**
```
Wave 1 (parallel, SetLimit(3)):
  Local Registry  ──┐
  MetalLB         ──┤
  Metrics Server  ──┤──> [Wave 1 completes] ──> Wave 2
  CoreDNS Tuning  ──┤
  Dashboard       ──┤
  Cert Manager    ──┘

Wave 2 (sequential after Wave 1):
  Envoy Gateway (requires MetalLB LoadBalancer IPs)
```

**errgroup pattern:**
```go
// Source: https://pkg.go.dev/golang.org/x/sync/errgroup
import "golang.org/x/sync/errgroup"

// Wave 1: independent addons — run up to 3 concurrently
// Wave 1 addons: MetalLB must be before EnvoyGateway; all others are independent
g, gCtx := errgroup.WithContext(actionsContext.Context)
g.SetLimit(3) // PARA-03: limit to 3 concurrent goroutines

var mu sync.Mutex // protect addonResults slice and Status spinner

wave1 := []AddonEntry{
    // Wave 1: all independent (no inter-addon deps)
    {"Local Registry", opts.Config.Addons.LocalRegistry, installlocalregistry.NewAction()},
    {"MetalLB", opts.Config.Addons.MetalLB, installmetallb.NewAction()},
    {"Metrics Server", opts.Config.Addons.MetricsServer, installmetricsserver.NewAction()},
    {"CoreDNS Tuning", opts.Config.Addons.CoreDNSTuning, installcorednstuning.NewAction()},
    {"Dashboard", opts.Config.Addons.Dashboard, installdashboard.NewAction()},
    {"Cert Manager", opts.Config.Addons.CertManager, installcertmanager.NewAction()},
}

for _, addon := range wave1 {
    addon := addon // capture loop variable (required pre-Go 1.22; Go 1.22+ handles this)
    g.Go(func() error {
        // runAddon is warn-and-continue: it appends to addonResults, never returns error
        // Use gCtx-derived context for cancellation propagation
        runAddon(&mu, gCtx, &addonResults, addon.Name, addon.Enabled, addon.Action, actionsContext)
        return nil
    })
}
// Wait for Wave 1 to complete before Wave 2
if err := g.Wait(); err != nil {
    // This path is only reached if runAddon returns a non-nil error.
    // Since addons are warn-and-continue, this should not occur.
    return err
}

// Wave 2: EnvoyGateway depends on MetalLB (Wave 1 must complete first)
wave2 := []AddonEntry{
    {"Envoy Gateway", opts.Config.Addons.EnvoyGateway, installenvoygw.NewAction()},
}
for _, addon := range wave2 {
    runAddon(nil, context.Background(), &addonResults, addon.Name, addon.Enabled, addon.Action, actionsContext)
}
```

**Key decision:** `runAddon` is warn-and-continue (never returns an error to errgroup). The goroutine passed to `g.Go` always returns nil. This means `g.Wait()` only blocks for completion, never cancels. This matches the existing behavior (addon failures do not stop cluster creation).

### Pattern 3: Per-Addon Duration (PARA-04)

**What:** Add `duration time.Duration` to `addonResult`. Record `time.Since(start)` before appending to results in `runAddon`. Update `logAddonSummary` to print duration for enabled (installed or failed) addons.

**addonResult struct change:**
```go
// Source: create.go (existing struct)
type addonResult struct {
    name     string
    enabled  bool
    err      error
    duration time.Duration // PARA-04: install duration (zero for disabled addons)
}
```

**Updated runAddon:**
```go
runAddon := func(mu *sync.Mutex, ctx context.Context, results *[]addonResult, name string, enabled bool, a actions.Action, ac *actions.ActionContext) {
    if !enabled {
        if mu != nil { mu.Lock() }
        *results = append(*results, addonResult{name: name, enabled: false})
        if mu != nil { mu.Unlock() }
        return
    }
    start := time.Now()
    err := a.Execute(ac)
    dur := time.Since(start)
    if mu != nil { mu.Lock() }
    *results = append(*results, addonResult{name: name, enabled: true, err: err, duration: dur})
    if mu != nil { mu.Unlock() }
}
```

**Updated logAddonSummary:**
```go
// Source: create.go (existing logAddonSummary)
func logAddonSummary(logger log.Logger, results []addonResult) {
    logger.V(0).Info("")
    logger.V(0).Info("Addons:")
    for _, r := range results {
        switch {
        case !r.enabled:
            logger.V(0).Infof(" * %-20s skipped (disabled)", r.name)
        case r.err != nil:
            logger.V(0).Infof(" * %-20s FAILED: %v (%.1fs)", r.name, r.err, r.duration.Seconds())
        default:
            logger.V(0).Infof(" * %-20s installed (%.1fs)", r.name, r.duration.Seconds())
        }
    }
}
```

### Pattern 4: cli.Status Spinner Concurrency

**What:** `cli.Status` is not documented as concurrent-safe. `ctx.Status.Start("Installing X")` and `ctx.Status.End(true)` are called by each addon. During parallel Wave 1 execution, multiple addons call these concurrently.

**Safe approach:** Two options:

1. **Use a mutex around Status calls inside each Execute()** — requires modifying all addon packages.
2. **Replace Status with a no-op during parallel execution** — create a parallel-safe ActionContext variant that uses a no-op Status.

**Recommended approach:** Create a per-goroutine ActionContext copy that uses a no-op or mutex-wrapped Status. Since `ActionContext` is a struct (not a pointer to an interface), clone it per goroutine with a `Mutex`-wrapped status.

**Simplest viable approach:** Since `Status.Start` and `Status.End` are purely cosmetic (they print spinner state), and kinder's `cli.Status` is a thin wrapper, wrap all Status calls in the parallel runAddon with a mutex:

```go
// Option: create a goroutine-safe wrapper status that serializes calls
type mutexStatus struct {
    mu sync.Mutex
    s  *cli.Status
}
// Or: use a per-goroutine no-op status during parallel execution
// and only print to log.Logger (already concurrent-safe via atomic)
```

**Most pragmatic approach per project constraints:** The `cli.Status` implementation in kind uses `fmt.Fprintf(os.Stderr, ...)`. Multiple goroutines writing to stderr will interleave output but not cause data races (OS write syscalls are atomic for small writes). Check the actual implementation before assuming a race.

### Pattern 5: Dependency DAG Documentation (PARA-01)

**What:** Document the DAG in source code via comments in the `addonRegistry` slice definition.

**Example:**
```go
// Addon dependency DAG:
// Wave 1 (parallel): Local Registry, MetalLB, Metrics Server, CoreDNS Tuning, Dashboard, Cert Manager
//   All Wave 1 addons are independent — no inter-addon dependencies.
// Wave 2 (sequential, after Wave 1): Envoy Gateway
//   EnvoyGateway depends on MetalLB: MetalLB must assign LoadBalancer IPs before
//   Envoy Gateway proxy services receive external IPs.
// Wave boundary is explicit: errgroup.Wait() separates Wave 1 from Wave 2.
```

### Anti-Patterns to Avoid

- **Running EnvoyGateway in Wave 1:** It depends on MetalLB. Running them concurrently means Envoy Gateway proxy services start without LoadBalancer IPs available.
- **Using errgroup.WithContext for cancel-on-first-error semantics on warn-and-continue addons:** Since all addon errors are warn-and-continue, do not cancel the group context on addon failure. The runAddon func should always return nil to errgroup.
- **Sharing addonResults slice without a mutex:** Multiple goroutines appending to the same slice is a data race. Use a `sync.Mutex` or a channel to collect results.
- **Setting SetLimit(0):** Zero means no new goroutines can be added. Must use positive integer (3 per PARA-03).
- **Mutating SetLimit while goroutines are active:** The docs state "The limit must not be modified while any goroutines in the group are active." Call `SetLimit` before the first `g.Go` call.
- **Loop variable capture pre-Go 1.22:** The go module is at `go 1.24` so loop variable capture is automatic. However, the `for _, addon := range wave1 { addon := addon }` pattern is still accepted by Go 1.24 and is explicitly safe.
- **Cloning ActionContext naively:** `ActionContext.nodesOnce` must be shared across goroutines (it is the whole point of the once-cache). Do not copy the ActionContext struct — pass a pointer.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bounded concurrency with error collection | Custom semaphore channel + WaitGroup + error channel | `errgroup.Group` with `SetLimit(3)` | errgroup composes all three correctly; semaphore + WaitGroup has subtle deadlock risks |
| Thread-safe lazy-init with error return | Custom mutex + flag + struct fields | `sync.OnceValues` (Go 1.21+) | OnceValues is atomic, well-tested, handles panics and errors correctly |
| Loop variable closure | Manual `addon := addon` inside loop | Go 1.22+ automatic per-iteration copy | go.mod is 1.24, so loop variables are already per-iteration; but the explicit copy is still valid |
| Concurrent error joining | Custom error accumulation | First-error semantics from errgroup.Wait() | For warn-and-continue addons, errors go into addonResult.err, not errgroup error path |

**Key insight:** The existing `runAddon` helper already abstracts the warn-and-continue pattern. Phase 28 extends it minimally: add timing, add mutex parameter, change call site to goroutine-based.

---

## Common Pitfalls

### Pitfall 1: TOCTOU in Nodes() Cache with RWMutex

**What goes wrong:** The current `cachedData` uses RLock to read, then Lock to write. Between the RLock-read-nil-RUnlock and the Lock-write-Unlock, another goroutine can also read nil. Both goroutines call `ListNodes`. `go test -race` will report a write-write race on `cachedData.nodes` even with the mutex, because the pattern is "read under read lock, decide to write, release read lock, acquire write lock" — the decision to write was made based on a value read under a shared lock, not the exclusive lock.

**Why it happens:** `sync.RWMutex` does not provide "upgrade from read to write lock." The check-and-set must be done under a single exclusive lock, or via `sync.Once`.

**How to avoid:** Replace `cachedData` entirely with `sync.OnceValues`:
```go
// Source: https://pkg.go.dev/sync#OnceValues (Go 1.21+)
nodesOnce: sync.OnceValues(func() ([]nodes.Node, error) {
    return provider.ListNodes(cfg.Name)
})
```

**Warning signs:** `go test -race ./...` reports DATA RACE on `(*cachedData).nodes` with two concurrent goroutines in `setNodes` / `getNodes`.

### Pitfall 2: cli.Status Spinner Concurrent Writes

**What goes wrong:** Two goroutines both call `ctx.Status.Start("Installing X")` simultaneously. The status spinner writes to `os.Stderr`. Concurrent writes from multiple goroutines may produce garbled output or, if `cli.Status` uses internal mutable state (a buffer or tracking field), a data race.

**Why it happens:** `cli.Status` was designed for sequential use (one action at a time).

**How to avoid:** Inspect `pkg/internal/cli/status.go` before writing the implementation. If it has internal mutable state, use one of:
1. A `sync.Mutex`-wrapped status passed to each goroutine
2. A no-op status per goroutine (suppress spinners during parallel execution, print only log lines)

**Warning signs:** `go test -race` reports a race on status internal fields.

### Pitfall 3: addonResults Slice Concurrent Append

**What goes wrong:** Multiple goroutines appending to `var addonResults []addonResult` without synchronization causes a data race on the slice header (len, cap, backing array pointer).

**Why it happens:** Go slices are not concurrent-safe for writes. Append may grow the backing array.

**How to avoid:** Use a `sync.Mutex` to protect all appends, or use a pre-allocated slice with atomic index, or collect via a channel and drain after `g.Wait()`.

**Recommended:** Mutex approach — simple and consistent with the existing single-mutex pattern. Pre-allocate the slice to `len(wave1) + len(wave2)` to avoid reallocation:
```go
addonResults := make([]addonResult, 0, len(wave1)+len(wave2))
var mu sync.Mutex
```

**Warning signs:** `go test -race` reports DATA RACE on `addonResults` slice backing array.

### Pitfall 4: errgroup SetLimit Must Be Called Before First Go()

**What goes wrong:** Calling `g.SetLimit(3)` after `g.Go(...)` panics with: "errgroup: modify limit while X goroutines in the group are running."

**Why it happens:** The SetLimit implementation checks if goroutines are active and panics if so.

**How to avoid:** Always call `g.SetLimit(n)` immediately after `g, gCtx := errgroup.WithContext(ctx)`, before any `g.Go(...)` call.

**Warning signs:** Panic at runtime: `"errgroup: modify limit while ... goroutines in the group are running"`.

### Pitfall 5: Addon Order in Summary May Differ from Registration Order

**What goes wrong:** Wave 1 addons run concurrently, so they finish in non-deterministic order. If results are appended as goroutines finish (inside the goroutine), the summary printed by `logAddonSummary` shows addons in completion order, not registration order.

**Why it happens:** Goroutines return in non-deterministic order.

**How to avoid:** Two approaches:
1. Pre-allocate `addonResults` with index positions and write by index (requires knowing index at goroutine launch).
2. After `g.Wait()`, sort results by a fixed order (registration order) before calling `logAddonSummary`.

**Recommended:** Pre-allocate by index. This is the cleanest approach:
```go
addonResults := make([]addonResult, len(wave1)+len(wave2))
for i, addon := range wave1 {
    i, addon := i, addon
    g.Go(func() error {
        result := runAddonReturn(...)
        mu.Lock()
        addonResults[i] = result
        mu.Unlock()
        return nil
    })
}
```

**Warning signs:** Summary shows addons in different order each run, confusing users who expect a consistent display.

### Pitfall 6: go test -race Requires CGO

**What goes wrong:** The race detector requires CGO. The project sets `CGO_ENABLED=0` in the Makefile. Running `go test -race ./...` with `CGO_ENABLED=0` will fail with: `race: CGO is not enabled`.

**Why it happens:** The Go race detector is implemented using C code that intercepts memory accesses.

**How to avoid:** Run race tests with `CGO_ENABLED=1`:
```bash
CGO_ENABLED=1 go test -race ./...
```
or add a Makefile target that temporarily enables CGO:
```makefile
test-race:
    CGO_ENABLED=1 go test -race ./pkg/cluster/internal/create/...
```

**Warning signs:** `go test -race: CGO is not enabled` error when running with the Makefile environment.

---

## Code Examples

Verified patterns from official sources and codebase inspection:

### sync.OnceValues for Nodes() cache fix

```go
// Source: https://pkg.go.dev/sync#OnceValues (Go 1.21+, stdlib)
// Replace cachedData struct in action.go with:

type ActionContext struct {
    Context   context.Context
    Logger    log.Logger
    Status    *cli.Status
    Config    *config.Cluster
    Provider  providers.Provider
    nodesOnce func() ([]nodes.Node, error) // replaces cache *cachedData
}

func NewActionContext(
    ctx context.Context,
    logger log.Logger,
    status *cli.Status,
    provider providers.Provider,
    cfg *config.Cluster,
) *ActionContext {
    return &ActionContext{
        Context:  ctx,
        Logger:   logger,
        Status:   status,
        Provider: provider,
        Config:   cfg,
        // sync.OnceValues: calls ListNodes exactly once; caches (nodes, error) pair.
        // Concurrent callers block until first call completes, then get cached result.
        nodesOnce: sync.OnceValues(func() ([]nodes.Node, error) {
            return provider.ListNodes(cfg.Name)
        }),
    }
}

// Nodes returns the list of cluster nodes, cached after the first call.
// Concurrent calls are safe: sync.OnceValues guarantees exactly-once execution.
func (ac *ActionContext) Nodes() ([]nodes.Node, error) {
    return ac.nodesOnce()
}
```

### errgroup Wave 1 parallel execution

```go
// Source: https://pkg.go.dev/golang.org/x/sync/errgroup
import "golang.org/x/sync/errgroup"

// Pre-allocate results slice for deterministic ordering in summary
addonResults := make([]addonResult, 0, 7) // 6 wave1 + 1 wave2
var mu sync.Mutex

// Addon dependency DAG:
// Wave 1 (parallel, SetLimit(3)): all independent addons
// Wave 2 (sequential, after Wave 1): EnvoyGateway (depends on MetalLB from Wave 1)
// MetalLB must be ready before EnvoyGateway — wave boundary enforces this.
wave1 := []AddonEntry{
    {"Local Registry", opts.Config.Addons.LocalRegistry, installlocalregistry.NewAction()},
    {"MetalLB", opts.Config.Addons.MetalLB, installmetallb.NewAction()},
    {"Metrics Server", opts.Config.Addons.MetricsServer, installmetricsserver.NewAction()},
    {"CoreDNS Tuning", opts.Config.Addons.CoreDNSTuning, installcorednstuning.NewAction()},
    {"Dashboard", opts.Config.Addons.Dashboard, installdashboard.NewAction()},
    {"Cert Manager", opts.Config.Addons.CertManager, installcertmanager.NewAction()},
}

// Wave 2: EnvoyGateway depends on MetalLB (must run after Wave 1 completes)
wave2 := []AddonEntry{
    {"Envoy Gateway", opts.Config.Addons.EnvoyGateway, installenvoygw.NewAction()},
}

g, _ := errgroup.WithContext(actionsContext.Context)
g.SetLimit(3) // PARA-03: at most 3 addon goroutines concurrent

wave1Results := make([]addonResult, len(wave1))
for i, addon := range wave1 {
    i, addon := i, addon // Go 1.24 loop vars are per-iteration; explicit copy still valid
    g.Go(func() error {
        res := runAddonTimed(actionsContext, addon.Name, addon.Enabled, addon.Action)
        mu.Lock()
        wave1Results[i] = res // write by index for deterministic ordering
        mu.Unlock()
        return nil // warn-and-continue: never propagate addon error to errgroup
    })
}
if err := g.Wait(); err != nil {
    return err // unreachable with warn-and-continue, but correct error handling
}
addonResults = append(addonResults, wave1Results...)

// Wave 2: sequential (EnvoyGateway depends on MetalLB completing in Wave 1)
for _, addon := range wave2 {
    res := runAddonTimed(actionsContext, addon.Name, addon.Enabled, addon.Action)
    addonResults = append(addonResults, res)
}
```

### runAddonTimed helper (returns addonResult with timing)

```go
// Source: create.go (extension of existing runAddon helper)
func runAddonTimed(ctx *actions.ActionContext, name string, enabled bool, a actions.Action) addonResult {
    if !enabled {
        ctx.Logger.V(0).Infof(" * Skipping %s (disabled in config)\n", name)
        return addonResult{name: name, enabled: false}
    }
    start := time.Now()
    err := a.Execute(ctx)
    dur := time.Since(start)
    if err != nil {
        ctx.Logger.Warnf("Addon %s failed to install (cluster still usable): %v", name, err)
    }
    return addonResult{name: name, enabled: true, err: err, duration: dur}
}
```

### Updated logAddonSummary with duration

```go
// Source: create.go (extension of existing logAddonSummary)
func logAddonSummary(logger log.Logger, results []addonResult) {
    logger.V(0).Info("")
    logger.V(0).Info("Addons:")
    for _, r := range results {
        switch {
        case !r.enabled:
            logger.V(0).Infof(" * %-20s skipped (disabled)", r.name)
        case r.err != nil:
            logger.V(0).Infof(" * %-20s FAILED: %v (%.1fs)", r.name, r.err, r.duration.Seconds())
        default:
            logger.V(0).Infof(" * %-20s installed (%.1fs)", r.name, r.duration.Seconds())
        }
    }
}
```

### Test for Nodes() race-safety

```go
// Source: pattern for verifying sync.Once behavior under concurrency
// Run with: CGO_ENABLED=1 go test -race ./pkg/cluster/internal/create/actions/...
func TestNodesRaceFree(t *testing.T) {
    t.Parallel()
    listCount := 0
    provider := &testutil.FakeProvider{
        Nodes: []nodes.Node{testutil.NewFakeControlPlane("cp", nil)},
        InfoResp: &providers.ProviderInfo{},
    }
    ctx := testutil.NewTestContext(provider)

    var wg sync.WaitGroup
    for range 10 {
        wg.Add(1)
        go func() {
            defer wg.Done()
            nodes, err := ctx.Nodes()
            if err != nil {
                t.Errorf("unexpected error: %v", err)
            }
            if len(nodes) == 0 {
                t.Error("expected at least one node")
            }
        }()
    }
    wg.Wait()
    _ = listCount // sync.OnceValues ensures ListNodes called exactly once
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Sequential addon loop | Wave-based parallel with errgroup | Phase 28 | 5-6 independent addons run concurrently, reducing total creation time |
| RWMutex TOCTOU cache | `sync.OnceValues` | Phase 28 | Race-free node list caching; `go test -race` clean |
| No per-addon timing | `time.Since(start)` in addonResult | Phase 28 | Summary shows "MetalLB: 12.3s, EnvoyGateway: 8.1s" style output |
| `cachedData` struct with mu/nodes fields | Single `func() ([]nodes.Node, error)` field | Phase 28 | Less code, correct semantics |

**Deprecated/outdated:**
- `cachedData` struct: replaced by `sync.OnceValues` closure stored directly on ActionContext.
- Sequential `for _, addon := range addonRegistry` loop: replaced by wave1/wave2 errgroup pattern.

---

## Open Questions

1. **Is cli.Status concurrent-safe?**
   - What we know: `cli.Status` is in `pkg/internal/cli/status.go`; it was designed for sequential addon execution
   - What's unclear: Whether its internal implementation uses shared mutable state that would race under concurrent goroutines
   - Recommendation: Read `pkg/internal/cli/status.go` at implementation time. If it has mutable fields accessed without locks, use a per-goroutine no-op status during Wave 1, or wrap in a mutex. The spinner output during parallel execution is already confusing (multiple spinners would interleave), so suppressing parallel spinners and relying only on the final summary is acceptable UX.

2. **Should disabled addons preserve their registration order in the summary?**
   - What we know: With index-based result storage, disabled addons (which skip immediately without timing) will appear at their correct index in the pre-allocated results slice
   - What's unclear: Whether the planner should use index-based or channel-based result collection
   - Recommendation: Use index-based (`wave1Results[i] = res`) to guarantee summary order matches registration order regardless of goroutine scheduling.

3. **Does `go test -race` require a Makefile target or can it be run manually?**
   - What we know: The Makefile sets `CGO_ENABLED=0` globally; the race detector requires CGO; there is no existing `test-race` Makefile target
   - What's unclear: Whether PARA-02's "go test -race ./... is clean" means adding a Makefile target or just documenting the manual command
   - Recommendation: Add a `test-race` Makefile target with `CGO_ENABLED=1 go test -race ./pkg/cluster/internal/create/...` so it's reproducible. Document the requirement in the plan.

---

## Sources

### Primary (HIGH confidence)
- `pkg/cluster/internal/create/create.go` — addonResult struct, runAddon helper, logAddonSummary, addonRegistry (inspected directly)
- `pkg/cluster/internal/create/actions/action.go` — ActionContext, cachedData, Nodes() TOCTOU race (inspected directly)
- `pkg/cluster/internal/create/actions/testutil/fake.go` — NewTestContext, FakeProvider (inspected directly)
- All 7 addon Execute() implementations — confirmed independent (no inter-addon calls) except EnvoyGateway needing MetalLB IPs
- `go.mod` — confirmed `golang.org/x/sync` is NOT in module graph; must be added
- `hack/make-rules/test.sh` — confirmed no `-race` flag; CGO_ENABLED=0 in Makefile environment
- https://pkg.go.dev/sync#OnceValues — sync.OnceValues API (Go 1.21+)
- https://pkg.go.dev/golang.org/x/sync/errgroup — errgroup.Group.SetLimit API (v0.10.0, v0.19.0)
- `.planning/REQUIREMENTS.md` — PARA-01/02/03/04 definitions; explicit "Out of Scope: Full DAG-based parallel" decision

### Secondary (MEDIUM confidence)
- https://pkg.go.dev/golang.org/x/sync — v0.19.0 confirmed as latest (Dec 4, 2025)
- WebSearch: errgroup SetLimit added May 2022; stable since
- WebSearch: sync.OnceValues added Go 1.21 — confirmed available at Go 1.24 minimum

### Tertiary (LOW confidence)
- cli.Status concurrency-safety: assumed not concurrent-safe based on design context; must verify at implementation time

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — errgroup API verified via official docs; sync.OnceValues API verified via official docs; both are stdlib or official Go extended library
- Architecture: HIGH — dependency DAG derived from direct code inspection (EnvoyGateway requires MetalLB IPs; all others independent); wave pattern matches project's explicit Out of Scope ruling against full DAG
- Pitfalls: HIGH — TOCTOU race verified in source code; errgroup SetLimit constraint verified in official docs; CGO race detector requirement is well-known Go fact

**Research date:** 2026-03-03
**Valid until:** 2026-04-03 (errgroup and sync APIs are stable; Go 1.24 stdlib is stable)
