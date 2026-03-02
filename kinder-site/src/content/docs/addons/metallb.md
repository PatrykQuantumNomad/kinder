---
title: MetalLB
description: MetalLB addon for LoadBalancer service support in kinder clusters.
---

MetalLB gives kinder clusters real LoadBalancer IP addresses. Without it, `kubectl` services of type `LoadBalancer` stay in `<pending>` forever. With it, they get an IP from the local Docker or Podman subnet within seconds of creation.

kinder installs MetalLB **v0.15.3**.

## What gets installed

| Resource | Namespace | Purpose |
|---|---|---|
| MetalLB controller | `metallb-system` | Watches services, assigns IPs |
| MetalLB speaker | `metallb-system` | Announces IPs via L2 ARP |
| `IPAddressPool` "kind-pool" | `metallb-system` | Defines the assignable IP range |
| `L2Advertisement` "kind-l2advert" | `metallb-system` | Enables L2 (ARP) advertisement |

### IP address pool

kinder auto-detects the Docker or Podman bridge subnet at cluster creation time and carves out a `.200–.250` range for LoadBalancer use. For example, if the bridge is `172.20.0.0/16`, the pool is `172.20.255.200–172.20.255.250`.

You do not need to configure this range manually.

## How to verify

After creating a cluster, confirm that both MetalLB pods are running:

```sh
kubectl get pods -n metallb-system
```

Expected output:

```
NAME                          READY   STATUS    RESTARTS   AGE
controller-...                1/1     Running   0          60s
speaker-...                   1/1     Running   0          60s
```

Create a test service to confirm IP assignment works:

```sh
kubectl create deployment test --image=nginx
kubectl expose deployment test --port=80 --type=LoadBalancer
kubectl get svc test
```

The `EXTERNAL-IP` column should show an IP in the `.200–.250` range within a few seconds.

## Configuration

MetalLB is controlled by the `addons.metalLB` field in your cluster config:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  metalLB: true  # default
```

See the [Configuration Reference](/configuration) for all available addon fields.

## How to disable

To create a cluster without MetalLB, set `metalLB: false`:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  metalLB: false
```

LoadBalancer services will remain in `<pending>` without a separate load balancer controller.

## Platform notes

:::caution[Rootless Podman limitation]
On **rootless Podman**, the MetalLB L2 speaker cannot send ARP packets because the network namespace lacks the necessary privileges. LoadBalancer services will receive an IP address from the pool, but external traffic routing via ARP will not work.

For rootless Podman environments, use `NodePort` services or a port-forward instead:

```sh
kubectl port-forward svc/<service-name> 8080:80
```
:::
