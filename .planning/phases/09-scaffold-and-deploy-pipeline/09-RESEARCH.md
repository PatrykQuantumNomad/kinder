# Phase 9: Scaffold and Deploy Pipeline - Research

**Researched:** 2026-03-01
**Domain:** Astro 5 / Starlight, GitHub Pages, GitHub Actions CI/CD
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Deploy trigger strategy
- Deploy to production on push to main only — no preview deploys
- Build check runs on PRs that touch `kinder-site/` to catch errors before merge
- Path filter: `kinder-site/**` and `.github/workflows/site*` — Go code changes do not trigger site workflow
- DNS for `kinder.patrykgolabek.dev` is not yet configured — phase plan should include DNS setup steps

#### Starlight initial config
- Site title: **kinder** (lowercase, matches CLI tool name)
- Sidebar: Starlight default — content phases will build the real structure later
- Nav links: GitHub repo link only in the header — minimal
- Landing page: Keep Starlight default splash page — Phase 12 replaces it with the real landing page

#### GitHub repo identity
- Correct GitHub repository: **patrykattc/kinder**
- Custom domain: **kinder.patrykgolabek.dev** (confirmed)
- Fix incorrect GitHub username references (patrykgolabek/kinder → patrykattc/kinder) as part of this phase

#### Version pinning and tooling
- Dependency versions: Claude's discretion (pick what's standard for Astro/Starlight)
- Package manager: Claude's discretion (pick based on simplicity and CI compatibility)
- Node.js version: Claude's discretion (decide based on what works best for CI + local dev)

### Claude's Discretion
- Astro/Starlight version pinning strategy (exact vs caret ranges)
- Package manager choice (npm vs pnpm)
- Node.js version enforcement approach
- GitHub Actions workflow implementation details
- CNAME file placement and build integration

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-01 | Astro + Starlight project scaffolded in `kinder-site/` directory with dark theme CSS variable overrides | `npm create astro@latest` with `--template starlight`, Starlight `customCss` option |
| INFRA-02 | `public/CNAME` file with `kinder.patrykgolabek.dev` for custom domain | GitHub Pages custom domain requires CNAME file in `public/`; Astro copies `public/` to `dist/` on build |
| INFRA-03 | `public/.nojekyll` file to prevent Jekyll from stripping `_astro/` assets | `public/.nojekyll` gets copied to `dist/.nojekyll`; needed when using branch-based Pages; withastro/action handles this automatically via Actions deployment, but explicit file satisfies the success criterion verification |
| INFRA-04 | `astro.config.mjs` with `site: 'https://kinder.patrykgolabek.dev'` and no `base` setting | Official Astro docs: custom domain uses `site` with full URL, no `base` needed |
| INFRA-05 | GitHub Actions workflow (`.github/workflows/deploy-site.yml`) that builds and deploys on push to main, path-filtered to `kinder-site/**` | withastro/action@v5 with `path: kinder-site`, GitHub Actions `paths:` filter on push |
</phase_requirements>

---

## Summary

This phase scaffolds an Astro/Starlight site in `kinder-site/` within the existing kinder Go monorepo, connects it to GitHub Pages with a custom domain, and wires up a GitHub Actions deploy pipeline. The goal is validated production infrastructure — not content. A successful outcome is `https://kinder.patrykgolabek.dev` loading the Starlight default splash page over HTTPS.

The technology stack is stable and well-documented. Astro 5 (currently at v5.18, February 2026) ships with Starlight 0.37.x as the documentation theme. The official deploy path — `withastro/action@v5` + `actions/deploy-pages@v4` — handles the full build-and-publish lifecycle with automatic package manager detection from lockfiles. The biggest operational concern is the DNS propagation delay: CNAME record changes can take up to 24 hours to propagate, and GitHub's HTTPS enforcement for custom domains requires an additional wait period after DNS is verified.

Two explicit files are required in `public/` to satisfy the success criteria: `CNAME` (containing the custom domain) and `.nojekyll` (to suppress Jekyll processing). Although `withastro/action` deployment via the GitHub Actions artifact system does not run Jekyll at all, the phase success criterion explicitly checks that `dist/.nojekyll` exists in the build output — which is achieved by placing `.nojekyll` in `public/` (Astro copies `public/` contents to `dist/` at build time).

**Primary recommendation:** Use npm (not pnpm) for simplicity — `npm create astro@latest` scaffolds with a `package-lock.json` that `withastro/action` auto-detects. Pin Node.js to v22 LTS. Use `withastro/action@v5` with `path: kinder-site`. No `base` config needed because a custom domain is in use.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| astro | ^5.x (latest: 5.18) | Static site framework | Industry standard for content-heavy sites; powers Starlight |
| @astrojs/starlight | ^0.37.x (latest: 0.37.6) | Documentation theme | Official Astro docs theme; batteries-included search, sidebar, nav |

### Supporting (GitHub Actions)

| Action | Version | Purpose | When to Use |
|--------|---------|---------|-------------|
| actions/checkout | @v5 | Checkout repository | Every workflow |
| withastro/action | @v5 (v5.2.0) | Build Astro and upload Pages artifact | Astro-to-Pages deploy |
| actions/deploy-pages | @v4 | Publish artifact to GitHub Pages | After withastro/action builds |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| npm | pnpm | pnpm requires `.npmrc` with `shamefully-hoist=true` for Astro; npm is simpler, withastro/action auto-detects both |
| withastro/action | Manual build + upload | More control but more complexity; withastro/action handles lockfile detection, caching, and artifact formatting |
| Starlight | Plain Astro | Starlight gives free search (Pagefind), nav, sidebar, TOC; no justification to skip it |

**Installation (scaffold command — run once manually in repo root):**

```bash
cd /path/to/kinder
npm create astro@latest -- --template starlight kinder-site
```

The CLI wizard prompts for TypeScript preference. Accept defaults. After scaffolding, install is already done.

---

## Architecture Patterns

### Recommended Project Structure

```
kinder/                         # Repo root (Go monorepo)
├── kinder-site/                # Astro/Starlight site (this phase)
│   ├── astro.config.mjs        # Site config: site URL, Starlight plugin
│   ├── package.json            # Node dependencies
│   ├── package-lock.json       # Lockfile (committed — withastro/action requires it)
│   ├── tsconfig.json           # TypeScript config (scaffolded)
│   ├── public/
│   │   ├── CNAME               # Custom domain: kinder.patrykgolabek.dev
│   │   └── .nojekyll           # Suppresses Jekyll (verified in dist/ after build)
│   └── src/
│       ├── content/
│       │   ├── config.ts       # Content collection schema
│       │   └── docs/
│       │       └── index.mdx   # Default splash page (Starlight default)
│       └── env.d.ts
├── .github/
│   └── workflows/
│       └── deploy-site.yml     # New: build + deploy workflow (this phase)
│       # (existing: docker.yaml, nerdctl.yaml, podman.yml, vm.yaml)
└── ... (Go source)
```

### Pattern 1: Custom Domain — No `base`, Full `site` URL

**What:** When using a custom domain (not `username.github.io/repo`), set `site` to the full HTTPS URL and omit `base`.

**When to use:** Any custom domain deployment. Using `base` with a custom domain causes double-path issues.

**Example:**

```javascript
// Source: https://docs.astro.build/en/guides/deploy/github/
// kinder-site/astro.config.mjs
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://kinder.patrykgolabek.dev',
  // NO base: setting — only needed for username.github.io/repo-name paths
  integrations: [
    starlight({
      title: 'kinder',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/patrykattc/kinder' },
      ],
    }),
  ],
});
```

### Pattern 2: Two-Job Deploy Workflow with Path Filter

**What:** Separate `build` and `deploy` jobs, path-filtered to only fire on site changes.

**When to use:** Monorepo with Go code + site code in same repo. This prevents Go CI from triggering site deploys and site deploys from triggering Go CI.

**Example:**

```yaml
# Source: https://docs.astro.build/en/guides/deploy/github/ + GitHub Actions docs
# .github/workflows/deploy-site.yml
name: Deploy Site

on:
  push:
    branches: [main]
    paths:
      - 'kinder-site/**'
      - '.github/workflows/deploy-site.yml'
  pull_request:
    branches: [main]
    paths:
      - 'kinder-site/**'
      - '.github/workflows/deploy-site.yml'

permissions:
  contents: read
  pages: write
  id-token: write

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v5
      - name: Build site
        uses: withastro/action@v5
        with:
          path: kinder-site
          node-version: '22'

  deploy:
    needs: build
    runs-on: ubuntu-latest
    # Only deploy on push to main, not on PRs
    if: github.event_name == 'push'
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
```

**Key detail:** The `deploy` job has `if: github.event_name == 'push'` so PRs trigger the build/check but not the deploy. The `build` job uses `path: kinder-site` to tell `withastro/action` where the Astro project root is within the monorepo.

### Pattern 3: Starlight Minimal Initial Config

**What:** Keep Starlight default splash page and sidebar for this phase; real content comes in later phases.

**Example:**

```javascript
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
      // sidebar: omitted — use Starlight default autogeneration
      // customCss: omitted this phase — dark theme CSS comes in INFRA-01 / next phase
    }),
  ],
});
```

### Anti-Patterns to Avoid

- **Setting `base: '/kinder'`:** Only needed for `username.github.io/repo` paths. With `kinder.patrykgolabek.dev`, omit `base` entirely or all internal links break.
- **Not committing lockfile:** `withastro/action` detects the package manager via lockfile. If `package-lock.json` is in `.gitignore`, the action falls back to defaults and may pick the wrong manager.
- **Forgetting `path: kinder-site` in workflow:** Without this, `withastro/action` looks for `astro.config.mjs` at the repo root, finds nothing, and fails.
- **Setting GitHub Pages source to "branch" instead of "GitHub Actions":** The deploy will fail with permissions errors. Must set source to GitHub Actions in repo Settings → Pages.
- **Configuring custom domain in GitHub Pages settings without DNS configured first:** GitHub will try to verify the domain and may mark it as improperly configured. Add DNS CNAME record first, wait for propagation, then configure in GitHub settings.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Build + upload Astro to Pages | Custom `npm run build && gh api` scripts | `withastro/action@v5` | Handles lockfile detection, Node version, caching, artifact formatting, and upload in one step |
| Documentation site framework | Custom Astro pages | `@astrojs/starlight` | Built-in search (Pagefind), sidebar, TOC, nav, dark mode, accessible markup — weeks of work for free |
| Jekyll suppression | Custom build scripts | `public/.nojekyll` file | One empty file; Astro copies it to `dist/` automatically |

**Key insight:** The `withastro/action` encapsulates the entire "install, build, upload artifact" sequence. The only custom work needed is the `paths:` filter and the conditional `if:` on the deploy job.

---

## Common Pitfalls

### Pitfall 1: `_astro/` Assets Stripped by Jekyll

**What goes wrong:** GitHub Pages runs Jekyll by default. Jekyll ignores directories starting with `_`. Astro's compiled JS/CSS goes into `dist/_astro/`. Result: site loads with no styles.

**Why it happens:** Jekyll was designed for Ruby-based sites where `_` prefix means "private" configuration. GitHub Pages inherited this behavior.

**How to avoid:** Place an empty `public/.nojekyll` file in the project. Astro copies all `public/` files to `dist/` during build, so `dist/.nojekyll` appears in the output and tells GitHub not to run Jekyll.

**Note:** When using the GitHub Actions artifact-based deploy (via `withastro/action` + `actions/deploy-pages`), Jekyll is NOT run at all — the artifact is deployed directly. However, having `public/.nojekyll` is still correct practice and satisfies the success criterion that checks `dist/.nojekyll` exists.

**Warning signs:** Site loads but CSS is missing; browser devtools show 404 for `/_astro/*.css` files.

### Pitfall 2: `base` Config Breaks Custom Domain

**What goes wrong:** Developer adds `base: '/kinder'` to `astro.config.mjs` thinking it's required. All asset paths become `kinder.patrykgolabek.dev/kinder/_astro/...`, causing 404s.

**Why it happens:** `base` is only needed when deploying to a subpath (e.g., `username.github.io/repo-name`). Custom domains serve from root.

**How to avoid:** Only set `site`. Never set `base` when using a custom domain.

**Warning signs:** `console.log(import.meta.env.BASE_URL)` returns `/kinder` instead of `/`.

### Pitfall 3: DNS Not Propagated Before Enabling HTTPS

**What goes wrong:** User configures custom domain in GitHub Pages settings, immediately tries to enable "Enforce HTTPS," and the option is greyed out or the site shows a certificate error for 24+ hours.

**Why it happens:** GitHub Pages must verify DNS before provisioning a Let's Encrypt certificate. Verification and cert provisioning take up to 24 hours after the CNAME record propagates.

**How to avoid:** Configure DNS CNAME record first (before touching GitHub Pages settings). Wait for propagation (`dig kinder.patrykgolabek.dev CNAME` should return `patrykattc.github.io`). Then set custom domain in GitHub Pages settings and enable HTTPS enforcement.

**Warning signs:** GitHub Pages shows "Domain's DNS record could not be retrieved" or HTTPS enforcement checkbox is disabled.

### Pitfall 4: GitHub Pages Source Not Set to "GitHub Actions"

**What goes wrong:** The `deploy-site.yml` workflow runs successfully but the site doesn't update. Or the workflow fails with a permissions error.

**Why it happens:** GitHub Pages defaults to branch-based deployment. The GitHub Actions deploy workflow requires changing the Pages source to "GitHub Actions" in repository Settings → Pages.

**How to avoid:** As part of phase setup, navigate to Settings → Pages → Build and deployment → Source → "GitHub Actions." This is a one-time manual step that cannot be automated by the workflow itself.

**Warning signs:** `actions/deploy-pages` fails with "Error: Failed to create deployment (status: 403)."

### Pitfall 5: Missing `path: kinder-site` in `withastro/action`

**What goes wrong:** `withastro/action` searches for `astro.config.mjs` starting from the repo root. In this monorepo, there is no such file at root (it's at `kinder-site/astro.config.mjs`). The action fails with "No Astro config found."

**Why it happens:** `withastro/action@v5` accepts an optional `path` input for monorepo scenarios. It defaults to the repo root if not specified.

**How to avoid:** Always specify `with: path: kinder-site` in the workflow step.

**Warning signs:** Workflow log shows "Cannot find Astro config" or "No package.json found."

### Pitfall 6: Existing Workflows Use `paths-ignore: ['site/**']`

**What goes wrong:** The existing Go workflows (`docker.yaml`, etc.) have `paths-ignore: - 'site/**'`. The new site lives in `kinder-site/**`, not `site/**`. If nothing is updated, Go CI could trigger on site changes (or the wrong directory filter silently continues working by coincidence).

**Why it happens:** The old `kind` repo had a `site/` directory. The kinder fork keeps this legacy filter but the new site directory is `kinder-site/`.

**How to avoid:** Either leave Go workflows as-is (they use `paths-ignore: site/**` which is unrelated to `kinder-site/`) and accept that Go CI may run on site-only changes, OR update Go workflows to also ignore `kinder-site/**`. The user decision is to have path filtering on the site workflow; Go CI running on site-only PRs is a minor inefficiency, not a blocker.

**Warning signs:** Go CI runs on PRs that only touch `kinder-site/`; if this is undesirable, update `paths-ignore` in Go workflow files.

---

## Code Examples

Verified patterns from official sources:

### `public/CNAME` File

```
kinder.patrykgolabek.dev
```

Single line, no trailing newline needed. Source: https://docs.astro.build/en/guides/deploy/github/

### `public/.nojekyll` File

Empty file. Create with:

```bash
touch kinder-site/public/.nojekyll
```

### DNS CNAME Record (for registrar/DNS provider)

```
Type:  CNAME
Name:  kinder  (subdomain prefix, or kinder.patrykgolabek.dev depending on provider)
Value: patrykattc.github.io
TTL:   3600 (or provider default)
```

Source: https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/managing-a-custom-domain-for-your-github-pages-site

### Verify DNS Propagation

```bash
dig kinder.patrykgolabek.dev CNAME +short
# Expected: patrykattc.github.io.
```

### Full `astro.config.mjs` for This Phase

```javascript
// Source: https://docs.astro.build/en/guides/deploy/github/
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
    }),
  ],
});
```

### Full GitHub Actions Workflow

```yaml
# Source: https://docs.astro.build/en/guides/deploy/github/ (modified with path filter + monorepo path)
# .github/workflows/deploy-site.yml
name: Deploy Site

on:
  push:
    branches: [main]
    paths:
      - 'kinder-site/**'
      - '.github/workflows/deploy-site.yml'
  pull_request:
    branches: [main]
    paths:
      - 'kinder-site/**'
      - '.github/workflows/deploy-site.yml'

permissions:
  contents: read
  pages: write
  id-token: write

jobs:
  build:
    name: Build
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v5
      - name: Build and upload site artifact
        uses: withastro/action@v5
        with:
          path: kinder-site
          node-version: '22'

  deploy:
    name: Deploy
    needs: build
    runs-on: ubuntu-latest
    if: github.event_name == 'push'
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
```

### Scaffold Command (one-time, run in repo root)

```bash
# From repo root — creates kinder-site/ directory
npm create astro@latest -- --template starlight kinder-site

# Answer CLI prompts:
# - Install dependencies? Yes
# - Initialize git repo? No (already in a git repo)
# - TypeScript: Strict (recommended)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Push to `gh-pages` branch (JamesIves/github-pages-deploy-action) | Upload artifact via GitHub Actions API (`withastro/action` + `actions/deploy-pages`) | GitHub Actions deployments became default ~2023 | Faster deploys, no `gh-pages` branch clutter, proper environment protection |
| Manual `npm install && npm run build` steps in workflow | `withastro/action@v5` wraps everything | Astro team released action ~2022, v5 Feb 2026 | Zero-config lockfile detection, built-in caching |
| Committing built files to repo | Ephemeral artifact-based deploy | Same shift | No `dist/` in git history |

**Deprecated/outdated:**
- `JamesIves/github-pages-deploy-action`: Still works but pushes to `gh-pages` branch — Astro's official docs now use the artifact API approach exclusively
- `actions/upload-artifact@v3` + `actions/download-artifact@v3`: Deprecated January 2025 — `withastro/action@v5` already uses v4
- Setting GitHub Pages source to a branch: Still functional but no longer the recommended approach for custom build processes

---

## Open Questions

1. **Does the scaffold command need `--` before `--template` with npm?**
   - What we know: `npm create astro@latest -- --template starlight kinder-site` is the documented syntax (the `--` separates npm options from script options)
   - What's unclear: Behavior may differ slightly between npm versions; `pnpm create astro@latest --template starlight kinder-site` (no `--`) works with pnpm
   - Recommendation: Use npm and include the `--` separator as documented

2. **Will `withastro/action` honor the `path: kinder-site` input when there is no `.nvmrc` or `.node-version` file?**
   - What we know: `withastro/action@v5` defaults to Node.js 22 and accepts an explicit `node-version` input
   - What's unclear: Whether the action respects an `.nvmrc` in `kinder-site/` if one is created
   - Recommendation: Explicitly set `node-version: '22'` in the workflow to avoid any ambiguity

3. **Timing of DNS setup vs GitHub Pages configuration**
   - What we know: DNS propagation can take up to 24 hours; HTTPS cert provisioning requires DNS to be resolved first; GitHub recommends adding DNS before configuring custom domain in Settings
   - What's unclear: Whether the phase plan should treat DNS setup and HTTPS enforcement as blocking tasks or document them as async steps
   - Recommendation: Plan includes DNS step as an early task; treat HTTPS enforcement as a separate verification step that may require waiting

---

## Sources

### Primary (HIGH confidence)
- https://docs.astro.build/en/guides/deploy/github/ — Full GitHub Pages deploy guide, workflow YAML, custom domain setup
- https://docs.astro.build/en/install-and-setup/ — Node.js version requirements (v18.20.8, v20.3.0, v22.0.0+)
- https://starlight.astro.build/getting-started/ — Create command, generated structure
- https://starlight.astro.build/reference/configuration/ — All Starlight config options (title, social, sidebar, customCss)
- https://github.com/withastro/action — withastro/action@v5 (v5.2.0, Feb 2026) inputs: path, node-version, package-manager
- https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/managing-a-custom-domain-for-your-github-pages-site — DNS CNAME for subdomains, IP addresses for apex, HTTPS enforcement timing

### Secondary (MEDIUM confidence)
- https://github.com/withastro/astro/issues/14247 — `_astro/` folder stripped by Jekyll; withastro/action handles automatically via artifact deploy; confirmed `public/.nojekyll` approach works
- https://github.com/withastro/starlight/issues/3339 — `.nojekyll` in `dist/` approach; Astro copies `public/` to `dist/` so `public/.nojekyll` is equivalent
- Astro blog February 2026 (https://astro.build/blog/whats-new-february-2026/) — Astro at v5.18 as of Feb 2026; v6 beta active but not stable

### Tertiary (LOW confidence)
- Community reports that pnpm requires `shamefully-hoist=true` in `.npmrc` for Astro — not independently verified against current Astro/pnpm docs but seen in multiple sources; reason npm is recommended here
- GitHub Actions path filter behavior for workflow file self-modification — `.github/workflows/deploy-site.yml` in `paths:` filter is standard practice but edge cases around re-triggering not verified

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Astro 5.18 + Starlight 0.37.6 verified via official blog and npm; withastro/action v5.2.0 verified via GitHub
- Architecture: HIGH — Workflow YAML from official Astro deploy guide; custom domain from GitHub Pages docs
- Pitfalls: HIGH — `_astro`/Jekyll issue verified across multiple official issues; DNS timing from GitHub Pages official docs

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (Astro releases frequently; Starlight is still pre-1.0)
