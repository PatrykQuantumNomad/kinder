# Phase 6: Dashboard - Research

**Researched:** 2026-03-01
**Domain:** Headlamp Kubernetes dashboard — manifest embedding, RBAC, long-lived service account token, output printing pattern
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DASH-01 | Headlamp is installed and running in `kube-system` namespace by default | Upstream `kubernetes-headlamp.yaml` deploys to `kube-system`; pinned image is `ghcr.io/headlamp-k8s/headlamp:v0.40.1`; container port 4466, service port 80; kinder must build a complete custom manifest (SA + RBAC + Deployment + Service + Secret) because upstream manifest omits ServiceAccount and ClusterRoleBinding |
| DASH-02 | A dedicated `kinder-dashboard` service account with cluster-admin role is created | Standard Kubernetes RBAC pattern: ServiceAccount + ClusterRoleBinding to `cluster-admin`; created in the embedded manifest alongside the Deployment, so it is deleted when Headlamp is uninstalled |
| DASH-03 | Service account token is printed at the end of `kinder create cluster` output | Long-lived token via `kubernetes.io/service-account-token` Secret; token is base64-encoded in `.data.token`; read with `kubectl get secret -o jsonpath='{.data.token}'` + `SetStdout(&buf)` + Go `encoding/base64.StdEncoding.DecodeString()` inside action; printed with `ctx.Logger.V(0).Infof()` after `ctx.Status.End(true)` |
| DASH-04 | Port-forward command is printed so user can access the dashboard immediately | `kubectl port-forward -n kube-system service/headlamp 8080:80` printed as a literal string alongside the token; user accesses `http://localhost:8080` and pastes the token |
| DASH-05 | User can view pods, services, deployments, and logs in the Headlamp UI | Headlamp with `-in-cluster` arg + cluster-admin token satisfies this natively; no extra config needed |
| DASH-06 | User can disable Dashboard via `addons.dashboard: false` in cluster config | Already wired in `create.go` line 203: `runAddon("Dashboard", opts.Config.Addons.Dashboard, installdashboard.NewAction())` — the `runAddon` closure skips disabled addons |
</phase_requirements>

## Summary

Phase 6 replaces the `installdashboard` stub action with a complete implementation. The deliverable is a working `Execute` function that: (1) applies a complete embedded Headlamp manifest (Service, Deployment, ServiceAccount, ClusterRoleBinding, and long-lived token Secret) using standard `kubectl apply`, (2) waits for the Headlamp Deployment to become Available, (3) reads the service account token from the Secret, decodes it in Go using `encoding/base64`, and (4) prints the decoded token and the exact `kubectl port-forward` command using `ctx.Logger.V(0).Infof()`.

The most important discovery in this research is that the upstream `kubernetes-headlamp.yaml` manifest (at `v0.40.1` tag) is **incomplete** — it includes the Deployment, Service, and a Secret template, but omits the ServiceAccount, ClusterRole reference, and ClusterRoleBinding. Additionally the upstream manifest pins to `ghcr.io/headlamp-k8s/headlamp:latest` instead of a versioned tag. Kinder must therefore supply a self-contained custom manifest that pins the image to `ghcr.io/headlamp-k8s/headlamp:v0.40.1` and includes all required RBAC and token resources. This manifest is embedded via `go:embed` following the pattern established in Phase 2 (MetalLB), Phase 3 (Metrics Server), and Phase 5 (Envoy Gateway).

The token retrieval pattern avoids shell tools entirely: `kubectl get secret` with `-o jsonpath='{.data.token}'` captures the base64-encoded token value into a `bytes.Buffer` via `SetStdout`, then Go's `encoding/base64.StdEncoding.DecodeString()` decodes it in-process. This is more portable than calling `base64 -d` or `base64 --decode` inside the node container (which differs between Debian-based containers and BusyBox-based environments). DASH-06 (disable flag) is **already implemented** in `create.go` line 203 and requires no changes.

**Primary recommendation:** Write a complete self-contained `headlamp.yaml` manifest, embed it via `go:embed`, apply with standard `kubectl apply -f -`, wait for Deployment Available, read and Go-decode the token, then print token + port-forward command via `ctx.Logger.V(0).Infof()` after `ctx.Status.End(true)`.

## Standard Stack

### Core
| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| Headlamp container image | v0.40.1 | Kubernetes web UI deployed in-cluster | Pinned in prior phase research notes; `ghcr.io/headlamp-k8s/headlamp:v0.40.1` confirmed as official image on GitHub Container Registry |
| `go:embed` (stdlib) | Go 1.16+ | Embed `headlamp.yaml` at compile time | Established pattern from Phase 2 (MetalLB), Phase 3 (Metrics Server), Phase 5 (Envoy Gateway); enables offline operation |
| `encoding/base64` (stdlib) | Go 1.0+ | Decode base64-encoded token from Secret `.data.token` | Avoids shell tool compatibility issue (`base64 -d` vs `base64 --decode`) in node containers; already imported in kindnet, standard Go idiom |
| `kubectl apply -f -` | kubectl 1.22+ | Apply Headlamp manifest — standard apply (not server-side) | Manifest is < 10 KB, no CRDs with large schemas, no 256 KB annotation limit concern; same pattern as MetalLB CR and Metrics Server |
| `kubectl wait --for=condition=Available` | kubectl 1.22+ | Wait for Headlamp Deployment readiness | Established pattern from Phase 2 (MetalLB), Phase 3 (Metrics Server), Phase 5 (Envoy Gateway) |
| `ctx.Logger.V(0).Infof()` | kind log interface | Print token and port-forward command to stdout | V(0) is "normal user-facing messages"; already used in `waitforready.go`; must be called AFTER `ctx.Status.End(true)` so spinner is stopped |

### Supporting
| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| `bytes.Buffer` (stdlib) | - | Capture `kubectl get secret` stdout for token reading | Same pattern as CoreDNS action reading Corefile with `SetStdout(&buf)` |
| `strings` (stdlib) | - | `strings.TrimSpace()` to strip trailing newline from base64 output before decoding | Defensive; kubectl jsonpath output may include trailing newline |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Pinned `v0.40.1` image | `latest` tag | `latest` is what upstream manifest uses; not reproducible; breaks go:embed offline guarantee if the node image doesn't preload it |
| Self-contained kinder manifest | Upstream `kubernetes-headlamp.yaml` | Upstream manifest lacks ServiceAccount and ClusterRoleBinding; cannot be used as-is |
| Go `encoding/base64` decode | Shell `base64 -d` inside node container | Shell variant differs between Debian (`--decode`) and BusyBox/Alpine (`-d`); Go decode is portable and eliminates shell dependency |
| Long-lived Secret token | `kubectl create token headlamp-admin -n kube-system` | `kubectl create token` tokens expire (default 1 hour); long-lived `kubernetes.io/service-account-token` Secret tokens never expire, which is appropriate for local development convenience |
| Print token in action | Return token string to create.go | The Action interface returns only `error`; modifying the interface is unnecessary complexity; `ctx.Logger.V(0).Infof()` from within the action is the established pattern (see `waitforready.go`) |

**Installation:** No new Go module dependencies. `encoding/base64` is stdlib. All patterns established in prior phases.

## Architecture Patterns

### Recommended Project Structure

```
pkg/cluster/internal/create/actions/installdashboard/
├── dashboard.go          # Action implementation (Execute func) — replace stub
└── manifests/
    └── headlamp.yaml     # Complete Headlamp manifest (embedded via go:embed)
```

No separate test file — the action is a sequential kubectl pipeline with no custom Go logic to unit test (unlike MetalLB which had subnet parsing logic).

### Pattern 1: Self-Contained Manifest with All RBAC

**What:** A single embedded `headlamp.yaml` contains ServiceAccount, ClusterRoleBinding, Service, Deployment, and the long-lived token Secret in one file. Applied with one `kubectl apply -f -` call.

**When to use:** When the upstream manifest is incomplete. Kinder owns the complete resource definition so no post-apply steps are needed to create RBAC separately.

**Example:**
```yaml
# Source: custom kinder manifest for Headlamp v0.40.1
# headlamp.yaml — all resources in dependency order
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kinder-dashboard
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kinder-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: kinder-dashboard
    namespace: kube-system
---
apiVersion: v1
kind: Secret
metadata:
  name: kinder-dashboard-token
  namespace: kube-system
  annotations:
    kubernetes.io/service-account.name: kinder-dashboard
type: kubernetes.io/service-account-token
---
apiVersion: v1
kind: Service
metadata:
  name: headlamp
  namespace: kube-system
spec:
  ports:
    - port: 80
      targetPort: 4466
  selector:
    k8s-app: headlamp
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: headlamp
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      k8s-app: headlamp
  template:
    metadata:
      labels:
        k8s-app: headlamp
    spec:
      serviceAccountName: kinder-dashboard
      containers:
        - name: headlamp
          image: ghcr.io/headlamp-k8s/headlamp:v0.40.1
          args:
            - "-in-cluster"
            - "-plugins-dir=/headlamp/plugins"
          ports:
            - containerPort: 4466
              name: http
          readinessProbe:
            httpGet:
              scheme: HTTP
              path: /
              port: 4466
            initialDelaySeconds: 30
            timeoutSeconds: 30
          livenessProbe:
            httpGet:
              scheme: HTTP
              path: /
              port: 4466
            initialDelaySeconds: 30
            timeoutSeconds: 30
      nodeSelector:
        'kubernetes.io/os': linux
```

**Key decisions in this manifest:**
- `serviceAccountName: kinder-dashboard` on the pod spec gives Headlamp in-cluster API access
- OpenTelemetry env vars (`HEADLAMP_CONFIG_TRACING_ENABLED`, `HEADLAMP_CONFIG_OTLP_ENDPOINT`) are **removed** because there is no otel-collector in a kinder cluster — keeping them causes connection errors in logs
- Image pinned to `v0.40.1`, not `latest`
- No metrics port 9090 exposed in Service (not needed for kinder's use case)

### Pattern 2: Token Retrieval via SetStdout + Go base64

**What:** Run `kubectl get secret` with jsonpath output captured into a `bytes.Buffer` via `SetStdout`, then decode the base64 value in Go using `encoding/base64`. Print the decoded token with `ctx.Logger.V(0).Infof()`.

**When to use:** Any time a value must be read from a kubectl command and printed to the user. Avoids shell tools.

**Example:**
```go
// Source: CoreDNS action pattern (corednstuning.go SetStdout pattern) +
//         Kubernetes official docs for service-account-token Secret

import (
    "bytes"
    "encoding/base64"
    "strings"
)

// Read the long-lived token from the Secret
var tokenBuf bytes.Buffer
if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "get", "secret", "kinder-dashboard-token",
    "--namespace=kube-system",
    "-o", "jsonpath={.data.token}",
).SetStdout(&tokenBuf).Run(); err != nil {
    return errors.Wrap(err, "failed to read dashboard token from secret")
}

// Decode base64 in Go — avoids shell base64 compatibility issues
tokenBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(tokenBuf.String()))
if err != nil {
    return errors.Wrap(err, "failed to decode dashboard token")
}
token := string(tokenBytes)

// End the spinner before printing multi-line output
ctx.Status.End(true)

// Print token and port-forward command for the user
ctx.Logger.V(0).Info("")
ctx.Logger.V(0).Info("Dashboard access:")
ctx.Logger.V(0).Infof("  Token: %s", token)
ctx.Logger.V(0).Info("  Port-forward: kubectl port-forward -n kube-system service/headlamp 8080:80")
ctx.Logger.V(0).Info("  Then open: http://localhost:8080")
ctx.Logger.V(0).Info("")
```

**CRITICAL:** `ctx.Status.End(true)` MUST be called before printing the token. The status spinner uses `defer ctx.Status.End(false)` at the top of Execute, but for the dashboard action we need to call `ctx.Status.End(true)` explicitly before logging the token, then return nil WITHOUT the deferred End firing again (or ensure it is idempotent). Looking at `cli/status.go` line 76: `if s.status == "" { return }` — calling `End()` a second time after the status is cleared is a no-op. The deferred `End(false)` will therefore be harmless after the explicit `End(true)`.

**IMPORTANT token timing:** The `kubernetes.io/service-account-token` Secret's `.data.token` field is populated asynchronously by the token controller after the Secret is created. After `kubectl apply`, there may be a brief delay. Wait for the Deployment to be Available first (which takes ~30-60 seconds due to `initialDelaySeconds: 30` in the readiness probe) — by the time the Deployment is Available, the token will already be populated. If needed, a short `kubectl wait` on the Secret's token field can be added, but in practice the Deployment wait provides sufficient time.

### Pattern 3: Manifest Resource Order (Dependency Order)

**What:** Resources in `headlamp.yaml` must be ordered so that dependencies appear before dependents.

**Order:**
1. ServiceAccount (`kinder-dashboard`) — must exist before ClusterRoleBinding references it
2. ClusterRoleBinding (`kinder-dashboard`) — references ServiceAccount and ClusterRole (cluster-admin already exists)
3. Secret (`kinder-dashboard-token`) — annotated with service account name; token controller populates `.data.token`
4. Service (`headlamp`) — does not depend on SA/RBAC; could go anywhere but grouped here
5. Deployment (`headlamp`) — references `serviceAccountName: kinder-dashboard`; must come after SA

`kubectl apply -f -` applies resources in document order, so ordering the manifest correctly ensures no dependency errors.

### Anti-Patterns to Avoid

- **Using upstream `kubernetes-headlamp.yaml` directly:** Missing ServiceAccount, ClusterRole, ClusterRoleBinding — Headlamp pod starts but has no API access; UI shows empty/error state.
- **Using `latest` image tag:** Not reproducible; breaks pinned-version guarantee of go:embed pattern; may pull different versions at different times (though kind node images preload images at build time anyway).
- **Using `kubectl create token` for a short-lived token:** Default expiry is 1 hour; user gets a "token expired" error the next time they try to connect. Use `kubernetes.io/service-account-token` Secret for a non-expiring token.
- **Printing token before `ctx.Status.End(true)`:** The spinner may overwrite the token line in a terminal. Always call `End(true)` before any multi-line output.
- **Reading token before Deployment wait:** The Secret's `.data.token` field is populated by the token controller, which runs quickly but asynchronously. The Deployment wait (30-60s due to readiness probe) provides more than enough time. Reading the token immediately after `kubectl apply` risks an empty or missing token field.
- **Keeping OpenTelemetry env vars from upstream manifest:** `otel-collector:4317` will not resolve in a kinder cluster; Headlamp will log connection errors on every startup, creating confusing noise. Remove these env vars.
- **Adding a `namespace:` field to the ClusterRoleBinding subjects entry:** `ClusterRoleBinding.subjects[].namespace` IS required when `kind: ServiceAccount` is used. This is a common YAML mistake that causes silent RBAC failures.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Kubernetes RBAC for dashboard access | Custom permission system | `ClusterRoleBinding` to `cluster-admin` in manifest | RBAC is Kubernetes-native; local dev cluster, cluster-admin is appropriate; same approach as documented in Headlamp official install guide |
| Token generation | Custom token signing | `kubernetes.io/service-account-token` Secret type | Kubernetes token controller populates it automatically; tokens are correctly signed for the cluster |
| Token base64 decoding | Shell `base64 -d` or `base64 --decode` | Go `encoding/base64.StdEncoding.DecodeString()` | Shell variant differs across container images; Go stdlib decode is portable and correct |
| Port-forward helper | Custom proxy | Print the `kubectl port-forward` command | kubectl is already present in the user's PATH; no additional tooling needed for local dev |

**Key insight:** All complexity in this phase is in the manifest design (correct resource ordering, correct RBAC, stripped otel env vars) and the token-printing pattern. The Go code is a simple pipeline: apply, wait, read secret, decode, print. No custom algorithms needed.

## Common Pitfalls

### Pitfall 1: Incomplete Manifest (No ServiceAccount or RBAC)
**What goes wrong:** Headlamp pods start and the Service is available, but the Headlamp UI shows errors or empty resource lists. The token printed by kinder is valid, but Headlamp's in-cluster client has no permissions to call the Kubernetes API.
**Why it happens:** Upstream `kubernetes-headlamp.yaml` does not include ServiceAccount, ClusterRole, or ClusterRoleBinding. A standalone Deployment without a `serviceAccountName` uses the default ServiceAccount, which has no cluster permissions.
**How to avoid:** Use the kinder-authored complete manifest that includes all RBAC resources. Verify with `kubectl auth can-i list pods --as=system:serviceaccount:kube-system:kinder-dashboard` after apply.
**Warning signs:** Headlamp UI loads but shows "Forbidden" errors or empty lists for all resources; token is accepted but cluster resources appear empty.

### Pitfall 2: Token Not Yet Populated When Read
**What goes wrong:** `kubectl get secret kinder-dashboard-token -o jsonpath='{.data.token}'` returns an empty string; `base64.StdEncoding.DecodeString("")` succeeds but returns an empty token; user receives a blank token.
**Why it happens:** The `kubernetes.io/service-account-token` Secret is created by `kubectl apply`, but the Kubernetes token controller populates `.data.token` asynchronously. On a fast machine, reading immediately after `apply` may catch the Secret before population.
**How to avoid:** Always wait for the Headlamp Deployment to become Available (which takes ~30-60 seconds due to the 30s `initialDelaySeconds` on the readiness probe) before reading the token. The Deployment wait provides sufficient time for the token controller to populate the Secret. If an edge case is observed, add `kubectl wait --for=jsonpath='{.data.token}' secret/kinder-dashboard-token` as a safety net.
**Warning signs:** Printed token is empty string; user cannot log in to Headlamp.

### Pitfall 3: Spinner Not Ended Before Multi-Line Token Print
**What goes wrong:** On terminals with the spinner active, the `ctx.Logger.V(0).Infof("Token: %s", token)` line is visually overwritten or garbled by the spinner animation.
**Why it happens:** The `Status.Start("Installing Dashboard")` + `defer Status.End(false)` pattern keeps the spinner running until `End()` is called. If `Infof` runs while the spinner is active, the terminal cursor position conflicts with the spinner's in-place update.
**How to avoid:** Call `ctx.Status.End(true)` explicitly before printing the token and port-forward command. The deferred `End(false)` will be a no-op (status is already empty, the early-return guard at `cli/status.go:76` catches it).
**Warning signs:** Token output appears garbled or partially overwritten in interactive terminals.

### Pitfall 4: ClusterRoleBinding Missing `namespace` in subjects
**What goes wrong:** RBAC silently fails. `kubectl auth can-i` returns no and Headlamp shows Forbidden errors.
**Why it happens:** A ClusterRoleBinding for a ServiceAccount subject REQUIRES the `namespace` field. Without it, the binding references a non-existent service account.
**How to avoid:** Always include `namespace: kube-system` in the ClusterRoleBinding subjects entry for a ServiceAccount.
**Warning signs:** `kubectl auth can-i list pods --as=system:serviceaccount:kube-system:kinder-dashboard` returns "no".

### Pitfall 5: OpenTelemetry Env Vars in Manifest
**What goes wrong:** Headlamp pods log repeated connection errors to `otel-collector:4317` which does not exist in a kinder cluster. Logs are noisy and may confuse users diagnosing other issues.
**Why it happens:** The upstream `kubernetes-headlamp.yaml` includes `HEADLAMP_CONFIG_TRACING_ENABLED=true` and `HEADLAMP_CONFIG_OTLP_ENDPOINT=otel-collector:4317`. There is no OpenTelemetry collector in kinder clusters.
**How to avoid:** Omit all `HEADLAMP_CONFIG_TRACING_ENABLED`, `HEADLAMP_CONFIG_METRICS_ENABLED`, and `HEADLAMP_CONFIG_OTLP_ENDPOINT` env vars from the kinder manifest. Headlamp runs fine without telemetry.
**Warning signs:** `kubectl logs -n kube-system deployment/headlamp` shows repeated "connection refused" or "dial tcp" errors to port 4317.

### Pitfall 6: DASH-06 Already Implemented — No Changes to create.go Needed
**What goes wrong:** Developer duplicates the disable flag wiring that is already present.
**Why it happens:** DASH-06 says "User can disable Dashboard via `addons.dashboard: false`" — a developer might think this needs to be implemented.
**How to avoid:** `create.go` line 203 already has `runAddon("Dashboard", opts.Config.Addons.Dashboard, installdashboard.NewAction())`. The `runAddon` closure handles the disable flag. No changes to `create.go` are needed for Phase 6.
**Warning signs:** Duplicate disable-flag logic in dashboard.go or a second call to installdashboard.NewAction().

## Code Examples

Verified patterns from official sources and existing codebase:

### Complete Execute Function

```go
// Source: pattern derived from installmetricsserver.go (embed + apply + wait),
//         installcorednstuning.go (SetStdout + buffer read),
//         Kubernetes docs (kubernetes.io/service-account-token Secret type),
//         log/types.go (V(0).Infof for user-facing messages),
//         cli/status.go (End() idempotency guarantee).

package installdashboard

import (
    "bytes"
    "encoding/base64"
    _ "embed"
    "strings"

    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/nodeutils"
    "sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/headlamp.yaml
var headlampManifest string

type action struct{}

// NewAction returns a new action for installing the Dashboard
func NewAction() actions.Action {
    return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing Dashboard")
    defer ctx.Status.End(false) // no-op if End(true) was already called

    // Get control plane node
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

    // Step 1: Apply Headlamp manifest (SA, RBAC, Secret, Service, Deployment)
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(headlampManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply Headlamp manifest")
    }

    // Step 2: Wait for Headlamp Deployment to be Available
    // (30s initialDelaySeconds means this takes at least 30s; token Secret is
    // populated by token controller well before Deployment becomes Available)
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait",
        "--namespace=kube-system",
        "--for=condition=Available",
        "deployment/headlamp",
        "--timeout=120s",
    ).Run(); err != nil {
        return errors.Wrap(err, "Headlamp deployment did not become available")
    }

    // Step 3: Read the long-lived token from the Secret
    // The token is base64-encoded in .data.token; decode in Go to avoid
    // shell base64 compatibility issues (Debian vs BusyBox containers)
    var tokenBuf bytes.Buffer
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "get", "secret", "kinder-dashboard-token",
        "--namespace=kube-system",
        "-o", "jsonpath={.data.token}",
    ).SetStdout(&tokenBuf).Run(); err != nil {
        return errors.Wrap(err, "failed to read dashboard token from secret")
    }

    tokenBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(tokenBuf.String()))
    if err != nil {
        return errors.Wrap(err, "failed to decode dashboard token")
    }
    token := string(tokenBytes)

    // Step 4: End spinner before printing multi-line output (End is idempotent;
    // the deferred End(false) will be a no-op since status is already cleared)
    ctx.Status.End(true)

    // Step 5: Print token and port-forward command for the user
    ctx.Logger.V(0).Info("")
    ctx.Logger.V(0).Info("Dashboard:")
    ctx.Logger.V(0).Infof("  Token: %s", token)
    ctx.Logger.V(0).Info("  Port-forward: kubectl port-forward -n kube-system service/headlamp 8080:80")
    ctx.Logger.V(0).Info("  Then open: http://localhost:8080")
    ctx.Logger.V(0).Info("")

    return nil
}
```

### Complete headlamp.yaml Manifest

```yaml
# Source: https://raw.githubusercontent.com/kubernetes-sigs/headlamp/v0.40.1/kubernetes-headlamp.yaml
# (upstream manifest verified — used as base; kinder adds SA, RBAC, pins image, removes otel vars)
# Resources in dependency order: SA -> ClusterRoleBinding -> Secret -> Service -> Deployment
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kinder-dashboard
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kinder-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: kinder-dashboard
    namespace: kube-system   # REQUIRED for ServiceAccount subjects
---
apiVersion: v1
kind: Secret
metadata:
  name: kinder-dashboard-token
  namespace: kube-system
  annotations:
    kubernetes.io/service-account.name: kinder-dashboard
type: kubernetes.io/service-account-token
---
apiVersion: v1
kind: Service
metadata:
  name: headlamp
  namespace: kube-system
spec:
  ports:
    - port: 80
      targetPort: 4466
  selector:
    k8s-app: headlamp
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: headlamp
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      k8s-app: headlamp
  template:
    metadata:
      labels:
        k8s-app: headlamp
    spec:
      serviceAccountName: kinder-dashboard
      containers:
        - name: headlamp
          image: ghcr.io/headlamp-k8s/headlamp:v0.40.1
          args:
            - "-in-cluster"
            - "-plugins-dir=/headlamp/plugins"
          ports:
            - containerPort: 4466
              name: http
          readinessProbe:
            httpGet:
              scheme: HTTP
              path: /
              port: 4466
            initialDelaySeconds: 30
            timeoutSeconds: 30
          livenessProbe:
            httpGet:
              scheme: HTTP
              path: /
              port: 4466
            initialDelaySeconds: 30
            timeoutSeconds: 30
      nodeSelector:
        'kubernetes.io/os': linux
```

### User-Facing Output (DASH-03 + DASH-04)

Expected output at end of `kinder create cluster`:

```
 ✓ Installing Dashboard

Dashboard:
  Token: eyJhbGciOiJSUzI1NiIsImtpZCI6Ii4uLiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3Nlcn...
  Port-forward: kubectl port-forward -n kube-system service/headlamp 8080:80
  Then open: http://localhost:8080

Addons:
 * MetalLB             installed
 * Metrics Server      installed
 * CoreDNS Tuning      installed
 * Envoy Gateway       installed
 * Dashboard           installed
```

Note: The token and port-forward are printed during the Dashboard action's `Execute()` call, BEFORE the addon summary is printed by `logAddonSummary()` in `create.go`. The spinner ending mark (`✓ Installing Dashboard`) appears first, then the multi-line dashboard block, then the addon summary.

### RBAC Verification Command

```bash
# Source: kubectl auth can-i docs
# Verify the kinder-dashboard service account has cluster-admin access
kubectl auth can-i list pods \
  --as=system:serviceaccount:kube-system:kinder-dashboard \
  --all-namespaces
# Expected output: yes
```

### Token Retrieval Verification

```bash
# Source: Kubernetes docs - manually retrieve long-lived token
kubectl get secret kinder-dashboard-token -n kube-system \
  -o jsonpath='{.data.token}' | base64 --decode
# Expected: eyJhbGciOiJSUzI1NiIsImtpZCI6... (long JWT token)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| kubernetes/dashboard (official) | Headlamp (kubernetes-sigs) | Jan 2026: kubernetes/dashboard archived | kubernetes/dashboard no longer maintained; Headlamp is the active replacement under kubernetes-sigs |
| Auto-generated SA tokens (< k8s 1.24) | Explicit `kubernetes.io/service-account-token` Secret | k8s 1.24 (2022) | Must create explicit Secret to get long-lived token; auto-generation was removed |
| Short-lived tokens (`kubectl create token`) | Long-lived token Secret | k8s 1.24+ | `kubectl create token` tokens expire (default 1h); local dev needs persistent tokens |
| Helm chart for Headlamp | Static manifest via go:embed | kinder project decision (Phase 1) | Avoids Helm dependency; enables offline operation; consistent with all other kinder addons |
| `base64 --decode` (GNU coreutils) | Go `encoding/base64` | kinder project decision (Phase 6) | Shell base64 variant varies; Go decode is portable across container images |

**Deprecated/outdated:**
- `kubernetes-retired/dashboard`: Archived January 2026; no longer receives updates; requires Helm as a hard dependency
- Auto-generated ServiceAccount tokens (Kubernetes < 1.24): No longer supported; kind clusters use k8s 1.29+
- `kubectl create token` for persistent dashboard access: Tokens expire; use long-lived Secret type instead

## Open Questions

1. **Token printed before or after addon summary?**
   - What we know: The token is printed inside `Execute()` via `ctx.Logger.V(0).Infof()`. `logAddonSummary()` is called after all `runAddon()` calls in `create.go`. So the token prints BEFORE the addon summary.
   - What's unclear: Whether the interleaving of token output with the spinner output is visually clean on all terminals. The explicit `ctx.Status.End(true)` before printing mitigates this, but non-interactive (piped) terminals output everything linearly.
   - Recommendation: Accept the current behavior (token before summary). It is more prominent this way and users see it before the compact summary table.

2. **Headlamp readiness probe `initialDelaySeconds: 30`**
   - What we know: The upstream manifest uses `initialDelaySeconds: 30` for both readiness and liveness probes. This means the Deployment wait will take AT LEAST 30 seconds after pod start.
   - What's unclear: Whether reducing this value would speed up cluster creation. Headlamp is a React SPA served by a Go HTTP server — it starts quickly but 30s is the upstream default.
   - Recommendation: Keep the upstream `initialDelaySeconds: 30` to match what has been tested. The total dashboard install time (including image pull if needed) fits within the `--timeout=120s` wait.

3. **Image pre-loading in kind node image**
   - What we know: Kind node images do NOT preload Headlamp's container image (`ghcr.io/headlamp-k8s/headlamp:v0.40.1`). The image will be pulled from GHCR when the Headlamp pod first starts. This requires internet access during cluster creation.
   - What's unclear: Whether this breaks the "offline-capable" promise of go:embed. The manifest is embedded offline-capable; the container image requires network access.
   - Recommendation: This is a known limitation for Phase 6. The go:embed pattern ensures the manifest definition is reproducible; the container image pull is a separate concern (as it is for MetalLB and Metrics Server). Document this limitation. A future phase could explore preloading images into the kind node image.

4. **Headlamp v0.40.1 static manifest assets**
   - What we know: The v0.40.1 GitHub release page lists 13 assets but they are desktop application binaries (AppImage, DMG, EXE) and no Kubernetes YAML files. The only YAML is `kubernetes-headlamp.yaml` at the repo root, which is incomplete (verified via WebFetch of raw content).
   - What's unclear: Whether there is an official "kinder-compatible" static manifest elsewhere in the repo (e.g., `charts/` templates rendered to YAML).
   - Recommendation: Use the custom kinder-authored manifest defined in this research. It is correct, complete, and pinned to v0.40.1. The upstream manifest cannot be used as-is.

## Sources

### Primary (HIGH confidence)
- Official Headlamp GitHub releases: `https://github.com/kubernetes-sigs/headlamp/releases/tag/v0.40.1` — confirmed container image `ghcr.io/headlamp-k8s/headlamp:v0.40.1`; confirmed no Kubernetes YAML in release assets
- Official Headlamp `kubernetes-headlamp.yaml` at v0.40.1: `https://raw.githubusercontent.com/kubernetes-sigs/headlamp/v0.40.1/kubernetes-headlamp.yaml` — verified content: 3 resources (Service, Deployment, Secret); MISSING ServiceAccount and ClusterRoleBinding; image tag is `latest` (not pinned); otel env vars present
- Kubernetes docs — long-lived service account token: `https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#manually-create-an-api-token-for-a-serviceaccount` — confirmed `kubernetes.io/service-account-token` Secret type; confirmed `jsonpath='{.data.token}'` retrieval; confirmed token controller auto-populates the Secret
- Kinder codebase `pkg/cluster/internal/create/create.go` — DASH-06 already implemented at line 203; `logAddonSummary` called after all `runAddon` calls; `ctx.Logger.V(0).Infof()` pattern confirmed in `waitforready.go`
- Kinder codebase `pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go` — `SetStdout(&buf)` pattern for capturing kubectl output
- Kinder codebase `pkg/internal/cli/status.go` lines 74-90 — `End()` idempotency confirmed: `if s.status == "" { return }` guard means second call to `End()` is always a no-op
- Kinder codebase `pkg/log/types.go` — `V(0)` is "normal user facing messages"; `V(1+)` is debug; confirmed `ctx.Logger.V(0).Infof()` is the correct method for printing user-visible output

### Secondary (MEDIUM confidence)
- Official Headlamp in-cluster installation docs: `https://headlamp.dev/docs/latest/installation/in-cluster/` — confirmed port-forward command `kubectl port-forward -n kube-system service/headlamp 8080:80`; confirmed `kube-system` namespace; confirmed Service port 80 → container port 4466
- DEV Community "Headlamp on Kind" guide: `https://dev.to/deebi9/kubernetes-dashboard-headlamp-on-kind-3c2d` — step-by-step kubectl commands: SA creation, ClusterRoleBinding, `kubectl create token` (short-lived), port-forward pattern; confirms the workflow but uses short-lived tokens (kinder uses long-lived Secret instead)
- Kitemetric "Headlamp on Kind" guide: `https://kitemetric.com/blogs/headlamp-on-kind-your-kubernetes-dashboard` — additional confirmation of same workflow

### Tertiary (LOW confidence)
- `base64 -d` vs `base64 --decode` difference in Alpine vs Debian: WebSearch finding confirmed directionally; Go `encoding/base64` used to avoid the issue entirely (no shell invocation needed)
- Token controller population timing (< 30s in practice): Inferred from general Kubernetes architecture knowledge; not directly measured in a kind cluster

## Metadata

**Confidence breakdown:**
- Standard stack (Headlamp v0.40.1, go:embed, manifest resources): HIGH — image confirmed from official GitHub releases; manifest content verified by fetching raw URL; resource types confirmed from Kubernetes docs
- DASH-06 already implemented: HIGH — confirmed by reading create.go source directly
- Token retrieval pattern (SetStdout + encoding/base64): HIGH — SetStdout pattern confirmed from corednstuning.go; base64 stdlib is standard Go
- Spinner idempotency (deferred End no-op after explicit End): HIGH — confirmed by reading cli/status.go source directly
- OpenTelemetry env var removal: HIGH — no otel-collector exists in kinder clusters; removing vars is correct
- Container image pull requires network: MEDIUM — inferred from kind node image design; kind images do not preload addon images by default
- Token timing (populated before Deployment Available): MEDIUM — inferred from token controller architecture; Deployment wait takes ~30-60s which is more than enough time in practice

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (Headlamp v0.40.1 is pinned; stable for 30 days)
