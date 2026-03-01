---
phase: 04-coredns-tuning
plan: "01"
subsystem: CoreDNS Tuning Action
tags: [coredns, dns, configmap, read-modify-write, autopath, cache]
dependency_graph:
  requires: [03-01]
  provides: [DNS-01, DNS-02, DNS-03, DNS-04, DNS-05]
  affects: [create.go addon pipeline]
tech_stack:
  added: []
  patterns: [read-modify-write ConfigMap via kubectl jsonpath, YAML envelope with indented literal block scalar, rollout restart with status wait]
key_files:
  created: []
  modified:
    - pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go
key_decisions:
  - "CoreDNS Corefile patched via in-memory read-modify-write: kubectl get with jsonpath, three string transforms, kubectl apply -f - with YAML envelope"
  - "Guard checks (pods insecure, cache 30, kubernetes cluster.local) added to fail safely if Corefile format changes upstream"
  - "indentCorefile helper prepends 4 spaces to each non-empty line for valid YAML literal block scalar embedding"
  - "No go:embed needed: Corefile read live from cluster at action time, not embedded at build time"
metrics:
  duration: "2 minutes"
  completed: "2026-03-01"
  tasks_completed: 1
  files_modified: 1
---

# Phase 4 Plan 01: CoreDNS Tuning Action Summary

**One-liner:** CoreDNS Corefile patched in-place with autopath @kubernetes, pods verified, and cache 60 via kubectl read-modify-write cycle with guard checks and rollout restart.

## What Was Built

Replaced the stub `Execute` function in `installcorednstuning/corednstuning.go` with a full six-step CoreDNS tuning implementation:

1. Gets the first control plane node via `nodeutils.ControlPlaneNodes`
2. Reads the live CoreDNS Corefile from the cluster using `kubectl get configmap coredns -o jsonpath={.data.Corefile}`
3. Validates three guard conditions to fail safely if the upstream Corefile format changes
4. Applies three string transforms: pods insecure -> pods verified, autopath @kubernetes insertion, cache 30 -> cache 60
5. Writes back the patched Corefile via `kubectl apply -f -` with a proper ConfigMap YAML envelope
6. Runs `kubectl rollout restart deployment/coredns` and waits via `kubectl rollout status --timeout=60s`

DNS-05 (opt-out) was already wired in `create.go` via `runAddon("CoreDNS Tuning", opts.Config.Addons.CoreDNSTuning, installcorednstuning.NewAction())` — no changes needed there.

## Requirements Addressed

| Requirement | Description | Implementation |
|-------------|-------------|----------------|
| DNS-01 | autopath @kubernetes in Corefile | `strings.ReplaceAll` inserts line before `kubernetes cluster.local` |
| DNS-02 | pods verified instead of pods insecure | `strings.ReplaceAll` on `pods insecure` |
| DNS-03 | cache 60 instead of cache 30 | `strings.ReplaceAll` on `cache 30` |
| DNS-04 | CoreDNS pods restart and reach Running | `rollout restart` + `rollout status --timeout=60s` |
| DNS-05 | addons.coreDNSTuning: false skips action | Pre-existing `runAddon` wiring in `create.go` |

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Implement CoreDNS tuning Execute function | 01c83d76 | pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go |

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

- File exists on disk: `pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go` — FOUND
- Commit `01c83d76` exists in git log — FOUND
- `go build ./...` passes — PASSED
- `go vet ./pkg/cluster/internal/create/actions/installcorednstuning/...` passes — PASSED
- No stub, no go:embed, no TODO in file — PASSED
