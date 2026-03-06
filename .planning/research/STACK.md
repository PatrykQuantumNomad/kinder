# Technology Stack

**Project:** kinder -- Diagnostic checks and auto-mitigations for `kinder doctor`
**Scope:** Go libraries and system APIs for 13 new diagnostic checks
**Researched:** 2026-03-06
**Overall confidence:** HIGH -- All 13 checks are implementable with Go stdlib + one already-present indirect dependency (`golang.org/x/sys`). No new external dependencies required.

---

## Executive Summary: What Actually Changes

This milestone adds 13 diagnostic checks to the existing `kinder doctor` command. The existing command already has the result/checkResult pattern, JSON output, ok/warn/fail formatters, and structured exit codes (0/1/2). Each new check follows the same `func check*() result` or `func check*() []result` pattern.

**The critical finding: every check can be implemented using Go stdlib packages (`os`, `os/exec`, `encoding/json`, `net/netip`, `strings`, `strconv`, `runtime`) plus `golang.org/x/sys/unix` (already an indirect dependency in go.mod at v0.41.0). Zero new go.mod dependencies are needed.**

The `opencontainers/selinux` library was evaluated and rejected -- shelling out to `getenforce` is simpler, avoids adding a dependency, and is consistent with the existing doctor pattern (which uses `exec.Command` for all checks).

---

## Recommended Stack

### Go Standard Library -- Core for All Checks

| Package | Purpose | Used By Checks | Why |
|---------|---------|----------------|-----|
| `os` | Read `/proc` files, `os.Stat` for socket permissions, `os.ReadFile` for config files | 1, 3, 4, 5, 7, 8, 10, 11 | Already used throughout codebase; `os.ReadFile` for procfs/config files is the idiomatic Go approach |
| `os/exec` (via `sigs.k8s.io/kind/pkg/exec`) | Shell out to `docker`, `kubectl`, `getenforce`, `aa-status`, `ip route`, `uname` | 3, 4, 5, 6, 7, 8, 9, 12, 13 | Kinder already wraps `os/exec` in its own `exec.Command`/`exec.OutputLines` interface; all existing doctor checks use this pattern |
| `encoding/json` | Parse `docker info --format`, `docker network inspect`, `kubectl version --client -o json`, `daemon.json` | 4, 12, 13 | Already imported in `doctor.go`; Docker and kubectl both produce JSON output |
| `net/netip` | Parse CIDR prefixes, detect subnet overlaps with `Prefix.Overlaps()` | 13 | Go 1.18+ stdlib; provides `ParsePrefix` and `Overlaps()` -- purpose-built for subnet clash detection; no external dependency needed |
| `strings` | Parse command output, detect "microsoft" in `/proc/version` for WSL2, detect "snap" in binary paths | 3, 6, 7, 8, 10, 11 | Already imported in `doctor.go` |
| `strconv` | Parse numeric values from `/proc/sys/fs/inotify/*`, disk sizes | 1, 2 | Already used elsewhere in codebase |
| `runtime` | `runtime.GOOS` for platform-gated checks (Linux-only) | 1, 2, 5, 6, 7, 8, 9, 10, 11 | Already used in `doctor.go` for NVIDIA GPU Linux gate |
| `path/filepath` | Resolve symlinks for snap detection, construct procfs paths | 3, 5 | Stdlib; used for `filepath.EvalSymlinks` to resolve `/usr/bin/docker` -> `/snap/bin/docker` |

### Extended Standard Library -- `golang.org/x/sys/unix`

| Package | Version | Purpose | Used By Checks | Why |
|---------|---------|---------|----------------|-----|
| `golang.org/x/sys/unix` | v0.41.0 | `unix.Statfs` for disk space, `unix.Uname` for kernel version | 2, 11 | **Already an indirect dependency in go.mod.** Preferred over deprecated `syscall.Statfs`. Provides `Statfs_t.Bavail` (available-to-user space) and `Utsname.Release` (kernel version string). Must be promoted from indirect to direct in go.mod (no version change). |

### Existing Kinder Internal Packages -- Reuse

| Package | Purpose | Used By Checks | Why |
|---------|---------|----------------|-----|
| `sigs.k8s.io/kind/pkg/exec` | `exec.Command`, `exec.OutputLines`, `exec.Output` wrappers | All command-based checks | The existing doctor command already uses this; maintains testability through the `Cmd` interface |
| `sigs.k8s.io/kind/pkg/internal/version` | `ParseSemantic`, `AtLeast`, `LessThan` for version comparison | 12 | Already exists in codebase; forked from `k8s.io/apimachinery/pkg/util/version`; handles Kubernetes `v1.30.0` format correctly; no need for Masterminds/semver |
| `sigs.k8s.io/kind/pkg/cmd` | `IOStreams` for output routing | Integration | Already used by `doctor.go` |
| `sigs.k8s.io/kind/pkg/log` | Logger interface | Integration | Already used by `doctor.go` |

---

## Check-by-Check Implementation Details

### Check 1: inotify Limits (`/proc/sys/fs/inotify/*`)

**Platform:** Linux only
**API:** `os.ReadFile` + `strconv.Atoi`
**Files read:**
- `/proc/sys/fs/inotify/max_user_watches` (should be >= 524288)
- `/proc/sys/fs/inotify/max_user_instances` (should be >= 512)

**Implementation:**
```go
func checkInotifyLimits() []result {
    // Skip on non-Linux
    if runtime.GOOS != "linux" {
        return nil
    }
    data, err := os.ReadFile("/proc/sys/fs/inotify/max_user_watches")
    if err != nil {
        return []result{{name: "inotify-watches", status: "warn", message: "could not read inotify limits"}}
    }
    val, _ := strconv.Atoi(strings.TrimSpace(string(data)))
    if val < 524288 {
        return []result{{name: "inotify-watches", status: "warn",
            message: fmt.Sprintf("max_user_watches=%d (< 524288) -- run: sudo sysctl fs.inotify.max_user_watches=524288")}}
    }
    // ... similar for max_user_instances
}
```

**External dependencies:** None. Procfs is a virtual filesystem readable with standard file I/O.
**Confidence:** HIGH -- this is the documented kind fix for "too many open files" (kind known issues page).

### Check 2: Available Disk Space

**Platform:** Linux and macOS (not Windows)
**API:** `golang.org/x/sys/unix.Statfs`
**Why `unix.Statfs` not `syscall.Statfs`:** The `syscall` package is frozen/deprecated per Go team policy. `golang.org/x/sys/unix` is the maintained replacement and is already in go.mod as an indirect dependency.

**Implementation:**
```go
func checkDiskSpace() result {
    var stat unix.Statfs_t
    if err := unix.Statfs("/var/lib/docker", &stat); err != nil {
        // Fall back to checking "/"
        if err := unix.Statfs("/", &stat); err != nil {
            return result{name: "disk-space", status: "warn", message: "could not check disk space"}
        }
    }
    availableBytes := stat.Bavail * uint64(stat.Bsize)
    availableGB := availableBytes / (1024 * 1024 * 1024)
    if availableGB < 10 {
        return result{name: "disk-space", status: "warn",
            message: fmt.Sprintf("%d GB available -- recommend at least 10 GB; run: docker system prune", availableGB)}
    }
    return result{name: "disk-space", status: "ok", message: fmt.Sprintf("%d GB available", availableGB)}
}
```

**Cross-platform:** `unix.Statfs` works on Linux and macOS (darwin). For macOS, check the Docker VM's disk via `docker system df --format '{{json .}}'` as an alternative since macOS Docker runs in a VM.
**Confidence:** HIGH -- `Bavail` (not `Bfree`) is the correct field (matches `df` behavior for non-root users).

### Check 3: Docker Installed via Snap

**Platform:** Linux only
**API:** `os/exec.LookPath` + `filepath.EvalSymlinks`
**Detection method:** Resolve the Docker binary path and check if it contains `/snap/`:

```go
func checkDockerSnap() result {
    if runtime.GOOS != "linux" {
        return result{} // skip
    }
    dockerPath, err := osexec.LookPath("docker")
    if err != nil {
        return result{} // no docker found -- other check handles this
    }
    resolved, err := filepath.EvalSymlinks(dockerPath)
    if err != nil {
        resolved = dockerPath
    }
    if strings.Contains(resolved, "/snap/") {
        return result{name: "docker-snap", status: "warn",
            message: "Docker installed via snap -- kind may fail due to snap confinement. Set TMPDIR=$HOME/tmp or install Docker via apt/dnf."}
    }
    return result{name: "docker-snap", status: "ok"}
}
```

**Why not `snap list docker`:** Shelling out to `snap` adds a dependency on the `snap` binary being on PATH. Checking the resolved binary path is simpler and works even when snap is not on PATH.
**Confidence:** HIGH -- kind's known issues page documents this exact problem.

### Check 4: Docker daemon.json Parsing

**Platform:** All (but path differs)
**API:** `os.ReadFile` + `encoding/json`
**Location:** `/etc/docker/daemon.json` (Linux), `~/.docker/daemon.json` (macOS user config)
**What to check:**
- `"init": true` causes kind startup failures (documented known issue)
- `"default-address-pools"` -- informational, relevant to check 13

```go
type daemonConfig struct {
    Init                bool            `json:"init"`
    DefaultAddressPools json.RawMessage `json:"default-address-pools,omitempty"`
    StorageDriver       string          `json:"storage-driver,omitempty"`
}

func checkDaemonJSON() result {
    data, err := os.ReadFile("/etc/docker/daemon.json")
    if err != nil {
        return result{name: "daemon-json", status: "ok", message: "no daemon.json found (using defaults)"}
    }
    var cfg daemonConfig
    if err := json.Unmarshal(data, &cfg); err != nil {
        return result{name: "daemon-json", status: "warn", message: "daemon.json exists but is not valid JSON"}
    }
    if cfg.Init {
        return result{name: "daemon-json", status: "fail",
            message: `"init": true in daemon.json causes kind startup failures -- remove or set to false`}
    }
    return result{name: "daemon-json", status: "ok"}
}
```

**External dependencies:** None. `encoding/json` handles this entirely.
**Confidence:** HIGH -- `"init": true` is a documented kind known issue.

### Check 5: Docker Socket Permissions

**Platform:** Linux and macOS
**API:** `os.Stat` + `os.Getgroups` + `os.Getuid`
**Socket path:** `/var/run/docker.sock` (standard), `/run/docker.sock` (some distros)

```go
func checkDockerSocket() result {
    socketPath := "/var/run/docker.sock"
    info, err := os.Stat(socketPath)
    if err != nil {
        return result{name: "docker-socket", status: "warn", message: "Docker socket not found at " + socketPath}
    }
    // Check if current user can access it
    if os.Getuid() == 0 {
        return result{name: "docker-socket", status: "ok", message: "running as root"}
    }
    // Try to actually open the socket to test access (most reliable)
    if _, err := net.DialTimeout("unix", socketPath, time.Second); err != nil {
        return result{name: "docker-socket", status: "fail",
            message: "cannot connect to Docker socket -- add user to docker group: sudo usermod -aG docker $USER"}
    }
    return result{name: "docker-socket", status: "ok"}
}
```

**Why test actual connectivity:** Checking `FileMode.Perm()` and group membership is unreliable (ACLs, Docker Desktop socket proxy, rootless Docker). Actually dialing the socket is the definitive test. The existing `checkBinary("docker")` already runs `docker version` which would catch this, but this check provides a more specific error message.
**Confidence:** HIGH -- `net.DialTimeout("unix", ...)` is stdlib.

### Check 6: Firewalld Backend (iptables vs nftables)

**Platform:** Linux only
**API:** `os.ReadFile` (config file) + `exec.Command` (fallback)
**Detection method:** Read `/etc/firewalld/firewalld.conf` and look for `FirewallBackend=`:

```go
func checkFirewalldBackend() result {
    if runtime.GOOS != "linux" {
        return result{}
    }
    data, err := os.ReadFile("/etc/firewalld/firewalld.conf")
    if err != nil {
        return result{name: "firewalld", status: "ok", message: "firewalld not installed"}
    }
    for _, line := range strings.Split(string(data), "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "FirewallBackend=") {
            backend := strings.TrimPrefix(line, "FirewallBackend=")
            if backend == "nftables" {
                return result{name: "firewalld", status: "warn",
                    message: "firewalld uses nftables backend which can break kind networking. " +
                        "Edit /etc/firewalld/firewalld.conf: FirewallBackend=iptables, then: sudo systemctl restart firewalld"}
            }
            return result{name: "firewalld", status: "ok", message: "firewalld backend: " + backend}
        }
    }
    // No explicit backend found -- default depends on version
    // firewalld 0.6.0+ defaults to nftables
    return result{name: "firewalld", status: "warn",
        message: "firewalld detected but backend not explicit in config -- may default to nftables. Verify with: firewall-cmd --version"}
}
```

**Why not use `google/nftables` or `coreos/go-iptables`:** Those libraries are for managing firewall rules, not detecting the firewalld backend. Reading the config file is simpler and has zero dependencies.
**Confidence:** HIGH -- `/etc/firewalld/firewalld.conf` with `FirewallBackend=` is the documented configuration mechanism.

### Check 7: SELinux Enforcing Mode

**Platform:** Linux only
**API:** `exec.Command("getenforce")` output parsing
**Why not `opencontainers/selinux`:** That library adds a dependency with 19 imports of its own. The existing doctor pattern shells out to binaries (nvidia-smi, docker, kubectl). Shelling out to `getenforce` is consistent and the output is trivially parseable (literally "Enforcing", "Permissive", or "Disabled").

```go
func checkSELinux() result {
    if runtime.GOOS != "linux" {
        return result{}
    }
    lines, err := exec.OutputLines(exec.Command("getenforce"))
    if err != nil || len(lines) == 0 {
        return result{name: "selinux", status: "ok", message: "SELinux not detected"}
    }
    mode := strings.TrimSpace(lines[0])
    if mode == "Enforcing" {
        return result{name: "selinux", status: "warn",
            message: "SELinux enforcing -- may cause kind permission errors on Fedora 33+. Temporary fix: sudo setenforce 0"}
    }
    return result{name: "selinux", status: "ok", message: "SELinux mode: " + mode}
}
```

**Confidence:** HIGH -- kind's known issues page documents this for Fedora.

### Check 8: AppArmor Status

**Platform:** Linux only
**API:** `os.ReadFile("/sys/module/apparmor/parameters/enabled")` for detection, optional `exec.Command("aa-status")` for profile details
**Why not `aa-status` as primary:** `aa-status` requires root. Reading `/sys/module/apparmor/parameters/enabled` works without root and returns "Y" or "N".

```go
func checkAppArmor() result {
    if runtime.GOOS != "linux" {
        return result{}
    }
    data, err := os.ReadFile("/sys/module/apparmor/parameters/enabled")
    if err != nil {
        return result{name: "apparmor", status: "ok", message: "AppArmor not loaded"}
    }
    if strings.TrimSpace(string(data)) == "Y" {
        // AppArmor is enabled -- this is informational, not a problem in most cases
        // Check if docker-default profile exists
        if _, err := os.Stat("/etc/apparmor.d/docker"); err == nil {
            return result{name: "apparmor", status: "ok", message: "AppArmor enabled with docker profile"}
        }
        return result{name: "apparmor", status: "ok", message: "AppArmor enabled"}
    }
    return result{name: "apparmor", status: "ok", message: "AppArmor disabled"}
}
```

**Confidence:** HIGH -- `/sys/module/apparmor/parameters/enabled` is the kernel-level check; no root required.

### Check 9: Device Node Access for Rootfs

**Platform:** Linux only
**API:** `exec.Command("docker", "info", "-f", "{{.Driver}}")` to get storage driver, then check device availability
**Integration:** The existing `mountDevMapper()` function in `pkg/cluster/internal/providers/docker/util.go` already detects btrfs/zfs/devicemapper storage drivers. This check extends that pattern.

```go
func checkDeviceNodeAccess() result {
    if runtime.GOOS != "linux" {
        return result{}
    }
    lines, err := exec.OutputLines(exec.Command("docker", "info", "-f", "{{.Driver}}"))
    if err != nil || len(lines) == 0 {
        return result{name: "rootfs-device", status: "warn", message: "could not query Docker storage driver"}
    }
    driver := strings.TrimSpace(strings.ToLower(lines[0]))
    if driver == "btrfs" || driver == "zfs" || driver == "devicemapper" {
        return result{name: "rootfs-device", status: "warn",
            message: fmt.Sprintf("Docker uses %s storage driver -- kind may need extraMounts for device nodes in cluster config", driver)}
    }
    return result{name: "rootfs-device", status: "ok", message: "storage driver: " + driver}
}
```

**External dependencies:** None -- reuses the same `docker info` approach as existing code.
**Confidence:** HIGH -- kind's known issues page documents rootfs device access concerns.

### Check 10: WSL2 Environment and Cgroup Configuration

**Platform:** Linux only (specifically WSL2 Linux userspace)
**API:** `os.ReadFile("/proc/version")` + `os.ReadFile("/proc/sys/fs/cgroup")`
**WSL2 detection:** Check if `/proc/version` contains "microsoft" (case-insensitive).

```go
func checkWSL2() result {
    if runtime.GOOS != "linux" {
        return result{}
    }
    data, err := os.ReadFile("/proc/version")
    if err != nil {
        return result{}
    }
    version := strings.ToLower(string(data))
    if !strings.Contains(version, "microsoft") {
        return result{} // Not WSL2
    }
    // We're in WSL2 -- check cgroup configuration
    // stat /sys/fs/cgroup to check cgroup version
    var cgroupVersion string
    if data, err := os.ReadFile("/proc/filesystems"); err == nil {
        if strings.Contains(string(data), "cgroup2") {
            cgroupVersion = "v2"
        } else {
            cgroupVersion = "v1"
        }
    }
    // Check if systemd is running (needed for cgroupv2 in WSL2)
    if _, err := os.Stat("/run/systemd/system"); os.IsNotExist(err) {
        return result{name: "wsl2", status: "warn",
            message: "WSL2 detected without systemd -- cgroup v2 may not work. Enable systemd in /etc/wsl.conf: [boot] systemd=true"}
    }
    return result{name: "wsl2", status: "ok", message: "WSL2 detected with systemd, cgroup " + cgroupVersion}
}
```

**External dependencies:** None -- pure procfs reads.
**Confidence:** HIGH -- `/proc/version` containing "microsoft" is the universally documented WSL2 detection method.

### Check 11: Kernel Version and Cgroup Namespace Support

**Platform:** Linux only
**API:** `golang.org/x/sys/unix.Uname` for kernel version, `os.Stat("/proc/self/ns/cgroup")` for cgroup namespace support
**Why `unix.Uname` not `exec.Command("uname")`:** `unix.Uname` is a direct syscall -- no fork/exec overhead, no PATH dependency. However, `exec.Command("uname", "-r")` also works and is more consistent with the existing doctor pattern. Either is acceptable.

```go
func checkKernelVersion() result {
    if runtime.GOOS != "linux" {
        return result{}
    }
    var uname unix.Utsname
    if err := unix.Uname(&uname); err != nil {
        return result{name: "kernel", status: "warn", message: "could not determine kernel version"}
    }
    release := unix.ByteSliceToString(uname.Release[:])
    ver, err := version.ParseGeneric(release)
    if err != nil {
        return result{name: "kernel", status: "warn", message: "could not parse kernel version: " + release}
    }
    // kind v0.20+ requires cgroup namespaces, which need kernel 4.6+
    minKernel := version.MustParseGeneric("4.6")
    if ver.LessThan(minKernel) {
        return result{name: "kernel", status: "fail",
            message: fmt.Sprintf("kernel %s is too old -- kind requires 4.6+ for cgroup namespaces", release)}
    }
    // Also check for cgroup namespace support directly
    if _, err := os.Stat("/proc/self/ns/cgroup"); os.IsNotExist(err) {
        return result{name: "kernel", status: "fail",
            message: "kernel does not support cgroup namespaces -- upgrade to kernel 4.6+ or a supported distribution"}
    }
    return result{name: "kernel", status: "ok", message: "kernel " + release}
}
```

**Reuse:** Uses the existing `sigs.k8s.io/kind/pkg/internal/version.ParseGeneric` for version comparison -- no external semver library needed.
**Confidence:** HIGH -- kind v0.20+ requires cgroup namespaces; kernel 4.6 is the documented minimum.

### Check 12: kubectl Version Skew

**Platform:** All
**API:** `exec.Command("kubectl", "version", "--client", "-o", "json")` + `encoding/json` + `version.ParseSemantic`
**Kubernetes version skew policy:** kubectl must be within +/-1 minor version of the API server.

```go
type kubectlVersionOutput struct {
    ClientVersion struct {
        GitVersion string `json:"gitVersion"`
    } `json:"clientVersion"`
}

func checkKubectlVersionSkew() result {
    lines, err := exec.OutputLines(exec.Command("kubectl", "version", "--client", "-o", "json"))
    if err != nil || len(lines) == 0 {
        return result{} // kubectl not found -- handled by existing check
    }
    var vOut kubectlVersionOutput
    if err := json.Unmarshal([]byte(strings.Join(lines, "")), &vOut); err != nil {
        return result{name: "kubectl-version", status: "warn", message: "could not parse kubectl version JSON"}
    }
    clientVer, err := version.ParseSemantic(vOut.ClientVersion.GitVersion)
    if err != nil {
        return result{name: "kubectl-version", status: "warn", message: "could not parse kubectl version: " + vOut.ClientVersion.GitVersion}
    }
    // Report the version; actual skew check needs a running cluster (server version)
    return result{name: "kubectl-version", status: "ok",
        message: fmt.Sprintf("kubectl %s (skew check requires running cluster)", clientVer.String())}
}
```

**Note:** Full version skew validation requires both client AND server versions. Without a running cluster, the doctor check can only report the client version and warn if it's very old. Consider adding `--cluster` flag to doctor to enable server-side checks.
**Reuse:** Uses existing `sigs.k8s.io/kind/pkg/internal/version.ParseSemantic`.
**Confidence:** HIGH -- `kubectl version --client -o json` output format is stable Kubernetes API.

### Check 13: Docker Network Subnet Clashes

**Platform:** All
**API:** `exec.Command("docker", "network", "inspect", "kind")` + `exec.Command("ip", "route")` (Linux) or `exec.Command("netstat", "-rn")` (macOS) + `net/netip.ParsePrefix` + `Prefix.Overlaps`
**Integration:** The existing `network.go` in the docker provider already handles network inspection and CIDR generation. This check adds a pre-creation diagnostic.

```go
func checkNetworkSubnetClash() result {
    // Get kind network subnets
    inspectOut, err := exec.Output(exec.Command("docker", "network", "inspect", "kind", "--format", "{{range .IPAM.Config}}{{.Subnet}} {{end}}"))
    if err != nil {
        return result{name: "network-subnet", status: "ok", message: "kind network not created yet"}
    }
    var kindSubnets []netip.Prefix
    for _, s := range strings.Fields(strings.TrimSpace(string(inspectOut))) {
        if prefix, err := netip.ParsePrefix(s); err == nil {
            kindSubnets = append(kindSubnets, prefix)
        }
    }
    // Get host routes
    var routeLines []string
    if runtime.GOOS == "linux" {
        routeLines, _ = exec.OutputLines(exec.Command("ip", "route"))
    } else {
        routeLines, _ = exec.OutputLines(exec.Command("netstat", "-rn"))
    }
    for _, line := range routeLines {
        fields := strings.Fields(line)
        for _, field := range fields {
            if hostPrefix, err := netip.ParsePrefix(field); err == nil {
                for _, kindPrefix := range kindSubnets {
                    if hostPrefix.Overlaps(kindPrefix) {
                        return result{name: "network-subnet", status: "warn",
                            message: fmt.Sprintf("kind network %s overlaps host route %s -- this may cause connectivity issues", kindPrefix, hostPrefix)}
                    }
                }
            }
        }
    }
    return result{name: "network-subnet", status: "ok"}
}
```

**Why `net/netip` not `net.ParseCIDR`:** `net/netip.Prefix` has the built-in `Overlaps()` method which is exactly what this check needs. `net.IPNet` would require manual overlap computation. `net/netip` has been in stdlib since Go 1.18 (kinder requires Go 1.24+).
**Confidence:** HIGH -- `Overlaps()` is purpose-built for this; `docker network inspect` format is stable.

---

## Dependency Impact Summary

### go.mod Changes Required

```
# Promote from indirect to direct (no version change):
golang.org/x/sys v0.41.0  # currently indirect; used for unix.Statfs (check 2) and unix.Uname (check 11)
```

**That is the only go.mod change.** No new dependencies. The dependency is already downloaded and in the module cache.

### No New Dependencies Needed

| Evaluated | Decision | Reason |
|-----------|----------|--------|
| `github.com/opencontainers/selinux/go-selinux` | REJECTED | `getenforce` shell-out is simpler, consistent with existing doctor pattern, avoids 19 transitive imports |
| `github.com/google/nftables` | REJECTED | We only need to read a config file, not manage nftables rules |
| `github.com/coreos/go-iptables` | REJECTED | Same reason -- detection, not management |
| `github.com/Masterminds/semver` | REJECTED | Kinder already has `pkg/internal/version` with `ParseSemantic` and comparison methods |
| `github.com/shirou/gopsutil` | REJECTED | Would provide disk/system info but is a large dependency; `unix.Statfs` and procfs reads are sufficient |
| `github.com/yl2chen/cidranger` | REJECTED | `net/netip.Prefix.Overlaps()` handles the subnet clash check without external deps |

---

## Platform Compatibility Matrix

| Check | Linux | macOS | Windows | Notes |
|-------|-------|-------|---------|-------|
| 1. inotify limits | YES | skip | skip | procfs is Linux-only |
| 2. disk space | YES | YES | skip | `unix.Statfs` works on both; on macOS check Docker Desktop VM disk |
| 3. snap docker | YES | skip | skip | Snap is Linux-only |
| 4. daemon.json | YES | YES | YES | Path differs: `/etc/docker/daemon.json` (Linux), `~/.docker/daemon.json` (macOS/Windows) |
| 5. docker socket | YES | YES | skip | Unix socket; Windows uses named pipe |
| 6. firewalld | YES | skip | skip | firewalld is Linux-only |
| 7. SELinux | YES | skip | skip | SELinux is Linux-only |
| 8. AppArmor | YES | skip | skip | AppArmor is Linux-only |
| 9. rootfs devices | YES | skip | skip | Storage driver concerns are Linux-specific |
| 10. WSL2 | YES | skip | skip | Runs inside WSL2 Linux userspace |
| 11. kernel version | YES | skip | skip | Kernel version relevant only for Linux |
| 12. kubectl version | YES | YES | YES | Cross-platform |
| 13. network clashes | YES | YES | skip | `ip route` (Linux), `netstat -rn` (macOS) |

**Pattern:** Use `runtime.GOOS == "linux"` guard (already used for NVIDIA checks in doctor.go). Return `nil` or empty result to skip silently on unsupported platforms.

---

## Integration Architecture

### Existing Pattern (to follow exactly)

Each check is a Go function returning `result` or `[]result`:

```go
// In doctor.go runE():
if runtime.GOOS == "linux" {
    results = append(results, checkInotifyLimits()...)
    results = append(results, checkFirewalldBackend())
    results = append(results, checkSELinux())
    results = append(results, checkAppArmor())
    results = append(results, checkKernelVersion())
    results = append(results, checkWSL2())
    results = append(results, checkDeviceNodeAccess())
}
results = append(results, checkDiskSpace())
results = append(results, checkDaemonJSON())
results = append(results, checkDockerSocket())
results = append(results, checkDockerSnap())
results = append(results, checkKubectlVersionSkew())
results = append(results, checkNetworkSubnetClash())
```

### File Organization

All check functions should go in `pkg/cmd/kind/doctor/` alongside `doctor.go`. Options:

1. **Single file** (recommended for <500 LOC): Add all checks to `doctor.go`
2. **Separate files by platform**: `checks_linux.go` (build-tagged), `checks_all.go` (cross-platform checks)

**Recommendation:** Use build-tagged files. Put Linux-only checks in `checks_linux.go` with `//go:build linux` and cross-platform checks in `checks.go`. This follows Go convention and avoids runtime `if runtime.GOOS` for platform-specific system calls like `unix.Statfs`/`unix.Uname`.

### Testing Pattern

The existing codebase does NOT have unit tests for `doctor.go`. For the new checks, testability comes from:

1. **Extract file paths and command names as package-level vars** for test overriding
2. **Use `exec.Cmd` interface** -- the existing FakeNode/FakeCmd infrastructure could be extended
3. **Table-driven tests** with mock procfs content (write temp files, override paths)

```go
// Example test setup
func TestCheckInotifyLimits(t *testing.T) {
    tmpDir := t.TempDir()
    watchesFile := filepath.Join(tmpDir, "max_user_watches")
    os.WriteFile(watchesFile, []byte("8192\n"), 0644)
    // Override the file path for testing
    origPath := inotifyWatchesPath
    inotifyWatchesPath = watchesFile
    defer func() { inotifyWatchesPath = origPath }()

    results := checkInotifyLimits()
    if results[0].status != "warn" {
        t.Errorf("expected warn for 8192, got %s", results[0].status)
    }
}
```

---

## Auto-Mitigation Integration Points

For auto-mitigations during cluster creation, the same check functions can be called from the create flow:

```go
// In cluster creation code, before provisioning:
if results := checkInotifyLimits(); hasFailOrWarn(results) {
    logger.Warnf("inotify limits too low; attempting to fix...")
    // Auto-mitigation: write sysctl values
    os.WriteFile("/proc/sys/fs/inotify/max_user_watches", []byte("524288"), 0644)
}
```

**Which checks support auto-mitigation:**

| Check | Auto-Mitigatable | How |
|-------|-------------------|-----|
| 1. inotify | YES (if root) | Write to `/proc/sys/fs/inotify/*` |
| 2. disk space | NO | Cannot free space automatically |
| 3. snap docker | NO | Cannot reinstall Docker |
| 4. daemon.json | NO | Should not modify user's Docker config |
| 5. socket perms | NO | Should not modify system permissions |
| 6. firewalld | NO | Requires config change + service restart |
| 7. SELinux | MAYBE | `setenforce 0` is dangerous; warn only |
| 8. AppArmor | NO | Should not disable security modules |
| 9. rootfs device | YES | Add `extraMounts` to kind config automatically |
| 10. WSL2 | NO | Requires WSL config change |
| 11. kernel | NO | Cannot upgrade kernel |
| 12. kubectl | NO | Cannot install/upgrade kubectl |
| 13. network | YES | Choose non-clashing subnet in network creation |

---

## Sources

- `golang.org/x/sys/unix` package docs: https://pkg.go.dev/golang.org/x/sys/unix -- v0.41.0, published 2026-02-08 (HIGH confidence -- official Go extended stdlib)
- `net/netip` package docs: https://pkg.go.dev/net/netip -- `ParsePrefix`, `Overlaps` methods confirmed (HIGH confidence -- Go stdlib since 1.18)
- Kind known issues: https://kind.sigs.k8s.io/docs/user/known-issues/ -- inotify, snap Docker, daemon.json `init:true`, firewalld nftables, SELinux Fedora 33+, cgroup namespaces, rootfs devices (HIGH confidence -- official kind docs)
- Firewalld nftables backend: https://firewalld.org/2018/07/nftables-backend -- `/etc/firewalld/firewalld.conf` `FirewallBackend=` setting (HIGH confidence -- official firewalld docs)
- `opencontainers/selinux` Go package: https://pkg.go.dev/github.com/opencontainers/selinux/go-selinux -- v1.13.1, evaluated and rejected (HIGH confidence)
- Kubernetes inotify issue: https://github.com/kubernetes/kubernetes/issues/46230 -- recommended values 524288/512 (HIGH confidence)
- Docker daemon.json location: https://docs.docker.com/reference/cli/dockerd/#daemon-configuration-file -- `/etc/docker/daemon.json` (HIGH confidence -- official Docker docs)
- AppArmor kernel parameter: `/sys/module/apparmor/parameters/enabled` returns "Y"/"N" (HIGH confidence -- kernel documentation)
- WSL2 detection via `/proc/version`: https://gist.github.com/s0kil/336a246cc2bc8608e645c69876c17466 -- "microsoft" marker (MEDIUM confidence -- community pattern, universally used)
- Kinder codebase: direct reads of `doctor.go`, `go.mod`, `network.go`, `util.go`, `cgroups.go`, `version.go`, `exec/` package (HIGH confidence -- primary source)

---
*Stack research for: kinder -- diagnostic checks and auto-mitigations for `kinder doctor`*
*Researched: 2026-03-06*
