---
title: NVIDIA GPU
description: NVIDIA GPU addon for running GPU workloads in kinder clusters on Linux hosts.
---

The NVIDIA GPU addon enables GPU workloads in kinder clusters by installing the NVIDIA device plugin DaemonSet and an `nvidia` RuntimeClass. This allows Kubernetes to schedule pods that request `nvidia.com/gpu` resources onto GPU-capable nodes.

The addon is **Linux only** — on macOS and Windows it skips automatically with an informational message. It is **opt-in** (not enabled by default) because it requires specific hardware and host configuration.

kinder installs NVIDIA k8s-device-plugin **v0.17.1**.

## Prerequisites

The NVIDIA GPU addon requires:

- **Linux host** — macOS and Windows do not support GPU passthrough to Docker containers. The addon skips silently on non-Linux hosts.
- **NVIDIA GPU hardware** — a physical or pass-through GPU visible to the host.
- **NVIDIA driver installed** — `nvidia-smi` must be on your PATH.
- **nvidia-container-toolkit** — installed and Docker configured with the nvidia runtime.

Install and configure the toolkit:

```sh
# Install nvidia-container-toolkit
sudo apt-get install -y nvidia-container-toolkit
# Configure Docker to use nvidia runtime
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
```

**Required for kind:** kind cluster nodes run as Docker containers. The default environment-variable GPU injection strategy does not work for nested containers. Enable the volume-mounts strategy:

```sh
# Required for kind: enable volume-mounts strategy for GPU device injection
sudo nvidia-ctk config --set accept-nvidia-visible-devices-as-volume-mounts=true --in-place
sudo systemctl restart docker
```

:::tip[Verify prerequisites before creating a cluster]
Run `kinder doctor` to check that all prerequisites are met before creating a GPU cluster. This saves time compared to discovering missing components mid-installation.
:::

## What gets installed

| Resource | Namespace | Purpose |
|---|---|---|
| `nvidia-device-plugin` DaemonSet | `kube-system` | Exposes GPU resources to the Kubernetes scheduler |
| `nvidia` RuntimeClass | cluster-scoped | Allows pods to target GPU-enabled nodes |

## How to use

Create a cluster config file that enables the GPU addon:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  nvidiaGPU: true
```

Create the cluster:

```sh
kinder create cluster --config gpu-cluster.yaml
```

Once the cluster is running, apply a GPU test pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gpu-test
spec:
  restartPolicy: Never
  containers:
    - name: cuda-test
      image: nvcr.io/nvidia/k8s/cuda-sample:vectoradd-cuda11.7.1-ubuntu20.04
      resources:
        limits:
          nvidia.com/gpu: 1
```

Verify the pod completes successfully:

```sh
kubectl apply -f gpu-test.yaml
kubectl wait --for=condition=Ready pod/gpu-test --timeout=120s
kubectl logs gpu-test
```

Expected output:

```
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

## How to verify

After cluster creation, confirm the device plugin and RuntimeClass are installed:

```sh
kubectl get daemonset -n kube-system nvidia-device-plugin-daemonset
kubectl get runtimeclass nvidia
```

## Configuration

The GPU addon is controlled by the `addons.nvidiaGPU` field in your cluster config:

```yaml
addons:
  nvidiaGPU: true  # default: false (opt-in)
```

Unlike other kinder addons which are enabled by default, the GPU addon is opt-in because it requires specific hardware and host configuration.

See the [Configuration Reference](/configuration) for all available addon fields.

## How to disable

Set `nvidiaGPU: false` or omit the field entirely (the default is `false`):

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  nvidiaGPU: false
```

## Troubleshooting

### Pod stuck in Pending with "0/1 nodes are available: insufficient nvidia.com/gpu"

**Symptom:** A pod requesting `nvidia.com/gpu` stays in `Pending` state and `kubectl describe pod` shows `Insufficient nvidia.com/gpu`.

**Cause:** The NVIDIA device plugin has not registered any GPUs with Kubernetes. The most common cause with kind is that the `accept-nvidia-visible-devices-as-volume-mounts` setting is not enabled — kind nodes are Docker containers and the default environment-variable injection strategy does not work for nested containers.

Other possible causes:
- (b) nvidia-container-toolkit is not installed or Docker is not configured with the nvidia runtime
- (c) The host GPU is not visible inside the kind node container
- (d) The device plugin pod is crashing or not running

**Fix:**

```sh
# Step 1 (most likely fix): Enable volume-mounts strategy for kind
sudo nvidia-ctk config --set accept-nvidia-visible-devices-as-volume-mounts=true --in-place
sudo systemctl restart docker
# Then recreate the cluster

# Step 2: Check device plugin pod status
kubectl get pods -n kube-system -l name=nvidia-device-plugin-ds

# Step 3: Check if GPU is visible inside the node
docker exec -it <cluster-name>-control-plane nvidia-smi

# Step 4: If nvidia-smi fails inside the container, reconfigure Docker:
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
# Then recreate the cluster
```

### kinder create cluster fails with "nvidia-container-toolkit not found"

**Symptom:** `kinder create cluster` with `nvidiaGPU: true` fails immediately with an error about `nvidia-container-toolkit`.

**Cause:** The `nvidia-ctk` binary is not on your PATH.

**Fix:**

```sh
sudo apt-get install -y nvidia-container-toolkit
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
```

### GPU addon skipped with "Linux only" message

**Symptom:** The cluster creates successfully but the GPU addon logs "skipping on darwin (Linux only)" or similar.

**Cause:** The NVIDIA GPU addon only works on Linux hosts. macOS and Windows do not support GPU passthrough to Docker containers.

**Fix:** No fix — this is expected behavior. Use a Linux host or a Linux VM with GPU passthrough configured.

## Technical notes

:::note[FAIL_ON_INIT_ERROR: false]
The device plugin DaemonSet runs with `FAIL_ON_INIT_ERROR: "false"`. This allows the DaemonSet to start even on nodes without GPUs, which is useful for mixed clusters. The DaemonSet will log a warning on non-GPU nodes but will not crash.
:::

:::note[RuntimeClass]
The `nvidia` RuntimeClass is created for forward compatibility. Pods can optionally specify `runtimeClassName: nvidia` in their spec to ensure they run on a GPU-capable runtime. Most workloads do not need this — the device plugin makes `nvidia.com/gpu` resource requests work without specifying a RuntimeClass.
:::
