---
title: Quick Start
description: Create a kinder cluster, see what you get, and verify every addon is working.
---

:::tip
The default cluster name is `kind`. Pick a different name with `--name <name>` on `kinder create cluster`. Lifecycle commands (`pause`, `resume`, `status`, `delete cluster`, `get nodes`, `snapshot`) accept the cluster name as a **positional argument** — for example `kinder delete cluster my-cluster`.
:::

## Create a Cluster

```sh
kinder create cluster
```

You should see output like:

```
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.35.1) 🖼
 ✓ Preparing nodes 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
 ✓ Installing MetalLB
 ✓ Installing Metrics Server
 ✓ Tuning CoreDNS
 ✓ Installing Local Path Provisioner
 ✓ Installing Envoy Gateway
 ✓ Installing Dashboard
 ✓ Installing Local Registry
 ✓ Installing cert-manager

Addons:
 * MetalLB                 installed
 * Metrics Server          installed
 * CoreDNS Tuning          installed
 * Local Path Provisioner  installed
 * Envoy Gateway           installed
 * Dashboard               installed
 * Local Registry          installed
 * cert-manager            installed
Set kubectl context to "kind-kind"
You can now use your cluster with:

kubectl cluster-info --context kind-kind
```

kinder also prints a dashboard token and port-forward command — save these for later.

:::tip[Addon profiles]
`kinder create cluster` enables all 8 addons by default. Use `--profile` to select a preset:

- `--profile minimal` — no kinder addons (plain kind cluster)
- `--profile gateway` — MetalLB + Envoy Gateway only
- `--profile ci` — Metrics Server + cert-manager (CI-optimized)
- `--profile full` — all addons (same as default)
:::

:::tip[Offline / air-gapped]
Add `--air-gapped` to create a cluster without any network calls for image pulls:

```sh
kinder create cluster --air-gapped
```

kinder fails fast with a complete list of missing images instead of hanging on failed pulls. See the [Working Offline](/guides/working-offline/) guide for the pre-load workflow.
:::

## What You Get

| Addon | Namespace | Purpose |
|-------|-----------|---------|
| MetalLB | `metallb-system` | LoadBalancer IP assignment |
| Envoy Gateway | `envoy-gateway-system` | Gateway API ingress |
| Metrics Server | `kube-system` | `kubectl top` support |
| CoreDNS tuning | `kube-system` | Optimised DNS caching |
| Local Path Provisioner | `local-path-storage` | Automatic dynamic PVC provisioning with `local-path` as default StorageClass |
| Headlamp | `kube-system` | Web UI for cluster inspection |
| Local Registry | `default` (host network) | Private container registry at localhost:5001 |
| cert-manager | `cert-manager` | Automatic TLS certificates with self-signed ClusterIssuer |

## Verify Core Addons

### MetalLB

Deploy a LoadBalancer service and confirm it gets an external IP:

```sh
kubectl create deployment nginx --image=nginx
kubectl expose deployment nginx --type=LoadBalancer --port=80
kubectl get svc nginx
```

You should see a real IP under `EXTERNAL-IP`:

```
NAME    TYPE           CLUSTER-IP     EXTERNAL-IP      PORT(S)        AGE
nginx   LoadBalancer   10.96.150.34   172.19.255.200   80:30693/TCP   7s
```

:::note[macOS / Windows]
On macOS and Windows, LoadBalancer IPs are not directly reachable from the host. Use port-forward to access the service:

```sh
kubectl port-forward svc/nginx 8081:80
# Open http://localhost:8081
```
:::

Clean up:

```sh
kubectl delete svc nginx && kubectl delete deployment nginx
```

### Metrics Server

Check that `kubectl top` returns real CPU and memory data:

```sh
kubectl top nodes
```

Expected output:

```
NAME                 CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
kind-control-plane   168m         0%       1385Mi          8%
```

:::tip
Metrics may take a minute to populate after cluster creation.
:::

### CoreDNS Tuning

Verify that `autopath` and `cache 60` are present in the Corefile:

```sh
kubectl get configmap coredns -n kube-system -o jsonpath='{.data.Corefile}'
```

Look for these entries in the output:

```
autopath @kubernetes
```

```
cache 60 {
    disable success cluster.local
    disable denial cluster.local
}
```

## Verify Optional Addons

### Envoy Gateway

Confirm the GatewayClass is accepted:

```sh
kubectl get gatewayclass eg
```

Expected output:

```
NAME   CONTROLLER                                      ACCEPTED   AGE
eg     gateway.envoyproxy.io/gatewayclass-controller   True       9m
```

### Headlamp Dashboard

Port-forward to the dashboard:

```sh
kubectl port-forward -n kube-system service/headlamp 8080:80
```

Open [http://localhost:8080](http://localhost:8080) and paste the token that was printed during cluster creation.

If you need to retrieve the token later:

```sh
kubectl get secret kinder-dashboard-token -n kube-system \
  -o jsonpath='{.data.token}' | base64 -d
```

### Local Registry

Confirm the registry container is running and accessible:

```sh
docker ps --filter name=kind-registry
```

Expected output:

```
CONTAINER ID   IMAGE        COMMAND                  PORTS                    NAMES
abc123def456   registry:2   "/entrypoint.sh /etc…"   0.0.0.0:5001->5000/tcp   kind-registry
```

Verify dev tool discovery ConfigMap is present:

```sh
kubectl get configmap local-registry-hosting -n kube-public
```

Expected output:

```
NAME                     DATA   AGE
local-registry-hosting   1      60s
```

### cert-manager

Check all three cert-manager components are running:

```sh
kubectl get pods -n cert-manager
```

Expected output:

```
NAME                                       READY   STATUS    RESTARTS   AGE
cert-manager-...                           1/1     Running   0          60s
cert-manager-cainjector-...                1/1     Running   0          60s
cert-manager-webhook-...                   1/1     Running   0          60s
```

Confirm the self-signed ClusterIssuer is ready:

```sh
kubectl get clusterissuer selfsigned-issuer
```

Expected output:

```
NAME                READY   AGE
selfsigned-issuer   True    60s
```

## Loading local images

To load a locally-built image into every node of the cluster, use `kinder load images`:

```sh
docker build -t myapp:dev .
kinder load images myapp:dev
```

This works with all three providers (docker, podman, nerdctl) and skips re-importing if the image is already present on every node. See the [Load Images CLI reference](/cli-reference/load-images/) for full details.

## Pause, resume, snapshot

Once your cluster is up, kinder gives you a small set of lifecycle verbs for daily iteration:

```sh
# Free CPU/RAM without losing state — pods, PVCs, services all survive
kinder pause my-cluster
kinder resume my-cluster

# Capture a complete snapshot (etcd + images + PV contents) for instant reset
kinder snapshot create my-cluster baseline
kinder snapshot list my-cluster
kinder snapshot restore my-cluster baseline

# See cluster + per-node container state
kinder status my-cluster
```

Snapshots refuse to restore across mismatched Kubernetes versions, topologies, or addon versions, so the captured state is always consistent. See the changelog [v1.5 entry](/changelog/#v15--inner-loop) for the full surface.

## Hot-reload your app with `kinder dev`

Skip the manual build → push → rollout loop:

```sh
kinder dev --watch ./src --target myapp
```

`kinder dev` watches the directory, builds a Docker image on every change, loads it into every node via the `kinder load images` pipeline, and rolls the target Deployment automatically — printing per-step timing per cycle. Use `--poll` on Docker Desktop for macOS where fsnotify events are unreliable. Pair with the [Local Dev Workflow guide](/guides/local-dev-workflow/) for the full inner-loop pattern.

## Something wrong?

Run `kinder doctor` to check prerequisites and identify issues:

```sh
kinder doctor
```

This checks that Docker (or Podman/nerdctl), kubectl, and other dependencies are installed and reachable.

For cryptic errors that surface AFTER cluster creation (kubelet stuck, addon won't roll out, image pull mysteriously failing), `kinder doctor decode` scans recent docker logs and `kubectl get events` against a catalog of known patterns and prints plain-English explanations with suggested fixes:

```sh
kinder doctor decode --since 30m
```

See the [Troubleshooting reference](/cli-reference/troubleshooting/#kinder-doctor-decode) for the full catalog and `--auto-fix` whitelist.

## Delete the Cluster

When you are done:

```sh
kinder delete cluster                # deletes the default "kind" cluster
kinder delete cluster my-cluster     # delete by positional name
kinder delete cluster --name my-cluster   # or by --name flag
```
