# Architecture Patterns: Doctor Check Integration

**Domain:** Diagnostic check integration into existing kinder doctor command
**Researched:** 2026-03-06
**Overall confidence:** HIGH (based on direct codebase analysis, no external dependencies)

## Recommended Architecture

### Decision 1: Check Registration Pattern -- Lightweight Check Interface with Registry Slice

**Recommendation:** Introduce a `Check` interface and a registry slice in a shared internal package. Do NOT use a plugin/auto-registration pattern.

**Rationale:** The existing code uses direct function calls (`checkBinary`, `checkKubectl`, `checkNvidiaDriver`, etc.) with results appended inline. This works for 6 checks but becomes unmaintainable at 19+. However, kinder's codebase favors simplicity over abstraction (e.g., the action system uses a flat `[]actions.Action` slice in `create.go`, not a plugin registry). A lightweight interface with an explicit slice preserves this philosophy while making checks composable and testable.

```go
// Check represents a single diagnostic check.
type Check interface {
    // Name returns the display name for this check (e.g., "ip-forwarding").
    Name() string
    // Category returns the category grouping (e.g., "kernel", "docker", "network").
    Category() string
    // Platforms returns the set of GOOS values this check applies to.
    // An empty slice means "all platforms".
    Platforms() []string
    // Run executes the check and returns one or more results.
    Run() []Result
}
```

**Why not individual functions:** The existing pattern of `checkNvidiaDriver() []result` provides no metadata (name, category, platform). Each check is called manually, so adding 13 more would require 13 new conditional blocks in `runE`. An interface lets the main loop handle platform filtering, categorized output, and JSON serialization generically.

**Why not auto-registration (init):** The action system in `create.go` explicitly lists addons in `wave1` and `wave2` slices. Kinder does not use init()-based registration anywhere. An explicit slice keeps check ordering deterministic and discoverable -- you read the slice, you know what runs.

**Explicit registry:**

```go
// allChecks returns the ordered list of all doctor checks.
// This is the single source of truth for which checks exist and their order.
func AllChecks() []Check {
    return []Check{
        // Category: Runtime
        newContainerRuntimeCheck(),
        newKubectlCheck(),
        newDockerCgroupDriverCheck(),
        newDockerStorageDriverCheck(),
        // Category: Kernel
        newIPForwardingCheck(),
        newBridgeNFCallCheck(),
        newConntrackCheck(),
        newSwapCheck(),
        // Category: Network
        newPortAvailabilityCheck(),
        newDNSResolutionCheck(),
        newProxyEnvCheck(),
        // Category: Resources
        newDiskSpaceCheck(),
        newMemoryCheck(),
        newCPUCheck(),
        newInotifyCheck(),
        // Category: GPU (Linux only)
        newNvidiaDriverCheck(),
        newNvidiaContainerToolkitCheck(),
        newNvidiaDockerRuntimeCheck(),
    }
}
```

### Decision 2: Platform-Specific Checks -- Explicit "skip" Status with Reason

**Recommendation:** Add a `"skip"` status to the result type. Checks that do not apply to the current platform return `status: "skip"` with a message like `"Linux only"`.

**Rationale:** Silent skipping hides information from the user. The user running `kinder doctor` on macOS should see that kernel checks exist but are not applicable, so they know what would be checked on the target Linux host. JSON consumers need this too -- an absent check vs. a skipped check have different semantics.

```go
type Result struct {
    Name     string
    Status   string // "ok", "warn", "fail", "skip"
    Message  string
    Category string
}
```

**Human-readable output:**
```
[ OK ] docker
[ OK ] kubectl
[SKIP] ip-forwarding: Linux only
[SKIP] bridge-nf-call: Linux only
[ OK ] disk-space: 45.2 GB available
```

**JSON output:**
```json
[
  {"name": "docker", "status": "ok", "category": "runtime"},
  {"name": "ip-forwarding", "status": "skip", "message": "Linux only", "category": "kernel"}
]
```

**Platform filtering in the main loop** (not in each check):

```go
for _, check := range checks.AllChecks() {
    platforms := check.Platforms()
    if len(platforms) > 0 && !contains(platforms, runtime.GOOS) {
        results = append(results, checks.Result{
            Name:     check.Name(),
            Status:   "skip",
            Message:  fmt.Sprintf("%s only", strings.Join(platforms, "/")),
            Category: check.Category(),
        })
        continue
    }
    results = append(results, check.Run()...)
}
```

This keeps platform logic out of individual check implementations. The `Platforms()` method on the interface is pure metadata; the filtering decision is centralized.

### Decision 3: Auto-Mitigations -- Shared Package, Doctor Reports, Create Applies

**Recommendation:** Create check implementations AND their associated mitigation functions in a shared `pkg/internal/doctor/` package. Doctor reports mitigations as suggestions. The create flow calls mitigations automatically before provisioning.

**Why `pkg/internal/doctor/` (not `pkg/cmd/kind/doctor/checks/`):** The create flow in `pkg/cluster/internal/create/create.go` needs to import mitigation functions. It already imports from `pkg/internal/` (e.g., `pkg/internal/apis/config/`, `pkg/internal/cli/`). The `pkg/cmd/` layer is CLI-specific and should NOT be imported by cluster internals -- that would invert the dependency direction.

**Package structure:**

```
pkg/internal/doctor/
    check.go           -- Check interface, Result type, AllChecks() registry
    runtime.go         -- Container runtime checks
    runtime_test.go
    kernel.go          -- Kernel parameter checks (ip_forward, bridge-nf-call, etc.)
    kernel_test.go
    network.go         -- Network checks (ports, DNS, proxy env)
    network_test.go
    resources.go       -- Resource checks (disk, memory, CPU, inotify)
    resources_test.go
    gpu.go             -- NVIDIA GPU checks (migrated from doctor.go)
    gpu_test.go
    mitigations.go     -- Mitigation functions (sysctl, etc.)
    mitigations_test.go

pkg/cmd/kind/doctor/
    doctor.go          -- CLI command, imports pkg/internal/doctor/

pkg/cluster/internal/create/
    create.go          -- Imports pkg/internal/doctor/ for pre-flight mitigations
```

**Doctor behavior (report only):**
```
[FAIL] ip-forwarding: net.ipv4.ip_forward is 0
       Fix: sudo sysctl -w net.ipv4.ip_forward=1
[WARN] bridge-nf-call: bridge-nf-call-iptables is 0
       Fix: sudo sysctl -w net.bridge.bridge-nf-call-iptables=1
```

**Create behavior (auto-apply non-destructive mitigations):**

The create flow should auto-apply non-destructive, idempotent mitigations (like setting sysctl values) before provisioning. This matches the existing `validateProvider()` pattern in `create.go` -- pre-flight checks that block creation. The distinction:

| Mitigation Type | Doctor | Create |
|----------------|--------|--------|
| Non-destructive sysctl (ip_forward=1) | Report with fix command | Auto-apply silently |
| Destructive (swapoff) | Report with fix command | Warn but do NOT auto-apply |
| Missing binary (nvidia-smi) | Report install instructions | Fail with error |
| Configuration (Docker cgroup driver) | Report fix steps | Warn, do NOT auto-apply |

```go
// In create.go, before p.Provision():
if errs := doctor.ApplySafeMitigations(logger); len(errs) > 0 {
    for _, err := range errs {
        logger.Warnf("Pre-flight mitigation: %v", err)
    }
}
```

### Decision 4: Check Organization -- By Category with Flat Execution

**Recommendation:** Organize check source files by category (runtime, kernel, network, resources, gpu) but execute them as a flat ordered list. Category is metadata on each check, used for grouped output display.

**Rationale:** The existing doctor output is a flat list. Users benefit from visual grouping but do not need nested command structure. Categories appear in output formatting only.

**Human-readable grouped output:**
```
Runtime:
  [ OK ] docker: Docker 24.0.7
  [ OK ] kubectl: v1.29.0

Kernel:
  [ OK ] ip-forwarding
  [ OK ] bridge-nf-call-iptables
  [WARN] conntrack-max: 65536 (recommend 131072+ for multi-node)

Network:
  [ OK ] port-6443: available
  [ OK ] dns-resolution
  [SKIP] proxy-env: no HTTP_PROXY set

Resources:
  [ OK ] disk-space: 45.2 GB available
  [ OK ] memory: 16 GB available
  [WARN] inotify-watches: 8192 (recommend 524288+)
```

**JSON output preserves category for filtering:**
```json
[
  {"name": "docker", "status": "ok", "category": "runtime", "message": "Docker 24.0.7"},
  {"name": "ip-forwarding", "status": "ok", "category": "kernel"}
]
```

### Decision 5: Root/Sudo Checks -- Check Without Root, Report Fix With Sudo

**Recommendation:** Doctor checks should NEVER require root to run. They read system state (files in /proc, /sys, etc.) which is world-readable. Mitigations that require root include `sudo` in the reported fix command. Auto-mitigations in the create flow should detect if running as root and apply directly, or skip with a warning if not root.

**Design:**

```go
// checkIPForwarding reads /proc/sys/net/ipv4/ip_forward (world-readable).
// No root required to CHECK.
func (c *ipForwardingCheck) Run() []Result {
    val, err := c.readSysctl("net.ipv4.ip_forward")
    if err != nil {
        return []Result{{Name: c.Name(), Status: "warn", Category: c.Category(),
            Message: "could not read ip_forward: " + err.Error()}}
    }
    if val == "1" {
        return []Result{{Name: c.Name(), Status: "ok", Category: c.Category()}}
    }
    return []Result{{
        Name:     c.Name(),
        Status:   "fail",
        Category: c.Category(),
        Message:  "net.ipv4.ip_forward is 0\n       Fix: sudo sysctl -w net.ipv4.ip_forward=1",
    }}
}

// MitigateIPForwarding sets ip_forward=1. Requires root.
func MitigateIPForwarding() error {
    if os.Geteuid() != 0 {
        return errors.New("ip_forward mitigation requires root -- run with sudo or set manually")
    }
    return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
}
```

**Key principle:** Reading /proc/sys is always safe without root. Writing to /proc/sys requires root. Doctor reads. Create's pre-flight mitigation writes (if root) or warns (if not root).

### Decision 6: Testability -- Deps Struct Over Package-Level Vars

**Recommendation:** Use a deps struct on each check type for injectable dependencies, rather than package-level function variables. This is an improvement over the existing `currentOS`/`checkPrerequisites` pattern from the NVIDIA GPU addon.

**Rationale:** The NVIDIA GPU addon uses package-level vars:

```go
// Existing pattern (installnvidiagpu/nvidiagpu.go):
var currentOS = runtime.GOOS
var checkPrerequisites = checkHostPrerequisites
```

Tests that modify these are explicitly NOT parallel (`TestExecute_NonLinuxSkips` saves/restores `currentOS`). With 13+ checks, this approach creates 20+ package-level vars and forces many tests to be sequential. The deps struct approach scales better:

```go
type ipForwardingCheck struct {
    readSysctl func(string) (string, error)
}

func newIPForwardingCheck() Check {
    return &ipForwardingCheck{
        readSysctl: readSysctlFromProc,
    }
}
```

Tests inject dependencies at construction time, enabling full parallel execution:

```go
func TestIPForwardingCheck(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name       string
        sysctlVal  string
        sysctlErr  error
        wantStatus string
    }{
        {"enabled", "1", nil, "ok"},
        {"disabled", "0", nil, "fail"},
        {"read error", "", errors.New("no such file"), "warn"},
    }
    for _, tc := range tests {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            check := &ipForwardingCheck{
                readSysctl: func(_ string) (string, error) {
                    return tc.sysctlVal, tc.sysctlErr
                },
            }
            results := check.Run()
            if results[0].Status != tc.wantStatus {
                t.Errorf("got status %q, want %q", results[0].Status, tc.wantStatus)
            }
        })
    }
}
```

**Key advantage:** No global state mutation, no save/restore, full parallel test execution. The `newIPForwardingCheck()` constructor wires real dependencies; tests construct with fakes directly.

**For checks using exec.Command (docker, kubectl):** Use the same FakeCmd infrastructure from `testutil/fake.go`:

```go
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
```

Tests inject `lookPath` that returns/errors and `execCmd` that returns `*testutil.FakeCmd`.

### Decision 7: Where Auto-Mitigation Code Lives and Integration Points

**Recommendation:** Mitigations live in `pkg/internal/doctor/mitigations.go`. The create flow imports and calls them via a single entry point.

**Integration point in create.go:**

```go
// In pkg/cluster/internal/create/create.go, Cluster() function,
// AFTER validateProvider() and BEFORE p.Provision():

import "sigs.k8s.io/kind/pkg/internal/doctor"

// Pre-flight mitigations (best-effort, warn-and-continue).
if errs := doctor.ApplySafeMitigations(logger); len(errs) > 0 {
    for _, err := range errs {
        logger.Warnf("Pre-flight mitigation: %v", err)
    }
}
```

**Why this location:** The `Cluster()` function in `create.go` already has a pre-flight section (`validateProvider`, `fixupOptions`, `alreadyExists`). Adding `ApplySafeMitigations` fits naturally in this sequence -- it is a pre-condition check with auto-fix capability.

**Mitigation design:**

```go
// mitigations.go

// SafeMitigation represents an idempotent, non-destructive fix.
type SafeMitigation struct {
    Name      string
    NeedsFix  func() bool
    Apply     func() error
    NeedsRoot bool
}

// SafeMitigations returns the list of mitigations safe for auto-apply.
func SafeMitigations() []SafeMitigation {
    return []SafeMitigation{
        {
            Name:      "ip-forwarding",
            NeedsFix:  func() bool { v, _ := readSysctlFromProc("net.ipv4.ip_forward"); return v != "1" },
            Apply:     func() error { return writeSysctl("net.ipv4.ip_forward", "1") },
            NeedsRoot: true,
        },
        {
            Name:      "bridge-nf-call-iptables",
            NeedsFix:  func() bool { v, _ := readSysctlFromProc("net.bridge.bridge-nf-call-iptables"); return v != "1" },
            Apply:     func() error { return writeSysctl("net.bridge.bridge-nf-call-iptables", "1") },
            NeedsRoot: true,
        },
    }
}

// ApplySafeMitigations runs all safe mitigations, logging results.
// Returns errors for mitigations that failed to apply (informational, not fatal).
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

**Import direction (verified clean):**
```
pkg/cluster/internal/create/create.go
    imports -> pkg/internal/doctor/           (for ApplySafeMitigations)
    imports -> pkg/cluster/internal/create/actions/  (for addon actions)

pkg/cmd/kind/doctor/doctor.go
    imports -> pkg/internal/doctor/           (for AllChecks, Result)
```

Both consumers import from `pkg/internal/` which is the correct shared-logic layer. No circular dependencies. No CLI-to-internals imports.

## Component Boundaries

| Component | Responsibility | Location | New/Modified |
|-----------|---------------|----------|-------------|
| Check interface | Defines the contract for all checks | `pkg/internal/doctor/check.go` | **NEW** |
| Result type | Exported result struct with category | `pkg/internal/doctor/check.go` | **NEW** |
| AllChecks registry | Ordered slice of all checks | `pkg/internal/doctor/check.go` | **NEW** |
| Runtime checks | Docker/podman/nerdctl, kubectl, cgroup driver, storage driver | `pkg/internal/doctor/runtime.go` | **NEW** |
| Kernel checks | ip_forward, bridge-nf-call, conntrack, swap | `pkg/internal/doctor/kernel.go` | **NEW** |
| Network checks | Port availability, DNS, proxy env | `pkg/internal/doctor/network.go` | **NEW** |
| Resource checks | Disk, memory, CPU, inotify | `pkg/internal/doctor/resources.go` | **NEW** |
| GPU checks | NVIDIA driver, toolkit, runtime | `pkg/internal/doctor/gpu.go` | **NEW** (migrated from doctor.go) |
| Mitigations | Safe auto-fix functions + ApplySafeMitigations | `pkg/internal/doctor/mitigations.go` | **NEW** |
| Doctor command | CLI entry point, output formatting, uses Check interface | `pkg/cmd/kind/doctor/doctor.go` | **MODIFIED** |
| Create flow | Pre-flight mitigations call before provisioning | `pkg/cluster/internal/create/create.go` | **MODIFIED** |
| checkResult JSON struct | Add Category field (backward-compatible) | `pkg/cmd/kind/doctor/doctor.go` | **MODIFIED** |

## Data Flow

### Doctor Command Flow

```
User runs: kinder doctor [--output json]
    |
    v
doctor.runE()
    |
    +-- doctor.AllChecks() returns []Check
    |
    +-- for each check:
    |     +-- check.Platforms() -> platform filter (centralized in runE)
    |     +-- if skip: append Result{Status: "skip"}
    |     +-- if applicable: check.Run() -> append Results
    |
    +-- compute exit code from results
    |     (skip does NOT trigger warning exit code 2)
    |
    +-- if --output json:
    |     +-- json.NewEncoder -> stdout (with category field)
    |     +-- os.Exit(code)
    |
    +-- else (human-readable):
          +-- group by category for display
          +-- format [STATUS] name: message
          +-- os.Exit(code)
```

### Create Command Flow (with mitigations)

```
User runs: kinder create cluster
    |
    v
create.Cluster()
    |
    +-- validateProvider()                      (existing)
    +-- fixupOptions()                          (existing)
    +-- alreadyExists()                         (existing)
    +-- doctor.ApplySafeMitigations(logger)     <-- NEW
    |     +-- runtime.GOOS != "linux"? return nil
    |     +-- for each SafeMitigation:
    |           +-- NeedsFix()? -> check current state
    |           +-- NeedsRoot && !root? -> warn, skip
    |           +-- Apply() -> write sysctl / fix
    |
    +-- p.Provision()                           (existing)
    +-- [sequential actions: loadbalancer, kubeadm, CNI, ...]  (existing)
    +-- [addon waves 1 and 2]                   (existing)
```

## Patterns to Follow

### Pattern 1: Check Implementation with Deps Struct

**What:** Each check type embeds function dependencies for injectable testing.
**When:** All new check implementations.

```go
type ipForwardingCheck struct {
    readSysctl func(string) (string, error)
}

func newIPForwardingCheck() Check {
    return &ipForwardingCheck{
        readSysctl: readSysctlFromProc,
    }
}

func (c *ipForwardingCheck) Name() string        { return "ip-forwarding" }
func (c *ipForwardingCheck) Category() string     { return "kernel" }
func (c *ipForwardingCheck) Platforms() []string  { return []string{"linux"} }

func (c *ipForwardingCheck) Run() []Result {
    val, err := c.readSysctl("net.ipv4.ip_forward")
    if err != nil {
        return []Result{{Name: c.Name(), Status: "warn", Category: c.Category(),
            Message: "could not read ip_forward: " + err.Error()}}
    }
    if val == "1" {
        return []Result{{Name: c.Name(), Status: "ok", Category: c.Category()}}
    }
    return []Result{{Name: c.Name(), Status: "fail", Category: c.Category(),
        Message: "net.ipv4.ip_forward is 0\n       Fix: sudo sysctl -w net.ipv4.ip_forward=1"}}
}
```

### Pattern 2: Table-Driven Sysctl Checks (Reduce Boilerplate)

**What:** Many kernel checks follow the same pattern: read a sysctl value, compare to expected. A generic struct handles this.
**When:** ip_forward, bridge-nf-call-iptables, bridge-nf-call-ip6tables, nf_conntrack_max.

```go
type sysctlCheck struct {
    checkName  string
    key        string
    expected   string
    comparator string // "eq", "gte"
    severity   string // "fail" or "warn"
    fixCmd     string
    readSysctl func(string) (string, error)
}

func newSysctlCheck(name, key, expected, comparator, severity, fixCmd string) Check {
    return &sysctlCheck{
        checkName:  name,
        key:        key,
        expected:   expected,
        comparator: comparator,
        severity:   severity,
        fixCmd:     fixCmd,
        readSysctl: readSysctlFromProc,
    }
}

func (c *sysctlCheck) Name() string        { return c.checkName }
func (c *sysctlCheck) Category() string     { return "kernel" }
func (c *sysctlCheck) Platforms() []string  { return []string{"linux"} }
```

This reduces boilerplate: 4 kernel sysctl checks become 4 calls to `newSysctlCheck()` in the registry.

### Pattern 3: Mitigation as Paired Function

**What:** Each mitigation is a standalone function paired with a check by naming convention.
**When:** For all mitigations that the create flow should auto-apply.

The pairing is explicit in `SafeMitigations()` -- each entry references the check logic (NeedsFix) and the fix logic (Apply). This is not a formal interface relationship; it is a convention that keeps checks and mitigations co-located in the same package.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Check Registration via init()

**What:** Using `func init() { RegisterCheck(myCheck) }` in each check file.
**Why bad:** Kinder does not use init-registration anywhere. It makes check ordering non-deterministic (depends on file compilation order within a package). It makes it impossible to read which checks run by looking at one place.
**Instead:** Explicit slice in `AllChecks()` function.

### Anti-Pattern 2: Checks That Modify System State

**What:** Having a check function that "fixes" things while checking them.
**Why bad:** `kinder doctor` should be safe to run at any time, even by non-root users, even on production hosts. A check that modifies state could break things unexpectedly.
**Instead:** Checks are pure readers. Mitigations are separate functions called only from the create flow.

### Anti-Pattern 3: Global Package-Level Vars for Every Dependency

**What:** Following the NVIDIA GPU addon pattern of `var readSysctl = readSysctlFromProc` at package level for every injectable dependency.
**Why bad:** With 13+ checks, you would have 20+ package-level vars. Tests that modify these cannot run in parallel. The NVIDIA addon had a single check; this approach does not scale to many checks.
**Instead:** Deps struct on each check type. Package-level vars only for backward compatibility with existing NVIDIA code (which can be migrated later).

### Anti-Pattern 4: Putting Check Logic in pkg/cmd/kind/doctor/

**What:** Implementing diagnostic check logic directly in the CLI command package.
**Why bad:** The create flow needs access to the same check/mitigation logic. If it lives in `pkg/cmd/`, the cluster internals (`pkg/cluster/internal/create/`) would need to import from the CLI layer, violating the dependency direction. The existing codebase is strict about this: `pkg/cluster/internal/` imports from `pkg/internal/`, never from `pkg/cmd/`.
**Instead:** Shared `pkg/internal/doctor/` package importable by both the CLI and the create flow.

### Anti-Pattern 5: Adding --auto-fix Flag to Doctor

**What:** Having `kinder doctor --auto-fix` that both diagnoses and remediates.
**Why bad:** Conflates two concerns. Doctor is a diagnostic tool that should be safe and read-only. Auto-fix belongs in the create flow where the user has already consented to system changes by running `kinder create cluster`.
**Instead:** Doctor reports. Create auto-mitigates (for safe, idempotent fixes only).

## Migration Path for Existing Checks

The current `doctor.go` has inline check functions (`checkBinary`, `checkKubectl`, `checkNvidiaDriver`, etc.). These should be migrated to the new Check interface:

1. **Step 1:** Create `pkg/internal/doctor/` with Check interface, Result type, and AllChecks() registry
2. **Step 2:** Implement new checks (kernel, network, resources) in the new package
3. **Step 3:** Migrate existing checks (container runtime, kubectl, NVIDIA) to Check implementations
4. **Step 4:** Refactor `doctor.go` to use `AllChecks()` loop instead of inline calls
5. **Step 5:** Add mitigations and integrate with create flow
6. **Step 6:** Add category-grouped output formatting

**The existing `checkResult` JSON struct gains a `Category` field.** This is backward-compatible for JSON consumers (new field with `omitempty`, existing fields unchanged).

```go
type checkResult struct {
    Name     string `json:"name"`
    Status   string `json:"status"`
    Message  string `json:"message,omitempty"`
    Category string `json:"category,omitempty"` // NEW: backward-compatible
}
```

## Exit Code Semantics

The existing exit codes remain unchanged:
- **0:** All checks pass (ok or skip)
- **1:** At least one check failed
- **2:** No failures, but at least one warning

The new `"skip"` status does NOT trigger exit code 2 (it is informational, not a warning).

## Scalability Considerations

| Concern | At 19 checks (current goal) | At 50+ checks (future) |
|---------|---------------------------|----------------------|
| Execution time | Run sequentially, under 2s total | Add `--check` flag to run subset |
| Output verbosity | Grouped by category | Add `--category` filter flag |
| Test isolation | Deps struct per check, full parallel | Same approach scales |
| Package organization | Single `pkg/internal/doctor/` | Split into sub-packages by category if needed |

## Sources

- Direct codebase analysis of `pkg/cmd/kind/doctor/doctor.go` (current doctor implementation) -- HIGH confidence
- Direct codebase analysis of `pkg/cluster/internal/create/create.go` (create flow, action system, wave pattern) -- HIGH confidence
- Direct codebase analysis of `pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu.go` (existing package-level var test injection pattern) -- HIGH confidence
- Direct codebase analysis of `pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu_test.go` (test patterns: init override, save/restore, non-parallel) -- HIGH confidence
- Direct codebase analysis of `pkg/cluster/internal/create/actions/testutil/fake.go` (FakeCmd/FakeNode/FakeProvider test infrastructure) -- HIGH confidence
- Direct codebase analysis of `pkg/exec/types.go` (Cmd interface) -- HIGH confidence
- Direct codebase analysis of `pkg/cluster/internal/providers/provider.go` (Provider interface, layering) -- HIGH confidence
- Direct codebase analysis of `.planning/codebase/ARCHITECTURE.md` (layer boundaries and conventions) -- HIGH confidence
- Direct codebase analysis of `.planning/codebase/TESTING.md` (test patterns, parallel, table-driven) -- HIGH confidence
- Direct codebase analysis of `.planning/codebase/CONVENTIONS.md` (naming, imports, error handling) -- HIGH confidence
