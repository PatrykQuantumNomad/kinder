# Testing Patterns

**Analysis Date:** 2026-03-02

## Test Framework

**Runner:**
- Go's native `testing` package (stdlib)
- Test runner: `gotestsum` (wrapper for junit output)
- Config: None (Go standard)

**Assertion Library:**
- Custom assertion helpers: `sigs.k8s.io/kind/pkg/internal/assert`
- Standard library `testing.T` methods: `t.Errorf()`, `t.Fatalf()`, `t.Run()`
- `reflect.DeepEqual()` for struct comparisons

**Run Commands:**
```bash
make test                 # Run all tests (unit + integration)
make unit                 # Run unit tests only (fast, hermetic)
make integration          # Run integration tests only
MODE=unit hack/make-rules/test.sh     # Direct test runner
```

**Coverage:**
- Coverage enabled by default: `-coverprofile=bin/${MODE}.cov`
- Coverage mode: count (counts how many times code executed)
- Filter: Excludes generated files (`zz_generated*`)
- View coverage HTML: `go tool cover -html=bin/unit-filtered.html`

## Test File Organization

**Location:**
- Co-located: `<file>.go` and `<file>_test.go` in same directory
- Structure: `pkg/cmd/kind/version/version.go` → `pkg/cmd/kind/version/version_test.go`
- Integration tests: `<file>_integration_test.go` (separate from unit tests)

**Naming:**
- Test files: `*_test.go`
- Integration test files: `*_integration_test.go`
- Test functions: `Test<FunctionName>` (standard Go convention)
- Sub-tests: Named via `t.Run()` with descriptive names like "simple ID sort", "wrong number of networks"

**Build Tags:**
- Integration tests marked: `//go:build !nointegration` and `// +build !nointegration`
- Unit tests run with `-short` and `-tags=nointegration` flags
- Integration tests run with `-run '^TestIntegration'` pattern
- Example from `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/providers/docker/network_integration_test.go`:
  ```go
  //go:build !nointegration
  // +build !nointegration

  package docker

  func TestIntegrationEnsureNetworkConcurrent(t *testing.T) {
    integration.MaybeSkip(t)
    ...
  }
  ```

## Test Structure

**Suite Organization:**
Table-driven tests with slice of structs (Go idiomatic):
```go
func TestCheckQuiet(t *testing.T) {
  t.Parallel()  // Mark parallel-safe tests
  cases := []struct {
    Name        string
    Args        []string
    ExpectQuiet bool
  }{
    {
      Name:        "simply q",
      Args:        []string{"-q"},
      ExpectQuiet: true,
    },
    // more cases...
  }
  for _, tc := range cases {
    tc := tc  // capture variable for parallel safety
    t.Run(tc.Name, func(t *testing.T) {
      t.Parallel()
      // test logic
    })
  }
}
```

**Patterns:**
- Setup/Cleanup: Inline with `defer` cleanup (not setUp/tearDown methods)
  ```go
  dir, err := os.MkdirTemp("", "kind-testwritemerged")
  if err != nil {
    t.Fatalf("Failed to create tempdir: %d", err)
  }
  defer os.RemoveAll(dir)
  ```
- Error checking: Immediate fatals on setup errors, deferred cleanup
- Parallel execution: `t.Parallel()` on compatible tests for speed
- Nested tests: `t.Run()` for sub-tests with shared data

**Test data verification:**
- Deep equality via custom `assert.DeepEqual(t, expected, actual)`
- String comparison via `assert.StringEqual(t, expected, actual)`
- Boolean comparison via `assert.BoolEqual(t, expected, actual)`
- Error expectation via `assert.ExpectError(t, expectError bool, err error)`

## Mocking

**Framework:**
- Go interfaces used for mocking
- Function type aliases for callback injection
- Example from `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/load/docker-image/docker-image_test.go`:
  ```go
  type (
    imageTagFetcher func(nodes.Node, string) (map[string]bool, error)
  )

  func Test_checkIfImageReTagRequired(t *testing.T) {
    // ...
    exists, reTagRequired, sanitizedImage := checkIfImageReTagRequired(
      nil, // node can be nil
      tc.imageID,
      tc.imageName,
      func(n nodes.Node, s string) (map[string]bool, error) {
        return tc.imageTags.tags, tc.imageTags.err
      },
    )
  }
  ```

**Patterns:**
- Dependency injection via function parameters
- Mock functions passed in test cases as struct fields
- Closures capture test case data
- Interface-based mocking for complex dependencies

**What to Mock:**
- External system calls (Docker, kubectl, file I/O)
- Slow operations (network, process execution)
- Non-deterministic behavior (time, randomness)

**What NOT to Mock:**
- Core business logic (test real logic)
- Simple data structures
- Pure functions (no I/O dependencies)

## Fixtures and Factories

**Test Data:**
- Inline struct literal creation in test cases
- Example from `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/kubeconfig/internal/kubeconfig/merge_test.go`:
  ```go
  cases := []struct {
    Name        string
    Existing    *Config
    Kind        *Config
    Expected    *Config
    ExpectError bool
  }{
    {
      Name:     "empty existing",
      Existing: &Config{},
      Kind: &Config{
        Clusters: []NamedCluster{
          {Name: "kind-kind"},
        },
        Users: []NamedUser{
          {Name: "kind-kind"},
        },
        Contexts: []NamedContext{
          {Name: "kind-kind"},
        },
      },
      Expected: &Config{...},
      ExpectError: false,
    },
  }
  ```

**Location:**
- Test fixtures defined at top of test function as local slices
- Shared test utilities in `pkg/internal/assert` for common helpers
- Integration test helpers in `pkg/internal/integration` (e.g., `MaybeSkip()`)

## Coverage

**Requirements:**
- Unit tests: `-short` flag enabled
- Integration tests: Separate execution with `^TestIntegration` pattern
- No enforced minimum, but coverage tracked via HTML reports
- Coverage output location: `bin/${MODE}-filtered.html`

**View Coverage:**
```bash
go tool cover -html=bin/unit-filtered.html        # HTML report
cat bin/unit-filtered.cov                          # Text coverage data
```

## Test Types

**Unit Tests:**
- Scope: Individual functions in isolation
- Approach: Table-driven tests with mocked dependencies
- Speed: Fast, hermetic (no I/O), run with `-short` flag
- Examples: `version_test.go`, `namer_test.go`, `docker-image_test.go`
- Naming: `Test<FunctionName>()` without "Integration" prefix

**Integration Tests:**
- Scope: Cross-component interaction (Docker, Kubernetes APIs)
- Approach: Setup cleanup via defer, skip if prerequisites missing
- Speed: Slower, may require external tools
- Skip mechanism: `integration.MaybeSkip(t)` (called at test start)
- Naming: `TestIntegration<Component>()` - must match `^TestIntegration` regex
- Example from `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/providers/docker/network_integration_test.go`:
  ```go
  func TestIntegrationEnsureNetworkConcurrent(t *testing.T) {
    integration.MaybeSkip(t)  // Skip if !nointegration tag present

    testNetworkName := "integration-test-ensure-kind-network"

    cleanup := func() {
      ids, _ := networksWithName(testNetworkName)
      if len(ids) > 0 {
        _ = deleteNetworks(ids...)
      }
    }
    cleanup()
    defer cleanup()

    // actual test...
  }
  ```

**E2E Tests:**
- Not present in analyzed codebase
- Integration tests serve as end-to-end validation

## Common Patterns

**Async Testing:**
- Goroutines with error channels
- Example from `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/providers/docker/network_integration_test.go`:
  ```go
  errCh := make(chan error, networkConcurrency)
  for i := 0; i < networkConcurrency; i++ {
    go func() {
      errCh <- ensureNetwork(testNetworkName)
    }()
  }
  for i := 0; i < networkConcurrency; i++ {
    if err := <-errCh; err != nil {
      t.Errorf("error creating network: %v", err)
    }
  }
  ```

**Error Testing:**
```go
err := runE(logger, flags, []string{tc.Command, tc.Subcommand})
if err == nil {
  t.Errorf("Subcommand should raise an error if not called with correct params")
}
```

**File I/O Testing:**
- Create temp directories: `os.MkdirTemp("", "kind-testwritemerged")`
- Deferred cleanup: `defer os.RemoveAll(dir)`
- Write test files and verify: `os.WriteFile()`, `os.Open()`, `io.ReadAll()`
- Example from `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/kubeconfig/internal/kubeconfig/merge_test.go`:
  ```go
  func testWriteMergedNormal(t *testing.T) {
    t.Parallel()
    dir, err := os.MkdirTemp("", "kind-testwritemerged")
    if err != nil {
      t.Fatalf("Failed to create tempdir: %d", err)
    }
    defer os.RemoveAll(dir)

    // write test files and verify...
    if err := os.WriteFile(existingConfigPath, []byte(existingConfig), os.ModePerm); err != nil {
      t.Fatalf("Failed to create existing kubeconfig: %d", err)
    }
  }
  ```

---

*Testing analysis: 2026-03-02*
