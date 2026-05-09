---
title: kinder dev
description: Reference for kinder dev — watch a directory, build a Docker image, load it into every node, roll the target Deployment.
---

`kinder dev` watches a directory and runs a debounced **build → load → rollout** cycle on every file change. It collapses the manual iteration loop (`docker build` → `kinder load images` → `kubectl rollout restart`) into a single watch process with per-cycle timing.

## Synopsis

```
kinder dev --watch <dir> --target <deployment> [flags]
```

Both `--watch` and `--target` are required. Cluster name auto-detects when exactly one cluster exists; otherwise pass `--name`.

## Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--watch` | | (required) | Directory to watch for file changes |
| `--target` | | (required) | Name of the Deployment to roll on every cycle |
| `--name` | `-n` | (auto-detect) | Cluster name. If exactly one cluster exists, auto-selected. Otherwise required |
| `--image` | | `<target>:dev` | Image tag to build and load. Defaults to the deployment name plus `:dev` |
| `--namespace` | | `default` | Namespace of the target Deployment |
| `--debounce` | | `500ms` | Coalesce rapid file changes within this window into a single cycle |
| `--poll` | | `false` | Use a stdlib polling watcher instead of fsnotify (for fsnotify-unfriendly environments like Docker Desktop on macOS) |
| `--poll-interval` | | `1s` | Polling interval when `--poll` is set |
| `--rollout-timeout` | | `2m` | Maximum time to wait for `kubectl rollout status` per cycle |
| `--json` | | `false` | Reserved (currently unused; per-cycle output is human-readable) |

## Cycle anatomy

Each cycle runs three steps in strict order:

1. **Build** (`docker build -t <image> <watch-dir>`) — the watch directory must contain a `Dockerfile`
2. **Load** (`LoadImagesIntoCluster`) — uses the same path as `kinder load images` to import the image into every node
3. **Rollout** (`kubectl rollout restart deployment/<target>` then `kubectl rollout status --timeout=<rollout-timeout>`) — runs on the host with `--kubeconfig=<temp file>`

If a step fails, the cycle exits with the error logged but the watcher keeps running. `kinder dev` does not auto-exit on cycle failure — you can fix the issue and save again.

### Per-step timing

```
[cycle 1] Change detected: index.html
build:    0.3s
load:     2.3s
rollout:  1.5s
total:    4.1s
```

Format is `%.1fs` per step. Total is wall time of the cycle including small overhead between steps.

## Watch vs poll

By default, `kinder dev` uses `fsnotify` for filesystem events. This is fast and efficient but **unreliable on Docker Desktop volume mounts on macOS** — saves inside a bind-mounted directory don't always emit fsnotify events.

If your watcher misses changes, switch to polling:

```sh
kinder dev --watch ./src --target myapp --poll --poll-interval 500ms
```

Polling walks the watch directory at the given interval and compares file size + mtime against a snapshot. It uses more CPU than fsnotify but works reliably across all platforms and bind-mount setups.

| Mode | When to use |
|---|---|
| `fsnotify` (default) | Linux native filesystems; most macOS native paths; Windows |
| `--poll` | Docker Desktop on macOS bind-mounts; corporate VPN-mounted drives; any environment where fsnotify events are dropped |

## Debouncing

The debouncer uses **leading-trigger** semantics: the first event in a debounce window arms the timer and fires immediately. Subsequent events within the window are absorbed.

This is the right model for save-driven dev loops because:

- IDE atomic-save bursts (5-50 events fired over &lt;100ms) coalesce into one cycle
- Build starts ASAP — you don't wait for the editor to finish swap-rename before the build kicks off
- The "trailing-trigger" model used by some watchers (reset timer on every event, fire after N ms of silence) is for fast-typing UIs, not file-save events

You can tune the window:

```sh
kinder dev --watch . --target myapp --debounce 1s    # less reactive, batchier
kinder dev --watch . --target myapp --debounce 100ms # very reactive
```

## fsnotify overflow

Heavy builds can write thousands of files in a short window and overflow the inotify queue. When this happens, fsnotify emits `ErrEventOverflow` and the kernel drops events.

`kinder dev` handles overflow by **synthesizing** a trigger event into the output channel. The cycle still fires — you just see a `WARN: fsnotify queue overflowed; synthesised one cycle` log entry. Without this, a heavy build would silently skip the rebuild it just produced.

To raise the inotify watch limit, see the [Inotify Limits](/known-issues/#inotify-limits) doctor check.

## Concurrent-cycle prevention

Cycles run **serially**. If a file change arrives during an in-flight cycle, the change is queued (one slot, via `Debounce(cap=1)`) and a follow-up cycle runs after the current one completes. This guarantees:

- No overlapping `docker build` invocations on the same target
- The user always gets a fresh cycle for changes that arrived during a build
- No silently-dropped changes (the leading-trigger debouncer with cap=1 absorbs *bursts* but preserves *separate* changes)

## Pod spec requirements

For `kinder dev` to actually replace the running pod with the freshly-built image, the target Deployment must satisfy two conditions:

1. **`imagePullPolicy: IfNotPresent`** — so Kubernetes uses the image already loaded into the node instead of trying to pull from a registry
2. **Image reference matches `--image`** — by default `<target>:dev`

Minimal Deployment that works with `kinder dev --target myapp`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 1
  selector: { matchLabels: { app: myapp } }
  template:
    metadata: { labels: { app: myapp } }
    spec:
      containers:
        - name: myapp
          image: myapp:dev
          imagePullPolicy: IfNotPresent
          ports: [ { containerPort: 8080 } ]
```

## SIGINT teardown

`kinder dev` registers `signal.NotifyContext` for SIGINT and SIGTERM. When you press Ctrl+C:

1. The current cycle (if any) is allowed to complete
2. The watcher is closed
3. The temporary kubeconfig file written at startup is cleaned up

The cleanup is deferred at startup, so even an unexpected exit (panic, kill) leaves no stray kubeconfigs in `$TMPDIR`.

## Examples

### Default Go server iteration

```sh
# Project layout: ./main.go, ./Dockerfile, ./deploy.yaml
kubectl apply -f deploy.yaml
kinder dev --watch . --target myapp
```

Edit `main.go`, save, and watch a 4-5 second build → load → rollout cycle complete.

### Specify image tag explicitly

```sh
kinder dev --watch . --target myapp --image myapp:v2-experimental
```

Useful if your Deployment references a specific tag and you don't want kinder dev to override it.

### Different namespace

```sh
kinder dev --watch . --target frontend --namespace web
```

### Tighter rollout timeout for fast-starting apps

```sh
kinder dev --watch . --target myapp --rollout-timeout 30s
```

### Polling mode on macOS Docker Desktop

```sh
kinder dev --watch ./src --target myapp --poll --poll-interval 500ms
```

---

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Watcher terminated cleanly via SIGINT/SIGTERM |
| `1` | Validation error (bad `--watch` path, missing required flag, no Dockerfile, target Deployment not found at startup) |

Cycle errors do NOT exit `kinder dev`. The watcher keeps running so you can fix the source and save again.

---

## Comparison with manual iteration

| Workflow | Commands per save | Coalesces bursts | Per-step timing |
|---|---|---|---|
| Manual (`docker build` + `kinder load images` + `kubectl rollout`) | 3 | No | No |
| Manual via `localhost:5001` registry | 4 | No | No |
| `kinder dev` | 0 (just save the file) | Yes (`--debounce`) | Yes |

For team-shared clusters or multi-image apps, the [Local Registry workflow](/guides/local-dev-workflow/) remains the right model. For single-developer iteration, `kinder dev` collapses the loop.

---

## See also

- [Local Dev Workflow](/guides/local-dev-workflow/#even-faster-kinder-dev) — full walkthrough with Dockerfile + Deployment
- [Load Images CLI reference](/cli-reference/load-images/) — the underlying image-load pipeline
- [Inotify Limits doctor check](/known-issues/#inotify-limits) — raise watch limits if the watcher is dropping events
