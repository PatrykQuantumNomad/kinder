---
phase: 48-cluster-snapshot-restore
plan: 05
subsystem: cli
tags: [cobra, snapshot, tabwriter, yaml, kinder, cli]

# Dependency graph
requires:
  - phase: 48-04
    provides: snapshot.Create, snapshot.Restore, snapshot.NewStore, snapshot.PrunePlan, snapshot.BundleReader
  - phase: 47-06
    provides: lifecycle.ResolveClusterName, DurationVar convention, fn-injection test pattern

provides:
  - "kinder snapshot create [cluster] [snap-name] — full cluster snapshot with --name/--json flags"
  - "kinder snapshot restore [cluster] <snap-name> — hard-overwrite restore, no --yes flag"
  - "kinder snapshot list [cluster] — NAME/AGE/SIZE/K8S/ADDONS/STATUS tabwriter table + JSON + YAML"
  - "kinder snapshot show [cluster] <snap-name> — vertical key/value metadata layout + JSON + YAML"
  - "kinder snapshot prune [cluster] — keep-last/older-than/max-size retention with y/N prompt"
  - "root.go registers snapshot group between resume and status"
  - "helpers.go: humanizeBytes, humanizeAge, formatAddons, parseSize (base-2 K/M/G/T)"

affects:
  - "48-06 — integration/e2e tests and Phase 48 final verification"
  - "any plan that documents kinder CLI surface"

tech-stack:
  added: ["sigs.k8s.io/yaml (list/show YAML output)", "text/tabwriter (list table)"]
  patterns:
    - "package-level fn injection (createFn/restoreFn/listFn/showFn/pruneStoreFn) mirrors pause.go:51"
    - "positional arg disambiguation: 2 args = cluster+snap, 1 arg = snap (or cluster for create)"
    - "cobra.DurationVar for --older-than, cobra.IntVar for --keep-last (count not time)"
    - "CONTEXT.md locked: no --yes on restore; prune refuses no-flag invocation"
    - "ADDONS column truncated at 50 runes by default; --no-trunc bypasses"

key-files:
  created:
    - pkg/cmd/kind/snapshot/snapshot.go
    - pkg/cmd/kind/snapshot/helpers.go
    - pkg/cmd/kind/snapshot/create.go
    - pkg/cmd/kind/snapshot/create_test.go
    - pkg/cmd/kind/snapshot/restore.go
    - pkg/cmd/kind/snapshot/restore_test.go
    - pkg/cmd/kind/snapshot/list.go
    - pkg/cmd/kind/snapshot/list_test.go
    - pkg/cmd/kind/snapshot/show.go
    - pkg/cmd/kind/snapshot/show_test.go
    - pkg/cmd/kind/snapshot/prune.go
    - pkg/cmd/kind/snapshot/prune_test.go
  modified:
    - pkg/cmd/kind/root.go

key-decisions:
  - "restore has NO --yes flag — CONTEXT.md locked; hard overwrite is the design"
  - "prune enforces at least one policy flag — CONTEXT.md locked; never delete on naked invocation"
  - "show uses vertical key/value layout — planner discretion for addon map readability"
  - "ADDONS column truncation threshold = 50 runes to fit typical 120-col terminal"
  - "parseSize uses base-2 multipliers (1K=1024) for K/M/G/T; no custom 'd' for duration (rely on Go ParseDuration h/m/s)"
  - "fn injection pattern (var createFn = snapshot.Create) used for all 5 subcommands — no real cluster needed in unit tests"

patterns-established:
  - "Snapshot CLI fn-injection: var xFn = snapshot.X; withXFn(t, fake) swaps for tests"
  - "pruneStoreFns struct bundles list+delete fns for atomic injection in prune tests"
  - "All cluster-targeting snapshot commands accept positional cluster name matching Phase 47-06 pattern"

duration: 9min
completed: "2026-05-06"
---

# Phase 48 Plan 05: Snapshot CLI Commands Summary

**Five Cobra subcommands (create/restore/list/show/prune) wiring Plan 04 orchestrators into kinder CLI with fn-injection unit tests, tabwriter table output, JSON/YAML formats, and CONTEXT.md-locked safety constraints**

## Performance

- **Duration:** 9 min
- **Started:** 2026-05-06T13:18:36Z
- **Completed:** 2026-05-06T13:27:42Z
- **Tasks:** 3
- **Files modified:** 13 (12 created, 1 modified)

## Accomplishments

- `kinder snapshot {create,restore,list,show,prune}` all wired and unit-tested with package-level fn injection
- CONTEXT.md locked decisions enforced: restore has no --yes flag; prune refuses no-flag invocation with clear error listing all 3 policy flags
- list emits NAME/AGE/SIZE/K8S/ADDONS/STATUS tabwriter table with ADDONS truncation at 50 runes (--no-trunc to bypass) + JSON + YAML via sigs.k8s.io/yaml
- show uses vertical key/value layout (planner discretion) with full addon map display + JSON + YAML
- All time-typed flags use cobra.DurationVar (--older-than); --keep-last uses IntVar (count, not time)
- root.go registers `snapshot.NewCommand()` between resume and status alphabetically

## Task Commits

1. **Task 1: snapshot group + create/restore + root.go** - `7d458993` (feat)
2. **Task 2: list/show with table/JSON/YAML** - `52794cbb` (feat)
3. **Task 3: prune with no-flag refusal + y/N prompt** - `1c1ddef2` (feat)

## Files Created/Modified

- `pkg/cmd/kind/snapshot/snapshot.go` - NewCommand() group registering all 5 subcommands
- `pkg/cmd/kind/snapshot/helpers.go` - humanizeBytes, humanizeAge, formatAddons, sortedKeys, truncateString, parseSize (base-2 K/M/G/T)
- `pkg/cmd/kind/snapshot/create.go` - createFn injection, MaximumNArgs(2), --name/--json
- `pkg/cmd/kind/snapshot/create_test.go` - 6 tests (defaults, two-positionals, --name, JSON, text, error)
- `pkg/cmd/kind/snapshot/restore.go` - restoreFn injection, RangeArgs(1,2), no --yes flag
- `pkg/cmd/kind/snapshot/restore_test.go` - 5 tests (1-arg, 2-args, no-args reject, compat error, no --yes assertion)
- `pkg/cmd/kind/snapshot/list.go` - listFn injection, tabwriter table, --output json/yaml, --no-trunc
- `pkg/cmd/kind/snapshot/list_test.go` - 5 tests (table, JSON, YAML, empty, truncation)
- `pkg/cmd/kind/snapshot/show.go` - showFn injection, vertical layout, --output json/yaml
- `pkg/cmd/kind/snapshot/show_test.go` - 4 tests (vertical, JSON, not-found, arg disambiguation)
- `pkg/cmd/kind/snapshot/prune.go` - pruneStoreFn injection, no-flag refusal, y/N prompt, --yes skip, kinderrors.NewAggregate
- `pkg/cmd/kind/snapshot/prune_test.go` - 7 tests (no-flags, no-victims, prompt-yes, prompt-no, --yes, delete error, parseSize table)
- `pkg/cmd/kind/root.go` - added snapshot.NewCommand() import and AddCommand registration

## Decisions Made

- **restore no --yes flag:** CONTEXT.md locked — hard overwrite is intentional signaling
- **prune no-flag refusal:** CONTEXT.md locked — error message explicitly lists all 3 policy flags
- **show vertical layout:** Planner discretion for addon map readability (vs. JSON default)
- **ADDONS truncation threshold = 50 runes:** Tuned for 120-col terminal with other columns consuming ~70 cols
- **parseSize base-2:** 1K=1024, 1M=1024^2, etc.; case-insensitive, accepts KiB/MiB/GiB/TiB variants
- **No custom 'd' duration suffix:** Rely on Go ParseDuration (h/m/s); document 168h for 7 days in flag help (deferred per Research open question 4)
- **pruneStoreFns struct:** Bundles list+delete injection together for cleaner test setup

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test args prefix — NewCommand() is the group command, not root**
- **Found during:** Task 1 (test execution)
- **Issue:** Tests passed `["snapshot", "create", ...]` as args to `NewCommand()` which IS the snapshot group command — cobra rejected "snapshot" as unknown subcommand
- **Fix:** Changed all test args to drop the `"snapshot"` prefix (e.g., `["create", "mycluster"]`)
- **Files modified:** create_test.go, restore_test.go, list_test.go, show_test.go, prune_test.go
- **Committed in:** 7d458993, 52794cbb, 1c1ddef2 (part of respective task commits)

**2. [Rule 1 - Bug] Fixed JSON key assertions in TestCreateCommand_JSONOutput**
- **Found during:** Task 1 (test execution)
- **Issue:** Test checked for lowercase JSON keys (`snapName`, `path`, `size`) but CreateResult has no json tags so Go encodes as `SnapName`, `Path`, `Size`
- **Fix:** Updated test to use actual capitalized field names
- **Files modified:** create_test.go
- **Committed in:** 7d458993

---

**Total deviations:** 2 auto-fixed (both Rule 1 bugs — test setup issues, not logic bugs)
**Impact on plan:** No behavior changes; all fixes were test correctness corrections.

## Issues Encountered

None — build was clean from the first attempt, all 3 tasks compiled on first write.

## Self-Check verification

- `go test -race ./pkg/cmd/kind/snapshot/...` passes: ok 1.727s
- `go vet ./...` clean
- `go build ./...` clean
- root.go line 94: `cmd.AddCommand(snapshot.NewCommand(logger, streams))` between resume and status
- restore.go: no `--yes` flag registered
- prune.go: `cobra.DurationVar` for `--older-than`; no IntVar for time

## Next Phase Readiness

- All 5 snapshot CLI commands are wired, tested, and building clean
- Plan 06 (integration/e2e tests + Phase 48 final verification) can proceed
- `kinder snapshot --help` will list all 5 subcommands after `go build`

---
*Phase: 48-cluster-snapshot-restore*
*Completed: 2026-05-06*
