---
gsd_state_version: 1.0
milestone: v2.1
milestone_name: Known Issues & Proactive Diagnostics
status: active
stopped_at: null
last_updated: "2026-03-06"
last_activity: "2026-03-06 — Completed 39-02 (kubectl version skew, docker socket checks)"
progress:
  total_phases: 4
  completed_phases: 2
  total_plans: 4
  completed_plans: 4
  percent: 50
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-06)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 39 - Docker and Tool Configuration Checks

## Current Position

Phase: 39 (2 of 4) — Docker and Tool Configuration Checks [COMPLETE]
Plan: 2 of 2 complete
Status: Phase 39 complete, ready for next phase
Last activity: 2026-03-06 — Completed 39-02 (kubectl version skew, docker socket checks)

Progress: [█████░░░░░] 50%

## Performance Metrics

**Velocity:**
- v1.0: 12 plans, 8 phases, 1 day
- v1.1: 8 plans, 6 phases, 2 days
- v1.2: 4 phases, 1 day
- v1.3: 8 plans, 6 phases, ~5 hours
- v1.4: 13 plans, 5 phases, 2 days
- v1.5: 7 plans, 5 phases, 1 day
- v2.0: 7 plans, 3 phases, 2 days

## Accumulated Context

### Decisions

- v1.0-v2.0: See PROJECT.md Key Decisions table
- v2.1: INFRA-05 (create flow integration) moved to Phase 41 so all checks exist before wiring mitigations into create flow
- v2.1: Research confirms zero new go.mod dependencies; golang.org/x/sys/unix promoted from indirect to direct
- v2.1 P38-01: allChecks initialized as []Check{} (not nil) for non-nil guarantee
- v2.1 P38-01: FormatJSON returns map[string]interface{} for flexible JSON serialization
- v2.1 P38-01: ApplySafeMitigations early-returns on non-Linux platforms
- [Phase 38]: P38-02: Inline fakeCmd test doubles instead of importing testutil from pkg/cluster/internal/
- [Phase 38]: P38-02: nvidiaDockerRuntimeCheck returns skip when docker binary not found
- [Phase 39]: P39-01: Build-tagged statfsFreeBytes with int64 cast for macOS/Linux Bsize portability
- [Phase 39]: P39-01: daemonJSONCheck searches 6 candidate paths including Windows ProgramData
- [Phase 39]: P39-01: dockerSnapCheck uses filepath.EvalSymlinks for snap detection
- [Phase 39]: P39-02: referenceK8sMinor = 31 as constant, +/-1 minor skew tolerance for kubectl
- [Phase 39]: P39-02: dockerSocketCheck linux-only, macOS Docker Desktop manages socket permissions
- [Phase 39]: P39-02: Non-permission docker info failures return ok (daemon-not-running handled elsewhere)

### Pending Todos

None.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-03-06
Stopped at: Completed 39-02-PLAN.md, Phase 39 complete. Ready for next phase.
Resume file: None
