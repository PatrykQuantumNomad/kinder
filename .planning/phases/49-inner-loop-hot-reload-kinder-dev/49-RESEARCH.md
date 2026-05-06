# Phase 49: Inner-Loop Hot Reload (`kinder dev`) - Research

**Researched:** 2026-05-06
**Domain:** Go CLI (Cobra), file-watching, Docker build, kinder load images pipeline, kubectl rollout
**Confidence:** HIGH (codebase fully verified; external library verified via Context7 + Go proxy)

---

## Summary

Phase 49 adds `kinder dev --watch <dir> --target <deployment>` — a watch-mode command that runs a build → load → rollout cycle every time source files change. The command is new but reuses the project's established patterns without exception.

The load images pipeline (`pkg/cmd/kind/load/images/images.go`) has all its business logic in an unexported `runE` function. `kinder dev` cannot call that function directly. Instead, it must replicate or extract the core load logic into a new internal helper package (`pkg/internal/dev/`) that both the CLI command and the new `kinder dev` command call programmatically. The pattern for this extraction is already established by `pkg/internal/lifecycle/` (shared between `kinder pause`, `kinder resume`, `kinder status`).

The critical dependency question is fsnotify vs pure stdlib polling. Research finds that fsnotify v1.10.1 (the current release) provides only native OS notification backends (inotify/kqueue/ReadDirectoryChangesW) — there is **no polling backend** in fsnotify. For the `--poll` flag (DEV-05), the planner must implement a stdlib polling loop using `os.ReadDir` + `os.Stat.ModTime` as a separate code path. Given the project's near-zero-dep stance and the fact that fsnotify only introduces `golang.org/x/sys` as a transitive dep (already present in `go.mod`), adding fsnotify is acceptable. The recommendation is: add fsnotify for the default event-driven path and implement stdlib polling for the `--poll` path — no additional dependencies.

**Primary recommendation:** Add `pkg/cmd/kind/dev/dev.go` (top-level command) + `pkg/internal/dev/` (business logic) following the `pause`/`lifecycle` split. Use fsnotify for the default watcher, stdlib for `--poll`, and wire into `root.go` exactly as `pause` and `snapshot` are wired.

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DEV-01 | User can run `kinder dev --watch <dir> --target <deployment>` to watch a directory and trigger a build-load-rollout pipeline on file changes | Section: CLI Command Structure, File-Watching Approach |
| DEV-02 | `kinder dev` builds an image from the watched directory (Dockerfile-based) and imports it via the existing `kinder load images` pipeline | Section: Docker Build Invocation, Load Images Pipeline |
| DEV-03 | After successful image load, `kinder dev` rolls the target Deployment via `kubectl rollout restart` and waits for ready | Section: kubectl Rollout Pattern |
| DEV-04 | `kinder dev` debounces rapid file changes (configurable, default 500ms) and shows build/load/rollout timing per cycle | Section: Debouncer Pattern, Timing/UX |
| DEV-05 | `kinder dev` supports `--poll` flag for fsnotify-unfriendly environments (Docker Desktop volume mounts on macOS); polls watched directory at configurable interval | Section: File-Watching Approach |
</phase_requirements>

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| File watching / polling | Host process | — | Host owns the source directory; events arrive on the host FS |
| Docker image build | Host CLI (`docker build`) | — | Dockerfile lives on the host; docker daemon is on the host |
| Image load into nodes | Host process → node containerd | — | Reuses existing `kinder load images` path: save TAR on host, import into each node |
| kubectl rollout restart | Host kubectl + KUBECONFIG | — | `kinder dev` is a host command; rollout must be via host kubectl using the cluster's kubeconfig, not via node.CommandContext (inside-node pattern used only during cluster create actions) |
| Per-cycle timing / UX | Host process stdout | — | Printed to `streams.Out` per established kinder CLI convention |
| Debouncing | Host process (goroutine) | — | Pure in-process Go timer logic, no external service |

**Key architecture note:** All existing `kubectl rollout` calls in the codebase go through `node.CommandContext` (i.e., they exec kubectl *inside* a node container using `/etc/kubernetes/admin.conf`). For `kinder dev`, the rollout target is a user Deployment — not a system addon — and the user is on the host. The command must use the host `kubectl` binary with the cluster's kubeconfig obtained via `provider.KubeConfig(clusterName, false)`. This is a new pattern in the codebase; document it explicitly in the plan.

---

## 1. Existing `kinder load images` Pipeline

**File:** `pkg/cmd/kind/load/images/images.go`
**Package:** `sigs.k8s.io/kind/pkg/cmd/kind/load/images` [VERIFIED: codebase read]

### What the pipeline does (verified line-by-line)

```
runE(logger, flags, args) error
  1. cluster.NewProvider + providerBinaryName() → detects docker/podman/nerdctl
  2. For each image name: exec "<binary> image inspect -f {{ .Id }}" → get image ID
  3. provider.ListInternalNodes(flags.Name) → all cluster nodes
  4. Smart-load: check each node for existing image by ID via nodeutils.ImageTags
     - If ID present but tag missing → nodeutils.ReTagImage
     - If absent → mark node as candidate
  5. fs.TempDir() → staging area
  6. save(binaryName, imageNames, imagesTarPath) → "<binary> save -o dest <images...>"
  7. errors.UntilErrorConcurrent: for each selected node →
       nodeutils.LoadImageArchiveWithFallback(node, func() (io.ReadCloser, error) { os.Open(tar) })
```

### Key types and functions to reuse for DEV-02

| Symbol | Package | Role |
|--------|---------|------|
| `cluster.NewProvider` | `sigs.k8s.io/kind/pkg/cluster` | Create provider |
| `runtime.GetDefault(logger)` | `sigs.k8s.io/kind/pkg/internal/runtime` | Detect KIND_EXPERIMENTAL_PROVIDER |
| `provider.ListInternalNodes(name)` | `sigs.k8s.io/kind/pkg/cluster` | List cluster nodes |
| `nodeutils.ImageTags` | `sigs.k8s.io/kind/pkg/cluster/nodeutils` | Check if image exists on node |
| `nodeutils.LoadImageArchiveWithFallback` | `sigs.k8s.io/kind/pkg/cluster/nodeutils` | Import TAR into node with Docker Desktop fallback |
| `fs.TempDir` | `sigs.k8s.io/kind/pkg/fs` | Staging temp dir |
| `errors.UntilErrorConcurrent` | `sigs.k8s.io/kind/pkg/errors` | Concurrent per-node load |
| `exec.Command(binaryName, "save", ...)` | `sigs.k8s.io/kind/pkg/exec` | Save image to TAR |

### How `kinder dev` calls the pipeline (DEV-02)

The `runE` function in `images.go` is unexported and takes a `flagpole` struct. `kinder dev` cannot call it directly. Two viable approaches:

**Option A (recommended):** Extract the core logic into a new `pkg/internal/dev/` package as a `LoadImages(opts LoadImagesOptions) error` function — modeled exactly on how `lifecycle.Pause(opts PauseOptions)` wraps the pause logic. The `kinder dev` command calls `dev.LoadImages(...)`. The `kinder load images` command can remain unchanged (it re-implements the same logic internally).

**Option B (simpler, slight duplication):** `kinder dev` replicates the save/load sequence inline using the same public APIs (`cluster.NewProvider`, `nodeutils.LoadImageArchiveWithFallback`, etc.). Given that the load sequence is ~10 lines of API calls on established types, this is acceptable for the first implementation — less abstraction overhead, no new package boundary. Choose based on whether the planner wants testability via injection.

**Recommendation:** Option A (new `pkg/internal/dev/` package) because:
1. The test pattern for kinder commands requires an injected function var (like `pauseFn`, `createFn`).
2. The load logic will be called in a loop across many cycles — having a named boundary aids testing.
3. The `loadImagesFn` pattern already established in other commands makes unit testing trivial.

### Image name for DEV-02 cycles

`kinder dev` must pick a stable image name so the Deployment's `image:` field stays constant across cycles. The build step should tag with a fixed name like `kinder-dev/<deployment>:latest` (or user-specified via `--image`). The Deployment must already reference this tag, or the user must patch it before the first `kinder dev` run. The RESEARCH cannot determine the Deployment's `image:` field value — this is a user constraint, not a kinder concern. The `--target` flag accepts a deployment name; `kinder dev` passes the `--image` flag value to `docker build -t`. [ASSUMED: the user has already deployed with imagePullPolicy: Never or Always, and the Deployment's image field matches what kinder dev builds. If not, the rollout may not pick up the new image.]

---

## 2. CLI Command Structure

**Pattern:** Single command, not a subcommand group. Parallel to `kinder pause`, `kinder resume`. [VERIFIED: root.go]

### Registration location

`pkg/cmd/kind/root.go` is where all top-level commands are registered via `cmd.AddCommand(...)`. After Phase 49, it should add:

```go
// pkg/cmd/kind/root.go — in NewCommand():
import "sigs.k8s.io/kind/pkg/cmd/kind/dev"
// ...
cmd.AddCommand(dev.NewCommand(logger, streams))
```

### New file locations

```
pkg/cmd/kind/dev/
└── dev.go           # NewCommand(...) + runE(...) for kinder dev
└── dev_test.go      # unit tests via injected devFn

pkg/internal/dev/
└── dev.go           # DevOptions, DevResult, business logic (build/load/rollout/watch)
└── dev_test.go      # unit tests for business logic
```

### Command signature (derived from requirements)

```go
// Use: "dev --watch <dir> --target <deployment>"
// Required flags: --watch, --target
// Optional flags: --name (cluster, default "kind"), --image, --debounce (500ms),
//                 --poll (bool), --poll-interval (1s), --namespace (default "default"),
//                 --rollout-timeout (2m)
```

### Flag conventions observed in the codebase [VERIFIED]

| Convention | Evidence |
|------------|---------|
| `--name` / `-n` for cluster name with default `cluster.DefaultName` | `load/images/images.go:69` |
| `DurationVar` for timeout flags, never bare integers | `pause.go:79`, STATE.md entry 47-06 |
| `BoolVar` for boolean flags | `pause.go:80`, `create.go:66` |
| `cli.OverrideDefaultName(cmd.Flags())` called in RunE before business logic | Every load/create/pause command |
| `SilenceUsage: true`, `SilenceErrors: true` on root | `root.go:58-59` |
| `cobra.NoArgs` for parent groups, specific validators for leaves | `load.go:35`, `pause.go:68` |

---

## 3. kubectl Rollout Pattern (DEV-03)

### Existing in-node rollout pattern [VERIFIED: corednstuning.go]

Existing code runs kubectl **inside the node container** using `node.CommandContext`:

```go
// pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go
// rollout restart inside node:
node.CommandContext(ctx.Context,
    "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
    "rollout", "restart",
    "--namespace=kube-system",
    "deployment/coredns",
).Run()

// rollout status inside node:
node.CommandContext(ctx.Context,
    "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
    "rollout", "status",
    "--namespace=kube-system",
    "deployment/coredns",
    "--timeout=60s",
).Run()
```

This pattern works for system addons during cluster creation where kubectl is available inside the node image. It is **not appropriate** for `kinder dev` because:
1. The target Deployment is user-defined (not a system addon).
2. `kinder dev` is a host command that should use the user's existing kubectl and KUBECONFIG.
3. The host kubectl is already confirmed available (`/Users/patrykattc/work/google-cloud-sdk/bin/kubectl` v1.32.4).

### Correct pattern for kinder dev (host-side kubectl)

`kinder dev` should use the **host kubectl** with the cluster's kubeconfig written to a temp file. The kubeconfig is obtained via `provider.KubeConfig(clusterName, false)` — `false` = external IP (host-reachable). [VERIFIED: `provider.go:220`, `kubeconfig.go:53`]

```go
// Source: sigs.k8s.io/kind/pkg/cluster.Provider
kubeconfigContent, err := provider.KubeConfig(clusterName, false)
// write to temp file
f, _ := os.CreateTemp("", "kinder-dev-*.kubeconfig")
f.WriteString(kubeconfigContent)
f.Close()
defer os.Remove(f.Name())

// rollout restart (host kubectl)
exec.Command("kubectl", "--kubeconfig="+f.Name(),
    "rollout", "restart",
    "--namespace="+namespace,
    "deployment/"+deploymentName,
).Run()

// rollout status with timeout
exec.Command("kubectl", "--kubeconfig="+f.Name(),
    "rollout", "status",
    "--namespace="+namespace,
    "deployment/"+deploymentName,
    "--timeout="+rolloutTimeout.String(),
).Run()
```

**Writing the kubeconfig once per `kinder dev` invocation (not per cycle)** avoids creating a new temp file every build-load-rollout cycle.

### Injection point for tests

The kubectl calls should be abstracted behind a package-level function var for testability, following the lifecycle pattern:

```go
// pkg/internal/dev/dev.go
var rolloutFn = defaultRollout // tests swap this

func defaultRollout(kubeconfigPath, namespace, deployment string, timeout time.Duration) error {
    // exec kubectl rollout restart + status
}
```

---

## 4. Docker Build Invocation (DEV-02)

### Current codebase Docker build usage [VERIFIED: buildcontext.go]

The existing `pkg/build/nodeimage/buildcontext.go` uses `docker run` + `docker commit` (not `docker build`) to build node images. This is a kinder-internal pattern for kindest/node construction — **not reusable** for DEV-02 which needs a standard user-project Dockerfile.

### Recommendation for kinder dev: shell out to `docker build`

`kinder dev` should shell out to the container runtime binary for building user images. This is correct because:
1. The user's Dockerfile may require BuildKit features, build args, cache mounts, etc.
2. The image build context is arbitrary user code — kinder has no need to understand it.
3. The established project pattern for shelling out is `exec.Command(binaryName, args...).Run()`.

**Implementation sketch:**

```go
// Source: pkg/exec (established codebase pattern)
// binaryName from lifecycle.ProviderBinaryName() or providerBinaryName(provider)
buildErr := exec.Command(binaryName, "build",
    "-t", imageTag,    // e.g. "kinder-dev/myapp:latest" or --image flag value
    watchDir,           // build context = watched directory
).Run()
```

**Why not use the `build/nodeimage` package:** That package builds kindest/node images from Kubernetes source. It has nothing to do with user application images.

**Note:** The `--image` flag on `kinder dev` lets users specify the image name that both the Deployment and `docker build -t` use. If omitted, derive a deterministic name from the deployment name: `kinder-dev/<deployment>:latest`.

**Provider binary:** Use `lifecycle.ProviderBinaryName()` (already exported, used by `status.go` and `get/clusters`) rather than re-detecting. This ensures docker/podman/nerdctl consistency.

---

## 5. File-Watching Approach

### fsnotify vs stdlib polling — resolved

| | fsnotify v1.10.1 | stdlib polling |
|---|---|---|
| Mechanism | OS native (inotify/kqueue/ReadDirectoryChangesW) | `os.ReadDir` + `os.Stat.ModTime` in a ticker loop |
| New dep added | `github.com/fsnotify/fsnotify` (transitive: `golang.org/x/sys` already in go.mod) | None |
| Docker Desktop macOS volume mounts | **Unreliable** — kqueue events may not fire for bind-mount writes on macOS VirtioFS | Works — polling reads actual filesystem state |
| Recursive watch | Not built-in; requires manual `w.Add(subdir)` for each subdirectory discovered via `os.ReadDir` | Naturally recursive via `filepath.WalkDir` |
| Event coalescing | Multiple writes in rapid succession produce multiple events → must debounce | Polling interval itself is a natural debounce window |
| False negatives | Zero (OS delivers all events, subject to buffer overflow — see ErrEventOverflow) | Zero (polling sees final state at each interval) |
| Implementation cost | Low — stdlib-like API | Low — 20 lines |

**Decision: add fsnotify for the default (--watch without --poll) path. Implement stdlib polling for --poll path.**

Rationale:
- fsnotify v1.10.1's only new transitive dep is `golang.org/x/sys@v0.13.0` which is already required at v0.41.0 — the go.mod sum will pick the higher version, adding zero new entries to the module graph conceptually (the min-version selection algorithm already requires v0.41.0).
- Polling is needed anyway for DEV-05 (--poll flag) and is trivially implementable in stdlib.
- The fsnotify debounce pattern (per-file timer map) is well-documented (verified via Context7).

**Note from docs (VERIFIED via Context7):** fsnotify `Watcher.Add` is **non-recursive**. For a directory tree, the code must walk subdirectories and call `w.Add(subdir)` for each. New subdirectories created after watch starts also need `w.Add`. For the first implementation, use `filepath.WalkDir` at startup to discover and register all subdirectories. Handle `Create` events for new directories by adding them to the watcher.

### fsnotify API (VERIFIED via Context7, library ID: /fsnotify/fsnotify)

```go
// Current version: v1.10.1 (published 2026-05-04, verified via Go proxy)
// go get github.com/fsnotify/fsnotify@v1.10.1

import "github.com/fsnotify/fsnotify"

w, err := fsnotify.NewWatcher()
// ...
defer w.Close()

// Register all subdirectories recursively at startup:
filepath.WalkDir(watchDir, func(path string, d fs.DirEntry, err error) error {
    if d.IsDir() {
        return w.Add(path)
    }
    return nil
})

go func() {
    for {
        select {
        case e, ok := <-w.Events:
            if !ok { return }
            if e.Has(fsnotify.Write) || e.Has(fsnotify.Create) {
                // trigger debounce
                if e.Has(fsnotify.Create) {
                    // if it's a directory, add it to the watcher
                    if info, statErr := os.Stat(e.Name); statErr == nil && info.IsDir() {
                        _ = w.Add(e.Name)
                    }
                }
            }
        case err, ok := <-w.Errors:
            if !ok { return }
            logger.Warnf("watcher error: %v", err)
        case <-ctx.Done():
            return
        }
    }
}()
```

### Stdlib polling path (--poll)

```go
// No new imports beyond os, path/filepath, time
func pollLoop(ctx context.Context, dir string, interval time.Duration, onChange func()) {
    type fileState struct{ size int64; mod time.Time }
    prev := map[string]fileState{}

    tick := time.NewTicker(interval)
    defer tick.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-tick.C:
            curr := map[string]fileState{}
            filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
                if err != nil || d.IsDir() { return nil }
                info, e := d.Info()
                if e != nil { return nil }
                curr[path] = fileState{size: info.Size(), mod: info.ModTime()}
                return nil
            })
            changed := false
            for p, s := range curr {
                if p2, ok := prev[p]; !ok || p2.mod != s.mod || p2.size != s.size {
                    changed = true
                    break
                }
            }
            if changed {
                onChange()
            }
            prev = curr
        }
    }
}
```

**Important:** When `--poll` is used, the polling interval itself acts as the debounce floor. The debounce timer still applies on top of the onChange signal for consistency.

### Docker Desktop macOS context (DEV-05 motivation) [CITED: docker/for-mac issues #2216, #2417; CITED: fsnotify README.md FAQ]

Docker Desktop on macOS uses VirtioFS (or previously osxfs) to share host directories into containers. When the user edits files on the host FS, the kqueue events delivered to a watcher process **running on the host** should work normally — the issue is the opposite direction (events inside the container for files on a mounted volume). However, kinder dev runs on the host and watches the host FS directly. The `--poll` flag is a safety net for users who report events not firing (e.g., editor writes to a temp file and renames atomically, which kqueue sometimes misses, or Docker Desktop SDK-level write hooks that bypass normal FS notifications). The `--poll` flag is a user escape hatch, not a bug fix.

---

## 6. Debouncer Pattern (DEV-04)

The debouncer coalesces rapid file events into a single cycle trigger. The established pattern from fsnotify docs uses a per-file timer map, but for `kinder dev` we want a simpler **global single-timer debounce** (any change anywhere triggers the same cycle):

```go
// Source: adapted from Context7 /fsnotify/fsnotify debounce example
// Single-timer global debounce — any event resets the window

type debouncer struct {
    mu      sync.Mutex
    timer   *time.Timer
    window  time.Duration
    trigger chan struct{} // closed/sent when window expires; receiver runs cycle
}

func newDebouncer(window time.Duration) *debouncer {
    ch := make(chan struct{}, 1)
    d := &debouncer{window: window, trigger: ch}
    // Pre-allocate a stopped timer so Reset works correctly.
    d.timer = time.AfterFunc(time.Hour*24*365, func() {
        select {
        case ch <- struct{}{}:
        default: // already pending
        }
    })
    d.timer.Stop()
    return d
}

func (d *debouncer) fire() {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.timer.Reset(d.window)
}

// Main watch loop:
// go watchFiles() → calls d.fire() on every event
// for { <-d.trigger; runCycle() }
```

**Alternative simpler approach (no sync.Mutex):**

```go
// Channel-based debounce — the canonical Go idiom
func debounce(in <-chan struct{}, window time.Duration) <-chan struct{} {
    out := make(chan struct{}, 1)
    go func() {
        timer := time.NewTimer(window)
        timer.Stop()
        pending := false
        for {
            select {
            case _, ok := <-in:
                if !ok { close(out); return }
                if !pending {
                    timer.Reset(window)
                    pending = true
                }
            case <-timer.C:
                pending = false
                select {
                case out <- struct{}{}:
                default:
                }
            }
        }
    }()
    return out
}
```

**Recommendation:** Use the channel-based debounce (second variant) as it is pure Go stdlib, requires no mutex, and is idiomatic. The `in` channel is fed by both the fsnotify goroutine and the polling goroutine through a unified `events chan struct{}` channel.

---

## 7. Timing/UX (DEV-04, banner, per-cycle timing)

### Established timing display patterns [VERIFIED: pause.go:118, resume.go:127, restore.go:101]

The codebase uses `%.1fs` float64 seconds for all durations:

```go
fmt.Fprintf(streams.Out, "Cluster paused. Total time: %.1fs\n", result.Duration)
fmt.Fprintf(streams.Out, "Cluster resumed. Total time: %.1fs\n", result.Duration)
fmt.Fprintf(streams.Out, "Restored snapshot %s in %.1fs\n", res.SnapName, res.DurationS)
```

### Recommended per-cycle output for kinder dev

```
[watch] Watching /path/to/dir for changes (debounce: 500ms)...
[cycle 1] Change detected — starting build-load-rollout
  build:   2.3s
  load:    1.1s
  rollout: 4.7s
  total:   8.1s
[watch] Ready. Watching for changes...
[cycle 2] Change detected — starting build-load-rollout
  build:   1.9s (error: docker build failed; see output above)
```

This follows `logger.V(0).Infof(...)` for watch status (to stderr via the logger) and `fmt.Fprintf(streams.Out, ...)` for cycle summary lines (to stdout, structured). The `[watch]` prefix bracket style is consistent with how the project's status lines use prefix indicators (` • `, ` ✓ `, ` ✗ `). Since the watch loop is long-running, cycle banners should be printed to `streams.Out` so they can be piped.

### Watch-mode banner

Print once at start:
```
Watching /path/to/dir → deployment/<name> (cluster: kind, namespace: default)
Debounce: 500ms | Mode: fsnotify
Press Ctrl+C to exit.
```

---

## 8. Tests — CLI Command Testing Pattern

### The established pattern [VERIFIED: pause_test.go, snapshot/create_test.go]

Every CLI command in this codebase uses **package-level function-var injection** for testability:

1. Business logic lives in `pkg/internal/<domain>/` (e.g., `lifecycle`, `snapshot`).
2. The `pkg/cmd/kind/<cmd>/cmd.go` file has a `var <domain>Fn = <internal>.<Func>`.
3. Tests define a `with<Domain>Fn(t, fake)` helper that swaps the var via `t.Cleanup`.
4. Tests create `cmd.IOStreams` from `bytes.Buffer` and call `c.Execute()`.

### Template for `kinder dev` tests

```go
// pkg/cmd/kind/dev/dev_test.go

// devFn is the package-level injection point (in dev.go)
var devFn = dev.Run // dev.Run in pkg/internal/dev

func withDevFn(t *testing.T, fn func(opts dev.Options) error) {
    t.Helper()
    prev := devFn
    devFn = fn
    t.Cleanup(func() { devFn = prev })
}

func newTestStreams() (cmd.IOStreams, *bytes.Buffer, *bytes.Buffer) {
    out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
    return cmd.IOStreams{In: nil, Out: out, ErrOut: errOut}, out, errOut
}

// Test: --watch and --target are required; missing either → non-zero exit
func TestDevCmd_MissingWatchFlag(t *testing.T) { ... }

// Test: flags propagated correctly to devFn
func TestDevCmd_FlagsPropagated(t *testing.T) { ... }

// Test: devFn error → non-zero exit
func TestDevCmd_PropagatesError(t *testing.T) { ... }

// Test: debounce flag default = 500ms
func TestDevCmd_DebounceDefault(t *testing.T) { ... }
```

**For the internal package (`pkg/internal/dev/`)**, tests must cover:
- `buildImage(opts)` — inject a fake that captures args
- `loadImages(opts)` — inject `loadImagesFn` var
- `rolloutRestart(opts)` — inject `rolloutFn` var
- Debouncer: use a buffered input channel, send N events rapidly, verify exactly 1 output after window

### Test command: `go test ./pkg/cmd/kind/dev/... ./pkg/internal/dev/...`

---

## 9. Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Image tar staging | Custom temp file management | `fs.TempDir` + `defer os.RemoveAll` | Established pattern from `images.go` and `docker-image.go` |
| Image existence check on nodes | Custom inspect loop | `nodeutils.ImageTags` | Already handles ctr/containerd/docker ID lookup |
| Node import with Docker Desktop fallback | Custom ctr import | `nodeutils.LoadImageArchiveWithFallback` | Handles both normal ctr import and --all-platforms fallback |
| Provider detection | Parse `DOCKER_HOST`, `CONTAINER_RUNTIME_ENDPOINT` | `lifecycle.ProviderBinaryName()` | Single source of truth, already tested |
| Concurrent per-node loading | Manual goroutine fan-out | `errors.UntilErrorConcurrent` | Handles error aggregation correctly |
| Cluster name auto-detection | Custom list-then-match | `lifecycle.ResolveClusterName` | Handles 0/1/many clusters with correct error messages |
| Kubeconfig retrieval | Parse `~/.kube/config` manually | `provider.KubeConfig(name, false)` | Returns correct external endpoint for host-side kubectl |

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.8.0 | CLI command | Already in go.mod; all kinder commands use it |
| `github.com/fsnotify/fsnotify` | v1.10.1 | File-system events (default watcher) | Only new dep; only adds x/sys (already present) |
| `sigs.k8s.io/kind/pkg/exec` | internal | Shell out to docker, kubectl | Established project pattern |
| `sigs.k8s.io/kind/pkg/cluster` | internal | Provider, node listing, kubeconfig | Owner of all runtime operations |
| `sigs.k8s.io/kind/pkg/cluster/nodeutils` | internal | Image load, tag checking | Has LoadImageArchiveWithFallback |
| `sigs.k8s.io/kind/pkg/internal/lifecycle` | internal | ProviderBinaryName, ResolveClusterName | Reusable lifecycle helpers |
| `sigs.k8s.io/kind/pkg/fs` | internal | TempDir | Already used by load commands |
| `sigs.k8s.io/kind/pkg/errors` | internal | UntilErrorConcurrent, Wrap | Project error package |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/internal/cli` | internal | `OverrideDefaultName`, `StatusForLogger` | Must call in RunE; optional for spinner |
| `sigs.k8s.io/kind/pkg/internal/runtime` | internal | `GetDefault(logger)` | Pass to NewProvider in RunE |
| `sigs.k8s.io/kind/pkg/log` | internal | `log.Logger` | All commands take logger as arg |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| fsnotify | Pure stdlib polling everywhere | Eliminates new dep; events are less responsive (polling interval latency); Docker Desktop issue becomes non-issue |
| fsnotify | `golang.org/x/fsnotify` (older path) | Same library, just old import path — use github.com/fsnotify/fsnotify |
| `provider.KubeConfig` → temp file | `KUBECONFIG` env var from `kinder env` | `kinder dev` would need the user to export KUBECONFIG first; temp file is self-contained |

**go.mod changes required:**
```bash
cd /Users/patrykattc/work/git/kinder
go get github.com/fsnotify/fsnotify@v1.10.1
go mod tidy
```

---

## Architecture Patterns

### Recommended Project Structure
```
pkg/
├── cmd/kind/
│   ├── root.go              # MODIFY: add dev.NewCommand(logger, streams)
│   └── dev/
│       ├── dev.go           # NewCommand, flagpole, runE (thin CLI shell)
│       └── dev_test.go      # unit tests via devFn injection
└── internal/
    └── dev/
        ├── dev.go           # DevOptions, Run(), build/load/rollout/watch loop
        └── dev_test.go      # unit tests for build/load/rollout functions
```

### System Architecture Diagram

```
User: kinder dev --watch ./src --target myapp --name kind
         │
         ▼
[pkg/cmd/kind/dev/dev.go] NewCommand / runE
         │  flag parse + cli.OverrideDefaultName
         │  cluster.NewProvider + provider.KubeConfig → kubeconfig tempfile
         │  signal.NotifyContext(SIGINT, SIGTERM) → ctx
         │
         ├──▶ [watcher goroutine] fsnotify.NewWatcher
         │      │  w.Add(watchDir + all subdirs)
         │      │  every Write/Create event → events chan<- struct{}{}
         │      └──────────────────────────────────────────────────┐
         │                                                          │
         ├──▶ [debouncer goroutine] channel debounce               │
         │      │  receives from events chan                        │
         │      │  waits debounce window (default 500ms)           │
         │      │  emits to cycles chan<- struct{}{}  ◀────────────┘
         │
         ▼
[pkg/internal/dev/dev.go] Run (main watch loop)
         │
         for range cycles chan:
         │
         ├─1─▶ [docker build]
         │      exec.Command(binaryName, "build", "-t", imageTag, watchDir)
         │      t1 := time.Since(cycleStart)
         │
         ├─2─▶ [load images] dev.LoadImages(LoadImagesOptions)
         │      → cluster.NewProvider.ListInternalNodes
         │      → nodeutils.ImageTags (smart-skip)
         │      → exec save to TempDir
         │      → nodeutils.LoadImageArchiveWithFallback (concurrent per node)
         │      t2 := time.Since(cycleStart) - t1
         │
         ├─3─▶ [kubectl rollout restart]
         │      exec.Command("kubectl", "--kubeconfig=...", "rollout", "restart", ...)
         │      exec.Command("kubectl", "--kubeconfig=...", "rollout", "status", ...)
         │      t3 := time.Since(cycleStart) - t1 - t2
         │
         └─4─▶ [print cycle summary]
                fmt.Fprintf(streams.Out, "  build: %.1fs\n  load: %.1fs\n  rollout: %.1fs\n  total: %.1fs\n", ...)
```

### Pattern: Test Injection for Long-Running Commands

`kinder dev` is the first watch-mode (long-running) command. The established injection pattern still applies, but with a twist: the watch loop itself must be injectable for tests. The recommended approach is to inject the "events source" — tests send events directly to the channel without starting a real file watcher.

```go
// pkg/internal/dev/dev.go
type Options struct {
    WatchDir        string
    Target          string          // deployment name
    Namespace       string
    ClusterName     string
    ImageTag        string
    Debounce        time.Duration
    Poll            bool
    PollInterval    time.Duration
    RolloutTimeout  time.Duration
    Logger          log.Logger
    Streams         cmd.IOStreams
    Provider        *cluster.Provider
    KubeconfigPath  string          // temp file path, set by cmd layer
    // Test hooks
    EventSource     <-chan struct{}  // nil = start real watcher; non-nil = use this (tests)
}
```

### Anti-Patterns to Avoid

- **Running the watch loop without SIGINT handling:** Ctrl+C on a long-running command must clean up the fsnotify watcher and any in-progress build. Use `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`.
- **Creating a new kubeconfig temp file every cycle:** Write it once at startup, delete on exit.
- **Calling `runE` in `load/images/images.go` directly:** It is unexported and has no injection points. Replicate the load logic in `pkg/internal/dev/` against the public nodeutils APIs instead.
- **Using `node.CommandContext` for kubectl rollout:** That pattern runs kubectl inside the node container using the in-cluster admin.conf. For user Deployments, use host kubectl with provider.KubeConfig.
- **Blocking the watcher goroutine during a cycle:** The cycle should run on the main goroutine (or a separate "cycle runner" goroutine); the watcher goroutine only writes to the events channel and must never block on the cycle completing.
- **Image tag collisions:** Always use a fixed tag (`:latest` or user-specified). Never increment `:v1`, `:v2` because the Deployment spec would need patching.

---

## Common Pitfalls

### Pitfall 1: Build Context Too Large
**What goes wrong:** `docker build` uploads the entire watched directory as build context. If `--watch` points to a large source tree with `node_modules/`, `.git/`, or build artifacts, every cycle uploads gigabytes.
**Why it happens:** `docker build <dir>` uses `dir` as the full build context.
**How to avoid:** Respect `.dockerignore` in the watched directory (docker does this automatically). Document this requirement. Optionally validate that a `.dockerignore` exists and warn if not.
**Warning signs:** Build step takes >30s on first cycle for a trivially small code change.

### Pitfall 2: Image Not Updated in Deployment (imagePullPolicy)
**What goes wrong:** After `kinder load images`, `kubectl rollout restart` completes but the Deployment still uses the old image (0 new pods with the new image).
**Why it happens:** `imagePullPolicy: Always` pulls from a registry (which doesn't have the new image). `imagePullPolicy: IfNotPresent` uses the already-cached old image. Only `imagePullPolicy: Never` forces containerd to use the locally loaded image.
**How to avoid:** Document that the Deployment must use `imagePullPolicy: Never` for local images loaded via `kinder load images`. Optionally warn if the Deployment's policy is not `Never`.
**Warning signs:** Rollout completes but `kubectl get pods -o wide` shows old image digest.

### Pitfall 3: Watch Loop Blocking on Cycle
**What goes wrong:** A slow build (>30s) causes fsnotify event queue to back up. When the debounce window fires again during the cycle, a second cycle starts concurrently, leading to a race on the kubeconfig temp file or the image tag.
**Why it happens:** The watcher goroutine fires events regardless of cycle state.
**How to avoid:** Run only one cycle at a time. Use a `select { case cycles <- struct{}{}: default: }` non-blocking send to the cycles channel — if a cycle is already pending or running, drop the event. Alternatively, gate on a `cycleInProgress bool` protected by a mutex.
**Warning signs:** Two overlapping "Change detected" log lines.

### Pitfall 4: fsnotify ErrEventOverflow on High-Churn Directories
**What goes wrong:** Watching a directory where a build tool writes many files rapidly (e.g., `go build ./...` output dir) produces `fsnotify.ErrEventOverflow`. The watcher drops events, so some changes may not trigger a cycle.
**Why it happens:** The kernel's inotify event queue fills up when events arrive faster than the process drains them.
**How to avoid:** Watch the source directory, not the build output directory. Handle `ErrEventOverflow` by logging a warning and immediately triggering a cycle (since overflow implies high activity).
**Warning signs:** `watcher error: fsnotify.ErrEventOverflow` in stderr.

### Pitfall 5: Deployment Not Found at Rollout
**What goes wrong:** `kubectl rollout restart deployment/<name>` exits non-zero with "not found". The cycle fails on the rollout step even though the image loaded successfully.
**Why it happens:** `--target` specifies the deployment name but not the namespace. If the Deployment is not in `default`, the rollout fails.
**How to avoid:** Expose `--namespace` (default: `default`) and always pass `--namespace` to kubectl. On rollout failure, print the kubectl error output so users can diagnose.
**Warning signs:** rollout step fails immediately with "not found" on first cycle.

### Pitfall 6: SIGINT Leaks Background Goroutines
**What goes wrong:** User presses Ctrl+C. The fsnotify watcher goroutine and debouncer goroutine continue running, keeping the process alive. The kubeconfig temp file is not deleted.
**Why it happens:** No context cancellation wired into the goroutines.
**How to avoid:** Use `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`. Pass `ctx` to all goroutines. On `ctx.Done()`, close the events channel and exit cleanly. Use `defer os.Remove(kubeconfigPath)` registered before the loop starts.
**Warning signs:** `kinder dev` process does not exit after Ctrl+C; zombie child processes.

### Pitfall 7: Atomic Saves Produce No Events (Editor Temp Files)
**What goes wrong:** Some editors (vim, emacs, some IDEs) write to a temp file and rename it to the target path. Rename events produce a `Create` on the destination but not a `Write`. If the watcher only handles `Write`, these saves are silently missed.
**Why it happens:** The OS emits `Create` for the destination of a rename, not `Write`.
**How to avoid:** Always handle both `fsnotify.Write` and `fsnotify.Create` events (verified in Context7 docs). For new files that are directories, re-add to watcher.
**Warning signs:** Edits in vim never trigger a cycle; edits in a basic `echo > file` always do.

---

## Code Examples

### Example 1: Debounced Watch Loop (verified pattern from fsnotify Context7 docs)

```go
// Source: Context7 /fsnotify/fsnotify — adapted for single-signal global debounce
// pkg/internal/dev/dev.go

func runWatchLoop(ctx context.Context, watchDir string, debounce time.Duration, logger log.Logger) (<-chan struct{}, error) {
    w, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }

    // Register root + all subdirectories
    if walkErr := filepath.WalkDir(watchDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return nil } // skip unreadable entries
        if d.IsDir() { return w.Add(path) }
        return nil
    }); walkErr != nil {
        w.Close()
        return nil, walkErr
    }

    raw := make(chan struct{}, 64)
    go func() {
        defer w.Close()
        defer close(raw)
        for {
            select {
            case e, ok := <-w.Events:
                if !ok { return }
                if e.Has(fsnotify.Write) || e.Has(fsnotify.Create) {
                    if e.Has(fsnotify.Create) {
                        if info, sErr := os.Stat(e.Name); sErr == nil && info.IsDir() {
                            _ = w.Add(e.Name) // watch new subdirs
                        }
                    }
                    select { case raw <- struct{}{}: default: }
                }
            case err, ok := <-w.Errors:
                if !ok { return }
                if errors.Is(err, fsnotify.ErrEventOverflow) {
                    logger.Warnf("fsnotify: event overflow — triggering cycle")
                    select { case raw <- struct{}{}: default: }
                } else {
                    logger.Warnf("fsnotify: %v", err)
                }
            case <-ctx.Done():
                return
            }
        }
    }()

    out := make(chan struct{}, 1)
    go func() {
        defer close(out)
        t := time.NewTimer(time.Hour * 24 * 365)
        t.Stop()
        pending := false
        for {
            select {
            case _, ok := <-raw:
                if !ok { return }
                if !pending {
                    t.Reset(debounce)
                    pending = true
                }
            case <-t.C:
                pending = false
                select { case out <- struct{}{}: default: }
            case <-ctx.Done():
                return
            }
        }
    }()

    return out, nil
}
```

### Example 2: Rollout Restart + Status (host kubectl with kubeconfig file)

```go
// Source: adapted from corednstuning.go pattern; host kubectl variant
// pkg/internal/dev/dev.go

func rolloutRestartAndWait(kubeconfigPath, namespace, deployment string, timeout time.Duration) error {
    if err := exec.Command("kubectl",
        "--kubeconfig="+kubeconfigPath,
        "rollout", "restart",
        "--namespace="+namespace,
        "deployment/"+deployment,
    ).Run(); err != nil {
        return fmt.Errorf("rollout restart: %w", err)
    }
    if err := exec.Command("kubectl",
        "--kubeconfig="+kubeconfigPath,
        "rollout", "status",
        "--namespace="+namespace,
        "deployment/"+deployment,
        "--timeout="+timeout.String(),
    ).Run(); err != nil {
        return fmt.Errorf("rollout status: %w", err)
    }
    return nil
}
```

### Example 3: Kubeconfig Tempfile (once at startup)

```go
// pkg/internal/dev/dev.go

func writeKubeconfigTemp(provider *cluster.Provider, clusterName string) (path string, cleanup func(), err error) {
    cfg, err := provider.KubeConfig(clusterName, false)
    if err != nil {
        return "", nil, fmt.Errorf("get kubeconfig: %w", err)
    }
    f, err := os.CreateTemp("", "kinder-dev-*.kubeconfig")
    if err != nil {
        return "", nil, fmt.Errorf("create kubeconfig temp: %w", err)
    }
    if _, err := f.WriteString(cfg); err != nil {
        f.Close()
        os.Remove(f.Name())
        return "", nil, err
    }
    f.Close()
    return f.Name(), func() { os.Remove(f.Name()) }, nil
}
```

### Example 4: Per-Cycle Timing Output

```go
// pkg/internal/dev/dev.go — cycle runner

func runOneCycle(opts cycleOpts) error {
    cycleStart := time.Now()
    opts.logger.V(0).Infof("[cycle %d] Change detected — starting build-load-rollout", opts.cycleNum)

    t0 := time.Now()
    if err := buildImageFn(opts.binaryName, opts.imageTag, opts.watchDir); err != nil {
        return fmt.Errorf("build: %w", err)
    }
    buildDur := time.Since(t0)

    t1 := time.Now()
    if err := loadImagesFn(opts.loadOpts); err != nil {
        return fmt.Errorf("load: %w", err)
    }
    loadDur := time.Since(t1)

    t2 := time.Now()
    if err := rolloutFn(opts.kubeconfigPath, opts.namespace, opts.target, opts.rolloutTimeout); err != nil {
        return fmt.Errorf("rollout: %w", err)
    }
    rolloutDur := time.Since(t2)

    fmt.Fprintf(opts.streams.Out,
        "  build:   %.1fs\n  load:    %.1fs\n  rollout: %.1fs\n  total:   %.1fs\n",
        buildDur.Seconds(), loadDur.Seconds(), rolloutDur.Seconds(), time.Since(cycleStart).Seconds(),
    )
    return nil
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `docker build` (Docker Desktop <27 containerd) | `docker build` + containerd image store fallback for load | Docker Desktop 27+ (2024) | LoadImageArchiveWithFallback already handles this |
| fsnotify `Watcher.Add` non-recursive (always) | Still non-recursive in v1.10.1 | Never changed | Must manually walk and add subdirs |
| DurationVar with bare integers | DurationVar requiring unit suffix (e.g. `500ms`) | Phase 47 decision (STATE.md 47-06) | All kinder duration flags require suffix — follow same convention |

**Deprecated/outdated:**
- `load docker-image` command: hardcodes `docker save`; superseded by `load images` (Phase 46) which detects provider binary. `kinder dev` should use the same provider-aware approach.
- `node.CommandContext("kubectl", ...)` for user Deployment rollouts: correct for system addons during create; wrong for `kinder dev` host-side use.

---

## Open Questions (RESOLVED)

1. **Image tag convention — should `kinder dev` patch the Deployment's image field?**
   - What we know: `kinder dev` builds with a fixed image tag. The Deployment must already reference that tag.
   - What's unclear: Should `kinder dev` accept `--image` and also patch `deployment/<name>` to use `imagePullPolicy: Never` and the given tag on first run?
   - **RESOLVED:** Out-of-scope for Phase 49. Document the precondition (Deployment must already reference the image tag with `imagePullPolicy: Never`) in `--help` and emit a runtime warning if the rollout completes but the image digest is unchanged. Plan 49-04 owns the help text.

2. **Polling interval default and flag name for DEV-05**
   - What we know: The requirement says "configurable interval". No default specified.
   - **RESOLVED:** `--poll-interval=1s` (cobra `DurationVar`, unit required per STATE.md 47-06 convention). Wired in Plan 49-04.

3. **Rollout timeout default**
   - What we know: corednstuning uses `--timeout=60s`. Resume uses 5m WaitTimeout.
   - **RESOLVED:** `--rollout-timeout=2m` (cobra `DurationVar`). User Deployments may have longer startup than addons. Wired in Plan 49-04.

4. **Namespace: should --target accept `namespace/deployment` syntax or separate --namespace flag?**
   - What we know: kubectl flags use `--namespace` separately.
   - **RESOLVED:** Separate `--namespace` flag (default: `default`) to follow kubectl conventions. Wired in Plan 49-04.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| docker | DEV-02 (build + save) | ✓ | 29.4.1 | None — required |
| kubectl | DEV-03 (rollout restart + status) | ✓ | v1.32.4 | None — required (kinder doctor already checks for it) |
| go (build) | go.mod update | ✓ | 1.26.2 darwin/arm64 | — |
| fsnotify v1.10.1 (new module dep) | DEV-01 (default watcher) | ✗ (not in go.mod yet) | — | Stdlib polling (DEV-05 path) |

**Missing dependencies with no fallback:**
- `docker` and `kubectl` — both required; already checked by `kinder doctor`; Phase 49 should fail fast with a clear error if either is missing.

**Missing dependencies with fallback:**
- `fsnotify` — not yet in go.mod; stdlib polling is the fallback if the `--poll` flag is always used. For the first cycle (before the user knows to pass `--poll`), the default watcher must work, so `go get github.com/fsnotify/fsnotify@v1.10.1` is a Wave 0 task.

---

## Validation Architecture

> nyquist_validation key absent from config.json (no config.json found) — treat as enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` |
| Config file | none (go test discovers files) |
| Quick run command | `go test ./pkg/cmd/kind/dev/... ./pkg/internal/dev/... -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DEV-01 | `--watch` and `--target` flags required; missing either → non-zero | unit | `go test ./pkg/cmd/kind/dev/... -run TestDevCmd_MissingFlags` | ❌ Wave 0 |
| DEV-01 | Entering watch mode prints banner with dir + target + debounce window | unit | `go test ./pkg/internal/dev/... -run TestRun_Banner` | ❌ Wave 0 |
| DEV-02 | Build step calls docker build with correct image tag and watch dir | unit | `go test ./pkg/internal/dev/... -run TestRunOneCycle_BuildArgs` | ❌ Wave 0 |
| DEV-02 | Load step invokes LoadImages with correct provider + image | unit | `go test ./pkg/internal/dev/... -run TestRunOneCycle_LoadCalled` | ❌ Wave 0 |
| DEV-03 | Rollout step calls `kubectl rollout restart` then `rollout status` | unit | `go test ./pkg/internal/dev/... -run TestRolloutRestartAndWait` | ❌ Wave 0 |
| DEV-03 | Rollout failure returns non-nil error and stops cycle | unit | `go test ./pkg/internal/dev/... -run TestRolloutRestartAndWait_Error` | ❌ Wave 0 |
| DEV-04 | N rapid events in < debounce window → exactly 1 cycle | unit | `go test ./pkg/internal/dev/... -run TestDebounce_CoalescesBurst` | ❌ Wave 0 |
| DEV-04 | Per-cycle timing lines printed to streams.Out | unit | `go test ./pkg/internal/dev/... -run TestRunOneCycle_TimingOutput` | ❌ Wave 0 |
| DEV-05 | `--poll` flag accepted; pollLoop called instead of fsnotify | unit | `go test ./pkg/cmd/kind/dev/... -run TestDevCmd_PollFlag` | ❌ Wave 0 |
| DEV-05 | Poll loop detects a changed file (mtime change) and fires onChange | unit | `go test ./pkg/internal/dev/... -run TestPollLoop_DetectsChange` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./pkg/cmd/kind/dev/... ./pkg/internal/dev/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `pkg/cmd/kind/dev/dev.go` — command file
- [ ] `pkg/cmd/kind/dev/dev_test.go` — command unit tests
- [ ] `pkg/internal/dev/dev.go` — business logic
- [ ] `pkg/internal/dev/dev_test.go` — business logic unit tests
- [ ] `go.mod` / `go.sum` — add `github.com/fsnotify/fsnotify v1.10.1`

---

## Security Domain

> security_enforcement key absent from config.json — treat as enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | yes (indirect) | kubeconfig written to temp file with 0600 permissions |
| V5 Input Validation | yes | validate `--target` is a valid K8s name (no path traversal); validate `--watch` is an existing directory |
| V6 Cryptography | no | — |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Kubeconfig written to world-readable temp file | Information Disclosure | `os.CreateTemp` + `os.Chmod(path, 0600)` before writing |
| `--watch` pointing to `/` or FS root | Tampering / DoS (inotify exhaustion) | Validate path depth / sanity; warn if dir contains >10k files |
| `--target` with shell-injection via deployment name | Tampering | Pass as separate argument (not interpolated into shell string); `exec.Command` is safe (no shell involved) |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | User's Deployment already has `imagePullPolicy: Never` and references the image tag built by `kinder dev` | DEV-02, Docker Build | Rollout will not pick up new image; user gets confusing "rollout completed" with old image |
| A2 | fsnotify v1.10.1 has no polling backend (confirmed by reading CHANGELOG and README FAQ) | File-Watching Approach | If wrong (misread docs), the polling alternative is moot — but stdlib polling is still simpler |
| A3 | Docker Desktop macOS kqueue issue affects watch from inside container mounts, not host-side watches | File-Watching Approach | If wrong, the default watcher is broken on macOS Docker Desktop; `--poll` becomes required for all macOS Docker Desktop users |
| A4 | `provider.KubeConfig(name, false)` returns a kubeconfig with an endpoint reachable from the host | kubectl Rollout Pattern | If it returns the internal cluster IP, kubectl from the host cannot connect |

**Note on A4:** `false` = external (not internal). Looking at `provider.go:220` and `kubeconfig.go:53`, `external=true` = host-reachable endpoint. The parameter name is confusing: `KubeConfig(name, internal bool)` where `false` = NOT internal = external. Verified correct. [VERIFIED: provider.go:220: `return kubeconfig.Get(p.provider, defaultName(name), !internal)` — so `KubeConfig(name, false)` → `Get(..., !false)` → `Get(..., true)` = external]

---

## Sources

### Primary (HIGH confidence)
- Codebase read: `pkg/cmd/kind/load/images/images.go` — load pipeline implementation
- Codebase read: `pkg/cmd/kind/root.go` — command registration pattern
- Codebase read: `pkg/cmd/kind/pause/pause.go`, `pause_test.go` — command+test pattern
- Codebase read: `pkg/cmd/kind/snapshot/create.go`, `create_test.go` — injection pattern
- Codebase read: `pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go` — rollout restart pattern
- Codebase read: `pkg/internal/lifecycle/resume.go` — WaitForNodesReady, ResolveClusterName
- Codebase read: `pkg/exec/local.go`, `pkg/exec/default.go` — exec.Command pattern
- Codebase read: `pkg/cluster/provider.go:220` — KubeConfig(name, internal bool)
- Context7 `/fsnotify/fsnotify` — NewWatcher, Add, debounce, ErrEventOverflow [HIGH]
- Go proxy: `github.com/fsnotify/fsnotify@v1.10.1` published 2026-05-04 [VERIFIED]
- Go proxy: `github.com/fsnotify/fsnotify@v1.10.1.mod` — requires `golang.org/x/sys v0.13.0` [VERIFIED]

### Secondary (MEDIUM confidence)
- `.planning/STATE.md` — fsnotify gray-area decision (2026-05-03), DurationVar convention (47-06)
- `.planning/ROADMAP.md` — Phase 49 goal, success criteria
- `.planning/REQUIREMENTS.md` — DEV-01 through DEV-05

### Tertiary (LOW confidence)
- WebFetch github.com/fsnotify/fsnotify CHANGELOG — "no polling backend" claim [CITED: GitHub CHANGELOG]
- WebSearch docker/for-mac issues — Docker Desktop volume mount event unreliability [LOW — old issues, not 2024 specific]

---

## Metadata

**Confidence breakdown:**
- Load images pipeline API: HIGH — fully read and verified line-by-line
- CLI command structure: HIGH — verified pattern from 4 existing commands
- kubectl rollout pattern: HIGH — both in-node pattern (corednstuning) and host-kubectl approach verified
- Docker build invocation: HIGH — established exec.Command pattern; confirmed no existing docker-build wrapper to reuse
- fsnotify API: HIGH — verified via Context7 + Go proxy
- fsnotify polling absence: HIGH — verified CHANGELOG and README FAQ
- Docker Desktop macOS fsnotify issue: MEDIUM — documented in old issues; behavior in 2024-2026 not specifically verified
- Stdlib polling pattern: HIGH — pure stdlib, no external verification needed

**Research date:** 2026-05-06
**Valid until:** 2026-06-06 (stable — fsnotify, Go stdlib, cobra all stable; docker/kubectl CLIs stable)
