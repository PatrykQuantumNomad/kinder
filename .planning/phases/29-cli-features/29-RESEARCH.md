# Phase 29: CLI Features - Research

**Researched:** 2026-03-04
**Domain:** Go CLI flag extension (cobra/pflag), encoding/json, addon preset pattern
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-01 | `--output json` flag on `kinder env` | `env.go` prints three key=value lines to stdout; JSON struct with 3 fields; logger already goes to stderr; no logger redirection needed |
| CLI-02 | `--output json` flag on `kinder doctor` | `doctor.go` has internal `result` struct with name/status/message; marshal `[]result` as JSON array; currently prints to `streams.ErrOut` (stderr); JSON must go to `streams.Out` (stdout) while logger noise stays on stderr |
| CLI-03 | `--output json` flag on `kinder get clusters` | currently prints one name per line; JSON = `["name1","name2"]` string array; `provider.List()` already returns `[]string` |
| CLI-04 | `--output json` flag on `kinder get nodes` | currently prints `node.String()` one per line; JSON = array of objects with `name` and `role` fields; `node.Role()` returns `(string, error)` which must be handled |
| CLI-05 | `--profile` flag on `kinder create cluster` with minimal/full/gateway/ci presets | profiles map to `config.Addons` bool sets; implemented as a `CreateOption` that patches `opts.Config.Addons` after `fixupOptions` runs; no-config default must remain unchanged |
</phase_requirements>

---

## Summary

Phase 29 adds two independent feature families to the kinder CLI. The first is `--output json` on four read commands (`env`, `doctor`, `get clusters`, `get nodes`), enabling machine-readable output suitable for piping to `jq`. The second is `--profile` on `kinder create cluster`, which selects a named addon preset without requiring a YAML config file.

The `--output json` pattern is idiomatic in Kubernetes tooling (kubectl uses `--output json`, `--output yaml`). The implementation is straightforward: add an `Output string` flag to each command's `flagpole`, check `flags.Output == "json"` in `runE`, and emit `json.NewEncoder(streams.Out).Encode(data)` instead of `fmt.Fprintln`. The critical requirement is that logger/diagnostic output stays on `streams.ErrOut` (stderr) even in JSON mode — none of the four commands need logger redirection because `env`, `get clusters`, and `get nodes` already write human output to `streams.Out` (stdout) and log output to the `logger`/`streams.ErrOut`. The `doctor` command currently writes results to `streams.ErrOut`; in JSON mode those results must move to `streams.Out`.

The `--profile` flag maps named strings to `config.Addons` structs. The four required profiles are: `minimal` (all addons false — pure kind cluster), `full` (all addons true — same as current default), `gateway` (MetalLB + EnvoyGateway only), `ci` (MetricsServer + CertManager only, no UI/registry). The profile is applied via a new `CreateWithAddonProfile` option that patches `ClusterOptions.Config.Addons` after the config file is loaded. When `--profile` is not set, behavior is 100% identical to current (all addons enabled via the `boolPtrTrue` defaults in `v1alpha4.SetDefaultsCluster`).

**Primary recommendation:** Use `encoding/json` stdlib (`json.NewEncoder(streams.Out).Encode(v)`) for all JSON output; add `--output` to flagpoles as a `string` flag; implement profiles as a `CreateOption` that sets all `Addons` bool fields explicitly.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/json` | stdlib (Go 1.24) | JSON encoding of output structs | Already used in 10+ files in pkg; no external dependency needed |
| `github.com/spf13/cobra` | v1.8.0 (current in go.mod) | `cmd.Flags().StringVar(...)` for new flags | All CLI flags already use cobra/pflag; same pattern everywhere |
| `github.com/spf13/pflag` | v1.0.5 (current in go.mod) | Flag registration | Transitive dep already present |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `fmt` | stdlib | Error output when `--output` value is invalid | For the `--output` validation error path only |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `json.NewEncoder(streams.Out).Encode(v)` | `json.Marshal` + `fmt.Fprintf` | Encoder writes directly to writer, avoids intermediate buffer — prefer Encoder |
| `--output json` string flag | `--json` bool flag | String matches kubectl convention; bool is simpler but non-extensible |
| Profile as `--profile` string flag | `--addons` comma-list | Profile is cleaner UX; `--addons` is EXT-02 (future/out of scope) |

**No new external dependencies.** `encoding/json` is stdlib. cobra/pflag already in go.mod.

---

## Architecture Patterns

### Recommended File Changes

```
pkg/cmd/kind/env/env.go                    # Add Output flag, JSON branch in runE
pkg/cmd/kind/doctor/doctor.go             # Add Output flag, JSON branch in runE
pkg/cmd/kind/get/clusters/clusters.go     # Add Output flag, JSON branch in runE
pkg/cmd/kind/get/nodes/nodes.go           # Add Output flag, JSON branch in runE
pkg/cmd/kind/create/cluster/createcluster.go  # Add Profile flag, apply preset
pkg/cluster/createoption.go               # New CreateWithAddonProfile option (or inline in createcluster.go)
```

### Pattern 1: `--output json` Flag on a Read Command

**What:** Add `Output string` to the command's `flagpole`; in `runE`, branch on `flags.Output`; emit JSON to `streams.Out`; keep all logger/warning output on `streams.ErrOut`.

**When to use:** All four read commands (CLI-01 through CLI-04).

**Example (env command):**
```go
// pkg/cmd/kind/env/env.go

type flagpole struct {
    Name   string
    Output string // NEW: "" means human-readable, "json" means JSON
}

func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
    flags := &flagpole{}
    cmd := &cobra.Command{...}
    cmd.Flags().StringVarP(&flags.Name, "name", "n", cluster.DefaultName, "the cluster context name")
    // NEW:
    cmd.Flags().StringVar(&flags.Output, "output", "", `output format; "" for human-readable, "json" for JSON`)
    return cmd
}

func runE(logger log.Logger, streams cmd.IOStreams, flags *flagpole) error {
    providerName := activeProviderName(logger)
    clusterName := flags.Name
    kubeconfigPath := activeKubeconfigPath()

    if flags.Output == "json" {
        out := struct {
            KinderProvider string `json:"kinderProvider"`
            ClusterName    string `json:"clusterName"`
            Kubeconfig     string `json:"kubeconfig"`
        }{
            KinderProvider: providerName,
            ClusterName:    clusterName,
            Kubeconfig:     kubeconfigPath,
        }
        return json.NewEncoder(streams.Out).Encode(out)
    }

    // Default human-readable output (unchanged)
    fmt.Fprintf(streams.Out, "KINDER_PROVIDER=%s\n", providerName) //nolint:errcheck
    fmt.Fprintf(streams.Out, "KIND_CLUSTER_NAME=%s\n", clusterName) //nolint:errcheck
    fmt.Fprintf(streams.Out, "KUBECONFIG=%s\n", kubeconfigPath)     //nolint:errcheck
    return nil
}
```

### Pattern 2: JSON Output for `doctor` — Results Move from Stderr to Stdout

**What:** `doctor`'s `result` struct is already defined but unexported. For JSON mode, build the same `[]result` slice, then marshal it to `streams.Out`. The `result` struct needs exported fields (or a separate JSON-only struct) for `json.Marshal` to work.

**When to use:** CLI-02 only.

**Key difference from other commands:** In human mode, results print to `streams.ErrOut`. In JSON mode, the structured array goes to `streams.Out`. Logger warnings (from `checkBinary` internals) remain on `streams.ErrOut`.

**JSON schema for doctor:**
```go
// Source: doctor.go result struct (adapt for JSON)
type checkResult struct {
    Name    string `json:"name"`
    Status  string `json:"status"`  // "ok", "warn", "fail"
    Message string `json:"message,omitempty"`
}
// Output: JSON array [{"name":"docker","status":"ok"},{"name":"kubectl","status":"fail","message":"..."}]
```

**Exit code behavior in JSON mode:** When `--output json` is set, the command should still use the same exit code semantics (0=ok, 1=fail, 2=warn). The JSON output is on stdout; exit codes remain. `os.Exit` is currently used to bypass cobra — this approach must remain or be replicated.

### Pattern 3: JSON Output for `get clusters` — String Array

**What:** `provider.List()` returns `[]string`. JSON output is a JSON array of strings.

**When to use:** CLI-03.

```go
// pkg/cmd/kind/get/clusters/clusters.go
if flags.Output == "json" {
    if clusters == nil {
        clusters = []string{} // avoid null, emit []
    }
    return json.NewEncoder(streams.Out).Encode(clusters)
}
```

**Critical:** When no clusters exist, human mode logs "No kind clusters found." via `logger.V(0).Info(...)`. JSON mode must emit `[]` (empty array) to stdout and return nil, not an error. `jq empty` on `[]` exits 0.

### Pattern 4: JSON Output for `get nodes` — Object Array with Role

**What:** `nodes.Node` is an interface. `node.String()` gives the name. `node.Role()` returns `(string, error)`. Build a slice of structs before encoding.

**When to use:** CLI-04.

```go
// pkg/cmd/kind/get/nodes/nodes.go
type nodeInfo struct {
    Name string `json:"name"`
    Role string `json:"role"`
}

if flags.Output == "json" {
    var infos []nodeInfo
    for _, n := range nodes {
        role, err := n.Role()
        if err != nil {
            role = "unknown"
        }
        infos = append(infos, nodeInfo{Name: n.String(), Role: role})
    }
    if infos == nil {
        infos = []nodeInfo{} // avoid null
    }
    return json.NewEncoder(streams.Out).Encode(infos)
}
```

### Pattern 5: `--profile` Flag on `create cluster`

**What:** Add `Profile string` to `flagpole`. After `configOption(flags.Config, streams.In)` resolves the config, apply a profile override to `opts.Config.Addons`. Profile is applied via a new `CreateOption`.

**When to use:** CLI-05.

**Profile definitions (canonical):**
```
minimal  → all false (MetalLB=false, EnvoyGateway=false, MetricsServer=false, CoreDNSTuning=false, Dashboard=false, LocalRegistry=false, CertManager=false)
full     → all true  (MetalLB=true,  EnvoyGateway=true,  MetricsServer=true,  CoreDNSTuning=true,  Dashboard=true,  LocalRegistry=true,  CertManager=true)
gateway  → MetalLB=true, EnvoyGateway=true; all others false
ci       → MetricsServer=true, CertManager=true; all others false
```

**Implementation in `createcluster.go`:**
```go
// pkg/cmd/kind/create/cluster/createcluster.go

type flagpole struct {
    Name       string
    Config     string
    ImageName  string
    Retain     bool
    Wait       time.Duration
    Kubeconfig string
    Profile    string // NEW
}

// In NewCommand:
cmd.Flags().StringVar(&flags.Profile, "profile", "", `addon preset: "minimal", "full", "gateway", or "ci"`)

// In runE, after building withConfig:
if err = provider.Create(
    flags.Name,
    withConfig,
    cluster.CreateWithAddonProfile(flags.Profile), // NEW — applied after config load
    cluster.CreateWithNodeImage(flags.ImageName),
    ...
); err != nil { ... }
```

**`CreateWithAddonProfile` option in `createoption.go`:**
```go
// pkg/cluster/createoption.go

// CreateWithAddonProfile applies a named addon preset to ClusterOptions.Addons.
// Profile is applied after any --config file is loaded, overriding addon settings.
// If profile is "" (empty string), this option is a no-op (preserves existing defaults).
func CreateWithAddonProfile(profile string) CreateOption {
    return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
        switch profile {
        case "":
            return nil // no-op: default behavior (all addons enabled by v1alpha4 defaults)
        case "minimal":
            o.Config.Addons = internalconfig.Addons{} // all false (zero value)
        case "full":
            o.Config.Addons = internalconfig.Addons{
                MetalLB: true, EnvoyGateway: true, MetricsServer: true,
                CoreDNSTuning: true, Dashboard: true, LocalRegistry: true, CertManager: true,
            }
        case "gateway":
            o.Config.Addons = internalconfig.Addons{MetalLB: true, EnvoyGateway: true}
        case "ci":
            o.Config.Addons = internalconfig.Addons{MetricsServer: true, CertManager: true}
        default:
            return fmt.Errorf("unknown profile %q: valid values are minimal, full, gateway, ci", profile)
        }
        return nil
    })
}
```

**Order of `CreateOption` application matters.** In `cluster.Provider.Create()`, options are applied in order. `CreateWithAddonProfile` must be applied AFTER `CreateWithConfigFile`/`CreateWithRawConfig` because profiles override config-file addon settings. In `createcluster.go`, position it right after `withConfig`.

**No-profile default is identical to current.** When `--profile ""` (unset), `CreateWithAddonProfile("")` is a no-op. The `v1alpha4.SetDefaultsCluster` still runs (via `fixupOptions → encoding.Load("")`) and sets all addons to true via `boolPtrTrue`.

### Pattern 6: `--output` Flag Validation

**What:** Reject unknown output formats early with a clear error message.

```go
switch flags.Output {
case "", "json":
    // valid
default:
    return fmt.Errorf("unknown output format %q: valid values are \"\" and \"json\"", flags.Output)
}
```

Apply this validation at the top of `runE` for each command.

### Anti-Patterns to Avoid

- **Writing JSON to `streams.ErrOut`:** JSON output MUST go to `streams.Out` (stdout). Tools like `jq` read from stdout. `jq empty` on stderr output would fail.
- **Emitting `null` for empty collections:** `json.Marshal(nil)` on a nil slice produces `null`, not `[]`. Always initialize to empty slice before encoding.
- **Mixing human text and JSON on stdout:** In JSON mode, ONLY JSON goes to stdout. All human-readable messages (warnings, info) must go to `streams.ErrOut` or be suppressed.
- **Applying profile before config is loaded:** If `CreateWithAddonProfile` runs before `CreateWithConfigFile`, `o.Config` is nil and the option will panic. Place profile option AFTER the config option in the `provider.Create(...)` call.
- **Using `json.Marshal` + `fmt.Fprintf`:** Prefer `json.NewEncoder(w).Encode(v)` — it writes directly to the writer and appends a trailing newline automatically.
- **Exporting internal `result` struct from doctor package:** The `result` struct in doctor.go is unexported. Do not change the package's API. Either add exported fields to the existing struct or create a parallel exported `CheckResult` struct used only in the JSON path.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON serialization | Custom string concat for JSON | `encoding/json` stdlib | Handles escaping, nesting, edge cases |
| Profile-to-Addons mapping | Complex flag matrix | Simple `switch` on profile string | 4 profiles × 7 addons is a 28-cell table, not a graph |
| Output format dispatch | Separate command subsets | Single `--output` flag with switch | kubectl pattern; avoids command proliferation |

**Key insight:** All the data structures being serialized (`env` vars, doctor results, cluster names, node names/roles) are already computed by the existing `runE` functions. The JSON path is strictly additive — it replaces the `fmt.Fprintln` output section only.

---

## Common Pitfalls

### Pitfall 1: `doctor` Uses `os.Exit` — JSON Must Still Exit Correctly

**What goes wrong:** `doctor.runE` calls `os.Exit(1)` for fail and `os.Exit(2)` for warn to bypass cobra's error handling. If JSON output is added naively before those `os.Exit` calls, the JSON may be flushed partially if `os.Exit` is called before the encoder finishes.

**Why it happens:** `json.NewEncoder(w).Encode(v)` is buffered but flushes immediately (the encoder writes to the underlying writer without a separate Flush). `os.Exit` does NOT flush Go's bufio.Writer; however `streams.Out = os.Stdout` is unbuffered (an `*os.File`), so the write completes before `os.Exit`.

**How to avoid:** Structure the JSON branch to: (1) build `results []result`, (2) call `json.NewEncoder(streams.Out).Encode(results)`, (3) THEN call `os.Exit` with the correct code. Do NOT call `os.Exit` before encoding.

**Warning signs:** JSON output truncated or missing when doctor finds failures.

### Pitfall 2: Empty vs. `null` in JSON Arrays

**What goes wrong:** `var clusters []string` is nil in Go. `json.Marshal(nil)` produces `null`, not `[]`. `jq empty` on `null` exits 0, but `jq '.[]'` on `null` would error. The requirement says `jq empty` exits 0, but producing `null` is non-idiomatic.

**Why it happens:** Go nil slices marshal as JSON `null`.

**How to avoid:** Initialize slices before encoding:
```go
if clusters == nil {
    clusters = []string{}
}
```
or use `make([]string, 0)` when building the slice.

**Warning signs:** `jq empty` passes but `jq length` returns `null` instead of 0.

### Pitfall 3: Profile Applied Before Config Causes Nil Pointer Panic

**What goes wrong:** `CreateOption` functions in `createoption.go` receive `*internalcreate.ClusterOptions`. When `--profile` is set but no `--config` is given, the `Create()` call order is: `CreateWithAddonProfile(...)`, then other options, then `fixupOptions`. If `o.Config` is nil when `CreateWithAddonProfile` tries to set `o.Config.Addons`, it panics.

**Why it happens:** `fixupOptions` populates `opts.Config` if it is nil (via `encoding.Load("")`). Options run BEFORE `fixupOptions` in `cluster.Provider.Create()`.

**How to avoid:** In `CreateWithAddonProfile`, check if `o.Config` is nil and initialize it first:
```go
if o.Config == nil {
    cfg, err := internalencoding.Load("")
    if err != nil {
        return err
    }
    o.Config = cfg
}
o.Config.Addons = ...
```
OR ensure profile option is placed AFTER config option AND relies on `fixupOptions` running. Verify the option application order in `cluster.Provider.Create()`.

**Warning signs:** Panic: `runtime error: invalid memory address or nil pointer dereference` in `CreateWithAddonProfile`.

### Pitfall 4: `--profile` and `--config` Interaction

**What goes wrong:** User passes both `--config my.yaml` and `--profile minimal`. The profile should win (override the config-file addon settings). But if options are applied in wrong order, config file overrides profile.

**Why it happens:** `CreateOption.apply()` functions run in the order they are passed to `provider.Create()`.

**How to avoid:** In `createcluster.go`, pass `withConfig` (from `--config`) first, then `CreateWithAddonProfile(flags.Profile)`. Profile option runs second and explicitly overwrites `o.Config.Addons`.

**Documented behavior:** `--profile` always wins over `--config`'s addon settings. This is expected and desirable — profiles are meant to override. Document this in the flag's usage string.

### Pitfall 5: `kinder get nodes --output json` Without a Cluster

**What goes wrong:** When `--output json` is set and no nodes exist for the cluster, the human-readable path calls `logger.V(0).Infof("No kind nodes found...")` and returns nil. JSON mode should return `[]` not an error.

**Why it happens:** The empty-list check branches to a log message and early return before the encoding loop.

**How to avoid:** In JSON mode, skip the empty-list early return and always encode the (possibly empty) slice:
```go
if flags.Output == "json" {
    if infos == nil {
        infos = []nodeInfo{}
    }
    return json.NewEncoder(streams.Out).Encode(infos)
}
// Only reach human-mode empty check here:
if len(nodes) == 0 {
    logger.V(0).Infof("No kind nodes found...")
    return nil
}
```

---

## Code Examples

Verified patterns from codebase inspection and stdlib docs:

### JSON encoding to streams.Out

```go
// Source: encoding/json stdlib, https://pkg.go.dev/encoding/json#NewEncoder
import "encoding/json"

// Preferred: NewEncoder writes directly to writer, appends trailing newline automatically
if err := json.NewEncoder(streams.Out).Encode(data); err != nil {
    return fmt.Errorf("encoding JSON: %w", err)
}
// Do NOT use json.Marshal + fmt.Fprintf — two allocations for no benefit
```

### Empty slice initialization to avoid `null`

```go
// Source: Go spec — nil slice marshals as null
infos := make([]nodeInfo, 0)           // marshals as []
// or after building: if infos == nil { infos = []nodeInfo{} }
```

### Flag registration (consistent with existing pattern)

```go
// Source: pkg/cmd/kind/get/nodes/nodes.go (existing pattern)
cmd.Flags().StringVar(
    &flags.Output,
    "output",
    "",
    `output format; supported values: "", "json"`,
)
```

### Profile option application order in createcluster.go

```go
// Source: pkg/cmd/kind/create/cluster/createcluster.go (runE)
// CORRECT order: config first, profile second (profile wins)
if err = provider.Create(
    flags.Name,
    withConfig,                                    // 1. load config file (or default)
    cluster.CreateWithAddonProfile(flags.Profile), // 2. override addons (no-op if "")
    cluster.CreateWithNodeImage(flags.ImageName),
    cluster.CreateWithRetain(flags.Retain),
    cluster.CreateWithWaitForReady(flags.Wait),
    cluster.CreateWithKubeconfigPath(flags.Kubeconfig),
    cluster.CreateWithDisplayUsage(true),
    cluster.CreateWithDisplaySalutation(true),
); err != nil {
    return errors.Wrap(err, "failed to create cluster")
}
```

### doctor JSON output structure

```go
// Source: doctor.go result struct, adapted for JSON
// In doctor.go — rename or add exported fields, or use anonymous struct:
type checkResult struct {
    Name    string `json:"name"`
    Status  string `json:"status"`
    Message string `json:"message,omitempty"`
}

// In runE, JSON branch:
if flags.Output == "json" {
    var jsonResults []checkResult
    for _, r := range results {
        jsonResults = append(jsonResults, checkResult{
            Name:    r.name,
            Status:  r.status,
            Message: r.message,
        })
    }
    if jsonResults == nil {
        jsonResults = []checkResult{}
    }
    if err := json.NewEncoder(streams.Out).Encode(jsonResults); err != nil {
        return err
    }
    // Still exit with correct codes (bypass cobra)
    if hasFail {
        os.Exit(1)
    }
    if hasWarn {
        os.Exit(2)
    }
    return nil
}
```

### CreateWithAddonProfile option

```go
// Source: pkg/cluster/createoption.go (new function, matches existing pattern)
import (
    internalconfig "sigs.k8s.io/kind/pkg/internal/apis/config"
    internalencoding "sigs.k8s.io/kind/pkg/internal/apis/config/encoding"
    internalcreate "sigs.k8s.io/kind/pkg/cluster/internal/create"
)

// CreateWithAddonProfile applies a named addon preset to the cluster configuration.
// If profile is "" (empty), this is a no-op (all addons remain enabled by default).
func CreateWithAddonProfile(profile string) CreateOption {
    return createOptionAdapter(func(o *internalcreate.ClusterOptions) error {
        if profile == "" {
            return nil
        }
        // Ensure Config is initialized (fixupOptions may not have run yet)
        if o.Config == nil {
            cfg, err := internalencoding.Load("")
            if err != nil {
                return err
            }
            o.Config = cfg
        }
        switch profile {
        case "minimal":
            o.Config.Addons = internalconfig.Addons{} // zero value = all false
        case "full":
            o.Config.Addons = internalconfig.Addons{
                MetalLB: true, EnvoyGateway: true, MetricsServer: true,
                CoreDNSTuning: true, Dashboard: true, LocalRegistry: true, CertManager: true,
            }
        case "gateway":
            o.Config.Addons = internalconfig.Addons{MetalLB: true, EnvoyGateway: true}
        case "ci":
            o.Config.Addons = internalconfig.Addons{MetricsServer: true, CertManager: true}
        default:
            return fmt.Errorf("unknown profile %q: valid values are minimal, full, gateway, ci", profile)
        }
        return nil
    })
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No machine-readable output | `--output json` on read commands | Phase 29 | Enables scripting and CI integration |
| All addons on by default, no override without YAML | `--profile` for named presets | Phase 29 | Developers pick a preset without writing YAML |

**Deprecated/outdated:**
- Nothing deprecated — this phase is purely additive.

---

## Open Questions

1. **Should `kinder get clusters --output json` include the zero-cluster case as `[]` or as a non-zero exit code?**
   - What we know: Human mode uses `logger.V(0).Info("No kind clusters found.")` and returns nil (exit 0). The requirement says `jq empty` exits 0.
   - What's unclear: Whether an empty `[]` array is the correct response for zero clusters vs. an error.
   - Recommendation: Return `[]` (empty array) with exit 0. `jq empty` on `[]` exits 0. This is consistent with kubectl behavior.

2. **Should `--output` be a persistent flag on the root command or per-subcommand?**
   - What we know: Only 4 commands need it now. Persistent flags on the root would add it to ALL commands including `create cluster` where it doesn't apply (OUT-02 is future scope).
   - What's unclear: Whether future phases will add `--output json` to more commands.
   - Recommendation: Add per-subcommand flags on the 4 read commands only. No root-level persistent flag. This matches the requirement scope and avoids confusion on commands that don't implement it.

3. **Should `--profile minimal` override or merge with `--config` addon settings?**
   - What we know: The phase description says "selects a named addon preset without requiring a YAML config file." The profile should be additive-override: profile wins for addon settings.
   - What's unclear: If user passes both `--config` (with specific addons) and `--profile`, which wins?
   - Recommendation: Profile always wins. Document this in the flag usage string. This is the simplest, most predictable behavior.

---

## Validation Architecture

No `.planning/config.json` found — `nyquist_validation` setting unknown. Including validation section based on existing `go test` infrastructure in the project.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no external test deps per REQUIREMENTS.md out-of-scope) |
| Config file | none — `go test ./...` |
| Quick run command | `go test ./pkg/cmd/kind/...` |
| Full suite command | `make unit` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | Notes |
|--------|----------|-----------|-------------------|-------|
| CLI-01 | `kinder env --output json` produces valid JSON to stdout | unit | `go test ./pkg/cmd/kind/env/...` | Table-driven test with IOStreams capture |
| CLI-02 | `kinder doctor --output json` produces JSON array to stdout | unit | `go test ./pkg/cmd/kind/doctor/...` | Mock checkBinary; verify stdout vs stderr |
| CLI-03 | `kinder get clusters --output json` produces JSON array | unit | `go test ./pkg/cmd/kind/get/clusters/...` | Stub provider.List(); verify `[]` for empty |
| CLI-04 | `kinder get nodes --output json` produces JSON array with name+role | unit | `go test ./pkg/cmd/kind/get/nodes/...` | Use FakeNode from testutil |
| CLI-05 | `--profile minimal` disables all addons in ClusterOptions.Config | unit | `go test ./pkg/cluster/...` | Verify Addons struct values after CreateWithAddonProfile |

### Sampling Rate

- Per task commit: `go test ./pkg/cmd/kind/...`
- Per wave merge: `make unit`
- Phase gate: Full suite green before verification

---

## Sources

### Primary (HIGH confidence)

- `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/env/env.go` — current env command structure; flagpole, runE, IOStreams usage
- `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/doctor/doctor.go` — internal `result` struct; os.Exit semantics; current stderr-only output
- `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/get/clusters/clusters.go` — provider.List() return type; current stdout output
- `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/get/nodes/nodes.go` — nodes.Node interface usage; node.Role() call pattern
- `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/create/cluster/createcluster.go` — flagpole, CreateOption pattern, configOption function
- `/Users/patrykattc/work/git/kinder/pkg/cluster/createoption.go` — CreateOption adapter pattern (createOptionAdapter)
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/create.go` — ClusterOptions.Addons field; wave execution; fixupOptions timing
- `/Users/patrykattc/work/git/kinder/pkg/internal/apis/config/types.go` — internal `Addons` struct (all bool fields)
- `/Users/patrykattc/work/git/kinder/pkg/apis/config/v1alpha4/types.go` — v1alpha4 `Addons` struct (all `*bool` fields)
- `/Users/patrykattc/work/git/kinder/pkg/apis/config/v1alpha4/default.go` — boolPtrTrue defaults for all 7 addons
- `/Users/patrykattc/work/git/kinder/pkg/internal/apis/config/convert_v1alpha4.go` — boolVal conversion (nil → true)
- `/Users/patrykattc/work/git/kinder/pkg/cluster/nodes/types.go` — nodes.Node interface; String(), Role() signatures
- `/Users/patrykattc/work/git/kinder/pkg/cmd/iostreams.go` — IOStreams struct; Out=stdout, ErrOut=stderr
- `/Users/patrykattc/work/git/kinder/go.mod` — confirmed `encoding/json` is stdlib; no new deps needed
- https://pkg.go.dev/encoding/json#NewEncoder — Encoder.Encode appends trailing newline

### Secondary (MEDIUM confidence)

- `/Users/patrykattc/work/git/kinder/.planning/REQUIREMENTS.md` — CLI-01 through CLI-05 definitions; OUT-01/EXT-02 out-of-scope confirmation
- `/Users/patrykattc/work/git/kinder/.planning/phases/28-parallel-execution/28-RESEARCH.md` — confirmed addonResult struct, wave execution, `config.Addons` boolean field names

### Tertiary (LOW confidence)

- None — all findings verified against source code directly.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries (encoding/json, cobra, pflag) already in use in the codebase; no new dependencies
- Architecture: HIGH — all command structures and option patterns directly read from source; profiles derived from REQUIREMENTS.md spec
- Pitfalls: HIGH — nil pointer risk verified against createoption.go option-application pattern; os.Exit concern verified in doctor.go; null-vs-empty verified against Go json stdlib behavior

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (stdlib APIs and cobra v1.8.0 are stable; Go 1.24 stdlib is stable)
