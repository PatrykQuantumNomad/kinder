---
gsd_state_version: 1.0
milestone: v2.4
milestone_name: Hardening
status: executing
stopped_at: "Plan 52-01 complete — IPAM probe + doctor check. Commits: bb31049e (Task 1), 143c4588 (Task 2). Next: plan 52-02 (ip-pin) or 52-03 (cert-regen)."
last_updated: "2026-05-10T11:32:20.999Z"
last_activity: 2026-05-10 — Plan 52-01 complete; ProbeIPAM + ipam-probe doctor check; allChecks 24→25
progress:
  total_phases: 7
  completed_phases: 0
  total_plans: 4
  completed_plans: 2
  percent: 50
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-09 — v2.4 Hardening roadmap created)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v2.4 Hardening — Phase 52 (HA Etcd Peer-TLS Fix) — highest blast radius; discuss before planning.

## Current Position

Phase: 52 of 58 (HA Etcd Peer-TLS Fix — Plan 01 complete)
Plan: 52-02 (next: ip-pin lifecycle path) or 52-03 (cert-regen fallback)
Status: In progress — Phase 52, Plan 01 landed
Last activity: 2026-05-10 — Plan 52-01 complete; ProbeIPAM + ipam-probe doctor check; allChecks 24→25

Progress: [█░░░░░░░░░] v2.4 ~5% (1/~20 plans done)

## Performance Metrics

**Velocity:**

- v1.0: 12 plans, 8 phases, 1 day
- v1.1: 8 plans, 6 phases, 2 days
- v1.3: 8 plans, 6 phases, ~5 hours
- v1.4: 13 plans, 5 phases, 2 days
- v2.1: 10 plans, 4 phases, 1 day
- v2.2: 14 plans, 5 phases, ~2.5 days
- v2.3: 25 plans, 5 phases, 5 days
- v2.4 estimate: ~20 plans, 7 phases, 3-4 days

**By Phase:**

| Phase | Plans | Duration |
|-------|-------|----------|
| 52-01 | 2 tasks | ~8 min |

*(v2.4 plan counts evolving — updated after each plan)*

*Updated after each plan completion*

## Accumulated Context

### Decisions

- v1.0–v2.3: See PROJECT.md Key Decisions table
- 2026-05-07 (51-04): SYNC-02 DEFERRED — Docker Hub probe count=0 for kindest/node:v1.36.x. Now tracked as SYNC-05 in v2.4. Re-run once kind publishes v1.36 image.
- 2026-05-09 (roadmap): REQUIREMENTS.md locks cert-manager to v1.20.2 and Envoy Gateway to v1.7.2 — superseding research SUMMARY.md recommendations (v1.16.5 and hold-at-v1.3.1). EG v1.7.2 bump requires companion Gateway API CRD audit and dedicated HTTPRoute UAT in Phase 53-04.
- 2026-05-09 (roadmap): Phase 52 approach — IP pinning preferred (k3d precedent); cert regen is fallback. Docker IPAM feasibility probe is Plan 52-01 Task 1; no code until probe result known.
- 2026-05-10 (52-01): ProbeIPAM API locked — (Verdict, string, error) signature; Verdict constants VerdictIPPinned/VerdictCertRegen/VerdictUnsupported. Tests that use package-level ipamProbeCmder global must NOT be parallel (documented in ipamprobe_test.go).
- 2026-05-10 (52-01): allChecks count: 24 (before 52-01) → 25 (after 52-01) → 26 expected after 52-04. TestAllChecks_CountIs25 must be renamed to CountIs26 in plan 52-04.
- 2026-05-09 (roadmap): Phase 53 sub-plans are strictly sequential (not parallel wave) — ambiguous failures across simultaneous addon bumps are undiagnosable.
- 2026-05-09 (roadmap): Phase 56 (DEBT-04) must precede Phase 57 (doctor cosmetics) — same package, race-clean baseline required.
- 2026-05-09 (roadmap): Phase 58 runs LAST — UAT must verify the final v2.4 binary; Pitfall 23 (stale binary) is the definitive gate.

### Pending Todos

Four pre-existing issues from v2.3 — all addressed as requirements in v2.4:

1. Etcd peer TLS / IP reassignment on pause/resume (→ LIFE-09, Phase 52)
2. cluster-node-skew LB false-positive (→ DIAG-05, Phase 57)
3. cluster-resume-readiness raw JSON dump (→ DIAG-06, Phase 57)
4. allChecks global race under t.Parallel (→ DEBT-04, Phase 56)

### Blockers/Concerns

- **Phase 52 (LIFE-09)**: Docker IPAM static IP feasibility is MEDIUM confidence. Must be verified empirically as first task. Failure triggers cert-regen fallback (not IP pinning). Recommend `/gsd:discuss-phase 52` before planning.
- **Phase 53-02 (ADDON-02)**: Headlamp v0.42.0 token flow verification must precede writing the bump plan. Released 2026-05-07 (2 days before research). Hold at v0.40.1 if token auth regressed.
- **Phase 53-04 (ADDON-04)**: Envoy Gateway v1.7.2 is a two-major-version jump. Companion Gateway API CRD version must be audited. `eg-gateway-helm-certgen` job name must be re-verified in v1.7.2 install.yaml.
- **SYNC-05**: Still externally gated on Docker Hub publishing `kindest/node:v1.36.x`. Probe in Plan 53-00 before any source change.

## Session Continuity

Last session: 2026-05-10T11:32:20.992Z
Stopped at: Plan 52-01 complete — IPAM probe + doctor check. Commits: bb31049e (Task 1), 143c4588 (Task 2). Next: plan 52-02 (ip-pin) or 52-03 (cert-regen).
Resume file: None
