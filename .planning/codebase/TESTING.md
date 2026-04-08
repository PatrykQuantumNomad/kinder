# Testing Patterns

**Analysis Date:** 2026-04-08

## Test Framework

**Runner:**
- Framework: Go's built-in `testing` package
- Test command: `go test ./...`
- Config: Configured via `hack/make-rules/test.sh`

**Assertion Library:**
- Internal: `sigs.k8s.io/pkg/internal/assert` (DeepEqual pattern)
- Standard: `testing.T` methods (`Errorf`, `Run`, `Parallel`)
- No external assertion library (standard Go testing patterns)

**Run Commands:**
```bash
make test              # Run all tests (unit + integration)
make unit              # Run only unit tests (hermetic)
make integration       # Run only integration tests
make test-race         # Run race detector (CGO_ENABLED=1)
hack/make-rules/test.sh  # Direct test execution
```

**Build Tools:**
- Test runner: `gotest.tools/gotestsum` (provides JUnit XML output)
- Coverage tool: `go tool cover` (HTML reports)
- Located in: `hack/tools/` (built on demand)

## Test File Organization

**Location:**
- Co-located: Test files in same directory as implementation
- Pattern: `<module>.go` paired with `<module>_test.go`
- Example: `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/load/docker-image/docker-image.go` and `docker-image_test.go`

**Naming:**
- Test files: `*_test.go` (Go standard)
- Test functions: `Test<FunctionName>(t *testing.T)` for exported functions
- Private function tests: `Test_<functionName>(t *testing.T)` for unexported functions

**Structure:**
```
pkg/
├── cmd/
│   └── kind/
│       └── load/
│           └── docker-image/
│               ├── docker-image.go
│               └── docker-image_test.go
├── cluster/
│   ├── provider.go
│   └── provider_test.go
└── errors/
    ├── aggregate.go
    └── errors_test.go
```

## Test Structure

**Suite Organization:**
Use table-driven tests with subtests:

```go
func Test_removeDuplicates(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		want  []string
	}{
		{
			name:  "empty",
			slice: []string{},
			want:  []string{},
		},
		{
			name:  "all different",
			slice: []string{"one", "two"},
			want:  []string{"one", "two"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeDuplicates(tt.slice)
			// ... assertions
		})
	}
}
```

**Patterns:**

**Setup Pattern:**
- Variable capture in loop: Declare `tc := tc` or similar to capture loop variable
```go
for _, tc := range cases {
	tc := tc  // Capture for t.Run
	t.Run("name="+tc.name, func(t *testing.T) {
		// test code
	})
}
```

**Parallel execution:**
```go
func TestFunction(t *testing.T) {
	t.Parallel()  // Run multiple tests in parallel
	// ...
}
```

**Teardown Pattern:**
- Not explicitly used in examined tests
- Cleanup handled by test-specific local variables

**Assertion Pattern:**
- Direct comparison with error messages
- `reflect.DeepEqual()` for comparing slices/complex types
- `assert.DeepEqual(t, expected, result)` from internal assert package

```go
if got != tt.sanitizedImage {
	t.Errorf("sanitizeImage(%s) = %s, want %s", tt.image, got, tt.sanitizedImage)
}
```

## Mocking

**Framework:** Manual mocking (no external mock library)

**Patterns:**
Define mock structs that implement the interface:

```go
type mockProvider struct {
	lastListNodesName string
}

var _ internalproviders.Provider = (*mockProvider)(nil)

func (m *mockProvider) ListNodes(cluster string) ([]nodes.Node, error) {
	m.lastListNodesName = cluster
	return nil, nil
}
```

**What to Mock:**
- External dependencies (providers, loggers)
- Interfaces with multiple implementations
- Components that have side effects (file I/O, network)

**What NOT to Mock:**
- Standard library functions
- Helper functions in the same package
- Value types (use real instances)
- Error types (create real error values)

**Function callback mocking:**
Pass function types as parameters:

```go
type imageTagFetcher func(nodes.Node, string) (map[string]bool, error)

func checkIfImageReTagRequired(
	node nodes.Node,
	imageID string,
	imageName string,
	fetcher imageTagFetcher,  // Inject behavior
) (bool, bool, string) {
	// Use fetcher() to get tags
	tags, err := fetcher(node, imageName)
	// ...
}
```

In test:
```go
exists, reTagRequired, sanitizedImage := checkIfImageReTagRequired(
	nil,
	tc.imageID,
	tc.imageName,
	func(n nodes.Node, s string) (map[string]bool, error) {
		return tc.imageTags.tags, tc.imageTags.err
	},
)
```

## Fixtures and Factories

**Test Data:**
Inline in test table structs:

```go
tests := []struct {
	name  string
	input string
	want  string
}{
	{
		name:  "basic case",
		input: "ubuntu:18.04",
		want:  "docker.io/library/ubuntu:18.04",
	},
}
```

**Location:**
- No separate fixtures directory
- Data defined directly in test functions
- Reusable mock types in test package (e.g., `mockProvider`)

## Coverage

**Requirements:**
- Not enforced via CI/coverage gates
- Targets: Higher is better, no minimum enforced

**View Coverage:**
```bash
make unit  # Generates bin/unit-filtered.html
go tool cover -html=bin/unit-filtered.cov -o bin/coverage.html
```

**Coverage Configuration:**
- Mode: `count` (track how many times code is executed)
- Package: All `sigs.k8s.io/kind/...`
- Filtered: Generated code excluded (`zz_generated/` removed)

## Test Types

**Unit Tests:**
- Scope: Single function/package
- Execution: Hermetic (no external dependencies unless mocked)
- Build tag: `-short -tags=nointegration` filters these
- Example: `Test_sanitizeImage`, `Test_removeDuplicates`

**Integration Tests:**
- Scope: Multiple components working together
- Execution: May require container runtime or external tools
- Build tag: `-run '^TestIntegration'` or no `-short` flag
- Identified by: May reference real providers, actual cluster operations
- Example: Race detector tests in `test-race` target

**E2E Tests:**
- Framework: Shell scripts in `/Users/patrykattc/work/git/kinder/hack/ci/`
- Location: `hack/ci/e2e.sh`, `hack/ci/e2e-k8s.sh`
- Scope: Full CLI workflow against real Kubernetes
- Not unit test framework

## Common Patterns

**Parallel Subtests:**
```go
func TestListInternalNodes_DefaultName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		inputName    string
		expectedName string
	}{
		{inputName: "", expectedName: DefaultName},
		{inputName: "custom", expectedName: "custom"},
	}

	for _, tc := range cases {
		tc := tc  // Capture loop variable
		t.Run("name="+tc.inputName, func(t *testing.T) {
			t.Parallel()
			// Test code
		})
	}
}
```

**Error Testing:**
Test error cases in table-driven tests using error struct:

```go
type testCase struct {
	name  string
	input string
	want  string
	err   error  // Expected error
}

// In test:
if (got != nil) != (tc.err != nil) {
	t.Errorf("unexpected error: got %v, want %v", got, tc.err)
}
```

**Nil return handling:**
```go
func (m *mockProvider) ListClusters() ([]string, error) {
	return nil, nil  // Nil slice, nil error
}
```

## Test Tags and Build Constraints

**Unit Tests:**
- Build flag: `-tags=nointegration` to exclude integration tests
- Hermetic execution without external resources

**Integration Tests:**
- Require full container runtime
- Slower execution
- Optional in CI, run separately via `make integration`

**Race Detector:**
```bash
make test-race  # CGO_ENABLED=1 go test -race ./pkg/cluster/internal/create/... -count=1
```
- Requires CGO
- Checks for data races
- Limited to specific packages (cluster creation)

## CI Integration

**JUnit Output:**
Generated during test runs for CI systems:
- File: `bin/unit-junit.xml` or `bin/integration-junit.xml`
- Artifact upload location: `${ARTIFACTS}/junit.xml` (in CI environment)

**Coverage Reports:**
- HTML: `bin/unit-filtered.html`, `bin/integration-filtered.html`
- Artifact upload: `${ARTIFACTS}/filtered.html` (in CI environment)

## Known Testing Gaps

**Documented in code:**

`/Users/patrykattc/work/git/kinder/pkg/cmd/kind/delete/clusters/deleteclusters_test.go`:
- Note: The `deleteClusters` function creates concrete providers (not interfaces)
- Makes unit testing error propagation impractical
- Recommends: Refactor to accept provider interface
- Workaround: Integration tests against real container runtime

---

*Testing analysis: 2026-04-08*
