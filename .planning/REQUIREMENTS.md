# Requirements: Kinder

**Defined:** 2026-03-06
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## v2.1 Requirements

Requirements for v2.1 Known Issues & Proactive Diagnostics. Each maps to roadmap phases.

### Infrastructure

- [ ] **INFRA-01**: Check interface with Name(), Category(), Platforms(), Run() methods in pkg/internal/doctor/
- [ ] **INFRA-02**: Result type with ok/warn/fail/skip statuses and category-grouped output
- [ ] **INFRA-03**: AllChecks() registry with centralized platform filtering
- [ ] **INFRA-04**: Mitigation tier system (auto-apply, suggest-only, document-only) with SafeMitigation struct
- [ ] **INFRA-05**: ApplySafeMitigations() integrated into create flow before p.Provision()
- [ ] **INFRA-06**: Existing checks (container-runtime, kubectl, NVIDIA GPU) migrated to Check interface

### Docker & Tools

- [ ] **DOCK-01**: Doctor checks available disk space, warns at <5GB, fails at <2GB
- [ ] **DOCK-02**: Doctor detects daemon.json "init: true" across 6+ location candidates
- [ ] **DOCK-03**: Doctor detects Docker installed via snap and warns about TMPDIR issues
- [ ] **DOCK-04**: Doctor detects kubectl version skew and warns about incompatibility
- [ ] **DOCK-05**: Doctor detects Docker socket permission denied and suggests fix

### Kernel & Security

- [ ] **KERN-01**: Doctor checks inotify max_user_watches (>=524288) and max_user_instances (>=512)
- [ ] **KERN-02**: Doctor detects AppArmor profiles interfering with kind containers
- [ ] **KERN-03**: Doctor detects SELinux enforcing mode on Fedora
- [ ] **KERN-04**: Doctor checks kernel version >=4.6 for cgroup namespace support

### Platform & Network

- [ ] **PLAT-01**: Doctor detects firewalld nftables backend on Fedora 32+
- [ ] **PLAT-02**: Doctor detects WSL2 with multi-signal approach and checks cgroup v2 config
- [ ] **PLAT-03**: Doctor detects Docker network subnet clashes with host routes
- [ ] **PLAT-04**: Doctor checks device node access for rootfs (BTRFS/NVMe)

### Website

- [ ] **SITE-01**: Known Issues / Troubleshooting page documenting all checks and mitigations

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
| INFRA-01 | — | Pending |
| INFRA-02 | — | Pending |
| INFRA-03 | — | Pending |
| INFRA-04 | — | Pending |
| INFRA-05 | — | Pending |
| INFRA-06 | — | Pending |
| DOCK-01 | — | Pending |
| DOCK-02 | — | Pending |
| DOCK-03 | — | Pending |
| DOCK-04 | — | Pending |
| DOCK-05 | — | Pending |
| KERN-01 | — | Pending |
| KERN-02 | — | Pending |
| KERN-03 | — | Pending |
| KERN-04 | — | Pending |
| PLAT-01 | — | Pending |
| PLAT-02 | — | Pending |
| PLAT-03 | — | Pending |
| PLAT-04 | — | Pending |
| SITE-01 | — | Pending |

**Coverage:**
- v2.1 requirements: 20 total
- Mapped to phases: 0
- Unmapped: 20

---
*Requirements defined: 2026-03-06*
*Last updated: 2026-03-06 after initial definition*
