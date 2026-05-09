---
title: Snapshot &amp; Restore
description: Reference for kinder snapshot — capture and restore complete cluster state including etcd, images, and local-path PV contents.
---

`kinder snapshot` captures a complete cluster's state into a single archive and restores it in seconds. A snapshot includes etcd state, every container image loaded into the cluster, local-path-provisioner PV contents, and metadata describing the topology + addon versions for reproducibility.

Snapshots live under `~/.kinder/snapshots/<cluster>/` (mode `0700`) as `.tar.gz` bundles with a `.sha256` sidecar for integrity verification.

## Synopsis

```
kinder snapshot create  [cluster-name] [snap-name] [flags]
kinder snapshot restore [cluster-name] <snap-name>  [flags]
kinder snapshot list    [cluster-name]              [flags]
kinder snapshot show    [cluster-name] <snap-name>  [flags]
kinder snapshot prune   [cluster-name]              [flags]
```

All five subcommands accept the cluster name as a **positional argument** or via auto-detection when exactly one cluster exists.

---

## kinder snapshot create

Captures a snapshot of a running cluster. Pauses the cluster mid-flow to capture image and PV contents consistently, then resumes automatically.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--name` | `snap-YYYYMMDD-HHMMSS` | Explicit snapshot name. Defaults to a timestamp-based name |
| `--json` | `false` | Output a JSON `CreateResult` describing capture phases and final metadata |

### Examples

```sh
# Auto-named snapshot of the default cluster
kinder snapshot create
# Created snapshot "snap-20260507-143022" (size: 412 MiB, k8s: v1.35.1)

# Named snapshot
kinder snapshot create my-cluster baseline

# Or via --name
kinder snapshot create my-cluster --name baseline

# JSON output
kinder snapshot create my-cluster baseline --json
```

### Capture flow

1. **etcd snapshot** is captured WHILE the cluster is still running, via `crictl exec <etcd-id> etcdctl snapshot save`. This is the only step that runs on a live cluster — it produces a consistent point-in-time database export
2. **`lifecycle.Pause`** is called next to stop containers in quorum-safe order, freezing image and PV state
3. **Container images** are exported per-node via `ctr --namespace=k8s.io images export`
4. **Local-path PVs** are tarred per-node from `/opt/local-path-provisioner` (only nodes where the directory exists are included)
5. **Metadata** is written: K8s version, node image, addon versions, image-bundle digest, etcd digest, PVs digest
6. **Bundle** is written as a single `.tar.gz` with a streaming sha256 computed on-the-fly into a `.sha256` sidecar
7. **`lifecycle.Resume`** is called via `defer` so the cluster comes back automatically — even on capture failure

The defer-resume guarantee means a failed snapshot leaves the cluster running, not stuck in a paused state.

---

## kinder snapshot restore

Restores a cluster to a previously captured snapshot. Hard overwrite by design: there is no `--yes` flag and the restore cannot be auto-rolled-back.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--json` | `false` | Output a JSON `RestoreResult` describing pre-flight + mutation phases |

### Pre-flight gauntlet (runs BEFORE any mutation)

`kinder snapshot restore` runs **seven** pre-flight checks before any container is stopped or any state is touched. If any check fails, the cluster keeps running and the command exits non-zero with an actionable error.

1. **Bundle integrity** — re-hash the archive and compare against the `.sha256` sidecar
2. **Disk space** — ensure free space on the cluster's container root is at least 8 GiB
3. **Cluster running** — restore requires a running cluster (etcd reachability)
4. **K8s version match** — fail if `snapshot.K8sVersion != live.K8sVersion`
5. **Topology match** — fail if node count or roles differ
6. **Addon version match** — fail if any addon's deployed version differs from snapshot metadata
7. **Aggregate compat** — `CheckCompatibility` returns all violations at once via `kinderrors.NewAggregate`, so `errors.Is` can drill into each sentinel independently

### Mutation phase (no rollback)

After all pre-flight checks pass:

1. `lifecycle.Pause` (quorum-safe stop)
2. **Etcd restore** — HA-safe with shared `--initial-cluster-token`, manifest-aside + atomic data-dir swap. Per-CP restore runs in parallel
3. **Image re-import** — via the existing `nodeutils.LoadImageArchiveWithFallback` pipeline (Docker Desktop 27+ containerd compatible)
4. **PV restore** — per-node untar of `/opt/local-path-provisioner`
5. `lifecycle.Resume`

If a mutation step fails, the error message includes a recovery hint: `run "kinder resume <cluster>" to restart`. There is no auto-rollback — by design, the operator decides whether to retry the restore or recover manually.

### Examples

```sh
# Restore by snap-name (auto-detects single cluster)
kinder snapshot restore baseline

# Restore on a named cluster
kinder snapshot restore my-cluster baseline

# JSON output
kinder snapshot restore my-cluster baseline --json
```

### Common refusal errors

| Error | Cause |
|---|---|
| `bundle sha256 mismatch` | The archive on disk has been corrupted or partially written |
| `k8s version mismatch: snapshot=v1.35.1, live=v1.36.0` | Cluster was upgraded after the snapshot was taken |
| `topology mismatch: snapshot=3 nodes, live=2 nodes` | Cluster was resized — restore would need a matching topology |
| `addon version mismatch: metallb snapshot=v0.15.3, live=v0.15.4` | An addon was upgraded after snapshot |

---

## kinder snapshot list

Lists snapshots for a cluster in a tabwriter table.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--output` | `""` (table) | Output format: `json` or `yaml` |
| `--no-trunc` | `false` | Do not truncate any column (useful for long addon lists) |

### Default columns

| Column | Source |
|---|---|
| `NAME` | snap-name from filename |
| `AGE` | `time.Since(metadata.Created)` rendered relative |
| `SIZE` | size of the `.tar.gz` archive |
| `K8S` | `metadata.K8sVersion` |
| `ADDONS` | comma-separated addon versions; truncated to 50 runes for 120-col terminals (use `--no-trunc` to disable) |
| `STATUS` | `ok` if archive sha256 matches sidecar; `corrupt` if integrity check fails; `incomplete` if the sidecar is missing |

### Examples

```sh
kinder snapshot list
# NAME                    AGE     SIZE     K8S       ADDONS                        STATUS
# baseline                2h ago  412 MiB  v1.35.1   metallb=v0.15.3,headlamp=v...  ok
# snap-20260507-093022    8h ago  408 MiB  v1.35.1   metallb=v0.15.3,headlamp=v...  ok

kinder snapshot list my-cluster --output json | jq '.[] | select(.status == "corrupt")'
```

---

## kinder snapshot show

Prints full metadata for a single snapshot in a vertical key/value layout.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--output` | `""` (vertical) | Output format: `json` or `yaml` |

### Fields shown

```
Name:           baseline
Cluster:        my-cluster
Created:        2026-05-07T14:30:22Z (2h ago)
Size:           412 MiB
K8s Version:    v1.35.1
Node Image:     kindest/node:v1.35.1@sha256:...
Topology:       1 control-plane, 2 workers
Addons:
  metallb:        v0.15.3
  headlamp:       v0.40.1
  envoyGateway:   v1.3.1
  certManager:    v1.16.3
  localPath:      v0.0.35
Etcd Digest:    sha256:abc123...
Images Digest:  sha256:def456...
PVs Digest:     sha256:789012...
Archive Digest: sha256:fed987... (from sidecar)
```

The metadata is what `restore` checks against for compat. Use `show` to confirm a snapshot is compatible with your current cluster before attempting a restore.

---

## kinder snapshot prune

Deletes snapshots according to one or more retention policies. **Refuses to run** if no policy flag is given — there is no naked-invocation purge to prevent accidents.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--keep-last N` | `0` (off) | Keep the N most-recent snapshots, delete older |
| `--older-than DUR` | `0` (off) | Delete snapshots older than the given duration (e.g. `168h` for 7 days) |
| `--max-size SIZE` | `""` (off) | Delete oldest snapshots until the total cluster footprint is &lt;= SIZE (e.g. `10G`, `500M`; base-2 multipliers, case-insensitive) |
| `-y, --yes` | `false` | Skip the y/N confirmation prompt |

### Policy semantics

The deletion plan is the **union** of all active policies — a snapshot is in the deletion set if **any** active policy marks it. A flag with a zero value is inactive (e.g., `--keep-last 0` means "no keep-last policy"; not "delete everything").

The plan is printed before any file is touched:

```sh
kinder snapshot prune --keep-last 3 --older-than 168h
# Prune plan for cluster "my-cluster":
#   DELETE  snap-20260430-101502  (8d ago, 410 MiB)
#   DELETE  snap-20260429-145830  (9d ago, 412 MiB)
#   KEEP    baseline              (most recent 3)
#   KEEP    snap-20260506-203015  (most recent 3)
#   KEEP    snap-20260507-093022  (most recent 3)
# Total to delete: 822 MiB across 2 snapshots
# Continue? [y/N]
```

### Examples

```sh
# Keep only the 5 most recent snapshots
kinder snapshot prune my-cluster --keep-last 5

# Delete snapshots older than 7 days, no prompt
kinder snapshot prune my-cluster --older-than 168h --yes

# Cap total size at 10 GiB
kinder snapshot prune my-cluster --max-size 10G

# Combine policies (union semantics)
kinder snapshot prune my-cluster --keep-last 3 --max-size 5G
```

### Refusal on naked invocation

```sh
kinder snapshot prune my-cluster
# ERROR: at least one policy flag is required (--keep-last, --older-than, --max-size)
```

---

## See also

- [Cluster Lifecycle](/cli-reference/cluster-lifecycle/) — `kinder pause`, `resume`, `status`
- [Cluster Resume Readiness check](/known-issues/#cluster-resume-readiness) — runs before resume to detect HA quorum risk
- [Quick Start: Pause, resume, snapshot](/quick-start/#pause-resume-snapshot)
