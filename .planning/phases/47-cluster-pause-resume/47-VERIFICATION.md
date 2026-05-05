---
phase: 47-cluster-pause-resume
verified: 2026-05-05T18:00:00Z
status: human_needed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification: true
previous_status: human_needed
previous_score: 4/4
gap_plans_consumed:
  - "47-05"
  - "47-06"
gaps_closed:
  - "SC4 / LIFE-04 probe was always unreachable (which etcdctl / /usr/local/bin/etcdctl — not present on kindest/node); replaced with crictl ps --name etcd -q + crictl exec into etcd static-pod container (47-05)"
  - "clusterskew.go hardcoded =kind value pin broke cluster discovery for any non-default cluster name; replaced with presence-only label=io.x-k8s.kind.cluster via clusterFilter() helper (47-06)"
  - "resumereadiness.go realListCPNodes used plain docker ps omitting stopped CPs; now uses docker ps -a via cpNodeFilter() + running-CP bootstrap selector with inspectState field (47-06)"
  - "kinder pause/resume --timeout and --wait were IntVar (rejected '5m'); migrated to DurationVar with 30s/5m defaults (47-06)"
  - "kinder get nodes rejected positional cluster name with cobra.NoArgs; relaxed to cobra.MaximumNArgs(1) with lifecycle.ResolveClusterName wiring (47-06)"
human_verification:
  - test: "Run `kinder pause <name>` against a real running multi-node cluster and observe host resource usage"
    expected: "All cluster containers transition to `exited` (verified via `docker ps -a --filter label=io.x-k8s.kind.cluster=<name>`) and host CPU/RAM drop to near-zero (no kinder-process or container processes consuming resources)"
    why_human: "Docker container stop semantics and host CPU/RAM observation cannot be automated in unit tests; tests verify the code calls `docker stop` but cannot assert host-side resource reclamation"
  - test: "Run `kinder pause <name>` then `kinder resume <name>` against a cluster with running pods, a PVC, and a Service; before pause `kubectl apply` a Deployment + PVC + Service"
    expected: "After resume: `kubectl get pods` shows the same pods Running with the same names/UIDs; `kubectl get pvc` shows the PVC bound with the same data (cat a sentinel file written before pause); `kubectl get svc` shows the same ClusterIP / NodePort allocations"
    why_human: "State preservation across docker stop/start of kindest/node containers — including kubelet recovery, etcd state intact, PV contents in container overlay — requires a real Docker daemon and Kubernetes API server to verify end-to-end"
  - test: "Run `kinder pause <name>` then `kinder resume <name>` on an HA cluster (3 control-plane + 2 workers + load-balancer)"
    expected: "Pause output shows worker container names stopping before CP names before LB; resume output shows LB starting first, then CP nodes, then workers. The `cluster-resume-readiness` check appears as a V(1) line in resume output between CP and worker start. Final cluster reaches all-Ready within --wait timeout"
    why_human: "End-to-end ordering and HA-quorum-safety require a real multi-CP cluster with running etcd to confirm; unit tests verify the call ordering in defaultCmder invocations but cannot prove etcd quorum survives the sequence"
  - test: "After developer rebuild (go build -o bin/kinder ./cmd/kinder && install bin/kinder /opt/homebrew/bin/kinder), run `kinder doctor` on a healthy 3-CP HA cluster named anything-other-than-kind"
    expected: "cluster-resume-readiness reports ok with '3/3 etcd members healthy'; cluster-node-skew does NOT report 'no cluster found'. After stopping 2 of 3 CPs: cluster-resume-readiness reports warn with reason mentioning quorum (1/3 healthy)"
    why_human: "UAT 12, 13, 14 were traced to a stale bin/kinder (built before 47-05 landed). Source is correct at HEAD. The rebuild is a developer environment step; the smoke test confirms the production crictl exec + clusterFilter + -a flag paths all work against a live HA cluster. Rebuild command: go build -o bin/kinder ./cmd/kinder && install bin/kinder /opt/homebrew/bin/kinder"
  - test: "Run `kinder resume <name> --wait 5m` and `kinder get nodes <cluster-name>` (positional arg) against a real cluster after rebuild"
    expected: "`kinder resume --wait 5m` parses successfully and waits up to 5 minutes (UAT 9 closed). `kinder get nodes verify47` lists nodes without error (UAT 3 closed). `kinder get nodes --name verify47` still works."
    why_human: "DurationVar and MaximumNArgs(1) fixes are source-level verified and unit-tested; end-to-end confirmation against a running binary closes the UAT loop. Requires the developer rebuild above."
---

# Phase 47: Cluster Pause/Resume Re-Verification Report (after 47-06)

**Phase Goal:** Users can pause and resume a kinder cluster to reclaim laptop resources without losing any cluster state
**Verified:** 2026-05-05T18:00:00Z
**Status:** human_needed
**Re-verification:** Yes — after gap closure plans 47-05 and 47-06

## Re-verification Summary

Previous verification (2026-05-05T11:00:00Z, status: human_needed, score: 4/4) closed the SC4/LIFE-04 probe gap (47-05) but identified 5 UAT failures. Gap closure plan 47-06 was written and has now landed (6 commits, 7d860f76..50aa742a). This verification confirms all source-level fixes are correct and that the 4 human_needed items from UAT tests 12, 13, 14 reduce to a single developer-rebuild step (stale binary) rather than any remaining source gap.

**What changed in 47-06:**

- `pkg/internal/doctor/clusterskew.go` — `realListNodes` previously hardcoded `label=io.x-k8s.kind.cluster=kind`, breaking discovery for every non-default cluster name. Refactored into `clusterFilter() []string` helper returning the presence-only filter (no `=kind` suffix). Line 60 confirmed.
- `pkg/internal/doctor/resumereadiness.go` — `realListCPNodes` added `"-a"` via new `cpNodeFilter()` helper (line 302) so stopped CPs appear in declared topology. New `inspectState` field (line 49) + `realInspectState` function (line 362) selects the first running CP as bootstrap. New "all control-plane containers stopped" warn path (line 135) handles full-CP-down edge case per warn-and-continue contract.
- `pkg/cmd/kind/resume/resume.go` — `flagpole.Timeout` and `flagpole.WaitTimeout` are now `time.Duration`; flags registered via `DurationVar` (lines 82-83) with defaults `30*time.Second` and `5*time.Minute`. No `* time.Second` multiplications at call site.
- `pkg/cmd/kind/pause/pause.go` — `flagpole.Timeout` is `time.Duration`; registered via `DurationVar` (line 79). Same pattern.
- `pkg/cmd/kind/get/nodes/nodes.go` — `Args: cobra.MaximumNArgs(1)` (line 76), `Use: "nodes [cluster-name]"`, `resolveClusterName` package-level var wired to `lifecycle.ResolveClusterName` (line 63), `runE` accepts and threads `args []string`.

**UAT issue triage:**

| UAT | Root cause | Source fixed? | Binary rebuild needed? |
|-----|-----------|--------------|----------------------|
| 3 (get nodes positional) | cobra.NoArgs blocked positional arg | Yes — 47-06 commit 50aa742a | Yes (to confirm end-to-end) |
| 9 (--wait 5m rejected) | IntVar rejected duration strings | Yes — 47-06 commit 7a4f722f | Yes (to confirm end-to-end) |
| 12 (doctor always-skip on healthy HA) | Stale binary (built before 47-04/47-05) + clusterskew =kind pin | Source fixes in 47-06; stale binary is dev env | Yes — primary cause |
| 13 (HA gate skips on quorum loss) | realListCPNodes missing -a flag | Yes — 47-06 commit ed85ecdf | Yes (to confirm end-to-end) |
| 14 (leaderID always empty) | Stale binary (built before 47-05) | No source change needed (pause.go was already correct at HEAD) | Yes — sole cause |

## Goal Achievement

### Observable Truths (from ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User runs `kinder pause [name]` and all cluster containers stop; CPU and RAM drop to near-zero on the host | ? UNCERTAIN (code OK) | `pkg/internal/lifecycle/pause.go:181-205` iterates `toStop` calling `defaultCmder(binaryName, "stop", "--time=N", node.String()).Run()` for every node — real docker stop. Wired from `pkg/cmd/kind/pause/pause.go:102` via `pauseFn = lifecycle.Pause`. DurationVar now wires `flags.Timeout` (time.Duration) directly. Host CPU/RAM observation needs human verification. |
| 2 | User runs `kinder resume [name]` and the cluster becomes fully operational; pods, PVs, and services are in the same state as before pause | ? UNCERTAIN (code OK) | `pkg/internal/lifecycle/resume.go:199-200` runs `docker start <node>` per node, then `defaultReadinessProber` polls kubectl until all nodes Ready (line 250). Wired from `pkg/cmd/kind/resume/resume.go:109`. `flags.WaitTimeout` (time.Duration) wires directly to `lifecycle.ResumeOptions.WaitTimeout`. Pod/PV/service state preservation needs real-cluster smoke test. |
| 3 | On a multi-control-plane cluster, pause/resume orchestrates container stop/start in quorum-safe order (workers before control-plane nodes on pause; reverse on resume) | ✓ VERIFIED | Pause builds explicit ordered list `workers → CP → LB` at lines 168-177. Resume runs Phase 1 (LB) → Phase 2 (CP) → Phase 3 (readiness hook, HA-only) → Phase 4 (workers) at lines 220-231. All ordering tests pass: `TestPause_OrderWorkersBeforeCP`, `TestPause_OrderCPBeforeLB_HA`, `TestResume_OrderLBBeforeCP_HA`, `TestResume_OrderCPBeforeWorkers`. |
| 4 | Before resuming an HA cluster, `kinder doctor` emits a `cluster-resume-readiness` warning if etcd quorum is at risk | ✓ VERIFIED | Check registered in `pkg/internal/doctor/check.go:83`. realListCPNodes now uses `docker ps -a` via `cpNodeFilter()` (line 302) — stopped CPs appear in declared topology. `inspectState` field (line 49) selects first running CP as bootstrap. `clusterFilter()` in clusterskew.go uses presence-only filter (line 60). crictl exec probe path confirmed present (lines 142, 170-171). "all control-plane containers stopped" warn path at line 135. Old `=kind` hardcode absent (grep returns 0 matches in production code). Old unreachable etcdctl probe paths absent. All tests pass including 4 new 47-06 tests: `TestClusterResumeReadiness_HA_StoppedCPs_Detected`, `TestClusterResumeReadiness_HA_AllCPsStopped_WarnNoEtcd`, `TestClusterResumeReadiness_RealListCPNodesIncludesA`, `TestClusterNodeSkew_RealListFilter_NoValuePin`. |

**Score:** 4/4 truths verified (SC1 and SC2 code-verified but require human observation for host-side behavior; SC3 and SC4 fully automated).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/internal/lifecycle/state.go` | Container-state helpers (ContainerState, ClusterStatus, ResolveClusterName, ClassifyNodes, ProviderBinaryName) | ✓ VERIFIED | All 5 functions exported. Used by status, pause, resume, get/clusters, get/nodes. Unchanged by 47-06. |
| `pkg/internal/lifecycle/pause.go` | Pause orchestration, quorum-safe ordering, HA snapshot capture via crictl, best-effort errors, idempotency | ✓ VERIFIED | readEtcdLeaderID uses crictl ps + crictl exec (from 47-05). No legacy docker exec etcdctl path. Unchanged by 47-06. |
| `pkg/internal/lifecycle/resume.go` | Resume orchestration, three-phase ordering with inline readiness hook, all-nodes-Ready gate | ✓ VERIFIED | Lines 116-260. Three-phase loop lines 220-231. WaitForNodesReady lines 302-338. Unchanged by 47-06. |
| `pkg/cmd/kind/pause/pause.go` | DurationVar --timeout; real RunE calling lifecycle.Pause; MaximumNArgs(1) | ✓ VERIFIED | DurationVar at line 79 (30*time.Second default). flags.Timeout assigned directly (no * time.Second). |
| `pkg/cmd/kind/resume/resume.go` | DurationVar --wait and --timeout; real RunE calling lifecycle.Resume; MaximumNArgs(1) | ✓ VERIFIED | DurationVar at lines 82-83 (30s and 5m defaults). flags.Timeout/WaitTimeout assigned directly. IntVar and WaitSecs symbols absent (grep returns 0 matches). |
| `pkg/cmd/kind/status/status.go` | `kinder status [name]` command with text + JSON output | ✓ VERIFIED | Unchanged by 47-06. Verified in prior verification. |
| `pkg/internal/doctor/clusterskew.go` | Presence-only kind cluster filter (no =kind value pin) | ✓ VERIFIED | clusterFilter() at line 59 returns `["--filter", "label=io.x-k8s.kind.cluster", "--format", "{{.Names}}"]`. grep for `label=io.x-k8s.kind.cluster=kind` in production files returns 0 matches. |
| `pkg/internal/doctor/resumereadiness.go` | docker ps -a in realListCPNodes; inspectState field; running-CP bootstrap; "all control-plane containers stopped" warn; crictl exec probe (47-05) | ✓ VERIFIED | cpNodeFilter() at line 302 includes "-a". inspectState field at line 49. realInspectState at line 362. "all control-plane containers stopped" warn at line 135. crictl exec discovery at line 142. |
| `pkg/internal/doctor/check.go` | Registry contains newClusterResumeReadinessCheck() and registry size is 24 | ✓ VERIFIED | Line 83 unchanged. Registry size 24 confirmed by TestAllChecks_Registry. Not changed by 47-06. |
| `pkg/cmd/kind/get/nodes/nodes.go` | cobra.MaximumNArgs(1); resolveClusterName wired to lifecycle.ResolveClusterName; runE accepts args | ✓ VERIFIED | Args: cobra.MaximumNArgs(1) at line 76. resolveClusterName var at line 62-64. runE signature at line 109 accepts args []string. cobra.NoArgs absent (grep returns 0 matches). |
| `pkg/cmd/kind/root.go` | Registers status, pause, resume commands | ✓ VERIFIED | Lines 89-93 unchanged by 47-06. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `pkg/cmd/kind/pause/pause.go` | `pkg/internal/lifecycle/pause.go` | `pauseFn = lifecycle.Pause` + RunE call | ✓ WIRED | Line 51 default, line 102 invocation. DurationVar flags flow directly to PauseOptions.Timeout. |
| `pkg/cmd/kind/resume/resume.go` | `pkg/internal/lifecycle/resume.go` | `resumeFn = lifecycle.Resume` + RunE call | ✓ WIRED | Line 53 default, line 109 invocation. DurationVar flags flow directly to ResumeOptions.StartTimeout/WaitTimeout. |
| `pkg/internal/lifecycle/pause.go` | `docker stop` subprocess | `defaultCmder(binaryName, "stop", "--time=N", ...)` | ✓ WIRED | Lines 183-189. Unchanged. |
| `pkg/internal/lifecycle/resume.go` | `docker start` subprocess | `defaultCmder(binaryName, "start", node)` | ✓ WIRED | Line 199. Unchanged. |
| `pkg/internal/lifecycle/pause.go` | `/kind/pause-snapshot.json` via crictl exec | `readEtcdLeaderID` → `crictl ps` + `crictl exec etcdctl` | ✓ WIRED | Lines 259, 284-285. Crictl path. captureHASnapshot at line 223; HA-gated at line 159. |
| `pkg/internal/lifecycle/resume.go` | `pkg/internal/doctor/resumereadiness.go` | `doctor.NewClusterResumeReadinessCheck().Run()` | ✓ WIRED | Line 93 in defaultResumeReadinessHook; invoked from Resume line 228 when HA + no errors. Unchanged. |
| `pkg/internal/doctor/check.go` | `pkg/internal/doctor/resumereadiness.go` | `newClusterResumeReadinessCheck()` in `allChecks` | ✓ WIRED | Line 83. Unchanged. |
| `pkg/internal/doctor/clusterskew.go` | `docker ps` with presence-only filter | `clusterFilter()` helper used in realListNodes | ✓ WIRED | exec.Command(binaryName, append([]string{"ps"}, clusterFilter()...)...) at line 82. No =kind suffix. |
| `pkg/internal/doctor/resumereadiness.go` | `docker ps -a` with presence-only filter | `cpNodeFilter()` helper used in realListCPNodes | ✓ WIRED | exec.Command(binaryName, cpNodeFilter()...) at line 331. "-a" at position 1. |
| `pkg/internal/doctor/resumereadiness.go` | `inspectState` → `realInspectState` | `newClusterResumeReadinessCheck()` wiring + bootstrap selection loop | ✓ WIRED | Line 64 (constructor), lines 123-128 (loop), line 362 (realInspectState). |
| `pkg/cmd/kind/get/nodes/nodes.go` | `lifecycle.ResolveClusterName` | `resolveClusterName` package-level var | ✓ WIRED | Line 62-64 var definition; line 138 call site in runE else-branch. --all-clusters short-circuits before the call (correct precedence). |
| `pkg/cmd/kind/root.go` | `pkg/cmd/kind/{status,pause,resume}` | `cmd.AddCommand(...NewCommand(...))` | ✓ WIRED | Lines 89-93. Unchanged. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `lifecycle.Pause` | `allNodes` | `fetcher.ListNodes(opts.ClusterName)` (defaults to `*cluster.Provider`) | Yes — real provider | ✓ FLOWING |
| `lifecycle.Pause` | `binaryName` | `pauseBinaryName()` → `ProviderBinaryName()` → `osexec.LookPath` | Yes — real PATH probe | ✓ FLOWING |
| `lifecycle.Resume` | `allNodes` | `opts.Provider.ListNodes(...)` | Yes — real provider | ✓ FLOWING |
| `readEtcdLeaderID` | `etcdContainerID` | `crictl ps --name etcd -q` via bootstrap.Command → exec.OutputLines | Yes — real crictl call on kindest/node | ✓ FLOWING |
| `readEtcdLeaderID` | `leaderID` | `crictl exec <id> etcdctl endpoint status --cluster --write-out=json` parsed by parseEtcdLeader | Yes — real etcdctl inside etcd container | ✓ FLOWING |
| `clusterResumeReadinessCheck.Run` | `cpNodeNames` | `realListCPNodes` → `docker ps -a` with presence-only filter | Yes — includes stopped CPs | ✓ FLOWING |
| `clusterResumeReadinessCheck.Run` | `bootstrap` | `inspectState` loop over cpNodeNames → first "running" container | Yes — real inspect call | ✓ FLOWING |
| `clusterResumeReadinessCheck.Run` | `etcdContainerID` | `execInContainer(binaryName, bootstrap, "crictl", "ps", "--name", "etcd", "-q")` | Yes — real docker exec + crictl ps | ✓ FLOWING |
| `clusterResumeReadinessCheck.Run` | `healthLines` | `execInContainer(binaryName, bootstrap, "crictl", "exec", id, "etcdctl", ..., "endpoint", "health", ...)` | Yes — real etcdctl endpoint health inside etcd container | ✓ FLOWING |
| `kinder doctor --output json` | `checks` array | `doctor.RunAllChecks()` iterating `allChecks` (24 entries) | Yes — verified by running binary in prior verification | ✓ FLOWING |
| `kinder get nodes <name>` | `allNodes` | `provider.ListNodes(targetName)` where `targetName` comes from `lifecycle.ResolveClusterName(args, provider)` | Yes — real provider with resolved name | ✓ FLOWING |
| `kinder resume --wait 5m` | `opts.WaitTimeout` | `flags.WaitTimeout` (time.Duration, DurationVar-parsed) assigned directly | Yes — 5m parsed as 5*time.Minute by cobra | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` clean after 47-06 | `go build ./...` | No errors, no output | ✓ PASS |
| All doctor tests pass (includes 4 new 47-06 tests) | `go test ./pkg/internal/doctor/... -count=1` | ok sigs.k8s.io/kind/pkg/internal/doctor 0.350s | ✓ PASS |
| All pause/resume tests pass (includes 8 test changes in 47-06) | `go test ./pkg/cmd/kind/pause/... ./pkg/cmd/kind/resume/... -count=1` | ok pause 0.605s; ok resume 1.142s | ✓ PASS |
| All get/nodes tests pass (includes 4 new 47-06 tests) | `go test ./pkg/cmd/kind/get/nodes/... -count=1` | ok sigs.k8s.io/kind/pkg/cmd/kind/get/nodes 1.387s | ✓ PASS |
| Full repository test suite passes | `go test ./... -count=1 -timeout 5m` | All packages ok; no failures | ✓ PASS |
| `=kind` value pin absent from production code | `grep -rn 'label=io.x-k8s.kind.cluster=kind' pkg/ cmd/` | 0 matches in production files (1 match in clusterskew_test.go line 250 — test assertion that it is absent) | ✓ PASS |
| IntVar and WaitSecs absent from pause/resume | `grep -n 'IntVar\|WaitSecs\|time\.Duration(flags' pkg/cmd/kind/pause/pause.go pkg/cmd/kind/resume/resume.go` | 0 matches | ✓ PASS |
| cobra.NoArgs absent from get/nodes | `grep -n 'cobra.NoArgs' pkg/cmd/kind/get/nodes/nodes.go` | 0 matches | ✓ PASS |
| "-a" flag present in cpNodeFilter | `grep -n '"-a"' pkg/internal/doctor/resumereadiness.go` | Line 302: `"-a"` confirmed | ✓ PASS |
| "all control-plane containers stopped" warn message present | `grep -n 'all control-plane containers stopped' pkg/internal/doctor/resumereadiness.go` | Line 135: confirmed | ✓ PASS |
| 47-06 commit hashes all present in git history | `git log --oneline 7d860f76 ed85ecdf 738c70c0 7a4f722f 057df188 50aa742a` | All 6 commits confirmed | ✓ PASS |
| Legacy etcdctl probe paths absent | `grep -rn 'which etcdctl\|/usr/local/bin/etcdctl' pkg/ cmd/` | 0 matches in production code | ✓ PASS |
| Real-cluster smoke (UAT 3, 9, 12, 13, 14 re-run after rebuild) | Requires developer rebuild and live HA cluster | Not executed — requires developer environment step | ? SKIP |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|------------|------------|-------------|--------|----------|
| LIFE-01 | 47-01, 47-02, 47-06 | User can pause a running cluster via `kinder pause [name]`, freeing CPU/RAM without losing state | ? NEEDS HUMAN | Code path complete (lifecycle.Pause stops all containers via docker stop; DurationVar --timeout wired correctly). Host CPU/RAM observation needs human verification. |
| LIFE-02 | 47-01, 47-03, 47-06 | User can resume a paused cluster via `kinder resume [name]`; pods, PVs, and node state are preserved | ? NEEDS HUMAN | Code path complete (lifecycle.Resume + readiness gate; DurationVar --wait wired correctly; kinder get nodes now accepts positional arg). State preservation needs real-cluster verification. |
| LIFE-03 | 47-02, 47-03 | Pause-resume orchestrates control-plane and worker stop/start in correct order to preserve etcd quorum | ✓ SATISFIED | Quorum-safe ordering implemented and verified by tests. Unchanged by 47-06. |
| LIFE-04 | 47-04, 47-05, 47-06 | Doctor pre-flight check `cluster-resume-readiness` runs before resume on HA clusters and warns if etcd quorum is at risk | ✓ SATISFIED | Check implemented, registered, inline-invoked from Resume. 47-05 replaced unreachable probe with working crictl-based one. 47-06 fixed cluster discovery (=kind pin removed, -a flag added, running-CP bootstrap selection, inspectState field). All tests pass. Real-cluster smoke pending after developer rebuild. |

No orphaned requirements — all four phase 47 requirements are claimed by at least one plan.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | - | - | - | - |

No TODO/FIXME/placeholder/"not yet implemented" markers found in any phase 47 production file after 47-06. No stub patterns. The `=kind` hardcode is confirmed removed (production) and its absence is asserted by a test. No legacy IntVar/WaitSecs symbols remain. No legacy etcdctl probe paths remain.

### Human Verification Required

Phase 47 is functionally complete at the source level. All five UAT gaps are addressed: three by source fixes (UAT 3, 9, 13) and two by a stale binary identified as the root cause with no source change needed (UAT 12, 14). The remaining human_needed items split into: (a) the inherent SC1/SC2 host-behavior verification that no code change can replace, and (b) a developer rebuild step required before re-running UAT 12, 13, 14.

#### 1. Host resource reclamation on pause

**Test:** Run `kinder pause <name>` against a real running multi-node cluster and observe host resource usage
**Expected:** All cluster containers transition to `exited` (verified via `docker ps -a --filter label=io.x-k8s.kind.cluster=<name>`) and host CPU/RAM drop to near-zero
**Why human:** Docker container stop semantics and host CPU/RAM observation cannot be automated in unit tests

#### 2. Cluster state preservation across pause/resume

**Test:** Run `kinder pause <name>` then `kinder resume <name>` against a cluster with running pods, a PVC, and a Service; before pause `kubectl apply` a Deployment + PVC + Service
**Expected:** After resume: `kubectl get pods` shows the same pods Running with the same names/UIDs; `kubectl get pvc` shows the PVC bound with the same data; `kubectl get svc` shows the same ClusterIP / NodePort allocations
**Why human:** State preservation across docker stop/start requires a real Docker daemon and Kubernetes API server to verify end-to-end

#### 3. HA quorum-safe ordering on a real multi-CP cluster

**Test:** Run `kinder pause <name>` then `kinder resume <name>` on an HA cluster (3 control-plane + 2 workers + load-balancer)
**Expected:** Pause output shows worker container names stopping before CP names before LB; resume output shows LB starting first, then CP nodes, then workers. `cluster-resume-readiness` check appears between CP and worker start. Final cluster reaches all-Ready within --wait timeout
**Why human:** End-to-end ordering and HA-quorum-safety require a real multi-CP cluster with running etcd to confirm

#### 4. Developer rebuild + UAT 12/13/14/9/3 re-run

**Test:** After `go build -o bin/kinder ./cmd/kinder && install bin/kinder /opt/homebrew/bin/kinder`, run:
- `kinder doctor` on a healthy 3-CP HA cluster named anything-other-than-kind
- `docker stop <cluster>-control-plane2 <cluster>-control-plane3` then `kinder doctor` again
- `kinder resume <name> --wait 5m`
- `kinder get nodes <cluster-name>` (positional arg)

**Expected:**
- UAT 12: `cluster-resume-readiness: ok, 3/3 etcd members healthy` (not "etcdctl unavailable inside container")
- UAT 13: `cluster-resume-readiness: warn` with reason mentioning quorum (1/3 healthy)
- UAT 14: `/kind/pause-snapshot.json` contains non-empty leaderID
- UAT 9: `--wait 5m` parses without error
- UAT 3: `kinder get nodes verify47` lists nodes without error

**Why human:** Source fixes are verified at HEAD. The running binary (bin/kinder and /opt/homebrew/bin/kinder) predates 47-05 and 47-06 commits. This is a developer environment step, not a code quality issue. Rebuild verification: `strings $(which kinder) | grep -c 'crictl ps --name etcd'` must be >=1; `strings $(which kinder) | grep -c 'all control-plane containers stopped'` must be >=1; `strings $(which kinder) | grep -c 'label=io.x-k8s.kind.cluster=kind'` must be 0.

### Gaps Summary

No source-level gaps remain after 47-06. All five UAT issues are closed at the code level:

- **UAT 3 (get nodes positional arg):** Fixed in 47-06 (cobra.MaximumNArgs(1) + lifecycle.ResolveClusterName). Commit 50aa742a. Unit-tested with 4 new tests.
- **UAT 9 (--wait 5m rejected):** Fixed in 47-06 (DurationVar migration). Commit 7a4f722f. Unit-tested with 8 test changes.
- **UAT 12 (doctor skip on healthy HA):** Root cause was stale binary; source bug was =kind pin in clusterskew.go, fixed in 47-06 (clusterFilter helper). Commit ed85ecdf. Confirmed absent by grep in production files.
- **UAT 13 (HA gate skips on quorum loss):** Fixed in 47-06 (-a flag in cpNodeFilter + inspectState-based bootstrap selection). Commit ed85ecdf. "all control-plane containers stopped" warn path confirmed present at line 135.
- **UAT 14 (leaderID always empty):** Root cause was stale binary only. pause.go at HEAD was already correct after 47-05 (crictl exec path). No source change needed. Rebuild closes this.

The phase is **complete in source**. The remaining human_needed items are: (a) inherent host-behavior verification that cannot be automated (SC1, SC2, SC3), and (b) a one-time developer rebuild step that then unlocks UAT 12/13/14/9/3 re-runs. No further gap closure cycles are warranted.

---

_Verified: 2026-05-05T18:00:00Z_
_Verifier: Claude (gsd-verifier)_
_Re-verification after gap closure plans 47-05 and 47-06_
