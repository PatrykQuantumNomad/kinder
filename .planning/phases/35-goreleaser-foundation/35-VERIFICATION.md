---
phase: 35-goreleaser-foundation
verified: 2026-03-04T00:00:00Z
status: human_needed
score: 4/5 success criteria verified automatically
human_verification:
  - test: "Push a v* tag to GitHub and observe the release workflow"
    expected: "A GitHub Release page is created with five platform archives (kinder_VERSION_linux_amd64.tar.gz, kinder_VERSION_linux_arm64.tar.gz, kinder_VERSION_darwin_amd64.tar.gz, kinder_VERSION_darwin_arm64.tar.gz, kinder_VERSION_windows_amd64.zip), a checksums.txt, and an auto-generated changelog — without any manual intervention"
    why_human: "Requires actually pushing a git tag to GitHub and waiting for Actions to run; cannot be simulated locally"
---

# Phase 35: GoReleaser Foundation Verification Report

**Phase Goal:** Users can download pre-built kinder binaries for all platforms from GitHub Releases, produced by an automated pipeline that replaces cross.sh

**Verified:** 2026-03-04
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `goreleaser build --snapshot --clean` produces correct kinder binaries for linux/darwin amd64+arm64 and windows/amd64 — not upstream kind | VERIFIED | Build ran successfully: 5 binaries produced in `dist/` — `kinder_linux_amd64_v1/kinder`, `kinder_linux_arm64_v8.0/kinder`, `kinder_darwin_amd64_v1/kinder`, `kinder_darwin_arm64_v8.0/kinder`, `kinder_windows_amd64_v1/kinder.exe`. Binary named `kinder`, not `kind`. |
| 2 | `kinder version` on a snapshot binary shows the real git commit hash, not empty or "unknown" | VERIFIED | `./dist/kinder_darwin_arm64_v8.0/kinder version` outputs `kind v0.4.1-alpha+9768e4ff39c8e5 go1.26.0 darwin/arm64`. Commit hash `9768e4ff39c8e5` is non-empty, 14 characters, matches HEAD. |
| 3 | A tagged release on GitHub creates a Release page with five platform archives, a checksums.txt, and an auto-generated changelog — without any manual intervention | HUMAN NEEDED | Cannot verify without pushing a real tag to GitHub. Workflow config is correct (see criterion 4), but actual Release page creation requires CI execution. |
| 4 | GitHub Actions release workflow uses goreleaser-action replacing cross.sh and softprops; old cross.sh is retired in the same commit | VERIFIED | `release.yml` uses `goreleaser/goreleaser-action@v7` with `args: release --clean`. `softprops` absent. `hack/release/build/cross.sh` deleted (confirmed via `test ! -f`). All 3 referencing scripts updated atomically in commits `d0113fe7` + `7e6d46a5`. |
| 5 | `goreleaser check` passes with zero errors and zero deprecation warnings | VERIFIED | `goreleaser check` output: `• 1 configuration file(s) validated` with `• thanks for using GoReleaser!` — no errors, no deprecation warnings. |

**Score:** 4/5 criteria verified automatically (criterion 3 requires human)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.goreleaser.yaml` | GoReleaser v2 config for cross-platform kinder builds | VERIFIED | Exists, 102 lines, substantive. Contains `project_name: kinder`, `proxy: false`, 5-platform matrix, SHA-256 checksums, categorized changelog, release footer. |
| `.github/workflows/release.yml` | GoReleaser-based release pipeline triggered on v* tags | VERIFIED | Exists, 31 lines. Uses `goreleaser-action@v7`, `fetch-depth: 0`, `go-version-file: .go-version`, `GITHUB_TOKEN`, `contents: write` permission. No `softprops`, no `cross.sh`. |
| `Makefile` | goreleaser-check and goreleaser-snapshot targets | VERIFIED | Both targets present. `goreleaser-check` runs `goreleaser check`; `goreleaser-snapshot` runs `goreleaser build --snapshot --clean`. Both in `.PHONY`. |
| `hack/release/create.sh` | Manual release script updated for goreleaser | VERIFIED | Uses `goreleaser build --clean` (line 71). Follow-up echo instructions updated to reference GoReleaser workflow. |
| `hack/ci/build-all.sh` | CI build verification without cross.sh | VERIFIED | Uses `go build -v ./...`. Comment confirms cross-compile script retired. |
| `hack/ci/push-latest-cli/push-latest-cli.sh` | Disabled — upstream kind GCS script not used by fork | VERIFIED | Exits 0 immediately after `cd REPO_ROOT` with explanatory echo. Dead code preserved for reference. |
| `hack/release/build/cross.sh` | DELETED (retired) | VERIFIED | File does not exist. Confirmed with `test ! -f hack/release/build/cross.sh`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `.goreleaser.yaml` | `pkg/internal/kindversion/version.go` | ldflags `-X` injection of `gitCommit` | WIRED | Config contains `-X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .FullCommit }}`. Snapshot binary output confirms injection worked: hash `9768e4ff39c8e5` appears in `kinder version`. |
| `.goreleaser.yaml` | `Makefile` | ldflags path matches `KIND_VERSION_PKG` | WIRED | Both use `sigs.k8s.io/kind/pkg/internal/kindversion`. No path divergence. |
| `.github/workflows/release.yml` | `.goreleaser.yaml` | goreleaser-action reads config at repo root | WIRED | `goreleaser-action@v7` with `args: release --clean` — reads `.goreleaser.yaml` by convention. Config validated with `goreleaser check`. |
| `.github/workflows/release.yml` | GitHub Release API | `secrets.GITHUB_TOKEN` for asset upload | WIRED | Env var `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` present. `permissions: contents: write` set. |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| REL-01 | 35-01 | GoReleaser config produces cross-platform binaries (linux/darwin amd64+arm64, windows amd64) | SATISFIED | 5-platform matrix in `.goreleaser.yaml`. `goreleaser build --snapshot --clean` produced all 5 binaries. |
| REL-02 | 35-01 | GoReleaser generates SHA-256 checksums file for all release artifacts | SATISFIED | `checksum: name_template: "checksums.txt"`, `algorithm: sha256` in config. `goreleaser check` validates this. |
| REL-03 | 35-01 | GoReleaser generates automated changelog from git commit history | SATISFIED | `changelog` block with `use: git`, categorized groups (feat/fix/docs/other), filters for chore/test/ci/merge commits. |
| REL-04 | 35-01 | Release binaries show correct version metadata via `kinder version` | SATISFIED | Snapshot binary outputs `kind v0.4.1-alpha+9768e4ff39c8e5 go1.26.0 darwin/arm64`. Real commit hash injected via ldflags. |
| REL-05 | 35-02 | GitHub Actions release workflow uses goreleaser-action replacing cross.sh + softprops | SATISFIED | `release.yml` uses goreleaser-action@v7. cross.sh deleted. softprops absent. All referencing scripts updated atomically. |
| REL-06 | 35-01 | GoReleaser explicitly sets `gomod.proxy: false` and `project_name: kinder` (fork safety) | SATISFIED | Both set in `.goreleaser.yaml`. Comments explain the fork-safety rationale (prevents resolution of upstream `sigs.k8s.io/kind` via module proxy). |

All 6 requirements satisfied. No orphaned requirements found.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `hack/ci/push-latest-cli/push-latest-cli.sh` | 30–74 | Dead code after `exit 0` (gsutil upload loop) | Info | Intentional: kept for reference per plan decision. No runtime impact. |

No blockers or warnings found.

### Human Verification Required

#### 1. Tagged Release Creates GitHub Release Page

**Test:** Push a version tag to GitHub — e.g. `git tag v0.4.1 && git push origin v0.4.1`

**Expected:**
- GitHub Actions workflow `Release` triggers on the tag push
- Workflow completes successfully (green check)
- GitHub Releases page shows a new release `v0.4.1` containing exactly:
  - `kinder_0.4.1_linux_amd64.tar.gz`
  - `kinder_0.4.1_linux_arm64.tar.gz`
  - `kinder_0.4.1_darwin_amd64.tar.gz`
  - `kinder_0.4.1_darwin_arm64.tar.gz`
  - `kinder_0.4.1_windows_amd64.zip`
  - `checksums.txt`
- Release body contains an auto-generated changelog with categorized commit sections
- Release body contains the install footer with curl one-liner
- No manual steps were required after the tag push

**Why human:** Requires actually pushing a tag to GitHub and waiting for the Actions pipeline to execute. Cannot be simulated locally — the goreleaser-action step needs GITHUB_TOKEN with write permissions to create the Release.

### Gaps Summary

No gaps. All automated success criteria are satisfied. The single human-needed item (criterion 3) is a deployment verification that requires pushing a real tag — the code, config, and workflow wiring are all correct and verified.

---

_Verified: 2026-03-04_
_Verifier: Claude (gsd-verifier)_
