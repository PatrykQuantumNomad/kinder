# Feature Landscape: Kinder Addons

**Domain:** Batteries-included local Kubernetes (kind fork with 5 default addons)
**Researched:** 2026-03-01
**Milestone scope:** Adding MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, and Kubernetes Dashboard as default addons to kinder.

---

## Addon 1: MetalLB (LoadBalancer IPs)

MetalLB (v0.15.3 current) gives LoadBalancer services real IPs in clusters that have no cloud provider. For kind clusters, it draws an IP range from the Docker network subnet automatically.

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `LoadBalancer` services get an IP immediately | Users expect `kubectl get svc` to show an EXTERNAL-IP, not `<pending>` | Low | Without MetalLB, every LoadBalancer service hangs in pending — this is the core problem |
| Layer 2 mode (ARP/NDP) by default | Simplest mode for local dev; no BGP router required | Low | Layer 2 is the only viable mode in kind — BGP requires a real routing infrastructure |
| IP pool auto-detected from Docker network | Users should not need to look up subnets manually | Medium | `docker network inspect kind --format '{{ (index .IPAM.Config 0).Subnet }}'` gives the CIDR; carve a /27 from the upper range to avoid conflicts |
| MetalLB ready before cluster reported ready | User runs `kinder create cluster` and gets a working cluster | Low | Apply manifests, wait for controller and speaker pods to be Running |
| Works with Podman and Nerdctl networks | kinder supports 3 providers | Medium | Network inspection command differs per runtime; Podman uses `podman network inspect`, Nerdctl uses CNI config; subnet extraction logic must branch per provider |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Deterministic IP range carved from runtime subnet | No manual config; no conflicts with node IPs | Medium | The auto-carve logic (take the subnet, replace last octet range to pick a safe /27 or /28) is what makes it truly zero-config |
| Opt-out per cluster via config | Power users who bring their own LB can skip MetalLB | Low | Add `addons.metalLB: false` to the kinder cluster config |
| Pre-configured `L2Advertisement` resource | Users do not need to know MetalLB CRD names | Low | Ship a ready-to-go `IPAddressPool` + `L2Advertisement` manifest pair |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| BGP mode | Requires FRR or external router; impossible to wire in kind without extra containers | Layer 2 only for v1 |
| Exposing MetalLB UI or Helm | Project constraint: no Helm dependency | Apply manifests directly from embedded YAML |
| Manual IP pool input from user | Defeats the batteries-included goal | Auto-detect from Docker/Podman/Nerdctl network |
| MetalLB Operator | Additional CRD surface area, not needed for simple L2 | Plain manifests are sufficient |

### User-Facing Behavior

User runs `kinder create cluster`. After the cluster is ready, they run `kubectl apply -f my-service.yaml` with `type: LoadBalancer`. Within a few seconds `kubectl get svc` shows an EXTERNAL-IP in the `172.x.x.x` range (or equivalent Docker subnet). That IP is reachable from the host machine via the Docker network interface.

### Required Configuration

```yaml
# kinder cluster config (example)
kind: Cluster
apiVersion: kinder.dev/v1alpha1
addons:
  metalLB: true   # default; set false to skip
```

MetalLB itself needs two CRs after installation:
```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kinder-pool
  namespace: metallb-system
spec:
  addresses:
  - <auto-detected-range>   # e.g. 172.19.0.200-172.19.0.250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: kinder-l2adv
  namespace: metallb-system
spec:
  ipAddressPools:
  - kinder-pool
```

---

## Addon 2: Envoy Gateway (Gateway API)

Envoy Gateway (v1.7.0 current) implements the Kubernetes Gateway API standard, replacing the older Ingress API with richer routing semantics: HTTPRoute, TLSRoute, TCPRoute, GRPCRoute, and UDPRoute.

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Gateway API CRDs installed | Without them Envoy Gateway cannot start; any Gateway API manifest the user applies will fail | Low | CRDs are bundled in the Envoy Gateway Helm chart as of v1.2+; apply them first in the action |
| `GatewayClass` resource created | Required entry point — without it no Gateway resources are reconciled | Low | Helm chart creates this automatically with `controllerName: gateway.envoyproxy.io/gatewayclass-controller` |
| Envoy Gateway controller running | Watches Gateway API CRDs and provisions Envoy proxy pods | Low | Wait for `envoy-gateway` deployment in `envoy-gateway-system` namespace to be Available |
| `HTTPRoute` traffic actually routable | End-to-end test: create a Gateway + HTTPRoute + backend Service; curl the LoadBalancer IP | Medium | This only works when MetalLB is also installed; the Envoy proxy Service needs a real EXTERNAL-IP |
| Gateway gets an IP (not stuck pending) | Same problem as any LoadBalancer service | Low | Envoy Gateway depends on MetalLB being up first; install MetalLB action must run before Envoy Gateway action |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| HTTP/HTTPS routing out of the box | Users can do `HTTPRoute` path-based and header-based routing immediately | Low | Gateway API HTTPRoute is the standard replacement for Ingress |
| TLS termination support (via TLSRoute / HTTPRoute with cert) | Covers the HTTPS use case that developers encounter early | Medium | Requires cert-manager or manual secret; do not install cert-manager by default, document the manual path |
| TCPRoute / UDPRoute for raw TCP/UDP | Covers database proxying and other non-HTTP workloads | Low | Supported out of the box in Envoy Gateway, just needs correct listener |
| Envoy Gateway's `EnvoyProxy` CRD extensions | Users can customize the data plane (timeouts, circuit breakers) | Low | Available automatically; don't need to configure for default install |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Defaulting to NodePort if MetalLB is absent | Silently broken behavior is worse than an explicit error | Enforce MetalLB-first install order; error clearly if MetalLB is disabled and Envoy Gateway is enabled |
| Pinning to ClusterIP service type | Gateway never gets an address; HTTPRoutes unreachable from host | Always use LoadBalancer type, relying on MetalLB |
| Installing cert-manager as a dependency | Scope creep, additional complexity | Document TLS as "bring your own cert"; v2 can add cert-manager addon |
| Installing Istio or other service meshes alongside | Conflict risk with Envoy proxy ports | Envoy Gateway stands alone; no service mesh in v1 |

### User-Facing Behavior

User creates a `Gateway` resource and an `HTTPRoute`. The Gateway proxy pod comes up, gets an EXTERNAL-IP from MetalLB, and `curl http://<EXTERNAL-IP>/my-path` routes to the correct backend service. The user does not need to install Gateway API CRDs manually, does not run `helm install`, and does not configure a GatewayClass controller name.

### Required Configuration

```yaml
# kinder cluster config
addons:
  envoyGateway: true   # default; requires metalLB: true
```

Minimal user resources after cluster creation:
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
spec:
  gatewayClassName: eg
  listeners:
  - name: http
    port: 80
    protocol: HTTP
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-route
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

---

## Addon 3: Metrics Server (kubectl top / HPA)

Metrics Server (v0.8.1, released January 2026) collects CPU and memory metrics from kubelets and exposes them via the Kubernetes Metrics API. It enables `kubectl top`, HPA, and VPA.

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| `kubectl top nodes` works | Basic cluster health check every developer runs | Low | Requires `--kubelet-insecure-tls` flag for kind clusters where kubelet certs are self-signed |
| `kubectl top pods` works | Debugging resource usage is a daily developer task | Low | Same flag requirement |
| HPA can read CPU/memory metrics | Users testing autoscaling need this; HPA silently fails without Metrics Server | Low | HPA controller polls the Metrics API; Metrics Server satisfies this API |
| Metrics available within 60 seconds of cluster ready | User shouldn't have to wait long after creating the cluster | Low | Metrics Server starts quickly; first scrape takes ~15s by default |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Sane default metric resolution (15s) | Balanced between freshness and kubelet load | Low | Default `--metric-resolution=15s` is appropriate for local dev |
| Pre-patched for kind's self-signed certs | Users don't hit the common "failing to scrape" error | Low | The `--kubelet-insecure-tls` flag is the only kind-specific change; ship it as default |
| Resource limits set conservatively | Keeps kind nodes from running out of memory | Low | 100m CPU / 200MiB memory is sufficient for clusters up to 100 nodes |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Prometheus / full monitoring stack | Completely different scope, much heavier | Metrics Server only; document Prometheus as "next step" in user docs |
| Custom metrics adapter (Prometheus adapter) | Allows HPA on arbitrary metrics, but adds significant complexity | Out of scope for v1; standard CPU/memory metrics cover 90% of local dev use cases |
| Vertical Pod Autoscaler (VPA) | Separate project, separate CRDs, rarely needed in local dev | Out of scope |
| Skipping `--kubelet-insecure-tls` | Will fail with TLS errors in kind because kubelet certs aren't cluster-CA-signed | Always apply this flag for kind clusters |

### User-Facing Behavior

User runs `kubectl top nodes` immediately after `kinder create cluster` completes. Within about 30 seconds they see CPU and memory usage per node. When they create an HPA resource, it works without any additional setup.

### Required Configuration

The only kind-specific configuration is a patch to the standard Metrics Server manifest:
```yaml
# Patch to metrics-server Deployment
containers:
- name: metrics-server
  args:
  - --cert-dir=/tmp
  - --secure-port=10250
  - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
  - --kubelet-use-node-status-port
  - --metric-resolution=15s
  - --kubelet-insecure-tls   # Required for kind — kubelet certs are self-signed
```

```yaml
# kinder cluster config
addons:
  metricsServer: true   # default; no additional options needed
```

---

## Addon 4: CoreDNS Tuning

kind already installs CoreDNS as part of the cluster. This addon is not about installing CoreDNS — it is about patching the default CoreDNS ConfigMap to apply settings that improve local development DNS behavior.

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Cluster DNS works (no regression) | CoreDNS is already installed by kind; tuning must not break it | Low | Patch, don't replace — use `kubectl patch configmap coredns -n kube-system` |
| External DNS queries succeed | Developers access external APIs from pods | Low | `forward . /etc/resolv.conf` must remain, pointing to the host's resolver |
| In-cluster service names resolve | `svc.cluster.local` FQDN resolution is the core use case | Low | The `kubernetes` plugin handles this; do not remove or reorder it |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| `autopath @kubernetes` plugin enabled | Reduces 5 DNS lookups per external query to 1-2 by doing server-side search path walking | Medium | Requires `pods verified` mode in the kubernetes plugin block; has a small CPU cost on CoreDNS pod |
| `cache` TTL increased to 60s (from 30s) | Reduces repeated external lookups in long-running dev sessions | Low | External queries are cached longer; in-cluster TTL stays at 5s via kubernetes plugin |
| `log` plugin enabled in development mode | Surfacing DNS query logs helps developers debug DNS issues | Low | Optional; consider making it opt-in since it adds verbosity |
| NodeLocal DNSCache consideration | Node-local DNS caching reduces latency for pods on the same node | High | NodeLocal DNSCache is a node-level DaemonSet change; too invasive for v1 — document as future |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Reducing `ndots` globally | Changing ndots on CoreDNS affects all pods; breaks short-name resolution for service-to-service calls | If ndots reduction is needed, document per-pod `dnsConfig` as the right lever |
| Replacing the CoreDNS Deployment | Risky; kind pins CoreDNS version and image | Patch only the ConfigMap |
| Installing a second DNS server | Creates routing ambiguity | Single CoreDNS with patched config |
| Enabling `autopath` without `pods verified` | Autopath requires knowing the source pod's namespace; `pods insecure` disables this | Always pair `autopath @kubernetes` with `pods verified` |

### User-Facing Behavior

Pods resolve external DNS names faster (fewer round-trips to upstream). Developers doing `nslookup` or `curl` to external services from inside pods see faster first responses. Short service names (`my-service`, not `my-service.default.svc.cluster.local`) still resolve correctly because autopath handles the search path on the server side.

### Required Configuration

Patched CoreDNS Corefile:
```
.:53 {
    errors
    health {
        lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
        pods verified          # Changed from "insecure" — required for autopath
        fallthrough in-addr.arpa ip6.arpa
        ttl 30
    }
    autopath @kubernetes       # Added: server-side search path walking
    prometheus :9153
    forward . /etc/resolv.conf {
        max_concurrent 1000
    }
    cache 60                   # Increased from 30 to 60 seconds
    loop
    reload
    loadbalance
}
```

```yaml
# kinder cluster config
addons:
  coreDNSTuning: true   # default; applies the patched Corefile
```

---

## Addon 5: Kubernetes Dashboard (Web UI)

**CRITICAL NOTE:** The official `kubernetes/dashboard` project was archived on January 21, 2026 and moved to `kubernetes-retired/dashboard`. It receives no further security patches or updates. The Kubernetes SIG UI group now officially recommends **Headlamp** (v0.40.1, February 2026) as the successor. This research recommends shipping Headlamp, not the retired Dashboard.

### Table Stakes

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| A web UI is reachable after cluster creation | "Dashboard" is an expected feature of any "batteries-included" cluster | Low | Headlamp deploys via Helm chart into `kube-system`; must use static manifests per project constraint |
| View pods, services, deployments | Core cluster state visibility — the minimum useful dashboard | Low | Headlamp provides this out of the box |
| View logs from pods in the browser | Log tailing is the most common dashboard use case | Low | Headlamp supports this |
| No security footgun for local dev | Default access must not be a blank-password admin | Medium | Headlamp requires a service account token; kinder should create a dedicated `kinder-dashboard` SA with cluster-admin role and print the token after cluster creation |
| Access via `kubectl port-forward` or printed URL | User needs a clear path to open the dashboard | Low | Print the port-forward command and URL after cluster creation; alternatively ship an HTTPRoute via Envoy Gateway |

### Differentiators

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Use Headlamp instead of archived Dashboard | Actively maintained, official Kubernetes sub-project, better UX | Low | Swap the Docker image and manifests; no architectural difference from the user's perspective |
| Service account token printed on cluster create | Zero-friction first login | Low | Create SA + ClusterRoleBinding, generate token, print it at the end of `kinder create cluster` output |
| Gateway API HTTPRoute for dashboard access | If Envoy Gateway is enabled, the dashboard gets a real URL on a LoadBalancer IP | Medium | Optional enhancement; fall back to port-forward instructions when Envoy Gateway is disabled |
| Headlamp's Gateway API awareness (v0.40+) | Headlamp can display HTTPRoute resources in the UI | Low | Provides visibility into Envoy Gateway resources without any extra config |
| Plugin extensibility | Headlamp supports plugins for Prometheus metrics, etc. | Low | Available but out of scope for v1; document as future capability |

### Anti-Features

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Shipping `kubernetes/dashboard` (archived) | No security patches from Jan 2026 onward; bad default for a new project | Use Headlamp |
| Creating a `cluster-admin` binding for the `default` SA | Classic security mistake; all pods in the namespace inherit cluster-admin | Create a dedicated `kinder-dashboard` service account instead |
| OIDC/OAuth setup by default | The PROJECT.md explicitly lists this as out of scope for v1 | Token-based login only |
| Exposing the dashboard on a NodePort without token | Unauthenticated cluster admin access on an open port | Always require token; always use port-forward or HTTPRoute (which requires token auth via Headlamp) |
| Using `kubectl proxy` as the documented access method | Flaky, requires a running terminal, not beginner-friendly | Pre-create the HTTPRoute or print a port-forward command that users can run once |

### User-Facing Behavior

At the end of `kinder create cluster`, kinder prints:

```
Dashboard: http://localhost:8888 (run: kubectl port-forward -n kube-system svc/headlamp 8888:80)
Token:     eyJhbGciOiJSUzI1NiIs...  (expires: never for local dev)
```

User pastes the token into Headlamp's login screen. They see their cluster's pods, services, deployments, and logs. If Envoy Gateway is also enabled, the dashboard is additionally reachable at the Gateway's LoadBalancer IP on a dedicated HTTPRoute.

### Required Configuration

```yaml
# kinder cluster config
addons:
  dashboard: true   # default; set false to skip
```

Manifests to apply (static, no Helm):
1. Headlamp Deployment + Service in `kube-system`
2. ServiceAccount `kinder-dashboard` in `kube-system`
3. ClusterRoleBinding: `kinder-dashboard` -> `cluster-admin` (local dev only; document clearly)
4. Token Secret for the SA (long-lived for local dev)

---

## Feature Dependencies

```
MetalLB
  └── (no addon dependencies; depends on Docker/Podman/Nerdctl network existing)

Envoy Gateway
  └── Requires: MetalLB (Gateway proxy Service needs a LoadBalancer IP)
  └── Requires: Gateway API CRDs (installed as part of Envoy Gateway action)

Metrics Server
  └── (no addon dependencies; reads from kubelet directly)

CoreDNS Tuning
  └── Requires: CoreDNS already running (kind installs this; always true)
  └── Soft dependency: none

Dashboard (Headlamp)
  └── Soft dependency on Envoy Gateway (optional HTTPRoute for URL access)
  └── (no hard addon dependencies; port-forward access works without Envoy Gateway)
```

### Installation Order

```
1. kindnet CNI          (existing kind action — must come first)
2. local-path-provisioner (existing kind action)
3. MetalLB              (new action — must come before Envoy Gateway)
4. Envoy Gateway        (new action — after MetalLB)
5. Metrics Server       (new action — independent, parallel-possible)
6. CoreDNS Tuning       (new action — after CoreDNS pod is ready)
7. Dashboard (Headlamp) (new action — last; can create HTTPRoute only after Envoy Gateway ready)
```

Steps 5 and 6 are independent and could theoretically run in parallel, but sequential is simpler and safer given the existing kind action pipeline architecture.

---

## MVP Definition

The v1.0 MVP is: all 5 addons installed by default, all opt-outable, all verified ready before cluster is reported ready.

### Must-Have for MVP

1. **MetalLB**: Layer 2 mode, auto-detected IP pool, IPAddressPool + L2Advertisement applied.
2. **Envoy Gateway**: Controller running, GatewayClass created, Gateway API CRDs installed.
3. **Metrics Server**: Running with `--kubelet-insecure-tls`, `kubectl top` works within 60 seconds.
4. **CoreDNS Tuning**: Patched Corefile with `autopath`, `pods verified`, `cache 60`.
5. **Dashboard (Headlamp)**: Running, token printed, port-forward command printed.

### Defer to v1.1

| Feature | Reason to Defer |
|---------|----------------|
| cert-manager integration for TLS | Scope creep; document manual TLS path instead |
| NodeLocal DNSCache | Invasive node-level change; high complexity |
| Headlamp HTTPRoute via Envoy Gateway | Nice to have; port-forward covers MVP |
| Custom addon plugin system | Explicitly out of scope in PROJECT.md |
| VPA (Vertical Pod Autoscaler) | Separate project, not a standard local dev need |
| Prometheus + Grafana stack | Full monitoring is a separate milestone |

---

## Feature Prioritization Matrix

| Addon | User Impact | Implementation Complexity | MVP Required | v1.1 |
|-------|------------|--------------------------|-------------|-------|
| MetalLB Layer 2 + auto IP pool | Critical (everything else depends on it) | Medium | Yes | — |
| Metrics Server `kubectl top` | High (daily use) | Low | Yes | — |
| Envoy Gateway HTTPRoute | High (replaces Ingress) | Medium | Yes | — |
| CoreDNS `autopath` + cache | Medium (DX improvement) | Low | Yes | — |
| Headlamp Dashboard + token | Medium (visual ops) | Low | Yes | — |
| MetalLB Podman/Nerdctl subnet detection | Medium (multi-provider) | Medium | Yes | — |
| Headlamp HTTPRoute (via Envoy GW) | Low (nice to have) | Low | No | Yes |
| TLS termination in Envoy Gateway | Medium | Medium | No | Yes |
| CoreDNS NodeLocal cache | Low | High | No | v2 |
| Prometheus metrics | High | High | No | v2 |

---

## Sources

- [MetalLB Installation](https://metallb.universe.tf/installation/) — MEDIUM confidence (official docs)
- [MetalLB Configuration: IPAddressPool and L2Advertisement](https://metallb.universe.tf/configuration/) — HIGH confidence (official docs)
- [MetalLB v0.15.3 Helm on ArtifactHub](https://artifacthub.io/packages/helm/metallb/metallb) — MEDIUM confidence (package registry)
- [Auto-detecting Docker subnet for MetalLB in kind](https://michaelheap.com/metallb-ip-address-pool/) — MEDIUM confidence (community blog, verified technique)
- [MetalLB Layer 2 Concepts](https://metallb.universe.tf/concepts/layer2/) — HIGH confidence (official docs)
- [Envoy Gateway Quickstart v1.7.0](https://gateway.envoyproxy.io/docs/tasks/quickstart/) — HIGH confidence (official docs)
- [Envoy Gateway v1.7.0 Release Announcement](https://gateway.envoyproxy.io/news/releases/v1.7/) — HIGH confidence (official)
- [Envoy Gateway: Deploy without LoadBalancer via NodePort (Issue #3385)](https://github.com/envoyproxy/gateway/issues/3385) — MEDIUM confidence (official GitHub issue)
- [Metrics Server GitHub — v0.8.1](https://github.com/kubernetes-sigs/metrics-server) — HIGH confidence (official repo)
- [Kubernetes Resource Metrics Pipeline](https://kubernetes.io/docs/tasks/debug/debug-cluster/resource-metrics-pipeline/) — HIGH confidence (official Kubernetes docs)
- [CoreDNS Customizing DNS Service](https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/) — HIGH confidence (official Kubernetes docs)
- [CoreDNS autopath plugin](https://coredns.io/plugins/autopath/) — HIGH confidence (official CoreDNS docs)
- [Using CoreDNS Effectively with Kubernetes](https://www.infracloud.io/blogs/using-coredns-effectively-kubernetes/) — MEDIUM confidence (well-regarded community blog)
- [Kubernetes Dashboard archived (kubernetes-retired/dashboard)](https://github.com/kubernetes-retired/dashboard) — HIGH confidence (official GitHub archive)
- [Headlamp v0.40.1 releases](https://github.com/kubernetes-sigs/headlamp/releases) — HIGH confidence (official repo)
- [Headlamp official site](https://headlamp.dev/) — HIGH confidence (official)
- [Kubernetes Dashboard Alternatives 2026](https://alexandre-vazquez.com/kubernetes-dashboard-alternatives-2026/) — LOW confidence (community blog, useful context)
- [Kubernetes Gateway API in 2026: Envoy Gateway, Istio, Cilium and Kong](https://dev.to/mechcloud_academy/kubernetes-gateway-api-in-2026-the-definitive-guide-to-envoy-gateway-istio-cilium-and-kong-2bkl) — LOW confidence (community, useful ecosystem overview)
