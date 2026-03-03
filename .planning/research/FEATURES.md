# Feature Research

**Domain:** Kubernetes local development CLI tool — code quality and new capabilities milestone
**Researched:** 2026-03-03
**Confidence:** HIGH (codebase direct analysis + official docs + current tooling patterns)

---

## Context and Scope

This research answers five specific questions about feature patterns for adding to the **existing** kinder codebase:

1. Parallel addon/component installation with dependency ordering
2. Structured JSON output (`--output=json`) for CLI commands
3. Cluster presets/profiles
4. Addon self-registration registry patterns
5. Unit testing patterns for code that calls kubectl/applies manifests

The existing implementation status:
- 7 addons install **sequentially** in `create.go` via a `runAddon()` helper loop
- CLI output is **human-only** (`fmt.Fprintf` / `logger.V(0).Infof`)
- **No presets** — users must write full YAML config files
- **Addon registration is hard-coded** in 4 places: `create.go` imports, `create.go` `runAddon()` calls, `config/v1alpha4/types.go` `Addons` struct, `internal/apis/config/types.go` `Addons` struct
- **Unit tests exist only for pure functions** (subnet math, corefile text manipulation, log message formatting, timer logic) — no tests for the addon `Execute()` path that calls `node.Command()`

---

## Feature Landscape

### Table Stakes (Users Expect These)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| JSON output for `kinder get clusters` and `kinder env` | Any tool used in CI/CD scripts must be machine-readable; users pipe output to `jq`; `--output=json` is the kubectl convention | LOW | Add `--output` flag defaulting to `text`; marshal struct to `encoding/json`; no new dependencies; `kinder env` already has structured data |
| JSON output for `kinder doctor` | Automated setup scripts need parseable pass/fail per check; human text can't be reliably parsed | LOW | Same pattern: `--output json` marshals `[]result` struct; exit codes stay the same |
| Unit tests for addon `Execute()` paths | Go projects with untested execution paths cannot be safely refactored; when adding parallel execution, tests prevent regressions; reviewers expect test coverage | HIGH | Requires mock `Node` interface implementing `exec.Cmder`; the `Node` type IS an interface already (in `pkg/cluster/nodes/types.go`) — this is the key insight; no monkey-patching needed |
| Preset `--profile minimal` (no addons) | Developers creating scratch clusters for kubeadm testing don't want MetalLB/cert-manager/etc.; zero-addon cluster should be one flag, not a full YAML file | LOW | Named preset = pre-built `Addons{}` struct value; `--profile` flag applied before config is parsed; 3–5 built-in presets cover 90% of use cases |
| Addon install summary with timing | Users waiting 3–5 minutes for addon installs want to know "which one is slow?"; `* MetalLB installed (8s)` vs `* cert-manager installed (47s)` | LOW | Already have `logAddonSummary()`; add `time.Since(start)` per addon; no arch change |

### Differentiators (Competitive Advantage)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Parallel addon installation with dependency graph | cert-manager and MetalLB have no dependencies on each other; installing them concurrently cuts total create time by ~40–50s; no comparable kind-based tool does this | MEDIUM | Use `errgroup` from `golang.org/x/sync`; model dependencies as `[]string` field on each addon descriptor; toposort into waves; run each wave with `errgroup.Go()`; thread-safety concern: `cli.Status` and `logger` must be goroutine-safe |
| Addon self-registration via `init()` | Adding a new addon currently requires edits in 4 files; self-registration cuts this to 1 file (the new addon package); minikube uses a dual-registry approach; the Go `init()` + map pattern is the standard for this | MEDIUM | Central `registry.go` with `var registry map[string]AddonDescriptor`; each addon package calls `Register()` in `init()`; blank imports in `create.go` trigger registration; `Addons` struct becomes `map[string]bool` or is generated from registry; this is the most architecturally significant change |
| Preset `--profile gateway` (MetalLB + Envoy + cert-manager) | API gateway development workflow is the most common advanced use case; a single flag enabling the right 3 addons is a DX win | LOW | Extends the same preset map; just an `Addons{}` with `EnvoyGateway: true, MetalLB: true, CertManager: true` |
| Preset `--profile ci` (no dashboard, no registry) | CI environments don't need UI or push targets; a CI preset disables the heavy addons that slow cluster creation | LOW | Same preset map; `Dashboard: false, LocalRegistry: false`; headless cluster creation |
| Architecture: `--addons` flag as CLI override | minikube supports `minikube start --addons=ingress,dashboard`; kinder could support `kinder create cluster --addons=metallb,cert-manager` to override config-file defaults | MEDIUM | Parses comma-separated list; maps to `Addons` struct fields; must be composable with `--profile` |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Full DAG-based parallel with cycle detection | Terraform/helmfile do this for large dependency graphs | Kinder has 7 addons with shallow dependencies; a 50-line DAG implementation for 7 nodes is over-engineering; cycle detection adds 200+ lines for zero real benefit | Use wave-based parallelism: pre-defined dependency layers (wave 1: MetalLB + LocalRegistry; wave 2: EnvoyGateway + CertManager; etc.); no runtime graph construction needed |
| Plugin system for external addons | Users want to add their own addons without rebuilding | Requires plugin ABI stability, versioning, security review, documentation; this is a 2+ sprint feature | Provide clear guidelines for contributing addons upstream; the `init()` self-registration pattern makes adding new addons trivial for contributors |
| Async (non-blocking) addon install | "Don't wait, let me use the cluster while addons install" | MetalLB must be ready before Envoy Gateway needs LoadBalancer IPs; cert-manager webhook must be ready before Certificate resources work; async install violates these dependencies silently | Parallel WITHIN dependency waves, synchronous BETWEEN waves; faster without the complexity or silent failure modes |
| Dynamic output format switching per subcommand | Different `--format` options for each command (e.g., `--format wide` for nodes, `--format json` for env) | Inconsistent flag names across commands confuse users; kubectl standardized on `-o`/`--output` | Use `--output` (matching kubectl convention) with values `text|json`; keep it to two modes only |
| YAML output (`--output=yaml`) | YAML is human-readable structured output | kinder's output types are not Kubernetes resources; marshaling to YAML adds a gopkg.in/yaml.v2 dependency for minimal benefit; `jq` works on JSON anyway | JSON only; document `yq` for users who need YAML |
| Interactive TUI for addon selection | "Let me check boxes for which addons I want" | Requires terminal capability detection, arrow-key handling, escape codes; fundamentally incompatible with CI usage | Presets cover the 90% case; YAML config covers the 10% case |

---

## Feature Dependencies

```
Parallel Addon Installation
    requires──> Addon self-registration registry (to know which addons exist and their deps)
    OR
    requires──> Hard-coded wave definitions in create.go (simpler path, less arch change)
    requires──> goroutine-safe logger (verify: kind's logger IS goroutine-safe per sync.Mutex in cli.Status)
    requires──> goroutine-safe cli.Status (verify before enabling parallel)
    enhances──> Cluster creation time (40–50s faster for full addon suite)
    conflicts──> Sequential runAddon() loop in create.go (must be replaced)

Addon Self-Registration Registry
    requires──> New AddonDescriptor type (Name, Dependencies []string, Enabled func(*config.Addons) bool, Action func() actions.Action)
    requires──> Central registry.go file
    requires──> Each addon package gets an init() calling Register()
    requires──> Blank imports in create.go trigger init() calls
    enables──> Parallel installation (registry has dependency metadata)
    enables──> --addons CLI flag (registry knows addon names)
    conflicts──> Current hard-coded Addons struct (must be updated or generated)
    WARNING──> Breaking change to config API if Addons struct changes shape

JSON Output
    requires──> --output flag added to affected commands (get clusters, get nodes, env, doctor)
    requires──> Struct-based return values from underlying functions (already true for doctor/env)
    no dependency on──> Self-registration or parallel install

Cluster Presets
    requires──> --profile flag on `kinder create cluster`
    requires──> Pre-built Addons{} preset map in code
    no dependency on──> Self-registration or parallel install (presets just set Addons{} values)
    enhances──> UX of parallel install (presets give users fast paths to common configs)

Unit Tests for Addon Execute()
    requires──> Mock Node implementation (Node IS an interface; write fakeNode{} with FakeCmd behavior)
    requires──> Extract pure logic from Execute() for separate unit testing
    enhances──> Safety of parallel install refactor
    no conflict with──> Any other feature
```

### Dependency Notes

- **Self-registration is not required for parallel install.** Wave-based parallelism (hard-coded dependency layers) can be implemented without changing the addon registry architecture. This is the pragmatic path: ship parallel install first, self-registration as a follow-on.
- **JSON output is independent.** It can be added to any command without touching addon install or presets. Start here — lowest complexity, highest CI value.
- **Presets are independent.** A `--profile` flag with a `map[string]Addons{}` constant map takes ~50 lines. No architecture change needed.
- **Unit tests are independent.** Write `fakeNode{}` once; use it across all addon test files. The existing test in `installcorednstuning/corefile_test.go` proves the pattern: test pure functions without a running cluster. The addon `Execute()` can be split into pure functions + one integration entry point.
- **The safest order is: unit tests → JSON output → presets → parallel install → self-registration (optional)**. Each step is independently shippable and tested.

---

## How Similar Tools Implement These Features

### (1) Parallel Addon Installation with Dependency Ordering

**minikube:** Sequential. Each `EnableOrDisableAddon` callback runs synchronously. Dependency checks are pre-flight validations (e.g., `isRuntimeContainerd`), not ordering mechanisms. No parallel installation.

**helmfile:** Uses a true DAG — topological sort of release dependencies, then concurrent execution within dependency waves using goroutines. Overkill for 7 addons.

**Terraform:** Full DAG with up to N parallel operations (`-parallelism` flag). Each node unblocks downstream nodes when complete.

**Recommended pattern for kinder (LOW complexity, HIGH value):**

```go
// Wave-based parallel install — no DAG needed for 7 addons
type addonWave []struct {
    name    string
    enabled bool
    action  actions.Action
}

waves := []addonWave{
    // Wave 1: independent addons (no deps on each other)
    {{"Local Registry", cfg.Addons.LocalRegistry, installlocalregistry.NewAction()},
     {"MetalLB", cfg.Addons.MetalLB, installmetallb.NewAction()},
     {"Metrics Server", cfg.Addons.MetricsServer, installmetricsserver.NewAction()},
     {"CoreDNS Tuning", cfg.Addons.CoreDNSTuning, installcorednstuning.NewAction()}},
    // Wave 2: depends on Wave 1 (EnvoyGateway needs MetalLB for LB IPs)
    {{"Envoy Gateway", cfg.Addons.EnvoyGateway, installenvoygw.NewAction()},
     {"Cert Manager", cfg.Addons.CertManager, installcertmanager.NewAction()}},
    // Wave 3: depends on Wave 2 (Dashboard can start after networking is up)
    {{"Dashboard", cfg.Addons.Dashboard, installdashboard.NewAction()}},
}

for _, wave := range waves {
    g := errgroup.Group{}
    for _, a := range wave {
        a := a // capture
        g.Go(func() error {
            return runAddon(ctx, a.name, a.enabled, a.action)
        })
    }
    if err := g.Wait(); err != nil { /* warn and continue */ }
}
```

**Key concern:** `cli.Status` in `ActionContext` uses a spinner/progress indicator. Concurrent writes to it will interleave output. Solution: each goroutine gets its own status line, or use a mutex-protected aggregator, or suppress per-addon status and collect results.

**Confidence:** HIGH (pattern from helmfile/terraform; errgroup is stdlib-adjacent; wave approach is proven in Go CI tooling)

---

### (2) Structured JSON Output (`--output=json`)

**kubectl convention:** `-o json` / `--output=json`. Two modes only: human text and JSON. No YAML (for CLI output; YAML is for Kubernetes manifests).

**GitHub CLI pattern (from `cli/go-gh`):** Uses a `tableprinter` that detects TTY and switches between human-readable table and TSV automatically. For JSON: marshal a struct.

**Recommended pattern for kinder:**

```go
// In flagpole:
type flagpole struct {
    Name   string
    Output string // "text" or "json"
}

// In command setup:
cmd.Flags().StringVarP(&flags.Output, "output", "o", "text", "output format: text|json")

// In runE:
type clusterInfo struct {
    Provider  string   `json:"provider"`
    Name      string   `json:"name"`
    Kubeconfig string  `json:"kubeconfig"`
    // ...
}

if flags.Output == "json" {
    enc := json.NewEncoder(streams.Out)
    enc.SetIndent("", "  ")
    return enc.Encode(clusterInfo{...})
}
// else: fmt.Fprintf human text
```

**Commands to update:** `kinder get clusters`, `kinder get nodes`, `kinder env`, `kinder doctor`.

**Doctor JSON output structure:**
```json
{
  "checks": [
    {"name": "docker", "status": "ok", "message": ""},
    {"name": "kubectl", "status": "fail", "message": "kubectl not found — install from https://..."}
  ],
  "overall": "fail"
}
```

**Anti-pattern to avoid:** Adding `--json` bool flag (not `--output string`). The `--output string` form is composable (`--output=json` or `-o json`) and matches kubectl, making it intuitive for Kubernetes users.

**Confidence:** HIGH (directly observed in kubectl, gh CLI, AWS CLI; trivially implemented in Go with `encoding/json`)

---

### (3) Cluster Presets/Profiles

**minikube approach:** `minikube profile <name>` creates named clusters (isolated state directories). Each profile is a named cluster with its own config, not a template. Not what kinder needs.

**skaffold approach:** Named profiles that overlay the base config via JSON Patch. Activate with `-p profile-name`. Useful for build/deploy environments, not cluster topology.

**k3d approach:** YAML config files only. No named presets. Users write their own YAML and reference it with `--config`.

**What kinder needs:** Named shorthand for common `Addons{}` configurations. This is much simpler than minikube profiles or skaffold overlays.

**Recommended pattern for kinder:**

```go
// In pkg/cluster/presets.go (new file, ~50 lines):
var builtinProfiles = map[string]config.Addons{
    "full": {
        MetalLB: true, EnvoyGateway: true, MetricsServer: true,
        CoreDNSTuning: true, Dashboard: true, LocalRegistry: true,
        CertManager: true,
    },
    "minimal": {
        MetalLB: false, EnvoyGateway: false, MetricsServer: false,
        CoreDNSTuning: false, Dashboard: false, LocalRegistry: false,
        CertManager: false,
    },
    "gateway": {
        MetalLB: true, EnvoyGateway: true, CertManager: true,
        MetricsServer: true, CoreDNSTuning: true,
        Dashboard: false, LocalRegistry: false,
    },
    "ci": {
        MetalLB: true, MetricsServer: true, CoreDNSTuning: true,
        Dashboard: false, LocalRegistry: false,
        EnvoyGateway: false, CertManager: false,
    },
}

// In createcluster.go flagpole:
Profile string // --profile flag

// Applied after config load, before validation:
if flags.Profile != "" {
    preset, ok := builtinProfiles[flags.Profile]
    if !ok { return fmt.Errorf("unknown profile %q; available: full, minimal, gateway, ci") }
    opts.Config.Addons = preset
}
```

**Flag placement:** `kinder create cluster --profile minimal`. The `--profile` flag overrides the YAML config's `addons` block, so YAML addons + `--profile` interacts predictably (flag wins).

**Names matter:** "minimal", "full", "gateway", "ci" are self-documenting. Avoid "dev" (ambiguous) or "production" (local tool, no production).

**Confidence:** HIGH (pattern derived from how all similar tools handle shorthand configs; the implementation is simple)

---

### (4) Addon Self-Registration Registry Pattern

**minikube approach:** Two parallel maps in source code — `assets.Addons` (metadata + embedded manifests) and `addons.Addons` (validation + callbacks). Both maps are populated at package init time. New addons require editing both maps. The approach works but requires discipline.

**Go standard pattern (register-on-init):** Each plugin package has an `init()` function that calls `Register()` on a central registry. The main package imports the plugin packages with blank imports (`_ "..."`) to trigger `init()`. The registry is a `map[string]T` with a mutex for thread safety.

**Current kinder problem:** Adding an addon requires changes in:
1. `pkg/cluster/internal/create/create.go` — new import + new `runAddon()` call
2. `pkg/apis/config/v1alpha4/types.go` — new `*bool` field in `Addons` struct
3. `pkg/internal/apis/config/types.go` — new `bool` field in internal `Addons` struct
4. `pkg/internal/apis/config/convert_v1alpha4.go` — conversion code

**Recommended pattern for kinder (if pursuing self-registration):**

```go
// pkg/cluster/internal/create/addonregistry/registry.go
type AddonDescriptor struct {
    Name         string
    DefaultEnabled bool
    DependsOn    []string  // names of addons this one needs to run first
    NewAction    func() actions.Action
    IsEnabled    func(cfg *config.Cluster) bool  // reads from config
}

var (
    mu       sync.Mutex
    registry = map[string]*AddonDescriptor{}
)

func Register(d *AddonDescriptor) {
    mu.Lock()
    defer mu.Unlock()
    registry[d.Name] = d
}

func All() []*AddonDescriptor {
    mu.Lock()
    defer mu.Unlock()
    // return sorted copy
}

// pkg/cluster/internal/create/actions/installmetallb/metallb.go (new)
func init() {
    addonregistry.Register(&addonregistry.AddonDescriptor{
        Name:           "MetalLB",
        DefaultEnabled: true,
        DependsOn:      nil,
        NewAction:      NewAction,
        IsEnabled:      func(cfg *config.Cluster) bool { return cfg.Addons.MetalLB },
    })
}
```

**Honest assessment:** This is an architectural refactor, not a feature addition. The `Addons` struct in the config API is the core problem — it's a fixed set of named fields, not a dynamic map. Self-registration without changing the config API is half-measures (you still need to add the bool field). Full self-registration requires changing the config API to `Addons map[string]bool`, which is a breaking change to the public-facing YAML format.

**Pragmatic alternative:** Keep the config struct as-is. Move the `runAddon()` dispatch table into a single slice/map in `create.go` (consolidating from scattered calls). This reduces the 4-place edit to 2 places (struct + dispatch table) without a config API break.

**Confidence:** MEDIUM (pattern is well-established in Go; applicability to kinder's config-struct constraint requires architectural decision)

---

### (5) Unit Testing Patterns for Code That Calls kubectl/Applies Manifests

**The core problem in kinder:** Addon `Execute()` methods call `node.Command("kubectl", ...)`. The `node` is a `nodes.Node` interface. Interfaces in Go are trivially mockable without any framework.

**Existing kinder exec interface:**
```go
// pkg/exec/types.go — already exists
type Cmd interface {
    Run() error
    SetEnv(...string) Cmd
    SetStdin(io.Reader) Cmd
    SetStdout(io.Writer) Cmd
    SetStderr(io.Writer) Cmd
}

type Cmder interface {
    Command(string, ...string) Cmd
    CommandContext(context.Context, string, ...string) Cmd
}

// pkg/cluster/nodes/types.go — already exists
type Node interface {
    exec.Cmder   // <-- Node IS a Cmder; Command() returns a Cmd
    String() string
    Role() (string, error)
    IP() (string, string, error)
    SerialLogs(io.Writer) error
}
```

**Recommended pattern — write fakeNode in test files:**

```go
// In installmetallb/metallb_test.go:

type fakeCmd struct {
    args    []string
    stdin   string
    runErr  error
}

func (c *fakeCmd) Run() error                    { return c.runErr }
func (c *fakeCmd) SetEnv(...string) exec.Cmd     { return c }
func (c *fakeCmd) SetStdout(io.Writer) exec.Cmd  { return c }
func (c *fakeCmd) SetStderr(io.Writer) exec.Cmd  { return c }
func (c *fakeCmd) SetStdin(r io.Reader) exec.Cmd {
    b, _ := io.ReadAll(r)
    c.stdin = string(b)
    return c
}

type fakeNode struct {
    commands []*fakeCmd
    idx      int
}

func (n *fakeNode) Command(name string, args ...string) exec.Cmd {
    cmd := &fakeCmd{args: append([]string{name}, args...)}
    n.commands = append(n.commands, cmd)
    return cmd
}
// ... implement rest of Node interface returning zero values

// Test:
func TestMetalLBAction_AppliesManifest(t *testing.T) {
    node := &fakeNode{}
    // inject node into action context
    // call action.Execute()
    // assert node.commands[0].args contains "kubectl", "apply"
    // assert node.commands[0].stdin contains "metallb"
}
```

**What to test vs. not test:**

| Test Category | Test It | Don't Test It |
|--------------|---------|---------------|
| Manifest is applied (kubectl apply called) | YES | Exact manifest content (use contains check) |
| Wait conditions are issued | YES | kubectl output parsing |
| Error from kubectl propagates correctly | YES | Network/container state |
| Addon skipped when disabled in config | YES | — |
| Manifest embedded content is non-empty | YES | — |
| Actual cluster behavior | NO | NO |

**Existing proven patterns in kinder (HIGH confidence, direct observation):**
- `create_addon_test.go`: tests `logAddonSummary()` with `testLogger` — pure log output, no Node mock needed
- `subnet_test.go`: tests `parseSubnetFromJSON()` and `carvePoolFromSubnet()` — pure math, table-driven
- `corefile_test.go`: tests `patchCorefile()` — string manipulation, table-driven
- `waitforready_test.go`: tests `tryUntil()` — timing logic, no external calls

**Pattern: Extract pure logic first.** All existing tests in kinder test pure functions extracted from the action, not the action `Execute()` itself. This is correct — the Execute() method is an integration boundary. The right approach:

1. Extract logic from `Execute()` into pure functions (e.g., `buildMetalLBCR(subnet string) string`)
2. Unit test those pure functions (no mock needed — pure input/output)
3. For the `Execute()` itself, write a thin integration test using `fakeNode{}` to assert the sequence of kubectl calls

**k8s.io/utils/exec/testing.FakeExec:** Exists and works for code using `k8s.io/utils/exec.Interface`. Kinder uses its own `sigs.k8s.io/kind/pkg/exec` package with identical interface contracts — so `FakeExec` from k8s.io/utils cannot be used directly. Kinder needs its own `fakeNode{}` and `fakeCmd{}` implementations (25–30 lines total, shared across test files via a `testutil` package).

**Confidence:** HIGH (Node is a Go interface; fakeNode pattern is standard Go; existing tests confirm the project style)

---

## MVP Definition

### Must Ship (Next Milestone)

- [ ] **Unit tests for addon Execute() paths** — fakeNode + fakeCmd implementation; tests for MetalLB, CertManager, LocalRegistry, and the runAddon dispatch logic; prerequisite for safe parallel refactor
- [ ] **JSON output for `kinder env`** — `--output json` flag; marshal `EnvInfo` struct; no new dependencies
- [ ] **JSON output for `kinder doctor`** — `--output json` flag; marshal `[]result` struct; exit codes unchanged
- [ ] **Cluster presets via `--profile`** — `minimal`, `full`, `gateway`, `ci` presets; ~50 lines of new code; zero architecture change

### Add After Core Quality Is Proven

- [ ] **Wave-based parallel addon install** — requires unit tests in place first (to verify no regression); errgroup-based; wave definitions in `create.go`; timing output in addon summary
- [ ] **JSON output for `kinder get clusters` and `kinder get nodes`** — same pattern as env/doctor; lower priority because these are less CI-critical

### Future Consideration

- [ ] **Addon self-registration via init()** — only valuable if kinder grows beyond ~10 addons; requires config API decision (Addons map vs struct); architectural; defer until the pain is felt
- [ ] **`--addons` CLI flag** — comma-separated addon enable/disable; useful complement to presets; depends on how addon names are exposed; after registry pattern is settled
- [ ] **Pull-through cache for local registry** — high complexity; niche use case; defer

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Unit tests for addon Execute() | HIGH (enables safe refactoring) | MEDIUM (fakeNode pattern) | P1 |
| JSON output for env + doctor | HIGH (CI scripting) | LOW (encoding/json) | P1 |
| Cluster presets (`--profile`) | HIGH (DX, zero-config goal) | LOW (~50 lines) | P1 |
| Parallel addon install (wave-based) | MEDIUM (speed improvement) | MEDIUM (errgroup + wave design) | P2 |
| JSON output for get clusters/nodes | MEDIUM (scripting) | LOW | P2 |
| Addon self-registration | LOW (contributor DX only) | HIGH (arch change) | P3 |
| `--addons` CLI flag | MEDIUM (power user DX) | MEDIUM | P3 |

---

## Competitor Feature Analysis

| Feature | minikube | k3d | skaffold | kinder (current) | kinder (target) |
|---------|----------|-----|----------|-----------------|-----------------|
| Parallel addon install | No (sequential) | N/A (k3s bundles addons) | N/A (not a cluster tool) | No (sequential) | Yes (wave-based) |
| JSON output | `minikube status --output json` YES | `k3d cluster list --output json` YES | N/A | No | Yes (`--output json`) |
| Named presets/profiles | Yes (full cluster profiles, not addon presets) | No (YAML only) | Yes (build/deploy profiles) | No | Yes (`--profile minimal/full/gateway/ci`) |
| Addon self-registration | Yes (dual-registry map, compile-time) | N/A | N/A | No (4-place hard-code) | Optional future |
| Unit tests for addon installs | Limited (most tests are integration) | Limited | N/A | Limited (pure functions only) | Yes (fakeNode pattern) |
| `--addons` CLI flag | `minikube start --addons=dashboard,ingress` | No | N/A | No | Future |

---

## Sources

- Kinder codebase direct analysis (`pkg/cluster/internal/create/`, `pkg/cluster/nodes/`, `pkg/exec/`, `pkg/cmd/kind/`) — HIGH confidence (first-party source, read files directly)
- [minikube addon system architecture (DeepWiki)](https://deepwiki.com/kubernetes/minikube/5-addon-system) — MEDIUM confidence (derived docs; confirmed sequential execution)
- [minikube profile system (DeepWiki)](https://deepwiki.com/kubernetes/minikube/2.4-profile-and-configuration-management) — MEDIUM confidence (derived docs; confirmed file-based profiles, not addon presets)
- [k3d configuration system (DeepWiki)](https://deepwiki.com/k3d-io/k3d/3.2-advanced-configuration) — MEDIUM confidence (no named presets confirmed)
- [Skaffold profiles documentation](https://skaffold.dev/docs/environment/profiles/) — HIGH confidence (official docs; confirmed overlay pattern)
- [helmfile dependency ordering (DeepWiki)](https://deepwiki.com/helmfile/helmfile/3.3-dependencies) — HIGH confidence (confirmed DAG + wave pattern for parallel execution)
- [golang.org/x/sync errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) — HIGH confidence (official Go package)
- [k8s.io/utils/exec/testing FakeExec](https://pkg.go.dev/k8s.io/utils/exec/testing) — HIGH confidence (confirmed interface; not directly usable with kinder's own exec package but confirms pattern)
- Go init() self-registration pattern — HIGH confidence (standard Go idiom; multiple sources confirm)
- [b1-88er.github.io: mocking exec.Cmd in Go](https://b1-88er.github.io/posts/testing-exec-cmd-in-go/) — MEDIUM confidence (pattern applies; kinder's interface makes it simpler than the article's approach)

---

*Feature research for: kinder code quality and new capabilities milestone*
*Researched: 2026-03-03*
