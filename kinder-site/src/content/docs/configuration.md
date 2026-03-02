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

## Using a Config File

Pass a configuration file to `kinder create cluster` with the `--config` flag:

```sh
kinder create cluster --config cluster.yaml
```

## Addon Fields

The `addons` section controls which addons are installed when the cluster is created. All addons are enabled by default; set a field to `false` to skip installation.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `metalLB` | `bool` | `true` | Install [MetalLB](https://metallb.universe.tf/) for LoadBalancer IP assignment |
| `envoyGateway` | `bool` | `true` | Install [Envoy Gateway](https://gateway.envoyproxy.io/) for Gateway API ingress |
| `metricsServer` | `bool` | `true` | Install [Metrics Server](https://github.com/kubernetes-sigs/metrics-server) for `kubectl top` support |
| `coreDNSTuning` | `bool` | `true` | Apply CoreDNS tuning for optimised local DNS caching |
| `dashboard` | `bool` | `true` | Install [Kubernetes Dashboard](https://github.com/kubernetes/dashboard) web UI |

## Disabling Addons

To create a cluster without specific addons, set the corresponding fields to `false`:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  dashboard: false
  envoyGateway: false
```

This creates a cluster with MetalLB, Metrics Server, and CoreDNS tuning, but skips the Dashboard and Envoy Gateway.

To skip all addons and get a plain kind-equivalent cluster:

```yaml
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  metalLB: false
  envoyGateway: false
  metricsServer: false
  coreDNSTuning: false
  dashboard: false
```

## Compatibility with kind

Because kinder uses `kind.x-k8s.io/v1alpha4` as its API version, existing kind configuration files work without modification. The `addons` section is ignored by kind and is only processed by kinder.
