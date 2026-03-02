# Phase 13: Assets and Identity - Research

**Researched:** 2026-03-01
**Domain:** Astro Starlight — favicon, Open Graph meta, custom 404 page
**Confidence:** HIGH

## Summary

Phase 13 adds the three remaining visual-identity elements to the kinder Starlight site: a wired-up favicon, an Open Graph image for social sharing previews, and a custom 404 page. All three are straightforward Starlight configuration tasks that require no new dependencies — the current `@astrojs/starlight@^0.37.6` and `astro@^5.6.1` already support everything needed.

The favicon is already partially solved: `public/favicon.svg` exists with the kinder snowflake SVG and the correct light/dark color switching. The gap is that the Starlight `favicon` config option is not set in `astro.config.mjs`, so Starlight continues to serve its default Houston icon from Starlight's own assets. Wiring the config closes the gap without touching the SVG file.

For OG image, the simplest approach for a single-brand site is a static PNG in `public/` combined with global `head` configuration in `astro.config.mjs`. This avoids build-time image generation complexity (Satori/sharp pipelines) while satisfying the requirement. `sharp@^0.34.2` is already a dependency if a programmatic approach is preferred later, but it is not required for a static image. The 404 page uses `disable404Route: true` plus a `src/pages/404.astro` file that wraps content in the `<StarlightPage>` component so it inherits the site theme.

**Primary recommendation:** Wire favicon via Starlight config, place a static `og.png` in `public/` and add global head meta tags, and create `src/pages/404.astro` using `<StarlightPage>` with `disable404Route: true`.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PLSH-01 | Favicon and site logo (SVG) | Starlight `favicon` config option + existing `public/favicon.svg`; optional `logo` config for header logo |
| PLSH-02 | Open Graph image for social media sharing | Static PNG in `public/`, global `head` config with `og:image` and `twitter:image` meta tags |
| PLSH-03 | Custom 404 page | `disable404Route: true` + `src/pages/404.astro` using `<StarlightPage>` component |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| @astrojs/starlight | ^0.37.6 (already installed) | Favicon config, head meta, disable404Route | All three requirements are native Starlight features |
| astro | ^5.6.1 (already installed) | File-based routing for 404.astro | Standard Astro page routing |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| sharp | ^0.34.2 (already installed) | PNG image conversion | Only needed if OG image is generated programmatically; not required for static PNG approach |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Static `og.png` in `public/` | `astro-og-canvas` + route middleware | Dynamic generation creates a per-page OG image but adds complexity and a `canvaskit-wasm` dependency; overkill for a brand-level single image |
| `<StarlightPage>` for 404 | Raw `404.astro` with custom HTML | Raw approach loses Starlight theme styling; `<StarlightPage>` is the correct integration point |
| SVG favicon | ICO favicon | SVG is smaller, scales perfectly, and Starlight supports it natively; ICO is legacy |

**Installation:** No new packages required. All functionality is in already-installed dependencies.

## Architecture Patterns

### Recommended Project Structure

```
kinder-site/
├── public/
│   ├── favicon.svg          # already exists — kinder snowflake SVG
│   └── og.png               # NEW — 1200x630 OG image
├── src/
│   ├── assets/
│   │   └── logo.svg         # OPTIONAL — if header logo is wanted
│   └── pages/
│       └── 404.astro        # NEW — custom 404 page
└── astro.config.mjs         # MODIFIED — favicon, head, disable404Route
```

### Pattern 1: Favicon via Starlight Config

**What:** Tell Starlight which file in `public/` to use as favicon. The file `public/favicon.svg` already exists.
**When to use:** Any time the default Starlight Houston favicon is being served instead of the project's own.

```javascript
// Source: https://starlight.astro.build/reference/configuration/
// astro.config.mjs
starlight({
  favicon: '/favicon.svg',   // points to public/favicon.svg
})
```

**Optional — header logo:** If a logo image should also appear in the nav bar beside the site title:

```javascript
// Source: https://starlight.astro.build/guides/customization/
starlight({
  logo: {
    src: './src/assets/logo.svg',   // file in src/assets/, not public/
    // replacesTitle: true,         // hides "kinder" text when logo contains it
  },
})
```

### Pattern 2: Static OG Image via Global Head Config

**What:** Place a single PNG in `public/` and inject `og:image` + `twitter:image` meta tags for every page via Starlight's `head` config option.
**When to use:** Single-brand sites where all pages share the same social preview image.

```javascript
// Source: https://hideoo.dev/notes/starlight-custom-html-head
//         https://starlight.astro.build/reference/configuration/
starlight({
  head: [
    {
      tag: 'meta',
      attrs: {
        property: 'og:image',
        content: 'https://kinder.patrykgolabek.dev/og.png',
      },
    },
    {
      tag: 'meta',
      attrs: {
        name: 'twitter:image',
        content: 'https://kinder.patrykgolabek.dev/og.png',
      },
    },
    {
      tag: 'meta',
      attrs: {
        property: 'og:title',
        content: 'kinder',
      },
    },
    {
      tag: 'meta',
      attrs: {
        property: 'og:description',
        content: 'kind, but with everything you actually need.',
      },
    },
    {
      tag: 'meta',
      attrs: {
        name: 'twitter:card',
        content: 'summary_large_image',
      },
    },
  ],
})
```

The `site` property in `astro.config.mjs` is already set to `'https://kinder.patrykgolabek.dev'`, so the absolute URL is known.

**OG image design guidance:**
- Size: 1200x630px (Facebook/Twitter standard)
- Content: kinder wordmark or snowflake logo, tagline "kind, but with everything you actually need.", dark background matching the site's `hsl(220, 15%, 7%)` theme
- Format: PNG (JPEG also acceptable; PNG preferred for text sharpness)
- Tool: Any image editor; Figma/Inkscape export; or generate with sharp since it is already installed

### Pattern 3: Custom 404 Page with StarlightPage

**What:** Disable Starlight's default 404 route, then provide `src/pages/404.astro` using `<StarlightPage>` to keep site-wide styling.
**When to use:** Whenever a branded 404 experience is required.

```javascript
// Source: https://starlight.astro.build/reference/configuration/
// astro.config.mjs
starlight({
  disable404Route: true,
})
```

```astro
---
// Source: https://starlight.astro.build/guides/pages/
// src/pages/404.astro
import StarlightPage from '@astrojs/starlight/components/StarlightPage.astro';
---

<StarlightPage frontmatter={{ title: 'Page Not Found', template: 'splash' }}>
  <div style="text-align: center; padding: 4rem 0;">
    <p>This page doesn't exist.</p>
    <a href="/" style="margin-top: 1rem; display: inline-block;">
      Back to Home
    </a>
  </div>
</StarlightPage>
```

GitHub Pages serves `404.html` at the root for all unmatched routes. Astro's static build emits `dist/404.html` when `src/pages/404.astro` exists. No extra configuration is needed for GitHub Pages.

### Anti-Patterns to Avoid

- **Putting logo in `public/` instead of `src/assets/`:** Starlight's `logo.src` config must point to a file in `src/assets/` (processed by Astro's image pipeline). `favicon` points to `public/`.
- **Using a relative URL for `og:image`:** Social crawlers require an absolute URL. Always use the full `https://kinder.patrykgolabek.dev/og.png` form.
- **Creating `src/pages/404.astro` without `disable404Route: true`:** Starlight's injected route collides with the file, causing a build error (confirmed in withastro/starlight#1080).
- **Skipping `twitter:card: summary_large_image`:** Without this tag, Twitter/X shows a small thumbnail instead of the large card format.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Favicon wiring | Custom `<link rel="icon">` in a Head override component | Starlight `favicon` config option | Built-in — Starlight injects the correct `<link>` tags including type attribute |
| Global meta tags | Custom Head component override | Starlight `head` config array | Head override is "last resort" per official docs; config option achieves same result without losing Starlight's deduplication logic |
| 404 styling | Copy-paste Starlight CSS into a standalone 404 | `<StarlightPage>` component | StarlightPage is the official integration point; it inherits all theme variables, dark mode, and nav automatically |

**Key insight:** All three requirements have first-class Starlight config support. Custom code is not needed for any of them.

## Common Pitfalls

### Pitfall 1: Favicon Not Updating in Browser Cache

**What goes wrong:** Developer adds `favicon: '/favicon.svg'` but the browser still shows the old icon.
**Why it happens:** Browsers aggressively cache favicons, sometimes for hours.
**How to avoid:** Test in a private/incognito window, or force-refresh with `Ctrl+Shift+R`. Verify the `<link rel="icon">` is present in the built HTML.
**Warning signs:** The dev server shows the correct icon but production seems wrong.

### Pitfall 2: OG Image URL is Relative

**What goes wrong:** Social media platforms (Slack, Twitter, LinkedIn) show a broken image or no preview.
**Why it happens:** The `og:image` content is a path like `/og.png` instead of the full absolute URL.
**How to avoid:** Always use the full `https://kinder.patrykgolabek.dev/og.png`. Verify with the [OpenGraph.xyz debugger](https://www.opengraph.xyz) or [cards-dev.twitter.com/validator](https://cards-dev.twitter.com/validator).
**Warning signs:** The image loads fine when visited directly but does not appear in social previews.

### Pitfall 3: 404 Route Collision

**What goes wrong:** Build fails with "This route collides with: src/pages/404.astro".
**Why it happens:** Starlight injects its own 404 route, which conflicts with a user-provided `src/pages/404.astro`.
**How to avoid:** Set `disable404Route: true` in `astro.config.mjs` before creating `src/pages/404.astro`.
**Warning signs:** Build error mentioning route collision for the 404 path.

### Pitfall 4: OG Image Dimensions Wrong

**What goes wrong:** LinkedIn or Twitter crops the image oddly or uses a small thumbnail.
**Why it happens:** Image is not 1200x630px (2:1 or 1.91:1 ratio).
**How to avoid:** Export the OG image at exactly 1200x630px. Include `og:image:width` and `og:image:height` meta tags for platforms that read them.
**Warning signs:** Social previews look distorted or fall back to a generic card style.

## Code Examples

Verified patterns from official sources:

### Complete astro.config.mjs with All Three Features

```javascript
// Source: https://starlight.astro.build/reference/configuration/
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  integrations: [
    starlight({
      title: 'kinder',
      favicon: '/favicon.svg',          // PLSH-01
      disable404Route: true,            // PLSH-03
      head: [                           // PLSH-02
        {
          tag: 'meta',
          attrs: { property: 'og:image', content: 'https://kinder.patrykgolabek.dev/og.png' },
        },
        {
          tag: 'meta',
          attrs: { property: 'og:title', content: 'kinder' },
        },
        {
          tag: 'meta',
          attrs: { property: 'og:description', content: 'kind, but with everything you actually need.' },
        },
        {
          tag: 'meta',
          attrs: { name: 'twitter:card', content: 'summary_large_image' },
        },
        {
          tag: 'meta',
          attrs: { name: 'twitter:image', content: 'https://kinder.patrykgolabek.dev/og.png' },
        },
      ],
      sidebar: [
        { slug: 'installation' },
        { slug: 'quick-start' },
        { slug: 'configuration' },
        {
          label: 'Addons',
          items: [
            { slug: 'addons/metallb' },
            { slug: 'addons/envoy-gateway' },
            { slug: 'addons/metrics-server' },
            { slug: 'addons/coredns' },
            { slug: 'addons/headlamp' },
          ],
        },
      ],
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/patrykattc/kinder' },
      ],
      customCss: ['./src/styles/theme.css'],
      components: {
        ThemeSelect: './src/components/ThemeSelect.astro',
      },
    }),
  ],
});
```

### 404 Page Using StarlightPage

```astro
---
// Source: https://starlight.astro.build/guides/pages/
// src/pages/404.astro
import StarlightPage from '@astrojs/starlight/components/StarlightPage.astro';
---

<StarlightPage frontmatter={{ title: 'Page Not Found', template: 'splash' }}>
  <div style="text-align: center; padding: 4rem 1rem;">
    <p style="font-size: 1.25rem; margin-bottom: 1.5rem;">
      This page doesn't exist.
    </p>
    <a
      href="/"
      style="
        display: inline-block;
        padding: 0.75rem 1.5rem;
        background: var(--sl-color-accent);
        color: var(--sl-color-black);
        border-radius: 0.5rem;
        text-decoration: none;
        font-weight: 600;
      "
    >
      Back to Home
    </a>
  </div>
</StarlightPage>
```

### Verifying OG Tags in Built HTML

```bash
# Run from kinder-site/ after build
grep -E 'og:|twitter:' dist/index.html
```

Expected output should include `og:image`, `og:title`, `og:description`, `twitter:card`, `twitter:image`.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual `<link rel="icon">` in Head override | Starlight `favicon` config option | Starlight v0.x | No override component needed |
| Starlight had no 404 customization | `disable404Route` config option | Starlight v0.17.0 | Clean custom 404 without route collisions |
| Custom `<Head>` component for all meta tags | Starlight `head` config array | Early Starlight | Head array handles deduplication automatically |

**Deprecated/outdated:**
- Overriding the `Head` component just to add meta tags: Starlight docs explicitly call this "a last resort." The `head` config array achieves the same without overriding internal components.

## Open Questions

1. **Header logo (PLSH-01 scope)**
   - What we know: PLSH-01 says "Favicon and site logo (SVG)". The favicon is solved. "Site logo" could mean the header nav logo, or it could just refer to the favicon itself.
   - What's unclear: Does the planner intend a logo image in the Starlight nav header in addition to the favicon, or is the favicon the only deliverable?
   - Recommendation: The planner should interpret "site logo" as the favicon (it is the primary identity asset). The nav logo can be added later if requested. If both are intended, add `logo.src: './src/assets/logo.svg'` and copy `logo/logo.svg` to `src/assets/`.

2. **OG image design ownership**
   - What we know: The phase requires an OG image; the design is not specified.
   - What's unclear: Should the OG image be created as a task in this phase, or is a design provided externally?
   - Recommendation: Treat the OG image creation as a task within this phase. Use the existing kinder snowflake SVG from `public/favicon.svg` rendered on a dark background matching `hsl(220, 15%, 7%)` with site accent cyan `hsl(185, 100%, 45%)`. A simple Inkscape/Figma export or sharp-based script suffices.

## Sources

### Primary (HIGH confidence)

- https://starlight.astro.build/reference/configuration/ — `favicon`, `head`, `logo`, `disable404Route` config options with types and examples
- https://starlight.astro.build/guides/pages/ — `StarlightPage` component API and custom 404 pattern
- https://starlight.astro.build/guides/customization/ — logo configuration with light/dark variants and `replacesTitle`
- https://starlight.astro.build/reference/route-data/ — `head` array property on `starlightRoute`

### Secondary (MEDIUM confidence)

- https://hideoo.dev/notes/starlight-custom-html-head — og:image head config pattern, verified against official Starlight config reference
- https://hideoo.dev/notes/starlight-og-images/ — `astro-og-canvas` dynamic approach, documented for comparison; not recommended for this phase
- withastro/starlight#1080 GitHub issue — confirms route collision between injected 404 and `src/pages/404.astro` without `disable404Route: true`

### Tertiary (LOW confidence)

- https://starlight-changelog.netlify.app/ — version 0.17.0 as the release adding `disable404Route`; not independently verified against the actual CHANGELOG

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all features verified in current official docs; no new packages needed
- Architecture: HIGH — all three patterns verified against Starlight configuration reference and guides
- Pitfalls: HIGH — route collision and OG URL pitfalls confirmed via official docs and GitHub issues

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (Starlight releases frequently but config API is stable)
