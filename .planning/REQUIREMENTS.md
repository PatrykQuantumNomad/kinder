# Requirements: Kinder Website

**Defined:** 2026-03-01
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## v1.1 Requirements

Requirements for the kinder website milestone. Each maps to roadmap phases.

### Infrastructure

- [ ] **INFRA-01**: Astro + Starlight project scaffolded in `kinder-site/` directory with dark theme CSS variable overrides
- [ ] **INFRA-02**: `public/CNAME` file with `kinder.patrykgolabek.dev` for custom domain
- [ ] **INFRA-03**: `public/.nojekyll` file to prevent Jekyll from stripping `_astro/` assets
- [ ] **INFRA-04**: `astro.config.mjs` with `site: 'https://kinder.patrykgolabek.dev'` and no `base` setting
- [ ] **INFRA-05**: GitHub Actions workflow (`.github/workflows/deploy-site.yml`) that builds and deploys on push to main, path-filtered to `kinder-site/**`
- [ ] **INFRA-06**: Dark mode FOUC prevention with synchronous inline script in base layout

### Documentation

- [ ] **DOCS-01**: Installation guide page covering `go install` and binary download
- [ ] **DOCS-02**: Quick start page walking through `kinder create cluster` and verifying addons
- [ ] **DOCS-03**: Configuration reference page documenting `v1alpha4` addons schema and all config options
- [ ] **DOCS-04**: MetalLB addon documentation page (what it does, config options, platform notes)
- [ ] **DOCS-05**: Envoy Gateway addon documentation page (Gateway API routing, GatewayClass, HTTPRoute)
- [ ] **DOCS-06**: Metrics Server addon documentation page (`kubectl top`, HPA support)
- [ ] **DOCS-07**: CoreDNS tuning addon documentation page (autopath, cache, what changes)
- [ ] **DOCS-08**: Headlamp dashboard addon documentation page (access, token, port-forward)

### Landing Page

- [ ] **LAND-01**: Hero section with kinder tagline, one-line description, and install command with copy button
- [ ] **LAND-02**: Addon feature grid with cards for each of the 5 addons (icon, name, one-line description)
- [ ] **LAND-03**: Before/after comparison section (kind vs kinder — what you get out of the box)
- [ ] **LAND-04**: GitHub link and call-to-action to documentation

### Polish

- [ ] **PLSH-01**: Favicon and site logo (SVG)
- [ ] **PLSH-02**: Open Graph image for social media sharing
- [ ] **PLSH-03**: Custom 404 page
- [ ] **PLSH-04**: Mobile-responsive validation
- [ ] **PLSH-05**: Lighthouse performance pass (90+ on all metrics)

## Future Requirements

### Enhancements

- **ENH-01**: Animated terminal demo showing `kinder create cluster` output
- **ENH-02**: Blog section for release announcements
- **ENH-03**: Contributing guide page
- **ENH-04**: Changelog page auto-generated from git history

## Out of Scope

| Feature | Reason |
|---------|--------|
| Versioned documentation | No breaking changes yet; overhead before kinder has multiple versions |
| Interactive playground | Impossible with Docker dependency; fake demos break trust |
| Email signup / newsletter | GDPR complexity, no audience yet |
| Blog section | Stale blog = dead project signal; defer until regular content cadence established |
| Reuse of kind's Hugo website | Fresh identity for kinder; different framework (Astro vs Hugo) |
| Tailwind CSS | Starlight's CSS custom properties sufficient; Tailwind v4 integration unstable |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | Phase 9 | Pending |
| INFRA-02 | Phase 9 | Pending |
| INFRA-03 | Phase 9 | Pending |
| INFRA-04 | Phase 9 | Pending |
| INFRA-05 | Phase 9 | Pending |
| INFRA-06 | Phase 10 | Pending |
| DOCS-01 | Phase 11 | Pending |
| DOCS-02 | Phase 11 | Pending |
| DOCS-03 | Phase 11 | Pending |
| DOCS-04 | Phase 11 | Pending |
| DOCS-05 | Phase 11 | Pending |
| DOCS-06 | Phase 11 | Pending |
| DOCS-07 | Phase 11 | Pending |
| DOCS-08 | Phase 11 | Pending |
| LAND-01 | Phase 12 | Pending |
| LAND-02 | Phase 12 | Pending |
| LAND-03 | Phase 12 | Pending |
| LAND-04 | Phase 12 | Pending |
| PLSH-01 | Phase 13 | Pending |
| PLSH-02 | Phase 13 | Pending |
| PLSH-03 | Phase 13 | Pending |
| PLSH-04 | Phase 14 | Pending |
| PLSH-05 | Phase 14 | Pending |

**Coverage:**
- v1.1 requirements: 23 total
- Mapped to phases: 23
- Unmapped: 0

---
*Requirements defined: 2026-03-01*
*Last updated: 2026-03-01 — traceability complete after roadmap creation*
