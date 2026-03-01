# Phase 9: Scaffold and Deploy Pipeline - Context

**Gathered:** 2026-03-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Scaffold an Astro/Starlight project in `kinder-site/`, set up GitHub Actions deploy pipeline to GitHub Pages, and get the site live at `kinder.patrykgolabek.dev` over HTTPS. No real content — just validated infrastructure. Content, theming, and landing page are separate phases.

</domain>

<decisions>
## Implementation Decisions

### Deploy trigger strategy
- Deploy to production on push to main only — no preview deploys
- Build check runs on PRs that touch `kinder-site/` to catch errors before merge
- Path filter: `kinder-site/**` and `.github/workflows/site*` — Go code changes do not trigger site workflow
- DNS for `kinder.patrykgolabek.dev` is not yet configured — phase plan should include DNS setup steps

### Starlight initial config
- Site title: **kinder** (lowercase, matches CLI tool name)
- Sidebar: Starlight default — content phases will build the real structure later
- Nav links: GitHub repo link only in the header — minimal
- Landing page: Keep Starlight default splash page — Phase 12 replaces it with the real landing page

### GitHub repo identity
- Correct GitHub repository: **patrykattc/kinder**
- Custom domain: **kinder.patrykgolabek.dev** (confirmed)
- Fix incorrect GitHub username references (patrykgolabek/kinder → patrykattc/kinder) as part of this phase

### Version pinning and tooling
- Dependency versions: Claude's discretion (pick what's standard for Astro/Starlight)
- Package manager: Claude's discretion (pick based on simplicity and CI compatibility)
- Node.js version: Claude's discretion (decide based on what works best for CI + local dev)

### Claude's Discretion
- Astro/Starlight version pinning strategy (exact vs caret ranges)
- Package manager choice (npm vs pnpm)
- Node.js version enforcement approach
- GitHub Actions workflow implementation details
- CNAME file placement and build integration

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. The goal is validated infrastructure, not content or design.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 09-scaffold-and-deploy-pipeline*
*Context gathered: 2026-03-01*
