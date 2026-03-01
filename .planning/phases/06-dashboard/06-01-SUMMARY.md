---
phase: 06-dashboard
plan: "01"
subsystem: installdashboard
tags: [headlamp, dashboard, rbac, go-embed, base64, service-account-token]
dependency_graph:
  requires: [05-01]
  provides: [dashboard-access, headlamp-deployment, kinder-dashboard-token]
  affects: [pkg/cluster/internal/create/actions/installdashboard]
tech_stack:
  added: [Headlamp v0.40.1]
  patterns: [go:embed, kubectl stdin apply, base64.StdEncoding.DecodeString, ctx.Logger.V(0).Infof]
key_files:
  created:
    - pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
  modified:
    - pkg/cluster/internal/create/actions/installdashboard/dashboard.go
decisions:
  - "Headlamp manifest applied via kubectl stdin (standard apply, not server-side — manifest is < 10 KB)"
  - "base64 decoded in Go not shell — avoids cross-platform base64 flag differences (GNU vs BSD -d vs -D)"
  - "ctx.Status.End(true) called before Logger output — spinner must end before multi-line token print"
  - "Long-lived token via kubernetes.io/service-account-token Secret — survives pod restarts, no TTL"
metrics:
  duration: "1 minute"
  completed: "2026-03-01"
  tasks_completed: 2
  files_modified: 2
---

# Phase 6 Plan 1: Headlamp Dashboard Implementation Summary

**One-liner:** Headlamp v0.40.1 installed via embedded manifest with cluster-admin RBAC, long-lived service account token decoded in Go, and port-forward command printed to stdout after cluster creation.

## What Was Built

Replaced the stub `installdashboard` action with a complete working implementation that:

1. Embeds a complete `headlamp.yaml` manifest (5 resources) at build time via `go:embed`
2. Applies the manifest via `kubectl apply -f -` using stdin (standard apply, < 10 KB)
3. Waits for `deployment/headlamp` to reach `Available` condition (120s timeout)
4. Reads the `kinder-dashboard-token` Secret via `kubectl get` with `jsonpath={.data.token}` captured into a `bytes.Buffer`
5. Decodes base64 in Go using `base64.StdEncoding.DecodeString` (not shell, avoids GNU/BSD flag differences)
6. Ends the spinner with `ctx.Status.End(true)` before printing multi-line output
7. Prints the decoded JWT token and `kubectl port-forward` command via `ctx.Logger.V(0).Infof`

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create Headlamp v0.40.1 manifest with RBAC and long-lived token Secret | a05000fa | manifests/headlamp.yaml |
| 2 | Implement dashboard Execute function with manifest apply, wait, token decode, output | 90d19027 | dashboard.go |

## Manifest Resources (headlamp.yaml)

All 5 resources in dependency order:

1. **ServiceAccount** `kinder-dashboard` in `kube-system`
2. **ClusterRoleBinding** `kinder-dashboard` -> `cluster-admin` with `namespace: kube-system` in subjects (required for ServiceAccount kind)
3. **Secret** `kinder-dashboard-token` type `kubernetes.io/service-account-token` with `kubernetes.io/service-account.name: kinder-dashboard` annotation — long-lived, survives pod restarts
4. **Service** `headlamp` port 80->4466 with `k8s-app: headlamp` selector
5. **Deployment** `headlamp` image `ghcr.io/headlamp-k8s/headlamp:v0.40.1`, readiness/liveness probes on HTTP GET / port 4466 (initialDelaySeconds 30, timeoutSeconds 30), no OpenTelemetry env vars

## User Experience After Cluster Creation

```
 * Installing Dashboard ... done

Dashboard:
  Token: eyJhbGciOiJSUzI1NiIsImtpZCI6Ii...
  Port-forward: kubectl port-forward -n kube-system service/headlamp 8080:80
  Then open: http://localhost:8080
```

Developer workflow: copy the port-forward command, run it in a terminal, paste the token in the Headlamp UI. Zero manual setup.

## Deviations from Plan

None — plan executed exactly as written.

## Verification Results

All 10 post-execution checks passed:

1. `go build ./...` — PASS
2. `go vet ./pkg/cluster/internal/create/actions/installdashboard/...` — PASS
3. Manifest contains all 5 resource types — PASS
4. Image pinned to `ghcr.io/headlamp-k8s/headlamp:v0.40.1` (not `latest`) — PASS
5. No OpenTelemetry env vars in manifest — PASS
6. `go:embed manifests/headlamp.yaml` directive in dashboard.go — PASS
7. `encoding/base64` imported and `base64.StdEncoding.DecodeString` called — PASS
8. `ctx.Status.End(true)` called before Logger output — PASS
9. `ctx.Logger.V(0)` used for token and port-forward printing — PASS
10. ClusterRoleBinding subjects includes `namespace: kube-system` — PASS

## Self-Check: PASSED

Files verified:
- `pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml` — FOUND
- `pkg/cluster/internal/create/actions/installdashboard/dashboard.go` — FOUND

Commits verified:
- `a05000fa` — FOUND (feat(06-01): add Headlamp v0.40.1 manifest with RBAC and long-lived token Secret)
- `90d19027` — FOUND (feat(06-01): implement Headlamp dashboard Execute function with token decode and output)
