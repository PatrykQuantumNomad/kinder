---
gsd_state_version: 1.0
milestone: v2.1
milestone_name: Known Issues & Proactive Diagnostics
status: active
stopped_at: null
last_updated: "2026-03-06"
last_activity: "2026-03-06 — Completed 41-01 (subnet clash detection with TDD)"
progress:
  total_phases: 4
  completed_phases: 3
  total_plans: 10
  completed_plans: 9
  percent: 90
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-06)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 41 - Network, Create-Flow Integration, and Website

## Current Position

Phase: 41 (4 of 4) — Network, Create-Flow Integration, and Website
Plan: 2 of 3 (41-01, 41-02 complete)
Status: Executing Phase 41
Last activity: 2026-03-06 — Completed 41-01 (subnet clash detection with TDD)

Progress: [█████████░] 90%

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
- [Phase 40]: P40-02: AppArmor and SELinux checks are completely independent (LSM stacking since kernel 5.1)
- [Phase 40]: P40-02: SELinux warns only on Fedora (ID=fedora in os-release); returns ok on non-Fedora enforcing
- [Phase 40]: P40-02: Firewalld defaults to nftables when FirewallBackend config line absent (Fedora 32+ default)
- [Phase 40]: P40-02: isFedora returns false on os-release read error (err on safe side)
- [Phase 40]: P40-01: inotifyCheck returns multiple results when both limits low, single ok when sufficient
- [Phase 40]: P40-01: kernelVersionCheck uses fail (not warn) for < 4.6 -- cgroup namespace is hard blocker
- [Phase 40]: P40-01: kernel_other.go stub returns nil from Run(), matching platform filtering pattern
- [Phase 40]: P40-01: fakeReadFile test helper in inotify_test.go, reusable across doctor package
- [Phase 40]: P40-03: WSL2 detection requires two signals (microsoft + WSL_DISTRO_NAME/WSLInterop) to prevent Azure VM false positives
- [Phase 40]: P40-03: Cgroup v2 check verifies cpu, memory, pids controllers when WSL2 confirmed
- [Phase 40]: P40-03: Rootfs check queries both Docker .Driver and .DriverStatus for BTRFS detection
- [Phase 40]: P40-03: allChecks registry ordered: Runtime(1), Docker(4), Tools(2), GPU(3), Kernel(2), Security(2), Platform(3)
- [Phase 41]: P41-02: SafeMitigations returns []SafeMitigation{} matching allChecks non-nil guarantee pattern
- [Phase 41]: P41-02: ApplySafeMitigations called between containerd config patches and p.Provision()
- [Phase 41]: P41-02: Mitigation errors are warnings, never block cluster creation
- [Phase 41]: P41-01: Injectable getRoutesFunc for testable route parsing without system calls
- [Phase 41]: P41-01: Self-referential Docker routes skipped by exact netip.Prefix equality
- [Phase 41]: P41-01: IPv4 only -- IPv6 ULA clashes are extremely rare
- [Phase 41]: P41-01: allChecks registry now 18 checks: Runtime(1), Docker(4), Tools(2), GPU(3), Kernel(2), Security(2), Platform(3), Network(1)

### Pending Todos

None.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-03-06
Stopped at: Completed 41-01-PLAN.md. Subnet clash detection implemented with TDD. Next: 41-03 (website).
Resume file: None
