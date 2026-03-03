---
phase: 20-provider-code-deduplication
plan: 01
subsystem: infra
tags: [go, providers, docker, nerdctl, podman, refactor, deduplication]

# Dependency graph
requires:
  - phase: 19-bug-fixes
    provides: stable provider base with correct sort ordering and port mapping fixes
provides:
  - Shared common/node.go with exported Node struct parameterized by BinaryName
  - Exported NodeRoleLabelKey constant in common/constants.go
  - go.mod with go 1.21.0 and toolchain go1.26.0 directive
affects:
  - phase-21
  - phase-22
  - phase-23
  - phase-24

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "BinaryName-parameterized shared Node struct: single struct serves docker/nerdctl/podman via BinaryName field"
    - "Exported NodeRoleLabelKey: shared constant replaces three identical unexported copies"

key-files:
  created:
    - pkg/cluster/internal/providers/common/node.go
  modified:
    - pkg/cluster/internal/providers/common/constants.go
    - pkg/cluster/internal/providers/docker/provider.go
    - pkg/cluster/internal/providers/nerdctl/provider.go
    - pkg/cluster/internal/providers/podman/provider.go
    - go.mod

key-decisions:
  - "Use exported Node struct with BinaryName string field rather than interface/callback pattern for provider dispatch"
  - "Keep nodeCmd unexported (lowercase) — it is an implementation detail returned as exec.Cmd interface"
  - "Node.Role() references NodeRoleLabelKey from constants.go in same package, not hardcoded string"
  - "go.mod updated to go 1.21.0 with toolchain go1.26.0 to enable toolchain directive support"

patterns-established:
  - "Provider node factory: return &common.Node{Name: name, BinaryName: <binary>} pattern for all three providers"

# Metrics
duration: 15min
completed: 2026-03-03
---

# Phase 20 Plan 01: Provider Code Deduplication Summary

**Eliminated ~525 lines of copy-paste by extracting a BinaryName-parameterized common.Node struct, deleting docker/nerdctl/podman node.go files, and updating go.mod to go 1.21.0 with toolchain directive**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-03T13:20:00Z
- **Completed:** 2026-03-03T13:35:06Z
- **Tasks:** 2
- **Files modified:** 6 (+ 1 deleted x3)

## Accomplishments
- Created `pkg/cluster/internal/providers/common/node.go` with exported Node struct and unexported nodeCmd, eliminating ~500 lines of copy-paste across three providers
- Exported `NodeRoleLabelKey` from `common/constants.go` so the shared Node struct can reference it without hardcoding the label key string
- Updated all three provider `node()` factories (docker, nerdctl, podman) to return `&common.Node{...}` and deleted the now-redundant per-provider node.go files
- Updated go.mod from `go 1.17` to `go 1.21.0` with `toolchain go1.26.0` directive

## Task Commits

Each task was committed atomically:

1. **Task 1: Create common/node.go and update common/constants.go** - `a74cf6cf` (feat)
2. **Task 2: Update provider factories, delete old node.go files, update go.mod** - `5c9638de` (feat)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified
- `pkg/cluster/internal/providers/common/node.go` - New shared Node struct with BinaryName field and unexported nodeCmd; implements nodes.Node interface for all three providers
- `pkg/cluster/internal/providers/common/constants.go` - Added exported NodeRoleLabelKey constant alongside existing APIServerInternalPort
- `pkg/cluster/internal/providers/docker/provider.go` - node() factory returns &common.Node{Name: name, BinaryName: "docker"}
- `pkg/cluster/internal/providers/nerdctl/provider.go` - node() factory returns &common.Node{Name: name, BinaryName: p.binaryName}
- `pkg/cluster/internal/providers/podman/provider.go` - node() factory returns &common.Node{Name: name, BinaryName: "podman"}
- `go.mod` - Updated go 1.17 to go 1.21.0, added toolchain go1.26.0 directive
- `pkg/cluster/internal/providers/docker/node.go` - DELETED
- `pkg/cluster/internal/providers/nerdctl/node.go` - DELETED
- `pkg/cluster/internal/providers/podman/node.go` - DELETED

## Decisions Made
- Used exported `Node` struct with `BinaryName string` field (not interface/callback) for provider dispatch — the simplest approach since the only variation between providers is the binary name
- `nodeCmd` kept unexported (lowercase) — callers receive it as `exec.Cmd` interface, internal detail hidden
- `Node.Role()` uses `NodeRoleLabelKey` from the same package (no import cycle) rather than hardcoding the string
- go.mod bumped to 1.21.0 (minimum required for toolchain directive support) with toolchain go1.26.0 to match the compiler used for building

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
None - `go build ./...` and `go test ./...` both passed cleanly on first attempt.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Provider code deduplication complete; common/node.go is the single source of truth for container node behavior
- Phase 21 and beyond can reference common.Node directly if new provider features are needed
- No blockers; go.mod now supports toolchain directive for future Go version management

## Self-Check

### Verified files exist:
- `pkg/cluster/internal/providers/common/node.go` - FOUND
- `pkg/cluster/internal/providers/common/constants.go` - FOUND (with NodeRoleLabelKey)

### Verified commits exist:
- `a74cf6cf` - FOUND (feat(20-01): create common/node.go...)
- `5c9638de` - FOUND (feat(20-01): update provider factories...)

### Verified deletions:
- `pkg/cluster/internal/providers/docker/node.go` - DELETED (confirmed No such file)
- `pkg/cluster/internal/providers/nerdctl/node.go` - DELETED (confirmed No such file)
- `pkg/cluster/internal/providers/podman/node.go` - DELETED (confirmed No such file)

### go.mod verification:
- `go 1.21.0` - FOUND
- `toolchain go1.26.0` - FOUND

## Self-Check: PASSED

---
*Phase: 20-provider-code-deduplication*
*Completed: 2026-03-03*
