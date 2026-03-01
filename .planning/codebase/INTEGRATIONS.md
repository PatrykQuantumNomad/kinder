# External Integrations

**Analysis Date:** 2026-03-01

## APIs & External Services

**Container Runtimes (Pluggable Providers):**
- Docker - Primary container runtime via CLI (`docker` commands)
  - Location: `pkg/cluster/internal/providers/docker/`
  - Integration: Executes docker CLI commands for container management

- Podman - OCI-compatible container runtime
  - Location: `pkg/cluster/internal/providers/podman/`
  - Integration: Podman API/CLI compatible with docker interface

- Nerdctl - containerd native CLI
  - Location: `pkg/cluster/internal/providers/nerdctl/`
  - Integration: Nerdctl/Finch CLI for containerd management
  - Fallback detection: Checks for both `nerdctl` and `finch` binaries

- Containerd - Container runtime daemon
  - Accessed via container runtime provider layers
  - Used for image management and container lifecycle

**Kubernetes Integration:**
- kubectl - Kubernetes CLI (assumes pre-installed on host)
  - Used in test workflows for cluster verification
  - Workflows: `.github/workflows/docker.yaml`, etc.

## Data Storage

**Databases:**
- Not applicable - Kind is a cluster provisioning tool, not a database-backed service

**File Storage:**
- Local filesystem only
  - Kubeconfig: `~/.kube/config` (configurable)
  - Cluster state: Docker/container filesystem
  - Node data: Persisted in container volumes
  - Logs: Host filesystem via `kind export logs` command

**Caching:**
- None - Kind does not require external caching systems

## Authentication & Identity

**Auth Provider:**
- Custom - Kind uses container runtime authentication
  - Docker: Uses Docker daemon's authentication (from ~/.docker/config.json)
  - Podman: Uses Podman's authentication credentials
  - Nerdctl: Uses containerd's authentication configuration
- Kubernetes: Uses generated kubeconfig with embedded certificates for API access

## Monitoring & Observability

**Error Tracking:**
- None - Kind is a CLI tool with local error reporting

**Logs:**
- Console output to stdout/stderr
- Log levels: Controlled by `-v` verbosity flag (0-7+ levels)
- Logger implementation: `pkg/log/logger.go` - custom log wrapper
- Export capability: `kind export logs <output-dir>` command
- Log output streaming from container runtimes

**Observability Integration:**
- Error stack traces: Logged to stderr with verbosity control
- Command execution output: Captured from container runtime CLI calls
- Location: `pkg/exec/` - Execution framework for capturing command output

## CI/CD & Deployment

**Hosting:**
- GitHub (source repository)
- Netlify - Static site hosting for documentation
  - Configuration: `netlify.toml`
  - Hugo build: Runs `hugo` in `site/` directory
  - Base URL: `https://kind.sigs.k8s.io/`

**CI Pipeline:**
- GitHub Actions - Automated testing and validation
- Workflows in `.github/workflows/`:
  - `docker.yaml` - Docker runtime testing (ubuntu-24.04, IPv4/IPv6, single/multi-node)
  - `podman.yml` - Podman runtime testing
  - `nerdctl.yaml` - Nerdctl/containerd runtime testing
  - `vm.yaml` - Virtual machine environments
- Test matrix: IPv4/IPv6, single-node/multi-node deployments

## Environment Configuration

**Required env vars:**
- `DOCKER_HOST` (optional) - Docker daemon connection string for remote Docker
- `KIND_KUBECONFIG_DIR` (optional) - Override kubeconfig storage location
- `HUGO_VERSION` (for site builds) - Set to 0.111.3 in netlify.toml

**Container runtime selection:**
- Auto-detection: Checks for available runtimes in order (Docker, Podman, Nerdctl)
- Manual selection via provider plugins in cluster configuration
- Runtime specified in `kind: Cluster` YAML via `provider` field

**Secrets location:**
- Not applicable - Kind is a local development tool

## Webhooks & Callbacks

**Incoming:**
- None - Kind is a CLI tool without HTTP endpoints

**Outgoing:**
- None - Kind does not make external API calls
- GitHub Actions webhooks (GitHub-managed, not kind-specific)

## Kubernetes API Integration

**Kubeconfig Generation:**
- Generated during cluster creation
- Location: `pkg/cluster/internal/kubeconfig/`
- Contains: Kubernetes API server certificate, CA, client credentials
- Embedded in config file with base64-encoded certificates

**Cluster Configuration:**
- Kind cluster spec: `kind: Cluster, apiVersion: kind.x-k8s.io/v1alpha4`
- Nodes specification: Control-plane and worker node definitions
- Networking: IP family (IPv4/IPv6) configuration
- Image pull policy: Configured per node
- Configuration parsing: `pkg/internal/apis/config/` for YAML unmarshaling

## Container Image Sources

**Base Images:**
- `docker.io/library/golang` - For build stages in Dockerfiles
- `debian:trixie-slim` - Base OS for kind node images
- Pre-built kind node images: Hosted on registry.k8s.io or docker.io
- Custom images: Can be loaded via `kind load docker-image` command

## Network Integration

**Provider Network Management:**
- Docker network: `kind` default bridge network for container connectivity
- Podman network: Podman CNI network configuration
- Nerdctl network: containerd network namespace setup
- Network classes: `pkg/cluster/internal/providers/*/network.go`

---

*Integration audit: 2026-03-01*
