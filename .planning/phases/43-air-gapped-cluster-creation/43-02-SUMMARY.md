---
phase: 43-air-gapped-cluster-creation
plan: "02"
subsystem: providers/create
tags: [air-gapped, docker, podman, nerdctl, fast-fail, images, addons]
dependency_graph:
  requires: ["43-01"]
  provides: ["air-gapped fast-fail in all providers", "addon image warning in create.go"]
  affects: ["pkg/cluster/internal/providers/docker", "pkg/cluster/internal/providers/podman", "pkg/cluster/internal/providers/nerdctl", "pkg/cluster/internal/create"]
tech_stack:
  added: []
  patterns: ["injectable function var for testability", "early-return air-gapped branch", "accumulate-all-missing error pattern"]
key_files:
  created:
    - pkg/cluster/internal/providers/docker/images_test.go
  modified:
    - pkg/cluster/internal/providers/docker/images.go
    - pkg/cluster/internal/providers/podman/images.go
    - pkg/cluster/internal/providers/nerdctl/images.go
    - pkg/cluster/internal/create/create.go
decisions:
  - "inspectImageFunc package-level var in docker provider enables test injection without requiring a real Docker daemon"
  - "nerdctl formatMissingImagesError takes binaryName parameter since nerdctl can use any binary name (nerdctl/containerd/etc)"
  - "Addon image warning uses addonImages.Len() > 0 guard to avoid printing NOTE with empty list when no addons are enabled"
metrics:
  duration: "~10 minutes"
  completed: "2026-04-09"
  tasks_completed: 3
  files_changed: 5
---

# Phase 43 Plan 02: Provider Air-Gapped Fast-Fail and Addon Image Warning Summary

Air-gapped fast-fail implemented in all three providers (docker, podman, nerdctl) and addon image pre-pull warning added to create.go.

## What Was Built

### Task 1: Air-Gapped Fast-Fail in All Three Providers (commit: 7e455484)

Modified `ensureNodeImages` in docker, podman, and nerdctl providers to branch on `cfg.AirGapped` at the top of the function. When air-gapped mode is active:

- Calls `checkAllImagesPresent` which runs `<runtime> inspect --type=image` for every image in `RequiredAllImages(cfg)` (node images + addon images + LB image)
- Accumulates ALL missing images into a slice — does not fail on first miss
- Returns a single `formatMissingImagesError` error listing every missing image with runtime-specific pre-load instructions
- Existing non-air-gapped pull logic is untouched

Docker provider uses an injectable `inspectImageFunc` package-level variable for testability. Nerdctl passes `binaryName` through to both `checkAllImagesPresent` and `formatMissingImagesError` since nerdctl may be invoked via different binary names.

### Task 2: Addon Image Warning in create.go (commit: bc0b31cc)

Added `common` package import and a warning block in `pkg/cluster/internal/create/create.go` immediately before `p.Provision`. When `!opts.Config.AirGapped` and at least one addon is enabled:

```
NOTE: The following addon images will be pulled during cluster creation.
      Pre-load them and use --air-gapped to skip pulls:
        <image1>
        <image2>
        ...
```

Suppressed when `--air-gapped` is set (the provider fast-fail path handles that case). The `AirGapped` propagation from `ClusterOptions` to `Config` was already in place from Plan 43-01.

### Task 3: Unit Tests for Docker Provider Fast-Fail (commit: 4b7a6a7c)

Created `pkg/cluster/internal/providers/docker/images_test.go` with 4 tests:

- `TestCheckAllImagesPresent_AllPresent`: nil error when all images found
- `TestCheckAllImagesPresent_SomeMissing`: error lists only the missing image, not the present one
- `TestCheckAllImagesPresent_AllMissing`: error lists all missing images (validates accumulate-all pattern)
- `TestFormatMissingImagesError`: verifies image names, "air-gapped mode" prefix, and docker pre-load instructions

All tests use `inspectImageFunc` injection with `t.Cleanup` restoration. No Docker daemon required.

## Verification Results

1. `go build ./...` — PASS
2. `go vet ./...` — PASS
3. All three provider images.go files have `AirGapped` branch at top of `ensureNodeImages`
4. `create.go` prints addon image warning only when `!AirGapped` and `addonImages.Len() > 0`
5. Missing image error accumulates all missing images before returning
6. `go test ./pkg/cluster/internal/providers/docker/...` — PASS (4 new tests + all existing pass)

## Deviations from Plan

### Auto-added: TestCheckAllImagesPresent_AllMissing

**Rule 2 - Missing Critical:** The plan specified tests for "all present" and "some missing" but not the critical "all missing" case which validates the accumulate-all-missing pattern (vs fail-on-first). Added `TestCheckAllImagesPresent_AllMissing` to explicitly cover the "collect all, not just first" requirement from the success criteria.

- **Found during:** Task 3
- **Fix:** Added third test function covering all-missing scenario verifying both image names appear in error
- **Files modified:** `pkg/cluster/internal/providers/docker/images_test.go`
- **Commit:** 4b7a6a7c

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1    | 7e455484 | feat(43-02): add air-gapped fast-fail to all three provider ensureNodeImages |
| 2    | bc0b31cc | feat(43-02): add addon image warning in create.go before Provision |
| 3    | 4b7a6a7c | test(43-02): add unit tests for docker provider air-gapped fast-fail |

## Known Stubs

None. All image lists and checks are wired to real data from `RequiredAllImages(cfg)` and `RequiredAddonImages(cfg)` which were implemented in Plan 43-01.

## Threat Flags

None. This plan adds no new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries.

## Self-Check: PASSED

- FOUND: `pkg/cluster/internal/providers/docker/images_test.go`
- FOUND: `pkg/cluster/internal/providers/docker/images.go`
- Commits found: 7e455484, bc0b31cc, 4b7a6a7c (all 43-02 commits present)
