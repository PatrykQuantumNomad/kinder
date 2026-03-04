# Phase 35: GoReleaser Foundation - Research

**Researched:** 2026-03-04
**Domain:** GoReleaser v2 configuration, GitHub Actions release pipeline, Go CLI versioning, cross-platform binary distribution
**Confidence:** HIGH — GoReleaser versions live-verified via GitHub API; all configuration patterns cross-checked against official docs; kinder codebase read directly

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Release artifacts:**
- Archive naming: `kinder_VERSION_OS_ARCH` format (e.g., `kinder_2.0.0_linux_amd64.tar.gz`) — version in filename
- Archive contents: just the binary — no LICENSE, README, or extras bundled
- Windows archives use `.zip`; Linux and macOS use `.tar.gz`
- GitHub Release body includes install snippet (curl one-liner) for quick setup

**Version display:**
- Match kind's existing `kind version` output format for consistency
- Dev/snapshot builds should be clearly distinguishable from tagged releases

**Changelog style:**
- GoReleaser auto-generates changelog categorized by commit prefix (feat:, fix:, docs:, etc.)
- Commits grouped into sections like Features, Bug Fixes, Documentation, Other

**Cross.sh retirement:**
- Remove cross.sh in the same commit that adds GoReleaser — atomic, clean cut
- GoReleaser handles both building AND publishing — full replacement of cross.sh + softprops/action-gh-release
- Claude should check codebase for any other cross.sh references beyond the release workflow

### Claude's Discretion
- Exact changelog categories and whether to include author handles
- Whether pre-release/RC tags get draft releases
- Dev/snapshot version string format (GoReleaser snapshot defaults are fine)
- Whether `kinder version` also shows node image version (check what kind does)
- Whether `kinder version --json` is added (check existing CLI patterns from v1.4)
- Local developer build workflow (Makefile integration vs plain go build)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| REL-01 | GoReleaser config produces cross-platform binaries (linux/darwin amd64+arm64, windows amd64) | `.goreleaser.yaml` `builds[*].goos`/`goarch`/`ignore` matrix documented below |
| REL-02 | GoReleaser generates SHA-256 checksums file for all release artifacts | `checksum:` section with `algorithm: sha256` and `name_template` pattern documented |
| REL-03 | GoReleaser generates automated changelog from git commit history | `changelog:` section with `groups` for feat/fix/docs/chore patterns documented |
| REL-04 | Release binaries show correct version metadata via `kinder version` | ldflags `-X` path to `pkg/internal/kindversion` identified; `{{ .FullCommit }}` template verified |
| REL-05 | GitHub Actions release workflow uses goreleaser-action replacing cross.sh + softprops | Full workflow YAML provided; goreleaser-action@v7 confirmed current |
| REL-06 | GoReleaser explicitly sets `gomod.proxy: false` and `project_name: kinder` (fork safety) | Fork module path mismatch pitfall documented; explicit config fields shown |
</phase_requirements>

---

## Summary

This phase replaces two shell-based release mechanisms — `hack/release/build/cross.sh` (cross-compilation via `xargs -P`) and `softprops/action-gh-release@v2` (GitHub Release creation) — with a single GoReleaser configuration. The change is purely in tooling and CI: no Go source files change, no `go.mod` changes, and no new Go dependencies are added. GoReleaser is a binary tool, not a Go library.

The kinder codebase already has working cross-compilation, SHA-256 checksums, and a tag-triggered release workflow. GoReleaser formalizes these into a declarative `.goreleaser.yaml` that handles building, archiving, checksum generation, changelog generation, and GitHub Release creation in one pass. The main complexity in this phase is the fork safety issue: kinder's `go.mod` declares `module sigs.k8s.io/kind` (the upstream module path), while the repository lives at `github.com/PatrykQuantumNomad/kinder`. GoReleaser's `gomod.proxy` feature would pull upstream kind binaries if not explicitly disabled.

The cross.sh retirement has three references beyond the release workflow: `hack/release/create.sh` (manual release script), `hack/ci/push-latest-cli/push-latest-cli.sh` (CI upload to GCS bucket), and `hack/ci/build-all.sh` (CI verification). These must all be audited and updated in the same commit per the locked decision.

**Primary recommendation:** Write `.goreleaser.yaml` with `gomod.proxy: false`, `project_name: kinder`, explicit ldflags matching the Makefile's `KIND_VERSION_PKG`, binary-only archives, and `goreleaser/goreleaser-action@v7` in the release workflow. Retire cross.sh in the same atomic commit.

---

## Standard Stack

### Core

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| GoReleaser OSS | v2.14.1 | Cross-platform build + archive + checksum + changelog + GitHub Release creation | Industry standard for Go CLI tools; single declarative config replaces the xargs-based cross.sh; released 2026-02-25 (live-verified) |
| goreleaser-action | v7.0.0 | GitHub Actions integration for GoReleaser | Official action maintained by GoReleaser team; v7 defaults to GoReleaser v2; released 2026-02-21 (live-verified) |

### Supporting

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| `goreleaser check` | (same binary) | Validate `.goreleaser.yaml` locally before pushing | Run before any PR that touches the config; also add as a Makefile target |
| `goreleaser build --snapshot --clean` | (same binary) | Local dry-run that builds all platform binaries without publishing | Run locally to verify `kinder version` shows real commit hash |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| GoReleaser v2.14.1 | Keep cross.sh + softprops | Only valid if Homebrew tap and structured changelogs are not needed; rejected per locked decisions |
| goreleaser-action@v7 | goreleaser-action@v6 | v6 is previous major; v7 is current with Node 24; use v7 |

**Installation (local dev only — not in go.mod):**
```bash
# macOS
brew install goreleaser

# Linux / any platform via GitHub Releases
curl -sSL https://github.com/goreleaser/goreleaser/releases/download/v2.14.1/goreleaser_Linux_x86_64.tar.gz | tar xz
sudo mv goreleaser /usr/local/bin/
```

GoReleaser is NOT added to `go.mod`. It is a standalone binary used only in CI and local dev.

---

## Architecture Patterns

### Recommended File Structure Changes

```
kinder/
├── .goreleaser.yaml                 NEW — GoReleaser config (replaces cross.sh in CI)
├── .github/
│   └── workflows/
│       └── release.yml              MODIFIED — replace cross.sh + softprops with goreleaser-action
├── Makefile                         MODIFIED — add goreleaser-check and goreleaser-snapshot targets
└── hack/
    └── release/
        └── build/
            └── cross.sh             DELETED — retired in the same commit as .goreleaser.yaml
```

Additional cross.sh references that must be removed/updated in the same commit:
- `hack/release/create.sh` line 71: `make clean && ./hack/release/build/cross.sh` — replace with `goreleaser build --clean`
- `hack/ci/push-latest-cli/push-latest-cli.sh` line 42: `hack/release/build/cross.sh` — this script uploads to GCS, evaluate whether it stays or is replaced
- `hack/ci/build-all.sh` line 26: `hack/release/build/cross.sh` — used for CI verification; replace with `go build ./...` or `make build`

### Pattern 1: GoReleaser Configuration — The Complete `.goreleaser.yaml`

**What:** A declarative YAML file at the repo root that tells GoReleaser how to build, package, and publish kinder binaries.
**When to use:** This is the single source of truth for the release pipeline — write it once, validate with `goreleaser check`.

```yaml
# Source: https://goreleaser.com/customization/build/ and https://goreleaser.com/customization/archive/
# Validated against GoReleaser v2.14.1

version: 2

project_name: kinder   # REQUIRED: explicit name prevents GoReleaser from inferring "kind" from module path

before:
  hooks:
    - go mod tidy

gomod:
  proxy: false          # REQUIRED: prevents GoReleaser from fetching sigs.k8s.io/kind from proxy
                        # (which would download upstream kind, not this fork)

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
        goarch: arm64   # Windows ARM64 not in scope (matches existing cross.sh matrix)
    ldflags:
      - -trimpath
      - -buildid=
      - -w
      - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .FullCommit }}
    mod_timestamp: "{{ .CommitTimestamp }}"

archives:
  - id: kinder
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    files:
      - none*             # Binary-only archive — no LICENSE, README, or extras

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc
  use: git
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: Bug Fixes
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: Documentation
      regexp: '^.*?docs(\([[:word:]]+\))??!?:.+$'
      order: 2
    - title: Other Changes
      order: 999
  filters:
    exclude:
      - '^chore:'
      - '^test:'
      - '^ci:'
      - Merge pull request
      - Merge branch

release:
  github:
    owner: PatrykQuantumNomad
    name: kinder
  draft: false
  mode: replace           # Allow re-running on same tag if workflow fails mid-flight
  footer: |
    ## Install

    **Linux / macOS (curl):**
    ```bash
    curl -Lo kinder https://github.com/PatrykQuantumNomad/kinder/releases/download/{{ .Tag }}/kinder_{{ .Version }}_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz | tar xz kinder
    chmod +x kinder && sudo mv kinder /usr/local/bin/
    ```

    **Windows (PowerShell):**
    Download `kinder_{{ .Version }}_windows_amd64.zip` from the assets above.
```

**Note on `gitCommitCount`:** The Makefile derives `COMMIT_COUNT` via `git describe --tags`. GoReleaser has no built-in template for this. For tagged release builds, `versionPreRelease` in `version.go` is set to `""` — so `gitCommitCount` is unused in the version string for tagged releases. The ldflags can safely omit the `gitCommitCount` `-X` flag for production releases. Only snapshot/dev builds use it, and GoReleaser snapshot mode clearly marks those versions anyway.

### Pattern 2: GitHub Actions Release Workflow

**What:** Replace the existing `release.yml` entirely — from a multi-step shell build to a single goreleaser-action step.
**When to use:** This is the only supported way to run GoReleaser in CI.

```yaml
# Source: https://goreleaser.com/ci/actions/
# .github/workflows/release.yml — complete replacement

name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write   # Required for GitHub Release creation and asset upload

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          # REQUIRED for GoReleaser: full history needed for changelog and git describe
          # DO NOT change to fetch-depth: 1 — this silently breaks changelog generation
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: .go-version   # Reads the pinned Go version from .go-version (currently 1.25.7)

      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Pattern 3: Makefile Additions for Local Development

**What:** Two new Makefile targets to integrate GoReleaser into the local workflow.
**When to use:** Add to `Makefile` alongside existing `build`, `test`, `lint` targets.

```makefile
# Validate goreleaser config locally (does not build or release)
goreleaser-check:
	goreleaser check

# Build all platform binaries locally (dry-run — does not publish)
goreleaser-snapshot:
	goreleaser build --snapshot --clean

.PHONY: goreleaser-check goreleaser-snapshot
```

Add `goreleaser-check goreleaser-snapshot` to the existing `.PHONY` line.

### Pattern 4: Version String Behavior

The `kindversion` package (at `pkg/internal/kindversion/version.go`) controls what `kinder version` outputs. The current version string logic:

- **Tagged release build** (`versionPreRelease = ""`): `kind v0.4.1 go1.25.7 linux/amd64` — gitCommit and gitCommitCount are ignored
- **Dev/snapshot build** (`versionPreRelease = "alpha"`): `kind v0.4.1-alpha.123+abc1234567890 go1.25.7 linux/amd64`

GoReleaser's `{{ .FullCommit }}` injects the 40-character SHA. The `version()` function truncates to 14 characters (matching Kubernetes convention). No source changes are needed to `version.go` to support this — the existing ldflags injection via `-X` works exactly.

For snapshot builds (`goreleaser build --snapshot`), GoReleaser appends `SNAPSHOT-{commit}` to the version by default. Since `versionPreRelease = "alpha"` in source, the snapshot binary will show `kind v0.4.1-alpha go1.25.7 linux/amd64` plus whatever GoReleaser snapshot suffix — clearly distinguishable from a release.

**The `kinder version` command currently outputs:** `kind v{version} {go-runtime} {os}/{arch}` — this matches kind's format exactly, satisfying the locked decision. No changes needed.

### Anti-Patterns to Avoid

- **Running `goreleaser release` and `cross.sh` in the same CI run:** Duplicate asset uploads produce GitHub API 422 errors. Retire cross.sh atomically.
- **Using `gomod.proxy: true` (or omitting it):** The default was `false` historically but explicit is safer — this fork's module path mismatch will silently produce upstream kind binaries if the proxy is consulted.
- **Using `brews:` section:** Deprecated since GoReleaser v2.10, will be removed in v3. Not needed for Phase 35 (Homebrew is Phase 36), but do not include it.
- **Setting `fetch-depth: 1` in checkout:** Silently breaks changelog generation and git describe.
- **Including files in archives:** The locked decision is binary-only. Use `files: [none*]` to suppress GoReleaser's default inclusion of LICENSE and README.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| SHA-256 checksums file | Shell loop with `shasum -a 256` (what cross.sh does) | GoReleaser `checksum:` section | GoReleaser computes checksums for all archives atomically; no risk of partial checksum files |
| Cross-platform archives (.tar.gz/.zip) | `tar czf` + `zip` in shell | GoReleaser `archives:` section | Format override per-OS, name template, binary permissions all handled |
| Changelog generation | `git log --oneline` parsing | GoReleaser `changelog:` section with groups | Regex-based grouping, sort order, exclude filters all built-in |
| GitHub Release creation + asset upload | `gh release create` + upload loop | GoReleaser `release:` section + goreleaser-action | Handles re-run with `mode: replace`, draft support, header/footer templating |
| Version injection | Makefile `$(COMMIT)` variable | GoReleaser `{{ .FullCommit }}` ldflag template | GoReleaser resolves at build time from git ref; same result, integrated into the build step |

**Key insight:** The existing cross.sh script manually implements exactly what GoReleaser's core features provide. The migration is a 1:1 replacement with better reliability (parallel builds by default, atomic checksum generation, no shell quoting bugs).

---

## Common Pitfalls

### Pitfall 1: Fork Module Path Causes Silent Wrong Binary

**What goes wrong:** `gomod.proxy: true` (or missing `gomod.proxy: false`) causes GoReleaser to resolve `sigs.k8s.io/kind@vX.Y.Z` from `proxy.golang.org`, downloading and building upstream kind source instead of this fork.

**Why it happens:** GoReleaser assumes module path matches repository URL. Forks that keep upstream module paths break this assumption.

**How to avoid:** Set `gomod.proxy: false` explicitly. This is REL-06's explicit requirement. Run `goreleaser build --snapshot --clean` locally and verify `./dist/kinder_linux_amd64_v1/kinder version` shows the fork's version string.

**Warning signs:** `go version -m ./dist/kinder_*/kinder` shows module path resolving to a commit hash that exists in the upstream `kubernetes-sigs/kind` repo.

### Pitfall 2: Ldflags Not Replicated from Makefile — Empty Version on Release Binaries

**What goes wrong:** GoReleaser does not read the Makefile. Without explicit ldflags in `.goreleaser.yaml`, `kinder version` outputs empty gitCommit and no pre-release metadata.

**Why it happens:** Two independent build systems each define ldflags. GoReleaser uses its own template syntax (`{{ .FullCommit }}` vs make's `$(COMMIT)`).

**How to avoid:** Copy the exact ldflags from `Makefile`'s `KIND_BUILD_LD_FLAGS` variable, translated to GoReleaser template syntax. The package path is `sigs.k8s.io/kind/pkg/internal/kindversion` — confirmed by reading `version.go` directly. After `goreleaser build --snapshot --clean`, verify with:
```bash
./dist/kinder_linux_amd64_v1/kinder version
# Expected: kind v0.4.1-alpha go1.25.7 linux/amd64
# (with non-empty build metadata if snapshot)
```

**Warning signs:** `kinder version` returns `kind v0.4.1-alpha go1.25.7 linux/amd64` with no `+commit` suffix in a snapshot build.

### Pitfall 3: Release Workflow Re-run Fails with 422 "Asset Already Exists"

**What goes wrong:** A failed GoReleaser run that uploaded some (not all) assets prevents re-running: the GitHub API rejects duplicate asset uploads with a 422 error.

**Why it happens:** GoReleaser uploads assets one by one. If a network error interrupts mid-upload, partial state is left on the GitHub Release.

**How to avoid:** Set `release: mode: replace` in `.goreleaser.yaml`. This makes GoReleaser delete and re-upload existing assets on re-run. Also always run GoReleaser with `--clean` flag to reset the local `dist/` directory.

**Warning signs:** CI logs show `422 Validation Failed: already_exists` from GitHub API.

### Pitfall 4: fetch-depth Shallow Clone Breaks Changelog

**What goes wrong:** Without `fetch-depth: 0`, GoReleaser cannot compute the git log range between the previous tag and the current tag. The changelog is empty or only shows "Initial commit".

**Why it happens:** GitHub Actions defaults to shallow clone (`fetch-depth: 1`). GoReleaser needs full history to run `git log v1.4.0..v2.0.0`.

**How to avoid:** The existing `release.yml` already uses `fetch-depth: 0`. Add a protective comment when rewriting the workflow to prevent future "optimization" regressions.

**Warning signs:** GitHub Release changelog section is empty after a tagged release.

### Pitfall 5: cross.sh References Beyond release.yml

**What goes wrong:** Retiring cross.sh from `release.yml` while leaving it in `hack/ci/build-all.sh` and `hack/ci/push-latest-cli/push-latest-cli.sh` means CI jobs that call those scripts will fail with "file not found".

**Why it happens:** The context decision says "check codebase for any other cross.sh references" — research confirms three additional references.

**How to avoid:** Audit and update all references in the same atomic commit:
1. `hack/release/create.sh:71` — replace `./hack/release/build/cross.sh` with a note or `goreleaser build --clean`
2. `hack/ci/push-latest-cli/push-latest-cli.sh:42` — this uploads to a GCS bucket (`k8s-staging-kind`); evaluate if this CI job is still used; if not, disable the job; if yes, replace with `go build` for each platform
3. `hack/ci/build-all.sh:26` — used for CI verification; replace with `go build ./...` or `make build`

**Warning signs:** Post-merge CI failures in jobs that are not the release workflow.

### Pitfall 6: Archive `name_template` Format Matters for Checksums

**What goes wrong:** If `name_template` uses a different case convention than expected (e.g., `Linux` vs `linux`), the archive filenames won't match the convention stated in the locked decisions (`kinder_VERSION_OS_ARCH` with lowercase OS).

**Why it happens:** GoReleaser's `{{ .Os }}` returns lowercase (`linux`, `darwin`, `windows`) by default. Using `{{ title .Os }}` would produce capitalized names. The locked decision uses lowercase.

**How to avoid:** Use `{{ .Os }}` and `{{ .Arch }}` directly (lowercase). Do NOT use `{{ title .Os }}`. The template `"{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"` produces `kinder_2.0.0_linux_amd64`, which matches the locked naming convention.

---

## Code Examples

### Complete `.goreleaser.yaml` for kinder

```yaml
# Source: https://goreleaser.com/customization/build/ + https://goreleaser.com/customization/archive/
# Verified against GoReleaser v2.14.1 (released 2026-02-25)

version: 2

project_name: kinder

before:
  hooks:
    - go mod tidy

gomod:
  proxy: false  # Fork safety: prevents resolving sigs.k8s.io/kind from Go module proxy

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
    ldflags:
      - -trimpath
      - -buildid=
      - -w
      - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .FullCommit }}
    mod_timestamp: "{{ .CommitTimestamp }}"

archives:
  - id: kinder
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    files:
      - none*  # Binary-only: no LICENSE, README, or extras

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc
  use: git
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: Bug Fixes
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: Documentation
      regexp: '^.*?docs(\([[:word:]]+\))??!?:.+$'
      order: 2
    - title: Other Changes
      order: 999
  filters:
    exclude:
      - '^chore:'
      - '^test:'
      - '^ci:'
      - Merge pull request
      - Merge branch

release:
  github:
    owner: PatrykQuantumNomad
    name: kinder
  draft: false
  mode: replace
  footer: |
    ## Install

    **Linux / macOS:**
    ```bash
    # Replace OS and ARCH with your platform (linux/darwin, amd64/arm64)
    curl -sSL https://github.com/PatrykQuantumNomad/kinder/releases/download/{{ .Tag }}/kinder_{{ .Version }}_linux_amd64.tar.gz | tar xz
    chmod +x kinder && sudo mv kinder /usr/local/bin/
    ```

    **Windows:** Download `kinder_{{ .Version }}_windows_amd64.zip` from the assets above and extract to a directory in your `PATH`.

    Verify the download: `sha256sum -c checksums.txt`
```

### Complete `release.yml` replacement

```yaml
# Source: https://goreleaser.com/ci/actions/
# Replaces the existing workflow that used cross.sh + softprops/action-gh-release

name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          # REQUIRED for GoReleaser: full history needed for changelog generation
          # DO NOT change to fetch-depth: 1 — changelog will be empty
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: .go-version

      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Verifying correct version output after snapshot build

```bash
# Build all platforms locally (dry-run — does not publish)
goreleaser build --snapshot --clean

# Verify the Linux amd64 binary shows real git commit
./dist/kinder_linux_amd64_v1/kinder version
# Expected output format (matching kind):
# kind v0.4.1-alpha go1.25.7 linux/amd64
# (for a snapshot build — note: tagged release would show v2.0.0 without pre-release)
```

### Updated Makefile targets

```makefile
# Validate goreleaser config — run before any PR touching .goreleaser.yaml
goreleaser-check:
	goreleaser check

# Local dry-run — builds all platform binaries, does not publish
goreleaser-snapshot:
	goreleaser build --snapshot --clean

.PHONY: all kind build install unit integration test test-race clean update generate gofmt verify lint shellcheck goreleaser-check goreleaser-snapshot
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `brews:` section in GoReleaser | `homebrew_casks:` section | GoReleaser v2.10 (Feb 2025) | Do NOT use `brews:` — deprecated, will be removed in v3 |
| goreleaser-action@v6 | goreleaser-action@v7 | 2026-02-21 | v7 uses Node 24; v6 still works but v7 is current |
| `format: zip` (singular) | `formats: [zip]` (plural, array) | GoReleaser v2.6 | Both still accepted in v2.14.1 but array is canonical |
| `gomod.proxy: false` (implicit default) | `gomod.proxy: false` (explicit required for forks) | GoReleaser v2.x | Fork safety requires explicit declaration |

**Deprecated/outdated:**
- `softprops/action-gh-release@v2`: Fully replaced by goreleaser-action in this phase. Do not combine both.
- `hack/release/build/cross.sh`: Retired in this phase. Do not keep as a "fallback".
- `goreleaser-action@v6`: Superseded by v7 (released 2026-02-21).

---

## Open Questions

1. **`hack/ci/push-latest-cli/push-latest-cli.sh` — active or vestigial?**
   - What we know: This script uploads kinder binaries to a GCS bucket (`k8s-staging-kind`) for Kubernetes CI consumption. It calls cross.sh. It targets a bucket name inherited from upstream kind.
   - What's unclear: Whether this CI job is actively triggered in this fork or is legacy upstream infrastructure that no longer applies.
   - Recommendation: Check if there is a Prow/GCB trigger for this script. If not found, comment out or delete the script body (keep the file to avoid breaking any external reference) and remove the cross.sh call. If active, replace `hack/release/build/cross.sh` with `go build` for each platform inline.

2. **`versionCore` value — update to 2.0.0 in this phase or separately?**
   - What we know: Current `versionCore = "0.4.1"` with `versionPreRelease = "alpha"`. The milestone is v2.0. GoReleaser uses the git tag for the release, not `versionCore`.
   - What's unclear: Whether the tag `v2.0.0` will be pushed as part of this phase's acceptance, requiring `versionCore` to be updated.
   - Recommendation: The version source-of-truth for GoReleaser is the git tag (e.g., `v2.0.0`), not `versionCore`. For tagged builds, `versionPreRelease` is expected to be `""`. The `create.sh` script shows the manual process: bump `versionCore` in code, commit, tag, then build. The planner should decide whether this phase includes the version bump commit or just the tooling. For phase success criteria (#1 uses "snapshot" builds), `versionCore` update is likely out of scope here.

3. **Changelog `use: github` vs `use: git` — handles authors?**
   - What we know: `use: github` appends GitHub usernames to each changelog entry. `use: git` uses raw commit messages. The locked decision does not specify author handles.
   - Recommendation: Use `use: git` for Phase 35 (simpler, no additional API call). Whether to add author handles is Claude's discretion — recommend `use: git` with groups for a clean, focused changelog.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` package (`go test ./...`) |
| Config file | none (standard Go test runner) |
| Quick run command | `go test ./pkg/internal/kindversion/... -v` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REL-01 | GoReleaser produces 5 platform binaries | smoke | `goreleaser build --snapshot --clean && ls dist/` | ❌ Wave 0 (Makefile target) |
| REL-02 | SHA-256 checksums file generated | smoke | `goreleaser build --snapshot --clean && test -f dist/checksums.txt` | ❌ Wave 0 (Makefile target) |
| REL-03 | Changelog generated | smoke | `goreleaser release --skip=publish --snapshot --clean 2>&1 | grep -i changelog` | ❌ Wave 0 (Makefile target) |
| REL-04 | `kinder version` shows real commit hash | unit + smoke | `go test ./pkg/internal/kindversion/... -v` (existing) + `goreleaser build --snapshot --clean && ./dist/kinder_linux_amd64_v1/kinder version` | ✅ existing test |
| REL-05 | release.yml uses goreleaser-action | manual | Review `.github/workflows/release.yml` content | ✅ file exists (modified) |
| REL-06 | `goreleaser check` passes zero errors | smoke | `goreleaser check` | ❌ Wave 0 (Makefile target) |

### Sampling Rate

- **Per task commit:** `go test ./pkg/internal/kindversion/... -v && goreleaser check`
- **Per wave merge:** `goreleaser build --snapshot --clean` (verifies all 5 platform binaries)
- **Phase gate:** `goreleaser check` exits 0 with zero deprecation warnings; `goreleaser build --snapshot --clean` produces 5 archives in `dist/`; `./dist/kinder_linux_amd64_v1/kinder version` shows non-empty commit hash

### Wave 0 Gaps

- [ ] `Makefile` goreleaser-check target — covers REL-06 smoke test
- [ ] `Makefile` goreleaser-snapshot target — covers REL-01/REL-02/REL-04 smoke tests
- [ ] `.goreleaser.yaml` must exist before any smoke test can run

*(REL-04's unit test exists at `pkg/internal/kindversion/version_test.go` — no new test file needed)*

---

## Sources

### Primary (HIGH confidence)

- GoReleaser OSS GitHub Releases API — v2.14.1, released 2026-02-25 (live-verified)
- goreleaser-action GitHub Releases API — v7.0.0, released 2026-02-21 (live-verified)
- https://goreleaser.com/customization/archive/ — archive `formats`, `format_overrides`, `files: [none*]` pattern for binary-only archives
- https://goreleaser.com/customization/checksum/ — `name_template`, `algorithm` fields
- https://goreleaser.com/customization/changelog/ — `groups`, `sort`, `use`, `filters.exclude` fields
- https://goreleaser.com/customization/release/ — `mode: replace`, `draft`, `footer` fields
- https://goreleaser.com/ci/actions/ — goreleaser-action inputs, `fetch-depth: 0` requirement, GITHUB_TOKEN
- Kinder codebase direct reads: `Makefile`, `.github/workflows/release.yml`, `hack/release/build/cross.sh`, `hack/release/create.sh`, `hack/ci/push-latest-cli/push-latest-cli.sh`, `hack/ci/build-all.sh`, `pkg/internal/kindversion/version.go`, `pkg/internal/kindversion/version_test.go`, `pkg/cmd/kind/version/version.go`, `go.mod`, `.go-version`

### Secondary (MEDIUM confidence)

- `.planning/research/STACK.md` — prior milestone research on GoReleaser stack (GoReleaser versions re-verified above)
- `.planning/research/PITFALLS.md` — prior research on fork module path pitfall, cross.sh retirement, PAT scope
- `.planning/research/ARCHITECTURE.md` — prior research on ldflags mapping from Makefile to GoReleaser

### Tertiary (LOW confidence)

- None — all findings verified against official docs or primary source code

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — GoReleaser v2.14.1 and goreleaser-action@v7.0.0 live-verified via GitHub API 2026-03-04
- Architecture: HIGH — `.goreleaser.yaml` patterns from official docs; kinder codebase read directly; ldflags path confirmed from version.go
- Pitfalls: HIGH — fork module path pitfall from official GoReleaser docs; cross.sh references found by direct grep of codebase; 422 error pattern from GoReleaser docs (`mode: replace` is the documented solution)

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (GoReleaser releases frequently but v2.14.1 is stable; goreleaser-action@v7 is current)
