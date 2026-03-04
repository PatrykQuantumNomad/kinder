---
phase: 30-foundation-fixes
plan: 01
subsystem: docs
tags: [astro, starlight, documentation, addons, local-registry, cert-manager, kinder-site]

# Dependency graph
requires: []
provides:
  - Landing page Comparison component with all 7 addons (including Local Registry and cert-manager)
  - Landing page addon cards grouped into Core Addons and Optional Addons sections
  - Quick-start page with all 7 addon verifications, --profile tip, and kinder doctor section
  - Configuration page with all 7 addon fields documented with core/optional grouping
  - Sidebar Guides and CLI Reference sections with placeholder pages
  - 6 placeholder content files for future phases 32 and 33
affects: [31-addon-page-depth, 32-cli-reference, 33-tutorials, 34-verification-polish]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Core vs optional addon grouping used consistently across landing page, quick-start, and configuration pages"
    - "Collapsible sidebar groups for Addons, Guides, CLI Reference"
    - "Placeholder pages with Coming soon callout for future content"

key-files:
  created:
    - kinder-site/src/content/docs/guides/tls-web-app.md
    - kinder-site/src/content/docs/guides/hpa-auto-scaling.md
    - kinder-site/src/content/docs/guides/local-dev-workflow.md
    - kinder-site/src/content/docs/cli-reference/profile-comparison.md
    - kinder-site/src/content/docs/cli-reference/json-output.md
    - kinder-site/src/content/docs/cli-reference/troubleshooting.md
  modified:
    - kinder-site/src/components/Comparison.astro
    - kinder-site/src/content/docs/index.mdx
    - kinder-site/src/content/docs/quick-start.md
    - kinder-site/src/content/docs/configuration.md
    - kinder-site/astro.config.mjs

key-decisions:
  - "Group addons as core (MetalLB, Metrics Server, CoreDNS) vs optional (Envoy Gateway, Headlamp, Local Registry, cert-manager) consistently across all three pages"
  - "Sidebar groups Addons, Guides, CLI Reference are all collapsed by default"
  - "Placeholder pages use :::note[Coming soon] Starlight callout — minimal content, no stub sections"

patterns-established:
  - "Core addons: metalLB, metricsServer, coreDNSTuning — always-on essentials"
  - "Optional addons: envoyGateway, dashboard, localRegistry, certManager — powerful extras, disable with --profile minimal"

requirements-completed: [FOUND-01, FOUND-02, FOUND-03, FOUND-04]

# Metrics
duration: 3min
completed: 2026-03-04
---

# Phase 30 Plan 01: Foundation Fixes Summary

**Updated kinder site to document all 7 addons across landing page, quick-start, configuration, and sidebar — bringing v1.3-v1.4 features (Local Registry, cert-manager) into documentation parity**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-04T14:35:41Z
- **Completed:** 2026-03-04T14:38:25Z
- **Tasks:** 3
- **Files modified:** 11 (5 modified, 6 created)

## Accomplishments
- Comparison.astro updated to 8 items per column — Local Registry and cert-manager now appear in both kind and kinder columns
- index.mdx reorganised with Core Addons (MetalLB, Metrics Server, CoreDNS) and Optional Addons (Envoy Gateway, Headlamp, Local Registry, cert-manager) card groups and updated description meta
- quick-start.md expanded with all 7 addon verifications grouped as core/optional, --profile tip callout, and kinder doctor section
- configuration.md restructured with complete v1alpha4 example, core/optional field tables, and localRegistry/certManager documented
- astro.config.mjs sidebar updated: Addons group collapsed, Guides and CLI Reference groups added with 3 placeholder slugs each
- 6 placeholder content files created in guides/ and cli-reference/ directories

## Task Commits

Each task was committed atomically:

1. **Task 1: Update landing page, Comparison component, and sidebar with placeholder pages** - `c32f29ea` (feat)
2. **Task 2: Update quick-start page with all 7 addon verifications, --profile tip, and doctor section** - `5d01d0f8` (feat)
3. **Task 3: Update configuration page with all 7 addon fields and core/optional grouping** - `11f25172` (feat)

**Plan metadata:** (to be recorded in final metadata commit)

## Files Created/Modified
- `kinder-site/src/components/Comparison.astro` - Added Local Registry and cert-manager entries (8 items per column)
- `kinder-site/src/content/docs/index.mdx` - Updated description meta, reorganised addon cards into Core/Optional sections
- `kinder-site/src/content/docs/quick-start.md` - Full rewrite with all 7 addon verifications, --profile tip, kinder doctor section
- `kinder-site/src/content/docs/configuration.md` - Added complete YAML example, core/optional field tables, localRegistry and certManager fields
- `kinder-site/astro.config.mjs` - Added collapsed to Addons group; added Guides and CLI Reference groups
- `kinder-site/src/content/docs/guides/tls-web-app.md` - Placeholder guide page
- `kinder-site/src/content/docs/guides/hpa-auto-scaling.md` - Placeholder guide page
- `kinder-site/src/content/docs/guides/local-dev-workflow.md` - Placeholder guide page
- `kinder-site/src/content/docs/cli-reference/profile-comparison.md` - Placeholder CLI reference page
- `kinder-site/src/content/docs/cli-reference/json-output.md` - Placeholder CLI reference page
- `kinder-site/src/content/docs/cli-reference/troubleshooting.md` - Placeholder CLI reference page

## Decisions Made
- Core vs optional grouping applied consistently across all three pages (landing, quick-start, configuration) for unified mental model
- All sidebar groups (Addons, Guides, CLI Reference) use `collapsed: true` — users can expand on demand
- Placeholder pages contain only title frontmatter and a single `:::note[Coming soon]` callout — no stub sections to maintain

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 31 (Addon Page Depth) can begin: all 7 addon slugs are referenced in the sidebar
- Phase 32 (CLI Reference) can begin: cli-reference/ directory and placeholder pages exist
- Phase 33 (Tutorials) can begin: guides/ directory and placeholder pages exist
- Phase 34 (Verification) can begin after 31-33 complete

---
*Phase: 30-foundation-fixes*
*Completed: 2026-03-04*
