---
title: Metrics Server
description: Metrics Server addon for resource metrics and autoscaling in kinder clusters.
---

Metrics Server enables `kubectl top` commands and provides the resource metrics API required by the Horizontal Pod Autoscaler (HPA). Without it, `kubectl top` returns an error and HPA cannot function.

kinder installs Metrics Server **v0.8.1**.

## What gets installed

| Resource | Namespace | Purpose |
|---|---|---|
| Metrics Server deployment | `kube-system` | Collects CPU/memory metrics from kubelets |
| `metrics.k8s.io` APIService | cluster-scoped | Exposes the resource metrics API |

Metrics Server is configured with `--kubelet-insecure-tls` to work with the self-signed certificates that kind nodes use. This flag is expected and safe in a local development environment.

## What you get

- **`kubectl top nodes`** — shows CPU and memory usage per node
- **`kubectl top pods`** — shows CPU and memory usage per pod
- **HPA support** — Horizontal Pod Autoscaler can read CPU/memory metrics and scale deployments automatically

## How to verify

```sh
kubectl top nodes
```

Expected output:

```
NAME                 CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%
kind-control-plane   150m         7%     512Mi           13%
```

:::tip[Metrics take 30–60 seconds to appear]
Metrics Server scrapes kubelets on a 15-second interval and needs a few cycles before `kubectl top` returns data. If you see `error: metrics not available yet` immediately after cluster creation, wait 30–60 seconds and try again.
:::

## Configuration

Metrics Server is controlled by the `addons.metricsServer` field:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  metricsServer: true  # default
```

See the [Configuration Reference](/configuration) for all available addon fields.

## How to disable

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  metricsServer: false
```

`kubectl top` will return an error and HPA resources will not be functional when Metrics Server is disabled.

## Practical examples

### View pod resource usage

Show resource usage across all namespaces:

```sh
kubectl top pods -A
```

Expected output:

```
NAMESPACE     NAME                          CPU(cores)   MEMORY(bytes)
kube-system   coredns-787d4945fb-abcde      3m           15Mi
kube-system   metrics-server-7d75f5b5-xyz   5m           20Mi
default       my-app-6d4f8b9c-pqrst         12m          64Mi
```

Show pods in a specific namespace sorted by CPU usage:

```sh
kubectl top pods -n default --sort-by=cpu
```

### Set up a Horizontal Pod Autoscaler

The following example creates a deployment with resource requests and an HPA that scales it based on CPU utilization.

:::caution
The deployment **must** have `resources.requests.cpu` set on its containers. Without it, the HPA cannot calculate CPU utilization and will show `unknown/50%`. Resource requests are what HPA uses as the denominator: `current CPU / requested CPU`.
:::

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: my-app
          image: nginx
          resources:
            requests:
              cpu: "100m"
            limits:
              cpu: "200m"
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-app-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  minReplicas: 1
  maxReplicas: 5
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 50
```

After applying, verify the HPA is reading metrics:

```sh
kubectl get hpa my-app-hpa
```

Expected output (once metrics are available after ~60 seconds):

```
NAME          REFERENCE           TARGETS   MINPODS   MAXPODS   REPLICAS   AGE
my-app-hpa   Deployment/my-app   8%/50%    1         5         1          90s
```

## Troubleshooting

### HPA shows unknown/50%

**Symptom:** `kubectl describe hpa my-app-hpa` shows `<unknown>/50%` for the CPU metric target.

**Cause:** The target deployment's containers do not have `resources.requests.cpu` set. Metrics Server calculates CPU utilization as `current CPU / requested CPU`. Without a request, the denominator is undefined and the metric cannot be calculated.

**Fix:** Add `resources.requests.cpu` to the container spec in the deployment and redeploy:

```yaml
containers:
  - name: my-app
    image: nginx
    resources:
      requests:
        cpu: "100m"
```

After redeploying, wait 60 seconds for Metrics Server to collect a new scrape cycle, then check the HPA again.
