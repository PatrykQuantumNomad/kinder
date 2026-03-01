---
phase: 01-foundation
plan: 02
subsystem: create-pipeline
tags: [addon-pipeline, stub-actions, warn-continue, platform-detection, summary-output]
dependency_graph:
  requires: [01-01]
  provides: [addon-execution-pipeline, stub-action-packages]
  affects: [02-metallb, 03-metrics-server, 04-coredns-tuning, 05-envoy-gateway, 06-dashboard]
tech_stack:
  added: [runtime.GOOS platform detection]
  patterns: [warn-and-continue error handling, closure-based action runner, addon result accumulation]
key_files:
  created:
    - pkg/cluster/internal/create/actions/installmetallb/metallb.go
    - pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
    - pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go
    - pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go
    - pkg/cluster/internal/create/actions/installdashboard/dashboard.go
  modified:
    - pkg/cluster/internal/create/create.go
decisions:
  - "Addon loop uses a closure (runAddon) rather than a separate function to keep actionsContext and addonResults in natural scope"
  - "Platform warning fires after all addon actions so it groups visually with the addon summary, not during execution"
  - "Salutation updated from 'kind' to 'kinder' — URLs left pointing to kind docs until kinder docs exist"
metrics:
  duration: 2 minutes
  completed: 2026-03-01
  tasks_completed: 2
  files_created: 5
  files_modified: 1
requirements_satisfied: [FOUND-04, FOUND-05]
---

# Phase 1 Plan 02: Action Pipeline Scaffolding Summary

**One-liner:** Five no-op addon action stubs wired into create.go with warn-and-continue loop, runtime.GOOS platform warning for MetalLB on macOS/Windows, dependency conflict detection, and a scannable addon summary block.

## Performance

- Duration: ~2 minutes
- Tasks completed: 2/2
- Files created: 5
- Files modified: 1
- Tests: 17 packages, all passing

## Accomplishments

- Created five stub addon action packages under `pkg/cluster/internal/create/actions/`:
  - `installmetallb` — no-op stub, TODO Phase 2
  - `installmetricsserver` — no-op stub, TODO Phase 3
  - `installcorednstuning` — no-op stub, TODO Phase 4
  - `installenvoygw` — no-op stub, TODO Phase 5
  - `installdashboard` — no-op stub, TODO Phase 6
- Each stub follows the `installstorage` pattern: Apache 2.0 header, single-import, `NewAction() actions.Action`, `Execute()` that returns nil
- Wired addon execution pipeline into `create.go`:
  - `addonResult` struct tracks name/enabled/err per addon
  - `runAddon` closure handles skip (disabled) and warn-and-continue (failed) cases
  - Dependency conflict check warns when MetalLB disabled but Envoy Gateway enabled
  - Five addon actions run in dependency order gated on `opts.Config.Addons.<Name>` booleans
  - `logMetalLBPlatformWarning` uses `runtime.GOOS` to warn on darwin/windows
  - `logAddonSummary` prints installed/skipped/FAILED per addon after all actions run
- Updated salutation from "Thanks for using kind!" to "Thanks for using kinder!"

## Task Commits

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Create five stub addon action packages | 82951b29 | 5 new files |
| 2 | Wire addon action loop into create.go | db99cb00 | create.go |

## Key Files

**Created:**
- `/pkg/cluster/internal/create/actions/installmetallb/metallb.go`
- `/pkg/cluster/internal/create/actions/installenvoygw/envoygw.go`
- `/pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go`
- `/pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go`
- `/pkg/cluster/internal/create/actions/installdashboard/dashboard.go`

**Modified:**
- `/pkg/cluster/internal/create/create.go` — imports, addonResult struct, runAddon closure, dependency check, 5 addon calls, platform warning, addon summary, helper functions, salutation update

## Decisions Made

1. **Addon loop uses a closure** (`runAddon`) rather than a top-level function. This keeps `actionsContext` and `addonResults` in natural closure scope without threading them as parameters, matching the surrounding code style.

2. **Platform warning fires after all addon runs, before summary.** Groups the warning visually with the addon results block rather than interrupting the execution output mid-stream.

3. **Salutation updated "kind" to "kinder".** URLs left pointing to kind docs (https://kind.sigs.k8s.io/) as kinder-specific docs do not yet exist.

## Deviations from Plan

None — plan executed exactly as written.

## Issues / Deferred Items

None.

## Next Phase Readiness

Phase 1 is complete. The addon action pipeline is in place. Each of Phases 2-6 will replace one stub `Execute()` with real implementation — no changes to create.go or the pipeline structure are needed. MetalLB (Phase 2) is the first dependency and must complete before Envoy Gateway (Phase 5).

## Self-Check: PASSED

All 6 key files verified present. Both task commits (82951b29, db99cb00) verified in git history. All 17 test packages pass.
