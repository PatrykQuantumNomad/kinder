# Architecture Research

**Domain:** Static documentation + landing page site integrated into Go CLI monorepo
**Researched:** 2026-03-01
**Confidence:** HIGH (Astro/Starlight official docs verified; GitHub Actions pattern verified; existing repo read directly)

---

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     kinder GitHub repo (main)                    │
├────────────────────────────┬────────────────────────────────────┤
│       Go CLI (existing)    │       Website (new)                 │
│  ┌─────────────────────┐  │  ┌────────────────────────────┐    │
│  │  cmd/kinder/        │  │  │  kinder-site/              │    │
│  │  pkg/cluster/...    │  │  │  ├── src/                  │    │
│  │  internal/actions/  │  │  │  │   ├── content/docs/     │    │
│  │  Makefile           │  │  │  │   ├── components/       │    │
│  └─────────────────────┘  │  │  │   └── assets/           │    │
│                            │  │  ├── public/               │    │
│  (Go build, no changes)   │  │  │   └── CNAME             │    │
│                            │  │  ├── astro.config.mjs      │    │
│                            │  │  └── package.json          │    │
│                            │  └────────────────────────────┘    │
├────────────────────────────┴────────────────────────────────────┤
│                     GitHub Actions CI                            │
│  ┌──────────────────┐  ┌─────────────────────────────────────┐  │
│  │  Go test/build   │  │  Astro build + GitHub Pages deploy  │  │
│  │  (unchanged)     │  │  (new workflow, path: kinder-site/) │  │
│  └──────────────────┘  └─────────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│                     GitHub Pages                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  kinder.patrykgolabek.dev  (custom CNAME)               │    │
│  │  Landing page (/)  +  Docs (/docs/...)                  │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Implementation |
|-----------|----------------|----------------|
| `kinder-site/` | Entire website source; fully self-contained Node project | Astro 5 + Starlight 0.37.x |
| `src/content/docs/` | All documentation pages as Markdown/MDX | Starlight content collection; auto-generates sidebar |
| `src/pages/index.astro` | Custom landing page (splash template, no sidebar) | Astro `.astro` page outside the docs collection |
| `src/components/` | Reusable Astro components for landing sections | Hero, FeatureGrid, AddonCard, QuickStart |
| `src/assets/` | Images, logo — processed by Astro's image pipeline | SVG logo, hero screenshots |
| `public/CNAME` | Custom domain declaration for GitHub Pages | Single line: `kinder.patrykgolabek.dev` |
| `astro.config.mjs` | Starlight integration config, site URL, sidebar, social links | Root config for entire site |
| `.github/workflows/deploy-site.yml` | Build Astro and deploy to GitHub Pages | `withastro/action@v5` with `path: ./kinder-site` |

---

## Recommended Project Structure

```
kinder-site/
├── astro.config.mjs          # Site config: title, social links, sidebar, customCss
├── package.json              # Dependencies: astro, @astrojs/starlight
├── package-lock.json         # Lockfile committed (required by withastro/action)
├── tsconfig.json             # TypeScript config (Astro scaffold provides this)
│
├── public/
│   ├── CNAME                 # "kinder.patrykgolabek.dev" — required for custom domain
│   └── favicon.svg           # Site icon (dark-friendly)
│
└── src/
    ├── content.config.ts     # Content Layer schema: extends docsSchema()
    │
    ├── content/
    │   └── docs/             # Starlight maps every file here to a URL
    │       ├── index.mdx     # Redirect to /docs/getting-started/ (or remove if landing is separate)
    │       ├── getting-started.mdx
    │       ├── configuration.mdx
    │       └── addons/
    │           ├── overview.mdx
    │           ├── metallb.mdx
    │           ├── envoy-gateway.mdx
    │           ├── metrics-server.mdx
    │           ├── coredns.mdx
    │           └── headlamp.mdx
    │
    ├── pages/
    │   └── index.astro       # Landing page — overrides Starlight's root page
    │
    ├── components/
    │   ├── Hero.astro         # Full-width hero: headline + install command
    │   ├── FeatureGrid.astro  # 3-column grid of addon feature cards
    │   ├── AddonCard.astro    # Individual card: icon, name, one-liner
    │   ├── QuickStart.astro   # Tabbed code block: install + create cluster
    │   └── GithubBadge.astro  # Stars/version badge linking to GitHub
    │
    └── assets/
        ├── logo.svg           # Kinder logo (light + dark safe)
        └── hero-terminal.png  # Terminal screenshot showing kinder create cluster
```

### Structure Rationale

- **`kinder-site/` at repo root:** Keeps Go codebase completely separate. Go tooling ignores `kinder-site/` entirely; Node tooling starts from `kinder-site/`. No workspace or symlink hacks needed.
- **`src/content/docs/`:** Starlight's required path. Everything here becomes a documentation page automatically with sidebar, breadcrumbs, search, and prev/next navigation.
- **`src/pages/index.astro`:** Landing pages need no sidebar and a custom layout. Placing a real `.astro` file at `src/pages/index.astro` overrides Starlight's generated root page, giving full layout control while keeping the docs integration intact.
- **`public/CNAME`:** The `withastro/action` copies the `public/` directory verbatim into the build output. GitHub Pages reads `CNAME` from the root of the deployed artifact to configure the custom domain.
- **`package-lock.json` committed:** `withastro/action` requires a lockfile to determine which package manager to use and to enable caching. Without it, the action falls back to a slower uncached install.

---

## Architectural Patterns

### Pattern 1: Starlight Integration (Docs) with Custom Landing Page

**What:** Use Starlight for all documentation pages while adding a completely custom `src/pages/index.astro` landing page. Starlight handles `/docs/**` routes via content collections; the landing page handles `/`.

**When to use:** Any site that needs both a marketing landing page (no sidebar, custom layout) and structured documentation (sidebar, search, prev/next). This is the standard pattern for developer tools.

**Trade-offs:** Starlight controls its own layout for docs pages. Custom component injection requires Starlight's override mechanism (`components:` in the Starlight config), which adds complexity for non-trivial customizations. For kinder, basic CSS variable overrides are sufficient — avoid the override mechanism for the initial build.

**Example `astro.config.mjs`:**
```javascript
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  // No `base` needed when using a custom domain at apex of subdomain
  integrations: [
    starlight({
      title: 'kinder',
      description: 'Batteries-included local Kubernetes clusters',
      logo: { src: './src/assets/logo.svg' },
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/patrykattc/kinder' },
      ],
      customCss: ['./src/custom.css'],
      defaultLocale: 'en',
      sidebar: [
        { label: 'Getting Started', link: '/docs/getting-started/' },
        { label: 'Configuration', link: '/docs/configuration/' },
        {
          label: 'Addons',
          autogenerate: { directory: 'addons' },
        },
      ],
    }),
  ],
});
```

**Example `src/content.config.ts`:**
```typescript
import { defineCollection } from 'astro:content';
import { docsSchema } from '@astrojs/starlight/schema';

export const collections = {
  docs: defineCollection({ schema: docsSchema() }),
};
```

### Pattern 2: CSS Variable Override for Dark Terminal Theme

**What:** Override Starlight's CSS custom properties in a `customCss` file to match a dark developer-tool aesthetic. Starlight already defaults to dark mode; the overrides tune colors to a terminal-inspired palette (dark backgrounds, green/cyan accents, monospace code).

**When to use:** When Starlight's default Night Owl palette does not match the product's brand. For kinder, the target is a darker, more muted palette similar to tools like `kind`'s docs or `flux`'s site.

**Trade-offs:** CSS variable overrides are stable across Starlight versions (they are a documented public API). Overriding component templates via Starlight's `components:` prop is less stable and should be avoided unless absolutely necessary.

**Example `src/custom.css`:**
```css
/* Dark terminal theme for kinder */
:root {
  --sl-color-accent-low: #0d2b2b;
  --sl-color-accent: #00b4b4;       /* cyan — Kubernetes brand adjacent */
  --sl-color-accent-high: #7fffee;
  --sl-color-white: #f4f4f5;
  --sl-color-gray-1: #eaeaeb;
  --sl-color-gray-6: #111318;       /* near-black background */
  --sl-font-system-mono: 'JetBrains Mono', 'Fira Code', monospace;
}

/* Force dark background even on light system preference */
:root[data-theme='light'] {
  /* intentionally close to dark — kinder is a dev tool, dark-first */
  --sl-color-bg: #1a1d24;
  --sl-color-bg-nav: #111318;
}
```

### Pattern 3: Static Code Blocks for Install Instructions

**What:** Use Starlight's built-in Expressive Code syntax highlighting for the install commands. Place the most important command (the quickstart) prominently on the landing page using a custom `QuickStart.astro` component, not inside a docs page.

**When to use:** Landing pages need scannable install commands. Docs pages use standard Markdown code fences.

**Trade-offs:** Custom components in `src/components/` are not automatically styled by Starlight. They inherit global CSS variables but require their own layout CSS. Keep landing page components simple — flexbox/grid, no complex JS.

**Example landing page snippet:**
```astro
---
// src/pages/index.astro
import StarlightPage from '@astrojs/starlight/components/StarlightPage.astro';
import Hero from '../components/Hero.astro';
import FeatureGrid from '../components/FeatureGrid.astro';
---
<StarlightPage frontmatter={{ title: 'kinder', template: 'splash' }}>
  <Hero />
  <FeatureGrid />
</StarlightPage>
```

### Pattern 4: Content Collections with Autogenerated Sidebar

**What:** Place all addon docs in `src/content/docs/addons/` and use `autogenerate: { directory: 'addons' }` in the sidebar config. Starlight generates sidebar entries alphabetically by filename; use numeric prefixes (`01-metallb.mdx`) or the `sidebar.order` frontmatter field to control order.

**When to use:** Any section with multiple related pages. Avoids manually maintaining a sidebar list.

**Trade-offs:** Autogenerated labels default to the filename (with capitalization). Override with `sidebar.label` in page frontmatter to get clean display names.

**Example frontmatter in `src/content/docs/addons/metallb.mdx`:**
```yaml
---
title: MetalLB
description: LoadBalancer support for kinder clusters via Layer 2 ARP
sidebar:
  label: MetalLB
  order: 1
---
```

---

## Data Flow

### Build-Time Flow (GitHub Actions)

```
git push to main
    |
    v
.github/workflows/deploy-site.yml triggers
    |
    v
actions/checkout@v5  (full repo checkout)
    |
    v
withastro/action@v5
    path: ./kinder-site         <- tells action where Astro project lives
    node-version: 22            <- default
    |
    v
npm install                     <- installs astro + @astrojs/starlight
    |
    v
astro build                     <- reads astro.config.mjs
    |
    +-- reads src/content/docs/**   -> generates HTML for each doc page
    +-- reads src/pages/index.astro -> generates landing page HTML
    +-- processes src/assets/       -> optimized images
    +-- copies public/CNAME         -> into dist/
    |
    v
dist/                           <- static site output
    |
    v
actions/deploy-pages@v4         <- uploads dist/ as GitHub Pages artifact
    |
    v
GitHub Pages serves at kinder.patrykgolabek.dev
```

### Request Flow (End User)

```
Browser: https://kinder.patrykgolabek.dev/
    |
    v
GitHub Pages CDN (cached static files)
    |
    v
Landing page HTML (index.astro compiled output)
    -> Hero component: headline, install command, GitHub link
    -> FeatureGrid: 5 addon cards
    -> QuickStart: code block with kinder create cluster

Browser: https://kinder.patrykgolabek.dev/docs/getting-started/
    |
    v
GitHub Pages CDN
    |
    v
Starlight-generated HTML
    -> Left sidebar: navigation tree
    -> Content: getting-started.mdx rendered
    -> Right sidebar: table of contents
    -> Footer: prev/next pagination
```

### Go CLI / Website Data Boundary

There is intentionally no runtime coupling between the Go CLI and the website. The website is pure static HTML. The only data that crosses the boundary is:

| Data | Direction | Mechanism |
|------|-----------|-----------|
| CLI version string | Go CLI -> website | Hard-coded in docs frontmatter; updated manually on release |
| Addon version numbers | Go CLI -> website | Hard-coded in addon docs; updated manually on release |
| Install instructions | Human -> website | Authored in MDX once, updated on CLI changes |

No dynamic API calls. No version auto-detection from the binary. This is correct for a static site: keep complexity low, update docs as part of the release process.

---

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 0-1k monthly visitors | GitHub Pages free tier is more than sufficient; no CDN tuning needed |
| 1k-100k monthly visitors | GitHub Pages handles this without any changes (served via Fastly CDN) |
| 100k+ monthly visitors | Consider Cloudflare proxy in front of GitHub Pages for analytics and caching control; no site architecture change needed |

This is a developer tool docs site. Traffic will be dominated by organic search and GitHub referrals. GitHub Pages free tier supports unlimited bandwidth for public repos. No scaling concerns exist for this project.

---

## Anti-Patterns

### Anti-Pattern 1: Reusing the Existing `site/` Directory

**What people do:** Modify the existing `site/` directory (Hugo-based, kind's site) to add kinder branding.

**Why it's wrong:** The `site/` directory is a Hugo project targeting kind's upstream website. It has kind's config, kind's theme, kind's menus. Mixing kinder content in creates maintenance confusion, makes upgrades from upstream harder, and produces a site that looks like kind's site with kinder content.

**Do this instead:** Create a completely separate `kinder-site/` directory at the repo root. The existing `site/` directory can be removed or left as-is — it is unused by kinder's build.

### Anti-Pattern 2: Setting `base` When Using a Custom Domain

**What people do:** Set `base: '/kinder'` in `astro.config.mjs` because they assume the site lives at a sub-path.

**Why it's wrong:** When a custom domain (`kinder.patrykgolabek.dev`) is configured, the site is at the root of that domain, not at a sub-path. Setting `base` causes all internal links and asset paths to be prefixed with `/kinder`, breaking navigation.

**Do this instead:** Set only `site: 'https://kinder.patrykgolabek.dev'` and do not set `base`. The `public/CNAME` file handles domain association with GitHub Pages.

### Anti-Pattern 3: Checking In `node_modules/`

**What people do:** Commit `node_modules/` to avoid the install step in CI.

**Why it's wrong:** Node modules are platform-specific binaries + thousands of files. They bloat the repo, slow down clones, and cause cache invalidation issues.

**Do this instead:** Commit `package-lock.json`. The `withastro/action` uses it to restore the npm cache between runs, making subsequent CI builds fast without committing node_modules.

### Anti-Pattern 4: Embedding the Website in Go's Build System

**What people do:** Add a `make site` target that shells out to `npm run build` inside the repo's root Makefile, using the Go-managed `PATH` setup from `hack/build/setup-go.sh`.

**Why it's wrong:** Node and Go have entirely different toolchain management. Mixing them in the same Makefile creates confusion about which PATH is active, what versions of tools are available, and what CI jobs are responsible for what. The existing Makefile uses `hack/build/setup-go.sh` to configure Go's toolchain — injecting `npm` into this path is fragile.

**Do this instead:** The website lives in `kinder-site/` with its own `package.json`. Developers who want to work on the site `cd kinder-site && npm install && npm run dev`. GitHub Actions uses the `withastro/action` with `path: ./kinder-site`. No Makefile integration needed.

### Anti-Pattern 5: Using Starlight's Component Override System for Minor Styling

**What people do:** Override Starlight's built-in components (Header, Sidebar, Footer) by creating local copies in `src/components/starlight/` and wiring them in via `components:` in the Starlight config.

**Why it's wrong:** Overriding components is a copy-paste of Starlight's internal implementation. When Starlight releases updates (bug fixes, accessibility improvements), the overridden components don't get the fixes. Minor styling doesn't justify the maintenance cost.

**Do this instead:** Use CSS variable overrides in `customCss`. They are a documented, stable API. Only use component overrides for structural changes that CSS cannot achieve.

---

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| GitHub Pages | `withastro/action@v5` in deploy workflow | `path: ./kinder-site` for subdirectory; `public/CNAME` for custom domain |
| DNS (custom domain) | CNAME record: `kinder.patrykgolabek.dev -> patrykattc.github.io` | Set in DNS provider; GitHub Pages Settings > Custom Domain; can take hours to propagate |
| GitHub (social link) | `social:` in Starlight config | Icon link in site header to the repo |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| Go CLI source ↔ website docs | None at runtime; manual sync at release time | Addon versions and CLI flags documented in MDX; update docs as part of the release process |
| `kinder-site/` ↔ root Makefile | None — fully decoupled | No make target needed; Go CI workflow explicitly ignores `kinder-site/**` via `paths-ignore` |
| Existing Go CI workflows ↔ deploy-site workflow | None — independent triggers | Go CI: `paths-ignore: ['kinder-site/**']`; Site CI: `paths: ['kinder-site/**']` |

### New Files in Repo Root (not in `kinder-site/`)

| File | Purpose |
|------|---------|
| `.github/workflows/deploy-site.yml` | Build + deploy Astro site to GitHub Pages |

The existing Go CI workflows (`docker.yaml`, `podman.yml`, `nerdctl.yaml`, `vm.yaml`) already ignore the `site/` path. They should also ignore `kinder-site/**` to avoid triggering Go test runs when only docs are changed.

---

## GitHub Actions Workflow

Complete workflow for reference. Place at `.github/workflows/deploy-site.yml`:

```yaml
name: Deploy kinder site

on:
  push:
    branches: [main]
    paths:
      - 'kinder-site/**'
  workflow_dispatch:

permissions:
  contents: read
  pages: write
  id-token: write

concurrency:
  group: pages
  cancel-in-progress: false

jobs:
  build:
    name: Build
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Install, build, and upload site
        uses: withastro/action@v5
        with:
          path: ./kinder-site
          node-version: 22

  deploy:
    name: Deploy
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
```

**Path trigger:** Only runs when files under `kinder-site/` change. Go source changes do not trigger a site deploy.

---

## Build Order for Implementation Phases

The following order respects dependencies and allows incremental validation:

```
Phase 1: Scaffold
    scaffold kinder-site/ with Starlight template
    configure astro.config.mjs (title, social, sidebar structure)
    add public/CNAME for custom domain
    verify: npm run dev shows Starlight default site locally

Phase 2: Deploy pipeline
    add .github/workflows/deploy-site.yml
    configure GitHub Pages: Source = GitHub Actions, Custom Domain
    configure DNS: CNAME record
    verify: push to main deploys placeholder site to kinder.patrykgolabek.dev

Phase 3: Dark theme
    add src/custom.css with CSS variable overrides
    verify: dark terminal aesthetic in local dev and deployed

Phase 4: Documentation pages
    write src/content/docs/getting-started.mdx
    write src/content/docs/configuration.mdx
    write src/content/docs/addons/*.mdx (one per addon)
    verify: sidebar shows all pages; search works; prev/next navigation works

Phase 5: Landing page
    implement src/pages/index.astro
    implement src/components/ (Hero, FeatureGrid, AddonCard, QuickStart)
    verify: / renders landing page without sidebar; /docs/* still renders with sidebar

Phase 6: Polish
    add logo to Starlight config
    add favicon
    verify: mobile responsive; Lighthouse score acceptable
```

**Why deploy pipeline before content:** Validates the deployment plumbing early. If GitHub Pages configuration or DNS is broken, finding it in Phase 2 is much cheaper than discovering it after writing all the content.

---

## Sources

- [Starlight Project Structure](https://starlight.astro.build/guides/project-structure/) — HIGH confidence (official Starlight docs)
- [Starlight Getting Started](https://starlight.astro.build/getting-started/) — HIGH confidence (official)
- [Starlight Configuration Reference](https://starlight.astro.build/reference/configuration/) — HIGH confidence (official)
- [Starlight Customization Guide](https://starlight.astro.build/guides/customization/) — HIGH confidence (official)
- [Starlight Sidebar Navigation](https://starlight.astro.build/guides/sidebar/) — HIGH confidence (official)
- [Astro Deploy to GitHub Pages](https://docs.astro.build/en/guides/deploy/github/) — HIGH confidence (official Astro docs)
- [withastro/action GitHub README](https://github.com/withastro/action) — HIGH confidence (official action repo, verified `path` parameter)
- [Astro content collections](https://docs.astro.build/en/guides/content-collections/) — HIGH confidence (official)
- [Starlight 0.37.6 release](https://github.com/withastro/starlight/releases) — HIGH confidence (verified latest version)
- [Astro 5.x on npm](https://www.npmjs.com/package/astro) — MEDIUM confidence (npm registry, version 5.16.6 current as of 2026-03)
- Existing kinder repo read directly at `/Users/patrykattc/work/git/kinder` — HIGH confidence

---
*Architecture research for: Astro/Starlight website integration into kinder Go CLI repo*
*Researched: 2026-03-01*
