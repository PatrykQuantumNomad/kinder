# Technology Stack: Kinder Website

**Project:** kinder website — landing page + docs at kinder.patrykgolabek.dev
**Milestone:** v1.1 Website (new work only — Go/kind stack unchanged)
**Researched:** 2026-03-01
**Confidence:** HIGH (versions verified against npm, GitHub releases, and official docs)

---

## Context: What This Stack Is For

The existing kinder codebase is Go. The website is an entirely separate artifact in `kinder-site/` with no Go dependencies. This document covers only the website stack additions. Do not change Go build tooling, go.mod, or the existing `site/` directory (Hugo-based, kind's original site — leave it alone).

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Astro | 5.x (currently 5.17) | Site framework | Zero-JS-by-default content framework; 62% of sites with good Core Web Vitals use it. Static output with GitHub Pages adapter. Starlight is built on it. |
| @astrojs/starlight | 0.37.x (currently 0.37.6) | Docs theme + site structure | Purpose-built docs framework: built-in search (Pagefind), sidebar nav, dark mode, MDX, syntax highlighting (Expressive Code), i18n. Single install covers all doc needs. |
| TypeScript | bundled with Astro | Type safety | Astro ships `strict` tsconfig presets. Use `astro/tsconfigs/strict`. No separate install. |
| Node.js | 22.x LTS | Runtime | Astro 5.8+ dropped Node 18. Node 22 is the current LTS. withastro/action@v5 defaults to Node 22. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| @fontsource-variable/inter | ^5.x | Variable font, self-hosted | Use for body text. Self-hosting avoids Google Fonts GDPR exposure and improves load time. |
| @fontsource/geist-mono | ^1.x | Monospace font, self-hosted | Use for code blocks. Matches the aesthetic of Bun/Deno/Linear developer sites. Only install if Expressive Code can't be themed to use system mono — for most cases, CSS `font-family` override in Starlight custom CSS is sufficient. |

**Note on Tailwind:** Do NOT add Tailwind to the Starlight site. Starlight's Tailwind integration (`@astrojs/starlight-tailwind`) does not yet have stable Tailwind v4 support (the issue is open as of March 2026 and Tailwind changed its plugin architecture). Starlight has its own CSS custom property system (`--sl-*` variables) that is simpler and sufficient for the dark theme customization needed. Adding Tailwind introduces an unstable integration layer for no meaningful gain.

### Dark Theme Approach

Starlight ships with dark mode built in (toggle and `prefers-color-scheme` detection). The "modern dark developer tool" aesthetic (like Bun, Deno) requires only CSS custom property overrides in a custom CSS file — no additional library.

Key `--sl-color-*` CSS variables to override for dark terminal aesthetic:

```css
/* src/styles/custom.css */
:root[data-theme='dark'] {
  --sl-color-accent:        #00e5ff;   /* cyan accent — links, active items */
  --sl-color-accent-high:   #00b8cc;
  --sl-color-accent-low:    #003a42;
  --sl-color-bg:            #0d0f12;   /* near-black background */
  --sl-color-bg-nav:        #0d0f12;
  --sl-color-bg-sidebar:    #111318;
  --sl-color-hairline:      #1e2128;
  --sl-font:                'Inter Variable', ui-sans-serif, system-ui, sans-serif;
  --sl-font-mono:           'Geist Mono', ui-monospace, 'Cascadia Code', monospace;
}
```

Reference this file in `astro.config.mjs` via `customCss: ['./src/styles/custom.css']`.

### Deployment and CI/CD

| Tool | Version | Purpose | Why |
|------|---------|---------|-----|
| withastro/action | v5 (v5.2.0) | GitHub Actions — build Astro site | Official Astro action; detects package manager from lockfile; supports `path:` for subdirectory projects. |
| actions/deploy-pages | v4 | GitHub Actions — publish to GitHub Pages | Pairs with withastro/action; handles Pages environment and URL output. |
| actions/checkout | v4+ | GitHub Actions — checkout repo | Standard; use latest stable. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| npm | Package manager | Default; withastro/action auto-detects from `package-lock.json`. Do not use pnpm unless you commit `pnpm-lock.yaml` — the action detects the PM from the lockfile. npm is simpler for a small site with no workspace requirements. |
| Expressive Code | Syntax highlighting | Bundled inside Starlight — no separate install. Default theme is Night Owl (dark+light pair). Override via `expressiveCode` in `astro.config.mjs` to use `github-dark` or `dracula`. |
| Pagefind | Full-text search | Bundled inside Starlight — no separate install. Zero-config. Runs as part of the build, indexes all docs content. |

---

## Installation

```bash
# Bootstrap new Astro + Starlight project in kinder-site/
npm create astro@latest kinder-site -- --template starlight

cd kinder-site

# Self-hosted fonts (optional but recommended)
npm install @fontsource-variable/inter

# That's it — Astro, Starlight, TypeScript, Expressive Code, Pagefind
# are all included in the create-astro template
```

**What you get from the Starlight template (no additional installs):**
- Astro 5.x
- @astrojs/starlight latest
- Expressive Code (syntax highlighting)
- Pagefind (search)
- TypeScript config (`astro/tsconfigs/strict`)
- File-based routing (`src/content/docs/`)
- Dark/light mode toggle

---

## GitHub Pages Configuration

### `kinder-site/astro.config.mjs`

```javascript
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  // No `base` needed — custom domain, not a github.io/repo-name path
  integrations: [
    starlight({
      title: 'kinder',
      description: 'Batteries-included local Kubernetes clusters',
      customCss: ['./src/styles/custom.css'],
      social: [
        { label: 'GitHub', icon: 'github', href: 'https://github.com/patrykgolabek/kinder' },
      ],
      logo: {
        src: './src/assets/logo.svg',
      },
      editLink: {
        baseUrl: 'https://github.com/patrykgolabek/kinder/edit/main/kinder-site/',
      },
      sidebar: [
        { label: 'Getting Started', items: [
          { label: 'Installation', slug: 'getting-started/installation' },
          { label: 'Quick Start', slug: 'getting-started/quick-start' },
        ]},
        { label: 'Configuration', autogenerate: { directory: 'configuration' }},
        { label: 'Addons', autogenerate: { directory: 'addons' }},
      ],
    }),
  ],
});
```

### `kinder-site/public/CNAME`

```
kinder.patrykgolabek.dev
```

This single file in `public/` is deployed to the root of the GitHub Pages site. GitHub reads it to serve the site at the custom domain. No other GitHub configuration file is needed.

### DNS Record (at your domain registrar)

```
Type:  CNAME
Host:  kinder
Value: patrykgolabek.github.io
TTL:   3600
```

**Note:** Add the custom domain in GitHub repo Settings > Pages > Custom domain BEFORE pushing the CNAME file, or GitHub will overwrite it. After DNS propagation (up to 24h), enable "Enforce HTTPS" in the same settings panel.

### `.github/workflows/deploy-site.yml`

```yaml
name: Deploy kinder website

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
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      - name: Build and upload
        uses: withastro/action@v5
        with:
          path: kinder-site/

  deploy:
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

**Key details:**
- `paths: ['kinder-site/**']` — only triggers on website changes, not Go code changes
- `path: kinder-site/` in `withastro/action` — tells the action where the Astro project root is
- `concurrency.cancel-in-progress: false` — prevents a partial deploy from leaving Pages broken
- The action auto-detects `npm` from `package-lock.json` and uses Node 22 by default

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| Astro + Starlight | Docusaurus (React) | Only if you need React component library compatibility across docs and app; Starlight is faster and simpler for a pure docs+landing use case |
| Astro + Starlight | VitePress | If the project is entirely docs with no landing page; VitePress has less customization flexibility for a distinct landing page |
| Astro + Starlight | Hugo (existing site/) | Hugo is already in the repo for kind's site. Do not reuse it — it's kind's identity, not kinder's. Fresh Astro start gives a distinct visual identity. |
| Astro + Starlight | Next.js | If you need SSR, API routes, or React app features; pure marketing/docs site doesn't need this; adds unnecessary complexity |
| CSS custom properties | Tailwind CSS | Only if the site grows to need a design system with many one-off utility classes; not needed for a docs site with Starlight's variable system |
| npm | pnpm | If this site were part of a larger monorepo with shared packages; standalone kinder-site/ has no benefit from pnpm's disk deduplication |
| Self-hosted fonts | Google Fonts | Never for GDPR-sensitive audiences; self-hosting via Fontsource is equivalent DX with zero privacy cost |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| @astrojs/tailwind | Deprecated for Tailwind v4; replaced by @tailwindcss/vite | CSS custom properties (`--sl-*` variables) — sufficient for Starlight theming |
| @astrojs/starlight-tailwind | Tailwind v4 support unresolved as of March 2026; unstable integration | Starlight's native `customCss` |
| Gatsby | Framework sunset trajectory; build times slow; overkill for static docs | Astro |
| Jekyll | No JSX/MDX support; GitHub Pages default but 2010s-era DX | Astro |
| Netlify | Project already uses GitHub Pages per requirements; dual hosting adds complexity | GitHub Pages |
| React (standalone) | Not needed — Astro's default output is zero-JS; adding React increases bundle size with no benefit for a static doc site | Astro's `.astro` components |
| `withastro/action@v3` or lower | Older versions; v5.2.0 is current (February 2026) | withastro/action@v5 |

---

## Stack Patterns by Variant

**For the landing page (index page with splash layout):**
- Use Starlight's `template: splash` frontmatter option on `src/content/docs/index.mdx`
- Override the `Hero` component via `components.Hero` in `astro.config.mjs` to add custom sections (feature grid, install command, addon showcase)
- The splash template removes the sidebar; full-width layout suitable for marketing content

**For documentation pages:**
- Standard Starlight pages in `src/content/docs/` with Markdown or MDX
- Use MDX when a page needs interactive components (e.g., a tabbed addon config example)
- Use plain Markdown for everything else — simpler, faster to write

**For code snippets (terminal commands):**
- Expressive Code is bundled and active by default
- Use `bash` language tag for shell commands; Expressive Code renders them with dark theme automatically
- For the `kinder create cluster` hero command on the landing page, use a styled code block component — not a screenshot

---

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| @astrojs/starlight@0.37.x | astro@5.x | Starlight 0.37.x requires Astro 5; the create-astro template installs compatible versions |
| withastro/action@v5 | Node.js 22 | Default; also accepts 20.3.0 minimum |
| Astro 5.17 | Node.js 20.3.0+ or 22.0.0+ | Node 18 support dropped in Astro 5.8 (May 2025) |
| tailwindcss@4.x | @tailwindcss/vite (NOT @astrojs/tailwind) | @astrojs/tailwind is deprecated for Tailwind v4; NOT RECOMMENDED for this project |

---

## Directory Structure

```
kinder/                          # repo root (Go project)
├── kinder-site/                 # Astro project root
│   ├── astro.config.mjs
│   ├── package.json
│   ├── package-lock.json        # required — withastro/action detects npm from this
│   ├── tsconfig.json            # extends astro/tsconfigs/strict
│   ├── public/
│   │   └── CNAME                # kinder.patrykgolabek.dev
│   └── src/
│       ├── assets/              # logo, images
│       ├── styles/
│       │   └── custom.css       # --sl-* CSS variable overrides for dark theme
│       └── content/
│           └── docs/            # all documentation (file-based routing)
│               ├── index.mdx    # landing page (template: splash)
│               ├── getting-started/
│               ├── configuration/
│               └── addons/
└── .github/
    └── workflows/
        └── deploy-site.yml      # GitHub Actions deployment
```

---

## Sources

- Astro 5.17 release: https://astro.build/ (verified March 2026)
- @astrojs/starlight@0.37.6: https://github.com/withastro/starlight/releases (latest as of March 2026)
- Starlight getting started: https://starlight.astro.build/getting-started/
- Starlight CSS and theming: https://starlight.astro.build/guides/css-and-tailwind/
- Starlight configuration reference: https://starlight.astro.build/reference/configuration/
- Astro GitHub Pages deploy guide: https://docs.astro.build/en/guides/deploy/github/
- withastro/action@v5.2.0: https://github.com/withastro/action (released February 11, 2026)
- Tailwind v4 + Astro setup: https://tailwindcss.com/docs/installation/framework-guides/astro
- starlight-tailwind Tailwind v4 issue (OPEN): https://github.com/withastro/starlight/issues/2862
- Astro Node.js requirements (5.8 dropped Node 18): https://alternativeto.net/news/2025/5/astro-5-8-raises-node-js-requirements-as-support-for-node-js-v18-ends/
- GitHub Pages custom domain docs: https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/managing-a-custom-domain-for-your-github-pages-site

---
*Stack research for: kinder website (v1.1 milestone)*
*Researched: 2026-03-01*
