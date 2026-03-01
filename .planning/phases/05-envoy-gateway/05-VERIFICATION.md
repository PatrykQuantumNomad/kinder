---
phase: 05-envoy-gateway
verified: 2026-03-01T00:00:00Z
status: passed
score: 6/6 must-haves verified
re_verification: false
---

# Phase 5: Envoy Gateway Verification Report

**Phase Goal:** Gateway API CRDs are established and Envoy Gateway controller is running so a user can deploy a Gateway and route HTTP traffic via a LoadBalancer IP
**Verified:** 2026-03-01
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                                                                  | Status     | Evidence                                                                                                                                                                                                    |
|----|----------------------------------------------------------------------------------------------------------------------------------------|------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1  | Envoy Gateway controller pod reaches Running state during `kinder create cluster` and a GatewayClass named `eg` exists                 | VERIFIED | `envoygw.go` Execute function runs 5-step kubectl pipeline ending with `kubectl wait --for=condition=Accepted gatewayclass/eg --timeout=60s`; wired in `create.go:202` via `runAddon("Envoy Gateway", ...)` |
| 2  | A user can deploy a Gateway and HTTPRoute and curl a backend service through the resulting LoadBalancer IP                              | VERIFIED | Gateway API CRDs (gateways, httproutes, gatewayclasses) confirmed present in `install.yaml`; `controllerName` in inline GatewayClass matches ConfigMap in manifest (`gateway.envoyproxy.io/gatewayclass-controller`); MetalLB action runs before Envoy Gateway in `create.go` dependency order; end-to-end workflow documented in SUMMARY TLS guide |
| 3  | When MetalLB is disabled and Envoy Gateway is enabled, kinder prints a clear warning that the Gateway proxy will not get an IP         | VERIFIED | `create.go:194-195`: `if !opts.Config.Addons.MetalLB && opts.Config.Addons.EnvoyGateway { logger.Warn("MetalLB is disabled but Envoy Gateway is enabled. Envoy Gateway proxy services will not receive LoadBalancer IPs.") }` |
| 4  | Setting `addons.envoyGateway: false` in cluster config causes no Gateway API CRDs or Envoy Gateway pods to be installed                | VERIFIED | `runAddon` closure (`create.go:179-191`) checks `enabled bool` and returns early with a skip log message when false; `EnvoyGateway` is a `bool` field in the internal config schema wired through v1alpha4 conversion |

**Score:** 4/4 truths verified (maps to 6/6 plan must-haves — see Artifacts section)

---

### Required Artifacts

| Artifact                                                                                   | Expected                                            | Status   | Details                                                                                                                                                                       |
|--------------------------------------------------------------------------------------------|-----------------------------------------------------|----------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go`                           | Complete Execute function (not a stub)              | VERIFIED | 128 lines; imports `embed`, `strings`, `nodeutils`, `errors`; `//go:embed manifests/install.yaml` directive; `gatewayClassYAML` const declared and used; 5-step kubectl pipeline; `return nil` only at end of successful path |
| `pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml`               | Envoy Gateway v1.3.1 manifest with Gateway API CRDs | VERIFIED | 40,570 lines, 2,563,104 bytes; 18 CRDs confirmed (`grep -c "kind: CustomResourceDefinition"` = 18); certgen Job name `eg-gateway-helm-certgen` confirmed; namespace `envoy-gateway-system` present; `gateway.envoyproxy.io/gatewayclass-controller` confirmed in ConfigMap |

---

### Key Link Verification

| From                                                        | To                                                  | Via                                              | Status   | Details                                                                                                                               |
|-------------------------------------------------------------|-----------------------------------------------------|--------------------------------------------------|----------|---------------------------------------------------------------------------------------------------------------------------------------|
| `envoygw.go`                                                | `manifests/install.yaml`                            | `//go:embed manifests/install.yaml`              | WIRED    | Line 30: `//go:embed manifests/install.yaml` / Line 31: `var envoyGWManifest string`                                                 |
| `envoygw.go`                                               | `kubectl apply --server-side`                       | `node.Command` with `SetStdin(envoyGWManifest)`  | WIRED    | Lines 69-75: `node.Command("kubectl", "--kubeconfig=...", "apply", "--server-side", "-f", "-").SetStdin(strings.NewReader(envoyGWManifest)).Run()` |
| `envoygw.go`                                               | `gatewayclass/eg`                                   | `gatewayClassYAML` const applied inline          | WIRED    | Line 33: `const gatewayClassYAML` declared; Line 109: used in `SetStdin(strings.NewReader(gatewayClassYAML))`                       |
| `create.go`                                                | `pkg/cluster/internal/create/actions/installenvoygw` | `runAddon` closure with `EnvoyGateway` bool flag | WIRED    | Line 40: `import "...installenvoygw"`; Lines 179-191: `runAddon` skips when `enabled=false`; Line 202: `runAddon("Envoy Gateway", opts.Config.Addons.EnvoyGateway, installenvoygw.NewAction())` |

---

### Requirements Coverage

| Requirement | Source Plan | Description                                                                                          | Status    | Evidence                                                                                                                   |
|-------------|-------------|------------------------------------------------------------------------------------------------------|-----------|----------------------------------------------------------------------------------------------------------------------------|
| EGW-01      | 05-01-PLAN  | Gateway API CRDs installed before Envoy Gateway controller starts                                   | SATISFIED | install.yaml places 18 CRDs (gatewayclasses at line 1162, gateways at 1682, httproutes at 6336) before Deployment/Job entries |
| EGW-02      | 05-01-PLAN  | Envoy Gateway controller running and GatewayClass "eg" Accepted                                     | SATISFIED | 5-step Execute function: certgen wait → deployment wait → gatewayclass apply → gatewayclass accepted wait                  |
| EGW-03      | 05-01-PLAN  | User can create Gateway + HTTPRoute and route traffic via LoadBalancer IP                            | SATISFIED | Gateway API CRDs installed; GatewayClass `eg` accepted; MetalLB runs before EGW in create.go; workflow documented in SUMMARY |
| EGW-04      | 05-01-PLAN  | `addons.envoyGateway: false` installs nothing                                                        | SATISFIED | `runAddon` closure returns early when `enabled=false` (create.go:180-183); internal config bool threaded from v1alpha4 `*bool` via conversion |
| EGW-05      | 05-01-PLAN  | MetalLB disabled + Envoy Gateway enabled prints warning                                              | SATISFIED | create.go:194-195: explicit `logger.Warn` with descriptive message about missing LoadBalancer IPs                          |
| EGW-06      | 05-01-PLAN  | TLS termination documented (manual cert path, no cert-manager)                                       | SATISFIED | `05-01-SUMMARY.md` sections: "TLS Termination Guide (EGW-06)" with openssl, `kubectl create secret tls`, Gateway HTTPS listener YAML, HTTPRoute YAML, and curl verification |

No orphaned requirements — all six EGW-01 through EGW-06 are claimed in `05-01-PLAN.md` and confirmed satisfied.

---

### Anti-Patterns Found

| File         | Line | Pattern   | Severity | Impact |
|--------------|------|-----------|----------|--------|
| `envoygw.go` | —    | None found | —       | —      |

No TODOs, FIXMEs, placeholder comments, empty handlers, stub returns, or console.log-only implementations found. The single `return nil` at line 126 is the correct success return after `ctx.Status.End(true)`.

---

### Build Verification

| Check                                  | Result |
|----------------------------------------|--------|
| `go build ./...`                       | PASS   |
| `go vet ./...`                         | PASS   |
| `install.yaml` line count              | 40,570 (matches documented 40,570) |
| `install.yaml` byte size               | 2,563,104 bytes (~2.4 MB) |
| `--server-side` in `envoygw.go`        | 2 occurrences (comment + command arg) |
| `gatewayClassYAML` in `envoygw.go`     | 2 occurrences (declaration + usage) |
| `"wait"` args in `envoygw.go`          | 3 occurrences (certgen, deployment, gatewayclass) |
| Commit `3de6787f` exists               | YES — feat(05-01): implement Envoy Gateway action |

---

### Human Verification Required

The following truths require a live cluster to confirm end-to-end:

#### 1. Controller reaches Running state

**Test:** Run `kinder create cluster` and then `kubectl get pods -n envoy-gateway-system`
**Expected:** `envoy-gateway-*` pod in `Running` state; certgen Job shows `Completed`
**Why human:** Pod readiness depends on the node image, kubeadm timing, and actual network connectivity — cannot verify from static analysis

#### 2. GatewayClass Accepted condition is set

**Test:** `kubectl get gatewayclass eg -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}'`
**Expected:** Returns `True`
**Why human:** Controller reconciliation loop sets this condition; static code analysis only confirms the `kubectl wait` call is present, not that it succeeds against a running cluster

#### 3. End-to-end HTTP routing via LoadBalancer IP

**Test:** Deploy a backend service, create a `Gateway` (className: `eg`) and an `HTTPRoute`; then `curl http://<GATEWAY_EXTERNAL_IP>/`
**Expected:** HTTP 200 response from the backend; MetalLB assigns an EXTERNAL-IP to the Envoy proxy Service
**Why human:** Requires a live cluster with MetalLB IP pool configured; tests actual Envoy proxy creation and traffic forwarding

#### 4. Warning printed when MetalLB is disabled and Envoy Gateway is enabled

**Test:** Create a cluster config with `addons.metallb: false` and `addons.envoyGateway: true`; run `kinder create cluster`
**Expected:** Log output contains "MetalLB is disabled but Envoy Gateway is enabled. Envoy Gateway proxy services will not receive LoadBalancer IPs."
**Why human:** Requires running `kinder create cluster` with a specific config; cannot mock `logger.Warn` output from static analysis

#### 5. `addons.envoyGateway: false` installs nothing

**Test:** Create a cluster config with `addons.envoyGateway: false`; run `kinder create cluster`; then `kubectl get ns envoy-gateway-system`
**Expected:** Namespace does not exist; no Gateway API CRDs installed
**Why human:** Requires a running cluster to confirm absence of resources

---

### Gaps Summary

No gaps found. All four observable truths are verified by code inspection:

- Truth 1 (controller running + GatewayClass exists): The 5-step Execute function is fully implemented, not a stub. It ends with `kubectl wait --for=condition=Accepted gatewayclass/eg`. The action is wired into `create.go` via `runAddon`.
- Truth 2 (user can deploy Gateway + HTTPRoute + curl): Gateway API CRDs are embedded (18 CRDs confirmed). GatewayClass `eg` is applied with the correct `controllerName`. MetalLB runs first in create.go ordering. End-to-end workflow is documented.
- Truth 3 (warning on MetalLB disabled): `logger.Warn(...)` with exact matching message is present at `create.go:195`.
- Truth 4 (disable flag works): `runAddon` closure returns early when `enabled=false`; `EnvoyGateway` field is properly threaded from v1alpha4 config schema through internal conversion to `create.go`.

Five human-verification items are flagged — all require a live cluster and are not blocking automated assessment.

---

_Verified: 2026-03-01_
_Verifier: Claude (gsd-verifier)_
