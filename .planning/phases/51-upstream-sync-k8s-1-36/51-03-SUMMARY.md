---
phase: 51-upstream-sync-k8s-1-36
plan: "03"
subsystem: docs
tags: [kubernetes, k8s-1-36, user-namespaces, in-place-pod-resize, astro, starlight, kinder-site]

requires: []
provides:
  - "Guide page kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md with User Namespaces and In-Place Pod Resize demos"
  - "Starlight sidebar entry for guides/k8s-1-36-whats-new in astro.config.mjs"
affects:
  - "51-04 (may reference this guide from tls-web-app.md or multi-version-clusters.md)"

tech-stack:
  added: []
  patterns:
    - "Starlight guide: frontmatter title+description, H2 sections, :::tip and :::note callouts, yaml/bash code fences"

key-files:
  created:
    - kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md
  modified:
    - kinder-site/astro.config.mjs

key-decisions:
  - "Placed k8s-1-36-whats-new sidebar entry after multi-version-clusters (chronologically most relevant predecessor) and before working-offline"
  - "Used container-level resize (GA in 1.35, default-on in 1.36) — not pod-level (Beta in 1.36)"
  - "kubeadm v1beta4 note included as forward-looking text, no code change required in this phase"

patterns-established:
  - "Version-specific K8s feature guides follow the same frontmatter+H2 callout pattern as other kinder guides"

duration: 8min
completed: 2026-05-07
---

# Phase 51 Plan 03: Website K8s 1.36 Recipe Summary

**'What's new in Kubernetes 1.36' Starlight guide with User Namespaces (hostUsers: false) and In-Place Pod Resize (container-level resizePolicy) demos, registered in the sidebar**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-05-07T08:32:00Z
- **Completed:** 2026-05-07T08:34:40Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Created 167-line guide page with User Namespaces GA demo (`hostUsers: false` pod spec, uid_map verification), In-Place Pod Resize GA demo (container-level `resizePolicy`, resize subresource patch), kubeadm v1beta4 forward-looking note, cleanup, and references
- Registered `{ slug: 'guides/k8s-1-36-whats-new' }` in the Starlight Guides items array after `multi-version-clusters`
- `npm run build` exit 0 — 28 pages built, `/guides/k8s-1-36-whats-new/index.html` confirmed in build output

## Task Commits

1. **Task 1: Author the K8s 1.36 guide page** - `079bc4ba` (docs)
2. **Task 2: Register the guide in the Starlight sidebar** - `4e40cb5d` (docs)

**Plan metadata:** (final commit below)

## Files Created/Modified

- `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` - New guide: 167 lines, User Namespaces + In-Place Pod Resize sections, frontmatter, callouts, references
- `kinder-site/astro.config.mjs` - Added `{ slug: 'guides/k8s-1-36-whats-new' }` at line 83, after `guides/multi-version-clusters`

## Decisions Made

- Sidebar placement: after `guides/multi-version-clusters` and before `guides/working-offline` — the most recent K8s-version-related guide in the array is `multi-version-clusters`, so the 1.36 guide follows it logically
- Container-level resize demonstrated (GA in 1.35, default-on in 1.36) — pod-level (`InPlacePodLevelResourcesVerticalScaling`) is Beta in 1.36 and is only mentioned in a :::note callout
- Kubernetes 1.36 release blog URL used as `https://kubernetes.io/releases/` (stable redirect) rather than a date-specific URL that may drift
- kubeadm v1beta4 note kept as a single paragraph — no kinder code change needed in this phase

## Deviations from Plan

None - plan executed exactly as written.

## Build Verification

- `node --check kinder-site/astro.config.mjs`: exit 0
- `npm run build` (Task 1): exit 0, 28 pages built, `/guides/k8s-1-36-whats-new/index.html` generated
- `npm run build` (Task 2): exit 0, 28 pages built, guide in build output with sidebar registered

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 51-04 (Wave 2) can proceed; it will edit `multi-version-clusters.md` and `tls-web-app.md` — neither was touched in this plan
- The new guide is live in the site build and reachable from the Guides sidebar

---
*Phase: 51-upstream-sync-k8s-1-36*
*Completed: 2026-05-07*

## Self-Check: PASSED

- [x] `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` exists (167 lines, hostUsers: false x3, resizePolicy x2)
- [x] `kinder-site/astro.config.mjs` contains `guides/k8s-1-36-whats-new` at line 83
- [x] Commit `079bc4ba` exists (Task 1)
- [x] Commit `4e40cb5d` exists (Task 2)
- [x] `npm run build` exit 0 both after Task 1 and Task 2
