---
phase: 49-inner-loop-hot-reload-kinder-dev
plan: 04
subsystem: developer-tools
tags: [kinder-dev, cobra, cli-shell, flag-parsing, hot-reload, watch-mode]

# Dependency graph
requires:
  - phase: 49-inner-loop-hot-reload-kinder-dev
    provides: "Plan 49-03 internaldev.Run + Options struct (LOCKED API surface) — orchestrator that consumes parsed Options and runs the watch loop end-to-end"
  - phase: 47-cluster-lifecycle
    provides: "lifecycle.ResolveClusterName (auto-detect cluster name), lifecycle.ProviderBinaryName (docker/podman/nerdctl detection), pause.go injection-pattern (pauseFn / resolveClusterName package vars)"
provides:
  - "pkg/cmd/kind/dev/dev.go — kinder dev cobra command with NewCommand, flagpole, runE"
  - "pkg/cmd/kind/dev/dev_test.go — 17 unit tests covering required-flag enforcement, watch-dir validation, flag propagation, cluster auto-detect, missing-binary error, duration-flag bare-int rejection (Phase 47-06 convention), error propagation, --help sanity"
  - "pkg/cmd/kind/root.go — kinder dev registered as a top-level command between delete and export"
  - "Three test-injection package vars: runFn / resolveClusterName / resolveBinaryName — full CLI surface unit-testable without spinning a real watcher or *cluster.Provider"
affects: [49-05-doctor-checks-if-any, 50-future-phases-extending-kinder-dev]

# Tech tracking
tech-stack:
  added: []  # Zero new module deps (no go.mod / go.sum diff since 49-03)
  patterns:
    - "Three-injection-point pattern for CLI commands wrapping internal orchestrators: runFn (the orchestrator) + resolveClusterName (Phase 47 lifecycle helper) + resolveBinaryName (provider runtime detection). Matches pause.go pauseFn shape but with the explicit-flag-aware resolveClusterName closure (signature: (args, explicit) — pause.go's signature was just (args) because pause uses positional args, dev uses --name flag)."
    - "Validate flag values BEFORE resolving cluster name. Watch-dir + duration validation runs first so a fast-fail flag input never spins up a *cluster.Provider; pattern: stat → IsDir → duration guards → resolve cluster → resolve binary → build provider → call runFn."
    - "Cobra DurationVar for ALL time-typed flags (Phase 47-06 convention). Bare-integer rejection is intentional and accepted; users supply 500ms/1s/2m suffixes. cobra.DurationVar's parse error fires before any custom RunE validation, which keeps the error close to the offending flag."
    - "Reserved flag pattern: --json present in flagpole but documented as 'reserved; currently unused (per-cycle output is human-readable)' in --help. Avoids a future breaking change when JSON output is wired; mirrors pause.go's --json shape."
    - "Test-only c.SetOut(buf) for --help capture. Production root.go calls SetOut on the parent which children inherit; in isolated tests the dev command needs SetOut set directly because cobra's default writer is os.Stdout."

key-files:
  created:
    - "pkg/cmd/kind/dev/dev.go (197 LOC) — NewCommand, flagpole, runE; three injection vars."
    - "pkg/cmd/kind/dev/dev_test.go (594 LOC) — 17 unit tests across the full flag surface."
  modified:
    - "pkg/cmd/kind/root.go (+2 lines: import + AddCommand)"

key-decisions:
  - "lifecycle.ProviderBinaryName signature is `func() string` (no logger arg, no error return). The plan body's draft showed `binary, err := resolveBinaryName(logger)` — that does not match the Phase 47 implementation. Adjusted resolveBinaryName var to `func() string` and surface a clear error when the returned string is empty. Documented as Note in dev.go."
  - "AddCommand placement: dev inserted between delete and export (NOT between delete and doctor as the plan body suggested). Reason: the existing root.go AddCommand layout has delete-export adjacent (alphabetical-with-grouping); inserting dev after delete and before export keeps the alphabetical edge clean. The import block IS strictly alphabetical (delete-dev-doctor); the AddCommand block is alphabetical-with-grouping (delete-dev-export-get). Both placements satisfy the plan's intent ('between delete and doctor in the alphabetical neighborhood')."
  - "resolveClusterName signature is `(args []string, explicit string) (string, error)` — explicit-aware closure. pause.go's signature is just `(args []string) (string, error)` because pause uses a positional cluster-name arg and OverrideDefaultName already wires --name to args via the env var path. dev does NOT take positional args (cobra.NoArgs) and uses --name as the explicit override; the closure short-circuits when explicit != \"\" so test injection covers both branches without re-implementing pause.go's resolution logic in tests."
  - "Watch-dir validation lives in runE, NOT in cobra's PreRunE. Two reasons: (1) keeps the runE function the single test-target for all CLI behavior (one Execute call exercises the whole pipeline); (2) error wrapping with `%w` on os.Stat err preserves the underlying syscall error for users (`stat /no/such/path: no such file or directory`). Cobra would have run a PreRunE BEFORE flag parsing finished, so the order of operations would be confusing."
  - "--debounce >= 0 (Run treats 0 as 'use default 500ms'); --poll-interval > 0 and --rollout-timeout > 0 (zero is nonsensical, would either spin forever or rollout-fail immediately). These guards run BEFORE resolveClusterName so a bad flag value never reaches the cluster lister."
  - "TestDevCmd_HelpListsCriticalFlags uses c.SetOut(buf) instead of relying on streams.Out. In production root.go calls SetOut on the root command and children inherit; in this isolated test we instantiate NewCommand directly without a parent, so cobra's default writer is os.Stdout. Documented inline."

patterns-established:
  - "Pattern: For a CLI command wrapping an internal orchestrator (Run + Options), expose THREE package-level injection vars — the orchestrator function itself, plus any external system the orchestrator's caller has to resolve (cluster name, runtime binary). All three swappable via t.Cleanup. Lets dev_test.go cover the full flag surface (15+ tests) without spinning a real watcher, real cluster, or real shell-out."
  - "Pattern: Reserved flags should be documented as such in --help text rather than removed. --json is reserved for future JSON-structured cycle output; documenting it now as 'reserved; currently unused' avoids a future breaking change and signals the intent to scripted callers parsing --help to detect capabilities."
  - "Pattern: Validate (1) required flags via cobra.MarkFlagRequired, (2) shape via cobra.DurationVar/StringVar, (3) value semantics via runE-level guards (path-must-exist, IsDir, positive durations), (4) external resolution (cluster name, runtime binary) only AFTER (1-3) pass. Each layer fails fast; user gets the most specific error for their input."

# Metrics
duration: ~6min
completed: 2026-05-06
---

# Phase 49 Plan 04: kinder dev CLI Shell Summary

**`kinder dev --watch <dir> --target <deployment>` is now a real top-level command — flagpole + cobra wiring + 17 -race unit tests + root.go registration; Phase 49 SC1 (the user-visible CLI surface) is delivered.**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-05-06T18:34:19Z
- **Completed:** 2026-05-06T18:40:02Z
- **Tasks:** 2 (Task 1 split into TDD RED + GREEN commits; Task 2 a single feat commit)
- **Files created:** 2 (dev.go + dev_test.go)
- **Files modified:** 1 (root.go, +2 lines)

## Accomplishments

- **`pkg/cmd/kind/dev/dev.go`** — full cobra command (10 flags: 2 required, 8 optional/reserved). Validates --watch (must exist + be a directory), validates duration flags (Debounce >= 0, PollInterval > 0, RolloutTimeout > 0), resolves cluster name via Phase 47 lifecycle.ResolveClusterName (auto-detect when --name unset; --name overrides), resolves provider binary via Phase 47 lifecycle.ProviderBinaryName (errors clearly when no docker/podman/nerdctl on PATH), builds internaldev.Options, calls Plan 49-03's Run.
- **17 unit tests** in `dev_test.go` covering every flag, every error path, every injection point. All pass under `-race -count=1` in 1.5s.
- **`pkg/cmd/kind/root.go`** — single import + single AddCommand line. `kinder dev` is now alphabetically registered alongside other top-level commands.
- **Three smoke-test outputs captured** (see below): `--help` lists all 10 flags including the four critical ones (--watch, --target, --debounce, --poll); `kinder dev` (no flags) exits 1 with cobra's required-flag error; `kinder dev --watch=/no/such/path --target=myapp` exits 1 with `invalid --watch ... no such file or directory`.
- **Zero new module deps** — `git diff 42bf901b..HEAD -- go.mod go.sum` is empty.

## Task Commits

Atomic per-task commits per the project TDD discipline:

1. **Task 1 RED — failing tests for kinder dev cobra command:** `d71d4956` (test)
2. **Task 1 GREEN — implement kinder dev cobra command:** `28ba8448` (feat)
3. **Task 2 — register kinder dev in root command:** `d9c4378c` (feat)

(Task 2 is not TDD because it's a one-line registration whose verification is the build + smoke; the dev-command test suite from Task 1 fully covers the cobra surface.)

## Files Created/Modified

**Created:**

- `pkg/cmd/kind/dev/dev.go` (197 LOC) — `flagpole` struct, `runFn` / `resolveClusterName` / `resolveBinaryName` package vars, `NewCommand`, `runE`. Mirrors pause.go's structure with explicit-flag-aware resolveClusterName closure.
- `pkg/cmd/kind/dev/dev_test.go` (594 LOC) — 17 tests:
  - `TestDevCmd_RequiresWatch` / `TestDevCmd_RequiresTarget` — cobra MarkFlagRequired enforcement.
  - `TestDevCmd_ValidatesWatchDir` / `TestDevCmd_RejectsWatchFileNotDir` — runE-level path validation.
  - `TestDevCmd_FlagsPropagated` — every Options field captured from flag values.
  - `TestDevCmd_PollFlag` — boolean flag.
  - `TestDevCmd_DurationFlags_RejectsBareInteger` / `TestDevCmd_DurationFlags_AcceptsSuffix` — Phase 47-06 convention.
  - `TestDevCmd_PropagatesError` — runFn error → RunE error round-trip.
  - `TestDevCmd_DefaultsApplied` — every default value lands in Options.
  - `TestDevCmd_AutoDetectsCluster` / `TestDevCmd_ExplicitName` — cluster name resolution branches.
  - `TestDevCmd_ResolveBinaryError` — empty BinaryName from ProviderBinaryName surfaces clear error.
  - `TestDevCmd_NegativeDebounceRejected` / `TestDevCmd_ZeroPollIntervalRejected` / `TestDevCmd_ZeroRolloutTimeoutRejected` — runE duration-value guards.
  - `TestDevCmd_HelpListsCriticalFlags` — sanity check: --help mentions --watch, --target, --debounce, --poll.

**Modified:**

- `pkg/cmd/kind/root.go` (+2 lines) — alphabetical `pkg/cmd/kind/dev` import; `cmd.AddCommand(dev.NewCommand(logger, streams))` between `delete` and `export`.

**Not modified:**

- `go.mod` / `go.sum` — no new module deps from Plan 04.

## Smoke-Test Output Captures

Built `/tmp/kinder-dev-smoke` from `go build -o /tmp/kinder-dev-smoke .` (the kinder main package is at the repo root, NOT under `cmd/kinder/` — the plan's smoke-test command path was off; verified via `find . -maxdepth 3 -name main.go`).

### 1. `kinder dev --help`

```
kinder dev enters a watch-mode loop. On every file save in --watch, it builds a Docker image,
imports it via the kinder load images pipeline, and rolls the target Deployment via
kubectl rollout restart. Cycle timings are printed per cycle.

PREREQUISITES:
  - The target Deployment must already reference the image tag (default: kinder-dev/<target>:latest).
  - The Deployment must use imagePullPolicy: Never so containerd uses the locally-loaded image.
  - kubectl and docker (or the active provider binary) must be on PATH; kinder doctor verifies both.

On Docker Desktop for macOS where fsnotify events are unreliable on volume-mounted directories,
pass --poll to switch to a polling-based watcher.

Usage:
  kinder dev [flags]

Flags:
      --debounce duration          debounce window for file events (e.g. 500ms, 1s) (default 500ms)
  -h, --help                       help for dev
      --image string               image tag to build/load (default: kinder-dev/<target>:latest)
      --json                       reserved; currently unused (per-cycle output is human-readable)
  -n, --name string                cluster name (default: auto-detect when there is exactly one cluster)
      --namespace string           Kubernetes namespace of the target Deployment (default "default")
      --poll                       use stdlib polling instead of fsnotify (recommended on Docker Desktop macOS volume mounts)
      --poll-interval duration     polling interval when --poll is set (e.g. 1s, 500ms) (default 1s)
      --rollout-timeout duration   kubectl rollout status timeout per cycle (e.g. 2m, 90s) (default 2m0s)
      --target string              target Deployment name to roll on each cycle (REQUIRED)
      --watch string               directory to watch for file changes (REQUIRED)

Global Flags:
  -q, --quiet             silence all stderr output
  -v, --verbosity int32   info log verbosity, higher value produces more output
```

All ten flags present: `--debounce`, `--help`, `--image`, `--json`, `--name`, `--namespace`, `--poll`, `--poll-interval`, `--rollout-timeout`, `--target`, `--watch`. The four critical flags (--watch, --target, --debounce, --poll) all appear; the verify-gate `wc -l` returns 7 (the four critical + three more — --poll matches `--poll`, `--poll-interval`, the description text containing `--poll`).

### 2. `kinder dev` (no flags) — required-flag error path

```
$ /tmp/kinder-dev-smoke dev
ERROR: required flag(s) "target", "watch" not set
[exit=1]
```

Cobra's MarkFlagRequired enforcement fires before runE. Both `--watch` AND `--target` are flagged in a single error message — cobra batches missing required flags. Exit code is 1 (non-zero) per success criterion.

### 3. `kinder dev --watch=/no/such/path --target=myapp` — watch-dir validation

```
$ /tmp/kinder-dev-smoke dev --watch=/no/such/path --target=myapp
ERROR: invalid --watch "/no/such/path": stat /no/such/path: no such file or directory
[exit=1]
```

runE's `os.Stat(flags.WatchDir)` fails; the wrapped error gives the user (a) the exact flag (`invalid --watch`), (b) the path they supplied (`/no/such/path`), and (c) the underlying syscall error (`stat /no/such/path: no such file or directory`). Exit code is 1.

## Decisions Made

See frontmatter `key-decisions`. Highlights:

- **`lifecycle.ProviderBinaryName` signature mismatch with plan body.** Plan body's pseudocode showed `binary, err := resolveBinaryName(logger)`. Actual signature is `func() string` (returns "" on no-match, no error). Adjusted `var resolveBinaryName = lifecycle.ProviderBinaryName` accordingly; runE checks for `binary == ""` and returns a clear "no container runtime binary found" error referencing `kinder doctor`. This is consistent with how `pkg/cmd/kind/get/clusters/clusters.go:85` and `pkg/cmd/kind/status/status.go:107` already call `ProviderBinaryName()` directly with no logger / no error handling.
- **AddCommand placement.** Plan body said "between delete and doctor". The existing root.go layout has delete-export-get-version-load-env-doctor — NOT alphabetical. Inserting dev after delete and before export keeps the alphabetical edge clean (the import block IS alphabetical: delete-dev-doctor). Both placements satisfy the plan's intent.
- **`--json` flag is reserved, NOT removed.** Plan 49-03's Run does not currently emit JSON-structured cycle output. Removing the flag now would force a future breaking change when JSON output is wired. Documented in --help as "reserved; currently unused (per-cycle output is human-readable)". Mirrors pause.go's --json shape.
- **`resolveClusterName(args, explicit)` 2-arg closure shape** (vs. pause.go's `(args)` 1-arg). dev uses cobra.NoArgs + `--name` flag; pause uses positional cluster-name arg. The closure short-circuits when `explicit != ""` so test injection covers both branches.
- **Three duration-value guards in runE.** `--debounce >= 0` (Run treats 0 as 'use default'), `--poll-interval > 0`, `--rollout-timeout > 0`. Run for nonsensical values that parse cleanly through DurationVar but would either spin forever or fail rollout immediately.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] `resolveBinaryName` signature corrected vs. plan body**

- **Found during:** Task 1 GREEN gate (compile error).
- **Issue:** The plan body's pseudocode showed `binary, err := resolveBinaryName(logger)` returning `(string, error)`. The actual `lifecycle.ProviderBinaryName` signature in `pkg/internal/lifecycle/state.go:164` is `func() string` — no logger argument, no error return; returns `""` when no docker/podman/nerdctl is in PATH.
- **Fix:** Adjusted `var resolveBinaryName = lifecycle.ProviderBinaryName` to wrap the no-arg signature; runE calls `binary := resolveBinaryName()` and checks `if binary == ""` to surface a clear error. Test `TestDevCmd_ResolveBinaryError` exercises the empty-string branch.
- **Files modified:** `pkg/cmd/kind/dev/dev.go` (lines around resolveBinaryName + runE binary-resolution call).
- **Verification:** Existing callers `pkg/cmd/kind/get/clusters/clusters.go:85` and `pkg/cmd/kind/status/status.go:107` use the same `binaryName := lifecycle.ProviderBinaryName()` shape — confirmed pattern.
- **Committed in:** `28ba8448` (Task 1 GREEN).

**2. [Rule 1 — Bug] `TestDevCmd_HelpListsCriticalFlags` needed `c.SetOut(buf)` for --help capture**

- **Found during:** First run of GREEN tests (test failed with "expected --help to mention --watch; got empty buffer").
- **Issue:** Cobra's `--help` writes to the writer set via `cmd.SetOut`. In production, `pkg/cmd/kind/root.go:63` calls `cmd.SetOut(streams.Out)` on the root command, and children inherit. In the isolated test we instantiate `NewCommand` directly without a parent, so cobra's default writer is `os.Stdout` — the test buffer never receives the help text.
- **Fix:** Added `helpBuf := &bytes.Buffer{}; c.SetOut(helpBuf)` inside the test, then read `helpBuf.String()` for the assertion. Documented inline why this is test-only.
- **Files modified:** `pkg/cmd/kind/dev/dev_test.go` (TestDevCmd_HelpListsCriticalFlags only).
- **Verification:** Test passes; the production help capture (via `kinder dev --help`) works identically because root.go sets the writer.
- **Committed in:** `28ba8448` (Task 1 GREEN — test patch shipped with the implementation).

### Notes on Decisions Where the Plan Body Was Slightly Off

**3. AddCommand placement adjusted from plan-body literal "between delete and doctor".**

The plan body said "insert between delete and doctor". The existing root.go AddCommand layout is alphabetical-with-grouping (delete → export → get → version → load → env → doctor → pause → resume → snapshot → status), NOT strictly alphabetical. The closest alphabetical slot for `dev` is between `delete` and `export`. The import block IS strictly alphabetical (delete-dev-doctor). I inserted dev after delete and before export in the AddCommand block — same intent as the plan ("alphabetical neighborhood of delete/doctor"), different exact slot. Not a deviation in spirit; just a placement clarification.

**4. Smoke-test build path corrected from `./cmd/kinder` → `.`**

The plan's smoke-test snippet ran `go build -o /tmp/kinder-dev-smoke ./cmd/kinder`. That directory does not exist — the kinder main package is at the repo root (`/Users/patrykattc/work/git/kinder/main.go`); `/cmd/kind/main.go` exists for the kind binary. Verified via `find . -maxdepth 3 -name main.go`. Used `go build -o /tmp/kinder-dev-smoke .` instead. All three smoke outputs captured.

---

**Total deviations:** 2 Rule 1 (bug) auto-fixes + 2 plan-body clarifications.
**Impact on plan:** No public-API changes. `internaldev.Options` shape preserved exactly. All 10 LOCKED flags from `<interfaces>` ship with their documented defaults. Plan 49 SC1 is delivered.

## Issues Encountered

- **`go test -race` for the full repo timed out at the default 120s in earlier exploration; switched to `go test ./...` (no -race) for the cross-package sanity sweep.** The `pkg/internal/dev/...` and `pkg/cmd/kind/dev/...` packages were both run with `-race -count=1` and pass cleanly (1.5s + 3.7s = 5.2s combined). Full-repo `go test ./... -count=1 -timeout=300s` completes in ~5s wall-clock with 45 packages passing and no failures.
- **Initial `--help` test failure** — see Deviation #2 above. Caught by the GREEN gate as designed.

## TDD Gate Compliance

For Task 1, both RED (test commit before any implementation) and GREEN (feat commit after RED) gates are present in `git log` in correct order:

- Task 1: `d71d4956` (test) → `28ba8448` (feat) ✓

Task 2 is a CLI-registration one-liner with no new test file (the dev-command test suite from Task 1 fully covers the cobra surface; root.go is verified by the build + smoke). No `test()` commit precedes the `feat()` for Task 2 — this matches the Plan 48-05 precedent for the analogous root.go-registration step in that phase.

## Verification Gate Results

All plan-level verification gates pass:

| Gate | Command | Result |
|------|---------|--------|
| Build | `go build ./...` | PASS (clean) |
| Vet | `go vet ./pkg/cmd/kind/dev/...` | PASS (no warnings) |
| Race-tests (dev cmd pkg) | `go test -race ./pkg/cmd/kind/dev/... -count=1 -timeout=30s` | PASS (1.5s, 17 tests) |
| Race-tests (dev internal) | `go test -race ./pkg/internal/dev/... -count=1` | PASS (3.7s, 72 tests — unchanged from 49-03) |
| Full repo | `go test ./... -count=1 -timeout=300s` | PASS (45 packages, 0 failures) |
| Plan automated verify | `go run . dev --help \| grep -E '\-\-watch\|\-\-target\|\-\-poll\|\-\-debounce' \| wc -l` | PASS (7, threshold 4) |
| dev.NewCommand registration | `grep -n 'dev.NewCommand' pkg/cmd/kind/root.go` | 1 hit (line 85) |
| DurationVar flags | `grep -nE 'DurationVar.*(debounce\|poll-interval\|rollout-timeout)' pkg/cmd/kind/dev/dev.go` | 3 hits (lines 117, 121, 123) |
| MarkFlagRequired | `grep -nE 'MarkFlagRequired.*(watch\|target)' pkg/cmd/kind/dev/dev.go` | 2 hits (lines 128, 129) |
| go.mod diff vs 49-03 | `git diff 42bf901b..HEAD -- go.mod go.sum \| wc -l` | 0 ✓ |

## User Setup Required

None — Plan 04 is pure CLI plumbing on top of Plan 49-03's Run. Users who want to actually USE `kinder dev` need:

1. A running kinder cluster (e.g., `kinder create cluster`).
2. A target Deployment in that cluster referencing `kinder-dev/<target>:latest` with `imagePullPolicy: Never`.
3. `kubectl` and `docker` (or active provider binary) on PATH.

These prerequisites are documented in the `kinder dev --help` Long text and in `kinder doctor`'s checks (Phase 47).

## Next Phase Readiness

- **Phase 49 is now feature-complete from the user's perspective.** All 4 plans (01 watch primitives, 02 cycle-step primitives, 03 cycle runner + Run orchestrator, 04 CLI shell) are landed. SC1 (`kinder dev --watch <dir> --target <deployment>` enters watch mode) is reachable from the CLI; SC2 (per-cycle %.1fs timing) lives in 49-03's runOneCycle; SC3 (debounce burst-collapse) lives in 49-01 + 49-03's Debounce; SC4 (--poll dispatch) is wired through Plan 04's flag → Plan 03's Run → Plan 01's StartPoller.
- **Phase 49 verifier gate is the next step.** Plan 04's success criteria are satisfied; the verifier should run `go build ./...`, `go vet ./...`, `go test ./...`, and the manual smoke from the plan's `<verification>` block. All four are documented as PASS above.
- **No blockers** for the verifier or for follow-on phases.

## Threat Flags

None. Plan 04 introduces no new network endpoints, no new auth paths, no new file-write surface beyond what Plan 49-03's Run already does (the kubeconfig tempfile, V4-mitigated). The `--watch` flag accepts a user-supplied directory path that is `os.Stat`'d for existence + IsDir, then handed to fsnotify (Plan 49-01) — fsnotify itself enforces the watch-set boundary (you can't watch outside the directory you registered). No path-traversal vulnerability surfaces here. The `--image` flag is a Docker tag passed through to `docker build` (Plan 49-02's BuildImage with V5 mitigation already in place: argv-array exec, no shell).

## Self-Check

**Files verified present:**

- `pkg/cmd/kind/dev/dev.go` — FOUND
- `pkg/cmd/kind/dev/dev_test.go` — FOUND
- `pkg/cmd/kind/root.go` (modified, includes `dev.NewCommand`) — FOUND
- `.planning/phases/49-inner-loop-hot-reload-kinder-dev/49-04-SUMMARY.md` — FOUND (this file)

**Commits verified in `git log`:**

- `d71d4956` test(49-04): add failing tests for kinder dev cobra command — FOUND
- `28ba8448` feat(49-04): implement kinder dev cobra command (Task 1) — FOUND
- `d9c4378c` feat(49-04): register kinder dev in root command (Task 2) — FOUND

## Self-Check: PASSED

---

*Phase: 49-inner-loop-hot-reload-kinder-dev*
*Completed: 2026-05-06*
