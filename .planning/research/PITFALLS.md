# Domain Pitfalls: Adding Addons to kind

**Domain:** Kubernetes-in-Docker cluster tool addon integration
**Researched:** 2026-03-01
**Scope:** MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, Kubernetes Dashboard — all in a kind fork targeting Docker/Podman/Nerdctl providers

---

## Critical Pitfalls

Mistakes that cause silent failures, full rewrites, or "works on my machine but not CI" syndrome.

---

### Pitfall C1: MetalLB — macOS/Windows LoadBalancer IPs Are Unreachable from Host

**What goes wrong:** MetalLB assigns an external IP to a LoadBalancer service (e.g., `172.18.0.200`), `kubectl get svc` shows `EXTERNAL-IP` populated, but `curl` to that IP hangs forever on macOS and Windows. The cluster appears fully functional. The failure is silent.

**Why it happens:** On macOS and Windows, Docker runs containers inside a Linux VM (HyperKit / WSL2). The Docker bridge network — and MetalLB's L2 ARP announcements — are confined to that VM. The host OS has no route to the `172.18.x.x` Docker bridge subnet. ARP replies from MetalLB's speaker pod never reach the macOS host network stack.

**Consequences:** Any user on macOS or Windows who installs kinder, creates a cluster, deploys a LoadBalancer service, gets an IP, and tries to `curl` it will be broken. This is the most user-visible failure and will generate the most bug reports.

**Prevention:**
- Detect the host OS at cluster creation time (or surface this in documentation prominently)
- On Linux: MetalLB L2 mode works as-is
- On macOS/Windows: Recommend `cloud-provider-kind` (the official alternative) or document that users must use `kubectl port-forward` instead
- Document in the kinder README that MetalLB IP reachability is Linux-only
- Consider adding a warning message printed to stderr when running on macOS/Windows during addon installation

**Detection (warning signs):**
- `EXTERNAL-IP` is assigned but `curl <ip>` hangs
- Host is macOS or Windows Docker Desktop
- `docker network inspect kind` shows subnet, but host has no route table entry for it
- `ping <external-ip>` from macOS: 100% packet loss

**Sources:** [kind issue #3556](https://github.com/kubernetes-sigs/kind/issues/3556), [Kind + MetalLB on macOS guide](https://medium.com/@jehadnasser/setting-up-metallb-with-kind-cluster-on-linux-but-not-on-macos-e47f83c2718d)

---

### Pitfall C2: MetalLB — `node.kubernetes.io/exclude-from-external-load-balancers` Blocks Single-Node Clusters

**What goes wrong:** MetalLB speaker pod is running and healthy, IPAddressPool and L2Advertisement are configured correctly, but services never get an external IP (stay `<pending>`). Affects all single-node kind clusters.

**Why it happens:** kubeadm adds the label `node.kubernetes.io/exclude-from-external-load-balancers` to control-plane nodes. MetalLB honors this label and refuses to announce services from labeled nodes. In a single-node kind cluster, the only node is the control plane, so MetalLB has no eligible nodes to announce from. The kinder codebase already removes this label in `kubeadminit/init.go` (line 161-168) — but only for `len(allNodes) == 1`. If the addon installer runs before this label removal completes, MetalLB may start with the label still present.

**Consequences:** MetalLB appears installed and healthy, but LoadBalancer services never get IPs. Users see `<pending>` permanently. The `MetalLB speaker` logs contain no obvious error.

**Prevention:**
- Verify the label removal in `kubeadminit/init.go` runs before MetalLB addon action
- Consider adding `--ignore-exclude-lb` flag to MetalLB speaker via kustomize patch for robustness
- Add a post-install check: confirm at least one node is MetalLB-eligible before reporting addon as ready

**Detection:**
- `kubectl get svc` shows `EXTERNAL-IP: <pending>` for MetalLB-managed services
- `kubectl describe svc` shows MetalLB events mentioning "no nodes available"
- `kubectl get nodes --show-labels | grep exclude-from-external-load-balancers`

**Sources:** [MetalLB troubleshooting docs](https://metallb.universe.tf/troubleshooting/), [exclude-from-external-load-balancers issue](https://soc.meschbach.com/posts/2024/03/23-metallb-kubernetes-and-node.kubernetes.io/exclude-from-external-load-balancers/)

---

### Pitfall C3: MetalLB — IPAddressPool Uses Wrong Subnet (Collides with Node IPs or Uses Unreachable Range)

**What goes wrong:** MetalLB is assigned an IP pool from the wrong subnet — either the Kubernetes pod CIDR, the service CIDR, or a hardcoded range that doesn't match the actual Docker `kind` network subnet. Services get IPs that are either unreachable or collide with existing node IPs.

**Why it happens:** The Docker `kind` network subnet is dynamic — it's generated based on the cluster name using a hash. A hardcoded pool like `172.18.0.200-172.18.0.250` works on one machine, fails on another where the kind network allocated `192.168.207.0/24`. The subnet must be discovered at runtime by querying the Docker (or Podman/Nerdctl) network.

**The correct approach:**
```bash
# Docker
docker network inspect kind --format '{{ (index .IPAM.Config 0).Subnet }}'

# Nerdctl
nerdctl network inspect kind --format '{{ (index .IPAM.Config 0).Subnet }}'

# Podman — different JSON structure
podman network inspect kind --format '{{range .Subnets}}{{.Subnet}}{{end}}'
```
Then allocate a /27 sub-range from that subnet (e.g., IPs `.200` to `.250` within the discovered /24) to avoid colliding with node IPs at the low end of the range.

**Consequences:** Services get IPs outside the routable Docker subnet (unreachable), or IPs that collide with kind node container IPs (routing chaos).

**Prevention:**
- Implement runtime subnet discovery in the addon action using the provider-specific network inspect command
- Use a stable sub-range: take the discovered /24 and allocate a /27 at the upper end (e.g., `.200-.250`)
- Each provider has a different `network inspect` output format — test all three
- Set `avoidBuggyIPs: true` on the IPAddressPool to skip `.0` and `.255` addresses

**Detection:**
- External IP is assigned but unreachable
- `ip route` on a kind node doesn't show a route for the assigned IP
- Node IPs and service IPs are in the same range

**Sources:** [Automatically set MetalLB IP addresses with kind](https://michaelheap.com/metallb-ip-address-pool/), [MetalLB IPAddressPool advanced config](https://metallb.universe.tf/configuration/_advanced_ipaddresspool_configuration/)

---

### Pitfall C4: Envoy Gateway — CRDs Must Be Installed Before the Controller, and Channel Conflicts Break Upgrades

**What goes wrong:** Envoy Gateway controller is deployed before Gateway API CRDs exist, causing the controller to start and crash-loop with `no matches for kind "GatewayClass"`. Alternatively, another Gateway implementation (or a previous Envoy Gateway install) installed standard-channel CRDs, and Envoy Gateway's Helm chart tries to overwrite them with experimental-channel CRDs — blocked by a Validating Admission Policy (VAP).

**Why it happens:**
- Envoy Gateway's Helm chart defaults to installing Gateway API CRDs from the **experimental channel** (includes TCPRoute, BackendTLSPolicy, etc.)
- Gateway API has a VAP that prevents applying experimental-channel CRDs over standard-channel CRDs
- If any other Gateway API implementation is already installed, CRD ownership conflicts arise
- If the controller pod starts before the CRD installation is Established (not just created), it may crash immediately

**Consequences:**
- Controller pod crash-loops with CRD-not-found errors
- HTTPRoute resources fail to apply with "no matches for kind" errors
- Silent failure: controller reports ready but cannot reconcile Gateway resources

**Prevention:**
- Install Gateway API CRDs first, wait for `kubectl wait --for=condition=Established` on each CRD before applying Envoy Gateway controller
- Apply CRDs using `kubectl apply` (not Helm), then apply the Envoy Gateway controller separately
- For kinder: apply CRDs as a separate step before the main Envoy Gateway manifest, with a readiness wait between them
- Prefer explicit CRD management over relying on Helm's CRD folder behavior

**Detection:**
- `kubectl get gatewayclass` returns "No resources found" after Envoy Gateway install
- Envoy Gateway controller pod logs show `no matches for kind "GatewayClass" in version "gateway.networking.k8s.io/v1"`
- `kubectl get crd | grep gateway` shows CRDs in `Terminating` or absent

**Sources:** [Envoy Gateway install with Helm](https://gateway.envoyproxy.io/docs/install/install-helm/), [Gateway API CRD management](https://gateway-api.sigs.k8s.io/guides/crd-management/), [Envoy Gateway CRD channel issue #7238](https://github.com/envoyproxy/gateway/issues/7238)

---

### Pitfall C5: Envoy Gateway — GatewayClass Has No LoadBalancer IP (Stays `Unknown`)

**What goes wrong:** Envoy Gateway installs successfully, GatewayClass is `Accepted`, but the Gateway resource stays in `Unknown` status with no address assigned. HTTPRoutes created against this Gateway never receive traffic.

**Why it happens:** Envoy Gateway provisions a LoadBalancer service for each Gateway. In kind without a LoadBalancer implementation, this service stays `<pending>` indefinitely. The Gateway's status reflects its assigned address — if the LoadBalancer service has no IP, the Gateway has no address, and traffic cannot flow.

**The dependency:** MetalLB (or cloud-provider-kind) must be installed and operational *before* Envoy Gateway creates any Gateway resources. The install ordering is:
1. MetalLB (with working IPAddressPool)
2. Envoy Gateway controller + CRDs
3. Gateway resource creation

**Consequences:** Gateway stays `Unknown`, all HTTPRoutes report `Accepted: False`, all traffic returns connection refused or DNS failure.

**Prevention:**
- Install MetalLB addon action before Envoy Gateway addon action in the kinder action pipeline
- Add a readiness check after MetalLB installation that verifies the IPAddressPool can assign IPs (e.g., create a test LoadBalancer service, verify IP is assigned, then delete it)
- On macOS/Windows where MetalLB IPs are unreachable, note that Gateway address will be assigned but unreachable (same root cause as C1)

**Detection:**
- `kubectl get gateway -A` shows status `Unknown` or no addresses
- Envoy Gateway provisioned LoadBalancer service shows `EXTERNAL-IP: <pending>`
- `kubectl describe gateway` shows reason `AddressNotAssigned`

**Sources:** [Gateway Address docs](https://gateway.envoyproxy.io/docs/tasks/traffic/gateway-address/), [Envoy Gateway issue #5012](https://github.com/envoyproxy/gateway/issues/5012)

---

### Pitfall C6: Metrics Server — Self-Signed Kubelet TLS Certificates Cause Perpetual CrashLoop

**What goes wrong:** Metrics Server deploys but stays in `CrashLoopBackOff` or reports "no metrics available". `kubectl top nodes` returns "Error from server (ServiceUnavailable): the server is currently unable to handle the request". HPA cannot function.

**Why it happens:** In kind, kubelet uses self-signed TLS certificates. The default Metrics Server configuration attempts to verify the Kubelet's serving certificate, which fails because the CA that signed it is not in Metrics Server's trust store. The server-side TLS handshake fails before any metrics are collected.

**The required patch:**
```yaml
# kustomize patch applied after base metrics-server manifest
- op: add
  path: /spec/template/spec/containers/0/args/-
  value: --kubelet-insecure-tls
```

Additionally, set `--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname` to prevent metrics-server from attempting hostname-based connections (which fail inside kind).

**Version-specific trap (metrics-server v0.8.0):** Version 0.8.0 introduced `appProtocol: https` on the Service object. This field causes the kube-apiserver's aggregation layer to route traffic incorrectly in some configurations, breaking metrics even with `--kubelet-insecure-tls`. If using v0.8.0+, verify the Service does not have `appProtocol: https` or remove it via patch.

**Consequences:** No `kubectl top` output, HPA cannot scale workloads, cluster appears functional but monitoring is broken.

**Prevention:**
- Always apply `--kubelet-insecure-tls` and `--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname` as patches when installing in kind
- Pin to a known-good version or test the Service manifest for the `appProtocol` field before applying
- After installation, verify with `kubectl top nodes` before reporting addon ready

**Detection:**
- `kubectl top nodes` returns ServiceUnavailable
- Metrics Server pod logs: `x509: certificate signed by unknown authority`
- `kubectl get apiservice v1beta1.metrics.k8s.io -o yaml` shows `status: False`

**Sources:** [Metrics Server README](https://github.com/kubernetes-sigs/metrics-server), [Kind metrics-server gist](https://gist.github.com/sanketsudake/a089e691286bf2189bfedf295222bd43), [metrics-server issue #1695](https://github.com/kubernetes-sigs/metrics-server/issues/1695)

---

### Pitfall C7: CoreDNS — DNS Forwarding Loop When Host Uses systemd-resolved

**What goes wrong:** CoreDNS pods start but repeatedly crash with `Loop ... detected for zone "."`. Or CoreDNS runs but all DNS resolution inside the cluster fails intermittently. The cluster appears ready but service discovery is broken.

**Why it happens:** On Ubuntu/Debian hosts (including many CI environments and developer machines), `systemd-resolved` listens on `127.0.0.53`. This address appears in `/etc/resolv.conf`, which kind nodes inherit. CoreDNS's `forward . /etc/resolv.conf` directive forwards upstream queries to `127.0.0.53`, which resolves back to the CoreDNS pod — creating an infinite loop detected by the `loop` plugin.

**Consequences:** CoreDNS crash-loops or DNS resolution fails intermittently. Pods can't resolve service names. The cluster is non-functional despite all other components reporting healthy.

**Prevention:**
- When modifying CoreDNS ConfigMap, replace `forward . /etc/resolv.conf` with an explicit upstream: `forward . 8.8.8.8 1.1.1.1` or use `/run/systemd/resolve/resolv.conf` (the non-loopback resolv.conf) as the forward target
- Do NOT simply add entries to the existing Corefile without auditing the `forward` directive first
- After ConfigMap changes, restart CoreDNS pods and verify no loop errors in logs
- Test DNS resolution inside the cluster after any CoreDNS modification: `kubectl run -it --rm debug --image=busybox -- nslookup kubernetes`

**Detection:**
- CoreDNS pod logs: `Loop (127.0.0.1:53 -> :53) detected for zone "."`
- CoreDNS pods repeatedly restarting (CrashLoopBackOff)
- All cluster-internal DNS lookups fail or time out
- Host has `nameserver 127.0.0.53` in `/etc/resolv.conf`

**Sources:** [CoreDNS loop plugin docs](https://coredns.io/plugins/loop/), [CoreDNS loop issue #2354](https://github.com/coredns/coredns/issues/2354)

---

### Pitfall C8: Kubernetes Dashboard v3 — Architecture Break from v2 (Kong Proxy Required)

**What goes wrong:** The v3 dashboard Helm chart installs successfully but the Dashboard UI is unreachable, shows TLS errors, or the login page CSRF token call returns HTTP 200 with an unparseable body (presenting as an "Unknown error (200)" in the UI).

**Why it happens:** Dashboard v3 (Helm chart v7+) fundamentally changed architecture. The Dashboard now runs behind an internal Kong proxy. Direct HTTP access is intentionally blocked — the Kong proxy always serves HTTPS. If you access via `kubectl proxy` (which serves HTTP), the CSRF token endpoint fails with a parse error because the response is HTML rather than JSON. Port-forward to the Kong proxy service (`kubernetes-dashboard-kong-proxy`) over HTTPS is the correct access method.

Additionally:
- `--enable-skip-login` was removed in v3. Bearer token login is the only supported method.
- Tokens must be manually created since Kubernetes 1.24 (no longer auto-generated)
- The Dashboard requires `cert-manager` for TLS. If cert-manager is absent, the Kong proxy fails to start.

**Consequences:** Dashboard installs but is inaccessible or shows confusing errors. Users cannot log in.

**Prevention:**
- Use Helm chart with `cert-manager.enabled=true` (default) — cert-manager is now a hard dependency
- Access via `kubectl port-forward -n kubernetes-dashboard svc/kubernetes-dashboard-kong-proxy 8443:443`
- Create a dedicated service account and generate a token: `kubectl -n kubernetes-dashboard create token <sa-name>`
- In kinder, either include cert-manager as a prerequisite addon, or use an alternative dashboard version (see Alternatives Considered below)
- Document HTTPS-only access in user-facing output

**Detection:**
- Dashboard pod logs: Kong failing to start due to missing certificates
- Browser shows TLS cert error when accessing Dashboard
- Login page shows "Unknown error (200)" — this is the CSRF over HTTP failure
- `kubectl get pods -n kubernetes-dashboard` shows `cert-manager` related pods missing/failing

**Sources:** [Dashboard v3 architecture](https://spacelift.io/blog/kubernetes-dashboard), [CSRF issue #8829](https://github.com/kubernetes/dashboard/issues/8829), [Dashboard v7.x Unknown error (200)](https://medium.com/@tinhtq97/kubernetes-dashboard-7-x-unknown-error-200-a5be156db23f)

---

## Moderate Pitfalls

### Pitfall M1: MetalLB — L2Advertisement Missing or Misconfigured

**What goes wrong:** MetalLB is installed, IPAddressPool is configured, but services still get no IP. The IPAddressPool alone does not cause MetalLB to advertise.

**Prevention:** Always create an `L2Advertisement` resource alongside the `IPAddressPool`. The L2Advertisement must either reference the IPAddressPool explicitly or use an empty `ipAddressPools` list to match all pools. If you want to restrict which nodes announce, configure `nodeSelectors` — but be careful: if the selector matches no nodes (due to label absence), no announcements occur.

```yaml
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: default
  namespace: metallb-system
spec:
  ipAddressPools:
  - default-pool  # must match IPAddressPool name
```

**Sources:** [MetalLB configuration docs](https://metallb.universe.tf/configuration/)

---

### Pitfall M2: MetalLB — Speaker Pods Require `hostNetwork: true` — Fails in Some Rootless Podman Configurations

**What goes wrong:** MetalLB speaker pods fail to start or start but cannot send ARP replies when using kind with rootless Podman. The speaker needs host-network access to send layer-2 packets.

**Why it happens:** In rootless Podman mode, kind nodes run in user namespaces. Pods requesting `hostNetwork: true` may not get full host-network access depending on Podman version and system configuration. MetalLB's L2 speaker relies on raw socket access for ARP — which is restricted in rootless environments.

**Prevention:**
- Test MetalLB speaker pod status after installation in all three providers (Docker, Podman, Nerdctl)
- For rootless Podman: document that MetalLB L2 mode may not work; consider disabling MetalLB or switching to `cloud-provider-kind` on rootless setups
- Check speaker pod logs for `permission denied` or socket errors

**Sources:** [kind rootless docs](https://kind.sigs.k8s.io/docs/user/rootless/)

---

### Pitfall M3: Envoy Gateway — OOM from Disabled CPU Limits and Default Resource Settings

**What goes wrong:** Envoy Gateway or its provisioned EnvoyProxy pods run out of memory in resource-constrained kind nodes, causing pod eviction or node pressure.

**Why it happens:** Envoy Gateway v1.2.0 removed the default CPU limit to eliminate CPU throttling. Without CPU limits, a misconfigured Gateway can consume all CPU. The EnvoyProxy sidecar and the gateway-controller each require non-trivial memory baseline.

**Prevention:**
- Apply resource requests/limits via `EnvoyProxy` custom resource for kind environments
- kind nodes are Docker containers — ensure the Docker daemon has enough memory (at least 4GB for a cluster with all addons)
- Monitor node memory pressure: `kubectl describe node | grep -A 5 "Conditions"`

---

### Pitfall M4: CoreDNS — ConfigMap Patch Overwrites Entire Corefile (No Strategic Merge)

**What goes wrong:** A `kubectl apply` with a full Corefile replacement breaks existing DNS plugins that kind relies on (e.g., the `kubernetes` plugin for service discovery). The cluster DNS stops working for service resolution.

**Why it happens:** CoreDNS uses a ConfigMap with a key named `Corefile`. Unlike Kubernetes Deployment specs, the Corefile content has no strategic merge path — applying a new ConfigMap with `kubectl apply` replaces the entire value. Any plugins present in the original Corefile but absent from the replacement are silently dropped.

**Prevention:**
- Use `kubectl patch configmap coredns -n kube-system --patch-merge-key` or read-modify-write via `kubectl get / kubectl apply`
- Always include the full set of plugins in any modified Corefile, especially `kubernetes cluster.local in-addr.arpa ip6.arpa`
- Test DNS for both external hosts and internal service names after any CoreDNS change
- Prefer additive changes (adding a `rewrite` rule or `hosts` block) over full replacements

**Detection:**
- `nslookup kubernetes.default.svc.cluster.local` fails from inside a pod
- CoreDNS pod logs show `plugin/kubernetes: No kubernetes server defined`

---

### Pitfall M5: Metrics Server — APIService Registration Fails if kube-apiserver Aggregation Layer Is Disabled

**What goes wrong:** Metrics Server installs and runs, but `kubectl top` returns "Metrics API not available". The APIService registration fails.

**Why it happens:** Metrics Server registers itself as an aggregated API (`v1beta1.metrics.k8s.io`). This requires the kube-apiserver's aggregation layer to be enabled via `--enable-aggregator-routing`. In kind clusters, this is enabled by default via kubeadm. However, if the kinder fork modifies kubeadm config, it must not disable this flag.

**Prevention:**
- Do not modify the kube-apiserver aggregation flags in kubeadm configuration
- After installation, verify: `kubectl get apiservice v1beta1.metrics.k8s.io -o jsonpath='{.status.conditions[0].status}'` should return `True`

---

### Pitfall M6: Kubernetes Dashboard — RBAC Insufficient for Default Service Account

**What goes wrong:** Dashboard logs in successfully with a service account token, but shows "Forbidden" or empty resource lists in the UI.

**Why it happens:** The Dashboard's default ServiceAccount has minimal permissions. Displaying nodes, pods, and other cluster resources requires explicit RBAC grants.

**Prevention:**
- Create a dedicated service account with a ClusterRoleBinding to `cluster-admin` for development use (document the security implication)
- Never bind the `kubernetes-dashboard` ServiceAccount itself to cluster-admin — create a separate admin SA
- Generate the token explicitly: `kubectl -n kubernetes-dashboard create token admin-user --duration=24h`

---

### Pitfall M7: Addon Installation Timing — CRD Not Established Before CR Application

**What goes wrong:** Installing a CRD and its custom resources in the same `kubectl apply` call fails with "no matches for kind" even though the CRD is in the manifest.

**Why it happens:** Kubernetes must process and establish a CRD before CRs using it can be accepted. The API server's in-memory cache updates asynchronously. Applying CRDs and CRs in a single call races this propagation.

**Prevention:**
- For Envoy Gateway: apply Gateway API CRDs, wait for `kubectl wait --for=condition=Established crd/gateways.gateway.networking.k8s.io --timeout=60s`, then apply the controller manifest
- For MetalLB: apply the operator manifest, wait for controller to be Running, then apply IPAddressPool and L2Advertisement
- The kinder action pipeline naturally serializes these steps — use explicit readiness waits between each addon phase

---

## Minor Pitfalls

### Pitfall N1: MetalLB FRR Mode Speaker Init Container CrashLoop

**What goes wrong:** MetalLB deployed with the `metallb-frr.yaml` manifest (BGP/FRR mode) has speaker init containers crash-looping. L2 mode (`metallb-native.yaml`) works fine.

**Prevention:** For kind clusters, use L2 mode (native manifest). FRR mode is for BGP environments with physical routers. Don't deploy FRR manifests in kind.

---

### Pitfall N2: Envoy Gateway — `direct_response` 500s Due to Missing Backend Service

**What goes wrong:** HTTPRoute is `Accepted` but all requests return HTTP 500 with no visible error on the client. Envoy logs show `response_code_details: direct_response`.

**Prevention:** When a backend service referenced in an HTTPRoute does not exist, Envoy Gateway assigns a `direct_response` rule (which returns 500) to that route. Check `kubectl describe httproute` for `ResolvedRefs: False` before assuming routing is configured correctly.

---

### Pitfall N3: kind `extraPortMappings` Required for NodePort Dashboard Access

**What goes wrong:** Dashboard NodePort service is created (e.g., port 32443), but `localhost:32443` is not accessible from the host.

**Why it happens:** kind nodes are Docker containers. NodePort on port 32443 is accessible at the *container's* IP, not `localhost`, unless `extraPortMappings` is configured in the kind cluster config.

**Prevention:** Either configure `extraPortMappings` in the cluster config before cluster creation, or access Dashboard via `kubectl port-forward`. Since kinder cannot add `extraPortMappings` after cluster creation, port-forward is the simpler path.

---

### Pitfall N4: Nerdctl Provider — MTU Mismatch Causes Packet Fragmentation

**What goes wrong:** Cluster comes up, pods communicate, but MetalLB LoadBalancer services drop large packets. HTTP works for small responses but fails for large transfers.

**Why it happens:** Nerdctl does not support `ip_masquerade` option for networks (as noted in the kinder source code comment at `/pkg/cluster/internal/providers/nerdctl/network.go` line 121). MTU mismatches between the kind network and the outer interface can cause packet fragmentation that only manifests with larger payloads.

**Prevention:** Set MTU explicitly on the kind network to match the host's default interface MTU. Verify with `ping -M do -s 1400 <service-ip>` after MetalLB is configured.

---

### Pitfall N5: Dashboard Token Expiry — 1-Hour Default

**What goes wrong:** Dashboard works on first access, then shows "Unauthorized" an hour later with no apparent change.

**Prevention:** `kubectl create token` issues tokens with a default 1-hour expiry. For development clusters, use `--duration=8760h` (1 year) or create a `Secret` of type `kubernetes.io/service-account-token` which persists indefinitely (and works across Dashboard restarts).

---

## Technical Debt Patterns

These patterns don't break immediately but create compounding problems over time.

### TD1: Hardcoded Addon Manifest Versions

**Pattern:** Embedding specific manifest URLs with pinned versions (e.g., `metallb/v0.14.3/...`) in the kinder binary.

**Why it becomes debt:** Each Kubernetes minor release may require updated addon versions for compatibility. A hardcoded version works today but may fail with newer Kubernetes node images. Users upgrading their cluster Kubernetes version will get the old addon version.

**Better approach:** Make addon versions configurable (env var or config field), with sensible defaults, so users can override without recompiling.

---

### TD2: No Addon Health Verification After Installation

**Pattern:** Apply manifests, move on. Report cluster as ready.

**Why it becomes debt:** Many of the pitfalls above (C1 through C6) produce a cluster that appears ready but has non-functional addons. Without a post-installation health check, kinder will report "cluster ready" and users will discover broken addons only when they try to use them.

**Better approach:** Each addon action should include a readiness probe after installation (e.g., `kubectl wait --for=condition=Available deployment/metrics-server -n kube-system --timeout=120s`).

---

### TD3: Provider-Specific Network Inspection Duplicated Across Addons

**Pattern:** Each addon that needs the cluster network subnet (MetalLB needs it, potentially others) implements its own provider-specific network inspection.

**Why it becomes debt:** Each provider (Docker, Podman, Nerdctl) has different `network inspect` output format. Duplicating this logic creates three places to break and maintain.

**Better approach:** Implement a shared `GetClusterNetworkSubnet(providerName, clusterName string) (string, error)` utility function used by all addons that need subnet information.

---

## Integration Gotchas

Cross-addon interactions that are non-obvious.

### IG1: Envoy Gateway Depends on MetalLB — Install Order Matters

Envoy Gateway creates a LoadBalancer service for the Gateway. If MetalLB isn't running when the Gateway resource is created, the LoadBalancer stays `<pending>`. The Gateway status reflects this as `Unknown`. **MetalLB must be ready before any Gateway resources are created.**

In the kinder action pipeline: `MetalLB install → MetalLB ready check → Envoy Gateway install → Gateway resource creation`.

---

### IG2: CoreDNS Tuning Must Not Break MetalLB or Envoy Gateway Service Discovery

MetalLB and Envoy Gateway use Kubernetes service discovery (DNS) to find each other and their backends. If a CoreDNS ConfigMap change drops the `kubernetes` plugin, these components lose the ability to resolve service names. Always validate that `kubernetes.default.svc.cluster.local` resolves after CoreDNS changes.

---

### IG3: Dashboard Depends on Metrics Server for Resource Usage Display

Dashboard's resource usage graphs (CPU/memory per pod) require Metrics Server. Dashboard will install and display fine without it, but the resource graphs will show "metrics not available". Install Metrics Server before Dashboard, or accept degraded Dashboard functionality if Metrics Server fails.

---

### IG4: cert-manager (Dashboard Dependency) May Conflict With Existing cert-manager

Dashboard v3's Helm chart installs cert-manager by default. If a user has cert-manager already installed in their cluster (common in company-wide dev clusters), the Dashboard Helm chart will conflict during CRD installation.

**Mitigation:** Detect existing cert-manager and use `--set cert-manager.enabled=false` when applying the Dashboard chart.

---

## Performance Traps

### PT1: All Addons Installed Sequentially With No Parallelism

**Trap:** Installing MetalLB, then waiting for it, then Envoy Gateway, then waiting, etc. makes cluster creation feel slow (2-4 minutes just for addon setup).

**Better:** MetalLB and Metrics Server have no dependency on each other. CoreDNS tuning and Dashboard also have no dependency on each other. Group non-dependent addon installations to run concurrently, then serialize the dependent ones.

---

### PT2: Envoy Gateway's EnvoyProxy Sidecar Memory Overhead

**Trap:** Each Gateway resource provisions a separate Envoy proxy deployment. In a small kind cluster with 2GB RAM allocated to Docker, a single Gateway + Envoy Gateway controller + MetalLB + Dashboard can exhaust memory.

**Mitigation:** Set conservative resource limits on the EnvoyProxy custom resource for kind environments. Document minimum Docker memory requirements (recommend 4GB).

---

### PT3: CoreDNS Cache TTL Tuning Can Cause Stale DNS During Addon Roll-out

**Trap:** Increasing CoreDNS cache TTL (a common "tuning" for performance) means that if a service IP changes or an addon restarts with a new ClusterIP during installation, old DNS responses are served for up to the cache TTL.

**Mitigation:** Apply CoreDNS tuning *after* all addons are installed and stable, not during the installation sequence.

---

## Security Mistakes

### SM1: Dashboard Service Account with Cluster-Admin in a Long-Running Cluster

**Mistake:** Creating a `cluster-admin` service account for Dashboard and leaving the cluster running as a shared dev environment.

**Consequence:** Anyone with access to the kinder cluster (or who can port-forward to the Dashboard service) can execute any cluster operation.

**Mitigation for kinder:** Document clearly that the cluster-admin token is for local single-user development only. Consider scoping the token to read-only resources by default, with a flag to enable full admin access.

---

### SM2: MetalLB IP Pool Overlapping with Host Network

**Mistake:** The MetalLB IP pool overlaps with IP addresses used by other services on the host network (e.g., `192.168.1.100-200` when the host is on `192.168.1.0/24`).

**Consequence:** MetalLB "steals" ARP responses for those IPs on the host network, causing intermittent connectivity failures for other hosts on the same subnet (only on Linux where MetalLB L2 works).

**Mitigation:** Always allocate MetalLB pools from the Docker `kind` network subnet, not from the host LAN subnet.

---

## "Looks Done But Isn't" Checklist

This is the list of states that indicate successful installation but actually indicate broken addons.

### MetalLB
- [ ] MetalLB pods are Running (`kubectl get pods -n metallb-system`) — does NOT mean LoadBalancer IPs are reachable
- [ ] A LoadBalancer service has `EXTERNAL-IP` assigned — does NOT mean the IP is reachable from the host on macOS/Windows
- [ ] L2Advertisement exists — does NOT mean it's associated with any IPAddressPool (check `ipAddressPools` field)
- [ ] Speaker pods are Running — does NOT mean they can announce (check `exclude-from-external-load-balancers` label)

**True verification:** Create a test Deployment + LoadBalancer Service, wait for IP assignment, then `curl <external-ip>:<port>` from the host. On Linux only.

### Envoy Gateway
- [ ] Envoy Gateway controller pod is Running — does NOT mean GatewayClass is accepted
- [ ] GatewayClass shows `Accepted: True` — does NOT mean the Gateway has an address
- [ ] Gateway shows an address — does NOT mean HTTPRoutes are routing correctly

**True verification:** Deploy a test pod + Service + HTTPRoute, verify `curl <gateway-ip>/test-path` returns a response from the pod.

### Metrics Server
- [ ] Metrics Server pod is Running — does NOT mean metrics are being collected
- [ ] No crash-loop — does NOT mean TLS verification is succeeding

**True verification:** `kubectl top nodes` returns actual CPU/memory numbers (not an error). Wait 60 seconds after pod starts for initial scrape cycle.

### CoreDNS Tuning
- [ ] CoreDNS pods are Running after ConfigMap change — does NOT mean DNS is working
- [ ] No crash-loops — does NOT mean the forwarding loop wasn't silently swallowed

**True verification:** From a test pod: `nslookup kubernetes.default.svc.cluster.local` AND `nslookup google.com` — both must succeed.

### Kubernetes Dashboard
- [ ] Dashboard pod is Running — does NOT mean the UI is accessible
- [ ] Port-forward is established — does NOT mean login will succeed (CSRF failure on HTTP)
- [ ] Token was generated — does NOT mean the token has sufficient permissions

**True verification:** `kubectl port-forward -n kubernetes-dashboard svc/kubernetes-dashboard-kong-proxy 8443:443`, open `https://localhost:8443`, log in with the token, verify node list loads.

---

## Phase-Specific Warnings

| Implementation Phase | Likely Pitfall | Mitigation |
|---------------------|---------------|------------|
| Addon action scaffolding | No readiness waiting between addons causes race conditions (C4, M7) | Add `kubectl wait` calls after each manifest apply |
| MetalLB integration | Wrong subnet for IPAddressPool (C3), single-node label (C2), macOS silent failure (C1) | Runtime subnet detection, verify label removal in kubeadminit runs first |
| Envoy Gateway integration | CRD-before-controller ordering (C4), pending Gateway when MetalLB absent (C5) | Strict install ordering with readiness checks |
| Metrics Server integration | TLS failure without insecure flag (C6), v0.8.0 appProtocol trap | Apply kustomize patch with insecure-tls; post-install `kubectl top nodes` check |
| CoreDNS tuning | Loop when systemd-resolved present (C7), full ConfigMap overwrite breaking kubernetes plugin (M4) | Read-modify-write ConfigMap, explicit forwarding upstreams |
| Dashboard integration | v3 architecture requires Kong+cert-manager (C8), RBAC insufficient (M6) | Use port-forward over HTTPS, create explicit admin service account |
| Cross-provider testing | MetalLB ARP in rootless Podman (M2), MTU in Nerdctl (N4) | Test each provider; document known limitations |
| "Looks done" validation | All above pitfalls have states that appear successful | Implement per-addon functional smoke test before reporting cluster ready |

---

## Sources

- [MetalLB Troubleshooting](https://metallb.universe.tf/troubleshooting/) — L2 mode specifics, exclude-from-external-load-balancers, ARP verification
- [MetalLB Configuration](https://metallb.universe.tf/configuration/) — IPAddressPool, L2Advertisement requirements
- [MetalLB Advanced IPAddressPool Config](https://metallb.universe.tf/configuration/_advanced_ipaddresspool_configuration/) — avoidBuggyIPs, pool selection
- [kind issue #3556: bridge network not working for MetalLB](https://github.com/kubernetes-sigs/kind/issues/3556) — macOS platform limitation confirmed
- [Automatically set MetalLB IP addresses with kind](https://michaelheap.com/metallb-ip-address-pool/) — runtime subnet detection approach
- [MetalLB + kind on macOS](https://medium.com/@jehadnasser/setting-up-metallb-with-kind-cluster-on-linux-but-not-on-macos-e47f83c2718d) — platform-specific behavior
- [exclude-from-external-load-balancers + MetalLB](https://soc.meschbach.com/posts/2024/03/23-metallb-kubernetes-and-node.kubernetes.io/exclude-from-external-load-balancers/) — single-node workarounds
- [Envoy Gateway Install with Helm](https://gateway.envoyproxy.io/docs/install/install-helm/) — CRD installation ordering
- [Envoy Gateway Configuration Issues](https://gateway.envoyproxy.io/docs/troubleshooting/configuration/) — status field debugging
- [Envoy Gateway CRD Channel Issue #7238](https://github.com/envoyproxy/gateway/issues/7238) — experimental vs standard channel conflict
- [Gateway API CRD Management](https://gateway-api.sigs.k8s.io/guides/crd-management/) — Validating Admission Policy blocking upgrades
- [Envoy Gateway Gateway Address docs](https://gateway.envoyproxy.io/docs/tasks/traffic/gateway-address/) — LoadBalancer IP dependency
- [metrics-server issue #1695: appProtocol: https](https://github.com/kubernetes-sigs/metrics-server/issues/1695) — v0.8.0-specific breakage
- [Metrics Server on Kind gist](https://gist.github.com/sanketsudake/a089e691286bf2189bfedf295222bd43) — required patches
- [CoreDNS loop plugin docs](https://coredns.io/plugins/loop/) — loop detection and resolution
- [CoreDNS loop issue #2354: systemd-resolved](https://github.com/coredns/coredns/issues/2354) — forwarding loop root cause
- [Kubernetes Dashboard v3 architecture](https://spacelift.io/blog/kubernetes-dashboard) — Kong proxy and cert-manager dependencies
- [Dashboard CSRF issue #8829](https://github.com/kubernetes/dashboard/issues/8829) — HTTP vs HTTPS CSRF failure
- [Dashboard v7.x Unknown error (200)](https://medium.com/@tinhtq97/kubernetes-dashboard-7-x-unknown-error-200-a5be156db23f) — concrete error description and fix
- [kind rootless docs](https://kind.sigs.k8s.io/docs/user/rootless/) — Podman networking limitations
- [kind LoadBalancer docs](https://kind.sigs.k8s.io/docs/user/loadbalancer/) — cloud-provider-kind as alternative
