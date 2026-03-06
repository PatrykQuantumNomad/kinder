---
phase: 40-kernel-security-and-platform-specific-checks
verified: 2026-03-06T18:45:00Z
status: passed
score: 5/5 must-haves verified
---

# Phase 40: Kernel, Security, and Platform-Specific Checks Verification Report

**Phase Goal:** Linux users running `kinder doctor` get advance warnings about kernel limits, security modules, and platform configurations that would silently break or degrade kind clusters
**Verified:** 2026-03-06T18:45:00Z
**Status:** passed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `kinder doctor` on Linux checks inotify max_user_watches (warns if <524288) and max_user_instances (warns if <512), and suggests the exact sysctl commands to fix them | VERIFIED | inotify.go lines 80-99: threshold checks against minWatches=524288 and minInstances=512 with Fix strings `sudo sysctl fs.inotify.max_user_watches=524288` and `sudo sysctl fs.inotify.max_user_instances=512`. 7 table-driven test cases in inotify_test.go cover ok, watches-low, instances-low, both-low, unreadable scenarios. |
| 2 | `kinder doctor` on Linux detects AppArmor profiles that interfere with kind containers and SELinux enforcing mode on Fedora, checking both independently (not mutually exclusive) | VERIFIED | apparmor.go and selinux.go are completely separate files with separate Check implementations. Both are registered independently in allChecks (check.go lines 73-74). AppArmor warns when enabled=Y with moby/moby#7512 reference and aa-remove-unknown fix. SELinux warns only on Fedora+enforcing via isFedora() helper reading /etc/os-release. 3 test cases for AppArmor, 5 for SELinux. |
| 3 | `kinder doctor` on Linux checks kernel version and warns if below 4.6 (no cgroup namespace support), which is a hard blocker for kind | VERIFIED | kernel_linux.go line 79: threshold check `major < 4 || (major == 4 && minor < 6)` returns status "fail" (not warn) with reason containing "cgroup namespace". parseKernelVersion handles release strings with suffixes (-generic, -azure, -WSL2). 6 Run tests + 10 parse tests in kernel_linux_test.go cover ok (5.15), boundary (4.6), fail (4.5, 3.10), error cases. kernel_other.go stub compiles on macOS. |
| 4 | `kinder doctor` on Linux detects firewalld with nftables backend on Fedora 32+ and warns about networking issues | VERIFIED | firewalld.go implements 3-step detection: lookPath for firewall-cmd, exec for --state, readFile for /etc/firewalld/firewalld.conf. Defaults backend to nftables when line absent (Fedora 32+ default, line 83). Warns with Fix suggesting iptables + systemctl restart. 6 test cases cover all states. |
| 5 | `kinder doctor` detects WSL2 using multi-signal approach (/proc/version + $WSL_DISTRO_NAME or /proc/sys/fs/binfmt_misc/WSLInterop) and checks cgroup v2 configuration, without false-positiving on Azure VMs | VERIFIED | wsl2.go isWSL2() method (lines 119-146) requires /proc/version containing "microsoft" (case-insensitive) AND at least one of: WSL_DISTRO_NAME env var, WSLInterop file, or WSLInterop-late file. Azure VM test case explicitly verifies "microsoft" in /proc/version without second signal returns false. cgroup v2 check verifies cpu, memory, pids controllers via /sys/fs/cgroup/cgroup.controllers. 10 test cases including Azure VM false positive prevention. |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/internal/doctor/inotify.go` | inotifyCheck with readFile dep, Name=inotify-limits, Category=Kernel | VERIFIED | 123 lines, substantive implementation with readSysctl helper, threshold constants, multi-result logic |
| `pkg/internal/doctor/inotify_test.go` | Table-driven tests for inotify thresholds | VERIFIED | 175 lines, 7 test cases, fakeReadFile helper definition |
| `pkg/internal/doctor/kernel_linux.go` | kernelVersionCheck with unix.Uname dep, parseKernelVersion helper | VERIFIED | 128 lines, build-tagged `//go:build linux`, imports golang.org/x/sys/unix, parseKernelVersion handles all suffix formats |
| `pkg/internal/doctor/kernel_other.go` | Stub kernelVersionCheck for non-Linux compilation | VERIFIED | 35 lines, build-tagged `//go:build !linux`, stub with nil return (intentional for platform filtering) |
| `pkg/internal/doctor/kernel_linux_test.go` | Tests for kernel version parsing and threshold comparison | VERIFIED | 197 lines, build-tagged `//go:build linux`, TestKernelVersionCheck_Run (6 cases), TestParseKernelVersion (10 cases) |
| `pkg/internal/doctor/apparmor.go` | apparmorCheck with readFile dep, Category=Security | VERIFIED | 72 lines, reads /sys/module/apparmor/parameters/enabled, warns on Y with moby/moby#7512 reference |
| `pkg/internal/doctor/apparmor_test.go` | Tests for AppArmor enabled/disabled detection | VERIFIED | 110 lines, 3 test cases (enabled/disabled/not-loaded) |
| `pkg/internal/doctor/selinux.go` | selinuxCheck with readFile dep and isFedora helper, Category=Security | VERIFIED | 99 lines, reads /sys/fs/selinux/enforce, isFedora reads /etc/os-release case-insensitively |
| `pkg/internal/doctor/selinux_test.go` | Tests for SELinux enforcing/permissive + Fedora detection | VERIFIED | 132 lines, 5 test cases (enforcing+Fedora, enforcing+Ubuntu, permissive, not-available, unreadable) |
| `pkg/internal/doctor/firewalld.go` | firewalldCheck with readFile/execCmd/lookPath deps, Category=Platform | VERIFIED | 114 lines, 3-step detection with nftables default when line absent |
| `pkg/internal/doctor/firewalld_test.go` | Tests for firewalld backend detection | VERIFIED | 172 lines, 6 test cases including defaults-to-nftables |
| `pkg/internal/doctor/wsl2.go` | wsl2Check with multi-signal WSL2 detection and cgroup v2 check | VERIFIED | 147 lines, 3 injectable deps, isWSL2() multi-signal, cgroup controller verification |
| `pkg/internal/doctor/wsl2_test.go` | Tests for WSL2 multi-signal detection and cgroup v2 controllers | VERIFIED | 243 lines, 10 test cases including Azure VM false positive prevention |
| `pkg/internal/doctor/rootfs.go` | rootfsDeviceCheck with BTRFS detection via docker info | VERIFIED | 106 lines, queries both .Driver and .DriverStatus for BTRFS |
| `pkg/internal/doctor/rootfs_test.go` | Tests for rootfs device BTRFS detection | VERIFIED | 141 lines, 5 test cases (not found, daemon down, overlay2, btrfs driver, btrfs backing fs) |
| `pkg/internal/doctor/check.go` | allChecks registry expanded to 17 with Kernel/Security/Platform categories | VERIFIED | Lines 53-78: 17 entries in correct category order Runtime(1), Docker(4), Tools(2), GPU(3), Kernel(2), Security(2), Platform(3) |
| `pkg/internal/doctor/gpu_test.go` | TestAllChecks_RegisteredOrder updated for 17 checks | VERIFIED | Lines 287-323: expects exactly 17 checks with all 7 new entries in correct order |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| inotify.go | /proc/sys/fs/inotify/ | readFile dependency injection | WIRED | Line 51-52: `c.readFile("/proc/sys/fs/inotify/max_user_watches")` and `max_user_instances` |
| kernel_linux.go | golang.org/x/sys/unix | unix.Uname syscall | WIRED | Line 26: `import "golang.org/x/sys/unix"`, line 55: `c.uname(&buf)` |
| apparmor.go | /sys/module/apparmor/parameters/enabled | readFile dep | WIRED | Line 42: `c.readFile("/sys/module/apparmor/parameters/enabled")` |
| selinux.go | /sys/fs/selinux/enforce | readFile dep | WIRED | Line 42: `c.readFile("/sys/fs/selinux/enforce")` |
| selinux.go | /etc/os-release | isFedora() helper | WIRED | Line 88: `c.readFile("/etc/os-release")`, line 94: case-insensitive `id=fedora` match |
| firewalld.go | /etc/firewalld/firewalld.conf | readFile for FirewallBackend | WIRED | Line 72: `c.readFile("/etc/firewalld/firewalld.conf")`, line 89: `"FirewallBackend="` prefix parsing |
| wsl2.go | /proc/version | readFile for microsoft detection | WIRED | Line 121: `c.readFile("/proc/version")`, line 125: `strings.Contains(strings.ToLower(...), "microsoft")` |
| wsl2.go | WSLInterop | stat for file existence | WIRED | Line 135: `c.stat("/proc/sys/fs/binfmt_misc/WSLInterop")`, line 140: `WSLInterop-late` |
| wsl2.go | cgroup.controllers | readFile for cgroup v2 | WIRED | Line 71: `c.readFile("/sys/fs/cgroup/cgroup.controllers")` |
| rootfs.go | docker info | execCmd for storage driver | WIRED | Line 58: `c.execCmd("docker", "info", "-f", "{{.Driver}}")`, line 83: `{{json .DriverStatus}}` |
| check.go | all 7 new check constructors | allChecks registry | WIRED | Lines 69-77: all 7 constructors called and assigned to allChecks slice |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| KERN-01 | 40-01 | Doctor checks inotify max_user_watches (>=524288) and max_user_instances (>=512) | SATISFIED | inotify.go: thresholds match spec exactly, sysctl fix commands in results |
| KERN-02 | 40-02 | Doctor detects AppArmor profiles interfering with kind containers | SATISFIED | apparmor.go: detects enabled state, warns with moby/moby#7512 and aa-remove-unknown fix |
| KERN-03 | 40-02 | Doctor detects SELinux enforcing mode on Fedora | SATISFIED | selinux.go: enforcing+Fedora=warn with /dev/dma_heap reason and setenforce fix |
| KERN-04 | 40-01 | Doctor checks kernel version >=4.6 for cgroup namespace support | SATISFIED | kernel_linux.go: fails (hard blocker) when kernel < 4.6, cites cgroup namespace |
| PLAT-01 | 40-02 | Doctor detects firewalld nftables backend on Fedora 32+ | SATISFIED | firewalld.go: 3-step detection, defaults nftables when line absent (Fedora 32+ default) |
| PLAT-02 | 40-03 | Doctor detects WSL2 with multi-signal approach and checks cgroup v2 config | SATISFIED | wsl2.go: multi-signal detection prevents Azure VM false positives, checks cpu/memory/pids controllers |
| PLAT-04 | 40-03 | Doctor checks device node access for rootfs (BTRFS/NVMe) | SATISFIED | rootfs.go: detects BTRFS via both storage driver and DriverStatus backing filesystem |

No orphaned requirements. PLAT-03 is mapped to Phase 41 per REQUIREMENTS.md.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| kernel_other.go | 34 | `return nil` | Info | Intentional stub for non-Linux platforms; Run() never called due to Platforms() filtering |

No blockers, warnings, or actual anti-patterns found. All implementations are substantive with real logic.

### Human Verification Required

### 1. Platform Skip Messages on macOS

**Test:** Run `kinder doctor` on macOS and verify all 7 new linux-only checks show "(linux only)" skip messages
**Expected:** 7 additional skip entries for inotify-limits, kernel-version, apparmor, selinux, firewalld-backend, wsl2-cgroup, rootfs-device
**Why human:** Cannot programmatically verify the full doctor command output rendering on macOS without running the binary

### 2. Linux Integration Test

**Test:** Run `kinder doctor` on actual Linux system with Docker installed
**Expected:** inotify and kernel checks produce real ok/warn results; security checks detect actual system state
**Why human:** Tests use dependency injection; real /proc and /sys reads require actual Linux host

### Gaps Summary

No gaps found. All 5 success criteria truths are fully verified against the codebase:

1. inotify check warns with exact sysctl fix commands at correct thresholds
2. AppArmor and SELinux are independent checks with correct detection logic
3. Kernel version check uses "fail" status (hard blocker) for < 4.6 with cgroup namespace reason
4. Firewalld check detects nftables backend with correct Fedora 32+ default
5. WSL2 multi-signal detection prevents Azure VM false positives and verifies cgroup v2 controllers

All 17 checks are registered in allChecks in correct category order. Full test suite (go test ./...) passes with zero failures. Build (go build ./...) succeeds on macOS with both build-tag variants compiling.

---

_Verified: 2026-03-06T18:45:00Z_
_Verifier: Claude (gsd-verifier)_
