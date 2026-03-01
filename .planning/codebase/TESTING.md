# Testing Patterns

**Analysis Date:** 2026-03-01

## Test Framework

**Runner:**
- Standard Go testing package (`testing`)
- `gotestsum` for enhanced output formatting and JUnit XML reporting
- Built from `github.com/gotestsum/gotestsum` via `hack/tools` build
- Configuration: `hack/make-rules/test.sh`

**Coverage:**
- Coverage tool: Go standard `go tool cover`
- Mode: `count` (counts execution frequency)
- Package scope: `sigs.k8s.io/kind/...` (all Kind packages)
- Coverage files generated: `{mode}.cov`, `{mode}-filtered.cov` (excludes generated code)
- HTML coverage reports: `{mode}-filtered.html`
- Generated files filtered out via `sed '/zz_generated/d'`

**Run Commands:**
```bash
make test               # Run all tests (unit + integration)
make unit              # Run unit tests only (with -short flag, excludes integration tests)
make integration       # Run integration tests (with -run '^TestIntegration')
```

**Script Location:** `hack/make-rules/test.sh`

**Test Modes:**
- `MODE=unit`: Runs with `-short` flag and `tags=nointegration`
- `MODE=integration`: Runs with `-run '^TestIntegration'` filter
- `MODE=all` (default): Runs full test suite without mode filters

**CI/CD Integration:**
- JUnit XML output to: `bin/{mode}-junit.xml`
- Artifacts uploaded to CI location: `${ARTIFACTS}/junit.xml`
- Coverage report uploaded: `${ARTIFACTS}/filtered.cov`
- Coverage HTML uploaded: `${ARTIFACTS}/filtered.html`

## Test File Organization

**Location:**
- Co-located with source files in same directory
- Example: `namer.go` paired with `namer_test.go` in `pkg/cluster/internal/providers/common/`

**Naming:**
- Unit tests: `{source}_test.go`
- Integration tests: `{source}_integration_test.go`
- Examples: `read_test.go`, `network_integration_test.go`

**Package:**
- Test file uses same package as source file (no `_test` package suffix)
- Allows testing of unexported functions and variables
- Example: `package common` in both `namer.go` and `namer_test.go`

## Test Structure

**Suite Organization:**
```go
func TestFunctionName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		clusterName string
		nodes       []string
		want        []string
	}{
		{
			name:        "Test case 1 description",
			clusterName: "value",
			nodes:       []string{"item"},
			want:        []string{"expected"},
		},
		{
			name:        "Test case 2 description",
			clusterName: "value2",
			nodes:       []string{"item2"},
			want:        []string{"expected2"},
		},
	}
	for _, tc := range cases {
		tc := tc // capture range variable (required for Go < 1.22)
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// test implementation
			result := FunctionUnderTest(tc.clusterName)
			assert.DeepEqual(t, tc.want, result)
		})
	}
}
```

**Key Patterns:**
- All test functions start with `Test` prefix
- `t.Parallel()` called at function level and within `t.Run()` subtests for concurrent execution
- Table-driven tests using `struct` slices with descriptive field names (`name`, `want`, `expected`)
- Loop variable captured (`tc := tc`) to avoid closure issues
- Subtests via `t.Run()` for logical grouping of cases
- Single test per case for focused assertions

**Assertion Patterns:**
```go
// Using built-in assertions (when appropriate)
if err != nil {
    t.Fatalf("failed to decode kubeconfig: %v", err)
}

// Using reflect.DeepEqual for complex types
if !reflect.DeepEqual(got, expected) {
    t.Errorf("Result = %v, want %v", got, expected)
}

// Using custom assert helpers from pkg/internal/assert
assert.DeepEqual(t, tc.want, names)
assert.ExpectError(t, true, err)
assert.StringEqual(t, expected, result)
```

## Custom Assertion Helpers

**Location:** `pkg/internal/assert/assert.go`

**Available Helpers:**
- `ExpectError(t testingDotT, expectError bool, err error)` - Asserts error presence/absence
- `BoolEqual(t testingDotT, expected, result bool)` - Asserts boolean equality
- `StringEqual(t testingDotT, expected, result string)` - Asserts string equality with quoted output
- `DeepEqual(t testingDotT, expected, result interface{})` - Asserts deep equality for complex types

**Interface:**
- `testingDotT` interface abstracts `*testing.T`, allowing test helper reuse
- Only requires `Errorf(format string, args ...interface{})` method
- Enables mocking and testing without direct testing package dependency

**Example Usage:**
```go
assert.DeepEqual(t, tc.want, names)
assert.ExpectError(t, true, err)
```

## Integration Tests

**Build Tags:**
- Defined with: `//go:build !nointegration` and `// +build !nointegration`
- Located at top of integration test files
- Filter condition: `-tags=nointegration` in unit test mode excludes these

**Example Location:** `pkg/cluster/internal/providers/docker/network_integration_test.go`

**Execution:**
- Run via: `make integration` or `MODE=integration make test`
- Uses filter: `-run '^TestIntegration'` to match integration test names
- Skipped during unit test runs via build tags

**Pattern:**
```go
//go:build !nointegration
// +build !nointegration

func TestIntegrationEnsureNetworkConcurrent(t *testing.T) {
    integration.MaybeSkip(t)
    // ... integration test implementation
}
```

**Helper:** `integration.MaybeSkip(t)` allows conditional skip based on environment

## Test Data & Fixtures

**Inline Fixtures:**
- TOML/YAML configuration embedded as multi-line strings in test cases
- Example: `nodeutils/util_test.go` embeds full containerd config in test (lines 25-244)
- Realistic data allows testing parser edge cases and complex configs

**Factory Functions:**
- Functions to create test objects: `newDefaultedNode()`, `newDefaultedCluster()`
- Located in test files or shared test utilities
- Used in table-driven tests to reduce repetition

**Test Struct Closure:**
- Anonymous functions create test objects: `func() Cluster { c := Cluster{}; ... return c }()`
- Allows conditional initialization and complex setup logic
- Example in `validate_test.go` lines 36-40

## Error Testing

**Expected Error Pattern:**
```go
_, err := KINDFromRawKubeadm("	", "kind", "")
assert.ExpectError(t, true, err)

// Or explicit check:
if err == nil && expectError {
    t.Errorf("Expected error but got none")
}
```

**Stack Trace Debugging:**
```go
if err != nil {
    t.Errorf("error creating network: %v", err)
    rerr := exec.RunErrorForError(err)
    if rerr != nil {
        t.Errorf("%q", rerr.Output)
    }
    t.Errorf("%+v", errors.StackTrace(err))
}
```

**Pattern:** Errors from custom `sigs.k8s.io/kind/pkg/errors` package include stack traces accessible via `errors.StackTrace()`

## Concurrency Testing

**Concurrent Test Example:**
```go
func TestIntegrationEnsureNetworkConcurrent(t *testing.T) {
    networkConcurrency := 10
    errCh := make(chan error, networkConcurrency)

    for i := 0; i < networkConcurrency; i++ {
        go func() {
            errCh <- ensureNetwork(testNetworkName)
        }()
    }

    for i := 0; i < networkConcurrency; i++ {
        if err := <-errCh; err != nil {
            t.Errorf("error: %v", err)
        }
    }
}
```

**Pattern:**
- Goroutines spawned in loop sending results to buffered channel
- Main goroutine collects results from channel
- Useful for testing race conditions and concurrent safety

## Coverage Requirements

**Target:** No enforced minimum coverage percentage (none specified in config)

**Measurement:**
- Coverage collected per test run
- Available as HTML report: `bin/{mode}-filtered.html`
- Generated files excluded from coverage calculation

**Viewing Coverage:**
```bash
make test              # Generates bin/all.cov and bin/all-filtered.html
make unit              # Generates bin/unit.cov and bin/unit-filtered.html
make integration       # Generates bin/integration.cov and bin/integration-filtered.html

# View HTML report in browser
open bin/all-filtered.html
```

## Mocking

**Approach:** Minimal mocking; prefers table-driven tests with real data

**Interface Mocking:**
- Uses Go interfaces for dependency injection
- Example: Logger interface injected to allow test observation
- Avoids heavy mocking frameworks

**Embedded Config Data:**
- Tests embed realistic configuration strings rather than mocking parsers
- `nodeutils/util_test.go` embeds actual containerd TOML config
- Allows testing against real parsing behavior

**No External Mock Library:**
- Standard library `testing` and interfaces used
- Interface assertions allow verifying behavior without mocking library
- Type assertions used for optional feature detection

## Test Execution Flow

**Local Execution:**
```bash
# Full test suite with coverage
make test

# Unit tests only
make unit

# Integration tests only
make integration

# Linting before running tests
make verify           # Runs linters and code generation checks
make lint             # Runs golangci-lint on source
```

**CI Execution (GitHub Actions):**
- Workflows in `.github/workflows/`
- Tests run on `ubuntu-24.04` in matrix strategy
- Coverage reports uploaded as artifacts
- JUnit XML output for integration with GitHub

## Test Annotation Patterns

**Parallel Markers:**
- `t.Parallel()` enables safe concurrent execution
- Called at test function level and within subtests
- Requires loop variable capture in range loops

**Build Tags for Test Variants:**
- `//go:build !nointegration` for integration tests
- `//go:build !ignore_autogenerated` for generated code tests
- Allows selective test filtering without modifying test code

---

*Testing analysis: 2026-03-01*
