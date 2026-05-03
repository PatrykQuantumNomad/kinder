<!-- refreshed: 2026-05-03 -->
# Architecture

**Analysis Date:** 2026-05-03

## System Overview

Kinder is a fork of kind (Kubernetes IN Docker) implementing a **wave-based parallel addon execution architecture** with provider abstraction for container runtimes (Docker/Podman/Nerdctl) and a Check-interface-based diagnostic system.

```text
┌──────────────────────────────────────────────────────────────────────┐
│                         CLI Layer (Cobra)                             │
│  root.go: kinder, create, delete, doctor, load, build, export, etc.  │
│  `pkg/cmd/kind/`                                                       │
└─────────────────────┬────────────────────────────────────────────────┘
                      │
┌─────────────────────▼────────────────────────────────────────────────┐
│                     Cluster Management Layer                          │
│              `pkg/cluster/` (Provider Pattern)                        │
├──────────────┬──────────────┬──────────────┬───────────────────────┤
│   Provider   │   Cluster    │ Node Config  │    Kubeconfig         │
│  interface   │  lifecycle   │  & Addons    │    management         │
│              │              │              │                       │
│ `provider.go`│ `provider.go`│`types.go`    │  `kubeconfig/`       │
└──────────────┴──────────────┴──────────────┴───────────────────────┘
         │              │              │              │
         ▼              ▼              ▼              ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    Internal Cluster Layer                             │
│           `pkg/cluster/internal/`                                     │
├─────────────────┬──────────────┬──────────────┬──────────────────────┤
│   Provisioning  │  Create Flow │  Actions &  │ Runtime Providers    │
│   Delete Flow   │ (wave-based) │  Addons     │ (Docker/Podman/etc)  │
│                 │              │             │                      │
│  `delete/`      │`create/`     │`create/`    │ `providers/`         │
│                 │              │actions/     │                      │
└─────────────────┴──────────────┴──────────────┴──────────────────────┘
         │              │              │              │
         ▼              ▼              ▼              ▼
┌──────────────────────────────────────────────────────────────────────┐
│                   Diagnostic & Utility Layers                         │
│           `pkg/internal/doctor/` & `pkg/internal/runtime/`           │
├──────────────────┬──────────────┬───────────────┬──────────────────┤
│  Doctor Checks   │  Kubeadm     │   Utilities   │    Configuration │
│  (23 checks/8    │  Integration │               │    APIs & Schemas│
│  categories)     │              │  exec, log,   │                  │
│                  │              │  errors, fs   │                  │
│  `check.go`      │  `kubeadm/`  │  `pkg/*/`     │ `pkg/internal/   │
│                  │              │               │  apis/config/`   │
└──────────────────┴──────────────┴───────────────┴──────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| **Root Command** | CLI entry point, logger setup, global flags (verbosity, quiet) | `pkg/cmd/kind/root.go` |
| **Create Command** | Cluster creation orchestration, profile selection, air-gapped mode | `pkg/cmd/kind/create/cluster/createcluster.go` |
| **Provider** | Container runtime abstraction (Docker/Podman/Nerdctl), node provisioning | `pkg/cluster/provider.go`, `pkg/cluster/internal/providers/` |
| **Create Flow** | Sequential node provisioning + parallel wave-based addon installation | `pkg/cluster/internal/create/create.go` |
| **Action Interface** | Plugin pattern for sequential/parallel cluster setup steps | `pkg/cluster/internal/create/actions/action.go` |
| **Doctor Command** | Diagnostic checks with category grouping and mitigations | `pkg/cmd/kind/doctor/doctor.go`, `pkg/internal/doctor/check.go` |
| **Load Command** | Image loading (Docker image, image archive) into cluster | `pkg/cmd/kind/load/` |
| **Kubeadm Integration** | API server, join node, CNI installation orchestration | `pkg/cluster/internal/kubeadm/` |
| **Configuration** | Cluster schema, node roles, networking, feature gates | `pkg/internal/apis/config/types.go` |
| **Website** | Astro-based documentation site (v2.2 guides, feature docs) | `kinder-site/` |

## Pattern Overview

**Overall:** Orchestration pipeline with provider abstraction, check-based diagnostics, and plugin-driven addon installation.

**Key Characteristics:**
- **Provider abstraction**: Runtime-agnostic via `providers.Provider` interface (Docker, Podman, Nerdctl implementations)
- **Wave-based parallel addon execution**: Wave 1 (3 concurrent) for independent addons; Wave 2 (sequential) for dependency-ordered addons (Envoy Gateway depends on MetalLB)
- **Action interface pattern**: Each setup step (kubeadm init, CNI, addons) implements `actions.Action` interface
- **Diagnostic check registry**: 23 checks across 8 categories with platform-specific filtering and exit codes (0=ok, 1=fail, 2=warn)
- **Sequential core setup, parallel addon installation**: Node provisioning → kubeadm init → CNI → join → wait-for-ready (sequential) → addons (waves 1-2 parallel/sequential)

## Layers

**CLI Layer:**
- Purpose: Parse flags, invoke subcommands, manage IO streams and logging
- Location: `pkg/cmd/kind/` (root.go), `pkg/cmd/kind/{create,delete,doctor,load,build,export,get,version,env,completion}/`
- Contains: Cobra command definitions, flagpole structs, RunE functions
- Depends on: cluster.Provider, actions.Action, doctor.Check, cmd.Logger, cmd.IOStreams
- Used by: cmd/kind/main.go → cmd/kind/app/main.go

**Cluster Management Layer:**
- Purpose: Public-facing cluster lifecycle API (Create, Delete, Export logs)
- Location: `pkg/cluster/`
- Contains: Provider interface, CreateOption/DeleteOption builders, public cluster operations
- Depends on: internal/providers.Provider, internal/create.Cluster(), internal/delete.Cluster()
- Used by: CLI commands, user code importing pkg/cluster

**Internal Cluster Layer:**
- Purpose: Implementation of cluster provisioning, creation flow, deletion, and provider-specific logic
- Location: `pkg/cluster/internal/`
- Contains: Provisioning (Provision), create flow (Cluster function), delete flow, provider implementations
- Depends on: Action plugins, kubeadm, kubeconfig, nodes/nodeutils
- Used by: public Provider.Create/Delete methods

**Create Flow:**
- Purpose: Orchestrate sequential core setup (nodes, kubeadm init, CNI, join) + parallel wave-based addons
- Location: `pkg/cluster/internal/create/create.go`
- Contains: ClusterOptions, Action execution loop, Wave 1/Wave 2 addon registry, error handling with optional retain
- Depends on: actions.Action, providers.Provider, addon action implementations
- Used by: Provider.Create()

**Action & Addon Layer:**
- Purpose: Encapsulate independent, composable setup steps with execute-once semantics
- Location: `pkg/cluster/internal/create/actions/`, `pkg/cluster/internal/create/actions/{kubeadminit,installcni,installmetallb,etc}/`
- Contains: Action interface, ActionContext, per-addon action types
- Depends on: config.Cluster, nodes.Node, providers.Provider, kubeadm, log.Logger
- Used by: Create flow for sequential or parallel execution

**Diagnostic Layer:**
- Purpose: Platform-specific prerequisite checks (runtime, GPU, kernel, security, network, mounts) with grouping and mitigations
- Location: `pkg/internal/doctor/`
- Contains: Check interface, 23 concrete check implementations (daemon, disk, apparmor, selinux, firewalld, wsl2, rootfs-device, inotify, kernel-version, nvidia, kubectl, cluster-skew, localpath-cve, offline-readiness, host-mount, docker-desktop-file-sharing)
- Depends on: exec, log, config (for mount paths)
- Used by: doctor command, create flow (safe mitigations tier-1)

**Configuration & Types Layer:**
- Purpose: Cluster schema, node roles, networking, feature gates, mount propagation
- Location: `pkg/internal/apis/config/`
- Contains: Cluster, Node, Addons, Networking, Mount, PortMapping types; validation; defaults; encoding (YAML/JSON)
- Depends on: none (leaf layer)
- Used by: Provider, Create, Doctor, CLI flags

**Utility Layers:**
- **Execution**: `pkg/exec/` - Command execution (local), error wrapping
- **Logging**: `pkg/log/` - Logger interface, noop implementation
- **Errors**: `pkg/errors/` - Custom error types, aggregate errors, concurrent error collection
- **Filesystem**: `pkg/fs/` - Filesystem helpers
- **CLI**: `pkg/internal/cli/` - Status reporting, flag helpers

## Data Flow

### Primary Request Path: `kinder create cluster`

1. **Entry Point** (`cmd/kind/main.go` → `cmd/kind/app/app.go:Main()`)
   - Initializes logger, IO streams, parses args

2. **Root Command** (`pkg/cmd/kind/root.go:NewCommand()`)
   - Sets up persistent flags (verbosity, quiet)
   - Registers subcommands (create, delete, doctor, load, etc.)

3. **Create Command** (`pkg/cmd/kind/create/create.go`)
   - Routes to create cluster

4. **Create Cluster Command** (`pkg/cmd/kind/create/cluster/createcluster.go:runE()`)
   - Parses flags (name, config, image, retain, wait, air-gapped, profile)
   - Initializes Provider via runtime detection or env var

5. **Provider.Create()** (`pkg/cluster/provider.go`)
   - Applies CreateOption builders (config, profile, air-gapped, etc.)
   - Delegates to `internal/create.Cluster()` function

6. **Create Flow** (`pkg/cluster/internal/create/create.go:Cluster()`)
   - Validates provider capabilities (rootless, cgroup v2, memory/pid limits)
   - Validates cluster config (name length, node configs, extra mounts)
   - Applies safe doctor mitigations (env vars, cluster adjustments)
   - **Provision Phase**: Calls `provider.Provision()` to create node containers (runs in parallel for all nodes in provider)
   - **Sequential Core Setup**:
     * Load balancer setup (if multi-control-plane)
     * Kubeadm config generation
     * `kubeadm init` on control-plane
     * CNI installation
     * Legacy storage class (if not using local-path)
     * `kubeadm join` on workers
     * Wait for cluster ready
   - Exports kubeconfig
   - **Wave 1 Addons** (parallel, max 3 concurrent):
     * Local Registry, MetalLB, Metrics Server, CoreDNS Tuning, Dashboard, Cert Manager, Local Path Provisioner, NVIDIA GPU
     * Each addon skipped if disabled, timed, error logged (warn-and-continue)
   - **Wave 2 Addons** (sequential after Wave 1):
     * Envoy Gateway (depends on MetalLB for LoadBalancer IPs)
   - **Finalization**: Print addon summary, usage, salutation

7. **Provider.Provision()** (`pkg/cluster/internal/providers/{docker,podman,nerdctl}/provider.go`)
   - Ensures node images are pulled
   - Creates network
   - Plans node container creation
   - Creates all node containers in parallel

### Secondary Flow: `kinder doctor`

1. Entry → Root → Doctor Command (`pkg/cmd/kind/doctor/doctor.go`)
2. Load cluster config if `--config` flag provided (extract mount paths)
3. Inject mount paths into checks via `doctor.SetMountPaths()`
4. Run all checks via `doctor.RunAllChecks()` (`pkg/internal/doctor/check.go`)
5. Filter by platform (skip checks not applicable to GOOS)
6. Output results (human-readable or JSON)
7. Exit with code based on results (0=ok, 1=fail, 2=warn)

**State Management:**
- **Create flow**: Cluster options → ClusterOptions struct → Provider.Create() passes options down → ActionContext shared across sequential/parallel actions via sync.OnceValues for cached ListNodes
- **Doctor checks**: AllChecks registry (order defines category grouping) → per-platform filtering → results collected
- **Addon execution**: Wave 1 uses errgroup.WithContext + SetLimit(3) for concurrency; Wave 2 sequential; results collected in deterministic order
- **Global state**: None at module level; all state passed through function parameters or context

## Key Abstractions

**Provider Interface:**
- Purpose: Abstract container runtime (Docker, Podman, Nerdctl)
- Examples: `pkg/cluster/internal/providers/docker/provider.go`, `pkg/cluster/internal/providers/podman/provider.go`, `pkg/cluster/internal/providers/nerdctl/provider.go`
- Pattern: Each provider implements `providers.Provider` interface (Provision, ListClusters, ListNodes, DeleteNodes, GetAPIServerEndpoint, CollectLogs, Info)

**Action Interface:**
- Purpose: Plugin pattern for composable cluster setup steps
- Examples: `pkg/cluster/internal/create/actions/kubeadminit/`, `pkg/cluster/internal/create/actions/installcni/`, `pkg/cluster/internal/create/actions/installmetallb/`
- Pattern: Each action implements `actions.Action` interface (Execute(ctx *ActionContext) error); executes via kubectl, kubeadm, or helm; receives shared ActionContext with Logger, Status, Config, Provider, cached Nodes

**Check Interface:**
- Purpose: Plugin pattern for diagnostic checks with platform filtering
- Examples: 23 checks in `pkg/internal/doctor/` (daemon, disk, apparmor, selinux, firewalld, wsl2, rootfs-device, inotify, kernel-version, nvidia-driver, nvidia-container-toolkit, nvidia-docker-runtime, kubectl, kubectl-version-skew, nvidiaGPU, cluster-node-skew, localpath-cve, offline-readiness, host-mount, docker-desktop-file-sharing)
- Pattern: Each check implements `doctor.Check` interface (Name(), Category(), Platforms(), Run() []Result); registered in `allChecks` slice; filtered by platform; results contain Name, Category, Status (ok/warn/fail/skip), Message, Reason, Fix

**Config Schema (Internal):**
- Purpose: Declarative cluster specification
- Examples: Cluster, Node, Addons, Networking, Mount, PortMapping, PatchJSON6902
- Pattern: YAML/JSON encoded to config.Cluster; validated before use; supports kubeadm patches, containerd patches, node labels, extra mounts/port-mappings, multi-version nodes, feature gates, runtime config

**CreateOption Builder Pattern:**
- Purpose: Type-safe cluster creation configuration
- Examples: CreateWithConfigFile, CreateWithAddonProfile, CreateWithNodeImage, CreateWithRetain, CreateWithWaitForReady, CreateWithAirGapped
- Pattern: Each option is a function implementing `CreateOption` interface; applied in Provider.Create() to populate ClusterOptions struct

## Entry Points

**Binary Entry Point:**
- Location: `cmd/kind/main.go` (stub) → `cmd/kind/app/app.go:Main()`
- Triggers: Binary execution
- Responsibilities: Initialize logger, IO streams, call Run() which creates root command and executes it

**Command Entry Points:**
- **create cluster**: `pkg/cmd/kind/create/cluster/createcluster.go:runE()`
- **delete cluster**: `pkg/cmd/kind/delete/cluster/deletecommand.go`
- **doctor**: `pkg/cmd/kind/doctor/doctor.go:runE()`
- **load docker-image / image-archive**: `pkg/cmd/kind/load/docker-image/`, `pkg/cmd/kind/load/image-archive/`
- **build node-image**: `pkg/cmd/kind/build/nodeimage/buildnode.go`
- **export logs**: `pkg/cmd/kind/export/export.go`

**Public Library Entry Points (for programmatic use):**
- `pkg/cluster.NewProvider(options...)` → provider with auto-detection or explicit runtime
- `provider.Create(name string, options...)`
- `provider.Delete(name string)`
- `pkg/internal/doctor.RunAllChecks()` → []Result
- `pkg/internal/doctor.AllChecks()` → []Check

## Architectural Constraints

- **Threading:** Single-threaded event loop at CLI layer (Cobra) and action execution (sequential actions). Wave 1 addons use errgroup for concurrency (SetLimit(3)). Node provisioning parallelized in provider implementations.
- **Global state:** None at module level; all state passed explicitly. Doctor check registry (`allChecks` slice) is initialized at package load time but not modified.
- **Circular imports:** None detected; pkg/cluster depends on internal packages, internal packages depend on leaf utilities, no upward dependencies.
- **Provider swapping:** Must happen before Create() via ProviderWithDocker/Podman/Nerdctl options. Runtime auto-detection via kind.IsAvailable() checks.
- **Addon execution model:** Actions executed sequentially by default. Wave 1 addons run in parallel (errgroup, limit 3). Wave 2 (Envoy Gateway) sequential after Wave 1 completes. Addons don't propagate errors (warn-and-continue).
- **Config immutability:** Config passed to Create() is modified in place (addons, containerd patches injected); caller should not reuse config after Create().
- **Kubeconfig export:** Retried with backoff (0, 1ms, 50ms, 100ms) due to file locking on some systems.

## Anti-Patterns

### Wave 1 AddOn Execution Order

**What happens:** Although Wave 1 addons are run in parallel, the registry order in the `wave1` slice determines the order in which they appear in the summary output.

**Why it's wrong:** Developers adding new Wave 1 addons may assume order implies dependency, but all Wave 1 addons are explicitly designed to be independent.

**Do this instead:** Clearly document in code (or via comments in the wave1 slice) that order is for summary output only. Add inter-addon dependency checks before execution if a new addon has a hard requirement. Consider moving to Wave 2 if dependencies can't be eliminated.

**Reference:** `pkg/cluster/internal/create/create.go` lines 291-300 (wave1 registry).

### Error Handling in Doctor Checks

**What happens:** Individual check failures are collected as Result entries (status="fail") rather than returning errors. The RunAllChecks() function collects all results regardless of failures.

**Why it's wrong:** Could mask systematic failures if a check panics or crashes (not wrapped as Result).

**Do this instead:** Each check should defensively recover from panics and return a "fail" Result. Current implementation relies on checks being well-written. Add a wrapper in RunAllChecks() that catches panics per check.

**Reference:** `pkg/internal/doctor/check.go` lines 114-130 (RunAllChecks).

### Mount Path Configuration via External Config

**What happens:** Mount paths for HostMount checks are injected post-registry via `doctor.SetMountPaths()` call in doctor command, before running checks.

**Why it's wrong:** Coupling between doctor command and check implementations; no type-safe validation of paths.

**Do this instead:** Consider using dependency injection at check instantiation time, or a configuration struct passed to Run().

**Reference:** `pkg/cmd/kind/doctor/doctor.go` lines 80-85, `pkg/internal/doctor/check.go` lines 92-105.

---

*Architecture analysis: 2026-05-03*
