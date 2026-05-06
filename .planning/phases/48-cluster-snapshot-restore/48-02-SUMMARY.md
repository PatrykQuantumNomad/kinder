---
phase: 48-cluster-snapshot-restore
plan: 02
subsystem: snapshot-capture
tags: [snapshot, etcd, images, pvs, topology, kindconfig, tdd]
dependency_graph:
  requires: [48-01]
  provides: [CaptureEtcd, CaptureImages, CapturePVs, CaptureTopology, CaptureAddonVersions, ReconstructKindConfig, ClassifyFn, AddonRegistry, ListImageRefs, LocalPathDir]
  affects: [48-04, 48-05]
tech_stack:
  added: []
  patterns: [FakeNode/FakeCmd injection, ClassifyFn dependency injection, tee-SHA256 streaming, nested-tar per-node PV layout]
key_files:
  created:
    - pkg/internal/snapshot/etcd.go
    - pkg/internal/snapshot/etcd_test.go
    - pkg/internal/snapshot/images.go
    - pkg/internal/snapshot/images_test.go
    - pkg/internal/snapshot/kindconfig.go
    - pkg/internal/snapshot/kindconfig_test.go
    - pkg/internal/snapshot/pvs.go
    - pkg/internal/snapshot/pvs_test.go
    - pkg/internal/snapshot/topology.go
    - pkg/internal/snapshot/topology_test.go
    - pkg/internal/snapshot/capture_test_helpers_test.go
  modified: []
decisions:
  - "etcdctlAuthArgs duplicated inline in etcd.go (not imported from doctor) to avoid import cycle; TODO comment references original location"
  - "ClassifyFn defined in snapshot package (not lifecycle) so Plan 04 can inject lifecycle.ClassifyNodes without circular import"
  - "CapturePVs uses nested tar layout: <nodeName>/local-path-provisioner.tar per node (RESEARCH Q8 resolution)"
  - "ReconstructKindConfig does not use v1alpha4 API types — pure string builder to keep snapshot pkg free of cluster API imports"
  - "AddonRegistry omits localRegistry — it is a host-level Docker container, not a k8s Deployment; topology covers it via nodeImage"
  - "TestRestoreEtcdSingleCP_Sequence failure pre-exists Plan 02 (Plan 03 parallel wave issue, out of scope)"
metrics:
  duration: "7m 41s"
  completed: "2026-05-06"
  tasks_completed: 2
  tasks_total: 2
  files_created: 11
  files_modified: 0
---

# Phase 48 Plan 02: Capture Sources Summary

One-liner: TDD-implemented etcd/images/PVs/topology/kindconfig capture functions streaming to host temp files via FakeNode/FakeCmd injection, with ClassifyFn injection pattern to avoid circular lifecycle import.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | captureEtcd + captureImages + captureKindConfig (TDD) | 94354c97 | etcd.go, images.go, kindconfig.go + tests + capture_test_helpers_test.go |
| 2 | capturePVs + topology + addon versions (TDD) | ae83e17a | pvs.go, topology.go + tests |

## Capture API Surface

### etcd.go
- `CaptureEtcd(ctx, cp nodes.Node, dstPath string) (digest string, err error)`
  - Discovers etcd container via `crictl ps --name etcd -q`
  - Runs `crictl exec <id> etcdctl <authArgs> snapshot save /tmp/kinder-etcd.snap`
  - Streams back via `cat` teed through sha256
  - `etcdctlAuthArgs` duplicated inline (no doctor import — avoids cycle)

### images.go
- `ListImageRefs(ctx, cp nodes.Node) ([]string, error)` — `ctr --namespace=k8s.io images list -q`
- `CaptureImages(ctx, cp nodes.Node, dstPath string) (digest string, err error)`
  - Exports via `ctr --namespace=k8s.io images export /tmp/kinder-images.tar <refs...>`
  - Streams back via `cat` teed through sha256
  - Empty cluster: no export call; digest = sha256("")

### pvs.go
- `LocalPathDir = "/opt/local-path-provisioner"` (matches embedded manifest constant)
- `CapturePVs(ctx, allNodes []nodes.Node, dstPath string) (digest string, err error)`
  - Probes each node via `test -d LocalPathDir`
  - Streams `tar -cf - -C /opt local-path-provisioner` per node into outer tar
  - Outer tar layout: `<nodeName>/local-path-provisioner.tar` per node
  - Returns ("", nil) if no node has the dir (Plan 04 writes PVsDigest: "")

### topology.go
- `ClassifyFn func(allNodes []nodes.Node) (cp, workers []nodes.Node, lb nodes.Node, err error)` — injection point for lifecycle.ClassifyNodes
- `CaptureTopology(ctx, allNodes, classify ClassifyFn, providerBin string) (TopologyInfo, k8sVersion, nodeImage string, err error)`
  - Reads `/kind/version` inside cp[0] for k8sVersion
  - Calls `<providerBin> inspect --format {{.Config.Image}} <cp[0]>` for nodeImage
- `CaptureAddonVersions(ctx, cp nodes.Node) (map[string]string, error)`
  - Queries each `AddonRegistry` entry via kubectl jsonpath
  - Returns only installed addons (NotFound exits are skipped)
  - Parses `:tag` suffix for version value

### AddonRegistry (7 entries)
| CanonicalName | Namespace | Deployment |
|---|---|---|
| localPath | local-path-storage | local-path-provisioner |
| metalLB | metallb-system | controller |
| metricsServer | kube-system | metrics-server |
| dashboard | kubernetes-dashboard | kubernetes-dashboard |
| certManager | cert-manager | cert-manager |
| envoyGateway | envoy-gateway-system | envoy-gateway |
| coredns | kube-system | coredns |

*localRegistry omitted: host-level Docker container, not a k8s Deployment.*

### kindconfig.go
- `ReconstructKindConfig(topo TopologyInfo, k8sVersion, nodeImage string, addons map[string]string) ([]byte, error)`
  - Emits minimal v1alpha4 YAML (string-builder, not API type marshal)
  - Includes prominent "NOT included" comment for ExtraMounts, ContainerdConfigPatches, FeatureGates, RuntimeConfig, KubeProxyMode
  - HA topology: no `role: load-balancer` entry (not a valid v1alpha4 role); documented via comment

## Test Coverage (16 new tests, all TDD)

### Task 1 (9 tests)
- TestCaptureEtcdSuccess, TestCaptureEtcdNoEtcdContainer, TestCaptureEtcdSnapshotFails
- TestListImageRefs, TestCaptureImagesSuccess, TestCaptureImagesEmptyCluster
- TestReconstructSingleCP, TestReconstructHA, TestReconstructHasNotice

### Task 2 (7 tests)
- TestCapturePVsNoNodes, TestCapturePVsOneNodeHasData, TestCapturePVsMultipleNodes
- TestCaptureTopologySingleCP, TestCaptureTopologyHA
- TestCaptureAddonVersionsAllPresent, TestCaptureAddonVersionsNoAddons

## Test Infrastructure

`captureCallbackNode` / `captureTestCmd` defined in `capture_test_helpers_test.go` — the snapshot-package equivalent of lifecycle's `commandCallbackNode` pattern. All tests use FakeNode injection; no real `crictl`, `ctr`, `docker`, or `kubectl` calls are made.

## Deviations from Plan

### Minor: kindconfig.go comment wording adjustment
- **Found during:** Task 1, TestReconstructHA (GREEN phase)
- **Issue:** Draft comment used "role: load-balancer" literally; test correctly checked that string not appear as a node role; comment substring collision.
- **Fix:** Changed comment to "load balancer" (no hyphen, no "role:" prefix) to avoid the literal role value in comment text.
- **Files modified:** kindconfig.go
- **Rule:** 1 (bug fix)

### Out-of-scope: Plan 03 TestRestoreEtcdSingleCP_Sequence failure
- Plan 03 is running in parallel wave 2. Its `etcdrestore_test.go` has a pre-existing test failure (`TestRestoreEtcdSingleCP_Sequence` — sequencing assertion flap). This failure existed before any Plan 02 files were created (verified by checking git status showing etcdrestore* as untracked/pre-existing). Logged to deferred-items.

## Import Constraint Verification

```
! grep '"sigs.k8s.io/kind/pkg/internal/lifecycle"' pkg/internal/snapshot/*.go  PASS
! grep '"sigs.k8s.io/kind/pkg/internal/doctor"' pkg/internal/snapshot/*.go      PASS
```

No circular imports introduced.

## Known Stubs

None. All capture functions are fully implemented against the specified behavior.

## Self-Check: PASSED

- pkg/internal/snapshot/etcd.go: FOUND
- pkg/internal/snapshot/images.go: FOUND
- pkg/internal/snapshot/kindconfig.go: FOUND
- pkg/internal/snapshot/pvs.go: FOUND
- pkg/internal/snapshot/topology.go: FOUND
- Commit 94354c97: FOUND
- Commit ae83e17a: FOUND
- go test -race (Plan 02 tests): PASS
- go vet: PASS
- go build: PASS
