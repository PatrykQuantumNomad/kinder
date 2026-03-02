---
title: Quick Start
description: Create a kinder cluster and verify all addons are working in 5 steps.
---

:::tip
The default cluster name is `kind`. You can choose a different name by passing `--name <name>` to `kinder create cluster`.
:::

## 5 Steps to a Running Cluster

### Step 1: Create a cluster

```sh
kinder create cluster
```

kinder provisions a local Kubernetes node using your container runtime and automatically installs all default addons (MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, Kubernetes Dashboard).

### Step 2: Verify the node is ready

```sh
kubectl get nodes
```

Expected output:

```
NAME                 STATUS   ROLES           AGE   VERSION
kind-control-plane   Ready    control-plane   30s   v1.32.x
```

Wait until `STATUS` is `Ready` before proceeding.

### Step 3: Verify MetalLB is running

```sh
kubectl get pods -n metallb-system
```

All MetalLB pods should be in `Running` state. MetalLB enables `LoadBalancer` type services to receive an external IP on your local machine.

### Step 4: Verify Envoy Gateway is running

```sh
kubectl get pods -n envoy-gateway-system
```

All Envoy Gateway pods should be in `Running` state. Envoy Gateway provides a Gateway API-compatible ingress controller.

### Step 5: Open the Kubernetes Dashboard

```sh
kubectl -n kubernetes-dashboard port-forward svc/kubernetes-dashboard-kong-proxy 8443:443
```

Then open [https://localhost:8443](https://localhost:8443) in your browser.

To generate a login token:

```sh
kubectl -n kubernetes-dashboard create token kubernetes-dashboard
```

Copy the token and use it to log in.

## What You Get

After these 5 steps you have a fully functional local cluster with:

| Addon | Namespace | Purpose |
|-------|-----------|---------|
| MetalLB | `metallb-system` | LoadBalancer IP assignment |
| Envoy Gateway | `envoy-gateway-system` | Gateway API ingress |
| Metrics Server | `kube-system` | `kubectl top` support |
| CoreDNS tuning | `kube-system` | Optimised DNS caching |
| Kubernetes Dashboard | `kubernetes-dashboard` | Web UI for cluster inspection |

To delete the cluster when you are done:

```sh
kinder delete cluster
```
