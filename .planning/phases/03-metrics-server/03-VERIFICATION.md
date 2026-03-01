---
phase: 03-metrics-server
verified: 2026-03-01T00:00:00Z
status: human_needed
score: 5/7 must-haves verified (2 require runtime human verification)
human_verification:
  - test: "kubectl top nodes returns CPU/memory data within 60s of kinder create cluster completing"
    expected: "Output shows node names with CPU cores/millicores and memory in MiB within 60 seconds"
    why_human: "Requires a live kind cluster; cannot verify kubelet metrics scraping works without running the cluster"
  - test: "HPA can read CPU/memory metrics from v1beta1.metrics.k8s.io APIService without errors"
    expected: "kubectl get apiservice v1beta1.metrics.k8s.io shows Available=True; HPA targeting CPU shows TARGETS with real values not <unknown>"
    why_human: "Requires running HPA workload against live cluster to confirm metrics API is reachable by the HPA controller"
---

# Phase 3: Metrics Server Verification Report

**Phase Goal:** `kubectl top nodes` and `kubectl top pods` return data immediately after cluster creation and HPA can read the metrics API
**Verified:** 2026-03-01
**Status:** human_needed
**Re-verification:** No (initial verification)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Metrics Server v0.8.1 manifest is embedded at build time with --kubelet-insecure-tls pre-patched | VERIFIED | `manifests/components.yaml` exists; grep confirms `--kubelet-insecure-tls` appears exactly once inside Deployment args section at line 141 |
| 2 | kubectl apply applies the manifest to kube-system namespace during cluster creation | VERIFIED | `metricsserver.go` line 60-62: `node.Command("kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "apply", "-f", "-").SetStdin(strings.NewReader(metricsServerManifest)).Run()` wired to control plane node |
| 3 | Action waits for deployment/metrics-server Available before returning | VERIFIED | `metricsserver.go` lines 66-70: `node.Command("kubectl", ..., "wait", "--namespace=kube-system", "--for=condition=Available", "deployment/metrics-server", "--timeout=120s")` |
| 4 | kubectl top nodes returns CPU/memory data within 60 seconds of cluster creation | NEEDS HUMAN | Static analysis cannot verify kubelet scraping succeeds at runtime |
| 5 | kubectl top pods returns data for pods in any namespace | NEEDS HUMAN | Static analysis cannot verify the metrics API serves per-pod data without running a cluster |
| 6 | HPA can read CPU/memory metrics from the v1beta1.metrics.k8s.io APIService | NEEDS HUMAN | Requires live HPA controller and workload to confirm |
| 7 | Setting addons.metricsServer: false skips Metrics Server installation entirely | VERIFIED | `create.go` line 200: `runAddon("Metrics Server", opts.Config.Addons.MetricsServer, installmetricsserver.NewAction())` — `runAddon` at line 180 returns immediately (skips) when `enabled=false`; `MetricsServer *bool` defaults to `true` via `boolPtrTrue` in `v1alpha4/default.go:88`; setting `false` in YAML converts via `boolVal()` in `convert_v1alpha4.go:53` |

**Score:** 4/7 truths verified by static analysis; 3/7 require human (runtime) verification.

Note: Truths 4, 5, 6 are downstream effects that depend on Truth 1-3 being correct. All upstream wiring (embed, apply, wait) is verified. Truths 4-6 require a running cluster to confirm.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/cluster/internal/create/actions/installmetricsserver/manifests/components.yaml` | Metrics Server v0.8.1 manifest with --kubelet-insecure-tls | VERIFIED | File exists; contains `--kubelet-insecure-tls` exactly once (line 141); image `registry.k8s.io/metrics-server/metrics-server:v0.8.1` confirmed; `v1beta1.metrics.k8s.io` APIService present; all expected resources present (ServiceAccount, ClusterRole x2, ClusterRoleBinding x2, RoleBinding, Service, Deployment, APIService) |
| `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go` | Complete Metrics Server action Execute function, exports NewAction, min 40 lines | VERIFIED | File is 75 lines; exports `NewAction()`; Execute function is substantive (not a stub): embeds manifest, gets control plane node, applies manifest, waits for readiness; `go build` and `go vet` both pass |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `metricsserver.go` | `manifests/components.yaml` | `go:embed` directive | VERIFIED | Line 30: `//go:embed manifests/components.yaml` immediately above `var metricsServerManifest string` |
| `metricsserver.go` | `kubectl apply -f -` on control plane node | `node.Command` with `SetStdin` | VERIFIED | Line 60-62: `node.Command("kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "apply", "-f", "-").SetStdin(strings.NewReader(metricsServerManifest)).Run()` — manifest variable is the embedded content |
| `metricsserver.go` | `kubectl wait deployment/metrics-server` | `node.Command` wait | VERIFIED | Lines 66-70: `node.Command("kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "wait", "--namespace=kube-system", "--for=condition=Available", "deployment/metrics-server", "--timeout=120s").Run()` — all expected arguments present |
| `create.go` | `installmetricsserver.NewAction()` | `runAddon` | VERIFIED | Line 200 of `create.go`: `runAddon("Metrics Server", opts.Config.Addons.MetricsServer, installmetricsserver.NewAction())` — package imported at line 42 |

Note: The plan's `key_links[2]` pattern `"wait.*deployment/metrics-server.*timeout=120s"` did not match as a single-line regex because the arguments are split across multiple lines in a multi-argument `node.Command(...)` call. The arguments are all present and correct — this is a grep pattern limitation, not an implementation issue.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| MET-01 | 03-01 | Metrics Server installed with --kubelet-insecure-tls by default | SATISFIED | `components.yaml` has flag in Deployment args; action wired in `create.go` with default `true` |
| MET-02 | 03-01 | kubectl top nodes returns data within 60s of cluster creation | NEEDS HUMAN | Static: deployment wait is 120s max; actual top latency is runtime behavior |
| MET-03 | 03-01 | kubectl top pods works for any namespace | NEEDS HUMAN | Depends on MET-02 being true at runtime |
| MET-04 | 03-01 | HPA can read CPU/memory from Metrics API | NEEDS HUMAN | APIService resource `v1beta1.metrics.k8s.io` is present in manifest — runtime verification needed |
| MET-05 | 03-01 | User can disable via addons.metricsServer: false | SATISFIED | `runAddon` skips when `enabled=false`; `*bool` -> `bool` conversion chain verified end-to-end |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | — | — | No anti-patterns found |

No TODOs, FIXMEs, placeholders, empty implementations, or stub patterns found in either modified file.

### Human Verification Required

#### 1. kubectl top nodes returns CPU and memory data within 60 seconds

**Test:** Create a cluster with `kinder create cluster`, wait for it to complete, then immediately run `kubectl top nodes` in a loop for up to 60 seconds.
**Expected:** Within 60 seconds of cluster creation completing, `kubectl top nodes` returns a table with node name, CPU(cores), CPU%, MEMORY(bytes), MEMORY% — no "Metrics API not available" or "error: metrics not available yet" messages.
**Why human:** Requires a running kind cluster. Static analysis confirms the manifest is applied and the wait condition is set, but cannot verify that the kubelet metrics scraping succeeds within the expected time window with `--kubelet-insecure-tls`.

#### 2. HPA reads CPU/memory metrics from v1beta1.metrics.k8s.io

**Test:** Deploy a workload with a CPU HPA (`kubectl autoscale deployment ...`), then run `kubectl get hpa` and check the TARGETS column.
**Expected:** TARGETS shows actual values (e.g., `5%/50%`) not `<unknown>/50%`. Running `kubectl get apiservice v1beta1.metrics.k8s.io` should show `True` in the AVAILABLE column.
**Why human:** Requires a live HPA controller scraping the Metrics API. The manifest includes the APIService resource, but whether the HPA controller successfully reads from it depends on runtime cluster state.

### Gaps Summary

No gaps in the static-verifiable implementation. All build-time requirements are met:
- Manifest is embedded via `go:embed` with `--kubelet-insecure-tls` correctly patched into Deployment args
- Execute function is complete (75 lines, not a stub), applies the manifest via stdin, and waits for the deployment
- The addon is wired into the cluster creation pipeline in `create.go`
- The opt-out path (`addons.metricsServer: false`) works end-to-end through the config conversion chain

The 3 unverified truths (kubectl top nodes, kubectl top pods, HPA metrics) all require a running cluster and cannot be verified by static analysis alone. They are downstream effects of the correctly-wired implementation.

Full project build: PASS (`go build ./...`)
Package vet: PASS (`go vet ./pkg/cluster/internal/create/actions/installmetricsserver/...`)

---

_Verified: 2026-03-01_
_Verifier: Claude (gsd-verifier)_
