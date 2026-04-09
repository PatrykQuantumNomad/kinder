---
phase: 44-local-path-provisioner-addon
plan: "01"
subsystem: storage/addon
tags: [local-path-provisioner, storage, addon, config-pipeline]
dependency_graph:
  requires: []
  provides:
    - installlocalpath action package with embedded manifest
    - LocalPath field in 5-location config pipeline
    - installstorage gate in sequential pipeline
    - wave1 registration for local-path-provisioner
    - RequiredAddonImages registration for local-path-provisioner and busybox
  affects:
    - pkg/cluster/internal/create/create.go
    - pkg/cluster/internal/providers/common/images.go
tech_stack:
  added: []
  patterns:
    - embed directive for manifest (same as installmetricsserver)
    - opt-out bool pattern with boolVal (same as MetalLB, CertManager)
    - wave1 parallel addon registration
key_files:
  created:
    - pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go
    - pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml
  modified:
    - pkg/apis/config/v1alpha4/types.go
    - pkg/apis/config/v1alpha4/default.go
    - pkg/apis/config/v1alpha4/zz_generated.deepcopy.go
    - pkg/internal/apis/config/types.go
    - pkg/internal/apis/config/convert_v1alpha4.go
    - pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-new-fields.yaml
    - pkg/internal/apis/config/encoding/load_test.go
    - pkg/cluster/internal/create/create.go
    - pkg/cluster/internal/providers/common/images.go
key_decisions:
  - LocalPath uses boolVal (opt-out, default true) not boolValOptIn — consistent with MetalLB/CertManager pattern
  - StorageClass named local-path (not standard) to avoid collision with legacy installstorage StorageClass
  - installstorage gated behind !LocalPath in sequential pipeline (before kubeadmjoin)
  - Images var uses docker.io/ prefix for both images (docker.io/rancher/local-path-provisioner:v0.0.35, docker.io/library/busybox:1.37.0)
  - busybox pinned to 1.37.0 with IfNotPresent in both manifest command args and helperPod.yaml
metrics:
  duration: ~3m
  completed: "2026-04-09T13:19:14Z"
  started: "2026-04-09T13:16:12Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 9
---

# Phase 44 Plan 01: Local Path Provisioner Config Pipeline and Action Package Summary

**One-liner:** Added LocalPath *bool to the 5-location config pipeline (opt-out, default true), created the `installlocalpath` action package with embedded v0.0.35 manifest (busybox:1.37.0/IfNotPresent, StorageClass `local-path`), gated `installstorage` behind `!LocalPath`, and registered `installlocalpath` in wave1 with image entries in RequiredAddonImages.

## Duration

- Started: 2026-04-09T13:16:12Z
- Completed: 2026-04-09T13:19:14Z
- Duration: ~3 minutes
- Tasks: 2/2
- Files created: 2, files modified: 9

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Add LocalPath to config pipeline (5-location) and create installlocalpath action package | 0fb8ce77 | types.go, default.go, zz_generated.deepcopy.go, convert_v1alpha4.go, localpathprovisioner.go, local-path-storage.yaml, load_test.go, valid-addons-new-fields.yaml |
| 2 | Wire installlocalpath into create.go and images.go | 63b9b71e | create.go, images.go |

## What Was Built

### 5-Location Config Pipeline

1. `pkg/apis/config/v1alpha4/types.go` — `LocalPath *bool` added to `Addons` struct after `CertManager`, with yaml tag `localPath,omitempty`.
2. `pkg/apis/config/v1alpha4/default.go` — `boolPtrTrue(&obj.Addons.LocalPath)` added in `SetDefaultsCluster` (opt-out, default enabled).
3. `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` — nil-check deepcopy block for `LocalPath *bool` added between `CertManager` and `NvidiaGPU`.
4. `pkg/internal/apis/config/types.go` — `LocalPath bool` added to internal `Addons` struct.
5. `pkg/internal/apis/config/convert_v1alpha4.go` — `LocalPath: boolVal(in.Addons.LocalPath)` added (opt-out semantics, nil → true).

### Addon Action Package

- `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go` — follows exact pattern of `installmetricsserver`. Exports `NewAction()` and `Images` var. Uses `//go:embed` for manifest. Applies manifest via `kubectl apply -f -` and waits with `kubectl wait --for=condition=Available`.
- `pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` — patched upstream v0.0.35 manifest. Key changes: StorageClass named `local-path` (not `standard`), annotated as default class, `busybox:1.37.0` with `imagePullPolicy: IfNotPresent` in both Deployment command args and `helperPod.yaml` in ConfigMap.

### Wiring

- `create.go` — `installstorage.NewAction()` now conditional on `!opts.Config.Addons.LocalPath`; `installlocalpath.NewAction()` added to wave1 after Cert Manager.
- `images.go` — `installlocalpath` imported; `LocalPath` branch added to `RequiredAddonImages` inserting both `docker.io/rancher/local-path-provisioner:v0.0.35` and `docker.io/library/busybox:1.37.0`.

### Tests

- `valid-addons-new-fields.yaml` — appended `localPath: false` as third addon override.
- `load_test.go` — `TestAddonsDefaults` Test 1 asserts `LocalPath` defaults true when absent; Test 4 asserts `LocalPath` is false when explicitly set to false.

## Verification Results

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./pkg/internal/apis/config/encoding/...` — PASS (TestAddonsDefaults passes)
- All 5 config pipeline locations confirmed with grep
- `busybox:1.37.0` and `IfNotPresent` confirmed in manifest
- StorageClass `local-path` confirmed in manifest
- `installstorage` gate confirmed in create.go
- `installlocalpath` in wave1 confirmed
- `RequiredAddonImages` branch confirmed in images.go

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — no stubs. The action package is fully wired end-to-end. The manifest is the real upstream v0.0.35 manifest with pinned images.

## Threat Flags

None — no new network endpoints, auth paths, or trust boundary changes introduced. The new package applies a static manifest to a local cluster node via `kubectl` using the existing admin kubeconfig pattern.

## Self-Check: PASSED

- `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go` — FOUND
- `pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` — FOUND
- Commits verified: `0fb8ce77` (Task 1), `63b9b71e` (Task 2)
