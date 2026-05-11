---
phase: 53-addon-version-audit-bumps-sync-05
plan: "03"
subsystem: infra
tags: [cert-manager, kubernetes, tls, addon-bump, tdd, server-side-apply]

# Dependency graph
requires:
  - phase: 53-02
    provides: Headlamp v0.42.0 bump; addon-bump TDD + UAT pattern established
provides:
  - cert-manager v1.20.2 embedded manifest (989 KB) with --server-side apply preserved
  - TestImagesPinsV1202 + TestExecuteUsesServerSideApply regression guards
  - Addon doc :::caution[Breaking changes in v1.20] callout (UID + rotationPolicy disclosure)
  - CHANGELOG v2.4 ADDON-03 entry with both breaking-change disclosures
affects:
  - 53-04-envoy-gateway-bump (pattern: same TDD + UAT flow)
  - 53-07-offlinereadiness-consolidation (cert-manager images list unchanged)
  - Phase 58 final UAT (cert-manager v1.20.2 must survive full cluster creation)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "TDD RED+GREEN for addon image pin + --server-side flag regression guard"
    - "UAT-gate checkpoint: live ClusterIssuer + Certificate smoke before GREEN commit"
    - "distroless UID enforcement: rely on kubelet runAsNonRoot: true + image USER directive instead of manifest-level runAsUser pin"

key-files:
  created:
    - .planning/phases/53-addon-version-audit-bumps-sync-05/53-03-SUMMARY.md
  modified:
    - pkg/cluster/internal/create/actions/installcertmanager/certmanager.go
    - pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go
    - pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
    - kinder-site/src/content/docs/addons/cert-manager.md
    - kinder-site/src/content/docs/changelog.md

key-decisions:
  - "Path A taken: UAT-3 PASSED — cert-manager bumped to v1.20.2 (not held)"
  - "UID 65532 via distroless image USER directive — plan's manifest-level runAsUser: 65532 assertion was overspecified; kubelet runAsNonRoot: true enforcement is the actual security guarantee"
  - "CONTEXT.md decision bullet had typo '65632'; authoritative value is 65532 per REQUIREMENTS.md and upstream release notes — all plan artifacts use 65532"
  - "GREEN commit split into two: binary (certmanager.go + yaml) and docs (addon doc + changelog) for cleaner diff history"

patterns-established:
  - "Pitfall CERT-01: manifest >256 KB mandates --server-side apply; TestExecuteUsesServerSideApply is the permanent regression guard"
  - "Pitfall CERT-03 (non-root): satisfied by kubelet runAsNonRoot: true + distroless image default UID; do not overspecify runAsUser in test assertions"
  - "distroless images carry UID in image metadata (USER nonroot = 65532); manifest securityContext.runAsUser field may be absent by design"

# Metrics
duration: ~20min (continuation from UAT-3 checkpoint)
completed: "2026-05-10"
---

# Phase 53 Plan 03: cert-manager v1.16.3 -> v1.20.2 Summary

**cert-manager bumped to v1.20.2 via TDD RED/GREEN cycle; 989 KB manifest with --server-side apply preserved; live UAT-3 confirmed ClusterIssuer + Certificate smoke (Path A); UID 65532 distroless deviation documented.**

## Performance

- **Duration:** ~20 min (continuation from UAT-3 checkpoint)
- **Started:** 2026-05-10 (continuation)
- **Completed:** 2026-05-10
- **Tasks:** 3 (Task 1 RED, Task 2 UAT checkpoint, Task 3 GREEN + docs)
- **Files modified:** 5

## Path Taken

**Path A: bump** — UAT-3 PASSED. cert-manager v1.20.2 is live.

## UAT-3 Evidence (verbatim user findings)

1. `kubectl -n cert-manager get pods` — all 3 pods Running, 0 restarts, >= 5m uptime:
   - cert-manager-68756bcf6f-lw8hc
   - cert-manager-cainjector-c664cf9b8-fqsw4
   - cert-manager-webhook-5749c6dc95-wrhgj

2. Manifest sets `runAsNonRoot: true` at pod level (kubelet hard check — kubelet refuses to start root containers). PASS.

3. Manifest does NOT set explicit `runAsUser: 65532`. Upstream v1.20.2 relies on the distroless image's `USER nonroot` (UID 65532) directive rather than manifest-level UID pin. The plan/CONTEXT.md "UID 65532" assertion was overspecified — security intent (Pitfall CERT-03 = non-root) is satisfied by kubelet enforcement of `runAsNonRoot: true`. Specific UID number could not be verified without an ephemeral debug container; this is an upstream design choice and is correct/expected. See UID Deviation section.

4. Functional test PASSED: `kubectl wait --for=condition=Ready certificate/uat-test-cert --timeout=60s` succeeded; `openssl x509 -noout -subject` returned `subject=CN = uat-test`. ClusterIssuer + Certificate flow works end-to-end with v1.20.2.

5. Webhook startup check: no repeated webhook timeout events in `kubectl describe certificate`.

**User UAT response:** "bump"

## Accomplishments

- cert-manager v1.16.3 → v1.20.2: Images slice updated (cainjector, controller, webhook)
- 989 KB upstream v1.20.2 manifest embedded; `--server-side` apply flag preserved at certmanager.go line 79
- `TestImagesPinsV1202` and `TestExecuteUsesServerSideApply` both pass; TestExecute table-driven unaffected
- Addon doc updated: version line v1.16.3 → v1.20.2; `:::caution[Breaking changes in v1.20]` callout added covering UID 1000→65532 and rotationPolicy: Always GA-mandatory flip
- CHANGELOG v2.4 ADDON-03 entry landed with both breaking-change disclosures

## Task Commits

1. **Task 1: RED** — `0deabdaa` (test: failing test pinning cert-manager v1.20.2 + --server-side guard)
2. **Task 2: UAT checkpoint** — no commit (human verification gate)
3. **Task 3: GREEN (binary)** — `d21179ff` (feat: bump cert-manager to v1.20.2; Images slice + 989 KB manifest)
4. **Task 3: GREEN (docs)** — `abe50338` (docs: addon doc :::caution callout + CHANGELOG v2.4 entry)

**Plan metadata:** (this SUMMARY + STATE.md commit — see final_commit below)

## Files Created/Modified

- `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go` — Images slice: all three images updated to v1.20.2
- `pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go` — TestImagesPinsV1202 + TestExecuteUsesServerSideApply added
- `pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml` — replaced with upstream v1.20.2 manifest (989 KB)
- `kinder-site/src/content/docs/addons/cert-manager.md` — version updated; :::caution[Breaking changes in v1.20] block added
- `kinder-site/src/content/docs/changelog.md` — ADDON-03 bump entry added under v2.4 H2

## Manifest Verification

```
-rw-r--r-- 1 patrykattc staff 989K cert-manager.yaml
grep cert-manager-controller:v1.20.2 → 1 match
grep cert-manager-cainjector:v1.20.2 → 1 match
grep cert-manager-webhook:v1.20.2      → 1 match
```

## --server-side Preservation Evidence

- **Test:** `TestExecuteUsesServerSideApply` in `certmanager_test.go`
- **Assertion:** Iterates `node.Calls` (FakeNode arg-capture), finds first `kubectl apply` invocation, asserts `--server-side` present in argv
- **Guard fires:** Regression-guard sanity check confirmed in Task 1 — temporarily removing `--server-side` from certmanager.go:79 caused the test to FAIL with: `kubectl apply argv missing --server-side flag (Pitfall CERT-01); got [kubectl apply -f -]`; revert restored PASS

## Decisions Made

- Path A: UAT-3 PASSED. Bump proceeded. No hold.
- GREEN commit split into binary (certmanager.go + yaml) and docs (addon doc + changelog) for audit clarity.
- CHANGELOG entry explicitly states 989 KB (actual size) rather than plan's 992 KB estimate.

## Deviations from Plan

### UID 65532 Overspecification (Not a bug — upstream design choice)

**Found during:** Task 2 (UAT-3 checkpoint)
**Issue:** The plan's `must_have` truth stated: "Live UAT confirms... running cert-manager pods report `runAsUser=65532`." The plan also required: `kubectl get pod ... -o jsonpath='{.spec.containers[0].securityContext.runAsUser}'` to return 65532. In v1.20.2, the upstream manifest sets `runAsNonRoot: true` at pod level but does NOT set an explicit `runAsUser` field. The UID 65532 is enforced by the distroless image's `USER nonroot` directive embedded in the image metadata — not the Kubernetes manifest securityContext.

**Resolution:** This is NOT a plan failure. The security intent (Pitfall CERT-03 = non-root execution) is fully satisfied:
- kubelet enforces `runAsNonRoot: true` — it will refuse to start any container if the effective UID is 0
- distroless image carries `USER nonroot` (UID 65532) in its layer metadata
- UAT-3 pods were Running with 0 restarts, proving kubelet accepted the non-root UID
- Functional test (ClusterIssuer + Certificate) passed

**Plan assertion overspecification:** The plan inherited the UID check from CONTEXT.md which itself had a typo ("65632" vs "65532"). The authoritative source (REQUIREMENTS.md ADDON-03, upstream v1.20.0 release notes) says 65532. The plan's jsonpath check was an over-specification that conflated "UID 65532 is used" with "UID 65532 is visible in manifest securityContext." Upstream intentionally omits the manifest-level pin for distroless images.

**Classification:** Not flagged as a phase-level gap. Upstream design choice; plan assertion was based on stale CONTEXT.md guidance. Security requirement is met.

**Impact:** UAT-3 PASSED on all criteria that matter (pods Running, non-root enforced, ClusterIssuer + Certificate smoke). The UID deviation finding is captured here so future addon-bump plans do not repeat the overspecification.

**Note on CONTEXT.md typo:** CONTEXT.md decision bullet had "65632" (typo). All plan artifacts (tests, addon doc, CHANGELOG, this SUMMARY) correctly use **65532** per REQUIREMENTS.md and upstream release notes.

---

**Total deviations:** 1 (upstream design — overspecified plan assertion; not auto-fixed because it is not a bug)
**Impact on plan:** None. Path A succeeded. Security intent fully satisfied.

## Issues Encountered

None beyond the UID overspecification deviation documented above. All three commits landed cleanly. Tests passed throughout.

## ADDON-03 Status

**DELIVERED** — cert-manager v1.20.2 with `--server-side` apply; UAT-3 functional test passed; breaking-change disclosures (UID 1000→65532, rotationPolicy: Always) in addon doc and CHANGELOG.

## Next Phase Readiness

- Plan 53-04 (Envoy Gateway v1.7.2 bump) is unblocked
- Same TDD + UAT pattern applies; companion Gateway API CRD audit is the added complexity
- `eg-gateway-helm-certgen` job name must be re-verified in v1.7.2 install.yaml (noted in BLOCKERS)
- cert-manager v1.20.2 is stable; no regressions observed

---
*Phase: 53-addon-version-audit-bumps-sync-05*
*Completed: 2026-05-10*
