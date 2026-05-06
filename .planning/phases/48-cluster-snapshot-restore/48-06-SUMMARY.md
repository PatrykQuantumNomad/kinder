---
phase: 48-cluster-snapshot-restore
plan: "06"
subsystem: snapshot-integration-tests
tags: [integration-tests, snapshot, restore, kind, go-test]
dependency_graph:
  requires: ["48-05"]
  provides: ["integration-test-gate", "phase-48-human-verify-checkpoint"]
  affects: ["pkg/internal/snapshot"]
tech_stack:
  added: []
  patterns:
    - "//go:build integration tag with integration.MaybeSkip(t) gate"
    - "go run ./cmd/kind as test binary invocation (always exercises current branch source)"
    - "docker exec <container> sh -c for CP tampering (K8s version mismatch scenario)"
    - "docker rm -f <worker> for topology mismatch scenario"
    - "kubectl set image for addon version mismatch scenario"
    - "flipByteAtOffset (XOR 0xFF at offset 512) for archive corruption scenario"
key_files:
  created:
    - pkg/internal/snapshot/snapshot_integration_test.go
    - pkg/internal/snapshot/restore_refusal_integration_test.go
  modified: []
decisions:
  - "go run ./cmd/kind (not a pre-built binary) used to invoke kinder in tests — always exercises current branch source, no binary path assumptions"
  - "kubeconfig resolution tries ~/.kube/kind-<cluster>, ~/.kube/<cluster>.kubeconfig, ~/.kube/config then falls back to kinder get kubeconfig output"
  - "Integration test cluster names derived from t.Name() sanitised to lowercase alphanum+hyphen, prefixed 'kit-' to avoid collision with UAT clusters"
  - "PVC-backed sentinel pod exercises CapturePVs path; pod Must be Ready before snapshot to ensure PV data exists"
  - "Topology mismatch uses docker rm -f (not docker stop) to ensure ListNodes returns only CP — stopped containers still appear in docker ps -a but removed ones do not"
  - "Addon mismatch bumps deployment spec image (not pod image) — CaptureAddonVersions reads kubectl get deployment jsonpath which reflects spec immediately without rollout completing"
  - "ArchiveDigest inside tarred metadata.json intentionally empty (chicken-and-egg); test logs this as expected rather than failing"
  - "assertKinderRestoreFails checks both wantInStderr and noneOfInStderr lists — addon test specifically excludes 'k8s version mismatch' and 'topology mismatch' so the error is isolated to the addon dimension"
metrics:
  duration: "~48 minutes (writing + verification + live UAT)"
  completed: "2026-05-06"
  tasks_completed: 3
  tasks_total: 3
  files_created: 2
  files_modified: 0
---

# Phase 48 Plan 06: Integration Tests + Human-Verification Checkpoint Summary

**One-liner:** Five real-cluster integration tests covering ConfigMap round-trip (LIFE-08), K8s/topology/addon refusals (CONTEXT.md hard-fails), and STATUS=corrupt detection — gated behind `//go:build integration` and `make integration`.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | ConfigMap round-trip + LIFE-08 metadata integration test | 70c7667e | pkg/internal/snapshot/snapshot_integration_test.go |
| 2 | Restore-refusal cases + STATUS=corrupt integration tests | 70c7667e | pkg/internal/snapshot/restore_refusal_integration_test.go |

| 3 | Human verification — `make integration` + manual smoke on host | user-approved | (live UAT 2026-05-06) |

## What Was Built

### Task 1 — `snapshot_integration_test.go`

`TestIntegrationSnapshotConfigMapRoundTrip`:
1. Creates a real kinder cluster with `--local-path`.
2. Seeds a ConfigMap (with timestamped sentinel value) and a PVC-backed Pod with sentinel file `sentinel-original`.
3. Runs `kinder snapshot create <cluster> golden` — asserts exit 0, archive + sidecar exist on disk.
4. Opens the bundle via `OpenBundle` and asserts all LIFE-08 metadata fields:
   - `K8sVersion != ""`
   - `NodeImage != ""`
   - `Topology.ControlPlaneCount >= 1`
   - `len(AddonVersions) >= 1`
   - `AddonVersions["localPath"] != ""`
   - `len(EtcdDigest) == 64` (sha256 hex)
   - `ImagesDigest != ""`
5. Mutates: deletes the ConfigMap, overwrites PV sentinel to `sentinel-mutated`.
6. Runs `kinder snapshot restore <cluster> golden` — asserts exit 0.
7. Asserts: ConfigMap value matches original sentinel; PV file contains `sentinel-original`.

### Task 2 — `restore_refusal_integration_test.go`

`TestIntegrationRestoreRefusesOnK8sMismatch`:
- Creates cluster → takes snapshot → `docker exec <CP> sh -c 'echo v9.99.99 > /kind/version'`
- Asserts `kinder snapshot restore` exits non-zero with stderr containing "k8s version mismatch"

`TestIntegrationRestoreRefusesOnTopologyMismatch`:
- Creates 2-node cluster (1 CP + 1 worker via kind config file) → takes snapshot → `docker rm -f <worker>`
- Asserts `kinder snapshot restore` exits non-zero with stderr containing "topology mismatch"

`TestIntegrationRestoreRefusesOnAddonMismatch`:
- Creates cluster with `--local-path` → takes snapshot → `kubectl set image deployment/local-path-provisioner ...=rancher/local-path-provisioner:v0.0.99`
- Asserts restore exits non-zero with stderr containing "addon" but NOT "k8s version mismatch" or "topology mismatch"

`TestIntegrationListShowsCorrupt`:
- Creates cluster → takes snapshot → `flipByteAtOffset(archive, 512)` (XOR 0xFF, sidecar left intact)
- Asserts `kinder snapshot list` table shows STATUS="corrupt"
- Also asserts `kinder snapshot list --output json` contains corrupt in JSON

## Verification Results

| Check | Result |
|-------|--------|
| `go vet -tags integration ./pkg/internal/snapshot/...` | PASS (no output) |
| `go build -tags integration ./...` | PASS (no output) |
| `go test ./pkg/internal/snapshot/...` (no integration tag) | PASS (0.432s, regular unit suite untouched) |
| `make integration` on real Docker (Task 3 live UAT) | PASS — user approved 2026-05-06 (all 4 integration tests green) |
| Manual smoke: `kinder create cluster --name uat48 --local-path` full sequence | PASS — ConfigMap value "before" restored; snapshot list STATUS=ok; prune no-flag refusal; prune --keep-last 1 y/N prompt |

## Human Verification (Task 3 — Live UAT)

Approved by user on 2026-05-06 after running the full acceptance sequence:

1. `make integration` — all 4 integration tests passed (ConfigMap round-trip, K8s/topology/addon refusals, STATUS=corrupt).
2. Manual smoke on a fresh `uat48` cluster with `--local-path`:
   - `kinder snapshot create uat48` — snapshot created.
   - `kinder snapshot list uat48` — NAME/AGE/SIZE/K8S/ADDONS/STATUS columns visible, STATUS=ok.
   - `kinder snapshot show uat48 <snap>` — metadata fields displayed.
   - `kubectl delete configmap uat-cm` → `kinder snapshot restore uat48 <snap>` → `kubectl get configmap uat-cm -o jsonpath='{.data.key}'` returned "before".
   - `kinder snapshot prune uat48` (no flags) — exited non-zero with error listing all 3 policy flags.
   - `kinder snapshot prune uat48 --keep-last 1` — showed y/N confirmation prompt.
3. All acceptance criteria from the plan passed.

Phase 48 full-stack delivery confirmed: source (Plans 01–05) + integration tests (Plan 06) + live UAT all green.

## Deviations from Plan

None — plan executed exactly as written for Tasks 1 and 2. The integration test files mirror all five scenarios from the plan's must_haves.

## Known Stubs

None. Tests exercise real code paths; no hardcoded empty values or placeholder text.

## Threat Flags

None — integration test files only; no new network endpoints, auth paths, file access patterns, or schema changes.

## Self-Check

**Commit 70c7667e exists:** verified via `git rev-parse --short HEAD` immediately after commit.

**Files created:**
- `/Users/patrykattc/work/git/kinder/pkg/internal/snapshot/snapshot_integration_test.go` — confirmed
- `/Users/patrykattc/work/git/kinder/pkg/internal/snapshot/restore_refusal_integration_test.go` — confirmed

**No accidental file deletions** in commit 70c7667e (git diff --diff-filter=D was empty).

## Self-Check: PASSED
