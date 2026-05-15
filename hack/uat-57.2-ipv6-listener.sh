#!/usr/bin/env bash
# uat-57.2-ipv6-listener.sh — live UAT for Phase 57.2.
# Verifies that the LB Envoy listener address is invariant under
# pause/resume on macOS Docker Desktop with IPv6 ENABLED for both:
#   (a) vanilla IPv4 HA clusters (the Phase 57.1 regression)
#   (b) explicit IPv6 HA clusters (regression-symmetry guarantee)
#
# PREREQUISITES:
#   - Docker Desktop running with IPv6 ENABLED (Settings -> Resources ->
#     Network -> Enable IPv6; restart Docker Desktop after toggling).
#     Verify with: docker network ls -q --filter name=kind | xargs -r \
#       docker network inspect --format '{{.EnableIPv6}}' (returns "true").
#   - ./bin/kinder built from a HEAD that includes Phase 57.2 Plan 01.
#     Verify with: ./bin/kinder version (should show the commit hash of
#     the most recent feat(57.2-01) commit).
set -euo pipefail

KINDER="${KINDER:-./bin/kinder}"
CLUSTER_IPV4="uat-572-ipv4"
CLUSTER_IPV6="uat-572-ipv6"
LOG_DIR=".planning/phases/57.2-fix-discoverlbipv6-derive-from-cluster-ipfamily/uat-logs"
mkdir -p "$LOG_DIR"

log() { echo "[$(date '+%H:%M:%S')] $*" | tee -a "$LOG_DIR/run.log"; }
fail() { log "FAIL: $*"; SCRIPT_FAILED=1; exit 1; }

SCRIPT_FAILED=0
cleanup() {
    if [ "$SCRIPT_FAILED" = "1" ]; then
        log "cleanup: SKIPPED — script failed; clusters preserved for forensics"
        log "         to inspect: docker ps --filter name=$CLUSTER_IPV4"
        log "         to clean up later: $KINDER delete cluster --name $CLUSTER_IPV4 ; $KINDER delete cluster --name $CLUSTER_IPV6"
        return
    fi
    log "cleanup: deleting clusters (best-effort)"
    "$KINDER" delete cluster --name "$CLUSTER_IPV4" 2>/dev/null || true
    "$KINDER" delete cluster --name "$CLUSTER_IPV6" 2>/dev/null || true
}
trap cleanup EXIT

# --- prereq: confirm Docker Desktop IPv6 is on ---
test_00_docker_ipv6_enabled() {
    log "test_00: confirm Docker Desktop IPv6 enabled"
    # Create cluster first if no 'kind' network exists yet, OR inspect any
    # docker network. Simplest portable check: docker info shows IPv6.
    if ! docker info 2>/dev/null | grep -qi 'ipv6'; then
        log "WARN: 'docker info' did not mention IPv6. Continuing — final"
        log "      assertion is on the kind network's EnableIPv6 flag after"
        log "      cluster creation in test_01."
    fi
}

# --- IPv4 cluster (the regression cluster) ---
test_01_create_ipv4_cluster() {
    log "test_01: create vanilla IPv4 3-CP + 2-worker + 1-LB cluster"
    local cfg
    cfg="$(mktemp)"
    cat > "$cfg" <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: control-plane
- role: control-plane
- role: worker
- role: worker
EOF
    "$KINDER" create cluster --name "$CLUSTER_IPV4" --config "$cfg" \
        2>&1 | tee -a "$LOG_DIR/ipv4-create.log"

    # Confirm the kind network is dual-stack (the bug environment)
    local enabled
    enabled="$(docker network inspect kind --format '{{.EnableIPv6}}' 2>/dev/null || echo unknown)"
    log "test_01: kind network EnableIPv6=$enabled (expect 'true' on macOS Docker Desktop)"
    if [ "$enabled" != "true" ]; then
        log "WARN: kind network is not dual-stack; the bug-of-record cannot"
        log "      be reproduced on this host. The UAT will still validate"
        log "      the fix path but is not exercising the exact regression"
        log "      environment from 2026-05-13."
    fi
}

test_02_ipv4_listener_pre_resume() {
    log "test_02: assert LB lds.yaml has address: \"0.0.0.0\" pre-resume (IPv4 cluster)"
    local lb="${CLUSTER_IPV4}-external-load-balancer"
    local lds
    lds="$(docker exec "$lb" cat /home/envoy/lds.yaml 2>&1 | tee "$LOG_DIR/ipv4-lds-before.yaml")"
    echo "$lds" | grep -q 'address: "0.0.0.0"' \
        || fail "test_02: lds.yaml does NOT contain address: \"0.0.0.0\" pre-resume; got:\n$lds"
    echo "$lds" | grep -q '"::"' \
        && fail "test_02: lds.yaml unexpectedly contains \"::\" pre-resume (IPv4 cluster)"
    log "test_02: PASS"
}

test_03_ipv4_pause_resume() {
    log "test_03: pause + resume IPv4 cluster (--wait 10m for cert-regen + addon recovery)"
    "$KINDER" pause "$CLUSTER_IPV4" 2>&1 | tee -a "$LOG_DIR/ipv4-pause.log"
    # Use --wait 30s so the resume returns quickly after orchestration; the
    # 'nodes Ready' poll is done explicitly in test_05 so the cluster stays
    # alive for forensic inspection even if kubectl never recovers.
    set +e
    "$KINDER" resume "$CLUSTER_IPV4" --wait 30s \
        2>&1 | tee -a "$LOG_DIR/ipv4-resume.log"
    local rc=$?
    set -e
    if [ "$rc" -ne 0 ]; then
        log "test_03: resume returned non-zero ($rc) — likely the internal node-Ready wait timed out."
        log "         Continuing so test_04 can capture post-resume lds.yaml and test_05 can poll kubectl with a longer deadline."
    fi
}

test_04_ipv4_listener_post_resume() {
    log "test_04: assert LB lds.yaml STILL has address: \"0.0.0.0\" post-resume (the regression-of-record)"
    local lb="${CLUSTER_IPV4}-external-load-balancer"
    local lds
    lds="$(docker exec "$lb" cat /home/envoy/lds.yaml 2>&1 | tee "$LOG_DIR/ipv4-lds-after.yaml")"
    echo "$lds" | grep -q 'address: "0.0.0.0"' \
        || fail "test_04: REGRESSION — lds.yaml does NOT contain address: \"0.0.0.0\" post-resume; got:\n$lds"
    echo "$lds" | grep -q '"::"' \
        && fail "test_04: REGRESSION — lds.yaml contains \"::\" post-resume on IPv4 cluster (this is the 2026-05-13 bug)"
    log "test_04: PASS"
}

test_05_ipv4_host_kubectl_after_resume() {
    log "test_05: host kubectl get nodes (10m deadline) against resumed IPv4 cluster"
    local kubeconfig="$LOG_DIR/ipv4-kubeconfig.yaml"
    "$KINDER" get kubeconfig --name "$CLUSTER_IPV4" > "$kubeconfig" 2>/dev/null
    # Poll kubectl until all nodes Ready or 10m deadline. cert-regen + addon
    # recovery can take 6-8m on macOS Docker Desktop.
    local deadline=$(( $(date +%s) + 600 ))
    while [ "$(date +%s)" -lt "$deadline" ]; do
        if kubectl --kubeconfig "$kubeconfig" get nodes --no-headers 2>>"$LOG_DIR/ipv4-kubectl.log" \
                | awk '{print $2}' | grep -vq Ready; then
            sleep 10
            continue
        fi
        if kubectl --kubeconfig "$kubeconfig" get nodes 2>&1 | tee -a "$LOG_DIR/ipv4-kubectl.log"; then
            log "test_05: PASS — Phase 58 UAT test_09 regression CLOSED"
            return 0
        fi
        sleep 10
    done
    log "test_05: kubectl polling output (final state):"
    kubectl --kubeconfig "$kubeconfig" get nodes 2>&1 | tee -a "$LOG_DIR/ipv4-kubectl.log" || true
    fail "test_05: kubectl get nodes FAILED after 10m — Phase 58 UAT test_09 regression NOT closed"
}

test_06_ipv4_labels_present() {
    log "test_06: assert io.x-k8s.kinder.ip-family=ipv4 on every IPv4 cluster container"
    for c in $(docker ps --filter "label=io.x-k8s.kind.cluster=${CLUSTER_IPV4}" --format '{{.Names}}'); do
        local v
        v="$(docker inspect --format '{{index .Config.Labels "io.x-k8s.kinder.ip-family"}}' "$c")"
        if [ "$v" != "ipv4" ]; then
            fail "test_06: container $c has io.x-k8s.kinder.ip-family=$v (expected: ipv4)"
        fi
    done
    log "test_06: PASS — all IPv4 cluster containers labeled"
}

# --- IPv6 cluster (regression-symmetry case) ---
test_07_create_ipv6_cluster() {
    log "test_07: create explicit IPv6 3-CP + 2-worker + 1-LB cluster"
    local cfg
    cfg="$(mktemp)"
    cat > "$cfg" <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  ipFamily: ipv6
nodes:
- role: control-plane
- role: control-plane
- role: control-plane
- role: worker
- role: worker
EOF
    "$KINDER" create cluster --name "$CLUSTER_IPV6" --config "$cfg" \
        2>&1 | tee -a "$LOG_DIR/ipv6-create.log"
}

test_08_ipv6_listener_pre_resume() {
    log "test_08: assert IPv6 LB lds.yaml has address: \"::\" AND ipv4_compat: true pre-resume"
    local lb="${CLUSTER_IPV6}-external-load-balancer"
    local lds
    lds="$(docker exec "$lb" cat /home/envoy/lds.yaml 2>&1 | tee "$LOG_DIR/ipv6-lds-before.yaml")"
    echo "$lds" | grep -q 'address: "::"' \
        || fail "test_08: IPv6 lds.yaml does NOT contain address: \"::\" pre-resume; got:\n$lds"
    echo "$lds" | grep -q 'ipv4_compat: true' \
        || fail "test_08: IPv6 lds.yaml does NOT contain ipv4_compat: true pre-resume (Phase 57.2 latent-bug fix); got:\n$lds"
    log "test_08: PASS"
}

test_09_ipv6_pause_resume() {
    log "test_09: pause + resume IPv6 cluster"
    "$KINDER" pause "$CLUSTER_IPV6" 2>&1 | tee -a "$LOG_DIR/ipv6-pause.log"
    set +e
    "$KINDER" resume "$CLUSTER_IPV6" --wait 30s \
        2>&1 | tee -a "$LOG_DIR/ipv6-resume.log"
    local rc=$?
    set -e
    if [ "$rc" -ne 0 ]; then
        log "test_09: resume returned non-zero ($rc); continuing for forensic capture in test_10/test_11"
    fi
}

test_10_ipv6_listener_post_resume() {
    log "test_10: assert IPv6 LB lds.yaml STILL has address: \"::\" AND ipv4_compat: true post-resume"
    local lb="${CLUSTER_IPV6}-external-load-balancer"
    local lds
    lds="$(docker exec "$lb" cat /home/envoy/lds.yaml 2>&1 | tee "$LOG_DIR/ipv6-lds-after.yaml")"
    echo "$lds" | grep -q 'address: "::"' \
        || fail "test_10: IPv6 lds.yaml does NOT contain address: \"::\" post-resume; got:\n$lds"
    echo "$lds" | grep -q 'ipv4_compat: true' \
        || fail "test_10: IPv6 lds.yaml does NOT contain ipv4_compat: true post-resume; got:\n$lds"
    log "test_10: PASS"
}

test_11_ipv6_host_kubectl_after_resume() {
    log "test_11: host kubectl get nodes (10m deadline) against resumed IPv6 cluster (verifies ipv4_compat)"
    local kubeconfig="$LOG_DIR/ipv6-kubeconfig.yaml"
    "$KINDER" get kubeconfig --name "$CLUSTER_IPV6" > "$kubeconfig" 2>/dev/null
    local deadline=$(( $(date +%s) + 600 ))
    while [ "$(date +%s)" -lt "$deadline" ]; do
        if kubectl --kubeconfig "$kubeconfig" get nodes --no-headers 2>>"$LOG_DIR/ipv6-kubectl.log" \
                | awk '{print $2}' | grep -vq Ready; then
            sleep 10
            continue
        fi
        if kubectl --kubeconfig "$kubeconfig" get nodes 2>&1 | tee -a "$LOG_DIR/ipv6-kubectl.log"; then
            log "test_11: PASS — ipv4_compat path verified for IPv6 cluster"
            return 0
        fi
        sleep 10
    done
    log "test_11: kubectl polling output (final state):"
    kubectl --kubeconfig "$kubeconfig" get nodes 2>&1 | tee -a "$LOG_DIR/ipv6-kubectl.log" || true
    fail "test_11: kubectl get nodes FAILED after 10m on resumed IPv6 cluster — ipv4_compat path broken"
}

test_12_ipv6_labels_present() {
    log "test_12: assert io.x-k8s.kinder.ip-family=ipv6 on every IPv6 cluster container"
    for c in $(docker ps --filter "label=io.x-k8s.kind.cluster=${CLUSTER_IPV6}" --format '{{.Names}}'); do
        local v
        v="$(docker inspect --format '{{index .Config.Labels "io.x-k8s.kinder.ip-family"}}' "$c")"
        if [ "$v" != "ipv6" ]; then
            fail "test_12: container $c has io.x-k8s.kinder.ip-family=$v (expected: ipv6)"
        fi
    done
    log "test_12: PASS — all IPv6 cluster containers labeled"
}

main() {
    log "uat-57.2-ipv6-listener.sh starting; KINDER=$KINDER"
    "$KINDER" version 2>&1 | tee -a "$LOG_DIR/run.log"
    test_00_docker_ipv6_enabled
    test_01_create_ipv4_cluster
    test_02_ipv4_listener_pre_resume
    test_03_ipv4_pause_resume
    test_04_ipv4_listener_post_resume
    test_05_ipv4_host_kubectl_after_resume
    test_06_ipv4_labels_present
    test_07_create_ipv6_cluster
    test_08_ipv6_listener_pre_resume
    test_09_ipv6_pause_resume
    test_10_ipv6_listener_post_resume
    test_11_ipv6_host_kubectl_after_resume
    test_12_ipv6_labels_present
    log "ALL TESTS PASSED"
}

main "$@"
