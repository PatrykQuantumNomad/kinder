---
title: "Working Offline"
menu:
  main:
    parent: "user"
    identifier: "working-offline"
    weight: 3
description: |-
  This guide covers how to work with KIND in an offline / airgapped environment.

  You should first [install kind][installation documentation] before continuing.

  [installation documentation]: https://kind.sigs.k8s.io/docs/user/quick-start#installation
---
## Using a pre-built [node image][node image]

KIND provides some pre-built images,
these images contain everything necessary to create a cluster and can be used in an offline environment.

You can find available image tags on the [releases page][releases page].
Please include the `@sha256:` [image digest][image digest] from the image in the release notes.

You can pull it when you have network access,
or pull it on another machine and then transfer it to the target machine.

```
➜  ~ docker pull kindest/node:v1.17.0@sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62
sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62: Pulling from kindest/node
cc5a81c29aab: Pull complete 
81c62728355f: Pull complete 
ed9cffdd962a: Pull complete 
6a46f000fce2: Pull complete 
6bd890da28be: Pull complete 
0d88bd219ffe: Pull complete 
af5240f230f0: Pull complete 
Digest: sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62
Status: Downloaded newer image for kindest/node@sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62
docker.io/kindest/node:v1.17.0@sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62
```

You can [save node image][docker save] to a tarball.

```
➜  ~ docker save -o kind.v1.17.0.tar kindest/node:v1.17.0@sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62
# or
➜  ~ docker save kindest/node:v1.17.0@sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62 | gzip > kind.v1.17.0.tar.gz
```

When you transport image tarball to the machine,
you can load the node image by [`docker load`][docker load] command.

```
➜  ~ docker load -i kind.v1.17.0.tar
Loaded image ID: sha256:ec6ab22d89efc045f4da4fc862f6a13c64c0670fa7656fbecdec5307380f9cb0
# or
➜  ~ docker load -i kind.v1.17.0.tar.gz
Loaded image ID: sha256:ec6ab22d89efc045f4da4fc862f6a13c64c0670fa7656fbecdec5307380f9cb0
```

And [create a tag][docker tag] for it.

```
➜  ~ docker image tag kindest/node:v1.17.0@sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62 kindest/node:v1.17.0
➜  ~ docker image ls kindest/node
REPOSITORY          TAG                 IMAGE ID            CREATED             SIZE
kindest/node        v1.17.0             ec6ab22d89ef        3 weeks ago         1.23GB
```

Finally, you can create a cluster by specifying the `--image` flag.

```
➜  ~ kind create cluster --image kindest/node:v1.17.0
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.17.0) 🖼
 ✓ Preparing nodes 📦  
 ✓ Writing configuration 📜 
 ✓ Starting control-plane 🕹️ 
 ✓ Installing CNI 🔌 
 ✓ Installing StorageClass 💾 
Set kubectl context to "kind-kind"
You can now use your cluster with:

kubectl cluster-info --context kind-kind

Have a question, bug, or feature request? Let us know! https://kind.sigs.k8s.io/#community 🙂
```

## Building the [node image][node image]

In addition to using pre-built node image, 
KIND also provides the ability to build [node image][node image] from Kubernetes source code.

Please note that during the image building process, you need to download many dependencies.
It is recommended that you build at least once online to ensure that these dependencies are downloaded to your local.
See [building the node image][building the node image] for more detail.

The node-image in turn is built off the [base image][base image].

### Prepare Kubernetes source code

You can clone Kubernetes source code.

```sh
➜  ~ mkdir -p $GOPATH/src/k8s.io
➜  ~ cd $GOPATH/src/k8s.io
➜  ~ git clone https://github.com/kubernetes/kubernetes
```

### Building image

```sh
➜  ~ kind build node-image --image kindest/node:main $GOPATH/src/k8s.io/kubernetes 
Starting to build Kubernetes
...
Image build completed.
```

When the image build is complete, you can create a cluster by passing the `--image` flag.

```sh
➜  ~ kind create cluster --image kindest/node:main
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:main) 🖼
 ✓ Preparing nodes 📦  
 ✓ Writing configuration 📜 
 ✓ Starting control-plane 🕹️ 
 ✓ Installing CNI 🔌 
 ✓ Installing StorageClass 💾 
Set kubectl context to "kind-kind"
You can now use your cluster with:

kubectl cluster-info --context kind-kind

Have a question, bug, or feature request? Let us know! https://kind.sigs.k8s.io/#community 🙂
```

## HA cluster

If you want to create a control-plane HA cluster
then you need to create a config file and use this file to start the cluster.

```sh
➜  ~ cat << EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
# 3 control plane node and 1 workers
nodes:
- role: control-plane
- role: control-plane
- role: control-plane
- role: worker
EOF
```

Note that in an offline environment, in addition to preparing the node image,
you also need to prepare HAProxy image in advance.

You can find the specific tag currently in use at [loadbalancer source code][loadbalancer source code].

## Addon images

In addition to the node image and (for HA clusters) the HAProxy load balancer
image, kinder installs several addon images during cluster creation. In an
offline environment, these images must be available locally before you create
the cluster.

### Check which images you need

Run `kinder doctor` to see which addon images are missing:

```
kinder doctor
```

The **offline-readiness** check lists every addon image and whether it is
present locally. Images are labeled by addon, so you can skip images for
addons you plan to disable.

Alternatively, create a cluster without `--air-gapped` first — kinder prints
a NOTE listing every addon image that will be pulled.

### Pre-load addon images

On a machine with internet access, pull and save the images you need:

```sh
# Example: save MetalLB images
docker pull quay.io/metallb/controller:v0.15.3
docker pull quay.io/metallb/speaker:v0.15.3
docker save quay.io/metallb/controller:v0.15.3 quay.io/metallb/speaker:v0.15.3 | gzip > metallb.tar.gz
```

Transfer the tarball to your offline machine and load:

```sh
docker load < metallb.tar.gz
```

Repeat for every addon you intend to enable.

### Create the cluster in air-gapped mode

Once all images are pre-loaded:

```sh
kinder create cluster --air-gapped
```

If any required image is missing, kinder exits immediately with a complete
list of missing images — no partial creation, no hung pulls.

### Two-mode offline workflow

There are two approaches to working offline with kinder:

**Mode 1: Pre-create image baking (recommended)**

Pre-load all required images onto the host before creating the cluster.
This is the simplest approach and works with `--air-gapped`:

1. `kinder doctor` — check which images are missing
2. `docker pull` + `docker save` — pull and export on a connected machine
3. `docker load` — import on the air-gapped machine
4. `kinder create cluster --air-gapped` — create the cluster

**Mode 2: Post-create image loading**

Create the cluster first (with internet access or with `--air-gapped` for
node images only), then load additional images into the running nodes:

1. `kinder create cluster` — create with at least the node image available
2. `kinder load docker-image <image> [<image>...]` — load images into all cluster nodes

This mode is useful when you need to add images after the cluster is
already running, for example to test a locally-built application image.

> **Note:** `kinder load docker-image` currently requires Docker on the host
> side (it uses `docker save` internally). Podman and nerdctl users should
> use Mode 1.




[installation documentation]: https://kind.sigs.k8s.io/docs/user/quick-start#installation
[node image]: https://kind.sigs.k8s.io/docs/design/node-image
[releases page]: https://github.com/kubernetes-sigs/kind/releases
[image digest]: https://docs.docker.com/engine/reference/commandline/pull/#pull-an-image-by-digest-immutable-identifier
[docker save]: https://docs.docker.com/engine/reference/commandline/save/
[docker load]: https://docs.docker.com/engine/reference/commandline/load/
[docker tag]: https://docs.docker.com/engine/reference/commandline/tag/
[base image]: https://kind.sigs.k8s.io/docs/design/base-image/
[building the node image]: https://kind.sigs.k8s.io/docs/user/quick-start/#building-images
[loadbalancer source code]: https://github.com/kubernetes-sigs/kind/blob/main/pkg/cluster/internal/loadbalancer/const.go#L20
