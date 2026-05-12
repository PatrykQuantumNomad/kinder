---
title: Changelog
description: All kinder releases since the fork from kind — version history, new addons, CLI verbs, doctor checks, and breaking changes by release.
---

All notable changes to kinder since forking from [kind](https://kind.sigs.k8s.io/) at commit `89ff06bd`.

:::note[Version scheme change]
Starting with v1.2, kinder uses its own version sequence (`v1.0`, `v1.1`, `v1.2`, ...) independent of upstream kind's `v0.x` series. Earlier releases used `v0.x.y-alpha` tags inherited from the fork. The Go binary reports the tag version via `kinder version`.
:::

---

## v2.4 — Hardening (in progress)

- **`local-path-provisioner` bumped to v0.0.36** — closes [GHSA-7fxv-8wr2-mfc4](https://github.com/rancher/local-path-provisioner/security/advisories/GHSA-7fxv-8wr2-mfc4) HelperPod Template Injection security advisory. Embedded `busybox:1.37.0` pin and `is-default-class` StorageClass annotation preserved. (ADDON-01)
- **`Headlamp` bumped to v0.42.0** — token-print authentication flow re-verified live (`kubectl auth can-i` + UI curl with the printed SA token both succeed). Existing kinder-specific Secret + `-in-cluster` deployment arg pattern preserved. (ADDON-02)
- **`cert-manager` bumped to v1.20.2** — `--server-side` apply preserved (manifest is 989 KB, exceeds 256 KB annotation limit). Live UAT verified self-signed ClusterIssuer issues a Certificate and pods run as UID `65532` (via distroless image `USER nonroot` directive; kubelet enforces `runAsNonRoot: true`). **Breaking changes:** container UID changed from `1000` to `65532` (Secret/PVC ownership impact); `Certificate.spec.privateKey.rotationPolicy: Always` is GA-mandatory (set `Never` explicitly to keep old behavior). See addon doc for details. (ADDON-03)
- **`Envoy Gateway` bumped to v1.7.2** (single-jump from v1.3.1). Bundled Gateway API CRDs upgrade from `v1.2.1` to `v1.4.1` in-band. Live HTTPRoute end-to-end UAT verified traffic returns 200 through the gateway (in-cluster curl via `kubectl run uat-curl`). `eg-gateway-helm-certgen` Job name unchanged (verified in upstream install.yaml — Pitfall EG-02 cleared). Ratelimit image bumped from `ae4cee11` to `05c08d03`. (ADDON-04)

---

## v1.5 — Inner Loop

**Released:** May 7, 2026

Daily iteration on a kinder cluster is now as fast as creating one. Five capabilities make the inner-loop tight: pause/resume to reclaim laptop CPU and RAM without losing state, snapshot/restore for instant clean-state reset, `kinder dev` for hot-reload from a watched directory, `kinder doctor decode` for plain-English explanations of cryptic runtime errors, and an upstream sync that adopts kind's HAProxy→Envoy load balancer transition. One new module dependency (`fsnotify` v1.10.1) — the first since v1.2.

### Cluster Pause/Resume

- **`kinder pause [cluster-name]`** — gracefully stops every node container in quorum-safe order (workers → control-plane → load balancer) so the host can reclaim CPU and RAM. Cluster state survives the pause; pods, PVCs, services, and node identities are intact on resume
- **`kinder resume [cluster-name]`** — restarts containers in the reverse order (LB → CP → workers) and gates on all-nodes-Ready via kubectl with K8s 1.24 selector fallback
- **`--timeout`/`--wait` accept duration strings** — flags use `cobra.DurationVar`, so `5m`, `30s`, `2m` parse cleanly. Bare integers are intentionally rejected
- **`cluster-resume-readiness` doctor check** — runs before resume on HA clusters and warns when etcd quorum is at risk. Probes etcd health via `crictl exec <etcd-id> etcdctl endpoint health` (etcdctl ships only inside the etcd static-pod container)
- **`kinder status [cluster-name]`** — new command surfacing container-runtime state plus a Status column on `kinder get clusters` (JSON schema migrated to `[]{name, status}`) and real container state on `kinder get nodes`
- **HA pre-pause etcd snapshot** — captures `/kind/pause-snapshot.json` with leader ID before pause, so resume can detect quorum risk

### Cluster Snapshot/Restore

- **`kinder snapshot create [snap-name]`** — captures etcd state, all loaded container images, and local-path-provisioner PV contents into a single tar.gz bundle with a sha256 sidecar for integrity verification. Stored under `~/.kinder/snapshots/<cluster>/` with mode 0700
- **`kinder snapshot restore [snap-name]`** — full pre-flight gauntlet (sha256 + disk space + K8s/topology/addon hard-fail compatibility checks) runs BEFORE any cluster mutation. HA-safe etcd restore uses a shared `--initial-cluster-token` with manifest-aside + atomic data-dir swap. Image re-import via the existing `LoadImageArchiveWithFallback` path
- **No auto-rollback** — post-pause failures emit a recovery-hint error pointing the user to `kinder resume`. Restore has no `--yes` flag (intentional: hard overwrite signals destructiveness)
- **`kinder snapshot list/show/prune`** — list shows NAME/AGE/SIZE/K8S/ADDONS/STATUS columns; show prints size, age, K8s version, addon versions, image-bundle digest. Prune refuses no-flag invocation and prompts y/N unless `--yes` is given. STATUS=corrupt detection via sidecar re-hash
- **Air-gap reproducible metadata** — every snapshot records cluster K8s version, addon versions, and image-bundle digest

### Inner-Loop Hot Reload

- **`kinder dev --watch <dir> --target <deployment>`** — enters watch mode; saving a file in the watched directory triggers a build → load → rollout cycle automatically. Per-cycle timing printed in `%.1fs` format (build / load / rollout / total)
- **fsnotify recursive watcher** — `fsnotify` v1.10.1 added (first new module dep since v1.2). Synthesises a trigger event on `ErrEventOverflow` so heavy builds writing thousands of files never silently drop the rebuild
- **`--poll` mode for Docker Desktop on macOS** — switches to a stdlib polling watcher when fsnotify events are unreliable (the macOS volume-mount case). `--poll-interval` configurable
- **Leading-trigger debouncer** — first event in a window arms the timer and fires immediately; subsequent events within `--debounce` (default 500ms) are absorbed. Build starts ASAP; doesn't wait for editor swap-rename to finish
- **Reuses `kinder load images` core** — `LoadImagesIntoCluster` calls `nodeutils.LoadImageArchiveWithFallback` directly via public APIs rather than importing `pkg/cmd/kind/load`
- **Host kubectl rollout** — `kubectl rollout restart` runs on the host with `--kubeconfig=<external>` so user Deployments are managed in the user's existing kubectl context. Concurrent-cycle prevention + `signal.NotifyContext` SIGINT/SIGTERM teardown

### Runtime Error Decoder

- **`kinder doctor decode`** — scans recent docker logs and `kubectl get events` and matches lines against a 16-pattern catalog covering kubelet, kubeadm, containerd, docker, and addon-startup failures (KUB-01..05, KADM-01..03, CTD-01..03, DOCK-01..03, ADDON-01..02). Bare `kinder doctor` is unchanged; decode is an additive sibling subcommand
- **Plain-English output** — every match shows the pattern ID, plain-English explanation, suggested fix, and a doc/issue link where applicable. Both `--output=human` (default) and `--output=json` carry all four fields per match
- **`--auto-fix` whitelist** — only three SafeMitigation factories are allowed (inotify-raise sysctl, coredns rollout restart, node container restart). Preview-before-apply enforced; `NeedsFix` precondition + `NeedsRoot` guard skip cleanly when conditions don't apply
- **`--since` and `--include-normal`** — single duration applied to both docker logs and kubectl events; default filter is `type!=Normal` (Warnings only), `--include-normal` flips it
- **First-match-wins matcher** — `sync.Map` regex cache keyed by pattern string; each unique regex compiles once across process lifetime. Catalog-coverage integration test guards against orphan/stale fixtures

### Upstream Sync & K8s 1.36

- **HAProxy → Envoy load balancer** — adopts kind PR #4127. HA clusters now use `docker.io/envoyproxy/envoy:v1.36.2` instead of `kindest/haproxy`. LDS+CDS atomic file swap via `chmod && mv && mv` (no SIGHUP needed; Envoy xDS polling picks up swapped files). Wired across docker, podman, and nerdctl providers
- **`kinder delete cluster <name>`** — accepts a positional cluster name argument (previously only `--name` worked, with cobra.NoArgs rejecting positional input). Positional takes precedence over `--name`; consistent with pause/resume/status/get-nodes
- **IPVS-on-1.36+ guard** — `kubeProxyMode: ipvs` is rejected at config validation time when the node K8s version is 1.36 or higher, with a clear error message ("deprecated, will be removed in a future release") and a link to the iptables migration path
- **K8s 1.36 recipe page** — new [What's new in K8s 1.36](/guides/k8s-1-36-whats-new/) guide on the kinder website demonstrating User Namespaces (GA) and In-Place Pod Resize (GA) on a kinder cluster
- **Default node image bump deferred** — `kindest/node:v1.36.x` is not yet published on Docker Hub (probe 2026-05-07). The default remains `kindest/node:v1.35.1`. The bump will land as a follow-up release once kind v0.32.0 publishes the v1.36 image

---

## v1.4 — Cluster Capabilities

**Released:** April 10, 2026

Four cluster capabilities that fill the gap between plain kind and a production-like local environment: multi-version per-node Kubernetes, offline/air-gapped cluster creation, local-path-provisioner dynamic storage, and host-directory mounting — plus a provider-abstracted `kinder load images` utility that ties the offline and multi-version workflows together. Zero new Go module dependencies.

### Multi-Version Node Validation

- **Per-node image preservation** — `--image` flag no longer overrides explicit per-node `image:` entries. Explicit image assignments capture pre-defaults via `ExplicitImage` sentinel in `encoding/convert.go`
- **Version-skew validation** — config parse rejects workers more than 3 minor versions behind the control-plane before any containers are created, with a precise error message stating the violating node and version delta
- **HA consistency check** — control-plane nodes at different versions are rejected at config validation time
- **Doctor cluster-skew check** — `kinder doctor` reports a warning when a running multi-version cluster violates version-skew policy; injectable node reader enables test coverage without a live runtime
- **New `kinder get nodes` columns** — output now includes `VERSION`, `IMAGE`, and `SKEW` columns sourced via container `inspect` (avoids import cycle with `pkg/cluster`)
- Non-semver image tags (e.g. `latest`) skip version-skew validation to preserve backward compatibility with test/dev configs

### Air-Gapped Cluster Creation

- **`--air-gapped` flag** — new flag on `kinder create cluster` disables all network calls for image pulls across docker, podman, and nerdctl providers
- **Fast-fail with actionable error** — missing images produce a complete list of what must be pre-loaded, instead of timing out or hanging on failed pulls. Per-runtime pre-load instructions via `formatMissingImagesError(binaryName)`
- **Addon image warning** — non-air-gapped creation prints a NOTE listing every addon image that will be pulled, so users know what to pre-load before switching to offline mode
- **Doctor offline-readiness check** — `kinder doctor` lists which required images are absent from the local image store, serving as a pre-flight offline readiness check. Skips gracefully when no container runtime is found
- **`RequiredAddonImages` utility** — centralised image inventory imported from addon packages (no import cycle, since addon packages don't import common)
- **Working Offline guide** — new [working-offline guide](/guides/working-offline/) documenting the two-mode offline workflow: pre-create image baking vs. post-create `kinder load images`

### Local-Path-Provisioner Addon

- **New default addon** — [local-path-provisioner](/addons/local-path-provisioner/) v0.0.35 installed by default; `local-path` is the only default StorageClass (the legacy `standard` StorageClass from `installstorage` is gated off)
- **Automatic dynamic PVC provisioning** — `PersistentVolumeClaim` resources transition to `Bound` automatically in both single-node and multi-node clusters without manual operator action
- **Opt-out config** — `addons.localPath: false` in the cluster config skips the addon and restores the legacy `standard` StorageClass (exact pre-v1.4 behavior)
- **CVE-2025-62878 doctor check** — `kinder doctor` warns when local-path-provisioner is below v0.0.34 (the fix version; strictly less-than triggers warn)
- **Air-gapped compatible** — embedded manifest pins `busybox:1.37.0` with `imagePullPolicy: IfNotPresent`, ensuring PVC operations work in air-gapped clusters where `busybox:latest` cannot be pulled
- Uses `boolVal` (opt-out, default true) consistent with MetalLB/CertManager pattern

### Host-Directory Mounting

- **Pre-flight path validation** — `extraMounts` entries with non-existent host paths are rejected before any containers are created, with an error message identifying the missing path. Relative paths resolved via `filepath.Abs` before `os.Stat`
- **Platform propagation warnings** — `propagation: HostToContainer` or `Bidirectional` on macOS or Windows now emits a visible warning explaining that propagation is unsupported on Docker Desktop and defaults to `None`
- **Doctor host-mount check** — on macOS, `kinder doctor` verifies that configured host mount paths exist and that Docker Desktop file sharing is enabled for that path, reporting actionable guidance when either check fails. Falls back to Docker Desktop default dirs when `settings-store.json` is absent
- **`--config` flag on doctor** — `kinder doctor --config cluster.yaml` extracts `extraMounts` paths from the config and wires them into mount checks; `mountPathConfigurable` interface allows per-check mount path injection
- **Host Directory Mounting guide** — new [guide](/guides/host-directory-mounting/) walks through the two-hop mount pattern (host directory → node `extraMount` → pod `hostPath` PV) with complete YAML examples

### `kinder load images` Command

- **New `kinder load images <image> [<image>...]` subcommand** — loads one or more local images into every node of a running cluster with a single command
- **Provider-abstracted** — `providerBinaryName()` resolves the actual binary for docker, podman, nerdctl, finch, and nerdctl.lima. `save()` and `imageID()` take `binaryName` as a parameter instead of hardcoding `docker save`
- **Docker Desktop 27+ containerd fallback** — `LoadImageArchiveWithFallback` in `nodeutils` detects the "content digest: not found" error from `ctr images import --all-platforms` and retries without `--all-platforms`. Factory pattern (`openArchive func() (io.ReadCloser, error)`) provides a fresh reader for the retry since tar streams cannot be rewound
- **Smart-load skip** — re-running with an image already present on all nodes completes without re-importing and logs `"Image ... found to be already present on all nodes."`
- **Load Images CLI reference** — new [CLI reference page](/cli-reference/load-images/) with per-provider examples, smart-load behavior, and the Docker Desktop 27+ compatibility note

### Website

- **[Local Path Provisioner](/addons/local-path-provisioner/)** addon page documenting config, verification, CVE check, and air-gapped image list
- **[Working Offline](/guides/working-offline/)** guide with pre-load and post-create workflows
- **[Host Directory Mounting](/guides/host-directory-mounting/)** tutorial with two-hop mount walkthrough
- **[Load Images](/cli-reference/load-images/)** CLI reference covering all three providers, smart-load behavior, and Docker Desktop 27+ fallback
- Landing page updated with local-path in Core Addons
- Configuration reference gained the `localPath` addon field

### Internal

- `ExplicitImage` captured pre-defaults in `encoding/convert.go` (SetDefaultsCluster fills empty Image fields before `Convertv1alpha4`, making post-defaults detection impossible)
- `stderrors` alias for stdlib `errors` avoids conflict with `sigs.k8s.io/kind/pkg/errors` import
- `isContentDigestError` checks `RunError.Output` before falling back to `err.Error()` string
- `LoadImageArchive` (existing public API) unchanged — new `LoadImageArchiveWithFallback` coexists for `kinder load images` usage
- `realListNodes` / `realGetProvisionerVersion` / `realListNodes` in doctor checks use the low-level container `exec` CLI approach to avoid import cycles with `pkg/cluster/internal`

---

## v1.3 — Known Issues & Proactive Diagnostics

**Released:** March 6, 2026

Expanded `kinder doctor` from 3 to 18 diagnostic checks across 8 categories, wired automatic mitigations into `kinder create cluster`, and added a comprehensive Known Issues documentation page.

### Doctor Infrastructure

- **Check interface** — unified `Check` contract with `Name()`, `Category()`, `Platforms()`, `Run()` methods. All checks return structured `Result` values with ok/warn/fail/skip status
- **Category-grouped output** — `kinder doctor` groups checks by category (Runtime, Docker, Tools, GPU, Kernel, Security, Platform, Network) with Unicode status icons
- **JSON output** — `kinder doctor --output json` produces an envelope with checks array and summary object (total/ok/warn/fail/skip counts)
- **Platform filtering** — checks declare target platforms; non-matching platforms get `skip` status instead of crashing
- **SafeMitigation system** — tier-based mitigation infrastructure wired into `kinder create cluster` before provisioning. Errors logged as warnings, never fatal

### Docker & Tool Checks

- **Disk space** — warns at <5GB, fails at <2GB using Docker's data root path. Build-tagged `statfs` for Linux/macOS
- **daemon.json init flag** — detects `"init": true` across 6 candidate paths (native Linux, Docker Desktop macOS, rootless, Snap, Rancher Desktop, Windows)
- **Docker snap** — detects Docker installed via snap through symlink resolution. Warns about `TMPDIR` issues
- **kubectl version skew** — parses `kubectl version --client -o json` and warns when skew exceeds +/-1 minor version from reference (v1.31)
- **Docker socket permissions** — detects permission denied on Linux and suggests `usermod -aG docker $USER` fix

### Kernel & Security Checks (Linux)

- **inotify limits** — warns when `max_user_watches` < 524288 or `max_user_instances` < 512 with exact `sysctl` fix commands
- **Kernel version** — fails on kernels below 4.6 (cgroup namespace support is a hard blocker for kind)
- **AppArmor** — detects enabled AppArmor and warns about stale profile interference (`moby/moby#7512`)
- **SELinux** — detects enforcing mode on Fedora and warns about `/dev/dma_heap` denials
- **firewalld** — detects nftables backend (Fedora 32+ default) and warns about Docker networking issues

### Platform Checks

- **WSL2** — multi-signal detection (microsoft in `/proc/version` + `WSL_DISTRO_NAME` or `WSLInterop`) prevents Azure VM false positives. Checks cgroup v2 controllers (cpu, memory, pids)
- **Rootfs device** — detects BTRFS as Docker storage driver or backing filesystem
- **Subnet clash** — detects Docker network subnet overlaps with host routing table using `netip.Prefix.Overlaps`. Handles macOS abbreviated CIDR notation

### Create-Flow Integration

- `kinder create cluster` calls `ApplySafeMitigations()` after containerd config patches and before provisioning
- Only tier-1 mitigations applied (env vars, cluster config adjustments) — never calls `sudo` or modifies system files
- Mitigation errors are informational warnings, never block cluster creation

### Website

- **[Known Issues](/known-issues/)** page documenting all 18 diagnostic checks across 8 categories with What/Why/Platforms/Fix structure
- Known Issues added to sidebar navigation
- Cross-linked from [Troubleshooting](/cli-reference/troubleshooting/) page

### Internal

- `golang.org/x/sys/unix` promoted from indirect to direct dependency for `unix.Statfs` and `unix.Uname`
- Deps struct injection pattern for all checks: injectable `readFile`, `execCmd`, `lookPath` for unit testing without system calls
- Build-tagged platform pairs: `kernel_linux.go`/`kernel_other.go`, `disk_unix.go`/`disk_other.go`
- 80+ new unit tests across 10 check files with table-driven parallel execution

---

## v1.2 — Distribution & GPU Support

**Released:** March 5, 2026

First stable release with automated binary distribution via GoReleaser, Homebrew tap, and NVIDIA GPU addon.

### Distribution

- **GoReleaser pipeline** — automated cross-platform binary builds for linux/darwin (amd64 + arm64) and windows (amd64) with SHA-256 checksums and categorized changelog
- **GitHub Releases** — tagged releases automatically publish platform archives to [GitHub Releases](https://github.com/PatrykQuantumNomad/kinder/releases)
- **Homebrew tap** — `brew install patrykquantumnomad/kinder/kinder` installs a pre-built binary on macOS. Cask auto-published on each stable release via GoReleaser
- **goreleaser-action** — replaces legacy `cross.sh` + `softprops` release workflow; `cross.sh` retired

### NVIDIA GPU Addon

- **NVIDIA device plugin** (v0.17.0) — DaemonSet installed via go:embed + kubectl apply when `addons.nvidiaGPU: true`. RuntimeClass `nvidia` created for GPU pod scheduling
- **Opt-in config** — `NvidiaGPU *bool` field in v1alpha4 config API, defaults to `false` (unlike other addons which default to `true`)
- **Platform guard** — GPU addon skips with informational message on non-Linux platforms without failing cluster creation
- **Pre-flight validation** — checks for `nvidia-smi`, `nvidia-ctk`, and nvidia runtime in Docker config before applying manifests. Fails fast with actionable error messages
- **Doctor checks** — `kinder doctor` reports NVIDIA driver version, container toolkit presence, and Docker runtime configuration (Linux only, warn-not-fail)
- **Documentation** — GPU addon page at [kinder.patrykgolabek.dev/addons/nvidia-gpu](https://kinder.patrykgolabek.dev/addons/nvidia-gpu/) with prerequisites, configuration, usage, and troubleshooting

### Website

- Installation page updated with Homebrew install instructions and GitHub Releases download links

### Internal

- `project_name: kinder` and `gomod.proxy: false` in GoReleaser config for fork safety
- `skip_upload: auto` on Homebrew cask to prevent publishing pre-release builds
- `HOMEBREW_TAP_TOKEN` fine-grained PAT scoped to `homebrew-kinder` repo for cross-repo cask push

---

## v0.4.1-alpha — Website Use Cases & Documentation

**Released:** March 4, 2026

Expanded the documentation site with 3 tutorials, 3 CLI reference pages, and enriched all 7 addon pages with examples, troubleshooting, and configuration details.

### Tutorials

- **[TLS Web App](https://kinder.patrykgolabek.dev/guides/tls-web-app/)** — deploy a web app with TLS termination using cert-manager + Envoy Gateway
- **[HPA Auto-Scaling](https://kinder.patrykgolabek.dev/guides/hpa-auto-scaling/)** — set up Horizontal Pod Autoscaler with Metrics Server and load-test it
- **[Local Dev Workflow](https://kinder.patrykgolabek.dev/guides/local-dev-workflow/)** — build, push to local registry, and deploy with hot-reload iteration

### CLI Reference

- **[Profile Comparison](https://kinder.patrykgolabek.dev/reference/profile-comparison/)** — side-by-side table of all 4 addon profiles (minimal, full, gateway, ci)
- **[JSON Output](https://kinder.patrykgolabek.dev/reference/json-output/)** — schema reference for `--output json` on env, doctor, get clusters, get nodes
- **[Troubleshooting](https://kinder.patrykgolabek.dev/reference/troubleshooting/)** — common issues with `kinder env` and `kinder doctor`, exit codes

### Addon Page Enrichment

- All 7 addon pages updated with: configuration examples, version pinning details, symptom/cause/fix troubleshooting tables, and verification commands
- Core vs optional addon grouping on landing page and configuration reference
- Quick-start guide updated with all 7 addon verifications and `--profile` tip

---

## v0.4.0-alpha — Code Quality & Features

**Released:** March 4, 2026

Modernized the Go toolchain, added context.Context cancellation plumbing, built a comprehensive unit test suite, implemented wave-based parallel addon execution, and shipped JSON output and cluster profile presets for the CLI.

### Go Toolchain & Code Quality

- **Go 1.24 baseline** — go.mod bumped to 1.24.0, `golang.org/x/sys` updated to v0.41.0, `rand.NewSource` dead code cleaned up
- **golangci-lint v2** — migrated from v1.62.2 to v2.10.1 with full config conversion, 55+ lint violations fixed across 60+ files
- **Layer violation fix** — version package moved from `pkg/cmd/kind/version` to `pkg/internal/kindversion` to enforce clean `cmd -> cluster -> internal` import direction
- **SHA-256 subnet hashing** — SHA-1 replaced with SHA-256 for Docker/Podman/Nerdctl subnet generation
- **Code quality** — log directory permissions `0777` → `0755`, dashboard token at `V(1)`, error naming convention (`ErrNoNodeProviderDetected`)

### Architecture

- **context.Context plumbing** — `Context` field added to `ActionContext` and propagated through all 7 addon `Execute()` methods via `node.CommandContext()`. `waitForReady`/`tryUntil` are now cancellation-aware with `select` on `ctx.Done()`
- **Centralized addon registry** — 7 hard-coded `runAddon()` calls replaced with a data-driven `[]AddonEntry` registry loop in `create.go`

### Unit Tests

- **Test infrastructure** — shared `testutil` package with `FakeNode`, `FakeCmd`, and `FakeProvider` types for testing addon actions without a live cluster
- **Addon test coverage** — 30+ table-driven tests covering `installenvoygw`, `installmetricsserver`, `installcertmanager`, `installdashboard`, and `installlocalregistry`
- **Race-detector clean** — all tests pass under `go test -race`

### Parallel Addon Execution

- **Wave-based execution** — 6 independent addons run concurrently via `errgroup.WithContext` + `SetLimit(3)` in Wave 1; EnvoyGateway runs sequentially in Wave 2 (depends on MetalLB)
- **Race-free node caching** — `RWMutex`-based `cachedData` replaced with `sync.OnceValues` for exactly-once node caching, eliminating a TOCTOU race
- **Install timing** — per-addon install duration printed in the creation summary (e.g., "MetalLB: 12.3s")
- Added `golang.org/x/sync` dependency and `make test-race` Makefile target

### CLI Features

- **`--output json`** — added to `kinder env`, `kinder doctor`, `kinder get clusters`, and `kinder get nodes`. All produce clean, `jq`-parseable JSON on stdout; logger output redirected to stderr in JSON mode
- **`--profile` flag** — `kinder create cluster --profile <name>` selects a named addon preset:
  - `minimal` — no kinder addons (core kind only)
  - `full` — all addons enabled
  - `gateway` — MetalLB + Envoy Gateway only
  - `ci` — Metrics Server + cert-manager (CI-optimized)
- Default behavior (no `--profile`) is fully preserved

### Internal

- Added `golang.org/x/sync` v0.19.0 for `errgroup`
- `CreateWithAddonProfile` nil-guards `o.Config` by loading default config when no `--config` flag given
- `--profile` applied after `withConfig` so profile addons override config-file addon settings

---

## v0.3.0-alpha — Harden & Extend

**Released:** March 3, 2026

Fixed 4 correctness bugs, eliminated ~525 lines of provider code duplication, and added batteries-included local registry, cert-manager, and CLI diagnostic tools.

### Bug Fixes

- **Port leak fix** — port listeners in `generatePortMappings` are now released at loop iteration end, not deferred to function return, across docker/nerdctl/podman providers
- **Tar truncation fix** — `extractTarball` returns a descriptive error on truncated archives instead of silently succeeding
- **Cluster name fix** — `ListInternalNodes` wraps empty cluster names with `defaultName()` for consistent resolution across all providers
- **Network sort fix** — network sort comparator uses strict weak ordering with `!=` guard for deterministic results

### New Addons

- **Local Registry** (`localhost:5001`) — a `registry:2` container is created on the kind network during cluster creation. All nodes are patched with containerd `certs.d` configuration. A `kube-public/local-registry-hosting` ConfigMap is applied for Tilt/Skaffold/dev-tool discovery. Disable with `addons.localRegistry: false`
- **cert-manager** (v1.16.3) — embedded manifest applied via `--server-side`. All three components (controller, webhook, cainjector) reach Available status before the cluster is reported ready. A self-signed `ClusterIssuer` (`selfsigned-issuer`) is created automatically so `Certificate` resources work immediately. Disable with `addons.certManager: false`

### New Commands

- **`kinder env`** — prints `KINDER_PROVIDER`, `KIND_CLUSTER_NAME`, and `KUBECONFIG` in eval-safe `key=value` format. Warnings go to stderr. Use with `eval $(kinder env)` in shell scripts
- **`kinder doctor`** — checks binary prerequisites (docker/podman/nerdctl, kubectl) and prints actionable fix messages. Exit codes: `0` = all good, `1` = hard failure, `2` = warnings only

### Config API

- Added `LocalRegistry` and `CertManager` fields to the v1alpha4 `Addons` struct (both `*bool`, default `true`)
- Wired through all 5 config pipeline locations: types, defaults, deepcopy, conversion, validation

### Internal

- Extracted shared docker/podman/nerdctl logic to `common/` package (`common/node.go`, `common/provision.go`)
- Deleted per-provider `provision.go` files (~525 lines eliminated)
- Updated `go.mod` to `go 1.21.0` with `toolchain go1.26.0`
- Added `Provider.Name()` method via `fmt.Stringer` type assertion

---

## v0.2.0-alpha — Branding & Polish

**Released:** March 2, 2026

Established kinder's visual identity with a custom logo, SEO discoverability, documentation rewrite, and dark-only theme enforcement.

### Branding

- **Kinder logo** — modified kind robot with "er" in cyan, exported as SVG, PNG, `favicon.ico`, and OG image
- Original kind logo preserved in `logo/` directory
- Logo displayed in hero section of landing page

### SEO & Discoverability

- `llms.txt` and `llms-full.txt` for AI crawler discovery
- JSON-LD `SoftwareApplication` structured data
- Complete Open Graph and Twitter Card meta tags
- Author backlinks and attribution to [patrykgolabek.dev](https://patrykgolabek.dev)

### Documentation

- Root README rewritten from kind boilerplate to kinder identity
- `kinder-site/` README updated with project-specific documentation

### Design

- Dark-only theme enforced site-wide (light mode toggle removed)
- Terminal aesthetic with cyan accents as core visual identity

---

## v0.1.0-alpha — Kinder Website

**Released:** March 2, 2026

Launched the documentation website at [kinder.patrykgolabek.dev](https://kinder.patrykgolabek.dev) with dark terminal aesthetic, interactive landing page, and comprehensive documentation.

### Website

- Astro v5 + Starlight documentation site
- GitHub Actions deployment to GitHub Pages
- Custom domain: `kinder.patrykgolabek.dev` with HTTPS
- Dark terminal aesthetic (cyan accents, `hsl(185)`)

### Documentation Pages

- **Installation** — pre-built binary and build-from-source instructions
- **Quick Start** — create your first cluster walkthrough
- **Configuration** — v1alpha4 config reference with addon fields
- **MetalLB** — LoadBalancer addon documentation
- **Envoy Gateway** — Gateway API routing documentation
- **Metrics Server** — `kubectl top` and HPA documentation
- **CoreDNS** — DNS tuning documentation
- **Headlamp** — dashboard addon documentation

### Landing Page

- Hero section with feature overview
- Copy-to-clipboard install command
- Kind vs Kinder feature comparison grid
- Addon feature cards for all 5 default addons

### Quality

- Mobile responsive at 375px viewport
- Lighthouse 90+ on all metrics
- `robots.txt` and Pagefind search index
- Custom 404 page

---

## v0.0.1-alpha — Batteries Included

**Released:** March 1, 2026

Forked kind into kinder with 5 default addons that work out of the box. One command gives you a fully functional Kubernetes development environment.

### Core

- Binary renamed from `kind` to `kinder` (backward compatible)
- Config schema extended with `addons` section in v1alpha4
- Existing kind configs work unchanged
- Each addon individually disableable via `addons.<name>: false`
- All addons wait for readiness before the cluster is reported ready

### Default Addons

- **MetalLB** (v0.15.3) — auto-detects Docker/Podman/Nerdctl subnet and assigns LoadBalancer IPs without user input. Platform warning on macOS/Windows
- **Envoy Gateway** (v1.3.1) — Gateway API CRDs installed, HTTP routing via LoadBalancer IPs. Uses `--server-side` apply for large CRDs
- **Metrics Server** (v0.8.1) — `kubectl top nodes` and `kubectl top pods` work immediately. Configured with `--kubelet-insecure-tls` for local clusters
- **CoreDNS tuning** — in-place Corefile modification: `autopath`, `pods verified`, `cache 60`
- **Headlamp** (v0.40.1) — web dashboard with auto-generated admin token and printed port-forward command

### Architecture

- Addons implemented as creation actions (follows kind's `installcni`/`installstorage` pattern)
- All manifests embedded via `go:embed` (offline-capable)
- Runtime apply via `kubectl` (not baked into node image)
- `*bool` addon fields: `nil` defaults to `true`, explicit `false` disables
