---
title: Local Registry
description: Local container registry addon for pushing and pulling images in kinder clusters.
---

The local registry gives kinder clusters a private container registry at `localhost:5001`. Push images from your host and pull them directly in pods — no external registry or image loading required.

kinder installs the **registry:2** image.

## What gets installed

| Resource | Namespace / Location | Purpose |
|---|---|---|
| `kind-registry` container | Docker/Podman host | Runs the registry on port 5001 |
| containerd `certs.d` config | All cluster nodes | Routes `localhost:5001` to the registry container |
| `local-registry-hosting` ConfigMap | `kube-public` | Dev tool discovery (Tilt, Skaffold, etc.) |

### How it works

The registry runs as a standalone container on the same Docker/Podman network as the cluster nodes. Each node is configured with a containerd `hosts.toml` entry that resolves `localhost:5001` to the registry container. This means both the host machine and pods inside the cluster can push and pull from the same address.

The registry container persists across cluster recreations — cached images survive `kinder delete cluster` and `kinder create cluster`.

## How to use

Push an image from your host:

```sh
docker build -t localhost:5001/myapp:latest .
docker push localhost:5001/myapp:latest
```

Reference it in a pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: myapp
spec:
  containers:
    - name: myapp
      image: localhost:5001/myapp:latest
```

## How to verify

After creating a cluster, confirm the registry container is running:

```sh
docker ps --filter name=kind-registry
```

Expected output:

```
CONTAINER ID   IMAGE        ...   PORTS                    NAMES
abc123         registry:2   ...   0.0.0.0:5001->5000/tcp   kind-registry
```

Verify the ConfigMap exists for dev tool discovery:

```sh
kubectl get configmap local-registry-hosting -n kube-public -o yaml
```

Test a full push-and-pull cycle:

```sh
docker pull nginx:alpine
docker tag nginx:alpine localhost:5001/nginx:test
docker push localhost:5001/nginx:test
kubectl run regtest --image=localhost:5001/nginx:test
kubectl get pod regtest
```

## Configuration

The local registry is controlled by the `addons.localRegistry` field in your cluster config:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  localRegistry: true  # default
```

See the [Configuration Reference](/configuration) for all available addon fields.

## How to disable

To create a cluster without the local registry, set `localRegistry: false`:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  localRegistry: false
```

## Dev tool integration

The `local-registry-hosting` ConfigMap in `kube-public` follows [KEP-1755](https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/1755-communicating-a-local-registry), which allows tools like Tilt and Skaffold to auto-discover the registry endpoint. No additional configuration is needed in these tools.

## Multi-image workflow

Build and push multiple images, then verify they are all available in the registry:

```sh
docker build -t localhost:5001/frontend:v1 ./frontend
docker push localhost:5001/frontend:v1

docker build -t localhost:5001/backend:v1 ./backend
docker push localhost:5001/backend:v1
```

Verify both images are registered:

```sh
curl http://localhost:5001/v2/_catalog
```

Expected output:

```json
{"repositories":["backend","frontend"]}
```

Reference both images in a Deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
        - name: frontend
          image: localhost:5001/frontend:v1
        - name: backend
          image: localhost:5001/backend:v1
```

## Cleaning up images

The simplest cleanup approach is to re-push with the same tag to overwrite an existing image:

```sh
docker build -t localhost:5001/myapp:latest .
docker push localhost:5001/myapp:latest  # overwrites the previous image
```

To list all tags for a specific image:

```sh
curl http://localhost:5001/v2/myapp/tags/list
```

Note that `registry:2` does not garbage-collect by default — disk space is reclaimed only when the registry container is recreated. For a fresh start:

```sh
docker rm -f kind-registry
```

The registry will be recreated on the next `kinder create cluster`.

## Troubleshooting

### ImagePullBackOff from localhost:5001

**Symptom:** `kubectl describe pod` shows `Failed to pull image "localhost:5001/myapp:latest"` with `ImagePullBackOff`.

**Cause:** The `kind-registry` container is not running — it may have been stopped or removed.

**Fix:** Check whether the registry is running:

```sh
docker ps --filter name=kind-registry
```

If the container exists but is stopped, start it:

```sh
docker start kind-registry
```

If it was removed, the simplest fix is to recreate the cluster:

```sh
kinder create cluster
```

This recreates the registry container automatically.

## Platform notes

:::caution[Rootless Podman limitation]
On **rootless Podman**, pushing to `localhost:5001` may require adding the registry as an insecure registry in `/etc/containers/registries.conf`:

```toml
[[registry]]
location = "localhost:5001"
insecure = true
```

kinder prints a warning with these instructions when it detects a rootless Podman environment.
:::
