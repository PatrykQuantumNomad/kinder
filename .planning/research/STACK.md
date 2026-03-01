# Technology Stack: Kinder Addons

**Project:** kinder — batteries-included kind fork
**Milestone:** v1.0 Addons (MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, Kubernetes Dashboard)
**Researched:** 2026-03-01
**Overall confidence:** HIGH (versions verified against GitHub releases and official docs)

---

## Recommended Stack

### Addon Versions and Manifests

| Addon | Version | Manifest / Install Method | Confidence |
|-------|---------|--------------------------|------------|
| MetalLB | v0.15.3 | `kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.15.3/config/manifests/metallb-native.yaml` | HIGH |
| Envoy Gateway | v1.7.0 | `kubectl apply --server-side -f https://github.com/envoyproxy/gateway/releases/download/v1.7.0/install.yaml` | HIGH |
| Gateway API CRDs | v1.4.1 | Bundled inside Envoy Gateway v1.7.0 install.yaml (experimental channel) | HIGH |
| Metrics Server | v0.8.1 | `kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.8.1/components.yaml` + patch | HIGH |
| CoreDNS tuning | N/A (ConfigMap edit) | `kubectl patch configmap/coredns -n kube-system --patch-file <embedded-patch>` | HIGH |
| Kubernetes Dashboard | Helm chart 7.14.0 | Helm only — no static manifest available | HIGH |

---

## Per-Addon Detail

### 1. MetalLB v0.15.3

**Why v0.15.3:** Latest stable release (December 4, 2024). v0.15.x adds IPAddressPool status counters and ServiceBGPStatus CRD. No breaking changes to L2 mode or IPAddressPool/L2Advertisement CRDs from v0.14.x.

**Manifest URL (pin this exactly):**
```
https://raw.githubusercontent.com/metallb/metallb/v0.15.3/config/manifests/metallb-native.yaml
```

**What the manifest installs:**
- Namespace `metallb-system`
- Controller Deployment (IP allocation)
- Speaker DaemonSet (ARP responder for L2 mode)
- 7 CRDs under `metallb.io` group, all `v1beta1` (IPAddressPool, L2Advertisement, BGPPeer v1beta2, BGPAdvertisement, BFDProfile, Community, ConfigurationState)
- RBAC, ValidatingWebhookConfiguration

**Post-install configuration required (two CRs):**

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kinder-pool
  namespace: metallb-system
spec:
  addresses:
    - <DOCKER_SUBNET_UPPER_RANGE>
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: kinder-l2advert
  namespace: metallb-system
spec:
  ipAddressPools:
    - kinder-pool
```

**Kind/Docker-specific: IP range detection.**
The Docker network for kind is typically `172.18.0.0/16` but is not guaranteed. At action execution time, detect the subnet dynamically:

```bash
docker network inspect -f '{{(index .IPAM.Config 0).Subnet}}' kind
```

Allocate the upper /27 of that subnet (last 32 addresses) for MetalLB to avoid clashes with container IPs. Example: if subnet is `172.18.0.0/16`, pool is `172.18.255.200-172.18.255.250`.

In Go, within the action, use `exec.Command("docker", "network", "inspect", ...)` on the provider name, then template the IPAddressPool CIDR before applying. Alternatively hard-code a subnet that does not overlap with Docker's default gateway assignments and document the assumption.

**Why L2 mode (not BGP):** L2 mode requires no BGP router configuration. It works via ARP, which functions correctly within Docker's bridge network. BGP mode requires a BGP peer (like FRR) which does not exist in kind's network by default.

**Wait condition:** Poll `kubectl get pods -n metallb-system` until controller and speaker pods are Running. Then apply the IPAddressPool and L2Advertisement CRs. CRs must be applied AFTER the webhook is ready (controller pod Running), otherwise the ValidatingWebhook will reject them.

---

### 2. Envoy Gateway v1.7.0

**Why v1.7.0:** Latest stable release (February 5, 2026). Bundles Gateway API v1.4.1 (verified via compatibility matrix). Supports Kubernetes v1.32–v1.35.

**Install command:**
```bash
kubectl apply --server-side -f https://github.com/envoyproxy/gateway/releases/download/v1.7.0/install.yaml
```

The `--server-side` flag is required (official docs specify it). The install.yaml bundles:
- Gateway API CRDs from the **experimental channel** (includes TCPRoute, BackendTLSPolicy, etc.)
- Envoy Gateway CRDs
- Envoy Gateway controller Deployment in namespace `envoy-gateway-system`

**Alternative Helm install (for reference, NOT used in kinder):**
```bash
helm install eg oci://docker.io/envoyproxy/gateway-helm --version v1.7.0 -n envoy-gateway-system --create-namespace
```

**Kind/Docker-specific: Gateway service type.**
Envoy Gateway creates Envoy Proxy services as `LoadBalancer` by default. In kind without MetalLB, this would cause the service to pend forever. Since MetalLB is installed first (see Installation Order below), LoadBalancer services receive IPs from the MetalLB pool — this works correctly.

If MetalLB is disabled by the user, the Envoy service will stay Pending. The action should warn the user rather than hanging.

**Post-install: no quickstart.yaml.** The `quickstart.yaml` distributed with v1.7.0 creates a GatewayClass, Gateway, HTTPRoute, and sample app. Do NOT apply this by default — it creates user-facing resources. Install only the controller.

**Wait condition:** `kubectl wait --timeout=5m -n envoy-gateway-system deployment/envoy-gateway --for=condition=Available`

**Ordering constraint:** CRDs from install.yaml must be committed to etcd before the Envoy Gateway controller starts. The `install.yaml` includes CRDs and the controller in a single file. Server-side apply handles the ordering, but add a brief readiness wait after apply to ensure webhooks are registered.

---

### 3. Metrics Server v0.8.1

**Why v0.8.1:** Latest stable release (January 29, 2026). v0.8.x requires Kubernetes v1.25+ (kind's default images are v1.30+, so no issue).

**Base manifest URL:**
```
https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.8.1/components.yaml
```

**Kind-specific patch required.**
Kind node images do not have proper TLS certificates for the kubelet. Without the patch, metrics-server fails to scrape kubelet metrics. The required flag is `--kubelet-insecure-tls`.

In addition, `--kubelet-preferred-address-types=InternalIP` is required because kind nodes have hostnames that do not resolve inside the cluster; InternalIP is the only reachable address type.

**Do NOT use raw Kustomize** (avoids Helm/Kustomize binary dependency). Instead, apply the manifest and then patch the Deployment with a JSON merge patch:

```go
// In the action: fetch components.yaml content, apply it, then patch deployment
const metricsServerPatch = `
{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "metrics-server",
          "args": [
            "--cert-dir=/tmp",
            "--secure-port=10250",
            "--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
            "--kubelet-use-node-status-port",
            "--metric-resolution=15s",
            "--kubelet-insecure-tls"
          ]
        }]
      }
    }
  }
}`
```

Apply via: `kubectl patch deployment metrics-server -n kube-system --type=merge -p '<patch>'`

Or, more aligned with the existing kind pattern (embedded manifest), embed the full components.yaml with the extra arg pre-patched so no runtime patching is required. This is the cleaner approach since kind embeds CNI and storage manifests as Go string constants.

**Why not Helm chart:** adds Helm as a runtime dependency, contradicts PROJECT.md constraint.

**Wait condition:** `kubectl wait --timeout=3m -n kube-system deployment/metrics-server --for=condition=Available`

---

### 4. CoreDNS Tuning

**Why patch CoreDNS (not reinstall):** kind's kubeadm already installs CoreDNS. The version is determined by the node image (e.g., CoreDNS v1.11.x for Kubernetes v1.30+). We tune the existing deployment by patching its ConfigMap.

**What to tune and why:**

| Setting | Default | Kinder recommendation | Rationale |
|---------|---------|----------------------|-----------|
| `cache` TTL | 30 seconds | `cache 300` (5 minutes) | Reduces upstream DNS pressure for local dev; services don't change frequently |
| `forward` | `/etc/resolv.conf` | unchanged | Host resolver is fine for local dev |
| `health` | `health` | `health { lameduck 5s }` | Prevents in-flight requests from failing during restart |

**The ConfigMap patch (Corefile):**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
    .:53 {
        errors
        health {
            lameduck 5s
        }
        ready
        kubernetes cluster.local in-addr.arpa ip6.arpa {
            pods insecure
            fallthrough in-addr.arpa ip6.arpa
            ttl 30
        }
        prometheus :9153
        forward . /etc/resolv.conf {
            max_concurrent 1000
        }
        cache 300
        loop
        reload
        loadbalance
    }
```

Key change from default: `cache 30` -> `cache 300`. Everything else is preserved from the kind default Corefile to avoid breaking cluster DNS.

**Do NOT add ndots tuning here.** ndots is a per-pod setting (pod DNS policy), not a CoreDNS server setting. Do not modify it in the Corefile.

**Apply method:**
```bash
kubectl apply -f - <<EOF
<configmap yaml above>
EOF
```

CoreDNS has a `reload` plugin that auto-detects ConfigMap changes within ~2 minutes (no pod restart needed). However, to guarantee immediate effect in a fresh cluster, optionally rolling-restart CoreDNS: `kubectl rollout restart deployment/coredns -n kube-system`.

**Wait condition:** `kubectl rollout status deployment/coredns -n kube-system --timeout=2m`

---

### 5. Kubernetes Dashboard Helm Chart 7.14.0

**Why Helm chart 7.14.0:** This is the latest release (October 30, 2024). Dashboard v3 dropped all static manifest support starting with chart v7.0.0. There is no alternative to Helm for current versions.

**IMPORTANT:** This means kinder needs a Helm binary available at cluster creation time, OR must embed/vendor the rendered manifests.

**Recommended approach for kinder:** Pre-render Helm templates and embed them as static manifests. This avoids a runtime Helm dependency:

```bash
helm repo add kubernetes-dashboard https://kubernetes.github.io/dashboard/
helm template kubernetes-dashboard kubernetes-dashboard/kubernetes-dashboard \
  --version 7.14.0 \
  --namespace kubernetes-dashboard \
  --create-namespace \
  --set nginx.enabled=false \
  --set cert-manager.enabled=false \
  --set app.ingress.enabled=false \
  > embedded-dashboard-7.14.0.yaml
```

Embed this rendered YAML as a Go constant in the dashboard action. Update manually when chart version bumps.

**Rationale for disabled flags:**
- `nginx.enabled=false`: kind uses kindnet CNI, not nginx-ingress. Adding nginx-ingress bloats the cluster and conflicts with Envoy Gateway.
- `cert-manager.enabled=false`: kind clusters use self-signed kubeadm certs. cert-manager adds complexity without benefit for local dev.
- `app.ingress.enabled=false`: no Ingress resource created by default; users access via `kubectl port-forward` or through Envoy Gateway manually.

**Access method (document for users):**
```bash
# Create admin ServiceAccount
kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: admin-user
  namespace: kubernetes-dashboard
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: admin-user
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: admin-user
  namespace: kubernetes-dashboard
EOF

# Generate token
kubectl create token admin-user -n kubernetes-dashboard

# Port-forward
kubectl port-forward svc/kubernetes-dashboard-kong-proxy 8443:443 -n kubernetes-dashboard
```

The service name `kubernetes-dashboard-kong-proxy` is the Dashboard v3 Kong-based proxy service — this is different from Dashboard v2's `kubernetes-dashboard` service. Verify service name against the rendered templates.

**Wait condition:** `kubectl rollout status deployment/kubernetes-dashboard-api -n kubernetes-dashboard --timeout=3m`

---

## Installation Order

This order is mandatory due to hard dependencies:

```
1. MetalLB controller + speaker
   Wait: MetalLB pods Running (webhook ready)

2. MetalLB IPAddressPool + L2Advertisement CRs
   Wait: CR created successfully (webhook validates)

3. CoreDNS ConfigMap patch
   Wait: coredns rollout complete

4. Metrics Server (base manifest + args patch)
   Wait: metrics-server deployment Available

5. Envoy Gateway (install.yaml — CRDs + controller)
   Wait: envoy-gateway deployment Available

6. Kubernetes Dashboard (pre-rendered Helm manifests)
   Wait: kubernetes-dashboard-api deployment Available
```

**Ordering rationale:**

- MetalLB must be first because Envoy Gateway creates LoadBalancer services. If MetalLB is not ready when Envoy Gateway provisions its Gateway, the LoadBalancer service will get an external IP from MetalLB immediately. Reverse order causes the Envoy Gateway service to remain Pending until MetalLB catches up (which it eventually does, but it creates a confusing startup experience).

- MetalLB webhook must be ready before IPAddressPool/L2Advertisement CRs are created. The ValidatingWebhookConfiguration rejects CRs if the webhook pod is not Running. This is a known stumbling block — add a readiness wait between manifest apply and CR creation.

- CoreDNS tuning is independent and can run any time after kubeadminit. Placing it early (step 3) means all subsequent addon pods benefit from the improved cache settings from the start.

- Metrics Server has no dependencies on other addons. Position is arbitrary; placing it mid-sequence is fine.

- Envoy Gateway requires Gateway API CRDs (bundled in its install.yaml). It must come after MetalLB so its LoadBalancer service gets an IP. Install.yaml applies CRDs and controller in one shot.

- Kubernetes Dashboard is last because it has no dependencies on other addons but benefits from having metrics-server available for the metrics scraper component.

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| LoadBalancer | MetalLB v0.15.3 | cloud-provider-kind | cloud-provider-kind runs as a separate binary on the host outside the cluster; cannot be installed as an in-cluster action in kind's pipeline |
| LoadBalancer | MetalLB v0.15.3 | Cilium LB IPAM | Requires replacing kindnet CNI with Cilium — scope creep, breaks existing kind CNI action |
| Gateway | Envoy Gateway v1.7.0 | Contour | Less active development; Envoy Gateway is the CNCF-incubating reference implementation backed by Envoy team |
| Gateway | Envoy Gateway v1.7.0 | Istio Gateway | Istio adds service mesh complexity; 10x larger memory footprint; overkill for local dev |
| Gateway | Envoy Gateway v1.7.0 | NGINX Gateway Fabric | Less ecosystem momentum in 2025-2026; Envoy Gateway has wider adoption |
| Dashboard | Kubernetes Dashboard 7.14.0 | Headlamp | Headlamp requires separate binary install; Kubernetes Dashboard is the canonical upstream option |
| Dashboard | Kubernetes Dashboard 7.14.0 | Lens/OpenLens | Desktop app, not cluster-embedded |
| Dashboard (install) | Pre-rendered Helm | Runtime Helm | Runtime Helm requires `helm` binary present on the host; violates kinder's no-external-tool-dependency design |
| Dashboard (install) | Pre-rendered Helm | Static YAML v2 | Dashboard v2 (raw YAML) was archived January 21, 2026; no security updates |
| Metrics | metrics-server v0.8.1 | kube-state-metrics | Different purpose (object state vs. resource usage); not a replacement for `kubectl top` |

---

## What NOT to Bundle

| Item | Why Not |
|------|---------|
| Prometheus/Grafana stack | Too large for default install; not universally wanted; separate addon or user responsibility |
| cert-manager | Not required by any default addon when Dashboard nginx is disabled; adds 3 CRDs and a controller |
| nginx-ingress-controller | Conflicts with Envoy Gateway; users should use Gateway API, not Ingress |
| Envoy Gateway quickstart.yaml | Creates user-facing GatewayClass/Gateway/HTTPRoute resources; not appropriate for default install |
| MetalLB FRR mode | Requires BGP peering which doesn't exist in kind's Docker network; L2 mode is sufficient |
| Dashboard admin ClusterRoleBinding | Should be user-created post-install; don't bake in full cluster-admin access by default |

---

## Version Compatibility Matrix

| Addon | Kubernetes | Notes |
|-------|-----------|-------|
| MetalLB v0.15.3 | v1.22+ | No explicit upper bound in docs; tested against current kind images (v1.30–v1.35) |
| Envoy Gateway v1.7.0 | v1.32–v1.35 | Verified from official compatibility matrix |
| Gateway API v1.4.1 | Bundled with EG v1.7.0 | Standard + experimental channel CRDs |
| Metrics Server v0.8.1 | v1.25+ | Uses Kubernetes dependencies v0.33.7 |
| CoreDNS | Any | Patch only; CoreDNS version is set by node image |
| Dashboard Helm 7.14.0 | v1.21+ | Helm chart requirement from official docs |

**Kind node image default as of 2026-03:** kind's default node image targets Kubernetes v1.32.x (verify with `kind version` before release). All addons are compatible with v1.32.

---

## Go Implementation Pattern

All addons follow the existing kind action pattern. Reference `installstorage` as the template:

```go
// pkg/cluster/internal/create/actions/installmetallb/metallb.go

package installmetallb

import (
    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/nodeutils"
)

type action struct{}

func NewAction() actions.Action { return &action{} }

func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing MetalLB (LoadBalancer) ⚖️")
    defer ctx.Status.End(false)

    controlPlanes, err := nodeutils.ControlPlaneNodes(ctx.Nodes())
    // ... detect docker subnet, apply manifest, wait for pods, apply CRs
    ctx.Status.End(true)
    return nil
}
```

**Embedded manifests** (Go string constants): MetalLB manifest, Metrics Server components.yaml (pre-patched), CoreDNS Corefile patch, Dashboard pre-rendered YAML, Envoy Gateway install.yaml.

**Fetching at runtime vs embedding:** Embedding is strongly preferred. It:
- Works offline / air-gapped
- Guarantees version pinning
- Avoids network call at cluster creation time
- Follows the pattern of kind's existing CNI/storage manifests (read from node image or fallback constant)

Envoy Gateway install.yaml is large (~3,000 lines). Embed it compressed or reference from a bundled file in the binary. Use Go's `embed` package.

---

## Sources

- MetalLB v0.15.3 release: https://github.com/metallb/metallb/releases/tag/v0.15.3
- MetalLB installation docs: https://metallb.universe.tf/installation/
- MetalLB release notes: https://metallb.universe.tf/release-notes/
- MetalLB kind Docker subnet detection pattern: https://michaelheap.com/metallb-ip-address-pool/
- MetalLB kind issue (gateway IP conflict): https://github.com/kubernetes-sigs/kind/issues/3167
- Envoy Gateway v1.7.0 release: https://github.com/envoyproxy/gateway/releases/tag/v1.7.0
- Envoy Gateway install YAML docs: https://gateway.envoyproxy.io/docs/install/install-yaml/
- Envoy Gateway quickstart: https://gateway.envoyproxy.io/docs/tasks/quickstart/
- Envoy Gateway compatibility matrix: https://gateway.envoyproxy.io/news/releases/matrix/
- Gateway API v1.5.0 release: https://github.com/kubernetes-sigs/gateway-api/releases/tag/v1.5.0
- Metrics Server v0.8.1 release: https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.8.1
- Metrics Server kind patch (gist): https://gist.github.com/sanketsudake/a089e691286bf2189bfedf295222bd43
- Kubernetes Dashboard 7.14.0: https://github.com/kubernetes/dashboard/releases/tag/kubernetes-dashboard-7.14.0
- Dashboard helm-only since v7: https://spacelift.io/blog/kubernetes-dashboard (confirmed)
- Dashboard repo archived Jan 2026: https://github.com/kubernetes/dashboard (read-only)
- CoreDNS cache tuning: https://oneuptime.com/blog/post/2026-02-09-coredns-cache-settings-high-qps/view
- CoreDNS Kubernetes customization: https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/
- kind action pipeline source: /Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/create.go
