# Coding Conventions

**Analysis Date:** 2026-03-01

## Naming Patterns

**Files:**
- Lowercase with dashes for multi-word names: `docker-image.go`, `network-integration_test.go`
- Test files use `_test.go` suffix: `namer_test.go`, `images_test.go`
- Integration tests use `_integration_test.go` suffix: `network_integration_test.go`
- Generated files prefixed with `zz_generated.` before the type: `zz_generated.deepcopy.go`

**Functions:**
- PascalCase for exported functions: `NewLogger()`, `NewCommand()`, `ExpectError()`
- camelCase for unexported functions: `newDefaultedNode()`, `parseSnapshotter()`, `createFile()`
- Verb-first pattern for constructors: `NewString()`, `NewCommand()`, `NewLogger()`
- Descriptive names indicating functionality: `MakeNodeNamer()`, `KubeVersion()`, `WriteFile()`

**Variables:**
- camelCase for all variables: `clusterName`, `nodeNamer`, `testNetworkName`, `errCh`
- Single letters only for loop indices: `i`, `n`, `t`
- Explicit loop variable capture in range-based for loops to avoid closure issues (see `tc := tc` pattern in tests)

**Types:**
- PascalCase for all type names: `Cluster`, `Node`, `Config`, `IOStreams`
- Interface names end with "er" when representing actions: `Causer`, `StackTracer`, `testingDotT`
- Struct names use PascalCase without "Struct" suffix: `flagpole` (unexported), `buildContext` (unexported)

**Constants:**
- PascalCase for exported constants: `IPv6Family`, `WorkerRole`, `ControlPlaneRole`

**Package Names:**
- Lowercase, single word, matching directory: `cmd`, `cluster`, `config`, `docker`, `podman`
- Descriptive namespacing using directory hierarchy: `sigs.k8s.io/kind/pkg/cluster/internal/providers/docker`

## Code Style

**Formatting:**
- `gofmt -s` (standard Go formatter with simplification flag)
- Manual application via `hack/make-rules/update/gofmt.sh`
- Applied across all `.go` files in the repository
- Make target: `make gofmt`

**Linting:**
- Tool: `golangci-lint`
- Config: `hack/tools/.golangci.yml`
- Enabled linters: errcheck, gosimple, govet, ineffassign, staticcheck, typecheck, gochecknoinits, gofmt, revive, misspell, exportloopref, unparam
- Specific exclusion: unused-parameter warnings from revive (allows placeholder names in interface implementations)
- Run via: `make lint` or `hack/make-rules/verify/lint.sh`
- Timeout: 3 minutes for full linter run

**Comment Style:**
- Standard Go comment blocks with `//` for single-line comments
- No code comments unless clarifying non-obvious behavior
- Package documentation comments at top of files (example: `// This package is a stub main wrapping cmd/kind.Main()`)
- Function documentation: One-line descriptions starting with function name, end with period

**Apache License Header:**
- Every `.go` file includes Apache License 2.0 header block
- Format: `/*` block with copyright notice and license text
- Located at file start before package declaration

## Import Organization

**Order:**
1. Standard library imports (`io`, `os`, `testing`, `fmt`, `errors`)
2. Blank line
3. Third-party imports (`github.com/...`, `go.yaml.in/...`, `sigs.k8s.io/yaml`)
4. Blank line
5. Local project imports (`sigs.k8s.io/kind/...`)

**Path Aliases:**
- Rarely used in source files
- Standard imports use full paths: `sigs.k8s.io/kind/pkg/cmd`
- Aliasing only when necessary to avoid naming conflicts (example: `stderrors "errors"` to distinguish from custom error package)

**Examples:**
```go
import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/log"
	"sigs.k8s.io/kind/pkg/cmd"
)
```

## Error Handling

**Patterns:**
- Custom error wrapper at `sigs.k8s.io/kind/pkg/errors` providing stack trace support
- Functions returning `(result, error)` tuple: `KubeVersion()` returns `(string, error)`
- Wrapped errors preserve context: `Wrap(err, message)`, `Wrapf(err, format, args...)`
- Stack traces recorded at error creation point: `New()` captures stack, `NewWithoutStack()` for re-exported errors
- Error checking deferred to caller in most cases (no panic on error inside functions)
- Integration test helper: `exec.RunErrorForError()` extracts command output from errors for debugging

**Example:**
```go
func KubeVersion(n nodes.Node) (version string, err error) {
    // ... implementation that may return error
    return version, err
}

// Caller checks:
if err != nil {
    t.Fatalf("failed to decode kubeconfig: %v", err)
}
```

## Logging

**Framework:** Custom logger in `sigs.k8s.io/kind/pkg/log` and `sigs.k8s.io/kind/pkg/internal/cli`

**Patterns:**
- Logger injected as dependency into functions (pattern from `pkg/cmd/logger.go`)
- Logger writes to `os.Stderr` by default
- Smart terminal detection enables spinner visualization
- Color support checked via type assertion: `ColorEnabled(logger)` returns bool
- No structured logging; simple message-based logging via logger interface
- CLI commands receive logger: `NewCommand(logger log.Logger, streams cmd.IOStreams)`

**Setup:**
```go
func NewLogger() log.Logger {
	var writer io.Writer = os.Stderr
	if env.IsSmartTerminal(writer) {
		writer = cli.NewSpinner(writer)
	}
	return cli.NewLogger(writer, 0)
}
```

## Documentation

**Function Comments:**
- Start with function name, followed by description and period
- Example: `// NewLogger returns the standard logger used by the kind CLI`
- Should describe what function does and return values when non-obvious

**Interface Documentation:**
- Document the interface contract, not just the name
- Example: `// *testing.T methods used by assert`
- Include usage context when helpful

**Type Comments:**
- Explain purpose and usage of structs and interfaces
- Include examples where clarification needed

## Special Patterns

**Interface Embedding:**
- Custom type assertions for checking capabilities: `type maybeColorer interface { ColorEnabled() bool }`
- Used to avoid breaking public API while enabling optional features
- Pattern in `pkg/cmd/logger.go` shows dark pattern for feature detection

**Test Table-Driven Tests:**
- Standard table-driven test structure with `cases []struct` or `tests []struct`
- Fields use PascalCase: `Name`, `Value`, `Expected`, `ExpectError`, `ExpectErrors`
- Loop variable captured for parallel tests: `tc := tc` (required in Go versions before 1.22)
- Nested `t.Run()` for subtest organization
- Parallel execution via `t.Parallel()` at top of test functions and in subtests

**Unexported Package-Level Variables:**
- Used for module-private state without public accessors
- Example: Flag structs used internally within command packages: `type flagpole struct`

## Module Minimum Version

**Go Version:** 1.17 (specified in `go.mod`)
- Minimum Go language version for module compatibility
- Compiler version tracked separately in `.go-version` (currently 1.25.7)
- Feature parity with Go 1.17+ language features

## Interface Design

**Testing Interface:**
- Custom `testingDotT` interface in `pkg/internal/assert/assert.go` abstracts `*testing.T`
- Only implements methods used by assert helpers: `Errorf(format string, args ...interface{})`
- Allows mocking and testing without direct `*testing.T` dependency

**Logger Interface:**
- Minimal interface at `sigs.k8s.io/kind/pkg/log`
- Implementations injected rather than created within packages
- Supports both plain and colored output

---

*Convention analysis: 2026-03-01*
