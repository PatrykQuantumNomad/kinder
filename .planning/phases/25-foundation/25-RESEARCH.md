# Phase 25: Foundation - Research

**Researched:** 2026-03-03
**Domain:** Go module maintenance, golangci-lint v2 migration, code quality cleanup
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Go version and dependencies
- Bump go.mod minimum from 1.17/1.21 to 1.23
- Update golang.org/x/sys from v0.6.0 to v0.41.0
- Remove rand.NewSource dead code that was comment-gated for pre-1.23

#### golangci-lint v2 migration
- Upgrade from v1.62.2 to v2.10.1
- Migrate existing config to v2 format
- Fix all violations so `golangci-lint run ./...` passes clean

#### Layer violation fix
- Move version package from `pkg/cmd/kind/version/` to `pkg/internal/kindversion/`
- Update all import paths
- Update linker -X flags to reference new package path
- Verify `kinder version` prints correct string after move

#### Code quality fixes
- SHA-1 → SHA-256 for subnet generation in all three providers
- os.ModePerm (0777) → 0755 for log directory permissions
- Dashboard token log level V(0) → V(1)
- Rename NoNodeProviderDetectedError → ErrNoNodeProviderDetected
- Remove redundant contains helper from subnet_test.go

### Claude's Discretion
- golangci-lint v2 config strictness — carry over existing rules or tighten
- Change sequencing and plan granularity
- Whether to fix similar naming patterns beyond FOUND-09 if encountered
- How to handle any cascading lint violations from v2 migration
- Backward compatibility for subnet hash changes (SHA-256 produces different subnets)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| FOUND-01 | go.mod minimum bumped from 1.17/1.21 to 1.23 | Go 1.23 is stable; `go 1.23` directive syntax documented |
| FOUND-02 | golang.org/x/sys updated from v0.6.0 to v0.41.0 | Standard `go get` workflow; indirect dep |
| FOUND-03 | golangci-lint upgraded from v1.62.2 to v2.10.1 with migrated config | Full v1→v2 migration guide researched; breaking changes documented below |
| FOUND-04 | Version package moved from pkg/cmd/kind/version to pkg/internal/kindversion | Import impact mapped: 2 Go files + Makefile + release script; new package name must be `kindversion` to avoid conflict with existing `pkg/internal/version` |
| FOUND-05 | rand.NewSource comment-gated dead code cleaned up after 1.23 bump | Two sites: buildcontext.go and create.go; replace with `rand.Intn`/`rand.Int32()` using global source |
| FOUND-06 | SHA-1 replaced with SHA-256 for subnet generation in all providers | Three files: docker/network.go, nerdctl/network.go, podman/network.go; only bytes 2-7 consumed |
| FOUND-07 | os.ModePerm (0777) replaced with 0755 for log directories | One site: pkg/cluster/provider.go line 257; common/logs.go and fs/fs.go ModePerm usages are for files/kubeconfig — scope limited |
| FOUND-08 | Dashboard token log level changed from V(0) to V(1) | One file: pkg/cluster/internal/create/actions/installdashboard/dashboard.go lines 106-111 |
| FOUND-09 | Error var renamed from NoNodeProviderDetectedError to ErrNoNodeProviderDetected | One definition in pkg/cluster/provider.go:102; one usage at provider.go:129 |
| FOUND-10 | Redundant contains helper removed from subnet_test.go | subnet_test.go contains custom `contains`+`containsStr`; replace call sites with `strings.Contains` |
</phase_requirements>

## Summary

Phase 25 is a maintenance phase: no new features, all changes are internal. The work splits cleanly into four categories: (1) Go ecosystem updates (go.mod version, x/sys dep, dead rand code), (2) golangci-lint v1→v2 migration, (3) a package layer violation fix, and (4) five targeted code quality fixes. Each category is well-understood and independently testable.

The most technically nuanced changes are the golangci-lint v2 migration — which involves a non-trivial config format rewrite and handling of removed/merged linters — and the version package move, which has blast radius across two Go files, the Makefile's `KIND_VERSION_PKG` variable, and the `hack/release/create.sh` `VERSION_FILE` path. SHA-256 subnet changes will produce different IPv6 ULA subnets than SHA-1, which is acceptable since clusters are transient.

The dashboard token change (FOUND-08) is subtler than it appears: the context says `V(0)→V(1)` for the token line only, but the dashboard printing block has six `V(0)` calls (lines 106-111). Only line 108 (`Token: %s`) should move to `V(1)` — the others (blank lines, "Dashboard:", port-forward instructions, "Then open:") remain at `V(0)` for user-facing output. Clarify scope during planning.

**Primary recommendation:** Sequence tasks as: FOUND-01/02 (go.mod) → FOUND-05 (dead code) → FOUND-03 (lint upgrade + fix violations) → FOUND-04 (package move) → FOUND-06/07/08/09/10 (quality fixes). Verify `go build ./...` and `golangci-lint run ./...` at each stage.

## Standard Stack

### Core Tools

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Go | 1.23 | Language runtime + module minimum | Locked decision; Go 1.20+ auto-seeds global rand source |
| golangci-lint | v2.10.1 | Static analysis runner | Locked decision; v2 released 2025-03-23, v2.10.1 released 2026-02-17 |
| golang.org/x/sys | v0.41.0 | Low-level OS primitives (indirect dep) | Required update from v0.6.0 |

### Supporting

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| crypto/sha256 | stdlib | Deterministic subnet name hashing | Replaces crypto/sha1 in three network.go files |
| math/rand (global) | Go 1.20+ | Auto-seeded global random source | Replaces explicit `rand.New(rand.NewSource(...))` after go.mod bump |

### No Alternatives Needed

All changes use stdlib or existing project dependencies. No new packages need to be imported.

**Update commands:**
```bash
# In repo root: bump go.mod
go mod edit -go=1.23
go get golang.org/x/sys@v0.41.0
go mod tidy

# In hack/tools: upgrade golangci-lint
cd hack/tools
go get github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1
go mod tidy
```

**Note on golangci-lint v2 module path:** The v2 module path is `github.com/golangci/golangci-lint/v2` (not `/golangci-lint`). The `tools.go` import and the `go build` command in `hack/make-rules/verify/lint.sh` must both be updated to `github.com/golangci/golangci-lint/v2/cmd/golangci-lint`.

## Architecture Patterns

### Recommended Change Sequence

```
Stage 1: Go ecosystem (FOUND-01, FOUND-02, FOUND-05)
  go.mod go directive → x/sys → rand dead code → go mod tidy → go build ./...

Stage 2: Lint migration (FOUND-03)
  upgrade tools go.mod → update tools.go import → rewrite .golangci.yml → fix violations → lint passes

Stage 3: Package move (FOUND-04)
  create pkg/internal/kindversion/ → copy+rename → update imports → update Makefile → update release script → verify kinder version

Stage 4: Quality fixes (FOUND-06 through FOUND-10)
  SHA-256 providers → ModePerm → log level → error rename → contains removal → final lint pass
```

### Pattern 1: golangci-lint v1 → v2 Config Migration

**What:** The `.golangci.yml` format changed substantially. The existing config at `hack/tools/.golangci.yml` uses v1 structure and must be rewritten.

**Current v1 config (verified from file):**
```yaml
run:
  timeout: 3m

linters:
  disable-all: true
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - typecheck
    - gochecknoinits
    - gofmt
    - revive
    - misspell
    - exportloopref
    - unparam

linters-settings:
  staticcheck:
    checks:
    - all

issues:
  exclude-rules:
    - text: "^unused-parameter: .*"
      linters:
        - revive
```

**Required v2 config (based on official migration guide):**
```yaml
version: "2"

run:
  timeout: 3m

linters:
  default: none
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck   # now includes gosimple and stylecheck
    - typecheck
    - gochecknoinits
    - revive
    - misspell
    - unparam
    # exportloopref REMOVED — deprecated since v1.60.2, irrelevant for Go 1.22+
    # gofmt MOVED to formatters section
  settings:
    staticcheck:
      checks:
        - all
  exclusions:
    rules:
      - text: "^unused-parameter: .*"
        linters:
          - revive

formatters:
  enable:
    - gofmt
```

**Key v1→v2 mapping (HIGH confidence — verified from official migration guide):**

| v1 | v2 | Notes |
|----|----|----|
| `linters.disable-all: true` | `linters.default: none` | Direct rename |
| `linters-settings:` | `linters.settings:` | Moved under `linters` |
| `issues.exclude-rules:` | `linters.exclusions.rules:` | Moved under `linters.exclusions` |
| `gofmt` in `linters.enable` | `formatters.enable: [gofmt]` | Formatting linters split out |
| `gosimple` in `linters.enable` | Remove (merged into `staticcheck`) | `staticcheck` now includes gosimple |
| `exportloopref` in `linters.enable` | Remove | Deprecated; replaced by `copyloopvar` (irrelevant for Go 1.22+) |
| `version: "2"` | Required at top | Signals v2 format |

**Migration tool available:** `golangci-lint migrate` auto-converts configs. However, it does not preserve comments — verify output manually. (MEDIUM confidence — from official docs; behavior may vary for edge cases.)

### Pattern 2: Version Package Move (Layer Violation Fix)

**What:** `pkg/cluster/provider.go` imports `pkg/cmd/kind/version` — a cmd-layer package used from a library-layer package. Move the version constants and string-building logic to `pkg/internal/kindversion/` to fix the dependency direction.

**Target path:** `pkg/internal/kindversion/` — named `kindversion` to avoid conflict with the existing `pkg/internal/version/` package (which is a Kubernetes version parser, unrelated).

**Files requiring import path updates:**

| File | Current Import | New Import |
|------|---------------|-----------|
| `pkg/cluster/provider.go` | `sigs.k8s.io/kind/pkg/cmd/kind/version` | `sigs.k8s.io/kind/pkg/internal/kindversion` |
| `pkg/cmd/kind/root.go` | `sigs.k8s.io/kind/pkg/cmd/kind/version` | stays (this is the cmd package registering the subcommand — keeps `NewCommand`) |
| `Makefile` line 60 | `KIND_VERSION_PKG:=sigs.k8s.io/kind/pkg/cmd/kind/version` | `KIND_VERSION_PKG:=sigs.k8s.io/kind/pkg/internal/kindversion` |
| `hack/release/create.sh` line 44 | `VERSION_FILE="./pkg/cmd/kind/version/version.go"` | `VERSION_FILE="./pkg/internal/kindversion/version.go"` |

**Critical design decision:** `pkg/cmd/kind/version/version.go` contains BOTH the version constants/logic AND the `NewCommand()` cobra subcommand. The cobra command must remain in `pkg/cmd/kind/version/` (or be moved to a cmd-layer package) since `pkg/cmd/kind/root.go` uses it.

**Correct split:**
- `pkg/internal/kindversion/` — contains: `Version()`, `DisplayVersion()`, `versionCore`, `versionPreRelease`, `gitCommit`, `gitCommitCount`, `truncate()`, and package-level vars
- `pkg/cmd/kind/version/` — retains only: `NewCommand()`, which imports from `pkg/internal/kindversion`
- `pkg/cluster/provider.go` — switches import from cmd to internal

**After move, linker flags target the internal package:**
```makefile
KIND_VERSION_PKG:=sigs.k8s.io/kind/pkg/internal/kindversion
KIND_BUILD_LD_FLAGS:=-X=$(KIND_VERSION_PKG).gitCommit=$(COMMIT) -X=$(KIND_VERSION_PKG).gitCommitCount=$(COMMIT_COUNT)
```

**Verification:** `make build && ./bin/kinder version` must print the correct version string including the git commit injected by the linker flags.

### Pattern 3: rand.NewSource Dead Code Removal

**What:** Two comment-gated dead code blocks use `rand.New(rand.NewSource(time.Now().UnixNano()))` with comments explaining this is required while go.mod minimum is 1.17. After bumping to 1.23, the global source is auto-seeded (since Go 1.20).

**Files:**
1. `pkg/build/nodeimage/buildcontext.go` line 346
2. `pkg/cluster/internal/create/create.go` line 276

**Before (both sites, same pattern):**
```go
// NOTE: explicit seeding is required while the module minimum is Go 1.17.
// Once the minimum Go version is bumped to 1.20+, this can be simplified
// to just rand.Intn() since the global source is auto-seeded.
r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
s := salutations[r.Intn(len(salutations))]
```

**After:**
```go
s := salutations[rand.Intn(len(salutations))]
```

```go
// buildcontext.go uses rand.Int32() instead of rand.Int31() (Int31 deprecated Go 1.20)
random := rand.Int32()
```

**Import cleanup:** After removing `rand.New(rand.NewSource(...))`, check if `time` import is still needed in each file. Remove unused imports.

### Pattern 4: SHA-1 → SHA-256 in generateULASubnetFromName

**What:** Three providers use `crypto/sha1` to generate a deterministic IPv6 ULA subnet from a cluster name. Replace with `crypto/sha256`.

**Files:**
- `pkg/cluster/internal/providers/docker/network.go`
- `pkg/cluster/internal/providers/nerdctl/network.go`
- `pkg/cluster/internal/providers/podman/network.go`

**Current pattern (identical in all three):**
```go
import "crypto/sha1"

func generateULASubnetFromName(name string, attempt int32) string {
    ip := make([]byte, 16)
    ip[0] = 0xfc
    ip[1] = 0x00
    h := sha1.New()
    _, _ = h.Write([]byte(name))
    _ = binary.Write(h, binary.LittleEndian, attempt)
    bs := h.Sum(nil)
    for i := 2; i < 8; i++ {
        ip[i] = bs[i]
    }
    // ...
}
```

**After:**
```go
import "crypto/sha256"

func generateULASubnetFromName(name string, attempt int32) string {
    ip := make([]byte, 16)
    ip[0] = 0xfc
    ip[1] = 0x00
    h := sha256.New()
    _, _ = h.Write([]byte(name))
    _ = binary.Write(h, binary.LittleEndian, attempt)
    bs := h.Sum(nil)
    for i := 2; i < 8; i++ {
        ip[i] = bs[i]
    }
    // ...
}
```

The function only reads bytes 2-7 of the hash output. SHA-256 produces 32 bytes (vs SHA-1's 20 bytes), so the slice `bs[2:8]` is always valid for both. The API is identical (`hash.Hash` interface), so the change is mechanical.

**Behavioral note:** SHA-256 produces different hash values than SHA-1 for the same input. Existing clusters' IPv6 ULA subnets will change if recreated. This is acceptable for a transient tool (kind clusters are recreated, not upgraded in-place).

### Pattern 5: os.ModePerm Scope

**What:** The context says `os.ModePerm (0777) → 0755 for log directory permissions`. Research reveals multiple `os.ModePerm` usages in the codebase:

| File | Line | Context | Change? |
|------|------|---------|---------|
| `pkg/cluster/provider.go` | 257 | `os.MkdirAll(dir, os.ModePerm)` — log collection dir | YES — fix to 0755 |
| `pkg/cluster/internal/providers/common/logs.go` | 12 | `os.MkdirAll(filepath.Dir(path), os.ModePerm)` — log file parent dir | YES — fix to 0755 |
| `pkg/fs/fs.go` | 63 | `os.MkdirAll(filepath.Dir(dst), os.ModePerm)` — copy dst dir | Likely YES |
| Test files | various | `os.WriteFile(..., os.ModePerm)`, `os.Mkdir(..., os.ModePerm)` | NO — test fixtures, not production |

The CONTEXT.md says "log directory permissions". The planner should decide whether `pkg/fs/fs.go` is in scope. A conservative read is: fix all production `MkdirAll` calls that create directories accessible to non-root users. Test file usages should remain as-is.

### Pattern 6: Error Variable Rename

**What:** Per Go error naming conventions (`Err`-prefix for sentinel errors), rename:
- `NoNodeProviderDetectedError` → `ErrNoNodeProviderDetected`

**Files affected:**
- `pkg/cluster/provider.go` line 100 (definition)
- `pkg/cluster/provider.go` line 129 (usage: `errors.WithStack(NoNodeProviderDetectedError)`)

**Check for external callers:** Search confirms only internal usage — no external packages in this repo reference `NoNodeProviderDetectedError`.

### Anti-Patterns to Avoid

- **Partial v2 config migration:** Do not keep v1 keys in the config alongside v2 keys. The `version: "2"` directive at top signals full v2 interpretation; mixing causes silent or confusing failures.
- **Moving the NewCommand:** Do not move `NewCommand()` out of `pkg/cmd/kind/version/` — it must stay in the cmd layer. Only the version constants/logic moves to `pkg/internal/kindversion/`.
- **Changing test file ModePerm:** Test files using `os.ModePerm` for temporary fixtures are not in scope.
- **Using `rand.Seed()`:** Do not call `rand.Seed()` as a replacement — it is deprecated since Go 1.20 and a no-op since Go 1.24.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Config migration | Manual YAML rewrite from scratch | `golangci-lint migrate` then verify | Catches keys you'd miss; handles nested renames automatically |
| SHA-256 hashing | Custom hash implementation | `crypto/sha256` (stdlib) | stdlib is audited, constant-time, no import needed |
| Import path updates | Sed scripts | `go build ./...` after change to catch missed imports | Compiler catches missed updates; grep first to enumerate, compiler confirms |

**Key insight:** The Go compiler is the authoritative tool for verifying import path changes — after editing, `go build ./...` will fail with explicit missing-import errors for every file that still references the old path.

## Common Pitfalls

### Pitfall 1: golangci-lint v2 binary module path
**What goes wrong:** `go build github.com/golangci/golangci-lint/cmd/golangci-lint` (v1 path) produces a v1 binary even after updating `go.mod` to v2.
**Why it happens:** The module path changed to include `/v2` in golangci-lint v2.
**How to avoid:** Update both `hack/tools/tools.go` and `hack/make-rules/verify/lint.sh` to use `github.com/golangci/golangci-lint/v2/cmd/golangci-lint`.
**Warning signs:** `golangci-lint version` still shows `1.x.x` after the dependency update.

### Pitfall 2: exportloopref causes build error in v2
**What goes wrong:** `golangci-lint run` errors with "linter 'exportloopref' is not found" or similar.
**Why it happens:** `exportloopref` was removed in golangci-lint v2 (deprecated since v1.60.2; irrelevant for Go 1.22+). The project's go.mod is now on 1.23.
**How to avoid:** Remove `exportloopref` from the linters enable list during config migration.
**Warning signs:** First `golangci-lint run` after v2 upgrade fails immediately at linter resolution.

### Pitfall 3: gosimple still listed as separate linter
**What goes wrong:** golangci-lint v2 errors with "linter 'gosimple' has been merged into 'staticcheck'".
**Why it happens:** In v2, `gosimple` and `stylecheck` were merged into `staticcheck`. Listing `gosimple` separately causes an error.
**How to avoid:** Remove `gosimple` from the enable list; `staticcheck` already covers it.
**Warning signs:** Error message at lint startup: "linter 'gosimple' has been merged".

### Pitfall 4: Version package split leaves broken import in pkg/cmd/kind/root.go
**What goes wrong:** After moving the entire `pkg/cmd/kind/version/` package to `pkg/internal/kindversion/`, `pkg/cmd/kind/root.go` fails to compile because it imports from the old path.
**Why it happens:** `root.go` uses the `version.NewCommand()` function — which must remain in the cmd layer.
**How to avoid:** Keep `pkg/cmd/kind/version/version.go` as a thin cmd-layer file that imports from `pkg/internal/kindversion` and exposes `NewCommand()`. Only move the version constants/logic, not the cobra command.
**Warning signs:** `go build ./...` error: "cannot import 'pkg/internal/kindversion' in 'pkg/cmd/kind/root.go'" (internal packages can only be imported by code in the parent tree).

**Important:** `pkg/internal/` packages can only be imported by code within `pkg/`. Both `pkg/cluster/provider.go` and `pkg/cmd/kind/version/` are within `pkg/`, so this is valid.

### Pitfall 5: Linker -X flag targets old package path
**What goes wrong:** `kinder version` prints an empty or default version string without the git commit.
**Why it happens:** The `-X` ldflags still point to `sigs.k8s.io/kind/pkg/cmd/kind/version.gitCommit` but the variables now live in `pkg/internal/kindversion`.
**How to avoid:** Update `KIND_VERSION_PKG` in `Makefile` to `sigs.k8s.io/kind/pkg/internal/kindversion` before testing the version command.
**Warning signs:** `kinder version` output lacks git commit info; shows default empty string.

### Pitfall 6: Dashboard log level — all 6 lines vs. only the token line
**What goes wrong:** All six `V(0)` lines in the dashboard output block are changed to `V(1)`.
**Why it happens:** The context says "Dashboard token log level V(0) → V(1)" — ambiguous about scope.
**How to avoid:** Only the token line (`ctx.Logger.V(0).Infof("  Token: %s", token)`) should move to `V(1)` per the requirement name "FOUND-08: Dashboard token log level". The surrounding info lines are user-facing and should remain `V(0)`.
**Warning signs:** `kinder create cluster --dashboard` shows no output at default verbosity.

### Pitfall 7: `go mod tidy` removes golang.org/x/sys if not verified
**What goes wrong:** After bumping go.mod, `go mod tidy` may downgrade or remove x/sys if no direct import requires v0.41.0.
**Why it happens:** x/sys is an indirect dependency — its version is controlled by what direct deps require.
**How to avoid:** Run `go get golang.org/x/sys@v0.41.0` explicitly before `go mod tidy`, then verify the version in go.sum.
**Warning signs:** go.mod shows `golang.org/x/sys` at a version other than v0.41.0 after tidy.

### Pitfall 8: rand.Int31() deprecated in Go 1.20
**What goes wrong:** Linter flags `rand.Int31()` as deprecated after the go.mod bump.
**Why it happens:** `rand.Int31()` was deprecated in Go 1.20; the replacement is `rand.Int32()`.
**How to avoid:** Replace `rand.New(rand.NewSource(...)).Int31()` with `rand.Int32()` (not `rand.Int31()`).
**Warning signs:** golangci-lint staticcheck SA1019 flag on `rand.Int31`.

## Code Examples

Verified patterns from official sources and codebase inspection:

### golangci-lint v2 Config (verified against official migration guide)
```yaml
# hack/tools/.golangci.yml
version: "2"

run:
  timeout: 3m

linters:
  default: none
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - typecheck
    - gochecknoinits
    - revive
    - misspell
    - unparam
  settings:
    staticcheck:
      checks:
        - all
  exclusions:
    rules:
      - text: "^unused-parameter: .*"
        linters:
          - revive

formatters:
  enable:
    - gofmt
```

### SHA-1 → SHA-256 (all three provider network.go files)
```go
// Source: crypto/sha256 stdlib documentation
import "crypto/sha256"  // replace "crypto/sha1"

func generateULASubnetFromName(name string, attempt int32) string {
    ip := make([]byte, 16)
    ip[0] = 0xfc
    ip[1] = 0x00
    h := sha256.New()  // replace sha1.New()
    _, _ = h.Write([]byte(name))
    _ = binary.Write(h, binary.LittleEndian, attempt)
    bs := h.Sum(nil)
    for i := 2; i < 8; i++ {
        ip[i] = bs[i]
    }
    subnet := &net.IPNet{
        IP:   net.IP(ip),
        Mask: net.CIDRMask(64, 128),
    }
    return subnet.String()
}
```

### rand.NewSource dead code cleanup (create.go)
```go
// Before:
// NOTE: explicit seeding is required while the module minimum is Go 1.17.
// Once the minimum Go version is bumped to 1.20+, this can be simplified
// to just rand.Intn() since the global source is auto-seeded.
r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
s := salutations[r.Intn(len(salutations))]

// After (global source auto-seeded since Go 1.20):
s := salutations[rand.Intn(len(salutations))]
```

### rand.NewSource dead code cleanup (buildcontext.go)
```go
// Before:
random := rand.New(rand.NewSource(time.Now().UnixNano())).Int31()

// After:
random := rand.Int32()  // not Int31(); Int31 deprecated Go 1.20
```

### Version package split — pkg/internal/kindversion/version.go
```go
// New file: pkg/internal/kindversion/version.go
// Move from: pkg/cmd/kind/version/version.go (remove NewCommand, truncate stays)
package kindversion

import "runtime"

// Version returns the kind CLI Semantic Version
func Version() string {
    return version(versionCore, versionPreRelease, gitCommit, gitCommitCount)
}

func version(core, preRelease, commit, commitCount string) string {
    // ... same logic as before ...
}

// DisplayVersion is Version() display formatted
func DisplayVersion() string {
    return "kind v" + Version() + " " + runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH
}

const versionCore = "0.3.0"
var versionPreRelease = "alpha"
var gitCommitCount = ""
var gitCommit = ""

func truncate(s string, maxLen int) string {
    if len(s) < maxLen {
        return s
    }
    return s[:maxLen]
}
```

### Version package split — pkg/cmd/kind/version/version.go (thin wrapper)
```go
// Retained cmd-layer file: pkg/cmd/kind/version/version.go
// Only NewCommand remains; version logic moved to pkg/internal/kindversion
package version

import (
    "fmt"

    "github.com/spf13/cobra"

    "sigs.k8s.io/kind/pkg/cmd"
    "sigs.k8s.io/kind/pkg/internal/kindversion"
    "sigs.k8s.io/kind/pkg/log"
)

// NewCommand returns a new cobra.Command for version
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "version",
        Short: "Prints the kind CLI version",
        Long:  "Prints the kind CLI version",
        RunE: func(cmd *cobra.Command, args []string) error {
            if logger.V(0).Enabled() {
                fmt.Fprintln(streams.Out, kindversion.DisplayVersion())
            } else {
                fmt.Fprintln(streams.Out, kindversion.Version())
            }
            return nil
        },
    }
    return cmd
}
```

### Error rename
```go
// pkg/cluster/provider.go — before:
var NoNodeProviderDetectedError = errors.NewWithoutStack("failed to detect any supported node provider")
// ...
return nil, errors.WithStack(NoNodeProviderDetectedError)

// After:
var ErrNoNodeProviderDetected = errors.NewWithoutStack("failed to detect any supported node provider")
// ...
return nil, errors.WithStack(ErrNoNodeProviderDetected)
```

### Redundant contains removal (subnet_test.go)
```go
// Before — custom helper (lines 203-215):
func contains(s, substr string) bool { ... }
func containsStr(s, substr string) bool { ... }

// Usage in test:
if !contains(err.Error(), tc.errContains) { ... }

// After — use stdlib:
import "strings"

if !strings.Contains(err.Error(), tc.errContains) { ... }
// Remove the two helper functions entirely
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `rand.New(rand.NewSource(time.Now().UnixNano()))` | `rand.Intn()` / `rand.Int32()` directly | Go 1.20 (2023) | Global source auto-seeded; explicit seeding is boilerplate |
| `rand.Seed()` | No-op | Go 1.20 deprecated; Go 1.24 made no-op | Do not call |
| `crypto/sha1` for non-cryptographic use | `crypto/sha256` | golangci-lint G401 | SHA-1 flagged as weak; SHA-256 is the minimum |
| `golangci-lint` v1.x config format | v2.x with `version: "2"` at top | golangci-lint v2 (2025-03-23) | Config format restructured; linters split from formatters |
| `linters.disable-all: true` | `linters.default: none` | golangci-lint v2 | v1 option removed |
| `gosimple` as standalone linter | Merged into `staticcheck` | golangci-lint v2 | Remove from enable list |
| `exportloopref` | Removed (irrelevant for Go 1.22+) | golangci-lint v1.60.2 deprecated, removed v2 | Remove from enable list |
| Error naming: `XxxError` | `ErrXxx` (Go stdlib convention) | Go style guide | `Err`-prefix is the idiomatic Go convention for sentinel errors |

**Deprecated/outdated items in this codebase:**
- `rand.NewSource` explicit seeding: comment-gated dead code waiting for this go.mod bump
- `crypto/sha1`: flagged by golangci-lint G401 (weak cryptographic hash)
- `os.ModePerm`: overly permissive for directories; 0755 is appropriate
- `NoNodeProviderDetectedError`: violates Go error naming convention

## Open Questions

1. **Dashboard log level scope (FOUND-08)**
   - What we know: Context says "Dashboard token log level V(0) → V(1)". The dashboard block has 6 `V(0)` calls.
   - What's unclear: Does "Dashboard token log level" mean only line 108 (`Token: %s`), or all 6 lines in the block?
   - Recommendation: Change only line 108 (the literal token value). Lines 106-107 and 109-111 (blank lines, labels, instructions) are user-facing info and belong at V(0). Document this interpretation in the task.

2. **os.ModePerm scope (FOUND-07)**
   - What we know: `provider.go:257` is clearly the log directory. `common/logs.go:12` also creates log-related directories. `fs/fs.go:63` creates arbitrary destination dirs.
   - What's unclear: Does FOUND-07 cover only `provider.go` or also `common/logs.go` and `fs/fs.go`?
   - Recommendation: Fix both `provider.go` and `common/logs.go` (both are log infrastructure). Leave `fs/fs.go` as-is or evaluate separately — it creates user-specified copy destinations where permissions may be intentionally permissive.

3. **hack/release/create.sh update (FOUND-04)**
   - What we know: `create.sh` hardcodes `VERSION_FILE="./pkg/cmd/kind/version/version.go"` for sed-based version bumping.
   - What's unclear: Should this script be updated as part of FOUND-04 or treated as a separate maintenance item?
   - Recommendation: Update the `VERSION_FILE` path in `create.sh` as part of FOUND-04 since it directly references the file being moved. The file's new location is `./pkg/internal/kindversion/version.go`.

4. **Similar naming patterns beyond FOUND-09**
   - What we know: Claude's Discretion covers whether to fix similar `XxxError` naming patterns beyond FOUND-09.
   - What's unclear: Are there other `XxxError` sentinel error vars in the codebase?
   - Recommendation: Run `grep -rn "var.*Error " pkg/ --include="*.go"` as part of FOUND-09 task to enumerate. If found, fix them in the same task for consistency; if none, note absence.

## Sources

### Primary (HIGH confidence)
- Official golangci-lint migration guide: https://golangci-lint.run/docs/product/migration-guide/ — config format changes, v1→v2 key mapping
- golangci-lint v2.10.1 release: https://github.com/golangci/golangci-lint/releases/tag/v2.10.1 — confirmed release date 2026-02-17
- Go 1.20 math/rand auto-seeding: https://pkg.go.dev/math/rand — Seed deprecated Go 1.20, global source auto-seeded
- Codebase inspection (direct file reads) — all affected files enumerated above

### Secondary (MEDIUM confidence)
- golangci-lint v2 module path `github.com/golangci/golangci-lint/v2` — verified via pkg.go.dev listing and search results
- `exportloopref` removed in v2: https://github.com/golangci/golangci-lint/issues/5052 — confirmed deprecated v1.60.2, removed v2
- `gosimple`/`staticcheck` merge: https://github.com/golangci/golangci-lint/pull/5487 — confirmed merge in v2

### Tertiary (LOW confidence)
- `rand.Int31()` deprecation in Go 1.20 — from search results, not directly verified against Go 1.20 release notes; mark for validation during implementation

## Metadata

**Confidence breakdown:**
- Go version bump (FOUND-01/02/05): HIGH — standard Go workflow, comment-gated code clearly identified in codebase
- golangci-lint v2 migration (FOUND-03): HIGH for config format changes (verified from official docs); MEDIUM for which cascading violations the v2 upgrade surfaces (unknown until runtime)
- Version package move (FOUND-04): HIGH — all affected files enumerated, import rules understood
- Code quality fixes (FOUND-06 to FOUND-10): HIGH — all sites identified in codebase, changes are mechanical

**Research date:** 2026-03-03
**Valid until:** 2026-04-03 (golangci-lint ecosystem stable; Go stdlib stable)
