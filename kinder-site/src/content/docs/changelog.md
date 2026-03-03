---
title: Changelog
description: All releases and changes since kinder was forked from kind.
---

All notable changes to kinder since forking from [kind](https://kind.sigs.k8s.io/) at commit `89ff06bd`.

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
