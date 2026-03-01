# Roadmap: Kinder

## Overview

Starting from the upstream kind fork, this roadmap builds kinder's batteries-included cluster experience in eight phases. Phase 1 lays the shared infrastructure (config schema and action scaffolding) that every subsequent phase depends on. Phases 2-6 each deliver one complete addon — MetalLB, Metrics Server, CoreDNS tuning, Envoy Gateway, and Dashboard — in dependency order. Phase 7 validates the full system with cross-addon integration tests. Phase 8 closes audit gaps from milestone verification. When all eight phases are complete, `kinder create cluster` produces a fully functional development cluster with no manual setup required.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - Config schema extensions and action pipeline scaffolding enabling addon opt-out
- [x] **Phase 2: MetalLB** - LoadBalancer IP assignment working on all three container providers
- [x] **Phase 3: Metrics Server** - kubectl top and HPA metrics functional within 60 seconds of cluster creation
- [x] **Phase 4: CoreDNS Tuning** - DNS cache improved and CoreDNS patched in-place without breaking resolution
- [x] **Phase 5: Envoy Gateway** - Gateway API CRDs and controller installed with end-to-end HTTPRoute traffic
- [x] **Phase 6: Dashboard** - Headlamp installed with printed token and port-forward command for immediate access
- [x] **Phase 7: Integration Testing** - All addons verified functional together via cross-addon smoke tests
- [ ] **Phase 8: Gap Closure** - Fix all-false config guard edge case and harden integration script

## Phase Details

### Phase 1: Foundation
**Goal**: The `kinder` binary exists with a backward-compatible config schema that supports addon opt-out, and the action pipeline accepts addon action hooks
**Depends on**: Nothing (first phase)
**Requirements**: FOUND-01, FOUND-02, FOUND-03, FOUND-04, FOUND-05
**Success Criteria** (what must be TRUE):
  1. Running `kinder create cluster` succeeds and the binary coexists with any installed `kind` binary
  2. An existing kind `v1alpha4` cluster config file (without an `addons:` section) works unchanged with kinder
  3. A cluster config with `addons.metalLB: false` parses without error and the opt-out flag is visible to action code
  4. On macOS or Windows, kinder prints a warning that MetalLB LoadBalancer IPs may not be reachable from the host
**Plans**: 2 plans
Plans:
- [x] 01-01-PLAN.md — Config schema extension with Addons struct, binary rename to kinder, config parsing tests
- [x] 01-02-PLAN.md — Action pipeline scaffolding with stub addon actions, warn-continue loop, platform warning, addon summary

### Phase 2: MetalLB
**Goal**: Services of type LoadBalancer receive an EXTERNAL-IP automatically on every supported container provider
**Depends on**: Phase 1
**Requirements**: MLB-01, MLB-02, MLB-03, MLB-04, MLB-05, MLB-06, MLB-07, MLB-08
**Success Criteria** (what must be TRUE):
  1. MetalLB controller and speaker pods reach Running state during `kinder create cluster` before the command returns
  2. A service of type LoadBalancer gets an EXTERNAL-IP within seconds of creation on a Docker-backed cluster
  3. Subnet detection runs without user input and produces a valid IP pool carved from the active Docker/Podman/Nerdctl network
  4. Setting `addons.metalLB: false` in cluster config causes no MetalLB pods to be installed
**Plans**: 2 plans
Plans:
- [x] 02-01-PLAN.md — TDD: Subnet detection and IP pool carving with unit tests for Docker/Podman/Nerdctl JSON schemas
- [x] 02-02-PLAN.md — MetalLB action implementation with embedded manifest, webhook wait, and CR application

### Phase 3: Metrics Server
**Goal**: `kubectl top nodes` and `kubectl top pods` return data immediately after cluster creation and HPA can read the metrics API
**Depends on**: Phase 1
**Requirements**: MET-01, MET-02, MET-03, MET-04, MET-05
**Success Criteria** (what must be TRUE):
  1. `kubectl top nodes` returns CPU and memory data within 60 seconds of `kinder create cluster` completing
  2. `kubectl top pods` returns data for pods in any namespace
  3. An HPA targeting CPU utilization successfully reads metrics from the Metrics API without errors
  4. Setting `addons.metricsServer: false` in cluster config causes no Metrics Server pods to be installed
**Plans**: 1 plan
Plans:
- [x] 03-01-PLAN.md — Metrics Server v0.8.1 manifest embedding with --kubelet-insecure-tls and action Execute implementation

### Phase 4: CoreDNS Tuning
**Goal**: CoreDNS ConfigMap is patched in-place with improved cache settings and existing in-cluster DNS resolution continues to work
**Depends on**: Phase 1
**Requirements**: DNS-01, DNS-02, DNS-03, DNS-04, DNS-05
**Success Criteria** (what must be TRUE):
  1. CoreDNS pods restart and reach Running state after kinder applies the ConfigMap patch
  2. In-cluster DNS resolution works correctly after patching — a pod can resolve `kubernetes.default.svc.cluster.local`
  3. The CoreDNS Corefile contains the updated cache TTL for external queries after cluster creation
  4. Setting `addons.coreDNSTuning: false` in cluster config leaves the CoreDNS ConfigMap at its kind default
**Plans**: 1 plan
Plans:
- [x] 04-01-PLAN.md — CoreDNS Corefile read-modify-write with autopath, pods verified, cache 60, and rollout restart

### Phase 5: Envoy Gateway
**Goal**: Gateway API CRDs are established and Envoy Gateway controller is running so a user can deploy a Gateway and route HTTP traffic via a LoadBalancer IP
**Depends on**: Phase 2 (MetalLB must assign IPs), Phase 1
**Requirements**: EGW-01, EGW-02, EGW-03, EGW-04, EGW-05, EGW-06
**Success Criteria** (what must be TRUE):
  1. Envoy Gateway controller pod reaches Running state during `kinder create cluster` and a `GatewayClass` named `eg` exists
  2. A user can deploy a Gateway and HTTPRoute and curl a backend service through the resulting LoadBalancer IP
  3. When MetalLB is disabled and Envoy Gateway is enabled, kinder prints a clear warning that the Gateway proxy will not get an IP
  4. Setting `addons.envoyGateway: false` in cluster config causes no Gateway API CRDs or Envoy Gateway pods to be installed
**Plans**: 1 plan
Plans:
- [x] 05-01-PLAN.md — Envoy Gateway v1.3.1 install.yaml embedding with server-side apply, certgen/controller/GatewayClass wait sequence, TLS termination docs

### Phase 6: Dashboard
**Goal**: Headlamp is installed and a developer can immediately open the Kubernetes dashboard using a printed token and port-forward command
**Depends on**: Phase 1
**Requirements**: DASH-01, DASH-02, DASH-03, DASH-04, DASH-05, DASH-06
**Success Criteria** (what must be TRUE):
  1. `kinder create cluster` output includes a long-lived service account token and the exact `kubectl port-forward` command to access the dashboard
  2. Following the printed port-forward command, the Headlamp UI loads in a browser and accepts the printed token
  3. In the Headlamp UI, a user can view pods, services, deployments, and logs across namespaces
  4. Setting `addons.dashboard: false` in cluster config causes no Headlamp pods or RBAC resources to be installed
**Plans**: 1 plan
Plans:
- [x] 06-01-PLAN.md — Headlamp v0.40.1 manifest embedding with RBAC, long-lived token Secret, token decode and print

### Phase 7: Integration Testing
**Goal**: All five addons work correctly together in a single `kinder create cluster` run and each addon's functional health is verified — not just pod readiness
**Depends on**: Phases 2, 3, 4, 5, 6
**Requirements**: *(cross-phase validation — all 34 v1 requirements exercised)*
**Success Criteria** (what must be TRUE):
  1. A full `kinder create cluster` run with all addons enabled completes without errors and all addon pods are Running
  2. The MetalLB-to-Envoy-Gateway end-to-end path works: a Gateway service gets an EXTERNAL-IP and an HTTPRoute routes traffic to a backend
  3. `kubectl top nodes` returns data and an HPA object shows current CPU metrics after cluster creation
  4. CoreDNS resolves external hostnames from within a pod and in-cluster service names resolve correctly
  5. The Headlamp dashboard is accessible using the printed token and port-forward command
**Plans**: 2 plans
Plans:
- [x] 07-01-PLAN.md — TDD: Extract CoreDNS patchCorefile function and write table-driven unit tests for all transforms and guard checks
- [x] 07-02-PLAN.md — Unit tests for create.go addon summary/platform warning + live-cluster integration verification script

### Phase 8: Gap Closure
**Goal**: Fix the all-addons-disabled config edge case so explicit opt-out is respected, and harden the integration test script with proper kubectl context targeting
**Depends on**: Phases 1, 7
**Requirements**: FOUND-04 (edge case fix)
**Gap Closure:** Closes integration issues 1-2 and flow gaps 3-4 from v1.0 audit
**Success Criteria** (what must be TRUE):
  1. A cluster config with all five addons set to `false` results in zero addon pods being installed
  2. Unit tests verify the all-addons-disabled config path produces an internal config with all bools false
  3. `hack/verify-integration.sh` explicitly sets kubectl context to `kind-kinder-integration-test` before running checks
  4. Re-running `/gsd:audit-milestone` produces zero integration issues and zero partial flows
**Plans**: 0 plans
Plans:
- *(to be planned)*

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3 -> 4 -> 5 -> 6 -> 7 -> 8

Note: Phases 2, 3, and 4 each depend only on Phase 1 and are independent of each other. Phases 5 and 6 depend on Phase 1 (Phase 5 also hard-depends on Phase 2). In practice, execute in the order listed — MetalLB first ensures Phase 5 can be verified end-to-end.

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 2/2 | Complete | 2026-03-01 |
| 2. MetalLB | 2/2 | Complete | 2026-03-01 |
| 3. Metrics Server | 1/1 | Complete | 2026-03-01 |
| 4. CoreDNS Tuning | 1/1 | Complete | 2026-03-01 |
| 5. Envoy Gateway | 1/1 | Complete | 2026-03-01 |
| 6. Dashboard | 1/1 | Complete | 2026-03-01 |
| 7. Integration Testing | 2/2 | Complete | 2026-03-01 |
| 8. Gap Closure | 0/0 | Pending | — |
