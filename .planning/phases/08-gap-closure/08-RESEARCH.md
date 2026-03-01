# Phase 8: Gap Closure - Research

**Researched:** 2026-03-01
**Domain:** Go internal config defaulting, kubectl context targeting in bash scripts, table-driven unit tests
**Confidence:** HIGH

---

## Summary

Phase 8 closes two concrete tech debt items identified by the v1.0 milestone audit. Both items are small, well-bounded changes to existing code — no new architecture or dependencies are introduced.

**Item 1 (Bug fix):** The internal `SetDefaultsCluster` function in `pkg/internal/apis/config/default.go` (lines 95-101) contains an all-or-nothing safety guard that fires when all five addon booleans in the internal `Addons` struct are `false`. This guard was designed for library callers who never run the v1alpha4 conversion. However, it also fires when a user writes `addons: { metalLB: false, envoyGateway: false, metricsServer: false, coreDNSTuning: false, dashboard: false }` in their YAML config. The root cause: `V1Alpha4ToInternal` correctly converts five explicit `false` pointers to five `false` bools via `boolVal`, but `fixupOptions` then calls `config.SetDefaultsCluster` on the already-converted struct, which sees all-false and overrides everything to `true`. The fix is to add a sentinel field — or more simply, to delete the guard entirely and rely on `v1alpha4.SetDefaultsCluster` (which uses nil-pointer detection, not zero-value detection) as the correct defaulting layer.

**Item 2 (Script hardening):** `hack/verify-integration.sh` creates a cluster named `kinder-integration-test` but never sets the kubectl context. All subsequent `kubectl` commands use whatever context is currently active in `~/.kube/config`. If the developer has a pre-existing cluster active, the script's pod-readiness checks, `kubectl top`, HPA, DNS, and port-forward tests all run against the wrong cluster. The fix is a single `kubectl config use-context "kind-kinder-integration-test"` line after cluster creation, plus `--context` flags or the single use-context call is sufficient.

Both fixes are low-risk. Neither requires new dependencies, new packages, or new types. The only new code is: one modified Go function, one new Go test file, and three added lines to a bash script.

**Primary recommendation:** Delete the all-false guard from `pkg/internal/apis/config/default.go` lines 95-101, add a `TestSetDefaultsCluster_AllAddonsDisabled` unit test in the same package, and add `kubectl config use-context "kind-kinder-integration-test"` to `hack/verify-integration.sh` immediately after the `kinder create cluster` call.

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| FOUND-04 | Each addon action checks its enable flag before executing | Already implemented correctly in `create.go` runAddon closure (lines 180-191); the bug is upstream in SetDefaultsCluster which overrides the flag before runAddon sees it. Fix SetDefaultsCluster to not stomp explicit-false values. |
</phase_requirements>

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `testing` | Go 1.17+ | Unit test framework | Already used in all test files across the project |
| `pkg/internal/apis/config` package | internal | Config defaulting under test | Direct — this is the package being fixed |
| bash (POSIX) | system | Integration script modification | Same shell used in `hack/` already |
| `kubectl` | any compatible | Context targeting after cluster creation | Already required and used throughout verify-integration.sh |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `pkg/internal/assert` | internal | Test assertions | Already used in `validate_test.go` and `cluster_util_test.go` in the same package |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Deleting the guard | Adding a sentinel `AddonsExplicitlySet bool` field to `Addons` struct | Sentinel approach preserves the guard logic but requires touching types.go, zz_generated.deepcopy.go, convert_v1alpha4.go, and the v1alpha4 types. Too invasive for an edge case fix. Deletion is correct and simpler. |
| `kubectl config use-context` | Passing `--context` to every `kubectl` call in the script | Per-call `--context` is more explicit but requires touching ~20 lines. Single `use-context` call after cluster creation is simpler and achieves the same effect. |

**Installation:** No new Go dependencies. No new tools.

---

## Architecture Patterns

### The All-False Guard Problem — Precise Diagnosis

The call chain for user config loading:

```
user YAML file
  -> encoding.Parse()
     -> yaml.Unmarshal into v1alpha4.Cluster
     -> v1alpha4.SetDefaultsCluster(cluster)   // sets nil *bool pointers to &true; respects explicit false
     -> config.Convertv1alpha4(cluster)          // boolVal(*bool): nil->true, &false->false, &true->true
     -> returns internal config.Cluster{Addons: {false,false,false,false,false}}  // when user sets all false
  <- encoding.Load() returns cfg

fixupOptions(opts):
  -> opts.Config = cfg    // all-false Addons struct
  -> config.SetDefaultsCluster(opts.Config)   // BUG: sees all-false, overrides to all-true
```

The bug: `config.SetDefaultsCluster` at the internal level cannot distinguish between:
- A newly-constructed zero-value `Addons{}` struct (library caller who skipped conversion)
- A fully-converted struct where the user explicitly set all five to `false`

Both look identical at the Go level (five `false` bools).

### The Correct Fix — Remove the Guard

The internal `SetDefaultsCluster` is only called in two places:

1. `pkg/internal/apis/config/encoding/load.go` line 32 — for the `path == ""` case (no config file, pure defaults). At this point `Addons{}` is a true zero-value and defaulting to all-true is correct.
2. `pkg/cluster/internal/create/create.go` line 289 — for the already-converted cluster. At this point Addons was correctly populated by `Convertv1alpha4`.

**Approach A (recommended):** Remove the all-false guard entirely from `config.SetDefaultsCluster`. The `path == ""` case is handled by `encoding.Load()` calling `config.SetDefaultsCluster` on a fresh `&config.Cluster{}` — but since `encoding.Load("")` for the empty path returns a freshly defaulted config via `config.SetDefaultsCluster(out)`, removing the guard from `SetDefaultsCluster` means the empty-path case no longer gets defaulted addons either. This breaks the no-config-file default.

**Approach B (correct):** Move the all-false guard to `encoding.Load()` only, for the `path == ""` case. The internal `SetDefaultsCluster` should not contain it at all.

**Approach C (simplest, recommended):** Keep the guard in `SetDefaultsCluster` but change it to not override if the config came from an explicit user file. The way to do this: `fixupOptions` should only call `SetDefaultsCluster` if `opts.Config` was nil (i.e., we created it from defaults), not when it was loaded from a file. Looking at `fixupOptions`:

```go
func fixupOptions(opts *ClusterOptions) error {
    if opts.Config == nil {
        cfg, err := encoding.Load("")   // this already calls SetDefaultsCluster internally
        if err != nil {
            return err
        }
        opts.Config = cfg
    }
    // ... name/image overrides ...
    config.SetDefaultsCluster(opts.Config)  // THIS is the redundant call that causes the bug
    return nil
}
```

`encoding.Load("")` already calls `config.SetDefaultsCluster(out)` before returning. Then `fixupOptions` calls it again. For the file-loaded path, `encoding.Load(path)` calls `V1Alpha4ToInternal` which calls `v1alpha4.SetDefaultsCluster` (correct) and `config.Convertv1alpha4` (correct), but does NOT call internal `config.SetDefaultsCluster`. Then `fixupOptions` calls it — that's the bug.

**The cleanest fix:** Remove the `config.SetDefaultsCluster(opts.Config)` call from `fixupOptions`. The internal-level defaulting is already handled correctly in `encoding.Load("")` for the nil-config case. For the file-loaded path, `v1alpha4.SetDefaultsCluster` handles all defaulting correctly at the v1alpha4 layer before conversion.

**Alternative (also correct):** Keep `config.SetDefaultsCluster(opts.Config)` in `fixupOptions` but change the all-false guard to only fire when the Addons struct is a true zero-value AND no config was explicitly provided. This requires a sentinel — more complex.

**Final recommendation:** Remove lines 93-101 from `pkg/internal/apis/config/default.go` (the all-false guard), AND remove the `config.SetDefaultsCluster(opts.Config)` call from `fixupOptions` in `create.go`. The v1alpha4 defaulting layer already handles all cases correctly.

Wait — need to verify: if `opts.Config` was nil and `encoding.Load("")` is called, does `encoding.Load("")` invoke `config.SetDefaultsCluster`? Reading `load.go` line 31-33:

```go
if path == "" {
    out := &config.Cluster{}
    config.SetDefaultsCluster(out)
    return out, nil
}
```

Yes. So `encoding.Load("")` already calls `config.SetDefaultsCluster`. The second call in `fixupOptions` is redundant and harmful. Removing it solves the bug without needing to touch `SetDefaultsCluster` at all.

**Best fix path:**

1. Remove `config.SetDefaultsCluster(opts.Config)` from `fixupOptions` in `create.go` (line 289)
2. Also remove or narrow the all-false guard from `default.go` lines 93-101 (defensive cleanup so the guard cannot fire from any other call site)
3. Add unit tests in `pkg/internal/apis/config/` for the all-addons-disabled path

### Pattern 1: Table-Driven Unit Test for SetDefaultsCluster (Addons)

**What:** Add test cases to `cluster_util_test.go` (or a new `default_test.go`) in `pkg/internal/apis/config/` that verify the all-addons-disabled config path.

**When to use:** Testing the defaulting behavior without a live cluster.

```go
// Source: derived from existing validate_test.go pattern in pkg/internal/apis/config/
// File: pkg/internal/apis/config/default_test.go (new file)

package config

import (
    "testing"
)

func TestSetDefaultsCluster_AddonsNotOverridden(t *testing.T) {
    tests := []struct {
        name          string
        inputAddons   Addons
        wantAddons    Addons
    }{
        {
            name:        "all addons explicitly false remain false",
            inputAddons: Addons{
                MetalLB:       false,
                EnvoyGateway:  false,
                MetricsServer: false,
                CoreDNSTuning: false,
                Dashboard:     false,
            },
            wantAddons: Addons{
                MetalLB:       false,
                EnvoyGateway:  false,
                MetricsServer: false,
                CoreDNSTuning: false,
                Dashboard:     false,
            },
        },
        {
            name:        "zero-value addons (nil config case) default to true",
            inputAddons: Addons{},
            // After fix: zero-value is same as all-false; SetDefaultsCluster should NOT
            // be used to default addons -- the guard should be removed entirely.
            // For the nil-config path, encoding.Load("") handles it.
            // This test documents that SetDefaultsCluster no longer touches Addons.
            wantAddons: Addons{},
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            c := &Cluster{Addons: tc.inputAddons}
            SetDefaultsCluster(c)
            if c.Addons != tc.wantAddons {
                t.Errorf("SetDefaultsCluster() Addons = %+v, want %+v",
                    c.Addons, tc.wantAddons)
            }
        })
    }
}
```

**Important note for the planner:** After removing the guard from `SetDefaultsCluster`, the `TestClusterValidate / "Defaulted"` test in `validate_test.go` that calls `SetDefaultsCluster(&c)` on a zero-value `Cluster{}` will produce a cluster with all-false addons. This is correct behavior — the `Validate()` function does not check addon booleans. The existing validate tests will continue to pass.

### Pattern 2: Integration Between Layers Test

**What:** Test via the `encoding` package that an all-false v1alpha4 config survives conversion without being overridden.

```go
// File: pkg/internal/apis/config/encoding/convert_test.go (new file, or add to existing)
// or add to pkg/internal/apis/config/default_test.go via the Convertv1alpha4 path

package config

import (
    "testing"
    v1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

func TestConvertv1alpha4_AllAddonsExplicitFalse(t *testing.T) {
    f := false
    in := &v1alpha4.Cluster{
        Addons: v1alpha4.Addons{
            MetalLB:       &f,
            EnvoyGateway:  &f,
            MetricsServer: &f,
            CoreDNSTuning: &f,
            Dashboard:     &f,
        },
    }
    out := Convertv1alpha4(in)
    if out.Addons.MetalLB       { t.Error("MetalLB should be false") }
    if out.Addons.EnvoyGateway  { t.Error("EnvoyGateway should be false") }
    if out.Addons.MetricsServer { t.Error("MetricsServer should be false") }
    if out.Addons.CoreDNSTuning { t.Error("CoreDNSTuning should be false") }
    if out.Addons.Dashboard     { t.Error("Dashboard should be false") }
}
```

### Pattern 3: kubectl Context Targeting in Bash Script

**What:** Add `kubectl config use-context` after cluster creation in `hack/verify-integration.sh`.

**When to use:** Immediately after the `kinder create cluster` command succeeds.

```bash
# Source: kubectl documentation — context switching
# Add AFTER the kinder create cluster call succeeds:
echo "  Setting kubectl context to kind-${CLUSTER_NAME}..."
kubectl config use-context "kind-${CLUSTER_NAME}"
```

The cluster name is `kinder-integration-test`, so the kind-generated context name is `kind-kinder-integration-test`. This matches the kind naming convention (prefix `kind-` + cluster name).

Current script structure (lines 83-97):

```bash
kinder delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true

section "SC-1: Full cluster creation with all addons"

echo "  Creating cluster (this takes a few minutes)..."
if kinder create cluster --name "$CLUSTER_NAME" 2>&1 | tee "$CREATE_LOG"; then
  pass "SC-1a: kinder create cluster exited 0"
else
  fail "SC-1a: kinder create cluster exited non-zero"
fi
```

After the fix, add between the pass/fail block and the next kubectl call:

```bash
if kinder create cluster --name "$CLUSTER_NAME" 2>&1 | tee "$CREATE_LOG"; then
  pass "SC-1a: kinder create cluster exited 0"
  # Explicitly target the integration test cluster context
  kubectl config use-context "kind-${CLUSTER_NAME}"
else
  fail "SC-1a: kinder create cluster exited non-zero"
fi
```

### Anti-Patterns to Avoid

- **Adding a sentinel field to Addons:** Adding `AddonsExplicitlySet bool` or similar to differentiate zero-value from explicit-false is unnecessarily complex. The real fix is to remove the redundant `SetDefaultsCluster` call from `fixupOptions`.
- **Modifying the v1alpha4.SetDefaultsCluster:** That function is correct — it uses nil-pointer detection and does not override explicit `false` values. Do not touch it.
- **Fixing via fixupOptions only without touching default.go:** Remove the guard from `default.go` too, to prevent the bug from resurfacing if another call site is added.
- **Using `--context` on every kubectl call in the script:** Correct but high-churn. One `use-context` call is sufficient and idiomatic.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Addon defaulting at internal layer | Custom logic to detect "was this from a file?" | Remove the redundant SetDefaultsCluster call from fixupOptions | The v1alpha4 layer already handles correct defaulting; the bug is the second call |
| Context tracking in bash script | Stateful variable tracking which context is active | `kubectl config use-context` | Built-in kubectl command; idiomatic; no custom logic needed |
| Test assertion helpers | Custom comparison functions | Direct `t.Error` with `%+v` formatting | Existing pattern in `cluster_util_test.go` and `validate_test.go` |

**Key insight:** Both bugs are "too much code" problems, not "too little code" problems. The fix is deletion and simplification, not addition.

---

## Common Pitfalls

### Pitfall 1: Removing SetDefaultsCluster Breaks the Empty-Config Path

**What goes wrong:** After removing `config.SetDefaultsCluster(opts.Config)` from `fixupOptions`, the empty-config case (`opts.Config == nil`) appears to lose its defaulting. Developer panics and adds the call back.

**Why it happens:** Misreading `encoding.Load("")` — it already calls `config.SetDefaultsCluster` internally (load.go lines 31-33). The `fixupOptions` call on line 289 of create.go is genuinely redundant.

**How to avoid:** Before removing, verify that `encoding.Load("")` → `config.SetDefaultsCluster(out)` already handles the nil-config case. Read `pkg/internal/apis/config/encoding/load.go` lines 30-34. Then confirm by running `go test ./...` after the change.

**Warning signs:** If removing the call causes test failures in packages that call `fixupOptions` with a nil Config, inspect where `encoding.Load` is being called — it likely is not being called for those paths.

### Pitfall 2: Test Asserts Old Behavior (All-True After SetDefaultsCluster)

**What goes wrong:** After fixing the guard, the existing `TestClusterValidate / "Defaulted"` test may fail because it calls `SetDefaultsCluster` on a zero-value `Cluster{}` and then validates — but if addons are all-false after the fix, and `Validate()` checks that at least one addon is enabled, validation fails.

**Why it happens:** Validator might have an addon-requires-at-least-one check.

**How to avoid:** Read `pkg/internal/apis/config/validate.go` to confirm `Validate()` does not check addon booleans. Based on reading `validate_test.go`, there are no addon-related test cases there, and `Validate()` checks nodes, networking, and ports only. Safe to proceed.

**Warning signs:** `TestClusterValidate` starts failing with addon-related error messages after the fix.

### Pitfall 3: Context Name Mismatch in Bash Script

**What goes wrong:** Script uses `kubectl config use-context "kinder-integration-test"` (missing the `kind-` prefix) instead of `kubectl config use-context "kind-kinder-integration-test"`. kubectl reports "context not found" and subsequent checks fail.

**Why it happens:** kind prefixes context names with `kind-` to avoid collisions with existing contexts. The cluster is named `kinder-integration-test` but the kubectl context is `kind-kinder-integration-test`.

**How to avoid:** Use the variable: `kubectl config use-context "kind-${CLUSTER_NAME}"` where `CLUSTER_NAME=kinder-integration-test`. The result is `kind-kinder-integration-test` which matches kind's naming convention.

**Warning signs:** `kubectl config use-context` exits non-zero with "no context exists with the name".

### Pitfall 4: Script Fails If Context Switch Happens Before Cluster is Ready

**What goes wrong:** `kubectl config use-context` is called outside the success branch of the `kinder create cluster` if-block, running it even when cluster creation failed. Subsequent kubectl commands then fail for a different reason (wrong context or non-existent cluster).

**Why it happens:** Context switch placed unconditionally after the create call.

**How to avoid:** Place `kubectl config use-context` inside the success branch:

```bash
if kinder create cluster ...; then
  pass "SC-1a: ..."
  kubectl config use-context "kind-${CLUSTER_NAME}"
else
  fail "SC-1a: ..."
fi
```

**Warning signs:** Script tries to set context after failed create and subsequent checks report incorrect cluster state.

### Pitfall 5: Restore Call to SetDefaultsCluster Lost During Cleanup Refactor

**What goes wrong:** Developer removes BOTH the guard from `default.go` AND the call from `fixupOptions`, but also removes it from `encoding.Load("")` — breaking the no-config-file default entirely.

**Why it happens:** Over-aggressive cleanup.

**How to avoid:** Only remove the guard from `default.go`. Keep `config.SetDefaultsCluster(out)` in `encoding.Load("")` (load.go line 32). Only remove the call from `fixupOptions` (create.go line 289).

---

## Code Examples

Verified patterns from the actual codebase:

### Current Buggy Code (default.go lines 93-101)

```go
// Source: pkg/internal/apis/config/default.go (current — BUGGY)
// Default all addons to enabled (safety net for library usage where
// conversion may not have run).
if !obj.Addons.MetalLB && !obj.Addons.EnvoyGateway && !obj.Addons.MetricsServer && !obj.Addons.CoreDNSTuning && !obj.Addons.Dashboard {
    obj.Addons.MetalLB = true
    obj.Addons.EnvoyGateway = true
    obj.Addons.MetricsServer = true
    obj.Addons.CoreDNSTuning = true
    obj.Addons.Dashboard = true
}
```

### Fixed Code (default.go) — Remove the guard

```go
// Source: pkg/internal/apis/config/default.go (after fix)
// The all-addons guard is removed. Addon defaulting is handled at the
// v1alpha4 layer (pkg/apis/config/v1alpha4/default.go) using nil *bool detection.
// Internal-level SetDefaultsCluster does not touch Addons.
// (No code here for Addons — the block is deleted entirely)
```

### Current Buggy Code (create.go line 289)

```go
// Source: pkg/cluster/internal/create/create.go lines 287-291 (current — REDUNDANT CALL)
// default config fields (important for usage as a library, where the config
// may be constructed in memory rather than from disk)
config.SetDefaultsCluster(opts.Config)
```

### Fixed Code (create.go) — Remove the redundant call

```go
// Source: pkg/cluster/internal/create/create.go (after fix)
// config.SetDefaultsCluster call removed — encoding.Load already calls it
// for the nil-config case. For file-loaded configs, v1alpha4.SetDefaultsCluster
// runs before conversion and is the correct defaulting layer.
// (The SetDefaultsCluster call on line 289 is deleted entirely)
```

### Correct Code (verify-integration.sh) — Context targeting

```bash
# Source: hack/verify-integration.sh (after fix)
# Add after "pass SC-1a" inside the success branch:
echo "  Setting kubectl context to kind-${CLUSTER_NAME}..."
kubectl config use-context "kind-${CLUSTER_NAME}"
```

### Unit Test: All-Addons-Disabled Config Path

```go
// Source: new file pkg/internal/apis/config/default_test.go
package config

import "testing"

func TestSetDefaultsCluster_AddonsNotAffected(t *testing.T) {
    // After the fix, SetDefaultsCluster should NOT touch Addons at all.
    // An all-false Addons struct should remain all-false.
    c := &Cluster{
        Addons: Addons{
            MetalLB:       false,
            EnvoyGateway:  false,
            MetricsServer: false,
            CoreDNSTuning: false,
            Dashboard:     false,
        },
    }
    SetDefaultsCluster(c)

    if c.Addons.MetalLB {
        t.Error("SetDefaultsCluster must not enable MetalLB when explicitly set to false")
    }
    if c.Addons.EnvoyGateway {
        t.Error("SetDefaultsCluster must not enable EnvoyGateway when explicitly set to false")
    }
    if c.Addons.MetricsServer {
        t.Error("SetDefaultsCluster must not enable MetricsServer when explicitly set to false")
    }
    if c.Addons.CoreDNSTuning {
        t.Error("SetDefaultsCluster must not enable CoreDNSTuning when explicitly set to false")
    }
    if c.Addons.Dashboard {
        t.Error("SetDefaultsCluster must not enable Dashboard when explicitly set to false")
    }
}
```

### Existing Test Pattern to Follow (same package)

```go
// Source: pkg/internal/apis/config/cluster_util_test.go (existing)
// This is the canonical test pattern in pkg/internal/apis/config/
package config

import (
    "testing"
    "sigs.k8s.io/kind/pkg/internal/assert"
)

func TestClusterHasIPv6(t *testing.T) {
    cases := []struct {
        Name     string
        c        *Cluster
        expected bool
    }{
        // ...
    }
    for _, tc := range cases {
        tc := tc
        t.Run(tc.Name, func(t *testing.T) {
            r := ClusterHasIPv6(tc.c)
            assert.BoolEqual(t, tc.expected, r)
        })
    }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| All-false guard in internal SetDefaultsCluster | Remove guard; rely on v1alpha4-layer defaulting | Phase 8 (this phase) | All-addons-disabled config respected correctly |
| Integration script uses ambient kubectl context | Explicit `kubectl config use-context kind-$CLUSTER_NAME` | Phase 8 (this phase) | Script reliably targets the correct cluster |

**Deprecated/outdated:**
- Lines 93-101 of `pkg/internal/apis/config/default.go` — the all-or-nothing addon guard. Replaced by doing nothing (the correct defaulting already happened upstream).
- Line 289 of `pkg/cluster/internal/create/create.go` — the redundant `config.SetDefaultsCluster` call. Remove entirely.

---

## Open Questions

1. **Is removing the redundant SetDefaultsCluster call safe for all callers of Cluster()?**
   - What we know: `fixupOptions` is the only caller of internal `config.SetDefaultsCluster` outside of `encoding.Load`. Read the grep results: two call sites total. `encoding/load.go` line 32 (correct, keep it) and `create.go` line 289 (redundant, remove it). One more call in `pkg/cluster/internal/providers/common/proxy_test.go` line 30 — this is a test that calls it directly on a fresh `&config.Cluster{}` struct to set networking defaults. That test will still work correctly since the non-addon defaults remain in `SetDefaultsCluster`.
   - What's unclear: Whether any test in `proxy_test.go` reads addon values after calling `SetDefaultsCluster`. If it does, it would observe all-false after the fix.
   - Recommendation: Read `proxy_test.go` during planning to confirm it does not assert on Addons values. Based on the test name ("proxy"), it almost certainly only tests networking fields.

2. **Does the audit re-run criteria require modifying REQUIREMENTS.md or other planning docs?**
   - What we know: Phase 8 success criterion 4 is "Re-running `/gsd:audit-milestone` produces zero integration issues and zero partial flows." This is a verification criterion, not a code change.
   - What's unclear: Whether REQUIREMENTS.md needs updating to mark the edge case fix. FOUND-04 is already marked `Complete` in REQUIREMENTS.md.
   - Recommendation: No REQUIREMENTS.md change needed. The fix makes FOUND-04 truly complete for the all-addons-disabled edge case. The audit will reflect this.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` |
| Config file | None — uses `go test ./...` |
| Quick run command | `go test ./pkg/internal/apis/config/... ./pkg/cluster/internal/create/...` |
| Full suite command | `go test ./...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FOUND-04 | All-addons-disabled config produces zero-addon internal struct | Unit | `go test ./pkg/internal/apis/config/...` | ❌ Wave 0: `pkg/internal/apis/config/default_test.go` |
| SC-1 (implicit) | Integration script targets correct kubectl context | Manual (script run) | `./hack/verify-integration.sh` | ✅ exists, needs 3-line patch |

### Sampling Rate

- **Per task commit:** `go test ./...` must pass (all unit tests green)
- **Per wave merge:** `go test ./...` + `bash -n hack/verify-integration.sh` (syntax check)
- **Phase gate:** `go test ./...` green + zero audit issues from `/gsd:audit-milestone`

### Wave 0 Gaps

- [ ] `pkg/internal/apis/config/default_test.go` — covers the all-addons-disabled edge case (FOUND-04 completeness)

*(All other test infrastructure exists. `hack/verify-integration.sh` exists and only needs 3 lines added.)*

---

## Sources

### Primary (HIGH confidence)

- Kinder codebase, direct file reads (2026-03-01):
  - `pkg/internal/apis/config/default.go` — buggy guard at lines 93-101
  - `pkg/cluster/internal/create/create.go` — `fixupOptions` at lines 262-292, redundant `SetDefaultsCluster` call at line 289
  - `pkg/internal/apis/config/encoding/load.go` — shows `SetDefaultsCluster` already called in empty-path case
  - `pkg/internal/apis/config/encoding/convert.go` — `V1Alpha4ToInternal` calls `v1alpha4.SetDefaultsCluster` then `config.Convertv1alpha4`
  - `pkg/apis/config/v1alpha4/default.go` — correct nil-pointer-based addon defaulting
  - `pkg/internal/apis/config/convert_v1alpha4.go` — `boolVal` function correctly handles `nil`->true, `&false`->false
  - `hack/verify-integration.sh` — script missing context targeting, confirmed by reading lines 83-117
  - `.planning/v1.0-MILESTONE-AUDIT.md` — audit that identified both issues with exact line references
  - `pkg/internal/apis/config/cluster_util_test.go` — canonical test pattern for the config package
  - `pkg/internal/apis/config/validate_test.go` — confirms Validate() does not check Addons booleans
  - `go test ./...` output — all 15+ existing tests pass; no regressions before fix

### Secondary (MEDIUM confidence)

- kubectl documentation (well-known behavior): `kubectl config use-context <name>` switches the active context for all subsequent commands in the same shell session. The kind cluster name convention `kind-<cluster-name>` is documented in kind's own output.

### Tertiary (LOW confidence)

- None — this phase is purely internal code with no external dependencies.

---

## Metadata

**Confidence breakdown:**
- Root cause diagnosis: HIGH — traced the exact call chain from YAML parsing through v1alpha4 conversion to internal defaulting; confirmed by reading all relevant source files
- Fix approach: HIGH — two-line removal (guard + redundant call); backed by reading all call sites
- Unit test approach: HIGH — follows existing pattern in the same package exactly
- Script fix: HIGH — kubectl use-context is well-known; kind context naming is documented in kind output

**Research date:** 2026-03-01
**Valid until:** 2026-06-01 (stable — internal Go code changes slowly; this is a targeted bug fix)
