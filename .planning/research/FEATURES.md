# Feature Research: Diagnostic Checks and Automatic Mitigations

**Domain:** Kubernetes local development tooling -- preflight diagnostics and proactive issue detection for Kind clusters
**Researched:** 2026-03-06
**Confidence:** HIGH for check behavior/thresholds (Kind known issues page + kubeadm preflight patterns + OS-level docs verified); MEDIUM for auto-fix feasibility (some fixes require root or are destructive)

---

## Context and Scope

This research covers **13 new diagnostic checks** for `kinder doctor` and **automatic mitigations during `kinder create cluster`**. The existing doctor infrastructure provides:

- `result` struct with `name`, `status` ("ok" / "warn" / "fail"), and `message`
- Human-readable output: `[ OK ]`, `[WARN]`, `[FAIL]` with inline messages
- JSON output mode (`--output json`) with structured exit codes (0=ok, 1=fail, 2=warn)
- Platform gating via `runtime.GOOS == "linux"` for OS-specific checks
- Existing checks: container runtime (docker/podman/nerdctl), kubectl, NVIDIA GPU (3 checks)
- Pattern: `osexec.LookPath` for binary detection, `exec.OutputLines` for command output

The 13 new checks address every actionable item from Kind's documented [Known Issues](https://kind.sigs.k8s.io/docs/user/known-issues/) page.

---

## Feature Landscape

### Table Stakes (Users Expect These)

These checks detect the most common Kind failure modes. Users hitting these issues waste hours debugging. A "batteries-included" tool must catch them proactively.

| # | Feature | Why Expected | Complexity | Platform | Notes |
|---|---------|--------------|------------|----------|-------|
| 1 | **Kubectl version skew detection** | kubectl +/- 1 minor version from API server is the official K8s policy; Docker Desktop bundled kubectl is commonly stale | MEDIUM | All | Must parse semver from `kubectl version --client -o json`; compare against Kind node image's K8s version |
| 3 | **Disk space check** | Most common silent failure; kubelet evicts pods with no obvious error; Kind issue [#2717](https://github.com/kubernetes-sigs/kind/issues/2717) and [#3313](https://github.com/kubernetes-sigs/kind/issues/3313) | LOW | All | Check Docker data-root free space via `docker system info` or `df`; threshold: warn at <5GB, fail at <2GB |
| 4 | **Inotify limits check** | "too many open files" is a top-5 Kind debugging question; Ubuntu default 8192 watches is too low for multi-node clusters | LOW | Linux | Read `/proc/sys/fs/inotify/max_user_watches` and `max_user_instances`; warn if <65536 / <512 |
| 5 | **Docker socket permission check** | "permission denied" on Docker socket is the #1 new-user issue on Linux; wastes time before cluster creation even starts | LOW | Linux | Test `docker info` execution; parse error for "permission denied"; provide `usermod -aG docker` guidance |
| 6 | **Docker daemon.json "init: true" detection** | Causes cryptic "Couldn't find an alternative telinit implementation to spawn" error that is completely opaque | LOW | All | Read and parse `/etc/docker/daemon.json` (or Docker Desktop equivalent); check for `"init": true` |
| 13 | **Docker subnet clash detection** | VPN/corporate network conflicts with Docker's 172.17.0.0/16 default cause routing failures; extremely common in enterprise environments | MEDIUM | All | Compare Docker bridge subnet against host routing table; detect overlap with `ip route` / `netstat -rn` |

### Differentiators (Competitive Advantage)

These checks go beyond what Kind itself provides. No Kind-based tool proactively detects these issues -- they all rely on users reading the Known Issues page after failure.

| # | Feature | Value Proposition | Complexity | Platform | Notes |
|---|---------|-------------------|------------|----------|-------|
| 2 | **Docker snap installation detection** | Snap-installed Docker silently breaks `kind build` and some `kind create` operations due to TMPDIR sandboxing; minikube has a [similar fix](https://github.com/kubernetes/minikube/pull/10372) | LOW | Linux | Check if `docker` binary path contains `/snap/`; also check `snap list docker` |
| 7 | **AppArmor interference detection** | AppArmor profiles can silently break container operations; debugging requires knowing to check dmesg/journalctl | MEDIUM | Linux | Check `aa-status` for loaded profiles; detect if docker-default profile exists; warn if custom profiles may interfere |
| 8 | **Rootfs device node access check** | BTRFS/NVMe/LUKS users get cryptic kubelet "Failed to start ContainerManager" errors; Kind issue [#2411](https://github.com/kubernetes-sigs/kind/issues/2411) | HIGH | Linux | Detect filesystem type on Docker data-root; if BTRFS/device-mapper, warn about potential `extraMounts` requirement |
| 9 | **Firewalld nftables backend detection** | Fedora 32+ defaults to nftables backend which breaks Docker container networking; nodes cannot reach each other | LOW | Linux (Fedora) | Parse `/etc/firewalld/firewalld.conf` for `FirewallBackend=nftables`; provide iptables switch guidance |
| 10 | **SELinux enforcing mode detection** | Fedora 33 SELinux policy blocks /dev/dma_heap access; Docker container creation fails with permission denied | LOW | Linux (Fedora) | Run `getenforce` or use `go-selinux` library; warn if enforcing on Fedora 33 specifically |
| 11 | **Old kernel / cgroup namespace check** | RHEL 7 and clones with kernel <4.6 lack cgroup namespace support required by Kind 0.20+ | LOW | Linux | Parse `uname -r`; warn if kernel < 4.6; check `/proc/cgroups` for namespace support |
| 12 | **WSL2 cgroup misconfiguration detection** | WSL2 cgroup v1/v2 hybrid mode breaks container cgroup assignment; error is "error adding pid to cgroups" | MEDIUM | Linux (WSL2) | Detect WSL2 via `/proc/version` containing "microsoft"; check cgroup v2 controller availability at `/sys/fs/cgroup/cgroup.controllers` |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Auto-fix all detected issues** | "Just fix it for me" feels like a premium experience | Many fixes require root privileges, modify system config files, or have side effects (e.g., changing firewalld backend affects all networking). Auto-fixing Docker daemon.json or sysctl values without consent is dangerous | Provide copy-pasteable fix commands in the warning message; let users decide. Only auto-fix within `kinder create cluster` for non-destructive, scoped mitigations |
| **Block cluster creation on any warning** | "Prevent broken clusters" | Many warnings are platform-specific edge cases that may not actually cause failure (e.g., SELinux enforcing works fine on Fedora 34+; AppArmor is usually fine). Blocking on warnings creates false-positive friction | Use `warn` for likely-but-uncertain issues, `fail` only for definite blockers. Let `kinder create cluster` warn and continue; users can run `kinder doctor` to investigate pre-creation |
| **Network-dependent checks** (pull test images, DNS resolution checks) | "Verify full connectivity" | Adds latency to doctor command; may fail behind corporate proxies/firewalls creating false failures; kinder doctor should be fast and offline-capable | Keep doctor offline-only; network checks belong in cluster post-creation validation, not preflight |
| **Windows container mode detection** | "Detect wrong Docker mode on Windows" | Kind's own provider layer already validates this; kinder should not duplicate provider-level validation | Rely on existing provider validation in `validateProvider()` which already checks rootless/cgroup requirements |
| **Memory check** (Docker VM memory allocation) | "Detect insufficient RAM for node builds" | Only affects `kind build node-image` (not `kinder create cluster`); Docker Desktop memory is configurable via GUI/settings.json, not easily detectable cross-platform | Document the 8GB requirement for node image builds; do not add a check that only applies to one subcommand |

---

## Detailed Check Specifications

### Check 1: Kubectl Version Skew Detection

**What it detects:** Client kubectl version more than 1 minor version away from the Kind node image's Kubernetes version.

**Detection method:**
1. Run `kubectl version --client -o json` to get client version
2. Extract the Kind node image's Kubernetes version from the image tag (e.g., `kindest/node:v1.31.0` means K8s 1.31)
3. Compare minor versions: `abs(clientMinor - serverMinor) > 1` is a problem

**Statuses:**
- `ok`: "kubectl v1.31.2 (compatible with node image K8s v1.31.0)"
- `warn`: "kubectl v1.28.4 is 3 minor versions behind node image K8s v1.31.0 -- schema errors likely. Install matching version: https://kubernetes.io/docs/tasks/tools/"
- `fail`: N/A (version skew causes subtle errors, not hard failures; `warn` is appropriate)

**Auto-fixable:** No. Installing/upgrading kubectl is a user decision (brew, apt, manual download).

**Integration with create cluster:** Warn before provisioning if kubectl is detected and skewed. Do not block creation.

**Complexity:** MEDIUM -- requires semver parsing; must handle missing kubectl (already checked), missing `-o json` flag on old kubectl versions, and the case where no cluster exists yet (use node image tag, not live API server).

---

### Check 2: Docker Snap Installation Detection

**What it detects:** Docker installed via snap package manager, which sandboxes TMPDIR access.

**Detection method:**
1. Resolve `docker` binary path via `osexec.LookPath("docker")`
2. Check if path contains `/snap/` (e.g., `/snap/bin/docker` or `/snap/docker/current/bin/docker`)
3. Alternative: run `snap list docker 2>/dev/null` and check exit code

**Statuses:**
- `ok`: (no output -- snap not detected)
- `warn`: "Docker appears to be installed via snap, which restricts TMPDIR access. Set TMPDIR to a directory under $HOME: export TMPDIR=~/tmp && mkdir -p $TMPDIR"
- `fail`: N/A (snap Docker works for basic operations; only `kind build` is affected)

**Auto-fixable:** No at the system level. Kinder could set `TMPDIR` internally for its own operations, but this does not fix the underlying snap confinement issue.

**Integration with create cluster:** Warn once at the start. No mitigation needed during cluster creation (TMPDIR issues primarily affect `kind build`, not `kind create`).

**Complexity:** LOW -- path string check.

---

### Check 3: Disk Space Check

**What it detects:** Insufficient free disk space in Docker's data directory, which causes kubelet eviction and silent cluster failures.

**Detection method:**
1. Get Docker data-root from `docker info --format '{{.DockerRootDir}}'` (typically `/var/lib/docker`)
2. Check free space on that partition via `syscall.Statfs` (Go stdlib) or `df` command
3. Thresholds: warn <5GB free, fail <2GB free

**Statuses:**
- `ok`: "disk space: 45.2 GB free on /var/lib/docker"
- `warn`: "disk space: 3.1 GB free on /var/lib/docker -- cluster may experience eviction. Run: docker system prune"
- `fail`: "disk space: 1.2 GB free on /var/lib/docker -- insufficient for cluster creation. Run: docker system prune && docker image prune -a"

**Auto-fixable:** No. `docker system prune` is destructive (removes stopped containers, unused networks, dangling images). Must be explicit user action.

**Integration with create cluster:** Check before `p.Provision()`. Emit warning but do not block -- the user may know their disk usage is recoverable.

**Complexity:** LOW -- `syscall.Statfs` is cross-platform in Go; Docker data-root detection is one command.

---

### Check 4: Inotify Limits Check

**What it detects:** Linux inotify resource limits too low for Kind multi-node clusters, causing "too many open files" errors.

**Detection method:**
1. Read `/proc/sys/fs/inotify/max_user_watches` (default: 8192 on Ubuntu)
2. Read `/proc/sys/fs/inotify/max_user_instances` (default: 128 on Ubuntu)
3. Thresholds: warn if watches < 65536 or instances < 512; these values match Kind's documented recommendations

**Statuses:**
- `ok`: "inotify: max_user_watches=524288, max_user_instances=512"
- `warn`: "inotify: max_user_watches=8192 (recommended: 65536+). Fix: sudo sysctl fs.inotify.max_user_watches=524288. Persist: echo 'fs.inotify.max_user_watches=524288' | sudo tee -a /etc/sysctl.conf"
- `fail`: N/A (low limits cause intermittent failures, not guaranteed failure; `warn` is appropriate)

**Auto-fixable:** YES (candidate for auto-mitigation during `kinder create cluster`). The sysctl change is non-destructive, immediately effective, and reverts on reboot if not persisted. However, it requires root. Recommendation: auto-fix only if kinder is running as root; otherwise provide the command.

**Integration with create cluster:** If running as root and limits are low, auto-apply `sysctl -w` before provisioning. Log the change. If not root, warn with fix command.

**Complexity:** LOW -- file reads from procfs.

---

### Check 5: Docker Socket Permission Check

**What it detects:** Current user cannot access the Docker daemon socket, typically because they are not in the `docker` group.

**Detection method:**
1. Attempt `docker info` and check for "permission denied" in stderr
2. Alternative: check if `/var/run/docker.sock` exists and if current user's groups include `docker` (via `os.Getgroups()` + group lookup)

**Statuses:**
- `ok`: (already covered by existing container-runtime check -- if `docker info` works, permissions are fine)
- `warn`: N/A
- `fail`: "Docker permission denied -- your user is not in the 'docker' group. Fix: sudo usermod -aG docker $USER && newgrp docker"

**Auto-fixable:** No. Adding user to docker group requires root and a new login session.

**Integration with create cluster:** This is already implicitly checked by the existing container-runtime check. The enhancement is providing a specific, actionable message when the failure mode is permission-related rather than "docker not installed."

**Complexity:** LOW -- parse error output from existing `docker info` call. This is an enhancement to check #1 (container runtime), not a new independent check.

---

### Check 6: Docker daemon.json "init: true" Detection

**What it detects:** Docker daemon configured with `"init": true` which injects tini as PID 1 in all containers, breaking Kind node containers that expect systemd.

**Detection method:**
1. Read `/etc/docker/daemon.json` (Linux) or Docker Desktop settings
2. Parse JSON; check for `"init": true` at top level
3. On macOS: check `~/.docker/daemon.json` or Docker Desktop's `settings.json`

**Statuses:**
- `ok`: (no output if init is not set or is false)
- `warn`: "Docker daemon.json has 'init: true' which breaks Kind node containers. Error you'll see: 'Couldn't find an alternative telinit implementation to spawn'. Fix: remove '\"init\": true' from /etc/docker/daemon.json and restart Docker"
- `fail`: N/A (some users may have init:true intentionally for non-Kind workloads; `warn` gives them the information without blocking)

**Auto-fixable:** No. Modifying Docker daemon configuration is a system-level change that affects all containers.

**Integration with create cluster:** Check before provisioning. Emit warning. Do not modify daemon.json.

**Complexity:** LOW -- JSON file read and parse.

---

### Check 7: AppArmor Interference Detection

**What it detects:** AppArmor in enforcing mode with profiles that may interfere with Kind container operations.

**Detection method:**
1. Check if AppArmor is enabled: `aa-status --enabled` (exit code 0 = enabled)
2. If enabled, check if docker-default profile is loaded (this is normal and expected)
3. Check for custom profiles that may restrict mount/network operations inside containers

**Statuses:**
- `ok`: "AppArmor: enabled with docker-default profile (normal)"
- `warn`: "AppArmor is active. If cluster creation fails with mount/permission errors, try: sudo aa-teardown (temporary) or add Kind-specific AppArmor profile exceptions"
- `fail`: N/A (AppArmor usually works fine with Kind; interference is rare but hard to diagnose)

**Auto-fixable:** No. Disabling AppArmor profiles is a security decision.

**Integration with create cluster:** Include in doctor output on Linux. No automatic mitigation.

**Complexity:** MEDIUM -- `aa-status` requires root to get profile details; non-root can only check if AppArmor is enabled. The check should degrade gracefully when `aa-status` requires privileges.

---

### Check 8: Rootfs Device Node Access Check

**What it detects:** Filesystem types (BTRFS, device-mapper, LUKS/encrypted) where kubelet cannot find the expected device node inside Kind containers, causing "Failed to start ContainerManager" errors.

**Detection method:**
1. Get Docker data-root filesystem type: `stat -f -c %T /var/lib/docker` or `df -T /var/lib/docker`
2. If BTRFS: warn about potential device node issue
3. If device-mapper or LUKS: warn about encrypted filesystem complications
4. Check if `/dev/` contains the expected block device for the Docker data-root partition

**Statuses:**
- `ok`: "rootfs: ext4 on /dev/sda1 (standard, no known issues)"
- `warn`: "rootfs: btrfs detected on Docker data-root. Kind may fail with 'Failed to get rootfs info'. You may need to add an extraMounts entry for your block device. See: https://kind.sigs.k8s.io/docs/user/known-issues/#rootfs-device-node-access"
- `fail`: N/A (BTRFS works in many configurations; only certain setups trigger the issue)

**Auto-fixable:** No. The correct `extraMounts` device path varies per host.

**Integration with create cluster:** Warn before provisioning. Cannot auto-fix because the correct device path is host-specific.

**Complexity:** HIGH -- filesystem type detection is reliable, but determining whether the specific BTRFS/NVMe configuration will actually trigger the kubelet issue requires deeper analysis of device major/minor numbers.

---

### Check 9: Firewalld nftables Backend Detection

**What it detects:** Firewalld configured with nftables backend on Fedora 32+, which breaks Docker container networking.

**Detection method:**
1. Check if `firewalld` is running: `systemctl is-active firewalld`
2. If active, check backend: parse `/etc/firewalld/firewalld.conf` for `FirewallBackend=nftables`
3. Alternative: `firewall-cmd --state` + `journalctl -u firewalld | grep "Using backend"`

**Statuses:**
- `ok`: "firewalld: using iptables backend" or "firewalld: not active"
- `warn`: "firewalld: nftables backend detected. Docker container networking may fail. Fix: change FirewallBackend=iptables in /etc/firewalld/firewalld.conf and run: sudo systemctl restart firewalld"
- `fail`: N/A (newer Docker versions have improving nftables support; `warn` is appropriate)

**Auto-fixable:** No. Changing firewall backend affects all system networking.

**Integration with create cluster:** Check on Linux only. Warn before provisioning.

**Complexity:** LOW -- config file parse.

---

### Check 10: SELinux Enforcing Mode Detection

**What it detects:** SELinux in enforcing mode on Fedora 33, which blocks /dev/dma_heap access during Docker container creation.

**Detection method:**
1. Run `getenforce` -- returns "Enforcing", "Permissive", or "Disabled"
2. If enforcing, check OS version via `/etc/os-release` for Fedora 33 specifically
3. Alternative: use `github.com/opencontainers/selinux/go-selinux` library (vendored by Kubernetes itself)

**Statuses:**
- `ok`: "SELinux: permissive" or "SELinux: disabled" or "SELinux: enforcing (Fedora 34+ -- no known issues)"
- `warn`: "SELinux: enforcing on Fedora 33. Docker may fail with '/dev/dma_heap: permission denied'. Temporary fix: sudo setenforce 0. Permanent fix: upgrade to Fedora 34+"
- `fail`: N/A (SELinux enforcing is a security posture; warning is sufficient)

**Auto-fixable:** No. Changing SELinux mode is a security decision.

**Integration with create cluster:** Check on Linux only. Warn on Fedora 33 specifically. On other distros, SELinux enforcing is generally fine with Kind.

**Complexity:** LOW -- command execution + OS detection.

---

### Check 11: Old Kernel / Cgroup Namespace Support Check

**What it detects:** Linux kernels older than 4.6 that lack cgroup namespace support, which Kind 0.20+ requires.

**Detection method:**
1. Parse kernel version from `uname -r` (or Go's `syscall.Uname`)
2. Compare major.minor against 4.6 threshold
3. Additional: check `/proc/self/ns/cgroup` existence as direct cgroup namespace support indicator

**Statuses:**
- `ok`: "kernel: 5.15.0-91 (cgroup namespaces supported)"
- `warn`: N/A
- `fail`: "kernel: 3.10.0-1160 (cgroup namespaces require kernel 4.6+). Kind cannot create clusters on this kernel. Upgrade your OS or use a VM with a newer kernel"

**Auto-fixable:** No. Kernel upgrades are OS-level operations.

**Integration with create cluster:** Check before provisioning. This is a hard blocker -- `fail` status.

**Complexity:** LOW -- version string parsing.

---

### Check 12: WSL2 Cgroup Misconfiguration Detection

**What it detects:** WSL2 environments with cgroup v1/v2 hybrid mode that prevents container cgroup assignment.

**Detection method:**
1. Detect WSL2: check `/proc/version` for "microsoft" or "Microsoft" string
2. If WSL2, check cgroup v2 controller availability: read `/sys/fs/cgroup/cgroup.controllers`
3. Verify essential controllers are present: cpu, memory, io, pids
4. Check WSL version (2.5.1+ has cgroupsv2 by default)

**Statuses:**
- `ok`: "WSL2: cgroup v2 controllers available (cpu memory io pids)"
- `warn`: "WSL2: cgroup v2 controllers missing (cpu memory). Cluster creation may fail with 'error adding pid to cgroups'. Fix: add 'kernelCommandLine=cgroup_no_v1=all' to ~/.wslconfig under [wsl2], then run: wsl --shutdown. See: https://github.com/spurin/wsl-cgroupsv2"
- `fail`: N/A (WSL2 5.1+ usually works; `warn` for older WSL versions)

**Auto-fixable:** No. Modifying .wslconfig and restarting WSL requires user action outside of the Linux environment.

**Integration with create cluster:** Check on WSL2 Linux only. Warn before provisioning.

**Complexity:** MEDIUM -- WSL2 detection is reliable; cgroup controller availability parsing requires reading procfs.

---

### Check 13: Docker Subnet Clash Detection

**What it detects:** Docker's default bridge subnet (172.17.0.0/16) or custom subnets conflicting with host network routes (VPN, corporate LAN).

**Detection method:**
1. Get Docker bridge network CIDR: `docker network inspect bridge --format '{{range .IPAM.Config}}{{.Subnet}}{{end}}'`
2. Get host routing table: `ip route show` (Linux) or `netstat -rn` (macOS)
3. Check for overlap between Docker subnet and any host route entries in 172.16.0.0/12 or 10.0.0.0/8 ranges

**Statuses:**
- `ok`: "Docker bridge: 172.17.0.0/16 (no conflicts with host routes)"
- `warn`: "Docker bridge subnet 172.17.0.0/16 overlaps with host route 172.16.0.0/12 (likely VPN). Cluster nodes may have network routing issues. Fix: configure custom Docker address pools in /etc/docker/daemon.json. See: https://kind.sigs.k8s.io/docs/user/known-issues/#docker-subnet-clash"
- `fail`: N/A (subnet clashes cause routing issues, not hard failures; `warn` is appropriate)

**Auto-fixable:** No. Modifying Docker daemon.json address pools affects all Docker networking.

**Integration with create cluster:** Check before provisioning. Emit warning with documentation link.

**Complexity:** MEDIUM -- subnet overlap calculation requires CIDR parsing; cross-platform route table parsing differs between Linux and macOS.

---

## Feature Dependencies

```
Existing doctor infrastructure
    provides──> result struct, ok/warn/fail formatting, JSON output, exit codes
    provides──> osexec.LookPath pattern, exec.OutputLines pattern
    provides──> runtime.GOOS platform gating pattern
    all 13 checks depend on this

Check 5 (Docker socket permission)
    enhances──> Existing container-runtime check (better error messages)
    not a new independent check -- refinement of existing logic

Check 1 (kubectl version skew)
    enhances──> Existing kubectl check (adds version comparison)
    depends on──> kubectl check passing (binary must exist first)

Check 3 (disk space)
    depends on──> Container runtime check passing (needs docker info)

Check 6 (daemon.json init:true)
    depends on──> Container runtime being Docker (not podman/nerdctl)

Check 9 (firewalld nftables)
    platform-specific──> Linux + Fedora only
    depends on──> firewalld being installed

Check 10 (SELinux enforcing)
    platform-specific──> Linux + Fedora 33 only
    depends on──> getenforce binary or go-selinux library

Check 12 (WSL2 cgroups)
    platform-specific──> Linux under WSL2 only
    depends on──> WSL2 detection via /proc/version

Check 4 (inotify limits)
    auto-mitigation-candidate──> Can be auto-fixed during kinder create cluster
    platform-specific──> Linux only

Check 7 (AppArmor) ──conflicts──> Check 10 (SELinux)
    Both are MAC systems; a host runs one or the other, never both
```

### Dependency Notes

- **Check 5 is an enhancement, not a new check:** The existing container-runtime check already runs `docker info`. The enhancement adds a specific "permission denied" error path with actionable guidance. This should be implemented as a refinement of `checkBinary()`, not a separate check function.
- **Platform-specific checks must be gated:** Checks 2, 4, 5, 7, 8, 9, 10, 11, 12 are Linux-only. Checks 9 and 10 are further scoped to Fedora. Check 12 is scoped to WSL2. These use the existing `runtime.GOOS == "linux"` pattern, plus additional OS detection.
- **AppArmor and SELinux are mutually exclusive:** A system runs AppArmor (Ubuntu/Debian) OR SELinux (RHEL/Fedora), never both. The doctor should detect which MAC system is present and only run the relevant check.
- **Auto-mitigation is limited to inotify (check 4):** This is the only check where automatic fix is non-destructive, immediately effective, and scoped. All other fixes modify system configuration or require user judgment.

---

## MVP Definition

### Launch With (v1 -- initial milestone)

All 13 checks in `kinder doctor`:

- [ ] **Check 1: Kubectl version skew** -- most common user confusion after Docker Desktop updates
- [ ] **Check 2: Docker snap detection** -- simple path check, prevents TMPDIR debugging
- [ ] **Check 3: Disk space** -- catches the #1 silent failure mode
- [ ] **Check 4: Inotify limits** -- prevents "too many open files" on Linux
- [ ] **Check 5: Docker socket permissions** -- enhances existing container-runtime check
- [ ] **Check 6: Docker init:true detection** -- catches the most cryptic error message
- [ ] **Check 7: AppArmor interference** -- Linux desktop users frequently hit this
- [ ] **Check 8: Rootfs device node** -- BTRFS/NVMe users need advance warning
- [ ] **Check 9: Firewalld nftables** -- Fedora users need this detected early
- [ ] **Check 10: SELinux enforcing** -- Fedora 33 specific; simple check
- [ ] **Check 11: Old kernel check** -- hard blocker detection for RHEL 7
- [ ] **Check 12: WSL2 cgroup check** -- Windows developers using WSL2 need this
- [ ] **Check 13: Subnet clash** -- enterprise/VPN users need advance warning

Automatic mitigations during `kinder create cluster`:

- [ ] **Inotify auto-fix** -- If limits are below threshold and process has CAP_SYS_ADMIN, apply sysctl fix before provisioning
- [ ] **Pre-creation doctor subset** -- Run critical checks (disk space, kernel version, docker init:true) before `p.Provision()` and warn

### Add After Validation (v1.x)

- [ ] **`kinder doctor --check <name>`** -- Run a specific subset of checks instead of all
- [ ] **`kinder doctor --fix`** -- Apply all safe auto-fixes (inotify, TMPDIR env) with user confirmation
- [ ] **Check result caching** -- Cache results for 5 minutes to avoid re-running slow checks during `kinder create cluster`
- [ ] **Remediation scripts** -- Generate a shell script with all fix commands: `kinder doctor --remediate > fix.sh`

### Future Consideration (v2+)

- [ ] **Post-creation health checks** -- Verify cluster health after creation (pod scheduling, DNS, CoreDNS)
- [ ] **Plugin-based check system** -- Allow users to add custom checks via Go plugins or YAML-defined checks
- [ ] **CI mode** -- Machine-readable output optimized for CI pipelines with JUnit XML format

---

## Feature Prioritization Matrix

| Check | User Value | Implementation Cost | Priority | Platform |
|-------|------------|---------------------|----------|----------|
| Disk space (3) | HIGH | LOW | P1 | All |
| Docker init:true (6) | HIGH | LOW | P1 | All |
| Kubectl version skew (1) | HIGH | MEDIUM | P1 | All |
| Docker socket permissions (5) | HIGH | LOW | P1 | Linux |
| Inotify limits (4) | HIGH | LOW | P1 | Linux |
| Docker subnet clash (13) | HIGH | MEDIUM | P1 | All |
| Old kernel check (11) | HIGH | LOW | P1 | Linux |
| WSL2 cgroup check (12) | HIGH | MEDIUM | P1 | WSL2 |
| Docker snap detection (2) | MEDIUM | LOW | P1 | Linux |
| Firewalld nftables (9) | MEDIUM | LOW | P1 | Fedora |
| SELinux enforcing (10) | MEDIUM | LOW | P1 | Fedora |
| AppArmor interference (7) | MEDIUM | MEDIUM | P1 | Linux |
| Rootfs device node (8) | MEDIUM | HIGH | P1 | Linux |
| Inotify auto-mitigation | MEDIUM | LOW | P1 | Linux |
| Pre-creation doctor subset | MEDIUM | MEDIUM | P1 | All |

**Priority key:**
- P1: All checks ship in this milestone (they are the milestone)
- P2: Post-milestone enhancements (--check flag, --fix flag)
- P3: Future capabilities (plugins, CI mode)

---

## Implementation Patterns

### Pattern: Check Function Signature

Follow the existing pattern in doctor.go. Each check is a function returning `result` or `[]result`:

```go
// Single result check
func checkDiskSpace() result { ... }

// Multi-result check (like checkNvidiaDriver which returns []result)
func checkInotifyLimits() []result { ... }
```

### Pattern: Platform Gating

Group platform-specific checks under `runtime.GOOS` guards, matching the existing NVIDIA pattern:

```go
if runtime.GOOS == "linux" {
    results = append(results, checkInotifyLimits()...)
    results = append(results, checkDockerSnap())
    results = append(results, checkDockerSocketPermissions())

    // MAC system checks (mutually exclusive)
    results = append(results, checkMACSystem()...) // detects AppArmor or SELinux

    // Distro-specific
    if isFedora() {
        results = append(results, checkFirewalldBackend())
        results = append(results, checkSELinuxEnforcing())
    }

    // WSL2-specific
    if isWSL2() {
        results = append(results, checkWSL2Cgroups())
    }

    results = append(results, checkKernelVersion())
    results = append(results, checkRootfsDevice())
}

// Cross-platform checks
results = append(results, checkDiskSpace())
results = append(results, checkDockerInit())
results = append(results, checkSubnetClash())
results = append(results, checkKubectlVersionSkew()...)
```

### Pattern: Create-time Mitigation

Mitigations during `kinder create cluster` should be a lightweight subset, not the full doctor:

```go
// In create.go, before p.Provision():
func runPreflightChecks(logger log.Logger, config *config.Cluster) {
    // Only checks that could prevent successful cluster creation
    if r := checkDiskSpace(); r.status == "fail" {
        logger.Warnf("[preflight] %s: %s", r.name, r.message)
    }
    if runtime.GOOS == "linux" {
        if r := checkInotifyLimits(); needsFix(r) {
            applyInotifyFix(logger) // auto-fix if possible
        }
    }
    // ... other critical checks
}
```

### Pattern: Testability

Following the established `currentOS` pattern from the NVIDIA addon, make system-level calls injectable:

```go
// Package-level vars for test injection
var (
    readFile    = os.ReadFile
    lookPath    = osexec.LookPath
    commandExec = exec.OutputLines
)
```

---

## Competitor Feature Analysis

| Feature | kind (upstream) | minikube | k3d | kinder (current) | kinder (this milestone) |
|---------|----------------|----------|-----|-------------------|------------------------|
| Preflight checks | None (relies on kubeadm's built-in checks inside the node) | Basic driver checks, snap detection ([PR #10372](https://github.com/kubernetes/minikube/pull/10372)) | None | docker/kubectl/GPU checks | 13+ checks covering all Known Issues |
| Disk space warning | None (kubelet evicts silently) | Yes (driver-level check) | None | None | Yes, with thresholds |
| Subnet clash detection | None | None | None | None | Yes, with route table comparison |
| Auto-fix capabilities | None | Driver-specific fixes | None | None | Inotify auto-fix |
| Structured output | None | JSON for some commands | None | JSON mode (`--output json`) | JSON mode with all new checks |
| Exit codes | Unstructured | Unstructured | Unstructured | Structured (0/1/2) | Structured (0/1/2) |

**Kinder's advantage:** No Kind-based tool provides comprehensive preflight diagnostics. Upstream Kind's known issues page is documentation-only -- users must read it after hitting problems. Kinder detects these issues before they cause failures, with actionable fix commands.

---

## Sources

- Kind Known Issues page: https://kind.sigs.k8s.io/docs/user/known-issues/ (HIGH confidence -- primary source for all 13 checks)
- Kubernetes Version Skew Policy: https://kubernetes.io/releases/version-skew-policy/ (HIGH confidence -- official policy)
- kubeadm preflight checks implementation: https://github.com/kubernetes/kubernetes/blob/master/cmd/kubeadm/app/preflight/checks.go (HIGH confidence -- pattern reference)
- kubeadm preflight package docs: https://pkg.go.dev/k8s.io/kubernetes/cmd/kubeadm/app/preflight (HIGH confidence -- interface definition)
- Replicated Troubleshoot framework: https://github.com/replicatedhq/troubleshoot (MEDIUM confidence -- pattern reference for ok/warn/fail)
- Kind disk space issue #2717: https://github.com/kubernetes-sigs/kind/issues/2717 (HIGH confidence)
- Kind disk space issue #3313: https://github.com/kubernetes-sigs/kind/issues/3313 (HIGH confidence)
- Kind Docker snap issue #431: https://github.com/kubernetes-sigs/kind/issues/431 (HIGH confidence)
- minikube snap detection PR #10372: https://github.com/kubernetes/minikube/pull/10372 (HIGH confidence)
- Kind BTRFS device node issue #2411: https://github.com/kubernetes-sigs/kind/issues/2411 (HIGH confidence)
- Kind RHEL 7 kernel issue #3311: https://github.com/kubernetes-sigs/kind/issues/3311 (HIGH confidence)
- Kind WSL2 cgroup issue #3685: https://github.com/kubernetes-sigs/kind/issues/3685 (HIGH confidence)
- WSL2 cgroupsv2 fix guide: https://github.com/spurin/wsl-cgroupsv2 (MEDIUM confidence -- community solution)
- OpenContainers go-selinux library: https://pkg.go.dev/github.com/opencontainers/selinux/go-selinux (HIGH confidence)
- Fedora firewalld nftables change: https://fedoraproject.org/wiki/Changes/firewalld_default_to_nftables (HIGH confidence)
- Docker nftables documentation: https://docs.docker.com/engine/network/firewall-nftables/ (HIGH confidence)
- Kubernetes inotify issue #46230: https://github.com/kubernetes/kubernetes/issues/46230 (HIGH confidence)
- Kinder codebase: `pkg/cmd/kind/doctor/doctor.go`, `pkg/cluster/internal/create/create.go` (HIGH confidence -- direct code reading)

---
*Feature research for: kinder diagnostic checks and automatic mitigations milestone*
*Researched: 2026-03-06*
