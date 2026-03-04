---
phase: 33-tutorials
plan: 01
subsystem: docs
tags: [tutorial, tls, cert-manager, envoy-gateway, metallb, local-registry, nginx, https]

# Dependency graph
requires:
  - phase: 31-addon-page-depth
    provides: cert-manager ClusterIssuer pattern, Envoy Gateway HTTPRoute pattern, MetalLB platform notes, local-registry push/pull conventions
  - phase: 33-tutorials
    provides: 33-RESEARCH.md with verified technical content for all tutorials
provides:
  - Complete TLS web app tutorial replacing Coming soon placeholder
  - End-to-end walkthrough: cluster creation, image push, deployment, TLS certificate, gateway, HTTPRoute, HTTPS verification
affects: [33-02, 33-03, guides index]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Tutorial page structure: opening paragraph, prerequisites, numbered steps, clean up"
    - "Every kubectl get / curl command followed by Expected output: bare fenced block"
    - "macOS/Windows platform note with kubectl get svc discovery before port-forward"
    - "caution callout for ClusterIssuer vs Issuer distinction"

key-files:
  created: []
  modified:
    - kinder-site/src/content/docs/guides/tls-web-app.md

key-decisions:
  - "Tutorial covers all four addons (Local Registry, Envoy Gateway, cert-manager, MetalLB) in a single coherent flow"
  - "macOS/Windows note shows kubectl get svc -n envoy-gateway-system to discover the envoy service name before port-forward (avoids hardcoding unpredictable hash)"
  - "caution callout placed inline in Step 5 immediately after Certificate YAML to catch ClusterIssuer vs Issuer mistake at point of use"
  - "note callout in Step 2 warns about cert-manager webhook 30-60s timing before user proceeds to certificate creation"

patterns-established:
  - "Tutorial step structure: command block, Expected output: label, bare fenced block"
  - "Platform divergence via :::note[macOS / Windows] callout with full port-forward alternative included inline"

requirements-completed: [GUIDE-01]

# Metrics
duration: 15min
completed: 2026-03-04
---

# Phase 33 Plan 01: TLS Web App Tutorial Summary

**nginx TLS tutorial via Local Registry push, cert-manager ClusterIssuer certificate, Envoy Gateway with HTTPS termination, and MetalLB-assigned external IP**

## Performance

- **Duration:** 15 min
- **Started:** 2026-03-04T10:40:00Z
- **Completed:** 2026-03-04T10:55:00Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Replaced `:::note[Coming soon]` placeholder with a 334-line end-to-end TLS tutorial
- Tutorial covers the complete flow: create cluster, verify addons, push nginx:alpine to localhost:5001, deploy app, issue cert-manager Certificate, create Envoy Gateway with TLS termination, create HTTPRoute, verify HTTPS with curl
- macOS/Windows users have a working port-forward path with `kubectl get svc` discovery step to find the Envoy Gateway service name
- All YAML manifests use correct resource names: `selfsigned-issuer`, `kind: ClusterIssuer`, `gatewayClassName: eg`
- Caution callout and note callout placed at exactly the right points in the tutorial flow

## Task Commits

Each task was committed atomically:

1. **Task 1: Write complete TLS web app tutorial** - `1e3cdcc6` (docs)

**Plan metadata:** (this summary commit)

## Files Created/Modified

- `kinder-site/src/content/docs/guides/tls-web-app.md` - Complete 334-line TLS web app tutorial replacing placeholder

## Decisions Made

- macOS/Windows note shows `kubectl get svc -n envoy-gateway-system` before giving the port-forward command. This is necessary because Envoy Gateway creates services with a hash suffix (`envoy-default-myapp-gateway-<hash>`) that cannot be predicted in advance. Showing the discovery command first is more robust than showing a hardcoded example.
- Placed the `:::caution` for `ClusterIssuer` vs `Issuer` inline in Step 5, immediately after the Certificate YAML. This is the point of use where users are most likely to make the mistake.
- Placed the `:::note` about cert-manager webhook timing in Step 2 (addon verification), before any Certificate resources are applied. This prevents the most common timing failure.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- TLS web app tutorial complete and verified (npm run build passes, all grep checks pass)
- Ready for Plan 02 (HPA auto-scaling tutorial) and Plan 03 (local dev workflow tutorial)
- Established tutorial page structure and Expected output pattern for remaining two tutorials to follow

---
*Phase: 33-tutorials*
*Completed: 2026-03-04*
