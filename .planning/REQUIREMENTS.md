# Requirements: Kinder

**Defined:** 2026-03-03
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## v1.4 Requirements

Requirements for v1.4 Code Quality & Features milestone. Each maps to roadmap phases.

### Foundation

- [x] **FOUND-01**: go.mod minimum bumped from 1.17/1.21 to 1.23 (settled at 1.24.0 due to toolchain go1.26)
- [x] **FOUND-02**: golang.org/x/sys updated from v0.6.0 to v0.41.0
- [x] **FOUND-03**: golangci-lint upgraded from v1.62.2 to v2.10.1 with migrated config
- [x] **FOUND-04**: Version package moved from pkg/cmd/kind/version to pkg/internal/kindversion (layer violation fixed)
- [x] **FOUND-05**: rand.NewSource comment-gated dead code cleaned up after 1.23 bump
- [x] **FOUND-06**: SHA-1 replaced with SHA-256 for subnet generation in all providers
- [x] **FOUND-07**: os.ModePerm (0777) replaced with 0755 for log directories
- [x] **FOUND-08**: Dashboard token log level changed from V(0) to V(1)
- [x] **FOUND-09**: Error var renamed from NoNodeProviderDetectedError to ErrNoNodeProviderDetected
- [x] **FOUND-10**: Redundant contains helper removed from subnet_test.go

### Architecture

- [x] **ARCH-01**: context.Context added to ActionContext and propagated from create.go
- [x] **ARCH-02**: All addon Execute() methods use CommandContext instead of Command
- [x] **ARCH-03**: Centralized AddonEntry registry replaces hard-coded runAddon calls in create.go
- [x] **ARCH-04**: waitforready.tryUntil is context-aware and respects cancellation

### Tests

- [ ] **TEST-01**: fakeNode and fakeCmd test infrastructure created in testutil package
- [ ] **TEST-02**: Unit tests for installenvoygw addon action
- [ ] **TEST-03**: Unit tests for installlocalregistry addon action
- [ ] **TEST-04**: Unit tests for installcertmanager addon action
- [ ] **TEST-05**: Unit tests for installmetricsserver addon action
- [ ] **TEST-06**: Unit tests for installdashboard addon action

### Parallel

- [ ] **PARA-01**: Addon dependency DAG documented
- [ ] **PARA-02**: sync.Once fix for ActionContext.Nodes() cache TOCTOU race
- [ ] **PARA-03**: Wave-based parallel addon execution via errgroup with SetLimit(3)
- [ ] **PARA-04**: Per-addon install duration printed in summary

### CLI

- [ ] **CLI-01**: `--output json` flag on kinder env
- [ ] **CLI-02**: `--output json` flag on kinder doctor
- [ ] **CLI-03**: `--output json` flag on kinder get clusters
- [ ] **CLI-04**: `--output json` flag on kinder get nodes
- [ ] **CLI-05**: `--profile` flag on kinder create cluster with minimal/full/gateway/ci presets

## Future Requirements

Deferred to future release. Tracked but not in current roadmap.

### Extensibility

- **EXT-01**: Full addon self-registration via init() with config API change (Addons struct → map)
- **EXT-02**: `--addons` CLI flag for comma-separated addon override
- **EXT-03**: Custom addon plugin system for external addons

### Output

- **OUT-01**: YAML output (`--output=yaml`) for CLI commands
- **OUT-02**: `--output json` for `kinder create cluster` with per-addon result JSON

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Full DAG-based parallel with cycle detection | 7 addons with shallow deps; wave-based is sufficient; DAG adds 200+ lines for zero benefit |
| Interactive TUI for addon selection | Incompatible with CI usage; presets + YAML cover 100% of use cases |
| Async (non-blocking) addon install | MetalLB must be ready before EnvoyGateway; async violates dependencies silently |
| Module path change (sigs.k8s.io/kind → github.com/...) | Massive breaking change; accumulates upstream sync debt; not in v1.x scope |
| testify/gomock in main module | Existing tests use zero external test deps; table-driven stdlib testing is sufficient |
| Go plugin system | Requires CGO, incompatible with static builds |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FOUND-01 | Phase 25 | Complete |
| FOUND-02 | Phase 25 | Complete |
| FOUND-03 | Phase 25 | Complete (25-02) |
| FOUND-04 | Phase 25 | Complete (25-03) |
| FOUND-05 | Phase 25 | Complete |
| FOUND-06 | Phase 25 | Complete (25-04) |
| FOUND-07 | Phase 25 | Complete (25-04) |
| FOUND-08 | Phase 25 | Complete (25-04) |
| FOUND-09 | Phase 25 | Complete (25-04) |
| FOUND-10 | Phase 25 | Complete (25-04) |
| ARCH-01 | Phase 26 | Complete (26-01) |
| ARCH-02 | Phase 26 | Complete (26-02) |
| ARCH-03 | Phase 26 | Complete (26-01) |
| ARCH-04 | Phase 26 | Complete (26-02) |
| TEST-01 | Phase 27 | Pending |
| TEST-02 | Phase 27 | Pending |
| TEST-03 | Phase 27 | Pending |
| TEST-04 | Phase 27 | Pending |
| TEST-05 | Phase 27 | Pending |
| TEST-06 | Phase 27 | Pending |
| PARA-01 | Phase 28 | Pending |
| PARA-02 | Phase 28 | Pending |
| PARA-03 | Phase 28 | Pending |
| PARA-04 | Phase 28 | Pending |
| CLI-01 | Phase 29 | Pending |
| CLI-02 | Phase 29 | Pending |
| CLI-03 | Phase 29 | Pending |
| CLI-04 | Phase 29 | Pending |
| CLI-05 | Phase 29 | Pending |

**Coverage:**
- v1.4 requirements: 29 total
- Mapped to phases: 29
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-03*
*Last updated: 2026-03-04 after plan 26-01 (ARCH-01, ARCH-03 complete)*
