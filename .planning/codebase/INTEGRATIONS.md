# External Integrations

**Analysis Date:** 2026-05-03

## APIs & External Services

**Container Runtime Providers:**
- Docker - Primary container runtime provider
  - SDK/Client: Native Docker API via exec commands
  - Provider: `pkg/cluster/internal/providers/docker/`
  
- Podman - Container runtime provider (drop-in Docker alternative)
  - SDK/Client: Podman CLI via exec commands
  - Provider: `pkg/cluster/internal/providers/podman/`
  
- nerdctl - Containerd client (Docker-compatible CLI)
  - SDK/Client: nerdctl CLI via exec commands
  - Provider: `pkg/cluster/internal/providers/nerdctl/`

**Kubernetes Distribution:**
- kubeadm - Bootstraps Kubernetes control plane inside container nodes
  - Used by: `pkg/cluster/internal/create/actions/`
  - Integration: Executed inside container nodes to initialize cluster

## Data Storage

**Databases:**
- None - Kinder is a stateless CLI tool

**File Storage:**
- Local filesystem only - Clusters stored as container volumes on host
  - Config location: `~/.kinder/clusters/` (default)
  - Kubeconfig: User's `~/.kube/config`
  - Node state: Docker/Podman volume mounts

**Caching:**
- None - No explicit caching layer

## Authentication & Identity

**Auth Provider:**
- Kubernetes native authentication
  - Implementation: Kubeconfig token authentication within created clusters
  - No external identity provider integration
- GitHub (for releases only)
  - Used in: `.goreleaser.yaml` release automation
  - Token: `GITHUB_TOKEN` (GitHub Actions environment)

## Monitoring & Observability

**Error Tracking:**
- None - No external error tracking service integrated

**Logs:**
- Local file capture approach
  - Cluster logs: Captured to `*.log` files in temporary directories
  - Node containerd logs: Retrieved via `journalctl --no-pager -u containerd.service`
  - Container runtime logs: Retrieved via `crictl images` and Docker CLI
  - See: `pkg/cluster/provider.go` (SerialLogs functionality)
- Stdout/Stderr console output for CLI operations
- Optional verbose logging (`-v7` flag) for troubleshooting

**Included Addon:** Metrics Server for cluster metrics (`kubectl top` support)

## CI/CD & Deployment

**Hosting:**
- GitHub (source code and releases)
  - Repository: `https://github.com/PatrykQuantumNomad/kinder`
  - Release artifacts: GitHub Releases via GoReleaser

**CI Pipeline:**
- GitHub Actions
  - Release workflow: `.github/workflows/release.yml`
    - Triggers on: Git tags matching `v*`
    - Uses: GoReleaser v2+ for multi-platform builds
    - Publishes to: GitHub Releases, Homebrew tap
  
  - Docker tests: `.github/workflows/docker.yaml`
    - Runs on: ubuntu-24.04
    - Matrix: ipv4/ipv6, singleNode/multiNode configurations
  
  - Podman tests: `.github/workflows/podman.yml`
  
  - nerdctl tests: `.github/workflows/nerdctl.yaml`
  
  - Website deployment: `.github/workflows/deploy-site.yml`
    - Triggers on: Pushes to main (kinder-site changes)
    - Uses: Astro GitHub Actions
    - Deploys to: GitHub Pages

**Package Distribution:**
- Homebrew (macOS)
  - Tap repository: `patrykquantumnomad/kinder`
  - Token: `HOMEBREW_TAP_TOKEN` (GitHub Actions secret)
- GitHub Releases (all platforms: Linux, macOS, Windows)

## Environment Configuration

**Required env vars (for cluster operations):**
- `KUBECONFIG` - Path to kubeconfig file (optional, defaults to `~/.kube/config`)
- Container runtime detection: Auto-detects from Docker/Podman/nerdctl availability

**For CI/CD (GitHub Actions):**
- `GITHUB_TOKEN` - GitHub Actions provided token (release automation)
- `HOMEBREW_TAP_TOKEN` - Personal access token for Homebrew tap updates

**Secrets location:**
- GitHub Actions Secrets (project-level)
  - `HOMEBREW_TAP_TOKEN` - For Homebrew cask updates

## Webhooks & Callbacks

**Incoming:**
- None

**Outgoing:**
- None direct webhooks; GoReleaser handles GitHub Release creation and Homebrew tap updates via API calls

## Kubernetes Add-ons (Pre-installed, No External Dependency)

These are packaged components deployed within clusters:

**MetalLB** - LoadBalancer service IP allocation
- Deployed to: `metallb-system` namespace
- Provides: Real external IPs from Docker/Podman subnet

**Envoy Gateway** - Gateway API implementation
- Deployed to: `envoy-gateway-system` namespace
- Provides: Advanced routing via `eg` GatewayClass

**Metrics Server** - Cluster metrics
- Provides: `kubectl top` and HPA support
- Deployed to: `kube-system` namespace

**CoreDNS Tuning** - Enhanced DNS resolution
- Autopath, verified pod records, doubled cache TTL

**Local Path Provisioner** - Dynamic storage
- Deployed to: `local-path-storage` namespace
- Provides: `local-path` StorageClass for persistent volumes

**Headlamp** - Web-based cluster dashboard
- Deployed to: `kube-system` namespace
- Access: Port-forward to service

**Local Registry** - Container image registry
- Pre-configured: `localhost:5001`
- For local image development without external registry

**cert-manager** - TLS certificate management
- Provides: Self-signed ClusterIssuer ready to use

**NVIDIA GPU** - GPU support (optional)
- Requires: Host has NVIDIA drivers
- Deployed to: `kube-system` namespace
- Passthrough for AI/ML workloads

## Website Integrations

**Content Management:**
- Markdown-based documentation (no CMS)
  - Source: `kinder-site/src/content/docs/`
  - Built with: Astro static site generator

**Analytics:**
- None integrated

**Deployment:**
- GitHub Pages (primary)
  - Built on: Node.js (Astro build)
  - Deployed via: `.github/workflows/deploy-site.yml`
  - URL: `https://kinder.patrykgolabek.dev`

**Legacy Deployment (obsolete):**
- Netlify (old site generator with Hugo)
  - Config: `netlify.toml`
  - Build: Hugo 0.111.3
  - Original URL: `https://kind.sigs.k8s.io/` (upstream kind, not used for kinder)

---

*Integration audit: 2026-05-03*
