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

## Homebrew (macOS)

```sh
brew install patrykquantumnomad/kinder/kinder
```

This installs a pre-built binary for your architecture (Apple Silicon or Intel). No Go toolchain required.

## Download Pre-built Binary

Pre-built binaries for macOS, Linux, and Windows are available from
[GitHub Releases](https://github.com/PatrykQuantumNomad/kinder/releases/latest).

Download the archive for your platform, extract, and move to your PATH:

### macOS / Linux

```sh
# Replace the URL with the archive for your platform from the latest release
tar xzf kinder_*_darwin_arm64.tar.gz
chmod +x kinder
sudo mv kinder /usr/local/bin/
```

:::caution[macOS direct download]
kinder macOS binaries are **ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`**.

Ad-hoc signing satisfies Apple's AMFI kernel check on Apple Silicon (the binary no longer gets `Killed: 9` on first run), but it does **not** satisfy Gatekeeper for files downloaded via a web browser. After downloading from GitHub Releases, remove the quarantine attribute before running:

```sh
xattr -d com.apple.quarantine kinder
```

Users installing via `brew install patrykquantumnomad/kinder/kinder` are unaffected — Homebrew bypasses Gatekeeper quarantine for formula-installed binaries.
:::

### Windows

Download the `.zip` archive from [GitHub Releases](https://github.com/PatrykQuantumNomad/kinder/releases/latest) and extract `kinder.exe` to a directory in your `PATH`.

## Build from Source

If you prefer to build from source, you will also need **Go 1.24+** ([Install Go](https://go.dev/doc/install)).

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
kind v1.5.0 go1.26.2 darwin/arm64
```
