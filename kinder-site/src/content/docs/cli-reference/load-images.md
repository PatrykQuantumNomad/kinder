---
title: Load Images
description: Reference for the kinder load images command — provider-abstracted image loading with smart-load skip and Docker Desktop 27+ compatibility.
---

`kinder load images` loads one or more local container images into every node of a running cluster with a single command. It uses the active provider's native image export (docker, podman, nerdctl, finch, nerdctl.lima) and handles Docker Desktop 27+ containerd image store compatibility automatically.

## Synopsis

```
kinder load images <IMAGE> [IMAGE...] [flags]
```

## Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--name` | `-n` | `kind` | The cluster context name |
| `--nodes` | | (all nodes) | Comma-separated list of node names to load images into |
| `--help` | `-h` | | Show command help |

## Examples

### Load a single image

```sh
kinder load images nginx:alpine
```

Loads `nginx:alpine` from the host's local image store into every node of the `kind` cluster.

### Load multiple images in one invocation

```sh
kinder load images nginx:alpine redis:7-alpine postgres:16-alpine
```

All three images are saved to a single tar file, then imported concurrently on each node via `errors.UntilErrorConcurrent`. This is significantly faster than three separate invocations.

### Target a specific cluster by name

```sh
kinder load images myapp:latest --name my-cluster
```

Loads `myapp:latest` into nodes of the cluster named `my-cluster`.

### Target specific nodes only

```sh
kinder load images myapp:latest --nodes kind-worker,kind-worker2
```

Loads `myapp:latest` into only the named nodes. Useful for testing pod scheduling scenarios or when you want to verify node affinity behavior.

### Build-and-load iteration loop

A common dev workflow — rebuild locally and reload into the cluster:

```sh
docker build -t myapp:dev .
kinder load images myapp:dev
kubectl rollout restart deployment/myapp
```

Combined with `imagePullPolicy: IfNotPresent` on the pod spec, this gives you fast iteration without pushing to an external registry. For longer-running workflows, consider the [Local Registry](/addons/local-registry/) addon instead.

---

## Provider support

`kinder load images` works identically with all three providers. It uses `providerBinaryName()` to resolve the actual binary for the active runtime:

| `KIND_EXPERIMENTAL_PROVIDER` | Binary invoked for `save` / `image inspect` |
|---|---|
| `docker` (or unset default) | `docker` |
| `podman` | `podman` |
| `nerdctl` | `nerdctl` |
| `finch` | `finch` |
| `nerdctl.lima` | `nerdctl.lima` |

For nerdctl-family providers, `kinder load images` reads `KIND_EXPERIMENTAL_PROVIDER` directly rather than using the generic `"nerdctl"` provider name, because the actual binary can differ (finch, nerdctl.lima, nerdctl).

### Podman example

```sh
KIND_EXPERIMENTAL_PROVIDER=podman kinder load images nginx:alpine
```

Under the hood, this calls `podman save` (not `docker save`) to export the image tar.

### Finch example

```sh
KIND_EXPERIMENTAL_PROVIDER=finch kinder load images nginx:alpine
```

This calls `finch save` to export the image tar.

---

## Smart-load skip

Re-running `kinder load images` with an image already present on all target nodes completes without re-importing:

```sh
kinder load images nginx:alpine
# Image: "nginx:alpine" with ID "..." found to be already present on all nodes.
```

Smart-load works by comparing the host's image ID (from `docker image inspect -f '{{ .Id }}'`) against the image ID on each node (via `crictl inspecti`). If IDs match on every candidate node, the import is skipped.

### Re-tag without re-import

If an image with the same content (same ID) exists on a node but is missing the requested tag, kinder re-tags it in place using `ctr images tag --force` instead of importing again:

```
Image with ID: sha256:abc... already present on the node kind-control-plane
but is missing the tag nginx:alpine. re-tagging...
```

This is much faster than a full import and is especially useful when the same image has been loaded under a different tag.

---

## Docker Desktop 27+ containerd compatibility

Docker Desktop 27+ with the containerd image store enabled produces archives that `ctr images import --all-platforms` rejects with `content digest: not found`. `kinder load images` detects this error and automatically retries without `--all-platforms`:

1. **First attempt:** `ctr --namespace=k8s.io images import --all-platforms --digests --snapshotter=<snap> -` (standard behavior)
2. **If the first attempt fails with "content digest"** → retry without `--all-platforms`
3. **All other errors** propagate immediately

The fallback uses a factory pattern (`openArchive func() (io.ReadCloser, error)`) to re-open the tar stream for the retry, because tar streams cannot be rewound once read.

This fallback is transparent — you don't need a flag to enable it, and you won't see the first-attempt error unless both attempts fail.

:::note[Why this happens]
See upstream [kind#3795](https://github.com/kubernetes-sigs/kind/issues/3795). Docker Desktop 27+ `docker save` with the containerd image store produces multi-platform blob references that containerd 1.7 cannot resolve in the `ctr images import --all-platforms` path. Dropping `--all-platforms` forces single-platform import, which works.
:::

---

## Exit codes

| Code | Meaning |
|---|---|
| `0` | All images loaded (or all already present on all nodes) |
| non-zero | Image not found locally, cluster not found, or node import failed |

If the command finds an image missing on the host, it exits immediately with:

```
ERROR: image: "nginx:alpine" not present locally
```

Pull it first with your provider's `pull` command, then retry.

---

## Comparison with other load commands

kinder inherits two additional load subcommands from upstream kind:

| Command | Source | Provider support | Best for |
|---|---|---|---|
| `kinder load images` | kinder v1.4+ | docker, podman, nerdctl, finch, nerdctl.lima | Most use cases — provider-abstracted, smart-load, Docker Desktop 27+ compatible |
| `kinder load docker-image` | upstream kind | docker only (hardcoded `docker save`) | Legacy workflows; prefer `load images` |
| `kinder load image-archive` | upstream kind | any (reads a pre-built tar file) | Loading a tar produced by a build system |

Use `kinder load images` as your default. The other two subcommands remain available for backward compatibility and edge cases.

---

## See also

- [Working Offline](/guides/working-offline/) — the two-mode offline workflow that uses `kinder load images` in Mode 2
- [Local Registry](/addons/local-registry/) — for longer-running dev loops that benefit from a persistent registry instead of per-rebuild loads
- [CLI Troubleshooting](/cli-reference/troubleshooting/) — common issues with kinder commands
