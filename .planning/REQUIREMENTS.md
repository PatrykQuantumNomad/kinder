# Requirements: Kinder

**Defined:** 2026-03-01
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## v1 Requirements

Requirements for v1.0 release. Each maps to roadmap phases.

### Foundation

- [ ] **FOUND-01**: Binary is named `kinder` and coexists with `kind` on the system
- [ ] **FOUND-02**: Config schema extends v1alpha4 with an `addons` section for enabling/disabling each addon
- [ ] **FOUND-03**: Existing kind configs without addons section work unchanged (backward compatible)
- [ ] **FOUND-04**: Each addon action checks its enable flag before executing
- [ ] **FOUND-05**: Platform detection warns macOS/Windows users that MetalLB LoadBalancer IPs may not be reachable from the host

### MetalLB

- [ ] **MLB-01**: MetalLB controller and speaker pods are installed and running by default on cluster creation
- [ ] **MLB-02**: IP address pool is auto-detected from the Docker network subnet without user input
- [ ] **MLB-03**: IPAddressPool and L2Advertisement custom resources are applied after MetalLB webhook is ready
- [ ] **MLB-04**: Services of type LoadBalancer receive an EXTERNAL-IP within seconds of creation
- [ ] **MLB-05**: Subnet detection works with Docker provider
- [ ] **MLB-06**: Subnet detection works with Podman provider
- [ ] **MLB-07**: Subnet detection works with Nerdctl provider
- [ ] **MLB-08**: User can disable MetalLB via `addons.metalLB: false` in cluster config

### Envoy Gateway

- [ ] **EGW-01**: Gateway API CRDs are installed before Envoy Gateway controller starts
- [ ] **EGW-02**: Envoy Gateway controller is running and a default GatewayClass is created
- [ ] **EGW-03**: User can create a Gateway + HTTPRoute and traffic routes to backend service via LoadBalancer IP
- [ ] **EGW-04**: User can disable Envoy Gateway via `addons.envoyGateway: false` in cluster config
- [ ] **EGW-05**: If MetalLB is disabled and Envoy Gateway is enabled, kinder prints a clear warning that Gateway proxy will not get an IP
- [ ] **EGW-06**: TLS termination is documented (manual cert path, no cert-manager)

### Metrics Server

- [ ] **MET-01**: Metrics Server is installed with `--kubelet-insecure-tls` flag by default
- [ ] **MET-02**: `kubectl top nodes` returns data within 60 seconds of cluster creation
- [ ] **MET-03**: `kubectl top pods` works for pods in any namespace
- [ ] **MET-04**: HPA can read CPU/memory metrics from the Metrics API
- [ ] **MET-05**: User can disable Metrics Server via `addons.metricsServer: false` in cluster config

### CoreDNS Tuning

- [ ] **DNS-01**: CoreDNS Corefile is patched (not replaced) with `autopath @kubernetes` plugin
- [ ] **DNS-02**: CoreDNS `pods insecure` is changed to `pods verified` (required for autopath)
- [ ] **DNS-03**: Cache TTL is increased from 30s to 60s for external queries
- [ ] **DNS-04**: Existing in-cluster DNS resolution continues to work after patching
- [ ] **DNS-05**: User can disable CoreDNS tuning via `addons.coreDNSTuning: false` in cluster config

### Dashboard (Headlamp)

- [ ] **DASH-01**: Headlamp is installed and running in `kube-system` namespace by default
- [ ] **DASH-02**: A dedicated `kinder-dashboard` service account with cluster-admin role is created
- [ ] **DASH-03**: Service account token is printed at the end of `kinder create cluster` output
- [ ] **DASH-04**: Port-forward command is printed so user can access the dashboard immediately
- [ ] **DASH-05**: User can view pods, services, deployments, and logs in the Headlamp UI
- [ ] **DASH-06**: User can disable Dashboard via `addons.dashboard: false` in cluster config

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Enhanced Networking

- **NET-01**: cert-manager addon for automated TLS certificate management
- **NET-02**: NodeLocal DNSCache for reduced DNS latency

### Dashboard Enhancements

- **DASH-10**: Headlamp HTTPRoute via Envoy Gateway for URL-based access (no port-forward needed)
- **DASH-11**: Headlamp plugin ecosystem documentation

### Monitoring

- **MON-01**: Prometheus + Grafana stack addon
- **MON-02**: Custom metrics adapter for HPA on arbitrary metrics

### Extensibility

- **EXT-01**: Custom addon plugin system for user-defined addons
- **EXT-02**: Addon version pinning via config

## Out of Scope

| Feature | Reason |
|---------|--------|
| OAuth/OIDC for dashboard | Too complex for v1; token-based auth is sufficient for local dev |
| Multi-cluster networking | Single cluster focus |
| Helm as addon manager | Avoid Helm dependency; use static manifests/go:embed |
| Service mesh (Istio, Linkerd) | Conflicts with Envoy Gateway; separate concern |
| BGP mode for MetalLB | Requires external router; impossible in kind without extra containers |
| VPA (Vertical Pod Autoscaler) | Separate project, rarely needed in local dev |
| Real-time chat / notifications | Not applicable to CLI tool |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FOUND-01 | Phase 1 | Pending |
| FOUND-02 | Phase 1 | Pending |
| FOUND-03 | Phase 1 | Pending |
| FOUND-04 | Phase 1 | Pending |
| FOUND-05 | Phase 1 | Pending |
| MLB-01 | Phase 2 | Pending |
| MLB-02 | Phase 2 | Pending |
| MLB-03 | Phase 2 | Pending |
| MLB-04 | Phase 2 | Pending |
| MLB-05 | Phase 2 | Pending |
| MLB-06 | Phase 2 | Pending |
| MLB-07 | Phase 2 | Pending |
| MLB-08 | Phase 2 | Pending |
| MET-01 | Phase 3 | Pending |
| MET-02 | Phase 3 | Pending |
| MET-03 | Phase 3 | Pending |
| MET-04 | Phase 3 | Pending |
| MET-05 | Phase 3 | Pending |
| DNS-01 | Phase 4 | Pending |
| DNS-02 | Phase 4 | Pending |
| DNS-03 | Phase 4 | Pending |
| DNS-04 | Phase 4 | Pending |
| DNS-05 | Phase 4 | Pending |
| EGW-01 | Phase 5 | Pending |
| EGW-02 | Phase 5 | Pending |
| EGW-03 | Phase 5 | Pending |
| EGW-04 | Phase 5 | Pending |
| EGW-05 | Phase 5 | Pending |
| EGW-06 | Phase 5 | Pending |
| DASH-01 | Phase 6 | Pending |
| DASH-02 | Phase 6 | Pending |
| DASH-03 | Phase 6 | Pending |
| DASH-04 | Phase 6 | Pending |
| DASH-05 | Phase 6 | Pending |
| DASH-06 | Phase 6 | Pending |

**Coverage:**
- v1 requirements: 34 total
- Mapped to phases: 34
- Unmapped: 0

---
*Requirements defined: 2026-03-01*
*Last updated: 2026-03-01 after roadmap creation — all 34 requirements mapped*
