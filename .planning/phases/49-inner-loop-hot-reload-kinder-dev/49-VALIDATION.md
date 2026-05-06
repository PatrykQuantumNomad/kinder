---
phase: 49
phase_name: "Inner-Loop Hot Reload (`kinder dev`)"
nyquist_validation: enabled
source: "Extracted from 49-RESEARCH.md §Validation Architecture (lines 961–998)"
generated: 2026-05-06
---

# Phase 49 — Validation Plan

> Test map for `kinder dev`. All listed test files are Wave 0 (do not exist yet — Plans 49-01 through 49-04 create them as first-task RED tests).

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` |
| Config file | none (go test discovers files) |
| Quick run command | `go test ./pkg/cmd/kind/dev/... ./pkg/internal/dev/... -count=1` |
| Full suite command | `go test ./... -count=1` |

## Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | Owning Plan | File Exists? |
|--------|----------|-----------|-------------------|-------------|-------------|
| DEV-01 | `--watch` and `--target` flags required; missing either → non-zero exit | unit | `go test ./pkg/cmd/kind/dev/... -run TestDevCmd_MissingFlags` | 49-04 | ❌ Wave 0 |
| DEV-01 | Entering watch mode prints banner with dir + target + debounce window | unit | `go test ./pkg/internal/dev/... -run TestRun_Banner` | 49-03 | ❌ Wave 0 |
| DEV-02 | Build step calls `docker build` with correct image tag and watch dir | unit | `go test ./pkg/internal/dev/... -run TestRunOneCycle_BuildArgs` | 49-03 (asserts on Plan 02 BuildImageFn) | ❌ Wave 0 |
| DEV-02 | Load step invokes LoadImagesIntoCluster with correct provider + image | unit | `go test ./pkg/internal/dev/... -run TestRunOneCycle_LoadCalled` | 49-03 (asserts on Plan 02 ImageLoaderFn) | ❌ Wave 0 |
| DEV-03 | Rollout step calls `kubectl rollout restart` then `rollout status` | unit | `go test ./pkg/internal/dev/... -run TestRolloutRestartAndWait` | 49-02 | ❌ Wave 0 |
| DEV-03 | Rollout failure returns non-nil error and stops cycle | unit | `go test ./pkg/internal/dev/... -run TestRolloutRestartAndWait_Error` | 49-02 | ❌ Wave 0 |
| DEV-04 | N rapid events in < debounce window → exactly 1 cycle | unit | `go test ./pkg/internal/dev/... -run TestDebounce_CoalescesBurst` | 49-01 | ❌ Wave 0 |
| DEV-04 | Per-cycle timing lines (build / load / rollout / total) printed to streams.Out | unit | `go test ./pkg/internal/dev/... -run TestRunOneCycle_TimingOutput` | 49-03 | ❌ Wave 0 |
| DEV-05 | `--poll` flag accepted; pollLoop called instead of fsnotify | unit | `go test ./pkg/cmd/kind/dev/... -run TestDevCmd_PollFlag` | 49-04 | ❌ Wave 0 |
| DEV-05 | Poll loop detects a changed file (mtime change) and fires onChange | unit | `go test ./pkg/internal/dev/... -run TestPollLoop_DetectsChange` | 49-01 | ❌ Wave 0 |

## Sampling Rate

- **Per task commit:** `go test ./pkg/cmd/kind/dev/... ./pkg/internal/dev/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`.

## Wave 0 Gaps (files to be created during execution)

- [ ] `pkg/cmd/kind/dev/dev.go` — Cobra command (Plan 49-04)
- [ ] `pkg/cmd/kind/dev/dev_test.go` — command unit tests (Plan 49-04)
- [ ] `pkg/internal/dev/watch.go` + `_test.go` — fsnotify watcher (Plan 49-01)
- [ ] `pkg/internal/dev/poll.go` + `_test.go` — stdlib polling fallback (Plan 49-01)
- [ ] `pkg/internal/dev/debounce.go` + `_test.go` — channel-based debouncer (Plan 49-01)
- [ ] `pkg/internal/dev/build.go` + `_test.go` — docker build shell-out (Plan 49-02)
- [ ] `pkg/internal/dev/load.go` + `_test.go` — load-images replication (Plan 49-02)
- [ ] `pkg/internal/dev/rollout.go` + `_test.go` — kubectl rollout restart + wait (Plan 49-02)
- [ ] `pkg/internal/dev/kubeconfig.go` + `_test.go` — temp 0600 kubeconfig writer (Plan 49-02)
- [ ] `pkg/internal/dev/cycle.go` + `_test.go` — runOneCycle with per-step timing (Plan 49-03)
- [ ] `pkg/internal/dev/run.go` + `_test.go` — Run orchestrator + signal handling (Plan 49-03)
- [ ] `go.mod` / `go.sum` — add `github.com/fsnotify/fsnotify v1.10.1` (Plan 49-01)

## Success Criteria → Test Coverage

| ROADMAP SC | Covered By |
|-----------|-----------|
| SC1 (watch mode entry + file-triggered cycle) | TestDevCmd_MissingFlags + TestRun_Banner + integration: TestRun_FileChangeTriggersCycle |
| SC2 (build → load → rollout per cycle with timing) | TestRunOneCycle_BuildArgs + TestRunOneCycle_LoadCalled + TestRolloutRestartAndWait + TestRunOneCycle_TimingOutput |
| SC3 (debounce rapid saves) | TestDebounce_CoalescesBurst |
| SC4 (--poll flag) | TestDevCmd_PollFlag + TestPollLoop_DetectsChange |
