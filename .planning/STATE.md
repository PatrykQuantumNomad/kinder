---
gsd_state_version: 1.0
milestone: v2.2
milestone_name: Cluster Capabilities
status: completed
stopped_at: Completed 45-03-PLAN.md
last_updated: "2026-04-09T16:39:13.965Z"
last_activity: 2026-04-09 — Phase 45 Plan 01 complete (2 tasks, all tests pass, go vet clean)
progress:
  total_phases: 5
  completed_phases: 4
  total_plans: 11
  completed_plans: 11
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-08)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v2.2 Cluster Capabilities — Phase 44 complete, ready for Phase 45

## Current Position

Phase: 45 of 46 (Host Directory Mounting) — IN PROGRESS
Plan: 1/3 complete
Status: Plan 01 complete — validateExtraMounts and logMountPropagationPlatformWarning implemented and tested
Last activity: 2026-04-09 — Phase 45 Plan 01 complete (2 tasks, all tests pass, go vet clean)

Progress: [██████░░░░] 62%

## Performance Metrics

**Velocity:**

- v1.0: 12 plans, 8 phases, 1 day
- v1.1: 8 plans, 6 phases, 2 days
- v1.2: 4 phases, 1 day
- v1.3: 8 plans, 6 phases, ~5 hours
- v1.4: 13 plans, 5 phases, 2 days
- v1.5: 7 plans, 5 phases, 1 day
- v2.0: 7 plans, 3 phases, 2 days
- v2.1: 10 plans, 4 phases, 1 day

**By Phase:** Not started

*Updated after each plan completion*

## Accumulated Context

### Decisions

- v1.0-v2.1: See PROJECT.md Key Decisions table
- v2.2 planning: Zero new Go module dependencies — all features use packages already in go.mod
- v2.2 planning: Phase 43 must be stable before Phase 44 (air-gap image warning lists local-path images)
- v2.2 planning: Phase 43 is a dependency of Phase 46 (load images supports the offline workflow)
- v2.2 planning: Phases 43 and 46 flagged for `/gsd-research-phase` during planning (Provider interface change, Docker Desktop 27+ fallback)
- [Phase 42]: Non-semver image tags (e.g. 'latest') skip version-skew validation to preserve backward compat with test/dev configs
- [Phase 42]: ExplicitImage captured pre-defaults in encoding/convert.go — SetDefaultsCluster fills empty Image fields before Convertv1alpha4 runs, making post-defaults detection impossible
- [Phase 42 plan 02]: ComputeSkew always shows cross+delta for any non-zero difference; ok=false only when >3 behind or any ahead of CP
- [Phase 42 plan 02]: nodeEntry.VersionErr field enables test injection of read-failures without real container runtime
- [Phase 42]: IMAGE column populated via container inspect CLI (avoids import cycle with cluster package); doctor check realListNodes uses same low-level CLI approach
- [Phase 43]: RequiredAddonImages imports addon packages into common/images.go; no import cycle since addon packages do not import common
- [Phase 43]: localregistry Images var references existing private registryImage const rather than duplicating the literal string
- [Phase 43]: inspectImageFunc package-level var in docker provider enables test injection without requiring a real Docker daemon
- [Phase 43]: nerdctl formatMissingImagesError takes binaryName parameter for runtime-specific pre-load instructions
- [Phase 43]: Addon image warning uses addonImages.Len() > 0 guard to avoid empty NOTE when no addons enabled
- [Phase 43]: Image list defined inline in offlinereadiness.go — no import from pkg/cluster/internal to avoid import cycle
- [Phase 43]: offlineReadinessCheck skips gracefully when no container runtime found (lookPath detection before inspect)
- [Phase 44-01]: LocalPath uses boolVal (opt-out, default true) not boolValOptIn — consistent with MetalLB/CertManager pattern
- [Phase 44-01]: StorageClass named local-path (not standard) to avoid collision with legacy installstorage
- [Phase 44-01]: installstorage gated behind !LocalPath in sequential pipeline (before kubeadmjoin)
- [Phase 44-01]: Images var uses docker.io/ prefix for both images (docker.io/rancher/local-path-provisioner:v0.0.35, docker.io/library/busybox:1.37.0)
- [Phase 44-02]: offlinereadiness entries use docker.io/ prefix matching the Images var declaration in localpathprovisioner.go
- [Phase 44-02]: AllChecks count tests updated to 21 — feat(44-03) ran out of order adding local-path-cve check before plan 44-02 tests were written
- [Phase 44-03]: CVE threshold v0.0.34 returns ok (it is the fix version); only strictly less-than triggers warn
- [Phase 44-03]: realGetProvisionerVersion uses container exec kubectl inside kind control-plane — avoids import cycle with pkg/cluster/internal same as realListNodes in clusterskew.go
- [Phase 45-01]: validateExtraMounts called after Config.Validate() and before Provision() — host paths verified before any container is created
- [Phase 45-01]: logMountPropagationPlatformWarning warns once (return after first non-None match) — mirrors logMetalLBPlatformWarning pattern
- [Phase 45-01]: Relative paths resolved via filepath.Abs before os.Stat — avoids false negatives for CWD-relative paths
- [Phase 45-01]: Test file reuses existing testLogger from create_addon_test.go (same package) — avoids duplicate type declarations
- [Phase 45]: isPathCovered uses dir+/ separator to prevent prefix false positives (/Userspace vs /Users)
- [Phase 45]: dockerDesktopFileSharingCheck falls back to Docker Desktop default dirs when settings-store.json absent
- [Phase 45-03]: Guide uses absolute hostPath in YAML example (not tilde) because kind config does not perform shell expansion

### Pending Todos

None.

### Blockers/Concerns

- Phase 46 (load images): Docker Desktop 27+ `--local` flag availability needs verification against a live environment before implementation begins

## Session Continuity

Last session: 2026-04-09T16:39:09.921Z
Stopped at: Completed 45-03-PLAN.md
Resume file: None
