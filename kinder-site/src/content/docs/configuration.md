---
title: Configuration Reference
description: Complete reference for kinder cluster configuration including all addon fields.
---

:::note
kinder accepts the same cluster configuration format as kind. Any option supported by `kind` in a `kind.x-k8s.io/v1alpha4` config file also works with `kinder`. This page documents the kinder-specific `addons` extension.
:::

## Cluster Configuration

kinder uses the same `kind.x-k8s.io/v1alpha4` API version as kind. A minimal configuration file looks like:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
```

## Complete Configuration Example

A complete kinder configuration showing all addon fields:

```yaml
# Full kinder v1alpha4 configuration -- all fields shown
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  # Core addons (always useful -- enabled by default)
  metalLB: true
  metricsServer: true
  coreDNSTuning: true
  localPath: true
  # Optional addons (enabled by default -- disable for lightweight clusters)
  envoyGateway: true
  dashboard: true
  localRegistry: true
  certManager: true
```

:::note[All addons enabled by default]
All 8 addons are installed by default. The **core** group (MetalLB, Metrics Server, CoreDNS, Local Path Provisioner) is always useful for any workflow. The **optional** group (Envoy Gateway, Headlamp, Local Registry, cert-manager) is also enabled by default but commonly disabled for lightweight or CI clusters using `--profile minimal` or `--profile ci`.
:::

## Using a Config File

Pass a configuration file to `kinder create cluster` with the `--config` flag:

```sh
kinder create cluster --config cluster.yaml
```

## Addon Fields

### Core Addons

These addons are always useful regardless of your workload.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `metalLB` | `bool` | `true` | Install [MetalLB](https://metallb.universe.tf/) for LoadBalancer IP assignment |
| `metricsServer` | `bool` | `true` | Install [Metrics Server](https://github.com/kubernetes-sigs/metrics-server) for `kubectl top` support |
| `coreDNSTuning` | `bool` | `true` | Apply CoreDNS tuning for optimised local DNS caching |
| `localPath` | `bool` | `true` | Install [local-path-provisioner](/addons/local-path-provisioner/) — `local-path` as the default StorageClass. Set to `false` to restore the legacy `standard` StorageClass |

### Optional Addons

These addons are powerful but commonly disabled for minimal or CI clusters.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `envoyGateway` | `bool` | `true` | Install [Envoy Gateway](https://gateway.envoyproxy.io/) for Gateway API ingress |
| `dashboard` | `bool` | `true` | Install [Headlamp](https://headlamp.dev/) web dashboard |
| `localRegistry` | `bool` | `true` | Run a private container registry at `localhost:5001` with dev tool auto-discovery |
| `certManager` | `bool` | `true` | Install [cert-manager](https://cert-manager.io/) with a self-signed ClusterIssuer |

## Disabling Addons

To create a cluster without specific addons, set the corresponding fields to `false`:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  dashboard: false
  envoyGateway: false
```

This creates a cluster with MetalLB, Metrics Server, CoreDNS tuning, Local Registry, and cert-manager, but skips the Dashboard and Envoy Gateway.

To skip all addons and get a plain kind-equivalent cluster:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  metalLB: false
  envoyGateway: false
  metricsServer: false
  coreDNSTuning: false
  localPath: false
  dashboard: false
  localRegistry: false
  certManager: false
```

## Compatibility with kind

Because kinder uses `kind.x-k8s.io/v1alpha4` as its API version, existing kind configuration files work without modification. The `addons` section is ignored by kind and is only processed by kinder.
