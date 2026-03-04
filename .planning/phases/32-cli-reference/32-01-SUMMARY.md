---
phase: 32-cli-reference
plan: 01
subsystem: docs
tags: [cli, starlight, markdown, profile-presets, json-output, troubleshooting, jq]

# Dependency graph
requires:
  - phase: 31-addon-page-depth
    provides: "Symptom/Cause/Fix troubleshooting pattern established for docs"
  - phase: 30-foundation-fixes
    provides: "Placeholder pages with Starlight callout structure"
provides:
  - "Profile comparison page with 4 presets x 7 addons table and use-case recommendations"
  - "JSON output reference with examples for all 4 commands (get clusters, get nodes, env, doctor) and jq recipes"
  - "Troubleshooting page with exit code tables for kinder env (0,1) and kinder doctor (0,1,2) and Symptom/Cause/Fix entries"
affects: [future-cli-reference, guides-section]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Symptom/Cause/Fix format applied to CLI error troubleshooting"
    - ":::tip[When to use] callouts for preset use-case recommendations"
    - "Exit code tables in dedicated ### Exit codes subsections"

key-files:
  created: []
  modified:
    - kinder-site/src/content/docs/cli-reference/profile-comparison.md
    - kinder-site/src/content/docs/cli-reference/json-output.md
    - kinder-site/src/content/docs/cli-reference/troubleshooting.md

key-decisions:
  - "kinder env outputs a single JSON object (not array) — documented with :::note to prevent .[] misuse"
  - "container-runtime check name (fallback) vs actual runtime name (docker/podman/nerdctl) distinction clarified"
  - "Commands without JSON support (get kubeconfig, create cluster, delete cluster) explicitly listed to prevent user confusion"
  - "Headlamp used as user-facing addon name (not Dashboard) matching configuration.md and addon pages"

patterns-established:
  - "CLI reference pages: Exit codes table + Common errors (Symptom/Cause/Fix) + Shell integration tip"
  - "JSON output pages: command invocation + shape description + example JSON + jq recipes per command"

requirements-completed: [CLI-01, CLI-02, CLI-03]

# Metrics
duration: 2min
completed: 2026-03-04
---

# Phase 32 Plan 01: CLI Reference Summary

**Three CLI reference placeholder pages replaced with profile preset tables, JSON output examples for all 4 commands with jq recipes, and exit code tables plus Symptom/Cause/Fix troubleshooting for kinder env and kinder doctor.**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-04T15:33:55Z
- **Completed:** 2026-03-04T15:35:30Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Profile comparison page: 4 presets x 7 addons table, per-preset details with :::tip use-case callouts, --profile vs --config caution
- JSON output page: --output json shapes and example output for get clusters, get nodes, env, doctor; 3+ jq recipes per command; common jq patterns section; explicit list of commands without JSON support
- Troubleshooting page: exit code tables for env (0,1) and doctor (0,1,2); check result table with detection order; 4 Symptom/Cause/Fix scenarios; CI guard pattern

## Task Commits

Each task was committed atomically:

1. **Task 1: Write profile comparison page** - `a413c8b4` (feat)
2. **Task 2: Write JSON output reference page** - `4afcb7c1` (feat)
3. **Task 3: Write troubleshooting page for env and doctor** - `baf97475` (feat)

## Files Created/Modified
- `kinder-site/src/content/docs/cli-reference/profile-comparison.md` - Profile preset comparison table and per-preset details with use cases
- `kinder-site/src/content/docs/cli-reference/json-output.md` - JSON output examples for all 4 commands with jq recipes and common patterns
- `kinder-site/src/content/docs/cli-reference/troubleshooting.md` - Exit code tables and Symptom/Cause/Fix troubleshooting for kinder env and doctor

## Decisions Made
- Documented kinder env as single-object JSON (not array) with a :::note callout to prevent users from applying `.[]` incorrectly
- Used "Headlamp" as the addon name (not "Dashboard") matching the user-facing convention across all documentation
- Listed commands without JSON support explicitly so users do not waste time trying unsupported flags
- Used `container-runtime` as the fallback check name vs actual runtime names (docker/podman/nerdctl) with a note explaining the detection order

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- All 3 CLI reference placeholder pages are now substantive content
- Site builds cleanly, all pages render without errors
- Phase 32 plan 01 complete; ready for remaining plans in phase 32 (if any)

---
*Phase: 32-cli-reference*
*Completed: 2026-03-04*
