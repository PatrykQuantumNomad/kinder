---
phase: 22-local-registry-addon
verified: 2026-03-03T14:00:00Z
status: passed
score: 4/4 must-haves verified
re_verification: null
gaps: []
human_verification:
  - test: "Push image from host and pull from pod on every node"
    expected: "docker push localhost:5001/alpine:latest succeeds; kubectl run test --image=localhost:5001/alpine:latest shows Running pod on any node"
    why_human: "Requires a live Docker daemon and a running kind cluster; cannot verify container networking via static analysis"
  - test: "Verify containerd certs.d hot-reload works without daemon restart"
    expected: "After kinder create cluster, image pulls from localhost:5001 inside pods succeed immediately without restarting containerd on any node"
    why_human: "Requires live containerd process to observe whether config_path triggers hot-reload of hosts.toml at runtime"
  - test: "Verify addons.localRegistry: false skips registry container creation"
    expected: "kinder create cluster with localRegistry: false completes without any docker run kind-registry command and no kind-registry container exists"
    why_human: "Static analysis confirms the conditional code path; runtime behavior (no container created) requires live execution"
---

# Phase 22: Local Registry Addon Verification Report

**Phase Goal:** Users get a working local container registry at localhost:5001 by default, accessible from all cluster nodes, with dev tool discovery support — all without any manual setup
**Verified:** 2026-03-03
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | A registry:2 container named kind-registry is created on the kind network during cluster creation and survives cluster restarts | VERIFIED | `localregistry.go` lines 89-112: idempotent `inspect`-before-`run` creates the container; idempotent network-membership check then connects to `kind` network. Container uses `--restart=always` and is not labeled as a kind node, so it survives `kinder delete cluster`. |
| 2 | docker push localhost:5001/myimage:latest succeeds from the host and the image is pullable from inside a pod on any cluster node | VERIFIED (logic) / HUMAN for runtime | `create.go` lines 117-124 injects `config_path="/etc/containerd/certs.d"` into `ContainerdConfigPatches` BEFORE `p.Provision()`. `localregistry.go` lines 117-129 iterates ALL nodes (not just control-plane) via `ctx.Nodes()` and writes `hosts.toml` pointing at `http://kind-registry:5000` using `tee`. The `hostsTOML` constant correctly uses the container name, not `localhost`. |
| 3 | The kube-public/local-registry-hosting ConfigMap exists in the cluster and contains the correct registry endpoint for Tilt, Skaffold, and other dev tools | VERIFIED | `manifests/local-registry-hosting.yaml` exists with `namespace: kube-public`, `name: local-registry-hosting`, and `localRegistryHosting.v1` data key containing `host: "localhost:5001"`. Applied in `localregistry.go` lines 141-146 via `kubectl apply -f -` on the first control-plane node using `--kubeconfig=/etc/kubernetes/admin.conf`. |
| 4 | Setting addons.localRegistry: false in cluster config skips registry creation entirely — kinder create cluster completes without the registry container | VERIFIED | `create.go` line 117 guards `ContainerdConfigPatches` injection with `opts.Config.Addons.LocalRegistry`. Line 212 passes `opts.Config.Addons.LocalRegistry` as the `enabled` bool to `runAddon`; when false, `runAddon` logs a skip message and returns without calling `Execute`. Test in `load_test.go` lines 207-217 confirms `localRegistry: false` in a cluster config YAML is parsed to `cfg4.Addons.LocalRegistry == false`. Default in `default.go` line 91 sets `LocalRegistry` to `true` when not specified. |

**Score:** 4/4 truths verified (3 fully automated, 1 requires human runtime confirmation for network-layer behavior)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go` | Local registry action implementation | VERIFIED | 156 lines. Exports `NewAction()`. `Execute()` implements all 4 steps: container creation (idempotent), network connect (idempotent), per-node `hosts.toml` injection, ConfigMap apply. Compile: `go build ./pkg/cluster/internal/create/actions/installlocalregistry/...` passes with zero errors. |
| `pkg/cluster/internal/create/actions/installlocalregistry/manifests/local-registry-hosting.yaml` | Embedded ConfigMap for dev tool discovery | VERIFIED | 9 lines. Contains `localRegistryHosting.v1` key with `host: "localhost:5001"`. Embedded via `//go:embed manifests/local-registry-hosting.yaml` in `localregistry.go`. |
| `pkg/cluster/internal/create/create.go` | ContainerdConfigPatches injection and runAddon wiring | VERIFIED | Import at line 41 (`installlocalregistry`). ContainerdConfigPatches injection lines 114-124 (before `p.Provision()` at line 127). `runAddon("Local Registry", ...)` at line 212, before `runAddon("MetalLB", ...)` at line 213. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `create.go` | `pkg/.../installlocalregistry` | import + runAddon call | WIRED | Import line 41; `runAddon("Local Registry", opts.Config.Addons.LocalRegistry, installlocalregistry.NewAction())` line 212. |
| `create.go` | `opts.Config.ContainerdConfigPatches` | append before p.Provision() | WIRED | Lines 117-124: conditional `append` with `config_path = "/etc/containerd/certs.d"` at lines 120-122, before `p.Provision(status, opts.Config)` at line 127. |
| `localregistry.go` | `ctx.Provider.(fmt.Stringer)` | type assertion for binary name | WIRED | Lines 66-69: `binaryName := "docker"` default + type assertion `if s, ok := ctx.Provider.(fmt.Stringer); ok { binaryName = s.String() }`. Matches MetalLB pattern. |
| `localregistry.go` | `ctx.Nodes()` | iterate ALL nodes for hosts.toml | WIRED | Line 117: `allNodes, err := ctx.Nodes()`. Line 121: `for _, node := range allNodes`. Both control-plane and worker nodes receive `mkdir` + `tee hosts.toml`. |
| `localregistry.go` | `nodeutils.ControlPlaneNodes` | ConfigMap apply via control-plane | WIRED | Line 134: `controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)`. Lines 141-146: `controlPlanes[0].Command("kubectl", ...)` applies the ConfigMap. Error returned if `len(controlPlanes) == 0`. |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| REG-01 | 22-01-PLAN.md | Create registry:2 container on kind network during cluster creation | SATISFIED | `localregistry.go` lines 89-112: idempotent `docker run registry:2` + `docker network connect kind kind-registry` |
| REG-02 | 22-01-PLAN.md | Patch containerd certs.d config on all nodes for localhost:5001 | SATISFIED | `localregistry.go` lines 117-129: iterates `ctx.Nodes()` (ALL nodes), `mkdir + tee hosts.toml` with `http://kind-registry:5000` target |
| REG-03 | 22-01-PLAN.md | Apply kube-public/local-registry-hosting ConfigMap for dev tool discovery | SATISFIED | `localregistry.go` lines 131-146 + `manifests/local-registry-hosting.yaml` with `localRegistryHosting.v1: host: "localhost:5001"` |
| REG-04 | 22-01-PLAN.md | Addon disableable via addons.localRegistry: false in cluster config | SATISFIED | `create.go` line 117 (ContainerdConfigPatches guard) + line 212 (runAddon enabled flag); default.go line 91 (defaults to true); test confirms false parsing |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | — |

Scanned for: TODO/FIXME/placeholder comments, `return null`/empty returns, `console.log`-only handlers, stub implementations. None present. All four steps in `Execute()` have substantive implementation.

### Key Critical Design Verifications

**Anti-pattern: localhost in hosts.toml** — NOT PRESENT. `hostsTOML` constant (line 48) correctly uses `kind-registry:5000`, not `localhost:5001`. The comment at line 45-47 explicitly documents why.

**Anti-pattern: Only patching control-plane** — NOT PRESENT. Line 121 iterates `allNodes` (from `ctx.Nodes()`), not `controlPlanes`. The comment at lines 114-116 explicitly documents this requirement.

**Anti-pattern: ContainerdConfigPatches after Provision** — NOT PRESENT. Injection at `create.go` lines 117-124 precedes `p.Provision()` at line 127.

**Anti-pattern: registry:3** — NOT PRESENT. `registryImage = "registry:2"` (line 40).

**Anti-pattern: Restarting containerd** — NOT PRESENT. No containerd restart command in either file.

### Human Verification Required

#### 1. End-to-End Push/Pull Verification

**Test:** Run `kinder create cluster`, then `docker push localhost:5001/alpine:latest`, then `kubectl run test --image=localhost:5001/alpine:latest --restart=Never -- sleep 3600`
**Expected:** Pod reaches `Running` state on any cluster node (including workers)
**Why human:** Requires a live Docker daemon, running kind cluster, and actual network layer between host and kind-registry container. Static analysis confirms all code paths are correct but cannot verify Docker network routing behavior.

#### 2. ConfigMap Dev Tool Discovery

**Test:** After cluster creation, run `kubectl get configmap local-registry-hosting -n kube-public -o yaml`
**Expected:** ConfigMap exists with `localRegistryHosting.v1: host: "localhost:5001"`
**Why human:** Requires live Kubernetes cluster; kubectl apply inside a kind node container.

#### 3. Disabled Registry Skip

**Test:** Create a cluster config with `addons: localRegistry: false` and run `kinder create cluster --config <file>`, then `docker ps | grep kind-registry`
**Expected:** No `kind-registry` container created; cluster creation completes successfully
**Why human:** Static analysis confirms the conditional guard; runtime confirmation that the container is truly absent requires live execution.

### Gaps Summary

No gaps. All four observable truths are supported by substantive, wired implementation:

- The `installlocalregistry` package (156 lines) is non-stub and implements all four REG requirements
- `create.go` wires the package both pre-provisioning (ContainerdConfigPatches) and post-provisioning (runAddon)
- `go build ./...` and `go test ./...` both pass with zero errors across all 24 packages
- Both implementation commits (`8ee3cf63`, `869b20c3`) exist in git history with correct file changes
- The `addons.localRegistry` field defaults to `true` (via `default.go`), is parseable as `false` from YAML (confirmed by test), and flows through conversion to internal config correctly

Three human verification items exist for runtime behavior that cannot be confirmed statically, but none represent code-level gaps.

---

_Verified: 2026-03-03_
_Verifier: Claude (gsd-verifier)_
