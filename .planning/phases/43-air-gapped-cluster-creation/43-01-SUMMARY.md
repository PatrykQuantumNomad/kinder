---
phase: 43-air-gapped-cluster-creation
plan: 01
subsystem: config, cluster, providers/common
tags: [air-gapped, config, cli, addon-images]
dependency_graph:
  requires: []
  provides: [AirGapped config field, CreateWithAirGapped option, --air-gapped CLI flag, addon Images vars, RequiredAddonImages utility]
  affects: [pkg/internal/apis/config, pkg/apis/config/v1alpha4, pkg/cluster, pkg/cmd, pkg/cluster/internal/providers/common]
tech_stack:
  added: []
  patterns: [option function pattern (CreateWithRetain), exported package-level vars for image manifests]
key_files:
  created:
    - path: pkg/cluster/internal/providers/common/images_test.go
      note: tests for RequiredAddonImages and RequiredAllImages (replaced existing file with expanded tests)
  modified:
    - pkg/internal/apis/config/types.go
    - pkg/apis/config/v1alpha4/types.go
    - pkg/internal/apis/config/convert_v1alpha4.go
    - pkg/cluster/createoption.go
    - pkg/cluster/internal/create/create.go
    - pkg/cmd/kind/create/cluster/createcluster.go
    - pkg/cluster/internal/create/actions/installmetallb/metallb.go
    - pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go
    - pkg/cluster/internal/create/actions/installcertmanager/certmanager.go
    - pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
    - pkg/cluster/internal/create/actions/installdashboard/dashboard.go
    - pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go
    - pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu.go
    - pkg/cluster/internal/providers/common/images.go
decisions:
  - "AirGapped bool added after Addons field in both internal and v1alpha4 Cluster structs to maintain field ordering convention"
  - "CLI flag --air-gapped placed after --profile in flag registration to group addon-related flags"
  - "RequiredAddonImages imports addon packages into common/images.go; verified no import cycle (addon packages import actions/nodeutils, not common)"
  - "localregistry Images var references existing private registryImage const rather than duplicating the string"
metrics:
  duration: ~15 minutes
  completed: "2026-04-09T12:16:50Z"
  tasks_completed: 2
  files_modified: 14
---

# Phase 43 Plan 01: Air-Gapped Flag Plumbing and Addon Image Constants Summary

AirGapped bool field wired from `--air-gapped` CLI flag through `CreateWithAirGapped` option into `config.Cluster.AirGapped`; all 7 addon packages export `var Images []string`; `RequiredAddonImages` and `RequiredAllImages` utilities added to `common/images.go` with full test coverage.

## What Was Built

### Task 1: AirGapped config field and CLI flag

- Added `AirGapped bool` to `pkg/internal/apis/config/types.go` (internal `Cluster` struct, after `Addons`)
- Added `AirGapped bool` with `yaml:"airGapped,omitempty"` tags to `pkg/apis/config/v1alpha4/types.go`
- Propagated `in.AirGapped -> out.AirGapped` in `Convertv1alpha4` in `convert_v1alpha4.go`
- Added `AirGapped bool` to `ClusterOptions` struct and `opts.Config.AirGapped = true` in `fixupOptions` when `opts.AirGapped` is set
- Added `CreateWithAirGapped(airGapped bool) CreateOption` to `createoption.go` following the `CreateWithRetain` pattern
- Added `--air-gapped` flag to `createcluster.go` flagpole and wired it via `cluster.CreateWithAirGapped(flags.AirGapped)` in `runE`

### Task 2: Addon image exports and RequiredAddonImages utility

Exported `var Images []string` from all 7 addon packages with image refs verified against embedded manifests:
- `installmetallb`: `quay.io/metallb/controller:v0.15.3`, `quay.io/metallb/speaker:v0.15.3`
- `installmetricsserver`: `registry.k8s.io/metrics-server/metrics-server:v0.8.1`
- `installcertmanager`: 3 jetstack images at v1.16.3
- `installenvoygw`: `docker.io/envoyproxy/ratelimit:ae4cee11`, `envoyproxy/gateway:v1.3.1`
- `installdashboard`: `ghcr.io/headlamp-k8s/headlamp:v0.40.1`
- `installlocalregistry`: `registry:2` (via existing `registryImage` const)
- `installnvidiagpu`: `nvcr.io/nvidia/k8s-device-plugin:v0.17.1`

Added to `common/images.go`:
- `RequiredAddonImages(cfg)`: returns union of enabled addon images + LB image for multi-CP clusters
- `RequiredAllImages(cfg)`: unions `RequiredNodeImages` + `RequiredAddonImages`

Test coverage in `common/images_test.go`:
- `TestRequiredNodeImages` (existing, preserved)
- `TestRequiredAddonImages_AllDisabled`
- `TestRequiredAddonImages_LBOnlyWithMultiCP`
- `TestRequiredAddonImages_MetalLBOnly`
- `TestRequiredAddonImages_AllEnabled`
- `TestRequiredAllImages`

## Verification Results

- `go build ./...` — passes, no import cycles
- `go test ./pkg/cluster/internal/providers/common/...` — passes (6 tests)
- `go vet ./...` — no issues
- All 7 addon packages export `var Images` confirmed via grep
- AirGapped field present in all 3 config locations confirmed via grep

## Commits

| Hash | Message |
|------|---------|
| `7feb7044` | feat(43-01): add AirGapped field and wire --air-gapped CLI flag |
| `c3310735` | feat(43-01): export addon Images vars and add RequiredAddonImages utility |

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None. All image references are real values sourced from embedded manifests, not placeholders.

## Threat Flags

No new network endpoints, auth paths, or trust boundary changes introduced. The `AirGapped` field is purely a local config flag that will be read by subsequent plans (43-02/43-03) to gate behavior at cluster creation time.

## Self-Check: PASSED
