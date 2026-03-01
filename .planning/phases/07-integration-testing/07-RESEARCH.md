# Phase 7: Integration Testing - Research

**Researched:** 2026-03-01
**Domain:** Go stdlib `testing`, table-driven unit tests, E2E verification scripts, kinder addon integration patterns
**Confidence:** HIGH

---

## Summary

Phase 7 is the final phase. Its job is to write tests that prove all five addons work correctly together in a single `kinder create cluster` run and that each addon's functional health is verified beyond pod readiness. The phase does not add new features — it adds tests and a structured verification checklist.

The key insight from reading all prior VERIFICATION.md files is that **every prior phase ended with human_needed items** because the verifier could not run a live Docker-backed cluster. Phase 7 must produce: (1) additional Go unit tests covering the untested pure-Go logic, and (2) a single `hack/verify-integration.sh` script that automates the live-cluster checks that prior phases deferred. The Go tests run in CI without Docker. The shell script requires a running `kinder` cluster and documents exactly what to run, what to expect, and why it proves each requirement.

The split matters because Go's `testing` package can run fast, hermetic, in-memory tests for all pure logic (config parsing, subnet carving, corefile string transforms, base64 decode), but cannot provision a real cluster. The live tests must be a script a human (or CI with Docker) can run and read. Both artifacts exist inside the repo and can be committed.

**Primary recommendation:** Write targeted Go unit tests for untested pure logic (CoreDNS Corefile patch transforms, create.go addon-skip logic, logAddonSummary formatting), and write `hack/verify-integration.sh` as a structured bash verification script with clear pass/fail output for all five success criteria and all 34 v1 requirements.

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| ALL-34 | All 34 v1 requirements exercised (cross-phase validation) | Split into unit-testable (pure Go) and live-cluster (shell script) categories; see Validation Architecture |
| SC-1 | Full `kinder create cluster` run with all addons completes without errors, all addon pods Running | Live cluster; `kubectl get pods -A` check in verify script |
| SC-2 | MetalLB-to-Envoy-Gateway E2E: Gateway gets EXTERNAL-IP, HTTPRoute routes traffic to backend | Live cluster; deploy test backend, create Gateway + HTTPRoute, curl via EXTERNAL-IP |
| SC-3 | `kubectl top nodes` returns data and HPA shows current CPU metrics | Live cluster; 60s polling loop for top nodes, deploy HPA workload |
| SC-4 | CoreDNS resolves external hostnames from inside a pod, in-cluster service names resolve correctly | Live cluster; `kubectl exec` with `nslookup` / `dig` |
| SC-5 | Headlamp accessible using printed token and port-forward command | Live cluster; output capture from `kinder create cluster`, port-forward, health check |
</phase_requirements>

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `testing` | Go 1.17+ (project minimum) | Unit test framework | Already used in 15+ test files across the project |
| `go test ./...` | - | Run all unit tests | Standard Go command; matches existing `MODE=unit hack/make-rules/test.sh` |
| bash (POSIX) | system | Integration verification script | Same shell used in `hack/` already; no new tooling |
| `kubectl` | any compatible | Live cluster checks in script | Already in PATH on any kinder-capable machine |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `strings` stdlib | - | String contains checks in unit tests | Already imported throughout codebase |
| `bytes` stdlib | - | Buffer assertions | Already used in dashboard.go |
| `net` stdlib | - | IP/CIDR assertions | Already used in subnet.go |
| `encoding/base64` stdlib | - | Token decode assertions | Already used in dashboard.go |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| bash verification script | Go test with `//go:build integration` build tag | Build tag approach is more portable but requires Docker in the Go test environment; bash script with clear output is better for human verification and matches prior phases' VERIFICATION.md pattern |
| bash verification script | bats-core (Bash test framework) | bats-core is not in the project; adding a new test framework is more than Phase 7 needs |
| `kubectl` in script | Go e2e framework (like kubetest) | kubetest has no presence in this codebase; kubectl is simpler and already required |

**Installation:** No new Go dependencies. No new tools. Bash and kubectl are already required to use kinder.

---

## Architecture Patterns

### Recommended Project Structure

New files added in Phase 7:

```
pkg/cluster/internal/create/
├── create_addon_test.go       # Unit tests: logAddonSummary, runAddon gate, platform warning logic
pkg/cluster/internal/create/actions/installcorednstuning/
├── corefile_test.go           # Unit tests: Corefile patch string transforms (missing from prior phases)
hack/
└── verify-integration.sh      # Live cluster integration verification script
.planning/phases/07-integration-testing/
└── 07-VERIFICATION.md         # Verification report (produced after execution)
```

No changes to existing production code. Phase 7 is tests-only.

### Pattern 1: Table-Driven Unit Test (Go stdlib)

**What:** Use `t.Run(name, func(t *testing.T){...})` with a slice of test cases. This is the dominant pattern in the codebase.
**When to use:** All Go unit tests in this phase.

**Example:**

```go
// Source: pkg/cluster/internal/create/actions/installmetallb/subnet_test.go (existing pattern)
func TestLogAddonSummary(t *testing.T) {
    tests := []struct {
        name     string
        results  []addonResult
        wantLine string
    }{
        {
            name: "enabled addon appears as installed",
            results: []addonResult{{name: "MetalLB", enabled: true}},
            wantLine: "MetalLB",
        },
        {
            name: "disabled addon shows skipped",
            results: []addonResult{{name: "MetalLB", enabled: false}},
            wantLine: "skipped",
        },
        {
            name: "failed addon shows FAILED",
            results: []addonResult{{name: "MetalLB", enabled: true, err: errors.New("boom")}},
            wantLine: "FAILED",
        },
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            // capture logger output, assert wantLine appears
        })
    }
}
```

### Pattern 2: Exported-Function Unit Test for String Transforms

**What:** CoreDNS tuning involves three string replacements on the Corefile. These transforms are pure functions and are ideal for unit testing, but no unit test exists yet.
**When to use:** Test the Corefile patch logic without a live cluster.

The coredns action reads the Corefile, applies transforms, and writes it back. To unit-test this, the transforms need to be expressed as exported or package-level functions that accept a string and return a string. If they are currently inlined in `Execute()`, Phase 7 may need to extract them to package-level functions to make them testable.

```go
// pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go
// (if the transforms are inlined in Execute, extract them as package-level functions)

// patchCorefile applies kinder's CoreDNS tuning to the raw Corefile string.
// Returns the patched string and an error if any required marker is absent.
func patchCorefile(raw string) (string, error) {
    // three transforms: autopath, pods verified, cache 60
    ...
}
```

```go
// pkg/cluster/internal/create/actions/installcorednstuning/corefile_test.go
func TestPatchCorefile(t *testing.T) {
    input := `.:53 {
    errors
    health {
       lameduck 5s
    }
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
    }
    forward . /etc/resolv.conf {
       max_concurrent 1000
    }
    cache 30
    loop
    reload
    loadbalance
}`
    got, err := patchCorefile(input)
    if err != nil {
        t.Fatalf("patchCorefile() error: %v", err)
    }
    if !strings.Contains(got, "autopath @kubernetes") {
        t.Error("expected autopath @kubernetes in patched Corefile")
    }
    if !strings.Contains(got, "pods verified") {
        t.Error("expected pods verified in patched Corefile")
    }
    if !strings.Contains(got, "cache 60") {
        t.Error("expected cache 60 in patched Corefile")
    }
    if strings.Contains(got, "pods insecure") {
        t.Error("pods insecure should have been replaced")
    }
    if strings.Contains(got, "cache 30") {
        t.Error("cache 30 should have been replaced")
    }
}
```

### Pattern 3: Integration Verification Script Structure

**What:** A bash script that runs against a live kinder cluster and reports PASS/FAIL for each success criterion.
**When to use:** This is the single file that documents and automates all human-verification items from prior phases.

```bash
#!/usr/bin/env bash
# hack/verify-integration.sh
# Verifies all 5 success criteria for kinder v1 integration testing.
# Usage: ./hack/verify-integration.sh
# Requires: kinder binary in PATH, Docker running, sufficient RAM (2GB+)

set -euo pipefail
PASS=0
FAIL=0

pass() { echo "[PASS] $1"; PASS=$((PASS+1)); }
fail() { echo "[FAIL] $1"; FAIL=$((FAIL+1)); }

# SC-1: Full create cluster with all addons
echo "=== SC-1: Full cluster creation ==="
kinder create cluster --name kinder-integration-test 2>&1 | tee /tmp/kinder-create.log
kubectl get pods -A --field-selector=status.phase!=Running 2>&1 | grep -v "Completed" | grep -v "^NAME" \
    && fail "SC-1: some pods are not Running" || pass "SC-1: all addon pods Running"

# ... (see full structure in Code Examples section)
```

### Pattern 4: Create Addon Gating Unit Test

**What:** Test that `runAddon` skips the action when `enabled=false` and calls Execute when `enabled=true`. The `logAddonSummary` and `logMetalLBPlatformWarning` functions in `create.go` are unexported — test them from within the `create` package.
**When to use:** Unit tests in `create_addon_test.go` (package `create`).

The `runAddon` closure in `create.go` is an internal function. To unit-test it without a live cluster, extract it as a package-level function that takes a `log.Logger`, `actions.ActionContext`, and `[]addonResult`. Alternatively, test only `logAddonSummary` which is a pure function (takes logger + slice, produces formatted output) — this is more tractable.

### Anti-Patterns to Avoid

- **Testing action Execute functions with a mock ActionContext:** The `ActionContext` type embeds a `nodes.Node` interface that calls `docker exec` internally. Attempting to mock this for unit tests produces fragile mocks that test the mock, not the code. Unit tests should target pure functions only; leave Execute to the live cluster script.
- **Requiring a live cluster for Go unit tests:** Phase 7 Go tests must pass in CI without Docker. If a test requires a cluster, it belongs in the shell script, not `go test`.
- **One test file covering all addons:** Each package owns its own test file. CoreDNS tests go in `installcorednstuning/`, create-level tests go in `create/`. Follow the existing pattern.
- **Parameterized cluster creation in the shell script:** The script creates exactly one cluster with default config (all addons enabled). It does not test opt-out paths at this stage — those are already covered by unit tests of the config loading layer.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Test assertions | Custom `assertEqual`, `mustEqual` helpers | Direct `t.Error` / `t.Fatal` with descriptive messages | Go stdlib testing idiom; existing tests use this pattern; `pkg/internal/assert` package exists but is minimal |
| Cluster lifecycle in Go tests | `exec.Command("kinder", "create", "cluster")` in a Go test | Shell script | Go tests should be hermetic; live cluster management belongs in shell |
| JSON parsing in shell script | `jq` | `kubectl -o jsonpath` | `jq` may not be installed; `kubectl jsonpath` is always available when kubectl is present |
| Waiting for readiness in shell script | `sleep` loops | `kubectl wait --for=condition=...` | kubectl wait is idiomatic; sleep is fragile |
| HTTP checks in shell script | `python3 -c "import urllib..."` | `curl` | curl is universally available |

**Key insight:** Phase 7 has no production code to write. The entire effort is test authorship. The hardest part is identifying what is actually unit-testable versus what requires a live cluster, and writing the live-cluster script in a way that is self-documenting and produces clear PASS/FAIL output.

---

## Common Pitfalls

### Pitfall 1: Trying to Unit-Test Execute Functions

**What goes wrong:** Developer writes a test for `metallb.Execute()` and needs a mock `ActionContext`. The `ActionContext.Nodes()` method returns `[]nodes.Node` where each node implements command execution via `docker exec`. Mocking this requires implementing a full `nodes.Node` interface — at least 8 methods. The resulting test is 200 lines of mock boilerplate and breaks every time the interface changes.

**Why it happens:** Desire for high coverage metrics; misunderstanding where the unit-testable boundary is.

**How to avoid:** Only unit-test functions that take and return plain Go types (strings, structs, errors). The testable functions in this codebase are: `parseSubnetFromJSON`, `carvePoolFromSubnet`, `patchCorefile` (if extracted), `logAddonSummary`, `logMetalLBPlatformWarning`. Everything that touches `node.Command(...)` belongs in the shell script.

**Warning signs:** Test imports `"sigs.k8s.io/kind/pkg/cluster/nodes"` or requires building a mock provider struct.

### Pitfall 2: Shell Script Exits on First Failure

**What goes wrong:** Script uses `set -e` and the first FAIL (e.g., `kubectl top nodes` not ready yet) kills the script before it can verify the remaining success criteria.

**Why it happens:** `set -e` is good practice for scripts that must not continue on error, but for a verification script we want all checks to run.

**How to avoid:** Structure each check to capture its exit code without `set -e` killing the script. Use subshells with `|| true` and explicit PASS/FAIL tracking:

```bash
if kubectl top nodes 2>/dev/null | grep -q "NAME"; then
    pass "SC-3: kubectl top nodes returns data"
else
    fail "SC-3: kubectl top nodes not returning data"
fi
```

Use `set -euo pipefail` only at the top level, then wrap each check in an `if` or subshell.

**Warning signs:** Script reports 1 failure and exits with no output for the remaining 4 success criteria.

### Pitfall 3: Port-Forward Races in Verification Script

**What goes wrong:** Script starts `kubectl port-forward` in the background, immediately tries `curl localhost:8080`, and fails because the tunnel is not established yet.

**Why it happens:** `kubectl port-forward` has a brief startup delay (TLS handshake, connection pooling). Running curl immediately after the backgrounded port-forward always fails.

**How to avoid:** Add a wait loop after starting port-forward:

```bash
kubectl port-forward -n kube-system service/headlamp 8080:80 &
PF_PID=$!
for i in $(seq 1 10); do
    sleep 1
    curl -sf http://localhost:8080 >/dev/null 2>&1 && break
done
# then check result
kill $PF_PID 2>/dev/null || true
```

**Warning signs:** SC-5 always fails even though Headlamp is running.

### Pitfall 4: Corefile Transform Tests Missing Guard Checks

**What goes wrong:** `patchCorefile` tests pass on a well-formed Corefile but the real action has guard checks that return errors when expected markers are absent (e.g., `pods insecure` not found). Tests that don't test the guard paths leave the guards untested.

**Why it happens:** Tests only cover the "happy path."

**How to avoid:** Add test cases for missing markers:

```go
{
    name: "missing pods insecure returns error",
    input: `.:53 { kubernetes cluster.local { pods verified } cache 30 }`,
    wantErr: true,
},
```

**Warning signs:** Test coverage report shows the guard branches uncovered.

### Pitfall 5: Integration Script Cluster Name Collision

**What goes wrong:** If a `kinder-integration-test` cluster already exists from a previous failed run, `kinder create cluster --name kinder-integration-test` fails immediately.

**Why it happens:** Prior run failed and `--retain` was not set, or cleanup failed.

**How to avoid:** Script's preamble deletes any pre-existing cluster with that name before creating:

```bash
kinder delete cluster --name kinder-integration-test 2>/dev/null || true
```

**Warning signs:** Script fails at step 1 with "node(s) already exist for a cluster with the name 'kinder-integration-test'".

### Pitfall 6: HTTPRoute/Gateway E2E Test Requires MetalLB IP Reachability

**What goes wrong:** On macOS/Windows the MetalLB LoadBalancer IP is not reachable from the host (this is documented and warned by kinder). SC-2 requires curling via the EXTERNAL-IP, which cannot work on macOS.

**Why it happens:** MetalLB L2 mode uses ARP which only works on Linux with direct network access to the Docker bridge.

**How to avoid:** SC-2 should use `kubectl exec` from inside a cluster pod to curl the Gateway EXTERNAL-IP, not from the host. This works on all platforms because pod-to-pod traffic routes through the kind network even on macOS. Alternatively, use `kubectl port-forward` on the Gateway service as the curl target — but this bypasses the MetalLB IP path. The correct approach is curl from inside the cluster:

```bash
# Inside a pod, curl the Gateway service EXTERNAL-IP
kubectl run curl-test --image=curlimages/curl --restart=Never --rm -i -- \
    curl -sf http://<EXTERNAL-IP>/
```

**Warning signs:** SC-2 fails on macOS with "Connection refused" or "No route to host" but works on Linux.

---

## Code Examples

Verified patterns from the existing codebase:

### Go Unit Test: logAddonSummary

The `logAddonSummary` function is already written in `create.go` (line 325-338). It takes a `log.Logger` and `[]addonResult`. The `log.Logger` interface is defined in `pkg/log/types.go`. The test captures output by providing a test logger.

```go
// pkg/cluster/internal/create/create_addon_test.go
// Source: derived from existing subnet_test.go and validate_test.go patterns

package create

import (
    "strings"
    "testing"
)

// testLogger captures Info calls for assertion
type testLogger struct {
    lines []string
}

func (l *testLogger) Info(message string) { l.lines = append(l.lines, message) }
func (l *testLogger) Infof(format string, args ...interface{}) {
    l.lines = append(l.lines, fmt.Sprintf(format, args...))
}
// implement remaining log.Logger interface methods as no-ops...

func TestLogAddonSummary(t *testing.T) {
    tests := []struct {
        name     string
        results  []addonResult
        wantIn   []string // strings that must appear in combined output
        wantOut  []string // strings that must NOT appear in combined output
    }{
        {
            name:   "enabled addon shows installed",
            results: []addonResult{{name: "MetalLB", enabled: true}},
            wantIn: []string{"MetalLB", "installed"},
        },
        {
            name:   "disabled addon shows skipped",
            results: []addonResult{{name: "MetalLB", enabled: false}},
            wantIn: []string{"MetalLB", "skipped"},
        },
        {
            name:   "failed addon shows FAILED with error",
            results: []addonResult{{name: "MetalLB", enabled: true, err: errors.New("boom")}},
            wantIn: []string{"MetalLB", "FAILED", "boom"},
        },
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            logger := &testLogger{}
            logAddonSummary(logger, tc.results)
            combined := strings.Join(logger.lines, "\n")
            for _, want := range tc.wantIn {
                if !strings.Contains(combined, want) {
                    t.Errorf("expected %q in output, got:\n%s", want, combined)
                }
            }
        })
    }
}
```

### Go Unit Test: CoreDNS Corefile Patch

The coredns action (`installcorednstuning/corednstuning.go`) currently performs the transforms inline in `Execute()`. Phase 7 extracts them to a testable function. This is a minor refactor of production code in one action file.

```go
// pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go
// Extract the three string transforms to a package-level function:

// patchCorefile applies kinder's CoreDNS tuning to the raw Corefile string.
// It expects the standard kind Corefile format with:
//   - "pods insecure" keyword
//   - "cache 30" setting
// Returns the patched Corefile and an error if required markers are absent.
func patchCorefile(raw string) (string, error) {
    if !strings.Contains(raw, "pods insecure") {
        return "", errors.New("expected 'pods insecure' in Corefile but it was not found; Corefile format may have changed")
    }
    if !strings.Contains(raw, "cache 30") {
        return "", errors.New("expected 'cache 30' in Corefile but it was not found; Corefile format may have changed")
    }
    patched := strings.ReplaceAll(raw, "pods insecure", "pods verified")
    patched = strings.ReplaceAll(patched, "cache 30", "cache 60")
    // Insert autopath before kubernetes block
    patched = strings.ReplaceAll(patched, "kubernetes cluster.local", "autopath @kubernetes\n    kubernetes cluster.local")
    return patched, nil
}
```

```go
// pkg/cluster/internal/create/actions/installcorednstuning/corefile_test.go
package installcorednstuning

import (
    "strings"
    "testing"
)

const sampleCorefile = `.:53 {
    errors
    health {
       lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
       ttl 30
    }
    prometheus :9153
    forward . /etc/resolv.conf {
       max_concurrent 1000
    }
    cache 30
    loop
    reload
    loadbalance
}`

func TestPatchCorefile(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantIn  []string
        wantOut []string
        wantErr bool
    }{
        {
            name:    "standard kind Corefile is patched correctly",
            input:   sampleCorefile,
            wantIn:  []string{"autopath @kubernetes", "pods verified", "cache 60"},
            wantOut: []string{"pods insecure", "cache 30"},
            wantErr: false,
        },
        {
            name:    "missing pods insecure returns error",
            input:   strings.ReplaceAll(sampleCorefile, "pods insecure", "pods verified"),
            wantErr: true,
        },
        {
            name:    "missing cache 30 returns error",
            input:   strings.ReplaceAll(sampleCorefile, "cache 30", "cache 60"),
            wantErr: true,
        },
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got, err := patchCorefile(tc.input)
            if tc.wantErr {
                if err == nil {
                    t.Error("patchCorefile() expected error, got nil")
                }
                return
            }
            if err != nil {
                t.Fatalf("patchCorefile() unexpected error: %v", err)
            }
            for _, want := range tc.wantIn {
                if !strings.Contains(got, want) {
                    t.Errorf("expected %q in patched Corefile, got:\n%s", want, got)
                }
            }
            for _, dontWant := range tc.wantOut {
                if strings.Contains(got, dontWant) {
                    t.Errorf("did not expect %q in patched Corefile, got:\n%s", dontWant, got)
                }
            }
        })
    }
}
```

### Integration Verification Script Skeleton

```bash
#!/usr/bin/env bash
# hack/verify-integration.sh
# Integration verification for kinder v1 - all 5 success criteria
# Usage:  ./hack/verify-integration.sh
# Prereqs: kinder in PATH, Docker running, kubectl in PATH, curl in PATH
# Cleanup: kinder delete cluster --name kinder-integration-test

set -uo pipefail

CLUSTER_NAME="kinder-integration-test"
PASS=0
FAIL=0

pass() { echo "  [PASS] $1"; PASS=$((PASS+1)); }
fail() { echo "  [FAIL] $1"; FAIL=$((FAIL+1)); }
section() { echo ""; echo "=== $1 ==="; }

# --- Preamble: clean up any prior run ---
section "Cleanup"
kinder delete cluster --name "$CLUSTER_NAME" 2>/dev/null && echo "  Deleted existing cluster" || echo "  No existing cluster to delete"

# --- SC-1: Full cluster creation ---
section "SC-1: Full cluster creation with all addons"
if kinder create cluster --name "$CLUSTER_NAME" 2>&1 | tee /tmp/kinder-create-output.log; then
    pass "SC-1a: kinder create cluster exited 0"
else
    fail "SC-1a: kinder create cluster failed (check /tmp/kinder-create-output.log)"
fi

# Verify all addon pods are Running
NOT_RUNNING=$(kubectl get pods -A \
    --field-selector=status.phase!=Running,status.phase!=Succeeded \
    --no-headers 2>/dev/null | grep -v "^$" || true)
if [ -z "$NOT_RUNNING" ]; then
    pass "SC-1b: all addon pods are Running or Succeeded"
else
    fail "SC-1b: some pods are not Running:"
    echo "$NOT_RUNNING" | head -10
fi

# Check addons summary appears in output
for ADDON in "MetalLB" "Metrics Server" "CoreDNS Tuning" "Envoy Gateway" "Dashboard"; do
    if grep -q "$ADDON" /tmp/kinder-create-output.log; then
        pass "SC-1c: addon $ADDON appears in output"
    else
        fail "SC-1c: addon $ADDON missing from output"
    fi
done

# --- SC-2: MetalLB + Envoy Gateway E2E ---
section "SC-2: MetalLB-to-Envoy-Gateway E2E"

# Deploy a test backend
kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: integration-backend
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: integration-backend
  template:
    metadata:
      labels:
        app: integration-backend
    spec:
      containers:
      - name: backend
        image: hashicorp/http-echo:latest
        args: ["-text=kinder-integration-ok"]
        ports:
        - containerPort: 5678
---
apiVersion: v1
kind: Service
metadata:
  name: integration-backend
  namespace: default
spec:
  selector:
    app: integration-backend
  ports:
  - port: 80
    targetPort: 5678
EOF

# Create Gateway + HTTPRoute
kubectl apply -f - <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: integration-gateway
  namespace: default
spec:
  gatewayClassName: eg
  listeners:
  - name: http
    port: 80
    protocol: HTTP
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: integration-route
  namespace: default
spec:
  parentRefs:
  - name: integration-gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: integration-backend
      port: 80
EOF

# Wait for gateway to get EXTERNAL-IP (up to 60s)
GW_IP=""
for i in $(seq 1 12); do
    GW_IP=$(kubectl get gateway integration-gateway -o jsonpath='{.status.addresses[0].value}' 2>/dev/null || true)
    [ -n "$GW_IP" ] && break
    sleep 5
done

if [ -n "$GW_IP" ]; then
    pass "SC-2a: Gateway received EXTERNAL-IP: $GW_IP"
    # Curl from inside the cluster (works on macOS too)
    RESPONSE=$(kubectl run curl-sc2 --image=curlimages/curl --restart=Never --rm -i --quiet \
        -- curl -sf "http://$GW_IP/" 2>/dev/null || true)
    if echo "$RESPONSE" | grep -q "kinder-integration-ok"; then
        pass "SC-2b: HTTPRoute routes traffic to backend"
    else
        fail "SC-2b: HTTPRoute did not return expected response (got: $RESPONSE)"
    fi
else
    fail "SC-2a: Gateway did not receive EXTERNAL-IP within 60s"
    fail "SC-2b: (skipped - no EXTERNAL-IP)"
fi

# --- SC-3: Metrics Server ---
section "SC-3: kubectl top and HPA metrics"

# Wait up to 90s for kubectl top nodes
TOP_OK=false
for i in $(seq 1 18); do
    if kubectl top nodes 2>/dev/null | grep -q "NAME"; then
        TOP_OK=true
        break
    fi
    sleep 5
done
if $TOP_OK; then
    pass "SC-3a: kubectl top nodes returns data"
else
    fail "SC-3a: kubectl top nodes did not return data within 90s"
fi

# Deploy HPA test workload
kubectl apply -f - <<'EOF'
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
      - name: app
        image: registry.k8s.io/pause:3.9
        resources:
          requests:
            cpu: "10m"
EOF

kubectl autoscale deployment hpa-test --cpu-percent=50 --min=1 --max=3 -n default 2>/dev/null || true

# Wait for HPA to show real metrics (up to 60s)
HPA_OK=false
for i in $(seq 1 12); do
    TARGETS=$(kubectl get hpa hpa-test -n default -o jsonpath='{.status.currentMetrics[0].resource.current.averageUtilization}' 2>/dev/null || true)
    [ -n "$TARGETS" ] && HPA_OK=true && break
    sleep 5
done
if $HPA_OK; then
    pass "SC-3b: HPA shows current CPU metrics"
else
    fail "SC-3b: HPA did not show current CPU metrics within 60s"
fi

# --- SC-4: CoreDNS resolution ---
section "SC-4: CoreDNS resolution"

# External hostname resolution from inside a pod
EXT_RESULT=$(kubectl run dns-test-ext --image=registry.k8s.io/e2e-test-images/agnhost:2.43 \
    --restart=Never --rm -i --quiet \
    -- /agnhost resolve-host --host=kubernetes.io 2>/dev/null || true)
if echo "$EXT_RESULT" | grep -q "RESOLVED\|[0-9]\+\.[0-9]\+"; then
    pass "SC-4a: CoreDNS resolves external hostname (kubernetes.io)"
else
    fail "SC-4a: CoreDNS failed to resolve external hostname"
fi

# In-cluster service name resolution
SVC_RESULT=$(kubectl run dns-test-svc --image=registry.k8s.io/e2e-test-images/agnhost:2.43 \
    --restart=Never --rm -i --quiet \
    -- /agnhost resolve-host --host=kubernetes.default.svc.cluster.local 2>/dev/null || true)
if echo "$SVC_RESULT" | grep -q "RESOLVED\|[0-9]\+\.[0-9]\+"; then
    pass "SC-4b: CoreDNS resolves in-cluster service name"
else
    fail "SC-4b: CoreDNS failed to resolve in-cluster service name"
fi

# --- SC-5: Headlamp dashboard ---
section "SC-5: Headlamp dashboard accessibility"

# Check token appears in create output
if grep -q "Token:" /tmp/kinder-create-output.log; then
    pass "SC-5a: Token printed in kinder create cluster output"
    TOKEN=$(grep "Token:" /tmp/kinder-create-output.log | awk '{print $NF}')
else
    fail "SC-5a: Token not found in kinder create cluster output"
    TOKEN=""
fi

# Check port-forward command appears
if grep -q "kubectl port-forward" /tmp/kinder-create-output.log; then
    pass "SC-5b: Port-forward command printed in output"
else
    fail "SC-5b: Port-forward command not found in output"
fi

# Start port-forward and check Headlamp responds
kubectl port-forward -n kube-system service/headlamp 18080:80 &
PF_PID=$!
HEADLAMP_OK=false
for i in $(seq 1 10); do
    sleep 1
    if curl -sf http://localhost:18080/ >/dev/null 2>&1; then
        HEADLAMP_OK=true
        break
    fi
done
kill $PF_PID 2>/dev/null || true
if $HEADLAMP_OK; then
    pass "SC-5c: Headlamp responds on port-forward (HTTP 200)"
else
    fail "SC-5c: Headlamp did not respond via port-forward"
fi

# --- Summary ---
echo ""
echo "=================================="
echo "Integration Test Summary"
echo "=================================="
echo "  PASSED: $PASS"
echo "  FAILED: $FAIL"
echo "=================================="
if [ "$FAIL" -eq 0 ]; then
    echo "ALL CHECKS PASSED"
    exit 0
else
    echo "SOME CHECKS FAILED"
    exit 1
fi
```

### Existing Unit Test Pattern to Follow

```go
// Source: pkg/cluster/internal/create/actions/installmetallb/subnet_test.go (existing)
// This is the canonical pattern for the project — table-driven, uses only stdlib testing

package installmetallb

import (
    "testing"
)

func TestCarvePoolFromSubnet(t *testing.T) {
    tests := []struct {
        name        string
        cidr        string
        wantPool    string
        wantErr     bool
        errContains string
    }{
        {
            name:     "/16 network returns last .255.200-.255.250",
            cidr:     "172.18.0.0/16",
            wantPool: "172.18.255.200-172.18.255.250",
        },
        // ...
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got, err := carvePoolFromSubnet(tc.cidr)
            // assertions...
        })
    }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual verification checklists in VERIFICATION.md | Automated script `hack/verify-integration.sh` + Go unit tests | Phase 7 (this phase) | Human steps become runnable commands with PASS/FAIL output |
| Ad-hoc integration testing | Structured per-SC verification with cleanup | Phase 7 | Repeatable; any developer can run `./hack/verify-integration.sh` |

**Deprecated/outdated:**
- None — Phase 7 adds new artifacts without replacing existing ones.

---

## Open Questions

1. **CoreDNS transform extraction**
   - What we know: The three string transforms in `installcorednstuning/corednstuning.go` are currently inside `Execute()`. To write unit tests, they must be extractable to a package-level function.
   - What's unclear: Whether they are already extracted or still inlined (need to read the file during planning).
   - Recommendation: If inlined, extract to `patchCorefile(raw string) (string, error)`. This is a minor refactor of one file and is safe — the function signature is simple and testable.

2. **create.go logAddonSummary testability**
   - What we know: `logAddonSummary` in `create.go` takes a `log.Logger` (interface) and `[]addonResult`. The `addonResult` type is defined in `create.go`. Both are accessible from `create_addon_test.go` since it's in the same package.
   - What's unclear: Whether the `log.Logger` interface is simple enough to implement in a test (no unexported methods). Checking `pkg/log/types.go` will confirm this during planning.
   - Recommendation: Write a `testLogger` struct implementing `log.Logger` that captures all output to a `[]string`. This is straightforward if the interface has 3-5 methods.

3. **SC-2 curl test image availability**
   - What we know: `curlimages/curl` is a Docker Hub image. In air-gapped or restricted environments it may not pull.
   - What's unclear: Whether kinder users typically have unrestricted Docker Hub access.
   - Recommendation: Alternatively use `registry.k8s.io/e2e-test-images/agnhost:2.43` which has a `curl` command built in, or use `busybox` with wget. Flag this in the script with a comment.

4. **Platform handling for SC-2 EXTERNAL-IP reachability**
   - What we know: On macOS/Windows, MetalLB IPs are not reachable from the host. The integration script must curl from inside the cluster.
   - What's unclear: Whether `kubectl run` with `--rm -i` is reliable for this kind of inline probe.
   - Recommendation: Use `kubectl run` with `--rm -i` as shown in the script skeleton. This is well-supported. Add a note in the script that curl tests run from inside the cluster to be platform-neutral.

---

## Validation Architecture

No `.planning/config.json` found — `workflow.nyquist_validation` defaults to not set. Including section because this is a testing phase and the mapping is central to the work.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + bash shell script |
| Config file | None — uses `go test ./...` |
| Quick run command | `go test ./pkg/cluster/internal/create/... ./pkg/cluster/internal/create/actions/installcorednstuning/...` |
| Full suite command | `go test ./...` (all unit tests, no cluster required) |
| Integration script | `./hack/verify-integration.sh` (requires Docker + running kinder binary) |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FOUND-01 through FOUND-05 | Binary name, config schema, addon defaults, platform warning | Unit (existing + new) | `go test ./pkg/internal/apis/config/encoding/... ./pkg/cluster/internal/create/...` | ✅ existing + ❌ Wave 0 for platform warning |
| MLB-01 through MLB-08 | MetalLB install, subnet detection, pool carving | Unit (subnet) + Live (pods, EXTERNAL-IP) | `go test ./pkg/cluster/internal/create/actions/installmetallb/...` | ✅ existing 15 tests |
| MET-01 through MET-05 | Metrics Server manifest, kubelet-insecure-tls, top nodes, HPA | Unit (manifest check) + Live | `go test ./pkg/cluster/internal/create/actions/installmetricsserver/...` | ❌ Wave 0 |
| DNS-01 through DNS-05 | CoreDNS Corefile patch, autopath, pods verified, cache 60 | Unit (string transforms) + Live | `go test ./pkg/cluster/internal/create/actions/installcorednstuning/...` | ❌ Wave 0 |
| EGW-01 through EGW-06 | Envoy Gateway manifest, GatewayClass, HTTPRoute E2E | Live only (no pure-Go logic to unit-test) | `./hack/verify-integration.sh` (SC-2) | ❌ Wave 0 (shell script) |
| DASH-01 through DASH-06 | Headlamp manifest, token decode, port-forward | Unit (base64 decode) + Live | `go test ./pkg/cluster/internal/create/actions/installdashboard/...` | ❌ Wave 0 |
| SC-1 | Full create cluster, all addons | Live | `./hack/verify-integration.sh` (SC-1 section) | ❌ Wave 0 (shell script) |
| SC-2 | MetalLB + Envoy Gateway E2E | Live | `./hack/verify-integration.sh` (SC-2 section) | ❌ Wave 0 (shell script) |
| SC-3 | kubectl top + HPA | Live | `./hack/verify-integration.sh` (SC-3 section) | ❌ Wave 0 (shell script) |
| SC-4 | CoreDNS external + in-cluster resolution | Live | `./hack/verify-integration.sh` (SC-4 section) | ❌ Wave 0 (shell script) |
| SC-5 | Headlamp token + port-forward | Live | `./hack/verify-integration.sh` (SC-5 section) | ❌ Wave 0 (shell script) |

### Sampling Rate

- **Per task commit:** `go test ./...` (all unit tests pass before commit)
- **Per wave merge:** `go test ./...` + manual inspection that script is syntactically valid (`bash -n hack/verify-integration.sh`)
- **Phase gate:** `go test ./...` green + `./hack/verify-integration.sh` all-PASS before `/gsd:verify-work`

### Wave 0 Gaps

All of the following must be created before implementation tasks can write tests:

- [ ] `pkg/cluster/internal/create/actions/installcorednstuning/corefile_test.go` — covers DNS-01 through DNS-03 (string transforms)
- [ ] `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver_test.go` — covers MET-01 (manifest embed check, `--kubelet-insecure-tls` presence)
- [ ] `pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go` — covers DASH-03 (base64 decode roundtrip)
- [ ] `pkg/cluster/internal/create/create_addon_test.go` — covers `logAddonSummary`, `logMetalLBPlatformWarning`
- [ ] `hack/verify-integration.sh` — covers all 5 success criteria, all 34 requirements at live-cluster level

*(The 15 existing tests in `installmetallb` already cover MLB-05, MLB-06, MLB-07 and the subnet logic. No gap there.)*

---

## Sources

### Primary (HIGH confidence)

- Kinder codebase, direct file reads (2026-03-01):
  - `pkg/cluster/internal/create/actions/installmetallb/subnet_test.go` — canonical test pattern for the project
  - `pkg/cluster/internal/create/actions/installmetallb/subnet.go` — testable pure functions pattern
  - `pkg/internal/apis/config/encoding/load_test.go` — table-driven test + testdata pattern
  - `pkg/internal/apis/config/validate_test.go` — parallel test + t.Run pattern
  - `pkg/cluster/internal/create/create.go` — `logAddonSummary`, `runAddon`, `addonResult` types
  - `pkg/cluster/internal/create/actions/installdashboard/dashboard.go` — base64 decode logic
  - All six VERIFICATION.md files — catalogued human-needed items that the script must exercise
  - `go test ./...` output (2026-03-01) — confirmed all 15 existing tests pass; build is clean

### Secondary (MEDIUM confidence)

- Go stdlib `testing` package documentation — table-driven tests, t.Run, t.Parallel patterns (stable, well-known)
- `hack/make-rules/test.sh` — confirms `MODE=unit` runs `go test ./...`

### Tertiary (LOW confidence)

- `curlimages/curl` Docker Hub availability — assumed available; flag in script as substitutable
- `registry.k8s.io/e2e-test-images/agnhost:2.43` availability — commonly used in Kubernetes e2e tests; likely available

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all tools already in repo, no new dependencies
- Architecture patterns: HIGH — derived directly from existing test files in the codebase
- Pitfalls: HIGH for pure Go (well-understood), MEDIUM for shell script edge cases (platform-specific curl behavior)
- Validation Architecture: HIGH — based on reading all prior VERIFICATION.md files and understanding exactly what was deferred

**Research date:** 2026-03-01
**Valid until:** 2026-06-01 (stable — Go testing patterns and bash are not fast-moving)
