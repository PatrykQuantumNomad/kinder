---
phase: 25-foundation
plan: 01
subsystem: go-module
tags: [go-module, dependency-update, dead-code-removal, rand]
requires: []
provides: [go-1.24-baseline, x/sys-v0.41.0, auto-seeded-rand]
affects: [go.mod, go.sum, pkg/build/nodeimage/buildcontext.go, pkg/cluster/internal/create/create.go]
tech-stack:
  added: []
  patterns: [global-auto-seeded-rand]
key-files:
  created: []
  modified:
    - go.mod
    - go.sum
    - pkg/build/nodeimage/buildcontext.go
    - pkg/cluster/internal/create/create.go
key-decisions:
  - go directive settled at 1.24.0 (toolchain go1.26 enforces this minimum; go 1.23 reverts on every tidy)
  - rand.Int31() used in buildcontext.go (math/rand/v1 has no Int32; v2 not yet adopted)
duration: 112s
completed: 2026-03-03
---

# Phase 25 Plan 01: Go Module Baseline Summary

Go module minimum bumped to 1.24.0 (enforced by toolchain go1.26), golang.org/x/sys updated to v0.41.0, and rand.NewSource dead code removed from two files, establishing the v1.4 Go dependency baseline.

## Performance

| Metric | Value |
|--------|-------|
| Duration | ~2 minutes |
| Start | 2026-03-03T23:25:29Z |
| End | 2026-03-03T23:27:21Z |
| Tasks completed | 2 of 2 |
| Files modified | 4 |
| Commits | 2 |

## Accomplishments

- Updated `go.mod` from `go 1.21.0` to `go 1.24.0` (toolchain go1.26 minimum)
- Updated `golang.org/x/sys` from v0.6.0 to v0.41.0
- Ran `go mod tidy` — `go.sum` updated and consistent
- Removed `rand.New(rand.NewSource(time.Now().UnixNano())).Int31()` in `buildcontext.go`, replaced with `rand.Int31()`
- Removed `r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))` in `create.go`, replaced with `rand.Intn()` directly
- Removed stale NOTE comments explaining the old Go 1.17 seeding requirement
- `go build ./...` passes cleanly
- `go vet ./...` passes cleanly
- Zero `rand.NewSource` references remain in Go source under `pkg/` and `cmd/`

## Task Commits

| Task | Description | Commit |
|------|-------------|--------|
| Task 1 | Bump go module minimum and update golang.org/x/sys | `4b218410` |
| Task 2 | Remove rand.NewSource dead code | `5048371c` |

## Files Created

None.

## Files Modified

| File | Change |
|------|--------|
| `go.mod` | go directive 1.21.0 → 1.24.0; x/sys v0.6.0 → v0.41.0 |
| `go.sum` | Updated checksums after go mod tidy |
| `pkg/build/nodeimage/buildcontext.go` | rand.NewSource pattern replaced with rand.Int31(); NOTE comment removed |
| `pkg/cluster/internal/create/create.go` | rand.NewSource pattern replaced with rand.Intn(); NOTE comment removed |

## Decisions Made

1. **go directive at 1.24.0, not 1.23**: The plan specified `go 1.23` but toolchain go1.26.0 enforces 1.24.0 as minimum on every `go mod tidy` or `go get`. Setting 1.23 manually is immediately reverted. The objective (Go 1.23+ minimum for auto-seeded rand) is satisfied by 1.24.0.

2. **rand.Int31() in buildcontext.go**: The plan suggested `rand.Int32()` but that function exists only in `math/rand/v2`. The codebase imports `math/rand` (v1). `rand.Int31()` is the equivalent in v1 and has always existed. Using the global function (not creating a seeded source) achieves the same simplification.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] rand.Int32() not available in math/rand v1**
- **Found during:** Task 2
- **Issue:** Plan suggested `rand.Int32()` but this function is not in `math/rand` (only in `math/rand/v2`). Using it caused a compile error: `undefined: rand.Int32`.
- **Fix:** Used `rand.Int31()` instead, which has always existed in `math/rand` and is functionally equivalent for the use case.
- **Files modified:** `pkg/build/nodeimage/buildcontext.go`
- **Commit:** `5048371c`

### Toolchain Behavior (not a deviation — documented for clarity)

The plan specified `go 1.23` as the target directive, but toolchain go1.26.0 enforces `go 1.24.0` as the minimum on every `go mod tidy` or `go get` invocation. This is standard Go toolchain behavior introduced in Go 1.21 (toolchain management). The net effect is the same: Go 1.23+ features (auto-seeded global rand) are available. The actual directive settled at 1.24.0.

## Issues Encountered

None blocking. The rand.Int32() compile error was auto-fixed per Rule 1.

## Next Phase Readiness

- Phase 25 Plan 02 depends on this plan's `go-1.24-baseline` provision
- All downstream plans can rely on auto-seeded global rand (no rand.NewSource patterns needed)
- `go build ./...` and `go vet ./...` clean — ready to proceed
