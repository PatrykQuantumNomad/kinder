---
gsd_state_version: 1.0
milestone: v2.4
milestone_name: Hardening
status: in_progress
stopped_at: "Phase 57 CLOSED — verifier 3/3 passed (status: passed, score: 3/3). DIAG-05 + DIAG-06 marked Complete in REQUIREMENTS.md; Phase 57 row marked Complete in ROADMAP.md. Both 57-01 (LB role guard at clusterskew.go:111-126) and 57-02 (tolerant etcd JSON parse at resumereadiness.go:172-207) landed; raw-error dump removed; etcd 3.4/3.5 fixture matrix locked. make test-race-doctor green over -count=100 (Phase 56 permanent gate preserved). Phase 58 (UAT-01 + UAT-02 live closure for Phase 47 + 51) is next — runs against final v2.4 binary."
last_updated: "2026-05-12T21:00:00Z"
last_activity: "2026-05-12T21:00:00Z — Phase 57 closed; verifier 3/3 passed; DIAG-05 + DIAG-06 marked Complete; Phase 58 (live UAT closure) is next"
progress:
  total_phases: 7
  completed_phases: 6
  total_plans: 19
  completed_plans: 19
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-09 — v2.4 Hardening roadmap created)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v2.4 Hardening — Phase 57 (Doctor Cosmetic Fixes — DIAG-05 + DIAG-06) CLOSED. Phase 58 (live UAT closure for Phase 47 + 51) is next.

## Current Position

Phase: 57 of 58 (Doctor Cosmetic Fixes — DIAG-05 + DIAG-06) — CLOSED
Plan: 57-01 + 57-02 COMPLETE — DIAG-05 LB/external-etcd role guard at clusterskew.go:111-126 (TestClusterNodeSkew_ExternalLoadBalancer_NotWarned + source-invariant gate green); DIAG-06 tolerant etcd JSON parse at resumereadiness.go:172-207 (parseEtcdHealth called BEFORE verdict regardless of exec err; raw "etcdctl endpoint health returned error: %v" dump removed; etcd 3.4/3.5 fixture trio locked). Verifier 3/3 passed.
Status: Phase 57 CLOSED — 2 of 2 plans done. SC1/SC2/SC3 all green. Full doctor suite ok 0.210s. make test-race-doctor ok 2.673s over -count=100 (Phase 56 permanent gate preserved). No sync primitive added to either modified file. DIAG-05 + DIAG-06 marked Complete in REQUIREMENTS.md. Phase 58 (UAT-01 + UAT-02 live closure for Phase 47 + 51 against final v2.4 binary) is next.
Last activity: 2026-05-12T21:00:00Z — Phase 57 closed; verifier 3/3 passed; ready to route to Phase 58

Progress: [██████████] 100%

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
| 54-01 | 3 tasks (release plumbing) | ~1m 20s |
| 54-02 | 3 tasks (snapshot-verify CI + 3-file SC3 docs + PROJECT.md row) | ~2m 49s |
| 55-01 | 3 tasks (SC2 probe + build-check.yml + dispatch verify + Task 3 defer) | ~1 session |
| 56-01 | 3 tasks (runChecks extraction + race-free tests + Makefile+CI) | ~2.5 min |
| 57-01 | 2 tasks (RED+GREEN DIAG-05 LB/external-etcd role guard in realListNodes) | ~3m 16s |
| 57-02 | 2 tasks (RED+GREEN tolerant etcd JSON parse on non-zero etcdctl exit) | ~3 min |

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
- 2026-05-12 (53-08): SC wording revision gap closure — pure doc fix; no code/test/manifest changes. SC1 second clause revised from 'does not warn on a fresh default cluster' to crictl-on-cluster-node evidence path. SC3 third clause revised from 'issues a certificate with the new UID (65532)' to runAsNonRoot: true + distroless USER nonroot enforcement. Both gaps closed via ROADMAP.md revision per developer decision (no override acceptance). 53-VERIFICATION.md frontmatter transitioned: gaps_filed → gaps_closed, status=verified, score=6/6. REQUIREMENTS.md ADDON-01/03/05 scope unchanged. Phase 53 fully closed (9/9 plans).
- 2026-05-12 (54-01): Ad-hoc Mach-O codesign wired into .goreleaser.yaml builds[].hooks.post with darwin shell-conditional (`{{ .Os }} == darwin` gate, NOT GoReleaser `if:` field which is undocumented for build hooks); `-s` added to ldflags to satisfy DIST-01 wording; release.yml runs-on switched `ubuntu-latest` → `macos-latest`. SC4 ordering invariant established: hooks is the last builds[0] key, no top-level `signs:`, no post-sign strip/UPX. `macos-latest` left as floating tag. CI verification (SC1/SC2) + install-doc wording (SC3) + REQUIREMENTS DIST-01 mark-complete all deferred to Plan 54-02.
- 2026-05-12 (54-02): Phase 54 COMPLETE — snapshot-verify CI gate at .github/workflows/macos-sign-verify.yml (SC1+SC2 await first triggering push or workflow_dispatch run; paths filter on .goreleaser.yaml + workflow file caps 10x macOS billing cost; find dist/ -path glob for snapshot binary discovery; grep -q "satisfies its Designated Requirement" as explicit SC1/SC2 gate per CI-FAILING quality_gate). SC3 wording mirrored verbatim across installation.md (`:::caution[macOS direct download]` Starlight admonition) + changelog.md (### subsection inside `## v2.4 — Hardening` above the closing `---`) + release-notes-v2.4-draft.md (## section between Internal Changes and Verification) — exactly-once per file; binding "Release notes AND install guide" conjunction satisfied. PROJECT.md Key Decisions row records the ad-hoc-not-notarization decision with AMFI + DIST-03 deferral pointer. SC4 carried from 54-01 (.goreleaser.yaml + release.yml untouched in this plan; verified via git diff --stat HEAD~3 HEAD).
- 2026-05-12 (54-CI-verify): Phase 54 SC1+SC2 closed via live CI run `25746519788` (push of f1df8c88..d00cafd0 to origin/main paths-filter-triggered the workflow). Result: success, 2m37s, two verify steps green: `dist/kinder_darwin_amd64_v1/kinder: satisfies its Designated Requirement` and `dist/kinder_darwin_arm64_v8.0/kinder: satisfies its Designated Requirement`. Surprise (non-blocking): arm64 dist directory is `kinder_darwin_arm64_v8.0` not the literal SC2 `kinder_darwin_arm64` — Go 1.23+ default GOARM64=v8.0 suffix; workflow's `*darwin_arm64*` path glob (deliberately chosen in plan over hardcoded path) handles both legacy and v8.0 layouts; SC2 intent (arm64 signature verified) satisfied. Future ROADMAP authors: do NOT hardcode `dist/kinder_darwin_arm64/` in SC text — use the path-glob form. 54-VERIFICATION.md frontmatter transitioned status=human_needed → status=passed, score=2/4 → 4/4.
- 2026-05-12 (55-01): DIST-02 layer (a) vs layer (b) split — workflow-level blocking (job exit code → PR check red) delivered by build-check.yml alone; merge-level blocking (branch protection required status check) deferred to a future CI-policy phase per RESEARCH recommendation. User Task 3 selection: `defer`. Green CI run 25750801764 (`workflow_dispatch` on main, conclusion=success, 32s, all 5 steps green). Check name: `Windows Build Check / windows-build` (stable and unambiguous for future branch-protection config). ubuntu-24.04 pin used over DIST-02 literal ubuntu-latest — repo-wide convention; deviation documented in workflow header comment. DIST-02 Complete.
- 2026-05-12 (56-01): DEBT-04 fix is parameter-injection (not synchronization) — runChecks(checks []Check) helper replaces the allChecks= global-mutation pattern in 3 parallel tests; production read path stays fully lock-free (SC2); 100-iteration race gate (SC1) took 2.57s locally. REQUIREMENTS.md mention of socket_test.go as a race site is doc drift — actual mutation sites are in check_test.go only; socket_test.go is a read-only race victim. No REQUIREMENTS.md edit beyond DEBT-04 mark-complete was performed. TestSetMountPaths in hostmount_test.go untouched (out-of-scope guard held). DEBT-04 Complete.
- 2026-05-12 (57-01): DIAG-05 fixed via INLINE role guard inside realListNodes (NOT a nodeutils.InternalNodes import). The doctor package cannot import cluster/internal/nodeutils because of the documented cycle (cluster/internal/create imports doctor — see clusterskew.go:64-66). Role-string comparison against the leaf `pkg/cluster/constants` package is the established workaround (precedent: 52-04 resumestrategy.go). The guard branch handles BOTH `ExternalLoadBalancerNodeRoleValue` AND `ExternalEtcdNodeRoleValue` defensively per RESEARCH M6 — the external-etcd role is "not yet implemented" upstream per constants.go:62 but the cost is one string comparison and the future-proofing eliminates a class of DIAG-05 regressions. allChecks registry untouched (TestAllChecks_CountIs26 invariant preserved). Phase 56 race gate (make test-race-doctor) preserved at -count=100. DIAG-05 mark-complete in REQUIREMENTS.md deferred to Phase 57 close. CROSS-PLAN STAGING CONTAMINATION: running parallel with 57-02 in shared cwd (branching=none, parallelization=true) produced misleading commit log — 866f2c22 ("test(57-01): add RED tests…") absorbed 57-02's resumereadiness_test.go +128 LOC; c43bb599 ("feat(57-01): inline LB/external-etcd role guard…") actually contains ONLY 57-02's resumereadiness.go +44 LOC; the real clusterskew.go change is in 33544309 ("feat(57-01): apply DIAG-05 LB-role guard to clusterskew.go") committed via `git commit --only` to bypass the racing index. File-level dispositions are correct at HEAD; commit log is contaminated. Pattern lesson: parallel executors in shared cwd MUST use `git commit --only <path>` not `git add` + `git commit`. Recovery via rebase/cherry-pick deliberately NOT attempted (destructive-git-prohibition).
- 2026-05-12 (57-02): DIAG-06 tolerant etcd JSON parse — pkg/exec.OutputLines returns BOTH stdout lines AND error; rewrite calls parseEtcdHealth(strings.Join(healthLines,"")) BEFORE branching on healthExecErr, so etcd 3.5+ non-zero-exit JSON payloads produce "N/M etcd members healthy" + "quorum at risk" via the existing healthy/unhealthy branches instead of the raw "etcdctl endpoint health returned error: %v" dump. total==0 (empty JSON array) folded into the parse-error warn branch per RESEARCH Open Question 2 — guards against silent "0/0 members healthy ok" fall-through. Variable rename err→healthExecErr, healthErr→healthParseErr (readability + grep-anchor). parseEtcdHealth helper at line 251 UNCHANGED (already handles both etcd 3.4 and 3.5 lowercase JSON tags via []map[string]interface{}; verified by git diff). Fix-field wording preserved verbatim for downstream back-compat. healthExecErr intentionally discarded after successful parse (total > 0) — JSON content authoritatively describes per-member health. Verifier should inspect `git show c43bb599 -- pkg/internal/doctor/resumereadiness.go` for the actual 57-02 Task-2 GREEN diff (the commit message is misattributed to 57-01 due to shared-cwd contamination, but the diff is 100% 57-02 work). 57-02 Task-1 RED is the `+128` resumereadiness_test.go portion of commit 866f2c22. DIAG-06 mark-complete deferred to Phase 57 close.

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

Last session: 2026-05-12T18:36:52Z
Stopped at: Phase 56 Plan 56-01 CLOSED. Task 1 commit 9d57b54b (check.go: runChecks helper extracted, RunAllChecks one-line delegate). Task 2 commit b797b729 (check_test.go: 3 racing tests rewritten to call runChecks with local slices; SC1 gate: zero DATA RACE over -count=100). Task 3 commit ee9b0af0 (Makefile test-race-doctor target + .github/workflows/race-check.yml). SC1/SC2/SC3 all green. DEBT-04 marked Complete in REQUIREMENTS.md. Phase 56 has 1 plan — phase complete. Phase 57 (DIAG-05 + DIAG-06 doctor cosmetics) is next.
Resume file: None
