#!/usr/bin/env bash
#
# verify-integration.sh — Live-cluster integration test for kinder.
#
# Exercises all 5 success criteria from Phase 7 of the kinder roadmap
# against a real kinder cluster. This is the single artifact that automates
# all human-verification items deferred from prior phases.
#
# Usage:
#   ./hack/verify-integration.sh
#
# Prerequisites:
#   - kinder binary in PATH (or ../bin/kinder)
#   - Docker (or compatible runtime) running
#   - kubectl in PATH
#   - curl in PATH (used inside cluster pods, not on host)
#
# Cleanup:
#   The script deletes its test cluster at start and end.
#   If interrupted, run: kinder delete cluster --name kinder-integration-test
#
# Exit codes:
#   0 — all checks passed
#   1 — one or more checks failed

set -uo pipefail

###############################################################################
# Configuration
###############################################################################

CLUSTER_NAME="kinder-integration-test"
CREATE_LOG="/tmp/kinder-create-output.log"

PASS=0
FAIL=0

###############################################################################
# Helpers
###############################################################################

pass() {
  local label="$1"
  PASS=$((PASS + 1))
  echo "  [PASS] $label"
}

fail() {
  local label="$1"
  shift
  FAIL=$((FAIL + 1))
  echo "  [FAIL] $label${1:+ — $1}"
}

section() {
  echo ""
  echo "============================================================"
  echo " $1"
  echo "============================================================"
}

# wait_for runs a command repeatedly until it succeeds or timeout (seconds).
# Usage: wait_for <timeout_seconds> <description> <command...>
wait_for() {
  local timeout="$1"; shift
  local desc="$1"; shift
  local elapsed=0
  while ! "$@" >/dev/null 2>&1; do
    sleep 3
    elapsed=$((elapsed + 3))
    if [ "$elapsed" -ge "$timeout" ]; then
      echo "    Timed out waiting for: $desc (${timeout}s)"
      return 1
    fi
  done
  return 0
}

###############################################################################
# Preamble cleanup
###############################################################################

echo "Cleaning up any previous test cluster..."
kinder delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true

###############################################################################
# SC-1: Full cluster creation with all addons
###############################################################################

section "SC-1: Full cluster creation with all addons"

echo "  Creating cluster (this takes a few minutes)..."
if kinder create cluster --name "$CLUSTER_NAME" 2>&1 | tee "$CREATE_LOG"; then
  pass "SC-1a: kinder create cluster exited 0"
  echo "  Setting kubectl context to kind-${CLUSTER_NAME}..."
  kubectl config use-context "kind-${CLUSTER_NAME}"
else
  fail "SC-1a: kinder create cluster exited non-zero"
fi

# SC-1b: All pods Running or Succeeded (no stuck pods)
echo "  Checking for non-running pods..."
BAD_PODS=$(kubectl get pods -A \
  --field-selector=status.phase!=Running,status.phase!=Succeeded \
  --no-headers 2>/dev/null)
if [ -z "$BAD_PODS" ]; then
  pass "SC-1b: All pods are Running or Succeeded"
else
  fail "SC-1b: Some pods are not Running/Succeeded" "$BAD_PODS"
fi

# SC-1c: Each addon name appears in create output
for ADDON in "MetalLB" "Metrics Server" "CoreDNS Tuning" "Envoy Gateway" "Dashboard"; do
  if grep -qi "$ADDON" "$CREATE_LOG" 2>/dev/null; then
    pass "SC-1c: '$ADDON' found in create output"
  else
    fail "SC-1c: '$ADDON' not found in create output"
  fi
done

###############################################################################
# SC-2: MetalLB-to-Envoy-Gateway E2E
###############################################################################

section "SC-2: MetalLB-to-Envoy-Gateway E2E"

# Deploy test backend
echo "  Deploying test backend..."
kubectl apply -f - <<'BACKEND_EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo-backend
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: echo-backend
  template:
    metadata:
      labels:
        app: echo-backend
    spec:
      containers:
      - name: echo
        image: hashicorp/http-echo:latest
        args: ["-text=kinder-integration-ok"]
        ports:
        - containerPort: 5678
---
apiVersion: v1
kind: Service
metadata:
  name: echo-backend
  namespace: default
spec:
  selector:
    app: echo-backend
  ports:
  - port: 5678
    targetPort: 5678
BACKEND_EOF

# Deploy Gateway + HTTPRoute
echo "  Deploying Gateway and HTTPRoute..."
kubectl apply -f - <<'GW_EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: integration-gw
  namespace: default
spec:
  gatewayClassName: eg
  listeners:
  - name: http
    protocol: HTTP
    port: 80
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: integration-route
  namespace: default
spec:
  parentRefs:
  - name: integration-gw
  rules:
  - backendRefs:
    - name: echo-backend
      port: 5678
GW_EOF

# Wait for Gateway to get EXTERNAL-IP
echo "  Waiting for Gateway EXTERNAL-IP (up to 60s)..."
GW_IP=""
ELAPSED=0
while [ "$ELAPSED" -lt 60 ]; do
  GW_IP=$(kubectl get gateway integration-gw -o jsonpath='{.status.addresses[0].value}' 2>/dev/null)
  if [ -n "$GW_IP" ] && [ "$GW_IP" != "<none>" ]; then
    break
  fi
  sleep 3
  ELAPSED=$((ELAPSED + 3))
done

if [ -n "$GW_IP" ] && [ "$GW_IP" != "<none>" ]; then
  pass "SC-2a: Gateway received EXTERNAL-IP: $GW_IP"
else
  fail "SC-2a: Gateway did not receive EXTERNAL-IP within 60s"
fi

# Curl from inside the cluster (MetalLB IPs not reachable from macOS/Windows host)
echo "  Curling backend from inside cluster..."
if [ -n "$GW_IP" ] && [ "$GW_IP" != "<none>" ]; then
  # Wait a bit for envoy proxy pod to be ready
  sleep 5
  CURL_RESULT=$(kubectl run curl-sc2 \
    --image=curlimages/curl \
    --restart=Never --rm -i --quiet \
    -- curl -sf --max-time 10 "http://${GW_IP}/" 2>/dev/null)
  if echo "$CURL_RESULT" | grep -q "kinder-integration-ok"; then
    pass "SC-2b: In-cluster curl returned expected response"
  else
    fail "SC-2b: In-cluster curl did not return expected response" "$CURL_RESULT"
  fi
else
  fail "SC-2b: Skipped (no Gateway IP)"
fi

###############################################################################
# SC-3: Metrics Server kubectl top and HPA
###############################################################################

section "SC-3: Metrics Server kubectl top and HPA"

# Wait for kubectl top nodes to return data
echo "  Waiting for kubectl top nodes (up to 90s)..."
TOP_OK=false
ELAPSED=0
while [ "$ELAPSED" -lt 90 ]; do
  TOP_OUT=$(kubectl top nodes 2>/dev/null)
  if echo "$TOP_OUT" | grep -q "NAME"; then
    TOP_OK=true
    break
  fi
  sleep 5
  ELAPSED=$((ELAPSED + 5))
done

if $TOP_OK; then
  pass "SC-3a: kubectl top nodes returns data"
else
  fail "SC-3a: kubectl top nodes did not return data within 90s"
fi

# Deploy HPA test workload
echo "  Deploying HPA test workload..."
kubectl apply -f - <<'HPA_EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hpa-test
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: hpa-test
  template:
    metadata:
      labels:
        app: hpa-test
    spec:
      containers:
      - name: pause
        image: registry.k8s.io/pause:3.9
        resources:
          requests:
            cpu: 10m
HPA_EOF

kubectl autoscale deployment hpa-test --cpu-percent=50 --min=1 --max=3 2>/dev/null

# Wait for HPA to show current metrics
echo "  Waiting for HPA metrics (up to 60s)..."
HPA_OK=false
ELAPSED=0
while [ "$ELAPSED" -lt 60 ]; do
  HPA_METRICS=$(kubectl get hpa hpa-test -o jsonpath='{.status.currentMetrics[0].resource.current.averageUtilization}' 2>/dev/null)
  if [ -n "$HPA_METRICS" ] && [ "$HPA_METRICS" != "<unknown>" ]; then
    HPA_OK=true
    break
  fi
  sleep 5
  ELAPSED=$((ELAPSED + 5))
done

if $HPA_OK; then
  pass "SC-3b: HPA shows current CPU metrics ($HPA_METRICS%)"
else
  fail "SC-3b: HPA did not report CPU metrics within 60s"
fi

###############################################################################
# SC-4: CoreDNS resolution
###############################################################################

section "SC-4: CoreDNS resolution"

# Test external hostname resolution
echo "  Testing external DNS resolution..."
EXT_DNS=$(kubectl run dns-ext-sc4 \
  --image=registry.k8s.io/e2e-test-images/agnhost:2.43 \
  --restart=Never --rm -i --quiet \
  -- nslookup kubernetes.io 2>&1)
if echo "$EXT_DNS" | grep -qi "address"; then
  pass "SC-4a: External hostname resolution works (kubernetes.io)"
else
  fail "SC-4a: External hostname resolution failed" "$EXT_DNS"
fi

# Test in-cluster service name resolution
echo "  Testing in-cluster DNS resolution..."
INT_DNS=$(kubectl run dns-int-sc4 \
  --image=registry.k8s.io/e2e-test-images/agnhost:2.43 \
  --restart=Never --rm -i --quiet \
  -- nslookup kubernetes.default.svc.cluster.local 2>&1)
if echo "$INT_DNS" | grep -qi "address"; then
  pass "SC-4b: In-cluster service name resolution works"
else
  fail "SC-4b: In-cluster service name resolution failed" "$INT_DNS"
fi

###############################################################################
# SC-5: Headlamp dashboard accessibility
###############################################################################

section "SC-5: Headlamp dashboard accessibility"

# Check for token in create output
if grep -qi "Token:" "$CREATE_LOG" 2>/dev/null; then
  pass "SC-5a: Token found in create output"
else
  fail "SC-5a: Token not found in create output"
fi

# Check for port-forward instructions in create output
if grep -qi "kubectl port-forward" "$CREATE_LOG" 2>/dev/null; then
  pass "SC-5b: port-forward instructions found in create output"
else
  fail "SC-5b: port-forward instructions not found in create output"
fi

# Start port-forward and check dashboard
echo "  Starting port-forward to Headlamp..."
kubectl port-forward -n kube-system service/headlamp 18080:80 &
PF_PID=$!

DASH_OK=false
ELAPSED=0
while [ "$ELAPSED" -lt 10 ]; do
  if curl -sf http://localhost:18080/ >/dev/null 2>&1; then
    DASH_OK=true
    break
  fi
  sleep 1
  ELAPSED=$((ELAPSED + 1))
done

if $DASH_OK; then
  pass "SC-5c: Headlamp dashboard accessible via port-forward"
else
  fail "SC-5c: Headlamp dashboard not accessible within 10s"
fi

# Kill port-forward
kill "$PF_PID" 2>/dev/null || true
wait "$PF_PID" 2>/dev/null || true

###############################################################################
# Cleanup
###############################################################################

section "Cleanup"

echo "  Deleting test cluster..."
kinder delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
echo "  Done."

###############################################################################
# Summary
###############################################################################

section "Summary"

TOTAL=$((PASS + FAIL))
echo ""
echo "  Total checks: $TOTAL"
echo "  Passed:       $PASS"
echo "  Failed:       $FAIL"
echo ""

if [ "$FAIL" -eq 0 ]; then
  echo "  ALL CHECKS PASSED"
  exit 0
else
  echo "  SOME CHECKS FAILED"
  exit 1
fi
