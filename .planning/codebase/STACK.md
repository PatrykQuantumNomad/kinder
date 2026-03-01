# Technology Stack

**Analysis Date:** 2026-03-01

## Languages

**Primary:**
- Go 1.17+ (minimum language version) - Core kind binary and cluster management
- Go 1.25.7 (compiler version, defined in `.go-version`) - Used for builds and releases

**Secondary:**
- Bash/Shell - Build scripts in `hack/` directory
- Dockerfile/containerfile - Container image definitions in `images/`
- Hugo - Static site generation for documentation

## Runtime

**Environment:**
- Linux (primary), macOS, Windows (WSL2) - Host system for kind binary
- Docker (default), containerd, Podman, Nerdctl - Container runtimes supported by kind
- Kubernetes 1.20+ - Target environment for created clusters

**Package Manager:**
- Go Modules (go.mod/go.sum)
- Lockfile: Present (`go.sum`)

## Frameworks

**Core:**
- Cobra v1.8.0 - CLI command framework for kind binary
- Spf13/pflag v1.0.5 - Command-line flag parsing
- Spf13/cobra v1.8.0 - CLI framework with subcommands

**Development/Build:**
- Golangci-lint v1.62.2 - Code linting (in `hack/tools/go.mod`)
- Gotestsum v1.12.0 - Test result formatting
- k8s.io/code-generator v0.31.0 - Kubernetes code generation

**Documentation:**
- Hugo v0.111.3 - Static site generator for `site/` (in `site/go.mod`)

## Key Dependencies

**Critical:**
- github.com/BurntSushi/toml v1.4.0 - TOML parsing for configuration files
- go.yaml.in/yaml/v3 v3.0.4 - YAML parsing for cluster configuration
- sigs.k8s.io/yaml v1.4.0 - Kubernetes YAML utilities
- github.com/evanphx/json-patch/v5 v5.6.0 - JSON patch operations for cluster updates
- github.com/spf13/cobra v1.8.0 - Command-line interface framework

**System Integration:**
- github.com/pkg/errors v0.9.1 - Error handling with stack traces
- github.com/mattn/go-isatty v0.0.20 - Terminal/TTY detection for color output
- golang.org/x/sys v0.6.0 - System-level utilities (file descriptors, signals)
- al.essio.dev/pkg/shellescape v1.5.1 - Shell script escaping for safe command execution

## Configuration

**Environment:**
- Cluster configuration via YAML files - `kind: Cluster` CRD format
- Kubeconfig management - Stored in `~/.kube/config` by default
- Environment variables for container runtime selection (DOCKER_HOST, etc.)

**Build:**
- `Makefile` - Primary build orchestration
- `hack/build/` - Build scripts for Go version setup and compilation
- `hack/make-rules/` - Modular make rules for testing, linting, updates
- `.go-version` - Go compiler version specification

## Platform Requirements

**Development:**
- Go 1.25.7+ (compiler version from `.go-version`)
- Make
- Docker/Podman/Nerdctl/Containerd (for container runtime testing)
- Git

**Production:**
- Container runtime on host:
  - Docker 20.10+
  - Podman 3.0+
  - Nerdctl/Finch 0.20+
  - Containerd 1.6+
- Linux kernel 4.15+ or equivalent macOS/Windows versions
- ~2GB RAM minimum per cluster node

---

*Stack analysis: 2026-03-01*
