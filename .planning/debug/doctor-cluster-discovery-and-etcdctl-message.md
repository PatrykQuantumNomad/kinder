---
status: diagnosed
trigger: "doctor-cluster-discovery-and-etcdctl-message: kinder doctor reports cluster-resume-readiness skip with legacy 'etcdctl unavailable inside container' message and cluster-node-skew reports '(no cluster found)' on a healthy 3-CP HA cluster"
created: 2026-05-05T00:00:00Z
updated: 2026-05-05T12:30:00Z
---

## Current Focus

hypothesis: Two distinct root causes — (1) stale local build of bin/kinder predates 47-05, (2) clusterskew.go has hardcoded `=kind` cluster-name filter
test: Compare strings in bin/kinder vs source on disk; compare container-discovery filters across all three Cluster-category checks
expecting: Confirmed
next_action: Report ROOT CAUSE FOUND

## Symptoms

expected: On a 3-CP HA cluster that is healthy, `kinder doctor` reports `cluster-resume-readiness` with status `ok` and message containing "3/3 etcd members healthy" — using the crictl-exec probe path introduced by 47-05.
actual: On healthy 3-CP HA cluster verify47 (all 6 containers running, k8s v1.35.1), `kinder doctor` reports:
  - cluster-resume-readiness "etcdctl unavailable inside container" (legacy message text)
  - cluster-node-skew "(no cluster found)" (cluster IS running)
  - local-path-provisioner correctly detects cluster
errors: None — graceful skip
reproduction: Test 12 in .planning/phases/47-cluster-pause-resume/47-UAT.md. On 3-CP HA cluster verify47, run kinder doctor
started: 2026-05-05 during UAT

## Eliminated

- hypothesis: "Source-code regression — 47-05 changes were partial, leaving legacy etcdctl probe in resumereadiness.go"
  evidence: "Direct read of pkg/internal/doctor/resumereadiness.go on disk shows full crictl-based implementation. Skip messages are 'crictl unavailable inside container' (line 122) and 'etcd container not running' (line 137). Legacy string 'etcdctl unavailable inside container' does not appear anywhere in pkg/ source. grep confirmed."
  timestamp: 2026-05-05T12:00:00Z

## Evidence

- timestamp: 2026-05-05T12:05:00Z
  checked: pkg/internal/doctor/resumereadiness.go full file
  found: Source uses crictl ps + crictl exec etcdctl. Skip paths are 'no kind cluster detected', 'single-control-plane cluster; HA check not applicable', 'crictl unavailable inside container' (line 122), 'etcd container not running' (line 137). No legacy etcdctl-direct probe path exists.
  implication: 47-05 implementation is complete and correct in source.

- timestamp: 2026-05-05T12:08:00Z
  checked: grep for 'etcdctl unavailable inside container' across pkg/
  found: Zero matches in source for the literal as a string value; line 88 of resumereadiness.go is a doc-comment using the same words descriptively. Symbol literally absent from compiled-from-source code paths.
  implication: User-visible legacy message must come from a binary that contains older source.

- timestamp: 2026-05-05T12:10:00Z
  checked: pkg/internal/doctor/clusterskew.go realListNodes filter (line 77)
  found: docker ps filter is hardcoded as 'label=io.x-k8s.kind.cluster=kind' — pins cluster name to 'kind', the default. Blame shows commit afbfb7d7d (phase 42, 2026-04-08) — this hardcode predates phase 47 entirely.
  implication: cluster-node-skew can only discover the default cluster; on cluster 'verify47' it returns zero containers and check returns 'skip (no cluster found)'. Independent bug from the resume-readiness symptom.

- timestamp: 2026-05-05T12:11:00Z
  checked: pkg/internal/doctor/resumereadiness.go realListCPNodes filter (line 294)
  found: docker ps filter is 'label=io.x-k8s.kind.cluster' — presence-only, no value pin. Works for any cluster name.
  implication: resumereadiness.go discovery path is correct for cluster verify47.

- timestamp: 2026-05-05T12:12:00Z
  checked: pkg/internal/doctor/localpath.go realGetProvisionerVersion filter (line 70)
  found: docker ps filter is 'label=io.x-k8s.kind.role=control-plane' — pins role only, not cluster name. Works for any cluster name.
  implication: local-path-cve discovery path is correct for cluster verify47, explaining why that check correctly detected the cluster while cluster-node-skew did not.

- timestamp: 2026-05-05T12:15:00Z
  checked: which kinder; ls -la /opt/homebrew/bin/kinder; strings on Homebrew binary
  found: /opt/homebrew/bin/kinder → /opt/homebrew/Caskroom/kinder/1.4/kinder, dated Apr 11 2026, version 'kind v1.4.0'. strings returned ZERO matches for 'cluster-resume-readiness', 'etcdctl unavailable', 'crictl unavailable', or 'etcd container not running'.
  implication: Homebrew binary is from tag v1.4 (commit e4e08c6a) which predates phase 47 entirely; it does NOT contain the resume-readiness check at all. So the symptom output cannot have come from /opt/homebrew/bin/kinder.

- timestamp: 2026-05-05T12:18:00Z
  checked: stat -f /Users/patrykattc/work/git/kinder/bin/kinder; git log timestamps
  found: Local build dated 'May 5 08:40:53 2026'. 47-04 label fix (b4327d99) committed at 2026-05-05 08:41:43 (50 seconds AFTER bin/kinder was built). 47-05 crictl-exec (a54de41c) committed at 2026-05-05 09:50:36 (~70 minutes after bin/kinder was built).
  implication: bin/kinder is a snapshot of source state immediately BEFORE both 47-04's label-template fix and 47-05's crictl rewrite.

- timestamp: 2026-05-05T12:20:00Z
  checked: strings /Users/patrykattc/work/git/kinder/bin/kinder | grep doctor-related literals
  found: Contains 'cluster-resume-readiness', 'no kind cluster detected', 'etcdctl unavailable inside container'. Does NOT contain 'crictl unavailable inside container' or 'etcd container not running'. Contains 'label=io.x-k8s.kind.cluster=kind' (the hardcoded =kind value), and DOES NOT contain a presence-only 'label=io.x-k8s.kind.cluster' literal.
  implication: bin/kinder is the original 47-04 implementation pre-b4327d99. It (a) has the legacy 'which etcdctl' code path emitting 'etcdctl unavailable inside container', and (b) uses the hardcoded =kind filter even in resumereadiness.go realListCPNodes (the bug that b4327d99 fixed). This is the binary the user actually ran to produce the UAT report.

- timestamp: 2026-05-05T12:22:00Z
  checked: git show 8d5504c8:pkg/internal/doctor/resumereadiness.go for legacy strings
  found: Original 47-04 version contains literal 'etcdctl unavailable inside container' (line 121) and 'which etcdctl' probe.
  implication: Confirms bin/kinder content matches the 8d5504c8 source state.

## Resolution

root_cause: |
  TWO distinct root causes — neither is a 47-05 source regression:

  (1) STALE LOCAL BUILD — explains Symptom 1 (cluster-resume-readiness "etcdctl unavailable inside container").
  The user ran /Users/patrykattc/work/git/kinder/bin/kinder, built 2026-05-05 08:40:53. That binary is a snapshot of source roughly 50 seconds BEFORE commit b4327d99 (47-04 label-template fix at 08:41:43) and ~70 minutes BEFORE commit a54de41c (47-05 crictl-exec replacement at 09:50:36). The compiled check still runs `which etcdctl` inside the CP container — which fails because etcdctl is not on the kindest/node container's PATH (it lives only inside the etcd static-pod container) — so it emits the legacy "etcdctl unavailable inside container" skip. The Homebrew binary on $PATH (kinder 1.4) does not even contain this check, confirming the user invoked ./bin/kinder. Source on disk for resumereadiness.go is fully correct and complete; the 47-05 fix simply was not recompiled.

  (2) GENUINE SOURCE BUG IN clusterskew.go — explains Symptom 2 (cluster-node-skew "(no cluster found)").
  pkg/internal/doctor/clusterskew.go line 77 hardcodes `--filter label=io.x-k8s.kind.cluster=kind`, pinning the discovery query to the default cluster name "kind". The UAT cluster is named "verify47", so the filter returns zero rows → realListNodes returns empty entries → check emits skip "(no cluster found)". This bug originated in commit afbfb7d7d (2026-04-08, phase 42) and predates phase 47 entirely; it would also reproduce against the freshly-rebuilt binary. Companion Cluster-category checks use presence-only or role-only filters (resumereadiness.go realListCPNodes uses `label=io.x-k8s.kind.cluster` presence-only; localpath.go realGetProvisionerVersion uses `label=io.x-k8s.kind.role=control-plane`), which is exactly why those checks discover verify47 correctly while cluster-node-skew does not.

fix: |
  (1) For Symptom 1 (legacy etcdctl message): rebuild bin/kinder from current HEAD. No source change needed — 47-05 is already merged as commit a54de41c. After rebuild, `kinder doctor` on a healthy 3-CP HA cluster should emit `ok 3/3 etcd members healthy` (or a warn if quorum is at risk).

  (2) For Symptom 2 (no cluster found): change pkg/internal/doctor/clusterskew.go:77 from
        "--filter", "label=io.x-k8s.kind.cluster=kind",
      to
        "--filter", "label=io.x-k8s.kind.cluster",
      matching the presence-only pattern used by resumereadiness.go realListCPNodes. Add a clusterskew_test.go case covering a non-default cluster name to lock in the fix.

verification: ""
files_changed:
  - pkg/internal/doctor/clusterskew.go (proposed source change)
  - bin/kinder (rebuild only — no source change)
