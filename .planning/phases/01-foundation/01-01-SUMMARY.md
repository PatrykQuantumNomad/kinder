---
phase: 01-foundation
plan: 01
subsystem: config
tags: [config, types, addons, binary-rename, deepcopy, conversion, defaults, tests]
requires: []
provides: [addons-config-schema, kinder-binary]
affects: [all-addon-phases]
tech-stack:
  added: []
  patterns: [pointer-to-bool-for-optional-config, nil-means-true-default, internal-external-type-split]
key-files:
  created:
    - pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-all-enabled.yaml
    - pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-some-disabled.yaml
    - pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-absent.yaml
  modified:
    - pkg/apis/config/v1alpha4/types.go
    - pkg/apis/config/v1alpha4/default.go
    - pkg/apis/config/v1alpha4/zz_generated.deepcopy.go
    - pkg/internal/apis/config/types.go
    - pkg/internal/apis/config/default.go
    - pkg/internal/apis/config/convert_v1alpha4.go
    - pkg/internal/apis/config/zz_generated.deepcopy.go
    - pkg/internal/apis/config/encoding/load_test.go
    - Makefile
    - pkg/cmd/kind/root.go
key-decisions:
  - "*bool for v1alpha4 Addons fields: nil means not-set (defaults to true), false means explicitly disabled — avoids Go bool zero-value ambiguity after YAML decode"
  - "Internal config uses plain bool (no pointers): simpler consumption, nil-to-true conversion happens at conversion boundary in convert_v1alpha4.go"
  - "Safety-net defaulting in internal SetDefaultsCluster: guards against library usage where conversion may not run"
  - "Binary renamed via Makefile KIND_BINARY_NAME and cobra Use field only — module path stays sigs.k8s.io/kind unchanged"
metrics:
  duration: "2 minutes"
  completed: "2026-03-01T14:30:26Z"
  tasks: 2
  files_changed: 10
---

# Phase 1 Plan 1: Config Schema and Binary Rename Summary

**One-liner:** Addons struct with *bool fields wired through v1alpha4/internal config layers with nil=true conversion, plus kinder binary rename via Makefile.

## Performance

- **Duration:** ~2 minutes
- **Started:** 2026-03-01T14:28:00Z
- **Completed:** 2026-03-01T14:30:26Z
- **Tasks completed:** 2/2
- **Files changed:** 10

## Accomplishments

1. Added `Addons` struct with 5 `*bool` fields (MetalLB, EnvoyGateway, MetricsServer, CoreDNSTuning, Dashboard) to v1alpha4 public types with camelCase yaml/json tags and omitempty
2. Added `Addons Addons` field to v1alpha4 `Cluster` struct after `Networking`
3. Implemented nil-defaults-to-true pattern in v1alpha4 `SetDefaultsCluster` via `boolPtrTrue` helper
4. Added `Addons.DeepCopyInto` and `DeepCopy` methods to v1alpha4 `zz_generated.deepcopy.go` with proper pointer field handling; wired into `Cluster.DeepCopyInto`
5. Added internal `Addons` struct with plain `bool` fields (no pointers, no tags) to `pkg/internal/apis/config/types.go`
6. Added `Addons Addons` field to internal `Cluster` struct
7. Added safety-net defaulting in internal `SetDefaultsCluster` (all-false guard pattern)
8. Added trivial `DeepCopyInto`/`DeepCopy` for internal `Addons` (value type, no pointers)
9. Wired `Addons` conversion in `Convertv1alpha4` with `boolVal` helper (nil or true pointer => true, false pointer => false)
10. Renamed binary: `KIND_BINARY_NAME?=kinder` in Makefile, `Use: "kinder"` in cobra root command
11. Updated Short/Long descriptions in root.go to reflect kinder branding
12. Created 3 YAML test fixtures for addons scenarios
13. Extended `TestLoadCurrent` with 3 new table-driven test cases
14. Added `TestAddonsDefaults` function verifying semantic behavior across all 3 scenarios

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add Addons struct to both config layers with defaults, conversion, and deepcopy | d800b450 | 7 files modified |
| 2 | Rename binary to kinder and add config parsing tests for addons | 20f3f141 | 3 files modified, 3 files created |

## Files Created

- `/Users/patrykattc/work/git/kinder/pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-all-enabled.yaml`
- `/Users/patrykattc/work/git/kinder/pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-some-disabled.yaml`
- `/Users/patrykattc/work/git/kinder/pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-absent.yaml`

## Files Modified

- `pkg/apis/config/v1alpha4/types.go` — added Addons struct and Cluster.Addons field
- `pkg/apis/config/v1alpha4/default.go` — added boolPtrTrue defaulting for all 5 addon fields
- `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` — added Addons DeepCopyInto/DeepCopy, wired into Cluster.DeepCopyInto
- `pkg/internal/apis/config/types.go` — added internal Addons struct (plain bool) and Cluster.Addons field
- `pkg/internal/apis/config/default.go` — added all-false guard defaulting for Addons
- `pkg/internal/apis/config/convert_v1alpha4.go` — added boolVal helper and Addons conversion block
- `pkg/internal/apis/config/zz_generated.deepcopy.go` — added Addons DeepCopyInto/DeepCopy, wired out.Addons = in.Addons in Cluster
- `pkg/internal/apis/config/encoding/load_test.go` — added 3 TestLoadCurrent cases and TestAddonsDefaults function
- `Makefile` — KIND_BINARY_NAME changed from kind to kinder
- `pkg/cmd/kind/root.go` — cobra Use/Short/Long updated to kinder

## Decisions Made

1. **`*bool` for v1alpha4 Addons fields:** Go's bool zero value (`false`) is indistinguishable from explicit `false` after YAML decode. Using `*bool` makes nil mean "not specified" which then defaults to true, while an explicit `false` pointer means the user explicitly disabled the addon.

2. **Internal config uses plain `bool`:** The internal config layer has no serialization concerns. Conversion in `convert_v1alpha4.go` is the single point where `nil *bool => true` and `*false => false` translation happens. Simpler consumption by addon action code.

3. **Safety-net defaulting in internal `SetDefaultsCluster`:** The all-false guard (`if !A && !B && !C && !D && !E`) ensures library callers who bypass conversion still get addons enabled. Conversion will always run before this check so the guard never fires in normal flow.

4. **Module path unchanged:** Binary rename is purely cosmetic (Makefile + cobra Use field). The Go module path stays `sigs.k8s.io/kind` to avoid breaking all imports.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## Next Phase Readiness

- Phase 1 Plan 2 (01-02): Creates the MetalLB addon action that will read `cfg.Addons.MetalLB` from the internal config. The `Addons` struct now exists in both layers and the conversion is wired. Plan 02 can proceed immediately.
- All subsequent addon phases (MetalLB, EnvoyGateway, MetricsServer, CoreDNS, Dashboard) depend on the `Addons` struct established here.

## Self-Check: PASSED

Files verified present:
- pkg/apis/config/v1alpha4/types.go — FOUND (contains `type Addons struct`)
- pkg/apis/config/v1alpha4/default.go — FOUND (contains `Addons`)
- pkg/internal/apis/config/types.go — FOUND (contains `type Addons struct`)
- pkg/internal/apis/config/convert_v1alpha4.go — FOUND (contains `Addons`)
- Makefile — FOUND (contains `KIND_BINARY_NAME?=kinder`)
- pkg/cmd/kind/root.go — FOUND (contains `Use: "kinder"`)
- bin/kinder — FOUND (binary built successfully)

Commits verified present:
- d800b450 — FOUND (feat(01-01): add Addons struct to both config layers)
- 20f3f141 — FOUND (feat(01-01): rename binary to kinder)
