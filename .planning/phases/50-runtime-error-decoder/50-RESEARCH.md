# Phase 50: Runtime Error Decoder - Research

**Researched:** 2026-05-06
**Domain:** Go CLI extension — pattern-matching log decoder integrated into existing `kinder doctor` framework
**Confidence:** HIGH

---

## Summary

Phase 50 extends the v2.1 doctor framework with a `kinder doctor decode` subcommand. The command is a one-shot, live-cluster scanner: it collects recent docker logs from node containers and `kubectl get events` output, runs them through a catalog of known error patterns, and prints plain-English explanations with suggested fixes. An optional `--auto-fix` flag applies a whitelist of non-destructive remediations automatically.

The codebase investigation reveals a fully formed extension path. The `pkg/internal/doctor` package owns the `Check` interface, `Result` struct, `FormatHumanReadable`, and `FormatJSON` renderers. The `pkg/cmd/kind/doctor/doctor.go` command is the current top-level command (no subcommands yet). Phase 50 must convert `doctor` into a parent command with two subcommands: the existing `run` checks (possibly renamed or kept as the default action) and the new `decode` subcommand. The actual pattern-matching engine, pattern catalog, and log collectors must live in `pkg/internal/doctor/` to follow the established pattern, while the CLI wiring lives in `pkg/cmd/kind/doctor/`.

The pattern catalog can be sourced entirely from: (a) `pkg/internal/doctor` existing check bodies (they describe pre-flight conditions that become post-hoc patterns), (b) `pkg/cluster/internal/providers/common/cgroups.go` (cgroup regex), (c) kind Known Issues page, (d) kubeadm troubleshooting guide, and (e) CI failure knowledge baked into this repo's doctor checks. No external data source or new module dep is needed.

**Primary recommendation:** Implement decode as a `doctor decode` subcommand. The doctor parent command becomes `cobra.Command{Use: "doctor", RunE: nil}` and adds two children: a `run` subcommand (the current `doctor.go` behavior, kept identical) and a new `decode` subcommand. This is the smallest structural change and matches how `kinder snapshot` was handled in Phase 48.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Pattern catalog + matcher engine | `pkg/internal/doctor/` | — | All doctor check logic lives here; decoder is another check-type, not a CLI concept |
| Log collection (docker logs) | `pkg/internal/doctor/` (via injected fn vars) | `pkg/internal/lifecycle/` helpers | Same pattern as `realListNodes` / `realExecInContainer` — direct `exec.Command(binaryName, "logs", ...)` is the convention |
| Event collection (kubectl get events) | `pkg/internal/doctor/` (via injected fn vars) | Host kubectl (external kubeconfig) | Phase 49-02 established: user-facing commands use host kubectl with external kubeconfig |
| Auto-fix remediation execution | `pkg/internal/doctor/` (SafeMitigation pattern) | — | `mitigations.go` already defines `SafeMitigation` struct with `NeedsFix/Apply/NeedsRoot` |
| Cluster-name resolution | `pkg/internal/lifecycle/ResolveClusterName` | — | Exact same pattern used by pause, resume, status, dev |
| Node enumeration (for log collection) | `pkg/cluster.Provider.ListNodes` | — | Same as status.go and clusters.go |
| CLI wire-up (cobra subcommand) | `pkg/cmd/kind/doctor/` | — | Matches Phase 48's snapshot command pattern |
| Output rendering | `pkg/internal/doctor/FormatHumanReadable` + new decode variant | — | SC3 mandates same render style; decoder adds its own per-match format |

---

## Codebase Touchpoints

### Files to Read Before Planning

| File | Role in Phase 50 |
|------|-----------------|
| `pkg/cmd/kind/doctor/doctor.go` | The current `doctor` command — becomes a parent; `NewCommand` must change to add `AddCommand(decode.NewCommand(...))` and `AddCommand(run.NewCommand(...))` or keep RunE as default |
| `pkg/internal/doctor/check.go` | `Check` interface and `allChecks` registry — `decode` does NOT add to `allChecks`; it has its own parallel catalog |
| `pkg/internal/doctor/format.go` | `FormatHumanReadable` / `FormatJSON` — decoder output must match the same rendering style; reuse or produce a `DecodeResult` that renders consistently |
| `pkg/internal/doctor/mitigations.go` | `SafeMitigation` struct — decoder's auto-fix whitelist SHOULD reuse this struct verbatim for `NeedsFix/Apply/NeedsRoot` lifecycle |
| `pkg/internal/doctor/clusterskew.go` | `realListNodes` / `realExecInContainer` pattern — copy this pattern for docker log collection |
| `pkg/internal/doctor/resumereadiness.go` | `realExecInContainer` / `realInspectState` — same injectable-fn injection pattern, same cycle-avoidance for doctor→lifecycle import |
| `pkg/internal/lifecycle/state.go` | `ResolveClusterName`, `ProviderBinaryName`, `ClassifyNodes` — all needed by decode command's cluster resolution |
| `pkg/cmd/kind/pause/pause.go` | Gold standard for cluster-name resolution pattern in a CLI command |
| `pkg/cmd/kind/status/status.go` | Gold standard for multi-node iteration (lines 120-157) |
| `pkg/internal/doctor/testhelpers_test.go` | `fakeCmd` / `newFakeExecCmd` pattern — decoder tests MUST reuse these test helpers (same package) |

### New Files to Create

| Package | File | Contents |
|---------|------|----------|
| `pkg/internal/doctor/` | `decode.go` | `DecodePattern`, `DecodeResult`, `MatchLog`, catalog declaration, `RunDecode` orchestrator |
| `pkg/internal/doctor/` | `decode_catalog.go` | The 15+ error pattern entries as Go struct literals |
| `pkg/internal/doctor/` | `decode_autofix.go` | Auto-fix whitelist entries (`SafeMitigation`-shaped structs) |
| `pkg/internal/doctor/` | `decode_test.go` | Unit tests for matcher engine and catalog |
| `pkg/cmd/kind/doctor/` | `decode/decode.go` | Cobra `decode` subcommand: flags, cluster resolution, calls `doctor.RunDecode` |
| `pkg/cmd/kind/doctor/` | `decode/decode_test.go` | Unit tests for CLI flag parsing and output format |

**Important:** `doctor.go` (the top-level command) must be modified to become a parent command. The current `RunE` must move to a new `run` subcommand OR kept as default action (cobra supports both — see Phase 48's `snapshot.go` pattern which has no default RunE and requires an explicit subcommand name).

### Import Boundary Check

The existing import cycle rule: `pkg/internal/doctor` MUST NOT import `pkg/internal/lifecycle` (because `lifecycle/resume.go` already imports `doctor`). The decode command's node-enumeration will use the same workaround as `clusterskew.go`: inline `osexec.LookPath` + direct `exec.Command` calls without going through `lifecycle`. The CLI layer (`pkg/cmd/kind/doctor/decode/`) CAN import both `lifecycle` and `doctor`.

[VERIFIED: codebase inspection — `clusterskew.go` line 71 and `resumereadiness.go` line 316 both inline binary detection rather than calling `lifecycle.ProviderBinaryName`]

---

## Pattern Catalog Seed

The following 22 candidate patterns were compiled from: kind Known Issues page [CITED: kind.sigs.k8s.io/docs/user/known-issues/], kubeadm troubleshooting guide [CITED: kubernetes.io/docs/setup/production-environment/tools/kubeadm/troubleshooting-kubeadm/], existing `pkg/internal/doctor/` check bodies [VERIFIED: codebase inspection], and common kind issue tracker patterns [CITED: github.com/kubernetes-sigs/kind/issues].

Planner must pick ≥15 of these for the initial catalog. Candidates are ranked by expected frequency in kinder CI and local development contexts.

### Kubelet Patterns

| ID | Scope | Substring / Regex Pattern | Plain-English | Suggested Fix | Link | Confidence |
|----|-------|--------------------------|---------------|---------------|------|------------|
| KUB-01 | kubelet | `"too many open files"` | Inotify watch limit exhausted — kubelet cannot watch required files | `sudo sysctl fs.inotify.max_user_watches=524288 && sudo sysctl fs.inotify.max_user_instances=512` | https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files | HIGH |
| KUB-02 | kubelet | `"failed to create fsnotify watcher"` | Same root cause as KUB-01 — inotify resource exhaustion | Same fix as KUB-01 | https://kind.sigs.k8s.io/docs/user/known-issues/ | HIGH |
| KUB-03 | kubelet | `"kubelet is not running"` OR `"kubelet isn't running"` | Kubelet service has not started — usually a cgroup or CRI misconfiguration | Check `docker logs <node>` for prior errors; verify cgroup driver | https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/troubleshooting-kubeadm/ | HIGH |
| KUB-04 | kubelet | `"Get \"http://127.0.0.1:10248/healthz\": context deadline exceeded"` | Kubelet health check timed out — kubelet is not responding; often caused by missing cgroup support | Increase Docker memory limit; check `docker logs <node>` for cgroup errors | https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/troubleshooting-kubeadm/ | HIGH |
| KUB-05 | kubelet | `"unable to start container process: error adding pid"` OR `"error adding pid .* to cgroups"` | Cgroup v2 hierarchy conflict — node entrypoint hasn't finished cgroup setup when exec ran | Wait for node readiness regex; run `kinder doctor` before re-creating cluster | https://github.com/kubernetes-sigs/kind/issues/2409 | HIGH |
| KUB-06 | kubelet | `"certificate has expired or is not yet valid"` | TLS certificate expiry or clock skew — kubelet cannot connect to API server | Check system clock (`date`); if cluster is old, recreate it | https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/troubleshooting-kubeadm/ | MEDIUM |
| KUB-07 | kubelet | `"failed to run Kubelet: validate service connection: validate CRI v1 runtime API"` | Kubelet cannot connect to containerd's CRI — containerd CRI plugin disabled or socket mismatch | Check containerd config: `docker exec <node> cat /etc/containerd/config.toml` | https://github.com/containerd/containerd/issues/8139 | MEDIUM |

### Kubeadm Patterns

| ID | Scope | Substring / Regex Pattern | Plain-English | Suggested Fix | Link | Confidence |
|----|-------|--------------------------|---------------|---------------|------|------------|
| KADM-01 | kubeadm | `"[ERROR CRI]: container runtime is not running"` | kubeadm pre-flight: containerd is present but its CRI service is disabled | `docker exec <node> systemctl restart containerd` or recreate cluster | https://github.com/containerd/containerd/issues/8139 | HIGH |
| KADM-02 | kubeadm | `"coredns"` + `"Pending"` in events | CoreDNS stuck in Pending — CNI plugin not installed or misconfigured | Check CNI pods: `kubectl get pods -n kube-system`; verify kindnet/flannel running | https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/troubleshooting-kubeadm/ | HIGH |
| KADM-03 | kubeadm | `"context deadline exceeded"` (in kubeadm log context) | kubeadm timed out waiting for a component to become ready — usually kubelet health or etcd | Increase Docker resources; check `docker logs <node>` for prior errors | https://github.com/kubernetes/kubeadm/issues/3069 | HIGH |
| KADM-04 | kubeadm | `"node not found"` | Node has not registered with the API server yet — join step completed but kubelet not ready | `kubectl get nodes --watch`; wait or recreate cluster if node never appears | kubernetes.io troubleshooting | MEDIUM |
| KADM-05 | kubeadm | `"x509: certificate signed by unknown authority"` | Kubeconfig points to a re-created cluster but references old certs | Delete old kubeconfig: `kind export kubeconfig` to refresh | kubernetes.io troubleshooting | MEDIUM |

### Containerd Patterns

| ID | Scope | Substring / Regex Pattern | Plain-English | Suggested Fix | Link | Confidence |
|----|-------|--------------------------|---------------|---------------|------|------------|
| CTD-01 | containerd | `"failed to pull image"` + `"not found"` | Image tag or digest not found in registry — pull policy, wrong tag, or private registry | Verify image tag; if loading local image: `kinder load images <image>` | komodor.com/learn/how-to-fix-errimagepull-and-imagepullbackoff/ | HIGH |
| CTD-02 | containerd | `"failed to pull image"` + `"connection refused"` OR `"i/o timeout"` | Registry unreachable — network issue or air-gapped environment | Check network connectivity; pre-load image with `kinder load images` | kind known issues | HIGH |
| CTD-03 | containerd | `"ImagePullBackOff"` in events | Containerd cannot pull the image after repeated retries — see CTD-01/CTD-02 for root cause | Describe pod: `kubectl describe pod <name>`; check events for specific error | komodor.com | HIGH |
| CTD-04 | containerd | `"OCI runtime create failed"` + `"cgroup"` | Container runtime cannot set cgroup config — kernel or cgroup v2 issue | Check kernel version; try `--feature-gates=KubeletInUserNamespace=true` | kind known issues | MEDIUM |

### Docker Patterns

| ID | Scope | Substring / Regex Pattern | Plain-English | Suggested Fix | Link | Confidence |
|----|-------|--------------------------|---------------|---------------|------|------------|
| DOCK-01 | docker | `"no space left on device"` | Docker has run out of disk space | `docker system prune` to reclaim space; check `df -h` | kind known issues | HIGH |
| DOCK-02 | docker | `"permission denied"` + `"docker.sock"` OR `"/var/run/docker.sock"` | Docker socket not accessible — user not in docker group or socket permissions wrong | `sudo usermod -aG docker $USER && newgrp docker` | kind known issues | HIGH |
| DOCK-03 | docker | `"TMPDIR"` + `"snap"` OR `"cannot create temp file"` | Docker installed via Snap lacks access to system TMPDIR | `export TMPDIR=$HOME/tmp && mkdir -p $HOME/tmp` | https://kind.sigs.k8s.io/docs/user/known-issues/#docker-installed-with-snap | HIGH |

### Addon-Startup Patterns

| ID | Scope | Substring / Regex Pattern | Plain-English | Suggested Fix | Link | Confidence |
|----|-------|--------------------------|---------------|---------------|------|------------|
| ADDON-01 | addon | `"CrashLoopBackOff"` in events + pod namespace `kube-system` | A system addon pod is crash-looping — check `kubectl logs -n kube-system <pod>` for details | `kubectl logs -n kube-system <pod> --previous`; look for config or image errors | kubernetes.io troubleshooting | HIGH |
| ADDON-02 | addon | `"MountVolume.SetUp failed"` + `"configmap"` + `"not found"` | A ConfigMap required by an addon pod does not exist — cluster creation was incomplete | Check for missing resources: `kubectl get configmap -n kube-system`; recreate cluster | kubernetes.io troubleshooting | MEDIUM |
| ADDON-03 | addon | events `"BackOff"` + `"pulling image"` | Image pull back-off for addon images — registry unreachable or image missing | Pre-load addon images with `kinder snapshot create` / `kinder load images` | kind offline docs | MEDIUM |

**Planner note:** 22 candidates above — 15 minimum required. Recommend including all HIGH confidence patterns (16 items) as v1. MEDIUM confidence patterns are correct but less common in local dev workflows.

---

## Pattern Schema Recommendation

**Decision: Go struct literals in `decode_catalog.go`, NOT YAML/JSON.**

Rationale: The existing doctor package uses zero external file dependencies. All check logic and data are in Go struct literals. Adding YAML/JSON at runtime would require file embedding (`embed.FS`) or a YAML dep — both violate the "zero new module deps preferred" rule. The pattern data is small (≤25 entries) and benefits from compile-time type safety.

### Recommended Types (in `pkg/internal/doctor/decode.go`)

```go
// Scope groups patterns into broad categories for display and filtering.
type DecodeScope string

const (
    ScopeKubelet    DecodeScope = "kubelet"
    ScopeKubeadm   DecodeScope = "kubeadm"
    ScopeContainerd DecodeScope = "containerd"
    ScopeDocker     DecodeScope = "docker"
    ScopeAddon      DecodeScope = "addon"
)

// DecodePattern is a single entry in the error catalog.
type DecodePattern struct {
    // ID is the unique pattern identifier, e.g. "KUB-01".
    ID string
    // Scope is the component this pattern belongs to.
    Scope DecodeScope
    // Match is a substring or regex string. Use strings.Contains for fixed
    // strings; compile to regexp when the pattern starts with "regex:".
    // Keeping simple patterns as plain substrings avoids regexp overhead for
    // the common case.
    Match string
    // Explanation is the plain-English description of what went wrong.
    Explanation string
    // Fix is the suggested remediation (one-liner or short paragraph).
    Fix string
    // DocLink is an optional URL to docs or the kind issue tracker.
    // Empty string means no link.
    DocLink string
    // AutoFixable is true when a non-destructive SafeMitigation exists for this pattern.
    AutoFixable bool
    // AutoFix is the safe remediation to apply when --auto-fix is set.
    // nil when AutoFixable is false.
    AutoFix *SafeMitigation
}

// DecodeMatch is a single pattern match found in the collected logs/events.
type DecodeMatch struct {
    // Source identifies where the match was found: "docker-logs:<nodeName>" or "k8s-events".
    Source      string
    // Line is the raw log or event line that triggered the match.
    Line        string
    // Pattern is the matched catalog entry.
    Pattern     DecodePattern
}

// DecodeResult is the top-level output of a RunDecode call.
type DecodeResult struct {
    Cluster   string        // the resolved cluster name
    Matches   []DecodeMatch // all matches found, in source order
    Unmatched int           // count of log lines that matched no pattern (for debug)
}
```

### Rendering (SC3 compliance)

The human-readable output for each `DecodeMatch` should follow the same Unicode-icon style as `format.go`:

```
=== Decode Results: <cluster> ===

  [KUB-01] kubelet — Inotify watch limit exhausted
    Source: docker-logs:kind-control-plane
    Line:   failed to create fsnotify watcher: too many open files
    Fix:    sudo sysctl fs.inotify.max_user_watches=524288 ...
    Docs:   https://kind.sigs.k8s.io/docs/user/known-issues/...

───
3 lines scanned, 1 pattern matched.
```

JSON mode wraps `DecodeResult` as-is via `json.NewEncoder`.

---

## Log / Event Collection Plan

### Docker Log Collection

**How:** `exec.Command(binaryName, "logs", "--since", window, nodeName)` where `window` is the time-window flag (default `"30m"`).

**Pattern origin:** `realListNodes` in `clusterskew.go` and `realExecInContainer` in `resumereadiness.go` — both use `exec.OutputLines(exec.Command(binaryName, ...))` directly. Decoder follows the same pattern.

**Multi-node iteration:** Use `lifecycle.ResolveClusterName` to get the cluster name, then `provider.ListNodes(name)` to enumerate all nodes (same as `status.go` lines 99-107). For each node, call `docker logs --since <window> <nodeName>`.

**Node name:** `n.String()` — the container name, which is the same string passed to `docker logs`.

**Time-window default:** `30m` (30 minutes). Justification: kind cluster creation takes ≤5 minutes; addon startup ≤10 minutes; total headroom. `--since` accepts Docker time formats (`30m`, `1h`, `2006-01-02T15:04:05`).

**Error handling:**
- `docker logs` fails on a paused/stopped node → skip that node, log at V(1), continue
- No cluster found → return error "no kind cluster found; specify a cluster name with --name"
- No docker binary → error "no container runtime found; run kinder doctor"

**Injection point:** A `dockerLogsFn func(binaryName, nodeName, since string) ([]string, error)` package-level var (nil → real impl) for unit testing without a live cluster.

### kubectl Events Collection

**How:** Host kubectl with the cluster's kubeconfig, same convention as Phase 49-02's `RolloutRestartAndWait`.

```
kubectl get events --all-namespaces --sort-by=.lastTimestamp --since=<window>
```

**Kubeconfig path:** kind writes the cluster kubeconfig to `~/.kube/config` by default. Decode should use `kind export kubeconfig` approach — or simply detect the kubeconfig using the same mechanism as `WaitForNodesReady` (in-node kubectl with `--kubeconfig=/etc/kubernetes/admin.conf`).

**Convention question (answered by investigation):** Two patterns exist:
- `localpath.go` line 84: `binaryName exec cpName kubectl --kubeconfig=/etc/kubernetes/admin.conf ...` — in-node kubectl (for cluster-internal queries that need the admin cert)
- Phase 49-02: host kubectl with external kubeconfig written to a tempfile — for user-facing operations

For `kubectl get events`, either works. The in-node approach is simpler (no tempfile, no kubeconfig resolution). Recommend the **in-node approach** (consistent with `localpath.go` and `resumereadiness.go`) because:
1. No dependency on the host kubectl's context (the user may have a different cluster selected in `~/.kube/config`)
2. Avoids tempfile lifecycle management
3. The doctor framework's existing live-cluster queries all use in-node exec

**Injection point:** An `eventsFn func(binaryName, cpNodeName, since string) ([]string, error)` package-level var for testing.

**Failure modes:**
- `kubectl get events` exits non-zero (cluster not yet ready) → return empty list, not error
- No events in time window → return empty list, no match
- Output too large (> 1000 lines) → warn and truncate to last 1000 lines

---

## Auto-Fix Whitelist Proposal

DIAG-04 requires `--auto-fix` to apply only whitelisted, non-destructive remediations. The existing `SafeMitigation` struct in `mitigations.go` is the correct abstraction — `NeedsFix/Apply/NeedsRoot` already encode the precondition + idempotency contract.

### Candidate Remediations

| Pattern | Remediation | Precondition | Idempotent? | Destructive? | NeedsRoot |
|---------|-------------|--------------|-------------|--------------|-----------|
| KUB-01/KUB-02 (inotify) | `sysctl -w fs.inotify.max_user_watches=524288` AND `sysctl -w fs.inotify.max_user_instances=512` | `current < min` (read from `/proc/sys/fs/inotify/`) | Yes — sysctl write is idempotent | No — raises limit, never lowers | Yes |
| KADM-02 (CoreDNS Pending) | `kubectl rollout restart deployment/coredns -n kube-system` | CoreDNS deployment exists AND pod status is not Running | Only triggers if not already Running | No — restart, not delete | No |
| CTD-01/CTD-02 (image pull) | NO AUTO-FIX | Pull failure root cause varies; cannot be non-destructively fixed automatically | — | — | — |
| DOCK-01 (no space) | NO AUTO-FIX | `docker system prune` is destructive (removes images) | — | Yes — removes data | — |
| KUB-05 (cgroup race) | `docker restart <nodeName>` | Node container is in a broken state (`docker inspect` state ≠ "running") | Restart is idempotent if node was stuck | No — but could interrupt a running node | Yes-ish |

**Recommended v1 auto-fix list (3 safe entries):**
1. **inotify-raise** — Apply when KUB-01 or KUB-02 matched; raise `max_user_watches` and `max_user_instances` to minimum safe values. Precondition: current values below threshold. NeedsRoot=true.
2. **coredns-restart** — Apply when KADM-02 matched and CoreDNS pods are in non-Running state. Execute `docker exec <cpNode> kubectl --kubeconfig=/etc/kubernetes/admin.conf rollout restart deployment/coredns -n kube-system`. NeedsRoot=false.
3. **node-container-restart** — Apply when KUB-05 matched and the specific node container is in an error state (`State.Status` ≠ "running"). Execute `docker start <nodeName>`. NeedsRoot=false.

KUB-06 (cert expiry) and DOCK-01 (no space) are explicitly excluded — cert renewal or disk reclaim require human judgment. CTD patterns cannot be auto-fixed without knowing what image to pre-load.

**Gate:** Before any auto-fix: print "Auto-fix: applying <N> remediation(s) to cluster <name>..." and list the planned actions. For NeedsRoot items: check `os.Geteuid() == 0`; if not root, warn and skip (same as `ApplySafeMitigations` in `mitigations.go`).

---

## Doctor Framework Integration

### Current command tree

```
kinder doctor          ← currently a leaf command (no subcommands)
```

### Phase 50 target tree

```
kinder doctor          ← becomes a parent (RunE = nil or shows help)
  kinder doctor run    ← the existing checks (moved from doctor.go's RunE)
  kinder doctor decode ← new subcommand
    --name <cluster>
    --since <duration>  (default 30m)
    --output <format>   ("" or "json")
    --auto-fix
```

**Design choice — keep doctor as default or require explicit subcommand?**

Phase 48's `kinder snapshot` has no default RunE and requires an explicit subcommand name (`kinder snapshot create`, `kinder snapshot list`, etc.). That is cleaner than hiding `doctor run` behind the bare `kinder doctor` call. **Recommendation:** Convert `doctor` to a parent group command. Move existing logic to `kinder doctor run`. Existing users who call `kinder doctor` directly will see a help message listing the subcommands — a one-level breaking change acceptable for a v2.3 milestone (similar to how Phase 47 knowingly broke `kinder get clusters --output json` schema).

If backward-compat is a hard requirement, keep `doctor.RunE` pointing to the check-runner and add `decode` as a peer subcommand; Cobra supports parent commands with RunE AND subcommands simultaneously. **The planner decides** — both are implementable. Research recommends the clean break (parent + two subcommands) as it matches the snapshot pattern established in Phase 48.

### Package structure for decode subcommand

Following Phase 48's pattern (`pkg/cmd/kind/snapshot/` → separate subcommand files):

```
pkg/cmd/kind/doctor/
├── doctor.go           ← modified: parent command, adds decode + run as subcommands
├── doctor_test.go      ← unchanged (tests extractMountPaths)
├── decode/
│   ├── decode.go       ← cobra subcommand wiring
│   └── decode_test.go  ← unit tests for flag parsing and output
```

If `run` is kept as explicit subcommand:
```
pkg/cmd/kind/doctor/
├── doctor.go           ← parent only
├── run/
│   └── run.go          ← current doctor.go RunE moved here
└── decode/
    └── decode.go
```

---

## Standard Stack

No new module dependencies. Zero new deps is the project invariant; any dep requires STATE.md authorization.

| Component | Where | Notes |
|-----------|-------|-------|
| Pattern matching | `strings.Contains` for plain substrings; `regexp.MustCompile` for patterns with `"regex:"` prefix | Both are stdlib. `cgroups.go` already uses `regexp.MustCompile` in the same codebase |
| Docker log collection | `exec.Command(binaryName, "logs", "--since", ...)` | Same `exec` package used everywhere |
| kubectl events | In-node `docker exec <cp> kubectl --kubeconfig=/etc/kubernetes/admin.conf get events ...` | Same pattern as `localpath.go` |
| Output rendering | Existing `FormatHumanReadable` style + new decode-specific table format | No new dep |
| Auto-fix execution | `SafeMitigation` struct from `mitigations.go` | Reused verbatim |

[VERIFIED: go.mod — all needed stdlib packages already present; no new imports required]

---

## Architecture Patterns

### Pattern 1: Injected Function Variables (the "fn-var" pattern)

All live-cluster interactions in `pkg/internal/doctor` use package-level function variables defaulted to the real implementation, swapped in tests via `t.Cleanup`. The decoder MUST follow this pattern because it needs both docker logs and kubectl events as injectable sources.

```go
// Source: pkg/internal/doctor/resumereadiness.go pattern

// dockerLogsFn is the test-injection point for collecting docker logs.
// Production calls realDockerLogs; tests inject a func returning fixture lines.
var dockerLogsFn = realDockerLogs

func realDockerLogs(binaryName, nodeName, since string) ([]string, error) {
    return exec.OutputLines(exec.Command(binaryName, "logs", "--since", since, nodeName))
}

// k8sEventsFn is the test-injection point for collecting kubectl events.
var k8sEventsFn = realK8sEvents

func realK8sEvents(binaryName, cpNodeName, since string) ([]string, error) {
    return exec.OutputLines(exec.Command(
        binaryName, "exec", cpNodeName,
        "kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
        "get", "events", "--all-namespaces",
        "--sort-by=.lastTimestamp",
        "--field-selector", "type!=Normal", // only Warning and related
    ))
}
```

[VERIFIED: `pkg/internal/doctor/resumereadiness.go` lines 354-390 — same `exec.OutputLines(exec.Command(...))` pattern for in-container execution]

### Pattern 2: Match against collected lines

```go
// Source: pkg/internal/doctor/decode.go (to be created)

func matchLines(lines []string, patterns []DecodePattern, source string) []DecodeMatch {
    var matches []DecodeMatch
    for _, line := range lines {
        for _, pat := range patterns {
            if matchesPattern(line, pat.Match) {
                matches = append(matches, DecodeMatch{
                    Source:  source,
                    Line:    line,
                    Pattern: pat,
                })
                break // first match wins per line; avoids flooding on same error
            }
        }
    }
    return matches
}
```

### Anti-Patterns to Avoid

- **Importing `lifecycle` from `doctor`**: `lifecycle/resume.go` imports `doctor`. This creates a cycle if `doctor` imports `lifecycle`. The decode engine in `pkg/internal/doctor/` must detect the binary name using the same inline `osexec.LookPath` pattern as `clusterskew.go`.
- **Adding YAML/JSON external pattern file**: Requires `embed.FS` and a parse dep or file read at runtime — fragile and unnecessary for a catalog of ≤25 patterns. Go struct literals are compile-time verified.
- **Using `--output` flag from `docker logs`**: Docker's `--format` filter is template-based and not safe to parse as structured data. Raw `--since` + `OutputLines` is safer.
- **Running `docker logs` synchronously on all nodes in sequence**: For multi-node clusters, iterate serially. Parallel collection adds complexity without significant speed benefit for a diagnostic command.

---

## Common Pitfalls

### Pitfall 1: Import cycle (doctor ↔ lifecycle)
**What goes wrong:** Decoder in `pkg/internal/doctor/decode.go` imports `pkg/internal/lifecycle` to use `ProviderBinaryName` or `ResolveClusterName` — this creates a cycle because `lifecycle/resume.go` already imports `doctor`.
**Why it happens:** The convenience of lifecycle helpers is tempting; the cycle is not obvious at first.
**How to avoid:** Copy the `osexec.LookPath` binary detection inline (3 lines, as in `clusterskew.go` line 70). Node enumeration (`ListNodes`) must be done in the CLI layer (`decode/decode.go`), not in the engine.
**Warning signs:** `go build` error "import cycle not allowed"

### Pitfall 2: `docker logs` on paused/stopped nodes
**What goes wrong:** `docker logs <stopped-node>` exits non-zero and returns no output, causing the decoder to silently miss logs.
**Why it happens:** Calling `docker logs` on a container that was paused or stopped.
**How to avoid:** Call `docker inspect --format {{.State.Status}} <node>` first (or use `lifecycle.ContainerState` via the CLI layer); skip nodes not in "running" state. OR treat a non-zero exit from `docker logs` as "no logs" rather than a fatal error.
**Warning signs:** Decoder reports "0 patterns matched" even when errors are known to exist.

### Pitfall 3: Multiple patterns matching the same log line causing duplicate output
**What goes wrong:** A log line containing both "too many open files" and "fsnotify" matches KUB-01 AND KUB-02, printing the same issue twice.
**How to avoid:** `matchLines` uses `break` after first match per line (first-match-wins). Or deduplicate by pattern ID before rendering.
**Warning signs:** User sees the same fix suggestion repeated 2-3 times.

### Pitfall 4: `--since` not supported by `kubectl get events`
**What goes wrong:** `kubectl get events --since` is NOT a valid kubectl flag (kubectl uses `--field-selector` for time filtering, which requires a different approach).
**How to avoid:** Use `kubectl get events --sort-by=.lastTimestamp` without `--since`. Apply time-window filtering client-side: parse the `LAST SEEN` column and filter lines older than the window. Or accept that events returns all events (typically ≤100) and rely on sort-by-time to put recent events first.
**Warning signs:** `kubectl get events --since` error at runtime.

### Pitfall 5: Cobra parent command with RunE AND subcommands confusion
**What goes wrong:** If `doctor.NewCommand` keeps `RunE` pointing to the existing checks AND adds `decode` as a subcommand, `kinder doctor decode` works but `kinder doctor` still runs the full check suite. This is confusing but functional. The risk is that `--output` flag on the parent is shared with the `run` subcommand but not with `decode`, causing flag parse errors.
**How to avoid:** Either (a) remove `RunE` from the parent and create explicit `run` and `decode` subcommands, OR (b) keep `RunE` on the parent and flag ALL `doctor` flags as applicable to the `run` behavior (document this choice clearly). The Phase 48 snapshot pattern (no parent RunE) is the cleaner approach.

---

## Plan-Shaping Recommendation

Based on dependency analysis, the following wave/plan breakdown minimizes risk:

### Plan 50-01: Pattern catalog + matcher engine (Wave 1)
**Files:** `pkg/internal/doctor/decode.go`, `pkg/internal/doctor/decode_catalog.go`, `pkg/internal/doctor/decode_test.go`
**Contents:** `DecodePattern`, `DecodeMatch`, `DecodeResult` types; `decode_catalog.go` with 15+ Go struct literal entries; `matchLines()` pure function (no I/O); unit tests with golden fixture strings.
**TDD:** RED — test that KUB-01 pattern matches "too many open files" line; test that non-matching line produces no match. GREEN — implement `matchLines` and catalog.
**Zero deps added.** All tests pass in `pkg/internal/doctor/` package.

### Plan 50-02: Log and event collectors (Wave 1, parallel-safe with 50-01 if separate files)
**Files:** `pkg/internal/doctor/decode_collectors.go`, `pkg/internal/doctor/decode_collectors_test.go`
**Contents:** `dockerLogsFn`, `k8sEventsFn` package vars with real implementations and injectable test hooks; `RunDecode(...)` orchestrator function that takes a cluster name, binary name, CP node name, since-window string, and returns `DecodeResult`.
**TDD:** RED — test that `RunDecode` with injected mock log lines returns expected `DecodeMatch` entries. GREEN — implement `RunDecode` calling `dockerLogsFn` per node + `k8sEventsFn` once.
**Note:** Cannot test against a live cluster in unit tests. Integration test (build-tagged) for 50-05.

### Plan 50-03: Doctor subcommand wiring (Wave 2 — depends on 50-01, 50-02)
**Files:** `pkg/cmd/kind/doctor/doctor.go` (modified), `pkg/cmd/kind/doctor/decode/decode.go`, `pkg/cmd/kind/doctor/decode/decode_test.go`
**Contents:** Modify `doctor.go` to be a parent command; add `decode.NewCommand(...)` as subcommand; decode CLI flags: `--name`, `--since`, `--output`, `--auto-fix`; cluster resolution using `lifecycle.ResolveClusterName`; node enumeration using `provider.ListNodes`; call `doctor.RunDecode(...)`; format output using new decode renderer.
**If `run` is broken out:** Add `pkg/cmd/kind/doctor/run/run.go` (current `doctor.go` logic, unchanged).
**TDD:** RED — test decode subcommand registers under `kinder doctor`; test `--name` / `--since` / `--output` flag parsing. GREEN — implement.

### Plan 50-04: Auto-fix whitelist (Wave 2 — depends on 50-01, 50-02)
**Files:** `pkg/internal/doctor/decode_autofix.go`, updated `decode_catalog.go` (AutoFixable=true on qualifying patterns), `decode_autofix_test.go`
**Contents:** 3 `SafeMitigation`-shaped entries for inotify-raise, coredns-restart, node-container-restart; `ApplyDecodeAutoFix(matches []DecodeMatch, logger)` function that iterates matches, applies remediations for AutoFixable patterns, respects NeedsRoot; update `DecodePattern.AutoFix` field in catalog.
**TDD:** RED — test that auto-fix is applied when inotify pattern matches AND current value is below threshold; test no auto-fix when NeedsRoot=true and Geteuid()!=0. GREEN — implement.

### Plan 50-05: Integration test (Wave 3 — depends on 50-03)
**Files:** `pkg/internal/integration/decode_integration_test.go` (or co-located build-tagged test)
**Contents:** Build-tagged `//go:build integration` test that creates a real cluster, induces a known-patterned condition (or reads from golden fixture logs), runs `RunDecode`, asserts at least 1 match. Models Phase 48-06's integration test pattern.
**Note:** Human-verify step: run `kinder doctor decode` against the openshell-dev cluster and confirm output is readable.

### Dependency graph

```
50-01 (catalog + matcher) ──► 50-02 (collectors + RunDecode) ──► 50-03 (CLI wiring)
                                                                 └──► 50-04 (auto-fix)
                                                                       └──► 50-05 (integration)
```

50-01 and 50-02 can be written as one plan if desired (smaller surface, fewer atomic commits). The split is recommended to keep each plan's test count manageable (target ≤20 new tests per plan per project velocity data).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Pattern matching | Custom regex engine | `strings.Contains` + stdlib `regexp` | Already used in `cgroups.go`; zero new dep |
| Docker log streaming | Custom websocket / Docker API client | `exec.Command(binary, "logs", "--since", ...)` | Existing pattern used by `realExecInContainer`; no dep |
| Kubeconfig resolution | Custom kubeconfig parser | In-node `kubectl --kubeconfig=/etc/kubernetes/admin.conf` | Consistent with `localpath.go` and all existing live-cluster queries |
| Cluster name resolution | Custom cluster discovery | `lifecycle.ResolveClusterName(args, provider)` | Shared by pause, resume, status, dev — single source of truth |
| Safe mitigation framework | New auto-fix abstraction | `SafeMitigation` struct in `mitigations.go` | Already exists and has `NeedsFix/Apply/NeedsRoot` contract |
| Output formatting | Custom table renderer | Extend existing `FormatHumanReadable` style + `tabwriter` | Consistency with SC3; existing `format.go` is the model |

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go test (`go test`) — same as all other packages |
| Config file | None — `go test ./...` |
| Quick run command | `go test -race ./pkg/internal/doctor/... ./pkg/cmd/kind/doctor/...` |
| Full suite command | `go test -race ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DIAG-01 | `decode` scans docker logs and events, matches patterns | unit | `go test -race ./pkg/internal/doctor/... -run TestRunDecode` | No — Wave 0 |
| DIAG-01 | `decode` runs as `kinder doctor decode` subcommand | unit | `go test -race ./pkg/cmd/kind/doctor/... -run TestDecodeCmd` | No — Wave 0 |
| DIAG-02 | ≥15 patterns in catalog | unit | `go test -race ./pkg/internal/doctor/... -run TestCatalogCount` | No — Wave 0 |
| DIAG-03 | Each match has 4 fields (pattern, explanation, fix, link) | unit | `go test -race ./pkg/internal/doctor/... -run TestDecodeMatch_Fields` | No — Wave 0 |
| DIAG-04 | `--auto-fix` applies only whitelisted remediations | unit | `go test -race ./pkg/internal/doctor/... -run TestApplyAutoFix` | No — Wave 0 |
| DIAG-04 | `--auto-fix` skips NeedsRoot items when not root | unit | `go test -race ./pkg/internal/doctor/... -run TestAutoFix_SkipsRoot` | No — Wave 0 |

### Wave 0 Gaps
- [ ] `pkg/internal/doctor/decode_test.go` — covers DIAG-01, DIAG-02, DIAG-03
- [ ] `pkg/internal/doctor/decode_autofix_test.go` — covers DIAG-04
- [ ] `pkg/cmd/kind/doctor/decode/decode_test.go` — covers DIAG-01 (CLI layer)

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Converting `kinder doctor` to a parent command (adding `run` + `decode` subcommands) is acceptable as a breaking change in v2.3 | Integration plan | If backward-compat is required, keep `doctor.RunE` and add `decode` as peer subcommand — both work |
| A2 | In-node kubectl (`docker exec <cp> kubectl --kubeconfig=/etc/kubernetes/admin.conf get events`) is the right approach for event collection | Log/Event Collection | If the CP is paused or events require host kubeconfig, need to fall back to host kubectl with tempfile (Phase 49-02 pattern) |
| A3 | `kubectl get events` does not support `--since` flag (needs client-side filtering) | Pitfall 4 | Verify: run `kubectl get events --since=30m` — if it works in your kubectl version, simplify the collection |
| A4 | Auto-fix for inotify limits requires root (`NeedsRoot=true`) | Auto-fix whitelist | If `/proc/sys/fs/inotify/` is writable without root in some configs, NeedsRoot can be false |

---

## Open Questions

1. **`kinder doctor` command tree restructure — parent or keep RunE?**
   - What we know: Phase 48 used the "parent with no RunE" pattern for `kinder snapshot`. Phase 49 used a single command with no subcommands for `kinder dev`.
   - What's unclear: Whether existing CI/docs reference `kinder doctor` directly (would break if RunE is removed).
   - Recommendation: Check `ROADMAP.md`, `README.md`, and any CI scripts for bare `kinder doctor` invocations. If none, use the clean parent pattern. If found, keep RunE and add `decode` as an additive subcommand.

2. **Time window default: 30m or "last N lines"?**
   - What we know: `docker logs --since 30m` works. `docker logs --tail 500` also works.
   - What's unclear: CI failures may be older than 30m; "last N lines" is more predictable.
   - Recommendation: Use `--since 30m` as default with flag override. Add `--tail` flag if users report misses.

3. **`--field-selector type!=Normal` for events — right filter?**
   - What we know: Kubernetes events have type "Normal" (informational) and "Warning" (problems). Filtering to Warning reduces noise.
   - What's unclear: Some addon-startup issues produce Normal events that still indicate a stuck state.
   - Recommendation: Default to no filter (show all events); let pattern catalog match only warning-type content.

4. **Should `decode` also scan containerd logs inside the node (via `docker exec <node> cat /var/log/containers/*.log`)?**
   - What we know: Docker logs capture the node entrypoint stdout/stderr, not pod-level containerd logs. Pod-level errors show up in kubectl events.
   - What's unclear: Some containerd errors only appear in `/var/log/containers/` inside the node.
   - Recommendation: Defer per phase "out of scope" — `decode` is one-shot against docker logs + kubectl events. Pod-level log tailing is explicitly deferred.

---

## Sources

### Primary (HIGH confidence)
- Codebase: `pkg/internal/doctor/check.go`, `format.go`, `mitigations.go`, `clusterskew.go`, `resumereadiness.go`, `localpath.go` — all patterns and conventions verified by direct file inspection
- Codebase: `pkg/cmd/kind/doctor/doctor.go`, `pkg/cmd/kind/status/status.go`, `pkg/cmd/kind/pause/pause.go` — CLI pattern verified
- Codebase: `pkg/internal/lifecycle/state.go`, `pause.go`, `resume.go` — lifecycle helper API verified
- Codebase: `pkg/cluster/internal/providers/common/cgroups.go` — regexp usage in doctor-adjacent code verified

### Secondary (MEDIUM confidence)
- [kind Known Issues](https://kind.sigs.k8s.io/docs/user/known-issues/) — pattern sources for KUB-01, KUB-02, KUB-05, DOCK-01, DOCK-03
- [kubeadm Troubleshooting Guide](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/troubleshooting-kubeadm/) — pattern sources for KUB-03, KUB-04, KUB-06, KADM-01..05

### Tertiary (LOW confidence — for pattern content, not architecture)
- [kind issue #3762](https://github.com/kubernetes-sigs/kind/issues/3762) — "failed to create cluster" patterns
- [kind issue #2731](https://github.com/kubernetes-sigs/kind/issues/2731) — cgroup v2 related failures
- [containerd issue #8139](https://github.com/containerd/containerd/issues/8139) — CRI runtime not running error string

---

## Metadata

**Confidence breakdown:**
- Codebase touchpoints: HIGH — all files verified by direct inspection
- Pattern catalog: MEDIUM-HIGH — 16 HIGH-confidence patterns sourced from official docs + codebase; 6 MEDIUM patterns from community sources
- Architecture (import boundaries, fn-var injection, CLI wiring): HIGH — all verified against existing Phase 47/48/49 code
- Auto-fix whitelist: MEDIUM — preconditions and idempotency reasoning is sound but not live-tested
- kubectl events `--since` caveat: MEDIUM — based on kubectl documentation knowledge [ASSUMED: kubectl ≤1.30 does not support `--since` on get events; verify against actual kubectl version in use]

**Research date:** 2026-05-06
**Valid until:** 2026-06-06 (stable Go codebase, no external moving parts)
