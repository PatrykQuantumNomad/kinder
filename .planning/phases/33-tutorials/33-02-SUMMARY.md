---
phase: 33-tutorials
plan: 02
subsystem: docs
tags: [tutorial, hpa, autoscaling, metrics-server, local-registry, golang, docker, kubectl]

# Dependency graph
requires:
  - phase: 31-addon-page-depth
    provides: Metrics Server addon page with cpu requests caution, local-registry push/pull conventions
  - phase: 33-tutorials
    provides: 33-RESEARCH.md with verified technical content, established tutorial page structure from Plan 01
provides:
  - Complete HPA auto-scaling tutorial replacing Coming soon placeholder
  - Complete local dev workflow tutorial replacing Coming soon placeholder
  - End-to-end HPA walkthrough: deploy with cpu requests, create HPA, generate load, watch scaling, 5-minute scale-down
  - End-to-end local dev loop: Go app, build/push to localhost:5001, deploy, port-forward, iterate to v2
affects: [33-03, guides index]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "HPA tutorial: cpu requests on deployment container is prerequisite — caution callout placed immediately after deployment YAML"
    - "Load generation via kubectl run busybox:1.28 with wget loop inside cluster"
    - "Scale-down stabilization window (5 min) documented as expected behavior via :::note callout"
    - "Local dev iteration loop: build tag vN -> push -> kubectl set image -> rollout status"
    - "Port-forward restart note: must restart after each rollout since connection terminates"

key-files:
  created: []
  modified:
    - kinder-site/src/content/docs/guides/hpa-auto-scaling.md
    - kinder-site/src/content/docs/guides/local-dev-workflow.md

key-decisions:
  - "HPA tutorial uses registry.k8s.io/hpa-example (standard Kubernetes HPA demo workload) rather than a custom image"
  - "caution callout for resources.requests.cpu placed inline after deployment YAML — same pattern as metrics-server addon page"
  - "5-minute scale-down delay documented with :::note[Scale-down takes ~5 minutes] to prevent users thinking something is broken"
  - "Local dev tutorial uses Go HTTP server for concrete iteration demo but tip callout explains any language/framework works"
  - "Iteration loop uses incrementing tags (:v2, :v3) with tip note explaining reusing same tag can cause caching issues"

patterns-established:
  - "HPA tutorial step structure: prerequisite check -> deploy with cpu requests -> create HPA -> verify metrics -> generate load -> watch -> scale-down"
  - "Local dev iteration loop: edit code -> docker build -> docker push -> kubectl set image -> kubectl rollout status"

requirements-completed: [GUIDE-02, GUIDE-03]

# Metrics
duration: 20min
completed: 2026-03-04
---

# Phase 33 Plan 02: HPA and Local Dev Tutorials Summary

**HPA auto-scaling tutorial with registry.k8s.io/hpa-example and busybox load generator, plus Go app local dev loop using localhost:5001 with v1-to-v2 kubectl set image iteration**

## Performance

- **Duration:** 20 min
- **Started:** 2026-03-04T10:55:00Z
- **Completed:** 2026-03-04T11:15:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Replaced `:::note[Coming soon]` placeholder in `hpa-auto-scaling.md` with a 254-line HPA tutorial covering deployment, HPA creation, load generation, scaling observation, and scale-down
- Replaced `:::note[Coming soon]` placeholder in `local-dev-workflow.md` with a 257-line local dev workflow tutorial covering the full code-build-push-deploy-iterate cycle
- HPA tutorial includes the critical `resources.requests.cpu` caution, expected multi-line `--watch` output showing REPLICAS increasing, and a 5-minute scale-down stabilization note
- Local dev workflow tutorial uses a concrete Go HTTP server example with Dockerfile, iterates from v1 to v2, and documents the 5-step iteration loop in a tip callout
- All 7 verification checks pass: build succeeds, `HorizontalPodAutoscaler` present, `load-generator` present, `requests:` present, `localhost:5001` appears 16 times, `kubectl set image` appears 2 times, no "Coming soon" remaining

## Task Commits

Each task was committed atomically:

1. **Task 1: Write HPA auto-scaling tutorial** - `0a6c9091` (docs)
2. **Task 2: Write local dev workflow tutorial** - `67a2901f` (docs)

**Plan metadata:** (this summary commit)

## Files Created/Modified

- `kinder-site/src/content/docs/guides/hpa-auto-scaling.md` - Complete 254-line HPA auto-scaling tutorial replacing placeholder
- `kinder-site/src/content/docs/guides/local-dev-workflow.md` - Complete 257-line local dev workflow tutorial replacing placeholder

## Decisions Made

- Used `registry.k8s.io/hpa-example` (the standard Kubernetes HPA demo workload). This image is well-known, reliably CPU-intensive, and commonly used in official Kubernetes documentation — no need to build a custom image.
- Placed the `:::caution` for `resources.requests.cpu` inline immediately after the deployment YAML in the HPA tutorial. This matches the pattern established on the metrics-server addon page and ensures users see the warning at exactly the point where they could omit the field.
- Documented the 5-minute scale-down stabilization window as `:::note[Scale-down takes ~5 minutes]` rather than leaving it as implicit behavior. This is the single most common point where users think the HPA is broken.
- Local dev tutorial uses a Go HTTP server because it compiles to a tiny static binary (small image, fast build) but the `:::tip` callout explicitly notes any language works — this prevents the tutorial from feeling Go-specific.
- Iteration loop uses incrementing version tags (`:v2`, `:v3`) with a tip note warning that reusing the same tag can cause nodes to serve a cached layer. This prevents a subtle caching footgun.
- Port-forward restart note added in Step 8 (verify new version): users need to restart port-forward after a rollout since the previous connection terminates with the old pod.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- HPA auto-scaling and local dev workflow tutorials complete and verified (npm run build passes, all grep checks pass)
- All three guides in Phase 33 are now complete (tls-web-app, hpa-auto-scaling, local-dev-workflow)
- Phase 33 is ready for completion if Plan 03 is the final plan, or for Plan 03 execution if it exists

---
*Phase: 33-tutorials*
*Completed: 2026-03-04*
