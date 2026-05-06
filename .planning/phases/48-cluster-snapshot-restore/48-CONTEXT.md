# Phase 48: Cluster Snapshot/Restore - Context

**Gathered:** 2026-05-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Capture a complete kinder cluster state — etcd, all loaded container images, local-path-provisioner PV contents, the original kind config YAML, and identity/integrity metadata — as a named, restorable archive on the host filesystem. Provide lifecycle commands `create / restore / list / show / prune`. Restore refuses on Kubernetes-version mismatch (SC2) and on cluster-topology / addon-version drift. Cross-cluster snapshot transfer, scheduled snapshots, and snapshot upload to remote storage are out of scope for this phase.

</domain>

<decisions>
## Implementation Decisions

### Storage layout & format
- Snapshots live at `~/.kinder/snapshots/<cluster>/` — per-cluster subdirectory under user home; survives `docker system prune`.
- Single `.tar.gz` bundle per snapshot containing: `etcd.snap`, `images.tar`, `pvs.tar`, `kind-config.yaml`, `metadata.json`. Component layout decided at planning time; archive is one file for easy copy/share.
- Snapshots are **cluster-scoped**: every command implicitly operates on the named/current cluster; `kinder snapshot list` shows snapshots for that cluster only.
- Naming: `kinder snapshot create [<name>]` — name is **optional**; if omitted, default to `snap-YYYYMMDD-HHMMSS` (timestamp-based, never auto-overwriting).

### Capture scope & metadata
- The original kind cluster config YAML **is captured** inside the archive (`kind-config.yaml`). Snapshot is fully self-describing for future restore-into-fresh-cluster scenarios.
- Secrets/ConfigMaps captured **as-is inside the etcd snapshot** — no redaction, no warning. Snapshots are local-dev artifacts; document the implication in the snapshot command help / website docs (not required as runtime UX).
- `metadata.json` carries SC4 fields (cluster K8s version, addon versions, image-bundle digest) PLUS:
  - SHA-256 of the full `.tar.gz` archive (sidecar `<name>.tar.gz.sha256`).
  - Per-component SHA-256 digests (etcd.snap, images.tar, pvs.tar) inside `metadata.json`.
- Image-bundle digest mismatch on restore: **always re-load images from `images.tar`** — the archive is the source of truth. Local docker image cache is treated as untrusted.

### Restore semantics & safety
- `kinder snapshot restore` works on a **running** cluster: implicitly pause (reuse Phase 47 `lifecycle.Pause`), restore etcd + PVs + image bundle, resume (reuse Phase 47 `lifecycle.Resume`). One-shot UX, no separate `kinder pause` step required.
- **Hard overwrite, no prompt** — `restore` is the action verb; expectation is the cluster is wiped to snapshot state. No interactive confirmation, no `--yes` flag needed.
- **Atomicity = pre-flight + fail-fast, no rollback.** Validate archive integrity (sha256), free disk space, K8s/topology/addon compatibility, etcd reachability BEFORE any mutation. If a later step still fails, surface the error with a clear recovery hint and leave the cluster in its current (inconsistent) state. No implicit pre-restore snapshot.
- Compatibility checks that **hard-fail** restore:
  - K8s version mismatch (SC2 minimum, mandatory).
  - Cluster topology mismatch (node count or roles differ from `metadata.json`).
  - Addon-version mismatch (any installed addon's version differs).
  All three are hard-fail; no warn-only path. User must recreate the cluster or pick a matching snapshot.

### Lifecycle UX (list / show / prune)
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

</decisions>

<specifics>
## Specific Ideas

- "Restore in seconds" (ROADMAP goal) — design must keep the warm-restore path on a running cluster fast: pause → swap etcd + PVs → resume should be the inner loop, not "destroy & recreate cluster".
- Reuse Phase 47 `lifecycle.Pause` / `lifecycle.Resume` directly inside the restore flow rather than re-implementing quorum-safe ordering.
- Match Phase 47's CLI conventions wherever possible: positional cluster argument, `--output`/`--json`, `DurationVar` for time flags (e.g. `--timeout` if added during planning).
- The `cluster-resume-readiness` Phase 47-04 doctor check semantics inform `STATUS` UX: prefer `ok / warn / corrupt` over binary states if integrity is partially verifiable.

</specifics>

<deferred>
## Deferred Ideas

- **Snapshot diff** (`kinder snapshot diff <a> <b>`): not in this phase; potential future UX once the on-disk format settles.
- **Cross-cluster restore** (snapshot from cluster-A into cluster-B): explicitly out of scope; current decisions assume same cluster name and topology.
- **Scheduled snapshots / retention policy daemon**: out of scope; users invoke `prune` manually.
- **Remote storage backends (S3, OCI registry, GCS)**: out of scope; archives live on the local filesystem only. Could become its own phase if user demand emerges.
- **`--redact-secrets` flag**: deferred. Captured-as-is is the v2.3 behavior; redaction can be added later as an additive flag if a real use case appears.
- **Auto pre-restore rollback snapshot**: deferred. Phase 48 ships fail-fast with manual rollback only; a future phase may add a `--rollback-on-failure` flag.
- **Pause/Resume of in-flight restore (signal handling)**: not in scope; if the user Ctrl-Cs mid-restore, the failure mode is the same as any other partial-restore failure.

</deferred>

---

*Phase: 48-cluster-snapshot-restore*
*Context gathered: 2026-05-06*
