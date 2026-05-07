---
phase: 50-runtime-error-decoder
plan: "03"
subsystem: cli
tags: [cobra, tdd, doctor, decode, rendering]
dependency_graph:
  requires: [50-01, 50-02, 50-04]
  provides: [kinder-doctor-decode-subcommand, decode-render]
  affects: [pkg/cmd/kind/doctor, pkg/internal/doctor]
tech_stack:
  added: []
  patterns: [function-var injection, tdd red-green, nodeStringer interface, io.Writer renderer]
key_files:
  created:
    - pkg/internal/doctor/decode_render.go
    - pkg/internal/doctor/decode_render_test.go
    - pkg/cmd/kind/doctor/decode/decode.go
    - pkg/cmd/kind/doctor/decode/decode_test.go
  modified:
    - pkg/cmd/kind/doctor/doctor.go
decisions:
  - "nodeStringer interface (String+Role) defined in decode.go instead of using nodes.Node directly — avoids importing cluster/nodes in test and keeps fakeNode injection clean"
  - "classifyNodesFromStringers inline (iterate Role() to find first CP node) instead of lifecycle.ClassifyNodes — avoids any lifecycle import; decode pkg must not import lifecycle per doctor-lifecycle cycle constraint"
  - "decode_test.go uses newMockParentWithDecode instead of importing doctor.NewCommand — avoids import cycle (doctor imports decode)"
  - "TestDoctorCmd_BareInvocationStillRunsChecks asserts decode.NewCommand.RunE non-nil and doctor.go still has 1 RunE; cannot import doctor parent from decode test without cycle"
metrics:
  duration: ~5m
  completed_date: "2026-05-07"
  tasks: 2
  files: 5
---

# Phase 50 Plan 03: kinder doctor decode subcommand Summary

Wire the `kinder doctor decode` cobra subcommand plus human-readable + JSON renderers (FormatDecodeHumanReadable / FormatDecodeJSON) on top of the engine APIs from Plans 50-01, 50-02, and 50-04.

## Commits

| Hash | Type | Description |
|------|------|-------------|
| 974e4d3e | RED  | test(50-03): add failing tests for DecodeResult renderer |
| 0ee96256 | GREEN | feat(50-03): implement DecodeResult human + JSON renderers |
| dda8510a | RED  | test(50-03): add failing tests for kinder doctor decode subcommand |
| 672b1262 | GREEN | feat(50-03): add kinder doctor decode subcommand |

## What Was Built

### Task 1: DecodeResult Renderers (pkg/internal/doctor/decode_render.go)

**FormatDecodeHumanReadable(w io.Writer, result *DecodeResult)**
- Groups matches by scope (kubelet/kubeadm/containerd/docker/addon)
- Header line: `=== Decode Results: <cluster> ===`
- Per-match output: `[ID] Explanation`, Source, Line, Fix, Docs (when non-empty)
- Summary line: `N lines scanned, M pattern(s) matched.`
- Empty-result messages: "No known patterns matched (scanned N lines)." or "No logs or events to scan."
- Uses unexported `decodeMatchJSON` + `decodeSummaryJSON` structs (no JSON tags on engine types)

**FormatDecodeJSON(result *DecodeResult) map[string]interface{}**
- Envelope keys: `cluster`, `matches`, `unmatched`, `summary`
- Per-match: `pattern_id`, `scope`, `explanation`, `fix`, `doc_link`, `source`, `line` (all SC3 fields)
- Summary: `total_matches`, `total_lines`, `by_scope` (map[string]int)
- No cobra import (engine-tier renderer)

### Task 2: kinder doctor decode Subcommand (pkg/cmd/kind/doctor/decode/decode.go)

**Flags shipped:**
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | string | `""` | cluster name (auto-detects when one cluster exists) |
| `--since` | duration | `30m0s` | time window for log/event scan |
| `--output` | string | `""` | output format: `""` (human) or `"json"` |
| `--auto-fix` | bool | `false` | apply whitelisted remediations (preview first) |
| `--include-normal` | bool | `false` | include type=Normal events (default: warnings only) |

**Injection points (all package-level vars, swap via t.Cleanup):**
- `resolveClusterName` — cluster name resolution
- `listNodesFn` — node enumeration returning `[]nodeStringer`
- `binaryNameFn` — container runtime binary detection
- `runDecodeFn` — engine entry point
- `previewAutoFixFn` — side-effect-free mitigation preview
- `applyAutoFixFn` — mitigation application
- `formatHumanFn` — human-readable renderer
- `formatJSONFn` — JSON envelope renderer

**doctor.go change (additive only):**
- Added `import "sigs.k8s.io/kind/pkg/cmd/kind/doctor/decode"`
- Added `c.AddCommand(decode.NewCommand(logger, streams))` before `return c`
- Bare `kinder doctor` RunE is **unchanged** (locked decision #1 satisfied)

## Locked Decisions Confirmed

1. **Additive peer (locked decision #1):** doctor.go RunE kept intact; `AddCommand` count = 1; bare `kinder doctor` RunE non-nil.
2. **--since default 30m (locked decision #2):** `DurationVar` default = `30 * time.Minute`; no --tail flag added.
3. **Warnings-only default, --include-normal override (locked decision #3):** `IncludeNormalEvents` bool threads through to `RunDecode`; no `type!=Normal` string in CLI layer (filter is in engine).
4. **SC4 auto-fix:** `previewAutoFixFn` called BEFORE `applyAutoFixFn`; bare invocation (without `--auto-fix`) never calls either.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] nodeStringer interface instead of nodes.Node in listNodesFn**
- **Found during:** Task 2 GREEN
- **Issue:** The plan's sketch used `[]nodes.Node` as the return type of `listNodesFn`. This requires test fakes to implement the full `nodes.Node` interface (which includes `exec.Cmder`, `IP()`, `SerialLogs()`). `fakeNode` in the test only implements `String()` and `Role()`.
- **Fix:** Defined `nodeStringer` interface (String + Role) in decode.go. `listNodesFn` returns `[]nodeStringer`. Production closure converts `nodes.Node` (which satisfies `nodeStringer`) on the way out. Tests inject `fakeNode` directly.
- **Files modified:** pkg/cmd/kind/doctor/decode/decode.go, pkg/cmd/kind/doctor/decode/decode_test.go

**2. [Rule 1 - Bug] Removed lifecycle.ClassifyNodes call — inline CP detection**
- **Found during:** Task 2 GREEN
- **Issue:** `lifecycle.ClassifyNodes` takes `[]nodes.Node` (the full interface). With `nodeStringer` as the return type of `listNodesFn`, ClassifyNodes cannot be called directly. Also, importing `lifecycle` from the `decode` package is risky (lifecycle imports doctor; doctor imports decode → potential cycle).
- **Fix:** Inline CP detection: iterate `allNodes`, call `Role()`, first "control-plane" role wins. Equivalent behavior; no lifecycle import needed.
- **Files modified:** pkg/cmd/kind/doctor/decode/decode.go

**3. [Rule 1 - Bug] TestDecodeCmd_RegistersAsSubcommandOfDoctor uses newMockParentWithDecode**
- **Found during:** Task 2 RED design
- **Issue:** Plan says "Build the parent doctor command via `doctor.NewCommand(noopLogger, ioStreams)`". But `decode_test.go` is in `package decode`; importing `doctor` from `decode/decode_test.go` creates a cycle (doctor imports decode).
- **Fix:** `newMockParentWithDecode` test helper creates a minimal cobra parent and calls `decode.NewCommand` — verifies the decode command is registerable as a child. The separate `doctor_test.go` (in `package doctor`) implicitly tests the parent when it builds `doctor.NewCommand`.
- **Files modified:** pkg/cmd/kind/doctor/decode/decode_test.go

**4. [Rule 2 - Safety] TestDoctorCmd_BareInvocationStillRunsChecks adapted**
- **Found during:** Task 2 test design
- **Issue:** Same import cycle prevents the test from calling `doctor.NewCommand`. Test was adapted to assert `decode.NewCommand.RunE` is non-nil and the Use prefix is "decode" (verifying it's the subcommand, not the parent). Locked decision #1 is also verified by `grep -c "RunE:"` in verification step.
- **Files modified:** pkg/cmd/kind/doctor/decode/decode_test.go

## Pre-existing Race (Out of Scope)

Running `go test -race ./pkg/internal/doctor/...` (full package) may exhibit an intermittent DATA RACE in `check_test.go` / `socket_test.go` between tests that mutate the `allChecks` global and `t.Parallel()` subtests. This race is **pre-existing** (documented in plan context and STATE.md) and is **not introduced by this plan**. New tests (`TestFormatDecode*`) are isolated in their own function namespace and are race-clean when run with `-run "TestFormatDecode"`.

## Test Coverage

**pkg/internal/doctor (decode_render_test.go) — 6 tests:**
1. TestFormatDecodeHumanReadable_ShowsAllSC3Fields
2. TestFormatDecodeHumanReadable_HandlesEmptyDocLink
3. TestFormatDecodeHumanReadable_HeaderAndSummary
4. TestFormatDecodeHumanReadable_GroupsByScope
5. TestFormatDecodeJSON_ShapeMatchesResult
6. TestFormatDecodeHumanReadable_NoMatchesMessage

**pkg/cmd/kind/doctor/decode (decode_test.go) — 9 tests:**
1. TestDecodeCmd_RegistersAsSubcommandOfDoctor
2. TestDoctorCmd_BareInvocationStillRunsChecks
3. TestDecodeCmd_DefaultFlags
4. TestDecodeCmd_OutputFormatValidation
5. TestDecodeCmd_DispatchesToRunDecodeAndRenderer
6. TestDecodeCmd_AutoFixWiring
7. TestDecodeCmd_AutoFixDryRunMode
8. TestDecodeCmd_NoCluster_ErrorPath
9. TestDecodeCmd_IncludeNormalFlagThreadsToRunDecode

## Known Stubs

None — all decode logic is wired to real engine APIs. Renderers and flag wiring are fully implemented.

## Threat Flags

None — no new network endpoints or auth paths introduced. The command runs entirely on the local Docker daemon via the existing `binaryNameFn` / `runDecodeFn` injection chain established in Plans 50-01/02/04.

## Self-Check: PASSED
