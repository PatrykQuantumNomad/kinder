# Coding Conventions

**Analysis Date:** 2026-04-08

## Language Split

This codebase has two primary components with different languages:
- **Go**: Core CLI tool and runtime components (`/Users/patrykattc/work/git/kinder/pkg`, `/Users/patrykattc/work/git/kinder/cmd`)
- **TypeScript/Astro**: Documentation site (`/Users/patrykattc/work/git/kinder/kinder-site`)

## Go Naming Patterns

**Files:**
- Exported functions/packages: PascalCase (e.g., `NewCommand`, `DeleteClusters`)
- Test files: `<module>_test.go` (e.g., `docker-image_test.go`, `provider_test.go`)
- Internal helper functions: lowercase with underscores (e.g., `removeDuplicates`, `sanitizeImage`)
- Package names: lowercase, single word (e.g., `load`, `delete`, `clusters`, `errors`)

**Functions:**
- Command constructors: `NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command`
- Entry points: `runE(...)` or command-specific names like `deleteClusters(...)`
- Helper functions: lowercase (e.g., `activeProviderName`, `checkIfImageReTagRequired`)
- Test functions: `func Test<FunctionName>(t *testing.T)` or `func Test_<functionName>(t *testing.T)` for private functions

**Variables:**
- Field names in structs: PascalCase (e.g., `Kubeconfig`, `All`, `Nodes`, `Name`)
- Local variables: camelCase (e.g., `imageNames`, `imageIDs`, `flags`)
- Constants in error messages: descriptive strings
- Test cases: `tc` for test case variable in table-driven tests

**Types:**
- Public interfaces: PascalCase (e.g., `Provider`, `Logger`, `StackTracer`)
- Struct names: PascalCase (e.g., `flagpole`, `mockProvider`)
- Type aliases: PascalCase (e.g., `imageTagFetcher`)

## Code Style

**Formatting:**
- Tool: `gofmt` with `-s` flag (simplify)
- Applied via: `hack/make-rules/update/gofmt.sh`
- Line length: Not explicitly restricted, but favor readability
- Indentation: Tabs (Go standard)

**Linting:**
- Tool: `golangci-lint` v2
- Config: `hack/tools/.golangci.yml`
- Key rules enabled:
  - `errcheck`: Ensure error returns are handled
  - `govet`: Go vet checker
  - `ineffassign`: Detect ineffectual assignments
  - `staticcheck`: Static analysis
  - `gochecknoinits`: Prevent init functions
  - `revive`: Go style rules
  - `misspell`: Spelling check
  - `unparam`: Find unused parameters

**Notable exclusions:**
- Unused parameters: Excluded for interface implementations (handlers may not use all params)
- Package names: Exclusions for `common`, `errors`, `log`, `version` (established convention)
- Test packages: Can use same name as package under test

## Import Organization

**Order:**
1. Standard library imports (e.g., `fmt`, `io`, `os`, `testing`)
2. Third-party imports (e.g., `github.com/spf13/cobra`, `golang.org/x/sync`)
3. Internal imports (e.g., `sigs.k8s.io/kind/pkg/...`)

**Path Aliases:**
- No explicit aliases in current code
- Uses full module path: `sigs.k8s.io/kind/...`
- Internal packages clearly marked: `sigs.k8s.io/kind/pkg/internal/...`

**Example import block:**
```go
import (
	"fmt"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/log"
)
```

## Error Handling

**Patterns:**
- Custom error wrapper: `sigs.k8s.io/kind/pkg/errors` package (wraps `github.com/pkg/errors`)
- Functions return `error` as last return value
- Errors wrapped with context: `errors.Wrap(err, "context message")` or `errors.Wrapf(err, "formatted %s", value)`
- New errors: `errors.New("message")` or `errors.Errorf("formatted %s", value)`
- Aggregate errors: `errors.NewAggregate(errlist)` for multiple errors

**Example from `deleteclusters.go`:**
```go
var errs []error
for _, cluster := range clusters {
	if err = provider.Delete(cluster, flags.Kubeconfig); err != nil {
		logger.V(0).Infof("%s\n", errors.Wrapf(err, "failed to delete cluster %q", cluster))
		errs = append(errs, errors.Wrapf(err, "failed to delete cluster %q", cluster))
		continue
	}
	success = append(success, cluster)
}
if len(errs) > 0 {
	return errors.Errorf("failed to delete %d cluster(s)", len(errs))
}
return nil
```

**Logging of errors:** Log individual errors via logger, then return aggregate error to CLI

## Logging

**Framework:** Custom `sigs.k8s.io/kind/pkg/log` package

**Patterns:**
- Logger passed as parameter to functions: `logger log.Logger`
- Verbosity levels: `logger.V(0).Infof(...)` for info, higher verbosity numbers for debug
- Spinner support: Logger wraps output in spinner for terminal interactivity
- Usage pattern:
  ```go
  logger.V(0).Infof("Deleted clusters: %q", success)
  ```

**When to log:**
- Operation start: Log what operation is beginning
- Errors (non-fatal): Log errors that are recovered from
- Completions: Log successful operations
- High verbosity: Debug information

## Comments

**When to Comment:**
- Package-level documentation: Always include `// Package <name> ...` comment before main function
- Exported items: Comments on all exported functions, types, and variables
- Complex logic: Explain non-obvious implementation choices
- Workarounds: Document any temporary fixes or known issues

**Example package comment:**
```go
// Package clusters implements the `delete` command for multiple clusters
package clusters
```

**JSDoc/GoDoc:**
- Functions: Include return type description if not obvious
- Packages: Always include package comment
- Interfaces: Document purpose and contract

## Function Design

**Size:** Keep functions under ~50 lines; split complex logic into helpers

**Parameters:**
- Logger first if needed: `logger log.Logger`
- Flags/config next: `flags *flagpole`
- Then business data: `args []string`
- Functions accepting interfaces over concrete types

**Return Values:**
- Single error as last return: `error`
- Multiple returns ordered: data first, error last: `(result string, err error)`
- Use named returns rarely; prefer explicit return statements

**Example command function signature:**
```go
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command
func runE(logger log.Logger, flags *flagpole, args []string) error
```

## Module Design

**Package Structure:**
- Each command gets its own package: `pkg/cmd/kind/delete/clusters/`
- Shared utilities in `pkg/` subdirectories: `pkg/errors`, `pkg/log`, `pkg/cluster`
- Internal implementation details in `pkg/internal/...`
- Test packages co-located with implementation: `docker-image.go` and `docker-image_test.go` in same directory

**Exports:**
- Only `NewCommand()` typically exported from command packages
- Helper functions unexported (lowercase) unless reused across packages
- Use interfaces for extensibility: `log.Logger`, `cmd.IOStreams`

**Barrel Files:** Not used; direct imports of specific packages

## TypeScript/Astro Conventions

**File naming:**
- Components: PascalCase with `.astro` extension (e.g., `Comparison.astro`, `ThemeSelect.astro`)
- Config: camelCase with `.mjs` extension (e.g., `astro.config.mjs`)
- TypeScript files: camelCase (e.g., `content.config.ts`)

**Astro components:**
- Front matter section for script logic (top of file between `---`)
- HTML markup after front matter
- Scoped styles in `<style>` tag at bottom
- CSS classes: kebab-case (e.g., `not-content`, `comparison`)

**TypeScript:**
- Strict tsconfig (inherits from `astro/tsconfigs/strict`)
- Collection definitions use Starlight loaders and schemas

---

*Convention analysis: 2026-04-08*
