---
title: JSON Output Reference
description: Use --output json with kinder commands and filter results with jq.
---

Four kinder commands support `--output json` for scripting and automation: `get clusters`, `get nodes`, `env`, and `doctor`. JSON output goes to stdout; informational messages (such as progress output from `doctor`) go to stderr so they do not interfere with piped JSON.

## Commands

### kinder get clusters

```sh
kinder get clusters --output json
```

Returns a JSON array of strings — one entry per cluster name. An empty result returns `[]`.

```json
["kind","my-cluster"]
```

```sh
# Get the first cluster name
kinder get clusters --output json | jq '.[0]'

# Count how many clusters exist
kinder get clusters --output json | jq 'length'

# Check if a specific cluster exists
kinder get clusters --output json | jq 'any(. == "kind")'
```

### kinder get nodes

```sh
kinder get nodes --output json
```

Returns a JSON array of objects. Each object has a `name` field (string) and a `role` field (string: `"control-plane"` or `"worker"`). An empty result returns `[]`. The `--all-clusters` / `-A` flag is compatible with `--output json`.

```json
[{"name":"kind-control-plane","role":"control-plane"},{"name":"kind-worker","role":"worker"}]
```

```sh
# List only control-plane node names
kinder get nodes --output json | jq '[.[] | select(.role == "control-plane") | .name]'

# Count worker nodes
kinder get nodes --output json | jq '[.[] | select(.role == "worker")] | length'

# List nodes across all clusters
kinder get nodes --all-clusters --output json | jq '.[].name'
```

### kinder env

```sh
kinder env --output json
```

Returns a single JSON object (not an array) with fields: `kinderProvider` (string), `clusterName` (string), and `kubeconfig` (string path).

:::note
`kinder env` outputs a single object, not an array. Use `.fieldName` to extract values, not `.[]`.
:::

```json
{"kinderProvider":"docker","clusterName":"kind","kubeconfig":"/Users/user/.kube/config"}
```

Use `--name` to target a cluster created with a non-default name:

```sh
kinder env --name my-cluster --output json
```

```sh
# Extract the container runtime provider
kinder env --output json | jq -r '.kinderProvider'

# Set KUBECONFIG from JSON output
export KUBECONFIG=$(kinder env --output json | jq -r '.kubeconfig')
```

### kinder doctor

```sh
kinder doctor --output json
```

Returns a JSON array of check result objects. Each object has a `name` field (string) and a `status` field (`"ok"`, `"warn"`, or `"fail"`). A `message` field is included when `status` is `"warn"` or `"fail"` and omitted when `status` is `"ok"`.

All checks passing:

```json
[{"name":"docker","status":"ok"},{"name":"kubectl","status":"ok"}]
```

With a failure:

```json
[{"name":"container-runtime","status":"fail","message":"no container runtime found..."},{"name":"kubectl","status":"ok"}]
```

```sh
# Check if any checks failed
kinder doctor --output json | jq 'any(.[]; .status == "fail")'

# Get only failed checks with their messages
kinder doctor --output json | jq '[.[] | select(.status == "fail") | {name, message}]'

# Exit non-zero if any check is not ok (useful in CI)
kinder doctor --output json | jq -e 'all(.[]; .status == "ok")' > /dev/null
```

## Common jq patterns

These patterns work across multiple kinder commands:

```sh
# Pretty-print JSON output
kinder doctor --output json | jq '.'

# Extract all node names as plain text (one per line)
kinder get nodes --output json | jq -r '.[].name'

# CI guard: fail the pipeline if any doctor check is not ok
kinder doctor --output json | jq -e 'all(.[]; .status == "ok")' > /dev/null
```

## Commands without JSON support

The following commands do not support `--output json`:

- `kinder get kubeconfig`
- `kinder create cluster`
- `kinder delete cluster`

Passing `--output json` to these commands will return an error.
