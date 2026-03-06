# Requirements: Kinder

**Defined:** 2026-03-06
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## v2.1 Requirements

Requirements for v2.1 Known Issues & Proactive Diagnostics. Each maps to roadmap phases.

### Infrastructure

- [x] **INFRA-01**: Check interface with Name(), Category(), Platforms(), Run() methods in pkg/internal/doctor/
- [x] **INFRA-02**: Result type with ok/warn/fail/skip statuses and category-grouped output
- [x] **INFRA-03**: AllChecks() registry with centralized platform filtering
- [x] **INFRA-04**: Mitigation tier system (auto-apply, suggest-only, document-only) with SafeMitigation struct
- [x] **INFRA-05**: ApplySafeMitigations() integrated into create flow before p.Provision()
- [x] **INFRA-06**: Existing checks (container-runtime, kubectl, NVIDIA GPU) migrated to Check interface

### Docker & Tools

- [x] **DOCK-01**: Doctor checks available disk space, warns at <5GB, fails at <2GB
- [x] **DOCK-02**: Doctor detects daemon.json "init: true" across 6+ location candidates
- [x] **DOCK-03**: Doctor detects Docker installed via snap and warns about TMPDIR issues
- [x] **DOCK-04**: Doctor detects kubectl version skew and warns about incompatibility
- [x] **DOCK-05**: Doctor detects Docker socket permission denied and suggests fix

### Kernel & Security

- [x] **KERN-01**: Doctor checks inotify max_user_watches (>=524288) and max_user_instances (>=512)
- [x] **KERN-02**: Doctor detects AppArmor profiles interfering with kind containers
- [x] **KERN-03**: Doctor detects SELinux enforcing mode on Fedora
- [x] **KERN-04**: Doctor checks kernel version >=4.6 for cgroup namespace support

### Platform & Network

- [x] **PLAT-01**: Doctor detects firewalld nftables backend on Fedora 32+
- [x] **PLAT-02**: Doctor detects WSL2 with multi-signal approach and checks cgroup v2 config
- [x] **PLAT-03**: Doctor detects Docker network subnet clashes with host routes
- [x] **PLAT-04**: Doctor checks device node access for rootfs (BTRFS/NVMe)

### Website

- [x] **SITE-01**: Known Issues / Troubleshooting page documenting all checks and mitigations

## v2.0 Requirements (Complete)

### Release Pipeline

- [x] **REL-01**: GoReleaser config produces cross-platform binaries (linux/darwin amd64+arm64, windows amd64)
- [x] **REL-02**: GoReleaser generates SHA-256 checksums file for all release artifacts
- [x] **REL-03**: GoReleaser generates automated changelog from git commit history
- [x] **REL-04**: Release binaries show correct version metadata via `kinder version`
- [x] **REL-05**: GitHub Actions release workflow uses goreleaser-action replacing cross.sh + softprops
- [x] **REL-06**: GoReleaser explicitly sets `gomod.proxy: false` and `project_name: kinder` (fork safety)

### Homebrew

- [ ] **BREW-01**: `homebrew-kinder` tap repository exists under PatrykQuantumNomad
- [ ] **BREW-02**: GoReleaser publishes Cask to tap repo on tagged release via HOMEBREW_TAP_TOKEN
- [ ] **BREW-03**: User can install kinder via `brew install patrykquantumnomad/kinder/kinder`

### GPU Addon

- [x] **GPU-01**: NVIDIA device plugin DaemonSet installs via go:embed + kubectl apply when addon enabled
- [x] **GPU-02**: NVIDIA RuntimeClass "nvidia" created so pods can target GPU nodes
- [x] **GPU-03**: v1alpha4 config API has `NvidiaGPU *bool` field defaulting to false (opt-in)
- [x] **GPU-04**: GPU addon skips with informational message on non-Linux platforms
- [x] **GPU-05**: `kinder doctor` checks for NVIDIA driver, container toolkit, and nvidia runtime in Docker
- [x] **GPU-06**: GPU addon pre-flight check in `Execute()` validates host toolkit before applying manifests

### v2.0 Website

- [x] **SITE-v2.0-01**: Installation page updated with Homebrew install instructions alongside make install
- [x] **SITE-v2.0-02**: GPU addon documentation page with prerequisites, configuration, usage, and troubleshooting

## Future Requirements

Deferred to future release. Tracked but not in current roadmap.

### CLI Enhancements

- **CLI-01**: `kinder doctor --check <name>` to run specific check subsets
- **CLI-02**: `kinder doctor --fix` to apply safe auto-fixes with user confirmation
- **CLI-03**: Check result caching for repeated runs

### Distribution Enhancements

- **DIST-01**: Shell completions bundled in Homebrew Cask (bash/zsh/fish)
- **DIST-02**: Cosign binary signing for supply chain security
- **DIST-03**: Chocolatey/Winget packages for Windows

### GPU Enhancements

- **GPUX-01**: GPU time-slicing ConfigMap for multi-pod GPU sharing
- **GPUX-02**: AMD ROCm GPU addon

### Post-Creation Health

- **HLTH-01**: Post-creation health checks (pod scheduling, DNS, CoreDNS)

### Extensibility

- **EXT-01**: Plugin-based check system for custom user checks
- **EXT-02**: CI mode with JUnit XML output

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Auto-applying sysctl changes | Modifying kernel parameters without explicit user action is dangerous |
| Calling sudo from kinder | Never elevate privileges automatically; suggest commands for user to run |
| Modifying system config files | daemon.json, sysctl.conf, firewalld.conf are system-level; document-only |
| Post-creation pod health checks | Different concern; v2.1 focuses on pre-creation diagnostics |
| Check plugins | Hardcoded checks sufficient; extensibility deferred |
| Homebrew core submission | Requires project maturity, 30-day review; fork module path complicates indefinitely |
| GPU Operator (full stack) | Kind is not on NVIDIA's official supported platforms list; device plugin is sufficient |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | Phase 38 | Complete |
| INFRA-02 | Phase 38 | Complete |
| INFRA-03 | Phase 38 | Complete |
| INFRA-04 | Phase 38 | Complete |
| INFRA-05 | Phase 41 | Complete |
| INFRA-06 | Phase 38 | Complete |
| DOCK-01 | Phase 39 | Complete |
| DOCK-02 | Phase 39 | Complete |
| DOCK-03 | Phase 39 | Complete |
| DOCK-04 | Phase 39 | Complete |
| DOCK-05 | Phase 39 | Complete |
| KERN-01 | Phase 40 | Complete |
| KERN-02 | Phase 40 | Complete |
| KERN-03 | Phase 40 | Complete |
| KERN-04 | Phase 40 | Complete |
| PLAT-01 | Phase 40 | Complete |
| PLAT-02 | Phase 40 | Complete |
| PLAT-03 | Phase 41 | Complete |
| PLAT-04 | Phase 40 | Complete |
| SITE-01 | Phase 41 | Complete |

**Coverage:**
- v2.1 requirements: 20 total
- Mapped to phases: 20
- Unmapped: 0

---
*Requirements defined: 2026-03-06*
*Last updated: 2026-03-06 after roadmap creation*
