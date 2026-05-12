---
gsd_state_version: 1.0
milestone: v2.4
milestone_name: Hardening
status: executing
stopped_at: "Phase 53 ALL 8 PLANS COMPLETE — Plan 53-07 (offlinereadiness consolidation) closed with pass-with-deviation. SC1 first clause satisfied; SC1 second clause wording unsatisfiable as written (see 53-07-SUMMARY.md DEVIATION). Phase 53 ready for /gsd-verify-work."
last_updated: "2026-05-10T18:00:00Z"
last_activity: "2026-05-10 — Plan 53-07 complete; allAddonImages updated (14 entries, ADDON-05 closed); v2.4 changelog + release-notes-v2.4-draft.md shipped; Phase 53 all 8 plans done"
progress:
  total_phases: 7
  completed_phases: 1
  total_plans: 12
  completed_plans: 12
  percent: 79
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-09 — v2.4 Hardening roadmap created)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v2.4 Hardening — Phase 53 (Addon Version Audit, Bumps & SYNC-05) — ALL 8 PLANS COMPLETE. Phase ready for /gsd-verify-work.

## Current Position

Phase: 53 of 58 (Addon Version Audit, Bumps & SYNC-05)
Plan: 53-07 COMPLETE (offlinereadiness consolidation; ADDON-05 closed; all 14 allAddonImages tags mirrored; v2.4 changelog + release-notes-v2.4-draft.md shipped)
Status: Phase 53 COMPLETE — Plans 53-00 through 53-07 all done (8/8). Ready for /gsd-verify-work.
Last activity: 2026-05-10 — Plan 53-07 complete; allAddonImages updated; Phase 53 all 8 plans done

Progress: [████░░░░░░] v2.4 ~36% (10/~28 plans done)

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
| 52-02 | 2 tasks | ~35 min |
| 52-03 | 2 tasks | ~11 min |
| 52-04 | 2 tasks | ~12 min |
| 53-00 | 1 task (Outcome B) | ~3 min |
| 53-01 | 2 tasks (RED+GREEN) | ~3 min |
| 53-02 | 3 tasks (RED+UAT+GREEN) | ~15 min |
| 53-03 | 3 tasks (RED+UAT+GREEN) | ~20 min |
| 53-04 | 3 tasks (RED+UAT+GREEN) | ~45 min (two sessions; includes live UAT-4) |
| 53-05 | 1 task (hold-verify probe) | ~2 min |
| 53-06 | 1 task (hold-verify probe) | ~2 min |
| 53-07 | 2 tasks (RED+GREEN) + UAT-5 | ~30 min (two sessions) |

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
- 2026-05-10 (52-03): certRegenSleeper package-level var injection prevents 45s+ test blocks; same pattern as ipamProbeCmder in doctor package.
- 2026-05-10 (52-03): applyPinnedIPsBeforeCPStart uses os.TempDir() as tmpDir; tests pre-write ipam-state.json there with t.Cleanup removal.
- 2026-05-10 (52-03): Strategy constants re-exported as typed const in lifecycle/ippin.go so resume.go calls StrategyIPPinned (not constants.StrategyIPPinned) — W2 naming requirement satisfied.
- 2026-05-10 (52-03): haTestCmder dispatch: switch on name first (kubeadm, mv) then args[0] (start, inspect, network) — covers node.Command() routing through defaultCmder.
- 2026-05-10 (52-04): listKinderCPContainersByCluster is a NEW helper returning map[clusterName][]containerName; realListCPNodes was NOT reused because it flattens all CPs across clusters into []string, making multi-cluster detection (Verdict 8) impossible.
- 2026-05-10 (52-04): pkg/cluster/constants imported directly from pkg/internal/doctor — zero-import package, no cycle. No local constant mirrors needed.
- 2026-05-10 (52-04): Mixed-label verdict is fail (genuine corruption); legacy absent-label and explicit cert-regen are both warn — per CONTEXT.md D-locks.
- 2026-05-09 (roadmap): Phase 53 sub-plans are strictly sequential (not parallel wave) — ambiguous failures across simultaneous addon bumps are undiagnosable.
- 2026-05-09 (roadmap): Phase 56 (DEBT-04) must precede Phase 57 (doctor cosmetics) — same package, race-clean baseline required.
- 2026-05-09 (roadmap): Phase 58 runs LAST — UAT must verify the final v2.4 binary; Pitfall 23 (stale binary) is the definitive gate.
- 2026-05-10 (53-00): SYNC-05 DEFERRED — Docker Hub probe count=0 for kindest/node:v1.36.x (same as SYNC-02 on 2026-05-07). SC6 remains DEFERRED. Sub-plans 53-01 through 53-07 proceed normally. Re-run once kind publishes v1.36 image.
- 2026-05-10 (53-01): local-path-provisioner v0.0.36 dropped --helper-image deployment flag; busybox:1.37.0 pin now only required in helperPod.yaml ConfigMap template (one occurrence, not two). TestManifestPinsBusybox threshold updated to >= 1. Upstream RBAC simplification and CONFIG_MOUNT_PATH env var accepted.
- 2026-05-10 (53-02): Headlamp v0.42.0 Path A — live UAT-2 confirmed RBAC=yes, UI=200, SA+Secret resolve. Upstream OTEL telemetry env vars merged; kinder-dashboard SA, kinder-dashboard-token Secret, -in-cluster arg, targetPort:4466 all preserved. ADDON-02 delivered.
- 2026-05-10 (53-03): cert-manager v1.20.2 Path A — live UAT-3 confirmed ClusterIssuer + Certificate smoke; pods Running. ADDON-03 delivered. DEVIATION: plan's runAsUser=65532 jsonpath assertion was overspecified — upstream v1.20.2 uses distroless image USER directive (UID 65532) rather than manifest securityContext.runAsUser; kubelet enforces runAsNonRoot: true; security intent (Pitfall CERT-03) is satisfied. Future addon-bump plans: do NOT assert specific UID via manifest jsonpath for distroless images; check runAsNonRoot: true instead. CONTEXT.md had typo "65632"; authoritative value is 65532 per REQUIREMENTS.md and upstream release notes.
- 2026-05-10 (53-04): Envoy Gateway v1.7.2 Path A — live UAT-4 confirmed GatewayClass Accepted, Gateway Programmed, HTTPRoute Accepted, HTTP 200 in-cluster curl. ADDON-04 delivered. Gateway API CRDs upgraded from v1.2.1 to v1.4.1 in-band. eg-gateway-helm-certgen Job name unchanged (Pitfall EG-02 cleared). UAT-SCRIPT NOTE 1: hashicorp/http-echo image has CLI-arg shape issues causing CrashLoopBackOff — future EG UAT scripts should use nginx as backend. UAT-SCRIPT NOTE 2: macOS hosts cannot curl docker-bridge IPs (curl HTTP 000); EG UAT scripts should use kubectl run uat-curl (in-cluster curl) or kubectl port-forward on macOS (matching Headlamp UAT-2 pattern).
- 2026-05-10 (53-05): MetalLB hold reaffirmed at v0.15.3 — GitHub releases API probe on 2026-05-10 confirms v0.15.3 is still the latest release (published 2025-12-04); no v0.16.x present in top-5 listing. ADDON-05 hold-verify delivered. No Go source change; offlinereadiness consolidation in 53-07.
- 2026-05-10 (53-06): Metrics Server hold reaffirmed at v0.8.1 — GitHub releases API probe on 2026-05-10 confirms v0.8.1 is still the latest release (published 2026-01-29); no v0.9.x present in top-5 listing. ADDON-05 hold-verify delivered. No Go source change; offlinereadiness consolidation in 53-07.
- 2026-05-10 (53-07): offlinereadiness.go realInspectImage calls 'docker inspect --type=image' against the HOST docker store (not the cluster node's containerd store). This is the correct semantic: the check measures air-gapped readiness (images must be pre-pulled on host before 'kinder create cluster --air-gapped'). On a fresh default cluster the check correctly warns for any addon image absent from host docker — this is NOT a regression. All 14 allAddonImages tags verified present on uat-53-07 cluster node via 'crictl images' (SC1 first clause satisfied). SC1 second clause as written ('no warn|missing on a fresh default cluster') conflates default-cluster boot with air-gapped readiness — the two semantics are different. Plan 53-07 closes with pass-with-deviation; Phase 53 verifier should re-word SC1 second clause to reference crictl verification on the node rather than host docker. ADDON-05 closed; three-tier disclosure complete; Phase 53 all 8 plans done.

### Pending Todos

Four pre-existing issues from v2.3 — all addressed as requirements in v2.4:

1. Etcd peer TLS / IP reassignment on pause/resume (→ LIFE-09, Phase 52)
2. cluster-node-skew LB false-positive (→ DIAG-05, Phase 57)
3. cluster-resume-readiness raw JSON dump (→ DIAG-06, Phase 57)
4. allChecks global race under t.Parallel (→ DEBT-04, Phase 56)

### Blockers/Concerns

- **Phase 52 (LIFE-09)**: Docker IPAM static IP feasibility is MEDIUM confidence. Must be verified empirically as first task. Failure triggers cert-regen fallback (not IP pinning). Recommend `/gsd:discuss-phase 52` before planning.
- **Phase 53-02 (ADDON-02)**: RESOLVED — Headlamp v0.42.0 bumped; UAT-2 Path A confirmed. ADDON-02 delivered.
- **Phase 53-03 (ADDON-03)**: RESOLVED — cert-manager v1.20.2 bumped; UAT-3 Path A confirmed. ADDON-03 delivered.
- **Phase 53-04 (ADDON-04)**: RESOLVED — Envoy Gateway v1.7.2 bumped; UAT-4 Path A confirmed. ADDON-04 delivered. Gateway API CRDs at v1.4.1; eg-gateway-helm-certgen Job name verified unchanged.
- **SYNC-05**: Probe ran in Plan 53-00 (2026-05-10) — Outcome B (count=0). DEFERRED. Re-run when kind publishes v1.36 image. Sub-plans 53-01 through 53-07 unblocked.

## Session Continuity

Last session: 2026-05-10T18:00:00Z
Stopped at: Phase 53 ALL 8 PLANS COMPLETE — Plan 53-07 (offlinereadiness consolidation) closed with pass-with-deviation. Commits: 5c61b7f2 (RED), d87f1fa7 (GREEN). ADDON-05 closed. SC1 first clause satisfied; SC1 second-clause wording deviation documented in 53-07-SUMMARY.md. Phase 53 ready for /gsd-verify-work.
Resume file: None
