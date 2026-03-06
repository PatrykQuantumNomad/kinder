---
phase: 41-network-create-flow-integration-and-website
plan: 03
subsystem: website
tags: [astro, starlight, documentation, diagnostics, known-issues]

# Dependency graph
requires:
  - phase: 41-01
    provides: subnet clash detection check (check #18)
  - phase: 41-02
    provides: ApplySafeMitigations wiring into create flow
provides:
  - Comprehensive Known Issues page documenting all 18 diagnostic checks
  - Sidebar navigation entry for known-issues
  - Cross-link from Troubleshooting page to Known Issues
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "What/Why/Platforms/Fix documentation structure for each diagnostic check"

key-files:
  created:
    - kinder-site/src/content/docs/known-issues.md
  modified:
    - kinder-site/astro.config.mjs
    - kinder-site/src/content/docs/cli-reference/troubleshooting.md

key-decisions:
  - "Organized checks by category matching allChecks registry order: Runtime, Docker, Tools, GPU, Kernel, Security, Platform, Network"
  - "GPU checks section prefaced with note that they are informational and only relevant for NVIDIA GPU addon users"
  - "SELinux section notes Fedora-only warning behavior matching code implementation"
  - "Automatic Mitigations section documents tier-1 constraints and current empty state"

patterns-established:
  - "What/Why/Platforms/Fix: consistent four-section structure for each diagnostic check documentation"

requirements-completed: [SITE-01]

# Metrics
duration: 2min
completed: 2026-03-06
---

# Phase 41 Plan 03: Known Issues Documentation Summary

**Comprehensive Known Issues page documenting all 18 kinder doctor diagnostic checks across 8 categories with What/Why/Fix structure and sidebar integration**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-06T18:12:30Z
- **Completed:** 2026-03-06T18:15:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Created 487-line Known Issues page documenting all 18 diagnostic checks across 8 categories
- Each check section includes What it detects, Why it matters, Platforms, and How to fix
- Added Known Issues to sidebar navigation before Changelog for top-level discoverability
- Cross-linked Troubleshooting page to Known Issues with tip admonition
- Documented Automatic Mitigations system with tier-1 constraints

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Known Issues documentation page** - `eddc3ebe` (feat)
2. **Task 2: Add Known Issues to sidebar and cross-link from Troubleshooting** - `8854fe22` (feat)

## Files Created/Modified
- `kinder-site/src/content/docs/known-issues.md` - Comprehensive Known Issues page with all 18 checks documented
- `kinder-site/astro.config.mjs` - Added known-issues slug to sidebar before changelog
- `kinder-site/src/content/docs/cli-reference/troubleshooting.md` - Added cross-link tip to Known Issues page

## Decisions Made
- Organized checks by category matching the `allChecks` registry order in `check.go`: Runtime, Docker, Tools, GPU, Kernel, Security, Platform, Network
- Prefaced GPU checks section with a note that they are informational and only relevant for NVIDIA GPU addon users
- Documented SELinux Fedora-only warning behavior, matching the actual code implementation that returns ok on non-Fedora enforcing
- WSL2 section explains the multi-signal detection approach to avoid false positives on Azure VMs
- Automatic Mitigations section documents that no tier-1 mitigations currently exist but infrastructure is wired

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- This is the final plan in Phase 41 and the final phase of v2.1
- All 18 diagnostic checks are implemented, integrated, and documented
- v2.1 milestone (Known Issues & Proactive Diagnostics) is complete

## Self-Check: PASSED

- FOUND: kinder-site/src/content/docs/known-issues.md (487 lines)
- FOUND: kinder-site/dist/known-issues/index.html
- FOUND: eddc3ebe (Task 1 commit)
- FOUND: 8854fe22 (Task 2 commit)
- 18 check sections across 8 categories verified
- Astro build completes with 21 pages

---
*Phase: 41-network-create-flow-integration-and-website*
*Completed: 2026-03-06*
