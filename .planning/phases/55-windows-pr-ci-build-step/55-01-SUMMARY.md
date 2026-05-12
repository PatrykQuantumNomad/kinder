---
phase: 55-windows-pr-ci-build-step
plan: 01
subsystem: infra
tags: [github-actions, ci, windows, cross-compile, cgo, dist-02]

# Dependency graph
requires:
  - phase: 54-macos-ad-hoc-code-signing
    provides: DIST-01 delivered; CI pipeline pattern established (SHA-pinned actions, ubuntu-24.04, workflow_dispatch smoke-test convention)
provides:
  - .github/workflows/build-check.yml — PR-event Windows/amd64 cross-compile build check (DIST-02 SC1+SC3 workflow-level)
  - Green workflow_dispatch run 25750801764 proving trigger wiring and CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./... exits 0 on ubuntu-24.04 runner
  - DIST-02 marked Complete in REQUIREMENTS.md
affects: [Phase 56 (DEBT-04), Phase 57 (DIAG), Phase 58 (UAT), any future CI-policy phase configuring branch protection]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "workflow_dispatch smoke-test: every new CI workflow includes workflow_dispatch: {} so it can be immediately verified without opening a PR (established in Phase 54, reinforced here)"
    - "ubuntu-24.04 pin over ubuntu-latest: repo-wide convention for PR-event workflows (docker.yaml, nerdctl.yaml, podman.yml, vm.yaml, build-check.yml) — deviation from DIST-02 literal wording documented in workflow header comment"
    - "env: block for build environment (CGO_ENABLED/GOOS/GOARCH) rather than inline shell vars — GitHub Actions renders env block as collapsible section in CI logs"
    - "Layer (a) vs layer (b) blocking distinction: workflow-level (job exit code → PR check red) vs merge-level (branch protection required status check); only layer (a) in scope for this phase"

key-files:
  created:
    - .github/workflows/build-check.yml
    - .planning/phases/55-windows-pr-ci-build-step/55-01-SUMMARY.md
  modified:
    - .planning/REQUIREMENTS.md
    - .planning/STATE.md

key-decisions:
  - "ubuntu-24.04 used instead of DIST-02 literal ubuntu-latest — repo-wide pin convention; deviation documented in workflow header comment"
  - "Layer (a) blocking (workflow-level) delivered; Layer (b) blocking (merge-level branch protection) deferred to future CI-policy phase per RESEARCH recommendation and user selection"
  - "Task 3 = defer: windows-build check is advisory (paints PR red but does not prevent merge) until a dedicated CI-policy phase configures required status checks for ALL PR workflows consistently"

patterns-established:
  - "DIST-02 audit trail: SC2 local probe transcript (CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...) must be documented before YAML authoring for any future cross-compile workflow"
  - "Check name clarity: workflow name + job key + job name all specified explicitly so the resulting GitHub check name is deterministic (Windows Build Check / windows-build)"

# Metrics
duration: ~1 session (Task 1 commit c8d4bfbd + Task 2 dispatch verification + Task 3 defer decision)
completed: 2026-05-12
---

# Phase 55 Plan 01: Windows PR-CI Build Step Summary

**`CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` wired into PR CI via `.github/workflows/build-check.yml` on ubuntu-24.04; green workflow_dispatch run 25750801764 proves end-to-end; DIST-02 closed.**

## Performance

- **Duration:** ~1 session
- **Started:** 2026-05-12 (Task 1 implementation)
- **Completed:** 2026-05-12T16:xx:xxZ (Task 2 dispatch verify + Task 3 defer + plan close)
- **Tasks:** 3 (Task 1 auto-completed, Task 2 human-verified, Task 3 deferred per user decision)
- **Files modified:** 1 new workflow file + 3 planning docs (SUMMARY + REQUIREMENTS + STATE)

## Accomplishments

- Created `.github/workflows/build-check.yml` — single-job workflow that cross-compiles the entire Go module for windows/amd64 on every PR to main, preventing silent cgo transitive dependency regressions from reaching release-tag time (Pitfall 18 closed)
- Verified end-to-end via `workflow_dispatch` run 25750801764: conclusion=success, duration=32s, all 5 steps green including the cross-compile step; check name resolved to `Windows Build Check / windows-build` (Pitfall 7 cleared)
- DIST-02 marked Complete in REQUIREMENTS.md; Task 3 (branch protection) deferred to a future CI-policy phase per RESEARCH recommendation

## Task Commits

1. **Task 1: Re-run local SC2 cgo probe + create build-check.yml** — `c8d4bfbd` (ci)
   - Local probe at HEAD d69a67cb (RESEARCH) and at implementation HEAD: `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` exit 0 both times
   - `.github/workflows/build-check.yml` written and pushed to `origin/main`
2. **Task 2: Green CI via workflow_dispatch** — no commit (verification-only task)
   - Run 25750801764 on `main` completed `success` in 32s
3. **Task 3: Branch protection decision** — `defer` (no commit required per plan options block)

**Plan metadata (docs close):** `<this-commit-hash>` (docs: plan close, DIST-02 Complete, STATE)

## SC2 Local Probe Transcript

Both RESEARCH pre-verification and executor re-verification at implementation HEAD passed:

- RESEARCH probe at HEAD `d69a67cb`: `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` — exit 0
- Executor re-probe at implementation HEAD (immediately before writing YAML, same HEAD): exit 0

Go toolchain: `.go-version` contains `1.25.7` (single-source-of-truth; consumed by `actions/setup-go` via `go-version-file: .go-version`).

Optional confidence probe (no cgo files in Windows build graph):
```
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go list -f '{{.ImportPath}} {{.CgoFiles}}' ./... | grep -v '\[\]'
```
Result: no output (clean — no cgo files under GOOS=windows).

## Workflow File (Committed YAML — Audit Trail)

File: `.github/workflows/build-check.yml` (commit `c8d4bfbd`)

```yaml
# Source: .planning/phases/55-windows-pr-ci-build-step/55-RESEARCH.md (Example 1)
# Purpose: DIST-02 — Pitfall 18 prevention. Cross-compile the Go module for
# windows/amd64 from a Linux runner on every PR to main, so silent cgo transitive
# dependency regressions surface at PR-open instead of release-tag time.
#
# Runner pin: this workflow uses `ubuntu-24.04` (NOT `ubuntu-latest` per DIST-02
# literal wording). Rationale: the repo pins ubuntu versions in four other PR
# workflows (docker.yaml, nerdctl.yaml, podman.yml, vm.yaml); `ubuntu-latest`
# floats and has caused CI drift industry-wide. The DIST-02 intent (a Linux
# Ubuntu runner) is fully satisfied by `ubuntu-24.04`.
#
# Check name in PR UI: "Windows Build Check / windows-build"
# Workflow-level "blocking": this job's non-zero exit fails the PR check status.
# Merge-level "blocking" (preventing merge on red): requires repo-level branch
# protection on `main` listing `windows-build` as a required status check. As of
# this workflow's creation, `main` is NOT protected (verified via `gh api`); the
# check is therefore advisory until branch protection is configured. See Plan
# 55-01 Task 3 (optional, [NEEDS USER CONFIRMATION]).
name: Windows Build Check

on:
  pull_request:
    branches:
      - main
    paths-ignore:
      - 'site/**'
      - 'kinder-site/**'
  workflow_dispatch: {}

permissions:
  contents: read

jobs:
  windows-build:
    name: windows-build
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2

      - uses: actions/setup-go@7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5 # v6.2.0
        with:
          go-version-file: .go-version

      - name: Cross-compile for windows/amd64 (cgo transitive dep probe — Pitfall 18)
        env:
          CGO_ENABLED: "0"
          GOOS: windows
          GOARCH: amd64
        run: go build ./...
```

## Task 2 — Green CI Run (Checkpoint Resume Signal)

Resume signal: `green-via-dispatch 25750801764`

- Run URL: https://github.com/PatrykQuantumNomad/kinder/actions/runs/25750801764
- Trigger: `workflow_dispatch` on `main` (branching_strategy=none; no PR available immediately)
- Conclusion: `success`
- Duration: 32s
- All 5 steps green, including "Cross-compile for windows/amd64 (cgo transitive dep probe — Pitfall 18)"
- Check name resolved: `Windows Build Check / windows-build` (Pitfall 7 cleared — unambiguous for any future branch-protection required-status-check config)

## Task 3 — Branch Protection Decision

**User selection: `defer`** (RESEARCH-recommended option)

Rationale: Workflow-level SC3 satisfied; merge-level protection deferred to a future dedicated CI-policy phase to avoid `windows-build`-only asymmetry vs other PR workflows (`docker.yaml`, `nerdctl.yaml`, `podman.yml`, `vm.yaml`, `macos-sign-verify.yml`) which are also not listed as required status checks.

The layer (a) vs layer (b) "blocking" distinction is documented in the workflow file header (lines 12–17 of `.github/workflows/build-check.yml`) as the persistent audit trail. No `gh api` call. No PROJECT.md row (per plan options block: `document-only` is the option that adds a PROJECT.md row, not `defer`).

## Out-of-Scope Guards (Negative Grep Suite)

All verified immediately before writing this SUMMARY (set -e; all exit 0):

| Guard | Pattern | Status |
|-------|---------|--------|
| No pull_request_target (Pitfall 2) | `pull_request_target:` | NOT present |
| No matrix (single target only) | `matrix:` | NOT present |
| No go vet (build-only per ROADMAP) | `go vet` | NOT present |
| No concurrency block (not used elsewhere) | `concurrency:` | NOT present |
| No fetch-depth (build needs only HEAD) | `fetch-depth:` | NOT present |
| No manual actions/cache (setup-go v6 caches by default) | `uses: actions/cache@` | NOT present |
| No continue-on-error neutralizer | `continue-on-error: true` | NOT present |
| No if: success() neutralizer | `if: success()` | NOT present |

Scope discipline ("smaller is better") holds.

## Files Created/Modified

- `.github/workflows/build-check.yml` — NEW: 51-line single-job PR+dispatch workflow; cross-compiles for windows/amd64 with CGO disabled; SHA-pinned actions; ubuntu-24.04; go-version-file pin; DIST-02 SC1+SC3
- `.planning/phases/55-windows-pr-ci-build-step/55-01-SUMMARY.md` — NEW: this file
- `.planning/REQUIREMENTS.md` — DIST-02 checkbox toggled Pending → Complete; traceability row updated; last-updated note appended
- `.planning/STATE.md` — progress counters, current position, decisions, session continuity all updated

## Decisions Made

1. `ubuntu-24.04` over `ubuntu-latest`: Repo-wide pin convention for PR-event workflows. Deviation from DIST-02 literal wording; documented in workflow header comment.
2. `env:` block over inline shell vars: GitHub Actions renders env blocks as collapsible sections in CI logs; improves failure diagnosis.
3. Task 3 = `defer`: Layer (a) blocking (workflow-level) is the in-scope DIST-02 SC3 reading per RESEARCH. Layer (b) (merge-prevention) left to a future CI-policy phase that can configure ALL required checks at once.

## Deviations from Plan

None — plan executed exactly as specified. Task 3 `defer` was the RESEARCH-recommended option; the plan's checkpoint:decision block anticipated and documented it.

## Issues Encountered

None. SC2 local probe exited 0 at both verification points (RESEARCH pre-verify at d69a67cb + executor re-verify at implementation HEAD). workflow_dispatch run 25750801764 succeeded in 32s on first attempt.

## Next Phase Readiness

- Phase 56 (DEBT-04 — pre-existing data race in doctor package tests) is unblocked; no dependency on Phase 55 artifacts
- Phase 57 (DIAG-05, DIAG-06) unblocked; same package, no dependency
- Phase 58 (UAT-01, UAT-02) unblocked; requires final binary
- Future CI-policy phase: when ready to configure branch protection, the check name `Windows Build Check / windows-build` is stable and unambiguous; the `gh api` command is documented in the plan's Task 3 `configure-now` option block for reference

---
*Phase: 55-windows-pr-ci-build-step*
*Completed: 2026-05-12*
