---
title: Host Directory Mounting
description: Mount host directories into cluster nodes and expose them to pods via hostPath PersistentVolumes.
---

In this tutorial you will share a directory from your host machine with pods running inside a kinder cluster. Because kinder nodes are Docker (or Podman) containers, a host directory cannot reach a pod in a single step — it travels through two hops: the host directory is mounted into the node container via `extraMounts`, and from there a Kubernetes `hostPath` PersistentVolume exposes it to pods. By the end of this guide you will have a running pod that reads a file written on your host.

## Prerequisites

- [kinder installed](/installation)
- Docker (or Podman) installed and running
- `kubectl` installed and on PATH
- A host directory you want to share with the cluster

## Step 1: Create a host directory

Create the directory and add a test file so you can verify the mount end-to-end:

```sh
mkdir -p ~/shared-data
echo "hello from the host" > ~/shared-data/hello.txt
```

## Step 2: Create the cluster with extraMounts

Save the following as `cluster-config.yaml`. The `extraMounts` field tells kinder (and the underlying kind cluster) to bind-mount `~/shared-data` from your host into each node container at `/mnt/host-data`:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraMounts:
      - hostPath: /Users/you/shared-data
        containerPath: /mnt/host-data
```

Replace `/Users/you/shared-data` with the absolute path of the directory you created in Step 1 (use `echo ~/shared-data` to resolve the tilde).

:::note
`hostPath` is the path on your host machine. `containerPath` is where that path appears inside the node container. These two fields are the only ones you need for most use cases.
:::

Create the cluster:

```sh
kinder create cluster --config cluster-config.yaml
```

:::tip
kinder validates that `hostPath` exists before creating any containers. If the path does not exist you will see an error before cluster creation begins, so you can fix the path without having to tear down a partially created cluster.
:::

## Step 3: Create a PersistentVolume and PersistentVolumeClaim

Now that the directory is available inside every node container at `/mnt/host-data`, create a Kubernetes `hostPath` PersistentVolume that points to that `containerPath`. Save the following as `pv-pvc.yaml`:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: host-data-pv
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  storageClassName: manual
  hostPath:
    path: /mnt/host-data
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: host-data-pvc
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: manual
  resources:
    requests:
      storage: 1Gi
```

Apply it:

```sh
kubectl apply -f pv-pvc.yaml
```

Verify the PV is bound:

```sh
kubectl get pv host-data-pv
```

Expected output:

```
NAME           CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                    STORAGECLASS   AGE
host-data-pv   1Gi        RWO            Retain           Bound    default/host-data-pvc    manual         10s
```

:::caution
The `storageClassName: manual` value must match exactly in both the PersistentVolume and PersistentVolumeClaim. Kubernetes uses it to bind them together. If they do not match, the PVC will remain in `Pending` state.
:::

## Step 4: Deploy a pod that consumes the PVC

Save the following as `reader-pod.yaml`. The pod mounts the PVC at `/data` and reads `hello.txt`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: host-data-reader
spec:
  containers:
    - name: reader
      image: busybox:1.37.0
      command: ["sh", "-c", "cat /data/hello.txt && sleep 3600"]
      volumeMounts:
        - name: host-data
          mountPath: /data
  volumes:
    - name: host-data
      persistentVolumeClaim:
        claimName: host-data-pvc
```

Apply it:

```sh
kubectl apply -f reader-pod.yaml
```

Wait for the pod to be running:

```sh
kubectl get pod host-data-reader
```

Expected output:

```
NAME               READY   STATUS    RESTARTS   AGE
host-data-reader   1/1     Running   0          15s
```

## Step 5: Verify the mount

Read the logs from the pod to confirm it can see the file from your host:

```sh
kubectl logs host-data-reader
```

Expected output:

```
hello from the host
```

The pod read `hello.txt` directly from your host machine through the two-hop mount chain.

## How it works

Data travels through two hops to reach the pod:

```
Host machine             Node container          Pod
~/shared-data    --->    /mnt/host-data   --->   /data
(extraMounts)            (hostPath PV)           (volumeMount)
```

1. **Hop 1 — `extraMounts`:** kinder passes the `hostPath`/`containerPath` pair to the container runtime (Docker or Podman) when it creates the node container. The host directory appears at `containerPath` inside the node.
2. **Hop 2 — `hostPath` PV:** Kubernetes creates a PersistentVolume backed by `hostPath: /mnt/host-data`. Because this path already exists in the node's filesystem (from Hop 1), the volume is immediately available. The PVC binds to the PV and the pod mounts it.

:::note
Changes you make to files in `~/shared-data` on your host are visible immediately inside the pod — no rebuild, restart, or sync step required.
:::

## Platform notes

### macOS (Docker Desktop)

On macOS, Docker Desktop runs a Linux VM. For a host directory to reach a kinder node container, Docker Desktop must be configured to share it with that VM first.

**File sharing:** Open Docker Desktop → Settings → Resources → File sharing. Add the parent directory of your `hostPath` (e.g., `/Users/you`) if it is not already listed. The default shared paths are `/Users`, `/Volumes`, `/private`, `/tmp`, and `/var/folders`. Paths outside these directories will not be accessible.

:::caution
If your `hostPath` is outside the Docker Desktop file-sharing list, the `extraMounts` bind-mount will silently produce an empty directory inside the node container. The PV will bind and the pod will start, but the mounted path will be empty. Add the path to Docker Desktop file sharing and recreate the cluster.
:::

**Mount propagation:** The `propagation` field in `extraMounts` controls whether sub-mounts created at runtime are visible across the host/node boundary. On macOS, propagation modes other than `None` are not supported for paths that originate on the macOS host (outside the Docker Desktop VM). Set `propagation: None` (the default) unless you are mounting a path that exists only inside the Docker Desktop Linux VM.

### Windows (Docker Desktop with WSL 2)

Windows paths must use forward slashes and be expressed as WSL paths (e.g., `/mnt/c/Users/you/shared-data` rather than `C:\Users\you\shared-data`). Ensure the directory is within a WSL 2 filesystem or is exposed via Docker Desktop's drive sharing.

### Linux

On Linux, Docker (or Podman) runs natively and there is no intermediate VM. All host paths are directly accessible. Mount propagation modes (`HostToContainer`, `Bidirectional`) work as expected.

## Troubleshooting

:::caution[PVC stuck in Pending]
If `kubectl get pvc host-data-pvc` shows `STATUS: Pending`, the PV and PVC `storageClassName` fields do not match, or the PV `accessModes` do not satisfy the PVC request. Describe both resources and compare the fields:

```sh
kubectl describe pv host-data-pv
kubectl describe pvc host-data-pvc
```
:::

:::caution[Pod sees an empty /data directory]
The mount chain is intact but the source directory is empty or not shared. On macOS, check Docker Desktop file sharing settings. On all platforms, verify the `containerPath` in your cluster config matches the `path` in your PersistentVolume spec exactly.
:::

:::caution[cluster creation fails with path not found]
kinder checks that `hostPath` exists before creating any containers. Create the directory first (`mkdir -p ~/shared-data`) and then retry `kinder create cluster --config cluster-config.yaml`.
:::

:::tip[Run kinder doctor for mount diagnostics]
`kinder doctor` includes a host mount check that inspects configured `extraMounts` paths and, on macOS, verifies that Docker Desktop file sharing covers each path. Run it after cluster creation if you suspect a mount issue:

```sh
kinder doctor
```

Look for any `WARN` entries under the mount checks section. Each warning includes an actionable message describing what to fix.
:::

## Clean up

Delete the pod and Kubernetes objects:

```sh
kubectl delete pod host-data-reader
kubectl delete pvc host-data-pvc
kubectl delete pv host-data-pv
```

Delete the cluster:

```sh
kinder delete cluster
```

Optionally remove the host directory:

```sh
rm -rf ~/shared-data
```
