# Phase 12: Landing Page - Research

**Researched:** 2026-03-01
**Domain:** Starlight splash template, MDX component composition, custom Astro components with scoped CSS
**Confidence:** HIGH

## Summary

Phase 12 builds the kinder landing page by replacing the placeholder `src/content/docs/index.mdx` with a real splash page. Starlight's `template: splash` frontmatter renders a full-width page without sidebars. The hero section is entirely frontmatter-driven (tagline, install command display, action buttons). Feature cards use Starlight's built-in `<Card>` and `<CardGrid>` components imported in the MDX body.

The before/after comparison section (LAND-03) cannot be built with raw MDX alone — MDX files cannot contain `<style>` tags. The solution is a custom Astro component (`src/components/Comparison.astro`) imported into the MDX file. That component carries its own scoped CSS and renders the two-column before/after layout. The `not-content` class must be applied to its root element to suppress Starlight's markdown content styles.

The copy-to-clipboard button on code blocks is **built into Expressive Code by default** (`showCopyToClipboardButton: true`). However, the hero section's install command is in frontmatter, not a code block — it is displayed as button text, not a fenced code block. LAND-01 requires a working copy button for the install command. This is the only non-trivial interaction on the page. The install command copy button must be implemented as a small custom Astro component (a styled `<code>` element with a copy `<button>` powered by a few lines of inline JavaScript).

**Primary recommendation:** index.mdx with `template: splash`, hero via frontmatter, feature grid via `<CardGrid>` in MDX body, before/after via a custom `<Comparison>` Astro component, and a custom `<InstallCommand>` Astro component for the copy button.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| LAND-01 | Hero section with kinder tagline, one-line description, and install command with copy button | Splash template hero frontmatter handles tagline and description; install command copy button requires custom `<InstallCommand>` Astro component with JS clipboard API — no library needed |
| LAND-02 | Addon feature grid with cards for each of the 5 addons (icon, name, one-line description) | Starlight `<Card>` and `<CardGrid>` built-in components; icon names from Starlight's built-in icon set (no Kubernetes-specific icons — use generic alternatives); each Card links to existing addon doc pages |
| LAND-03 | Before/after comparison section (kind vs kinder — what you get out of the box) | Cannot use raw MDX style tags; requires custom `Comparison.astro` component with scoped CSS two-column grid; use `not-content` class to escape Starlight's markdown styles |
| LAND-04 | GitHub link and call-to-action to documentation from landing page without scrolling | Hero `actions` frontmatter array — "Get Started" links to `/installation/` and "GitHub" links externally; both render above the fold in hero section |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| @astrojs/starlight | 0.37.6 (installed) | Splash template, CardGrid, Card, built-in icons, Expressive Code copy button | Already installed; splash template is the official landing page pattern |
| astro | 5.6.1 (installed) | Astro component composition in MDX, scoped CSS in .astro files | Already installed; .astro files support `<style>` tags; MDX files do not |

### Supporting
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| Clipboard API (browser built-in) | N/A | navigator.clipboard.writeText() for copy button | The only interaction needed — no npm package required |
| Starlight Icon set | built-in | Card icons for addon feature grid | Use for LAND-02 cards; no Kubernetes-specific icons exist; use generic alternatives |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom `<InstallCommand>` Astro component | Third-party clipboard library | No library needed; `navigator.clipboard.writeText()` is baseline supported in all modern browsers; adding a package is unnecessary complexity |
| Custom `<Comparison>` Astro component | Markdown table | A Markdown table cannot achieve two-column visual layout with colored headers and icons without CSS; the Astro component approach is the correct Starlight-idiomatic solution |
| Custom `<Comparison>` Astro component | Raw HTML in MDX | Raw HTML works in MDX but cannot carry scoped `<style>` tags; all CSS would have to go in `theme.css` globally — worse for maintainability |
| Starlight built-in `<Card>` for addon grid | Custom card components | Built-in `<Card>` handles responsive grid, icon, title, body text; no custom component needed for LAND-02 |

**No new npm packages required.** Everything is achievable with installed Astro + Starlight plus two small custom `.astro` components.

## Architecture Patterns

### Recommended Project Structure
```
kinder-site/src/
├── components/
│   ├── ThemeSelect.astro        # Existing: dark-only override
│   ├── InstallCommand.astro     # NEW: install command + copy button
│   └── Comparison.astro         # NEW: before/after kind vs kinder
├── content/docs/
│   └── index.mdx                # REPLACE: placeholder -> real landing page
└── styles/
    └── theme.css                # Existing: dark-only cyan theme (no changes)
```

### Pattern 1: Splash Page with Hero via Frontmatter

**What:** The `template: splash` + `hero:` frontmatter renders Starlight's full-width landing layout. No sidebar, no TOC, no pagination.

**When to use:** The site root page (`index.mdx`).

**Example:**
```mdx
---
# Source: https://starlight.astro.build/reference/frontmatter/#hero
title: kinder — kind with batteries
description: A kind fork that ships MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, and Headlamp pre-installed.
template: splash
hero:
  tagline: kind, but with everything you actually need.
  actions:
    - text: Get Started
      link: /installation/
      icon: right-arrow
      variant: primary
    - text: GitHub
      link: https://github.com/patrykattc/kinder
      icon: external
      variant: minimal
---

import { Card, CardGrid } from '@astrojs/starlight/components';
import InstallCommand from '../../components/InstallCommand.astro';
import Comparison from '../../components/Comparison.astro';

<InstallCommand />

## What you get out of the box

<Comparison />

## Addons

<CardGrid>
  <Card title="MetalLB" icon="seti:db">
    LoadBalancer services that actually work. Your Services get a real external IP automatically.
    [Learn more](/addons/metallb/)
  </Card>
  ...
</CardGrid>
```

**Note:** The hero `actions` array already places GitHub and docs links above the fold (LAND-04 satisfied by frontmatter alone).

### Pattern 2: Custom Astro Component in MDX (with Scoped CSS)

**What:** Create a `.astro` component in `src/components/`, import it into `index.mdx`, and use it as a JSX element. The `.astro` component may have a `<style>` tag — styles are automatically scoped by Astro.

**When to use:** Any section of the landing page that needs CSS beyond what Markdown + Starlight components provide (before/after comparison, install command box).

**Example:**
```astro
---
// Source: https://starlight.astro.build/components/using-components/
// src/components/Comparison.astro
---

<div class="comparison not-content">
  <div class="column">
    <h3>kind</h3>
    <ul>
      <li>Kubernetes cluster</li>
      <li>Manual LoadBalancer setup</li>
      <li>No metrics by default</li>
    </ul>
  </div>
  <div class="column kinder">
    <h3>kinder</h3>
    <ul>
      <li>Kubernetes cluster</li>
      <li>MetalLB pre-installed</li>
      <li>Metrics Server pre-installed</li>
      <li>Envoy Gateway pre-installed</li>
      <li>CoreDNS tuned</li>
      <li>Headlamp dashboard</li>
    </ul>
  </div>
</div>

<style>
  .comparison {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.5rem;
    margin-block: 2rem;
  }
  .column {
    border: 1px solid var(--sl-color-gray-5);
    border-radius: 0.5rem;
    padding: 1.25rem;
  }
  .column.kinder {
    border-color: var(--sl-color-accent);
  }
  h3 {
    margin-top: 0;
    color: var(--sl-color-white);
  }
  .kinder h3 {
    color: var(--sl-color-accent);
  }
  ul {
    margin: 0;
    padding-inline-start: 1.25rem;
    color: var(--sl-color-gray-2);
  }
  @media (max-width: 50rem) {
    .comparison {
      grid-template-columns: 1fr;
    }
  }
</style>
```

**Critical:** Apply `not-content` class to the root element to prevent Starlight's `.sl-markdown-content` styles from adding unwanted margins and font sizing to list items.

### Pattern 3: Install Command with Copy Button

**What:** A styled code display with a copy-to-clipboard button using the browser's native `navigator.clipboard` API.

**When to use:** LAND-01 — the hero section install command needs a working copy button. The hero `actions` buttons do not display code; the command must be in a separate component below the hero or inline.

**Example:**
```astro
---
// src/components/InstallCommand.astro
const command = 'git clone https://github.com/patrykattc/kinder.git && cd kinder && make install';
---

<div class="install-command not-content">
  <code class="command-text">{command}</code>
  <button class="copy-btn" data-command={command} aria-label="Copy install command">
    Copy
  </button>
</div>

<script>
  document.querySelectorAll('.copy-btn').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const command = (btn as HTMLElement).dataset.command ?? '';
      await navigator.clipboard.writeText(command);
      btn.textContent = 'Copied!';
      setTimeout(() => { btn.textContent = 'Copy'; }, 2000);
    });
  });
</script>

<style>
  .install-command {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    background: var(--sl-color-black);
    border: 1px solid var(--sl-color-gray-5);
    border-radius: 0.375rem;
    padding: 0.75rem 1rem;
    margin-block: 1.5rem;
    max-width: 42rem;
  }
  .command-text {
    flex: 1;
    font-family: var(--sl-font-mono);
    font-size: var(--sl-text-sm);
    color: var(--sl-color-accent-high);
    overflow-x: auto;
    white-space: nowrap;
  }
  .copy-btn {
    flex-shrink: 0;
    background: var(--sl-color-accent);
    color: var(--sl-color-black);
    border: none;
    border-radius: 0.25rem;
    padding: 0.375rem 0.75rem;
    font-size: var(--sl-text-sm);
    cursor: pointer;
    font-weight: 600;
  }
  .copy-btn:hover {
    background: var(--sl-color-accent-high);
  }
</style>
```

**Note:** Astro client-side `<script>` tags are bundled and executed in the browser. This is the correct pattern for interactivity in Astro (no framework needed).

### Pattern 4: Addon Cards with Links

**What:** Starlight's `<Card>` does not support an `href` prop. To make addon cards link to doc pages, include a Markdown link inside the card body.

**When to use:** LAND-02 — each addon card must link to its doc page.

**Example:**
```mdx
import { Card, CardGrid } from '@astrojs/starlight/components';

<CardGrid>
  <Card title="MetalLB" icon="seti:db">
    Real LoadBalancer IPs from your local Docker/Podman subnet.
    [View docs](/addons/metallb/)
  </Card>
  <Card title="Envoy Gateway" icon="random">
    Gateway API routing with the `eg` GatewayClass pre-installed.
    [View docs](/addons/envoy-gateway/)
  </Card>
  <Card title="Metrics Server" icon="bars">
    `kubectl top` and HPA support out of the box.
    [View docs](/addons/metrics-server/)
  </Card>
  <Card title="CoreDNS Tuning" icon="setting">
    Autopath, pods verified, and doubled cache TTL.
    [View docs](/addons/coredns/)
  </Card>
  <Card title="Headlamp Dashboard" icon="laptop">
    Web-based cluster UI accessible via port-forward.
    [View docs](/addons/headlamp/)
  </Card>
</CardGrid>
```

**Note on icon names:** Starlight's built-in icon set does not include Kubernetes, networking, or gateway-specific icons. Use generic icons from the available set. Verified available names include: `setting`, `open-book`, `external`, `right-arrow`, `laptop`, `bars`, `github`, `star`, `pencil`, `information`, `document`, `add-document`, `puzzle`. The `seti:` prefix accesses the Seti icon set (e.g., `seti:db`, `seti:json`).

### Anti-Patterns to Avoid

- **Using `<style>` directly in index.mdx:** MDX files do not support `<style>` tags. Any CSS for custom sections must live in a `.astro` component or in `theme.css`.
- **Forgetting `not-content` on custom components:** Without this class, Starlight's `.sl-markdown-content` styles apply to the component's interior, adding unwanted margins, list styling, and font sizes.
- **Putting the install command inside the hero `actions`:** Hero actions render as styled buttons, not code. The install command needs its own component below the hero or within the hero HTML. Hero `actions` are for navigation, not code display.
- **Using `<LinkCard>` for the addon grid:** `<LinkCard>` is for navigation-only cards with no body content. Use `<Card>` with a Markdown link in the body to show the addon description AND link to docs.
- **Modifying theme.css for landing-page-only styles:** `theme.css` applies globally. Page-specific CSS belongs in component `<style>` blocks.
- **Hero image with light/dark variants:** The project is dark-only. Do not use `hero.image.dark`/`hero.image.light` — use `hero.image.file` or omit the image entirely. The existing `src/assets/houston.webp` is a placeholder and should be replaced or omitted.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Copy-to-clipboard button | Custom clipboard library (clipboard.js, etc.) | `navigator.clipboard.writeText()` natively | Single method, no dependency, fully supported in all modern browsers |
| Feature card grid | Custom CSS grid component | `<CardGrid>` + `<Card>` from `@astrojs/starlight/components` | Responsive, themed, uses Starlight accent colors, accessible |
| Code block on hero with auto-copy | Custom syntax highlighter | Simple styled `<code>` element in `InstallCommand.astro` | The install command doesn't need syntax highlighting — a styled `<code>` with monospace font is sufficient |
| Full-page layout | Custom page layout | `template: splash` frontmatter | Starlight handles the full-width, no-sidebar layout; no custom layout needed |

**Key insight:** All structural work is solved by Starlight's splash template + built-in components. The only custom work is two small `.astro` components (InstallCommand + Comparison) totalling ~60-80 lines of HTML + CSS each.

## Common Pitfalls

### Pitfall 1: Style Tags in MDX Files
**What goes wrong:** Developer adds `<style>` tag directly to `index.mdx` for the comparison section; styles are silently ignored or cause build errors.
**Why it happens:** MDX supports JSX-like syntax but does NOT support `<style>` or `<script>` tags directly.
**How to avoid:** Any section requiring custom CSS must be extracted into a `.astro` component. The component is then imported and used as a JSX element in the MDX file.
**Warning signs:** CSS written in MDX has no effect; comparison columns render unstyled side-by-side in a single-column layout.

### Pitfall 2: Forgetting `not-content` Class
**What goes wrong:** Custom component's list items, headings, and paragraphs inherit Starlight's markdown content styles — unexpected margins, font sizes, and colors from `.sl-markdown-content`.
**Why it happens:** Starlight applies `.sl-markdown-content` styles to all content inside the main content area, including imported components.
**How to avoid:** Add `class="not-content"` (or `:class="not-content"` in Astro syntax) to the root element of any custom component that manages its own layout.
**Warning signs:** The comparison table looks fine in isolation but has extra top margins, different bullet styles, or wrong font size when rendered in the landing page.

### Pitfall 3: Hero Install Command Display
**What goes wrong:** Developer puts the install command in hero `actions` — it appears as a button, not a copyable code display.
**Why it happens:** The hero `actions` array renders `<LinkButton>` elements. These are navigation buttons, not code displays.
**How to avoid:** Display the install command in `<InstallCommand>` component placed in the MDX body immediately below the hero. The hero actions should contain only navigation (Get Started, GitHub). The success criterion for LAND-01 says "without requiring any documentation page to be open" — the install command component in the page body below the hero satisfies this.
**Warning signs:** The install command text is styled like a button and has no monospace font; there is no distinct code appearance.

### Pitfall 4: Card Links Without Slugs
**What goes wrong:** Card body Markdown links use incorrect paths; clicking "View docs" results in 404.
**Why it happens:** Starlight doc page URLs are determined by file path under `src/content/docs/`. The addon pages are at `addons/metallb.md` etc., so their URLs are `/addons/metallb/` (with trailing slash by Astro default).
**How to avoid:** Use root-relative links: `/addons/metallb/`, `/addons/envoy-gateway/`, `/addons/metrics-server/`, `/addons/coredns/`, `/addons/headlamp/`. Verify each link during build with `npm run build` (broken links may warn or error depending on Astro config).
**Warning signs:** Card links produce 404 errors.

### Pitfall 5: index.mdx File Extension (Already MDX)
**What goes wrong:** Developer assumes `index.md` must be renamed to `index.mdx` to use components.
**Why it happens:** The placeholder file is already `index.mdx`. No rename needed.
**How to avoid:** Confirm the file is `src/content/docs/index.mdx` before starting work. (It is — confirmed from project exploration.)
**Warning signs:** This is a non-issue for this project; flagged only because it is a common mistake for new Starlight projects.

### Pitfall 6: Clipboard API Requires HTTPS or localhost
**What goes wrong:** The copy button silently fails or throws a DOMException in production on HTTP.
**Why it happens:** `navigator.clipboard` is only available in secure contexts (HTTPS or localhost). The site uses `kinder.patrykgolabek.dev` with a custom domain — it must be HTTPS.
**How to avoid:** The custom domain with Netlify/GitHub Pages is HTTPS by default. No action needed. During local development, `npm run dev` serves on localhost (secure context). The site does NOT run on plain HTTP in production.
**Warning signs:** Copy button works locally but fails after deploy — usually means HTTP deploy, not HTTPS.

## Code Examples

Verified patterns from official sources:

### Complete index.mdx Landing Page Structure
```mdx
---
# Source: https://starlight.astro.build/reference/frontmatter/
title: kinder
description: kind with MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, and Headlamp pre-installed.
template: splash
hero:
  tagline: kind, but with everything you actually need.
  actions:
    - text: Get Started
      link: /installation/
      icon: right-arrow
      variant: primary
    - text: GitHub
      link: https://github.com/patrykattc/kinder
      icon: external
      variant: minimal
---

import { Card, CardGrid } from '@astrojs/starlight/components';
import InstallCommand from '../../components/InstallCommand.astro';
import Comparison from '../../components/Comparison.astro';

## Install

<InstallCommand />

## kind vs kinder

<Comparison />

## Addons

<CardGrid>
  <Card title="MetalLB" icon="seti:db">
    LoadBalancer services with real external IPs from your Docker or Podman subnet.
    [View docs](/addons/metallb/)
  </Card>
  <Card title="Envoy Gateway" icon="random">
    Gateway API routing with the `eg` GatewayClass pre-configured.
    [View docs](/addons/envoy-gateway/)
  </Card>
  <Card title="Metrics Server" icon="bars">
    Enables `kubectl top` and Horizontal Pod Autoscaler support.
    [View docs](/addons/metrics-server/)
  </Card>
  <Card title="CoreDNS Tuning" icon="setting">
    Autopath short names, verified pod records, and doubled cache TTL.
    [View docs](/addons/coredns/)
  </Card>
  <Card title="Headlamp Dashboard" icon="laptop">
    Web-based cluster UI. Access via port-forward, token printed at cluster creation.
    [View docs](/addons/headlamp/)
  </Card>
</CardGrid>
```

### Hero Frontmatter (verified structure from official docs)
```yaml
# Source: https://starlight.astro.build/reference/frontmatter/#hero
hero:
  title: Optional — overrides page title in the hero
  tagline: One-line description shown below the title
  image:
    file: ~/assets/logo.png   # optional; relative to src/assets/
    alt: Logo alt text
  actions:
    - text: Button label
      link: /path/
      icon: right-arrow        # any Starlight icon name
      variant: primary         # primary | secondary | minimal
    - text: External button
      link: https://github.com/...
      icon: external
      variant: minimal
```

### CSS Variables Available for Custom Components
```css
/* Source: https://github.com/withastro/starlight/blob/main/packages/starlight/style/props.css */
/* Dark-mode accent (cyan) from this project's theme.css */
--sl-color-accent-low: hsl(185, 54%, 15%);
--sl-color-accent: hsl(185, 100%, 45%);
--sl-color-accent-high: hsl(185, 100%, 75%);
--sl-color-black: hsl(220, 15%, 7%);
/* Standard Starlight text/gray variables */
--sl-color-white
--sl-color-gray-1 through --sl-color-gray-6
--sl-font-mono
--sl-text-sm
--sl-text-base
--sl-text-xl
```

### Astro Client Script Pattern
```astro
<!-- Source: https://docs.astro.build/en/guides/client-side-scripts/ -->
<script>
  // Scripts in .astro components are bundled by Astro and run in the browser.
  // No type="module" needed — Astro handles this automatically.
  document.querySelectorAll('.copy-btn').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const command = (btn as HTMLElement).dataset.command ?? '';
      await navigator.clipboard.writeText(command);
      btn.textContent = 'Copied!';
      setTimeout(() => { btn.textContent = 'Copy'; }, 2000);
    });
  });
</script>
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate Docusaurus/VitePress landing pages | Starlight `template: splash` with MDX | Starlight launch 2023 | No separate framework needed; landing page is a content file |
| clipboard.js for copy buttons | `navigator.clipboard.writeText()` | Web standard; broadly supported by 2022 | Zero dependency; simpler code |
| Custom React components for cards | Starlight built-in `<Card>/<CardGrid>` | Starlight v0.2+ | No framework overhead; works in plain MDX |
| MDX inline `<style>` tags | Astro component `<style>` tags | Astro launch | MDX never supported style tags; the pattern is always extract to .astro |

**Deprecated/outdated:**
- `~/assets/houston.webp`: The placeholder image from Starlight's default scaffold. Must be replaced with a kinder logo/image or the hero image omitted. The project has `logo/logo.svg` and `logo/logo.png` which could be copied to `src/assets/` for use.

## Open Questions

1. **Whether to include a hero image**
   - What we know: The project has `logo/logo.svg` and `logo/logo.png`. The current `src/assets/houston.webp` is a Starlight placeholder.
   - What's unclear: Whether the kinder logo looks good in the Starlight hero layout (which places the image at 400x400 on the right side at desktop widths).
   - Recommendation: Planner should include a task to copy `logo/logo.svg` to `src/assets/kinder-logo.svg` and reference it in the hero frontmatter as `image.file`. If the logo does not render well at square dimensions, omit the image and use a text-only hero. The dark-only theme means only `image.file` is needed (not `image.dark`/`image.light`).

2. **Exact install command to display in InstallCommand component**
   - What we know: Phase 11 established that `make install` (build from source) is the only confirmed install method. The quick-start page shows `kinder create cluster`. The installation page shows `git clone ... && cd kinder && make install`.
   - What's unclear: Whether to show the full multi-command sequence or just `make install` or whether to show a one-liner.
   - Recommendation: Use the full one-liner: `git clone https://github.com/patrykattc/kinder.git && cd kinder && make install`. This is the minimal complete install sequence. Alternatively, use just `make install` with a note "after cloning". The planner should match whatever the installation.md page shows to maintain consistency.

3. **CardGrid stagger vs non-stagger for addon grid**
   - What we know: `<CardGrid stagger>` shifts the second column down for visual interest; works best with an even number of cards. There are 5 addon cards (odd number) — the stagger effect may look off with an orphaned last card.
   - What's unclear: Whether stagger looks good with 5 cards.
   - Recommendation: Use `<CardGrid>` without `stagger` for 5 cards. The stagger is primarily a design flourish for even-count grids.

## Sources

### Primary (HIGH confidence)
- https://starlight.astro.build/reference/frontmatter/ — hero frontmatter structure, template: splash, all hero fields (fetched 2026-03-01)
- https://starlight.astro.build/components/cards/ — Card, CardGrid, LinkCard components, props, import syntax (fetched 2026-03-01)
- https://starlight.astro.build/components/card-grids/ — CardGrid stagger prop, responsive behavior (fetched 2026-03-01)
- https://starlight.astro.build/components/using-components/ — importing custom Astro components in MDX, `not-content` class (fetched 2026-03-01)
- https://starlight.astro.build/guides/css-and-tailwind/ — customCss configuration, CSS custom properties pattern (fetched 2026-03-01)
- https://github.com/withastro/starlight/blob/main/packages/starlight/components/Hero.astro — actual Hero component source, CSS structure (fetched 2026-03-01)
- https://raw.githubusercontent.com/withastro/starlight/main/docs/src/content/docs/index.mdx — Starlight's own homepage MDX structure as reference (fetched 2026-03-01)
- https://expressive-code.com/key-features/frames/ — `showCopyToClipboardButton: true` default, copy button behavior (fetched 2026-03-01)
- `/Users/patrykattc/work/git/kinder/kinder-site/astro.config.mjs` — confirmed Astro 5.6.1, Starlight 0.37.6, dark-only ThemeSelect override, customCss path
- `/Users/patrykattc/work/git/kinder/kinder-site/src/content/docs/index.mdx` — confirmed file is already .mdx (not .md), currently placeholder
- `/Users/patrykattc/work/git/kinder/kinder-site/src/styles/theme.css` — confirmed CSS variables for accent cyan theme
- `/Users/patrykattc/work/git/kinder/kinder-site/src/content/docs/installation.md` — confirmed install command is `make install` after `git clone`

### Secondary (MEDIUM confidence)
- https://docs.astro.build/en/guides/styling/ — MDX cannot contain `<style>` tags; `.astro` components support scoped `<style>` (fetched 2026-03-01; confirmed by multiple sources)
- https://starlight.astro.build/reference/icons/ — built-in icon set; no Kubernetes-specific icons confirmed (fetched 2026-03-01)

### Tertiary (LOW confidence)
- Icon name availability for specific names (`seti:db`, `random`, `bars`, `laptop`): listed as plausible from the Starlight icon reference page, but exact names were not enumerated exhaustively. The planner should verify icon names against `https://starlight.astro.build/reference/icons/` or by testing in `npm run dev`.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Astro 5.6.1 + Starlight 0.37.6 installed and confirmed; all APIs verified against official docs fetched 2026-03-01
- Architecture (splash template, MDX component imports, custom .astro components): HIGH — verified against official Starlight docs and Hero.astro source code
- Copy-to-clipboard approach: HIGH — `navigator.clipboard.writeText()` is a web standard; MDX style tag limitation confirmed from official Astro docs
- Icon names for CardGrid cards: LOW — icon set page was checked but specific names for abstract concepts (metrics, gateway) are not from a verified exhaustive list; planner must verify at build time
- Comparison component CSS structure: MEDIUM — pattern follows documented Starlight CSS variables and `not-content` class; exact visual outcome requires dev server verification

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (Starlight pre-1.0; splash template and MDX patterns are stable but check changelog if Starlight is updated beyond 0.37.6)
