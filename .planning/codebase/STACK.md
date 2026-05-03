# Technology Stack

**Analysis Date:** 2026-05-03

## Languages

**Primary:**
- Go 1.24+ (minimum language version) - CLI application and core runtime (`main.go`, `cmd/kind/`, `pkg/`)
- Go 1.25.7 (compiler version, pinned in `.go-version`) - Build toolchain

**Secondary:**
- TypeScript 5.9.3 - Website documentation (`kinder-site/`)
- YAML - Kubernetes manifests and configuration files
- Shell (Bash/sh) - Build scripts and CI/CD workflows

## Runtime

**Environment:**
- Linux, macOS, Windows (cross-platform binary support)
- Requires container runtime: Docker, Podman, or nerdctl

**Package Manager:**
- Go Modules (go.sum and go.mod)
- npm/Node.js 22 (for website)

## Frameworks

**Core:**
- Cobra v1.8.0 - CLI framework for command structure (`github.com/spf13/cobra`)
- pflag v1.0.5 - Command-line flag parsing (`github.com/spf13/pflag`)

**Website:**
- Astro 5.6.1 - Static site generation and documentation (`kinder-site/`)
- Starlight 0.37.6 - Documentation theme and component library (`@astrojs/starlight`)

**Build/Dev:**
- GoReleaser 2.14.1+ - Multi-platform binary building and release automation (`.goreleaser.yaml`)
- Hugo 0.111.3 - Legacy site generation (old `site/` directory, used by Netlify)
- Make - Build orchestration (`Makefile`)

## Key Dependencies

**Critical:**
- `al.essio.dev/pkg/shellescape` v1.5.1 - Shell command escaping
- `github.com/BurntSushi/toml` v1.4.0 - TOML configuration parsing
- `go.yaml.in/yaml/v3` v3.0.4 - YAML parsing for kind config
- `sigs.k8s.io/yaml` v1.4.0 - Kubernetes-specific YAML utilities
- `github.com/evanphx/json-patch/v5` v5.6.0 - JSON patch operations
- `github.com/pelletier/go-toml` v1.9.5 - TOML support
- `github.com/pkg/errors` v0.9.1 - Error handling utilities

**Infrastructure:**
- `golang.org/x/sync` v0.19.0 - Concurrency primitives (WaitGroup, sync patterns)
- `golang.org/x/sys` v0.41.0 - System-level interfaces (POSIX compliance)
- `github.com/mattn/go-isatty` v0.0.20 - Terminal detection for colored output

## Configuration

**Environment:**
- `.go-version` - Pinned Go compiler version (1.25.7)
- `.goreleaser.yaml` - Release build configuration with multi-platform targets (Linux/macOS/Windows, amd64/arm64)
- `go.mod` / `go.sum` - Go module dependency lock

**Build:**
- `Makefile` - Build targets for compilation, testing, linting, code generation
  - `make build` / `make kind` - Build CLI binary to `bin/kinder`
  - `make install` - Install binary to `$INSTALL_DIR` (default: Go bin dir)
  - `make test` / `make unit` / `make integration` - Run test suites
  - `make verify` / `make lint` / `make shellcheck` - Run linters and code checks
  - `make update` / `make generate` / `make gofmt` - Code generation and formatting
- `hack/build/` - Build infrastructure scripts
  - `gotoolchain.sh` - Go version management
  - `setup-go.sh` - PATH and Go environment setup
  - `goinstalldir.sh` - Installation directory detection

**Code Generation:**
- `hack/make-rules/update/generated.sh` - Generated code regeneration
- `hack/make-rules/update/gofmt.sh` - Code formatting enforcement

## Platform Requirements

**Development:**
- Go 1.24+ compiler
- Docker, Podman, or nerdctl (for container runtime operations)
- Make (for build orchestration)
- GNU tools (sed, awk, etc.) in build scripts
- Git (for commit hash embedding and changelog generation)
- Node.js 22 (for website development)

**Production:**
- Docker, Podman, or nerdctl as container runtime
- kubectl (for cluster interaction)
- Linux, macOS, or Windows host OS
- Optional: NVIDIA drivers (for GPU addon support)

## Build Flags

**Static Binaries:**
- `CGO_ENABLED=0` - Disable C Go extensions for portability
- `-trimpath` - Remove filesystem paths from binaries for reproducibility
- `-ldflags="-buildid= -w"` - Strip debug info and buildid for size reduction
- `-X` flags - Embed version information at build time (`gitCommit`, `gitCommitCount`)

## Testing Infrastructure

**Unit Tests:**
- Standard Go testing (no external framework)
- Test files: `*_test.go` pattern throughout `pkg/`
- Run with: `make unit` or `go test ./...`

**Integration Tests:**
- Container runtime integration tests
- Docker, Podman, nerdctl-specific workflows
- Run with: `make integration`
- GitHub Actions workflows for each runtime
  - `.github/workflows/docker.yaml`
  - `.github/workflows/podman.yml`
  - `.github/workflows/nerdctl.yaml`

**Race Detection:**
- `make test-race` - Runs race detector on cluster creation code
- Requires `CGO_ENABLED=1`

## Website Stack

**Build:**
- Node.js 22 (npm package manager)
- Astro 5.6.1
- TypeScript 5.9.3
- sharp 0.34.2 - Image optimization
- @astrojs/check 0.9.6 - TypeScript checking

**Deployment:**
- GitHub Pages (primary) - via `.github/workflows/deploy-site.yml`
- Netlify (legacy, uses `site/` directory with Hugo) - `netlify.toml`
- Site URL: `https://kinder.patrykgolabek.dev`

---

*Stack analysis: 2026-05-03*
