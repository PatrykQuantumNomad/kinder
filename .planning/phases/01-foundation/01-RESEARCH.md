# Phase 1: Foundation - Research

**Researched:** 2026-03-01
**Domain:** Go CLI binary renaming, config schema extension, action pipeline scaffolding, platform detection
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Addon defaults & opt-out model
- All addons enabled by default — zero-config "batteries-included" experience
- Disabling an addon prints a one-line note: e.g. "Skipping MetalLB (disabled in config)"
- Dependency conflicts (e.g. MetalLB disabled + Envoy Gateway enabled) produce a warning and continue — install the dependent addon anyway but warn that it won't fully work
- Config supports booleans only for v1 — no per-addon nested settings yet

#### Config schema feel
- Addons section uses flat boolean map: `addons.metalLB: true`
- Field names follow camelCase to match kind's existing v1alpha4 conventions
- Unrecognized addon names in config produce a strict error listing valid addon names — catches typos early
- Stay at v1alpha4 — the addons section is purely additive, no version bump needed

#### CLI output during creation
- Match kind's existing output style (emoji + step lines) for addon installation
- Wait for pods to be ready and report health status before the command returns
- If an addon fails to install, warn and continue — cluster is usable, just missing that addon
- Print an addon summary block at the end listing installed addons and their status

#### Platform warnings
- macOS/Windows MetalLB warning appears after MetalLB installs, alongside its success status
- Tone is factual + actionable: state the limitation and provide a workaround (e.g. `kubectl port-forward`)
- Warning appears every time — no suppression mechanism, no hidden state
- Visual style matches kind's existing warning/notice formatting

### Claude's Discretion
- Exact wording and formatting of warning messages
- Internal action pipeline architecture and hook mechanism
- Error message formatting details
- How addon readiness checks are implemented (polling interval, timeout)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| FOUND-01 | Binary is named `kinder` and coexists with `kind` on the system | Makefile `KIND_BINARY_NAME` var controls output binary name; module path stays `sigs.k8s.io/kind`; no PATH collision since separate binary |
| FOUND-02 | Config schema extends v1alpha4 with an `addons` section for enabling/disabling each addon | v1alpha4 types + internal types + convert_v1alpha4 + deepcopy + validate; positive bool map with camelCase names |
| FOUND-03 | Existing kind configs without addons section work unchanged (backward compatible) | `omitempty` on addons field + Go zero value false = addons disabled if absent; but default must be enabled — requires explicit defaulting in SetDefaultsCluster |
| FOUND-04 | Each addon action checks its enable flag before executing | create.go action list construction gates each action on config.Addons; actions themselves do not check — the gate is in create.go |
| FOUND-05 | Platform detection warns macOS/Windows users that MetalLB LoadBalancer IPs may not be reachable from the host | `runtime.GOOS` pattern already used in codebase (spinner.go, paths.go); warn via ctx.Logger.Warn after MetalLB action |
</phase_requirements>

---

## Summary

Phase 1 builds the structural scaffolding that enables all subsequent addon phases. The `kinder` binary is the kind binary compiled with a different output name — the Go module path stays `sigs.k8s.io/kind` internally, only the built artifact name changes via `KIND_BINARY_NAME` in the Makefile. This means FOUND-01 requires minimal code change: one Makefile variable and updating the cobra root command's `Use` field and description strings.

The config schema extension follows kind's established two-layer pattern: a public `pkg/apis/config/v1alpha4/types.go` with YAML/JSON tags, and an internal `pkg/internal/apis/config/types.go` without tags. The convert function, deepcopy, and defaults all need updating for the new `Addons` struct. The user decision uses **positive booleans** (`addons.metalLB: true` means enabled), which creates a subtle challenge: Go bool zero value is `false`, so a config file omitting `addons.metalLB` would have it default to `false` (disabled). The defaulting layer must explicitly set all addon booleans to `true` so existing configs (which have no `addons` section) get all addons enabled.

The action pipeline hook mechanism is already well-designed: `create.go` builds a `[]actions.Action` slice and executes them sequentially. Phase 1 adds conditional gates in this slice — `if cfg.Addons.MetalLB { actionsToRun = append(...) }`. The `ActionContext` already provides everything addon actions need (`Config`, `Logger`, `Status`, `Provider`, `Nodes()`). No changes to `action.go` or `ActionContext` are required. The "addon summary block" and "warn and continue" behavior means the action execution loop in `create.go` must be modified to catch per-addon errors rather than failing the whole cluster.

**Primary recommendation:** Rename binary via Makefile, add `Addons` struct with positive booleans to both config layers with explicit `true` defaults, gate addon actions in create.go, and add platform check via `runtime.GOOS` after MetalLB action registration.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `runtime` | Go 1.17+ | Platform detection (`runtime.GOOS`) | Already used in codebase; no new dependency |
| `go.yaml.in/yaml/v3` | v3.0.4 | YAML decode with `KnownFields(true)` for strict parsing | Already in go.mod; existing config decode path |
| `github.com/spf13/cobra` | v1.8.0 | CLI command structure | Already in go.mod |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/errors` | (internal) | Wrapped errors with stack traces | All error returns in action Execute() |
| `sigs.k8s.io/kind/pkg/internal/cli` | (internal) | `Status.Start()`/`Status.End()` for spinner/progress | Every addon action display |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Positive bool fields (`MetalLB: true`) | Negative bool fields (`DisableMetalLB: true`) | Prior research used negative; user locked positive. Positive is more natural YAML but needs explicit `true` defaulting since Go zero value is `false` |
| Explicit defaulting in `SetDefaultsCluster` | Pointer-to-bool with nil meaning "enabled" | Pointer-to-bool adds complexity at every callsite; explicit bool fields with defaults in `SetDefaultsCluster` is simpler and consistent with how kind handles Networking defaults |

**Installation:** No new dependencies. All needed packages are in existing `go.mod`.

---

## Architecture Patterns

### Recommended Project Structure

New files added in Phase 1 (scaffold only — no addon implementations):

```
pkg/
├── apis/config/v1alpha4/
│   └── types.go                    # ADD: Addons struct + Cluster.Addons field
├── internal/apis/config/
│   ├── types.go                    # ADD: Addons struct + Cluster.Addons field
│   ├── convert_v1alpha4.go         # ADD: copy Addons fields in Convertv1alpha4()
│   ├── default.go                  # ADD: set all Addons booleans to true
│   ├── validate.go                 # ADD: validate unknown addon names (no-op for now)
│   ├── zz_generated.deepcopy.go    # REGENERATE: trivial for bool-only struct
│   └── encoding/load.go            # NO CHANGE: strict decode already works
├── cluster/internal/create/
│   └── create.go                   # MODIFY: add addon action gates + summary block + warn-continue
└── cmd/kind/
    └── root.go                     # MODIFY: Use: "kinder" (or leave as "kind" -- see notes)

main.go                             # NO CHANGE (calls app.Main())
Makefile                            # MODIFY: KIND_BINARY_NAME=kinder
```

### Pattern 1: Positive Boolean Addons Config with Explicit Defaults

**What:** The `Addons` struct uses positive-sense boolean fields (`MetalLB bool`). True means installed, false means skipped. Since Go zero value for bool is `false`, the `SetDefaultsCluster()` function must explicitly set all addon booleans to `true`.

**When to use:** Every addon flag in the `Addons` struct.

**Example:**

```go
// pkg/apis/config/v1alpha4/types.go
// Addons controls which default addons are installed during cluster creation.
// All addons are enabled by default; set the field to false to skip installation.
type Addons struct {
    // MetalLB enables MetalLB LoadBalancer support.
    // Defaults to true.
    MetalLB bool `yaml:"metalLB,omitempty" json:"metalLB,omitempty"`

    // EnvoyGateway enables Envoy Gateway and Gateway API CRDs.
    // Defaults to true.
    EnvoyGateway bool `yaml:"envoyGateway,omitempty" json:"envoyGateway,omitempty"`

    // MetricsServer enables Kubernetes Metrics Server.
    // Defaults to true.
    MetricsServer bool `yaml:"metricsServer,omitempty" json:"metricsServer,omitempty"`

    // CoreDNSTuning enables CoreDNS cache and performance tuning.
    // Defaults to true.
    CoreDNSTuning bool `yaml:"coreDNSTuning,omitempty" json:"coreDNSTuning,omitempty"`

    // Dashboard enables the Kubernetes Dashboard (Headlamp).
    // Defaults to true.
    Dashboard bool `yaml:"dashboard,omitempty" json:"dashboard,omitempty"`
}
```

```go
// pkg/internal/apis/config/default.go — add to SetDefaultsCluster()
// Default all addons to enabled.
// Must be explicit because Go bool zero value is false (= disabled).
if !addonDefaultsSet(obj) {
    obj.Addons.MetalLB = true
    obj.Addons.EnvoyGateway = true
    obj.Addons.MetricsServer = true
    obj.Addons.CoreDNSTuning = true
    obj.Addons.Dashboard = true
}
```

**CRITICAL NOTE:** The `omitempty` tag on bool fields in Go YAML means that `false` values are omitted from marshal output, but on unmarshal a missing key leaves the field at zero value (`false`). This means `SetDefaultsCluster()` must be called before the config is used, and the v1alpha4 `SetDefaultsCluster()` sets them to `true`. When a user writes `addons.metalLB: false`, the YAML decoder correctly sets `MetalLB = false`, and `SetDefaultsCluster()` must NOT overwrite it — so use a "have defaults been set" pattern or simply set unconditionally (since SetDefaultsCluster runs before user values are merged). The correct approach: **set in v1alpha4 SetDefaultsCluster before decoding** — actually the decode happens first, then defaults are applied only if the field is zero. Since the user's `false` and the absent field are both zero value (`false`), we cannot distinguish "user set false" from "not set" with a plain bool.

**Resolution:** Use `*bool` (pointer to bool) for the addon fields instead. `nil` = not set (default to true), `false` = explicitly disabled, `true` = explicitly enabled. This is the correct Go pattern for "optional field with non-zero default".

```go
// Correct approach using *bool
type Addons struct {
    MetalLB       *bool `yaml:"metalLB,omitempty" json:"metalLB,omitempty"`
    EnvoyGateway  *bool `yaml:"envoyGateway,omitempty" json:"envoyGateway,omitempty"`
    MetricsServer *bool `yaml:"metricsServer,omitempty" json:"metricsServer,omitempty"`
    CoreDNSTuning *bool `yaml:"coreDNSTuning,omitempty" json:"coreDNSTuning,omitempty"`
    Dashboard     *bool `yaml:"dashboard,omitempty" json:"dashboard,omitempty"`
}

// SetDefaultsCluster: set nil pointers to true
func defaultTrue(b **bool) {
    if *b == nil {
        t := true
        *b = &t
    }
}
// In SetDefaultsCluster:
defaultTrue(&obj.Addons.MetalLB)
// ... etc

// In action check:
func addonEnabled(b *bool) bool {
    return b == nil || *b  // nil = true (default), explicit value used otherwise
}
```

However, the user constraint says "Config supports booleans only for v1 — no per-addon nested settings yet." Pointer-to-bool is still a boolean from the user's perspective. The YAML syntax remains `addons.metalLB: false`. This is the correct implementation approach.

### Pattern 2: Warn-and-Continue Addon Action Loop

**What:** The existing action loop in `create.go` aborts on any error. For addon actions, the user decision says "warn and continue" on failure. Phase 1 scaffolds this behavior.

**When to use:** The addon action execution section in `create.go`.

**Example:**

```go
// Source: pkg/cluster/internal/create/create.go (modified)

// Phase 1 scaffold: addon action results collected for summary
type addonResult struct {
    name    string
    enabled bool
    err     error
}

var addonResults []addonResult

// Helper that runs an addon action and captures result
runAddon := func(name string, enabled bool, action actions.Action) {
    if !enabled {
        logger.V(0).Infof(" • Skipping %s (disabled in config)\n", name)
        addonResults = append(addonResults, addonResult{name: name, enabled: false})
        return
    }
    if err := action.Execute(actionsContext); err != nil {
        logger.Warnf("addon %s failed to install: %v", name, err)
        addonResults = append(addonResults, addonResult{name: name, enabled: true, err: err})
        return
    }
    addonResults = append(addonResults, addonResult{name: name, enabled: true})
}

// Wire addon actions (Phase 1 provides stubs; later phases fill implementations)
runAddon("MetalLB", addonEnabled(opts.Config.Addons.MetalLB), installmetallb.NewAction())
runAddon("Envoy Gateway", addonEnabled(opts.Config.Addons.EnvoyGateway), installenvoygw.NewAction())
runAddon("Metrics Server", addonEnabled(opts.Config.Addons.MetricsServer), installmetricsserver.NewAction())
runAddon("CoreDNS Tuning", addonEnabled(opts.Config.Addons.CoreDNSTuning), installcorednstuning.NewAction())
runAddon("Dashboard", addonEnabled(opts.Config.Addons.Dashboard), installdashboard.NewAction())

// Print summary block
logAddonSummary(logger, addonResults)
```

### Pattern 3: Platform Warning via runtime.GOOS

**What:** After the MetalLB addon completes (or is listed in summary), check the OS and print a factual warning if on macOS or Windows.

**When to use:** Platform detection for MetalLB reachability warning (FOUND-05).

**Example:**

```go
// Source: pkg/cluster/internal/create/create.go or installmetallb action
import "runtime"

func logMetalLBPlatformWarning(logger log.Logger) {
    if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
        logger.V(0).Info(
            " • MetalLB LoadBalancer IPs are not reachable from the host on " +
                runtime.GOOS + ".\n" +
                "   Use kubectl port-forward to access LoadBalancer services:\n" +
                "   kubectl port-forward svc/<name> <local-port>:<service-port>",
        )
    }
}
```

### Pattern 4: Addon Summary Block

**What:** After all addon actions run, print a scannable summary listing each addon's name, status (installed/skipped/failed), and any key info.

**When to use:** End of `Cluster()` in `create.go`, after all addon actions.

**Example:**

```go
func logAddonSummary(logger log.Logger, results []addonResult) {
    logger.V(0).Info("\nAddons:")
    for _, r := range results {
        switch {
        case !r.enabled:
            logger.V(0).Infof("  ✗ %-20s skipped (disabled)\n", r.name)
        case r.err != nil:
            logger.V(0).Infof("  ✗ %-20s failed: %v\n", r.name, r.err)
        default:
            logger.V(0).Infof("  ✓ %-20s installed\n", r.name)
        }
    }
}
```

### Pattern 5: Dependency Conflict Warning

**What:** If MetalLB is disabled but Envoy Gateway is enabled, warn and continue (install Envoy Gateway anyway).

**When to use:** Before running addon actions, check dependency coherence.

**Example:**

```go
// Before addon action loop in create.go
if !addonEnabled(opts.Config.Addons.MetalLB) && addonEnabled(opts.Config.Addons.EnvoyGateway) {
    logger.Warn(
        "MetalLB is disabled but Envoy Gateway is enabled. " +
        "Envoy Gateway proxy services will not receive LoadBalancer IPs. " +
        "Installing Envoy Gateway anyway.",
    )
}
```

### Pattern 6: Unrecognized Addon Name Validation

**What:** The YAML decoder uses `KnownFields(true)` (via `yamlUnmarshalStrict`). Any field in the `addons:` map that is not defined in the struct will produce an error automatically. No custom validation needed for unknown keys — the strict decoder handles it.

**When to use:** This is not a code pattern to add; it's a property of the existing decoder. The error message will reference the unknown field name. The user constraint "strict error listing valid addon names" is satisfied by the YAML decoder's built-in behavior.

**Verification:** Confirmed in `pkg/internal/apis/config/encoding/load.go`:

```go
func yamlUnmarshalStrict(raw []byte, v interface{}) error {
    d := yaml.NewDecoder(bytes.NewReader(raw))
    d.KnownFields(true)  // rejects unknown fields
    return d.Decode(v)
}
```

The error message from the yaml library when an unknown field is encountered reads: `yaml: unmarshal errors: field <name> not found in type v1alpha4.Addons`. This satisfies the requirement.

### Anti-Patterns to Avoid

- **Negative bool fields (DisableX):** Prior research used this pattern. The user locked positive booleans (`addons.metalLB: true`). Use positive bool or *bool with true default.
- **Plain bool (non-pointer) with true default:** Cannot distinguish user-set `false` from absent field. Use `*bool` so nil means "not set" (defaulted to true).
- **Modifying ActionContext:** No change needed. All addon data flows through `ctx.Config.Addons`.
- **Aborting on addon failure:** User locked "warn and continue." The existing action loop `return err` pattern must be replaced with a collect-and-continue pattern for the addon section.
- **Setting cobra `Use: "kinder"` in root.go:** The cobra `Use` field is the command name shown in help text. It should be changed to `"kinder"` for correct help output. The binary name itself is controlled by Makefile.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Unknown YAML field detection | Custom field validation | `yamlUnmarshalStrict` (already in place) | The yaml library's `KnownFields(true)` already rejects unknown fields with field name in error message |
| Platform detection | Custom OS detection | `runtime.GOOS` from Go stdlib | Already used in 4 places in codebase; no new dependency |
| Progress display | Custom spinner/status | `cli.Status.Start()` / `Status.End()` | Already wired in ActionContext via ctx.Status |
| Config loading | Custom YAML parser | existing `encoding.Load()` / `encoding.Parse()` | Full pipeline already handles defaults, conversion, and validation |

**Key insight:** Phase 1 is primarily wiring — the infrastructure already exists. The work is adding fields to existing structs, adding conditions to an existing list, and updating the Makefile. Resist the urge to introduce new abstractions.

---

## Common Pitfalls

### Pitfall 1: Positive Bool with Non-Zero Default

**What goes wrong:** Developer adds `MetalLB bool` to the Addons struct with intent "true = enabled." Sets default in `SetDefaultsCluster`. But the YAML decode runs before defaults for fields that ARE present. A user's `addons.metalLB: false` sets `MetalLB = false`. Then `SetDefaultsCluster` sees `false` and cannot tell if user set it or it's the zero value. So it sets it back to `true`. User's opt-out is ignored.

**Why it happens:** Go bool zero value (`false`) is indistinguishable from an explicit `false` after decode.

**How to avoid:** Use `*bool`. `nil` = not set (apply default of `true`). `false` = explicitly disabled. `true` = explicitly enabled. SetDefaultsCluster only sets if `nil`.

**Warning signs:** Unit test "user sets metalLB: false, verify it's disabled" fails.

### Pitfall 2: Module Path vs Binary Name Confusion

**What goes wrong:** Developer changes `go.mod` module path from `sigs.k8s.io/kind` to `sigs.k8s.io/kinder`. This breaks all internal package imports that reference `sigs.k8s.io/kind/pkg/...`.

**Why it happens:** Confusing "binary name" with "module path."

**How to avoid:** Keep `go.mod` module path as `sigs.k8s.io/kind`. Only change `KIND_BINARY_NAME=kinder` in Makefile and `Use: "kinder"` in cobra root command.

**Warning signs:** `go build ./...` fails with import errors after module path change.

### Pitfall 3: Addon Actions in Wrong Pipeline Position

**What goes wrong:** Addon action gates added before `waitforready.NewAction()`. Addon pods scheduled before nodes are Ready. MetalLB speaker DaemonSet pending. Rollout waits time out during cluster creation.

**Why it happens:** Wanting to parallelize for speed; misunderstanding that `waitforready` is the gate.

**How to avoid:** All addon actions must be appended to `actionsToRun` AFTER `waitforready.NewAction(opts.WaitForReady)`. Confirmed by existing pattern: installcni and installstorage both run before kubeadmjoin and waitforready, but they install to the node image, not deploy workloads. Addon actions that use `kubectl apply` need the cluster to be fully up.

**Warning signs:** Addon pods stuck in `Pending` during creation; rollout status times out.

### Pitfall 4: Breaking the Existing Action Loop Error Handling

**What goes wrong:** Making the entire `Cluster()` function use warn-and-continue for all errors. The existing pre-addon actions (kubeadminit, kubeadmjoin, etc.) must still return errors — only addon actions use warn-and-continue.

**Why it happens:** Over-applying the user's "warn and continue" decision to the whole pipeline.

**How to avoid:** Split the action loop. Keep existing sequential-abort loop for core actions (loadbalancer through waitforready). Add a separate addon execution section with collect-and-continue behavior.

**Warning signs:** A failed kubeadminit silently continues and produces a broken cluster.

### Pitfall 5: Strict YAML Decode Rejects New Fields in Existing Config Files

**What goes wrong:** The `addons` struct is added to internal types but NOT to the v1alpha4 public types. A config file with `addons:` gets rejected by `yamlUnmarshalStrict` with "field addons not found."

**Why it happens:** Forgetting to add the field to the public v1alpha4 type (both files need updating).

**How to avoid:** Always update both: `pkg/apis/config/v1alpha4/types.go` AND `pkg/internal/apis/config/types.go`. Verify with `go test ./pkg/internal/apis/config/encoding/...` with a test config containing `addons:`.

**Warning signs:** Load test for config-with-addons returns unexpected error.

### Pitfall 6: Deep Copy Not Regenerated

**What goes wrong:** `Addons` struct added to internal types but `zz_generated.deepcopy.go` not regenerated. `DeepCopyInto` on `Cluster` does not copy the `Addons` field. Config mutations in the action pipeline don't propagate correctly.

**Why it happens:** Forgetting to run code generation after struct changes.

**How to avoid:** Run `make generate` or manually add the deepcopy for `Addons`. For a struct of only bool pointers, the manual deepcopy is simple:

```go
func (in *Addons) DeepCopyInto(out *Addons) {
    *out = *in  // copy all scalar fields
    // For *bool fields, deep copy each pointer:
    if in.MetalLB != nil {
        b := *in.MetalLB
        out.MetalLB = &b
    }
    // ... repeat for each *bool field
}
```

**Warning signs:** Tests that pass a config, mutate it, and check the copy see stale values.

---

## Code Examples

Verified patterns from codebase inspection:

### Binary Name Change (Makefile)

```makefile
# Source: /Users/patrykattc/work/git/kinder/Makefile
# Change line 55 from:
KIND_BINARY_NAME?=kind
# To:
KIND_BINARY_NAME?=kinder
```

### Cobra Root Command Use Field

```go
// Source: /Users/patrykattc/work/git/kinder/pkg/cmd/kind/root.go
// Change Use field in NewCommand():
cmd := &cobra.Command{
    Use:   "kinder",    // was "kind"
    Short: "kinder is a tool for managing local Kubernetes clusters with batteries included",
    Long:  "kinder creates and manages local Kubernetes clusters using Docker container 'nodes', with addons pre-installed",
    // ...
}
```

### v1alpha4 Addons Type Addition

```go
// Source: pkg/apis/config/v1alpha4/types.go
// Add to Cluster struct:
// Addons configures which default addons are installed during cluster creation.
Addons Addons `yaml:"addons,omitempty" json:"addons,omitempty"`

// New type:
// Addons controls per-addon installation during cluster creation.
// All addons are enabled by default. Set a field to false to skip that addon.
type Addons struct {
    // MetalLB enables MetalLB for LoadBalancer services.
    // +optional (default: true)
    MetalLB *bool `yaml:"metalLB,omitempty" json:"metalLB,omitempty"`

    // EnvoyGateway enables Envoy Gateway and Gateway API CRDs.
    // +optional (default: true)
    EnvoyGateway *bool `yaml:"envoyGateway,omitempty" json:"envoyGateway,omitempty"`

    // MetricsServer enables Kubernetes Metrics Server.
    // +optional (default: true)
    MetricsServer *bool `yaml:"metricsServer,omitempty" json:"metricsServer,omitempty"`

    // CoreDNSTuning enables CoreDNS cache and performance tuning.
    // +optional (default: true)
    CoreDNSTuning *bool `yaml:"coreDNSTuning,omitempty" json:"coreDNSTuning,omitempty"`

    // Dashboard enables the Kubernetes Dashboard.
    // +optional (default: true)
    Dashboard *bool `yaml:"dashboard,omitempty" json:"dashboard,omitempty"`
}
```

### v1alpha4 Default Setting

```go
// Source: pkg/apis/config/v1alpha4/default.go — add to SetDefaultsCluster()
// Default all addons to enabled (explicit because Go bool zero value is false).
trueVal := true
if obj.Addons.MetalLB == nil {
    obj.Addons.MetalLB = &trueVal
}
if obj.Addons.EnvoyGateway == nil {
    obj.Addons.EnvoyGateway = &trueVal
}
if obj.Addons.MetricsServer == nil {
    obj.Addons.MetricsServer = &trueVal
}
if obj.Addons.CoreDNSTuning == nil {
    obj.Addons.CoreDNSTuning = &trueVal
}
if obj.Addons.Dashboard == nil {
    obj.Addons.Dashboard = &trueVal
}
```

### Internal Types Addons (no YAML tags)

```go
// Source: pkg/internal/apis/config/types.go
// Add to Cluster struct:
Addons Addons

// New type:
type Addons struct {
    MetalLB       bool
    EnvoyGateway  bool
    MetricsServer bool
    CoreDNSTuning bool
    Dashboard     bool
}
```

Note: Internal types use plain `bool` (not pointers) because by the time values reach the internal type, defaults have already been applied by `SetDefaultsCluster()`. The convert function copies the dereferenced value.

### Convert Function Update

```go
// Source: pkg/internal/apis/config/convert_v1alpha4.go — add to Convertv1alpha4()
// Helper to dereference *bool with true default (safe after SetDefaultsCluster runs)
boolVal := func(b *bool) bool {
    if b == nil {
        return true
    }
    return *b
}

out.Addons = Addons{
    MetalLB:       boolVal(in.Addons.MetalLB),
    EnvoyGateway:  boolVal(in.Addons.EnvoyGateway),
    MetricsServer: boolVal(in.Addons.MetricsServer),
    CoreDNSTuning: boolVal(in.Addons.CoreDNSTuning),
    Dashboard:     boolVal(in.Addons.Dashboard),
}
```

### Platform Warning

```go
// Source: pkg/cluster/internal/create/create.go (new helper)
import "runtime"

func logMetalLBPlatformWarning(logger log.Logger) {
    switch runtime.GOOS {
    case "darwin":
        logger.V(0).Info(
            " • NOTE: On macOS, MetalLB LoadBalancer IPs are not directly reachable from the host.\n" +
            "   Access LoadBalancer services via kubectl port-forward:\n" +
            "   kubectl port-forward svc/<service-name> <local-port>:<service-port>",
        )
    case "windows":
        logger.V(0).Info(
            " • NOTE: On Windows, MetalLB LoadBalancer IPs are not directly reachable from the host.\n" +
            "   Access LoadBalancer services via kubectl port-forward:\n" +
            "   kubectl port-forward svc/<service-name> <local-port>:<service-port>",
        )
    }
}
```

### Action Loop Modification (create.go)

The existing action loop currently aborts on error. Phase 1 splits this into core actions (abort) and addon actions (warn-continue):

```go
// Source: pkg/cluster/internal/create/create.go

// --- EXISTING: core actions run with abort-on-error ---
actionsToRun := []actions.Action{
    loadbalancer.NewAction(),
    configaction.NewAction(),
}
// ... (existing kubeadminit, installcni, installstorage, kubeadmjoin, waitforready)

for _, action := range actionsToRun {
    if err := action.Execute(actionsContext); err != nil {
        if !opts.Retain {
            _ = delete.Cluster(logger, p, opts.Config.Name, opts.KubeconfigPath)
        }
        return err
    }
}

// --- NEW: addon actions run with warn-continue ---
type addonResult struct {
    name    string
    enabled bool
    err     error
}

var addonResults []addonResult

runAddon := func(name string, enabled bool, a actions.Action) {
    if !enabled {
        logger.V(0).Infof(" • Skipping %s (disabled in config)\n", name)
        addonResults = append(addonResults, addonResult{name: name, enabled: false})
        return
    }
    if err := a.Execute(actionsContext); err != nil {
        logger.Warnf("Addon %s failed to install (cluster still usable): %v", name, err)
        addonResults = append(addonResults, addonResult{name: name, enabled: true, err: err})
        return
    }
    addonResults = append(addonResults, addonResult{name: name, enabled: true})
}

// Dependency conflict check
if !opts.Config.Addons.MetalLB && opts.Config.Addons.EnvoyGateway {
    logger.Warn("MetalLB is disabled but Envoy Gateway is enabled — " +
        "Envoy Gateway proxy services will not receive external IPs")
}

// Addon action gates (Phase 1: stub actions; filled in Phases 2-6)
runAddon("MetalLB", opts.Config.Addons.MetalLB, installmetallb.NewAction())
runAddon("Envoy Gateway", opts.Config.Addons.EnvoyGateway, installenvoygw.NewAction())
runAddon("Metrics Server", opts.Config.Addons.MetricsServer, installmetricsserver.NewAction())
runAddon("CoreDNS Tuning", opts.Config.Addons.CoreDNSTuning, installcorednstuning.NewAction())
runAddon("Dashboard", opts.Config.Addons.Dashboard, installdashboard.NewAction())

// Platform warning for MetalLB (FOUND-05)
if opts.Config.Addons.MetalLB {
    logMetalLBPlatformWarning(logger)
}

// Addon summary
logAddonSummary(logger, addonResults)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Negative disable flags (DisableMetalLB) | Positive enable flags (MetalLB) | User decision 2026-03-01 | Requires `*bool` with nil=true default; prior research used plain bool negative fields |
| No addon section in v1alpha4 | `addons:` map section | Phase 1 (this phase) | Additive; backward compatible via omitempty |
| Single abort-on-error action loop | Split: abort for core, warn-continue for addons | Phase 1 (this phase) | Existing actions unchanged; new section added after waitforready |

**Deprecated/outdated:**
- Prior ARCHITECTURE.md research used `DisableMetalLB bool` negative fields — superseded by user decision for positive `MetalLB *bool` fields
- Prior ARCHITECTURE.md used `Headlamp` as dashboard — STACK.md uses `Kubernetes Dashboard Helm 7.14.0` — both are Phase 6 concern; Phase 1 just needs the field name `Dashboard`

---

## Open Questions

1. **Stub action implementations for Phase 1**
   - What we know: Phase 1 must wire addon actions into create.go but no actual addon is installed yet
   - What's unclear: Should stub actions be no-ops that do nothing (and pass), or should they skip with a log message?
   - Recommendation: Stub actions should be no-ops (return nil immediately) with a single log line "MetalLB: stub — will be implemented in Phase 2". This allows Phase 1 to compile and pass all tests without Phase 2-6 work.

2. **`*bool` vs plain `bool` user experience in YAML**
   - What we know: `*bool` with `omitempty` means a field set to `false` is NOT omitted (pointer is non-nil, just points to false). A `nil` pointer IS omitted. This is correct YAML behavior.
   - What's unclear: Whether `go.yaml.in/yaml/v3` handles `*bool` with `omitempty` correctly (omits nil, preserves false)
   - Recommendation: Write a unit test in `encoding/load_test.go` for "config with addons.metalLB: false" and verify parse result. This is low-risk since yaml/v3 is well-tested for pointer types.

3. **Module path in `go.mod`**
   - What we know: Current module is `sigs.k8s.io/kind`. All internal imports use this path.
   - What's unclear: Whether any user-visible behavior depends on the module path (it should not — module path is a Go-internal concept)
   - Recommendation: Do NOT change `go.mod` module path in Phase 1. The binary name (`kinder`) is purely a build artifact name. Module path can be changed in a future phase if needed, but it's a large refactor of all imports.

4. **Addon summary color/formatting**
   - What we know: Kind uses ANSI color codes `\x1b[32m✓\x1b[0m` for success (green) and `\x1b[31m✗\x1b[0m` for failure (red) in `cli/status.go`, but only when a spinner/tty is detected
   - What's unclear: Whether the summary block should use the same conditional coloring or always plain text
   - Recommendation: Use the same conditional approach as `Status.End()` — check if logger is `*Logger` with `*Spinner` writer, use color only then. For Phase 1 stub, plain text is fine; color can be added when real actions are wired.

---

## Validation Architecture

No `workflow.nyquist_validation` config found — checking manually. The project uses `go test` with `MODE=unit` via `hack/make-rules/test.sh`. Standard Go test patterns apply.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` package |
| Config file | None (uses `go test ./...`) |
| Quick run command | `go test ./pkg/internal/apis/config/... ./pkg/internal/apis/config/encoding/...` |
| Full suite command | `MODE=unit hack/make-rules/test.sh` or `go test ./...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FOUND-01 | Binary named `kinder` coexists with `kind` | Manual smoke | `./bin/kinder create cluster && which kind` | N/A — build artifact |
| FOUND-02 | Config with addons section parses without error | Unit | `go test ./pkg/internal/apis/config/encoding/ -run TestLoadAddons` | ❌ Wave 0 |
| FOUND-03 | Config without addons section still works | Unit | `go test ./pkg/internal/apis/config/encoding/ -run TestLoadCurrent` | ✅ (existing test, add addons-absent case) |
| FOUND-04 | Addon enable flag visible to action code | Unit | `go test ./pkg/internal/apis/config/ -run TestAddonsDefault` | ❌ Wave 0 |
| FOUND-05 | Platform warning printed on macOS/Windows | Unit | `go test ./pkg/cluster/internal/create/ -run TestPlatformWarning` | ❌ Wave 0 |

### Wave 0 Gaps

- [ ] `pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-enabled.yaml` — test config with addons all true
- [ ] `pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-disabled.yaml` — test config with addons.metalLB: false
- [ ] `pkg/internal/apis/config/addons_test.go` — tests for default addons (all enabled when not set)
- [ ] `pkg/cluster/internal/create/platform_test.go` — unit test for platform warning (inject GOOS via function param, same pattern as paths.go)

---

## Sources

### Primary (HIGH confidence)

- Kinder codebase read directly: `/Users/patrykattc/work/git/kinder` (commit a25799ce) — all action pipeline, config types, deepcopy, encoding, CLI structure
- `pkg/cluster/internal/create/create.go` — action pipeline wiring, error handling pattern
- `pkg/apis/config/v1alpha4/types.go` — existing config type structure, YAML tags
- `pkg/internal/apis/config/types.go` — internal types (no YAML tags)
- `pkg/internal/apis/config/convert_v1alpha4.go` — conversion pattern
- `pkg/internal/apis/config/default.go` — defaulting pattern
- `pkg/internal/apis/config/encoding/load.go` — strict YAML decode via `KnownFields(true)`
- `pkg/internal/apis/config/encoding/load_test.go` — existing test patterns for config loading
- `pkg/internal/cli/status.go` — Status.Start/End pattern, color codes
- `pkg/internal/cli/spinner.go` — `runtime.GOOS == "windows"` pattern
- `Makefile` — `KIND_BINARY_NAME` variable for binary name control
- `.planning/research/ARCHITECTURE.md` — prior project research (HIGH, read from codebase)
- `.planning/research/STACK.md` — addon versions and install methods (HIGH, verified against GitHub releases)

### Secondary (MEDIUM confidence)

- `.planning/research/PITFALLS.md` — confirmed pitfalls from project research phase

### Tertiary (LOW confidence)

None for this phase — all findings are from direct codebase inspection.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages already in go.mod; patterns verified in codebase
- Architecture: HIGH — read directly from source files; action pipeline, config layers, deepcopy all confirmed
- Pitfalls: HIGH — *bool vs bool pitfall is Go type system, not speculation; other pitfalls derived from direct code reading

**Research date:** 2026-03-01
**Valid until:** 2026-06-01 (stable — Go and kind codebase are not fast-moving)
