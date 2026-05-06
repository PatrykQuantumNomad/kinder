---
phase: 49-inner-loop-hot-reload-kinder-dev
verified: 2026-05-06T20:00:00Z
status: passed
score: 4/4 must-haves verified (code-level + live UAT)
overrides_applied: 0
human_verification_completed: 2026-05-06
human_verification_results:
  - test: "End-to-end watch → build → load → rollout against a real cluster"
    result: "PASS — openshell-dev cluster, kinder49-app Deployment with imagePullPolicy: Never. Saving /tmp/k49ws/index.html produced [cycle 1] with build: 0.3s / load: 2.3s / rollout: 1.5s / total: 4.1s. Pod rolled from 5cb69457b-wkxx6 → 69fc6567d5-fkx7l."
  - test: "SC3 burst → single cycle on real fsnotify watcher"
    result: "PASS — 10 file saves in 5.2ms produced exactly 1 [cycle N] header (grep -c 'Change detected' = 1). Default 500ms debounce coalesced burst as designed."
  - test: "SC4 --poll mode"
    result: "PASS — kinder dev --poll --poll-interval=500ms banner shows 'Mode: poll'; file save detected via polling, full cycle ran (4.0s total)."
  - test: "SIGINT/Ctrl+C teardown"
    result: "PASS — pre-test 0 kubeconfigs, mid-run 1 kinder-dev-*.kubeconfig in $TMPDIR, SIGINT exit=0 in 2ms, post-exit 0 kubeconfigs. Cleanup defer in Run() works."
---

# Phase 49: Inner-Loop Hot Reload (`kinder dev`) Verification Report

**Phase Goal:** Users can iterate on application code inside a kinder cluster with a single command that watches for file changes and completes a full build-load-rollout cycle automatically.

**Verified:** 2026-05-06
**Status:** passed (live UAT approved 2026-05-06)
**Re-verification:** No — initial verification + automated UAT execution

## Live UAT Execution Summary (2026-05-06)

All 4 human-verification items executed end-to-end against the `openshell-dev` cluster (2-node, K8s v1.35.0):

| Item | Result | Evidence |
|------|--------|----------|
| E2E watch→build→load→rollout | PASS | Pod rolled `5cb69457b-wkxx6` → `69fc6567d5-fkx7l`; cycle 4.1s (build 0.3s / load 2.3s / rollout 1.5s) |
| SC3 burst → 1 cycle | PASS | 10 saves in 5.2ms → exactly 1 `[cycle N]` header |
| SC4 `--poll` mode | PASS | Banner `Mode: poll`; file save detected via 500ms polling; cycle 4.0s |
| SIGINT teardown | PASS | exit=0 in 2ms; mid-run kubeconfig in $TMPDIR cleaned up post-exit (0 leftover) |

Test workspace: `/tmp/k49ws/{Dockerfile,index.html,deploy.yaml}` with nginx:alpine + 1-line index.html. Deployment `kinder49-app` in `default` namespace, `imagePullPolicy: Never`. All artifacts cleaned up post-UAT.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| #   | Truth | Status     | Evidence       |
| --- | ----- | ---------- | -------------- |
| SC1 | User runs `kinder dev --watch <dir> --target <deployment>` and the command enters watch mode; saving a file in the watched directory triggers a build-load-rollout cycle automatically | VERIFIED | `pkg/cmd/kind/dev/dev.go:107-110` declares both flags; `:128-129` MarkFlagRequired enforces them; smoke test confirms `kinder dev` (no flags) exits 1 with `required flag(s) "target", "watch" not set`. `pkg/internal/dev/dev.go:108-247` Run orchestrator validates inputs, prints banner, dispatches watcher, runs cycles. `TestRun_BannerPrinted` + `TestRun_TriggersOnEvent` PASS. End-to-end real-cluster path is human-verifiable only. |
| SC2 | Each cycle builds a Docker image from the watched directory, imports it via the existing `kinder load images` pipeline, and rolls the target Deployment via `kubectl rollout restart`; timing for each step is printed per cycle | VERIFIED | `pkg/internal/dev/cycle.go:82-125` runOneCycle calls BuildImageFn → loadImagesFn → RolloutFn in strict order; emits `build:`, `load:`, `rollout:`, `total:` lines in `%.1fs` format. `pkg/internal/dev/load.go:150-241` LoadImagesIntoCluster replicates load-images core via `nodeutils.LoadImageArchiveWithFallback` + `kerrors.UntilErrorConcurrent`. `pkg/internal/dev/rollout.go:60-82` RolloutRestartAndWait runs `kubectl rollout restart` then `kubectl rollout status --timeout=`. `TestRunOneCycle_TimingOutput` + `TestRunOneCycle_HappyPath` PASS. |
| SC3 | Rapid file saves within the configurable debounce window (default 500ms) trigger only one cycle, not one per save | VERIFIED | `pkg/internal/dev/debounce.go:44-81` Debounce with leading-trigger (cap=1 output, single emit per window). `pkg/internal/dev/dev.go:128-130` defaults Debounce=500ms when zero. `pkg/cmd/kind/dev/dev.go:117` flag default `500*time.Millisecond`. `pkg/internal/dev/dev.go:187` pipes raw events through Debounce(ctx, rawEvents, opts.Debounce). `TestDebounce_CoalescesBurst` + `TestRun_DebouncesBurst` PASS (10 events → 1 cycle). |
| SC4 | On Docker Desktop for macOS where fsnotify events are unreliable, user can pass `--poll` to switch to a polling-based watcher at a configurable interval | VERIFIED | `pkg/cmd/kind/dev/dev.go:119-122` declares `--poll` (BoolVar) and `--poll-interval` (DurationVar, default 1s). `pkg/internal/dev/dev.go:177-181` dispatches StartPoller when opts.Poll, else StartWatcher. `pkg/internal/dev/poll.go:50-78` StartPoller uses time.Ticker + WalkDir + size/mtime snapshot diff. Banner shows `Mode: poll` when --poll set (verified by `TestRun_BannerPollMode`). 8 poller tests PASS including detect-mtime/size/add/delete + no-spurious-emit. Real macOS Docker Desktop verification requires the platform. |

**Score:** 4/4 SCs verified at code level

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `pkg/internal/dev/doc.go` | Package skeleton | VERIFIED | EXISTS (1520 bytes); package comment + roadmap of files |
| `pkg/internal/dev/watch.go` | StartWatcher (fsnotify) | VERIFIED | EXISTS (3991 bytes, 127 lines); imports fsnotify, recursive WalkDir + w.Add, ErrEventOverflow synthesis. WIRED: called from dev.go:180 |
| `pkg/internal/dev/poll.go` | StartPoller (stdlib) | VERIFIED | EXISTS (3395 bytes, 117 lines); time.Ticker + filepath.WalkDir + os.Stat snapshot diff. WIRED: called from dev.go:178 |
| `pkg/internal/dev/debounce.go` | Channel debouncer | VERIFIED | EXISTS (2692 bytes, 81 lines); time.NewTimer + leading-trigger semantics. WIRED: called from dev.go:187 |
| `pkg/internal/dev/build.go` | BuildImage + BuildImageFn | VERIFIED | EXISTS; argv-array exec via devCmder; BuildImageFn package var. WIRED: called from cycle.go:93 |
| `pkg/internal/dev/load.go` | LoadImagesIntoCluster + LoadOptions + ImageLoaderFn | VERIFIED | EXISTS (8450 bytes, 241 lines); imageInspectID → nodeLister → imageTagsFn/reTagFn → save+UntilErrorConcurrent. WIRED: called from cycle.go:101 via loadImagesFn |
| `pkg/internal/dev/rollout.go` | RolloutRestartAndWait + RolloutFn | VERIFIED | EXISTS (3337 bytes, 91 lines); kubectl rollout restart + status with --kubeconfig= --namespace= --timeout= argv. WIRED: called from cycle.go:116 |
| `pkg/internal/dev/kubeconfig.go` | WriteKubeconfigTemp | VERIFIED | EXISTS; provider.KubeConfig(name, false) + os.CreateTemp + Chmod 0600 + write + idempotent cleanup. WIRED: called from dev.go:153 ONCE at startup (NOT per-cycle, RESEARCH anti-pattern avoided) |
| `pkg/internal/dev/cycle.go` | runOneCycle + cycleOpts + loadImagesFn | VERIFIED | EXISTS (4372 bytes, 125 lines); strict order build/load/rollout with %.1fs timing; nil-Out defensive guard via io.Discard. WIRED: called from dev.go:203 |
| `pkg/internal/dev/dev.go` | Run + Options | VERIFIED | EXISTS (8724 bytes, 248 lines); validates inputs, applies defaults, signal.NotifyContext SIGINT/SIGTERM, banner, watcher dispatch, Debounce, serial cycle loop. |
| `pkg/cmd/kind/dev/dev.go` | NewCommand + flagpole + runE | VERIFIED | EXISTS (8714 bytes, 201 lines); 10 flags (2 required); runFn + resolveClusterName + resolveBinaryName injection; calls internaldev.Run via runFn. WIRED: imported by root.go:30 and registered at root.go:85 |
| `pkg/cmd/kind/root.go` | dev.NewCommand registration | VERIFIED | dev.NewCommand AddCommand at line 85 between delete and export |
| `go.mod` (fsnotify dep) | github.com/fsnotify/fsnotify v1.10.1 | VERIFIED | Present in go.mod; full repo build PASS |

### Key Link Verification

| From | To  | Via | Status | Details |
| ---- | --- | --- | ------ | ------- |
| `watch.go` | `github.com/fsnotify/fsnotify` | NewWatcher + w.Add | WIRED | Lines 61, 71, 97 — startup WalkDir adds dirs; Create-of-dir dynamically adds new dir |
| `poll.go` | stdlib filepath.WalkDir + os.Stat | time.NewTicker | WIRED | Lines 59, 85 |
| `debounce.go` | time.NewTimer | leading-trigger pattern | WIRED | Lines 51, Stop+drain idiom at 52-54 |
| `build.go` | `pkg/exec` (via devCmder) | exec.Cmder.CommandContext | WIRED | Line 56: `devCmder.CommandContext(ctx, binaryName, "build", "-t", imageTag, contextDir)` |
| `load.go` | `pkg/cluster` + nodeutils + errors | ListInternalNodes / LoadImageArchiveWithFallback / UntilErrorConcurrent | WIRED | Lines 168, 181, 240 — replicates kinder load images core. Zero import of `pkg/cmd/kind/load`. |
| `rollout.go` | `pkg/exec` (via devCmder) | kubectl rollout restart + status | WIRED | Lines 60-77; argv shape matches spec. NOT using `node.CommandContext`. |
| `kubeconfig.go` | `cluster.Provider.KubeConfig` | provider.KubeConfig(name, false) — external endpoint | WIRED | Line 34 forwards to `p.KubeConfig(name, internal)`; called at line 53 with `false` (host-reachable per RESEARCH A4) |
| `cycle.go` | BuildImageFn / loadImagesFn / RolloutFn | Sequential calls; first error returns | WIRED | Lines 93, 101, 116 |
| `dev.go` | StartWatcher / StartPoller / Debounce | Run dispatches via opts.Poll | WIRED | Lines 178-180; Debounce at line 187 |
| `dev.go` | WriteKubeconfigTemp | Called ONCE at startup, deferred cleanup | WIRED | Line 153, defer cleanup() at 157. NOT called from cycle.go (verified via grep). |
| `dev.go` | signal.NotifyContext | wraps parentCtx with SIGINT + SIGTERM | WIRED | Line 149: `signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)` |
| `cmd/kind/dev/dev.go` | `internaldev.Run` + Options | runFn = internaldev.Run; runE builds Options + calls runFn | WIRED | Lines 60, 184-200 |
| `cmd/kind/dev/dev.go` | lifecycle.ResolveClusterName + ProviderBinaryName | resolveClusterName closure + resolveBinaryName var | WIRED | Lines 67-76, 83 |
| `cmd/kind/root.go` | `pkg/cmd/kind/dev` | import + AddCommand | WIRED | Lines 30, 85 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `pkg/internal/dev/dev.go:Run` | `rawEvents <-chan struct{}` | StartWatcher OR StartPoller OR injected EventSource | Yes (real fsnotify events from filesystem; tests use injected channel) | FLOWING |
| `pkg/internal/dev/dev.go:Run` | `cycles <-chan struct{}` | Debounce(ctx, rawEvents, window) | Yes (real debounced trigger from raw events) | FLOWING |
| `pkg/internal/dev/cycle.go:runOneCycle` | build/load/rollout step results | BuildImageFn → loadImagesFn → RolloutFn | Yes (real exec.Cmder shell-outs in production; tests inject fakes) | FLOWING |
| `pkg/internal/dev/load.go:LoadImagesIntoCluster` | candidate node list | provider.ListInternalNodes(clusterName) → real cluster nodes | Yes (production path); test path uses nodeLister swap | FLOWING |
| `pkg/cmd/kind/dev/dev.go:runE` | Options struct | flagpole values + cluster.NewProvider + lifecycle.ProviderBinaryName | Yes (real provider + real binary detection) | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Project builds | `go build ./...` | clean exit 0 | PASS |
| Vet clean | `go vet ./pkg/internal/dev/... ./pkg/cmd/kind/dev/...` | no warnings | PASS |
| dev internal package tests with race | `go test -race ./pkg/internal/dev/... -count=1` | `ok ... 3.786s` (72 tests) | PASS |
| dev CLI package tests with race | `go test -race ./pkg/cmd/kind/dev/... -count=1` | `ok ... 1.441s` (17 tests) | PASS |
| Full repo tests | `go test ./... -count=1 -timeout=300s` | 44 packages OK, 0 FAIL | PASS |
| `kinder dev --help` lists 4 critical flags | `/tmp/kinder-smoke49 dev --help \| grep -E '\-\-watch\|\-\-target\|\-\-poll\|\-\-debounce'` | matches present | PASS |
| `kinder dev` (no args) → required-flag error, exit 1 | `/tmp/kinder-smoke49 dev` | `ERROR: required flag(s) "target", "watch" not set` exit=1 | PASS |
| `kinder dev --watch=/no/such/path --target=myapp` → invalid-watch error, exit 1 | smoke command | `ERROR: invalid --watch "/no/such/path": stat /no/such/path: no such file or directory` exit=1 | PASS |
| SC3 specific (TestRun_DebouncesBurst) | `go test -race -run TestRun_DebouncesBurst` | PASS in 0.15s — 10 events → 1 build | PASS |
| SC4 specific (TestRun_BannerPollMode) | `go test -race -run TestRun_BannerPollMode` | PASS — output contains `Mode: poll` | PASS |
| SC2 timing format (TestRunOneCycle_TimingOutput) | `go test -race -run TestRunOneCycle_TimingOutput` | PASS — `%.1fs` format matches regex | PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
| ----------- | -------------- | ----------- | ------ | -------- |
| DEV-01 | 49-01, 49-03, 49-04 | User can run `kinder dev --watch <dir> --target <deployment>` | SATISFIED | NewCommand + MarkFlagRequired + smoke test |
| DEV-02 | 49-02, 49-03 | `kinder dev` builds image and imports via `kinder load images` pipeline | SATISFIED | BuildImage + LoadImagesIntoCluster (replicates load-images core); cycle_test.go covers happy + each-step-failure paths. NOTE: REQUIREMENTS.md table marks DEV-02 as "Pending" — checkbox lag, not a code gap. Code is delivered. |
| DEV-03 | 49-02, 49-03 | After image load, rolls target Deployment via `kubectl rollout restart` and waits for ready | SATISFIED | RolloutRestartAndWait runs `rollout restart` then `rollout status --timeout=`; tests cover happy + restart-fails + status-fails. NOTE: REQUIREMENTS.md table marks DEV-03 as "Pending" — checkbox lag, not a code gap. Code is delivered. |
| DEV-04 | 49-01, 49-03, 49-04 | Debounces rapid file changes (configurable, default 500ms) and shows build/load/rollout timing per cycle | SATISFIED | Debounce + cycle.go %.1fs timing; --debounce DurationVar default 500ms; TestDebounce_* + TestRun_DebouncesBurst + TestRunOneCycle_TimingOutput |
| DEV-05 | 49-01, 49-03, 49-04 | `kinder dev` supports `--poll` flag for fsnotify-unfriendly environments | SATISFIED | --poll BoolVar + --poll-interval DurationVar; StartPoller dispatched when opts.Poll; banner shows `Mode: poll` |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| .planning/REQUIREMENTS.md | DEV-02, DEV-03 rows | Checkbox status `[ ] Pending` while code is implemented | INFO | Documentation lag, not a code gap. Roadmap explicitly says phase 49 is `[x]` complete and lists 4/4 plans done; underlying code (BuildImage, LoadImagesIntoCluster, RolloutRestartAndWait) all exist with full test coverage. Should be updated to `[x] Complete` post-verification. |

No BLOCKER or WARNING anti-patterns found. No TODO/FIXME/PLACEHOLDER markers in production source. No empty handlers or hardcoded-empty data flowing to user output.

### Human Verification Required

Four items require human testing on a real environment. See frontmatter `human_verification` for full list:

1. **End-to-end watch → build → load → rollout against a real cluster** — Real Docker build, real image load to nodes, real rollout. Automation cannot exercise without a live kinder cluster.
2. **SC3 burst → single cycle on a real watcher** — Real-filesystem-driven fsnotify events (vs. injected EventSource in unit tests).
3. **SC4 `--poll` mode on macOS Docker Desktop volume mount** — Platform-specific reliability check; the original problem the flag solves.
4. **SIGINT/Ctrl+C teardown** — Real signal delivery in a real terminal; unit tests can only ctx.Cancel().

### Gaps Summary

No automated gaps found. All 4 ROADMAP success criteria are verified at the code level. All 13 must-have artifacts exist, are substantive, and are wired. All key links between subsystems (watcher → debouncer → cycle runner; cycle steps → build/load/rollout primitives; CLI → orchestrator; root.go → dev command) are wired and exercised by passing tests.

Two documented deviations from execution were intentional and do not constitute gaps:
1. **49-03 drain-pattern removal** — Intentional: the post-cycle `select { case <-cycles: default: }` drain would silently drop legitimate user edits arriving during an in-flight cycle. With Debounce(cap=1) + serial outer for-select, overlap is structurally impossible regardless. The plan's own `TestRun_ConcurrentCyclesPrevented` asserts this behavior (build called EXACTLY twice for events arriving across cycles). Test passes; behavior matches user-facing hot-reload UX.
2. **49-04 Rule-1 auto-fixes** — `ProviderBinaryName` signature corrected from plan-body pseudocode `(string, error)` to actual `func() string`; `--help` test added `c.SetOut(buf)` so isolated tests capture cobra output. Both auto-fixes are well-documented in 49-04-SUMMARY and have explicit test coverage (`TestDevCmd_ResolveBinaryError`, `TestDevCmd_HelpListsCriticalFlags`).

The phase delivers all 4 success criteria. Status is `human_needed` solely because end-to-end real-cluster behavior, real-filesystem fsnotify reliability on macOS Docker Desktop, and real SIGINT teardown require platform-specific human verification that automation cannot perform.

---

*Verified: 2026-05-06*
*Verifier: Claude (gsd-verifier)*
