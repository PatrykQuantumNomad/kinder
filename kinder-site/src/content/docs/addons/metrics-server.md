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
