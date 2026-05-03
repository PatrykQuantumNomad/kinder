# Coding Conventions

**Analysis Date:** 2026-05-03

## Naming Patterns

**Files:**
- Source files: lowercase with underscores for compound words (e.g., `provider.go`, `docker-image.go`)
- Test files: `{name}_test.go` (e.g., `provider_test.go`)
- Helper test files: suffixes like `testhelpers_test.go` for shared test utilities within a package

**Functions:**
- Exported (public): PascalCase (e.g., `NewProvider()`, `ListNodes()`, `Run()`)
- Unexported (private): camelCase (e.g., `defaultName()`, `newDaemonJSONCheck()`, `newFakeExecCmd()`)
- Constructor functions: `New{Type}()` pattern (e.g., `NewProvider()`, `NewLogger()`)
- Helper functions in tests: often prefixed with descriptive words (e.g., `captureOutput()`, `platformSkipMessage()`)

**Variables:**
- Package-level exported: PascalCase (e.g., `DefaultName`, `ErrNoNodeProviderDetected`)
- Package-level unexported: camelCase (e.g., `defaultCmder`)
- Local variables: camelCase (e.g., `providerOpt`, `stackErr`, `mock`)
- Unused parameters in interface implementations: use `_` (e.g., `Provision(_ *cli.Status, _ *config.Cluster) error`)

**Types:**
- Structs: PascalCase (e.g., `Provider`, `Result`, `mockProvider` for test-only types)
- Interfaces: PascalCase, typically verbs or roles (e.g., `Logger`, `Cmd`, `Check`, `Causer`)
- Type aliases for primitives: PascalCase (e.g., `Level int32` for logging verbosity)
- Test-only types: prefix with `mock` or `fake` in lowercase (e.g., `mockProvider`, `fakeCmd`, `fakeExecResult`)

**Interfaces and type assertions:**
- Compile-time assertions: `var _ InterfaceName = (*ConcreteType)(nil)` placed immediately after type definition in `*_test.go` files (e.g., line 35 in `provider_test.go`)

## Code Style

**Formatting:**
- Tool: `gofmt` (enforced)
- Line length: natural, no hard limit enforced
- Indentation: 1 tab character
- See `make gofmt` command in `Makefile`

**Linting:**
- Tool: `golangci-lint v2` with configuration in `hack/tools/.golangci.yml`
- Core linters enabled: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `gochecknoinits`, `revive`, `misspell`, `unparam`
- Key exclusions:
  - Unused parameters (allowed when implementing interfaces): `revive` rule disabled
  - Package names (`common`, `errors`, `log`, `version`): established conventions, `var-naming` exclusion
  - Test packages sharing names with source packages: allowed
- Run: `make lint` or `hack/make-rules/verify/lint.sh`

## Import Organization

**Order:**
1. Standard library imports (`fmt`, `os`, `testing`, etc.)
2. Third-party imports (`github.com/...`, `golang.org/...`)
3. Internal/project imports (everything `sigs.k8s.io/kind/...`)

**Path Aliases:**
- No explicit aliases defined in observed code
- Import paths use full module path: `sigs.k8s.io/kind/pkg/...`
- Nested packages imported with their full path (e.g., `sigs.k8s.io/kind/pkg/internal/doctor`)

**Example (from `provider.go`):**
```go
import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"sigs.k8s.io/kind/pkg/internal/kindversion"

	"sigs.k8s.io/kind/pkg/cluster/constants"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/errors"
)
```

## Error Handling

**Patterns:**
- Use `github.com/pkg/errors` for stack trace support via custom error package wrapper `sigs.k8s.io/kind/pkg/errors`
- Stack trace errors: `errors.New()`, `errors.Errorf()`, `errors.Wrap()`, `errors.Wrapf()`, `errors.WithStack()`
- Non-stack errors (public/exported): `errors.NewWithoutStack()` for user-facing errors (e.g., `ErrNoNodeProviderDetected`)
- Error checking: always check returned `err != nil` and wrap with context when propagating
- Error interfaces: implement `Causer` (`.Cause()`) and `StackTracer` (`.StackTrace()`) interfaces when needed
- Exported errors: define at package level as `var ErrXxx = ...` and document in comments

**Example (from `errors.go`):**
```go
var ErrNoNodeProviderDetected = errors.NewWithoutStack("failed to detect any supported node provider")
```

## Logging

**Framework:** `sigs.k8s.io/kind/pkg/log` — custom interface wrapping Kubernetes klog conventions

**Logger interface methods:**
- `Warn(message string)` / `Warnf(format string, args ...interface{})` — user-facing warnings
- `Error(message string)` / `Errorf(format string, args ...interface{})` — error reporting (prefer returning errors)
- `V(Level) InfoLogger` — verbosity-based logging with levels 0-3+
  - V(0): normal user messages
  - V(1): debug messages, high-level
  - V(2): more detailed
  - V(3+): trace level

**Patterns observed:**
- Logger passed as dependency to structs (e.g., `Provider` has `logger log.Logger` field)
- Default: `log.NoopLogger{}` for testing or when no output needed
- CLI creates logger via `cmd.NewLogger()` which wraps output with spinner if terminal supports it
- No direct use of `fmt.Printf` or `log.Print` in library code (use logger instead)

**Example (from `provider.go`):**
```go
type Provider struct {
	provider internalproviders.Provider
	logger   log.Logger
}
```

## Comments

**When to Comment:**
- Public API elements (types, functions, constants, variables): always include comment starting with name
- Non-obvious logic: explain "why", not just "what"
- TODO, NOTE, FIXME: documented with context and justification
- Complex algorithms or workarounds: explain the reasoning

**JSDoc/GoDoc:**
- Format: `// Name description.` for exported items (capital letter start, period end)
- Multiline comments: use `/* ... */` for longer descriptions
- Comment placement: immediately above the declared item (no blank line between)
- Example (from `provider.go`):
  ```go
  // DefaultName is the default cluster name
  const DefaultName = constants.DefaultClusterName

  // Provider is used to perform cluster operations
  type Provider struct {
  	provider internalproviders.Provider
  	logger   log.Logger
  }

  // NewProvider returns a new provider based on the supplied options
  func NewProvider(options ...ProviderOption) *Provider {
  ```

## Function Design

**Size:** 
- Small functions preferred (20-50 lines typical)
- Large functions broken into named helpers when logic exceeds ~100 lines
- Constructor functions: typically 10-30 lines including setup

**Parameters:**
- Avoid many positional parameters; use options pattern for complex configuration (e.g., `ProviderOption`)
- Interfaces preferred over concrete types for dependencies
- Context passed as first parameter when async operations possible (e.g., `CommandContext(ctx context.Context, ...)`)
- Unused parameters marked with `_` in interface implementations

**Return Values:**
- Single return type + error tuple: `(result T, err error)` — standard Go pattern
- Multiple values: when semantically distinct (e.g., `endpoint string, err error`)
- Named returns: rarely used (most functions use implicit returns)
- Nil checks: always test error first before using returned values

**Example (from `provider.go`):**
```go
func NewProvider(options ...ProviderOption) *Provider {
	p := &Provider{
		logger: log.NoopLogger{},
	}
	for _, o := range options {
		if o != nil {
			o.apply(p)
		}
	}
	return p
}
```

## Module Design

**Exports:**
- Intentional: only export what's needed for public API
- Unexported helpers grouped at end of file or in separate `internal/` subdirectories
- Interfaces defined in same package as consumers when possible

**Barrel Files:**
- Not heavily used; each package exports its own types
- `pkg/` organized by domain (cluster, errors, exec, log, cmd, etc.)
- `internal/` subdirectories used for non-public implementation details

**Package organization (from `pkg/`):**
- `pkg/cluster/` — Provider API and cluster management
- `pkg/cluster/internal/` — Private cluster implementation (create, delete, providers, etc.)
- `pkg/errors/` — Error handling utilities
- `pkg/exec/` — Command execution abstraction
- `pkg/log/` — Logging interface
- `pkg/cmd/` — CLI utilities
- `pkg/internal/` — Application-wide internal utilities (doctor, kindversion, apis, etc.)

---

*Convention analysis: 2026-05-03*
