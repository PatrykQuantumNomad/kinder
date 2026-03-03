# Phase 24: CLI Commands - Research

**Researched:** 2026-03-03
**Domain:** Go CLI — Cobra command patterns, eval-safe stdout, binary prerequisite checking, structured exit codes
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-01 | kinder env command shows provider, cluster name, and kubeconfig path | Cobra NewCommand pattern from get/clusters and version; provider name via fmt.Stringer type assertion on internal provider; kubeconfig path from os.Getenv("KUBECONFIG") or $HOME/.kube/config |
| CLI-02 | kinder env output is machine-readable (eval-safe stdout, warnings to stderr) | IOStreams.Out for key=value lines, logger.Warn/IOStreams.ErrOut for warnings; fmt.Fprintf(streams.ErrOut, ...) for stderr-only messages |
| CLI-03 | kinder doctor checks binary prerequisites with actionable fix messages | os/exec.LookPath + exec.Command("-v") pattern from docker/podman/nerdctl IsAvailable(); fmt.Fprintf(streams.ErrOut) for each failure message |
| CLI-04 | kinder doctor uses structured exit codes (0=ok, 1=fail, 2=warn) | Cobra RunE cannot return structured codes; must use os.Exit() inside RunE; pattern: collect results then call os.Exit at end |
</phase_requirements>

## Summary

Phase 24 adds two top-level diagnostic commands to the kinder CLI: `kinder env` (machine-readable cluster environment info) and `kinder doctor` (prerequisite checker with structured exit codes). Both follow the established Cobra pattern in this codebase — each command is a Go package under `pkg/cmd/kind/{name}/` with a `NewCommand(logger, streams)` function, wired into `pkg/cmd/kind/root.go`.

The critical design constraints are: (1) `kinder env` must write key=value pairs to `streams.Out` only and warnings to `streams.ErrOut` only so that `eval $(kinder env)` works correctly in bash; (2) `kinder doctor` must call `os.Exit(1)` or `os.Exit(2)` directly inside `RunE` because Cobra's error return always exits with code 1. The provider name for `kinder env` is obtained via a `fmt.Stringer` type assertion on the internal provider — a pattern already used in `installlocalregistry` and `installmetallb`. A thin `Name() string` method must be added to the public `*cluster.Provider` struct to expose this capability.

No new Go dependencies are required. Both commands use only the standard library (`os`, `os/exec`, `fmt`, `path/filepath`) plus the existing project packages (`cluster`, `cmd`, `log`, `internal/runtime`).

**Primary recommendation:** Add `pkg/cmd/kind/env/env.go` and `pkg/cmd/kind/doctor/doctor.go` as leaf command packages, add `Name() string` to the public `cluster.Provider`, and wire both commands into `root.go`. Use `os.Exit` for doctor's structured exit codes — do NOT use error returns for codes 1/2.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/spf13/cobra | v1.8.0 | Cobra command framework | Already in go.mod; all existing commands use it |
| os/exec (stdlib) | Go 1.21.0+ | Binary detection via LookPath | Existing providers use it for IsAvailable checks |
| fmt (stdlib) | Go 1.21.0+ | key=value output formatting | Existing commands use fmt.Fprintln(streams.Out, ...) |
| os (stdlib) | Go 1.21.0+ | Getenv for KUBECONFIG, os.Exit | Existing kubeconfig path logic uses os.Getenv |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| sigs.k8s.io/kind/pkg/cluster | internal | Provider and cluster name resolution | kinder env uses cluster.NewProvider() + Name() |
| sigs.k8s.io/kind/pkg/cmd | internal | IOStreams struct (Out/ErrOut) | All commands use cmd.IOStreams |
| sigs.k8s.io/kind/pkg/log | internal | Logger for warnings | logger.Warn() goes to ErrOut |
| sigs.k8s.io/kind/pkg/internal/runtime | internal | GetDefault for KIND_EXPERIMENTAL_PROVIDER | Both commands need to honor this env var |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| os.Exit inside RunE | Cobra error return | Error return always exits 1; structured codes (1/2) require os.Exit |
| fmt.Stringer type assertion | Adding Name() to Provider interface | Type assertion is the project pattern for action packages; CLI code can add a public Name() method to *cluster.Provider instead |
| os.Getenv("KUBECONFIG") | Full pathForMerge from internal kubeconfig pkg | pathForMerge is internal; direct os.Getenv + filepath.Join(os.Getenv("HOME"), ".kube", "config") is simpler for display purposes and avoids internal package access |

**Installation:** No new packages needed — use existing dependencies.

## Architecture Patterns

### Recommended Project Structure
```
pkg/cmd/kind/
├── env/
│   └── env.go           # kinder env command
├── doctor/
│   └── doctor.go        # kinder doctor command
└── root.go              # add env.NewCommand + doctor.NewCommand

pkg/cluster/
└── provider.go          # add Name() string method to *Provider
```

### Pattern 1: Leaf Command Package
**What:** Each command is its own package with a `NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command` function that returns a fully configured Cobra command. No subcommands — these are leaf commands.
**When to use:** For all top-level commands without subcommands.
**Example:**
```go
// Source: pkg/cmd/kind/version/version.go (existing pattern)
package version

import (
    "fmt"
    "github.com/spf13/cobra"
    "sigs.k8s.io/kind/pkg/cmd"
    "sigs.k8s.io/kind/pkg/log"
)

func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
    cmd := &cobra.Command{
        Args:  cobra.NoArgs,
        Use:   "version",
        Short: "Prints the kind CLI version",
        RunE: func(cmd *cobra.Command, args []string) error {
            fmt.Fprintln(streams.Out, DisplayVersion())
            return nil
        },
    }
    return cmd
}
```

### Pattern 2: Provider Name via fmt.Stringer
**What:** The internal provider (docker/podman/nerdctl) implements `fmt.Stringer` returning its name. The public `*cluster.Provider` does NOT have a `String()` or `Name()` method — one must be added, or a type assertion is needed. The project pattern in `installlocalregistry` and `installmetallb` uses type assertion on the internal interface. For CLI code using the public Provider, a new `Name() string` method on `*cluster.Provider` is the clean approach.
**When to use:** kinder env needs to print the active provider name.
**Example:**
```go
// To add to pkg/cluster/provider.go
// Name returns the name of the underlying node provider (docker, podman, or nerdctl).
func (p *Provider) Name() string {
    if s, ok := p.provider.(fmt.Stringer); ok {
        return s.String()
    }
    return "unknown"
}
```

### Pattern 3: eval-safe stdout Output
**What:** `kinder env` must print ONLY key=value pairs to stdout, with NO informational messages. Cobra's default behavior writes errors and usage to stdout/stderr based on configuration. The IOStreams.Out writer is for program output; IOStreams.ErrOut is for human-readable messages.
**When to use:** Any machine-readable output command.
**Example:**
```go
// Source: pattern from pkg/cmd/kind/get/clusters/clusters.go
func runE(logger log.Logger, streams cmd.IOStreams, name, kubeconfig string) error {
    // ALL output to streams.Out must be key=value, no trailing newlines with extra text
    fmt.Fprintf(streams.Out, "KINDER_PROVIDER=%s\n", providerName)
    fmt.Fprintf(streams.Out, "KINDER_CLUSTER=%s\n", clusterName)
    fmt.Fprintf(streams.Out, "KINDER_KUBECONFIG=%s\n", kubeconfigPath)
    // warnings go to logger (which goes to ErrOut):
    if someWarning {
        logger.Warn("warning message — goes to stderr only")
    }
    return nil
}
```

### Pattern 4: Structured Exit Codes with os.Exit
**What:** Cobra always converts non-nil error returns from `RunE` into exit code 1. To emit exit code 2 (warning), `os.Exit(2)` must be called directly inside `RunE` before returning. The convention is: collect all check results first, then exit.
**When to use:** `kinder doctor` which needs exit codes 0/1/2.
**Example:**
```go
// pkg/cmd/kind/doctor/doctor.go
func runE(logger log.Logger, streams cmd.IOStreams) error {
    type checkResult struct {
        name    string
        status  string // "ok", "warn", "fail"
        message string
    }
    var results []checkResult
    // ... run checks, append to results ...

    hasWarn := false
    hasFail := false
    for _, r := range results {
        switch r.status {
        case "fail":
            hasFail = true
            fmt.Fprintf(streams.ErrOut, "[FAIL] %s: %s\n", r.name, r.message)
        case "warn":
            hasWarn = true
            fmt.Fprintf(streams.ErrOut, "[WARN] %s: %s\n", r.name, r.message)
        case "ok":
            fmt.Fprintf(streams.ErrOut, "[ OK ] %s\n", r.name)
        }
    }

    if hasFail {
        os.Exit(1)
    }
    if hasWarn {
        os.Exit(2)
    }
    return nil // exit 0
}
```

### Pattern 5: Binary Detection (for kinder doctor)
**What:** Use `os/exec.LookPath` to check if a binary is on PATH, then run `binary -v` and check output prefix to verify it actually works (not just present). This is the exact pattern from docker/podman/nerdctl IsAvailable() functions.
**When to use:** Checking for docker/podman/nerdctl and kubectl in kinder doctor.
**Example:**
```go
// Source: pkg/cluster/internal/providers/docker/util.go (existing pattern)
import osexec "os/exec"
import "sigs.k8s.io/kind/pkg/exec"

// checkBinary returns "" if ok, or an actionable message if not.
func checkBinary(name string, versionPrefix string) string {
    // first check PATH
    if _, err := osexec.LookPath(name); err != nil {
        return fmt.Sprintf("'%s' not found in PATH — install %s", name, installInstructions(name))
    }
    // then verify it runs
    cmd := exec.Command(name, "version")
    lines, err := exec.OutputLines(cmd)
    if err != nil || len(lines) == 0 {
        return fmt.Sprintf("'%s' found but not responding — check if daemon is running", name)
    }
    return "" // ok
}
```

### Pattern 6: Kubeconfig Path for kinder env
**What:** The effective kubeconfig path for display is computed the same way kubectl does: if `$KUBECONFIG` is set, use its first entry; otherwise use `$HOME/.kube/config`. The internal `pathForMerge` function is in an `internal` package and cannot be used from CLI commands. Use `os.Getenv("KUBECONFIG")` directly.
**When to use:** kinder env KINDER_KUBECONFIG output.
**Example:**
```go
func activeKubeconfigPath() string {
    if kc := os.Getenv("KUBECONFIG"); kc != "" {
        // KUBECONFIG may be a colon-separated list; use first non-empty entry
        parts := filepath.SplitList(kc)
        for _, p := range parts {
            if p != "" {
                return p
            }
        }
    }
    home := os.Getenv("HOME")
    return filepath.Join(home, ".kube", "config")
}
```

### Anti-Patterns to Avoid
- **Using Cobra error returns for exit code 2:** Cobra converts all non-nil errors to exit 1. Use `os.Exit(2)` directly.
- **Writing warnings to stdout in kinder env:** `eval $(kinder env)` will fail if any non-key=value text appears on stdout. All human-readable messages go to `streams.ErrOut` or `logger.Warn`.
- **Accessing internal kubeconfig package from CLI commands:** `pathForMerge` is in `pkg/cluster/internal/kubeconfig/internal/kubeconfig` — this is an `internal` package, inaccessible from `pkg/cmd/kind/env`. Reimplement the path logic inline.
- **Using `cluster.Provider.List()` in kinder env to verify cluster exists:** The env command should display configuration info without requiring a running cluster. It reads env vars and config, not container state.
- **Mixing check output with key=value in kinder env:** The output format must be strictly `KEY=VALUE\n` per line on stdout.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Binary availability check | Custom PATH walker | osexec.LookPath | stdlib handles PATH splitting, symlinks, permissions |
| Provider name detection | Switch on env var | fmt.Stringer type assertion on provider | Already the project pattern; handles all three providers without string matching |
| IOStreams separation | Custom writers | cmd.IOStreams.Out vs .ErrOut | Already wired to os.Stdout/os.Stderr; testable via injection |

**Key insight:** The codebase has all the primitives needed. The task is wiring, not building new infrastructure.

## Common Pitfalls

### Pitfall 1: Cobra Exit Code Limitation
**What goes wrong:** Returning a custom `exitError` type from RunE and expecting Cobra to use its code — Cobra ignores exit codes from errors, always calls `os.Exit(1)`.
**Why it happens:** Cobra's Execute() checks only `err != nil` for exit behavior.
**How to avoid:** Call `os.Exit(N)` directly inside RunE before returning for exit codes other than 0. This is the only reliable way.
**Warning signs:** Test shows exit code 1 when 2 was expected.

### Pitfall 2: Stdout Pollution in kinder env
**What goes wrong:** Any informational message written to `streams.Out` (stdout) breaks `eval $(kinder env)` — bash tries to parse the message as a shell variable assignment.
**Why it happens:** `fmt.Fprintln(streams.Out, ...)` sends to stdout regardless of content.
**How to avoid:** Audit every write in env.go — ONLY `KEY=VALUE` lines go to `streams.Out`. Use `logger.Warn(...)` or `fmt.Fprintf(streams.ErrOut, ...)` for all other messages.
**Warning signs:** `eval $(kinder env)` produces a bash error like `bash: KIND: not found`.

### Pitfall 3: kinder env Requires No Running Cluster
**What goes wrong:** Calling `provider.List()` or `provider.KubeConfig()` to verify cluster state — these call docker/podman and fail if the cluster doesn't exist.
**Why it happens:** Natural to check if cluster exists; but env command is for showing configuration intent.
**How to avoid:** kinder env reads: provider name (from env or auto-detect), cluster name (from `KIND_CLUSTER_NAME` env var or default "kind"), kubeconfig path (from `$KUBECONFIG` or `$HOME/.kube/config`). No container runtime calls needed.
**Warning signs:** `kinder env` hangs or errors when called before `kinder create cluster`.

### Pitfall 4: Provider Name When KIND_EXPERIMENTAL_PROVIDER Is Unset
**What goes wrong:** If `KIND_EXPERIMENTAL_PROVIDER` is not set, the provider is auto-detected by calling docker/podman/nerdctl. If no runtime is available, `DetectNodeProvider` returns an error. kinder env should still produce output in this case.
**Why it happens:** The env command calls `cluster.NewProvider()` which calls auto-detection on creation.
**How to avoid:** Check `KIND_EXPERIMENTAL_PROVIDER` env var first. If set, use that value as the provider name directly without calling NewProvider. If unset, attempt auto-detection and warn on stderr if detection fails, defaulting to "docker" (the project's historic default).
**Warning signs:** `kinder env` errors out when docker daemon is not running.

### Pitfall 5: doctor Exit Codes and Deferred Cleanup
**What goes wrong:** Calling `os.Exit(1)` inside RunE bypasses deferred functions — any cleanup registered with `defer` will not run.
**Why it happens:** `os.Exit` terminates immediately.
**How to avoid:** Structure doctor's RunE to collect all results first (no deferred state), then call `os.Exit`. Do not use `defer` for anything that must run before exit in doctor's RunE.
**Warning signs:** Temporary files or resources not cleaned up after doctor runs.

### Pitfall 6: kubectl Check vs kubectl Available
**What goes wrong:** Using `exec.Command("kubectl", "cluster-info")` to check if kubectl is working — this contacts the API server and is slow/fragile.
**Why it happens:** Conflating "is kubectl installed" with "is kubectl connected".
**How to avoid:** For kinder doctor, only check if the binary exists and responds to `kubectl version --client`. This is a local check only. Do not check connectivity.
**Warning signs:** `kinder doctor` times out when there is no running cluster.

## Code Examples

Verified patterns from existing codebase files:

### kinder env — Full Command Structure
```go
// pkg/cmd/kind/env/env.go
package env

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"

    "sigs.k8s.io/kind/pkg/cluster"
    "sigs.k8s.io/kind/pkg/cmd"
    "sigs.k8s.io/kind/pkg/log"

    "sigs.k8s.io/kind/pkg/internal/cli"
    "sigs.k8s.io/kind/pkg/internal/runtime"
)

type flagpole struct {
    Name string
}

// NewCommand returns a new cobra.Command for kinder env
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
    flags := &flagpole{}
    cmd := &cobra.Command{
        Args:  cobra.NoArgs,
        Use:   "env",
        Short: "Prints cluster environment variables in eval-safe key=value format",
        Long:  "Prints cluster environment variables (provider, cluster name, kubeconfig path) to stdout in key=value format suitable for eval",
        RunE: func(cmd *cobra.Command, args []string) error {
            cli.OverrideDefaultName(cmd.Flags())
            return runE(logger, streams, flags)
        },
    }
    cmd.Flags().StringVarP(
        &flags.Name,
        "name",
        "n",
        cluster.DefaultName,
        "the cluster context name",
    )
    return cmd
}

func runE(logger log.Logger, streams cmd.IOStreams, flags *flagpole) error {
    // Resolve provider name — check env var first to avoid runtime calls
    providerName := activeProviderName(logger)

    // Cluster name (already resolved by OverrideDefaultName)
    clusterName := flags.Name
    if clusterName == "" {
        clusterName = cluster.DefaultName
    }

    // Kubeconfig path — same logic as kubectl, no runtime call needed
    kubeconfigPath := activeKubeconfigPath()

    // All output to stdout must be eval-safe key=value
    fmt.Fprintf(streams.Out, "KINDER_PROVIDER=%s\n", providerName)
    fmt.Fprintf(streams.Out, "KINDER_CLUSTER=%s\n", clusterName)
    fmt.Fprintf(streams.Out, "KUBECONFIG=%s\n", kubeconfigPath)
    return nil
}

func activeProviderName(logger log.Logger) string {
    // Honor KIND_EXPERIMENTAL_PROVIDER if set
    if p := os.Getenv("KIND_EXPERIMENTAL_PROVIDER"); p != "" {
        switch p {
        case "docker", "podman", "nerdctl", "finch", "nerdctl.lima":
            return p
        default:
            logger.Warnf("unknown KIND_EXPERIMENTAL_PROVIDER value %q", p)
        }
    }
    // Auto-detect without creating a full Provider (avoids runtime dependency)
    // Use the same detection logic as DetectNodeProvider
    opt, err := cluster.DetectNodeProvider()
    if err != nil {
        logger.Warn("could not detect node provider; defaulting to docker")
        return "docker"
    }
    // Create a minimal provider to get the name
    p := cluster.NewProvider(cluster.ProviderWithLogger(log.NoopLogger{}), opt)
    return p.Name()
}

func activeKubeconfigPath() string {
    if kc := os.Getenv("KUBECONFIG"); kc != "" {
        parts := filepath.SplitList(kc)
        for _, p := range parts {
            if p != "" {
                return p
            }
        }
    }
    return filepath.Join(os.Getenv("HOME"), ".kube", "config")
}
```

### kinder doctor — Binary Check Pattern
```go
// pkg/cmd/kind/doctor/doctor.go
package doctor

import (
    "fmt"
    osexec "os/exec"
    "os"
    "strings"

    "github.com/spf13/cobra"

    "sigs.k8s.io/kind/pkg/cmd"
    "sigs.k8s.io/kind/pkg/exec"
    "sigs.k8s.io/kind/pkg/log"
)

// NewCommand returns a new cobra.Command for kinder doctor
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
    return &cobra.Command{
        Args:  cobra.NoArgs,
        Use:   "doctor",
        Short: "Checks prerequisite binaries and prints actionable fix messages",
        Long:  "Checks for required binaries (container runtime, kubectl) and exits 0 if all ok, 1 on failure, 2 on warnings",
        RunE: func(cmd *cobra.Command, args []string) error {
            return runE(streams)
        },
    }
}

type result struct {
    name    string
    ok      bool
    warn    bool
    message string
}

func runE(streams cmd.IOStreams) error {
    var results []result

    // Check container runtimes (at least one must be present)
    runtimeFound := false
    for _, rt := range []struct{ name, versionPrefix, install string }{
        {"docker", "Docker version", "https://docs.docker.com/get-docker/"},
        {"podman", "podman version", "https://podman.io/getting-started/installation"},
        {"nerdctl", "nerdctl version", "https://github.com/containerd/nerdctl#install"},
    } {
        if checkBinary(rt.name, rt.versionPrefix) {
            results = append(results, result{name: rt.name, ok: true})
            runtimeFound = true
            break
        }
    }
    if !runtimeFound {
        results = append(results, result{
            name:    "container-runtime",
            ok:      false,
            message: "no container runtime found; install Docker (https://docs.docker.com/get-docker/), Podman, or nerdctl",
        })
    }

    // Check kubectl
    if !checkBinary("kubectl", "Client Version") {
        results = append(results, result{
            name:    "kubectl",
            ok:      false,
            message: "kubectl not found; install from https://kubernetes.io/docs/tasks/tools/",
        })
    } else {
        results = append(results, result{name: "kubectl", ok: true})
    }

    hasFail := false
    for _, r := range results {
        if !r.ok {
            hasFail = true
            fmt.Fprintf(streams.ErrOut, "[FAIL] %s: %s\n", r.name, r.message)
        } else {
            fmt.Fprintf(streams.ErrOut, "[ OK ] %s\n", r.name)
        }
    }

    if hasFail {
        os.Exit(1)
    }
    return nil // exit 0
}

// Source: mirrors pattern from pkg/cluster/internal/providers/docker/util.go
func checkBinary(name, versionPrefix string) bool {
    if _, err := osexec.LookPath(name); err != nil {
        return false
    }
    cmd := exec.Command(name, "version")
    lines, err := exec.OutputLines(cmd)
    if err != nil || len(lines) == 0 {
        // binary exists but failed to run; try -v as fallback
        cmd = exec.Command(name, "-v")
        lines, err = exec.OutputLines(cmd)
        if err != nil || len(lines) == 0 {
            return false
        }
    }
    return strings.Contains(lines[0], versionPrefix) || len(lines) > 0
}
```

### Name() method on public cluster.Provider
```go
// To add to pkg/cluster/provider.go
import "fmt"

// Name returns the name of the active node provider backend ("docker", "podman", or "nerdctl").
// Returns "unknown" if the provider does not implement fmt.Stringer.
func (p *Provider) Name() string {
    if s, ok := p.provider.(fmt.Stringer); ok {
        return s.String()
    }
    return "unknown"
}
```

### Root wiring
```go
// pkg/cmd/kind/root.go — add two imports and two AddCommand calls
import (
    // ... existing imports ...
    "sigs.k8s.io/kind/pkg/cmd/kind/doctor"
    "sigs.k8s.io/kind/pkg/cmd/kind/env"
)

// in NewCommand:
cmd.AddCommand(env.NewCommand(logger, streams))
cmd.AddCommand(doctor.NewCommand(logger, streams))
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Cobra error-based exit codes | os.Exit() inside RunE | Always true in Cobra | Must be explicit; no library magic |
| Shell scripts for env info | Native kinder env command | Phase 24 | Machine-readable, no shell parsing required |
| Manual binary prerequisite docs | kinder doctor | Phase 24 | Actionable fix messages in CLI itself |

**Deprecated/outdated:**
- KIND_EXPERIMENTAL_PROVIDER: This env var name is marked as "experimental" in provider.go comments and may change in future. For Phase 24, use it as-is — changing the name is out of scope (v1.4+ per REQUIREMENTS.md CLI-05+).

## Open Questions

1. **Should kinder env output KINDER_CLUSTER or KIND_CLUSTER_NAME?**
   - What we know: `KIND_CLUSTER_NAME` is the input env var the CLI reads. `KINDER_CLUSTER` is a new output var.
   - What's unclear: Using `KIND_CLUSTER_NAME` as the output key would allow round-tripping: `eval $(kinder env)` then `kinder create cluster` picks it up.
   - Recommendation: Output `KIND_CLUSTER_NAME=<name>` for the cluster name field to enable round-trip eval usage. Output `KUBECONFIG=<path>` for kubeconfig (standard var). Output `KINDER_PROVIDER=<name>` for provider (new namespace). Planner should decide the exact key names — the success criteria says "key=value format" without specifying key names.

2. **Should kinder doctor check for warns (exit 2) or only fails (exit 1)?**
   - What we know: CLI-04 specifies exits 0/1/2 with 2=warnings. The success criteria mentions "hard failure" for exit 1 and "warnings" for exit 2.
   - What's unclear: What constitutes a "warning" vs "failure" for binary prerequisites. A binary present but old version = warn? A binary completely absent = fail?
   - Recommendation: Absent binary = fail (exit 1). Binary present but unable to connect to daemon = warn (exit 2). Binary present and working = ok (exit 0). Planner can refine this.

3. **Does kinder env need a running cluster to be useful?**
   - What we know: The success criteria says `eval $(kinder env)` succeeds — implies it must work without a cluster running.
   - What's unclear: Whether to warn on stderr when no cluster with the given name exists.
   - Recommendation: kinder env emits only env vars from the environment/defaults. No container runtime calls. Optionally warn on stderr if KIND_CLUSTER_NAME cluster does not appear to exist (list clusters via provider), but stdout remains clean.

## Sources

### Primary (HIGH confidence)
- `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/root.go` — Cobra command wiring pattern (AddCommand, NewCommand signature)
- `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/version/version.go` — Minimal leaf command pattern
- `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/get/clusters/clusters.go` — fmt.Fprintln(streams.Out) output pattern
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/providers/docker/util.go` — IsAvailable / binary detection pattern
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/providers/nerdctl/provider.go` — osexec.LookPath usage
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go` — fmt.Stringer type assertion for provider name
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/kubeconfig/internal/kubeconfig/paths.go` — Kubeconfig path resolution logic
- `/Users/patrykattc/work/git/kinder/pkg/cmd/iostreams.go` — IOStreams struct (Out=stdout, ErrOut=stderr)
- `/Users/patrykattc/work/git/kinder/pkg/internal/runtime/runtime.go` — KIND_EXPERIMENTAL_PROVIDER handling
- `/Users/patrykattc/work/git/kinder/.planning/REQUIREMENTS.md` — CLI-01 through CLI-04 scope definitions

### Secondary (MEDIUM confidence)
- Cobra v1.8.0 documentation: RunE error returns always produce exit code 1; os.Exit is required for custom codes. Verified by examining cmd/kind/app/main.go which calls `os.Exit(1)` on any Run error — no finer codes.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in go.mod; no new dependencies
- Architecture: HIGH — command structure directly mirrors 5+ existing commands in the codebase
- Pitfalls: HIGH — Cobra exit code limitation verified in main.go; stdout pollution constraint is from success criteria; all provider patterns verified in existing action packages

**Research date:** 2026-03-03
**Valid until:** 2026-04-03 (stable domain — Cobra and stdlib patterns don't change)
