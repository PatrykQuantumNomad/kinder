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

## Practical examples

### Create a LoadBalancer service with a custom IP

To request a specific IP for a service, use the `metallb.universe.tf/loadBalancerIPs` annotation:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    metallb.universe.tf/loadBalancerIPs: 172.20.255.210
spec:
  selector:
    app: my-app
  ports:
    - port: 80
      targetPort: 8080
  type: LoadBalancer
```

Apply and confirm the IP is assigned:

```sh
kubectl apply -f my-service.yaml
kubectl get svc my-service
```

Expected output:

```
NAME         TYPE           CLUSTER-IP      EXTERNAL-IP      PORT(S)        AGE
my-service   LoadBalancer   10.96.100.200   172.20.255.210   80:31234/TCP   5s
```

:::tip
The requested IP must fall within the `.200–.250` pool range. kinder auto-detects this range from the Docker or Podman bridge subnet. If your bridge is `172.20.0.0/16`, valid IPs are `172.20.255.200` through `172.20.255.210`.
:::

### When to use NodePort instead

There are three scenarios where `NodePort` is preferable to `LoadBalancer`:

1. **Rootless Podman** — ARP announcement requires privileges that rootless Podman does not grant. The service gets an IP but traffic does not route.
2. **macOS or Windows** — LoadBalancer IPs are assigned inside the Docker/Podman VM network and are not directly reachable from the host. NodePort on `localhost` works instead.
3. **Single-service access** — When you only need to reach one service temporarily, `kubectl port-forward` is simpler and does not consume a pool IP.

To expose a deployment with NodePort:

```sh
kubectl expose deployment my-app --type=NodePort --port=80
kubectl get svc my-app
```

Expected output:

```
NAME     TYPE       CLUSTER-IP     EXTERNAL-IP   PORT(S)        AGE
my-app   NodePort   10.96.50.100   <none>        80:32456/TCP   3s
```

Access the service via `localhost` using the assigned node port:

```sh
curl http://localhost:32456
```

## Troubleshooting

### Service stuck in pending

**Symptom:** `kubectl get svc` shows `<pending>` in the `EXTERNAL-IP` column even after waiting several seconds.

**Cause:** Either MetalLB pods are not running, or the IPAddressPool is exhausted. The default pool supports up to 51 addresses (`.200–.250`), so you can have at most 51 LoadBalancer services active at once.

**Fix:**

Check that MetalLB pods are running:

```sh
kubectl get pods -n metallb-system
```

Both `controller` and `speaker` pods must show `Running`. If either is in `Pending` or `CrashLoopBackOff`, describe the pod for details.

Check whether the pool has available IPs:

```sh
kubectl get ipaddresspool -n metallb-system -o yaml
```

If `addresses` in the status are all allocated, delete unused LoadBalancer services to free IPs.

## Platform notes

:::caution[Rootless Podman limitation]
On **rootless Podman**, the MetalLB L2 speaker cannot send ARP packets because the network namespace lacks the necessary privileges. LoadBalancer services will receive an IP address from the pool, but external traffic routing via ARP will not work.

For rootless Podman environments, use `NodePort` services or a port-forward instead:

```sh
kubectl port-forward svc/<service-name> 8080:80
```
:::
