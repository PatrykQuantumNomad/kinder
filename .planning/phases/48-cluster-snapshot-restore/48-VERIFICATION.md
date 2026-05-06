---
phase: 48-cluster-snapshot-restore
verified: 2026-05-06T00:00:00Z
status: passed
score: 4/4
overrides_applied: 0
---

# Phase 48: Cluster Snapshot/Restore — Verification Report

**Phase Goal:** Users can capture a complete cluster state as a named snapshot and restore it in seconds, enabling instant reset between development cycles
**Verified:** 2026-05-06
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `kinder snapshot create [snap-name]` captures etcd state, container images, and PV contents | VERIFIED | `CaptureEtcd` (etcd.go:62), `CaptureImages` (images.go:72), `CapturePVs` (pvs.go:58) all fully implemented with real shell-out logic. Orchestrated in `create.go:138`. Integration test `TestIntegrationSnapshotConfigMapRoundTrip` confirms archive + sidecar produced and PV round-trip works on a live cluster. |
| 2 | `kinder snapshot restore [snap-name]` returns cluster to captured state and refuses with clear error on K8s version mismatch | VERIFIED | `CheckCompatibility` (compat.go:50) hard-fails on K8s/topology/addon mismatches before any mutation. Error messages include "k8s version mismatch" (ErrCompatK8sMismatch), "topology mismatch", and "addon version mismatch". Integration tests `TestIntegrationRestoreRefusesOnK8sMismatch`, `TestIntegrationRestoreRefusesOnTopologyMismatch`, `TestIntegrationRestoreRefusesOnAddonMismatch` all exercised against a real cluster. |
| 3 | `kinder snapshot list`, `kinder snapshot show [snap-name]`, `kinder snapshot prune` all implemented | VERIFIED | All five subcommands registered in `snapshot.go:40-46`. `list.go:127` prints NAME/AGE/SIZE/K8S/ADDONS/STATUS columns. `show.go:125-143` prints size, age, K8s version, node image, topology, addons, and digests (including ImagesDigest at show.go:141). `prune.go:99-105` refuses with error when no policy flag given; prompts y/N unless `--yes`. Live UAT confirmed all three. |
| 4 | Snapshot metadata records K8s version, addon versions, and image-bundle digest | VERIFIED | `Metadata` struct (metadata.go:53-67) has `K8sVersion`, `NodeImage`, `AddonVersions map[string]string`, `EtcdDigest`, `ImagesDigest`, `PVsDigest`, `ArchiveDigest` fields. Integration test step 4 (snapshot_integration_test.go:306-344) explicitly asserts `K8sVersion != ""`, `NodeImage != ""`, `AddonVersions["localPath"] != ""`, `len(EtcdDigest) == 64`, and `ImagesDigest != ""`. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/internal/snapshot/metadata.go` | Metadata schema with LIFE-08 fields | VERIFIED | Struct with K8sVersion, AddonVersions, ImagesDigest, EtcdDigest, ArchiveDigest fields; MarshalMetadata/UnmarshalMetadata implemented |
| `pkg/internal/snapshot/bundle.go` | Single-pass tar.gz writer with sha256 sidecar | VERIFIED | WriteBundle: streaming sha256 via io.MultiWriter; sidecar written as sha256sum-format file; VerifyBundle re-hashes and detects ErrCorruptArchive |
| `pkg/internal/snapshot/store.go` | SnapshotStore over ~/.kinder/snapshots/ (mode 0700) | VERIFIED | NewStore creates dirs with 0700; DefaultRoot returns ~/.kinder/snapshots; List/Open/Delete all implemented |
| `pkg/internal/snapshot/prune.go` | Pure prune-policy filters (KeepLast/OlderThan/MaxSize) | VERIFIED | KeepLast, OlderThan, MaxSize filter functions; PrunePlan composes as UNION |
| `pkg/internal/snapshot/etcd.go` | CaptureEtcd via crictl exec etcdctl | VERIFIED | Discovers etcd container via crictl ps, runs etcdctl snapshot save, streams file with sha256 tee |
| `pkg/internal/snapshot/images.go` | CaptureImages via ctr images export | VERIFIED | Lists refs via ctr images list -q, exports via ctr images export, streams with sha256 tee |
| `pkg/internal/snapshot/pvs.go` | CapturePVs per-node tar of /opt/local-path-provisioner | VERIFIED | Probes each node for LocalPathDir, builds outer tar with per-node nested entries |
| `pkg/internal/snapshot/etcdrestore.go` | HA-safe etcd restore with shared cluster token | VERIFIED | manifest-aside + atomic data-dir swap; parallel per-CP restore; fresh cluster token generated per restore |
| `pkg/internal/snapshot/imagesrestore.go` | Image re-import via LoadImageArchiveWithFallback | VERIFIED | Wraps nodeutils.LoadImageArchiveWithFallback; empty-file gate |
| `pkg/internal/snapshot/pvsrestore.go` | Per-node PV untar | VERIFIED | Reads outer tar, looks up node by name, pipes inner tar into tar -xf - -C /opt on node |
| `pkg/internal/snapshot/compat.go` | CheckCompatibility with K8s/topology/addon hard-fails | VERIFIED | Three sentinel errors; CheckCompatibility returns aggregate of all violations |
| `pkg/internal/snapshot/create.go` | Create orchestrator (etcd-while-running + pause + images+PVs + bundle + defer-resume) | VERIFIED | 13-step orchestration matching plan order; defer-Resume guarantee on all exit paths |
| `pkg/internal/snapshot/restore.go` | Restore orchestrator (full pre-flight before mutation; no auto-rollback) | VERIFIED | 7 pre-flight checks before any mutation; wrapMutationError adds recovery hint; no deferred Resume |
| `pkg/cmd/kind/snapshot/snapshot.go` | Command group with 5 subcommands | VERIFIED | NewCommand registers create/restore/list/show/prune |
| `pkg/cmd/kind/snapshot/create.go` | CLI create command | VERIFIED | Positional args, --json, --name flags; calls snapshot.Create |
| `pkg/cmd/kind/snapshot/restore.go` | CLI restore command | VERIFIED | Positional args, --json flag; calls snapshot.Restore |
| `pkg/cmd/kind/snapshot/list.go` | CLI list with NAME/AGE/SIZE/K8S/ADDONS/STATUS columns | VERIFIED | tabwriter table with all required columns; --output=json/yaml; STATUS=corrupt detection from VerifyBundle |
| `pkg/cmd/kind/snapshot/show.go` | CLI show with size/age/k8s/image digest | VERIFIED | printVertical displays Name, Cluster, Created, Size, K8s Version, Node Image, Topology, Addons, Digests (including ImagesDigest) |
| `pkg/cmd/kind/snapshot/prune.go` | CLI prune with no-flag refusal and y/N prompt | VERIFIED | Refuses if all three flags zero; prints deletion plan; prompts y/N unless --yes |
| `pkg/internal/snapshot/snapshot_integration_test.go` | ConfigMap+PV round-trip integration test | VERIFIED | TestIntegrationSnapshotConfigMapRoundTrip: real cluster, sentinel data seeded, snapshot, mutate, restore, assert |
| `pkg/internal/snapshot/restore_refusal_integration_test.go` | Refusal integration tests | VERIFIED | Four tests: K8s mismatch, topology mismatch, addon mismatch, STATUS=corrupt |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `pkg/cmd/kind/root.go` | `snapshot.NewCommand` | `cmd.AddCommand` at line 94 | WIRED | Import at line 37, registration at line 94 |
| `create.go (CLI)` | `snapshot.Create` | `createFn = snapshot.Create` | WIRED | Production fn var set at pkg/cmd/kind/snapshot/create.go:44; called at line 92 |
| `restore.go (CLI)` | `snapshot.Restore` | `restoreFn = snapshot.Restore` | WIRED | Production fn var set at pkg/cmd/kind/snapshot/restore.go:42; called at line 88 |
| `list.go (CLI)` | `snapshot.NewStore + List` | `listFn` closure | WIRED | listFn creates store and calls List; called at runList:103 |
| `show.go (CLI)` | `snapshot.NewStore + Open` | `showFn` closure | WIRED | showFn creates store, opens bundle, reads metadata; called at runShow:102 |
| `prune.go (CLI)` | `snapshot.NewStore + PrunePlan + Delete` | `pruneStoreFn + PrunePlan` | WIRED | pruneStoreFn wraps store.List+Delete; PrunePlan called at line 142 |
| `create.go (pkg)` | `lifecycle.Pause / lifecycle.Resume` | `pauseFn / resumeFn` wired to lifecycle package | WIRED | create.go:174,180 default to lifecycle.Pause/Resume |
| `restore.go (pkg)` | `CheckCompatibility` | called at PF7 (line 317) | WIRED | Invoked with snapMeta and liveMeta before any mutation |
| `restore.go (pkg)` | `RestoreEtcd / RestoreImages / RestorePVs` | called M3/M4/M5 (lines 369-386) | WIRED | All three restore primitives called in mutation phase |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `list.go` printTable | `infos []snapshot.Info` | store.List → VerifyBundle + OpenBundle | Yes — reads actual .tar.gz from disk, verifies sha256 | FLOWING |
| `show.go` printVertical | `m *Metadata` | store.Open → OpenBundle → UnmarshalMetadata | Yes — reads actual metadata.json from archive | FLOWING |
| `create.go` CreateResult | `meta *Metadata` | CaptureTopology + CaptureAddonVersions + CaptureEtcd (all live cluster calls) | Yes — all fields populated from real cluster nodes | FLOWING |
| `restore.go` RestoreResult | `snapMeta *Metadata` | bundleReader.Metadata() from archive | Yes — reads live archive on disk | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Unit test suite | `go test -count=1 ./pkg/internal/snapshot/...` | PASS (0.398s) | PASS |
| Build check | `go build ./pkg/internal/snapshot/... ./pkg/cmd/kind/snapshot/...` | No output (clean) | PASS |
| Static analysis | `go vet ./pkg/internal/snapshot/... ./pkg/cmd/kind/snapshot/...` | No output (clean) | PASS |
| Integration tests (live UAT, Plan 06) | `make integration` on real Docker | All 4 tests green — user approved 2026-05-06 | PASS |
| Manual smoke (live UAT) | Full create/list/show/restore/prune sequence on real cluster | All acceptance criteria green — user approved 2026-05-06 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| LIFE-05 | 48-01/02/04/05 | User can snapshot via `kinder snapshot create`; captures etcd, images, PVs | SATISFIED | CaptureEtcd/CaptureImages/CapturePVs + Create orchestrator + CLI create command |
| LIFE-06 | 48-03/04/05 | User can restore via `kinder snapshot restore`; refuses on K8s version mismatch | SATISFIED | Restore orchestrator + CheckCompatibility + integration refusal tests all green |
| LIFE-07 | 48-05 | User can list, inspect, and prune via `kinder snapshot list/show/prune` | SATISFIED | All three CLI commands implemented with correct columns/fields/safety guards |
| LIFE-08 | 48-01/02/04/05 | Snapshot metadata records K8s version, addon versions, image-bundle digest | SATISFIED | Metadata struct + CaptureAddonVersions + ImagesDigest + integration test step 4 asserts all fields |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `pkg/internal/snapshot/etcd.go` | 24 | `TODO: refactor to a shared internal constant if the cert paths ever move` | Info | Documents cert-path duplication with doctor package; informational only — not a stub, code is fully functional |

No blockers found. No placeholder implementations, no empty returns masking missing logic, no disconnected wiring.

### Human Verification Required

All automated checks passed. Live UAT was performed by the user on 2026-05-06 with full stack confirmed:

1. `make integration` — all 4 integration tests passed (ConfigMap+PV round-trip, K8s/topology/addon refusals, STATUS=corrupt detection).
2. Manual smoke sequence on `uat48` cluster with `--local-path`: create, list, show, restore (ConfigMap value "before" correctly returned), prune no-flag refusal, prune --keep-last 1 y/N prompt.

No additional human verification items remain.

### Gaps Summary

No gaps. All 4 success criteria are satisfied by substantive, fully-wired implementations verified at code level and confirmed by live UAT.

---

_Verified: 2026-05-06_
_Verifier: Claude (gsd-verifier)_
