---
title: Local Path Provisioner
description: Automatic dynamic PersistentVolume provisioning for kinder clusters via local-path-provisioner.
---

Local Path Provisioner gives kinder clusters automatic dynamic PVC provisioning out of the box. Create a `PersistentVolumeClaim` and a matching `PersistentVolume` is provisioned on the node's local disk immediately — no manual operator action required.

kinder installs **local-path-provisioner v0.0.36** as a default addon. `local-path` is set as the only default StorageClass, replacing the legacy `standard` StorageClass that shipped with upstream kind.

:::note[Security fix in v0.0.36]
v0.0.36 closes [GHSA-7fxv-8wr2-mfc4](https://github.com/rancher/local-path-provisioner/security/advisories/GHSA-7fxv-8wr2-mfc4), a HelperPod Template Injection vulnerability. Upgrade from v0.0.35 is strongly recommended.
:::

## What gets installed

| Resource | Namespace | Purpose |
|---|---|---|
| `local-path-provisioner` Deployment | `local-path-storage` | Watches for unbound PVCs referencing `local-path` and provisions `hostPath` PVs on the node |
| `local-path` StorageClass | cluster-wide | Default StorageClass for dynamic provisioning |
| `local-path-provisioner-service-account` | `local-path-storage` | Service account with minimal RBAC for PV lifecycle |
| `local-path-config` ConfigMap | `local-path-storage` | Provisioner configuration (paths, `helperPod` template) |

### How it works

The provisioner runs as a Deployment in the `local-path-storage` namespace. When a `PersistentVolumeClaim` is created with `storageClassName: local-path` (or no storageClassName, because `local-path` is the cluster default), the controller:

1. Schedules a short-lived `helperPod` on the node where the pod requesting the PVC will run.
2. The helper creates a directory under `/opt/local-path-provisioner/` on the node's filesystem.
3. A `PersistentVolume` with `hostPath` pointing at that directory is created and bound to the PVC.
4. When the PVC is deleted (and the reclaim policy is `Delete`), another helper pod cleans up the directory.

The embedded manifest pins `busybox:1.37.0` with `imagePullPolicy: IfNotPresent`, so PVC operations work correctly in air-gapped clusters where `busybox:latest` cannot be pulled.

## How to use

Create a `PersistentVolumeClaim`:

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

:::note
No `storageClassName` is required — `local-path` is the cluster default.
:::

Apply it and confirm it transitions to `Bound` automatically:

```sh
kubectl apply -f pvc.yaml
kubectl get pvc data
```

Expected output:

```
NAME   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
data   Bound    pvc-abc123...                              1Gi        RWO            local-path     5s
```

Reference it in a pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: writer
spec:
  containers:
    - name: writer
      image: busybox:1.37.0
      command: ["sh", "-c", "echo hello > /data/hello.txt && sleep 3600"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: data
```

## How to verify

After creating a cluster, confirm the provisioner Deployment is running:

```sh
kubectl get pods -n local-path-storage
```

Expected output:

```
NAME                                      READY   STATUS    RESTARTS   AGE
local-path-provisioner-...                1/1     Running   0          60s
```

Confirm `local-path` is the default StorageClass (and the only one):

```sh
kubectl get storageclass
```

Expected output:

```
NAME                   PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
local-path (default)   rancher.io/local-path   Delete          WaitForFirstConsumer   false                  2m
```

:::tip
Only one StorageClass should be present. If you see both `local-path` and `standard`, the legacy `installstorage` action did not get gated correctly — run `kinder doctor` to check.
:::

Test a full provision cycle:

```sh
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: smoke-test
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Mi
EOF
kubectl wait --for=jsonpath='{.status.phase}'=Bound pvc/smoke-test --timeout=30s
kubectl delete pvc smoke-test
```

## Configuration

Local Path Provisioner is controlled by the `addons.localPath` field in your cluster config:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  localPath: true  # default
```

See the [Configuration Reference](/configuration) for all available addon fields.

## How to disable

To use the legacy `standard` StorageClass from upstream kind (pre-v1.4 behavior), set `localPath: false`:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  localPath: false
```

When disabled, kinder's `installstorage` action runs and installs the legacy `standard` StorageClass — the exact same behavior as pre-v1.4.

## Multi-node clusters

Local Path Provisioner works in multi-node clusters without additional configuration. Each PVC is bound to a PV on whichever node the consuming pod is scheduled to. Because the `VolumeBindingMode` is `WaitForFirstConsumer`, the provisioner waits until a pod references the PVC before creating the PV on the pod's node.

:::caution[Node affinity]
Because each PV is tied to a specific node's local filesystem, pods using the PVC cannot be rescheduled to a different node. If the node is deleted, the PV data is lost. This is expected for local storage — use a networked storage class for persistence guarantees.
:::

## Air-gapped clusters

The local-path-provisioner images are included in kinder's required addon images list, so they show up in:

- `kinder doctor` offline-readiness check — images are listed under the Local Path Provisioner addon
- `kinder create cluster` (non-air-gapped) addon image warning NOTE
- `kinder create cluster --air-gapped` missing image error message

Two images are required:

| Image | Purpose |
|---|---|
| `docker.io/rancher/local-path-provisioner:v0.0.36` | The provisioner Deployment |
| `docker.io/library/busybox:1.37.0` | The `helperPod` that creates/deletes PV directories |

See the [Working Offline](/guides/working-offline/) guide for the full pre-load workflow.

## Security: CVE-2025-62878

Versions of local-path-provisioner below **v0.0.34** are affected by CVE-2025-62878. kinder v1.4 ships v0.0.35 (above the fix threshold), so fresh clusters are not affected.

`kinder doctor` includes a CVE check that warns when a running cluster has local-path-provisioner below v0.0.34:

```sh
kinder doctor
```

Look for the `local-path-cve` check in the output. Any version strictly less than v0.0.34 triggers a warn (v0.0.34 itself is the fix version and passes).

## Troubleshooting

### PVC stuck in Pending

**Symptom:** `kubectl get pvc` shows `STATUS: Pending` indefinitely.

**Cause:** The `WaitForFirstConsumer` volume binding mode delays provisioning until a pod references the PVC. If no pod is mounting the PVC, it will remain Pending forever.

**Fix:** Create a pod that mounts the PVC. The PV will be provisioned and bound as soon as the pod is scheduled.

### helperPod fails with "ImagePullBackOff"

**Symptom:** `kubectl describe pvc <name>` shows provisioner events with `ImagePullBackOff` on the helperPod.

**Cause:** The cluster cannot pull `docker.io/library/busybox:1.37.0` — typically in an air-gapped environment where the image was not pre-loaded.

**Fix:** Pre-load the image:

```sh
docker pull docker.io/library/busybox:1.37.0
kinder load images docker.io/library/busybox:1.37.0
```

Or recreate the cluster after adding busybox to your pre-load workflow — see [Working Offline](/guides/working-offline/).

### Two default StorageClasses

**Symptom:** `kubectl get storageclass` shows both `local-path (default)` and `standard (default)`.

**Cause:** The `installstorage` gating logic did not engage — this should not happen in a fresh v1.4 cluster, but can occur if an old cluster config was reused.

**Fix:** Recreate the cluster. If the issue persists, file an issue at [github.com/PatrykQuantumNomad/kinder](https://github.com/PatrykQuantumNomad/kinder/issues).
