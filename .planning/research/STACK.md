# Technology Stack

**Project:** kinder — Milestone 2 (Go CLI additions)
**Scope:** Stack additions/changes for local registry addon, cert-manager addon, `kinder env` command, `kinder doctor` command, and provider code deduplication. Existing validated stack (Go, Cobra, go:embed, exec patterns) is not re-researched.
**Researched:** 2026-03-03
**Overall confidence:** HIGH for implementation patterns; MEDIUM for cert-manager version choice (depends on default node image k8s version)

---

## Executive Summary: What Actually Changes in go.mod

**New external dependencies: ZERO.**

All four feature areas are implemented using the standard library plus packages already present in go.mod. The only mandatory go.mod change is updating the `go` minimum version directive. No `require` block additions are needed.

---

## Recommended Stack Changes

### 1. Go Module Minimum Version — MANDATORY UPDATE

| Item | Current | Recommended |
|------|---------|-------------|
| `go` directive | `go 1.17` | `go 1.21.0` |
| `toolchain` directive | absent | `toolchain go1.26.0` |

**Why `go 1.21.0`:**
- Go 1.21 changed the `go` directive from advisory to a mandatory minimum. Before 1.21 it was mostly unenforced; from 1.21+ the toolchain refuses if the consumer's Go is older than declared.
- The codebase already has a comment flagging this. In `pkg/cluster/internal/create/create.go` line 258: `// NOTE: explicit seeding is required while the module minimum is Go 1.17. Once the minimum Go version is bumped to 1.20+, this can be simplified...` — bumping to 1.21.0 allows replacing `rand.New(rand.NewSource(time.Now().UTC().UnixNano()))` with just `rand.Intn()` (global source auto-seeded since 1.20).
- 1.21.0 also unlocks `slices` and `maps` standard packages for cleaner provider deduplication helpers.
- The system already builds with go1.26.0 (per `.go-version` = `1.25.7`, confirmed host = `go1.26.0`). This is a paperwork alignment, not a toolchain change.

**Why `toolchain go1.26.0`:**
- `.go-version` already pins `1.25.7`. Adding the `toolchain` directive in go.mod makes the intended build toolchain explicit and reproducible. Unlike the `go` directive, `toolchain` does not impose a requirement on consumers of this module — it only affects developers working in this module.

**go.mod diff:**

```diff
-go 1.17
+go 1.21.0
+
+toolchain go1.26.0
```

No changes to the `require` block.

**Confidence:** HIGH — verified against official Go toolchain docs and the codebase comment.

---

### 2. Local Registry Addon — No New Dependencies

**Runtime component (not a Go library):**

| Component | What | Version |
|-----------|------|---------|
| Registry container image | `registry:2` (Docker Hub official) | v2.8.x series |

**Why `registry:2` and not `registry:3`:**

As of 2026-03-03, Docker Hub shows `registry:3` / `registry:latest` point to v3.0.0 (released April 2025, first stable v3). However, `registry:2` remains pinned to the v2.8.x series and is the image the entire kind ecosystem (official kind docs, tilt-dev, openfaas, community guides) universally uses. v3.0.0 deprecated several storage drivers (the release notes call these "deprecations of certain storage drivers and dependency replacements"). The risk of migrating to v3 mid-project is not justified by any feature gain for a local dev registry. Revisit when kind upstream migrates.

**Go implementation — uses only existing packages:**

| Mechanism | Existing package used |
|-----------|----------------------|
| Start registry container | `exec.Command("docker"/"podman"/"nerdctl", "run", ...)` — existing exec pattern |
| Connect to kind network | `exec.Command(..., "network", "connect", "kind", "kind-registry")` |
| Write hosts.toml per node | `node.Command("mkdir", "-p", ...)` + `node.Command("tee", ...)` — existing node exec pattern |
| Apply ConfigMap | `node.Command("kubectl", "apply", "-f", "-")` with embedded YAML — same as metallb/envoygw |

**Containerd config patch (kind v0.27.0+ pattern):**

Applied via the existing `ContainerdConfigPatches` field in the v1alpha4 config — no new config fields. The patch enables the registry config directory:

```toml
[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "/etc/containerd/certs.d"
```

Then per node, write `/etc/containerd/certs.d/localhost:5001/hosts.toml`:

```toml
[host."http://kind-registry:5000"]
```

This alias bridges host `localhost:5001` to the `kind-registry` container name inside the node's network namespace — the reason the registry must be connected to the `kind` Docker/Podman/Nerdctl network.

**New API field:**

```go
// pkg/apis/config/v1alpha4/types.go — Addons struct
Registry *bool `yaml:"registry,omitempty" json:"registry,omitempty"`
```

Same `*bool` pattern as existing five addon fields. Default: true (enabled).

**New package:** `pkg/cluster/internal/create/actions/installregistry/registry.go`

The action runs: start container → connect to network → patch containerd config per node → apply optional ConfigMap. Follows the `installmetallb` action pattern exactly.

**Confidence:** HIGH — verified against official kind docs at https://kind.sigs.k8s.io/docs/user/local-registry/

---

### 3. cert-manager Addon — No New Dependencies

| Component | Version | Source |
|-----------|---------|--------|
| cert-manager manifest | v1.17.6 | `https://github.com/cert-manager/cert-manager/releases/download/v1.17.6/cert-manager.yaml` |

**Why v1.17.6:**

| Release | Kubernetes Compatibility | Status (2026-03-03) | Notes |
|---------|--------------------------|---------------------|-------|
| v1.19.4 | k8s 1.31–1.35 | Latest stable | Too new — may break users on older k8s node images |
| v1.18.x | k8s 1.29–1.33 | Supported | Reasonable but still narrows k8s range |
| v1.17.6 | k8s 1.28–1.31 | Supported (EOL when v1.19 ships) | Best balance — overlaps the widest range of kind node images |

The default kind node image typically targets the previous-stable k8s release. Locking to v1.19.x would mean users pulling the default kind node image (likely k8s 1.30 or 1.31) might hit compatibility issues. v1.17.6 is the safest conservative choice. Bump to v1.19.x when the project pins to k8s 1.31+ node images.

**cert-manager-specific wait requirement:**

The webhook deployment must be `Available` before any `Certificate` or `Issuer` CRs can be applied, otherwise the Kubernetes admission webhook rejects them. The install action must include:

```bash
kubectl wait --namespace=cert-manager \
  --for=condition=Available deployment/cert-manager-webhook \
  --timeout=120s
```

This is more stringent than the MetalLB controller wait — both the cert-manager controller AND the webhook must be ready. Failure to wait causes confusing admission rejection errors.

**Go implementation — uses only existing packages:**

| Mechanism | Existing package used |
|-----------|----------------------|
| Embed manifest | `//go:embed manifests/cert-manager.yaml` — same as metallb |
| Apply manifest | `node.Command("kubectl", "apply", "-f", "-")` with embedded YAML stdin |
| Wait for readiness | `node.Command("kubectl", "wait", "--for=condition=Available", ...)` |

**New API field:**

```go
// pkg/apis/config/v1alpha4/types.go — Addons struct
CertManager *bool `yaml:"certManager,omitempty" json:"certManager,omitempty"`
```

Default: true (enabled). Note: cert-manager is relatively heavyweight (~50MB manifest, ~3 CRD-heavy). Consider defaulting to `false` (disabled by default, opt-in) to keep cluster creation fast for users who don't need TLS certificate management. This is a design decision for the roadmap phase, not a stack constraint.

**New package:** `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go`

**Addon ordering constraint:** cert-manager installs after `waitforready` and before any future addon that issues `Certificate` resources. For this milestone, no existing addon depends on cert-manager, so it can append to the current addon sequence.

**Confidence:** MEDIUM — cert-manager version verified via https://cert-manager.io/docs/releases/; Kubernetes compatibility range is research-confirmed but the actual default node image k8s version should be verified before shipping.

---

### 4. `kinder env` Command — No New Dependencies

| Component | What |
|-----------|------|
| Package | `pkg/cmd/kind/env/env.go` (new) |
| Framework | Cobra — already a dependency |
| Wire-up | `root.go`: `cmd.AddCommand(env.NewCommand(logger, streams))` |

**What `env` outputs:**

```
KUBECONFIG=/home/user/.kube/config
KIND_CLUSTER_NAME=kind
KIND_PROVIDER=docker
```

With `--shell` flag: prefixes `export ` to each line (bash/zsh compatible). This matches the established pattern of `eval $(kinder env)` for shell integration.

**Implementation:** Uses only packages already imported in the codebase:
- `p.ListClusters()` — existing `Provider` interface method
- `kubeconfig.PathForCluster(name)` — existing package
- `os/exec.LookPath()` — standard library (already used in `nerdctl/util.go`)
- `fmt.Fprintf(streams.Out, ...)` — standard library

**Confidence:** MEDIUM — pattern is clear from codebase conventions; exact output fields are a design decision.

---

### 5. `kinder doctor` Command — No New Dependencies

| Component | What |
|-----------|------|
| Package | `pkg/cmd/kind/doctor/doctor.go` (new) |
| Framework | Cobra — already a dependency |
| Wire-up | `root.go`: `cmd.AddCommand(doctor.NewCommand(logger, streams))` |

**What `doctor` checks:**

| Check | Source | Method |
|-------|--------|--------|
| Container runtime available | `os/exec.LookPath("docker"/"podman"/"nerdctl")` | Standard library |
| Provider info (cgroup2, rootless, memory limit) | `p.Info()` → `providers.ProviderInfo` struct | Existing Provider interface |
| Active clusters reachable | `p.ListClusters()` | Existing Provider interface |
| kubectl available | `os/exec.LookPath("kubectl")` | Standard library |
| kubeconfig exists | `os.Stat(kubeconfig.PathForCluster(name))` | Standard library |

The `providers.ProviderInfo` struct already contains all health data doctor needs: `Rootless`, `Cgroup2`, `SupportsMemoryLimit`, `SupportsPidsLimit`, `SupportsCPUShares`.

**Output format:**

```
[OK]   Docker available (version 27.x)
[OK]   Cgroup v2 enabled
[WARN] Running rootless — MetalLB L2 speaker limited
[OK]   Cluster "kind" reachable (3 nodes)
[FAIL] kubectl not found in PATH
```

**Confidence:** MEDIUM — implementation is straightforward; exact check list is a design decision for the roadmap phase.

---

### 6. Provider Code Deduplication — No New Dependencies

**What duplicates exist** (confirmed by direct source file reading):

| Duplicated item | docker | nerdctl | podman | Action |
|-----------------|--------|---------|--------|--------|
| `clusterLabelKey` constant | yes | yes | yes | Move to `common/constants.go` |
| `nodeRoleLabelKey` constant | yes | yes | yes | Move to `common/constants.go` |
| `dockerInfo` struct | yes | yes | no (uses `podmanInfo`) | Move docker+nerdctl shared struct to `common/info.go` |
| `info()` function body (docker+nerdctl) | identical except `binaryName` param | identical | different | Extract to `common/info.go` with `binaryName string` param |
| `mountFuse()` function | identical | identical | absent | Move to `common/mount.go` |
| `planCreation` preamble (node naming + LB setup) | ~85% identical | ~80% identical | ~80% identical | Extract shared helpers to `common/provision.go` |

**What stays provider-specific (do not consolidate):**

- `podman/info.go` — uses entirely different `podmanInfo` JSON structure and version-gates cgroup detection
- `nerdctl` binary dispatch — the `binaryName string` parameter pattern is nerdctl-specific (also handles `finch`)
- `podman/DeleteNodes` — extra volume cleanup step not present in docker/nerdctl
- Network functions — each provider has distinct subnet detection and network creation logic
- `podman/GetAPIServerEndpoint` — handles three different podman version behaviors; unique to podman

**New files in `pkg/cluster/internal/providers/common/`:**

```
common/
  constants.go    # clusterLabelKey, nodeRoleLabelKey (currently 3 identical copies)
  info.go         # shared dockerInfo struct + info(binaryName string) for docker+nerdctl
  mount.go        # shared mountFuse(binaryName string, infoFn func() (*ProviderInfo, error))
```

**No interface changes.** `providers.Provider` in `pkg/cluster/internal/providers/provider.go` is unchanged. All changes are internal to provider packages.

**Confidence:** HIGH — directly observed from reading all three provider source files.

---

## What NOT to Add

| Avoid | Why |
|-------|-----|
| `helm/helm` Go library | No Helm runtime needed — cert-manager install uses the static `cert-manager.yaml` manifest, same pattern as MetalLB |
| `k8s.io/client-go` | No in-process Kubernetes API calls — all kubectl operations use the `node.Command("kubectl", ...)` pattern (runs kubectl inside the kind node container) |
| `github.com/docker/docker` SDK | Docker/podman/nerdctl are invoked as external binaries via `exec.Command` — this is intentional in the kind architecture and must not change |
| `viper` configuration | `kinder env` output is simple key=value strings; Viper is overkill and would add an unwanted dependency |
| Any new test framework | Use existing `testing` standard library patterns already present in the codebase |

---

## Integration Points Summary

```
go.mod
  go 1.21.0                              (was 1.17)
  toolchain go1.26.0                     (new)

pkg/apis/config/v1alpha4/types.go
  Addons.Registry   *bool               (new field)
  Addons.CertManager *bool              (new field)

pkg/cluster/internal/create/create.go
  runAddon("Registry", ...)             (after waitforready, before MetalLB)
  runAddon("cert-manager", ...)         (after Registry)

pkg/cmd/kind/root.go
  cmd.AddCommand(env.NewCommand(...))   (new)
  cmd.AddCommand(doctor.NewCommand(...)) (new)

pkg/cluster/internal/providers/common/
  constants.go                           (new — moves 3 identical constant definitions)
  info.go                                (new — moves docker+nerdctl shared info code)
  mount.go                               (new — moves docker+nerdctl shared mountFuse)

New packages (all follow existing installmetallb pattern):
  pkg/cluster/internal/create/actions/installregistry/
  pkg/cluster/internal/create/actions/installcertmanager/
  pkg/cmd/kind/env/
  pkg/cmd/kind/doctor/
```

---

## Sources

- Go toolchain directive semantics: https://go.dev/doc/toolchain (HIGH confidence)
- Go 1.21 go directive change: https://tip.golang.org/doc/go1.21 (HIGH confidence)
- kind local registry official docs: https://kind.sigs.k8s.io/docs/user/local-registry/ (HIGH confidence)
- Docker Hub registry tags (registry:2 = v2.8.x, registry:3 = v3.0.0): https://hub.docker.com/_/registry/tags (MEDIUM confidence — confirmed via search; page CSS-blocked on direct fetch)
- distribution/distribution v3.0.0: https://github.com/distribution/distribution/releases (HIGH confidence)
- cert-manager supported releases + k8s compatibility: https://cert-manager.io/docs/releases/ (HIGH confidence)
- cert-manager v1.17.6 released 2025-12-17: search results from cert-manager release index (MEDIUM confidence)
- Provider duplication analysis: direct source file reading of docker/nerdctl/podman packages in this repo (HIGH confidence)
