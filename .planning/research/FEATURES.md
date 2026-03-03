# Feature Landscape

**Domain:** Kubernetes local development CLI tool — v1.3 Harden & Extend milestone
**Researched:** 2026-03-03
**Confidence:** HIGH (based on direct codebase analysis + official docs for kind, cert-manager, registry:2)

---

## Context

This research covers the **v1.3 milestone**, not the v1.1 website work. The four feature areas are:

1. **Local registry addon** — `addons.localRegistry: true`, replaces kind's external shell script pattern
2. **cert-manager addon** — `addons.certManager: true`, TLS management for local services
3. **`kinder env` command** — Show cluster environment/config info for debugging
4. **`kinder doctor` command** — Diagnose common setup issues before they cause cluster creation failures
5. **Provider code deduplication** — Extract shared docker/podman/nerdctl code to `common/` package

There are also **four bug fixes** that are table stakes for this milestone (not new features, but blocking quality). These are documented separately in PITFALLS.md.

The existing Addons struct is `{MetalLB, EnvoyGateway, MetricsServer, CoreDNSTuning, Dashboard}`, all `*bool`. New addons follow the same `*bool` opt-out pattern.

---

## Table Stakes

Features users expect from a batteries-included Kubernetes tool. Missing these makes v1.3 feel incomplete or unreliable.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Local registry at `localhost:5001` | kind docs show this as the recommended pattern; every "kind + local dev" tutorial uses it; developers expect `docker push localhost:5001/myapp:tag` to just work | MEDIUM | Must create a `registry:2` container, configure containerd on every node via `/etc/containerd/certs.d/localhost:5001/hosts.toml`, create the `kind` network connection, and post the discovery ConfigMap to `kube-public/local-registry-hosting` |
| ConfigMap discovery for local registry | Tilt, Skaffold, and other dev tools look for `kube-public/local-registry-hosting` ConfigMap to auto-detect the registry; without it, third-party tools won't auto-configure | LOW | A single ConfigMap apply after registry container is up; data key is `localRegistryHosting.v1` |
| cert-manager installs and CRDs are ready | Users who enable `certManager: true` expect `kubectl apply -f my-certificate.yaml` to work immediately after cluster creation; CRD not-ready errors are silent killers | MEDIUM | cert-manager single-manifest install (all CRDs + controller in one YAML); must wait for webhook deployment readiness before cluster reported ready |
| Self-signed ClusterIssuer bootstrapped | cert-manager alone is not useful; a `ClusterIssuer` named `selfsigned` must exist so users can immediately create certificates without any manual setup | LOW | Two-resource apply: `ClusterIssuer` (selfSigned) + a CA `Certificate` + a CA `Issuer`; or minimally just the `ClusterIssuer`; depends on target use case |
| `kinder env` output machine-readable or clearly structured | CLI diagnostic commands must produce parseable output; mixing prose with data prevents scripting | LOW | Follow Go CLI convention: key=value lines or a clear table; support `--json` or structured format |
| `kinder env` shows provider, node image, cluster name, config path | These are the four things developers look up when something goes wrong ("which docker am I using?", "what image version?") | LOW | All data available from existing provider detection + config loading code; no new infrastructure needed |
| `kinder doctor` checks binary prerequisites | Checks that docker/podman/nerdctl is available and running before create; users expect an explicit diagnostic step, not cryptic "failed to list clusters" errors | LOW | Path lookup + a `docker info`/`podman info`/`nerdctl info` call; fail fast with actionable message |
| `kinder doctor` checks resource availability | Memory and disk space warnings before cluster creation prevents confusing OOM failures mid-cluster-setup | MEDIUM | Platform-specific syscall or parsing `/proc/meminfo`; 4GB RAM and 10GB disk are reasonable minimum thresholds for kind clusters |
| Provider code deduplication does not break existing behavior | Any refactoring of docker/podman/nerdctl providers must produce identical runtime behavior; this is pure internal quality work | HIGH | The three provider files share 70-80% logic; only the binary name and minor behavioral quirks differ; extract to `common/` without changing observable behavior |
| Bug fixes ship in v1.3 | The four identified bugs (defer-in-loop port leak, tar extraction data loss, ListInternalNodes default name, network sort) are correctness issues; they must be fixed | LOW-MEDIUM | Each bug is self-contained; see PITFALLS.md for details |

---

## Differentiators

Features that go beyond what kind offers or what users minimally expect. These are what make kinder's batteries-included promise feel complete.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Registry addon integrated with containerd config patches | kind's official local registry guide is a 50-line shell script that users must run manually; kinder does it via `addons.localRegistry: true` in YAML — same interface as all other addons | MEDIUM | Uses `ContainerdConfigPatches` mechanism already in kind config to inject `config_path` into containerd; then patches each node's `/etc/containerd/certs.d/` directory post-creation |
| cert-manager as default-enabled addon (or easy opt-in) | No other kind-based tool ships cert-manager integrated; developers doing TLS work spend 30-60 minutes manually setting it up for every cluster | MEDIUM | Must embed the cert-manager release manifest (currently v1.17.x, ~1.5MB YAML); wait for cert-manager webhook pod readiness before proceeding |
| `kinder doctor` checks addon-specific prerequisites | MetalLB requires subnet detection (macOS/Windows warning is already implemented); cert-manager requires cert-manager CRDs to be ready; a doctor command should surface these in advance | MEDIUM | Requires knowing which addons are enabled in the current config; doctor should accept a `--config` flag or infer from default config location |
| `kinder env` shows which addons are enabled | When debugging a cluster, knowing "did cert-manager actually get installed?" is the first question; `kinder env` should show the enabled/disabled state of each addon | LOW | Requires reading the config that was used to create the cluster; consider storing config summary in a cluster label or file |
| Pull-through cache mode for local registry | Configuring the registry as a Docker Hub mirror eliminates rate limiting in CI and speeds up iterative development; kind has an open issue requesting this | HIGH | Requires passing mirror config to containerd; adds complexity to registry setup; defer to v1.4 unless trivially composable |
| `kinder doctor` output has exit codes | Scripts testing infrastructure rely on `kinder doctor && kinder create cluster`; non-zero exit code on failure enables this pattern | LOW | Standard CLI convention; just ensure `os.Exit(1)` on any failed check |

---

## Anti-Features

Features that seem relevant but should explicitly not be built in v1.3.

| Anti-Feature | Why Requested | Why Problematic | What to Do Instead |
|--------------|---------------|-----------------|-------------------|
| Harbor as local registry | Harbor has a UI, scanning, RBAC, replication — "proper" registry | Harbor requires Helm; requires 3+ pods; 500MB+ images; defeats local dev zero-config goal | Use `registry:2` (Docker Distribution); it's 25MB, zero config, sufficient for local push/pull |
| ACME/Let's Encrypt issuer in cert-manager addon | "Real" TLS for local services | ACME requires internet reachability and public DNS; local clusters can't get Let's Encrypt certs | Ship with self-signed `ClusterIssuer` only; document that ACME issuers require additional setup |
| cert-manager trust-manager addon | Distributes CA certs across namespaces automatically | Adds another CRD bundle and controller; cross-namespace trust is an edge case for local dev | Document that users needing cross-namespace trust can install trust-manager separately |
| Interactive `kinder doctor --fix` | Auto-fixing detected issues (e.g., starting Docker daemon) | Side effects without user consent are dangerous; "fix" for one issue may break something else | Print clear instructions for how to fix each issue; leave execution to the user |
| `kinder doctor` as an ongoing health monitor | Polling cluster health | This is `kubectl get componentstatuses` territory; doctor is a pre-creation diagnostic tool, not a monitoring agent | Keep doctor as a point-in-time check; document `kubectl get nodes` and `kubectl cluster-info` for runtime health |
| Registry UI (docker registry UI) | Visual interface for browsing pushed images | Adds a second container; requires port mapping; not standard in kind-like tools | Use `docker images` or `curl localhost:5001/v2/_catalog` for introspection; document in addon docs |
| Helm-based cert-manager install | Helm gives more configuration options | Kinder has explicitly avoided Helm as a dependency (PROJECT.md constraint); Helm install adds complexity | Embed the official cert-manager `kubectl apply` manifest; pin a specific version |

---

## Feature Dependencies

```
Local Registry Addon
    requires──> ContainerdConfigPatches mechanism (already in kind/kinder config)
    requires──> Provider.Provision() completes (nodes must exist before patching containerd)
    requires──> Network connection between registry container and kind network
    enables──> cert-manager addon can pull images from registry (optional but nice)
    enables──> Any future CI/CD addon that needs a push target

cert-manager Addon
    requires──> Cluster is running (CRDs applied via kubectl)
    requires──> waitforready action completes (apiserver must be up)
    enables──> Envoy Gateway TLS termination (existing addon gains TLS capability)
    enables──> any user Certificate resources
    ordering──> Must come AFTER Envoy Gateway addon (so cert-manager can issue certs
                for Gateway routes if desired; Gateway CRDs must exist first)

kinder env command
    requires──> Existing provider detection code (already in pkg/cluster/provider.go)
    requires──> Config loading machinery (already in pkg/internal/apis/config/encoding/)
    no new infrastructure needed

kinder doctor command
    requires──> Binary detection code (same as provider detection)
    enhanced by──> Reading config file to know which addons to check
    depends on──> kinder env output (may share underlying data gathering)

Provider Deduplication
    requires──> Understanding all three provider implementations (done: analysis above)
    blocks──> Adding Binary() method uniformly to all providers (nerdctl already has it;
              docker and podman hardcode the binary name as a const)
    enables──> Local registry addon (registry container management reuses provider binary)
    enables──> Easier future provider additions
    risk──> Must not change observable behavior; needs tests for all three providers
```

### Dependency Notes

- **Provider deduplication should come first:** The local registry addon needs to manage a registry container using whichever container runtime is active (docker/podman/nerdctl). If provider code is consolidated first, the registry addon can use the provider's binary uniformly. If not, registry will hardcode the binary or add its own detection.
- **cert-manager requires embedded manifest:** At ~1.5MB, the cert-manager YAML is larger than MetalLB (~300KB). The `go:embed` approach (already used for all other addons) handles this without issue. The file must be version-pinned.
- **Local registry is provider-aware:** On Docker, the registry container connects to the `kind` Docker network. On Podman, networking differences may apply (Podman uses a different network model for rootless). On nerdctl, the network is nerdctl-specific. The registry addon must use the detected provider binary consistently.
- **`kinder doctor` is additive:** It adds a new top-level command alongside `create`, `delete`, `get`. It follows the existing Cobra command pattern in `pkg/cmd/kind/`.

---

## MVP Definition for v1.3

### Must Ship (v1.3)

The minimum changes that fulfill the milestone promise.

- Bug fixes (defer-in-loop, tar extraction, ListInternalNodes default name, network sort) — correctness, not optional
- Provider code deduplication — precondition for registry addon; also reduces drift risk
- Local registry addon (`addons.localRegistry: true`) with registry container, containerd config, and discovery ConfigMap
- cert-manager addon (`addons.certManager: true`) with embedded manifest and self-signed ClusterIssuer
- `kinder env` command showing provider, node image, cluster name, addon states
- `kinder doctor` command checking binary availability, daemon running, resource minimums
- Updated `go.mod` minimum version and dependency pins

### Explicitly Out of Scope for v1.3

- Pull-through cache mode for local registry — added complexity, defer to v1.4
- trust-manager alongside cert-manager — edge case, defer
- `kinder doctor --fix` auto-remediation — too risky
- ACME/Let's Encrypt issuer — requires internet, not suitable for local dev

---

## Complexity Assessment

| Feature | Estimated Complexity | Primary Risk |
|---------|---------------------|--------------|
| Bug fixes (4 bugs) | LOW each | Regression if not tested |
| Provider deduplication | HIGH | Breaking behavioral changes in one of three providers |
| Local registry addon | MEDIUM | Networking differences across Docker/Podman/nerdctl providers |
| cert-manager addon | MEDIUM | Webhook readiness timing; large embedded manifest |
| `kinder env` command | LOW | None; purely read-only data display |
| `kinder doctor` command | LOW-MEDIUM | Platform-specific resource checks (disk/memory differ by OS) |

---

## Implementation Notes

### Local Registry Addon — Key Technical Details

The official kind local registry guide (https://kind.sigs.k8s.io/docs/user/local-registry/) establishes the pattern. The addon must:

1. Create a `registry:2` container (e.g., named `kinder-registry`) on the host
2. Connect it to the kind docker/podman/nerdctl network
3. Patch containerd config on every node to enable `config_path = "/etc/containerd/certs.d"`
4. Create `/etc/containerd/certs.d/localhost:5001/hosts.toml` on each node:
   ```
   [host."http://kinder-registry:5000"]
   ```
5. Apply the discovery ConfigMap to `kube-public` namespace:
   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: local-registry-hosting
     namespace: kube-public
   data:
     localRegistryHosting.v1: |
       host: "localhost:5001"
       help: "https://kinder.patrykgolabek.dev/docs/addons/local-registry"
   ```

Port mapping: host `localhost:5001` → registry container `5000`. Image push command: `docker push localhost:5001/myapp:tag`.

**Provider difference:** The registry container itself needs to be created using the same container runtime as the cluster nodes. The `Binary()` method (already present on nerdctl provider, absent from docker/podman) must be uniformly available post-deduplication.

### cert-manager Addon — Key Technical Details

Standard installation via `kubectl apply -f cert-manager.yaml` using go:embed. Current stable: v1.17.x (check releases for exact pin). The addon must:

1. Apply the single-file cert-manager manifest (all CRDs + namespace + controllers)
2. Wait for the cert-manager webhook deployment to be ready (webhook not ready = certificate requests fail silently)
3. Apply a `ClusterIssuer` for self-signed usage:
   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: selfsigned
   spec:
     selfSigned: {}
   ```

The webhook readiness wait is the critical step — cert-manager is known for failing silently if you create a `Certificate` resource before the webhook pod is ready. Wait pattern: same as existing addons (check deployment rollout).

### Provider Deduplication — Key Technical Details

From direct code analysis, docker and nerdctl `provider.go` files are structurally identical except:
- nerdctl passes `binaryName string` through all functions (where docker hardcodes `"docker"`)
- podman has a different JSON port parsing path for its API versioning history
- `dockerInfo` struct is duplicated verbatim in both docker and nerdctl

The `info()` function, `dockerInfo` struct, `CollectLogs()`, `ListClusters()`, `ListNodes()`, `DeleteNodes()`, `GetAPIServerEndpoint()`, and `GetAPIServerInternalEndpoint()` are all near-identical across the three providers.

The `common/` package already exists (`pkg/cluster/internal/providers/common/`). The refactoring strategy is: extract all binary-agnostic shared logic to `common/`, pass `binaryName string` as a parameter. Podman's unique path-parsing logic stays in `podman/`.

### `kinder env` Command — Key Technical Details

Placement: `pkg/cmd/kind/env/env.go` (new directory following existing command pattern).

Data sources:
- Provider: from `cluster.DetectNodeProvider()` or stored in cluster
- Node image: from cluster node containers (inspect labels)
- Config path: from `--config` flag or default `~/.kinder/config.yaml`
- Addon state: from the config that was used to create the cluster

Output format: structured key-value for easy grepping; optional `--json` for scripting.

### `kinder doctor` Command — Key Technical Details

Placement: `pkg/cmd/kind/doctor/doctor.go` (new directory).

Checks to implement (ordered by severity):
1. Container runtime binary found in PATH
2. Container runtime daemon is running (`docker info`, `podman info`, `nerdctl info`)
3. Minimum memory available (warn at < 4GB, error at < 2GB)
4. Minimum disk space available (warn at < 10GB free)
5. `kubectl` binary found in PATH (warn, not error — kinder bundles its own kubectl calls)
6. Existing cluster name conflict check (if `--name` flag provided)

Exit codes: 0 = all checks passed, 1 = any check failed, 2 = any check warns (if distinguishing warnings from errors).

---

## Sources

- [kind Local Registry Guide](https://kind.sigs.k8s.io/docs/user/local-registry/) — HIGH confidence (official docs; defines the canonical pattern kinder should automate)
- [cert-manager Installation via kubectl](https://cert-manager.io/docs/installation/kubectl/) — HIGH confidence (official cert-manager docs)
- [cert-manager SelfSigned Issuer Configuration](https://cert-manager.io/docs/configuration/selfsigned/) — HIGH confidence (official docs)
- [cert-manager GitHub Releases](https://github.com/cert-manager/cert-manager/releases) — HIGH confidence (for version pinning)
- [Docker Distribution (registry:2) Pull-Through Cache](https://distribution.github.io/distribution/recipes/mirror/) — HIGH confidence (official CNCF distribution docs)
- [MicroK8s cert-manager addon](https://microk8s.io/docs/addon-cert-manager) — MEDIUM confidence (precedent for cert-manager as a batteries-included addon)
- Kinder codebase direct analysis (`pkg/cluster/internal/providers/*/provider.go`, `provision.go`, `network.go`) — HIGH confidence (first-party source)
- Kinder `.planning/PROJECT.md` and `.planning/codebase/` docs — HIGH confidence (project documentation)

---

*Feature research for: kinder v1.3 Harden & Extend milestone*
*Researched: 2026-03-03*
