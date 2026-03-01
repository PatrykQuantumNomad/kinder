# Codebase Structure

**Analysis Date:** 2026-03-01

## Directory Layout

```
kinder/
├── cmd/                       # Binary entry points and application startup
│   └── kind/
│       ├── main.go            # Binary stub
│       └── app/
│           ├── main.go        # Application main with logger/streams init
│           └── main_test.go   # Tests for app startup
├── pkg/                       # Core packages (public and internal APIs)
│   ├── apis/                  # Public API definitions
│   │   └── config/
│   │       ├── defaults/      # Default config values
│   │       └── v1alpha4/      # v1alpha4 config schema (types, defaults, YAML)
│   ├── build/                 # Node image building
│   │   └── nodeimage/
│   │       ├── internal/
│   │       │   ├── container/ # Container runtime abstraction
│   │       │   │   └── docker/
│   │       │   └── kube/      # Kubernetes binary acquisition
│   │       ├── options.go     # Builder configuration
│   │       └── build.go       # Main build logic
│   ├── cluster/               # Cluster operations (public API)
│   │   ├── provider.go        # Public cluster provider interface
│   │   ├── createoption.go    # Cluster creation options
│   │   ├── constants/         # Cluster constants
│   │   ├── nodes/             # Node types and interfaces
│   │   ├── nodeutils/         # Node utility functions
│   │   └── internal/          # Internal cluster implementation
│   │       ├── create/        # Cluster creation logic
│   │       │   ├── create.go  # Main creation orchestrator
│   │       │   └── actions/   # Creation action pipeline
│   │       │       ├── action.go      # Action interface
│   │       │       ├── config/        # Config action
│   │       │       ├── kubeadminit/   # Kubeadm init action
│   │       │       ├── kubeadmjoin/   # Kubeadm join action
│   │       │       ├── installcni/    # CNI installation action
│   │       │       ├── installstorage/# Storage installation action
│   │       │       ├── loadbalancer/  # Load balancer action (HA)
│   │       │       └── waitforready/  # Readiness check action
│   │       ├── delete/        # Cluster deletion logic
│   │       ├── providers/     # Provider implementations (Docker/Podman/Nerdctl)
│   │       │   ├── provider.go        # Provider interface definition
│   │       │   ├── common/            # Shared provider utilities
│   │       │   ├── docker/            # Docker provider
│   │       │   ├── podman/            # Podman provider
│   │       │   └── nerdctl/           # Nerdctl provider
│   │       ├── kubeconfig/    # Kubeconfig extraction/handling
│   │       ├── kubeadm/       # Kubeadm configuration management
│   │       ├── loadbalancer/  # Load balancer setup for HA
│   │       └── logs/          # Log collection from nodes
│   ├── cmd/                   # CLI command implementations
│   │   ├── logger.go          # Logger factory
│   │   ├── iostreams.go       # IO stream types
│   │   └── kind/              # Root and subcommands
│   │       ├── root.go        # Root command + subcommand registration
│   │       ├── create/        # Create command
│   │       │   └── cluster/   # Create cluster subcommand
│   │       ├── delete/        # Delete command
│   │       │   ├── cluster/   # Delete cluster subcommand
│   │       │   └── clusters/  # Delete all clusters subcommand
│   │       ├── get/           # Get command
│   │       │   ├── clusters/  # List clusters subcommand
│   │       │   ├── nodes/     # List nodes subcommand
│   │       │   └── kubeconfig/# Get kubeconfig subcommand
│   │       ├── load/          # Load command
│   │       │   ├── docker-image/      # Load docker image subcommand
│   │       │   └── image-archive/     # Load image archive subcommand
│   │       ├── build/         # Build command
│   │       │   └── nodeimage/ # Build node image subcommand
│   │       ├── export/        # Export command
│   │       │   ├── kubeconfig/# Export kubeconfig subcommand
│   │       │   └── logs/      # Export logs subcommand
│   │       ├── version/       # Version command
│   │       └── completion/    # Shell completion
│   │           ├── bash/      # Bash completion
│   │           ├── zsh/       # Zsh completion
│   │           ├── fish/      # Fish completion
│   │           └── powershell/# PowerShell completion
│   ├── errors/                # Error handling utilities
│   ├── exec/                  # Command execution wrapper
│   ├── fs/                    # File system utilities
│   ├── log/                   # Logging interface and implementations
│   │   ├── types.go           # Logger and InfoLogger interfaces
│   │   ├── noop.go            # No-op logger implementation
│   │   └── doc.go             # Package documentation
│   └── internal/              # Internal utilities (not part of public API)
│       ├── apis/              # Internal config APIs (different from public)
│       │   ├── config/        # Internal config types and encoding
│       │   │   └── encoding/  # YAML/JSON encoding/decoding
│       │   │       └── testdata/ # Test data for encodings
│       │   └── patch/         # Kubeadm patch handling
│       ├── assert/            # Test assertions
│       ├── cli/               # CLI utilities (status, progress)
│       ├── env/               # Environment utilities
│       ├── integration/       # Integration test helpers
│       ├── runtime/           # Runtime detection
│       ├── sets/              # Set data structure
│       └── version/           # Version utilities
├── hack/                      # Development and build tools
│   ├── build/                 # Build scripts
│   ├── ci/                    # CI/CD scripts
│   ├── make-rules/            # Makefile rules
│   │   ├── update/            # Update scripts
│   │   └── verify/            # Verification scripts
│   ├── release/               # Release scripts
│   │   └── build/             # Release build scripts
│   ├── tools/                 # Tool definitions
│   └── third_party/           # Third-party tools
├── images/                    # Container image definitions
│   ├── base/                  # Base node image
│   │   ├── Dockerfile        # (implied)
│   │   ├── files/            # Files to include in image
│   │   └── scripts/          # Setup scripts
│   ├── node/                  # Kubernetes node image
│   ├── kindnetd/              # Networking daemon image
│   │   ├── cmd/kindnetd/     # CNI setup binary source
│   │   ├── files/            # Image files
│   │   └── scripts/          # Setup scripts
│   ├── haproxy/               # HAProxy load balancer image
│   ├── local-path-provisioner/# Local path storage provisioner
│   └── local-path-helper/     # Local path provisioning helper
├── site/                      # Documentation website
│   ├── content/docs/          # Documentation content
│   ├── layouts/               # Hugo layouts
│   ├── assets/                # CSS/JS assets
│   ├── static/                # Static files
│   └── data/                  # Hugo data files
├── logo/                      # Project logo files
├── main.go                    # Binary entry point
├── Makefile                   # Build and development targets
├── go.mod                     # Go module definition
├── go.sum                     # Go module checksums
├── README.md                  # Project readme
├── CONTRIBUTING.md            # Contributing guidelines
├── LICENSE                    # Apache 2.0 license
└── SECURITY_CONTACTS          # Security contact info
```

## Directory Purposes

**cmd/kind/app/:**
- Purpose: Application initialization and startup
- Contains: Main function that initializes logging and IO streams
- Key files: `main.go` (entry point logic), `main_test.go` (startup tests)

**pkg/apis/config/v1alpha4/:**
- Purpose: Public cluster configuration schema with versioning
- Contains: Cluster, Node, Networking type definitions; defaults; YAML marshaling
- Key files: `types.go` (schema), `default.go` (default values), `yaml.go` (serialization)

**pkg/cluster/:**
- Purpose: Public API for cluster operations
- Contains: Provider facade, creation options, cluster interfaces
- Key files: `provider.go` (main public API)

**pkg/cluster/internal/create/:**
- Purpose: Multi-step cluster creation orchestration
- Contains: Creation flow, action pipeline, status tracking
- Key files: `create.go` (main orchestrator), `actions/action.go` (action interface)

**pkg/cluster/internal/providers/:**
- Purpose: Container runtime abstractions and implementations
- Contains: Provider interface, Docker/Podman/Nerdctl implementations, common utilities
- Key files: `provider.go` (interface), `{docker,podman,nerdctl}/provider.go` (implementations)

**pkg/cluster/internal/providers/common/:**
- Purpose: Shared functionality across all provider implementations
- Contains: Network management, image handling, port utilities, naming
- Key files: All providers import from here

**pkg/cmd/kind/:**
- Purpose: CLI command structure and handlers
- Contains: Root command, all subcommands with their handlers
- Key files: `root.go` (command registration), each subdir implements one command

**pkg/build/nodeimage/:**
- Purpose: Custom Kubernetes node image building
- Contains: Build orchestration, builder strategies (release/file/URL)
- Key files: `build.go` (main build logic), `options.go` (configuration)

**pkg/internal/apis/config/:**
- Purpose: Internal configuration handling separate from public API
- Contains: Config decoding, validation, patching, encoding
- Key files: `encoding/` (YAML/JSON handling)

**pkg/log/:**
- Purpose: Logging interface definition
- Contains: Logger interface, InfoLogger interface, no-op implementation
- Key files: `types.go` (interfaces), `noop.go` (default implementation)

**hack/:**
- Purpose: Development automation
- Contains: Build scripts, CI configuration, release tools, code generation
- Key files: Makefile targets reference these

**images/:**
- Purpose: Container image source definitions
- Contains: Dockerfiles and setup scripts for kind images
- Key files: `{base,kindnetd,haproxy,local-path-provisioner}/Dockerfile` (implied or scripts/)

## Key File Locations

**Entry Points:**
- `main.go`: Binary stub entry point (calls `app.Main()`)
- `cmd/kind/app/main.go`: Application entry point (initializes logger/streams)
- `pkg/cmd/kind/root.go`: CLI root command and subcommand registration
- `pkg/cluster/provider.go`: Public cluster API entry point

**Configuration:**
- `pkg/apis/config/v1alpha4/types.go`: Cluster configuration schema
- `pkg/apis/config/v1alpha4/default.go`: Default config values
- `pkg/internal/apis/config/encoding/`: Config file encoding/decoding

**Core Logic:**
- `pkg/cluster/internal/create/create.go`: Cluster creation orchestration
- `pkg/cluster/internal/create/actions/`: Action pipeline implementations
- `pkg/cluster/internal/providers/provider.go`: Provider interface
- `pkg/cluster/internal/providers/{docker,podman,nerdctl}/provider.go`: Runtime-specific implementations

**Testing:**
- Test files co-located with source: `*_test.go` in same directory
- Test data in: `pkg/internal/apis/config/encoding/testdata/`

## Naming Conventions

**Files:**
- Source files: `lowercase_with_underscores.go`
- Test files: `source_test.go` (co-located with source)
- Package docs: `doc.go` (package documentation)
- Generated files: `zz_generated.*.go` (e.g., `zz_generated.deepcopy.go`)

**Directories:**
- Package directories: lowercase, no underscores, semantic names
- Command packages: lowercase matching command name (create, delete, get, load, build, export)
- Provider packages: provider name lowercase (docker, podman, nerdctl, common)
- Action packages: action name lowercase (kubeadminit, kubeadmjoin, installcni)

**Packages and Types:**
- Packages: lowercase, importable via `sigs.k8s.io/kind/pkg/...`
- Public types: PascalCase (e.g., Cluster, Provider, Node)
- Private types: camelCase (e.g., provider, flagpole, actionContext)
- Interfaces: PascalCase, often end in -er (e.g., Provider, Logger, Action)
- Functions: PascalCase for exported, camelCase for unexported

**Constants:**
- Exported: UPPER_SNAKE_CASE (DefaultName, DefaultClusterName)
- Unexported: camelCase (fixedNetworkName, clusterNameMax)

## Where to Add New Code

**New Feature:**
- Primary code: `pkg/cmd/kind/{feature}/` for CLI command
- Core logic: `pkg/cluster/internal/` for cluster operations
- Public API: `pkg/cluster/provider.go` methods or new public types in `pkg/cluster/`
- Tests: `{feature}_test.go` alongside implementation

**New Command/Subcommand:**
- Implementation: `pkg/cmd/kind/{command}/{subcommand}/subcommand.go`
- Pattern: Implement `NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command`
- Registration: Add to root command in `pkg/cmd/kind/root.go` via `cmd.AddCommand()`

**New Provider (Docker/Podman/Nerdctl variant):**
- Implementation: `pkg/cluster/internal/providers/{provider}/provider.go`
- Pattern: Implement `providers.Provider` interface
- Common utilities: Extend `pkg/cluster/internal/providers/common/`
- Registration: Add to provider selection logic in `pkg/cluster/provider.go`

**New Action in Creation Pipeline:**
- Implementation: `pkg/cluster/internal/create/actions/{action}/action.go`
- Pattern: Implement `actions.Action` interface with `Execute(ctx *ActionContext) error`
- Registration: Add to action sequence in `pkg/cluster/internal/create/create.go`

**Utilities:**
- Shared helpers: `pkg/{category}/`
- Internal-only utilities: `pkg/internal/{category}/`
- CLI utilities: `pkg/cmd/` (logger, iostreams, shared CLI logic)

**Tests:**
- Unit tests: `*_test.go` in source directory
- Integration tests: `*_integration_test.go` or in `pkg/internal/integration/`
- Test data: `testdata/` subdirectory of package

## Special Directories

**pkg/internal/:**
- Purpose: Internal utilities not part of public API
- Generated: Some files generated via `//go:generate` directives
- Committed: Yes, all committed to git

**images/:**
- Purpose: Container image definitions
- Generated: Docker images built from these sources
- Committed: Yes, Dockerfile and scripts committed

**hack/:**
- Purpose: Development and build automation
- Generated: No (scripts, not generated code)
- Committed: Yes

**site/:**
- Purpose: Hugo-based documentation website
- Generated: HTML generated from markdown at build time
- Committed: Markdown and layouts committed, not HTML

---

*Structure analysis: 2026-03-01*
