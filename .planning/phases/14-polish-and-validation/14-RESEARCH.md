# Phase 14: Polish and Validation - Research

**Researched:** 2026-03-01
**Domain:** Mobile responsiveness validation and Lighthouse performance auditing for Astro + Starlight site
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PLSH-04 | Mobile-responsive validation — landing page and all docs pages fully readable and usable on 375px-wide mobile viewport with no horizontal scroll or overlapping elements | InstallCommand.astro has `white-space: nowrap` that causes horizontal overflow on narrow viewports; Comparison.astro already has correct `@media (max-width: 50rem)` responsive breakpoint |
| PLSH-05 | Lighthouse performance pass — 90+ on Performance, Accessibility, Best Practices, and SEO when run against live production URL | Site is missing `public/robots.txt`; Starlight auto-generates sitemap (already in dist); external links in HTML need `rel` verification; site otherwise meets Lighthouse fundamentals by Astro's static-first architecture |
</phase_requirements>

---

## Summary

Phase 14 validates that the kinder site is production-ready on two dimensions: mobile responsiveness at 375px and Lighthouse 90+ scores across all four categories. The site is built with Astro 5.6 + Starlight 0.37.6, which produces fully static HTML with zero client-side JavaScript by default — this gives the site a strong Performance baseline. Starlight also provides a `<meta name="viewport">` tag, canonical URLs, titles, and meta descriptions automatically, covering the majority of Lighthouse's SEO and Accessibility audit categories out of the box.

The primary mobile issue to address is `InstallCommand.astro`, which uses `white-space: nowrap` on a command that is approximately 70 characters long (`git clone https://github.com/patrykattc/kinder.git && cd kinder && make install`). The `.install-command` container has `overflow-x: auto` on the inner `<code>` element and `max-width: 42rem`, but the flex container itself has no `flex-wrap` or responsive override, meaning the Copy button and the command text will both be too wide at 375px. The `Comparison.astro` already has a correct `@media (max-width: 50rem)` responsive breakpoint that stacks the two columns to single-column — this part is fine.

The primary Lighthouse gap is a missing `public/robots.txt`. The live site returns HTTP 404 for `/robots.txt`. Lighthouse's SEO audit checks this file — a 404 response is flagged as "unable to download robots.txt" and can fail the robots.txt validity audit, which at equal weighting with the other ~8 SEO audits could drop the SEO score meaningfully. The sitemap is already handled — Starlight internally uses `@astrojs/sitemap` and generates `sitemap-index.xml` and `sitemap-0.xml` automatically when `site` is configured in `astro.config.mjs` (already done with `https://kinder.patrykgolabek.dev`).

**Primary recommendation:** Fix `InstallCommand.astro` for mobile, add `public/robots.txt`, then run Lighthouse against the live production URL to verify 90+ across all four categories.

---

## Standard Stack

### Core (already installed)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Astro | ^5.6.1 | Static site framework | Zero JS by default, Lighthouse-optimized |
| @astrojs/starlight | ^0.37.6 | Documentation theme | Built-in viewport meta, canonical, sitemap, a11y |
| sharp | ^0.34.2 | Image processing | Required for Astro image optimization |

### Tools (no new install required)
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| lighthouse (CLI) | latest | Audit live URLs from terminal | Run against `https://kinder.patrykgolabek.dev` |
| Chrome DevTools | browser | Visual mobile emulation at 375px | Manual inspection before fixing |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Lighthouse CLI | PageSpeed Insights (web UI) | PSI is easier but CLI is scriptable and more reproducible |
| Lighthouse CLI | WebPageTest | WebPageTest is more detailed but overkill for 90+ gate |
| Manual 375px check | Playwright/Cypress screenshot | Automation is heavier than needed; manual DevTools check is sufficient for this phase |

**No new npm packages are required for this phase.** Lighthouse is run as a one-off CLI audit, not integrated into the build.

**Optional Lighthouse install (for CLI audit):**
```bash
npm install -g lighthouse
```

---

## Architecture Patterns

### What Already Works (Starlight Handles Automatically)

Starlight 0.37.6 provides the following for free — do not re-implement or override:

- `<meta name="viewport" content="width=device-width, initial-scale=1"/>` — verified in built HTML
- `<meta name="description" .../>` — from page frontmatter `description` field
- `<title>` element — from frontmatter `title` + site title
- `<link rel="canonical" href="..."/>` — Starlight generates per-page canonical
- `<link rel="sitemap" href="/sitemap-index.xml"/>` — Starlight bundles `@astrojs/sitemap` internally
- `sitemap-index.xml` and `sitemap-0.xml` in dist — generated at build time
- Mobile hamburger navigation — Starlight switches sidebar to dropdown below `50rem`
- Skip-to-content link — Starlight renders `<a href="#_top">Skip to content</a>`
- `lang="en" dir="ltr"` on `<html>` — required for Lighthouse accessibility

### Mobile Responsive Fix Pattern

**What to fix:** `InstallCommand.astro` — the flex container causes horizontal overflow on narrow viewports.

**Current behavior at 375px:** The `.install-command` div is flex with `align-items: center` and `gap: 1rem`. The `.command-text` has `overflow-x: auto` and `white-space: nowrap`, but the outer container is constrained by `max-width: 42rem` which is `~672px` — wider than 375px. The container itself will try to be the full viewport width, but the button (`flex-shrink: 0`) plus the command text create a combined natural width that exceeds the viewport, causing the container to generate horizontal scroll on the page.

**Fix pattern — responsive wrapping:**
```css
/* In InstallCommand.astro <style> — add responsive behavior */
@media (max-width: 50rem) {
  .install-command {
    flex-direction: column;
    align-items: stretch;
  }

  .command-text {
    overflow-x: auto;
  }

  .copy-btn {
    width: 100%;
  }
}
```

**Alternative fix — keep horizontal but ensure container doesn't overflow page:**
```css
@media (max-width: 50rem) {
  .install-command {
    max-width: 100%;
    overflow-x: auto;
  }
}
```

The column-stack approach (first option) is more usable on mobile — the button fills the full width and the command scrolls independently within its container.

### Recommended Project Structure (No Changes)
```
kinder-site/
├── public/
│   ├── .nojekyll         (exists)
│   ├── CNAME             (exists)
│   ├── favicon.svg       (exists)
│   ├── og.png            (exists)
│   └── robots.txt        (MISSING — add this)
├── src/
│   ├── components/
│   │   ├── InstallCommand.astro   (fix mobile CSS)
│   │   ├── Comparison.astro       (responsive already correct)
│   │   └── ThemeSelect.astro      (override, no changes needed)
│   └── content/docs/...
```

### robots.txt Pattern

A minimal robots.txt for the kinder site:
```
User-agent: *
Allow: /

Sitemap: https://kinder.patrykgolabek.dev/sitemap-index.xml
```

This passes Lighthouse's robots.txt validity audit. The `Sitemap:` directive helps crawlers find the auto-generated sitemap.

### Lighthouse CLI Audit Pattern

Run against the live production URL (not localhost — Lighthouse scores differ significantly on production vs local):
```bash
# Install once if needed
npm install -g lighthouse

# Run full audit against live URL, output HTML report
lighthouse https://kinder.patrykgolabek.dev \
  --output html \
  --output-path lighthouse-report.html \
  --only-categories=performance,accessibility,best-practices,seo

# Open report
open lighthouse-report.html
```

For a quick JSON score check:
```bash
lighthouse https://kinder.patrykgolabek.dev \
  --output json \
  --output-path /tmp/lh.json \
  --only-categories=performance,accessibility,best-practices,seo \
  --quiet && \
  node -e "
    const r = require('/tmp/lh.json');
    const cats = r.categories;
    ['performance','accessibility','best-practices','seo'].forEach(k => {
      const score = Math.round(cats[k].score * 100);
      const pass = score >= 90 ? 'PASS' : 'FAIL';
      console.log(k + ': ' + score + ' [' + pass + ']');
    });
  "
```

### Chrome DevTools Mobile Emulation Pattern

Manual validation at 375px (no tooling required):

1. Open `https://kinder.patrykgolabek.dev` in Chrome
2. Open DevTools (F12 or Cmd+Option+I)
3. Click the device-toggle icon (or Ctrl+Shift+M / Cmd+Shift+M)
4. Set dimensions to `375` width (Responsive mode — this is labeled "Mobile M" in the preset bar)
5. Scroll horizontally on each page — any scrollbar at page level = FAIL
6. Check for: overlapping elements, text cut off, buttons too small to tap

Pages to check:
- `/` (landing page — InstallCommand is the known risk)
- `/installation/`
- `/quick-start/`
- `/configuration/`
- `/addons/metallb/` (representative doc page)

### Anti-Patterns to Avoid

- **Do not add `overflow-x: hidden` to `<body>` or `<html>`** — this masks horizontal scroll instead of fixing it and can clip sticky elements or popovers. Fix the source.
- **Do not audit against localhost** — Lighthouse scores on `localhost` differ significantly from production (no CDN, no HTTPS, no caching headers). Always audit against the live production URL.
- **Do not re-add sitemap configuration** — Starlight bundles sitemap internally. Adding `@astrojs/sitemap` separately would create a conflict (the Starlight issue tracker confirms this causes duplicate entries).
- **Do not force `target="_blank"` on all external links for Best Practices** — Starlight already includes `rel="me"` on social links. The Lighthouse Best Practices check for external links is specifically about `rel="noopener"` for links with `target="_blank"`. Standard external links without `target="_blank"` don't need these attributes.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Sitemap generation | Custom sitemap.xml.ts endpoint | Starlight bundled @astrojs/sitemap | Already active, already outputs correct URLs |
| Viewport meta tag | Custom head injection | Starlight built-in | Already in every page's `<head>` |
| Canonical URLs | Custom component | Starlight built-in | Already in every page's `<head>` |
| Lighthouse CI integration | GitHub Action | Manual CLI audit | Phase only requires a one-time pass, not CI gating |
| robots.txt generator | npm package (astro-robots-txt) | Plain `public/robots.txt` file | Static file is sufficient; no dynamic paths |

**Key insight:** This phase is about fixing two targeted gaps (mobile CSS + robots.txt) and then auditing. The heavy lifting — SEO metadata, sitemaps, accessibility markup — is already done by Starlight. The verification task is the real work.

---

## Common Pitfalls

### Pitfall 1: Auditing localhost Instead of Production
**What goes wrong:** Lighthouse reports high Performance scores locally (fast local server) but production score is different due to CDN behavior, HTTPS negotiation, and asset delivery from GitHub Pages.
**Why it happens:** Developers run `astro preview` and audit that URL.
**How to avoid:** Always run `lighthouse https://kinder.patrykgolabek.dev` — never a localhost URL.
**Warning signs:** If Performance score is 100 locally but you haven't deployed, the number is meaningless.

### Pitfall 2: Masking Mobile Overflow Instead of Fixing It
**What goes wrong:** Adding `overflow-x: hidden` to a parent element hides the horizontal scroll indicator but the element still overflows — Lighthouse and real devices still consider it broken.
**Why it happens:** It's a fast "fix" that makes the scrollbar disappear.
**How to avoid:** Use browser DevTools to identify the specific element causing overflow (red outline or computed width > viewport), then fix that element's CSS.
**Warning signs:** DevTools shows an element with `overflow-x: hidden` on the body — this is the masking pattern.

### Pitfall 3: robots.txt Returning 404 Failing Lighthouse SEO
**What goes wrong:** Lighthouse's robots.txt audit fetches `/robots.txt` — if it gets a 404, it reports "Lighthouse was unable to download a robots.txt file" which is treated as an invalid robots.txt, not N/A.
**Why it happens:** `public/robots.txt` doesn't exist, so GitHub Pages serves a 404 for that path.
**How to avoid:** Create `public/robots.txt` with a minimal valid content before auditing.
**Warning signs:** Lighthouse SEO score below 90 with "robots.txt is not valid" in the failed audits list.

### Pitfall 4: Lighthouse Throttling Making Performance Look Bad
**What goes wrong:** Lighthouse by default uses mobile network throttling (Moto G Power device emulation). GitHub Pages is fast but the simulated throttling can push LCP above 2.5s even for static sites.
**Why it happens:** Default Lighthouse mode simulates a slow 4G connection.
**How to avoid:** First audit with defaults (this is what the requirement measures). If Performance score is below 90, investigate actual LCP element — for a text-heavy Starlight site the LCP is usually the hero heading text, which is instant.
**Warning signs:** Performance score between 80-90 with "Reduce initial server response time" as the only audit failure — this is GitHub Pages TTFB variability, not a code issue. Re-run 2-3 times to average out.

### Pitfall 5: Comparison Grid Not Accounting for Viewport Width
**What goes wrong:** The `Comparison.astro` uses `@media (max-width: 50rem)` which is `800px`. At 375px this should trigger correctly — but if the page has a horizontal scroll issue from another element (like `InstallCommand`), the comparison section can appear clipped.
**Why it happens:** Horizontal overflow from one element pushes the computed viewport width.
**How to avoid:** Fix `InstallCommand` first, then re-check `Comparison`. Its media query is already correct.
**Warning signs:** Comparison shows two columns at 375px — this means the media query didn't fire, which means the viewport width calculation is wrong (likely because of horizontal scroll).

### Pitfall 6: Title Tag Shows "kinder | kinder" (Duplicate)
**What goes wrong:** The built HTML shows `<title>kinder | kinder</title>` for the index page. Lighthouse SEO checks that title elements are "descriptive." A duplicate title may be flagged.
**Why it happens:** Starlight constructs the title as `[page title] | [site title]`. The index.mdx has `title: kinder` and the site is named `kinder`.
**How to avoid:** Change the landing page frontmatter `title` to something more descriptive like `kinder — kind with batteries included` or use Starlight's `titleDelimiter` option. However, this is a low-risk item — Lighthouse doesn't penalize duplicate words in titles, only missing or very short titles.
**Warning signs:** Check if Lighthouse's SEO audit flags the title element specifically.

---

## Code Examples

### robots.txt (complete file)
```
# Source: https://developer.chrome.com/docs/lighthouse/seo/invalid-robots-txt
User-agent: *
Allow: /

Sitemap: https://kinder.patrykgolabek.dev/sitemap-index.xml
```

### InstallCommand.astro — Mobile Fix (style block addition)
```astro
<!-- Source: CSS responsive pattern — add to existing <style> block in InstallCommand.astro -->
<style>
  /* ... existing styles remain ... */

  @media (max-width: 50rem) {
    .install-command {
      flex-direction: column;
      align-items: stretch;
      max-width: 100%;
    }

    .copy-btn {
      width: 100%;
      padding: 0.5rem 0.625rem;
    }
  }
</style>
```

### Lighthouse CLI — Score Verification Script
```bash
# Run after deploying fixes to production
lighthouse https://kinder.patrykgolabek.dev \
  --output json \
  --output-path /tmp/lh-kinder.json \
  --only-categories=performance,accessibility,best-practices,seo \
  --quiet

node -e "
  const r = require('/tmp/lh-kinder.json');
  const cats = r.categories;
  let allPass = true;
  ['performance','accessibility','best-practices','seo'].forEach(k => {
    const score = Math.round(cats[k].score * 100);
    const pass = score >= 90;
    if (!pass) allPass = false;
    console.log(k + ': ' + score + ' [' + (pass ? 'PASS' : 'FAIL') + ']');
  });
  process.exit(allPass ? 0 : 1);
"
```

### Chrome DevTools — Check for Horizontal Overflow
```javascript
// Paste in DevTools console to find elements wider than viewport
const vw = document.documentElement.clientWidth;
document.querySelectorAll('*').forEach(el => {
  const rect = el.getBoundingClientRect();
  if (rect.right > vw) {
    console.log('Overflow element:', el, 'right:', rect.right, 'vw:', vw);
  }
});
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manually add sitemap integration | Starlight bundles @astrojs/sitemap | Starlight ~0.20 | Don't add @astrojs/sitemap separately |
| Run Lighthouse via PageSpeed Insights only | Lighthouse CLI v12 | 2024 | CLI gives reproducible results, JSON output, scriptable |
| `overflow-x: hidden` on body to "fix" mobile | Fix source element CSS | Always best practice | Actually fixes the issue rather than hiding it |

**Current Lighthouse version:** 12.x (requires Node 22 LTS). Install with `npm install -g lighthouse`.

---

## What the Planner Should Know About Scope

This phase has exactly two implementation tasks and one verification task:

1. **Fix `InstallCommand.astro`** — add mobile CSS (the only known mobile overflow risk)
2. **Add `public/robots.txt`** — single file, 3 lines, zero dependencies
3. **Audit + verify** — run Lighthouse against live URL, check mobile manually in DevTools

The Comparison.astro responsive behavior is already correct. Starlight's built-in accessibility, SEO metadata, and sitemap are already correct. The site is static HTML — Performance and Best Practices scores should be 90+ without changes, assuming GitHub Pages serves with HTTPS (it does via the CNAME/Pages setup already verified in Phase 9).

The planner should structure this as two implementation tasks (one per fix) and one verification task (Lighthouse CLI + DevTools manual check). Total implementation effort is small — this is a validation phase, not a build phase.

---

## Open Questions

1. **Lighthouse Performance score under throttling**
   - What we know: Starlight generates zero-JS static HTML. Astro builds optimized CSS bundles. GitHub Pages serves over HTTPS with caching headers.
   - What's unclear: The actual LCP time under Lighthouse's default mobile throttling (Moto G Power, slow 4G). GitHub Pages CDN latency from PageSpeed Insights' testing infrastructure.
   - Recommendation: Audit first without changes. If Performance is below 90, investigate LCP element specifically. Re-run 2-3 times to average out CDN variability. For a text-only hero (no hero image), LCP is the heading text which renders immediately.

2. **Duplicate title "kinder | kinder"**
   - What we know: The built HTML has `<title>kinder | kinder</title>` for the index page. Lighthouse checks that the title is descriptive.
   - What's unclear: Whether Lighthouse's current version penalizes this specific pattern.
   - Recommendation: If the SEO score is below 90, check the title audit. Can be fixed by changing the index.mdx frontmatter `title: kinder` to `title: Home` or a more descriptive value. Defer unless the audit actually fails.

3. **Lighthouse robots.txt audit — 404 behavior**
   - What we know: The live site returns 404 for `/robots.txt`. Multiple Lighthouse GitHub issues show 404 responses are flagged as "unable to download robots.txt" — not N/A. Safest action is to provide the file.
   - What's unclear: Whether the current Lighthouse version (12.x) treats a clean 404 as N/A or as a failure.
   - Recommendation: Add `public/robots.txt` regardless. It's a 3-line file with no downside.

---

## Sources

### Primary (HIGH confidence)
- Built HTML inspection (`/Users/patrykattc/work/git/kinder/kinder-site/dist/index.html`) — verified viewport meta, canonical, sitemap link, title, description, all Starlight-generated SEO fields present
- Sitemap files inspection (`dist/sitemap-index.xml`, `dist/sitemap-0.xml`) — verified Starlight generates sitemap automatically from `site` config
- `InstallCommand.astro` source inspection — confirmed `white-space: nowrap` + flex layout pattern causing mobile overflow risk
- `Comparison.astro` source inspection — confirmed `@media (max-width: 50rem)` responsive breakpoint already correct
- `astro.config.mjs` inspection — confirmed `site: 'https://kinder.patrykgolabek.dev'` set, required for sitemap generation
- [Starlight CSS Styling Guide](https://starlight.astro.build/guides/css-and-tailwind/) — confirmed `@media (min-width: 50rem)` is Starlight's primary breakpoint
- [Chrome Lighthouse Overview](https://developer.chrome.com/docs/lighthouse/overview/) — CLI usage and category list
- [Lighthouse robots.txt audit](https://developer.chrome.com/docs/lighthouse/seo/invalid-robots-txt) — what the audit checks

### Secondary (MEDIUM confidence)
- Live site curl: `curl -o /dev/null -w "%{http_code}" https://kinder.patrykgolabek.dev/robots.txt` returned 404 — confirms robots.txt is missing on production
- [Starlight GitHub issue #717](https://github.com/withastro/starlight/issues/717) — confirms Starlight bundles @astrojs/sitemap; cannot use separate instance
- [Lighthouse SEO audit guide (DebugBear)](https://www.debugbear.com/blog/lighthouse-seo-score) — lists all 14 SEO audits Lighthouse runs
- [Astro Performance Optimization Guide](https://eastondev.com/blog/en/posts/dev/20251202-astro-performance-optimization/) — Astro static sites achieve 90+ through zero-JS default and static HTML generation

### Tertiary (LOW confidence — for context only)
- [Starlight accessibility issue #2693](https://github.com/withastro/starlight/issues/2693) — historical accessibility findings; most may be fixed in 0.37.6 but not individually verified

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — verified from package.json and dist/ output
- Mobile fix pattern: HIGH — source code inspected, specific CSS lines identified
- robots.txt gap: HIGH — verified from live site 404 response
- Lighthouse tooling: HIGH — CLI usage from official docs
- Lighthouse scores: MEDIUM — depends on production CDN performance; Performance score in particular is network-dependent

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (Lighthouse scoring rules can change with major releases; Starlight 0.38+ may change sitemap behavior)
