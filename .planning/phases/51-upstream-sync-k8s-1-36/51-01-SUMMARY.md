---
phase: 51-upstream-sync-k8s-1-36
plan: "01"
subsystem: loadbalancer
tags:
  - envoy
  - haproxy-removal
  - tdd
  - upstream-sync
  - providers

dependency_graph:
  requires: []
  provides:
    - loadbalancer.Image (Envoy v1.36.2)
    - loadbalancer.ProxyConfigPath/CDS/LDS/Dir constants
    - loadbalancer.Config (two-arg form)
    - loadbalancer.GenerateBootstrapCommand
    - loadbalancer.ProxyLDSConfigTemplate
    - loadbalancer.ProxyCDSConfigTemplate
    - loadbalancer.DynamicFilesystemConfigTemplate
  affects:
    - pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go
    - pkg/cluster/internal/providers/docker/create.go
    - pkg/cluster/internal/providers/podman/provision.go
    - pkg/cluster/internal/providers/nerdctl/create.go
    - pkg/internal/doctor/offlinereadiness.go

tech_stack:
  added: []
  patterns:
    - TDD RED/GREEN per task (2 RED commits + 2 GREEN commits)
    - text/template FuncMap for hostPort splitting in CDS template
    - Envoy xDS filesystem-based dynamic config (atomic mv-swap reload)
    - GenerateBootstrapCommand pattern: bash -c "mkdir+echo+retry-loop"

key_files:
  created:
    - pkg/cluster/internal/loadbalancer/config_test.go
  modified:
    - pkg/cluster/internal/loadbalancer/const.go
    - pkg/cluster/internal/loadbalancer/config.go
    - pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go
    - pkg/cluster/internal/providers/docker/create.go
    - pkg/cluster/internal/providers/podman/provision.go
    - pkg/cluster/internal/providers/nerdctl/create.go
    - pkg/internal/doctor/offlinereadiness.go
    - pkg/internal/doctor/offlinereadiness_test.go
    - pkg/cluster/internal/providers/docker/create_test.go
    - pkg/cluster/internal/providers/podman/provision_test.go
    - pkg/cluster/internal/providers/nerdctl/create_test.go

decisions:
  - "GenerateBootstrapCommand appends bash/-c/<mkdir+echo+retry> after image in all three providers; exact upstream kind behavior"
  - "LB reload uses atomic mv-swap (chmod&&mv&&mv) not SIGHUP — Envoy xDS filesystem polling picks up changes"
  - "Config() now two-arg (data, template string) matching upstream kind PR #4127"
  - "hostPort FuncMap uses net.SplitHostPort returning map[string]string{host,port} for CDS template"
  - "DualStackFamily also gets IPv6=true in loadbalancer action (preserved from kinder's original code)"

metrics:
  duration: "~5m"
  completed: "2026-05-07"
  tasks: 3
  commits: 4
  files_changed: 11
---

# Phase 51 Plan 01: Envoy LB Migration Summary

Ported upstream kind PR #4127 (merged 2026-04-02) into kinder: replaced the HAProxy load-balancer container with `envoyproxy/envoy:v1.36.2` across all three providers (docker/podman/nerdctl), rewrote the LB config action to use atomic `mv`-swap for xDS reload, and updated the doctor offline-readiness image list.

## Tasks Executed

| # | Task | Type | Status | Commit |
|---|------|------|--------|--------|
| 1 | TDD RED — failing tests for Envoy const + config | auto | DONE | 5955fc80 |
| 2 | TDD GREEN — port Envoy const + config from upstream | auto | DONE | 8c005c28 |
| 3 RED | TDD RED — failing tests for provider LB bootstrap wiring | auto | DONE | 90e793d8 |
| 3 GREEN | TDD GREEN — atomic-swap LB action + provider bootstrap | auto | DONE | 4267886a |

## Commits Landed

| Hash | Type | Message |
|------|------|---------|
| 5955fc80 | RED | test(51-01): add failing tests for Envoy LB constants and bootstrap command |
| 8c005c28 | GREEN | feat(51-01): port Envoy LB constants, templates, and bootstrap command from kind PR #4127 |
| 90e793d8 | RED | test(51-01): add failing tests for provider LB bootstrap wiring |
| 4267886a | GREEN | feat(51-01): wire Envoy bootstrap into providers and atomic-swap LB reload |

## Test Counts

| Package | Tests Before | Tests After | New Tests |
|---------|-------------|-------------|-----------|
| pkg/cluster/internal/loadbalancer | 0 | 9 | 9 |
| pkg/cluster/internal/providers/docker | 3 | 4 | 1 |
| pkg/cluster/internal/providers/podman | 5 | 6 | 1 |
| pkg/cluster/internal/providers/nerdctl | 1 | 2 | 1 |
| pkg/internal/doctor (offline-readiness only) | 5 | 6 | 1 |
| **Total** | **14** | **27** | **13** |

All 27 tests pass with `-race`.

## Files Changed

### Created
- `pkg/cluster/internal/loadbalancer/config_test.go` — 9 tests: image constant, 4 path constants, LDS IPv4/IPv6, CDS backend servers, GenerateBootstrapCommand shape, offline readiness Envoy image

### Modified
- `pkg/cluster/internal/loadbalancer/const.go` — replace HAProxy image + `ConfigPath` with Envoy image + 4 Proxy path constants
- `pkg/cluster/internal/loadbalancer/config.go` — full rewrite: `DynamicFilesystemConfigTemplate`, `ProxyLDSConfigTemplate`, `ProxyCDSConfigTemplate`, two-arg `Config(data, template)`, `hostPort` FuncMap, `GenerateBootstrapCommand`
- `pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go` — dual `Config()` calls for LDS+CDS, write to `.tmp` files, atomic `mv`-swap, remove `kill -s HUP 1`
- `pkg/cluster/internal/providers/docker/create.go` — append `GenerateBootstrapCommand` after `loadbalancer.Image`
- `pkg/cluster/internal/providers/podman/provision.go` — append `GenerateBootstrapCommand` after sanitized `loadbalancer.Image`
- `pkg/cluster/internal/providers/nerdctl/create.go` — append `GenerateBootstrapCommand` after `loadbalancer.Image`
- `pkg/internal/doctor/offlinereadiness.go` — update LB (HA) entry from `kindest/haproxy:v20260131-7181c60a` to `envoyproxy/envoy:v1.36.2`
- `pkg/internal/doctor/offlinereadiness_test.go` — add `TestOfflineReadinessIncludesEnvoyImage`
- `pkg/cluster/internal/providers/docker/create_test.go` — add `TestRunArgsForLoadBalancerAppendsBootstrap`
- `pkg/cluster/internal/providers/podman/provision_test.go` — add `TestRunArgsForLoadBalancerAppendsBootstrap_Podman`
- `pkg/cluster/internal/providers/nerdctl/create_test.go` — add `TestRunArgsForLoadBalancerAppendsBootstrap_Nerdctl`

## Final Sanity Check

```
grep -rn "kindest/haproxy" pkg/ | grep -v _test.go  → no output (CLEAN)
grep -rn "kill.*HUP" pkg/cluster/internal/create/actions/loadbalancer/ → no output (CLEAN)
grep -rn "GenerateBootstrapCommand" pkg/cluster/internal/providers/*/  → 1 match per file (CORRECT)
go build ./...  → exit 0 (CLEAN)
```

## Deviations from Plan

### None — plan executed exactly as written.

The only noteworthy decision: kinder's original `loadbalancer.go` set `IPv6: ctx.Config.Networking.IPFamily == config.IPv6Family`. The upstream kind code uses the same condition. Kinder also had `|| config.DualStackFamily` in the original. This was preserved since it was kinder-specific and not a regression.

Pre-existing data race in `pkg/internal/doctor/check_test.go` (allChecks global under t.Parallel) — documented in STATE.md at baseline commit c138ad62, pre-Phase-50. NOT caused by this plan. Full doctor package tests with `-race` flag trigger these pre-existing races; running targeted tests passes cleanly.

## Known Stubs

None. All production code paths are fully wired.

## Threat Flags

None. No new network endpoints, auth paths, or trust boundaries introduced. The Envoy image reference replaces the HAProxy reference — same surface, different implementation.

## Self-Check: PASSED

Files exist:
- pkg/cluster/internal/loadbalancer/const.go ✓
- pkg/cluster/internal/loadbalancer/config.go ✓
- pkg/cluster/internal/loadbalancer/config_test.go ✓
- pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go ✓
- pkg/cluster/internal/providers/docker/create.go ✓
- pkg/cluster/internal/providers/podman/provision.go ✓
- pkg/cluster/internal/providers/nerdctl/create.go ✓
- pkg/internal/doctor/offlinereadiness.go ✓
- pkg/internal/doctor/offlinereadiness_test.go ✓

Commits exist: 5955fc80, 8c005c28, 90e793d8, 4267886a ✓
