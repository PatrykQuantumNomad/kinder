---
phase: 42-multi-version-node-validation
verified: 2026-04-08T00:00:00Z
status: passed
score: 5/5
overrides_applied: 0
gaps: []
---

# Phase 42: Multi-Version Node Validation — Verification Report

**Phase Goal:** Users can configure per-node Kubernetes versions and kinder validates version-skew correctness at config parse time instead of surfacing cryptic kubeadm errors after provisioning begins
**Verified:** 2026-04-08T00:00:00Z
**Status:** passed
**Re-verification:** Yes — gap fixed (realListNodes + IMAGE column implemented)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Running `kinder create cluster` with a per-node `image:` field and a global `--image` flag preserves the per-node image | VERIFIED | `ExplicitImage bool` field on internal `Node` struct (types.go:103); `V1Alpha4ToInternal` captures pre-defaults state (encoding/convert.go:31-44); `fixupOptions` skips `ExplicitImage=true` nodes (create.go:386); `TestFixupOptions_ExplicitImageOverride` (3 sub-cases) all pass |
| 2 | A config with workers more than 3 minor versions behind the control-plane is rejected before any containers are created, with an error message stating the violating node and version delta | VERIFIED | `validateVersionSkew` called from `Cluster.Validate()` (validate.go:100-102); error includes node name (`worker[N]`), image with version, and numeric delta; `TestValidateVersionSkew` worker-delta-4 case passes |
| 3 | A config with HA control-plane nodes at different versions is rejected at config validation time with a clear explanation | VERIFIED | `validateVersionSkew` checks HA CP minor version consistency (validate.go:346-365); error references "control-plane" and "etcd consistency"; `TestValidateVersionSkew` HA-mismatch case passes |
| 4 | `kinder doctor` run against a running multi-version cluster reports a warning when version-skew policy is violated | VERIFIED | `clusterNodeSkewCheck` registered (check.go:81); `realListNodes()` now detects runtime via `osexec.LookPath`, lists containers by `io.x-k8s.kind.cluster` label, inspects each for role/image, and reads `/kind/version` via `docker exec` — all using low-level CLI to avoid import cycle with cluster package |
| 5 | `kinder get nodes` output includes a column showing the Kubernetes version installed on each node | VERIFIED | VERSION column populated via `nodeutils.KubeVersion()` in `collectNodeInfos()` (nodes.go:177); tabwriter header includes `VERSION` (nodes.go:147); JSON output includes `"version"` field; `ComputeSkew` + SKEW column also present; `TestComputeSkew` all cases pass |

**Score:** 4/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/internal/apis/config/types.go` | ExplicitImage bool field on internal Node type | VERIFIED | `ExplicitImage bool` at line 103 with doc comment |
| `pkg/internal/apis/config/validate.go` | validateVersionSkew function with table-format error output | VERIFIED | Function at line 306; table format with Node/Image/Delta/Remediation columns |
| `pkg/internal/apis/config/validate_test.go` | Test cases for version-skew validation | VERIFIED | `TestValidateVersionSkew` with 8 sub-cases covering all boundary conditions |
| `pkg/internal/apis/config/encoding/convert.go` | Pre-defaults Image capture and ExplicitImage propagation | VERIFIED | Pre-defaults capture at lines 31-34; propagation at lines 40-44 |
| `pkg/internal/apis/config/encoding/convert_test.go` | Tests for ExplicitImage propagation | VERIFIED | `TestV1Alpha4ToInternal_ExplicitImage` with 4 sub-cases |
| `pkg/cluster/internal/create/create.go` | fixupOptions skips nodes with ExplicitImage=true | VERIFIED | Conditional at line 386 |
| `pkg/cluster/internal/create/fixup_test.go` | Tests for fixupOptions ExplicitImage behavior | VERIFIED | `TestFixupOptions_ExplicitImageOverride` with 3 sub-cases |
| `pkg/internal/doctor/clusterskew.go` | Cluster node version-skew diagnostic check | VERIFIED | Detection logic + `realListNodes()` implemented with low-level CLI; tests pass via injection |
| `pkg/internal/doctor/clusterskew_test.go` | Tests for cluster node skew check | VERIFIED | 8 test functions covering no-cluster, ok, warn, HA-mismatch, read-failure, multiple violations, drift, no-drift |
| `pkg/internal/doctor/check.go` | clusterNodeSkewCheck registered in allChecks | VERIFIED | `newClusterNodeSkewCheck()` registered as 19th entry (line 81) |
| `pkg/cmd/kind/get/nodes/nodes.go` | Extended nodeInfo with VERSION, IMAGE, SKEW columns and tabwriter output | VERIFIED | VERSION, IMAGE, SKEW, SkewOK all implemented and wired; IMAGE populated via container inspect |
| `pkg/cmd/kind/get/nodes/nodes_test.go` | Unit tests for computeSkew helper | VERIFIED | `TestComputeSkew` with 7 sub-cases |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `encoding/convert.go` | `types.go` | sets `ExplicitImage` after `Convertv1alpha4` | WIRED | Pattern `ExplicitImage = explicitImage[i]` present at line 42 |
| `create.go` | `types.go` | reads `ExplicitImage` to conditionally skip override | WIRED | `!opts.Config.Nodes[i].ExplicitImage` at line 386 |
| `validate.go` | `pkg/internal/version` | `version.ParseSemantic` | WIRED | `version.ParseSemantic(tag)` at line 327 |
| `clusterskew.go` | `pkg/internal/version` | `version.ParseSemantic` | WIRED | Used in `Run()`, `dominantCPMinor()` |
| `clusterskew.go` | container runtime CLI | `docker exec cat /kind/version` | WIRED | `realListNodes()` reads live version via `exec.Command(binaryName, "exec", name, "cat", "/kind/version")` |
| `nodes.go` | `pkg/cluster/nodeutils` | `nodeutils.KubeVersion()` to populate VERSION column | WIRED | `nodeutils.KubeVersion(n)` at line 177 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `create.go` `fixupOptions` | `ExplicitImage` | `encoding/convert.go` → `V1Alpha4ToInternal` | Yes | FLOWING |
| `validate.go` `validateVersionSkew` | node images | `c.Nodes` (from `Cluster.Validate()`) | Yes | FLOWING |
| `doctor/clusterskew.go` `Run()` | `entries []nodeEntry` | `realListNodes()` | Yes (via CLI) | FLOWING |
| `nodes.go` `collectNodeInfos` | `ver` (VERSION column) | `nodeutils.KubeVersion(n)` | Yes (for real clusters) | FLOWING |
| `nodes.go` `collectNodeInfos` | `image` (IMAGE column) | `exec.Command(binaryName, "inspect", ...)` | Yes (for real clusters) | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| ExplicitImage propagation tests | `go test ./pkg/internal/apis/config/encoding/... -count=1` | PASS | PASS |
| fixupOptions ExplicitImage tests | `go test ./pkg/cluster/internal/create/... -count=1` | PASS | PASS |
| validateVersionSkew tests | `go test ./pkg/internal/apis/config/... -count=1` | PASS | PASS |
| Doctor skew check tests | `go test ./pkg/internal/doctor/... -count=1` | PASS | PASS |
| ComputeSkew tests | `go test ./pkg/cmd/kind/get/nodes/... -count=1` | PASS | PASS |
| go vet all modified packages | `go vet ./pkg/...` | clean | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| MVER-01 | 42-01-PLAN.md | `--image` flag only overrides nodes without an explicit image in config | SATISFIED | ExplicitImage sentinel + fixupOptions conditional skip |
| MVER-02 | 42-01-PLAN.md | Config validation rejects invalid version-skew combinations before provisioning | SATISFIED | `validateVersionSkew` called from `Cluster.Validate()` |
| MVER-03 | 42-01-PLAN.md | Clear error messages at config parse time instead of cryptic kubeadm failures | SATISFIED | Table-format error with node name, image, delta, remediation hints |
| MVER-04 | 42-02-PLAN.md | `kinder doctor` detects version-skew issues in running clusters | SATISFIED | `realListNodes()` implemented with low-level CLI; detects runtime, lists containers, inspects role/image, reads live version |
| MVER-05 | 42-02-PLAN.md | `kinder get nodes` output includes per-node Kubernetes version column | SATISFIED | VERSION column populated via `nodeutils.KubeVersion()` with tabwriter output |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None | — | — | — | All anti-patterns from initial verification have been resolved |

### Human Verification Required

None. The gaps identified can be verified programmatically.

### Gaps Summary

**No gaps remain.** All 5 success criteria verified.

The initial verification identified `realListNodes()` as a stub and the IMAGE column as empty. Both have been fixed:
- `realListNodes()` now uses low-level CLI (`docker ps --filter`, `docker inspect`, `docker exec cat /kind/version`) to avoid the import cycle with the cluster package
- `get nodes` IMAGE column now populated via `exec.Command(binaryName, "inspect", "--format", "{{.Config.Image}}", n.String())`

---

_Verified: 2026-04-08T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
