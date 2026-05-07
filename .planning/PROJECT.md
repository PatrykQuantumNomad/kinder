# Kinder

## What This Is

Kinder is a fork of kind (Kubernetes IN Docker) that provides a batteries-included local Kubernetes development experience. Where kind gives you a bare cluster, kinder comes with LoadBalancer support (MetalLB), Gateway API ingress (Envoy Gateway), metrics (`kubectl top` / HPA), tuned DNS (CoreDNS autopath + cache), a dashboard (Headlamp), a local container registry (localhost:5001), and cert-manager with self-signed TLS — all working out of the box. Addons install in parallel via wave-based execution for faster cluster creation. Users run `kinder create cluster` and get a fully functional development environment with zero manual setup, or use `--profile minimal|gateway|ci` for targeted addon presets. All read commands support `--output json` for scripting. Diagnostic tools (`kinder env`, `kinder doctor`) help with troubleshooting. The project website at kinder.patrykgolabek.dev provides documentation, installation guides, and addon references.

## Core Value

A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## Requirements

### Validated

- ✓ Binary renamed to `kinder`, coexists with `kind` — v1.0
- ✓ Config schema extends v1alpha4 with `addons` section — v1.0
- ✓ Existing kind configs work unchanged (backward compatible) — v1.0
- ✓ Each addon checks enable flag before executing — v1.0
- ✓ Platform warning for MetalLB on macOS/Windows — v1.0
- ✓ MetalLB installed by default with auto-detected subnet — v1.0
- ✓ Envoy Gateway installed by default with Gateway API CRDs — v1.0
- ✓ Metrics Server installed by default with `--kubelet-insecure-tls` — v1.0
- ✓ CoreDNS tuning applied by default (autopath, pods verified, cache 60) — v1.0
- ✓ Headlamp dashboard installed with printed token and port-forward command — v1.0
- ✓ Each addon individually disableable via cluster config — v1.0
- ✓ Addons wait for readiness before cluster is reported ready — v1.0

- ✓ Kinder website with landing page and documentation, built with Astro — v1.1
- ✓ Modern dark developer-tool aesthetic (terminal-style, cyan accents) — v1.1
- ✓ Landing page showcasing features, install instructions, and addon highlights — v1.1
- ✓ Documentation pages: installation guide, configuration reference, addon docs — v1.1
- ✓ Custom domain: kinder.patrykgolabek.dev via GitHub Pages — v1.1
- ✓ Source lives in `kinder-site/` directory at repo root — v1.1

- ✓ Kinder logo (modified kind robot with cyan "er") as SVG, PNG, favicon, OG image — v1.2
- ✓ SEO: llms.txt, JSON-LD structured data, complete meta tags, author backlinks — v1.2
- ✓ README rewrite for kinder identity — v1.2
- ✓ Hero section with logo on landing page — v1.2

- ✓ Fix port leak, tar truncation, cluster name resolution, network sort — v1.3
- ✓ Provider code deduplication: shared common/ package for docker/nerdctl/podman — v1.3
- ✓ Local registry addon at localhost:5001 with dev tool discovery — v1.3
- ✓ cert-manager addon with self-signed ClusterIssuer — v1.3
- ✓ v1alpha4 config API extended with LocalRegistry and CertManager fields — v1.3
- ✓ kinder env command for machine-readable cluster environment info — v1.3
- ✓ kinder doctor command for prerequisite checking with structured exit codes — v1.3

- ✓ Go 1.24 baseline, golangci-lint v2, SHA-256 subnet hashing, layer violation fix — v1.4
- ✓ context.Context propagated through all addon Execute() methods and waitForReady — v1.4
- ✓ FakeNode/FakeCmd test infrastructure with 30+ unit tests for all addon actions — v1.4
- ✓ Wave-based parallel addon execution with per-addon timing summary — v1.4
- ✓ `--output json` on all read commands (env, doctor, get clusters, get nodes) — v1.4
- ✓ `--profile` flag on create cluster with minimal/full/gateway/ci presets — v1.4

- ✓ Website updated with 3 tutorials, 3 CLI reference pages, 7 enriched addon pages, 19-page clean build — v1.5

- ✓ 18 diagnostic checks in `kinder doctor` across 8 categories with Check interface infrastructure — v2.1
- ✓ Docker checks: disk space, daemon.json init, snap, kubectl version skew, socket permissions — v2.1
- ✓ Kernel/security checks: inotify limits, kernel >=4.6, AppArmor, SELinux, firewalld — v2.1
- ✓ Platform checks: WSL2 cgroup v2, BTRFS rootfs, subnet clash detection — v2.1
- ✓ ApplySafeMitigations wired into create flow before provisioning — v2.1
- ✓ Known Issues documentation page on website with all 18 checks — v2.1

- ✓ Multi-version clusters with per-node Kubernetes version, ExplicitImage sentinel, and version-skew config validation — v2.2
- ✓ `kinder doctor` cluster-node-skew check and VERSION/IMAGE/SKEW columns on `kinder get nodes` — v2.2
- ✓ Offline/air-gapped cluster creation via `--air-gapped` flag, addon image warnings, fast-fail in all 3 providers — v2.2
- ✓ `kinder doctor` offline-readiness check + two-mode offline workflow docs — v2.2
- ✓ local-path-provisioner addon (default on) with CVE-2025-62878 doctor check, busybox:1.37.0 pinned for air-gap — v2.2
- ✓ Host directory mounting pre-flight validation, propagation platform warning, host-mount-path + docker-desktop-file-sharing doctor checks, two-hop host→pod guide — v2.2
- ✓ `kinder load images` subcommand with provider-abstracted save/import, smart-load skip, Docker Desktop 27+ containerd fallback — v2.2
- ✓ Doctor registry expanded from 18 to 23 checks — v2.2

- ✓ `kinder pause`/`kinder resume` with quorum-safe HA ordering (workers→CP→LB on pause, LB→CP→workers on resume) — v2.3
- ✓ `cluster-resume-readiness` doctor check using `crictl exec <etcd-id> etcdctl ...` probe — v2.3
- ✓ `kinder status` command + Status column on `kinder get clusters` (JSON schema migration to `[]{name,status}`) — v2.3
- ✓ `kinder snapshot create/restore/list/show/prune` with tar.gz bundle + sha256 sidecar capturing etcd + container images + local-path PVs — v2.3
- ✓ Snapshot restore with K8s/topology/addon hard-fail compatibility checks BEFORE any mutation (no auto-rollback) — v2.3
- ✓ Snapshot metadata records cluster K8s version, addon versions, image-bundle digest for air-gap reproducibility — v2.3
- ✓ `kinder dev --watch <dir> --target <deployment>` hot-reload with fsnotify + stdlib polling fallback, leading-trigger debouncer, build/load/rollout cycle — v2.3
- ✓ `kinder dev --poll` mode for Docker Desktop macOS (fsnotify-unfriendly volume mounts) — v2.3
- ✓ `kinder doctor decode` with 16-pattern catalog (kubelet/kubeadm/containerd/docker/addon-startup) and `--auto-fix` whitelist (3 SafeMitigation factories, preview-before-apply) — v2.3
- ✓ HAProxy→Envoy LB migration (kind PR #4127 port) wired across docker/podman/nerdctl with atomic xDS file-swap — v2.3
- ✓ IPVS-on-1.36+ validation guard at config-time with migration URL — v2.3
- ✓ K8s 1.36 "What's new" website recipe (User Namespaces GA + In-Place Pod Resize GA) — v2.3

### Active

(No active milestone — v2.3 shipped. Use `/gsd:new-milestone` to define v2.4.)

### Deferred / Carried Forward

- **SYNC-02** (from v2.3): Default `kindest/node` image bump to K8s 1.36.x — external blocker (`kindest/node:v1.36.x` not on Docker Hub as of 2026-05-07). Plan 51-04 halted cleanly at gating probe; Task 2 fully authored and ready to re-execute once kind v0.32.0 publishes the image.

### Out of Scope

- OAuth/OIDC integration for dashboard — too complex for v1; token-based auth is sufficient for local dev
- Multi-cluster networking — single cluster focus
- Helm as addon manager — avoid Helm dependency; use static manifests/go:embed
- Custom addon plugin system — hardcoded addons for v1, extensibility later
- Service mesh (Istio, Linkerd) — conflicts with Envoy Gateway; separate concern
- BGP mode for MetalLB — requires external router; impossible in kind without extra containers
- Versioned documentation — no breaking changes yet; overhead before kinder has multiple versions
- Interactive playground — impossible with Docker dependency; fake demos break trust
- Tailwind CSS — Starlight's CSS custom properties sufficient; Tailwind v4 integration unstable
- Harbor registry — too heavy for local dev; registry:2 is sufficient
- registry:3 image — v3 deprecated storage drivers; kind ecosystem on v2
- ACME issuers — requires internet; incompatible with offline local clusters

## Context

- Kinder is a fork of sigs.k8s.io/kind at commit 89ff06bd
- Shipped v1.0 with ~1,950 LOC Go across 5 addon action packages
- Shipped v1.1 with 878 LOC Astro/MDX/CSS/TS in kinder-site/
- Shipped v1.2 with logo, SEO, branding polish
- Shipped v1.3 with bug fixes, provider dedup, local registry, cert-manager, CLI tools
- Shipped v1.4 with Go 1.24, context.Context, unit tests, parallel execution, JSON output, profile presets
- Shipped v1.5 with 3 tutorials, 3 CLI reference pages, 7 enriched addon pages, 19-page clean production build
- Shipped v2.0 with GoReleaser, Homebrew tap, NVIDIA GPU addon
- Shipped v2.1 with 18 diagnostic checks, create-flow mitigations, Known Issues page
- Shipped v2.2 with multi-version validation, air-gapped clusters, local-path-provisioner, host-mount pre-flight, `kinder load images`; doctor registry expanded from 18 to 23 checks
- Total codebase: ~38,751 LOC Go (pkg/), ~5,200 LOC site (Astro/MDX/TS/CSS)
- Tech stack: Go (core), Astro + Starlight (website)
- Website live at https://kinder.patrykgolabek.dev via GitHub Pages
- All addons applied at runtime via kubectl (not baked into node image)
- Addon versions pinned: MetalLB v0.15.3, Metrics Server v0.8.1, Envoy Gateway v1.3.1, Headlamp v0.40.1, cert-manager v1.16.3
- 7 addons total: MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, Headlamp, Local Registry, cert-manager
- Provider code deduplicated: shared common/ package for docker/nerdctl/podman
- GitHub org: PatrykQuantumNomad/kinder

## Constraints

- **Tech stack (core)**: Go, same build system as kind — no new languages or build tools
- **Tech stack (website)**: Astro framework, TypeScript/JS — standard static site tooling
- **Compatibility**: Must work with Docker, Podman, and Nerdctl (all existing providers)
- **Config format**: Extend kind's `v1alpha4` config API with addon fields, don't break existing configs
- **Image size**: Addon manifests applied at runtime, not baked into node image
- **Hosting**: GitHub Pages with custom domain kinder.patrykgolabek.dev

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Fork kind, don't wrap it | Full control over action pipeline, single binary | ✓ Good |
| Addons as creation actions | Follows existing kind pattern (installcni, installstorage) | ✓ Good |
| On by default, opt-out | Target audience wants batteries included; power users can disable | ✓ Good |
| Runtime manifest apply | Apply addon manifests via kubectl during creation | ✓ Good |
| *bool for v1alpha4 Addons | nil means not-set, defaults to true; avoids Go zero-value ambiguity | ✓ Good |
| go:embed for all manifests | Offline-capable, no external tools at cluster creation | ✓ Good |
| Headlamp over kubernetes/dashboard | kubernetes/dashboard archived Jan 2026, Helm dependency | ✓ Good |
| Server-side apply for Envoy Gateway | httproutes CRD exceeds 256 KB annotation limit | ✓ Good |
| CoreDNS read-modify-write in Go | Corefile is a string blob, not structured YAML | ✓ Good |
| Astro for website | Static site generator, fast, great for docs + landing pages | ✓ Good |
| Separate kinder-site/ directory | Clean separation from Go codebase, no reuse of kind's site | ✓ Good |
| No kind website code reuse | Fresh design for kinder identity, avoids inheriting kind's patterns | ✓ Good |
| Dark-only mode (no light theme) | Terminal aesthetic is core identity; removed theme toggle | ✓ Good |
| npm over pnpm for CI | GitHub Actions compatibility; avoids extra setup step | ✓ Good |
| Starlight with CSS custom properties | No Tailwind needed; theme overrides via CSS variables | ✓ Good |
| `make install` as only install method | Binary distribution unconfirmed; build-from-source is reliable | ✓ Good |
| Kinder logo from modified kind robot | Distinct identity, "er" in cyan matches theme | ✓ Good |
| favicon.ico over favicon.svg | SVG had font rendering issues; ICO universally supported | ✓ Good |
| llms.txt for GEO | AI crawler discoverability; emerging standard | ✓ Good |
| JSON-LD SoftwareApplication schema | Rich snippets in search, author attribution | ✓ Good |
| Extract shared provider code to common/ | Eliminate ~70-80% duplication, prevent drift bugs | ✓ Good |
| Local registry as addon, not shell script | Consistent with batteries-included ethos | ✓ Good |
| Cert-manager alongside Envoy Gateway | Natural pairing; TLS commonly needed with Gateway API | ✓ Good |
| registry:2 not registry:3 | Kind ecosystem on v2; v3 deprecated storage drivers | ✓ Good |
| ContainerdConfigPatches before Provision | Cannot inject post-provisioning; must be in create.go before p.Provision() | ✓ Good |
| cert-manager v1.16.3 with --server-side | 986KB manifest exceeds 256KB annotation limit; v1.17.6 not yet released | ✓ Good |
| Provider.Name() via fmt.Stringer | Type assertion instead of new interface method; zero-impact on existing code | ✓ Good |
| Context in ActionContext struct (not param) | Minimal call-site churn; deliberate trade-off | ✓ Good |
| Wave-based parallel not full DAG | 7 addons with shallow deps; DAG adds 200+ lines for zero benefit | ✓ Good |
| sync.OnceValues for Nodes() cache | Eliminates TOCTOU race; single-call guarantee over RWMutex | ✓ Good |
| errgroup.SetLimit(3) for parallel addons | Bounds concurrent kubectl apply calls | ✓ Good |
| Consistent flagpole/switch/json.NewEncoder | All JSON commands follow same pattern | ✓ Good |
| CreateWithAddonProfile with 4 presets | Covers minimal/full/gateway/ci without YAML config files | ✓ Good |

| Check interface + registry architecture | Clean single-integration-point for all 18 checks | Good |
| Build-tagged platform abstractions | disk_unix.go/kernel_linux.go compile cleanly on all platforms | Good |
| Deps struct injection for check testability | Injectable readFile/execCmd/lookPath without mocking packages | Good |
| WSL2 multi-signal detection | Prevents Azure VM false positives with corroborating evidence | Good |
| Warn-and-continue mitigations | Mitigation errors never block cluster creation | Good |

| ExplicitImage sentinel captured pre-defaults | SetDefaultsCluster fills Image before conversion; post-defaults detection impossible | ✓ Good |
| Skip version-skew check on non-semver tags | Preserves backward compat with "latest" in test/dev configs | ✓ Good |
| AirGapped propagated via ClusterOptions → Config | Matches existing `CreateWithRetain` option pattern; no new trust boundary | ✓ Good |
| Accumulate-all-missing images (not fail-on-first) | Users get one actionable error listing every pre-load needed | ✓ Good |
| `RequiredAddonImages` imports addon packages in common | Addon packages do not import common; zero import cycle risk | ✓ Good |
| Inline allAddonImages in offlinereadiness.go | Importing pkg/cluster/internal creates a doctor import cycle | ✓ Good |
| LocalPath uses opt-out `boolVal` (default true) | Consistent with MetalLB/CertManager batteries-included pattern | ✓ Good |
| StorageClass named `local-path` (not `standard`) | Avoids collision with legacy `installstorage` StorageClass | ✓ Good |
| busybox pinned to 1.37.0 with IfNotPresent | PVC operations work in air-gap where `busybox:latest` can't pull | ✓ Good |
| CVE threshold v0.0.34 returns ok | v0.0.34 is the fix version; only strictly less-than warns | ✓ Good |
| `validateExtraMounts` runs between Validate and Provision | Clean rollback-free failure before any container is created | ✓ Good |
| `mountPathConfigurable` interface (unexported) | Doctor package owns wiring; cmd layer only sees exported SetMountPaths | ✓ Good |
| `providerBinaryName` reads KIND_EXPERIMENTAL_PROVIDER | `provider.Name()` always returns "nerdctl" — env var distinguishes finch/nerdctl.lima | ✓ Good |
| `LoadImageArchiveWithFallback` factory pattern | Tar streams cannot be rewound; factory yields fresh reader per attempt | ✓ Good |
| `stderrors` alias for stdlib errors.As | Avoids conflict with `sigs.k8s.io/kind/pkg/errors` import | ✓ Good |
| Container-exec kubectl for CVE version probe | Same pattern as realListNodes; avoids pkg/cluster/internal import cycle | ✓ Good |
| Zero new Go module dependencies in v2.2 | All five features use packages already in go.mod | ✓ Good |

---
*Last updated: 2026-05-03 — milestone v2.3 Inner Loop started*
