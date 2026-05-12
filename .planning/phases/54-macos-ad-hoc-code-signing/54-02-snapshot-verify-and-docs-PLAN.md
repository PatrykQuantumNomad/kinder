---
phase: 54-macos-ad-hoc-code-signing
plan: 02
type: execute
wave: 2
status: pending
depends_on:
  - 54-01
files_modified:
  - .github/workflows/macos-sign-verify.yml
  - kinder-site/src/content/docs/installation.md
  - kinder-site/src/content/docs/changelog.md
  - .planning/release-notes-v2.4-draft.md
  - .planning/PROJECT.md
autonomous: true
requirements:
  - DIST-01

must_haves:
  truths:
    - "codesign -vvv dist/kinder_darwin_amd64_v1/kinder returns `satisfies its Designated Requirement` in CI after a snapshot build"
    - "codesign -vvv dist/kinder_darwin_arm64/kinder returns `satisfies its Designated Requirement` in CI after a snapshot build (both architectures verified independently)"
    - "Release notes and install guide explicitly state: \"ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`\""
    - "The snapshot-verify CI job FAILS (non-zero exit) if either codesign verification fails — verification is a CI gate, not advisory logging"
    - "PROJECT.md Key Decisions table records the ad-hoc signing decision for future-self lookup"
  artifacts:
    - path: ".github/workflows/macos-sign-verify.yml"
      provides: "Snapshot-build + codesign-verify CI job for SC1 and SC2"
      contains: "satisfies its Designated Requirement"
    - path: "kinder-site/src/content/docs/installation.md"
      provides: "SC3 wording: ad-hoc signed, not notarized, Homebrew unaffected, xattr instruction for direct download (install-guide half of SC3 'and')"
      contains: "xattr -d com.apple.quarantine"
    - path: "kinder-site/src/content/docs/changelog.md"
      provides: "SC3 wording mirrored into the v2.4 Hardening changelog section (changelog half of SC3 'and')"
      contains: "xattr -d com.apple.quarantine"
    - path: ".planning/release-notes-v2.4-draft.md"
      provides: "SC3 wording mirrored into the v2.4 release-notes draft (release-notes half of SC3 'and')"
      contains: "xattr -d com.apple.quarantine"
    - path: ".planning/PROJECT.md"
      provides: "Key Decisions table row recording ad-hoc signing choice"
      contains: "ad-hoc"
  key_links:
    - from: ".github/workflows/macos-sign-verify.yml"
      to: "dist/kinder_darwin_amd64_v1/kinder"
      via: "codesign -vvv with grep -q gate"
      pattern: "codesign -vvv .*darwin_amd64.*kinder"
    - from: ".github/workflows/macos-sign-verify.yml"
      to: "dist/kinder_darwin_arm64/kinder"
      via: "codesign -vvv with grep -q gate"
      pattern: "codesign -vvv .*darwin_arm64.*kinder"
    - from: "kinder-site/src/content/docs/installation.md"
      to: "Homebrew install path"
      via: "doc statement that Homebrew is unaffected"
      pattern: "Homebrew install unaffected"
---

<objective>
Wire automated CI verification of the darwin ad-hoc signatures (SC1+SC2), publish the user-facing install guide wording (SC3), and record the decision in PROJECT.md. Plan 54-01 made the signing happen; this plan PROVES it happens (in CI) and TELLS USERS what it means.

Purpose:
- SC1/SC2 require CI-side proof that the signed binaries verify with `codesign -vvv`. Running this only on tag push (in release.yml) would let signing regressions sit undetected until the next release. A snapshot-verify workflow on main-branch push (gated by `paths:` filter so it only fires when `.goreleaser.yaml` or the workflow itself changes) catches regressions immediately while bounding CI cost.
- SC3 requires BOTH the install guide AND release notes to explicitly disclose that the binary is ad-hoc signed (not notarized), that Homebrew is unaffected, and that direct downloads need `xattr -d com.apple.quarantine` (with the literal backtick-wrapped form). The "Release notes AND install guide" conjunction is binding — SC3 is NOT satisfied by inclusion in only one venue. The release-notes side of the conjunction is satisfied by inserting the wording into BOTH `kinder-site/src/content/docs/changelog.md` (the canonical user-facing changelog rendered by the docs site) AND `.planning/release-notes-v2.4-draft.md` (the planning-side draft that explicitly mirrors the changelog v2.4 slice). Without all three insertions, users who download from GitHub Releases hit the Gatekeeper quarantine on first run and have no actionable guidance, AND the v2.4 release announcement that ships with the tag would omit the disclosure.
- The Key Decisions row in PROJECT.md is the future-self memo: when the question "why didn't we do full Developer ID signing/notarization in v2.4?" comes up, the answer is one grep away.

Output:
- `.github/workflows/macos-sign-verify.yml` (NEW): macos-latest job that runs `goreleaser release --snapshot --clean --skip=publish` then verifies both darwin binaries with `codesign -vvv` and fails the job if either verification fails or the expected "satisfies its Designated Requirement" string is absent
- `kinder-site/src/content/docs/installation.md`: appended `:::caution[macOS direct download]` Starlight block with SC3 wording verbatim (install-guide half of SC3 conjunction)
- `kinder-site/src/content/docs/changelog.md`: new `### macOS Ad-Hoc Signing (Phase 54)` subsection inside `## v2.4 — Hardening` with SC3 wording verbatim (changelog half of SC3 conjunction)
- `.planning/release-notes-v2.4-draft.md`: new `## macOS Ad-Hoc Signing (Phase 54)` section between `## Internal Changes` and `## Verification`, mirroring the changelog content (release-notes-draft half of SC3 conjunction)
- `.planning/PROJECT.md`: new row in Key Decisions table for ad-hoc signing
</objective>

<execution_context>
@/Users/patrykattc/work/git/kinder/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/work/git/kinder/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/54-macos-ad-hoc-code-signing/54-RESEARCH.md
@.planning/phases/54-macos-ad-hoc-code-signing/54-01-SUMMARY.md

# Doc file being modified — read once at start
@kinder-site/src/content/docs/installation.md
</context>

<interfaces>
<!-- Snapshot build path semantics — critical for Task 1 verify step -->
<!-- Source: RESEARCH Topics 6, 7; goreleaser/discussions/3183 -->

GoReleaser snapshot dist layout (verified):
  dist/kinder_<snapshot-version>_darwin_amd64_v1/kinder    -- amd64 binary path (matches SC1)
  dist/kinder_<snapshot-version>_darwin_arm64/kinder       -- arm64 binary path (matches SC2)

The `_v1` suffix on amd64 comes from GOAMD64=v1 (Go's default since 1.18). arm64 has no equivalent suffix.

SC1/SC2 reference these paths WITHOUT the snapshot version component (e.g., `dist/kinder_darwin_amd64_v1/kinder`). The snapshot dist directory DOES include the version (`kinder_0.1.0-SNAPSHOT-abc1234_darwin_amd64_v1`). The verify step must use a glob/find pattern rather than the hardcoded SC path:

  find dist/ -name kinder -path '*darwin_amd64*' | head -1
  find dist/ -name kinder -path '*darwin_arm64*' | head -1

This matches both snapshot and release dist layouts and aligns with the SC intent (the SC wording is about which binary is verified, not about the literal snapshot path string).

codesign -vvv output (verified via Apple TN2206):
  Success exit code: 0
  Success stderr line (note: -vvv writes to stderr, not stdout):
    <path>: valid on disk
    <path>: satisfies its Designated Requirement
  Failure exit code: non-zero (1, 2, or higher depending on failure mode)

CI verification pattern (CI-FAILING per quality_gate):
  set -euo pipefail
  BIN=$(find dist/ -name kinder -path '*darwin_amd64*' | head -1)
  [ -n "$BIN" ] || { echo "FAIL: amd64 binary not found"; exit 1; }
  codesign -vvv "$BIN" 2>&1 | tee /tmp/codesign_amd64.log
  grep -q "satisfies its Designated Requirement" /tmp/codesign_amd64.log

The `set -euo pipefail` + explicit exit codes + `grep -q` ensures a missing signature breaks the build, not just logs a warning. Per quality_gate constraint.

GitHub Actions trigger scope (RESEARCH Topic 7, Pitfall 8):
  on:
    push:
      branches: [main]
      paths:
        - .goreleaser.yaml
        - .github/workflows/macos-sign-verify.yml
    workflow_dispatch: {}

`paths:` filter bounds macOS-runner billing (10x multiplier) by only firing the workflow when signing-relevant files change. `workflow_dispatch:` allows manual re-runs. Do NOT trigger on every PR (cost) or on every push to main (cost).
</interfaces>

<tasks>

<task type="auto" n="1">
  <name>Task 1: Create .github/workflows/macos-sign-verify.yml snapshot+verify CI workflow</name>
  <files>.github/workflows/macos-sign-verify.yml</files>
  <atomic_commit>ci(54-02): add macos-sign-verify workflow for snapshot SC1+SC2 verification</atomic_commit>
  <action>
Create NEW file `.github/workflows/macos-sign-verify.yml`. Write the file with the EXACT content below (no edits beyond what is specified):

```yaml
# Source: .planning/phases/54-macos-ad-hoc-code-signing/54-RESEARCH.md (Topics 6-7)
# Purpose: SC1 + SC2 verification — proves darwin/amd64 and darwin/arm64 GoReleaser
# snapshot binaries satisfy codesign -vvv on a macOS runner with the same toolchain
# as the release pipeline. Catches signing regressions before they reach a tag push.
name: macOS Sign Verify

on:
  push:
    branches:
      - main
    paths:
      - .goreleaser.yaml
      - .github/workflows/macos-sign-verify.yml
  workflow_dispatch: {}

permissions:
  contents: read

jobs:
  snapshot-sign-verify:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
        with:
          fetch-depth: 0

      - uses: actions/setup-go@7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5 # v6.2.0
        with:
          go-version-file: .go-version

      - name: Snapshot build (no publish)
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --snapshot --clean --skip=publish
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Verify darwin/amd64 ad-hoc signature (SC1)
        shell: bash
        run: |
          set -euo pipefail
          BIN=$(find dist/ -name kinder -path '*darwin_amd64*' | head -1)
          if [ -z "$BIN" ]; then
            echo "FAIL: darwin/amd64 binary not found under dist/"
            exit 1
          fi
          echo "Verifying: $BIN"
          codesign -vvv "$BIN" 2>&1 | tee /tmp/codesign_amd64.log
          grep -q "satisfies its Designated Requirement" /tmp/codesign_amd64.log

      - name: Verify darwin/arm64 ad-hoc signature (SC2)
        shell: bash
        run: |
          set -euo pipefail
          BIN=$(find dist/ -name kinder -path '*darwin_arm64*' | head -1)
          if [ -z "$BIN" ]; then
            echo "FAIL: darwin/arm64 binary not found under dist/"
            exit 1
          fi
          echo "Verifying: $BIN"
          codesign -vvv "$BIN" 2>&1 | tee /tmp/codesign_arm64.log
          grep -q "satisfies its Designated Requirement" /tmp/codesign_arm64.log
```

WHY EVERY ELEMENT (do NOT change without re-reading RESEARCH Topic 7):

- Trigger scope (`on:`): push to main + `paths:` filter on `.goreleaser.yaml` and this workflow file ONLY. macOS runners are 10x Linux billing (RESEARCH Pitfall 7) — gating on signing-relevant files is the cost control. `workflow_dispatch: {}` enables manual re-runs without a code change.
- `permissions: contents: read`: Snapshot does not publish anything. Read-only token is sufficient; least-privilege.
- `runs-on: macos-latest`: codesign is macOS-only. Same runner class as release.yml (Plan 54-01 Task 3) so the verification matches the release pipeline exactly.
- Pinned action SHAs for checkout and setup-go: Match the SHAs already in `release.yml` (verified during planning). Pinning to SHA is the repo's established CI convention.
- `goreleaser-action@v7` with `version: "~> v2"`: Identical configuration to release.yml so the snapshot build uses the same goreleaser version as release.
- `args: release --snapshot --clean --skip=publish`: Snapshot build that produces dist/ artifacts but does NOT cut a GitHub release or update Homebrew. `--clean` removes any stale dist/ from a prior run.
- `set -euo pipefail`: REQUIRED per quality_gate. Unset variables, command failures, and pipe failures all fail the step. Without this, a missing binary or a codesign error could silently pass.
- `find dist/ -name kinder -path '*darwin_amd64*' | head -1`: Snapshot dist paths include the version string (`kinder_0.1.0-SNAPSHOT-abc1234_darwin_amd64_v1`), not the bare SC path. find-by-name + path-glob locates the binary regardless of version string. `head -1` is defensive (there should be exactly one match).
- Explicit `if [ -z "$BIN" ]` check: Belt-and-suspenders alongside `set -e`. If find returns nothing (e.g., GoReleaser config breaks darwin builds), this prints an actionable error message before exit.
- `codesign -vvv "$BIN" 2>&1 | tee /tmp/codesign_*.log`: `codesign -vvv` writes to STDERR (RESEARCH Topic 6 + Pitfall 9). `2>&1` merges stderr to stdout so `tee` captures it and `grep` can read it. The log file is for debugging in CI; `tee` also keeps the output visible in the job log.
- `grep -q "satisfies its Designated Requirement"`: SC1/SC2 specify this EXACT string as the success signal. `grep -q` exits 0 if found, non-zero if not — combined with `set -e`, a missing-string failure aborts the job.
- Two separate verify steps (one per architecture): SC2 wording mandates "both architectures verified independently" — separate named steps make this explicit in the CI log and ensure both run even if one fails (well, `set -e` aborts on first failure, but each step is its own job step so the failure attribution is clear).

DO NOT:
- Add a `goreleaser check` step before snapshot — the snapshot build itself validates the config.
- Add a `pull_request:` trigger — billing concern (RESEARCH Pitfall 8).
- Use `if: success()` between verify steps — `set -e` already enforces fail-fast within a step; cross-step gating is GitHub Actions' default behavior.
- Hardcode the snapshot version in the find pattern.
- Use `codesign --verify` (deprecated alias) — use `codesign -vvv` to match SC wording exactly.
- Strip the `-vvv` to fewer v's — SC wording specifies three v's (verbose verify, which is what produces the "satisfies" line).
- Replace `grep -q` with just relying on `codesign` exit code — the SC wording is about the OUTPUT STRING, not just exit code. Both gates together is the correct read of the SC.

CRITICAL: Do not add a `paths:` entry for `.github/workflows/release.yml`. The release workflow runs only on tag push; changing it does NOT require a re-verify (the verify here is about the signed binary contents, which depend on `.goreleaser.yaml` only). Including release.yml in paths would over-trigger.
  </action>
  <verify>
    <automated>set -e; test -f .github/workflows/macos-sign-verify.yml; grep -q "^name: macOS Sign Verify$" .github/workflows/macos-sign-verify.yml; grep -q "runs-on: macos-latest" .github/workflows/macos-sign-verify.yml; grep -q "release --snapshot --clean --skip=publish" .github/workflows/macos-sign-verify.yml; grep -q "set -euo pipefail" .github/workflows/macos-sign-verify.yml; test "$(grep -c 'satisfies its Designated Requirement' .github/workflows/macos-sign-verify.yml)" = "2"; grep -q "find dist/ -name kinder -path '\*darwin_amd64\*'" .github/workflows/macos-sign-verify.yml; grep -q "find dist/ -name kinder -path '\*darwin_arm64\*'" .github/workflows/macos-sign-verify.yml; grep -q "workflow_dispatch" .github/workflows/macos-sign-verify.yml; ! grep -q "pull_request:" .github/workflows/macos-sign-verify.yml</automated>
  </verify>
  <done>`.github/workflows/macos-sign-verify.yml` exists; declares `runs-on: macos-latest`; runs `goreleaser release --snapshot --clean --skip=publish`; contains two `grep -q "satisfies its Designated Requirement"` checks (one per arch); both verify steps use `set -euo pipefail`; trigger is push-to-main with paths filter on `.goreleaser.yaml` + this workflow file plus `workflow_dispatch`; no pull_request trigger.</done>
</task>

<task type="auto" n="2">
  <name>Task 2: Insert SC3 ad-hoc-signing wording into install guide + changelog + release-notes draft (3 doc files)</name>
  <files>kinder-site/src/content/docs/installation.md, kinder-site/src/content/docs/changelog.md, .planning/release-notes-v2.4-draft.md</files>
  <atomic_commit>docs(54-02): document ad-hoc signing + xattr across install guide, changelog, and v2.4 release-notes draft (SC3)</atomic_commit>
  <action>
SC3 wording is "Release notes AND install guide explicitly state ..." — the conjunction is binding. This task inserts the SC3 string into all THREE doc files that constitute the user-facing release surface for v2.4: the install guide (Starlight site), the changelog (Starlight site, source of truth for release-notes content), and the v2.4 release-notes draft (mirror under `.planning/`). The SC3 contract is NOT satisfied by inclusion in the install guide alone.

The SC3 wording, verbatim and binding (the phase verifier `grep -F`s this exact substring):

> `ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires \`xattr -d com.apple.quarantine\``

The backticks around `xattr -d com.apple.quarantine` are part of the SC wording (Markdown inline code formatting). Every insertion below MUST preserve those backticks literally.

---

### Insertion 1 — `kinder-site/src/content/docs/installation.md` (Starlight admonition)

The file uses Astro Starlight with `:::note` and `:::caution` directive blocks (confirmed: the file already has a `:::note` block at lines 6-8).

Append a new `:::caution` block to the existing "### macOS / Linux" subsection of the "## Download Pre-built Binary" section. Insert the block AFTER the existing `tar xzf ... ; chmod +x kinder ; sudo mv kinder /usr/local/bin/` code fence at line 37 and BEFORE the "### Windows" heading at line 39.

The exact block to insert (DO NOT paraphrase — SC3 wording is binding):

```markdown

:::caution[macOS direct download]
kinder macOS binaries are **ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`**.

Ad-hoc signing satisfies Apple's AMFI kernel check on Apple Silicon (the binary no longer gets `Killed: 9` on first run), but it does **not** satisfy Gatekeeper for files downloaded via a web browser. After downloading from GitHub Releases, remove the quarantine attribute before running:

```sh
xattr -d com.apple.quarantine kinder
```

Users installing via `brew install patrykquantumnomad/kinder/kinder` are unaffected — Homebrew bypasses Gatekeeper quarantine for formula-installed binaries.
:::

```

Requirements for THIS insertion:
- The bolded sentence MUST contain the SC3 string with backticks around `xattr -d com.apple.quarantine` exactly as shown.
- Use `:::caution`, not `:::warning` or `:::tip`.
- Code fence uses `sh` language tag (matches the convention of other code fences at lines 17-19, 33-37, 47-51, 56-58, 63-65).
- Tap name `patrykquantumnomad/kinder/kinder` matches line 18 of the existing file.
- Preserve LF line endings.

---

### Insertion 2 — `kinder-site/src/content/docs/changelog.md` (new subsection inside `## v2.4 — Hardening`)

The file already has a `## v2.4 — Hardening` section (line 14) with subsections `### Addon Bumps` (line 20), `### Documented Holds` (line 27), `### Breaking Changes for Upgraders` (line 32), `### SYNC-05 (Default Node Image)` (line 44), and `### Internal` (line 48). The `## v1.5 — Inner Loop` section begins at line 54.

Insert a NEW subsection titled `### macOS Ad-Hoc Signing (Phase 54)` immediately AFTER the `### Internal` subsection's last bullet (line 50) and BEFORE the `---` separator at line 52. The insertion must preserve the existing `---` separator that closes the v2.4 block.

The exact block to insert:

```markdown

### macOS Ad-Hoc Signing (Phase 54)

macOS binaries shipped from v2.4 are **ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`**.

Apple Silicon (Apple-Mx) macOS enforces AMFI kernel-level signature checks on every Mach-O binary; unsigned binaries are killed with `Killed: 9` on first run. v2.4 wires `codesign --force --sign -` (ad-hoc identity, hash-only signature) into the GoReleaser `builds[].hooks.post` pipeline so every darwin/amd64 and darwin/arm64 binary carries an embedded ad-hoc signature before it is archived. The signature satisfies AMFI but not Gatekeeper notarization — direct downloads from GitHub Releases still hit the macOS quarantine attribute. Use `xattr -d com.apple.quarantine kinder` after extracting the archive, or install via Homebrew (`brew install patrykquantumnomad/kinder/kinder`), which bypasses quarantine for formula-installed binaries.

Full Developer ID signing + notarization is deferred to a future phase (DIST-03). (DIST-01)
```

Requirements for THIS insertion:
- The bolded sentence MUST contain the SC3 string with backticks around `xattr -d com.apple.quarantine` exactly as shown.
- The new subsection MUST sit inside the `## v2.4 — Hardening` block (above the closing `---` at line 52), NOT below it.
- The "(DIST-01)" requirement-ID suffix matches the convention of existing v2.4 bullets (see line 22 `(ADDON-01)`).
- DIST-03 reference matches the deferral pointer recorded in PROJECT.md Key Decisions (added in Task 3).

---

### Insertion 3 — `.planning/release-notes-v2.4-draft.md` (mirror of changelog v2.4 slice)

The file's header declares "Source of truth: `kinder-site/src/content/docs/changelog.md`" — this draft mirrors the user-facing changelog slice. The file currently has `## Highlights` (line 6), `## Upgraders' Notes` (line 16), `## Internal Changes (informational)` (line 47), and `## Verification` (line 52). The file ends at line 54.

Insert a NEW top-level section titled `## macOS Ad-Hoc Signing (Phase 54)` immediately AFTER the `## Internal Changes (informational)` section's last bullet (line 50) and BEFORE the `## Verification` section header (line 52).

The exact block to insert:

```markdown

## macOS Ad-Hoc Signing (Phase 54)

macOS binaries shipped from v2.4 are **ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`**.

Apple Silicon (Apple-Mx) macOS enforces AMFI kernel-level signature checks on every Mach-O binary; unsigned binaries are killed with `Killed: 9` on first run. v2.4 wires `codesign --force --sign -` (ad-hoc identity, hash-only signature) into the GoReleaser `builds[].hooks.post` pipeline so every darwin/amd64 and darwin/arm64 binary carries an embedded ad-hoc signature before it is archived.

The signature satisfies AMFI but does NOT satisfy Gatekeeper notarization. Direct downloads from GitHub Releases still hit the macOS quarantine attribute on first run. To work around the quarantine after extracting the archive:

```sh
xattr -d com.apple.quarantine kinder
```

Homebrew installs (`brew install patrykquantumnomad/kinder/kinder`) are unaffected — Homebrew bypasses Gatekeeper quarantine for formula-installed binaries.

Full Developer ID signing + notarization is deferred to a future phase (DIST-03). (DIST-01)
```

Requirements for THIS insertion:
- The bolded sentence MUST contain the SC3 string with backticks around `xattr -d com.apple.quarantine` exactly as shown.
- The new section MUST be placed BEFORE the `## Verification` section header (line 52), so the file's final section stays "Verification".
- The `sh` code fence (matching the install guide and changelog insertions).
- Preserve LF line endings.

---

### Why all three doc files (do NOT collapse to one)

SC3 wording is binding: "Release notes AND install guide". The mapping is:
- **Install guide** = `kinder-site/src/content/docs/installation.md`
- **Release notes (canonical, user-facing on the docs site)** = `kinder-site/src/content/docs/changelog.md` `## v2.4 — Hardening` section
- **Release notes (planning-side draft)** = `.planning/release-notes-v2.4-draft.md` — explicitly declared as a mirror of the changelog (see its header: "Source of truth: `kinder-site/src/content/docs/changelog.md`")

A single insertion in installation.md does NOT satisfy SC3's "and" clause — the release-notes half must also carry the wording. The draft file is included because it is the planning-side artifact tracked under `.planning/` and is the source the phase verifier audits when SC3 evidence is collected before the v2.4 release ships.

DO NOT:
- Paraphrase the SC3 string anywhere — every insertion uses the verbatim wording (the phase verifier `grep -F`s it).
- Omit the backticks around `xattr -d com.apple.quarantine` — the verifier asserts `grep -F 'requires `xattr -d com.apple.quarantine`'` (i.e., the backtick-wrapped form) on every file. A document with `direct download requires xattr -d com.apple.quarantine` (no backticks) would fail the verifier.
- Re-order the v2.4 — Hardening subsections in changelog.md — the new subsection goes at the END of the v2.4 block, immediately before the closing `---`.
- Touch the `## Verification` section of release-notes-v2.4-draft.md (it stays the final section).
- Touch the README.md — installation.md is the authoritative install guide for kinder; the README is project-level and does not currently include install instructions in this style.
  </action>
  <verify>
    <automated>set -e; \
# --- installation.md (install guide half of SC3) ---
grep -F 'ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`' kinder-site/src/content/docs/installation.md; \
grep -F 'requires `xattr -d com.apple.quarantine`' kinder-site/src/content/docs/installation.md; \
grep -q ':::caution\[macOS direct download\]' kinder-site/src/content/docs/installation.md; \
grep -q "Homebrew bypasses Gatekeeper quarantine" kinder-site/src/content/docs/installation.md; \
# --- changelog.md (changelog half of SC3) ---
grep -F 'ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`' kinder-site/src/content/docs/changelog.md; \
grep -F 'requires `xattr -d com.apple.quarantine`' kinder-site/src/content/docs/changelog.md; \
grep -q '^### macOS Ad-Hoc Signing (Phase 54)$' kinder-site/src/content/docs/changelog.md; \
# --- release-notes-v2.4-draft.md (release-notes half of SC3) ---
grep -F 'ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`' .planning/release-notes-v2.4-draft.md; \
grep -F 'requires `xattr -d com.apple.quarantine`' .planning/release-notes-v2.4-draft.md; \
grep -q '^## macOS Ad-Hoc Signing (Phase 54)$' .planning/release-notes-v2.4-draft.md; \
# --- structural: each file's SC3 string appears exactly once (no accidental dupes) ---
test "$(grep -cF 'ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`' kinder-site/src/content/docs/installation.md)" = "1"; \
test "$(grep -cF 'ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`' kinder-site/src/content/docs/changelog.md)" = "1"; \
test "$(grep -cF 'ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`' .planning/release-notes-v2.4-draft.md)" = "1"</automated>
  </verify>
  <done>All THREE doc files (installation.md, changelog.md, release-notes-v2.4-draft.md) contain the exact SC3 wording with backticks around `xattr -d com.apple.quarantine`. installation.md uses a `:::caution[macOS direct download]` Starlight block between the macOS/Linux download fence and the Windows heading. changelog.md has a new `### macOS Ad-Hoc Signing (Phase 54)` subsection inside the `## v2.4 — Hardening` block. release-notes-v2.4-draft.md has a new `## macOS Ad-Hoc Signing (Phase 54)` section inserted between `## Internal Changes (informational)` and `## Verification`. The SC3 string appears exactly once in each file (no accidental duplication). All other content in all three files is unchanged.</done>
</task>

<task type="auto" n="3">
  <name>Task 3: Add ad-hoc signing row to PROJECT.md Key Decisions table</name>
  <files>.planning/PROJECT.md</files>
  <atomic_commit>docs(54-02): record ad-hoc signing decision in PROJECT.md Key Decisions</atomic_commit>
  <action>
Edit `.planning/PROJECT.md`. Append a new row to the LAST "Key Decisions" table block (the v2.2 block ending at line 224 with the "Zero new Go module dependencies" row).

Insert the new row IMMEDIATELY AFTER line 224 (the `| Zero new Go module dependencies in v2.2 ...` row) and BEFORE the closing `---` separator on line 226.

Exact line to insert (one new table row, pipe-delimited, matching the column widths of surrounding rows):

```
| Ad-hoc codesign on macos-latest, not notarization | AMFI on Apple Silicon kills unsigned arm64 binaries; ad-hoc = hash-only signature satisfies AMFI with zero certificate/cost overhead; full Developer ID + notarization deferred (DIST-03) | ✓ Good |
```

ALSO: Update the "Last updated" footer line at the bottom of the file (currently line 227: `*Last updated: 2026-05-09 — milestone v2.4 Hardening started*`) to:

```
*Last updated: 2026-05-12 — Phase 54 (macOS ad-hoc code signing) decision recorded*
```

WHY THIS ROW WORDING:
- "Ad-hoc codesign on macos-latest, not notarization" captures BOTH dimensions (what + runner) in the Decision column, matching the style of other rows like "Headlamp over kubernetes/dashboard" (compares the chosen option to the alternative).
- Rationale mentions: (1) the technical driver (AMFI on Apple Silicon kills unsigned arm64 binaries) — this is the why-now; (2) the cost trade-off (zero certificate overhead); (3) the deferral pointer (DIST-03 notarization) — future-self memo so the answer to "why not notarize?" is one grep away.
- "✓ Good" matches the existing convention (all v2.2 rows end in ✓ Good).
- The 2026-05-12 date matches the project's current date and aligns with Plan 54-02's commit timestamp.

DO NOT:
- Add columns or change the table schema.
- Add a separate new section header — append to the existing last Key Decisions block.
- Reorder existing rows.
- Touch any other section (Active, Deferred, Out of Scope, Context, Constraints).
- Add an emoji to the new row that other rows don't use.
  </action>
  <verify>
    <automated>set -e; grep -q "^| Ad-hoc codesign on macos-latest, not notarization | " .planning/PROJECT.md; grep -q "AMFI on Apple Silicon kills unsigned arm64 binaries" .planning/PROJECT.md; grep -q "full Developer ID + notarization deferred (DIST-03)" .planning/PROJECT.md; grep -q "^\*Last updated: 2026-05-12 — Phase 54" .planning/PROJECT.md; ! grep -q "^\*Last updated: 2026-05-09" .planning/PROJECT.md</automated>
  </verify>
  <done>`.planning/PROJECT.md` has a new Key Decisions row recording the ad-hoc signing decision (with AMFI rationale and DIST-03 deferral pointer), and the "Last updated" footer line is bumped to 2026-05-12 referencing Phase 54. No other sections of PROJECT.md are touched.</done>
</task>

</tasks>

<verification>
After all three tasks complete:

1. **Workflow file structural correctness**: The new workflow file must be valid YAML and parseable by `actionlint` if installed. If not installed, a simple `python3 -c 'import yaml; yaml.safe_load(open(".github/workflows/macos-sign-verify.yml"))'` confirms YAML validity. The workflow file should be syntactically valid even without running it (we cannot trigger a real macOS CI job from this verification block).

2. **End-to-end SC1/SC2 dry-run (HUMAN-OPTIONAL — for confidence)**: After pushing to a branch, trigger the workflow via `gh workflow run macos-sign-verify.yml --ref <branch>` (workflow_dispatch is enabled). Expect both verify steps to pass green with the "satisfies its Designated Requirement" line visible in the CI log. This is the authoritative SC1+SC2 satisfaction signal and should fire automatically on next push-to-main that touches `.goreleaser.yaml`.

3. **SC3 wording grep across all three doc files**: SC3 wording is "Release notes AND install guide" — the conjunction requires the string in installation.md AND changelog.md AND the release-notes draft. Each file must contain the SC3 string AND the backtick-wrapped `xattr` form. Verify all six gates pass:
   ```bash
   for f in kinder-site/src/content/docs/installation.md kinder-site/src/content/docs/changelog.md .planning/release-notes-v2.4-draft.md; do
     grep -F "ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires \`xattr -d com.apple.quarantine\`" "$f" || { echo "FAIL: SC3 string missing in $f"; exit 1; }
     grep -F 'requires `xattr -d com.apple.quarantine`' "$f" || { echo "FAIL: backtick-wrapped xattr missing in $f"; exit 1; }
   done
   ```

4. **PROJECT.md decision row**: One new row appended in the LAST Key Decisions block; no other table rows modified.

5. **SC4 untouched**: Plan 54-01 owns SC4 (sign-as-last-op). This plan must NOT modify `.goreleaser.yaml`. Confirm:
   ```bash
   git diff --stat HEAD~3 HEAD -- .goreleaser.yaml
   # Expected: no .goreleaser.yaml change in Plan 54-02's three commits
   ```
</verification>

<success_criteria>
- [ ] `.github/workflows/macos-sign-verify.yml` exists, valid YAML, runs on macos-latest, triggers on push-to-main with paths filter on `.goreleaser.yaml` + this workflow file + workflow_dispatch
- [ ] Workflow contains TWO `grep -q "satisfies its Designated Requirement"` checks (one for amd64, one for arm64), each preceded by `set -euo pipefail`
- [ ] Workflow uses `find dist/ -name kinder -path '*darwin_amd64*'` and `find dist/ -name kinder -path '*darwin_arm64*'` glob patterns (NOT hardcoded snapshot version paths)
- [ ] `kinder-site/src/content/docs/installation.md` contains the SC3 wording verbatim (with backtick-wrapped `xattr -d com.apple.quarantine`) inside a `:::caution[macOS direct download]` block
- [ ] `kinder-site/src/content/docs/installation.md` `xattr -d com.apple.quarantine kinder` code fence uses `sh` language tag
- [ ] `kinder-site/src/content/docs/changelog.md` contains the SC3 wording verbatim (with backtick-wrapped `xattr`) inside a new `### macOS Ad-Hoc Signing (Phase 54)` subsection of the `## v2.4 — Hardening` block — satisfies the changelog half of SC3's "Release notes AND install guide" conjunction
- [ ] `.planning/release-notes-v2.4-draft.md` contains the SC3 wording verbatim (with backtick-wrapped `xattr`) inside a new `## macOS Ad-Hoc Signing (Phase 54)` section inserted between `## Internal Changes (informational)` and `## Verification` — satisfies the release-notes-draft half of SC3
- [ ] The SC3 string appears EXACTLY ONCE in each of the three doc files (no duplicate insertions)
- [ ] `.planning/PROJECT.md` has a new Key Decisions row recording the ad-hoc signing decision and DIST-03 deferral pointer
- [ ] `.planning/PROJECT.md` "Last updated" footer line bumped to 2026-05-12 with Phase 54 reference
- [ ] All three tasks committed as separate atomic commits
- [ ] No changes to `.goreleaser.yaml` (SC4 already delivered in Plan 54-01)
- [ ] No changes to `.github/workflows/release.yml` (runner already changed in Plan 54-01 Task 3)
</success_criteria>

<output>
After completion, create `.planning/phases/54-macos-ad-hoc-code-signing/54-02-SUMMARY.md` documenting:
- The three commits in order (workflow, three-file SC3 docs insertion, PROJECT.md decision row)
- Confirmation that all four phase-level SCs are now satisfied: SC1+SC2 via the new workflow; SC3 via the three-file insertion (installation.md + changelog.md + release-notes-v2.4-draft.md, all containing the backtick-wrapped SC3 string); SC4 via Plan 54-01
- Confirmation that the SC3 "Release notes AND install guide" conjunction is fully covered — the verifier's 3-file × 2-grep gate (literal SC3 string + backtick-wrapped `xattr` form) passes for all three files
- Note any actual `gh workflow run` execution result if a manual dry-run was performed
- Any deviations (none expected) with rationale
- Pointer for the Phase-54 verifier: SC1/SC2 evidence = CI log; SC3 evidence = three-file `grep -F` diff (installation.md + changelog.md + release-notes-v2.4-draft.md); SC4 evidence = `.goreleaser.yaml` hooks placement (delivered in 54-01)
</output>
