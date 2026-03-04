---
phase: 28-parallel-execution
plan: "01"
subsystem: actions
tags: [concurrency, sync, race-free, caching]
dependency_graph:
  requires: []
  provides: [race-free-nodes-cache, x-sync-dep]
  affects: [pkg/cluster/internal/create/actions/action.go]
tech_stack:
  added: [golang.org/x/sync v0.19.0]
  patterns: [sync.OnceValues for exactly-once caching]
key_files:
  created:
    - pkg/cluster/internal/create/actions/action_test.go
  modified:
    - pkg/cluster/internal/create/actions/action.go
    - go.mod
    - go.sum
decisions:
  - "sync.OnceValues used over RWMutex-based cachedData: eliminates TOCTOU race, single-call guarantee"
  - "golang.org/x/sync added as direct dep via go.mod edit + go mod download (not go mod tidy which removes unused)"
metrics:
  duration: "~10 minutes"
  completed: "2026-03-03"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 4
---

# Phase 28 Plan 01: sync.OnceValues Nodes() Cache Summary

Race-free ActionContext.Nodes() cache using sync.OnceValues, with concurrent access test and golang.org/x/sync v0.19.0 as direct module dependency.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Replace cachedData with sync.OnceValues in ActionContext | d8a41dff | action.go |
| 2 | Add golang.org/x/sync dependency and Nodes() concurrency race test | 638905b4 | go.mod, go.sum, action_test.go |

## What Was Built

**Task 1 - Replace cachedData with sync.OnceValues:**

The original `ActionContext` used a `cachedData` struct with `sync.RWMutex` for node caching. This had a TOCTOU race: two goroutines could both see `nil` nodes, both release the RLock, and both call `ListNodes`. The fix replaces the entire `cachedData` struct with a single `nodesOnce` field of type `func() ([]nodes.Node, error)` initialized via `sync.OnceValues`. The `Nodes()` method is now a single line: `return ac.nodesOnce()`. `sync.OnceValues` guarantees exactly-once execution regardless of concurrent callers.

**Task 2 - golang.org/x/sync dependency and race tests:**

Added `golang.org/x/sync v0.19.0` to `go.mod` as a direct dependency (required by Plan 02 for `errgroup.Group.SetLimit(3)`). Created `action_test.go` with two tests:
- `TestNodes_ConcurrentAccess`: 10 goroutines call `Nodes()` simultaneously; all get 1 node, no error, and the race detector reports no data races.
- `TestNodes_CachesResult`: verifies the second call returns the same slice memory (pointer equality of first element), confirming caching works.

## Verification Results

```
go build ./...                                       -- PASS (no errors)
go vet ./...                                         -- PASS (no warnings)
go test ./pkg/cluster/internal/create/actions/...   -- PASS (all packages)
CGO_ENABLED=1 go test -race ./...actions/ -count=1  -- PASS (no data races)
grep "sync.OnceValues" action.go                     -- FOUND (3 occurrences)
grep "golang.org/x/sync" go.mod                     -- FOUND (v0.19.0)
grep -c "cachedData" action.go                       -- 0 (fully removed)
```

## Deviations from Plan

**1. [Rule 2 - Missing Critical Functionality] go mod download instead of go get + go mod tidy**

- **Found during:** Task 2
- **Issue:** `go get golang.org/x/sync@latest` followed by `go mod tidy` removes the dependency since no code in the current commit uses it. The must_haves artifact requires `golang.org/x/sync` in go.mod at the end of this plan.
- **Fix:** Added `golang.org/x/sync v0.19.0` directly to go.mod's `require` block, then ran `go mod download golang.org/x/sync` to populate go.sum. Plan 02 will run `go mod tidy` after importing `errgroup`, at which point the dependency becomes naturally retained.
- **Files modified:** go.mod, go.sum
- **Commit:** 638905b4

## Self-Check: PASSED

All files verified:
- FOUND: pkg/cluster/internal/create/actions/action.go
- FOUND: pkg/cluster/internal/create/actions/action_test.go
- FOUND: go.mod
- FOUND: .planning/phases/28-parallel-execution/28-01-SUMMARY.md

All commits verified:
- FOUND: d8a41dff (refactor(28-01): replace cachedData with sync.OnceValues)
- FOUND: 638905b4 (feat(28-01): add golang.org/x/sync dependency and Nodes() race tests)
