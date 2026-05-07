# Roadmap: Kinder

## Milestones

- SHIPPED **v1.0 Batteries Included** - Phases 1-8 (shipped 2026-03-01)
- SHIPPED **v1.1 Kinder Website** - Phases 9-14 (shipped 2026-03-02)
- SHIPPED **v1.2 Branding & Polish** - Phases 15-18 (shipped 2026-03-02)
- SHIPPED **v1.3 Harden & Extend** - Phases 19-24 (shipped 2026-03-03)
- SHIPPED **v1.4 Code Quality & Features** - Phases 25-29 (shipped 2026-03-04)
- SHIPPED **v1.5 Website Use Cases & Documentation** - Phases 30-34 (shipped 2026-03-04)
- SHIPPED **v2.0 Distribution & GPU Support** - Phases 35-37 (shipped 2026-03-05)
- SHIPPED **v2.1 Known Issues & Proactive Diagnostics** - Phases 38-41 (shipped 2026-03-06)
- SHIPPED **v2.2 Cluster Capabilities** - Phases 42-46 (shipped 2026-04-10)
- SHIPPED **v2.3 Inner Loop** - Phases 47-51 (shipped 2026-05-07)

## Phases

<details>
<summary>SHIPPED v1.0 Batteries Included (Phases 1-8) - SHIPPED 2026-03-01</summary>

See `.planning/milestones/v1.0-ROADMAP.md` for full phase details.

Phases 1-8: Foundation, MetalLB, Metrics Server, CoreDNS Tuning, Envoy Gateway, Dashboard, Integration Testing, Gap Closure.

</details>

<details>
<summary>SHIPPED v1.1 Kinder Website (Phases 9-14) - SHIPPED 2026-03-02</summary>

See `.planning/milestones/v1.1-ROADMAP.md` for full phase details.

Phases 9-14: Scaffold & Deploy Pipeline, Dark Theme, Documentation Content, Landing Page, Assets & Identity, Polish & Validation.

</details>

<details>
<summary>SHIPPED v1.2 Branding & Polish (Phases 15-18) - SHIPPED 2026-03-02</summary>

Phases 15-18: Logo, SEO, Docs Rewrite, Dark Theme Enforcement.

</details>

<details>
<summary>SHIPPED v1.3 Harden & Extend (Phases 19-24) - SHIPPED 2026-03-03</summary>

Phases 19-24: Bug Fixes, Provider Code Deduplication, Config Type Additions, Local Registry Addon, Cert-Manager Addon, CLI Diagnostic Tools.

</details>

<details>
<summary>SHIPPED v1.4 Code Quality & Features (Phases 25-29) - SHIPPED 2026-03-04</summary>

See `.planning/milestones/v1.4-ROADMAP.md` for full phase details.

Phases 25-29: Foundation (Go 1.24, golangci-lint v2, layer fix), Architecture (context.Context, addon registry), Unit Tests (FakeNode/FakeCmd test infra), Parallel Execution (wave-based errgroup), CLI Features (JSON output, profile presets).

</details>

<details>
<summary>SHIPPED v1.5 Website Use Cases & Documentation (Phases 30-34) - SHIPPED 2026-03-04</summary>

See `.planning/milestones/v1.5-ROADMAP.md` for full phase details.

Phases 30-34: Foundation Fixes, Addon Page Depth, CLI Reference, Tutorials, Verification & Polish.

</details>

<details>
<summary>SHIPPED v2.0 Distribution & GPU Support (Phases 35-37) - SHIPPED 2026-03-05</summary>

Phases 35-37: GoReleaser Foundation, Homebrew Tap, NVIDIA GPU Addon.

</details>

<details>
<summary>SHIPPED v2.1 Known Issues & Proactive Diagnostics (Phases 38-41) - SHIPPED 2026-03-06</summary>

See `.planning/milestones/v2.1-ROADMAP.md` for full phase details.

Phases 38-41: Check Infrastructure, Docker & Tool Checks, Kernel & Platform Checks, Network/Create-Flow/Website.

</details>

<details>
<summary>SHIPPED v2.2 Cluster Capabilities (Phases 42-46) - SHIPPED 2026-04-10</summary>

See `.planning/milestones/v2.2-ROADMAP.md` for full phase details.

Phases 42-46: Multi-Version Node Validation, Air-Gapped Cluster Creation, Local-Path-Provisioner Addon, Host-Directory Mounting, `kinder load images` Command. Doctor registry expanded from 18 to 23 checks. Zero new Go module dependencies.

</details>

<details>
<summary>SHIPPED v2.3 Inner Loop (Phases 47-51) - SHIPPED 2026-05-07</summary>

See `.planning/milestones/v2.3-ROADMAP.md` for full phase details.

Phases 47-51: Cluster Pause/Resume, Cluster Snapshot/Restore, Inner-Loop Hot Reload (`kinder dev`), Runtime Error Decoder (`kinder doctor decode` with 16-pattern catalog), Upstream Sync (HAProxy→Envoy LB across docker/podman/nerdctl + IPVS-on-1.36+ guard + K8s 1.36 website recipe). SYNC-02 (default node image bump to K8s 1.36.x) DEFERRED — `kindest/node:v1.36.x` not yet on Docker Hub.

</details>

## Progress

**Execution Order:**
Phases execute in numeric order. Decimal phases (inserted via `/gsd-insert-phase`) run between their surrounding integers.

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8. v1.0 phases | v1.0 | 12/12 | Complete | 2026-03-01 |
| 9-14. v1.1 phases | v1.1 | 8/8 | Complete | 2026-03-02 |
| 15-18. v1.2 phases | v1.2 | 4/4 | Complete | 2026-03-02 |
| 19-24. v1.3 phases | v1.3 | 8/8 | Complete | 2026-03-03 |
| 25-29. v1.4 phases | v1.4 | 13/13 | Complete | 2026-03-04 |
| 30-34. v1.5 phases | v1.5 | 7/7 | Complete | 2026-03-04 |
| 35-37. v2.0 phases | v2.0 | 7/7 | Complete | 2026-03-05 |
| 38-41. v2.1 phases | v2.1 | 10/10 | Complete | 2026-03-06 |
| 42-46. v2.2 phases | v2.2 | 14/14 | Complete | 2026-04-10 |
| 47-51. v2.3 phases | v2.3 | 25/25 | Complete (SYNC-02 deferred) | 2026-05-07 |
