---
title: Installation
description: How to install kinder by building from source.
---

:::note
kinder is a fork of [kind](https://kind.sigs.k8s.io/) with opinionated addons pre-installed. If you are familiar with kind, most concepts transfer directly.
:::

## Prerequisites

Before installing kinder, make sure you have the following tools installed and available on your `PATH`:

- **Go 1.25.7 or later** — [Install Go](https://go.dev/doc/install)
- **Docker, Podman, or nerdctl** — kinder uses a container runtime to create cluster nodes
- **kubectl** — [Install kubectl](https://kubernetes.io/docs/tasks/tools/)

Verify your Go version:

```sh
go version
# go version go1.25.7 ...
```

## Build from Source

Clone the kinder repository and build with `make install`:

```sh
git clone https://github.com/PatrykQuantumNomad/kinder.git
cd kinder
make install
```

This compiles the `kinder` binary and places it in your `$GOPATH/bin` (or `$GOBIN` if set).

## Verify Installation

Confirm that `kinder` is on your `PATH` and reports the correct version:

```sh
kinder version
```

You should see output like:

```
kinder version: v0.1.0
```

If the command is not found, make sure `$(go env GOPATH)/bin` is included in your `PATH`:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```
