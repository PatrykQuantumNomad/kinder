# External Integrations

**Analysis Date:** 2026-03-02

## APIs & External Services

**Container Runtimes:**
- Docker - Primary container runtime
  - Integration: `pkg/cluster/internal/providers/docker/`
  - Invoked via exec, not SDK
  - Auto-detection support

- Podman - Alternative container runtime
  - Integration: `pkg/cluster/internal/providers/podman/`
  - Invoked via exec, not SDK
  - Auto-detection support

- nerdctl - Alternative container runtime
  - Integration: `pkg/cluster/internal/providers/nerdctl/`
  - Invoked via exec, not SDK
  - Auto-detection support

**Kubernetes Components:**
- kubeadm - Kubernetes initialization
  - Integration: `pkg/cluster/internal/create/actions/kubeadminit/`
  - Used to bootstrap control plane and worker nodes
  - Invoked via container exec

- kubelet - Node agent (embedded in node image)
  - Integration: Node provisioning via container

- kubectl - Cluster management
  - Integration: Used for applying manifests, checking readiness
  - Invoked via container exec or locally

**Kubernetes Addons (installed during cluster creation):**
1. **MetalLB** (Load Balancer)
   - Namespace: `metallb-system`
   - Configuration: `pkg/cluster/internal/create/actions/installmetallb/`
   - Manifests: `pkg/cluster/internal/create/actions/installmetallb/manifests/metallb-native.yaml`
   - Purpose: Enables LoadBalancer service type support
   - Enabled by: `Config.Addons.MetalLB` (default: true)

2. **Envoy Gateway** (Gateway API)
   - Configuration: `pkg/cluster/internal/create/actions/installenvoygw/`
   - Purpose: Provides Gateway API CRD support
   - Enabled by: `Config.Addons.EnvoyGateway` (default: true)

3. **Metrics Server** (Metrics for HPA/kubectl top)
   - Configuration: `pkg/cluster/internal/create/actions/installmetricsserver/`
   - Purpose: Enables `kubectl top` and Horizontal Pod Autoscaler metrics
   - Enabled by: `Config.Addons.MetricsServer` (default: true)

4. **CoreDNS Tuning** (Performance optimization)
   - Configuration: `pkg/cluster/internal/create/actions/installcorednstuning/`
   - Manifests: `pkg/cluster/internal/create/actions/installcorednstuning/`
   - Purpose: Enables caching and performance tuning for CoreDNS
   - Enabled by: `Config.Addons.CoreDNSTuning` (default: true)

5. **Headlamp Dashboard** (Kubernetes UI)
   - Configuration: `pkg/cluster/internal/create/actions/installdashboard/`
   - Manifests: `pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml`
   - Creates: RBAC, long-lived service account token, deployment/headlamp
   - Secret: `kinder-dashboard-token` (contains access token)
   - Enabled by: `Config.Addons.Dashboard` (default: true)

## Data Storage

**Databases:**
- None - Kinder is a stateless CLI tool
- Cluster state: Stored in Docker containers/volumes managed by container runtime
- kubeconfig: Stored in `~/.kube/config` or `$KUBECONFIG` (standard Kubernetes location)

**File Storage:**
- Local filesystem only
- kubeconfig merging/removal: `pkg/cluster/internal/kubeconfig/`
- Node image caching: Via container runtime (Docker/Podman/nerdctl)
- Configuration files: YAML files on disk, no remote storage

**Caching:**
- None - Each cluster creation fresh; no persistent caching across clusters

## Authentication & Identity

**Auth Provider:**
- Custom - Kubernetes built-in auth
  - Implementation: kubeadm-generated certificates and tokens
  - kubeconfig contains client certificate auth to control plane
  - Service accounts: Created per addon (MetalLB, Metrics Server, Headlamp)
  - Headlamp: Long-lived token in `kinder-dashboard-token` secret
  - No external auth provider required

## Monitoring & Observability

**Error Tracking:**
- None detected - Standard Go error handling and CLI output

**Logs:**
- Console output - Real-time status and logs during cluster creation
- CLI logging: `pkg/cmd/logger.go`, `pkg/internal/cli/logger.go`
- Smart terminal detection for formatted output (spinners, colors)
- Log levels: `logger.V(0)` for info messages
- Cluster logs export: `kind export logs` command in upstream kind

## CI/CD & Deployment

**Hosting:**
- GitHub Pages - New Astro documentation site
- Netlify - Legacy Hugo site (via `netlify.toml`)

**CI Pipeline:**
- GitHub Actions workflow files:
  - `.github/workflows/deploy-site.yml` - Build and deploy documentation
    - Runs on: `push` to main (paths: `kinder-site/**`)
    - Uses: `withastro/action@v5`
    - Deploys to GitHub Pages
    - Node.js 22

  - `.github/workflows/release.yml` - Release binary builds

  - `.github/workflows/docker.yaml` - Test with Docker

  - `.github/workflows/podman.yml` - Test with Podman

  - `.github/workflows/nerdctl.yaml` - Test with nerdctl

  - `.github/workflows/vm.yaml` - Test on VM

**Build Process:**
- Local: `make build` → outputs to `bin/kinder` binary
- Cross-compilation: Go's native support for GOOS/GOARCH
- Build flags: Reproducible builds (`-trimpath`), minimal binaries (`-w`), git metadata

## Environment Configuration

**Required env vars:**
- None - Tool works out-of-box
- Optional: `KIND_CLUSTER_NAME`, `KIND_EXPERIMENTAL_PROVIDER`, container runtime env vars
- Standard: `KUBECONFIG`, proxy vars (HTTP_PROXY, HTTPS_PROXY, NO_PROXY)

**Secrets location:**
- kubeconfig: `~/.kube/config` (contains admin credentials for created cluster)
- Headlamp token: `kinder-dashboard-token` secret in cluster (ephemeral)
- No external secrets management

## Webhooks & Callbacks

**Incoming:**
- None detected

**Outgoing:**
- None detected - Tool is read-only from external systems perspective

## Container Runtime Selection

**Detection Strategy:**
- Order: Docker (preferred) → Podman → nerdctl → error if none found
- Override: `KIND_EXPERIMENTAL_PROVIDER` environment variable
- Auto-fallback: If no explicit provider set, defaults to Docker
- Location: `pkg/cluster/provider.go`, `pkg/internal/runtime/runtime.go`

## Configuration File Format

**Cluster Config:**
- Type: Kubernetes-style YAML (kind: Cluster, apiVersion: kind.x-k8s.io/v1alpha4)
- Location: Passed via `--config` flag or stdin via `-`
- Format example:
  ```yaml
  kind: Cluster
  apiVersion: kind.x-k8s.io/v1alpha4
  name: my-cluster
  addons:
    metalLB: true
    envoyGateway: true
    metricsServer: true
    coreDNSTuning: true
    dashboard: true
  ```
- Schema: `pkg/apis/config/v1alpha4/types.go`

---

*Integration audit: 2026-03-02*
