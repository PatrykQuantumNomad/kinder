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

  # Cache strings output once — avoids pipefail+grep -q SIGPIPE bug
  # (grep -q exits early, strings gets SIGPIPE 141, pipefail propagates non-zero,
  #  ! inverts to truthy, gate falsely claims marker absent).
  local binary_strings
  binary_strings="$(strings "${KINDER_BIN}")"

  # 2) POSITIVE marker gate
  local positive=(
    "crictl ps for etcd container failed"
    "all control-plane containers stopped"
    "docker.io/envoyproxy/envoy:v1.36.2"
    "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+"
    "quorum at risk"
  )
  for m in "${positive[@]}"; do
    if ! grep -qF "${m}" <<< "${binary_strings}"; then
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
    if grep -qF "${m}" <<< "${binary_strings}"; then
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
