# Phase 51: Upstream Sync & K8s 1.36 - Research

**Researched:** 2026-05-07
**Domain:** Kubernetes upstream sync, load-balancer migration (HAProxy → Envoy), K8s 1.36 adoption
**Confidence:** HIGH (all claims verified against GitHub API, official Kubernetes docs, or direct code inspection)

---

## Summary

Phase 51 has four independent tracks: (1) adopting kind PR #4127 — the HAProxy→Envoy LB transition already merged to kind main, (2) bumping the default `kindest/node` to 1.36 once a publishable image exists, (3) adding a config-time guard that rejects `kubeProxyMode: ipvs` on K8s ≥1.36 nodes, and (4) writing a recipe page for K8s 1.36 GA features.

The upstream kind HAProxy→Envoy migration (PR #4127, merged 2026-04-02) is fully documented here with exact diff needed in kinder. The migration touches five files in kinder: `loadbalancer/const.go`, `loadbalancer/config.go`, `actions/loadbalancer/loadbalancer.go`, and all three provider `create.go`/`provision.go` files. The change is mechanical but requires care because the Envoy container is started with a bootstrap command (not just an image), unlike HAProxy which was configured via SIGHUP.

The default node image situation is blocked: as of 2026-05-07, no `kindest/node:v1.36.x` image has been published to Docker Hub (the latest is `v1.35.1`). Kind's main branch still defaults to `v1.35.1`. Kinder will need to either wait for kind to publish a v1.36 image (likely with a v0.32.0 release), or build and push its own. The requirement `≥1.36.4` in the ROADMAP reflects the issue #4131 regression pattern applied by analogy to 1.36 — but issue #4131 was actually about a 1.35 regression (not 1.36). As of today there are no 1.36 patch releases yet (only v1.36.0 exists). The planner must decide whether to: (a) use v1.36.0 and add a note about the floor, or (b) wait for v1.36.4 to ship.

IPVS in kube-proxy was deprecated in K8s v1.35 (not removed in 1.36). The 1.36 changelog bug-fix note actually _fixed_ an IPVS regression from 1.34. The ROADMAP intent of "silent IPVS removal breakage" is better understood as: users who set `kubeProxyMode: ipvs` in a 1.36+ cluster will eventually hit undefined behavior as IPVS becomes increasingly unmaintained. Kinder should add a deprecation warning (not hard rejection) that fires when `ipvs` is used with a node image ≥1.36.

**Primary recommendation:** Implement tracks in order of value: (1) Envoy LB migration — pure code sync, no image dependency, (2) IPVS deprecation guard — small validate.go addition, (3) default image bump — gated on image availability, (4) recipe page — content work with no code dependencies.

---

## Standard Stack

### Core (no new dependencies needed)
| Component | Current in kinder | Upstream kind | Change needed |
|-----------|-------------------|---------------|---------------|
| LB image | `docker.io/kindest/haproxy:v20260131-7181c60a` | `docker.io/envoyproxy/envoy:v1.36.2` | Update `const.go` |
| LB config | HAProxy `haproxy.cfg` Go template | Envoy YAML (3 templates: bootstrap, LDS, CDS) | Replace `config.go` |
| LB config path | `/usr/local/etc/haproxy/haproxy.cfg` | `/home/envoy/envoy.yaml` + `/home/envoy/cds.yaml` + `/home/envoy/lds.yaml` | Update `const.go` |
| LB start mechanism | `docker run <image>` (HAProxy reads config on start) | `docker run <image> bash -c "..."` (Envoy needs bootstrap) | Update all 3 providers |
| LB reload mechanism | `kill -s HUP 1` | Atomic file swap via `mv` | Update `actions/loadbalancer/loadbalancer.go` |
| Default node image | `kindest/node:v1.35.1@sha256:...` | `kindest/node:v1.35.1@sha256:...` (same on main) | Bump to 1.36 when available |
| IPVS guard | none | none | New — kinder-specific addition |
| Recipe page | n/a | n/a | New content file |

### No New Go Dependencies
The Envoy migration does not add any new Go module dependencies. All changes are template strings and constant updates within existing packages.

---

## Architecture Patterns

### SYNC-01: HAProxy → Envoy LB Migration

**What changed in upstream kind (PR #4127, merged 2026-04-02):**

#### Deleted files (kind)
- `images/haproxy/` entire directory — Dockerfile, Makefile, cloudbuild.yaml, haproxy.cfg, stage-binary-and-deps.sh. This directory does NOT exist in kinder (kinder uses the pre-built `kindest/haproxy` image). No deletion needed.

#### Changed: `pkg/cluster/internal/loadbalancer/const.go`

**Kinder current** (`/Users/patrykattc/work/git/kinder/pkg/cluster/internal/loadbalancer/const.go`, lines 20–23):
```go
const Image = "docker.io/kindest/haproxy:v20260131-7181c60a"
const ConfigPath = "/usr/local/etc/haproxy/haproxy.cfg"
```

**Upstream kind new** (`docker.io/envoyproxy/envoy:v1.36.2`, no `ConfigPath` — replaced by three path constants):
```go
const Image = "docker.io/envoyproxy/envoy:v1.36.2"
// No ConfigPath — replaced by:
const (
    ProxyConfigPathCDS = "/home/envoy/cds.yaml"
    ProxyConfigPathLDS = "/home/envoy/lds.yaml"
    ProxyConfigPath    = "/home/envoy/envoy.yaml"
    ProxyConfigDir     = "/home/envoy"
)
```

#### Changed: `pkg/cluster/internal/loadbalancer/config.go`

**Kinder current**: Single `DefaultConfigTemplate` (HAProxy config), single `Config(data)` function → one string.

**Upstream kind new**: Three templates + modified `Config()` + new `GenerateBootstrapCommand()`:
- `DynamicFilesystemConfigTemplate` — bootstrap YAML (uses `fmt.Sprintf`, not Go template)
- `ProxyLDSConfigTemplate` — listener config (Go template, uses `ConfigData`)
- `ProxyCDSConfigTemplate` — cluster config with health checks + backend servers (Go template, uses `hostPort` FuncMap)
- `Config(data *ConfigData, configTemplate string) (string, error)` — now takes template as parameter, adds `hostPort` FuncMap
- `GenerateBootstrapCommand(clusterName, containerName string) []string` — new function; returns the bash command used to initialize Envoy and start it

The `ConfigData` struct is **unchanged** (same three fields: `ControlPlanePort int`, `BackendServers map[string]string`, `IPv6 bool`).

**Full upstream config.go content is captured below under "Code Examples".**

#### Changed: `pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go`

**Kinder current**: generates one config string, writes to `loadbalancer.ConfigPath`, sends `kill -s HUP 1`.

**Upstream kind new**:
- Generates two configs: `ldsConfig` and `cdsConfig`  
- Writes to tmp files then atomically swaps with `mv` (no more SIGHUP)
- Uses `loadbalancer.ProxyConfigPathLDS` and `loadbalancer.ProxyConfigPathCDS`

Key diff:
```go
// OLD (kinder):
loadbalancerConfig, err := loadbalancer.Config(&loadbalancer.ConfigData{...})
nodeutils.WriteFile(loadBalancerNode, loadbalancer.ConfigPath, loadbalancerConfig)
loadBalancerNode.Command("kill", "-s", "HUP", "1").Run()

// NEW (upstream kind):
ldsConfig, _ := loadbalancer.Config(data, loadbalancer.ProxyLDSConfigTemplate)
cdsConfig, _ := loadbalancer.Config(data, loadbalancer.ProxyCDSConfigTemplate)
nodeutils.WriteFile(loadBalancerNode, tmpLDS, ldsConfig)
nodeutils.WriteFile(loadBalancerNode, tmpCDS, cdsConfig)
cmd := fmt.Sprintf("chmod 666 %s %s && mv %s %s && mv %s %s", ...)
loadBalancerNode.Command("sh", "-c", cmd).Run()
```

#### Changed: Provider create/provision files (3 files)

All three providers need `GenerateBootstrapCommand` appended after the image in docker run args:

**docker** (`pkg/cluster/internal/providers/docker/create.go`, line 284):
```go
// OLD:
return append(args, loadbalancer.Image), nil

// NEW:
args = append(args, loadbalancer.Image)
args = append(args, loadbalancer.GenerateBootstrapCommand(cfg.Name, name)...)
return args, nil
```

**podman** (`pkg/cluster/internal/providers/podman/provision.go`): Same pattern — add `GenerateBootstrapCommand` after image.

**nerdctl** (`pkg/cluster/internal/providers/nerdctl/create.go`): Same pattern — add `GenerateBootstrapCommand` after image.

#### Changed: `pkg/internal/doctor/offlinereadiness.go`

Line 51: Update the hardcoded haproxy image to the envoy image:
```go
// OLD:
{"docker.io/kindest/haproxy:v20260131-7181c60a", "Load Balancer (HA)"},

// NEW:
{"docker.io/envoyproxy/envoy:v1.36.2", "Load Balancer (HA)"},
```

#### Snapshot / lifecycle files

Files in `pkg/internal/snapshot/` and `pkg/internal/lifecycle/` reference `kindest/haproxy` only in test fixtures and comments that use the string `"haproxy"` contextually (not the image constant). These do NOT need to be updated — they reference the role `external-load-balancer`, not the image name directly. Verify with `grep -r "haproxy" pkg/internal/` before the phase starts.

---

### SYNC-02: Default Node Image Bump

**Current kinder default** (`/Users/patrykattc/work/git/kinder/pkg/apis/config/defaults/image.go`, line 21):
```go
const Image = "kindest/node:v1.35.1@sha256:05d7bcdefbda08b4e038f644c4df690cdac3fba8b06f8289f30e10026720a1ab"
```

**Target**: `kindest/node:v1.36.x@sha256:<digest>` (≥1.36.4 per ROADMAP, but see caveat below)

**Status as of 2026-05-07**:
- K8s v1.36.0 released 2026-04-22 (only stable release; no patch releases exist)
- `kindest/node:v1.36.x` images: **NONE published** on Docker Hub (latest tag is `v1.35.1` from 2026-02-14)
- Kind's own `main` branch still defaults to `v1.35.1`
- Kind's latest release is v0.31.0 (2025-12-18); v0.32.0-alpha tag exists in git but no release

**Caveat on `≥1.36.4` floor**: The ROADMAP says "≥1.36.4 to avoid kind #4131 regression" — but issue #4131 is about a **1.35** StatefulSet regression (`MaxUnavailableStatefulSet` Beta on-by-default), not 1.36. The 1.36.4 floor in the ROADMAP appears to be a forward projection by analogy. As of today, v1.36.4 does not exist and may not be needed. The planner should decide: use v1.36.0 (available, but no published kindest/node image), or hold until kind publishes an image.

**Migration cost for image bump**: single-line change in `pkg/apis/config/defaults/image.go`. Also update `kinder-site/src/content/docs/guides/tls-web-app.md` line 28 (`kindest/node:v1.32.0` reference in example output) and the multi-version-clusters guide which shows `v1.35.1`.

---

### SYNC-03: IPVS Deprecation Guard

**What's happening upstream**:
- K8s v1.35 (released 2025-12): `ipvs` mode deprecated, not removed
- K8s v1.36 (released 2026-04): `ipvs` still present; a 1.34 regression in IPVS was actually *fixed* in 1.36
- IPVS removal version: "a future version" (not yet scheduled for a specific release)

**The guard kinder should add**: NOT a hard rejection, but a validation error that fires when `kubeProxyMode: ipvs` is used AND the node image version is ≥1.36. The SC says "rejected at validation time with a clear error message" — so a hard `errors.Errorf` in `Cluster.Validate()` is appropriate per the success criteria, even if the Kubernetes upstream status is "deprecated not removed."

**Where to add it**: `pkg/internal/apis/config/validate.go`, `Cluster.Validate()` function, after the existing KubeProxyMode check (lines 71–75). The validation needs to:
1. Check if `KubeProxyMode == IPVSProxyMode`
2. If yes, parse node image versions (reuse `imageTagVersion()` + `version.ParseSemantic()` already in that file)
3. If any node is ≥1.36, return error

**Error message template**:
```
kubeProxyMode: ipvs is not supported with Kubernetes 1.36+; kube-proxy IPVS mode was deprecated in v1.35 
and will be removed in a future release. Migrate to iptables (recommended) or nftables mode. 
See: https://kubernetes.io/docs/reference/networking/virtual-ips/ for migration guidance.
```

**Existing validation code path** (all in `pkg/internal/apis/config/validate.go`):
- `imageTagVersion(image string) (string, error)` — strips sha digest, extracts version tag (line 276)
- `version.ParseSemantic(tag)` — imported from `sigs.k8s.io/kind/pkg/internal/version` (line 326)
- Both already used by `validateVersionSkew()` — the IPVS check can reuse the same parsing approach

**Test to add**: `pkg/internal/apis/config/validate_test.go` — new test case "ipvs rejected on 1.36 node" in `TestClusterValidate`.

---

### SYNC-04: "What's New in K8s 1.36" Recipe Page

**Location**: `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md`

**Sidebar registration**: `kinder-site/astro.config.mjs`, in the `Guides` section items array (around line 77):
```js
{ slug: 'guides/k8s-1-36-whats-new' },
```

**Page format** (Starlight MDX, same as other guide pages):
- Frontmatter: `title`, `description`
- Prerequisites section
- Step-by-step sections
- Tip/note callouts using `:::tip`, `:::note`

**Feature 1: User Namespaces (GA in 1.36)**

- Status: GA as of v1.36, no feature gate needed (graduated from alpha/beta)
- Feature gate was `UserNamespacesSupport`, now stable and on by default
- REQUIREMENT: Linux kernel ≥5.12 (kind nodes run on Linux; default base image is `kindest/base:v20260214-ea8e5717` which should satisfy this)
- Pod spec minimal example:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: userns-demo
spec:
  hostUsers: false
  containers:
  - name: app
    image: fedora:42
    securityContext:
      runAsUser: 0
    command: ["sh", "-c", "whoami && cat /proc/self/uid_map"]
```
- Verification: `kubectl exec userns-demo -- cat /proc/self/uid_map` should show non-root UID mapping

**Feature 2: In-Place Pod Resize**

- Clarification: there are TWO distinct features:
  - `InPlacePodVerticalScaling` (container-level resize) — graduated to GA in **v1.35**, enabled by default in 1.36
  - `InPlacePodLevelResourcesVerticalScaling` (pod-level shared resource pool) — graduated to **Beta** in v1.36, enabled by default
- For the recipe page, demonstrate container-level resize (GA, simpler, works without extra feature gates)
- Requirements: cgroupv2 (kind nodes use cgroupv2 since K8s dropped cgroupv1 in 1.35)
- Minimal pod spec for container-level resize:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: resize-demo
spec:
  containers:
  - name: app
    image: registry.k8s.io/pause:3.10
    resources:
      requests:
        cpu: "100m"
        memory: "64Mi"
      limits:
        cpu: "200m"
        memory: "128Mi"
    resizePolicy:
    - resourceName: cpu
      restartPolicy: NotRequired
    - resourceName: memory
      restartPolicy: NotRequired
```
- Resize command: `kubectl patch pod resize-demo --subresource resize --patch '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"200m"}}}]}}'`
- Verify: `kubectl get pod resize-demo -o jsonpath='{.status.containerStatuses[0].resources}'`

**kubeadm config note**: K8s 1.36 is when kind plans to adopt kubeadm v1beta4 (issue #3847). Users with `kubeadmConfigPatches` using v1beta3 `extraArgs` (map syntax) will need dual patches. The recipe page should note this.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Envoy config templates | Custom template system | Direct port of upstream kind templates | Upstream tested; templates are simple YAML strings |
| Version parsing for IPVS guard | New parser | `imageTagVersion()` + `version.ParseSemantic()` (already in validate.go) | Already used for version-skew validation |
| kindest/node v1.36 image | Building from scratch | Wait for kind to publish, or `kind build node-image` | Kind's CI builds multi-arch images; single-arch local builds miss arm64 |

---

## Common Pitfalls

### Pitfall 1: Envoy needs bootstrap command, HAProxy did not
**What goes wrong:** Copying just the image swap will result in a container that exits immediately because Envoy requires its config directories to be initialized before it starts.
**Why it happens:** HAProxy reads `/usr/local/etc/haproxy/haproxy.cfg` on startup; Envoy needs both a bootstrap file AND the initial CDS/LDS files to exist (even as empty `resources: []`) or it errors out.
**How to avoid:** The `GenerateBootstrapCommand(clusterName, containerName)` function in upstream kind handles this — it generates the bash command that creates the dirs, writes the bootstrap config, writes empty initial CDS/LDS, then starts Envoy in a retry loop. This MUST be appended to docker run args in all three providers.
**Warning signs:** LB container exits immediately, HA cluster creation hangs at "Configuring the external load balancer" step.

### Pitfall 2: `kill -s HUP 1` no longer works for reload
**What goes wrong:** The old SIGHUP reload mechanism won't work with Envoy — Envoy uses file-based dynamic config (xDS from filesystem), so atomic file swap is the reload mechanism.
**How to avoid:** The `actions/loadbalancer/loadbalancer.go` must use the `mv` swap pattern, not `kill -s HUP 1`.

### Pitfall 3: Snapshot/metadata files reference HAProxy by image string
**What goes wrong:** `pkg/internal/snapshot/kindconfig.go`, metadata.go, and offlinereadiness.go still reference the old HAProxy image.
**How to avoid:** `offlinereadiness.go:51` must be updated. Run `grep -r "kindest/haproxy" .` before closing the PR.

### Pitfall 4: IPv6 handling in CDS template
**What goes wrong:** The CDS template uses `dns_lookup_family: AUTO` for IPv6 (not `V4_PREFERRED`). Getting this wrong causes connection failures in IPv6 clusters.
**How to avoid:** Port the exact template from upstream without changes to the conditionals.

### Pitfall 5: kindest/node v1.36 image not yet available
**What goes wrong:** Setting `Image = "kindest/node:v1.36.x@sha256:..."` fails because no such image exists on Docker Hub as of 2026-05-07.
**How to avoid:** Don't block the other three requirements on SYNC-02. Implement SYNC-01, SYNC-03, SYNC-04 first. Add a placeholder comment for SYNC-02 and revisit once kind publishes the image.

### Pitfall 6: IPVS "removed" vs "deprecated"
**What goes wrong:** Implementing a hard rejection may surprise users who are intentionally running 1.36 with IPVS (which still works in 1.36).
**How to avoid:** The error message should say "deprecated" not "removed," and include the migration URL. The hard rejection at config-validate time is the SC requirement, so it should be done — but the message must be accurate.

### Pitfall 7: kubeadm v1beta4 for 1.36 clusters
**What goes wrong:** Kind issue #3847 says kind will adopt v1beta4 for K8s 1.36+. The switch hasn't landed in a kind release yet, but if users have existing `kubeadmConfigPatches` they'll need dual patches.
**How to avoid:** Note in the recipe page that v1beta4 `extraArgs` changed from map to list syntax.

---

## Code Examples

### Upstream Envoy const.go (complete)
```go
package loadbalancer

// Image defines the loadbalancer image:tag
const Image = "docker.io/envoyproxy/envoy:v1.36.2"

const (
    ProxyConfigPathCDS = "/home/envoy/cds.yaml"
    ProxyConfigPathLDS = "/home/envoy/lds.yaml"
    ProxyConfigPath    = "/home/envoy/envoy.yaml"
    ProxyConfigDir     = "/home/envoy"
)
```

### Upstream GenerateBootstrapCommand (complete, from upstream config.go)
```go
func GenerateBootstrapCommand(clusterName, containerName string) []string {
    envoyConfig := fmt.Sprintf(
        DynamicFilesystemConfigTemplate,
        clusterName,
        containerName,
        ProxyConfigPathCDS,
        ProxyConfigPathLDS,
    )
    emptyConfig := "resources: []"
    return []string{"bash", "-c",
        fmt.Sprintf(`mkdir -p %s && echo -en '%s' > %s && echo -en '%s' > %s && echo -en '%s' > %s && while true; do envoy -c %s && break; sleep 1; done`,
            ProxyConfigDir,
            envoyConfig, ProxyConfigPath,
            emptyConfig, ProxyConfigPathCDS,
            emptyConfig, ProxyConfigPathLDS,
            ProxyConfigPath)}
}
```

### IPVS version guard (to add in validate.go Cluster.Validate())
```go
// After the existing kubeProxyMode check at line ~72
if c.Networking.KubeProxyMode == IPVSProxyMode {
    for _, n := range c.Nodes {
        tag, err := imageTagVersion(n.Image)
        if err != nil {
            continue // version unknown, skip guard
        }
        v, err := version.ParseSemantic(tag)
        if err != nil {
            continue // non-semver tag (e.g. "latest"), skip guard
        }
        if v.Minor() >= 36 {
            errs = append(errs, errors.Errorf(
                "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+ "+
                    "(node image %q uses v1.%d); kube-proxy IPVS mode was deprecated in v1.35. "+
                    "Switch to iptables or nftables. Migration guide: "+
                    "https://kubernetes.io/docs/reference/networking/virtual-ips/",
                n.Image, v.Minor()))
            break // one error is enough
        }
    }
}
```

### LDS config template (from upstream config.go)
```go
const ProxyLDSConfigTemplate = `
resources:
- "@type": type.googleapis.com/envoy.config.listener.v3.Listener
  name: listener_apiserver
  address:
    socket_address:
      address: {{ if .IPv6 }}"::"{{ else }}"0.0.0.0"{{ end }}
      port_value: {{ .ControlPlanePort }}
  filter_chains:
  - filters:
    - name: envoy.filters.network.tcp_proxy
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
        stat_prefix: ingress_tcp
        cluster: kube_apiservers
`
```

### Website guide sidebar registration (astro.config.mjs)
```js
// In the Guides items array:
{ slug: 'guides/k8s-1-36-whats-new' },
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `kindest/haproxy` LB container | `envoyproxy/envoy:v1.36.2` LB container | kind PR #4127, merged 2026-04-02 | Drops HAProxy; Envoy needs bootstrap command |
| SIGHUP reload for LB config | Atomic file swap (`mv`) | kind PR #4127 | No process kill needed |
| `ConfigPath = "/usr/local/etc/haproxy/haproxy.cfg"` | Three paths under `/home/envoy/` | kind PR #4127 | Three config files instead of one |
| IPVS not validated | IPVS rejected on ≥1.36 | kinder-specific (SYNC-03) | Users get actionable error instead of silent failure |
| K8s 1.35 default node | K8s 1.36 default node | SYNC-02 (blocked pending image) | New GA features available by default |
| UserNamespacesSupport feature gate | No gate needed (GA in 1.36) | K8s 1.36.0, 2026-04-22 | `hostUsers: false` works out of the box |
| `InPlacePodVerticalScaling` Beta | GA in 1.35, default-on in 1.36 | K8s 1.35 GA, 1.36 drops gate | Container resize without feature gate |

**Deprecated/outdated in this phase:**
- `docker.io/kindest/haproxy:v20260131-7181c60a` — no longer used; replace with envoyproxy/envoy:v1.36.2
- `loadbalancer.ConfigPath` constant — replaced by `ProxyConfigPathCDS`, `ProxyConfigPathLDS`, `ProxyConfigPath`
- HAProxy `DefaultConfigTemplate` — replaced by three Envoy YAML templates
- `kill -s HUP 1` pattern — replaced by atomic `mv` swap

---

## Open Questions

1. **kindest/node v1.36 image availability**
   - What we know: no v1.36.x kindest/node image exists on Docker Hub as of 2026-05-07; kind main still defaults to v1.35.1
   - What's unclear: when kind will publish v1.36 images (requires a new kind release, likely v0.32.0); whether there will be patch releases before 1.36.4
   - Recommendation: plan SYNC-02 as a separate sub-task that blocks on `kindest/node:v1.36.x` availability; the RESEARCH.md should be re-verified once images appear

2. **IPVS removal timeline vs. hard rejection semantics**
   - What we know: IPVS deprecated in v1.35, still present in v1.36 with bug fixes
   - What's unclear: whether a hard validation rejection is overly aggressive vs. a warning
   - Recommendation: implement as hard error per SC requirement, but phrase message as "deprecated and will be removed" not "removed"

3. **kubeadm v1beta4 and the 1.36 clusters**
   - What we know: kind issue #3847 is open; kubeadm v1beta4 planned for K8s 1.36+ in kind; hasn't shipped in a kind release yet
   - What's unclear: will kinder need to handle this before SYNC-02 can ship?
   - Recommendation: the recipe page should mention v1beta4 `extraArgs` syntax change but kinder's kubeadm config generation doesn't need to change until kind ships a release with v1beta4

4. **In-Place Pod Resize feature for recipe page: container-level or pod-level?**
   - What we know: container-level (`InPlacePodVerticalScaling`) is GA; pod-level (`InPlacePodLevelResourcesVerticalScaling`) is Beta in 1.36
   - Recommendation: demonstrate container-level resize (simpler, GA, no extra feature gates needed)

---

## Sources

### Primary (HIGH confidence — direct code inspection and API)
- `pkg/cluster/internal/loadbalancer/const.go` (kinder) — current HAProxy image and config path
- `pkg/cluster/internal/loadbalancer/config.go` (kinder) — current HAProxy template
- `pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go` (kinder) — current LB action
- `pkg/apis/config/defaults/image.go` (kinder) — current default node image
- `pkg/internal/apis/config/validate.go` (kinder) — existing validation logic
- `pkg/internal/doctor/offlinereadiness.go` (kinder) — HAProxy image reference
- GitHub API: `kubernetes-sigs/kind` `pkg/cluster/internal/loadbalancer/const.go` — upstream Image constant
- GitHub API: `kubernetes-sigs/kind` `pkg/cluster/internal/loadbalancer/config.go` — upstream Envoy templates
- GitHub API: `kubernetes-sigs/kind` `pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go` — upstream action
- GitHub API: `kubernetes-sigs/kind` `pkg/cluster/internal/providers/{docker,podman,nerdctl}/provision.go` — upstream provider changes
- `gh pr view 4127 --repo kubernetes-sigs/kind` — PR description with full test output and generated configs
- `gh issue view 4131 --repo kubernetes-sigs/kind` — regression about v1.35 StatefulSet (not v1.36)
- `gh release view v0.31.0 --repo kubernetes-sigs/kind` — latest kind release (v1.35.1 default)
- `gh release list --repo kubernetes/kubernetes` — only v1.36.0 exists, no patch releases
- Docker Hub API `hub.docker.com/v2/repositories/kindest/node/tags/?name=v1.36` — returns zero results
- `kinder-site/astro.config.mjs` — Starlight sidebar structure
- `kinder-site/src/content/docs/guides/` — recipe page format reference
- CHANGELOG-1.36.md (raw GitHub) — confirmed `UserNamespacesSupport` GA, `InPlacePodLevelResourcesVerticalScaling` Beta

### Secondary (MEDIUM confidence — official Kubernetes blog posts)
- https://kubernetes.io/blog/2026/04/23/kubernetes-v1-36-userns-ga/ — User Namespaces GA: `hostUsers: false`, no feature gate, kernel ≥5.12
- https://kubernetes.io/blog/2026/04/30/kubernetes-v1-36-inplace-pod-level-resources-beta/ — pod-level resize Beta, 4 feature gates required
- https://kubernetes.io/blog/2025/12/19/kubernetes-v1-35-in-place-pod-resize-ga/ — container-level resize GA in v1.35

### Tertiary (MEDIUM confidence — official blog + corroborating search)
- https://www.tigera.io/blog/from-ipvs-to-nftables-a-migration-guide-for-kubernetes-v1-35/ — IPVS deprecated in 1.35, nftables migration guide
- https://github.com/kubernetes/website/commit/6672edc94925d5b891c8dd988b822a61ac2d3e26 — "Mark IPVS proxy mode as deprecated in kubernetes 1.35"

---

## Metadata

**Confidence breakdown:**
- SYNC-01 (Envoy LB migration): HIGH — full upstream code read via GitHub API, PR description confirmed
- SYNC-02 (node image bump): MEDIUM — image doesn't exist yet; floor rationale (≥1.36.4) is unclear; planner decision needed
- SYNC-03 (IPVS guard): HIGH — validation path fully read, "deprecated in 1.35" confirmed from multiple sources
- SYNC-04 (recipe page): HIGH — K8s 1.36 feature status confirmed from official blog posts and CHANGELOG

**Research date:** 2026-05-07
**Valid until:** 2026-06-07 (kindest/node:v1.36.x image availability may change any day; re-verify SYNC-02 before planning)
