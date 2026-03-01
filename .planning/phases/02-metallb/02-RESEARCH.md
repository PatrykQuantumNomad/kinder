# Phase 2: MetalLB - Research

**Researched:** 2026-03-01
**Domain:** MetalLB installation, subnet auto-detection across container providers (Docker/Podman/Nerdctl), webhook readiness, go:embed manifests
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MLB-01 | MetalLB controller and speaker pods are installed and running by default on cluster creation | MetalLB v0.15.3 native manifest deploys controller Deployment + speaker DaemonSet to metallb-system namespace |
| MLB-02 | IP address pool is auto-detected from the Docker network subnet without user input | `docker/podman/nerdctl network inspect kind` JSON output exposes IPAM.Config[0].Subnet; carve /27 from it |
| MLB-03 | IPAddressPool and L2Advertisement custom resources are applied after MetalLB webhook is ready | Must wait for `metallb-webhook-configuration` ValidatingWebhookConfiguration endpoint to accept connections; `kubectl wait --for=condition=Available` on controller deployment then apply CRs |
| MLB-04 | Services of type LoadBalancer receive an EXTERNAL-IP within seconds of creation | L2 mode with correct IPAddressPool + L2Advertisement produces EXTERNAL-IP assignment |
| MLB-05 | Subnet detection works with Docker provider | `docker network inspect kind --format '{{(index .IPAM.Config 0).Subnet}}'` returns IPv4 CIDR |
| MLB-06 | Subnet detection works with Podman provider | `podman network inspect kind` returns JSON with same IPAM structure; must filter for IPv4 subnet only |
| MLB-07 | Subnet detection works with Nerdctl provider | `nerdctl network inspect kind --format={{.Name}}` — nerdctl uses same docker-compatible JSON IPAM output |
| MLB-08 | User can disable MetalLB via `addons.metalLB: false` in cluster config | Already wired in create.go `runAddon("MetalLB", opts.Config.Addons.MetalLB, ...)` — stub must return nil |
</phase_requirements>

## Summary

Phase 2 replaces the `installmetallb` stub action with a real implementation that installs MetalLB v0.15.3, detects the active Docker/Podman/Nerdctl "kind" network subnet, and applies the IPAddressPool + L2Advertisement custom resources that make services of type LoadBalancer functional.

The implementation has three sequential concerns: (1) apply the MetalLB native manifest (embedded at build time via `go:embed`), (2) wait for the controller webhook to be ready before applying CRs, and (3) detect the IPv4 CIDR of the "kind" network via the appropriate container runtime CLI and carve a usable IP range from it. The provider type is identified via the `Provider.String()` method which returns "docker", "podman", or "nerdctl". All three providers expose network IPAM information through their respective `network inspect` commands with compatible JSON output shapes.

The primary risk is Podman rootless mode: the MetalLB speaker DaemonSet requires `NET_RAW` capability and `hostNetwork: true` for ARP announcements. Rootless Podman containers run in an isolated network namespace with restricted capabilities, making L2 mode announcements likely non-functional. The blocker noted in the phase context is real. The implementation should install MetalLB regardless but must emit a clear runtime warning for rootless Podman. The feature still satisfies MLB-06 (subnet detection works) even if L2 ARP does not reach the host.

**Primary recommendation:** Embed `metallb-native.yaml` via `go:embed` in the `installmetallb` package, detect the provider type from `ctx.Provider.String()`, run the matching `network inspect` command on the host (not inside the node container), parse IPAM.Config for the first IPv4 subnet, carve the upper /27 of that subnet for the MetalLB pool, wait for the controller Deployment to be Available, then apply IPAddressPool + L2Advertisement CRs via `node.Command("kubectl", "apply", ...)`.

## Standard Stack

### Core
| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| MetalLB native manifest | v0.15.3 | Installs controller + speaker + RBAC + webhook + CRDs | Current stable release; BGP mode excluded (out of scope per REQUIREMENTS.md) |
| `go:embed` (stdlib) | Go 1.16+ (project is 1.17+) | Embed YAML manifest at compile time | Decided in prior research; offline-capable, no external tools |
| `encoding/json` (stdlib) | - | Parse `network inspect` JSON output | Already used throughout providers package |
| `net` (stdlib) | - | Parse and manipulate CIDR subnets | Already used in docker/podman/nerdctl network.go |
| `exec.Command` (local) | - | Run container CLI on host for network inspect | Pattern established throughout codebase |
| `kubectl wait` (inside node) | - | Block until controller Deployment is Available | Used by waitforready action; same pattern |
| `kubectl apply` (inside node) | - | Apply IPAddressPool + L2Advertisement CRs | Same pattern as installcni action |

### Supporting
| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| `strings.NewReader` | stdlib | Pipe manifest YAML into kubectl stdin | Same pattern as installcni.go line 126 |
| `exec.OutputLines` / `exec.Output` | local pkg | Capture CLI output | Already used in nodeutils, waitforready |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `go:embed` manifest | Fetch from GitHub URL at runtime | Requires internet; breaks offline; decided against |
| `kubectl wait --for=condition=Available` | Poll webhook endpoint directly | kubectl wait is simpler and already in codebase |
| Carve upper /27 from detected subnet | Use entire subnet as pool | Risks collision with infrastructure IPs (gateway, DNS) |
| L2 mode | BGP mode | BGP requires external router — explicitly out of scope per REQUIREMENTS.md |

**Installation:** No new Go module dependencies are required. MetalLB YAML is embedded, `net` and `encoding/json` are stdlib, and all exec patterns are already present.

## Architecture Patterns

### Recommended Project Structure

```
pkg/cluster/internal/create/actions/installmetallb/
├── metallb.go          # Action implementation (Execute func)
├── subnet.go           # detectSubnet(providerName string) (string, error)
├── manifests/
│   └── metallb-native.yaml   # Embedded at build time via go:embed
```

The `manifests/` subdirectory follows the existing pattern already established in the kind codebase (e.g. `/kind/manifests/default-cni.yaml` on the node), but here the manifest lives in the Go package directory and is embedded at compile time rather than baked into the node image.

### Pattern 1: go:embed Static Manifest

**What:** Embed the MetalLB YAML directly into the binary at compile time.
**When to use:** All addon manifests (decided in Phase 1 research as the standard approach).
**Example:**
```go
// Source: Go stdlib embed docs + prior research decision
package installmetallb

import _ "embed"

//go:embed manifests/metallb-native.yaml
var metalLBManifest string
```

### Pattern 2: Host-Side Network Inspect (Provider-Dispatched)

**What:** Run `docker/podman/nerdctl network inspect kind` on the host (not inside the node container) to read the IPv4 CIDR of the kind bridge network.
**When to use:** During MetalLB action execution, before applying IPAddressPool CR.
**Example:**
```go
// Source: derived from providers/docker/provider.go exec pattern
// and from providers/docker/network.go JSON parse pattern

import (
    "encoding/json"
    "net"
    "strings"
    "sigs.k8s.io/kind/pkg/exec"
)

type ipamConfig struct {
    Subnet  string `json:"Subnet"`
    Gateway string `json:"Gateway"`
}

type networkInspect struct {
    IPAM struct {
        Config []ipamConfig `json:"Config"`
    } `json:"IPAM"`
}

// detectSubnet runs the appropriate CLI to inspect the "kind" network
// and returns the first IPv4 CIDR found in IPAM.Config.
func detectSubnet(providerName string) (string, error) {
    binary := "docker"
    if providerName == "podman" {
        binary = "podman"
    } else if providerName == "nerdctl" {
        binary = "nerdctl"
    }

    out, err := exec.Output(exec.Command(binary, "network", "inspect", "kind"))
    if err != nil {
        return "", errors.Wrap(err, "failed to inspect kind network")
    }

    var networks []networkInspect
    if err := json.Unmarshal(out, &networks); err != nil {
        return "", errors.Wrap(err, "failed to parse network inspect output")
    }
    if len(networks) == 0 {
        return "", errors.New("no kind network found")
    }

    for _, cfg := range networks[0].IPAM.Config {
        ip, _, err := net.ParseCIDR(cfg.Subnet)
        if err != nil {
            continue
        }
        if ip.To4() != nil { // IPv4 only
            return cfg.Subnet, nil
        }
    }
    return "", errors.New("no IPv4 subnet found in kind network")
}
```

Note: `exec.Command` here targets the host binary, not the node container. The node's `Command()` method is for in-container execution (via `docker exec`). Host-side execution uses the top-level `exec.Command(binary, args...)` — the same pattern as `docker network ls` calls in `providers/docker/network.go`.

### Pattern 3: Carve IP Pool from Detected Subnet

**What:** Take the detected IPv4 CIDR (e.g. `172.18.0.0/16`) and carve the upper portion as the MetalLB address pool, avoiding the gateway and low addresses.
**When to use:** After subnet detection, before building IPAddressPool YAML.
**Example:**
```go
// Source: derived from community pattern (michaelheap.com) and net stdlib
// Produces a /27 block from the upper portion of the detected subnet.

func carvePoolFromSubnet(cidr string) (string, error) {
    ip, network, err := net.ParseCIDR(cidr)
    if err != nil {
        return "", errors.Wrap(err, "invalid subnet")
    }
    _ = ip // ParseCIDR returns network IP

    // For a /16 like 172.18.0.0/16, we want something like 172.18.255.200-172.18.255.250
    // Strategy: walk to the upper 25% of the address space, take 50 addresses
    // This avoids gateway (.1), DHCP range (low), and other infrastructure

    // Simple approach: replace last octet range with x.200-x.250 block
    // based on masked address. Works for /16, /24, /20 commons.
    // For kind's Docker networks (172.18.0.0/16 or 192.168.x.0/24):

    // Get the base network address
    base := network.IP.To4()
    if base == nil {
        return "", errors.New("only IPv4 subnets are supported")
    }

    ones, bits := network.Mask.Size()
    _ = bits

    if ones <= 16 {
        // For /16 networks (172.18.0.0/16): use 172.18.255.200-172.18.255.250
        return fmt.Sprintf("%d.%d.255.200-%d.%d.255.250",
            base[0], base[1], base[0], base[1]), nil
    }
    // For /24 networks (192.168.207.0/24): use 192.168.207.200-192.168.207.250
    return fmt.Sprintf("%d.%d.%d.200-%d.%d.%d.250",
        base[0], base[1], base[2], base[0], base[1], base[2]), nil
}
```

### Pattern 4: Wait for MetalLB Webhook Before Applying CRs

**What:** After applying the base manifest, wait for the MetalLB controller to be Available before applying IPAddressPool and L2Advertisement. Applying CRs before the webhook is ready causes `InternalError: failed calling webhook` failures.
**When to use:** Between step "apply manifest" and step "apply CRs".
**Example:**
```go
// Source: derived from waitforready.go pattern, kubectl wait docs
// Run inside the node container (same as other kubectl calls)

if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "wait",
    "--namespace=metallb-system",
    "--for=condition=Available",
    "deployment/controller",
    "--timeout=120s",
).Run(); err != nil {
    return errors.Wrap(err, "timed out waiting for MetalLB controller")
}
```

### Pattern 5: Apply CRs via Stdin (same as installcni)

**What:** Build IPAddressPool + L2Advertisement YAML in-memory and pipe to `kubectl apply -f -`.
**When to use:** After webhook is ready.
**Example:**
```go
// Source: derived from installcni.go lines 123-128

const crTemplate = `---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kind-pool
  namespace: metallb-system
spec:
  addresses:
  - %s
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: kind-l2advert
  namespace: metallb-system
spec:
  ipAddressPools:
  - kind-pool
`

crYAML := fmt.Sprintf(crTemplate, poolRange)
if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "-f", "-",
).SetStdin(strings.NewReader(crYAML)).Run(); err != nil {
    return errors.Wrap(err, "failed to apply MetalLB CRs")
}
```

### Anti-Patterns to Avoid

- **Applying CRs before webhook is ready:** Results in `InternalError: failed calling webhook "ipaddresspoolvalidationwebhook.metallb.io"`. Always wait for `deployment/controller` Available first.
- **Using the entire detected subnet as the pool:** Will conflict with node IPs, gateway, and DNS. Always carve a small range from the upper portion.
- **Running `network inspect` inside the node container:** The node container does not have `docker`/`podman`/`nerdctl` installed. Network inspect must run on the host via top-level `exec.Command(binary, ...)`.
- **Selecting IPv6 subnets:** Docker/Podman networks may return both IPv4 and IPv6 entries in `IPAM.Config`. Filter for IPv4 only using `ip.To4() != nil`.
- **Assuming a fixed network name:** The network name is `kind` by default but can be overridden via `KIND_EXPERIMENTAL_DOCKER_NETWORK` env. Check providers/docker/provider.go — the action runs after provisioning so the network exists, but using the constant `fixedNetworkName = "kind"` is the standard approach.
- **Fetching manifests at runtime:** Decided against in prior research. Always embed via `go:embed`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CIDR parsing and manipulation | Custom octet arithmetic | `net.ParseCIDR`, `net.IPNet` | Standard library handles all edge cases including boundary IPs |
| JSON parsing of network inspect | Custom string parsing | `encoding/json` + typed structs | Network inspect output format can vary slightly across provider versions |
| Waiting for Deployment readiness | Custom polling loop | `kubectl wait --for=condition=Available` | Handles all edge cases; already used by kind in waitforready |
| MetalLB manifest contents | Writing YAML by hand | Download official `metallb-native.yaml` and embed it | Official manifest has correct RBAC, CRDs, security contexts; hand-rolling risks missing resources |
| Provider type detection | Custom detection logic | `ctx.Provider.String()` | Provider already knows its type; "docker"/"podman"/"nerdctl" |

**Key insight:** The hard parts of MetalLB integration (correct RBAC, CRD ordering, webhook cert management, speaker security context) are entirely solved by the official `metallb-native.yaml` manifest. The only custom logic needed is subnet detection and CR templating — both simple operations using standard library types.

## Common Pitfalls

### Pitfall 1: Webhook Not Ready When CRs Applied
**What goes wrong:** `kubectl apply` of IPAddressPool fails with `InternalError: failed calling webhook "ipaddresspoolvalidationwebhook.metallb.io"`.
**Why it happens:** The MetalLB controller starts its metrics endpoint (readiness probe) before the webhook TLS is fully established. The pod appears Ready but the webhook server is not accepting connections yet.
**How to avoid:** Use `kubectl wait --for=condition=Available deployment/controller --timeout=120s` AFTER the manifest is applied. This waits for the controller's readiness probe which is wired to the webhook server being ready (as of MetalLB v0.13+).
**Warning signs:** Error contains "failed calling webhook" or "connection refused" on port 9443.

### Pitfall 2: IPv6 Subnet Selected Instead of IPv4
**What goes wrong:** MetalLB receives an IPv6 address pool (e.g. `fc00:f853:ccd:e793::/64`) and services do not get IPv4 EXTERNAL-IPs.
**Why it happens:** Kind Docker networks have both IPv4 and IPv6 entries in `IPAM.Config`. The JSON array order is not guaranteed.
**How to avoid:** Iterate `IPAM.Config` and filter for entries where `net.ParseCIDR(subnet).IP.To4() != nil`.
**Warning signs:** Pool range contains `:` characters; services stuck in `<pending>` for EXTERNAL-IP.

### Pitfall 3: Podman Rootless L2 Speaker Cannot ARP
**What goes wrong:** MetalLB installs and pods run, but services never get EXTERNAL-IP assigned because the speaker cannot send ARP responses.
**Why it happens:** Rootless Podman runs in a user network namespace. The MetalLB speaker DaemonSet needs `hostNetwork: true` and `NET_RAW` capability to send ARP packets on the host's bridge network. In rootless mode, the container's "host" network is the user namespace, not the real host network.
**How to avoid:** Cannot be fixed without rootful containers. Detect rootless mode via `ctx.Provider.Info().Rootless` and emit a clear warning: "MetalLB L2 speaker cannot ARP in rootless Podman mode; LoadBalancer services will not receive EXTERNAL-IP".
**Warning signs:** `p.info.Rootless == true` for Podman provider.

### Pitfall 4: Network Inspect Runs Inside Node Container
**What goes wrong:** `node.Command("docker", "network", "inspect", "kind")` fails because docker/podman/nerdctl is not installed inside the kind node container.
**Why it happens:** The kind node containers are minimal Ubuntu/Debian images with only Kubernetes components. Container runtime CLIs are not available inside them.
**How to avoid:** Use top-level `exec.Command(binary, "network", "inspect", "kind")` to run on the host. This is the same pattern used in `providers/docker/network.go` (e.g., lines 160-161, 214-216).
**Warning signs:** `exec: "docker": executable file not found in $PATH` error in action output.

### Pitfall 5: Pool Range Conflicts with Node/Gateway IPs
**What goes wrong:** MetalLB assigns an IP that conflicts with a kind node container IP or the network gateway, causing routing failures.
**Why it happens:** Kind node containers receive IPs from the kind network DHCP range, typically starting at `.2`, `.3`, etc. The gateway is at `.1`. If the pool range starts at `.1` or overlaps with node IPs, ARP conflicts occur.
**How to avoid:** Carve the pool from the upper portion of the address space (.200-.250) which DHCP typically never reaches for the small number of kind nodes.
**Warning signs:** Services get EXTERNAL-IP but traffic is unreachable; `kubectl describe svc` shows the IP assigned.

### Pitfall 6: Missing `metallb-system` Namespace Wait
**What goes wrong:** `kubectl wait` fails immediately because the `metallb-system` namespace and `controller` deployment don't exist yet when wait runs right after `kubectl apply`.
**Why it happens:** `kubectl apply` is asynchronous — it submits the manifests but the operator has not yet created all resources. The CRDs and namespace may take a second to appear.
**How to avoid:** Use a brief retry or use `kubectl wait --for=jsonpath='{.status.availableReplicas}'=1` with a timeout. Alternatively, use `kubectl rollout status deployment/controller -n metallb-system --timeout=120s` which inherently waits for the rollout to begin.
**Warning signs:** "Error from server (NotFound): deployments.apps 'controller' not found" immediately after apply.

## Code Examples

Verified patterns from official sources and codebase:

### Complete Execute Function Structure
```go
// Source: derived from installcni.go + waitforready.go patterns in this codebase
func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing MetalLB")
    defer ctx.Status.End(false)

    allNodes, err := ctx.Nodes()
    if err != nil {
        return err
    }
    controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)
    if err != nil {
        return err
    }
    node := controlPlanes[0]

    // Step 1: warn for rootless Podman
    info, err := ctx.Provider.Info()
    if err != nil {
        return err
    }
    if info.Rootless {
        ctx.Logger.Warn("MetalLB L2 speaker cannot ARP in rootless mode; LoadBalancer services may not get EXTERNAL-IP")
    }

    // Step 2: apply base manifest (embedded)
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(metalLBManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply MetalLB manifest")
    }

    // Step 3: wait for controller webhook readiness
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait",
        "--namespace=metallb-system",
        "--for=condition=Available",
        "deployment/controller",
        "--timeout=120s",
    ).Run(); err != nil {
        return errors.Wrap(err, "MetalLB controller did not become available")
    }

    // Step 4: detect subnet from host network
    subnet, err := detectSubnet(ctx.Provider.String())
    if err != nil {
        return errors.Wrap(err, "failed to detect kind network subnet for MetalLB")
    }

    // Step 5: carve pool range
    poolRange, err := carvePoolFromSubnet(subnet)
    if err != nil {
        return errors.Wrap(err, "failed to compute MetalLB IP pool")
    }

    // Step 6: apply IPAddressPool + L2Advertisement
    crYAML := fmt.Sprintf(crTemplate, poolRange)
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(crYAML)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply MetalLB CRs")
    }

    ctx.Status.End(true)
    return nil
}
```

### go:embed Declaration
```go
// Source: Go stdlib embed documentation; requires Go 1.16+ (project minimum is 1.17)
package installmetallb

import (
    _ "embed"
    "fmt"
    "strings"
    // ...
)

//go:embed manifests/metallb-native.yaml
var metalLBManifest string
```

### Network Inspect JSON Structure (Docker/Nerdctl)
```json
[
  {
    "Name": "kind",
    "IPAM": {
      "Config": [
        {"Subnet": "172.18.0.0/16", "Gateway": "172.18.0.1"},
        {"Subnet": "fc00:f853:ccd:e793::/64"}
      ]
    }
  }
]
```

### Network Inspect JSON Structure (Podman)
```json
[
  {
    "name": "kind",
    "subnets": [
      {"subnet": "10.89.0.0/24", "gateway": "10.89.0.1"}
    ]
  }
]
```

**IMPORTANT:** Podman's network inspect JSON uses a different schema than Docker/Nerdctl. Podman uses `subnets[].subnet` (lowercase `name`, `subnets` array) while Docker/Nerdctl use `IPAM.Config[].Subnet`. The subnet detection code must handle both schemas.

### Podman Network Inspect Schema
```go
// Podman-specific schema for network inspect output
type podmanNetworkInspect struct {
    Subnets []struct {
        Subnet  string `json:"subnet"`
        Gateway string `json:"gateway"`
    } `json:"subnets"`
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| MetalLB ConfigMap-based config | CRD-based config (IPAddressPool, L2Advertisement) | v0.13.0 (2022) | ConfigMap approach is removed; CRDs required |
| `metallb.io/v1beta2` API | `metallb.io/v1beta1` is stable and current | v0.13.2+ | Use `v1beta1` for IPAddressPool and L2Advertisement |
| Apply CRs immediately after manifest | Wait for webhook before applying CRs | v0.13.0+ (webhook added) | Required to avoid webhook not-ready errors |
| MetalLB v0.14.x | MetalLB v0.15.3 (current stable) | 2024-2025 | v0.15.3 is the version to embed |
| Fetch manifest at runtime | Embed via go:embed | Project decision (prior research) | Enables offline operation |

**Deprecated/outdated:**
- ConfigMap-based MetalLB configuration: entirely removed in v0.13.0+; do not use
- `metallb.io/v1alpha1` API: removed; use `metallb.io/v1beta1`
- `metallb.universe.tf` as the only docs URL: project also lives at `metallb.io`

## Open Questions

1. **Podman rootless: warn only or also skip CRs?**
   - What we know: Rootless Podman cannot send ARP via NET_RAW. MetalLB pods will run but L2 mode won't announce IPs.
   - What's unclear: Whether to skip CR application (pool configuration) entirely for rootless, or apply it anyway (future rootful upgrade path).
   - Recommendation: Apply CRs regardless, emit warning. The cluster remains usable and rootful Podman users benefit. This is consistent with "warn and continue" addon philosophy.

2. **Podman network inspect JSON schema difference**
   - What we know: Podman uses `subnets[].subnet` while Docker/Nerdctl use `IPAM.Config[].Subnet` in network inspect JSON.
   - What's unclear: Whether newer Podman versions have converged toward Docker-compatible output.
   - Recommendation: Implement a two-schema parser: try Docker schema first, fall back to Podman schema. Test both paths explicitly.

3. **Nerdctl binary name vs. Finch**
   - What we know: The nerdctl provider already handles `finch` as an alias (see `providers/nerdctl/provider.go` lines 49-58). The `Provider.String()` returns "nerdctl" for both.
   - What's unclear: Whether `finch` and `nerdctl` produce identical `network inspect` output format.
   - Recommendation: Use `ctx.Provider.String()` to get "nerdctl" then look up the actual binary via the same fallback logic. Alternatively, expose a `Binary() string` method (already exists on nerdctl provider at line 80) — but the Provider interface does not include it. Simplest approach: use "nerdctl" as the binary name for "nerdctl" providers, accepting that Finch users may need to test separately.

4. **Network name when KIND_EXPERIMENTAL_DOCKER_NETWORK is set**
   - What we know: Docker provider uses `KIND_EXPERIMENTAL_DOCKER_NETWORK` env to override the network name from "kind".
   - What's unclear: Whether MetalLB subnet detection should respect this env variable.
   - Recommendation: Read `KIND_EXPERIMENTAL_DOCKER_NETWORK` in `detectSubnet` and use it if set; otherwise use "kind". This mirrors the existing provider behavior.

## Sources

### Primary (HIGH confidence)
- MetalLB official docs: https://metallb.universe.tf/installation/ — version v0.15.3 confirmed current stable
- MetalLB official docs: https://metallb.universe.tf/configuration/ — IPAddressPool + L2Advertisement API verified as `metallb.io/v1beta1`
- MetalLB native manifest: https://raw.githubusercontent.com/metallb/metallb/v0.15.3/config/manifests/metallb-native.yaml — exact resources confirmed (ValidatingWebhookConfiguration named `metallb-webhook-configuration`, speaker DaemonSet with NET_RAW + hostNetwork)
- MetalLB release notes: https://metallb.universe.tf/release-notes/ — v0.15.3 is latest stable
- Kinder codebase: `pkg/cluster/internal/providers/docker/network.go` — exec pattern for host-side CLI calls
- Kinder codebase: `pkg/cluster/internal/create/actions/installcni/cni.go` — manifest-via-stdin kubectl pattern
- Kinder codebase: `pkg/cluster/internal/create/actions/waitforready/waitforready.go` — readiness wait pattern
- Kinder codebase: `pkg/cluster/internal/providers/provider.go` — ProviderInfo.Rootless field
- Kinder codebase: `pkg/internal/apis/config/convert_v1alpha4.go` — confirms nil *bool -> true conversion is already done

### Secondary (MEDIUM confidence)
- michaelheap.com/metallb-ip-address-pool/ — subnet carving approach verified against docker network inspect output format
- MetalLB webhook issue #1597 and PR #1648 — webhook readiness timing problem documented and confirmed
- MetalLB kind issue #2971 — `kubectl wait` for MetalLB controller pod documented as correct approach
- Podman rootless networking docs — confirmed rootless containers use user network namespace with limited capabilities

### Tertiary (LOW confidence)
- Podman network inspect JSON schema difference — inferred from Podman docs and community reports; needs validation with actual `podman network inspect` output during implementation
- Nerdctl/Finch binary name consistency — inferred from nerdctl provider code; not directly verified

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — MetalLB v0.15.3, go:embed, kubectl wait all verified against official sources
- Architecture: HIGH — all patterns derived directly from existing codebase code in use
- Pitfalls: HIGH for webhook timing and IPv6 filtering (documented in MetalLB issues); MEDIUM for Podman schema difference (inferred)
- Subnet carving algorithm: MEDIUM — confirmed approach works for /16 and /24 networks; edge cases for unusual CIDRs need testing

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (MetalLB releases regularly but v0.15.3 is stable)
