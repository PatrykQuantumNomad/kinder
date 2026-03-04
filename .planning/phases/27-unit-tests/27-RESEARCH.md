# Phase 27: Unit Tests - Research

**Researched:** 2026-03-03
**Domain:** Go unit testing for addon action packages (stdlib only, table-driven, fake implementations)
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TEST-01 | fakeNode and fakeCmd test infrastructure created in testutil package | FakeNode must implement nodes.Node interface (exec.Cmder + String/Role/IP/SerialLogs); FakeCmd must implement exec.Cmd; both need per-call error configuration |
| TEST-02 | Unit tests for installenvoygw addon action | Execute() calls CommandContext 5 times on a single control-plane node; FakeNode records calls; table-driven: success path + each error step |
| TEST-03 | Unit tests for installlocalregistry addon action | Mixed: host-side exec.Command calls (not interceptable via FakeNode) + node CommandContext calls; tests focus on node-interaction logic via FakeNode; host-side calls require exec.DefaultCmder swap or are skipped |
| TEST-04 | Unit tests for installcertmanager addon action | Execute() calls CommandContext 5 times (apply + 3 wait loops + apply issuer); FakeNode records calls |
| TEST-05 | Unit tests for installmetricsserver addon action | Execute() calls CommandContext 2 times (apply + wait); simplest addon |
| TEST-06 | Unit tests for installdashboard addon action | Execute() calls CommandContext 4 times (apply + wait + get secret + decode); dashboard reads stdout output from FakeCmd; needs FakeCmd.SetStdout support |
</phase_requirements>

---

## Summary

Phase 27 adds unit tests for the five addon action packages that were added in Phase 25/26. The test approach is pure stdlib Go testing — no external test dependencies — consistent with the existing pattern throughout this codebase (installcorednstuning, installmetallb/subnet, waitforready all use stdlib only).

The fundamental challenge is that all five addons call `node.CommandContext(...)` which in production runs commands inside a Docker/Podman container. Tests cannot have a live container, so a `FakeNode` type must be provided that implements the `nodes.Node` interface and records calls without actually running any process. Similarly, the `FakeCmd` returned by `FakeNode` must implement `exec.Cmd` and support error injection and stdout writing (needed by installdashboard which reads a token from command stdout).

The `installlocalregistry` addon has a unique complication: it also calls `exec.Command(binaryName, ...)` (the package-level global) directly for host-side Docker operations. Per the Phase 26 decision, these are intentionally NOT wrapped in the Node interface. Tests for localregistry must either (a) only test the node-interaction paths via `FakeNode` while accepting that host-side calls will fail/succeed depending on environment, or (b) swap `exec.DefaultCmder` during the test and restore it after. Option (b) is not race-safe without a mutex or test-scoped swap. The recommended approach is option (a): focus tests on the logic that can be isolated (node patching and ConfigMap apply), and document that host-side steps require a real Docker daemon.

**Primary recommendation:** Create `pkg/cluster/internal/create/actions/testutil/` with `FakeNode` and `FakeCmd`, then write table-driven tests in each addon's package using the project's established patterns (t.Parallel, stdlib assertions, no testify).

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `testing` | stdlib (Go 1.24+) | Test framework | Project constraint: zero external test deps |
| `context` | stdlib | Context passing to FakeCmd | Needed for CommandContext |
| `strings` | stdlib | stdout content checking | Used across all existing tests |
| `io` | stdlib | FakeCmd stdout/stdin wiring | nodes.Node requires io.Writer/Reader |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/cluster/nodes` | internal | nodes.Node interface definition | FakeNode must implement this |
| `sigs.k8s.io/kind/pkg/exec` | internal | exec.Cmd interface definition | FakeCmd must implement this |
| `sigs.k8s.io/kind/pkg/cluster/constants` | internal | ControlPlaneNodeRoleValue | FakeNode.Role() returns this |
| `sigs.k8s.io/kind/pkg/internal/cli` | internal | cli.StatusForLogger | ActionContext requires *cli.Status |
| `sigs.k8s.io/kind/pkg/log` | internal | log.NoopLogger | ActionContext requires log.Logger |
| `sigs.k8s.io/kind/pkg/internal/apis/config` | internal | config.Cluster | ActionContext requires *config.Cluster |
| `sigs.k8s.io/kind/pkg/cluster/internal/providers` | internal | providers.Provider | ActionContext requires providers.Provider |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom FakeNode in testutil | testify/mock | Project explicitly forbids external test deps in main module |
| Swapping exec.DefaultCmder for localregistry host tests | Skipping host-side assertions | DefaultCmder swap is not race-safe; skipping is simpler and honest |
| Inline fake types per test file | Shared testutil package | TEST-01 explicitly requires a shared testutil; avoids duplication |

**No external packages to install** — Go stdlib only.

---

## Architecture Patterns

### Recommended Project Structure

```
pkg/cluster/internal/create/actions/
├── testutil/
│   └── fake.go          # FakeNode, FakeCmd, FakeProvider types
├── installenvoygw/
│   ├── envoygw.go       # existing
│   └── envoygw_test.go  # new: table-driven Execute() tests
├── installlocalregistry/
│   ├── localregistry.go  # existing
│   └── localregistry_test.go  # new: tests for node-interaction logic
├── installcertmanager/
│   ├── certmanager.go    # existing
│   └── certmanager_test.go  # new
├── installmetricsserver/
│   ├── metricsserver.go  # existing
│   └── metricsserver_test.go  # new
└── installdashboard/
    ├── dashboard.go      # existing
    └── dashboard_test.go # new
```

### Pattern 1: FakeNode + FakeCmd Design

**What:** FakeNode implements `nodes.Node` by returning a configurable series of FakeCmds. Each FakeCmd implements `exec.Cmd` and returns a pre-configured error on `Run()`. FakeCmd also writes pre-configured content to Stdout when `SetStdout` is called.

**When to use:** All five addon Execute() tests.

**Key interfaces to implement:**

```go
// Source: pkg/cluster/nodes/types.go
// nodes.Node interface requires exec.Cmder + String/Role/IP/SerialLogs
type Node interface {
    exec.Cmder
    String() string
    Role() (string, error)
    IP() (ipv4 string, ipv6 string, err error)
    SerialLogs(writer io.Writer) error
}

// Source: pkg/exec/types.go
// exec.Cmder requires Command + CommandContext
type Cmder interface {
    Command(string, ...string) Cmd
    CommandContext(context.Context, string, ...string) Cmd
}

// exec.Cmd requires Run + SetEnv + SetStdin + SetStdout + SetStderr
type Cmd interface {
    Run() error
    SetEnv(...string) Cmd
    SetStdin(io.Reader) Cmd
    SetStdout(io.Writer) Cmd
    SetStderr(io.Writer) Cmd
}
```

**Example FakeCmd:**
```go
// Source: pkg/exec/types.go (interface), pattern from providers/common/node.go
package testutil

import (
    "io"
    "context"

    "sigs.k8s.io/kind/pkg/exec"
)

// FakeCmd implements exec.Cmd for testing. It returns Err on Run()
// and writes Output to the stdout writer if SetStdout was called.
type FakeCmd struct {
    Err    error
    Output []byte // written to stdout on Run()
    stdout io.Writer
}

func (c *FakeCmd) Run() error {
    if c.stdout != nil && len(c.Output) > 0 {
        _, _ = c.stdout.Write(c.Output)
    }
    return c.Err
}
func (c *FakeCmd) SetEnv(...string) exec.Cmd       { return c }
func (c *FakeCmd) SetStdin(io.Reader) exec.Cmd     { return c }
func (c *FakeCmd) SetStdout(w io.Writer) exec.Cmd  { c.stdout = w; return c }
func (c *FakeCmd) SetStderr(io.Writer) exec.Cmd    { return c }
```

**Example FakeNode:**
```go
// Source: pkg/cluster/nodes/types.go + pkg/cluster/constants/constants.go
package testutil

import (
    "context"
    "io"

    "sigs.k8s.io/kind/pkg/cluster/constants"
    "sigs.k8s.io/kind/pkg/exec"
)

// FakeNode implements nodes.Node. Cmds is a queue of FakeCmd responses.
// Each CommandContext/Command call pops from the front.
type FakeNode struct {
    name string
    role string
    Cmds []*FakeCmd // queue: first call gets Cmds[0], second gets Cmds[1], etc.
    idx  int
    // Calls records (command, args...) for each invocation
    Calls [][]string
}

// NewFakeControlPlane returns a FakeNode with control-plane role.
func NewFakeControlPlane(name string, cmds []*FakeCmd) *FakeNode {
    return &FakeNode{name: name, role: constants.ControlPlaneNodeRoleValue, Cmds: cmds}
}

func (n *FakeNode) String() string { return n.name }

func (n *FakeNode) Role() (string, error) { return n.role, nil }

func (n *FakeNode) IP() (string, string, error) { return "10.0.0.1", "", nil }

func (n *FakeNode) SerialLogs(io.Writer) error { return nil }

func (n *FakeNode) Command(command string, args ...string) exec.Cmd {
    return n.nextCmd(command, args)
}

func (n *FakeNode) CommandContext(_ context.Context, command string, args ...string) exec.Cmd {
    return n.nextCmd(command, args)
}

func (n *FakeNode) nextCmd(command string, args []string) exec.Cmd {
    n.Calls = append(n.Calls, append([]string{command}, args...))
    if n.idx < len(n.Cmds) {
        cmd := n.Cmds[n.idx]
        n.idx++
        return cmd
    }
    return &FakeCmd{} // default: success, no output
}
```

### Pattern 2: FakeProvider for ActionContext

**What:** ActionContext requires a `providers.Provider`. All five addons call `ctx.Nodes()` which calls `provider.ListNodes(clusterName)`. `installlocalregistry` also calls `provider.Info()`. A FakeProvider must return pre-configured nodes.

```go
package testutil

import (
    "sigs.k8s.io/kind/pkg/cluster/nodes"
    "sigs.k8s.io/kind/pkg/cluster/internal/providers"
    "sigs.k8s.io/kind/pkg/internal/apis/config"
    "sigs.k8s.io/kind/pkg/internal/cli"
)

// FakeProvider implements providers.Provider for testing.
type FakeProvider struct {
    Nodes    []nodes.Node
    InfoResp *providers.ProviderInfo
    InfoErr  error
}

func (p *FakeProvider) ListNodes(_ string) ([]nodes.Node, error) { return p.Nodes, nil }
func (p *FakeProvider) Info() (*providers.ProviderInfo, error)   { return p.InfoResp, p.InfoErr }
// All other methods return nil/errors as needed — addon tests don't call them.
func (p *FakeProvider) Provision(*cli.Status, *config.Cluster) error { return nil }
func (p *FakeProvider) ListClusters() ([]string, error)              { return nil, nil }
func (p *FakeProvider) DeleteNodes([]nodes.Node) error               { return nil }
func (p *FakeProvider) GetAPIServerEndpoint(string) (string, error)  { return "", nil }
func (p *FakeProvider) GetAPIServerInternalEndpoint(string) (string, error) { return "", nil }
func (p *FakeProvider) CollectLogs(string, []nodes.Node) error        { return nil }
```

### Pattern 3: Building ActionContext in Tests

```go
// Source: pkg/cluster/internal/create/actions/action.go (NewActionContext)
// Source: pkg/log/noop.go (NoopLogger)
// Source: pkg/internal/cli/status.go (StatusForLogger)
func makeCtx(p providers.Provider, cfg *config.Cluster) *actions.ActionContext {
    logger := log.NoopLogger{}
    status := cli.StatusForLogger(logger)
    return actions.NewActionContext(context.Background(), logger, status, p, cfg)
}
```

### Pattern 4: Table-Driven Test Structure (Project Standard)

```go
// Source: established pattern from installcorednstuning/corefile_test.go, installmetallb/subnet_test.go
func TestExecute_EnvoyGW(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name    string
        cmds    []*testutil.FakeCmd
        wantErr bool
        errContains string
    }{
        {
            name: "all steps succeed",
            cmds: []*testutil.FakeCmd{
                {}, {}, {}, {}, {}, // 5 kubectl calls
            },
            wantErr: false,
        },
        {
            name: "apply manifest fails",
            cmds: []*testutil.FakeCmd{
                {Err: errors.New("apply failed")},
            },
            wantErr: true,
            errContains: "failed to apply Envoy Gateway manifest",
        },
        // ... one case per CommandContext call
    }
    for _, tc := range tests {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            node := testutil.NewFakeControlPlane("cp1", tc.cmds)
            provider := &testutil.FakeProvider{
                Nodes: []nodes.Node{node},
                InfoResp: &providers.ProviderInfo{},
            }
            ctx := makeCtx(provider, defaultConfig())
            a := NewAction()
            err := a.Execute(ctx)
            if tc.wantErr {
                if err == nil {
                    t.Error("expected error, got nil")
                } else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
                    t.Errorf("error = %q, want containing %q", err.Error(), tc.errContains)
                }
                return
            }
            if err != nil {
                t.Errorf("unexpected error: %v", err)
            }
        })
    }
}
```

### Pattern 5: config.Cluster Minimum Construction

```go
// Source: pkg/internal/apis/config/types.go
// ActionContext.Nodes() calls provider.ListNodes(ac.Config.Name) — Config.Name is enough
func defaultConfig() *config.Cluster {
    return &config.Cluster{Name: "test-cluster"}
}
```

### Anti-Patterns to Avoid

- **Calling `t.Fatal` after subtests are spawned:** Use `t.Error` so all subtests run.
- **Not calling t.Parallel() in subtests:** All existing tests use `tc := tc; t.Parallel()` — follow this.
- **Using exec.DefaultCmder swap for localregistry tests:** Not race-safe; test the node paths only.
- **Testing that specific kubectl flags appear in Calls:** Too brittle; test error propagation, not implementation detail of flag strings.
- **Using `t.Helper()` inside FakeNode.nextCmd:** Not needed; FakeCmd.Run() error is surfaced through action error.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Base64 decoding in dashboard test | Custom decoder | `encoding/base64.StdEncoding.EncodeToString` to build test input | Dashboard already handles decoding; test needs to feed valid b64 |
| Logger for ActionContext in tests | Custom logger | `log.NoopLogger{}` (already exists in pkg/log/noop.go) | NoopLogger is a ready-made no-op logger |
| Status for ActionContext in tests | Custom status | `cli.StatusForLogger(log.NoopLogger{})` | StatusForLogger accepts any log.Logger |
| Node list lookup | Re-implement | `nodeutils.ControlPlaneNodes(allNodes)` is already called in action code | Tests provide nodes via FakeProvider.ListNodes |

**Key insight:** The project already provides `log.NoopLogger` and `cli.StatusForLogger` — no custom logger needed for tests.

---

## Common Pitfalls

### Pitfall 1: installdashboard reads Stdout — FakeCmd must capture the writer

**What goes wrong:** `dashboard.Execute()` calls `node.CommandContext(...).SetStdout(&tokenBuf).Run()` and then reads from `tokenBuf`. If `FakeCmd.SetStdout` does not capture the writer, the token buffer stays empty and `base64.StdEncoding.DecodeString("")` returns an error (or returns empty, then `string(tokenBytes)` is empty string).

**Why it happens:** FakeCmd must store the writer passed to `SetStdout` and write `Output` to it during `Run()`.

**How to avoid:** In `FakeCmd.Run()`, if `c.stdout != nil && len(c.Output) > 0`, write `c.Output` to `c.stdout`. Test cases for dashboard success must set `FakeCmd.Output` to a valid base64-encoded token on the "get secret" command.

**Warning signs:** `"failed to decode dashboard token"` error in success test case.

### Pitfall 2: installlocalregistry host-side exec.Command calls cannot be faked via FakeNode

**What goes wrong:** Lines 89-112 in `localregistry.go` use `exec.Command(binaryName, ...)` (the package-level global), not `node.CommandContext`. These run the real Docker/Podman binary on the host. In a CI environment without Docker, they will fail.

**Why it happens:** The Phase 26 decision explicitly says "Host-side exec.Command calls in installlocalregistry intentionally unchanged (not Node interface)." Swapping `exec.DefaultCmder` is not race-safe without a global mutex.

**How to avoid:** Two approaches are viable:
1. Test only the node-interaction path by making the host-side `inspect` command succeed (requires Docker) — skip in CI with `testing.Short()`.
2. Restructure test to call an internal helper that accepts a cmder parameter — but this requires extracting logic from Execute() which may be out of scope for this phase.

**Recommendation:** Write localregistry tests that cover the node-patching paths (Step 3 and Step 4 of Execute) using FakeNode, and use `t.Skip("requires Docker daemon")` for the full Execute() path. This is honest about the limitation.

**Warning signs:** Test hangs or fails with `docker: command not found` in CI.

### Pitfall 3: ActionContext.cache not reset between table test cases

**What goes wrong:** `ActionContext` has an internal `cachedData` that caches nodes after the first `Nodes()` call. If you reuse an `ActionContext` across table test cases, the second case gets the first case's node list.

**Why it happens:** `cachedData.nodes` is set after the first `provider.ListNodes()` call.

**How to avoid:** Create a fresh `ActionContext` (and fresh FakeNode/FakeProvider) per test case — never share across subtests.

**Warning signs:** Second test case in a table behaves exactly like first despite different FakeNode configuration.

### Pitfall 4: installcertmanager loops over 3 deployments — FakeCmd queue must account for 5 total calls

**What goes wrong:** `certmanager.Execute()` calls CommandContext 5 times total:
1. `kubectl apply --server-side -f -` (apply cert-manager manifest)
2. `kubectl wait cert-manager` (first deployment)
3. `kubectl wait cert-manager-cainjector` (second deployment)
4. `kubectl wait cert-manager-webhook` (third deployment)
5. `kubectl apply -f -` (apply ClusterIssuer)

If the FakeNode queue has fewer than 5 FakeCmds, `nextCmd` returns a default `&FakeCmd{}` which succeeds silently — that's fine for the success case, but for error injection tests, the queue index matters.

**How to avoid:** Comment each FakeCmd in the test with which call it corresponds to. Provide exactly N FakeCmds for N expected calls.

### Pitfall 5: Race detector — don't share FakeNode state across goroutines

**What goes wrong:** `go test -race` will flag `FakeNode.idx` and `FakeNode.Calls` modifications if tests are run in parallel AND if any action uses goroutines internally (they do not currently, but the race detector is still required by TEST-04).

**Why it happens:** Each subtest creates its own FakeNode, but if multiple subtests share a single FakeNode (wrong), the index increment is a data race.

**How to avoid:** Each subtest creates its own `node` and `provider` instances. Since all test cases are table-driven with `tc := tc; t.Parallel()`, this is automatically safe if FakeNode instances are not shared.

---

## Code Examples

Verified patterns from official sources (pkg source code):

### Building a minimal ActionContext for tests

```go
// Source: pkg/cluster/internal/create/actions/action.go + pkg/log/noop.go + pkg/internal/cli/status.go
import (
    "context"
    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/internal/providers"
    "sigs.k8s.io/kind/pkg/internal/apis/config"
    "sigs.k8s.io/kind/pkg/internal/cli"
    "sigs.k8s.io/kind/pkg/log"
)

func newTestContext(p providers.Provider) *actions.ActionContext {
    logger := log.NoopLogger{}
    return actions.NewActionContext(
        context.Background(),
        logger,
        cli.StatusForLogger(logger),
        p,
        &config.Cluster{Name: "test"},
    )
}
```

### Dashboard test: feeding base64-encoded token output

```go
// Source: pkg/cluster/internal/create/actions/installdashboard/dashboard.go (base64 decode logic)
import "encoding/base64"

// The action decodes base64 from the "get secret" command output.
// Test must feed a valid base64 string.
token := "my-test-token"
tokenB64 := base64.StdEncoding.EncodeToString([]byte(token))

node := testutil.NewFakeControlPlane("cp1", []*testutil.FakeCmd{
    {},                                              // apply manifest: success
    {},                                              // wait deployment: success
    {Output: []byte(tokenB64)},                     // get secret: returns b64 token
})
```

### installenvoygw: 5-call sequence

```go
// Source: pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
// Execute calls CommandContext 5 times on the control-plane node:
// 1. kubectl apply --server-side -f - (install.yaml)
// 2. kubectl wait --for=condition=Complete job/eg-gateway-helm-certgen
// 3. kubectl wait --for=condition=Available deployment/envoy-gateway
// 4. kubectl apply -f - (GatewayClass)
// 5. kubectl wait --for=condition=Accepted gatewayclass/eg

node := testutil.NewFakeControlPlane("cp1", []*testutil.FakeCmd{
    {},  // step 1: apply manifest
    {},  // step 2: wait certgen job
    {},  // step 3: wait envoy-gateway deployment
    {},  // step 4: apply GatewayClass
    {},  // step 5: wait GatewayClass accepted
})
```

### installlocalregistry: FakeProvider.Info() for Rootless detection

```go
// Source: pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go (lines 73-84)
// Execute calls ctx.Provider.Info() to check Rootless+Podman.
// Test must provide InfoResp or the test will error at Info().
provider := &testutil.FakeProvider{
    Nodes: []nodes.Node{node},
    InfoResp: &providers.ProviderInfo{Rootless: false},
}
```

### Verifying FakeNode.Calls for key assertions

```go
// Optional: verify that the right kubectl command was invoked
// Keep assertions minimal — test error propagation, not flag details
if len(node.Calls) == 0 {
    t.Error("expected at least one kubectl call, got none")
}
// For error injection: verify action returns error with expected substring
if !strings.Contains(err.Error(), tc.errContains) {
    t.Errorf("err = %q, want containing %q", err.Error(), tc.errContains)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No Execute() tests for addon actions | Table-driven Execute() tests via FakeNode | Phase 27 | Full coverage of error paths without Docker |
| Per-test inline fake types | Shared testutil package | Phase 27 | Avoids 5x code duplication |
| testify/gomock for fakes | Stdlib hand-rolled FakeNode | Project constraint | Zero external deps |

**Deprecated/outdated:**
- None. This is greenfield test infrastructure.

---

## Open Questions

1. **How to handle localregistry host-side Docker calls in tests**
   - What we know: `exec.Command(binaryName, ...)` calls real Docker; Phase 26 decision says these are intentionally not wrapped in Node interface
   - What's unclear: Whether to skip the full Execute() test or restructure localregistry to accept a Cmder injector
   - Recommendation: Skip full Execute() with `t.Skip("requires Docker daemon")` and test only the node-patching logic by extracting it into a testable helper (if the planner chooses), OR accept that localregistry tests are partial and document why

2. **testutil package import path**
   - What we know: Module root is `sigs.k8s.io/kind`; actions are at `sigs.k8s.io/kind/pkg/cluster/internal/create/actions`
   - What's unclear: Whether testutil should live at `actions/testutil/` or higher up
   - Recommendation: `pkg/cluster/internal/create/actions/testutil/` — closest to consumers, no circular import risk since no production code imports testutil

3. **FakeNode.Calls granularity**
   - What we know: Some tests may want to verify command arguments (e.g., `--server-side` flag), others only care about error propagation
   - Recommendation: Record calls but only assert on them selectively; keep assertions in tests focused on error propagation, not argument detail, to reduce brittleness

---

## Sources

### Primary (HIGH confidence)
- `pkg/cluster/nodes/types.go` — nodes.Node interface definition (inspected directly)
- `pkg/exec/types.go` — exec.Cmd and exec.Cmder interface definitions (inspected directly)
- `pkg/cluster/internal/providers/provider.go` — providers.Provider interface (inspected directly)
- `pkg/log/noop.go` — NoopLogger (inspected directly)
- `pkg/internal/cli/status.go` — StatusForLogger (inspected directly)
- `pkg/cluster/internal/create/actions/action.go` — ActionContext, NewActionContext (inspected directly)
- `pkg/exec/default.go` — exec.DefaultCmder, global Command (inspected directly)
- All five addon Execute() implementations — call sequence verified by direct code inspection
- `pkg/cluster/internal/create/actions/waitforready/waitforready_test.go` — project test style (inspected directly)
- `pkg/cluster/internal/create/actions/installmetallb/subnet_test.go` — table-driven pattern (inspected directly)
- `pkg/cluster/internal/create/actions/installcorednstuning/corefile_test.go` — project test style (inspected directly)
- `pkg/cluster/internal/create/create_addon_test.go` — testLogger pattern; shows FakeLogger needed for tests in create package (inspected directly)

### Secondary (MEDIUM confidence)
- Go 1.24 testing documentation (stdlib) — table-driven test patterns with t.Parallel and tc := tc copy

### Tertiary (LOW confidence)
- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — verified by direct source inspection of all interfaces and existing tests
- Architecture: HIGH — FakeNode/FakeCmd design is straightforward interface implementation; all interface methods confirmed
- Pitfalls: HIGH — dashboard stdout issue and localregistry host-side limitation verified in source code

**Research date:** 2026-03-03
**Valid until:** 2026-04-03 (stdlib patterns are stable; Go 1.24+ behavior is stable)

---

## Appendix: Per-Addon CommandContext Call Counts

Confirmed by reading each `Execute()` implementation:

| Addon | CommandContext Calls | Notes |
|-------|---------------------|-------|
| installenvoygw | 5 | apply manifest, wait certgen, wait deployment, apply GatewayClass, wait GatewayClass |
| installlocalregistry | 2N + 1 per N nodes (node patching) + 1 ConfigMap apply | N = number of nodes; host-side calls use exec.Command global, NOT CommandContext |
| installcertmanager | 5 | apply manifest, wait cert-manager, wait cainjector, wait webhook, apply ClusterIssuer |
| installmetricsserver | 2 | apply manifest, wait deployment |
| installdashboard | 4 | apply manifest, wait deployment, get secret (with stdout), NO separate port-forward (user-facing) |

**installlocalregistry detail:**
- 4 host-side `exec.Command` calls (inspect registry, run registry, inspect networks, connect to kind network) — NOT interceptable via FakeNode
- For each node: 2 `node.CommandContext` calls (mkdir + tee hosts.toml)
- 1 `controlPlane.CommandContext` call (kubectl apply ConfigMap)
- Total node calls for 1 worker + 1 control-plane: 2*2 + 1 = 5 CommandContext calls on nodes

**installdashboard detail:**
- Step 3 (`get secret`) uses `SetStdout(&tokenBuf)` — FakeCmd must write base64 content to the writer
- The `base64.StdEncoding.DecodeString` call is in the action code (not a CommandContext call) — it processes stdout from step 3
