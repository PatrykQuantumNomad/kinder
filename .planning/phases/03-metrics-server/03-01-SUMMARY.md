---
phase: 03-metrics-server
plan: "01"
subsystem: installmetricsserver
tags: [metrics-server, kubectl-top, hpa, embed, kube-system]
dependency_graph:
  requires: [02-02]
  provides: [metrics-server-action]
  affects: [cluster-create-pipeline]
tech_stack:
  added: [metrics-server-v0.8.1]
  patterns: [go-embed, kubectl-apply-stdin, kubectl-wait]
key_files:
  created:
    - pkg/cluster/internal/create/actions/installmetricsserver/manifests/components.yaml
  modified:
    - pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go
decisions:
  - "Manifest embedded at build time via go:embed (offline-capable, no network at cluster creation)"
  - "--kubelet-insecure-tls pre-patched into manifest (mandatory for kind self-signed TLS)"
  - "Namespace is kube-system (not a dedicated namespace like MetalLB)"
  - "No webhook wait needed (Metrics Server has no ValidatingWebhookConfiguration)"
  - "No CR application needed (no IPAddressPool equivalent)"
metrics:
  duration: "~1 minute"
  completed: "2026-03-01"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
requirements: [MET-01, MET-02, MET-03, MET-04, MET-05]
---

# Phase 3 Plan 01: Metrics Server Action Summary

**One-liner:** Metrics Server v0.8.1 embedded via go:embed with --kubelet-insecure-tls pre-patched, applied via kubectl stdin with deployment readiness wait in kube-system.

## What Was Built

The Metrics Server addon action replaces a stub implementation with a working Execute function that:

1. Embeds `manifests/components.yaml` (Metrics Server v0.8.1) at build time via `go:embed`
2. Gets the control plane node via `nodeutils.ControlPlaneNodes`
3. Applies the manifest via `kubectl apply -f -` with stdin on the control plane node
4. Waits for `deployment/metrics-server` to become Available in kube-system (120s timeout)

This enables `kubectl top nodes`, `kubectl top pods`, and HPA CPU/memory metrics in kind clusters.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Embed Metrics Server v0.8.1 manifest with --kubelet-insecure-tls | e753994d | manifests/components.yaml (created) |
| 2 | Implement Metrics Server action Execute function | f02805b2 | metricsserver.go (replaced stub) |

## Key Implementation Details

### Manifest (components.yaml)

- Downloaded from official `kubernetes-sigs/metrics-server` v0.8.1 GitHub release
- `--kubelet-insecure-tls` added to Deployment container args (mandatory because kind kubelets use self-signed TLS certificates)
- Contains all required resources: ServiceAccount, ClusterRole (x2), ClusterRoleBinding (x2), RoleBinding, Service, Deployment, APIService (v1beta1.metrics.k8s.io)
- Image: `registry.k8s.io/metrics-server/metrics-server:v0.8.1`

### Action (metricsserver.go)

Follows the MetalLB action pattern exactly, but simpler:

```go
//go:embed manifests/components.yaml
var metricsServerManifest string

func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing Metrics Server")
    defer ctx.Status.End(false)

    allNodes, err := ctx.Nodes()
    // ... get control plane node ...

    // Apply manifest
    node.Command("kubectl", "--kubeconfig=/etc/kubernetes/admin.conf", "apply", "-f", "-").
        SetStdin(strings.NewReader(metricsServerManifest)).Run()

    // Wait for deployment
    node.Command("kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait", "--namespace=kube-system",
        "--for=condition=Available", "deployment/metrics-server",
        "--timeout=120s").Run()

    ctx.Status.End(true)
    return nil
}
```

## Deviations from Plan

None — plan executed exactly as written.

## Verification Results

All 8 verification checks passed:

1. `go build ./pkg/cluster/internal/create/actions/installmetricsserver/...` — PASS
2. `go vet ./pkg/cluster/internal/create/actions/installmetricsserver/...` — PASS
3. `go build ./...` — PASS (full project builds)
4. `--kubelet-insecure-tls` present exactly once in manifest — PASS
5. `registry.k8s.io/metrics-server/metrics-server:v0.8.1` image reference — PASS
6. `v1beta1.metrics.k8s.io` APIService resource — PASS
7. `go:embed manifests/components.yaml` directive in metricsserver.go — PASS
8. `kubectl wait --namespace=kube-system deployment/metrics-server` in metricsserver.go — PASS

## Self-Check: PASSED

- FOUND: pkg/cluster/internal/create/actions/installmetricsserver/manifests/components.yaml
- FOUND: pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go
- FOUND commit: e753994d (Task 1)
- FOUND commit: f02805b2 (Task 2)
