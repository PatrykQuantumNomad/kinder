---
phase: 54
phase_name: "macOS Ad-Hoc Code Signing"
nyquist_validation: enabled
source: "Extracted from 54-RESEARCH.md §Validation Architecture (lines 694-723) + per-plan must_haves"
generated: 2026-05-12
---

# Phase 54 — Validation Plan

> Test map for the macOS ad-hoc code signing phase. Phase 54 is CI-config + docs only. No Go test framework applies. Validation is via GitHub Actions workflow execution (`.github/workflows/macos-sign-verify.yml`) plus shell `grep` gates over `.goreleaser.yaml` and the three SC3-bearing doc files.

**SC ownership (binding, from ROADMAP.md):**

| SC  | Owning Plan | Validation Vector |
|-----|-------------|-------------------|
| SC1 | 54-02       | CI `codesign -vvv` over darwin/amd64 snapshot binary |
| SC2 | 54-02       | CI `codesign -vvv` over darwin/arm64 snapshot binary |
| SC3 | 54-02       | Shell `grep -F` over installation.md + changelog.md + release-notes-v2.4-draft.md (all three must contain SC3 wording with backtick-wrapped `xattr` command) |
| SC4 | 54-01       | Structural grep of `.goreleaser.yaml` — `hooks.post` exists on `builds[]`, no top-level `signs:` key, hooks block is the last key inside `builds[0]` (architectural ordering guarantee via GoReleaser pipeline) |

---

## Test Framework

| Property | Value |
|----------|-------|
| Framework | shell + GoReleaser snapshot + `codesign` (no Go test framework — phase is CI-config + docs) |
| Config file | `.github/workflows/macos-sign-verify.yml` (new in 54-02) |
| Quick run command (per SC) | See "Phase Requirements → Test Map" below |
| Full suite command | `gh workflow run macos-sign-verify.yml --ref main` then poll `gh run list --workflow=macos-sign-verify.yml --limit 1 --json conclusion` until non-empty |
| Build gate (local smoke) | `goreleaser release --snapshot --clean --skip=publish` then `codesign -vvv $(find dist/ -name kinder -path '*darwin*')` |
| Phase gate | All SC1–SC4 grep / codesign commands return 0 AND `macos-sign-verify.yml` runs green on a main-branch push that touches `.goreleaser.yaml` |

---

## Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | Owning Plan | File Exists? |
|--------|----------|-----------|-------------------|-------------|--------------|
| DIST-01 (SC1) | darwin/amd64 snapshot binary returns "satisfies its Designated Requirement" under `codesign -vvv` in CI | integration (shell in CI) | `set -euo pipefail; BIN=$(find dist/ -name kinder -path '*darwin_amd64*' \| head -1); [ -n "$BIN" ] \|\| { echo FAIL; exit 1; }; codesign -vvv "$BIN" 2>&1 \| tee /tmp/codesign_amd64.log; grep -q "satisfies its Designated Requirement" /tmp/codesign_amd64.log` | 54-02 (Task 1) | ❌ Wave 0 (file `.github/workflows/macos-sign-verify.yml` created by 54-02 Task 1) |
| DIST-01 (SC2) | darwin/arm64 snapshot binary returns "satisfies its Designated Requirement" under `codesign -vvv` in CI (both architectures verified independently) | integration (shell in CI) | `set -euo pipefail; BIN=$(find dist/ -name kinder -path '*darwin_arm64*' \| head -1); [ -n "$BIN" ] \|\| { echo FAIL; exit 1; }; codesign -vvv "$BIN" 2>&1 \| tee /tmp/codesign_arm64.log; grep -q "satisfies its Designated Requirement" /tmp/codesign_arm64.log` | 54-02 (Task 1) | ❌ Wave 0 (same workflow file) |
| DIST-01 (SC3) | install guide AND release notes AND changelog all contain the SC3 wording with backtick-wrapped `xattr` command | structural (shell grep) | `grep -F 'ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`' kinder-site/src/content/docs/installation.md && grep -F 'ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`' kinder-site/src/content/docs/changelog.md && grep -F 'ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`' .planning/release-notes-v2.4-draft.md` | 54-02 (Task 2) | ❌ Wave 0 (insertion in all three files by Task 2) |
| DIST-01 (SC4) | sign step is on `builds[].hooks.post` (NOT `signs:`, NOT `archives:`) | structural grep | `grep -q "^    hooks:$" .goreleaser.yaml && grep -q "^      post:$" .goreleaser.yaml && ! grep -q "^signs:" .goreleaser.yaml` | 54-01 (Task 2) | ❌ Wave 0 (hooks block added to `.goreleaser.yaml` builds[0] by 54-01 Task 2) |
| DIST-01 (hook order) | `-s -w` strip happens via Go build ldflags BEFORE the codesign hook runs | architectural (GoReleaser pipeline guarantee) | n/a — guaranteed by GoReleaser pipeline order (ldflags are passed to `go build`; `builds[].hooks.post` runs AFTER `go build` completes; no UPX/strip post-hook exists) | 54-01 (Tasks 1+2) | architectural |
| DIST-01 (ldflags ordering) | `-s` precedes `-w` in `.goreleaser.yaml` ldflags list (matches DIST-01 literal wording `-ldflags="-s -w"`) | structural awk | `awk '/^    ldflags:/{found=1} found && /- -s$/{s=NR} found && /- -w$/{w=NR} END{exit (s>0 && w>0 && s<w) ? 0 : 1}' .goreleaser.yaml` | 54-01 (Task 1) | ❌ Wave 0 |

---

## Wave 0 Gaps (artifacts created during execution)

These are file-creation responsibilities of each plan, not pre-Wave-0 work. The validation commands above reference them; they ship in the named owning plan's commits.

- [ ] `.github/workflows/macos-sign-verify.yml` — covers SC1 and SC2 (Plan 54-02 Task 1)
- [ ] `.goreleaser.yaml` `builds[0].hooks.post` block — covers SC4 + hook order (Plan 54-01 Task 2)
- [ ] `.goreleaser.yaml` ldflags `-s` insertion (before `-w`) — covers DIST-01 literal wording (Plan 54-01 Task 1)
- [ ] `.github/workflows/release.yml` `runs-on: macos-latest` switch — prerequisite for SC1/SC2 (Plan 54-01 Task 3)
- [ ] `kinder-site/src/content/docs/installation.md` `:::caution[macOS direct download]` block — covers SC3 install-guide half (Plan 54-02 Task 2)
- [ ] `kinder-site/src/content/docs/changelog.md` `## v2.4 — Hardening` → SC3 subsection — covers SC3 changelog half (Plan 54-02 Task 2)
- [ ] `.planning/release-notes-v2.4-draft.md` SC3 disclosure block — covers SC3 release-notes half (Plan 54-02 Task 2)

---

## Manual UATs (Cannot Be Automated at Plan Time)

Phase 54 has no manual UAT — verification is fully automatable via the CI workflow + grep gates. The "human-optional" smoke tests called out in 54-01 and 54-02 (`goreleaser release --snapshot --clean --skip=publish` locally + Quarantine-attribute test on a downloaded release binary) are confidence checks, not phase-gate requirements; the authoritative verification is the green CI run.

---

## Phase-Wide Acceptance Tests

| Acceptance Truth | Test Type | Harness | Automated Command |
|------------------|-----------|---------|-------------------|
| `macos-sign-verify.yml` runs green on a main-branch push that touches `.goreleaser.yaml` | integration | GitHub Actions | `gh run list --workflow=macos-sign-verify.yml --limit 1 --json conclusion --jq '.[0].conclusion == "success"'` |
| All three SC3-bearing doc files contain the exact SC3 wording with backtick-wrapped `xattr` | structural | shell grep | `for f in kinder-site/src/content/docs/installation.md kinder-site/src/content/docs/changelog.md .planning/release-notes-v2.4-draft.md; do grep -F 'requires `xattr -d com.apple.quarantine`' "$f" \|\| exit 1; done` |
| `.goreleaser.yaml` has no top-level `signs:` key (SC4 boundary: ad-hoc Mach-O signing lives on `builds[]`, not on archives) | structural | shell grep | `! grep -q "^signs:" .goreleaser.yaml` |
| `.github/workflows/release.yml` uses `macos-latest` runner (codesign availability prerequisite) | structural | shell grep | `grep -q "^    runs-on: macos-latest$" .github/workflows/release.yml && ! grep -q "ubuntu-latest" .github/workflows/release.yml` |
| Requirement coverage: every plan's `requirements:` field lists DIST-01 | static | frontmatter validation | `for f in .planning/phases/54-macos-ad-hoc-code-signing/54-0?-PLAN.md; do gsd-sdk query frontmatter.validate "$f" --schema plan; done` |

---

## Coverage Audit

Every truth in each plan's `must_haves` block has a row in this document:

- **54-01 must_haves.truths** (6 items): rows cover ldflags `-s` insertion (DIST-01 ldflags ordering), darwin codesign on amd64/arm64 (SC1+SC2 via 54-02 verification path), linux/windows unaffected (architectural shell-conditional guarantee), sign-as-last-op (SC4), macOS runner switch (DIST-01 hook order prerequisite).
- **54-02 must_haves.truths** (5 items): rows cover SC1+SC2 codesign verification, SC3 wording in all three doc files (install.md + changelog.md + release-notes-v2.4-draft.md), CI gate (`set -euo pipefail` + `grep -q` non-zero exit on missing signature), PROJECT.md decision row (architectural — not a phase SC but plan-internal must_have).

Every key link has at least one row that exercises it:

- `.goreleaser.yaml builds[].hooks.post → codesign --force --sign -` → SC4 structural grep + SC1/SC2 CI codesign run (the hook produces the signed binary that SC1/SC2 verify).
- `.github/workflows/release.yml jobs.release → macos-latest runner` → release.yml runner grep gate.
- `.github/workflows/macos-sign-verify.yml → dist/kinder_darwin_{amd64,arm64}*/kinder` → SC1 and SC2 rows (the workflow IS the test harness).
- `kinder-site/src/content/docs/installation.md → Homebrew install path` → SC3 row's three-file grep (Homebrew-unaffected clause is part of the SC3 string).

If during execution any plan adds a truth or removes one, this VALIDATION.md MUST be updated in the same commit so the map stays in sync.

---

## Phase Gate

Phase 54 is COMPLETE when all of the following return 0:

1. SC1 row command (codesign on darwin/amd64 in CI) — green
2. SC2 row command (codesign on darwin/arm64 in CI) — green
3. SC3 row command (3-file `grep -F` for backtick-wrapped `xattr` SC3 string) — exit 0
4. SC4 row command (`.goreleaser.yaml` structural grep) — exit 0
5. `macos-sign-verify.yml` workflow has run green at least once on main branch push (proves end-to-end signing → verification chain)

If any of (1)–(5) fail, the phase is held and the gap is filed against the owning plan.
