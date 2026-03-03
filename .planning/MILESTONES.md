# Project Milestones: Kinder

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

**Git range:** `docs(19)` → `docs(phase-24)`

**What's next:** TBD — potential v1.4 with registry enhancements, trust-manager, kinder env --shell fish, kinder doctor --fix

---

## v1.2 Branding & Polish (Shipped: 2026-03-02)

**Delivered:** Distinct kinder visual identity, SEO discoverability, documentation rewrite, and dark-only theme enforcement — establishing kinder as its own project beyond the kind fork.

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

**What's next:** TBD — potential v1.3 with animated terminal demo, contributing guide, or blog section

---

## v1.1 Kinder Website (Shipped: 2026-03-02)

**Delivered:** Standalone documentation website with dark terminal aesthetic, interactive landing page, and 10 documentation pages — live at kinder.patrykgolabek.dev via GitHub Pages.

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

**Git range:** `feat(09-01)` → `feat(14-01)`

**What's next:** TBD — potential v1.2 with animated terminal demo, blog section, or contributing guide

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

**Git range:** `feat(01-01)` → `fix(08-02)`

**What's next:** TBD — potential v1.1 with cert-manager, NodeLocal DNSCache, or Prometheus stack

---
