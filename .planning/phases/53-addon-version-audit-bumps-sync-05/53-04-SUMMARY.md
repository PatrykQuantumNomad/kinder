---
phase: 53-addon-version-audit-bumps-sync-05
plan: 04
subsystem: infra
tags: [envoy-gateway, gateway-api, kubernetes, addon-bump, httproute, metallb, uat]

# Dependency graph
requires:
  - phase: 53-addon-version-audit-bumps-sync-05
    provides: cert-manager v1.20.2 bump (Plan 53-03) — EG depends on MetalLB for LB IPs, which 53-03 left running

provides:
  - Envoy Gateway v1.7.2 pinned in Images slice (envoygw.go)
  - Upstream EG v1.7.2 install.yaml embedded (3.2 MB; replaces v1.3.1 bundle)
  - Gateway API CRDs at bundle-version v1.4.1 (was v1.2.1; in-band with EG manifest)
  - eg-gateway-helm-certgen Job name verified unchanged in v1.7.2 (Pitfall EG-02 cleared)
  - Ratelimit image upgraded from ae4cee11 to 05c08d03
  - Three regression tests: TestImagesPinsEGV172, TestManifestContainsCertgenJobName, TestManifestPinsGatewayAPIBundleV141
  - Addon doc caution callout for two-major-version jump
  - CHANGELOG ADDON-04 entry

affects: [53-05, 53-06, 53-07, 58-UAT]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Whole-unit manifest swap (Pitfall EG-01): replace install.yaml as one file; never split CRDs from the rest"
    - "Job-name regression test pattern: pin certgen job name in test to catch upstream renames before they break kubectl wait"
    - "Bundle-version annotation test: pin gateway.networking.k8s.io/bundle-version in test to track CRD set identity"
    - "macOS UAT platform constraint: curl to docker-bridge IPs from macOS host fails; use kubectl run uat-curl or kubectl port-forward"

key-files:
  created:
    - pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go (TestImagesPinsEGV172, TestManifestContainsCertgenJobName, TestManifestPinsGatewayAPIBundleV141)
  modified:
    - pkg/cluster/internal/create/actions/installenvoygw/envoygw.go (Images slice: v1.7.2 + ratelimit:05c08d03)
    - pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml (upstream EG v1.7.2; 3.2 MB)
    - kinder-site/src/content/docs/addons/envoy-gateway.md (version line + caution callout)
    - kinder-site/src/content/docs/changelog.md (ADDON-04 bump entry under v2.4)

key-decisions:
  - "Path A (bump): UAT-4 confirmed HTTPRoute end-to-end HTTP 200 via in-cluster curl; v1.7.2 delivered"
  - "Single-jump strategy (v1.3.1 → v1.7.2 direct) confirmed safe — Gateway API v1.4.1 CRDs backward compatible"
  - "UAT-4 script should swap hashicorp/http-echo backend to nginx (more stable, no CLI-arg shape issues)"
  - "UAT-4 script should use kubectl run uat-curl (in-cluster curl) on macOS hosts — docker-bridge IPs unreachable from macOS"
  - "eg-gateway-helm-certgen Job name unchanged in v1.7.2 — Pitfall EG-02 is cleared; forward regression guarded by TestManifestContainsCertgenJobName"

patterns-established:
  - "EG manifest is a single atomic unit: replace whole install.yaml, never extract CRDs separately (Pitfall EG-01)"
  - "Certgen job name test pattern: embed job name assertion so upstream renames are caught before kinder waits forever"

# Metrics
duration: ~45min (across two sessions including live UAT)
completed: 2026-05-10
---

# Phase 53 Plan 04: Envoy Gateway Bump Summary

**Envoy Gateway v1.3.1 → v1.7.2 two-major jump with Gateway API CRDs v1.2.1 → v1.4.1; HTTPRoute end-to-end UAT returned HTTP 200 via in-cluster curl (Path A).**

## Performance

- **Duration:** ~45 min (two sessions: RED commit + GREEN edits in session 1; final commit + SUMMARY in session 2)
- **Started:** 2026-05-10T14:00:00Z (approx)
- **Completed:** 2026-05-10T16:30:00Z (approx)
- **Tasks:** 3 (Task 1 RED, Task 2 UAT checkpoint, Task 3 GREEN + docs)
- **Files modified:** 5 (envoygw.go, install.yaml, envoygw_test.go, envoy-gateway.md, changelog.md)

## Path Taken

**Path A — bump confirmed.** UAT-4 passed. All four live checks cleared:
1. EG pods Running (controller + data-plane 2/2)
2. GatewayClass `eg`: Accepted=True
3. Gateway `uat-gw`: Accepted=True + Programmed=True (address assigned, 1/1 envoy replicas)
4. HTTPRoute `uat-route`: Accepted + ResolvedRefs → HTTP 200 via in-cluster curl

## Accomplishments

- Envoy Gateway bumped to v1.7.2 (two-major jump from v1.3.1; user-authorized single jump per REQUIREMENTS.md)
- Upstream install.yaml (3.2 MB) replaces 1.3.1 bundle as a whole unit (Pitfall EG-01 compliant)
- Gateway API CRDs upgraded from bundle-version v1.2.1 to v1.4.1 in-band (no separate CRD split)
- eg-gateway-helm-certgen Job name verified unchanged in v1.7.2 upstream manifest (Pitfall EG-02 cleared)
- Ratelimit image upgraded: docker.io/envoyproxy/ratelimit:ae4cee11 → 05c08d03
- Three regression tests added (TDD RED → GREEN cycle followed)
- Addon doc updated with caution callout disclosing two-major-version jump and CRD set change
- CHANGELOG entry landed under v2.4

## Live UAT-4 Transcript (verbatim from user report)

**Cluster:** uat-53-04 (had been up ~23h when UAT ran; EG v1.7.2 stability over that window is incidental positive evidence)

**EG pods:**
```
envoy-gateway-775985d57d-zsl8j          Running 23h   (controller)
envoy-default-uat-gw-fb204cf5-6ffd8dffdd-jzttj   2/2 Running 2h   (data-plane)
```

**GatewayClass:**
```
eg: Accepted=True
```

**Gateway uat-gw:**
```
Accepted=True, Programmed=True
"Address assigned to the Gateway, 1/1 envoy replicas available"
```

**HTTPRoute uat-route:**
```
Accepted + ResolvedRefs
```

**LB service:**
```
envoy-default-uat-gw-fb204cf5: EXTERNAL-IP 172.19.255.200 (MetalLB allocated, port 80→NodePort 32527)
```

**Final functional test:**
```
kubectl run uat-curl --image=curlimages/curl ... curl http://172.19.255.200/
→ HTTP 200 (nginx backend)
```

**User UAT signal:** `bump` (HTTP 200 confirmed end-to-end through Envoy Gateway → nginx backend)

## Task Commits

1. **Task 1 RED: failing tests pinning EG v1.7.2 + Gateway API v1.4.1** - `c901ab89` (test)
2. **Task 3 GREEN: v1.7.2 bump + addon doc + CHANGELOG** - `4c185804` (feat)

**Plan metadata:** (this commit — docs)

_Note: Task 2 was a human-verify checkpoint (live UAT); it has no commit of its own. GREEN commit encompasses envoygw.go + install.yaml + addon doc + CHANGELOG as a single atomic commit per Pitfall EG-01._

## Files Created/Modified

- `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` — Images slice updated: envoyproxy/gateway:v1.7.2 + docker.io/envoyproxy/ratelimit:05c08d03
- `pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go` — Three new tests: TestImagesPinsEGV172, TestManifestContainsCertgenJobName, TestManifestPinsGatewayAPIBundleV141
- `pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml` — upstream EG v1.7.2 bundle (3.2 MB; Gateway API CRDs at bundle-version v1.4.1)
- `kinder-site/src/content/docs/addons/envoy-gateway.md` — version line v1.3.1 → v1.7.2; :::caution[Major upgrade] callout added
- `kinder-site/src/content/docs/changelog.md` — ADDON-04 bump line under v2.4

## Decisions Made

- **Single-jump strategy confirmed safe:** v1.3.1 → v1.7.2 direct jump authorized by user via REQUIREMENTS.md. Live UAT confirmed no breaking issues in CRD migration or certgen flow.
- **Whole-unit manifest swap (Pitfall EG-01):** install.yaml replaced as a single file; CRDs are never extracted or split. This is the documented pattern for all future EG bumps.
- **Job name test guards Pitfall EG-02:** TestManifestContainsCertgenJobName will catch any upstream certgen Job rename before it breaks `kubectl wait`.
- **Gateway API CRD version pinned in test:** TestManifestPinsGatewayAPIBundleV141 locks bundle-version v1.4.1 for regression detection.

## Deviations from Plan

### UAT-4 Script Deviations (not EG regressions — UAT infrastructure issues)

**DEVIATION 1: hashicorp/http-echo backend entered CrashLoopBackOff (272 restarts in 22h)**
- **Nature:** UAT-script issue, NOT an EG v1.7.2 regression. The http-echo crashloop existed before EG received any traffic; it was a backend startup failure unrelated to the gateway.
- **Root cause:** `kubectl create deployment ... -- -text="..."` arg shape issue with the http-echo image (empty logs; likely the image requires a different flag format or entrypoint).
- **Resolution during UAT:** Swapped backend to nginx + patched HTTPRoute backendRefs[0].port from 5678 → 80.
- **Recommended fix for canonical UAT-4 script:** Replace `hashicorp/http-echo` with `nginx` as the backend image. Nginx is simpler, more reliable, and does not require CLI argument flags. Port 80 is the natural default.
- **EG v1.7.2 verdict:** Unaffected. The gateway itself never received a bad request; the backend issue was isolated to the pod startup.

**DEVIATION 2: macOS host cannot curl docker-bridge IPs (HTTP 000 — connection refused)**
- **Nature:** Platform constraint, NOT an EG issue. On macOS, Docker runs inside a Linux VM; the docker-bridge network (172.x.x.x) is not routable from the macOS host.
- **Resolution during UAT:** Ran `kubectl run uat-curl --image=curlimages/curl` from inside the cluster network; in-cluster pod successfully curled `http://172.19.255.200/` and received HTTP 200.
- **Recommended fix for canonical UAT-4 script:** On macOS hosts, the script should use either (a) `kubectl run uat-curl --image=curlimages/curl` (in-cluster curl, matching the pattern used here) or (b) `kubectl port-forward svc/envoy-default-uat-gw-<hash> 8080:80 &` followed by `curl http://localhost:8080/` (matching the pattern used in the Headlamp UAT-2 script). Option (b) is more portable; option (a) is simpler.
- **EG v1.7.2 verdict:** Unaffected. The HTTP 000 was a macOS-to-docker-bridge routing limitation; in-cluster curl confirmed end-to-end traffic flow through the gateway.

These are UAT-script improvements for future re-runs. The functional acceptance criterion (HTTPRoute returns HTTP 200 end-to-end) is satisfied.

---

**Total deviations:** 2 UAT-script issues (not code regressions)
**Impact on plan:** Both resolved during UAT session without blocking the bump decision. ADDON-04 delivered at v1.7.2.

## Test Results

```
--- PASS: TestImagesPinsEGV172 (0.00s)
--- PASS: TestManifestPinsGatewayAPIBundleV141 (0.00s)
--- PASS: TestManifestContainsCertgenJobName (0.01s)
--- PASS: TestExecute (0.00s)
    --- PASS: TestExecute/all_steps_succeed (0.00s)
    --- PASS: TestExecute/apply_manifest_fails (0.00s)
    --- PASS: TestExecute/wait_certgen_fails (0.00s)
    --- PASS: TestExecute/wait_deployment_fails (0.00s)
    --- PASS: TestExecute/apply_GatewayClass_fails (0.00s)
    --- PASS: TestExecute/wait_GatewayClass_fails (0.00s)
PASS
ok  sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installenvoygw  1.431s
```

`go build ./...` — clean (no errors).

## ADDON-04 Status

**DELIVERED at v1.7.2**

- Envoy Gateway: v1.3.1 → v1.7.2 (two-major jump)
- Gateway API CRD bundle-version: v1.2.1 → v1.4.1
- Certgen Job name: eg-gateway-helm-certgen (unchanged — Pitfall EG-02 cleared)
- Ratelimit image: ae4cee11 → 05c08d03
- UAT-4: HTTP 200 confirmed via in-cluster curl

## Issues Encountered

None beyond the two UAT-script deviations documented above. The EG v1.7.2 install itself was clean on first attempt.

## Next Phase Readiness

- Plan 53-05 (MetalLB hold/verify) can proceed — MetalLB was running throughout UAT-4 and allocated LB IPs successfully
- Plan 53-06 (Metrics Server hold/verify) unblocked
- Plan 53-07 (offline readiness consolidation) unblocked — do NOT modify offlinereadiness.go until 53-07
- Phase 58 UAT: final v2.4 binary will exercise EG v1.7.2 with the nginx backend pattern (not http-echo)

---
*Phase: 53-addon-version-audit-bumps-sync-05*
*Completed: 2026-05-10*
