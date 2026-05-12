# Phase 54: macOS Ad-Hoc Code Signing - Research

**Researched:** 2026-05-12
**Domain:** GoReleaser v2 build hooks, macOS codesign, GitHub Actions macOS runners, AMFI
**Confidence:** HIGH overall (all critical claims verified via Context7, official GoReleaser docs, or Apple official documentation)

---

## Summary

Phase 54 adds ad-hoc code signing to the kinder GoReleaser pipeline so that darwin/amd64 and darwin/arm64 binaries pass Apple AMFI checks on first run. The signing mechanism is `codesign --force --sign -` (dash identity = ad-hoc; no Developer ID or certificate required). Ad-hoc signing does NOT satisfy Gatekeeper for quarantined downloads — users who download the binary directly still need `xattr -d com.apple.quarantine kinder` — but it does satisfy the AMFI kernel enforcement that kills unsigned arm64 Mach-O executables on Apple Silicon.

The canonical implementation uses `builds[].hooks.post` in `.goreleaser.yaml`, which runs AFTER the Go compiler's `go build` (including `-w` strip) but BEFORE the archive step. This guarantees the signed binary is what ends up inside the `.tar.gz`. The release CI job must move from `ubuntu-latest` to `macos-latest` because `codesign` is a macOS-only tool. With `CGO_ENABLED=0`, the macOS runner can cross-compile all targets (linux, windows) without issue.

**Primary recommendation:** Single `builds` entry with a shell-conditional post hook (`sh -c 'if [ "{{ .Os }}" = "darwin" ]; then codesign --force --sign - "{{ .Path }}"; fi'`), runner changed to `macos-latest`, and a separate lightweight snapshot-verify workflow to prove SC1/SC2 in CI. Two plans: (a) GoReleaser config + runner change, (b) snapshot verify workflow + docs/install-guide update.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Ad-hoc signing of Mach-O binary | CI runner (macOS) | GoReleaser post-hook | codesign is macOS-only; must run on macOS runner |
| Build pipeline ordering (sign before archive) | GoReleaser build hook | — | builds[].hooks.post runs before archive pipe |
| Cross-compile linux/windows from macOS runner | Go compiler (CGO_ENABLED=0) | GoReleaser | Pure-Go cross-compile; no native C toolchain needed |
| Quarantine warning bypass (end user) | User CLI (xattr) | Install guide doc | Gatekeeper quarantine is per-download; cannot be pre-cleared by publisher |
| Homebrew install path (unaffected) | Homebrew internals | — | brew formula installs never go through Gatekeeper quarantine |
| CI verification of signing | GitHub Actions job | codesign -vvv | snapshot build + verify step in separate workflow |

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DIST-01 | macOS GoReleaser artifacts ad-hoc signed via `codesign --force --sign -` post-hook running on `macos-latest` CI runner; signing happens AFTER `-ldflags="-s -w"` strip; release notes explicitly state ad-hoc signing does NOT bypass Gatekeeper quarantine | Confirmed: builds[].hooks.post runs after go build (strip in go build), before archive; codesign available on macos-latest |
</phase_requirements>

---

## Topic 1: GoReleaser Post-Hook Mechanism for Binary Signing (SC4 Contract)

**Question:** Is `builds[].hooks.post` the right layer for signing the Mach-O binary in-place BEFORE archiving?

**Answer: YES.** [VERIFIED: Context7/goreleaser/goreleaser, goreleaser.com/customization/builds/hooks/]

GoReleaser's top-level `signs:` key signs **artifacts after archiving** — it operates on archive files (`.tar.gz`, `.zip`) and checksum files, not on the raw binary. Using `signs:` would put the signature inside the archive wrapping; extracting the archive produces an unsigned binary (or one with a broken signature due to the tar operation stripping xattrs). This violates SC4.

`builds[].hooks.post` runs **for each build target, immediately after the binary is compiled and placed in `dist/`**, before any archiving. The hook receives `{{ .Path }}` pointing to the compiled Mach-O file on disk. `codesign --force --sign -` modifies the binary in-place. The archive step then picks up the already-signed binary. SC4 is satisfied.

**Official example from GoReleaser docs:**
```yaml
builds:
  - id: "with-hooks"
    hooks:
      post:
        - upx "{{ .Path }}"
        - codesign -project="{{ .ProjectName }}" "{{ .Path }}"
```
[CITED: https://goreleaser.com/customization/builds/hooks/]

---

## Topic 2: GoReleaser Pipeline Order of Operations

**Answer:** Build → `builds[].hooks.post` → Archive. [VERIFIED: Context7/goreleaser/goreleaser pipeline.go inspection]

The GoReleaser release pipeline executes in this order (simplified):

1. Clean dist
2. Validate git / semver
3. Run global `before.hooks`
4. **Build binaries** (go build, including -w flag strip)
5. **`builds[].hooks.post` runs here** (per target)
6. UPX compression (if configured)
7. Universal binary merge (if configured)
8. **Archive pipe** (tar.gz, zip)
9. Checksum
10. `signs:` (signs archive files, NOT the binary)
11. Publish

**Critical for DIST-01:** The `-w` ldflags strip flag is applied by `go build` at step 4. The post-hook at step 5 runs AFTER the binary is fully written to disk. There is no post-archive strip. Go does not re-strip after writing the binary.

**Conclusion:** `codesign` in `builds[].hooks.post` sees the final, stripped Mach-O binary. Signing after strip is the correct order (signing before strip would invalidate the signature). DIST-01 wording "signing happens AFTER `-ldflags="-s -w"` strip" is satisfied by the hook placement.

**Note on `-s` vs `-w`:** The current `.goreleaser.yaml` has `-w` (omit DWARF debug info) but NOT `-s` (omit symbol table). DIST-01 wording says "after `-ldflags="-s -w"` strip" implying both flags. The repo currently only uses `-w`. **This is a gap the planner must address**: either add `-s` to satisfy DIST-01 wording literally, or confirm with the developer that `-w` alone is the intended baseline. [ASSUMED] that adding `-s` alongside `-w` is acceptable for this CLI tool (standard practice for release binaries to reduce size), but this should be confirmed before locking.

---

## Topic 3: Per-OS Hook Gating

**Question:** How to restrict `codesign` to `goos: darwin` targets only when the builds entry covers linux/darwin/windows?

Two valid approaches:

### Option A: Shell conditional (RECOMMENDED for single-build-entry approach)

```yaml
builds:
  - id: kinder
    hooks:
      post:
        - cmd: sh -c 'if [ "{{ .Os }}" = "darwin" ]; then codesign --force --sign - "{{ .Path }}"; fi'
          output: true
```

**Template variable:** `{{ .Os }}` resolves to the GOOS value for the current target (`darwin`, `linux`, `windows`). [VERIFIED: goreleaser.com/customization/builds/go/ — `.Os` is GOOS, `.Arch` is GOARCH]

**Quoting rules:** Use single quotes for the outer `sh -c '...'` argument. The template expands before the shell executes, so `{{ .Os }}` and `{{ .Path }}` are substituted by GoReleaser's template engine first, then the result is passed to `sh -c`. Paths with spaces are unlikely for a CLI binary but can be quoted inside the conditional: `codesign --force --sign - "{{ .Path }}"`.

### Option B: GoReleaser `if:` field (available for global hooks since v2.7)

The `if:` field with template condition is documented for global hooks (`before.hooks`, `after.hooks`) using `{{ eq .Runtime.Goos "darwin" }}` syntax. [CITED: goreleaser.com/customization/general/hooks/]

**However:** The `if:` field is NOT documented for build-specific hooks (`builds[].hooks.post`) in the GoReleaser v2 build hooks documentation. The build hooks page lists `cmd`, `dir`, `output`, `env` as available fields — no `if`. [CITED: goreleaser.com/customization/builds/hooks/]

**Recommendation:** Use Option A (shell conditional) — it is cross-runner-safe (if someone runs goreleaser locally on Linux, the hook no-ops), documented by example in the goreleaser community, and does not rely on a feature whose availability in build hooks is unconfirmed.

### Option C: Separate darwin build entry

Split the `builds` list into two entries:
- `id: kinder` (linux + windows) — no hook
- `id: kinder-darwin` (darwin only) — post hook with codesign

This requires a matching split in `archives` (using `ids: [kinder-darwin]` for the darwin archive). The archive name template would need adjustment to avoid the `_v1` suffix duplication. This adds config complexity for a one-hook case.

**Decision:** Option A is recommended. The phase doc's "GoReleaser config split" refers to logical plan-file split, not a physical `builds[]` split. If the developer prefers Option C during discuss, the planner should note the archive `ids:` cascade.

---

## Topic 4: Build Split vs Single-Build Conditional

**Recommendation: Keep single build entry, gate with shell conditional (Option A above).**

Rationale:
- One `builds` entry means one `archives` entry; no cascade edits required
- `CGO_ENABLED=0` cross-compile is symmetric — macOS runner builds all targets equally well
- Shell conditional is no-ops on linux/windows targets with zero performance cost
- The phase doc phrase "GoReleaser config split + CI runner change + release notes" means splitting the work across two PLAN files, not splitting the builds config

If the developer insists on a clean-config approach (Option C), the planner needs two `archives` entries with `ids:` filters, and the archive name template for darwin-only must use `{{ .Arch }}` carefully to avoid conflicts with the `_v1` suffix difference between amd64 and arm64.

---

## Topic 5: Runner Migration

**Current:** `release.yml` uses `ubuntu-latest`.
**Required:** Must change to `macos-latest` (or explicit `macos-15`).

**Why macOS runner is mandatory:** `codesign` is a macOS-only tool. It is part of the Xcode Command Line Tools, which are preinstalled on all GitHub-hosted macOS runners. It is not available on ubuntu-latest. [VERIFIED: github.com/actions/runner-images — macos-15-Readme.md lists Xcode CLT preinstalled]

**What `macos-latest` maps to (as of 2026):**
- Since August 2025, `macos-latest` maps to `macos-15`. [VERIFIED: github.com/actions/runner-images/issues/12520]
- `macos-15` uses **Intel x64** architecture by default. Apple Silicon arm64 requires the `macos-15-arm64` label or `macos-15-xlarge` for the larger runner.
- `macos-latest` (= `macos-15`, Intel x64) is fully capable of running `codesign` to sign both darwin/amd64 AND darwin/arm64 Mach-O binaries — `codesign` is architecture-agnostic as a signing tool; it does not require the host to match the binary target architecture.

**CGO_ENABLED=0 cross-compile from macOS runner:**
- Go's pure cross-compile (`CGO_ENABLED=0`) works from any host OS to any GOOS target.
- The current config has `CGO_ENABLED=0`. A macOS runner can build linux/amd64, linux/arm64, windows/amd64 with no extra toolchain setup.
- [VERIFIED: goreleaser-action/issues/233 and direct Go documentation — CGO_ENABLED=0 is sufficient for pure-Go projects]

**Billing note (non-blocking):** GitHub macOS runners consume minutes at a 10x multiplier vs Linux. The `release.yml` job runs on tag push only (not on every PR), so the cost impact is per-release, not per-commit. [CITED: GitHub Actions billing docs]

**Explicit runner recommendation:** Use `macos-latest` (= macos-15, Intel x64, codesign preinstalled). This is sufficient for ad-hoc signing of both darwin/amd64 and darwin/arm64 binaries. `macos-15-arm64` would also work but is a larger/paid runner class for public repos.

---

## Topic 6: `--sign -` Ad-Hoc Identity Semantics

**`-` (dash) as identity means ad-hoc signing.** No Developer ID, no Apple Developer certificate, no notarization. The signature anchors the binary's content hash and prevents AMFI from killing it, but it does NOT satisfy Gatekeeper's trust chain requirements for quarantined downloads.

**What `codesign -vvv` outputs for a properly ad-hoc signed binary:** [VERIFIED: Apple TN2206 — developer.apple.com/library/archive/technotes/tn2206/]

```
./kinder: valid on disk
./kinder: satisfies its Designated Requirement
```

These exact two lines are what SC1 and SC2 assert. The `satisfies its Designated Requirement` line indicates the binary has a Designated Requirement that it meets — for ad-hoc signed binaries, the DR is automatically synthesized as "anchor hash" (the hash of the binary itself). This always passes for a self-consistent ad-hoc signed binary.

**Difference between `-vvv` (verify) and `-dv` (display):**
- `codesign -vvv <path>` — **verification mode**: performs cryptographic checks, prints `valid on disk` + `satisfies its Designated Requirement` on success. This is what SC1/SC2 check.
- `codesign -dv <path>` — **display mode**: shows metadata WITHOUT verifying (Identifier, Format, CodeDirectory hash, Signature=adhoc). Does NOT produce the SC assertion lines.

**SC verification command (exact):**
```bash
codesign -vvv dist/kinder_darwin_amd64_v1/kinder
# expect exit 0 + stdout: "dist/kinder_darwin_amd64_v1/kinder: satisfies its Designated Requirement"

codesign -vvv dist/kinder_darwin_arm64/kinder
# expect exit 0 + stdout: "dist/kinder_darwin_arm64/kinder: satisfies its Designated Requirement"
```

**Note:** `codesign -vvv` outputs to stderr, not stdout. The verify step in CI should use `2>&1` or check exit code. A non-zero exit from `-vvv` means the signature is invalid.

---

## Topic 7: Snapshot Build Verification in CI

**SC1/SC2 require:** "`codesign -vvv dist/...` returns `satisfies its Designated Requirement` in CI after a snapshot build."

**Snapshot build command:**
```bash
goreleaser release --snapshot --clean --skip=publish
```
Or with older goreleaser: `goreleaser release --snapshot --rm-dist` (v2 uses `--clean`). [CITED: goreleaser.com/customization/snapshots/]

**Snapshot version naming:** Default version template is `{{ .Version }}-SNAPSHOT-{{.ShortCommit}}`. The dist/ directory artifact paths use `kinder_<snapshot-version>_darwin_amd64_v1/kinder` and `kinder_<snapshot-version>_darwin_arm64/kinder`. SC1/SC2 reference the binary path without the version component (just `dist/kinder_darwin_amd64_v1/kinder`) — the CI verify step needs to use a glob or find command rather than a hardcoded path.

**`_v1` suffix explanation:** [VERIFIED: goreleaser/discussions/3183]
- `darwin_amd64_v1` — The `_v1` suffix comes from `GOAMD64=v1` (the default Go AMD64 microarchitecture version introduced in Go 1.18). GoReleaser appends it to disambiguate potential multiple AMD64 GOAMD64 targets.
- `darwin_arm64` — arm64 has no equivalent version suffix in the default config (no `GOARM64` variants configured).

**SC assertion paths (exact):**
- `dist/kinder_darwin_amd64_v1/kinder` (SC1)
- `dist/kinder_darwin_arm64/kinder` (SC2)

These match exactly the SC text. The planner should use `find dist/ -name kinder -path '*darwin*'` to locate both binaries robustly in the verify step.

**Where to put the verify step:**

**Recommended (Option B from phase brief):** A NEW workflow file `.github/workflows/snapshot-sign-verify.yml` that:
1. Triggers on `push: branches: [main]` or `pull_request:` (not on tag push)
2. Runs on `macos-latest`
3. Runs `goreleaser release --snapshot --clean --skip=publish`
4. Runs `codesign -vvv` on both darwin binaries
5. Fails the workflow if codesign exits non-zero

This keeps `release.yml` clean (the tag-push release path) and adds an independent verification job for the signing change.

**Minimal-touch alternative (Option A):** Add a second job to `release.yml` before the release job, conditioned on non-tag pushes, that runs the snapshot + verify. More complex to gate correctly.

**Recommendation: Option B (separate workflow) is cleaner and lower risk.**

---

## Topic 8: AMFI Behavior Baseline (Why Ad-Hoc Signing Fixes the Kill)

[VERIFIED: golang/go/issues/42684, actions/runner-images/issues/8439, uwekorn.com 2024 article]

Apple Mobile File Integrity (AMFI) is a kernel extension that enforces code signing policy on macOS. The enforcement differs by architecture:

- **macOS Intel (x86_64):** AMFI allows unsigned binaries to run. Gatekeeper handles trust at the user-visible level, but the kernel itself does not kill unsigned x86_64 executables.
- **macOS Apple Silicon (arm64), macOS 11+:** AMFI enforces that ALL arm64 Mach-O executables must have a valid code signature — even a minimal ad-hoc signature. An arm64 binary without any signature gets `SIGKILL` (exit 9) on first execution. This is not a Gatekeeper prompt; it is a silent kernel kill.

**Why cross-compiled Go binaries get killed:** When GoReleaser runs on a Linux runner, `go build` produces a Mach-O arm64 binary. The Go linker on Linux does NOT apply an ad-hoc signature (only the macOS linker does this automatically via `ld64`). The resulting binary is unsigned and gets `Killed: 9` on Apple Silicon.

**The fix:** Running `codesign --force --sign -` on the binary (on a macOS machine) embeds an ad-hoc signature. AMFI accepts this as a valid signature and allows execution. The binary is still not notarized (no Gatekeeper trust chain), so downloaded binaries still trigger the quarantine warning — but they run.

---

## Topic 9: Homebrew Unaffected by Ad-Hoc Signing

[MEDIUM confidence — based on Homebrew behavior analysis; ASSUMED for exact mechanism]

When a user installs kinder via `brew install patrykquantumnomad/kinder/kinder`, Homebrew:
1. Downloads the release `.tar.gz` from GitHub
2. Extracts to the Cellar (`/opt/homebrew/Cellar/kinder/`)
3. Symlinks the binary into `/opt/homebrew/bin/`

**Why quarantine does not block the Homebrew install:**
Homebrew itself is a non-quarantined process. Files it downloads and places in the Cellar are written by Homebrew (not by a web browser or Download manager), and Homebrew explicitly handles quarantine removal for its installed files. CLI formula binaries placed in the Cellar do NOT receive `com.apple.quarantine` because Homebrew writes them via its own internal API, not via a quarantine-tagged download mechanism.

**Why ad-hoc signing does not regress Homebrew:**
The current kinder formula is a Homebrew cask using GoReleaser's `homebrew_casks:` config. Cask installs extract the pre-built binary from the release tarball. An ad-hoc-signed binary extracts correctly — the signature is embedded in the Mach-O binary itself (not the tarball). After extraction, Homebrew may apply its own re-signing for binaries it modifies (e.g., patching install paths). Since kinder is a pure-Go binary with no hardcoded paths, Homebrew does not modify it, so the ad-hoc signature survives installation intact.

**Result:** Homebrew users see no change. The binary runs on first use without Gatekeeper prompts or AMFI kills.

[ASSUMED: that Homebrew does not apply post-install patches that would invalidate the signature for this binary type]

---

## Topic 10: `xattr` Quarantine Guidance

[VERIFIED: xattr man page — confirmed on local macOS machine]

Users who download the kinder binary directly from GitHub Releases (not via Homebrew) will have `com.apple.quarantine` set on the downloaded file. Ad-hoc signing does NOT remove this attribute — it is applied by the web browser or `curl` at download time, not by the publisher.

**Correct install-guide wording:**
```
After downloading, remove the quarantine attribute before first run:
xattr -d com.apple.quarantine kinder
```

**`-d` vs `-c` flag:**
- `xattr -d com.apple.quarantine kinder` — deletes ONLY the `com.apple.quarantine` attribute. Leaves any other extended attributes (resource forks, etc.) intact. This is the targeted and correct command.
- `xattr -c kinder` — clears ALL extended attributes from the file. More aggressive; only needed if there are multiple quarantine-related attributes. Safe to use but unnecessary for typical downloads.

**Variant for recursive/directory:** `xattr -r -d com.apple.quarantine kinder` (for app bundles with sub-resources; not needed for a single CLI binary).

**Install guide should state:**
> "ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine kinder`"

This exactly matches SC3's required wording.

---

## Topic 11: Common Pitfalls

### Pitfall 1: Using `signs:` Instead of `builds[].hooks.post`

**What goes wrong:** The top-level `signs:` key in GoReleaser signs archive files (`.tar.gz`) or checksum files, not raw binaries. If you codesign the tarball, the extracted binary on the user's machine has no signature.

**Why it happens:** Developers confuse "signs" (artifact signing for distribution integrity, e.g., cosign/GPG) with Mach-O code signing.

**How to avoid:** Always use `builds[].hooks.post` with `codesign --force --sign -`. Confirm with `codesign -vvv` on the extracted binary (not the archive).

**Warning sign:** SC1/SC2 verify the binary in `dist/kinder_darwin_*/kinder`, not the tarball — if you're signing the tarball, the dist binary will fail `-vvv`.

### Pitfall 2: Re-Stripping or Modifying the Binary After Codesign

**What goes wrong:** Any operation that modifies binary bytes AFTER codesign invalidates the Mach-O signature block. Examples: UPX compression, `strip` command, binary patching, `upx` post-hook placed after codesign hook.

**Why it happens:** Pipelines add post-processing steps without realizing codesign must be last.

**How to avoid:** Codesign MUST be the last operation in `builds[].hooks.post`. If UPX is ever added, it must come BEFORE codesign. SC4 explicitly states "sign step is the LAST operation on each binary before archiving."

**Warning sign:** `codesign -vvv` returns non-zero after the full pipeline — the binary fails verification.

### Pitfall 3: Runner Not Changed (Still ubuntu-latest)

**What goes wrong:** `codesign` is not installed on ubuntu-latest. GoReleaser's post hook `sh -c '...; codesign ...'` silently no-ops or fails, depending on how the conditional is written. If the conditional checks `$GOOS` only via the shell (not the goreleaser template), the codesign call may be skipped entirely on Linux.

**Why it happens:** Forgetting to update `release.yml` `runs-on:` after updating `.goreleaser.yaml`.

**How to avoid:** Change `runs-on: ubuntu-latest` to `runs-on: macos-latest` in `release.yml`. Verify by checking codesign output in CI logs (`output: true` on the hook).

### Pitfall 4: GoReleaser v1 vs v2 Hook Syntax

**What goes wrong:** GoReleaser v1 used slightly different YAML for hooks. The current config is `version: 2`, so v2 syntax applies. In v2, hooks can be inline strings OR structured objects with `cmd:`, `dir:`, `env:`, `output:` fields.

**How to avoid:** Always use the structured form for hooks that need `output: true` for CI log visibility. Confirm `version: 2` at top of `.goreleaser.yaml` (it is set).

### Pitfall 5: CGO Cross-Compile Issue (Non-Issue for This Project)

**Concern:** Running goreleaser on a macOS runner to cross-compile Linux/Windows targets.

**Status: NOT a pitfall here.** `CGO_ENABLED=0` is set in the config. Pure-Go cross-compilation to linux/amd64, linux/arm64, windows/amd64 works from a macOS runner with no extra toolchain. [VERIFIED: goreleaser-action/issues/233]

**If CGO were enabled:** Cross-compilation from macOS to Linux would require a cross-toolchain (e.g., musl). Not applicable here.

### Pitfall 6: Snapshot Path Uses Version String

**What goes wrong:** The snapshot dist path includes the snapshot version: `dist/kinder_0.1.0-SNAPSHOT-abc1234_darwin_amd64_v1/kinder`, not `dist/kinder_darwin_amd64_v1/kinder`. CI verify scripts that hardcode the path fail.

**How to avoid:** Use `find dist/ -name kinder -path '*darwin_amd64*'` and `find dist/ -name kinder -path '*darwin_arm64*'` in the verify step, or set a custom `snapshot.version_template` and reference it explicitly.

### Pitfall 7: macOS Runner Billing Multiplier (Non-Blocking Observation)

**Note:** macOS runners consume GitHub Actions minutes at a 10x multiplier. The `release.yml` runs on tag push only — cost is per-release, not per-PR. A typical goreleaser release job takes 2-5 minutes. At 10x, this is 20-50 minutes consumed per release. For a public repo on the free tier, this is negligible. Document for awareness only; not a blocker.

### Pitfall 8: Snapshot Verification Workflow Trigger Scope

**What goes wrong:** If the snapshot-verify workflow triggers on every push/PR, macOS runner minutes accumulate quickly during development.

**How to avoid:** Gate the snapshot-verify workflow on `push: branches: [main]` only (not `pull_request`), or add a `paths:` filter to trigger only when `.goreleaser.yaml` or `release.yml` changes.

### Pitfall 9: `codesign -vvv` Outputs to stderr

**What goes wrong:** CI log parsers that check stdout for "satisfies its Designated Requirement" find nothing; the string is on stderr.

**How to avoid:** Use `codesign -vvv dist/.../kinder 2>&1` in the verify step, or check exit code only (0 = passes, non-zero = fails). The SC wording says "returns ... in CI" — exit code 0 is the primary signal.

---

## Topic 12: Files Modified (Planner Inventory)

| File | Change |
|------|--------|
| `.goreleaser.yaml` | Add `builds[].hooks.post` with darwin-conditional codesign; optionally add `-s` to ldflags |
| `.github/workflows/release.yml` | Change `runs-on: ubuntu-latest` → `runs-on: macos-latest` |
| `.github/workflows/snapshot-sign-verify.yml` | NEW: snapshot build + codesign verify on main branch push |
| `kinder-site/src/content/docs/installation.md` | Add sc3-required wording about ad-hoc signing and xattr |
| `.planning/PROJECT.md` | Append signing decision to Key Decisions table |
| `.planning/REQUIREMENTS.md` | Mark DIST-01 complete after phase closes |

---

## Topic 13: Plan Split Recommendation

**Recommendation: TWO plans.**

**Plan A: GoReleaser + CI Runner (release plumbing)**
- Edit `.goreleaser.yaml`: add `builds[].hooks.post` codesign hook (and optionally add `-s` to ldflags)
- Edit `.github/workflows/release.yml`: change `runs-on: ubuntu-latest` → `runs-on: macos-latest`
- Commit: 1-2 atomic commits

**Plan B: Snapshot Verify Workflow + Docs**
- Create `.github/workflows/snapshot-sign-verify.yml`
- Edit `kinder-site/src/content/docs/installation.md` with SC3 wording
- Edit `.planning/PROJECT.md` Key Decisions table
- Commit: 2-3 atomic commits

**Why two plans instead of one:**
- Plan A is purely pipeline config — reviewable in isolation, no new workflow complexity
- Plan B introduces a new workflow file that needs careful trigger/gate design
- Separating them allows the pipeline change to be verified by hand (goreleaser build --snapshot locally or in CI) before the automated verify harness is wired up
- Each plan stays under 4 atomic commits — well within the "reviewable" target
- SC1/SC2 require CI verification — Plan B delivers this; it has a clear dependency on Plan A being merged first

**If reviewer insists on single plan:** All edits fit in ~5 commits (goreleaser.yaml, release.yml, snapshot-sign-verify.yml, installation.md, PROJECT.md) — still manageable, but Plan B's workflow is harder to review without Plan A's context.

---

## Standard Stack

### Core Tools

| Tool | Version | Purpose | Source |
|------|---------|---------|--------|
| `codesign` | System (macOS Xcode CLT) | Ad-hoc sign Mach-O binaries | macOS preinstalled |
| GoReleaser | v2.14.1 (current in repo) | Build pipeline, archive, release | In use |
| goreleaser-action | v7 (pinned ~> v2) | GitHub Actions integration | In use |
| `macos-latest` runner | maps to macos-15, Intel x64 | CI runner with codesign | GitHub Actions |

### No New Go Dependencies

Phase 54 touches only CI config and documentation — zero new Go packages.

---

## Architecture Patterns

### System Architecture Diagram

```
GitHub Actions trigger (tag push v*)
          |
          v
    macos-latest runner
          |
          v
    goreleaser release --clean
          |
    ┌─────┴──────────────────────────────────────────┐
    │  GoReleaser Pipeline                           │
    │                                                │
    │  [1] go build (all targets)                   │
    │       ├── linux/amd64    → dist/              │
    │       ├── linux/arm64    → dist/              │
    │       ├── darwin/amd64   → dist/  ─┐          │
    │       ├── darwin/arm64   → dist/  ─┤          │
    │       └── windows/amd64  → dist/  │          │
    │                                    │          │
    │  [2] builds[].hooks.post (per target)        │
    │       ├── linux targets: sh conditional → no-op│
    │       ├── windows target: sh conditional → no-op│
    │       ├── darwin/amd64: codesign --force --sign -│
    │       └── darwin/arm64: codesign --force --sign -│
    │                                                │
    │  [3] Archive pipe                             │
    │       ├── kinder_*_linux_*.tar.gz             │
    │       ├── kinder_*_darwin_amd64.tar.gz        │  ← signed binary inside
    │       ├── kinder_*_darwin_arm64.tar.gz        │  ← signed binary inside
    │       └── kinder_*_windows_amd64.zip          │
    │                                                │
    │  [4] Checksum + GitHub Release publish        │
    └────────────────────────────────────────────────┘
                     |
                     v
             Homebrew tap update
             (homebrew_casks: config)
```

Separate verification workflow (non-tag push, main branch):

```
push to main
      |
      v
snapshot-sign-verify.yml (macos-latest)
      |
      v
goreleaser release --snapshot --clean --skip=publish
      |
      v
find dist/ -name kinder -path '*darwin*'
      |
      v
codesign -vvv <path>  [exit 0 = PASS]
```

### Recommended Project Structure (no changes to Go source)

```
.github/
  workflows/
    release.yml          # MODIFIED: runs-on macos-latest
    snapshot-sign-verify.yml  # NEW: snapshot + codesign verify
.goreleaser.yaml         # MODIFIED: builds[].hooks.post
kinder-site/src/content/docs/
    installation.md      # MODIFIED: SC3 wording
```

### Pattern: Darwin-Conditional Post Hook

```yaml
# Source: goreleaser.com/customization/builds/hooks/
# Template variable .Os resolves to GOOS for each target
builds:
  - id: kinder
    main: .
    binary: kinder
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    flags:
      - -trimpath
    ldflags:
      - -buildid=
      - -w
      # Consider adding -s here to match DIST-01 wording
      - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .FullCommit }}
    mod_timestamp: "{{ .CommitTimestamp }}"
    hooks:
      post:
        - cmd: sh -c 'if [ "{{ .Os }}" = "darwin" ]; then codesign --force --sign - "{{ .Path }}"; fi'
          output: true
```

### Pattern: Snapshot + Verify Workflow

```yaml
# .github/workflows/snapshot-sign-verify.yml
name: Sign Verify (macOS snapshot)

on:
  push:
    branches:
      - main
    paths:
      - .goreleaser.yaml
      - .github/workflows/release.yml
      - .github/workflows/snapshot-sign-verify.yml

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

      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --snapshot --clean --skip=publish
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Verify darwin/amd64 signature
        run: |
          BIN=$(find dist/ -name kinder -path '*darwin_amd64*' | head -1)
          echo "Verifying: $BIN"
          codesign -vvv "$BIN"

      - name: Verify darwin/arm64 signature
        run: |
          BIN=$(find dist/ -name kinder -path '*darwin_arm64*' | head -1)
          echo "Verifying: $BIN"
          codesign -vvv "$BIN"
```

### Pattern: Release.yml Runner Change

```yaml
# .github/workflows/release.yml
jobs:
  release:
    runs-on: macos-latest   # CHANGED from ubuntu-latest
    # ... rest unchanged
```

### Anti-Patterns to Avoid

- **Using `signs:` for Mach-O signing:** `signs:` operates on archives, not raw binaries. The extracted binary is unsigned.
- **UPX or strip AFTER codesign:** Any byte modification after codesign invalidates the Mach-O signature.
- **Hardcoding snapshot version in verify path:** Snapshot version includes commit hash; use `find` glob instead.
- **Running codesign on linux/windows targets:** The shell conditional (`if [ "{{ .Os }}" = "darwin" ]`) prevents this.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Binary code signing | Custom signing script | `codesign --force --sign -` (system tool) | System tool handles Mach-O format, LC_CODE_SIGNATURE, SHA256 sealing |
| Quarantine removal docs | Custom explanation | Standard `xattr -d com.apple.quarantine` | Well-known Apple mechanism; deviate and users get confused |
| Archive integrity | Custom checksum | GoReleaser's `checksum:` (already configured) | sha256 checksum already in pipeline |

---

## Code Examples

### Complete `.goreleaser.yaml` builds entry (verified pattern)

```yaml
# Source: goreleaser.com/customization/builds/hooks/ + goreleaser.com/customization/builds/go/
builds:
  - id: kinder
    main: .
    binary: kinder
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    flags:
      - -trimpath
    ldflags:
      - -buildid=
      - -w
      - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .FullCommit }}
    mod_timestamp: "{{ .CommitTimestamp }}"
    hooks:
      post:
        - cmd: sh -c 'if [ "{{ .Os }}" = "darwin" ]; then codesign --force --sign - "{{ .Path }}"; fi'
          output: true
```

### codesign verification (exact SC1/SC2 assertion)

```bash
# After snapshot build, verify both architectures:
BIN_AMD64=$(find dist/ -name kinder -path '*darwin_amd64*' | head -1)
BIN_ARM64=$(find dist/ -name kinder -path '*darwin_arm64*' | head -1)

codesign -vvv "$BIN_AMD64" 2>&1  # exit 0 + "satisfies its Designated Requirement"
codesign -vvv "$BIN_ARM64" 2>&1  # exit 0 + "satisfies its Designated Requirement"
```

### Install guide addition (SC3 wording)

```markdown
:::caution[macOS direct download]
kinder binaries are **ad-hoc signed** (not notarized). Homebrew install is unaffected.
If you download the binary directly from GitHub Releases, macOS Gatekeeper will quarantine it.
Remove the quarantine attribute before first run:

    xattr -d com.apple.quarantine kinder

Ad-hoc signing prevents AMFI from killing the binary on Apple Silicon,
but does **not** bypass Gatekeeper quarantine for downloaded files.
:::
```

---

## Runtime State Inventory

> Not a rename/refactor phase. No runtime state to inventory. Skipped.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `codesign` | darwin ad-hoc signing | ✓ (local macOS) | System (Xcode CLT) | None — macOS only |
| `goreleaser` | Build pipeline | n/a (CI only via action) | v2.14.1 | — |
| `macos-latest` runner | codesign in CI | ✓ (GitHub-hosted) | macos-15 (Intel x64) | macos-15 explicit |
| `goreleaser-action@v7` | CI integration | ✓ (already in release.yml) | ~> v2 | — |

**Missing dependencies with no fallback:**
- None that block execution. The `codesign` tool is preinstalled on all macOS runners via Xcode CLT.

---

## Validation Architecture

> Note: `.planning/config.json` does not exist; treating `nyquist_validation` as enabled.

### Test Framework

Phase 54 is CI-config + docs only. No Go test framework applies. Validation is via CI workflow execution.

| Property | Value |
|----------|-------|
| Framework | GitHub Actions (CI job) |
| Config file | `.github/workflows/snapshot-sign-verify.yml` (new) |
| Quick run | `goreleaser build --single-target --snapshot` (local, darwin only) |
| Full suite | `goreleaser release --snapshot --clean --skip=publish` then `codesign -vvv` (CI) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DIST-01 (SC1) | darwin/amd64 binary satisfies codesign -vvv | integration | `codesign -vvv $(find dist/ -name kinder -path '*darwin_amd64*')` | ❌ Wave 0 |
| DIST-01 (SC2) | darwin/arm64 binary satisfies codesign -vvv | integration | `codesign -vvv $(find dist/ -name kinder -path '*darwin_arm64*')` | ❌ Wave 0 |
| DIST-01 (SC3) | install.md contains required wording | manual | grep check or doc review | ❌ Wave 0 |
| DIST-01 (SC4) | sign is last op before archive | structural | inspect goreleaser pipeline order | ❌ (enforced by hook placement) |

### Wave 0 Gaps

- [ ] `.github/workflows/snapshot-sign-verify.yml` — covers SC1 and SC2 automated verification
- [ ] Update `installation.md` — covers SC3 (doc content; no automated test)
- [ ] `builds[].hooks.post` in `.goreleaser.yaml` — covers SC4 (correct hook placement)

---

## Security Domain

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | — |
| V5 Input Validation | no | — |
| V6 Cryptography | no (ad-hoc = no private key) | Ad-hoc = hash-only, no certificate |

**Threat model note:** Ad-hoc signing does NOT provide tamper protection against a supply-chain attacker who can modify the binary after signing, because ad-hoc has no trusted signer identity. Notarization (Phase DIST-03, deferred) would provide that. For v2.4, ad-hoc signing's sole purpose is to satisfy AMFI's "binary must have some signature" enforcement on arm64. This is documented in SC3 ("not notarized").

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Ubuntu runner for GoReleaser | macOS runner for GoReleaser | Phase 54 | +10x CI billing multiplier; codesign now available |
| Unsigned Mach-O (cross-compiled) | Ad-hoc signed Mach-O | Phase 54 | Fixes AMFI kill on Apple Silicon |
| No install guidance for macOS downloads | xattr + ad-hoc signing docs | Phase 54 | Users know what to do on first run |
| `signs:` for notarization (DIST-03, deferred) | `builds[].hooks.post` for ad-hoc | v2.4 scope | Ad-hoc is cheaper and sufficient for AMFI |

**Deprecated/outdated:**
- `gon` (mitchellh/gon): Old tool for macOS signing/notarization via goreleaser. Now superseded by goreleaser's native notarize and post-hook patterns. Not needed for ad-hoc signing.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Adding `-s` to ldflags alongside `-w` is acceptable for DIST-01 compliance | Topic 2 | If developer wants `-w` only, DIST-01 wording needs clarification |
| A2 | Homebrew formula install does not apply quarantine xattr to the extracted binary | Topic 9 | If Homebrew quarantines, Homebrew users also need xattr; SC3 wording may need to change |
| A3 | GoReleaser hook `if:` field is NOT available in `builds[].hooks.post` (only global hooks) | Topic 3 | If `if:` is available in build hooks, the shell conditional can be simplified |
| A4 | `macos-latest` in 2026 is macos-15 (Intel x64) with codesign preinstalled | Topic 5 | If macos-latest changes mapping again, runner label may need explicit pinning |

---

## Open Questions

1. **Should `-s` be added to ldflags?**
   - What we know: DIST-01 says "after `-ldflags="-s -w"` strip"; current config has only `-w`
   - What's unclear: Was DIST-01 written assuming `-s` would be added, or was it written describing existing state?
   - Recommendation: Add `-s` in Plan A. It reduces binary size and is standard for release builds. If developer objects, revert; `-w` alone is fine for AMFI.

2. **Should `macos-latest` be pinned to `macos-15` explicitly?**
   - What we know: `macos-latest` maps to macos-15 as of mid-2025. macos-26 is GA but not yet default.
   - What's unclear: Will macos-latest shift to macos-26 before/during this phase?
   - Recommendation: Use `macos-latest` (not pinned). If a pinned version is desired, use `macos-15`. Avoid `macos-26` until it is the stable default.

3. **Does the snapshot-verify workflow need to run on PR or main-branch-push only?**
   - What we know: macOS runner minutes are 10x cost; full goreleaser snapshot builds take 3-5 minutes
   - Recommendation: Main-branch push only, with `paths:` filter on goreleaser.yaml and workflow files.

---

## Sources

### Primary (HIGH confidence)
- `goreleaser/goreleaser` (Context7 /goreleaser/goreleaser) — builds hooks, signs, template variables, pipeline order
- `goreleaser.com/customization/builds/hooks/` — build hook fields, template variables
- `goreleaser.com/customization/builds/go/` — .Os, .Arch, .Path template variables
- `goreleaser.com/customization/templates/` — template functions available
- `goreleaser.com/customization/snapshots/` — snapshot build command
- `developer.apple.com/library/archive/technotes/tn2206/` — codesign -vvv output, "satisfies its Designated Requirement"
- `github.com/actions/runner-images/issues/12520` — macos-latest maps to macos-15
- xattr man page (local macOS) — `-d` vs `-c` flag semantics

### Secondary (MEDIUM confidence)
- `goreleaser.com/customization/general/hooks/` — `if:` field in global hooks (since v2.7)
- `goreleaser.com/cookbooks/builds-complex-envs/` — `.Os` template variable in build env conditionals
- `github.blog/changelog/2026-02-26-macos-26-is-now-generally-available/` — macos-26 GA, architecture details
- `goreleaser/discussions/3183` — `_v1` suffix explanation for amd64 GOAMD64
- `golang/go/issues/42684` — arm64 requires codesigning for Go binaries
- `github.com/Homebrew/brew/issues/9082` — Homebrew code signing on Apple Silicon

### Tertiary (LOW confidence — marked ASSUMED in text)
- Community analysis of Homebrew quarantine behavior for formula binaries

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — GoReleaser v2 docs verified via Context7 and official site
- Architecture (pipeline order): HIGH — pipeline.go source verified
- Template variables (.Os, .Path): HIGH — verified via official docs
- codesign -vvv output string: HIGH — verified via Apple TN2206
- AMFI behavior: MEDIUM — verified via golang/go issue and community reports
- Homebrew quarantine behavior: MEDIUM-LOW — inferred from Homebrew source discussions

**Research date:** 2026-05-12
**Valid until:** 2026-08-12 (90 days; GoReleaser releases are frequent but v2 hook API is stable)
