---
title: Quick Start
description: Create a kinder cluster, see what you get, and verify every addon is working.
---

:::tip
The default cluster name is `kind`. You can choose a different name by passing `--name <name>` to `kinder create cluster`.
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
 ✓ Installing Envoy Gateway
 ✓ Installing Dashboard

Addons:
 * MetalLB              installed
 * Metrics Server       installed
 * CoreDNS Tuning       installed
 * Envoy Gateway        installed
 * Dashboard            installed
Set kubectl context to "kind-kind"
You can now use your cluster with:

kubectl cluster-info --context kind-kind
```

kinder also prints a dashboard token and port-forward command — save these for later.

## What You Get

| Addon | Namespace | Purpose |
|-------|-----------|---------|
| MetalLB | `metallb-system` | LoadBalancer IP assignment |
| Envoy Gateway | `envoy-gateway-system` | Gateway API ingress |
| Metrics Server | `kube-system` | `kubectl top` support |
| CoreDNS tuning | `kube-system` | Optimised DNS caching |
| Headlamp | `kube-system` | Web UI for cluster inspection |

## Verify Addons

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

## Delete the Cluster

When you are done:

```sh
kinder delete cluster
```
