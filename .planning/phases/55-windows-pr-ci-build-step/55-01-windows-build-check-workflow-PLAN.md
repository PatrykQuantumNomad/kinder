---
phase: 55-windows-pr-ci-build-step
plan: 01
type: execute
wave: 1
status: pending
depends_on: []
files_modified:
  - .github/workflows/build-check.yml
autonomous: false
requirements:
  - DIST-02

must_haves:
  truths:
    - "A new `.github/workflows/build-check.yml` workflow runs `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` on every pull_request to main (SC1)."
    - "The local cgo transitive-dep probe (`CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...`) exits 0 at the implementation HEAD BEFORE the workflow YAML is written (SC2 — re-verify in case main moved past d69a67cb)."
    - "The `windows-build` job in the workflow propagates the `go build` non-zero exit code to the GitHub Actions job status, causing the resulting PR check to fail on cross-compile regressions (SC3 — workflow-level blocking)."
    - "The PR check name resolves to `Windows Build Check / windows-build` (specific, unambiguous for any future branch-protection / required-status-check configuration)."
    - "The workflow uses SHA-pinned third-party actions matching the repo convention (`actions/checkout@de0fac2e... # v6.0.2`, `actions/setup-go@7a3fe6cf... # v6.2.0`) and pins the Go toolchain via `go-version-file: .go-version`."
  artifacts:
    - path: ".github/workflows/build-check.yml"
      provides: "PR-event Windows cross-compile build check (SC1, SC3 workflow-level)"
      contains: "GOOS: windows"
      min_lines: 25
  key_links:
    - from: ".github/workflows/build-check.yml"
      to: ".go-version"
      via: "actions/setup-go with go-version-file"
      pattern: "go-version-file:\\s*\\.go-version"
    - from: ".github/workflows/build-check.yml"
      to: "go build ./..."
      via: "env block sets CGO_ENABLED=0 GOOS=windows GOARCH=amd64 on the run step"
      pattern: "GOOS:\\s*windows"
    - from: ".github/workflows/build-check.yml"
      to: "pull_request event on main"
      via: "on.pull_request.branches: [main] with paths-ignore for site dirs"
      pattern: "pull_request:"
---

<objective>
Add a PR-event GitHub Actions workflow that cross-compiles the entire Go module for `windows/amd64` from a Linux runner on every pull request to `main`, satisfying DIST-02 by closing the feedback-loop gap between PR-open and release-tag for silent Windows compile regressions (Pitfall 18 — cgo transitive dependency leaks).

Purpose:
- Today, the only signal for a Windows cross-compile regression is `goreleaser build` failing at release-tag time — weeks after the offending PR merged. A direct-or-transitive Go dependency that activates cgo on Linux/macOS will pass `go build ./...` natively but fail under `GOOS=windows CGO_ENABLED=0`. The new workflow runs that exact cross-compile on every PR, so the failure surfaces at PR-open, attributed to the specific change.
- The implementation surface is minimal — one new ~30-line YAML file. The phase is a "smaller is better" exercise per RESEARCH: NO matrix (one target only), NO `go vet` (Pitfall 19 / Strict-Windows runtime is explicitly out of scope per ROADMAP Non-Goals), NO `concurrency:` block (not used elsewhere in this repo), NO manual `actions/cache` (setup-go v6 caches by default), NO `fetch-depth: 0` (build needs only HEAD).
- The local SC2 re-probe is a NON-NEGOTIABLE first step: RESEARCH ran the probe at HEAD `d69a67cb` and observed exit 0, but SC2 wording requires the probe re-verified locally at implementation HEAD in case `main` moved between research and execution. Writing the YAML without re-probing risks committing a workflow file alongside a regression that the workflow itself would catch — embarrassing and avoidable.
- "Blocking" in DIST-02 has two layers: (a) workflow-level — the job's non-zero exit produces a red PR check — fully delivered by the YAML alone; (b) merge-level — a red check prevents merge, which requires repo-level branch-protection configuration. RESEARCH verified via `gh api` that `main` is NOT currently protected and no rulesets exist. This plan delivers layer (a) (the in-scope reading of DIST-02 SC3 per RESEARCH recommendation), and surfaces layer (b) as a `[NEEDS USER CONFIRMATION]` optional task — making the check merge-blocking changes merge behavior repo-wide and is a deliberate user decision, not a side effect of this phase.

Output:
- `.github/workflows/build-check.yml` (NEW): single-job `windows-build` workflow on `ubuntu-24.04`, `permissions: contents: read`, triggers on `pull_request` to `main` (with `paths-ignore` for site dirs) plus `workflow_dispatch`, runs `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` via an `env:` block on the build step. Header comment documents the `ubuntu-24.04`-vs-`ubuntu-latest` deviation from DIST-02 wording (repo pins ubuntu versions; `ubuntu-latest` floats and has caused CI drift) and the workflow-level vs merge-level "blocking" distinction.
- No code changes. No `Makefile` change. No `hack/` script change. No docs change (CI changes are not user-visible — release notes / changelog mention is a separate concern out of scope).
</objective>

<execution_context>
@/Users/patrykattc/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/REQUIREMENTS.md
@.planning/phases/55-windows-pr-ci-build-step/55-RESEARCH.md

# Closest sibling workflow — pattern reference for header, triggers, permissions, pinned SHAs
@.github/workflows/macos-sign-verify.yml

# Go version single-source-of-truth — referenced via go-version-file
@.go-version
</context>

<interfaces>
<!-- SC2 local probe — must exit 0 before any YAML is written -->
<!-- Source: RESEARCH Example 2; Pitfall 1 (Pitfall 18 restated) -->

Local cgo transitive-dep probe (DIST-02 SC2):

  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...
  echo "exit: $?"   # MUST be 0

If the probe fails, the failure message names the offending import chain. Recovery options (Pitfall 1):
  - Add `//go:build !windows` to the offending file
  - Switch to a pure-Go alternative (e.g., fsnotify v1.10+)
  - Revert the cgo-introducing dependency bump
DO NOT proceed to writing the workflow YAML if the probe fails — investigate the dep chain first.

Optional confidence probe (no-op if no cgo files are reported under GOOS=windows):

  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go list -f '{{.ImportPath}} {{.CgoFiles}}' ./... | grep -v '\[\]' || echo "clean — no cgo files under GOOS=windows"

---

Pinned action SHAs (VERIFIED against existing repo workflows during RESEARCH):

  actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
  actions/setup-go@7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5 # v6.2.0

Pin format MUST include the trailing `# vX.Y.Z` version comment — this is the repo-wide convention used in all six existing workflows. A bare SHA without the comment fails Pitfall 4.

---

PR-check name resolution (Pitfall 7):

  GitHub renders the check name in the PR UI as `<workflow .name> / <job .name>`.
  This plan produces: `Windows Build Check / windows-build`

  Workflow `name: Windows Build Check`
  Job key: `windows-build:` (the YAML key)
  Job `name: windows-build` (explicit, matching the key for symmetry)

  Specific enough that a future branch-protection rule listing `windows-build` as a required status check is unambiguous — no other workflow in this repo has a job called `windows-build`.

---

Trigger contract (RESEARCH Example 1, Pitfall 2):

  on:
    pull_request:
      branches: [main]
      paths-ignore:
        - 'site/**'           # Hugo site (no Go source)
        - 'kinder-site/**'    # Astro site (no Go source)
    workflow_dispatch: {}

Use `pull_request` (NOT `pull_request_target` — Pitfall 2 — no secrets exposed to PR-author code, runs PR head SHA not base).
Use `workflow_dispatch: {}` to enable manual re-runs from the Actions UI without opening a PR.

---

Runner choice rationale (Pitfall 3, Open Question 4):

  runs-on: ubuntu-24.04   # NOT ubuntu-latest

  DIST-02 wording says "on ubuntu-latest". Four sibling PR workflows in this repo pin `ubuntu-24.04`. `ubuntu-latest` is a floating alias that GitHub silently bumps (20.04 → 22.04 → 24.04), historically causing green-on-Friday red-on-Monday surprises. RESEARCH recommendation: use `ubuntu-24.04` for consistency, document the deviation from DIST-02 literal wording in the workflow header comment.

---

DO NOT (Anti-patterns from RESEARCH):

  - DO NOT add a `matrix:` over goos/goarch — single target (windows/amd64); Windows ARM64 is explicitly excluded per `.goreleaser.yaml` (`ignore: { goos: windows, goarch: arm64 }`).
  - DO NOT add a `go vet` step — DIST-02 is build-only; ROADMAP Non-Goals defers Strict-Windows runtime support.
  - DO NOT add `concurrency: { group: ..., cancel-in-progress: true }` — not used in any existing workflow; introducing it here would be a one-off.
  - DO NOT add manual `actions/cache` — `actions/setup-go@v6` caches modules + build outputs by default.
  - DO NOT add `fetch-depth: 0` to checkout — build needs only HEAD; only goreleaser-driven workflows need full history.
  - DO NOT use the `setup-env` composite action — it does `make install` + `kubectl` install, neither of which is needed for a build-only check.
  - DO NOT add a job to `docker.yaml` instead of creating a new file — SC1 explicitly mandates the new path `.github/workflows/build-check.yml`.
  - DO NOT narrow the build target (e.g., `./cmd/...`) — `./...` matches DIST-02 wording verbatim.
</interfaces>

<tasks>

<task type="auto" n="1">
  <name>Task 1: Re-run local SC2 cgo probe at implementation HEAD and create .github/workflows/build-check.yml</name>
  <files>.github/workflows/build-check.yml</files>
  <atomic_commit>ci(55-01): add windows/amd64 cross-compile build check on PR (DIST-02)</atomic_commit>
  <action>
This task has TWO sub-steps that MUST execute in order. The local probe is a hard gate — if it fails, stop and investigate the cgo dependency chain BEFORE writing any YAML.

---

### Sub-step A — Re-run the SC2 local probe (NON-NEGOTIABLE first step)

RESEARCH ran the probe at HEAD `d69a67cb` and observed exit 0, but SC2 requires the probe re-verified at implementation HEAD. Run from the repo root:

```bash
go version           # report toolchain
cat .go-version      # expect: 1.25.7
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...
echo "build exit: $?"   # MUST be 0
```

(Optional confidence probe — confirms no cgo files in the Windows build graph):

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go list -f '{{.ImportPath}} {{.CgoFiles}}' ./... | grep -v '\[\]' || echo "clean"
```

If `go build` exits 0, proceed to Sub-step B.

If `go build` exits non-zero, STOP. The failure output names the import chain. Common recoveries (Pitfall 1 in RESEARCH):
- Add `//go:build !windows` build tag to the offending file
- Switch to a pure-Go alternative (e.g., fsnotify v1.10+ which is pure-Go on Windows via x/sys/windows)
- Revert the cgo-introducing dependency bump

Do NOT write the workflow YAML until the probe exits 0. The workflow's job is to catch this exact failure on every future PR; committing the workflow alongside a current failure would produce a red check on the very PR that introduces the workflow — embarrassing and avoidable.

Record the probe result in the eventual atomic commit message body (e.g., `Local probe at HEAD <sha>: CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./... exit 0`).

---

### Sub-step B — Write `.github/workflows/build-check.yml`

Create the NEW file with the EXACT content below. The values and structure are the RESEARCH-verified baseline (Example 1); no deviations beyond what is specified.

```yaml
# Source: .planning/phases/55-windows-pr-ci-build-step/55-RESEARCH.md (Example 1)
# Purpose: DIST-02 — Pitfall 18 prevention. Cross-compile the Go module for
# windows/amd64 from a Linux runner on every PR to main, so silent cgo transitive
# dependency regressions surface at PR-open instead of release-tag time.
#
# Runner pin: this workflow uses `ubuntu-24.04` (NOT `ubuntu-latest` per DIST-02
# literal wording). Rationale: the repo pins ubuntu versions in four other PR
# workflows (docker.yaml, nerdctl.yaml, podman.yml, vm.yaml); `ubuntu-latest`
# floats and has caused CI drift industry-wide. The DIST-02 intent (a Linux
# Ubuntu runner) is fully satisfied by `ubuntu-24.04`.
#
# Check name in PR UI: "Windows Build Check / windows-build"
# Workflow-level "blocking": this job's non-zero exit fails the PR check status.
# Merge-level "blocking" (preventing merge on red): requires repo-level branch
# protection on `main` listing `windows-build` as a required status check. As of
# this workflow's creation, `main` is NOT protected (verified via `gh api`); the
# check is therefore advisory until branch protection is configured. See Plan
# 55-01 Task 3 (optional, [NEEDS USER CONFIRMATION]).
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

WHY EVERY ELEMENT (do NOT change without re-reading RESEARCH Example 1 and the Pitfalls section):

- Header comment block: documents (a) the `ubuntu-24.04`-vs-`ubuntu-latest` deviation from DIST-02 literal wording with rationale, and (b) the workflow-level vs merge-level "blocking" distinction so a future reader does not assume the check is merge-blocking. Both are explicitly called out in RESEARCH Pitfall 3 and Pitfall 6.
- `name: Windows Build Check`: specific enough to disambiguate from any other "Build" workflow in the repo (`docker.yaml`, etc. do not have job names that collide).
- `on: pull_request: branches: [main]`: matches the four existing PR-event workflows in this repo. PR-event ensures we check the PR head SHA, not main — which is the correct semantic for catching regressions before merge.
- `paths-ignore: ['site/**', 'kinder-site/**']`: both site directories contain no Go source. Skipping site-only PRs saves ~30s of CI minutes per docs PR. `docker.yaml` excludes `site/**`; adding `kinder-site/**` is incremental (the kinder-site dir exists at repo root and contains no Go code).
- `workflow_dispatch: {}`: enables manual re-runs from the Actions UI without opening a PR. Matches `macos-sign-verify.yml`. Cheap to include; saves a future cycle when investigating a flake.
- `permissions: contents: read`: least-privilege; matches all six existing workflows in the repo. The build-check has no secrets, no writes, no need for any token elevation.
- `runs-on: ubuntu-24.04`: see header comment rationale. Pin matches four sibling PR workflows.
- `timeout-minutes: 10`: generous for a single `go build ./...` (RESEARCH estimate: <60s warm, <3min cold). 10 is enough headroom for cold cache without being so long that a hung job wastes meaningful CI minutes.
- `actions/checkout@de0fac2e... # v6.0.2`: SHA-pinned with version comment per repo convention (Pitfall 4). No `with: fetch-depth: 0` — build needs only HEAD; full history is only needed by goreleaser-driven workflows.
- `actions/setup-go@7a3fe6cf... # v6.2.0`: matches the SHA used in `release.yml`, `vm.yaml`, and `macos-sign-verify.yml`. v6.x caches modules + build outputs by default — no manual `actions/cache` needed.
- `go-version-file: .go-version`: single-source-of-truth for the Go toolchain version (content: `1.25.7`). Avoids inline version drift across workflows.
- `env:` block on the build step (NOT inline before the command): GitHub Actions idiomatic form. GitHub renders the env block as a collapsible section in the step header, making failures easier to diagnose in CI logs. (Inline `CGO_ENABLED=0 GOOS=windows ... go build` works too but is less log-friendly.)
- `CGO_ENABLED: "0"`: string-quoted per YAML 1.1 convention (`0` and `1` are valid octal/integer in YAML 1.1 — quoting prevents any parsing ambiguity).
- `go build ./...`: exact wording from DIST-02. `./...` is the canonical "all packages" pattern; do not narrow (e.g., `./cmd/...`) — that would let a non-binary package regression slip through.

After writing the file, run the verification block (see <verify>) and create the atomic commit. The commit message body SHOULD include the probe result from Sub-step A.
  </action>
  <verify>
    <automated>set -e; CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...; test -f .github/workflows/build-check.yml; grep -q "^name: Windows Build Check$" .github/workflows/build-check.yml; grep -q "^      name: windows-build$" .github/workflows/build-check.yml; grep -q "runs-on: ubuntu-24.04" .github/workflows/build-check.yml; grep -q "^      - main$" .github/workflows/build-check.yml; grep -q "paths-ignore:" .github/workflows/build-check.yml; grep -q "'site/\*\*'" .github/workflows/build-check.yml; grep -q "'kinder-site/\*\*'" .github/workflows/build-check.yml; grep -q "workflow_dispatch: {}" .github/workflows/build-check.yml; grep -q "permissions:" .github/workflows/build-check.yml; grep -q "contents: read" .github/workflows/build-check.yml; grep -q "actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2" .github/workflows/build-check.yml; grep -q "actions/setup-go@7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5 # v6.2.0" .github/workflows/build-check.yml; grep -q "go-version-file: .go-version" .github/workflows/build-check.yml; grep -q 'CGO_ENABLED: "0"' .github/workflows/build-check.yml; grep -q "GOOS: windows" .github/workflows/build-check.yml; grep -q "GOARCH: amd64" .github/workflows/build-check.yml; grep -q "run: go build \./\.\.\.$" .github/workflows/build-check.yml; grep -q "timeout-minutes: 10" .github/workflows/build-check.yml; ! grep -q "pull_request_target:" .github/workflows/build-check.yml; ! grep -q "matrix:" .github/workflows/build-check.yml; ! grep -q "go vet" .github/workflows/build-check.yml; ! grep -q "concurrency:" .github/workflows/build-check.yml; ! grep -q "fetch-depth:" .github/workflows/build-check.yml</automated>
  </verify>
  <done>SC2: local probe `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` exited 0 before YAML was written. SC1: `.github/workflows/build-check.yml` exists, triggers on `pull_request` to `main` with `paths-ignore` for both site dirs plus `workflow_dispatch`, runs on `ubuntu-24.04`, uses SHA-pinned `actions/checkout@de0fac2e... # v6.0.2` and `actions/setup-go@7a3fe6cf... # v6.2.0` with `go-version-file: .go-version`, and runs `go build ./...` with `CGO_ENABLED=0 GOOS=windows GOARCH=amd64` set via an `env:` block. SC3 (workflow-level): the build step's non-zero exit propagates to the job's status; no `if: success()` overrides or `continue-on-error: true` neutralize the failure. Header comment documents the `ubuntu-24.04` deviation and the workflow-level vs merge-level blocking distinction. Atomic commit landed on `main` with body referencing the local probe result.</done>
</task>

<task type="checkpoint:human-verify" n="2" gate="blocking">
  <name>Task 2: Observe the windows-build check fire green on a real PR or via workflow_dispatch</name>
  <what-built>
The new `.github/workflows/build-check.yml` workflow from Task 1 is committed to `main`. This checkpoint confirms the workflow actually runs on GitHub Actions and produces a green check, proving the trigger wiring and `go build` cross-compile work end-to-end — not just on paper.

There are two valid verification paths; either is sufficient for SC1+SC3 workflow-level satisfaction:

**Path A (preferred — exercises the `pull_request` trigger):**
A subsequent PR to `main` opens (any PR — could be the next phase's work, an unrelated dependency bump, or a deliberate trivial doc PR). The PR's Checks panel SHOULD list `Windows Build Check / windows-build` among the runs. The check SHOULD complete green (exit 0).

**Path B (immediate — exercises the `workflow_dispatch` trigger):**
Manually trigger the workflow from the Actions UI on `main` via `gh workflow run build-check.yml --ref main`, then observe the run completes green.

Path B is the **fallback** because the repo's `branching_strategy=none` means commits land directly on main (no automatic next-PR is guaranteed). Path B provides immediate confidence; Path A is the production-trigger smoke test.
  </what-built>
  <how-to-verify>
1. Confirm the Task 1 commit reached GitHub (`git log -1 --oneline` shows the `ci(55-01): add windows/amd64 cross-compile build check on PR (DIST-02)` commit; `git status` is clean; `git fetch origin main && git rev-parse HEAD == origin/main`).

2. **Path B (immediate fallback — DO THIS FIRST since branching_strategy=none):**

   ```bash
   gh workflow run build-check.yml --ref main
   # wait ~5-10 seconds for the dispatch to register
   gh run list --workflow build-check.yml --limit 1
   # observe the latest run; once status is "completed", confirm conclusion:
   gh run list --workflow build-check.yml --limit 1 --json conclusion,status,databaseId,headBranch
   ```

   Expected JSON output (approximately):
   ```json
   [{"conclusion":"success","status":"completed","databaseId":<some-id>,"headBranch":"main"}]
   ```

   `conclusion: success` proves the workflow ran end-to-end and `go build ./...` under `GOOS=windows GOARCH=amd64 CGO_ENABLED=0` exited 0 on the runner. Record the `databaseId` (the run number) in the eventual SUMMARY.md so the green run is traceable.

3. **Path A (production-trigger smoke — DO opportunistically):**

   The next PR opened against `main` (any PR — could be the next phase's work) SHOULD list `Windows Build Check / windows-build` in its Checks panel. The check SHOULD be green for a clean PR. If a PR opens before this phase closes, capture the run ID; if no PR opens within the phase window, Path B alone is sufficient evidence for SC1+SC3 — the `on: pull_request` trigger is declarative YAML that GitHub Actions parses identically for any PR, so the dispatch run proves the build step itself works.

4. Confirm the resulting check name (visible in `gh run view <run-id> --json name,jobs`) is exactly `Windows Build Check` (workflow) with a job named `windows-build` (Pitfall 7 — the resolved check name in any future branch-protection config is `Windows Build Check / windows-build`).

5. **Optional sentinel-PR test for SC3 negative path (RESEARCH calls this OPTIONAL — "trust by construction"):**
   The strictest reading of SC3 ("failure fails the PR check") could be proven by deliberately introducing a cgo-only import in a sentinel PR and confirming the check turns RED. RESEARCH explicitly recommends SKIPPING this — GitHub Actions propagates `go build` non-zero exit to job status by default, and the sentinel imposes test-PR cleanup overhead. SKIP unless a reviewer flags trust-by-construction as insufficient.

**Resume signal — type one of:**
- `green-via-dispatch <run-id>` — Path B succeeded; provide the run ID for traceability
- `green-via-pr <pr-number>` — Path A succeeded; provide the PR number
- `red <run-id> <brief-failure>` — the check failed; provide the run ID and the failure mode so the phase can pivot to debugging the cgo dep chain or the workflow syntax
  </how-to-verify>
  <resume-signal>Type `green-via-dispatch <run-id>`, `green-via-pr <pr-number>`, or `red <run-id> <brief-failure>`.</resume-signal>
</task>

<task type="checkpoint:decision" n="3" gate="blocking">
  <name>Task 3 (OPTIONAL — [NEEDS USER CONFIRMATION]): Configure branch protection on `main` to make windows-build a required status check</name>
  <decision>Should `main` be configured with a branch-protection rule (or repo ruleset) that requires the `Windows Build Check / windows-build` check to pass before merge?</decision>
  <context>
DIST-02 wording says "blocking (failure fails the PR check)." RESEARCH established this phrase has two layers:

- **Layer (a) — workflow-level:** the job's `go build` non-zero exit propagates to the GitHub check status, painting the PR check red. Task 1 delivers this. RESEARCH RECOMMENDS treating layer (a) as the in-scope SC1/SC3 deliverable for this phase.
- **Layer (b) — merge-level:** a red check actually prevents the PR from being merged. Layer (b) requires repo-level branch-protection configuration listing the check as required. This phase's YAML alone does NOT deliver layer (b).

RESEARCH verified the current repo state via `gh api`:
```
$ gh api repos/:owner/:repo/branches/main/protection
{"message":"Branch not protected", ... "status":"404"}
$ gh api repos/:owner/:repo/rulesets
[]
```

`main` has no protection and no rulesets. So today, a red `windows-build` check is advisory only — a developer with merge permissions could merge a PR with a red check.

**Why this is gated `[NEEDS USER CONFIRMATION]`:**
Configuring branch protection changes merge behavior repo-wide. It would mean:
- Future PRs CANNOT be merged with red checks (windows-build OR any other required check the user lists).
- The repo's current `branching_strategy=none` model (commits land directly on main via `git push`) would be disrupted IF the protection rule also enforces "require pull request before merging" — without that sub-option, direct pushes still bypass checks (which negates the protection).
- Solo-developer workflows often prefer no protection for velocity; team-scale workflows prefer protection for safety. The user is the only one who can decide which mode this repo is in NOW.

This task is therefore a `checkpoint:decision` — present the choice and let the user select.
  </context>
  <options>
    <option id="defer">
      <name>Defer (RECOMMENDED by RESEARCH for this phase)</name>
      <pros>Phase 55 ships clean with workflow-level "blocking" satisfying the in-scope reading of DIST-02 SC3. Merge-level blocking is a separate, deliberate decision that can be made as a follow-up "CI policy" phase or a one-off `gh api` call when the user is ready. No surprise to the repo's current direct-push-to-main workflow.</pros>
      <cons>Strictest reading of DIST-02 SC3 ("blocking") is not delivered at the merge-prevention layer. A future reviewer who reads "blocking" as "merge-preventing" might flag a gap. Mitigation: document the layer (a) vs layer (b) distinction in the workflow file header (Task 1 already does this) and in the phase SUMMARY.md.</cons>
    </option>
    <option id="configure-now">
      <name>Configure branch protection NOW (require `windows-build` check)</name>
      <pros>Strictest possible delivery of DIST-02 SC3. A red `windows-build` check will literally prevent merging a PR. Establishes the "required check" pattern in the repo for future CI work to extend.</pros>
      <cons>Changes merge behavior repo-wide; requires deliberate handling of the repo's current `branching_strategy=none` model. The user must decide:
(a) Whether to ALSO require "Pull request before merging" (otherwise direct pushes bypass the check — making the protection cosmetic).
(b) Whether to also list the other CI checks (`docker.yaml`, `nerdctl.yaml`, etc.) as required (asymmetry — only `windows-build` required, others advisory — is confusing).
(c) Whether to allow admin bypass (`enforce_admins`) — the user is presumably an admin and would otherwise be locked out of urgent direct-push fixes.

Implementation if chosen — Claude can do this via `gh api`:
```bash
gh api -X PUT repos/:owner/:repo/branches/main/protection \
  -f required_status_checks[strict]=true \
  -f required_status_checks[contexts][]="Windows Build Check / windows-build" \
  -f enforce_admins=false \
  -F required_pull_request_reviews=null \
  -f restrictions=null
```
(Exact flags depend on the user's answers to (a)/(b)/(c); the gh-api call shape is verified per GitHub REST API v2022-11-28.)</cons>
    </option>
    <option id="document-only">
      <name>Document only (add a short note to PROJECT.md Key Decisions and the workflow header)</name>
      <pros>Captures the conscious decision to NOT protect main in this phase, so future-self / future-reviewer doesn't re-litigate. Lowest-effort path that preserves an audit trail.</pros>
      <cons>Adds a documentation touch outside the strict scope of DIST-02 (which is about CI YAML, not docs). The workflow header comment (already in Task 1) is arguably sufficient; PROJECT.md addition may be over-engineering for a single-decision phase.</cons>
    </option>
  </options>
  <resume-signal>Select: `defer`, `configure-now`, or `document-only`. If `configure-now`, also answer: (a) require PRs before merge? (b) which other checks to require? (c) enforce admins? If `document-only`, Claude will draft a one-row PROJECT.md Key Decisions entry for review before commit.</resume-signal>
</task>

</tasks>

<verification>

## SC1 — Workflow file exists and triggers on PR

```bash
test -f .github/workflows/build-check.yml
grep -q "^name: Windows Build Check$" .github/workflows/build-check.yml
grep -q "pull_request:" .github/workflows/build-check.yml
grep -q "^      - main$" .github/workflows/build-check.yml
grep -q "run: go build \./\.\.\.$" .github/workflows/build-check.yml
grep -q 'CGO_ENABLED: "0"' .github/workflows/build-check.yml
grep -q "GOOS: windows" .github/workflows/build-check.yml
grep -q "GOARCH: amd64" .github/workflows/build-check.yml
```

All commands MUST exit 0. The workflow file MUST exist at exactly `.github/workflows/build-check.yml`, declare `pull_request` on `main` as a trigger, and run `go build ./...` with `CGO_ENABLED=0 GOOS=windows GOARCH=amd64` set via env.

## SC2 — Local cgo probe passed BEFORE the workflow YAML was written

```bash
# Re-runnable at any time; must exit 0 at the implementation commit's HEAD
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...
echo "exit: $?"   # MUST be 0
```

The Task 1 atomic commit message body MUST reference the probe result. RESEARCH already observed exit 0 at HEAD `d69a67cb`; the SC2 obligation is that the probe be re-run at implementation HEAD (in case main moved) — the commit message body is the audit trail.

## SC3 — Workflow-level blocking: failure fails the PR check

Structural verification (no `continue-on-error: true` or `if: success()` neutralizes the build step):

```bash
! grep -q "continue-on-error: true" .github/workflows/build-check.yml
! grep -q "if: success()" .github/workflows/build-check.yml
```

End-to-end verification (Task 2 checkpoint output):

```bash
gh run list --workflow build-check.yml --limit 1 --json conclusion,status
# Expected: {"conclusion":"success","status":"completed"}
```

A successful `workflow_dispatch` run OR a green check on a real PR proves the workflow's `go build` step's exit code is propagated to the job status by GitHub Actions' default behavior. Per RESEARCH "Validation Architecture" — this is the trust-by-construction path; the optional sentinel-PR test for the negative path is explicitly NOT required.

## Pin and convention adherence

```bash
grep -q "actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2" .github/workflows/build-check.yml
grep -q "actions/setup-go@7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5 # v6.2.0" .github/workflows/build-check.yml
grep -q "go-version-file: .go-version" .github/workflows/build-check.yml
grep -q "permissions:" .github/workflows/build-check.yml
grep -q "contents: read" .github/workflows/build-check.yml
grep -q "runs-on: ubuntu-24.04" .github/workflows/build-check.yml
```

All MUST exit 0. SHA-pinned actions with version comments per repo convention (Pitfall 4); least-privilege permissions matching siblings; pinned ubuntu version (Pitfall 3 — `ubuntu-latest` floats).

## Out-of-scope guard (anti-patterns NOT introduced)

```bash
! grep -q "pull_request_target:" .github/workflows/build-check.yml    # Pitfall 2
! grep -q "matrix:" .github/workflows/build-check.yml                  # one target only
! grep -q "go vet" .github/workflows/build-check.yml                   # build-only per ROADMAP
! grep -q "concurrency:" .github/workflows/build-check.yml             # not used elsewhere
! grep -q "fetch-depth:" .github/workflows/build-check.yml             # build needs only HEAD
! grep -q "uses: actions/cache@" .github/workflows/build-check.yml     # setup-go v6 caches by default
```

All MUST exit 0. These guards enforce the "smaller is better" scope discipline established in RESEARCH and the ROADMAP Non-Goals.

</verification>

<success_criteria>

Plan 55-01 is complete when ALL the following are true:

1. **DIST-02 SC1 satisfied:** `.github/workflows/build-check.yml` exists on `main`, triggers on `pull_request` to `main` (with `paths-ignore` for `site/**` and `kinder-site/**`) plus `workflow_dispatch`, runs `go build ./...` with `CGO_ENABLED=0 GOOS=windows GOARCH=amd64` on `ubuntu-24.04`, and uses SHA-pinned `actions/checkout@de0fac2e... # v6.0.2` and `actions/setup-go@7a3fe6cf... # v6.2.0` with `go-version-file: .go-version`. (Verified by the verification block grep suite.)

2. **DIST-02 SC2 satisfied:** The local cgo probe `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` exited 0 at implementation HEAD before the workflow YAML was written, and the result is referenced in the Task 1 atomic commit message body. (RESEARCH already observed exit 0 at HEAD `d69a67cb`; Sub-step A re-verifies at implementation HEAD.)

3. **DIST-02 SC3 satisfied (workflow-level):** The workflow's `go build` step's non-zero exit propagates to the GitHub Actions job status — no `continue-on-error: true`, no `if: success()` neutralizes the failure (structural check). At least one workflow run (via `workflow_dispatch` per Task 2 Path B, or a real PR per Task 2 Path A) completes with `conclusion: success` on `main`'s HEAD, proving the trigger wiring and `go build` cross-compile work end-to-end.

4. **Out-of-scope guards green:** The workflow does NOT contain `matrix:`, `go vet`, `concurrency:`, `fetch-depth:`, `pull_request_target:`, or manual `actions/cache` usage. The "smaller is better" scope discipline holds.

5. **PR-check name resolves to `Windows Build Check / windows-build`:** workflow `name: Windows Build Check`, job key `windows-build`, job `name: windows-build`. Unambiguous for any future branch-protection / required-status-check configuration.

6. **Task 3 outcome recorded:** The branch-protection decision (defer / configure-now / document-only) is captured — either by `defer` selection (no action — note in SUMMARY.md), by `configure-now` execution (an additional commit / `gh api` invocation), or by `document-only` execution (a PROJECT.md Key Decisions row commit). The phase SUMMARY.md records the chosen path.

7. **REQUIREMENTS.md DIST-02 traceability row transitions Pending → Complete** with reference to this plan and to the at-least-one green workflow run ID. (Updated as part of the phase-close commit, not this plan's task — flagged here so the phase verifier doesn't miss it.)

</success_criteria>

<output>
After completion, create `.planning/phases/55-windows-pr-ci-build-step/55-01-SUMMARY.md` capturing:

- The workflow file final contents (a quote of the committed YAML, for the audit trail)
- The SC2 local probe transcript (go version, .go-version content, `go build` exit code) — proving the probe re-ran at implementation HEAD
- The Task 2 checkpoint resume signal (`green-via-dispatch <run-id>` or `green-via-pr <pr-number>`) and a link to the green Actions run
- The Task 3 decision selection and any follow-up artifacts (`gh api` command output if `configure-now`; PROJECT.md row diff if `document-only`; "deferred per RESEARCH recommendation" note if `defer`)
- A short note confirming none of the out-of-scope items (matrix, go vet, concurrency, fetch-depth, pull_request_target, manual cache) were introduced
- The atomic commit hash(es) produced by this plan
</output>
