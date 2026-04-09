---
phase: 44-local-path-provisioner-addon
verified: 2026-04-09T14:00:00Z
status: human_needed
score: 5/6
overrides_applied: 0
human_verification:
  - test: "Create a single-node cluster with default config, then create a PVC with no storageClassName and confirm it transitions to Bound"
    expected: "PVC reaches Bound state automatically within a few seconds; kubectl get pvc shows STATUS=Bound and the provisioner creates a PersistentVolume under /opt/local-path-provisioner"
    why_human: "Requires a running Kubernetes cluster; cannot verify dynamic provisioning binding behavior with static code analysis alone"
  - test: "Create a multi-node cluster (1 control-plane, 1 worker) with default config, create a PVC with storageClassName: local-path, schedule a pod that mounts it, and confirm binding and pod scheduling succeed"
    expected: "PVC reaches Bound (WaitForFirstConsumer) when pod is scheduled; pod reaches Running state with the volume mounted"
    why_human: "WaitForFirstConsumer binding mode requires actual pod scheduling to trigger binding; cannot simulate the full scheduler + CSI controller loop in unit tests"
---

# Phase 44: Local-Path-Provisioner Addon — Verification Report

**Phase Goal:** Users get automatic dynamic PVC provisioning out of the box via local-path-provisioner as a default addon, with `local-path` as the only default StorageClass and no StorageClass collision

**Verified:** 2026-04-09T14:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Success Criteria

| # | Criterion | Status | Evidence |
|---|-----------|--------|----------|
| 1 | `kinder create cluster` installs local-path-provisioner v0.0.35 and `local-path` is the only default StorageClass (no `standard` StorageClass) | VERIFIED | Manifest has `rancher/local-path-provisioner:v0.0.35` and StorageClass named `local-path` with `is-default-class: "true"`. `installstorage` gated by `!opts.Config.Addons.LocalPath` in create.go (line 225). `standard` not present anywhere in the manifest. |
| 2 | PVC with `storageClassName: local-path` (or no storageClassName) transitions to `Bound` automatically without manual action | HUMAN NEEDED | Requires a running cluster. Code wiring is complete (WaitForFirstConsumer StorageClass, provisioner deployment, kubectl wait), but binding behavior can only be verified at runtime. |
| 3 | `addons.localPath: false` skips the addon and `installstorage` installs the legacy `standard` StorageClass | VERIFIED | `installstorage.NewAction()` only called when `!opts.Config.Addons.LocalPath` (create.go line 225-227). Wave1 entry uses `opts.Config.Addons.LocalPath` as enable flag (line 288). TestAddonsDefaults asserts `LocalPath=false` when explicitly set. valid-addons-new-fields.yaml has `localPath: false`. |
| 4 | `kinder doctor` warns when local-path-provisioner is below v0.0.34 (CVE-2025-62878) | VERIFIED | `localPathCVECheck` in `pkg/internal/doctor/localpath.go` uses `version.ParseSemantic` comparing against threshold `v0.0.34`. Returns `warn` when `ver.LessThan(threshold)`. Registered in `allChecks` (check.go line 82). All 7 test scenarios pass including `TestLocalPathCVE_Vulnerable` (v0.0.33 → warn) and `TestLocalPathCVE_ExactThreshold` (v0.0.34 → ok). |
| 5 | Embedded manifest uses `busybox:1.37.0` with `imagePullPolicy: IfNotPresent` | VERIFIED | Manifest has `busybox:1.37.0` at lines 107 and 169 (Deployment command args and helperPod.yaml in ConfigMap). Both locations have `imagePullPolicy: IfNotPresent` (lines 101 and 170). No `busybox:latest` or `imagePullPolicy: Always` anywhere in manifest. `Images` var in localpathprovisioner.go uses `docker.io/library/busybox:1.37.0`. offlinereadiness.go includes both provisioner and busybox entries. |

## Must-Haves Verified

### From Plan 01 must_haves.truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `LocalPath *bool` field exists on v1alpha4 Addons struct with yaml tag `localPath`, defaults to true | VERIFIED | `pkg/apis/config/v1alpha4/types.go` line 121: `LocalPath *bool \`yaml:"localPath,omitempty"\``. `default.go` line 93: `boolPtrTrue(&obj.Addons.LocalPath)` |
| 2 | `LocalPath bool` on internal Addons struct, converted via `boolVal` (opt-out, not opt-in) | VERIFIED | `pkg/internal/apis/config/types.go` line 89: `LocalPath bool`. `convert_v1alpha4.go` line 65: `LocalPath: boolVal(in.Addons.LocalPath)` |
| 3 | `installlocalpath` action applies embedded manifest and waits for deployment readiness | VERIFIED | `localpathprovisioner.go`: `kubectl apply -f -` with embedded manifest, then `kubectl wait --for=condition=Available deployment/local-path-provisioner --timeout=120s` |
| 4 | Embedded manifest uses `busybox:1.37.0` with `imagePullPolicy: IfNotPresent` | VERIFIED | Manifest lines 107, 169 (image), lines 101, 170 (pullPolicy). No `busybox:latest`. |
| 5 | StorageClass name in manifest is `local-path` (not `standard`) | VERIFIED | Manifest line 126: `name: local-path`. No `standard` present. |
| 6 | `installstorage` is gated out when LocalPath is enabled in the sequential pipeline | VERIFIED | `create.go` lines 225-227: `if !opts.Config.Addons.LocalPath { actionsToRun = append(..., installstorage.NewAction()) }` |
| 7 | `installlocalpath` is added to wave1 addons in create.go | VERIFIED | `create.go` line 288: `{"Local Path Provisioner", opts.Config.Addons.LocalPath, installlocalpath.NewAction()}` in wave1 slice |
| 8 | `RequiredAddonImages` includes local-path-provisioner and busybox images when LocalPath is enabled | VERIFIED | `images.go` lines 70-72: `if cfg.Addons.LocalPath { images.Insert(installlocalpath.Images...) }`. `TestRequiredAddonImages_LocalPathOnly` and `TestRequiredAddonImages_AllEnabled` pass. |
| 9 | `valid-addons-new-fields.yaml` includes `localPath: false` and `TestAddonsDefaults` asserts `LocalPath` default=true and explicit false | VERIFIED | Test fixture line 6: `localPath: false`. `load_test.go` lines 176-177 (default=true assertion) and lines 221-222 (explicit false assertion). Tests pass. |

### From Plan 02 must_haves.truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `installlocalpath` unit tests pass with FakeNode testing all success and failure paths | VERIFIED | `localpathprovisioner_test.go`: `TestExecute` (3 cases: all-succeed, apply-fail, wait-fail), `TestImages`. All pass. |
| 2 | `offlinereadiness` allAddonImages includes local-path-provisioner and busybox:1.37.0 entries | VERIFIED | `offlinereadiness.go` lines 71-72: `docker.io/rancher/local-path-provisioner:v0.0.35` and `docker.io/library/busybox:1.37.0`. Total: 14 (confirmed in comment line 48 and test). |
| 3 | `RequiredAddonImages` tests cover the LocalPath enabled and disabled cases | VERIFIED | `images_test.go`: `TestRequiredAddonImages_LocalPathOnly` (enabled), `TestRequiredAddonImages_AllEnabled` includes `LocalPath: true`, `TestRequiredAddonImages_AllDisabled` (disabled via zero-value). |
| 4 | `go test` passes for all modified packages | VERIFIED | `go build ./...` PASS. Tests for encoding, installlocalpath, doctor, and common providers all pass. |

### From Plan 03 must_haves.truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `kinder doctor` warns when local-path-provisioner is below v0.0.34 (CVE-2025-62878) | VERIFIED | `localpath.go` Run() returns `warn` with CVE reference when `ver.LessThan(v0.0.34)`. `TestLocalPathCVE_Vulnerable` (v0.0.33 → warn) passes. |
| 2 | CVE check skips gracefully when no cluster running or no container runtime | VERIFIED | `realGetProvisionerVersion` returns `("", nil)` when no runtime found or `docker ps` returns no containers. Run() returns `skip` on empty tag. `TestLocalPathCVE_NoCluster` passes. |
| 3 | CVE check passes when local-path-provisioner is at or above v0.0.34 | VERIFIED | Threshold comparison: `ver.LessThan(threshold)` is false for v0.0.34 and above. Returns `ok`. `TestLocalPathCVE_Safe` (v0.0.35) and `TestLocalPathCVE_ExactThreshold` (v0.0.34) both pass. |
| 4 | CVE check is registered in allChecks under Cluster category | VERIFIED | `check.go` line 82: `newLocalPathCVECheck()` in allChecks, after `newClusterNodeSkewCheck()` in the Cluster category comment block. |

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/apis/config/v1alpha4/types.go` | `LocalPath *bool` on Addons struct | VERIFIED | Field present with correct yaml tag |
| `pkg/apis/config/v1alpha4/default.go` | `boolPtrTrue(&obj.Addons.LocalPath)` | VERIFIED | Present in SetDefaultsCluster |
| `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` | nil-check deepcopy for LocalPath | VERIFIED | Nil-check block present (lines 62-65) |
| `pkg/internal/apis/config/types.go` | `LocalPath bool` on internal Addons | VERIFIED | Field present (line 89) |
| `pkg/internal/apis/config/convert_v1alpha4.go` | `LocalPath: boolVal(in.Addons.LocalPath)` | VERIFIED | Present in conversion block (line 65) |
| `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go` | `NewAction()`, `Images` var | VERIFIED | Both exported; `//go:embed` directive; apply + wait logic |
| `pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` | v0.0.35 manifest, busybox:1.37.0, IfNotPresent, local-path SC | VERIFIED | All four properties confirmed present, no regressions |
| `pkg/cluster/internal/create/create.go` | installstorage gate and installlocalpath in wave1 | VERIFIED | Gate at line 225, wave1 entry at line 288 |
| `pkg/cluster/internal/providers/common/images.go` | LocalPath branch in RequiredAddonImages | VERIFIED | Branch at lines 70-72 |
| `pkg/internal/apis/config/encoding/testdata/v1alpha4/valid-addons-new-fields.yaml` | `localPath: false` | VERIFIED | Line 6 |
| `pkg/internal/apis/config/encoding/load_test.go` | TestAddonsDefaults LocalPath assertions | VERIFIED | Lines 176-177, 221-222 |
| `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go` | TestExecute, TestImages | VERIFIED | All tests pass |
| `pkg/internal/doctor/offlinereadiness.go` | local-path-provisioner and busybox entries | VERIFIED | Lines 71-72, total 14 |
| `pkg/internal/doctor/localpath.go` | localPathCVECheck with CVE-2025-62878 | VERIFIED | Full implementation with injectable dependency |
| `pkg/internal/doctor/localpath_test.go` | 7 TestLocalPathCVE_* scenarios | VERIFIED | All 7 pass |
| `pkg/internal/doctor/check.go` | newLocalPathCVECheck() registered | VERIFIED | Line 82 |

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `create.go` | `installlocalpath` package | import + `NewAction()` in wave1 | VERIFIED | Import line 49, wave1 entry line 288 with `installlocalpath.NewAction()` |
| `create.go` | `installstorage` package | `!opts.Config.Addons.LocalPath` gate | VERIFIED | Lines 225-227 — installstorage only runs when LocalPath=false |
| `images.go` | `installlocalpath` package | `installlocalpath.Images` in RequiredAddonImages | VERIFIED | Import line 23, usage lines 70-72 |
| `localpath.go` | `check.go` | registered as `newLocalPathCVECheck()` | VERIFIED | check.go line 82 |
| `localpath.go` | `pkg/internal/version` | `version.ParseSemantic` for CVE threshold | VERIFIED | Used in Run() to parse both tag and threshold `v0.0.34` |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` compiles | `go build ./...` | Exit 0, no output | PASS |
| TestAddonsDefaults (LocalPath default=true, explicit false) | `go test ./pkg/internal/apis/config/encoding/... -run TestAddonsDefaults` | PASS | PASS |
| installlocalpath tests (TestExecute + TestImages) | `go test ./pkg/cluster/internal/create/actions/installlocalpath/...` | PASS (5 subtests) | PASS |
| Doctor CVE tests (7 scenarios) | `go test ./pkg/internal/doctor/... -run TestLocalPathCVE` | PASS (7 subtests) | PASS |
| RequiredAddonImages LocalPath coverage | `go test ./pkg/cluster/internal/providers/common/... -run TestRequiredAddonImages` | PASS (5 tests including LocalPathOnly) | PASS |

## Requirements Coverage

| Requirement | Plan | Description | Status | Evidence |
|-------------|------|-------------|--------|----------|
| STOR-01 | 44-01 | local-path-provisioner v0.0.35 as default addon | SATISFIED | Manifest pinned to v0.0.35; wave1 entry; LocalPath defaults true |
| STOR-02 | 44-01 | PVC auto-binding | HUMAN NEEDED | WaitForFirstConsumer SC + provisioner action wired; runtime binding unverifiable statically |
| STOR-03 | 44-01 | `localPath: false` opt-out | SATISFIED | installstorage gate + wave1 conditional; TestAddonsDefaults asserts explicit false |
| STOR-04 | 44-01 | No StorageClass collision | SATISFIED | `local-path` named SC; `standard` absent from manifest; installstorage mutually exclusive |
| STOR-05 | 44-03 | doctor CVE check | SATISFIED | localPathCVECheck registered; 7 test scenarios pass including vulnerable/ok paths |
| STOR-06 | 44-01/02 | Air-gapped busybox pinning | SATISFIED | busybox:1.37.0 + IfNotPresent in both manifest locations and Images var; offlinereadiness includes entry |

## Anti-Patterns Found

No blockers or warnings found.

- `installstorage` gate correctly excludes the action (not just disables it) — no empty action stub
- `localPathManifest` is embedded at compile time (not fetched at runtime) — correct for air-gapped use
- `Images` var uses `docker.io/` prefix — consistent with rest of codebase
- `realGetProvisionerVersion` returns `("", nil)` gracefully for no-runtime/no-cluster — no panics on missing deps

## Human Verification Required

### 1. PVC Auto-Binding in Single-Node Cluster (STOR-02)

**Test:** `kinder create cluster` with default config (LocalPath=true). Then: `kubectl apply -f pvc-test.yaml` where the PVC has no `storageClassName`. Observe `kubectl get pvc -w`.

**Expected:** PVC transitions from `Pending` to `Bound` within a few seconds. A matching PersistentVolume appears with `STORAGECLASS=local-path`. A directory is created under `/opt/local-path-provisioner` on the node.

**Why human:** Dynamic provisioner binding requires the local-path-provisioner pod to be running, watching PVC events, and creating PV + binding. The full kubelet + controller-manager + provisioner interaction cannot be simulated via unit tests.

### 2. PVC Auto-Binding in Multi-Node Cluster (STOR-02, WaitForFirstConsumer)

**Test:** `kinder create cluster --config multi-node.yaml` (2+ nodes). Create a PVC with `storageClassName: local-path`. Verify the PVC stays `Pending` until a pod consuming it is scheduled, then transitions to `Bound`.

**Expected:** PVC status is `Pending (WaitForFirstConsumer)` before pod scheduling. After pod is scheduled to a node, PVC transitions to `Bound` and pod reaches `Running`.

**Why human:** `WaitForFirstConsumer` binding mode is a scheduler integration that only triggers when a pod consuming the PVC is actually scheduled. This requires a live scheduler and cannot be verified statically.

## Gaps Summary

No gaps blocking goal achievement. All automatable success criteria are verified in the codebase. The single human-needed item (SC2: PVC auto-binding) requires a live cluster and is expected for any storage integration test.

---

_Verified: 2026-04-09T14:00:00Z_
_Verifier: Claude (gsd-verifier)_
