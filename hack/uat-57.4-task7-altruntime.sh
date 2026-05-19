#!/usr/bin/env bash
# hack/uat-57.4-task7-altruntime.sh — behavioral sanity check for podman + nerdctl
# Cannot exercise kinder's doctor ipam-probe against alternate runtimes on this
# host without removing docker from PATH. This script instead tests the
# UNDERLYING capability the probe gates on: does `<rt> network connect --ip`
# actually work for each runtime?
#
# Records evidence into:
#   .planning/phases/57.4-ipam-probe-regression/uat-logs/57.4-01-uat-macos-podman.txt
#   .planning/phases/57.4-ipam-probe-regression/uat-logs/57.4-01-uat-macos-nerdctl.txt

set -u
cd "$(dirname "$0")/.."

OUT_DIR=.planning/phases/57.4-ipam-probe-regression/uat-logs
mkdir -p "$OUT_DIR"

SUFFIX="alt-$(date +%s)"

# --- PODMAN -----------------------------------------------------------------
PODMAN_OUT="$OUT_DIR/57.4-01-uat-macos-podman.txt"
: > "$PODMAN_OUT"
PROBE="uat574-podman-${SUFFIX}"
PROBE_NET="uat574-podman-net-${SUFFIX}"

{
  echo "=== Phase 57.4 alt-runtime UAT — podman $(podman --version 2>/dev/null || echo unavailable) ==="
  echo "Host date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo ""
  echo "--- network create ---"
  podman network create --subnet=172.210.0.0/24 "$PROBE_NET"
  echo "--- run -d ---"
  podman run -d --name "$PROBE" --network "$PROBE_NET" docker.io/library/alpine:3.21 sleep 30
  sleep 1
  echo "--- inspect (original IP) ---"
  ORIG_IP=$(podman inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$PROBE")
  echo "captured IP: $ORIG_IP"
  echo "--- network disconnect ---"
  podman network disconnect "$PROBE_NET" "$PROBE"
  echo "--- network connect --ip $ORIG_IP (the actual IP-pin operation) ---"
  if podman network connect --ip "$ORIG_IP" "$PROBE_NET" "$PROBE" 2>&1; then
    echo "RESULT: podman network connect --ip WORKS"
    echo "VERDICT: ip-pin path would be VerdictIPPinned"
  else
    echo "RESULT: podman network connect --ip FAILED (see error above)"
    echo "VERDICT: ip-pin path would be VerdictCertRegen or VerdictUnsupported"
  fi
  echo "--- cleanup ---"
  podman rm -f "$PROBE" 2>&1 || true
  podman network rm "$PROBE_NET" 2>&1 || true
} 2>&1 | tee -a "$PODMAN_OUT"

# --- NERDCTL (lima-backed) --------------------------------------------------
NERDCTL_OUT="$OUT_DIR/57.4-01-uat-macos-nerdctl.txt"
: > "$NERDCTL_OUT"
NERDCTL_BIN="${NERDCTL_BIN:-nerdctl.lima}"

{
  echo "=== Phase 57.4 alt-runtime UAT — $NERDCTL_BIN ==="
  echo "Host date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo ""
  echo "--- $NERDCTL_BIN --version ---"
  $NERDCTL_BIN --version 2>&1 || echo "unavailable"
  echo ""
  echo "--- $NERDCTL_BIN network connect --help (key flags: does --ip exist?) ---"
  if $NERDCTL_BIN network connect --help 2>&1 | grep -E -- "--ip"; then
    echo "NOTE: --ip flag is documented on this nerdctl version (2.x added it)"
    echo "However, the production probe SHORT-CIRCUITS for any binary basename"
    echo "=='nerdctl' regardless of the underlying capability — see"
    echo "pkg/internal/doctor/ipamprobe.go:77-78. The basename check uses"
    echo "filepath.Base(binaryName); for 'nerdctl.lima' the basename is"
    echo "'nerdctl.lima' which does NOT match 'nerdctl' literal — so the"
    echo "short-circuit only fires if a binary literally named 'nerdctl' is"
    echo "on PATH. Recording this as a latent observation for review."
  else
    echo "RESULT: --ip flag NOT documented on this nerdctl"
    echo "VERDICT: ip-pin path would be VerdictUnsupported (short-circuit correct)"
  fi
  echo ""
  echo "--- behavioral probe: does $NERDCTL_BIN network connect --ip work? ---"
  PROBE="uat574-nerdctl-${SUFFIX}"
  PROBE_NET="uat574-nerdctl-net-${SUFFIX}"
  echo "(network create)"
  $NERDCTL_BIN network create --subnet=172.220.0.0/24 "$PROBE_NET" 2>&1 || echo "create failed"
  echo "(run -d)"
  $NERDCTL_BIN run -d --name "$PROBE" --network "$PROBE_NET" docker.io/library/alpine:3.21 sleep 30 2>&1 || echo "run failed"
  sleep 1
  echo "(inspect)"
  ORIG_IP=$($NERDCTL_BIN inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$PROBE" 2>/dev/null || echo "")
  echo "captured IP: '$ORIG_IP'"
  if [ -n "$ORIG_IP" ]; then
    echo "(network disconnect)"
    $NERDCTL_BIN network disconnect "$PROBE_NET" "$PROBE" 2>&1 || echo "disconnect failed (may be unsupported on nerdctl)"
    echo "(network connect --ip $ORIG_IP)"
    if $NERDCTL_BIN network connect --ip "$ORIG_IP" "$PROBE_NET" "$PROBE" 2>&1; then
      echo "RESULT: $NERDCTL_BIN network connect --ip WORKS"
    else
      echo "RESULT: $NERDCTL_BIN network connect --ip FAILED (expected — RESEARCH PIT-4)"
    fi
  else
    echo "RESULT: could not capture IP from $NERDCTL_BIN inspect; further testing skipped"
  fi
  echo "--- cleanup ---"
  $NERDCTL_BIN rm -f "$PROBE" 2>&1 || true
  $NERDCTL_BIN network rm "$PROBE_NET" 2>&1 || true
} 2>&1 | tee -a "$NERDCTL_OUT"

echo ""
echo "=== alt-runtime UAT complete ==="
echo "  podman evidence: $PODMAN_OUT"
echo "  nerdctl evidence: $NERDCTL_OUT"
