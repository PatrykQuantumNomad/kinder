# Architecture Patterns

**Domain:** Go CLI — kinder v1.3 feature integration
**Researched:** 2026-03-03
**Confidence:** HIGH — all analysis is from direct codebase read, no inference needed

---

## Recommended Architecture

### System Overview

```
pkg/
├── apis/config/v1alpha4/types.go          ← PUBLIC API: Addons struct (*bool fields)
├── internal/apis/config/types.go          ← INTERNAL: Addons struct (bool fields, no pointers)
│
├── cluster/
│   ├── internal/
│   │   ├── providers/
│   │   │   ├── provider.go                ← Provider interface (unchanged)
│   │   │   ├── common/                    ← SHARED: grows in v1.3 (node.go, provision.go, etc.)
│   │   │   │   ├── cgroups.go             ← WaitUntilLogRegexpMatches (existing)
│   │   │   │   ├── constants.go           ← APIServerInternalPort (existing)
│   │   │   │   ├── images.go              ← RequiredNodeImages (existing)
│   │   │   │   ├── logs.go                ← FileOnHost (existing)
│   │   │   │   ├── namer.go               ← MakeNodeNamer (existing)
│   │   │   │   ├── proxy.go               ← GetProxyEnvs (existing)
│   │   │   │   ├── getport.go             ← PortOrGetFreePort (existing)
│   │   │   │   ├── node.go                ← NEW: shared node + nodeCmd struct
│   │   │   │   └── provision.go           ← NEW: shared planCreation, commonArgs, generateMountBindings, generatePortMappings
│   │   │   ├── docker/
│   │   │   │   ├── provider.go            ← MODIFY: add binaryName field, use common.node, common.planCreation
│   │   │   │   ├── node.go                ← DELETE: replaced by common/node.go
│   │   │   │   ├── provision.go           ← DELETE: replaced by common/provision.go
│   │   │   │   ├── images.go              ← MODIFY: call common.EnsureNodeImages(binaryName)
│   │   │   │   ├── network.go             ← KEEP: docker-specific (removeDuplicateNetworks logic)
│   │   │   │   ├── util.go                ← KEEP: docker-specific (usernsRemap, mountDevMapper)
│   │   │   │   └── constants.go           ← KEEP: clusterLabelKey, nodeRoleLabelKey
│   │   │   ├── nerdctl/
│   │   │   │   ├── provider.go            ← MODIFY: delegate to common.node, common.planCreation
│   │   │   │   ├── node.go                ← DELETE: replaced by common/node.go
│   │   │   │   ├── provision.go           ← DELETE: replaced by common/provision.go
│   │   │   │   ├── images.go              ← MODIFY: call common.EnsureNodeImages(binaryName)
│   │   │   │   ├── network.go             ← KEEP: nerdctl-specific network logic
│   │   │   │   ├── util.go                ← KEEP: IsAvailable, mountFuse (nerdctl-specific)
│   │   │   │   └── constants.go           ← KEEP: clusterLabelKey, nodeRoleLabelKey
│   │   │   └── podman/
│   │   │       ├── provider.go            ← MODIFY: delegate to common.node, common.planCreation
│   │   │       ├── node.go                ← DELETE: replaced by common/node.go
│   │   │       ├── provision.go           ← MODIFY: keep podman-specific runArgsForNode (anon volumes)
│   │   │       ├── images.go              ← KEEP: sanitizeImage differs (podman registry format)
│   │   │       ├── network.go             ← KEEP: podman JSON subnet parsing (different API)
│   │   │       ├── util.go                ← KEEP: createAnonymousVolume, deleteVolumes (podman-only)
│   │   │       └── constants.go           ← KEEP: clusterLabelKey, nodeRoleLabelKey
│   │   │
│   │   └── create/
│   │       ├── create.go                  ← MODIFY: add LocalRegistry, CertManager to runAddon calls
│   │       └── actions/
│   │           ├── action.go              ← UNCHANGED
│   │           ├── installlocalregistry/  ← NEW package
│   │           │   ├── localregistry.go
│   │           │   └── manifests/         ← registry Deployment + Service + configmap YAMLs
│   │           └── installcertmanager/    ← NEW package
│   │               ├── certmanager.go
│   │               └── manifests/         ← cert-manager install YAML (embedded)
│   │
│   └── provider.go                        ← UNCHANGED (public API, wraps internal)
│
└── cmd/kind/
    ├── root.go                            ← MODIFY: add env.NewCommand, doctor.NewCommand
    ├── env/                               ← NEW package
    │   └── env.go                         ← kinder env: prints KUBECONFIG, cluster name, provider
    └── doctor/                            ← NEW package
        └── doctor.go                      ← kinder doctor: checks docker/podman/nerdctl availability + version
```

---

## Component Boundaries

### Existing Components (Unchanged Interface, Possibly Modified Internals)

| Component | Responsibility | What Changes in v1.3 |
|-----------|---------------|----------------------|
| `Provider` interface (`provider.go`) | Provision/list/delete/inspect cluster nodes | Nothing — interface is stable |
| `ActionContext` (`action.go`) | Passes logger, status, provider, config to actions | Nothing — data bag is correct as-is |
| `Addons` struct (internal `config/types.go`) | Holds bool flags for each addon | Add `LocalRegistry bool`, `CertManager bool` fields |
| `Addons` struct (v1alpha4 `types.go`) | Public API with `*bool` fields for YAML config | Add `LocalRegistry *bool`, `CertManager *bool` fields |
| `create.go` | Orchestrates action pipeline and addon runAddon calls | Add `runAddon` calls for LocalRegistry and CertManager |
| `convert_v1alpha4.go` | Converts public config to internal config | Add LocalRegistry + CertManager field conversion |
| `default.go` | Sets addon defaults (true/false) | Add LocalRegistry + CertManager defaults |

### New Components

| Component | Responsibility | Integrates With |
|-----------|---------------|-----------------|
| `common/node.go` | Shared `node` struct + `nodeCmd` struct + all methods (`Role`, `IP`, `Command`, `CommandContext`, `SerialLogs`) | docker, nerdctl, podman providers — each provider's `node()` factory returns `&common.Node{binaryName: ..., name: ...}` |
| `common/provision.go` | Shared `planCreation`, `commonArgs`, `runArgsForNode`, `runArgsForLoadBalancer`, `generateMountBindings`, `generatePortMappings`, `createContainer`, `createContainerWithWaitUntilSystemdReachesMultiUserSystem` — all parameterized with `binaryName string` | docker, nerdctl, podman providers delegate provisioning to this |
| `actions/installlocalregistry/` | Action: run a registry container on the cluster network, patch containerd config on all nodes to mirror `localhost:5001` to it, create a `ConfigMap` in the cluster advertising the registry address | `ActionContext.Nodes()`, `node.Command(kubectl)`, `node.Command(containerd config)` |
| `actions/installcertmanager/` | Action: kubectl apply embedded cert-manager manifest, wait for cert-manager-webhook Deployment to be Available | `ActionContext.Nodes()`, `node.Command(kubectl apply)`, `node.Command(kubectl wait)` |
| `pkg/cmd/kind/env/` | Cobra subcommand: print environment info (which KUBECONFIG would be set, current cluster name, detected provider) | `cluster.Provider.ListClusters()`, `cluster.Provider.Info()`, kubeconfig package |
| `pkg/cmd/kind/doctor/` | Cobra subcommand: check that the active provider binary exists, its version is supported, and connectivity works | `docker.IsAvailable()`, `nerdctl.IsAvailable()`, `podman.IsAvailable()`, provider `Info()` |

---

## Data Flow

### Provider Deduplication: Before and After

**Before (current state):** Each provider package contains its own complete implementation of `node`, `nodeCmd`, and all provisioning functions. `docker/node.go` hardcodes `"docker"` as the binary name string. `podman/node.go` hardcodes `"podman"`. `nerdctl/node.go` stores `binaryName` on the struct (already parameterized).

**After (target state):** `common/node.go` holds one `Node` struct with `binaryName string` and `name string` fields. All `node.*` methods use `n.binaryName` instead of a hardcoded string. Provider packages' `provider.node(name)` factory returns `&common.Node{binaryName: p.binaryName, name: name}`. Docker and podman acquire `binaryName` fields on their provider structs (currently only nerdctl has this). Provisioning logic follows the same pattern via `common/provision.go`.

```
Provider.Provision(cfg)
    |
    v
common.EnsureNodeImages(logger, status, cfg, binaryName)   ← new common func
    |
    v
ensureNetwork(networkName, binaryName)     ← per-provider (network logic differs)
    |
    v
common.PlanCreation(cfg, networkName, binaryName, providerSpecificArgs)
    |                                         ↑
    |                              podman: runArgsForNode has extra anon volume creation
    v
createContainerFuncs []func() error
    |
    v
execute each func → common.createContainer(name, args, binaryName)
                  → common.createContainerWithWaitUntilSystemdReachesMultiUserSystem(...)
```

The key design decision: podman's `runArgsForNode` is NOT fully shareable because it calls `createAnonymousVolume(name)` for the `/var` mount — a podman-specific operation. The shared `provision.go` accepts a `runArgsForNodeFn` function parameter, allowing podman to supply its own `runArgsForNode` while sharing everything else. Alternatively, podman's provision.go can continue to call into the common provision for the container creation parts only, keeping its own `runArgsForNode`. Either pattern works; the second is simpler.

### Local Registry Addon: Data Flow

```
create.go: runAddon("Local Registry", cfg.Addons.LocalRegistry, installlocalregistry.NewAction())
    |
    v
installlocalregistry.Execute(ctx *ActionContext)
    |
    +-- run registry container on host:
    |       exec.Command(ctx.Provider.(fmt.Stringer).String(), "run", "-d",
    |           "--name", "kind-registry", "--network=kind",
    |           "-p", "127.0.0.1:5001:5000", "registry:2")
    |
    +-- for each node (ctx.Nodes()):
    |       node.Command("mkdir", "-p", "/etc/containerd/certs.d/localhost:5001")
    |       node.Command("tee", "/etc/containerd/certs.d/localhost:5001/hosts.toml")
    |           stdin: "[host.\"http://kind-registry:5000\"]\n"
    |
    +-- kubectl apply ConfigMap (registry-info in kube-public namespace)
    |       node[0].Command("kubectl", "--kubeconfig=...", "apply", "-f", "-")
    |           stdin: configmap YAML with registry endpoint
    |
    +-- ctx.Status.End(true)
```

The registry container runs on the `kind` Docker/nerdctl/podman network alongside node containers, so nodes can reach it by container name `kind-registry` over the internal network.

### Cert-Manager Addon: Data Flow

```
create.go: runAddon("cert-manager", cfg.Addons.CertManager, installcertmanager.NewAction())
    |
    v
installcertmanager.Execute(ctx *ActionContext)
    |
    +-- ctx.Status.Start("Installing cert-manager")
    +-- node[0].Command("kubectl", "--kubeconfig=...", "apply", "-f", "-")
    |       stdin: embedded certManagerManifest (//go:embed manifests/cert-manager.yaml)
    |
    +-- node[0].Command("kubectl", "--kubeconfig=...",
    |       "wait", "--namespace=cert-manager",
    |       "--for=condition=Available", "deployment/cert-manager-webhook",
    |       "--timeout=120s")
    |
    +-- ctx.Status.End(true)
```

Pattern is identical to `installdashboard` and `installmetallb`. Embed one YAML file, apply it, wait for a deployment.

### env Command: Data Flow

```
kinder env [--name cluster-name]
    |
    v
cmd/kind/env/env.go: runE()
    |
    +-- create Provider (same as create cluster: runtime.GetDefault(logger))
    +-- detect cluster name: flag > KIND_CLUSTER_NAME env > "kind"
    +-- provider.ListClusters()     ← detect which clusters exist
    +-- provider.Info()             ← detect rootless, provider type
    +-- construct kubeconfig path: kubeconfig.PathForCluster(name)
    |
    +-- print to stdout:
    |       KUBECONFIG=<path>
    |       KIND_CLUSTER_NAME=<name>
    |       KINDER_PROVIDER=<docker|nerdctl|podman>
    |
    (optionally: eval $(kinder env) sets shell vars)
```

Pattern: modeled after `docker machine env` / `minikube docker-env`. Simple read-only command, no side effects.

### doctor Command: Data Flow

```
kinder doctor
    |
    v
cmd/kind/doctor/doctor.go: runE()
    |
    +-- check docker: docker.IsAvailable() → print OK/FAIL + version
    +-- check nerdctl: nerdctl.IsAvailable() → print OK/FAIL + version
    +-- check podman: podman.IsAvailable() → print OK/FAIL + version
    +-- detect active provider: runtime.GetDefault(logger)
    +-- provider.Info() → print cgroup capabilities (rootless, cgroup2, memory limit, etc.)
    +-- check kubeconfig path exists and is readable
    +-- print summary: "All checks passed" or list of failures
    |
    (exit code 0 = all OK, exit code 1 = any failure)
```

---

## Patterns to Follow

### Pattern 1: binaryName Parameterization (nerdctl precedent)

**What:** All provider operations that shell out to a container runtime accept `binaryName string` as a parameter rather than hardcoding the runtime name. The provider struct stores `binaryName` and passes it through.

**When:** Whenever implementing shared code that must work with docker, nerdctl, podman, or finch.

**Example (from existing nerdctl/node.go — the model):**
```go
type node struct {
    name       string
    binaryName string
}

func (n *node) Role() (string, error) {
    cmd := exec.Command(n.binaryName, "inspect", ...)
    // ...
}
```

**Target pattern for common/node.go:**
```go
// Node implements nodes.Node for all container runtime providers.
// The binaryName field holds the runtime binary (docker, nerdctl, podman, finch).
type Node struct {
    name       string
    binaryName string
}

func NewNode(name, binaryName string) *Node {
    return &Node{name: name, binaryName: binaryName}
}
```

### Pattern 2: Action with Embedded Manifest (existing addon pattern)

**What:** Each addon action embeds its YAML manifest(s) using `//go:embed`, applies them via `kubectl apply -f -` with stdin, then waits for a deployment to become Available.

**When:** All new addons (LocalRegistry, CertManager) follow this pattern.

**Example (from installdashboard/dashboard.go — the model):**
```go
//go:embed manifests/headlamp.yaml
var headlampManifest string

func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing cert-manager")
    defer ctx.Status.End(false)

    allNodes, err := ctx.Nodes()
    // ... find controlPlanes[0] ...

    if err := node.Command(
        "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(certManagerManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply cert-manager manifest")
    }

    if err := node.Command(
        "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait", "--namespace=cert-manager",
        "--for=condition=Available", "deployment/cert-manager-webhook",
        "--timeout=120s",
    ).Run(); err != nil {
        return errors.Wrap(err, "cert-manager webhook did not become available")
    }

    ctx.Status.End(true)
    return nil
}
```

### Pattern 3: Addon Config with *bool (existing pattern)

**What:** Public API uses `*bool` for each addon flag. `nil` means "use default". Internal config uses plain `bool`. `convert_v1alpha4.go` converts with a `boolOrDefault(field *bool, defaultVal bool)` helper.

**When:** Adding any new addon field to the config types.

**Locations requiring change for each new addon:**
1. `pkg/apis/config/v1alpha4/types.go` — add `LocalRegistry *bool` to `Addons` struct
2. `pkg/internal/apis/config/types.go` — add `LocalRegistry bool` to `Addons` struct
3. `pkg/internal/apis/config/convert_v1alpha4.go` — convert the field
4. `pkg/internal/apis/config/default.go` — set default value (true or false)
5. `pkg/cluster/internal/create/create.go` — add `runAddon()` call

### Pattern 4: Cobra Subcommand (existing pattern)

**What:** Each top-level command group has a parent command in `pkg/cmd/kind/<name>/<name>.go` that adds subcommands. Leaf commands define `flagpole` struct, `NewCommand()`, and `runE()`. Logger and IOStreams flow through all levels.

**When:** Adding `env` and `doctor` commands.

**Example (from pkg/cmd/kind/get/get.go — the model):**
```go
package env

import (
    "github.com/spf13/cobra"
    "sigs.k8s.io/kind/pkg/cmd"
    "sigs.k8s.io/kind/pkg/log"
)

type flagpole struct {
    Name string
}

func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
    flags := &flagpole{}
    cmd := &cobra.Command{
        Use:   "env",
        Short: "Print environment variables for a cluster",
        RunE: func(cmd *cobra.Command, args []string) error {
            return runE(logger, streams, flags)
        },
    }
    cmd.Flags().StringVarP(&flags.Name, "name", "n", "", "cluster name")
    return cmd
}
```

**Then in root.go:**
```go
import "sigs.k8s.io/kind/pkg/cmd/kind/env"
// ...
cmd.AddCommand(env.NewCommand(logger, streams))
```

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Sharing podman's runArgsForNode With Docker/Nerdctl

**What goes wrong:** Podman's `runArgsForNode` calls `createAnonymousVolume(name)` before building args, and uses a different `--volume` syntax with `:suid,exec,dev` options. Docker's version uses `"--volume", "/var"` (anonymous, no label). These are not the same.

**Why bad:** Forcing them into one function requires a provider-type switch inside the shared code, defeating the purpose.

**Instead:** Keep `runArgsForNode` in each provider package. Only share `generateMountBindings`, `generatePortMappings`, `createContainer`, and `createContainerWithWaitUntilSystemdReachesMultiUserSystem` — those are truly identical. The `planCreation` skeleton (the outer loop) can also be shared if it accepts `runArgsForNodeFn` as a callback.

### Anti-Pattern 2: Making LocalRegistry a Node Container

**What goes wrong:** Running the registry as a kind node (inside a kind node container image) adds complexity and requires modifying the provisioning pipeline.

**Instead:** Run the registry as a separate container on the same Docker/nerdctl/podman network (`kind` network). This is how the kind docs recommend it. The registry container is a standard `registry:2` image, separate from kind nodes, reachable by container name within the network.

### Anti-Pattern 3: Deleting provider-specific node.go Before Common Is Proven

**What goes wrong:** Deleting the per-provider `node.go` files before `common/node.go` compiles and passes tests breaks the build. There are no compile-time guarantees that the common struct satisfies `nodes.Node` until it's wired up and tested.

**Instead:** Build order must be: (1) write `common/node.go`, (2) verify it satisfies `nodes.Node` interface, (3) update providers to use it, (4) delete per-provider `node.go` files, (5) run tests. Do not delete before step 4 is confirmed green.

### Anti-Pattern 4: Embedding Large cert-manager Manifests Verbatim

**What goes wrong:** cert-manager's official install YAML is ~5000 lines. Embedding it verbatim bloats the binary and makes the file hard to review. Worse, it requires manual update with each cert-manager release.

**Instead:** Embed the YAML but process it through the same pattern as other addons — confirm the manifest version at the start of each milestone. Add a `// cert-manager version: X.Y.Z` comment at the top of the embedded YAML file so it's clear what version is baked in. Consider extracting just the CRDs + controller deployments if the full manifest is truly unwieldy.

### Anti-Pattern 5: doctor Command Calling provider.Info() for Non-Active Providers

**What goes wrong:** Calling `podman.info()` when Docker is active requires podman to be installed, which defeats the purpose of a diagnostic command.

**Instead:** `doctor` checks `IsAvailable()` for each provider binary, then only calls `Info()` on the detected active provider. Unavailable providers are reported as "not found" without attempting further introspection.

---

## Integration Points: New vs. Modified

### Modified Files

| File | Nature of Change |
|------|-----------------|
| `pkg/apis/config/v1alpha4/types.go` | Add `LocalRegistry *bool`, `CertManager *bool` to `Addons` struct |
| `pkg/internal/apis/config/types.go` | Add `LocalRegistry bool`, `CertManager bool` to `Addons` struct |
| `pkg/internal/apis/config/convert_v1alpha4.go` | Convert new `*bool` fields to `bool` |
| `pkg/internal/apis/config/default.go` | Set defaults for `LocalRegistry` and `CertManager` |
| `pkg/cluster/internal/create/create.go` | Add `runAddon` calls for `LocalRegistry` (before CertManager) and `CertManager` |
| `pkg/cmd/kind/root.go` | Add `cmd.AddCommand(env.NewCommand(...))` and `cmd.AddCommand(doctor.NewCommand(...))` |
| `pkg/cluster/internal/providers/docker/provider.go` | Add `binaryName string` field; wire `binaryName: "docker"`; use `common.Node` in `node()` factory |
| `pkg/cluster/internal/providers/podman/provider.go` | Use `common.Node` in `node()` factory |
| `pkg/cluster/internal/providers/nerdctl/provider.go` | Use `common.Node` in `node()` factory (already has `binaryName`) |
| `pkg/cluster/internal/providers/docker/images.go` | Delegate to common image-pull function (accept binaryName) |
| `pkg/cluster/internal/providers/nerdctl/images.go` | Delegate to common image-pull function |

### New Files

| File | Purpose |
|------|---------|
| `pkg/cluster/internal/providers/common/node.go` | Shared `Node` struct, `nodeCmd` struct, all `nodes.Node` interface methods |
| `pkg/cluster/internal/providers/common/provision.go` | Shared `generateMountBindings`, `generatePortMappings`, `createContainer`, `createContainerWithWaitUntilSystemdReachesMultiUserSystem` |
| `pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go` | Local registry Action implementation |
| `pkg/cluster/internal/create/actions/installlocalregistry/manifests/` | Registry ConfigMap YAML for cluster advertisement |
| `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go` | cert-manager Action implementation |
| `pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml` | Embedded cert-manager install manifest |
| `pkg/cmd/kind/env/env.go` | `kinder env` Cobra command |
| `pkg/cmd/kind/doctor/doctor.go` | `kinder doctor` Cobra command |

### Deleted Files

| File | Reason |
|------|--------|
| `pkg/cluster/internal/providers/docker/node.go` | Replaced by `common/node.go` |
| `pkg/cluster/internal/providers/nerdctl/node.go` | Replaced by `common/node.go` |
| `pkg/cluster/internal/providers/podman/node.go` | Replaced by `common/node.go` |
| `pkg/cluster/internal/providers/docker/provision.go` | Replaced by `common/provision.go` + thin docker wrapper |
| `pkg/cluster/internal/providers/nerdctl/provision.go` | Replaced by `common/provision.go` + thin nerdctl wrapper |

Note: `podman/provision.go` is MODIFIED not deleted, because `runArgsForNode` (anonymous volume creation) is podman-specific.

---

## Build Order Recommendation

This order respects compile-time dependencies and allows incremental validation.

```
Step 1: Provider code deduplication (foundational, de-risks all else)
    1a. Write common/node.go — verify nodes.Node interface is satisfied
    1b. Update docker/provider.go: add binaryName, use common.Node in node()
    1c. Update nerdctl/provider.go: use common.Node in node()
    1d. Update podman/provider.go: use common.Node in node()
    1e. Delete docker/node.go, nerdctl/node.go, podman/node.go
    1f. Run: go build ./... (must compile)
    1g. Run: go test ./pkg/cluster/internal/providers/...

    1h. Write common/provision.go — shared generateMountBindings, generatePortMappings, createContainer
    1i. Update docker/provision.go: delegate to common, delete duplicated functions
    1j. Update nerdctl/provision.go: delegate to common, delete duplicated functions
    1k. Podman/provision.go: keep runArgsForNode, delegate rest to common
    1l. Run: go build ./... + go test ./...

Step 2: Config types for new addons (required before action code references cfg.Addons.*)
    2a. Add LocalRegistry, CertManager to v1alpha4/types.go
    2b. Add LocalRegistry, CertManager to internal config/types.go
    2c. Update convert_v1alpha4.go
    2d. Update default.go (set defaults)
    2e. Run: go build ./... (config package must compile before actions reference it)

Step 3: Local Registry addon
    3a. Create actions/installlocalregistry/manifests/ (registry-info ConfigMap YAML)
    3b. Implement actions/installlocalregistry/localregistry.go
    3c. Wire into create.go runAddon (before CertManager in dependency order)
    3d. Manual test: kinder create cluster, verify registry reachable from node

Step 4: cert-manager addon
    4a. Download cert-manager.yaml, place in actions/installcertmanager/manifests/
    4b. Implement actions/installcertmanager/certmanager.go
    4c. Wire into create.go runAddon
    4d. Manual test: kinder create cluster, verify cert-manager-webhook Available

Step 5: CLI commands (env, doctor)
    5a. Implement pkg/cmd/kind/env/env.go
    5b. Implement pkg/cmd/kind/doctor/doctor.go
    5c. Register both in pkg/cmd/kind/root.go
    5d. Run: go build ./...; kinder env --help; kinder doctor

Step 6: Bug fixes (independent of steps 1-5, can be done in any order)
```

---

## Duplication Analysis: What Is Actually Shared

The three provider implementations share the following code verbatim (or with only a binaryName parameter difference). This is what goes into `common/`:

| Code Unit | Docker | Nerdctl | Podman | Shareable? |
|-----------|--------|---------|--------|------------|
| `node` struct fields | name string | name+binaryName | name string | YES — add binaryName to all |
| `node.String()` | identical | identical | identical | YES |
| `node.Role()` | identical except binary | identical | identical | YES (binaryName param) |
| `node.IP()` | identical except binary | identical | identical | YES (binaryName param) |
| `node.Command()` | identical except binary | identical | identical | YES (binaryName param) |
| `node.CommandContext()` | identical except binary | identical | identical | YES (binaryName param) |
| `node.SerialLogs()` | identical except binary | identical | identical | YES (binaryName param) |
| `nodeCmd` struct | no binaryName | has binaryName | no binaryName | YES — add binaryName to all |
| `nodeCmd.Run()` | identical except binary | identical | identical | YES (binaryName param) |
| `generateMountBindings()` | identical | identical | identical | YES |
| `generatePortMappings()` | identical | identical | podman: empty string for 0 | MOSTLY — podman has one extra line; factor out or use a flag |
| `createContainer()` | identical except binary | identical | identical | YES (binaryName param) |
| `createContainerWithWait...()` | identical except binary | identical | identical | YES (binaryName param) |
| `getSubnets()` | docker JSON format | same | podman JSON — DIFFERENT | NO — keep per-provider |
| `runArgsForNode()` | standard mounts | identical to docker | anon volume for /var | PARTIALLY — podman differs |
| `ensureNetwork()` | docker network API + duplicate removal | nerdctl network API | podman JSON network API | NO — all three differ |
| `info()` / `Info()` | docker JSON | same as docker | podman JSON — different struct | NO — all three differ |

---

## Scalability Considerations

| Concern | Now (3 providers) | After deduplication | If 4th provider added |
|---------|-------------------|--------------------|-----------------------|
| Adding new node method | 3 files to update | 1 file (common/node.go) | 1 file |
| Adding new provision arg | 3 files to update | 1-2 files | 1-2 files |
| Testing coverage | 3x test surface | 1x shared + per-provider specifics | Same |
| Adding new addon | 1 action pkg + 1 line in create.go + 5 config locations | Same (no change from dedup) | Same |

---

## Sources

All findings from direct codebase read at `/Users/patrykattc/work/git/kinder` — HIGH confidence.

Key files examined:
- `pkg/cluster/internal/providers/{docker,nerdctl,podman}/{provider,node,provision,images,network,util,constants}.go`
- `pkg/cluster/internal/providers/common/*.go`
- `pkg/cluster/internal/providers/provider.go`
- `pkg/cluster/internal/create/create.go`
- `pkg/cluster/internal/create/actions/action.go`
- `pkg/cluster/internal/create/actions/{installdashboard,installmetallb}/`
- `pkg/apis/config/v1alpha4/types.go`
- `pkg/internal/apis/config/types.go`
- `pkg/cmd/kind/root.go`
- `pkg/cmd/kind/get/get.go`
- `pkg/cmd/kind/create/cluster/createcluster.go`

---
*Architecture research for: kinder v1.3 — local registry addon, cert-manager addon, env/doctor commands, provider code deduplication*
*Researched: 2026-03-03*
