---
title: Multi-Version Clusters for Upgrade Testing
description: Configure per-node Kubernetes versions, test version-skew validation, and simulate upgrade scenarios with kinder's multi-version cluster support.
---

In this tutorial you will create a kinder cluster where the control-plane and workers run **different Kubernetes versions** — a common setup for testing kubeadm upgrade paths, validating version-skew behavior, or reproducing user-reported bugs against a specific version combination. You will see kinder's config-time version-skew validator in action, explore the `kinder get nodes` extended output, and use `kinder doctor` to detect skew violations on a running cluster.

## Prerequisites

- [kinder installed](/installation)
- Docker (or Podman) installed and running
- `kubectl` installed and on PATH
- Familiarity with the Kubernetes [version skew policy](https://kubernetes.io/releases/version-skew-policy/) — workers can be up to 3 minor versions behind the control-plane, HA control-plane nodes must share the same minor version

## Step 1: Create a valid multi-version cluster

Save the following as `multi-version.yaml`. This creates a cluster with a `v1.35.1` control-plane and two workers: one at `v1.34.2` (1 minor behind) and one at `v1.32.5` (3 minors behind, the maximum allowed):

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    image: kindest/node:v1.35.1
  - role: worker
    image: kindest/node:v1.34.2
  - role: worker
    image: kindest/node:v1.32.5
```

:::note
The `image:` field on each node is a **per-node override**. kinder's v1alpha4 config extends kind's standard schema — simply set `image:` on any node to run a specific version.
:::

Create the cluster:

```sh
kinder create cluster --config multi-version.yaml
```

Expected output:

```
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.35.1) 🖼
 ✓ Ensuring node image (kindest/node:v1.34.2) 🖼
 ✓ Ensuring node image (kindest/node:v1.32.5) 🖼
 ✓ Preparing nodes 📦 📦 📦
 ✓ Writing configuration 📜
 ✓ Starting control-plane 🕹️
 ✓ Installing CNI 🔌
 ✓ Installing StorageClass 💾
 ✓ Joining worker nodes 🚜
 ✓ Installing addons
Set kubectl context to "kind-kind"
```

kinder pulled three different node images and joined each worker to the control-plane at its own version.

## Step 2: Inspect per-node versions with `kinder get nodes`

kinder's `get nodes` command shows the Kubernetes version and image of every node, along with a SKEW column indicating how each worker compares to the control-plane:

```sh
kinder get nodes
```

Expected output:

```
NAME                 ROLE           STATUS   VERSION   IMAGE                        SKEW
kind-control-plane   control-plane  Ready    v1.35.1   kindest/node:v1.35.1         ✓
kind-worker          worker         Ready    v1.34.2   kindest/node:v1.34.2         ✗ (-1)
kind-worker2         worker         Ready    v1.32.5   kindest/node:v1.32.5         ✗ (-3)
```

- **`✓`** means the node's minor matches the control-plane exactly
- **`✗ (-N)`** means the node is N minor versions behind
- **`✗ (+N)`** means the node is ahead of the control-plane — **not allowed by the kubeadm version skew policy**

Both workers here are within the 3-minor policy window, so although the SKEW column flags them as non-matching, the cluster is valid.

The same data is available as JSON for scripting:

```sh
kinder get nodes --output json
```

## Step 3: Test the config-time version-skew validator

kinder validates version skew **before any containers are created**, so invalid configs fail instantly without waiting for kubeadm errors deep into provisioning.

Save the following as `invalid-skew.yaml` — a worker 4 minors behind the control-plane (one beyond the policy limit):

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    image: kindest/node:v1.35.1
  - role: worker
    image: kindest/node:v1.31.0
```

Try to create it:

```sh
kinder create cluster --config invalid-skew.yaml --name skew-test
```

Expected output:

```
ERROR: invalid config: node version-skew policy violated:

  NODE          VERSION   CP VERSION   DELTA   ISSUE
  kind-worker   v1.31.0   v1.35.1      -4      workers cannot be more than 3 minor versions behind the control-plane

Fix: bump the worker image to kindest/node:v1.32.0 or newer, or downgrade the control-plane.
```

No containers were created. The failure is clean — you can fix the config and retry without having to `kinder delete cluster`.

### HA control-plane skew is also rejected

HA control-plane nodes must all run the same minor version (etcd consistency requirement). Save `invalid-ha.yaml`:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    image: kindest/node:v1.35.1
  - role: control-plane
    image: kindest/node:v1.34.2
  - role: control-plane
    image: kindest/node:v1.35.1
```

Try to create it:

```sh
kinder create cluster --config invalid-ha.yaml --name ha-test
```

Expected output:

```
ERROR: invalid config: node version-skew policy violated:

  NODE                  VERSION   CP VERSION   DELTA   ISSUE
  kind-control-plane2   v1.34.2   v1.35.1      -1      HA control-plane nodes must all run the same minor version

Fix: align all control-plane images to the same minor version.
```

:::note[Non-semver tags skip validation]
Tags like `latest`, `dev`, or `feature-branch-sha` are not semver-parseable. When any node image uses a non-semver tag, the version-skew validator **skips the entire check** rather than erroring. This preserves backward compatibility for test and dev configs that use rolling tags.
:::

## Step 4: The `--image` flag now respects per-node images

Before kinder v1.4, passing a global `--image` flag to `kinder create cluster` would override per-node images in the config, silently discarding the multi-version intent. This was a common footgun when scripting cluster creation.

kinder v1.4+ introduces an **`ExplicitImage` sentinel**: when you set `image:` on a node in config, that node is marked as explicitly versioned, and the `--image` flag only overrides nodes **without** an explicit image.

Try it with the original `multi-version.yaml`:

```sh
kinder delete cluster
kinder create cluster \
  --config multi-version.yaml \
  --image kindest/node:v1.30.0
```

Expected output: the cluster is created with the **original per-node images** (v1.35.1, v1.34.2, v1.32.5). The `--image` flag is ignored because every node has an explicit image.

Verify:

```sh
kinder get nodes
```

Expected output (same as Step 2 — `--image` had no effect):

```
NAME                 ROLE           STATUS   VERSION   IMAGE                        SKEW
kind-control-plane   control-plane  Ready    v1.35.1   kindest/node:v1.35.1         ✓
kind-worker          worker         Ready    v1.34.2   kindest/node:v1.34.2         ✗ (-1)
kind-worker2         worker         Ready    v1.32.5   kindest/node:v1.32.5         ✗ (-3)
```

### When `--image` **does** apply

If you create a cluster from a config where nodes have no `image:` field, or from no config at all, `--image` applies as before. Save `no-explicit.yaml`:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
  - role: worker
```

```sh
kinder delete cluster
kinder create cluster --config no-explicit.yaml --image kindest/node:v1.34.2
kinder get nodes
```

Expected output:

```
NAME                 ROLE           STATUS   VERSION   IMAGE                        SKEW
kind-control-plane   control-plane  Ready    v1.34.2   kindest/node:v1.34.2         ✓
kind-worker          worker         Ready    v1.34.2   kindest/node:v1.34.2         ✓
kind-worker2         worker         Ready    v1.34.2   kindest/node:v1.34.2         ✓
```

All three nodes use the flag-supplied image because none had an explicit override.

## Step 5: Detect skew violations with `kinder doctor`

`kinder doctor` includes a `cluster-node-skew` check that inspects the running cluster and warns about:

- **HA control-plane minor mismatches** — e.g., one control-plane at v1.35 and another at v1.34
- **Workers more than 3 minors behind** the control-plane
- **Workers ahead of the control-plane** (any amount)
- **Config drift** — when the node's declared image tag differs from the Kubernetes version reported by `/kind/version` inside the container (for example, after a manual in-place upgrade)

Recreate the valid multi-version cluster:

```sh
kinder delete cluster
kinder create cluster --config multi-version.yaml
kinder doctor
```

Look for the `cluster-node-skew` check in the output. With the valid cluster it will report `ok`:

```
[Cluster]
  ✓ cluster-node-skew           all nodes within version skew policy
```

With a cluster that has a skew violation the check reports `warn` with a tabwriter table of offending nodes:

```
[Cluster]
  ⚠ cluster-node-skew
                           NODE                 VERSION   CP VERSION   ISSUE
                           kind-worker2         v1.31.0   v1.35.1      -4 minors (exceeds policy)
```

:::tip
`kinder doctor` version-skew checks run as part of the standard doctor suite. You don't need any flags — just run `kinder doctor` after creating a cluster to catch drift or misconfiguration.
:::

## Use cases

### Testing kubeadm upgrades

Create a cluster at `v1.34.2` and then simulate an upgrade to `v1.35.1` by rebuilding just the control-plane node with a newer image. The skew window (3 minor versions) gives you room to stage upgrades one control-plane at a time before rolling workers forward.

### Reproducing version-specific bugs

When a user reports an issue against a specific version combination, you can reproduce it locally in seconds by updating the `image:` fields in your config and creating the cluster. No need to set up real clusters or spin up cloud resources.

### Validating client compatibility

Run your kubectl plugins, operators, or controllers against a matrix of Kubernetes versions without maintaining separate clusters. Create two multi-version clusters in parallel with `--name` and switch kubectl contexts between them.

```sh
kinder create cluster --config v1.32-cluster.yaml --name legacy
kinder create cluster --config v1.35-cluster.yaml --name current
kubectl config use-context kind-legacy   # run tests
kubectl config use-context kind-current  # run tests
```

## Clean up

```sh
kinder delete cluster
kinder delete cluster --name skew-test 2>/dev/null || true
kinder delete cluster --name ha-test 2>/dev/null || true
```

No other cleanup required — the invalid-config clusters were never created, so there's nothing to tear down for those attempts.

## See also

- [Configuration Reference](/configuration) — full v1alpha4 schema including per-node `image:` override
- [CLI Troubleshooting](/cli-reference/troubleshooting/) — common issues with `kinder doctor` checks
- Kubernetes [version skew policy](https://kubernetes.io/releases/version-skew-policy/) — the official policy that kinder enforces at config-parse time
