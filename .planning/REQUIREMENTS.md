# Requirements: Kinder

**Defined:** 2026-03-03
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## v1.3 Requirements

Requirements for v1.3 Harden & Extend milestone. Each maps to roadmap phases.

### Bug Fixes

- [x] **BUG-01**: Fix defer-in-loop port leak in generatePortMappings across all 3 providers
- [x] **BUG-02**: Fix tar extraction silent data loss on truncated files (return error instead of break)
- [x] **BUG-03**: Fix ListInternalNodes missing defaultName() call for consistent cluster name resolution
- [x] **BUG-04**: Fix network sort comparator to use strict weak ordering

### Provider Refactor

- [x] **PROV-01**: Extract shared node.go to common/ package with binaryName parameter
- [x] **PROV-02**: Extract shared provision.go functions (generateMountBindings, generatePortMappings, createContainer) to common/
- [x] **PROV-03**: Update go.mod minimum to go 1.21.0 with toolchain go1.26.0

### Config

- [x] **CFG-01**: Add LocalRegistry *bool to v1alpha4 Addons struct with default true
- [x] **CFG-02**: Add CertManager *bool to v1alpha4 Addons struct with default true
- [x] **CFG-03**: Wire both fields through internal config types, conversion, and defaults (5 locations)

### Local Registry

- [ ] **REG-01**: Create registry:2 container on kind network during cluster creation
- [ ] **REG-02**: Patch containerd certs.d config on all nodes for localhost:5001
- [ ] **REG-03**: Apply kube-public/local-registry-hosting ConfigMap for dev tool discovery
- [ ] **REG-04**: Addon disableable via addons.localRegistry: false in cluster config

### cert-manager

- [ ] **CERT-01**: Embed and apply cert-manager v1.17.6 manifest via go:embed
- [ ] **CERT-02**: Wait for all 3 components (controller, webhook, cainjector) to reach Available status
- [ ] **CERT-03**: Bootstrap self-signed ClusterIssuer so Certificate resources work immediately
- [ ] **CERT-04**: Addon enabled by default, disableable via addons.certManager: false

### CLI Commands

- [ ] **CLI-01**: kinder env command shows provider, cluster name, and kubeconfig path
- [ ] **CLI-02**: kinder env output is machine-readable (eval-safe stdout, warnings to stderr)
- [ ] **CLI-03**: kinder doctor checks binary prerequisites with actionable fix messages
- [ ] **CLI-04**: kinder doctor uses structured exit codes (0=ok, 1=fail, 2=warn)

## v1.4+ Requirements

Deferred to future release. Tracked but not in current roadmap.

### Registry Enhancements

- **REG-05**: Pull-through cache (Docker Hub mirror) for local registry
- **REG-06**: Registry web UI for browsing images

### cert-manager Enhancements

- **CERT-05**: trust-manager addon for distributing CA bundles
- **CERT-06**: ACME/Let's Encrypt issuer support

### CLI Enhancements

- **CLI-05**: kinder env --shell flag for fish shell compatibility
- **CLI-06**: kinder env shows enabled/disabled addon state
- **CLI-07**: kinder doctor checks resource minimums (4GB RAM, 10GB disk)
- **CLI-08**: kinder doctor --fix auto-remediation

## Out of Scope

| Feature | Reason |
|---------|--------|
| Helm-based addon installation | Project constraint: static manifests + go:embed only |
| k8s.io/client-go SDK | Architecture: all kubectl ops via node.Command inside containers |
| Harbor registry | Too heavy for local dev; registry:2 is sufficient |
| registry:3 image | v3 deprecated storage drivers; kind ecosystem on v2 |
| ACME issuers | Requires internet; incompatible with offline local clusters |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| BUG-01 | Phase 19 | Complete |
| BUG-02 | Phase 19 | Complete |
| BUG-03 | Phase 19 | Complete |
| BUG-04 | Phase 19 | Complete |
| PROV-01 | Phase 20 | Complete |
| PROV-02 | Phase 20 | Complete |
| PROV-03 | Phase 20 | Complete |
| CFG-01 | Phase 21 | Complete |
| CFG-02 | Phase 21 | Complete |
| CFG-03 | Phase 21 | Complete |
| REG-01 | Phase 22 | Pending |
| REG-02 | Phase 22 | Pending |
| REG-03 | Phase 22 | Pending |
| REG-04 | Phase 22 | Pending |
| CERT-01 | Phase 23 | Pending |
| CERT-02 | Phase 23 | Pending |
| CERT-03 | Phase 23 | Pending |
| CERT-04 | Phase 23 | Pending |
| CLI-01 | Phase 24 | Pending |
| CLI-02 | Phase 24 | Pending |
| CLI-03 | Phase 24 | Pending |
| CLI-04 | Phase 24 | Pending |

**Coverage:**
- v1.3 requirements: 22 total
- Mapped to phases: 22
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-03*
*Last updated: 2026-03-03 — Phase 21 Plan 01 complete: CFG-01, CFG-02, CFG-03 done; Phase 21 fully complete*
