---
phase: 42-multi-version-node-validation
plan: "01"
subsystem: config-validation
tags: [version-skew, image-override, explicit-image, validation, tdd]
dependency_graph:
  requires: []
  provides: [ExplicitImage-sentinel, validateVersionSkew]
  affects: [pkg/internal/apis/config, pkg/internal/apis/config/encoding, pkg/cluster/internal/create]
tech_stack:
  added: []
  patterns: [TDD-red-green, table-format-errors, pre-defaults-capture]
key_files:
  created:
    - pkg/cluster/internal/create/fixup_test.go
    - pkg/internal/apis/config/encoding/convert_test.go
  modified:
    - pkg/internal/apis/config/types.go
    - pkg/internal/apis/config/encoding/convert.go
    - pkg/cluster/internal/create/create.go
    - pkg/internal/apis/config/validate.go
    - pkg/internal/apis/config/validate_test.go
decisions:
  - Non-semver image tags (e.g. "latest") skip version-skew validation entirely rather than error, preserving backward compat with test/dev configs
  - ExplicitImage captured pre-defaults in encoding/convert.go because SetDefaultsCluster fills empty Image fields before Convertv1alpha4 runs
  - HA CP version mismatch produces a separate error from worker skew violations (different policy reasons)
metrics:
  duration_seconds: 312
  completed_date: "2026-04-08"
  tasks_completed: 2
  files_modified: 7
requirements_satisfied: [MVER-01, MVER-02, MVER-03]
---

# Phase 42 Plan 01: ExplicitImage Sentinel and Version-Skew Validation Summary

**One-liner:** ExplicitImage bool sentinel on internal Node preserves per-node images from --image flag override; validateVersionSkew rejects HA CP minor mismatch and workers >3 minor behind with table-format error output.

## What Was Built

### Task 1: ExplicitImage Sentinel (MVER-01)

Added `ExplicitImage bool` field to the internal `Node` struct in `pkg/internal/apis/config/types.go`. This field is set in `V1Alpha4ToInternal` by capturing which nodes had non-empty `Image` fields **before** `SetDefaultsCluster` fills in defaults — the key insight being that after defaults run, all nodes have an Image value and the explicit/defaulted distinction is lost.

`fixupOptions` in `create.go` now skips nodes with `ExplicitImage=true` when applying the `--image` flag override. Backward compatibility is preserved: configs without per-node images behave exactly as before.

The `DeepCopyInto` for `Node` uses `*out = *in` as its first operation, which copies all value-type fields including the new bool — no change to `zz_generated.deepcopy.go` was needed.

### Task 2: Version-Skew Validation (MVER-02, MVER-03)

Added `validateVersionSkew(nodes []Node) error` to `validate.go`, called from `Cluster.Validate()` after existing checks. The function:

1. Extracts version tags from node images via `imageTagVersion` (strips `@sha256:` digests, then takes the substring after the last `:`).
2. Parses each tag with `version.ParseSemantic`. If any tag is non-semver (e.g. `"latest"`), the entire skew check is skipped — non-semver tags are used in test/dev scenarios.
3. Checks HA control-plane nodes share the same minor version (etcd consistency requirement).
4. Checks workers are within 3 minor versions of the control-plane. All violations are collected and reported together in a table with remediation hints.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Non-semver image tags ("latest") caused unexpected validation errors**
- **Found during:** Task 2 GREEN phase
- **Issue:** Existing `TestClusterValidate` tests use `"myImage:latest"` as node images. The initial implementation returned an error for unparseable semver tags, breaking these tests.
- **Fix:** Changed `validateVersionSkew` to return `nil` (skip check) when any node's image tag fails semver parsing. Only truly missing tags (no colon at all) return an error.
- **Files modified:** `pkg/internal/apis/config/validate.go`
- **Commit:** `8c67c5fd`

## Test Coverage

| File | Tests Added |
|------|-------------|
| `pkg/cluster/internal/create/fixup_test.go` | `TestFixupOptions_ExplicitImageOverride` (3 sub-cases) |
| `pkg/internal/apis/config/encoding/convert_test.go` | `TestV1Alpha4ToInternal_ExplicitImage` (4 sub-cases) |
| `pkg/internal/apis/config/validate_test.go` | `TestValidateVersionSkew` (8 sub-cases) |

## Commits

| Hash | Type | Description |
|------|------|-------------|
| `e33502e7` | test | RED: failing tests for ExplicitImage and FixupOptions |
| `2baec8c6` | feat | GREEN: ExplicitImage sentinel + fixupOptions fix (MVER-01) |
| `f949ed83` | test | RED: failing tests for validateVersionSkew |
| `8c67c5fd` | feat | GREEN: validateVersionSkew implementation (MVER-02, MVER-03) |

## Verification Results

All verification commands pass:
- `go test ./pkg/internal/apis/config/... -count=1` — PASS
- `go test ./pkg/internal/apis/config/encoding/... -count=1` — PASS
- `go test ./pkg/cluster/internal/create -count=1` — PASS
- `go vet ./...` — clean

## Known Stubs

None. All functionality is fully implemented and wired.

## Threat Flags

None. The new validation runs at config-time (before any provisioning), does not introduce network endpoints, auth paths, or file access patterns.

## Self-Check: PASSED

- [x] `pkg/internal/apis/config/types.go` — ExplicitImage field present
- [x] `pkg/internal/apis/config/encoding/convert.go` — pre-defaults capture present
- [x] `pkg/cluster/internal/create/create.go` — ExplicitImage check present
- [x] `pkg/internal/apis/config/validate.go` — validateVersionSkew present
- [x] `pkg/cluster/internal/create/fixup_test.go` — TestFixupOptions tests present
- [x] `pkg/internal/apis/config/encoding/convert_test.go` — ExplicitImage tests present
- [x] `pkg/internal/apis/config/validate_test.go` — VersionSkew tests present
- [x] Commits e33502e7, 2baec8c6, f949ed83, 8c67c5fd — all present in git log
