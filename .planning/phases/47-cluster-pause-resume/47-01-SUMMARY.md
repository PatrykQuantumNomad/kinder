---
phase: 47-cluster-pause-resume
plan: 01
subsystem: cluster-lifecycle
tags: [cobra, container-state, docker, podman, nerdctl, tabwriter, json-schema-migration]

# Dependency graph
requires:
  - phase: pre-existing
    provides: cluster.Provider, nodes.Node, nodeutils helpers, exec.Command, docker/podman/nerdctl runtime detection (clusterskew.go pattern)
provides:
  - Container-state visibility surface (ContainerState, ClusterStatus helpers) backing the kinder status, get clusters, get nodes, pause, and resume commands
  - kinder status [name] command with text + JSON output
  - kinder get clusters Status column (with breaking JSON schema migration to []{name,status})
  - kinder get nodes Status column derived from real container state (replaces hardcoded "Ready")
  - kinder pause and kinder resume command stubs registered in root.go (bodies arrive in 47-02 and 47-03)
  - Shared lifecycle.ResolveClusterName, lifecycle.ClassifyNodes, lifecycle.ProviderBinaryName helpers for plans 02-04
affects: [47-02-PLAN pause, 47-03-PLAN resume, 47-04-PLAN doctor readiness check, 47-05-PLAN docs]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Package-level Cmder injection (Cmder type + defaultCmder var) for testable container CLI calls without docker
    - Render-helper extraction (renderJSON/renderText) so command output shape is unit-testable without provider/runtime
    - ClusterLister minimal interface satisfied by *cluster.Provider for ResolveClusterName testability

key-files:
  created:
    - pkg/internal/lifecycle/state.go
    - pkg/internal/lifecycle/state_test.go
    - pkg/internal/lifecycle/doc.go
    - pkg/cmd/kind/status/status.go
    - pkg/cmd/kind/status/status_test.go
    - pkg/cmd/kind/get/clusters/clusters_test.go
    - pkg/cmd/kind/pause/pause.go
    - pkg/cmd/kind/resume/resume.go
  modified:
    - pkg/cmd/kind/get/clusters/clusters.go
    - pkg/cmd/kind/get/nodes/nodes.go
    - pkg/cmd/kind/root.go

key-decisions:
  - "lifecycle package located at pkg/internal/lifecycle (not pkg/cluster/internal/lifecycle as planned) so it is importable by pkg/cmd/kind/* consumers - Go internal-package rule blocked the planned path"
  - "Use package-level defaultCmder var + Cmder type for test injection, swap via t.Cleanup (matches pattern referenced in CONVENTIONS.md)"
  - "Refactor command output into renderText/renderJSON helpers to enable schema unit tests without provider/docker dependency"
  - "kinder status uses --output string flag (matches kinder get nodes); kinder pause/resume use --json bool flag (matches D-04 in CONTEXT.md, also Pattern 7 in 47-RESEARCH.md)"
  - "JSON schema for kinder get clusters --output json migrated from []string to []{name,status} - intentional breaking change accepted by user in CONTEXT.md"
  - "ClusterStatus maps any non-stopped non-running container state (paused, restarting, removing) to Error so the user investigates with kinder status"
  - "Pause/resume stubs return errors.New(...) (non-zero exit) rather than success - clearly visible in dev, won't accidentally run in CI before plans 02/03 ship"

patterns-established:
  - "Pattern: shared container-state helpers in lifecycle package - reuse from any new pause/resume/status surface area"
  - "Pattern: render-helper extraction - command output shape is unit-tested via render* helpers, full runE is integration territory"

# Metrics
duration: ~4h (mostly reading/research; implementation+test ~45min)
completed: 2026-05-03
---

# Phase 47 Plan 01: Cluster Status Visibility Summary

**Cluster status visibility surface (lifecycle helpers, kinder status command, get clusters Status column, real container state in get nodes) with kinder pause/resume command stubs wired into root.go for parallel implementation in plans 02/03**

## Performance

- **Duration:** ~4h elapsed (research-heavy plan; implementation+test ~45min)
- **Started:** 2026-05-03T15:38:50-04:00
- **Completed:** 2026-05-03T19:44:48Z
- **Tasks:** 3
- **Files modified:** 11 (8 created, 3 modified)

## Accomplishments

- New `pkg/internal/lifecycle` package with five exported helpers used by status, get clusters, get nodes, and (next) pause/resume.
- New `kinder status [cluster-name]` command renders per-node container state via tabwriter and JSON.
- `kinder get clusters` gains a Status column (text) and migrates JSON output from `[]string` to `[]{name,status}` (intentional breaking change per CONTEXT.md).
- `kinder get nodes` Status column now reflects real container state (`Ready`/`Stopped`/`Paused`/`Unknown`) instead of hardcoded `"Ready"`.
- `kinder pause` and `kinder resume` stub commands wired into `root.go` so plans 47-02 and 47-03 can land in parallel without root.go conflicts.
- Zero new module dependencies (go.mod and go.sum untouched).

## Task Commits

Each task was committed atomically (TDD where marked):

1. **Task 1 RED: lifecycle test cases** — `12c746c1` (test)
2. **Task 1 GREEN: lifecycle state helpers** — `0386dd09` (feat)
3. **Task 2 RED: status + clusters tests** — `d4f4cdbd` (test)
4. **Task 2 GREEN: status command + Status column + nodes fix + lifecycle move** — `2b69b481` (feat)
5. **Task 3: pause/resume stubs + root.go wiring** — `592161b9` (feat)

## Files Created/Modified

- `pkg/internal/lifecycle/doc.go` — package comment for the lifecycle package.
- `pkg/internal/lifecycle/state.go` — exports `ContainerState`, `ClusterStatus`, `ResolveClusterName`, `ClassifyNodes`, `ProviderBinaryName`, plus `Cmder` injection point and `ClusterLister` interface.
- `pkg/internal/lifecycle/state_test.go` — 13 unit tests using fake Cmder + fakeNode + fakeLister; no docker dependency.
- `pkg/cmd/kind/status/status.go` — new `kinder status [cluster-name]` cobra command with `--output` flag, renderText/renderJSON helpers, collectNodeStatus that uses lifecycle helpers.
- `pkg/cmd/kind/status/status_test.go` — schema-shape and tabwriter-column tests.
- `pkg/cmd/kind/get/clusters/clusters.go` — adds `clusterInfo` struct, `collectClusterInfos`, `renderText`, `renderJSON`; switches to tabwriter; consumes `lifecycle.ClusterStatus` and `lifecycle.ProviderBinaryName`.
- `pkg/cmd/kind/get/clusters/clusters_test.go` — JSON schema migration tests (new []{name,status} shape, empty array stays `[]`) and STATUS column text test.
- `pkg/cmd/kind/get/nodes/nodes.go` — adds `state` field to internal `raw` struct, calls `lifecycle.ContainerState` per node, replaces hardcoded `status := "Ready"` with a switch mapping container state to `Ready`/`Stopped`/`Paused`/`Unknown`.
- `pkg/cmd/kind/pause/pause.go` — stub command exporting `NewCommand`; flags `--timeout int (30)` and `--json bool (false)`; RunE returns `"kinder pause: not yet implemented (phase 47 plan 02)"`.
- `pkg/cmd/kind/resume/resume.go` — stub command exporting `NewCommand`; flags `--timeout int (30)`, `--wait int (300)`, `--json bool (false)`; RunE returns `"kinder resume: not yet implemented (phase 47 plan 03)"`.
- `pkg/cmd/kind/root.go` — adds imports for `pause`, `resume`, `status` packages and three `cmd.AddCommand(...)` calls.

## Exported Symbols (for plans 02, 03, 04)

From `pkg/internal/lifecycle`:

```go
// Container-state queries
func ContainerState(binaryName, containerName string) (string, error)
func ClusterStatus(binaryName string, allNodes []nodes.Node) string  // "Running" | "Paused" | "Error"

// CLI ergonomics
type ClusterLister interface { List() ([]string, error) }
func ResolveClusterName(args []string, lister ClusterLister) (string, error)

// Node classification
func ClassifyNodes(allNodes []nodes.Node) (cp, workers []nodes.Node, lb nodes.Node, err error)

// Runtime auto-detect
func ProviderBinaryName() string  // "docker" | "podman" | "nerdctl" | ""

// Test injection point (unexported package var, set via t.Cleanup pattern)
type Cmder func(name string, args ...string) exec.Cmd
```

`*cluster.Provider` satisfies `ClusterLister` structurally — pass it directly.

## JSON Schema Migration (Breaking Change)

`kinder get clusters --output json` previously emitted a bare array of cluster-name strings:

```json
["kind", "dev"]
```

It now emits an array of objects with `name` and `status`:

```json
[{"name":"kind","status":"Running"},{"name":"dev","status":"Paused"}]
```

This is the intentional break documented in `47-CONTEXT.md` (Cluster list integration: add Status column). Empty case still encodes as `[]` (not `null`), preserving the prior no-null contract.

`kinder get nodes --output json` is unchanged in shape — only the value of the `Status` field changes from constant `"Ready"` to one of `Ready`/`Stopped`/`Paused`/`Unknown` based on container state.

## Decisions Made

See frontmatter `key-decisions` for the full list. Highlights:

- **lifecycle relocated:** Plan asked for `pkg/cluster/internal/lifecycle/`; Go's internal-package rule blocks `pkg/cmd/kind/...` from importing under `pkg/cluster/internal/`. Moved to `pkg/internal/lifecycle/` which is reachable by every package under `pkg/`. Documented as Rule 3 deviation below.
- **Stubs return errors, not success:** Cleaner signal during development and CI than a "not yet implemented but exit 0" stub.
- **Output flag inconsistency is by design:** `status`/`get clusters`/`get nodes` use `--output string` (matches existing kinder convention); `pause`/`resume` use `--json bool` (D-04 in CONTEXT.md). The plan calls this out explicitly.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Relocated lifecycle package from `pkg/cluster/internal/lifecycle` to `pkg/internal/lifecycle`**

- **Found during:** Task 2 (first build of status/clusters/nodes after wiring lifecycle imports).
- **Issue:** Go's internal-package visibility rule restricts `pkg/cluster/internal/...` to consumers under `pkg/cluster/...`. The plan's path made the package unimportable from `pkg/cmd/kind/status`, `pkg/cmd/kind/get/clusters`, and `pkg/cmd/kind/get/nodes` — exactly the three consumers Task 2 needed. Build failed with `use of internal package ... not allowed`.
- **Fix:** `git mv` all three lifecycle files to `pkg/internal/lifecycle/`. Updated import paths in three command files. The package is still internal (private to the `pkg/...` tree) so the original encapsulation intent is preserved.
- **Files affected:** `pkg/internal/lifecycle/{state.go,state_test.go,doc.go}` (new locations), `pkg/cmd/kind/status/status.go`, `pkg/cmd/kind/get/clusters/clusters.go`, `pkg/cmd/kind/get/nodes/nodes.go` (import path updates).
- **Verification:** `go build ./...` and full `go test ./pkg/...` pass.
- **Committed in:** `2b69b481` (combined with the rest of Task 2 GREEN).
- **Forward note for plans 02/03/04:** import path is `sigs.k8s.io/kind/pkg/internal/lifecycle`, NOT `sigs.k8s.io/kind/pkg/cluster/internal/lifecycle` as the plan and frontmatter list. Update those plan files when scheduling 47-02 / 47-03 / 47-04.

**2. [Rule 2 - Missing Critical] Removed unused `nodeLister` interface in status package**

- **Found during:** Task 2 build after lifecycle move.
- **Issue:** I had introduced a `nodeLister` interface to make `collectNodeStatus` "testable", but Go won't auto-convert `[]nodes.Node` to `[]nodeLister`, so the call site failed to compile. The interface added no test value either (status tests target `renderText`/`renderJSON`, not the collector).
- **Fix:** Dropped the interface, `collectNodeStatus` accepts `[]nodes.Node` directly; added the `nodes` import.
- **Files modified:** `pkg/cmd/kind/status/status.go`.
- **Verification:** `go test ./pkg/cmd/kind/status/...` passes.
- **Committed in:** `2b69b481`.

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 dead-code cleanup). Both necessary for build correctness. No scope creep.

**Impact on downstream plans:** Plans 47-02, 47-03, and 47-04 must reference `sigs.k8s.io/kind/pkg/internal/lifecycle` as the import path. Their plan files currently say `pkg/cluster/internal/lifecycle/state.go` in `files_modified` — that should be updated to `pkg/internal/lifecycle/state.go` when those plans are scheduled.

## Issues Encountered

None beyond the two deviations above.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- All five lifecycle helpers are exported and documented with godoc.
- `kinder status [name]` works end-to-end (read-only, safe to ship).
- `kinder get clusters` and `kinder get nodes` show real cluster status.
- `kinder pause`/`kinder resume` registered and discoverable (`--help` shows flags), bodies stubbed pending plan 02/03.
- No blockers for plan 47-02 (pause body) or plan 47-03 (resume body); they can run in parallel since both touch only their own command package and not root.go.
- Plan 47-04 (doctor readiness check) can consume `lifecycle.ClassifyNodes` and `lifecycle.ContainerState` without further plumbing.

## Self-Check: PASSED

All 12 created/modified files verified present on disk. All 5 task commits verified present in git log (`12c746c1`, `0386dd09`, `d4f4cdbd`, `2b69b481`, `592161b9`).

---
*Phase: 47-cluster-pause-resume*
*Plan: 01*
*Completed: 2026-05-03*
