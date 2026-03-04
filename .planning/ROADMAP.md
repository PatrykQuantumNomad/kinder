# Roadmap: Kinder

## Milestones

- ✅ **v1.0 Batteries Included** - Phases 1-8 (shipped 2026-03-01)
- ✅ **v1.1 Kinder Website** - Phases 9-14 (shipped 2026-03-02)
- ✅ **v1.2 Branding & Polish** - Phases 15-18 (shipped 2026-03-02)
- ✅ **v1.3 Harden & Extend** - Phases 19-24 (shipped 2026-03-03)
- 🚧 **v1.4 Code Quality & Features** - Phases 25-29 (in progress)

## Phases

<details>
<summary>✅ v1.0 Batteries Included (Phases 1-8) - SHIPPED 2026-03-01</summary>

See `.planning/milestones/v1.0-ROADMAP.md` for full phase details.

Phases 1-8: Foundation, MetalLB, Metrics Server, CoreDNS Tuning, Envoy Gateway, Dashboard, Integration Testing, Gap Closure.

</details>

<details>
<summary>✅ v1.1 Kinder Website (Phases 9-14) - SHIPPED 2026-03-02</summary>

See `.planning/milestones/v1.1-ROADMAP.md` for full phase details.

Phases 9-14: Scaffold & Deploy Pipeline, Dark Theme, Documentation Content, Landing Page, Assets & Identity, Polish & Validation.

</details>

<details>
<summary>✅ v1.2 Branding & Polish (Phases 15-18) - SHIPPED 2026-03-02</summary>

Phases 15-18: Logo, SEO, Docs Rewrite, Dark Theme Enforcement.

</details>

<details>
<summary>✅ v1.3 Harden & Extend (Phases 19-24) - SHIPPED 2026-03-03</summary>

Phases 19-24: Bug Fixes, Provider Code Deduplication, Config Type Additions, Local Registry Addon, Cert-Manager Addon, CLI Diagnostic Tools.

</details>

### 🚧 v1.4 Code Quality & Features (In Progress)

**Milestone Goal:** Clean up remaining code quality issues, improve architecture, add unit tests for addon actions, and deliver new developer-facing features. Every change is independently buildable and verifiable with `go build ./... && go test ./...`.

## Phase Summary

- [x] **Phase 25: Foundation** - go.mod bump, dependency updates, golangci-lint v2, layer violation fix, code quality cleanup
- [x] **Phase 26: Architecture** - context.Context propagation, centralized addon registry, context-aware readiness waiting
- [x] **Phase 27: Unit Tests** - fakeNode/fakeCmd test infrastructure, unit tests for all addon action packages
- [ ] **Phase 28: Parallel Execution** - dependency DAG documentation, sync.Once fix, wave-based parallel addon install, timing summary
- [ ] **Phase 29: CLI Features** - JSON output for all commands, cluster presets via --profile flag

## Phase Details

### Phase 25: Foundation
**Goal**: The codebase has a clean dependency baseline — go.mod is on 1.23, deps are current, golangci-lint v2 passes, the layer violation is fixed, and dead code from the version bump is removed
**Depends on**: Nothing (first phase of v1.4)
**Requirements**: FOUND-01, FOUND-02, FOUND-03, FOUND-04, FOUND-05, FOUND-06, FOUND-07, FOUND-08, FOUND-09, FOUND-10
**Success Criteria** (what must be TRUE):
  1. `go build ./...` succeeds with `go 1.23` directive in go.mod and no reference to rand.NewSource dead code
  2. `golangci-lint run ./...` completes with zero errors using the v2.10.1 config
  3. `pkg/cluster/provider.go` imports nothing from `pkg/cmd/kind/version/`; version constants live in `pkg/internal/kindversion/`
  4. `kinder version` prints the correct version string (linker -X flags updated for new package path)
  5. SHA-256 is used for subnet generation in all three providers; log directory permissions are 0755; dashboard token logs at V(1)
**Plans:** 4 plans
Plans:
- [x] 25-01-PLAN.md — Go ecosystem updates (go.mod 1.24.0, x/sys v0.41.0, dead rand code)
- [x] 25-02-PLAN.md — golangci-lint v1 to v2 migration
- [x] 25-03-PLAN.md — Layer violation fix (version package move to internal)
- [x] 25-04-PLAN.md — Code quality fixes (SHA-256, permissions, log level, naming, test helper)

### Phase 26: Architecture
**Goal**: context.Context flows from create.go through ActionContext into every addon Execute() call and waitforready loop; a centralized AddonEntry registry replaces hard-coded runAddon calls in create.go
**Depends on**: Phase 25
**Requirements**: ARCH-01, ARCH-02, ARCH-03, ARCH-04
**Success Criteria** (what must be TRUE):
  1. ActionContext carries a Context field; all addon Execute() methods call node.CommandContext(ctx.Context, ...) instead of node.Command(...)
  2. create.go drives addon installation through a registry loop over []AddonEntry rather than 7 individual runAddon() call sites
  3. waitforready.tryUntil returns immediately when the context is cancelled rather than spinning until timeout
  4. `go build ./...` and `go vet ./...` pass with no import cycles introduced
**Plans:** 2 plans
Plans:
- [x] 26-01-PLAN.md — Context field in ActionContext + AddonEntry registry loop
- [x] 26-02-PLAN.md — CommandContext propagation in all addons + context-aware waitforready

### Phase 27: Unit Tests
**Goal**: Every addon action package has unit tests that run without a real cluster and pass under `go test ./pkg/cluster/internal/create/actions/...`
**Depends on**: Phase 26
**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04, TEST-05, TEST-06
**Success Criteria** (what must be TRUE):
  1. A testutil package provides FakeNode and FakeCmd types usable across all addon test files
  2. `go test ./pkg/cluster/internal/create/actions/...` passes with no $KUBECONFIG or live cluster required
  3. Tests for installenvoygw, installlocalregistry, installcertmanager, installmetricsserver, and installdashboard each exercise the non-trivial logic paths of their respective Execute() methods
  4. `go test -race ./pkg/cluster/internal/create/actions/...` reports no data races
**Plans:** 3 plans
Plans:
- [x] 27-01-PLAN.md — testutil package (FakeNode/FakeCmd/FakeProvider) + metricsserver and envoygw tests
- [x] 27-02-PLAN.md — certmanager and dashboard tests (loop + stdout capture patterns)
- [x] 27-03-PLAN.md — localregistry tests (host-side exec.Command limitation handling)

### Phase 28: Parallel Execution
**Goal**: Independent addons install concurrently in waves during cluster creation, reducing total creation time, with the Nodes() cache race fixed and per-addon durations printed in the summary
**Depends on**: Phase 26, Phase 27
**Requirements**: PARA-01, PARA-02, PARA-03, PARA-04
**Success Criteria** (what must be TRUE):
  1. The addon dependency DAG is documented in source (MetalLB before EnvoyGateway; all others independent); wave boundaries are visible in code comments
  2. ActionContext.Nodes() uses sync.Once so concurrent goroutines cannot trigger a TOCTOU race; `go test -race ./...` is clean
  3. `kinder create cluster` runs independent addons in parallel via errgroup with SetLimit(3); total creation time is measurably shorter than sequential
  4. The post-creation summary prints each addon's install duration (e.g., "MetalLB: 12.3s, EnvoyGateway: 8.1s")
**Plans**: TBD

### Phase 29: CLI Features
**Goal**: Every kinder read command accepts `--output json` and produces clean, jq-parseable JSON on stdout; `kinder create cluster --profile <name>` selects a named addon preset without requiring a YAML config file
**Depends on**: Phase 25 (clean baseline); Phase 28 (per-addon result data available)
**Requirements**: CLI-01, CLI-02, CLI-03, CLI-04, CLI-05
**Success Criteria** (what must be TRUE):
  1. `kinder env --output json | jq empty` exits 0; logger output is on stderr not stdout when JSON mode is active
  2. `kinder doctor --output json | jq empty` exits 0 with a structured array of check results
  3. `kinder get clusters --output json | jq empty` and `kinder get nodes --output json | jq empty` each exit 0
  4. `kinder create cluster --profile minimal` creates a cluster with only core kind functionality (no kinder addons); `--profile full` enables all addons; `--profile gateway` enables MetalLB + EnvoyGateway; `--profile ci` enables a CI-optimized subset
  5. `kinder create cluster` without `--profile` behaves identically to current default (all addons on)
**Plans**: TBD

## Progress

**Execution Order:** 25 → 26 → 27 → 28 → 29

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8. v1.0 phases | v1.0 | 12/12 | Complete | 2026-03-01 |
| 9-14. v1.1 phases | v1.1 | 8/8 | Complete | 2026-03-02 |
| 15-18. v1.2 phases | v1.2 | 4/4 | Complete | 2026-03-02 |
| 19-24. v1.3 phases | v1.3 | 8/8 | Complete | 2026-03-03 |
| 25. Foundation | v1.4 | 4/4 | Complete | 2026-03-03 |
| 26. Architecture | v1.4 | 2/2 | Complete | 2026-03-04 |
| 27. Unit Tests | v1.4 | 3/3 | Complete | 2026-03-03 |
| 28. Parallel Execution | v1.4 | 0/? | Not started | - |
| 29. CLI Features | v1.4 | 0/? | Not started | - |
