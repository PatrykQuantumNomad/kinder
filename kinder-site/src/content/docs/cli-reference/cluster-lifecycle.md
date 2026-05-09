---
title: Cluster Lifecycle
description: Reference for kinder pause, resume, and status — quorum-safe cluster start/stop without losing state.
---

`kinder pause`, `kinder resume`, and `kinder status` are the three commands that manage a cluster's runtime state without recreating it. Pause stops every node container in quorum-safe order so the host can reclaim CPU and RAM. Resume brings them back in the reverse order and gates on all-nodes-Ready before returning. Status reports current cluster + per-node container state.

All three commands accept the cluster name as a **positional argument** or via `--name`. If neither is given and exactly one cluster exists, that cluster is auto-selected.

## kinder pause

```
kinder pause [cluster-name] [flags]
```

Gracefully stops every node container. Workers stop first, then control-plane nodes, then the load balancer (HA only). Cluster state — pods, PVCs, services, node identities, etcd data — survives the pause.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--timeout` | `30s` | Per-container graceful stop timeout before SIGKILL is sent. Generous default leaves room for kubelet/etcd flush. Accepts Go duration strings (`30s`, `2m`) |
| `--json` | `false` | Output a JSON object describing per-node stop results |

### Examples

```sh
# Pause the default "kind" cluster
kinder pause

# Pause a named cluster
kinder pause my-cluster

# Pause with a longer graceful-stop window
kinder pause my-cluster --timeout 2m

# JSON output for scripted consumers
kinder pause my-cluster --json
```

### HA pre-pause snapshot

On a multi-control-plane cluster, `kinder pause` captures `/kind/pause-snapshot.json` containing the etcd leader ID before stopping any container. This lets `cluster-resume-readiness` (run by `kinder resume` and `kinder doctor`) detect quorum risk on resume. The snapshot is best-effort — failures log a warning but do not abort the pause.

### Idempotency

Calling `kinder pause` on an already-paused cluster is a no-op that logs a warning and returns success. The same applies if a pre-existing pause was incomplete; surviving running containers are stopped on re-invocation.

---

## kinder resume

```
kinder resume [cluster-name] [flags]
```

Restarts every node container in the reverse order — load balancer first, then control-plane nodes, then workers — and waits for all nodes to report Ready before returning. On HA clusters, the `cluster-resume-readiness` doctor check runs inline between control-plane start and worker start so you see a quorum warning **before** workers are wedged into a degraded etcd.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--timeout` | `30s` | Per-container graceful start timeout. Accepts Go duration strings |
| `--wait` | `5m` | Maximum time to wait for all nodes Ready after start. Set to `0s` to skip the readiness gate |
| `--json` | `false` | Output a JSON object describing per-node start results and final cluster status |

### Examples

```sh
# Resume the default cluster, wait up to 5 minutes for Ready
kinder resume

# Resume a named cluster with a longer wait window
kinder resume my-cluster --wait 10m

# Resume without waiting (returns as soon as containers are started)
kinder resume my-cluster --wait 0s

# JSON output
kinder resume my-cluster --json
```

### Readiness gating

`kinder resume` polls `kubectl get nodes --no-headers` until all nodes report `Ready` or `--wait` elapses. The gate uses a built-in selector fallback (`control-plane` ↔ `master`) so it works with K8s 1.24 clusters that still use the legacy label.

If any container fails to start, the readiness probe is skipped (no point waiting for a known-incomplete cluster) and the aggregated start errors are returned directly.

---

## kinder status

```
kinder status [cluster-name] [flags]
```

Reports the cluster's overall state plus per-node container state. Useful for confirming what shape a cluster is in after `pause` / `resume` cycles or before running `snapshot`.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--output` | `""` (text) | Output format. Pass `json` for machine-readable output |

### Cluster status values

| Value | Meaning |
|---|---|
| `Running` | All node containers are running |
| `Paused` | Every node container is in `exited` or `paused` state (typical post-`kinder pause`) |
| `Mixed` | Some containers running, some not — usually a partial pause/resume failure |
| `NotFound` | No cluster with this name exists |

### Examples

```sh
# Default text output
kinder status my-cluster
# Cluster: my-cluster  Status: Running
#   my-cluster-control-plane    running
#   my-cluster-worker           running

# JSON output for scripts
kinder status my-cluster --output json
```

The same Status column also appears on `kinder get clusters` (one line per cluster) and on `kinder get nodes` (per-node `STATUS` column derived from container-runtime state).

---

## Status / state semantics

`kinder pause`, `kinder resume`, and `kinder status` all read the underlying container-runtime state directly via `docker inspect` (or the equivalent for podman / nerdctl). The mapping from runtime state to user-visible Status is:

| Container state | Reported as |
|---|---|
| `running` | `Ready` (on `get nodes`) / `Running` (on cluster Status) |
| `exited`, `created` | `Stopped` |
| `paused` (`docker pause`, distinct from `kinder pause`) | `Paused` |
| anything else, or inspect error | `Unknown` |

Note that `kinder pause` uses `docker stop` (graceful SIGTERM → SIGKILL) — not `docker pause` (cgroup freeze). The reason is that resume time matters more than nanosecond-precise pause time for a local dev cluster, and `docker stop` lets etcd flush state cleanly.

---

## See also

- [Snapshot &amp; Restore](/cli-reference/snapshot/) — capture and replay full cluster state including etcd, images, and PVs
- [Cluster Resume Readiness check](/known-issues/#cluster-resume-readiness) — the doctor check that warns when HA quorum is at risk on resume
- [Quick Start: Pause, resume, snapshot](/quick-start/#pause-resume-snapshot)
