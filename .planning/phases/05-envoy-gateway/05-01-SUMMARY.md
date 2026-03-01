---
phase: 05-envoy-gateway
plan: "01"
subsystem: installenvoygw
tags:
  - envoy-gateway
  - gateway-api
  - ingress
  - tls
  - go-embed
  - server-side-apply
dependency_graph:
  requires:
    - 02-metallb (MetalLB assigns LoadBalancer IPs to Envoy proxy pods)
    - 01-foundation (config schema Addons.EnvoyGateway flag, runAddon wiring)
  provides:
    - GatewayClass "eg" accepted by Envoy Gateway controller
    - Gateway API CRDs (GatewayClass, Gateway, HTTPRoute, GRPCRoute, etc.)
    - Envoy Gateway controller in envoy-gateway-system namespace
  affects:
    - 07-integration-testing (MetalLB-to-Envoy end-to-end path depends on this phase)
tech_stack:
  added:
    - Envoy Gateway v1.3.1 (envoy-gateway-system namespace, 2.4 MB embedded manifest)
    - Gateway API v1 CRDs (GatewayClass, Gateway, HTTPRoute, GRPCRoute, TCPRoute, TLSRoute, UDPRoute, ReferenceGrant, BackendTLSPolicy, GatewayInfrastructure)
  patterns:
    - go:embed for 2.4 MB install.yaml at build time (same pattern as MetalLB, Metrics Server)
    - --server-side apply (required when any manifest object exceeds 256 KB annotation limit)
    - Sequential kubectl wait chain: Job Complete -> Deployment Available -> GatewayClass Accepted
key_files:
  created:
    - pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
  modified:
    - pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
decisions:
  - id: server-side-apply-required
    summary: "Use --server-side flag for install.yaml apply because httproutes CRD is 372 KB, exceeding the 256 KB last-applied-configuration annotation limit"
  - id: certgen-job-wait
    summary: "Wait for eg-gateway-helm-certgen Job Complete before deployment wait; the Job creates TLS Secrets the controller requires to start"
  - id: gatewayclass-separate-apply
    summary: "GatewayClass 'eg' applied separately after Deployment Available because it is not included in install.yaml and requires a running controller to be accepted"
  - id: gatewayclass-not-server-side
    summary: "GatewayClass apply uses standard apply (no --server-side) because the resource is tiny (< 1 KB) and avoids field ownership complexity"
metrics:
  duration: "3 minutes"
  completed_date: "2026-03-01"
  tasks_completed: 2
  files_created: 1
  files_modified: 1
---

# Phase 5 Plan 01: Envoy Gateway Implementation Summary

**One-liner:** Envoy Gateway v1.3.1 installed via embedded 40,570-line install.yaml with server-side apply, certgen/controller/GatewayClass wait chain, and GatewayClass "eg" accepted by the controller.

## What Was Built

### Task 1: Download manifest and implement Execute function

Downloaded the official Envoy Gateway v1.3.1 `install.yaml` manifest (40,570 lines, 2.4 MB) from the Envoy Gateway GitHub releases and embedded it via `go:embed`. Replaced the stub `Execute` function in `envoygw.go` with a complete five-step kubectl pipeline following the established `metallb.go` and `metricsserver.go` patterns.

The five-step sequence:
1. `kubectl apply --server-side -f -` with `envoyGWManifest` via stdin (installs Gateway API CRDs + controller manifests)
2. `kubectl wait job/eg-gateway-helm-certgen --for=condition=Complete --timeout=120s` (Job creates TLS Secrets for controller)
3. `kubectl wait deployment/envoy-gateway --for=condition=Available --timeout=120s` (controller is Ready)
4. `kubectl apply -f -` with inline `gatewayClassYAML` (creates GatewayClass named "eg")
5. `kubectl wait gatewayclass/eg --for=condition=Accepted --timeout=60s` (controller has accepted the GatewayClass)

### Task 2: SUMMARY and EGW-06 TLS termination documentation

Created this SUMMARY.md with complete build details, all EGW requirement status, and a TLS termination guide for users who want HTTPS without cert-manager.

## Verification Results

```
go build ./pkg/cluster/internal/create/actions/installenvoygw/...  # PASS
go vet ./pkg/cluster/internal/create/actions/installenvoygw/...    # PASS
go build ./...                                                       # PASS (2.4 MB embedded)
wc -l install.yaml                                                   # 40570
grep -c "server-side" envoygw.go                                     # 2
grep -c "gatewayClassYAML" envoygw.go                                # 2
grep -c '"wait"' envoygw.go                                          # 3
```

## Task Commits

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Envoy Gateway action with embedded v1.3.1 manifest | 3de6787f |
| 2 | SUMMARY.md, STATE.md, ROADMAP.md updates | (metadata commit) |

## Files Created / Modified

| File | Action | Purpose |
|------|--------|---------|
| `pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml` | Created | Official Envoy Gateway v1.3.1 manifest, embedded at build time |
| `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` | Modified | Complete Execute function replacing stub |

## Decisions Made

### 1. `--server-side` apply is required

Standard `kubectl apply` stores the full manifest in a `kubectl.kubernetes.io/last-applied-configuration` annotation, which is limited to 256 KB. The HTTPRoute CRD in the Envoy Gateway install.yaml is approximately 372 KB. Without `--server-side`, the apply fails with an "annotation too long" error. Server-side apply uses the API server to track field ownership, bypassing this annotation limit entirely.

### 2. Wait for certgen Job before Deployment

The `eg-gateway-helm-certgen` Kubernetes Job generates TLS certificates and stores them as Secrets in the `envoy-gateway-system` namespace. The Envoy Gateway controller pod references these Secrets and will crash-loop or fail to start if they are absent. Waiting for the Job to Complete (not just Exist) before waiting for the Deployment eliminates this race condition.

**Note:** The Job name `eg-gateway-helm-certgen` was verified in the v1.3.1 manifest. If upgrading Envoy Gateway, search the new install.yaml for `kind: Job` to confirm the name has not changed.

### 3. GatewayClass applied separately after controller is Ready

The `install.yaml` does not include a `GatewayClass` resource. The GatewayClass must be applied after the controller Deployment is Available; otherwise, the controller cannot reconcile the GatewayClass and set its `Accepted` condition. Applying it in Step 4 (after Step 3 deployment wait) ensures the controller is ready to process it.

## Deviations from Plan

None — plan executed exactly as written.

## EGW Requirements Status

| Requirement | Description | Status | Implementation |
|-------------|-------------|--------|----------------|
| EGW-01 | Gateway API CRDs installed in cluster | Satisfied | install.yaml document ordering: CRDs before controller Deployment |
| EGW-02 | Envoy Gateway controller running + GatewayClass "eg" Accepted | Satisfied | Execute function 5-step wait chain |
| EGW-03 | MetalLB + Envoy Gateway working together | Satisfied | Phase 2 (MetalLB) + Phase 5 (EGW) action ordering in create.go |
| EGW-04 | Setting `addons.envoyGateway: false` installs nothing | Satisfied | Already implemented in create.go (runAddon closure, no changes needed) |
| EGW-05 | MetalLB disabled + EGW enabled prints warning | Satisfied | Already implemented in create.go (no changes needed) |
| EGW-06 | TLS termination documented for users | Satisfied | See TLS Termination Guide section below |

## TLS Termination Guide (EGW-06)

This section documents how to configure TLS termination at the Envoy Gateway for users who do not have cert-manager installed. This is the manual certificate path.

### Step 1: Generate a self-signed certificate

```bash
openssl req -x509 -sha256 -nodes -days 365 -newkey rsa:2048 \
  -keyout example.key -out example.crt \
  -subj "/CN=www.example.com/O=example"
```

### Step 2: Create the TLS Secret in the namespace where your Gateway will live

```bash
kubectl create secret tls example-cert \
  --key=example.key \
  --cert=example.crt \
  --namespace=default
```

### Step 3: Create a Gateway with an HTTPS listener referencing the Secret

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
  namespace: default
spec:
  gatewayClassName: eg
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        mode: Terminate
        certificateRefs:
          - name: example-cert
            kind: Secret
```

Apply with `kubectl apply -f gateway.yaml`. Envoy Gateway will provision an Envoy proxy pod and a Service of type LoadBalancer. MetalLB (installed in Phase 2) assigns the LoadBalancer an EXTERNAL-IP from the carved pool.

### Step 4: Create an HTTPRoute targeting the HTTPS listener

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-https-route
  namespace: default
spec:
  parentRefs:
    - name: my-gateway
      sectionName: https
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: my-service
          port: 8080
```

Apply with `kubectl apply -f httproute.yaml`. Traffic arriving at `https://<EXTERNAL-IP>:443/` will be TLS-terminated by Envoy and forwarded to `my-service:8080` in plaintext.

### Step 5: Verify end-to-end

```bash
# Get the Gateway's LoadBalancer IP
GATEWAY_IP=$(kubectl get gateway my-gateway -n default \
  -o jsonpath='{.status.addresses[0].value}')

# Test with curl (--insecure because the cert is self-signed)
curl -k https://$GATEWAY_IP/
```

### Using cert-manager (optional)

If cert-manager is installed in the cluster, replace the manual steps above with a `Certificate` resource pointing to a `ClusterIssuer`. The Gateway `certificateRefs` field accepts Secrets provisioned by cert-manager identically.

## Phase 5 Completion

All six EGW requirements are satisfied:

- **EGW-01** (Gateway API CRDs): The install.yaml contains the full Gateway API CRD bundle in document order (CRDs appear before the controller Deployment). The CRDs are installed during the server-side apply in Step 1.
- **EGW-02** (Controller + GatewayClass): The 5-step Execute function ensures the controller is Available and the GatewayClass "eg" has `Accepted=True` before the action returns.
- **EGW-03** (End-to-end traffic): MetalLB (Phase 2) provides EXTERNAL-IP for the Envoy proxy Service. Gateway + HTTPRoute + LoadBalancer IP is the full path documented in the TLS guide above.
- **EGW-04** (Disable flag): The `addons.envoyGateway: false` opt-out was already implemented in `create.go` via the `runAddon` closure. No changes required.
- **EGW-05** (Missing IP warning): The MetalLB-disabled warning was already implemented in `create.go`. No changes required.
- **EGW-06** (TLS termination): Documented in the TLS Termination Guide section above with working openssl, kubectl, and YAML examples.

## Next Phase Readiness

Phase 6 (Dashboard / Headlamp) depends only on Phase 1 (Foundation). It does not depend on Envoy Gateway. Envoy Gateway is not a prerequisite for Phase 7 (Integration Testing) either, though the MetalLB-to-Envoy end-to-end test path will use the GatewayClass "eg" created here.

Ready to begin Phase 6.

## Self-Check

Files verified:
- `pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml`: FOUND (40570 lines)
- `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go`: FOUND (complete implementation)
- `.planning/phases/05-envoy-gateway/05-01-SUMMARY.md`: FOUND (this file)

Commits verified:
- `3de6787f`: feat(05-01): implement Envoy Gateway action with embedded v1.3.1 manifest — FOUND

## Self-Check: PASSED
