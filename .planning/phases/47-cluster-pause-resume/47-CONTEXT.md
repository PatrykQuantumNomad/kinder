# Phase 47: Cluster Pause/Resume - Context

**Gathered:** 2026-05-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Two CLI lifecycle commands — `kinder pause [name]` and `kinder resume [name]` — that stop and start all cluster containers in a quorum-safe order so users can reclaim laptop CPU/RAM and bring the cluster back to the same state. Includes a `cluster-resume-readiness` check registered in the v2.1 doctor catalog and a status column on the existing cluster-list output. A new `kinder status [name]` command surfaces detailed per-node state.

Snapshot/restore (Phase 48), hot-reload (Phase 49), and runtime error decoding (Phase 50) are explicitly out of scope.

</domain>

<decisions>
## Implementation Decisions

### Command UX & output
- **No-arg behavior:** auto-detect — if exactly one cluster exists, pause/resume it; if multiple, error and list them; if none, error.
- **Output style:** per-node line as each node stops/starts (matches existing `kinder create` style), plus a final total-time line.
- **Idempotency:** running `pause` on a paused cluster (or `resume` on a running cluster) is a no-op with a warning, exit 0. Friendly for scripted use.
- **JSON output:** `--json` flag supported on both commands, full parity with the v1.4 phase-29 JSON output convention. Emit node list, per-node timings, final state.

### Multi-node orchestration
- **Underlying primitive:** `docker stop` / `docker start`. Containers fully exit so RAM is released — required for the "CPU and RAM drop to near-zero" success criterion. (`docker pause`/`unpause` does not release RAM and is rejected.)
- **Graceful timeout:** 30s default before SIGKILL, overridable via `--timeout` flag. Generous enough for kubelet/etcd flush.
- **Per-node failure handling:** continue best-effort — keep operating on remaining nodes, report all successes/failures at the end. Cluster may be left in partial state; user is informed.
- **Resume completion gate:** wait for all nodes to show `Ready` via the Kubernetes API (with a timeout) before exiting success. Containers-running-only is not enough.
- **Quorum-safe order** (already locked by ROADMAP success criterion 3): on pause, workers stop before control-plane nodes; on resume, control-plane nodes start before workers.

### State visibility & persistence
- **Source of truth:** Docker container status. No separate state file, no kinder-managed marker. A cluster is "paused" when all its containers are stopped. Survives host reboots automatically.
- **Cluster list integration:** add a `Status` column to the existing cluster-list output with values like `Running`, `Paused`, `Error`.
- **New `kinder status [name]` command:** in scope for this phase. Shows per-node container state, k8s version, paused/running, last pause time (where derivable). Useful for HA debugging.
- **Crash vs intentional pause:** no distinction. `kinder resume` treats any stopped state as resumable — simple, forgiving, and consistent with the "container status only" source of truth.

### Doctor readiness check
- **When it runs:** automatically inline on `kinder resume` for HA (multi-control-plane) clusters only. Single-CP clusters skip the inline check. Also runnable as part of a full `kinder doctor` invocation.
- **What it checks:** both — (a) a pre-pause snapshot of etcd member health/leader recorded at pause time, and (b) post-start verification that all etcd containers came up before kubelet starts.
- **Default behavior on problem:** warn and continue. The check emits a `cluster-resume-readiness` warning but does not block resume — matches the ROADMAP success-criterion language ("emits a warning").
- **Doctor catalog registration:** yes — register as a regular check in the v2.1 doctor framework so it's reusable from `kinder doctor` runs, not just resume-internal logic.

### Claude's Discretion
- Exact format of the per-node output line (icons, color, spacing) — match the existing `kinder create` aesthetic.
- Where the pre-pause etcd snapshot is stored on disk (container labels vs sidecar file) — pick whatever fits cleanest given the rest of the kinder code.
- Output schema details for `kinder status` and the `Status` column ordering — Claude picks based on existing list-command structure.
- Exact threshold for "etcd quorum at risk" beyond the basic member-health/container-restart signals.
- Whether `--timeout` is global or per-step (pause-graceful, resume-ready, etc.).

</decisions>

<specifics>
## Specific Ideas

- Reuse the existing v2.1 doctor framework for `cluster-resume-readiness` rather than introducing a parallel check mechanism.
- The "auto-detect single cluster" no-arg behavior should be consistent across `pause`, `resume`, and `status` — one ergonomic rule, not three different ones.
- JSON output should follow whatever schema convention v1.4 phase 29 established for `kinder create --json` etc.
- "All nodes Ready" gate on resume means using the kubeconfig that kinder already manages — no new connection plumbing needed.

</specifics>

<deferred>
## Deferred Ideas

- Snapshot/restore of cluster state — Phase 48 (already in v2.3 roadmap).
- Auto-rollback of partial pause/resume failures (restart already-stopped nodes if pause fails mid-flight) — considered and rejected for this phase in favor of "continue best-effort + report". Could be revisited in a future hardening phase if users hit it in practice.
- Distinguishing intentional pause from host-crash stop (via container labels or state file) — rejected for this phase. Could be added later if "resume after crash" turns out to be unsafe in practice.
- `--freeze` opt-in flag for `docker pause`/`unpause` semantics (fast unfreeze, RAM not released) — out of scope; success criterion explicitly requires RAM drop.

</deferred>

---

*Phase: 47-cluster-pause-resume*
*Context gathered: 2026-05-03*
