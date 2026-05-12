---
phase: 54-macos-ad-hoc-code-signing
plan: 01
type: execute
wave: 1
status: pending
depends_on: []
files_modified:
  - .goreleaser.yaml
  - .github/workflows/release.yml
autonomous: true
requirements:
  - DIST-01

must_haves:
  truths:
    - "darwin/amd64 GoReleaser builds emit a Mach-O binary with an embedded ad-hoc signature (codesign --sign -) before archiving"
    - "darwin/arm64 GoReleaser builds emit a Mach-O binary with an embedded ad-hoc signature (codesign --sign -) before archiving"
    - "linux and windows build targets are NOT affected by the codesign hook (shell conditional gates on .Os == darwin)"
    - "The sign step is the LAST operation on each binary before archiving — no post-sign strip, UPX, or binary copy invalidates the Mach-O signature block"
    - "ldflags carry -s -w per DIST-01 wording (current state has only -w)"
    - "The release workflow runs on a macOS runner so that codesign is available; CGO_ENABLED=0 cross-compile to linux/windows continues to work"
  artifacts:
    - path: ".goreleaser.yaml"
      provides: "Darwin-gated codesign post-hook on builds[]; -s flag added to ldflags"
      contains: "hooks:\n        post:"
    - path: ".github/workflows/release.yml"
      provides: "macOS runner for release pipeline so codesign is available"
      contains: "runs-on: macos-latest"
  key_links:
    - from: ".goreleaser.yaml builds[].hooks.post"
      to: "codesign --force --sign -"
      via: "sh -c conditional on {{ .Os }} == darwin"
      pattern: "codesign --force --sign - \"\\{\\{ \\.Path \\}\\}\""
    - from: ".github/workflows/release.yml jobs.release"
      to: "macos-latest runner"
      via: "runs-on declaration"
      pattern: "runs-on:\\s*macos-latest"
---

<objective>
Wire ad-hoc Mach-O code signing into the GoReleaser build pipeline for darwin targets and move the release workflow to a macOS runner so that `codesign` is available. This plan delivers SC4 (sign as last op before archive) and the prerequisite plumbing for SC1/SC2 (the actual SC1/SC2 verification runs in Plan 54-02).

Purpose: AMFI on Apple Silicon (macOS 11+, arm64) `SIGKILL`s any unsigned Mach-O executable on first run. The Go cross-compiler running on Linux does NOT emit an ad-hoc signature (only the macOS linker does, via `ld64`). Without this fix, every darwin/arm64 binary we ship is killed on first execution. Ad-hoc signing (`codesign --sign -`) embeds the binary's content hash so AMFI accepts it; it does NOT satisfy Gatekeeper for quarantined downloads (that wording is the job of Plan 54-02 SC3).

Output:
- `.goreleaser.yaml` with `-s` added to ldflags and a darwin-gated `builds[].hooks.post` that runs `codesign --force --sign -` on the freshly-built Mach-O binary
- `.github/workflows/release.yml` switched from `ubuntu-latest` to `macos-latest` so the codesign tool is on PATH
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

# Files being modified — read once at start, do not re-read mid-task
@.goreleaser.yaml
@.github/workflows/release.yml
</context>

<interfaces>
<!-- Key contracts and template variables the executor will use. From research + GoReleaser docs. -->
<!-- Source: goreleaser.com/customization/builds/hooks/ + goreleaser.com/customization/builds/go/ -->

GoReleaser v2 builds[].hooks.post fields:
  cmd:    string  (template-expanded shell command)
  dir:    string  (optional working directory)
  env:    []string
  output: bool    (true = stream stdout/stderr to GoReleaser logs — REQUIRED for CI debuggability)

Template variables available in hook cmd:
  {{ .Os }}     -> GOOS for the current target (darwin | linux | windows)
  {{ .Arch }}   -> GOARCH for the current target (amd64 | arm64)
  {{ .Path }}   -> full filesystem path to the built binary in dist/
  {{ .Name }}   -> binary name (kinder)

codesign invocation (ad-hoc signing):
  codesign --force --sign - "<path>"
    --force  : replace any existing signature (idempotent on re-runs)
    --sign - : ad-hoc identity (dash = no certificate, hash-only signature)

Shell conditional pattern (Option A from RESEARCH Topic 3):
  sh -c 'if [ "{{ .Os }}" = "darwin" ]; then codesign --force --sign - "{{ .Path }}"; fi'

Note: The `if:` field on hooks is NOT documented for `builds[].hooks.post` in GoReleaser v2 (only on global before/after hooks). Use shell conditional. Single-quote the outer `sh -c '...'` so the template engine expands {{ .Os }} and {{ .Path }} before sh sees them.

GitHub Actions runner labels (verified 2026-05-12):
  macos-latest -> macos-15 (Intel x64, Xcode CLT preinstalled with codesign)
  CGO_ENABLED=0 cross-compile from macOS to linux/windows: WORKS (pure-Go)
</interfaces>

<tasks>

<task type="auto" n="1">
  <name>Task 1: Add -s strip flag to .goreleaser.yaml ldflags</name>
  <files>.goreleaser.yaml</files>
  <atomic_commit>build(54-01): add -s strip flag to ldflags to satisfy DIST-01 wording</atomic_commit>
  <action>
Edit `.goreleaser.yaml`. In the `builds[0].ldflags` list (currently lines 38-41), insert `-s` immediately after `-buildid=` and before `-w`. The resulting block must be:

```yaml
    ldflags:
      - -buildid=
      - -s
      - -w
      - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .FullCommit }}
```

WHY: DIST-01 requires "signing happens AFTER `-ldflags=\"-s -w\"` strip". The current config only has `-w` (omit DWARF debug info). Adding `-s` (omit symbol table) matches DIST-01 wording literally and is standard practice for release binaries (further size reduction, no runtime effect on a Go binary). Research assumption [A1] flags this as planner-actionable; we proceed with adding `-s`.

DO NOT touch any other field in `.goreleaser.yaml` in this task. Hooks come in Task 2.
  </action>
  <verify>
    <automated>set -e; grep -q "^      - -s$" .goreleaser.yaml; grep -q "^      - -w$" .goreleaser.yaml; grep -c '^      - -' .goreleaser.yaml | awk '$1>=4{exit 0}{exit 1}'; awk '/^    ldflags:/{found=1} found &amp;&amp; /- -s$/{s=NR} found &amp;&amp; /- -w$/{w=NR} END{exit (s>0 &amp;&amp; w>0 &amp;&amp; s&lt;w) ? 0 : 1}' .goreleaser.yaml</automated>
  </verify>
  <done>`.goreleaser.yaml` ldflags list contains both `-s` and `-w` as separate entries, with `-s` line strictly before `-w` line (asserted by awk ordering check on line numbers), `-buildid=` first and `-X=...` last. No other config changes.</done>
</task>

<task type="auto" n="2">
  <name>Task 2: Add darwin-gated codesign post-hook to .goreleaser.yaml builds[]</name>
  <files>.goreleaser.yaml</files>
  <atomic_commit>build(54-01): add darwin-gated codesign post-hook to builds[] (SC4)</atomic_commit>
  <action>
Edit `.goreleaser.yaml`. Append a `hooks` block as the LAST key inside `builds[0]` (i.e., directly after the `mod_timestamp:` line that currently ends the builds entry on line 42). The new block must be:

```yaml
    hooks:
      post:
        - cmd: sh -c 'if [ "{{ .Os }}" = "darwin" ]; then codesign --force --sign - "{{ .Path }}"; fi'
          output: true
```

INDENTATION: `hooks:` is indented 4 spaces (same level as `mod_timestamp:`, `ldflags:`, `flags:`). `post:` is indented 6 spaces. The `- cmd:` list item is indented 8 spaces. `output: true` is indented 10 spaces (aligned under `cmd`).

WHY THIS EXACT FORM (do NOT change without re-reading RESEARCH Topics 1, 3, 4):
- `builds[].hooks.post` runs AFTER `go build` (including `-s -w` strip from Task 1) and BEFORE the archive pipe. This satisfies SC4: "sign step is the LAST operation on each binary before archiving."
- DO NOT use top-level `signs:` — that signs the .tar.gz wrapping, not the Mach-O binary inside it. SC1/SC2 verify the binary in `dist/`, not the archive, so a top-level `signs:` would fail SC1/SC2.
- DO NOT add UPX or `strip` AFTER this hook — any byte modification after codesign invalidates the Mach-O signature block (Pitfall 2 in RESEARCH).
- The shell conditional (`if [ "{{ .Os }}" = "darwin" ]`) is required because the single `builds` entry covers linux/darwin/windows. The hook fires for every target; the conditional no-ops on linux/windows. This is the pattern from RESEARCH Topic 3 Option A. DO NOT use the `if:` hook field — it is undocumented for `builds[].hooks.post` in GoReleaser v2 (Assumption A3 in RESEARCH; only confirmed for global hooks).
- `--force` makes the codesign call idempotent (replaces any prior signature; harmless on re-runs).
- `--sign -` is ad-hoc identity (dash = no certificate). Hash-only. Satisfies AMFI but not Gatekeeper notarization (per SC3 wording, handled in Plan 54-02).
- `output: true` streams codesign stdout/stderr to GoReleaser's log so CI debug is possible. DO NOT omit.
- Single-quote `sh -c '...'` so GoReleaser's Go template engine expands `{{ .Os }}` and `{{ .Path }}` BEFORE sh sees the string. Double-quote `"{{ .Path }}"` inside the conditional to handle any path with spaces (defensive; unlikely for this binary).

DO NOT change ldflags, flags, env, goos, goarch, ignore, archives, or anything outside builds[0].hooks in this task.
  </action>
  <verify>
    <automated>set -e; grep -q "^    hooks:$" .goreleaser.yaml; grep -q "^      post:$" .goreleaser.yaml; grep -q "codesign --force --sign - \"{{ .Path }}\"" .goreleaser.yaml; grep -q "if \\[ \"{{ .Os }}\" = \"darwin\" \\]" .goreleaser.yaml; grep -q "output: true" .goreleaser.yaml; # Confirm hooks block is LAST inside builds entry (no other builds[] fields after it before archives:): awk '/^builds:/{inblock=1} /^archives:/{inblock=0} inblock' .goreleaser.yaml | tail -5 | grep -q "output: true"</automated>
  </verify>
  <done>`.goreleaser.yaml` `builds[0]` ends with a `hooks.post` entry containing the darwin-gated codesign command, `output: true` is set, the codesign command uses `--force --sign -` ad-hoc identity, the template var `{{ .Path }}` is quoted, and no UPX/strip step exists after the hook. The hooks block is the last key inside `builds[0]` before the `archives:` top-level key.</done>
</task>

<task type="auto" n="3">
  <name>Task 3: Switch release.yml runner from ubuntu-latest to macos-latest</name>
  <files>.github/workflows/release.yml</files>
  <atomic_commit>ci(54-01): switch release runner to macos-latest so codesign is available</atomic_commit>
  <action>
Edit `.github/workflows/release.yml` line 13. Change:

```yaml
    runs-on: ubuntu-latest
```

to:

```yaml
    runs-on: macos-latest
```

WHY: `codesign` is a macOS-only tool, part of the Xcode Command Line Tools that are preinstalled on all GitHub-hosted macOS runners. It is NOT available on ubuntu-latest. Without this change, the post-hook from Task 2 will fail in CI when GoReleaser tries to run codesign during a darwin build.

Cross-compile to linux/windows continues to work because `CGO_ENABLED=0` is already set in `.goreleaser.yaml` (line 25). Pure-Go cross-compilation is host-OS-agnostic (verified in RESEARCH Topic 5).

Runner mapping (verified 2026-05-12 per RESEARCH Topic 5 Assumption A4): `macos-latest` -> `macos-15` (Intel x64). codesign on Intel x64 macOS can sign both amd64 AND arm64 Mach-O binaries — codesign is architecture-agnostic as a signing tool; it does not require the host to match the target. DO NOT pin to `macos-15-arm64` (larger paid runner class) or `macos-26` (not yet default).

DO NOT change any other field in `release.yml` (checkout, setup-go, goreleaser-action, env vars, permissions all stay identical).

BILLING NOTE (non-blocking, awareness only): macOS runners consume Actions minutes at a 10x multiplier vs Linux. The release workflow runs on tag push only (not on every PR), so cost impact is per-release, not per-commit. This is documented in RESEARCH Pitfall 7.
  </action>
  <verify>
    <automated>set -e; grep -q "^    runs-on: macos-latest$" .github/workflows/release.yml; ! grep -q "ubuntu-latest" .github/workflows/release.yml</automated>
  </verify>
  <done>`.github/workflows/release.yml` line 13 reads `runs-on: macos-latest`. No occurrences of `ubuntu-latest` remain anywhere in the file. All other lines unchanged.</done>
</task>

</tasks>

<verification>
After all three tasks complete:

1. **Hook placement (SC4 invariant)**: Confirm the `hooks.post` block is the LAST key inside `builds[0]` and that no UPX / strip / binary-copy step exists after it. Top-level `signs:` MUST NOT be added (would sign the archive, not the binary).
   ```bash
   # SC4 structural check: hooks must be inside builds[], not at top level
   grep -n "^signs:" .goreleaser.yaml && echo "FAIL: top-level signs: must NOT exist" || echo "PASS: no top-level signs:"
   # SC4 ordering: hooks must be the last builds[0] field before archives:
   awk '/^builds:/{inblock=1} /^archives:/{exit} inblock' .goreleaser.yaml | grep -E "^    (hooks|ldflags|flags|env|goos|goarch|ignore|main|binary|id|mod_timestamp):" | tail -1 | grep -q "hooks:"
   ```

2. **Local snapshot smoke test (HUMAN-OPTIONAL — for confidence; not required to pass this plan)**: On a local macOS machine, run `goreleaser release --snapshot --clean --skip=publish` and check `find dist/ -name kinder -path '*darwin*' -exec codesign -vvv {} \; 2>&1`. Expect "satisfies its Designated Requirement" for both darwin binaries. NOTE: The authoritative CI verification is delivered by Plan 54-02 (`.github/workflows/macos-sign-verify.yml`). This local smoke is purely a confidence check.

3. **Linux/windows targets unaffected**: The shell conditional ensures codesign is not invoked for linux/windows targets. Local snapshot will show those binaries built but not signed (which is correct — they are not Mach-O).
</verification>

<success_criteria>
- [ ] `.goreleaser.yaml` ldflags contains `-s` and `-w` as separate list entries
- [ ] `.goreleaser.yaml` `builds[0]` contains a `hooks.post` block with `cmd: sh -c 'if [ "{{ .Os }}" = "darwin" ]; then codesign --force --sign - "{{ .Path }}"; fi'` and `output: true`
- [ ] `.goreleaser.yaml` `hooks` block is the LAST key inside `builds[0]` (SC4 ordering invariant)
- [ ] `.goreleaser.yaml` contains NO top-level `signs:` key (SC4 boundary: ad-hoc Mach-O signing belongs on `builds[]`, not on archives)
- [ ] `.github/workflows/release.yml` `runs-on: macos-latest` (was `ubuntu-latest`)
- [ ] All three tasks committed as separate atomic commits
- [ ] No changes to archives, checksum, changelog, release, or homebrew_casks sections of `.goreleaser.yaml`
- [ ] No changes to checkout, setup-go, goreleaser-action steps in release.yml
</success_criteria>

<output>
After completion, create `.planning/phases/54-macos-ad-hoc-code-signing/54-01-SUMMARY.md` documenting:
- The three commits in order (ldflags-s, hooks.post, runs-on)
- Confirmation that SC4 ordering invariant holds (hooks is last builds[] field; no top-level signs:)
- That CI verification (SC1/SC2) is deferred to Plan 54-02
- That installation docs and PROJECT.md decision row are deferred to Plan 54-02
- Any deviations from the plan (none expected) with rationale
</output>
