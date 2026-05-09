---
title: Profile Comparison
description: "Compare kinder's --profile presets — minimal, full, gateway, and ci — and pick the right addon set for your local Kubernetes development workflow."
---

The `--profile` flag is a shortcut for preset addon configurations; see the [Configuration Reference](/configuration) for manual control of individual addons.

## Presets at a glance

| Addon | minimal | full | gateway | ci |
|-------|:-------:|:----:|:-------:|:--:|
| MetalLB | - | ✓ | ✓ | - |
| Envoy Gateway | - | ✓ | ✓ | - |
| Metrics Server | - | ✓ | - | ✓ |
| CoreDNS Tuning | - | ✓ | - | - |
| Headlamp | - | ✓ | - | - |
| Local Registry | - | ✓ | - | - |
| cert-manager | - | ✓ | - | ✓ |

## Preset details

### minimal

Starts a plain kind cluster with no addons installed.

:::tip[When to use]
Use `minimal` when you want to manage addons manually or are testing kind compatibility without any kinder extensions.
:::

```sh
kinder create cluster --profile minimal
```

### full

Enables all 7 addons: MetalLB, Envoy Gateway, Metrics Server, CoreDNS Tuning, Headlamp, Local Registry, and cert-manager.

:::tip[When to use]
Use `full` for demos, exploration, or when you want every kinder feature available without deciding which addons to enable.
:::

```sh
kinder create cluster --profile full
```

### gateway

Enables MetalLB and Envoy Gateway for Gateway API routing with real LoadBalancer IPs.

:::tip[When to use]
Use `gateway` when you are working on Gateway API routing and need real LoadBalancer IP allocation but do not need the full addon suite.
:::

```sh
kinder create cluster --profile gateway
```

### ci

Enables Metrics Server and cert-manager for HPA scaling and TLS certificate management.

:::tip[When to use]
Use `ci` in CI pipelines that need HPA scaling or TLS support with minimal install time and no dashboard or registry overhead.
:::

```sh
kinder create cluster --profile ci
```

## How to use a profile

Pass `--profile` to `kinder create cluster`:

```sh
kinder create cluster --profile gateway
kinder create cluster --name my-cluster --profile ci
```

## Profiles and custom configuration

:::caution
Do not combine `--profile` and `--config` in the same command. The `--profile` flag sets the entire addons block; combining it with a `--config` file that also sets addons produces unpredictable results.

- Use `--profile` for quick presets.
- Use `--config` with a YAML file for granular control. See the [Configuration Reference](/configuration).
:::
