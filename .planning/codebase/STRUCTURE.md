# Codebase Structure

**Analysis Date:** 2026-05-03

## Directory Layout

```
kinder/
├── cmd/
│   └── kind/
│       ├── main.go              # Stub entry point
│       └── app/
│           └── main.go          # Actual CLI entry point (Main, Run functions)
│
├── pkg/
│   ├── cmd/
│   │   ├── doc.go               # Package doc (CLI helpers)
│   │   ├── logger.go            # Logger interface wrappers
│   │   ├── iostreams.go         # IO streams (Out, ErrOut)
│   │   └── kind/
│   │       ├── root.go          # Root command (kinder, --verbosity, --quiet)
│   │       ├── create/          # Create command group
│   │       │   └── cluster/     # Create cluster subcommand
│   │       ├── delete/          # Delete command
│   │       ├── doctor/          # Doctor (diagnostic checks) command
│   │       ├── load/            # Load images command
│   │       │   ├── docker-image/
│   │       │   ├── image-archive/
│   │       │   └── images/
│   │       ├── build/           # Build node image command
│   │       ├── export/          # Export logs command
│   │       ├── get/             # Get command (clusters, nodes, kubeconfig)
│   │       ├── version/         # Version command
│   │       ├── completion/      # Shell completion
│   │       └── env/             # Environment variables
│   │
│   ├── cluster/
│   │   ├── doc.go               # Package doc (cluster lifecycle)
│   │   ├── provider.go          # Provider interface, NewProvider, Create/Delete/Export options
│   │   ├── createoption.go      # CreateOption builder pattern
│   │   ├── nodes/
│   │   │   ├── types.go         # Node type definition
│   │   │   └── doc.go
│   │   ├── constants/
│   │   │   └── constants.go     # DefaultClusterName, etc.
│   │   ├── nodeutils/
│   │   │   ├── roles.go         # IsControlPlane, IsWorker helpers
│   │   │   └── util.go
│   │   └── internal/
│   │       ├── create/
│   │       │   ├── create.go    # Main Cluster() function: provision + sequential setup + wave addons
│   │       │   ├── actions/
│   │       │   │   ├── action.go          # Action interface, ActionContext
│   │       │   │   ├── kubeadminit/      # kubeadm init action
│   │       │   │   ├── kubeadmjoin/      # kubeadm join action
│   │       │   │   ├── installcni/       # CNI installation
│   │       │   │   ├── installstorage/   # Legacy storage class
│   │       │   │   ├── installmetricsserver/
│   │       │   │   ├── installmetallb/   # Wave 1 addon
│   │       │   │   ├── installenvoygw/   # Wave 2 addon
│   │       │   │   ├── installlocalregistry/
│   │       │   │   ├── installcertmanager/
│   │       │   │   ├── installcorednstuning/
│   │       │   │   ├── installdashboard/
│   │       │   │   ├── installlocalpath/ # LocalPath provisioner addon
│   │       │   │   ├── installnvidiagpu/ # NVIDIA GPU addon
│   │       │   │   ├── loadbalancer/    # External LB setup
│   │       │   │   ├── waitforready/    # Wait for cluster ready
│   │       │   │   ├── config/          # Kubeadm config action
│   │       │   │   └── testutil/        # Test helpers
│   │       │   └── actions/manifests/   # Embedded YAML for addon deployments
│   │       │
│   │       ├── delete/
│   │       │   └── delete.go    # Cluster() function: delete all nodes
│   │       │
│   │       ├── providers/
│   │       │   ├── provider.go     # Provider interface
│   │       │   ├── docker/
│   │       │   │   ├── provider.go   # Docker provider impl
│   │       │   │   ├── create.go     # Node container creation
│   │       │   │   ├── network.go    # Docker network management
│   │       │   │   ├── images.go     # Image management
│   │       │   │   └── util.go
│   │       │   ├── podman/
│   │       │   │   └── provider.go   # Podman provider impl
│   │       │   ├── nerdctl/
│   │       │   │   └── provider.go   # Nerdctl provider impl
│   │       │   └── common/
│   │       │       ├── create.go     # Shared node creation logic
│   │       │       └── net.go        # Network utilities
│   │       │
│   │       ├── kubeadm/
│   │       │   ├── doc.go
│   │       │   └── ...             # Kubeadm integration, config generation
│   │       │
│   │       ├── kubeconfig/
│   │       │   ├── export.go       # Export kubeconfig
│   │       │   └── internal/       # kubeconfig library code
│   │       │
│   │       ├── logs/
│   │       │   └── ...             # Log collection from nodes
│   │       │
│   │       └── loadbalancer/
│   │           └── ...             # Load balancer setup (HAProxy)
│   │
│   ├── exec/
│   │   ├── types.go              # Command, Output types
│   │   ├── local.go              # Local command execution
│   │   ├── default.go            # Default executor
│   │   ├── helpers.go
│   │   └── doc.go
│   │
│   ├── log/
│   │   ├── types.go              # Logger interface
│   │   ├── noop.go               # NoopLogger impl
│   │   └── doc.go
│   │
│   ├── errors/
│   │   ├── errors.go             # Custom error types, WithStack, StackTrace
│   │   ├── aggregate.go          # Error aggregation
│   │   ├── aggregate_forked.go
│   │   ├── concurrent.go         # Concurrent error collection (UntilErrorConcurrent)
│   │   └── doc.go
│   │
│   ├── fs/
│   │   └── fs.go                 # Filesystem helpers
│   │
│   ├── build/
│   │   └── nodeimage/
│   │       ├── build.go          # Build node image
│   │       └── ...
│   │
│   └── internal/
│       ├── apis/
│       │   └── config/
│       │       ├── types.go           # Cluster, Node, Addons, Networking, Mount schemas
│       │       ├── validate.go        # Config validation
│       │       ├── default.go         # Defaults
│       │       ├── cluster_util.go
│       │       ├── convert_v1alpha4.go # Config version migration
│       │       ├── encoding/
│       │       │   └── ...            # YAML/JSON encoding
│       │       └── zz_generated.deepcopy.go
│       │
│       ├── doctor/
│       │   ├── check.go                      # Check interface, registry, RunAllChecks
│       │   ├── daemon.go                     # Docker daemon check
│       │   ├── disk.go (disk_unix.go, disk_other.go)  # Disk space check
│       │   ├── apparmor.go, apparmor_test.go          # AppArmor check
│       │   ├── selinux.go                   # SELinux check
│       │   ├── firewalld.go                 # Firewalld check
│       │   ├── wsl2.go                      # WSL2 check
│       │   ├── rootfs_device.go             # Root filesystem device check
│       │   ├── inotify.go                   # Inotify limit check
│       │   ├── kernel_version.go            # Kernel version check
│       │   ├── gpu.go (nvidia-driver, nvidia-container-toolkit, nvidia-docker-runtime)  # GPU checks
│       │   ├── containertocme.go (daemon, socket, snap, docker, storage)  # Docker-specific checks
│       │   ├── kubectl.go                   # kubectl availability check
│       │   ├── clusterskew.go               # kubectl/kubelet version skew check
│       │   ├── hostmount.go                 # Host mount path check
│       │   ├── localpath.go                 # LocalPath provisioner CVE check
│       │   ├── offline.go                   # Offline readiness check
│       │   ├── docker_desktop_file_sharing.go  # Docker Desktop file sharing check
│       │   ├── format.go                    # Result formatting
│       │   └── ...
│       │
│       ├── runtime/
│       │   ├── runtime.go         # GetDefault (auto-detect runtime)
│       │   └── ...
│       │
│       ├── cli/
│       │   ├── status.go          # Status reporting
│       │   └── ...
│       │
│       ├── kindversion/
│       │   └── version.go         # Version info
│       │
│       ├── integration/
│       │   └── integration.go
│       │
│       ├── patch/
│       │   └── ...                # Strategic merge patch helpers
│       │
│       ├── version/
│       │   └── ...
│       │
│       ├── sets/
│       │   └── ...                # Set utilities
│       │
│       ├── assert/
│       │   └── ...
│       │
│       └── env/
│           └── ...                # Environment variable helpers
│
├── kinder-site/
│   ├── src/
│   │   ├── pages/               # Astro pages
│   │   ├── components/          # Astro components (reusable UI)
│   │   ├── content/
│   │   │   └── docs/
│   │   │       ├── addons/      # Addon documentation
│   │   │       ├── guides/      # User guides (v2.2 feature guides)
│   │   │       └── cli-reference/  # CLI reference docs
│   │   ├── assets/              # Static assets (images, etc.)
│   │   ├── styles/              # Global styles
│   │   └── content.config.ts    # Content collection config
│   ├── package.json             # Astro + dependencies
│   ├── astro.config.mjs         # Astro configuration
│   └── README.md
│
├── images/                       # Container images
│   ├── base/                    # Base image
│   ├── node/                    # Node image
│   ├── kindnetd/                # Network pod image
│   ├── local-path-provisioner/
│   ├── local-path-helper/
│   ├── haproxy/                 # Load balancer image
│   └── README files
│
├── hack/
│   ├── build/                   # Build scripts
│   ├── ci/                      # CI/CD configurations
│   └── tools/                   # Build tools definitions
│
├── .planning/                    # Planning documents
│   └── codebase/                # THIS DIRECTORY
│       ├── ARCHITECTURE.md      # Architecture patterns
│       └── STRUCTURE.md         # Directory structure (this file)
│
├── go.mod                       # Go module definition
├── go.sum                       # Go module checksums
├── Makefile                     # Build targets
├── main.go                      # Package main entry point
└── cmd/kind/main.go             # CLI entry point
```

## Directory Purposes

**`pkg/cmd/kind/`:** CLI command implementations using Cobra framework. Each subdirectory is a command (create, delete, doctor, load, build, export, get, version, env, completion). Root command defined in `root.go`.

**`pkg/cluster/`:** Public cluster management API. Provider interface, options builders, and delegation to internal implementations.

**`pkg/cluster/internal/create/`:** Cluster creation flow orchestration. Main `Cluster()` function implements: provision → sequential core setup (kubeadm init, CNI, join) → export kubeconfig → wave-based addon installation (Wave 1 parallel, Wave 2 sequential).

**`pkg/cluster/internal/create/actions/`:** Plugin implementations for cluster setup steps. Each action (kubeadm init, CNI, addon installs) implements `Action` interface. Manifests embedded as string constants for helm/kubectl deployment.

**`pkg/cluster/internal/providers/{docker,podman,nerdctl}/`:** Runtime-specific implementations of `providers.Provider` interface. Docker is the default; Podman and Nerdctl are experimental.

**`pkg/internal/doctor/`:** Diagnostic checks. 23 checks across 8 categories (Runtime, Docker, Tools, GPU, Kernel, Security, Platform, Network, Cluster, Offline, Mounts). Check interface, registry, platform filtering, result formatting.

**`pkg/internal/apis/config/`:** Cluster configuration schema and validation. Defines Cluster, Node, Addons, Networking, Mount types. Supports YAML/JSON encoding, kubeadm/containerd patches, feature gates.

**`kinder-site/`:** Astro-based documentation website. Content sourced from markdown files in `src/content/docs/`. Built statically, deployed to Netlify.

**`hack/`:** Build and CI tooling. Scripts for compiling, testing, linting; CI/CD workflow definitions.

## Key File Locations

**Entry Points:**
- `cmd/kind/main.go`: Stub that imports `cmd/kind/app`
- `cmd/kind/app/main.go`: Real entry point; `Main()` initializes logger and calls `Run()`

**Configuration:**
- `pkg/internal/apis/config/types.go`: Cluster, Node, Addons, Networking schemas
- `pkg/internal/apis/config/validate.go`: Config validation logic
- `pkg/internal/apis/config/encoding/`: YAML/JSON parsing

**Core Logic:**
- `pkg/cluster/provider.go`: Provider interface, NewProvider, Create/Delete options
- `pkg/cluster/internal/create/create.go`: Main Cluster() function (provision, sequential setup, wave addons)
- `pkg/cluster/internal/providers/{docker,podman,nerdctl}/provider.go`: Runtime implementations

**Diagnostics:**
- `pkg/internal/doctor/check.go`: Check interface, registry, RunAllChecks()
- `pkg/internal/doctor/{daemon,disk,apparmor,selinux,firewalld,wsl2,inotify,kernel,gpu,kubectl,clusterskew,localpath,offline,hostmount,docker_desktop_file_sharing}.go`: Individual checks
- `pkg/cmd/kind/doctor/doctor.go`: CLI integration

**Addons:**
- `pkg/cluster/internal/create/actions/install{metallb,envoygateway,localregistry,metricsserver,certmanager,corednstuning,dashboard,localpath,nvidiagpu}/`: Addon implementations
- Each addon has a `{addon}.go` file with NewAction() and Execute() implementation

**Website:**
- `kinder-site/astro.config.mjs`: Astro build configuration
- `kinder-site/src/content/docs/`: Markdown content (addons, guides, CLI reference)
- `kinder-site/src/components/`: Reusable UI components

**Testing:**
- Test files co-located: `*_test.go` alongside source files

## Naming Conventions

**Files:**
- Source files: `*.go` (command, provider, check implementations)
- Test files: `*_test.go` (unit tests)
- Config files: `config.toml`, `.env` (example), `go.mod`
- Markdown docs: `*.md`

**Directories:**
- Command groups: lowercase, matching Cobra command name (e.g., `create/`, `delete/`, `doctor/`)
- Package prefixes for clarity: `internal/` (not exported), `pkg/` (exported library)
- Addon implementations: `install{AddonName}/` (e.g., `installmetallb/`)
- Provider implementations: provider name (e.g., `docker/`, `podman/`)

**Functions:**
- Exported (public): PascalCase (e.g., `NewProvider()`, `Create()`, `NewCommand()`)
- Unexported (private): camelCase (e.g., `runE()`, `validateProvider()`, `planCreation()`)
- Interface implementations: Typically don't have receiver-specific prefix

**Variables:**
- Config structs: Singular noun (e.g., `opts`, `flags`, `cfg`)
- Receiver variable: Lowercase, often single letter (e.g., `p *Provider`, `c *Cluster`)

**Constants:**
- All caps with underscores (e.g., `DefaultClusterName`, `ControlPlaneRole`)

## Where to Add New Code

**New CLI Command:**
1. Create new directory: `pkg/cmd/kind/{command}/`
2. Define command file: `pkg/cmd/kind/{command}/{command}.go` with `NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command`
3. Register in root: Add `cmd.AddCommand({command}.NewCommand(logger, streams))` in `pkg/cmd/kind/root.go`

**New Addon Installation Action:**
1. Create directory: `pkg/cluster/internal/create/actions/install{AddonName}/`
2. Implement action: `{addon}.go` with `func NewAction() actions.Action` and struct implementing `Execute(ctx *ActionContext) error`
3. Create manifest directory: `pkg/cluster/internal/create/actions/install{AddonName}/manifests/` with embedded YAML strings
4. Register in wave: Add `AddonEntry` to `wave1` or `wave2` slice in `pkg/cluster/internal/create/create.go`
5. Add config field: Add `{AddonName} bool` to `pkg/internal/apis/config/types.go:Addons` struct

**New Diagnostic Check:**
1. Create check file: `pkg/internal/doctor/{check_name}.go`
2. Implement Check interface: `func new{CheckName}Check() doctor.Check` returning struct with Name(), Category(), Platforms(), Run() methods
3. Register: Add to `allChecks` slice in `pkg/internal/doctor/check.go`, ensuring category grouping order
4. (Optional) If needs config: Implement `mountPathConfigurable` interface in `pkg/internal/doctor/check.go`

**New Provider Runtime:**
1. Create directory: `pkg/cluster/internal/providers/{runtime}/`
2. Implement Provider interface: `provider.go` with Provision(), ListClusters(), ListNodes(), DeleteNodes(), GetAPIServerEndpoint(), CollectLogs(), Info()
3. Implement common logic: Reuse from `pkg/cluster/internal/providers/common/` where possible
4. Register detection: Add `IsAvailable()` check in `{runtime}/provider.go`
5. Update Provider detection: Add case in `pkg/cluster/provider.go:DetectNodeProvider()`

**New Website Page/Guide:**
1. Create markdown file: `kinder-site/src/content/docs/{category}/{page}.md`
2. Add frontmatter: `title`, `description`, optional `sidebar` metadata
3. Reference in sidebar: Edit `kinder-site/src/content.config.ts` or `astro.config.mjs` sidebar configuration

**New Utility Package:**
1. Create directory: `pkg/{name}/` (if exported) or `pkg/internal/{name}/` (if internal)
2. Define doc file: `doc.go` with package documentation
3. Implement types/functions: `types.go` for types, `{name}.go` for functions
4. Add tests: `*_test.go` files

## Special Directories

**`pkg/cluster/internal/create/actions/manifests/`:**
- Purpose: Embedded YAML manifests for addon deployments
- Generated: No, hand-written YAML strings
- Committed: Yes, part of source
- Usage: Each addon loads its manifest via `embed.FS` or string constant, applies kustomize, deploys via kubectl

**`images/`:**
- Purpose: Container image Dockerfiles and build contexts
- Generated: Docker images built from these
- Committed: Yes, source Dockerfiles and support files
- Content: Dockerfile, build scripts, base images (node, network, provisioner, haproxy)

**`hack/`:**
- Purpose: Build and development tooling
- Generated: Some outputs (built binaries)
- Committed: Yes, scripts and configs
- Content: Makefile targets, CI/CD configs, linting/testing scripts, code generation

**`.planning/codebase/`:**
- Purpose: Architecture and structure documentation
- Generated: No, written manually
- Committed: Yes
- Files: ARCHITECTURE.md, STRUCTURE.md, CONVENTIONS.md, TESTING.md, CONCERNS.md, STACK.md, INTEGRATIONS.md

---

*Structure analysis: 2026-05-03*
