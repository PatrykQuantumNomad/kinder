---
status: diagnosed
phase: 47-cluster-pause-resume
source: [47-01-SUMMARY.md, 47-02-SUMMARY.md, 47-03-SUMMARY.md, 47-04-SUMMARY.md, 47-05-SUMMARY.md]
started: 2026-05-05T14:07:44Z
updated: 2026-05-05T14:55:00Z
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
  root_cause: "kinder get nodes is declared with cobra.NoArgs (pkg/cmd/kind/get/nodes/nodes.go:63) and only accepts cluster name via --name/-n flag (default \"kind\"). Phase 47-01 changed Status derivation but did NOT relax NoArgs to accept positional args. The new Phase 47 lifecycle commands kinder pause/resume DO take positional args (cobra.MaximumNArgs(1)), creating an inconsistency. Two valid framings: (a) docs error — UAT/SUMMARY should say --name <cluster>, or (b) UX gap — get nodes should match pause/resume positional convention."
  artifacts:
    - path: "pkg/cmd/kind/get/nodes/nodes.go"
      issue: "Line 63: Args: cobra.NoArgs blocks positional cluster name; line 67-70 RunE never reads args[]"
  missing:
    - "Either: docs-only fix updating UAT and 47-01-SUMMARY to specify kinder get nodes --name <cluster> (or -n)"
    - "Or: relax to cobra.MaximumNArgs(1), update Use to 'nodes [cluster-name]', resolve via lifecycle.ResolveClusterName(args, provider) — recommended for parity with pause/resume"
  debug_session: ".planning/debug/get-nodes-positional-arg.md"

- truth: "kinder resume <name> --wait 5m waits up to 5 minutes for all nodes Ready"
  status: failed
  reason: "User reported: kinder resume verify47 --wait 5m -> ERROR: invalid argument \"5m\" for \"--wait\" flag: strconv.ParseInt: parsing \"5m\": invalid syntax. Flag is currently int (seconds) per 47-03 SUMMARY; either flag should accept duration strings (5m/30s) or docs/help should clarify it is integer seconds only."
  severity: major
  test: 9
  root_cause: "resume.go:82-83 and pause.go:79 register --wait and --timeout via IntVar (parses via strconv.ParseInt → '5m' rejected). Should be DurationVar. Three reasons this is a regression not working-as-designed: (1) in-repo precedent: kinder create cluster --wait uses DurationVar at createcluster.go:84-89; (2) 47-RESEARCH.md Open Question #1 explicitly recommended DurationVar with default '5m'; (3) lifecycle.ResumeOptions/PauseOptions internal types are already time.Duration — the IntVar→time.Duration*time.Second conversion at the CLI boundary is artificial. No decision in 47-CONTEXT.md defends int-seconds. The plan files silently downgraded to IntVar with no recorded rationale."
  artifacts:
    - path: "pkg/cmd/kind/resume/resume.go"
      issue: "Lines 42-45 (flagpole int fields), 82-83 (IntVar registration), 89-94 (negative-int validation), 111-112 (manual time.Duration() * time.Second conversion)"
    - path: "pkg/cmd/kind/pause/pause.go"
      issue: "Line 43 (flagpole.Timeout int), 79 (IntVar), 85-87 (validation), 104 (conversion) — same defect for --timeout"
    - path: "pkg/cmd/kind/resume/resume_test.go"
      issue: "Lines 238, 282 — test args '--wait=600' / '--wait=-1' will need updating to duration strings"
  missing:
    - "Change flagpole.Timeout/WaitSecs from int to time.Duration in both pause.go and resume.go"
    - "Replace IntVar with DurationVar(..., 30*time.Second, ...) and DurationVar(..., 5*time.Minute, ...)"
    - "Drop the time.Duration(flags.X) * time.Second conversions at lifecycle call sites"
    - "Switch negative-value guards to compare against time.Duration(0)"
    - "Update test args to duration strings ('--wait=600s', '--wait=-1s')"
  debug_session: ".planning/debug/resume-wait-int-vs-duration.md"

- truth: "cluster-resume-readiness reports ok with '3/3 etcd members healthy' on a healthy 3-CP HA cluster"
  status: failed
  reason: "User reported: Healthy 3-CP HA cluster (verify47, all 6 containers running, k8s v1.35.1) and kinder doctor reports cluster-resume-readiness as skip with reason 'etcdctl unavailable inside container'. Reason text is the pre-47-05 legacy message; new code per 47-05 SUMMARY would emit 'crictl unavailable inside container' or 'etcd container not running'. Also: cluster-node-skew reports '(no cluster found)' on the same healthy cluster — both Cluster-category checks failing to discover the cluster. SC4 (Phase 47 success criterion 4) is not actually delivered."
  severity: major
  test: 12
  root_cause: "TWO distinct causes. (1) Stale local build: bin/kinder built May 5 08:40, ~50s before 47-04 fix b4327d99 (08:41:43) and ~70min before 47-05 a54de41c (09:50:36). strings(bin/kinder) confirms it contains the legacy 'etcdctl unavailable inside container' literal but NOT the new 'crictl unavailable inside container' or 'etcd container not running' strings. Source on disk (resumereadiness.go:116-138) is correct — uses crictl ps + crictl exec exactly as 47-05 specified. (2) Genuine source bug in clusterskew.go:77 — hardcodes filter 'label=io.x-k8s.kind.cluster=kind', pinning discovery to the default cluster name 'kind'. On verify47 this returns zero rows and check skips with '(no cluster found)'. Origin: commit afbfb7d7d (2026-04-08, phase 42) — predates phase 47. Reproduces against fresh binary. Companion check resumereadiness.go:294 uses presence-only 'label=io.x-k8s.kind.cluster' which works correctly."
  artifacts:
    - path: "bin/kinder"
      issue: "Stale build (May 5 08:40), predates 47-04 (08:41:43) and 47-05 (09:50:36) commits"
    - path: "pkg/internal/doctor/clusterskew.go"
      issue: "Line 77 hardcodes 'label=io.x-k8s.kind.cluster=kind' instead of presence-only 'label=io.x-k8s.kind.cluster' — breaks discovery for any cluster not named 'kind'"
  missing:
    - "Rebuild bin/kinder from current HEAD (go build -o bin/kinder ./cmd/kinder)"
    - "Change clusterskew.go:77 filter from 'label=io.x-k8s.kind.cluster=kind' to 'label=io.x-k8s.kind.cluster' (presence-only)"
    - "Add clusterskew_test.go case using a non-default cluster name to lock in the fix"
  debug_session: ".planning/debug/doctor-cluster-discovery-and-etcdctl-message.md"

- truth: "cluster-resume-readiness reports warn with reason mentioning quorum/unhealthy members when 2 of 3 CPs are stopped"
  status: failed
  reason: "User reported: After docker stop verify47-control-plane2 verify47-control-plane3 (forcing 2-of-3 CP loss), kinder doctor reports cluster-resume-readiness as skip with reason 'single-control-plane cluster; HA check not applicable'. The HA gate is counting RUNNING CPs only, so when CPs are stopped (the exact failure mode quorum-loss detection should catch), the check decides it is not an HA cluster and skips. Combined with test 12's failure, the check cannot trigger in either direction (healthy -> skip etcdctl, quorum-loss -> skip single-CP). SC4 reverse direction not delivered."
  severity: major
  test: 13
  root_cause: "realListCPNodes in resumereadiness.go:291-295 enumerates control-plane containers via plain 'docker ps' (no -a flag), so stopped containers are silently omitted from cpNodeNames. The HA gate at line 104 (if len(cpNodeNames) <= 1) then treats the cluster as single-CP and skips with 'single-control-plane cluster; HA check not applicable'. The check's HA detection reflects RUNTIME state (CPs alive), not DECLARED TOPOLOGY (CPs the cluster was created with). In the exact failure mode the check exists to detect, it bails out before any etcdctl probe. Kind's own provider.ListNodes (provider.go:114-134) uses '-a' to enumerate stopped nodes — this is the canonical pattern used by kinder get nodes, kinder status, lifecycle.Pause, lifecycle.Resume. Test resumereadiness_test.go has no integration test covering HA-with-stopped-CPs; all tests inject cpNodeNames directly via fakeReadinessOpts."
  artifacts:
    - path: "pkg/internal/doctor/resumereadiness.go"
      issue: "Lines 291-295: realListCPNodes uses plain 'docker ps' instead of 'docker ps -a' — omits stopped containers from CP enumeration"
    - path: "pkg/internal/doctor/resumereadiness.go"
      issue: "Line 113: bootstrap = cpNodeNames[0] picks alphabetically-first CP, which after fix could be a stopped container; should pick first CP whose ContainerState returns 'running' via lifecycle.ContainerState"
    - path: "pkg/internal/doctor/resumereadiness_test.go"
      issue: "No test exercises realListCPNodes against stopped containers; gap allowed bug to escape"
  missing:
    - "Add '-a' flag to exec.Command args in realListCPNodes (single-line minimal fix)"
    - "After HA gate passes, select bootstrap as first cpNodeNames[i] whose lifecycle.ContainerState returns 'running' — fall back to warn 'all CPs stopped; cannot probe etcd' if none running"
    - "Add test: cpNodeNames=[cp1,cp2,cp3] with etcd health probe returning 1/3 healthy → assert warn with 'quorum at risk'"
    - "Add unit-level assertion that realListCPNodes constructed command line contains '-a'"
  debug_session: ".planning/debug/ha-gate-counts-running-cps.md"

- truth: "After kinder pause on a 3-CP HA cluster, /kind/pause-snapshot.json contains a non-empty leaderID"
  status: failed
  reason: "User reported: After kinder pause on healthy 3-CP HA verify47, /kind/pause-snapshot.json contains {\"leaderID\":\"\",\"pauseTime\":\"2026-05-05T17:26:23.124014Z\"} — leaderID is empty string. kinder pause stderr also showed: 'failed to capture etcd leader id for HA pause snapshot (continuing): command \"docker exec --privileged verify47-control-plane etcdctl ...\" failed with error: exit status 127'. The legacy docker-exec-etcdctl path is still in use; 47-05 was supposed to replace this with crictl exec but the running binary still uses the broken path."
  severity: major
  test: 14
  root_cause: "STALE BINARY ONLY — no source regression. pkg/internal/lifecycle/pause.go:257-294 at HEAD already uses the correct crictl path (crictl ps --name etcd -q + crictl exec <id> etcdctl). No legacy 'docker exec ... etcdctl' path exists anywhere in pkg/ or cmd/. The user's stderr is the wrapped form of bootstrap.Command('etcdctl', ...) from the 47-02 candidate-loop (commit ac8c1a16, replaced by 0c612a54). Two stale artifacts: (1) /opt/homebrew/bin/kinder → /opt/homebrew/Caskroom/kinder/1.4/kinder built Apr 11 2026 (sealed pre-built; will never reflect repo state). (2) /Users/patrykattc/work/git/kinder/bin/kinder built May 5 08:40, 74 minutes before 47-05 commit 0c612a54 (May 5 09:54). Same root cause covers tests 12/14 — both verify behavior added in 47-04/47-05; they will start passing simultaneously after one rebuild. NO source change needed for pause.go."
  artifacts:
    - path: "bin/kinder"
      issue: "Built May 5 08:40 — predates 47-05 commit 0c612a54 (May 5 09:54) by 74 minutes; user invoked this binary explicitly"
    - path: "/opt/homebrew/bin/kinder"
      issue: "Symlinks to /opt/homebrew/Caskroom/kinder/1.4/kinder (Apr 11 2026 sealed Cask install); needs symlink replacement to local build"
  missing:
    - "Rebuild local binary: go build -o bin/kinder ./cmd/kinder (handled in same step as test 12 fix)"
    - "Replace Homebrew symlink: install bin/kinder /opt/homebrew/bin/kinder (so `which kinder` resolves to fresh build)"
    - "Verification: strings $(which kinder) | grep 'crictl ps --name etcd' must match; grep '/usr/local/bin/etcdctl' must NOT match"
  debug_session: ".planning/debug/pause-snapshot-leaderid-empty.md"
