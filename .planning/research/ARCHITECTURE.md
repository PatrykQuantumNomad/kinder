# Architecture Patterns: Addon Integration for Kinder

**Domain:** Kubernetes cluster tooling — addon integration into creation pipeline
**Researched:** 2026-03-01
**Overall confidence:** HIGH (codebase read directly; addon installation patterns verified against official docs)

---

## System Overview: Updated Pipeline Diagram

The existing kind action pipeline runs sequentially inside `Cluster()` in
`pkg/cluster/internal/create/create.go`. Each step satisfies the `Action`
interface (`Execute(*ActionContext) error`). Five new addon actions slot in
**after** `waitforready`, because addons require a fully ready API server and
scheduled node to apply manifests.

```
kinder create cluster
  │
  ▼
Provider.Provision()          ← unchanged; starts node containers
  │
  ├─ loadbalancer.NewAction()  ← unchanged; configures HAProxy for multi-CP
  ├─ configaction.NewAction()  ← unchanged; writes kubeadm config
  ├─ kubeadminit.NewAction()   ← unchanged; kubeadm init
  ├─ installcni.NewAction()    ← unchanged; installs kindnet (if not disabled)
  ├─ installstorage.NewAction()← unchanged; installs local-path StorageClass
  ├─ kubeadmjoin.NewAction()   ← unchanged; joins worker nodes
  ├─ waitforready.NewAction()  ← unchanged; waits for control-plane Ready
  │
  │   ── NEW ADDON ACTIONS (all opt-out via config.Addons) ──
  │
  ├─ installmetallb.NewAction()       ← Layer2 LB; needs docker network subnet
  ├─ installgatewayapi.NewAction()    ← Gateway API CRDs (prereq for Envoy GW)
  ├─ installenvoygw.NewAction()       ← Envoy Gateway controller
  ├─ installmetricsserver.NewAction() ← Metrics Server (+insecure-tls patch)
  ├─ installcorednstuning.NewAction() ← Patches CoreDNS ConfigMap
  └─ installdashboard.NewAction()     ← Kubernetes Dashboard + RBAC
```

**Why addons come after `waitforready`:** All addon installations use `kubectl
apply` or `kubectl patch` against the live API server via the node's
`/etc/kubernetes/admin.conf`. The API server and at least one node must be
`Ready` before any workload can be scheduled. Installing before `waitforready`
results in pods stuck in `Pending` and race conditions when checking rollout
status.

**Why Gateway API CRDs precede Envoy Gateway:** Envoy Gateway controller
startup validates that Gateway API CRDs exist in the cluster. If they are
absent, the controller enters a crash loop. A dedicated `installgatewayapi`
action makes this dependency explicit and separately opt-outable from the
controller.

---

## Component Responsibilities: New Actions

### `installmetallb` — `pkg/cluster/internal/create/actions/installmetallb/`

| Concern | Approach |
|---------|----------|
| Manifest source | Embed `metallb-native.yaml` at build time in `pkg/cluster/internal/create/actions/installmetallb/manifests/` or fetch from a pinned URL at runtime. Embedding is strongly preferred so cluster creation works offline. |
| IP pool range | Call `docker network inspect kind --format '{{(index .IPAM.Config 0).Subnet}}'` via `exec.Command` on the host (not inside a node), parse the CIDR, then derive a safe tail range (last /28 of the IPv4 space). This mirrors the documented kind+MetalLB pattern. |
| Pool config | After the controller is running, apply an `IPAddressPool` + `L2Advertisement` manifest constructed in-memory with the computed range. Both objects are `metallb.io/v1beta1` CRDs. |
| Wait | Poll `kubectl -n metallb-system rollout status deployment/controller` before returning. |
| Responsibility | This action owns: manifest apply, pool CRD creation, readiness wait. |

**Provider compatibility note:** Docker, Podman, and Nerdctl all expose a
`network inspect` subcommand. The network name is `kind` for Docker/Nerdctl
and may differ for Podman (typically `podman`). The action must resolve the
correct network name by reading the provider string from `ctx.Provider.String()`
and selecting the appropriate inspect command. Layer2 mode works inside any
bridge network, so the same approach applies across providers.

### `installgatewayapi` — `pkg/cluster/internal/create/actions/installgatewayapi/`

| Concern | Approach |
|---------|----------|
| Manifest source | Embed `standard-install.yaml` from the Gateway API release corresponding to the Envoy Gateway version pinned in kinder. Standard channel is sufficient (HTTPRoute, GRPCRoute, ReferenceGrant). |
| Apply method | `kubectl apply --server-side -f -` piped from embedded manifest via node's `admin.conf`. Server-side apply handles large CRD objects without annotation size limits. |
| Wait | None needed beyond apply completing; CRDs are cluster-scoped resources, not workloads. |

### `installenvoygw` — `pkg/cluster/internal/create/actions/installenvoygw/`

| Concern | Approach |
|---------|----------|
| Manifest source | Embed `install.yaml` from the Envoy Gateway GitHub release (e.g., `github.com/envoyproxy/gateway/releases/download/v1.2.x/install.yaml`). This file bundles the controller Deployment plus Envoy Gateway CRDs. |
| Apply method | `kubectl apply --server-side -f -` (Envoy GW manifests are large). |
| Wait | Poll `kubectl -n envoy-gateway-system rollout status deployment/envoy-gateway` before returning. |
| Dependency | Runs after `installgatewayapi` so Gateway API CRDs exist when the controller starts. |

### `installmetricsserver` — `pkg/cluster/internal/create/actions/installmetricsserver/`

| Concern | Approach |
|---------|----------|
| Manifest source | Embed `components.yaml` from `github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml`. |
| kind-specific patch | Metrics Server refuses to start in kind because kubelet uses self-signed TLS certificates. The action must patch the Deployment to add `--kubelet-insecure-tls` to the container args immediately after applying the manifest, before waiting for rollout. Use `kubectl patch -n kube-system deployment metrics-server --type=json -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'` run as a node command. |
| Wait | `kubectl -n kube-system rollout status deployment/metrics-server`. |

### `installcorednstuning` — `pkg/cluster/internal/create/actions/installcorednstuning/`

| Concern | Approach |
|---------|----------|
| What changes | Patch the `coredns` ConfigMap in `kube-system` to increase cache TTL and add `cache 300` directive. Does not replace the full ConfigMap; uses `kubectl patch` to avoid overwriting cluster-specific configuration. |
| Implementation | Run a heredoc-style `kubectl patch configmap coredns -n kube-system --type merge` on the control plane node. Then `kubectl -n kube-system rollout restart deployment/coredns` to pick up the change. |
| Wait | `kubectl -n kube-system rollout status deployment/coredns`. |
| Why this approach | CoreDNS is already installed by kubeadm before the addon actions run. This action tunes it rather than reinstalling it. No new manifest is needed — only a targeted patch. |

### `installdashboard` — `pkg/cluster/internal/create/actions/installdashboard/`

| Concern | Approach |
|---------|----------|
| Version | Kubernetes Dashboard v2.7.0 via `recommended.yaml` manifest. v3+ dropped plain manifest support in favour of Helm+Kong; v2.7.0 is the last version installable via a single `kubectl apply -f` and remains compatible with current Kubernetes versions. |
| Manifest source | Embed `recommended.yaml` at build time. |
| RBAC | After apply, create a `ServiceAccount` named `kinder-admin` in `kubernetes-dashboard` namespace and bind it to `cluster-admin`. This is appropriate for a local development tool. |
| Access | The action logs a `kubectl -n kubernetes-dashboard create token kinder-admin` command to stdout so users can immediately get a token. |
| Wait | `kubectl -n kubernetes-dashboard rollout status deployment/kubernetes-dashboard`. |

---

## Recommended Changes to Project Structure

### New Files

```
pkg/cluster/internal/create/actions/
├── installmetallb/
│   ├── metallb.go               ← action implementation
│   └── manifests/
│       └── metallb-native.yaml  ← embedded, pinned version
├── installgatewayapi/
│   ├── gatewayapi.go
│   └── manifests/
│       └── standard-install.yaml
├── installenvoygw/
│   ├── envoygw.go
│   └── manifests/
│       └── install.yaml
├── installmetricsserver/
│   ├── metricsserver.go
│   └── manifests/
│       └── components.yaml
├── installcorednstuning/
│   └── corednstuning.go         ← no embedded manifest; patches in-place
└── installdashboard/
    ├── dashboard.go
    └── manifests/
        └── recommended.yaml
```

### Modified Files

```
pkg/internal/apis/config/types.go          ← add Addons struct to Cluster
pkg/apis/config/v1alpha4/types.go          ← add Addons to public Cluster type
pkg/internal/apis/config/convert_v1alpha4.go ← convert Addons field
pkg/internal/apis/config/default.go        ← set all Addons.Disable* = false
pkg/internal/apis/config/validate.go       ← validate Addons (no-op for bools)
pkg/internal/apis/config/zz_generated.deepcopy.go ← regenerate
pkg/cluster/internal/create/create.go      ← wire new actions into pipeline
```

---

## Config Schema Extension

### Internal types (`pkg/internal/apis/config/types.go`)

Add to the `Cluster` struct:

```go
// Addons controls which default addons are installed during cluster creation.
// All addons are enabled by default; set the corresponding Disable field to
// true to skip installation.
Addons Addons
```

New type in the same file:

```go
// Addons holds per-addon opt-out flags for kinder's default addon suite.
type Addons struct {
    // DisableMetalLB skips MetalLB installation when true.
    // MetalLB provides LoadBalancer service support via Layer2 ARP.
    DisableMetalLB bool

    // DisableGatewayAPI skips Gateway API CRD installation when true.
    // Disabling this also implicitly skips Envoy Gateway.
    DisableGatewayAPI bool

    // DisableEnvoyGateway skips Envoy Gateway controller installation when true.
    // Has no effect if DisableGatewayAPI is also true.
    DisableEnvoyGateway bool

    // DisableMetricsServer skips Metrics Server installation when true.
    DisableMetricsServer bool

    // DisableCoreDNSTuning skips CoreDNS cache tuning when true.
    DisableCoreDNSTuning bool

    // DisableDashboard skips Kubernetes Dashboard installation when true.
    DisableDashboard bool
}
```

### v1alpha4 public type (`pkg/apis/config/v1alpha4/types.go`)

Mirror the same `Addons` struct with YAML/JSON tags on `Cluster`:

```go
Addons Addons `yaml:"addons,omitempty" json:"addons,omitempty"`
```

```go
type Addons struct {
    DisableMetalLB       bool `yaml:"disableMetalLB,omitempty" json:"disableMetalLB,omitempty"`
    DisableGatewayAPI    bool `yaml:"disableGatewayAPI,omitempty" json:"disableGatewayAPI,omitempty"`
    DisableEnvoyGateway  bool `yaml:"disableEnvoyGateway,omitempty" json:"disableEnvoyGateway,omitempty"`
    DisableMetricsServer bool `yaml:"disableMetricsServer,omitempty" json:"disableMetricsServer,omitempty"`
    DisableCoreDNSTuning bool `yaml:"disableCoreDNSTuning,omitempty" json:"disableCoreDNSTuning,omitempty"`
    DisableDashboard     bool `yaml:"disableDashboard,omitempty" json:"disableDashboard,omitempty"`
}
```

**Backward compatibility:** All fields are `omitempty` and default to `false` (all addons enabled). Existing `kind.x-k8s.io/v1alpha4` cluster configs that omit the `addons` key continue to work unchanged — they get all addons. The YAML decoder in `encoding/load.go` uses `yamlUnmarshalStrict` which will reject unknown fields, so the field must be added to the v1alpha4 type before any config files use it.

### Example user config (opt-out pattern)

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
addons:
  disableDashboard: true
  disableEnvoyGateway: true
nodes:
  - role: control-plane
```

---

## Architectural Patterns

### Pattern 1: Embedded Manifest with Embedded Go

**What:** Each addon action embeds its manifest YAML using `//go:embed manifests/*.yaml` (Go 1.16+). The manifest bytes are passed directly to `kubectl apply -f -` via stdin.

**When:** All addons except CoreDNS tuning (which patches in place).

**Example (installmetricsserver):**

```go
package installmetricsserver

import _ "embed"

//go:embed manifests/components.yaml
var metricsServerManifest []byte

func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing Metrics Server")
    defer ctx.Status.End(false)

    allNodes, err := ctx.Nodes()
    // ...
    node := controlPlanes[0]

    if err := node.Command(
        "kubectl", "apply", "--kubeconfig=/etc/kubernetes/admin.conf",
        "--server-side", "-f", "-",
    ).SetStdin(bytes.NewReader(metricsServerManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply metrics-server manifest")
    }

    // kind-required patch: kubelet uses self-signed certs
    if err := node.Command(
        "kubectl", "patch", "-n", "kube-system",
        "deployment", "metrics-server",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "--type=json",
        "-p", `[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]`,
    ).Run(); err != nil {
        return errors.Wrap(err, "failed to patch metrics-server for insecure TLS")
    }

    return waitForRollout(node, "kube-system", "deployment/metrics-server")
}
```

### Pattern 2: Dynamic Manifest (MetalLB IP Pool)

**What:** Some configuration cannot be known until runtime (the docker network CIDR). Generate the manifest string in Go after querying the provider, then pipe it to `kubectl apply`.

**When:** MetalLB `IPAddressPool` and `L2Advertisement` objects.

**Example:**

```go
func detectDockerNetworkSubnet(clusterName string) (string, error) {
    // "kind" is the fixed docker network name for kind clusters
    // exec runs on the host, not inside a node
    out, err := exec.Output(exec.Command(
        "docker", "network", "inspect", "kind",
        "--format", `{{(index .IPAM.Config 0).Subnet}}`,
    ))
    if err != nil {
        return "", errors.Wrap(err, "failed to inspect docker network")
    }
    return strings.TrimSpace(string(out)), nil
}

func ipPoolRange(subnet string) (string, error) {
    _, ipNet, err := net.ParseCIDR(subnet)
    if err != nil {
        return "", err
    }
    // Use the last /28 of the network as the MetalLB pool
    // e.g. 172.18.0.0/16 -> 172.18.255.200-172.18.255.250
    // Implementation: set last octet range to 200-250 on base network
    // ...
    return computedRange, nil
}
```

### Pattern 3: Rollout Wait via Node Command

**What:** All addon actions that deploy workloads must wait for them to be Ready before returning, to ensure the cluster is fully functional when `kinder create cluster` exits.

**When:** Every addon action except CoreDNS tuning (which waits for the existing CoreDNS rollout to complete after restart).

**Example:**

```go
func waitForRollout(node nodes.Node, namespace, target string) error {
    cmd := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "rollout", "status",
        "--namespace", namespace,
        "--timeout=120s",
        target,
    )
    if err := cmd.Run(); err != nil {
        return errors.Wrapf(err, "timed out waiting for %s rollout", target)
    }
    return nil
}
```

### Pattern 4: Opt-Out Guard

**What:** Each new action in `create.go` is wrapped in a conditional that checks `ctx.Config.Addons.DisableX`.

**When:** Wiring all addon actions into the pipeline.

**Example (in `create.go`):**

```go
// addon actions — all enabled by default, opt-out via config.Addons
if !opts.Config.Addons.DisableMetalLB {
    actionsToRun = append(actionsToRun, installmetallb.NewAction())
}
if !opts.Config.Addons.DisableGatewayAPI {
    actionsToRun = append(actionsToRun, installgatewayapi.NewAction())
    if !opts.Config.Addons.DisableEnvoyGateway {
        actionsToRun = append(actionsToRun, installenvoygw.NewAction())
    }
}
if !opts.Config.Addons.DisableMetricsServer {
    actionsToRun = append(actionsToRun, installmetricsserver.NewAction())
}
if !opts.Config.Addons.DisableCoreDNSTuning {
    actionsToRun = append(actionsToRun, installcorednstuning.NewAction())
}
if !opts.Config.Addons.DisableDashboard {
    actionsToRun = append(actionsToRun, installdashboard.NewAction())
}
```

**Note:** `DisableGatewayAPI` gates `DisableEnvoyGateway`. If a user disables the Gateway API CRDs, the Envoy Gateway action is automatically skipped regardless of its own flag, because the controller cannot function without the CRDs.

---

## Data Flow: Addon Installation

```
kinder create cluster
  │
  │  1. Container network exists
  │     Provider.Provision() creates node containers on the "kind" bridge network.
  │     The bridge has a subnet (e.g., 172.18.0.0/16) assigned by Docker.
  │
  │  2. Kubernetes is running (waitforready complete)
  │     admin.conf is at /etc/kubernetes/admin.conf on control-plane node.
  │     All subsequent kubectl commands use this kubeconfig.
  │
  │  3. MetalLB addon
  │     a. Host: exec "docker network inspect kind" → parse subnet CIDR
  │     b. Compute IP pool range from subnet tail
  │     c. controlPlane.Command("kubectl apply ... -f -").SetStdin(metallbManifest)
  │     d. Generate IPAddressPool + L2Advertisement YAML in memory
  │     e. controlPlane.Command("kubectl apply ... -f -").SetStdin(poolManifest)
  │     f. Wait: "kubectl -n metallb-system rollout status deployment/controller"
  │
  │  4. Gateway API CRDs addon
  │     a. controlPlane.Command("kubectl apply --server-side -f -")
  │        .SetStdin(gatewayAPIManifest)   ← embedded standard-install.yaml
  │     b. No rollout wait (CRDs, not pods)
  │
  │  5. Envoy Gateway addon
  │     a. controlPlane.Command("kubectl apply --server-side -f -")
  │        .SetStdin(envoyGWManifest)      ← embedded install.yaml
  │     b. Wait: "kubectl -n envoy-gateway-system rollout status deployment/envoy-gateway"
  │
  │  6. Metrics Server addon
  │     a. controlPlane.Command("kubectl apply -f -")
  │        .SetStdin(metricsManifest)      ← embedded components.yaml
  │     b. Patch: kubectl patch deployment metrics-server --type=json
  │        (adds --kubelet-insecure-tls arg)
  │     c. Wait: "kubectl -n kube-system rollout status deployment/metrics-server"
  │
  │  7. CoreDNS tuning addon
  │     a. controlPlane.Command("kubectl patch configmap coredns -n kube-system ...")
  │        (merges cache settings into Corefile)
  │     b. controlPlane.Command("kubectl rollout restart deployment/coredns -n kube-system")
  │     c. Wait: "kubectl -n kube-system rollout status deployment/coredns"
  │
  │  8. Dashboard addon
  │     a. controlPlane.Command("kubectl apply -f -")
  │        .SetStdin(dashboardManifest)    ← embedded recommended.yaml
  │     b. controlPlane.Command("kubectl apply -f -")
  │        .SetStdin(rbacManifest)         ← inline YAML for kinder-admin SA + ClusterRoleBinding
  │     c. Wait: "kubectl -n kubernetes-dashboard rollout status deployment/kubernetes-dashboard"
  │     d. ctx.Logger.V(0).Info("Get dashboard token: kubectl -n kubernetes-dashboard create token kinder-admin")
  │
  └─ cluster ready; kubeconfig exported
```

---

## Integration Points with Existing Kind Code

### `pkg/cluster/internal/create/create.go`

**Lines 110-131** (action list construction) — the only file that must be
edited to wire new actions in. New actions are appended after
`waitforready.NewAction(...)`.

```go
// EXISTING (unchanged)
actionsToRun = append(actionsToRun,
    installstorage.NewAction(),
    kubeadmjoin.NewAction(),
    waitforready.NewAction(opts.WaitForReady),
)

// NEW — addon actions
if !opts.Config.Addons.DisableMetalLB {
    actionsToRun = append(actionsToRun, installmetallb.NewAction())
}
// ... (see Pattern 4 above)
```

New imports required:

```go
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installmetallb"
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installgatewayapi"
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installenvoygw"
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installmetricsserver"
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installcorednstuning"
"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installdashboard"
```

### `pkg/cluster/internal/create/actions/action.go`

**No changes.** `ActionContext` already provides everything addon actions need:
- `ctx.Nodes()` — get the control-plane node to run kubectl
- `ctx.Config` — access `cfg.Addons` for any runtime decisions
- `ctx.Provider` — get provider string to handle Docker vs Podman network names
- `ctx.Status` — display progress to user
- `ctx.Logger` — emit the dashboard token hint

### `pkg/internal/apis/config/types.go`

Add `Addons Addons` field to `Cluster` struct. Add `Addons` type. No existing
fields are modified.

### `pkg/apis/config/v1alpha4/types.go`

Add `Addons Addons` field with `yaml:"addons,omitempty"` to `Cluster`. Add
`Addons` type. `yamlUnmarshalStrict` will reject unknown fields, so this must
be done before any config file uses the new key.

### `pkg/internal/apis/config/convert_v1alpha4.go`

`Convertv1alpha4()` function — add one field copy:

```go
out.Addons = Addons{
    DisableMetalLB:       in.Addons.DisableMetalLB,
    DisableGatewayAPI:    in.Addons.DisableGatewayAPI,
    DisableEnvoyGateway:  in.Addons.DisableEnvoyGateway,
    DisableMetricsServer: in.Addons.DisableMetricsServer,
    DisableCoreDNSTuning: in.Addons.DisableCoreDNSTuning,
    DisableDashboard:     in.Addons.DisableDashboard,
}
```

### `pkg/internal/apis/config/default.go`

`SetDefaultsCluster()` — no change needed. Go zero values for `bool` are
`false`, which means all addons enabled by default. No explicit defaulting
required.

### `pkg/internal/apis/config/zz_generated.deepcopy.go`

Must be regenerated after adding `Addons` to the internal type. The `Addons`
struct contains only scalars (bools), so `DeepCopyInto` is trivially a field
copy. Run `go generate ./pkg/internal/apis/config/...` (or manually write the
trivial deep-copy, since booleans need no pointer handling).

### `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go`

Same regeneration requirement for the public v1alpha4 type.

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Applying Addon Manifests Before `waitforready`

**What:** Inserting addon actions between `kubeadmjoin` and `waitforready`.

**Why bad:** The API server may be up but nodes are `NotReady`. DaemonSets like
MetalLB speaker will be scheduled but pods won't start. Rollout waits will
time out. Race conditions produce flaky cluster creation.

**Instead:** All addon actions come after `waitforready`. The additional time
cost is minimal — `waitforready` ensures the node is schedulable before addon
pods are created.

### Anti-Pattern 2: Fetching Manifests at Runtime from the Internet

**What:** `kubectl apply -f https://...` or `curl | kubectl apply` inside node
commands.

**Why bad:** Cluster creation fails in air-gapped environments. Version pinning
is broken (latest URL is not pinned). Node containers may not have outbound
internet access depending on provider configuration.

**Instead:** Embed all manifests at build time using `//go:embed`. Embed the
specific pinned version. Update manifests by updating the embedded file and
rebuilding.

### Anti-Pattern 3: Adding New Methods to the Provider Interface

**What:** Adding a `GetNetworkSubnet()` method to `providers.Provider` to
abstract Docker/Podman/Nerdctl network inspection.

**Why bad:** All three providers would need updating. The provider interface
is an alpha-grade internal API that is already complex. The MetalLB action
only needs a network subnet, which can be obtained with a straightforward
`exec.Command("docker", "network", "inspect", ...)` keyed on the provider name.

**Instead:** In `installmetallb`, switch on `ctx.Provider.String()` to select
the appropriate CLI command. Docker and Nerdctl both use `docker network
inspect kind`. Podman uses `podman network inspect kind`. This keeps the
provider interface stable.

### Anti-Pattern 4: Using Helm for Addon Installation

**What:** Bundling Helm in the kinder binary or shelling out to a `helm`
binary to install addons.

**Why bad:** Adds a new external dependency. Helm must be present on the host
and version-compatible. The project constraint explicitly prohibits this.
Helm adds complexity for what is essentially a `kubectl apply`.

**Instead:** Apply static YAML manifests via kubectl. For addons with runtime
configuration (MetalLB pool range), generate the YAML string in Go.

### Anti-Pattern 5: Silently Skipping Failed Addon Installs

**What:** Catching errors from addon actions and logging a warning rather than
returning the error.

**Why bad:** The cluster appears ready but addons are non-functional. Users
discover this later with confusing failures.

**Instead:** Return errors from `Execute()` as the existing actions do. If
`opts.Retain` is false, the cluster is deleted on failure (existing behaviour).
If users need a cluster without a specific addon, they should use the opt-out
config flag rather than relying on silent failure.

---

## Action Ordering: Dependency Rationale

| Order | Action | Depends On | Reason |
|-------|--------|------------|--------|
| 1 | `installmetallb` | `waitforready` | Needs schedulable nodes; no dependency on other addons |
| 2 | `installgatewayapi` | `waitforready` | CRD apply only; no pod dependency; must precede Envoy GW |
| 3 | `installenvoygw` | `installgatewayapi` | Controller crashes without Gateway API CRDs present |
| 4 | `installmetricsserver` | `waitforready` | Independent; kubelets must be ready to serve metrics |
| 5 | `installcorednstuning` | kubeadm-installed CoreDNS (already up after `waitforready`) | Patches running deployment |
| 6 | `installdashboard` | `waitforready` | Independent; placing last is a style choice (cosmetic addon) |

MetalLB is placed first so that by the time Envoy Gateway is ready, any
`LoadBalancer` services it creates can be immediately assigned IPs. This
produces a cleaner end state where all services have external IPs when the
creation command exits.

---

## Scalability Considerations

| Concern | Single node (dev) | Multi-node |
|---------|------------------|------------|
| MetalLB IP pool | Sufficient with /28 from docker network | Same; pool is shared across all LB services |
| Envoy GW pods | Scheduled on control-plane (taint removed for single-node) | Scheduled on workers |
| Metrics Server | Works; kubelet endpoint on single node | Works; scrapes all kubelets |
| Dashboard | localhost port-forward sufficient | Same; not exposed externally by design |
| CoreDNS tuning | Minimal impact | Same configmap patch |

All addons are designed for local development clusters (1-3 nodes). No
production-scale considerations are needed.

---

## Sources

- Kind codebase read directly at `/Users/patrykattc/work/git/kinder` (commit 89ff06bd) — HIGH confidence
- [MetalLB Installation](https://metallb.universe.tf/installation/) — official docs, metallb-native.yaml manifest — HIGH confidence
- [MetalLB Configuration: IPAddressPool + L2Advertisement](https://metallb.universe.tf/configuration/) — HIGH confidence
- [kind LoadBalancer guide (MetalLB + kind pattern)](https://master--k8s-kind.netlify.app/docs/user/loadbalancer/) — MEDIUM confidence
- [Envoy Gateway Install with YAML](https://gateway.envoyproxy.io/docs/install/install-yaml/) — official docs — HIGH confidence
- [Envoy Gateway Quickstart](https://gateway.envoyproxy.io/latest/tasks/quickstart/) — HIGH confidence
- [Metrics Server GitHub: kind insecure-TLS requirement](https://github.com/kubernetes-sigs/metrics-server) — HIGH confidence
- [Kubernetes Dashboard v2.7.0 recommended.yaml](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/) — HIGH confidence; v3 Helm-only confirmed by multiple sources — HIGH confidence
- [CoreDNS cache tuning best practices](https://medium.com/@GiteshWadhwa/optimizing-dns-resolution-in-kubernetes-best-practices-for-coredns-performance-e3f6ed041bbb) — MEDIUM confidence (community source, consistent with official CoreDNS docs)
- [MetalLB + kind: compute docker network subnet for IP pool](https://michaelheap.com/metallb-ip-address-pool/) — MEDIUM confidence (community, consistent with official MetalLB docs)
