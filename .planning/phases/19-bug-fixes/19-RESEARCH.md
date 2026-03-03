# Phase 19: Bug Fixes - Research

**Researched:** 2026-03-03
**Domain:** Go — correctness bugs in container runtime provider code
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Fix isolation
- One commit per bug — each fix independently testable and revertable
- Each bug gets its own unit test(s) proving the fix
- Fix order: tackle them in BUG-01 → BUG-04 sequence (port leak, tar truncation, empty cluster name, network sort)

#### BUG-01: Port listener leak
- Port listeners acquired in `generatePortMappings` must be released at iteration end, not at function return
- Fix must handle early exit paths — no port leak under any code path
- Verify fix across all three providers since `generatePortMappings` exists in each

#### BUG-02: Silent tar truncation
- Tar extraction must return an error on truncated files instead of silently stopping mid-archive
- Fail explicitly — no silent data loss
- Error message should indicate truncation specifically (not a generic I/O error)

#### BUG-03: Empty cluster name resolution
- `ListInternalNodes` must resolve empty cluster name to the default cluster name consistently across all three providers
- Verify the default name resolution matches what `kind` itself uses
- All three providers must behave identically for this case

#### BUG-04: Network sort comparator
- Network sort comparator must satisfy strict weak ordering — identical networks compare equal
- Sort must be deterministic across runs
- Fix the comparison logic, don't work around it with stable sort

### Claude's Discretion
- Exact test structure (table-driven vs individual test functions)
- Whether to add helper functions for test setup
- Specific error message wording for BUG-02

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| BUG-01 | Fix defer-in-loop port leak in generatePortMappings across all 3 providers | Bug location confirmed in docker, podman, nerdctl provision.go; fix pattern identified |
| BUG-02 | Fix tar extraction silent data loss on truncated files (return error instead of break) | Bug location confirmed in pkg/build/nodeimage/internal/kube/tar.go:78-80; fix pattern identified |
| BUG-03 | Fix ListInternalNodes missing defaultName() call for consistent cluster name resolution | Bug location confirmed in pkg/cluster/provider.go:231; defaultName() helper exists at line 48 |
| BUG-04 | Fix network sort comparator to use strict weak ordering | Bug location confirmed in pkg/cluster/internal/providers/docker/network.go:204-212; fix pattern identified |
</phase_requirements>

## Summary

This phase fixes four correctness bugs found during codebase review, all in the Go provider code. They are well-isolated, independently fixable, and each affects real user-visible behavior. The project uses standard Go testing patterns with table-driven tests, the stdlib `testing` package, and a small project-local `assert` package.

The bugs are located in specific files: BUG-01 (`defer` in loop) in all three provider provision.go files; BUG-02 (silent break on truncation) in `pkg/build/nodeimage/internal/kube/tar.go`; BUG-03 (missing `defaultName()` call) in `pkg/cluster/provider.go`; BUG-04 (non-strict sort comparator) in `pkg/cluster/internal/providers/docker/network.go`. All bugs have been read and confirmed against the actual source code.

No new dependencies are required. All fixes are pure Go changes to existing functions. Each fix can be proven with a unit test that would fail before the fix and pass after. The existing test suite runs with `go test ./pkg/...` and all tests pass at the start of this phase.

**Primary recommendation:** Fix one bug at a time, write the test first (it should fail), apply the fix (test passes), commit. Run `go test ./pkg/...` between each commit to confirm no regressions.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `testing` | stdlib | Unit test framework | Only test framework in use across the project |
| `archive/tar` | stdlib | Tar archive reading | Already in use in the affected files |
| `io` | stdlib | io.EOF, io.ErrUnexpectedEOF sentinel errors | Used for truncation detection |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/internal/assert` | local | DeepEqual, ExpectError helpers | Use for asserting slice equality in BUG-04 test |
| `compress/gzip` | stdlib | Gzip reader for tar tests | Used in existing `tar_test.go` helper |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| stdlib `testing` | testify | testify is not in go.mod; don't add it — this project uses stdlib + local assert |
| `io.ErrUnexpectedEOF` | custom error | stdlib sentinel is clearer and machine-comparable; custom error requires string matching |
| Inline test helpers | shared helpers | Inline is fine for 4 independent bug tests; reuse only if helper is already in the package |

**Installation:** No new packages needed. All dependencies are stdlib or already in `go.mod`.

## Architecture Patterns

### Recommended Project Structure

All bug fixes are in-place changes to existing files. No new files are needed for the fixes themselves. New test files may be needed where test coverage is absent.

```
pkg/
├── cluster/
│   ├── provider.go                                         # BUG-03 fix here
│   └── internal/providers/
│       ├── common/
│       │   └── getport_test.go                             # existing; no changes needed
│       ├── docker/
│       │   ├── provision.go                                # BUG-01 fix here
│       │   ├── provision_test.go                           # BUG-01 test here (new file)
│       │   └── network.go                                  # BUG-04 fix here
│       │   └── network_test.go                             # BUG-04 test added here (existing file)
│       ├── podman/
│       │   └── provision.go                                # BUG-01 fix here
│       │   └── provision_test.go                           # BUG-01 test here (new file)
│       └── nerdctl/
│           └── provision.go                                # BUG-01 fix here
│           └── provision_test.go                           # BUG-01 test already exists; extend
└── build/nodeimage/internal/kube/
    ├── tar.go                                              # BUG-02 fix here
    └── tar_test.go                                         # BUG-02 test added here (existing file)
```

### Pattern 1: Defer-in-Loop Fix (BUG-01)

**What:** `defer` inside a `for` loop defers until function return, not loop iteration end. The `releaseHostPortFn` (which closes a TCP listener holding a free port) is deferred inside `generatePortMappings`'s loop. All port listeners accumulate and are only released when the function returns — but by then the caller has already passed the port string to docker, making the listener unnecessary. The critical window is: if the function returns an error early (e.g., unknown protocol), all previously acquired ports leak until function exit.

**When to use:** Replace `defer releaseHostPortFn()` with an explicit call at the end of each iteration, using a local closure or a `// release immediately after use` comment.

**The correct fix pattern:**
```go
// Source: https://go.dev/doc/faq#closures_and_goroutines (defer in loop anti-pattern)

// BEFORE (buggy — all releases deferred to function return):
hostPort, releaseHostPortFn, err := common.PortOrGetFreePort(pm.HostPort, pm.ListenAddress)
if err != nil {
    return nil, errors.Wrap(err, "failed to get random host port for port mapping")
}
if releaseHostPortFn != nil {
    defer releaseHostPortFn()
}

// AFTER (fixed — release at end of each iteration):
hostPort, releaseHostPortFn, err := common.PortOrGetFreePort(pm.HostPort, pm.ListenAddress)
if err != nil {
    return nil, errors.Wrap(err, "failed to get random host port for port mapping")
}
if releaseHostPortFn != nil {
    releaseHostPortFn()
}
```

**Why immediate release is correct:** `GetFreePort` opens a TCP listener on port 0 to get the OS to assign a free port, then returns that port number. The listener exists to "hold" the port while other ports are being selected — preventing the OS from reusing it for another call. Once all ports in the batch are determined, the listeners must be released so the OS can then bind those same ports to docker. Releasing immediately after the port string is generated is correct because the port is now committed in the `args` slice — no further calls to `GetFreePort` happen in this iteration.

### Pattern 2: Truncation Error (BUG-02)

**What:** `io.CopyN` returns `(n, io.EOF)` when the source is exhausted before copying `n` bytes — this is the definition of a truncated file. The existing code treats this as a silent `break`, successfully extracting partial content with no error.

**The correct fix pattern:**
```go
// Source: https://pkg.go.dev/io#CopyN (io.EOF from CopyN means premature end)

// BEFORE (buggy — silently stops on truncation):
if _, err := io.CopyN(f, tr, hdr.Size); err != nil {
    f.Close()
    if err == io.EOF {
        break  // BUG: truncated file treated as success
    }
    return fmt.Errorf("extracting image data: %w", err)
}

// AFTER (fixed — explicit truncation error):
if _, err := io.CopyN(f, tr, hdr.Size); err != nil {
    f.Close()
    if err == io.EOF {
        return fmt.Errorf("archive truncated: unexpected EOF while extracting %s", hdr.Name)
    }
    return fmt.Errorf("extracting image data: %w", err)
}
```

**io.ErrUnexpectedEOF vs io.EOF:** `io.CopyN` wraps the `io.EOF` it receives from the source into `io.ErrUnexpectedEOF` when n bytes have not been fully copied. Verify this behavior: if `io.CopyN` already returns `io.ErrUnexpectedEOF` (not `io.EOF`) in this case, the existing `if err == io.EOF` branch is dead code and truncation already surfaces as an error. Read the stdlib source to confirm before writing the test.

**Verification:** Check Go stdlib behavior. `io.CopyN` source at https://cs.opensource.google/go/go/+/refs/tags/go1.22.0:src/io/io.go;l=339 shows that `CopyN` calls `Copy` and if `n == 0 && err == nil`, it returns. Actually `CopyN` returns `io.ErrUnexpectedEOF` when it reads less than N bytes and gets `io.EOF`. This means the current `if err == io.EOF` branch may never trigger — the truncation escapes through the `return fmt.Errorf("extracting image data: %w", err)` line with `io.ErrUnexpectedEOF`. Write a test with a truncated tar to confirm actual behavior, then make the error message specifically describe truncation.

### Pattern 3: defaultName() Resolution (BUG-03)

**What:** `ListInternalNodes` at line 231 calls `p.provider.ListNodes(name)` directly. Compare to `ListNodes` at line 225 which calls `p.provider.ListNodes(defaultName(name))`. The `defaultName()` helper resolves empty string to `"kind"` (the default cluster name). When callers pass `""` as name, `ListNodes` returns nodes for the default cluster, but `ListInternalNodes` returns nodes for a cluster named `""` (which doesn't exist), silently returning an empty list.

**The correct fix:**
```go
// Source: pkg/cluster/provider.go lines 47-53

// BEFORE (buggy — no defaultName call):
func (p *Provider) ListInternalNodes(name string) ([]nodes.Node, error) {
    n, err := p.provider.ListNodes(name)

// AFTER (fixed — consistent with ListNodes):
func (p *Provider) ListInternalNodes(name string) ([]nodes.Node, error) {
    n, err := p.provider.ListNodes(defaultName(name))
```

**One-line fix.** No other changes needed. The `defaultName()` function is already defined in the same file (lines 47-53). All three providers implement the same `ListNodes` interface, so the fix propagates to all three correctly.

### Pattern 4: Strict Weak Ordering Fix (BUG-04)

**What:** `sortNetworkInspectEntries` uses `sort.Slice` with a comparator that violates strict weak ordering. For elements `i` and `j` where `len(networks[i].Containers) == len(networks[j].Containers)`, the comparator falls through to `networks[i].ID < networks[j].ID`. This is actually correct for the equal-container case — the bug is when `len(networks[i].Containers) < len(networks[j].Containers)`: the first condition is false, and the function returns `networks[i].ID < networks[j].ID` regardless of container count, which means a network with fewer containers could sort before one with more containers (if its ID happens to be lexicographically smaller).

**The current buggy comparator:**
```go
// Source: pkg/cluster/internal/providers/docker/network.go:204-212
func sortNetworkInspectEntries(networks []networkInspectEntry) {
    sort.Slice(networks, func(i, j int) bool {
        // we want networks with active containers first
        if len(networks[i].Containers) > len(networks[j].Containers) {
            return true
        }
        return networks[i].ID < networks[j].ID
    })
}
```

**The violation:** When `len(i.Containers) < len(j.Containers)`, this returns `i.ID < j.ID` — but for strict weak ordering, it should return `false` (j should precede i). If `i.ID < j.ID` happens to be true, the sort places i before j even though j has more containers. This produces non-deterministic results depending on the ID ordering relative to container counts.

**The correct fix — explicit three-way comparison:**
```go
// Source: Go blog on sort.Slice comparator requirements
func sortNetworkInspectEntries(networks []networkInspectEntry) {
    sort.Slice(networks, func(i, j int) bool {
        // networks with more active containers sort first
        iLen := len(networks[i].Containers)
        jLen := len(networks[j].Containers)
        if iLen != jLen {
            return iLen > jLen
        }
        // break ties deterministically by ID
        return networks[i].ID < networks[j].ID
    })
}
```

**Why this satisfies strict weak ordering:** When container counts differ, the higher-count network always wins (deterministic). When counts are equal, ID comparison is a total order. The `if iLen != jLen` branch handles both `i > j` and `i < j` correctly.

### Anti-Patterns to Avoid

- **defer in loop:** Already the bug — don't re-introduce it in the fix.
- **Using `sort.Stable` as a workaround for BUG-04:** The decision says "fix the comparison logic, don't work around it with stable sort." `sort.Stable` would hide the bug, not fix it — the comparator itself violates the contract.
- **Wrapping io.ErrUnexpectedEOF with fmt.Errorf:** Use `errors.Is()` for sentinel checks in tests, not string matching.
- **Testing `ListInternalNodes` by mocking the provider at the interface level:** The function is package-level in `pkg/cluster`; test it by verifying the `defaultName()` call is present. Since the provider interface requires a real docker/podman/nerdctl to test fully, the unit test should verify the defaultName behavior directly or via a stub.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Finding a free TCP port | Custom port scanner | `common.PortOrGetFreePort` / `common.GetFreePort` | Already in the codebase; handles all edge cases |
| Tar archive creation in tests | Custom binary builder | `archive/tar` + `compress/gzip` pattern from `tar_test.go` | `writeTarGz` helper in `tar_test.go` already shows the pattern |
| Truncated tar file in tests | Special tool | Write header with `Size > len(body)`, then close early | Construct via stdlib tar writer with mismatched sizes |

**Key insight:** The codebase already has `writeTarGz` in `tar_test.go` — use that same pattern to construct a truncated archive for BUG-02 tests.

## Common Pitfalls

### Pitfall 1: Misunderstanding io.CopyN and io.EOF vs io.ErrUnexpectedEOF
**What goes wrong:** Writing the BUG-02 test expecting `io.EOF` from `io.CopyN`, but the function actually wraps it as `io.ErrUnexpectedEOF`.
**Why it happens:** `io.CopyN` calls `io.Copy` internally and converts short reads. Go 1.17+ behavior: if CopyN reads fewer bytes than requested and gets EOF from the reader, it returns `io.ErrUnexpectedEOF`, not `io.EOF`.
**How to avoid:** Write the test first with a truncated archive, observe what error is returned with the buggy code, then write the fix to return a specific truncation message.
**Warning signs:** Test passes on the current buggy code without any code change — this means the truncation already errors, and the fix is only about the error message quality.

### Pitfall 2: BUG-01 Fix Releases Port Too Early
**What goes wrong:** Moving `releaseHostPortFn()` to end of loop body, but calling it before `args = append(args, ...)` — the port is released before it's captured in the string, allowing the OS to reuse it.
**Why it happens:** Refactoring the code without reading the sequence carefully.
**How to avoid:** Release the listener AFTER the `args = append(args, fmt.Sprintf(...hostPort...))` line. The port number is captured in the string, after which the listener is no longer needed.
**Warning signs:** Test that calls `generatePortMappings` multiple times and checks for unique ports fails intermittently.

### Pitfall 3: BUG-03 Test Requires Mocking the Provider
**What goes wrong:** Writing a unit test for `ListInternalNodes` that requires a real provider instance, leading to docker/podman exec failures in CI.
**Why it happens:** `ListInternalNodes` calls into the provider interface, which calls docker/podman.
**How to avoid:** Test at the unit level by verifying the source code uses `defaultName(name)` rather than `name` directly. Alternatively, implement a minimal mock provider that tracks what argument `ListNodes` was called with.
**Warning signs:** Test passes locally (docker installed) but fails in CI.

### Pitfall 4: BUG-04 Only Fixed in Docker Provider
**What goes wrong:** Only fixing `sortNetworkInspectEntries` in `docker/network.go` but missing that nerdctl/network.go does NOT have this function — it uses a simpler `checkIfNetworkExists` without deduplication logic. Podman also does not have `sortNetworkInspectEntries`.
**Why it happens:** Assumption that all three providers share the same network sorting code.
**How to avoid:** Read all three providers' network files. The sort bug is ONLY in `docker/network.go`. Nerdctl and podman use different deduplication strategies.
**Warning signs:** Grepping for `sortNetworkInspectEntries` only returns one file.

### Pitfall 5: Writing Tests That Require External Processes
**What goes wrong:** Writing tests for `generatePortMappings` that rely on docker being available to actually test port allocation.
**Why it happens:** `generatePortMappings` calls `common.PortOrGetFreePort` which calls `net.Listen` — this actually opens a TCP listener on localhost. This is NOT an external process and works fine in unit tests.
**How to avoid:** `net.Listen("tcp", "localhost:0")` works in unit tests. Only avoid calls to `exec.Command("docker", ...)`.

## Code Examples

Verified patterns from reading actual source:

### BUG-01: The Defer-in-Loop (Confirmed in Three Files)
```go
// Source: pkg/cluster/internal/providers/docker/provision.go:392-398
// Source: pkg/cluster/internal/providers/nerdctl/provision.go:362-368
// Source: pkg/cluster/internal/providers/podman/provision.go:402-408

// All three have identical structure:
hostPort, releaseHostPortFn, err := common.PortOrGetFreePort(pm.HostPort, pm.ListenAddress)
if err != nil {
    return nil, errors.Wrap(err, "failed to get random host port for port mapping")
}
if releaseHostPortFn != nil {
    defer releaseHostPortFn()  // BUG: deferred to function return, not loop end
}
```

### BUG-02: The Silent Break (Confirmed in tar.go)
```go
// Source: pkg/build/nodeimage/internal/kube/tar.go:76-83
if _, err := io.CopyN(f, tr, hdr.Size); err != nil {
    f.Close()
    if err == io.EOF {
        break  // BUG: silently stops, caller gets nil error and partial extraction
    }
    return fmt.Errorf("extracting image data: %w", err)
}
```

### BUG-03: Missing defaultName (Confirmed in provider.go)
```go
// Source: pkg/cluster/provider.go:224-236
// Compare ListNodes (correct) vs ListInternalNodes (buggy):

func (p *Provider) ListNodes(name string) ([]nodes.Node, error) {
    return p.provider.ListNodes(defaultName(name))  // CORRECT: defaultName called
}

func (p *Provider) ListInternalNodes(name string) ([]nodes.Node, error) {
    n, err := p.provider.ListNodes(name)  // BUG: missing defaultName(name)
    if err != nil {
        return nil, err
    }
    return nodeutils.InternalNodes(n)
}
```

### BUG-04: The Broken Comparator (Confirmed in docker/network.go)
```go
// Source: pkg/cluster/internal/providers/docker/network.go:204-212
func sortNetworkInspectEntries(networks []networkInspectEntry) {
    sort.Slice(networks, func(i, j int) bool {
        // BUG: when i has FEWER containers than j, we fall through to ID comparison.
        // This means a network with 0 containers but ID "aaa" would sort before
        // a network with 5 containers but ID "zzz" — violating the intent.
        if len(networks[i].Containers) > len(networks[j].Containers) {
            return true
        }
        return networks[i].ID < networks[j].ID
    })
}
```

### Constructing a Truncated Tar for BUG-02 Test
```go
// Pattern: declare larger Size in header than actual body written
func writeTruncatedTarGz(t *testing.T) string {
    t.Helper()
    var buf bytes.Buffer
    gw := gzip.NewWriter(&buf)
    tw := tar.NewWriter(gw)
    hdr := &tar.Header{
        Name: "truncated.txt",
        Mode: 0o644,
        Size: 100,  // claim 100 bytes
    }
    if err := tw.WriteHeader(hdr); err != nil {
        t.Fatalf("writing header: %v", err)
    }
    // write only 5 bytes — archive is truncated
    if _, err := tw.Write([]byte("hello")); err != nil {
        t.Fatalf("writing body: %v", err)
    }
    // Close without proper padding — this produces a truncated archive
    _ = gw.Close()
    tmpFile := filepath.Join(t.TempDir(), "truncated.tar.gz")
    if err := os.WriteFile(tmpFile, buf.Bytes(), 0o644); err != nil {
        t.Fatalf("writing temp file: %v", err)
    }
    return tmpFile
}
```

Note: Verify this actually triggers the truncation path — the tar writer may add padding. An alternative approach is to write a valid archive and then truncate the file itself with `os.Truncate(path, size/2)`.

### Test Infrastructure Pattern (from existing tests)
```go
// Source: pkg/cluster/internal/providers/docker/network_test.go:72-162
// Table-driven tests with tc capture for parallel safety:
for _, tc := range cases {
    tc := tc  // capture range variable
    t.Run(tc.Name, func(t *testing.T) {
        // not parallel here — sort modifies in-place
        toSort := make([]networkInspectEntry, len(tc.Networks))
        copy(toSort, tc.Networks)
        sortNetworkInspectEntries(toSort)
        assert.DeepEqual(t, tc.Sorted, toSort)
    })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `defer` in loop for cleanup | Explicit call at end of iteration | Go FAQ since Go 1.x | Standard Go idiom; the bug predates kinder's fork |
| `break` on EOF in extraction | Return explicit error | N/A — this is the fix | Callers get explicit signal instead of silent partial success |
| Implicit sort ordering | Explicit three-way comparison | N/A — this is the fix | Deterministic, contract-satisfying sort |

**Deprecated/outdated:**
- `tc := tc` capture pattern in Go range loops: needed in Go < 1.22 (which kinder targets — go 1.17 minimum). Keep it. Go 1.22+ changed range variable semantics, but since go.mod says `go 1.17`, capturing is still required for correctness across all supported toolchains.

## Open Questions

1. **Does io.CopyN return io.EOF or io.ErrUnexpectedEOF on truncation?**
   - What we know: Go stdlib documentation says CopyN wraps short reads as io.ErrUnexpectedEOF
   - What's unclear: Whether the tar library's reader layer transforms this before CopyN sees it
   - Recommendation: Write the truncated-archive test FIRST on unmodified code to observe actual error behavior, then tailor the fix accordingly

2. **Can the BUG-03 test be a pure unit test without mocking?**
   - What we know: `ListInternalNodes` directly calls `p.provider.ListNodes(name)` — testing this requires either a real provider or a mock
   - What's unclear: Whether the project convention is to test at this level or only test through integration
   - Recommendation: Implement a minimal mock provider in the test file (private to the test, not exported). The mock records the argument passed to `ListNodes`. Verify it receives `"kind"` (the default) when called with `""`.

3. **Does the `extractTarball` truncation truly return nil today?**
   - What we know: The code says `break` on `io.EOF`, and the function ends with `return err` where `err` is the last loop `err` value
   - What's unclear: The outer loop `err` variable gets reassigned each iteration; after the `break`, `err` might be `io.EOF` (not nil)
   - Recommendation: The variable `err` at the end of the function (`return err`) holds the value from the last `tr.Next()` call that was not EOF. After `break`, control goes to the `return err` at line 90. At that point `err` is `nil` (the function-level `err` declared at line 31 is the named return, distinct from the loop `err`). Confirm by reading the scoping carefully: `hdr, err := tr.Next()` uses `:=` in the loop, creating a new scope — the function-level `err` at line 90 would be `nil`.

## Sources

### Primary (HIGH confidence)
- Direct source code read: `pkg/cluster/internal/providers/docker/provision.go` — BUG-01 confirmed
- Direct source code read: `pkg/cluster/internal/providers/nerdctl/provision.go` — BUG-01 confirmed
- Direct source code read: `pkg/cluster/internal/providers/podman/provision.go` — BUG-01 confirmed
- Direct source code read: `pkg/build/nodeimage/internal/kube/tar.go` — BUG-02 confirmed
- Direct source code read: `pkg/cluster/provider.go` — BUG-03 confirmed
- Direct source code read: `pkg/cluster/internal/providers/docker/network.go` — BUG-04 confirmed
- Direct source code read: existing test files in all three provider packages — testing patterns confirmed
- `go test ./pkg/...` run — all tests pass at baseline; no pre-existing failures

### Secondary (MEDIUM confidence)
- Go FAQ on defer in loops: https://go.dev/doc/faq#closures_and_goroutines
- Go stdlib io.CopyN behavior: https://pkg.go.dev/io#CopyN

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Bug locations: HIGH — confirmed by reading actual source code
- Fix patterns: HIGH — confirmed against Go stdlib semantics and existing codebase patterns
- Test patterns: HIGH — confirmed by reading existing tests in the same packages
- io.CopyN truncation behavior: MEDIUM — documented but not verified by running the truncated-archive test

**Research date:** 2026-03-03
**Valid until:** 2026-04-03 (stable Go codebase; bugs won't move unless someone else changes these files first)
