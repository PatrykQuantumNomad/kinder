# Architecture

**Analysis Date:** 2026-04-08

## Pattern Overview

**Overall:** Command-based CLI with layered abstraction over container runtimes (Docker, Podman, Nerdctl). The architecture follows a provider pattern to support multiple container backends while maintaining a consistent public API for Kubernetes cluster lifecycle operations.

**Key Characteristics:**
- Multi-layered separation between CLI commands, cluster operations, and container runtime providers
- Provider abstraction pattern enabling pluggable container runtime backends
- Action-based orchestration model for cluster creation with composable setup steps
- Configuration-driven cluster definition via YAML/JSON specs
- Wrapper pattern around Cobra for CLI command construction
- Interface-based dependency injection for logging and I/O streams

## Layers

**CLI Command Layer:**
- Purpose: Parse user input and orchestrate cluster operations through the Provider API
- Location: `pkg/cmd/` and `cmd/kind/app/`
- Contains: Cobra command definitions, flag parsing, user-facing output handling
- Depends on: Logger, IOStreams, cluster.Provider
- Used by: Direct user invocation through binary entry point

**Cluster Operations Layer:**
- Purpose: High-level cluster lifecycle management (create, delete, export, load images)
- Location: `pkg/cluster/provider.go`, `pkg/cluster/internal/create/`, `pkg/cluster/internal/delete/`
- Contains: Provider struct, cluster-level operations, orchestration of creation actions
- Depends on: Runtime provider abstractions, kubeadm configuration, node management
- Used by: CLI command layer for user-driven operations

**Container Runtime Provider Layer:**
- Purpose: Abstract container-specific operations (image pulling, network setup, node spawning)
- Location: `pkg/cluster/internal/providers/docker/`, `pkg/cluster/internal/providers/nerdctl/`, `pkg/cluster/internal/providers/podman/`
- Contains: Provider implementations for Docker, Podman, and Nerdctl; common utility functions
- Depends on: External container CLIs via execution abstraction (`pkg/exec/`)
- Used by: Cluster operations layer to execute container-level tasks

**Configuration & API Layer:**
- Purpose: Define and manage cluster configuration schemas and defaults
- Location: `pkg/apis/config/v1alpha4/`, `pkg/internal/apis/config/`
- Contains: Cluster, Node, Networking, Addon type definitions; YAML/JSON serialization
- Depends on: Standard Go libraries for encoding/decoding
- Used by: Cluster creation and configuration patching subsystems

**Build & Node Image Layer:**
- Purpose: Construct Kubernetes node images with pre-installed components
- Location: `pkg/build/nodeimage/`
- Contains: Image build logic, base image selection, Kubernetes binary integration
- Depends on: Container runtime provider, external Kubernetes release artifacts
- Used by: Create cluster flow when custom node images are specified

**Execution Abstraction Layer:**
- Purpose: Execute shell commands locally with error and output capture
- Location: `pkg/exec/`
- Contains: Local command executor interfaces and implementations
- Depends on: OS process execution APIs
- Used by: Container runtime providers to invoke docker/podman/nerdctl CLIs

## Data Flow

**Cluster Creation Flow:**

1. User invokes `kinder create cluster` with optional config file and flags
2. CLI layer (CreateCommand) parses inputs and creates cluster.Provider instance
3. Provider.Create() is called with cluster configuration
4. Cluster operations layer loads/defaults configuration, validates node setup
5. Runtime provider initializes container-specific resources (networks, images)
6. Creation actions execute sequentially (kubeadm init, addon installation, etc.)
7. Kubeconfig is generated and exported to user's filesystem
8. Operation returns success or triggers rollback (node cleanup if --retain not set)

**Node Provisioning Within Create:**

1. Create action framework loads sequence of Action implementations
2. Each action (kubeadminit, installcni, installmetallb, etc.) operates on nodes
3. Actions can be parallelized via sync.WaitGroup or errgroup.Group
4. Node operations delegate to runtime provider for container exec
5. State is tracked per node with kubeconfig and runtime status
6. Optional addon profiles determine which actions are included

**State Management:**
- Cluster state resides in container runtime (node containers themselves)
- Kubeconfig state written to disk after creation
- Node metadata tracked in memory during operation; persisted via container labels
- No external state backend; provider queries container runtime for cluster metadata

## Key Abstractions

**Provider Interface:**
- Purpose: Defines contract for container runtime backends
- Examples: `pkg/cluster/internal/providers/docker/provider.go`, `pkg/cluster/internal/providers/nerdctl/provider.go`
- Pattern: Each provider implements methods like PullImage, CreateNode, GetNodes, ExecNode
- Used by: Cluster operations layer to remain agnostic to specific container runtime

**Action Interface:**
- Purpose: Composable steps in cluster creation pipeline
- Examples: `pkg/cluster/internal/create/actions/kubeadminit/kubeadminit.go`, `pkg/cluster/internal/create/actions/installcni/installcni.go`
- Pattern: Each action implements Action interface with Execute(ctx, logger) error
- Used by: Create orchestrator to apply setup steps to nodes

**Logger Interface:**
- Purpose: Abstract logging with verbosity control
- Location: `pkg/log/types.go`
- Pattern: Implements Log, Logf, Error, Errorf, V(level) for hierarchical verbosity
- Used by: All layers for consistent output formatting

**Config Encoding/Decoding:**
- Purpose: Parse YAML cluster definitions into typed structures
- Location: `pkg/internal/apis/config/encoding/`
- Pattern: Unmarshals YAML → v1alpha4.Cluster → internal default-merged config
- Used by: CLI layer before passing config to Provider.Create()

## Entry Points

**Binary Entry Point:**
- Location: `main.go`
- Triggers: User runs `kinder` binary
- Responsibilities: Minimal stub that calls cmd/kind/app.Main()

**App Main Entry:**
- Location: `cmd/kind/app/main.go`
- Triggers: Binary execution
- Responsibilities: Initialize Logger, IOStreams; invoke Run() with CLI args

**Root Command Entry:**
- Location: `pkg/cmd/kind/root.go`
- Triggers: App Main after I/O setup
- Responsibilities: Construct root cobra.Command; register all subcommands (create, delete, get, build, etc.)

**Create Cluster Command:**
- Location: `pkg/cmd/kind/create/cluster/createcluster.go`
- Triggers: `kinder create cluster` invocation
- Responsibilities: Parse cluster-specific flags, load config file, invoke cluster.Provider.Create()

## Error Handling

**Strategy:** Layered error propagation with context-specific logging and optional stack traces.

**Patterns:**
- All errors returned up stack with context added at each layer
- Error wrapping with github.com/pkg/errors for stack trace capture
- ExecutionErrors from container runtimes include captured command output
- Verbosity flag (-v) enables full stack trace printing in top-level error handler
- Rollback (node deletion) triggered on creation failure unless --retain specified

## Cross-Cutting Concerns

**Logging:**
- Interface-based Logger injected into all major components
- Verbosity controlled via -v flag mapped to log.Level
- Special handling for --quiet flag that discards stderr, allowing only stdout
- Error handler in app.go formats ERROR messages with color on TTY

**Validation:**
- Config validation happens in cluster operations layer after loading
- Node specifications validated for proper topology (e.g., at least one control-plane)
- Container runtime availability checked at provider initialization (DetectNodeProvider)

**Authentication:**
- No authentication within kind itself
- Kubeconfig generation includes embedded certificates for local cluster access
- Node access via container CLI (docker exec, podman exec) uses local daemon auth

---

*Architecture analysis: 2026-04-08*
