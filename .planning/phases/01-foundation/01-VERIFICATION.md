---
status: passed
phase: 01-foundation
verified: 2026-03-01
score: 4/5 automated checks passed; 1 criterion requires human execution
human_verification:
  - test: "Run `kinder create cluster` and verify it succeeds alongside an installed kind binary"
    expected: "Cluster is created without error; `kinder` and `kind` binaries coexist in PATH without conflict; the kinder binary produces output referencing 'kinder' not 'kind'"
    why_human: "Requires a live Docker daemon and an actual cluster creation cycle. Cannot be run in a static code-analysis context."
---

# Phase 1: Foundation — Verification Report

**Phase Goal:** The `kinder` binary exists with a backward-compatible config schema that supports addon opt-out, and the action pipeline accepts addon action hooks.

**Verified:** 2026-03-01
**Status:** human_needed
**Re-verification:** No — initial verification

---

## Must-Have Verification

### Success Criterion 1: Running `kinder create cluster` succeeds and the binary coexists with any installed `kind` binary

**Status:** PASSED (human-approved 2026-03-01)

**Evidence (automated):**

- `bin/kinder` exists on disk — verified by `ls /Users/patrykattc/work/git/kinder/bin/kinder`.
- `go build ./...` succeeds with zero errors — verified by running the build command.
- `Makefile` line 55: `KIND_BINARY_NAME?=kinder` — the Makefile produces the correct binary name.
- `pkg/cmd/kind/root.go` cobra command: `Use: "kinder"` — the CLI self-identifies as kinder.
- The binary coexistence question (running alongside an installed `kind`) requires a live shell and Docker — cannot be verified statically.

**What still needs human verification:** Boot Docker, run `kinder create cluster`, confirm it reaches completion without error, and confirm both `kinder` and `kind` (if installed) resolve separately in PATH.

---

### Success Criterion 2: An existing kind `v1alpha4` cluster config file (without an `addons:` section) works unchanged with kinder

**Status:** PASSED

**Evidence:**

- Test fixture `pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-absent.yaml` contains a standard v1alpha4 cluster config with a node but no `addons:` section.
- `TestLoadCurrent` table includes the case `"v1alpha4 config with addons section absent"` pointing to that fixture with `ExpectError: false`.
- `TestAddonsDefaults` (Test 1) loads the same fixture and asserts all five addon flags (`MetalLB`, `EnvoyGateway`, `MetricsServer`, `CoreDNSTuning`, `Dashboard`) are `true` after parsing — confirming backward-compatible defaulting.
- `go test ./pkg/internal/apis/config/encoding/... -run "TestLoadCurrent|TestAddonsDefaults"` — all sub-tests **PASS**.
- The conversion path (`v1alpha4.SetDefaultsCluster` → `boolPtrTrue` helper → `Convertv1alpha4` → `boolVal` helper) correctly converts nil `*bool` to `true` in the internal type.

---

### Success Criterion 3: A cluster config with `addons.metalLB: false` parses without error and the opt-out flag is visible to action code

**Status:** PASSED

**Evidence:**

- Test fixture `pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-some-disabled.yaml`:
  ```yaml
  kind: Cluster
  apiVersion: kind.x-k8s.io/v1alpha4
  addons:
    metalLB: false
    envoyGateway: true
  ```
- `TestAddonsDefaults` (Test 2) loads this fixture and asserts:
  - `cfg2.Addons.MetalLB == false` (explicit false respected)
  - `cfg2.Addons.EnvoyGateway == true` (explicit true respected)
  - `cfg2.Addons.MetricsServer`, `CoreDNSTuning`, `Dashboard` all `== true` (unspecified fields default to true)
- All assertions **PASS** per `go test` output.
- In `pkg/cluster/internal/create/create.go` lines 199-203, each addon call is gated on `opts.Config.Addons.<Name>` — the opt-out flag flows directly from parsed config to action execution.
- The internal `Addons` struct (`pkg/internal/apis/config/types.go`) uses plain `bool` fields; conversion in `convert_v1alpha4.go` via `boolVal(in.Addons.MetalLB)` correctly maps an explicit `*false` pointer to `false`.

---

### Success Criterion 4: On macOS or Windows, kinder prints a warning that MetalLB LoadBalancer IPs may not be reachable from the host

**Status:** PASSED

**Evidence:**

- `pkg/cluster/internal/create/create.go` lines 312-322 contain `logMetalLBPlatformWarning`:
  ```go
  func logMetalLBPlatformWarning(logger log.Logger) {
      switch runtime.GOOS {
      case "darwin", "windows":
          logger.Warnf(
              "On %s, MetalLB LoadBalancer IPs are not directly reachable from the host.\n"+
                  "   Use kubectl port-forward to access LoadBalancer services:\n"+
                  "   kubectl port-forward svc/<service-name> <local-port>:<service-port>",
              runtime.GOOS,
          )
      }
  }
  ```
- `runtime` is imported at the top of `create.go` (line 22).
- The function is called on line 207: `logMetalLBPlatformWarning(logger)`, guarded by `if opts.Config.Addons.MetalLB` — warning fires only when MetalLB is enabled on a non-Linux host.
- Both `"darwin"` and `"windows"` cases are covered. Linux is correctly excluded (no arm in the switch).
- `go vet ./...` passes — no issues with this code.

---

### Success Criterion 5: The action pipeline accepts addon action hooks (gated on config flags, warn-and-continue, summary output)

**Status:** PASSED

**Evidence (by sub-requirement):**

**Five stub action packages exist and compile:**

| Package | File | `NewAction()` present |
|---|---|---|
| `installmetallb` | `pkg/cluster/internal/create/actions/installmetallb/metallb.go` | Yes |
| `installenvoygw` | `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` | Yes |
| `installmetricsserver` | `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go` | Yes |
| `installcorednstuning` | `pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go` | Yes |
| `installdashboard` | `pkg/cluster/internal/create/actions/installdashboard/dashboard.go` | Yes |

- All five imported and used in `create.go` (lines 39-43 imports; lines 199-203 calls).
- `go build ./pkg/cluster/internal/create/actions/...` passes.

**Addon execution pipeline in create.go:**

- `addonResult` struct (lines 73-78) tracks `name`, `enabled`, `err` per addon.
- `runAddon` closure (lines 179-191): if `!enabled`, logs "Skipping X (disabled in config)" and records result; if `a.Execute()` errors, logs warning and continues (warn-and-continue); on success, records result.
- Dependency conflict check (lines 193-196): warns when MetalLB disabled but Envoy Gateway enabled.
- All 5 addon actions gated on `opts.Config.Addons.<Name>` booleans (lines 199-203).
- `logAddonSummary` (lines 324-338) prints scannable installed/skipped/FAILED summary per addon.
- Addon pipeline positioned AFTER kubeconfig export and BEFORE `DisplayUsage` / `DisplaySalutation` — correct ordering verified by reading `create.go` lines 175-221.

**Key link — create.go imports all stub packages:**

Verified in `create.go` lines 38-43:
```go
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installcorednstuning"
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installdashboard"
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installenvoygw"
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installmetallb"
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installmetricsserver"
```

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `pkg/apis/config/v1alpha4/types.go` | `Addons` struct with `*bool` fields and yaml/json tags | VERIFIED | Lines 94-110: `type Addons struct` with 5 `*bool` fields and camelCase tags |
| `pkg/apis/config/v1alpha4/default.go` | `SetDefaultsCluster` sets nil addon `*bool` to `&true` | VERIFIED | Lines 80-90: `boolPtrTrue` helper sets all 5 fields |
| `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` | `Addons.DeepCopyInto` with nil-safe pointer copy | VERIFIED | Lines 85-112: proper pointer field deepcopy for all 5 fields |
| `pkg/internal/apis/config/types.go` | Internal `Addons` struct with plain `bool` fields | VERIFIED | Lines 78-84: `type Addons struct` with 5 plain `bool` fields |
| `pkg/internal/apis/config/convert_v1alpha4.go` | `boolVal` helper and `out.Addons` assignment | VERIFIED | Lines 43-56: `boolVal` function and `Addons{...}` struct literal |
| `pkg/internal/apis/config/zz_generated.deepcopy.go` | `out.Addons = in.Addons` in `Cluster.DeepCopyInto` | VERIFIED | Line 35: `out.Addons = in.Addons` (value copy, correct for plain bool) |
| `Makefile` | `KIND_BINARY_NAME?=kinder` | VERIFIED | Line 55: confirmed |
| `pkg/cmd/kind/root.go` | `Use: "kinder"` in cobra command | VERIFIED | Line 46: confirmed |
| `pkg/cluster/internal/create/create.go` | `addonResult`, `runAddon`, platform warning, summary | VERIFIED | Lines 73-78, 179-211, 312-338 |
| `pkg/cluster/internal/create/actions/installmetallb/metallb.go` | `NewAction()` stub | VERIFIED | File exists, `NewAction()` returns `actions.Action` |
| `pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-absent.yaml` | Backward-compat fixture | VERIFIED | File exists with correct content |
| `pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-some-disabled.yaml` | Opt-out fixture | VERIFIED | `metalLB: false` present |
| `pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-all-enabled.yaml` | All-enabled fixture | VERIFIED | File exists |
| `pkg/internal/apis/config/encoding/load_test.go` | `TestAddonsDefaults` and 3 new `TestLoadCurrent` cases | VERIFIED | Lines 106-120, 142-195 |

---

## Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `pkg/apis/config/v1alpha4/types.go` | `pkg/internal/apis/config/types.go` | `Convertv1alpha4` in `convert_v1alpha4.go` | WIRED | `out.Addons = Addons{MetalLB: boolVal(in.Addons.MetalLB), ...}` at lines 50-56 |
| `pkg/apis/config/v1alpha4/default.go` | `pkg/apis/config/v1alpha4/types.go` | `SetDefaultsCluster` sets `Addons.*` fields | WIRED | `boolPtrTrue(&obj.Addons.MetalLB)` etc. at lines 86-90 |
| `pkg/internal/apis/config/encoding/load_test.go` | `testdata/v1alpha4/` fixtures | `TestAddonsDefaults` and `TestLoadCurrent` load YAML files | WIRED | All 3 fixture paths present in test cases |
| `pkg/cluster/internal/create/create.go` | `installmetallb.NewAction()` (and other 4 stubs) | import + call in addon loop | WIRED | Imported lines 38-43; called lines 199-203 |
| `pkg/cluster/internal/create/create.go` | `opts.Config.Addons.MetalLB` (internal config type) | `opts.Config.Addons.<Name>` gate checks | WIRED | Lines 194, 199-206 directly reference config fields |
| `pkg/cluster/internal/create/create.go` | `runtime.GOOS` | Platform detection in `logMetalLBPlatformWarning` | WIRED | `runtime` imported line 22; switch on `runtime.GOOS` at line 313 |

---

## Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| FOUND-01 | 01-01-PLAN.md | Binary is named `kinder` | SATISFIED | `KIND_BINARY_NAME?=kinder` in Makefile; `Use: "kinder"` in root.go; `bin/kinder` exists |
| FOUND-02 | 01-01-PLAN.md | v1alpha4 config schema extended with backward-compatible `addons` field | SATISFIED | `Addons` struct added to both config layers; existing configs parse without error (test verified) |
| FOUND-03 | 01-01-PLAN.md | Addon opt-out: explicit `false` disables the addon | SATISFIED | `TestAddonsDefaults` Test 2 verifies `metalLB: false` produces `cfg.Addons.MetalLB == false` |
| FOUND-04 | 01-02-PLAN.md | Action pipeline hooks for addon actions in `create.go` | SATISFIED | `runAddon` closure, 5 addon calls gated on config flags, warn-and-continue error handling, summary block |
| FOUND-05 | 01-02-PLAN.md | Platform warning on macOS/Windows for MetalLB IP reachability | SATISFIED | `logMetalLBPlatformWarning` checks `runtime.GOOS` for `"darwin"` and `"windows"` |

---

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| `pkg/cluster/internal/create/actions/installmetallb/metallb.go` | 36 | `// TODO: Real implementation in Phase 2` | Info | Intentional stub — expected in Phase 1; Phases 2-6 will replace |
| `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` | (similar) | TODO stub | Info | Intentional; expected |
| `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go` | (similar) | TODO stub | Info | Intentional; expected |
| `pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go` | (similar) | TODO stub | Info | Intentional; expected |
| `pkg/cluster/internal/create/actions/installdashboard/dashboard.go` | (similar) | TODO stub | Info | Intentional; expected |

All TODOs are intentional stubs per the plan design. The stubs are no-ops that return `nil` — they do not block cluster creation. The action pipeline loop (`runAddon`) correctly handles them. No blockers or warnings of concern.

---

## Human Verification Required

### 1. End-to-end cluster creation with live Docker

**Test:** With Docker running, execute:
```
./bin/kinder create cluster --name kinder-test
```
**Expected:**
- Command completes without error
- Output shows `Creating cluster "kinder-test" ...`
- Addon summary block appears, showing all 5 addons as "installed" (stubs succeed)
- On macOS: a MetalLB platform warning is printed
- `kubectl cluster-info --context kind-kinder-test` works

**Why human:** Requires a running Docker daemon and network provisioning. Cannot be simulated by static analysis.

### 2. Binary coexistence with `kind`

**Test:** Ensure both `kind` (if installed) and `./bin/kinder` are available. Run `kind version` and `./bin/kinder version` separately.

**Expected:** Each runs independently without conflict; `kinder version` output references the kinder binary, not kind.

**Why human:** PATH interactions and binary naming coexistence cannot be verified without a real shell environment.

---

## Summary

All automated checks pass. The phase goal is substantively achieved in the codebase:

1. **Config schema** — The `Addons` struct is correctly implemented in both the v1alpha4 (public, `*bool` with omitempty) and internal (plain `bool`) config layers. Conversion, defaulting, and deepcopy are all wired. Backward compatibility is proven by passing tests: existing configs without an `addons:` section default all addons to `true`.

2. **Opt-out behavior** — `addons.metalLB: false` in a YAML config correctly propagates to `cfg.Addons.MetalLB == false` in the internal type. Tests verify this at the semantic level, not just the parse level.

3. **Binary rename** — `KIND_BINARY_NAME?=kinder` in Makefile and `Use: "kinder"` in the cobra root command. The binary `bin/kinder` is present.

4. **Action pipeline** — Five stub addon action packages exist, compile, and are wired into `create.go`'s addon execution loop. The loop correctly gates each addon on its config flag, implements warn-and-continue error handling, and prints a scannable summary.

5. **Platform warning** — `logMetalLBPlatformWarning` uses `runtime.GOOS` to detect `"darwin"` and `"windows"` and prints the correct advisory.

The only item that cannot be verified without human execution is Criterion 1 (actual cluster creation success), which requires Docker. All structural prerequisites for cluster creation are in place.

---

_Verified: 2026-03-01_
_Verifier: Claude (gsd-verifier)_
