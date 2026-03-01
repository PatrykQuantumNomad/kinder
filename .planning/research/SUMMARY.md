# Project Research Summary

**Project:** kinder website — landing page + docs at kinder.patrykgolabek.dev
**Domain:** CLI tool documentation site + marketing landing page (static, GitHub Pages)
**Researched:** 2026-03-01
**Confidence:** HIGH

## Executive Summary

The kinder website (v1.1 milestone) is a static developer-tool site consisting of a marketing landing page and structured documentation for a batteries-included kind fork. Research across all four areas converges on a single clear approach: Astro 5 with the Starlight theme, deployed to GitHub Pages via GitHub Actions, with a custom domain at `kinder.patrykgolabek.dev`. This stack is purpose-built for exactly this use case — Starlight provides search, sidebar navigation, dark mode, syntax highlighting, and MDX out of the box with a single install. The site lives in a new `kinder-site/` subdirectory, fully decoupled from the existing Go CLI codebase with no shared tooling or build system. Do not modify the existing Hugo `site/` directory, which belongs to the upstream kind project.

The recommended build order is: scaffold the Astro/Starlight project, establish the deployment pipeline, validate the live URL, then write all content. This front-loads infrastructure risk and means content work is never blocked by a broken deploy. The dark terminal aesthetic requires only CSS custom property overrides (`--sl-*` variables) — no Tailwind, no component overrides, no additional libraries. Feature scope for launch is well-defined: hero landing page with addon grid, installation guide, quickstart, configuration reference, and five addon doc pages.

The primary risks are all deployment-related, not content-related: Jekyll silently stripping Astro's `_astro/` assets folder, custom domain resetting on every deploy without a committed `CNAME` file, incorrect `base` config breaking all internal links, and missing GitHub Actions permissions blocking the deploy step entirely. Every one of these fails silently in local dev and only manifests in production. All are prevented by doing Phase 1 correctly — address them once during scaffolding and they never recur.

---

## Key Findings

### Recommended Stack

Astro 5 (currently 5.17) with `@astrojs/starlight` (0.37.x) is the correct choice for this project. The Starlight template installs Astro, TypeScript, Expressive Code (syntax highlighting), and Pagefind (full-text search) in a single command — covering all documentation infrastructure needs without additional packages. Node.js 22 LTS is required (Astro 5.8+ dropped Node 18). Do not add Tailwind: the `@astrojs/starlight-tailwind` integration has an open Tailwind v4 compatibility issue as of March 2026 and Starlight's `--sl-*` CSS variable system is sufficient for all theming needs.

**Core technologies:**
- **Astro 5.17**: Site framework — zero-JS-by-default output; Starlight built on it; GitHub Pages adapter
- **@astrojs/starlight 0.37.x**: Docs theme — built-in search, sidebar, dark mode, MDX, syntax highlighting; single install covers all doc needs
- **Node.js 22 LTS**: Runtime — required by Astro 5.8+; `withastro/action@v5` defaults to Node 22
- **withastro/action@v5**: GitHub Actions build — detects npm from lockfile; supports `path:` for subdirectory projects
- **Self-hosted Inter Variable font**: Body typography — `@fontsource-variable/inter`; avoids Google Fonts GDPR exposure
- **CSS custom properties only**: Dark theme — override `--sl-*` variables in `custom.css`; no Tailwind needed

**Explicit exclusions:**
- No Tailwind (`@astrojs/starlight-tailwind` has unresolved Tailwind v4 compatibility as of March 2026)
- No Hugo modifications (existing `site/` belongs to upstream kind; leave it alone)
- No Next.js, Docusaurus, or Jekyll (overkill, wrong fit, or 2010s-era DX)

### Expected Features

Research analyzed Bun, Deno, Astro, kind, Headlamp, k9s, and DevSpace sites directly. The audience is developers who know Kubernetes and have used kind — they need to understand kinder's value over kind in under 10 seconds.

**Must have (table stakes):**
- One-line install command in hero — developers copy-paste before reading anything else
- Copy-to-clipboard on all code blocks — Starlight provides this for doc pages; hero needs manual implementation
- Dark mode as default — developer audience; Starlight provides toggle; dark must be the default
- Installation guide doc page — deeplinked from GitHub README, blog posts, Stack Overflow answers
- Configuration reference doc page — full `kinder.dev/v1alpha4` schema with all addon fields and defaults
- Five addon doc pages (MetalLB, Envoy Gateway, Metrics Server, CoreDNS Tuning, Headlamp)
- Quickstart page — 5-step path from zero to working cluster
- GitHub link in nav — source visibility is a trust signal
- Favicon + og:image — site identity and social share previews
- Custom 404 page — avoids GitHub's default 404 breaking the experience

**Should have (competitive advantage):**
- Addon card grid on landing page — the 5 addons are the product differentiator; visual cards make the value concrete with links to each doc page
- Before/after comparison block — "kind gives you a bare cluster, kinder gives you a cluster with these 5 addons" is the fastest conversion pitch
- Cluster config YAML example on landing page — opt-out config is a key DX feature; showing it early sets expectations
- Animated terminal demo in hero — CSS animation preferred over asciinema; shows real `kinder create cluster` output

**Defer (v2+):**
- Blog section — requires ongoing content cadence; stale blog signals dead project
- Versioned docs — overhead not justified until a breaking change exists between versions
- Interactive playground — kinder requires Docker; browser sandbox is not feasible

### Architecture Approach

The website is a fully self-contained Node project in `kinder-site/` at the repo root, completely decoupled from Go tooling, `go.mod`, and the existing Hugo `site/` directory. There is intentionally no runtime coupling between the CLI and the website — version numbers and addon details are hard-coded in MDX frontmatter and updated manually at release time. The landing page (`src/pages/index.astro`) overrides Starlight's root page to provide a full-width marketing layout without a sidebar; all documentation routes use Starlight's standard layout with sidebar, search, and breadcrumbs.

**Major components:**
1. `kinder-site/` — entire Astro project; self-contained with own `package.json` and `package-lock.json`
2. `src/content/docs/` — all documentation as Markdown/MDX; Starlight auto-generates sidebar, search index, and prev/next navigation
3. `src/pages/index.astro` — custom landing page with Hero, FeatureGrid, AddonCard, QuickStart components; no sidebar
4. `public/CNAME` — custom domain declaration; must be present in the build artifact on every deploy
5. `public/.nojekyll` — prevents Jekyll from stripping the `_astro/` assets directory on GitHub Pages
6. `.github/workflows/deploy-site.yml` — path-filtered workflow triggering only on `kinder-site/**` changes

### Critical Pitfalls

Full details in `.planning/research/PITFALLS.md`. Top issues that cause silent production failures:

1. **Jekyll strips the `_astro/` assets folder (C1)** — GitHub Pages runs Jekyll by default; Jekyll ignores directories starting with `_`; Astro outputs compiled assets to `dist/_astro/`. The site loads but has no styles, no JS, and broken images in production while working perfectly locally. Fix: add `public/.nojekyll` AND set `build: { assets: 'assets' }` in `astro.config.mjs`. Apply in Phase 1.

2. **Custom domain resets on every deploy (C2)** — GitHub Pages stores the domain in a `CNAME` file in the deploy artifact. If absent, the domain clears silently after every push. Fix: commit `public/CNAME` containing `kinder.patrykgolabek.dev`; it copies to `dist/` on every build. Apply in Phase 1.

3. **Wrong `base` config breaks all internal links (C3)** — Setting `base: '/kinder'` is correct for `username.github.io/kinder` sub-paths but wrong for a custom domain. Fix: set only `site: 'https://kinder.patrykgolabek.dev'`; omit `base`. Apply in Phase 1.

4. **Missing GitHub Actions permissions block deploy (C6)** — `actions/deploy-pages` requires `pages: write` and `id-token: write` explicitly declared; repository Pages setting must be "GitHub Actions" source. Apply in Phase 1.

5. **Dark mode FOUC — page flashes white on load (C5)** — Browser renders light styles before the JS-managed `dark` class is applied. Fix: inject a blocking `is:inline` script in `<head>` that reads `localStorage` synchronously before first paint; also handle `astro:after-swap` for View Transitions. Apply in Phase 2.

---

## Implications for Roadmap

The architecture document's 6-phase build order is validated by both the feature dependencies and the pitfall-to-phase mapping. The order is non-negotiable: scaffold and deploy pipeline first, then theme, then documentation, then landing page, then assets, then polish. This sequence ensures deployment infrastructure is validated before any content is written.

### Phase 1: Scaffold and Deploy Pipeline

**Rationale:** All 7 critical pitfalls except FOUC map to Phase 1. Infrastructure risk must be front-loaded. Getting a placeholder site live at `kinder.patrykgolabek.dev` before writing a word of content validates DNS, CNAME, GitHub Pages permissions, path filtering, and the `withastro/action` subdirectory setup simultaneously.

**Delivers:** Working `kinder.patrykgolabek.dev` URL with Starlight default content; GitHub Actions deploy pipeline; DNS + HTTPS configured.

**Addresses:**
- Bootstrap `kinder-site/` with `npm create astro@latest -- --template starlight`
- Configure `astro.config.mjs`: `site`, sidebar structure, social links, `build: { assets: 'assets' }`
- Add `public/CNAME` (`kinder.patrykgolabek.dev`), `public/.nojekyll`
- Add `.github/workflows/deploy-site.yml` with path filtering (`kinder-site/**`) and correct permissions
- Configure GitHub Pages: source = GitHub Actions; custom domain
- Configure DNS: CNAME record `kinder -> patrykgolabek.github.io`
- Update existing Go CI workflows: add `kinder-site/**` to `paths-ignore` in all four Go workflow files

**Avoids:** C1 (Jekyll asset stripping), C2 (CNAME reset), C3 (base config), C4 (site URL mismatch), C6 (workflow permissions), C7 (path filtering)

**Research flag:** SKIP — all patterns are well-documented in official Astro and GitHub Pages docs.

---

### Phase 2: Dark Theme

**Rationale:** Theme must be established before content is written so all pages render correctly from the start. FOUC prevention requires a layout-level change that is cheap to add once and expensive to retrofit.

**Delivers:** Dark terminal aesthetic across the entire Starlight site; no white flash on load or page navigation.

**Addresses:**
- Install `@fontsource-variable/inter`
- Add `src/styles/custom.css` with `--sl-*` CSS variable overrides (near-black background, cyan accent)
- Override `--sl-font` to Inter Variable; `--sl-font-mono` to Geist Mono or system monospace
- Add blocking `is:inline` script in base layout `<head>` for FOUC prevention
- Handle `astro:after-swap` for View Transitions dark mode persistence

**Avoids:** C5 (dark mode FOUC)

**Research flag:** SKIP — CSS variable overrides are a documented Starlight public API; inline script pattern is well-documented.

---

### Phase 3: Documentation Content

**Rationale:** Documentation pages are structurally simple (MDX files in `src/content/docs/`) and unblock the landing page's addon card grid. The `kinder.dev/v1alpha4` schema from v1.0 is already finalized, unblocking the config reference immediately.

**Delivers:** Complete documentation set: installation guide, quickstart, configuration reference, and 5 addon pages — all reachable via Starlight sidebar and Pagefind search.

**Addresses (all P1 from FEATURES.md):**
- `getting-started/installation.mdx` — Linux/macOS/Windows; go install + binary download
- `getting-started/quick-start.mdx` — 5-step flow: prerequisites, install, create cluster, verify
- `configuration.mdx` — full `kinder.dev/v1alpha4` schema with all addon fields, defaults, and examples
- `addons/metallb.mdx`, `addons/envoy-gateway.mdx`, `addons/metrics-server.mdx`, `addons/coredns.mdx`, `addons/headlamp.mdx` — what each installs, what you get, how to disable, known constraints

**Feature dependency note:** Config and addon docs are immediately unblocked by v1.0 work. Installation guide depends on confirming the binary distribution method (see Gaps).

**Research flag:** SKIP for structure — file-based routing is well-documented. FLAG for content: confirm binary distribution method before finalizing the installation guide page.

---

### Phase 4: Landing Page

**Rationale:** The landing page requires all doc pages to exist first (addon cards link to them). Custom components live in `src/components/` using Starlight's CSS variables but with their own layout CSS.

**Delivers:** Full marketing landing page at `/` with hero, addon grid, and quickstart section. Distinguishes kinder from kind visually and communicates value in under 10 seconds.

**Addresses (P1 + selected P2 from FEATURES.md):**
- Hero: headline, value proposition, one-line install command with copy-to-clipboard
- Addon card grid: 5 cards with icon, name, one-liner, link to doc page
- Before/after comparison block: kind bare cluster vs kinder with addons (P2 but low cost, high value)
- Cluster config YAML example showing opt-out addon flags
- Quickstart code block (`kinder create cluster`)
- Version badge and GitHub link in nav

**Implementation note:** Use `src/pages/index.astro` overriding Starlight's root. Wrap with `<StarlightPage frontmatter={{ template: 'splash' }}>` for full-width no-sidebar layout while keeping Starlight's nav and footer.

**Research flag:** SKIP — splash template and `src/pages/index.astro` override pattern are documented in ARCHITECTURE.md with working code examples.

---

### Phase 5: Assets and Identity

**Rationale:** Favicon, og:image, and 404 page complete the site's identity. Deferred until after content so the og:image reflects the final headline and branding.

**Delivers:** Complete site identity: favicon, custom og:image for social sharing previews, styled 404 page.

**Addresses:**
- `public/favicon.svg` — dark-friendly site icon
- `public/og-image.png` (1200x630) — logo + tagline for Slack/Discord/Twitter card previews
- `src/pages/404.astro` or `public/404.html` — styled 404 with link back to home

**Research flag:** SKIP — standard static site patterns.

---

### Phase 6: Polish and Validation

**Rationale:** Final pre-launch verification against the PITFALLS.md "Looks Done But Isn't" checklist. Confirms nothing was missed in production that worked locally.

**Delivers:** Confirmed production-ready site. All pitfall checklist items verified against the live URL.

**Addresses:**
- Run full checklist from PITFALLS.md against `https://kinder.patrykgolabek.dev`
- Confirm `dist/.nojekyll`, `dist/CNAME`, `dist/assets/` all exist after build
- HTTPS enforced, no mixed content
- OG tags: paste URL in opengraph.xyz — title, description, image all correct
- Sitemap: all URLs start with `https://kinder.patrykgolabek.dev`
- Mobile responsive: custom landing page components tested on small viewport
- CI isolation: push Go change and confirm site deploy does NOT trigger; push site change and confirm Go CI does NOT trigger

**Research flag:** SKIP — verification checklist is already fully specified in PITFALLS.md.

---

### Phase Ordering Rationale

- **Infrastructure before content (Phases 1-2 before 3-6):** Deployment pitfalls are silent in local dev and only appear in production. Discovering a broken deploy after writing 8 doc pages wastes time; discovering it with a placeholder site costs nothing.
- **Theme before content (Phase 2 before Phase 3):** FOUC is a layout-level problem. Adding the blocking inline script after content is the same work, but the visual regression is harder to catch across many pages.
- **Docs before landing page (Phase 3 before Phase 4):** Addon cards on the landing page link to addon doc pages. Content first means links resolve when the landing page is built.
- **Content before identity (Phase 5 after Phase 4):** The og:image should reflect the final landing page headline and branding, which is only stable after the landing page is complete.

### Research Flags

Phases needing deeper research during planning:
- **Phase 3 (Documentation — content only):** Confirm binary distribution method before writing the installation guide. The install command in the hero depends on whether GitHub Releases binaries exist, whether `go install` is the supported path, or whether a Homebrew tap is planned. This is a product decision that unblocks the installation guide.

Phases with standard patterns (no deeper research needed):
- **Phase 1:** Official Astro + GitHub Pages docs are comprehensive. CNAME, `.nojekyll`, workflow permissions — all verified with sources.
- **Phase 2:** Starlight CSS variable system is a documented public API. FOUC inline script is a known, sourced pattern.
- **Phase 4:** Splash template and `src/pages/index.astro` override pattern are documented with working code examples in ARCHITECTURE.md.
- **Phase 5:** Standard static site asset patterns.
- **Phase 6:** Checklist already derived from PITFALLS.md research.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Versions verified against npm, GitHub releases, and official docs as of March 2026. Tailwind exclusion based on confirmed open issue. Node 22 requirement sourced to official Astro changelog. |
| Features | HIGH | Based on direct analysis of 7 reference sites (Bun, Deno, Astro, kind, Headlamp, k9s, DevSpace) plus Evil Martians dev-tool landing page study. |
| Architecture | HIGH | Official Astro and Starlight docs; existing kinder repo read directly. All code examples verified against working patterns. Anti-patterns sourced to confirmed GitHub issues and community discussions. |
| Pitfalls | HIGH | Each pitfall sourced to official GitHub Actions docs, confirmed GitHub community discussions, or open Astro/Starlight issues with confirmed reproduction steps. |

**Overall confidence:** HIGH

### Gaps to Address

- **Binary distribution method:** The install command on the landing page and in the installation guide depends on this. Options: `go install github.com/patrykgolabek/kinder@latest` (simplest, requires Go), direct binary download from GitHub Releases (broader audience), Homebrew tap (easiest UX, most setup). Confirm before Phase 3.

- **v1.1 release timing vs. site launch:** If the site goes live before a tagged release exists, the install command must reference an existing tag or commit. Coordinate site launch with a tagged release.

- **Logo asset:** Research assumes a `logo.svg` will be created for `src/assets/logo.svg`. No logo design is scoped in this research. If no logo exists, the Starlight site title text serves as a placeholder until Phase 5.

- **GitHub repo ownership:** ARCHITECTURE.md and STACK.md reference both `patrykgolabek/kinder` and `patrykattc/kinder` as the GitHub URL. Confirm the correct GitHub username before writing any docs or config files that include the install command or GitHub link.

---

## Sources

### Primary (HIGH confidence)
- Astro 5.17 official docs: https://astro.build/
- @astrojs/starlight@0.37.6 official docs: https://starlight.astro.build/
- Starlight configuration reference: https://starlight.astro.build/reference/configuration/
- Starlight CSS and theming guide: https://starlight.astro.build/guides/css-and-tailwind/
- Astro GitHub Pages deployment guide: https://docs.astro.build/en/guides/deploy/github/
- withastro/action@v5.2.0: https://github.com/withastro/action
- GitHub Pages custom domain docs: https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/
- GitHub Actions concurrency docs: https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions
- Astro issue #14247 (Jekyll `_astro` stripping): https://github.com/withastro/astro/issues/14247
- Starlight issue #3339 (.nojekyll fix): https://github.com/withastro/starlight/issues/3339
- starlight-tailwind Tailwind v4 issue (OPEN): https://github.com/withastro/starlight/issues/2862
- GitHub community: CNAME reset on deploy: https://github.com/orgs/community/discussions/48422
- Existing kinder repo read directly at `/Users/patrykattc/work/git/kinder`

### Secondary (MEDIUM confidence)
- Bun, Deno, Astro, kind, Headlamp, k9s, DevSpace landing pages — direct analysis for feature research
- Evil Martians: "We studied 100 dev tool landing pages" — partially paywalled; used for feature prioritization validation
- Astro dark mode FOUC patterns: simonporter.co.uk, danielnewton.dev — community-confirmed inline script pattern
- Astro issue #8711 (FOUC during View Transitions): https://github.com/withastro/astro/issues/8711

### Tertiary (LOW confidence)
- Astro in 2026 (sitepins.com) — used only for Cloudflare acquisition context; not used for technical decisions

---
*Research completed: 2026-03-01*
*Ready for roadmap: yes*
