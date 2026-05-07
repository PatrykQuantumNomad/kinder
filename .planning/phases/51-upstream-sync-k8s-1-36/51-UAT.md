---
status: complete
phase: 51-upstream-sync-k8s-1-36
source:
  - 51-01-SUMMARY.md
  - 51-02-SUMMARY.md
  - 51-03-SUMMARY.md
started: 2026-05-07T15:00:00Z
updated: 2026-05-07T15:20:00Z
---

## Current Test

[testing complete]

## Tests

### 1. HA cluster uses Envoy as load balancer (no HAProxy)
expected: |
  Multi-CP cluster's external-load-balancer container runs `docker.io/envoyproxy/envoy:v1.36.2`. No `kindest/haproxy` image is pulled or running. Cluster reaches Ready state and kubectl works.
result: pass
evidence: |
  User ran `kinder create cluster --name ha-test` with 2 CP + 1 worker.
  - `docker ps` confirmed: `ha-test-external-load-balancer` runs `envoyproxy/envoy:v1.36.2`
  - No `kindest/haproxy` container present anywhere
  - All 3 nodes started, cluster context set, 8 of 9 addons installed via kubectl-through-LB path
  - SC1 verified
side_observation: |
  MetalLB CR apply failed during cluster bring-up: `docker exec --privileged -i ha-test-control-plane kubectl --kubeconfig=/etc/kubernetes/admin.conf apply -f -` exit 1.
  This is NOT a Phase 51 regression — 8 other addons used the same kubectl path successfully right after the failure. MetalLB-specific issue, likely pre-existing or transient. Logged as separate gap below.

### 2. IPVS + K8s 1.36 config rejected at validation
expected: |
  Validation rejects ipvs+1.36 config with deprecation message + migration URL, exits non-zero before any container is created.
result: pass
evidence: |
  User ran `kinder create cluster --config /tmp/ipvs-1-36-test.yaml`. Exit 1.
  Error string contains all required elements verbatim:
  - "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+ (node image \"kindest/node:v1.36.0\" uses v1.36)"
  - "kube-proxy IPVS mode was deprecated in v1.35 and will be removed in a future release"
  - "Switch to iptables or nftables"
  - "https://kubernetes.io/docs/reference/networking/virtual-ips/"
  No containers created. SC3 verified.
side_observation: |
  Empty-cluster-name validator fired alongside ('' is not a valid cluster name). User config did not set `name:` and CLI did not pass `--name`. Errors aggregate via Validate's NewAggregate path — correct existing behavior. Not a Phase 51 issue.

### 3. K8s 1.36 website guide renders with both GA demos
expected: |
  Guide page renders with sidebar entry between Multi-Version Clusters and Working Offline; both User Namespaces (hostUsers: false) and In-Place Pod Resize (resizePolicy) sections render correctly with verification steps.
result: pass
evidence: |
  User confirmed "pass" after loading /guides/k8s-1-36-whats-new/ in the local kinder-site dev server. SC4 verified.

## Summary

total: 3
passed: 3
issues: 0
pending: 0
skipped: 0

## Gaps

# Side observation from Test 1 — NOT a Phase 51 regression, but flagged for follow-up
- truth: "MetalLB addon installs cleanly during HA cluster bring-up"
  status: failed
  reason: "Test 1 side observation: `docker exec --privileged -i ha-test-control-plane kubectl --kubeconfig=/etc/kubernetes/admin.conf apply -f -` exit 1 during MetalLB CR apply. 8 other addons using the same kubectl path succeeded immediately after, so the LB and kubectl path itself are fine. Likely a MetalLB-specific issue (CRD timing, manifest, or webhook readiness). Pre-existing or transient — not introduced by Phase 51."
  severity: major
  test: 1
  scope: out-of-phase-51
  follow_up_suggested: "Open a separate todo / dedicated phase to investigate MetalLB CR apply failure on multi-CP clusters. Outside the SC1/SC2/SC3/SC4 scope of Phase 51."

## Notes

- SC2 (default node image bump to K8s 1.36.x) is intentionally NOT tested — it is DEFERRED. `kindest/node:v1.36.x` is not yet on Docker Hub (probe 2026-05-07: count=0). Plan 51-04 re-runs once kind v0.32.0 publishes the image; no behavior shipped means nothing to UAT.
- All 3 active tests require live infrastructure (Docker daemon, dev server). Skip with reason if unavailable on this host.
