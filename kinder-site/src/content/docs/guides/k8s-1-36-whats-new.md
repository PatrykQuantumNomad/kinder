---
title: What's new in Kubernetes 1.36
description: Demonstrate User Namespaces (GA) and In-Place Pod Resize (GA) on a kinder cluster running Kubernetes 1.36.
---

Kubernetes 1.36 (released April 2026) graduates two security and scalability features to General Availability: **User Namespaces** and **In-Place Pod Resize** (container-level). Both are enabled by default in 1.36 — no feature gates or extra flags required. This guide walks through applying each feature on a kinder cluster and verifying the result.

This guide focuses on the two GA features you can exercise immediately. Pod-level resource sharing (`InPlacePodLevelResourcesVerticalScaling`) graduated to Beta in 1.36 but is not covered here.

## Prerequisites

- kinder v0.X or later (the version that ships with default Kubernetes 1.36 support — see SYNC-02 release notes)
- A running kinder cluster on Kubernetes 1.36 or later, for example created with `kinder create cluster`
- `kubectl` configured to reach the cluster (`kubectl cluster-info` returns a 1.36 API server)
- A Linux host with kernel 5.12 or later for the User Namespaces section; on macOS, Docker Desktop's Linux VM satisfies this requirement out of the box

## User Namespaces (GA)

User Namespaces map a container's internal UID/GID range to a distinct, unprivileged range on the host. A process that appears as `root` (UID 0) inside a container is mapped to a high, non-root UID on the host — for example, `1000000`. This limits the blast radius of a container escape: the escaped process has no privileges outside its mapped range.

In Kubernetes 1.36, User Namespaces are **GA**. The `UserNamespacesSupport` feature gate is permanently enabled; you do not need to set it. All you need is `hostUsers: false` in the pod spec.

### Apply the demo pod

Save the following as `userns-demo.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: userns-demo
spec:
  hostUsers: false
  containers:
  - name: app
    image: fedora:42
    securityContext:
      runAsUser: 0
    command: ["sh", "-c", "whoami && cat /proc/self/uid_map"]
```

Apply it:

```bash
kubectl apply -f userns-demo.yaml
```

### Verify the namespace mapping

Wait for the pod to complete, then inspect the logs:

```bash
kubectl logs userns-demo
```

Expected output (exact UID range numbers may differ):

```
root
         0    1000000      65536
```

The first line (`root`) comes from `whoami` — inside the container the process is UID 0. The second line comes from `/proc/self/uid_map` — on the host, that UID 0 is mapped to UID `1000000` (or similar high value), proving the namespace is active. A breakout from this container would land as an unprivileged user on the host.

:::tip
On Docker Desktop the host-side UID range is allocated by the Docker VM's `/etc/subuid` configuration — this is usually fine out of the box with the default `kindest/base` image. If you see "permission denied" errors when starting pods with `hostUsers: false`, check the User Namespaces requirements page at https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/ for the kernel version and subuid prerequisites.
:::

## In-Place Pod Resize (GA, container-level)

In-Place Pod Resize lets you change CPU and memory `requests` and `limits` on a **running** container without restarting the pod, provided the container's `resizePolicy` marks those resources as `NotRequired` for restart. Container-level resize (`InPlacePodVerticalScaling`) graduated to GA in Kubernetes 1.35 and is on by default in 1.36 — no feature gate required.

### Apply the demo pod

Save the following as `resize-demo.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: resize-demo
spec:
  containers:
  - name: app
    image: registry.k8s.io/pause:3.10
    resources:
      requests:
        cpu: "100m"
        memory: "64Mi"
      limits:
        cpu: "200m"
        memory: "128Mi"
    resizePolicy:
    - resourceName: cpu
      restartPolicy: NotRequired
    - resourceName: memory
      restartPolicy: NotRequired
```

Apply it:

```bash
kubectl apply -f resize-demo.yaml
```

### Inspect the initial resources

Once the pod is running, read its actual allocated resources from the status:

```bash
kubectl get pod resize-demo -o jsonpath='{.status.containerStatuses[0].resources}'
```

Expected output:

```json
{"limits":{"cpu":"200m","memory":"128Mi"},"requests":{"cpu":"100m","memory":"64Mi"}}
```

### Resize without restarting

Patch the pod using the `resize` subresource to double the CPU request:

```bash
kubectl patch pod resize-demo --subresource resize \
  --patch '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"200m"}}}]}}'
```

Re-run the inspect command:

```bash
kubectl get pod resize-demo -o jsonpath='{.status.containerStatuses[0].resources}'
```

The CPU request should now reflect `200m`. Verify the pod was never restarted:

```bash
kubectl get pod resize-demo -o jsonpath='{.status.containerStatuses[0].restartCount}'
```

Expected output: `0`. The container absorbed the resource change in place.

:::note
Pod-level resource sharing (`InPlacePodLevelResourcesVerticalScaling`) is **Beta** in Kubernetes 1.36 and uses a different pod spec with its own four feature gates. This guide covers only the GA container-level path described above.
:::

## kubeadm v1beta4 note

Kind is tracking the adoption of kubeadm configuration API `v1beta4` for Kubernetes 1.36+ clusters (kind issue #3847). The `v1beta4` API changes the `extraArgs` field from a `map[string]string` (v1beta3) to a list of `{name, value}` objects.

If you have existing `kubeadmConfigPatches` in your kinder cluster config that use the v1beta3 `extraArgs` map syntax, you may need to update them once kind ships a release that defaults to v1beta4 for 1.36+ clusters. Until then, v1beta3 patches continue to work. See the kubeadm config reference for the v1beta4 schema: https://kubernetes.io/docs/reference/config-api/kubeadm-config.v1beta4/

## Clean up

Delete both demo pods when you are done:

```bash
kubectl delete pod userns-demo resize-demo
```

## References

- Kubernetes 1.36 release notes: https://kubernetes.io/releases/
- User Namespaces GA blog post: https://kubernetes.io/blog/2026/04/23/kubernetes-v1-36-userns-ga/
- In-Place Pod Resize GA (v1.35) blog post: https://kubernetes.io/blog/2025/12/19/kubernetes-v1-35-in-place-pod-resize-ga/
- kubeadm v1beta4 config reference: https://kubernetes.io/docs/reference/config-api/kubeadm-config.v1beta4/
- User Namespaces concepts: https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/
