---
title: Troubleshooting
description: Diagnose issues with kinder env and kinder doctor.
---

This page covers exit codes, check results, and common errors for `kinder env` and `kinder doctor`.

## kinder env

### Exit codes

| Exit code | Meaning |
|-----------|---------|
| 0 | Success — provider detected, output written |
| 1 | Error — unsupported output format or provider detection failure |

:::note
`kinder env` does not verify that the cluster is running. It resolves environment variables (provider, cluster name, kubeconfig path) without contacting the container runtime.
:::

### Common errors

**Unknown output format**

- **Symptom:** `kinder env --output yaml` returns an error.
- **Cause:** Only `json` and the default text format are supported. `yaml`, `table`, and other formats are not recognized.
- **Fix:** Use `kinder env` (default text) or `kinder env --output json`.

**Wrong cluster name in output**

- **Symptom:** `kinder env` shows `KIND_CLUSTER_NAME=kind` but your cluster has a different name.
- **Cause:** The `--name` flag defaults to `"kind"`. If you created a cluster with `--name my-cluster`, you must also pass `--name my-cluster` to `kinder env`.
- **Fix:** `kinder env --name my-cluster`

### Shell integration

:::tip
Use `eval $(kinder env)` to set all kinder environment variables in your current shell in one step:

```sh
eval $(kinder env)
eval $(kinder env --name my-cluster)
```
:::

## kinder doctor

:::tip
For a complete reference of every diagnostic check, what it detects, and how to fix it, see the [Known Issues](/known-issues/) page.
:::

### Exit codes

| Exit code | Meaning |
|-----------|---------|
| 0 | All checks passed |
| 1 | One or more checks failed |
| 2 | One or more checks warned (no failures) |

If both failures and warnings exist, exit code 1 takes priority over exit code 2.

### Check results

| Check name | What it checks | Possible statuses |
|------------|----------------|-------------------|
| `docker` / `podman` / `nerdctl` | Container runtime binary found and daemon responding | ok, warn, fail |
| `container-runtime` | Fallback name when no runtime binary is found at all | fail |
| `kubectl` | kubectl binary found and `kubectl version --client` succeeds | ok, warn |

:::note
The check name is the actual runtime found (e.g., `docker`), not a generic label. The name `container-runtime` only appears when no runtime binary is found.

Detection order: docker, then podman, then nerdctl. kinder stops at the first binary found.
:::

### Common scenarios

**No container runtime found**

- **Symptom:** Exit code 1. JSON output shows `{"name":"container-runtime","status":"fail","message":"no container runtime found..."}`.
- **Cause:** None of `docker`, `podman`, or `nerdctl` is installed or in PATH.
- **Fix:** Install [Docker](https://docs.docker.com/get-docker/), [Podman](https://podman.io/getting-started/installation), or [nerdctl](https://github.com/containerd/nerdctl). Then re-run `kinder doctor`.

**Docker found but not responding**

- **Symptom:** Exit code 2. JSON shows `{"name":"docker","status":"warn","message":"docker found but not responding..."}`.
- **Cause:** The `docker` binary is on PATH but the Docker daemon is not running.
- **Fix:** Start the Docker daemon (`systemctl start docker` on Linux, open Docker Desktop on macOS/Windows), then re-run `kinder doctor`.

**kubectl not found**

- **Symptom:** Exit code 1 (if combined with a runtime fail) or exit code 2 (if runtime is ok but kubectl warns). JSON shows `{"name":"kubectl","status":"warn"}` or `{"name":"kubectl","status":"fail"}`.
- **Cause:** `kubectl` is not installed or not in PATH.
- **Fix:** Install kubectl: https://kubernetes.io/docs/tasks/tools/. Then re-run `kinder doctor`.

**All checks pass but cluster still fails**

- **Symptom:** `kinder doctor` returns exit code 0 but `kinder create cluster` fails.
- **Cause:** `kinder doctor` checks prerequisites (runtime and kubectl), not cluster state. Issues like insufficient disk space, port conflicts, or Docker resource limits are not checked.
- **Fix:** Check Docker resource settings (memory, disk), ensure no port conflicts on the host, and review the `kinder create cluster` error output.

### Using doctor in CI

:::tip
Use `kinder doctor --output json` as a CI gate before running cluster operations:

```sh
kinder doctor --output json | jq -e 'all(.[]; .status == "ok")' > /dev/null
if [ $? -ne 0 ]; then
  echo "Prerequisites not met"
  kinder doctor
  exit 1
fi
```
:::
