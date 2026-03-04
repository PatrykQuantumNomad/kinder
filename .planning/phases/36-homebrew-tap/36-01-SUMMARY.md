---
phase: 36-homebrew-tap
plan: "01"
subsystem: infra
tags: [homebrew, goreleaser, github-actions, pat, cask, tap, homebrew-casks]

# Dependency graph
requires:
  - phase: 35-goreleaser-foundation
    provides: GoReleaser pipeline producing tagged GitHub Releases with platform archives — prerequisite for cask publishing
provides:
  - homebrew-kinder tap repo on GitHub (PatrykQuantumNomad/homebrew-kinder, public)
  - homebrew_casks section in .goreleaser.yaml targeting the tap repo with skip_upload:auto
  - HOMEBREW_TAP_TOKEN wired into release.yml for cross-repo push auth
  - HOMEBREW_TAP_TOKEN PAT created and stored as kinder repo secret
affects: [36-homebrew-tap]

# Tech tracking
tech-stack:
  added:
    - "homebrew_casks (GoReleaser v2 publisher for Homebrew Cask formula generation)"
  patterns:
    - "Use homebrew_casks (not deprecated brews:) for GoReleaser v2 cask publishing"
    - "skip_upload:auto prevents cask publishing for pre-release tags (e.g. v2.0.0-rc1)"
    - "Cross-repo push via fine-grained PAT stored as repo secret — GITHUB_TOKEN has no cross-repo write permissions"
    - "Casks/ directory (not Formula/) — Casks go in Casks/, formulas in Formula/"

key-files:
  created:
    - .planning/phases/36-homebrew-tap/36-USER-SETUP.md
  modified:
    - .goreleaser.yaml
    - .github/workflows/release.yml

key-decisions:
  - "Used homebrew_casks (not deprecated brews:) — brews: was deprecated in GoReleaser v2.10"
  - "skip_upload:auto for pre-release safety — prevents publishing casks for rc/beta tags"
  - "Casks/ directory (not Formula/) — cask publishing convention"
  - "HOMEBREW_TAP_TOKEN as fine-grained PAT scoped to homebrew-kinder repo Contents:write — minimal privilege"

patterns-established:
  - "homebrew_casks: use skip_upload:auto, Casks/ directory, and a dedicated PAT token"

# Metrics
duration: ~15min
completed: 2026-03-04
---

# Phase 36 Plan 01: Tap Repo and Cask Config Summary

**Homebrew tap repo created, GoReleaser homebrew_casks config with skip_upload:auto, HOMEBREW_TAP_TOKEN wired into release workflow and stored as repo secret**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-04T18:00:00Z
- **Completed:** 2026-03-04T23:30:00Z
- **Tasks:** 2 (1 auto + 1 checkpoint:human-action completed)
- **Files modified:** 2

## Accomplishments

- Created `PatrykQuantumNomad/homebrew-kinder` public GitHub repo with initial README — this is the Homebrew tap that `brew tap patrykquantumnomad/kinder` will use
- Added `homebrew_casks` section to `.goreleaser.yaml` with `skip_upload:auto`, `Casks/` directory, and token template pointing to `HOMEBREW_TAP_TOKEN`
- Added `HOMEBREW_TAP_TOKEN` env var to goreleaser-action step in `.github/workflows/release.yml`
- User created fine-grained PAT with Contents:write on homebrew-kinder and stored it as `HOMEBREW_TAP_TOKEN` repo secret in kinder

## Task Commits

Each task was committed atomically:

1. **Task 1: Create homebrew-kinder tap repository and configure GoReleaser cask publishing** - `11876e07` (feat)
2. **Task 2: Create PAT and store as repository secret** - checkpoint:human-action, completed by user

**Plan metadata:** TBD (docs: complete tap repo and cask config plan)

## Files Created/Modified

- `.goreleaser.yaml` - Added `homebrew_casks` section after `release:` block: targets PatrykQuantumNomad/homebrew-kinder, Casks/ directory, skip_upload:auto, token from HOMEBREW_TAP_TOKEN env var
- `.github/workflows/release.yml` - Added `HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}` to goreleaser-action env block with inline comment

## Decisions Made

- **homebrew_casks (not brews:):** `brews:` was deprecated in GoReleaser v2.10. `homebrew_casks:` is the correct publisher for v2 cask generation.
- **skip_upload:auto:** Prevents GoReleaser from publishing a cask for pre-release tags (e.g. `v2.0.0-rc1`). Only stable tags trigger a cask push. Without this, every RC would spam the tap repo with broken casks.
- **Casks/ directory:** Homebrew Cask convention. Formulas go in `Formula/`, casks in `Casks/`. Using the wrong directory would break `brew install`.
- **Fine-grained PAT scoped to homebrew-kinder only:** `GITHUB_TOKEN` cannot push to a different repo. A fine-grained PAT with `Contents: write` on only `homebrew-kinder` follows the principle of minimal privilege.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

**External services required manual configuration.** See [36-USER-SETUP.md](./36-USER-SETUP.md) for:
- HOMEBREW_TAP_TOKEN: Fine-grained PAT with Contents:write on homebrew-kinder
- Steps to store PAT as repository secret in kinder
- Verification commands to confirm cask was published after a tagged release

**Status: COMPLETE** — PAT created and stored as repo secret.

## Next Phase Readiness

- All Phase 36 infrastructure is now in place: tap repo exists, GoReleaser cask config is wired, PAT secret is stored
- Phase 36 success criteria 1 and 2 (brew install + auto cask update) will be verified after the next tagged release
- Phase 36 success criteria 3 (installation page) was completed in Plan 36-02
- Phase 37 (NVIDIA GPU Addon) can begin — it is independent of the distribution pipeline

---
*Phase: 36-homebrew-tap*
*Completed: 2026-03-04*
