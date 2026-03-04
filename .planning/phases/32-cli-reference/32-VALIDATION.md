# Phase 32: CLI Reference - Validation

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Astro build (no automated test suite) |
| Quick run | `cd kinder-site && npm run build 2>&1 \| tail -5` |
| Full suite | `cd kinder-site && npm run build` |

## Requirements Test Map

| Req ID | Behavior | Automated Command |
|--------|----------|-------------------|
| CLI-01 | profile-comparison.md contains table of all 4 presets | `grep -c "minimal\|full\|gateway\|ci" kinder-site/src/content/docs/cli-reference/profile-comparison.md` (expect >= 8) |
| CLI-02 | json-output.md has examples for all 4 commands | `grep -c "get clusters\|get nodes\|kinder env\|kinder doctor" kinder-site/src/content/docs/cli-reference/json-output.md` (expect >= 4) |
| CLI-03 | troubleshooting.md has exit code tables | `grep -c "Exit code\|exit code" kinder-site/src/content/docs/cli-reference/troubleshooting.md` (expect >= 2) |

## Sampling Rates

- **Per task commit:** `cd kinder-site && npm run build 2>&1 | tail -10`
- **Per wave merge:** `cd kinder-site && npm run build`
- **Phase gate:** Full build green + grep checks above

## Wave 0 Gaps

None. Existing Astro build infrastructure covers all phase requirements.
