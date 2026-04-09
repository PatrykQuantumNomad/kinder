# Phase 45: Host-Directory Mounting - Research

**Researched:** 2026-04-09
**Domain:** Go pre-flight validation, mount propagation platform warnings, `kinder doctor` check pattern, Astro documentation
**Confidence:** HIGH — grounded entirely in direct codebase analysis of kinder and live inspection of the Docker Desktop settings JSON on macOS

---

## Summary

Phase 45 adds four concrete capabilities to kinder: pre-flight host-path existence validation before container creation (MOUNT-01), a platform warning for unsupported propagation modes on macOS/Windows (MOUNT-02), a `kinder doctor` check for host mount paths and Docker Desktop file-sharing on macOS (MOUNT-03), and documentation of the two-hop host→node→pod mount pattern (MOUNT-04).

The implementation splits cleanly into three work units: (1) pre-flight validation + propagation warning injected into the `Cluster()` create flow in `create.go`, (2) a new `hostmount.go` doctor check registered in `check.go`, and (3) a new guide page in `kinder-site/src/content/docs/guides/`. No new Go module dependencies are required. All four requirements use patterns already established in the codebase.

The trickiest design decision is **where** to place the pre-flight host-path check. It must run before `p.Provision()` is called (before any containers are created) but after `opts.Config.Validate()`. The correct insertion point is in `Cluster()` in `pkg/cluster/internal/create/create.go`, immediately after the existing `opts.Config.Validate()` call. This is provider-agnostic and avoids duplicating logic across docker/podman/nerdctl providers. The propagation warning belongs at the same point — one runtime.GOOS switch, similar to `logMetalLBPlatformWarning()` which already exists in the same file.

The `kinder doctor` check for MOUNT-03 uses the established `Check` interface from `pkg/internal/doctor/check.go`. The Docker Desktop file-sharing check reads `~/Library/Group Containers/group.com.docker/settings-store.json` and parses the `FilesharingDirectories` JSON array. This is the canonical location confirmed by live inspection on macOS. The default shared directories are `/Users`, `/Volumes`, `/private`, `/tmp`, `/var/folders`. Any configured host path not covered by these (or a user-added prefix) should trigger an actionable warning.

**Primary recommendation:** Implement in three sequential tasks: (45-01) pre-flight + propagation warning in `create.go`; (45-02) `hostmount.go` doctor check + registry entry; (45-03) documentation guide page.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os` (stdlib) | Go stdlib | `os.Stat(hostPath)` for path existence, `os.ReadFile(settingsPath)`, `os.UserHomeDir()` | Used throughout the doctor package |
| `encoding/json` (stdlib) | Go stdlib | Parse Docker Desktop `settings-store.json` | Used in `daemon.go` already |
| `path/filepath` (stdlib) | Go stdlib | `filepath.HasPrefix`-style prefix check for file-sharing directories | Used in docker/nerdctl create for absolute paths |
| `runtime` (stdlib) | Go stdlib | `runtime.GOOS` for platform detection | Used in `create.go`'s `logMetalLBPlatformWarning` and multiple doctor checks |
| `sigs.k8s.io/kind/pkg/errors` | in-repo | Structured error formatting | Project-wide standard |
| `sigs.k8s.io/kind/pkg/internal/apis/config` | in-repo | `config.Mount`, `config.MountPropagationNone` type references | Already used everywhere |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `strings` (stdlib) | Go stdlib | `strings.HasPrefix()` for path-prefix file-sharing check | Only in doctor check |
| `osexec "os/exec"` (stdlib) | Go stdlib | `osexec.LookPath` for runtime detection guard | Used in every doctor check |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Reading `settings-store.json` for file-sharing check | Running `docker info` or `docker inspect` | `docker info` does not expose file-sharing directories (confirmed). `settings-store.json` is the only machine-readable source. |
| Pre-flight check in `create.go` | Pre-flight check inside each provider's `planCreation` | Provider-per-check would require 3× duplication (docker/podman/nerdctl). `create.go` is provider-agnostic and is already where config-level validation runs. |
| Warn-and-continue propagation warning | Hard-fail on non-None propagation | Phase requirement says "emits a visible warning" not "blocks creation". Matches how MetalLB platform warning works. |

**Installation:** No new packages. Zero go.mod changes required.

---

## Architecture Patterns

### Recommended Project Structure

```
pkg/cluster/internal/create/
└── create.go          # Add validateExtraMounts() + logMountPropagationPlatformWarning()
    create_test.go     # (or fixup_test.go) — unit tests for the two new functions

pkg/internal/doctor/
├── hostmount.go       # New file: hostMountPathCheck + dockerDesktopFileSharingCheck
└── hostmount_test.go  # Tests for both checks

kinder-site/src/content/docs/guides/
└── host-directory-mounting.md   # New guide: two-hop mount pattern
```

### Pattern 1: Pre-flight Validation in `create.go`

**What:** A function `validateExtraMounts(cfg *config.Cluster) error` called between `opts.Config.Validate()` and `p.Provision()` in the `Cluster()` function. It iterates all `node.ExtraMounts`, resolves relative paths to absolute (matching the docker/nerdctl behavior already in `planCreation`), and calls `os.Stat` on each `HostPath`. Returns a named error before any containers are created.

**When to use:** Always, regardless of provider.

**Insertion point in `create.go`:**
```go
// then validate
if err := opts.Config.Validate(); err != nil {
    return err
}

// NEW: pre-flight host path existence check (MOUNT-01)
if err := validateExtraMounts(opts.Config); err != nil {
    return err
}

// setup a status object to show progress to the user
status := cli.StatusForLogger(logger)
```

**Example implementation:**
```go
// validateExtraMounts checks that all hostPath entries in ExtraMounts exist
// before any containers are created. Returns an error identifying the first
// missing path and which node it belongs to.
func validateExtraMounts(cfg *config.Cluster) error {
    for i, node := range cfg.Nodes {
        for j, mount := range node.ExtraMounts {
            hostPath := mount.HostPath
            if !filepath.IsAbs(hostPath) {
                abs, err := filepath.Abs(hostPath)
                if err != nil {
                    return errors.Errorf("node[%d] extraMount[%d]: cannot resolve path %q: %v", i, j, hostPath, err)
                }
                hostPath = abs
            }
            if _, err := os.Stat(hostPath); err != nil {
                if os.IsNotExist(err) {
                    return errors.Errorf("node[%d] extraMount[%d]: host path %q does not exist", i, j, hostPath)
                }
                return errors.Errorf("node[%d] extraMount[%d]: cannot access host path %q: %v", i, j, hostPath, err)
            }
        }
    }
    return nil
}
```

### Pattern 2: Platform Warning for Propagation Mode (MOUNT-02)

**What:** A function `logMountPropagationPlatformWarning(logger, cfg)` called in `Cluster()` after `validateExtraMounts()` but still before `p.Provision()`. Mirrors `logMetalLBPlatformWarning()` exactly.

**When to use:** On macOS and Windows when any mount has non-None propagation.

**Example implementation:**
```go
func logMountPropagationPlatformWarning(logger log.Logger, cfg *config.Cluster) {
    switch runtime.GOOS {
    case "darwin", "windows":
        for _, node := range cfg.Nodes {
            for _, mount := range node.ExtraMounts {
                if mount.Propagation != config.MountPropagationNone && mount.Propagation != "" {
                    logger.Warnf(
                        "On %s, mount propagation mode %q is not supported by Docker Desktop "+
                            "and will be silently treated as None. "+
                            "Use propagation: None to suppress this warning.",
                        runtime.GOOS, mount.Propagation,
                    )
                    return // warn once
                }
            }
        }
    }
}
```

### Pattern 3: Doctor Check (`hostmount.go`)

**What:** Two new `Check` implementations in a single new file `pkg/internal/doctor/hostmount.go`, registered in `allChecks` in `check.go`. Follows the exact struct/method pattern of all existing checks (e.g., `localPathCVECheck`, `daemonJSONCheck`).

**Check 1 — `hostMountPathCheck`:**
- Name: `"host-mount-path"`
- Category: `"Mounts"`
- Platforms: `nil` (all platforms)
- Logic: Reads the kinder cluster config (if any) and verifies each `extraMounts[].hostPath` exists on the host filesystem. Skips when no config file is found. Returns `ok`/`warn`/`fail` per path.
- **Note:** Because `kinder doctor` is a standalone command that doesn't receive cluster config as input, the check must either (a) look for a default config file at a known path, (b) accept the config path as an injected dependency, or (c) check paths that are passed in via an injected getter. **Best approach:** inject a `getMountPaths func() []string` for testability, defaulting to reading the active kinder cluster config file. However, if no config is available at runtime (the check runs without context), the check should skip gracefully. This is a design open question — see Open Questions below.

**Check 2 — `dockerDesktopFileSharingCheck`:**
- Name: `"docker-desktop-file-sharing"`
- Category: `"Mounts"`
- Platforms: `[]string{"darwin"}` (macOS only)
- Logic: 
  1. Read `~/Library/Group Containers/group.com.docker/settings-store.json`
  2. Parse JSON → extract `FilesharingDirectories []string`
  3. For each mount path from the config: check if any entry in `FilesharingDirectories` is a prefix of the absolute mount path
  4. If not covered → `warn` with actionable guidance
- **Confirmed default shared paths on macOS:** `/Users`, `/Volumes`, `/private`, `/tmp`, `/var/folders` (verified from live `settings-store.json` at `~/Library/Group Containers/group.com.docker/settings-store.json`)

**Example `dockerDesktopFileSharingCheck` skeleton:**
```go
type dockerDesktopFileSharingCheck struct {
    readFile func(string) ([]byte, error)
    homeDir  func() (string, error)
    getMountPaths func() []string // paths to verify
}

func (c *dockerDesktopFileSharingCheck) Name() string       { return "docker-desktop-file-sharing" }
func (c *dockerDesktopFileSharingCheck) Category() string    { return "Mounts" }
func (c *dockerDesktopFileSharingCheck) Platforms() []string { return []string{"darwin"} }

func isPathCovered(path string, sharedDirs []string) bool {
    for _, dir := range sharedDirs {
        if strings.HasPrefix(path, dir+"/") || path == dir {
            return true
        }
    }
    return false
}
```

### Pattern 4: Documentation Guide (MOUNT-04)

**What:** A new Markdown file in `kinder-site/src/content/docs/guides/host-directory-mounting.md` using Astro Starlight frontmatter. Follows the structure of existing guides (`local-dev-workflow.md`, `hpa-auto-scaling.md`).

**Content required per MOUNT-04:**
- Explanation of the two-hop pattern: host dir → node `extraMounts` (binds into container) → pod `hostPath` PV
- Complete YAML example showing: cluster config with `extraMounts`, a `PersistentVolume` using `hostPath`, a `PersistentVolumeClaim`, and a pod consuming the PVC
- Platform notes (macOS Docker Desktop file sharing requirement, propagation limitations)
- `kinder doctor` tip for the new checks

**Frontmatter pattern from existing guides:**
```yaml
---
title: Host Directory Mounting
description: Mount host directories into cluster nodes and expose them to pods via hostPath PersistentVolumes.
---
```

### Anti-Patterns to Avoid

- **Duplicating pre-flight validation in each provider:** Don't add existence checks inside `docker/create.go`, `nerdctl/create.go`, and `podman/provision.go`. All three already call `filepath.Abs` on `ExtraMounts` — they handle relative paths but do NOT check existence. The single `validateExtraMounts()` in `create.go` is the right location.
- **Hard-failing on non-None propagation:** The requirement says "emits a visible warning" — do not return an error. Match the MetalLB pattern exactly.
- **Checking Docker-only propagation behavior in all providers:** The propagation warning is about Docker Desktop on macOS/Windows, not about Linux Docker. Podman on Linux supports propagation natively. The `runtime.GOOS` guard is correct and sufficient.
- **Making the doctor check load a full cluster config:** Keep the doctor check simple. If config is unavailable, `skip`. Do not replicate the full config loading pipeline inside the doctor package.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Absolute path resolution | Custom path-walking | `filepath.Abs()` | Already used in docker/nerdctl/podman providers |
| File-sharing prefix matching | Regex or complex tree walk | `strings.HasPrefix()` with `/`-terminated dir prefix | Simple and correct for directory prefix matching |
| Platform detection | Custom OS detection | `runtime.GOOS` switch | Already used in `create.go` and multiple doctor checks |
| JSON settings parsing | Regexp parsing of settings file | `encoding/json` → `map[string]interface{}` or specific struct | Same approach as `daemonJSONCheck` |

**Key insight:** All required logic uses Go stdlib primitives. The complexity is in understanding the design intent (where to plug in, what to skip vs fail), not in the implementation.

---

## Common Pitfalls

### Pitfall 1: Mis-Scoping the Pre-flight Check

**What goes wrong:** Adding the host-path existence check inside `planCreation` in each provider package. This duplicates code across three files and means the check runs inside `Provision()` — after the status spinner has started and after node names have been computed — producing a confusing error mid-creation.

**Why it happens:** The providers are where `ExtraMounts` are consumed (for `--volume` arg construction), so it seems natural to validate there.

**How to avoid:** Add `validateExtraMounts()` in `create.go`, called between `opts.Config.Validate()` and `p.Provision()`. This is consistent with where `validateProvider()` and `alreadyExists()` run.

**Warning signs:** If the error message appears after "Preparing nodes 📦 📦" has printed, the check is too late.

### Pitfall 2: Docker Desktop File-Sharing False Negatives

**What goes wrong:** The doctor check passes even when a path is not shared, because the prefix comparison doesn't account for `/`-terminated directory entries.

**Why it happens:** `strings.HasPrefix("/Users/foo", "/Users")` returns true, but so does `strings.HasPrefix("/Userspace/foo", "/Users")`. The correct check is `strings.HasPrefix(path, dir+"/") || path == dir`.

**How to avoid:** Use the `isPathCovered(path, sharedDirs)` helper shown in Pattern 3 above. Include a unit test with `/Userspace/foo` as a test case.

### Pitfall 3: AllChecks Registry Count Test

**What goes wrong:** Adding two new doctor checks without updating the count assertion in `check_test.go`.

**Why it happens:** `check_test.go` has a test that asserts the exact length of `AllChecks()`. When Phase 44 added `newLocalPathCVECheck`, the count was updated to 21. Adding 2 new checks in Phase 45 means updating it to 23.

**How to avoid:** Grep for the count assertion in `check_test.go` before and after adding new checks. Update accordingly.

### Pitfall 4: Doctor Check Graceful Skip When No Mount Config

**What goes wrong:** The `hostMountPathCheck` fails with a confusing error because it can't find a kinder config file to read mount paths from.

**Why it happens:** `kinder doctor` runs standalone — it doesn't receive the cluster config as input. There is no active config at runtime unless the user ran `kinder create cluster --config file.yaml`.

**How to avoid:** The check must gracefully return `skip` when no mount paths are available. The injected `getMountPaths` function should return an empty slice when no config is accessible. See Open Questions for the recommended approach.

### Pitfall 5: Windows Path Handling

**What goes wrong:** `filepath.Abs` and `strings.HasPrefix` behave differently on Windows (backslash paths, drive letters).

**Why it happens:** The doctor check is designed for macOS/darwin platform only for file-sharing, but the pre-flight check in `create.go` must work on all platforms including Windows.

**How to avoid:** Use `filepath.Abs()` (which handles OS-specific separators) for path resolution. The `strings.HasPrefix` comparison in the doctor check is only used on macOS (platform-gated), so Windows path format is not a concern there.

---

## Code Examples

Verified patterns from existing codebase:

### Platform Warning Pattern (from `create.go`)
```go
// Source: pkg/cluster/internal/create/create.go:438-450
func logMetalLBPlatformWarning(logger log.Logger) {
    switch runtime.GOOS {
    case "darwin", "windows":
        logger.Warnf(
            "On %s, MetalLB LoadBalancer IPs are not directly reachable from the host.\n"+
                "   Use kubectl port-forward to access LoadBalancer services:\n"+
                "   kubectl port-forward svc/<service-name> <local-port>:<service-port>",
            runtime.GOOS,
        )
    }
}
```

### Doctor Check With Injected readFile and homeDir (from `daemon.go`)
```go
// Source: pkg/internal/doctor/daemon.go
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
```

### ExtraMounts Abs Path Resolution (from docker/create.go)
```go
// Source: pkg/cluster/internal/providers/docker/create.go:88-96
for m := range node.ExtraMounts {
    hostPath := node.ExtraMounts[m].HostPath
    if !fs.IsAbs(hostPath) {
        absHostPath, err := filepath.Abs(hostPath)
        if err != nil {
            return nil, errors.Wrapf(err, "unable to resolve absolute path for hostPath: %q", hostPath)
        }
        node.ExtraMounts[m].HostPath = absHostPath
    }
}
```

### Doctor Check Registry Entry (from `check.go`)
```go
// Source: pkg/internal/doctor/check.go
var allChecks = []Check{
    // ...existing 21 entries...
    // Category: Mounts (Phase 45)
    newHostMountPathCheck(),
    newDockerDesktopFileSharingCheck(),
}
```

### Docker Desktop Settings File Key
```
Path: ~/Library/Group Containers/group.com.docker/settings-store.json
Key: "FilesharingDirectories"
Type: []string
Default value: ["/Users", "/Volumes", "/private", "/tmp", "/var/folders"]
```

### Two-Hop Mount YAML (for MOUNT-04 documentation)
```yaml
# Cluster config: mount host directory into node
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
  - role: control-plane
    extraMounts:
      - hostPath: /Users/you/data          # host directory
        containerPath: /mnt/host-data      # path inside the node container
---
# PersistentVolume: expose node path to Kubernetes
apiVersion: v1
kind: PersistentVolume
metadata:
  name: host-data-pv
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /mnt/host-data                   # matches containerPath above
---
# PersistentVolumeClaim
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: host-data-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
---
# Pod consuming the PVC
apiVersion: v1
kind: Pod
metadata:
  name: data-reader
spec:
  containers:
    - name: reader
      image: busybox
      command: ["sh", "-c", "ls /data && sleep 3600"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: host-data-pvc
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Docker Desktop settings at `settings.json` | Docker Desktop settings at `settings-store.json` | Docker Desktop 4.x+ | The new path is confirmed by live inspection on macOS 25.3.0 with Docker Desktop. The old `settings.json` may not exist on new installs. |
| `MountPropagationBidirectional` → `rshared` on all platforms | `rshared` only works on Linux; silently ignored on Docker Desktop macOS/Windows | N/A (always been this way) | The warning in MOUNT-02 is new UX; the underlying behavior was always provider-limited |

**Deprecated/outdated:**
- `~/Library/Group Containers/group.com.docker/settings.json`: May not exist on Docker Desktop 4.x+. Always check `settings-store.json` first; fall back gracefully.

---

## Open Questions

1. **How does `hostMountPathCheck` obtain mount paths at runtime?**
   - What we know: `kinder doctor` runs standalone without a cluster config path flag. The doctor package has no access to the current cluster config.
   - What's unclear: Should the check skip entirely when no config is provided? Should `kinder doctor` gain a `--config` flag? Or should the check read a well-known config path (e.g., the default `kind` config)?
   - Recommendation: For Phase 45, make `hostMountPathCheck` **skip gracefully** when no config paths are injected. The check is most useful when MOUNT-01 (pre-flight validation) doesn't catch the issue (e.g., path existed at create time but was later deleted). Inject `getMountPaths func() []string` defaulting to a no-op that returns `nil`. The planner should decide whether to wire up config reading or leave it as a skip-by-default check. This is the **primary planning decision** that needs a concrete answer.

2. **Should the propagation warning be emitted for Podman on macOS?**
   - What we know: Docker Desktop on macOS does not support `rslave`/`rshared`. Podman on macOS (via a VM) may have different behavior.
   - What's unclear: Is the propagation limitation specific to Docker Desktop or all container runtimes on macOS?
   - Recommendation: Scope the warning to `runtime.GOOS == "darwin" || runtime.GOOS == "windows"` without runtime-specific discrimination. The warning is correct for Docker Desktop (the dominant case) and errs on the side of informing users.

3. **Should MOUNT-01 use `os.Stat` or `fs.IsAbs` + `os.Stat`?**
   - What we know: The docker and nerdctl providers use `fs.IsAbs(hostPath)` (from `sigs.k8s.io/kind/pkg/fs`) to check for absolute paths before calling `filepath.Abs`. The `fs` package is a thin wrapper.
   - Recommendation: Use `filepath.IsAbs(hostPath)` (stdlib) in `create.go` for consistency with Go stdlib. The `fs.IsAbs` usage in provider code is a legacy pattern predating the `create.go` validation location.

---

## Sources

### Primary (HIGH confidence)
- Direct codebase analysis: `pkg/cluster/internal/create/create.go` — create flow, validation insertion point, MetalLB platform warning pattern
- Direct codebase analysis: `pkg/internal/doctor/check.go` — `Check` interface, `allChecks` registry, `RunAllChecks`, platform filtering
- Direct codebase analysis: `pkg/internal/docker/create.go`, `nerdctl/create.go`, `podman/provision.go` — ExtraMounts absolute-path handling (confirming no existence check exists)
- Direct codebase analysis: `pkg/internal/doctor/daemon.go` — injected readFile/homeDir pattern
- Direct codebase analysis: `pkg/internal/doctor/localpath.go` — CVE check pattern for injected dependencies
- Direct codebase analysis: `pkg/internal/apis/config/types.go` — `Mount` struct, `MountPropagation` constants
- Live macOS system file: `~/Library/Group Containers/group.com.docker/settings-store.json` — `FilesharingDirectories` key confirmed with values `["/Users", "/Volumes", "/private", "/tmp", "/var/folders"]`
- Direct codebase analysis: `kinder-site/src/content/docs/guides/local-dev-workflow.md` — guide page structure/frontmatter pattern

### Secondary (MEDIUM confidence)
- Docker Desktop docs (fetched): `FilesharingDirectories` in `settings-store.json` confirmed as the canonical location
- Docker Desktop docs (fetched): Default shared directories on macOS confirmed: `/Users`, `/Volumes`, `/private`, `/tmp`, `/var/folders`
- Docker docs (fetched): `docker info` does NOT expose file-sharing directories (confirmed — no shared-paths field in output)

### Tertiary (LOW confidence)
- None — all critical claims verified by live inspection or codebase reading.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages are in-repo stdlib, verified by usage in existing code
- Architecture: HIGH — insertion points verified by reading actual file contents; no guessing
- Pitfalls: HIGH — pitfall #2 (prefix matching) and #3 (count test) verified by reading the actual code
- Docker Desktop file-sharing detection: HIGH — verified by reading live `settings-store.json` on macOS 25.3.0

**Research date:** 2026-04-09
**Valid until:** 2026-05-09 (Docker Desktop settings file path is stable; check if Docker Desktop major version changes)
