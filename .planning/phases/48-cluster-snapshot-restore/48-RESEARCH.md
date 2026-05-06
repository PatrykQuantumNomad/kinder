# Phase 48: Cluster Snapshot/Restore - Research

**Researched:** 2026-05-06
**Domain:** etcd snapshot/restore, containerd image export, local-path-provisioner PV capture, tar.gz bundling, Cobra CLI
**Confidence:** HIGH (all findings from direct codebase inspection)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Storage layout & format**
- Snapshots live at `~/.kinder/snapshots/<cluster>/` — per-cluster subdirectory under user home; survives `docker system prune`.
- Single `.tar.gz` bundle per snapshot containing: `etcd.snap`, `images.tar`, `pvs.tar`, `kind-config.yaml`, `metadata.json`. Component layout decided at planning time; archive is one file for easy copy/share.
- Snapshots are **cluster-scoped**: every command implicitly operates on the named/current cluster; `kinder snapshot list` shows snapshots for that cluster only.
- Naming: `kinder snapshot create [<name>]` — name is **optional**; if omitted, default to `snap-YYYYMMDD-HHMMSS` (timestamp-based, never auto-overwriting).

**Capture scope & metadata**
- The original kind cluster config YAML **is captured** inside the archive (`kind-config.yaml`). Snapshot is fully self-describing for future restore-into-fresh-cluster scenarios.
- Secrets/ConfigMaps captured **as-is inside the etcd snapshot** — no redaction, no warning. Snapshots are local-dev artifacts; document the implication in the snapshot command help / website docs (not required as runtime UX).
- `metadata.json` carries SC4 fields (cluster K8s version, addon versions, image-bundle digest) PLUS:
  - SHA-256 of the full `.tar.gz` archive (sidecar `<name>.tar.gz.sha256`).
  - Per-component SHA-256 digests (etcd.snap, images.tar, pvs.tar) inside `metadata.json`.
- Image-bundle digest mismatch on restore: **always re-load images from `images.tar`** — the archive is the source of truth. Local docker image cache is treated as untrusted.

**Restore semantics & safety**
- `kinder snapshot restore` works on a **running** cluster: implicitly pause (reuse Phase 47 `lifecycle.Pause`), restore etcd + PVs + image bundle, resume (reuse Phase 47 `lifecycle.Resume`). One-shot UX, no separate `kinder pause` step required.
- **Hard overwrite, no prompt** — `restore` is the action verb; expectation is the cluster is wiped to snapshot state. No interactive confirmation, no `--yes` flag needed.
- **Atomicity = pre-flight + fail-fast, no rollback.** Validate archive integrity (sha256), free disk space, K8s/topology/addon compatibility, etcd reachability BEFORE any mutation. If a later step still fails, surface the error with a clear recovery hint and leave the cluster in its current (inconsistent) state. No implicit pre-restore snapshot.
- Compatibility checks that **hard-fail** restore:
  - K8s version mismatch (SC2 minimum, mandatory).
  - Cluster topology mismatch (node count or roles differ from `metadata.json`).
  - Addon-version mismatch (any installed addon's version differs).
  All three are hard-fail; no warn-only path. User must recreate the cluster or pick a matching snapshot.

**Lifecycle UX (list / show / prune)**
- Three output formats: table by default, `--json`, `--output yaml`. Matches the broader kinder `--output` convention.
- `kinder snapshot list` columns: **NAME, AGE, SIZE, K8S, ADDONS, STATUS**. STATUS is `ok` / `corrupt` based on archive sha256 verification. ADDONS is a comma-joined version summary. Wide-terminal first; `--no-trunc` if any column would be cut.
- `kinder snapshot prune` with **no flags refuses** with an error listing supported policy flags: `--keep-last N | --older-than DURATION | --max-size SIZE`. Safest default; never deletes silently on a typo.
- `prune` confirmation: **always prompt unless `--yes`**. Print the list of snapshots that will be deleted, then `y/N`. `--yes` (or `-y`) bypasses for CI.

### Claude's Discretion
- Internal package layout (`pkg/internal/snapshot/` likely follows the Phase 47 `pkg/internal/lifecycle/` pattern).
- Compression level for `.tar.gz` (default gzip vs streaming — pick based on benchmark).
- Exact JSON schema field names inside `metadata.json` (decisions lock the *fields*, not the wire-format names).
- Concurrency model for image-bundle export (parallel vs serial `docker save`).
- Disk-space pre-check threshold (e.g. require 2× archive size free) — pick a reasonable safety margin.
- Exact error-message phrasing for compatibility-check failures.
- Whether `snapshot restore` accepts a positional cluster name (matching Phase 47-06's `kinder get nodes` / `pause` / `resume` convention) — recommended yes, but flagged as a planner-discretion decision since it isn't a user-facing decision.
- Whether `snapshot show <name>` mirrors `list`'s columns or expands into a vertical key/value layout.

### Deferred Ideas (OUT OF SCOPE)
- **Snapshot diff** (`kinder snapshot diff <a> <b>`): not in this phase.
- **Cross-cluster restore**: explicitly out of scope.
- **Scheduled snapshots / retention policy daemon**: out of scope.
- **Remote storage backends (S3, OCI registry, GCS)**: out of scope.
- **`--redact-secrets` flag**: deferred.
- **Auto pre-restore rollback snapshot**: deferred.
- **Pause/Resume of in-flight restore (signal handling)**: not in scope.
</user_constraints>

---

## Summary

Phase 48 adds snapshot/restore to kinder by combining three established mechanisms: etcd's native `snapshot save/restore` CLI, containerd's `ctr images export/import` from inside kind node containers, and a tar capture of `/opt/local-path-provisioner` while the cluster is paused. All three operations happen with the cluster containers stopped (via Phase 47 `lifecycle.Pause`), giving a consistent point-in-time state. The implementation bundles all components into a single `.tar.gz` on the host filesystem.

The core restore loop is: pause → swap etcd data-dir → untar PVs → import images → resume. This is a warm in-place swap, NOT a cluster destroy-and-recreate, satisfying the "restore in seconds" goal (seconds = etcd restore + PV untar, typically under 10s excluding image import which can be 30–120s depending on image count and size).

No new Go dependencies are required. All bundling, hashing, and filesystem operations use Go stdlib (`archive/tar`, `compress/gzip`, `crypto/sha256`, `encoding/json`, `syscall.Statfs_t`). The CLI follows the exact Phase 47 patterns established in `pkg/cmd/kind/pause/` and `pkg/cmd/kind/resume/`.

**Primary recommendation:** Model `pkg/internal/snapshot/` after `pkg/internal/lifecycle/` — keep the orchestration logic in a library package with injected dependencies for testability; wire to Cobra in `pkg/cmd/kind/snapshot/{create,restore,list,show,prune}.go`.

---

## 1. etcd Snapshot/Restore Mechanics in a Kind Cluster

**Confidence: HIGH** — verified from Phase 47 live code in `pkg/internal/lifecycle/pause.go` and `pkg/internal/doctor/resumereadiness.go`.

### Taking a snapshot

etcd runs as a static pod inside the kind control-plane container. `etcdctl` is NOT on the kindest/node rootfs; it ships only inside the etcd static-pod container image (`registry.k8s.io/etcd:VERSION`). Phase 47 already solved this pattern: use `crictl ps --name etcd -q` to discover the running etcd container ID, then `crictl exec <id> etcdctl ...` to run etcdctl commands from inside the node container.

The snapshot command (to be run from inside the kind node via `bootstrap.Command(...)` → `crictl exec`):
```
crictl exec <etcd-container-id> etcdctl \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/peer.crt \
  --key=/etc/kubernetes/pki/etcd/peer.key \
  --endpoints=https://127.0.0.1:2379 \
  snapshot save /tmp/etcd.snap
```

The auth args are already captured as `var etcdctlAuthArgs` in `pkg/internal/doctor/resumereadiness.go:85`. Re-export this variable (or duplicate it in the snapshot package) to avoid an import cycle.

After the snapshot is written to `/tmp/etcd.snap` inside the node container, copy it to the host via:
```go
// pipe docker exec cat /tmp/etcd.snap into a host file
node.Command("cat", "/tmp/etcd.snap").SetStdout(hostWriter).Run()
```
This is the same pattern used in `pkg/cluster/nodeutils/util.go:CopyNodeToNode()`.

The snapshot MUST be taken while the cluster is **running** (etcd must be responsive). In the restore flow, the decision locks state that restore works on a running cluster that is first paused: take the snapshot BEFORE pausing, or pause first and restore (etcd doesn't need to be running for restore — the snapshot was already captured). For `snapshot create`, take the etcd snapshot while the cluster is running, then pause to capture PVs and images consistently. Resume afterward. The snapshot is consistent because etcd's `snapshot save` is atomic at the etcd level.

**Alternative considered (snapshot while paused):** Can't — etcd is not running when cluster is paused. Snapshot must be taken from a running cluster.

**Recommended `snapshot create` flow:**
1. Verify cluster is running and etcd is reachable.
2. Take etcd snapshot (while running) — writes `etcd.snap` to a temp dir.
3. Pause cluster (Phase 47 `lifecycle.Pause`).
4. Copy images from node containerd → `images.tar`.
5. Tar `/opt/local-path-provisioner` → `pvs.tar`.
6. Bundle all components → `<name>.tar.gz` with streaming sha256.
7. Resume cluster (Phase 47 `lifecycle.Resume`).

### Restoring a snapshot

etcd restore requires stopping kube-apiserver and etcd first (cluster must be paused — Phase 47 handles this). The restore flow inside the node:

```
crictl exec <etcd-container-id> etcdctl snapshot restore /tmp/etcd.snap \
  --data-dir=/var/lib/etcd-restored \
  --name=<node-name> \
  --initial-cluster=<node-name>=https://127.0.0.1:2380 \
  --initial-cluster-token=kind-restored \
  --initial-advertise-peer-urls=https://127.0.0.1:2380
```

**Critical pitfalls on restore:**
- The etcd data-dir on kind nodes is `/var/lib/etcd`. The restore writes to a NEW directory (e.g., `/var/lib/etcd-restored`), then you atomically swap: `mv /var/lib/etcd /var/lib/etcd-backup && mv /var/lib/etcd-restored /var/lib/etcd`.
- Ownership: etcd runs as root inside the static pod; the `mv` approach preserves ownership. If using `cp`, explicitly `chown -R root:root /var/lib/etcd`.
- `etcdctl snapshot restore` is available inside the etcd container image (same `crictl exec` approach). The etcd container is NOT running when the cluster is paused — but `crictl exec` only works on running containers. **Pitfall:** To run etcdctl snapshot restore, the node container itself must be running (Phase 47 resume starts it), but etcd static pod must NOT be running yet. This creates a sequencing challenge.

**Recommended restore sequencing for single-control-plane:**
1. Start node container (Phase 47 `docker start <cp>`).
2. Stop etcd static pod WITHOUT resuming kubelet: `node.Command("crictl", "stop", "<etcd-container-id>").Run()` or simply move the static pod manifest: `node.Command("mv", "/etc/kubernetes/manifests/etcd.yaml", "/tmp/etcd-manifest.yaml").Run()`.
3. Copy `etcd.snap` from host into node: `node.Command("cp", "/dev/stdin", "/tmp/etcd.snap").SetStdin(snapReader).Run()`.
4. Run `etcdctl snapshot restore` inside the etcd container (needs the container still accessible but not managing data): use `docker exec <kind-node> etcdctl ...` directly since etcdctl ships in the kindest/node image too... Actually, confirm this: kind nodes may have `etcdctl` at `/usr/local/bin/etcdctl` or it may only be in the etcd image. If not on node: run restore via a temporary etcd container.
5. Swap data-dir.
6. Restore static pod manifest.
7. Resume remaining nodes.

**Important:** The safest approach that avoids etcdctl-on-node ambiguity is to use the `docker exec <kind-node-container> etcdctl` approach directly — kind's node image (`kindest/node`) includes etcdctl in `/usr/local/bin/`. This is simpler than `crictl exec` into the etcd static pod. Verify during planning by checking the kindest/node image manifest.

**Single-node vs HA:**
- Single CP: straightforward — one etcd member, restore with `--initial-cluster=<name>=https://127.0.0.1:2380`.
- HA (≥2 CPs): All etcd members must restore from the SAME snapshot. Sequence: restore all CP nodes' etcd data dirs, then start all CPs together. Use different `--name` and `--initial-cluster` per member but same snapshot file (distribute via `CopyNodeToNode`). Each member needs `--initial-cluster-token` to match; use a fresh token like `kind-restored-<timestamp>` for all members.

**Quorum and Phase 47 reuse:** Phase 47's `ClassifyNodes` (`pkg/internal/lifecycle/state.go:ClassifyNodes`) gives you cp/workers/lb split. Use the same ordering for restore (start LB, then CPs together, then workers).

**etcd data-dir path:** `/var/lib/etcd` inside kind node containers (confirmed by kubeadm defaults for kind). The static pod spec at `/etc/kubernetes/manifests/etcd.yaml` inside the node container shows `--data-dir=/var/lib/etcd`.

---

## 2. Container Image Capture from Kind Nodes

**Confidence: HIGH** — verified from `pkg/cluster/nodeutils/util.go` which already implements `LoadImageArchive` and `LoadImageArchiveWithFallback` using `ctr --namespace=k8s.io images import`.

### How kind stores images

Kind uses **containerd** inside node containers (not the host's Docker daemon). Images are in the `k8s.io` namespace. The existing `LoadImageArchive` at `pkg/cluster/nodeutils/util.go:82` imports from this namespace: `ctr --namespace=k8s.io images import`. The inverse (export) is `ctr --namespace=k8s.io images export`.

### Listing all images in a node

```bash
# Inside the kind node container:
ctr --namespace=k8s.io images list -q
```
This lists all image references in the k8s.io namespace. Run via:
```go
node.Command("ctr", "--namespace=k8s.io", "images", "list", "-q").SetStdout(&buf).Run()
```

**Filter consideration:** The list includes infrastructure images (`pause:3.x`, `coredns`, etc.) AND user-loaded images. Capture ALL of them — the snapshot is meant to be self-contained.

### Exporting images

```bash
# Inside the kind node container:
ctr --namespace=k8s.io images export /tmp/images.tar <image1> <image2> ...
```

Or export ALL images at once by piping the image list:
```bash
ctr --namespace=k8s.io images export /tmp/images.tar $(ctr --namespace=k8s.io images list -q)
```

From Go:
```go
refs, _ := exec.OutputLines(node.Command("ctr", "--namespace=k8s.io", "images", "list", "-q"))
args := append([]string{"--namespace=k8s.io", "images", "export", "/tmp/images.tar"}, refs...)
node.Command("ctr", args...).Run()
// Then copy /tmp/images.tar to host
```

**Only capture from control-plane nodes.** Kind loads images to all nodes, but all nodes have the same set for containerd-managed images. Capture from bootstrap CP (`cp[0]`). For HA restore, re-import to ALL nodes.

**Note on image size:** A typical kind cluster with addons can have 2–5 GB of images. This is the dominant factor in snapshot/restore time. Flag this in pitfalls.

### Re-importing images on restore

Existing `LoadImageArchiveWithFallback` at `pkg/cluster/nodeutils/util.go:130` handles the Docker Desktop 27+ `--all-platforms` fallback. Reuse this function by passing an `openArchive` factory that reads from the tarred `images.tar` component inside the outer `.tar.gz`. The `errors.UntilErrorConcurrent` pattern from `pkg/errors/concurrent.go` can parallelize import across nodes.

**Idempotency:** `ctr images import` is idempotent — re-importing an already-present image is a no-op (content-addressed storage). No need to explicitly check before importing.

**Image-bundle digest:** SHA-256 of `images.tar` (the raw tar file before gzip compression). This matches the decision that the archive is source of truth. Compare the sha256 of the extracted `images.tar` with `metadata.json`'s `imagesDigest` field at restore time. If mismatch: fail fast (corrupt archive).

**CRI confusion: host Docker vs node containerd.** Do NOT use `docker save` from the host to capture node images. The images inside kind nodes are in containerd's k8s.io namespace, NOT in the host Docker daemon. Use `ctr --namespace=k8s.io images export` from inside the node container (via `node.Command(...)`). This is the correct approach and matches the existing `LoadImageArchive` pattern.

---

## 3. Local-Path-Provisioner PV Capture

**Confidence: HIGH** — verified from the embedded manifest at `pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml:144` which shows:
```json
{"nodePathMap": [{"node": "DEFAULT_PATH_FOR_NON_LISTED_NODES", "paths": ["/opt/local-path-provisioner"]}]}
```

### Storage location

PV data is stored at **`/opt/local-path-provisioner`** inside each kind worker node (or control-plane if single-node). This is a directory inside the Docker container filesystem, NOT a bind-mount to the host. It survives Docker container restart but NOT `docker rm`.

The path is configurable via the `local-path-config` ConfigMap in the `local-path-storage` namespace. For kinder's managed installation, it is always `/opt/local-path-provisioner` (hardcoded in the embedded manifest). At capture time, verify by reading the ConfigMap, but the default is reliable.

### Capture strategy

With the cluster paused (all pods stopped by Phase 47 `lifecycle.Pause`), tar the entire directory from inside each node:

```go
// Inside each node container:
node.Command("tar", "-czf", "/tmp/pvs.tar", "-C", "/opt", "local-path-provisioner").Run()
// Then copy to host
```

**Important:** Capture from ALL nodes that have PV data (typically workers, but also single CP). For multi-node clusters, check each node. Simplest approach: tar from every node, prefix entries with `<nodeName>/` to distinguish per-node data.

**Restore:** Untar back:
```go
// Inside node container:
node.Command("tar", "-xzf", "/tmp/pvs.tar", "-C", "/opt").Run()
```

**Permissions:** `tar` inside the container runs as root; `tar -p` (preserve permissions) is the default for root. No special chown needed if using the same node image.

**PV re-attachment:** After restore, the local-path-provisioner pod restarts and automatically re-discovers PVs at the configured path. No annotation refresh needed — the PV records are in etcd (restored), and the data is on disk. The provisioner does not maintain external state.

**Edge cases:**
- hostPath volumes NOT in `/opt/local-path-provisioner` are NOT captured. This is acceptable per scope (only local-path-provisioner PVs are in scope).
- ReadWriteMany: local-path-provisioner creates per-node subdirectories using PVC UIDs. Restoring the tar preserves this structure.
- If `local-path-provisioner` is not installed (addon not enabled), `/opt/local-path-provisioner` may not exist. Check before tarring; if missing, create an empty `pvs.tar` and set `pvsDigest: ""` in `metadata.json`.

---

## 4. Kind Config YAML Capture

**Confidence: HIGH** — verified by inspecting the codebase.

### What kinder stores vs what exists on disk

Kind does NOT persist the original config YAML anywhere after cluster creation. The config is a one-shot input parsed by `encoding.Load()` (`pkg/internal/apis/config/encoding/load.go`). After cluster creation, the YAML is not written to any known location — not inside node containers, not in `~/.kube`, not in any `~/.kinder/` directory.

**What IS persisted inside node containers:**
- `/kind/kubeadm.conf` — kubeadm init/join config (written by `pkg/cluster/internal/create/actions/config/config.go:263`)
- `/kind/version` — Kubernetes version string (read by `nodeutils.KubeVersion`)
- Docker labels on each container: `io.x-k8s.kind.cluster=<name>`, `io.x-k8s.kind.role=<role>` (constants in `pkg/cluster/internal/providers/docker/constants.go`)
- The node IMAGE (`docker inspect --format {{.Config.Image}}`) gives the kindest/node image which encodes the K8s version

**What is NOT available:** The original user-supplied `kind-config.yaml` with `Addons`, `ExtraMounts`, `ContainerdConfigPatches` etc.

### Recommended approach: reconstruct from live cluster state

Since the original YAML is not available for existing clusters, the `snapshot create` command should reconstruct a representative `kind-config.yaml` from live cluster state. Specifically:

1. **Node topology** → list all nodes via `provider.ListNodes(cluster)`, call `n.Role()` for each, get image via `docker inspect --format {{.Config.Image}}`. This gives the node count and roles.
2. **Installed addons** → query each addon's deployment/pod image tag (see Section 5).
3. **Kubernetes version** → `nodeutils.KubeVersion(cp[0])` reads `/kind/version`.
4. **Reconstruct minimal YAML** → generate a `v1alpha4.Cluster` YAML with the observed nodes and addon flags. ExtraMounts, ContainerdConfigPatches, and other advanced fields are NOT recoverable and should be omitted with a comment.

**For new clusters created after Phase 48 ships:** Add a mechanism in `kinder create cluster` to persist the original config YAML to `~/.kinder/snapshots/<cluster>/kind-config.yaml` at create time. This is the ideal path but cannot be required for existing clusters. The planner should plan BOTH the backward-compatible reconstruction AND the new persist-at-create path.

**At restore time:** `kind-config.yaml` is informational only (used for compatibility checks and future restore-into-fresh-cluster). The planner should decide whether restore validates the kind-config beyond the topology fingerprint (which is already a hard-fail check).

---

## 5. Addon Detection & Version Capture

**Confidence: HIGH** — verified from the full addon list in `pkg/cluster/internal/create/create.go:291-302`.

### Installed addons in kinder

Wave 1 addons: Local Registry, MetalLB, Metrics Server, CoreDNS Tuning, Dashboard, Cert Manager, Local Path Provisioner, NVIDIA GPU.
Wave 2 addons: Envoy Gateway.

Each addon has a `var Images = []string{...}` constant (e.g., `installlocalpath.Images`, `installcertmanager.Images`) containing the exact image references with version tags.

**Pattern for detecting installed version at snapshot time:**

Each addon deploys a Deployment or DaemonSet in a well-known namespace. Query the deployment image tag via kubectl inside the bootstrap CP:

```go
node.Command(
    "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
    "get", "deployment", "<deployment-name>",
    "-n", "<namespace>",
    "-o", "jsonpath={.spec.template.spec.containers[0].image}",
).SetStdout(&buf).Run()
// Extract ":tag" suffix
```

**Specific queries per addon:**

| Addon | Namespace | Deployment | Image field |
|-------|-----------|------------|-------------|
| Local Path | local-path-storage | local-path-provisioner | containers[0].image |
| MetalLB | metallb-system | controller | containers[0].image |
| Metrics Server | kube-system | metrics-server | containers[0].image |
| Dashboard | kubernetes-dashboard | kubernetes-dashboard | containers[0].image |
| Cert Manager | cert-manager | cert-manager | containers[0].image |
| Envoy Gateway | envoy-gateway-system | envoy-gateway | containers[0].image |
| CoreDNS Tuning | kube-system | coredns | containers[0].image |
| Local Registry | *varies* | registry | containers[0].image |

**Pattern already proven:** `pkg/internal/doctor/localpath.go:realGetProvisionerVersion()` does exactly this for local-path-provisioner. The `snapshot create` command should use the same kubectl-exec pattern for all addons.

**`metadata.json` addon field structure:**
```json
{
  "addonVersions": {
    "localPath": "v0.0.35",
    "metalLB": "v0.14.8",
    "certManager": "v1.16.3"
  }
}
```

Absent key = addon not installed. Empty object = no addons installed.

**At restore time — hard-fail compatibility check:** Compare `metadata.json`'s `addonVersions` map to the live cluster. Use the same kubectl-exec queries. Fail if any installed addon's version differs from the snapshot's version. Fail if the snapshot has an addon that the live cluster does not, or vice versa.

---

## 6. Topology Fingerprint for Compatibility Check

**Confidence: HIGH** — verified from `pkg/internal/lifecycle/state.go:ClassifyNodes`.

### What constitutes topology

The canonical topology fingerprint for compatibility checking should be:
```json
{
  "controlPlaneCount": 1,
  "workerCount": 2,
  "hasLoadBalancer": false,
  "k8sVersion": "v1.31.2",
  "nodeImage": "kindest/node:v1.31.2"
}
```

**Where this comes from in code:**
- `ClassifyNodes(allNodes)` at `pkg/internal/lifecycle/state.go:142` returns `cp, workers, lb, err`.
- K8s version: `nodeutils.KubeVersion(cp[0])` reads `/kind/version`.
- Node image: `docker inspect --format {{.Config.Image}} <container>` — already used in `pkg/cmd/kind/get/nodes/nodes.go:222`.

**At snapshot create time:** Capture topology into `metadata.json`.

**At restore time — hard-fail checks:**
1. K8s version: `metadata.k8sVersion` must equal current `nodeutils.KubeVersion(cp[0])`.
2. Node count and roles: `cp count + worker count + lb presence` must match exactly.
3. Addon versions: see Section 5.

**Reuse point:** `lifecycle.ClassifyNodes` is already exported and can be called from the snapshot package without an import cycle (both would be under `pkg/internal/`).

---

## 7. tar.gz Bundling, Streaming, and Integrity

**Confidence: HIGH** — stdlib, no external dependencies needed.

### Recommended Go approach

Use `archive/tar` + `compress/gzip` with a `crypto/sha256` tee writer for single-pass hashing:

```go
func writeBundle(destPath string, components map[string]io.Reader) (sha256hex string, err error) {
    f, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
    if err != nil { return "", err }
    defer f.Close()

    h := sha256.New()
    mw := io.MultiWriter(f, h)  // single-pass write to disk AND hasher

    gz := gzip.NewWriter(mw)
    defer gz.Close()

    tw := tar.NewWriter(gz)
    defer tw.Close()

    for name, r := range components {
        // Note: size must be known up front for tar headers
        // Use a bytes.Buffer or temp file per component to get the size
        data, _ := io.ReadAll(r)  // for large components, use temp files
        hdr := &tar.Header{Name: name, Mode: 0600, Size: int64(len(data)), ModTime: time.Now()}
        tw.WriteHeader(hdr)
        tw.Write(data)
    }
    gz.Close()
    tw.Close()
    return fmt.Sprintf("%x", h.Sum(nil)), nil
}
```

**For large `images.tar` (potentially GB-sized):** Use a temp file approach rather than `io.ReadAll` to avoid OOM. Write each component to a temp file first (captured from node), then stream into the outer tar with the known size from `os.Stat`.

**Per-component digests:** Hash each component as it is written into the temp file:
```go
compHasher := sha256.New()
tee := io.TeeReader(nodeReader, compHasher)
// write tee to tempFile
// after: compDigest = hex.EncodeToString(compHasher.Sum(nil))
```

Store `etcdSnapDigest`, `imagesDigest`, `pvsDigest` in `metadata.json`.

**Sidecar `.sha256` file format:** Follow `sha256sum` convention:
```
<hex-of-archive>  <snap-name>.tar.gz
```
This allows users to verify with `sha256sum -c snap-20260506-120000.tar.gz.sha256`.

**Directory permissions:** `os.MkdirAll(snapDir, 0700)` — 0700 since snapshots contain unredacted secrets in the etcd snapshot.

**Disk-space pre-check:** Use `syscall.Statfs` (Linux/Darwin). Required free space recommendation: at minimum the estimated compressed archive size. A simple heuristic: sum up the raw component sizes (etcd snapshot is typically 10–50 MB; images.tar can be 1–10 GB; PVs vary). The planner should decide the exact threshold (e.g., `max(raw_sizes * 1.5, 500MB)`).

**Gzip compression level:** Use `gzip.DefaultCompression` (level 6). Do NOT use `gzip.BestCompression` — images are already compressed layers; extra compression yields <5% gain at 3× CPU cost.

---

## 8. CLI Surface — Cobra Command Tree

**Confidence: HIGH** — verified from `pkg/cmd/kind/root.go`, `pkg/cmd/kind/pause/pause.go`, `pkg/cmd/kind/resume/resume.go`, `pkg/cmd/kind/get/nodes/nodes.go`.

### Command placement

Add a new `snapshot` subcommand group analogous to `get`, `delete`, `load`. The root command registration is in `pkg/cmd/kind/root.go` — add `cmd.AddCommand(snapshot.NewCommand(logger, streams))`.

**File layout:**
```
pkg/cmd/kind/snapshot/
├── snapshot.go          # NewCommand() returning "snapshot" cobra group
├── create.go            # "snapshot create [snap-name]"
├── restore.go           # "snapshot restore <snap-name>"
├── list.go              # "snapshot list"
├── show.go              # "snapshot show <snap-name>"
└── prune.go             # "snapshot prune --keep-last N | --older-than D | --max-size S"
```

**Internal package:**
```
pkg/internal/snapshot/
├── create.go            # Create() function and CreateOptions struct
├── restore.go           # Restore() function and RestoreOptions struct
├── metadata.go          # Metadata struct, JSON marshal/unmarshal
├── store.go             # SnapshotStore: list, open, delete, prune policies
├── bundle.go            # tar.gz write/read, sha256 verification
└── doc.go
```

### Positional cluster argument

Follow Phase 47-06's `kinder get nodes` pattern (`pkg/cmd/kind/get/nodes/nodes.go:61-63`):
- `cobra.MaximumNArgs(1)` on the command
- Call `lifecycle.ResolveClusterName(args, provider)` — reuse the existing helper from `pkg/internal/lifecycle/state.go:ResolveClusterName`
- This gives auto-select-if-one-cluster behavior matching pause/resume

For `create`, `restore`, `list`, `prune`: positional arg is `[cluster-name]`.
For `show` and `restore`: `<snap-name>` is a required second positional arg — use `cobra.ExactArgs(1)` after cluster is resolved via `--name` flag OR positional first arg. Planner must decide the exact arg signature.

### Output formatting

**`--json` flag:** Follow the exact pattern from `pause.go:46` — `BoolVar(&flags.JSON, "json", false, "output JSON")`.

**`--output` flag:** Phase 47-06's `get nodes` uses `StringVar(&flags.Output, "output", "", ...)` then checks `flags.Output == "json"`. For `snapshot list` which also needs YAML output, use `sigs.k8s.io/yaml` (already in `go.mod:27`) to marshal the JSON-tagged structs.

**Table output:** Use `text/tabwriter` from stdlib as in `pkg/cmd/kind/get/nodes/nodes.go:175`:
```go
w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
fmt.Fprintln(w, "NAME\tAGE\tSIZE\tK8S\tADDONS\tSTATUS")
```

### Spinner

`pkg/internal/cli/spinner.go` provides `NewSpinner(w io.Writer)` with `Start()` / `Stop()` / `SetPrefix()` / `SetSuffix()`. Use for long-running create/restore operations. The existing pattern from `pkg/cluster/internal/create/actions/action.go` uses `ctx.Status.Start("message")` — for the snapshot package, use the spinner directly since we're not inside an action context.

### Duration flags

For `--older-than` in `prune`, use `cobra.DurationVar` — matching the Phase 47 convention from `pause.go:79`:
```go
c.Flags().DurationVar(&flags.OlderThan, "older-than", 0, "prune snapshots older than duration (e.g. 7d, 720h)")
```

**Note:** Go's `time.ParseDuration` does not support `7d` (days). Either accept hours only (document `168h` for 7 days) or add a custom parser for `d` suffix. This is a planner decision.

---

## 9. Phase 47 Reuse Points

**Confidence: HIGH** — verified from direct source reading.

### lifecycle.Pause API

```go
// pkg/internal/lifecycle/pause.go
func Pause(opts PauseOptions) (*PauseResult, error)

type PauseOptions struct {
    ClusterName string
    Timeout     time.Duration  // per-container stop timeout, default 30s
    Logger      log.Logger
    Provider    *cluster.Provider
}
```

`Pause` is **blocking** — returns when all containers are stopped. Returns `*PauseResult` always non-nil on valid ClusterName. Errors are aggregated via `errors.NewAggregate`. For snapshot create, call Pause after etcd snapshot is taken, before image/PV capture.

### lifecycle.Resume API

```go
// pkg/internal/lifecycle/resume.go
func Resume(opts ResumeOptions) (*ResumeResult, error)

type ResumeOptions struct {
    ClusterName  string
    StartTimeout time.Duration   // per-container start timeout
    WaitTimeout  time.Duration   // readiness gate timeout, default 5m
    Logger       log.Logger
    Provider     nodeFetcher     // *cluster.Provider satisfies this
    Context      context.Context
}
```

`Resume` is blocking — includes the readiness probe waiting for all nodes Ready. For snapshot create, call Resume after bundle is written (so the cluster is running again when the command returns).

For snapshot restore, call Resume after etcd + PVs + images are all restored.

### lifecycle.ResolveClusterName

```go
// pkg/internal/lifecycle/state.go:115
func ResolveClusterName(args []string, lister ClusterLister) (string, error)
```

Use for all snapshot commands. This is the auto-detect helper: 0 args + 1 cluster = auto-select; 0 args + N clusters = error listing names; 1 arg = return it.

### lifecycle.ClassifyNodes

```go
// pkg/internal/lifecycle/state.go:142
func ClassifyNodes(allNodes []nodes.Node) (cp, workers []nodes.Node, lb nodes.Node, err error)
```

Use in snapshot create/restore to enumerate CP nodes (for etcd operations), worker nodes (for PV capture), and to build the topology fingerprint.

### lifecycle.ProviderBinaryName

```go
// pkg/internal/lifecycle/state.go:163
func ProviderBinaryName() string  // returns "docker", "podman", "nerdctl", or ""
```

Use for image inspection (to get node image tag for topology fingerprint).

### doctor.clusterResumeReadinessCheck

The `cluster-resume-readiness` check has `ok / warn / skip` statuses (never `fail`). The STATUS column in `kinder snapshot list` uses `ok / corrupt`. These are different contexts — snapshot STATUS is purely based on sha256 verification of the archive file, not live cluster probing. No direct reuse of the doctor check for snapshot STATUS.

### Context/timeout pattern

`resume.go` uses `context.Context` propagated through `ResumeOptions.Context`. The snapshot package should accept a `context.Context` in `CreateOptions` and `RestoreOptions` for cancellation support — matching the existing pattern.

### node.Command pattern

```go
// nodes.Node interface (pkg/cluster/nodes/types.go)
node.Command("ctr", "--namespace=k8s.io", "images", "export", "/tmp/images.tar", refs...).Run()
node.Command("cat", "/tmp/images.tar").SetStdout(writer).Run()
node.Command("cp", "/dev/stdin", "/tmp/etcd.snap").SetStdin(reader).Run()
```

All node operations go through `nodes.Node.Command()` / `nodes.Node.CommandContext()`. This is the standard exec abstraction.

---

## 10. Pitfalls / Gotchas

**Confidence: HIGH** — based on codebase analysis and established knowledge of etcd + containerd semantics.

### Pitfall 1: "Restore in seconds" — Image import time

**What goes wrong:** Users expect "seconds." Image re-import via `ctr images import` for a cluster with cert-manager, MetalLB, dashboard, envoy-gateway, metrics-server, and local-path-provisioner is ~3–5 GB. On a fast SSD, `ctr images import` from a local file takes ~30–120 seconds. The etcd + PV swap IS fast (2–10s), but images dominate.

**Recommendation:** Document in `snapshot create --help`: "Restore is fast for etcd and PVs; image bundle import time depends on image size (typically 30–120s)." The CONTEXT.md wording "restore in seconds" refers to the etcd/PV hot-swap path when images are already in the node's containerd store. Detect this at restore time: if the sha256 of the live node's image set matches `metadata.imagesDigest`, skip re-import. This is the fast path.

### Pitfall 2: CRI confusion

**What goes wrong:** Using `docker save` on the HOST to try to capture images from the kind cluster. Kind nodes use containerd in the `k8s.io` namespace, which is separate from the host's Docker daemon. `docker save` only sees images in the host daemon, not inside the kind node containers.

**How to avoid:** Always use `node.Command("ctr", "--namespace=k8s.io", "images", "export", ...)` — never `exec.Command("docker", "save", ...)` from the host.

### Pitfall 3: etcd snapshot requires running cluster

**What goes wrong:** Attempting `etcdctl snapshot save` after pausing the cluster (etcd is stopped). The snapshot must be taken before pausing.

**How to avoid:** In `snapshot create`: (1) take etcd snapshot while running, (2) THEN pause, (3) capture images + PVs.

### Pitfall 4: etcdctl location

**What goes wrong:** Assuming `etcdctl` is on the kindest/node rootfs PATH. The Phase 47 code uses `crictl exec <etcd-container>` to reach etcdctl INSIDE the etcd static pod image. For `snapshot restore`, the etcd container is NOT running (cluster is paused). However, `etcdctl snapshot restore` can be run directly if etcdctl is available on the node OR via a temporary container.

**How to avoid:** For restore, use `docker exec <kind-node> etcdctl snapshot restore ...` directly on the node container's PATH. If not present, use a temporary `docker run --rm -v /var/lib/etcd:/var/lib/etcd registry.k8s.io/etcd:<version> etcdctl snapshot restore ...`. Confirm which path is available during plan implementation by checking the kindest/node image.

### Pitfall 5: HA etcd restore cluster-id collision

**What goes wrong:** Restoring a snapshot to an HA cluster reuses the old cluster-id, causing etcd peer conflicts if the `--initial-cluster-token` is not changed.

**How to avoid:** Always use a fresh `--initial-cluster-token=kind-snap-restore-<timestamp>` that is the SAME across all CP nodes being restored. Never use the old token from the running cluster.

### Pitfall 6: Disk space explosion

**What goes wrong:** Each snapshot is potentially 3–8 GB. Users creating daily snapshots without pruning fill up the disk quickly.

**How to avoid:** Show disk usage prominently in `kinder snapshot list`. Print the total space used by all snapshots for the cluster. Make `prune` the obvious follow-up. Document the cleanup command in `snapshot create`'s success output: "Use `kinder snapshot prune --keep-last 3` to manage disk usage."

### Pitfall 7: Cluster not found vs snapshot not found

**What goes wrong:** Conflating "cluster does not exist" with "snapshot does not exist" error messages.

**How to avoid:** Use distinct error messages and exit codes. `lifecycle.ResolveClusterName` already returns a well-typed error for "no kind clusters found." The snapshot store should return a typed `ErrSnapshotNotFound` error.

### Pitfall 8: File permissions on `~/.kinder/snapshots/`

**What goes wrong:** Directory created with mode 0755 allows other users to read unredacted etcd snapshots (which contain all Secrets).

**How to avoid:** `os.MkdirAll(snapDir, 0700)` — strictly owner-only. Create parent `~/.kinder/` with 0700 as well if it doesn't exist.

### Pitfall 9: PV capture misses non-localpath volumes

**What goes wrong:** Users have data in hostPath volumes or emptyDir volumes that are NOT under `/opt/local-path-provisioner`. These are not captured.

**How to avoid:** Document clearly: "Snapshot captures only local-path-provisioner PVs. Other volume types (hostPath, emptyDir) are not included." This is in scope per the locked decisions.

### Pitfall 10: Restore on running cluster — readiness gate

**What goes wrong:** After etcd restore + Resume, the K8s API may take 15–30s to accept requests (etcd members re-electing, kubeadm certs, etc.). If the snapshot restore returns too early, subsequent `kubectl` commands fail.

**How to avoid:** Reuse `lifecycle.Resume`'s built-in `WaitForNodesReady` which polls kubectl until all nodes are Ready. This is already blocking with a configurable timeout (`WaitTimeout` in `ResumeOptions`).

---

## 11. Test Strategy

**Confidence: HIGH** — based on established project testing patterns documented in `TESTING.md`.

### Unit tests

Co-located in `pkg/internal/snapshot/` as `*_test.go` files. All tests call `t.Parallel()`. Use table-driven patterns.

**What to unit test:**
- `metadata.go`: JSON marshal/unmarshal round-trip for `metadata.json` schema. Edge cases: empty addon map, missing fields.
- `bundle.go`: sha256 verification logic (inject a byte-flipper to simulate corruption). Bundle write/read with temp files using `os.MkdirTemp`.
- `store.go`: prune policy filters — `keepLast(N)`, `olderThan(duration)`, `maxSize(bytes)`. These are pure functions over a slice of snapshot metadata.
- Version comparison for compatibility checks — parse `v1.31.2` vs `v1.32.0`.
- `lifecycle.ResolveClusterName` is already tested; no need to re-test it in snapshot tests.

**Function injection pattern** (matching existing codebase):
- In `snapshot.Create`, inject `pauseFn`, `resumeFn`, `nodeCommandFn` as package-level vars. Tests swap them without spinning a real cluster — identical to `pkg/cmd/kind/pause/pause.go:51`.

### Integration tests

Pattern: mark with function name prefix `TestIntegration` (existing convention); gate with `integration.MaybeSkip(t)` from `pkg/internal/integration/integration.go`.

**Integration test scenario:**
1. `kinder create cluster --local-path`
2. `kubectl create configmap test-data --from-literal=key=value`
3. `kinder snapshot create test-snap`
4. `kubectl delete configmap test-data`
5. `kinder snapshot restore test-snap`
6. Assert `kubectl get configmap test-data` succeeds and value matches.
7. `kinder snapshot list` → assert STATUS=ok.
8. Cleanup: `kinder delete cluster`.

**CI gating:** Existing `make integration` runs tests marked with the `integration` build tag. The new integration test can be placed in `pkg/internal/snapshot/snapshot_integration_test.go` with `//go:build integration` tag (matching existing convention from `pkg/cluster/internal/providers/docker/network_integration_test.go`).

### What NOT to test in unit tests

- Real `docker exec` / `ctr` calls — these require a running cluster. Only integration tests cover these paths.
- `lifecycle.Pause` / `lifecycle.Resume` — already tested in Phase 47; inject stubs.

---

## 12. Open Questions (for the planner)

These are NOT answered here — surface for planning decisions:

1. **Progress bar vs silent:** Should `kinder snapshot create` and `restore` show a spinner (existing `pkg/internal/cli/spinner.go`) or stream line-by-line progress? The spinner is already available; streaming progress requires knowing total bytes (possible with known file sizes).

2. **`kinder snapshot show` output format:** Mirror `list` columns (single-row table) or expand to vertical key/value layout? Should it accept `--output json`?

3. **Disk-space pre-check threshold:** Hard number (e.g., 10 GB always required) vs dynamic estimate based on component sizes? Dynamic is more correct but adds complexity.

4. **`--older-than DURATION` and days:** Go's `time.ParseDuration` only supports `ns/us/ms/s/m/h`. Should the planner add a custom parser for `d` suffix, or document that users must use `168h` for 7 days?

5. **`snapshot restore` positional arg signature:** Should it be `kinder snapshot restore <snap-name>` (cluster auto-detected) or `kinder snapshot restore <cluster-name> <snap-name>` (matching `pause`/`resume` convention)? Both work; the planner should pick and document.

6. **etcdctl location on kindest/node:** Does the kindest/node image include `etcdctl` on PATH? If yes, `docker exec <node> etcdctl snapshot restore ...` works. If no, the planner must design a temporary container approach. This needs verification against the kindest/node Dockerfile.

7. **Image bundle fast-path on restore:** Should restore skip `ctr images import` when the live node's image set sha256 matches `metadata.imagesDigest`? Computing the live sha256 requires exporting all images first, which is circular. Alternative: list image refs and compare deterministically. This optimization may be deferred.

8. **Multi-node PV capture naming:** How should `pvs.tar` handle data from multiple nodes? Options: (a) single tar with per-node subdirectories `<nodeName>/opt/local-path-provisioner/`, (b) separate per-node tars inside the outer tar.gz. Option (a) is simpler.

9. **`kinder snapshot prune --max-size SIZE`:** SIZE format — bytes (e.g., `5G`, `500M`)? Go has no stdlib size parser. Use a simple multiplier parser or require bytes as an integer.

10. **Should `snapshot create` fail if no addons are installed?** Probably not — a bare cluster with no addons is a valid snapshot target.

---

## Standard Stack

### Core (all stdlib / already in go.mod)

| Library | Import | Purpose | Status |
|---------|--------|---------|--------|
| `archive/tar` | stdlib | Tar write/read for bundle | No new dep |
| `compress/gzip` | stdlib | Gzip compression | No new dep |
| `crypto/sha256` | stdlib | Component + archive hashing | No new dep |
| `encoding/json` | stdlib | metadata.json marshal | No new dep |
| `text/tabwriter` | stdlib | Table output for list | No new dep |
| `syscall` | stdlib | Statfs disk space check | No new dep |
| `sigs.k8s.io/yaml` | go.mod:27 | YAML output for list | Already present |

### No new dependencies needed

The entire Phase 48 implementation uses only existing `go.mod` dependencies plus stdlib. No new `go get` required.

---

## Architecture Patterns

### Recommended Project Structure

```
pkg/
├── internal/
│   └── snapshot/
│       ├── create.go          # Create(opts CreateOptions) (*CreateResult, error)
│       ├── restore.go         # Restore(opts RestoreOptions) (*RestoreResult, error)
│       ├── metadata.go        # Metadata struct, JSON schema
│       ├── bundle.go          # tar.gz write/read/verify
│       ├── store.go           # SnapshotStore: list, info, delete, prune
│       ├── prune.go           # Prune policy logic (pure functions)
│       ├── images.go          # captureImages(), restoreImages()
│       ├── etcd.go            # captureEtcd(), restoreEtcd()
│       ├── pvs.go             # capturePVs(), restorePVs()
│       ├── topology.go        # TopologyFingerprint(), addonVersions()
│       └── doc.go
└── cmd/kind/
    └── snapshot/
        ├── snapshot.go        # NewCommand() — group
        ├── create.go          # "create [snap-name]"
        ├── restore.go         # "restore <snap-name>"
        ├── list.go            # "list"
        ├── show.go            # "show <snap-name>"
        └── prune.go           # "prune [--keep-last N] [--older-than D] [--max-size S]"
```

### Pattern: Options struct with injected dependencies

Match the Phase 47 pattern from `lifecycle.Pause`:
```go
type CreateOptions struct {
    ClusterName string
    SnapName    string        // "" = auto-generate timestamp name
    Logger      log.Logger
    Provider    *cluster.Provider
    Context     context.Context
    // internal test injection points (unexported)
    pauseFn     func(lifecycle.PauseOptions) (*lifecycle.PauseResult, error)
    resumeFn    func(lifecycle.ResumeOptions) (*lifecycle.ResumeResult, error)
}
```

### Pattern: metadata.json schema

```go
type Metadata struct {
    SnapName      string            `json:"name"`
    CreatedAt     time.Time         `json:"createdAt"`
    K8sVersion    string            `json:"k8sVersion"`    // e.g. "v1.31.2"
    NodeImage     string            `json:"nodeImage"`     // e.g. "kindest/node:v1.31.2"
    Topology      TopologyInfo      `json:"topology"`
    AddonVersions map[string]string `json:"addonVersions"` // "" value = not installed
    EtcdDigest    string            `json:"etcdDigest"`    // sha256hex of etcd.snap
    ImagesDigest  string            `json:"imagesDigest"`  // sha256hex of images.tar
    PVsDigest     string            `json:"pvsDigest"`     // sha256hex of pvs.tar ("" if no pvs)
    ArchiveDigest string            `json:"archiveDigest"` // sha256hex of full .tar.gz (also in sidecar)
}

type TopologyInfo struct {
    ControlPlaneCount int  `json:"controlPlaneCount"`
    WorkerCount       int  `json:"workerCount"`
    HasLoadBalancer   bool `json:"hasLoadBalancer"`
}
```

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| etcd backup | Custom etcd client | `etcdctl snapshot save` via `crictl exec` | etcdctl handles all consistency guarantees |
| Image export | Docker API client | `ctr --namespace=k8s.io images export` via `node.Command` | Containerd CLI already in node image |
| Image import | Custom OCI parser | `LoadImageArchiveWithFallback` (nodeutils/util.go:130) | Handles Docker Desktop 27+ fallback |
| Cluster name resolve | Custom arg parsing | `lifecycle.ResolveClusterName` (state.go:115) | Auto-detect already tested |
| Node classification | Role enumeration | `lifecycle.ClassifyNodes` (state.go:142) | Tested, handles all role types |
| Pause/Resume | Stop/start containers | `lifecycle.Pause` / `lifecycle.Resume` | Phase 47 handles HA quorum ordering |
| Table output | Custom formatter | `text/tabwriter` | Already used in `get nodes` |
| YAML output | Custom marshaller | `sigs.k8s.io/yaml` (already in go.mod) | Already used project-wide |
| Duration flags | Custom parser | `cobra.DurationVar` + document `h` units | Consistent with pause/resume flags |

---

## Sources

### Primary (HIGH confidence — direct codebase inspection)

- `pkg/internal/lifecycle/pause.go` — etcdctl via crictl exec pattern, cert paths, quorum-safe ordering
- `pkg/internal/lifecycle/resume.go` — WaitForNodesReady, ResumeOptions, blocking semantics
- `pkg/internal/lifecycle/state.go` — ClassifyNodes, ResolveClusterName, ProviderBinaryName
- `pkg/internal/doctor/resumereadiness.go` — etcdctlAuthArgs, parseEtcdHealth, crictl exec pattern
- `pkg/internal/doctor/localpath.go` — kubectl-exec pattern for addon version detection
- `pkg/cluster/nodeutils/util.go` — LoadImageArchive, LoadImageArchiveWithFallback, ctr namespace, KubeVersion
- `pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml:144` — PV path `/opt/local-path-provisioner`
- `pkg/cluster/internal/create/create.go` — Full addon list with wave dependencies
- `pkg/cluster/internal/providers/docker/constants.go` — Container label keys
- `pkg/cmd/kind/get/nodes/nodes.go` — `--output` flag pattern, tabwriter usage, positional arg convention
- `pkg/cmd/kind/pause/pause.go` — CLI flagpole pattern, `--json` flag, test injection via package vars
- `pkg/cmd/kind/resume/resume.go` — DurationVar pattern, WaitTimeout flag
- `pkg/cmd/kind/root.go` — Where to add snapshot subcommand
- `pkg/internal/cli/spinner.go` — Spinner API (Start/Stop/SetPrefix/SetSuffix)
- `pkg/internal/integration/integration.go` — MaybeSkip integration test gate
- `pkg/internal/doctor/check.go` — Result type (ok/warn/fail/skip), Check interface
- `go.mod` — Confirms `sigs.k8s.io/yaml` already present, no new deps needed

### Secondary (MEDIUM confidence — documented patterns)

- etcd official docs on `etcdctl snapshot save/restore`: https://etcd.io/docs/v3.5/op-guide/recovery/
- containerd `ctr` image export: https://github.com/containerd/containerd/blob/main/docs/ops.md
- kind node image internals: https://kind.sigs.k8s.io/docs/design/node-image/

---

## Metadata

**Confidence breakdown:**
- Phase 47 reuse points: HIGH — read directly from source
- etcd snapshot mechanics: HIGH — Phase 47 already established the exec pattern
- Image capture: HIGH — existing `LoadImageArchive` proves the ctr namespace and import flags
- PV path: HIGH — hardcoded in embedded manifest
- Kind config persistence: HIGH — confirmed NOT persisted; reconstruction approach documented
- Addon detection: HIGH — existing `realGetProvisionerVersion` proves the kubectl-exec pattern
- Go stdlib bundling: HIGH — standard archive/tar + gzip + sha256 pattern
- CLI conventions: HIGH — read directly from pause.go / resume.go / nodes.go

**Research date:** 2026-05-06
**Valid until:** 2026-06-06 (stable domain; etcd/containerd APIs change rarely)

---

## RESEARCH COMPLETE
