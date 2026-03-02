# Roadmap: Kinder v1.2 Branding & Polish

**Milestone:** v1.2 Branding & Polish
**Phases:** 15-18 (continuing from v1.1 Phase 14)
**Status:** Complete

---

## Phase 15: Kinder Logo & Brand Assets

**Goal:** Create distinct kinder visual identity — logo SVG/PNG, favicon, OG image.

**Requirements covered:** BRAND-01, BRAND-02, BRAND-03, BRAND-04, BRAND-05

**Deliverables:**
- Modified kind robot logo with "er" in cyan (`kinder-logo/logo.svg`)
- PNG export at 1200px width (`kinder-logo/logo.png`)
- Original kind logo preserved unmodified in `logo/`
- Multi-resolution favicon.ico (16, 32, 48px) replacing favicon.svg
- Branded OG image (1200x630) with logo, title, and tagline

**Status:** Complete

---

## Phase 16: SEO & Discoverability

**Goal:** Maximize search engine and AI crawler discoverability for kinder.patrykgolabek.dev.

**Requirements covered:** SEO-01, SEO-02, SEO-03, SEO-04, SEO-05, SEO-06

**Deliverables:**
- `llms.txt` with concise site summary for AI crawlers
- `llms-full.txt` with full documentation content for LLM ingestion
- JSON-LD structured data (SoftwareApplication + WebSite schemas)
- Complete Twitter Card meta tags (title, description, image)
- Author meta tag and rel=author link to patrykgolabek.dev
- Keywords meta tag with relevant Kubernetes/devtool terms

**Status:** Complete

---

## Phase 17: Documentation Rewrite

**Goal:** Rewrite READMEs from inherited kind content to proper kinder branding.

**Requirements covered:** DOCS-01, DOCS-02, DOCS-03

**Deliverables:**
- Root README rewritten with kinder badges, quick start, addon table, acknowledgements
- kinder-site README updated from Starlight boilerplate to project README
- Author backlink to patrykgolabek.dev on homepage footer

**Status:** Complete

---

## Phase 18: Site Theme & Hero

**Goal:** Enforce dark-only theme and add kinder logo to hero section.

**Requirements covered:** SITE-01, SITE-02, SITE-03

**Deliverables:**
- Dark theme enforced via CSS and inline script (no light mode option)
- Kinder logo displayed in hero section of landing page
- Favicon.ico configured in Astro (replacing removed favicon.svg)

**Status:** Complete

---

**Coverage:**
- Total requirements: 17
- Mapped to phases: 17
- Unmapped: 0

*Created: 2026-03-02*
*Last updated: 2026-03-02 — all phases complete*
