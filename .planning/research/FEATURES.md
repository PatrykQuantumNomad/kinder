# Feature Research

**Domain:** CLI tool website — landing page + documentation site (Astro, GitHub Pages)
**Researched:** 2026-03-01
**Confidence:** HIGH (based on direct analysis of Bun, Deno, Astro, Headlamp, k9s, kind, DevSpace sites + Evil Martians study of 100 dev tool landing pages)

---

## Context

This research covers the **v1.1 website milestone** — not the CLI addons (v1.0 already shipped). kinder is a batteries-included kind fork. The website must explain what kinder is, why you'd use it over vanilla kind, how to install it, and how to configure its 5 addons. The audience is developers who know Kubernetes and have probably used kind before.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features a dev tool site must have. Missing these makes the site feel broken or untrustworthy.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| One-line install command in hero | Developers copy-paste this before reading anything else; it's the first signal of quality | LOW | Platform-conditional: Linux/macOS curl, Homebrew, go install. Show the most universal first. |
| Copy-to-clipboard on all code blocks | Standard expectation since 2020; not having it feels unfinished | LOW | Astro Starlight provides this for doc pages; hero page code block needs manual implementation |
| Syntax-highlighted code blocks | Raw monospace code looks amateur; highlights communicate intent | LOW | Astro has Shiki built in; Starlight uses it by default |
| Dark mode (default dark) | Developer audience expects dark mode; kinder targets dev-tool aesthetic explicitly | LOW | Starlight provides dark/light toggle; set dark as default to match project brief |
| Mobile-responsive layout | Google rankings, people reading docs on phones | LOW | Starlight is responsive by default; landing page needs explicit responsive design |
| Installation guide page | Deeplinked from hero, GitHub README, blog posts, Stack Overflow answers | LOW | Standalone `/docs/install/` page covering all platforms and methods |
| Configuration reference page | Developers paste config and need to know what every field does | MEDIUM | Documents `kinder.dev/v1alpha4` config schema with all addon fields, defaults, examples |
| Docs for each addon | Users need to know what each addon does, how to disable it, and what to expect | MEDIUM | One page per addon: MetalLB, Envoy Gateway, Metrics Server, CoreDNS Tuning, Headlamp |
| GitHub link in nav | Developers distrust tools with no visible source; stars are social proof | LOW | Link to github.com/patrykg/kinder in top nav and footer |
| Clear value proposition in hero | Visitors decide in under 10 seconds; "kind but with batteries" must be immediately obvious | LOW | Headline + subtitle must name the problem kind has (manual addon setup) and kinder's answer |
| Working anchor links in docs | Navigation within long reference pages | LOW | Starlight generates these from headings automatically |
| Page titles and meta descriptions | SEO, browser tabs, social share previews | LOW | Astro and Starlight handle this with frontmatter |
| 404 page | GitHub Pages serves a 404; custom page keeps users in the site | LOW | Add `public/404.html` or configure Astro to generate it |

### Differentiators (Competitive Advantage)

Features that set kinder's site apart from kind's plain Hugo site and generic tool docs. These are what make Bun, Deno, and Astro sites feel high quality.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Animated terminal demo in hero | Shows exactly what `kinder create cluster` output looks like; replaces abstract feature claims with reality | MEDIUM | Use asciinema embed or CSS-animated terminal component. Bun, k9s both do this effectively. Pure CSS animation is simpler and loads faster than asciinema. |
| "What you get" addon grid on landing page | Five addons are the product differentiator; calling them out visually makes the value concrete | LOW | Card grid showing each addon with icon, name, and one-line description (e.g. "LoadBalancer IPs — MetalLB v0.15.3"). Link each to its doc page. |
| Before/after comparison | Developers already know kind; "kind gives you X, kinder gives you X + these 5 things" is the fastest conversion pitch | LOW | Side-by-side comparison block: `kind create cluster` (bare cluster) vs `kinder create cluster` (what you get). No library needed, just styled HTML. |
| Platform-specific install tabs | macOS / Linux / Windows tabs so users see exactly their command without reading all variations | LOW | Simple tab component; or code block with OS detection via JavaScript. Headlamp does this well. |
| Version badge in hero or nav | Shows the project is active and tells users what they're installing | LOW | Static badge from shields.io or hardcoded version string (update on release) |
| Quickstart page separate from full install guide | Developers want the path of least resistance first; don't bury it in a long install doc | LOW | Short page: prerequisites, one install command, `kinder create cluster`, `kubectl get nodes`. Maximum 5 steps. |
| Inline feature showcase on landing page | Moves from "what it is" to "here's the actual output" within a single scroll | MEDIUM | Show terminal output for `kubectl top nodes`, `kubectl get svc` with EXTERNAL-IP populated, Headlamp screenshot |
| Cluster config file example on landing page | Kinder's opt-out config is a key DX feature; showing it early sets expectations | LOW | A short YAML block showing addons section with comments explaining each field |
| Favicon + og:image | Site identity; og:image appears in Slack/Discord/Twitter shares | LOW | Use kinder logo as favicon; create 1200x630 og:image with logo + tagline |
| Breadcrumbs in docs | Navigation orientation within documentation hierarchy | LOW | Starlight provides breadcrumbs automatically |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Blog section | Looks like a complete site; content marketing | Requires ongoing content creation; a stale blog (last post 3 months ago) signals a dead project; v1.1 scope is launch, not content | Add a blog only after cadence is established; link to GitHub releases for now |
| Newsletter/email signup | Community building | No audience to justify the friction; adds GDPR complexity; creates expectation of emails that won't get sent | Link to GitHub for release notifications (`Watch > Releases`) |
| Search bar on landing page | Seems useful | Redundant with browser find; landing page has no indexed content; complicates layout | Docs pages get search (Starlight Pagefind) automatically; landing page doesn't need it |
| Version dropdown for docs | Enterprise docs pattern (React, Python) | kinder is pre-v2; maintaining versioned docs creates ongoing overhead immediately | Single version of docs; mention version compatibility in text where needed |
| Interactive playground / sandbox | Conversion boost | kinder requires Docker; you can't spin up a real cluster in a browser sandbox; a fake demo breaks trust | Asciinema recording or animated terminal component showing real output |
| Dark/light mode toggle in hero section | User preference | Adds implementation complexity; kinder targets a developer-tool aesthetic with dark default | Provide Starlight's built-in toggle in docs; keep landing page dark by default |
| Changelog page separate from GitHub releases | Keeps users on site | Duplicates GitHub releases; creates maintenance burden (updating two places) | Link directly to GitHub releases page; possibly embed latest release info via static build step |
| Comment system on docs | Community engagement | Docs pages aren't blog posts; comments add noise and spam risk | Link to GitHub Discussions or GitHub Issues for feedback |
| Cookie consent banner | Legal compliance | Heavy-handed for a GitHub Pages static site with no analytics; breaks the clean aesthetic | Don't add analytics. If you do add analytics, use Plausible (privacy-preserving, no cookie banner needed) |
| Auto-redirects based on OS detection | Personalization | JavaScript-dependent; degrades on curl/wget; confusing when not on expected OS | Show platform tabs that default to the most common platform (Linux/macOS) and let users click |

---

## Feature Dependencies

```
Landing Page Hero
    └──requires──> Install command finalized (can't show command before release binary exists)
    └──requires──> Addon descriptions (for the addon grid)
    └──enhances via──> Animated terminal demo (optional, adds significantly to conversion)

Docs: Installation Guide
    └──requires──> Binary distribution method decided (GitHub Releases, Homebrew tap, go install)
    └──requires──> Platform support matrix confirmed (Linux amd64, arm64, macOS, Windows?)

Docs: Configuration Reference
    └──requires──> v1alpha4 config schema finalized (already done in v1.0)
    └──is required by──> Each addon doc page (config field for that addon lives here)

Docs: Addon Pages (×5)
    └──requires──> Configuration Reference (cross-reference config fields)
    └──requires──> Installation Guide (prerequisite page in user journey)

GitHub Pages Deployment
    └──requires──> CNAME file at public/CNAME with kinder.patrykgolabek.dev
    └──requires──> DNS: CNAME record pointing to patrykattc.github.io
    └──requires──> GitHub Actions workflow (.github/workflows/deploy.yml using withastro/action)
    └──requires──> astro.config.mjs: site = "https://kinder.patrykgolabek.dev" (no base path)

Starlight Docs Site
    └──enhances──> Landing Page (search, nav, all doc pages)
    └──requires──> Content in MDX/Markdown files under src/content/docs/

Addon Grid (landing page)
    └──enhances──> Docs: each addon page (click-through from cards)
```

### Dependency Notes

- **Install command requires binary distribution:** The landing page hero cannot finalize its install command until the release pipeline (GitHub Releases or Homebrew tap) is in place. If releasing via GitHub Releases, the install command is `curl ... | sh` or direct binary download link. If releasing via go install, that's simpler but requires users to have Go.
- **Config reference requires schema finalized:** The v1.0 work already finalized the schema (`kinder.dev/v1alpha4` with addons section). This unblocks the config reference page immediately.
- **Custom domain requires DNS config before site goes live:** The CNAME file deploys with the site but DNS propagation takes time. Set DNS before the first deployment.

---

## MVP Definition

### Launch With (v1.1)

The minimum site that replaces a blank GitHub Pages URL and gives users enough to evaluate and install kinder.

- [ ] Landing page with hero, value proposition, one-line install command, addon grid — essential for first impressions
- [ ] Installation guide doc page (Linux/macOS/Windows, go install + binary download) — without this, users cannot install the tool
- [ ] Configuration reference doc page (all v1alpha4 fields, addon flags, defaults) — without this, power users cannot self-serve
- [ ] One doc page per addon ×5 (what it installs, what you get, how to disable, known constraints) — completes the documentation set for launched features
- [ ] Quickstart page (5-step flow from zero to working cluster) — conversion funnel; users need an easy path
- [ ] GitHub Actions deploy workflow to GitHub Pages with CNAME for kinder.patrykgolabek.dev — gets the site live
- [ ] Favicon and og:image — site identity; og:image appears in every share
- [ ] Custom 404 page — avoids GitHub's default 404 breaking the site experience

### Add After Validation (v1.x)

- [ ] Animated terminal demo in hero — add when basic conversion data suggests users aren't understanding the value proposition
- [ ] Before/after comparison block — add if analytics show high bounce rate from landing page
- [ ] Platform-specific install tabs (if multi-platform binary distribution is added) — add when Homebrew tap or additional platform binaries are released

### Future Consideration (v2+)

- [ ] Blog section — only after regular content cadence is established
- [ ] Versioned docs — only when a breaking change between kinder versions requires it
- [ ] Changelog page — only if GitHub releases become hard to discover

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Landing page hero + install command | HIGH | LOW | P1 |
| Addon grid on landing page | HIGH | LOW | P1 |
| Installation guide doc page | HIGH | LOW | P1 |
| Quickstart doc page | HIGH | LOW | P1 |
| Configuration reference doc page | HIGH | MEDIUM | P1 |
| Addon doc pages (×5) | HIGH | MEDIUM | P1 |
| GitHub Actions deploy + CNAME | HIGH | LOW | P1 |
| Dark mode (Starlight default) | MEDIUM | LOW | P1 (built in) |
| Copy-to-clipboard code blocks | MEDIUM | LOW | P1 (built in) |
| Favicon + og:image | MEDIUM | LOW | P1 |
| Custom 404 page | LOW | LOW | P1 |
| Before/after comparison block | MEDIUM | LOW | P2 |
| Platform-specific install tabs | MEDIUM | LOW | P2 |
| Animated terminal demo | HIGH | MEDIUM | P2 |
| Version badge | LOW | LOW | P2 |
| Blog section | LOW | MEDIUM | P3 |
| Versioned docs | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch
- P2: Should have, add when possible (v1.1.x)
- P3: Nice to have, future consideration

---

## Competitor Feature Analysis

Reference sites studied: Bun (bun.com), Deno (deno.com), Astro (astro.build), kind (kind.sigs.k8s.io), Headlamp (headlamp.dev), k9s (k9scli.io), DevSpace (devspace.sh).

| Feature | Bun / Deno | kind (parent project) | Our Approach |
|---------|------------|-----------------------|--------------|
| Hero headline | Action-oriented, benefit-led ("A fast all-in-one JS runtime") | Descriptive only ("tool for running local Kubernetes clusters") | Benefit-led: "Local Kubernetes clusters, batteries included" or similar |
| Install command in hero | Yes (prominent, copy-to-clipboard) | Yes (in body text, not hero) | Yes, in hero, copy-to-clipboard |
| Performance benchmarks | Central to Bun/Deno pitch (10x faster) | N/A | Not applicable to kinder; kinder's value is DX, not speed |
| Addon/feature grid | Feature cards | Bullet list in README-style | Visual card grid on landing page |
| Dark mode | Yes | No (light only) | Yes, dark default |
| Animated terminal / asciinema | Bun has code tabs; Deno has terminal-style output | No | Target: CSS-animated terminal block in landing page hero |
| Documentation search | Yes (Bun uses Algolia; Deno has built-in search) | No search | Starlight Pagefind (built-in, no API key) |
| Sidebar navigation in docs | Yes | Yes | Yes (Starlight auto-generates from file structure) |
| Separate quickstart vs full docs | Yes (both) | Quick Start is one page | Yes, short quickstart + full reference separate |
| Before/after comparison | Deno does this (Deno vs Node) | No | Yes (kinder vs kind) |
| GitHub stars displayed | Bun: displayed prominently | No | Optional; add after star count is worth showing |

---

## Sources

- [Evil Martians: We studied 100 dev tool landing pages](https://evilmartians.com/chronicles/we-studied-100-devtool-landing-pages-here-is-what-actually-works-in-2025) — MEDIUM confidence (paywalled content partially accessible)
- [Bun landing page](https://bun.com) — HIGH confidence (direct analysis)
- [Deno landing page](https://deno.com) — HIGH confidence (direct analysis; includes install command, benchmarks, feature sections, CTAs)
- [Astro landing page](https://astro.build) — HIGH confidence (direct analysis; Islands architecture showcase, framework integration grid, social proof)
- [kind website](https://kind.sigs.k8s.io/) — HIGH confidence (direct analysis; baseline comparison — plain Hugo, no visual hierarchy, no dark mode)
- [Headlamp landing page](https://headlamp.dev) — HIGH confidence (direct analysis; multi-platform install commands, feature cards, no screenshots — notable gap)
- [k9s website](https://k9scli.io) — HIGH confidence (direct analysis; terminal preview/asciinema, feature list, sponsorship CTA)
- [DevSpace landing page](https://devspace.sh) — HIGH confidence (direct analysis; Kubernetes dev tool, 5-reason feature showcase, CLI command examples)
- [Astro Starlight documentation](https://starlight.astro.build) — HIGH confidence (official; lists all built-in features: search, nav, dark mode, code blocks, SEO, i18n)
- [Astro GitHub Pages deployment guide](https://docs.astro.build/en/guides/deploy/github/) — HIGH confidence (official Astro docs; CNAME, workflow, astro.config.mjs settings)
- [Starlight vs Docusaurus comparison](https://blog.logrocket.com/starlight-vs-docusaurus-building-documentation/) — MEDIUM confidence (community blog, verified against Starlight docs)
- [Astro in 2026](https://sitepins.com/blog/astro-sitepins-2026) — LOW confidence (community blog, used only for Cloudflare acquisition context)

---
*Feature research for: kinder website (v1.1 milestone)*
*Researched: 2026-03-01*
