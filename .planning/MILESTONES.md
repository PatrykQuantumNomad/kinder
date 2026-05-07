# Project Milestones: Kinder

## v2.3 Inner Loop (Shipped: 2026-05-07)

**Delivered:** Made daily iteration on a kinder cluster as fast as creating one — pause/resume with quorum-safe HA ordering, snapshot/restore with hard-fail compatibility checks, hot-reload via `kinder dev` (fsnotify + polling fallback), and a runtime error decoder extending the v2.1 doctor framework with 16 cataloged patterns. Adopted kind upstream's HAProxy→Envoy LB transition across docker/podman/nerdctl providers and shipped a K8s 1.36 recipe page (default node image bump deferred pending Docker Hub publication).

**Phases completed:** 47-51 (25 plans total)

**Key accomplishments:**
- **Phase 47 — Cluster Pause/Resume**: `lifecycle.Pause`/`Resume` with explicit quorum-safe ordering (workers→CP→LB on pause, LB→CP→workers on resume), `cluster-resume-readiness` doctor check using `crictl exec <etcd-id> etcdctl ...` probe, `kinder status` command, JSON schema migration on `kinder get clusters`, two re-verification gap-closure plans (47-05 + 47-06) closing 5 UAT failures
- **Phase 48 — Cluster Snapshot/Restore**: tar.gz bundle with sha256 sidecar capturing etcd + container images + local-path PVs, restore with K8s/topology/addon hard-fail compatibility checks BEFORE any mutation, 5 CLI subcommands (create/restore/list/show/prune), full integration test suite, live UAT approved
- **Phase 49 — Inner-Loop Hot Reload**: `kinder dev --watch <dir> --target <deployment>` with fsnotify v1.10.1 (first new module dep since v2.0) + stdlib polling fallback, leading-trigger debouncer, `kinder load images` core reuse via public APIs, `--poll` mode for Docker Desktop macOS, live UAT 4.1s end-to-end cycle
- **Phase 50 — Runtime Error Decoder**: `kinder doctor decode` with 16-pattern catalog (KUB-01..05, KADM-01..03, CTD-01..03, DOCK-01..03, ADDON-01..02), plain-English explanations + suggested fixes + doc links, `--auto-fix` whitelist with 3 SafeMitigation factories (preview-before-apply enforced), bare `kinder doctor` 24-check pipeline preserved
- **Phase 51 — Upstream Sync & K8s 1.36**: HAProxy→Envoy LB migration (kind PR #4127 port) wired across all 3 providers with atomic xDS file-swap (no SIGHUP), IPVS-on-1.36+ validation guard with migration URL, K8s 1.36 "What's new" website recipe (User Namespaces GA + In-Place Pod Resize GA). SC2 (default node image bump) DEFERRED — `kindest/node:v1.36.x` not yet on Docker Hub

**Stats:**
- 125 code files created/modified (+23,683 / -91 lines)
  - pkg/ Go: 123 files, +23,515 / -91
  - kinder-site/ site: 2 files, +168 / -0
- ~110 commits in v2.3 range (`test(47-01)` → `docs(51-04)`)
- 5 phases, 25 plans, 21 requirements (20 satisfied, 1 deferred on external blocker)
- 5 days (2026-05-03 → 2026-05-07)

**Git range:** `test(47-01)` → `docs(51-04)`

**Audit:** `.planning/milestones/v2.3-MILESTONE-AUDIT.md` — status `tech_debt`; 9/9 cross-phase integration points wired; 4/4 E2E flows complete; zero regressions.

**What's next:** v2.4 (TBD) — likely candidates: SYNC-02 re-execution once kind v0.32.0 ships, addon bumps (cert-manager v1.20, Envoy Gateway v1.7, Headlamp v0.41), pause/resume support for podman + nerdctl providers, snapshot remote-storage backend.

---

## v2.2 Cluster Capabilities (Shipped: 2026-04-10)

**Delivered:** Added four cluster capabilities to kinder — multi-version per-node Kubernetes validation, offline/air-gapped cluster creation, local-path-provisioner dynamic storage, host-directory mounting pre-flight — plus a provider-abstracted `kinder load images` subcommand that ties the offline and multi-version workflows together. All features integrate using packages already in `go.mod` with zero new dependencies.

**Phases completed:** 42-46 (14 plans total)

**Key accomplishments:**
- Multi-version node validation: ExplicitImage sentinel fixes `--image` flag override bug, config-time version-skew validator rejects HA CP minor mismatches and workers >3 minor behind the control-plane with table-format error output, `cluster-node-skew` doctor check and VERSION/IMAGE/SKEW columns on `kinder get nodes`
- Air-gapped cluster creation: `--air-gapped` flag plumbed through config + ClusterOptions, 7 addon packages export `var Images []string`, `RequiredAddonImages`/`RequiredAllImages` utilities in `providers/common`, fast-fail accumulate-all-missing path in docker/podman/nerdctl providers, addon image pre-pull warning in create flow, `offline-readiness` doctor check with tabwriter missing-image table, two-mode offline workflow documented
- Local-path-provisioner addon: `LocalPath *bool` wired through 5-location config pipeline (opt-out, default on), new `installlocalpath` action package with embedded v0.0.35 manifest patched to `busybox:1.37.0`/`IfNotPresent`, StorageClass renamed `local-path`, `installstorage` gated out when enabled, wave1 registration, CVE-2025-62878 (`local-path-cve`) doctor check against v0.0.34 fix version
- Host-directory mounting: `validateExtraMounts` pre-flight host-path existence check runs before any container is created, `logMountPropagationPlatformWarning` on Docker Desktop for non-None propagation, `host-mount-path` and `docker-desktop-file-sharing` (macOS) doctor checks with `--config` flag wiring via `mountPathConfigurable` interface, two-hop host-to-pod mount guide on the website
- `kinder load images` subcommand: provider-abstracted `save()`/`imageID()` using `providerBinaryName()` (reads `KIND_EXPERIMENTAL_PROVIDER` for finch/nerdctl.lima variants), `LoadImageArchiveWithFallback` in nodeutils with factory-pattern reader and `isContentDigestError` detection for Docker Desktop 27+ containerd image store two-attempt `ctr import` fallback, smart-load skip (re-tag in place when image ID matches), registered as third `load` subcommand
- Doctor registry expanded from 18 to 23 checks (added cluster-node-skew, offline-readiness, local-path-cve, host-mount-path, docker-desktop-file-sharing)

**Stats:**
- 61 code files created/modified (+5,439 / -209 lines)
  - pkg/ Go: 52 files, +4,426 / -24
  - kinder-site/ site: 8 files, +1,013 / -10
- 38,751 total Go LOC in pkg/ (up from 35,636 at v2.1 ship)
- 5 phases, 14 plans, 25 requirements (all satisfied)
- 31 feat/fix/test commits (61 total commits including docs)
- ~2.5 days (2026-04-08 → 2026-04-10)

**Git range:** `test(42-01)` → `docs(phase-46)`

**What's next:** TBD

---

## v2.1 Known Issues & Proactive Diagnostics (Shipped: 2026-03-06)

**Delivered:** Expanded `kinder doctor` from 3 to 18 diagnostic checks across 8 categories, wired automatic mitigations into cluster creation, and documented all checks on the website's Known Issues page.

**Phases completed:** 38-41 (10 plans total)

**Key accomplishments:**
- Built Check interface infrastructure with category-grouped output, platform filtering, JSON/human-readable formatters, and SafeMitigation system
- Expanded `kinder doctor` from 3 to 18 diagnostic checks across 8 categories (Runtime, Docker, Tools, GPU, Kernel, Security, Platform, Network)
- Docker environment checks: disk space thresholds, daemon.json init flag, snap detection, kubectl version skew, socket permissions
- Linux kernel and security checks: inotify limits, kernel >=4.6, AppArmor interference, SELinux on Fedora, firewalld nftables backend
- Platform checks: WSL2 multi-signal detection with cgroup v2 verification, BTRFS rootfs, Docker subnet clash detection
- Website Known Issues page documenting all 18 checks with What/Why/Platforms/Fix structure, plus create-flow mitigation wiring

**Stats:**
- 77 files created/modified
- 12,609 lines inserted, 305 deleted (35,636 total Go LOC)
- 4 phases, 10 plans
- 1 day (2026-03-06)

**Git range:** `feat(38-01)` -> `feat(41-03)`

**What's next:** TBD

---

## v2.0 Distribution & GPU Support (Shipped: 2026-03-05)

**Delivered:** GoReleaser cross-platform binary distribution, Homebrew tap for macOS installation, and NVIDIA GPU addon with device plugin, RuntimeClass, and doctor checks.

**Phases completed:** 35-37 (7 plans total)

**Key accomplishments:**
- GoReleaser config producing cross-platform binaries (linux/darwin amd64+arm64, windows amd64) with SHA-256 checksums and automated changelog
- Homebrew Cask in PatrykQuantumNomad/homebrew-kinder tap for `brew install` distribution
- NVIDIA GPU addon with device plugin DaemonSet, RuntimeClass, and opt-in v1alpha4 config field
- Three new `kinder doctor` checks for NVIDIA driver, container toolkit, and Docker nvidia runtime
- Website updated with Homebrew install instructions and GPU addon documentation page

**Stats:**
- 3 phases, 7 plans
- 2 days (2026-03-04 -> 2026-03-05)

**Git range:** `feat(35-01)` -> `docs(phase-37)`

**What's next:** v2.1 Known Issues & Proactive Diagnostics

---

## v1.5 Website Use Cases & Documentation (Shipped: 2026-03-04)

**Delivered:** Updated the kinder website with detailed use cases, tutorials, and CLI reference pages -- enriching all 7 addon pages with practical examples and troubleshooting, writing 3 end-to-end tutorials, and creating 3 CLI reference pages.

**Phases completed:** 30-34 (7 plans total)

**Key accomplishments:**
- Updated landing page, quick-start, and configuration to document all 7 addons with core/optional grouping
- Added practical examples and Symptom/Cause/Fix troubleshooting to all 7 addon pages
- Created 3 CLI reference pages: profile comparison (4 presets x 7 addons), JSON output with jq recipes, env/doctor troubleshooting with exit codes
- Wrote 3 end-to-end tutorials: TLS web app (4 addons), HPA auto-scaling, local dev workflow
- Fixed ci profile documentation bug and Go version mismatch, verified 19-page clean production build

**Stats:**
- 46 files created/modified
- 6,830 lines inserted, 66 deleted (3,386 site LOC)
- 5 phases, 7 plans, 35 commits
- 1 day (2026-03-04)

**Git range:** `feat(30-01)` -> `docs(phase-34)`

**What's next:** TBD

---

## v1.4 Code Quality & Features (Shipped: 2026-03-04)

**Delivered:** Modernized the Go toolchain, added context.Context cancellation, built a unit test suite for all addon actions, implemented wave-based parallel addon execution, and shipped JSON output + cluster profile presets for the CLI.

**Phases completed:** 25-29 (13 plans total)

**Key accomplishments:**
- Go 1.24 baseline with golangci-lint v2 zero-issue pass across 60+ files, SHA-256 subnet hashing, and layer violation fix
- context.Context propagated through all 7 addon Execute() methods and waitForReady loop for cancellation support
- FakeNode/FakeCmd/FakeProvider test infrastructure with 30+ unit tests covering all addon actions without a live cluster
- Wave-based parallel addon execution via errgroup.SetLimit(3) with race-free sync.OnceValues node caching and per-addon timing
- `--output json` on all 4 read commands (env, doctor, get clusters, get nodes) with consistent flagpole pattern
- `--profile` flag on `create cluster` with 4 presets (minimal, full, gateway, ci) backed by CreateWithAddonProfile

**Stats:**
- 195 files created/modified
- 13,646 lines inserted, 3,775 deleted (29,592 total Go LOC)
- 5 phases, 13 plans, 66 commits
- 2 days (2026-03-03 -> 2026-03-04)

**Git range:** `feat(25-03)` -> `docs(phase-29)`

**What's next:** TBD

---

## v1.3 Harden & Extend (Shipped: 2026-03-03)

**Delivered:** Fixed 4 correctness bugs, eliminated ~525 lines of provider code duplication, and added batteries-included local registry, cert-manager addons, and CLI diagnostic tools.

**Phases completed:** 19-24 (8 plans total)

**Key accomplishments:**
- Fixed 4 correctness bugs: port leak in generatePortMappings, tar truncation silent data loss, ListInternalNodes default name, network sort strict weak ordering
- Extracted shared docker/podman/nerdctl provider code to common/ package, eliminating ~525 lines of duplication
- Added local registry addon at localhost:5001 with containerd certs.d wiring and dev tool discovery ConfigMap
- Added cert-manager addon with embedded v1.16.3 manifest, webhook readiness gate, and self-signed ClusterIssuer
- Created kinder env (eval-safe machine-readable output) and kinder doctor (prerequisite checker with structured exit codes)
- Extended v1alpha4 config API with LocalRegistry and CertManager addon fields wired through all 5 pipeline locations

**Stats:**
- 69 files created/modified
- 21,695 lines inserted, 672 deleted
- 6 phases, 8 plans, 43 commits
- ~5 hours (single day, 2026-03-03)

**Git range:** `docs(19)` -> `docs(phase-24)`

**What's next:** TBD -- potential v1.4 with registry enhancements, trust-manager, kinder env --shell fish, kinder doctor --fix

---

## v1.2 Branding & Polish (Shipped: 2026-03-02)

**Delivered:** Distinct kinder visual identity, SEO discoverability, documentation rewrite, and dark-only theme enforcement -- establishing kinder as its own project beyond the kind fork.

**Phases completed:** 15-18

**Key accomplishments:**
- Kinder logo created from modified kind robot with "er" in cyan, exported as SVG, PNG, favicon.ico, and OG image
- Original kind logo preserved unmodified in logo/ directory
- SEO: llms.txt/llms-full.txt for AI crawlers, JSON-LD structured data, complete meta tags, author backlinks
- Root README and kinder-site README rewritten from kind/boilerplate to kinder branding
- Dark theme enforced site-wide with no light mode option
- Kinder logo displayed in hero section of landing page

**Stats:**
- 17 requirements across 4 categories (Brand, SEO, Docs, Site)
- 4 phases, all formalized from completed work
- Assets created: logo.svg, logo.png, favicon.ico, og.png, llms.txt, llms-full.txt

**Git range:** v1.2 formalized existing work

**What's next:** TBD -- potential v1.3 with animated terminal demo, contributing guide, or blog section

---

## v1.1 Kinder Website (Shipped: 2026-03-02)

**Delivered:** Standalone documentation website with dark terminal aesthetic, interactive landing page, and 10 documentation pages -- live at kinder.patrykgolabek.dev via GitHub Pages.

**Phases completed:** 9-14 (8 plans total)

**Key accomplishments:**
- Astro/Starlight site scaffolded, deployed via GitHub Actions to kinder.patrykgolabek.dev with DNS, HTTPS, and custom domain
- Dark cyan terminal aesthetic enforced site-wide with no theme toggle and no FOUC
- 10 documentation pages: installation, quick-start, configuration reference, and 5 addon guides
- Interactive landing page with copy-to-clipboard install command, kind vs kinder comparison, and addon feature cards
- Branded OG image, favicon, custom 404 page, robots.txt, and Lighthouse 90+ on all metrics
- Mobile-responsive at 375px viewport, all GitHub links aligned to PatrykQuantumNomad org

**Stats:**
- 27 files created/modified
- 878 lines of code (Astro/MDX/CSS/TS)
- 6 phases, 8 plans, 41 commits
- 2 days from start to ship

**Git range:** `feat(09-01)` -> `feat(14-01)`

**What's next:** TBD -- potential v1.2 with animated terminal demo, blog section, or contributing guide

---

## v1.0 Batteries Included (Shipped: 2026-03-01)

**Delivered:** Forked kind into kinder with 5 default addons (MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, Headlamp dashboard) that work out of the box and can be individually disabled via config.

**Phases completed:** 1-8 (12 plans total)

**Key accomplishments:**
- Binary renamed to `kinder` with backward-compatible v1alpha4 config schema extended with `addons` section
- MetalLB auto-detects Docker/Podman/Nerdctl subnet and assigns LoadBalancer IPs without user input
- Envoy Gateway installed with full wait chain for end-to-end Gateway API routing
- Metrics Server, CoreDNS tuning, and Headlamp dashboard all install automatically with printed access instructions
- Each addon individually disableable via `addons.<name>: false` in cluster config
- Integration test suite validates all 5 addons functional together

**Stats:**
- 65 files created/modified
- ~1,950 lines of Go (addon actions)
- 8 phases, 12 plans, 36 commits
- 1 day from start to ship

**Git range:** `feat(01-01)` -> `fix(08-02)`

**What's next:** TBD -- potential v1.1 with cert-manager, NodeLocal DNSCache, or Prometheus stack

---
