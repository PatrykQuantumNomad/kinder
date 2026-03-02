# Technology Stack

**Analysis Date:** 2026-03-02

## Languages

**Primary:**
- Go 1.17+ - Core CLI tool and cluster management logic (minimum language version requirement)
- Go 1.25.7 - Compiler version used for building binaries (`/.go-version`)

**Secondary:**
- JavaScript/Node.js 22 - Website (Astro framework)
- YAML - Configuration and manifests (Kubernetes YAML, Astro config)
- TOML - Configuration and Makefiles
- Markdown - Documentation

## Runtime

**Environment:**
- Go (compiled binaries, cross-platform support: Linux, macOS, Windows)
- Node.js 22 - For website development and building

**Package Manager:**
- Go Modules (go.mod/go.sum) - Primary project at `/Users/patrykattc/work/git/kinder/go.mod`
- Hugo modules - Legacy site (`/Users/patrykattc/work/git/kinder/site/go.mod`)
- npm - Website dependency management (`/Users/patrykattc/work/git/kinder/kinder-site/package.json`)

**Lockfiles:**
- `go.sum` - Go dependencies locked
- `package-lock.json` - npm dependencies locked (kinder-site)
- Both present and maintained

## Frameworks

**Core:**
- Cobra v1.8.0 - CLI command framework for kinder CLI
- pflag v1.0.5 - Command-line flag parsing

**Website:**
- Astro v5.6.1 - Static site generator for kinder documentation
- Starlight v0.37.6 - Documentation theme built on Astro
- Hugo v0.111.3 - Legacy site documentation (still hosted, being phased out)

**Build/Dev:**
- Make - Build orchestration (`/Users/patrykattc/work/git/kinder/Makefile`)
- GitHub Actions - CI/CD workflows (`.github/workflows/`)
- Netlify - Legacy site deployment configuration (`netlify.toml`)
- GitHub Pages - New site deployment

## Key Dependencies

**Critical (Go):**
- `sigs.k8s.io/yaml` v1.4.0 - YAML parsing for Kubernetes-style configs
- `github.com/spf13/cobra` v1.8.0 - CLI framework
- `github.com/BurntSushi/toml` v1.4.0 - TOML configuration parsing
- `github.com/pelletier/go-toml` v1.9.5 - TOML utilities
- `github.com/evanphx/json-patch/v5` v5.6.0 - JSON patch operations for kubeadm config modifications
- `go.yaml.in/yaml/v3` v3.0.4 - YAML marshaling/unmarshaling
- `github.com/pkg/errors` v0.9.1 - Error wrapping utilities

**Infrastructure (Go):**
- `al.essio.dev/pkg/shellescape` v1.5.1 - Shell escaping for safe command execution
- `github.com/mattn/go-isatty` v0.0.20 - Terminal detection for smart formatting
- No external cloud SDKs (Docker/Podman/nerdctl via exec, not SDK)

**Website (npm):**
- `astro` v5.6.1 - Core framework
- `@astrojs/starlight` v0.37.6 - Documentation components
- `sharp` v0.34.2 - Image optimization

## Configuration

**Environment:**
- `KIND_CLUSTER_NAME` - Override cluster name (default: "kind")
- `KIND_EXPERIMENTAL_PROVIDER` - Select container runtime (docker/podman/nerdctl)
- `KIND_EXPERIMENTAL_DOCKER_NETWORK` - Custom Docker network name
- `KIND_EXPERIMENTAL_PODMAN_NETWORK` - Custom Podman network name
- KUBECONFIG - Standard Kubernetes kubeconfig path
- Proxy environment variables (HTTP_PROXY, HTTPS_PROXY, NO_PROXY) - Passed through to cluster

**Build:**
- Makefile targets:
  - `make build` - Build kinder binary
  - `make install` - Install binary to INSTALL_DIR
  - `make test` - Run all tests
  - `make unit` - Unit tests only
  - `make integration` - Integration tests only
  - `make verify` - Linting and verification
  - `make lint` - Code linting
  - `make shellcheck` - Shell script linting

**Website Build:**
- Astro development: `npm run dev`
- Astro build: `npm run build`
- Astro preview: `npm run preview`
- Output: Static HTML to `/Users/patrykattc/work/git/kinder/kinder-site/dist/`

## Platform Requirements

**Development:**
- Go 1.17+ (minimum language version)
- Go 1.25.7+ (recommended compiler version)
- Make build tools
- Container runtime (Docker, Podman, or nerdctl) for testing
- Node.js 22+ (for website development)
- Optional: Shellcheck for shell script validation

**Production:**
- Linux, macOS, or Windows host with Docker, Podman, or nerdctl installed
- Kubernetes 1.x compatible cluster (created as local kind cluster)
- Static binary deployment (no Go runtime required on deployment)
- Website: Static HTML hosting (GitHub Pages or compatible CDN)

---

*Stack analysis: 2026-03-02*
