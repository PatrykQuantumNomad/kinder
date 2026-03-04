# Phase 36: Homebrew Tap - Research

**Researched:** 2026-03-04
**Domain:** GoReleaser homebrew_casks, Homebrew tap repository structure, GitHub PAT token scoping, Astro/Starlight documentation update
**Confidence:** HIGH — GoReleaser homebrew_casks docs fetched directly from goreleaser.com; Homebrew tap structure from official docs.brew.sh; token requirements verified via multiple community sources and GoReleaser official discussion

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| BREW-01 | `homebrew-kinder` tap repository exists under PatrykQuantumNomad | Homebrew naming convention `homebrew-{name}` required; `Casks/` subdirectory required; minimal structure documented below |
| BREW-02 | GoReleaser publishes Cask to tap repo on tagged release via HOMEBREW_TAP_TOKEN | `homebrew_casks[].repository.token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"` pattern; PAT with Contents: write on tap repo; secret added to kinder repo; env var passed in release.yml |
| BREW-03 | User can install kinder via `brew install patrykquantumnomad/kinder/kinder` | Homebrew one-line install with full tap path verified against Homebrew docs; requires tap named `homebrew-kinder` under `PatrykQuantumNomad` |
| SITE-01 | Installation page updated with Homebrew install instructions alongside make install | `kinder-site/src/content/docs/installation.md` is the target file; Astro/Starlight site; update curl download URLs to GoReleaser archive naming too |
</phase_requirements>

---

## Summary

Phase 36 has four distinct deliverables that together enable one-command macOS installation. Three are infrastructure/config work (BREW-01 through BREW-03), and one is a documentation update (SITE-01).

The core technical mechanism is GoReleaser's `homebrew_casks` section (introduced in v2.10, available in the project's current v2.14.1). When a tagged release is pushed, GoReleaser builds binaries, creates archives, and then pushes a generated `Casks/kinder.rb` file to the `homebrew-kinder` tap repository. The generated cask contains platform-specific download URLs (darwin_arm64 and darwin_amd64) with SHA-256 checksums computed from the actual release archives. Users then install with `brew install patrykquantumnomad/kinder/kinder`.

The critical constraint is authentication: GoReleaser runs in the `kinder` repository's CI, but needs to write to the `homebrew-kinder` repository. The default `GITHUB_TOKEN` only has access to the repo it runs in. A Personal Access Token (PAT) with Contents: write permission on `homebrew-kinder` must be created, stored as secret `HOMEBREW_TAP_TOKEN` in the `kinder` repo, and referenced in both `.goreleaser.yaml` and `release.yml`. The existing release workflow only needs one new `env:` line added.

**Primary recommendation:** Create the `PatrykQuantumNomad/homebrew-kinder` tap repo with correct structure, add `homebrew_casks` section to `.goreleaser.yaml`, add `HOMEBREW_TAP_TOKEN` to `release.yml`, create the PAT secret — in that order.

---

## Standard Stack

### Core

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| GoReleaser OSS | v2.14.1 (already in project) | Generates and publishes the Homebrew Cask `Casks/kinder.rb` to the tap repo on release | Already used in Phase 35; `homebrew_casks` section added to existing `.goreleaser.yaml` — no new tooling needed |
| Homebrew tap repo | N/A (GitHub repo) | `PatrykQuantumNomad/homebrew-kinder` — stores the generated Cask file | Standard Homebrew convention; `homebrew-` prefix required for short-form `brew tap` command |
| GitHub PAT | Classic or fine-grained | Authenticates GoReleaser to push to the tap repo from a different repo's CI | Default `GITHUB_TOKEN` is repo-scoped; PAT required for cross-repo writes |

### Supporting

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| Astro/Starlight | ^5.6.1 / ^0.37.6 (already in project) | Renders the kinder-site installation page | Already used; only `installation.md` content changes |
| `brew install --verbose` | (brew CLI) | Local verification of cask installation end-to-end | Use after first real tagged release to verify BREW-03 |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `homebrew_casks` in GoReleaser | Manually maintain `Casks/kinder.rb` in the tap repo | Manual approach is error-prone (wrong SHA256 on every release); GoReleaser automates this entirely |
| Classic PAT (repo scope) | Fine-grained PAT (Contents: write on specific repo) | Fine-grained PAT is more secure (narrower scope); both work; fine-grained preferred |
| `skip_upload: auto` | `skip_upload: false` (default) | `auto` skips upload for pre-release tags (v1.0.0-rc1); `false` always uploads; use `auto` to avoid broken pre-release casks |

**No new software to install** — GoReleaser v2.14.1 is already installed in CI. The only new artifact is the tap repository on GitHub.

---

## Architecture Patterns

### Recommended Structure: Two Repositories

```
PatrykQuantumNomad/kinder              (existing main repo)
├── .goreleaser.yaml                   MODIFIED — add homebrew_casks section
└── .github/workflows/release.yml     MODIFIED — add HOMEBREW_TAP_TOKEN env var

PatrykQuantumNomad/homebrew-kinder    NEW tap repository
├── README.md                         Minimal README explaining this is a tap
└── Casks/
    └── kinder.rb                     AUTO-GENERATED by GoReleaser on each release
```

The `Casks/` directory and `kinder.rb` file are created and maintained entirely by GoReleaser. The planner does NOT create `kinder.rb` by hand.

### Pattern 1: homebrew_casks Section in .goreleaser.yaml

**What:** Minimal `homebrew_casks` stanza appended to the existing `.goreleaser.yaml`.
**When to use:** Append after the existing `release:` section.

```yaml
# Source: https://goreleaser.com/customization/homebrew_casks/
# Verified against GoReleaser v2.14.1

homebrew_casks:
  - name: kinder
    directory: Casks
    homepage: "https://kinder.patrykgolabek.dev"
    description: "kind, but with everything you actually need."
    skip_upload: auto   # Prevents uploading cask for pre-release tags (v1.0.0-rc1)
    repository:
      owner: PatrykQuantumNomad
      name: homebrew-kinder
      branch: main
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com
    commit_msg_template: "Brew cask update for {{ .ProjectName }} version {{ .Tag }}"
```

**What this generates:** GoReleaser will create/update `Casks/kinder.rb` in the `homebrew-kinder` repo with content like:

```ruby
# Source: generated by goreleaser v2.14.1
cask "kinder" do
  version "0.4.1"

  on_macos do
    on_arm do
      url "https://github.com/PatrykQuantumNomad/kinder/releases/download/v0.4.1/kinder_0.4.1_darwin_arm64.tar.gz"
      sha256 "<computed-from-release-artifact>"
    end
    on_intel do
      url "https://github.com/PatrykQuantumNomad/kinder/releases/download/v0.4.1/kinder_0.4.1_darwin_amd64.tar.gz"
      sha256 "<computed-from-release-artifact>"
    end
  end

  binary "kinder"

  homepage "https://kinder.patrykgolabek.dev"
  desc "kind, but with everything you actually need."

  livecheck do
    skip "Auto-generated on release."
  end
end
```

Note: GoReleaser uses the archive names from the existing `archives[].name_template` in `.goreleaser.yaml`. Since Phase 35 set `name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`, the URLs will be `kinder_VERSION_darwin_arm64.tar.gz` and `kinder_VERSION_darwin_amd64.tar.gz`. This matches what GoReleaser's cask generator expects.

### Pattern 2: release.yml Token Addition

**What:** Add one `env:` line to the existing `goreleaser-action` step in `.github/workflows/release.yml`.
**When to use:** The release.yml already works from Phase 35. Only the `env:` block changes.

```yaml
# Source: https://goreleaser.com/ci/actions/
# Modified from Phase 35 release.yml — add HOMEBREW_TAP_TOKEN

      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}  # PAT with Contents:write on homebrew-kinder
```

### Pattern 3: Tap Repository Initialization

**What:** Minimal `homebrew-kinder` repository structure.
**When to use:** Create once before first tagged release.

The `homebrew-kinder` repo needs:
1. Named exactly `homebrew-kinder` (not `kinder-tap` or `homebrew-tap`) — the `homebrew-` prefix enables `brew tap patrykquantumnomad/kinder` short form
2. A `Casks/` directory at the root — GoReleaser writes `Casks/kinder.rb` there by default
3. Default branch named `main` (matches `repository.branch: main` in goreleaser config)
4. At least one commit before GoReleaser can push (GitHub won't let you push to a repo with zero commits)

Create the repo with a README.md initial commit via GitHub UI or CLI. GoReleaser will add `Casks/kinder.rb` on first release.

```bash
# Source: https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap
# Verify the tap works after setup:
brew tap patrykquantumnomad/kinder
brew install patrykquantumnomad/kinder/kinder

# Or in one command (auto-taps):
brew install patrykquantumnomad/kinder/kinder
```

### Pattern 4: Installation Page Update (SITE-01)

**What:** Update `kinder-site/src/content/docs/installation.md` to add Homebrew install section and fix archive download URLs.
**When to use:** The file already exists. Two updates needed:

1. Add a new "Homebrew (macOS)" section as the first/preferred method
2. Update the existing curl download URLs — Phase 35 changed the archive naming convention from `kinder-darwin-arm64` (no extension) to `kinder_VERSION_darwin_arm64.tar.gz` (tar.gz with version). The current installation.md has the old pre-GoReleaser URLs.

```markdown
## Homebrew (macOS)

```sh
brew install patrykquantumnomad/kinder/kinder
```

This installs a pre-built binary for your architecture (Apple Silicon or Intel). No Go toolchain required.
```

The updated curl URLs for non-Homebrew install should reference the GoReleaser archive format:
```sh
# macOS Apple Silicon
curl -sSL https://github.com/PatrykQuantumNomad/kinder/releases/latest/download/kinder_VERSION_darwin_arm64.tar.gz | tar xz
chmod +x kinder && sudo mv kinder /usr/local/bin/
```

Note: `releases/latest/download/` works without knowing the version number, but the filename must match the archive name template. The site should use the GoReleaser release naming convention (`kinder_VERSION_OS_ARCH.tar.gz`), not the old cross.sh naming (`kinder-OS-ARCH`).

### Anti-Patterns to Avoid

- **Using `brews:` instead of `homebrew_casks:`:** `brews:` is deprecated since GoReleaser v2.10 and will be removed in v3. Use `homebrew_casks:` exclusively.
- **Placing the cask in `Formula/` directory:** Homebrew casks must be in `Casks/`. Placing a cask in `Formula/` produces install errors.
- **Using `GITHUB_TOKEN` for the tap push:** The default token only has access to the repo running the workflow. GoReleaser will get a 403 when trying to push to `homebrew-kinder`. Must use a PAT.
- **Creating `Casks/kinder.rb` manually:** Let GoReleaser generate this. Hand-crafting the cask leads to SHA256 mismatches and stale URLs.
- **Forgetting `skip_upload: auto`:** Without this, GoReleaser will push a cask for pre-release tags (e.g., `v2.0.0-rc1`) pointing to pre-release archives that may be removed. `auto` skips upload for any tag containing a pre-release indicator.
- **Setting `branch: main` but having no initial commit:** GitHub rejects pushes to branch-less repos. Initialize the tap repo with at least one commit before the first release.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SHA256 checksums in Cask | Manually update kinder.rb with `shasum -a 256` after each release | GoReleaser `homebrew_casks` section | GoReleaser computes checksums from the actual release artifacts during the release run; manual process fails on every release |
| Homebrew formula `.rb` file | Write and maintain a custom Ruby formula | GoReleaser `homebrew_casks` with `directory: Casks` | Formula must match exact archive URLs and hashes; one wrong byte causes `brew install` to fail silently or loudly |
| Cross-repo git push from CI | GitHub Actions step that clones tap repo and git pushes | GoReleaser tap publish | GoReleaser handles auth, git clone, file update, commit, and push atomically; race conditions and auth complexity avoided |
| Brew tap GitHub Action | Third-party "update-homebrew-tap" actions | GoReleaser native support | GoReleaser has first-class homebrew_casks support — no third-party action needed |

**Key insight:** GoReleaser treats the tap update as part of the release pipeline — SHA256 hashes are computed from the same artifacts being uploaded to GitHub Release. This means the cask is always consistent with the published binaries.

---

## Common Pitfalls

### Pitfall 1: GITHUB_TOKEN Cannot Write to homebrew-kinder Repository

**What goes wrong:** GoReleaser push to `homebrew-kinder` fails with a 403 Forbidden error, even though `GITHUB_TOKEN` has `contents: write` permission in `.github/workflows/release.yml`.

**Why it happens:** `GITHUB_TOKEN` is scoped to the repository where the Actions workflow runs (`kinder`). It cannot write to `homebrew-kinder` regardless of permissions declared in the workflow.

**How to avoid:** Create a GitHub Personal Access Token (PAT) with Contents: write permission specifically on `homebrew-kinder`. Add it as secret `HOMEBREW_TAP_TOKEN` in the `kinder` repository. Reference it in `.goreleaser.yaml` as `token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"` and expose it in `release.yml` as `HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}`.

**Warning signs:** Release workflow completes but `homebrew-kinder` has no new commit. CI logs show `403` or `push declined` from the GoReleaser tap publish step.

### Pitfall 2: Tap Repo Has Wrong Name or Missing Casks Directory

**What goes wrong:** `brew install patrykquantumnomad/kinder/kinder` fails with "Error: No available formula or cask with the name 'kinder'" even after a successful release.

**Why it happens:** Two causes:
1. Tap repo not named `homebrew-kinder` — Homebrew expands `patrykquantumnomad/kinder` to `github.com/patrykquantumnomad/homebrew-kinder`. If the repo is named differently, the tap command silently points to a non-existent repo.
2. `kinder.rb` placed in wrong directory — Homebrew looks in `Casks/` for casks; `Formula/` only for formulas.

**How to avoid:** Name the tap repo exactly `homebrew-kinder`. Keep `directory: Casks` in `.goreleaser.yaml`. After first release, verify with `brew tap patrykquantumnomad/kinder && ls $(brew --repository)/Library/Taps/patrykquantumnomad/homebrew-kinder/Casks/`.

**Warning signs:** `brew tap patrykquantumnomad/kinder` succeeds but `brew install patrykquantumnomad/kinder/kinder` fails.

### Pitfall 3: Archive URL Mismatch Between Cask and Actual Release Asset

**What goes wrong:** `brew install` downloads successfully but `sha256` verification fails, or brew reports "artifact not found".

**Why it happens:** The cask URL is based on GoReleaser's `archives[].name_template`. If the name template is changed between the `.goreleaser.yaml` commit and the release, the cask URL will reference a filename that doesn't exist in the GitHub Release.

**How to avoid:** Do not change `archives[].name_template` in Phase 36. The existing Phase 35 template `"{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"` produces `kinder_VERSION_darwin_arm64.tar.gz`, which is what the cask generator will reference. Verify after first release by checking the actual filename in the GitHub Release assets against the URL in `Casks/kinder.rb`.

**Warning signs:** `brew install` shows download error or SHA256 mismatch. Check `brew --verbose install patrykquantumnomad/kinder/kinder` for the exact URL being fetched.

### Pitfall 4: Pre-release Tags Publish Broken Casks

**What goes wrong:** Tag `v2.0.0-rc1` triggers a release, GoReleaser publishes a cask pointing to pre-release archives. If the pre-release is later deleted, the cask becomes broken for all users who tapped.

**Why it happens:** Without `skip_upload: auto`, GoReleaser always pushes the cask on any release.

**How to avoid:** Set `skip_upload: auto` in `homebrew_casks`. GoReleaser detects pre-release indicators in the tag (`-rc`, `-alpha`, `-beta`) and skips the tap push for those tags.

**Warning signs:** Cask file in tap repo has URL pointing to a release that no longer exists.

### Pitfall 5: installation.md Still References Old Pre-GoReleaser URLs

**What goes wrong:** After Phase 36, the site shows curl commands with the old cross.sh archive naming (`kinder-darwin-arm64`, no extension, no version number) which no longer exist in GitHub Releases — GoReleaser archives are `kinder_VERSION_darwin_arm64.tar.gz`.

**Why it happens:** `installation.md` was written before Phase 35 changed the release artifact naming.

**How to avoid:** Update `installation.md` in SITE-01 to reflect GoReleaser's archive naming convention. Use `releases/latest/download/` with the correct filename pattern. Also add the Homebrew section as the primary macOS install method.

**Warning signs:** curl commands on the installation page return 404 from GitHub.

### Pitfall 6: Tap Repo Has No Initial Commit

**What goes wrong:** GoReleaser fails to push `Casks/kinder.rb` because the tap repo has no commits (GitHub treats it as a branch-less repo that doesn't exist yet).

**Why it happens:** Creating a new GitHub repo without initializing it leaves the default branch in a non-existent state.

**How to avoid:** When creating `homebrew-kinder`, check "Add a README file" in the GitHub UI (or `gh repo create --readme`). This creates an initial commit on `main` before GoReleaser tries to push.

**Warning signs:** GoReleaser tap push step errors with `repository not found` or `fatal: could not read Username`.

---

## Code Examples

### Complete homebrew_casks Addition to .goreleaser.yaml

```yaml
# Source: https://goreleaser.com/customization/homebrew_casks/
# Append after existing release: section in .goreleaser.yaml

homebrew_casks:
  - name: kinder
    directory: Casks
    homepage: "https://kinder.patrykgolabek.dev"
    description: "kind, but with everything you actually need."
    skip_upload: auto
    repository:
      owner: PatrykQuantumNomad
      name: homebrew-kinder
      branch: main
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com
    commit_msg_template: "Brew cask update for {{ .ProjectName }} version {{ .Tag }}"
```

### Updated .github/workflows/release.yml (diff view)

```yaml
# Source: existing Phase 35 release.yml
# Only change: add HOMEBREW_TAP_TOKEN to env block

      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

### Homebrew Tap Repository Creation (gh CLI)

```bash
# Source: https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap
# Create the tap repo with an initial commit so GoReleaser can push to it

gh repo create PatrykQuantumNomad/homebrew-kinder \
  --public \
  --description "Homebrew tap for kinder" \
  --add-readme

# Verify the Casks directory path after GoReleaser runs its first release:
brew tap patrykquantumnomad/kinder
ls "$(brew --repository)/Library/Taps/patrykquantumnomad/homebrew-kinder/Casks/"
```

### GitHub PAT Creation (fine-grained)

```
# Source: https://github.com/settings/tokens?type=beta
# Steps:
# 1. GitHub Settings → Developer settings → Personal access tokens → Fine-grained tokens
# 2. Resource owner: PatrykQuantumNomad
# 3. Repository access: Only select repositories → homebrew-kinder
# 4. Repository permissions: Contents → Read and write
# 5. Copy the token value (shown only once)
# 6. In kinder repo: Settings → Secrets and variables → Actions → New repository secret
#    Name: HOMEBREW_TAP_TOKEN
#    Value: [paste token]
```

### Updated installation.md Homebrew Section

```markdown
## Homebrew (macOS)

```sh
brew install patrykquantumnomad/kinder/kinder
```

This installs a pre-built binary for your architecture (Apple Silicon or Intel). No Go toolchain required.

## Download Pre-built Binary

Pre-built binaries are available for macOS, Linux, and Windows from
[GitHub Releases](https://github.com/PatrykQuantumNomad/kinder/releases).

### macOS (Apple Silicon)

```sh
curl -sSL https://github.com/PatrykQuantumNomad/kinder/releases/latest/download/kinder_darwin_arm64.tar.gz | tar xz
chmod +x kinder && sudo mv kinder /usr/local/bin/
```
```

Note: `releases/latest/download/` does not support version-in-filename redirects natively. The installation page may need to use the GitHub releases page link and instruct users to download the correct file, or use the GitHub releases API for "latest". Verify the actual filename returned by GoReleaser's archive naming before finalizing the curl instructions.

### Verification Commands After First Release

```bash
# Source: https://docs.brew.sh/Taps
# Verify the tap is set up correctly
brew tap patrykquantumnomad/kinder

# Install and verify kinder works
brew install patrykquantumnomad/kinder/kinder
kinder version

# Verify tap commit was created (per success criteria)
gh api repos/PatrykQuantumNomad/homebrew-kinder/commits --jq '.[0] | {sha: .sha, message: .commit.message}'
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `brews:` section for Homebrew formulas | `homebrew_casks:` for Homebrew Casks | GoReleaser v2.10 (~Feb 2025) | `brews:` is deprecated, removed in v3; use `homebrew_casks:` |
| Homebrew Formulas (Formula/ dir) for pre-built binaries | Homebrew Casks (Casks/ dir) | Homebrew policy change | Pre-built binary CLI tools should be distributed as Casks, not Formulas; Formulas are for source-compiled software |
| Classic PAT (full repo scope) | Fine-grained PAT (specific repo, Contents:write only) | GitHub 2022+ | Fine-grained tokens are narrower scope and more secure; both work for this use case |

**Deprecated/outdated:**
- `brews:` GoReleaser section: Deprecated since v2.10, will be removed in v3. Do not use.
- `homebrew_formulas:` GoReleaser section: Also deprecated; same replacement.
- Pre-GoReleaser curl URLs in `installation.md` (e.g., `kinder-darwin-arm64` without extension): These files no longer exist in GitHub Releases after Phase 35. Must be updated in SITE-01.

---

## Open Questions

1. **Does `releases/latest/download/` work with GoReleaser's versioned filenames?**
   - What we know: GoReleaser archives are named `kinder_VERSION_darwin_arm64.tar.gz` (version in filename). GitHub's `releases/latest/download/` endpoint requires knowing the exact filename.
   - What's unclear: Whether there is a stable filename pattern (without version) that works, or whether the site should link to the releases page and let users download manually.
   - Recommendation: The Homebrew cask handles the version-specific URL automatically. For the manual curl instructions, use the GitHub releases API pattern or direct users to the releases page. Alternatively, provide the full releases page URL `https://github.com/PatrykQuantumNomad/kinder/releases/latest` rather than a direct download link. The planner should decide between: (a) linking to releases page, (b) using `releases/latest/download/kinder_<OS>_<ARCH>.tar.gz` with instructions to replace version, or (c) using a version-agnostic URL if GitHub supports redirect. Simplest for SITE-01: just add the Homebrew section and keep the "download from GitHub Releases" link without a fragile version-specific curl command.

2. **Should the tap repo be public or private?**
   - What we know: Homebrew requires public taps for `brew install` to work without authentication. Private taps require token setup on the user's machine.
   - What's unclear: The phase requirements imply public (BREW-03 expects any macOS user to run the install command).
   - Recommendation: Create `homebrew-kinder` as a public repository. This is the standard for third-party Homebrew taps.

3. **Does the goreleaser.com documentation show the exact generated .rb format for archives (not apps)?**
   - What we know: The discussion thread showed a real-world cask using `on_macos do ... on_arm do ...` blocks. The GoReleaser official docs do not show the exact generated output.
   - What's unclear: Whether GoReleaser generates `on_macos` only (excluding Linux since Casks are macOS-only) or also `on_linux` blocks.
   - Recommendation: Casks are macOS-only by Homebrew definition. GoReleaser's `homebrew_casks` will only include `darwin_arm64` and `darwin_amd64` archives in the cask — it ignores linux and windows builds. This is correct behavior. No action needed, but the planner should not worry about Linux coverage in the cask.

---

## Validation Architecture

No `config.json` found — nyquist_validation treated as enabled.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Manual verification + `gh api` (no automated test framework applicable) |
| Config file | none |
| Quick run command | `goreleaser check` (validates .goreleaser.yaml syntax including homebrew_casks) |
| Full suite command | `goreleaser release --snapshot --skip=publish --clean` (dry-run without tap push) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BREW-01 | `homebrew-kinder` tap repo exists with Casks/ dir | manual | `gh repo view PatrykQuantumNomad/homebrew-kinder` | ❌ Wave 0 (create repo) |
| BREW-02 | GoReleaser publishes cask on tagged release | smoke | `gh api repos/PatrykQuantumNomad/homebrew-kinder/commits --jq '.[0].commit.message'` (after release) | ❌ requires live tagged release |
| BREW-03 | `brew install patrykquantumnomad/kinder/kinder` works on macOS | e2e/manual | `brew install patrykquantumnomad/kinder/kinder && kinder version` | ❌ requires live tagged release |
| SITE-01 | Installation page shows Homebrew instructions | manual | `cd kinder-site && npm run build` (no errors) + visual review | ✅ file exists (modified) |

### Sampling Rate

- **Per task commit:** `goreleaser check` (validates goreleaser.yaml syntax — catches typos in homebrew_casks section)
- **Per wave merge:** `goreleaser release --snapshot --skip=publish --clean` (full dry-run, skips actual publish)
- **Phase gate:** Requires a real tagged release to fully verify BREW-02 and BREW-03. Use a test tag (e.g., `v0.4.1-test.1` with `skip_upload: false` temporarily, then revert) OR wait for the next real release tag.

### Wave 0 Gaps

- [ ] `PatrykQuantumNomad/homebrew-kinder` repo — must exist with `Casks/` directory before GoReleaser can push (create via `gh repo create`)
- [ ] `HOMEBREW_TAP_TOKEN` secret — must exist in `kinder` repo settings before release workflow can authenticate

*(SITE-01 has no wave 0 gaps — `installation.md` exists and `npm run build` works locally)*

---

## Sources

### Primary (HIGH confidence)
- https://goreleaser.com/customization/homebrew_casks/ — full configuration schema, all fields, skip_upload behavior, token template syntax, directory default (fetched directly 2026-03-04)
- https://goreleaser.com/deprecations/ — confirmed `brews:` deprecated in v2.10, replaced by `homebrew_casks:` (fetched directly 2026-03-04)
- https://goreleaser.com/blog/goreleaser-v2.10/ — confirmed feature introduction version and migration path (fetched directly 2026-03-04)
- https://docs.brew.sh/Taps — tap naming convention `homebrew-{name}`, install command format `brew install user/repo/cask` (fetched directly 2026-03-04)
- https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap — `Casks/` directory requirement, initial commit requirement (fetched directly 2026-03-04)
- https://docs.brew.sh/Cask-Cookbook — minimum valid cask structure, `binary` directive, `on_macos`/`on_arm`/`on_intel` blocks (fetched directly 2026-03-04)
- Project codebase direct reads: `.goreleaser.yaml`, `.github/workflows/release.yml`, `kinder-site/src/content/docs/installation.md`, `.planning/REQUIREMENTS.md`

### Secondary (MEDIUM confidence)
- https://github.com/orgs/goreleaser/discussions/4926 — token environment variable name `HOMEBREW_TOKEN` (community convention; can be any name as long as it matches goreleaser.yaml template and workflow env)
- https://github.com/orgs/goreleaser/discussions/5563 — real-world GoReleaser-generated cask example (`on_macos do on_arm do url/sha256` structure)
- https://goreleaser.com/ci/actions/ — release.yml pattern with multiple env vars (GITHUB_TOKEN + custom tap token)

### Tertiary (LOW confidence)
- Community tutorials (dev.to, learnfastmakethings.com) — PAT scope requirements; multiple sources agree on "Contents: read/write" as minimum. Classic PAT with "repo" scope also works but is broader.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — GoReleaser homebrew_casks section live-verified from official docs; no new tools needed
- Architecture: HIGH — tap repository naming, directory structure, and install command format from official Homebrew docs; release.yml change is minimal and well-understood
- Pitfalls: HIGH — GITHUB_TOKEN cross-repo limitation is a documented constraint; tap naming convention from official Homebrew docs; pre-release skip pattern from GoReleaser official docs

**Research date:** 2026-03-04
**Valid until:** 2026-05-04 (GoReleaser homebrew_casks is stable since v2.10; Homebrew tap conventions are stable)
