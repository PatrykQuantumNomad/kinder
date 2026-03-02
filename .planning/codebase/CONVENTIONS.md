# Coding Conventions

**Analysis Date:** 2026-03-02

## Naming Patterns

**Files:**
- Lowercase with hyphens for multi-word files: `docker-image.go`, `docker-image_test.go`
- Test files: `<filename>_test.go` (unit tests), `<filename>_integration_test.go` (integration tests)
- Internal packages may use underscores: `network_test.go`

**Functions:**
- PascalCase for exported functions: `NewCommand()`, `Version()`, `DisplayVersion()`
- camelCase for unexported functions: `runE()`, `checkQuiet()`, `logError()`, `removeDuplicates()`
- Command handlers often use `runE` naming convention for cobra RunE functions
- Helper functions prefixed with action: `generateULASubnetFromName()`, `sanitizeImage()`, `truncate()`

**Variables:**
- camelCase for local and unexported: `flags`, `imageNames`, `imageIDs`, `errCh`
- PascalCase for exported package-level: `Version`, `DisplayVersion()`
- Short names acceptable in loops: `tc` (test case), `i`, `j`
- Struct field names are PascalCase (exported): `Name`, `Nodes`, `Args`, `ExpectError`

**Types:**
- PascalCase for exported types: `IOStreams`, `Logger`, `Config`, `Command`
- camelCase or descriptive names for unexported: `flagpole`, `networkInspectEntry`, `imageTagFetcher`
- Interface types named with `-er` suffix for behavior: `imageTagFetcher func(...)`

**Constants:**
- PascalCase for exported constants: `DefaultName`, `Version`
- camelCase for unexported: `versionCore`, `versionPreRelease`
- Comments document constant purpose and behavior

## Code Style

**Formatting:**
- gofmt is used (standard Go formatting)
- Run via `make gofmt` or `hack/make-rules/update/gofmt.sh`
- Consistent indentation (Go standard: tabs)

**Linting:**
- Verified via `make verify` and `hack/make-rules/verify/lint.sh`
- Part of CI pipeline

**Import Organization:**
- Standard library imports first
- Third-party imports second (e.g., `github.com/`, `go.yaml.in/`)
- Local package imports last (e.g., `sigs.k8s.io/kind/...`)
- Example from `/Users/patrykattc/work/git/kinder/cmd/kind/app/main.go`:
  ```go
  import (
    "io"
    "os"

    "github.com/spf13/pflag"

    "sigs.k8s.io/kind/pkg/cmd"
    "sigs.k8s.io/kind/pkg/cmd/kind"
    "sigs.k8s.io/kind/pkg/errors"
    "sigs.k8s.io/kind/pkg/exec"
    "sigs.k8s.io/kind/pkg/log"
  )
  ```

**Path Aliases:**
- Full import paths used consistently: `sigs.k8s.io/kind/pkg/...`
- No import aliases observed (except for standard lib conflicts like `stderrors` vs `errors`)

## Error Handling

**Patterns:**
- Errors wrapped with context using `errors.Wrap()` and `errors.Wrapf()` from `sigs.k8s.io/kind/pkg/errors`
- Stack traces captured at creation point: `errors.New()` includes stack, `errors.NewWithoutStack()` for exports
- Error context logged at call sites before returning
- Example from `/Users/patrykattc/work/git/kinder/cmd/kind/app/main.go`:
  ```go
  if err := c.Execute(); err != nil {
    logError(logger, err)
    return err
  }
  ```
- For command execution errors, check with `exec.RunErrorForError(err)` to extract output
- Distinguish between expected errors (logged) and unexpected (with stack trace)

## Logging

**Framework:** `sigs.k8s.io/kind/pkg/log` (custom logger)

**Patterns:**
- Logger interface used, not direct stdout/stderr
- Created via `cmd.NewLogger()` which returns `log.Logger`
- Verbosity controlled via `.V(level).Enabled()` check
- Example from `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/version/version.go`:
  ```go
  if logger.V(0).Enabled() {
    fmt.Fprintln(streams.Out, DisplayVersion())
  } else {
    fmt.Fprintln(streams.Out, Version())
  }
  ```
- Error logging: `logger.Errorf(format, args...)`
- Color support checked with `cmd.ColorEnabled(logger)` before adding ANSI codes
- Standard output written to `streams.Out` (io.Writer), errors to `streams.ErrOut`
- Quiet mode (-q flag) suppresses stderr and uses NoopLogger

## Comments

**When to Comment:**
- Package-level comments explain package purpose: `// Package version implements the 'version' command`
- Public functions documented with purpose statement: `// Version returns the kind CLI Semantic Version`
- Inline comments explain non-obvious logic or important notes
- Example from `/Users/patrykattc/work/git/kinder/pkg/errors/errors.go`:
  ```go
  // New returns an error with the supplied message.
  // New also records the stack trace at the point it was called.
  func New(message string) error {
  ```
- NOTE comments for build-time or special handling:
  ```go
  // NOTE: use 14 character short hash, like Kubernetes
  // NOTE: we handle the quiet flag here so we can fully silence cobra
  ```

**JSDoc/TSDoc:**
- Not used (Go project, uses standard doc comments)
- Go doc comments placed above declarations
- Starts with declaration name for discoverability: `// Version returns...`

## Function Design

**Size:**
- Small, focused functions (100+ lines considered large)
- Examples: `merge()` (40 lines), `checkQuiet()` (15 lines), `sanitizeImage()` (20 lines)

**Parameters:**
- Logger and streams often passed as first parameters: `NewCommand(logger log.Logger, streams cmd.IOStreams)`
- Flags/options grouped in struct: `flags *flagpole`
- Variadic arguments used for multiple values: `(images []string)`
- Example pattern from `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/load/docker-image/docker-image.go`:
  ```go
  func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command
  func runE(logger log.Logger, flags *flagpole, args []string) error
  func loadImage(imageTarName string, node nodes.Node) error
  ```

**Return Values:**
- Multiple return values for error handling: `(result, error)` pattern
- Named return values when helpful: `(exists, reTagRequired bool, sanitizedImage string)`
- Error always last return value if present

## Module Design

**Exports:**
- Exported functions start with capital letter (Go rule)
- Packages follow `sigs.k8s.io/kind/pkg/<feature>` structure
- Command packages under `pkg/cmd/kind/<command>/` with subcommand structure
- Internal implementation under `internal/` subdirectories (not exported)

**Barrel Files:**
- Package-level initialization in main files
- Example: `cmd/kind/app/main.go` calls `kind.NewCommand()` from `pkg/cmd/kind/`
- Internal packages use aggregation: internal kubeconfig imports internal helpers

## Copyright Headers

**All files:** Apache 2.0 license header included
```
/*
Copyright [Year] The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
...
*/
```

---

*Convention analysis: 2026-03-02*
