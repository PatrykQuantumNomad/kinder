---
phase: 53-addon-version-audit-bumps-sync-05
plan: 05
subsystem: infra
tags: [metallb, addon-version-audit, hold-verify, changelog]

requires:
  - phase: 53-addon-version-audit-bumps-sync-05
    provides: "53-04 Envoy Gateway bump complete; MetalLB hold-verify is plan 05"

provides:
  - "Hold-verify record for MetalLB v0.15.3 — upstream still latest as of 2026-05-10"
  - "CHANGELOG stub confirming MetalLB hold under v2.4"

affects: ["53-07-offlinereadiness-consolidation", "Phase 58 UAT"]

tech-stack:
  added: []
  patterns: ["Hold-verify pattern: probe upstream API at execute time, record verbatim output, either confirm hold or halt on divergence"]

key-files:
  created:
    - ".planning/phases/53-addon-version-audit-bumps-sync-05/53-05-SUMMARY.md"
  modified:
    - "kinder-site/src/content/docs/changelog.md"

key-decisions:
  - "MetalLB hold at v0.15.3 reaffirmed — upstream probe on 2026-05-10 confirms v0.15.3 is still the latest release; no v0.16.x published"
  - "No Go source change in this plan — metallb.go remains untouched; offlinereadiness entries consolidated in 53-07"

patterns-established:
  - "Hold-verify pattern: probe GitHub releases API, record verbatim top-5 listing, compare against pinned tag, document disposition in SUMMARY"

duration: 2min
completed: 2026-05-10
---

# Phase 53 Plan 05: MetalLB Hold-Verify Summary

**MetalLB hold at v0.15.3 reaffirmed — GitHub releases API probe on 2026-05-10 confirms no newer release exists (top-5 listing shows v0.15.3 as latest non-prerelease tag)**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-05-10T16:40:00Z
- **Completed:** 2026-05-10T16:42:00Z
- **Tasks:** 1
- **Files modified:** 2 (SUMMARY.md + changelog.md)

## Accomplishments

- Probed upstream `metallb/metallb` GitHub releases API and confirmed v0.15.3 is still the latest release
- Verified no v0.16.x or newer release exists in top-5 listing
- Documented hold disposition in CHANGELOG stub under v2.4

## HOLD CONFIRMED — MetalLB v0.15.3

Probed upstream `metallb/metallb` GitHub releases API on 2026-05-10:

**Probe command:**
```bash
LATEST_TAG=$(curl -s 'https://api.github.com/repos/metallb/metallb/releases/latest' | jq -r '.tag_name')
echo "Latest MetalLB upstream release: $LATEST_TAG"

curl -s 'https://api.github.com/repos/metallb/metallb/releases?per_page=5' | jq -r '.[] | "\(.tag_name)\t\(.published_at)\t\(.draft)\t\(.prerelease)"'
```

**Verbatim output:**
```
Latest MetalLB upstream release: v0.15.3

Top 5 releases:
v0.15.3	2025-12-04T15:48:02Z	false	false
metallb-chart-0.15.3	2025-12-04T15:20:55Z	false	false
v0.15.2	2025-06-04T11:19:42Z	false	false
metallb-chart-0.15.2	2025-06-04T10:50:26Z	false	false
metallb-chart-0.15.1	2025-06-04T09:03:36Z	false	false
```

**`tag_name` of latest release:** v0.15.3  
**Published:** 2025-12-04T15:48:02Z  
**No v0.16.x present** in top-5 listing — confirmed.

The pinned version in `pkg/cluster/internal/create/actions/installmetallb/metallb.go` is unchanged at v0.15.3 (both `controller` and `speaker` images). No source change.

The MetalLB hold from REQUIREMENTS.md "Documented Holds" section is reaffirmed: "Latest available; no v0.16 released."

This plan made no Go source changes. The MetalLB image entries in `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` are consolidated in 53-07 (no per-plan offlinereadiness edits per CONTEXT.md decision).

## Task Commits

1. **Task 1: Hold-verify probe + CHANGELOG stub** - see final commit hash below (docs)

## Files Created/Modified

- `.planning/phases/53-addon-version-audit-bumps-sync-05/53-05-SUMMARY.md` — this file; records probe output and hold disposition
- `kinder-site/src/content/docs/changelog.md` — CHANGELOG stub confirming MetalLB hold under v2.4

## Decisions Made

- MetalLB hold at v0.15.3 reaffirmed — upstream probe confirms v0.15.3 is still the latest release; no v0.16.x published. Hold criterion ("Latest available; no v0.16 released") satisfied.
- No Go source change — `pkg/cluster/internal/create/actions/installmetallb/metallb.go` untouched. `offlinereadiness.go` entries consolidated in 53-07.

## Deviations from Plan

None — plan executed exactly as written. Upstream probe confirmed hold path; CHANGELOG stub and SUMMARY committed atomically.

## Issues Encountered

None.

## Next Phase Readiness

- 53-05 complete — MetalLB hold reaffirmed with documented upstream evidence
- 53-06 (Metrics Server hold/verify) is next
- 53-07 (offlinereadiness consolidation) will handle MetalLB image entries — no per-plan offlinereadiness edits in this plan

---
*Phase: 53-addon-version-audit-bumps-sync-05*
*Completed: 2026-05-10*
