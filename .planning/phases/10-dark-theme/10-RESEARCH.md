# Phase 10: Dark Theme - Research

**Researched:** 2026-03-01
**Domain:** Starlight CSS theming, FOUC prevention, Astro component overrides
**Confidence:** HIGH

## Summary

Phase 10 adds a terminal-aesthetic dark theme to the existing Astro/Starlight site and ensures it never flashes white during load or navigation. This is a two-part problem: (1) overriding Starlight's default blue accent colors with cyan variants for the "dark terminal" look, and (2) preventing flash of unstyled content (FOUC) when the page loads.

The good news: Starlight already defaults to dark mode. Its `ThemeProvider.astro` component contains an inline script that runs synchronously in `<head>` before any paint, reading `localStorage['starlight-theme']` and setting `document.documentElement.dataset.theme` immediately. Dark mode is the fallback when no localStorage value is set and the system preference is not explicitly light. So FOUC for first-time visitors is already handled — the site defaults to dark out of the box.

The work for this phase is: (1) override the accent color CSS variables (`--sl-color-accent*`) from Starlight's default blue-purple hue to cyan, and (2) deepen the background if desired below Starlight's default `hsl(224, 10%, 10%)`. Both are accomplished via a custom CSS file registered in `customCss` in `astro.config.mjs`. The FOUC prevention is already provided by the installed `ThemeProvider.astro` — no custom override of that component is needed.

**Primary recommendation:** Add `./src/styles/theme.css` with overridden `--sl-color-accent*` variables in `:root` (dark mode) and `:root[data-theme='light']` (light mode). Register it in `astro.config.mjs` via `customCss`. No `ThemeProvider` component override is needed — Starlight's default already prevents FOUC.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| @astrojs/starlight | ^0.37.6 (installed) | Documentation theme with built-in theming | Already installed; owns ThemeProvider, ThemeSelect, CSS variables |
| CSS Custom Properties | N/A (browser native) | Override Starlight's color palette | Starlight exposes all colors as CSS variables; no additional library needed |

### Supporting
| File | Purpose | When to Use |
|------|---------|-------------|
| `src/styles/theme.css` | Custom CSS for color overrides | Created this phase; registered via `customCss` |
| `src/components/ThemeProvider.astro` | Custom ThemeProvider override | Only needed if you must force dark mode to always override system preference; NOT needed for this phase's requirements |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| CSS custom property overrides | ThemeProvider component override | ThemeProvider override is more complex and fragile across Starlight upgrades; CSS-only approach is stable and sufficient |
| CSS variable approach | Starlight theme plugin (e.g., starlight-theme-black) | Third-party theme plugins add a dependency; CSS variables are the official supported path |
| Leaving accent as default blue | Cyan accent via CSS override | Phase requirement specifies "cyan accent colors matching dark terminal aesthetic" |

**No new npm packages required.** All work is CSS + config changes.

## Architecture Patterns

### Recommended Project Structure
```
kinder-site/
├── astro.config.mjs       # Add customCss array with './src/styles/theme.css'
└── src/
    ├── styles/
    │   └── theme.css      # New: color variable overrides for dark terminal aesthetic
    └── components/        # Not needed this phase — no component overrides required
```

### Pattern 1: CSS Custom Property Override (the standard Starlight theming pattern)

**What:** Create a custom CSS file that overrides Starlight's CSS variables. Register it in `customCss`. Starlight applies your variables on top of its defaults because custom CSS is unlayered and takes precedence over Starlight's `@layer starlight.base` rules.

**When to use:** Any time you want to change colors, fonts, or spacing in Starlight without touching component code.

**Example:**
```css
/* Source: https://starlight.astro.build/guides/css-and-tailwind/ */
/* kinder-site/src/styles/theme.css */

/* Dark mode overrides (applied by default — :root is dark) */
:root {
  /* Cyan accent: replace Starlight's default blue (hsl 224) with cyan (hsl 185) */
  --sl-color-accent-low:  hsl(185, 54%, 15%);
  --sl-color-accent:      hsl(185, 100%, 50%);
  --sl-color-accent-high: hsl(185, 100%, 80%);

  /* Near-black background: deepen from Starlight default hsl(224, 10%, 10%) */
  --sl-color-black: hsl(220, 13%, 8%);
}

/* Light mode overrides (for users who switch to light) */
:root[data-theme='light'] {
  --sl-color-accent-high: hsl(185, 80%, 25%);
  --sl-color-accent:      hsl(185, 90%, 45%);
  --sl-color-accent-low:  hsl(185, 88%, 88%);
}
```

### Pattern 2: Register customCss in astro.config.mjs

**What:** Point Starlight to your custom CSS file.

**Example:**
```javascript
// Source: https://starlight.astro.build/guides/css-and-tailwind/
// kinder-site/astro.config.mjs
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  integrations: [
    starlight({
      title: 'kinder',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/patrykattc/kinder' },
      ],
      customCss: [
        './src/styles/theme.css',
      ],
    }),
  ],
});
```

### Pattern 3: How Starlight's Existing FOUC Prevention Works

**What:** The installed `ThemeProvider.astro` already contains an inline `<script is:inline>` that Starlight places in `<head>`. It runs synchronously before any HTML is painted, so the theme is applied before the browser renders any content.

**How it works (from installed source at `node_modules/@astrojs/starlight/components/ThemeProvider.astro`):**
```javascript
// Runs synchronously in <head> before first paint
window.StarlightThemeProvider = (() => {
  const storedTheme =
    typeof localStorage !== 'undefined' && localStorage.getItem('starlight-theme');
  const theme =
    storedTheme ||
    (window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark');
  document.documentElement.dataset.theme = theme === 'light' ? 'light' : 'dark';
  return { updatePickers(theme = storedTheme || 'auto') { /* ... */ } };
})();
```

**Key facts:**
- localStorage key: `'starlight-theme'`
- Default when no localStorage and no system preference: `'dark'` (system preference must explicitly be `light` to get light mode)
- The `dataset.theme` is set before any CSS is loaded, so CSS `[data-theme='dark']` rules apply from the first paint
- This is already working in the installed Starlight — no new component override is needed

**When component override IS needed:** If the requirement were "force dark mode, remove the toggle entirely" — that would require overriding `ThemeProvider` and `ThemeSelect`. This phase does NOT have that requirement. Requirement 3 says "the toggle persists," confirming the toggle stays.

### Pattern 4: ThemeSelect Persistence (Already Implemented)

**What:** The toggle stores user choice in `localStorage['starlight-theme']` as `'dark'`, `'light'`, or `''` (for auto). The `ThemeProvider` inline script reads it on every page load.

**How it works (from `ThemeSelect.astro` source):**
```javascript
const storageKey = 'starlight-theme';

function storeTheme(theme) {
  localStorage.setItem(storageKey, theme === 'light' || theme === 'dark' ? theme : '');
}

function onThemeChange(theme) {
  document.documentElement.dataset.theme = theme === 'auto' ? getPreferredColorScheme() : theme;
  storeTheme(theme);
}
```

**This already satisfies success criterion 3** — toggle persistence across navigations and sessions is built into Starlight. No additional work needed.

### Anti-Patterns to Avoid
- **Overriding `ThemeProvider` to hardcode `data-theme='dark'`:** Breaks the toggle — users can't switch to light. The phase requirement explicitly says the toggle must persist the chosen mode.
- **Putting CSS in `src/content/` or inline `<style>` in MDX:** These don't affect the global theme. CSS must be in `customCss`.
- **Using `!important` broadly:** Starlight's CSS uses `@layer`, so unlayered custom CSS already wins without `!important`. Using it creates specificity problems.
- **Forgetting light mode overrides:** If you only override dark mode colors, users who switch to light mode may see cyan on white with poor contrast. Always provide `[data-theme='light']` overrides.
- **Picking cyan values without checking contrast:** WCAG AA requires 4.5:1 for normal text. The accent is used for links (`--sl-color-text-accent` = `--sl-color-accent-high`). Verify the chosen cyan lightness value passes contrast against the background.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| FOUC prevention | Custom inline script in custom layout/head | Starlight's built-in `ThemeProvider.astro` | Already in `<head>` as `is:inline`; runs before paint; handles localStorage + system preference |
| Theme toggle persistence | Custom localStorage logic | Starlight's built-in `ThemeSelect.astro` | Already writes to `localStorage['starlight-theme']`; already reads on load |
| Color theming | Custom CSS classes / data attributes | CSS custom property overrides via `customCss` | Starlight already applies `data-theme` attribute; CSS variables cascade correctly |

**Key insight:** Starlight already solves all three of the phase's success criteria at the framework level. The only missing piece is the custom color palette (cyan instead of blue). The implementation is one CSS file + one config line.

## Common Pitfalls

### Pitfall 1: CSS Specificity Fight with Starlight's @layer

**What goes wrong:** Developer puts custom CSS in a `@layer` block expecting it to override Starlight, but instead Starlight's layer rules win.

**Why it happens:** CSS cascade order for layers: unlayered CSS always beats layered CSS regardless of specificity. Starlight's colors are in `@layer starlight.base`. If your CSS is also in a layer, cascade order determines priority.

**How to avoid:** Do NOT wrap your override CSS in `@layer`. Keep it as plain unlayered `:root { }` rules. Unlayered CSS overrides all layered CSS.

**Warning signs:** Color overrides in CSS file appear to have no effect; browser devtools show Starlight's values winning.

### Pitfall 2: Editing Wrong Selector for Dark Mode Background

**What goes wrong:** Developer tries to change the dark mode background by overriding `--sl-color-bg` directly, but `--sl-color-bg` is defined as `var(--sl-color-black)` — so `--sl-color-bg` is only an alias. The variable to override is `--sl-color-black` (which in dark mode holds the actual background color).

**Why it happens:** Starlight's variable naming is counterintuitive: in dark mode, `--sl-color-black` IS the background (near-black), and `--sl-color-white` IS used for bright text. In light mode, the mapping inverts — `--sl-color-black` becomes white, and `--sl-color-white` becomes near-black.

**How to avoid:** Override `--sl-color-black` in `:root` (dark mode) to change the background color. Override `--sl-color-black` in `:root[data-theme='light']` to change the light mode background (it becomes white).

**Warning signs:** Overriding `--sl-color-bg` has no effect; you must override the source variable.

### Pitfall 3: Cyan Contrast Failure

**What goes wrong:** Chosen cyan accent looks visually great but fails WCAG contrast checks. The accent is used for link text (`--sl-color-text-accent` = `--sl-color-accent-high`) and navigation highlights.

**Why it happens:** Cyan at full saturation (`hsl(185, 100%, 50%)`) is often fine as a background accent but too bright as text on near-black if the lightness exceeds ~85% (may wash out) or is below ~60% on near-black backgrounds.

**How to avoid:** Use the Starlight color theme editor at https://starlight.astro.build/guides/css-and-tailwind/ to preview and validate. Aim for `--sl-color-accent-high` (used for text links in dark mode) at ~70-85% lightness against a near-black background.

**Warning signs:** Links appear washed out or invisible; browser devtools accessibility panel flags contrast ratio below 4.5:1.

### Pitfall 4: FOUC on Page Navigation with View Transitions

**What goes wrong:** If View Transitions are enabled (Astro's `<ViewTransitions />`), page navigation replaces the DOM but the theme script in `<head>` may not re-run, causing a momentary wrong theme on transition.

**Why it happens:** View Transitions swap page content using the browser's transition API. The inline script in `<head>` runs once on initial load but may not re-execute on soft navigation.

**How to avoid:** Starlight does NOT use Astro's `<ViewTransitions />` by default — it uses its own navigation system. This pitfall only applies if you add View Transitions manually. For this phase, which doesn't add View Transitions, this is not a concern.

**Warning signs:** Theme flashes only during page-to-page navigation, not on initial load.

## Code Examples

Verified patterns from official sources:

### Complete theme.css for Terminal Aesthetic

```css
/* Source: CSS variable names from props.css in installed Starlight 0.37.6
   https://starlight.astro.build/guides/css-and-tailwind/ */
/* kinder-site/src/styles/theme.css */

/* ===== Dark mode (default) — terminal aesthetic ===== */
:root {
  /* Cyan accent replaces Starlight's default blue/purple (hsl 224) */
  /* These values can be adjusted — check contrast at https://starlight.astro.build/guides/css-and-tailwind/ */
  --sl-color-accent-low:  hsl(185, 54%, 15%);   /* Deep cyan for bg badges */
  --sl-color-accent:      hsl(185, 100%, 45%);   /* Cyan for interactive elements */
  --sl-color-accent-high: hsl(185, 100%, 75%);   /* Light cyan for link text */

  /* Near-black background — slightly deeper than Starlight default hsl(224, 10%, 10%) */
  --sl-color-black: hsl(220, 15%, 7%);
}

/* ===== Light mode overrides — maintain cyan identity in light mode ===== */
:root[data-theme='light'] {
  --sl-color-accent-high: hsl(185, 80%, 22%);   /* Dark cyan for links on white bg */
  --sl-color-accent:      hsl(185, 90%, 40%);   /* Cyan for buttons/highlights */
  --sl-color-accent-low:  hsl(185, 88%, 88%);   /* Light cyan bg for badges */
}
```

### Updated astro.config.mjs

```javascript
// Source: https://starlight.astro.build/guides/css-and-tailwind/
// kinder-site/astro.config.mjs
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  integrations: [
    starlight({
      title: 'kinder',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/patrykattc/kinder' },
      ],
      customCss: [
        './src/styles/theme.css',
      ],
    }),
  ],
});
```

### Verifying the theme applies correctly

```bash
# Build the site and check that the CSS file is included
cd /path/to/kinder/kinder-site
npm run build

# Check that the custom CSS variables appear in the built output
grep -r "sl-color-accent" dist/_astro/*.css | head -5
```

### Verify FOUC prevention is working

Open the site in a browser with DevTools. In the Network tab, throttle to "Slow 3G." Reload. The background should be near-black from the very first paint with no white flash. This works because the `ThemeProvider.astro` inline script sets `document.documentElement.dataset.theme = 'dark'` before any CSS is loaded.

### Inspect existing ThemeProvider behavior

```bash
# Confirm the installed ThemeProvider already has the inline script
cat kinder-site/node_modules/@astrojs/starlight/components/ThemeProvider.astro
# Expected output: <script is:inline> with localStorage['starlight-theme'] logic
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Custom layout override with inline `<style>` in `<head>` | `customCss` array in `starlight()` config | Starlight 0.x initial release | Simpler, no Astro layout boilerplate needed |
| Forcing dark mode via ThemeProvider component override | CSS variable override only (FOUC already prevented by default ThemeProvider) | Starlight has always had this | Component overrides are fragile across upgrades; CSS-only is preferred |
| Separate `prefers-color-scheme` CSS media queries | `[data-theme='dark']` / `[data-theme='light']` attribute selectors | Starlight design from the start | Attribute selectors override system preference when user has explicitly toggled |

**Deprecated/outdated:**
- Component override of `ThemeProvider` just to default to dark: Not needed as of at least Starlight 0.x — system preference `dark` is the default fallback already
- `@astrojs/starlight-tailwind` package: Only needed if using Tailwind CSS; this project explicitly excludes Tailwind (per REQUIREMENTS.md Out of Scope)

## Open Questions

1. **Exact cyan HSL values for the terminal aesthetic**
   - What we know: The requirement says "near-black background with cyan accent colors matching the dark terminal aesthetic" — no exact hex values specified
   - What's unclear: Whether specific values like `#00ffff` (pure cyan) or a softer variant are intended; whether there's a reference design
   - Recommendation: The planner should choose values that work visually and pass WCAG AA (4.5:1 contrast ratio for text). Starting point: `hsl(185, 100%, 45%)` for accent, `hsl(185, 100%, 75%)` for accent-high (links). Verify with the Starlight color theme editor. Final values are implementation details for the executor.

2. **Whether to override the gray scale for a deeper, more terminal-like sidebar/nav**
   - What we know: The default gray-6 (sidebar bg in dark mode) is `hsl(224, 14%, 16%)` — slightly lighter than the background
   - What's unclear: Whether the terminal aesthetic requires matching the gray scale's hue to cyan (185) or keeping the default blue-gray (224)
   - Recommendation: Keep gray scale as-is for Phase 10. The accent color change is the primary visual shift. Gray scale tuning is polish work (Phase 14).

## Sources

### Primary (HIGH confidence)
- `/kinder-site/node_modules/@astrojs/starlight/components/ThemeProvider.astro` — Exact FOUC prevention script, localStorage key `'starlight-theme'`, `dataset.theme` mechanism (read directly from installed package)
- `/kinder-site/node_modules/@astrojs/starlight/components/ThemeSelect.astro` — Exact theme toggle persistence logic, `storeTheme()` function (read directly from installed package)
- `/kinder-site/node_modules/@astrojs/starlight/style/props.css` — All CSS custom properties with exact HSL values for dark mode (`:root`) and light mode (`:root[data-theme='light']`) (read directly from installed package)
- https://starlight.astro.build/guides/css-and-tailwind/ — Official `customCss` option format, CSS selector patterns, color theme editor reference

### Secondary (MEDIUM confidence)
- https://github.com/withastro/starlight/discussions/949 — Confirmed (Sep 2025): no native `defaultTheme` config option exists; ThemeProvider override is the workaround, but CSS-only approach works for color changes
- https://starlight.astro.build/reference/overrides/ — ThemeProvider component override API (confirmed available via `components:` config option)

### Tertiary (LOW confidence)
- https://github.com/AVGVSTVS96/astro-fouc-killer — Third-party FOUC prevention library; NOT needed here since Starlight's built-in ThemeProvider already handles it
- Community reports of View Transitions causing FOUC on navigation — Not verified against Starlight 0.37.x; only applies if View Transitions are added manually

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Read directly from installed package source files; all APIs verified against official docs
- Architecture (CSS variables): HIGH — CSS variable names, values, and selectors read from `props.css` in installed Starlight 0.37.6
- FOUC prevention mechanism: HIGH — Read directly from installed `ThemeProvider.astro` and `ThemeSelect.astro`; confirmed matches GitHub source
- Pitfalls: HIGH — Derived from actual CSS cascade rules and installed source; @layer behavior is a CSS specification fact
- Exact cyan color values: LOW — No reference design was provided; specific HSL numbers are recommendations, not verified requirements

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (Starlight is pre-1.0 and releases frequently; verify if major version bumps occur)
