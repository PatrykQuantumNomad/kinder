---
phase: 26-architecture
plan: "02"
subsystem: cluster-create-actions
tags: [context-propagation, cancellation, addon-actions, waitforready]
dependency_graph:
  requires: [26-01]
  provides: [ARCH-02, ARCH-04]
  affects: [pkg/cluster/internal/create/actions]
tech_stack:
  added: []
  patterns: [CommandContext, select-on-ctx-done, context-threading]
key_files:
  created: []
  modified:
    - pkg/cluster/internal/create/actions/installcertmanager/certmanager.go
    - pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
    - pkg/cluster/internal/create/actions/installmetallb/metallb.go
    - pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go
    - pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go
    - pkg/cluster/internal/create/actions/installdashboard/dashboard.go
    - pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go
    - pkg/cluster/internal/create/actions/waitforready/waitforready.go
    - pkg/cluster/internal/create/actions/waitforready/waitforready_test.go
decisions:
  - All 7 addon Execute() methods use node.CommandContext(ctx.Context, ...) for every in-node command
  - tryUntil uses select on ctx.Done() so poll loop exits immediately on cancellation
  - Host-side exec.Command calls in installlocalregistry are unchanged (not going through Node interface)
  - context.Context parameter threaded through waitForReady and tryUntil as first argument
metrics:
  duration: "~10 minutes"
  completed: "2026-03-04T00:38:44Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 9
---

# Phase 26 Plan 02: Context Propagation Through Addon Execute() Methods Summary

**One-liner:** All 7 addon Execute() methods use node.CommandContext(ctx.Context,...) and tryUntil uses select on ctx.Done() for immediate cancellation.

## What Was Done

Completed the context plumbing across the entire addon layer so every in-node command respects cancellation signals. This is the second half of Phase 26's architecture work — Plan 01 added the Context field to ActionContext; Plan 02 wires that context through to every actual command invocation.

## Task Breakdown

### Task 1: Update all 7 addon Execute() methods to use node.CommandContext

**Commits:** `0febe90a`

Performed mechanical replacement of `node.Command(` with `node.CommandContext(ctx.Context,` in all 7 addon files:

| File | Calls Changed |
|------|--------------|
| installcertmanager/certmanager.go | 3 |
| installenvoygw/envoygw.go | 5 |
| installmetallb/metallb.go | 3 |
| installmetricsserver/metricsserver.go | 2 |
| installcorednstuning/corednstuning.go | 4 |
| installdashboard/dashboard.go | 3 |
| installlocalregistry/localregistry.go | 3 in-node calls |

In installlocalregistry, the 4 host-side `exec.Command(binaryName, ...)` calls were intentionally left unchanged — these are host-side docker/nerdctl commands that do not go through the Node interface and have no context-aware equivalent in the exec package.

### Task 2: Make tryUntil and waitForReady context-aware

**Commits:** `d2dd712d`

Updated `waitforready.go`:
- Added `"context"` to imports
- `tryUntil(ctx context.Context, until time.Time, try func() bool) bool` — new signature
- Replaced `time.Sleep(500 * time.Millisecond)` with `select { case <-ctx.Done(): return false; case <-time.After(500 * time.Millisecond): }`
- `waitForReady(ctx context.Context, node nodes.Node, until time.Time, selectorLabel string) bool` — new signature
- Updated tryUntil call in waitForReady to pass ctx as first arg
- Updated Execute() to pass `ctx.Context` to waitForReady
- Updated node.Command inside waitForReady closure to node.CommandContext(ctx, ...)

Updated `waitforready_test.go`:
- Added `"context"` import
- Updated all 4 existing tests to pass `context.Background()` as first arg to tryUntil
- Added `TestTryUntil_RespectsContextCancellation` which cancels context immediately and verifies tryUntil returns false

## Verification Results

```
go build ./...    -- PASS
go vet ./...      -- PASS
go test ./pkg/cluster/internal/create/... -count=1  -- all PASS
go test -race ./pkg/cluster/internal/create/actions/waitforready/ -count=1  -- PASS (no races)

# Zero remaining node.Command( in 7 addon files:
grep node.Command | grep -v CommandContext | grep -v "//" -- 0 lines

# Host-side exec.Command preserved in localregistry:
grep -c "exec.Command(" localregistry.go -- 4
```

Test results:
- TestTryUntil_SucceedsImmediately: PASS
- TestTryUntil_SucceedsAfterRetries: PASS
- TestTryUntil_TimesOut: PASS
- TestTryUntil_DoesNotBusyLoop: PASS
- TestTryUntil_RespectsContextCancellation: PASS

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

Files verified to exist:
- pkg/cluster/internal/create/actions/installcertmanager/certmanager.go
- pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
- pkg/cluster/internal/create/actions/installmetallb/metallb.go
- pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go
- pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go
- pkg/cluster/internal/create/actions/installdashboard/dashboard.go
- pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go
- pkg/cluster/internal/create/actions/waitforready/waitforready.go
- pkg/cluster/internal/create/actions/waitforready/waitforready_test.go

Commits verified:
- 0febe90a: feat(26-02): propagate ctx.Context to all 7 addon CommandContext calls
- d2dd712d: feat(26-02): make tryUntil and waitForReady context-aware
