---
phase: 02-metallb
verified: 2026-03-01T00:00:00Z
status: human_needed
score: 9/9 must-haves verified
human_verification:
  - test: "Run kinder create cluster and observe MetalLB pods starting"
    expected: "MetalLB controller and speaker pods reach Running state before kinder create cluster returns"
    why_human: "Requires an actual Docker-backed cluster creation end-to-end — cannot verify pod lifecycle programmatically"
  - test: "Apply a Service of type LoadBalancer after cluster creation"
    expected: "EXTERNAL-IP is assigned within seconds (not stuck in Pending)"
    why_human: "Requires a live cluster with MetalLB running and functional L2 advertisement"
  - test: "Run kinder create cluster with addons.metalLB: false in cluster config"
    expected: "No metallb-system pods exist in the cluster after creation"
    why_human: "Requires cluster creation; the runAddon gating logic is verified in code but the actual skip behavior needs a real cluster to confirm no pods appear"
---

# Phase 2: MetalLB Verification Report

**Phase Goal:** Services of type LoadBalancer receive an EXTERNAL-IP automatically on every supported container provider
**Verified:** 2026-03-01
**Status:** human_needed (all automated checks pass; 3 runtime behaviors require a live cluster)
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

All truths from both plans are verified against the actual codebase.

#### Plan 02-01 Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Given a Docker network inspect JSON blob with an IPv4 CIDR, the parser returns the correct subnet string | VERIFIED | `TestParseSubnetFromJSON/Docker_JSON_with_IPv4_and_IPv6_IPAM_returns_IPv4_CIDR` passes; `parseDockerSubnet` in `subnet.go:71` unmarshals `IPAM.Config[].Subnet` and filters `ip.To4() != nil` |
| 2 | Given a Podman network inspect JSON blob with its different schema, the parser returns the correct subnet string | VERIFIED | `TestParseSubnetFromJSON/Podman_JSON_with_subnets_array_returns_IPv4_CIDR` passes; `parsePodmanSubnet` in `subnet.go:95` uses separate `podmanNetworkInspect` struct with `subnets[].subnet` field |
| 3 | Given a /16 subnet, carvePoolFromSubnet returns a .255.200-.255.250 range | VERIFIED | `TestCarvePoolFromSubnet//16_network_returns_last_.255.200-.255.250` passes — `172.18.0.0/16` → `172.18.255.200-172.18.255.250`; broadcast arithmetic in `subnet.go:138-150` confirmed |
| 4 | Given a /24 subnet, carvePoolFromSubnet returns a .200-.250 range | VERIFIED | `TestCarvePoolFromSubnet//24_network_returns_.200-.250_in_that_subnet` and minikube-style variant pass — `10.89.0.0/24` → `10.89.0.200-10.89.0.250` |
| 5 | IPv6-only IPAM entries are skipped and only IPv4 is returned | VERIFIED | `TestParseSubnetFromJSON/Docker_JSON_with_only_IPv6_IPAM_returns_error` returns error `"no IPv4 subnet found"`; `ip.To4() != nil` guard in `subnet.go:87` skips IPv6 entries |

#### Plan 02-02 Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 6 | MetalLB controller and speaker pods are installed when the action Execute runs | VERIFIED (code) | `metallb.go:97-100` applies the embedded `metalLBManifest` (2288-line v0.15.3 manifest with controller Deployment and speaker DaemonSet); HUMAN NEEDED for runtime confirmation |
| 7 | The action waits for the controller deployment to be Available before applying CRs | VERIFIED | `metallb.go:103-108` runs `kubectl wait --namespace=metallb-system --for=condition=Available deployment/controller --timeout=120s` before calling `detectSubnet` |
| 8 | IPAddressPool and L2Advertisement CRs are applied with the auto-detected IP range | VERIFIED | `metallb.go:111-127` calls `detectSubnet` → `carvePoolFromSubnet` → formats `crTemplate` with pool range → applies via `kubectl apply -f -`; `crTemplate` uses `metallb.io/v1beta1` |
| 9 | Rootless Podman receives a warning that L2 ARP will not work | VERIFIED | `metallb.go:81-87` calls `ctx.Provider.Info()` and emits `ctx.Logger.Warn("MetalLB L2 speaker cannot send ARP in rootless mode...")` when `info.Rootless == true` |

**Score:** 9/9 truths verified (5 confirmed by passing unit tests; 4 confirmed by static analysis; runtime behavior requires human)

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/cluster/internal/create/actions/installmetallb/subnet.go` | Subnet detection and IP pool carving functions | VERIFIED | 173 lines; exports `detectSubnet`, `parseSubnetFromJSON`, `carvePoolFromSubnet`; uses `sigs.k8s.io/kind/pkg/errors` and `sigs.k8s.io/kind/pkg/exec` |
| `pkg/cluster/internal/create/actions/installmetallb/subnet_test.go` | Unit tests for subnet parsing and pool carving | VERIFIED | 203 lines; `TestParseSubnetFromJSON` (8 cases) and `TestCarvePoolFromSubnet` (7 cases); all 15 tests pass |
| `pkg/cluster/internal/create/actions/installmetallb/metallb.go` | Complete MetalLB action implementation replacing the stub | VERIFIED | 131 lines (min_lines: 60 — passes); exports `NewAction`; no TODO/stub/placeholder patterns found |
| `pkg/cluster/internal/create/actions/installmetallb/manifests/metallb-native.yaml` | Embedded MetalLB v0.15.3 native manifest | VERIFIED | 2288 lines; contains 29 references to `metallb-system`; images pinned to `quay.io/metallb/controller:v0.15.3` and `quay.io/metallb/speaker:v0.15.3` |

---

## Key Link Verification

### Plan 02-01 Key Links

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `subnet.go` | `sigs.k8s.io/kind/pkg/exec` | `exec.Command(` call | WIRED | `subnet.go:163`: `exec.Output(exec.Command(providerName, "network", "inspect", networkName))` |
| `subnet.go` | `encoding/json` | `json.Unmarshal` | WIRED | `subnet.go:73` and `subnet.go:97`: `json.Unmarshal(output, &networks)` in both Docker and Podman parsers |

### Plan 02-02 Key Links

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `metallb.go` | `subnet.go` | calls `detectSubnet` and `carvePoolFromSubnet` | WIRED | `metallb.go:111`: `detectSubnet(providerName)`; `metallb.go:117`: `carvePoolFromSubnet(subnet)` — same package, direct calls |
| `metallb.go` | `metallb-native.yaml` | `go:embed` directive | WIRED | `metallb.go:31-32`: `//go:embed manifests/metallb-native.yaml` + `var metalLBManifest string`; `_ "embed"` imported at line 22 |
| `metallb.go` | `kubectl apply` | `node.Command kubectl apply -f - with stdin` | WIRED | `metallb.go:97-100`: `node.Command("kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "apply", "-f", "-").SetStdin(strings.NewReader(metalLBManifest)).Run()` |
| `metallb.go` | `kubectl wait` | `node.Command kubectl wait for controller Available` | WIRED | `metallb.go:103-108`: `node.Command("kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "wait", "--namespace=metallb-system", "--for=condition=Available", "deployment/controller", "--timeout=120s").Run()` |

---

## Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| MLB-01 | 02-02 | MetalLB controller and speaker pods installed and running by default | SATISFIED (code) / HUMAN for runtime | `metallb.go` applies v0.15.3 manifest with controller Deployment + speaker DaemonSet; `runAddon("MetalLB", opts.Config.Addons.MetalLB, ...)` in `create.go:199` runs by default |
| MLB-02 | 02-01 | IP address pool auto-detected from Docker network subnet without user input | SATISFIED | `detectSubnet(providerName)` in `subnet.go:157` runs `docker/podman/nerdctl network inspect kind`; no user input required |
| MLB-03 | 02-02 | IPAddressPool and L2Advertisement CRs applied after MetalLB webhook is ready | SATISFIED | `kubectl wait --for=condition=Available deployment/controller` at `metallb.go:103-108` executes before CR apply at `metallb.go:124-127` |
| MLB-04 | 02-02 | Services of type LoadBalancer receive EXTERNAL-IP within seconds | HUMAN NEEDED | Code path correct (pool applied, L2Advertisement created), but actual IP assignment requires live cluster test |
| MLB-05 | 02-01 | Subnet detection works with Docker provider | SATISFIED | `parseSubnetFromJSON` with `providerName="docker"` routes to `parseDockerSubnet`; tested in `TestParseSubnetFromJSON` |
| MLB-06 | 02-01 | Subnet detection works with Podman provider | SATISFIED | `parseSubnetFromJSON` with `providerName="podman"` routes to `parsePodmanSubnet`; tested in `TestParseSubnetFromJSON` |
| MLB-07 | 02-01 | Subnet detection works with Nerdctl provider | SATISFIED | `nerdctl` falls through to `parseDockerSubnet` (same JSON schema); tested in `TestParseSubnetFromJSON/Nerdctl_JSON_with_IPv4_and_IPv6_IPAM` |
| MLB-08 | 02-02 | User can disable MetalLB via `addons.metalLB: false` | SATISFIED (code) / HUMAN for runtime | `runAddon` at `create.go:179-191` skips action and logs `"Skipping MetalLB (disabled in config)"` when `enabled=false`; `opts.Config.Addons.MetalLB` is the gate |

---

## Anti-Patterns Found

No anti-patterns detected. Scan results:

| File | Pattern | Result |
|------|---------|--------|
| `metallb.go` | TODO/FIXME/placeholder | None found |
| `metallb.go` | `return null / return {}` | None found |
| `subnet.go` | TODO/FIXME/placeholder | None found |
| `subnet.go` | `return null / return {}` | None found |

---

## Build and Test Results

| Check | Result |
|-------|--------|
| `go build ./...` | PASS — entire project compiles |
| `go vet ./pkg/cluster/internal/create/actions/installmetallb/...` | PASS — no issues |
| `go test ./pkg/cluster/internal/create/actions/installmetallb/... -v -count=1` | PASS — 15/15 tests |
| Commit `9f5fb379` (RED — failing tests) | EXISTS |
| Commit `999466b4` (GREEN — implementation) | EXISTS |
| Commit `b3d5e970` (manifest embed) | EXISTS |
| Commit `59236c76` (Execute function) | EXISTS |

---

## Human Verification Required

All automated checks pass. The following behaviors require a live Docker-backed cluster to confirm:

### 1. MetalLB Pods Reach Running State During Cluster Creation

**Test:** Run `kinder create cluster` on a Docker-backed host. After the command returns, run `kubectl get pods -n metallb-system`.
**Expected:** `controller-*` pod shows `1/1 Running`, `speaker-*` pod on each node shows `1/1 Running`. Both reach this state before `kinder create cluster` exits (the `kubectl wait` ensures this).
**Why human:** Pod lifecycle and readiness depends on Docker, image pull, and Kubernetes scheduler — not verifiable statically.

### 2. LoadBalancer Service Receives EXTERNAL-IP

**Test:** After cluster creation, apply a Service with `type: LoadBalancer`. Run `kubectl get svc` repeatedly for 30 seconds.
**Expected:** The `EXTERNAL-IP` column shows an IP in the range `<subnet>.200`–`<subnet>.250` (e.g., `172.18.255.200`) within seconds. The IP is NOT stuck in `<pending>`.
**Why human:** L2 ARP advertisement and IP assignment by the MetalLB speaker requires a live network stack. Cannot verify programmatically.

### 3. MetalLB Disabled via Config Produces No Pods

**Test:** Create a cluster config with `addons.metalLB: false`. Run `kinder create cluster --config <file>`. After creation, run `kubectl get ns metallb-system`.
**Expected:** The namespace does not exist, or exists with no pods.
**Why human:** The `runAddon` gating is verified in code at `create.go:179-191`, but the actual absence of pods in a real cluster requires cluster creation to confirm no side-effects install MetalLB anyway.

---

## Summary

Phase 2 (MetalLB) has achieved its goal at the code level. The implementation is complete, substantive, and fully wired:

- `subnet.go` (173 lines) implements all three subnet functions — `detectSubnet`, `parseSubnetFromJSON`, `carvePoolFromSubnet` — with correct Docker/Podman/Nerdctl branching and IPv6 filtering.
- `subnet_test.go` (203 lines) provides 15 passing unit tests covering all provider schemas and edge cases.
- `metallb.go` (131 lines) replaces the stub with a complete 10-step Execute function: status tracking, node selection, rootless warning, provider detection, manifest apply, webhook wait, subnet detection, pool carving, CR application.
- `manifests/metallb-native.yaml` (2288 lines) contains the official MetalLB v0.15.3 manifest embedded at build time.
- `create.go:199` wires `installmetallb.NewAction()` into the addon pipeline gated by `opts.Config.Addons.MetalLB`.

All 8 MLB requirements are satisfied in code. Three runtime behaviors (pod readiness, EXTERNAL-IP assignment, disable-via-config) require human verification with a live cluster.

---

_Verified: 2026-03-01_
_Verifier: Claude (gsd-verifier)_
