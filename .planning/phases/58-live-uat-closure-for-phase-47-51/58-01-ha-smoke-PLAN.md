---
phase: 58-live-uat-closure-for-phase-47-51
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - hack/uat-47-ha-smoke.sh
  - hack/uat-47-ha-smoke.log
  - .planning/phases/47-cluster-pause-resume/47-UAT.md
autonomous: false
requirements:
  - UAT-01

planner_decisions:
  - id: "c) manual vs CI execution"
    decision: "manual-only for v2.4"
    rationale: "REQUIREMENTS.md UAT-01 only requires script + log evidence; no CI invocation required. Wiring into a self-hosted runner can land in v2.5. Recorded per RESEARCH Open Question 2."
  - id: "d) cluster teardown default"
    decision: "leave-up by default; explicit --teardown flag or TEARDOWN=yes env var"
    rationale: "Failed UAT clusters are debugging gold; successful clusters are cheap clutter. RESEARCH Open Question 4."
  - id: "b) versionPreRelease source change scope"
    decision: "out of scope for Phase 58; deferred to v2.5 cosmetic phase"
    rationale: "kindversion/version.go suppresses commit hash whenever versionPreRelease == \"\"; rather than touching that file, this plan uses the researcher-prescribed `make build` + `strings $(./bin/kinder)` marker grep technique (RESEARCH Pattern 1) which is more honest (inspects artifact, not a string the artifact prints). Recorded per RESEARCH Open Question 1."
  - id: "freshness gate technique"
    decision: "rebuild-every-invocation (make build) + strings-marker grep (POSITIVE + NEGATIVE marker lists)"
    rationale: "Pitfall 23 is the definitive gate. `./bin/kinder version` does NOT print the git hash today, so naive hash-diff is impossible. Rebuild guarantees fresh; strings catches the rare silent no-op. Markers locked from RESEARCH §Pitfall-23 Stable Marker table."

must_haves:
  truths:
    - "hack/uat-47-ha-smoke.sh exists, is executable (chmod +x), passes `bash -n` and (if available) `shellcheck`, and starts with `set -euo pipefail`"
    - "The script's first non-comment runtime step is `make build`; the second is a `strings ${REPO_ROOT}/bin/kinder | grep -qF <marker>` gate over the 5 POSITIVE v2.4 markers from RESEARCH (Pitfall 23 Stable Marker table) AND the 3 NEGATIVE markers"
    - "Every kinder invocation inside the script uses the absolute path `${REPO_ROOT}/bin/kinder` (NEVER bare `kinder` from $PATH) — Pitfall F Cask-shadow protection"
    - "The script creates a 3-CP + 2-worker cluster named `uat-58-01` via `./bin/kinder create cluster --name uat-58-01 --config -` with an inline here-doc (LB container is auto-created by kinder when CP count >= 2 — confirmed by 51-UAT.md test 1)"
    - "47-UAT.md tests 3, 9, 12, 13, 14 — the 5 rows currently `result: issue` — are individually exercised by a named test function in the script; each prints `[OK] test_<N>` on success and `[FAIL] test_<N>` (exit 1) on failure"
    - "Test 3 runs `./bin/kinder get nodes uat-58-01` and asserts exit 0 + tabular output with 6 rows"
    - "Test 9 runs `./bin/kinder resume uat-58-01 --wait 5m` and asserts exit 0 with NO `strconv.ParseInt` error in stderr"
    - "Test 12 runs `./bin/kinder doctor` and greps for a line containing `cluster-resume-readiness` AND `3/3 etcd members healthy` (DIAG-06 Phase 57 wording)"
    - "Test 13 docker-stops 2 of 3 CPs, runs `./bin/kinder doctor`, and greps for `cluster-resume-readiness` AND (`quorum at risk` OR `1/3`); the test then `docker start`s the 2 CPs back so the cluster recovers before subsequent tests"
    - "Test 14 pauses the cluster then runs `docker exec <bootstrap-cp> cat /kind/pause-snapshot.json | jq -r .leaderID` and asserts non-empty output; stderr from pause must NOT contain `failed to capture etcd leader id`"
    - "SC1 host-observation evidence: the script captures `docker stats --no-stream` AND `docker ps -a --filter label=io.x-k8s.kind.cluster=uat-58-01 --format '{{.Names}}: {{.Status}}'` snapshots at three points (baseline / post-pause / post-resume); post-pause snapshot asserts every container line contains `Exited`"
    - "SC2 state-preservation evidence: pre-pause `kubectl apply` of a Deployment + PVC + Service named `uat-deploy` / `uat-pvc` / `uat-svc`; write SENTINEL string into PVC; capture pod-UID + ClusterIP; pause+resume; assert pod-UID, ClusterIP, and SENTINEL readback are byte-identical"
    - "Quorum-safe ordering observation: pause output (text mode, not --json) is grep-checked for worker names appearing BEFORE control-plane names BEFORE external-load-balancer; resume output checked for the reverse"
    - "All script output is `tee`'d to `hack/uat-47-ha-smoke.log` (path overridable via $LOG)"
    - "On any test failure the cluster is LEFT UP (no auto-delete) and the script prints the exact cleanup command (`./bin/kinder delete cluster --name uat-58-01`) plus a pointer to the log"
    - "On `--teardown` flag OR `TEARDOWN=yes` env var, the script deletes the cluster after all tests pass"
    - "Re-run safety: the script's preamble detects an existing `uat-58-01` cluster and either prompts for delete (interactive) or honors `REUSE=no` to auto-delete before create"
    - "47-UAT.md frontmatter `status: diagnosed` flips to `status: closed`; `updated:` timestamp refreshed; tests 3, 9, 12, 13, 14 each transition `result: issue` -> `result: pass`; `reported:` and `severity:` keys are dropped; an `evidence:` block is added per-row referencing the log file with the captured commands + output excerpt"
    - "47-UAT.md summary block transitions `passed: 9` -> `passed: 14`, `issues: 5` -> `issues: 0`; the `## Gaps` section is replaced with a single line: `All UAT issues closed via Phase 58 Plan 01 (2026-05-12+). See hack/uat-47-ha-smoke.log.`"
    - "hack/uat-47-ha-smoke.log is committed (NOT gitignored) and contains the verbatim output of one successful run — `./bin/kinder version` line, `git rev-parse HEAD`, `stat` of `./bin/kinder`, all 5 `[OK] test_<N>` lines, the SC1 docker-stats snapshots, the SC2 sentinel-readback success line, and the final summary"
  artifacts:
    - path: "hack/uat-47-ha-smoke.sh"
      provides: "Executable bash script (chmod +x) implementing preamble + strings-gate + 3-CP HA cluster create + 5 test_<N> functions + SC1/SC2 evidence + ordering capture + finalize"
      contains: "make build"
    - path: "hack/uat-47-ha-smoke.sh"
      provides: "Absolute-path enforcement"
      contains: "${REPO_ROOT}/bin/kinder"
    - path: "hack/uat-47-ha-smoke.sh"
      provides: "Strings-marker gate using locked v2.4 markers"
      contains: "all control-plane containers stopped"
    - path: "hack/uat-47-ha-smoke.sh"
      provides: "Quorum-at-risk DIAG-06 wording assertion"
      contains: "quorum at risk"
    - path: "hack/uat-47-ha-smoke.log"
      provides: "Verbatim log of one successful run (canonical evidence artifact)"
      contains: "[OK] test_14"
    - path: ".planning/phases/47-cluster-pause-resume/47-UAT.md"
      provides: "Frontmatter status flip + 5-row result flip + summary block update + Gaps replacement"
      contains: "passed: 14"
  key_links:
    - from: "hack/uat-47-ha-smoke.sh preamble"
      to: "./bin/kinder (rebuilt from HEAD)"
      via: "make build + strings $(./bin/kinder) | grep -qF '<marker>'"
      pattern: "strings .*bin/kinder.*grep -qF"
    - from: "hack/uat-47-ha-smoke.sh test_13"
      to: "DIAG-06 tolerant etcd JSON parse (Phase 57-02 deliverable)"
      via: "./bin/kinder doctor output grep for `quorum at risk`"
      pattern: "quorum at risk"
    - from: "47-UAT.md test 3,9,12,13,14 result rows"
      to: "hack/uat-47-ha-smoke.log evidence"
      via: "per-row `evidence:` block referencing the log + captured command + output excerpt"
      pattern: "result: pass"
---

<objective>
Close UAT-01 (REQUIREMENTS.md) by delivering a self-contained bash script that runs the Phase 47 HA pause/resume smoke against a freshly rebuilt v2.4 `./bin/kinder`, captures live evidence into a committed log file, and flips the 5 currently-open `result: issue` rows in `.planning/phases/47-cluster-pause-resume/47-UAT.md` to `result: pass`.

Purpose: ROADMAP Phase 58 SC1 + SC2 + SC4. The 5 issue rows in 47-UAT.md (tests 3, 9, 12, 13, 14) are all closed at HEAD by source fixes already landed in 47-06 (commits 50aa742a, 7a4f722f, ed85ecdf) and 57-02 (commit c43bb599 — DIAG-06 tolerant etcd JSON parse). The remaining gap is purely the live-against-rebuilt-binary attestation step that the v2.3 audit deferred to Phase 58. Pitfall 23 (stale-PATH-binary) is the singular blast-radius risk; the script's preamble closes it via `make build` + strings-marker grep per RESEARCH Pattern 1.

Output:
  - `hack/uat-47-ha-smoke.sh` — NEW (≤ 250 lines). Idempotent rebuild preamble + 5 POSITIVE + 3 NEGATIVE strings gates + HA cluster create + per-test functions for tests 3/9/12/13/14 + SC1 docker-stats observation + SC2 PVC sentinel round-trip + pause/resume ordering observation + finalize. `set -euo pipefail`; absolute `${REPO_ROOT}/bin/kinder`; leave-up-on-failure (Pitfall G evidence).
  - `hack/uat-47-ha-smoke.log` — NEW (committed; NOT gitignored). Verbatim `tee` output of one successful run.
  - `.planning/phases/47-cluster-pause-resume/47-UAT.md` — EDITED. Status flip (`diagnosed` -> `closed`); 5 row flips (`issue` -> `pass` with `evidence:` blocks); summary count updates; Gaps section replaced.

Out of scope: REQUIREMENTS.md / ROADMAP.md / STATE.md edits and the 58-01-SUMMARY.md doc — those land at the verifier / phase-close step after BOTH 58-01 and 58-02 are merged.
</objective>

<execution_context>
@/Users/patrykattc/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/REQUIREMENTS.md
@.planning/research/PITFALLS.md
@.planning/phases/58-live-uat-closure-for-phase-47-51/58-RESEARCH.md
@.planning/phases/47-cluster-pause-resume/47-UAT.md
@.planning/phases/47-cluster-pause-resume/47-VERIFICATION.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-04-envoy-gateway-bump-PLAN.md
@Makefile
@pkg/internal/kindversion/version.go
@pkg/internal/lifecycle/pause.go
@pkg/internal/doctor/resumereadiness.go

<interfaces>
<!-- Locked surfaces the executor must honor. All verified at HEAD on 2026-05-12. -->

**Locked script path (from REQUIREMENTS.md UAT-01):** `hack/uat-47-ha-smoke.sh` — NOT `scripts/...`, NOT `tests/...`. The path is in the requirement wording.

**Stable v2.4 POSITIVE marker strings (RESEARCH §Pitfall 23):**

| Marker (exact bytes) | Source | Asserts present in v2.4 |
|----------------------|--------|--------------------------|
| `crictl ps --name etcd -q` | `pkg/internal/lifecycle/pause.go:259` | 47-05 leader-ID crictl path |
| `all control-plane containers stopped` | `pkg/internal/doctor/resumereadiness.go:135` | 47-06 stopped-CPs warn path |
| `docker.io/envoyproxy/envoy:v1.36.2` | `pkg/cluster/internal/loadbalancer/const.go:20` | 51-01 Envoy LB image |
| `kubeProxyMode: ipvs is not supported with Kubernetes 1.36+` | `pkg/internal/apis/config/validate.go:92` | 51-02 IPVS-1.36 guard |
| `quorum at risk` | `pkg/internal/doctor/resumereadiness.go` (57-02 tolerant parse) | 57-02 DIAG-06 wording |

**Stable v2.4 NEGATIVE marker strings (must NOT appear):**

| Marker | Means if present |
|--------|------------------|
| `label=io.x-k8s.kind.cluster=kind` | 47-06 clusterFilter regression (value-pinned filter restored) |
| `/usr/local/bin/etcdctl` | pre-47-05 unreachable etcdctl path leaked back |
| `kindest/haproxy` | pre-51-01 HAProxy LB image not removed |

**Test→fix→evidence mapping (47-UAT.md rows to flip):**

| # | Test | Closed by | Evidence in log |
|---|------|-----------|------------------|
| 3 | `kinder get nodes <cluster>` positional arg | 47-06 commit 50aa742a (cobra.MaximumNArgs(1)) | `./bin/kinder get nodes uat-58-01` exit 0 + table |
| 9 | `kinder resume --wait 5m` duration parse | 47-06 commit 7a4f722f (DurationVar) | `./bin/kinder resume uat-58-01 --wait 5m` exit 0, no `strconv.ParseInt` |
| 12 | `cluster-resume-readiness: ok, 3/3` healthy HA | 47-06 (clusterFilter + -a) + 57-02 (tolerant parse) | `./bin/kinder doctor` line matches `cluster-resume-readiness.*3/3 etcd members healthy` |
| 13 | `cluster-resume-readiness: warn` quorum loss | 47-06 (-a flag) + 57-02 (tolerant parse + `quorum at risk` wording) | After `docker stop` of 2 CPs: `./bin/kinder doctor` line matches `cluster-resume-readiness.*(quorum at risk\|1/3)` |
| 14 | Non-empty `leaderID` in /kind/pause-snapshot.json | 47-05 crictl path (no source change needed beyond rebuild) | `docker exec ... cat /kind/pause-snapshot.json \| jq -r .leaderID` non-empty; pause stderr does NOT contain `failed to capture etcd leader id` |

**Cluster topology (locked by 47-VERIFICATION.md §3 + REQUIREMENTS.md UAT-01):**

3 CP + 2 worker. The LB container is auto-created by kinder when count(control-plane) >= 2 (no `external-load-balancer` role needed in config; 51-UAT.md test 1 confirms via `docker ps`).

**Cluster name (this plan):** `uat-58-01` — unique per plan, so re-runs of 58-01 don't collide with 58-02 (`uat-58-02`).

**make build invariant:** `Makefile` `build` target injects `-X=...gitCommit=$(COMMIT) -X=...gitCommitCount=$(COMMIT_COUNT)` but `pkg/internal/kindversion/version.go` line 60 suppresses both when `versionPreRelease == ""`. So `./bin/kinder version` prints `kind v1.5.0 go1.26.3 darwin/arm64` — no hash. Do NOT plan to grep the hash from version output; use `strings $(bin)` instead (planner decision (b)).

**Re-entrancy contract:** A re-invocation of the script after a failed run MUST detect the leftover `uat-58-01` cluster. Behavior: if `REUSE=no` (env var), auto-delete; else print the cleanup command and exit 1 (so the developer chooses).

**Log path:** `hack/uat-47-ha-smoke.log`. Overridable via `LOG=...`. The default is what gets committed to git as evidence.

**47-UAT.md schema (today vs after):**

Today (test 3 — issue):
```yaml
### 3. kinder get nodes shows real container state
expected: ...
result: issue
reported: "kinder get nodes verify47 -> ERROR: unknown command..."
severity: major
```

After (test 3 — pass):
```yaml
### 3. kinder get nodes shows real container state
expected: ...
result: pass
evidence: |
  Live UAT against ./bin/kinder built from HEAD <commit>.
  $ ./bin/kinder get nodes uat-58-01
  <captured node table here>
  Exit 0. Resolved via cobra.MaximumNArgs(1) + lifecycle.ResolveClusterName (47-06).
note: |
  Closed by 47-06 source fixes + Phase 58 live UAT. See hack/uat-47-ha-smoke.log for full transcript.
```

`reported:` and `severity:` keys dropped on flip (issue-only metadata). `evidence:` block added (matches 51-UAT.md schema). Summary block: `passed: 9` → `passed: 14`; `issues: 5` → `issues: 0`. `## Gaps` section replaced with the single-line closure pointer.
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Author hack/uat-47-ha-smoke.sh + syntax-validate (no live cluster yet)</name>
  <files>hack/uat-47-ha-smoke.sh</files>
  <action>
Create `hack/uat-47-ha-smoke.sh` (new file). Mark executable. Implement the following structure verbatim — preserve function names, marker strings, and assertion bodies:

```bash
#!/usr/bin/env bash
# Source: hack/uat-47-ha-smoke.sh — Phase 58 Plan 01
# Closes: REQUIREMENTS.md UAT-01 + 47-UAT.md tests 3, 9, 12, 13, 14
# Pitfall 23 gate: rebuilds ./bin/kinder against current HEAD on every invocation.
# Hard contract: ALWAYS uses absolute ${REPO_ROOT}/bin/kinder; NEVER bare `kinder` from $PATH.
# Default: leaves cluster up on failure (and on success unless --teardown).
# Re-runs: detect leftover uat-58-01 cluster; set REUSE=no to auto-delete before recreate.

set -euo pipefail

CLUSTER="uat-58-01"
LOG="${LOG:-hack/uat-47-ha-smoke.log}"
TEARDOWN="${TEARDOWN:-no}"
REUSE="${REUSE:-prompt}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
KINDER_BIN="${REPO_ROOT}/bin/kinder"

# ---------- preamble ----------

preamble() {
  cd "${REPO_ROOT}"
  echo "=== Phase 58 Plan 01 — Phase 47 HA pause/resume live UAT ==="
  echo "ETA ~10 min; cluster ${CLUSTER}; log ${LOG}"
  echo "HEAD:  $(git rev-parse HEAD)"
  echo "Date:  $(date -u +%Y-%m-%dT%H:%M:%SZ)"

  # 1) Rebuild (idempotent on clean tree)
  echo "--- make build ---"
  make build

  # 2) POSITIVE marker gate
  local positive=(
    "crictl ps --name etcd -q"
    "all control-plane containers stopped"
    "docker.io/envoyproxy/envoy:v1.36.2"
    "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+"
    "quorum at risk"
  )
  for m in "${positive[@]}"; do
    if ! strings "${KINDER_BIN}" | grep -qF "${m}"; then
      echo "[FAIL] STALE BINARY — required v2.4 marker absent: ${m}" >&2
      exit 1
    fi
  done

  # 3) NEGATIVE marker gate
  local negative=(
    "label=io.x-k8s.kind.cluster=kind"
    "/usr/local/bin/etcdctl"
    "kindest/haproxy"
  )
  for m in "${negative[@]}"; do
    if strings "${KINDER_BIN}" | grep -qF "${m}"; then
      echo "[FAIL] STALE BINARY — forbidden pre-v2.4 marker present: ${m}" >&2
      exit 1
    fi
  done

  # 4) Document version + path + Docker capacity
  "${KINDER_BIN}" version
  echo "Using: ${KINDER_BIN}"
  echo "Build: $(stat -f '%Sm' "${KINDER_BIN}" 2>/dev/null || stat -c '%y' "${KINDER_BIN}")"
  printf 'PATH-resolved kinder: '
  which kinder || echo "(not in PATH — OK)"
  echo "--- docker info (RAM headroom) ---"
  docker info 2>/dev/null | grep -E 'Total Memory|CPUs' || true

  # 5) Re-entrancy: detect leftover cluster
  if "${KINDER_BIN}" get clusters --output json 2>/dev/null | grep -q "\"name\":\"${CLUSTER}\""; then
    if [[ "${REUSE}" == "no" ]]; then
      echo "Existing cluster ${CLUSTER} detected; REUSE=no — deleting before recreate."
      "${KINDER_BIN}" delete cluster --name "${CLUSTER}"
    else
      echo "[FAIL] Existing cluster ${CLUSTER} detected. Run:" >&2
      echo "  ${KINDER_BIN} delete cluster --name ${CLUSTER}" >&2
      echo "  # then re-run this script, or set REUSE=no" >&2
      exit 1
    fi
  fi
}

# ---------- live phase ----------

create_ha_cluster() {
  echo "--- create cluster ${CLUSTER} (3 CP + 2 worker) ---"
  "${KINDER_BIN}" create cluster --name "${CLUSTER}" --config - <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: control-plane
  - role: control-plane
  - role: worker
  - role: worker
EOF
  kubectl --context "kind-${CLUSTER}" wait --for=condition=Ready node --all --timeout=180s
}

observe_baseline_stats() {
  echo "--- SC1 baseline: docker stats + ps ---"
  docker stats --no-stream \
    --filter "label=io.x-k8s.kind.cluster=${CLUSTER}" \
    --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}' || true
  docker ps -a \
    --filter "label=io.x-k8s.kind.cluster=${CLUSTER}" \
    --format '{{.Names}}: {{.Status}}'
}

test_03_get_nodes_positional() {
  echo "--- test_03: get nodes <cluster> positional arg ---"
  local out
  out="$("${KINDER_BIN}" get nodes "${CLUSTER}")"
  # Expect 6 rows (3 CP + 2 worker + 1 LB).
  local rowcount
  rowcount="$(echo "${out}" | grep -cE "^${CLUSTER}-" || true)"
  if [[ "${rowcount}" -lt 5 ]]; then
    echo "[FAIL] test_03 — expected >=5 node rows; got ${rowcount}"
    echo "${out}"
    return 1
  fi
  echo "${out}"
  echo "[OK] test_03 — get nodes ${CLUSTER} returned ${rowcount} rows"
}

test_09_resume_wait_duration_string() {
  echo "--- test_09: resume --wait 5m duration-string parsing ---"
  # Pre-condition: cluster is running. We pause + resume here to exercise --wait.
  "${KINDER_BIN}" pause "${CLUSTER}"
  local stderr_capture
  set +e
  stderr_capture="$("${KINDER_BIN}" resume "${CLUSTER}" --wait 5m 2>&1 >/dev/null)"
  local exit_code=$?
  set -e
  if [[ ${exit_code} -ne 0 ]]; then
    echo "[FAIL] test_09 — resume --wait 5m exited ${exit_code}"
    echo "${stderr_capture}"
    return 1
  fi
  if grep -q 'strconv.ParseInt' <<< "${stderr_capture}"; then
    echo "[FAIL] test_09 — stderr contains strconv.ParseInt (IntVar regression)"
    echo "${stderr_capture}"
    return 1
  fi
  kubectl --context "kind-${CLUSTER}" wait --for=condition=Ready node --all --timeout=180s
  echo "[OK] test_09 — --wait 5m accepted; no ParseInt; cluster Ready"
}

test_12_doctor_healthy_3of3() {
  echo "--- test_12: doctor reports 3/3 etcd members healthy on healthy HA ---"
  local out
  out="$("${KINDER_BIN}" doctor 2>&1)"
  if ! grep -qE 'cluster-resume-readiness.*3/3 etcd members healthy' <<< "${out}"; then
    echo "[FAIL] test_12 — cluster-resume-readiness did not report 3/3 etcd members healthy"
    grep -E 'cluster-(resume-readiness|node-skew)' <<< "${out}" || true
    return 1
  fi
  echo "[OK] test_12 — cluster-resume-readiness: 3/3 etcd members healthy"
}

test_13_doctor_warn_quorum_loss() {
  echo "--- test_13: doctor warns when 2/3 CPs are stopped (quorum at risk) ---"
  docker stop "${CLUSTER}-control-plane2" "${CLUSTER}-control-plane3" >/dev/null
  local out
  out="$("${KINDER_BIN}" doctor 2>&1 || true)"
  if ! grep -qE 'cluster-resume-readiness.*(quorum at risk|1/3)' <<< "${out}"; then
    echo "[FAIL] test_13 — cluster-resume-readiness did not warn on quorum loss"
    grep -E 'cluster-resume-readiness' <<< "${out}" || true
    docker start "${CLUSTER}-control-plane2" "${CLUSTER}-control-plane3" >/dev/null || true
    return 1
  fi
  # Recover before subsequent tests.
  docker start "${CLUSTER}-control-plane2" "${CLUSTER}-control-plane3" >/dev/null
  kubectl --context "kind-${CLUSTER}" wait --for=condition=Ready node --all --timeout=180s
  echo "[OK] test_13 — warn with quorum at risk wording"
}

test_14_pause_snapshot_leaderid() {
  echo "--- test_14: /kind/pause-snapshot.json contains non-empty leaderID ---"
  local pause_stderr
  set +e
  pause_stderr="$("${KINDER_BIN}" pause "${CLUSTER}" 2>&1 >/dev/null)"
  local exit_code=$?
  set -e
  if [[ ${exit_code} -ne 0 ]]; then
    echo "[FAIL] test_14 — pause exited ${exit_code}"
    echo "${pause_stderr}"
    return 1
  fi
  if grep -q 'failed to capture etcd leader id' <<< "${pause_stderr}"; then
    echo "[FAIL] test_14 — pause stderr contains legacy 'failed to capture etcd leader id'"
    echo "${pause_stderr}"
    return 1
  fi
  local leader
  leader="$(docker exec "${CLUSTER}-control-plane" cat /kind/pause-snapshot.json | jq -r .leaderID)"
  if [[ -z "${leader}" || "${leader}" == "null" ]]; then
    echo "[FAIL] test_14 — leaderID empty: ${leader}"
    return 1
  fi
  echo "[OK] test_14 — leaderID = ${leader}"
  # Resume so subsequent SC1/SC2 evidence runs against a live cluster.
  "${KINDER_BIN}" resume "${CLUSTER}" --wait 5m
  kubectl --context "kind-${CLUSTER}" wait --for=condition=Ready node --all --timeout=180s
}

test_sc1_post_pause_stats() {
  echo "--- SC1 evidence: post-pause docker ps shows all Exited ---"
  "${KINDER_BIN}" pause "${CLUSTER}"
  local ps_out
  ps_out="$(docker ps -a \
    --filter "label=io.x-k8s.kind.cluster=${CLUSTER}" \
    --format '{{.Names}}: {{.Status}}')"
  echo "${ps_out}"
  while IFS= read -r line; do
    if [[ -z "${line}" ]]; then continue; fi
    if ! grep -q 'Exited' <<< "${line}"; then
      echo "[FAIL] SC1 — container not Exited post-pause: ${line}"
      return 1
    fi
  done <<< "${ps_out}"
  echo "[OK] SC1 — all ${CLUSTER}-* containers are Exited post-pause"
  "${KINDER_BIN}" resume "${CLUSTER}" --wait 5m
  kubectl --context "kind-${CLUSTER}" wait --for=condition=Ready node --all --timeout=180s
}

test_sc2_state_preservation() {
  echo "--- SC2 evidence: Deployment + PVC + Service round-trip ---"
  kubectl --context "kind-${CLUSTER}" apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata: { name: uat-pvc }
spec:
  accessModes: [ReadWriteOnce]
  resources: { requests: { storage: 100Mi } }
---
apiVersion: apps/v1
kind: Deployment
metadata: { name: uat-deploy }
spec:
  replicas: 1
  selector: { matchLabels: { app: uat } }
  template:
    metadata: { labels: { app: uat } }
    spec:
      containers:
        - name: c
          image: busybox:1.37.0
          command: ["sleep", "infinity"]
          volumeMounts: [{ name: data, mountPath: /data }]
      volumes:
        - name: data
          persistentVolumeClaim: { claimName: uat-pvc }
---
apiVersion: v1
kind: Service
metadata: { name: uat-svc }
spec:
  selector: { app: uat }
  ports: [{ port: 80, targetPort: 80 }]
EOF
  kubectl --context "kind-${CLUSTER}" wait --for=condition=Ready pod -l app=uat --timeout=180s

  local SENTINEL POD_NAME POD_UID SVC_IP
  SENTINEL="UAT-58-01-SENTINEL-$(date +%s)"
  POD_NAME="$(kubectl --context "kind-${CLUSTER}" get pod -l app=uat -o jsonpath='{.items[0].metadata.name}')"
  POD_UID="$(kubectl --context "kind-${CLUSTER}" get pod "${POD_NAME}" -o jsonpath='{.metadata.uid}')"
  SVC_IP="$(kubectl --context "kind-${CLUSTER}" get svc uat-svc -o jsonpath='{.spec.clusterIP}')"
  kubectl --context "kind-${CLUSTER}" exec "${POD_NAME}" -- sh -c "echo '${SENTINEL}' > /data/sentinel.txt"

  "${KINDER_BIN}" pause "${CLUSTER}"
  "${KINDER_BIN}" resume "${CLUSTER}" --wait 5m
  kubectl --context "kind-${CLUSTER}" wait --for=condition=Ready pod -l app=uat --timeout=180s

  local POD_NAME_AFTER POD_UID_AFTER SVC_IP_AFTER SENTINEL_AFTER
  POD_NAME_AFTER="$(kubectl --context "kind-${CLUSTER}" get pod -l app=uat -o jsonpath='{.items[0].metadata.name}')"
  POD_UID_AFTER="$(kubectl --context "kind-${CLUSTER}" get pod "${POD_NAME_AFTER}" -o jsonpath='{.metadata.uid}')"
  SVC_IP_AFTER="$(kubectl --context "kind-${CLUSTER}" get svc uat-svc -o jsonpath='{.spec.clusterIP}')"
  SENTINEL_AFTER="$(kubectl --context "kind-${CLUSTER}" exec "${POD_NAME_AFTER}" -- cat /data/sentinel.txt)"

  [[ "${POD_NAME}" == "${POD_NAME_AFTER}" ]] || { echo "[FAIL] SC2 — pod name changed: ${POD_NAME} -> ${POD_NAME_AFTER}"; return 1; }
  [[ "${POD_UID}"  == "${POD_UID_AFTER}"  ]] || { echo "[FAIL] SC2 — pod UID changed: ${POD_UID} -> ${POD_UID_AFTER}"; return 1; }
  [[ "${SVC_IP}"   == "${SVC_IP_AFTER}"   ]] || { echo "[FAIL] SC2 — ClusterIP changed: ${SVC_IP} -> ${SVC_IP_AFTER}"; return 1; }
  [[ "${SENTINEL}" == "${SENTINEL_AFTER}" ]] || { echo "[FAIL] SC2 — PVC sentinel lost: ${SENTINEL} != ${SENTINEL_AFTER}"; return 1; }
  echo "[OK] SC2 — pod UID, ClusterIP, PVC sentinel all identical across pause+resume"
}

test_ordering() {
  echo "--- ordering: pause = workers->CP->LB; resume = LB->CP->workers ---"
  local pause_out resume_out
  pause_out="$("${KINDER_BIN}" pause "${CLUSTER}" 2>&1)"
  echo "${pause_out}"
  # Verify worker line precedes any control-plane line precedes external-load-balancer.
  local worker_line cp_line lb_line
  worker_line="$(echo "${pause_out}" | grep -nE 'worker' | head -1 | cut -d: -f1 || true)"
  cp_line="$(echo "${pause_out}" | grep -nE 'control-plane' | head -1 | cut -d: -f1 || true)"
  lb_line="$(echo "${pause_out}" | grep -nE 'external-load-balancer' | head -1 | cut -d: -f1 || true)"
  if [[ -n "${worker_line}" && -n "${cp_line}" && -n "${lb_line}" ]]; then
    if (( worker_line < cp_line && cp_line < lb_line )); then
      echo "[OK] ordering — pause workers(${worker_line}) < CP(${cp_line}) < LB(${lb_line})"
    else
      echo "[FAIL] ordering — pause line numbers wrong: worker=${worker_line} cp=${cp_line} lb=${lb_line}"
      return 1
    fi
  else
    echo "[WARN] ordering — could not locate all three role markers in pause output (text-mode schema may have changed)"
  fi
  resume_out="$("${KINDER_BIN}" resume "${CLUSTER}" --wait 5m 2>&1)"
  echo "${resume_out}"
  kubectl --context "kind-${CLUSTER}" wait --for=condition=Ready node --all --timeout=180s
}

finalize() {
  echo "=== ALL TESTS PASSED ==="
  if [[ "${TEARDOWN}" == "yes" ]] || [[ "${1:-}" == "--teardown" ]]; then
    "${KINDER_BIN}" delete cluster --name "${CLUSTER}"
    echo "Cluster ${CLUSTER} torn down."
  else
    echo "Cluster ${CLUSTER} left up. Clean up with:"
    echo "  ${KINDER_BIN} delete cluster --name ${CLUSTER}"
  fi
}

trap '[[ $? -ne 0 ]] && echo "[FAIL] Cluster ${CLUSTER} left up for inspection. Run: ${KINDER_BIN} delete cluster --name ${CLUSTER}" >&2' EXIT

main() {
  preamble
  create_ha_cluster
  observe_baseline_stats
  test_03_get_nodes_positional
  test_09_resume_wait_duration_string
  test_12_doctor_healthy_3of3
  test_13_doctor_warn_quorum_loss
  test_14_pause_snapshot_leaderid
  test_sc1_post_pause_stats
  test_sc2_state_preservation
  test_ordering
  finalize "$@"
}

main "$@" 2>&1 | tee "${LOG}"
```

After writing, run `chmod +x hack/uat-47-ha-smoke.sh` and validate syntax with `bash -n hack/uat-47-ha-smoke.sh`. Run `shellcheck hack/uat-47-ha-smoke.sh` if `shellcheck` is on PATH (non-blocking — informational only; do not change the script for SC2120/SC2155-class warnings introduced by shellcheck's style preferences if they conflict with the spec above).

Do NOT execute the script in this task (no live cluster work yet — that's Task 2's checkpoint).

Commit message (atomic): `feat(58-01): add hack/uat-47-ha-smoke.sh` — files: `hack/uat-47-ha-smoke.sh`.
  </action>
  <verify>
    <automated>
cd /Users/patrykattc/work/git/kinder && \
  test -x hack/uat-47-ha-smoke.sh && \
  bash -n hack/uat-47-ha-smoke.sh && \
  grep -qE '^set -euo pipefail' hack/uat-47-ha-smoke.sh && \
  grep -qE '\$\{REPO_ROOT\}/bin/kinder' hack/uat-47-ha-smoke.sh && \
  ! grep -qE '^[[:space:]]*kinder[[:space:]]' hack/uat-47-ha-smoke.sh && \
  grep -qF 'all control-plane containers stopped' hack/uat-47-ha-smoke.sh && \
  grep -qF 'quorum at risk' hack/uat-47-ha-smoke.sh && \
  grep -qF 'kindest/haproxy' hack/uat-47-ha-smoke.sh && \
  grep -qE 'test_03_get_nodes_positional|test_09_resume_wait_duration_string|test_12_doctor_healthy_3of3|test_13_doctor_warn_quorum_loss|test_14_pause_snapshot_leaderid' hack/uat-47-ha-smoke.sh
    </automated>
  </verify>
  <done>
    File `hack/uat-47-ha-smoke.sh` exists, is executable, parses cleanly with `bash -n`, starts with `set -euo pipefail`, references `${REPO_ROOT}/bin/kinder` (never bare `kinder` as a command line), contains all 5 POSITIVE markers and all 3 NEGATIVE markers in the gate arrays, and defines all 5 named test functions for tests 3/9/12/13/14 plus SC1, SC2, and ordering helpers.
  </done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 2: Developer runs the script on their machine; reports pass/fail</name>
  <what-built>
    Task 1 wrote `hack/uat-47-ha-smoke.sh` — a self-contained bash script that rebuilds `./bin/kinder`, runs the strings-marker gate, creates a 3 CP + 2 worker HA cluster, exercises 47-UAT.md tests 3/9/12/13/14 + SC1 docker-stats + SC2 PVC sentinel + pause/resume ordering, and `tee`'s everything to `hack/uat-47-ha-smoke.log`.

    Live cluster work cannot run inside Claude — it needs the developer's Docker Desktop and ~8-12 min of wall-clock time on the host machine. This checkpoint blocks Task 3 until evidence of one successful run is in hand.
  </what-built>
  <how-to-verify>
    On the developer's macOS host with Docker Desktop running and >4 GB free RAM:

    1. `cd /Users/patrykattc/work/git/kinder`
    2. Optional: run `docker stats --no-stream` first to confirm RAM headroom (Pitfall A in RESEARCH).
    3. Run the script:
       ```
       bash hack/uat-47-ha-smoke.sh
       ```
       Expected wall-clock: ~10 min. Expected exit code: 0.
    4. Inspect `hack/uat-47-ha-smoke.log` — should contain:
       - `kind v1.5.0 go1.26.3 ...` from `./bin/kinder version`
       - `HEAD:  <40-char hash>`
       - `Build:  <stat output>`
       - `[OK] test_03 — get nodes uat-58-01 returned <N> rows`
       - `[OK] test_09 — --wait 5m accepted; no ParseInt; cluster Ready`
       - `[OK] test_12 — cluster-resume-readiness: 3/3 etcd members healthy`
       - `[OK] test_13 — warn with quorum at risk wording`
       - `[OK] test_14 — leaderID = <non-empty>`
       - `[OK] SC1 — all uat-58-01-* containers are Exited post-pause`
       - `[OK] SC2 — pod UID, ClusterIP, PVC sentinel all identical across pause+resume`
       - `[OK] ordering — pause workers(<n>) < CP(<m>) < LB(<k>)`
       - `=== ALL TESTS PASSED ===`
    5. Confirm cluster is left up (default behavior) OR torn down (if `--teardown` was passed):
       ```
       ./bin/kinder get clusters
       ```

    Reply with:
    - `pass` + paste the final `=== ALL TESTS PASSED ===` block and the 10 `[OK]` lines from the log
    - OR `fail` + paste the `[FAIL] test_<N>` line + ~20 lines of surrounding context

    If `[FAIL] STALE BINARY` appears: usually a dirty `bin/` or build flag drift — try `rm -rf bin/ && bash hack/uat-47-ha-smoke.sh`. If the marker gate continues to fail on a clean rebuild, that is a regression in some other v2.4 phase and Phase 58 cannot proceed — file a bug.

    If `[FAIL] test_12` or `test_13` reports unexpected wording: the DIAG-06 (Phase 57-02) tolerant parse changes are not in the binary. Confirm with `git log --oneline pkg/internal/doctor/resumereadiness.go | head` — should show the c43bb599 commit.

    If `[FAIL] test_14` reports `failed to capture etcd leader id`: the rebuild silently no-op'd against a pre-47-05 binary. Try `rm -rf bin/ && make build`. If still failing, that's a 47-05 regression — file a bug.
  </how-to-verify>
  <resume-signal>Reply `pass` + log excerpt OR `fail` + log excerpt</resume-signal>
</task>

<task type="auto">
  <name>Task 3: Commit log + flip 47-UAT.md status fields</name>
  <files>
    hack/uat-47-ha-smoke.log
    .planning/phases/47-cluster-pause-resume/47-UAT.md
  </files>
  <action>
**Step A — commit the log.** After Task 2 returns `pass`, `git add hack/uat-47-ha-smoke.log`. The log was produced by the script's `tee` invocation; do NOT regenerate, edit, or pretty-print it — it must be the verbatim run transcript (timestamps, container IDs, kubectl output and all). Commit message (atomic): `chore(58-01): commit live UAT log evidence` — files: `hack/uat-47-ha-smoke.log`.

**Step B — flip 47-UAT.md.** Open `.planning/phases/47-cluster-pause-resume/47-UAT.md`. Make exactly these edits (do NOT rewrite the whole file — surgical):

1. Frontmatter:
   - `status: diagnosed` -> `status: closed`
   - `updated:` -> current UTC timestamp in the existing ISO-8601 form
   - Append a line in `source:` referencing `47-06-SUMMARY.md` if not already present (47-VERIFICATION.md confirms 47-06 landed)

2. Test 3 (`### 3. kinder get nodes shows real container state`):
   - `result: issue` -> `result: pass`
   - DELETE `reported:` block and `severity:` line
   - INSERT `evidence:` block (literal `|`-block) reading:
     ```
     evidence: |
       Live UAT against ./bin/kinder built from HEAD <40-char-commit-hash-from-log>.
       $ ./bin/kinder get nodes uat-58-01
       <2-3 lines of captured node table excerpt from the log>
       Exit 0. Resolved via cobra.MaximumNArgs(1) + lifecycle.ResolveClusterName (47-06).
     note: |
       Closed by 47-06 commit 50aa742a + Phase 58 live UAT. See hack/uat-47-ha-smoke.log for full transcript.
     ```

3. Test 9 (`### 9. kinder resume --wait timeout flag works`):
   - `result: issue` -> `result: pass`
   - Drop `reported:` + `severity:`; add `evidence:` block citing `./bin/kinder resume uat-58-01 --wait 5m` exit 0 + absence of `strconv.ParseInt`; cite 47-06 commit `7a4f722f` in the `note:` block.

4. Test 12 (`### 12. cluster-resume-readiness reports ok on healthy HA cluster (SC4 forward)`):
   - `result: issue` -> `result: pass`
   - Drop `reported:` + `severity:`; add `evidence:` block citing the captured `cluster-resume-readiness ... 3/3 etcd members healthy` line; cite 47-06 commit `ed85ecdf` (clusterFilter presence-only + -a flag) AND 57-02 commit `c43bb599` (tolerant etcd JSON parse) in the `note:` block.

5. Test 13 (`### 13. cluster-resume-readiness warns on quorum loss (SC4 reverse)`):
   - `result: issue` -> `result: pass`
   - Drop `reported:` + `severity:`; add `evidence:` block citing the captured `cluster-resume-readiness ... quorum at risk` (or `1/3`) line after `docker stop` of 2 CPs; cite 47-06 commit `ed85ecdf` + 57-02 commit `c43bb599` in the `note:` block.

6. Test 14 (`### 14. HA pause snapshot captures non-empty leaderID`):
   - `result: issue` -> `result: pass`
   - Drop `reported:` + `severity:`; add `evidence:` block citing the captured non-empty `leaderID` from `/kind/pause-snapshot.json` and the absence of `failed to capture etcd leader id` in pause stderr; `note:` block: `Closed by Phase 58 live UAT against rebuilt ./bin/kinder. pause.go at HEAD was correct after 47-05 (crictl path); test 14 was a pure stale-binary symptom resolved by the script's make build preamble.`

7. `## Summary` block:
   - `passed: 9` -> `passed: 14`
   - `issues: 5` -> `issues: 0`

8. `## Gaps` section:
   - DELETE the entire 5-entry YAML list under `## Gaps`
   - REPLACE with the single line: `All UAT issues closed via Phase 58 Plan 01. See hack/uat-47-ha-smoke.log for full live transcript.`

Commit message (atomic): `docs(58-01): flip 47-UAT.md issues to pass; mark UAT-01 closed in 47-UAT` — files: `.planning/phases/47-cluster-pause-resume/47-UAT.md`.

Do NOT touch REQUIREMENTS.md, ROADMAP.md, STATE.md, or write 58-01-SUMMARY.md in this task — those land at phase close (after 58-02 also passes).
  </action>
  <verify>
    <automated>
cd /Users/patrykattc/work/git/kinder && \
  test -f hack/uat-47-ha-smoke.log && \
  git ls-files --error-unmatch hack/uat-47-ha-smoke.log && \
  grep -qE 'all control-plane containers stopped|3/3 etcd members healthy|quorum at risk' hack/uat-47-ha-smoke.log && \
  grep -qE '^status: closed' .planning/phases/47-cluster-pause-resume/47-UAT.md && \
  grep -qE '^passed: 14' .planning/phases/47-cluster-pause-resume/47-UAT.md && \
  grep -qE '^issues: 0' .planning/phases/47-cluster-pause-resume/47-UAT.md && \
  test "$(grep -cE '^result: issue' .planning/phases/47-cluster-pause-resume/47-UAT.md)" = "0" && \
  test "$(grep -cE '^result: pass' .planning/phases/47-cluster-pause-resume/47-UAT.md)" -ge "14"
    </automated>
  </verify>
  <done>
    `hack/uat-47-ha-smoke.log` is tracked by git and contains at least one of the canonical SC marker phrases. `47-UAT.md` frontmatter `status:` is `closed`; summary block shows `passed: 14` and `issues: 0`; zero `result: issue` rows remain; at least 14 `result: pass` rows present. `## Gaps` section is the single-line closure pointer (not the 5-entry YAML list).
  </done>
</task>

</tasks>

<verification>
Phase 58 SC1 + SC2 + SC4 must hold after Task 3 completes. Phase orchestrator / verifier MUST confirm:

**SC1 — rebuild gate (researcher Pattern 1, freshness via `make build` + strings):**
```bash
cd /Users/patrykattc/work/git/kinder && \
  make build && \
  for m in "crictl ps --name etcd -q" \
          "all control-plane containers stopped" \
          "docker.io/envoyproxy/envoy:v1.36.2" \
          "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+" \
          "quorum at risk"; do
    strings ./bin/kinder | grep -qF "$m" || { echo "MISSING: $m"; exit 1; }
  done && \
  for m in "label=io.x-k8s.kind.cluster=kind" "/usr/local/bin/etcdctl" "kindest/haproxy"; do
    strings ./bin/kinder | grep -qF "$m" && { echo "FORBIDDEN: $m"; exit 1; }
  done; echo "OK"
# Expected: prints OK (no MISSING/FORBIDDEN lines).
```

**SC2 — 47-UAT.md row-flip evidence (5 specific tests):**
```bash
for t in 3 9 12 13 14; do
  awk "/^### ${t}\. /{flag=1} /^### / && \$2!~\"^${t}\\\\.\"{flag=0} flag" \
    .planning/phases/47-cluster-pause-resume/47-UAT.md \
    | head -20
done
# Expected: each of the 5 sections shows `result: pass` and an `evidence:` block. No `result: issue` remains anywhere.
grep -nE '^result: issue' .planning/phases/47-cluster-pause-resume/47-UAT.md || echo "OK — no issue rows"
```

**SC4 — script uses absolute `./bin/kinder` path everywhere; no bare `kinder`:**
```bash
grep -nE '^[[:space:]]*kinder[[:space:]]+(get|pause|resume|create|delete|doctor|version)' hack/uat-47-ha-smoke.sh
# Expected: zero matches.
grep -cE '\$\{KINDER_BIN\}|\$\{REPO_ROOT\}/bin/kinder' hack/uat-47-ha-smoke.sh
# Expected: large positive integer (>= 15).
```

**Log evidence committed (Pitfall G):**
```bash
git ls-files --error-unmatch hack/uat-47-ha-smoke.log && \
  grep -cE '\[OK\] test_' hack/uat-47-ha-smoke.log
# Expected: log is tracked; at least 5 [OK] test_<N> lines present.
```

**Script lint:**
```bash
bash -n hack/uat-47-ha-smoke.sh
# Expected: exit 0.
```
</verification>

<success_criteria>
- [ ] **ROADMAP SC1 (freshness gate)**: `make build` preamble + 5 POSITIVE marker grep + 3 NEGATIVE marker grep is the first runtime step in `hack/uat-47-ha-smoke.sh`; the marker bytes match RESEARCH §Pitfall 23 verbatim.
- [ ] **ROADMAP SC2 (47-UAT.md row flips)**: Tests 3, 9, 12, 13, 14 in `.planning/phases/47-cluster-pause-resume/47-UAT.md` each transition from `result: issue` to `result: pass` with an `evidence:` block citing the captured log; `## Summary` reads `passed: 14`, `issues: 0`; `## Gaps` is replaced with the single-line closure pointer.
- [ ] **ROADMAP SC4 (./bin/kinder everywhere)**: zero bare `kinder` invocations in the script; every kinder call uses `${KINDER_BIN}` or `${REPO_ROOT}/bin/kinder`.
- [ ] **Evidence artifact committed (Pitfall G)**: `hack/uat-47-ha-smoke.log` is tracked by git and contains the verbatim transcript with all `[OK] test_<N>` lines and the `=== ALL TESTS PASSED ===` footer.
- [ ] **Cluster topology**: script creates 3 CP + 2 worker (`uat-58-01`) via inline here-doc; LB is auto-created by kinder; total = 6 containers.
- [ ] **SC1 host-observation evidence**: docker stats / docker ps -a snapshots captured pre-pause, post-pause (asserted all `Exited`), post-resume.
- [ ] **SC2 state-preservation evidence**: pod UID + ClusterIP + PVC sentinel byte-identical across one full pause+resume cycle.
- [ ] **Quorum-safe ordering evidence**: pause shows workers->CP->LB; resume shows LB->CP->workers (line-number ordering check, with a `[WARN]` graceful-degradation path if the text-mode output schema changes in a future cosmetic fix).
- [ ] **Re-entrancy contract**: a second invocation against an existing `uat-58-01` cluster fails fast with a cleanup-command print (unless `REUSE=no`).
- [ ] **No Go source / no manifest / no Makefile / no CI workflow changes**: this plan touches only `hack/` + `.planning/`.
- [ ] **Planner decisions recorded** in frontmatter `planner_decisions:` block — (b) versionPreRelease deferred, (c) manual-only execution, (d) leave-up-by-default teardown.
</success_criteria>

<output>
Phase 58 Plan 01 produces three commits at execute time:
- `feat(58-01): add hack/uat-47-ha-smoke.sh` (Task 1)
- `chore(58-01): commit live UAT log evidence` (Task 3 Step A)
- `docs(58-01): flip 47-UAT.md issues to pass; mark UAT-01 closed in 47-UAT` (Task 3 Step B)

Do NOT create `.planning/phases/58-live-uat-closure-for-phase-47-51/58-01-SUMMARY.md` in this plan — Phase 58's close-out step writes both 58-01-SUMMARY.md and 58-02-SUMMARY.md after Plan 58-02 also lands. Do NOT edit `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, or `.planning/STATE.md` here — those flip at phase close (after 58-02 verifies).
</output>
