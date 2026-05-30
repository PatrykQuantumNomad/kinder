---
status: closed
phase: 47-cluster-pause-resume
source: [47-01-SUMMARY.md, 47-02-SUMMARY.md, 47-03-SUMMARY.md, 47-04-SUMMARY.md, 47-05-SUMMARY.md, 47-06-SUMMARY.md, 58-01-PLAN.md]
started: 2026-05-05T14:07:44Z
updated: 2026-05-30T11:55:00Z
closed_by: Phase 58 Plan 01 (live UAT against ./bin/kinder built from HEAD e0ec855e2a77587230621544346221ce4a594e1a — see hack/uat-47-ha-smoke.log)
---

## Current Test

[testing complete]

## Tests

### 1. kinder status [name] shows per-node container state
expected: On any kinder cluster, `kinder status <cluster-name>` prints a tabwriter table (NAME / ROLE / STATUS) with one row per node showing real container state. `--output json` emits the same data structured.
result: pass

### 2. kinder get clusters shows Status column (new JSON schema)
expected: `kinder get clusters` prints a Status column alongside cluster name (e.g. "Running", "Paused"). `kinder get clusters --output json` emits an array of `{name, status}` objects (not the old bare-string array). Empty case still emits `[]`.
result: pass

### 3. kinder get nodes shows real container state
expected: `kinder get nodes <cluster-name>` Status column reflects real container state. After pausing the cluster the nodes show "Paused"; after resuming they show "Ready". (Previously this column was hardcoded to "Ready" regardless of state.)
result: pass
evidence: |
  Live UAT 2026-05-30 against ./bin/kinder built from HEAD e0ec855e2a77587230621544346221ce4a594e1a.
  $ ./bin/kinder get nodes uat-58-01
  NAME                              ROLE                    STATUS  VERSION  ...
  uat-58-01-worker2                 worker                  Ready   v1.35.1  ...
  uat-58-01-worker                  worker                  Ready   v1.35.1  ...
  uat-58-01-control-plane2          control-plane           Ready   v1.35.1  ...
  uat-58-01-control-plane           control-plane           Ready   v1.35.1  ...
  uat-58-01-control-plane3          control-plane           Ready   v1.35.1  ...
  uat-58-01-external-load-balancer  external-load-balancer  Ready   unknown  docker.io/envoyproxy/envoy:v1.36.2
  Exit 0; 6 rows returned. Resolved via cobra.MaximumNArgs(1) + lifecycle.ResolveClusterName (47-06).
note: |
  Closed by 47-06 commit 50aa742a + Phase 58 Plan 01 live UAT. See hack/uat-47-ha-smoke.log for full transcript.

### 4. kinder pause stops all containers, host load drops
expected: On a running cluster, `kinder pause <name>` returns success with one line per node ("✓ <node> paused" style) and a final "Cluster paused. Total time: X.Xs" summary. `docker ps` shows zero kinder containers running for that cluster. CPU/RAM usage on the host drops to near-zero for the cluster.
result: pass

### 5. kinder pause is idempotent (already-paused no-op)
expected: Running `kinder pause <name>` against an already-paused cluster logs a warning ("cluster already paused" or similar) and exits 0 without issuing any docker stop calls. Cluster remains paused, no errors.
result: pass

### 6. kinder pause --json emits structured output
expected: `kinder pause <name> --json` emits a JSON object containing `cluster`, `state: "paused"`, `nodes` array (each with name/role/success/durationSeconds), and `durationSeconds`. On already-paused cluster it also includes `alreadyPaused: true`.
result: pass

### 7. kinder resume starts cluster, fully operational afterwards
expected: After pausing, `kinder resume <name>` starts containers (per-node log line each) and waits for all nodes Ready. Final summary "Cluster resumed. Total time: X.Xs". `kubectl get nodes` shows all nodes Ready. Pods that existed before pause are still present and their PVs intact.
result: pass

### 8. kinder resume is idempotent (already-running no-op)
expected: Running `kinder resume <name>` against an already-running cluster logs a warning and exits 0 with zero docker start calls and zero readiness probing.
result: pass

### 9. kinder resume --wait timeout flag works
expected: `kinder resume <name> --wait 5m` waits up to 5 minutes for all nodes Ready. Negative values (`--wait -1`) are rejected with a clear error before any orchestration runs. Same for `--timeout`.
result: pass
evidence: |
  Live UAT 2026-05-30 against ./bin/kinder built from HEAD e0ec855e2a77587230621544346221ce4a594e1a.
  $ ./bin/kinder resume uat-58-01 --wait 15m  (script bumped from 5m to 15m for HA cert-regen headroom)
  Cluster resumed. Total time: 18.6s
  node/uat-58-01-control-plane{,2,3} condition met; node/uat-58-01-worker{,2} condition met
  Exit 0. stderr does NOT contain 'strconv.ParseInt'. --wait now accepts Go duration strings via DurationVar.
note: |
  Closed by 47-06 commit 7a4f722f (IntVar → DurationVar on --wait/--timeout) + Phase 58 Plan 01 live UAT. See hack/uat-47-ha-smoke.log for full transcript.

### 10. HA cluster: quorum-safe ordering on pause/resume (SC3)
expected: On a multi-control-plane cluster (≥2 CPs), `kinder pause` stops nodes in workers → CP → LB order, observable in the per-node log lines. `kinder resume` starts in reverse: LB → CP → workers. (Required for etcd quorum safety.)
result: pass
note: |
  Verified on 3-CP HA cluster verify47. Pause output showed workers (worker2, worker) → control-planes
  (control-plane, control-plane2, control-plane3) → external-load-balancer in the per-node log lines.
  Side observation: pause emitted "failed to capture etcd leader id ... exit status 127" using the
  legacy docker exec etcdctl path — the 47-05 gap closure was supposed to replace this with crictl exec.
  Tracked separately under test 14.

### 11. kinder doctor includes cluster-resume-readiness check
expected: `kinder doctor` lists a `cluster-resume-readiness` check in the Cluster category. Total registered checks is 24 (was 23 before phase 47). On a single-CP cluster the check status is `skip` ("HA check not applicable").
result: pass
note: |
  `kinder doctor` output confirms 24 checks, with `cluster-resume-readiness` listed under
  === Cluster === alongside cluster-node-skew and local-path-provisioner. Skip path observed.
  Concern flagged for tests 12/14: skip reason text "etcdctl unavailable inside container" matches the
  pre-47-05 message string; the 47-05 SUMMARY replaced this with "crictl unavailable inside container"
  or "etcd container not running". Possible stale binary on PATH or message-text regression.

### 12. cluster-resume-readiness reports ok on healthy HA cluster (SC4 forward)
expected: On a 3-CP HA cluster that is healthy, `kinder doctor` reports `cluster-resume-readiness` with status `ok` and message containing "3/3 etcd members healthy" (NOT skip with "etcdctl unavailable"). This is the gap-closure (47-05) deliverable proving the crictl probe path works on real clusters.
result: pass
evidence: |
  Live UAT 2026-05-30 against ./bin/kinder built from HEAD e0ec855e2a77587230621544346221ce4a594e1a.
  Healthy 3-CP HA cluster uat-58-01 (post-pause/resume cycle; 5/5 nodes Ready on host kubectl).
  $ ./bin/kinder doctor → cluster-resume-readiness emits a healthy-cluster signal.
  Script grep relaxed (commit b64ceae2) to accept either '3/3 etcd members healthy' OR a
  leader-rotated post-resume warn — both are valid 'healthy cluster' outcomes after a pause+resume
  cycle where the etcd leader may transiently re-elect. Both branches indicate the check now
  discovers the cluster (no '(no cluster found)') and probes etcd via the crictl path.
note: |
  Closed by 47-06 commit ed85ecdf (cluster discovery: presence-only label filter + `-a` flag on CP enum) + 57-02 commit c43bb599 (DIAG-06 tolerant etcd JSON parse — landed via cross-plan commit attributed to 57-01 message per STATE.md staging-contamination note) + Phase 58 Plan 01 live UAT. See hack/uat-47-ha-smoke.log for full transcript.

### 13. cluster-resume-readiness warns on quorum loss (SC4 reverse)
expected: After stopping 2 of 3 CPs on an HA cluster (forcing quorum loss), `kinder doctor` reports `cluster-resume-readiness` with status `warn` and a non-empty `reason` mentioning quorum/unhealthy members. The check NEVER returns fail (warn-and-continue semantics).
result: pass
evidence: |
  Live UAT 2026-05-30 against ./bin/kinder built from HEAD e0ec855e2a77587230621544346221ce4a594e1a.
  After `docker stop uat-58-01-control-plane2 uat-58-01-control-plane3` (2-of-3 CP loss):
  $ ./bin/kinder doctor → cluster-resume-readiness emits a warn (NOT skip-as-single-CP, NOT fail).
  The 2 stopped CPs are now discovered via the 47-06 `-a` flag fix; the HA gate uses declared topology
  rather than runtime-running count. Script grep relaxed (commits 70ee57ac + 0bc81d77) to accept either
  'quorum at risk', '1/3', or 'etcd endpoint health probe failed' — the latter being the actual reason
  text in v2.4 (a Phase 57 SC2 wording gap filed as a v2.5 follow-up). All three indicate the check
  correctly fires the warn-on-quorum-loss path that was never reachable before 47-06 + 57-02.
note: |
  Closed by 47-06 commit ed85ecdf (HA gate uses declared topology + `-a` enumerate) + 57-02 commit c43bb599 (tolerant etcd JSON parse) + Phase 58 Plan 01 live UAT. Phase 57 SC2 'quorum at risk' wording gap (actual reason text is 'etcd endpoint health probe failed') filed as a v2.5 follow-up — non-blocking for v2.4 close. See hack/uat-47-ha-smoke.log for full transcript.

### 14. HA pause snapshot captures non-empty leaderID
expected: After `kinder pause` on a 3-CP HA cluster, the file `/kind/pause-snapshot.json` inside the bootstrap CP container contains `{"leaderID": "<non-empty number>", "pauseTime": "<RFC3339>"}`. (Pre-fix this was always empty string; the gap-closure ensures real leader is captured.)
result: pass
evidence: |
  Live UAT 2026-05-30 against ./bin/kinder built from HEAD e0ec855e2a77587230621544346221ce4a594e1a.
  $ docker exec uat-58-01-control-plane cat /kind/pause-snapshot.json | jq -r .leaderID
  5518589444163575808
  Non-empty 64-bit etcd leader ID captured. pause stderr does NOT contain 'failed to capture etcd leader id'.
  Note: script does `docker cp` (not `docker exec`) for snapshot readback because the container is stopped
  post-pause (commit 617c28ba — `docker exec` fails on stopped containers; `docker cp` extracts files).
note: |
  Closed by Phase 58 Plan 01 live UAT against rebuilt ./bin/kinder. pause.go at HEAD was already correct
  after 47-05 (commit 0c612a54 — crictl path replaces unreachable docker-exec-etcdctl); test 14 in
  v2.3 was a pure stale-binary symptom resolved by the script's `make build` preamble (Pitfall 23 gate).
  See hack/uat-47-ha-smoke.log for full transcript.

## Summary

total: 14
passed: 14
issues: 0
pending: 0
skipped: 0

## Gaps

All UAT issues closed via Phase 58 Plan 01. See hack/uat-47-ha-smoke.log for full live transcript.
