---
phase: 20-provider-code-deduplication
plan: 02
subsystem: infra
tags: [go, providers, docker, nerdctl, common, deduplication, refactor]

# Dependency graph
requires:
  - phase: 20-01
    provides: common/node.go with shared Node struct and NodeRoleLabelKey

provides:
  - common/provision.go with GenerateMountBindings, GeneratePortMappings, CreateContainer
  - docker/create.go replacing docker/provision.go with common callers
  - nerdctl/create.go replacing nerdctl/provision.go with common callers
  - common/provision_test.go with migrated port mapping and mount binding tests

affects:
  - 20-03
  - podman provider (unchanged, kept its own generatePortMappings)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared provision helpers in common/ called by docker/ and nerdctl/ providers"
    - "binaryName parameter pattern for common.CreateContainer to support multiple runtimes"
    - "Port listener immediate release (not defer) inside per-mapping loop"

key-files:
  created:
    - pkg/cluster/internal/providers/common/provision.go
    - pkg/cluster/internal/providers/common/provision_test.go
    - pkg/cluster/internal/providers/docker/create.go
    - pkg/cluster/internal/providers/docker/create_test.go
    - pkg/cluster/internal/providers/nerdctl/create.go
    - pkg/cluster/internal/providers/nerdctl/create_test.go
  modified: []
  deleted:
    - pkg/cluster/internal/providers/docker/provision.go
    - pkg/cluster/internal/providers/docker/provision_test.go
    - pkg/cluster/internal/providers/nerdctl/provision.go
    - pkg/cluster/internal/providers/nerdctl/provision_test.go

key-decisions:
  - "CreateContainer takes binaryName as first parameter (not hardcoded 'docker') to support both docker and nerdctl with a single shared function"
  - "Podman's generatePortMappings stays in podman/provision.go unchanged: it has unique behavior (lowercase protocol, :0 strip for random ports)"
  - "Migrated port-mapping tests to common/ as package-internal tests (package common, not common_test) to access internal helpers"

patterns-established:
  - "Provider-specific create logic lives in create.go; shared helpers in common/"
  - "Port listeners are released immediately after acquiring the port, never deferred to function return"

# Metrics
duration: 15min
completed: 2026-03-03
---

# Phase 20 Plan 02: Provider Provision Deduplication Summary

**GenerateMountBindings, GeneratePortMappings, and CreateContainer extracted to common/provision.go; docker and nerdctl provision.go files deleted and replaced with create.go files calling common helpers**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-03T13:00:00Z
- **Completed:** 2026-03-03T13:15:00Z
- **Tasks:** 2
- **Files modified:** 10 (4 created, 4 deleted, 2 new)

## Accomplishments
- Extracted three byte-for-byte duplicate functions (generateMountBindings, generatePortMappings, createContainer) from docker and nerdctl into common/provision.go as exported GenerateMountBindings, GeneratePortMappings, CreateContainer
- Preserved the Phase 19 bug fix: port listeners released immediately inside the loop (not deferred), preventing file descriptor leaks under high port-mapping counts
- Deleted docker/provision.go and nerdctl/provision.go; replaced with docker/create.go and nerdctl/create.go calling common helpers
- Migrated shared port mapping and mount binding tests to common/provision_test.go; kept docker-specific (Test_parseSubnetOutput) and nerdctl-specific (TestGetSubnetsEmptyLines) tests in their respective packages
- podman/provision.go left completely untouched as it has distinct behavior (lowercase protocol, :0 strip for random ports)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create common/provision.go and common/provision_test.go** - `ee04f929` (feat)
2. **Task 2: Restructure docker and nerdctl provisions, delete old files** - `962c998d` (feat)

## Files Created/Modified

- `pkg/cluster/internal/providers/common/provision.go` - Shared provision helpers: GenerateMountBindings, GeneratePortMappings, CreateContainer
- `pkg/cluster/internal/providers/common/provision_test.go` - Migrated tests: static port, random port, port=-1, invalid protocol, empty mappings, port-release regression
- `pkg/cluster/internal/providers/docker/create.go` - Docker-specific provision functions calling common helpers (replaces provision.go)
- `pkg/cluster/internal/providers/docker/create_test.go` - Docker-specific Test_parseSubnetOutput test
- `pkg/cluster/internal/providers/nerdctl/create.go` - Nerdctl-specific provision functions calling common helpers (replaces provision.go)
- `pkg/cluster/internal/providers/nerdctl/create_test.go` - Nerdctl-specific TestGetSubnetsEmptyLines test
- DELETED: `pkg/cluster/internal/providers/docker/provision.go`
- DELETED: `pkg/cluster/internal/providers/docker/provision_test.go`
- DELETED: `pkg/cluster/internal/providers/nerdctl/provision.go`
- DELETED: `pkg/cluster/internal/providers/nerdctl/provision_test.go`

## Decisions Made
- CreateContainer takes binaryName as first parameter so a single function handles both docker and nerdctl without hardcoding the binary name
- Podman's generatePortMappings stays local in podman/provision.go because it differs in two observable ways: it uses lowercase protocol strings and strips ":0" suffixes for random ports
- Tests migrated to common/ as `package common` (not `common_test`) to allow direct access to package-internal helpers

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Provider code deduplication phase (20) complete: Node struct in common/node.go (plan 01), provision helpers in common/provision.go (plan 02)
- All providers (docker, nerdctl, podman) build and test cleanly
- go build ./... and go test ./... pass with zero failures

## Self-Check: PASSED

Files exist:
- FOUND: pkg/cluster/internal/providers/common/provision.go
- FOUND: pkg/cluster/internal/providers/common/provision_test.go
- FOUND: pkg/cluster/internal/providers/docker/create.go
- FOUND: pkg/cluster/internal/providers/docker/create_test.go
- FOUND: pkg/cluster/internal/providers/nerdctl/create.go
- FOUND: pkg/cluster/internal/providers/nerdctl/create_test.go
- MISSING (deleted as expected): docker/provision.go, nerdctl/provision.go

Commits verified:
- ee04f929: feat(20-02): create common/provision.go with shared provision helpers
- 962c998d: feat(20-02): migrate docker/nerdctl provision functions to create.go files

---
*Phase: 20-provider-code-deduplication*
*Completed: 2026-03-03*
