---
title: Known Issues
description: Every diagnostic check kinder doctor runs, what it detects, why it matters, and how to fix it.
---

`kinder doctor` runs **24 diagnostic checks** across **9 categories** to detect common environment problems before they cause cryptic cluster creation failures. This page documents every check: what it detects, why it matters, and how to fix it.

:::tip[Decoding runtime errors]
For cryptic errors that surface AFTER cluster creation, use `kinder doctor decode` to scan recent docker logs and `kubectl get events` against a 16-pattern catalog (kubelet, kubeadm, containerd, docker, addon-startup) and print plain-English explanations with suggested fixes. Pass `--auto-fix` to apply whitelisted, non-destructive remediations.
:::

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

### Local Path CVE

**What it detects:** The deployed version of [local-path-provisioner](/addons/local-path-provisioner/) on a running cluster, by querying the deployed image tag. Warns when the version is below **v0.0.34** — the fix release for [CVE-2025-62878](https://nvd.nist.gov/vuln/detail/CVE-2025-62878) (path traversal allowing a malicious workload to write outside its PV).

**Why it matters:** A workload exploiting CVE-2025-62878 can read or write host filesystem paths outside its PersistentVolume, breaking the isolation guarantees of dynamic provisioning. The fix is straightforward (a version bump), but the vulnerability is silent — there is no functional symptom until exploitation.

**Platforms:** All. Skipped when local-path-provisioner is not installed (`addons.localPath: false`).

**Status values:**

- **ok** — local-path-provisioner is at v0.0.34 or higher (kinder ships v0.0.35+ pinned)
- **warn** — version below v0.0.34 detected
- **skip** — addon not installed, or unable to probe deployed version

**How to fix:**

If you're running the kinder default (`localPath: true`), simply create a fresh cluster — kinder pins the embedded manifest to a fix-version image. If you've overridden the manifest or are running a derived addon, bump the local-path-provisioner image tag to v0.0.34 or later.

---

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

### Host Mount Path

**What it detects:** Whether every host path declared in a cluster config's `extraMounts` exists and is accessible. The check reads `extraMounts` via `--config <file>`. Relative paths are resolved with `filepath.Abs` before `os.Stat`.

**Why it matters:** A non-existent host path causes `kinder create cluster` to fail with a cryptic Docker mount error mid-provisioning, after several other steps have already run. Catching the missing path at doctor-time saves minutes of false-start cluster creation.

**Platforms:** All. Skipped when `--config` is not passed or `extraMounts` is absent.

**Status values:**

- **ok** — host path exists and is accessible
- **fail** — host path does not exist on the filesystem
- **warn** — host path exists but cannot be read (permission error)

**How to fix:**

Create the directory before invoking `kinder create cluster`:

```sh
mkdir -p /Users/me/work/myapp-data
kinder create cluster --config cluster.yaml
```

For relative paths, remember that they resolve against your current working directory, not the config file's directory.

---

### Docker Desktop File Sharing

**What it detects:** On macOS, whether every host path in `extraMounts` is within Docker Desktop's configured file-sharing roots. Reads `~/Library/Group Containers/group.com.docker/settings-store.json` to extract the shared roots; falls back to Docker Desktop default sharing roots when `settings-store.json` is absent.

**Why it matters:** Docker Desktop on macOS only mounts paths that are under approved file-sharing roots. Paths outside the shared roots silently fail to mount — the container starts, but the bind mount is empty. The pod sees an empty directory instead of your expected files, causing confusing application-level errors.

**Platforms:** macOS only. Skipped on Linux and Windows.

**Status values:**

- **ok** — every configured `extraMount` host path is under a shared root
- **warn** — one or more `extraMount` paths are outside the shared roots

**How to fix:**

Open Docker Desktop &rarr; Settings &rarr; Resources &rarr; File Sharing, add the parent directory containing your mount path, and apply &amp; restart Docker Desktop. Or move your mount source under an already-shared root (e.g., `~/`).

See the [Host Directory Mounting guide](/guides/host-directory-mounting/) for the end-to-end pattern.

---

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

### Offline Readiness

**What it detects:** Whether every container image required by the active addon set is already present in the local image store. Used as a pre-flight check for `kinder create cluster --air-gapped`. The catalog is sourced from `RequiredAllImages()` in `pkg/cluster/internal/providers/common`, which aggregates `Images []string` from each enabled addon package.

**Why it matters:** Air-gapped cluster creation (`--air-gapped`) disables every network call for image pulls. If even one required image is absent locally, creation fast-fails with a missing-image list — useful, but only after you've already invoked the command. Running this check first lets you pre-load everything in advance.

**Platforms:** All. Skipped when no container runtime is found.

**Status values:**

- **ok** — every required image is present in the local image store
- **warn** — one or more required images are absent (printed as a tabwriter table)
- **skip** — no container runtime found

**How to fix:**

Pull each missing image with your provider's `pull` command:

```sh
docker pull docker.io/metallb/controller:v0.15.3
docker pull docker.io/envoyproxy/envoy:v1.36.2
# ...etc for every image listed by the warning
```

The [Working Offline guide](/guides/working-offline/) walks through the full two-mode workflow: pre-create image baking vs. post-create `kinder load images`.

---

## Cluster Lifecycle

### Cluster Node Skew

**What it detects:** Kubernetes version skew across the nodes of a running cluster. Reports a warning when worker nodes are more than 3 minor versions behind the control-plane, or when control-plane nodes in an HA cluster do not all share the same minor version. Reads node Kubernetes versions via in-container `kubelet --version` (matches `realListNodes` semantics in the doctor package).

**Why it matters:** Out-of-skew workers can fail to register with the control-plane, refuse to schedule pods that need newer API features, or break in-place upgrade paths. Mixed control-plane minors are not supported by kubeadm and produce subtle etcd / scheduler bugs.

**Platforms:** All. Skipped when no kinder cluster is found, or when nodes use non-semver image tags (e.g. `latest`) that cannot be parsed.

**Status values:**

- **ok** — all nodes within skew policy
- **warn** — workers more than 3 minors behind CP, or CP minor mismatch detected
- **skip** — cluster not found, or non-semver node image tags in use

**How to fix:**

Recreate the cluster with a config that complies with skew rules. For multi-version clusters, see the [Multi-Version Clusters guide](/guides/multi-version-clusters/) — kinder validates skew at config-parse time so most violations are caught before any container is created.

---

### Cluster Resume Readiness

**What it detects:** Whether etcd quorum is healthy on a multi-control-plane cluster before `kinder resume` brings worker nodes back up. Probes etcd via `crictl exec <etcd-id> etcdctl endpoint health` against the first running control-plane node.

**Why it matters:** When a stopped HA cluster has lost more control-plane containers than its quorum tolerates (e.g., 2 of 3 CPs were `docker rm`'d while the cluster was paused), bringing workers up against a broken etcd produces hours of cascading kubelet/scheduler errors with no obvious root cause. This check warns up-front so the user can recover etcd before resume.

**Platforms:** All. Skipped on single-control-plane clusters (no quorum risk).

**Status values:**

- **ok** — all declared CP containers are present and etcd reports healthy on the bootstrap node
- **warn** — at least one CP container is missing/stopped, OR etcd reports unhealthy members below quorum
- **skip** — single-CP cluster, or no kinder cluster found, or `crictl`/`etcdctl` not reachable inside the etcd container (rare; indicates a non-standard node image)

**How to fix:**

If the warning fires because CP containers are stopped, restart them with `docker start <node-name>` before running `kinder resume`. If etcd itself reports degraded health, restore from the most recent `kinder snapshot` (the snapshot's etcd state was captured while the cluster was running).

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

All 24 checks documented on this page are **diagnostic only**. They report problems and suggest fixes but do not modify your system.
