---
phase: 53-addon-version-audit-bumps-sync-05
plan: "01"
subsystem: installlocalpath
tags: [security, addon-bump, local-path-provisioner, tdd]
dependency_graph:
  requires: ["53-00"]
  provides: ["local-path-provisioner v0.0.36 baseline"]
  affects: ["53-07-offlinereadiness-consolidation"]
tech_stack:
  added: []
  patterns: [tdd-red-green, go-embed-manifest]
key_files:
  created: []
  modified:
    - pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go
    - pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go
    - pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml
    - kinder-site/src/content/docs/addons/local-path-provisioner.md
    - kinder-site/src/content/docs/changelog.md
decisions:
  - "TestManifestPinsBusybox threshold lowered to >= 1 because upstream v0.0.36 dropped the --helper-image deployment flag; only one busybox occurrence remains (helperPod.yaml ConfigMap template)"
  - "Accepted upstream v0.0.36 RBAC simplification: Role reduced to pods-only; persistentvolumes/persistentvolumeclaims/events/configmaps/storageclasses moved to ClusterRole"
  - "Accepted new CONFIG_MOUNT_PATH env var from upstream v0.0.36 deployment spec"
metrics:
  duration: "191s"
  completed: "2026-05-10"
  tasks_completed: 2
  files_changed: 5
---

# Phase 53 Plan 01: Local-Path-Provisioner v0.0.36 Bump Summary

**One-liner:** Bumped local-path-provisioner v0.0.35 → v0.0.36 (closes CVE GHSA-7fxv-8wr2-mfc4 HelperPod Template Injection) with TDD RED/GREEN pair; kinder overrides re-applied (busybox:1.37.0 pin + is-default-class annotation).

## Commits

| Task | Type | Commit | Description |
|------|------|--------|-------------|
| 1 (RED) | test | `810cc726` | Add failing test pinning local-path-provisioner v0.0.36 + override guards |
| 2 (GREEN) | feat | `3629c246` | Bump local-path-provisioner to v0.0.36 (CVE GHSA-7fxv-8wr2-mfc4) |

## Test Output (GREEN)

```
=== RUN   TestImagesPinsV0036
--- PASS: TestImagesPinsV0036 (0.00s)
=== RUN   TestManifestPinsBusybox
--- PASS: TestManifestPinsBusybox (0.00s)
=== RUN   TestStorageClassIsDefault
--- PASS: TestStorageClassIsDefault (0.00s)
=== RUN   TestExecute
    --- PASS: TestExecute/all_steps_succeed (0.00s)
    --- PASS: TestExecute/apply_manifest_fails (0.00s)
    --- PASS: TestExecute/wait_deployment_fails (0.00s)
=== RUN   TestImages
--- PASS: TestImages (0.00s)
PASS
ok  sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installlocalpath  1.293s (race detector clean)
```

## Manifest Diff — Key Override Points

### busybox:1.37.0 pin (helperPod.yaml ConfigMap template)

```diff
-        image: docker.io/library/busybox
+        image: busybox:1.37.0
```

### is-default-class annotation (StorageClass)

```diff
 apiVersion: storage.k8s.io/v1
 kind: StorageClass
 metadata:
   name: local-path
+  annotations:
+    storageclass.kubernetes.io/is-default-class: "true"
 provisioner: rancher.io/local-path
```

### Images slice (localpathprovisioner.go)

```diff
-	"docker.io/rancher/local-path-provisioner:v0.0.35",
+	"docker.io/rancher/local-path-provisioner:v0.0.36",
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestManifestPinsBusybox threshold lowered from >= 2 to >= 1**

- **Found during:** Task 2 (GREEN) — inspecting upstream v0.0.36 manifest
- **Issue:** The plan assumed v0.0.36 still had the `--helper-image` deployment flag (giving two busybox occurrences). Upstream v0.0.36 dropped `--helper-image` entirely. The deployment no longer has a `busybox` reference; only the helperPod.yaml ConfigMap template remains.
- **Fix:** Updated `TestManifestPinsBusybox` to require `count >= 1` (not `>= 2`) and updated the test comment to document why v0.0.36 has one occurrence.
- **Files modified:** `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go`
- **Commit:** `3629c246` (included in GREEN commit)

**2. [Rule 2 - RBAC] Accepted upstream v0.0.36 Role reduction**

- **Found during:** Task 2 (GREEN) — diff of upstream v0.0.36 vs kinder v0.0.35 manifest
- **Issue:** Upstream v0.0.36 significantly reduced the namespaced Role (pods-only) and moved persistentvolumes/events/configmaps/storageclasses to the ClusterRole. This is an intentional upstream security tightening.
- **Fix:** Accepted upstream RBAC changes as part of the security fix (this is part of GHSA-7fxv-8wr2-mfc4 remediation).
- **Files modified:** `manifests/local-path-storage.yaml`
- **Commit:** `3629c246`

**3. [Rule 2 - Env] Accepted CONFIG_MOUNT_PATH env var from upstream v0.0.36**

- **Found during:** Task 2 (GREEN)
- **Issue:** Upstream v0.0.36 adds a `CONFIG_MOUNT_PATH: /etc/config/` environment variable to the deployment container. This is a correctness addition that makes the mount path explicit.
- **Fix:** Retained in the vendored manifest.
- **Commit:** `3629c246`

## TDD Gate Compliance

- RED gate: `test(53-01): add failing test pinning local-path-provisioner v0.0.36 + override guards` — `810cc726`
- GREEN gate: `feat(53-01): bump local-path-provisioner to v0.0.36 (CVE GHSA-7fxv-8wr2-mfc4)` — `3629c246`

RED commit preceded GREEN commit. Both gates are present in git log.

## offlinereadiness.go — Deferred

The `pkg/cluster/internal/doctor/offlinereadiness.go` allAddonImages list is NOT updated in this plan. That consolidation (updating the doctor offline-readiness check to reference v0.0.36) is deferred to **Plan 53-07 (offlinereadiness-consolidation)** where all addon image bumps are applied together.

## Known Stubs

None — the Images slice and manifest are fully wired to production values (v0.0.36 tag).

## Self-Check

- [x] `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go` — FOUND, contains v0.0.36
- [x] `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go` — FOUND, contains TestImagesPinsV0036
- [x] `pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` — FOUND, contains v0.0.36 + busybox:1.37.0 + is-default-class
- [x] `kinder-site/src/content/docs/addons/local-path-provisioner.md` — FOUND, v0.0.36 + GHSA note
- [x] `kinder-site/src/content/docs/changelog.md` — FOUND, v2.4 H2 + ADDON-01 line

Commit `810cc726` (RED): FOUND in git log
Commit `3629c246` (GREEN): FOUND in git log

## Self-Check: PASSED
