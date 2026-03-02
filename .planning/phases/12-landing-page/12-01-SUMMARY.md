---
phase: 12-landing-page
plan: 01
subsystem: ui
tags: [astro, starlight, mdx, landing-page, splash-template, clipboard-api]

# Dependency graph
requires:
  - phase: 11-documentation-content
    provides: Addon doc pages at /addons/metallb/, /addons/envoy-gateway/, /addons/metrics-server/, /addons/coredns/, /addons/headlamp/
  - phase: 10-website-theme
    provides: CSS custom properties (--sl-color-accent, --sl-color-black, etc.) used in component styles
provides:
  - "InstallCommand.astro: copy-to-clipboard install command display component"
  - "Comparison.astro: two-column kind vs kinder feature comparison grid"
  - "index.mdx: complete landing page with splash template, hero, install section, comparison, and addon cards"
affects: [phase-13-if-any]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "not-content class on custom Astro component roots to prevent Starlight markdown style interference"
    - "navigator.clipboard.writeText() for copy-to-clipboard without external library"
    - "CSS Grid with responsive @media breakpoint for two-column to single-column stacking"

key-files:
  created:
    - kinder-site/src/components/InstallCommand.astro
    - kinder-site/src/components/Comparison.astro
  modified:
    - kinder-site/src/content/docs/index.mdx

key-decisions:
  - "not-content class required on custom component root elements to prevent Starlight .sl-markdown-content styles from overriding component layout"
  - "CardGrid without stagger attribute — 5 cards is an odd count where stagger looks unbalanced"
  - "All addon doc links use root-relative paths with trailing slash to match Starlight routing"

patterns-established:
  - "Custom Astro components in kinder-site/src/components/ with scoped styles using CSS custom properties from Starlight theme"
  - "not-content class pattern for escaping Starlight markdown content styles"

# Metrics
duration: 2min
completed: 2026-03-02
---

# Phase 12 Plan 01: Landing Page Summary

**Splash landing page with InstallCommand clipboard component and Comparison grid — visitor immediately sees kinder's value and can copy the install command or navigate to docs.**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-02T01:33:40Z
- **Completed:** 2026-03-02T01:34:45Z
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 replaced)

## Accomplishments
- Created InstallCommand.astro with navigator.clipboard.writeText() copy button and scoped styles
- Created Comparison.astro with responsive CSS Grid two-column kind vs kinder feature list
- Replaced default Starlight index.mdx with kinder landing page using splash template, hero CTAs (Get Started + GitHub), Install section, kind vs kinder comparison, and 5 addon Cards
- Full site builds with zero errors — 10 pages, Pagefind search index intact

## Task Commits

Each task was committed atomically:

1. **Task 1: Create InstallCommand and Comparison Astro components** - `bf65e3c6` (feat)
2. **Task 2: Replace index.mdx with complete landing page** - `7f62dcb1` (feat)

**Plan metadata:** `07918162` (docs: complete landing page plan)

## Files Created/Modified
- `kinder-site/src/components/InstallCommand.astro` - Install command display with copy-to-clipboard button using navigator.clipboard.writeText
- `kinder-site/src/components/Comparison.astro` - Two-column responsive CSS Grid comparison of kind vs kinder feature sets
- `kinder-site/src/content/docs/index.mdx` - Landing page with splash template, hero (tagline + Get Started + GitHub), Install section, kind vs kinder comparison, and 5 addon CardGrid

## Decisions Made
- Used `not-content` class on custom component root elements to prevent Starlight `.sl-markdown-content` styles from overriding component flex/grid layouts
- `CardGrid` without `stagger` — 5 cards at odd count looks unbalanced with stagger enabled
- Root-relative paths with trailing slash for all addon links (`/addons/metallb/`) to match Starlight static routing

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Landing page complete and site builds with zero errors
- All 5 addon doc pages exist and links resolve
- Ready for any final QA or deployment steps

## Self-Check: PASSED

All files verified present and both task commits confirmed in git history.

---
*Phase: 12-landing-page*
*Completed: 2026-03-02*
