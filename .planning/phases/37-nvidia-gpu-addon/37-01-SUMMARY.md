---
phase: 37-nvidia-gpu-addon
plan: 01
subsystem: infra
tags: [nvidia, gpu, kubernetes, daemonset, runtimeclass, device-plugin, addon, embed]

# Dependency graph
requires: []
provides:
  - NvidiaGPU *bool field in v1alpha4 Addons struct (opt-in, default false)
  - NvidiaGPU bool field in internal Addons struct
  - boolValOptIn helper for opt-in conversion semantics (nil = false)
  - installnvidiagpu package with embedded NVIDIA device plugin DaemonSet v0.17.1 and RuntimeClass
  - Platform guard via currentOS package-level var (not runtime.GOOS directly)
  - checkPrerequisites package-level var for test injection
  - Three pre-flight host checks: nvidia-smi, nvidia-ctk, nvidia Docker runtime
  - NVIDIA GPU addon wired into create.go wave1 list
affects:
  - 37-02 (doctor checks use the same checkHostPrerequisites pattern)
  - 37-03 (unit tests inject currentOS and checkPrerequisites vars)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - boolValOptIn conversion helper for opt-in (false-default) addon fields
    - Package-level vars for OS and prerequisite check injection (enables testing)
    - osexec alias for os/exec to avoid collision with kind's exec package
    - exec.OutputLines(exec.Command(...)) pattern for Docker info check

key-files:
  created:
    - pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu.go
    - pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-device-plugin.yaml
    - pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-runtimeclass.yaml
  modified:
    - pkg/apis/config/v1alpha4/types.go
    - pkg/apis/config/v1alpha4/zz_generated.deepcopy.go
    - pkg/internal/apis/config/types.go
    - pkg/internal/apis/config/convert_v1alpha4.go
    - pkg/internal/apis/config/default_test.go
    - pkg/cluster/internal/create/create.go

key-decisions:
  - "NvidiaGPU uses boolValOptIn (nil=false) unlike all other addons which use boolVal (nil=true) — GPU is opt-in"
  - "currentOS and checkPrerequisites are package-level vars so Plan 37-03 tests can inject overrides without build tags"
  - "osexec imported as alias for os/exec to avoid collision with kind's exec package"
  - "nvidiaRuntimeInDocker uses kind exec package (exec.OutputLines/exec.Command) for Docker info check"
  - "NVIDIA GPU placed in wave1 (parallel) — no inter-addon dependencies"

patterns-established:
  - "boolValOptIn pattern: add a separate helper for opt-in fields instead of reusing boolVal"
  - "Testability pattern: expose OS guard and prerequisite function as package-level vars"

# Metrics
duration: 3min
completed: 2026-03-05
---

# Phase 37 Plan 01: Config API and GPU Addon Action Summary

**NvidiaGPU opt-in addon with embedded NVIDIA device plugin v0.17.1, RuntimeClass, three pre-flight host checks, and wave1 wiring via package-level testability vars**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-05T00:23:07Z
- **Completed:** 2026-03-05T00:25:35Z
- **Tasks:** 2
- **Files modified:** 8 (5 modified, 3 created)

## Accomplishments
- Extended v1alpha4 and internal config Addons structs with NvidiaGPU field (opt-in, *bool → bool)
- Added boolValOptIn helper for opt-in conversion semantics (nil = false, distinct from existing boolVal)
- Created installnvidiagpu package with embedded RuntimeClass and NVIDIA device plugin DaemonSet v0.17.1
- Implemented three pre-flight host checks with actionable error messages (nvidia-smi, nvidia-ctk, Docker runtime)
- Platform guard on currentOS package-level var (not runtime.GOOS directly) for test injection
- Wired NVIDIA GPU into create.go wave1 alongside other parallel addons

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend config API with NvidiaGPU field** - `9d50f677` (feat)
2. **Task 2: Create installnvidiagpu package and wire into create.go** - `89fd977a` (feat)

**Plan metadata:** (committed with docs commit below)

## Files Created/Modified
- `pkg/apis/config/v1alpha4/types.go` - Added NvidiaGPU *bool field to Addons struct
- `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` - Added NvidiaGPU *bool deepcopy block
- `pkg/internal/apis/config/types.go` - Added NvidiaGPU bool field to internal Addons struct
- `pkg/internal/apis/config/convert_v1alpha4.go` - Added boolValOptIn helper and NvidiaGPU conversion
- `pkg/internal/apis/config/default_test.go` - Added NvidiaGPU opt-in test cases
- `pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu.go` - Main addon action
- `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-device-plugin.yaml` - Embedded DaemonSet
- `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-runtimeclass.yaml` - Embedded RuntimeClass
- `pkg/cluster/internal/create/create.go` - Added installnvidiagpu import and wave1 entry

## Decisions Made
- Used boolValOptIn (nil=false) for NvidiaGPU conversion — first opt-in addon in the project. All other addons use boolVal (nil=true). This is correct: GPU hardware is not universally available.
- Exposed currentOS and checkPrerequisites as package-level vars for test injection in Plan 37-03 without build tags. This allows tests to simulate both Linux and non-Linux behavior.
- Used `osexec "os/exec"` import alias to avoid collision with kind's exec package (which is imported as bare `exec`).
- nvidiaRuntimeInDocker uses kind's exec package (exec.OutputLines/exec.Command) for consistency with the rest of the codebase.
- NVIDIA GPU addon placed in wave1 (parallel group) — it has no inter-addon dependencies.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. NVIDIA host prerequisites are documented in error messages returned by the pre-flight check.

## Next Phase Readiness
- Config API and addon action are complete; Plan 37-02 (doctor checks) and Plan 37-03 (unit tests) can proceed
- currentOS and checkPrerequisites vars are available for test injection in 37-03
- installnvidiagpu package compiles cleanly; go build ./... and go vet ./... pass

---
*Phase: 37-nvidia-gpu-addon*
*Completed: 2026-03-05*
