# Pitfalls Research: Kinder Website

**Domain:** Astro static site on GitHub Pages with custom domain, dark theme, and developer-tool aesthetic — added to an existing Go CLI monorepo
**Researched:** 2026-03-01
**Confidence:** HIGH (official Astro docs + GitHub Pages docs + verified community issues)

---

## Critical Pitfalls

Mistakes that cause the site to not load at all, custom domain to silently reset, or assets to 404 in production.

---

### Pitfall C1: Jekyll Strips the `_astro` Assets Folder

**What goes wrong:**
The site builds locally and the Astro dev server works perfectly. After deploying to GitHub Pages via a branch push, the site loads but has no styles, no JavaScript, and all images are broken. The 404 errors point to paths under `/_astro/...`.

**Why it happens:**
GitHub Pages runs all content through Jekyll by default. Jekyll ignores any file or directory whose name starts with an underscore — treating them as Jekyll internals. Astro outputs all compiled JS, CSS, and image hashes into `dist/_astro/` by default. Jekyll silently drops the entire folder. The site HTML loads (it does not start with an underscore) but every asset reference 404s.

**How to avoid:**
Two complementary fixes — apply both:

1. Add an empty `.nojekyll` file to the Astro `public/` directory. Astro copies `public/` verbatim to `dist/`, so `.nojekyll` ends up at the root of the deploy artifact and tells GitHub Pages to skip Jekyll entirely.

2. Change the assets directory name in `astro.config.mjs`:
```js
export default defineConfig({
  build: {
    assets: 'assets',  // replaces default '_astro'
  },
});
```
Fix 1 alone is sufficient if using GitHub Actions deployment (which is recommended). Fix 2 is belt-and-suspenders and protects against future GitHub Pages policy changes around underscores.

**Warning signs:**
- CSS/JS 404s after deploy; works in `astro dev` and `astro preview`
- Browser network tab shows requests to `/_astro/*.js` returning 404
- GitHub Pages deploy log shows no Jekyll errors (Jekyll silently drops the folder)

**Phase to address:** Phase 1 (repo scaffolding and initial deploy) — must be done before the first production deploy

---

### Pitfall C2: Custom Domain Resets to `username.github.io` on Every Deploy

**What goes wrong:**
Custom domain `kinder.patrykgolabek.dev` is configured in GitHub Pages settings. It works. Then a new commit is pushed and the deploy workflow runs. After the deploy, the custom domain field in repository Settings → Pages is blank, and the site reverts to the default `username.github.io` URL. This happens silently on every push.

**Why it happens:**
GitHub Pages stores the custom domain setting as a `CNAME` file in the root of the deployment artifact. When deploying from a branch, if the branch does not include a `CNAME` file, the deploy overwrites the existing one with nothing. The GitHub UI setting writes the CNAME file, but if the build pipeline replaces the branch contents, the file is gone.

When deploying via GitHub Actions (the recommended method for Astro), the CNAME file is handled differently: the `actions/deploy-pages` action does not read the CNAME from the branch — it reads it from the Pages settings API. This means the CNAME file in the repo source controls the domain, and if it is missing from the build output, the domain resets.

**How to avoid:**
Create `public/CNAME` in the Astro project with a single line containing the custom domain:
```
kinder.patrykgolabek.dev
```
Astro copies `public/` verbatim to `dist/`. The CNAME file will be in the deploy artifact on every build. This is the one permanent fix — commit it once and never touch it again.

Also set `site` in `astro.config.mjs` to match:
```js
export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
});
```

**Warning signs:**
- Site URL reverts to `patrykgolabek.github.io/kinder` after a push
- GitHub Settings → Pages shows blank "Custom domain" field
- HTTPS certificate is revoked and re-issued after every deploy

**Phase to address:** Phase 1 (initial site scaffolding) — prevents an entire class of silent deploy regressions

---

### Pitfall C3: `base` Config Set When Using a Custom Domain (Breaks All Links)

**What goes wrong:**
Developer follows GitHub Pages tutorials that say "set `base: '/repo-name'`" and does so in `astro.config.mjs`. The site deploys to the custom domain `kinder.patrykgolabek.dev` but all internal links resolve as `kinder.patrykgolabek.dev/kinder/docs/...` (double prefix), navigation fails, and CSS/JS paths break.

**Why it happens:**
The `base` config option is only needed when deploying to a subdirectory path (e.g., `username.github.io/kinder/`). When using a custom domain at the root (`kinder.patrykgolabek.dev`), the site is served from `/`, and `base` must not be set (or set to `'/'`). Many tutorials conflate the two cases because most tutorial authors deploy to the default `*.github.io` URL.

**How to avoid:**
- When using a custom domain: set `site` only, omit `base` (or explicitly set `base: '/'`)
- When using `username.github.io/kinder` (no custom domain): set both `site` and `base: '/kinder'`
- Never hardcode absolute paths in components; use Astro's built-in `<a href="/">` and image imports which respect the `base` setting automatically

Correct config for this project:
```js
export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  // base: NOT set — custom domain serves from root
});
```

**Warning signs:**
- `console.log(import.meta.env.BASE_URL)` returns `/kinder/` instead of `/`
- Navigation links point to `/kinder/docs` instead of `/docs`
- Canonical URLs in `<head>` include the repo name twice

**Phase to address:** Phase 1 (initial Astro configuration)

---

### Pitfall C4: `astro.config.mjs` `site` Mismatch Breaks Sitemap and Canonical URLs

**What goes wrong:**
Sitemap entries and canonical `<link>` tags reference the wrong domain — either the old `username.github.io` URL or `localhost:4321` from development — causing duplicate content signals to search engines and social preview cards pointing to the wrong URL.

**Why it happens:**
`site` in `astro.config.mjs` is the canonical source of truth for sitemaps, canonical links, OG URLs, and `Astro.site`. If left unset during development, `Astro.site` is `undefined` and the sitemap integration skips generation entirely. If set to the wrong URL (e.g., copied from a tutorial), every canonical tag points elsewhere.

**How to avoid:**
Set `site` immediately when scaffolding the project, before writing any page content:
```js
site: 'https://kinder.patrykgolabek.dev',
```
The `@astrojs/sitemap` integration reads this value at build time. Verify by checking `dist/sitemap-index.xml` after `astro build` — every URL must start with `https://kinder.patrykgolabek.dev`.

**Warning signs:**
- Sitemap not generated at all (`dist/sitemap-index.xml` missing)
- OG image preview URLs contain `localhost` or `github.io`
- Google Search Console shows canonical pointing to wrong URL

**Phase to address:** Phase 1 (initial Astro configuration)

---

### Pitfall C5: Dark Mode FOUC — Page Flashes White Before Script Applies Dark Theme

**What goes wrong:**
On every page load or navigation, the page briefly renders in white/light mode before snapping to dark. On slow connections or fast machines with visible repaints, this produces a jarring flash. Using `prefers-color-scheme` alone does not eliminate it because the browser must still parse CSS and run JavaScript before applying the correct class.

**Why it happens:**
Astro sends HTML from the server with no knowledge of the user's theme preference stored in `localStorage`. The browser renders the HTML with default (light) styles first, then client-side JavaScript reads `localStorage`, applies the `dark` class to `<html>`, and Tailwind's `dark:` variants kick in. The visible interval between initial paint and script execution is the FOUC.

This is compounded by Astro's View Transitions: navigating between pages triggers a new page load, resetting the document before the `astro:after-swap` event fires. Without explicit handling, theme flickers on every View Transition navigation even if the initial load is fine.

**How to avoid:**
Inject a blocking inline script in the `<head>` — before any CSS or body content — that reads `localStorage` and applies the dark class synchronously:

```html
<!-- In your base layout's <head>, with is:inline to prevent bundling -->
<script is:inline>
  (function() {
    const stored = localStorage.getItem('theme');
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    if (stored === 'dark' || (!stored && prefersDark)) {
      document.documentElement.classList.add('dark');
    }
  })();
</script>
```

For View Transitions, also listen to `astro:after-swap`:
```js
document.addEventListener('astro:after-swap', () => {
  const stored = localStorage.getItem('theme');
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  if (stored === 'dark' || (!stored && prefersDark)) {
    document.documentElement.classList.add('dark');
  }
});
```

The `is:inline` directive is critical: Astro normally bundles and defers scripts, which delays execution until after paint. `is:inline` forces the script to remain in the `<head>` and execute synchronously before the body renders.

**Warning signs:**
- Page visibly flashes light on load in dark mode
- Flash is reproducible with browser DevTools throttled to "Slow 3G"
- Flash occurs on every page navigation when View Transitions are enabled
- `document.documentElement.classList` does not contain `dark` during `DOMContentLoaded`

**Phase to address:** Phase 2 (dark theme implementation) — must be baked in from the start, not patched later

---

### Pitfall C6: GitHub Actions Workflow Lacks Required `pages: write` and `id-token: write` Permissions

**What goes wrong:**
The GitHub Actions workflow runs, Astro builds successfully, but the deploy step fails with: `Error: HttpError: Resource not accessible by integration` or `RequestError: Deployment not created`. The site is never published.

**Why it happens:**
Deploying to GitHub Pages via `actions/deploy-pages` requires two non-default permissions: `pages: write` (to create a Pages deployment) and `id-token: write` (to request an OIDC JWT token for authentication). The default `GITHUB_TOKEN` in a new workflow does not have either. Additionally, the repository Pages setting must be configured to use "GitHub Actions" as the deployment source — using "Deploy from a branch" disables the Actions-based deploy API.

**How to avoid:**
The workflow must explicitly declare:
```yaml
permissions:
  contents: read
  pages: write
  id-token: write
```

And the repository Pages setting (Settings → Pages → Source) must be set to "GitHub Actions", not "Deploy from a branch".

Also required: the deploy job must reference the `github-pages` environment:
```yaml
environment:
  name: github-pages
  url: ${{ steps.deployment.outputs.page_url }}
```

**Warning signs:**
- Astro build step passes, deploy step fails with HTTP 403 or resource error
- Deployment shows in Actions but no corresponding Pages deployment appears in Settings → Pages
- The `pages-build-deployment` workflow is not triggered after the custom workflow runs

**Phase to address:** Phase 1 (CI/CD pipeline setup)

---

### Pitfall C7: Workflow Triggers Without Path Filtering — Every Go Code Change Triggers Site Rebuild

**What goes wrong:**
The site deploy workflow runs on every push to `main`, including Go source changes that have nothing to do with the website. This wastes CI minutes, causes unnecessary deploys, and adds noise to the deployment history.

**Why it happens:**
GitHub Actions workflows trigger on `push` to the specified branch by default, with no file-path filtering. Without path filtering, a change to `pkg/cluster/create.go` triggers a full Astro build and Pages deploy.

The inverse problem also exists: Go CI workflows (which already use `paths-ignore: ['site/**']`) correctly skip site changes, but the new site workflow must similarly ignore Go source changes.

**How to avoid:**
Scope the site deploy workflow to run only on changes under `kinder-site/`:
```yaml
on:
  push:
    branches: [main]
    paths:
      - 'kinder-site/**'
  workflow_dispatch:
```

The `withastro/action` action supports a `path` input for projects not at the repo root:
```yaml
- uses: withastro/action@v3
  with:
    path: kinder-site/
```

Also ensure all existing Go CI workflows include `kinder-site/**` in their `paths-ignore` (the existing workflows already use `site/**` for the Hugo site — update this to cover `kinder-site/**`).

**Warning signs:**
- GitHub Actions → Deployments shows tens of deploys per week from unrelated commits
- Site rebuild takes several minutes on every Go code change
- Actions bill shows unexpected usage

**Phase to address:** Phase 1 (CI/CD pipeline setup)

---

## Technical Debt Patterns

Shortcuts that seem fine during initial build but create rework.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hardcode absolute image paths in Markdown (`/images/hero.png`) instead of using Astro imports | Simpler docs writing | Breaks if `base` is ever set; images never optimized by Astro | Never — use `public/` for images that must be absolute, or import for optimization |
| Use `theme="dark"` class fixed in HTML rather than system-preference detection | No FOUC | Users with light-mode preference see dark forcibly; no toggle possible | Only if explicitly building a "dark only" site with no toggle |
| Deploy by pushing build output to `gh-pages` branch instead of GitHub Actions | Simpler initial setup | CNAME reset on every push; no build cache; leaks build artifacts into repo history | Never — use GitHub Actions source |
| Skip lockfile commit (`package-lock.json` or `pnpm-lock.yaml`) | Fewer git changes | `withastro/action` cannot detect package manager; falls back to npm which may conflict; Dependabot cannot pin deps | Never — commit the lockfile |
| Use `@astrojs/starlight` for docs instead of hand-built pages | Faster docs setup | Hard to match a fully custom dark developer-tool aesthetic; Starlight has opinionated CSS hard to override | Acceptable if design flexibility is not needed; not appropriate for a custom visual identity |
| Keep the Hugo `site/` directory alongside `kinder-site/` | Preserves kind upstream compat | Confusion about which site is authoritative; two build systems in one repo | Only during transition; remove Hugo `site/` once Astro site is live |

---

## Integration Gotchas

Common mistakes when connecting the Astro site to external services and the repo.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| GitHub Pages custom domain | Configuring domain only in GitHub UI settings | Add `public/CNAME` to the Astro project so CNAME persists through every deploy |
| GitHub Pages HTTPS enforcement | Trying to enable "Enforce HTTPS" immediately after adding the domain | Wait up to 1 hour for GitHub to provision the Let's Encrypt certificate; HTTPS checkbox is greyed out until cert is issued |
| DNS for `kinder.patrykgolabek.dev` (subdomain) | Adding an A record (apex-only) | Add a `CNAME` record for `kinder` pointing to `patrykgolabek.github.io` — subdomains use CNAME, not A records |
| DNS propagation and HTTPS cert | Expecting HTTPS to work immediately after DNS change | DNS propagation takes up to 24 hours; GitHub cert provisioning takes an additional hour after DNS resolves correctly; plan 2+ hours between DNS change and testing HTTPS |
| `withastro/action` with `kinder-site/` subdirectory | Pointing workflow at repo root | Use `path: kinder-site/` input; ensure `out-dir` matches Astro's `outDir` config (default `dist`) |
| Go CI existing workflows | Forgetting to update `paths-ignore` | Add `kinder-site/**` to `paths-ignore` in all four existing Go CI workflow files (`docker.yaml`, `nerdctl.yaml`, `podman.yml`, `vm.yaml`) |
| `netlify.toml` conflict | Netlify config exists at repo root pointing to `site/` (Hugo); GitHub Pages picks different source | `netlify.toml` does not affect GitHub Pages; the file is inert but can confuse contributors — add a comment clarifying that Netlify is for the old kind upstream Hugo site, not the kinder Astro site |
| Dark mode and `prefers-color-scheme` on initial deploy | Assuming the CSS media query eliminates FOUC without JavaScript | The media query controls CSS only; without the blocking inline script, the JS-managed `dark` class causes flicker regardless |

---

## Performance Traps

Patterns that build fast but degrade the experience.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Unoptimized images in `public/` | Slow LCP; hero images over 500 KB | Use Astro's `<Image>` component with `src/assets/` imports for images that need optimization; reserve `public/` for favicons and icons | Immediately on Lighthouse audit |
| All docs in a single long Markdown file | Content loads fast initially; no granular caching | Structure docs as individual `.md` files in a content collection; each gets its own URL and CDN cache entry | When docs grow beyond 5–10 sections |
| No `astro:prefetch` on navigation links | Each docs page feels slow on click | Add `<link rel="prefetch">` via Astro's prefetch integration or enable `prefetch: true` in Astro config | Any page with navigation to multiple docs |
| Code blocks without syntax highlighting lazy loading | Long TTFB on first code block render | Use Astro's built-in Shiki syntax highlighter (zero JS to client); configure in `astro.config.mjs` with `shikiConfig` | First page with a code block |
| No `concurrency` group in deploy workflow | Rapid pushes queue multiple simultaneous Pages deployments; GitHub Pages rejects all but one | Add `concurrency: { group: "pages", cancel-in-progress: false }` to the deploy workflow | After the second rapid push to main |

---

## Security Mistakes

Domain-specific security issues for a static developer-tool site.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Storing API keys or tokens in Astro env vars committed to repo | Credential leak in public repo | Static sites have no server — any secret in the build is in the HTML; use only `PUBLIC_` prefixed vars, which are expected to be public |
| Not enforcing HTTPS on the custom domain | Traffic to `http://kinder.patrykgolabek.dev` is unencrypted | Enable "Enforce HTTPS" in Settings → Pages as soon as the cert is provisioned |
| CAA DNS record blocking Let's Encrypt | GitHub Pages cannot provision TLS certificate | If the DNS zone has CAA records, add `0 issue "letsencrypt.org"` or the HTTPS certificate issuance fails silently |
| Wide RBAC token printed in docs | If docs show an example token, bots scrape it | Use obviously-fake placeholder tokens in documentation examples (`<your-token-here>`, never real base64 JWT fragments) |

---

## UX Pitfalls

Common mistakes in developer-tool site design.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Light-mode-only design | Alienates developers who use dark terminals and IDEs; kinder's target audience (DevOps/platform engineers) skews strongly toward dark mode | Implement dark mode as the default; offer a toggle for light mode; test both |
| Missing "copy to clipboard" on code blocks | Users manually select and copy CLI commands; frequent source of copy errors with trailing newlines | Add copy button to every `<pre>` code block; Astro + Shiki make this straightforward with a small client-side script |
| Navigation with no active state | Users cannot tell which docs page they are on | Use `Astro.url.pathname` to compare against nav link `href` and apply an active class |
| Docs page with no table of contents | Long docs pages feel unmapped | Generate ToC from heading anchors; Astro's `remark-toc` plugin or a custom component using `getHeadings()` |
| Hero section with no immediate CLI command | Landing page is abstract; developers want to see the install command immediately | Put `brew install / go install` command front and center on the landing page, before feature list |
| OG image is the default GitHub social card | Social previews look unprofessional when shared on Slack or Twitter/X | Create a custom OG image (`og-image.png` in `public/`) and reference it in the base layout's `<meta property="og:image">` tag |

---

## "Looks Done But Isn't" Checklist

Things that appear complete in local dev but are missing or broken in production.

- [ ] **Custom domain:** Verify `public/CNAME` exists and contains `kinder.patrykgolabek.dev` — open `dist/CNAME` after `astro build`
- [ ] **Jekyll bypass:** Verify `public/.nojekyll` exists — open `dist/.nojekyll` after `astro build`
- [ ] **Assets directory:** Confirm `dist/assets/` exists (not `dist/_astro/`) after build if `build.assets` was set
- [ ] **HTTPS enforced:** After first deploy, check Settings → Pages → "Enforce HTTPS" is checked (not greyed out)
- [ ] **Dark mode on first load:** Open the site in a fresh private window with `prefers-color-scheme: dark` (Chrome DevTools → Rendering → Emulate CSS media) and confirm no white flash
- [ ] **Dark mode after navigation:** Navigate between two pages and confirm the theme does not flicker during View Transition
- [ ] **Sitemap correct domain:** Check `https://kinder.patrykgolabek.dev/sitemap-index.xml` — every URL starts with the custom domain, not `localhost` or `github.io`
- [ ] **OG tags correct:** Paste the URL into Twitter Card Validator or opengraph.xyz — confirm image, title, and description appear correctly
- [ ] **Code blocks copyable:** Every code block on landing page and docs has a copy button
- [ ] **404 page styled:** Visit `https://kinder.patrykgolabek.dev/nonexistent` — confirm the 404 page uses the site's dark theme and has a navigation link back home (GitHub Pages serves `404.html` from the deploy root)
- [ ] **Workflow path filter correct:** Push a Go source change and confirm the site deploy workflow does NOT trigger
- [ ] **Go CI unaffected:** Push a site-only change and confirm Go CI workflows do NOT trigger

---

## Recovery Strategies

When pitfalls occur despite prevention.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Custom domain reset | LOW | Re-enter domain in Settings → Pages; add `public/CNAME` before next push; wait for DNS + cert reprovisioning |
| Jekyll stripping `_astro/` | LOW | Add `public/.nojekyll` and redeploy; or switch to GitHub Actions deployment source |
| HTTPS cert not issued | MEDIUM | Check DNS records with `dig kinder.patrykgolabek.dev CNAME`; check for conflicting A records; check for CAA records blocking letsencrypt.org; remove and re-add the custom domain to force cert re-request |
| Dark mode FOUC in production | LOW | Add `is:inline` blocking script to `<head>` in base layout; redeploy |
| `base` config set incorrectly — all links broken | MEDIUM | Remove `base` from config; grep codebase for hardcoded base path prefixes; update any `href="/kinder/..."` to `href="/..."`; rebuild and redeploy |
| Go CI workflows now triggering on site changes | LOW | Add `kinder-site/**` to `paths-ignore` in all four existing Go CI workflow YAML files |
| Concurrent deploys queued and failing | LOW | Add concurrency group to workflow; re-run the failed workflow manually via `workflow_dispatch` |

---

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Jekyll strips `_astro/` (C1) | Phase 1: scaffolding + initial deploy | `dist/.nojekyll` exists; `dist/assets/` exists (not `dist/_astro/`) |
| Custom domain resets on deploy (C2) | Phase 1: scaffolding + initial deploy | `dist/CNAME` exists with correct domain after every build |
| Wrong `base` config breaks links (C3) | Phase 1: initial Astro configuration | `import.meta.env.BASE_URL` is `/`; all nav links go to correct paths |
| `site` mismatch in sitemap/canonical (C4) | Phase 1: initial Astro configuration | `dist/sitemap-index.xml` contains only `https://kinder.patrykgolabek.dev` URLs |
| Dark mode FOUC (C5) | Phase 2: dark theme implementation | No white flash in private window with dark system preference; no flash during page navigation |
| Workflow missing permissions (C6) | Phase 1: CI/CD pipeline setup | Deploy workflow succeeds end-to-end; pages deployment visible in Settings → Pages |
| No path filtering — Go changes trigger site rebuild (C7) | Phase 1: CI/CD pipeline setup | Push Go change → site workflow does not appear in Actions |
| No concurrency group — overlapping deploys | Phase 1: CI/CD pipeline setup | Rapid pushes do not queue multiple conflicting deployments |
| DNS + HTTPS timing issues | Phase 1: custom domain setup | HTTPS enforced; site loads at `https://kinder.patrykgolabek.dev` |
| Missing OG image and social meta | Phase 3: landing page | Twitter Card Validator shows correct preview for the domain |
| Prose not dark-mode-styled (Tailwind Typography) | Phase 2: dark theme + Phase 4: docs | `prose dark:prose-invert` applied to all Markdown-rendered content |

---

## Sources

- [Astro GitHub Pages deploy guide](https://docs.astro.build/en/guides/deploy/github/) — official source for `site`, `base`, CNAME, and workflow permissions
- [withastro/action GitHub README](https://github.com/withastro/action) — `path` input for subdirectory projects; `cache` option; lockfile detection
- [Astro issue #14247: `_astro` folder ignored by GitHub Pages](https://github.com/withastro/astro/issues/14247) — confirmed Jekyll underscore-stripping behavior
- [Starlight issue #3339: Add .nojekyll File](https://github.com/withastro/starlight/issues/3339) — community confirmation + fix
- [GitHub community: Custom Domain Field Clears Every Deploy](https://github.com/orgs/community/discussions/48422) — CNAME reset root cause
- [GitHub community: Custom domain deleted after workflow deploy](https://github.com/orgs/community/discussions/159544) — confirms CNAME must be in build artifact
- [GitHub Pages DNS troubleshooting docs](https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/troubleshooting-custom-domains-and-github-pages) — CAA records, HTTPS timing, DNS propagation
- [Astro dark mode flicker — simonporter.co.uk](https://www.simonporter.co.uk/posts/what-the-fouc-astro-transitions-and-tailwind/) — `is:inline` blocking script pattern; `astro:after-swap` handling
- [Astro dark mode flicker — danielnewton.dev](https://www.danielnewton.dev/blog/dark-mode-astro-tailwind-fouc/) — Tailwind + localStorage pattern
- [Astro issue #8711: Flicker with ViewTransition and dark mode](https://github.com/withastro/astro/issues/8711) — confirmed FOUC during View Transitions
- [GitHub community: concurrency group for Pages](https://github.com/orgs/community/discussions/67961) — overlapping deploy behavior
- [GitHub Actions concurrency docs](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions) — `cancel-in-progress: false` for Pages deploys
- [Tailwind CSS Typography dark mode](https://tailwindcss.com/blog/tailwindcss-typography-v0-5) — `prose-invert` class for dark mode prose styling
- [Medium: GitHub Pages for Astro — Branch vs GitHub Actions](https://medium.com/vlead-tech/github-pages-for-astro-project-deploying-from-branch-vs-using-github-actions-af1f909322ee) — exhaustive comparison of both deploy methods and their trade-offs

---
*Pitfalls research for: Kinder website — Astro + GitHub Pages + custom domain + dark theme*
*Researched: 2026-03-01*
