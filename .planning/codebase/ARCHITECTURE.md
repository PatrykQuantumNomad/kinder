# Architecture

**Analysis Date:** 2026-03-02

## Pattern Overview

**Overall:** Layered CLI Application with Provider Abstraction

Kinder is a Go-based CLI tool built on top of kind that manages Kubernetes-in-Docker clusters with a batteries-included addon system. The architecture follows a clear separation of concerns:

1. **CLI Layer** - Cobra-based command structure with flag parsing
2. **Cluster Management Layer** - High-level cluster operations through a Provider interface
3. **Provider Layer** - Container runtime abstraction (Docker, Podman, nerdctl)
4. **Cluster Setup Layer** - Sequential actions for bootstrapping and configuring Kubernetes
5. **Configuration Layer** - Type-safe cluster configuration and validation

**Key Characteristics:**
- Plugin-based provider system allowing multiple container runtimes
- Action-based cluster creation with ordered sequential steps
- Configuration-driven cluster definition
- Error handling with stack traces and aggregation
- Addon installation through embedded Kubernetes manifests
- Internal/Public API separation for stability

## Layers

**CLI Layer:**
- Purpose: Command parsing, user interface, flag handling
- Location: `cmd/kind/app/main.go`, `pkg/cmd/kind/root.go`, `pkg/cmd/kind/create/`, `pkg/cmd/kind/delete/`, `pkg/cmd/kind/get/`, `pkg/cmd/kind/load/`, `pkg/cmd/kind/build/`
- Contains: Cobra command definitions, flag definitions, input validation
- Depends on: Cluster management layer, runtime detection, configuration loading
- Used by: Main entry point, orchestrates all user-facing operations

**Cluster Management Layer:**
- Purpose: Orchestrate cluster lifecycle operations (create, delete, list)
- Location: `pkg/cluster/provider.go`, `pkg/cluster/createoption.go`
- Contains: Provider struct, cluster operations, option builders
- Depends on: Internal providers, internal cluster creation, node utilities, configuration
- Used by: CLI layer commands, provides public API for programmatic use

**Cluster Setup Layer:**
- Purpose: Execute ordered setup steps during cluster creation
- Location: `pkg/cluster/internal/create/create.go`, `pkg/cluster/internal/create/actions/`
- Contains: Create orchestration logic, action execution engine, addon installation actions
- Depends on: Providers, configuration, action context
- Used by: Cluster management layer during create operation

**Action System:**
- Purpose: Implement sequential cluster setup operations
- Location: `pkg/cluster/internal/create/actions/action.go` and action subdirectories
- Contains: Action interface, ActionContext, per-addon implementations
- Depends on: Nodes, providers, configuration, manifests
- Used by: Cluster setup layer, executed in sequence during creation
- Action examples: `installmetallb/`, `installenvoygw/`, `installmetricsserver/`, `installdashboard/`, `installcni/`, `installstorage/`, `installcorednstuning/`, `kubeadminit/`, `kubeadmjoin/`, `loadbalancer/`, `waitforready/`, `config/`

**Provider Layer:**
- Purpose: Abstract container runtime operations
- Location: `pkg/cluster/internal/providers/provider.go`, `pkg/cluster/internal/providers/docker/`, `pkg/cluster/internal/providers/podman/`, `pkg/cluster/internal/providers/nerdctl/`
- Contains: Provider interface definition, concrete implementations per runtime
- Depends on: Node interface, configuration, CLI utilities
- Used by: Cluster management layer, provider detection, cluster operations

**Configuration Layer:**
- Purpose: Type-safe configuration representation and validation
- Location: `pkg/internal/apis/config/`, `pkg/apis/config/`
- Contains: Cluster struct, Node struct, Networking struct, Addons struct, validation logic
- Depends on: Encoding utilities
- Used by: All cluster operations, cluster creation actions

**Node Layer:**
- Purpose: Abstract cluster node operations
- Location: `pkg/cluster/nodes/types.go`, `pkg/cluster/nodeutils/`
- Contains: Node interface, node utility functions
- Depends on: Execution primitives
- Used by: Provider implementations, actions, cluster operations

**Utility Layers:**
- **Execution**: `pkg/exec/` - Command execution abstraction
- **Logging**: `pkg/log/` - Logger interface (Info, Warn, Error, Verbosity levels)
- **IO Streams**: `pkg/cmd/iostreams.go` - Standardized In/Out/ErrOut
- **Error Handling**: `pkg/errors/` - Error wrapping, aggregation, stack traces
- **Runtime Detection**: `pkg/internal/runtime/` - Provider selection from environment
- **CLI Utilities**: `pkg/internal/cli/` - Status/spinner rendering, logger overrides
- **File System**: `pkg/fs/` - File system operations

## Data Flow

**Cluster Creation Flow:**

1. **User Input** → CLI command receives flags and arguments
2. **Provider Detection** → Detect available container runtime (Docker/Podman/nerdctl) or use override
3. **Configuration Loading** → Read YAML config file or use defaults, merge with command flags
4. **Validation** → Validate cluster configuration against rules
5. **Provider.Provision()** → Container runtime creates nodes with networking
6. **Sequential Actions** → Execute ordered setup steps:
   - Load balancer setup
   - Kubeadm config generation
   - Kubeadm init on control plane
   - CNI installation
   - Storage class installation
   - Worker node join
   - Wait for cluster readiness
7. **Addon Installation** → Conditionally install enabled addons:
   - MetalLB (LoadBalancer support)
   - Envoy Gateway (Gateway API)
   - Metrics Server (kubectl top, HPA)
   - CoreDNS Tuning (DNS optimization)
   - Dashboard (Headlamp web UI)
8. **Kubeconfig Export** → Write cluster credentials to user's kubeconfig file
9. **User Notification** → Display token and access instructions

**Cluster Deletion Flow:**

1. User requests cluster deletion
2. Kubeconfig cleanup from user's config file
3. Provider.DeleteNodes() removes all container nodes
4. Cluster records cleaned up

**Get/List Cluster Flow:**

1. User requests cluster or node information
2. Provider.ListClusters() or Provider.ListNodes() queries container runtime
3. Results formatted and displayed to user

**State Management:**

- **Cluster State**: Stored in container runtime (Docker/Podman containers and networks)
- **Kubeconfig State**: Stored in user's kubeconfig file (typically ~/.kube/config)
- **In-Memory State**: ActionContext caches node lists during cluster creation to avoid repeated queries

## Key Abstractions

**Provider Interface:**
- Purpose: Enables support for Docker, Podman, nerdctl, Finch
- Location: `pkg/cluster/internal/providers/provider.go`
- Pattern: Strategy pattern - different implementations for different runtimes
- Examples: `pkg/cluster/internal/providers/docker/provider.go`, `pkg/cluster/internal/providers/podman/`, `pkg/cluster/internal/providers/nerdctl/`
- Key Methods: Provision(), ListClusters(), ListNodes(), DeleteNodes(), GetAPIServerEndpoint()

**Action Interface:**
- Purpose: Modular setup steps with shared context
- Location: `pkg/cluster/internal/create/actions/action.go`
- Pattern: Command pattern - encapsulates a setup step with dependencies
- Methods: Execute(ctx *ActionContext) error
- Context: ActionContext provides Logger, Status, Provider, Config, cached Nodes

**Node Interface:**
- Purpose: Abstract node-specific operations
- Location: `pkg/cluster/nodes/types.go`
- Pattern: Strategy pattern - different implementations per provider
- Key Methods: Command() (run commands), Role(), IP(), SerialLogs()

**Configuration Struct:**
- Purpose: Strongly-typed representation of kind cluster config
- Location: `pkg/internal/apis/config/types.go`
- Fields: Cluster, Nodes, Networking, Addons, FeatureGates, RuntimeConfig, KubeadmConfigPatches
- Validation: Validate() method checks constraints

**Logger Interface:**
- Purpose: Decoupled logging for testability and extensibility
- Location: `pkg/log/types.go`
- Pattern: Strategy pattern - different implementations (real, noop, test)
- Methods: Warn()/Warnf(), Error()/Errorf(), V() for verbosity levels

**ProviderOption Pattern:**
- Purpose: Functional options for Provider construction
- Location: `pkg/cluster/provider.go` and option files in `pkg/cluster/`
- Pattern: Builder pattern - chainable options (ProviderWithLogger, ProviderWithDocker, etc.)
- Used by: NewProvider() constructor

## Entry Points

**Main Binary Entry Point:**
- Location: `main.go`
- Triggers: Binary execution by user or CI/CD
- Responsibilities: Delegates to app.Main()

**App Main Entry Point:**
- Location: `cmd/kind/app/main.go` - Main() function
- Triggers: Invoked from main.go
- Responsibilities:
  - Creates logger and IO streams
  - Detects quiet flag for output suppression
  - Delegates to Run()
  - Handles exit codes

**Root Command Entry Point:**
- Location: `pkg/cmd/kind/root.go` - NewCommand() function
- Triggers: Invoked from app.Run()
- Responsibilities:
  - Creates root Cobra command
  - Registers subcommands (create, delete, get, load, build, completion, version)
  - Configures persistent flags (verbosity, quiet)
  - Handles pre-run setup

**Subcommand Entry Points:**
- `pkg/cmd/kind/create/cluster/createcluster.go` - Create cluster command
- `pkg/cmd/kind/delete/cluster/deletecluster.go` - Delete cluster command
- `pkg/cmd/kind/get/clusters/getclusters.go` - List clusters command
- `pkg/cmd/kind/get/nodes/getnodes.go` - List nodes command
- `pkg/cmd/kind/get/kubeconfig/getkubeconfig.go` - Export kubeconfig command
- `pkg/cmd/kind/export/logs/exportlogs.go` - Export logs command
- `pkg/cmd/kind/build/nodeimage/buildnodeimage.go` - Build node image command
- `pkg/cmd/kind/load/docker-image/load.go` - Load Docker image command
- `pkg/cmd/kind/version/version.go` - Version command

## Error Handling

**Strategy:** Explicit error wrapping with stack traces and aggregation

**Patterns:**

1. **Error Wrapping:**
   ```go
   if err != nil {
       return errors.Wrap(err, "context about what failed")
   }
   ```
   - Location: `pkg/errors/` - Wrap(), WithStack(), Cause()
   - Preserves error chain and adds context at each layer

2. **Error Aggregation:**
   - Used when multiple errors can occur (e.g., multiple addon installation failures)
   - Function: `errors.NewAggregate([]error)` - flattens and reduces error list
   - Returns combined error with all failures

3. **Stack Traces:**
   - Automatically captured with `errors.WithStack()`
   - Only printed when logger verbosity V(1) enabled
   - Provides debugging information without normal output pollution

4. **Run Error Extraction:**
   - Command execution errors have output captured
   - Accessible via `exec.RunErrorForError(err)`
   - Output displayed separately from error message

5. **Cleanup on Failure:**
   - If cluster creation fails, nodes are deleted (unless --retain flag)
   - Implemented in `pkg/cluster/internal/create/create.go`

## Cross-Cutting Concerns

**Logging:**
- Abstraction: `pkg/log/Logger` interface
- Implementation: Kubernetes klog-compatible interface
- Verbosity levels: V(0) normal, V(1) debug, V(2+) trace
- Color support: Colors used when terminal supports it
- Examples: Logger passed to all major components

**Validation:**
- Configuration validation: `config.Cluster.Validate()`
- Node image validation: Provider ensures images are pulled before provisioning
- Cluster name validation: Maximum 50 characters (containers add "-control-plane" suffix)
- Provider validation: `validateProvider()` in create flow

**Authentication:**
- Kubeconfig generation and management: `pkg/cluster/internal/kubeconfig/`
- Token extraction and display for dashboard access
- API server endpoint detection and export

**Status Display:**
- CLI status/spinner: `pkg/internal/cli/status.go`
- Progress indication during long operations
- Output formatting and color support

**Configuration Override Precedence:**
1. Command line flags (highest priority)
2. Environment variables (KIND_CLUSTER_NAME, KIND_EXPERIMENTAL_PROVIDER, etc.)
3. Config file values
4. Defaults (lowest priority)

**Provider Detection:**
- Sequence: Environment variable → try Docker → try Podman → try nerdctl → error
- Configurable via KIND_EXPERIMENTAL_PROVIDER environment variable
- Fallback to Docker for backward compatibility

---

*Architecture analysis: 2026-03-02*
