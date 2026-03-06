# Phase 39: Docker and Tool Configuration Checks - Research

**Researched:** 2026-03-06
**Domain:** System-level diagnostic checks for Docker configuration, disk space, snap detection, kubectl version skew, and Docker socket permissions
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DOCK-01 | Doctor checks available disk space, warns at <5GB, fails at <2GB | Standard Stack: `golang.org/x/sys/unix.Statfs`; Architecture: diskSpaceCheck with `readDiskFree` dep; Code Examples: cross-platform Statfs pattern; Pitfalls: macOS Docker Desktop disk check approach |
| DOCK-02 | Doctor detects daemon.json "init: true" across 6+ location candidates | Architecture: daemonJSONCheck with `readFile`+`homeDir` deps and candidate path list; Code Examples: multi-path resolution with JSON parsing; Pitfalls: platform-specific paths, file-not-found is OK |
| DOCK-03 | Doctor detects Docker installed via snap and warns about TMPDIR issues | Architecture: dockerSnapCheck with `lookPath`+`evalSymlinks` deps; Code Examples: symlink resolution to detect /snap/ path; Pitfalls: snap-installed Docker has daemon.json at different path |
| DOCK-04 | Doctor detects kubectl version skew and warns about incompatibility | Architecture: kubectlVersionSkewCheck with `lookPath`+`execCmd` deps; Code Examples: kubectl version JSON parsing + semver comparison via `pkg/internal/version`; Pitfalls: needs server version from node image or running cluster |
| DOCK-05 | Doctor detects Docker socket permission denied and suggests fix | Architecture: dockerSocketCheck with `lookPath`+`execCmd` deps; Code Examples: stderr parsing for "permission denied" on docker info; Pitfalls: Linux-only, rootless Docker uses different socket path |
</phase_requirements>

## Summary

Phase 39 adds five new diagnostic checks to the `kinder doctor` command, building on the Check interface, Result type, AllChecks() registry, and test infrastructure established in Phase 38. Each check follows the exact same deps struct injection pattern used by the existing 5 checks (container-runtime, kubectl, nvidia-driver, nvidia-container-toolkit, nvidia-docker-runtime). No new go.mod dependencies are needed for 4 of the 5 checks; only the disk space check (DOCK-01) requires promoting `golang.org/x/sys` from indirect to direct in go.mod (the package is already at v0.41.0 in the dependency tree).

The five checks address the most common Docker/tool configuration problems documented on kind's Known Issues page: insufficient disk space causing silent kubelet eviction, Docker daemon.json "init: true" causing cryptic container startup failures, snap-installed Docker breaking TMPDIR access, kubectl version skew causing schema validation errors, and Docker socket permission denied blocking cluster creation. Each check produces a Result with the user-specified 6-field struct (name, category, status, message, reason, fix) and integrates into the existing category-grouped output and JSON envelope.

The primary implementation challenge is DOCK-02 (daemon.json detection) which must search 6+ candidate file locations across native Linux, Docker Desktop macOS, rootless Docker, snap Docker, Rancher Desktop, and Windows. The other 4 checks are straightforward deps struct implementations following the established pattern exactly. DOCK-01 (disk space) needs a build-tagged helper file because `golang.org/x/sys/unix.Statfs_t` has different field types on Linux (Bsize is int64) vs macOS (Bsize is uint32), though both cast safely to int64 for the computation.

**Primary recommendation:** Add 5 new check source files to `pkg/internal/doctor/` (disk.go, daemon.go, snap.go, versionskew.go, socket.go) using the exact deps struct + fakeCmd test pattern from Phase 38, register all 5 in the AllChecks() slice in check.go under two new categories ("Docker" and "Tools"), and promote `golang.org/x/sys` to direct in go.mod.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/sys/unix` | v0.41.0 | `unix.Statfs` for disk space on Linux and macOS | Already indirect in go.mod; promoted to direct. Replaces deprecated `syscall.Statfs`. Provides `Bavail` (available-to-user blocks) and `Bsize` for byte computation |
| Go stdlib (`os`, `os/exec`, `encoding/json`, `path/filepath`, `strings`, `strconv`, `runtime`) | Go 1.24+ | File reads, JSON parsing, symlink resolution, string parsing | Already used by Phase 38 checks; zero new imports needed |
| `sigs.k8s.io/kind/pkg/exec` | (internal) | Command execution via Cmd interface | Same pattern as all Phase 38 checks; enables fakeCmd test injection |
| `sigs.k8s.io/kind/pkg/internal/version` | (internal) | `ParseSemantic`, `Minor()` for kubectl version skew comparison | Already in codebase; handles `v1.31.0` format correctly |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/internal/doctor` (testhelpers_test.go) | (internal) | `fakeCmd`, `fakeExecResult`, `newFakeExecCmd` | All check unit tests -- exact same test doubles from Phase 38 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `golang.org/x/sys/unix.Statfs` | `docker system df --format` | `unix.Statfs` is a direct syscall with no Docker dependency; `docker system df` requires Docker running and adds latency. Use Statfs for host disk check, Docker info for data-root path |
| Parsing stderr for "permission denied" | `net.DialTimeout("unix", socketPath, ...)` | Socket dial is more reliable but requires importing `net` and `time`; stderr parsing catches the exact Docker error message and is consistent with the existing exec.Command pattern |
| `github.com/shirou/gopsutil/disk` | `golang.org/x/sys/unix.Statfs` | gopsutil adds a large transitive dependency tree; Statfs is one function call with zero new deps |

**Installation:**
```bash
# Only change: promote golang.org/x/sys from indirect to direct in go.mod
# No version change needed -- already at v0.41.0
go mod tidy
```

## Architecture Patterns

### Recommended Project Structure
```
pkg/internal/doctor/
    check.go              # MODIFIED: add 5 new checks to allChecks registry
    disk.go               # NEW: diskSpaceCheck (DOCK-01)
    disk_unix.go          # NEW: build-tagged Statfs helper for linux+darwin
    disk_other.go         # NEW: build-tagged stub for windows/other
    disk_test.go          # NEW: tests for disk space check
    daemon.go             # NEW: daemonJSONCheck (DOCK-02)
    daemon_test.go        # NEW: tests for daemon.json check
    snap.go               # NEW: dockerSnapCheck (DOCK-03)
    snap_test.go          # NEW: tests for snap detection
    versionskew.go        # NEW: kubectlVersionSkewCheck (DOCK-04)
    versionskew_test.go   # NEW: tests for kubectl version skew
    socket.go             # NEW: dockerSocketCheck (DOCK-05)
    socket_test.go        # NEW: tests for docker socket permissions
    # existing files unchanged:
    runtime.go, runtime_test.go
    tools.go, tools_test.go
    gpu.go, gpu_test.go
    format.go, format_test.go
    mitigations.go, mitigations_test.go
    testhelpers_test.go
    check_test.go
```

### Pattern 1: Deps Struct for New Checks (follow exactly from Phase 38)
**What:** Each new check uses a struct with injectable function fields for system dependencies. The constructor wires real functions; tests inject fakes directly.
**When to use:** Every new check implementation.
**Example:**
```go
// Source: direct extension of Phase 38 containerRuntimeCheck pattern from runtime.go

type diskSpaceCheck struct {
    readDiskFree func(path string) (uint64, error)  // returns bytes available
    execCmd      func(name string, args ...string) exec.Cmd  // for docker info
}

func newDiskSpaceCheck() Check {
    return &diskSpaceCheck{
        readDiskFree: statfsFreeBytes,  // implemented in disk_unix.go
        execCmd:      exec.Command,
    }
}

func (c *diskSpaceCheck) Name() string       { return "disk-space" }
func (c *diskSpaceCheck) Category() string    { return "Docker" }
func (c *diskSpaceCheck) Platforms() []string { return nil } // all platforms
```

### Pattern 2: Build-Tagged Statfs Helper
**What:** The `unix.Statfs` function is only available on unix-like platforms. Use build tags to provide a platform-specific implementation and a stub.
**When to use:** Only for the disk space check.
**Example:**
```go
// disk_unix.go
//go:build linux || darwin

package doctor

import "golang.org/x/sys/unix"

// statfsFreeBytes returns available bytes on the filesystem containing path.
func statfsFreeBytes(path string) (uint64, error) {
    var stat unix.Statfs_t
    if err := unix.Statfs(path, &stat); err != nil {
        return 0, err
    }
    // Bavail is uint64 on both Linux and Darwin.
    // Bsize is int64 on Linux, uint32 on Darwin -- cast to int64 for safe multiplication.
    return stat.Bavail * uint64(int64(stat.Bsize)), nil
}
```

```go
// disk_other.go
//go:build !linux && !darwin

package doctor

import "errors"

func statfsFreeBytes(path string) (uint64, error) {
    return 0, errors.New("disk space check not supported on this platform")
}
```

### Pattern 3: Multi-Path Candidate Resolution for daemon.json
**What:** DOCK-02 requires checking 6+ file locations for daemon.json. Use a prioritized candidate list and injectable file reader.
**When to use:** The daemon.json init:true check.
**Example:**
```go
// Source: Docker docs + Rancher Desktop discussions + snap Docker docs

type daemonJSONCheck struct {
    readFile func(string) ([]byte, error)
    homeDir  func() (string, error)
    goos     string
}

func newDaemonJSONCheck() Check {
    return &daemonJSONCheck{
        readFile: os.ReadFile,
        homeDir:  os.UserHomeDir,
        goos:     runtime.GOOS,
    }
}

// daemonJSONCandidates returns the ordered list of daemon.json paths to check.
// The first readable file wins.
func (c *daemonJSONCheck) daemonJSONCandidates() []string {
    home, _ := c.homeDir()
    xdgConfig := os.Getenv("XDG_CONFIG_HOME")
    if xdgConfig == "" && home != "" {
        xdgConfig = filepath.Join(home, ".config")
    }

    candidates := []string{
        "/etc/docker/daemon.json",                                   // native Linux
    }
    if home != "" {
        candidates = append(candidates,
            filepath.Join(home, ".docker", "daemon.json"),           // Docker Desktop macOS + Rancher Desktop
        )
    }
    if xdgConfig != "" {
        candidates = append(candidates,
            filepath.Join(xdgConfig, "docker", "daemon.json"),       // rootless Docker
        )
    }
    candidates = append(candidates,
        "/var/snap/docker/current/config/daemon.json",               // snap Docker
    )
    if home != "" {
        candidates = append(candidates,
            filepath.Join(home, ".rd", "docker", "daemon.json"),     // Rancher Desktop alternate
        )
    }
    // Windows: C:\ProgramData\docker\config\daemon.json
    if c.goos == "windows" {
        candidates = append(candidates,
            filepath.Join(os.Getenv("ProgramData"), "docker", "config", "daemon.json"),
        )
    }
    return candidates
}
```

### Pattern 4: kubectl Version Skew Detection
**What:** Parse kubectl client version from JSON output, compare minor version against a reference. Since `kinder doctor` runs pre-cluster, compare against the latest kind node image version.
**When to use:** DOCK-04 kubectl version skew check.
**Example:**
```go
// Source: kubectl version --client -o json stable output format

type kubectlVersionOutput struct {
    ClientVersion struct {
        Major      string `json:"major"`
        Minor      string `json:"minor"`
        GitVersion string `json:"gitVersion"`
    } `json:"clientVersion"`
}

type kubectlVersionSkewCheck struct {
    lookPath func(string) (string, error)
    execCmd  func(name string, args ...string) exec.Cmd
}

func (c *kubectlVersionSkewCheck) Run() []Result {
    if _, err := c.lookPath("kubectl"); err != nil {
        // kubectl not found is handled by the existing kubectlCheck
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "skip",
            Message: "(kubectl not found)",
        }}
    }

    output, err := exec.CombinedOutputLines(c.execCmd("kubectl", "version", "--client", "-o", "json"))
    if err != nil || len(output) == 0 {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: "Could not parse kubectl version",
            Reason: "kubectl version --client -o json did not produce output",
            Fix: "Reinstall kubectl: https://kubernetes.io/docs/tasks/tools/",
        }}
    }

    var vOut kubectlVersionOutput
    if err := json.Unmarshal([]byte(strings.Join(output, "")), &vOut); err != nil {
        // ... handle parse error
    }

    clientVer, err := version.ParseSemantic(vOut.ClientVersion.GitVersion)
    if err != nil {
        // ... handle parse error
    }

    // Compare against latest stable K8s minor version
    // Kind supports N-3 minor versions; warn if kubectl is more than 1 minor version off
    // from the default node image
    // ...
}
```

### Pattern 5: Category Organization for New Checks
**What:** New checks register in the AllChecks() slice with appropriate categories.
**Recommended category assignments:**
```go
var allChecks = []Check{
    // Category: Runtime (existing)
    newContainerRuntimeCheck(),
    // Category: Docker (NEW -- configuration checks)
    newDiskSpaceCheck(),          // DOCK-01
    newDaemonJSONCheck(),         // DOCK-02
    newDockerSnapCheck(),         // DOCK-03
    newDockerSocketCheck(),       // DOCK-05
    // Category: Tools (existing + new)
    newKubectlCheck(),
    newKubectlVersionSkewCheck(), // DOCK-04
    // Category: GPU (existing)
    newNvidiaDriverCheck(),
    newNvidiaContainerToolkitCheck(),
    newNvidiaDockerRuntimeCheck(),
}
```
**Rationale:** DOCK-01, DOCK-02, DOCK-03, DOCK-05 are Docker-specific configuration problems grouped under "Docker". DOCK-04 (kubectl version skew) extends the existing "Tools" category alongside the existing kubectl check. GPU checks remain in their own category.

### Anti-Patterns to Avoid
- **Hardcoding a single daemon.json path:** The file lives in 6+ locations depending on platform and Docker install method. Use the candidate list pattern.
- **Using `unix.Statfs` without build tags:** `golang.org/x/sys/unix` does not compile on Windows. The Statfs helper must be in a build-tagged file.
- **Checking disk space on `/` instead of Docker's data root:** Docker's data directory may be on a different partition than root. Query `docker info --format '{{.DockerRootDir}}'` first, then check that path.
- **Platform gating inside Run() methods:** The Check interface's `Platforms()` method handles platform filtering centrally. Individual checks should NOT contain `if runtime.GOOS != "linux"` guards -- return the correct Platforms() slice instead.
- **Importing testutil from pkg/cluster/internal/:** Phase 38 established inline fakeCmd test doubles in `testhelpers_test.go` within the doctor package. Do NOT import from `pkg/cluster/internal/create/actions/testutil/`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Semver comparison for kubectl | String comparison or regex parsing of version numbers | `sigs.k8s.io/kind/pkg/internal/version.ParseSemantic` + `.Minor()` | Already exists; handles "v1.31.0-rc.1" correctly; has `AtLeast`, `LessThan`, `Compare` |
| Disk free space calculation | Shelling out to `df` and parsing text output | `golang.org/x/sys/unix.Statfs` with `Bavail * Bsize` | Direct syscall; no fork/exec; no text parsing; works on Linux and macOS |
| Docker data root detection | Hardcoding `/var/lib/docker` | `docker info --format '{{.DockerRootDir}}'` | Docker's data root is configurable; Docker Desktop uses a different path entirely |
| Command execution | Raw `os/exec.Command` calls | `sigs.k8s.io/kind/pkg/exec.Command` wrapper | Provides OutputLines, CombinedOutputLines, the Cmd interface for fakeCmd injection |
| Symlink resolution for snap detection | Manually reading `/proc/self/exe` or string matching on PATH | `filepath.EvalSymlinks` on the output of `exec.LookPath("docker")` | stdlib; correctly resolves chains of symlinks; handles edge cases |

**Key insight:** Every dependency needed for these 5 checks already exists in the codebase or go.mod. The only change is promoting `golang.org/x/sys` from indirect to direct. The implementation is entirely about wiring existing capabilities through the established Check interface pattern.

## Common Pitfalls

### Pitfall 1: Statfs Bsize Type Differs Between Linux and macOS
**What goes wrong:** On Linux, `unix.Statfs_t.Bsize` is `int64`. On macOS/Darwin, it is `uint32`. Code that assumes one type gets a compile error on the other platform.
**Why it happens:** The underlying C struct differs between Linux (`struct statfs` with `long f_bsize`) and Darwin (`struct statfs64` with `uint32_t f_bsize`).
**How to avoid:** Always cast to `int64` before multiplying: `stat.Bavail * uint64(int64(stat.Bsize))`. Put the Statfs helper in a build-tagged file (`//go:build linux || darwin`) so it compiles correctly on each platform.
**Warning signs:** CI passes on Linux but fails to compile on macOS, or vice versa.

### Pitfall 2: Docker Desktop on macOS Runs in a VM
**What goes wrong:** Running `unix.Statfs("/var/lib/docker")` on macOS returns "no such file or directory" because Docker's data directory is inside a Linux VM, not on the macOS filesystem.
**Why it happens:** Docker Desktop uses a LinuxKit VM. The Docker data root reported by `docker info` is a path inside that VM, not accessible from macOS.
**How to avoid:** On macOS, run `unix.Statfs` on the Docker Desktop VM disk image location (`~/Library/Containers/com.docker.docker/Data/`) or fall back to checking the root filesystem `/`. Alternatively, use `docker system df` for Docker-specific space, though this is slower. The simplest approach: if `statfsFreeBytes` on the Docker root dir fails, fall back to checking `/`.
**Warning signs:** Disk space check always shows "could not check disk space" on macOS.

### Pitfall 3: daemon.json File Absence Is OK, Not An Error
**What goes wrong:** The check reports "warn" or "fail" when no daemon.json exists at any candidate path. But daemon.json is entirely optional -- Docker uses defaults when it is absent.
**Why it happens:** Treating file-not-found the same as file-unreadable.
**How to avoid:** If no candidate path has a readable daemon.json, return "ok" with message "no daemon.json found (Docker using defaults)". Only warn if a file exists AND contains `"init": true`. Return "warn" (not "fail") because daemon.json contains other valid settings besides init.
**Warning signs:** Every macOS user without a ~/.docker/daemon.json gets a warning.

### Pitfall 4: Kubectl Version Skew Without a Running Cluster
**What goes wrong:** The Kubernetes version skew policy compares kubectl client against the API server. But `kinder doctor` runs before any cluster exists, so there is no server version to compare against.
**Why it happens:** The check specification says "detects kubectl client version skew (more than one minor version from the cluster's server version)" but the cluster may not exist.
**How to avoid:** When no cluster is running, compare against a reference version -- either the default kind node image version or the latest stable Kubernetes release. Report the client version and note the comparison basis: "kubectl v1.28.4 (3 minor versions behind latest stable v1.31.0)". If a cluster IS running (detected via existing kind context), compare against actual server version.
**Warning signs:** The check always returns "skip" because it cannot find a server version.

### Pitfall 5: Docker Socket Permission Check on rootless Docker
**What goes wrong:** rootless Docker uses a user-level socket at `$XDG_RUNTIME_DIR/docker.sock` (typically `/run/user/1000/docker.sock`), not `/var/run/docker.sock`. Checking only `/var/run/docker.sock` produces a false "socket not found" error for rootless Docker users.
**Why it happens:** The check hardcodes the standard socket path without considering rootless Docker.
**How to avoid:** Use the error output from `docker info` (which respects `DOCKER_HOST` env var and rootless config) rather than checking a specific socket path. If `docker info` returns "permission denied" in stderr, it is a permission problem regardless of which socket path is used. If `docker info` works (exit 0), permissions are fine -- the existing container-runtime check already validates this.
**Warning signs:** Rootless Docker users get false warnings about socket permissions.

### Pitfall 6: Snap Docker Detection on Systems Where Docker is Also Available via apt
**What goes wrong:** A system might have both `/usr/bin/docker` (apt) and `/snap/bin/docker` (snap) installed. `exec.LookPath("docker")` returns whichever is first on PATH. The check might miss the snap installation or incorrectly report snap when the user is actually using the apt version.
**Why it happens:** PATH ordering determines which binary is found first.
**How to avoid:** After LookPath, resolve the returned path with `filepath.EvalSymlinks`. If the resolved path contains `/snap/`, warn about snap. The resolved path is definitive regardless of PATH ordering.
**Warning signs:** The snap check incorrectly triggers (or does not trigger) based on PATH ordering.

## Code Examples

Verified patterns from the existing codebase and official documentation:

### DOCK-01: Disk Space Check Implementation
```go
// Source: Phase 38 deps struct pattern + golang.org/x/sys/unix.Statfs

type diskSpaceCheck struct {
    readDiskFree func(path string) (uint64, error)
    execCmd      func(name string, args ...string) exec.Cmd
}

func newDiskSpaceCheck() Check {
    return &diskSpaceCheck{
        readDiskFree: statfsFreeBytes, // build-tagged; see disk_unix.go
        execCmd:      exec.Command,
    }
}

func (c *diskSpaceCheck) Name() string       { return "disk-space" }
func (c *diskSpaceCheck) Category() string    { return "Docker" }
func (c *diskSpaceCheck) Platforms() []string { return nil }

func (c *diskSpaceCheck) Run() []Result {
    // Get Docker data root path
    dataRoot := "/var/lib/docker"
    lines, err := exec.OutputLines(c.execCmd("docker", "info", "--format", "{{.DockerRootDir}}"))
    if err == nil && len(lines) > 0 {
        dataRoot = strings.TrimSpace(lines[0])
    }

    freeBytes, err := c.readDiskFree(dataRoot)
    if err != nil {
        // Fall back to root filesystem
        freeBytes, err = c.readDiskFree("/")
        if err != nil {
            return []Result{{
                Name: c.Name(), Category: c.Category(), Status: "warn",
                Message: "Could not check disk space",
                Reason:  "Failed to read filesystem stats",
            }}
        }
    }

    freeGB := float64(freeBytes) / (1024 * 1024 * 1024)

    if freeGB < 2.0 {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "fail",
            Message: fmt.Sprintf("%.1f GB free", freeGB),
            Reason:  "Insufficient disk space for cluster creation (minimum 2 GB)",
            Fix:     "Free disk space: docker system prune && docker image prune -a",
        }}
    }
    if freeGB < 5.0 {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: fmt.Sprintf("%.1f GB free", freeGB),
            Reason:  "Low disk space may cause kubelet eviction during cluster operation",
            Fix:     "Free disk space: docker system prune",
        }}
    }
    return []Result{{
        Name: c.Name(), Category: c.Category(), Status: "ok",
        Message: fmt.Sprintf("%.1f GB free on %s", freeGB, dataRoot),
    }}
}
```

### DOCK-02: daemon.json Init Detection
```go
// Source: Docker docs (daemon.json paths) + kind Known Issues (init:true)

func (c *daemonJSONCheck) Run() []Result {
    candidates := c.daemonJSONCandidates()

    for _, path := range candidates {
        data, err := c.readFile(path)
        if err != nil {
            continue // file not found or unreadable -- try next
        }

        var cfg map[string]interface{}
        if err := json.Unmarshal(data, &cfg); err != nil {
            return []Result{{
                Name: c.Name(), Category: c.Category(), Status: "warn",
                Message: fmt.Sprintf("daemon.json at %s is not valid JSON", path),
                Reason:  "Docker may not start with a malformed daemon.json",
                Fix:     fmt.Sprintf("Validate JSON syntax in %s", path),
            }}
        }

        if initVal, ok := cfg["init"]; ok {
            if boolVal, isBool := initVal.(bool); isBool && boolVal {
                return []Result{{
                    Name: c.Name(), Category: c.Category(), Status: "warn",
                    Message: fmt.Sprintf("\"init\": true detected in %s", path),
                    Reason:  "Docker init=true breaks kind nodes with: Couldn't find an alternative telinit implementation to spawn",
                    Fix:     fmt.Sprintf("Remove \"init\": true from %s and restart Docker", path),
                }}
            }
        }

        // Found a valid daemon.json without init:true
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: fmt.Sprintf("daemon.json checked (%s)", path),
        }}
    }

    // No daemon.json found at any candidate path -- this is fine
    return []Result{{
        Name: c.Name(), Category: c.Category(), Status: "ok",
        Message: "No daemon.json found (Docker using defaults)",
    }}
}
```

### DOCK-03: Docker Snap Detection
```go
// Source: kind Known Issues + kind issue #431

type dockerSnapCheck struct {
    lookPath     func(string) (string, error)
    evalSymlinks func(string) (string, error)
}

func newDockerSnapCheck() Check {
    return &dockerSnapCheck{
        lookPath:     osexec.LookPath,
        evalSymlinks: filepath.EvalSymlinks,
    }
}

func (c *dockerSnapCheck) Name() string       { return "docker-snap" }
func (c *dockerSnapCheck) Category() string    { return "Docker" }
func (c *dockerSnapCheck) Platforms() []string { return []string{"linux"} }

func (c *dockerSnapCheck) Run() []Result {
    dockerPath, err := c.lookPath("docker")
    if err != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "skip",
            Message: "(docker not found)",
        }}
    }

    resolved, err := c.evalSymlinks(dockerPath)
    if err != nil {
        resolved = dockerPath
    }

    if strings.Contains(resolved, "/snap/") {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "warn",
            Message: fmt.Sprintf("Docker installed via snap (%s)", resolved),
            Reason:  "Snap confinement restricts TMPDIR access, which may break kind operations",
            Fix:     "Set TMPDIR to a snap-accessible directory: export TMPDIR=$HOME/tmp && mkdir -p $TMPDIR",
        }}
    }

    return []Result{{
        Name: c.Name(), Category: c.Category(), Status: "ok",
        Message: "Docker not installed via snap",
    }}
}
```

### DOCK-05: Docker Socket Permission Check
```go
// Source: Docker docs (usermod -aG docker) + kind Known Issues

type dockerSocketCheck struct {
    lookPath func(string) (string, error)
    execCmd  func(name string, args ...string) exec.Cmd
}

func newDockerSocketCheck() Check {
    return &dockerSocketCheck{
        lookPath: osexec.LookPath,
        execCmd:  exec.Command,
    }
}

func (c *dockerSocketCheck) Name() string       { return "docker-socket" }
func (c *dockerSocketCheck) Category() string    { return "Docker" }
func (c *dockerSocketCheck) Platforms() []string { return []string{"linux"} }

func (c *dockerSocketCheck) Run() []Result {
    if _, err := c.lookPath("docker"); err != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "skip",
            Message: "(docker not found)",
        }}
    }

    // Use CombinedOutputLines to capture both stdout and stderr
    cmd := c.execCmd("docker", "info")
    lines, err := exec.CombinedOutputLines(cmd)
    if err != nil {
        output := strings.Join(lines, "\n")
        if strings.Contains(strings.ToLower(output), "permission denied") {
            return []Result{{
                Name: c.Name(), Category: c.Category(), Status: "fail",
                Message: "Docker socket permission denied",
                Reason:  "Your user does not have permission to access the Docker daemon",
                Fix:     "sudo usermod -aG docker $USER && newgrp docker",
            }}
        }
        // Docker not responding for other reasons -- already handled by container-runtime check
        return []Result{{
            Name: c.Name(), Category: c.Category(), Status: "ok",
            Message: "Docker socket accessible (daemon may not be running)",
        }}
    }

    return []Result{{
        Name: c.Name(), Category: c.Category(), Status: "ok",
        Message: "Docker socket accessible",
    }}
}
```

### Test Pattern: Table-Driven with Inline Fakes
```go
// Source: Phase 38 test patterns from tools_test.go, runtime_test.go

func TestDaemonJSONCheck_Run(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name           string
        files          map[string]string // path -> content
        wantStatus     string
        wantMsgContain string
    }{
        {
            name:       "no daemon.json exists",
            files:      map[string]string{},
            wantStatus: "ok",
            wantMsgContain: "No daemon.json found",
        },
        {
            name:       "init true detected",
            files:      map[string]string{"/etc/docker/daemon.json": `{"init": true}`},
            wantStatus: "warn",
            wantMsgContain: "init",
        },
        {
            name:       "init false is ok",
            files:      map[string]string{"/etc/docker/daemon.json": `{"init": false}`},
            wantStatus: "ok",
        },
        {
            name:       "valid json without init field",
            files:      map[string]string{"/etc/docker/daemon.json": `{"storage-driver": "overlay2"}`},
            wantStatus: "ok",
        },
        {
            name:       "malformed json",
            files:      map[string]string{"/etc/docker/daemon.json": `{not valid`},
            wantStatus: "warn",
            wantMsgContain: "not valid JSON",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            check := &daemonJSONCheck{
                readFile: func(path string) ([]byte, error) {
                    if content, ok := tt.files[path]; ok {
                        return []byte(content), nil
                    }
                    return nil, os.ErrNotExist
                },
                homeDir: func() (string, error) { return "/home/testuser", nil },
                goos:    "linux",
            }
            results := check.Run()
            if len(results) != 1 {
                t.Fatalf("expected 1 result, got %d", len(results))
            }
            if results[0].Status != tt.wantStatus {
                t.Errorf("Status = %q, want %q", results[0].Status, tt.wantStatus)
            }
            if tt.wantMsgContain != "" && !strings.Contains(results[0].Message, tt.wantMsgContain) {
                t.Errorf("Message = %q, want to contain %q", results[0].Message, tt.wantMsgContain)
            }
        })
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No disk space check | `unix.Statfs` on Docker data root with 2GB/5GB thresholds | Phase 39 (now) | Catches #1 silent failure mode before cluster creation |
| No daemon.json detection | Multi-path candidate search with JSON parsing for init:true | Phase 39 (now) | Prevents the most cryptic kind error ("telinit implementation") |
| No snap Docker detection | Symlink resolution on docker binary path | Phase 39 (now) | Warns about TMPDIR issues before they cause failures |
| Basic kubectl presence check only | Full version skew detection via semver comparison | Phase 39 (now) | Warns about kubectl/server version incompatibility |
| Docker permission errors are opaque | Specific "permission denied" detection with actionable fix | Phase 39 (now) | Provides exact fix command for the #1 new-user issue on Linux |
| `syscall.Statfs` (deprecated) | `golang.org/x/sys/unix.Statfs` | Go 1.4+ recommendation | syscall package is frozen; golang.org/x/sys is the maintained replacement |

**Deprecated/outdated:**
- `syscall.Statfs` / `syscall.Statfs_t`: Deprecated since Go 1.4. Use `golang.org/x/sys/unix` instead. The `syscall` package will not receive updates.
- Checking disk space on hardcoded `/` path: Must use Docker's actual data root from `docker info`.

## Open Questions

1. **DOCK-04: What reference version to compare kubectl against when no cluster exists?**
   - What we know: The Kubernetes version skew policy is +/-1 minor version. `kinder doctor` runs pre-cluster, so there may be no server version.
   - What's unclear: Should we compare against (a) the default kind node image version, (b) the latest stable Kubernetes release, or (c) skip the skew check when no cluster exists?
   - Recommendation: Compare against the latest stable Kubernetes minor version as a constant (e.g., `1.31`). This catches the most common case (stale kubectl from Docker Desktop). Update the constant when bumping the kind node image version. If a cluster is running, prefer the actual server version.

2. **DOCK-01: Should disk space check on macOS check the VM disk or host disk?**
   - What we know: Docker Desktop on macOS runs in a Linux VM. `docker info` reports a data root inside the VM. `unix.Statfs` on the macOS host checks the host filesystem.
   - What's unclear: Should we check the host disk (where the VM disk image lives) or the VM's internal disk?
   - Recommendation: Check the host disk at the path where Docker stores its VM disk image (`~/Library/Containers/com.docker.docker/Data/`), falling back to `/`. The host disk being full prevents the VM from growing, which is the actual user-facing failure mode.

3. **DOCK-05: Should dockerSocketCheck be Linux-only or also run on macOS?**
   - What we know: Docker Desktop on macOS handles socket permissions differently (via a socket proxy). The "permission denied" error is primarily a Linux issue.
   - What's unclear: Whether macOS users ever hit socket permission issues.
   - Recommendation: Make it Linux-only (`Platforms() []string{"linux"}`). macOS Docker Desktop manages permissions automatically. This matches the success criteria which specifies "on Linux."

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
| DOCK-01 | Disk space warn <5GB, fail <2GB, ok >=5GB | unit | `go test ./pkg/internal/doctor/... -run TestDiskSpaceCheck -v` | No -- Wave 0 |
| DOCK-02 | daemon.json init:true across 6+ paths | unit | `go test ./pkg/internal/doctor/... -run TestDaemonJSONCheck -v` | No -- Wave 0 |
| DOCK-03 | Docker snap detection via symlink resolution | unit | `go test ./pkg/internal/doctor/... -run TestDockerSnapCheck -v` | No -- Wave 0 |
| DOCK-04 | kubectl version skew warn when >1 minor version | unit | `go test ./pkg/internal/doctor/... -run TestKubectlVersionSkewCheck -v` | No -- Wave 0 |
| DOCK-05 | Docker socket permission denied detection on Linux | unit | `go test ./pkg/internal/doctor/... -run TestDockerSocketCheck -v` | No -- Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./pkg/internal/doctor/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before verify-work

### Wave 0 Gaps
- [ ] `pkg/internal/doctor/disk_test.go` -- covers DOCK-01 (disk space thresholds, Docker data root detection, Statfs fallback)
- [ ] `pkg/internal/doctor/daemon_test.go` -- covers DOCK-02 (multi-path resolution, init:true detection, missing file OK, malformed JSON)
- [ ] `pkg/internal/doctor/snap_test.go` -- covers DOCK-03 (snap symlink detection, docker not found skip, non-snap OK)
- [ ] `pkg/internal/doctor/versionskew_test.go` -- covers DOCK-04 (version parsing, skew detection, kubectl not found skip)
- [ ] `pkg/internal/doctor/socket_test.go` -- covers DOCK-05 (permission denied detection, accessible OK, docker not found skip)
- [ ] Build-tagged `disk_unix.go` and `disk_other.go` must both compile -- verified by `go build ./...` on CI

## Sources

### Primary (HIGH confidence)
- Kind Known Issues page: https://kind.sigs.k8s.io/docs/user/known-issues/ -- ground truth for init:true, snap TMPDIR, disk space eviction, socket permissions
- Docker Daemon docs: https://docs.docker.com/engine/daemon/ -- daemon.json default paths (Linux: /etc/docker/daemon.json, rootless: $XDG_CONFIG_HOME/docker/daemon.json)
- Kubernetes Version Skew Policy: https://kubernetes.io/docs/reference/kubectl/generated/kubectl_version/ -- kubectl version JSON output format
- `golang.org/x/sys/unix` package docs: https://pkg.go.dev/golang.org/x/sys/unix -- Statfs_t struct definition, Statfs function
- Direct codebase analysis: `pkg/internal/doctor/` -- Phase 38 implemented Check interface, Result type, deps struct pattern, fakeCmd test doubles
- Direct codebase analysis: `pkg/internal/version/version.go` -- ParseSemantic, Minor(), AtLeast, LessThan for version comparison
- Direct codebase analysis: `go.mod` -- golang.org/x/sys v0.41.0 already indirect, Go 1.24+ minimum
- Local `go doc golang.org/x/sys/unix Statfs_t` -- verified Darwin Bsize is uint32, Linux Bsize is int64
- Prior project research: `.planning/research/FEATURES.md` -- check specifications with detection methods and thresholds
- Prior project research: `.planning/research/STACK.md` -- zero new deps confirmation, Statfs implementation patterns
- Prior project research: `.planning/research/PITFALLS.md` -- daemon.json multi-path, proc reads on non-Linux, auto-mitigation safety

### Secondary (MEDIUM confidence)
- Docker snap daemon.json path: https://github.com/docker-archive/docker-snap/issues/22 -- /var/snap/docker/current/config/daemon.json
- Rancher Desktop daemon.json: https://github.com/rancher-sandbox/rancher-desktop/discussions/3371 -- ~/.docker/daemon.json
- Docker Desktop macOS FAQ: https://docs.docker.com/desktop/troubleshoot-and-support/faqs/macfaqs/ -- disk image location ~/Library/Containers/com.docker.docker/
- Go cross-platform Statfs example: https://github.com/gokcehan/lf/blob/master/df_statfs.go -- int64(stat.Bsize) cast pattern

### Tertiary (LOW confidence)
- None -- all findings verified against codebase or official documentation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- zero new external deps for 4/5 checks; golang.org/x/sys already in go.mod at v0.41.0; verified Statfs_t types locally via `go doc`
- Architecture: HIGH -- exact same pattern as Phase 38 (deps struct, fakeCmd, AllChecks registry); no architectural innovation needed
- Pitfalls: HIGH -- all pitfalls identified from prior research (PITFALLS.md, FEATURES.md) and verified against official documentation; Statfs type mismatch verified by cross-platform `go doc`
- Code examples: HIGH -- patterns directly extend Phase 38 implementations; daemon.json paths verified against Docker docs, snap docs, and Rancher Desktop discussions

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable domain -- Go stdlib, Docker config paths, kind Known Issues are mature)
