# Phase 55: Windows PR-CI Build Step - Research

**Researched:** 2026-05-12
**Domain:** GitHub Actions workflow YAML, Go cross-compilation (`GOOS=windows GOARCH=amd64`), repo CI conventions
**Confidence:** HIGH (every load-bearing claim verified against the repo on disk; cgo probe executed locally and observed to pass on current `main`)

---

## Summary

Phase 55 adds a single new GitHub Actions workflow file — `.github/workflows/build-check.yml` — that runs `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` on `ubuntu-latest` (or `ubuntu-24.04`, matching repo convention) on every pull request to `main`. The job has one purpose: catch silent Windows compilation regressions before merge. The implementation surface is small (one YAML file, ~30 lines) but the value is asymmetric — every Go dependency bump, every new import, every refactor that adds a stdlib call has the potential to inadvertently break Windows cross-compile via a transitive cgo path (Pitfall 18). Without this job, the only signal is a failed `goreleaser build --snapshot --clean` at release time, which happens weeks after the offending PR merged.

The repo already has six existing workflow files (`docker.yaml`, `nerdctl.yaml`, `podman.yml`, `vm.yaml`, `release.yml`, `deploy-site.yml`, `macos-sign-verify.yml`) with strong, consistent conventions: pinned action SHAs with version comments, `go-version-file: .go-version`, `permissions: contents: read`, 30-minute timeouts on long-running jobs, `paths-ignore: ['site/**']` on PR triggers. The new workflow should match these conventions exactly — no new patterns, no concurrency groups (not used elsewhere), no matrix (we test exactly one target). The `GoReleaser` config (`.goreleaser.yaml`) ALREADY builds windows/amd64 with `CGO_ENABLED=0` for every release tag, so this phase is closing the feedback-loop gap: shift-left the cross-compile check from tag-push to PR-open.

**Empirical probe result (executed during research, satisfying SC2):**

```
$ go version
go version go1.26.3 darwin/arm64

$ CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...
$ echo $?
0

$ CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go vet ./...
$ echo $?
0

$ CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go list -f '{{.ImportPath}} {{.CgoFiles}}' ./... | grep -v '\[\]'
(no output — no packages declare CgoFiles under GOOS=windows)
```

Pitfall 18 is empirically clear at current `main` HEAD (commit `d69a67cb`). No cgo audit work is needed before writing the CI YAML — the planner can proceed directly to wiring the workflow. [VERIFIED: local probe, this session]

**Primary recommendation:** Create `.github/workflows/build-check.yml` with a single `windows-build` job: trigger on `pull_request` to `main` with `paths-ignore: ['site/**', 'kinder-site/**']`, `runs-on: ubuntu-24.04`, `permissions: contents: read`, checkout + setup-go (using same pinned SHAs as `macos-sign-verify.yml` and `release.yml`), one `go build` step exporting `CGO_ENABLED=0 GOOS=windows GOARCH=amd64`. Do NOT add a matrix — Windows ARM64 is explicitly out of scope per `.goreleaser.yaml` (`ignore: { goos: windows, goarch: arm64 }`). Do NOT add `go vet` — DIST-02 is `go build` only; mixing in vet expands scope. One plan, two tasks: (1) local probe re-verification + write the YAML, (2) commit + observe a PR check fire green.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Detect Windows compile regression in code change | GitHub Actions PR trigger | — | Pre-merge feedback loop; only PR-event delivers this signal |
| Cross-compile Go binary for windows/amd64 from Linux | Go toolchain (`go build` w/ GOOS/GOARCH/CGO_ENABLED) | — | Pure-Go cross-compile; no native toolchain on the runner |
| Determine Go version for the build | `.go-version` file | `actions/setup-go` with `go-version-file` | Single-source-of-truth file already used by 4 workflows |
| Block PR merge on failure | GitHub PR check status | Repo merge policy (currently advisory — see Open Questions) | Workflow declares the check; whether it's *required* depends on branch protection (NOT currently configured) |
| Cache Go modules between runs | `actions/setup-go` built-in cache | — | Caching is ON by default in setup-go v6; no extra config needed |

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DIST-02 | PR CI gains a blocking `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` step on `ubuntu-latest`; failure fails the PR check | `.github/workflows/build-check.yml` (new file) running on `pull_request` event with non-zero `go build` exit causing job failure satisfies the "fails the PR check" half. The "blocking" half is partially constrained — see Open Question 1 on branch protection. |

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `actions/checkout` | v6.0.2 (SHA `de0fac2e4500dabe0009e67214ff5f5447ce83dd`) | Check out repo source on the runner | Pinned exact SHA used by all 6 existing workflows in this repo — match for consistency |
| `actions/setup-go` | v6.2.0 (SHA `7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5`) | Install Go toolchain, restore module cache | Pinned exact SHA used by `release.yml`, `vm.yaml`, `macos-sign-verify.yml` |
| Go toolchain | 1.25.7 | Compile the binary | Pinned via `.go-version` file — single source of truth for all CI |
| `ubuntu-24.04` runner | n/a | Run the build | Used by `docker.yaml`, `nerdctl.yaml`, `podman.yml`, `vm.yaml`. `ubuntu-latest` is also acceptable per DIST-02 wording — see Pattern 1 for the choice rationale |

### Supporting

(none — no third-party actions beyond checkout + setup-go are needed)

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| New file `build-check.yml` | Add a job inside `docker.yaml` | Violates SC1 explicit naming. SC1 mandates a NEW file at the exact path `.github/workflows/build-check.yml`. Do not deviate. |
| `runs-on: ubuntu-24.04` | `runs-on: ubuntu-latest` | DIST-02 wording says "on `ubuntu-latest`". Repo convention says `ubuntu-24.04`. Recommendation: use `ubuntu-24.04` to match siblings — `ubuntu-latest` floats and could break consistency across the four other ubuntu-pinned workflows. See Open Question 4. |
| Direct `go build ./...` step | `make build` or reusing `hack/ci/build-all.sh` | Both alternatives bake in defaults that fight cross-compile. `Makefile` sets `CGO_ENABLED=0` (good) but its `kind:` target writes to `bin/$(KIND_BINARY_NAME)` with `-trimpath` ldflags — fine but irrelevant for a *check*. `hack/ci/build-all.sh` runs `go build -v ./...` with no GOOS/GOARCH/CGO env. Wrapping either to inject env adds indirection without value. Prefer direct `go build ./...` in the workflow step for minimum surface area. |
| Matrix over `goos: [windows], goarch: [amd64]` | Single explicit `env` block | Matrix adds visual noise for a single combination. Add matrix only if/when `goos: [windows, freebsd]` becomes in scope (it is not). |
| Add `go vet ./...` step under GOOS=windows | Build-only check | DIST-02 is explicitly build-only. Pitfall 19 (path-separator runtime issues) is acknowledged out of scope per ROADMAP "Non-Goals". Don't expand. |

**Installation:** No package installation. The new file is the entire deliverable.

**Version verification (executed this session):**
- `actions/setup-go` v6.2.0 SHA `7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5`: matches the pin in `release.yml:20` and `macos-sign-verify.yml:27` and `vm.yaml:52`. [VERIFIED: grep against repo]
- `actions/checkout` v6.0.2 SHA `de0fac2e4500dabe0009e67214ff5f5447ce83dd`: matches the pin in all six existing workflows. [VERIFIED: grep against repo]
- `actions/setup-go` v6.4.0 exists upstream (latest as of this research). [CITED: https://github.com/actions/setup-go/releases] Do NOT bump in this phase — would introduce inconsistency. Bumping setup-go across the four workflows is a separate concern (it should be uniform).
- `.go-version` file content: `1.25.7`. [VERIFIED: `cat .go-version`]
- `go.mod` declares `go 1.24.0` and `toolchain go1.26.0`. The compiler version that builds is governed by `.go-version` (`1.25.7`) via `actions/setup-go` — NOT by `go.mod`. [VERIFIED: `go.mod` and `hack/build/gotoolchain.sh`]

---

## Architecture Patterns

### System Architecture Diagram

```
                 ┌─────────────────────────────────┐
                 │   Developer opens PR to main    │
                 └────────────────┬────────────────┘
                                  │
                  pull_request event (with paths-ignore filter)
                                  │
                                  ▼
        ┌────────────────────────────────────────────────────┐
        │  GitHub Actions: build-check.yml `windows-build`   │
        │                                                    │
        │   1. actions/checkout@v6.0.2  (fetch HEAD)         │
        │   2. actions/setup-go@v6.2.0  (Go 1.25.7 + cache)  │
        │   3. env: CGO_ENABLED=0                            │
        │           GOOS=windows                             │
        │           GOARCH=amd64                             │
        │      run: go build ./...                           │
        │                                                    │
        └─────────────┬────────────────────────┬─────────────┘
                      │                        │
              exit 0 (success)         exit non-zero (failure)
                      │                        │
                      ▼                        ▼
              ✅ PR check green       ❌ PR check red
              (one of N PR checks)   (PR cannot be marked "all checks passing";
                                     blocking effect depends on merge policy —
                                     see Open Question 1)
```

The job is intentionally a one-step build (after setup). No matrix. No artifact upload. No log post-processing. The exit code IS the signal.

### Recommended Project Structure

The only file added in this phase:

```
.github/
└── workflows/
    └── build-check.yml          # NEW — one job, one step (build); runs on PR
```

No code files change. No `Makefile` change. No `hack/` script change. No docs change (release notes/CHANGELOG mention is a separate concern; CI changes are not user-visible).

### Pattern 1: Match Existing Repo Workflow Conventions Exactly

**What:** The new workflow YAML should be visually consistent with the closest existing analogue — `macos-sign-verify.yml` is the best precedent because it is also (a) a single-job verification workflow, (b) does not use the matrix pattern, (c) added in the most recent CI work (Phase 54).

**When to use:** Always. Diverging from house style for a 30-line file imposes review-time cost without payoff.

**Reference (the closest sibling — verified pattern):**
```yaml
# Source: /Users/patrykattc/work/git/kinder/.github/workflows/macos-sign-verify.yml
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
      ...
```

**Adaptations for Phase 55:**
- Trigger: `pull_request` (NOT `push`) with `paths-ignore: ['site/**', 'kinder-site/**']` (match `docker.yaml`/`podman.yml` pattern — `site/` is the Hugo site, `kinder-site/` is the Astro site, both irrelevant to Go compile)
- Runner: `ubuntu-24.04` (NOT `macos-latest`)
- `fetch-depth: 0` is NOT needed — only `release.yml` and `macos-sign-verify.yml` need full history (for goreleaser changelog generation). A build check needs only the working tree. Use default `fetch-depth: 1` (omit the `with:` block on checkout) to minimize clone time.
- Steps reduced to: checkout, setup-go, single `go build` step

### Pattern 2: Single Env Block Per Step

**What:** Set `CGO_ENABLED=0 GOOS=windows GOARCH=amd64` as `env:` keys on the `run` step rather than inline before the command. This is the GitHub Actions idiomatic form and is easier to scan in CI logs.

**Example:**
```yaml
- name: Cross-compile for windows/amd64 (Pitfall 18 — cgo transitive dep probe)
  env:
    CGO_ENABLED: "0"
    GOOS: windows
    GOARCH: amd64
  run: go build ./...
```

**Alternative (also acceptable, slightly less idiomatic):**
```yaml
- name: Cross-compile for windows/amd64
  run: CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...
```

Either works. The `env:` block form makes failures in CI logs cleaner because GitHub renders the env block as a collapsible section in the step header.

### Anti-Patterns to Avoid

- **Adding a job to `docker.yaml`:** SC1 mandates a new file at `.github/workflows/build-check.yml`. Adding to docker.yaml would (a) miss the SC1 file-creation criterion, (b) couple the build check to the Docker integration job's `paths-ignore`, (c) pollute the docker workflow's matrix.
- **Using `runs-on: ubuntu-latest` while four siblings use `ubuntu-24.04`:** DIST-02 wording says `ubuntu-latest` but the repo convention is pinned. Recommend deviating from DIST-02 wording to match repo convention — `ubuntu-latest` is a floating tag that has surprised teams when GitHub bumps the alias. Document the choice clearly in the workflow comment header. See Open Question 4.
- **Adding `concurrency: { group: ..., cancel-in-progress: true }`:** Not used in any existing workflow in this repo. Adding it now would be the first usage and should be a deliberate, separate decision applied uniformly across all PR workflows — not a one-off here.
- **Caching `go mod` manually with `actions/cache`:** `actions/setup-go@v6` caches modules AND build outputs by default. Don't double-cache. [CITED: https://github.com/actions/setup-go#caching-dependency-files-and-build-outputs]
- **Building only `./...` selectively (e.g., `./pkg/...` excluding `./cmd/...`):** `./...` is the canonical "all packages" pattern and matches the DIST-02 wording verbatim. Don't narrow.
- **Skipping the local re-probe in the implementation task:** SC2 explicitly requires the cgo probe be verified locally BEFORE the YAML is written. The research-session probe (this document) is one verification, but the implementation task should re-run it as the first step in case `main` advanced between research and implementation. State this explicitly in the plan.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Detecting Go version to install on the runner | Bash script that reads `.go-version` and `curl`s the toolchain | `actions/setup-go@v6` with `go-version-file: .go-version` | The action handles archive download, GOPATH/GOROOT setup, cache, and version validation in one line |
| Caching Go module downloads | `actions/cache` keyed on `go.sum` hash | `actions/setup-go@v6`'s built-in cache (on by default) | Built-in cache handles module + build cache, key collision, and OS/arch namespacing automatically |
| "Has the cgo path leaked in?" detection | Custom `grep` over `go build -x` output | Just `go build ./...` with `CGO_ENABLED=0` — exit code IS the signal | When CGO_ENABLED=0 and a transitive dep needs cgo, the build fails with a clear error. No log parsing needed. |
| Cross-platform path verification | Custom `find`/`grep` for `/` separators in Go source | (out of scope for Phase 55 — Pitfall 19 is a runtime concern, deferred per ROADMAP "Non-Goals") | Build-only check by design |

**Key insight:** This phase is a "smaller is better" exercise. The build-check job should be the simplest possible workflow that meaningfully exercises the cross-compile path. Every extra step (vet, test, lint, matrix) increases CI minutes, adds failure modes, and dilutes the signal. One job, three setup steps (checkout, setup-go, env-build), exit code as truth.

---

## Common Pitfalls

### Pitfall 1: cgo Transitive Dependencies Break Windows Cross-Compile (Pitfall 18, restated)

**What goes wrong:** A direct or transitive Go dependency imports a package with `import "C"` or build tags that activate cgo on Linux/macOS. With `CGO_ENABLED=0` on the Windows cross-compile target, `go build` fails with `cgo: C compiler "x86_64-w64-mingw32-gcc" not found` or `build constraints exclude all Go files in ...`.

**Why it happens:** Provider code in kind/kinder historically has touched paths like `opencontainers/runc`, `moby/sys/mountinfo`, `fsnotify` (pre-v1.10) — some of which have cgo paths that are excluded only by `//go:build !windows` tags. A refactor that removes such a build tag, or a new dependency that adds a cgo-tainted package, would slip through `go build ./...` on linux/macOS but break on windows/amd64.

**How to avoid:**
1. The CI step IS the prevention — every PR's `go build ./...` under `GOOS=windows CGO_ENABLED=0` either passes or fails.
2. Implementation task 1 MUST re-run the probe locally before writing YAML. Research-session probe passed this session, but `main` may have moved.
3. If a future PR fails the build-check, the debug path is `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go list -f '{{.ImportPath}} {{.CgoFiles}}' ./... | grep -v '\[\]'` to identify the offending package, then either add `//go:build !windows` to the offending file or switch to a pure-Go alternative.

**Warning signs:**
- `go build` output mentions `cgo:` under GOOS=windows
- `go list -f '{{.CgoFiles}}'` returns non-empty for any package under GOOS=windows
- Recent dependency bump in `go.mod`/`go.sum` involving any of: containerd, runc, mountinfo, fsnotify, libcontainer, gopsutil

**Source:** `.planning/research/PITFALLS.md` Pitfall 18 lines 314-327. [VERIFIED: file read this session]

### Pitfall 2: `pull_request` vs `pull_request_target` Confusion

**What goes wrong:** Using `pull_request_target` instead of `pull_request` would run the workflow with write permissions and the SHA of the base branch, not the PR head. For a build check this is wrong (we want to check the PR's code, not main's) AND dangerous (write permissions on PR-author-controlled code).

**Why it happens:** Some workflows use `pull_request_target` to allow access to secrets for PRs from forks. A build check does not need secrets and must check the PR head.

**How to avoid:** Use `on: pull_request:` (NOT `pull_request_target`). The four existing PR workflows (`docker.yaml`, `nerdctl.yaml`, `podman.yml`, `vm.yaml`) all use `pull_request` — match them.

**Warning signs:** Workflow file contains `pull_request_target` or references `${{ github.event.pull_request.head.sha }}` manually.

### Pitfall 3: `ubuntu-latest` Drift

**What goes wrong:** `ubuntu-latest` is a floating alias that GitHub updates on its own schedule (`ubuntu-latest` flipped from 20.04 → 22.04 → 24.04 across 2023-2024). A PR that was green on Friday on the old alias can be red on Monday with no code change. The four existing CI jobs in this repo pin `ubuntu-24.04` for exactly this reason.

**Why it happens:** DIST-02 wording says `ubuntu-latest` — taking the wording literally creates drift risk.

**How to avoid:** Use `ubuntu-24.04` to match the rest of the repo, document the deviation from DIST-02 wording in a workflow header comment. The DIST-02 *intent* is "an Ubuntu runner" — the version is incidental. See Open Question 4.

**Warning signs:** Workflow uses `ubuntu-latest` while sibling workflows use a pinned version.

### Pitfall 4: SHA-Pinned Actions Without Version Comments

**What goes wrong:** `uses: actions/setup-go@7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5` with no `# v6.2.0` comment makes future readers unable to tell what version is pinned without GitHub-API lookups. The k8s-sigs convention (visible in every existing workflow in this repo) is `SHA # vX.Y.Z`.

**How to avoid:** Match the pin format exactly: `uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2`.

**Warning signs:** Workflow review picks up a missing version comment.

### Pitfall 5: Cache-Key Collision Across Workflows

**What goes wrong:** `actions/setup-go@v6` caches under a key derived from runner OS, Go version, and `go.sum` hash. Two workflows on the same runner OS + Go version + go.sum will share the cache (good for hit rate, fine for correctness since cache content is identical). However, `setup-go` does not include GOOS/GOARCH in the build cache key — meaning the cross-compile build cache from `build-check.yml` could collide with native-Linux build cache from another job. In practice the action invalidates safely because the build cache directory is `~/.cache/go-build` keyed internally by GOOS/GOARCH. No action needed. Document so the planner doesn't waste a cycle worrying about it.

**How to avoid:** No action needed — setup-go handles this. Mentioned only so the planner can answer the question if it arises in review.

**Source:** [CITED: https://github.com/actions/setup-go#caching-dependency-files-and-build-outputs]

### Pitfall 6: PR Check is Not "Blocking" Without Branch Protection (CRITICAL)

**What goes wrong:** Creating the workflow file makes the check *appear* on every PR, but a red check does NOT prevent merge unless `main` is configured with a branch protection rule (or repo ruleset) that lists `windows-build` (or whatever the job name resolves to) as a "required status check." Today (verified this session), `main` on `github.com/PatrykQuantumNomad/kinder` is NOT protected:

```
$ gh api repos/:owner/:repo/branches/main/protection
{"message":"Branch not protected", ... "status":"404"}

$ gh api repos/:owner/:repo/rulesets
[]
```

DIST-02 says "fails the PR check" — strictly speaking, a red check IS a failure regardless of merge enforcement. But the *intent* of "blocking" implies the check prevents merge. With no protection in place, the check is advisory only.

**Why it happens:** DIST-02 wording conflates workflow-level "failing check" with repo-level "required check." These are two layers.

**How to avoid:**
1. The workflow file itself satisfies the literal DIST-02 wording — failure of the build step fails the GitHub check status.
2. Whether to make the check *enforced* (required for merge) is a separate decision involving branch protection or rulesets. This is OUT OF CODE — it's a GitHub UI / `gh api` setting.
3. **Recommendation:** Plan should include an optional task to configure branch protection requiring the new `windows-build` check (and possibly the existing checks). Mark this task as `[NEEDS USER DECISION]` because it changes merge behavior repo-wide. Alternatively, descope to "documentation note: this check is currently advisory" and lock blocking as a follow-up.

**Warning signs:** A red `windows-build` check on a PR is merged anyway because branch protection is not configured.

**Source:** `gh api` calls executed this session.

### Pitfall 7: Job Name vs Check Name Confusion

**What goes wrong:** The "check name" that appears in GitHub PR UI is `<workflow name> / <job name>` (e.g., `Windows Build Check / windows-build`). If branch protection is later configured to require a specific check, the required check name must match exactly. Using a misleading or generic `job:` name (`build:`, `test:`) makes the required-check label ambiguous if multiple workflows have a job called `build`.

**How to avoid:**
- Workflow `name:` — something specific like `Windows Build Check` or `Build Check (windows/amd64)`
- Job name — unique within the repo, e.g., `windows-build` (NOT just `build`)
- Document the resulting check name in the workflow header comment so future branch-protection configuration is unambiguous

---

## Runtime State Inventory

This phase is a pure code/config addition (a new YAML file). No runtime state lives in the repo for this phase to migrate.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — verified by review of phase scope (no DB, no Mem0, no Chroma, no n8n) | None |
| Live service config | GitHub Actions itself stores workflow state per run, but the PHASE adds a workflow — no pre-existing GitHub config to migrate | None |
| OS-registered state | None — verified (no Task Scheduler, launchd, systemd in scope) | None |
| Secrets/env vars | None — verified (no secrets used; `GITHUB_TOKEN` is implicit and read-only for build check) | None |
| Build artifacts | None — verified (no go-install package, no Docker image rebuild needed for a workflow file) | None |

---

## Code Examples

### Example 1: Complete Recommended `build-check.yml`

This is a concrete reference implementation. The planner can adapt — the values and structure here are the verified-conforming baseline.

```yaml
# Source: this RESEARCH.md (Phase 55) — pattern derived from .github/workflows/macos-sign-verify.yml
# Purpose: Pitfall 18 prevention — every PR is cross-compiled for windows/amd64 on Linux
# to catch silent cgo transitive dep regressions before they reach release.
#
# Check name in PR UI: "Windows Build Check / windows-build"
# Currently advisory. To make blocking, configure branch protection on `main` requiring this check.
name: Windows Build Check

on:
  pull_request:
    branches:
      - main
    paths-ignore:
      - 'site/**'
      - 'kinder-site/**'
  workflow_dispatch: {}

permissions:
  contents: read

jobs:
  windows-build:
    name: windows-build
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2

      - uses: actions/setup-go@7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5 # v6.2.0
        with:
          go-version-file: .go-version

      - name: Cross-compile for windows/amd64 (cgo transitive dep probe — Pitfall 18)
        env:
          CGO_ENABLED: "0"
          GOOS: windows
          GOARCH: amd64
        run: go build ./...
```

**Notes on every line:**
- `name: Windows Build Check` — appears as workflow title in the Actions UI. Specific enough to disambiguate from other "Build" workflows.
- `on: pull_request: branches: [main]` — matches docker/podman/nerdctl/vm pattern exactly.
- `paths-ignore: ['site/**', 'kinder-site/**']` — both site dirs are documentation only; no Go source. Skipping these saves ~30s of CI minutes per docs PR. Matches `docker.yaml`'s pattern (which only excludes `site/**` — adding `kinder-site/**` is incremental given the kinder-site dir exists at repo root and contains no Go code).
- `workflow_dispatch: {}` — allows manual re-run from the Actions UI without a PR open. Matches `macos-sign-verify.yml`.
- `permissions: contents: read` — least-privilege, matches all six existing workflows.
- `timeout-minutes: 10` — generous for a single `go build ./...` (typically <60s with cache, <3min cold). Matches the principle in `docker.yaml`'s `timeout-minutes: 30` (long jobs get long timeouts, short jobs get shorter).
- `runs-on: ubuntu-24.04` — pinned per repo convention. Deviates from DIST-02 literal wording (`ubuntu-latest`); see Open Question 4.
- No `fetch-depth: 0` on checkout — we don't need git history for a build check (only release.yml and macos-sign-verify.yml need it for goreleaser).
- `go-version-file: .go-version` — single source of truth, matches release.yml and macos-sign-verify.yml.
- `env:` block on the build step — idiomatic; lets GitHub log the values visibly in the step header.
- `go build ./...` — exact wording from DIST-02.

### Example 2: Local Re-Probe (Implementation Task 1 — SC2)

The implementation MUST re-run the probe locally as the first task in the plan, even though research-session probe passed. This is non-negotiable per SC2.

```bash
# Step 1: Verify Go is installed at the pinned version (or newer is fine for cross-compile)
go version
cat .go-version  # Expect 1.25.7

# Step 2: Run the probe — MUST exit 0 before any YAML is written
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...
echo "Exit code: $?"

# Step 3: (recommended) Confirm no transitive cgo paths under windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go list -f '{{.ImportPath}} {{.CgoFiles}}' ./... | grep -v '\[\]' || echo "No cgo files under GOOS=windows — clean"

# Step 4: (optional but recommended for confidence) Verify with goreleaser snapshot which uses the same env
# This builds ALL platforms including windows/amd64 — proves end-to-end release path
goreleaser build --snapshot --clean --single-target --id kinder
```

**Expected output:** All three steps exit 0. If step 2 fails, do NOT proceed to writing the YAML — investigate the cgo dependency first.

**If the probe fails:** The failure shows the import chain. Common recoveries (Pitfall 1 above):
- Add `//go:build !windows` build tag to the offending file
- Switch to a pure-Go alternative (e.g., fsnotify v1.10+ which is pure-Go on Windows via x/sys/windows)
- Revert the cgo-introducing dependency bump

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `hack/release/build/cross.sh` parallel cross-compile (Phase 35 retired this) | `.goreleaser.yaml` with `goos: [linux, darwin, windows]` | Phase 35 (2026-02-25 GoReleaser v2.14.1) | Cross-compile already proven for release. PR-time check (this phase) closes the feedback-loop gap. |
| No PR-time cross-compile check | New `build-check.yml` PR job | This phase (Phase 55) | Regressions caught at PR open, not release time |
| `actions/setup-go@v5.0.1` (the composite `setup-env` action) | `actions/setup-go@v6.2.0` (used in newer workflows) | Phase 54 (`macos-sign-verify.yml`) | New workflow should use v6.2.0 for consistency with the latest CI work |

**Deprecated/outdated:**
- `hack/release/build/cross.sh` — DELETED in Phase 35. Do NOT reintroduce. The new `build-check.yml` does NOT depend on this script.
- `actions/setup-go@v5.x` — still pinned in `.github/actions/setup-env/action.yaml` (the composite used by the 4 integration-test workflows). This is a pre-existing inconsistency. Do NOT fix in this phase — it's a separate concern that affects four workflows uniformly. Adding the build-check workflow at v6.2.0 (matching release.yml/macos-sign-verify.yml) does not worsen the inconsistency.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The phase author intends `runs-on: ubuntu-24.04` to satisfy "on `ubuntu-latest`" per DIST-02 because the repo pins ubuntu versions everywhere else | Standard Stack > Alternatives | If the author strictly requires `ubuntu-latest`, change one line. Trivial recovery. |
| A2 | `paths-ignore: ['site/**', 'kinder-site/**']` is desired (skip both site dirs) | Code Examples > Example 1 | If the author wants the check to fire on site-only changes too, drop the `paths-ignore` block. Low risk; would just consume CI minutes. |
| A3 | `workflow_dispatch: {}` should be included for manual re-runs | Code Examples > Example 1 | Trivial — drop the line if not wanted. |
| A4 | The phase does NOT include configuring branch protection / required checks in this iteration | Pitfall 6 | If "blocking" must be enforced, an additional task (or follow-up phase) is needed. Document explicitly. |
| A5 | `timeout-minutes: 10` is generous enough; typical CI run will be <3min cold, <60s warm | Code Examples > Example 1 | If runs trend longer, bump to 15-20. Trivial. |

**These assumptions surface decisions the planner should either confirm or escalate.** None are load-bearing; recovery is one-line edits.

---

## Open Questions

1. **Is the new `windows-build` check intended to be merge-blocking (required status check) on `main`?**
   - What we know: DIST-02 wording says "blocking (failure fails the PR check)" — the check itself fails on `go build` error. ROADMAP SC3 reinforces "blocking." However, `main` is not currently protected (verified via `gh api`), and no rulesets exist. So a red check is advisory unless protection is configured.
   - What's unclear: Whether "blocking" in DIST-02 means (a) "the workflow run's exit code is non-zero on failure" (satisfied by the YAML alone) or (b) "merge to main is prevented" (requires branch protection or ruleset configuration outside the YAML).
   - Recommendation: Treat (a) as the in-scope deliverable for this phase. Add an OPTIONAL follow-up task in the plan: "Configure branch protection on `main` to require `windows-build / windows-build` check." Mark `[NEEDS USER CONFIRMATION]` because it changes merge behavior. Alternatively, fold into a separate "CI policy" phase outside v2.4.

2. **Should the new workflow be added to the existing `setup-env` composite action's matrix, or kept as a standalone file?**
   - What we know: `.github/actions/setup-env/action.yaml` is a composite used by docker/podman/nerdctl/vm. It does setup-go + `make install` + `kubectl` install. For a *cross-compile build check*, none of those extras are needed — we don't run kinder, only build it.
   - What's unclear: Nothing — SC1 explicitly mandates a new file `.github/workflows/build-check.yml`, so the question is moot. Listed here for completeness.
   - Recommendation: Standalone file, do NOT use `setup-env` composite. Use `actions/setup-go@v6.2.0` directly.

3. **Should `go vet ./...` be added as a second step under GOOS=windows (Pitfall 19 mitigation)?**
   - What we know: Pitfall 19 (path-separator runtime issues) IS a real risk; `go vet` would catch some of it. ROADMAP "Non-Goals" explicitly defers Strict-Windows runtime support, and DIST-02 wording is `go build ./...` only.
   - What's unclear: Whether the planner has discretion to expand scope to `go vet` for "small additional value."
   - Recommendation: DO NOT add `go vet`. Strict scope adherence — Phase 55 is build-only per ROADMAP. If `go vet` value is desired, file it as a follow-up phase or a v2.5 candidate.

4. **`ubuntu-latest` vs `ubuntu-24.04` — DIST-02 says the former, repo convention is the latter. Which wins?**
   - What we know: DIST-02 says "on `ubuntu-latest`". Four other PR-event workflows in this repo pin `ubuntu-24.04`.
   - What's unclear: Whether DIST-02 wording is prescriptive or descriptive.
   - Recommendation: Use `ubuntu-24.04` for consistency with siblings, and document the deviation in the workflow's header comment. Rationale: `ubuntu-latest` is a floating alias that has caused CI breakage when GitHub silently bumps it. The pinned form is the established repo pattern; deviating to `ubuntu-latest` for one workflow creates fragmentation. The DIST-02 *intent* (cross-compile on a Linux Ubuntu runner) is fully satisfied by `ubuntu-24.04`.

5. **Should `paths-ignore` include the `.planning/**` directory?**
   - What we know: `.planning/` is documentation/planning artifacts. No Go source. Currently no workflow excludes `.planning/**`.
   - What's unclear: Whether the docker/podman/nerdctl workflows fire on `.planning/`-only PRs (they would today — they only exclude `site/**`). This is a minor CI-minutes optimization.
   - Recommendation: Match existing convention — exclude only `site/**` and `kinder-site/**`. Adding `.planning/**` is a cross-cutting CI-cost improvement that should be applied uniformly (separate concern, separate phase).

---

## Environment Availability

| Dependency | Required By | Available (locally) | Version | Fallback |
|------------|------------|---------------------|---------|----------|
| Go toolchain | Local SC2 cgo probe before writing YAML | ✓ | go1.26.3 darwin/arm64 (newer than .go-version 1.25.7 — fine for cross-compile) | — |
| `gh` CLI | (Optional) verifying branch protection status during planning | ✓ | (verified this session via `gh api`) | Skip protection-status verification; planner can document as "unknown, treat as advisory" |
| `goreleaser` | (Optional) Example 2 step 4 — end-to-end snapshot verification | (Not probed this session) | — | Skip; not load-bearing for SC2. Only `go build ./...` is required. |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** None blocking.

---

## Validation Architecture

### Test Framework

This phase has no Go source changes — therefore no Go test suite changes. The "test framework" for this phase IS the CI workflow itself: the build-check job is both the implementation and its own verifier. The validation evidence is "the workflow runs green on a real PR."

| Property | Value |
|----------|-------|
| Framework | GitHub Actions (`.github/workflows/build-check.yml` self-verifies) |
| Config file | `.github/workflows/build-check.yml` (the deliverable) |
| Quick run command | `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` (local — SC2) |
| Full suite command | Open a PR with the workflow file → observe the `windows-build` check turn green |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DIST-02 (SC1) | New file `.github/workflows/build-check.yml` exists and triggers on PR | smoke | `test -f .github/workflows/build-check.yml && yq '.on.pull_request.branches' .github/workflows/build-check.yml` | ❌ Wave 0 (file is the deliverable) |
| DIST-02 (SC2) | Local cgo probe passes before YAML is written | unit (one-shot) | `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` (expect exit 0) | ✅ (probe ran this session; passed at main HEAD `d69a67cb`) |
| DIST-02 (SC3) | The job fails the PR check when build fails | manual / one-time integration | Open a deliberately-broken PR (e.g., import a cgo-only package) and confirm the check is red | ❌ Wave 0 (requires a sentinel PR or manual injection — could be deferred to Pitfall 6 follow-up) |

### Sampling Rate

- **Per task commit:** `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` (the SC2 probe — should ALWAYS pass on commits to this phase since they only add YAML)
- **Per wave merge:** Open the PR, observe `windows-build` runs green
- **Phase gate:** PR with the new workflow merged to main, and at least one subsequent PR has fired the `windows-build` check successfully (proving the trigger works in practice, not just on paper)

### Wave 0 Gaps

- [ ] `.github/workflows/build-check.yml` itself — covers DIST-02 SC1
- [ ] (Recommended) A README note or comment in the workflow header documenting "check is advisory until branch protection is configured" — covers Pitfall 6 risk
- [ ] (Optional, deferrable) Sentinel-PR test to prove SC3 negative-path — covers DIST-02 SC3 strictly. If deferred, document as "trust by construction: `go build` exit non-zero is propagated by GitHub Actions to job status by default."

---

## Security Domain

This phase introduces a CI workflow with minimal attack surface. The threat model is small but worth enumerating:

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | (workflow uses no secrets) |
| V3 Session Management | no | — |
| V4 Access Control | yes (workflow permissions) | `permissions: contents: read` — least-privilege; matches sibling workflows |
| V5 Input Validation | no | (no user input; PR head SHA is implicit) |
| V6 Cryptography | no | — |
| V10 Malicious Code | yes (supply chain) | Pin all actions to exact SHA with version comment; use `pull_request` not `pull_request_target` to deny secret access to PR-author code |
| V14 Configuration | yes (CI) | Workflow committed to repo, version-controlled, reviewable; no inline secrets |

### Known Threat Patterns for GitHub Actions Workflows

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Compromised third-party action runs malicious code with repo write access | Elevation of Privilege | Pin actions to exact SHA (not floating tags); `permissions: contents: read` only |
| `pull_request_target` exposes secrets to fork PRs | Information Disclosure | Use `pull_request` (no secrets exposed); never `pull_request_target` for a build-only job |
| Cache poisoning via PR-controlled cache write | Tampering | `actions/setup-go@v6` write cache only on `push` events by default; PR runs read-only cache — no action needed [CITED: actions/setup-go docs] |
| Workflow injection via untrusted input in `run:` step | Tampering | This workflow has no user-controlled input strings — N/A |

**Recommendation:** Match the security posture of `macos-sign-verify.yml` exactly: SHA-pinned actions, `permissions: contents: read`, no secrets, `pull_request` event. The build-check workflow inherits the established secure baseline of the repo.

---

## Sources

### Primary (HIGH confidence)
- `.github/workflows/macos-sign-verify.yml` — closest sibling workflow, pattern reference (read this session)
- `.github/workflows/release.yml` — Go setup pattern reference (read this session)
- `.github/workflows/docker.yaml`, `podman.yml`, `nerdctl.yaml`, `vm.yaml` — `paths-ignore`, `ubuntu-24.04`, `permissions`, `timeout-minutes` convention (read this session)
- `.github/actions/setup-env/action.yaml` — composite action pattern (read this session)
- `.goreleaser.yaml` — proves windows/amd64 + CGO_ENABLED=0 cross-compile is the established release path (read this session)
- `go.mod`, `.go-version`, `hack/build/gotoolchain.sh` — Go version pinning mechanism (read this session)
- `.planning/research/PITFALLS.md` lines 314-327 — Pitfall 18 canonical text (read this session)
- `.planning/REQUIREMENTS.md` lines 29, 90, 105 — DIST-02 definition and traceability (grep this session)
- `.planning/ROADMAP.md` Phase 55 entry — success criteria source (in additional_context)
- Local probe execution this session — `go build`, `go vet`, `go list` all exited 0 under GOOS=windows GOARCH=amd64 CGO_ENABLED=0 at commit `d69a67cb`
- `gh api repos/:owner/:repo/branches/main/protection` → 404 Branch not protected (this session)
- `gh api repos/:owner/:repo/rulesets` → `[]` (this session)

### Secondary (MEDIUM confidence)
- [Go Wiki: WindowsCrossCompiling](https://go.dev/wiki/WindowsCrossCompiling) — official confirmation that `GOOS=windows GOARCH=amd64` cross-compile works from Linux for pure-Go programs and that cgo is disabled by default in cross-compile mode
- [actions/setup-go README caching section](https://github.com/actions/setup-go#caching-dependency-files-and-build-outputs) — caching is on by default in v6; module + build cache
- [actions/setup-go releases page](https://github.com/actions/setup-go/releases) — v6.4.0 exists upstream; v6.2.0 is the in-repo pin (consistency wins for this phase)

### Tertiary (LOW confidence)
- Upstream `kubernetes-sigs/kind` workflow list (WebFetch) — used only to confirm kinder is adding a *new* pattern (kind has no equivalent Windows build check), not to copy a pattern. No risk of being wrong here.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every action SHA and version is grep-verified against existing repo workflows; Go version is read from `.go-version`
- Architecture: HIGH — pattern matches the most recent CI work in the repo (`macos-sign-verify.yml`), and the deliverable is a single small file
- Pitfalls: HIGH — Pitfall 18 text is canonical (research/PITFALLS.md); local probe empirically verifies the current main is clean; Pitfall 6 (branch protection) is verified directly via `gh api`
- Open Questions: 5 listed, all minor (worst case: one-line workflow edits or deferred to follow-up)
- Assumptions: 5 listed, all low-risk

**Research date:** 2026-05-12
**Valid until:** 2026-06-12 (30 days — workflow YAML and Go toolchain are stable). If `go.mod` gets a major dependency bump (especially anything containerd / runc / mountinfo adjacent) before implementation, re-run the SC2 probe before writing YAML.
