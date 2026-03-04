# Requirements: Kinder

**Defined:** 2026-03-04
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## v2.0 Requirements

Requirements for v2.0 release. Each maps to roadmap phases.

### Release Pipeline

- [ ] **REL-01**: GoReleaser config produces cross-platform binaries (linux/darwin amd64+arm64, windows amd64)
- [ ] **REL-02**: GoReleaser generates SHA-256 checksums file for all release artifacts
- [ ] **REL-03**: GoReleaser generates automated changelog from git commit history
- [ ] **REL-04**: Release binaries show correct version metadata via `kinder version`
- [ ] **REL-05**: GitHub Actions release workflow uses goreleaser-action replacing cross.sh + softprops
- [ ] **REL-06**: GoReleaser explicitly sets `gomod.proxy: false` and `project_name: kinder` (fork safety)

### Homebrew

- [ ] **BREW-01**: `homebrew-kinder` tap repository exists under PatrykQuantumNomad
- [ ] **BREW-02**: GoReleaser publishes Cask to tap repo on tagged release via HOMEBREW_TAP_TOKEN
- [ ] **BREW-03**: User can install kinder via `brew install patrykquantumnomad/kinder/kinder`

### GPU Addon

- [ ] **GPU-01**: NVIDIA device plugin DaemonSet installs via go:embed + kubectl apply when addon enabled
- [ ] **GPU-02**: NVIDIA RuntimeClass "nvidia" created so pods can target GPU nodes
- [ ] **GPU-03**: v1alpha4 config API has `NvidiaGPU *bool` field defaulting to false (opt-in)
- [ ] **GPU-04**: GPU addon skips with informational message on non-Linux platforms
- [ ] **GPU-05**: `kinder doctor` checks for NVIDIA driver, container toolkit, and nvidia runtime in Docker
- [ ] **GPU-06**: GPU addon pre-flight check in `Execute()` validates host toolkit before applying manifests

### Website

- [ ] **SITE-01**: Installation page updated with Homebrew install instructions alongside make install
- [ ] **SITE-02**: GPU addon documentation page with prerequisites, configuration, usage, and troubleshooting

## Future Requirements

### Distribution Enhancements

- **DIST-01**: Shell completions bundled in Homebrew Cask (bash/zsh/fish)
- **DIST-02**: Cosign binary signing for supply chain security
- **DIST-03**: Chocolatey/Winget packages for Windows

### GPU Enhancements

- **GPUX-01**: GPU time-slicing ConfigMap for multi-pod GPU sharing
- **GPUX-02**: AMD ROCm GPU addon

## Out of Scope

| Feature | Reason |
|---------|--------|
| Homebrew core submission | Requires project maturity, 30-day review; fork module path complicates indefinitely |
| GPU Operator (full stack) | Kind is not on NVIDIA's official supported platforms list; device plugin is sufficient |
| `driver.enabled=true` mode | Kind nodes are Docker containers without kernel module access; host drivers required |
| Windows GPU passthrough | Docker Desktop GPU passthrough doesn't expose GPUs to kind's containerd |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| REL-01 | — | Pending |
| REL-02 | — | Pending |
| REL-03 | — | Pending |
| REL-04 | — | Pending |
| REL-05 | — | Pending |
| REL-06 | — | Pending |
| BREW-01 | — | Pending |
| BREW-02 | — | Pending |
| BREW-03 | — | Pending |
| GPU-01 | — | Pending |
| GPU-02 | — | Pending |
| GPU-03 | — | Pending |
| GPU-04 | — | Pending |
| GPU-05 | — | Pending |
| GPU-06 | — | Pending |
| SITE-01 | — | Pending |
| SITE-02 | — | Pending |

**Coverage:**
- v2.0 requirements: 17 total
- Mapped to phases: 0
- Unmapped: 17 ⚠️

---
*Requirements defined: 2026-03-04*
*Last updated: 2026-03-04 after initial definition*
