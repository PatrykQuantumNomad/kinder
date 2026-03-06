# Project Research Summary

**Project:** kinder -- Diagnostic checks and auto-mitigations for `kinder doctor`
**Domain:** System-level diagnostic tooling for Kubernetes-in-Docker cluster management (Go CLI)
**Researched:** 2026-03-06
**Confidence:** HIGH

## Executive Summary

This milestone adds 13 diagnostic checks to the existing `kinder doctor` command and introduces automatic mitigations during `kinder create cluster`. The research confirms that every check is implementable using Go standard library packages plus one already-present indirect dependency (`golang.org/x/sys/unix` v0.41.0). Zero new go.mod dependencies are needed. Six external libraries were evaluated and rejected. The existing doctor infrastructure -- result/checkResult pattern, JSON output, ok/warn/fail formatters, structured exit codes (0/1/2) -- provides a solid foundation, but the current inline-function approach does not scale to 19+ checks. A lightweight `Check` interface with an explicit registry slice and a shared `pkg/internal/doctor/` package is the recommended architecture.

The recommended approach is to first build the check infrastructure (interface, registry, platform filtering, "skip" status, mitigation tier system), then implement checks in three waves: Docker/tool configuration checks (cross-platform, highest user value), system resource and kernel checks (Linux-only, catches silent failures), and platform-specific/network checks (WSL2, SELinux, AppArmor, subnet clashes). Auto-mitigations must be strictly tiered: only environment variables and cluster config adjustments are safe for automatic application; sysctl changes should be suggested but never auto-applied; system configuration files should be documented but never modified by kinder. The doctor command must remain read-only and safe to run at any time.

The primary risks are: (1) `/proc` filesystem reads crashing on non-Linux platforms if checks lack proper gating, (2) daemon.json location varying across six different Docker install methods causing silent false negatives, (3) WSL2 detection producing false positives on Azure VMs, and (4) auto-mitigations modifying system state beyond the cluster lifecycle. All are preventable with the architecture and testing patterns documented in the research. The mitigation tier system must be defined in Phase 1 before any individual checks are implemented.

## Key Findings

### Recommended Stack

Every check uses Go stdlib (`os`, `os/exec`, `encoding/json`, `net/netip`, `strings`, `strconv`, `runtime`) plus the existing kinder exec wrappers (`sigs.k8s.io/kind/pkg/exec`). The only go.mod change is promoting `golang.org/x/sys` from indirect to direct dependency (same version, v0.41.0). Six external libraries were evaluated and rejected: `opencontainers/selinux` (shelling out to `getenforce` is simpler), `google/nftables` and `coreos/go-iptables` (only need to read config files), `Masterminds/semver` (kinder has `pkg/internal/version`), `shirou/gopsutil` (overkill for what `unix.Statfs` provides), and `yl2chen/cidranger` (`net/netip.Prefix.Overlaps()` is purpose-built).

**Core technologies:**
- `os.ReadFile` + `strconv`: procfs/sysfs reads for inotify, kernel, AppArmor, WSL2 -- standard Go approach for Linux system inspection
- `os/exec` via `sigs.k8s.io/kind/pkg/exec`: shell-outs to `docker`, `kubectl`, `getenforce`, `ip route` -- consistent with existing doctor pattern
- `encoding/json`: parse `docker info`, `docker network inspect`, `kubectl version`, `daemon.json` -- already imported in doctor.go
- `net/netip.ParsePrefix` + `Overlaps()`: subnet clash detection -- stdlib since Go 1.18, purpose-built for CIDR overlap
- `golang.org/x/sys/unix.Statfs` + `unix.Uname`: disk space and kernel version -- already in go.mod, replaces deprecated `syscall` package
- `sigs.k8s.io/kind/pkg/internal/version.ParseSemantic`: kubectl version skew comparison -- existing codebase, no external semver library needed

### Expected Features

**Must have (table stakes):**
- Disk space check -- #1 silent failure mode; warn at <5GB, fail at <2GB
- Docker daemon.json `"init": true` detection -- causes the most cryptic kind error message
- Kubectl version skew detection -- most common user confusion after Docker Desktop updates
- Docker socket permission check -- #1 new-user issue on Linux; enhancement to existing container-runtime check
- Inotify limits check -- top-5 kind debugging question on Linux; recommend >=524288 watches, >=512 instances
- Docker subnet clash detection -- extremely common in enterprise/VPN environments

**Should have (differentiators -- no Kind-based tool provides these):**
- Docker snap installation detection -- prevents TMPDIR debugging; simple path check
- AppArmor interference detection -- Linux desktop users frequently hit this
- Rootfs device node access check -- BTRFS/NVMe users need advance warning
- Firewalld nftables backend detection -- Fedora 32+ users need this detected early
- SELinux enforcing mode detection -- Fedora 33 specific, simple getenforce check
- Old kernel / cgroup namespace check -- hard blocker for RHEL 7 (kernel <4.6)
- WSL2 cgroup misconfiguration detection -- Windows developers using WSL2 need this

**Defer (v2+):**
- `kinder doctor --check <name>` -- run specific check subsets
- `kinder doctor --fix` -- apply safe auto-fixes with user confirmation
- Check result caching for repeated runs
- Post-creation health checks (pod scheduling, DNS, CoreDNS)
- Plugin-based check system for custom user checks
- CI mode with JUnit XML output

### Architecture Approach

The architecture centers on a `Check` interface with `Name()`, `Category()`, `Platforms()`, and `Run()` methods, registered in an explicit `AllChecks()` slice in a shared `pkg/internal/doctor/` package. This package is importable by both the doctor CLI command (`pkg/cmd/kind/doctor/`) and the create flow (`pkg/cluster/internal/create/`), maintaining correct dependency direction. Checks are organized by category (runtime, kernel, network, resources, gpu) in separate source files but execute as a flat ordered list. A new `"skip"` status is added to the result type for platform-inapplicable checks, treated as equivalent to `"ok"` for exit code purposes. Each check uses a deps struct pattern for injectable dependencies, enabling full parallel test execution without global state mutation.

**Major components:**
1. **Check interface + Result type + AllChecks registry** (`pkg/internal/doctor/check.go`) -- contract for all checks, explicit ordered registry, exported Result struct with Category field
2. **Category-organized check implementations** (`pkg/internal/doctor/runtime.go`, `kernel.go`, `network.go`, `resources.go`, `gpu.go`) -- each check uses deps struct for injectable testing
3. **Mitigations module** (`pkg/internal/doctor/mitigations.go`) -- `SafeMitigation` struct with `NeedsFix`/`Apply`/`NeedsRoot` fields; `ApplySafeMitigations()` entry point called by create flow
4. **Refactored doctor CLI** (`pkg/cmd/kind/doctor/doctor.go`) -- uses `AllChecks()` loop with centralized platform filtering, category-grouped human output, backward-compatible JSON with `category` field
5. **Create flow integration** (`pkg/cluster/internal/create/create.go`) -- calls `doctor.ApplySafeMitigations(logger)` after `validateProvider()` and before `p.Provision()`

### Critical Pitfalls

1. **`/proc` reads crash on non-Linux** -- Every check reading `/proc` or `/sys` must be gated by the Check interface's `Platforms()` method with centralized filtering in the main loop. Use `//go:build linux` for helper functions calling Linux-only syscalls; keep check registration cross-platform to emit "skip" status on macOS/Windows.

2. **daemon.json location varies across 6+ install methods** -- Must implement `daemonJSONPaths()` returning prioritized candidate paths based on `runtime.GOOS` and env vars. Native Linux (`/etc/docker/daemon.json`), Docker Desktop macOS (`~/.docker/daemon.json`), rootless Docker (`$XDG_CONFIG_HOME/docker/daemon.json`), Snap Docker (`/var/snap/docker/current/config/daemon.json`), Rancher Desktop (`~/.rd/docker/daemon.json`). Hardcoding a single path is never acceptable.

3. **Auto-mitigation modifying system state is dangerous** -- Never auto-apply sysctl changes, never call `sudo` from kinder, never modify system config files. Tier the strategy: Tier 1 (auto-apply: env vars, cluster config adjustments), Tier 2 (suggest only: `sysctl -w`), Tier 3 (document only: editing sysctl.conf, disabling firewalld/SELinux).

4. **WSL2 detection false positives on Azure VMs** -- `/proc/version` containing "microsoft" alone is insufficient. Require multi-signal detection: `/proc/version` + (`$WSL_DISTRO_NAME` OR `/proc/sys/fs/binfmt_misc/WSLInterop`). Never auto-mitigate based on WSL2 detection alone.

5. **SELinux and AppArmor can coexist since kernel 5.1** -- LSMs can stack. Check both independently, report both, indicate which is the active enforcing MAC. Do not use if/else pattern assuming mutual exclusivity.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Check Infrastructure and Interface

**Rationale:** All 13 checks and the create-flow integration depend on the Check interface, Result type with "skip" status, AllChecks registry, category-grouped output, and mitigation tier system. This is the foundation -- nothing else can proceed without it.
**Delivers:** `pkg/internal/doctor/` package with Check interface, Result type (ok/warn/fail/skip), AllChecks() registry, category-grouped human output, backward-compatible JSON output with category field, mitigation tier constants, SafeMitigation struct, and ApplySafeMitigations entry point skeleton. Migrate existing checks (container runtime, kubectl, NVIDIA GPU) to the new interface.
**Addresses:** Check registration pattern, platform filtering, "skip" status, exit code contract preservation (0/1/2), deps struct testability pattern
**Avoids:** Pitfall 1 (/proc crash on non-Linux by establishing platform filtering), Pitfall 5 (auto-mitigation safety by defining tiers), Pitfall F2 (import layering by establishing pkg/internal/doctor/), Pitfall F3 (exit code contract by adding "skip" status)

### Phase 2: Docker and Tool Configuration Checks

**Rationale:** Docker configuration checks (daemon.json, snap, socket permissions) and tool version checks (kubectl skew) are cross-platform or near-cross-platform, have the highest user value, and are the simplest to implement (file reads and command output parsing). They depend on the Phase 1 infrastructure but not on platform-specific kernel APIs. Also includes disk space check which works cross-platform via `unix.Statfs`.
**Delivers:** Checks 1 (kubectl version skew), 2 (Docker snap), 3 (disk space), 4 (daemon.json init:true), 5 (Docker socket permissions). Five of six table-stakes features.
**Uses:** `os.ReadFile`, `encoding/json`, `path/filepath.EvalSymlinks`, `golang.org/x/sys/unix.Statfs`, `pkg/internal/version.ParseSemantic`
**Avoids:** Pitfall 3 (daemon.json multi-path resolution implemented from day one)

### Phase 3: Kernel, Security, and Platform-Specific Checks

**Rationale:** Linux-only checks (inotify, kernel version, SELinux, AppArmor, firewalld, WSL2, rootfs device) share the pattern of reading `/proc`/`/sys` or shelling out to Linux-specific commands. Grouping them ensures consistent platform gating and testing. WSL2 detection requires multi-signal approach. SELinux+AppArmor require independent checking.
**Delivers:** Checks 6 (inotify limits), 7 (AppArmor), 8 (rootfs device node), 9 (firewalld nftables), 10 (SELinux), 11 (kernel version), 12 (WSL2 cgroups). All seven differentiator features.
**Uses:** `os.ReadFile` for procfs/sysfs, `exec.Command` for `getenforce`/`aa-status`, `golang.org/x/sys/unix.Uname`
**Avoids:** Pitfall 4 (WSL2 false positives with multi-signal detection), Pitfall 7 (SELinux+AppArmor coexistence with independent checks)

### Phase 4: Network Checks, Create-Flow Integration, and Polish

**Rationale:** Subnet clash detection (Check 13) requires cross-platform route table parsing which is the most complex single check. Create-flow integration (`ApplySafeMitigations` call in `create.go`) should come last so all checks and mitigations are available. Performance optimization rounds out the milestone.
**Delivers:** Check 13 (Docker subnet clash), create-flow pre-flight integration, performance optimization (batch `docker info` call, consider parallel check groups), comprehensive end-to-end testing.
**Uses:** `net/netip.ParsePrefix` + `Overlaps()`, platform-specific route parsers (`ip route` on Linux, `netstat -rn` on macOS), `docker network inspect`
**Avoids:** Pitfall 6 (cross-platform route enumeration with platform-separated implementations), performance traps (sequential execution of 19+ checks)

### Phase Ordering Rationale

- **Phase 1 first:** Every other phase depends on the Check interface and registry. The mitigation tier system prevents dangerous auto-fix patterns from creeping into later phases.
- **Phase 2 before Phase 3:** Cross-platform checks have wider user impact and simpler implementation. Docker/tool checks work on macOS where most kinder development happens, enabling faster iteration and feedback.
- **Phase 3 before Phase 4:** Linux-specific checks are independent of each other and can be implemented in parallel. They must be complete before create-flow integration so all mitigations are available.
- **Phase 4 last:** Subnet clash detection is the most complex single check. Create-flow integration is a final wiring step that benefits from all checks being available. Performance optimization is polish.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3:** WSL2 detection requires testing on actual WSL2 and Azure VM environments to validate multi-signal approach. SELinux+AppArmor coexistence needs testing on LSM-stacking distros. Rootfs device node detection for BTRFS/NVMe has high complexity and may produce false positives.
- **Phase 4:** Cross-platform route table parsing needs real output samples from various Linux distros and macOS versions. Docker Desktop network isolation behavior needs validation for subnet clash detection.

Phases with standard patterns (skip research-phase):
- **Phase 1:** Check interface pattern, registry slice, Result type -- all derived from direct codebase analysis. Well-documented Go patterns with clear precedent in the existing codebase.
- **Phase 2:** File reads, JSON parsing, version comparison, `Statfs` -- all use Go stdlib with well-documented APIs. Daemon.json paths documented in Docker's official docs.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Zero new dependencies; all APIs verified against Go stdlib docs and existing go.mod. `golang.org/x/sys/unix` already present at v0.41.0. |
| Features | HIGH | All 13 checks map directly to kind's documented Known Issues page. Thresholds and detection methods verified against official docs and kind issue tracker. |
| Architecture | HIGH | Based entirely on direct codebase analysis. Package layering, import direction, test patterns, and dependency struct approach all verified against existing code conventions. |
| Pitfalls | HIGH/MEDIUM | HIGH for /proc gating, daemon.json paths, test isolation, permission, exit code, and firewalld pitfalls. MEDIUM for WSL2 detection (heuristic-based, needs empirical validation) and SELinux+AppArmor coexistence (kernel 5.1+ LSM stacking is documented but rare in practice). |

**Overall confidence:** HIGH

### Gaps to Address

- **WSL2 multi-signal detection validation:** The recommended two-signal approach has not been tested on actual Azure VMs to confirm false-positive prevention. Validate during Phase 3 implementation with real WSL2 and Azure VM environments.
- **Docker Desktop inotify remediation:** On macOS/Windows, inotify limits are inside Docker's Linux VM. The correct remediation path (Docker Desktop settings vs. `wsl -d docker-desktop sysctl`) needs platform-specific validation during Phase 3.
- **Rootfs device node detection specificity:** The BTRFS/NVMe issue only affects certain configurations. The check may produce false positives on BTRFS setups that work fine with kind. Needs empirical testing during Phase 3.
- **Performance budget:** The 2-second target for all checks assumes some parallelization. The actual latency of `docker info`, `docker network inspect`, and other shell-outs needs measurement during Phase 4 to determine if parallel execution is required.
- **Existing check migration scope:** Migrating existing checks (container runtime, kubectl, NVIDIA) to the new Check interface is included in Phase 1 but could be deferred. Trade-off between clean migration vs. incremental adoption needs a decision during Phase 1 planning.
- **kubectl version skew without running cluster:** Full skew validation requires both client and server versions. Without a running cluster, doctor can only report client version and flag very old versions. Consider adding `--cluster` flag support as a v2 enhancement.

## Sources

### Primary (HIGH confidence)
- Kind Known Issues page: https://kind.sigs.k8s.io/docs/user/known-issues/ -- specification for all 13 checks
- Docker daemon configuration docs: https://docs.docker.com/engine/daemon/ -- daemon.json locations and options
- Go `net/netip` package: https://pkg.go.dev/net/netip -- `ParsePrefix`, `Overlaps` for subnet clash detection
- Go `golang.org/x/sys/unix` package: https://pkg.go.dev/golang.org/x/sys/unix -- v0.41.0, `Statfs`, `Uname`
- Kubernetes Version Skew Policy: https://kubernetes.io/releases/version-skew-policy/ -- kubectl +/-1 minor version
- Firewalld nftables backend docs: https://firewalld.org/2018/07/nftables-backend -- `FirewallBackend=` configuration
- Kubernetes inotify issue #46230: https://github.com/kubernetes/kubernetes/issues/46230 -- recommended thresholds
- Kinder codebase: `doctor.go`, `create.go`, `go.mod`, `network.go`, `nvidiagpu.go`, `exec/types.go`, `version.go` -- architecture patterns and conventions

### Secondary (MEDIUM confidence)
- WSL2 /proc/version detection: https://gist.github.com/s0kil/336a246cc2bc8608e645c69876c17466 -- community pattern, universally used
- Microsoft WSL detection issue #4071: https://github.com/microsoft/WSL/issues/4071 -- documents false positive/negative cases
- WSL2 cgroupsv2 fix guide: https://github.com/spurin/wsl-cgroupsv2 -- community solution for cgroup misconfiguration
- SELinux vs AppArmor LSM stacking analysis: https://securitylabs.datadoghq.com/articles/container-security-fundamentals-part-5/
- Inotify in containers behavior: https://william-yeh.net/post/2019/06/inotify-in-containers/

### Tertiary (LOW confidence)
- None -- all findings corroborated by at least two sources

---
*Research completed: 2026-03-06*
*Ready for roadmap: yes*
