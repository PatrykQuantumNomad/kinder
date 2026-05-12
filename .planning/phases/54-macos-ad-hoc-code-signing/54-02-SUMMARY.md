---
phase: 54-macos-ad-hoc-code-signing
plan: 02
subsystem: infra
tags: [github-actions, codesign, macos, snapshot-build, ci-verification, sc1, sc2, sc3, docs, key-decisions, ad-hoc-signing, dist-01]

# Dependency graph
requires:
  - phase: 54-macos-ad-hoc-code-signing
    plan: 01
    provides: "Darwin-gated codesign hook in .goreleaser.yaml builds[].hooks.post + release.yml runs-on: macos-latest. The new snapshot-verify workflow exercises that exact pipeline via `goreleaser release --snapshot --clean --skip=publish` on macos-latest."
provides:
  - "`.github/workflows/macos-sign-verify.yml`: snapshot-build + per-arch `codesign -vvv` CI gate (push-to-main with paths filter on .goreleaser.yaml + this workflow file, plus workflow_dispatch); fails non-zero if either darwin/amd64 or darwin/arm64 dist binary fails verification or omits the literal `satisfies its Designated Requirement` string — delivers SC1 + SC2 once a triggering push lands on main"
  - "SC3 user-facing disclosure landed in all three doc files: `kinder-site/src/content/docs/installation.md` (`:::caution[macOS direct download]` Starlight admonition), `kinder-site/src/content/docs/changelog.md` (new `### macOS Ad-Hoc Signing (Phase 54)` subsection inside `## v2.4 — Hardening` above the closing `---`), `.planning/release-notes-v2.4-draft.md` (new `## macOS Ad-Hoc Signing (Phase 54)` section between `## Internal Changes (informational)` and `## Verification`). SC3 binding conjunction (Release notes AND install guide) fully covered."
  - "`.planning/PROJECT.md` Key Decisions table: new row `Ad-hoc codesign on macos-latest, not notarization` with AMFI-on-Apple-Silicon rationale, zero-cert-cost trade-off, and DIST-03 deferral pointer; `Last updated` footer bumped to 2026-05-12 with Phase 54 attribution"
affects: [58-end-to-end-uat-and-release-tag]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Snapshot-verify CI gate as a SEPARATE workflow (not inline in release.yml) — keeps the tag-push release path clean; isolates the 10x macOS runner billing multiplier to verify-only triggers; gated by `paths:` filter on `.goreleaser.yaml` + the workflow file so PR/main pushes that don't touch signing config don't burn macOS minutes"
    - "`find dist/ -name kinder -path '*darwin_<arch>*' | head -1` glob pattern for snapshot binary discovery — survives the version-string component (`kinder_0.1.0-SNAPSHOT-<sha>_darwin_amd64_v1`) that snapshot builds prepend to dist paths; works for release builds too"
    - "`set -euo pipefail` + explicit empty-binary check + `tee` + `grep -q` — three-layer gate: bash strict mode fails on any command failure; the `if [ -z \"$BIN\" ]` branch gives an actionable error before exit; `codesign -vvv` writes to stderr so `2>&1` merges it for `tee` and `grep` to see; `grep -q` exits 0 only if the exact SC1/SC2 wording (`satisfies its Designated Requirement`) is present"
    - "Three-tier SC3 disclosure mirror: install guide (kinder-site/installation.md) + canonical changelog (kinder-site/changelog.md inside `## v2.4 — Hardening`) + planning-side release-notes draft (.planning/release-notes-v2.4-draft.md). All three carry the verbatim backtick-wrapped SC3 string exactly once — `grep -cF` returns 1 in each file. The draft explicitly declares `kinder-site/src/content/docs/changelog.md` as its source of truth, so the mirror is structural, not duplication."

key-files:
  created:
    - ".github/workflows/macos-sign-verify.yml — 64-line workflow file (name, on, permissions, single job with 4 steps: checkout/setup-go/snapshot-build/two verify steps)"
  modified:
    - "kinder-site/src/content/docs/installation.md — appended a `:::caution[macOS direct download]` admonition between the macOS/Linux extract code fence and the Windows heading; +12 lines, 0 deletions"
    - "kinder-site/src/content/docs/changelog.md — appended `### macOS Ad-Hoc Signing (Phase 54)` subsection inside `## v2.4 — Hardening` above the closing `---`; +8 lines, 0 deletions"
    - ".planning/release-notes-v2.4-draft.md — inserted `## macOS Ad-Hoc Signing (Phase 54)` section between `## Internal Changes (informational)` and `## Verification`; +16 lines, 0 deletions"
    - ".planning/PROJECT.md — appended Key Decisions row; bumped `Last updated` footer from 2026-05-09 to 2026-05-12; +1 line, 1 footer-line replacement"

key-decisions:
  - "Workflow trigger restricted to push-on-main with `paths:` filter on `.goreleaser.yaml` + this workflow file ONLY (plus workflow_dispatch for manual re-runs). DO NOT trigger on pull_request or on every push to main — macOS runners are a 10x Linux billing multiplier. Including .github/workflows/release.yml in the paths filter was deliberately AVOIDED: release.yml runs on tag push only, and its contents do not affect the signed-binary bytes (those depend on .goreleaser.yaml hooks + the codesign command)."
  - "Used `find dist/ -name kinder -path '*darwin_amd64*' | head -1` glob pattern instead of the hardcoded SC1 path `dist/kinder_darwin_amd64_v1/kinder`. Snapshot dist paths include the version string (e.g. `kinder_0.1.0-SNAPSHOT-abc1234_darwin_amd64_v1`); the SC wording is about the binary identity, not the literal snapshot path string. The glob works for both snapshot and tag-push release dist layouts."
  - "`grep -q \"satisfies its Designated Requirement\"` is the explicit success gate, NOT just relying on `codesign` exit code. The SC1/SC2 wording specifies the output STRING as the success signal. The two gates together (codesign exit 0 + grep finds the line) is the correct read."
  - "SC3's `Release notes AND install guide` conjunction interpreted as a 3-file insertion: install guide (installation.md) AND release notes — where `release notes` requires BOTH the user-facing canonical changelog (kinder-site/changelog.md) AND the planning-side draft (`.planning/release-notes-v2.4-draft.md`). The draft's header explicitly declares the changelog as source of truth, so mirroring is structural. The SC3 verifier `grep -F`s the literal backtick-wrapped form in each file; we asserted `grep -cF` returns exactly 1 per file."
  - "PROJECT.md row wording matches the existing convention: short comparative Decision phrase (`Ad-hoc codesign on macos-latest, not notarization` — mirrors `Headlamp over kubernetes/dashboard` style), Rationale captures (1) the technical why-now (AMFI on Apple Silicon kills unsigned arm64 binaries), (2) the cost trade-off (zero certificate overhead), and (3) the deferral pointer (DIST-03 notarization). Outcome column uses `✓ Good` per the v2.2-block convention."

patterns-established:
  - "**Snapshot-verify CI gate pattern**: For any release-pipeline regression where the cost of catching a bad config at tag-push time is high (e.g., a broken codesign config silently produces unsigned macOS binaries until the next release), add a SEPARATE workflow that triggers on main-branch push with a `paths:` filter constrained to the config files (and the workflow itself for self-tests). Use the same goreleaser-action + setup-go SHAs as the release workflow so the snapshot path exercises the same toolchain as the real release. Use `--snapshot --clean --skip=publish` for the build step."
  - "**SC-wording-as-grep-target convention**: When an SC quotes a specific output string (e.g., `satisfies its Designated Requirement`), the CI verify step MUST use that exact string as a `grep -q` gate, NOT a derived/paraphrased substring. The grep is the contract."
  - "**Three-tier disclosure mirror for v2.4 release notes**: Any release-affecting disclosure that ships in v2.4 lives in three places: (1) the user-facing canonical changelog (kinder-site/src/content/docs/changelog.md, inside the `## v2.4 — Hardening` section above the closing `---`), (2) the planning-side release-notes draft (`.planning/release-notes-v2.4-draft.md`, mirroring the changelog slice), and (3) if user-actionable, the install/usage docs (kinder-site/src/content/docs/installation.md). Each tier contains the same canonical wording verbatim; `grep -cF` should return exactly 1 per tier."

# Metrics
duration: ~2m 49s
completed: 2026-05-12
---

# Phase 54 Plan 54-02: Snapshot Verify and Docs Summary

**Snapshot-build + `codesign -vvv` CI gate landed at `.github/workflows/macos-sign-verify.yml`; SC3 ad-hoc-signing disclosure mirrored into three docs (install guide, changelog, v2.4 release-notes draft); ad-hoc-signing decision recorded in PROJECT.md Key Decisions — Phase 54 SCs 1+2+3 closed (SC4 was closed in Plan 54-01).**

## Performance

- **Duration:** ~2m 49s (169 seconds)
- **Started:** 2026-05-12T15:08:35Z
- **Completed:** 2026-05-12T15:11:24Z
- **Tasks:** 3 / 3
- **Files modified/created:** 5 (1 created: `.github/workflows/macos-sign-verify.yml`; 4 modified: installation.md, changelog.md, release-notes-v2.4-draft.md, PROJECT.md)
- **Commits:** 3 atomic task commits (no plan-metadata final commit yet — created after STATE.md/ROADMAP.md update)

## Accomplishments

- **SC1 + SC2 delivered structurally** via `.github/workflows/macos-sign-verify.yml`: macos-latest runner; same checkout (`actions/checkout@de0fac2e…`) + setup-go (`actions/setup-go@7a3fe6cf…`) SHA pins as release.yml; `goreleaser release --snapshot --clean --skip=publish` produces dist/ artifacts via the SAME `builds[].hooks.post` codesign pipeline that release.yml runs; two separate verify steps (one per arch) each gated by `set -euo pipefail` + empty-binary check + `codesign -vvv ... | tee` + `grep -q "satisfies its Designated Requirement"`. The first triggering push (or workflow_dispatch run) will produce the authoritative SC1+SC2 CI log evidence.
- **SC3 delivered in all three doc files**: installation.md gets a `:::caution[macOS direct download]` Starlight admonition with the verbatim backtick-wrapped SC3 wording, `Killed: 9`/AMFI-vs-Gatekeeper explainer, the literal `xattr -d com.apple.quarantine kinder` command, and the Homebrew-bypasses-quarantine note. changelog.md gets a new `### macOS Ad-Hoc Signing (Phase 54)` subsection inside the `## v2.4 — Hardening` block (above the closing `---`) with the same verbatim wording and a DIST-03 deferral pointer (DIST-01). release-notes-v2.4-draft.md gets a parallel `## macOS Ad-Hoc Signing (Phase 54)` section between `## Internal Changes (informational)` and the closing `## Verification` section. All three carry the SC3 wording verbatim and exactly once per file.
- **PROJECT.md Key Decisions row landed** as the future-self memo: `Ad-hoc codesign on macos-latest, not notarization` / AMFI-on-Apple-Silicon + zero-cert-cost + DIST-03 deferral / ✓ Good. Footer bumped to 2026-05-12 with Phase 54 attribution.
- **SC4 invariant preserved**: `.goreleaser.yaml` and `.github/workflows/release.yml` are NOT modified in this plan — they were finalized in Plan 54-01. `git diff --stat HEAD~3 HEAD -- .goreleaser.yaml` returns empty (verified during the plan-level verification block).

## Task Commits

Each task was committed atomically:

1. **Task 1: Create `.github/workflows/macos-sign-verify.yml` snapshot+verify CI workflow** — `693894be` (ci)
2. **Task 2: Insert SC3 wording into installation.md + changelog.md + release-notes-v2.4-draft.md** — `78d5fdb1` (docs)
3. **Task 3: Add ad-hoc signing row to PROJECT.md Key Decisions table** — `f078893e` (docs)

**Plan metadata commit:** created after STATE.md/ROADMAP.md/REQUIREMENTS.md updates (final docs commit; references this SUMMARY.md).

## Files Created/Modified

- `.github/workflows/macos-sign-verify.yml` (NEW, 64 lines): Single-job workflow on `macos-latest`. Triggers: push to main with paths filter on `.goreleaser.yaml` + this workflow file; workflow_dispatch with no inputs. Permissions: `contents: read` (no publish, no write needed). Steps: checkout (fetch-depth: 0 for goreleaser changelog), setup-go (`.go-version` pinned), goreleaser-action v7 `release --snapshot --clean --skip=publish`, then two verify steps (one per arch) each with bash strict mode, empty-binary guard, `codesign -vvv` with stderr merged for tee, `grep -q "satisfies its Designated Requirement"`. No `pull_request:` trigger (cost guard). No `release.yml` in paths (release is tag-push only).
- `kinder-site/src/content/docs/installation.md` (+12 lines): Added a `:::caution[macOS direct download]` Starlight admonition block immediately after the macOS/Linux extract code fence (line 37) and before the Windows heading (now at line 51). The bolded SC3 sentence preserves the backticks around `xattr -d com.apple.quarantine` exactly. A separate `sh` code fence inside the admonition shows the literal `xattr -d com.apple.quarantine kinder` command.
- `kinder-site/src/content/docs/changelog.md` (+8 lines): Appended `### macOS Ad-Hoc Signing (Phase 54)` subsection inside the `## v2.4 — Hardening` block, immediately before the closing `---` separator. Subsection contains: bolded SC3 wording, AMFI/AMfi-vs-Gatekeeper paragraph, mention of `xattr -d com.apple.quarantine kinder` and Homebrew bypass, DIST-03 deferral pointer with `(DIST-01)` requirement-ID suffix matching the convention of existing v2.4 bullets.
- `.planning/release-notes-v2.4-draft.md` (+16 lines): Inserted `## macOS Ad-Hoc Signing (Phase 54)` section between `## Internal Changes (informational)` and `## Verification`. Contains a slightly more verbose mirror of the changelog subsection — adds an explicit `sh` code fence with the `xattr` command (matching the install guide pattern). Verification section remains the final section of the file.
- `.planning/PROJECT.md` (+1 row, 1 footer line replaced): New Key Decisions row appended to the v2.2 block (the LAST Key Decisions block in the file, which by convention rolls forward with new decisions). Footer changed from `*Last updated: 2026-05-09 — milestone v2.4 Hardening started*` to `*Last updated: 2026-05-12 — Phase 54 (macOS ad-hoc code signing) decision recorded*`.

## Decisions Made

- **Workflow trigger scope (Task 1):** push-on-main with `paths:` filter on `.goreleaser.yaml` + this workflow file only, plus `workflow_dispatch: {}` for manual re-runs. Rejected: `pull_request:` (every PR on macos-latest at a 10x billing multiplier is wasteful), `push.branches: [main]` without `paths:` (same problem at lower frequency), including `.github/workflows/release.yml` in paths (release.yml runs on tag push only and its contents don't affect signed-binary bytes — the codesign command lives in .goreleaser.yaml).
- **Binary discovery glob (Task 1):** `find dist/ -name kinder -path '*darwin_<arch>*' | head -1` instead of the hardcoded SC1/SC2 paths (`dist/kinder_darwin_amd64_v1/kinder` and `dist/kinder_darwin_arm64/kinder`). Snapshot builds prefix the version string (`kinder_0.1.0-SNAPSHOT-<sha>_darwin_amd64_v1`); the SC wording is about which binary is verified, not about the literal snapshot path. Glob is robust across both snapshot and tag-push layouts.
- **Three-layer gate per verify step (Task 1):** `set -euo pipefail` (bash strict mode) + explicit `if [ -z "$BIN" ]` empty-binary check (gives an actionable error message before exit even though `set -e` would catch the downstream codesign failure) + `tee /tmp/codesign_<arch>.log` for CI-log visibility AND grep access + `grep -q "satisfies its Designated Requirement"` as the explicit SC-wording gate (SC1/SC2 specifies the output string, not just exit code).
- **`codesign -vvv` (not `--verify` and not fewer v's) (Task 1):** SC wording specifies `codesign -vvv` exactly. `codesign --verify` is a deprecated alias. `-vvv` (three v's) is verbose-verify mode and is what produces the "satisfies its Designated Requirement" line. `-vv` would suppress it.
- **SC3 "Release notes AND install guide" interpreted as three files (Task 2):** the conjunction is binding; the release-notes side has two artifacts (canonical user-facing changelog at `kinder-site/src/content/docs/changelog.md` AND planning-side draft at `.planning/release-notes-v2.4-draft.md`, which explicitly declares the changelog as its source of truth). Inserting the SC3 string into only installation.md OR only changelog.md would fail the conjunction. All three insertions use the verbatim backtick-wrapped form; `grep -cF` returns exactly 1 per file.
- **Starlight `:::caution` not `:::warning` or `:::tip` (Task 2):** `:::caution` matches the severity (not a critical hard-fail like `:::danger`, but more attention-grabbing than `:::note`) and matches the existing `:::note` admonition pattern at the top of installation.md.
- **PROJECT.md row wording (Task 3):** Decision column reads as a short comparative ("ad-hoc, not notarization") mirroring the `Headlamp over kubernetes/dashboard` style; Rationale column captures three dimensions (technical why-now: AMFI on Apple Silicon; trade-off: zero certificate overhead; deferral pointer: DIST-03). Footer date 2026-05-12 matches the project's current date.

## SC Status After This Plan

| SC | Status | Evidence |
|----|--------|----------|
| SC1 | DELIVERED structurally; awaits first CI run | `.github/workflows/macos-sign-verify.yml` verify-amd64 step runs `codesign -vvv` + `grep -q "satisfies its Designated Requirement"` on `dist/kinder_*_darwin_amd64_v1/kinder` after `goreleaser release --snapshot --clean --skip=publish` on macos-latest. First triggering push (or workflow_dispatch) will produce the authoritative CI log. |
| SC2 | DELIVERED structurally; awaits first CI run | Same workflow's verify-arm64 step, gated identically on `dist/kinder_*_darwin_arm64/kinder`. Both architectures verified independently per SC2 wording. |
| SC3 | DELIVERED in full | All THREE doc files (`kinder-site/src/content/docs/installation.md`, `kinder-site/src/content/docs/changelog.md`, `.planning/release-notes-v2.4-draft.md`) carry the verbatim backtick-wrapped SC3 wording exactly once per file. `grep -F` for both the full SC3 string AND the backtick-wrapped `xattr` form succeeds on every file. SC3 binding `Release notes AND install guide` conjunction is fully covered. |
| SC4 | DELIVERED in Plan 54-01 | Plan 54-02 explicitly does NOT modify `.goreleaser.yaml` — verified via `git diff --stat HEAD~3 HEAD -- .goreleaser.yaml` returning empty. The hook-as-last-builds-key invariant established in 54-01 is intact. |

The SC3 "Release notes AND install guide" binding conjunction is FULLY COVERED — the verifier's 3-file × 2-grep gate (literal SC3 string + backtick-wrapped `xattr` form) passes for all three files.

## Deviations from Plan

None — plan executed exactly as written. All three task verify blocks passed on first run. The plan's overall verification block (YAML validity + 6-gate SC3 grep + PROJECT.md row + SC4 untouched) all PASS. No auto-fixes triggered (Rules 1-3), no architectural decisions needed (Rule 4), no authentication gates encountered.

## Authentication Gates

None — pure file-creation and Markdown edits. No external services, no env vars, no secrets touched.

## Issues Encountered

None.

## User Setup Required

None for landing this plan. The first `gh workflow run macos-sign-verify.yml --ref main` (or the next push to main that touches `.goreleaser.yaml` or `.github/workflows/macos-sign-verify.yml`) will trigger the workflow and produce the authoritative SC1+SC2 CI log evidence. The workflow is self-contained and uses only `${{ secrets.GITHUB_TOKEN }}` (auto-provided) — no PAT, no Homebrew secret, no certificate.

## Verification Pointer for Phase-54 Verifier

- **SC1 evidence:** First green run of `.github/workflows/macos-sign-verify.yml` job `snapshot-sign-verify` step `Verify darwin/amd64 ad-hoc signature (SC1)` — the CI log will show the literal `Verifying: dist/.../kinder` line plus the `satisfies its Designated Requirement` line.
- **SC2 evidence:** Same run's `Verify darwin/arm64 ad-hoc signature (SC2)` step — identical structure on the arm64 binary.
- **SC3 evidence:** `grep -F 'ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires \`xattr -d com.apple.quarantine\`'` returns a hit in EACH of: `kinder-site/src/content/docs/installation.md`, `kinder-site/src/content/docs/changelog.md`, `.planning/release-notes-v2.4-draft.md`. `grep -cF` returns exactly `1` in each file (exactly-once cardinality).
- **SC4 evidence:** Already delivered in Plan 54-01. `.goreleaser.yaml` shows `builds[0].hooks.post` as the last key of the builds entry; no top-level `signs:` key; no post-sign strip/UPX. Confirmed in 54-01-SUMMARY.md `## SC4 Invariant Confirmation` section.

## Next Phase Readiness

- **Phase 54 complete (pending first CI run):** Both plans (54-01 + 54-02) are landed. The Phase 54 verifier can now collect SC1+SC2 evidence on the next triggering push or via `gh workflow run macos-sign-verify.yml --ref main`. SC3 + SC4 are evidence-complete at file/structure level.
- **DIST-01 requirement (Phase 54's only requirement):** Now fully evidenced. SC1+SC2 (CI gate) + SC3 (3-file disclosure) + SC4 (sign-as-last-op invariant) all delivered.
- **Phase 55 (Windows PR-CI Build Step) unblocked:** Phase 55 has no code dependency on Phase 54; both are CI-only changes targeting different platforms. Either can land next per the roadmap's `Phases 54 and 55 follow Phase 52` ordering rationale.
- **No carry-forward UAT:** Phase 54 is pure CI/docs plumbing. There is no live UAT requirement that defers to Phase 58 — the CI run IS the UAT for a signing pipeline.

## Self-Check: PASSED

- `.planning/phases/54-macos-ad-hoc-code-signing/54-02-SUMMARY.md` — exists (this file)
- `.github/workflows/macos-sign-verify.yml` — exists; YAML-valid; contains two `grep -q "satisfies its Designated Requirement"` checks; `find dist/` glob patterns present; `set -euo pipefail` in both verify steps; `workflow_dispatch` present; no `pull_request:` trigger
- `kinder-site/src/content/docs/installation.md` — contains the verbatim backtick-wrapped SC3 string inside a `:::caution[macOS direct download]` block; appears exactly once
- `kinder-site/src/content/docs/changelog.md` — contains the verbatim backtick-wrapped SC3 string inside a new `### macOS Ad-Hoc Signing (Phase 54)` subsection of `## v2.4 — Hardening`; appears exactly once
- `.planning/release-notes-v2.4-draft.md` — contains the verbatim backtick-wrapped SC3 string inside a new `## macOS Ad-Hoc Signing (Phase 54)` section between Internal Changes and Verification; appears exactly once
- `.planning/PROJECT.md` — contains the new `Ad-hoc codesign on macos-latest, not notarization` Key Decisions row; `Last updated` footer is `2026-05-12 — Phase 54 …`; old `2026-05-09` footer is absent
- Commit `693894be` (Task 1: macos-sign-verify.yml) — in `git log`
- Commit `78d5fdb1` (Task 2: SC3 3-file insertion) — in `git log`
- Commit `f078893e` (Task 3: PROJECT.md Key Decisions row) — in `git log`
- `.goreleaser.yaml` NOT modified in Plan 54-02 commits — `git diff --stat HEAD~3 HEAD -- .goreleaser.yaml` returns empty
- `.github/workflows/release.yml` NOT modified in Plan 54-02 commits — `git diff --stat HEAD~3 HEAD -- .github/workflows/release.yml` returns empty

---
*Phase: 54-macos-ad-hoc-code-signing*
*Plan: 54-02*
*Completed: 2026-05-12*
