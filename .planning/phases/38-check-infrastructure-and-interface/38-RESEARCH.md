# Phase 38: Check Infrastructure and Interface - Research

**Researched:** 2026-03-06
**Domain:** Go interface design, registry pattern, output formatting, check migration for `kinder doctor`
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Doctor output style
- Bold section headers per category (e.g., `=== Runtime ===`) with checks listed below each
- Unicode status icons: checkmark ok, x fail, warning warn, null skip
- No ANSI colors -- plain text only, works everywhere including piped output
- Summary line at the end: `13 checks: 9 ok, 1 warning, 0 failed, 3 skipped`

#### Check result detail
- Passing checks show name + detected value: `checkmark Container runtime detected (docker)`
- Warnings and failures show inline fix command: `arrow sudo usermod -aG docker $USER`
- Warnings and failures include a one-liner explanation of WHY it matters before the fix
- JSON output uses structured fields: separate `message`, `reason`, and `fix` fields per check result (plus `name`, `category`, `status`)

#### Platform skip behavior
- Skipped checks are visible in output with null icon and short platform tag: `null inotify watches (linux only)`
- Categories with all skipped checks still show their header and skipped checks
- Skipped checks are included in the summary count as a separate "skipped" number
- Skip reasons use concise platform tags: `(linux only)`, `(requires nvidia-smi)` -- not full sentences

#### Mitigation messaging
- Two severity levels only: warning warn (degraded but works) and x fail (will break)
- Auto-fixable issues shown in a separate `=== Auto-mitigations ===` section after all check categories
- Auto-mitigations section only appears when there are fixable issues -- no section when clean
- During `kinder create cluster`, print a brief line per applied mitigation: `Applied mitigation: override init=true`

### Claude's Discretion
- Exact category names and ordering for the three existing checks
- Section header formatting details (separator characters, spacing)
- JSON envelope structure (top-level fields beyond the checks array)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-01 | Check interface with Name(), Category(), Platforms(), Run() methods in pkg/internal/doctor/ | Architecture Patterns section: Check interface definition, deps struct pattern, placement in pkg/internal/doctor/ |
| INFRA-02 | Result type with ok/warn/fail/skip statuses and category-grouped output | Architecture Patterns section: Result type with 6 fields (Name, Status, Message, Reason, Fix, Category), output formatter pattern |
| INFRA-03 | AllChecks() registry with centralized platform filtering | Architecture Patterns section: Explicit registry slice, centralized platform filter loop |
| INFRA-04 | Mitigation tier system (auto-apply, suggest-only, document-only) with SafeMitigation struct | Architecture Patterns section: SafeMitigation struct with NeedsFix/Apply/NeedsRoot fields, tier definitions, skeleton ApplySafeMitigations() |
| INFRA-06 | Existing checks (container-runtime, kubectl, NVIDIA GPU) migrated to Check interface | Code Examples section: Migration patterns for all three existing checks with deps struct injection |
</phase_requirements>

## Summary

Phase 38 creates the foundational check infrastructure for the expanded `kinder doctor` command. The current `doctor.go` is a monolithic 298-line file with inline check functions (`checkBinary`, `checkKubectl`, `checkNvidiaDriver`, etc.), flat `[ OK ]/[WARN]/[FAIL]` output, and no category grouping. This phase extracts check logic into a shared `pkg/internal/doctor/` package with a `Check` interface, `Result` type, `AllChecks()` registry, and `SafeMitigation` struct, then migrates the three existing checks (container runtime, kubectl, NVIDIA GPU) to that interface. The doctor command is refactored to produce category-grouped output with the user-specified Unicode icons and summary line.

The key architectural decisions are already validated by prior project research: the Check interface lives in `pkg/internal/doctor/` (not `pkg/cmd/`) because the create flow in `pkg/cluster/internal/create/create.go` needs to import mitigation functions in Phase 41. Each check type uses a deps struct for injectable dependencies (not package-level vars), enabling fully parallel tests. Platform filtering is centralized in the run loop, not inside individual checks. The mitigation tier system is defined as a skeleton (struct + entry point) but not wired into the create flow until Phase 41.

No new go.mod dependencies are required. All check logic uses Go stdlib (`os`, `os/exec`, `encoding/json`, `runtime`, `strings`, `strconv`, `fmt`) and the existing `sigs.k8s.io/kind/pkg/exec` wrapper. The `golang.org/x/sys` dependency (already indirect at v0.41.0) is not needed until Phases 39-40 (disk space, kernel version checks).

**Primary recommendation:** Create `pkg/internal/doctor/` with Check interface, Result type (6-field struct matching user's JSON spec), AllChecks() registry with centralized platform filter, SafeMitigation skeleton, then migrate all 3 existing checks using deps struct injection pattern, and rewrite `doctor.go` to use category-grouped output with the locked Unicode icon format.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib (`os`, `os/exec`, `encoding/json`, `runtime`, `strings`, `fmt`) | Go 1.24+ | All check logic, JSON output, platform detection | Already used by existing doctor.go; zero new deps |
| `sigs.k8s.io/kind/pkg/exec` | (internal) | Command execution abstraction with Cmd interface | Already used by existing doctor checks; enables FakeCmd test injection |
| `sigs.k8s.io/kind/pkg/cmd` | (internal) | IOStreams for stdout/stderr routing | Already used by doctor.go NewCommand |
| `sigs.k8s.io/kind/pkg/log` | (internal) | Logger interface | Already used by doctor.go NewCommand |
| `github.com/spf13/cobra` | v1.8.0 | CLI command framework | Already used by doctor.go |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/cluster/internal/create/actions/testutil` | (internal) | FakeCmd, FakeNode test infrastructure | Unit testing checks that shell out to docker/kubectl/nvidia-smi |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Deps struct injection | Package-level var injection (existing NVIDIA pattern) | Package-level vars force sequential tests at 13+ checks; deps struct enables full parallel |
| Explicit AllChecks() slice | init()-based auto-registration | init() makes check ordering non-deterministic; explicit slice is discoverable and matches kinder's existing flat-list patterns |
| Shell out to `getenforce` | `opencontainers/selinux` library | Library adds 19 transitive imports; shell-out is consistent with existing doctor pattern |

**Installation:**
```bash
# No new dependencies needed. go.mod unchanged for this phase.
```

## Architecture Patterns

### Recommended Project Structure
```
pkg/internal/doctor/
    check.go              # Check interface, Result type, AllChecks() registry, platform filter
    check_test.go         # Registry tests, platform filter tests
    runtime.go            # Container runtime check (migrated from doctor.go)
    runtime_test.go
    tools.go              # kubectl check (migrated from doctor.go)
    tools_test.go
    gpu.go                # NVIDIA GPU checks (migrated from doctor.go)
    gpu_test.go
    mitigations.go        # SafeMitigation struct, tier system, ApplySafeMitigations() skeleton
    mitigations_test.go
    format.go             # Output formatting: category-grouped text, JSON envelope
    format_test.go

pkg/cmd/kind/doctor/
    doctor.go             # MODIFIED: CLI command, delegates to pkg/internal/doctor/
```

### Pattern 1: Check Interface with Deps Struct
**What:** Each check implements a 4-method interface with dependencies injected via struct fields, not package-level vars.
**When to use:** Every check implementation.
**Example:**
```go
// Source: codebase analysis of pkg/internal/doctor/ design
// (validated by .planning/research/ARCHITECTURE.md)

// Check represents a single diagnostic check.
type Check interface {
    Name() string       // e.g., "container-runtime"
    Category() string   // e.g., "Runtime"
    Platforms() []string // e.g., []string{"linux"} or nil for all
    Run() []Result
}

// Result is the outcome of a single check. JSON tags match user's locked spec.
type Result struct {
    Name     string `json:"name"`
    Category string `json:"category"`
    Status   string `json:"status"`           // "ok", "warn", "fail", "skip"
    Message  string `json:"message,omitempty"` // detected value or problem description
    Reason   string `json:"reason,omitempty"`  // WHY it matters (warn/fail only)
    Fix      string `json:"fix,omitempty"`     // fix command (warn/fail only)
}

// containerRuntimeCheck implements Check with injectable deps.
type containerRuntimeCheck struct {
    lookPath func(string) (string, error)
    execCmd  func(name string, args ...string) exec.Cmd
}

func newContainerRuntimeCheck() Check {
    return &containerRuntimeCheck{
        lookPath: osexec.LookPath,
        execCmd:  exec.Command,
    }
}

func (c *containerRuntimeCheck) Name() string        { return "container-runtime" }
func (c *containerRuntimeCheck) Category() string     { return "Runtime" }
func (c *containerRuntimeCheck) Platforms() []string  { return nil } // all platforms
func (c *containerRuntimeCheck) Run() []Result        { /* ... */ }
```

### Pattern 2: Centralized Platform Filtering
**What:** The run loop checks `Platforms()` and emits skip results, keeping platform logic out of individual checks.
**When to use:** Always -- in the main `RunAllChecks()` function.
**Example:**
```go
// Source: codebase analysis + .planning/research/ARCHITECTURE.md

// RunAllChecks executes all checks with platform filtering.
// Returns ordered results grouped by category.
func RunAllChecks() []Result {
    var results []Result
    for _, check := range AllChecks() {
        platforms := check.Platforms()
        if len(platforms) > 0 && !containsString(platforms, runtime.GOOS) {
            results = append(results, Result{
                Name:     check.Name(),
                Category: check.Category(),
                Status:   "skip",
                Message:  platformSkipMessage(platforms),
            })
            continue
        }
        results = append(results, check.Run()...)
    }
    return results
}

// platformSkipMessage generates concise platform tag per user spec.
// e.g., "(linux only)" or "(requires nvidia-smi)"
func platformSkipMessage(platforms []string) string {
    return "(" + strings.Join(platforms, "/") + " only)"
}
```

### Pattern 3: Category-Grouped Output Formatter
**What:** Results are rendered grouped by category with the user's locked Unicode format.
**When to use:** Human-readable output mode in the doctor command.
**Example:**
```go
// Source: user decisions in CONTEXT.md

// FormatHumanReadable renders results grouped by category.
// Output goes to streams.ErrOut (matching existing doctor behavior).
func FormatHumanReadable(w io.Writer, results []Result) {
    // Group by category preserving order
    categories := groupByCategory(results)

    for _, cat := range categories {
        fmt.Fprintf(w, "\n=== %s ===\n", cat.Name)
        for _, r := range cat.Results {
            switch r.Status {
            case "ok":
                fmt.Fprintf(w, "  \u2713 %s\n", r.Message) // checkmark
            case "fail":
                fmt.Fprintf(w, "  \u2717 %s\n", r.Name)     // x mark
                fmt.Fprintf(w, "    %s\n", r.Reason)
                fmt.Fprintf(w, "    \u2192 %s\n", r.Fix)    // arrow
            case "warn":
                fmt.Fprintf(w, "  \u26A0 %s\n", r.Name)     // warning
                fmt.Fprintf(w, "    %s\n", r.Reason)
                fmt.Fprintf(w, "    \u2192 %s\n", r.Fix)    // arrow
            case "skip":
                fmt.Fprintf(w, "  \u2298 %s %s\n", r.Name, r.Message) // null
            }
        }
    }

    // Summary separator and count
    fmt.Fprintf(w, "\n\u2500\u2500\u2500\n") // horizontal line
    ok, warn, fail, skip := countStatuses(results)
    total := ok + warn + fail + skip
    fmt.Fprintf(w, "%d checks: %d ok, %d warning, %d failed, %d skipped\n",
        total, ok, warn, fail, skip)
}
```

### Pattern 4: SafeMitigation Skeleton
**What:** Mitigation tier system defined as struct + entry point, but not wired until Phase 41.
**When to use:** Phase 38 defines the skeleton; Phases 39-41 populate it.
**Example:**
```go
// Source: .planning/research/ARCHITECTURE.md + user decisions

// SafeMitigation represents an idempotent, non-destructive fix
// that can be auto-applied during cluster creation.
type SafeMitigation struct {
    Name      string
    NeedsFix  func() bool   // returns true if mitigation is needed
    Apply     func() error  // applies the mitigation
    NeedsRoot bool          // true if mitigation requires root/sudo
}

// SafeMitigations returns the list of mitigations safe for auto-apply.
// Populated in later phases as checks are added.
func SafeMitigations() []SafeMitigation {
    return nil // skeleton -- populated in Phases 39-41
}

// ApplySafeMitigations runs all safe mitigations. Called from create flow.
// Returns errors for mitigations that failed (informational, not fatal).
func ApplySafeMitigations(logger log.Logger) []error {
    if runtime.GOOS != "linux" {
        return nil
    }
    var errs []error
    for _, m := range SafeMitigations() {
        if !m.NeedsFix() {
            continue
        }
        if m.NeedsRoot && os.Geteuid() != 0 {
            logger.Warnf("Skipping %s mitigation (requires root)", m.Name)
            continue
        }
        if err := m.Apply(); err != nil {
            errs = append(errs, fmt.Errorf("%s: %w", m.Name, err))
        } else {
            logger.V(0).Infof("Applied mitigation: %s", m.Name)
        }
    }
    return errs
}
```

### Pattern 5: JSON Envelope Structure (Claude's Discretion)
**What:** JSON output wraps the checks array with metadata fields.
**Recommended structure:**
```json
{
  "checks": [
    {
      "name": "container-runtime",
      "category": "Runtime",
      "status": "ok",
      "message": "Container runtime detected (docker)",
      "reason": "",
      "fix": ""
    }
  ],
  "summary": {
    "total": 6,
    "ok": 4,
    "warn": 0,
    "fail": 0,
    "skip": 2
  }
}
```
**Rationale:** Top-level object (not bare array) is more extensible. The `summary` field lets JSON consumers skip counting. Fields `reason` and `fix` are empty strings for ok/skip results (not omitted) to keep the schema consistent.

### Category Names and Ordering (Claude's Discretion)
**Recommended for the three existing checks:**
1. **Runtime** -- container-runtime check
2. **Tools** -- kubectl check
3. **GPU** -- nvidia-driver, nvidia-container-toolkit, nvidia-docker-runtime checks

**Rationale:** "Runtime" is the broadest and most critical (is Docker even running?). "Tools" covers CLI prerequisites. "GPU" is specialized and Linux-only, displayed last. This ordering mirrors the existing doctor.go execution order, so migrated output looks familiar.

### Anti-Patterns to Avoid
- **init()-based registration:** Kinder uses explicit slices everywhere (addon waves, action lists). init() makes ordering non-deterministic and hides what runs.
- **Checks that modify system state:** Doctor is read-only. Mitigations are separate and only called from the create flow.
- **Package-level vars for every dependency:** With 3+ checks now and 13+ in future phases, package-level vars create sequential test bottlenecks. Use deps struct.
- **Check logic in pkg/cmd/kind/doctor/:** The create flow (`pkg/cluster/internal/create/`) needs mitigation access. Putting logic in `pkg/cmd/` inverts the dependency direction. Use `pkg/internal/doctor/`.
- **Returning error from Check.Run():** The Run method returns `[]Result`, not `error`. All error conditions (command not found, file unreadable) are expressed as warn/fail results, keeping the interface uniform.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Command execution | Raw `os/exec.Command` calls | `sigs.k8s.io/kind/pkg/exec.Command` wrapper | The wrapper provides `OutputLines`, `Output`, `CombinedOutputLines` helpers and the `Cmd` interface for test injection via FakeCmd |
| Version comparison | String comparison on version numbers | `sigs.k8s.io/kind/pkg/internal/version.ParseSemantic` / `ParseGeneric` | Already exists in codebase; handles `v1.31.0` format, comparison operators, pre-release |
| Exit code computation | Manual if/else chains | Centralized `ExitCodeFromResults(results)` function | Keep exit code logic in one place; skip does NOT trigger exit 2 (existing contract) |
| Category ordering | Re-sorting results after collection | Preserve insertion order from AllChecks() registry | The registry defines the canonical order; groupByCategory should preserve first-seen order |

**Key insight:** The existing codebase already has all the building blocks: `exec.Cmd` interface for testing, `IOStreams` for output routing, `FakeCmd`/`FakeNode` for test doubles, and `version.Parse*` for version comparison. This phase is an integration and refactoring effort, not a greenfield build.

## Common Pitfalls

### Pitfall 1: /proc Reads Crash on Non-Linux
**What goes wrong:** Checks that read `/proc/sys/fs/inotify/max_user_watches` compile fine on macOS but crash at runtime with "no such file or directory". Developers test on macOS with FakeCmd and never exercise the real code path.
**Why it happens:** Go compiles `/proc` file reads on all platforms without error. There's no compile-time signal that the code is Linux-specific.
**How to avoid:** Every check declares Platforms() metadata. The centralized filter in RunAllChecks() emits skip results before calling Run(). Individual check Run() methods never see non-applicable platforms.
**Warning signs:** A check's Run() method contains `if runtime.GOOS != "linux"` -- that logic should be in Platforms() metadata and handled by the runner.

### Pitfall 2: Skip Status Treated as Warning for Exit Codes
**What goes wrong:** Adding "skip" as a fourth status but computing exit code 2 (warning) for it. macOS users always get exit 2 due to skipped Linux-only checks, making exit codes useless for CI.
**Why it happens:** The existing exit code logic checks `hasFail` and `hasWarn`. A naive addition of skip status may accidentally fall into the "warn" bucket.
**How to avoid:** Explicit exit code function: 0 for all-ok-or-skip, 1 for any fail, 2 for any warn but no fail. Test on macOS: all Linux checks skip, exit code is 0.
**Warning signs:** `kinder doctor` on macOS exits with code 2 when all applicable checks pass.

### Pitfall 3: NVIDIA Check Migration Breaks Existing Output
**What goes wrong:** The three existing NVIDIA checks produce specific message formats that scripts/users may depend on. Migration changes the output structure (adding category headers, changing icon format from `[ OK ]` to unicode checkmark).
**Why it happens:** The migration changes both the check execution path AND the output formatter simultaneously.
**How to avoid:** Verify that migrated check messages contain the same key information (driver version string, fix commands). The output format is intentionally changing (user decision: unicode icons, category headers), but the content within each check result must be identical or improved.
**Warning signs:** Existing `docker info --format` or `nvidia-smi --query-gpu=driver_version` queries produce different output after migration.

### Pitfall 4: JSON Output Backward Compatibility
**What goes wrong:** The existing JSON output is a bare array `[{"name":"docker","status":"ok","message":""}]`. Wrapping in an envelope `{"checks":[...]}` breaks existing JSON consumers.
**Why it happens:** The user spec requires `category`, `reason`, and `fix` fields which don't fit the old 3-field struct.
**How to avoid:** The JSON output format is changing by user decision (new fields). This is an intentional breaking change for v2.1. The new envelope structure (`{"checks":[...],"summary":{...}}`) is the target. Document the change.
**Warning signs:** None -- this is an intentional format upgrade.

### Pitfall 5: NVIDIA Container Runtime Check Assumes Docker
**What goes wrong:** The existing `checkNvidiaDockerRuntime()` runs `docker info --format {{.Runtimes}}` which only works with Docker. If the user has Podman or Nerdctl, this check fails with a confusing error.
**Why it happens:** The NVIDIA GPU addon is Docker-specific in the current implementation.
**How to avoid:** The migrated NVIDIA docker-runtime check should detect the active container runtime first. If it's not Docker, skip the check with `(requires docker)` platform tag.
**Warning signs:** `kinder doctor` on a Podman system shows NVIDIA docker-runtime as "warn" when there's no NVIDIA setup at all.

## Code Examples

Verified patterns from the existing codebase:

### Existing Doctor Check Pattern (to migrate FROM)
```go
// Source: pkg/cmd/kind/doctor/doctor.go lines 79-108
// This is what we're migrating away from:

runtimes := []string{"docker", "podman", "nerdctl"}
foundRuntime := false
for _, rt := range runtimes {
    found, working := checkBinary(rt)
    if found && working {
        results = append(results, result{name: rt, status: "ok"})
        foundRuntime = true
        break
    }
    // ...
}
```

### Migrated Check Implementation (to migrate TO)
```go
// Source: design based on .planning/research/ARCHITECTURE.md

type containerRuntimeCheck struct {
    lookPath func(string) (string, error)
    execCmd  func(name string, args ...string) exec.Cmd
}

func newContainerRuntimeCheck() Check {
    return &containerRuntimeCheck{
        lookPath: osexec.LookPath,
        execCmd:  exec.Command,
    }
}

func (c *containerRuntimeCheck) Name() string        { return "container-runtime" }
func (c *containerRuntimeCheck) Category() string     { return "Runtime" }
func (c *containerRuntimeCheck) Platforms() []string  { return nil } // all platforms

func (c *containerRuntimeCheck) Run() []Result {
    runtimes := []string{"docker", "podman", "nerdctl"}
    for _, rt := range runtimes {
        if _, err := c.lookPath(rt); err != nil {
            continue
        }
        lines, err := exec.OutputLines(c.execCmd(rt, "version"))
        if err == nil && len(lines) > 0 {
            return []Result{{
                Name:     c.Name(),
                Category: c.Category(),
                Status:   "ok",
                Message:  fmt.Sprintf("Container runtime detected (%s)", rt),
            }}
        }
        // Binary found but not responding
        return []Result{{
            Name:     c.Name(),
            Category: c.Category(),
            Status:   "warn",
            Message:  rt + " found but not responding",
            Reason:   "The container daemon must be running for cluster creation",
            Fix:      fmt.Sprintf("Start the %s daemon or check its status", rt),
        }}
    }
    return []Result{{
        Name:     c.Name(),
        Category: c.Category(),
        Status:   "fail",
        Message:  "No container runtime found",
        Reason:   "A container runtime (Docker, Podman, or nerdctl) is required",
        Fix:      "Install Docker: https://docs.docker.com/get-docker/",
    }}
}
```

### Test Pattern for Migrated Check
```go
// Source: based on existing testutil/fake.go + ARCHITECTURE.md deps struct pattern

func TestContainerRuntimeCheck_DockerFound(t *testing.T) {
    t.Parallel()
    check := &containerRuntimeCheck{
        lookPath: func(name string) (string, error) {
            if name == "docker" {
                return "/usr/bin/docker", nil
            }
            return "", errors.New("not found")
        },
        execCmd: func(name string, args ...string) exec.Cmd {
            return &testutil.FakeCmd{Output: []byte("Docker version 24.0.7\n")}
        },
    }
    results := check.Run()
    if len(results) != 1 {
        t.Fatalf("expected 1 result, got %d", len(results))
    }
    if results[0].Status != "ok" {
        t.Errorf("expected ok, got %s", results[0].Status)
    }
    if !strings.Contains(results[0].Message, "docker") {
        t.Errorf("expected message to mention docker, got %q", results[0].Message)
    }
}
```

### NVIDIA GPU Check Migration Pattern
```go
// Source: migrating from pkg/cmd/kind/doctor/doctor.go checkNvidiaDriver()

type nvidiaDriverCheck struct {
    lookPath func(string) (string, error)
    execCmd  func(name string, args ...string) exec.Cmd
}

func newNvidiaDriverCheck() Check {
    return &nvidiaDriverCheck{
        lookPath: osexec.LookPath,
        execCmd:  exec.Command,
    }
}

func (c *nvidiaDriverCheck) Name() string        { return "nvidia-driver" }
func (c *nvidiaDriverCheck) Category() string     { return "GPU" }
func (c *nvidiaDriverCheck) Platforms() []string  { return []string{"linux"} }

func (c *nvidiaDriverCheck) Run() []Result {
    if _, err := c.lookPath("nvidia-smi"); err != nil {
        return []Result{{
            Name:     c.Name(),
            Category: c.Category(),
            Status:   "warn",
            Message:  "nvidia-smi not found",
            Reason:   "NVIDIA GPU addon requires the NVIDIA driver to be installed",
            Fix:      "Install drivers: https://www.nvidia.com/drivers",
        }}
    }
    lines, err := exec.OutputLines(c.execCmd(
        "nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader",
    ))
    if err != nil || len(lines) == 0 {
        return []Result{{
            Name:     c.Name(),
            Category: c.Category(),
            Status:   "warn",
            Message:  "nvidia-smi found but could not query driver version",
            Reason:   "The GPU may not be accessible to the current user",
            Fix:      "Check GPU access: nvidia-smi",
        }}
    }
    return []Result{{
        Name:     c.Name(),
        Category: c.Category(),
        Status:   "ok",
        Message:  fmt.Sprintf("NVIDIA driver %s", strings.TrimSpace(lines[0])),
    }}
}
```

### Refactored doctor.go Command (CLI layer)
```go
// Source: design based on existing doctor.go + new infrastructure

func runE(streams cmd.IOStreams, flags *flagpole) error {
    switch flags.Output {
    case "", "json":
        // valid
    default:
        return fmt.Errorf("unsupported output format %q", flags.Output)
    }

    results := doctor.RunAllChecks()

    if flags.Output == "json" {
        envelope := doctor.JSONEnvelope(results)
        if err := json.NewEncoder(streams.Out).Encode(envelope); err != nil {
            return err
        }
    } else {
        doctor.FormatHumanReadable(streams.ErrOut, results)
    }

    os.Exit(doctor.ExitCodeFromResults(results))
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Inline check functions in doctor.go | Check interface with registry in pkg/internal/doctor/ | Phase 38 (now) | Enables 13+ new checks in Phases 39-40 without touching doctor.go |
| Flat `[ OK ]/[WARN]/[FAIL]` output | Category-grouped Unicode output with summary line | Phase 38 (now) | Better readability at 19+ checks |
| 3-field JSON struct (name, status, message) | 6-field struct (name, category, status, message, reason, fix) in envelope | Phase 38 (now) | Richer JSON for scripting/CI consumers |
| Platform gating via `runtime.GOOS == "linux"` in code | Declarative Platforms() metadata with centralized filter | Phase 38 (now) | Adds "skip" status; macOS users see all checks exist |
| No mitigation system | SafeMitigation struct with tier system (skeleton) | Phase 38 (now) | Foundation for auto-mitigations in Phase 41 |

**Deprecated/outdated:**
- The existing `result` and `checkResult` types in `pkg/cmd/kind/doctor/doctor.go` are replaced by `doctor.Result` in `pkg/internal/doctor/`
- The inline `checkBinary()`, `checkKubectl()`, `checkNvidiaDriver()`, `checkNvidiaContainerToolkit()`, `checkNvidiaDockerRuntime()` functions are replaced by Check interface implementations
- Package-level var injection pattern (as used in NVIDIA GPU addon) is replaced by deps struct pattern for new check implementations

## Open Questions

1. **NVIDIA docker-runtime check on non-Docker runtimes**
   - What we know: The existing check hardcodes `docker info --format {{.Runtimes}}`. Podman and nerdctl have different ways to query runtimes.
   - What's unclear: Should the check skip entirely on non-Docker runtimes, or should it check the equivalent for each runtime?
   - Recommendation: For Phase 38, add `(requires docker)` as a skip condition when the active runtime is not Docker. Extending to Podman/nerdctl can be done in a future phase.

2. **Exit code behavior for the auto-mitigations section**
   - What we know: User decided auto-fixable issues appear in a separate `=== Auto-mitigations ===` section.
   - What's unclear: Does the mitigation section content affect exit codes? If a check fails but has an available auto-mitigation, is it exit 1 (fail) or exit 0 (fixable)?
   - Recommendation: Exit codes reflect the CHECK status, not mitigation availability. A failing check with available mitigation still exits 1. The mitigation section is informational only for doctor output; actual application happens during `kinder create cluster` (Phase 41).

3. **Docker runtime detection for NVIDIA checks without importing provider internals**
   - What we know: pkg/cmd/kind/doctor/ cannot import pkg/cluster/internal/providers/.
   - What's unclear: How to detect the active container runtime from the doctor package.
   - Recommendation: Shell out to `docker info` / `podman info` to detect the runtime, matching the existing pattern. The doctor command already imports and uses `exec.Command`.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None needed -- `go test ./...` works |
| Quick run command | `go test ./pkg/internal/doctor/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INFRA-01 | Check interface with Name/Category/Platforms/Run | unit | `go test ./pkg/internal/doctor/... -run TestCheckInterface -x` | No -- Wave 0 |
| INFRA-02 | Result type with ok/warn/fail/skip and category-grouped output | unit | `go test ./pkg/internal/doctor/... -run TestFormat -x` | No -- Wave 0 |
| INFRA-03 | AllChecks() registry with platform filtering | unit | `go test ./pkg/internal/doctor/... -run TestAllChecks -x` | No -- Wave 0 |
| INFRA-04 | SafeMitigation struct and ApplySafeMitigations skeleton | unit | `go test ./pkg/internal/doctor/... -run TestMitigation -x` | No -- Wave 0 |
| INFRA-06 | Existing checks migrated produce correct results | unit | `go test ./pkg/internal/doctor/... -run TestRuntime -x` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./pkg/internal/doctor/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before verify-work

### Wave 0 Gaps
- [ ] `pkg/internal/doctor/check_test.go` -- covers INFRA-01, INFRA-03 (interface, registry, platform filter)
- [ ] `pkg/internal/doctor/format_test.go` -- covers INFRA-02 (output formatting, JSON envelope)
- [ ] `pkg/internal/doctor/runtime_test.go` -- covers INFRA-06 (container-runtime check migration)
- [ ] `pkg/internal/doctor/tools_test.go` -- covers INFRA-06 (kubectl check migration)
- [ ] `pkg/internal/doctor/gpu_test.go` -- covers INFRA-06 (NVIDIA checks migration)
- [ ] `pkg/internal/doctor/mitigations_test.go` -- covers INFRA-04 (SafeMitigation skeleton)

## Sources

### Primary (HIGH confidence)
- Direct codebase analysis: `pkg/cmd/kind/doctor/doctor.go` -- current 298-line monolithic doctor implementation with 3 check groups
- Direct codebase analysis: `pkg/exec/types.go` -- Cmd interface definition (Run, SetEnv, SetStdin, SetStdout, SetStderr)
- Direct codebase analysis: `pkg/cluster/internal/create/actions/testutil/fake.go` -- FakeCmd, FakeNode, FakeProvider test doubles
- Direct codebase analysis: `pkg/cluster/internal/create/create.go` -- create flow with validateProvider() before Provision(), addon wave system
- Direct codebase analysis: `pkg/cmd/kind/root.go` -- command registration pattern for doctor subcommand
- Direct codebase analysis: `pkg/cmd/iostreams.go` -- IOStreams type definition (In, Out, ErrOut)
- Direct codebase analysis: `pkg/internal/version/version.go` -- ParseSemantic, ParseGeneric for version comparison
- Direct codebase analysis: `go.mod` -- Go 1.24 minimum, no new dependencies needed, golang.org/x/sys v0.41.0 already indirect
- Prior project research: `.planning/research/ARCHITECTURE.md` -- Check interface design, deps struct pattern, package placement, migration path
- Prior project research: `.planning/research/FEATURES.md` -- All 13 check specifications with detection methods and thresholds
- Prior project research: `.planning/research/PITFALLS.md` -- 7 critical pitfalls with prevention strategies
- Prior project research: `.planning/research/STACK.md` -- Zero new dependencies confirmation, stdlib-only implementation

### Secondary (MEDIUM confidence)
- Kind Known Issues page: https://kind.sigs.k8s.io/docs/user/known-issues/ -- ground truth for check specifications
- Kubernetes version skew policy: https://kubernetes.io/releases/version-skew-policy/ -- kubectl compatibility rules

### Tertiary (LOW confidence)
- None -- all findings verified against codebase or official documentation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- zero new dependencies, all stdlib + existing internal packages, verified against go.mod
- Architecture: HIGH -- Check interface, Result type, registry pattern, package placement all validated by extensive prior codebase research
- Pitfalls: HIGH -- all pitfalls identified from direct codebase analysis and prior research documents
- Code examples: HIGH -- all examples derived from or directly referencing existing codebase patterns

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable domain -- Go interfaces, no external dependencies)
