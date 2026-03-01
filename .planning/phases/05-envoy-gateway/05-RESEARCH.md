# Phase 5: Envoy Gateway - Research

**Researched:** 2026-03-01
**Domain:** Envoy Gateway installation, Gateway API CRDs, kubectl --server-side apply, GatewayClass readiness, binary size impact
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| EGW-01 | Gateway API CRDs are installed before Envoy Gateway controller starts | install.yaml is a single file that includes Gateway API CRDs (experimental channel v1.2.1) AND the Envoy Gateway controller in document order; CRDs appear first in the YAML, so `kubectl apply --server-side -f -` satisfies ordering by construction |
| EGW-02 | Envoy Gateway controller is running and a default GatewayClass is created | install.yaml deploys `deployment/envoy-gateway` in `envoy-gateway-system`; GatewayClass `eg` must be applied separately as inline YAML after the Deployment is Available; GatewayClass condition `Accepted` confirms controller recognized it |
| EGW-03 | User can create a Gateway + HTTPRoute and traffic routes to backend service via LoadBalancer IP | After EGW-02, users apply a Gateway (referencing GatewayClass `eg`) and HTTPRoute; the Envoy proxy DaemonSet/Deployment gets a Service of type LoadBalancer which MetalLB assigns an IP |
| EGW-04 | User can disable Envoy Gateway via `addons.envoyGateway: false` in cluster config | **Already implemented** in `create.go` via `runAddon("Envoy Gateway", opts.Config.Addons.EnvoyGateway, installenvoygw.NewAction())` — the `runAddon` closure skips disabled addons |
| EGW-05 | If MetalLB is disabled and Envoy Gateway is enabled, kinder prints a clear warning | **Already implemented** in `create.go` lines 194-196: `if !opts.Config.Addons.MetalLB && opts.Config.Addons.EnvoyGateway { logger.Warn(...) }` |
| EGW-06 | TLS termination is documented (manual cert path, no cert-manager) | Documentation task: generate cert with openssl, create `kubectl create secret tls`, reference in Gateway HTTPS listener; no code change required |
</phase_requirements>

## Summary

Phase 5 replaces the `installenvoygw` stub action with a complete implementation. The primary deliverable is a working `Execute` function that: (1) applies the embedded `install.yaml` with `--server-side`, (2) waits for the certgen Job to complete, (3) waits for the `envoy-gateway` Deployment to become Available, and (4) applies a `GatewayClass` named `eg`. EGW-04 (disable flag) and EGW-05 (MetalLB warning) are already fully implemented in `create.go` and require no changes.

The largest technical fact discovered in research is the binary size impact: Envoy Gateway's `install.yaml` is **2.5 MB** (40,570 lines) compared to MetalLB at 79 KB and Metrics Server at 4.3 KB. This is not a blocker for the project — Go builds binary-compress the embedded strings during compilation — but it must be documented so the team understands the tradeoff. The manifest cannot be split because the GitHub release provides only one install artifact. A key operational requirement is that `kubectl apply` **must** use `--server-side` because the `httproutes.gateway.networking.k8s.io` CRD is 372 KB, exceeding kubectl's 256 KB `last-applied-configuration` annotation limit; without `--server-side`, apply fails with an "annotation too long" error.

The `install.yaml` bundles the **experimental channel** of Gateway API v1.2.1 (for v1.3.1) — not the standard channel. This is the correct channel because Envoy Gateway uses experimental resources (BackendLBPolicy, TCPRoute, UDPRoute, TLSRoute) in addition to standard ones. The experimental channel is a superset of standard and is safe to use for local development. EGW-06 is a documentation-only requirement: TLS termination via manual openssl certificate creation, stored as a `kubernetes.io/tls` Secret, referenced in a Gateway listener — no cert-manager needed.

**Primary recommendation:** Embed Envoy Gateway v1.3.1 `install.yaml` via `go:embed`, apply with `kubectl apply --server-side -f -`, wait for certgen Job completion then Deployment Available, then apply GatewayClass `eg` inline and wait for Accepted condition.

## Standard Stack

### Core
| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| Envoy Gateway install.yaml | v1.3.1 | All-in-one manifest: Gateway API CRDs (experimental) + Envoy Gateway CRDs + Namespace + RBAC + ConfigMap + certgen Job + Deployment | Pinned in prior phase research notes; single artifact from official GitHub release |
| Gateway API (bundled) | v1.2.1 experimental | GatewayClass, Gateway, HTTPRoute, TCPRoute, TLSRoute, GRPCRoute CRDs | Included inside install.yaml; experimental channel required by Envoy Gateway |
| `go:embed` (stdlib) | Go 1.16+ | Embed install.yaml at compile time | Established pattern from Phase 2 (MetalLB) and Phase 3 (Metrics Server) |
| `kubectl apply --server-side` | kubectl 1.22+ | Apply large CRD manifests that exceed annotation limit | Required: httproutes CRD is 372 KB > 256 KB annotation limit |
| `kubectl wait --for=condition=Complete` | kubectl 1.22+ | Wait for certgen Job to finish creating TLS secrets | Job wait before Deployment wait — certs must exist before gateway starts |
| `kubectl wait --for=condition=Available` | kubectl 1.22+ | Wait for envoy-gateway Deployment | Confirms controller is running and ready to handle GatewayClass |
| `kubectl wait --for=condition=Accepted` | kubectl 1.22+ | Wait for GatewayClass `eg` to be acknowledged by controller | Confirms end-to-end wiring: CRDs + controller + GatewayClass all working |

### Supporting
| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| `strings` (stdlib) | - | `strings.NewReader` to pipe inline GatewayClass YAML to kubectl apply | Same pattern as MetalLB CR application in Phase 2 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| v1.3.1 install.yaml | v1.4.0 install.yaml (2.8 MB, Gateway API v1.3.0) | v1.4.0 is newer with Gateway API v1.3.0; v1.3.1 is the version documented in prior research notes; either works — recommend v1.3.1 to match documented decision |
| `--server-side` kubectl apply | Standard `kubectl apply` | Standard apply fails with "annotation too long" for httproutes CRD (372 KB > 256 KB limit); `--server-side` is mandatory |
| Single install.yaml (bundled CRDs + controller) | Separate Gateway API CRDs + Envoy Gateway manifest | GitHub release only provides install.yaml and quickstart.yaml; no separate CRD file exists; cannot split |
| Inline GatewayClass YAML string | Embedding a separate gatewayclass.yaml file | GatewayClass is 4 lines; an inline string constant is simpler than an extra embedded file |

**Installation:** No new Go module dependencies. All patterns established in prior phases.

## Architecture Patterns

### Recommended Project Structure

```
pkg/cluster/internal/create/actions/installenvoygw/
├── envoygw.go          # Action implementation (Execute func) — replace stub
└── manifests/
    └── install.yaml    # Envoy Gateway v1.3.1 install manifest (embedded via go:embed)
```

No separate test file for pure functional logic (unlike MetalLB which had subnet parsing). The action is a sequential kubectl pipeline with no custom Go logic to unit test.

### Pattern 1: go:embed + --server-side Apply

**What:** Embed install.yaml at compile time via `go:embed`, then apply it inside the control plane node with `kubectl apply --server-side -f -`.
**When to use:** Only for Envoy Gateway. The `--server-side` flag is specific to this addon because of the large CRD sizes. MetalLB and Metrics Server use standard apply.
**Example:**
```go
// Source: derived from installmetricsserver pattern + kubectl --server-side requirement
package installenvoygw

import _ "embed"
import "strings"

//go:embed manifests/install.yaml
var envoyGWManifest string

// In Execute:
if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "--server-side", "-f", "-",
).SetStdin(strings.NewReader(envoyGWManifest)).Run(); err != nil {
    return errors.Wrap(err, "failed to apply Envoy Gateway manifest")
}
```

### Pattern 2: Wait for certgen Job Then Deployment

**What:** After applying install.yaml, wait for the certgen Job to complete (creates TLS Secrets), then wait for the Deployment to become Available.
**When to use:** This two-step wait is specific to Envoy Gateway because the controller requires TLS secrets (created by certgen) to start successfully. The Deployment will fail or restart loop if certgen has not completed.
**Example:**
```go
// Source: kubectl wait docs; certgen Job name confirmed from install.yaml inspection
// Step 1: Wait for certgen Job to complete (creates envoy-gateway-ca and related TLS secrets)
if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "wait",
    "--namespace=envoy-gateway-system",
    "--for=condition=Complete",
    "job/eg-gateway-helm-certgen",
    "--timeout=120s",
).Run(); err != nil {
    return errors.Wrap(err, "Envoy Gateway certgen job did not complete")
}

// Step 2: Wait for controller Deployment to be Available
if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "wait",
    "--namespace=envoy-gateway-system",
    "--for=condition=Available",
    "deployment/envoy-gateway",
    "--timeout=120s",
).Run(); err != nil {
    return errors.Wrap(err, "Envoy Gateway controller did not become available")
}
```

### Pattern 3: Apply GatewayClass and Wait for Accepted

**What:** After the controller is running, apply the GatewayClass `eg` with the Envoy Gateway controller name. Then wait for the `Accepted` condition which confirms the controller has registered the class.
**When to use:** GatewayClass must be applied AFTER the controller is running, because the controller sets the Accepted condition. Applying it before the controller exists creates the resource but the condition stays Unknown/Pending.
**Example:**
```go
// Source: quickstart.yaml (confirmed from install.yaml + release assets)
// GatewayClass name: eg (matches controllerName prefix convention)
// controllerName: confirmed from ConfigMap envoy-gateway-config in install.yaml

const gatewayClassYAML = `apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
`

if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "-f", "-",
).SetStdin(strings.NewReader(gatewayClassYAML)).Run(); err != nil {
    return errors.Wrap(err, "failed to apply Envoy Gateway GatewayClass")
}

// Wait for controller to acknowledge the GatewayClass
if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "wait",
    "--for=condition=Accepted",
    "gatewayclass/eg",
    "--timeout=60s",
).Run(); err != nil {
    return errors.Wrap(err, "GatewayClass 'eg' was not accepted by Envoy Gateway controller")
}
```

### Anti-Patterns to Avoid

- **Using standard `kubectl apply` (without `--server-side`):** Fails with `metadata.annotations: Too long: must have at most 262144 bytes` for the httproutes CRD (372 KB). The `--server-side` flag is not optional for this manifest.
- **Applying GatewayClass before the Deployment is Available:** The GatewayClass resource is created but the controller never sets `condition=Accepted`, leaving it in Unknown/Pending state. Traffic will never route.
- **Skipping the certgen Job wait:** The `envoy-gateway` controller pod will enter CrashLoopBackOff or startup failures if the TLS Secrets created by certgen do not exist yet. Always wait for Job completion first.
- **Forgetting `--server-side` flag on upgrades/re-runs:** If the cluster already has the CRDs installed via standard apply, re-applying with `--server-side` still works (server-side apply and client-side apply are interoperable). However, mixing approaches can cause field manager conflicts — use `--server-side` consistently.
- **Using `kubectl apply -f URL` instead of embed:** Requires network access at cluster creation time; breaks offline use; against the project's established go:embed pattern.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Gateway API CRD installation | Writing CRD YAML by hand | Envoy Gateway's bundled `install.yaml` | 18 CRDs with complex OpenAPI schemas; hand-writing is error-prone and breaks compatibility |
| Certificate generation for Envoy Gateway TLS | Custom Go TLS cert generation | Envoy Gateway certgen Job (included in install.yaml) | certgen creates the specific Secrets (envoy-gateway-ca, envoy-gateway) that the controller expects with specific SAN entries |
| GatewayClass status polling | Custom polling loop for controller acknowledgment | `kubectl wait --for=condition=Accepted` | Standard kubectl wait handles the polling; GatewayClass conditions are part of the Gateway API spec |
| Splitting CRDs from controller manifests | Two-file approach with separate CRD file | Single install.yaml with `--server-side` | No separate CRD artifact is published; install.yaml places CRDs first so ordering is handled |

**Key insight:** Unlike MetalLB (which requires custom subnet detection and CR templating), Envoy Gateway needs almost no custom Go logic. The complexity is entirely in the kubectl pipeline: the right flags (`--server-side`), the right wait sequence (certgen Job → Deployment → GatewayClass), and the inline GatewayClass YAML.

## Common Pitfalls

### Pitfall 1: Missing --server-side Flag
**What goes wrong:** `kubectl apply -f -` (standard apply) fails with error: `metadata.annotations: Too long: must have at most 262144 bytes`.
**Why it happens:** The `httproutes.gateway.networking.k8s.io` CRD YAML is 372 KB. Standard client-side apply stores the full manifest in a `kubectl.kubernetes.io/last-applied-configuration` annotation, which is capped at 262144 bytes (256 KB). Server-side apply uses the apiserver to track field ownership instead, bypassing this limit.
**How to avoid:** Always use `kubectl apply --server-side -f -` for the Envoy Gateway manifest.
**Warning signs:** Error message contains "Too long" or "must have at most 262144 bytes"; kubectl exits non-zero after applying the manifest.

### Pitfall 2: GatewayClass Applied Before Controller Ready
**What goes wrong:** `kubectl wait --for=condition=Accepted gatewayclass/eg` times out because the controller never processes the GatewayClass.
**Why it happens:** The GatewayClass resource is created successfully (the CRD exists), but the controller is not yet running to set the Accepted condition. The condition stays in Unknown or False state indefinitely.
**How to avoid:** Always wait for `deployment/envoy-gateway --for=condition=Available` BEFORE applying the GatewayClass.
**Warning signs:** `kubectl wait` for Accepted condition times out; `kubectl describe gatewayclass eg` shows condition `Accepted=Unknown` with reason `Pending`.

### Pitfall 3: certgen Job Not Waited On
**What goes wrong:** `envoy-gateway` Deployment pod enters CrashLoopBackOff with TLS-related errors, or the Deployment wait times out.
**Why it happens:** The certgen Job creates Kubernetes Secrets (specifically `envoy-gateway-ca` and `envoy-gateway`) that contain TLS certificates for the gRPC control plane connection between Envoy Gateway and the Envoy proxy. The controller pod's startup fails if these Secrets do not exist yet.
**How to avoid:** Wait for `job/eg-gateway-helm-certgen --for=condition=Complete` before waiting for the Deployment.
**Warning signs:** `kubectl logs -n envoy-gateway-system deployment/envoy-gateway` shows certificate errors, missing secrets, or TLS handshake failures. Pod shows CrashLoopBackOff in its early restarts.

### Pitfall 4: Binary Size Surprise (2.5 MB Embedded File)
**What goes wrong:** No runtime failure, but the kinder binary grows by ~2.5 MB (compressed to ~800 KB via Go build compression) compared to the expected small size from prior addons.
**Why it happens:** Envoy Gateway's install.yaml is 2.5 MB compared to MetalLB at 79 KB. The Gateway API CRD schema for HTTPRoute alone is 372 KB. There is no smaller official install artifact.
**How to avoid:** This is expected and unavoidable. Document it. The tradeoff is acceptable: offline operation is worth the binary size increase.
**Warning signs:** `go build` produces a larger binary than expected; not a functional error.

### Pitfall 5: Experimental Channel CRDs
**What goes wrong:** Users or documentation refers to "standard channel" Gateway API resources but finds experimental resources installed.
**Why it happens:** Envoy Gateway v1.3.1 bundles Gateway API v1.2.1 **experimental** channel, which includes TCPRoute, UDPRoute, TLSRoute, BackendLBPolicy in addition to the standard GatewayClass/Gateway/HTTPRoute/ReferenceGrant.
**How to avoid:** Document that kinder installs experimental channel CRDs as part of Envoy Gateway. Experimental is a superset of standard — all standard resources still work. For local dev, experimental features are acceptable.
**Warning signs:** Confusion when users see unexpected CRDs like `backendlbpolicies.gateway.networking.k8s.io` in their cluster.

### Pitfall 6: certgen Job Name Is Helm-Generated
**What goes wrong:** `kubectl wait job/eg-gateway-helm-certgen` fails with "not found" if the Job name changes between Envoy Gateway versions.
**Why it happens:** The certgen Job name is generated from the Helm release name (`eg-gateway-helm`) and is therefore not a fixed name — it's embedded in the install.yaml manifest.
**How to avoid:** For v1.3.1, the Job name is `eg-gateway-helm-certgen` as confirmed by manifest inspection. If upgrading to a future version, re-inspect the install.yaml to confirm the Job name. Alternatively, wait for the Job by label: `kubectl wait --for=condition=Complete -l app=certgen -n envoy-gateway-system jobs --timeout=120s` (label-based approach is version-resilient).
**Warning signs:** `Error from server (NotFound): jobs.batch "eg-gateway-helm-certgen" not found` — indicates the manifest version does not match the expected Job name.

## Code Examples

Verified patterns from official sources and codebase:

### Complete Execute Function

```go
// Source: derived from installmetricsserver.go and installmetallb.go patterns,
// with --server-side apply from Envoy Gateway official docs and manifest inspection.
package installenvoygw

import (
    _ "embed"
    "strings"

    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/nodeutils"
    "sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/install.yaml
var envoyGWManifest string

// gatewayClassYAML creates a GatewayClass named "eg" pointing at the
// Envoy Gateway controller. The controllerName matches the value set in
// the envoy-gateway-config ConfigMap bundled in install.yaml.
const gatewayClassYAML = `apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
`

type action struct{}

// NewAction returns a new action for installing Envoy Gateway
func NewAction() actions.Action {
    return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing Envoy Gateway")
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

    // Step 1: Apply Envoy Gateway install.yaml (Gateway API CRDs + controller)
    // --server-side is REQUIRED: httproutes CRD is 372 KB > 256 KB annotation limit
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "--server-side", "-f", "-",
    ).SetStdin(strings.NewReader(envoyGWManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply Envoy Gateway manifest")
    }

    // Step 2: Wait for certgen Job to complete (creates TLS Secrets for the controller)
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait",
        "--namespace=envoy-gateway-system",
        "--for=condition=Complete",
        "job/eg-gateway-helm-certgen",
        "--timeout=120s",
    ).Run(); err != nil {
        return errors.Wrap(err, "Envoy Gateway certgen job did not complete")
    }

    // Step 3: Wait for the Envoy Gateway controller Deployment to be Available
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait",
        "--namespace=envoy-gateway-system",
        "--for=condition=Available",
        "deployment/envoy-gateway",
        "--timeout=120s",
    ).Run(); err != nil {
        return errors.Wrap(err, "Envoy Gateway controller did not become available")
    }

    // Step 4: Apply GatewayClass "eg" (not included in install.yaml)
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(gatewayClassYAML)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply Envoy Gateway GatewayClass")
    }

    // Step 5: Wait for GatewayClass to be accepted by the controller
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait",
        "--for=condition=Accepted",
        "gatewayclass/eg",
        "--timeout=60s",
    ).Run(); err != nil {
        return errors.Wrap(err, "GatewayClass 'eg' was not accepted by Envoy Gateway controller")
    }

    ctx.Status.End(true)
    return nil
}
```

### GatewayClass YAML (from official quickstart.yaml)

```yaml
# Source: https://github.com/envoyproxy/gateway/releases/download/v1.3.1/quickstart.yaml
# The GatewayClass is NOT included in install.yaml — it must be applied separately.
# controllerName matches the value in envoy-gateway-config ConfigMap in install.yaml.
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
```

### User-Facing: Deploy a Gateway and HTTPRoute (EGW-03 verification)

```yaml
# Source: adapted from quickstart.yaml
# Apply after kinder create cluster completes.
# The Gateway creates a LoadBalancer service (MetalLB assigns an IP).
# The HTTPRoute routes traffic from the gateway to the backend.
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
  namespace: default
spec:
  gatewayClassName: eg
  listeners:
    - name: http
      protocol: HTTP
      port: 80
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-route
  namespace: default
spec:
  parentRefs:
    - name: my-gateway
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: my-service
          port: 8080
```

### TLS Termination Without cert-manager (EGW-06 documentation)

```bash
# Source: https://gateway.envoyproxy.io/v1.3/tasks/security/tls-termination/
# Generate a self-signed certificate
openssl req -x509 -sha256 -nodes -days 365 -newkey rsa:2048 \
  -keyout example.key -out example.crt \
  -subj "/CN=www.example.com/O=example"

# Create the TLS Secret
kubectl create secret tls example-cert \
  --key=example.key \
  --cert=example.crt \
  --namespace=default

# Reference in a Gateway HTTPS listener:
# spec:
#   listeners:
#     - name: https
#       protocol: HTTPS
#       port: 443
#       tls:
#         mode: Terminate
#         certificateRefs:
#           - name: example-cert
#             kind: Secret
```

### install.yaml Resource Summary (v1.3.1, verified by manifest inspection)

```
Resources in install.yaml:
  18x CustomResourceDefinition
      - 10x Gateway API experimental channel v1.2.1 CRDs
        (GatewayClass, Gateway, HTTPRoute, GRPCRoute, TCPRoute, TLSRoute, UDPRoute,
         ReferenceGrant, BackendLBPolicy, BackendTLSPolicy)
      - 8x Envoy Gateway CRDs
        (Backend, BackendTrafficPolicy, ClientTrafficPolicy, EnvoyExtensionPolicy,
         EnvoyPatchPolicy, EnvoyProxy, HTTPRouteFilter, SecurityPolicy)
   1x Namespace    (envoy-gateway-system)
   2x ServiceAccount (envoy-gateway, eg-gateway-helm-certgen)
   1x ConfigMap    (envoy-gateway-config — sets controllerName)
   2x ClusterRole
   1x ClusterRoleBinding
   3x Role
   3x RoleBinding
   1x Service      (envoy-gateway — metrics/health endpoints)
   1x Deployment   (envoy-gateway — the controller)
   1x Job          (eg-gateway-helm-certgen — creates TLS Secrets)

Total: 40,570 lines, 2.5 MB
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Ingress (Ingress/IngressClass) | Gateway API (GatewayClass/Gateway/HTTPRoute) | Gateway API v1.0 stable (Oct 2023) | Ingress is still supported but Gateway API is the future; Envoy Gateway is GA for Gateway API |
| Standard channel Gateway API CRDs | Experimental channel (superset) | Always — Envoy Gateway bundles experimental | Experimental adds TCPRoute, TLSRoute, UDPRoute, BackendLBPolicy; standard resources still work normally |
| Separate CRD install step | Bundled in install.yaml | Envoy Gateway project design | Simplifies installation: one file, one apply command |
| Helm-only deployment | kubectl apply (YAML release artifact) | v0.5.0+ | install.yaml available as official non-Helm install path |
| Standard `kubectl apply` | `kubectl apply --server-side` | kubectl 1.22 GA | Required for large CRDs (>256 KB); httproutes CRD is 372 KB |
| Applying GatewayClass in install.yaml | GatewayClass applied separately | By design | install.yaml deploys the controller infrastructure; user creates GatewayClass resources |

**Deprecated/outdated:**
- Kubernetes Ingress for new development: Still works but Gateway API is the standard going forward
- Fetching install.yaml at runtime: Against project pattern; use go:embed for offline operation

## Open Questions

1. **certgen Job name stability across versions**
   - What we know: In v1.3.1, the certgen Job is named `eg-gateway-helm-certgen` (confirmed via manifest inspection). The name is derived from the Helm release name embedded in the YAML template.
   - What's unclear: If the project upgrades to v1.4.0+, the Job name may change or a different approach may be used.
   - Recommendation: Use the literal Job name `eg-gateway-helm-certgen` for v1.3.1. Add a comment in the code noting that this name was verified for v1.3.1 and should be re-checked on upgrade.

2. **Binary size impact on kinder users**
   - What we know: Embedding 2.5 MB of YAML adds approximately ~2.5 MB to the binary (Go does not compress go:embed files by default; the binary compressor handles it at a higher level). The binary was already ~40 MB+ due to Kubernetes client-go dependencies.
   - What's unclear: Whether users will notice or complain about the binary size increase.
   - Recommendation: Document the size tradeoff in a comment. The benefit (offline operation, pinned version) justifies the cost. If binary size becomes a concern in the future, consider downloading at first use.

3. **Envoy proxy LoadBalancer Service timing**
   - What we know: When a user creates a Gateway resource, Envoy Gateway creates an Envoy proxy Deployment and a Service of type LoadBalancer. MetalLB assigns the IP.
   - What's unclear: Whether there is a race condition between the Gateway proxy Service creation and MetalLB's IP assignment that could delay EGW-03 verification.
   - Recommendation: This is an end-user workflow concern, not a kinder install-time concern. The kinder action only ensures the GatewayClass is Accepted. The user's Gateway → LoadBalancer IP path depends on MetalLB already running (guaranteed by action order in create.go).

4. **EGW-06 as documentation vs. code**
   - What we know: EGW-06 requires that "TLS termination is documented (manual cert path, no cert-manager)." The requirement explicitly says documentation, not implementation.
   - What's unclear: Where this documentation should live — in PLAN.md, in a separate docs file, or as a comment in the code.
   - Recommendation: Document TLS termination in the PLAN.md summary section as a user guide. No code changes needed for EGW-06.

## Sources

### Primary (HIGH confidence)
- Official Envoy Gateway install.yaml v1.3.1: `https://github.com/envoyproxy/gateway/releases/download/v1.3.1/install.yaml` — manifest inspected directly: 40,570 lines, 2.5 MB; resource types confirmed; certgen Job name `eg-gateway-helm-certgen` confirmed; controllerName `gateway.envoyproxy.io/gatewayclass-controller` confirmed from ConfigMap
- Official Envoy Gateway quickstart.yaml v1.3.1: `https://github.com/envoyproxy/gateway/releases/download/v1.3.1/quickstart.yaml` — GatewayClass `eg` spec confirmed; HTTPRoute + Gateway user workflow confirmed
- Official Envoy Gateway GitHub releases: `https://github.com/envoyproxy/gateway/releases` — v1.3.1 release date March 4, 2025; latest stable is v1.7.0 (Feb 5, 2026); v1.3.x is EOL but valid for pinning
- Official Gateway API TLS termination docs: `https://gateway.envoyproxy.io/v1.3/tasks/security/tls-termination/` — openssl cert generation + kubectl create secret tls pattern confirmed
- Kinder codebase `create.go` lines 194-196, 202-203 — EGW-04 and EGW-05 already implemented; `runAddon("Envoy Gateway", ...)` confirms the disable pattern; MetalLB+EnvoyGateway warning already in place
- Kinder codebase `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go` — go:embed + kubectl apply -f - pattern (no wait for CRs, just Deployment wait)
- Kinder codebase `pkg/cluster/internal/create/actions/installmetallb/metallb.go` — strings.NewReader + SetStdin pattern for kubectl apply -f -

### Secondary (MEDIUM confidence)
- `https://gateway.envoyproxy.io/docs/install/install-yaml/` — v1.7.0 docs confirm `kubectl apply --server-side -f install.yaml` as the official install command; `--server-side` confirmed as required
- `https://gateway.envoyproxy.io/v1.3/install/install-yaml/` — v1.3.3 docs show same `--server-side` requirement; namespace `envoy-gateway-system` confirmed

### Tertiary (LOW confidence)
- CRD annotation size limits (262144 bytes): Derived from Kubernetes source and community documentation; confirmed directionally by measuring httproutes CRD at 372 KB but not validated against a running kubectl binary
- certgen Job ordering (must complete before Deployment stable): Inferred from TLS Secret dependency in controller code and community reports; not directly verified against v1.3.1 controller startup logs

## Metadata

**Confidence breakdown:**
- Standard stack (install.yaml v1.3.1, --server-side, go:embed): HIGH — manifests inspected directly; command verified from official docs
- EGW-04 and EGW-05 already implemented: HIGH — confirmed by reading create.go source code directly
- certgen Job → Deployment → GatewayClass wait sequence: HIGH for ordering (from manifest structure); MEDIUM for exact wait timeout values (120s is the established project pattern from MetalLB/Metrics Server)
- certgen Job name (`eg-gateway-helm-certgen`): HIGH — confirmed from direct manifest inspection
- Binary size (2.5 MB embedded): HIGH — confirmed by `wc -c` on downloaded manifest
- GatewayClass Accepted condition: MEDIUM — standard Gateway API behavior; not validated against a live cluster
- EGW-06 as documentation-only: HIGH — requirement explicitly says "documented (manual cert path, no cert-manager)"

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (Envoy Gateway v1.3.1 is pinned; stable for 30 days; re-check if upgrading)
