---
phase: 35
plan: 01
subsystem: release-pipeline
tags: [goreleaser, cross-platform, binary-distribution, release-tooling]
dependency_graph:
  requires: []
  provides: [goreleaser-config, makefile-goreleaser-targets, snapshot-validation]
  affects: [.github/workflows/release.yml]
tech_stack:
  added: [GoReleaser v2.14.1]
  patterns: [declarative-release-config, ldflags-version-injection, binary-only-archives]
key_files:
  created: [.goreleaser.yaml]
  modified: [Makefile]
decisions:
  - "Use flags: [-trimpath] not ldflags — -trimpath is a go build compiler flag, not a linker flag"
  - "gomod.proxy: false and project_name: kinder are mandatory fork-safety settings"
  - "goreleaser build --snapshot only builds binaries; checksums are produced by goreleaser release"
  - "gitCommitCount omitted from GoReleaser ldflags — no built-in template; safe to omit for tagged releases"
metrics:
  duration_minutes: 2
  completed_date: "2026-03-04"
  tasks_completed: 2
  files_changed: 2
---

# Phase 35 Plan 01: GoReleaser Foundation Summary

**One-liner:** GoReleaser v2.14.1 config with fork-safe settings (project_name: kinder, gomod.proxy: false) producing 5-platform binaries via ldflags-injected gitCommit, validated by snapshot build showing `kind v0.4.1-alpha+ba923e0a51fb49 go1.25.7 darwin/arm64`.

## What Was Built

Created `.goreleaser.yaml` as the foundation of the kinder release pipeline, with two Makefile targets for local developer workflow.

### Files Created

**`.goreleaser.yaml`** — GoReleaser v2 configuration:
- `project_name: kinder` — explicit fork safety (prevents inference from `sigs.k8s.io/kind` module path)
- `gomod.proxy: false` — prevents GoReleaser from resolving upstream kind from Go module proxy
- 5-platform build matrix: linux/darwin amd64+arm64, windows amd64 (arm64 excluded)
- `flags: [-trimpath]` + `ldflags: [-buildid=, -w, -X gitCommit]` for reproducible, versioned binaries
- Binary-only tar.gz archives (zip for Windows), sha256 checksums, categorized git changelog
- `release.mode: replace` for safe workflow re-runs on same tag

### Files Modified

**`Makefile`** — Added two targets:
- `goreleaser-check` — runs `goreleaser check` to validate config before any PR
- `goreleaser-snapshot` — runs `goreleaser build --snapshot --clean` for local dry-runs
- Both added to `.PHONY` line

## Verification Results

| Check | Result |
|-------|--------|
| `goreleaser check` exits 0 | PASS — zero errors, zero deprecation warnings |
| 5 platform binaries in dist/ | PASS — linux_amd64, linux_arm64, darwin_amd64, darwin_arm64, windows_amd64 |
| Binary named `kinder` (not `kind`) | PASS — verified in dist/kinder_darwin_arm64_v8.0/kinder |
| Version output format | PASS — `kind v0.4.1-alpha+ba923e0a51fb49 go1.25.7 darwin/arm64` |
| `project_name: kinder` in config | PASS |
| `proxy: false` in config | PASS |
| `make goreleaser-check` target | PASS |
| `make goreleaser-snapshot` target | PASS |

## Success Criteria Status

| Requirement | Status |
|-------------|--------|
| REL-01: 5-platform binary matrix | COMPLETE |
| REL-02: sha256 checksums configured | COMPLETE (config present; activates on full release) |
| REL-03: Changelog groups (feat/fix/docs) | COMPLETE |
| REL-04: kinder version shows real git commit hash | COMPLETE — ba923e0a51fb49 injected via ldflag |
| REL-06: gomod.proxy: false + project_name: kinder | COMPLETE |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed -trimpath placement: ldflags → flags**
- **Found during:** Task 2 (snapshot build)
- **Issue:** `-trimpath` is a `go build` compiler flag, not a linker flag. Placing it in `ldflags` causes the linker to reject it: `flag provided but not defined: -trimpath`
- **Fix:** Moved `-trimpath` from the `ldflags` list to a separate `flags` list. The Makefile uses `-trimpath` as a go build flag via `KIND_BUILD_FLAGS?=-trimpath -ldflags=...`, which is the correct pattern.
- **Files modified:** `.goreleaser.yaml`
- **Commit:** 2b8d62fa

**2. [Rule 3 - Blocking] Installed goreleaser (not available in PATH)**
- **Found during:** Task 1 (pre-verification)
- **Issue:** `goreleaser` binary not found in PATH — cannot validate config or run snapshot
- **Fix:** Installed via `brew install goreleaser` (v2.14.1, matching the research-verified version)
- **Files modified:** None (system tool install)

### Observations (not deviations)

- `goreleaser build --snapshot` does not produce `checksums.txt` — checksums are part of the archive phase which only runs during `goreleaser release`. The checksum configuration in `.goreleaser.yaml` is correct and will activate on tagged release runs.
- `gitCommitCount` is correctly omitted from GoReleaser ldflags per plan analysis — GoReleaser has no built-in template for it, and it is unused in tagged release version strings.

## Commits

| Hash | Message |
|------|---------|
| ba923e0a | feat(35-01): add GoReleaser v2 config for cross-platform kinder builds |
| 2b8d62fa | feat(35-01): add Makefile goreleaser targets and fix -trimpath ldflags placement |

## Self-Check: PASSED

- FOUND: .goreleaser.yaml
- FOUND: Makefile (modified)
- FOUND: 35-01-SUMMARY.md
- FOUND commit: ba923e0a (feat: add GoReleaser v2 config)
- FOUND commit: 2b8d62fa (feat: add Makefile targets and fix -trimpath)
