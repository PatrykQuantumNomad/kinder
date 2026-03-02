# Roadmap: Kinder

## Milestones

- ✅ **v1.0 Batteries Included** - Phases 1-8 (shipped 2026-03-01)
- 🚧 **v1.1 Kinder Website** - Phases 9-14 (in progress)

## Phases

<details>
<summary>✅ v1.0 Batteries Included (Phases 1-8) - SHIPPED 2026-03-01</summary>

Forked kind into kinder with 5 default addons (MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, Headlamp dashboard). Binary renamed to `kinder` with backward-compatible v1alpha4 config schema. 8 phases, 12 plans, 36 commits.

</details>

---

### 🚧 v1.1 Kinder Website (In Progress)

**Milestone Goal:** Create a standalone website for kinder with a marketing landing page and full documentation, hosted on GitHub Pages at kinder.patrykgolabek.dev.

**Phase Numbering:**
- Integer phases (9, 10, 11...): Planned milestone work
- Decimal phases (9.1, 9.2): Urgent insertions (marked with INSERTED)

- [x] **Phase 9: Scaffold and Deploy Pipeline** - Astro/Starlight project scaffolded and live at kinder.patrykgolabek.dev
- [x] **Phase 10: Dark Theme** - Terminal-aesthetic dark theme applied site-wide with FOUC prevention
- [x] **Phase 11: Documentation Content** - All 8 documentation pages written and navigable
- [ ] **Phase 12: Landing Page** - Full marketing landing page with hero, addon grid, and comparison section
- [ ] **Phase 13: Assets and Identity** - Favicon, og:image, and custom 404 page in place
- [ ] **Phase 14: Polish and Validation** - Mobile-responsive and Lighthouse 90+ confirmed

## Phase Details

### Phase 9: Scaffold and Deploy Pipeline
**Goal**: A live placeholder site at kinder.patrykgolabek.dev validates DNS, HTTPS, GitHub Actions deploy, and all production infrastructure before any content is written
**Depends on**: Nothing (first phase of v1.1)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-05
**Success Criteria** (what must be TRUE):
  1. Visiting `https://kinder.patrykgolabek.dev` returns the Starlight default site over HTTPS with no certificate errors
  2. Pushing a change to `kinder-site/**` triggers the GitHub Actions deploy workflow and the live site updates
  3. Pushing a Go code change does NOT trigger the site deploy workflow
  4. The built `dist/` directory contains `CNAME`, `.nojekyll`, and `assets/` (not `_astro/`) confirming pitfall mitigations are in place
**Plans**: 2 plans
Plans:
- [x] 09-01-PLAN.md -- Scaffold Astro/Starlight project and create GitHub Actions deploy workflow
- [x] 09-02-PLAN.md -- Configure DNS, GitHub Pages, and verify live site

### Phase 10: Dark Theme
**Goal**: The entire site renders with a dark terminal aesthetic and never flashes white on load or page navigation
**Depends on**: Phase 9
**Requirements**: INFRA-06
**Success Criteria** (what must be TRUE):
  1. All Starlight pages display a near-black background with cyan accent colors matching the dark terminal aesthetic
  2. Loading or navigating to any page does not produce a white flash before the dark theme is applied
  3. The dark/light theme toggle persists the chosen mode across page navigations and browser sessions
**Plans**: 1 plan
Plans:
- [x] 10-01-PLAN.md -- Apply cyan terminal theme via CSS custom properties and verify visual appearance

### Phase 11: Documentation Content
**Goal**: Every documentation page a user needs to install, configure, and use kinder is written and reachable via sidebar navigation and search
**Depends on**: Phase 10
**Requirements**: DOCS-01, DOCS-02, DOCS-03, DOCS-04, DOCS-05, DOCS-06, DOCS-07, DOCS-08
**Success Criteria** (what must be TRUE):
  1. A user landing on the docs can install kinder by following the installation guide without leaving the site
  2. A user can follow the quick start page from zero to a working cluster with verified addons in 5 steps
  3. The configuration reference shows all `kind.x-k8s.io/v1alpha4` addon fields, their defaults, and example YAML snippets
  4. Each of the 5 addons (MetalLB, Envoy Gateway, Metrics Server, CoreDNS, Headlamp) has its own dedicated page explaining what it installs, what the user gets, and how to disable it
  5. Pagefind search returns relevant results when a user searches for addon names or config fields
**Plans**: 2 plans
Plans:
- [x] 11-01-PLAN.md -- Write installation, quick start, and configuration reference pages with sidebar config
- [x] 11-02-PLAN.md -- Write all five addon documentation pages and add Addons sidebar group

### Phase 12: Landing Page
**Goal**: A visitor arriving at kinder.patrykgolabek.dev understands what kinder offers over kind within 10 seconds and can immediately copy the install command or navigate to documentation
**Depends on**: Phase 11
**Requirements**: LAND-01, LAND-02, LAND-03, LAND-04
**Success Criteria** (what must be TRUE):
  1. The hero section shows the install command and a working copy-to-clipboard button without requiring any documentation page to be open
  2. The addon feature grid displays all 5 addons as cards with name and description, each linking to the corresponding doc page
  3. The before/after comparison section communicates the difference between a bare kind cluster and a kinder cluster without additional explanation
  4. A user can reach the GitHub repository and the documentation from the landing page without scrolling
**Plans**: 1 plan
Plans:
- [ ] 12-01-PLAN.md -- Create landing page with hero, install command, comparison section, and addon grid

### Phase 13: Assets and Identity
**Goal**: The site has a complete visual identity with favicon, social sharing preview, and a useful 404 page
**Depends on**: Phase 12
**Requirements**: PLSH-01, PLSH-02, PLSH-03
**Success Criteria** (what must be TRUE):
  1. The browser tab displays the kinder favicon SVG instead of the Starlight default
  2. Pasting the site URL in a social media or Slack message shows the kinder og:image with the correct title and description
  3. Visiting a non-existent URL on the site returns a styled 404 page with a working link back to the home page instead of GitHub's default 404
**Plans**: TBD

### Phase 14: Polish and Validation
**Goal**: The site is confirmed production-ready — fully functional on mobile viewports and meeting Lighthouse 90+ across all metrics
**Depends on**: Phase 13
**Requirements**: PLSH-04, PLSH-05
**Success Criteria** (what must be TRUE):
  1. The landing page and all documentation pages are fully readable and usable on a 375px-wide mobile viewport with no horizontal scroll or overlapping elements
  2. Lighthouse reports 90 or above on Performance, Accessibility, Best Practices, and SEO when run against the live production URL
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 9 → 10 → 11 → 12 → 13 → 14

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8. v1.0 Batteries Included | v1.0 | 12/12 | Complete | 2026-03-01 |
| 9. Scaffold and Deploy Pipeline | v1.1 | 2/2 | Complete | 2026-03-01 |
| 10. Dark Theme | v1.1 | 1/1 | Complete | 2026-03-01 |
| 11. Documentation Content | v1.1 | 2/2 | Complete | 2026-03-02 |
| 12. Landing Page | v1.1 | 0/1 | Not started | - |
| 13. Assets and Identity | v1.1 | 0/TBD | Not started | - |
| 14. Polish and Validation | v1.1 | 0/TBD | Not started | - |
