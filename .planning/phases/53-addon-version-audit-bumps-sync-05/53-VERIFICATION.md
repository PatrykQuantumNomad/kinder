---
phase: 53-addon-version-audit-bumps-sync-05
verified: 2026-05-10T00:00:00Z
human_resolved: 2026-05-12T00:00:00Z
status: verified
score: 6/6 must-haves verified (SC6 deferred per Outcome B re-runnable status; SC1 second clause + SC3 third clause closed via 53-08 SC wording revision landed 2026-05-12)
human_decision: "Developer (golysoft@gmail.com) declined override acceptance for both SC1 second clause and SC3 third clause — filing both as gap closures so the SC wording is revised formally rather than silently overridden."
gaps_closed:
  - sc: "SC1 second clause"
    closed_at: "2026-05-12T00:00:00Z"
    closure_plan: "53-08-sc-wording-revision-PLAN.md"
    resolution: >
      ROADMAP.md SC1 second clause re-worded from 'kinder doctor offline-readiness does
      not warn on a fresh default cluster' (unsatisfiable by design) to 'all 14 bumped/held
      addon tags are present on the cluster node containerd image store (verified via
      crictl images on the control-plane node)' — the actually-verified evidence path.
      Air-gapped semantics of offline-readiness check explicitly documented inline. See
      53-07-SUMMARY.md DEVIATION section for full architectural analysis.
  - sc: "SC3 third clause"
    closed_at: "2026-05-12T00:00:00Z"
    closure_plan: "53-08-sc-wording-revision-PLAN.md"
    resolution: >
      ROADMAP.md SC3 third clause re-worded from 'issues a certificate with the new UID
      (65532)' (implied manifest runAsUser pin — absent in upstream v1.20.2 by design) to
      'pods enforced non-root via pod-level runAsNonRoot: true + distroless image USER
      nonroot directive (functional UID 65532)'. Same security guarantee, accurate mechanism.
      Live UAT-3 evidence preserved (pods Running, CN=uat-test Certificate issued). See
      53-03-SUMMARY.md UID Deviation section for full upstream-design rationale.
deferred:
  - truth: "SC6: default image constant updated to kindest/node:v1.36.x"
    addressed_in: "Phase 53 sub-plan 53-00 re-run (when kind publishes v0.32.0)"
    evidence: >
      Plan 53-00 Outcome B executed correctly: Docker Hub API returned HTTP 200 with count=0 for
      v1.36 tags. SC6 defines Outcome B as the valid halting condition with re-runnable status.
      image.go correctly remains at kindest/node:v1.35.1. No code regression; re-run
      instructions documented in 53-00-SUMMARY.md.
---

# Phase 53: Addon Version Audit, Bumps & SYNC-05 — Verification Report

**Phase Goal:** All 7 addons are at verified current-stable versions (or documented holds), the security fix for local-path-provisioner is closed, and the SYNC-05 node image bump executes if Docker Hub has published `kindest/node:v1.36.x`
**Verified:** 2026-05-10T00:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 first: local-path-provisioner v0.0.36 installed; GHSA-7fxv-8wr2-mfc4 closed | VERIFIED | `localpathprovisioner.go` Images[0]="docker.io/rancher/local-path-provisioner:v0.0.36"; manifest image line 91 matches; TestImagesPinsV0036 PASSES; busybox:1.37.0 pinned (TestManifestPinsBusybox PASS); is-default-class annotation present (TestStorageClassIsDefault PASS); GHSA referenced in changelog.md |
| 2 | SC1 second: kinder doctor offline-readiness does not warn on fresh default cluster | PASSED (override) | Override: realInspectImage inspects HOST docker store (correct air-gapped semantics); warn on default cluster is by-design. UAT-5 confirms all 14 addon tags present on cluster node via crictl. Override pending human confirmation — see human_verification. |
| 3 | SC2: Headlamp v0.42.0 installed; SA token authenticates against UI (or hold documented) | VERIFIED | `dashboard.go` Images[0]="ghcr.io/headlamp-k8s/headlamp:v0.42.0"; headlamp.yaml line 62 matches; TestImagesPinsHeadlampV042 PASSES; TestExecute (all sub-cases) PASSES; live UAT-2 Path A: kubectl auth can-i YES + curl UI HTTP 200 confirmed in 53-02 SUMMARY |
| 4 | SC3 first+second: cert-manager v1.20.2 installed with --server-side apply; CRD API version correct | VERIFIED | `certmanager.go` Images=[cainjector,controller,webhook]:v1.20.2; Apply call at line 79 includes "--server-side"; TestImagesPinsV1202 PASSES; TestExecuteUsesServerSideApply PASSES; live UAT-3 all 3 pods Running, ClusterIssuer issued CN=uat-test Certificate |
| 5 | SC3 third: cert-manager issues certificate with UID 65532 | PASSED (override) | Override: upstream v1.20.2 uses distroless USER nonroot (UID 65532) + kubelet runAsNonRoot: true; manifest has no explicit runAsUser field by upstream design. SC wording was overspecified. UAT-3 confirmed pods ran non-root; cert issued. Override pending human confirmation. |
| 6 | SC4: Envoy Gateway v1.7.2 installed; HTTPRoute routes traffic; certgen job name verified in install.yaml | VERIFIED | `envoygw.go` Images=["ratelimit:05c08d03","gateway:v1.7.2"]; install.yaml contains "eg-gateway-helm-certgen" (grep confirmed); TestManifestContainsCertgenJobName PASSES; TestImagesPinsEGV172 PASSES; TestManifestPinsGatewayAPIBundleV141 PASSES (v1.4.1); live UAT-4: HTTPRoute → HTTP 200 via in-cluster curl confirmed in 53-04 SUMMARY |
| 7 | SC5: allAddonImages and TestAllAddonImages_CountMatchesExpected reflect all bumped image refs; go test passes | VERIFIED | offlinereadiness.go allAddonImages = 14 entries (counted); tags: local-path v0.0.36, headlamp v0.42.0, cert-manager {cainjector,controller,webhook} v1.20.2, EG gateway v1.7.2 + ratelimit 05c08d03, MetalLB v0.15.3 (held), metrics-server v0.8.1 (held); TestAllAddonImages_CountMatchesExpected PASSES (expected=14); TestAllAddonImages_TagsMatchActions PASSES |
| 8 | SC6: default image updated to v1.36.x OR SYNC-05 halts INCONCLUSIVE with re-runnable status | VERIFIED (deferred SC6-A) | Outcome B: Docker Hub probe returned HTTP 200 count=0 for v1.36.x tags. image.go correctly unchanged at kindest/node:v1.35.1. INCONCLUSIVE marker in 53-00-SUMMARY.md. Re-run instructions documented. |
| 9 | MetalLB held at v0.15.3 with documented verification | VERIFIED | metallb.go Images=[controller:v0.15.3, speaker:v0.15.3]; GitHub releases API probe 2026-05-10 confirmed no newer release; 53-05-SUMMARY.md documents hold with verbatim probe output |
| 10 | Metrics Server held at v0.8.1 with documented verification | VERIFIED | metricsserver.go Images=[metrics-server:v0.8.1]; hold documented in 53-06-SUMMARY.md |

**Score:** 8/10 truths directly verified; 2 pending human confirmation (SC1-second, SC3-third UID override acceptance)

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go` | v0.0.36 in Images slice | VERIFIED | Line 35: "docker.io/rancher/local-path-provisioner:v0.0.36" |
| `pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` | v0.0.36 image tag | VERIFIED | Line 91: "image: docker.io/rancher/local-path-provisioner:v0.0.36" |
| `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go` | TestImagesPinsV0036, TestManifestPinsBusybox, TestStorageClassIsDefault | VERIFIED | All 3 tests PASS |
| `pkg/cluster/internal/create/actions/installdashboard/dashboard.go` | v0.42.0 in Images slice | VERIFIED | Line 37: "ghcr.io/headlamp-k8s/headlamp:v0.42.0" |
| `pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml` | v0.42.0 image tag; kinder-dashboard SA+Secret preserved | VERIFIED | Line 62: "image: ghcr.io/headlamp-k8s/headlamp:v0.42.0"; SA/Secret confirmed |
| `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go` | v1.20.2 images; --server-side in apply call | VERIFIED | Images=[cainjector,controller,webhook]:v1.20.2; line 79: "apply", "--server-side" |
| `pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go` | TestImagesPinsV1202, TestExecuteUsesServerSideApply | VERIFIED | Both PASS |
| `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` | v1.7.2 + ratelimit:05c08d03 in Images; certgen job name | VERIFIED | Images correct; job wait at "job/eg-gateway-helm-certgen" |
| `pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml` | v1.7.2 references; certgen job name; bundle-version v1.4.1 | VERIFIED | 3x v1.7.2 refs; "name: eg-gateway-helm-certgen" at line 52626; bundle-version: v1.4.1 confirmed |
| `pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go` | TestImagesPinsEGV172, TestManifestContainsCertgenJobName, TestManifestPinsGatewayAPIBundleV141 | VERIFIED | All 3 PASS |
| `pkg/internal/doctor/offlinereadiness.go` | allAddonImages 14-entry slice with all bumped/held tags | VERIFIED | 14 entries confirmed; all Phase 53 tags present |
| `pkg/internal/doctor/offlinereadiness_test.go` | TestAllAddonImages_CountMatchesExpected (expected=14); TestAllAddonImages_TagsMatchActions | VERIFIED | Both PASS |
| `pkg/apis/config/defaults/image.go` | Remains v1.35.1 (Outcome B; no change) | VERIFIED | Image = "kindest/node:v1.35.1@sha256:..." unchanged |
| `kinder-site/src/content/docs/changelog.md` | v2.4 Hardening section with all 4 addon bumps + 2 holds + SYNC-05 | VERIFIED | Lines 14–96: polished v2.4 section present with all sub-sections |
| `.planning/release-notes-v2.4-draft.md` | Tier-3 release notes with Upgraders Notes | VERIFIED | Exists; contains cert-manager UID 65532, rotationPolicy, EG v1.3→v1.7, SYNC-05 Outcome B |
| `kinder-site/src/content/docs/addons/cert-manager.md` | Breaking changes caution callout (UID 65532 + rotationPolicy) | VERIFIED | Lines 10–14: :::caution[Breaking changes in v1.20] with both bullets |
| `kinder-site/src/content/docs/addons/envoy-gateway.md` | Caution callout for two-major-version jump; v1.7.2 + v1.4.1 | VERIFIED | Lines 8–14: version line + :::caution[Major upgrade in v2.4] |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| installlocalpath.go::Images | offlinereadiness.go::allAddonImages | v0.0.36 tag match | WIRED | TestAllAddonImages_TagsMatchActions verifies tag equality; PASS |
| installdashboard.go::Images | offlinereadiness.go::allAddonImages | v0.42.0 tag match | WIRED | TestAllAddonImages_TagsMatchActions; PASS |
| installcertmanager.go::Images | offlinereadiness.go::allAddonImages | v1.20.2 tag match (3 images) | WIRED | TestAllAddonImages_TagsMatchActions; PASS |
| installenvoygw.go::Images | offlinereadiness.go::allAddonImages | v1.7.2 + 05c08d03 tag match | WIRED | TestAllAddonImages_TagsMatchActions; PASS |
| certmanager.go::Execute | kubectl apply --server-side | "--server-side" in argv | WIRED | Line 79 confirmed; TestExecuteUsesServerSideApply captures FakeNode.Calls and asserts flag present |
| envoygw.go::certgenWait | job/eg-gateway-helm-certgen | kubectl wait job name | WIRED | Line 91: "job/eg-gateway-helm-certgen"; TestManifestContainsCertgenJobName verifies name in embedded manifest |
| dashboard.go::Execute | kinder-dashboard-token Secret | kubectl get secret | WIRED | Lines 89–97 confirmed; TestExecute/get_secret_fails covers error path |

---

## Data-Flow Trace (Level 4)

Not applicable for this phase. All artifacts are Go action packages with deterministic manifests embedded at build time (go:embed). No dynamic data sources — image tags are constants in code, not fetched at runtime.

---

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build compiles cleanly | `go build ./...` | exit 0, no output | PASS |
| Static analysis clean | `go vet ./...` | exit 0, no output | PASS |
| TestAllAddonImages_CountMatchesExpected | `go test ./pkg/internal/doctor/... -run TestAllAddonImages_CountMatchesExpected -race` | PASS (14/14) | PASS |
| TestAllAddonImages_TagsMatchActions | `go test ./pkg/internal/doctor/... -run TestAllAddonImages_TagsMatchActions -race` | PASS | PASS |
| All action package tests | `go test ./pkg/cluster/internal/create/actions/... -race` | 12/12 packages ok | PASS |
| TestImagesPinsV0036 + guards | `go test ./pkg/cluster/internal/create/actions/installlocalpath/... -race -v` | PASS | PASS |
| TestImagesPinsHeadlampV042 | `go test ./pkg/cluster/internal/create/actions/installdashboard/... -race -v` | PASS | PASS |
| TestImagesPinsV1202 + TestExecuteUsesServerSideApply | `go test ./pkg/cluster/internal/create/actions/installcertmanager/... -race -v` | PASS | PASS |
| TestImagesPinsEGV172 + TestManifestContainsCertgenJobName + TestManifestPinsGatewayAPIBundleV141 | `go test ./pkg/cluster/internal/create/actions/installenvoygw/... -race -v` | PASS | PASS |
| Full doctor package (-race) | `go test ./pkg/internal/doctor/... -race` | FAIL (pre-existing race in socket_test.go; unrelated to Phase 53) | SKIP (pre-existing) |
| Targeted doctor offline-readiness tests | `go test ./pkg/internal/doctor/... -run TestOfflineReadiness\|TestAllAddonImages -race` | PASS | PASS |

**Note on doctor race:** `go test ./pkg/internal/doctor/... -race` fails due to a pre-existing race condition in `socket_test.go` / `check_test.go` that predates Phase 53. The `deferred-items.md` documents this as an unrelated pre-existing issue targeted for a future plan. Targeted subset tests for offline-readiness pass cleanly.

---

## Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| ADDON-01 | 53-01 | local-path-provisioner v0.0.36 with security fix | SATISFIED | v0.0.36 in Images + manifest; TestImagesPinsV0036 PASS |
| ADDON-02 | 53-02 | Headlamp v0.42.0 with token auth verified | SATISFIED | v0.42.0 in Images + manifest; UAT-2 Path A PASS |
| ADDON-03 | 53-03 | cert-manager v1.20.2 with --server-side | SATISFIED | v1.20.2 in Images; --server-side in Execute; UAT-3 PASS |
| ADDON-04 | 53-04 | Envoy Gateway v1.7.2 with HTTPRoute verified | SATISFIED | v1.7.2 in Images; certgen job verified; UAT-4 PASS |
| ADDON-05 | 53-07 | allAddonImages reflects all delivered tags; count=14 | SATISFIED | offlinereadiness.go allAddonImages = 14 entries; TestAllAddonImages_TagsMatchActions PASS |
| SYNC-05 | 53-00 | Default image bump to v1.36.x or INCONCLUSIVE halt | SATISFIED (Outcome B) | Probe returned count=0; image.go unchanged; re-runnable status preserved |

---

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` | 84 | Stale comment: "verified for v1.3.1; re-check on upgrade" (now at v1.7.2) | Info | None — TestManifestContainsCertgenJobName is the actual enforcement gate; test comment correctly says "Researcher confirmed v1.7.2 still ships this job name". Cosmetic only. |

No blockers found. No stub implementations. No empty return patterns in Phase 53 code.

---

## Human Verification Required

### 1. SC1 Second-Clause Override Acceptance

**Test:** Review 53-07-SUMMARY.md DEVIATION section and the `realInspectImage` implementation in `pkg/internal/doctor/offlinereadiness.go` lines 158–169.

**Expected:** Developer confirms that `kinder doctor offline-readiness` warning on a default cluster (no `--air-gapped`) is correct by design — the check measures whether the host docker store has pre-pulled addon images, not whether the cluster node has pulled them. All 14 addon tags ARE present on the cluster node containerd store (confirmed by UAT-5 crictl output in 53-07-SUMMARY.md).

**Why human:** SC1 second clause is an explicit ROADMAP Success Criterion. The verifier cannot autonomously accept an override for a ROADMAP SC without developer sign-off. If accepted, SC1 second clause closes as PASSED (override). If not accepted, this is a gap requiring either: (a) ROADMAP SC wording update, or (b) a future plan to add a `--check-node-store` variant to the doctor command.

### 2. SC3 Third-Clause UID 65532 Override Acceptance

**Test:** Review 53-03-SUMMARY.md UID Deviation section. Check cert-manager v1.20.2 manifest: `grep runAsNonRoot /Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml | wc -l` (returns 3 — one per component pod). The manifest has no explicit `runAsUser: 65532` field (upstream design choice; distroless image USER directive handles it).

**Expected:** Developer confirms that runAsNonRoot: true at pod level + distroless image USER nonroot (UID 65532) satisfies "self-signed ClusterIssuer issues a certificate with the new UID (65532)". Live UAT-3 confirmed pods ran non-root and Certificate CN=uat-test was issued successfully.

**Why human:** SC3 wording says "with the new UID (65532)" which could require either manifest-level pin (absent) or functional UID enforcement (present via distroless). This is a semantics judgment the developer must make. If accepted, SC3 third clause closes as PASSED (override). If not accepted, a future plan would need to add an explicit `runAsUser: 65532` field to the kinder cert-manager wrapper or document why it cannot.

---

## Deferred Items

Items not yet met but explicitly addressed in later phases or re-run conditions.

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | SC6: default image updated to kindest/node:v1.36.x | Phase 53-00 re-run (when kind publishes v0.32.0) | Outcome B halting is correct per SC6. 53-00-SUMMARY.md re-run instructions complete. |
| 2 | SC1 second clause wording corrected in ROADMAP.md | Phase 54+ (documentation fix) | 53-07-SUMMARY.md explicitly recommends re-wording. Not a code gap. |

---

## Gaps Summary

No code-level gaps identified. All Phase 53 source changes are in place, all unit tests pass (action packages + targeted offline-readiness), build and vet are clean.

Two human verification items remain:
1. **SC1 second clause:** Developer must decide whether to accept the override (offline-readiness warns on default cluster by design) or open a follow-up task to disambiguate the check behavior.
2. **SC3 UID 65532:** Developer must decide whether distroless image enforcement satisfies the literal SC wording or requires an explicit manifest-level runAsUser field.

One cosmetic warning (not a blocker):
- `envoygw.go` line 84 comment says "verified for v1.3.1; re-check on upgrade" — stale after the v1.7.2 bump. The test comment correctly states v1.7.2 verification. Source comment should be updated but has no functional impact.

---

_Verified: 2026-05-10_
_Verifier: Claude (gsd-verifier)_
