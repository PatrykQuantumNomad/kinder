---
phase: 53-addon-version-audit-bumps-sync-05
plan: "02"
subsystem: infra
tags: [headlamp, kubernetes-dashboard, addon-bump, token-auth, tdd]

# Dependency graph
requires:
  - phase: 53-01
    provides: "local-path-provisioner v0.0.36 bump landed; phase 53 infrastructure in place"
provides:
  - "Headlamp bumped from v0.40.1 to v0.42.0 with live token UAT verified (Path A)"
  - "ADDON-02 requirement delivered"
  - "Changelog ADDON-02 stub under v2.4 H2"
affects: [53-03, 53-07, phase-58-uat]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "TDD RED/GREEN with live UAT gate before final commit (checkpoint:human-verify guards production correctness)"
    - "Vendored manifest preservation: kinder-specific Secret/SA shape and -in-cluster arg survive upstream YAML replacement"

key-files:
  created: []
  modified:
    - pkg/cluster/internal/create/actions/installdashboard/dashboard.go
    - pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go
    - pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
    - kinder-site/src/content/docs/addons/headlamp.md
    - kinder-site/src/content/docs/changelog.md

key-decisions:
  - "Path A taken: UAT-2 confirmed RBAC=yes, UI=200, SA+Secret resolve — bump committed"
  - "Upstream OTEL telemetry env vars from v0.42.0 manifest merged into vendored headlamp.yaml"
  - "kinder-dashboard SA, kinder-dashboard-token Secret, -in-cluster arg, and targetPort:4466 all preserved verbatim"

patterns-established:
  - "Addon bump: always verify SA/Secret/arg/port preservation invariants via grep after YAML replacement"
  - "Addon bump: live UAT checkpoint is a blocking gate — unit tests alone are insufficient"

# Metrics
duration: 15min
completed: 2026-05-10
---

# Phase 53 Plan 02: Headlamp Bump Summary

**Headlamp bumped v0.40.1 → v0.42.0; kinder-dashboard SA token-print flow re-verified live (kubectl auth can-i + UI curl 200); ADDON-02 delivered.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-05-10T13:48:57Z
- **Completed:** 2026-05-10T14:03:00Z
- **Tasks:** 3 (RED + UAT checkpoint + GREEN)
- **Files modified:** 5

## Accomplishments

- TestImagesPinsHeadlampV042 (RED) written and committed before implementation
- Headlamp image bumped to v0.42.0 in both dashboard.go Images slice and embedded headlamp.yaml
- Upstream OTEL telemetry env vars from v0.42.0 manifest merged without breaking kinder-specific wiring
- Live UAT-2 (Path A): RBAC check, UI HTTP 200, SA+Secret resolution all passed — user confirmed with "bump"
- Addon doc updated, CHANGELOG ADDON-02 stub appended under v2.4 H2
- All unit tests (TestImagesPinsHeadlampV042 + TestExecute suite) pass under -race; go build ./... clean

## Live UAT Transcript (Task 2)

**Path A — UAT PASSED**

User verification results (verbatim reply: "bump"):
- RBAC check (`kubectl auth can-i --token=$TOKEN get pods --all-namespaces`): YES
- UI check (`curl ... http://localhost:4466/` with Bearer token): HTTP 200
- SA+Secret resolve (`kubectl -n kube-system get sa kinder-dashboard`, `kubectl -n kube-system get secret kinder-dashboard-token`): both succeed

No hold triggers fired. All three UAT-2 gate criteria passed.

## Task Commits

1. **Task 1: RED test pinning Headlamp v0.42.0** - `f0e6b6d8` (test)
2. **Task 2: Live UAT gate** - checkpoint:human-verify (user verified Path A)
3. **Task 3: GREEN bump + CHANGELOG + addon doc** - `b71f269b` (feat)

## Files Created/Modified

- `pkg/cluster/internal/create/actions/installdashboard/dashboard.go` - Images slice: v0.40.1 → v0.42.0
- `pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go` - TestImagesPinsHeadlampV042 added (RED commit)
- `pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml` - image bumped to v0.42.0; upstream OTEL env vars merged; all kinder wiring preserved
- `kinder-site/src/content/docs/addons/headlamp.md` - version line updated v0.40.1 → v0.42.0
- `kinder-site/src/content/docs/changelog.md` - ADDON-02 bump line appended under v2.4 H2

## Preservation Invariants Verified

| Invariant | Status |
|-----------|--------|
| `kinder-dashboard` ServiceAccount | PRESERVED |
| `kinder-dashboard-token` Secret (type: kubernetes.io/service-account-token) | PRESERVED |
| `-in-cluster` deployment arg | PRESERVED |
| `targetPort: 4466` on Service | PRESERVED |
| `ghcr.io/headlamp-k8s/headlamp:v0.42.0` image | PRESENT |

## Decisions Made

- Path A taken: live UAT-2 confirmed all three gate criteria passed; no hold needed
- Upstream OTEL env vars from v0.42.0 manifest accepted (non-breaking addition)
- No `:::caution[Breaking change]` callout in addon doc (UAT confirmed no breaking auth changes)

## Deviations from Plan

None — plan executed exactly as written. Path A was the expected happy path. All preservation invariants were intact in the working tree provided at handoff.

## Issues Encountered

None. Working tree at handoff contained all GREEN edits already applied (dashboard.go + headlamp.yaml). Continuation agent committed them after verifying tests passed and adding the remaining Task 3 artifacts (addon doc + CHANGELOG).

## ADDON-02 Status

**DELIVERED** — Headlamp v0.42.0 running in kinder with token-print flow verified live.

## Next Phase Readiness

- Plan 53-03 (cert-manager v1.16.3 → v1.20.2) is unblocked
- offlinereadiness.go consolidation is deferred to Plan 53-07 as planned — no modifications to that file in this plan
- No blockers

---
*Phase: 53-addon-version-audit-bumps-sync-05*
*Completed: 2026-05-10*
