# Roadmap: Kinder

## Milestones

- SHIPPED **v1.0 Batteries Included** - Phases 1-8 (shipped 2026-03-01)
- SHIPPED **v1.1 Kinder Website** - Phases 9-14 (shipped 2026-03-02)
- SHIPPED **v1.2 Branding & Polish** - Phases 15-18 (shipped 2026-03-02)
- SHIPPED **v1.3 Harden & Extend** - Phases 19-24 (shipped 2026-03-03)
- SHIPPED **v1.4 Code Quality & Features** - Phases 25-29 (shipped 2026-03-04)
- SHIPPED **v1.5 Website Use Cases & Documentation** - Phases 30-34 (shipped 2026-03-04)
- SHIPPED **v2.0 Distribution & GPU Support** - Phases 35-37 (shipped 2026-03-05)
- IN PROGRESS **v2.1 Known Issues & Proactive Diagnostics** - Phases 38-41

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

### v2.1 Known Issues & Proactive Diagnostics (In Progress)

**Milestone Goal:** Address Kind's documented known issues by expanding `kinder doctor` with 13 new diagnostic checks, adding automatic mitigations during cluster creation where safe, and documenting all checks on the website.

- [ ] **Phase 38: Check Infrastructure and Interface** - Shared doctor package with Check interface, Result type, registry, mitigation tiers, and existing check migration
- [ ] **Phase 39: Docker and Tool Configuration Checks** - Cross-platform checks for disk space, daemon.json, Docker snap, kubectl skew, and socket permissions
- [ ] **Phase 40: Kernel, Security, and Platform-Specific Checks** - Linux-only checks for inotify, AppArmor, SELinux, kernel version, firewalld, WSL2, and rootfs devices
- [ ] **Phase 41: Network, Create-Flow Integration, and Website** - Subnet clash detection, ApplySafeMitigations wiring into create flow, and Known Issues documentation page

## Phase Details

### Phase 38: Check Infrastructure and Interface
**Goal**: Developers can run `kinder doctor` and see all existing checks (container runtime, kubectl, NVIDIA GPU) executing through a unified Check interface with category-grouped output and platform filtering
**Depends on**: Phase 37 (v2.0 complete)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-06
**Success Criteria** (what must be TRUE):
  1. `kinder doctor` produces category-grouped output (e.g., "Runtime", "Tools", "GPU") instead of a flat list, with each check showing ok/warn/fail/skip status
  2. Running `kinder doctor` on macOS skips Linux-only checks with a "skip" status instead of crashing or silently omitting them
  3. `kinder doctor --output json` includes a `category` field on every check result and treats "skip" as equivalent to "ok" for exit code purposes (exit 0 when all checks are ok or skip)
  4. The mitigation tier system is defined with SafeMitigation struct exposing NeedsFix/Apply/NeedsRoot fields, and an ApplySafeMitigations() entry point exists (skeleton -- wired in Phase 41)
  5. All three existing checks (container runtime, kubectl, NVIDIA GPU) are migrated to the Check interface and produce identical user-visible output as before migration
**Plans**: 2 plans

Plans:
- [ ] 38-01-PLAN.md — Check interface, Result type, registry, platform filtering, output formatters, SafeMitigation skeleton
- [ ] 38-02-PLAN.md — Migrate 3 existing checks to Check interface and refactor doctor.go CLI layer

### Phase 39: Docker and Tool Configuration Checks
**Goal**: Users running `kinder doctor` get actionable warnings about the five most common Docker/tool configuration problems before they cause cryptic cluster creation failures
**Depends on**: Phase 38 (Check interface and registry must exist)
**Requirements**: DOCK-01, DOCK-02, DOCK-03, DOCK-04, DOCK-05
**Success Criteria** (what must be TRUE):
  1. `kinder doctor` warns when available disk space is below 5GB and fails when below 2GB, with the check working on both Linux and macOS
  2. `kinder doctor` detects `"init": true` in daemon.json across all six candidate locations (native Linux, Docker Desktop macOS, rootless, Snap, Rancher Desktop, Windows) and warns that it will cause kind cluster failures
  3. `kinder doctor` detects Docker installed via snap (symlink to /snap/bin/docker) and warns about TMPDIR issues
  4. `kinder doctor` detects kubectl client version skew (more than one minor version from the cluster's server version) and warns about potential incompatibility
  5. `kinder doctor` detects Docker socket permission denied on Linux and suggests the specific fix command (adding user to docker group)
**Plans**: TBD

Plans:
- [ ] 39-01: TBD
- [ ] 39-02: TBD

### Phase 40: Kernel, Security, and Platform-Specific Checks
**Goal**: Linux users running `kinder doctor` get advance warnings about kernel limits, security modules, and platform configurations that would silently break or degrade kind clusters
**Depends on**: Phase 39 (Docker checks complete; all checks share testing patterns established in earlier phases)
**Requirements**: KERN-01, KERN-02, KERN-03, KERN-04, PLAT-01, PLAT-02, PLAT-04
**Success Criteria** (what must be TRUE):
  1. `kinder doctor` on Linux checks inotify max_user_watches (warns if <524288) and max_user_instances (warns if <512), and suggests the exact sysctl commands to fix them
  2. `kinder doctor` on Linux detects AppArmor profiles that interfere with kind containers and SELinux enforcing mode on Fedora, checking both independently (not mutually exclusive)
  3. `kinder doctor` on Linux checks kernel version and warns if below 4.6 (no cgroup namespace support), which is a hard blocker for kind
  4. `kinder doctor` on Linux detects firewalld with nftables backend on Fedora 32+ and warns about networking issues
  5. `kinder doctor` detects WSL2 using multi-signal approach (/proc/version + $WSL_DISTRO_NAME or /proc/sys/fs/binfmt_misc/WSLInterop) and checks cgroup v2 configuration, without false-positiving on Azure VMs
**Plans**: TBD

Plans:
- [ ] 40-01: TBD
- [ ] 40-02: TBD

### Phase 41: Network, Create-Flow Integration, and Website
**Goal**: Users get subnet clash detection before cluster creation, automatic safe mitigations during `kinder create cluster`, and a comprehensive Known Issues page on the website documenting all checks
**Depends on**: Phase 40 (all checks must exist before create-flow integration wires mitigations)
**Requirements**: PLAT-03, INFRA-05, SITE-01
**Success Criteria** (what must be TRUE):
  1. `kinder doctor` detects when Docker network subnets overlap with host routing table entries and warns about potential connectivity issues, working on both Linux (`ip route`) and macOS (`netstat -rn`)
  2. `kinder create cluster` calls ApplySafeMitigations() after provider validation and before provisioning, applying only tier-1 mitigations (env vars, cluster config adjustments) automatically -- never calling sudo or modifying system files
  3. The kinder website at kinder.patrykgolabek.dev has a Known Issues / Troubleshooting page documenting every diagnostic check, what it detects, why it matters, and how to fix it
**Plans**: TBD

Plans:
- [ ] 41-01: TBD
- [ ] 41-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 38 -> 39 -> 40 -> 41

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8. v1.0 phases | v1.0 | 12/12 | Complete | 2026-03-01 |
| 9-14. v1.1 phases | v1.1 | 8/8 | Complete | 2026-03-02 |
| 15-18. v1.2 phases | v1.2 | 4/4 | Complete | 2026-03-02 |
| 19-24. v1.3 phases | v1.3 | 8/8 | Complete | 2026-03-03 |
| 25-29. v1.4 phases | v1.4 | 13/13 | Complete | 2026-03-04 |
| 30-34. v1.5 phases | v1.5 | 7/7 | Complete | 2026-03-04 |
| 35. GoReleaser Foundation | v2.0 | 2/2 | Complete | 2026-03-04 |
| 36. Homebrew Tap | v2.0 | 2/2 | Complete | 2026-03-04 |
| 37. NVIDIA GPU Addon | v2.0 | 3/3 | Complete | 2026-03-05 |
| 38. Check Infrastructure | v2.1 | 0/2 | Not started | - |
| 39. Docker & Tool Checks | v2.1 | 0/? | Not started | - |
| 40. Kernel & Platform Checks | v2.1 | 0/? | Not started | - |
| 41. Network, Integration & Website | v2.1 | 0/? | Not started | - |
