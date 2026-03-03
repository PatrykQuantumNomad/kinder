# Architecture Patterns

**Domain:** Go CLI — kinder code quality and feature milestone (addon registry, context propagation, parallelism, JSON output, testing)
**Researched:** 2026-03-03
**Confidence:** HIGH — all analysis is from direct codebase read; no inference needed

---

## System Overview

```
pkg/
├── apis/config/v1alpha4/types.go       ← PUBLIC API: Addons struct (*bool fields)
├── internal/apis/config/types.go       ← INTERNAL: Addons struct (plain bool fields)
│
├── cluster/
│   ├── provider.go                     ← LAYER VIOLATION: imports cmd/kind/version
│   │                                      FIX: move version constants to pkg/internal/version
│   │                                      or pkg/cluster/internal/version
│   │
│   └── internal/
│       ├── create/
│       │   ├── create.go               ← Action pipeline orchestration
│       │   │                             FIX: context.Context propagation
│       │   │                             FIX: parallel addon phase
│       │   │                             FIX: addon registry pattern
│       │   │                             FIX: JSON output alongside human output
│       │   └── actions/
│       │       ├── action.go           ← ActionContext — add context.Context field
│       │       ├── installmetallb/     ← existing addon (pattern reference)
│       │       │   ├── metallb.go
│       │       │   └── subnet.go + subnet_test.go (EXISTING unit test model)
│       │       ├── installcorednstuning/
│       │       │   └── corefile_test.go (EXISTING unit test model)
│       │       ├── installenvoygw/
│       │       ├── installlocalregistry/
│       │       ├── installcertmanager/
│       │       ├── installmetricsserver/
│       │       ├── installdashboard/
│       │       └── installstorage/
│       │
│       └── providers/
│           └── common/                 ← CommandContext already implemented (node.go)
│
├── cmd/kind/
│   ├── root.go
│   └── version/                        ← PROBLEM SOURCE: CLI-layer package
│       └── version.go                  ← imported by pkg/cluster/provider.go (violation)
│
└── internal/
    └── version/                        ← DOES NOT EXIST YET
        └── kindversion.go              ← NEW: move Version/DisplayVersion here
```

---

## Component Responsibilities

| Component | Responsibility | What Changes |
|-----------|---------------|--------------|
| `create.go` | Pipeline orchestration | Add ctx propagation; addon registry loop; parallel phase; JSON output |
| `action.go` | ActionContext data bag | Add `context.Context` field |
| `pkg/cluster/provider.go` | Public cluster API | Remove `cmd/kind/version` import |
| `pkg/cmd/kind/version/version.go` | CLI command + version strings | Remove `Version()` / `DisplayVersion()` from here; re-export from new internal package |
| Addon actions (`.go` files) | kubectl exec via node.Command | Split pure logic into testable helper functions (pattern: corefile_test.go) |

---

## Six Improvements: Integration Analysis

### Improvement 1: Addon Registry Pattern

**Problem:** `create.go` lines 213-219 hard-code seven `runAddon(...)` calls. Adding an eighth addon requires editing two files: the action package + `create.go`.

**Current structure:**
```go
// create.go — hard-coded list
runAddon("Local Registry",  opts.Config.Addons.LocalRegistry, installlocalregistry.NewAction())
runAddon("MetalLB",         opts.Config.Addons.MetalLB, installmetallb.NewAction())
runAddon("Metrics Server",  opts.Config.Addons.MetricsServer, installmetricsserver.NewAction())
runAddon("CoreDNS Tuning",  opts.Config.Addons.CoreDNSTuning, installcorednstuning.NewAction())
runAddon("Envoy Gateway",   opts.Config.Addons.EnvoyGateway, installenvoygw.NewAction())
runAddon("Dashboard",       opts.Config.Addons.Dashboard, installdashboard.NewAction())
runAddon("Cert Manager",    opts.Config.Addons.CertManager, installcertmanager.NewAction())
```

**Target structure — registry in `actions/addon_registry.go`:**
```go
// AddonEntry describes one addon in the registry.
type AddonEntry struct {
    Name    string
    Enabled func(cfg *config.Cluster) bool
    New     func() Action
}

// Registry returns the ordered list of addons to run.
// Order defines dependency: LocalRegistry first (containerd config), MetalLB
// before EnvoyGateway (LoadBalancer IPs), CertManager last (depends on nothing).
func Registry(cfg *config.Cluster) []AddonEntry {
    return []AddonEntry{
        {
            Name:    "Local Registry",
            Enabled: func(c *config.Cluster) bool { return c.Addons.LocalRegistry },
            New:     installlocalregistry.NewAction,
        },
        // ... remaining addons in dependency order ...
    }
}
```

**`create.go` then becomes:**
```go
for _, entry := range actions.Registry(opts.Config) {
    runAddon(entry.Name, entry.Enabled(opts.Config), entry.New())
}
```

**Files modified:** `pkg/cluster/internal/create/actions/action.go` (or new `addon_registry.go`), `pkg/cluster/internal/create/create.go`.

**Files NOT modified:** Each addon action package is unchanged. No new imports needed in `create.go` if the registry owns the imports.

**Import consideration:** The registry file must import all addon packages. Currently `create.go` owns those imports. The registry file takes them over — it lives in the `actions` package, so it imports sibling packages (`actions/installmetallb`, etc.). This is correct: the registry is in `pkg/cluster/internal/create/actions/`, which can import sub-packages.

---

### Improvement 2: Moving the version Package to Fix Layer Violation

**Problem:** `pkg/cluster/provider.go` imports `sigs.k8s.io/kind/pkg/cmd/kind/version`. The library layer (`pkg/cluster/`) must not import the CLI layer (`pkg/cmd/`). This is a hard dependency inversion — the library depends on a command.

**Diagnosis:** `provider.go` calls `version.DisplayVersion()` in `CollectLogs` to write `kind-version.txt`. The `version` package contains:
1. `Version() string` — assembles a version string from build-time injected vars
2. `DisplayVersion() string` — prefixes `Version()` with "kind v" + runtime info
3. `NewCommand()` — Cobra command (CLI concern, must stay in `pkg/cmd/kind/version/`)

**Move plan — new package `pkg/internal/kindversion/`:**

```
pkg/internal/kindversion/
└── version.go    ← Version(), DisplayVersion(), version constants and vars
```

The existing `pkg/cmd/kind/version/version.go` becomes a thin wrapper:
```go
package version

import "sigs.k8s.io/kind/pkg/internal/kindversion"

// Version re-exports from the internal package.
func Version() string { return kindversion.Version() }

// DisplayVersion re-exports from the internal package.
func DisplayVersion() string { return kindversion.DisplayVersion() }

// NewCommand remains here — CLI concern, does not move.
func NewCommand(...) *cobra.Command { ... }
```

`pkg/cluster/provider.go` changes its import from `pkg/cmd/kind/version` to `pkg/internal/kindversion`.

`root.go` still works unchanged because `version.NewCommand` stays in `pkg/cmd/kind/version`.

**Build order for this change:**
1. Create `pkg/internal/kindversion/version.go` — copy `Version()`, `DisplayVersion()`, constants, build-time vars
2. Update `pkg/cluster/provider.go` — change import, update function call
3. Simplify `pkg/cmd/kind/version/version.go` — delegate to `kindversion`, keep `NewCommand`
4. `go build ./...` must pass
5. `go test ./pkg/cmd/kind/version/...` must pass (tests reference unexported `version()` and `truncate()` — these stay in the version package since they test internal logic)

**Test file consideration:** `pkg/cmd/kind/version/version_test.go` tests the internal `version()` function (unexported, same package). This test stays in `pkg/cmd/kind/version/` because `version()` stays there (it's a private helper called by the public `Version()` which delegates to `kindversion`). Alternatively, move the internal `version()` function to `kindversion` along with its test.

**Recommended:** Move the `version()` function and its test to `kindversion` for full coverage of the moved code. The `truncate()` helper and its test move with `version()`.

---

### Improvement 3: Adding context.Context to Blocking Operations

**Problem:** Blocking calls (`kubectl wait --timeout=120s`, `kubectl apply`, `tryUntil` loops in `waitforready`) have no cancellation path. Long-running cluster creates cannot be interrupted.

**Where context.Context already exists:**
- `exec.Cmder` interface: `CommandContext(context.Context, string, ...string) Cmd` (already in `pkg/exec/types.go`)
- `common.Node.CommandContext(ctx, command, args...)` (already in `pkg/cluster/internal/providers/common/node.go`)
- `exec.LocalCmder.CommandContext(ctx, ...)` (already in `pkg/exec/local.go`)

The infrastructure for context-aware commands is already present. It is unused by addon actions.

**What must change:**

**Step 1 — Add `context.Context` to `ActionContext`:**
```go
// pkg/cluster/internal/create/actions/action.go
type ActionContext struct {
    Context  context.Context   // NEW field
    Logger   log.Logger
    Status   *cli.Status
    Config   *config.Cluster
    Provider providers.Provider
    cache    *cachedData
}

func NewActionContext(
    ctx context.Context,       // NEW parameter
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

**Step 2 — Thread context through `create.go`:**
```go
// Cluster() function signature update
func Cluster(ctx context.Context, logger log.Logger, p providers.Provider, opts *ClusterOptions) error {
    // ...
    actionsContext := actions.NewActionContext(ctx, logger, status, p, opts.Config)
    // ...
}
```

**Step 3 — Addon actions use `ctx.CommandContext` instead of `node.Command`:**
```go
// In installcertmanager/certmanager.go — BEFORE:
if err := node.Command("kubectl", ...).Run(); err != nil { ... }

// AFTER:
if err := node.CommandContext(ctx.Context, "kubectl", ...).Run(); err != nil { ... }
```

**Step 4 — `waitforready.go` tryUntil loop:**

The `tryUntil` function uses `time.Sleep` and time-based cancellation. It needs context awareness:
```go
func waitForReady(ctx context.Context, node nodes.Node, until time.Time, selectorLabel string) bool {
    return tryUntil(ctx, until, func() bool { ... })
}

func tryUntil(ctx context.Context, until time.Time, try func() bool) bool {
    for until.After(time.Now()) {
        select {
        case <-ctx.Done():
            return false
        default:
        }
        if try() {
            return true
        }
        time.Sleep(500 * time.Millisecond)
    }
    return false
}
```

**Callers:** `create.go` calls `internalcreate.Cluster(...)`. The public API `pkg/cluster/provider.go` calls `internalcreate.Cluster(p.logger, p.provider, opts)`. The context must be threaded from `provider.go:Create()` through to `internalcreate.Cluster()`. Public API change:
```go
// pkg/cluster/provider.go
func (p *Provider) Create(name string, options ...CreateOption) error {
    // ...
    return internalcreate.Cluster(context.Background(), p.logger, p.provider, opts)
}
```

Or accept a context parameter on `Create()` — which is a public API change. Start with `context.Background()` in `Create()` and document that a future API version will accept caller-provided contexts.

**Files modified:**
- `pkg/cluster/internal/create/actions/action.go` — add Context field + param
- `pkg/cluster/internal/create/create.go` — accept ctx, thread to NewActionContext
- `pkg/cluster/provider.go` — pass `context.Background()` to Cluster()
- All addon `Execute()` methods — use `ctx.Context` with `CommandContext`
- `pkg/cluster/internal/create/actions/waitforready/waitforready.go` — context-aware tryUntil

---

### Improvement 4: Parallel Addon Installation

**Problem:** Seven addons run sequentially. `CertManager` takes 2-3 minutes (300s timeout). Some addons are independent and could run concurrently.

**Dependency graph for current addons:**
```
LocalRegistry   (no deps — uses containerd/exec path, independent of k8s API)
MetalLB         (no deps — applies manifest + waits for deployment)
MetricsServer   (no deps — applies manifest only)
CoreDNSTuning   (no deps — reads+patches ConfigMap)
Dashboard       (no deps — applies manifest)
CertManager     (no deps — applies manifest + waits 300s for webhook)
EnvoyGateway    (depends on MetalLB for LoadBalancer IPs — but only at runtime, not at install time)
```

The MetalLB/EnvoyGateway dependency is a runtime dependency (MetalLB must provide IPs for EG services), not an install-time dependency (both manifests can be applied independently). The current code already warns about this rather than enforcing order. Therefore, all seven addons can be installed in parallel.

**Design: Two-phase addon execution within `create.go`**

Phase 1 (sequential, existing): Core Kubernetes setup — `loadbalancer`, `config`, `kubeadminit`, `installcni`, `installstorage`, `kubeadmjoin`, `waitforready`.

Phase 2 (parallel, new): Addon installation — all seven addon actions run concurrently via goroutines.

**Implementation using `errgroup`-style pattern (matching existing `errors.AggregateConcurrent` in the codebase):**

```go
// create.go — replace sequential runAddon loop with:
var mu sync.Mutex
var addonResults []addonResult

var wg sync.WaitGroup
for _, entry := range actions.Registry(opts.Config) {
    entry := entry // capture loop variable (Go < 1.22)
    wg.Add(1)
    go func() {
        defer wg.Done()
        var result addonResult
        if !entry.Enabled(opts.Config) {
            result = addonResult{name: entry.Name, enabled: false}
        } else {
            err := entry.New().Execute(actionsContext)
            if err != nil {
                logger.Warnf("Addon %s failed: %v", entry.Name, err)
            }
            result = addonResult{name: entry.Name, enabled: true, err: err}
        }
        mu.Lock()
        addonResults = append(addonResults, result)
        mu.Unlock()
    }()
}
wg.Wait()
```

**Status spinner conflict:** `cli.Status.Start()` and `Status.End()` are not concurrency-safe — two goroutines calling `Start()` simultaneously will overwrite each other's status string. Resolution options:
1. Each addon action creates its own `cli.Status` instance (not safe either — the spinner is shared)
2. Addon actions skip `ctx.Status.Start/End` and use `ctx.Logger` only
3. A channel-based serialized status printer receives messages from goroutines

Recommended: Option 2 for the parallel phase — addon actions log their progress via `ctx.Logger.V(0).Infof(...)` rather than the spinner status. The addon summary (`logAddonSummary`) already prints a clean final state. The spinner is most useful for the sequential core phase where one step runs at a time.

**Files modified:**
- `pkg/cluster/internal/create/create.go` — replace sequential loop with goroutine fan-out
- `pkg/cluster/internal/create/actions/action.go` — no change needed (ActionContext is safe to share read-only; Nodes() cache uses RWMutex already)
- Addon actions — replace `ctx.Status.Start/End` with logger calls in Execute(), or guard the status with a mutex

**Thread safety of ActionContext:** `ActionContext.Nodes()` is already protected by `cachedData.mu` (sync.RWMutex). `ActionContext.Logger`, `Status`, `Config`, `Provider` are read-only during addon execution. The only unsafe access is `ctx.Status.Start/End` — which must be removed from parallel addon actions.

---

### Improvement 5: Structured JSON Output

**Problem:** `create.go` produces human-readable log output only. Scripting and tooling need machine-readable output.

**Scope:** Two output formats must coexist. Human format (existing) via `logger.V(0).Infof(...)`. JSON format when `--output=json` flag is set.

**Design: Output mode in ClusterOptions + JSON summary at the end**

```go
// pkg/cluster/internal/create/create.go
type ClusterOptions struct {
    // ... existing fields ...
    OutputFormat string // "human" (default) or "json"
}

// JSON output struct
type ClusterOutput struct {
    ClusterName string        `json:"clusterName"`
    KubeConfig  string        `json:"kubeconfig"`
    Addons      []AddonOutput `json:"addons"`
}

type AddonOutput struct {
    Name    string `json:"name"`
    Enabled bool   `json:"enabled"`
    Success bool   `json:"success"`
    Error   string `json:"error,omitempty"`
}
```

**Integration point — `create.go` end of `Cluster()` function:**
```go
if opts.OutputFormat == "json" {
    out := buildClusterOutput(opts.Config.Name, opts.KubeconfigPath, addonResults)
    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    _ = enc.Encode(out)
} else {
    logAddonSummary(logger, addonResults)
    if opts.DisplayUsage { logUsage(logger, ...) }
    if opts.DisplaySalutation { logSalutation(logger) }
}
```

**CLI flag integration in `pkg/cmd/kind/create/cluster/createcluster.go`:**
```go
type flagpole struct {
    // ... existing ...
    Output string // new: "human" or "json"
}
// --output flag registered in NewCommand()
// passed as opts.OutputFormat
```

**Files modified:**
- `pkg/cluster/internal/create/create.go` — add OutputFormat field, JSON branch at end
- `pkg/cmd/kind/create/cluster/createcluster.go` — add `--output` flag

**Files NOT modified:** Logger, Status, action packages — JSON output is a post-execution summary, not per-step output.

**What JSON does NOT cover:** Per-step progress. The human-readable spinner and per-step messages continue during execution. JSON is emitted once at the end as a machine-readable summary (cluster name, kubeconfig path, per-addon result). This sidesteps the "quiet mode vs. progress mode" conflict.

---

### Improvement 6: Unit Tests for Addon Actions That Call kubectl

**Problem:** Addon actions call `node.Command("kubectl", ...)` which shells out to a real container runtime. These cannot be unit tested in the traditional sense.

**Existing solution (precedent in the codebase):** Extract the business logic that does NOT call kubectl into a pure function, test that function directly.

**Evidence from existing tests:**
- `installcorednstuning/corefile_test.go` — tests `patchCorefile()` and `indentCorefile()` (pure string functions, no kubectl)
- `installmetallb/subnet_test.go` — tests `parseSubnetFromJSON()` and `carvePoolFromSubnet()` (pure parse/compute functions, no kubectl)
- `waitforready/waitforready_test.go` — tests `tryUntil()` (pure timing function, no kubectl)

**The pattern:** Each addon action has an `Execute()` method that calls kubectl, and zero or more pure helper functions. Tests cover only the helpers.

**Where unit tests fit for each addon:**

| Addon | Existing testable logic | Extractable for testing |
|-------|------------------------|------------------------|
| `installmetallb` | `parseSubnetFromJSON`, `carvePoolFromSubnet` | Already tested |
| `installcorednstuning` | `patchCorefile`, `indentCorefile` | Already tested |
| `waitforready` | `tryUntil`, `formatDuration` | Already tested |
| `installenvoygw` | No pure helpers currently | Extract: `buildGatewayClassYAML()` |
| `installlocalregistry` | No pure helpers currently | Extract: `buildHostsTOML(registryName, port string) string` |
| `installcertmanager` | No pure helpers currently | Extract: `deploymentNames() []string` or test manifest embedding |
| `installmetricsserver` | No pure helpers currently | Little to extract (single manifest apply) |
| `installdashboard` | No pure helpers currently | Little to extract (single manifest apply) |
| `installstorage` | Unknown | Depends on implementation |

**Recommended extraction pattern for `installenvoygw`:**

```go
// envoygw.go — before: inline YAML string
// After: extract into testable function
func gatewayClassManifest(controllerName string) string {
    return fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: %s
`, controllerName)
}
```

Test:
```go
// envoygw_test.go
func TestGatewayClassManifest(t *testing.T) {
    got := gatewayClassManifest("gateway.envoyproxy.io/gatewayclass-controller")
    if !strings.Contains(got, "controllerName: gateway.envoyproxy.io") {
        t.Error("missing controllerName")
    }
}
```

**For `installlocalregistry`:**
```go
// Extract the TOML generation
func buildHostsTOML(registryName string) string {
    return fmt.Sprintf(`[host."http://%s:5000"]\n`, registryName)
}
```

**What cannot be unit tested** (without a test double for `nodes.Node`):
- `Execute()` method top-level flow
- `kubectl apply` calls
- `kubectl wait` calls

These require integration tests (e.g., using a real kind cluster) and are out of scope for unit testing. The right split is: extract pure logic → unit test it; leave kubectl orchestration in `Execute()` → cover with integration tests.

**Test file placement:** Same package as the source file (white-box testing), e.g., `installmetallb/subnet_test.go` is `package installmetallb`. This gives access to unexported helpers.

---

## Data Flow

### Addon Registry Flow (New)

```
create.go: Cluster()
    |
    v
actions.Registry(opts.Config)  →  returns []AddonEntry in dependency order
    |
    v
[goroutine per entry]
    |
    +-- entry.Enabled(opts.Config) == false → skip
    +-- entry.Enabled(opts.Config) == true  → entry.New().Execute(actionsContext)
    |                                              |
    |                                    node.CommandContext(ctx.Context, "kubectl", ...)
    |                                              |
    |                                    common.nodeCmd.Run()
    |                                              |
    |                                    exec.CommandContext(ctx, binaryName, "exec", ...)
    |
    +-- addonResult → collected via mutex
    |
    v
wg.Wait()
    |
    v
logAddonSummary() / JSON output
```

### Version Package Data Flow (Fix)

```
BEFORE:
pkg/cluster/provider.go
    → import "sigs.k8s.io/kind/pkg/cmd/kind/version"
    → version.DisplayVersion()                    (WRONG: library imports CLI)

AFTER:
pkg/cluster/provider.go
    → import "sigs.k8s.io/kind/pkg/internal/kindversion"
    → kindversion.DisplayVersion()                (CORRECT: library imports internal)

pkg/cmd/kind/version/version.go
    → import "sigs.k8s.io/kind/pkg/internal/kindversion"
    → re-exports Version(), DisplayVersion()      (thin wrapper)
    → NewCommand() stays here                     (CLI concern)

pkg/cmd/kind/root.go
    → import "sigs.k8s.io/kind/pkg/cmd/kind/version"
    → version.NewCommand()                        (UNCHANGED)
```

### Context Propagation Data Flow (New)

```
CLI: createcluster.go: opts.Context = cmd.Context() or context.Background()
    |
    v
pkg/cluster/provider.go: Create(name, options...)
    → internalcreate.Cluster(ctx, logger, provider, opts)
    |
    v
create.go: Cluster(ctx context.Context, ...)
    → actions.NewActionContext(ctx, logger, status, provider, cfg)
    |
    v
ActionContext.Context  ← available in every Execute() call
    |
    v
node.CommandContext(ctx.Context, "kubectl", "wait", ...)
    |
    v
common.nodeCmd{ctx: ctx}.Run()
    |
    v
exec.CommandContext(ctx, binaryName, "exec", ..., "kubectl", "wait", ...)
    (process killed on ctx.Done())
```

---

## Recommended Project Structure

```
pkg/
├── internal/
│   └── kindversion/           ← NEW: moved from cmd/kind/version/
│       └── version.go         ← Version(), DisplayVersion(), version vars
│
├── cluster/
│   ├── provider.go            ← MODIFY: import kindversion instead of cmd/kind/version
│   │
│   └── internal/create/
│       ├── create.go          ← MODIFY: context, parallel addons, JSON output
│       └── actions/
│           ├── action.go      ← MODIFY: add Context field to ActionContext
│           ├── addon_registry.go  ← NEW: Registry() func returning []AddonEntry
│           │
│           ├── installenvoygw/
│           │   ├── envoygw.go
│           │   └── envoygw_test.go   ← NEW: test gatewayClassManifest
│           │
│           ├── installlocalregistry/
│           │   ├── localregistry.go
│           │   └── localregistry_test.go   ← NEW: test buildHostsTOML
│           │
│           └── [all other addon pkgs — CommandContext instead of Command]
│
└── cmd/kind/
    ├── root.go                ← UNCHANGED
    ├── version/
    │   └── version.go         ← MODIFY: delegate to kindversion, keep NewCommand
    └── create/cluster/
        └── createcluster.go   ← MODIFY: add --output flag
```

---

## Patterns to Follow

### Pattern 1: Extract-and-Test (existing precedent)

**What:** Split `Execute()` into (a) pure logic functions with no I/O, and (b) kubectl orchestration. Test (a) directly. Accept that (b) requires integration tests.

**When:** Any addon action with non-trivial logic (string transforms, IP parsing, YAML generation).

**Example — existing pattern in `installcorednstuning`:**
```go
// corednstuning.go
func (a *action) Execute(ctx *actions.ActionContext) error {
    // ...kubectl get configmap...
    corefileStr, err = patchCorefile(corefileStr)  // pure function
    // ...kubectl apply...
}

// Tested in corefile_test.go — no kubectl needed
func TestPatchCorefile(t *testing.T) { ... }
```

### Pattern 2: AddonEntry Registry (new)

**What:** A slice of `AddonEntry` structs replaces the hard-coded `runAddon` calls. The registry owns all addon imports. Adding a new addon = add one entry to the slice.

**When:** Any new addon addition, or when refactoring create.go.

**Trade-off:** Slightly less explicit than named `runAddon` calls, but eliminates the need to edit `create.go` for new addons.

### Pattern 3: context.Context via CommandContext (existing infrastructure, new usage)

**What:** Replace `node.Command(...)` with `node.CommandContext(ctx.Context, ...)` in addon `Execute()` methods. The underlying `common.nodeCmd` and `exec.CommandContext` already handle cancellation.

**When:** Any blocking kubectl call in an addon action.

**Trade-off:** Minor verbosity increase in each addon action. No behavior change when context is `context.Background()` (which it will be until caller-provided contexts are exposed via the public API).

### Pattern 4: binaryName Parameterization (existing, unchanged)

**What:** All container runtime calls use the provider's binary name string. No hardcoding.

**When:** Any new code that shells out to a container runtime. Covered by existing `common/node.go`.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Testing Execute() Directly Without a Real Cluster

**What people do:** Write `TestExecute()` that calls `action.Execute(actionsContext)` with a real `ActionContext`.

**Why it's wrong:** `Execute()` calls `node.Command("kubectl", ...)`, which execs into a container that does not exist in a unit test environment. The test fails with "no such container."

**Do this instead:** Extract logic from `Execute()` into pure functions. Test those functions. Accept that `Execute()` itself is an integration concern.

### Anti-Pattern 2: Sharing cli.Status Across Goroutines Without Synchronization

**What people do:** Run addon goroutines that each call `ctx.Status.Start("Installing X")`.

**Why it's wrong:** `cli.Status` is not concurrency-safe. `Start()` sets `s.status` and potentially starts a spinner. Concurrent calls overwrite each other.

**Do this instead:** Addon actions in the parallel phase use `ctx.Logger.V(0).Infof(...)` directly. The spinner-based status is reserved for the sequential core phase. Alternatively, wrap `Status` with a mutex — but simpler to just use the logger in parallel code.

### Anti-Pattern 3: Moving Version Constants Without Updating Build Injection

**What people do:** Move `versionCore`, `versionPreRelease`, `gitCommit`, `gitCommitCount` vars to `kindversion` but forget to update the `-ldflags` linker flags in the build system.

**Why it's wrong:** The build system injects version strings at link time using `-X sigs.k8s.io/kind/pkg/cmd/kind/version.gitCommit=<hash>`. If the vars move to a new package, the `-X` flag must reference the new full package path.

**Do this instead:** Update `Makefile` / `hack/build.sh` linker flags to reference `pkg/internal/kindversion.gitCommit` etc. immediately after the move. Verify with `kinder version` that the hash is set correctly in a built binary.

### Anti-Pattern 4: Exposing context.Context in the Public API Prematurely

**What people do:** Change `provider.go:Create(name string, options ...CreateOption)` to `Create(ctx context.Context, name string, options ...CreateOption)`.

**Why it's wrong:** This is a breaking API change. Current callers of `kind`-the-library would fail to compile.

**Do this instead:** Pass `context.Background()` from `provider.go:Create()` internally. Document that a future version will support caller-provided contexts. Introduce `CreateWithContext(ctx, name, options...)` as an addition rather than a modification.

### Anti-Pattern 5: Removing Dependency Order Comments When Adding the Registry

**What people do:** Convert the sequential list to a registry and remove the comments that document why LocalRegistry comes before MetalLB, etc.

**Why it's wrong:** The ordering represents implicit dependencies (LocalRegistry injects containerd config before cluster is fully up; MetalLB before EnvoyGateway for LoadBalancer IPs at runtime). Without comments, future maintainers reorder incorrectly.

**Do this instead:** Add a doc comment on the `Registry()` function explaining dependency ordering, and add inline comments on entries that have ordering constraints.

---

## Integration Points: New vs. Modified

### Modified Files

| File | Nature of Change |
|------|-----------------|
| `pkg/cluster/internal/create/actions/action.go` | Add `Context context.Context` field; add ctx param to `NewActionContext` |
| `pkg/cluster/internal/create/create.go` | Accept `context.Context` param; use addon registry loop; goroutine fan-out for parallel phase; JSON output branch |
| `pkg/cluster/provider.go` | Change import from `cmd/kind/version` to `internal/kindversion`; pass `context.Background()` to `Cluster()` |
| `pkg/cmd/kind/version/version.go` | Delegate `Version()`, `DisplayVersion()` to `kindversion`; keep `NewCommand()` here |
| `pkg/cmd/kind/create/cluster/createcluster.go` | Add `--output` flag; pass `OutputFormat` to `ClusterOptions` |
| All addon `Execute()` methods | Replace `node.Command(...)` with `node.CommandContext(ctx.Context, ...)` |
| `pkg/cluster/internal/create/actions/waitforready/waitforready.go` | Context-aware `tryUntil()` loop |
| Makefile / build scripts | Update `-X` linker flags to point to `pkg/internal/kindversion.*` vars |

### New Files

| File | Purpose |
|------|---------|
| `pkg/internal/kindversion/version.go` | Version string logic, build-time vars — library-accessible |
| `pkg/cluster/internal/create/actions/addon_registry.go` | `AddonEntry` type, `Registry()` function with ordered addon list |
| `pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go` | Unit tests for extractable pure logic |
| `pkg/cluster/internal/create/actions/installlocalregistry/localregistry_test.go` | Unit tests for extractable pure logic |

---

## Build Order Recommendation

Dependencies between improvements determine order. Each step is independently verifiable.

```
Step 1: Version package move (no dependencies on other steps)
    1a. Create pkg/internal/kindversion/version.go
        - Copy Version(), DisplayVersion(), versionCore, versionPreRelease, gitCommit, gitCommitCount
        - Copy internal version() helper and truncate() helper
        - Copy their tests into pkg/internal/kindversion/version_test.go
    1b. Update pkg/cluster/provider.go import
    1c. Update pkg/cmd/kind/version/version.go to delegate
    1d. Update build system -X linker flags
    1e. go build ./... && go test ./...

Step 2: context.Context in ActionContext (prerequisite for Steps 3, 4)
    2a. Add Context field to ActionContext, update NewActionContext signature
    2b. Update create.go: Cluster() accepts ctx, passes to NewActionContext
    2c. Update provider.go: Create() passes context.Background()
    2d. go build ./...  (must compile — all callers of NewActionContext must be updated)

Step 3: Context propagation to addon actions (depends on Step 2)
    3a. Update waitforready.go: context-aware tryUntil
    3b. Update each addon Execute(): CommandContext instead of Command
    3c. go build ./... && go test ./pkg/cluster/internal/create/actions/...

Step 4: Addon registry (depends on Step 2; independent of Step 3)
    4a. Create addon_registry.go with AddonEntry type and Registry()
    4b. Update create.go: replace 7 runAddon calls with registry loop
    4c. go build ./... (verify no import cycle: actions/ imports sub-packages)

Step 5: Parallel addon installation (depends on Steps 3 and 4)
    5a. Wrap registry loop in goroutines with sync.WaitGroup + mutex
    5b. Remove ctx.Status.Start/End from addon Execute() methods; use logger
    5c. go build ./... && go test ./...
    5d. Manual test: kinder create cluster (observe parallel output)

Step 6: JSON output (independent of Steps 1-5; can be done any time after Step 2)
    6a. Add OutputFormat to ClusterOptions
    6b. Add --output flag to createcluster.go
    6c. Add JSON output branch at end of create.go Cluster()
    6d. go build ./... && kinder create cluster --output=json

Step 7: Unit tests for addon actions (independent; can be done any time)
    7a. Extract pure logic from installenvoygw, installlocalregistry
    7b. Write _test.go files using the corefile_test.go pattern
    7c. go test ./pkg/cluster/internal/create/actions/...
```

---

## Scalability Considerations

| Concern | Now | After Parallel Addons | After Registry Pattern |
|---------|-----|-----------------------|----------------------|
| Adding a new addon | Edit create.go + add pkg | Same | Add 1 AddonEntry to registry |
| Number of addons | 7 sequential | 7 parallel | N parallel |
| Create time (all addons) | CertManager 300s is serial bottleneck | Dominated by slowest addon | Same |
| Test coverage | 3 action packages tested | Same + 2 more | Same |

---

## Sources

All findings from direct codebase read at `/Users/patrykattc/work/git/kinder` — HIGH confidence.

Key files examined:
- `pkg/cluster/internal/create/create.go` — current pipeline and addon calls
- `pkg/cluster/internal/create/actions/action.go` — ActionContext definition
- `pkg/cluster/internal/create/actions/{installmetallb,installenvoygw,installlocalregistry,installcertmanager,installcorednstuning}/*.go` — addon action implementations
- `pkg/cluster/internal/create/actions/waitforready/waitforready.go` — blocking retry loop
- `pkg/cluster/internal/create/actions/{installcorednstuning,installmetallb,waitforready}/*_test.go` — existing test patterns
- `pkg/cluster/internal/providers/common/node.go` — CommandContext already implemented
- `pkg/exec/types.go`, `pkg/exec/local.go` — Cmder interface + CommandContext exists
- `pkg/cluster/nodes/types.go` — Node interface (exec.Cmder embedded)
- `pkg/cluster/provider.go` — layer violation site
- `pkg/cmd/kind/version/version.go` + `version_test.go` — version package structure
- `pkg/internal/apis/config/types.go` — internal Addons struct
- `pkg/apis/config/v1alpha4/types.go` — public Addons struct

---
*Architecture research for: kinder code quality milestone — addon registry, version layer fix, context propagation, parallel addons, JSON output, unit tests*
*Researched: 2026-03-03*
