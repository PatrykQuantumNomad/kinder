---
phase: 54-macos-ad-hoc-code-signing
verified: 2026-05-12T16:08:00Z
status: passed
score: 4/4 must-haves verified (SC1 + SC2 + SC3 + SC4 all green)
overrides_applied: 0
re_verification: 2026-05-12T16:08:00Z  # SC1+SC2 reverified after CI green run 25746519788
gaps: []
deferred: []
ci_evidence:
  workflow: macos-sign-verify.yml
  run_id: 25746519788
  conclusion: success
  duration: 2m37s
  triggered_by: push (commit d00cafd0; paths filter caught .goreleaser.yaml + macos-sign-verify.yml)
  sc1_log_line: "dist/kinder_darwin_amd64_v1/kinder: satisfies its Designated Requirement"
  sc2_log_line: "dist/kinder_darwin_arm64_v8.0/kinder: satisfies its Designated Requirement"
  notes:
    - "arm64 dist path includes _v8.0 GOARM64 suffix (Go 1.23+ default) — workflow's *darwin_arm64* path glob handles both legacy and v8.0 layouts; SC2 intent (arm64 signature verified) satisfied."
---

# Phase 54: macOS Ad-Hoc Code Signing — Verification Report

**Phase Goal:** darwin/amd64 and darwin/arm64 GoReleaser artifacts are ad-hoc signed so Apple Silicon AMFI no longer kills the binary on first run
**Verified:** 2026-05-12 (initial codebase audit) + 2026-05-12T16:08Z (CI re-verify after push)
**Status:** passed
**Re-verification:** Yes — SC1 + SC2 re-verified after `git push origin main` triggered workflow run `25746519788` (success, 2m37s). Both ad-hoc signatures printed `satisfies its Designated Requirement` in the live CI log.

## Goal Achievement

### Observable Truths (mapped to ROADMAP Success Criteria)

| #   | Truth (Success Criterion)                                                                                                                                                     | Status     | Evidence                                                                                                                                                                                                                                                                                                                                                          |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| SC1 | `codesign -vvv dist/kinder_darwin_amd64_v1/kinder` returns `satisfies its Designated Requirement` in CI after a snapshot build                                                | ✓ VERIFIED | CI run `25746519788` step "Verify darwin/amd64 ad-hoc signature (SC1)" printed `dist/kinder_darwin_amd64_v1/kinder: satisfies its Designated Requirement` (run log 2026-05-12T16:05:34Z) and exited 0. Step uses `set -euo pipefail` + `grep -q` gate at `.github/workflows/macos-sign-verify.yml:40-51`. |
| SC2 | `codesign -vvv dist/kinder_darwin_arm64/kinder` returns `satisfies its Designated Requirement` in CI after a snapshot build (both architectures verified independently)        | ✓ VERIFIED | CI run `25746519788` step "Verify darwin/arm64 ad-hoc signature (SC2)" printed `dist/kinder_darwin_arm64_v8.0/kinder: satisfies its Designated Requirement` (run log 2026-05-12T16:05:34Z) and exited 0. Arm64 dist path includes Go 1.23+ default `_v8.0` GOARM64 suffix; workflow's `*darwin_arm64*` path glob (`.github/workflows/macos-sign-verify.yml:53-64`) handles both layouts. Independent step satisfies SC2's "both architectures verified independently" clause. |
| SC3 | Release notes AND install guide explicitly state: "ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`"      | ✓ VERIFIED | Literal `grep -F` of the exact backtick-wrapped string matched in all three required surfaces: `kinder-site/src/content/docs/installation.md:40`, `kinder-site/src/content/docs/changelog.md:54`, and `.planning/release-notes-v2.4-draft.md:54`. The "and" conjunction (release notes + install guide) is satisfied; the release-notes side has both canonical (changelog.md) and planning-side draft mirrored. |
| SC4 | The sign step is the LAST operation on each binary before archiving — no post-sign strip, UPX, or binary copy invalidates the Mach-O signature block                          | ✓ VERIFIED | YAML structural parse via Python yaml.safe_load: `builds[0]` keys in declaration order are `['id','main','binary','env','goos','goarch','ignore','flags','ldflags','mod_timestamp','hooks']` — `hooks` is the LAST key. `signs` key absent at top level. No post-sign step exists. Codesign cmd at `.goreleaser.yaml:46` runs in builds[0].hooks.post.            |

**Score:** 4/4 truths VERIFIED. SC1 + SC2 closed by live CI run `25746519788` (success, 2m37s) after the 2026-05-12T16:03Z `git push origin main` paths-filter trigger.

### Required Artifacts

All three verification levels (exists, substantive, wired) checked. Level 4 data-flow trace is not applicable for build/config/docs files.

| Artifact                                                | Expected                                                                          | Status     | Details                                                                                                                                                                                                                                                                                            |
| ------------------------------------------------------- | --------------------------------------------------------------------------------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `.goreleaser.yaml`                                      | Darwin-gated codesign post-hook on `builds[0].hooks.post`; `-s` flag in ldflags   | ✓ VERIFIED | Line 46: `cmd: sh -c 'if [ "{{ .Os }}" = "darwin" ]; then codesign --force --sign - "{{ .Path }}"; fi'`. Line 40: `-s` present alongside `-w`. `hooks` confirmed last key in `builds[0]`. No top-level `signs:` key.                                                                              |
| `.github/workflows/release.yml`                         | `runs-on: macos-latest` (so `codesign` is on PATH)                                | ✓ VERIFIED | Line 13: `runs-on: macos-latest`.                                                                                                                                                                                                                                                                  |
| `.github/workflows/macos-sign-verify.yml` (NEW)         | Snapshot build + 2 `codesign -vvv` verify steps with `grep -q` gate              | ✓ VERIFIED | File at commit `693894be`; structurally correct (snapshot build step lines 31-38, amd64 verify lines 40-51, arm64 verify lines 53-64, `set -euo pipefail` + `grep -q "satisfies its Designated Requirement"` for both arches). Pushed to `origin/main` (commit `d00cafd0`) at 2026-05-12T16:03Z; CI run `25746519788` succeeded in 2m37s with both verify steps green. |
| `kinder-site/src/content/docs/installation.md`          | SC3 wording in `:::caution[macOS direct download]` block                          | ✓ VERIFIED | Line 39-49: caution block contains the literal SC3 string at line 40, plus actionable `xattr -d com.apple.quarantine kinder` example at line 45.                                                                                                                                                  |
| `kinder-site/src/content/docs/changelog.md`             | SC3 wording in new `### macOS Ad-Hoc Signing (Phase 54)` subsection               | ✓ VERIFIED | Line 52 introduces subsection header; line 54 carries the literal SC3 string. Includes DIST-01 attribution and DIST-03 deferral pointer (line 58).                                                                                                                                                |
| `.planning/release-notes-v2.4-draft.md`                 | SC3 wording in new `## macOS Ad-Hoc Signing (Phase 54)` section                   | ✓ VERIFIED | Line 52 section header; line 54 literal SC3 string; `xattr` worked example lines 60-62; DIST-01/DIST-03 attribution line 66.                                                                                                                                                                       |
| `.planning/PROJECT.md`                                  | Key Decisions row recording ad-hoc signing decision                               | ✓ VERIFIED | Line 225: row reads "Ad-hoc codesign on macos-latest, not notarization | AMFI on Apple Silicon kills unsigned arm64 binaries; ad-hoc = hash-only signature satisfies AMFI with zero certificate/cost overhead; full Developer ID + notarization deferred (DIST-03) | ✓ Good". Date stamp at line 228 confirms 2026-05-12 Phase 54 recording. |

### Key Link Verification

| From                                                | To                                       | Via                                              | Status     | Details                                                                                                                                                                                                                          |
| --------------------------------------------------- | ---------------------------------------- | ------------------------------------------------ | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `.goreleaser.yaml builds[0].hooks.post`             | `codesign --force --sign -`              | `sh -c` conditional on `{{ .Os }} == darwin`     | ✓ WIRED    | `.goreleaser.yaml:46` matches the expected pattern character-for-character: `codesign --force --sign - "{{ .Path }}"` under `sh -c 'if [ "{{ .Os }}" = "darwin" ]; then ...; fi'`. Linux/Windows are gated OFF.                |
| `.github/workflows/release.yml jobs.release`        | `macos-latest` runner                    | `runs-on` declaration                            | ✓ WIRED    | `.github/workflows/release.yml:13` — `runs-on: macos-latest` directly in `jobs.release`.                                                                                                                                          |
| `.github/workflows/macos-sign-verify.yml`           | `dist/kinder_darwin_amd64_v1/kinder`     | `find dist/ -path '*darwin_amd64*'` → `codesign -vvv` → `grep -q` gate | ✓ WIRED (structure) | Lines 40-51 chain `find` → assignment-check → `codesign -vvv` (with `tee /tmp/codesign_amd64.log`) → `grep -q "satisfies its Designated Requirement"`. `set -euo pipefail` ensures non-zero exit on grep miss.   |
| `.github/workflows/macos-sign-verify.yml`           | `dist/kinder_darwin_arm64/kinder`        | `find dist/ -path '*darwin_arm64*'` → `codesign -vvv` → `grep -q` gate | ✓ WIRED (structure) | Lines 53-64 mirror the amd64 step against the arm64 path with `/tmp/codesign_arm64.log`. Independent step satisfies "both architectures verified independently" (SC2 parenthetical).                                |

### Data-Flow Trace (Level 4)

Not applicable for this phase — outputs are build artifacts produced at GoReleaser snapshot/release time and consumed by `codesign`, not dynamic data rendered by a component. The runtime "data flow" is: `go build` → Mach-O binary → `codesign --force --sign -` → `tar` archive. This flow is verified structurally via SC4 (hooks.post position) and operationally via SC1+SC2 (human-verified CI run).

### Behavioral Spot-Checks

| Behavior                                                          | Command                                                                                                              | Result                                                                              | Status   |
| ----------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- | -------- |
| `.goreleaser.yaml` is valid YAML                                  | `python3 -c "import yaml; yaml.safe_load(open('.goreleaser.yaml'))"`                                                 | Parsed successfully                                                                 | ✓ PASS   |
| `hooks` is the LAST key in `builds[0]` (SC4 structural)           | Python: `list(data['builds'][0].keys())[-1]`                                                                          | `'hooks'`                                                                            | ✓ PASS   |
| No top-level `signs:` key (SC4 — no separate signing stage)      | Python: `'signs' in data`                                                                                            | `False`                                                                              | ✓ PASS   |
| `-s` strip flag in ldflags (DIST-01 wording)                      | Python: `'-s' in data['builds'][0]['ldflags']`                                                                       | `True` (alongside `-w`)                                                              | ✓ PASS   |
| `codesign --force --sign -` exact form present                    | `grep "codesign --force --sign -" .goreleaser.yaml`                                                                  | Line 46 match                                                                       | ✓ PASS   |
| Darwin OS gate is shell-conditional, not the unsupported `if:`    | `grep 'if \[ "{{ .Os }}" = "darwin" \]'` in `.goreleaser.yaml`                                                       | Line 46 match                                                                       | ✓ PASS   |
| SC3 literal in `installation.md`                                  | `grep -F` literal backtick-wrapped form                                                                              | `installation.md:40` match                                                          | ✓ PASS   |
| SC3 literal in `changelog.md`                                     | `grep -F` literal backtick-wrapped form                                                                              | `changelog.md:54` match                                                             | ✓ PASS   |
| SC3 literal in `release-notes-v2.4-draft.md`                      | `grep -F` literal backtick-wrapped form                                                                              | `release-notes-v2.4-draft.md:54` match                                              | ✓ PASS   |
| Actual CI run of `macos-sign-verify.yml` has succeeded            | `gh run watch 25746519788 --exit-status`                                                                              | Conclusion `success` in 2m37s; both verify steps printed `satisfies its Designated Requirement` for amd64 + arm64 binaries (timestamp 2026-05-12T16:05:34Z) | ✓ PASS   |

### Probe Execution

Not applicable — no `scripts/*/tests/probe-*.sh` declared or referenced by the Phase 54 PLAN/SUMMARY documents. The closest analog is the CI verify steps inside `macos-sign-verify.yml`, which is treated as a CI gate (and folded into the human_verification routing for SC1+SC2).

### Requirements Coverage

| Requirement | Source Plan       | Description                                                                                                                                                                                                                       | Status     | Evidence                                                                                                                                                                                                                  |
| ----------- | ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| DIST-01     | 54-01-PLAN, 54-02-PLAN | "macOS GoReleaser artifacts ad-hoc signed via `codesign --force --sign -` post-hook running on `macos-latest` CI runner; signing happens AFTER `-ldflags=\"-s -w\"` strip; release notes explicitly state ad-hoc signing does NOT bypass Gatekeeper quarantine" | ✓ SATISFIED | Codebase delivers every concrete clause: post-hook exact form (.goreleaser.yaml:46), `macos-latest` runner (release.yml:13), `-s -w` strip in ldflags (lines 40-41), explicit "does NOT bypass Gatekeeper" language inside SC3 wording surfaces (installation.md:42, changelog.md:54-58, release-notes-v2.4-draft.md:54-66). CI run `25746519788` green-confirms both signatures in `dist/`. REQUIREMENTS.md line 28 marks DIST-01 `[x]`. |

No orphaned requirements: REQUIREMENTS.md `## Phase ↔ Requirement Map` (line 104) maps only DIST-01 to Phase 54, and both plans (54-01, 54-02) claim DIST-01 in their frontmatter.

### Anti-Patterns Found

| File                                                | Line | Pattern | Severity | Impact                                                                                                                                                                                                                                            |
| --------------------------------------------------- | ---- | ------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `kinder-site/src/content/docs/changelog.md`         | 16   | `TBD`   | ℹ️ Info  | `**Released:** TBD (finalize when phase 58 ships v2.4)` — the TBD on the same line explicitly references formal follow-up work ("phase 58"), so it passes the debt-marker gate. This is the milestone-level release-date placeholder, not Phase 54 debt. |

No `FIXME`, `XXX`, `HACK`, `PLACEHOLDER`, "not yet implemented", "coming soon", or stub `return null` / hardcoded-empty constructs in any Phase 54 file.

### Human Verification — CLOSED

Originally routed to human verification on 2026-05-12T00:00Z because the workflow file was committed locally but unpushed. Resolved same day at 2026-05-12T16:08Z:

1. `git push origin main` (commits `f1df8c88`..`d00cafd0`) auto-triggered `macos-sign-verify.yml` via the paths filter on `.goreleaser.yaml` + the workflow file itself.
2. Run id `25746519788` completed with conclusion **success** in 2m37s.
3. Log evidence (`gh run view 25746519788 --log`):
   - amd64: `dist/kinder_darwin_amd64_v1/kinder: satisfies its Designated Requirement`
   - arm64: `dist/kinder_darwin_arm64_v8.0/kinder: satisfies its Designated Requirement`

SC1 + SC2 promoted from `? UNCERTAIN (human)` to `✓ VERIFIED`. No outstanding human verification items remain.

### Gaps Summary

No gaps. All four must-haves verified:
- SC1 + SC2: live CI run `25746519788` printed the exact "satisfies its Designated Requirement" string for both architectures and exited 0.
- SC3 literal string present in all three required surfaces (install guide + canonical changelog + release-notes draft).
- SC4 structural invariant holds: `hooks` is the last key in `builds[0]`; no post-sign step exists; no top-level `signs:` key.
- All seven required artifacts exist, are substantive, and are wired per their plan-declared `key_links` patterns.
- DIST-01 fully satisfied (codebase + CI evidence).

---

*Verified: 2026-05-12 (initial codebase audit) + 2026-05-12T16:08Z (CI re-verify, status promoted to passed)*
*Verifier: Claude (gsd-verifier) + Claude (execute-phase orchestrator, post-CI promotion)*
