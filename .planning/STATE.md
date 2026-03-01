# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-01)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** Phase 8 — Gap Closure (complete)

## Current Position

Phase: 8 of 8 (Gap Closure) — COMPLETE
Plan: 2 of 2 in current phase (phase complete)
Status: Phase 8 complete, all phases complete -- v1 milestone reached and hardened
Last activity: 2026-03-01 — Completed 08-01: Removed all-false addon guard from SetDefaultsCluster and redundant SetDefaultsCluster call from fixupOptions; added unit tests proving fix. Both 08-01 and 08-02 complete.

Progress: [██████████] 100%

## Performance Metrics

**Velocity:**
- Total plans completed: 10
- Average duration: 2 minutes
- Total execution time: 0.33 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation | 2 | 4 min | 2 min |
| 02-metallb | 2 | 3 min | 1.5 min |
| 03-metrics-server | 1 | 1 min | 1 min |
| 04-coredns-tuning | 1 | 2 min | 2 min |
| 05-envoy-gateway | 1 | 3 min | 3 min |
| 06-dashboard | 1 | 1 min | 1 min |

| 07-integration-testing | 2 | 4 min | 2 min |

**Recent Trend:**
- Last 5 plans: 04-01 (2m), 05-01 (3m), 06-01 (1m), 07-01 (2m), 07-02 (2m)
- Trend: Stable

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Research]: Use Headlamp v0.40.1 instead of kubernetes/dashboard (archived Jan 2026, Helm dependency)
- [Research]: Embed all addon manifests at build time using go:embed (offline-capable, no external tools)
- [Research]: MetalLB must be fully ready before Envoy Gateway actions run (hard ordering dependency)
- [Research]: CoreDNS patching uses read-modify-write in Go, not kubectl patch (Corefile is a string blob)
- [01-01]: *bool for v1alpha4 Addons fields — nil means not-set, defaults to true; avoids Go bool zero-value ambiguity after YAML decode
- [01-01]: Internal config uses plain bool — conversion is the single point where nil *bool => true translation happens
- [01-01]: Binary renamed via Makefile + cobra Use field only — module path stays sigs.k8s.io/kind
- [01-02]: Addon loop uses a closure (runAddon) rather than a top-level function — keeps actionsContext and addonResults in natural scope
- [01-02]: Platform warning fires after all addon runs, before summary — groups warning visually with addon results
- [01-02]: Salutation updated "kind" to "kinder" — URLs left pointing to kind docs until kinder docs exist
- [02-01]: carvePoolFromSubnet uses broadcast-address arithmetic — computes broadcast, then sets last octet to .200-.250; handles /16, /24, /20 automatically
- [02-01]: parseSubnetFromJSON branches on providerName=="podman" only — Docker and Nerdctl share IPAM.Config schema so no third branch needed
- [02-02]: fmt.Stringer type assertion for provider name — Provider interface lacks String(), type assertion with "docker" fallback avoids interface pollution
- [02-02]: MetalLB manifest embedded at build time via go:embed — pinned to v0.15.3, no network required at cluster creation
- [02-02]: Webhook wait targets deployment/controller Available (120s) before CR application to avoid webhook not ready errors
- [03-01]: Metrics Server manifest embedded at build time via go:embed — pinned to v0.8.1, no network required at cluster creation
- [03-01]: --kubelet-insecure-tls pre-patched into manifest — mandatory because kind kubelets serve self-signed TLS certificates
- [03-01]: Namespace is kube-system (not a dedicated namespace); no webhook wait or CR application needed
- [04-01]: CoreDNS Corefile patched via in-memory read-modify-write: kubectl get with jsonpath, three string transforms, kubectl apply -f - with YAML envelope
- [04-01]: Guard checks (pods insecure, cache 30, kubernetes cluster.local) added to fail safely if Corefile format changes upstream
- [04-01]: indentCorefile helper prepends 4 spaces to each non-empty line for valid YAML literal block scalar embedding
- [04-01]: No go:embed needed — Corefile read live from cluster at action time, not embedded at build time
- [05-01]: --server-side apply required for install.yaml because httproutes CRD is 372 KB, exceeding 256 KB last-applied-configuration annotation limit
- [05-01]: Wait for eg-gateway-helm-certgen Job Complete before deployment wait — Job creates TLS Secrets the controller requires to start
- [05-01]: GatewayClass "eg" applied separately after Deployment Available — not included in install.yaml, requires running controller to be accepted
- [05-01]: GatewayClass apply uses standard (not server-side) apply — resource is tiny (< 1 KB), avoids unnecessary field ownership complexity
- [06-01]: Headlamp manifest applied via kubectl stdin (standard apply, not server-side — manifest is < 10 KB)
- [06-01]: base64 decoded in Go not shell — avoids cross-platform base64 flag differences (GNU vs BSD -d vs -D)
- [06-01]: ctx.Status.End(true) called before Logger output — spinner must end before multi-line token print
- [06-01]: Long-lived token via kubernetes.io/service-account-token Secret — survives pod restarts, no TTL
- [07-01]: Extracted patchCorefile as package-level function returning (string, error) for hermetic unit testing without a live cluster
- [07-01]: Guard check error messages preserved exactly from Execute() to maintain identical runtime behavior
- [07-01]: Test uses realistic kind Corefile constant rather than minimal input for higher confidence
- [07-02]: testLogger captures all log levels into a single lines slice for simple output assertions in same-package tests
- [07-02]: Platform-aware assertion in TestLogMetalLBPlatformWarning uses runtime.GOOS to branch expected output (darwin vs linux)
- [07-02]: Integration script uses set -uo pipefail without set -e so all checks run regardless of earlier failures
- [07-02]: SC-2 curl runs inside the cluster via kubectl run because MetalLB IPs are unreachable on macOS/Windows hosts
- [08-01]: All-false addon guard removed from SetDefaultsCluster — encoding.Load('') path is the single correct defaulting point for library usage
- [08-01]: Redundant SetDefaultsCluster call removed from fixupOptions in create.go — opts.Config is already defaulted via encoding.Load('') for nil configs
- [08-02]: kubectl context switch placed inside success branch only — avoids secondary error when cluster creation fails
- [08-02]: Context name uses kind- prefix (kind-${CLUSTER_NAME}) matching kind's automatic kubectl context naming convention

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 2]: Podman rootless MetalLB viability (L2 speaker + raw sockets) needs testing during implementation

## Session Continuity

Last session: 2026-03-01
Stopped at: Completed 08-01-PLAN.md — all-false addon guard removed, unit tests added. Both gap closure plans complete. All 8 phases complete. v1 milestone fully hardened.
Resume file: None
