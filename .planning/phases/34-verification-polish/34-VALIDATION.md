# Phase 34: Verification & Polish - Validation

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Astro build + grep-based content audit (no automated test suite) |
| Quick run | `cd kinder-site && npm run build 2>&1 \| tee /tmp/kinder-build.log && grep -q '19 page' /tmp/kinder-build.log && echo PASS \|\| echo FAIL` |
| Full suite | `cd kinder-site && npm run build` |

## Requirements Test Map

| Req ID | Behavior | Automated Command |
|--------|----------|-------------------|
| GUIDE-01 | All internal links resolve without 404s | `cd kinder-site && npm run build` passes with zero errors (Astro validates all slug references at build time) |
| GUIDE-02 | Production build zero errors, 19 pages | `cd kinder-site && npm run build 2>&1 \| tee /tmp/kinder-build.log; grep -q '19 page' /tmp/kinder-build.log && echo PASS \|\| echo FAIL` |
| GUIDE-03 | All pages reachable from sidebar | Sidebar slug count in `astro.config.mjs` matches 19 built pages: `grep -c "slug:" kinder-site/astro.config.mjs` (expect 18, plus index.mdx = 19 total) |

## Per-Task Gates

| Task | Gate Command | Pass Condition |
|------|-------------|----------------|
| Task 1 (content fixes + audit) | `grep "ci —" kinder-site/src/content/docs/changelog.md \| grep -q "cert-manager" && grep "Go 1.24" kinder-site/src/content/docs/installation.md \| grep -q "or later" && echo PASS \|\| echo FAIL` | Both content bugs fixed, PASS printed |
| Task 2 (production build) | `cd kinder-site && npm run build 2>&1 \| tee /tmp/kinder-build.log; grep -q '19 page' /tmp/kinder-build.log && echo PASS \|\| echo FAIL` | Build succeeds, 19 pages confirmed, PASS printed |

## Phase Gate

All four success criteria from ROADMAP must be verified before closing the phase:

| Success Criterion | Verification Method |
|-------------------|---------------------|
| All internal links resolve without 404s | `npm run build` passes (Astro validates all slug references) + grep confirms no broken relative links in markdown |
| `npm run build` zero errors, zero broken link warnings | Direct: run `npm run build` in `kinder-site/` |
| All pages reachable from sidebar | Sidebar slug count in `astro.config.mjs` matches 19 built pages |
| Code blocks syntactically correct, commands match CLI | Grep Go source for flag definitions, compare to markdown command examples |

## Sampling Rates

- **Per task commit:** `cd kinder-site && npm run build 2>&1 | tee /tmp/kinder-build.log; grep -q '19 page' /tmp/kinder-build.log && echo PASS || echo FAIL`
- **Per wave merge:** `cd kinder-site && npm run build`
- **Phase gate:** Full build green + all four success criteria grep checks

## Wave 0 Gaps

None. Existing Astro build infrastructure covers all phase requirements. No test framework needed for a content audit phase.
