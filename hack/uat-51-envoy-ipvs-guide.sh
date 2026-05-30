#!/usr/bin/env bash
# Source: hack/uat-51-envoy-ipvs-guide.sh — Phase 58 Plan 02
# Closes: REQUIREMENTS.md UAT-02 + re-verifies 51-UAT.md tests 1/2/3 against the v2.4 binary
# Pitfall 23 gate: rebuilds ./bin/kinder against current HEAD on every invocation.
# Hard contract: ALWAYS uses absolute ${REPO_ROOT}/bin/kinder; NEVER bare `kinder` from $PATH.
# Default: leaves cluster up on failure (and on success unless --teardown).
# Re-runs: detect leftover uat-58-02 cluster; set REUSE=no to auto-delete before recreate.

set -euo pipefail

CLUSTER="uat-58-02"
LOG="${LOG:-hack/uat-51-envoy-ipvs-guide.log}"
TEARDOWN="${TEARDOWN:-no}"
REUSE="${REUSE:-prompt}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
KINDER_BIN="${REPO_ROOT}/bin/kinder"

DEV_PID=""
TMP_IPVS_CONFIG=""

cleanup() {
  if [[ -n "${DEV_PID}" ]] && kill -0 "${DEV_PID}" 2>/dev/null; then
    kill -TERM "${DEV_PID}" 2>/dev/null || true
    wait "${DEV_PID}" 2>/dev/null || true
  fi
  if [[ -n "${TMP_IPVS_CONFIG}" && -f "${TMP_IPVS_CONFIG}" ]]; then
    rm -f "${TMP_IPVS_CONFIG}"
  fi
}
trap cleanup EXIT INT TERM

# ---------- preamble ----------

preamble() {
  cd "${REPO_ROOT}"
  echo "=== Phase 58 Plan 02 — Phase 51 Envoy LB + IPVS guard + 1.36 guide live UAT ==="
  echo "ETA ~7 min; cluster ${CLUSTER}; log ${LOG}"
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

  # 2) POSITIVE marker gate (same set as 58-01)
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

  # 4) Astro dev-server port preflight (Pitfall C)
  if command -v lsof >/dev/null 2>&1 && lsof -i :4321 >/dev/null 2>&1; then
    echo "[FAIL] Port 4321 is already bound — astro dev server cannot start. Kill the existing process and re-run." >&2
    lsof -i :4321 >&2 || true
    exit 1
  fi

  # 5) Document version + path + Docker capacity
  "${KINDER_BIN}" version
  echo "Using: ${KINDER_BIN}"
  echo "Build: $(stat -f '%Sm' "${KINDER_BIN}" 2>/dev/null || stat -c '%y' "${KINDER_BIN}")"
  printf 'PATH-resolved kinder: '
  which kinder || echo "(not in PATH — OK)"
  echo "--- docker info (RAM headroom) ---"
  docker info 2>/dev/null | grep -E 'Total Memory|CPUs' || true

  # 6) Re-entrancy: detect leftover cluster
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

# ---------- tests ----------

test_01_envoy_lb_on_ha_cluster() {
  echo "--- test_01: HA cluster LB is Envoy (no HAProxy) ---"
  "${KINDER_BIN}" create cluster --name "${CLUSTER}" --config - <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: control-plane
  - role: worker
EOF
  kubectl --context "kind-${CLUSTER}" wait --for=condition=Ready node --all --timeout=180s

  local lb_image
  lb_image="$(docker ps \
    --filter "label=io.x-k8s.kind.cluster=${CLUSTER}" \
    --filter "name=external-load-balancer" \
    --format '{{.Image}}')"
  if [[ "${lb_image}" != "docker.io/envoyproxy/envoy:v1.36.2" ]]; then
    echo "[FAIL] test_01 — LB image = '${lb_image}', want 'docker.io/envoyproxy/envoy:v1.36.2'"
    return 1
  fi
  if docker ps -a --format '{{.Image}}' | grep -q 'kindest/haproxy'; then
    echo "[FAIL] test_01 — kindest/haproxy container present (must be absent post-51-01)"
    docker ps -a --format '{{.Image}}' | grep 'kindest/haproxy'
    return 1
  fi
  echo "[OK] test_01 — Envoy LB (${lb_image}) present; HAProxy absent"
}

test_02_ipvs_1_36_rejected_at_validate() {
  echo "--- test_02: kubeProxyMode:ipvs + 1.36 image rejected at validate ---"
  TMP_IPVS_CONFIG="$(mktemp -t ipvs-1-36-test.XXXXXX.yaml)"
  cat > "${TMP_IPVS_CONFIG}" <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  kubeProxyMode: ipvs
nodes:
  - role: control-plane
    image: kindest/node:v1.36.0
EOF

  local stderr_capture exit_code
  set +e
  stderr_capture="$("${KINDER_BIN}" create cluster --config "${TMP_IPVS_CONFIG}" --name should-not-exist 2>&1 >/dev/null)"
  exit_code=$?
  set -e

  if [[ ${exit_code} -eq 0 ]]; then
    echo "[FAIL] test_02 — kinder did NOT reject ipvs+1.36 config (exit 0)"
    return 1
  fi

  local required=(
    "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+"
    "kube-proxy IPVS mode was deprecated in v1.35"
    "Switch to iptables or nftables"
    "https://kubernetes.io/docs/reference/networking/virtual-ips/"
  )
  for s in "${required[@]}"; do
    if ! grep -qF "${s}" <<< "${stderr_capture}"; then
      echo "[FAIL] test_02 — stderr missing required substring: ${s}"
      echo "Captured stderr:"
      echo "${stderr_capture}"
      return 1
    fi
  done

  if docker ps -a --filter "name=should-not-exist" --format '{{.Names}}' | grep -q .; then
    echo "[FAIL] test_02 — kinder created a container despite validation rejection"
    return 1
  fi

  echo "[OK] test_02 — ipvs+1.36 rejected (exit ${exit_code}); all 4 required substrings present; no container created"
}

test_03_k8s_1_36_guide_renders() {
  echo "--- test_03: kinder-site k8s-1-36-whats-new guide page renders ---"
  if ! command -v npm >/dev/null 2>&1; then
    echo "[SKIP] test_03 — npm not installed; rendering deferred to a machine with kinder-site toolchain"
    return 0
  fi

  pushd "${REPO_ROOT}/kinder-site" >/dev/null
  if [[ ! -d node_modules ]]; then
    echo "Installing kinder-site dependencies..."
    npm install
  fi

  # Start astro dev in background; capture PID for cleanup trap.
  npm run dev >/tmp/uat-58-02-astro.log 2>&1 &
  DEV_PID=$!

  # Poll for HTTP 200
  local i success=0
  for i in {1..30}; do
    if curl -sf -o /dev/null --max-time 2 http://localhost:4321/guides/k8s-1-36-whats-new/; then
      success=1
      break
    fi
    sleep 2
  done
  if [[ ${success} -eq 0 ]]; then
    echo "[FAIL] test_03 — astro dev server did not serve /guides/k8s-1-36-whats-new/ within 60s"
    cat /tmp/uat-58-02-astro.log || true
    popd >/dev/null
    return 1
  fi

  local body
  body="$(curl -sf http://localhost:4321/guides/k8s-1-36-whats-new/)"
  popd >/dev/null

  if ! grep -qF "User Namespaces" <<< "${body}"; then
    echo "[FAIL] test_03 — response body missing 'User Namespaces'"
    return 1
  fi
  if ! grep -qF "In-Place Pod Resize" <<< "${body}"; then
    echo "[FAIL] test_03 — response body missing 'In-Place Pod Resize'"
    return 1
  fi

  echo "[OK] test_03 — guide page renders; both GA-feature headings present; HTTP 200"
  # Kill dev server proactively (trap will also handle it).
  kill -TERM "${DEV_PID}" 2>/dev/null || true
  wait "${DEV_PID}" 2>/dev/null || true
  DEV_PID=""
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

trap '[[ $? -ne 0 ]] && echo "[FAIL] Cluster ${CLUSTER} may be left up for inspection. Run: ${KINDER_BIN} delete cluster --name ${CLUSTER}" >&2' ERR

main() {
  preamble
  test_01_envoy_lb_on_ha_cluster
  test_02_ipvs_1_36_rejected_at_validate
  test_03_k8s_1_36_guide_renders
  finalize "$@"
}

main "$@" 2>&1 | tee "${LOG}"
