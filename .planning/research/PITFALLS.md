# Pitfalls Research

**Domain:** System-level diagnostic checks and auto-mitigations for a Go CLI (kinder) managing Kubernetes-in-Docker clusters
**Researched:** 2026-03-06
**Confidence:** HIGH for /proc, daemon.json, inotify, firewalld, test isolation, and permission pitfalls (verified against kind's known issues page, Go stdlib docs, Docker docs, and kinder's existing codebase); MEDIUM for WSL2 detection, SELinux+AppArmor coexistence, and subnet clash detection (synthesized from community sources + Microsoft WSL docs); MEDIUM for auto-mitigation safety (synthesized from security advisories and kind issue tracker)

---

## Critical Pitfalls

### Pitfall 1: /proc Filesystem Reads Panic or Return Garbage on Non-Linux

**What goes wrong:**
Diagnostic checks that read `/proc/sys/fs/inotify/max_user_watches`, `/proc/version`, `/proc/self/cgroup`, or `/sys/fs/selinux/enforce` compile fine on macOS/Windows (no build error) but fail at runtime with "no such file or directory" or silently return empty data. If the check code uses `os.ReadFile("/proc/...")` without a prior `runtime.GOOS == "linux"` guard, the error propagates as a check failure rather than a skip. Worse, on macOS, `/proc` does not exist at all, but a developer might not notice because `go test` runs on macOS and the test doesn't exercise the code path (the test uses FakeCmd). The real failure appears only when a user runs `kinder doctor` on macOS and gets a confusing error about missing `/proc` files instead of a clean skip message.

A subtler variant: Docker Desktop on macOS runs a Linux VM. The kinder binary runs on macOS (no `/proc`), but the kind node containers run Linux inside Docker's VM. A developer might incorrectly check the host `/proc` to determine container capabilities, but the host `/proc` doesn't exist on macOS. The check must either query via `docker exec` into the node container or skip entirely on non-Linux hosts.

**Why it happens:**
Go compiles `/proc` file reads on all platforms without error since `os.ReadFile` accepts any string path. There's no compile-time signal that the code is Linux-specific. The existing kinder codebase handles this correctly for the NVIDIA GPU addon (using `currentOS` package-level var), but new developers adding diagnostic checks may not follow the same pattern.

**How to avoid:**
- Every check that reads `/proc` or `/sys` MUST be gated by `if currentOS != "linux" { return skipResult }` using the injectable `currentOS` package-level var pattern established in `installnvidiagpu/nvidiagpu.go`
- Use `//go:build linux` file-level build tags for helper functions that call Linux-only syscalls (e.g., `syscall.Statfs`, netlink). Create corresponding `_notlinux.go` stubs that return "unsupported platform" errors
- Do NOT use build tags on the check registration code itself -- the check must exist on all platforms so it can emit a "skipped (Linux only)" message rather than silently disappearing
- Add a unit test that sets `currentOS = "darwin"` and verifies every `/proc`-reading check returns a skip result, not an error

**Warning signs:**
- A `.go` file contains `os.ReadFile("/proc/")` or `os.ReadFile("/sys/")` without a `runtime.GOOS` or `currentOS` guard in the same function or caller
- `go vet ./...` passes but `kinder doctor` crashes on macOS with "no such file or directory"
- A check function's signature returns only `(result, error)` with no way to express "not applicable on this platform"

**Phase to address:** Phase 1 (check infrastructure / interface design) -- define the check interface with a platform-applicability field or a dedicated "skip" status, before writing any individual checks

---

### Pitfall 2: Inotify Limit Check Reads Container Values, Not Host Values

**What goes wrong:**
The inotify limits (`fs.inotify.max_user_watches` and `fs.inotify.max_user_instances`) are kernel-level settings shared between the host and all containers. When `kinder doctor` reads `/proc/sys/fs/inotify/max_user_watches`, it reads the host's value -- which is correct. However, if a developer decides to "verify from inside the cluster" by running `docker exec kind-control-plane cat /proc/sys/fs/inotify/max_user_watches`, the value inside the container reflects the same host kernel parameter, which is also correct.

The real pitfall is the opposite direction: if `kinder doctor` runs inside a Docker container itself (e.g., in a CI pipeline where the CI runner is a container), the `/proc/sys/fs/inotify/` values may be the CI host's limits, not the developer's machine. The check passes in CI but fails for the user. Additionally, on Docker Desktop (macOS/Windows), the inotify limits are those of Docker's Linux VM, not the macOS/Windows host. A developer on macOS cannot tune these via `sysctl` on macOS -- they must configure them inside the Docker Desktop VM.

A second confusion: the kind documentation recommends `sysctl -w fs.inotify.max_user_watches=524288`. But kinder's auto-mitigation cannot run `sysctl -w` without root privileges. If the check detects a low value and suggests `sudo sysctl -w`, the user may not have sudo access on their machine.

**Why it happens:**
`/proc/sys/fs/inotify/` is a host-level kernel parameter exposed identically inside containers (it's not namespaced in the kernel). Developers assume the value they read is "the value for this machine" without realizing it might be a CI host or Docker Desktop VM value. The check is technically correct but the context of where it runs changes the meaning.

**How to avoid:**
- Read `/proc/sys/fs/inotify/max_user_watches` and `/proc/sys/fs/inotify/max_user_instances` directly on the host, not via `docker exec` -- this is correct and what kind's documentation recommends
- When running on Docker Desktop (detected by checking if the container runtime reports `docker-desktop` context), warn that inotify limits are managed by Docker Desktop's VM and cannot be changed via host `sysctl` -- provide Docker Desktop-specific instructions (settings.json or `wsl -d docker-desktop sysctl -w ...` on Windows)
- For the auto-mitigation suggestion, check `os.Geteuid() == 0` before suggesting `sysctl -w` directly; if not root, suggest `sudo sysctl -w` and document the `/etc/sysctl.conf` persistent method
- Use kind's documented thresholds: `max_user_watches >= 524288` and `max_user_instances >= 512`

**Warning signs:**
- The inotify check passes in CI but users report "too many open files" errors during cluster creation
- The auto-mitigation suggests `sysctl -w` to a user running Docker Desktop on macOS
- The check returns "ok" inside a CI container where the host has high limits, masking that the developer's actual machine has low limits

**Phase to address:** Phase 2 (system resource checks) -- implement inotify check with Docker Desktop context detection and appropriate per-platform remediation messages

---

### Pitfall 3: Docker daemon.json Location Differs Across Platforms and Install Methods

**What goes wrong:**
The check for Docker's `"init": true` setting (which breaks kind) needs to parse `daemon.json`. The file location varies:
- Native Linux Docker Engine: `/etc/docker/daemon.json`
- Docker Desktop on macOS: `~/.docker/daemon.json`
- Docker Desktop on Windows: `%USERPROFILE%\.docker\daemon.json`
- Rootless Docker on Linux: `$XDG_CONFIG_HOME/docker/daemon.json` (defaults to `~/.config/docker/daemon.json`)
- Snap Docker on Linux: `/var/snap/docker/current/config/daemon.json`
- Rancher Desktop: `~/.rd/docker/daemon.json`

A developer who hardcodes `/etc/docker/daemon.json` will miss the `"init": true` setting on Docker Desktop, rootless Docker, Snap Docker, or Rancher Desktop. The check silently passes (file doesn't exist or has different content), and the user hits the "Couldn't find an alternative telinit implementation to spawn" error during cluster creation with no prior warning.

Additionally, `daemon.json` may not exist at all (it's optional -- Docker uses defaults when absent). The check must distinguish "file not found" (fine, no problematic settings) from "file exists but unreadable" (permission error, warn) from "file exists with bad settings" (fail).

**Why it happens:**
Docker's documentation buries the platform-specific paths across multiple pages. Most developers test on a single platform. The daemon.json check is deceptively simple -- parse JSON, check for `"init": true` -- but finding the right file is the hard part.

**How to avoid:**
- Build a `daemonJSONPaths()` function that returns a prioritized list of candidate paths based on `runtime.GOOS` and environment variables (`$XDG_CONFIG_HOME`, `$HOME`, `$DOCKER_CONFIG`)
- Try all paths in order; use the first one that exists and is readable
- If no `daemon.json` exists at any path, return "ok" (absence means no problematic settings)
- If the file exists but is unreadable (permission error), return "warn" with the specific path and the permission error
- Parse with `encoding/json` into `map[string]interface{}` (daemon.json has many optional fields); check for `"init": true` specifically
- Additionally check for `"default-runtime"` being set to something unexpected, and `"storage-driver"` being deprecated values like `aufs`
- Make the path resolution injectable for testing: use a `daemonJSONFinder func() (string, error)` package-level var that tests can override

**Warning signs:**
- The daemon.json check hardcodes a single path like `/etc/docker/daemon.json`
- A developer tests on native Linux and the check works, but Docker Desktop users on macOS see no warning despite having `"init": true`
- The check fails with "permission denied" on rootless Docker because it tries to read `/etc/docker/daemon.json` which is root-owned and irrelevant for rootless setups

**Phase to address:** Phase 2 (Docker configuration checks) -- implement the multi-path resolution before writing the init-daemon check

---

### Pitfall 4: WSL2 Detection Has Multiple Failure Modes

**What goes wrong:**
The WSL2 cgroup misconfiguration check needs to first determine whether kinder is running inside WSL2. The standard detection method -- reading `/proc/version` and looking for "microsoft" or "Microsoft" in the kernel version string -- has documented false positives and false negatives:

1. **False positive:** Azure VMs and some cloud instances use Microsoft-built kernels that contain "Microsoft" in `/proc/version` but are not WSL2. The check would trigger WSL2-specific cgroup remediation on a cloud VM where it's irrelevant and potentially harmful.
2. **False negative:** WSL2 users who have compiled a custom kernel (documented in Microsoft's WSL docs) may have a kernel string that does not contain "microsoft". The check would miss WSL2 entirely.
3. **Case sensitivity:** WSL1 uses "Microsoft" (capital M), WSL2 uses "microsoft-standard-WSL2" (lowercase m). A case-insensitive check catches both but cannot distinguish WSL1 from WSL2, which have different cgroup behaviors.
4. **Environment variable unreliability:** `$WSL_DISTRO_NAME` is set automatically in WSL but could be set manually on any Linux system (pathological but possible).

**Why it happens:**
Microsoft designed WSL2 as a lightweight VM with a custom Linux kernel, not a distinct OS. There is no single authoritative API to detect WSL2 from inside the guest. Detection methods are heuristics, not guarantees.

**How to avoid:**
- Use a multi-signal detection approach, requiring at least two signals to confirm WSL2:
  1. `/proc/version` contains "microsoft" (case-insensitive)
  2. AND either `$WSL_DISTRO_NAME` is set, OR `/proc/sys/fs/binfmt_misc/WSLInterop` exists, OR `wslinfo --networking-mode` succeeds
- If only one signal matches, classify as "possible WSL2" and issue the warning with reduced confidence ("you may be running under WSL2")
- Never auto-mitigate based on WSL2 detection alone -- always prompt/suggest, never apply changes
- Test with explicit WSL2 signal injection in unit tests (mock both `/proc/version` content and env vars)
- If WSL2 is confirmed, check the specific cgroup issue: read `/proc/cgroups` and verify that the cgroup v2 hierarchy is properly mounted, as documented in kind's known issues (https://github.com/spurin/wsl-cgroupsv2)

**Warning signs:**
- The WSL2 check uses only `/proc/version` without a second signal
- The check runs and emits WSL2 warnings on GitHub Actions Linux runners (Azure-hosted, Microsoft kernel)
- Unit tests mock only `runtime.GOOS` but not the `/proc/version` content

**Phase to address:** Phase 3 (platform-specific checks) -- implement WSL2 detection as a reusable utility function used by the cgroup check, not as inline logic

---

### Pitfall 5: Auto-Mitigation That Modifies System State Is Dangerous

**What goes wrong:**
The milestone description mentions "automatic mitigations during cluster creation where safe." The word "safe" is load-bearing. Auto-mitigations that modify system state can:

1. **Persist beyond the cluster lifecycle:** Running `sysctl -w fs.inotify.max_user_watches=524288` changes a kernel parameter that persists until reboot. If the user later creates a non-kinder workload that depends on the original value, it breaks silently.
2. **Require root privileges:** `sysctl -w`, `setenforce 0`, modifying `/etc/docker/daemon.json`, restarting `firewalld` -- all require root. If kinder escalates privileges silently (e.g., using `pkexec` or `sudo`), it may trigger security warnings, password prompts, or audit log entries that surprise the user.
3. **Break other services:** Disabling SELinux (`setenforce 0`) or changing the firewalld backend affects all services on the machine, not just Docker/kinder. A user running kinder on a development server shared with other workloads could break production services.
4. **Create irreversible changes:** Changing `/etc/sysctl.conf` persists across reboots. If the user doesn't realize kinder made this change, they cannot revert it when they uninstall kinder.
5. **Mask root causes:** Auto-fixing low inotify limits hides the fact that the system has a deeper resource constraint. The user creates larger clusters without understanding why the limits were low in the first place.

**Why it happens:**
Developers conflate "we can detect the problem" with "we should fix the problem." The detection logic is straightforward; the mitigation has blast radius. The user experience of "it just works" is appealing, but the cost of unexpected system modifications is high for a local development tool.

**How to avoid:**
- **Never auto-modify** system files (`/etc/sysctl.conf`, `/etc/docker/daemon.json`, `/etc/firewalld/firewalld.conf`, SELinux policy) without explicit user opt-in
- **Tier the mitigation strategy:**
  - Tier 1 (safe to auto-apply): Environment variables (`TMPDIR`), Docker network creation (already done by kind), cluster config adjustments
  - Tier 2 (suggest, never auto-apply): `sysctl -w` (temporary), `setenforce 0` (temporary)
  - Tier 3 (document only, never suggest as a command): Editing `/etc/sysctl.conf`, disabling firewalld, disabling SELinux permanently
- If implementing `kinder doctor --fix`, require confirmation for Tier 2 and refuse Tier 3 entirely
- Print the exact command the user should run, with a clear explanation of what it does and what side effects it has
- Never call `sudo` or `pkexec` from within kinder itself

**Warning signs:**
- A PR adds `exec.Command("sudo", "sysctl", "-w", ...)` anywhere in the codebase
- A check's "fix" function modifies a file outside of `$HOME/.kinder/` or the cluster's Docker resources
- Auto-mitigation works in CI (where the runner has root) but fails on user machines (where kinder runs as non-root)

**Phase to address:** Phase 1 (check infrastructure) -- define the mitigation tier system in the check interface before implementing any individual checks; enforce via code review that no check crosses tier boundaries

---

### Pitfall 6: Subnet Clash Detection Requires Platform-Specific Route Enumeration

**What goes wrong:**
Detecting whether kind's Docker network (default `172.18.0.0/16`) clashes with the host's VPN, lab network, or other Docker networks requires enumerating the host's routing table. On Linux, this can be done via:
- Parsing `/proc/net/route` (fragile, hex-encoded, IPv4 only)
- Using the netlink API via `vishvananda/netlink` (Linux-only, requires `CAP_NET_ADMIN` for some operations)
- Running `ip route show` and parsing the text output (requires `iproute2` installed)

On macOS, routes are obtained via `netstat -rn` or `route -n get default`, which have completely different output formats. On Windows/WSL2, the routing table is the WSL2 VM's, not the Windows host's -- so a VPN on the Windows side won't show up in WSL2's route table.

The existing kinder codebase detects the kind network's subnet via `docker network inspect` (see `subnet.go`). A subnet clash check would need to compare this subnet against all host routes. The pitfall is that the cross-platform route enumeration is the hard part, not the comparison.

Additionally, `vishvananda/netlink` is Linux-only and imports `syscall` constants that don't exist on macOS/Windows. If imported unconditionally, the project won't compile on macOS. Even if gated behind build tags, adding a netlink dependency significantly increases the binary size and introduces a CGO-adjacent dependency (netlink uses raw syscalls).

**Why it happens:**
The Go standard library's `net` package provides address resolution but not route table enumeration. There is no cross-platform Go API for "list all routes on this machine." Every approach is OS-specific.

**How to avoid:**
- Use the simplest approach that works: shell out to `ip route show` on Linux and parse the output, similar to how kinder already shells out to `docker network inspect`
- On macOS, shell out to `netstat -rn` or `route -n get` and parse differently
- Do NOT add `vishvananda/netlink` as a dependency -- it's Linux-only and overkill for reading the route table; shelling out to `ip route` achieves the same result with zero new dependencies
- Compare each host route's CIDR against the kind network's CIDR using `net.IPNet.Contains()` for overlap detection
- On Docker Desktop (macOS/Windows), skip the host route check entirely and only check for Docker network conflicts (via `docker network ls` and `docker network inspect`) -- the Docker Desktop VM's network is isolated from the host's routes
- Make the route parser injectable for testing: `var getHostRoutes = getHostRoutesImpl` so tests can return synthetic route tables

**Warning signs:**
- `go.sum` contains `vishvananda/netlink` or any netlink library
- The subnet clash check only works on Linux and silently skips on macOS without any Docker-level network conflict check
- The route parser assumes `ip route show` output format is stable across distro versions (it is, but the parser should be tested against real output samples)

**Phase to address:** Phase 3 (network checks) -- implement route parsing with build-tag-separated implementations and injectable parsers for testing

---

### Pitfall 7: SELinux and AppArmor Can Both Be Present on the Same System

**What goes wrong:**
A naive check structure assumes SELinux and AppArmor are mutually exclusive (since distros historically ship one or the other). In 2025, openSUSE Tumbleweed switched its default from AppArmor to SELinux, meaning systems upgraded from older openSUSE may have both frameworks' tools installed. Additionally, container runtimes may load AppArmor profiles even on SELinux-primary systems (Docker applies AppArmor profiles by default on Debian/Ubuntu).

The pitfall: if the check for SELinux is `if fileExists("/sys/fs/selinux/enforce")` and the check for AppArmor is `if fileExists("/sys/kernel/security/apparmor")`, both can return true on the same machine. If the diagnostic logic says "SELinux detected, checking enforce mode" and separately "AppArmor detected, checking profiles," the user gets two warnings that may contradict each other ("disable SELinux" AND "disable AppArmor" -- which one is actually causing the problem?).

For kind specifically: the documented SELinux issue (Fedora 33, "open /dev/dma_heap: permission denied") is about SELinux denying device access. The AppArmor issue is about AppArmor profiles interfering with container execution. These are different failure modes with different remediation, but both affect kind on the same machine.

**Why it happens:**
Linux Security Modules (LSM) can stack since kernel 5.1. A system can have SELinux in enforcing mode AND AppArmor loaded simultaneously. Distribution defaults create false assumptions about mutual exclusivity.

**How to avoid:**
- Check both independently and report both, but clearly indicate which one is the *active enforcing* MAC:
  - SELinux: read `/sys/fs/selinux/enforce` -- if it contains "1", SELinux is actively enforcing
  - AppArmor: check if `/sys/kernel/security/apparmor/profiles` exists and is non-empty, AND check if Docker is using AppArmor profiles (`docker info --format '{{.SecurityOptions}}'` contains "apparmor")
- Report the state of both but only flag the one that is likely causing issues:
  - If SELinux is enforcing: warn about kind's known SELinux issue (device access denial on Fedora)
  - If Docker's AppArmor profiles are active: warn about kind's known AppArmor issue
  - If both are active: report both warnings and suggest checking `dmesg` for which MAC is denying access
- Do NOT suggest `setenforce 0` as a first resort -- suggest `setenforce Permissive` (less drastic) or checking the audit log (`ausearch -m AVC -ts recent`) first
- For AppArmor, suggest checking `aa-status` and looking for Docker-related profiles rather than disabling AppArmor entirely

**Warning signs:**
- The code has `if selinuxDetected { ... } else if apparmorDetected { ... }` -- the `else` is wrong; both can be true
- The remediation for SELinux says "disable SELinux" without mentioning that AppArmor might also be active
- The check tests SELinux and AppArmor in separate test functions but never tests the case where both are present

**Phase to address:** Phase 3 (security module checks) -- implement as two independent checks that both emit results, with a combined analysis step that detects the "both active" case

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hardcode `/etc/docker/daemon.json` as the only daemon.json path | Works on native Linux Docker Engine | Misses Docker Desktop, rootless, Snap installs -- silent false negatives | Never -- use multi-path resolution from day one |
| Use `os.ReadFile` for `/proc` files without platform guard | Code compiles on all platforms | Crashes on macOS/Windows with confusing error | Never -- gate with `currentOS` check |
| Shell out to `ip route show` and parse with string splitting | Quick implementation | Breaks on systems where `iproute2` is not installed (minimal containers, Alpine) or locale differences | Acceptable for Linux host checks only; always wrap in "command not found" error handling |
| Add `vishvananda/netlink` for route enumeration | Clean Go API, no string parsing | Linux-only dependency, binary size increase, CGO-adjacent raw syscalls | Never for kinder -- shelling out to `ip route` is sufficient |
| Use `exec.LookPath` as the only check for tool presence | Simple binary detection | Binary may exist but be a broken version, or may be from a different package (e.g., `firewall-cmd` exists but firewalld service is stopped) | Acceptable as first check; follow up with a version/status command |
| Auto-apply `sysctl -w` without user confirmation | "It just works" UX | Unexpected system state changes, requires root, persists until reboot | Never -- always suggest, never auto-apply |
| Skip Docker Desktop context detection | Fewer code paths | Inotify/sysctl suggestions are wrong on macOS/Windows (user can't `sysctl` on macOS host) | Never -- Docker Desktop is a primary kinder platform |
| Test all checks with `FakeCmd` only, no real filesystem tests | Fast tests, run on macOS CI | Logic errors in `/proc` parsing or `ip route` output parsing are never caught | Acceptable for CI; add Linux-only integration tests gated by build tag `//go:build linux` |

---

## Integration Gotchas

Common mistakes when connecting diagnostic checks to the existing kinder system.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Adding checks to `doctor.go` | Putting all 13 checks inline in `runE()` -- the function becomes 500+ lines | Create a `checks/` package with one file per check; `runE()` iterates a registered check list |
| Check result aggregation | Each check returns its own exit code independently | Collect all results, then compute a single exit code: fail if any check failed, warn if any warned, ok otherwise (existing pattern) |
| JSON output for new checks | New checks add fields to `checkResult` that break existing JSON consumers | Keep the same `checkResult` struct (`name`, `status`, `message`); add new checks as new array entries, not new fields |
| Platform-gated checks on macOS | Skipping the check entirely (no output) | Emit `{"name": "inotify", "status": "skip", "message": "Linux only"}` so the user knows the check exists but was skipped |
| Docker daemon.json check + Podman/Nerdctl | Checking daemon.json when the user is using Podman (Podman has no daemon.json) | Gate daemon.json checks with a provider detection step: only check daemon.json when Docker is the active runtime |
| Firewalld check + non-systemd systems | Running `systemctl status firewalld` on a system without systemd (Alpine, Void, WSL1) | Check for `firewall-cmd` binary first; if absent, skip with "firewalld not installed (not needed on non-systemd systems)" |
| Subnet clash check + Docker Desktop | Checking host routes on macOS/Windows where the Docker network is inside a VM | On Docker Desktop, only check for Docker-to-Docker network conflicts; host route checks are meaningless through the VM boundary |
| Auto-mitigation + non-root execution | Attempting `sysctl -w` without root | Check `os.Geteuid()` before suggesting mitigation commands; on non-root, prefix suggestions with `sudo` |
| SELinux check + non-RHEL distros | Assuming `/usr/sbin/getenforce` exists on all Linux distros | Check `/sys/fs/selinux/enforce` (kernel interface, always present if SELinux is compiled into kernel) instead of shelling out to `getenforce` |
| Disk space check + Docker data directory | Checking free space on `/` instead of Docker's data root | Read Docker's data root via `docker info --format '{{.DockerRootDir}}'` (usually `/var/lib/docker`); check free space on that partition, not necessarily `/` |

---

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Running all 13 checks sequentially | `kinder doctor` takes 5-10 seconds because each check shells out to a command | Group checks by dependency (Docker checks need Docker running; kernel checks don't); run independent groups in parallel with `errgroup` | Immediately noticeable with 13+ checks |
| Calling `docker info` once per Docker-related check | 4-5 separate `docker info` invocations, each taking 200-500ms | Call `docker info --format '{{json .}}'` once at the start, parse the JSON, and pass the parsed struct to all Docker checks | With 4+ Docker checks |
| Reading `/proc` files with `os.ReadFile` for each check | Multiple filesystem reads to the same `/proc` directory | For checks that read multiple `/proc/sys/fs/inotify/*` files, batch them into a single directory scan | Negligible impact, but good practice |
| Subnet clash check enumerating all Docker networks | `docker network ls` + `docker network inspect` for each network, one at a time | Use `docker network inspect $(docker network ls -q)` to inspect all networks in a single command (already done in kinder's `network.go`) | With 10+ Docker networks on the host |

---

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Auto-running `sudo sysctl -w` from kinder | Privilege escalation vector; if kinder binary is compromised, attacker gets root sysctl access | Never invoke `sudo` from kinder; print the command for the user to run manually |
| Suggesting `setenforce 0` as first-line fix | Disables SELinux system-wide, removing all mandatory access control protections until reboot | Suggest `setenforce Permissive` for debugging, or checking audit logs; never suggest permanent disable |
| Reading Docker daemon.json that may contain registry auth tokens | Parsing daemon.json may expose `"auths"` field with base64-encoded credentials in error messages or logs | Only parse the specific fields needed (`"init"`, `"default-runtime"`, `"storage-driver"`); never log the full file content |
| Subnet clash check exposing internal network topology in output | Listing all host routes and Docker subnets in JSON output reveals VPN subnets, lab network ranges | Only report whether a clash was detected and which Docker network it involves; do not dump the full route table in output |
| Docker permissions check suggesting `chmod 777 /var/run/docker.sock` | Gives all users Docker access, which is equivalent to root access | Suggest adding the user to the `docker` group (`sudo usermod -aG docker $USER`) per Docker's official documentation |

---

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Reporting raw `/proc` values without context | User sees "max_user_watches: 8192" but doesn't know if that's good or bad | Always show the value AND the required minimum: "max_user_watches: 8192 (required: 524288)" |
| Using technical jargon in check names | User sees "inotify-max-user-instances" and doesn't know what it means | Use descriptive names: "File watcher limits" with the technical name in parentheses |
| Emitting all 13 check results even when only 1 failed | Wall of green output obscures the one red failure | Show only failures and warnings by default; show all results with `--verbose` |
| Providing fix commands without explaining side effects | User runs `setenforce 0` without understanding they disabled SELinux | Always add a one-sentence explanation: "This temporarily disables SELinux enforcement (reverts on reboot)" |
| Warning about issues the user cannot fix | Docker Desktop user warned about host inotify limits they cannot change from macOS | Detect Docker Desktop and adjust the message to explain how to change the setting via Docker Desktop's settings or VM |
| Exit code 2 (warn) for optional/informational checks | CI pipelines treat exit code 2 as failure | Add `--level=error` flag that only exits non-zero for actual failures, ignoring warnings |

---

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **Inotify check:** Reads `/proc/sys/fs/inotify/max_user_watches` -- but also detects Docker Desktop context and adjusts remediation message, AND handles the case where `/proc` doesn't exist (macOS), AND tests with both high and low values, AND tests the "running inside a container" case
- [ ] **daemon.json check:** Parses `daemon.json` for `"init": true` -- but also checks all platform-specific paths (native Linux, Docker Desktop, rootless, Snap), AND handles missing file as "ok", AND handles permission errors as "warn", AND tests with each path variant
- [ ] **WSL2 check:** Reads `/proc/version` for "microsoft" -- but also uses a second signal (env var or `/proc/sys/fs/binfmt_misc/WSLInterop`), AND distinguishes WSL1 from WSL2, AND does not false-positive on Azure VMs, AND tests all combinations
- [ ] **SELinux check:** Reads `/sys/fs/selinux/enforce` -- but also checks if SELinux is the active MAC (vs. installed but not enforcing), AND works on non-RHEL distros, AND reports alongside AppArmor without contradiction
- [ ] **Firewalld check:** Checks if `firewall-cmd` exists -- but also verifies firewalld is running (`firewall-cmd --state`), AND checks the backend (`--get-backend` or parsing `firewalld.conf`), AND handles non-systemd systems, AND handles systems where firewalld is installed but not active
- [ ] **Disk space check:** Runs `syscall.Statfs` on a path -- but also checks Docker's data root path (not just `/`), AND reports both available and required space, AND handles the case where Docker is using a different partition than `/`
- [ ] **Subnet clash check:** Compares kind network to host routes -- but also handles Docker Desktop (VM-isolated network), AND parses `ip route` output correctly for both simple and policy-based routing, AND falls back gracefully when `ip` command is not available
- [ ] **Docker snap check:** Detects Snap Docker -- but also checks if TMPDIR is set correctly, AND provides the correct TMPDIR suggestion based on the user's home directory
- [ ] **Docker permissions check:** Detects `~/.docker/` permission issues -- but also distinguishes between "docker group missing" and "docker socket permissions wrong" and "rootless docker needing different config"
- [ ] **All checks:** Pass on macOS with skip status, AND pass on Linux with both ok and fail scenarios, AND produce valid JSON output, AND work with both `--output json` and human-readable output

---

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| /proc check crashes on macOS | LOW | Add `currentOS` guard to the failing check; the fix is a single if-statement |
| daemon.json check misses Docker Desktop path | LOW | Add macOS/Windows path to the path list; existing check logic works unchanged |
| WSL2 false positive on Azure VM | MEDIUM | Add second-signal verification; requires testing on actual Azure VM to validate |
| Auto-mitigation breaks user's system | HIGH | Cannot undo `sysctl -w` remotely; must document revert steps; if `/etc/sysctl.conf` was modified, must document how to find and remove the line kinder added |
| SELinux+AppArmor dual-warning confuses user | LOW | Change from if/else to independent checks; add "both active" combined message |
| Subnet clash check fails on macOS | LOW | Add Docker Desktop detection; skip host route check, keep Docker network conflict check |
| Firewalld check runs on Ubuntu (no firewalld) | LOW | Guard with `exec.LookPath("firewall-cmd")` before running any firewalld commands |
| Inotify check shows wrong values in CI container | MEDIUM | Add "running in container" detection (`/.dockerenv` exists or `/proc/1/cgroup` contains docker); adjust message to warn about container vs host values |
| Disk space check reads wrong partition | LOW | Use `docker info --format '{{.DockerRootDir}}'` to find the right path; re-run `Statfs` on that path |

---

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| /proc reads crash on non-Linux | Phase 1: Check infrastructure | `kinder doctor` runs on macOS without errors; all /proc checks show "skip" status |
| Check interface lacks "skip" status | Phase 1: Check infrastructure | `kinder doctor --output json` on macOS shows `"status": "skip"` for Linux-only checks |
| Auto-mitigation tier system undefined | Phase 1: Check infrastructure | Code review confirms no check calls `sudo` or modifies system files |
| Inotify container vs host confusion | Phase 2: System resource checks | Inotify check detects Docker Desktop and adjusts message; test covers both contexts |
| daemon.json multi-path resolution | Phase 2: Docker config checks | Check finds daemon.json on native Linux, Docker Desktop macOS, and rootless Docker |
| Docker snap TMPDIR detection | Phase 2: Docker config checks | Check detects Snap Docker and suggests correct TMPDIR |
| Docker init daemon setting | Phase 2: Docker config checks | Check warns when daemon.json has `"init": true` |
| Docker permissions check | Phase 2: Docker config checks | Check detects `~/.docker/` permission issues and docker group membership |
| Disk space on Docker data root | Phase 2: System resource checks | Check runs `Statfs` on Docker's data root, not hardcoded `/` |
| WSL2 false positives | Phase 3: Platform-specific checks | WSL2 detection requires two signals; test mocks Azure VM kernel string |
| SELinux + AppArmor dual detection | Phase 3: Security module checks | Both checks run independently; combined message emitted when both active |
| Firewalld on non-systemd systems | Phase 3: Security module checks | Firewalld check skips gracefully on Ubuntu/Debian where firewalld is absent |
| Subnet clash cross-platform | Phase 3: Network checks | Route parsing has Linux and macOS implementations; Docker Desktop skips host routes |
| Old kernel / cgroup namespace | Phase 3: Platform-specific checks | Check reads kernel version from `uname` (cross-platform via `syscall.Uname`) and warns if pre-4.6 |
| kubectl version skew | Phase 2: Tool version checks | Check compares kubectl client version against latest kind-supported server version |
| All checks in JSON output | Phase 1: Check infrastructure | `kinder doctor --output json` returns valid JSON array with all check names |
| Performance: sequential check execution | Phase 4: Polish / integration | `kinder doctor` completes in under 2 seconds with all 13 checks |

---

## Fork-Specific Pitfalls

Issues unique to kinder's status as a fork of kind.

### Pitfall F1: kind's Known Issues Page Is the Ground Truth -- Don't Reinvent Detection Logic

Kind's known issues page (https://kind.sigs.k8s.io/docs/user/known-issues/) documents the exact symptoms, affected platforms, and workarounds for each issue that kinder's new doctor checks will detect. The pitfall is implementing detection logic from first principles ("how do I check if firewalld uses nftables?") instead of reverse-engineering the known issue ("what exact state causes the documented symptom?").

For example, the firewalld issue is specifically: `FirewallBackend=nftables` in `/etc/firewalld/firewalld.conf` on Fedora 32+ breaks Docker networking. The check should read that specific file and look for that specific value, not try to detect "whether the firewall will interfere" in some general way.

**Prevention:** For each of the ~13 checks, start from kind's known issues page description, identify the exact system state that causes the issue, and write the check to detect that state. Do not generalize beyond what kind documents.

**Phase to address:** Every phase -- use kind's known issues as the specification for each check.

### Pitfall F2: Doctor Checks Must Not Import Provider-Internal Packages

The `doctor.go` command currently lives in `pkg/cmd/kind/doctor/` and only uses `pkg/exec` and standard library packages. The new diagnostic checks may need to detect which container runtime is active (Docker vs Podman vs Nerdctl) to decide which checks to run. The temptation is to import `pkg/cluster/internal/providers/docker/` to reuse runtime detection logic, but this would violate the package layering: `cmd/` packages should not import `internal/` packages from `cluster/`.

**Prevention:** Extract any needed runtime detection logic into a shared utility package (e.g., `pkg/runtime/detect.go`) that both `cmd/kind/doctor/` and `cluster/internal/providers/` can import. Or simply shell out to `docker info` / `podman info` in the doctor checks, since doctor already shells out for its existing checks.

**Phase to address:** Phase 1 (check infrastructure) -- decide the package structure before implementing checks.

### Pitfall F3: Existing Doctor Exit Code Contract (0/1/2) Must Be Preserved

The existing `kinder doctor` command uses exit codes: 0 (all ok), 1 (failure), 2 (warning). Adding 13 new checks that may emit "skip" status must not change this contract. A "skip" should not be treated as a warning (exit 2) because macOS users would always get exit 2 due to skipped Linux-only checks, making the exit code useless for CI.

**Prevention:** Add "skip" as a fourth status value in the result struct. Treat "skip" as equivalent to "ok" for exit code purposes. Update the JSON output to include "skip" status. Test that macOS runs exit 0 when all applicable checks pass and all Linux-only checks are skipped.

**Phase to address:** Phase 1 (check infrastructure) -- add "skip" status before adding any platform-gated checks.

---

## Sources

- Kind official known issues page: https://kind.sigs.k8s.io/docs/user/known-issues/ (HIGH confidence -- primary specification for all checks)
- Docker daemon configuration docs: https://docs.docker.com/engine/daemon/ (HIGH confidence -- daemon.json locations and options)
- Docker Desktop settings docs: https://docs.docker.com/desktop/settings-and-maintenance/settings/ (HIGH confidence -- macOS/Windows daemon.json path)
- Docker daemon.json macOS location issue: https://github.com/docker/docs/issues/2643 (HIGH confidence -- confirms `~/.docker/daemon.json` on macOS)
- Docker daemon.json Windows location issue: https://github.com/docker/docker.github.io/issues/15106 (HIGH confidence)
- WSL2 /proc/version detection gist: https://gist.github.com/s0kil/336a246cc2bc8608e645c69876c17466 (MEDIUM confidence -- community source, verified against Microsoft docs)
- Microsoft WSL detection issue #4071: https://github.com/microsoft/WSL/issues/4071 (HIGH confidence -- Microsoft official repo)
- wezterm WSL detection false negative: https://github.com/wezterm/wezterm/issues/7136 (MEDIUM confidence -- demonstrates real-world detection failures)
- Ubuntu discourse WSL detection: https://discourse.ubuntu.com/t/detecting-if-you-are-on-wsl-system/11837 (MEDIUM confidence -- community discussion with Canonical participation)
- Kubernetes inotify issue #46230: https://github.com/kubernetes/kubernetes/issues/46230 (HIGH confidence)
- Inotify in containers blog: https://william-yeh.net/post/2019/06/inotify-in-containers/ (MEDIUM confidence -- technical analysis of container vs host inotify behavior)
- Kind issue #431 Docker snap TMPDIR: https://github.com/kubernetes-sigs/kind/issues/431 (HIGH confidence -- documents the exact failure mode)
- Minikube snap TMPDIR fix: https://github.com/kubernetes/minikube/pull/10372 (HIGH confidence -- confirms the TMPDIR workaround pattern)
- SELinux vs AppArmor comparison (Datadog): https://securitylabs.datadoghq.com/articles/container-security-fundamentals-part-5/ (HIGH confidence -- detailed technical comparison of LSM stacking)
- openSUSE SELinux default switch: https://dohost.us/index.php/2025/10/05/selinux-vs-apparmor-a-comparative-deep-dive-into-linux-security-modules-lsm/ (MEDIUM confidence -- confirms 2025 SELinux adoption trend)
- vishvananda/netlink Go package: https://pkg.go.dev/github.com/vishvananda/netlink (HIGH confidence -- official package docs)
- Go build tags for testing: https://mickey.dev/posts/go-build-tags-testing/ (MEDIUM confidence -- community best practice)
- Go disk usage via syscall.Statfs: https://gist.github.com/lunny/9828326 (MEDIUM confidence -- demonstrates cross-platform Statfs pitfalls)
- Kinder codebase direct analysis: `pkg/cmd/kind/doctor/doctor.go`, `pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu.go`, `pkg/cluster/internal/create/actions/testutil/fake.go`, `pkg/exec/types.go`, `pkg/cluster/internal/providers/docker/network.go`, `pkg/cluster/internal/create/actions/installmetallb/subnet.go` (HIGH confidence -- primary sources)
- Kind rootless documentation: https://kind.sigs.k8s.io/docs/user/rootless/ (HIGH confidence -- documents rootless Docker/Podman considerations)

---
*Pitfalls research for: kinder v2.1 -- system-level diagnostic checks and auto-mitigations*
*Researched: 2026-03-06*
