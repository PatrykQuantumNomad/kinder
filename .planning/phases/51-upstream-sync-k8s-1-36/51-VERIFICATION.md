---
phase: 51-upstream-sync-k8s-1-36
verified: 2026-05-07T14:00:00Z
status: human_needed
score: 3/4 must-haves verified
overrides_applied: 0
deferred:
  - truth: "Running `kinder create cluster` without an explicit `image:` field provisions a K8s 1.36.x node (latest stable patch at ship time, >=1.36.4)"
    addressed_in: "Phase 51 Plan 04 re-run (same milestone, pending upstream)"
    evidence: "51-04-SUMMARY.md: Docker Hub probe returned HTTP 200 count=0 for kindest/node:v1.36.x; SC2 explicitly deferred per plan gating protocol. Plan 04 Task 2 is fully authored and ready to execute atomically once kind v0.32.0 publishes the v1.36 image. No source-level rework required."
human_verification:
  - test: "Create a multi-control-plane cluster and confirm the LB container is envoyproxy/envoy, not kindest/haproxy"
    expected: "`docker ps --format '{{.Image}}' | grep -E 'envoy|haproxy'` returns only envoyproxy/envoy lines; no kindest/haproxy container running"
    why_human: "Requires a live Docker daemon to spin up the cluster; not verifiable with static grep alone"
  - test: "Confirm an invalid cluster config with kubeProxyMode:ipvs + a 1.36 node image is rejected before any container is created"
    expected: "kinder create cluster --config ipvs-1-36.yaml exits non-zero with message containing 'ipvs is not supported with Kubernetes 1.36' and migration URL"
    why_human: "Requires a live kinder binary and the validate path being exercised end-to-end; Go unit tests cover the validation logic but not the CLI integration"
---

# Phase 51: Upstream Sync & K8s 1.36 — Verification Report

**Phase Goal:** Kinder adopts kind's HAProxy-to-Envoy LB transition, ships K8s 1.36 as the default node image, and protects users from the silent IPVS removal breakage introduced in 1.36
**Verified:** 2026-05-07T14:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | HA clusters use Envoy as LB container; `kindest/haproxy` is no longer pulled | VERIFIED | `const Image = "docker.io/envoyproxy/envoy:v1.36.2"` in `const.go`; `grep -rn "kindest/haproxy" pkg/` returns zero production matches; `GenerateBootstrapCommand` wired in all three providers (docker/podman/nerdctl); `offlinereadiness.go` lists `envoyproxy/envoy:v1.36.2` not haproxy; `go build ./...` exits 0 |
| 2 | `kinder create cluster` without explicit `image:` provisions a K8s 1.36.x node | DEFERRED | `pkg/apis/config/defaults/image.go` still reads `kindest/node:v1.35.1@sha256:...`; no v1.36.x image exists on Docker Hub as of 2026-05-07 (Docker Hub API returned count=0); SC2 deferred pending kind v0.32.0 — see Deferred Items section |
| 3 | A cluster config with `kubeProxyMode: ipvs` is rejected at validation with a clear error message when node version is 1.36+ | VERIFIED | Guard implemented at `validate.go` lines 80–100; error message confirmed: "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+ … deprecated in v1.35 and will be removed in a future release. Switch to iptables or nftables. Migration guide: https://kubernetes.io/docs/reference/networking/virtual-ips/"; `TestIPVSDeprecationGuard` with 7 table-driven cases passes with `-race`; `go test ./pkg/internal/apis/config/... -race` exits 0 |
| 4 | kinder website has a "What's new in K8s 1.36" recipe page with working examples demonstrating User Namespaces (GA) and In-Place Pod Resize (GA) | VERIFIED | `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` exists (168 lines); contains `hostUsers: false` (3 occurrences), `resizePolicy` (2 occurrences), both GA feature demo sections with apply+verify steps; registered in sidebar at `astro.config.mjs` line 83 between `guides/multi-version-clusters` and `guides/working-offline`; human spot-check required for rendered output quality |

**Score:** 3/4 truths verified (SC2 deferred — external blocker, not a code failure)

### Deferred Items

Items not yet met due to external upstream dependency, not code-level failures.

| # | Item | Addressed In | Evidence |
|---|------|-------------|----------|
| 1 | SC2: default node image is K8s 1.36.x | Phase 51 Plan 04 re-run | Plan 04 Task 2 (TDD RED→GREEN for `image.go`) is fully authored and ready. Re-run `gsd-execute-phase 51 04` once Docker Hub returns Outcome A for `?name=v1.36`. No source-level rework required — only the image constant and digest need updating. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/cluster/internal/loadbalancer/const.go` | Envoy image constant + 4 proxy path constants | VERIFIED | `Image = "docker.io/envoyproxy/envoy:v1.36.2"`, `ProxyConfigPath`, `ProxyConfigPathCDS`, `ProxyConfigPathLDS`, `ProxyConfigDir` all present |
| `pkg/cluster/internal/loadbalancer/config.go` | Envoy xDS templates, `Config(data, template)`, `GenerateBootstrapCommand`, `hostPort` FuncMap | VERIFIED | All 6 symbols present and substantive; full template content verified |
| `pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go` | Dual Config() calls for LDS+CDS, atomic mv-swap, no SIGHUP | VERIFIED | Lines 84–112 confirmed: dual Config() calls, tmpLDS/tmpCDS writes, `chmod&&mv&&mv` swap, comment "no SIGHUP needed" |
| `pkg/cluster/internal/providers/docker/create.go` | `loadbalancer.Image` + `GenerateBootstrapCommand` in runArgsForLoadBalancer | VERIFIED | Lines 284–285 append image then bootstrap args |
| `pkg/cluster/internal/providers/podman/provision.go` | `loadbalancer.Image` (sanitized) + `GenerateBootstrapCommand` | VERIFIED | Lines 256, 258 confirmed |
| `pkg/cluster/internal/providers/nerdctl/create.go` | `loadbalancer.Image` + `GenerateBootstrapCommand` | VERIFIED | Lines 254–255 confirmed |
| `pkg/internal/doctor/offlinereadiness.go` | Envoy image in allAddonImages, not haproxy | VERIFIED | Line 51: `"docker.io/envoyproxy/envoy:v1.36.2"` under "Load Balancer (HA)" |
| `pkg/internal/apis/config/validate.go` | IPVS-on-1.36+ guard in `Cluster.Validate()` | VERIFIED | Lines 77–100 confirmed; guard triggers on `v.Minor() >= 36`; error message contains deprecation text and migration URL |
| `pkg/internal/apis/config/validate_test.go` | `TestIPVSDeprecationGuard` with 7 subtests | VERIFIED | Function confirmed at line 549; test cases exercise v1.36.0 reject, mixed-node reject, v1.35.1 pass, iptables-with-1.36 pass, non-semver skip |
| `pkg/apis/config/defaults/image.go` | `kindest/node:v1.36.x` default | DEFERRED | Still reads `kindest/node:v1.35.1@sha256:...`; deferred per SC2 external blocker |
| `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` | Guide page with User Namespaces + In-Place Pod Resize GA demos | VERIFIED | 168 lines; both feature sections present with apply/verify steps; `hostUsers: false` x3, `resizePolicy` x2 |
| `kinder-site/astro.config.mjs` | Sidebar entry `guides/k8s-1-36-whats-new` | VERIFIED | Line 83 confirmed; placement after `multi-version-clusters`, before `working-offline` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `docker/create.go:runArgsForLoadBalancer` | `loadbalancer.Image` | direct reference, line 284 | WIRED | Image appended to args before bootstrap |
| `docker/create.go:runArgsForLoadBalancer` | `loadbalancer.GenerateBootstrapCommand` | direct call, line 285 | WIRED | Bootstrap args appended after image |
| `podman/provision.go:runArgsForLoadBalancer` | `loadbalancer.Image` (via sanitizeImage) | line 256 | WIRED | Sanitized image used |
| `podman/provision.go:runArgsForLoadBalancer` | `loadbalancer.GenerateBootstrapCommand` | line 258 | WIRED | Bootstrap args appended |
| `nerdctl/create.go:runArgsForLoadBalancer` | `loadbalancer.Image` | line 254 | WIRED | Image appended to args |
| `nerdctl/create.go:runArgsForLoadBalancer` | `loadbalancer.GenerateBootstrapCommand` | line 255 | WIRED | Bootstrap args appended |
| `loadbalancer/loadbalancer.go` | `loadbalancer.Config(data, ProxyLDSConfigTemplate)` | line 84 | WIRED | LDS config generated |
| `loadbalancer/loadbalancer.go` | `loadbalancer.Config(data, ProxyCDSConfigTemplate)` | line 88 | WIRED | CDS config generated |
| `loadbalancer/loadbalancer.go` | atomic mv-swap (`chmod&&mv&&mv`) | lines 106–110 | WIRED | No SIGHUP path; xDS polling picks up swapped files |
| `validate.go:Cluster.Validate()` | IPVS guard → migration URL error | lines 80–100 | WIRED | Guard fires before validateVersionSkew; break-on-first-match |
| `offlinereadiness.go:allAddonImages` | `envoyproxy/envoy:v1.36.2` | line 51 | WIRED | Doctor check covers the new LB image |
| `astro.config.mjs` sidebar | `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` | `{ slug: 'guides/k8s-1-36-whats-new' }` at line 83 | WIRED | Guide reachable from sidebar |

### Data-Flow Trace (Level 4)

Not applicable — this phase delivers Go validation logic, constants, command-generation functions, and a static documentation page. No dynamic data rendering paths exist.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go packages build cleanly | `go build ./...` | exit 0 | PASS |
| Loadbalancer + config validation tests pass with race detector | `go test ./pkg/cluster/internal/loadbalancer/... ./pkg/internal/apis/config/... -race -count=1` | `ok` for all 3 packages | PASS |
| No haproxy references in production Go code | `grep -rn "kindest/haproxy" pkg/ | grep -v _test.go` | no output | PASS |
| GenerateBootstrapCommand wired in all 3 providers | `grep -rn "GenerateBootstrapCommand" pkg/cluster/internal/providers/` | 3 matches (docker/podman/nerdctl) | PASS |
| IPVS guard fires on v.Minor() >= 36 | code inspection + `TestIPVSDeprecationGuard` | 7/7 subtests pass with `-race` | PASS |
| Website guide file exists with both GA feature demos | `wc -l` + content inspection | 168 lines; `hostUsers: false` x3, `resizePolicy` x2 | PASS |
| Sidebar registration in astro.config.mjs | `grep -n "k8s-1-36-whats-new" kinder-site/astro.config.mjs` | line 83 confirmed | PASS |
| Default image still v1.35.1 (SC2 deferred) | `cat pkg/apis/config/defaults/image.go` | `kindest/node:v1.35.1@sha256:...` | EXPECTED (deferred) |

### Requirements Coverage

| Success Criterion | Plan | Status | Evidence |
|------------------|------|--------|----------|
| SC1 — Envoy LB, no haproxy pulled | 51-01 | SATISFIED | `const.go` Image constant, all 3 provider bootstrap wirings, zero haproxy production references, offlinereadiness updated |
| SC2 — Default node image K8s 1.36.x | 51-04 | DEFERRED | External blocker: no `kindest/node:v1.36.x` on Docker Hub; plan halted cleanly at gating probe; `image.go` unchanged at v1.35.1 |
| SC3 — ipvs+1.36 rejected at validation with clear error + migration URL | 51-02 | SATISFIED | `validate.go` guard confirmed; error message verified; 7-case test suite passes `-race` |
| SC4 — Website "What's new in K8s 1.36" with User Namespaces + In-Place Pod Resize demos | 51-03 | SATISFIED (human spot-check pending) | Guide page exists with both GA demo workflows; sidebar registered; human verification of rendered page quality requested |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` | 13 | `kinder v0.X or later` placeholder version | Info | Documentation only; version not yet determined pending SC2 (no v1.36 default image yet) |

No code-level stubs found. No `return null`, empty handlers, or TODO/FIXME markers in any modified production Go files.

### Human Verification Required

#### 1. Live Envoy LB container verification

**Test:** Create a two-control-plane cluster with kinder (requires a local Docker daemon). Observe which container image runs as the load balancer.

```bash
kinder create cluster --name ha-test --config - <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: control-plane
  - role: worker
EOF
docker ps --format '{{.Image}}\t{{.Names}}' | grep ha-test
```

**Expected:** The load-balancer container shows `docker.io/envoyproxy/envoy:v1.36.2`. No `kindest/haproxy` container appears anywhere in the output.

**Why human:** Requires a live Docker daemon to instantiate containers. Static grep confirms the image constant and bootstrap wiring, but the end-to-end pull + run behaviour cannot be confirmed programmatically.

#### 2. IPVS guard CLI integration test

**Test:** Write a cluster config with `kubeProxyMode: ipvs` and a 1.36 node image and attempt to create a cluster.

```bash
cat > /tmp/ipvs-1-36-test.yaml <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  kubeProxyMode: ipvs
nodes:
  - role: control-plane
    image: kindest/node:v1.36.0
EOF
kinder create cluster --config /tmp/ipvs-1-36-test.yaml; echo "exit: $?"
```

**Expected:** Command exits non-zero before pulling any image, with an error message containing "ipvs is not supported with Kubernetes 1.36" and "https://kubernetes.io/docs/reference/networking/virtual-ips/".

**Why human:** Go unit tests validate the `Cluster.Validate()` function in isolation. The full CLI path (config parse → validate → early reject → no container creation) needs a live kinder binary.

### Gaps Summary

No blocking code-level gaps. The only unmet success criterion (SC2) is an external-dependency deferral, not a kinder defect:

- **SC2 deferral is clean:** Plan 04 Task 1 ran the Docker Hub gating probe and received HTTP 200 with `count: 0` for `?name=v1.36`. This is the authoritative signal that `kindest/node:v1.36.x` does not yet exist. The plan correctly halted at Outcome B without committing a broken default. Task 2 (TDD RED→GREEN for `image.go`) is fully authored and ready to execute without any rework.
- **Re-run path:** Once kind v0.32.0 publishes the v1.36 image, re-run `gsd-execute-phase 51 04`. Task 1 will return Outcome A; Task 2 will update `pkg/apis/config/defaults/image.go` and the website guide reference. SC2 closes immediately.
- **No source-level debt:** The codebase is in a clean state. The v1.35.1 default is not wrong — it is the current latest stable image. The bump to v1.36.x is blocked only on upstream image publication.

Human verification items (live cluster creation tests) are the remaining gate before the phase can be marked fully passed.

---

_Verified: 2026-05-07T14:00:00Z_
_Verifier: Claude (gsd-verifier)_
