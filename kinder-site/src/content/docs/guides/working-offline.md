---
title: Working Offline
description: Create and use kinder clusters in air-gapped or offline environments without network access.
---

kinder supports creating fully functional clusters without internet access. This guide covers the two-mode offline workflow, pre-flight readiness checks, and how to pre-load images from a connected machine onto an air-gapped host.

## Prerequisites

- [kinder installed](/installation) on the air-gapped host
- Docker, Podman, or nerdctl installed and running
- (For pre-loading) a connected machine with internet access and the same container runtime

## The two-mode offline workflow

There are two approaches to working offline with kinder:

**Mode 1: Pre-create image baking (recommended)**

Pre-load all required images onto the host before creating the cluster. This works with `--air-gapped` and is the simplest approach for fully disconnected environments.

**Mode 2: Post-create image loading**

Create the cluster first (with at least the node image available), then load additional application images into the running nodes via `kinder load images`. Useful for iterative development or for testing locally-built images.

---

## Mode 1: Pre-create image baking

### Step 1: Identify required images

Run `kinder doctor` on the air-gapped host to see which images are already present and which are missing:

```sh
kinder doctor
```

Look for the **offline-readiness** check in the output. It lists every image kinder will install, labeled by addon, so you can skip images for addons you plan to disable.

Alternatively, on a connected machine, run `kinder create cluster` without `--air-gapped` first — kinder prints a NOTE listing every addon image that will be pulled:

```
NOTE: the following addon images will be pulled during cluster creation:
  - quay.io/metallb/controller:v0.15.3
  - quay.io/metallb/speaker:v0.15.3
  - docker.io/envoyproxy/gateway:v1.3.1
  - registry.k8s.io/metrics-server/metrics-server:v0.8.1
  - ghcr.io/headlamp-k8s/headlamp:v0.40.1
  - registry:2
  - quay.io/jetstack/cert-manager-controller:v1.16.3
  - quay.io/jetstack/cert-manager-webhook:v1.16.3
  - quay.io/jetstack/cert-manager-cainjector:v1.16.3
  - docker.io/rancher/local-path-provisioner:v0.0.35
  - docker.io/library/busybox:1.37.0
```

### Step 2: Pull and save images on a connected machine

On a machine with internet access, pull each image and export it to a tarball:

```sh
# Node image
docker pull kindest/node:v1.35.1
docker save kindest/node:v1.35.1 | gzip > node.tar.gz

# Addon images — group them into a single tarball for efficient transfer
docker pull quay.io/metallb/controller:v0.15.3 quay.io/metallb/speaker:v0.15.3
docker pull docker.io/envoyproxy/gateway:v1.3.1
docker pull registry.k8s.io/metrics-server/metrics-server:v0.8.1
docker pull ghcr.io/headlamp-k8s/headlamp:v0.40.1
docker pull registry:2
docker pull quay.io/jetstack/cert-manager-controller:v1.16.3
docker pull quay.io/jetstack/cert-manager-webhook:v1.16.3
docker pull quay.io/jetstack/cert-manager-cainjector:v1.16.3
docker pull docker.io/rancher/local-path-provisioner:v0.0.35
docker pull docker.io/library/busybox:1.37.0

docker save \
  quay.io/metallb/controller:v0.15.3 \
  quay.io/metallb/speaker:v0.15.3 \
  docker.io/envoyproxy/gateway:v1.3.1 \
  registry.k8s.io/metrics-server/metrics-server:v0.8.1 \
  ghcr.io/headlamp-k8s/headlamp:v0.40.1 \
  registry:2 \
  quay.io/jetstack/cert-manager-controller:v1.16.3 \
  quay.io/jetstack/cert-manager-webhook:v1.16.3 \
  quay.io/jetstack/cert-manager-cainjector:v1.16.3 \
  docker.io/rancher/local-path-provisioner:v0.0.35 \
  docker.io/library/busybox:1.37.0 \
  | gzip > kinder-addons.tar.gz
```

:::note[Podman / nerdctl users]
Replace `docker pull` / `docker save` with your runtime's equivalent: `podman pull` / `podman save`, or `nerdctl pull` / `nerdctl save`. kinder's `--air-gapped` mode works identically with all three providers.
:::

### Step 3: Transfer and load on the air-gapped host

Transfer `node.tar.gz` and `kinder-addons.tar.gz` to the air-gapped host via your usual mechanism (USB drive, internal file transfer, etc.), then load them:

```sh
docker load < node.tar.gz
docker load < kinder-addons.tar.gz
```

Confirm readiness with `kinder doctor`:

```sh
kinder doctor
```

The offline-readiness check should now show all required images as present.

### Step 4: Create the cluster in air-gapped mode

Once all images are pre-loaded:

```sh
kinder create cluster --air-gapped
```

In `--air-gapped` mode, kinder:

- Fails fast with a complete list of missing images if anything is absent (no partial creation, no hung pulls)
- Skips all network calls that would normally pull images
- Uses only images already present in the local image store

If any required image is missing, you'll see an error like:

```
ERROR: air-gapped mode requires these images to be pre-loaded:
  - quay.io/metallb/controller:v0.15.3
  - quay.io/metallb/speaker:v0.15.3

Pre-load with: docker pull <image> && docker save <image> | docker load
```

Pre-load the missing images and re-run.

---

## Mode 2: Post-create image loading

Create the cluster (either with `--air-gapped` if node images are pre-loaded, or normally if you have limited connectivity), then load additional application images into the running nodes:

```sh
# Create the cluster
kinder create cluster --air-gapped

# Build or pull an image on the host
docker build -t myapp:latest .

# Load it into every node in the cluster
kinder load images myapp:latest
```

The `kinder load images` command works with all three providers (docker, podman, nerdctl) and handles Docker Desktop 27+ containerd image store compatibility automatically.

Re-running with an image that's already present is a no-op:

```sh
kinder load images myapp:latest
# Image: "myapp:latest" with ID "..." found to be already present on all nodes.
```

See the [Load Images CLI reference](/cli-reference/load-images/) for full details on flags, provider behavior, and smart-load semantics.

---

## Disabling addons to reduce image footprint

If you don't need all addons, disable them in your cluster config to skip their images entirely:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  envoyGateway: false     # skip envoyproxy/gateway
  dashboard: false        # skip headlamp
  localRegistry: false    # skip registry:2
  certManager: false      # skip 3 cert-manager images
```

Then create with your config:

```sh
kinder create cluster --config cluster.yaml --air-gapped
```

kinder's required image list adapts to the enabled addon set, so `--air-gapped` will only check for images of the addons you kept enabled.

:::tip[Using `--profile` for common presets]
The `--profile` flag selects curated addon sets:

- `--profile minimal` — no kinder addons (smallest image footprint)
- `--profile gateway` — MetalLB + Envoy Gateway only
- `--profile ci` — Metrics Server + cert-manager (CI-optimized)

Combine with `--air-gapped` to pre-load only what each profile needs.
:::

---

## Troubleshooting

### `docker save` produces an archive that fails to import

**Symptom:** On Docker Desktop 27+ with the containerd image store enabled, `kinder load images` or cluster creation fails with `content digest: not found`.

**Cause:** Docker Desktop 27+ `docker save` with the containerd image store produces multi-platform archives that `ctr images import --all-platforms` cannot resolve.

**Fix:** `kinder load images` automatically detects this and retries without `--all-platforms`. No action required — the fallback is transparent.

For cluster creation, the kind node image itself is affected. Workaround: in Docker Desktop Settings → General, disable "Use containerd for pulling and storing images", then re-pull and re-save the node image.

### Missing image error despite running `docker load`

**Symptom:** `kinder create cluster --air-gapped` reports an image as missing even though you just ran `docker load`.

**Cause:** The loaded image tag doesn't match exactly. kinder expects fully-qualified tags like `quay.io/metallb/controller:v0.15.3`, not short forms.

**Fix:** Verify the loaded image matches the expected tag:

```sh
docker image inspect quay.io/metallb/controller:v0.15.3 | head
```

If the tag differs, re-tag:

```sh
docker tag <loaded-tag> quay.io/metallb/controller:v0.15.3
```

### No container runtime found

**Symptom:** `kinder doctor` reports the offline-readiness check as skipped.

**Cause:** kinder did not find a container runtime binary on `PATH`. The offline-readiness check needs the runtime to inspect the local image store.

**Fix:** Ensure docker, podman, or nerdctl is installed and on `PATH`. If you have set `KIND_EXPERIMENTAL_PROVIDER`, verify the binary matches.
