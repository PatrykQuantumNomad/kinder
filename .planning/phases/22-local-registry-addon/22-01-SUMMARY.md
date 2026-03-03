---
phase: 22-local-registry-addon
plan: "01"
subsystem: addon/localregistry
tags: [registry, containerd, certs.d, addon, kep-1755]
dependency_graph:
  requires: [phase-21-config-type-additions]
  provides: [installlocalregistry-action, local-registry-configmap, containerd-certs-d-wiring]
  affects: [create.go, containerd-config-patches]
tech_stack:
  added: []
  patterns: [go-embed, fmt-stringer-provider-detection, node-command-tee, idempotent-container-ops]
key_files:
  created:
    - pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go
    - pkg/cluster/internal/create/actions/installlocalregistry/manifests/local-registry-hosting.yaml
  modified:
    - pkg/cluster/internal/create/create.go
key_decisions:
  - "Idempotent registry creation via inspect-before-run (registry persists across cluster recreations)"
  - "Idempotent network connect via inspect-networks-before-connect (avoids fragile error-message parsing)"
  - "ContainerdConfigPatches injected in create.go before p.Provision() — cannot be done post-provision"
  - "ALL nodes patched with hosts.toml (not just control-plane) — each node has independent containerd"
  - "Podman rootless: warn-and-continue, not hard failure (TLS enforcement requires manual host config)"
  - "registry:2 not registry:3 (kind ecosystem has not migrated; v3 deprecated storage drivers)"
metrics:
  duration: "~2 min"
  completed: "2026-03-03"
  tasks_completed: 2
  files_created: 2
  files_modified: 1
---

# Phase 22 Plan 01: Local Registry Addon Summary

Batteries-included local registry via registry:2 container on the kind network with per-node containerd certs.d/hosts.toml injection and KEP-1755 ConfigMap, wired into create.go as a disableable runAddon call with pre-provisioning ContainerdConfigPatches injection.

## Accomplishments

- Created `pkg/cluster/internal/create/actions/installlocalregistry/` package with full Execute() implementing REG-01 through REG-03
- Registry container creation is idempotent (inspect-before-run; registry persists across cluster recreations so cached images survive)
- Network connect is idempotent (inspect networks before connecting; avoids fragile error-message parsing across docker/podman/nerdctl versions)
- All cluster nodes (control-plane and workers) receive `/etc/containerd/certs.d/localhost:5001/hosts.toml` pointing at `http://kind-registry:5000`
- `kube-public/local-registry-hosting` ConfigMap applied via control-plane kubectl for KEP-1755 dev tool discovery (Tilt, Skaffold, ctlptl)
- Podman rootless detected via `ctx.Provider.Info().Rootless` — emits actionable warning, does not fail
- `create.go` injects `config_path = "/etc/containerd/certs.d"` into ContainerdConfigPatches before `p.Provision()` (critical: cannot be done post-provisioning)
- `runAddon("Local Registry", ...)` placed before MetalLB for correct dependency ordering
- Addon disableable via `addons.localRegistry: false` in cluster config (REG-04)
- `go build ./...` and `go test ./...` pass with zero errors and no regressions

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create installlocalregistry action package | 8ee3cf63 | localregistry.go, manifests/local-registry-hosting.yaml |
| 2 | Wire local registry into create.go | 869b20c3 | create.go |

## Files Created

- `pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go` — Action implementation with NewAction()/Execute()
- `pkg/cluster/internal/create/actions/installlocalregistry/manifests/local-registry-hosting.yaml` — Embedded KEP-1755 ConfigMap

## Files Modified

- `pkg/cluster/internal/create/create.go` — Added import, ContainerdConfigPatches injection before Provision, runAddon call before MetalLB

## Decisions Made

1. **Idempotent container creation:** Check `docker inspect --format={{.ID}} kind-registry` before `docker run`. Registry survives cluster deletion by design (consistent with kind upstream) — cached images persist across cluster recreations.

2. **Idempotent network connect:** Inspect `{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}` before `network connect`. Avoids parsing fragile error messages that differ across docker/podman/nerdctl versions.

3. **ContainerdConfigPatches before Provision:** containerd's `certs.d` hot-reload requires `config_path` in config.toml at node creation time. The addon action runs post-provisioning and cannot modify containerd config. Injecting in `create.go` before `p.Provision()` is the correct location.

4. **ALL nodes patched:** Each kind node is an independent container with its own containerd process. `ctx.Nodes()` returns all nodes; every one receives `mkdir + tee hosts.toml`. Only patching control-plane would cause worker-node image pull failures.

5. **Podman rootless warn-and-continue:** kind issue #3468 documents TLS enforcement failures on rootless Podman for host-side push. Node-side pulls may work. Emitting an actionable warning with the `registries.conf` fix is the correct approach — do not hard-fail.

6. **registry:2 not registry:3:** kind ecosystem has not migrated to registry:3 (released April 2025, deprecated storage drivers). Using registry:2 is consistent with all kind documentation and tooling.

## Deviations from Plan

None — plan executed exactly as written.

## Requirements Coverage

| Requirement | Status | Implementation |
|-------------|--------|----------------|
| REG-01 | Done | registry:2 container created on kind network (idempotent) |
| REG-02 | Done | hosts.toml written to ALL nodes via node.Command("tee") |
| REG-03 | Done | kube-public/local-registry-hosting ConfigMap applied via kubectl |
| REG-04 | Done | Disabled via addons.localRegistry: false, injected ContainerdConfigPatches conditional |

## Self-Check: PASSED

Files confirmed present:
- pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go: FOUND
- pkg/cluster/internal/create/actions/installlocalregistry/manifests/local-registry-hosting.yaml: FOUND

Commits confirmed:
- 8ee3cf63: feat(22-01): implement installlocalregistry action package
- 869b20c3: feat(22-01): wire local registry into create.go

Build/test: go build ./... PASSED, go test ./... PASSED (all cached + create package 0.361s)
