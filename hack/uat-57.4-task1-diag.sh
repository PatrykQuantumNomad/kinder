#!/usr/bin/env bash
# Phase 57.4 Task 1 — IPAM probe diagnostic forensic capture
# Run this on the macOS Docker Desktop host that produced
# `inspect returned invalid IP "invalid IP"` in Phase 58 Plan 01.
#
# Output: .planning/phases/57.4-ipam-probe-regression/uat-logs/57.4-01-ipam-probe-diag.txt
#
# After running, inspect DIAG=1 output and append ONE of these lines manually:
#   LOCKED — index template + --network=none refactor wins. Task 3 proceeds with the locked fix shape.
#   ESCALATE — Docker Desktop is returning a placeholder string for some network state. Task 3 spec changes; see TASK 1 ESCALATION.
#   HALT — bug-of-record cannot be reproduced. Tasks 2-7 do NOT execute.

set -u  # do NOT set -e — we want partial captures even if a step fails
cd "$(dirname "$0")/.."

OUT=.planning/phases/57.4-ipam-probe-regression/uat-logs/57.4-01-ipam-probe-diag.txt
mkdir -p "$(dirname "$OUT")"
: > "$OUT"

SUFFIX="diag-$(date +%s)"
PROBE_NET="kinder-ipam-probe-${SUFFIX}"
PROBE="kinder-ipam-probe-${SUFFIX}"

cleanup() {
  docker rm -f "$PROBE" >/dev/null 2>&1 || true
  docker network rm "$PROBE_NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

{
  echo "=== Phase 57.4 Task 1 — IPAM probe diagnostic ==="
  echo "HEAD: $(git rev-parse HEAD)"
  echo "Host date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "uname -a: $(uname -a)"
  echo "sw_vers (macOS):"
  sw_vers 2>/dev/null || echo "  (not macOS)"
  echo "PROBE_NET=$PROBE_NET"
  echo "PROBE=$PROBE"
} | tee -a "$OUT"

# Setup probe container ----------------------------------------------------
echo "--- creating probe network and container ---" | tee -a "$OUT"
docker network create --subnet=172.200.0.0/24 "$PROBE_NET" 2>&1 | tee -a "$OUT"
docker run -d --name "$PROBE" --network "$PROBE_NET" \
    kindest/node:v1.34.0 sleep 600 2>&1 | tee -a "$OUT"

# DIAG=1 — raw inspect, three formats --------------------------------------
{
  echo ""
  echo "=== DIAG=1 raw inspect — four formats ==="
  echo "--- format: (none — full JSON) ---"
  docker inspect "$PROBE"
  echo "--- format: current production template ---"
  docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$PROBE"
  echo "--- format: {{json .NetworkSettings.Networks}} ---"
  docker inspect --format '{{json .NetworkSettings.Networks}}' "$PROBE"
  echo "--- format: proposed index template (probeNet name) ---"
  docker inspect --format "{{(index .NetworkSettings.Networks \"${PROBE_NET}\").IPAddress}}" "$PROBE"
} 2>&1 | tee -a "$OUT"

# DIAG=2 — network attachments inventory -----------------------------------
{
  echo ""
  echo "=== DIAG=2 network attachments inventory ==="
  echo "--- docker network ls ---"
  docker network ls
  echo "--- docker network inspect $PROBE_NET (full) ---"
  docker network inspect "$PROBE_NET"
  echo "--- docker network inspect kind (full) ---"
  docker network inspect kind 2>/dev/null || echo "kind network not present"
  echo "--- docker network inspect bridge (full) ---"
  docker network inspect bridge
  echo "--- $PROBE attachments across all networks ---"
  for net in $(docker network ls --format '{{.Name}}'); do
    if docker network inspect "$net" --format '{{json .Containers}}' 2>/dev/null | grep -q "$PROBE"; then
      echo "  $PROBE is attached to network: $net"
    fi
  done
} 2>&1 | tee -a "$OUT"

# DIAG=3 — Docker Desktop version + kernel ---------------------------------
{
  echo ""
  echo "=== DIAG=3 Docker Desktop version + info ==="
  echo "--- docker version ---"
  docker version
  echo "--- docker info ---"
  docker info
  echo "--- macOS softwareupdate history (best-effort) ---"
  softwareupdate --history 2>/dev/null | head -20 || true
} 2>&1 | tee -a "$OUT"

# DIAG=4 — repro across networks -------------------------------------------
{
  echo ""
  echo "=== DIAG=4 repro across networks ==="
  for NET in "$PROBE_NET" bridge kind; do
    if ! docker network inspect "$NET" >/dev/null 2>&1; then
      echo "--- $NET — SKIP (network not present) ---"
      continue
    fi
    CNAME="ipam-repro-${NET}-${SUFFIX}"
    docker run -d --name "$CNAME" --network "$NET" kindest/node:v1.34.0 sleep 300 >/dev/null
    echo "--- $NET — current template ---"
    docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$CNAME"
    echo "--- $NET — index template ---"
    docker inspect --format "{{(index .NetworkSettings.Networks \"${NET}\").IPAddress}}" "$CNAME"
    echo "--- $NET — networks attached ---"
    docker inspect --format '{{json .NetworkSettings.Networks}}' "$CNAME"
    docker rm -f "$CNAME" >/dev/null
  done
} 2>&1 | tee -a "$OUT"

# DIAG=5 — two-step --network=none + network connect -----------------------
{
  echo ""
  echo "=== DIAG=5 --network=none + network connect two-step ==="
  VNAME="ipam-twostep-${SUFFIX}"
  VNET="kinder-twostep-${SUFFIX}"
  docker network create --subnet=172.201.0.0/24 "$VNET"
  docker run -d --name "$VNAME" --network=none kindest/node:v1.34.0 sleep 300
  echo "--- after --network=none run, before connect ---"
  docker inspect --format '{{json .NetworkSettings.Networks}}' "$VNAME"
  docker network connect "$VNET" "$VNAME"
  echo "--- after network connect, current template ---"
  docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$VNAME"
  echo "--- after network connect, index template ---"
  docker inspect --format "{{(index .NetworkSettings.Networks \"${VNET}\").IPAddress}}" "$VNAME"
  echo "--- after network connect, full networks ---"
  docker inspect --format '{{json .NetworkSettings.Networks}}' "$VNAME"
  docker rm -f "$VNAME" >/dev/null
  docker network rm "$VNET" >/dev/null
} 2>&1 | tee -a "$OUT"

echo ""
echo "=== capture complete ==="
echo "Output: $OUT"
echo ""
echo "Next step: inspect DIAG=1 output, then APPEND ONE of these lines to $OUT:"
echo "  LOCKED — index template + --network=none refactor wins. Task 3 proceeds with the locked fix shape."
echo "  ESCALATE — Docker Desktop is returning a placeholder string for some network state. Task 3 spec changes; see TASK 1 ESCALATION."
echo "  HALT — bug-of-record cannot be reproduced. Tasks 2-7 do NOT execute."
