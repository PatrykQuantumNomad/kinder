# Pitfalls Research

**Domain:** Go CLI tool — adding code quality improvements, architecture fixes, unit tests, and new features to an existing kind fork (~28K LOC)
**Researched:** 2026-03-03
**Confidence:** HIGH (sourced from direct codebase analysis, official Go documentation, verified community issues, cert-manager official docs)

---

## Critical Pitfalls

### Pitfall 1: go.mod Minimum Version Bump — Toolchain Directive Breaks CI

**What goes wrong:**
Bumping the `go` directive in `go.mod` from `1.17` to `1.21+` causes `go mod tidy` (on Go 1.21+) to automatically inject a `toolchain` directive. Any CI environment or developer machine still running Go 1.20 or earlier will then fail with `unknown directive: toolchain` because older toolchains do not recognize it. The `toolchain` directive also silently promotes which Go binary is downloaded when GOTOOLCHAIN=auto — the team may not realize the build environment has been upgraded until a version-specific behavior change surfaces.

The go.mod currently reads `go 1.21.0` with `toolchain go1.26.0`. If these are changed separately (e.g., a contributor bumps `go` but not `toolchain`, or vice versa), the tool downloads a mismatched toolchain.

Additionally, the comment in `create.go:276` says:
```go
// NOTE: explicit seeding is required while the module minimum is Go 1.17.
// Once the minimum Go version is bumped to 1.20+, this can be simplified
// to just rand.Intn() since the global source is auto-seeded.
```
This means the `rand.New(rand.NewSource(...))` call in `logSalutation()` should be removed after a go.mod bump. Failing to do so leaves dead code and a misleading comment that future readers will trust.

**Why it happens:**
The go.mod already has `go 1.21.0` and `toolchain go1.26.0`, which is correct. The pitfall arises when a code quality pass bumps `go` without updating `toolchain`, or bumps both but leaves comment-gated code that referenced the old minimum.

**How to avoid:**
- Always bump `go` and `toolchain` together; document the effective minimum Go version required for contributors in the README
- After bumping, run `grep -rn "1.17\|1.20\|rand.NewSource\|rand\.New\|time\.Now.*UnixNano" pkg/` to find comment-gated code that should be cleaned up
- Run `go mod tidy` with the new toolchain and verify the resulting go.mod/go.sum are checked in together
- Add `go version` to the CI check matrix to catch environment mismatches early

**Warning signs:**
- CI passes on developers' machines but fails on others with "unknown directive: toolchain"
- `go mod tidy` silently changes `toolchain` to a different version after each run on different machines
- Comments in code referencing `1.17`, `1.20` after the version bump

**Phase to address:** Phase 1 (go.mod and dependency updates) — do the version bump first, then clean up comment-gated code in the same commit

---

### Pitfall 2: Dependency Update Cascade — Transitive Deps Expand and Break Tests

**What goes wrong:**
Running `go get -u ./...` after a go.mod version bump updates direct dependencies, but Go 1.17+ lists ALL transitive (indirect) dependencies explicitly in go.mod. Updating one direct dependency (e.g., `github.com/spf13/cobra`) can pull in a new major version of `golang.org/x/sys`, which the providers use for OS-level operations. The update may break provider tests on specific platforms (e.g., `golang.org/x/sys` behavior differs between Linux and macOS at a minor version boundary).

Additionally, the `go.sum` file after a full `go get -u` may contain checksums for packages that are no longer actually imported, causing `go mod tidy` to remove them, which then causes `go get -u` to re-add them — creating an unstable loop.

The kinder codebase depends on `github.com/pkg/errors`, which is in maintenance mode. An update pass may surface deprecation warnings or removal of an API used in `pkg/errors/errors.go`.

**Why it happens:**
Developers run `go get -u ./...` intending to update direct dependencies but inadvertently trigger transitive cascades. The kinder codebase has 10 direct deps and ~15 indirect — manageable, but each indirect dep pin prevents cascade at the cost of staleness.

**How to avoid:**
- Update dependencies one at a time, not with `go get -u ./...`; start with direct dependencies, verify tests pass, commit, then move to the next
- Run `go test ./...` after each individual update before moving to the next
- Keep `github.com/pkg/errors` pinned and note in code that migration to `fmt.Errorf` wrapping is a separate task
- Check `golang.org/x/sys` version explicitly — it is used by the provider layer for OS calls; verify the new version's changelog

**Warning signs:**
- `go mod tidy` changes go.sum on a machine that didn't run `go get -u` — indicates stale sums
- Provider integration tests fail on Linux but pass on macOS (or vice versa) after a `golang.org/x/sys` update
- `go build ./...` errors about `undefined: errors.StackTrace` after a `pkg/errors` update

**Phase to address:** Phase 1 (dependency updates) — update incrementally, not bulk; run tests after each change

---

### Pitfall 3: context.Context Retrofitting — Adding ctx to Existing Interfaces Breaks Implementations

**What goes wrong:**
The `Action` interface is:
```go
type Action interface {
    Execute(ctx *ActionContext) error
}
```
The `ActionContext` struct does not embed `context.Context`. Adding `context.Context` to `Execute` or to `ActionContext` is a breaking change: every addon action file must be updated simultaneously, and any external implementations of the `Action` interface (if any exist) break.

The Go idiom is to add context via a new parallel function or to embed it in the existing context struct. Embedding a `context.Context` inside `ActionContext` appears natural but violates the [official Go guidance](https://go.dev/blog/context-and-structs): "Do not store Contexts inside a struct type; instead, pass a Context explicitly to each function that needs it."

The anti-pattern:
```go
// WRONG: context stored in struct
type ActionContext struct {
    Ctx      context.Context  // bad
    Logger   log.Logger
    ...
}
```

The correct pattern:
```go
// RIGHT: context passed as first function parameter
func (a *action) Execute(ctx context.Context, ac *ActionContext) error { ... }
```
But this changes the `Action` interface and breaks every existing addon.

**Why it happens:**
The `ActionContext` struct name creates the temptation to embed a `context.Context` there. Developers retrofitting context in a large codebase often choose the struct-embedding shortcut to minimize call-site changes — which is exactly what the official guide warns against.

**How to avoid:**
- Add `context.Context` as the first parameter of `Execute`, accepting that every addon file gets a one-line signature change
- Or, for a minimal-impact approach matching the database/sql pattern: add `ExecuteContext(ctx context.Context, ac *ActionContext) error` alongside the existing `Execute(*ActionContext) error`, and migrate callers incrementally
- Do not store `context.Context` inside `ActionContext`
- The `exec.Cmder` interface already has `CommandContext(context.Context, ...)` — use this, not `Command()`, in all future code

**Warning signs:**
- Any struct definition that has a field of type `context.Context` — flag for immediate review
- `context.Background()` or `context.TODO()` calls inside `Execute` methods — these are placeholders that mean "we forgot to propagate context"
- `timeout` or `deadline` logic that duplicates what `context.WithTimeout` would provide

**Phase to address:** Phase 2 (architecture improvements) — define the migration strategy before writing any context-aware code; do not allow `context.Context` in structs

---

### Pitfall 4: Testing kubectl-Based Addon Code Without a Real Cluster — Interface Boundary Confusion

**What goes wrong:**
Every addon action calls `node.Command("kubectl", ...).Run()` where `node` is `nodes.Node`, which embeds `exec.Cmder`. The `exec.Cmder` interface is:
```go
type Cmder interface {
    Command(string, ...string) Cmd
    CommandContext(context.Context, string, ...string) Cmd
}
```
This interface is the seam for testing. However, in practice addon actions obtain `node` from `ctx.Nodes()` which calls `ctx.Provider.ListNodes()` — a real Docker/Podman call. Tests need to inject a fake node that implements `nodes.Node` without a running cluster.

The pitfall: developers write tests that call `action.Execute()` directly and pass a real `ActionContext` created with a nil provider, causing a nil pointer panic when `ctx.Nodes()` is called. Or they mock the provider but their `FakeNode.Command()` returns a `FakeCmd` that doesn't properly capture stdin piped via `SetStdin()` — making tests pass even though the real `kubectl apply -f -` behavior is broken.

A second pitfall: `Status.Start()` / `Status.End()` in every addon's `Execute()` writes to a spinner. In tests with a nil status, these calls panic.

**Why it happens:**
The `ActionContext` is a concrete struct, not an interface. Unit testing requires constructing an `ActionContext` with mocked dependencies, but the constructor `NewActionContext()` requires a real `providers.Provider`. Tests that skip this end up either testing nothing useful or requiring a live cluster.

**How to avoid:**
- Create a `FakeNode` type in a `testutil` package that implements `nodes.Node` using a command script map:
  ```go
  type FakeNode struct {
      Commands map[string]FakeCmdOutput  // keyed by first arg (e.g., "kubectl")
  }
  ```
- Create a `FakeProvider` that returns `[]nodes.Node{fakeNode}` from `ListNodes()`
- Use `cli.StatusForLogger(log.NoopLogger{})` for the status field — the NoopLogger already exists in the codebase
- Test the logical behavior (does the addon call `kubectl apply --server-side`? does it wait for deployment availability?) not the kubectl output format
- Never test by asserting kubectl stderr — that's the cluster's concern, not the addon's

**Warning signs:**
- Test file imports `testing` but also `os/exec` directly — usually means the test is accidentally running real commands
- Test that passes when `$KUBECONFIG` is not set but fails when it is — means test is leaking into real cluster
- `defer ctx.Status.End(false)` called in test with a nil Status — panic on test run

**Phase to address:** Phase 3 (unit tests for addon actions) — build the FakeNode / FakeProvider test infrastructure first, then write addon tests on top of it

---

### Pitfall 5: Addon Registration Refactoring — Import Cycle When Moving to Registry Pattern

**What goes wrong:**
The current hard-coded addon list in `create.go` imports each addon package directly:
```go
import (
    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installcertmanager"
    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installenvoygw"
    ...
)
```
A registry pattern moves addon registration to a central `registry.go` that maps addon names to factory functions. The pitfall: if the registry package lives in the `actions/` directory and each addon package imports from `actions/` (to get the `Action` interface and `ActionContext`), then the registry importing each addon and each addon importing `actions/` creates a cycle:

```
actions/registry → actions/installcertmanager → actions/ (for ActionContext)
```

This is a compile error. Go's `internal/` boundary does not prevent cycles — it only prevents external access.

The second pitfall: the `Addons` struct in the config API (`pkg/internal/apis/config/types.go`) is a plain struct with boolean fields. A registry pattern that maps config field names to addon constructors must reference the config struct — creating a dependency from the registry down to the config package, which is fine, but if the config package is later moved, all registration code breaks.

**Why it happens:**
Developers design the registry as a "hub" that knows about all addons, not realizing that the Action/ActionContext types are in the package the addons also depend on. The classic solution is to put the interface in a separate package from its implementations and from the registry.

**How to avoid:**
- Keep the `Action` interface and `ActionContext` in a leaf package with no addon dependencies (they already are in `pkg/cluster/internal/create/actions/`)
- Put the registry in `create.go` itself (the caller), not in a sub-package — a simple `map[string]func() actions.Action` defined in `create.go` eliminates the cycle
- Alternatively: use `init()` registration where each addon package registers itself into a package-level map in `actions/` on import — but this requires blank imports (`_ "...installcertmanager"`) in `create.go`, which is an unusual pattern that confuses code readers
- The safest refactor: replace the hard-coded `runAddon(...)` sequence with a `[]addonEntry{name, enabled, factory}` slice in `create.go` — no new packages required, no cycles possible

**Warning signs:**
- `go build` error: "import cycle not allowed" immediately after introducing a registry package
- Registry package that contains `import "sigs.k8s.io/kind/pkg/cluster/internal/create/actions/install..."` — every such import is a dependency the registry must manage
- `init()` functions in addon packages that register into a global — global state, hard to test

**Phase to address:** Phase 2 (architecture improvements) — design the data structure for the registry in `create.go` before creating any new packages; verify no cycles with `go build ./...` after each step

---

### Pitfall 6: Parallel Addon Execution — Race Conditions and Dependency Ordering Violations

**What goes wrong:**
The current sequential addon execution has an implicit dependency ordering:
```
LocalRegistry → MetalLB → MetricsServer → CoreDNSTuning → EnvoyGateway → Dashboard → CertManager
```
EnvoyGateway depends on MetalLB being ready (it needs LoadBalancer IPs for its services). CertManager is last, and while it doesn't depend on the others structurally, it is the heaviest operation (300s wait for webhooks).

If addons are parallelized naively:
1. EnvoyGateway starts before MetalLB's `IPAddressPool` is created — EnvoyGateway's LoadBalancer service gets no external IP, Gateway never becomes ready
2. CertManager's webhook is applied and immediately another goroutine attempts a `ClusterIssuer` — but the webhook is not yet ready, causing "connection refused" errors that are mistaken for success (because `kubectl apply` treats webhook timeout as an error but the addon's error handling may swallow it)
3. The `ActionContext.cache` stores nodes in a `sync.RWMutex`-protected cache, but if two addons concurrently call `ctx.Nodes()` for the first time, both pass the nil check and make concurrent `ListNodes()` calls — this is a data race on the cache even though a mutex is present (the check-then-set is not atomic)

The existing `cachedData` struct in `action.go`:
```go
func (ac *ActionContext) Nodes() ([]nodes.Node, error) {
    cachedNodes := ac.cache.getNodes()  // RLock
    if cachedNodes != nil {
        return cachedNodes, nil
    }
    n, err := ac.Provider.ListNodes(ac.Config.Name)
    // ... setNodes — Lock
}
```
This is a TOCTOU (time-of-check/time-of-use) race: two goroutines can both see `cachedNodes == nil`, both call `ListNodes`, and both call `setNodes`. The result is duplicated provider calls (benign) but the mutex does not prevent the race between the nil check and the set.

Additionally, the `Status` spinner is not goroutine-safe for concurrent `Start()`/`End()` calls from multiple addons — the spinner writes to a single writer and is designed for sequential use.

**Why it happens:**
Sequential code that works correctly is parallelized by wrapping each addon in a goroutine and using `errors.AggregateConcurrent()` (which already exists in the codebase). The dependency ordering is implicit — not encoded anywhere — so parallelization appears to work in testing but fails in production when addons take different amounts of time.

**How to avoid:**
- Map out explicit dependencies before writing any parallel code:
  ```
  Group A (no deps): LocalRegistry, MetricsServer, CoreDNSTuning
  Group B (needs Group A done): MetalLB
  Group C (needs MetalLB): EnvoyGateway
  Group D (no deps, independent): CertManager
  Group E (no deps, independent): Dashboard
  ```
- Execute groups sequentially; within each group, execute in parallel with `errors.AggregateConcurrent()`
- Fix the `Nodes()` cache TOCTOU before enabling parallel execution: use `sync.Once` instead of check-then-set:
  ```go
  type cachedData struct {
      once  sync.Once
      nodes []nodes.Node
      err   error
  }
  ```
- For the Status spinner: each addon should report status through a channel to a single goroutine that owns the spinner, or use separate log lines per addon (no spinner in parallel mode)
- Run `go test -race ./...` explicitly after any parallelism is introduced

**Warning signs:**
- `go test -race` reports data races in `action.go` or any addon package after parallelization
- Cluster creation with `--addons all` succeeds in testing but EnvoyGateway Gateway has no external IP
- CertManager installation reports success but `kubectl get clusterissuer` shows `NotReady`

**Phase to address:** Phase 4 (parallel addon execution) — document dependency DAG first; implement group-sequential execution before full parallelism; enable `-race` in CI

---

### Pitfall 7: JSON Output — stdout Contamination Breaking Machine-Readable Output

**What goes wrong:**
All existing commands write human-readable output to `logger.V(0).Infof(...)` which writes to the logger's writer (typically stderr or stdout depending on setup). Adding `--output json` to commands like `kinder get clusters` requires that ONLY valid JSON goes to stdout when the flag is set. Any progress line, salutation, or warning that leaks to stdout breaks JSON consumers (`jq` fails with parse error on the first non-JSON line).

The kinder codebase mixes output destinations:
- `logger.V(0).Infof(...)` → logger writer (could be stdout)
- `fmt.Fprintln(streams.Out, cluster)` → `streams.Out` (explicitly stdout)
- `ctx.Status.Start(...)` → spinner writer

In the existing `get clusters` command, output is via `fmt.Fprintln(streams.Out, cluster)`. Adding JSON output means replacing this with `json.NewEncoder(streams.Out).Encode(result)`. But if any other code path writes to `streams.Out` before or after (e.g., "Creating cluster" progress lines that someone left routing to Out instead of Err), the JSON is broken.

Additionally, `json.Encoder.Encode()` appends a trailing newline after each value. If the caller naively JSON-encodes multiple items in a loop, they get JSON Lines (one JSON object per line), not a JSON array. The consumer expecting a JSON array will fail.

**Why it happens:**
The human-readable path was built first and only uses the logger. JSON output is added later as a flag, but the developer forgets that logger.V(0) writes to the same writer that JSON must exclusively own. The fix (route logger to stderr, JSON to stdout) requires changing how the logger is initialized at command startup — a wider change than it appears.

**How to avoid:**
- When `--output json` is active, the logger's writer MUST be `os.Stderr`, not `os.Stdout`; enforce this at the command entry point, not deep in the action layer
- Use `json.NewEncoder(streams.Out).Encode([]item{...})` once with a full array — do not encode individual items in a loop
- Add a golden-file test: run the command with `--output json` and pipe through `jq empty` — if jq exits non-zero, the JSON is malformed
- Define the JSON schema (which fields, what types) before implementation; use a struct with `json:"..."` tags, not `map[string]interface{}`
- Respect POSIX convention: errors → stderr, data → stdout; apply this regardless of output format

**Warning signs:**
- `kinder get clusters --output json | jq .` fails with "parse error: Invalid numeric literal at line 2"
- Logger output includes spinner frames interleaved with JSON (the spinner writes carriage returns that corrupt JSON)
- Output changes between `--verbosity 0` and `--verbosity 1` when `--output json` is set (should be identical)

**Phase to address:** Phase 5 (JSON output) — define the stream routing rule (logger → stderr, data → stdout) as a pre-condition; write the golden-file test before the implementation

---

### Pitfall 8: Package Movement (Layer Violations) — Compile Succeeds but Upstream Sync Breaks

**What goes wrong:**
The `pkg/fs/` package is marked as a layer violation (it is public API but should be `pkg/internal/fs/`). Moving it requires updating every import path in the codebase. The Go compiler enforces this correctly — `go build ./...` will fail if any import is missed.

However, kinder's module path is `sigs.k8s.io/kind` (not `sigs.k8s.io/kinder`). Any package movement must be reflected in import paths that still say `sigs.k8s.io/kind/pkg/fs/...`. This is correct for kinder's purposes, but any future attempt to sync upstream kind changes will encounter conflicts: upstream kind will have its own `pkg/fs/` unchanged, while kinder has moved it to `pkg/internal/fs/`. A cherry-pick or merge will produce import path conflicts that must be resolved manually.

The second layer violation is `pkg/internal/cli` importing from `pkg/cluster/internal/` — moving these packages changes the visibility of `internal` to external packages. Go's `internal` package rules apply per-tree: code in `pkg/internal/` cannot be imported by code outside `pkg/`. If the refactor moves a package from `internal` to public, the change may be noticed by upstream kind reviewers on any future PRs. If it moves from public to `internal`, existing code that referenced the old path stops compiling.

**Why it happens:**
Package organization in a fork is rarely updated because of the merge conflict cost. The original kind codebase has these "violations" as well — they accumulated over time and were never fixed upstream. In a fork, fixing them creates divergence that makes upstream syncs harder.

**How to avoid:**
- Before moving any package: `grep -rn "pkg/fs\b" --include="*.go" .` to find every import site; update them all atomically in one commit
- Use `gorename` or a simple `sed` script to update all import paths at once; do not do it manually file by file
- After the move: `go build ./...` and `go test ./...` must both pass
- Document the package move in a comment in the new location: `// Moved from pkg/fs/ in kinder v1.4; upstream kind still uses pkg/fs/`
- Accept that moving packages increases upstream sync difficulty; only move packages where the benefit (cleaner architecture) outweighs the merge cost

**Warning signs:**
- `go build ./...` fails with "use of internal package sigs.k8s.io/kind/pkg/internal/... not allowed" after a move
- A cherry-picked upstream kind commit fails because import paths no longer match
- IDE shows "import cycle" warnings after a move (the move introduced a cycle that was not present before)

**Phase to address:** Phase 2 (architecture improvements) — do package moves in isolated commits; never combine a package move with a behavioral change; verify with `go build` and `go vet` immediately

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Store `context.Context` in `ActionContext` struct | Fewer function signature changes | Violates Go idiom; contexts cannot be cancelled per-call; misleads future developers | Never |
| Parallelize all addons at once | Simple implementation | Hidden dependency violations cause intermittent failures | Never — use dependency-group sequential execution |
| `go get -u ./...` bulk update | One command | Cascading test failures across multiple deps; hard to bisect | Never in one step; update one dep at a time |
| `fmt.Println` for JSON output | Quick to implement | Breaks machine parsing when any other code writes to stdout | Never for JSON output |
| `init()` registration for addon registry | No changes to `create.go` | Global state, untestable, import order-dependent | Acceptable if no alternative, but document it clearly |
| Move package + change behavior in one commit | Fewer PRs | Impossible to bisect; refactor vs. bug fix confusion | Never — always separate moves from logic changes |

---

## Integration Gotchas

Common mistakes when connecting the new features to existing system.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| ActionContext + context.Context | Add `Ctx context.Context` field to ActionContext | Pass `ctx context.Context` as first param to `Execute()` |
| Addon registry + config API | Registry references config struct field names as strings | Registry receives enabled bool, not config field name; boolean is determined at the call site |
| JSON output + logger | Logger writes to streams.Out (stdout) | When JSON mode active, force logger writer to os.Stderr before any output |
| Parallel addons + spinner | Multiple goroutines call Status.Start() concurrently | Use a dedicated goroutine for status output; addons send events via channel |
| go.mod version bump + rand | Bump go directive but leave `rand.NewSource(time.Now().UnixNano())` | After bumping to 1.20+, simplify to `rand.Intn(len(salutations))` |
| Package move + internal boundary | Move pkg/fs to pkg/internal/fs without checking callers | Use `grep` to find all import sites; update atomically |
| Fake node tests + stdin piping | FakeCmd.Run() returns success without consuming stdin | FakeCmd must verify SetStdin was called for kubectl apply commands |

---

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Naive full parallelism of all addons | EnvoyGateway gets no LoadBalancer IP; CertManager ClusterIssuer fails | Execute dependency groups sequentially, parallelize within groups | Immediately — timing-dependent |
| Parallel addons sharing Status spinner | Spinner output garbled; carriage returns corrupt JSON output | Route status to a channel owned by a single writer goroutine | With 2+ concurrent addons |
| TOCTOU in ActionContext.Nodes() cache | Double ListNodes() calls under concurrency | Replace check-then-set with sync.Once | With 2+ concurrent addons calling Nodes() |
| json.Encode per item in loop | JSON Lines output instead of JSON array | Collect all items, encode once as array | When consumer expects []T not one T per line |

---

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Logging cluster credentials in JSON output | kubeconfig or token written to structured logs | Never include sensitive fields in JSON output structs; use redaction |
| World-readable permissions on moved pkg/fs output | Sensitive kubeconfig files copied with 0644 | Preserve or restrict permissions; audit pkg/fs.Copy() after any refactor |
| context.WithTimeout shorter than kubectl --timeout | Kubectl reports success, context cancelled before kubectl completes | Context timeout must be longer than the longest kubectl --timeout used in the action |

---

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **go.mod version bump:** `go` directive bumped — but `toolchain` directive also updated, AND comment-gated code (`rand.NewSource`) cleaned up, AND `go mod tidy` run with the new toolchain
- [ ] **JSON output:** Returns valid JSON — but also tested that `--output json | jq empty` exits 0, AND logger routes to stderr in JSON mode, AND output is a JSON array not JSON lines
- [ ] **Parallel addon execution:** Addons run concurrently — but also the dependency DAG is documented and enforced, AND `-race` detector shows no races, AND integration test verifies EnvoyGateway gets a LoadBalancer IP
- [ ] **Addon registry refactoring:** `create.go` no longer has hard-coded imports — but also `go build ./...` shows no import cycles, AND adding a new addon only requires one file change (not changes in create.go)
- [ ] **context.Context in actions:** `Execute()` signature updated — but also no `context.Context` stored in any struct field, AND all call sites pass a real context (not `context.Background()`)
- [ ] **Unit tests for addon actions:** Tests compile and pass — but also they test behavior (does the action call kubectl apply with --server-side?), not implementation, AND they work without `$KUBECONFIG` set
- [ ] **Package move (pkg/fs → pkg/internal/fs):** All imports updated — but also `go build ./...` passes, AND the internal boundary does not prevent packages that previously accessed pkg/fs from accessing pkg/internal/fs

---

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| toolchain directive breaks CI | LOW | Pin `toolchain go1.X.Y` explicitly; check in go.mod+go.sum together; update CI to use GOTOOLCHAIN=local |
| Dependency cascade breaks tests | MEDIUM | `git bisect` to find which `go get` broke the test; revert that dep to previous version; update selectively |
| context.Context in struct discovered late | MEDIUM | Create migration branch; add `ExecuteContext()` alongside `Execute()`; migrate callers incrementally; deprecate old signature |
| Import cycle after registry refactor | LOW | `go build ./...` immediately shows the cycle; move the registry to `create.go`; no new package needed |
| Parallel addon race condition discovered in production | HIGH | Revert to sequential execution; document the dependency DAG; re-implement parallel execution with explicit groups |
| JSON output with stdout contamination | LOW | Add `--output json` golden-file test that pipes through `jq`; fix logger routing to stderr; re-verify |
| Package move breaks upstream cherry-pick | MEDIUM | Document the moved path in the new location; apply upstream patch to both old and new path; verify with `go build` |
| Fake node test doesn't catch real bug | MEDIUM | Add integration test that creates a real cluster and verifies addon output (separate slow test suite) |

---

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| go.mod version bump + toolchain | Phase 1: go.mod and dependencies | `go mod tidy` produces stable go.mod; comment-gated code removed; CI passes |
| Dependency cascade | Phase 1: incremental dep update | `go test ./...` passes after each individual dep update |
| context.Context in struct | Phase 2: architecture improvements | `grep -rn "context\.Context" --include="*.go" pkg/ \| grep "struct {"` returns 0 matches |
| Import cycle in registry refactor | Phase 2: architecture improvements | `go build ./...` after each step; no packages import their callers |
| Package move breaks boundary | Phase 2: architecture improvements | `go build ./...` + `go test ./...` pass immediately after move commit |
| Fake node test infrastructure | Phase 3: unit tests | Addon tests pass without KUBECONFIG set; no real commands executed |
| JSON stdout contamination | Phase 5: JSON output | `kinder get clusters --output json \| jq empty` exits 0 |
| Parallel addon race condition | Phase 4: parallel execution | `go test -race ./...` shows no races; integration test verifies EnvoyGateway gets IP |
| Addon dependency ordering violation | Phase 4: parallel execution | Dependency DAG documented before any goroutine is introduced |

---

## Fork-Specific Pitfalls

Issues unique to kinder's status as a fork of `sigs.k8s.io/kind`.

### Pitfall F1: Module Path Still Says `sigs.k8s.io/kind`

The `go.mod` declares `module sigs.k8s.io/kind`. All internal imports use this path. If the module path is ever changed to `github.com/PatrykQuantumNomad/kinder`, EVERY import statement in 28K LOC must be updated simultaneously. The risk is not in the change itself — it is in partial changes, where some files are updated and others are not, causing compile failures that are hard to trace.

**Prevention:** Do not change the module path unless absolutely necessary. If changing, use a single automated script (`find . -name "*.go" -exec sed -i 's|sigs.k8s.io/kind|github.com/PatrykQuantumNomad/kinder|g' {} +`) and verify with `go build ./...` before committing anything.

**Phase to address:** Not in v1.4 scope — defer this decision; note in PROJECT.md.

### Pitfall F2: Code Quality Changes Make Upstream Syncs Harder

Every code quality improvement (renaming error variables, moving packages, adding context parameters, extracting shared code) increases the diff from upstream kind. When upstream releases a security fix, cherry-picking it may fail because the surrounding context has changed.

**Prevention:**
- Keep a `DIVERGENCE.md` (or equivalent) listing every deliberate divergence from upstream kind
- Prefer additive changes (new files, new packages) over modifying upstream files when possible
- When modifying upstream files, keep the change minimal and clearly marked with `// kinder: <reason>` comments
- Run `git diff upstream/main HEAD -- path/to/modified/file` periodically to assess upstream sync risk

**Phase to address:** Ongoing — document each divergence in the commit message with the prefix "kinder:"

---

## Sources

- [Go 1.21 Release Notes — toolchain directive](https://tip.golang.org/doc/go1.21) — authoritative source on go.mod toolchain behavior change (HIGH confidence)
- [Go Toolchains reference](https://go.dev/doc/toolchain) — GOTOOLCHAIN, toolchain directive semantics (HIGH confidence)
- [cmd/go Issue #62409 — go get -u + go mod tidy toolchain behavior change](https://github.com/golang/go/issues/62409) — real-world CI breakage from toolchain directive after update (HIGH confidence)
- [Contexts and structs — official Go blog](https://go.dev/blog/context-and-structs) — "Do not store Contexts inside a struct type" (HIGH confidence)
- [Go's context library anti-patterns — Medium](https://medium.com/@gosamv/gos-context-library-more-patterns-and-anti-patterns-6bc48eaf774e) — context retrofitting patterns (MEDIUM confidence)
- [cert-manager webhook readiness — official docs](https://cert-manager.io/docs/concepts/webhook/) — bootstrapping window and timing requirements (HIGH confidence)
- [cert-manager troubleshooting webhook — official docs](https://cert-manager.io/docs/troubleshooting/webhook/) — "connection refused" during webhook startup (HIGH confidence)
- [Testing os/exec.Command — npf.io](https://npf.io/2015/06/testing-exec-command/) — helper process pattern for fake commands (MEDIUM confidence)
- [k8s.io/utils/exec/testing — pkg.go.dev](https://pkg.go.dev/k8s.io/utils/exec/testing) — FakeExec pattern from Kubernetes itself (HIGH confidence)
- [Go data race detector — official docs](https://go.dev/doc/articles/race_detector) — `-race` flag usage for detecting parallel addon races (HIGH confidence)
- [Import cycles in Go — Jogendra](https://jogendra.dev/import-cycles-in-golang-and-how-to-deal-with-them) — import cycle causes and solutions (MEDIUM confidence)
- [Go forum: how to properly fork a golang module](https://forum.golangbridge.org/t/how-to-properly-fork-a-golang-module/27846) — module path management for forks (MEDIUM confidence)
- Kinder codebase direct analysis: `pkg/cluster/internal/create/create.go`, `pkg/cluster/internal/create/actions/action.go`, `pkg/exec/types.go`, `pkg/exec/local.go`, `pkg/errors/concurrent.go`, `go.mod` — (HIGH confidence — primary sources)

---

*Pitfalls research for: kinder v1.4 — code quality improvements, architecture fixes, unit tests, new features added to existing kind fork*
*Researched: 2026-03-03*
