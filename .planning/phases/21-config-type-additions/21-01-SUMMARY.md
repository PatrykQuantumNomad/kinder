---
phase: 21-config-type-additions
plan: 01
subsystem: config-api
tags: [config, v1alpha4, types, addons, local-registry, cert-manager]
dependency_graph:
  requires: []
  provides: [CFG-01, CFG-02, CFG-03]
  affects: [pkg/apis/config/v1alpha4, pkg/internal/apis/config]
tech_stack:
  added: []
  patterns: [nil-guard-deepcopy, boolPtrTrue-defaulting, boolVal-conversion]
key_files:
  created:
    - pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-new-fields.yaml
  modified:
    - pkg/apis/config/v1alpha4/types.go
    - pkg/apis/config/v1alpha4/default.go
    - pkg/apis/config/v1alpha4/zz_generated.deepcopy.go
    - pkg/internal/apis/config/types.go
    - pkg/internal/apis/config/convert_v1alpha4.go
    - pkg/internal/apis/config/encoding/load_test.go
decisions:
  - LocalRegistry and CertManager both default to true (on-by-default opt-out, consistent with existing addon pattern)
  - Plain bool in internal types, *bool in v1alpha4 public API (matching MetalLB/Dashboard pattern)
metrics:
  duration: ~10 min
  completed: 2026-03-03
  tasks: 2
  files_modified: 5
  files_created: 1
---

# Phase 21 Plan 01: Config Type Additions Summary

**One-liner:** LocalRegistry *bool and CertManager *bool added to v1alpha4 Addons struct and wired through defaults, deepcopy, internal types, and conversion with full test coverage.

## What Was Built

Added two new addon fields to the kinder config API, completing the five-location config pipeline pattern established for MetalLB, Dashboard, and other addons. Phase 22 (Local Registry) and Phase 23 (cert-manager) can now reference `opts.Config.Addons.LocalRegistry` and `opts.Config.Addons.CertManager` at compile time.

### Files Modified

**pkg/apis/config/v1alpha4/types.go** — Added LocalRegistry *bool and CertManager *bool to the Addons struct after Dashboard, with yaml/json camelCase tags and +optional comments.

**pkg/apis/config/v1alpha4/default.go** — Added `boolPtrTrue(&obj.Addons.LocalRegistry)` and `boolPtrTrue(&obj.Addons.CertManager)` calls in SetDefaultsCluster, defaulting both fields to true when nil.

**pkg/apis/config/v1alpha4/zz_generated.deepcopy.go** — Added nil-guard copy blocks for LocalRegistry and CertManager in the Addons DeepCopyInto function, preventing pointer aliasing on deep copy.

**pkg/internal/apis/config/types.go** — Added LocalRegistry bool and CertManager bool to the internal Addons struct (plain bool, no serialization tags per internal type convention).

**pkg/internal/apis/config/convert_v1alpha4.go** — Added `LocalRegistry: boolVal(in.Addons.LocalRegistry)` and `CertManager: boolVal(in.Addons.CertManager)` entries to the out.Addons literal, converting *bool to bool via the existing boolVal helper.

**pkg/internal/apis/config/encoding/load_test.go** — Added TestLoadCurrent case for new fixture; added LocalRegistry and CertManager default assertions in Test 1 of TestAddonsDefaults; added Test 4 verifying explicit false is respected.

### Files Created

**pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-new-fields.yaml** — New test fixture with `addons.localRegistry: false` and `addons.certManager: false` to prove the parse pipeline handles explicit false for both new fields.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | f38c0fe9 | feat(21-01): add LocalRegistry and CertManager fields to type/default/deepcopy |
| 2 | d5cf24ed | feat(21-01): wire conversion and add test coverage for LocalRegistry and CertManager |

## Verification Results

- `go build ./...` passed with no errors
- `go test ./pkg/internal/apis/config/encoding/ -v -run "TestLoadCurrent|TestAddonsDefaults"` passed (19/19 subtests)
- `go test ./...` passed with no regressions across all packages

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

Files exist:
- FOUND: pkg/apis/config/v1alpha4/types.go (contains LocalRegistry *bool)
- FOUND: pkg/apis/config/v1alpha4/default.go (contains boolPtrTrue calls)
- FOUND: pkg/apis/config/v1alpha4/zz_generated.deepcopy.go (contains nil-guard blocks)
- FOUND: pkg/internal/apis/config/types.go (contains LocalRegistry bool)
- FOUND: pkg/internal/apis/config/convert_v1alpha4.go (contains boolVal conversions)
- FOUND: pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-new-fields.yaml
- FOUND: pkg/internal/apis/config/encoding/load_test.go (updated)

Commits exist:
- FOUND: f38c0fe9
- FOUND: d5cf24ed
