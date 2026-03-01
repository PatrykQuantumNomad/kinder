# Phase 3: Metrics Server - Research

**Researched:** 2026-03-01
**Domain:** Kubernetes Metrics Server installation, go:embed manifest, --kubelet-insecure-tls for kind clusters, HPA metrics API readiness
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MET-01 | Metrics Server is installed with `--kubelet-insecure-tls` flag by default | Official components.yaml has args section; embed it with `--kubelet-insecure-tls` pre-added; kind kubelets use self-signed certs so this flag is required |
| MET-02 | `kubectl top nodes` returns data within 60 seconds of cluster creation | Metrics Server deployment has `initialDelaySeconds: 20` readiness probe + `--metric-resolution=15s`; after deployment Available, first scrape cycle completes within 60s total; wait for deployment Available before returning |
| MET-03 | `kubectl top pods` works for pods in any namespace | Metrics Server RBAC grants get/list/watch on pods cluster-wide via ClusterRole; the official components.yaml includes correct RBAC; no special configuration needed |
| MET-04 | HPA can read CPU/memory metrics from the Metrics API | Metrics Server registers as `v1beta1.metrics.k8s.io` APIService; once Deployment is Available, HPA controller can read from this API; no extra configuration needed beyond installing Metrics Server |
| MET-05 | User can disable Metrics Server via `addons.metricsServer: false` in cluster config | Already wired in create.go `runAddon("Metrics Server", opts.Config.Addons.MetricsServer, ...)` — Execute function only runs when enabled; MET-05 is already satisfied at the wiring level |
</phase_requirements>

## Summary

Phase 3 replaces the `installmetricsserver` stub action with a complete implementation that installs Metrics Server v0.8.1 in the `kube-system` namespace with the `--kubelet-insecure-tls` flag required for kind clusters. Kind kubelets serve self-signed TLS certificates; without this flag, Metrics Server cannot scrape kubelet metrics and `kubectl top` will fail with "Metrics API not available".

The implementation has two sequential concerns: (1) apply the embedded Metrics Server manifest (which includes the `--kubelet-insecure-tls` flag pre-patched into the Deployment args), and (2) wait for the `deployment/metrics-server` to become Available in `kube-system`. Unlike MetalLB, Metrics Server has no webhook and requires no CR application — the single manifest apply + deployment readiness wait is sufficient. The 60-second data availability requirement (MET-02) is met naturally: the Deployment readiness probe uses `initialDelaySeconds: 20` and `--metric-resolution=15s` collection interval; by the time the deployment is Available and action returns, the first scrape cycle will complete well within 60 seconds from when `kinder create cluster` finishes.

MET-05 (disable via config) is already implemented at the `create.go` level via the `runAddon` closure. The stub action is already registered with the correct enable/disable wiring. Phase 3 only needs to replace the stub's Execute body — no changes to `create.go` are required.

**Primary recommendation:** Download the official Metrics Server v0.8.1 `components.yaml`, manually add `--kubelet-insecure-tls` to the Deployment container args, embed via `go:embed manifests/components.yaml` in `installmetricsserver`, apply it via kubectl, and wait for `deployment/metrics-server` Available with a 120s timeout.

## Standard Stack

### Core
| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| Metrics Server manifest | v0.8.1 (2026-01-29) | Installs Deployment + RBAC + Service + APIService in kube-system | Current stable release per official GitHub releases |
| `go:embed` (stdlib) | Go 1.16+ | Embed components.yaml at compile time | Project-wide decision from Phase 1 research; offline-capable |
| `strings.NewReader` (stdlib) | - | Pipe manifest into kubectl stdin | Same pattern as MetalLB action (already in codebase) |
| `kubectl apply` (inside node) | - | Apply manifest from stdin | Established pattern — same as installmetallb |
| `kubectl wait --for=condition=Available` (inside node) | - | Block until Deployment is serving metrics | Same pattern as MetalLB webhook wait |

### Supporting
| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| `registry.k8s.io/metrics-server/metrics-server:v0.8.1` | v0.8.1 | Container image in embedded manifest | Already pinned in the official components.yaml release overlay |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Embedded pre-patched manifest | Fetch + patch at runtime with `kubectl patch` | Runtime patching requires two kubectl calls and internet access; go:embed is simpler and offline |
| Embedded pre-patched manifest | Kustomize overlay to add the arg | Requires `kustomize` binary; not available inside kind node containers; go:embed is the decided approach |
| Wait for `deployment/metrics-server` Available | Poll `kubectl top nodes` until it returns data | Polling top is fragile (timing-dependent); waiting for Deployment Available is deterministic |
| v0.8.1 | v0.7.2 (previous stable) | v0.8.1 is current stable (Jan 2026); v0.7.2 is older (Aug 2024); use latest |

**Installation:** No new Go module dependencies required. All patterns already exist in the codebase.

## Architecture Patterns

### Recommended Project Structure

```
pkg/cluster/internal/create/actions/installmetricsserver/
├── metricsserver.go          # Action implementation (Execute func)
├── manifests/
│   └── components.yaml       # Metrics Server v0.8.1 with --kubelet-insecure-tls pre-added
```

This mirrors the exact pattern established by `installmetallb/`:
- `metallb.go` → `metricsserver.go`
- `manifests/metallb-native.yaml` → `manifests/components.yaml`

No `subnet.go` equivalent needed — Metrics Server requires no cluster-specific configuration.

### Pattern 1: go:embed Static Manifest with Pre-Patched Flag

**What:** Embed the Metrics Server components.yaml with `--kubelet-insecure-tls` already present in the Deployment args. The manifest is modified once (at setup time), not at runtime.
**When to use:** All addon manifests use this approach (project-wide decision).
**Example:**
```go
// Package installmetricsserver implements the action to install Metrics Server
package installmetricsserver

import _ "embed"

//go:embed manifests/components.yaml
var metricsServerManifest string
```

The embedded `components.yaml` Deployment args section must look like:
```yaml
args:
  - --cert-dir=/tmp
  - --secure-port=10250
  - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
  - --kubelet-use-node-status-port
  - --metric-resolution=15s
  - --kubelet-insecure-tls
```

### Pattern 2: Apply Manifest + Wait for Deployment Available

**What:** Apply the embedded manifest via kubectl stdin, then wait for the deployment to be Available.
**When to use:** Only readiness wait is needed — Metrics Server has no webhook (unlike MetalLB). No CR application follows.
**Example:**
```go
// Source: derived from installmetallb/metallb.go Execute pattern
func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing Metrics Server")
    defer ctx.Status.End(false)

    allNodes, err := ctx.Nodes()
    if err != nil {
        return errors.Wrap(err, "failed to list cluster nodes")
    }
    controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)
    if err != nil {
        return errors.Wrap(err, "failed to find control plane nodes")
    }
    if len(controlPlanes) == 0 {
        return errors.New("no control plane nodes found")
    }
    node := controlPlanes[0]

    // Apply embedded manifest
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(metricsServerManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply Metrics Server manifest")
    }

    // Wait for Deployment to be Available (serves /readyz which reflects actual metric availability)
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait",
        "--namespace=kube-system",
        "--for=condition=Available",
        "deployment/metrics-server",
        "--timeout=120s",
    ).Run(); err != nil {
        return errors.Wrap(err, "Metrics Server deployment did not become available")
    }

    ctx.Status.End(true)
    return nil
}
```

### Pattern 3: Manifest Preparation (one-time, not runtime)

**What:** Obtain the official v0.8.1 `components.yaml` and add `--kubelet-insecure-tls` to the Deployment args before embedding.
**When to use:** During phase implementation (build-time setup), not at runtime.
**How:**
```bash
# Download official manifest
curl -L https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.8.1/components.yaml \
  -o pkg/cluster/internal/create/actions/installmetricsserver/manifests/components.yaml

# Manually add --kubelet-insecure-tls to the Deployment args section in the downloaded file
# (The args section already contains cert-dir, secure-port, kubelet-preferred-address-types,
#  kubelet-use-node-status-port, metric-resolution=15s)
# Add: - --kubelet-insecure-tls
```

### Anti-Patterns to Avoid

- **Patching the manifest at runtime with `kubectl patch`:** Requires two kubectl invocations, is more error-prone, and contradicts the go:embed offline approach.
- **Waiting for `kubectl top nodes` output instead of Deployment Available:** `kubectl top` can fail due to timing after the deployment is ready; Deployment Available is the correct readiness signal.
- **Applying the manifest without `--kubelet-insecure-tls`:** Kind kubelets use self-signed certificates that Metrics Server cannot verify; the default manifest will fail with TLS errors and `kubectl top` will return "Metrics API not available".
- **Using Kustomize to add the flag inside the action:** The kind node containers do not have `kustomize` installed.
- **Running `kubectl wait` immediately on a resource that may not exist yet:** Apply first, then wait. The `--timeout=120s` handles the brief creation delay.
- **Using `kubectl create` instead of `kubectl apply`:** `kubectl apply` is idempotent; `kubectl create` fails if resources already exist. The MetalLB action uses `apply` and the Metrics Server action should too.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Metrics scraping from kubelets | Custom kubelet metrics collector | Metrics Server v0.8.1 manifest | Official Metrics Server handles kubelet API versions, auth, TLS, aggregation API registration |
| APIService registration | Custom apiserver extension | Metrics Server manifest (includes APIService resource) | The official manifest registers `v1beta1.metrics.k8s.io` correctly with correct group/version priorities |
| RBAC for metrics access | Custom ClusterRole/ClusterRoleBinding | Metrics Server manifest (includes rbac.yaml) | Metrics Server RBAC is non-trivial: aggregation, auth delegation, node/metrics access all required |
| Deployment readiness detection | Custom polling loop or timeout | `kubectl wait --for=condition=Available` | kubectl wait handles race conditions; readiness probe at /readyz reflects actual metric availability |
| Flag injection at runtime | Custom YAML manipulation in Go | Pre-patch the manifest file before go:embed | Simpler, testable, offline; consistent with MetalLB pattern |

**Key insight:** The entire complexity of Metrics Server (kubelet TLS, aggregated API registration, RBAC, health checking) is solved by the official manifest. The only kind-specific customization needed is `--kubelet-insecure-tls` in the Deployment args — a one-line YAML addition.

## Common Pitfalls

### Pitfall 1: Missing --kubelet-insecure-tls
**What goes wrong:** `kubectl top nodes` returns "Error from server (ServiceUnavailable): the server is currently unable to handle the request (get nodes.metrics.k8s.io)" or "Metrics API not available".
**Why it happens:** Kind kubelets serve self-signed TLS certificates that Metrics Server cannot verify by default. Metrics Server fails to scrape kubelet metrics and the APIService shows `Available=False`.
**How to avoid:** Ensure `--kubelet-insecure-tls` is present in the Deployment container args in the embedded manifest. Verify by inspecting the YAML before embedding.
**Warning signs:** `kubectl get apiservice v1beta1.metrics.k8s.io` shows `False` for AVAILABLE column; pod logs show "x509: certificate signed by unknown authority".

### Pitfall 2: Deployment Available but Metrics Not Yet Populated
**What goes wrong:** Deployment is Available but `kubectl top nodes` returns "Error from server: the server is currently unable to handle the request" for a brief period.
**Why it happens:** Metrics Server has `initialDelaySeconds: 20` for readiness, and `--metric-resolution=15s` for collection interval. Being Available means the server is accepting requests, but the first scrape cycle may not have completed yet.
**How to avoid:** The 60-second MET-02 requirement is measured from `kinder create cluster` completing, not from Metrics Server Deployment Available. By the time addons are installed sequentially (MetalLB runs first), sufficient time has passed. The action should return after waiting for Deployment Available — this is sufficient.
**Warning signs:** If `kubectl top` fails immediately after cluster creation but works 10-15 seconds later, this is the first-scrape delay, not a bug.

### Pitfall 3: components.yaml Not Downloaded Correctly
**What goes wrong:** The embedded manifest is truncated or missing resources.
**Why it happens:** GitHub releases redirect to a signed CDN URL; tools like `curl` must follow redirects (`-L` flag).
**How to avoid:** Use `curl -L` to follow redirects when downloading the manifest. Verify the downloaded file contains all expected resources: ServiceAccount, ClusterRoles, ClusterRoleBindings, RoleBinding, Deployment, Service, APIService (7+ resources).
**Warning signs:** `go build` succeeds but `kubectl apply` fails with "error validating" or incomplete resources.

### Pitfall 4: Wrong Namespace in kubectl wait
**What goes wrong:** `kubectl wait` fails with "Error from server (NotFound): deployments.apps 'metrics-server' not found".
**Why it happens:** Metrics Server deploys to `kube-system` namespace, not a dedicated namespace (unlike MetalLB which uses `metallb-system`).
**How to avoid:** Use `--namespace=kube-system` in the `kubectl wait` command.
**Warning signs:** NotFound error on `deployment/metrics-server` immediately after apply.

### Pitfall 5: Attempting to Wait for APIService instead of Deployment
**What goes wrong:** Complex kubectl wait command targeting the APIService resource which has inconsistent "Available" condition behavior.
**Why it happens:** Developers try to be precise by waiting for the actual metrics API endpoint to be available.
**How to avoid:** Wait for `deployment/metrics-server --for=condition=Available` in kube-system. The Deployment's readiness probe at `/readyz` is wired to the metrics server's ability to respond to API requests. This is the correct readiness signal.
**Warning signs:** N/A — this is a design choice pitfall, not a runtime failure.

## Code Examples

Verified patterns from official sources and codebase:

### Complete Execute Function
```go
// Source: derived from installmetallb/metallb.go pattern in this codebase
// (same codebase, Phase 2 established this pattern)

package installmetricsserver

import (
    _ "embed"
    "strings"

    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/nodeutils"
    "sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/components.yaml
var metricsServerManifest string

type action struct{}

func NewAction() actions.Action {
    return &action{}
}

func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing Metrics Server")
    defer ctx.Status.End(false)

    allNodes, err := ctx.Nodes()
    if err != nil {
        return errors.Wrap(err, "failed to list cluster nodes")
    }
    controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)
    if err != nil {
        return errors.Wrap(err, "failed to find control plane nodes")
    }
    if len(controlPlanes) == 0 {
        return errors.New("no control plane nodes found")
    }
    node := controlPlanes[0]

    // Apply the embedded Metrics Server manifest (includes --kubelet-insecure-tls in args)
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(metricsServerManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply Metrics Server manifest")
    }

    // Wait for Deployment to be Available
    // Metrics Server has no webhook (unlike MetalLB), so this is the only wait needed
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait",
        "--namespace=kube-system",
        "--for=condition=Available",
        "deployment/metrics-server",
        "--timeout=120s",
    ).Run(); err != nil {
        return errors.Wrap(err, "Metrics Server deployment did not become available")
    }

    ctx.Status.End(true)
    return nil
}
```

### go:embed Declaration
```go
// Source: Go stdlib embed docs; requires Go 1.16+ (project minimum is 1.17)
// Must be in the same package directory as manifests/components.yaml

import _ "embed"

//go:embed manifests/components.yaml
var metricsServerManifest string
```

### Deployment Args Section in components.yaml (after pre-patching)
```yaml
# Source: https://raw.githubusercontent.com/kubernetes-sigs/metrics-server/v0.8.1/manifests/base/deployment.yaml
# + --kubelet-insecure-tls manually added for kind compatibility
containers:
- name: metrics-server
  image: registry.k8s.io/metrics-server/metrics-server:v0.8.1
  args:
    - --cert-dir=/tmp
    - --secure-port=10250
    - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
    - --kubelet-use-node-status-port
    - --metric-resolution=15s
    - --kubelet-insecure-tls
```

### Metrics Server APIService Resource (part of components.yaml)
```yaml
# Source: https://raw.githubusercontent.com/kubernetes-sigs/metrics-server/v0.8.1/manifests/base/apiservice.yaml
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1beta1.metrics.k8s.io
spec:
  service:
    name: metrics-server
    namespace: kube-system
  group: metrics.k8s.io
  version: v1beta1
  insecureSkipTLSVerify: true
  groupPriorityMinimum: 100
  versionPriority: 100
```

### Readiness Probe in Deployment (from official manifest)
```yaml
# Source: https://raw.githubusercontent.com/kubernetes-sigs/metrics-server/v0.8.1/manifests/base/deployment.yaml
readinessProbe:
  httpGet:
    path: /readyz
    port: https
    scheme: HTTPS
  periodSeconds: 10
  failureThreshold: 3
  initialDelaySeconds: 20
```

This means `kubectl wait --for=condition=Available` will not return until at least 20 seconds after the pod starts, then it must pass readiness 3 consecutive times. The wait is correct — it ensures Metrics Server is truly serving before the action returns.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Metrics Server v0.6.x | v0.8.1 (Jan 2026) | Ongoing | v0.8.1 is current stable; uses `registry.k8s.io` image registry |
| `k8s.gcr.io` image registry | `registry.k8s.io` | Kubernetes ecosystem migration (2022-2023) | Old registry is deprecated; v0.8.1 manifest already uses `registry.k8s.io` |
| `--secure-port=4443` | `--secure-port=10250` | Changed in v0.7+ | The port changed between versions; v0.8.1 uses 10250 |
| Poke/patch manifest at runtime | Pre-patch and go:embed | Project decision (Phase 1) | Embed approach is offline and simpler |
| ConfigMap for configuration | Command-line flags | Always flag-based | No ConfigMap needed; flags in Deployment args |
| HA installation with PodDisruptionBudget | Single replica for kind | kind is single-node or small; HA not needed | Use `components.yaml` (not `high-availability.yaml`) |

**Deprecated/outdated:**
- `k8s.gcr.io/metrics-server/metrics-server` image: deprecated, use `registry.k8s.io/metrics-server/metrics-server`
- `--secure-port=4443`: was the default in older versions; v0.8.1 uses 10250
- Applying CRs for metrics configuration: no CRs needed; Metrics Server is pure Deployment + built-in APIService

## Open Questions

1. **Should the action also wait for the APIService to be Available?**
   - What we know: Waiting for `deployment/metrics-server Available` is sufficient for the deployment to be serving. The APIService `v1beta1.metrics.k8s.io` takes a few more seconds to show `Available=True` after the deployment is ready.
   - What's unclear: Whether `kubectl top` can fail in the window between Deployment Available and APIService Available.
   - Recommendation: Wait only for Deployment Available. The 60-second MET-02 window from cluster creation completing is long enough for both. Adding an APIService wait would increase complexity without clear benefit for local dev clusters.

2. **Image pull timing for kind's preloaded images**
   - What we know: Kind node images include commonly-used container images pre-loaded. `registry.k8s.io/metrics-server/metrics-server:v0.8.1` may or may not be included.
   - What's unclear: Whether the current kind node image includes Metrics Server v0.8.1 image layers.
   - Recommendation: Do not assume the image is preloaded. The action should work regardless — Docker will pull the image from the registry if not cached. The 120s timeout for deployment Available accommodates image pull time.

3. **Port 10250 vs. 4443**
   - What we know: v0.8.1 deployment.yaml specifies `--secure-port=10250` and `containerPort: 10250`. The Service targets port name `https` which maps to 10250.
   - What's unclear: Whether the downloaded components.yaml (built from kustomize base + release overlay) preserves this exactly.
   - Recommendation: Verify the downloaded components.yaml container port and args before embedding. If the port differs, adjust accordingly.

## Sources

### Primary (HIGH confidence)
- `https://github.com/kubernetes-sigs/metrics-server/releases` — v0.8.1 confirmed as latest stable release (2026-01-29), verified via GitHub releases page
- `https://raw.githubusercontent.com/kubernetes-sigs/metrics-server/v0.8.1/manifests/base/deployment.yaml` — Full deployment YAML verified: args, readiness probe (initialDelaySeconds: 20), port 10250, image `gcr.io/k8s-staging-metrics-server/metrics-server:master` (overridden by release kustomization)
- `https://raw.githubusercontent.com/kubernetes-sigs/metrics-server/v0.8.1/manifests/components/release/kustomization.yaml` — Image override confirmed: `registry.k8s.io/metrics-server/metrics-server:v0.8.1`
- `https://raw.githubusercontent.com/kubernetes-sigs/metrics-server/v0.8.1/manifests/base/apiservice.yaml` — APIService resource confirmed: `v1beta1.metrics.k8s.io`, group `metrics.k8s.io`, insecureSkipTLSVerify: true
- `https://raw.githubusercontent.com/kubernetes-sigs/metrics-server/v0.8.1/manifests/base/kustomization.yaml` — Base resources confirmed: apiservice.yaml, deployment.yaml, rbac.yaml, service.yaml
- Kinder codebase: `pkg/cluster/internal/create/actions/installmetallb/metallb.go` — Exact pattern for go:embed + kubectl apply stdin + kubectl wait
- Kinder codebase: `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go` — Stub confirmed, package structure, NewAction pattern

### Secondary (MEDIUM confidence)
- `https://kubernetes-sigs.github.io/metrics-server/` — Official Metrics Server docs, confirms `--kubelet-insecure-tls` is the standard flag for clusters with self-signed kubelet certs (like kind)
- `https://gist.github.com/sanketsudake/a089e691286bf2189bfedf295222bd43` — Community guide: "running metric-server on Kind Kubernetes", confirms `--kubelet-insecure-tls` is the required change for kind clusters
- `https://medium.com/@cloudspinx/fix-error-metrics-api-not-available-in-kubernetes-aa10766e1c2f` — Documents the "Metrics API not available" error and confirms `--kubelet-insecure-tls` as the fix for kind-like clusters

### Tertiary (LOW confidence)
- Search results confirm metrics-server readiness probe at `/readyz` reflects actual metric-serving capability — inferred but not directly verified via official docs
- "kubectl top returns data within 15-60 seconds" timing — inferred from `--metric-resolution=15s` + `initialDelaySeconds: 20`; needs empirical testing on kind

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — v0.8.1 verified against official GitHub releases page (Jan 2026); go:embed pattern verified against codebase
- Architecture: HIGH — exact pattern mirrors working Phase 2 MetalLB implementation; no novel patterns introduced
- Pitfalls: HIGH for --kubelet-insecure-tls requirement (verified by official docs and multiple community sources); MEDIUM for timing behavior (inferred from manifest probe configuration)
- Manifest preparation: HIGH — base YAML resources verified directly; release image override verified via kustomization.yaml

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (Metrics Server releases occasionally; v0.8.1 is stable)
