# Phase 47: Cluster Pause/Resume - Research

**Researched:** 2026-05-03
**Domain:** Go CLI lifecycle commands, Docker stop/start, etcd quorum, Kubernetes node readiness, doctor check framework
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Command UX & output**
- No-arg behavior: auto-detect — if exactly one cluster exists, pause/resume it; if multiple, error and list them; if none, error.
- Output style: per-node line as each node stops/starts (matches existing `kinder create` style), plus a final total-time line.
- Idempotency: running `pause` on a paused cluster (or `resume` on a running cluster) is a no-op with a warning, exit 0.
- JSON output: `--json` flag supported on both commands, full parity with the v1.4 phase-29 JSON output convention.

**Multi-node orchestration**
- Underlying primitive: `docker stop` / `docker start`. Containers fully exit so RAM is released.
- Graceful timeout: 30s default before SIGKILL, overridable via `--timeout` flag.
- Per-node failure handling: continue best-effort — keep operating on remaining nodes, report all successes/failures at the end.
- Resume completion gate: wait for all nodes to show `Ready` via the Kubernetes API (with a timeout) before exiting success.
- Quorum-safe order: on pause, workers stop before control-plane nodes; on resume, control-plane nodes start before workers.

**State visibility & persistence**
- Source of truth: Docker container status. No separate state file, no kinder-managed marker.
- Cluster list integration: add a `Status` column to the existing cluster-list output with values like `Running`, `Paused`, `Error`.
- New `kinder status [name]` command: in scope for this phase.
- Crash vs intentional pause: no distinction.

**Doctor readiness check**
- When it runs: automatically inline on `kinder resume` for HA (multi-control-plane) clusters only. Single-CP clusters skip.
- What it checks: (a) pre-pause snapshot of etcd member health/leader recorded at pause time, and (b) post-start verification that all etcd containers came up before kubelet starts.
- Default behavior on problem: warn and continue.
- Doctor catalog registration: yes — register as a regular check in the v2.1 doctor framework.

### Claude's Discretion
- Exact format of the per-node output line (icons, color, spacing) — match existing `kinder create` aesthetic.
- Where the pre-pause etcd snapshot is stored on disk (container labels vs sidecar file) — pick whatever fits cleanest.
- Output schema details for `kinder status` and the `Status` column ordering.
- Exact threshold for "etcd quorum at risk" beyond the basic member-health/container-restart signals.
- Whether `--timeout` is global or per-step.

### Deferred Ideas (OUT OF SCOPE)
- Snapshot/restore of cluster state — Phase 48.
- Auto-rollback of partial pause/resume failures.
- Distinguishing intentional pause from host-crash stop via container labels.
- `--freeze` opt-in flag for `docker pause`/`unpause` semantics.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LIFE-01 | User can pause a running cluster via `kinder pause [name]`, freeing CPU/RAM without losing state | `docker stop --time=30 <container>` per node, workers before CP; container state verified via `docker inspect --format {{.State.Status}}` |
| LIFE-02 | User can resume a paused cluster via `kinder resume [name]`; pods, PVs, and node state are preserved | `docker start <container>` per node, CP before workers; wait for all nodes `Ready` via kubectl inside CP node using existing `waitForReady` pattern |
| LIFE-03 | Pause-resume orchestrates control-plane and worker stop/start in correct order to preserve etcd quorum on HA clusters | `nodeutils.ControlPlaneNodes()` and `SelectNodesByRole("worker")` already exist; ordering is trivial sequential slices |
| LIFE-04 | Doctor pre-flight check `cluster-resume-readiness` runs before resume on HA clusters and warns if etcd quorum is at risk | Implement as `Check` interface in `pkg/internal/doctor`; register in `allChecks`; inline call from resume logic |
</phase_requirements>

---

## Summary

Phase 47 is a pure Go CLI extension — no new external dependencies required. All primitives needed are already in the codebase: `exec.Command("docker", ...)` for stop/start, `nodeutils.ControlPlaneNodes()` / `SelectNodesByRole("worker")` for quorum-safe ordering, the existing `waitForReady` pattern (kubectl inside the CP node) for the resume gate, and the `Check` interface for the doctor integration.

The primary new construct is a `pkg/cluster/internal/lifecycle/` package containing `Pause()` and `Resume()` functions, analogous to the existing `pkg/cluster/internal/delete/` package. Each function orchestrates per-node container stop/start in the correct order, emitting per-node progress lines matching the `cli.Status` aesthetic used in `kinder create`. The three new top-level commands (`pause`, `resume`, `status`) are registered in `pkg/cmd/kind/root.go` following the same pattern as every other top-level command.

The `cluster-resume-readiness` doctor check is an HA-only check that receives a cluster name at construction time (or discovers the default cluster like `clusterNodeSkewCheck` does) and invokes `docker exec <cp-node> etcdctl ...` to verify member health. The check registers in `allChecks` and is also called inline by `Resume()` before starting kubelet on HA clusters. Pre-pause etcd snapshot storage uses Docker container labels on the CP node (key: `io.kinder.etcd-leader-id`, `io.kinder.pause-time`) — no files, no extra containers, survives host reboots.

**Primary recommendation:** Implement lifecycle functions in a new `pkg/cluster/internal/lifecycle/` package. Commands in `pkg/cmd/kind/{pause,resume,status}/`. Doctor check in `pkg/internal/doctor/resumereadiness.go`. Extend `kinder get clusters` and `kinder get nodes` Status columns to reflect actual container state from `docker ps` format.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Container stop/start (pause/resume primitives) | CLI / Host | — | `docker stop`/`docker start` run on host, manipulate containers |
| Quorum-safe ordering (workers vs CP) | CLI / Host | — | Pure Go: slice-ordering via existing `nodeutils` functions |
| Node readiness gate after resume | CLI → Container | — | kubectl inside CP container (same as `waitforready` action) |
| etcd member health check | CLI → Container | Doctor framework | `docker exec <cp> etcdctl endpoint health` run from CLI into container |
| Pre-pause etcd snapshot storage | Container labels | — | Docker labels on CP containers; no external storage needed |
| Doctor check registration | Doctor framework | CLI (inline call) | `allChecks` registry in `pkg/internal/doctor/check.go` |
| Status column (cluster list) | CLI (get clusters) | — | Compute status by querying container state for each node |
| Status column (node list) | CLI (get nodes) | — | Currently hardcoded "Ready" — fix to use real container state |
| `kinder status` command | CLI (new command) | — | Detailed per-node state, parallels `kinder get nodes` but lifecycle-focused |

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `sigs.k8s.io/kind/pkg/exec` | internal | Run `docker stop`/`docker start`/`docker inspect` | All Docker calls in this codebase go through `exec.Command(...)` — never `os/exec` directly |
| `sigs.k8s.io/kind/pkg/cluster/nodeutils` | internal | `ControlPlaneNodes()`, `SelectNodesByRole()`, `InternalNodes()` | Already provides all node-role filtering needed for quorum ordering |
| `sigs.k8s.io/kind/pkg/internal/doctor` | internal | `Check` interface, `allChecks` registry, `Result` struct | The v2.1 doctor framework this phase extends |
| `sigs.k8s.io/kind/pkg/internal/cli` | internal | `cli.Status` for spinner/per-node output lines | Used by all existing multi-node operations |
| `encoding/json` | stdlib | `--json` output | Same as phase-29 convention: `json.NewEncoder(streams.Out).Encode(v)` |
| `github.com/spf13/cobra` | v1.8.0 | Top-level command registration | All CLI commands use cobra |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/sync/errgroup` | v0.19.0 | Concurrent operations | Already in go.mod; used in create.go for parallel addons. Not needed for pause/resume (sequential per quorum safety) but available if needed for parallel status probing |
| `text/tabwriter` | stdlib | Status column alignment in `kinder status` and `kinder get clusters` | Same pattern as `kinder get nodes` tabwriter |
| `time` | stdlib | `--timeout` flag, per-node timing, total-time line | |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `docker stop --time=30` (CLI subprocess) | Docker SDK (`github.com/docker/docker/client`) | Docker SDK not in go.mod; adding it pulls in large dependency; CLI subprocess is established pattern throughout this codebase |
| Container labels for pre-pause snapshot | Sidecar JSON file in container | Labels survive host reboots with no file management; accessible via `docker inspect` without exec; simpler for the single leader-id we need |
| Sequential node stop/start | Concurrent stop/start | Quorum safety requires sequential for CP nodes; workers can be parallel in theory but sequential is simpler and correct |

**No new external dependencies.** Everything in go.mod already.

---

## Architecture Patterns

### Recommended Project Structure

```
pkg/cmd/kind/pause/              # new: kinder pause [name]
    pause.go
pkg/cmd/kind/resume/             # new: kinder resume [name]
    resume.go
pkg/cmd/kind/status/             # new: kinder status [name]
    status.go
pkg/cluster/internal/lifecycle/  # new: pause/resume business logic
    lifecycle.go
    lifecycle_test.go
pkg/internal/doctor/
    resumereadiness.go           # new: cluster-resume-readiness check
    resumereadiness_test.go      # new: check tests
```

Modified files:
```
pkg/cmd/kind/root.go             # register pause/resume/status commands
pkg/cmd/kind/get/clusters/clusters.go  # add Status column
pkg/cmd/kind/get/nodes/nodes.go       # fix Status column (currently hardcoded "Ready")
pkg/internal/doctor/check.go          # add newClusterResumeReadinessCheck() to allChecks
```

### System Architecture Diagram

```
kinder pause [name]
     │
     ├── resolve cluster name (auto-detect if 0 args)
     ├── ListNodes(name) → []Node
     ├── classify nodes by role (CP vs worker vs LB)
     ├── idempotency check: all containers stopped? → warn + exit 0
     │
     ├── [HA only] capture etcd snapshot → store in CP container label
     │      docker exec <cp0> etcdctl --cacert=... --cert=... --key=...
     │             endpoint status --cluster → JSON → extract leader ID
     │      docker label <cp0> io.kinder.etcd-leader-id=<id>
     │      docker label <cp0> io.kinder.pause-time=<RFC3339>
     │
     ├── for each worker (sequential): docker stop --time=30 <worker>
     │      emit " ✓ Stopped <worker-name> (role: worker)"
     │
     ├── for each CP node (sequential): docker stop --time=30 <cp>
     │      emit " ✓ Stopped <cp-name> (role: control-plane)"
     │
     └── emit "Cluster paused. Total time: Xs"

kinder resume [name]
     │
     ├── resolve cluster name (auto-detect if 0 args)
     ├── ListNodes(name) → []Node
     ├── idempotency check: all containers running? → warn + exit 0
     │
     ├── for each CP node (sequential): docker start <cp>
     │      emit " ✓ Started <cp-name> (role: control-plane)"
     │
     ├── [HA only] run cluster-resume-readiness check inline
     │      exec etcdctl endpoint health on each CP
     │      emit warn if any member unhealthy
     │
     ├── for each worker (sequential): docker start <worker>
     │      emit " ✓ Started <worker-name> (role: worker)"
     │
     ├── wait for all nodes Ready (kubectl inside CP, timeout)
     │      re-use tryUntil + waitForReady logic from waitforready action
     │
     └── emit "Cluster resumed. Total time: Xs"

kinder status [name]
     │
     ├── resolve cluster name
     ├── ListNodes(name) → []Node
     ├── for each node: docker inspect --format {{.State.Status}} <node>
     └── tabwriter output: NAME | ROLE | CONTAINER-STATE | K8S-STATUS | VERSION

kinder get clusters (modified)
     │
     ├── List() → []string (cluster names)
     ├── for each cluster: derive status from container states
     │      all running → "Running"
     │      all stopped → "Paused"
     │      mixed       → "Error"
     └── tabwriter: NAME | STATUS   (human) or {name,status} JSON array

kinder doctor (existing, extended)
     └── allChecks includes newClusterResumeReadinessCheck()
              only emits non-skip result if HA cluster detected
```

### Pattern 1: Container State Query

**What:** Determine if a container is running or stopped via `docker inspect`.
**When to use:** Idempotency check in pause/resume; Status column in list/status commands.

```go
// Source: verified from docker provider inspect patterns in provider.go
func containerState(binaryName, containerName string) (string, error) {
    lines, err := exec.OutputLines(exec.Command(
        binaryName, "inspect",
        "--format", "{{.State.Status}}",
        containerName,
    ))
    if err != nil || len(lines) == 0 {
        return "", errors.Wrap(err, "failed to inspect container state")
    }
    return lines[0], nil // "running", "exited", "paused", "dead"
}
```

### Pattern 2: Sequential Node Stop (quorum-safe pause)

**What:** Stop workers first, then CP nodes, one at a time, with a configurable timeout.
**When to use:** `kinder pause`.

```go
// Source: derived from nerdctl DeleteNodes pattern (provider.go:172-202)
// and docker provider ListNodes pattern (provider.go:115-134)
func stopNode(binaryName, containerName string, timeoutSecs int, logger log.Logger) error {
    logger.V(0).Infof(" • Stopping %s ...", containerName)
    start := time.Now()
    if err := exec.Command(
        binaryName, "stop",
        fmt.Sprintf("--time=%d", timeoutSecs),
        containerName,
    ).Run(); err != nil {
        return errors.Wrapf(err, "failed to stop %s", containerName)
    }
    logger.V(0).Infof(" \x1b[32m✓\x1b[0m Stopped %s (%.1fs)", containerName, time.Since(start).Seconds())
    return nil
}
```

### Pattern 3: Sequential Node Start (quorum-safe resume)

**What:** Start CP nodes first, then workers.
**When to use:** `kinder resume`.

```go
// Source: docker start is direct CLI analog to docker stop
func startNode(binaryName, containerName string, logger log.Logger) error {
    logger.V(0).Infof(" • Starting %s ...", containerName)
    start := time.Now()
    if err := exec.Command(binaryName, "start", containerName).Run(); err != nil {
        return errors.Wrapf(err, "failed to start %s", containerName)
    }
    logger.V(0).Infof(" \x1b[32m✓\x1b[0m Started %s (%.1fs)", containerName, time.Since(start).Seconds())
    return nil
}
```

### Pattern 4: Node Readiness Gate

**What:** Wait for all nodes Ready using `kubectl --kubeconfig=/etc/kubernetes/admin.conf` inside the CP node.
**When to use:** End of `kinder resume`, before reporting success.

```go
// Source: verified copy of waitforready.go pattern
// pkg/cluster/internal/create/actions/waitforready/waitforready.go:103-133
// Re-use tryUntil and the same kubectl command pattern.
// Key: kubectl is inside the CP node container; no host-side kubeconfig needed.
node.CommandContext(ctx,
    "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
    "get", "nodes",
    "--selector=node-role.kubernetes.io/control-plane",
    "-o=jsonpath='{.items..status.conditions[-1:].status}'",
)
```

### Pattern 5: Doctor Check Registration

**What:** Add new check to `allChecks` in `pkg/internal/doctor/check.go`.
**When to use:** Registering `cluster-resume-readiness`.

```go
// Source: verified from pkg/internal/doctor/check.go:53-88
// The check implements the Check interface:
//   Name() string        → "cluster-resume-readiness"
//   Category() string    → "Cluster"
//   Platforms() []string → nil (all platforms)
//   Run() []Result       → skip if no HA cluster; warn if etcd member unhealthy

// Register in allChecks after existing Cluster category entries:
// newClusterNodeSkewCheck(),
// newLocalPathCVECheck(),
// newClusterResumeReadinessCheck(),   // ← new
```

### Pattern 6: etcd Health Check Inside Container

**What:** Run etcdctl inside the CP node container to verify quorum.
**When to use:** `cluster-resume-readiness` check; both inline at resume time and in `kinder doctor`.

```go
// Source: etcd PKI paths from kubeadminit/init.go:114-116, verified
// etcdctl is shipped with kindest/node images [ASSUMED: etcdctl available in PATH inside container]
node.Command(
    "etcdctl",
    "--cacert=/etc/kubernetes/pki/etcd/ca.crt",
    "--cert=/etc/kubernetes/pki/etcd/peer.crt",
    "--key=/etc/kubernetes/pki/etcd/peer.key",
    "--endpoints=https://127.0.0.1:2379",
    "endpoint", "health",
    "--cluster", "--write-out=json",
)
```

### Pattern 7: JSON Output Convention (phase-29 / v1.4)

**What:** `--json` bool flag (NOT `--output json` — the CONTEXT specifies `--json` flag for pause/resume).
**When to use:** `kinder pause --json`, `kinder resume --json`.

```go
// Source: verified from phase-29 RESEARCH.md and get/nodes implementation
// Convention: json.NewEncoder(streams.Out).Encode(v)
// The pause/resume JSON output struct:
type pauseResult struct {
    Cluster  string       `json:"cluster"`
    State    string       `json:"state"` // "paused" or "resumed"
    Nodes    []nodeResult `json:"nodes"`
    Duration float64      `json:"durationSeconds"`
}
type nodeResult struct {
    Name     string  `json:"name"`
    Role     string  `json:"role"`
    Success  bool    `json:"success"`
    Error    string  `json:"error,omitempty"`
    Duration float64 `json:"durationSeconds"`
}
```

**Note on `--json` vs `--output json`:** The CONTEXT explicitly says "`--json` flag supported on both commands". This is a bool flag (`--json`), unlike `kinder get clusters` which uses `--output json` (a string flag). The CONTEXT's specific wording "full parity with the v1.4 phase-29 JSON output convention" means the schema/encoding convention, not the flag form.

### Pattern 8: Auto-detect Cluster Name (no-arg behavior)

**What:** If `args` is empty, call `provider.List()` — if len==1 use it, otherwise error.
**When to use:** `kinder pause`, `kinder resume`, `kinder status` with no positional arg.

```go
// Source: derived from delete/clusters pattern (deleteclusters.go) and CONTEXT.md
func resolveClusterName(args []string, provider *cluster.Provider) (string, error) {
    if len(args) == 1 {
        return args[0], nil
    }
    clusters, err := provider.List()
    if err != nil {
        return "", err
    }
    switch len(clusters) {
    case 0:
        return "", errors.New("no kind clusters found")
    case 1:
        return clusters[0], nil
    default:
        return "", errors.Errorf("multiple clusters found; specify one: %s", strings.Join(clusters, ", "))
    }
}
```

### Anti-Patterns to Avoid

- **Using `docker pause`/`docker unpause`:** Does NOT release RAM. Rejected by user decision. Use `docker stop`/`docker start` only.
- **Stopping CP nodes before workers:** Breaks etcd quorum if workers are still running. Always stop workers first.
- **Only checking containers-running as resume success:** The resume gate is "all nodes Ready via K8s API" — not just containers started. Re-use `waitForReady` logic.
- **Introducing a state file:** Source of truth is Docker container status only. No `~/.kinder/clusters/name/paused` files.
- **Skipping the LB node:** The external-load-balancer container (for HA clusters) must also be stopped/started. Include it, order: workers → CP → LB on pause; LB → CP → workers on resume.
- **Hardcoding "Ready" in get nodes Status:** The current `status := "Ready"` in `pkg/cmd/kind/get/nodes/nodes.go:230` must be replaced with a real container state query.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Node role filtering | Custom label parsing | `nodeutils.ControlPlaneNodes()`, `SelectNodesByRole()`, `InternalNodes()` | Already handles all roles; tested; handles sort for deterministic bootstrap node |
| Container exec | `os/exec.Command("docker", ...)` directly | `exec.Command(...)` from `sigs.k8s.io/kind/pkg/exec` | Provides `RunError` with output capture; consistent error wrapping |
| Quorum check framework | Custom health endpoint logic | `Check` interface in `pkg/internal/doctor` | Framework provides skip/warn/fail/ok status; JSON output; platform filtering; format consistency |
| Kubernetes node wait | Custom polling loop | `waitForReady` + `tryUntil` pattern from `waitforready.go` | Already handles context cancellation, 500ms poll interval, kubectl-inside-node invocation |
| Concurrent error collection | Custom goroutine fan-out | `errors.AggregateConcurrent` / `errors.UntilErrorConcurrent` | Already handles WaitGroup + channel; aggregate errors |
| Per-node spinner output | Custom terminal output | `cli.Status.Start()` / `cli.Status.End()` | Provides colored ✓/✗ symbols, spinner on TTY, plain on non-TTY |

**Key insight:** This phase is 90% wiring — all hard primitives exist. The new code is orchestration logic and command registration.

---

## Runtime State Inventory

> This is a greenfield lifecycle command addition, not a rename/refactor. No runtime state migration required.

None — verified. No existing stored data, live service config, OS-registered state, secrets, or build artifacts carry a "pause/resume" string that would need changing. The `io.kinder.etcd-leader-id` and `io.kinder.pause-time` labels introduced by this phase are new, not a rename of existing labels.

---

## Common Pitfalls

### Pitfall 1: Load Balancer Node Ordering
**What goes wrong:** HA clusters have an `external-load-balancer` container in addition to CP and worker nodes. If it is not included in the stop/start sequence, the HAProxy/Envoy LB keeps running during pause (wasting resources) or is not started on resume (breaking API access from the host).
**Why it happens:** `nodeutils.InternalNodes()` excludes the LB node. `ListNodes()` returns all including LB.
**How to avoid:** Use `p.ListNodes(name)` (all nodes), then classify: workers, CP, LB separately. Stop order: workers → CP → LB. Start order: LB → CP → workers.
**Warning signs:** HA cluster resume leaves kubectl unreachable from host even though K8s reports Ready.

### Pitfall 2: Restart Policy Conflict
**What goes wrong:** Kind containers are created with `--restart=on-failure:1`. After `docker stop`, on a host reboot Docker daemon restarts the container automatically (this is actually desirable). But calling `docker stop` directly without changing the restart policy means Docker will NOT restart on failure (only on daemon restart), which is the intended behavior for pause.
**Why it happens:** `on-failure:1` means restart once on failure — but `docker stop` sends SIGTERM then SIGKILL, which is a clean exit (code 0 or 143), not a failure. Docker treats this as normal exit, no restart.
**How to avoid:** No action needed — `docker stop` does not trigger the `on-failure` restart policy. Unlike the nerdctl DeleteNodes which first sets `--restart=no` before stopping, pause intentionally retains the restart policy so `docker start` works cleanly later.
**Warning signs:** If containers restart immediately after pause completes, check the restart policy.

### Pitfall 3: etcdctl Not in PATH Inside Container
**What goes wrong:** `etcdctl` may not be in the system PATH inside kindest/node containers but exists at a non-standard path.
**Why it happens:** kindest/node ships etcdctl as part of the Kubernetes component bundle; the exact location varies by K8s version. Common path: `/usr/local/bin/etcdctl`.
**How to avoid:** Use absolute path `/usr/local/bin/etcdctl` in the node command, or fall back to `skip` result if etcdctl is not found (check via `which etcdctl` before invoking). The check should gracefully degrade to `skip` rather than `fail` if etcdctl is unavailable.
**Warning signs:** `cluster-resume-readiness` check produces `fail` on all clusters regardless of health.

### Pitfall 4: waitForReady Selector Label Version Skew
**What goes wrong:** The `waitForReady` action contains a version-dependent selector: clusters on K8s < 1.24.0-alpha use `node-role.kubernetes.io/master` instead of `node-role.kubernetes.io/control-plane`.
**Why it happens:** Label was renamed in K8s 1.24.
**How to avoid:** Copy the version-detection logic from `waitforready.go:76-89` when implementing the resume readiness gate. Do not hardcode `control-plane`.
**Warning signs:** Resume times out on old K8s versions because kubectl returns empty results.

### Pitfall 5: Best-Effort Failure Reporting
**What goes wrong:** The CONTEXT requires continuing past per-node failures ("best-effort") but the current codebase pattern uses `errors.UntilErrorConcurrent` which stops on the first error.
**Why it happens:** `UntilErrorConcurrent` is designed for create (stop on first failure, then rollback). Pause/resume need `AggregateConcurrent`-style semantics but sequential.
**How to avoid:** Implement pause/resume as a sequential loop that collects errors in a `[]error` slice, continues the loop on error, then returns `errors.NewAggregate(errs)` at the end. Do not use `UntilErrorConcurrent`.
**Warning signs:** Pause stops at the first failed node, leaving other nodes running.

### Pitfall 6: Status Column Breaks `get clusters` JSON Consumers
**What goes wrong:** Currently `kinder get clusters --output json` returns `["name1","name2"]` (a `[]string`). Adding a Status column means the JSON schema must change to `[{"name":"name1","status":"Running"},...]`. This is a breaking API change.
**Why it happens:** The CONTEXT says add Status column to cluster-list. The existing JSON output is a flat string array.
**How to avoid:** Change the JSON output type to `[]clusterInfo{Name, Status}`. This is intentional per the CONTEXT. Document the schema change in the implementation. The JSON convention (phase-29) used struct-per-item for nodes — apply the same pattern to clusters.
**Warning signs:** Scripts using `jq '.[]' kinder get clusters --output json` break.

---

## Code Examples

Verified patterns from official sources (all from this codebase):

### Docker Container State via Inspect
```go
// Source: docker provider pattern from provider.go:177, verified
lines, err := exec.OutputLines(exec.Command(
    "docker", "inspect",
    "--format", "{{.State.Status}}",
    containerName,
))
// Returns: "running", "exited", "paused", "dead", "created"
// "exited" = stopped (pause uses docker stop, not docker pause)
```

### Doctor Check Skeleton (matches cluster-node-skew pattern)
```go
// Source: verified from pkg/internal/doctor/clusterskew.go
type clusterResumeReadinessCheck struct {
    listNodes func(clusterName string) ([]nodeEntry, error)
}

func (c *clusterResumeReadinessCheck) Name() string       { return "cluster-resume-readiness" }
func (c *clusterResumeReadinessCheck) Category() string    { return "Cluster" }
func (c *clusterResumeReadinessCheck) Platforms() []string { return nil }

func (c *clusterResumeReadinessCheck) Run() []Result {
    // 1. Detect default cluster; if none → skip
    // 2. Count CP nodes; if len(cpNodes) <= 1 → skip (single-CP does not need HA check)
    // 3. Run etcdctl endpoint health --cluster on each CP node
    // 4. If any member unhealthy → warn (not fail, per CONTEXT)
    // 5. Return Result{Status: "warn", Name: c.Name(), Category: c.Category(), ...}
}
```

### Top-Level Command Registration Pattern
```go
// Source: verified from pkg/cmd/kind/root.go
// New entries in NewCommand():
cmd.AddCommand(pause.NewCommand(logger, streams))
cmd.AddCommand(resume.NewCommand(logger, streams))
cmd.AddCommand(status.NewCommand(logger, streams))
```

### Pause Command Cobra Skeleton
```go
// Source: derived from delete/cluster/deletecluster.go pattern
// kinder pause [name] — positional optional arg for cluster name
cmd := &cobra.Command{
    Use:   "pause [cluster-name]",
    Short: "Pauses a running cluster, stopping all containers to reclaim CPU/RAM",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        cli.OverrideDefaultName(cmd.Flags()) // respect KIND_CLUSTER_NAME env
        return runE(logger, streams, flags, args)
    },
}
cmd.Flags().IntVar(&flags.Timeout, "timeout", 30, "graceful stop timeout in seconds before SIGKILL")
cmd.Flags().BoolVar(&flags.JSON, "json", false, "output JSON")
```

### Cluster Status Computation for `get clusters`
```go
// Source: derived from docker ps -a pattern in docker/provider.go:117-134
// and docker inspect pattern above
func clusterStatus(p providers.Provider, binaryName, clusterName string) string {
    nodes, err := p.ListNodes(clusterName)
    if err != nil || len(nodes) == 0 {
        return "Error"
    }
    running, stopped := 0, 0
    for _, n := range nodes {
        lines, err := exec.OutputLines(exec.Command(
            binaryName, "inspect",
            "--format", "{{.State.Status}}",
            n.String(),
        ))
        if err != nil || len(lines) == 0 {
            continue
        }
        if lines[0] == "running" {
            running++
        } else {
            stopped++
        }
    }
    switch {
    case running > 0 && stopped == 0:
        return "Running"
    case stopped > 0 && running == 0:
        return "Paused"
    default:
        return "Error" // mixed or empty
    }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `docker stop` + status file | Container state only (no file) | Phase 47 design decision | Simpler; survives reboots; no sync issues |
| Hardcoded `Status: "Ready"` in `get nodes` | Real container state from `docker inspect` | Phase 47 | Fix existing gap: stopped nodes were incorrectly shown as "Ready" |
| `kinder get clusters` returns `[]string` JSON | `[]clusterInfo{Name,Status}` JSON | Phase 47 | Breaking JSON schema change — intentional |

**Deprecated/outdated in this phase:**
- `status := "Ready"` hardcode in `pkg/cmd/kind/get/nodes/nodes.go:230`: Replace with real container state check.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `etcdctl` is available at `/usr/local/bin/etcdctl` inside kindest/node containers | Code Examples (etcd health check) | `cluster-resume-readiness` check needs path fallback; use `which etcdctl` probe first |
| A2 | etcdctl PKI cert paths are `/etc/kubernetes/pki/etcd/peer.crt` and `/etc/kubernetes/pki/etcd/peer.key` | Code Examples (etcd health check) | Certs may be at different paths on some K8s versions; may need to probe `/etc/kubernetes/pki/etcd/` |
| A3 | `docker label <container> key=value` (add label post-creation) is supported by Docker 29.4 | Architecture Patterns (pre-pause snapshot storage) | If not supported, use a temp file inside the CP container at `/kind/pause-snapshot.json` instead |
| A4 | The external-load-balancer node should be included in pause/stop (included as last on stop, first on start) | Architecture Patterns (System Diagram) | If LB does not gracefully stop and resume cleanly, may need to skip it and only stop internal nodes |

**A3 needs verification:** `docker container update` supports CPU/memory but NOT labels. Docker does NOT support adding labels to running containers post-creation. The correct approach for pre-pause etcd snapshot is a small JSON file inside the CP container (e.g., `/kind/pause-snapshot.json`) written via `node.Command("sh", "-c", "echo '...' > /kind/pause-snapshot.json")`. Container labels cannot be modified after creation.

**Revised recommendation for pre-pause snapshot storage:** Write `/kind/pause-snapshot.json` inside the CP container using `node.Command(...)`. Read it back on resume with `node.Command("cat", "/kind/pause-snapshot.json")`. This file persists across `docker stop`/`docker start` cycles (it is in the container's writable layer, not in a volume). The file disappears on `docker rm` (cluster delete), which is correct behavior.

---

## Open Questions

1. **`--timeout` granularity: global or per-step?**
   - What we know: CONTEXT says "30s default before SIGKILL, overridable via `--timeout` flag"
   - What's unclear: Is `--timeout` the graceful-stop timeout per container, or a total-operation timeout, or the resume-readiness gate timeout?
   - Recommendation: Implement as graceful-stop timeout per container (passed to `docker stop --time=N`). Add a separate `--wait` flag (default `5m`) for the resume readiness gate, matching the `--wait` flag precedent from `kinder create cluster`. This gives users two intuitive knobs.

2. **etcdctl endpoint: all CP nodes or just local?**
   - What we know: `--endpoints=https://127.0.0.1:2379` is the standard for connecting to the local etcd member inside a kind CP container.
   - What's unclear: For HA quorum check, should we exec into each CP node and check `endpoint health` locally, or use `--cluster` flag to query all members from one node?
   - Recommendation: Use `endpoint health --cluster` from the first CP node only. This checks all members in one exec call and avoids needing to iterate all CP nodes for the health check.

3. **Kinder status: include LB node and etcd status?**
   - What we know: CONTEXT says "shows per-node container state, k8s version, paused/running, last pause time (where derivable)"
   - What's unclear: Should `kinder status` also show etcd member health? Or just container state?
   - Recommendation: Show container state (`running`/`exited`) and K8s role. Etcd health is for `kinder doctor` — don't duplicate. Last pause time from `/kind/pause-snapshot.json` if present.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker CLI | `docker stop`, `docker start`, `docker inspect` | ✓ | 29.4.0 | nerdctl/podman (not in scope for phase 47 per CONTEXT deferred ideas) |
| `etcdctl` (inside container) | `cluster-resume-readiness` | [ASSUMED] | Ships with kindest/node | `skip` result if binary not found; graceful degrade |
| `kubectl` (inside container) | Resume readiness gate | ✓ | Available in all kindest/node images (verified: used by waitforready.go) | None needed |

**Missing dependencies with no fallback:** None.

**Note:** Phase 47 scope is Docker only. Podman/nerdctl pause/resume support is explicitly deferred (LIFE-09, LIFE-10 in v2.4+). The implementation should detect the active provider and fail with a clear error if not Docker.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package (no external test framework) |
| Config file | None — `go test ./...` |
| Quick run command | `go test ./pkg/internal/doctor/... ./pkg/cluster/internal/lifecycle/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LIFE-01 | Pause stops workers before CP (order verified) | unit | `go test ./pkg/cluster/internal/lifecycle/... -run TestPauseOrder` | ❌ Wave 0 |
| LIFE-01 | Idempotency: pause on already-paused → no-op | unit | `go test ./pkg/cluster/internal/lifecycle/... -run TestPauseIdempotent` | ❌ Wave 0 |
| LIFE-02 | Resume starts CP before workers (order verified) | unit | `go test ./pkg/cluster/internal/lifecycle/... -run TestResumeOrder` | ❌ Wave 0 |
| LIFE-02 | Resume readiness gate: waits for all nodes Ready | unit | `go test ./pkg/cluster/internal/lifecycle/... -run TestResumeReadinessGate` | ❌ Wave 0 |
| LIFE-03 | Quorum-safe: HA CP has 3 nodes, all stop/start in correct order | unit | `go test ./pkg/cluster/internal/lifecycle/... -run TestHAOrder` | ❌ Wave 0 |
| LIFE-04 | `cluster-resume-readiness` check: skip on single-CP | unit | `go test ./pkg/internal/doctor/... -run TestClusterResumeReadiness_SingleCP` | ❌ Wave 0 |
| LIFE-04 | `cluster-resume-readiness` check: warn on unhealthy member | unit | `go test ./pkg/internal/doctor/... -run TestClusterResumeReadiness_UnhealthyMember` | ❌ Wave 0 |
| LIFE-04 | `cluster-resume-readiness` check registered in allChecks | unit | `go test ./pkg/internal/doctor/... -run TestAllChecks_ContainsResumeReadiness` | ❌ Wave 0 |
| (all) | JSON output has correct schema for pause/resume | unit | `go test ./pkg/cmd/kind/pause/... -run TestJSONOutput` | ❌ Wave 0 |
| (all) | `kinder get clusters` Status column accuracy | unit | `go test ./pkg/cmd/kind/get/clusters/... -run TestClusterStatus` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./pkg/internal/doctor/... ./pkg/cluster/internal/lifecycle/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `pkg/cluster/internal/lifecycle/lifecycle_test.go` — covers LIFE-01, LIFE-02, LIFE-03 (mock provider)
- [ ] `pkg/internal/doctor/resumereadiness_test.go` — covers LIFE-04
- [ ] `pkg/cmd/kind/pause/pause_test.go` (optional; command-level JSON schema test)
- [ ] No new test frameworks needed — existing `go test` infrastructure covers all cases

*(No framework install required — `go test ./...` already works.)*

---

## Security Domain

> security_enforcement not explicitly disabled — including section.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A — local-only CLI, no auth boundary crossed |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A — Docker socket access is already required for all kinder operations |
| V5 Input Validation | yes | Cluster name from args: validated by `provider.ListNodes(name)` — invalid name returns empty list, not exec injection |
| V6 Cryptography | no | etcd certs are read-only, never generated or stored by this phase |

### Known Threat Patterns for Go CLI + Docker

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Shell injection via cluster name | Tampering | Cluster name passed as discrete arg to `exec.Command("docker", "stop", name)` — never interpolated into a shell string; `exec.Command` does not invoke a shell |
| Path traversal in etcd snapshot file write | Tampering | Write path is fixed: `/kind/pause-snapshot.json` — not user-controlled |
| Docker socket privilege escalation | Elevation of Privilege | Existing risk — unchanged by this phase; all kinder operations require Docker socket access |

---

## Sources

### Primary (HIGH confidence)
- `/Users/patrykattc/work/git/kinder/pkg/internal/doctor/check.go` — Check interface, allChecks registry, Result struct, RunAllChecks pattern
- `/Users/patrykattc/work/git/kinder/pkg/internal/doctor/clusterskew.go` — Complete reference implementation of a cluster-aware doctor check with injected test dependencies
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/actions/waitforready/waitforready.go` — waitForReady pattern, tryUntil loop, kubectl-inside-node invocation
- `/Users/patrykattc/work/git/kinder/pkg/cluster/nodeutils/roles.go` — ControlPlaneNodes, SelectNodesByRole, InternalNodes, ExternalLoadBalancerNode
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/providers/docker/provider.go` — docker inspect patterns, ListNodes, clusterLabelKey
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/providers/common/constants.go` — NodeRoleLabelKey constant
- `/Users/patrykattc/work/git/kinder/pkg/cluster/constants/constants.go` — ControlPlaneNodeRoleValue, WorkerNodeRoleValue, ExternalLoadBalancerNodeRoleValue
- `/Users/patrykattc/work/git/kinder/pkg/internal/cli/status.go` — Status spinner: Start/End; successFormat/failureFormat with colored ✓/✗
- `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/get/nodes/nodes.go` — Existing nodeInfo struct, tabwriter output, JSON output, status hardcode to fix
- `/Users/patrykattc/work/git/kinder/pkg/cmd/kind/get/clusters/clusters.go` — Existing clusters JSON output ([]string) that needs Status column
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/providers/nerdctl/provider.go:172-202` — Sequential stop pattern with argsStop construction

### Secondary (MEDIUM confidence)
- `/Users/patrykattc/work/git/kinder/.planning/phases/29-cli-features/29-RESEARCH.md` — Phase-29 JSON convention reference: `json.NewEncoder(streams.Out).Encode(v)`, `--output json` string flag vs `--json` bool flag distinction
- `go test ./...` output — all 30+ test packages pass; baseline established

### Tertiary (LOW confidence)
- [ASSUMED] `etcdctl` available at `/usr/local/bin/etcdctl` in kindest/node images — not directly verified; mitigated by graceful `skip` on binary-not-found

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries are in go.mod; all primitives verified in codebase
- Architecture: HIGH — all patterns verified from existing implementations; new code is primarily wiring
- Doctor check registration: HIGH — allChecks pattern fully understood from 20+ existing checks
- etcd health check: MEDIUM — etcdctl availability in container is ASSUMED (A1, A2)
- Pitfalls: HIGH — verified from code reading; restart policy behavior confirmed from create.go comment

**Research date:** 2026-05-03
**Valid until:** 2026-06-03 (stable Go codebase; no fast-moving dependencies)
