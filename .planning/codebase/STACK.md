# Technology Stack

**Analysis Date:** 2026-04-08

## Languages

**Primary:**
- Go 1.25.7 (compiler version) / 1.24.0 (minimum language version) - CLI tool and cluster management
- TypeScript 5.9.3 - Documentation site (Astro)

**Secondary:**
- Bash/Shell - Build scripts and tooling
- Dockerfile - Container image definitions

## Runtime

**Environment:**
- Go 1.26.0 (toolchain)
- Node.js (via package-lock.json) - Documentation site

**Package Manager:**
- Go modules (go.mod/go.sum)
- npm - JavaScript dependencies for documentation site

## Frameworks

**Core:**
- Cobra v1.8.0 - CLI framework for kind command structure
- Astro v5.6.1 - Static site generator for documentation
- @astrojs/starlight v0.37.6 - Documentation theme/framework

**Build/Dev:**
- GoReleaser v2.x - Binary release management and cross-platform compilation
- Make - Build automation

## Key Dependencies

**Critical:**
- github.com/spf13/cobra v1.8.0 - Command-line interface and subcommand handling
- github.com/spf13/pflag v1.0.5 - Flag parsing for CLI arguments
- sigs.k8s.io/yaml v1.4.0 - YAML parsing for Kubernetes manifests
- go.yaml.in/yaml/v3 v3.0.4 - YAML support

**Utilities:**
- github.com/BurntSushi/toml v1.4.0 - TOML configuration parsing
- github.com/evanphx/json-patch/v5 v5.6.0 - JSON patch operations
- github.com/pelletier/go-toml v1.9.5 - TOML parsing
- github.com/pkg/errors v0.9.1 - Error handling with stack traces
- github.com/mattn/go-isatty v0.0.20 - Terminal detection for color support
- golang.org/x/sync v0.19.0 - Concurrency primitives
- golang.org/x/sys v0.41.0 - System-level OS operations
- al.essio.dev/pkg/shellescape v1.5.1 - Shell escape utilities

**Documentation:**
- sharp v0.34.2 - Image processing for documentation assets
- @astrojs/check v0.9.6 - Type checking for Astro

## Configuration

**Environment:**
- `.go-version` - Specifies Go compiler version (1.25.7)
- `go.mod` - Module dependencies and minimum language version
- `astro.config.mjs` - Astro site configuration with Starlight theme
- `.goreleaser.yaml` - Release build configuration with cross-platform targeting
- `Makefile` - Build targets with CGO disabled for static binaries

**Build:**
- Cross-compilation targets: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- Static binary builds (CGO_ENABLED=0)
- Binary name: `kinder`
- Output directory: `bin/`

## Platform Requirements

**Development:**
- Go 1.25.7+ (compiler)
- Docker or compatible container runtime (Podman, nerdctl)
- Make
- Git (for version information)
- Node.js/npm (for documentation site)

**Production:**
- Docker Desktop / Docker Engine (primary)
- Podman (alternative container runtime)
- nerdctl (alternative container runtime)
- Linux, macOS, or Windows operating system
- Kubernetes API server endpoint accessibility

## Networking & Container Runtimes

**Supported Container Runtimes:**
- Docker - Primary provider (`pkg/cluster/internal/providers/docker`)
- Podman - Alternative provider (`pkg/cluster/internal/providers/podman`)
- nerdctl - Alternative provider (`pkg/cluster/internal/providers/nerdctl`)

**Container Communication:**
- Fixed network name: `kind` (configurable via `KIND_EXPERIMENTAL_DOCKER_NETWORK` env var)
- Kubernetes networking components preconfigured

## Release Management

**Version Control:**
- Git with semantic versioning (v*.*.* tags)
- GoReleaser for automated release builds
- GitHub Releases for distribution
- Homebrew tap integration for macOS package distribution

**Distribution Formats:**
- Linux/macOS: tar.gz archives
- Windows: zip archives
- SHA256 checksums for verification
- Binary-only archives (no LICENSE/README included)

**CI/CD Environment:**
- GitHub Actions (workflows in `.github/workflows/`)
- Ubuntu 24.04 runner for testing
- Multi-platform testing (single node and multi-node deployments)
- IPv4 and IPv6 testing variants
- Docker, Podman, and nerdctl testing workflows

---

*Stack analysis: 2026-04-08*
