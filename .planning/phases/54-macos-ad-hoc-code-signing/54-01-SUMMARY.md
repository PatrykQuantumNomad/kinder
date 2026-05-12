---
phase: 54-macos-ad-hoc-code-signing
plan: 01
subsystem: infra
tags: [goreleaser, codesign, macos, github-actions, ad-hoc-signing, amfi, ci, release-pipeline]

# Dependency graph
requires:
  - phase: 51-version-pin
    provides: "GoReleaser v2.14.1 build pipeline + homebrew_casks cask config (pre-Phase-54 baseline)"
provides:
  - "darwin-gated builds[].hooks.post in .goreleaser.yaml that runs codesign --force --sign - on Mach-O binaries before the archive pipe"
  - "-s strip flag added to ldflags (alongside existing -w), satisfying DIST-01 wording -ldflags=\"-s -w\""
  - "release.yml runs-on switched from ubuntu-latest to macos-latest so codesign is on PATH"
  - "SC4 ordering invariant established: codesign is the LAST op inside builds[0] before archives:; no post-sign strip/UPX/copy; no top-level signs: key"
affects: [54-02-snapshot-verify-and-docs, 58-end-to-end-uat-and-release-tag]

# Tech tracking
tech-stack:
  added:
    - "macOS Xcode Command Line Tools (codesign) — preinstalled on GitHub-hosted macos-latest runner; not a Go-dep, no go.mod change"
  patterns:
    - "GoReleaser builds[].hooks.post for in-place binary modification before the archive pipe (Option A: shell-conditional gate on {{ .Os }}, not the undocumented if: field on build hooks)"
    - "Single builds entry + sh -c conditional for per-OS hook gating (avoids cascading archives: ids: edits a split-builds approach would require)"
    - "macos-latest runner + CGO_ENABLED=0 for pure-Go cross-compile to linux/windows from macOS host"

key-files:
  created: []
  modified:
    - ".goreleaser.yaml — added -s to ldflags; appended builds[0].hooks.post with darwin-gated codesign command"
    - ".github/workflows/release.yml — runs-on: ubuntu-latest → macos-latest"

key-decisions:
  - "Added -s strip flag to ldflags (Task 1) to match DIST-01's literal -ldflags=\"-s -w\" wording — RESEARCH Topic 2 Assumption A1 resolved as 'add -s'"
  - "Used shell conditional (sh -c 'if [ \"{{ .Os }}\" = \"darwin\" ]; ...') for darwin-gating, NOT GoReleaser's if: field — if: is undocumented for builds[].hooks.post in v2 (RESEARCH Topic 3 Assumption A3); shell conditional is cross-runner-safe and works when running goreleaser locally on Linux"
  - "Kept single builds entry + conditional hook; did NOT split into kinder-linux/kinder-darwin builds (RESEARCH Topic 4) — avoids archives ids: cascade and _v1 suffix issues for a one-hook case"
  - "Used --force --sign - (ad-hoc identity, dash = no certificate): --force makes the hook idempotent on re-runs; --sign - satisfies AMFI on Apple Silicon but explicitly NOT Gatekeeper notarization (SC3 wording deferred to Plan 54-02)"
  - "output: true on the hook is mandatory (not optional) so codesign stdout/stderr streams to GoReleaser logs for CI debuggability"
  - "macos-latest left as floating tag (Plan-level RESEARCH Open Question 2 RESOLVED): maps to macos-15 (Intel x64); codesign is architecture-agnostic and signs both darwin/amd64 and darwin/arm64; no pin to macos-15 or macos-15-arm64 needed unless reproducibility drift surfaces later"

patterns-established:
  - "SC4 ordering invariant: codesign MUST be the last operation on the Mach-O binary before archive. No post-sign strip, UPX, or binary copy in builds[0]. Future plans that add post-build steps MUST come BEFORE the hooks.post block, never after."
  - "Top-level signs: forbidden for Mach-O ad-hoc signing — that key signs the .tar.gz wrapper, not the binary inside; SC1/SC2 verify the binary in dist/, not the archive. Anti-pattern documented in RESEARCH Pitfall 1."
  - "Pre-merge CI verification (SC1/SC2) for the signing config is delivered by a separate workflow (Plan 54-02: .github/workflows/macos-sign-verify.yml on push to main + workflow_dispatch with paths filter), NOT inline in release.yml — keeps the tag-push release path clean and isolates the 10x macOS billing multiplier to verify-only triggers."

# Metrics
duration: ~1m 20s
completed: 2026-05-12
---

# Phase 54 Plan 54-01: GoReleaser Darwin Signing Summary

**Darwin-gated ad-hoc Mach-O signing wired into GoReleaser builds[].hooks.post and release workflow switched to macos-latest runner; SC4 ordering invariant established.**

## Performance

- **Duration:** ~1m 20s (80 seconds)
- **Started:** 2026-05-12T15:02:44Z
- **Completed:** 2026-05-12T15:04:04Z
- **Tasks:** 3 / 3
- **Files modified:** 2 (.goreleaser.yaml, .github/workflows/release.yml)
- **Commits:** 3 atomic task commits (no plan-metadata final commit yet — created after STATE.md update)

## Accomplishments

- `.goreleaser.yaml` `builds[0].ldflags` now carries `-s` (omit symbol table) alongside `-w` (omit DWARF debug info), satisfying DIST-01's `-ldflags="-s -w"` literal wording.
- `.goreleaser.yaml` `builds[0].hooks.post` now runs `sh -c 'if [ "{{ .Os }}" = "darwin" ]; then codesign --force --sign - "{{ .Path }}"; fi'` with `output: true`. Darwin builds get an ad-hoc Mach-O signature embedded before the archive pipe; linux/windows targets no-op the conditional.
- `.github/workflows/release.yml` job `release` now runs on `macos-latest` (was `ubuntu-latest`), so `codesign` is on PATH during the tag-push release pipeline. CGO_ENABLED=0 cross-compile to linux/windows continues to work (pure-Go; host-OS-agnostic).
- SC4 invariant established and structurally verified: `hooks:` is the LAST key inside `builds[0]` before the top-level `archives:` key; no top-level `signs:` key exists; no post-sign strip/UPX/copy operations follow the hook.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add `-s` strip flag to ldflags** — `f1df8c88` (build)
2. **Task 2: Add darwin-gated codesign post-hook to builds[]** — `bedb9541` (build)
3. **Task 3: Switch release runner to macos-latest** — `5c894f67` (ci)

**Plan metadata commit:** created after STATE.md/ROADMAP.md/REQUIREMENTS.md updates (final docs commit; references this SUMMARY.md).

## Files Created/Modified

- `.goreleaser.yaml` — Two surgical edits inside `builds[0]`: (1) inserted `-s` between `-buildid=` and `-w` in `ldflags`; (2) appended `hooks.post` block after `mod_timestamp:` as the new last key in `builds[0]`. No other fields touched. Top-level `signs:` deliberately NOT added.
- `.github/workflows/release.yml` — Single-line change on the `release` job's `runs-on:` field. Checkout, setup-go, goreleaser-action steps and env vars/permissions all unchanged.

## Decisions Made

- **`-s` added to ldflags (Task 1):** RESEARCH Topic 2 Assumption A1 noted DIST-01 wording implies both `-s` and `-w` but the repo only had `-w`. Resolved by adding `-s`. Standard practice for release binaries; no runtime effect on Go binaries.
- **Shell conditional, not GoReleaser `if:` field (Task 2):** GoReleaser v2's `if:` field is documented for global `before.hooks`/`after.hooks` but NOT for `builds[].hooks.post` (RESEARCH Topic 3 / Assumption A3). Used `sh -c 'if [...] fi'` from RESEARCH Topic 3 Option A. Cross-runner-safe and well-documented.
- **Single builds entry kept (Task 2):** RESEARCH Topic 4 recommended NOT splitting `builds:` into linux-vs-darwin entries. A split would force `archives:` to gain an `ids:` filter cascade and the `_v1` GOAMD64 suffix on amd64 would complicate archive name templates. Conditional hook keeps config minimal.
- **`output: true` on the hook (Task 2):** Mandatory, not optional. RESEARCH Pitfall 3 notes the failure mode where codesign silently fails in CI; streaming stdout/stderr to GoReleaser logs prevents that.
- **`macos-latest` floating tag (Task 3):** RESEARCH Open Question 2 RESOLVED. Matches DIST-01 wording verbatim; GitHub auto-rolls the alias when macos-26 becomes default. If a pin is needed later for reproducibility, file a debt item rather than pre-pinning.

## Deviations from Plan

None — plan executed exactly as written. All three atomic edits matched the plan's YAML snippets byte-for-byte; all three task verify blocks passed on first run; overall verification block (no top-level `signs:`, hooks is last `builds[0]` field, darwin conditional present) all PASS.

## Issues Encountered

None.

## User Setup Required

None — pure release-pipeline plumbing. No external service configuration, no env vars added, no secrets needed. The existing `GITHUB_TOKEN` and `HOMEBREW_TAP_TOKEN` secrets in `release.yml` continue to apply unchanged.

## Verification Deferrals

- **SC1 / SC2 (CI signature verification):** Deferred to Plan 54-02. Plan 54-02 creates `.github/workflows/macos-sign-verify.yml` triggered on push-to-main + workflow_dispatch with a `paths:` filter on `.goreleaser.yaml`/release.yml/macos-sign-verify.yml. That workflow runs `goreleaser release --snapshot --clean --skip=publish` and asserts `codesign -vvv` exit 0 + "satisfies its Designated Requirement" on both darwin/amd64 and darwin/arm64 dist binaries.
- **SC3 (install doc wording — "ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine kinder`"):** Deferred to Plan 54-02 (`kinder-site/src/content/docs/installation.md` edit).
- **PROJECT.md Key Decisions row + REQUIREMENTS.md DIST-01 mark-complete:** Deferred to Plan 54-02 (after CI verification proves the pipeline works end-to-end).

## SC4 Invariant Confirmation

The plan's overall verification block was executed after Task 3 and PASSED on all three structural checks:

1. **No top-level `signs:` key in `.goreleaser.yaml`** — grep returns no match; PASS. (Would have signed the `.tar.gz` wrapper, breaking SC1/SC2 which verify the binary in `dist/`.)
2. **`hooks:` is the last `builds[0]` field before `archives:`** — awk-extracted last builds-block field is `    hooks:`; PASS. No subsequent strip/UPX/binary-copy can invalidate the Mach-O signature block.
3. **Darwin shell-conditional present** — `if [ "{{ .Os }}" = "darwin" ]` substring confirmed in file; PASS. Linux/windows targets remain unsigned (correct — they are not Mach-O).

## Next Phase Readiness

- **Plan 54-02 unblocked:** Ready to land snapshot-verify workflow + install-doc wording + PROJECT.md decision row + REQUIREMENTS.md DIST-01 mark-complete. Plan 54-02 depends on this plan's commits being on `main` (they now are).
- **Tag-push release path:** From this commit forward, any `v*` tag push will route to `macos-latest`, run codesign on darwin builds, and produce signed Mach-O binaries inside the tar.gz archives. The first real validation will come from Plan 54-02's snapshot-verify workflow (on the next push to `main` that touches `.goreleaser.yaml` or workflow files).
- **Billing awareness:** macOS runners are a 10x multiplier on Actions minutes vs Linux. Release pipeline runs on tag push only; per-release cost increase is ~5min × 10 = ~50 actions-minutes. Acceptable for a public-repo free-tier project.

## Self-Check: PASSED

- `.planning/phases/54-macos-ad-hoc-code-signing/54-01-SUMMARY.md` — exists
- `.goreleaser.yaml` contains `-s` in ldflags — confirmed
- `.goreleaser.yaml` contains `codesign --force --sign -` hook — confirmed
- `.github/workflows/release.yml` has `runs-on: macos-latest` (no remaining `ubuntu-latest`) — confirmed
- Commit `f1df8c88` (Task 1: ldflags `-s`) — in `git log`
- Commit `bedb9541` (Task 2: hooks.post) — in `git log`
- Commit `5c894f67` (Task 3: macos-latest runner) — in `git log`

---
*Phase: 54-macos-ad-hoc-code-signing*
*Plan: 54-01*
*Completed: 2026-05-12*
