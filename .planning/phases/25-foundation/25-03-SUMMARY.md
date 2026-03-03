---
phase: 25-foundation
plan: "03"
subsystem: version-package-layering
tags: [layer-violation, refactor, internal-package, linker-flags]
dependency_graph:
  requires: ["25-01"]
  provides: ["pkg/internal/kindversion", "clean-import-direction"]
  affects: ["pkg/cluster/provider.go", "pkg/cmd/kind/version/version.go", "pkg/cmd/kind/root.go", "Makefile", "hack/release/create.sh"]
tech_stack:
  added: ["pkg/internal/kindversion (new Go package)"]
  patterns: ["internal package boundary enforcement", "cmd -> cluster -> internal import direction"]
key_files:
  created:
    - pkg/internal/kindversion/version.go
  modified:
    - pkg/cmd/kind/version/version.go
    - pkg/cluster/provider.go
    - pkg/cmd/kind/root.go
    - Makefile
    - hack/release/create.sh
key_decisions:
  - "Version() and DisplayVersion() moved to pkg/internal/kindversion to enforce cmd -> internal dependency direction"
  - "pkg/cmd/kind/version/version.go retained as thin cobra command wrapper importing from kindversion"
  - "Makefile KIND_VERSION_PKG updated to pkg/internal/kindversion so linker -X flags inject commit metadata into correct package"
metrics:
  duration: "3m 53s"
  completed: "2026-03-03"
  tasks_completed: 2
  tasks_total: 2
  files_created: 1
  files_modified: 5
---

# Phase 25 Plan 03: Version Package Layer Violation Fix Summary

Fixed layer violation where pkg/cluster/provider.go imported from pkg/cmd/kind/version/ (CLI layer) by extracting version constants and functions into a new pkg/internal/kindversion/ package, establishing clean import direction: cmd -> cluster -> internal.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create pkg/internal/kindversion and split version package | c744fd1c | pkg/internal/kindversion/version.go, pkg/cmd/kind/version/version.go |
| 2 | Update imports, linker flags, and release script | 4c66214d | pkg/cluster/provider.go, pkg/cmd/kind/root.go, Makefile, hack/release/create.sh |

## What Was Built

### pkg/internal/kindversion/version.go (new)
New internal package containing all version logic previously in the CLI layer:
- `Version() string` - exported function returning semantic version string
- `DisplayVersion() string` - exported function returning formatted display version
- `version()` - unexported helper building version from components
- `truncate()` - unexported helper for string truncation
- `versionCore`, `versionPreRelease`, `gitCommit`, `gitCommitCount` - vars/constants (linker targets)

### pkg/cmd/kind/version/version.go (rewritten)
Reduced to thin cobra command wrapper:
- Only `NewCommand()` remains
- Imports `sigs.k8s.io/kind/pkg/internal/kindversion`
- Delegates to `kindversion.DisplayVersion()` and `kindversion.Version()`

### Import direction enforced
- `pkg/cluster/provider.go`: changed import from `pkg/cmd/kind/version` to `pkg/internal/kindversion`
- `pkg/cmd/kind/root.go`: added `kindversion` import, changed `version.Version()` to `kindversion.Version()` for cobra's `Version` field
- `Makefile`: `KIND_VERSION_PKG` updated to `sigs.k8s.io/kind/pkg/internal/kindversion`
- `hack/release/create.sh`: `VERSION_FILE` updated to `./pkg/internal/kindversion/version.go`

## Verification Results

All 7 plan verification checks passed:
1. `go build ./...` succeeds
2. `grep -r 'pkg/cmd/kind/version' pkg/cluster/` returns nothing
3. `pkg/internal/kindversion/version.go` exists with `Version()` and `DisplayVersion()`
4. `pkg/cmd/kind/version/version.go` has only `NewCommand()`
5. `make build && ./bin/kinder version` prints `kind v0.3.0-alpha.18+4c66214d3cbb7e go1.25.7 darwin/arm64`
6. Makefile `KIND_VERSION_PKG` references `pkg/internal/kindversion`
7. `hack/release/create.sh` `VERSION_FILE` references `pkg/internal/kindversion`

## Success Criteria Met

- No import of pkg/cmd/kind/version from pkg/cluster/ ✓
- Version functions live in pkg/internal/kindversion/ ✓
- Linker -X flags target the new package path ✓
- `kinder version` prints correct version string ✓
- Release script updated for new file location ✓

## Deviations from Plan

None - plan executed exactly as written.

## Next Steps

Plan 25-04 is unblocked. The clean import direction (cmd -> cluster -> internal) is now enforced for the version subsystem.
