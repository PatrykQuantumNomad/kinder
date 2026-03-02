# Codebase Structure

**Analysis Date:** 2026-03-02

## Directory Layout

```
kinder/
├── cmd/                           # Binary entry points
│   └── kind/
│       └── app/
│           ├── main.go            # App.Main() entry point
│           └── main_test.go
├── pkg/                           # Main package libraries
│   ├── apis/                      # Public API types (future versioning)
│   │   └── config/
│   │       ├── defaults/          # Default values
│   │       └── v1alpha4/          # Current version schema
│   ├── build/                     # Node image building
│   │   └── nodeimage/
│   │       └── internal/
│   ├── cluster/                   # Core cluster management
│   │   ├── constants/             # Constants (e.g., default names)
│   │   ├── internal/              # Private cluster implementation
│   │   │   ├── create/            # Cluster creation logic
│   │   │   │   └── actions/       # Setup actions (addons, bootstrap)
│   │   │   ├── delete/            # Cluster deletion logic
│   │   │   ├── kubeadm/           # Kubeadm configuration
│   │   │   ├── kubeconfig/        # Kubeconfig management
│   │   │   ├── loadbalancer/      # Load balancer setup
│   │   │   ├── logs/              # Log collection
│   │   │   └── providers/         # Container runtime providers
│   │   ├── nodes/                 # Node interface definition
│   │   └── nodeutils/             # Node utility functions
│   ├── cmd/                       # CLI infrastructure
│   │   ├── iostreams.go           # Standard IO handling
│   │   ├── logger.go              # Logger setup
│   │   └── kind/                  # Command tree
│   │       ├── root.go            # Root command
│   │       ├── build/             # Build subcommand
│   │       ├── completion/        # Shell completion
│   │       ├── create/            # Create subcommand
│   │       ├── delete/            # Delete subcommand
│   │       ├── export/            # Export subcommand
│   │       ├── get/               # Get subcommand
│   │       ├── load/              # Load subcommand
│   │       └── version/           # Version subcommand
│   ├── errors/                    # Error handling utilities
│   ├── exec/                      # Command execution
│   ├── fs/                        # File system operations
│   ├── internal/                  # Internal packages (not part of public API)
│   │   ├── apis/                  # Internal config types
│   │   │   └── config/
│   │   │       ├── encoding/      # YAML/JSON parsing
│   │   │       └── v1alpha4/      # Internal v1alpha4 schema
│   │   ├── assert/                # Assertion utilities for tests
│   │   ├── cli/                   # CLI utilities
│   │   ├── env/                   # Environment utilities
│   │   ├── integration/           # Integration test utilities
│   │   ├── patch/                 # Kubernetes patch utilities
│   │   ├── runtime/               # Runtime detection
│   │   ├── sets/                  # Set data structures
│   │   └── version/               # Version info
│   └── log/                       # Logging interface
├── main.go                        # Root entry point
├── Makefile                       # Build targets
├── go.mod                         # Go module definition
├── go.sum                         # Dependency checksums
└── [other files]                  # Docs, config, images
```

## Directory Purposes

**cmd/kind/app/ - Application Entry Point:**
- Purpose: Main binary entry point orchestration
- Contains: Main() function, Run() function, error logging, quiet mode handling
- Key files: `main.go` with app initialization logic

**pkg/cluster/ - Cluster Lifecycle Management:**
- Purpose: High-level cluster operations (create, delete, list)
- Contains: Provider interface usage, cluster options, public API
- Key files: `provider.go` (Provider type), `createoption.go` (builder options)

**pkg/cluster/internal/create/ - Cluster Creation Engine:**
- Purpose: Orchestrate ordered setup steps during cluster creation
- Contains: Create() function, action execution, addon coordination
- Key files: `create.go` (main orchestration), `create_addon_test.go` (addon tests)

**pkg/cluster/internal/create/actions/ - Cluster Setup Actions:**
- Purpose: Individual setup steps executed in sequence
- Contains: Action interface, ActionContext, concrete action implementations
- Action subdirectories: installmetallb, installenvoygw, installmetricsserver, installstorage, installdashboard, installcni, installcorednstuning, kubeadminit, kubeadmjoin, loadbalancer, waitforready, config
- Pattern: Each action is independent, receives ActionContext with logger, provider, config, nodes

**pkg/cluster/internal/providers/ - Container Runtime Abstraction:**
- Purpose: Abstract Docker/Podman/nerdctl operations
- Contains: Provider interface, provider implementations
- Subdirectories: docker, podman, nerdctl, common
- Key files: `provider.go` (interface), `docker/provider.go` (Docker implementation)

**pkg/internal/apis/config/ - Internal Configuration Schema:**
- Purpose: Type-safe internal representation of cluster configuration
- Contains: Cluster, Node, Networking, Addons types; validation logic
- Key files: `types.go` (type definitions), `encoding/` (YAML parsing)

**pkg/cmd/kind/ - CLI Command Tree:**
- Purpose: Cobra command definitions and flag parsing
- Contains: Root command, subcommands for each operation
- Subdirectories: create, delete, get, load, build, completion, export, version
- Pattern: Each subcommand in its own directory, NewCommand() factory function

**pkg/log/ - Logging Interface:**
- Purpose: Logger abstraction for testability
- Contains: Logger interface definition, implementations (Kubernetes klog compatible)
- Key files: `types.go` (interface), `noop.go` (no-op implementation)

**pkg/errors/ - Error Handling:**
- Purpose: Error wrapping, aggregation, stack traces
- Contains: Wrap, WithStack, Cause, Aggregate implementations
- Key files: `aggregate.go`, `concurrent.go`, error chain management

**pkg/exec/ - Command Execution:**
- Purpose: Abstract command execution on nodes
- Contains: Cmder interface, exec utilities, error types
- Used by: Node implementations, actions

**pkg/internal/cli/ - CLI Utilities:**
- Purpose: Status display, spinner, logger overrides
- Contains: Status struct for progress indication, spinner animation
- Key files: `status.go`, `spinner.go`, `override.go` (for --name flag)

**pkg/internal/runtime/ - Provider Detection:**
- Purpose: Detect available container runtime from environment
- Contains: GetDefault() function for KIND_EXPERIMENTAL_PROVIDER
- Returns: ProviderOption for Docker/Podman/nerdctl selection

## Key File Locations

**Entry Points:**
- `main.go` - Root binary entry point, delegates to app.Main()
- `cmd/kind/app/main.go` - app.Main() orchestrates Run(), handles exit codes
- `pkg/cmd/kind/root.go` - Root command with all subcommands registered
- `pkg/cmd/kind/create/cluster/createcluster.go` - Create cluster subcommand
- `pkg/cmd/kind/delete/cluster/deletecluster.go` - Delete cluster subcommand

**Configuration:**
- `pkg/internal/apis/config/types.go` - Cluster config type definitions
- `pkg/internal/apis/config/encoding/` - YAML/JSON config file parsing
- `pkg/apis/config/` - Public config API (currently mirrors internal)

**Core Logic:**
- `pkg/cluster/provider.go` - Provider type, cluster operations
- `pkg/cluster/internal/create/create.go` - Cluster creation orchestration
- `pkg/cluster/internal/providers/provider.go` - Provider interface
- `pkg/cluster/internal/create/actions/action.go` - Action interface and context

**Addon Installations:**
- `pkg/cluster/internal/create/actions/installmetallb/` - MetalLB LoadBalancer addon
- `pkg/cluster/internal/create/actions/installenvoygw/` - Envoy Gateway addon
- `pkg/cluster/internal/create/actions/installmetricsserver/` - Metrics Server addon
- `pkg/cluster/internal/create/actions/installdashboard/` - Headlamp Dashboard addon
- `pkg/cluster/internal/create/actions/installcni/` - CNI plugin installation
- `pkg/cluster/internal/create/actions/installstorage/` - Storage class installation
- `pkg/cluster/internal/create/actions/installcorednstuning/` - CoreDNS tuning

**Utilities:**
- `pkg/log/types.go` - Logger interface
- `pkg/cmd/iostreams.go` - IO streams type
- `pkg/errors/` - Error utilities
- `pkg/internal/cli/status.go` - Progress status display
- `pkg/internal/runtime/runtime.go` - Provider detection

## Naming Conventions

**Files:**
- `[description].go` - Standard Go source files, lowercase with underscores for multi-word names
- `[name]_test.go` - Test files corresponding to implementation files
- `doc.go` - Package documentation file
- `types.go` - Type definitions and interfaces
- `[verb][noun].go` - Action-oriented (e.g., `getkubeconfig.go`, `createsomething.go`)

**Directories:**
- `internal/` - Private packages not part of public API
- `cmd/` - Command-line interface and executable entry points
- `pkg/` - Library packages
- `[feature]/` - Feature-specific subdirectories (e.g., `cluster/`, `cmd/`, `build/`)
- `[feature]/internal/` - Internal implementation of a feature

**Packages:**
- `package main` - Only in cmd/kind/app/ and main.go
- `package kind` - Root command package
- `package [feature]` - Public feature packages
- `package [subfeature]` - Subfeatures within features

**Types and Interfaces:**
- `Capitalized` - Exported types (public API)
- `lowercase` - Unexported types (private to package)
- `[Name]Interface` - Exported interfaces (e.g., `Provider` not `ProviderInterface`)
- Struct methods: `(p *provider) MethodName()` - use pointer receiver type

**Functions:**
- `Capitalized()` - Exported functions (public)
- `lowercase()` - Unexported functions (private)
- `New[Type]()` - Constructor functions (e.g., `NewProvider()`)
- `[Verb][Noun]()` - Action-oriented names (e.g., `ListClusters()`)

## Where to Add New Code

**New Addon Installation:**
- Directory: `pkg/cluster/internal/create/actions/install[addonname]/`
- Files: `[addonname].go` with Kubernetes manifests in `manifests/` subdirectory
- Pattern: Create struct implementing Action interface, embed manifest via `//go:embed`, implement Execute() method
- Example: See `pkg/cluster/internal/create/actions/installmetallb/metallb.go`
- Integration: Add NewAction() call to actions list in `pkg/cluster/internal/create/create.go`
- Config: Add boolean field to `Addons` struct in `pkg/internal/apis/config/types.go`

**New Subcommand:**
- Directory: `pkg/cmd/kind/[commandname]/`
- Files: Create `[commandname].go` with NewCommand() factory function
- Pattern: Use Cobra, define flags, implement RunE callback
- Example: See `pkg/cmd/kind/create/cluster/createcluster.go`
- Integration: Add cmd.AddCommand() in `pkg/cmd/kind/root.go`

**New Provider (Container Runtime):**
- Directory: `pkg/cluster/internal/providers/[runtime]/`
- Files: `provider.go` implementing providers.Provider interface
- Methods: Provision(), ListClusters(), ListNodes(), DeleteNodes(), GetAPIServerEndpoint(), etc.
- Example: See `pkg/cluster/internal/providers/docker/`

**New Utility Package:**
- Location: `pkg/[utilname]/` or `pkg/internal/[utilname]/ ` (if internal)
- Pattern: Create doc.go with package documentation
- Exports: Only export public API surface

**New Tests:**
- Location: Same directory as code being tested
- Naming: `[filename]_test.go` corresponds to `[filename].go`
- Pattern: Use standard Go testing package (no special framework)
- Examples: See `*_test.go` files throughout

## Special Directories

**pkg/cluster/internal/create/actions/[action]/manifests/:**
- Purpose: Embedded Kubernetes manifests for addon installation
- Generated: No, manually created
- Committed: Yes, part of source control
- Format: YAML files, referenced via `//go:embed` in Go code
- Examples: MetalLB YAML in installmetallb/manifests/, Envoy Gateway in installenvoygw/manifests/

**pkg/internal/apis/config/encoding/testdata/:**
- Purpose: Test fixtures for config file parsing
- Generated: No, manually created test data
- Committed: Yes, test fixtures
- Format: YAML configuration examples
- Usage: Loaded by tests in `encoding/` package

**bin/:**
- Purpose: Built binary output directory
- Generated: Yes, created by build system
- Committed: No (.gitignore excludes)
- Format: Compiled executables

**hack/:**
- Purpose: Development and build scripts
- Generated: No, development utilities
- Committed: Yes
- Contents: Scripts for testing, building, code generation

**images/:**
- Purpose: Asset files and images for documentation
- Generated: No, manually maintained
- Committed: Yes

**.planning/:**
- Purpose: GSD planning documents and analysis
- Generated: Yes, created by GSD tools
- Committed: Yes (in .git)
- Contents: Phase plans, codebase analysis documents

---

*Structure analysis: 2026-03-02*
