---
title: Dynamic Storage with PVCs
description: Use local-path-provisioner to dynamically provision PersistentVolumes — create a PVC, write data, survive pod restarts, and watch WaitForFirstConsumer in action.
---

In this tutorial you will use kinder's built-in `local-path-provisioner` addon to dynamically provision storage for a stateful workload. You'll create a `PersistentVolumeClaim`, deploy a pod that writes to it, delete the pod and verify the data survives, then see how the `WaitForFirstConsumer` binding mode works in a multi-node cluster. By the end you will understand the full PVC → PV → Pod lifecycle on a local kinder cluster with zero manual provisioning.

## Prerequisites

- [kinder installed](/installation)
- Docker (or Podman) installed and running
- `kubectl` installed and on PATH

## Step 1: Create the cluster

```sh
kinder create cluster
```

:::tip
`local-path-provisioner` is enabled by default in every kinder cluster. `local-path` is the cluster's only default StorageClass. No flags or config files are needed.
:::

## Step 2: Verify the provisioner is running

Confirm the provisioner Deployment is up:

```sh
kubectl get pods -n local-path-storage
```

Expected output:

```
NAME                                      READY   STATUS    RESTARTS   AGE
local-path-provisioner-...                1/1     Running   0          60s
```

Confirm `local-path` is the only StorageClass and is marked default:

```sh
kubectl get storageclass
```

Expected output:

```
NAME                   PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
local-path (default)   rancher.io/local-path   Delete          WaitForFirstConsumer   false                  2m
```

:::note[WaitForFirstConsumer]
Notice `VOLUMEBINDINGMODE` is `WaitForFirstConsumer`. This means the PV is **not** provisioned when the PVC is created — it is deferred until a pod actually references the PVC. You'll see this in action in Step 4.
:::

## Step 3: Create a PersistentVolumeClaim

Save the following as `pvc.yaml`:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

:::tip
No `storageClassName` field is needed — `local-path` is the cluster default, so the PVC automatically uses it.
:::

Apply it:

```sh
kubectl apply -f pvc.yaml
```

Check the PVC status immediately:

```sh
kubectl get pvc data
```

Expected output:

```
NAME   STATUS    VOLUME   CAPACITY   ACCESS MODES   STORAGECLASS   AGE
data   Pending                                      local-path     5s
```

The PVC is `Pending` — and that's expected. Because of `WaitForFirstConsumer`, no PV has been created yet. The provisioner is waiting for a pod to consume the PVC before it picks a node and creates the backing volume.

## Step 4: Deploy a pod that writes to the PVC

Save the following as `writer.yaml`. The pod writes a timestamped message to `/data/log.txt` every 5 seconds:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: writer
spec:
  containers:
    - name: writer
      image: busybox:1.37.0
      command: ["sh", "-c", "while true; do echo \"$(date): writer alive\" >> /data/log.txt; sleep 5; done"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: data
```

Apply it:

```sh
kubectl apply -f writer.yaml
```

Watch the PVC transition from `Pending` to `Bound` as the pod is scheduled:

```sh
kubectl get pvc data --watch
```

Expected output (press Ctrl+C once it's Bound):

```
NAME   STATUS    VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
data   Pending                                              1Gi        RWO            local-path     10s
data   Bound     pvc-8f2d1a4c-7b6e-4f9d-a1c2-3d4e5f6a7b8c   1Gi        RWO            local-path     20s
```

The provisioner ran a short-lived `helperPod` to create the backing directory under `/opt/local-path-provisioner/` inside the node, then created the PV and bound it to the claim — all automatically.

Verify the pod is running:

```sh
kubectl get pod writer
```

Expected output:

```
NAME     READY   STATUS    RESTARTS   AGE
writer   1/1     Running   0          30s
```

## Step 5: Verify data is being written

Wait 15 seconds for the pod to write a few log entries, then read the file from inside the container:

```sh
kubectl exec writer -- cat /data/log.txt
```

Expected output (your timestamps will differ):

```
Fri Apr 10 14:23:05 UTC 2026: writer alive
Fri Apr 10 14:23:10 UTC 2026: writer alive
Fri Apr 10 14:23:15 UTC 2026: writer alive
```

## Step 6: Delete the pod and verify data persists

This is the real test — does the data survive pod deletion? Delete the pod:

```sh
kubectl delete pod writer
```

Expected output:

```
pod "writer" deleted
```

Check the PVC — it should still be `Bound`:

```sh
kubectl get pvc data
```

Expected output:

```
NAME   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
data   Bound    pvc-8f2d1a4c-7b6e-4f9d-a1c2-3d4e5f6a7b8c   1Gi        RWO            local-path     3m
```

The PVC (and its backing PV) survives pod deletion. The data on disk is untouched.

Now recreate the pod using the same manifest:

```sh
kubectl apply -f writer.yaml
```

Wait for it to be running:

```sh
kubectl wait --for=condition=Ready pod/writer --timeout=30s
```

Read the log file:

```sh
kubectl exec writer -- cat /data/log.txt
```

Expected output (you should see both the old entries from the first pod **and** new entries from the new pod):

```
Fri Apr 10 14:23:05 UTC 2026: writer alive
Fri Apr 10 14:23:10 UTC 2026: writer alive
Fri Apr 10 14:23:15 UTC 2026: writer alive
Fri Apr 10 14:25:42 UTC 2026: writer alive
Fri Apr 10 14:25:47 UTC 2026: writer alive
```

The file persisted across pod restart. The second pod attached to the same PVC, which is still backed by the same directory on the node's filesystem.

## How it works

```
PVC (data)                      PV (pvc-8f2d1a4c-...)            Node filesystem
┌──────────┐                    ┌──────────────────┐             ┌───────────────────────────────┐
│ 1Gi RWO  │ ◄─── bound to ───► │ hostPath:        │ ◄─── on ──► │ /opt/local-path-provisioner/  │
│ local-   │                    │   /opt/local-... │             │   pvc-8f2d1a4c-.../           │
│ path     │                    │ reclaim: Delete  │             │     log.txt                   │
└──────────┘                    └──────────────────┘             └───────────────────────────────┘
     ▲                                                                          ▲
     │ volume mount                                                              │
     │                                                                           │
┌──────────┐                                                                     │
│   Pod    │ ─────────── writes /data/log.txt, appended to ──────────────────────┘
│ (writer) │
└──────────┘
```

1. **PVC created** — `kubectl apply -f pvc.yaml` creates a claim referencing `local-path`. Because the binding mode is `WaitForFirstConsumer`, the provisioner does nothing yet.
2. **Pod scheduled** — when you apply `writer.yaml`, the scheduler picks a node for the pod. The provisioner sees a pod referencing an unbound PVC and runs a `helperPod` on that node.
3. **PV created** — the helperPod creates a directory under `/opt/local-path-provisioner/pvc-<uuid>/` on the node, then a `PersistentVolume` with `hostPath` pointing at that directory is created and bound to the PVC.
4. **Pod mounts** — the pod starts and mounts the PV into its filesystem at `/data`.
5. **Data persists on disk** — writes go through the hostPath mount directly to the node's filesystem. Even after the pod is deleted, the directory stays; a new pod referencing the same PVC reattaches to the same directory.

:::note[Reclaim policy]
The `local-path` StorageClass uses `reclaim: Delete`. When you delete the PVC (not the pod), the provisioner runs another helperPod that removes the directory and frees the space. This is safe for local dev but means you cannot recover data after `kubectl delete pvc`.
:::

## Multi-node clusters

In a multi-node cluster, each PV is pinned to a single node — the node where the pod that first consumed the PVC was scheduled. This is an inherent property of local storage: the data lives on one node's filesystem, so pods using that PVC cannot be rescheduled elsewhere.

Try it with a multi-node cluster:

```yaml
# multi-node.yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
  - role: control-plane
  - role: worker
  - role: worker
```

```sh
kinder delete cluster
kinder create cluster --config multi-node.yaml
kubectl apply -f pvc.yaml
kubectl get pvc data
```

Expected output (note it's still `Pending`):

```
NAME   STATUS    VOLUME   CAPACITY   ACCESS MODES   STORAGECLASS   AGE
data   Pending                                      local-path     5s
```

Apply the writer pod:

```sh
kubectl apply -f writer.yaml
kubectl wait --for=condition=Ready pod/writer --timeout=60s
```

Inspect which node the pod landed on, and which node the PV is bound to:

```sh
kubectl get pod writer -o jsonpath='{.spec.nodeName}' && echo
kubectl get pv -o jsonpath='{.items[0].spec.nodeAffinity.required.nodeSelectorTerms[0].matchExpressions[0].values[0]}' && echo
```

Expected output (the two values must match):

```
kind-worker2
kind-worker2
```

The PV has node affinity binding it to the same node as the pod. This is why `WaitForFirstConsumer` is important: without it, the PV would be provisioned on an arbitrary node and then the scheduler would have to pin the pod to that node — limiting scheduling flexibility.

:::caution[Node deletion deletes data]
Because each PV is tied to a specific node's local filesystem, deleting the node (`kinder delete cluster`, rebuilding, etc.) destroys the data. For production-grade persistence, use networked storage (NFS, Ceph, cloud-managed) — `local-path-provisioner` is for local development only.
:::

## Air-gapped clusters

`local-path-provisioner` and its `busybox:1.37.0` helper image are both included in kinder's required addon image list, so they are automatically handled by:

- `kinder doctor` offline-readiness check
- `kinder create cluster --air-gapped` missing image error
- The addon image pre-pull warning NOTE printed by `kinder create cluster`

See the [Working Offline](/guides/working-offline/) guide for the full pre-load workflow.

## Troubleshooting

:::caution[PVC stuck in Pending after pod is deployed]
**Symptom:** `kubectl get pvc data` shows `STATUS: Pending` even after the writer pod is scheduled.

**Cause:** Either the provisioner Deployment is not running, or the helperPod failed to pull `busybox:1.37.0`.

**Fix:** Check both:

```sh
kubectl get pods -n local-path-storage
kubectl describe pvc data
```

Look for events mentioning `ImagePullBackOff` on the helperPod. If seen in an air-gapped cluster, pre-load `busybox:1.37.0` with `kinder load images docker.io/library/busybox:1.37.0`.
:::

:::caution[Pod sees empty /data after restart]
**Symptom:** The writer pod restarts and `/data/log.txt` is missing.

**Cause:** You likely deleted the PVC instead of the pod. Deleting the PVC triggers the reclaim policy (`Delete`) which runs a helperPod to remove the directory.

**Fix:** Delete only the pod, not the PVC, when testing persistence. Use `kubectl delete pod writer` — not `kubectl delete -f writer.yaml` if your file also defines the PVC.
:::

:::tip[Check with kinder doctor]
`kinder doctor` includes a CVE check (`local-path-cve`) that warns if a running cluster has local-path-provisioner below v0.0.34 (the fix version for CVE-2025-62878). Fresh kinder clusters ship v0.0.35 and pass. Run `kinder doctor` if you are using a reused cluster or a customized manifest.
:::

## Clean up

Delete the pod, PVC, and cluster:

```sh
kubectl delete pod writer
kubectl delete pvc data
kinder delete cluster
```

Deleting the PVC triggers the `Delete` reclaim policy, which removes the backing directory from the node. Deleting the cluster removes everything regardless — no manual cleanup is needed.

## See also

- [Local Path Provisioner addon reference](/addons/local-path-provisioner/) — full addon configuration, disable instructions, and CVE details
- [Host Directory Mounting](/guides/host-directory-mounting/) — an alternative approach when you want to share a specific host directory (not dynamically provisioned storage)
- [Working Offline](/guides/working-offline/) — pre-loading local-path-provisioner images for air-gapped clusters
