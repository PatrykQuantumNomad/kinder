---
title: Installation
description: How to install kinder from a pre-built binary or by building from source.
---

:::note
kinder is a fork of [kind](https://kind.sigs.k8s.io/) with opinionated addons pre-installed. If you are familiar with kind, most concepts transfer directly.
:::

## Prerequisites

- **Docker, Podman, or nerdctl** — kinder uses a container runtime to create cluster nodes
- **kubectl** — [Install kubectl](https://kubernetes.io/docs/tasks/tools/)

## Download Pre-built Binary

Pre-built binaries are available for macOS, Linux, and Windows from [GitHub Releases](https://github.com/PatrykQuantumNomad/kinder/releases).

### macOS (Apple Silicon)

```sh
curl -Lo kinder https://github.com/PatrykQuantumNomad/kinder/releases/latest/download/kinder-darwin-arm64
chmod +x kinder
sudo mv kinder /usr/local/bin/
```

### macOS (Intel)

```sh
curl -Lo kinder https://github.com/PatrykQuantumNomad/kinder/releases/latest/download/kinder-darwin-amd64
chmod +x kinder
sudo mv kinder /usr/local/bin/
```

### Linux (amd64)

```sh
curl -Lo kinder https://github.com/PatrykQuantumNomad/kinder/releases/latest/download/kinder-linux-amd64
chmod +x kinder
sudo mv kinder /usr/local/bin/
```

### Linux (arm64)

```sh
curl -Lo kinder https://github.com/PatrykQuantumNomad/kinder/releases/latest/download/kinder-linux-arm64
chmod +x kinder
sudo mv kinder /usr/local/bin/
```

### Windows (amd64)

Download [kinder-windows-amd64](https://github.com/PatrykQuantumNomad/kinder/releases/latest/download/kinder-windows-amd64) and add it to your `PATH`.

## Build from Source

If you prefer to build from source, you will also need **Go 1.24 or later** ([Install Go](https://go.dev/doc/install)).

```sh
git clone https://github.com/PatrykQuantumNomad/kinder.git
cd kinder
make install
```

This compiles the `kinder` binary and places it in your `$GOPATH/bin` (or `$GOBIN` if set).

If the command is not found after building, make sure `$(go env GOPATH)/bin` is included in your `PATH`:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

## Verify Installation

```sh
kinder version
```

You should see output like:

```
kinder v0.1.0-alpha go1.25.7 darwin/arm64
```
