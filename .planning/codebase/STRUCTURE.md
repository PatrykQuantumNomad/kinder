# Codebase Structure

**Analysis Date:** 2026-04-08

## Directory Layout

```
kinder/
├── main.go                           # Binary entry point (stub wrapping cmd/kind/app.Main)
├── go.mod                            # Go module definition
├── go.sum                            # Go module checksums
├── Makefile                          # Build targets for testing, linting, releases
├── .goreleaser.yaml                  # Release configuration for multi-platform builds
│
├── cmd/
│   └── kind/
│       ├── app/
│       │   ├── main.go               # App entry point (logger, streams, Run invocation)
│       │   └── main_test.go
│       └── main.go                   # Kubernetes authors copyright notice
│
├── pkg/
│   ├── cmd/                          # CLI command implementations via Cobra
│   │   ├── kind/                     # Root command and subcommands
│   │   │   ├── root.go               # Root command definition, subcommand registration
│   │   │   ├── build/                # `kinder build` subcommand
│   │   │   ├── create/               # `kinder create` subcommand hierarchy
│   │   │   │   └── cluster/          # `kinder create cluster` implementation
│   │   │   ├── delete/               # `kinder delete` subcommand hierarchy
│   │   │   ├── get/                  # `kinder get` subcommand hierarchy
│   │   │   ├── export/               # `kinder export` subcommand hierarchy
│   │   │   ├── load/                 # `kinder load` subcommand hierarchy
│   │   │   ├── version/              # `kinder version` subcommand
│   │   │   ├── completion/           # Shell completion generators
│   │   │   ├── env/                  # `kinder env` subcommand
│   │   │   └── doctor/               # `kinder doctor` diagnostic command
│   │   ├── iostreams.go              # I/O stream abstraction (Out, ErrOut)
│   │   └── logger.go                 # Logger setup and color utilities
│   │
│   ├── cluster/                      # Core cluster operations API
│   │   ├── provider.go               # Provider struct, public cluster lifecycle methods
│   │   ├── createoption.go           # ProviderOption and builder pattern for provider setup
│   │   ├── nodes/                    # Node type definitions
│   │   │   ├── types.go              # Node struct, role constants
│   │   │   └── doc.go
│   │   ├── nodeutils/                # Node role detection and utilities
│   │   │   ├── roles.go              # Control plane vs worker role detection
│   │   │   └── util.go
│   │   ├── constants/                # Cluster-level constants
│   │   │   └── constants.go          # Default cluster name, network CIDR
│   │   └── internal/                 # Implementation details (not part of public API)
│   │       ├── providers/            # Container runtime provider implementations
│   │       │   ├── common/           # Shared utilities across providers
│   │       │   │   └── utils.go      # Image pulling, network utilities
│   │       │   ├── docker/           # Docker provider implementation
│   │       │   │   ├── provider.go   # Docker-specific provider methods
│   │       │   │   ├── create.go     # Node creation via docker
│   │       │   │   └── network.go    # Docker network management
│   │       │   ├── nerdctl/          # Nerdctl provider implementation
│   │       │   │   ├── provider.go   # Nerdctl-specific provider methods
│   │       │   │   └── create.go     # Node creation via nerdctl
│   │       │   └── podman/           # Podman provider implementation
│   │       ├── create/               # Cluster creation orchestration
│   │       │   ├── create.go         # Main creation logic, action sequencing
│   │       │   └── actions/          # Composable creation actions
│   │       │       ├── action.go     # Action interface definition
│   │       │       ├── config/       # Apply kubeadm/containerd config patches
│   │       │       ├── kubeadminit/  # Initialize control plane with kubeadm
│   │       │       ├── kubeadmjoin/  # Join worker nodes to cluster
│   │       │       ├── installcni/   # Install CNI plugin (Kindnetd)
│   │       │       ├── installmetallb/   # Install MetalLB addon
│   │       │       ├── installdashboard/ # Install Kubernetes dashboard addon
│   │       │       ├── installstorage/   # Configure storage provisioner
│   │       │       ├── installmetricsserver/ # Install metrics-server
│   │       │       ├── installcertmanager/  # Install cert-manager addon
│   │       │       ├── installenvoygw/     # Install Envoy Gateway addon
│   │       │       ├── installcorednstuning/ # CoreDNS configuration
│   │       │       ├── installnvidiagpu/    # NVIDIA GPU support addon
│   │       │       ├── installlocalregistry/ # Local image registry addon
│   │       │       ├── loadbalancer/        # Load balancer node setup
│   │       │       └── waitforready/        # Wait for cluster readiness
│   │       ├── delete/               # Cluster deletion logic
│   │       │   └── delete.go         # Clean up nodes and resources
│   │       ├── kubeconfig/           # Kubeconfig generation
│   │       │   └── kubeconfig.go     # Generate and merge kubeconfig files
│   │       ├── kubeadm/              # Kubeadm configuration generation
│   │       ├── loadbalancer/         # Load balancer for HA control planes
│   │       └── logs/                 # Log export functionality
│   │
│   ├── build/                        # Node image building
│   │   └── nodeimage/
│   │       ├── build.go              # Main build entry point
│   │       ├── buildcontext.go       # Build context and options
│   │       ├── internal/
│   │       │   └── kube/             # Kubernetes binary integration
│   │       ├── containerd.go         # Containerd configuration
│   │       ├── defaults.go           # Default image and base image constants
│   │       └── options.go            # BuildOption pattern for customization
│   │
│   ├── apis/                         # Configuration API definitions
│   │   └── config/
│   │       ├── v1alpha4/             # API version 1alpha4 (current)
│   │       │   ├── types.go          # Cluster, Node, Networking, Addon types
│   │       │   ├── default.go        # Default value assignment
│   │       │   └── zz_generated.deepcopy.go # Code-generated copy methods
│   │       └── defaults/             # Defaults for Cluster structure
│   │
│   ├── exec/                         # Command execution abstraction
│   │   ├── types.go                  # Executor interface definition
│   │   ├── local.go                  # Local shell command execution
│   │   └── default.go                # Default executor initialization
│   │
│   ├── log/                          # Logging interfaces and utilities
│   │   ├── types.go                  # Logger and InfoLogger interfaces
│   │   └── noop.go                   # No-operation logger for silent mode
│   │
│   ├── errors/                       # Error handling utilities
│   │   ├── errors.go                 # Stack trace wrapping
│   │   ├── aggregate.go              # Multi-error collection
│   │   ├── concurrent.go             # Concurrent error tracking
│   │   └── *_test.go                 # Error type tests
│   │
│   ├── fs/                           # Filesystem utilities
│   │   └── fs.go                     # Basic file operations
│   │
│   └── internal/                     # Internal utilities (not public API)
│       ├── apis/                     # Internal API structures
│       │   └── config/               # Internal config with defaults applied
│       ├── doctor/                   # Cluster diagnostics
│       ├── kindversion/              # Version information
│       ├── patch/                    # Patch application utilities
│       ├── cli/                      # CLI utilities
│       ├── runtime/                  # Runtime detection
│       ├── sets/                     # Data structure utilities
│       └── version/                  # Version detection
│
├── kinder-site/                      # Documentation website (Astro)
│   ├── astro.config.mjs              # Astro configuration
│   ├── package.json                  # Dependencies: Astro, Starlight
│   ├── src/
│   │   ├── content/
│   │   │   └── docs/                 # Markdown documentation pages
│   │   │       ├── index.mdx         # Home page
│   │   │       ├── installation.md
│   │   │       ├── quick-start.md
│   │   │       ├── configuration.md
│   │   │       ├── known-issues.md
│   │   │       ├── changelog.md
│   │   │       ├── cli-reference/    # Command reference pages
│   │   │       ├── guides/           # How-to guides
│   │   │       └── addons/           # Addon documentation
│   │   ├── components/               # Astro components
│   │   │   ├── Comparison.astro
│   │   │   ├── InstallCommand.astro
│   │   │   └── ThemeSelect.astro
│   │   ├── assets/                   # Images and logos
│   │   ├── styles/                   # CSS theme
│   │   └── pages/                    # Custom pages (404)
│   └── public/                       # Static assets
│
├── hack/                             # Build and test scripts
│   ├── build/                        # Build scripts
│   ├── ci/                           # CI/CD scripts
│   ├── make-rules/                   # Makefile rules
│   ├── release/                      # Release automation
│   ├── tools/                        # Code generation tools
│   └── verify-integration.sh         # Integration test verification
│
├── images/                           # Container image definitions
│   ├── base/                         # Base node image
│   │   ├── Dockerfile
│   │   └── files/                    # Image files and scripts
│   ├── haproxy/                      # HAProxy load balancer image
│   ├── kindnetd/                     # CNI daemon image
│   └── kindnvidiagpu/                # NVIDIA GPU support image
│
├── .github/                          # GitHub configuration
│   └── workflows/                    # CI/CD workflows
│
├── .planning/                        # GSD (Get Shit Done) planning directory
│   ├── codebase/                     # Architecture/structure documentation
│   └── [project files]/              # Project phases, milestones, etc.
│
└── dist/                             # Built binaries (generated by release process)
    └── [platform builds]/            # Multi-platform binary artifacts
```

## Directory Purposes

**`cmd/kind/app/`:**
- Purpose: Application initialization and entry point
- Contains: Logger setup, I/O stream routing, clean error handling with stack traces
- Key files: `main.go` (App.Main → Run), `main_test.go`

**`pkg/cmd/kind/`:**
- Purpose: CLI command implementations using Cobra framework
- Contains: Root command definition, all subcommand hierarchies (create, delete, get, etc.)
- Key files: `root.go`, command implementations per subdirectory

**`pkg/cluster/`:**
- Purpose: Public API for cluster lifecycle operations
- Contains: Provider struct, public methods (Create, Delete, ListNodes, etc.)
- Key files: `provider.go`, options pattern via `createoption.go`

**`pkg/cluster/internal/`:**
- Purpose: Implementation details for cluster operations (not exposed in public API)
- Contains: Runtime providers, creation actions, kubeconfig generation, deletion logic
- Key files: Provider implementations, action implementations, orchestration logic

**`pkg/cluster/internal/providers/`:**
- Purpose: Pluggable container runtime backends
- Contains: Docker, Podman, Nerdctl implementations with shared utilities
- Key files: `docker/provider.go`, `podman/provider.go`, `nerdctl/provider.go`, `common/utils.go`

**`pkg/cluster/internal/create/`:**
- Purpose: Cluster creation orchestration and action sequencing
- Contains: Creation flow logic, action interface, addon action implementations
- Key files: `create.go` (main logic), `actions/` (individual setup steps)

**`pkg/cluster/internal/create/actions/`:**
- Purpose: Composable steps executed during cluster creation
- Contains: Individual addon installers, kubeadm init/join, config patching
- Key files: Each addon has `{addon}.go` with Execute() implementation

**`pkg/apis/config/v1alpha4/`:**
- Purpose: Cluster configuration schema and type definitions
- Contains: Cluster, Node, Networking, Addon struct definitions with YAML tags
- Key files: `types.go` (all types), `default.go` (default values), `zz_generated.deepcopy.go` (code-generated)

**`pkg/build/nodeimage/`:**
- Purpose: Build Kubernetes node images for cluster creation
- Contains: Image build logic, base image selection, builder pattern options
- Key files: `build.go` (main entry), `buildcontext.go` (context), `options.go` (customization)

**`pkg/log/`:**
- Purpose: Abstract logging interface for consistent output across layers
- Contains: Logger interface, InfoLogger interface, no-op implementation
- Key files: `types.go` (interfaces), `noop.go` (no-op logger)

**`pkg/exec/`:**
- Purpose: Abstract local command execution
- Contains: Executor interface, local command runner, error handling
- Key files: `types.go` (interface), `local.go` (implementation)

**`pkg/errors/`:**
- Purpose: Error handling utilities with stack trace support
- Contains: Error wrapping, aggregation of multiple errors, concurrent error tracking
- Key files: `errors.go`, `aggregate.go`, `concurrent.go`

**`pkg/internal/`:**
- Purpose: Internal utilities not part of public API
- Contains: Config encoding/decoding, version detection, patch utilities, diagnostics
- Key files: `kindversion/`, `cli/`, `doctor/`, `patch/`

**`kinder-site/`:**
- Purpose: Documentation website built with Astro + Starlight
- Contains: Markdown docs, components, asset files, build configuration
- Key files: `astro.config.mjs`, `package.json`, `src/content/docs/`

**`hack/`:**
- Purpose: Build automation, testing scripts, CI integration
- Contains: Make rules, test runners, release scripts, tool installation
- Key files: `make-rules/test.sh`, `ci/e2e.sh`, `release/create.sh`

**`images/`:**
- Purpose: Container image definitions for cluster nodes and addons
- Contains: Dockerfiles for base node image, CNI daemon, load balancer, GPU support
- Key files: `base/Dockerfile`, `kindnetd/Dockerfile`, `haproxy/Dockerfile`

## Key File Locations

**Entry Points:**
- `main.go`: Binary stub entry point
- `cmd/kind/app/main.go`: App initialization (logger, streams, Run)
- `pkg/cmd/kind/root.go`: Root Cobra command with subcommand registration

**Configuration:**
- `pkg/apis/config/v1alpha4/types.go`: Cluster schema definition
- `pkg/apis/config/v1alpha4/default.go`: Default value application
- `pkg/cluster/createoption.go`: Provider builder pattern options

**Core Logic:**
- `pkg/cluster/provider.go`: Public Provider API with lifecycle methods
- `pkg/cluster/internal/create/create.go`: Creation orchestration and action sequencing
- `pkg/cluster/internal/providers/*/provider.go`: Container runtime implementations

**Testing:**
- `*_test.go` files co-located with implementation
- Examples: `pkg/cmd/kind/delete/clusters/deleteclusters_test.go`, `pkg/build/nodeimage/internal/*_test.go`
- Test utilities in `pkg/internal/assert/`, `pkg/cluster/internal/create/actions/testutil/`

## Naming Conventions

**Files:**
- Implementation files: `{feature}.go` (e.g., `create.go`, `provider.go`)
- Test files: `{feature}_test.go` (e.g., `create_test.go`)
- Code-generated files: `zz_generated.{purpose}.go` (e.g., `zz_generated.deepcopy.go`)
- Constants: often in dedicated files like `constants.go`, `const.go`, `defaults.go`

**Directories:**
- Feature-based organization: `{action}` for major subsystems (cluster, build, cmd)
- Subcommand hierarchy mirrors CLI structure: `pkg/cmd/kind/{verb}/{noun}/`
- Addon actions named: `install{AddonName}` (e.g., `installmetallb`, `installcni`)
- Internal packages marked with `internal/` to prevent external import

**Functions & Types:**
- Public types capitalized: `Cluster`, `Provider`, `Node`, `Logger`
- Public functions capitalized: `NewProvider()`, `Create()`, `Delete()`
- Private functions lowercase: `runE()`, `checkQuiet()`, `defaultName()`
- Option functions follow builder pattern: `ProviderWithDocker()`, `WithLogger()`

## Where to Add New Code

**New CLI Command/Subcommand:**
- Implementation: `pkg/cmd/kind/{verb}/{noun}/{noun}.go`
- Test: `pkg/cmd/kind/{verb}/{noun}/{noun}_test.go`
- Register: Add `cmd.AddCommand(...)` in parent command's NewCommand()
- Example: For `kinder get nodes` → `pkg/cmd/kind/get/nodes/nodes.go`

**New Addon:**
- Implementation: `pkg/cluster/internal/create/actions/install{AddonName}/{addon}.go`
- Config: Update `pkg/apis/config/v1alpha4/types.go` Addons struct
- Manifest templates: Place YAML templates in `pkg/cluster/internal/create/actions/install{AddonName}/manifests/`
- Action registration: Add action.Action to sequence in `pkg/cluster/internal/create/create.go`

**New Container Runtime Provider:**
- Implementation: Create `pkg/cluster/internal/providers/{runtime}/` directory
- Required methods: Create, KillContainers, ListNodes, GetNode, ExecNode, etc. (see Provider interface)
- Shared utilities: Use/extend `pkg/cluster/internal/providers/common/utils.go`
- Registration: Add detection in `pkg/cluster/provider.go` DetectNodeProvider()

**New Configuration Option:**
- Schema: Add field to appropriate struct in `pkg/apis/config/v1alpha4/types.go`
- Defaults: Add default value logic in `pkg/apis/config/v1alpha4/default.go`
- Validation: Add validation in config encoding layer if needed
- CLI flag: Add flag to command in `pkg/cmd/kind/create/cluster/createcluster.go` if user-exposed

**Utilities & Helpers:**
- Shared across packages: `pkg/{feature}/` at top level
- Internal to subsystem: `pkg/{feature}/internal/{utility}/`
- Public logging: Use injected `log.Logger` interface from `pkg/log/`
- Execution: Use `pkg/exec/` Executor interface instead of direct os/exec

## Special Directories

**`pkg/internal/`:**
- Purpose: Internal implementation details not part of public API
- Generated: Some files are code-generated (e.g., deepcopy)
- Committed: All files committed to repository
- Not for external import

**`.planning/`:**
- Purpose: GSD (Get Shit Done) project planning and tracking
- Generated: Phase documents, milestones, roadmaps
- Committed: Yes, planning artifacts tracked in git
- Structure: Mirrors GSD workflow with milestones, phases, research docs

**`dist/`:**
- Purpose: Multi-platform compiled binaries
- Generated: Yes, created during release build process
- Committed: No, .gitignore excludes
- Contents: Platform-specific binaries (darwin, linux, windows)

**`hack/`:**
- Purpose: Development automation and build tooling
- Generated: Some output (e.g., coverage reports)
- Committed: Scripts and tools committed
- Not for distribution

**`images/`:**
- Purpose: Container image sources (separate from binaries)
- Generated: Images built via docker/podman, artifacts stored in image registry
- Committed: Dockerfiles and image configs committed
- Note: Images pushed to container registry, not stored in git

**`node_modules/` (kinder-site/):**
- Purpose: npm dependencies for documentation site
- Generated: Created by `npm install`
- Committed: No, .gitignore excludes
- For: Astro build and development

---

*Structure analysis: 2026-04-08*
