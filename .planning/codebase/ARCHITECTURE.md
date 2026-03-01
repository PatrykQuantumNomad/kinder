# Architecture

**Analysis Date:** 2026-03-01

## Pattern Overview

**Overall:** Multi-layered CLI tool with provider abstraction pattern

**Key Characteristics:**
- Cobra-based command structure for CLI interface
- Provider pattern for pluggable container runtimes (Docker, Podman, Nerdctl)
- Action-based cluster creation pipeline with composable steps
- Configuration-driven Kubernetes cluster provisioning via kubeadm
- Internal/public API separation with configuration versioning

## Layers

**CLI Layer:**
- Purpose: Command-line interface and user interaction
- Location: `cmd/kind/app/`, `pkg/cmd/kind/`
- Contains: Cobra command definitions, flag parsing, IO stream handling
- Depends on: Logger, IOStreams, Cluster operations
- Used by: End users via binary

**Command Layer:**
- Purpose: Route CLI requests to cluster operations
- Location: `pkg/cmd/kind/root.go`, `pkg/cmd/kind/{create,delete,load,build,export,get}/`
- Contains: Command handlers for create, delete, get, load, build, export, version, completion
- Depends on: Cluster provider, configuration encoding, logging
- Used by: CLI layer

**Cluster Abstraction Layer:**
- Purpose: Public API for cluster operations and provider selection
- Location: `pkg/cluster/provider.go`
- Contains: `Provider` struct with methods: Create, Delete, Export, GetKubeconfig
- Depends on: Internal providers, internal create/delete logic
- Used by: Command handlers

**Provider Layer (abstraction):**
- Purpose: Define interface for container runtime interactions
- Location: `pkg/cluster/internal/providers/provider.go`
- Contains: Provider interface with methods: Provision, ListClusters, ListNodes, DeleteNodes, GetAPIServerEndpoint, CollectLogs, Info
- Depends on: None (interface definition)
- Used by: All provider implementations

**Provider Implementations:**
- Purpose: Execute cluster operations via specific container runtimes
- Location: `pkg/cluster/internal/providers/{docker,podman,nerdctl}/`
- Contains: Docker, Podman, Nerdctl provider implementations with network, image, and node management
- Depends on: Common utilities, exec package, configuration
- Used by: Cluster abstraction layer

**Creation Action Pipeline:**
- Purpose: Orchestrate multi-step cluster initialization
- Location: `pkg/cluster/internal/create/create.go`, `pkg/cluster/internal/create/actions/`
- Contains: Action interface with implementations: config, kubeadminit, kubeadjoin, installcni, installstorage, loadbalancer, waitforready
- Depends on: Nodes, configuration, providers, kubectl execution
- Used by: Cluster provider for cluster creation

**Configuration Layer:**
- Purpose: Define and validate cluster configuration schema
- Location: `pkg/apis/config/v1alpha4/`, `pkg/internal/apis/config/`
- Contains: Cluster, Node, Networking types; YAML encoding/decoding; defaults
- Depends on: None (data structures)
- Used by: All layers requiring cluster configuration

**Utilities Layer:**
- Purpose: Shared cross-cutting concerns
- Location: `pkg/log/`, `pkg/cmd/{logger,iostreams}.go`, `pkg/exec/`, `pkg/fs/`, `pkg/errors/`
- Contains: Logging interface, IO streams, command execution, file system ops, error handling
- Depends on: Standard library
- Used by: All other layers

**Build Layer:**
- Purpose: Build custom node images for Kubernetes
- Location: `pkg/build/nodeimage/`
- Contains: Node image builders for release, file, URL sources
- Depends on: Container management, kube tools, logger
- Used by: Build command

## Data Flow

**Cluster Creation Flow:**

1. User runs `kind create cluster --name my-cluster --config config.yaml`
2. `Main()` in `cmd/kind/app/main.go` initializes logger and IO streams
3. Root command handler in `pkg/cmd/kind/root.go` routes to create subcommand
4. `pkg/cmd/kind/create/cluster/` handler reads YAML config and parses flags
5. Config is validated and normalized via `pkg/apis/config/v1alpha4/`
6. `Provider.Create()` in `pkg/cluster/provider.go` is called with config
7. Provider selects runtime (Docker/Podman/Nerdctl) from `pkg/cluster/internal/providers/`
8. Provider's `Provision()` method creates node containers
9. Action pipeline executes in sequence:
   - Apply config patches
   - Initialize kubeadm control-plane
   - Join worker nodes
   - Install CNI
   - Install storage
   - Setup load balancer (if HA)
   - Wait for readiness
10. Kubeconfig is extracted and saved
11. Cluster status returned to user

**State Management:**
- Kubernetes cluster state stored as running containers in Docker/Podman/Nerdctl
- Node information tracked via container labels and inspection
- Configuration applied via kubeadm manifests and kubectl apply
- State discovered dynamically via provider interface (no persistent state file)

## Key Abstractions

**Provider:**
- Purpose: Abstract container runtime operations
- Examples: `pkg/cluster/internal/providers/docker/provider.go`, `pkg/cluster/internal/providers/podman/provider.go`, `pkg/cluster/internal/providers/nerdctl/provider.go`
- Pattern: Provider interface with multiple implementations; selected at runtime based on availability

**Action:**
- Purpose: Atomic step in cluster creation pipeline
- Examples: `pkg/cluster/internal/create/actions/kubeadminit/`, `pkg/cluster/internal/create/actions/installcni/`
- Pattern: Action interface with Execute method; context carries logger, config, provider, nodes

**Logger:**
- Purpose: Unified logging interface with verbosity levels
- Location: `pkg/log/types.go`
- Pattern: Logger interface (Warn, Warnf, Error, Errorf, V) with NoopLogger default implementation

**Cluster Configuration:**
- Purpose: Type-safe cluster definition with validation
- Examples: `pkg/apis/config/v1alpha4/types.go` (public), `pkg/internal/apis/config/` (internal)
- Pattern: Configuration versioning (v1alpha4), defaults, encoding/decoding, validation hooks

## Entry Points

**Binary Entry Point:**
- Location: `main.go`
- Triggers: User execution of `kind` binary
- Responsibilities: Stub main that calls `app.Main()`

**Application Entry Point:**
- Location: `cmd/kind/app/main.go`
- Triggers: Invoked by binary stub
- Responsibilities: Initialize logger and IO streams, handle quiet flag, execute root command

**Root Command:**
- Location: `pkg/cmd/kind/root.go`
- Triggers: User runs `kind` or `kind <subcommand>`
- Responsibilities: Register all subcommands, set verbosity, establish command hierarchy

**Create Command:**
- Location: `pkg/cmd/kind/create/cluster/`
- Triggers: User runs `kind create cluster`
- Responsibilities: Parse config file, apply flag overrides, invoke cluster creation

**Cluster Operations:**
- Location: `pkg/cluster/provider.go`
- Triggers: Command handlers via `cluster.Provider` methods
- Responsibilities: Orchestrate provider selection, create/delete/export operations

## Error Handling

**Strategy:** Error wrapping with context and stack trace capture

**Patterns:**
- Errors wrapped using `errors.Wrap(err, "context")` from `pkg/errors/`
- Stack traces captured on error creation for debugging via `-v` flag
- User-facing errors logged with `logger.Errorf()` with color support when available
- Command output from failed exec included in error logs for diagnostics
- Quiet flag (`-q`) suppresses error output to stderr except via `streams.Out`

## Cross-Cutting Concerns

**Logging:** Implemented via `pkg/log/Logger` interface with verbosity levels V(0-3+); NoopLogger used by default; can be set writer/verbosity via `SetWriter`/`SetVerbosity` interface pattern in `pkg/cmd/kind/root.go`

**Validation:** Configuration validation via `(*config.Cluster).Validate()` method called in create flow; schema enforced via struct tags in `pkg/apis/config/v1alpha4/types.go`

**Authentication:** Provider capabilities reported via `Info()` method returning `ProviderInfo` (rootless, cgroup2, memory/pid/cpu limits support); no user auth - operates on local machine

---

*Architecture analysis: 2026-03-01*
