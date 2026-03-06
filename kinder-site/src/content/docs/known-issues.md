---
title: Known Issues
description: Every diagnostic check kinder doctor runs, what it detects, why it matters, and how to fix it.
---

`kinder doctor` runs **18 diagnostic checks** across **8 categories** to detect common environment problems before they cause cryptic cluster creation failures. This page documents every check: what it detects, why it matters, and how to fix it.

For exit codes, CI usage, and common error scenarios, see the [Troubleshooting](/cli-reference/troubleshooting/) page.

## How to run diagnostics

```sh
kinder doctor
```

Use `--output json` for machine-readable output (useful in CI pipelines):

```sh
kinder doctor --output json
```

### Exit codes

| Exit code | Meaning |
|-----------|---------|
| 0 | All checks passed (or skipped) |
| 1 | One or more checks failed |
| 2 | One or more checks warned (no failures) |

If both failures and warnings exist, exit code 1 takes priority.

---

## Runtime

### Container Runtime

**What it detects:** Whether a container runtime (Docker, Podman, or nerdctl) is installed and responding. Checks in order: docker, podman, nerdctl -- stops at the first binary found.

**Why it matters:** A working container runtime is required for cluster creation. Without one, `kinder create cluster` cannot start.

**Platforms:** All

**How to fix:**

If no runtime is found, install one:

```sh
# Docker (recommended)
# https://docs.docker.com/get-docker/

# Podman
# https://podman.io/getting-started/installation

# nerdctl
# https://github.com/containerd/nerdctl
```

If a runtime is found but not responding, start the daemon:

```sh
# Linux
sudo systemctl start docker

# macOS / Windows
# Open Docker Desktop
```

---

## Docker

### Disk Space

**What it detects:** Available disk space on the Docker data root directory. Warns below 5 GB, fails below 2 GB.

**Why it matters:** Docker images and container layers require significant disk space. Running out mid-creation causes corrupted images and cryptic errors.

**Platforms:** All

**How to fix:**

```sh
# Remove unused containers, networks, and dangling images
docker system prune

# Also remove unused images
docker image prune -a
```

### Daemon JSON Init Config

**What it detects:** Whether any `daemon.json` file has `"init": true` set. Searches six candidate locations: `/etc/docker/daemon.json`, `~/.docker/daemon.json`, `$XDG_CONFIG_HOME/docker/daemon.json`, `/var/snap/docker/current/config/daemon.json`, `~/.rd/docker/daemon.json`, and `C:\ProgramData\docker\config\daemon.json` (Windows).

**Why it matters:** Docker's `init` option injects `tini` as PID 1 in every container. kind nodes expect to run their own init system, so `"init": true` breaks node startup with: `Couldn't find an alternative telinit implementation to spawn`.

**Platforms:** All

**How to fix:**

Remove `"init": true` from your `daemon.json` and restart Docker:

```json
{
  "init": false
}
```

```sh
sudo systemctl restart docker
```

See [kind known issues: Docker init](https://kind.sigs.k8s.io/docs/user/known-issues/#docker-init) for details.

### Docker Snap

**What it detects:** Whether Docker is installed via snap by resolving the `docker` binary path through symlinks and checking for `/snap/` in the resolved path.

**Why it matters:** Snap-confined Docker uses a restricted `TMPDIR` that prevents kind from writing temporary files during cluster creation.

**Platforms:** Linux only

**How to fix:**

Option A -- set a snap-accessible TMPDIR:

```sh
export TMPDIR=$HOME/tmp
mkdir -p $TMPDIR
```

Option B -- reinstall Docker via apt (recommended):

```sh
sudo snap remove docker
# Follow: https://docs.docker.com/engine/install/ubuntu/
```

### Docker Socket Permissions

**What it detects:** Whether `docker info` fails with "permission denied," indicating the current user cannot access the Docker socket.

**Why it matters:** Without socket access, no Docker commands can run, including cluster creation.

**Platforms:** Linux only

**How to fix:**

```sh
sudo usermod -aG docker $USER
newgrp docker
```

:::note
On macOS and Windows, Docker Desktop manages socket permissions automatically. This check only runs on Linux.
:::

---

## Tools

### kubectl

**What it detects:** Whether the `kubectl` binary is installed and responds to `kubectl version --client`.

**Why it matters:** kubectl is required to interact with the cluster after creation. Without it, you cannot deploy workloads or inspect cluster state.

**Platforms:** All

**How to fix:**

Install kubectl: https://kubernetes.io/docs/tasks/tools/

### kubectl Version Skew

**What it detects:** Whether the installed kubectl client version is more than one minor version away from the Kubernetes version used by the default kind node image.

**Why it matters:** The Kubernetes version skew policy allows kubectl to be within +/-1 minor version of the cluster. Larger skew can cause API incompatibilities, missing fields, or unexpected behavior.

**Platforms:** All

**How to fix:**

Update kubectl to match your cluster version: https://kubernetes.io/docs/tasks/tools/

---

## GPU

:::note
GPU checks are informational. They only matter if you plan to use the NVIDIA GPU addon (`nvidiaGPU: true` in your kinder config). All three checks are Linux-only.
:::

### NVIDIA Driver

**What it detects:** Whether `nvidia-smi` is installed and can query the driver version.

**Why it matters:** The NVIDIA driver is the foundation for GPU container support. Without it, the GPU device plugin cannot access GPUs.

**Platforms:** Linux only

**How to fix:**

Install NVIDIA drivers: https://www.nvidia.com/drivers

Verify installation:

```sh
nvidia-smi
```

### NVIDIA Container Toolkit

**What it detects:** Whether `nvidia-ctk` (NVIDIA Container Toolkit CLI) is available on PATH.

**Why it matters:** The container toolkit provides the runtime hooks that expose GPU devices inside containers.

**Platforms:** Linux only

**How to fix:**

```sh
sudo apt-get install -y nvidia-container-toolkit
```

See the [NVIDIA Container Toolkit install guide](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html) for other distributions.

### NVIDIA Docker Runtime

**What it detects:** Whether the `nvidia` runtime is registered in Docker's runtime configuration by querying `docker info`.

**Why it matters:** Docker must be configured to use the NVIDIA runtime so that GPU devices are passed through to kind node containers.

**Platforms:** Linux only

**How to fix:**

```sh
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
```

Verify the runtime is registered:

```sh
docker info | grep -i nvidia
```

---

## Kernel

:::note
Kernel checks only run on Linux. They are automatically skipped on macOS and Windows.
:::

### Inotify Limits

**What it detects:** The values of `fs.inotify.max_user_watches` and `fs.inotify.max_user_instances`. Warns if watches are below 524288 or instances are below 512.

**Why it matters:** kind nodes run multiple inotify watchers (kubelet, kube-apiserver, etcd). Low limits cause "too many open files" errors that crash cluster components.

**Platforms:** Linux only

**How to fix:**

```sh
sudo sysctl fs.inotify.max_user_watches=524288
sudo sysctl fs.inotify.max_user_instances=512
```

To make these changes persistent across reboots:

```sh
echo "fs.inotify.max_user_watches=524288" | sudo tee -a /etc/sysctl.d/99-kind.conf
echo "fs.inotify.max_user_instances=512" | sudo tee -a /etc/sysctl.d/99-kind.conf
sudo sysctl --system
```

See [kind known issues: inotify](https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files) for details.

### Kernel Version

**What it detects:** The running kernel version via `uname`. Fails if below 4.6.

**Why it matters:** Linux kernels before 4.6 lack cgroup namespace support, which kind requires to isolate container resources. This is a hard blocker -- kind cannot function on older kernels.

**Platforms:** Linux only

**How to fix:**

Upgrade your kernel to 4.6 or later. On Ubuntu:

```sh
sudo apt-get update && sudo apt-get dist-upgrade
```

---

## Security

:::note
AppArmor and SELinux are independent Linux Security Modules (LSMs). A system can have both active simultaneously (LSM stacking since kernel 5.1). These checks run independently.
:::

### AppArmor

**What it detects:** Whether the AppArmor kernel module is loaded and enabled by reading `/sys/module/apparmor/parameters/enabled`.

**Why it matters:** AppArmor can cache stale container profiles that prevent kind containers from starting. This is a known Docker issue ([moby/moby#7512](https://github.com/moby/moby/issues/7512)).

**Platforms:** Linux only

**How to fix:**

If kind containers fail to start with AppArmor-related errors:

```sh
sudo aa-remove-unknown
```

This removes stale AppArmor profiles without disabling AppArmor entirely.

### SELinux

**What it detects:** Whether SELinux is in enforcing mode by reading `/sys/fs/selinux/enforce`. Only warns on Fedora, where `/dev/dma_heap` permission denials are a known issue.

**Why it matters:** On Fedora, SELinux enforcing mode denies access to `/dev/dma_heap`, which breaks kind node containers.

**Platforms:** Linux only

**How to fix:**

Temporary (until reboot):

```sh
sudo setenforce 0
```

Persistent:

```sh
# Edit /etc/selinux/config
SELINUX=permissive
```

:::tip
On non-Fedora distributions (Ubuntu, Debian, CentOS), SELinux enforcing mode does not have known issues with kind. The check returns ok on those systems even when enforcing.
:::

---

## Platform

### Firewalld

**What it detects:** Whether firewalld is running with the nftables backend by checking `firewall-cmd --state` and parsing `/etc/firewalld/firewalld.conf`. If the `FirewallBackend` line is absent, it defaults to nftables (the Fedora 32+ default).

**Why it matters:** The nftables backend bypasses iptables rules that Docker and kind rely on for container networking. This causes network connectivity failures between pods and nodes.

**Platforms:** Linux only

**How to fix:**

Edit `/etc/firewalld/firewalld.conf`:

```ini
FirewallBackend=iptables
```

Then restart firewalld:

```sh
sudo systemctl restart firewalld
```

See [kind known issues: firewalld](https://kind.sigs.k8s.io/docs/user/known-issues/#firewalld) for details.

### WSL2 Cgroup

**What it detects:** Whether the system is running under WSL2 using a multi-signal approach (requires `/proc/version` containing "microsoft" AND at least one of: `$WSL_DISTRO_NAME` set, `/proc/sys/fs/binfmt_misc/WSLInterop` exists, or `WSLInterop-late` exists). If WSL2 is confirmed, verifies that cgroup v2 controllers (cpu, memory, pids) are available.

**Why it matters:** kind nodes need cgroup v2 controllers for resource management. WSL2 does not enable cgroup v2 by default on older distributions, causing kubelet failures.

**Platforms:** Linux only (WSL2 runs a Linux kernel)

**How to fix:**

Create or edit `%UserProfile%\.wslconfig` in Windows:

```ini
[wsl2]
kernelCommandLine = cgroup_no_v1=all systemd.unified_cgroup_hierarchy=1
```

Then restart WSL:

```powershell
wsl --shutdown
```

See [WSL2 cgroup v2 setup](https://github.com/spurin/wsl-cgroupsv2) for a detailed guide.

:::note
The WSL2 detection uses two signals to avoid false positives on Azure VMs, which also have "microsoft" in `/proc/version` but are not WSL2 environments.
:::

### Rootfs Device

**What it detects:** Whether Docker is using the BTRFS storage driver or a BTRFS backing filesystem by querying `docker info` for both the `.Driver` field and `.DriverStatus` metadata.

**Why it matters:** BTRFS has known issues with Docker's overlay storage and device node management. kind containers may fail to create or run with device-related errors.

**Platforms:** Linux only

**How to fix:**

Option A -- use a different filesystem for Docker storage:

Dedicate an ext4 or XFS partition for `/var/lib/docker`.

Option B -- switch Docker storage driver to overlay2:

See [kind known issues: Docker on BTRFS](https://kind.sigs.k8s.io/docs/user/known-issues/#docker-on-btrfs) for configuration steps.

---

## Network

### Subnet Clashes

**What it detects:** Whether Docker network subnets (from the `kind` and `bridge` networks) overlap with host routing table entries. On Linux, parses `ip route show`; on macOS, parses `netstat -rn`. Self-referential routes (Docker's own routes in the host table) are excluded from comparison.

**Why it matters:** When Docker subnets overlap with VPN, corporate network, or other host routes, cluster nodes cannot reach external services and pods may have connectivity failures.

**Platforms:** All (route parsing is platform-specific)

**How to fix:**

1. Identify the conflicting network:

```sh
docker network inspect kind
```

2. Configure custom Docker address pools in `/etc/docker/daemon.json`:

```json
{
  "default-address-pools": [
    { "base": "10.200.0.0/16", "size": 24 }
  ]
}
```

3. Restart Docker and recreate the cluster:

```sh
sudo systemctl restart docker
kinder delete cluster
kinder create cluster
```

:::tip
VPN software is the most common cause of subnet clashes. If you see this warning, check whether your VPN uses the `172.17.0.0/16` or `172.18.0.0/16` ranges that Docker defaults to.
:::

---

## Automatic Mitigations

When you run `kinder create cluster`, the create flow calls `ApplySafeMitigations()` before provisioning nodes. This system can automatically apply safe fixes without any manual intervention.

### Tier-1 constraints

Automatic mitigations are limited to **tier-1** operations only:

- **Allowed:** Environment variable adjustments, cluster configuration tweaks
- **Never allowed:** Running `sudo`, modifying system files (`daemon.json`, `sysctl.conf`, `firewalld.conf`), or elevating privileges

This design ensures that kinder never makes system-level changes without explicit user action. When a problem is detected that requires system changes, `kinder doctor` documents the fix command for you to run manually.

### Current state

No tier-1 automatic mitigations currently exist. The infrastructure is fully wired and ready for future use -- when a safe, idempotent mitigation is identified, it can be added to the `SafeMitigations()` registry without any architectural changes.

All 18 checks documented on this page are **diagnostic only**. They report problems and suggest fixes but do not modify your system.
