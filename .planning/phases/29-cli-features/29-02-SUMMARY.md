---
phase: 29
plan: "02"
subsystem: cli
tags: [json-output, profile-presets, get-nodes, create-cluster, cli-ux]
dependency_graph:
  requires: [29-01]
  provides: [get-nodes-json, create-cluster-profile]
  affects: [pkg/cmd/kind/get/nodes, pkg/cluster, pkg/cmd/kind/create/cluster]
tech_stack:
  added: [encoding/json]
  patterns: [flagpole-extension, createOptionAdapter, json-encoder]
key_files:
  created: []
  modified:
    - pkg/cmd/kind/get/nodes/nodes.go
    - pkg/cluster/createoption.go
    - pkg/cmd/kind/create/cluster/createcluster.go
decisions:
  - JSON branch fires before human-readable empty-node checks so --output json always returns valid JSON array (empty or populated)
  - CreateWithAddonProfile nil-guards o.Config by loading default config; avoids nil dereference when no --config flag given
  - Empty --profile is a strict no-op; default cluster creation behavior is fully preserved
  - --profile wired after withConfig in provider.Create() so profile addons override any config-file addon settings
metrics:
  duration: "~10 minutes"
  completed: "2026-03-04T11:28:00Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 3
---

# Phase 29 Plan 02: --output json on get nodes + --profile flag on create cluster Summary

JSON output for `kinder get nodes` via `--output json` flag returning `[]nodeInfo{name,role}`, plus four addon profile presets (minimal, full, gateway, ci) for `kinder create cluster` via `--profile` flag backed by `CreateWithAddonProfile` option.

## Tasks Completed

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Add --output json to kinder get nodes | `090fabc0` | pkg/cmd/kind/get/nodes/nodes.go |
| 2 | Add --profile flag to kinder create cluster + CreateWithAddonProfile option | `1741621e` | pkg/cluster/createoption.go, pkg/cmd/kind/create/cluster/createcluster.go |

## What Was Built

### Task 1 — --output json for kinder get nodes

`pkg/cmd/kind/get/nodes/nodes.go`:

- Added `Output string` to `flagpole` struct
- Added `nodeInfo` struct with `name` and `role` JSON fields
- Registered `--output` flag (`""` or `"json"`)
- Validation at top of `runE` rejects unknown output values
- JSON branch uses `json.NewEncoder(streams.Out).Encode(infos)` with `make([]nodeInfo, 0)` so empty clusters return `[]` not `null`
- Human-readable empty-node messages preserved in original if/else structure, just moved after the JSON branch

### Task 2 — --profile flag for kinder create cluster

`pkg/cluster/createoption.go`:

- Added imports: `"fmt"` and `internalconfig "sigs.k8s.io/kind/pkg/internal/apis/config"`
- Added `CreateWithAddonProfile(profile string) CreateOption`
  - Empty string is a strict no-op (returns nil immediately)
  - Nil `o.Config` guard loads default config via `internalencoding.Load("")`
  - Four profiles: `minimal` (all addons off), `full` (all seven addons on), `gateway` (MetalLB + EnvoyGateway), `ci` (MetricsServer + CertManager)
  - Unknown profile returns descriptive error listing valid values

`pkg/cmd/kind/create/cluster/createcluster.go`:

- Added `Profile string` to `flagpole`
- Registered `--profile` flag with empty default
- Wired `cluster.CreateWithAddonProfile(flags.Profile)` into `provider.Create()` after `withConfig` so profile settings override any config-file addon section

## Decisions Made

1. **JSON branch before empty-check**: Ensures `--output json` always returns valid JSON (`[]` for empty, not a log message). Consistent with plan specification.
2. **nil Config guard in CreateWithAddonProfile**: Without this, `o.Config` would be nil when no `--config` flag is given, causing a nil-pointer panic when setting `o.Config.Addons`.
3. **Profile after withConfig in Create() call**: Allows `--profile` to override addons set by a config file. This is the intentional semantic — the flag is an explicit override.
4. **Empty profile = strict no-op**: No changes to `o.Config` at all, so clusters created without `--profile` are completely unaffected.

## Deviations from Plan

None — plan executed exactly as written.

## Verification

```
go build ./... && go vet ./pkg/cmd/kind/get/nodes/ ./pkg/cluster/ ./pkg/cmd/kind/create/cluster/
```

Both commands succeeded with no errors or warnings.

## Self-Check

- [x] pkg/cmd/kind/get/nodes/nodes.go exists and has nodeInfo struct, Output flag, JSON branch
- [x] pkg/cluster/createoption.go has CreateWithAddonProfile with nil Config guard, 4 profiles, error for unknown
- [x] pkg/cmd/kind/create/cluster/createcluster.go has Profile flag, CreateWithAddonProfile wired after withConfig
- [x] Commits 090fabc0 and 1741621e exist
- [x] go build ./... succeeds
