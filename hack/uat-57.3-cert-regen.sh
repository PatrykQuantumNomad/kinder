#!/usr/bin/env bash
# hack/uat-57.3-cert-regen.sh — live UAT for Phase 57.3.
# Verifies `./bin/kinder resume --strategy=cert-regen --wait 15m` recovers
# paused HA clusters on IPv4 + IPv6 + dual-stack stacks. Each fixture goes
# through: create -> pause -> resume(--strategy=cert-regen) -> assert nodes
# Ready + cert serials advanced.
#
# PREREQUISITES:
#   - Docker Desktop running with IPv6 enabled (required for the IPv6 and
#     dual-stack fixtures) and >=8 GB RAM allocated.
#   - ./bin/kinder built from a HEAD that includes Phase 57.3 Plan 01
#     (string markers checked in the preamble's freshness gate).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
KINDER_BIN="${KINDER_BIN:-${REPO_ROOT}/bin/kinder}"
LOG_DIR="${REPO_ROOT}/.planning/phases/57.3-ha-cluster-cert-regen-recovery/uat-logs"
mkdir -p "$LOG_DIR"

CLUSTER_IPV4="uat-573-ipv4"
CLUSTER_IPV6="uat-573-ipv6"
CLUSTER_DUAL="uat-573-dual"

log() { echo "[$(date '+%H:%M:%S')] $*" | tee -a "$LOG_DIR/run.log"; }
fail() { log "FAIL: $*"; SCRIPT_FAILED=1; exit 1; }

# SCRIPT_FAILED starts at 1; only main() flips it to 0 after all asserts.
# The cleanup trap preserves clusters on any non-zero exit (including unset).
SCRIPT_FAILED=1
cleanup() {
    if [ "$SCRIPT_FAILED" = "1" ]; then
        log "cleanup: SKIPPED — clusters preserved for forensics"
        log "  inspect with: docker ps --filter label=io.x-k8s.kind.cluster"
        log "  manual delete: $KINDER_BIN delete cluster --name <name>"
        return
    fi
    log "cleanup: deleting all 3 fixtures"
    "$KINDER_BIN" delete cluster --name "$CLUSTER_IPV4" 2>/dev/null || true
    "$KINDER_BIN" delete cluster --name "$CLUSTER_IPV6" 2>/dev/null || true
    "$KINDER_BIN" delete cluster --name "$CLUSTER_DUAL" 2>/dev/null || true
}
trap cleanup EXIT

preamble() {
    cd "${REPO_ROOT}"
    log "=== Phase 57.3 cert-regen live UAT ==="
    log "HEAD: $(git rev-parse HEAD)"
    log "macOS: $(sw_vers -productVersion 2>/dev/null || uname -s)"
    log "Docker: $(docker version --format '{{.Server.Version}}' 2>/dev/null || echo unknown)"
    log "make build (rebuild against current HEAD)"
    make build 2>&1 | tee -a "$LOG_DIR/run.log"

    # Cache strings(1) output ONCE — Pitfall-23 SIGPIPE bug per uat-47-ha-smoke.sh:33-35
    local binary_strings
    binary_strings="$(strings "${KINDER_BIN}")"

    local positive=(
        "kubeadm certs renew %s failed on %s"
        "apiserver-etcd-client"
        "--strategy="
        "Cluster state is undefined"
        "etcd ready-gate timed out"
        "apiserver healthz timed out"
    )
    for m in "${positive[@]}"; do
        if ! grep -qF -- "${m}" <<< "${binary_strings}"; then
            fail "STALE BINARY — required 57.3 marker absent: ${m}. Run 'make build' against post-57.3-01 HEAD."
        fi
        log "  ok binary marker present: ${m}"
    done

    "${KINDER_BIN}" version 2>&1 | tee -a "$LOG_DIR/run.log"
}

# capture_serials writes the openssl x509 -serial of each cert in the
# locked Phase 57.3 cert-set to the given output file.
capture_serials() {
    local cluster="$1" outfile="$2"
    docker exec "${cluster}-control-plane" bash -c '
        for f in /etc/kubernetes/pki/etcd/peer.crt \
                 /etc/kubernetes/pki/etcd/server.crt \
                 /etc/kubernetes/pki/etcd/healthcheck-client.crt \
                 /etc/kubernetes/pki/apiserver-etcd-client.crt; do
            echo -n "$f: "
            openssl x509 -noout -serial -in "$f" 2>/dev/null || echo "MISSING"
        done
    ' > "$outfile" 2>&1
}

# all_nodes_ready polls kubectl get nodes inside a 15m deadline.
all_nodes_ready() {
    local kubeconfig="$1" expected_count="$2" deadline_s="${3:-900}"
    local deadline=$(( $(date +%s) + deadline_s ))
    while [ "$(date +%s)" -lt "$deadline" ]; do
        local lines ready_count not_ready
        lines="$(kubectl --kubeconfig "$kubeconfig" get nodes --no-headers 2>/dev/null || true)"
        if [ -n "$lines" ]; then
            ready_count="$(echo "$lines" | awk '$2 == "Ready" { c++ } END { print c+0 }')"
            not_ready="$(echo "$lines" | awk '$2 != "Ready" { c++ } END { print c+0 }')"
            if [ "$ready_count" -eq "$expected_count" ] && [ "$not_ready" -eq 0 ]; then
                return 0
            fi
        fi
        sleep 10
    done
    echo "timeout: kubectl get nodes did not reach $expected_count Ready in $deadline_s s"
    kubectl --kubeconfig "$kubeconfig" get nodes 2>&1 || true
    return 1
}

# apiserver_healthz_inside_cp1 returns the HTTP status code from
# `curl -k --max-time 5 <healthzURL>` run inside the cluster's cp1 container.
apiserver_healthz_inside_cp1() {
    local cluster="$1" loopback="$2"
    docker exec "${cluster}-control-plane" sh -c \
        "curl -k -s -o /dev/null -w '%{http_code}' --max-time 5 https://${loopback}:6443/healthz"
}

# run_fixture executes the full 5-step lifecycle for one IP-family fixture.
run_fixture() {
    local fixture="$1" config_yaml="$2" loopback="$3" expected_nodes="$4"
    log "=== fixture: $fixture (loopback=$loopback, expected_nodes=$expected_nodes) ==="

    # 1. Create cluster
    log "$fixture: kinder create cluster"
    echo "$config_yaml" > "$LOG_DIR/${fixture}-config.yaml"
    "$KINDER_BIN" create cluster --name "$fixture" --config "$LOG_DIR/${fixture}-config.yaml" \
        2>&1 | tee "$LOG_DIR/${fixture}-create.log"

    # 2. Capture pre-resume cert serials on cp1
    log "$fixture: capture pre-resume cert serials"
    capture_serials "$fixture" "$LOG_DIR/${fixture}-serials-pre.txt"

    # 3. Pause
    log "$fixture: kinder pause"
    "$KINDER_BIN" pause "$fixture" 2>&1 | tee "$LOG_DIR/${fixture}-pause.log"

    # 4. Resume with --strategy=cert-regen, --wait 15m
    log "$fixture: kinder resume --strategy=cert-regen --wait 15m"
    set +e
    "$KINDER_BIN" resume "$fixture" --strategy=cert-regen --wait 15m \
        2>&1 | tee "$LOG_DIR/${fixture}-resume.log"
    local rc=$?
    set -e
    if [ "$rc" -ne 0 ]; then
        fail "$fixture: kinder resume --strategy=cert-regen exited $rc (see $LOG_DIR/${fixture}-resume.log)"
    fi

    # 5. Capture post-resume cert serials
    log "$fixture: capture post-resume cert serials"
    capture_serials "$fixture" "$LOG_DIR/${fixture}-serials-post.txt"

    # 6. Assert cert serials changed
    if diff -q "$LOG_DIR/${fixture}-serials-pre.txt" "$LOG_DIR/${fixture}-serials-post.txt" >/dev/null; then
        fail "$fixture: cert serials UNCHANGED post-resume — cert-regen did not actually renew certs (see $LOG_DIR/${fixture}-serials-{pre,post}.txt)"
    fi
    log "$fixture: ok cert serials advanced (pre/post diff)"
    diff "$LOG_DIR/${fixture}-serials-pre.txt" "$LOG_DIR/${fixture}-serials-post.txt" | tee "$LOG_DIR/${fixture}-serials-diff.txt" || true

    # 7. Apiserver healthz from inside cp1
    log "$fixture: apiserver healthz from inside cp1 at https://${loopback}:6443/healthz"
    local hc
    hc="$(apiserver_healthz_inside_cp1 "$fixture" "$loopback" 2>&1)"
    if [ "$hc" != "200" ]; then
        fail "$fixture: apiserver healthz returned HTTP $hc (expected 200) — SC1/SC3 not met"
    fi
    log "$fixture: ok apiserver healthz returned 200 (SC1/SC3 satisfied)"

    # 8. Host kubectl: all expected_nodes Ready
    log "$fixture: capture kubeconfig + assert all $expected_nodes nodes Ready (15m deadline)"
    local kubeconfig="$LOG_DIR/${fixture}-kubeconfig.yaml"
    "$KINDER_BIN" get kubeconfig --name "$fixture" > "$kubeconfig" 2>/dev/null
    if ! all_nodes_ready "$kubeconfig" "$expected_nodes" 900 2>&1 | tee "$LOG_DIR/${fixture}-kubectl.log"; then
        fail "$fixture: host kubectl did not see $expected_nodes Ready nodes within 15m (see $LOG_DIR/${fixture}-kubectl.log)"
    fi
    log "$fixture: ok all $expected_nodes nodes Ready (SC2 satisfied)"

    log "=== fixture $fixture: PASS ==="
}

IPV4_CFG='kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: control-plane
- role: control-plane
- role: worker
- role: worker'

IPV6_CFG='kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  ipFamily: ipv6
nodes:
- role: control-plane
- role: control-plane
- role: control-plane
- role: worker
- role: worker'

DUAL_CFG='kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  ipFamily: dual
nodes:
- role: control-plane
- role: control-plane
- role: control-plane
- role: worker
- role: worker'

main() {
    log "uat-57.3-cert-regen.sh starting; KINDER_BIN=$KINDER_BIN"
    preamble

    # Delete any leftover clusters from prior runs
    for c in "$CLUSTER_IPV4" "$CLUSTER_IPV6" "$CLUSTER_DUAL"; do
        "$KINDER_BIN" delete cluster --name "$c" 2>/dev/null || true
    done

    run_fixture "$CLUSTER_IPV4" "$IPV4_CFG" "127.0.0.1" 5
    run_fixture "$CLUSTER_IPV6" "$IPV6_CFG" "[::1]"     5
    run_fixture "$CLUSTER_DUAL" "$DUAL_CFG" "127.0.0.1" 5

    SCRIPT_FAILED=0
    log "ALL TESTS PASSED — Phase 57.3 SC1 + SC2 + SC3 satisfied across IPv4 + IPv6 + dual-stack"
}

main "$@"
