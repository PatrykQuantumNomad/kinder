---
phase: 46-kinder-load-images-command
plan: 01
subsystem: nodeutils
tags: [load-images, docker-desktop-compat, containerd, fallback]
dependency_graph:
  requires: []
  provides: [LoadImageArchiveWithFallback, runImport, isContentDigestError]
  affects: [pkg/cluster/nodeutils/util.go]
tech_stack:
  added: []
  patterns: [factory-pattern-reader, two-attempt-fallback, stdlib-errors-As]
key_files:
  created: []
  modified:
    - pkg/cluster/nodeutils/util.go
    - pkg/cluster/nodeutils/util_test.go
decisions:
  - stderrors alias (stdlib "errors") avoids conflict with sigs.k8s.io/kind/pkg/errors import
  - openArchive factory pattern required because tar stream is consumed on read and cannot be rewound
  - isContentDigestError checks RunError.Output before falling back to err.Error() string
  - LoadImageArchive unchanged to preserve existing public API for image-archive command
metrics:
  duration: ~1 min
  completed: "2026-04-09"
  tasks_completed: 2
  files_modified: 2
---

# Phase 46 Plan 01: LoadImageArchiveWithFallback Summary

**One-liner:** Docker Desktop 27+ containerd image store fallback via two-attempt ctr import with factory-pattern reader and RunError.Output inspection.

## What Was Built

Added three functions to `pkg/cluster/nodeutils/util.go` that implement transparent Docker Desktop 27+ compatibility for image loading:

1. **`runImport`** (unexported helper) — encapsulates `ctr images import` invocation, builds args with or without `--all-platforms` based on a bool parameter.

2. **`isContentDigestError`** (unexported helper) — detects the specific `"content digest: not found"` failure from `ctr images import --all-platforms` on Docker Desktop 27+ with the containerd image store enabled. Checks `exec.RunError.Output` field first (the combined stdout+stderr), then falls back to the error string.

3. **`LoadImageArchiveWithFallback`** (exported) — loads an image archive with automatic fallback. Takes an `openArchive func() (io.ReadCloser, error)` factory so a fresh reader can be obtained for each attempt (tar streams cannot be rewound). Attempt 1 uses `--all-platforms`; if that fails with a content-digest error, attempt 2 retries without. All other errors are returned immediately.

Added `stderrors "errors"` alias to the import block to enable `stderrors.As` for `*exec.RunError` type assertion without conflicting with the existing `sigs.k8s.io/kind/pkg/errors` import.

Added `TestIsContentDigestError` (5 table-driven cases) to `util_test.go`.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| Task 1: Add functions to util.go | `599e1fdd` | feat(46-01): add LoadImageArchiveWithFallback and runImport to nodeutils |
| Task 2: Add unit tests | `a5797699` | test(46-01): add TestIsContentDigestError for nodeutils fallback logic |

## Deviations from Plan

None — plan executed exactly as written.

## Verification Results

- `go build ./pkg/cluster/nodeutils/...` — PASS
- `go vet ./pkg/cluster/nodeutils/...` — PASS (no issues)
- `go test ./pkg/cluster/nodeutils/... -v` — PASS (TestParseSnapshotter + 5 TestIsContentDigestError subtests)
- `LoadImageArchive` unchanged (confirmed by grep)

## Known Stubs

None. This plan adds pure logic with no UI rendering paths.

## Threat Flags

None. No new network endpoints, auth paths, file access patterns, or schema changes.

## Self-Check: PASSED

- `pkg/cluster/nodeutils/util.go` — FOUND, contains LoadImageArchiveWithFallback, runImport, isContentDigestError
- `pkg/cluster/nodeutils/util_test.go` — FOUND, contains TestIsContentDigestError with 5 cases
- Commit `599e1fdd` — FOUND
- Commit `a5797699` — FOUND
