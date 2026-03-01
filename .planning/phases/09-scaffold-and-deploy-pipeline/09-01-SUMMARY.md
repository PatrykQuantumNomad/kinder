---
phase: 09-scaffold-and-deploy-pipeline
plan: "01"
subsystem: website
tags: [astro, starlight, github-actions, github-pages, ci-cd]
requires: []
provides:
  - kinder-site Astro/Starlight project scaffold
  - GitHub Actions deploy pipeline for GitHub Pages
affects:
  - .github/workflows/ (new workflow added)
  - kinder-site/ (new project directory)
tech-stack:
  added:
    - astro@^5.6.1
    - "@astrojs/starlight@^0.37.6"
    - sharp@^0.34.2
    - withastro/action@v5
    - actions/deploy-pages@v4
  patterns:
    - Monorepo path filtering in GitHub Actions (kinder-site/**)
    - CNAME + .nojekyll in public/ for GitHub Pages custom domain
    - Two-job build/deploy split for PR build-check without deploy
key-files:
  created:
    - kinder-site/astro.config.mjs
    - kinder-site/package.json
    - kinder-site/package-lock.json
    - kinder-site/tsconfig.json
    - kinder-site/src/content/docs/index.mdx
    - kinder-site/src/content.config.ts
    - kinder-site/public/CNAME
    - kinder-site/public/.nojekyll
    - .github/workflows/deploy-site.yml
  modified: []
key-decisions:
  - "No base setting in astro.config.mjs — custom domain serves from root, base would break all asset paths"
  - "GitHub repo confirmed as patrykattc/kinder (not patrykgolabek/kinder)"
  - "npm chosen over pnpm — withastro/action auto-detects package-lock.json, no extra config"
  - "Deploy job gated to push only via github.event_name == 'push' — PRs get build check without deploy"
  - "Node.js v22 LTS pinned in workflow via withastro/action node-version input"
  - "src/env.d.ts absent in Astro v5 — replaced by src/content.config.ts (new content layer API)"
metrics:
  duration: "3 minutes"
  tasks_completed: 2
  tasks_total: 2
  files_created: 9
  files_modified: 0
  completed: "2026-03-01T22:59:09Z"
---

# Phase 9 Plan 01: Scaffold and Deploy Pipeline Summary

**One-liner:** Astro v5 + Starlight project scaffold in kinder-site/ with GitHub Actions two-job build/deploy pipeline targeting kinder.patrykgolabek.dev via GitHub Pages.

## Accomplishments

- Initialized `kinder-site/` using `npm create astro@latest --template starlight` with Astro v5.6.1 and Starlight v0.37.6
- Configured `astro.config.mjs` with `site: 'https://kinder.patrykgolabek.dev'`, title "kinder", GitHub social link to patrykattc/kinder — no `base` setting
- Added `public/CNAME` with `kinder.patrykgolabek.dev` (copied to `dist/CNAME` on build for GitHub Pages custom domain)
- Added `public/.nojekyll` to prevent Jekyll processing from stripping `_astro/` asset directories
- Verified `npm run build` produces `dist/` with CNAME, .nojekyll, and index.html with no base path prefix
- Created `.github/workflows/deploy-site.yml` with two-job structure: build (runs on push + PR) and deploy (push only)
- Workflow path-filtered to `kinder-site/**` and the workflow file itself — Go code changes do not trigger it

## Task Commits

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Scaffold Astro/Starlight project | 5a0db6b5 | kinder-site/ (14 files) |
| 2 | Create GitHub Actions deploy workflow | c9fc5428 | .github/workflows/deploy-site.yml |

## Files Created

| File | Purpose |
|------|---------|
| `kinder-site/astro.config.mjs` | Astro + Starlight config with site URL and GitHub social link |
| `kinder-site/package.json` | Node.js project with astro and @astrojs/starlight dependencies |
| `kinder-site/package-lock.json` | npm lockfile (required by withastro/action for lockfile detection) |
| `kinder-site/tsconfig.json` | TypeScript strict mode config |
| `kinder-site/src/content/docs/index.mdx` | Default Starlight splash landing page |
| `kinder-site/src/content.config.ts` | Astro v5 content layer config (replaces env.d.ts) |
| `kinder-site/public/CNAME` | Custom domain for GitHub Pages (kinder.patrykgolabek.dev) |
| `kinder-site/public/.nojekyll` | Jekyll suppression for _astro/ assets |
| `.github/workflows/deploy-site.yml` | CI/CD pipeline for site build and GitHub Pages deploy |

## Decisions Made

1. **No `base` setting** — Custom domain `kinder.patrykgolabek.dev` serves from root (`/`). Adding `base` would create double-prefixed asset paths (e.g., `/kinder/_astro/...`) breaking the site entirely.

2. **GitHub username confirmed: `patrykattc/kinder`** — CONTEXT.md explicitly confirmed `patrykattc/kinder` as the correct repo. All references use this URL.

3. **npm over pnpm** — `withastro/action` auto-detects `package-lock.json` for npm and runs the right install command. No `.npmrc` or special config needed.

4. **Two-job workflow** — `build` job runs on push and PRs (catches errors early). `deploy` job has `if: github.event_name == 'push'` so PRs get validation without deploying.

5. **Node.js v22 LTS** — Pinned via `node-version: '22'` in `withastro/action` input. Meets Astro's minimum of v18.20.8 with margin.

6. **Astro v5 content layer** — `src/env.d.ts` is absent in Astro v5 (it was a v2/v3 pattern). The scaffold generates `src/content.config.ts` instead, which serves the same role under the new content layer API.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Observation] src/env.d.ts absent in Astro v5**
- **Found during:** Task 1
- **Issue:** Plan listed `kinder-site/src/env.d.ts` as a file to commit, but Astro v5 scaffold does not generate this file. It was replaced by `src/content.config.ts` in the new content layer API.
- **Fix:** Staged `src/content.config.ts` instead. No code change needed — the scaffold already produced the correct file for Astro v5.
- **Files modified:** None (observation only, staged the correct file)
- **Commit:** 5a0db6b5

None other — plan executed as written.

## Verification Results

| Check | Result |
|-------|--------|
| `npm run build` completes without errors | PASS |
| `dist/CNAME` exists | PASS |
| `dist/.nojekyll` exists | PASS |
| `dist/index.html` exists | PASS |
| `dist/CNAME` contains kinder.patrykgolabek.dev | PASS |
| `astro.config.mjs` has site URL | PASS |
| `astro.config.mjs` has no base setting | PASS |
| Workflow has `path: kinder-site` | PASS |
| Workflow has deploy guard `github.event_name == 'push'` | PASS |

## Self-Check: PASSED

| Check | Result |
|-------|--------|
| kinder-site/astro.config.mjs exists | FOUND |
| kinder-site/public/CNAME exists | FOUND |
| kinder-site/public/.nojekyll exists | FOUND |
| .github/workflows/deploy-site.yml exists | FOUND |
| 09-01-SUMMARY.md exists | FOUND |
| Commit 5a0db6b5 in git log | FOUND |
| Commit c9fc5428 in git log | FOUND |

## Next Phase Readiness

Plan 02 (DNS + GitHub Pages configuration) is unblocked:
- `kinder-site/` builds successfully and produces correct `dist/`
- `dist/CNAME` contains `kinder.patrykgolabek.dev` for Pages custom domain recognition
- GitHub Actions workflow is ready to trigger when pushed to main
- Remaining prerequisite: Enable GitHub Pages in repo settings + configure DNS CNAME record
