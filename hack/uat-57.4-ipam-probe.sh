#!/usr/bin/env bash
# hack/uat-57.4-ipam-probe.sh — live UAT for Phase 57.4.
# Verifies the IPAM probe returns verdict=ip-pinned on a healthy host after the
# --privileged fix. Covers:
#   (a) ./bin/kinder doctor ipam-probe returns verdict=ip-pinned
#   (b) ./bin/kinder create cluster --config <ha.yaml> emits the ip-pin announcement
#   (c) docker inspect ... resume-strategy label is 'ip-pinned' on cp1
#
# 4-ENV MATRIX (record results in uat-logs/57.4-01-uat-<env>.txt per env):
#   1. macOS Docker Desktop (the regressing env — closes bug-of-record)
#   2. Linux Docker          (sanity baseline — confirm fix doesn't break working path)
#   3. macOS nerdctl         (VerdictUnsupported short-circuit must still trigger)
#   4. macOS podman          (podman path must remain healthy)
#
# PREREQUISITES:
#   - Docker Desktop running (IPv4 only required for basic probe; IPv6 for full matrix).
#   - ./bin/kinder built from a HEAD that includes Phase 57.4 Plan 01
#     (string markers checked in the preamble's freshness gate).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
KINDER_BIN="${KINDER_BIN:-${REPO_ROOT}/bin/kinder}"
LOG_DIR="${REPO_ROOT}/.planning/phases/57.4-ipam-probe-regression/uat-logs"
mkdir -p "${LOG_DIR}"

# Cluster name used for the create-cluster label assertion (test_03).
CLUSTER_NAME="${UAT_CLUSTER_NAME:-uat-574-ipam}"

log()  { echo "[$(date '+%H:%M:%S')] $*" | tee -a "${LOG_DIR}/run.log"; }
fail() { log "FAIL: $*"; SCRIPT_FAILED=1; exit 1; }

# SCRIPT_FAILED starts at 1; only main() flips it to 0 after all asserts.
# The cleanup trap preserves the cluster on any non-zero exit (including panic).
SCRIPT_FAILED=1
cleanup() {
    if [ "${SCRIPT_FAILED}" = "1" ]; then
        log "cleanup: SKIPPED — cluster preserved for forensics"
        log "  inspect with: docker ps --filter label=io.x-k8s.kind.cluster"
        log "  manual delete: ${KINDER_BIN} delete cluster --name ${CLUSTER_NAME}"
        return
    fi
    log "cleanup: deleting test cluster ${CLUSTER_NAME}"
    "${KINDER_BIN}" delete cluster --name "${CLUSTER_NAME}" 2>/dev/null || true
}
trap cleanup EXIT

preamble() {
    cd "${REPO_ROOT}"
    log "=== Phase 57.4 IPAM probe live UAT ==="
    log "HEAD: $(git rev-parse HEAD)"
    log "macOS: $(sw_vers -productVersion 2>/dev/null || uname -s)"
    log "Docker: $(docker version --format '{{.Server.Version}}' 2>/dev/null || echo unknown)"
    log "make build (rebuild against current HEAD)"
    make build 2>&1 | tee -a "${LOG_DIR}/run.log"

    # Cache strings(1) output ONCE — Pitfall-23 SIGPIPE bug per uat-47-ha-smoke.sh:33-35.
    local binary_strings
    binary_strings="$(strings "${KINDER_BIN}")"

    # Binary markers that MUST be present in a Phase 57.4 binary.
    local positive=(
        "IPAM probe returned unparseable IP"
        "this is likely a regression; please file an issue"
        "HA cluster will use ip-pin resume strategy"
        "IPAM supports IP pinning (ip-pinned path available)"
    )
    for m in "${positive[@]}"; do
        if ! grep -qF -- "${m}" <<< "${binary_strings}"; then
            fail "STALE BINARY — required 57.4 marker absent: ${m}. Run 'make build' against post-57.4-01 HEAD."
        fi
        log "  ok binary marker present: ${m}"
    done

    "${KINDER_BIN}" version 2>&1 | tee -a "${LOG_DIR}/run.log"
}

# test_01: kinder doctor returns verdict=ip-pinned for the ipam-probe check.
test_01_doctor_ipam_probe_verdict() {
    log "--- test_01: kinder doctor ipam-probe verdict ---"
    local out
    # Use --output json so we can grep the verdict field cleanly.
    set +e
    out="$("${KINDER_BIN}" doctor --output json 2>&1)"
    set -e
    log "doctor JSON output:"
    echo "${out}" | tee -a "${LOG_DIR}/run.log"

    # The ipam-probe check must be present and have status=ok (ip-pinned).
    if ! echo "${out}" | grep -qF '"ipam-probe"'; then
        fail "test_01: ipam-probe check not found in doctor JSON output"
    fi
    # The bash pre-grep '"ip-pinned"' was too strict — the verdict appears as a
    # substring inside the "message" field, not as a standalone JSON value.
    # The authoritative validation is the Python check below.
    # JSON shape: {"name":"ipam-probe","status":"ok","message":"...ip-pinned..."}
    if ! echo "${out}" | python3 -c "
import json, sys
data = json.load(sys.stdin)
checks = data.get('checks', [])
for c in checks:
    if c.get('name') == 'ipam-probe':
        if c.get('status') == 'ok' and 'ip-pinned' in c.get('message', ''):
            sys.exit(0)
        print('UNEXPECTED: ipam-probe status=%s message=%s' % (c.get('status'), c.get('message')), file=sys.stderr)
        sys.exit(1)
print('ipam-probe check not found in checks array', file=sys.stderr)
sys.exit(1)
" 2>>"${LOG_DIR}/run.log"; then
        fail "test_01: ipam-probe check did not return status=ok with ip-pinned message"
    fi
    log "test_01: PASS — kinder doctor ipam-probe returned verdict=ip-pinned (status=ok)"
}

# test_02: kinder create cluster emits the ip-pin announcement in output.
test_02_create_cluster_ippin_announcement() {
    log "--- test_02: create cluster emits ip-pin announcement ---"

    # Write a minimal 3-CP HA config.
    local config_file="${LOG_DIR}/uat-574-ha.yaml"
    cat > "${config_file}" <<'YAML'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: control-plane
- role: control-plane
YAML

    log "test_02: running kinder create cluster --name ${CLUSTER_NAME} (HA, 3 CP)"
    local create_log="${LOG_DIR}/uat-574-create.log"
    set +e
    "${KINDER_BIN}" create cluster --name "${CLUSTER_NAME}" --config "${config_file}" \
        2>&1 | tee "${create_log}"
    local rc=$?
    set -e

    if [ "${rc}" -ne 0 ]; then
        fail "test_02: kinder create cluster exited ${rc} (see ${create_log})"
    fi

    # Assert the ip-pin announcement appeared in create output.
    if ! grep -qF "HA cluster will use ip-pin resume strategy" "${create_log}"; then
        fail "test_02: 'HA cluster will use ip-pin resume strategy' not found in create output (see ${create_log})"
    fi
    log "test_02: PASS — ip-pin announcement found in create output"
}

# test_03: docker inspect resume-strategy label is 'ip-pinned' on cp1.
test_03_resume_strategy_label() {
    log "--- test_03: resume-strategy label on cp1 is ip-pinned ---"
    local cp1="${CLUSTER_NAME}-control-plane"
    local label_val
    label_val="$(docker inspect --format '{{index .Config.Labels "io.x-k8s.kinder.resume-strategy"}}' "${cp1}" 2>&1)"
    log "  resume-strategy label on ${cp1}: '${label_val}'"
    if [ "${label_val}" != "ip-pinned" ]; then
        fail "test_03: expected resume-strategy=ip-pinned on ${cp1}, got '${label_val}'"
    fi
    log "test_03: PASS — resume-strategy=ip-pinned on ${cp1}"
}

main() {
    preamble
    test_01_doctor_ipam_probe_verdict
    test_02_create_cluster_ippin_announcement
    test_03_resume_strategy_label

    log ""
    log "=== Phase 57.4 IPAM probe UAT: ALL TESTS PASSED ==="
    log ""
    log "Bug-of-record close gate satisfied (macOS Docker Desktop):"
    log "  verdict=ip-pinned on kinder doctor"
    log "  ip-pin announcement on kinder create cluster"
    log "  resume-strategy=ip-pinned label on cp1"
    log ""
    log "Next steps (Phase 58 unblock):"
    log "  1. Delete left-up uat-58-01 cluster (wrong strategy — cert-regen):"
    log "     ./bin/kinder delete cluster --name uat-58-01"
    log "  2. Re-run hack/uat-47-ha-smoke.sh on a FRESH HA cluster (ip-pin path)."
    log "     test_09 should now close Phase 58."

    SCRIPT_FAILED=0
}

main "$@"
