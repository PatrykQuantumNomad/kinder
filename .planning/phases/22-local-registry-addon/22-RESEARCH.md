# Phase 22: Local Registry Addon - Research

**Researched:** 2026-03-03
**Domain:** Local container registry integration — Docker/Podman/nerdctl network bridge, containerd certs.d config, kubectl ConfigMap apply inside kind nodes
**Confidence:** HIGH for Docker/nerdctl; MEDIUM for Podman rootless (documented issues flagged)

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| REG-01 | Create registry:2 container on kind network during cluster creation | `exec.Command(binaryName, "run", ...)` + `exec.Command(binaryName, "network", "connect", "kind", "kind-registry")` — verified against kind upstream script and codebase patterns |
| REG-02 | Patch containerd certs.d config on all nodes for localhost:5001 | `node.Command("mkdir", "-p", ...)` + `node.Command("tee", ...)` per node, containerdConfigPatches in cluster YAML — verified against official kind local-registry doc |
| REG-03 | Apply kube-public/local-registry-hosting ConfigMap for dev tool discovery | Standard embedded YAML + `kubectl apply -f -` via controlPlane node — matches MetalLB/dashboard addon pattern |
| REG-04 | Addon disableable via addons.localRegistry: false in cluster config | `LocalRegistry bool` already wired in internal config from Phase 21; `runAddon("Local Registry", cfg.Addons.LocalRegistry, ...)` in create.go |
</phase_requirements>

## Summary

Phase 22 implements a local container registry addon following the established kinder addon pattern. The core mechanism is well-documented by the official kind project: run a `registry:2` container on the `kind` Docker/nerdctl/podman network, write a `hosts.toml` file into each node's containerd `certs.d` directory, and apply a `kube-public/local-registry-hosting` ConfigMap so tools like Tilt and Skaffold can auto-discover it. The config types and `runAddon` wiring were completed in Phase 21 — Phase 22 only needs the action package implementation.

The implementation has two distinct sub-problems. The first is starting and networking the registry container: unlike all other addon actions (which only interact with the already-created cluster nodes via `node.Command`), the local registry action must also run a host-side container command using the active provider binary (docker/nerdctl/podman). The codebase already solves this via the `fmt.Stringer` type assertion pattern used in the MetalLB action — `ctx.Provider.(fmt.Stringer).String()` returns "docker", "nerdctl", or "podman". The second sub-problem is patching containerd inside each node: the `hosts.toml` file must be written to `/etc/containerd/certs.d/localhost:5001/hosts.toml` on every node (not just control-plane), pointing at `http://kind-registry:5000`. Containerd v2 (used in recent kind node images) picks up `certs.d` config without a daemon restart when `config_path` is set — but the `config_path` itself must be enabled via a `ContainerdConfigPatches` entry in the cluster YAML, which is a user-facing change that kinder must apply automatically at cluster creation time.

The primary blocker noted in STATE.md — verifying that `--network kind` and container-name DNS resolution work in Podman rootless — is a real risk. GitHub issue #3468 in kubernetes-sigs/kind confirms that the documented local registry setup fails on rootless Podman with TLS enforcement errors. The recommended approach is: implement for Docker/nerdctl first (where this is fully verified), detect Podman rootless via `ctx.Provider.Info().Rootless`, and emit a warning explaining the manual host-side configuration needed.

**Primary recommendation:** Implement the action as a new `pkg/cluster/internal/create/actions/installlocalregistry/` package. Use `exec.Command(binaryName, "run", ...)` for registry container creation, `exec.Command(binaryName, "network", "connect", "kind", "kind-registry")` for network attachment, per-node `node.Command("mkdir"/"tee")` for `hosts.toml` injection, and an embedded YAML ConfigMap applied via `kubectl apply -f -`. Register in `create.go` before MetalLB (the registry must be available before MetalLB potentially creates pods that pull images).

## Standard Stack

### Core

| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| `registry:2` | v2.8.x series | Local container registry image | Official kind ecosystem standard; v3 deprecated storage drivers; zero config; 25MB |
| Go `exec.Command` | stdlib | Run registry container via provider binary | Established pattern throughout codebase; no SDK needed |
| `node.Command` | existing pkg | Write files to nodes, apply ConfigMap | All existing addons use this; runs `docker/nerdctl/podman exec --privileged` |
| `//go:embed` | Go stdlib | Embed ConfigMap YAML | Same as metallb, dashboard, envoygw addons |

### Supporting

| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| `fmt.Stringer` type assertion | — | Detect active provider binary name | MetalLB already uses this; `ctx.Provider.(fmt.Stringer).String()` |
| `ctx.Provider.Info()` | — | Detect Podman rootless for warnings | Same Info() call used in MetalLB for rootless warning |
| `strings.NewReader` | stdlib | Pass YAML as stdin to kubectl | Same as every other addon manifest apply |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `registry:2` | `registry:3` (v3.0.0) | registry:3 deprecated storage drivers (released April 2025); kind ecosystem has not migrated; use v2 |
| `--network kind` at run time | `--network connect` post-run | Official kind docs use `docker run --network bridge` then `docker network connect kind` — either works; run-time `--network kind` is cleaner for kinder |
| `hosts.toml` per node | `ContainerdConfigPatches` in cluster config | certs.d per-node write is imperative and survives cluster restart; ConfigPatches approach requires cluster config change at creation time — both needed; see Architecture section |

**Installation:** No new Go dependencies. No `go get` required.

## Architecture Patterns

### Recommended Project Structure

```
pkg/cluster/internal/create/actions/installlocalregistry/
├── localregistry.go     # Action implementation
└── manifests/
    └── local-registry-hosting.yaml  # kube-public ConfigMap
```

### Pattern 1: Registry Container Creation and Network Attachment

**What:** Start the `registry:2` container on the host using the active provider binary, then connect it to the `kind` network so all cluster nodes can reach it by name.

**When to use:** REG-01 — executed once during cluster creation, before node containerd config is patched.

**Key design:** The registry container must be idempotent — if `kind-registry` already exists (from a previous cluster), skip creation. Check with `exec.Command(binaryName, "inspect", "kind-registry")` and ignore if exit code is 0.

```go
// Source: adapted from installmetallb/metallb.go + kind upstream script
// https://kind.sigs.k8s.io/docs/user/local-registry/

// Detect provider binary name (same pattern as MetalLB)
binaryName := "docker" // default fallback
if s, ok := ctx.Provider.(fmt.Stringer); ok {
    binaryName = s.String()
}

// Check if registry container already exists (idempotency)
alreadyExists := exec.Command(binaryName, "inspect", registryName).Run() == nil
if !alreadyExists {
    // Create registry container on bridge first, then connect to kind network
    if err := exec.Command(binaryName,
        "run", "-d", "--restart=always",
        "--name", registryName,
        "-p", "127.0.0.1:5001:5000",
        "registry:2",
    ).Run(); err != nil {
        return errors.Wrap(err, "failed to create registry container")
    }
}

// Connect registry to kind network (idempotent — network connect errors if already connected)
networks, _ := exec.Output(exec.Command(binaryName, "inspect", registryName,
    "--format", "{{range .NetworkSettings.Networks}}{{.NetworkID}} {{end}}"))
// Connect only if not already on kind network
if !strings.Contains(string(networks), "kind") {
    if err := exec.Command(binaryName, "network", "connect", "kind", registryName).Run(); err != nil {
        return errors.Wrap(err, "failed to connect registry to kind network")
    }
}
```

### Pattern 2: Per-Node containerd hosts.toml Injection

**What:** Write `/etc/containerd/certs.d/localhost:5001/hosts.toml` into every cluster node (not just control-plane). This file instructs containerd to proxy `localhost:5001` pull requests to `http://kind-registry:5000` on the kind network.

**When to use:** REG-02 — applied to ALL nodes (control-plane and workers), executed after registry container is running.

**Critical note:** containerd v2 (used in kind node images from kind v0.26+) supports hot-reload of `certs.d` directory changes WITHOUT restarting containerd — PROVIDED the `config_path` directive is already set. This means the `config_path` patch must be applied at cluster creation time via `ContainerdConfigPatches`. Kinder must inject this patch before provisioning (or detect and warn).

```go
// Source: kind upstream pattern — https://kind.sigs.k8s.io/docs/user/local-registry/
const registryDir = "/etc/containerd/certs.d/localhost:5001"
const hostsTOML = `[host."http://kind-registry:5000"]
`

for _, node := range allNodes {
    if err := node.Command("mkdir", "-p", registryDir).Run(); err != nil {
        return errors.Wrapf(err, "failed to create registry dir on node %s", node)
    }
    if err := node.Command(
        "sh", "-c", fmt.Sprintf("cat > %s/hosts.toml", registryDir),
    ).SetStdin(strings.NewReader(hostsTOML)).Run(); err != nil {
        return errors.Wrapf(err, "failed to write hosts.toml on node %s", node)
    }
}
```

**Alternative approach for writing the file:** Use `tee` instead of `sh -c "cat >"` for clarity and to avoid shell quoting issues:

```go
node.Command("tee", registryDir+"/hosts.toml").SetStdin(strings.NewReader(hostsTOML)).Run()
```

`tee` is simpler and avoids shell invocation inside the node container.

### Pattern 3: kube-public/local-registry-hosting ConfigMap

**What:** Apply a ConfigMap to `kube-public` namespace that tells dev tools (Tilt, Skaffold, ctlptl) about the local registry endpoint. This is a community standard from [KEP-1755](https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry).

**When to use:** REG-03 — applied once via control-plane node's kubectl, after registry is running.

```go
//go:embed manifests/local-registry-hosting.yaml
var localRegistryHostingManifest string

// Apply via control-plane node (same as MetalLB, Dashboard patterns)
if err := controlPlane.Command(
    "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "-f", "-",
).SetStdin(strings.NewReader(localRegistryHostingManifest)).Run(); err != nil {
    return errors.Wrap(err, "failed to apply local-registry-hosting ConfigMap")
}
```

The embedded YAML:

```yaml
# manifests/local-registry-hosting.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:5001"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
```

### Pattern 4: ContainerdConfigPatches Injection at Cluster Creation

**What:** The `config_path` directive for `certs.d` must be set in each node's `containerd/config.toml` BEFORE the cluster is created — it cannot be injected post-creation by the action. The kind upstream script does this via the cluster YAML `containerdConfigPatches` field.

**When to use:** This is a cluster-creation-time concern, not an addon-action concern. Two options:

**Option A (preferred):** Kinder injects the `containerdConfigPatches` entry automatically when `LocalRegistry: true`. This requires modifying the cluster config struct or the Provision flow to add the patch. The v1alpha4 `ContainerdConfigPatches` field already exists in the Cluster struct.

**Option B:** Document that users must add the patch to their cluster config YAML manually, and let the action detect whether `config_path` is already set before writing `hosts.toml`.

Given kinder's batteries-included philosophy, Option A is the correct choice. The action (or the pre-provisioning step) should inject:

```toml
[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "/etc/containerd/certs.d"
```

**Implementation location:** Inject in `create.go` before `p.Provision()`, if `cfg.Addons.LocalRegistry` is true — append to `cfg.ContainerdConfigPatches`. This ensures the node images are created with `config_path` set.

**CRITICAL:** If this approach is used, the `ContainerdConfigPatches` injection must happen BEFORE `p.Provision(status, opts.Config)` is called in `create.go`, not in the addon action itself. The action runs post-provision.

### Pattern 5: create.go Wiring (REG-04)

**What:** Register the local registry action in `create.go` via the `runAddon` helper, using `cfg.Addons.LocalRegistry`.

**Ordering:** Registry must run BEFORE MetalLB (or at minimum before any addon that deploys pods that might need to pull from the registry). Current ordering in `create.go`:

```go
// BEFORE MetalLB, immediately after other addons:
runAddon("Local Registry", cfg.Addons.LocalRegistry, installlocalregistry.NewAction())
runAddon("MetalLB", cfg.Addons.MetalLB, installmetallb.NewAction())
// ... rest of addons
```

### Anti-Patterns to Avoid

- **Running the registry as a kind node container:** The registry should be a standalone `registry:2` container on the kind network, not a pod inside the cluster. Pod-based registries require Kubernetes to be fully running before the registry is available, creating a chicken-and-egg problem.
- **Using `localhost` instead of `kind-registry` in hosts.toml:** Inside node containers, `localhost` refers to the container's loopback, not the host. The hosts.toml must reference the container name `kind-registry`.
- **Restarting containerd after writing hosts.toml:** With containerd v2 and `config_path` set, certs.d changes are picked up without restart. Restarting containerd is disruptive (kills running pods) and not needed.
- **Only patching the control-plane node:** ALL nodes (workers included) need the `hosts.toml` file — each node has its own containerd that pulls images independently.
- **Assuming the registry is new on every run:** The registry container survives cluster deletion (it's not labeled as a kind cluster node). Check for existence and skip creation if already present.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Registry container management | Custom Go registry server | `registry:2` container image | 25MB, zero-config, Docker Hub official, kind ecosystem standard |
| ConfigMap parsing/validation | Custom YAML parser | Pre-written embedded YAML + `kubectl apply -f -` | Same pattern works for all other addons; kubectl handles validation |
| Provider binary detection | Per-provider switch statement | `ctx.Provider.(fmt.Stringer).String()` | Established pattern in installmetallb; correctly handles docker/nerdctl/podman |

**Key insight:** The entire registry setup is a sequential shell script embedded in Go. There is no complex logic — it is idempotent container creation + file writing + kubectl apply. Any hand-rolled alternative would be more complex and less reliable than this minimal approach.

## Common Pitfalls

### Pitfall 1: localhost Networking Boundary

**What goes wrong:** The registry starts on `localhost:5001` on the host. Inside kind node containers, `localhost` is the container's own loopback — not the host. Any attempt to pull from `localhost:5001` inside a pod will fail with "connection refused."

**Why it happens:** Docker network namespacing — each container has its own network namespace. The kind nodes are containers; their `localhost` is not the host's `localhost`.

**How to avoid:** Place the registry on the `kind` Docker network. Reference it by container name `kind-registry` (at port 5000, the container port) inside `hosts.toml`. Reference it as `localhost:5001` from the host (where port 5001 is published). The containerd `hosts.toml` bridges these two namespaces: when a node pulls `localhost:5001/myimage`, containerd routes it to `http://kind-registry:5000/myimage`.

**Warning signs:** `curl http://localhost:5001/v2/` from the host works, but `kubectl run test --image=localhost:5001/myimage` fails with image pull errors.

### Pitfall 2: containerd config_path Not Set — hosts.toml Silently Ignored

**What goes wrong:** The `hosts.toml` file is written to `/etc/containerd/certs.d/localhost:5001/hosts.toml` inside each node, but containerd ignores it because the `config_path` directive is not set in `/etc/containerd/config.toml`.

**Why it happens:** containerd's registry host configuration (`certs.d` directory) is an opt-in feature. Without `config_path = "/etc/containerd/certs.d"` in containerd's config, the directory is never consulted.

**How to avoid:** Inject the `ContainerdConfigPatches` TOML snippet at cluster creation time (before `p.Provision()`), when `cfg.Addons.LocalRegistry == true`. This is the only way to set containerd config before nodes start.

**Warning signs:** Nodes start, hosts.toml file exists (verified with `docker exec <node> cat /etc/containerd/certs.d/localhost:5001/hosts.toml`), but image pulls from `localhost:5001` still fail. Check containerd config: `docker exec <node> cat /etc/containerd/config.toml | grep config_path`.

### Pitfall 3: Podman Rootless TLS Enforcement

**What goes wrong:** On Podman rootless, pushing to `localhost:5001` (an HTTP registry) fails with "server gave HTTP response to HTTPS client." Podman enforces HTTPS by default for all non-localhost registries, and even for localhost may require explicit `insecure = true` in `registries.conf`.

**Why it happens:** Podman has different default TLS policies than Docker. Docker treats `localhost` as inherently insecure; Podman does not.

**How to avoid:** Detect Podman rootless via `ctx.Provider.Info().Rootless` and the provider name being "podman". Emit an explicit warning with the fix instructions:

```
Registry is running but host-side Podman requires manual insecure registry configuration.
Add to /etc/containers/registries.conf:
[[registry]]
  location = "localhost:5001"
  insecure = true
Then: podman machine stop && podman machine start (if using podman machine)
```

Do NOT attempt to automate the Podman host configuration — it requires root access or machine restarts.

**Warning signs:** `docker push localhost:5001/myimage:latest` succeeds from Docker host but `podman push localhost:5001/myimage:latest` fails with TLS error.

### Pitfall 4: Registry Container Not Labeled as Kind Resource

**What goes wrong:** The `kind-registry` container is not labeled with `io.x-k8s.kind.cluster`, so `kinder delete cluster` does not remove it. The registry persists after cluster deletion and may conflict with the next cluster creation.

**Why it happens:** The registry is a separate container from the cluster nodes. The existing `DeleteNodes` provider method only deletes containers with the cluster label.

**How to avoid:** This is intentional and desirable — the registry should persist across cluster recreations so cached images survive cluster restart. The action's idempotency check handles the "already exists" case on re-create. Document this behavior in the status output. If users want to delete the registry, they must run `docker rm -f kind-registry` manually.

**Warning signs:** Confusing "container name already in use" errors when the action is not idempotent. Resolve by checking existence before creation.

### Pitfall 5: Worker Nodes Missing hosts.toml

**What goes wrong:** The action only patches the control-plane node, assuming that is sufficient. When a pod is scheduled to a worker node, the worker's containerd does not know about the registry, and image pulls fail.

**Why it happens:** Each kind node is an independent container with its own containerd process. The `hosts.toml` file is local to each container's filesystem.

**How to avoid:** Iterate over `ctx.Nodes()` (ALL nodes) — not just `nodeutils.ControlPlaneNodes()`. Apply `mkdir` + `tee hosts.toml` to every node in the cluster.

**Warning signs:** Image pulls succeed for pods on control-plane nodes but fail for pods scheduled to workers.

## Code Examples

Verified patterns from official sources:

### Complete Action Skeleton

```go
// Source: adapted from installmetallb/metallb.go (codebase) and
// https://kind.sigs.k8s.io/docs/user/local-registry/ (official kind docs)

package installlocalregistry

import (
    _ "embed"
    "fmt"
    "strings"

    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/nodeutils"
    "sigs.k8s.io/kind/pkg/errors"
    "sigs.k8s.io/kind/pkg/exec"
)

//go:embed manifests/local-registry-hosting.yaml
var localRegistryHostingManifest string

const (
    registryName    = "kind-registry"
    registryPort    = "5001"   // host port (published to host)
    registryIntPort = "5000"   // internal port inside registry container
    registryImage   = "registry:2"
    registryDir     = "/etc/containerd/certs.d/localhost:5001"
)

const hostsTOML = `[host."http://kind-registry:5000"]
`

type action struct{}

func NewAction() actions.Action {
    return &action{}
}

func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing Local Registry")
    defer ctx.Status.End(false)

    // Detect provider binary name (docker, nerdctl, or podman)
    binaryName := "docker"
    if s, ok := ctx.Provider.(fmt.Stringer); ok {
        binaryName = s.String()
    }

    // Warn on rootless Podman (host-side push requires manual config)
    info, err := ctx.Provider.Info()
    if err != nil {
        return errors.Wrap(err, "failed to get provider info")
    }
    if info.Rootless && binaryName == "podman" {
        ctx.Logger.Warn("Rootless Podman: pushing to localhost:5001 requires manual insecure registry config in /etc/containers/registries.conf")
    }

    // Step 1: Create registry container (idempotent)
    if exec.Command(binaryName, "inspect", "--format={{.ID}}", registryName).Run() != nil {
        if err := exec.Command(binaryName,
            "run", "-d", "--restart=always",
            "--name", registryName,
            "-p", "127.0.0.1:"+registryPort+":"+registryIntPort,
            registryImage,
        ).Run(); err != nil {
            return errors.Wrap(err, "failed to create registry container")
        }
    }

    // Step 2: Connect registry to kind network (idempotent check via inspect)
    if err := exec.Command(binaryName, "network", "connect", "kind", registryName).Run(); err != nil {
        // Ignore error — may already be connected; verify by continuing
        _ = err
    }

    // Step 3: Patch containerd certs.d on ALL nodes
    allNodes, err := ctx.Nodes()
    if err != nil {
        return errors.Wrap(err, "failed to list cluster nodes")
    }
    for _, node := range allNodes {
        if err := node.Command("mkdir", "-p", registryDir).Run(); err != nil {
            return errors.Wrapf(err, "failed to create certs.d dir on node %s", node)
        }
        if err := node.Command("tee", registryDir+"/hosts.toml").
            SetStdin(strings.NewReader(hostsTOML)).Run(); err != nil {
            return errors.Wrapf(err, "failed to write hosts.toml on node %s", node)
        }
    }

    // Step 4: Apply kube-public/local-registry-hosting ConfigMap
    controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)
    if err != nil || len(controlPlanes) == 0 {
        return errors.New("failed to find control plane nodes")
    }
    if err := controlPlanes[0].Command(
        "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(localRegistryHostingManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply local-registry-hosting ConfigMap")
    }

    ctx.Status.End(true)
    ctx.Logger.V(0).Info("")
    ctx.Logger.V(0).Info("Local Registry:")
    ctx.Logger.V(0).Info("  Push: docker push localhost:5001/myimage:latest")
    ctx.Logger.V(0).Info("  Pull in pods: image: localhost:5001/myimage:latest")
    ctx.Logger.V(0).Info("")

    return nil
}
```

### local-registry-hosting.yaml

```yaml
# Source: https://kind.sigs.k8s.io/docs/user/local-registry/
# KEP-1755: https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:5001"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
```

### ContainerdConfigPatches injection (in create.go, before Provision)

```go
// Source: kind upstream script — https://kind.sigs.k8s.io/docs/user/local-registry/
// Inject before p.Provision(status, opts.Config)
if opts.Config.Addons.LocalRegistry {
    const registryConfigPatch = `[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "/etc/containerd/certs.d"
`
    opts.Config.ContainerdConfigPatches = append(
        opts.Config.ContainerdConfigPatches,
        registryConfigPatch,
    )
}
```

### Idempotency: Network Connect Check

```go
// Source: pattern from kind upstream + codebase exec patterns
// More robust than ignoring the connect error:
var networksOut strings.Builder
_ = exec.Command(binaryName, "inspect",
    "--format", "{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}",
    registryName,
).SetStdout(&networksOut).Run()
if !strings.Contains(networksOut.String(), "kind") {
    if err := exec.Command(binaryName, "network", "connect", "kind", registryName).Run(); err != nil {
        return errors.Wrap(err, "failed to connect registry to kind network")
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Shell script in kind docs (external to cluster tool) | Embedded in cluster creation action (batteries included) | kinder v1.3 | Users get working registry without manual setup |
| `containerd config.toml` patching | `certs.d` directory-based config (containerd v1.5+) | containerd v1.5 / kind v0.20+ | No daemon restart needed; hot-reloads on file change |
| `localhost:5000` (original kind default) | `localhost:5001` (kind current default) | kind v0.11+ | Avoids conflict with macOS AirPlay Receiver on port 5000 |
| `registry:2` single tag | `registry:2` still standard | — | v3 released April 2025 but not yet adopted by kind ecosystem |

**Deprecated/outdated:**
- `containerd config.toml` patching for registry mirrors: replaced by `certs.d` directory approach; requires containerd restart and is fragile
- Port 5000: conflicts with macOS AirPlay Receiver; use 5001

## Open Questions

1. **ContainerdConfigPatches injection: create.go vs action**
   - What we know: `config_path` must be set before `p.Provision()` is called; the addon action runs after provisioning; the `ContainerdConfigPatches` field in `config.Cluster` is consumed during provisioning
   - What's unclear: The cleanest code location — inject in `create.go` before `p.Provision()`, or add a pre-provisioning hook concept
   - Recommendation: Inject directly in `create.go` before `p.Provision()` (simple, no new abstraction needed). If `opts.Config.Addons.LocalRegistry`, append the TOML patch to `opts.Config.ContainerdConfigPatches`. This is 3 lines of Go and requires no new mechanism.

2. **Network connect idempotency: error ignore vs pre-check**
   - What we know: `docker network connect kind kind-registry` errors if the container is already connected
   - What's unclear: Whether to check first (2 exec calls) or try-and-ignore-specific-error (1 exec call)
   - Recommendation: Check first with `docker inspect --format={{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}`. Ignoring specific error messages from docker/podman/nerdctl is fragile across provider versions.

3. **Podman rootless: full support vs warn-and-continue**
   - What we know: Kind issue #3468 documents TLS enforcement failures on rootless Podman for push; container-to-container DNS within the `kind` network may work; the node-side containerd `hosts.toml` approach should work for image pulls within the cluster
   - What's unclear: Whether `--network kind` connectivity (node → registry) works in rootless Podman with netavark/aardvark-dns
   - Recommendation: Implement for Docker/nerdctl. For Podman, implement and warn. Test by verifying `exec.Command("podman", "exec", <node>, "curl", "http://kind-registry:5000/v2/")` succeeds after setup. If not, escalate to a clear warning.

4. **Registry lifecycle: survive cluster deletion?**
   - What we know: The `kind-registry` container is not labeled with the kind cluster label; `kinder delete cluster` will not remove it; this is the kind upstream behavior
   - What's unclear: Whether users expect the registry to be deleted with the cluster
   - Recommendation: Keep it persistent across cluster deletion (consistent with kind upstream). Document this behavior in the action output. For users who want a clean delete, document `docker rm -f kind-registry`.

## Validation Architecture

No `nyquist_validation` config found — skipping structured test map.

However, the following unit-testable functions can be extracted from the action for standalone testing:

- `isContainerRunning(binaryName, name string) bool` — testable with a mock exec
- `isOnNetwork(binaryName, containerName, networkName string) bool` — testable with mock exec
- The ConfigMap YAML can be linted independently

Integration test (manual) after implementation:
```bash
kinder create cluster
docker push localhost:5001/alpine:latest
kubectl run test --image=localhost:5001/alpine:latest --restart=Never -- sleep 3600
kubectl get pod test  # should be Running
kubectl get configmap local-registry-hosting -n kube-public  # should exist
```

## Sources

### Primary (HIGH confidence)
- `https://kind.sigs.k8s.io/docs/user/local-registry/` — official kind local registry guide; registry container creation, network connect, certs.d pattern, hosts.toml format, local-registry-hosting ConfigMap
- `https://raw.githubusercontent.com/kubernetes-sigs/kind/main/site/static/examples/kind-with-registry.sh` — canonical upstream shell script; exact docker commands and ContainerdConfigPatches YAML
- Direct codebase read: `pkg/cluster/internal/create/actions/installmetallb/metallb.go` — `fmt.Stringer` provider detection pattern, rootless warning, `exec.Command` pattern for host-side commands
- Direct codebase read: `pkg/cluster/internal/create/create.go` — `runAddon` pattern, addon ordering, `p.Provision()` location for `ContainerdConfigPatches` injection
- Direct codebase read: `pkg/cluster/internal/providers/common/node.go` — `node.Command("tee", ...)` and `node.Command("mkdir", ...)` patterns
- Direct codebase read: `pkg/cluster/internal/providers/{docker,nerdctl,podman}/network.go` — `fixedNetworkName = "kind"` confirmed in all three providers
- Direct codebase read: `pkg/internal/apis/config/types.go` — `ContainerdConfigPatches []string` field confirmed on Cluster struct
- Direct codebase read: `pkg/internal/apis/config/convert_v1alpha4.go` — `LocalRegistry: boolVal(in.Addons.LocalRegistry)` already wired from Phase 21

### Secondary (MEDIUM confidence)
- `https://github.com/containerd/containerd/blob/main/docs/hosts.md` — containerd hosts.toml format; `[host."http://..."]` syntax for HTTP mirrors; no restart needed for certs.d changes in containerd v2
- `https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry` — KEP-1755 defining `localRegistryHosting.v1` ConfigMap format
- Existing project research: `.planning/research/SUMMARY.md`, `.planning/research/PITFALLS.md`, `.planning/research/ARCHITECTURE.md` — prior milestones research flagged all major pitfalls and the implementation pattern

### Tertiary (LOW confidence — flagged)
- `https://github.com/kubernetes-sigs/kind/issues/3468` — Podman rootless TLS enforcement issue; confirms push failure but does not confirm node-side pull failure (LOW — single issue report, no definitive resolution)
- WebSearch results on Podman rootless DNS — multiple issues open in 2024-2025, DNS resolution within kind network under rootless Podman is uncertain; treat Podman rootless support as "best effort with warning" until verified

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — `registry:2`, `exec.Command`, `node.Command("tee")` all verified in codebase and kind upstream docs
- Architecture: HIGH — all implementation patterns are directly derived from existing addon actions and kind upstream script; no inference required
- Pitfalls: HIGH — localhost networking boundary, containerd config_path requirement, worker-node patching, and Podman TLS issue all verified against official sources and codebase review
- Podman rootless support: MEDIUM — known documented issues exist; implementation approach (warn-and-continue) is sound but actual behavior needs validation

**Research date:** 2026-03-03
**Valid until:** 2026-04-03 (registry:2 stable; containerd certs.d pattern established; low churn domain)
