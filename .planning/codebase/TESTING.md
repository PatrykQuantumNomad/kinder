# Testing Patterns

**Analysis Date:** 2026-05-03

## Test Framework

**Runner:**
- Go testing: built-in `testing` package (no external framework)
- Test discovery: standard `*_test.go` file convention
- Tool: `gotestsum` for JUnit XML output and test summary
- Config: none (uses standard Go testing flags)

**Assertion Library:**
- Minimal assertions: mostly `if` statements and `t.Error()` / `t.Errorf()`
- Some tests use `assert.DeepEqual()` from `sigs.k8s.io/kind/pkg/internal/assert` — custom assertion helper
- Standard Go `testing.T` methods: `t.Fatal()`, `t.Errorf()`, `t.Run()`, `t.Parallel()`

**Run Commands:**
```bash
make test                  # Run all unit + integration tests with coverage
make unit                  # Run only unit tests (short mode, nointegration tag)
make integration          # Run only integration tests (TestIntegration* matching)
make test-race            # Run race detector (requires CGO_ENABLED=1)
go test ./...             # Direct test invocation
```

**Coverage:**
- Mode: `count` (default)
- Package scope: `sigs.k8s.io/kind/...` (all packages)
- Output: HTML report generated to `bin/{mode}-filtered.html`
- Artifacts copied to CI upload location when running under CI (via `ARTIFACTS` env var)
- Generated files excluded from coverage: `zz_generated*` files filtered out

## Test File Organization

**Location:**
- Co-located: `*_test.go` files in the same package as code under test
- Example: `pkg/cluster/provider.go` paired with `pkg/cluster/provider_test.go`
- Shared test helpers: `testhelpers_test.go` file in the same package (e.g., `pkg/internal/doctor/testhelpers_test.go`)

**Naming:**
- Test functions: `Test{FeatureName}` or `Test{FunctionName}_{Scenario}` (PascalCase, starts with `Test`)
- Sub-tests: conventional `t.Run("scenario-name", func(t *testing.T) {...})`
- Table-driven test cases: `tests` variable with slice of anonymous structs named `tt` in loop
- Mock/fake types: `mock{Type}` or `fake{Type}` (lowercase prefix)

**Structure:**
```
pkg/cluster/
├── provider.go
├── provider_test.go              # Tests for provider.go
├── createoption.go
├── nodeutils/
│   ├── util.go
│   ├── util_test.go              # Tests for util.go
│   └── ...
└── internal/providers/
    ├── docker/
    │   ├── images.go
    │   ├── images_test.go         # Tests for images.go
    │   └── ...
```

## Test Structure

**Suite Organization:**
Tests in this codebase do NOT use external test suites (no ginkgo, no testify suites). Each test is independent using `func Test*(t *testing.T)`.

**Example single test (from `check_test.go`):**
```go
func TestAllChecks_ReturnsNonNilSlice(t *testing.T) {
	t.Parallel()
	checks := AllChecks()
	if checks == nil {
		t.Fatal("AllChecks() returned nil, expected non-nil slice")
	}
}
```

**Patterns:**

1. **Parallel execution:** Nearly all tests call `t.Parallel()` at the start (290+ tests in pkg/ use it)
   - Enables concurrent test execution for faster CI builds
   - Each test must be isolation-safe
   ```go
   func TestXxx(t *testing.T) {
   	t.Parallel()
   	// test body
   }
   ```

2. **Setup/Teardown:** Use `defer` for cleanup, global state mutations, or function patching
   ```go
   original := allChecks
   defer func() { allChecks = original }()
   
   allChecks = []Check{...}  // patch for test
   ```

3. **Assertions:** Direct `if` checks with `t.Error()` or `t.Errorf()` for non-fatal, `t.Fatal()` for early exit
   ```go
   if got != expected {
   	t.Errorf("got %v, want %v", got, expected)
   }
   ```

4. **Table-driven tests:** Standard Go pattern with loop and sub-tests
   ```go
   tests := []struct {
   	name     string
   	input    string
   	expected int
   }{
   	{"case1", "data1", 1},
   	{"case2", "data2", 2},
   }
   
   for _, tt := range tests {
   	t.Run(tt.name, func(t *testing.T) {
   		t.Parallel()
   		got := Function(tt.input)
   		if got != tt.expected {
   			t.Errorf("got %d, want %d", got, tt.expected)
   		}
   	})
   }
   ```

5. **Variable shadowing in table-driven tests:** Explicit copy to avoid closure issues
   ```go
   for _, tc := range cases {
   	tc := tc  // shadow to avoid closure bug
   	t.Run("name="+tc.name, func(t *testing.T) {
   		t.Parallel()
   		// tc is now safe to use
   	})
   }
   ```

## Mocking

**Framework:** No external mocking library. Handwritten mocks using interface implementation.

**Patterns:**

1. **Interface mocks:** Struct implementing interface for test use
   ```go
   type mockProvider struct {
   	lastListNodesName string
   }
   
   var _ internalproviders.Provider = (*mockProvider)(nil)
   
   func (m *mockProvider) ListNodes(cluster string) ([]nodes.Node, error) {
   	m.lastListNodesName = cluster
   	return nil, nil
   }
   
   func (m *mockProvider) Provision(_ *cli.Status, _ *config.Cluster) error { return nil }
   // ... other required methods as stubs
   ```

2. **Fake command execution:** `fakeCmd` and `newFakeExecCmd` for testing command-line operations
   ```go
   type fakeCmd struct {
   	output string
   	err    error
   	stdout io.Writer
   }
   
   func (f *fakeCmd) Run() error {
   	if f.stdout != nil && f.output != "" {
   		fmt.Fprint(f.stdout, f.output)
   	}
   	return f.err
   }
   
   func (f *fakeCmd) SetStdout(w io.Writer) exec.Cmd { f.stdout = w; return f }
   // ... other methods
   
   // Usage:
   execOutput := map[string]fakeExecResult{
   	"docker version": {lines: "Docker version 24.0.7\n"},
   	"podman version": {lines: "podman version 4.9.0\n"},
   }
   check.execCmd = newFakeExecCmd(execOutput)
   ```

3. **Function patching for tests:** Replace package-level functions via deferred restoration
   ```go
   lookPath := func(name string) (string, error) {
   	if name == "docker" {
   		return "/usr/bin/docker", nil
   	}
   	return "", errors.New("not found")
   }
   check := &containerRuntimeCheck{
   	lookPath: tt.lookPath,
   	execCmd:  newFakeExecCmd(tt.execOutput),
   }
   ```

4. **Compile-time verification:** Ensure mock implements interface
   ```go
   var _ exec.Cmd = &fakeCmd{}
   ```

**What to Mock:**
- External command execution (`exec.Cmd`)
- External system calls (fs, network) via custom abstractions
- Dependency injection via constructor options

**What NOT to Mock:**
- Built-in Go types (strings, slices, maps, etc.)
- Logging (use real logger or `log.NoopLogger{}`)
- Internal types within the same package

## Fixtures and Factories

**Test Data:**

Table-driven test cases define data inline:
```go
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
```

Map-based fixtures for command output:
```go
files := map[string]string{
	"/etc/docker/daemon.json": `{"init": true}`,
}

results := []Result{
	{Name: "check-a", Category: "Zebra", Status: "ok", Message: "a"},
	{Name: "check-b", Category: "Aardvark", Status: "warn", Message: "b"},
}
```

Helper functions for common test setup (from `testhelpers_test.go`):
```go
func captureOutput() *bytes.Buffer {
	return &bytes.Buffer{}
}

func newFakeExecCmd(results map[string]fakeExecResult) func(name string, args ...string) exec.Cmd {
	return func(name string, args ...string) exec.Cmd {
		// map command strings to fakeExecResult values
	}
}
```

**Location:**
- Test data: inline in test function or test file (`*_test.go`)
- Shared helpers: `testhelpers_test.go` file in same package (e.g., `pkg/internal/doctor/testhelpers_test.go`)
- No separate fixtures directory

## Coverage

**Requirements:** No explicit target enforced in CI. Coverage reports generated for visibility.

**View Coverage:**
```bash
make unit                    # Generates bin/unit-filtered.cov and bin/unit-filtered.html
make integration            # Generates bin/integration-filtered.cov and bin/integration-filtered.html
go tool cover -html=<file>  # View any coverage file
```

## Test Types

**Unit Tests:**
- Scope: Individual functions or methods in isolation
- Location: `*_test.go` files in same package
- Setup: Minimal, mostly table-driven test cases
- Dependencies mocked or injected
- Run with: `make unit` (includes `-short` flag and `nointegration` build tag)
- Examples: `pkg/cluster/provider_test.go`, `pkg/errors/errors_test.go`, `pkg/internal/doctor/check_test.go`

**Integration Tests:**
- Scope: Multi-component interaction, often with real system calls
- Location: Marked with `// +build integration` or `-run '^TestIntegration'` naming convention
- Run with: `make integration`
- Examples: Tests calling actual `os/exec` or filesystem operations

**E2E Tests:**
- Not observed: No E2E test framework found
- Kind is designed for testing Kubernetes, not for automated end-to-end tests itself
- Manual testing via `kind create cluster`, `kind delete cluster`, etc.

## Common Patterns

**Async Testing:**
- `context.Context` propagated through test setup
- Example from `exec/default.go`: `CommandContext(ctx context.Context, command string, args ...string) Cmd`
- Tests using context: rarely spawn goroutines; context used for cancellation when needed

**Error Testing:**
```go
tests := []struct {
	name     string
	input    string
	wantErr  bool
	wantMsg  string
}{
	{
		name:    "invalid input",
		input:   "bad",
		wantErr: true,
		wantMsg: "some error message",
	},
}

for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		t.Parallel()
		got, err := Function(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
		}
		if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
			t.Errorf("error message = %q, want containing %q", err.Error(), tt.wantMsg)
		}
	})
}
```

**Output Capture:**
```go
// From format_test.go
var buf bytes.Buffer
FormatHumanReadable(&buf, results)
output := buf.String()

if !strings.Contains(output, "expected text") {
	t.Error("missing expected text in output")
}
```

**File System Testing:**
- Mock file operations via dependency injection
- Example: map-based file fixture (e.g., `files := map[string]string{path: content}`)
- Use `defer func()` to restore permissions or cleanup temporary state

---

*Testing analysis: 2026-05-03*
