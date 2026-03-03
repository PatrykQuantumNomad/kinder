---
phase: 23-cert-manager-addon
plan: "01"
subsystem: infra
tags: [cert-manager, kubernetes, go-embed, addon, cluster-issuer, tls, pki]

# Dependency graph
requires:
  - phase: 22-local-registry-addon
    provides: installlocalregistry action pattern + create.go runAddon wiring

provides:
  - installcertmanager action package at pkg/cluster/internal/create/actions/installcertmanager/
  - cert-manager v1.16.3 manifest embedded via go:embed (986KB, 13263 lines)
  - self-signed ClusterIssuer (selfsigned-issuer) applied after webhook is Available
  - create.go wiring: runAddon("Cert Manager", opts.Config.Addons.CertManager, ...)

affects: [24-cli-commands, future-phases-needing-tls]

# Tech tracking
tech-stack:
  added: [cert-manager v1.16.3]
  patterns:
    - go:embed for large Kubernetes manifests (must use --server-side when >256KB)
    - Sequential deployment wait loop before applying CRD-dependent resources
    - Webhook readiness gate: wait for webhook Available before applying CR that webhook validates

key-files:
  created:
    - pkg/cluster/internal/create/actions/installcertmanager/certmanager.go
    - pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
  modified:
    - pkg/cluster/internal/create/create.go

key-decisions:
  - "Used cert-manager v1.16.3 (latest available) instead of v1.17.6 (not yet released as of 2026-03-03)"
  - "Webhook readiness gate: wait for cert-manager-webhook Available before applying ClusterIssuer to prevent 'no endpoints available' errors"
  - "Standard (non-server-side) apply for ClusterIssuer: small resource, no annotation size issues"

patterns-established:
  - "Large manifest pattern: go:embed + --server-side apply (cert-manager CRDs are 986KB)"
  - "Ordered wait loop: iterate deployments array with individual kubectl wait calls for clear error attribution"
  - "Webhook gate pattern: webhook must be Available before applying resources that webhook validates"

requirements-completed: [CERT-01, CERT-02, CERT-03, CERT-04]

# Metrics
duration: 10min
completed: 2026-03-03
---

# Phase 23 Plan 01: cert-manager Addon Summary

**cert-manager v1.16.3 installcertmanager action with go:embed (986KB manifest), --server-side apply, 3-deployment wait loop (300s), and selfsigned-issuer ClusterIssuer wired into create.go**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-03-03T16:29:44Z
- **Completed:** 2026-03-03T16:40:00Z
- **Tasks:** 2
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments
- Created installcertmanager action package following the EnvoyGW addon pattern exactly
- Embedded cert-manager v1.16.3 manifest (986KB, 13263 lines) via go:embed with --server-side apply
- Implemented 3-deployment wait loop (cert-manager, cert-manager-cainjector, cert-manager-webhook) with 300s timeout each, ensuring webhook is Available before ClusterIssuer is applied
- Wired runAddon("Cert Manager", opts.Config.Addons.CertManager, ...) into create.go after Dashboard, alphabetically ordered imports

## Task Commits

Each task was committed atomically:

1. **Task 1: Create installcertmanager action package with embedded manifest** - `431fa0ef` (feat)
2. **Task 2: Wire installcertmanager into create.go as disableable addon** - `67eb1616` (feat)

**Plan metadata:** (docs commit - see final commit)

## Files Created/Modified
- `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go` - Action implementation with NewAction()/Execute(), --server-side manifest apply, 3-deployment wait loop, selfsigned-issuer ClusterIssuer
- `pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml` - Embedded cert-manager v1.16.3 manifest (986KB, 13263 lines)
- `pkg/cluster/internal/create/create.go` - Added installcertmanager import and runAddon("Cert Manager", ...) after Dashboard

## Decisions Made
- Used cert-manager v1.16.3 (latest stable available) instead of the plan's v1.17.6 which does not exist yet as of 2026-03-03. The implementation is version-agnostic — the only change needed on upgrade is re-downloading the manifest.
- Webhook readiness gate is critical: the cert-manager webhook validates cert-manager CRs. Applying ClusterIssuer before the webhook pod is Available causes "no endpoints available" admission errors. The 3-deployment wait loop ensures this ordering.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Used cert-manager v1.16.3 instead of v1.17.6**
- **Found during:** Task 1 (manifest download)
- **Issue:** cert-manager v1.17.6 does not exist — the GitHub release returns 404. Latest available is v1.16.3.
- **Fix:** Downloaded v1.16.3 manifest (986KB, 13263 lines, 3 Deployments confirmed). Implementation is version-agnostic.
- **Files modified:** `pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml`
- **Verification:** `wc -c` shows 986843 bytes (>256KB confirming --server-side is mandatory); `grep -c "kind: Deployment"` shows 3 deployments
- **Committed in:** `431fa0ef` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 version pin corrected)
**Impact on plan:** Minor — v1.16.3 vs planned v1.17.6. Implementation pattern identical. Upgrade path: replace manifest file when v1.17.x ships.

## Issues Encountered
None beyond the version deviation above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 23 complete: cert-manager addon fully implemented and wired
- Phase 24 (CLI Commands: kinder env + kinder doctor) can begin immediately — depends only on Phase 21 which was completed earlier
- cert-manager default is true (on-by-default), consistent with all other addons. Users opt out via `addons.certManager: false`

## Self-Check: PASSED

- certmanager.go: FOUND at pkg/cluster/internal/create/actions/installcertmanager/certmanager.go
- cert-manager.yaml: FOUND at pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
- 23-01-SUMMARY.md: FOUND at .planning/phases/23-cert-manager-addon/23-01-SUMMARY.md
- Commit 431fa0ef (Task 1): FOUND
- Commit 67eb1616 (Task 2): FOUND

---
*Phase: 23-cert-manager-addon*
*Completed: 2026-03-03*
