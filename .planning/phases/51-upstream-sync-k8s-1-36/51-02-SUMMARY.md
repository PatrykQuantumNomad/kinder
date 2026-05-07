---
phase: 51-upstream-sync-k8s-1-36
plan: "02"
subsystem: config-validation
tags: [ipvs, kube-proxy, kubernetes-1.36, deprecation-guard, validation, tdd]

requires:
  - phase: 50-runtime-error-decoder
    provides: "completed baseline; no direct coupling to this plan"

provides:
  - "IPVS-on-1.36+ deprecation guard inside Cluster.Validate() — rejects kubeProxyMode:ipvs when any node image is K8s 1.36+"
  - "7 table-driven test cases in TestIPVSDeprecationGuard covering reject/pass/skip semantics"
  - "SYNC-03 and SC3 delivered: hard validation rejection with actionable migration URL"

affects:
  - "51-01 (Envoy LB): no coupling — only validate.go modified"
  - "51-03 (guide): conceptual alignment — error message wording matches guide framing"
  - "51-04 (image bump): no coupling"

tech-stack:
  added: []
  patterns:
    - "imageTagVersion + version.ParseSemantic reuse pattern for per-node version checks in Validate()"
    - "break-on-first-match loop for single-error-per-cluster guard"
    - "TestIPVS* pattern: table-driven with errorMustContain + errorMustNotContain fields for message framing assertions"

key-files:
  created: []
  modified:
    - pkg/internal/apis/config/validate.go
    - pkg/internal/apis/config/validate_test.go

key-decisions:
  - "Guard placed immediately after KubeProxyMode validity check, before validateVersionSkew — ensures ipvs+1.36 error is always emitted even if version-skew also fires"
  - "Error message uses 'deprecated and will be removed in a future release' — NOT 'removed in 1.36' (IPVS is deprecated in 1.35, removal is future). Technically accurate per RESEARCH §Pitfall 6."
  - "Guard skips nodes with non-semver tags (e.g. 'latest') via continue — no spurious errors in test/dev clusters"
  - "break-on-first-match: only one error per Cluster — user gets one clear actionable message, not N errors for N 1.36 nodes"
  - "Mixed-node test (ipvs_rejected_on_mixed_with_one_1_36_node) uses CP + Worker roles (not two CPs) to avoid triggering HA minor-version-mismatch error from validateVersionSkew in the same aggregate"

patterns-established:
  - "Per-node version check in Cluster.Validate(): iterate c.Nodes, use imageTagVersion + version.ParseSemantic, skip on error, break on first match"

duration: ~8min
completed: "2026-05-07"
---

# Phase 51 Plan 02: IPVS-on-1.36+ Deprecation Guard Summary

**IPVS deprecation guard added to Cluster.Validate() — kubeProxyMode:ipvs is rejected at config time when any node image is K8s 1.36+, with "deprecated, will be removed" framing and migration URL**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-05-07T12:26:00Z
- **Completed:** 2026-05-07T12:34:42Z
- **Tasks:** 2 (RED + GREEN)
- **Files modified:** 2

## Accomplishments

- Added 7-case `TestIPVSDeprecationGuard` table-driven test covering all required scenarios (RED commit `6fc7b4ea`)
- Implemented IPVS-on-1.36+ guard in `Cluster.Validate()` using existing `imageTagVersion` + `version.ParseSemantic` helpers (GREEN commit `3d135694`)
- All 7 new test cases pass `-race`; no regressions in existing `TestClusterValidate` or `TestValidateVersionSkew` suites
- Error message verified: contains "deprecated", "will be removed in a future release", migration URL — does NOT contain "removed in 1.36"
- Zero new dependencies added

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Add failing tests for IPVS-on-1.36+ deprecation guard** - `6fc7b4ea` (test)
2. **Task 2 (GREEN): Reject kubeProxyMode ipvs on Kubernetes 1.36+ at validation time** - `3d135694` (feat)

## TDD Gate Compliance

- RED gate: `test(51-02)` commit `6fc7b4ea` — 4 expected-error cases failing (guard not yet implemented), 3 expected-pass cases passing
- GREEN gate: `feat(51-02)` commit `3d135694` — all 7 cases pass

## Files Created/Modified

- `pkg/internal/apis/config/validate_test.go` — Added `TestIPVSDeprecationGuard` with 7 subtests (120 lines)
- `pkg/internal/apis/config/validate.go` — Added IPVS-on-1.36+ guard block (25 lines) after the KubeProxyMode validity check

## New Error Message (exact text from validate.go)

```
kubeProxyMode: ipvs is not supported with Kubernetes 1.36+ (node image %q uses v1.%d); kube-proxy IPVS mode was deprecated in v1.35 and will be removed in a future release. Switch to iptables or nftables. Migration guide: https://kubernetes.io/docs/reference/networking/virtual-ips/
```

## Decisions Made

- Guard placed after KubeProxyMode validity check, before `validateVersionSkew` — ensures ipvs+1.36 always emits a guard error even when version-skew also fires
- "deprecated and will be removed in a future release" framing (not "removed in 1.36") — IPVS deprecated in v1.35, removal is a future release per upstream KEP; technically accurate
- Mixed-node test uses CP (v1.35.1) + Worker (v1.36.0) roles to avoid the HA minor-version-mismatch from `validateVersionSkew` firing on two CP nodes with different minor versions
- `break` on first 1.36+ node: one error per cluster, not one per violating node

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Mixed-node test adjusted from two CP nodes to CP + Worker**
- **Found during:** Task 1 (RED phase — test run)
- **Issue:** The original mixed-node test used `makeCluster(IPVSProxyMode, "kindest/node:v1.35.1", "kindest/node:v1.36.0")` which creates two ControlPlaneRole nodes. `validateVersionSkew` fires first (HA minor-version mismatch) and the test assertion `contains(errStr, "ipvs is not supported with Kubernetes 1.36")` fails because the error string only contains the version-skew message.
- **Fix:** Refactored `ipvs_rejected_on_mixed_with_one_1_36_node` test case to use explicit `ControlPlaneRole` (v1.35.1) + `WorkerRole` (v1.36.0) node slice — avoids HA check, lets the IPVS guard fire cleanly
- **Files modified:** pkg/internal/apis/config/validate_test.go
- **Verification:** All 7 cases pass after fix; skew test still passes independently
- **Committed in:** `6fc7b4ea` (RED commit, same file)

---

**Total deviations:** 1 auto-fixed (Rule 1 — test correctness fix before RED commit)
**Impact on plan:** Minor test construction adjustment, no behavior change. Guard semantics unchanged.

## Issues Encountered

- Pre-existing build failure in `pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go` (`loadbalancer.ConfigPath undefined`) from a parallel Wave-1 plan (51-01). Confirmed pre-existing at HEAD before my changes. Out-of-scope per deviation rules; `pkg/internal/apis/config` builds and tests independently with exit 0.

## Known Stubs

None — guard is fully implemented and wired into `Cluster.Validate()`.

## Threat Flags

None — no new network endpoints, auth paths, or trust-boundary changes. This is a validation-only change that adds a hard rejection at config-parse time.

## Verification Results

```
$ go test ./pkg/internal/apis/config/... -race
ok  sigs.k8s.io/kind/pkg/internal/apis/config        1.502s
ok  sigs.k8s.io/kind/pkg/internal/apis/config/encoding  1.275s

$ grep -n "kubeProxyMode: ipvs is not supported" pkg/internal/apis/config/validate.go
92: "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+ "+

$ grep -n "kubernetes.io/docs/reference/networking/virtual-ips" pkg/internal/apis/config/validate.go
95: "Migration guide: https://kubernetes.io/docs/reference/networking/virtual-ips/",
```

## Self-Check: PASSED

- `pkg/internal/apis/config/validate.go` — FOUND (modified with guard)
- `pkg/internal/apis/config/validate_test.go` — FOUND (modified with 7 test cases)
- RED commit `6fc7b4ea` — FOUND (git log)
- GREEN commit `3d135694` — FOUND (git log)
- All 7 `TestIPVSDeprecationGuard` subtests PASS
- Error message contains "deprecated": YES
- Error message contains migration URL: YES
- Error message does NOT contain "removed in 1.36": CONFIRMED

## Next Phase Readiness

- SC3 delivered: validation rejects ipvs+1.36 with clear actionable message
- Wave 1 still in-progress (51-01 Envoy LB, 51-03 docs): no blocking dependency on this plan
- 51-04 (default image bump): no coupling to this plan

---
*Phase: 51-upstream-sync-k8s-1-36*
*Completed: 2026-05-07*
