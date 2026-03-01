# Kinder

## What This Is

Kinder is a fork of kind (Kubernetes IN Docker) that provides a batteries-included local Kubernetes development experience. Where kind gives you a bare cluster, kinder comes with LoadBalancer support, Gateway API ingress, metrics, tuned DNS, and a dashboard — all working out of the box. Users run `kinder create cluster` and get a fully functional development environment.

## Core Value

A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## Current Milestone: v1.0 Batteries Included

**Goal:** Fork kind into kinder with 5 default addons that can be individually opted out.

**Target features:**
- MetalLB for LoadBalancer services
- Envoy Gateway for Gateway API ingress
- Metrics Server for kubectl top / HPA
- CoreDNS tuning for better caching
- Kubernetes Dashboard pre-installed

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

- ✓ All existing kind functionality — inherited from upstream fork (cluster create/delete/get/load/build/export)

### Active

<!-- Current scope. Building toward these. -->

- [ ] Binary renamed to `kinder`, coexists with `kind`
- [ ] MetalLB installed by default on cluster creation
- [ ] Envoy Gateway installed by default on cluster creation
- [ ] Metrics Server installed by default on cluster creation
- [ ] CoreDNS tuning applied by default on cluster creation
- [ ] Kubernetes Dashboard installed by default on cluster creation
- [ ] Each addon can be individually disabled via cluster config
- [ ] Addons wait for readiness before cluster is reported as ready

### Out of Scope

- OAuth/OIDC integration for dashboard — too complex for v1, basic token access is fine
- Multi-cluster networking — single cluster focus
- Helm as addon manager — use static manifests/kustomize to avoid Helm dependency
- Custom addon plugin system — hardcoded addons for v1, extensibility later

## Context

- Kinder is a fork of sigs.k8s.io/kind at commit 89ff06bd
- kind already has an action pipeline (`pkg/cluster/internal/create/actions/`) that runs steps sequentially during cluster creation — addons fit naturally as new actions
- kind already installs kindnet (CNI) and local-path-provisioner (storage) as actions — same pattern for MetalLB, Envoy Gateway, etc.
- MetalLB needs to know the Docker network subnet to allocate LoadBalancer IPs
- Envoy Gateway requires Gateway API CRDs installed before the controller
- The `kinder` name is already used in the Kubernetes testing ecosystem (k8s.io/kubeadm/kinder) but that project is largely dormant

## Constraints

- **Tech stack**: Go, same build system as kind — no new languages or build tools
- **Compatibility**: Must work with Docker, Podman, and Nerdctl (all existing providers)
- **Config format**: Extend kind's `v1alpha4` config API with addon fields, don't break existing configs
- **Image size**: Addon manifests bundled in node image or applied at runtime — prefer runtime to keep image size small

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Fork kind, don't wrap it | Full control over action pipeline, single binary, no subprocess overhead | — Pending |
| Addons as creation actions | Follows existing kind pattern (installcni, installstorage), consistent architecture | — Pending |
| On by default, opt-out | Target audience wants batteries included; power users can disable | — Pending |
| Runtime manifest apply | Apply addon manifests via kubectl during creation rather than baking into node image | — Pending |

---
*Last updated: 2026-03-01 after project initialization*
