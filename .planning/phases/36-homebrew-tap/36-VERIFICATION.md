---
phase: 36-homebrew-tap
verified: 2026-03-04T23:50:00Z
status: human_needed
score: 3/5 must-haves verified
human_verification:
  - test: "brew install patrykquantumnomad/kinder/kinder succeeds on macOS"
    expected: "A working kinder binary is installed — `kinder version` returns a version string"
    why_human: "No tagged release has been pushed yet. The homebrew-kinder tap exists but has no Casks/kinder.rb file (GoReleaser creates it on the first v* tag push). Cannot verify until a release is cut."
  - test: "Casks/kinder.rb is auto-updated in homebrew-kinder after a tagged release"
    expected: "After pushing a v* tag, `gh api repos/PatrykQuantumNomad/homebrew-kinder/commits --jq '.[0].commit.message'` returns a message like 'Brew cask update for kinder version vX.Y.Z'"
    why_human: "No tagged release has been pushed yet. The infrastructure (GoReleaser config, HOMEBREW_TAP_TOKEN secret, tap repo) is all wired — end-to-end can only be verified after the first release."
  - test: "Installation page visual verification"
    expected: "Homebrew (macOS) section appears before Download Pre-built Binary when visiting the live site or local dev server at /installation/"
    why_human: "Content verified programmatically but visual rendering and UX ordering requires human confirmation in a browser."
---

# Phase 36: Homebrew Tap Verification Report

**Phase Goal:** macOS users can install kinder with a single `brew install` command from a maintained custom tap, without needing a Go toolchain
**Verified:** 2026-03-04T23:50:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                       | Status       | Evidence                                                                                      |
|----|--------------------------------------------------------------------------------------------|--------------|-----------------------------------------------------------------------------------------------|
| 1  | homebrew-kinder tap repository exists on GitHub under PatrykQuantumNomad with a Casks/ dir | PARTIAL      | Repo exists (public, PatrykQuantumNomad/homebrew-kinder, correct description). Casks/ dir absent — expected until first release pushes Casks/kinder.rb |
| 2  | GoReleaser config has homebrew_casks section targeting homebrew-kinder repo                 | VERIFIED     | .goreleaser.yaml lines 103-117: homebrew_casks with correct owner/name/branch/token/directory/skip_upload |
| 3  | release.yml passes HOMEBREW_TAP_TOKEN to goreleaser-action for cross-repo push auth        | VERIFIED     | .github/workflows/release.yml line 31: `HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}` with inline comment |
| 4  | Installation page shows `brew install patrykquantumnomad/kinder/kinder` as first method    | VERIFIED     | kinder-site/src/content/docs/installation.md line 15-19: Homebrew (macOS) section precedes Download Pre-built Binary; kinder-site builds cleanly |
| 5  | `brew install patrykquantumnomad/kinder/kinder` actually works on macOS                    | HUMAN NEEDED | No tagged release pushed yet — Casks/kinder.rb does not exist in tap repo. Infrastructure verified; end-to-end requires a v* tag push |

**Score:** 3/5 truths verified (Truth 1 is partial — repo exists, Casks/ dir absent by design pre-release; Truth 5 is human-needed)

### Required Artifacts

| Artifact                                          | Expected                                                      | Status      | Details                                                                                                                         |
|---------------------------------------------------|---------------------------------------------------------------|-------------|---------------------------------------------------------------------------------------------------------------------------------|
| `.goreleaser.yaml`                                | homebrew_casks section targeting PatrykQuantumNomad/homebrew-kinder | VERIFIED | Lines 103-117: `homebrew_casks:`, `directory: Casks`, `skip_upload: auto`, `owner: PatrykQuantumNomad`, `name: homebrew-kinder`, `branch: main`, `token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"` |
| `.github/workflows/release.yml`                   | HOMEBREW_TAP_TOKEN env var for goreleaser-action             | VERIFIED    | Line 31: `HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}  # PAT with Contents:write on homebrew-kinder repo`           |
| `kinder-site/src/content/docs/installation.md`    | brew install command + no stale cross.sh URLs                | VERIFIED    | Line 18: `brew install patrykquantumnomad/kinder/kinder`; zero matches for old `kinder-darwin-arm64` URLs; site builds cleanly (19 pages, 1.99s) |
| `PatrykQuantumNomad/homebrew-kinder` (GitHub)     | Public tap repo with Casks/ directory                        | PARTIAL     | Repo is public with correct description. Only has initial README commit (96e9c04). No Casks/ directory yet — GoReleaser creates it on first v* tag. This is expected pre-release. |
| `HOMEBREW_TAP_TOKEN` repository secret            | Secret stored in kinder repo for CI use                      | VERIFIED    | `gh api repos/PatrykQuantumNomad/kinder/actions/secrets` confirms secret exists (created 2026-03-04T23:39:46Z) |

### Key Link Verification

| From                          | To                                    | Via                                        | Status   | Details                                                                                    |
|-------------------------------|---------------------------------------|--------------------------------------------|----------|--------------------------------------------------------------------------------------------|
| `.goreleaser.yaml`            | `PatrykQuantumNomad/homebrew-kinder`  | `homebrew_casks[].repository`              | WIRED    | `owner: PatrykQuantumNomad` and `name: homebrew-kinder` confirmed at lines 110-111         |
| `.github/workflows/release.yml` | `.goreleaser.yaml`                  | `HOMEBREW_TAP_TOKEN` env var consumed by GoReleaser template | WIRED | Line 31 passes `secrets.HOMEBREW_TAP_TOKEN` to env; goreleaser.yaml line 113 consumes it via `{{ .Env.HOMEBREW_TAP_TOKEN }}` |
| `kinder-site/src/content/docs/installation.md` | GitHub Releases | Download URLs in curl commands             | WIRED    | Line 26: links to `https://github.com/PatrykQuantumNomad/kinder/releases/latest`; line 34 uses `kinder_*_darwin_arm64.tar.gz` glob pattern matching GoReleaser naming |

### Requirements Coverage

| Requirement | Source Plan | Description                                                        | Status       | Evidence                                                                                                     |
|-------------|-------------|--------------------------------------------------------------------|--------------|--------------------------------------------------------------------------------------------------------------|
| BREW-01     | 36-01       | homebrew-kinder tap repository exists under PatrykQuantumNomad     | SATISFIED    | `gh repo view PatrykQuantumNomad/homebrew-kinder` returns public repo with correct description               |
| BREW-02     | 36-01       | GoReleaser publishes Cask to tap repo on tagged release via HOMEBREW_TAP_TOKEN | HUMAN NEEDED | Infrastructure fully wired (GoReleaser config, secret, tap repo). Cannot confirm end-to-end cask publication without a tagged release. |
| BREW-03     | 36-01       | User can install kinder via `brew install patrykquantumnomad/kinder/kinder` | HUMAN NEEDED | Depends on BREW-02 completing first (Cask must exist in tap repo before brew install works)                  |
| SITE-01     | 36-02       | Installation page updated with Homebrew install instructions alongside make install | SATISFIED | `brew install` command at line 18 of installation.md, Homebrew section precedes Download section; kinder-site builds cleanly |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | No TODOs, placeholders, empty handlers, or stale URLs detected |

### Human Verification Required

#### 1. brew install end-to-end test

**Test:** Push a v* tag to trigger the release workflow, then on macOS run `brew install patrykquantumnomad/kinder/kinder`
**Expected:** GoReleaser builds binaries, publishes a GitHub Release, pushes `Casks/kinder.rb` to `PatrykQuantumNomad/homebrew-kinder`, and `brew install` installs a working kinder binary. `kinder version` should return a version string.
**Why human:** No tagged release has been pushed. The entire pipeline (GoReleaser -> GitHub Release -> Cask push -> brew install) is only exercisable with a real v* tag. The infrastructure is verified in isolation but the chain cannot be confirmed programmatically.

#### 2. Auto-update verification after tagged release

**Test:** After pushing a v* tag, run `gh api repos/PatrykQuantumNomad/homebrew-kinder/commits --jq '.[0].commit.message'`
**Expected:** `"Brew cask update for kinder version vX.Y.Z"` (goreleaserbot commit)
**Why human:** Depends on first tagged release being pushed. Only the commit history of the tap repo can confirm GoReleaser successfully authenticated with HOMEBREW_TAP_TOKEN and pushed the Cask.

#### 3. Installation page visual review

**Test:** Run `cd /Users/patrykattc/work/git/kinder/kinder-site && npm run dev`, then visit http://localhost:4321/installation/
**Expected:** "Homebrew (macOS)" section with the `brew install` command appears at the top of install options, before "Download Pre-built Binary". "Build from Source" and "Verify Installation" sections are intact.
**Why human:** Content verified programmatically (grep, build check). Visual rendering, section ordering, and readability require human eyes in a browser.

### Gaps Summary

No blocking gaps — all automated infrastructure is wired and verified:

- GoReleaser config is syntactically valid (`goreleaser check` passes with zero errors) and structurally correct (homebrew_casks section with correct repo target, skip_upload:auto, directory:Casks)
- The HOMEBREW_TAP_TOKEN secret exists in the kinder repo and is passed through the release workflow to GoReleaser
- The homebrew-kinder tap repo exists as a public repo under PatrykQuantumNomad (as of 2026-03-04)
- The installation page is updated with Homebrew as the first macOS install method and the kinder-site builds cleanly

The remaining 2 human_needed items (brew install working, auto-update on release) are gated on the first v* tag push — a user action, not an implementation gap. This was explicitly anticipated in the phase plan.

SITE-01 is fully satisfied. BREW-01 is satisfied (tap repo exists). BREW-02 and BREW-03 are pending the first tagged release.

---

_Verified: 2026-03-04T23:50:00Z_
_Verifier: Claude (gsd-verifier)_
