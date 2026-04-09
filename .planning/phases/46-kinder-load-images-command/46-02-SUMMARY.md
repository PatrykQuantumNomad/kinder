---
phase: 46
plan: "02"
subsystem: load-images
tags: [load, images, provider-abstraction, smart-load, nerdctl, finch, docker-desktop-27]
dependency_graph:
  requires: [46-01]
  provides: [kinder-load-images-subcommand]
  affects: [pkg/cmd/kind/load/load.go, pkg/cmd/kind/load/images/images.go]
tech_stack:
  added: []
  patterns: [provider-binary-abstraction, smart-load-skip, factory-closure-fallback]
key_files:
  created:
    - pkg/cmd/kind/load/images/images.go
  modified:
    - pkg/cmd/kind/load/load.go
decisions:
  - "providerBinaryName reads KIND_EXPERIMENTAL_PROVIDER directly for nerdctl/finch/nerdctl.lima — provider.Name() always returns 'nerdctl' so the env var distinguishes actual binaries"
  - "loadImage uses LoadImageArchiveWithFallback with os.Open factory (not direct io.Reader) to support 2-attempt fallback without rewind"
  - "save() and imageID() take binaryName as parameter — avoids hardcoding 'docker', enables podman/finch/nerdctl.lima at call sites"
  - "removeDuplicates, checkIfImageReTagRequired, sanitizeImage copied from docker-image package — they are package-private there, no import cycle alternative"
metrics:
  duration: "~2 minutes"
  completed: "2026-04-09"
  tasks_completed: 2
  files_changed: 2
---

# Phase 46 Plan 02: kinder load images Subcommand Summary

Provider-abstracted `kinder load images` subcommand with smart-load skip and Docker Desktop 27+ fallback via `LoadImageArchiveWithFallback` factory pattern.

## What Was Built

Created `pkg/cmd/kind/load/images/images.go` — a new `kinder load images` subcommand that:

1. **Provider abstraction** — `providerBinaryName()` resolves the actual host binary for image operations. For docker and podman, `provider.Name()` is the binary. For the nerdctl provider, reads `KIND_EXPERIMENTAL_PROVIDER` directly to return `finch`, `nerdctl.lima`, or `nerdctl` as appropriate.

2. **Smart-load skip (LOAD-04)** — Before saving any tar, checks each candidate node: if image ID and tag already match, skip the node with a log message. If image ID exists but tag is missing, re-tag in place. Only nodes that truly lack the image are loaded.

3. **Docker Desktop 27+ fallback (LOAD-02)** — `loadImage()` calls `nodeutils.LoadImageArchiveWithFallback` with an `os.Open` factory closure. The factory is called twice if the first `ctr images import --all-platforms` fails with a "content digest" error — the second attempt omits `--all-platforms` to handle the Docker Desktop containerd image store behavior.

4. **Registration** — `pkg/cmd/kind/load/load.go` imports the new package and registers `images.NewCommand` as the third `load` subcommand alongside `docker-image` and `image-archive`.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create images.go subcommand | fad1adb3 | pkg/cmd/kind/load/images/images.go (created) |
| 2 | Register images subcommand in load.go | 2fbbbeb9 | pkg/cmd/kind/load/load.go (modified) |

## Verification

All 9 verification criteria confirmed:
- `go build ./pkg/cmd/kind/load/...` — PASS
- `go build -o /dev/null ./` full binary — PASS
- `go vet ./pkg/cmd/kind/load/...` — PASS
- `save()` uses `binaryName` parameter — PASS
- `imageID()` uses `binaryName` parameter — PASS
- `providerBinaryName` handles finch/nerdctl.lima — PASS
- Smart-load skip log present — PASS
- `LoadImageArchiveWithFallback` used — PASS
- All 3 subcommands registered in load.go — PASS

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries introduced.

## Self-Check: PASSED

- `pkg/cmd/kind/load/images/images.go` — FOUND
- `pkg/cmd/kind/load/load.go` — FOUND (modified)
- Commit fad1adb3 — verified via git log
- Commit 2fbbbeb9 — verified via git log
