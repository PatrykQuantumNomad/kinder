# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-01)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v1.1 Kinder Website — Phase 12: Landing Page

## Current Position

Phase: 11 of 14 (Documentation Content) — COMPLETE
Plan: 02/02 complete
Status: Phase 11 complete — verified and gap-fixed
Last activity: 2026-03-02 — Phase 11 verification passed (13/13). Fixed dashboard references (Kubernetes Dashboard → Headlamp) in quick-start and config.

Progress: [█████░░░░░] 42% (v1.1 — 5/12 plans complete)

## Performance Metrics

**Velocity (v1.0):**
- Total plans completed: 12
- Total phases: 8

**v1.1 (current):**
- Plans completed: 5
- Phases complete: 3/6 (Phase 9 complete, Phase 10 complete, Phase 11 complete)
- Phase 9 Plan 01: 3 min, 2 tasks, 9 files created
- Phase 9 Plan 02: ~5 min (user action), 2 checkpoint tasks
- Phase 10 Plan 01: 2 min, 2 tasks, 3 files
- Phase 11 Plan 01: 2 min, 2 tasks, 5 files (3 created, 1 modified, 2 deleted)
- Phase 11 Plan 02: 2 min, 2 tasks, 6 files (5 created, 1 modified)

## Accumulated Context

### Decisions

- v1.0: Fork kind (not wrap), addons as creation actions, on-by-default opt-out, go:embed for manifests
- v1.1: Astro + Starlight for website, kinder-site/ subdirectory, no Tailwind, custom domain kinder.patrykgolabek.dev
- Phase 9: npm (not pnpm) for CI compatibility; no base setting in astro.config.mjs (custom domain serves from root)
- Phase 9: GitHub repo confirmed patrykattc/kinder (resolves prior inconsistency in ARCHITECTURE.md)
- Phase 9: Deploy job gated to push only; PRs get build-check without deployment
- Phase 9: DNS CNAME kinder.patrykgolabek.dev → patrykattc.github.io; GitHub Pages source = GitHub Actions
- Phase 10: Dark-only mode — removed theme toggle, no light mode support. Starlight component override pattern for ThemeSelect.
- Phase 11 Plan 01: Build-from-source via `make install` is the only documented install method (binary distribution unconfirmed). apiVersion is `kind.x-k8s.io/v1alpha4` (confirmed from Go source). Sidebar uses slug-based entries for auto-sync with frontmatter titles.
- Phase 11 Plan 02: Starlight sidebar groups use `{ label, items: [{ slug }] }` pattern. Addon config field for Headlamp is `dashboard` (not `headlamp`). Starlight admonition syntax: `:::caution[Title]`, `:::note`, `:::tip`.

### Blockers/Concerns

- [Phase 11] RESOLVED: Binary distribution — documented `make install` (build-from-source) as the only confirmed method. Hero CTA will use the same.
- [Phase 9] RESOLVED: GitHub repo confirmed as patrykattc/kinder (per CONTEXT.md)

### Pending Todos

None.

## Session Continuity

Last session: 2026-03-02
Stopped at: Phase 11 Plan 02 complete — Five addon pages written, Addons sidebar group added, 10-page build verified with Pagefind index.
Resume file: None
