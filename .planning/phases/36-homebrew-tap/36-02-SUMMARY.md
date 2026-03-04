---
phase: 36-homebrew-tap
plan: "02"
subsystem: docs
tags: [homebrew, brew, installation, documentation, goreleaser, github-releases]

# Dependency graph
requires:
  - phase: 35-goreleaser-foundation
    provides: GoReleaser archive naming convention (kinder_VERSION_OS_ARCH.tar.gz) used in updated download URLs
provides:
  - Installation page with Homebrew as primary macOS install method
  - Updated binary download URLs pointing to GitHub Releases (no stale hardcoded versioned URLs)
affects: [36-homebrew-tap, 37-nvidia-gpu-addon]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Direct users to GitHub Releases page for downloads rather than hardcoded versioned curl URLs"

key-files:
  created: []
  modified:
    - kinder-site/src/content/docs/installation.md

key-decisions:
  - "Direct users to GitHub Releases page for downloads instead of hardcoded versioned URLs — avoids staleness when new releases are cut"
  - "Homebrew (macOS) section placed BEFORE Download Pre-built Binary section — establishes it as the preferred macOS install method"

patterns-established:
  - "Release downloads: link to releases page, not version-pinned curl URLs — prevents docs rot"

# Metrics
duration: ~5min
completed: 2026-03-04
---

# Phase 36 Plan 02: Installation Page Update Summary

**Homebrew install instructions as primary macOS method, stale cross.sh download URLs replaced with GitHub Releases links**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-04T18:07:00Z
- **Completed:** 2026-03-04T18:07:05Z
- **Tasks:** 2 (1 auto + 1 checkpoint:human-verify approved)
- **Files modified:** 1

## Accomplishments

- Added "Homebrew (macOS)" section as the first install method on the installation page with `brew install patrykquantumnomad/kinder/kinder`
- Replaced old cross.sh-era download URLs (`kinder-darwin-arm64`, no extension) with links to the GitHub Releases page
- Simplified download instructions to be version-agnostic — no more hardcoded versioned curl URLs that go stale

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Homebrew section and update download URLs in installation.md** - `9a20e9d7` (docs)
2. **Task 2: Verify installation page content** - checkpoint:human-verify, approved by user

**Plan metadata:** TBD (docs: complete installation page update plan)

## Files Created/Modified

- `kinder-site/src/content/docs/installation.md` - Added Homebrew section before binary download section; replaced stale cross.sh URLs with GitHub Releases links; simplified macOS/Linux/Windows download instructions

## Decisions Made

- Directed users to GitHub Releases page instead of providing hardcoded versioned `curl` download URLs. GoReleaser archive filenames embed the version (`kinder_VERSION_darwin_arm64.tar.gz`), so any hardcoded URL would become stale the moment a new release is cut. Linking to the releases page is perpetually correct.
- Homebrew section placed before the "Download Pre-built Binary" section to signal it as the recommended macOS install path, consistent with the phase goal of making `brew install` the primary method.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required for this plan. (HOMEBREW_TAP_TOKEN PAT setup is tracked in Plan 36-01.)

## Next Phase Readiness

- Installation page is updated and reflects Homebrew as the primary macOS install method
- Waiting on Plan 36-01 (tap repo creation + PAT secret) to complete before Phase 36 is fully done
- Once 36-01 completes, Phase 36 success criteria 1 and 3 are both satisfied

---
*Phase: 36-homebrew-tap*
*Completed: 2026-03-04*
