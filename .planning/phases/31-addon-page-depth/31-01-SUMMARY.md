---
phase: 31-addon-page-depth
plan: 01
subsystem: ui
tags: [documentation, kubernetes, metallb, envoy-gateway, metrics-server, coredns, hpa, gateway-api]

# Dependency graph
requires:
  - phase: 30-foundation-fixes
    provides: "All seven addon pages created with verification, configuration, and disable sections"
provides:
  - "MetalLB page with custom IP example, NodePort guidance, and pending troubleshooting"
  - "Envoy Gateway page with path-based routing, header-based routing, and Gateway pending troubleshooting"
  - "Metrics Server page with kubectl top pods examples, HPA manifest, and unknown/50% troubleshooting"
  - "CoreDNS page with in-pod DNS verification, short-name resolution, and DNS failure troubleshooting"
affects: [31-02-addon-page-depth, any-phase-writing-addon-docs]

# Tech tracking
tech-stack:
  added: []
  patterns: [symptom-cause-fix troubleshooting pattern, practical-examples-before-troubleshooting doc structure]

key-files:
  created: []
  modified:
    - kinder-site/src/content/docs/addons/metallb.md
    - kinder-site/src/content/docs/addons/envoy-gateway.md
    - kinder-site/src/content/docs/addons/metrics-server.md
    - kinder-site/src/content/docs/addons/coredns.md

key-decisions:
  - "MetalLB: use metallb.universe.tf/loadBalancerIPs annotation (not deprecated address-pool annotation)"
  - "HPA manifest: use autoscaling/v2 (not v2beta2) — v2 is stable since Kubernetes 1.26"
  - "CoreDNS DNS examples: always run nslookup from inside a pod (kubectl run busybox), never from host"
  - "Troubleshooting entries: always include concrete symptom, root cause explanation, and actionable fix commands"

patterns-established:
  - "Practical examples pattern: show two concrete subsections with working YAML and expected command output"
  - "Troubleshooting pattern: Symptom / Cause / Fix with kubectl commands, placed after Practical examples and before any technical notes"
  - "HPA caution: always call out resources.requests.cpu requirement to prevent unknown/50% issue"

requirements-completed: [ADDON-01, ADDON-02, ADDON-03, ADDON-04]

# Metrics
duration: 2min
completed: 2026-03-04
---

# Phase 31 Plan 01: Addon Page Depth Summary

**Practical examples and troubleshooting sections added to MetalLB, Envoy Gateway, Metrics Server, and CoreDNS pages with working YAML manifests, expected command output, and symptom/cause/fix troubleshooting entries**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-04T14:59:14Z
- **Completed:** 2026-03-04T15:01:22Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- MetalLB: custom IP service with metallb.universe.tf/loadBalancerIPs, NodePort alternative guidance, pending IP troubleshooting
- Envoy Gateway: path-based and header-based HTTPRoute examples using gateway.networking.k8s.io/v1, Gateway pending troubleshooting
- Metrics Server: kubectl top pods examples, complete HPA manifest (autoscaling/v2) with caution on resources.requests.cpu, unknown/50% troubleshooting
- CoreDNS: in-pod nslookup verification, short-name resolution explanation, DNS failure troubleshooting with Corefile check

## Task Commits

Each task was committed atomically:

1. **Task 1: Enrich MetalLB and Envoy Gateway addon pages** - `67946c35` (feat)
2. **Task 2: Enrich Metrics Server and CoreDNS addon pages** - `32e9e977` (feat)

**Plan metadata:** (docs commit below)

## Files Created/Modified

- `kinder-site/src/content/docs/addons/metallb.md` - Added Practical examples (custom IP, NodePort) and Troubleshooting (pending service)
- `kinder-site/src/content/docs/addons/envoy-gateway.md` - Added Practical examples (path routing, header routing) and Troubleshooting (Gateway pending)
- `kinder-site/src/content/docs/addons/metrics-server.md` - Added Practical examples (kubectl top, HPA manifest) and Troubleshooting (unknown/50%)
- `kinder-site/src/content/docs/addons/coredns.md` - Added Practical examples (in-pod nslookup, short-name) and Troubleshooting (DNS failure)

## Decisions Made

- Used `metallb.universe.tf/loadBalancerIPs` annotation (not the deprecated `metallb.universe.tf/address-pool`)
- Used `autoscaling/v2` for HPA (not v2beta2 which was removed in Kubernetes 1.26)
- CoreDNS DNS examples always use `kubectl run busybox` — running nslookup from the host hits host DNS, not CoreDNS
- Troubleshooting sections follow the Symptom / Cause / Fix pattern consistently across all four pages

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All four pages now have Practical examples and Troubleshooting sections
- Pattern established: Practical examples > Troubleshooting > Platform notes / Technical notes
- Phase 31 Plan 02 can proceed to enrich the remaining addon pages (Headlamp, cert-manager, Local Registry)

---
*Phase: 31-addon-page-depth*
*Completed: 2026-03-04*
