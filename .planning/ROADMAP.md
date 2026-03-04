# Roadmap: Kinder

## Milestones

- ✅ **v1.0 Batteries Included** - Phases 1-8 (shipped 2026-03-01)
- ✅ **v1.1 Kinder Website** - Phases 9-14 (shipped 2026-03-02)
- ✅ **v1.2 Branding & Polish** - Phases 15-18 (shipped 2026-03-02)
- ✅ **v1.3 Harden & Extend** - Phases 19-24 (shipped 2026-03-03)
- ✅ **v1.4 Code Quality & Features** - Phases 25-29 (shipped 2026-03-04)
- ✅ **v1.5 Website Use Cases & Documentation** - Phases 30-34 (shipped 2026-03-04)

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

<details>
<summary>✅ v1.4 Code Quality & Features (Phases 25-29) - SHIPPED 2026-03-04</summary>

See `.planning/milestones/v1.4-ROADMAP.md` for full phase details.

Phases 25-29: Foundation (Go 1.24, golangci-lint v2, layer fix), Architecture (context.Context, addon registry), Unit Tests (FakeNode/FakeCmd test infra), Parallel Execution (wave-based errgroup), CLI Features (JSON output, profile presets).

</details>

### 🚧 v1.5 Website Use Cases & Documentation (In Progress)

**Milestone Goal:** Update the kinder website with detailed use cases, tutorials, and CLI reference pages so users can see how to actually use every feature.

#### Phase 30: Foundation Fixes

**Goal**: Existing site pages accurately reflect all v1.3-v1.4 features and new page structure is in place
**Depends on**: Nothing (first phase of milestone)
**Requirements**: FOUND-01, FOUND-02, FOUND-03, FOUND-04
**Success Criteria** (what must be TRUE):
  1. Landing page Comparison component lists all 7 addons (MetalLB, Envoy Gateway, Metrics Server, CoreDNS, Headlamp, Local Registry, cert-manager)
  2. Quick-start page includes verification steps for all 7 addons and mentions the --profile flag
  3. Configuration page documents all 7 addon fields including localRegistry and certManager with correct v1alpha4 schema
  4. Sidebar navigation shows Guides and CLI Reference sections with placeholder entries for all new pages
**Plans**: 1 plan

Plans:
- [x] 30-01: Update landing page, quick-start, configuration, and sidebar for v1.3-v1.4 features

#### Phase 31: Addon Page Depth

**Goal**: Every addon page has practical examples and troubleshooting content that helps users succeed with that addon
**Depends on**: Phase 30
**Requirements**: ADDON-01, ADDON-02, ADDON-03, ADDON-04, ADDON-05, ADDON-06, ADDON-07
**Success Criteria** (what must be TRUE):
  1. MetalLB page shows how to create a LoadBalancer service with a custom IP and explains when to use NodePort instead
  2. Envoy Gateway page shows working HTTPRoute examples for path-based and header-based routing
  3. Metrics Server page shows `kubectl top` output examples and a basic HPA manifest with scale trigger
  4. CoreDNS, Headlamp, Local Registry, and cert-manager pages each have concrete verification commands and at least one troubleshooting entry
  5. All 7 addon pages include a troubleshooting section with the most common failure and its fix
**Plans**: 2 plans

Plans:
- [ ] 31-01: Enrich MetalLB, Envoy Gateway, Metrics Server, and CoreDNS addon pages
- [ ] 31-02: Enrich Headlamp, Local Registry, and cert-manager addon pages

#### Phase 32: CLI Reference

**Goal**: Users can look up any CLI flag or command behavior with concrete examples and know what to do when things go wrong
**Depends on**: Phase 30
**Requirements**: CLI-01, CLI-02, CLI-03
**Success Criteria** (what must be TRUE):
  1. Profile comparison page shows a table of all 4 presets (minimal, full, gateway, ci) with which addons each enables and a recommended use case
  2. JSON output reference page shows `--output json` examples for all 4 read commands and at least 3 jq filter recipes
  3. Troubleshooting guide for `kinder env` and `kinder doctor` lists all exit codes with their meanings and resolution steps
**Plans**: 1 plan

Plans:
- [x] 32-01: Create profile comparison, JSON output reference, and env/doctor troubleshooting pages

#### Phase 33: Tutorials

**Goal**: Users can follow end-to-end tutorials that demonstrate real workflows combining multiple kinder features
**Depends on**: Phase 31, Phase 32
**Requirements**: GUIDE-01, GUIDE-02, GUIDE-03
**Success Criteria** (what must be TRUE):
  1. TLS tutorial guides a user from cluster creation through pushing an image to the local registry, exposing it via Envoy Gateway with a cert-manager TLS certificate, and verifying HTTPS works
  2. HPA tutorial guides a user through deploying a workload, configuring an HPA, generating load with a tool like `hey` or `kubectl run`, and watching pods scale up in real time
  3. Local dev workflow tutorial shows the full code-build-push-deploy-iterate loop using localhost:5001 with a concrete example application
  4. Each tutorial has clearly marked prerequisites, copy-pasteable commands, and expected output so a user can follow without guessing
**Plans**: 2 plans

Plans:
- [x] 33-01: Write TLS web app tutorial (Local Registry + Envoy Gateway + cert-manager + MetalLB)
- [x] 33-02: Write HPA auto-scaling tutorial and local dev workflow tutorial

#### Phase 34: Verification & Polish

**Goal**: The site is consistent, all links resolve, and the production build is clean with no errors
**Depends on**: Phase 33
**Requirements**: (audit phase — covers all v1.5 requirements)
**Success Criteria** (what must be TRUE):
  1. All internal links across every new and updated page resolve without 404s
  2. Production build (`npm run build`) completes with zero errors and zero broken link warnings
  3. All new pages are reachable from the sidebar navigation without dead ends
  4. Code blocks in tutorials are syntactically correct and commands match what the CLI actually accepts
**Plans**: 1 plan

Plans:
- [x] 34-01: Full site audit, link check, and production build verification

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8. v1.0 phases | v1.0 | 12/12 | Complete | 2026-03-01 |
| 9-14. v1.1 phases | v1.1 | 8/8 | Complete | 2026-03-02 |
| 15-18. v1.2 phases | v1.2 | 4/4 | Complete | 2026-03-02 |
| 19-24. v1.3 phases | v1.3 | 8/8 | Complete | 2026-03-03 |
| 25-29. v1.4 phases | v1.4 | 13/13 | Complete | 2026-03-04 |
| 30. Foundation Fixes | v1.5 | 1/1 | Complete | 2026-03-04 |
| 31. Addon Page Depth | v1.5 | 2/2 | Complete | 2026-03-04 |
| 32. CLI Reference | v1.5 | 1/1 | Complete | 2026-03-04 |
| 33. Tutorials | v1.5 | 2/2 | Complete | 2026-03-04 |
| 34. Verification & Polish | v1.5 | 1/1 | Complete | 2026-03-04 |
