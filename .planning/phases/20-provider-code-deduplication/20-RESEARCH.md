# Phase 20: Provider Code Deduplication - Research

**Researched:** 2026-03-03
**Domain:** Go package refactoring — extracting shared code from docker/nerdctl/podman providers into common/
**Confidence:** HIGH

## Summary

This phase eliminates copy-paste duplication across three container runtime providers (docker, nerdctl, podman) by moving the shared `node` struct and the shared provision helpers (`generateMountBindings`, `generatePortMappings`, `createContainer`) into the existing `common/` package that already holds other shared code (proxy, namer, ports, logs, images).

All three providers already import `sigs.k8s.io/kind/pkg/cluster/internal/providers/common`. The `common/` package structure and import path are well-established. No new external dependencies are needed. The work is purely mechanical refactoring within the existing module.

The critical challenge is that the three `node.go` implementations are NOT identical: docker and podman hardcode their binary name (literal `"docker"` / `"podman"`), while nerdctl already carries `binaryName string` as a struct field. The shared `Node` must carry `binaryName` as a parameter so all three providers can reuse it. Docker instantiates `&node{name: name}` (binary fixed at `"docker"`), podman instantiates `&node{name: name}` (binary fixed at `"podman"`), nerdctl instantiates `&node{binaryName: p.binaryName, name: name}`. After extraction each provider's `node()` factory calls `&common.Node{Name: name, BinaryName: "docker"}` etc.

The `generatePortMappings` function has one behavioral divergence: podman lowercases the protocol string and strips trailing `:0` from the host port binding. Docker and nerdctl use the protocol as-is (uppercase). This means the common function needs either a parameter for protocol-casing behavior, or podman keeps its own `generatePortMappings` and only docker+nerdctl move to common. The phase description and requirements say docker/nerdctl provision.go is deleted but podman/provision.go is NOT deleted (only docker and nerdctl). This matches reality — podman has unique provision differences.

**Primary recommendation:** Extract `Node`/`nodeCmd` to `common/node.go` with `BinaryName string` field; extract `generateMountBindings`, `generatePortMappings` (docker/nerdctl behavior), and `createContainer` to `common/provision.go`; update each provider's factory method to use the common types; delete docker/node.go, nerdctl/node.go, podman/node.go, docker/provision.go, nerdctl/provision.go; podman/provision.go is retained (podman has unique provision behavior).

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PROV-01 | Extract shared node.go to common/ package with binaryName parameter | Nerdctl's node.go is the template — it already has `binaryName string`. Docker/podman harden the binary name as a literal; extracting to common with `BinaryName string` covers all three. The provider factory method updates are straightforward. |
| PROV-02 | Extract shared provision.go functions (generateMountBindings, generatePortMappings, createContainer) to common/ | `generateMountBindings` is byte-for-byte identical across all three. `generatePortMappings` is identical between docker and nerdctl; podman diverges on protocol casing and `:0` trimming, so only docker+nerdctl provisions.go can be deleted. `createContainer` differs only by binary — a `binaryName string` parameter covers it. |
| PROV-03 | Update go.mod minimum to go 1.21.0 with toolchain go1.26.0 | Current go.mod has `go 1.17`. go1.21.0 introduced the `toolchain` directive. The running toolchain is go1.26.0 (confirmed by `go version`). The syntax is `go 1.21.0` on one line + `toolchain go1.26.0` on the next, with no new external dependencies. |
</phase_requirements>

## Standard Stack

### Core

| Component | Version | Purpose | Notes |
|-----------|---------|---------|-------|
| Go stdlib | 1.21.0+ | Standard language features | No new features needed beyond 1.17; 1.21.0 is minimum for `toolchain` directive |
| `sigs.k8s.io/kind/pkg/exec` | existing | Command execution abstraction | Already used by all providers |
| `sigs.k8s.io/kind/pkg/errors` | existing | Error wrapping | Already used by all providers |
| `sigs.k8s.io/kind/pkg/internal/apis/config` | existing | Config types for Mount/PortMapping | Already imported in provision.go files |

### Supporting

| Component | Purpose | Notes |
|-----------|---------|-------|
| `common/` package (`sigs.k8s.io/kind/pkg/cluster/internal/providers/common`) | Target for extracted code | Already has constants.go, getport.go, namer.go, proxy.go, cgroups.go, images.go, logs.go |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Exported types in common/ | Interface in common/ | Using concrete struct with BinaryName is simpler; interface adds indirection without benefit since callers already use the `nodes.Node` interface from a higher package |
| One shared generatePortMappings | Two versions (common + podman) | Podman's protocol lowercasing and `:0` stripping are behavioral differences that affect test assertions; a flag parameter would work but keeps podman coupled to the common function; cleaner to leave podman/provision.go as-is |

**Installation:** No new packages required. All dependencies already present in go.mod.

## Architecture Patterns

### Existing Package Structure (after Phase 20)

```
pkg/cluster/internal/providers/
├── common/
│   ├── cgroups.go          # WaitUntilLogRegexpMatches, NodeReachedCgroupsReadyRegexp
│   ├── constants.go        # APIServerInternalPort = 6443
│   ├── doc.go              # package comment
│   ├── getport.go          # PortOrGetFreePort, GetFreePort
│   ├── images.go           # RequiredNodeImages
│   ├── logs.go             # FileOnHost
│   ├── namer.go            # MakeNodeNamer
│   ├── node.go             # NEW: Node struct, nodeCmd struct (exported)
│   ├── provision.go        # NEW: GenerateMountBindings, GeneratePortMappings, CreateContainer
│   └── proxy.go            # GetProxyEnvs
├── docker/
│   ├── constants.go        # clusterLabelKey, nodeRoleLabelKey (KEEP — package-private)
│   ├── images.go           # ensureNodeImages (KEEP)
│   ├── network.go          # ensureNetwork, getSubnets (KEEP — docker-specific)
│   ├── network_test.go     # KEEP
│   ├── provider.go         # KEEP — update node() factory to use common.Node
│   ├── provision.go        # DELETE
│   ├── provision_test.go   # MIGRATE to common/provision_test.go (or keep as wrapper)
│   └── util.go             # KEEP — usernsRemap, mountDevMapper, mountFuse (docker-specific)
├── nerdctl/
│   ├── constants.go        # clusterLabelKey, nodeRoleLabelKey (KEEP)
│   ├── images.go           # ensureNodeImages (KEEP)
│   ├── network.go          # ensureNetwork, getSubnets (KEEP — nerdctl-specific)
│   ├── provider.go         # KEEP — update node() factory to use common.Node
│   ├── provision.go        # DELETE
│   ├── provision_test.go   # MIGRATE to common/provision_test.go
│   └── util.go             # KEEP — mountFuse, IsAvailable (nerdctl-specific)
└── podman/
    ├── constants.go        # clusterLabelKey, nodeRoleLabelKey (KEEP)
    ├── images.go           # ensureNodeImages (KEEP)
    ├── network.go          # KEEP — podman-specific getSubnets (JSON parsing)
    ├── provider.go         # KEEP — update node() factory to use common.Node
    ├── provision.go        # KEEP — podman has unique generatePortMappings behavior
    ├── provision_test.go   # KEEP — tests podman-specific port behavior
    └── util.go             # KEEP — podman-specific volume management
```

### Pattern 1: Exported Node Struct in common/

**What:** Move the `node` + `nodeCmd` structs from per-provider packages into `common/`, exported, with `BinaryName string` as a field. Each provider's `node()` factory method returns `&common.Node{BinaryName: "docker", Name: name}`.

**When to use:** When all three providers have structurally identical types that differ only by the binary name string.

**Example:**

```go
// common/node.go
package common

import (
    "context"
    "io"

    "sigs.k8s.io/kind/pkg/errors"
    "sigs.k8s.io/kind/pkg/exec"
)

// Node implements nodes.Node for docker/nerdctl/podman providers.
// BinaryName controls which container runtime CLI is invoked.
type Node struct {
    Name       string
    BinaryName string
}

func (n *Node) String() string { return n.Name }

func (n *Node) Role() (string, error) {
    cmd := exec.Command(n.BinaryName, "inspect",
        "--format", fmt.Sprintf(`{{ index .Config.Labels "%s"}}`, nodeRoleLabelKey),
        n.Name,
    )
    lines, err := exec.OutputLines(cmd)
    if err != nil {
        return "", errors.Wrap(err, "failed to get role for node")
    }
    if len(lines) != 1 {
        return "", errors.Errorf("failed to get role for node: output lines %d != 1", len(lines))
    }
    return lines[0], nil
}
// ... IP(), Command(), CommandContext(), SerialLogs() follow same pattern
```

Note: `nodeRoleLabelKey` used in `Role()` is currently package-private in each provider's constants.go. It has the same value (`"io.x-k8s.kind.role"`) in all three. For `common/node.go` to use it, either move this constant to `common/constants.go`, or pass it as a parameter to `Role()`. Moving it to `common/constants.go` is the cleaner approach.

**Provider factory update:**

```go
// docker/provider.go
func (p *provider) node(name string) nodes.Node {
    return &common.Node{
        Name:       name,
        BinaryName: "docker",
    }
}

// nerdctl/provider.go
func (p *provider) node(name string) nodes.Node {
    return &common.Node{
        Name:       name,
        BinaryName: p.binaryName,
    }
}

// podman/provider.go
func (p *provider) node(name string) nodes.Node {
    return &common.Node{
        Name:       name,
        BinaryName: "podman",
    }
}
```

### Pattern 2: Exported Provision Functions in common/

**What:** Move `generateMountBindings`, `generatePortMappings` (docker/nerdctl flavor), and `createContainer` to `common/provision.go` as exported functions.

**When to use:** When functions are byte-for-byte identical across providers (mountBindings) or differ only by a binary name parameter (createContainer).

**Example:**

```go
// common/provision.go
package common

import (
    "fmt"
    "net"
    "strings"

    "sigs.k8s.io/kind/pkg/errors"
    "sigs.k8s.io/kind/pkg/exec"
    "sigs.k8s.io/kind/pkg/internal/apis/config"
)

// GenerateMountBindings converts the mount list to container run args.
// Format: '<HostPath>:<ContainerPath>[:options]'
func GenerateMountBindings(mounts ...config.Mount) []string {
    args := make([]string, 0, len(mounts))
    for _, m := range mounts {
        bind := fmt.Sprintf("%s:%s", m.HostPath, m.ContainerPath)
        var attrs []string
        if m.Readonly {
            attrs = append(attrs, "ro")
        }
        if m.SelinuxRelabel {
            attrs = append(attrs, "Z")
        }
        switch m.Propagation {
        case config.MountPropagationNone:
            // noop
        case config.MountPropagationBidirectional:
            attrs = append(attrs, "rshared")
        case config.MountPropagationHostToContainer:
            attrs = append(attrs, "rslave")
        }
        if len(attrs) > 0 {
            bind = fmt.Sprintf("%s:%s", bind, strings.Join(attrs, ","))
        }
        args = append(args, fmt.Sprintf("--volume=%s", bind))
    }
    return args
}

// GeneratePortMappings converts portMappings to container run args.
// protocol is left as-is (uppercase, as docker/nerdctl expect).
// Podman callers should use their own version that lowercases protocol.
func GeneratePortMappings(clusterIPFamily config.ClusterIPFamily, portMappings ...config.PortMapping) ([]string, error) {
    // ... identical to docker/nerdctl implementation
}

// CreateContainer runs "binaryName run --name name args..."
func CreateContainer(binaryName, name string, args []string) error {
    return exec.Command(binaryName, append([]string{"run", "--name", name}, args...)...).Run()
}
```

### Pattern 3: go.mod Update

**What:** Update `go` directive from `1.17` to `1.21.0` and add `toolchain go1.26.0`.

**When to use:** When raising the minimum Go version to match actual toolchain in use.

**Example:**

```
module sigs.k8s.io/kind

go 1.21.0

toolchain go1.26.0

require (
    ...
)
```

The `toolchain` directive was introduced in Go 1.21. It pins the exact toolchain version used for builds, without requiring consumers to upgrade beyond the `go` minimum. This matches the `.go-version` file which specifies `1.25.7` (the CI pinned version) and the running toolchain which is `go1.26.0`.

### Anti-Patterns to Avoid

- **Exporting `clusterLabelKey` and `nodeRoleLabelKey` from each provider's constants.go instead of moving to common:** These identical constants (same value in all three providers) belong in `common/constants.go` once `Node.Role()` moves to common.
- **Moving podman's `generatePortMappings` to common:** Podman lowercases the protocol (`strings.ToLower(protocol)`) and strips trailing `:0` from the host binding. This behavior is tested explicitly in `podman/provision_test.go`. Forcing a "podman mode" flag into the common function creates a leaky abstraction. Leave `podman/provision.go` intact.
- **Moving tests without updating package declarations:** Tests in `docker/provision_test.go` and `nerdctl/provision_test.go` use `package docker` / `package nerdctl` to access unexported functions. After the functions move to `common/`, the tests must move to `package common` (or `package common_test`).
- **Using `defer` inside the port mapping loop:** The existing code already correctly uses an explicit `if releaseHostPortFn != nil { releaseHostPortFn() }` (not defer) to avoid the defer-in-loop bug. The common version must preserve this pattern.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Port listener release | Custom RAII wrapper | Existing `releaseHostPortFn()` pattern | Already correctly implemented; just preserve it in the common function |
| Protocol lowercasing for podman | Flag parameter in common.GeneratePortMappings | Keep podman's own generatePortMappings | Behavioral difference is intentional; mixing into common adds complexity |
| Binary name dispatch | Switch statement per method | `BinaryName string` field on Node | The nerdctl provider already proved this pattern works |

**Key insight:** The nerdctl provider already implemented the correct pattern (BinaryName as a struct field) that docker and podman did not. The common implementation should follow nerdctl's approach exactly.

## Common Pitfalls

### Pitfall 1: nodeRoleLabelKey Visibility

**What goes wrong:** `common.Node.Role()` needs `nodeRoleLabelKey` but it's currently defined as package-private in each provider's `constants.go`. The constant has the same value (`"io.x-k8s.kind.role"`) in all three.

**Why it happens:** The constant was never moved to `common/` because `node.go` previously lived in each provider package.

**How to avoid:** Add `nodeRoleLabelKey = "io.x-k8s.kind.role"` to `common/constants.go` alongside `APIServerInternalPort`. Each provider's `constants.go` retains `clusterLabelKey` for use in `provider.go` (which is not moving).

**Warning signs:** Compiler error "undefined: nodeRoleLabelKey" when building common/node.go.

### Pitfall 2: Test Package Declarations

**What goes wrong:** Tests in `docker/provision_test.go` and `nerdctl/provision_test.go` declare `package docker` / `package nerdctl` to call unexported functions. After moving functions to common, these tests no longer compile (the functions no longer exist in those packages).

**Why it happens:** Package-private test access is standard Go pattern; moving the code changes the access context.

**How to avoid:** The provision tests should be migrated to `common/provision_test.go` (package `common`) since they will now be testing exported `common.GenerateMountBindings` and `common.GeneratePortMappings`. The docker-specific subnet parsing test (`Test_parseSubnetOutput`) should stay in `docker/provision_test.go` if it is testing docker-private code, or move to `docker/network_test.go` if it is testing network-related parsing.

**Warning signs:** `go test ./...` fails with "undefined: generatePortMappings" after deletion of provision.go.

### Pitfall 3: podman/provision.go Must Remain

**What goes wrong:** Phase requirements say "docker/provision.go and nerdctl/provision.go are deleted". Podman/provision.go is NOT in the deletion list. Accidentally deleting podman/provision.go breaks podman's unique port mapping behavior.

**Why it happens:** Podman's `generatePortMappings` lowercases protocol and strips `:0` from host binding; docker/nerdctl do not. The `podman/provision_test.go` explicitly tests these differences (e.g., `want := "--publish=0.0.0.0:8080:80/tcp"` with lowercase `tcp`).

**How to avoid:** Podman provision stays. Docker and nerdctl's `generatePortMappings` and `generateMountBindings` are replaced with calls to `common.GeneratePortMappings` and `common.GenerateMountBindings`. Update `planCreation`, `runArgsForNode`, `runArgsForLoadBalancer` in `docker/provision.go` and `nerdctl/provision.go` to call the common versions, then delete those files after all callers are updated.

**Warning signs:** `podman/provision_test.go` "static port produces correct publish arg" fails if podman accidentally adopts the uppercase-protocol common version.

### Pitfall 4: go.mod toolchain Directive Syntax

**What goes wrong:** The `toolchain` directive was introduced in Go 1.21. Writing `toolchain go1.26.0` with an older `go` directive (e.g., `go 1.17`) causes `go mod tidy` to warn or error.

**Why it happens:** The `toolchain` line requires `go 1.21.0` or higher as the minimum.

**How to avoid:** Update `go` directive to `go 1.21.0` first, then add `toolchain go1.26.0`. Both changes should be in the same commit. The `.go-version` file (`1.25.7`) is separate from go.mod — it controls CI toolchain pinning via GOTOOLCHAIN; do not change it.

**Warning signs:** `go build ./...` emits warnings about toolchain directive when go directive is below 1.21.

### Pitfall 5: context.Context Import in common/node.go

**What goes wrong:** `Node.CommandContext` uses `context.Context`. If the import is missing in `common/node.go`, it will fail to compile.

**Why it happens:** Mechanical copy without verifying all imports.

**How to avoid:** Include all imports from the nerdctl `node.go` (context, fmt, io, strings, errors, exec) in `common/node.go`. Verify with `go build ./...` after each file creation.

## Code Examples

### common/node.go — exported Node struct

```go
// Source: nerdctl/node.go (template — already uses binaryName pattern)
package common

import (
    "context"
    "fmt"
    "io"
    "strings"

    "sigs.k8s.io/kind/pkg/errors"
    "sigs.k8s.io/kind/pkg/exec"
)

// NodeRoleLabelKey is the label key for node role, used by Node.Role().
// Exported so provider.go files can reference it for ListNodes filters.
const NodeRoleLabelKey = "io.x-k8s.kind.role"

// Node implements nodes.Node for docker/nerdctl/podman providers.
// Set BinaryName to the container runtime CLI ("docker", "nerdctl", "podman", "finch").
type Node struct {
    Name       string
    BinaryName string
}

func (n *Node) String() string { return n.Name }

func (n *Node) Role() (string, error) {
    cmd := exec.Command(n.BinaryName, "inspect",
        "--format", fmt.Sprintf(`{{ index .Config.Labels "%s"}}`, NodeRoleLabelKey),
        n.Name,
    )
    lines, err := exec.OutputLines(cmd)
    if err != nil {
        return "", errors.Wrap(err, "failed to get role for node")
    }
    if len(lines) != 1 {
        return "", errors.Errorf("failed to get role for node: output lines %d != 1", len(lines))
    }
    return lines[0], nil
}

func (n *Node) IP() (ipv4 string, ipv6 string, err error) {
    cmd := exec.Command(n.BinaryName, "inspect",
        "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}},{{.GlobalIPv6Address}}{{end}}",
        n.Name,
    )
    lines, err := exec.OutputLines(cmd)
    if err != nil {
        return "", "", errors.Wrap(err, "failed to get container details")
    }
    if len(lines) != 1 {
        return "", "", errors.Errorf("file should only be one line, got %d lines", len(lines))
    }
    ips := strings.Split(lines[0], ",")
    if len(ips) != 2 {
        return "", "", errors.Errorf("container addresses should have 2 values, got %d values", len(ips))
    }
    return ips[0], ips[1], nil
}

func (n *Node) Command(command string, args ...string) exec.Cmd {
    return &nodeCmd{
        binaryName: n.BinaryName,
        nameOrID:   n.Name,
        command:    command,
        args:       args,
    }
}

func (n *Node) CommandContext(ctx context.Context, command string, args ...string) exec.Cmd {
    return &nodeCmd{
        binaryName: n.BinaryName,
        nameOrID:   n.Name,
        command:    command,
        args:       args,
        ctx:        ctx,
    }
}

func (n *Node) SerialLogs(w io.Writer) error {
    return exec.Command(n.BinaryName, "logs", n.Name).SetStdout(w).SetStderr(w).Run()
}
```

### common/provision.go — shared provision helpers

```go
// Source: docker/provision.go and nerdctl/provision.go (identical bodies)
package common

import (
    "fmt"
    "net"
    "strings"

    "sigs.k8s.io/kind/pkg/errors"
    "sigs.k8s.io/kind/pkg/exec"
    "sigs.k8s.io/kind/pkg/internal/apis/config"
)

// GenerateMountBindings converts the mount list to --volume args.
// Format: '<HostPath>:<ContainerPath>[:options]'
func GenerateMountBindings(mounts ...config.Mount) []string {
    args := make([]string, 0, len(mounts))
    for _, m := range mounts {
        bind := fmt.Sprintf("%s:%s", m.HostPath, m.ContainerPath)
        var attrs []string
        if m.Readonly {
            attrs = append(attrs, "ro")
        }
        if m.SelinuxRelabel {
            attrs = append(attrs, "Z")
        }
        switch m.Propagation {
        case config.MountPropagationNone:
        case config.MountPropagationBidirectional:
            attrs = append(attrs, "rshared")
        case config.MountPropagationHostToContainer:
            attrs = append(attrs, "rslave")
        }
        if len(attrs) > 0 {
            bind = fmt.Sprintf("%s:%s", bind, strings.Join(attrs, ","))
        }
        args = append(args, fmt.Sprintf("--volume=%s", bind))
    }
    return args
}

// GeneratePortMappings converts portMappings to --publish args.
// Protocol is preserved as-is (docker/nerdctl use uppercase TCP/UDP/SCTP).
// Podman callers use their own generatePortMappings in podman/provision.go.
func GeneratePortMappings(clusterIPFamily config.ClusterIPFamily, portMappings ...config.PortMapping) ([]string, error) {
    args := make([]string, 0, len(portMappings))
    for _, pm := range portMappings {
        if pm.ListenAddress == "" {
            switch clusterIPFamily {
            case config.IPv4Family, config.DualStackFamily:
                pm.ListenAddress = "0.0.0.0"
            case config.IPv6Family:
                pm.ListenAddress = "::"
            default:
                return nil, errors.Errorf("unknown cluster IP family: %v", clusterIPFamily)
            }
        }
        if string(pm.Protocol) == "" {
            pm.Protocol = config.PortMappingProtocolTCP
        }
        switch pm.Protocol {
        case config.PortMappingProtocolTCP:
        case config.PortMappingProtocolUDP:
        case config.PortMappingProtocolSCTP:
        default:
            return nil, errors.Errorf("unknown port mapping protocol: %v", pm.Protocol)
        }
        hostPort, releaseHostPortFn, err := PortOrGetFreePort(pm.HostPort, pm.ListenAddress)
        if err != nil {
            return nil, errors.Wrap(err, "failed to get random host port for port mapping")
        }
        protocol := string(pm.Protocol)
        hostPortBinding := net.JoinHostPort(pm.ListenAddress, fmt.Sprintf("%d", hostPort))
        args = append(args, fmt.Sprintf("--publish=%s:%d/%s", hostPortBinding, pm.ContainerPort, protocol))
        if releaseHostPortFn != nil {
            releaseHostPortFn()
        }
    }
    return args, nil
}

// CreateContainer runs "binaryName run --name name args..."
func CreateContainer(binaryName, name string, args []string) error {
    return exec.Command(binaryName, append([]string{"run", "--name", name}, args...)...).Run()
}
```

### go.mod update

```
// Before
go 1.17

// After
go 1.21.0

toolchain go1.26.0
```

### Provider factory update (docker example)

```go
// docker/provider.go — before
func (p *provider) node(name string) nodes.Node {
    return &node{
        name: name,
    }
}

// docker/provider.go — after
func (p *provider) node(name string) nodes.Node {
    return &common.Node{
        Name:       name,
        BinaryName: "docker",
    }
}
```

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|------------------|--------|
| `go 1.17` in go.mod | `go 1.21.0` + `toolchain go1.26.0` | Enables toolchain directive, documents actual minimum |
| Per-provider `node`/`nodeCmd` structs (3 copies) | Single `common.Node` + `common.nodeCmd` | Eliminates ~500 lines of duplication |
| Per-provider `generateMountBindings` (3 identical copies) | Single `common.GenerateMountBindings` | One place to fix bugs |
| Per-provider `generatePortMappings` (docker+nerdctl identical; podman diverges) | `common.GeneratePortMappings` (docker/nerdctl); podman keeps its own | Preserves behavioral difference, reduces duplication where safe |

**Notes on existing divergences NOT targeted by this phase:**
- docker: `createContainer` in provider.go is a separate function; nerdctl: same; podman: same — all deleted/moved except podman
- docker/nerdctl already have `--init=false` and `--restart=on-failure:1` in `commonArgs` but those are in `provision.go` caller code, not the functions being extracted
- podman has `createAnonymousVolume` and `--volume fmt.Sprintf("%s:/var:suid,exec,dev", varVolume)` — unique to podman, stays in podman/provision.go

## Open Questions

1. **Should `clusterLabelKey` and `nodeRoleLabelKey` both move to common?**
   - What we know: `nodeRoleLabelKey` is needed by `common.Node.Role()`. `clusterLabelKey` is used in `provider.go` (ListClusters, ListNodes) which stays per-provider.
   - What's unclear: Whether to move just `nodeRoleLabelKey` (which is needed by common) or both.
   - Recommendation: Move only `nodeRoleLabelKey` to `common/constants.go` as `NodeRoleLabelKey` (exported). Keep `clusterLabelKey` as package-private in each provider's constants.go since those files are not being deleted.

2. **What happens to docker/provision_test.go and nerdctl/provision_test.go?**
   - What we know: Tests call `generateMountBindings`, `generatePortMappings` which are package-private. After deletion of provision.go, these tests won't compile.
   - What's unclear: Whether to migrate tests to `common/provision_test.go` (using exported names) or convert docker/nerdctl provision_test.go to use the exported common functions.
   - Recommendation: Move the common tests (generateMountBindings, generatePortMappings, generatePortMappings-empty) to `common/provision_test.go`. The docker-specific `Test_parseSubnetOutput` can stay in `docker/network_test.go` or `docker/provision_test.go` (if provision_test.go is kept as a stub), since it tests docker network parsing logic.

3. **Does `nodeCmd` also need to be exported from common?**
   - What we know: `nodeCmd` is an implementation detail of `Node.Command()` and `Node.CommandContext()`. It's returned as `exec.Cmd` interface, so callers never see the concrete type.
   - Recommendation: Keep `nodeCmd` unexported (lowercase) in `common/node.go`. Only `Node` needs to be exported.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` package |
| Config file | none — standard `go test` |
| Quick run command | `go test ./pkg/cluster/internal/providers/...` |
| Full suite command | `go test ./...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PROV-01 | Node struct uses BinaryName, all methods route through it | unit | `go test ./pkg/cluster/internal/providers/common/...` | ❌ Wave 0 — new file |
| PROV-02 | GenerateMountBindings produces correct --volume args | unit | `go test ./pkg/cluster/internal/providers/common/...` | ❌ Wave 0 — migrate from docker+nerdctl |
| PROV-02 | GeneratePortMappings static port produces correct --publish arg | unit | `go test ./pkg/cluster/internal/providers/common/...` | ❌ Wave 0 — migrate from docker+nerdctl |
| PROV-02 | GeneratePortMappings port=0 acquires random port and releases listener | unit | `go test ./pkg/cluster/internal/providers/common/...` | ❌ Wave 0 — migrate from docker+nerdctl |
| PROV-02 | podman generatePortMappings still lowercases protocol | unit | `go test ./pkg/cluster/internal/providers/podman/...` | ✅ exists (podman/provision_test.go) |
| PROV-03 | go.mod has go 1.21.0 and toolchain go1.26.0 | build | `go build ./...` | ✅ file exists, needs edit |
| ALL | All providers compile and tests pass unchanged | build+test | `go build ./... && go test ./...` | ✅ baseline established |

### Sampling Rate

- **Per task commit:** `go test ./pkg/cluster/internal/providers/...`
- **Per wave merge:** `go build ./... && go test ./...`
- **Phase gate:** Full `go build ./... && go test ./...` green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `pkg/cluster/internal/providers/common/node.go` — covers PROV-01 (new file, not a test file)
- [ ] `pkg/cluster/internal/providers/common/provision.go` — covers PROV-02 (new file)
- [ ] `pkg/cluster/internal/providers/common/provision_test.go` — migrate GenerateMountBindings + GeneratePortMappings tests from docker/nerdctl provision_test.go
- [ ] Framework install: none needed — `go test` is built into the toolchain

## Sources

### Primary (HIGH confidence)

- Direct codebase inspection of all 6 files (`docker/node.go`, `nerdctl/node.go`, `podman/node.go`, `docker/provision.go`, `nerdctl/provision.go`, `podman/provision.go`) — byte-for-byte comparison performed
- `go build ./...` and `go test ./pkg/cluster/internal/providers/...` executed in repo, both pass
- `go version` confirms go1.26.0 is the running toolchain
- `.go-version` file: `1.25.7` (CI pin; do not change)
- `go.mod` current state: `go 1.17`, no `toolchain` directive
- Go official documentation on `toolchain` directive: introduced in Go 1.21 — https://go.dev/doc/toolchain

### Secondary (MEDIUM confidence)

- Go 1.21 release notes confirming `toolchain` directive syntax: https://go.dev/blog/toolchain

### Tertiary (LOW confidence)

- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all code examined directly, no external dependencies
- Architecture patterns: HIGH — based on direct code analysis; nerdctl already implements the binaryName pattern
- Pitfalls: HIGH — identified by static diff of the three provision.go files; podman divergence is concrete and verifiable

**Research date:** 2026-03-03
**Valid until:** 2026-04-03 (stable Go stdlib refactoring, 30-day window)
