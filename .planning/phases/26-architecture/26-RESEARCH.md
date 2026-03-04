# Phase 26: Architecture - Research

**Researched:** 2026-03-03
**Domain:** Go context propagation, refactoring internal dispatch patterns, poll-loop cancellation
**Confidence:** HIGH

## Summary

Phase 26 is a pure refactoring phase with four tightly coupled changes to the kinder cluster creation internals. The full codebase has been read; everything needed is directly observable — no external dependencies are introduced.

The changes are additive-then-replace: add `context.Context` to `ActionContext`, update all addon `Execute()` call sites to `CommandContext`, replace the 7 hard-coded `runAddon()` call sites with a registry loop, and make `waitforready.tryUntil` context-aware. All four changes must leave `go build ./...` and `go vet ./...` clean with no import cycles.

Key structural insight: the `exec.Cmder` interface already defines `CommandContext(context.Context, string, ...string) Cmd`, and `common.Node.CommandContext` is already implemented and wired to `os/exec.CommandContext` under the hood. All that is missing is plumbing a `context.Context` through `ActionContext` so addon code can call it.

**Primary recommendation:** Thread context in struct (not function params) as documented in STATE.md v1.4 decisions. Add `Context context.Context` field to `ActionContext`, construct it in `create.go` from `context.Background()` initially, and pass `ctx.Context` to every `node.CommandContext(ctx.Context, ...)` call in each addon.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| ARCH-01 | context.Context added to ActionContext and propagated from create.go | ActionContext struct in action.go; NewActionContext in action.go called by create.go line 161; add Context field + propagate from Cluster() |
| ARCH-02 | All addon Execute() methods use CommandContext instead of Command | 7 addon files each call node.Command(); Cmder interface and common.Node already implement CommandContext; swap is mechanical |
| ARCH-03 | Centralized AddonEntry registry replaces hard-coded runAddon calls in create.go | create.go lines 214-220 contain 7 runAddon() call sites; replace with []AddonEntry loop in create.go |
| ARCH-04 | waitforready.tryUntil is context-aware and respects cancellation | tryUntil(until, try) in waitforready.go; add ctx parameter, check ctx.Done() in loop |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `context` (stdlib) | Go 1.24.0 | Context propagation, cancellation | Standard Go idiom for deadline/cancellation propagation |
| `sigs.k8s.io/kind/pkg/exec` | in-repo | Cmd / Cmder interfaces | Already defines CommandContext; no new import needed |
| `sigs.k8s.io/kind/pkg/cluster/internal/providers/common` | in-repo | Node.CommandContext | Already implemented; wraps os/exec.CommandContext |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `select` statement | Go stdlib | Non-blocking context check in tryUntil | In poll loops where time.Sleep blocks cancellation |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Context in struct field | Context as function param to Execute() | Param avoids mutation risk but changes every Execute() signature; struct field is the decided approach (STATE.md) |
| `context.Background()` in Cluster() | `context.WithCancel` from cmd layer | WithCancel enables CLI Ctrl-C cancellation; background is sufficient for Phase 26 scope; full signal handling is a future concern |

**Installation:**
```bash
# No new dependencies — all packages are stdlib or already in-repo
```

## Architecture Patterns

### Files Changed

```
pkg/cluster/internal/create/
├── actions/
│   ├── action.go                          # +Context field, update NewActionContext
│   ├── waitforready/waitforready.go       # tryUntil ctx-aware
│   ├── installenvoygw/envoygw.go          # node.Command → node.CommandContext
│   ├── installmetallb/metallb.go          # node.Command → node.CommandContext
│   ├── installmetricsserver/metricsserver.go  # node.Command → node.CommandContext
│   ├── installcorednstuning/corednstuning.go  # node.Command → node.CommandContext
│   ├── installdashboard/dashboard.go      # node.Command → node.CommandContext
│   ├── installlocalregistry/localregistry.go  # node.Command → node.CommandContext
│   └── installcertmanager/certmanager.go  # node.Command → node.CommandContext
└── create.go                              # NewActionContext +ctx, AddonEntry registry
```

### Pattern 1: Context Field in ActionContext (ARCH-01)

**What:** Add `Context context.Context` as a public field to `ActionContext`. Populate it in `NewActionContext` from a passed-in context. Update `create.go` to call `NewActionContext` with `context.Background()`.

**When to use:** All phases where context flows through an orchestrator struct rather than per-function parameters. Minimizes call-site churn since `Execute(ctx *ActionContext)` signature stays unchanged.

**Example:**
```go
// pkg/cluster/internal/create/actions/action.go

import "context"

// ActionContext is data supplied to all actions
type ActionContext struct {
    // Context is used to propagate cancellation into node commands.
    // Populated by create.go; use node.CommandContext(ctx.Context, ...) in actions.
    Context  context.Context
    Logger   log.Logger
    Status   *cli.Status
    Config   *config.Cluster
    Provider providers.Provider
    cache    *cachedData
}

// NewActionContext returns a new ActionContext
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
        cache:    &cachedData{},
    }
}
```

In `create.go`:
```go
// pkg/cluster/internal/create/create.go
import "context"

// Line ~161 — existing call:
actionsContext := actions.NewActionContext(logger, status, p, opts.Config)

// Becomes:
actionsContext := actions.NewActionContext(context.Background(), logger, status, p, opts.Config)
```

### Pattern 2: CommandContext in Addon Execute() (ARCH-02)

**What:** Replace every `node.Command(...)` call in addon Execute() methods with `node.CommandContext(ctx.Context, ...)`. The `exec.Cmder` interface already declares `CommandContext`; `common.Node` already implements it.

**When to use:** Every place an addon runs a kubectl or shell command inside a node container.

**Example (installmetricsserver):**
```go
// Before:
if err := node.Command("kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "-f", "-").SetStdin(...).Run(); err != nil {

// After:
if err := node.CommandContext(ctx.Context,
    "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "-f", "-").SetStdin(...).Run(); err != nil {
```

**Scope — all 7 addon files that call node.Command:**
- `installenvoygw/envoygw.go` — 5 node.Command calls
- `installmetallb/metallb.go` — 3 node.Command calls
- `installmetricsserver/metricsserver.go` — 2 node.Command calls
- `installcorednstuning/corednstuning.go` — 4 node.Command calls
- `installdashboard/dashboard.go` — 3 node.Command calls
- `installlocalregistry/localregistry.go` — 4 node.Command calls (also uses `exec.Command` for host-side docker commands — those do NOT get ctx because they run on the host, not inside nodes; see Pitfalls)
- `installcertmanager/certmanager.go` — 4 node.Command calls

**Note on installstorage:** `installstorage/storage.go` uses `controlPlane.Command(...)` via `addDefaultStorage()` helper. This file also needs the same update but is not an addon (it is in `actionsToRun`). Update it for consistency if it is touched; if not, leave for Phase 27 test scope.

### Pattern 3: AddonEntry Registry (ARCH-03)

**What:** Define an `AddonEntry` struct in `create.go` that pairs a human-readable name, an enabled flag, and an `actions.Action`. Replace the 7 `runAddon(...)` call sites with a single loop over `[]AddonEntry`.

**When to use:** Whenever a set of similar items is dispatched with the same logic. Eliminates copy-paste drift, makes adding a future addon a one-liner.

**Example:**
```go
// pkg/cluster/internal/create/create.go

// AddonEntry pairs an addon's display name, enabled flag, and action.
type AddonEntry struct {
    Name    string
    Enabled bool
    Action  actions.Action
}

// In Cluster(), replace the 7 runAddon() lines with:
addons := []AddonEntry{
    {"Local Registry", opts.Config.Addons.LocalRegistry, installlocalregistry.NewAction()},
    {"MetalLB",        opts.Config.Addons.MetalLB,        installmetallb.NewAction()},
    {"Metrics Server", opts.Config.Addons.MetricsServer,  installmetricsserver.NewAction()},
    {"CoreDNS Tuning", opts.Config.Addons.CoreDNSTuning,  installcorednstuning.NewAction()},
    {"Envoy Gateway",  opts.Config.Addons.EnvoyGateway,   installenvoygw.NewAction()},
    {"Dashboard",      opts.Config.Addons.Dashboard,      installdashboard.NewAction()},
    {"Cert Manager",   opts.Config.Addons.CertManager,    installcertmanager.NewAction()},
}

for _, addon := range addons {
    runAddon(addon.Name, addon.Enabled, addon.Action)
}
```

The `runAddon` closure and `addonResults` slice remain unchanged. Only the call sites change.

**Why AddonEntry in create.go, not action.go:** No import cycle risk. AddonEntry only needs `actions.Action` which is already imported. It is an internal dispatch struct, not a public API.

### Pattern 4: Context-Aware tryUntil (ARCH-04)

**What:** Add `ctx context.Context` as the first parameter to `tryUntil`. Replace the `time.Sleep(500ms)` with a `select` that returns immediately when `ctx.Done()` is closed.

**When to use:** All poll loops that currently block on `time.Sleep` and do not honour context cancellation.

**Example:**
```go
// pkg/cluster/internal/create/actions/waitforready/waitforready.go

import "context"

// tryUntil calls try() in a loop until the deadline or context cancellation.
// Returns true if try() ever returned true.
func tryUntil(ctx context.Context, until time.Time, try func() bool) bool {
    for until.After(time.Now()) {
        if try() {
            return true
        }
        select {
        case <-ctx.Done():
            return false
        case <-time.After(500 * time.Millisecond):
        }
    }
    return false
}
```

Update the single call site in `waitForReady`:
```go
// Before:
isReady := waitForReady(node, startTime.Add(a.waitTime), selectorLabel)

// After:
isReady := waitForReady(ctx.Context, node, startTime.Add(a.waitTime), selectorLabel)
```

And thread ctx through `waitForReady`:
```go
func waitForReady(ctx context.Context, node nodes.Node, until time.Time, selectorLabel string) bool {
    return tryUntil(ctx, until, func() bool {
        // ... unchanged body using node.Command (no ctx needed here; the
        // context-awareness is from tryUntil's select, not per-command)
        ...
    })
}
```

**Note:** The `node.Command` calls inside `waitForReady`'s closure check can optionally be updated to `node.CommandContext` too — the context-awareness of the loop itself (via select) is what ARCH-04 requires.

### Anti-Patterns to Avoid

- **Changing the `Execute(ctx *ActionContext)` signature:** The interface must stay the same. Context goes inside the struct.
- **Using `exec.Command` for in-node commands after this phase:** All in-node commands should use `CommandContext`. Only host-side docker/nerdctl/podman CLI calls (in `installlocalregistry`) stay as `exec.Command`.
- **Initialising `ActionContext.Context` lazily or with nil:** Always populate in `NewActionContext`. A nil context causes panic at CommandContext call sites.
- **Adding `AddonEntry` to `actions/action.go`:** Would create a temptation to import addon packages from action.go creating cycles. Keep in `create.go`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Context-aware sleep | Custom timer goroutine | `select { case <-ctx.Done(): case <-time.After(d): }` | Idiomatic Go; zero extra deps; composes with any context |
| Command cancellation | Manual process kill | `node.CommandContext(ctx, ...)` which wraps `os/exec.CommandContext` | Already implemented in common.Node; os/exec handles SIGKILL on cancellation |
| Addon dispatch loop | Switch/case per addon name | `[]AddonEntry` slice | Already exists as repetitive code; slice is simpler and easier to extend |

**Key insight:** All the infrastructure for context-aware commands already exists. This phase is entirely plumbing — wiring up what's already there.

## Common Pitfalls

### Pitfall 1: Forgetting Host-Side exec.Command in installlocalregistry
**What goes wrong:** `installlocalregistry/localregistry.go` uses both `node.Command(...)` (inside cluster nodes) AND `exec.Command(binaryName, ...)` (host-side docker/nerdctl commands). Replacing the host-side `exec.Command` calls with `exec.CommandContext` is optional but incorrect to confuse with in-node calls.
**Why it happens:** The file mixes two execution targets. The host-side commands (docker inspect, docker run, docker network connect) do not go through the Node interface.
**How to avoid:** Only replace `node.Command(...)` calls (calls on a `nodes.Node` value) with `node.CommandContext(ctx.Context, ...)`. Leave `exec.Command(binaryName, ...)` unchanged unless explicitly extending scope.
**Warning signs:** If you see `exec.Command` called directly (not via a node variable), it is a host-side command.

### Pitfall 2: Import of "context" Missing
**What goes wrong:** `action.go`, `waitforready.go`, and `create.go` do not currently import `"context"`. Adding context fields/params without adding the import causes a compile error.
**Why it happens:** Oversight when adding new fields to existing files.
**How to avoid:** After each file edit, confirm `"context"` is in the import block.

### Pitfall 3: tryUntil Signature Change Breaks Build
**What goes wrong:** `tryUntil` is only called from `waitForReady` in the same file. But if any test file calls `tryUntil` directly, adding a `ctx` parameter breaks those tests.
**Why it happens:** Internal unexported functions can still be called from `_test.go` files in the same package.
**How to avoid:** Run `grep -r "tryUntil" pkg/` before changing the signature to find all call sites. Currently only one call site exists: `waitForReady` in waitforready.go.

### Pitfall 4: nil Context Panic
**What goes wrong:** If `NewActionContext` is called from test code without a context argument (after the signature change), callers pass `nil` or forget the new param, causing a panic when `node.CommandContext(ctx.Context, ...)` is called.
**Why it happens:** Signature change to `NewActionContext` must be updated at all call sites.
**How to avoid:** `NewActionContext` is called in exactly one place: `create.go` line ~161. Verify with `grep -r "NewActionContext" pkg/`. Update that one call site to pass `context.Background()`.

### Pitfall 5: installstorage addDefaultStorage Helper
**What goes wrong:** `installstorage/storage.go` calls `controlPlane.Command(...)` inside `addDefaultStorage()`. This function signature does not take a `*ActionContext`, so it cannot easily receive `ctx.Context` without signature changes to `addDefaultStorage`.
**Why it happens:** The helper was extracted for testability and does not have access to `ActionContext`.
**How to avoid:** For Phase 26 scope, leave `installstorage` unchanged — it is not in the addon section (it runs in `actionsToRun` before addons) and the requirement only specifies addon Execute() methods (ARCH-02). Document this as a known gap.

## Code Examples

### Complete action.go After Change
```go
// Source: direct analysis of pkg/cluster/internal/create/actions/action.go
package actions

import (
    "context"
    "sync"

    "sigs.k8s.io/kind/pkg/cluster/nodes"
    "sigs.k8s.io/kind/pkg/internal/apis/config"
    "sigs.k8s.io/kind/pkg/internal/cli"
    "sigs.k8s.io/kind/pkg/log"

    "sigs.k8s.io/kind/pkg/cluster/internal/providers"
)

// Action defines a step of bringing up a kind cluster after initial node
// container creation
type Action interface {
    Execute(ctx *ActionContext) error
}

// ActionContext is data supplied to all actions.
// Context is stored in the struct (not passed as a function param) to minimise
// call-site churn — Execute() signatures remain unchanged across all action packages.
type ActionContext struct {
    // Context carries cancellation and deadline for all node commands in this action.
    // Populated by create.go; never nil.
    Context  context.Context
    Logger   log.Logger
    Status   *cli.Status
    Config   *config.Cluster
    Provider providers.Provider
    cache    *cachedData
}

// NewActionContext returns a new ActionContext
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
        cache:    &cachedData{},
    }
}
```

### AddonEntry Loop in create.go
```go
// Source: direct analysis of pkg/cluster/internal/create/create.go (lines 190-220)

// AddonEntry pairs an addon's display name, enabled flag, and action for
// the registry-driven installation loop.
type AddonEntry struct {
    Name    string
    Enabled bool
    Action  actions.Action
}

// Inside Cluster():
// Dependency conflict check (per user decision: warn and continue)
if !opts.Config.Addons.MetalLB && opts.Config.Addons.EnvoyGateway {
    logger.Warn("MetalLB is disabled but Envoy Gateway is enabled. Envoy Gateway proxy services will not receive LoadBalancer IPs.")
}

// Run addon actions in dependency order via registry
addonRegistry := []AddonEntry{
    {"Local Registry", opts.Config.Addons.LocalRegistry, installlocalregistry.NewAction()},
    {"MetalLB",        opts.Config.Addons.MetalLB,        installmetallb.NewAction()},
    {"Metrics Server", opts.Config.Addons.MetricsServer,  installmetricsserver.NewAction()},
    {"CoreDNS Tuning", opts.Config.Addons.CoreDNSTuning,  installcorednstuning.NewAction()},
    {"Envoy Gateway",  opts.Config.Addons.EnvoyGateway,   installenvoygw.NewAction()},
    {"Dashboard",      opts.Config.Addons.Dashboard,      installdashboard.NewAction()},
    {"Cert Manager",   opts.Config.Addons.CertManager,    installcertmanager.NewAction()},
}
for _, addon := range addonRegistry {
    runAddon(addon.Name, addon.Enabled, addon.Action)
}
```

### Context-Aware tryUntil
```go
// Source: direct analysis of pkg/cluster/internal/create/actions/waitforready/waitforready.go

// tryUntil calls try() in a loop until the deadline or context cancellation.
// Returns true if try() ever returned true; returns false on timeout or cancellation.
func tryUntil(ctx context.Context, until time.Time, try func() bool) bool {
    for until.After(time.Now()) {
        if try() {
            return true
        }
        select {
        case <-ctx.Done():
            return false
        case <-time.After(500 * time.Millisecond):
        }
    }
    return false
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `node.Command(...)` in all actions | `node.CommandContext(ctx, ...)` | Phase 26 | Commands respect context cancellation |
| 7 explicit `runAddon()` calls | `[]AddonEntry` registry loop | Phase 26 | Adding an 8th addon is a one-liner |
| `tryUntil` spins on `time.Sleep` | `tryUntil` uses `select` with `ctx.Done()` | Phase 26 | Control-C or timeout cancels wait immediately |
| No context in `ActionContext` | `Context context.Context` field | Phase 26 | Prepares for Phase 28 wave-parallel execution |

**Deprecated/outdated after this phase:**
- `node.Command(...)` in addon Execute() methods (replaced by CommandContext)
- Direct 7-line `runAddon()` block in create.go (replaced by registry loop)

## Open Questions

1. **installstorage addDefaultStorage context gap**
   - What we know: `addDefaultStorage` takes `(logger, node)` not `ActionContext`; no context threading possible without signature change
   - What's unclear: Whether ARCH-02 intends to include non-addon actions like installstorage
   - Recommendation: Exclude installstorage from Phase 26 scope. ARCH-02 says "All addon Execute() methods"; installstorage is not an addon. Note the gap in PLAN summary.

2. **context.Background() vs signal-wired context**
   - What we know: Phase 26 uses `context.Background()` in create.go
   - What's unclear: Whether Ctrl-C during cluster creation should cancel in-flight kubectl commands
   - Recommendation: `context.Background()` is correct for Phase 26. Signal-wired context (listening for os.Signal) is a future concern outside this phase's scope.

## Sources

### Primary (HIGH confidence)
- Direct read of `pkg/cluster/internal/create/actions/action.go` — ActionContext struct, NewActionContext, Action interface
- Direct read of `pkg/cluster/internal/create/create.go` — Cluster(), runAddon(), 7 addon call sites (lines 214-220), NewActionContext call site
- Direct read of `pkg/cluster/internal/create/actions/waitforready/waitforready.go` — tryUntil, waitForReady
- Direct read of `pkg/exec/types.go` — Cmder interface (CommandContext already declared)
- Direct read of `pkg/cluster/nodes/types.go` — Node interface (embeds exec.Cmder)
- Direct read of `pkg/cluster/internal/providers/common/node.go` — CommandContext already implemented
- Direct read of `pkg/exec/local.go` — LocalCmd.CommandContext already implemented
- Direct read of all 7 addon Execute() files — confirmed node.Command usage patterns

### Secondary (MEDIUM confidence)
- `.planning/STATE.md` v1.4 decisions — "Context in struct (not function param)" confirmed as locked decision
- `.planning/REQUIREMENTS.md` — ARCH-01 through ARCH-04 requirements

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages are in-repo stdlib; no external deps
- Architecture: HIGH — all change sites directly observed in source; no guessing
- Pitfalls: HIGH — all pitfalls identified from reading the actual code (mixed exec targets, signature changes, test call sites)

**Research date:** 2026-03-03
**Valid until:** stable — codebase is under active refactoring but these files are unlikely to change between research and planning
