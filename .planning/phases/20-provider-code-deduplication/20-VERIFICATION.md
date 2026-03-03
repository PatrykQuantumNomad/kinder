---
phase: 20-provider-code-deduplication
verified: 2026-03-03T14:00:00Z
status: passed
score: 4/4 must-haves verified
re_verification: false
---

# Phase 20: Provider Code Deduplication Verification Report

**Phase Goal:** Shared docker/podman/nerdctl logic lives in one common/ package; per-provider files are deleted; all three providers compile and pass their test suites unchanged
**Verified:** 2026-03-03T14:00:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | common/node.go exists with a shared Node struct parameterized by binaryName; docker/node.go, nerdctl/node.go, and podman/node.go are deleted | VERIFIED | `pkg/cluster/internal/providers/common/node.go` line 31: `type Node struct { Name string; BinaryName string }`. All three per-provider node.go files confirmed absent (ls returns "No such file"). |
| 2 | common/provision.go exists with shared GenerateMountBindings, GeneratePortMappings, and CreateContainer; docker/provision.go and nerdctl/provision.go are deleted | VERIFIED | `pkg/cluster/internal/providers/common/provision.go` exports all three functions at lines 35, 70, 121. docker/provision.go and nerdctl/provision.go confirmed absent. podman/provision.go intentionally retained with its own local variants (different protocol casing, :0 strip behavior). |
| 3 | go.mod minimum directive is go 1.21.0 with toolchain go1.26.0 and the build passes with no new external dependencies | VERIFIED | `go.mod` lines 11-13: `go 1.21.0` and `toolchain go1.26.0`. No new entries in `require` blocks vs. prior baseline. `go build ./...` exits 0. |
| 4 | go build ./... and go test ./... both pass identically before and after extraction — no behavior change across any provider | VERIFIED | `go build ./...` exits 0 with no output. `go test ./...` shows all packages ok/cached including `providers/common`, `providers/docker`, `providers/nerdctl`, `providers/podman`. Zero failures. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/cluster/internal/providers/common/node.go` | Shared Node struct with BinaryName field; unexported nodeCmd | VERIFIED | 177 lines. Exports `Node` struct with `Name string` and `BinaryName string`. `nodeCmd` is unexported (lowercase). `Role()` references `NodeRoleLabelKey` from same package. All interface methods implemented. |
| `pkg/cluster/internal/providers/common/constants.go` | NodeRoleLabelKey exported alongside APIServerInternalPort | VERIFIED | Line 25: `const NodeRoleLabelKey = "io.x-k8s.kind.role"`. Both constants present. |
| `pkg/cluster/internal/providers/common/provision.go` | Exports GenerateMountBindings, GeneratePortMappings, CreateContainer | VERIFIED | 123 lines. All three functions exported. GeneratePortMappings uses immediate release pattern (not defer) inside the loop at lines 113-115. CreateContainer takes binaryName as first parameter. |
| `pkg/cluster/internal/providers/common/provision_test.go` | TestGeneratePortMappings covering static, random, invalid, empty, and port-release regression | VERIFIED | 163 lines. Six sub-tests covering static port, port=0 random, port=-1, invalid protocol, empty mappings, and the multi-port-release regression test. |
| `pkg/cluster/internal/providers/docker/create.go` | Docker-specific provision functions calling common helpers | VERIFIED | 333 lines. planCreation, commonArgs, runArgsForNode, runArgsForLoadBalancer, getProxyEnv, getSubnets, createContainerWithWaitUntilSystemdReachesMultiUserSystem — all calling common.GenerateMountBindings, common.GeneratePortMappings, common.CreateContainer. |
| `pkg/cluster/internal/providers/nerdctl/create.go` | Nerdctl-specific provision functions calling common helpers | VERIFIED | 303 lines. Same set of functions, all wired to common helpers, binaryName threaded through. |
| `pkg/cluster/internal/providers/docker/create_test.go` | Docker-specific Test_parseSubnetOutput | VERIFIED | Contains Test_parseSubnetOutput only; migrated port mapping tests not duplicated here. |
| `go.mod` | go 1.21.0 with toolchain go1.26.0 | VERIFIED | Lines 11-13 match exactly. No new external dependencies added. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `docker/provider.go` | `common/node.go` | `node()` returns `&common.Node{BinaryName: "docker"}` | WIRED | Line 231-234: `return &common.Node{ Name: name, BinaryName: "docker", }` |
| `nerdctl/provider.go` | `common/node.go` | `node()` returns `&common.Node{BinaryName: p.binaryName}` | WIRED | Lines 277-281: `return &common.Node{ Name: name, BinaryName: p.binaryName, }` |
| `podman/provider.go` | `common/node.go` | `node()` returns `&common.Node{BinaryName: "podman"}` | WIRED | Lines 312-316: `return &common.Node{ Name: name, BinaryName: "podman", }` |
| `common/node.go` | `common/constants.go` | `Role()` uses `NodeRoleLabelKey` | WIRED | Line 42: `fmt.Sprintf(`{{ index .Config.Labels "%s"}}`, NodeRoleLabelKey)` |
| `docker/create.go` | `common/provision.go` | `runArgsForNode` and `runArgsForLoadBalancer` call common.Generate* | WIRED | Lines 245-246, 271: `common.GenerateMountBindings(...)`, `common.GeneratePortMappings(...)` |
| `nerdctl/create.go` | `common/provision.go` | `runArgsForNode` and `runArgsForLoadBalancer` call common.Generate* | WIRED | Lines 215-216, 241: `common.GenerateMountBindings(...)`, `common.GeneratePortMappings(...)` |
| `docker/create.go` | `common/provision.go` | `planCreation` and `createContainerWith...` call `common.CreateContainer` | WIRED | Lines 78, 325: `common.CreateContainer("docker", name, args)` |
| `nerdctl/create.go` | `common/provision.go` | `planCreation` and `createContainerWith...` call `common.CreateContainer` | WIRED | Lines 78, 295: `common.CreateContainer(binaryName, name, args)` |
| `common/provision.go` | `common/getport.go` | `GeneratePortMappings` calls `PortOrGetFreePort` | WIRED | Line 99: `hostPort, releaseHostPortFn, err := PortOrGetFreePort(pm.HostPort, pm.ListenAddress)` |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| PROV-01 | 20-01 | Shared Node struct in common/ replacing per-provider node.go files | SATISFIED | common/node.go with BinaryName-parameterized Node; all three per-provider node.go files deleted |
| PROV-02 | 20-02 | Shared provision helpers in common/ replacing per-provider provision logic | SATISFIED | common/provision.go with GenerateMountBindings, GeneratePortMappings, CreateContainer; docker and nerdctl provision.go deleted; docker/create.go and nerdctl/create.go wire to common helpers |
| PROV-03 | 20-01 | go.mod minimum version update to go 1.21.0 with toolchain directive | SATISFIED | go.mod: `go 1.21.0`, `toolchain go1.26.0` |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `docker/create.go` | 61 | `// TODO: picking ports locally is less than ideal...` | Info | Pre-existing comment carried from the original docker/provision.go; not introduced by this phase, no behavior impact |
| `nerdctl/create.go` | 61 | Same TODO comment | Info | Same as above — same pre-existing comment |

Both TODO items are pre-existing architectural notes, not placeholders blocking goal achievement.

### Human Verification Required

None. All success criteria are verifiable programmatically and all checks pass.

### Gaps Summary

No gaps. All four observable truths are verified:

1. `common/node.go` exists with exported `Node{BinaryName string}` struct and unexported `nodeCmd`; all three per-provider `node.go` files are confirmed deleted.
2. `common/provision.go` exports `GenerateMountBindings`, `GeneratePortMappings`, and `CreateContainer`; `docker/provision.go` and `nerdctl/provision.go` are confirmed deleted; `podman/provision.go` is intentionally preserved.
3. `go.mod` declares `go 1.21.0` with `toolchain go1.26.0`, no new external dependencies.
4. `go build ./...` exits 0; `go test ./...` exits 0 with all four provider packages (`common`, `docker`, `nerdctl`, `podman`) showing `ok`.

The phase goal is fully achieved: shared docker/podman/nerdctl logic lives in `common/`, per-provider duplication files are deleted, and all three providers compile and pass their test suites.

---

_Verified: 2026-03-03T14:00:00Z_
_Verifier: Claude (gsd-verifier)_
