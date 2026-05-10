---
phase: 53-addon-version-audit-bumps-sync-05
plan: 07
type: execute
wave: 8
depends_on: ["53-06"]
files_modified:
  - pkg/internal/doctor/offlinereadiness.go
  - pkg/internal/doctor/offlinereadiness_test.go
  - kinder-site/src/content/docs/changelog.md
  - .planning/release-notes-v2.4-draft.md
autonomous: false
requirements: [ADDON-05]

must_haves:
  truths:
    - "pkg/internal/doctor/offlinereadiness.go allAddonImages slice mirrors the bumped image tags from 53-01..53-04 (and SYNC-05 bump if Outcome A) and the unchanged tags for held items (53-05 MetalLB, 53-06 Metrics Server)"
    - "Total entry count of allAddonImages remains 14 (no addon adds or removes an image; only tags change)"
    - "TestAllAddonImages_CountMatchesExpected (current name verified in research; constant `expected = 14`) passes unchanged"
    - "All TestAllAddonImages_* sub-tests still pass"
    - "If a prior plan was held (e.g. Headlamp at v0.40.1), the corresponding allAddonImages entry retains the old tag — only delivered bumps move forward"
    - "53-07 commit consolidates per-plan CHANGELOG stubs into a polished v2.4 section structure"
    - "Three-tier disclosure shipped: tier 1 CHANGELOG.md, tier 2 addon docs (already landed in 53-02..53-04), tier 3 v2.4 release-notes draft at .planning/release-notes-v2.4-draft.md"
    - "Live `kinder doctor offline-readiness` returns no warn|missing lines on a fresh default cluster (SC1 second clause)"
  artifacts:
    - path: "pkg/internal/doctor/offlinereadiness.go"
      provides: "Updated allAddonImages slice reflecting the final delivered state of phase 53"
      contains: "allAddonImages"
    - path: "kinder-site/src/content/docs/changelog.md"
      provides: "Consolidated v2.4 — Hardening section (per-plan stubs reformatted)"
      contains: "v2.4"
    - path: ".planning/release-notes-v2.4-draft.md"
      provides: "Tier 3 of three-tier disclosure: v2.4 release notes (GitHub release body) draft with dedicated Upgraders' Notes section"
      contains: "Upgraders' Notes"
  key_links:
    - from: "Each addon's Images slice (per installX/X.go)"
      to: "Mirror entry in allAddonImages with the same tag"
      via: "String equality between Images slice tags and allAddonImages.Image fields"
      pattern: "v[0-9]+\\.[0-9]+\\.[0-9]+"
    - from: "Per-plan CHANGELOG stubs (tier 1)"
      to: "Consolidated v2.4 — Hardening section + release-notes draft (tier 3)"
      via: "Reformat into final v2.4 structure (matches v2.3 H2 layout); release notes lift Breaking Changes + Upgraders' Notes from CHANGELOG"
      pattern: "## v2\\.4|## Upgraders' Notes"
    - from: "Live `kinder doctor offline-readiness` on fresh cluster"
      to: "SC1 second clause (no warn|missing on a fresh default cluster)"
      via: "Built binary + cluster create + doctor invocation + grep assertion"
      pattern: "kinder doctor offline-readiness"
---

<objective>
Consolidate the offline-readiness image list, the changelog, and ship the third tier of breaking-change disclosure (v2.4 release-notes draft). This plan is the single point of truth for ADDON-05's "all bumped image references propagated to `pkg/internal/doctor/offlinereadiness.go` `allAddonImages`" requirement (per CONTEXT.md decision: no per-plan offlinereadiness edits) and closes SC1's second clause via live verification.

Purpose: Land the phase-closing edits + the live SC1 verification. RESEARCH §Pitfall ALL-02 confirms the count stays at 14 — only tags shift; no addon adds or removes an image with these specific bumps. CONTEXT D-locks the three-tier disclosure (CHANGELOG.md + addon docs + v2.4 release notes); tiers 1 and 2 land in 53-00..53-06 atomically with each bump, and tier 3 (release-notes draft) lands here.

Output: A two-commit close — (1) atomic `feat(53-07)` for offlinereadiness + CHANGELOG consolidation + release-notes draft, then (2) live UAT result recorded in SUMMARY.md (no source change). Phase 53 closes here.
</objective>

<execution_context>
@/Users/patrykattc/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/REQUIREMENTS.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-CONTEXT.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-RESEARCH.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-00-SUMMARY.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-01-SUMMARY.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-02-SUMMARY.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-03-SUMMARY.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-04-SUMMARY.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-05-SUMMARY.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-06-SUMMARY.md

@pkg/internal/doctor/offlinereadiness.go
@pkg/internal/doctor/offlinereadiness_test.go
@kinder-site/src/content/docs/changelog.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Read final state of every addon's Images slice; update allAddonImages to mirror</name>
  <files>
    pkg/internal/doctor/offlinereadiness.go
  </files>
  <action>
**STEP A — Read the final delivered state of each addon by reading SUMMARYs and the Images slices.**

For each prior plan, read its `*-SUMMARY.md` and the corresponding `installX/X.go` Images slice to determine the post-phase truth:

| Plan | Addon | Final state to mirror |
|------|-------|------------------------|
| 53-00 | Default node image | If Outcome A: `pkg/apis/config/defaults/image.go` const Image = `kindest/node:v1.36.x@sha256:...`. **NOTE:** the default node image is NOT in `allAddonImages` (it is in `defaults/image.go`). 53-00 does not change `allAddonImages`. |
| 53-01 | local-path-provisioner | Final delivered tag: `docker.io/rancher/local-path-provisioner:v0.0.36` (busybox:1.37.0 unchanged) |
| 53-02 | Headlamp | If Path A: `ghcr.io/headlamp-k8s/headlamp:v0.42.0`. If Path B (hold): keep `ghcr.io/headlamp-k8s/headlamp:v0.40.1`. |
| 53-03 | cert-manager | If Path A: cainjector/controller/webhook all `quay.io/jetstack/cert-manager-*:v1.20.2`. If Path B (hold): keep all three at `v1.16.3`. |
| 53-04 | Envoy Gateway | If Path A: `envoyproxy/gateway:v1.7.2` and `docker.io/envoyproxy/ratelimit:05c08d03`. If Path B (hold): keep `envoyproxy/gateway:v1.3.1` and `docker.io/envoyproxy/ratelimit:ae4cee11`. |
| 53-05 | MetalLB | Unchanged (held v0.15.3). |
| 53-06 | Metrics Server | Unchanged (held v0.8.1). |

**Cross-check via grep:** the Images slice in each `installX/X.go` is the authoritative live truth. Run:
```bash
grep -A 5 "var Images" \
  pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go \
  pkg/cluster/internal/create/actions/installdashboard/dashboard.go \
  pkg/cluster/internal/create/actions/installcertmanager/certmanager.go \
  pkg/cluster/internal/create/actions/installenvoygw/envoygw.go \
  pkg/cluster/internal/create/actions/installmetallb/metallb.go \
  pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go
```
Whatever those slices say IS the source of truth — `allAddonImages` must mirror them exactly.

**STEP B — Update `pkg/internal/doctor/offlinereadiness.go`.**

The current slice (RESEARCH §Code Examples, lines 49-73) is:
```go
var allAddonImages = []addonImage{
	{"docker.io/envoyproxy/envoy:v1.36.2", "Load Balancer (HA)"},
	{"registry:2", "Local Registry"},
	{"quay.io/metallb/controller:v0.15.3", "MetalLB"},
	{"quay.io/metallb/speaker:v0.15.3", "MetalLB"},
	{"registry.k8s.io/metrics-server/metrics-server:v0.8.1", "Metrics Server"},
	{"quay.io/jetstack/cert-manager-cainjector:v1.16.3", "Cert Manager"},
	{"quay.io/jetstack/cert-manager-controller:v1.16.3", "Cert Manager"},
	{"quay.io/jetstack/cert-manager-webhook:v1.16.3", "Cert Manager"},
	{"docker.io/envoyproxy/ratelimit:ae4cee11", "Envoy Gateway"},
	{"envoyproxy/gateway:v1.3.1", "Envoy Gateway"},
	{"ghcr.io/headlamp-k8s/headlamp:v0.40.1", "Dashboard"},
	{"nvcr.io/nvidia/k8s-device-plugin:v0.17.1", "NVIDIA GPU"},
	{"docker.io/rancher/local-path-provisioner:v0.0.35", "Local Path Provisioner"},
	{"docker.io/library/busybox:1.37.0", "Local Path Provisioner"},
}
```

Update each tag to mirror the final delivered state observed in Step A. Examples for the all-Path-A scenario (no holds):
```go
var allAddonImages = []addonImage{
	{"docker.io/envoyproxy/envoy:v1.36.2", "Load Balancer (HA)"},               // unchanged
	{"registry:2", "Local Registry"},                                           // unchanged
	{"quay.io/metallb/controller:v0.15.3", "MetalLB"},                          // 53-05 held
	{"quay.io/metallb/speaker:v0.15.3", "MetalLB"},                             // 53-05 held
	{"registry.k8s.io/metrics-server/metrics-server:v0.8.1", "Metrics Server"}, // 53-06 held
	{"quay.io/jetstack/cert-manager-cainjector:v1.20.2", "Cert Manager"},        // 53-03 bumped
	{"quay.io/jetstack/cert-manager-controller:v1.20.2", "Cert Manager"},        // 53-03 bumped
	{"quay.io/jetstack/cert-manager-webhook:v1.20.2", "Cert Manager"},           // 53-03 bumped
	{"docker.io/envoyproxy/ratelimit:05c08d03", "Envoy Gateway"},                // 53-04 bumped
	{"envoyproxy/gateway:v1.7.2", "Envoy Gateway"},                              // 53-04 bumped
	{"ghcr.io/headlamp-k8s/headlamp:v0.42.0", "Dashboard"},                      // 53-02 bumped
	{"nvcr.io/nvidia/k8s-device-plugin:v0.17.1", "NVIDIA GPU"},                  // unchanged
	{"docker.io/rancher/local-path-provisioner:v0.0.36", "Local Path Provisioner"}, // 53-01 bumped
	{"docker.io/library/busybox:1.37.0", "Local Path Provisioner"},              // unchanged
}
```

**Important:** ONLY change tags. Do NOT add or remove entries. Per RESEARCH §Pitfall ALL-02, the count stays at 14. If a hold path was taken in 53-02/53-03/53-04, keep the OLD tag for those entries — `allAddonImages` always mirrors the live Go source code.

If any prior plan resulted in a hold (Path B), use the held tag here. Do NOT optimistically reflect the v0.42.0 / v1.20.2 / v1.7.2 if the working binary still ships the older version.

**STEP C — Run the offline-readiness test suite.**

```bash
go test ./pkg/internal/doctor/... -run TestAllAddonImages -race -v
go test ./pkg/internal/doctor/... -race
```

Expected:
- `TestAllAddonImages_CountMatchesExpected` passes (count is still 14).
- All other `TestAllAddonImages_*` tests pass.

If the count test fails (count != 14), investigate before adjusting the constant — it indicates an entry was accidentally added or removed. Per RESEARCH §Pitfall ALL-02, verify against the Images slices in the install actions: 1 LB + 1 registry + 2 metallb + 1 metrics + 3 cert-mgr + 2 EG + 1 dashboard + 1 GPU + 2 local-path = 14.

If after careful audit you genuinely need to change the constant (e.g. an addon legitimately added an image), update `pkg/internal/doctor/offlinereadiness_test.go` line 122 (`const expected = 14`) to the new number and document the why in the SUMMARY. (Default expected outcome: NO change to the constant.)

**Do NOT commit yet** — Task 2 adds the CHANGELOG consolidation + release-notes draft atomically.
  </action>
  <verify>
<automated>go test ./pkg/internal/doctor/... -race -run TestAllAddonImages -v && go build ./... && grep -c "addonImage{" pkg/internal/doctor/offlinereadiness.go | awk '{ if ($1 != 14) exit 1 }'</automated>
  </verify>
  <done>`allAddonImages` slice mirrors the final delivered state of every addon (bumped tags for delivered Path-A addons; old tags for any Path-B holds). Count remains 14. Full doctor package -race test suite green. `expected = 14` constant in `offlinereadiness_test.go` unchanged (default).</done>
</task>

<task type="auto">
  <name>Task 2: Consolidate CHANGELOG stubs into v2.4 — Hardening + write tier-3 release-notes draft + atomic commit</name>
  <files>
    pkg/internal/doctor/offlinereadiness.go
    pkg/internal/doctor/offlinereadiness_test.go
    kinder-site/src/content/docs/changelog.md
    .planning/release-notes-v2.4-draft.md
  </files>
  <action>
**STEP A — Read the existing v2.4 stubs accumulated by 53-00 through 53-06.**

The CHANGELOG should currently have an `## v2.4 — Hardening (in progress)` H2 with one bullet per prior plan. Read the entire H2 block.

**STEP B — Reformat into a polished v2.4 section.**

Match the section structure of the existing `## v1.5 — Inner Loop` H2 (RESEARCH §State of the Art and the existing changelog file are the references). Suggested final structure for `kinder-site/src/content/docs/changelog.md`:

```markdown
## v2.4 — Hardening

**Released:** <release date — leave as TBD until phase 58 runs and v2.4 ships>

Phase 53 brought all kinder addons to current-stable releases (where available), closed a security advisory in local-path-provisioner, and re-verified the SYNC default node image gate.

### Addon Bumps

- **`local-path-provisioner` v0.0.35 → v0.0.36** — closes [GHSA-7fxv-8wr2-mfc4](https://github.com/rancher/local-path-provisioner/security/advisories/GHSA-7fxv-8wr2-mfc4) HelperPod Template Injection security advisory. Embedded `busybox:1.37.0` pin and `is-default-class` StorageClass annotation preserved. (ADDON-01)
- **`Headlamp` v0.40.1 → v0.42.0** — token-print authentication flow re-verified live (`kubectl auth can-i` + UI curl with the printed SA token both succeed). Existing kinder-specific Secret + `-in-cluster` deployment arg pattern preserved. (ADDON-02) [OR if held: `Headlamp` HELD at v0.40.1 — see hold note below]
- **`cert-manager` v1.16.3 → v1.20.2** — `--server-side` apply preserved (manifest is 992 KB, exceeds 256 KB annotation limit). Live UAT verified self-signed ClusterIssuer issues a Certificate and pods run as UID `65532`. (ADDON-03) [OR if held: `cert-manager` HELD at v1.16.3 — see hold note below]
- **`Envoy Gateway` v1.3.1 → v1.7.2** (single-jump). Bundled Gateway API CRDs upgrade from `v1.2.1` to `v1.4.1` in-band. Live HTTPRoute end-to-end UAT verified traffic returns 200. `eg-gateway-helm-certgen` Job name unchanged. Ratelimit image bumped to `05c08d03`. (ADDON-04) [OR if held: `Envoy Gateway` HELD at v1.3.1 — see hold note below]

### Documented Holds

- **`MetalLB` held at v0.15.3** — verified upstream `metallb/metallb` latest release is still v0.15.3.
- **`Metrics Server` held at v0.8.1** — verified upstream `kubernetes-sigs/metrics-server` latest release is still v0.8.1.
[If any 53-02/53-03/53-04 went Path B, list the failed-bump-held items here too with the same format from the per-plan stubs.]

### Breaking Changes for Upgraders

[Only include subsections for addons that actually shipped Path A AND have breaking changes.]

#### cert-manager v1.20

- Container UID changed from `1000` to `65532`. PVCs/mounted Secrets pre-populated with files owned by UID 1000 will be unreadable to the new pods. cert-manager itself does not use PVCs, but custom integrations sharing volumes may break.
- `Certificate.spec.privateKey.rotationPolicy: Always` is GA-mandatory. Long-running Certificates without an explicit `rotationPolicy: Never` will see a NEW private key on next renewal.

#### Envoy Gateway v1.7

- Gateway API CRD bundle upgrades from `v1.2.1` to `v1.4.1`. v1alpha2-deprecated fields may need updates in custom HTTPRoute manifests.
- Existing kinder clusters are unaffected — addon versions are pinned at cluster-create time. Recreate the cluster to pick up v1.7.2.

### SYNC-05 (Default Node Image)

[If 53-00 was Outcome A:]
- **Default `kindest/node` bumped to `<TAG>` (Kubernetes 1.36.x)** — `kinder create cluster` without an explicit `image:` field now provisions a Kubernetes 1.36 node. Existing clusters are unaffected; recreate to pick up the new default. (SYNC-05)

[If 53-00 was Outcome B:]
- **SYNC-05 deferred** — `kindest/node:v1.36.x` not yet on Docker Hub at execute time; the default node image remains `v1.35.1`. Re-runnable in v2.5 once kind publishes a v1.36 image.

### Internal

- `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` updated to reflect all delivered addon tags. Count remains 14 (no addon added or removed an image; only tags shifted). `TestAllAddonImages_CountMatchesExpected` passes unchanged. (ADDON-05)
```

Replace the per-plan stubs with this consolidated structure. Do NOT delete the historical content of any prior milestone H2 (e.g. `## v1.5 — Inner Loop`).

**STEP C — Write tier 3 of the three-tier disclosure: v2.4 release-notes draft.**

Per CONTEXT.md "Three-tier disclosure": tier 1 is CHANGELOG.md (Step B above), tier 2 is addon docs (already landed in 53-02..53-04 atomically), and **tier 3 is the v2.4 release-notes draft (GitHub release body)**. No prior `release-notes-*` file exists in `.planning/` (verified by `find .planning -name "release-notes*"`), so create a new file at `.planning/release-notes-v2.4-draft.md`.

The release-notes draft is shorter than the CHANGELOG (which is the full historical log) and oriented toward the GitHub release body — it lifts the Breaking Changes and Upgraders' Notes from the CHANGELOG and frames them for the user clicking through from a release notification.

Create `.planning/release-notes-v2.4-draft.md`:

```markdown
# kinder v2.4 — Hardening (DRAFT)

**Status:** Draft — finalize when phase 58 ships v2.4.
**Source of truth:** `kinder-site/src/content/docs/changelog.md` `## v2.4 — Hardening` (this draft mirrors the user-facing slice).

## Highlights

Phase 53 brings all kinder addons to current-stable releases (where available), closes a HelperPod Template Injection advisory in local-path-provisioner, and re-verifies the default node image gate against Kubernetes 1.36.

- Addons bumped: local-path-provisioner, Headlamp, cert-manager, Envoy Gateway. (Hold disposition per addon recorded in CHANGELOG; defaults below assume all Path A.)
- Bundled Gateway API CRDs advance from v1.2.1 to v1.4.1 in-band with EG v1.7.
- cert-manager containers now run as the unprivileged UID `65532` (upstream default change).
- `Certificate.spec.privateKey.rotationPolicy: Always` is GA-mandatory in cert-manager v1.20.
- SYNC-05: default `kindest/node` image — see "SYNC-05" section below for the disposition decided at execute time.

## Upgraders' Notes

This is the section that matters to anyone running `kinder` v2.3 today.

### cert-manager UID 1000 → 65532

cert-manager pods now run as **UID 65532** (the upstream "non-root unprivileged" default since v1.20). cert-manager itself does not use PersistentVolumes, but if you mount Secrets pre-populated with files owned by UID 1000 into a cert-manager-adjacent pod, ownership/readability will break.

If you have custom integrations sharing volumes with cert-manager, audit Secret/PVC file ownership before upgrading.

### cert-manager rotationPolicy: Always now mandatory

`Certificate.spec.privateKey.rotationPolicy: Always` is now GA-mandatory. v1.18 changed the default from `Never` to `Always`; v1.20 makes it required.

If you depend on a stable private key across renewals (uncommon, but possible for some HSM-style flows), set `rotationPolicy: Never` explicitly on those `Certificate` resources before upgrading. kinder does NOT patch this for you — it is a behavior of the cert-manager controller.

### Envoy Gateway v1.3 → v1.7 (single-jump) + Gateway API v1.4.1

This is a single-step bump from EG v1.3.1 to v1.7.2 (the staged 1.3 → 1.5 → 1.7 path is documented as a fallback in the phase plan but was not exercised). Bundled Gateway API CRDs advance from v1.2.1 to v1.4.1 in-band — `kinder` rolls out the new CRDs as part of cluster create.

`v1alpha2`-deprecated fields in custom HTTPRoute manifests may need updates. Existing kinder clusters are unaffected — addon versions are pinned at cluster-create time. **Recreate the cluster to pick up v1.7.2.**

### SYNC-05: Default node image (Kubernetes 1.36)

[If 53-00 was Outcome A:]
- The default `kindest/node` image bumps to `<TAG>` (Kubernetes 1.36.x). `kinder create cluster` without an explicit `image:` field now provisions a 1.36 node. Existing clusters are unaffected — recreate to pick up the new default.

[If 53-00 was Outcome B:]
- SYNC-05 is **deferred**: `kindest/node:v1.36.x` was not yet published on Docker Hub at the time v2.4 shipped. The default node image remains `kindest/node:v1.35.1`. SYNC-05 will be re-evaluated in v2.5 once kind publishes a v1.36 image.

### Held addons

- **MetalLB** stays at v0.15.3 (upstream latest verified at execute time).
- **Metrics Server** stays at v0.8.1 (upstream latest verified at execute time).
- [If any 53-02 / 53-03 / 53-04 held: list here too — e.g. "Headlamp held at v0.40.1 — token UAT failed; see CHANGELOG entry for the reason."]

## Internal Changes (informational)

- `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` mirrors all delivered addon tags. Image count constant unchanged at 14 (no addon added or removed an image; only tags shifted). `kinder doctor offline-readiness` now matches the new addon image set.
- Three-tier breaking-change disclosure landed: this release-notes draft (tier 3), the addon docs (tier 2; already on the website), and the changelog (tier 1).

## Verification

`kinder doctor offline-readiness` against a fresh default cluster reports no `warn|missing` lines (SC1). Live verification transcript is recorded in `.planning/phases/53-addon-version-audit-bumps-sync-05/53-07-SUMMARY.md`.
```

This file is a *draft* — it lives in `.planning/` (project-local, not user-facing). When v2.4 actually ships in phase 58, the project owner copies/pastes this draft into the GitHub release body and finalizes the date.

**STEP D — Final test sweep.**

```bash
go test ./pkg/internal/doctor/... -race
go test ./pkg/cluster/internal/create/actions/... -race
go build ./...
go vet ./...
```

All must exit 0.

**STEP E — Atomic commit (offlinereadiness + CHANGELOG + release-notes draft).**

```bash
git add pkg/internal/doctor/offlinereadiness.go \
        pkg/internal/doctor/offlinereadiness_test.go \
        kinder-site/src/content/docs/changelog.md \
        .planning/release-notes-v2.4-draft.md
git commit -m "feat(53-07): consolidate offlinereadiness allAddonImages + v2.4 changelog + release-notes draft (ADDON-05)"
```

(Note: `offlinereadiness_test.go` is in the commit only if you genuinely changed the `expected = 14` constant; default is no change.)
  </action>
  <verify>
<automated>go test ./pkg/internal/doctor/... -race && go test ./pkg/cluster/internal/create/actions/... -race && go build ./... && go vet ./... && grep -q "## v2.4 — Hardening" kinder-site/src/content/docs/changelog.md && test -f .planning/release-notes-v2.4-draft.md && grep -q "## Upgraders' Notes" .planning/release-notes-v2.4-draft.md</automated>
  </verify>
  <done>(1) `allAddonImages` mirrors the final delivered state. (2) Per-plan CHANGELOG stubs replaced by a polished `## v2.4 — Hardening` section with subsections for Addon Bumps, Documented Holds, Breaking Changes, SYNC-05, and Internal. (3) `.planning/release-notes-v2.4-draft.md` exists with `## Upgraders' Notes` section covering cert-manager UID 65532 + rotationPolicy, EG v1.3→v1.7 + Gateway API v1.4.1, and SYNC-05 disposition (tier 3 of three-tier disclosure). (4) Full doctor + addon action -race test suites pass. (5) `go build ./...` and `go vet ./...` pass. (6) Atomic commit landed.</done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 3: Live UAT — `kinder doctor offline-readiness` no-warn on fresh default cluster (SC1 second clause)</name>
  <what-built>Phase 53 source-tree changes are landed (Tasks 1-2 commit). The remaining acceptance check is SC1's second clause: the live `kinder doctor offline-readiness` command must NOT report any `warn|missing` lines on a fresh default cluster. This cannot be exercised by `go test` — it requires a real cluster pulling the actual addon images and the doctor checking them against the live runtime. This is consistent with the live-UAT philosophy already used by 53-02 / 53-03 / 53-04.</what-built>
  <how-to-verify>
**This is the SC1-second-clause gate. The check is mechanical (grep) but the cluster lifecycle is live (cluster create + delete).**

1. Build the binary from the post-Task-2 working tree:
   ```bash
   go build -o bin/kinder ./
   ```

2. Create a fresh default cluster (no `--image` override — uses the post-53-00 default):
   ```bash
   ./bin/kinder create cluster --name uat-53-07
   ```
   Wait for the cluster-create to complete cleanly. If it fails, that itself is a phase regression — investigate before re-running.

3. Run the doctor and capture output:
   ```bash
   ./bin/kinder doctor offline-readiness 2>&1 | tee /tmp/53-07-doctor.out
   ```

4. Mechanical assertion — no `warn` or `missing` lines:
   ```bash
   if grep -E '^(warn|missing)\b' /tmp/53-07-doctor.out; then
     echo "FAIL: kinder doctor offline-readiness reports warn/missing lines (paste above)"
   else
     echo "PASS: no warn|missing lines"
   fi
   ```
   The exact `^(warn|missing)\b` regex may need adjustment if the doctor output format uses a different prefix (e.g. `WARN:` or `[warn]`). Inspect `/tmp/53-07-doctor.out` first; the assertion is "no line indicates a warn/missing addon image" regardless of exact format.

5. Tear down:
   ```bash
   ./bin/kinder delete cluster --name uat-53-07
   ```

6. Append the captured `/tmp/53-07-doctor.out` to `.planning/phases/53-addon-version-audit-bumps-sync-05/53-07-SUMMARY.md` under a `## SC1 Live Verification` H2.

**Decision tree:**

- **No `warn|missing` lines** → reply `pass` (SC1 second clause satisfied; phase 53 closes; no further commits).
- **One or more `warn|missing` lines** → reply `fail` with the offending lines pasted. Failure means a tag in `allAddonImages` does not match an image actually pulled by an addon's `Execute` — most likely cause is that 53-07 Task 1 used the wrong tag (mismatch with the addon's `Images` slice). Re-cross-check Task 1 STEP A grep; fix the offending tag; re-run from Step 1.
  </how-to-verify>
  <resume-signal>Reply `pass` if `kinder doctor offline-readiness` reports no warn|missing lines; reply `fail` with the offending lines if any are present.</resume-signal>
</task>

</tasks>

<verification>
- `go test ./pkg/internal/doctor/... -race` exits 0 (with `TestAllAddonImages_CountMatchesExpected` passing, count == 14).
- `go test ./pkg/cluster/internal/create/actions/... -race` exits 0.
- `go build ./...` exits 0.
- `go vet ./...` exits 0.
- Each tag in `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` matches the corresponding entry in the relevant `installX/X.go` Images slice (cross-check by grep).
- `kinder-site/src/content/docs/changelog.md` has a polished `## v2.4 — Hardening` H2 (with sub-sections), replacing the per-plan stubs.
- `.planning/release-notes-v2.4-draft.md` exists and contains a `## Upgraders' Notes` section covering cert-manager UID 65532 + rotationPolicy, EG v1.3→v1.7 + Gateway API v1.4.1, and SYNC-05 disposition.
- Task 3 live UAT: `kinder doctor offline-readiness` against a fresh default cluster reports no `warn|missing` lines (SC1 second clause); transcript appended to 53-07-SUMMARY.md.
- `git log --oneline -1` shows a single `feat(53-07): ...` commit (Tasks 1-2 atomic). Task 3 is verification-only; no commit required.
</verification>

<success_criteria>
SC1 (Phase 53, both clauses): `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` and `TestAllAddonImages_CountMatchesExpected` reflect all bumped image references; `go test ./pkg/internal/doctor/... -run TestAllAddonImages` passes (first clause); AND `kinder doctor offline-readiness` does not warn on a fresh default cluster (second clause, verified live in Task 3).

SC5 (Phase 53): same artifacts as SC1's first clause. Now also covered by Task 3's live verification that the addon image set is consistent end-to-end.

ADDON-05: All bumped image refs propagated to `allAddonImages`; count constant updated only if genuinely needed (default: no change, count remains 14).

Three-tier disclosure (CONTEXT.md D-lock): tier 1 (CHANGELOG.md `## v2.4 — Hardening`) and tier 3 (`.planning/release-notes-v2.4-draft.md`) land in this plan; tier 2 (addon docs) was landed atomically with each bump in 53-02..53-04.

This plan is the phase-closing commit for 53. Phase 53 is ready for `/gsd-verify-work` once Task 3's live UAT returns `pass`.
</success_criteria>

<output>
`.planning/phases/53-addon-version-audit-bumps-sync-05/53-07-SUMMARY.md` records:
- The final state of `allAddonImages` (paste the full slice as it now exists in the source)
- Cross-check showing each entry matches its corresponding addon's Images slice
- Whether `expected = 14` constant was unchanged (default) or changed (with the why)
- The CHANGELOG consolidation diff summary (what stubs were merged, which sub-sections were created)
- A pointer to `.planning/release-notes-v2.4-draft.md` confirming tier 3 of the three-tier disclosure shipped (and reciting which Upgraders' Notes subsections are present)
- `## SC1 Live Verification` H2 with the captured `kinder doctor offline-readiness` output and the `pass`/`fail` verdict
- Phase 53 disposition summary: how many addons delivered (Path A) vs held (Path B), whether SYNC-05 fired (Outcome A) or deferred (Outcome B)
</output>
