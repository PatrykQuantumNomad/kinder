---
status: complete
phase: 47-cluster-pause-resume
source: [47-01-SUMMARY.md, 47-02-SUMMARY.md, 47-03-SUMMARY.md, 47-04-SUMMARY.md, 47-05-SUMMARY.md]
started: 2026-05-05T14:07:44Z
updated: 2026-05-05T14:42:00Z
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
result: issue
reported: "kinder get nodes verify47 -> ERROR: unknown command \"verify47\" for \"kinder get nodes\". (kinder get clusters worked and showed Status column with \"Running\".)"
severity: major

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
result: issue
reported: "kinder resume verify47 --wait 5m -> ERROR: invalid argument \"5m\" for \"--wait\" flag: strconv.ParseInt: parsing \"5m\": invalid syntax"
severity: major

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
result: issue
reported: "Healthy 3-CP HA cluster (verify47, all 6 containers running, k8s v1.35.1) and kinder doctor reports cluster-resume-readiness as ⊘ (skip) with reason 'etcdctl unavailable inside container'. The reason text is the pre-47-05 legacy message; new code per 47-05 SUMMARY would emit 'crictl unavailable inside container' or 'etcd container not running'. Also: cluster-node-skew reports '(no cluster found)' on the same healthy cluster — both Cluster-category checks failing to discover the cluster."
severity: major

### 13. cluster-resume-readiness warns on quorum loss (SC4 reverse)
expected: After stopping 2 of 3 CPs on an HA cluster (forcing quorum loss), `kinder doctor` reports `cluster-resume-readiness` with status `warn` and a non-empty `reason` mentioning quorum/unhealthy members. The check NEVER returns fail (warn-and-continue semantics).
result: issue
reported: "After docker stop verify47-control-plane2 verify47-control-plane3 (forcing 2-of-3 CP loss), kinder doctor reports cluster-resume-readiness as ⊘ skip with reason 'single-control-plane cluster; HA check not applicable'. The HA gate is counting RUNNING CPs only, so when CPs are stopped (the exact failure mode quorum-loss detection should catch), the check decides it is not an HA cluster and skips. Combined with test 12's failure, the check cannot trigger in either direction (healthy → skip etcdctl, quorum-loss → skip single-CP). SC4 reverse direction not delivered."
severity: major

### 14. HA pause snapshot captures non-empty leaderID
expected: After `kinder pause` on a 3-CP HA cluster, the file `/kind/pause-snapshot.json` inside the bootstrap CP container contains `{"leaderID": "<non-empty number>", "pauseTime": "<RFC3339>"}`. (Pre-fix this was always empty string; the gap-closure ensures real leader is captured.)
result: issue
reported: "After kinder pause on healthy 3-CP HA verify47, /kind/pause-snapshot.json contains {\"leaderID\":\"\",\"pauseTime\":\"2026-05-05T17:26:23.124014Z\"} — leaderID is empty string. kinder pause stderr also showed: failed to capture etcd leader id for HA pause snapshot (continuing): command \"docker exec --privileged verify47-control-plane etcdctl ...\" failed with error: exit status 127. The legacy docker-exec-etcdctl path is still in use; 47-05 was supposed to replace this with crictl exec but the running binary still uses the broken path."
severity: major

## Summary

total: 14
passed: 9
issues: 5
pending: 0
skipped: 0

## Gaps

- truth: "kinder get nodes <cluster-name> Status column reflects real container state"
  status: failed
  reason: "User reported: kinder get nodes verify47 -> ERROR: unknown command \"verify47\" for \"kinder get nodes\". (kinder get clusters worked and showed Status column with \"Running\".)"
  severity: major
  test: 3
  artifacts: []
  missing: []

- truth: "kinder resume <name> --wait 5m waits up to 5 minutes for all nodes Ready"
  status: failed
  reason: "User reported: kinder resume verify47 --wait 5m -> ERROR: invalid argument \"5m\" for \"--wait\" flag: strconv.ParseInt: parsing \"5m\": invalid syntax. Flag is currently int (seconds) per 47-03 SUMMARY; either flag should accept duration strings (5m/30s) or docs/help should clarify it is integer seconds only."
  severity: major
  test: 9
  artifacts: []
  missing: []

- truth: "cluster-resume-readiness reports ok with '3/3 etcd members healthy' on a healthy 3-CP HA cluster"
  status: failed
  reason: "User reported: Healthy 3-CP HA cluster (verify47, all 6 containers running, k8s v1.35.1) and kinder doctor reports cluster-resume-readiness as skip with reason 'etcdctl unavailable inside container'. Reason text is the pre-47-05 legacy message; new code per 47-05 SUMMARY would emit 'crictl unavailable inside container' or 'etcd container not running'. Also: cluster-node-skew reports '(no cluster found)' on the same healthy cluster — both Cluster-category checks failing to discover the cluster. SC4 (Phase 47 success criterion 4) is not actually delivered."
  severity: major
  test: 12
  artifacts: []
  missing: []

- truth: "cluster-resume-readiness reports warn with reason mentioning quorum/unhealthy members when 2 of 3 CPs are stopped"
  status: failed
  reason: "User reported: After docker stop verify47-control-plane2 verify47-control-plane3 (forcing 2-of-3 CP loss), kinder doctor reports cluster-resume-readiness as skip with reason 'single-control-plane cluster; HA check not applicable'. The HA gate is counting RUNNING CPs only, so when CPs are stopped (the exact failure mode quorum-loss detection should catch), the check decides it is not an HA cluster and skips. Combined with test 12's failure, the check cannot trigger in either direction (healthy -> skip etcdctl, quorum-loss -> skip single-CP). SC4 reverse direction not delivered."
  severity: major
  test: 13
  artifacts: []
  missing: []

- truth: "After kinder pause on a 3-CP HA cluster, /kind/pause-snapshot.json contains a non-empty leaderID"
  status: failed
  reason: "User reported: After kinder pause on healthy 3-CP HA verify47, /kind/pause-snapshot.json contains {\"leaderID\":\"\",\"pauseTime\":\"2026-05-05T17:26:23.124014Z\"} — leaderID is empty string. kinder pause stderr also showed: 'failed to capture etcd leader id for HA pause snapshot (continuing): command \"docker exec --privileged verify47-control-plane etcdctl ...\" failed with error: exit status 127'. The legacy docker-exec-etcdctl path is still in use; 47-05 was supposed to replace this with crictl exec but the running binary still uses the broken path. Same root cause as tests 12/13 — readEtcdLeaderID in pause.go and the doctor check both still call etcdctl directly inside kindest/node rootfs (where it doesn't exist) instead of via crictl exec into the etcd static-pod container."
  severity: major
  test: 14
  artifacts: []
  missing: []
