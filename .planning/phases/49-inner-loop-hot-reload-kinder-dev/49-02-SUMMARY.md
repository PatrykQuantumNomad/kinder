---
phase: 49-inner-loop-hot-reload-kinder-dev
plan: 02
subsystem: developer-tools
tags: [kinder-dev, hot-reload, docker-build, kubectl-rollout, kubeconfig, image-load, tdd]

# Dependency graph
requires:
  - phase: 47-cluster-pause-resume
    provides: ProviderBinaryName / lifecycle test infra (Cmder + fakeNode + fakeCmd) reused by 49-02 tests
  - phase: 46-load-images
    provides: nodeutils.ImageTags / nodeutils.ReTagImage / nodeutils.LoadImageArchiveWithFallback — public APIs replicated against (RESEARCH §1 Option A)
provides:
  - BuildImage(ctx, binaryName, imageTag, contextDir) — host docker build wrapper with V5 mitigation
  - LoadImagesIntoCluster(ctx, opts LoadOptions) — single-image load pipeline replicating kinder load images core
  - RolloutRestartAndWait(ctx, kubeconfigPath, namespace, deployment, timeout) — host kubectl rollout
  - WriteKubeconfigTemp(provider, clusterName) — 0600-mode kubeconfig tempfile (V4 mitigation)
  - BuildImageFn / RolloutFn package-level test-injection points (consumed by Plan 03 cycle runner)
  - LoadOptions struct + ImageLoaderFn signature (locked API for Plan 03)
affects: [49-03, 49-04, kinder-dev-cycle-runner, kinder-dev-CLI]

# Tech tracking
tech-stack:
  added: []  # Zero new module deps — coordinated with 49-01 (which owns the fsnotify add)
  patterns:
    - "Package-level exec.Cmder indirection (devCmder) for unit-testing shell-outs without spinning real subprocesses — mirrors pkg/internal/lifecycle's defaultCmder pattern"
    - "Function-var indirection for hard-to-fake APIs (kubeconfigGetter, nodeLister, imageTagsFn, reTagFn, imageInspectID) so tests can avoid building fake *cluster.Provider"
    - "BuildImageFn / RolloutFn package-level vars match the Phase 47/48 cycle-runner injection convention"
    - "TDD RED→GREEN per task with separate test() and feat() commits — matches phase 47/48 discipline"

key-files:
  created:
    - pkg/internal/dev/build.go
    - pkg/internal/dev/build_test.go
    - pkg/internal/dev/load.go
    - pkg/internal/dev/load_test.go
    - pkg/internal/dev/rollout.go
    - pkg/internal/dev/rollout_test.go
    - pkg/internal/dev/kubeconfig.go
    - pkg/internal/dev/kubeconfig_test.go
  modified: []  # Plan 49-01 owns doc.go and the go.mod fsnotify add

key-decisions:
  - "Replicated kinder load images core pipeline via public APIs (nodeutils.ImageTags / ReTagImage / LoadImageArchiveWithFallback + errors.UntilErrorConcurrent) rather than calling the unexported runE in pkg/cmd/kind/load/images/images.go — RESEARCH §1 Option A. The runE takes an unexported flagpole and writes structured output to streams.Out, so reusing it would require either widening its export surface (scope creep) or a fragile reflection hack."
  - "Single-image LoadOptions API rather than multi-image (upstream runE accepts []string). kinder dev only ever loads ONE image per cycle — multi-image surface adds complexity for no gain in this codepath."
  - "Used host kubectl + external kubeconfig (provider.KubeConfig(name, false)) rather than node.CommandContext (the in-cluster system-addon pattern). RESEARCH §3: kinder dev targets a user-managed Deployment, not a system addon, so the rollout must run on the host where the user's docker / kubectl already live."
  - "Function-var indirection for nodeLister / kubeconfigGetter rather than threading interface arguments through every call. Tests can swap behavior without building a fake *cluster.Provider (which would require a real internal provider). Matches the lifecycle/state.go pattern."
  - "Used os.CreateTemp (not os.WriteFile) for kubeconfig tempfile so concurrent kinder dev invocations against different clusters do not clobber each other. os.Chmod to 0600 is BEFORE write to harden against unusual umask configurations (V4 mitigation)."
  - "imageInspectID inlined a stripped-down OutputLines pipeline rather than calling exec.OutputLines directly because exec.OutputLines closes over a buffer that races with cmd.Run() in some test fakes — the explicit io.Pipe + goroutine in this codepath is race-clean under -race."

patterns-established:
  - "Pattern: Package-level exec.Cmder var (devCmder) — production = exec.DefaultCmder, tests swap via withDevCmder. Future kinder primitives can reuse the same fakeExecCmder/fakeExecCmd test infra in build_test.go."
  - "Pattern: When a Provider method is hard to fake, wrap it in a package-level func var (kubeconfigGetter / nodeLister) and accept the Provider on the public Options struct as the production source. Tests swap the var, not the Provider."

# Metrics
duration: ~21min
completed: 2026-05-06
---

# Phase 49 Plan 02: kinder dev Cycle-Step Primitives Summary

**Four cycle-step primitives (BuildImage, LoadImagesIntoCluster, RolloutRestartAndWait, WriteKubeconfigTemp) that DEV-02 / DEV-03 require — built test-first against public APIs with zero new module deps.**

## Performance

- **Duration:** ~21 min
- **Started:** 2026-05-06T17:53Z
- **Completed:** 2026-05-06T18:14Z
- **Tasks:** 3 (each split into a test() RED commit + feat() GREEN commit)
- **Files created:** 8 (4 production + 4 test files; ~1,661 lines total)

## Accomplishments

- **BuildImage** — shells out to `<binary> build -t <tag> <ctx>` via `pkg/exec`, with non-empty validation and V5 shell-injection mitigation (arguments are individual argv elements, no shell layer).
- **WriteKubeconfigTemp** — produces a 0600-mode kinder-dev-*.kubeconfig tempfile from `provider.KubeConfig(name, false)` (external endpoint per RESEARCH A4); cleanup function is idempotent and is nil on error so callers cannot accidentally double-defer.
- **LoadImagesIntoCluster** — replicates the kinder load images core pipeline (image-ID inspect → ListInternalNodes → smart-skip-by-ID → re-tag-or-reimport → save+UntilErrorConcurrent) against PUBLIC APIs. Zero coupling to `pkg/cmd/kind/load`.
- **RolloutRestartAndWait** — runs host kubectl `rollout restart` then `rollout status --timeout=<dur>` with proper `--kubeconfig=`, `--namespace=`, and `deployment/<name>` argv shape. Restart failure short-circuits before status (verified by test).
- **LoadOptions struct + ImageLoaderFn signature locked** — Plan 03's cycle runner can build a fresh options per cycle and inject test doubles without touching primitive internals.
- **BuildImageFn + RolloutFn exported as package-level vars** — Plan 03's cycle runner test injection ready.

## Task Commits

Each task followed RED → GREEN with separate commits per the project TDD discipline:

1. **Task 1 RED — failing tests for BuildImage + WriteKubeconfigTemp:** `ead49464` (test)
2. **Task 1 GREEN — implement BuildImage + WriteKubeconfigTemp:** `a0feea00` (feat)
3. **Task 2 RED — failing tests for LoadImagesIntoCluster:** `32368662` (test)
4. **Task 2 GREEN — implement LoadImagesIntoCluster:** `78283ea0` (feat)
5. **Task 3 RED — failing tests for RolloutRestartAndWait:** `96833271` (test)
6. **Task 3 GREEN — implement RolloutRestartAndWait:** `c7d3e6a2` (feat)

## Files Created/Modified

**Created (production):**
- `pkg/internal/dev/build.go` (69 LOC) — `BuildImage` + `BuildImageFn` + package-level `devCmder`.
- `pkg/internal/dev/load.go` (241 LOC) — `LoadImagesIntoCluster`, `LoadOptions`, `ImageLoaderFn`, plus the `nodeLister` / `imageTagsFn` / `reTagFn` / `imageInspectID` indirection vars.
- `pkg/internal/dev/rollout.go` (91 LOC) — `RolloutRestartAndWait` + `RolloutFn`.
- `pkg/internal/dev/kubeconfig.go` (89 LOC) — `WriteKubeconfigTemp` + `kubeconfigGetter` indirection.

**Created (tests, all `-race` clean):**
- `pkg/internal/dev/build_test.go` (253 LOC) — 7 tests: argv shape, error propagation, three validation guards, V5 mitigation (shell metachar isolation), BuildImageFn wiring. Includes the shared `fakeExecCmder`/`fakeExecCmd` infra used across all four test files.
- `pkg/internal/dev/load_test.go` (489 LOC) — 12 tests: three validation guards, smart-skip, selective load, re-tag-by-ID, re-tag fallback, image-inspect failure, list-nodes failure, loader error propagation, tags-error fallback, no-nodes path.
- `pkg/internal/dev/rollout_test.go` (265 LOC) — 9 tests: happy path with full argv assertion, restart failure (status MUST NOT run), status failure (timeout in error), four validation guards, timeout-as-string canonical form (90s → 1m30s), RolloutFn wiring.
- `pkg/internal/dev/kubeconfig_test.go` (164 LOC) — 4 tests: mode 0600 + content + naming convention, cleanup-removes, cleanup idempotence, provider-error propagation with no temp-file leak.

**Not modified (plan-coordinated with 49-01):**
- `pkg/internal/dev/doc.go` — owned by 49-01.
- `go.mod` / `go.sum` — fsnotify add owned by 49-01; 49-02 added zero new module deps.

## Test Coverage

- `pkg/internal/dev/...` total: 32 tests in 49-02's four `_test.go` files (7 + 12 + 9 + 4), all pass under `-race -count=1`.
- Combined with 49-01's 18 tests (StartWatcher, StartPoller, Debounce), the package totals 50 tests, all green.
- Full project test gate `go test ./... -count=1`: 44 packages pass, no failures.

## Decisions Made

See frontmatter `key-decisions`. Highlights:

- **Public-API replication, not unexported runE reuse.** `runE` in `pkg/cmd/kind/load/images/images.go` takes an unexported `flagpole` and writes structured output to `streams.Out`. Reusing it would require widening its export surface (scope creep). Re-implementing the ~30-line pipeline against public APIs (`nodeutils.ImageTags`, `ReTagImage`, `LoadImageArchiveWithFallback`, `errors.UntilErrorConcurrent`) is RESEARCH §1 Option A.
- **Function-var indirection over interface threading.** `nodeLister`, `kubeconfigGetter`, `imageTagsFn`, `reTagFn`, `imageInspectID` are all package-level vars. Production paths default to the real implementation; tests swap via `t.Cleanup`. Threading interfaces through every signature would have widened the API for no test-injection gain.
- **Single-image API.** `kinder dev` only ever loads ONE image per cycle (the freshly-built one). The upstream `runE` multi-image surface (`[]string`) adds complexity for no gain.
- **Host kubectl, not in-node kubectl.** RESEARCH §3: rollout uses `pkg/exec.CommandContext` against host kubectl with `--kubeconfig=<external>`, NOT `node.CommandContext` (which is the in-cluster pattern for system addons during cluster create). User Deployments are user-managed, so the rollout runs on the host where the user's existing kubectl context lives.
- **0600 chmod BEFORE write.** Even though `os.CreateTemp` creates 0600 on Unix, we explicitly Chmod first as a V4 defensive measure for unusual umask configurations. The kubeconfig contains client cert+key — these ARE credentials.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking / parallel-execution race] Worked around mid-flight 49-01 RED state**

- **Found during:** Task 1 GREEN gate verification.
- **Issue:** 49-01 (running in parallel on the same `main` branch per phase strategy) had committed `test(49-01): add failing tests for StartPoller` *before* the corresponding `feat(49-01): implement stdlib StartPoller fallback`, leaving `pkg/internal/dev/poll_test.go` referencing an undefined `StartPoller`. The Go test binary compiles all `_test.go` files in a package together, so my Task 1 GREEN gate could not pass until 49-01 advanced to GREEN.
- **Fix:** Held position briefly. 49-01 advanced (commit `1f94b4d3 feat(49-01): implement stdlib StartPoller fallback`) within the same minute. Re-ran my GREEN gate → pass. No code changes on my side; pure timing coordination.
- **Verification:** `go test -race ./pkg/internal/dev/... -count=1` — all 50 tests (49-01's 18 + 49-02's 32) pass after 49-01 reached GREEN.
- **Note:** This is the documented hazard of two parallel TDD plans on the same `main` branch; the plan's concurrency note explicitly tolerates it. No action needed in future runs.

**2. [Rule 1 — Bug / test-fake interaction] Switched imageInspectID to explicit io.Pipe rather than exec.OutputLines**

- **Found during:** Task 2 GREEN gate (initial implementation).
- **Issue:** The plan suggested `exec.OutputLines(devCmder.CommandContext(...))` for `imageInspectID`. `exec.OutputLines` writes Cmd.Stdout into a closure-shared `bytes.Buffer` and reads it back after `Run()`. With my `fakeExecCmd.SetStdout(w)` recording the writer pointer for later `c.stdoutW.Write(...)` inside `Run()`, the buffer-vs-call ordering is fine — but `-race` flagged a writer-vs-reader concurrency edge in some test fakes that script per-call stdout.
- **Fix:** Inlined a small io.Pipe + goroutine + manual line splitter inside `imageInspectID`. The pipe writer is set on the cmd; the reader runs concurrently and is joined via a buffered channel after `Run()` returns and the pipe writer is closed. `-race` clean across all 32 tests.
- **Files modified:** `pkg/internal/dev/load.go` (the `imageInspectID` definition).
- **Verification:** `go test -race ./pkg/internal/dev/... -count=1 -timeout=60s` passes. Production behavior is identical (single-line stdout from `<binary> image inspect -f {{ .Id }} <ref>`).
- **Committed in:** `78283ea0` (Task 2 GREEN commit).

---

**Total deviations:** 2 (1 Rule 3 timing-only no-code, 1 Rule 1 minor implementation choice).
**Impact on plan:** None on the public surface. LoadOptions, ImageLoaderFn, BuildImageFn, RolloutFn are exactly as specified in the plan's `<interfaces>` block.

## Issues Encountered

- **Mid-execution file disappearance.** During Task 1, after writing `build_test.go` and `kubeconfig_test.go` (which compiled cleanly in isolation), a subsequent `ls` showed they had vanished — apparently a side-effect of the parallel plan environment. Re-wrote both files (identical content) and proceeded normally; the resulting RED gate then failed correctly with `undefined: BuildImage` etc. The lesson: stage and commit RED test files promptly to avoid losing them to filesystem races. Subsequent tasks committed RED immediately.

## User Setup Required

None — primitives are pure library code and have no external service / dashboard / auth setup. Plan 04 (CLI command) will surface user-facing flags.

## Next Phase Readiness

- **Plan 49-03 (cycle-runner) unblocked.** All four primitives implemented with the locked API surface. Plan 03 can build per-cycle `LoadOptions{...}`, call `dev.BuildImageFn(...)`, `dev.LoadImagesIntoCluster(...)`, `dev.RolloutFn(...)`, and reuse `dev.WriteKubeconfigTemp(...)` once at startup.
- **Plan 49-04 (CLI) unblocked from the data-flow side.** Once 49-03 produces the `Run` orchestrator, 49-04 only needs to flag-parse and call into it.
- **No blockers.**

## Threat Flags

None — Plan 49-02's surface is exactly the surface 49-RESEARCH already threat-modeled (V4 kubeconfig perms, V5 shell-arg isolation), both mitigated. No new network endpoints, auth paths, or schema changes introduced.

## Self-Check: PASSED

**Files verified present:**
- pkg/internal/dev/build.go
- pkg/internal/dev/build_test.go
- pkg/internal/dev/load.go
- pkg/internal/dev/load_test.go
- pkg/internal/dev/rollout.go
- pkg/internal/dev/rollout_test.go
- pkg/internal/dev/kubeconfig.go
- pkg/internal/dev/kubeconfig_test.go
- .planning/phases/49-inner-loop-hot-reload-kinder-dev/49-02-SUMMARY.md (this file)

**Commits verified in git log --all:**
- ead49464 (Task 1 RED)
- a0feea00 (Task 1 GREEN)
- 32368662 (Task 2 RED)
- 78283ea0 (Task 2 GREEN)
- 96833271 (Task 3 RED)
- c7d3e6a2 (Task 3 GREEN)

**Verification gates:**
- `go build ./...` PASS
- `go vet ./pkg/internal/dev/...` PASS
- `go test -race ./pkg/internal/dev/... -count=1 -timeout=60s` PASS (50 tests)
- `go test ./... -count=1 -timeout=120s` PASS (44 packages, no failures)

---
*Phase: 49-inner-loop-hot-reload-kinder-dev*
*Completed: 2026-05-06*
