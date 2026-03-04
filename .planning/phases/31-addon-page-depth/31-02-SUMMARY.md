---
phase: 31-addon-page-depth
plan: 02
subsystem: docs
tags: [headlamp, local-registry, cert-manager, troubleshooting, kubernetes, addons]

# Dependency graph
requires:
  - phase: 30-foundation-fixes
    provides: all 7 addon pages created with core structure and configuration sections
provides:
  - Headlamp navigation guide covering 5 dashboard areas (Workloads, Logs, Events, Config, Nodes)
  - Headlamp troubleshooting section with invalid token error diagnosis and fix
  - Local Registry multi-image build-and-push workflow with curl v2/_catalog verification
  - Local Registry image cleanup guidance and tag listing
  - Local Registry troubleshooting section with ImagePullBackOff diagnosis
  - cert-manager wildcard certificate example with duration/renewBefore and ClusterIssuer distinction
  - cert-manager troubleshooting section covering webhook timing and Issuer vs ClusterIssuer causes
affects:
  - 31-addon-page-depth plan 03 and beyond (remaining addon pages)
  - future guides referencing local registry or cert-manager workflows

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Troubleshooting sections follow Symptom/Cause/Fix structure consistently
    - Multi-step workflows verified with curl catalog endpoint
    - ClusterIssuer vs Issuer distinction emphasized via :::note callout

key-files:
  created: []
  modified:
    - kinder-site/src/content/docs/addons/headlamp.md
    - kinder-site/src/content/docs/addons/local-registry.md
    - kinder-site/src/content/docs/addons/cert-manager.md

key-decisions:
  - "Troubleshooting sections use Symptom/Cause/Fix structure for scannable problem resolution"
  - "Local Registry multi-image workflow uses two separate docker build+push commands to show the pattern clearly"
  - "cert-manager ClusterIssuer distinction called out in a :::note callout because it is the most common error"

patterns-established:
  - "Troubleshooting entries: bold Symptom, Cause, Fix labels for scannable structure"
  - "Code blocks for all diagnostic and fix commands — no prose-only instructions"

requirements-completed: [ADDON-05, ADDON-06, ADDON-07]

# Metrics
duration: 2min
completed: 2026-03-04
---

# Phase 31 Plan 02: Addon Page Depth (Headlamp, Local Registry, cert-manager) Summary

**Headlamp navigation guide, local registry multi-image workflow with cleanup, and cert-manager wildcard cert example — each page gains a Troubleshooting section with Symptom/Cause/Fix structure**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-04T15:00:11Z
- **Completed:** 2026-03-04T15:02:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Headlamp page gains a "What you can do" section covering 5 dashboard areas and a Troubleshooting section explaining the most common login failure (base64-encoded token pasted instead of decoded)
- Local Registry page gains a multi-image workflow with `curl v2/_catalog` verification, a cleanup guide, and a Troubleshooting section for ImagePullBackOff caused by stopped registry container
- cert-manager page gains a wildcard certificate example with `duration`/`renewBefore` fields and a Troubleshooting section covering the two most common READY: False causes (webhook timing and Issuer vs ClusterIssuer)

## Task Commits

Each task was committed atomically:

1. **Task 1: Enrich Headlamp and Local Registry addon pages** - `32e9e977` (feat)
2. **Task 2: Enrich cert-manager addon page** - `fca80c00` (feat)

**Plan metadata:** (docs commit below)

## Files Created/Modified

- `/Users/patrykattc/work/git/kinder/kinder-site/src/content/docs/addons/headlamp.md` — Added "What you can do" (5 dashboard areas) and "Troubleshooting" (invalid token) sections
- `/Users/patrykattc/work/git/kinder/kinder-site/src/content/docs/addons/local-registry.md` — Added "Multi-image workflow", "Cleaning up images", and "Troubleshooting" (ImagePullBackOff) sections
- `/Users/patrykattc/work/git/kinder/kinder-site/src/content/docs/addons/cert-manager.md` — Added "More certificate examples" (wildcard cert) and "Troubleshooting" (READY: False) sections

## Decisions Made

- Troubleshooting sections use bold **Symptom**, **Cause**, **Fix** labels for scannable problem resolution — consistent with the pattern established for plan 01
- Local Registry multi-image workflow shows two separate `docker build && docker push` pairs rather than a loop, making the pattern clearer for users who may only push one image
- cert-manager ClusterIssuer distinction is called out in a `:::note` callout (not just prose) because using `kind: Issuer` when a `ClusterIssuer` exists is the single most common error in cert-manager usage

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Plans 01 and 02 of phase 31 are complete — Headlamp, Local Registry, and cert-manager pages are fully enriched
- Plan 03 of phase 31 can proceed (remaining addon pages: MetalLB, Metrics Server, Envoy Gateway, CoreDNS)
- All three pages pass `npx astro check` with 0 errors and the full `npm run build` with 19 pages built successfully

---
*Phase: 31-addon-page-depth*
*Completed: 2026-03-04*

## Self-Check: PASSED

- FOUND: kinder-site/src/content/docs/addons/headlamp.md
- FOUND: kinder-site/src/content/docs/addons/local-registry.md
- FOUND: kinder-site/src/content/docs/addons/cert-manager.md
- FOUND: .planning/phases/31-addon-page-depth/31-02-SUMMARY.md
- FOUND commit: 32e9e977 (Task 1)
- FOUND commit: fca80c00 (Task 2)
