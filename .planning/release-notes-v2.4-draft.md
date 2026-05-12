# kinder v2.4 — Hardening (DRAFT)

**Status:** Draft — finalize when phase 58 ships v2.4.
**Source of truth:** `kinder-site/src/content/docs/changelog.md` `## v2.4 — Hardening` (this draft mirrors the user-facing slice).

## Highlights

Phase 53 brings all kinder addons to current-stable releases (where available), closes a HelperPod Template Injection advisory in local-path-provisioner, and re-verifies the default node image gate against Kubernetes 1.36.

- Addons bumped: local-path-provisioner v0.0.35 → v0.0.36, Headlamp v0.40.1 → v0.42.0, cert-manager v1.16.3 → v1.20.2, Envoy Gateway v1.3.1 → v1.7.2.
- Bundled Gateway API CRDs advance from v1.2.1 to v1.4.1 in-band with EG v1.7.
- cert-manager containers now run as the unprivileged UID `65532` (upstream default change since v1.20).
- `Certificate.spec.privateKey.rotationPolicy: Always` is GA-mandatory in cert-manager v1.20.
- SYNC-05 (default `kindest/node` image): deferred — see section below.

## Upgraders' Notes

This section matters to anyone running `kinder` v2.3 today and upgrading to v2.4.

### cert-manager UID 1000 → 65532

cert-manager pods now run as **UID 65532** (the upstream "non-root unprivileged" default since v1.20, enforced via the distroless image `USER nonroot` directive and kubelet `runAsNonRoot: true`). cert-manager itself does not use PersistentVolumes, but if you mount Secrets pre-populated with files owned by UID 1000 into a cert-manager-adjacent pod, ownership/readability will break.

If you have custom integrations sharing volumes with cert-manager, audit Secret/PVC file ownership before upgrading.

### cert-manager rotationPolicy: Always now mandatory

`Certificate.spec.privateKey.rotationPolicy: Always` is now GA-mandatory. v1.18 changed the default from `Never` to `Always`; v1.20 makes an explicit value required.

If you depend on a stable private key across renewals (uncommon, but possible for some HSM-style flows), set `rotationPolicy: Never` explicitly on those `Certificate` resources before upgrading. kinder does NOT patch this for you — it is a behavior of the cert-manager controller.

### Envoy Gateway v1.3 → v1.7 (single-jump) + Gateway API v1.4.1

This is a single-step bump from EG v1.3.1 to v1.7.2 (skipping v1.5.x). Bundled Gateway API CRDs advance from v1.2.1 to v1.4.1 in-band — `kinder` rolls out the new CRDs as part of cluster create.

`v1alpha2`-deprecated fields in custom HTTPRoute manifests may need updates. Existing kinder clusters are unaffected — addon versions are pinned at cluster-create time. **Recreate the cluster to pick up v1.7.2.**

### SYNC-05: Default node image (Kubernetes 1.36)

SYNC-05 is **deferred**: `kindest/node:v1.36.x` was not yet published on Docker Hub at the time v2.4 was cut (Docker Hub API returned `count: 0` for `?name=v1.36`). The default node image remains `kindest/node:v1.35.1`. SYNC-05 will be re-evaluated in v2.5 once kind publishes a v1.36 image.

### Held addons

- **MetalLB** stays at v0.15.3 (upstream latest verified at execute time, published 2025-12-04).
- **Metrics Server** stays at v0.8.1 (upstream latest verified at execute time, published 2026-01-29).

## Internal Changes (informational)

- `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` mirrors all delivered addon tags. Image count constant unchanged at 14 (no addon added or removed an image; only tags shifted). `kinder doctor offline-readiness` now checks against the new addon image set.
- Three-tier breaking-change disclosure landed: this release-notes draft (tier 3), the addon docs (tier 2; cert-manager and Envoy Gateway addon pages updated in 53-03 and 53-04), and the changelog `## v2.4 — Hardening` section (tier 1).

## Verification

`kinder doctor offline-readiness` against a fresh default cluster reports no `warn|missing` lines (SC1 second clause). Live verification transcript is recorded in `.planning/phases/53-addon-version-audit-bumps-sync-05/53-07-SUMMARY.md`.
