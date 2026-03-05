---
title: Changelog
description: All releases and changes since kinder was forked from kind.
---

All notable changes to kinder since forking from [kind](https://kind.sigs.k8s.io/) at commit `89ff06bd`.

:::note[Version scheme change]
Starting with v1.2, kinder uses its own version sequence (`v1.0`, `v1.1`, `v1.2`, ...) independent of upstream kind's `v0.x` series. Earlier releases used `v0.x.y-alpha` tags inherited from the fork. The Go binary reports the tag version via `kinder version`.
:::

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
