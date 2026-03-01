---
phase: "02-metallb"
plan: "02"
subsystem: "installmetallb"
tags: ["metallb", "go:embed", "kubectl", "loadbalancer", "ipaddresspool", "l2advertisement"]
dependency_graph:
  requires: ["02-01 (subnet.go with detectSubnet, carvePoolFromSubnet)"]
  provides: ["complete MetalLB action with embedded manifest and CR application"]
  affects: ["pkg/cluster/internal/create/actions/installmetallb"]
tech_stack:
  added:
    - "go:embed directive for static YAML manifest embedding"
    - "MetalLB v0.15.3 native manifest (metallb-native.yaml)"
  patterns:
    - "kubectl apply via stdin using SetStdin(strings.NewReader(manifest))"
    - "fmt.Stringer type assertion for provider name detection (interface without String())"
    - "Rootless Podman detection via ProviderInfo.Rootless for L2 speaker warning"
    - "Sequential kubectl apply + kubectl wait + kubectl apply CR pattern"
key_files:
  created:
    - "pkg/cluster/internal/create/actions/installmetallb/manifests/metallb-native.yaml"
  modified:
    - "pkg/cluster/internal/create/actions/installmetallb/metallb.go"
decisions:
  - id: "02-02-A"
    summary: "fmt.Stringer type assertion for provider name — Provider interface lacks String(), but concrete Docker/Podman/Nerdctl providers implement it; type assertion with 'docker' fallback avoids interface pollution"
  - id: "02-02-B"
    summary: "MetalLB manifest embedded at build time via go:embed — no network required at cluster creation time; manifest pinned to v0.15.3 for reproducible installs"
  - id: "02-02-C"
    summary: "Webhook wait targets deployment/controller Available condition (120s) — ensures ValidatingWebhookConfiguration is fully ready before applying CRs to avoid webhook not ready errors"
metrics:
  duration: "1 minute"
  completed: "2026-03-01"
  tasks_completed: 2
  files_created: 1
  files_modified: 1
---

# Phase 2 Plan 2: MetalLB Action Implementation Summary

**One-liner:** MetalLB v0.15.3 embedded manifest applied via kubectl stdin with webhook readiness wait, followed by auto-configured IPAddressPool and L2Advertisement CRs carved from the detected container network subnet.

## What Was Built

### Task 1: MetalLB v0.15.3 Manifest Download and Embedding

**`manifests/metallb-native.yaml`** — Official MetalLB v0.15.3 native manifest (2288 lines):
- Namespace: `metallb-system`
- CRDs for IPAddressPool, L2Advertisement, BGPPeer, BGPAdvertisement, BFDProfile, Community, ConfigurationState
- Deployment: `controller` in metallb-system
- DaemonSet: `speaker` in metallb-system
- ValidatingWebhookConfiguration for admission control
- 29 references to `metallb-system`

### Task 2: MetalLB Action Execute Implementation

**`metallb.go`** — Full Execute function replacing the stub:

1. `ctx.Status.Start("Installing MetalLB")` with `defer ctx.Status.End(false)` for proper status tracking

2. **Control plane node selection** via `nodeutils.ControlPlaneNodes(allNodes)` — uses the first sorted control plane node

3. **Rootless Podman detection** via `ctx.Provider.Info()` — emits `ctx.Logger.Warn` warning when `info.Rootless == true` (L2 speaker cannot send ARP in rootless mode)

4. **Provider name detection** via `fmt.Stringer` type assertion — `ctx.Provider.(fmt.Stringer)` with `"docker"` fallback since Provider interface lacks `String()`

5. **MetalLB manifest apply** via `kubectl apply -f -` reading from `strings.NewReader(metalLBManifest)` where `metalLBManifest` is the go:embed variable

6. **Webhook readiness wait** via `kubectl wait --for=condition=Available deployment/controller --timeout=120s` in metallb-system namespace

7. **Subnet detection** via `detectSubnet(providerName)` from subnet.go (queries container network inspect)

8. **IP pool carving** via `carvePoolFromSubnet(subnet)` from subnet.go (.200-.250 in broadcast octet block)

9. **CR application** via `kubectl apply -f -` with formatted `crTemplate` (IPAddressPool + L2Advertisement using metallb.io/v1beta1)

10. `ctx.Status.End(true)` on success, `return nil`

All error wrapping uses `errors.Wrap(err, "message")` from `sigs.k8s.io/kind/pkg/errors`.

## Verification Results

```
go build ./pkg/cluster/internal/create/actions/installmetallb/... -> OK
go vet ./pkg/cluster/internal/create/actions/installmetallb/...   -> OK
go build ./...                                                      -> OK
go test ./pkg/cluster/internal/create/actions/installmetallb/... -v -count=1 -> PASS (15/15)
```

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | `b3d5e970` | chore(02-02): add embedded MetalLB v0.15.3 native manifest |
| 2 | `59236c76` | feat(02-02): implement MetalLB action Execute function |

## Decisions Made

**02-02-A: fmt.Stringer type assertion for provider name**

The `providers.Provider` interface does not define `String()`, but the concrete Docker, Podman, and Nerdctl implementations all implement `fmt.Stringer`. A type assertion `ctx.Provider.(fmt.Stringer)` extracts the provider name without adding `String()` to the interface. The fallback is `"docker"` — the most common provider — so the worst case is a failed `docker network inspect` rather than a panic.

**02-02-B: go:embed for manifest at build time**

The MetalLB v0.15.3 manifest is embedded at compile time. This means cluster creation works offline, the binary is self-contained, and the manifest version is pinned in source control. The `//go:embed manifests/metallb-native.yaml` directive and blank `_ "embed"` import handle this transparently.

**02-02-C: Webhook wait before CR application**

MetalLB's ValidatingWebhookConfiguration intercepts IPAddressPool and L2Advertisement creation. If the webhook endpoint (controller pod) is not ready, CR application fails with a webhook connection error. Waiting for `deployment/controller` Available condition (120s timeout) ensures the webhook is accepting before CRs are applied.

## Deviations from Plan

None - plan executed exactly as written.

## Phase 2 Completion

Phase 2 (MetalLB) is now complete:
- **02-01**: subnet.go with detectSubnet, parseSubnetFromJSON, carvePoolFromSubnet (15 unit tests)
- **02-02**: Full Execute function with embedded manifest, webhook wait, and CR application

The MetalLB action chain is complete end-to-end. When `kinder create cluster` runs with MetalLB enabled (default), it will:
1. Apply the embedded v0.15.3 manifest to the cluster
2. Wait for the controller webhook to become available
3. Auto-detect the container network subnet
4. Carve a .200-.250 IP pool from the subnet
5. Apply IPAddressPool and L2Advertisement CRs

**Next Phase Readiness:** Phase 2 is complete. Phase 3 (Metrics Server) and Phase 4 (CoreDNS Tuning) depend only on Phase 1 and can now proceed. Phase 5 (Envoy Gateway) requires Phase 2 complete — that condition is now satisfied.

## Self-Check

Files created/modified:
- `pkg/cluster/internal/create/actions/installmetallb/manifests/metallb-native.yaml` - FOUND
- `pkg/cluster/internal/create/actions/installmetallb/metallb.go` - FOUND (stub replaced)

Commits:
- `b3d5e970` (chore: manifest) - FOUND
- `59236c76` (feat: Execute function) - FOUND

Build: PASS
Vet: PASS
Tests: 15/15 PASS

## Self-Check: PASSED
