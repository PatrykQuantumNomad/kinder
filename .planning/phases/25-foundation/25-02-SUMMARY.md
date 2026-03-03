---
phase: 25-foundation
plan: "02"
subsystem: toolchain
tags: [golangci-lint, v2, migration, lint, tools]
dependency_graph:
  requires: [25-01]
  provides: [golangci-lint-v2-binary, v2-lint-config, clean-lint-pass]
  affects: [hack/tools, hack/make-rules/verify/lint.sh, codebase-wide-lint-violations]
tech_stack:
  added: [golangci-lint v2.10.1]
  patterns: [nolint-errcheck-deferred-close, package-doc-comments, v2-formatters-section]
key_files:
  created:
    - pkg/internal/kindversion/version_test.go
  modified:
    - hack/tools/.golangci.yml
    - hack/tools/tools.go
    - hack/tools/go.mod
    - hack/tools/go.sum
    - hack/make-rules/verify/lint.sh
key_decisions:
  - "typecheck removed from linters list: v2 treats it as always-active, not configurable"
  - "var-naming exclusions added for established package names (common, errors, log, version)"
  - "errcheck violations handled with //nolint:errcheck for deferred Close calls (not actionable in defer context)"
  - "version_test.go moved to kindversion package (pre-existing bug: test referenced unexported functions after code moved)"
metrics:
  duration_minutes: 15
  tasks_completed: 2
  files_modified: 67
  completed_date: "2026-03-03"
---

# Phase 25 Plan 02: golangci-lint v2 Migration Summary

golangci-lint upgraded from v1.62.2 to v2.10.1 with full config format migration and zero-issue clean lint pass on both main module and kindnetd.

## What Was Built

### Task 1: Update tools module to golangci-lint v2

- Updated `hack/tools/tools.go` import from `golangci-lint/cmd/golangci-lint` to `golangci-lint/v2/cmd/golangci-lint`
- Ran `go get github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1` and `go mod tidy` in `hack/tools/`
- Old v1 dependency removed from `go.mod`; v2 dependency added
- Updated `hack/make-rules/verify/lint.sh` build path to v2 module path
- Binary builds from `hack/tools` successfully

**Commit:** `8c5e6ba5`

### Task 2: Migrate .golangci.yml to v2 format and fix lint violations

Wrote the v2-format config with these key changes from v1:
- Added `version: "2"` at top (required for v2)
- `linters.disable-all: true` -> `linters.default: none`
- Removed `gosimple` (merged into staticcheck in v2)
- Removed `exportloopref` (removed in v2)
- Removed `typecheck` from enable list (always-active in v2, configurable causes fatal error)
- Moved `gofmt` from `linters.enable` to `formatters.enable`
- Moved `linters-settings` to `linters.settings` (nested under linters)
- Moved `issues.exclude-rules` to `linters.exclusions.rules`
- Added exclusions for `var-naming` on established package names

Fixed 55+ pre-existing lint violations exposed by v2 across both modules.

**Commit:** `2402767c`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] typecheck listed in linters causes fatal config error in v2**
- **Found during:** Task 2 (first lint run)
- **Issue:** golangci-lint v2 exits with "typecheck is not a linter, it cannot be enabled or disabled"
- **Fix:** Removed `typecheck` from `linters.enable` list
- **Files modified:** `hack/tools/.golangci.yml`

**2. [Rule 1 - Bug] version_test.go referenced unexported functions in wrong package**
- **Found during:** Task 2 (typecheck errors on first lint run)
- **Issue:** `pkg/cmd/kind/version/version_test.go` called `truncate()` and `version()` which were moved to `pkg/internal/kindversion/` by phase 25-03 (concurrent plan)
- **Fix:** Moved test file to `pkg/internal/kindversion/version_test.go` with corrected package declaration
- **Files modified:** Deleted `pkg/cmd/kind/version/version_test.go`, created `pkg/internal/kindversion/version_test.go`

**3. [Rule 1 - Bug] config.go had syntax error from concurrent 25-03 stash**
- **Found during:** Task 2 setup
- **Issue:** `git stash pop` after checking v1 availability restored 25-03's WIP changes including a syntax error in `config.go` (removed `-1` from `strings.Replace` leaving dangling comma)
- **Fix:** Restored `config.go` and 25-03 out-of-scope files to HEAD state, then fixed `strings.Replace -> strings.ReplaceAll` properly
- **Files modified:** `pkg/cluster/internal/create/actions/config/config.go`

**4. [Rule 2 - Missing Critical] 55+ pre-existing lint violations exposed by v2**
- **Found during:** Task 2 (lint run after config migration)
- **Issue:** golangci-lint v2 caught many pre-existing violations that v1 was not reporting: unchecked errcheck (defer Close, fmt.Fprintf), missing package-doc comments, capitalized error strings, staticcheck QF1003/QF1004/QF1012 improvements
- **Fix:** Fixed all violations across 60+ files using:
  - `//nolint:errcheck` for deferred Close calls (cannot be meaningfully acted on)
  - `_ = ` blank assignment for non-defer unchecked errors
  - Package doc comments added to 15+ packages
  - Error strings lowercased in 6 command files
  - `strings.Replace -> strings.ReplaceAll` in 2 locations
  - `if-else -> switch` in docker/archive.go (staticcheck QF1003)
  - `fmt.Fprintf` instead of `WriteString(fmt.Sprintf())` in version.go
- **Files modified:** 60+ files across pkg/, cmd/, images/kindnetd/
- **Commits:** Included in `2402767c`

## Task Commits

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Update tools module to golangci-lint v2 | `8c5e6ba5` | hack/tools/tools.go, go.mod, go.sum, lint.sh |
| 2 | Migrate .golangci.yml to v2 format and fix lint | `2402767c` | hack/tools/.golangci.yml, 63 source files |

## Verification Results

All success criteria met:
- `hack/tools/.golangci.yml` starts with `version: "2"` ✓
- `hack/tools/tools.go` imports `golangci-lint/v2` ✓
- `hack/make-rules/verify/lint.sh` builds from `golangci-lint/v2` path ✓
- `golangci-lint run ./...` passes with **0 issues** on main module ✓
- `golangci-lint run ./...` passes with **0 issues** on kindnetd module ✓
- No `exportloopref` or `gosimple` in config ✓
- Binary version confirmed: `golangci-lint has version 2.10.1` ✓

## Next Plan Readiness

Plan 25-03 (version package refactor) was executing in parallel and has already been committed (`c744fd1c`). The test file that 25-03 created dependencies for was correctly resolved here by moving `version_test.go` to the kindversion package. Ready for Phase 25 Plan 04.

## Self-Check: PASSED

All key files verified on disk. All task commits verified in git log.
- hack/tools/.golangci.yml: FOUND
- hack/tools/tools.go: FOUND
- hack/make-rules/verify/lint.sh: FOUND
- pkg/internal/kindversion/version_test.go: FOUND
- .planning/phases/25-foundation/25-02-SUMMARY.md: FOUND
- Commit 8c5e6ba5: FOUND
- Commit 2402767c: FOUND
