# Project Milestones: Kinder

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
