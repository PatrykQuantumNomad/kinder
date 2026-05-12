# Phase 56: DEBT-04 Doctor Test Race Fix - Research

**Researched:** 2026-05-12
**Domain:** Go testing concurrency (`testing.T.Parallel()`), dependency-injection refactor over package-global mutation, Go race detector (`go test -race`)
**Confidence:** HIGH (race reproduced locally; all mutation sites enumerated by grep on the codebase; refactor target signature derived from existing production code shape)

---

## Summary

The phase 56 race is a classic Go testing anti-pattern: tests swap a package-level `var allChecks []Check` slice in/out using `defer func() { allChecks = original }()`, then call `t.Parallel()`. The `t.Parallel()` callers in the same package that READ `allChecks` (directly or via the `AllChecks()` accessor) then race against the swap writes. Running the existing test suite under `-race` confirms this immediately — every parallel test in the doctor package fails with a race report against `pkg/internal/doctor/check_test.go:57` (and lines 89, 121) as the writer.

`REQUIREMENTS.md` and `PROJECT.md` both name `check_test.go` and `socket_test.go` as the race sites. **This is partially incorrect — verified by `grep -rn "allChecks =" pkg/internal/doctor/`**. The actual mutators are `check_test.go` (3 parallel tests) and `hostmount_test.go::TestSetMountPaths` (1 NON-parallel test that deliberately mutates the global to exercise the public `SetMountPaths` function). `socket_test.go` only READS via `AllChecks()` under `t.Parallel()` — it is a victim, not a mutator. The fix must touch `check_test.go` to eliminate the race; whether `hostmount_test.go::TestSetMountPaths` also needs touching is a design choice (see Open Questions).

The roadmap-locked fix is **dependency injection by parameter**: extract the body of `RunAllChecks()` into an unexported `runChecks(checks []Check) []Result` helper. `RunAllChecks()` becomes a one-liner that calls `runChecks(allChecks)`. Tests that need to substitute fake checks call `runChecks([]Check{mockA, mockB})` directly with a local slice — no package-state mutation, no `t.Parallel()` ordering hazard. The production read path remains a plain slice iteration with zero synchronisation. This is the minimal-invasive fix the roadmap demands (SC2: no `sync.RWMutex` on the read path) and it is structurally identical to the `sync.OnceValues`/parameter-injection patterns already used in `pkg/cluster/internal/create/actions/action.go` for race-free shared state.

**Primary recommendation:** ONE plan with three tasks. Task 1: refactor `check.go` to extract `runChecks(checks []Check) []Result` and have `RunAllChecks()` delegate to it (zero behaviour change verified by existing test suite passing without `-race`). Task 2: rewrite the three `check_test.go` tests (lines 45, 83, 115) to call `runChecks([]Check{...})` instead of swapping `allChecks`; remove the save/restore `defer`s. Task 3: add the race verification target — `go test -race ./pkg/internal/doctor/... -count=100` runs clean — and gate it via a new `Makefile` target `test-race-doctor` and a CI job (or extend the existing `test-race` target). Leave `TestSetMountPaths` (`hostmount_test.go:339`) alone — it is intentionally non-parallel and tests the global-mutation path of `SetMountPaths`, which is a public API; refactoring it requires changing the public surface and is out of scope.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Hold canonical check registry (init-time, never mutated) | `var allChecks []Check` in `check.go` | — | Package-level var initialized at load; no runtime registration; no mutex needed because never written after init |
| Execute checks against a registry | Unexported `runChecks(checks []Check) []Result` helper | — | Pure function taking slice as parameter; test-friendly via parameter injection |
| Public production entry point | `RunAllChecks() []Result` calling `runChecks(allChecks)` | — | Preserves existing public API; one-liner; behavior unchanged |
| Public mount-path injection | `SetMountPaths(paths []string)` — still iterates `allChecks` | — | Out of scope: writes to internal check state, not to `allChecks` itself; remains as today |
| Race detector verification | `go test -race ./pkg/internal/doctor/... -count=100` | New `Makefile` target + optional CI job | 100-run threshold catches timing-dependent races (Pitfall 20); existing `test-race` target only covers `pkg/cluster/internal/create/...` |

The crucial insight: `allChecks` is **read-only after package init in production**. There is no mutex *because none is needed*. The race exists ONLY in test code because tests reach over the package wall and overwrite the variable. Fixing this at test scope (parameter injection) preserves the production lock-free invariant.

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DEBT-04 | Pre-existing data race in `pkg/internal/doctor/check_test.go` and `pkg/internal/doctor/socket_test.go` (`allChecks` global mutated under `t.Parallel()`) eliminated; production read path remains lock-free; tests use scoped helper `runChecks(checks []Check)` rather than mutating package state | Race reproduced locally this session (see Empirical Probe). Mutation sites enumerated by `grep -rn "allChecks =" pkg/internal/doctor/` — found in `check_test.go` (3 sites, all parallel — these ARE the race) and `hostmount_test.go` (1 site, explicitly non-parallel — NOT a race). `socket_test.go` does NOT mutate `allChecks` (REQUIREMENTS.md description is slightly inaccurate — see Open Question 1). The proposed `runChecks(checks []Check) []Result` helper is a 5-line refactor of `RunAllChecks` that preserves the production read path lock-free and lets tests inject local slices without touching package state. |

---

## Empirical Probe (executed during research)

**Goal:** Confirm the race exists at current HEAD and identify the exact write/read line numbers.

```
$ go version
go version go1.26.3 darwin/arm64

$ CGO_ENABLED=1 go test -race -run 'TestRunAllChecks|TestAllChecks|TestDockerSocketCheck' \
    ./pkg/internal/doctor/... -count=5
==================
WARNING: DATA RACE
Read at 0x000105190120 by goroutine 101:
  sigs.k8s.io/kind/pkg/internal/doctor.TestAllChecks_IncludesHAResumeStrategy()
      /Users/patrykattc/work/git/kinder/pkg/internal/doctor/check_test.go:255 +0x38

Previous write at 0x000105190120 by goroutine 96:
  sigs.k8s.io/kind/pkg/internal/doctor.TestRunAllChecks_PlatformSkip()
      /Users/patrykattc/work/git/kinder/pkg/internal/doctor/check_test.go:57 +0x328
==================
FAIL    sigs.k8s.io/kind/pkg/internal/doctor    0.399s
```

**100-run baseline (current HEAD):**

```
$ CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100 -timeout=300s
FAIL    sigs.k8s.io/kind/pkg/internal/doctor    1.603s
(multiple race reports from check_test.go:57, :89, :121 as writers;
 collateral failures across all parallel tests in the package)
```

[VERIFIED: local probe, this session, commit `f033158a` on `main`]

The race is reliably reproducible with `-count=5` — Pitfall 20's "100-run threshold" guards against the inverse (post-fix verification that the race is truly gone), not the discovery of an existing race.

---

## Standard Stack

This phase introduces no new libraries — it is pure refactor + verification.

### Core (already in use)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go `testing` stdlib | go1.25.7 (`.go-version`) | Run unit tests, `t.Parallel()`, `t.Cleanup` | Stdlib — no alternative for Go test infrastructure |
| Go race detector | shipped with Go toolchain | Detect data races at runtime via `-race` flag | Stdlib — the canonical Go race-detection tool |
| `gotestsum` | latest (built from `hack/tools`) | JUnit XML output for CI | Already used by `hack/make-rules/test.sh` |

### Supporting (no additions)

No new dependencies. The fix is a refactor inside one Go package (`pkg/internal/doctor`) plus an optional `Makefile` target.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `runChecks(checks []Check) []Result` parameter-injection helper | Add `sync.RWMutex` to `allChecks` read path | Explicitly forbidden by SC2 ("Production `check.go` does NOT add `sync.RWMutex`"). Also serializes doctor execution (Pitfall 20 warning). |
| Parameter-injection helper | Replace `allChecks` with `sync.OnceValues` | `sync.OnceValues` solves lazy-init races, not mutation races. The registry is already init-time-immutable in production; `OnceValues` adds complexity without fixing the test-only race. Pitfall 20 mentions it for lazy-init only. |
| Parameter-injection helper | Convert `allChecks` to a private field on a new `Registry` struct with `t.Cleanup` swap helpers | Larger API surface change; touches every check constructor and every caller. SC2 wants the fix "confined to test scope" — parameter-injection achieves that with one new unexported function. |
| Rewrite all three racing tests to call `runChecks(localSlice)` directly | Add a test helper `swapAllChecks(t, replacement) (restore func())` that uses `t.Cleanup` | The helper still mutates the global. Even with `t.Cleanup`, parallel reads in OTHER tests race against the swap. Pitfall 20's own remediation text suggests `t.Cleanup` — but that's incorrect for parallel tests. **Parameter-injection is the only correct fix.** |
| `go test -race -count=100` as the verification | `go test -race -count=10` (Pitfall 20 example) | SC1 mandates 100. Pitfall 20 prose says 10 but ROADMAP/SC1 overrides — use 100. The cost is ~10× a single race-run; on this package that's < 30s wall-clock (race-tests run in ~1.6s × 100 single-process invocations, but `-count=100` reuses one process so total is ~5s). |

---

## Architecture Patterns

### System Architecture Diagram

```
Production runtime path (UNCHANGED by this phase):

  kinder doctor CLI
       │
       ▼
  doctor.runE()  ── pkg/cmd/kind/doctor/doctor.go:74
       │
       │ optionally calls
       ▼
  doctor.SetMountPaths(paths)  ── iterates allChecks, calls setMountPaths on configurable checks
       │
       ▼
  doctor.RunAllChecks()  ── one-liner after refactor: return runChecks(allChecks)
       │
       ▼
  runChecks(checks []Check)  ── NEW unexported helper holding the existing loop body
       │
       ▼
  []Result  ── ordered, platform-filtered

Test paths AFTER refactor:

  TestRunAllChecks_PlatformSkip / NilPlatformsRunsOnAll / MultipleResultsPreserved
       │ (no longer touches the allChecks global)
       │
       ▼
  Build local []Check{...mockCheck...}
       │
       ▼
  runChecks(local)  ── direct call; no save/restore; safe under t.Parallel()
       │
       ▼
  Assertions on []Result

  All other tests (TestAllChecks_ReturnsNonNilSlice, TestAllChecks_IncludesIPAMProbe,
  TestAllChecks_CountIs26, TestAllChecks_Registry, TestAllChecks_RegisteredOrder,
  TestRegistry_ContainsResumeReadiness, TestAllChecks_IncludesHAResumeStrategy)
       │
       ▼
  AllChecks()  ── pure read of the immutable package-init slice → race-free
```

### Recommended Implementation Layout

```
pkg/internal/doctor/
├── check.go              # +5 lines: extract runChecks(checks); RunAllChecks → one-liner
├── check_test.go         # ~30 lines changed: 3 tests refactored to call runChecks(local)
├── socket_test.go        # NO CHANGE (only reads via AllChecks())
├── hostmount_test.go     # NO CHANGE — TestSetMountPaths intentionally non-parallel
└── (all other test files) # NO CHANGE
```

Plus optionally:
```
Makefile                  # +3 lines: new `test-race-doctor` target (or extend existing test-race)
.github/workflows/        # optional new job — see Open Question 3
```

### Pattern 1: Extract-and-delegate refactor for parameter injection

**What:** Move the body of `RunAllChecks` into a new unexported function that takes the registry as a parameter. The public function becomes a thin delegate.

**When to use:** Any time tests need to substitute the data a function operates on, and that data is currently held in a package-level variable that tests are tempted to swap.

**Before** (`check.go:117-133` today):
```go
// RunAllChecks executes all checks with centralized platform filtering.
// Returns ordered results preserving insertion order from the registry.
func RunAllChecks() []Result {
    var results []Result
    for _, check := range AllChecks() {
        platforms := check.Platforms()
        if len(platforms) > 0 && !containsString(platforms, runtime.GOOS) {
            results = append(results, Result{
                Name:     check.Name(),
                Category: check.Category(),
                Status:   "skip",
                Message:  platformSkipMessage(platforms),
            })
            continue
        }
        results = append(results, check.Run()...)
    }
    return results
}
```

**After** (proposed):
```go
// RunAllChecks executes all registered checks with centralized platform filtering.
// Returns ordered results preserving insertion order from the registry.
//
// This is a thin delegate over runChecks(allChecks) so tests can exercise the
// same execution logic against a locally-constructed []Check slice without
// mutating package state (see runChecks docstring).
func RunAllChecks() []Result {
    return runChecks(allChecks)
}

// runChecks executes the provided checks with centralized platform filtering.
// Returns ordered results preserving the input slice order. Pure function:
// reads only its argument, calls only Check methods, never touches package
// state. Test-friendly — pass a local []Check{...} to substitute checks
// without mutating the allChecks global (see DEBT-04, Phase 56).
func runChecks(checks []Check) []Result {
    var results []Result
    for _, check := range checks {
        platforms := check.Platforms()
        if len(platforms) > 0 && !containsString(platforms, runtime.GOOS) {
            results = append(results, Result{
                Name:     check.Name(),
                Category: check.Category(),
                Status:   "skip",
                Message:  platformSkipMessage(platforms),
            })
            continue
        }
        results = append(results, check.Run()...)
    }
    return results
}
```

**Behavior change:** Zero. `RunAllChecks()` returns the same `[]Result` for the same package-init slice. Verified by: existing test suite (minus the three soon-to-be-rewritten tests) passes without modification.

**Naming note:** `runChecks` is unexported. SC2 says "scoped helper" — keeping it private aligns with the intent that production callers go through `RunAllChecks()` and only tests reach for `runChecks` directly via in-package access.

### Pattern 2: Test rewrite — local slice, no swap

**What:** Replace `original := allChecks; defer ...; allChecks = []Check{...mocks...}; results := RunAllChecks()` with `results := runChecks([]Check{...mocks...})`.

**Before** (`check_test.go:45-81` — `TestRunAllChecks_PlatformSkip`):
```go
func TestRunAllChecks_PlatformSkip(t *testing.T) {
    t.Parallel()

    nonCurrentPlatform := "linux"
    if runtime.GOOS == "linux" {
        nonCurrentPlatform = "windows"
    }

    original := allChecks
    defer func() { allChecks = original }()

    allChecks = []Check{
        &mockCheck{ /* ... */ },
    }

    results := RunAllChecks()
    // assertions...
}
```

**After** (proposed):
```go
func TestRunAllChecks_PlatformSkip(t *testing.T) {
    t.Parallel()

    nonCurrentPlatform := "linux"
    if runtime.GOOS == "linux" {
        nonCurrentPlatform = "windows"
    }

    checks := []Check{
        &mockCheck{ /* ... */ },
    }

    results := runChecks(checks)
    // assertions unchanged...
}
```

**Net diff per test:** Remove 3 lines (save / defer / assign), change `RunAllChecks()` → `runChecks(checks)`, plus a one-line variable rename. Same applies to `TestRunAllChecks_NilPlatformsRunsOnAll` (line 83) and `TestRunAllChecks_MultipleResultsPreserved` (line 115).

### Anti-Patterns to Avoid

- **Adding `sync.RWMutex` to `allChecks` access**: Explicitly forbidden by SC2. Also creates the serialization regression Pitfall 20 warns about — if the lock is held for the duration of all check runs, `kinder doctor` becomes serially-blocking on every check, including I/O-bound ones like `docker info`.
- **Using `t.Cleanup(func() { allChecks = saved })`** as the fix: Pitfall 20's prose suggests this, but it's WRONG for parallel tests. `t.Cleanup` runs *after* the test, but the test's body still mutates the global *during* execution, racing against other `t.Parallel()` readers in the same package. **The pitfall doc itself is slightly off here — the ROADMAP SC2 phrasing is correct.**
- **Refactoring `TestSetMountPaths` to use `runChecks`**: It tests the `SetMountPaths` *public function*, which by design mutates `allChecks`-resident check state. The test must operate on the global to exercise the real code path. Leave it as `// Not parallel: manipulates global allChecks` — that comment is the correct mitigation for its specific case.
- **Replacing `allChecks` with a `sync.OnceValues` lazy-init**: The registry is already eagerly initialized at package load. Lazy init solves a different problem and would invite new complexity (what happens if a check constructor panics? what about init ordering between files?). Don't.
- **Bumping `-count` higher than 100**: SC1 says 100. Higher values inflate CI cost without changing the fail/pass verdict — if 100 doesn't catch the race, 1000 won't either (the race-detector's TSAN-style algorithm is not probability-based once schedule diversity is exhausted).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Detect race in `allChecks` swap | A custom race-detection harness or `runtime.SetMutexProfileFraction` instrumentation | `go test -race` (Go's built-in TSAN-backed race detector) | Built-in tool, integrated with `go test`, catches the exact pattern (write-then-read across goroutines), supported on linux/darwin/freebsd amd64+arm64 |
| Iterate while one writer / many readers | A `sync.RWMutex` wrapper around a private slice | Parameter-injection helper (`runChecks(checks []Check)`) | The production code does NOT need synchronization — it's init-then-read. Only tests touch the variable; passing a local slice avoids the problem entirely. |
| Save/restore the global registry around a test | A custom `swapAllChecks(t, replacement)` helper using `t.Cleanup` | Don't swap at all — call `runChecks(localSlice)` | Even with `t.Cleanup`, the swap is visible to any `t.Parallel()` reader scheduled during the test's run window. Avoidance > mitigation. |
| Time the doctor command to detect serialization regression | A custom benchmark harness | Manual `time kinder doctor` (sanity check) + visual diff of `check.go` (assert no mutex added) | SC3 ("kinder doctor command timing is unchanged") is structurally enforced by SC2 (no mutex). If SC2 is true, SC3 is automatically true. A benchmark is overkill — the refactor is `return runChecks(allChecks)`, a function call that the inliner will likely elide. |

**Key insight:** The Go race detector is the canonical verification mechanism for this class of bug. The roadmap's `-count=100` instruction is the canonical sampling rate. No custom tooling is appropriate.

---

## Runtime State Inventory

Not applicable — this is a Go test refactor with no stored data, no live service config, no OS-registered state, no secrets, and no build artifacts that persist across the change.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — refactor touches Go source only | None |
| Live service config | None — no service involved | None |
| OS-registered state | None — no daemons, schedulers, or installers | None |
| Secrets/env vars | None | None |
| Build artifacts | Coverage profiles (`bin/unit.cov`, `bin/unit-junit.xml`) regenerate on next `make unit` — no stale-artifact risk | None |

---

## Common Pitfalls

### Pitfall A: "Pitfall 20" prose suggests `t.Cleanup` — but that doesn't fix parallel races
**What goes wrong:** A planner reading PITFALLS.md sees Pitfall 20's "use `t.Cleanup(func() { allChecks = saved })`" advice and implements that. The race detector still fires because parallel readers race the cleanup write.
**Why it happens:** Pitfall 20's prose was written before the SC2 `runChecks(checks []Check)` decision crystallized. The ROADMAP SC2 phrasing supersedes the pitfall prose.
**How to avoid:** Use the parameter-injection pattern from SC2 (`runChecks(checks []Check)`). Do not use `t.Cleanup` for the three racing tests. (Other tests that need to mutate package state for non-race reasons — e.g. `TestSetMountPaths` — may still use `t.Cleanup` if they are also explicitly non-parallel; that's fine.)
**Warning signs:** Post-fix `go test -race -count=100` still reports a race; the PR diff contains a new `t.Cleanup(func() { allChecks = ... })` call.

### Pitfall B: Forgetting that `SetMountPaths` also iterates `allChecks`
**What goes wrong:** A refactor that removes `allChecks` entirely (e.g., privatizes it inside a struct) breaks `SetMountPaths` and its non-parallel test `TestSetMountPaths`.
**Why it happens:** `SetMountPaths` is a separate code path that also reads `allChecks` (check.go:103). It is not part of the race fix but is colocated.
**How to avoid:** Keep `allChecks` as-is (package-level `var`). The refactor extracts `runChecks(checks)` BESIDE it; `RunAllChecks()` keeps reading the same global. `SetMountPaths` unchanged. `TestSetMountPaths` unchanged.
**Warning signs:** Compilation failure in `SetMountPaths`; `TestSetMountPaths` start failing.

### Pitfall C: CGO_ENABLED=0 in CI breaks `-race`
**What goes wrong:** The Go race detector requires CGO. Workflows that set `CGO_ENABLED=0` (like the new `.github/workflows/build-check.yml`) cannot run `-race`.
**Why it happens:** TSAN (the race detector's backend) requires C runtime hooks. Without CGO, `go test -race` silently does nothing useful — and on some platforms it errors out.
**How to avoid:** Any CI job that runs `go test -race` must set `CGO_ENABLED=1` explicitly. The existing `Makefile` target `test-race` already does this (`CGO_ENABLED=1 go test -race ./pkg/cluster/internal/create/... -count=1`). Reuse that pattern.
**Warning signs:** CI run shows `go test -race` passing instantly with no race report — even though the race is known-present pre-fix. Suspect CGO_ENABLED=0 silently disabling the detector.

### Pitfall D: `-count=N` reuses one test binary; collateral race output is confusing
**What goes wrong:** When `check_test.go:57` writes to `allChecks`, it races against EVERY parallel test in the package — even ones that don't directly touch `allChecks`. The output shows races in `TestKubectlVersionSkewCheck_Run`, `TestDiskSpaceCheck_Run`, etc., which is misleading.
**Why it happens:** Go's race detector reports the GOROUTINE that holds the read at the time of the racing write. Any goroutine that did a transitive read of `allChecks` via `AllChecks()` is implicated.
**How to avoid:** After the fix, expect EVERY race report to disappear — not just the three originally-named tests. The fix is total (no swap = no race anywhere).
**Warning signs:** Post-fix run shows races in unrelated tests — investigate, there's likely a leftover swap somewhere.

### Pitfall E: REQUIREMENTS/PROJECT name `socket_test.go` but it doesn't mutate `allChecks`
**What goes wrong:** A planner trusts the REQUIREMENTS.md and PROJECT.md "check_test.go and socket_test.go" naming and edits `socket_test.go` looking for swaps that aren't there.
**Why it happens:** REQUIREMENTS.md and PROJECT.md were written from a high-level race-symptom view (any parallel test reading `AllChecks()` is implicated by the race detector) rather than a write-site view.
**How to avoid:** The TRUE mutation sites are in `check_test.go` only (lines 54, 86, 118 today). `socket_test.go` is a victim reader, not a mutator. The fix touches `check.go` (add `runChecks`) and `check_test.go` (rewrite 3 tests). `socket_test.go` needs no changes once the writers are eliminated.
**Warning signs:** Plan tasks named "edit socket_test.go" without a clear write site to change.

### Pitfall F: 100-run threshold may exhaust short test runs but is still cheap
**What goes wrong:** `-count=100` on the full doctor package is suspected to be expensive in CI minutes.
**Why it happens:** Operator caution.
**How to avoid:** Measure. The doctor package's unit tests complete in ~0.3s without `-race` and ~1.6s with `-race` for one run; `-count=100` reuses one process so the total is ~5–10s wall-clock locally. Acceptable for CI.
**Warning signs:** CI timeouts on the new race job — would indicate a real regression, not the `-count=100` overhead.

---

## Code Examples

### Example 1: The new `check.go` (full diff)

**File:** `pkg/internal/doctor/check.go`

Replace lines 115–133 with:

```go
// RunAllChecks executes all registered checks with centralized platform filtering.
// Returns ordered results preserving insertion order from the package-level
// allChecks registry.
//
// This is a thin delegate over runChecks(allChecks) so tests can exercise the
// same execution logic against a locally-constructed []Check slice without
// mutating package state (see DEBT-04 / Phase 56).
func RunAllChecks() []Result {
    return runChecks(allChecks)
}

// runChecks executes the provided checks with centralized platform filtering.
// Returns ordered results preserving the input slice order.
//
// runChecks is a pure function over its argument — it reads only the passed
// slice, calls only Check methods, and never touches package state. Tests pass
// a local []Check{...} to substitute mocks without mutating the allChecks
// global, eliminating the race that t.Parallel() callers would otherwise hit.
func runChecks(checks []Check) []Result {
    var results []Result
    for _, check := range checks {
        platforms := check.Platforms()
        if len(platforms) > 0 && !containsString(platforms, runtime.GOOS) {
            results = append(results, Result{
                Name:     check.Name(),
                Category: check.Category(),
                Status:   "skip",
                Message:  platformSkipMessage(platforms),
            })
            continue
        }
        results = append(results, check.Run()...)
    }
    return results
}
```

### Example 2: The new `check_test.go` test bodies

Each of `TestRunAllChecks_PlatformSkip` (line 45), `TestRunAllChecks_NilPlatformsRunsOnAll` (line 83), `TestRunAllChecks_MultipleResultsPreserved` (line 115) gets the same surgical change:

```go
func TestRunAllChecks_PlatformSkip(t *testing.T) {
    t.Parallel()

    nonCurrentPlatform := "linux"
    if runtime.GOOS == "linux" {
        nonCurrentPlatform = "windows"
    }

    // No more save/restore of allChecks — pass a local slice to runChecks instead.
    checks := []Check{
        &mockCheck{
            name:      "platform-specific",
            category:  "Test",
            platforms: []string{nonCurrentPlatform},
            results: []Result{{
                Name:     "platform-specific",
                Category: "Test",
                Status:   "ok",
                Message:  "should not appear",
            }},
        },
    }

    results := runChecks(checks)
    if len(results) != 1 {
        t.Fatalf("expected 1 result, got %d", len(results))
    }
    if results[0].Status != "skip" {
        t.Errorf("expected skip status, got %q", results[0].Status)
    }
    if results[0].Name != "platform-specific" {
        t.Errorf("expected name 'platform-specific', got %q", results[0].Name)
    }
}
```

Note: the test function name keeps `RunAllChecks_PlatformSkip` even though it now calls `runChecks` directly. That's intentional — the test exercises the same execution logic (which is now in `runChecks`), and the public surface (`RunAllChecks`) is just a one-line delegate. Renaming to `TestRunChecks_PlatformSkip` would also be reasonable (Open Question 4).

### Example 3: New `Makefile` target

Append to `Makefile` (after current `test-race` target at line 88):

```make
# race detector for doctor package — 100-run threshold per Phase 56 SC1 (DEBT-04)
test-race-doctor:
	CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100
```

And add `test-race-doctor` to the `.PHONY` line at line 127.

### Example 4 (optional, see Open Question 3): CI workflow snippet

If a CI job is added:

```yaml
# .github/workflows/race-check.yml (or add as a job in build-check.yml)
name: Race Check
on:
  pull_request:
    branches: [main]
    paths:
      - 'pkg/internal/doctor/**'
      - '.github/workflows/race-check.yml'
permissions:
  contents: read
jobs:
  doctor-race:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
      - uses: actions/setup-go@7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5 # v6.2.0
        with:
          go-version-file: .go-version
      - name: Run doctor race tests (100 iterations)
        env:
          CGO_ENABLED: "1"
        run: go test -race ./pkg/internal/doctor/... -count=100
```

(Uses the exact same action SHA pins as `build-check.yml` from Phase 55 — match for consistency.)

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Test mutates a package-level slice and restores via `defer`; expects `t.Parallel()` co-tests to be lucky | Pass the slice as a parameter; never mutate | Standard Go testing wisdom since `t.Parallel` existed (Go 1.0); `go vet` and `-race` make the bug findable from Go 1.1+ | Eliminates the entire class of test-only races |
| Wrap registry reads in a mutex to "fix" a test race | Refactor the test to not mutate the registry | Modern Go consensus (e.g., uber-go/guide, Google Go style guide) | Production code stays lock-free; test isolation improves |
| Use `t.Cleanup` to restore globals around a test | Use parameter injection for parallel-test cases; `t.Cleanup` only for sequential cases | `t.Cleanup` added in Go 1.14 (2020); still doesn't help with concurrent reads | The pitfall: `t.Cleanup` looks like a fix but isn't, for `t.Parallel()` |

**Deprecated/outdated:**
- The "save / overwrite / defer-restore" idiom for testable globals — superseded by parameter injection or constructor injection in modern Go.
- Pitfall 20's `t.Cleanup` advice in `.planning/research/PITFALLS.md` — superseded by ROADMAP SC2 (`runChecks(checks []Check)` parameter injection). The pitfall doc should be considered for a corrective edit in a future cleanup phase, but that is out of scope for Phase 56.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `runChecks` should be unexported (lowercase) | Pattern 1, Example 1 | Could be exported for external test packages, but the doctor package has no `*_test` external-package tests today; unexported matches the "scoped helper" wording in SC2. If wrong, just capitalize and re-export — trivial diff. [ASSUMED based on SC2 wording "scoped"] |
| A2 | `TestSetMountPaths` should NOT be migrated | Summary, Anti-Patterns | If the planner decides to also refactor `SetMountPaths` to take a `[]Check` parameter (out of scope but conceivable), this assumption changes. Risk: low — `TestSetMountPaths` is non-parallel and not part of the race; rewriting it is unnecessary churn. [ASSUMED based on minimal-change principle] |
| A3 | CI cost of `-count=100 -race` is < 30s wall-clock and acceptable | Pitfall F, Open Question 3 | If CI is dramatically slower than local (race detector on ARM64 macOS measured ~1.6s × 100 runs amortized in one process), the count could be reduced. Risk: low — measured locally at ~5–10s; CI runner perf is comparable. [ASSUMED based on local measurement; CI not yet measured] |
| A4 | No other `*_test.go` file in the package mutates `allChecks` beyond the three found | Mutation Site Audit | Risk: very low — `grep -rn "allChecks =" pkg/internal/doctor/` is an exhaustive search for assignment writes; no dynamic or reflection-based mutation patterns exist in Go test code style here. [VERIFIED via grep] |
| A5 | Pitfall 20's `t.Cleanup` advice is superseded by ROADMAP SC2 | Pitfall A, State of the Art | Risk: low — ROADMAP is the authoritative spec; PITFALLS.md is reference. If the user disagrees, a discuss-phase round can reconcile. [ASSUMED based on document precedence convention in this repo] |

---

## Open Questions

1. **`socket_test.go` mention in REQUIREMENTS/PROJECT is inaccurate — does the planner need to acknowledge this?**
   - What we know: `socket_test.go` reads via `AllChecks()` under `t.Parallel()` but does not WRITE to `allChecks`. The race writer is exclusively in `check_test.go`.
   - What's unclear: Should the plan SUMMARY note that REQUIREMENTS.md is slightly imprecise (and propose a doc-correction follow-up), or silently fix the actual writers without commenting?
   - Recommendation: Plan SUMMARY notes "REQUIREMENTS.md mentions `socket_test.go` but the actual mutation sites are in `check_test.go` only; this plan fixes the mutation sites and leaves `socket_test.go` untouched". REQUIREMENTS.md correction is documentation drift, not blocking.

2. **Should `TestSetMountPaths` in `hostmount_test.go` also migrate, even though it's already non-parallel?**
   - What we know: It mutates `allChecks` deliberately to test the public `SetMountPaths` function. It explicitly opts out of `t.Parallel()`. It's not currently racy.
   - What's unclear: There's a cosmetic value in eliminating ALL `allChecks =` writes for grep-cleanliness. But there's no functional reason to touch it.
   - Recommendation: Leave it. The comment `// Not parallel: manipulates global allChecks` is the right encoded mitigation. If a future planner wants total cleanliness, that's a separate cleanup phase. SC2's "fix confined to test scope" is satisfied by touching just `check_test.go`.

3. **Should a CI job be added, or is `Makefile` target + manual run enough?**
   - What we know: The repo has no existing CI job that runs `-race` on the doctor package; `Makefile`'s `test-race` only covers `pkg/cluster/internal/create/...`. The phase 55 pattern (one workflow file, ubuntu-24.04, SHA-pinned actions) sets the convention.
   - What's unclear: SC1 says `go test -race ./pkg/internal/doctor/... -count=100` must pass, but doesn't say WHERE it must run. Options: (a) Makefile target + verifier-runs-it-once approach; (b) new CI workflow file matching `build-check.yml` style; (c) extend existing `test-race` Makefile target to include doctor and let upstream `make test` plumb it.
   - Recommendation: Add the Makefile target (cheap, runnable locally for any future developer) AND a small CI workflow (~30 lines, matches phase 55 style). Both. Cost is low, and the CI job ensures regressions don't reintroduce the race. If the planner prefers minimal scope, the Makefile target alone is sufficient for SC1 verification (verifier runs it once during `/gsd-verify-work`).

4. **Should the three rewritten tests be renamed?**
   - What we know: Today they're `TestRunAllChecks_PlatformSkip`, `_NilPlatformsRunsOnAll`, `_MultipleResultsPreserved` and they call `RunAllChecks()`. After refactor, they call `runChecks(local)`.
   - What's unclear: `TestRunAllChecks_*` suggests they test the public function, but they actually test the inner helper.
   - Recommendation: Keep the names. The behavior they verify (platform skip, nil-platforms-runs-on-all, multiple-results-preserved) is shared by `RunAllChecks` and `runChecks` since the public function is now a one-line delegate. Renaming to `TestRunChecks_*` would also be defensible but is bikeshedding — not required by SC.

5. **Does `Makefile`'s existing `test-race` target need to be widened to include `./...` instead of just `./pkg/cluster/internal/create/...`?**
   - What we know: The existing `test-race` target is narrow — only the create path. Widening to `./...` would be safer (catch races anywhere) but expensive (likely takes minutes; many test packages don't yet pass `-race`).
   - What's unclear: Is widening in scope for Phase 56?
   - Recommendation: NO — out of scope. Phase 56 is "DEBT-04 doctor test race fix", not "all-package race coverage". Add a NEW target `test-race-doctor` to keep the change tightly scoped. Widening `test-race` is a separate hardening initiative for v2.5+.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All build + test work | YES | 1.26.3 on dev, 1.25.7 from `.go-version` for CI | none |
| `CGO_ENABLED=1` | Race detector (`go test -race`) | YES on darwin/arm64 (verified locally), YES on ubuntu-24.04 (CI default has gcc) | n/a | none — `-race` requires CGO |
| `gotestsum` | `hack/make-rules/test.sh` (existing CI test driver) | YES (built from `hack/tools/`) | latest from `go.mod` | direct `go test` if absent |
| `grep`, `make` | Local verification | YES (standard Unix tools) | n/a | none |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** None.

---

## Validation Architecture

> Phase 56 falls under standard `workflow.nyquist_validation` test mapping (no opt-out in config).

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` stdlib (go1.25.7 per `.go-version`) |
| Config file | None (Go convention: `*_test.go` files in same package) |
| Quick run command | `go test ./pkg/internal/doctor/... -count=1` |
| Race-detector run command | `CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100` |
| Full suite command | `make test` (delegates to `hack/make-rules/test.sh` → `gotestsum` over `./...`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DEBT-04 / SC1 | Race detector reports zero races over 100 iterations | race-detector | `CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100` | ✅ existing tests; this command currently FAILS — fix makes it PASS |
| DEBT-04 / SC2 | Production `check.go` has no `sync.RWMutex` on the `allChecks` read path | static review / grep | `! grep -n "sync\.\(RW\)\?Mutex" pkg/internal/doctor/check.go` (must return non-zero exit) | ✅ structural — grep |
| DEBT-04 / SC3 | `kinder doctor` command timing unchanged | structural | Same as SC2 — if no mutex was added, no serialization regression is possible. Optional sanity: `time ./bin/kinder doctor` before/after refactor. | ✅ structural — SC2 implies SC3 |

### Sampling Rate
- **Per task commit:** `go test ./pkg/internal/doctor/... -count=1` (~0.3s — fast feedback that refactor compiles and unit semantics intact)
- **Per task verification (SC1 gate):** `CGO_ENABLED=1 go test -race ./pkg/internal/doctor/... -count=100` (~5–10s wall-clock locally)
- **Per wave merge:** Full `make test` to confirm no regression in adjacent packages
- **Phase gate:** All three SCs verified by the verifier before `/gsd-verify-work` approval

### Wave 0 Gaps

None. The existing test infrastructure (`pkg/internal/doctor/check_test.go`, `socket_test.go`, `hostmount_test.go`, etc.) is already in place; the phase rewrites three existing tests in `check_test.go` and adds zero new test files.

If a CI workflow is added (Open Question 3), that's a NEW file (`.github/workflows/race-check.yml` or a job inside an existing workflow) — but it's a verification harness, not a test file, and uses already-installed CI tooling (`actions/checkout`, `actions/setup-go`).

---

## Security Domain

Phase 56 is a test-only refactor with no security surface. ASVS categories V2 (Auth), V3 (Session), V4 (Access Control), V5 (Input Validation), V6 (Cryptography) all answer "no" — the change does not touch any of these areas.

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | n/a — no auth code touched |
| V3 Session Management | no | n/a — no session state |
| V4 Access Control | no | n/a — no permission boundaries |
| V5 Input Validation | no | n/a — no inputs added/changed |
| V6 Cryptography | no | n/a — no crypto |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Data race causing flaky CI / hidden test gaps | n/a (correctness, not security) | `go test -race` — addressed by this phase |

No security concerns are introduced or affected by this phase.

---

## Mutation Site Audit (complete enumeration)

Full result of `grep -rn "allChecks =\|allChecks =" pkg/internal/doctor/`:

```
pkg/internal/doctor/check.go:53      var allChecks = []Check{         # the canonical init
pkg/internal/doctor/check_test.go:55     defer func() { allChecks = original }()  # RACE - parallel
pkg/internal/doctor/check_test.go:57     allChecks = []Check{                      # RACE - parallel
pkg/internal/doctor/check_test.go:87     defer func() { allChecks = original }()  # RACE - parallel
pkg/internal/doctor/check_test.go:89     allChecks = []Check{                      # RACE - parallel
pkg/internal/doctor/check_test.go:119    defer func() { allChecks = original }()  # RACE - parallel
pkg/internal/doctor/check_test.go:121    allChecks = []Check{                      # RACE - parallel
pkg/internal/doctor/hostmount_test.go:344 defer func() { allChecks = original }()  # NOT parallel
pkg/internal/doctor/hostmount_test.go:364 allChecks = []Check{hostCheck, ddCheck}  # NOT parallel
```

**Verdict:** Exactly 3 test functions mutate `allChecks` under `t.Parallel()`. All are in `check_test.go` at lines 45, 83, 115 (`TestRunAllChecks_PlatformSkip`, `_NilPlatformsRunsOnAll`, `_MultipleResultsPreserved`). The fourth mutator (`hostmount_test.go::TestSetMountPaths`) is correctly marked non-parallel and is not part of the race.

Read-only callers of `AllChecks()` that are themselves `t.Parallel()` (i.e., the victims):
- `check_test.go:38` `TestAllChecks_ReturnsNonNilSlice`
- `check_test.go:219` `TestAllChecks_IncludesIPAMProbe`
- `check_test.go:243` `TestAllChecks_CountIs26`
- `check_test.go:254` `TestAllChecks_IncludesHAResumeStrategy`
- `socket_test.go:160` `TestAllChecks_Registry`
- `gpu_test.go:289` `TestAllChecks_RegisteredOrder`
- `resumereadiness_test.go:507` `TestRegistry_ContainsResumeReadiness`

After the fix, ALL of these become unconditionally race-free because no writer remains.

---

## Sources

### Primary (HIGH confidence)
- `pkg/internal/doctor/check.go` (read this session — current production code) — defines `allChecks`, `AllChecks()`, `RunAllChecks()`, `SetMountPaths()`, `mountPathConfigurable` interface
- `pkg/internal/doctor/check_test.go` (read this session — current test code) — three races at lines 54, 86, 118
- `pkg/internal/doctor/socket_test.go` (read this session) — confirms NO mutation, only reads via `AllChecks()`
- `pkg/internal/doctor/hostmount_test.go` (read this session — lines 337–395) — confirms `TestSetMountPaths` is explicitly non-parallel
- `pkg/cmd/kind/doctor/doctor.go` (read this session) — confirms production call path: `RunAllChecks()` called once, single-threaded, from cobra `RunE`
- `.planning/ROADMAP.md` Phase 56 section (lines 186–197) — SC1/SC2/SC3 verbatim
- `.planning/REQUIREMENTS.md` DEBT-04 entry (line 33)
- `.planning/research/PITFALLS.md` Pitfall 20 (full text reviewed)
- Local race-detector probe (executed this session, go1.26.3 darwin/arm64) — confirms race reproducible on current HEAD

### Secondary (MEDIUM confidence)
- `pkg/cluster/internal/create/actions/action.go:63` (read this session) — `sync.OnceValues` precedent in repo (referenced in Pitfall 20 prose), confirming repo's pattern for race-free shared state
- `Makefile` lines 88–89 — existing `test-race` target pattern (`CGO_ENABLED=1 go test -race ./pkg/cluster/internal/create/... -count=1`); template for new `test-race-doctor` target
- `.github/workflows/build-check.yml` (referenced from Phase 55) — workflow style template for any new race-check workflow

### Tertiary (LOW confidence)
- None — every claim in this RESEARCH is backed by a code read, a grep, or a measured local probe in this session.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new deps; Go stdlib + existing `gotestsum`/Make tooling
- Architecture: HIGH — `runChecks(checks []Check)` pattern is mechanical; current `RunAllChecks` body lifts unchanged
- Pitfalls: HIGH — race reproduced locally; all six pitfalls anchored to concrete evidence in this session
- Mutation site enumeration: HIGH — `grep -rn "allChecks =" pkg/internal/doctor/` is exhaustive for Go source

**Research date:** 2026-05-12
**Valid until:** 2026-06-12 (30 days; stable refactor, no fast-moving deps). If `pkg/internal/doctor/` gains new files or new check registrations between research and plan execution, re-run the `grep -rn "allChecks ="` enumeration to confirm no new mutation sites.
