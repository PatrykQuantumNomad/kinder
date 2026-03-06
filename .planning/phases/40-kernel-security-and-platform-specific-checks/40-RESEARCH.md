# Phase 40: Kernel, Security, and Platform-Specific Checks - Research

**Researched:** 2026-03-06
**Domain:** Linux kernel parameter checks, security module detection (AppArmor/SELinux), platform-specific diagnostics (firewalld, WSL2, BTRFS device nodes)
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| KERN-01 | Doctor checks inotify max_user_watches (>=524288) and max_user_instances (>=512) | Architecture: inotifyCheck with readFile dep for /proc/sys/fs/inotify/ paths; Code Examples: strconv.Atoi parsing of proc values; Pitfalls: Docker Desktop context, container vs host values |
| KERN-02 | Doctor detects AppArmor profiles interfering with kind containers | Architecture: apparmorCheck with readFile dep for /sys/module/apparmor/parameters/enabled and execCmd for aa-status; Code Examples: profile presence detection; Pitfalls: AppArmor and SELinux can coexist on same system |
| KERN-03 | Doctor detects SELinux enforcing mode on Fedora | Architecture: selinuxCheck with readFile dep for /sys/fs/selinux/enforce and /etc/os-release; Code Examples: enforce file parsing + distro detection; Pitfalls: non-RHEL distros, getenforce may not be installed |
| KERN-04 | Doctor checks kernel version >=4.6 for cgroup namespace support | Architecture: kernelVersionCheck with uname dep (unix.Uname); Code Examples: release string parsing with major.minor extraction; Pitfalls: WSL2 kernel strings contain extra tokens |
| PLAT-01 | Doctor detects firewalld nftables backend on Fedora 32+ | Architecture: firewalldCheck with readFile dep for /etc/firewalld/firewalld.conf and execCmd for firewall-cmd; Code Examples: config file INI-style parsing; Pitfalls: firewalld not installed on non-Fedora |
| PLAT-02 | Doctor detects WSL2 with multi-signal approach and checks cgroup v2 config | Architecture: wsl2Check with readFile dep for /proc/version + stat dep for /proc/sys/fs/binfmt_misc/WSLInterop + getenv dep; Code Examples: multi-signal detection, cgroup.controllers parsing; Pitfalls: false positive on Azure VMs, WSLInterop-late on WSL 4.1.4+ |
| PLAT-04 | Doctor checks device node access for rootfs (BTRFS/NVMe) | Architecture: rootfsDeviceCheck with execCmd dep for docker info + readFile for /proc/mounts; Code Examples: backing filesystem detection via docker info DriverStatus; Pitfalls: BTRFS works in many configs, only warn |
</phase_requirements>

## Summary

Phase 40 adds seven new diagnostic checks to `kinder doctor` covering Linux kernel parameters, security modules, and platform-specific configurations that silently break or degrade kind clusters. All seven checks are Linux-only (Platforms returns `[]string{"linux"}`), following the exact same Check interface, deps struct injection pattern, and fakeCmd test doubles established in Phases 38-39. No new go.mod dependencies are required -- the only external package needed is `golang.org/x/sys/unix` (already direct at v0.41.0) for the kernel version check via `unix.Uname`. All other detection relies on reading `/proc` and `/sys` pseudo-filesystem files via injectable `readFile` functions and executing commands via injectable `execCmd` functions.

The seven checks address the remaining items from kind's Known Issues page: inotify limits causing "too many open files" (KERN-01), AppArmor profiles interfering with container operations (KERN-02), SELinux enforcing mode blocking /dev/dma_heap access on Fedora (KERN-03), old kernels lacking cgroup namespace support (KERN-04), firewalld nftables backend breaking Docker networking on Fedora 32+ (PLAT-01), WSL2 cgroup v2 misconfiguration (PLAT-02), and BTRFS/NVMe device node access failures (PLAT-04). Each check produces results with the standard 6-field Result struct and integrates into the AllChecks registry with two new categories: "Kernel" and "Platform".

The primary implementation challenge is PLAT-02 (WSL2 detection) which requires a multi-signal approach to avoid false-positiving on Azure VMs: checking /proc/version for "microsoft" AND requiring a second signal ($WSL_DISTRO_NAME or /proc/sys/fs/binfmt_misc/WSLInterop existence). The other six checks are straightforward file reads and command executions following established patterns.

**Primary recommendation:** Add seven new check source files to `pkg/internal/doctor/` (inotify.go, apparmor.go, selinux.go, kernel.go, firewalld.go, wsl2.go, rootfs.go) using the exact deps struct + fakeCmd test pattern, register all 7 in AllChecks() under "Kernel" and "Platform" categories, and update the AllChecks registry test count from 10 to 17.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/sys/unix` | v0.41.0 | `unix.Uname` for kernel version detection on Linux | Already direct in go.mod since Phase 39; provides `Utsname.Release` field for kernel version parsing |
| Go stdlib (`os`, `os/exec`, `strconv`, `strings`, `runtime`, `fmt`, `bufio`) | Go 1.24+ | File reads from /proc and /sys, string parsing, integer conversion | Already used by all existing checks; zero new imports |
| `sigs.k8s.io/kind/pkg/exec` | (internal) | Command execution via Cmd interface for aa-status, firewall-cmd, docker info | Same pattern as all Phase 38-39 checks; enables fakeCmd test injection |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/internal/doctor` (testhelpers_test.go) | (internal) | `fakeCmd`, `fakeExecResult`, `newFakeExecCmd` | All check unit tests -- exact same test doubles from Phase 38-39 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Reading `/sys/fs/selinux/enforce` directly | `github.com/opencontainers/selinux/go-selinux` library | go-selinux adds a dependency; reading the single sysfs file is sufficient for detecting enforcing mode and requires zero new deps |
| Shelling out to `getenforce` | Reading `/sys/fs/selinux/enforce` | getenforce binary may not be installed on non-RHEL distros; the sysfs file is a kernel interface always present when SELinux is compiled in |
| Shelling out to `uname -r` | `unix.Uname()` from golang.org/x/sys/unix | unix.Uname is a direct syscall, no fork/exec needed, and the package is already in go.mod |
| Using `github.com/containerd/cgroups` for cgroup v2 detection | Reading `/sys/fs/cgroup/cgroup.controllers` directly | containerd/cgroups is a heavy dependency; a single file read is sufficient for WSL2 cgroup v2 check |

**Installation:**
```bash
# No changes needed -- golang.org/x/sys/unix v0.41.0 already direct in go.mod
# No new dependencies required
go mod tidy
```

## Architecture Patterns

### Recommended Project Structure
```
pkg/internal/doctor/
    check.go              # MODIFIED: add 7 new checks to allChecks registry (total: 17)
    inotify.go            # NEW: inotifyCheck (KERN-01)
    inotify_test.go       # NEW: tests for inotify limits
    apparmor.go           # NEW: apparmorCheck (KERN-02)
    apparmor_test.go      # NEW: tests for AppArmor detection
    selinux.go            # NEW: selinuxCheck (KERN-03)
    selinux_test.go       # NEW: tests for SELinux detection
    kernel.go             # NEW: kernelVersionCheck (KERN-04)
    kernel_test.go        # NEW: tests for kernel version check
    firewalld.go          # NEW: firewalldCheck (PLAT-01)
    firewalld_test.go     # NEW: tests for firewalld backend detection
    wsl2.go               # NEW: wsl2Check (PLAT-02)
    wsl2_test.go          # NEW: tests for WSL2 detection and cgroup v2
    rootfs.go             # NEW: rootfsDeviceCheck (PLAT-04)
    rootfs_test.go        # NEW: tests for rootfs device node check
    # existing files unchanged:
    runtime.go, runtime_test.go
    tools.go, tools_test.go
    gpu.go, gpu_test.go
    disk.go, disk_unix.go, disk_other.go, disk_test.go
    daemon.go, daemon_test.go
    snap.go, snap_test.go
    socket.go, socket_test.go
    versionskew.go, versionskew_test.go
    format.go, format_test.go
    mitigations.go, mitigations_test.go
    testhelpers_test.go
    check_test.go
```

### Pattern 1: Deps Struct with readFile for /proc and /sys Reads
**What:** Checks that read pseudo-filesystem files use an injectable `readFile` function field instead of directly calling `os.ReadFile`. This enables deterministic testing without requiring a real /proc filesystem.
**When to use:** All kernel and security checks (KERN-01 through KERN-04, PLAT-01, PLAT-02, PLAT-04).
**Example:**
```go
// Source: Extension of Phase 38-39 deps struct pattern

type inotifyCheck struct {
    readFile func(string) ([]byte, error)
}

func newInotifyCheck() Check {
    return &inotifyCheck{
        readFile: os.ReadFile,
    }
}

func (c *inotifyCheck) Name() string       { return "inotify-limits" }
func (c *inotifyCheck) Category() string    { return "Kernel" }
func (c *inotifyCheck) Platforms() []string { return []string{"linux"} }
```

### Pattern 2: Multi-Signal Detection for WSL2
**What:** WSL2 detection requires at least two independent signals to avoid false positives on Azure VMs. The /proc/version "microsoft" string is necessary but not sufficient.
**When to use:** PLAT-02 (WSL2 check).
**Example:**
```go
// Source: kind Known Issues + Microsoft WSL documentation

type wsl2Check struct {
    readFile func(string) ([]byte, error)
    getenv   func(string) string
    stat     func(string) (os.FileInfo, error)
}

func newWSL2Check() Check {
    return &wsl2Check{
        readFile: os.ReadFile,
        getenv:   os.Getenv,
        stat:     os.Stat,
    }
}

// isWSL2 returns true only if multiple signals confirm WSL2 environment.
func (c *wsl2Check) isWSL2() bool {
    // Signal 1: /proc/version contains "microsoft" (case-insensitive)
    data, err := c.readFile("/proc/version")
    if err != nil {
        return false
    }
    version := strings.ToLower(string(data))
    if !strings.Contains(version, "microsoft") {
        return false
    }

    // Signal 2: At least one additional WSL2 indicator
    // - $WSL_DISTRO_NAME is set
    if c.getenv("WSL_DISTRO_NAME") != "" {
        return true
    }
    // - /proc/sys/fs/binfmt_misc/WSLInterop exists (or WSLInterop-late on WSL 4.1.4+)
    if _, err := c.stat("/proc/sys/fs/binfmt_misc/WSLInterop"); err == nil {
        return true
    }
    if _, err := c.stat("/proc/sys/fs/binfmt_misc/WSLInterop-late"); err == nil {
        return true
    }

    return false
}
```

### Pattern 3: Security Module Independent Detection
**What:** AppArmor and SELinux checks run independently -- they are NOT mutually exclusive. Both can be active on the same system (LSM stacking since kernel 5.1). Each check reports its own status without assuming the other is absent.
**When to use:** KERN-02 (AppArmor) and KERN-03 (SELinux).
**Important:** Do NOT use `if selinux { ... } else if apparmor { ... }` -- both must run independently.

### Pattern 4: Category Organization for New Checks
**What:** New checks register in the AllChecks() slice with two new categories: "Kernel" and "Platform".
**Recommended registry order:**
```go
var allChecks = []Check{
    // Category: Runtime (existing)
    newContainerRuntimeCheck(),
    // Category: Docker (existing)
    newDiskSpaceCheck(),
    newDaemonJSONCheck(),
    newDockerSnapCheck(),
    newDockerSocketCheck(),
    // Category: Tools (existing)
    newKubectlCheck(),
    newKubectlVersionSkewCheck(),
    // Category: GPU (existing)
    newNvidiaDriverCheck(),
    newNvidiaContainerToolkitCheck(),
    newNvidiaDockerRuntimeCheck(),
    // Category: Kernel (NEW -- Phase 40)
    newInotifyCheck(),          // KERN-01
    newKernelVersionCheck(),    // KERN-04
    // Category: Security (NEW -- Phase 40)
    newApparmorCheck(),         // KERN-02
    newSELinuxCheck(),          // KERN-03
    // Category: Platform (NEW -- Phase 40)
    newFirewalldCheck(),        // PLAT-01
    newWSL2Check(),             // PLAT-02
    newRootfsDeviceCheck(),     // PLAT-04
}
```
**Rationale:** KERN-01 and KERN-04 are kernel-level resource/version checks ("Kernel" category). KERN-02 and KERN-03 are security module checks ("Security" category). PLAT-01, PLAT-02, and PLAT-04 are platform/distro-specific environment checks ("Platform" category).

### Anti-Patterns to Avoid
- **Reading /proc or /sys without platform gating:** The Check interface's `Platforms()` method handles platform filtering centrally. All seven checks return `[]string{"linux"}`. Do NOT add `if runtime.GOOS != "linux"` inside Run() -- the framework handles this.
- **Treating AppArmor and SELinux as mutually exclusive:** Both can be active simultaneously since Linux kernel 5.1 (LSM stacking). Check both independently.
- **Using only /proc/version for WSL2 detection:** Azure VMs use Microsoft-built kernels. Require a second signal ($WSL_DISTRO_NAME or WSLInterop file).
- **Shelling out to getenforce for SELinux detection:** The binary may not exist on all distros. Read `/sys/fs/selinux/enforce` directly -- it is a kernel interface.
- **Importing provider-internal packages:** Doctor checks must NOT import from `pkg/cluster/internal/providers/docker/`. Use injectable `execCmd` to run `docker info` directly.
- **Auto-applying sysctl or setenforce changes:** Per project out-of-scope decisions, never modify system state. Only suggest commands for the user to run.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Kernel version parsing | Custom regex for /proc/version | `unix.Uname()` + simple major.minor integer parsing from Release field | unix.Uname is a direct syscall; Release field has format "5.15.0-91-generic" which is trivially parsed with strings.Cut and strconv.Atoi |
| SELinux status detection | Shelling out to getenforce | Read `/sys/fs/selinux/enforce` (returns "0" or "1") | Kernel sysfs interface is always present when SELinux is compiled in; getenforce binary may not be installed |
| AppArmor enabled detection | Parsing dmesg for AppArmor messages | Read `/sys/module/apparmor/parameters/enabled` (returns "Y" or "N") | Kernel sysfs interface is reliable and does not require root |
| Firewalld backend detection | Running firewall-cmd --get-backend (requires root) | Parse `/etc/firewalld/firewalld.conf` for `FirewallBackend=` | Config file is world-readable; firewall-cmd may require privileges |
| Cgroup v2 detection on WSL2 | Shelling out to stat -fc %T /sys/fs/cgroup | Read `/sys/fs/cgroup/cgroup.controllers` -- if readable, cgroup v2 is mounted | Direct file read; presence and readability of the file confirms cgroup v2 |
| BTRFS filesystem detection | Parsing /proc/mounts with custom parser | `docker info -f '{{.Driver}}'` and `docker info -f '{{json .DriverStatus}}'` | Kind's existing `mountDevMapper()` in docker/util.go uses this exact approach; consistent with how kind detects BTRFS |
| Inotify limit reading | Shelling out to sysctl | Read `/proc/sys/fs/inotify/max_user_watches` and `max_user_instances` | Direct file read from procfs; faster and more reliable than forking sysctl |

**Key insight:** Every check in this phase detects system state by reading pseudo-filesystem files (/proc, /sys) or parsing config files -- no new external tools or libraries needed. The only syscall is `unix.Uname()` for kernel version, and that package is already in go.mod.

## Common Pitfalls

### Pitfall 1: WSL2 False Positive on Azure VMs
**What goes wrong:** Azure VMs and some cloud instances use Microsoft-built Linux kernels. /proc/version contains "microsoft" or "Hyper-V" but the system is NOT WSL2. The WSL2 cgroup check triggers incorrectly, suggesting WSL-specific fixes on a regular Linux VM.
**Why it happens:** Microsoft provides Azure Linux kernels that share kernel branding with WSL2 kernels. Checking /proc/version alone cannot distinguish WSL2 from Azure/Hyper-V VMs.
**How to avoid:** Require TWO signals for WSL2 confirmation: (1) /proc/version contains "microsoft" (case-insensitive) AND (2) either $WSL_DISTRO_NAME is set OR /proc/sys/fs/binfmt_misc/WSLInterop (or WSLInterop-late) exists. Azure VMs have signal 1 but NOT signal 2.
**Warning signs:** WSL2 check warns about cgroup configuration on GitHub Actions runners (which are Azure-hosted VMs).

### Pitfall 2: AppArmor and SELinux Can Coexist
**What goes wrong:** Code uses `if selinux { } else if apparmor { }` assuming mutual exclusivity. On openSUSE Tumbleweed (2025+) or systems with LSM stacking (kernel 5.1+), both can be active simultaneously. The else-if skips one of them.
**Why it happens:** Historically, distros shipped one MAC system. Since kernel 5.1, LSM stacking allows both. openSUSE switched from AppArmor to SELinux in 2025, and transitional systems may have both.
**How to avoid:** Implement AppArmor and SELinux as completely independent checks. Both run, both report. Never assume one excludes the other.
**Warning signs:** Test code has `if/else if` logic for security modules; a test for "both active" is missing.

### Pitfall 3: /proc File Reads Fail on Non-Linux
**What goes wrong:** A check reads `/proc/sys/fs/inotify/max_user_watches` without a platform guard. On macOS (where /proc does not exist), the check fails with a confusing "no such file or directory" error instead of a clean "skip".
**Why it happens:** Go compiles file reads on all platforms. The Platforms() method returns `[]string{"linux"}` which should prevent Run() from being called on non-Linux, but the constructor (newXCheck) may read files during initialization.
**How to avoid:** Never read /proc or /sys files in the constructor. Only read them inside Run(). The RunAllChecks() framework skips checks whose Platforms() don't match the current OS, so Run() is never called on non-Linux.
**Warning signs:** `go test ./pkg/internal/doctor/... -count=1` passes on Linux but fails on macOS due to /proc file access in constructors.

### Pitfall 4: Inotify Values Inside Docker Desktop VM
**What goes wrong:** On macOS/Windows with Docker Desktop, inotify limits are from Docker Desktop's Linux VM, not the host. Users cannot fix them with macOS `sysctl`. The fix suggestion is wrong for Docker Desktop users.
**Why it happens:** kinder doctor runs on the host OS. On macOS, /proc does not exist, so the inotify check is correctly skipped (Platforms = linux). However, if someone runs kinder inside a Docker container or inside the Docker Desktop VM, the values are the VM's values, not the host's.
**How to avoid:** The check is Linux-only, so it will only run on actual Linux hosts or inside WSL2/Docker Desktop VM. The fix suggestion should include the sysctl command. Since the check targets Linux-native users (the platform gating handles Docker Desktop/macOS), this is acceptable behavior.
**Warning signs:** Docker Desktop macOS users somehow trigger the check and get inotify fix suggestions.

### Pitfall 5: SELinux /dev/dma_heap Issue Is Fedora-Specific
**What goes wrong:** The check warns about SELinux enforcing on ALL Linux distros, causing unnecessary alarm. The actual kind issue (docker: Error response from daemon: open /dev/dma_heap: permission denied) is specific to Fedora 33's SELinux policy.
**Why it happens:** Overly broad detection -- checking SELinux enforcing without considering distro context.
**How to avoid:** Check SELinux enforcing mode AND detect Fedora via /etc/os-release. Only warn if SELinux is enforcing AND the distro is Fedora. On RHEL/CentOS/other SELinux distros, SELinux enforcing is normal and does not cause kind issues.
**Warning signs:** Ubuntu users with SELinux installed get false warnings about kind failures.

### Pitfall 6: firewalld.conf Absent on Non-Fedora Systems
**What goes wrong:** The check tries to read /etc/firewalld/firewalld.conf on Ubuntu/Debian where firewalld is not installed. The file read error is treated as a check failure instead of a skip.
**Why it happens:** Assuming firewalld is universally installed on Linux.
**How to avoid:** First check if firewall-cmd exists via lookPath. If not found, skip the check with "(firewalld not installed)". If found, then check if firewalld is running (firewall-cmd --state), then parse the config file.
**Warning signs:** Ubuntu users get errors about missing firewalld configuration files.

### Pitfall 7: BTRFS Detection Requires Docker Running
**What goes wrong:** The rootfs device check queries `docker info` to detect the storage driver and backing filesystem. If Docker is not running, the check fails or errors instead of skipping gracefully.
**Why it happens:** Docker info requires a running daemon.
**How to avoid:** If docker info fails (daemon not running or docker not found), skip the check with an informative message. The container-runtime check already handles the "docker not running" case; this check focuses on detecting BTRFS when Docker IS running.
**Warning signs:** The rootfs check fails when Docker is stopped, duplicating the container-runtime check's error.

### Pitfall 8: WSLInterop File Renamed in WSL 4.1.4+
**What goes wrong:** The WSL2 detection checks for /proc/sys/fs/binfmt_misc/WSLInterop, but newer WSL versions (4.1.4+) renamed this to WSLInterop-late. The check misses WSL2 on newer Windows builds.
**Why it happens:** Microsoft changed the binfmt_misc registration name.
**How to avoid:** Check for BOTH /proc/sys/fs/binfmt_misc/WSLInterop AND /proc/sys/fs/binfmt_misc/WSLInterop-late. Either file existing counts as a positive signal.
**Warning signs:** WSL2 detection fails on Windows 11 24H2+ despite $WSL_DISTRO_NAME being set (multi-signal approach still catches it via env var).

## Code Examples

Verified patterns from official sources and the existing codebase:

### KERN-01: Inotify Limits Check
```go
// Source: kind Known Issues page + /proc/sys/fs/inotify/ kernel documentation

type inotifyCheck struct {
    readFile func(string) ([]byte, error)
}

func newInotifyCheck() Check {
    return &inotifyCheck{
        readFile: os.ReadFile,
    }
}

func (c *inotifyCheck) Name() string       { return "inotify-limits" }
func (c *inotifyCheck) Category() string    { return "Kernel" }
func (c *inotifyCheck) Platforms() []string { return []string{"linux"} }

func (c *inotifyCheck) Run() []Result {
    watches, watchesErr := c.readSysctl("/proc/sys/fs/inotify/max_user_watches")
    instances, instancesErr := c.readSysctl("/proc/sys/fs/inotify/max_user_instances")

    if watchesErr != nil && instancesErr != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: "Could not read inotify limits",
            Reason:  "Unable to read /proc/sys/fs/inotify/ files",
            Fix:     "Check manually: cat /proc/sys/fs/inotify/max_user_watches",
        }}
    }

    var results []Result

    if watchesErr == nil {
        if watches < 524288 {
            results = append(results, Result{
                Name: c.Name(), Category: c.Category(), Status: "warn",
                Message: fmt.Sprintf("max_user_watches=%d (recommended: >=524288)", watches),
                Reason:  "Low inotify watches can cause 'too many open files' errors in kind clusters",
                Fix:     "sudo sysctl fs.inotify.max_user_watches=524288",
            })
        } else {
            results = append(results, Result{
                Name: c.Name(), Category: c.Category(), Status: "ok",
                Message: fmt.Sprintf("max_user_watches=%d", watches),
            })
        }
    }

    if instancesErr == nil {
        if instances < 512 {
            results = append(results, Result{
                Name: c.Name(), Category: c.Category(), Status: "warn",
                Message: fmt.Sprintf("max_user_instances=%d (recommended: >=512)", instances),
                Reason:  "Low inotify instances can cause 'too many open files' errors in kind clusters",
                Fix:     "sudo sysctl fs.inotify.max_user_instances=512",
            })
        }
        // If instances is ok and watches was ok, don't duplicate the ok message
    }

    if len(results) == 0 {
        results = append(results, Result{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: fmt.Sprintf("max_user_watches=%d, max_user_instances=%d", watches, instances),
        })
    }

    return results
}

func (c *inotifyCheck) readSysctl(path string) (int, error) {
    data, err := c.readFile(path)
    if err != nil {
        return 0, err
    }
    return strconv.Atoi(strings.TrimSpace(string(data)))
}
```

### KERN-02: AppArmor Detection
```go
// Source: kind Known Issues (moby/moby#7512) + /sys/module/apparmor/ kernel docs

type apparmorCheck struct {
    readFile func(string) ([]byte, error)
    execCmd  func(name string, args ...string) exec.Cmd
    lookPath func(string) (string, error)
}

func newApparmorCheck() Check {
    return &apparmorCheck{
        readFile: os.ReadFile,
        execCmd:  exec.Command,
        lookPath: osexec.LookPath,
    }
}

func (c *apparmorCheck) Name() string       { return "apparmor" }
func (c *apparmorCheck) Category() string    { return "Security" }
func (c *apparmorCheck) Platforms() []string { return []string{"linux"} }

func (c *apparmorCheck) Run() []Result {
    // Check if AppArmor kernel module is enabled
    data, err := c.readFile("/sys/module/apparmor/parameters/enabled")
    if err != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: "AppArmor not enabled",
        }}
    }

    enabled := strings.TrimSpace(string(data))
    if enabled != "Y" {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: "AppArmor module loaded but not enabled",
        }}
    }

    // AppArmor is enabled -- warn about potential interference with kind
    return []Result{{
        Name: c.Name(), Category: c.Category(), Status: "warn",
        Message: "AppArmor is enabled",
        Reason:  "AppArmor profiles may interfere with kind container operations (see moby/moby#7512)",
        Fix:     "If cluster creation fails, try: sudo aa-remove-unknown",
    }}
}
```

### KERN-03: SELinux Enforcing Mode Detection
```go
// Source: kind Known Issues (Fedora 33 /dev/dma_heap) + /sys/fs/selinux/ kernel docs

type selinuxCheck struct {
    readFile func(string) ([]byte, error)
}

func newSELinuxCheck() Check {
    return &selinuxCheck{
        readFile: os.ReadFile,
    }
}

func (c *selinuxCheck) Name() string       { return "selinux" }
func (c *selinuxCheck) Category() string    { return "Security" }
func (c *selinuxCheck) Platforms() []string { return []string{"linux"} }

func (c *selinuxCheck) Run() []Result {
    data, err := c.readFile("/sys/fs/selinux/enforce")
    if err != nil {
        // SELinux not available -- this is fine
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: "SELinux not available",
        }}
    }

    enforcing := strings.TrimSpace(string(data)) == "1"
    if !enforcing {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: "SELinux permissive or disabled",
        }}
    }

    // SELinux is enforcing -- check if this is Fedora
    if c.isFedora() {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: "SELinux enforcing on Fedora",
            Reason:  "SELinux enforcing on Fedora may block kind with: open /dev/dma_heap: permission denied",
            Fix:     "Temporary fix: sudo setenforce 0 (reverts on reboot)",
        }}
    }

    return []Result{{
        Name: c.Name(), Category: c.Category(), Status: "ok",
        Message: "SELinux enforcing (no known kind issues on this distro)",
    }}
}

func (c *selinuxCheck) isFedora() bool {
    data, err := c.readFile("/etc/os-release")
    if err != nil {
        return false
    }
    content := strings.ToLower(string(data))
    return strings.Contains(content, "id=fedora")
}
```

### KERN-04: Kernel Version Check
```go
// Source: kind Known Issues (cgroup namespace) + cgroup_namespaces(7) man page

type kernelVersionCheck struct {
    uname func(buf *unix.Utsname) error
}

func newKernelVersionCheck() Check {
    return &kernelVersionCheck{
        uname: unix.Uname,
    }
}

func (c *kernelVersionCheck) Name() string       { return "kernel-version" }
func (c *kernelVersionCheck) Category() string    { return "Kernel" }
func (c *kernelVersionCheck) Platforms() []string { return []string{"linux"} }

func (c *kernelVersionCheck) Run() []Result {
    var buf unix.Utsname
    if err := c.uname(&buf); err != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: "Could not determine kernel version",
            Reason:  fmt.Sprintf("uname failed: %v", err),
            Fix:     "Check manually: uname -r",
        }}
    }

    release := unix.ByteSliceToString(buf.Release[:])
    major, minor, err := parseKernelVersion(release)
    if err != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: fmt.Sprintf("Could not parse kernel version %q", release),
            Reason:  fmt.Sprintf("Parse error: %v", err),
            Fix:     "Check manually: uname -r",
        }}
    }

    if major < 4 || (major == 4 && minor < 6) {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "fail",
            Message: fmt.Sprintf("Kernel %d.%d (from %s)", major, minor, release),
            Reason:  "Kernel 4.6+ required for cgroup namespace support (CLONE_NEWCGROUP)",
            Fix:     "Upgrade your kernel or OS to get kernel 4.6+",
        }}
    }

    return []Result{{
        Name: c.Name(), Category: c.Category(), Status: "ok",
        Message: fmt.Sprintf("Kernel %d.%d (%s)", major, minor, release),
    }}
}

// parseKernelVersion extracts major and minor from a release string like "5.15.0-91-generic".
func parseKernelVersion(release string) (major, minor int, err error) {
    // Split on "." and parse first two segments
    parts := strings.SplitN(release, ".", 3)
    if len(parts) < 2 {
        return 0, 0, fmt.Errorf("unexpected kernel release format: %s", release)
    }
    major, err = strconv.Atoi(parts[0])
    if err != nil {
        return 0, 0, fmt.Errorf("invalid major version %q: %w", parts[0], err)
    }
    // Minor may contain non-numeric suffix after a dash (e.g., "15" from "15.0-91-generic")
    minorStr := parts[1]
    if idx := strings.IndexAny(minorStr, "-+_"); idx >= 0 {
        minorStr = minorStr[:idx]
    }
    minor, err = strconv.Atoi(minorStr)
    if err != nil {
        return 0, 0, fmt.Errorf("invalid minor version %q: %w", parts[1], err)
    }
    return major, minor, nil
}
```

**IMPORTANT NOTE on kernel.go build tags:** The `kernelVersionCheck` struct uses `unix.Uname` which is Linux-only. This file MUST have a build tag `//go:build linux` and a corresponding `kernel_other.go` with `//go:build !linux` that provides a stub constructor. The Platforms() method returns `[]string{"linux"}` so Run() is never called on non-Linux, but the struct must compile on all platforms. The stub approach matches the existing `disk_unix.go` / `disk_other.go` pattern.

```go
// kernel_other.go
//go:build !linux

package doctor

// kernelVersionCheck stub for non-Linux platforms.
type kernelVersionCheck struct{}

func newKernelVersionCheck() Check {
    return &kernelVersionCheck{}
}

func (c *kernelVersionCheck) Name() string       { return "kernel-version" }
func (c *kernelVersionCheck) Category() string    { return "Kernel" }
func (c *kernelVersionCheck) Platforms() []string { return []string{"linux"} }
func (c *kernelVersionCheck) Run() []Result       { return nil }
```

### PLAT-01: Firewalld nftables Backend Detection
```go
// Source: kind Known Issues (Fedora 32+ nftables) + /etc/firewalld/firewalld.conf

type firewalldCheck struct {
    readFile func(string) ([]byte, error)
    lookPath func(string) (string, error)
    execCmd  func(name string, args ...string) exec.Cmd
}

func newFirewalldCheck() Check {
    return &firewalldCheck{
        readFile: os.ReadFile,
        lookPath: osexec.LookPath,
        execCmd:  exec.Command,
    }
}

func (c *firewalldCheck) Name() string       { return "firewalld-backend" }
func (c *firewalldCheck) Category() string    { return "Platform" }
func (c *firewalldCheck) Platforms() []string { return []string{"linux"} }

func (c *firewalldCheck) Run() []Result {
    // Check if firewall-cmd exists
    if _, err := c.lookPath("firewall-cmd"); err != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: "firewalld not installed",
        }}
    }

    // Check if firewalld is running
    lines, err := exec.CombinedOutputLines(c.execCmd("firewall-cmd", "--state"))
    if err != nil || len(lines) == 0 || strings.TrimSpace(lines[0]) != "running" {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: "firewalld not running",
        }}
    }

    // Parse /etc/firewalld/firewalld.conf for FirewallBackend
    data, err := c.readFile("/etc/firewalld/firewalld.conf")
    if err != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: "firewalld running (config not readable)",
        }}
    }

    backend := "nftables" // default since Fedora 32
    for _, line := range strings.Split(string(data), "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "#") || line == "" {
            continue
        }
        if strings.HasPrefix(line, "FirewallBackend=") {
            backend = strings.TrimPrefix(line, "FirewallBackend=")
            break
        }
    }

    if strings.ToLower(backend) == "nftables" {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: "firewalld using nftables backend",
            Reason:  "nftables backend may prevent Docker containers from reaching each other",
            Fix:     "Change FirewallBackend=iptables in /etc/firewalld/firewalld.conf and run: sudo systemctl restart firewalld",
        }}
    }

    return []Result{{
        Name: c.Name(), Category: c.Category(), Status: "ok",
        Message: fmt.Sprintf("firewalld using %s backend", backend),
    }}
}
```

### PLAT-02: WSL2 Detection and Cgroup v2 Check
```go
// Source: kind Known Issues (WSL2 cgroup) + kind WSL2 docs

func (c *wsl2Check) Run() []Result {
    if !c.isWSL2() {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: "Not running under WSL2",
        }}
    }

    // WSL2 confirmed -- check cgroup v2 controller availability
    data, err := c.readFile("/sys/fs/cgroup/cgroup.controllers")
    if err != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: "WSL2 detected but cgroup v2 controllers not available",
            Reason:  "Missing cgroup v2 controllers may cause 'error adding pid to cgroups' during cluster creation",
            Fix:     "See https://github.com/spurin/wsl-cgroupsv2 for WSL2 cgroup v2 configuration",
        }}
    }

    controllers := strings.TrimSpace(string(data))
    // Check for essential controllers: cpu, memory, pids
    required := []string{"cpu", "memory", "pids"}
    var missing []string
    for _, req := range required {
        if !strings.Contains(controllers, req) {
            missing = append(missing, req)
        }
    }

    if len(missing) > 0 {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: fmt.Sprintf("WSL2: cgroup v2 missing controllers: %s", strings.Join(missing, ", ")),
            Reason:  "Missing cgroup v2 controllers may cause cluster creation failures",
            Fix:     "See https://github.com/spurin/wsl-cgroupsv2 for WSL2 cgroup v2 configuration",
        }}
    }

    return []Result{{
        Name: c.Name(), Category: c.Category(), Status: "ok",
        Message: fmt.Sprintf("WSL2: cgroup v2 controllers available (%s)", controllers),
    }}
}
```

### PLAT-04: Rootfs Device Node Check
```go
// Source: kind Known Issues (BTRFS/NVMe rootfs) + kind issue #2411, #2555

type rootfsDeviceCheck struct {
    lookPath func(string) (string, error)
    execCmd  func(name string, args ...string) exec.Cmd
}

func newRootfsDeviceCheck() Check {
    return &rootfsDeviceCheck{
        lookPath: osexec.LookPath,
        execCmd:  exec.Command,
    }
}

func (c *rootfsDeviceCheck) Name() string       { return "rootfs-device" }
func (c *rootfsDeviceCheck) Category() string    { return "Platform" }
func (c *rootfsDeviceCheck) Platforms() []string { return []string{"linux"} }

func (c *rootfsDeviceCheck) Run() []Result {
    if _, err := c.lookPath("docker"); err != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "skip",
            Message: "(docker not found)",
        }}
    }

    // Check Docker storage driver
    lines, err := exec.OutputLines(c.execCmd("docker", "info", "-f", "{{.Driver}}"))
    if err != nil || len(lines) == 0 {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "skip",
            Message: "(could not query Docker storage driver)",
        }}
    }

    driver := strings.ToLower(strings.TrimSpace(lines[0]))
    if driver == "btrfs" {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: "Docker storage driver is btrfs",
            Reason:  "BTRFS may cause kubelet 'Failed to get rootfs info' errors; you may need extraMounts in cluster config",
            Fix:     "See https://kind.sigs.k8s.io/docs/user/known-issues/#failure-to-build-node-image",
        }}
    }

    // Check backing filesystem via DriverStatus
    lines, err = exec.OutputLines(c.execCmd("docker", "info", "-f", "{{json .DriverStatus}}"))
    if err == nil && len(lines) > 0 {
        output := strings.ToLower(lines[0])
        if strings.Contains(output, "btrfs") {
            return []Result{{
                Name: c.Name(), Category: c.Category(), Status: "warn",
                Message: "Docker backing filesystem is btrfs",
                Reason:  "BTRFS backing filesystem may cause kubelet 'Failed to get rootfs info' errors; you may need extraMounts",
                Fix:     "See https://kind.sigs.k8s.io/docs/user/known-issues/#failure-to-build-node-image",
            }}
        }
    }

    return []Result{{
        Name: c.Name(), Category: c.Category(), Status: "ok",
        Message: fmt.Sprintf("Docker storage driver: %s", driver),
    }}
}
```

### Test Pattern: File-Based Checks with Injectable readFile
```go
// Source: Extension of Phase 38-39 test patterns

func TestInotifyCheck_Run(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name           string
        files          map[string]string // path -> content
        wantStatus     string
        wantMsgContain string
    }{
        {
            name: "both limits ok",
            files: map[string]string{
                "/proc/sys/fs/inotify/max_user_watches":   "524288\n",
                "/proc/sys/fs/inotify/max_user_instances": "512\n",
            },
            wantStatus:     "ok",
            wantMsgContain: "524288",
        },
        {
            name: "watches too low",
            files: map[string]string{
                "/proc/sys/fs/inotify/max_user_watches":   "8192\n",
                "/proc/sys/fs/inotify/max_user_instances": "512\n",
            },
            wantStatus:     "warn",
            wantMsgContain: "8192",
        },
        {
            name: "instances too low",
            files: map[string]string{
                "/proc/sys/fs/inotify/max_user_watches":   "524288\n",
                "/proc/sys/fs/inotify/max_user_instances": "128\n",
            },
            wantStatus:     "warn",
            wantMsgContain: "128",
        },
        {
            name: "both files unreadable",
            files: map[string]string{},
            wantStatus:     "warn",
            wantMsgContain: "Could not read",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            check := &inotifyCheck{
                readFile: func(path string) ([]byte, error) {
                    if content, ok := tt.files[path]; ok {
                        return []byte(content), nil
                    }
                    return nil, os.ErrNotExist
                },
            }
            results := check.Run()
            if len(results) == 0 {
                t.Fatal("expected at least 1 result, got 0")
            }
            // Check first result status
            gotStatus := results[0].Status
            for _, r := range results {
                if r.Status == "warn" || r.Status == "fail" {
                    gotStatus = r.Status
                    break
                }
            }
            if gotStatus != tt.wantStatus {
                t.Errorf("Status = %q, want %q", gotStatus, tt.wantStatus)
            }
        })
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No inotify check | Read /proc/sys/fs/inotify/ with 524288/512 thresholds | Phase 40 (now) | Catches "too many open files" before cluster creation |
| No AppArmor detection | Read /sys/module/apparmor/parameters/enabled | Phase 40 (now) | Warns about container interference proactively |
| No SELinux detection | Read /sys/fs/selinux/enforce + Fedora detection | Phase 40 (now) | Catches Fedora 33 /dev/dma_heap denial |
| No kernel version check | unix.Uname() with 4.6 threshold | Phase 40 (now) | Hard blocker detection for RHEL 7 |
| No firewalld detection | Parse /etc/firewalld/firewalld.conf | Phase 40 (now) | Catches nftables networking breakage on Fedora 32+ |
| No WSL2 detection | Multi-signal /proc/version + env var + WSLInterop | Phase 40 (now) | Catches cgroup v2 misconfiguration on WSL2 |
| No BTRFS detection | docker info for storage driver/backing fs | Phase 40 (now) | Warns about rootfs device node access issues |
| Single detection for WSL2 | Multi-signal approach (2+ signals required) | 2025 best practice | Avoids false positives on Azure VMs |
| Mutual exclusion for AppArmor/SELinux | Independent detection (both checked) | Linux 5.1+ LSM stacking | Handles systems with both security modules |

**Deprecated/outdated:**
- Checking only /proc/version for WSL2 detection: Use multi-signal approach since Azure VMs contain "microsoft" in /proc/version
- Checking /proc/sys/fs/binfmt_misc/WSLInterop only: WSL 4.1.4+ renamed to WSLInterop-late; check both
- Assuming AppArmor and SELinux are mutually exclusive: LSM stacking since kernel 5.1 allows both

## Open Questions

1. **KERN-01: Should inotify check return one or two Results?**
   - What we know: The check reads two independent values (max_user_watches and max_user_instances). They can independently be low.
   - What's unclear: Should the check return a single combined result or two separate results (one per value)?
   - Recommendation: Return a single combined result when both are ok. When either is low, return one result per problematic value with its specific fix command. This matches how users need to fix them (separate sysctl commands).

2. **KERN-02: How detailed should AppArmor profile detection be?**
   - What we know: Kind's Known Issues page says "disable apparmor on your host or at least any profile(s) related to applications you are trying to run in KIND." Running aa-status requires root.
   - What's unclear: Should we try to list specific interfering profiles, or just warn that AppArmor is enabled?
   - Recommendation: Just detect if AppArmor is enabled (/sys/module/apparmor/parameters/enabled == "Y") and warn. Do NOT try to run aa-status (requires root) or parse profile lists. The warning should point users to the kind Known Issues page for details.

3. **KERN-04: Should kernel_test.go be build-tagged?**
   - What we know: kernel.go needs `//go:build linux` because it imports `golang.org/x/sys/unix`. Tests that construct the struct directly would also need the build tag.
   - What's unclear: Should tests be Linux-only, or should they test the non-Linux stub?
   - Recommendation: Create kernel_linux_test.go with `//go:build linux` for tests that exercise the real check logic (including parseKernelVersion). The parseKernelVersion function can be in a separate non-tagged file if it does not import unix, making it testable on all platforms.

4. **PLAT-02: What cgroup v2 controllers are "essential" for kind?**
   - What we know: Kind requires cpu, memory, and pids controllers at minimum. The io controller is used by cgroup v2 but may not be strictly required.
   - What's unclear: The exact minimum set of controllers kind needs.
   - Recommendation: Check for cpu, memory, and pids. These are the controllers most commonly missing in broken WSL2 configurations.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None needed -- `go test ./...` works |
| Quick run command | `go test ./pkg/internal/doctor/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| KERN-01 | Inotify watches warn <524288, instances warn <512, ok when both sufficient | unit | `go test ./pkg/internal/doctor/... -run TestInotifyCheck -v` | No -- Wave 0 |
| KERN-02 | AppArmor enabled detection, ok when not enabled | unit | `go test ./pkg/internal/doctor/... -run TestApparmorCheck -v` | No -- Wave 0 |
| KERN-03 | SELinux enforcing on Fedora warns, non-Fedora ok, permissive ok | unit | `go test ./pkg/internal/doctor/... -run TestSELinuxCheck -v` | No -- Wave 0 |
| KERN-04 | Kernel <4.6 fails, >=4.6 ok, parse error warns | unit | `go test ./pkg/internal/doctor/... -run TestKernelVersionCheck -v` | No -- Wave 0 |
| PLAT-01 | Firewalld nftables backend warns, iptables ok, not installed ok | unit | `go test ./pkg/internal/doctor/... -run TestFirewalldCheck -v` | No -- Wave 0 |
| PLAT-02 | WSL2 multi-signal detection, cgroup v2 controller check, Azure VM no false positive | unit | `go test ./pkg/internal/doctor/... -run TestWSL2Check -v` | No -- Wave 0 |
| PLAT-04 | BTRFS storage driver warns, ext4/overlay2 ok, docker not found skips | unit | `go test ./pkg/internal/doctor/... -run TestRootfsDeviceCheck -v` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./pkg/internal/doctor/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before verify-work

### Wave 0 Gaps
- [ ] `pkg/internal/doctor/inotify_test.go` -- covers KERN-01 (inotify limit thresholds, unreadable files, combined output)
- [ ] `pkg/internal/doctor/apparmor_test.go` -- covers KERN-02 (enabled/disabled detection, module not loaded)
- [ ] `pkg/internal/doctor/selinux_test.go` -- covers KERN-03 (enforcing/permissive/disabled, Fedora vs non-Fedora)
- [ ] `pkg/internal/doctor/kernel_test.go` -- covers KERN-04 (version parsing, threshold comparison, parse errors)
- [ ] `pkg/internal/doctor/firewalld_test.go` -- covers PLAT-01 (nftables/iptables backend, not installed, not running)
- [ ] `pkg/internal/doctor/wsl2_test.go` -- covers PLAT-02 (multi-signal WSL2 detection, cgroup v2 controllers, Azure VM false positive)
- [ ] `pkg/internal/doctor/rootfs_test.go` -- covers PLAT-04 (btrfs driver, btrfs backing fs, non-btrfs ok, docker not found)
- [ ] Build-tagged `kernel.go` (or kernel_linux.go) and `kernel_other.go` must both compile -- verified by `go build ./...` on CI

## Sources

### Primary (HIGH confidence)
- Kind Known Issues page: https://kind.sigs.k8s.io/docs/user/known-issues/ -- ground truth for all 7 checks (inotify limits, AppArmor, SELinux on Fedora, kernel 4.6 requirement, firewalld nftables, WSL2 cgroups, BTRFS rootfs)
- cgroup_namespaces(7) man page: https://man7.org/linux/man-pages/man7/cgroup_namespaces.7.html -- confirms kernel 4.6 requirement for CLONE_NEWCGROUP
- Fedora firewalld nftables change: https://fedoraproject.org/wiki/Changes/firewalld_default_to_nftables -- confirms Fedora 32+ default
- Kind WSL2 documentation: https://kind.sigs.k8s.io/docs/user/using-wsl2/ -- WSL2 cgroup configuration guidance
- Kind issue #3685: https://github.com/kubernetes-sigs/kind/issues/3685 -- WSL2 cgroup v2 configuration details
- Kind issue #2555: https://github.com/kubernetes-sigs/kind/issues/2555 -- BTRFS rootfs failure on Fedora 35
- Kind issue #2411: https://github.com/kubernetes-sigs/kind/issues/2411 -- BTRFS + LUKS device node access
- golang.org/x/sys/unix Uname docs: https://pkg.go.dev/golang.org/x/sys/unix -- Utsname struct, ByteSliceToString helper
- Direct codebase analysis: `pkg/internal/doctor/` -- Phase 38-39 Check interface, deps struct pattern, fakeCmd test doubles
- Direct codebase analysis: `pkg/cluster/internal/providers/docker/util.go` -- mountDevMapper() BTRFS detection pattern
- Prior project research: `.planning/research/FEATURES.md` -- check specifications for all 7 checks
- Prior project research: `.planning/research/PITFALLS.md` -- WSL2 false positives, AppArmor/SELinux coexistence, /proc reads on non-Linux

### Secondary (MEDIUM confidence)
- WSL2 cgroupsv2 fix guide: https://github.com/spurin/wsl-cgroupsv2 -- community solution for WSL2 cgroup configuration
- Docker AppArmor docs: https://docs.docker.com/engine/security/apparmor/ -- docker-default profile behavior
- Docker nftables docs: https://docs.docker.com/engine/network/firewall-nftables/ -- Docker 29.0.0+ nftables support status
- Kubernetes inotify issue #46230: https://github.com/kubernetes/kubernetes/issues/46230 -- inotify limit requirements
- SELinux sysfs interface: Red Hat SELinux docs -- /sys/fs/selinux/enforce returns "0" or "1"

### Tertiary (LOW confidence)
- WSLInterop-late rename in WSL 4.1.4+: Ubuntu WSL documentation -- needs validation on actual WSL 4.1.4+ system

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- zero new external deps; golang.org/x/sys/unix already direct at v0.41.0; all detection uses Go stdlib file reads
- Architecture: HIGH -- exact same pattern as Phase 38-39 (deps struct, fakeCmd/readFile injection, AllChecks registry); no architectural innovation needed
- Pitfalls: HIGH -- all pitfalls identified from prior research (PITFALLS.md, FEATURES.md) and verified against kind Known Issues page and codebase analysis; WSL2 false positive prevention verified against multiple sources
- Code examples: HIGH -- patterns directly extend Phase 38-39 implementations; /proc and /sys file paths verified against kernel documentation

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable domain -- kernel interfaces, /proc layout, kind Known Issues are mature)
