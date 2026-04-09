---
phase: 45-host-directory-mounting
verified: 2026-04-09T18:30:00Z
status: passed
score: 4/4
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 3/4
  gaps_closed:
    - "kinder doctor --config <path> injects ExtraMounts host paths into mount checks via mountPathConfigurable interface — both checks now run instead of always skipping"
  gaps_remaining: []
  regressions: []
---

# Phase 45: Host-Directory Mounting Verification Report

**Phase Goal:** Users can mount host directories into cluster nodes with clear pre-flight validation, explicit platform warnings, and documented guidance for wiring mounts through to pods via hostPath PVs
**Verified:** 2026-04-09T18:30:00Z
**Status:** passed
**Re-verification:** Yes — after SC-3 gap closure (plan 45-04)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Running `kinder create cluster` with an `extraMounts` entry pointing to a non-existent host path exits before any containers are created, with an error message identifying the missing path | VERIFIED | `validateExtraMounts()` in `create.go` (lines 462–488) iterates all node ExtraMounts, calls `os.Stat`, returns `errors.Errorf("node[%d] extraMount[%d]: host path %q does not exist", ...)`. Called before `p.Provision()`. 6 unit tests pass (TestValidateExtraMounts). No regression. |
| 2 | Specifying `propagation: HostToContainer` or `propagation: Bidirectional` on macOS or Windows emits a visible warning during cluster creation explaining that propagation is unsupported on Docker Desktop and defaults to `None` | VERIFIED | `logMountPropagationPlatformWarning()` in `create.go` (lines 490–510) switches on `runtime.GOOS == "darwin"/"windows"`, calls `logger.Warnf(...)` naming the propagation mode. Called before Provision(). 5 unit tests pass. No regression. |
| 3 | `kinder doctor` on macOS checks that a configured host mount path exists and that Docker Desktop file sharing is enabled for that path, reporting actionable guidance when either check fails | VERIFIED | `mountPathConfigurable` interface and `SetMountPaths` are in `check.go` (lines 91–105). Both `hostMountPathCheck` and `dockerDesktopFileSharingCheck` implement `setMountPaths` in `hostmount.go` (lines 48, 125). `doctor.go` `runE` loads config via `encoding.Load(flags.Config)`, calls `extractMountPaths`, then `doctor.SetMountPaths(paths)` before `RunAllChecks` when `--config` is provided. Without `--config`, mount checks skip as before (backward compat). All 20 tests pass (3 new SetMountPaths tests + 5 extractMountPaths tests + 17 pre-existing). |
| 4 | The website documents the two-hop mount pattern (host directory → node extraMount → pod hostPath PV) with a complete example YAML showing host dir mounted as a PV-backed volume | VERIFIED | `kinder-site/src/content/docs/guides/host-directory-mounting.md` (254 lines). Contains cluster config YAML with `extraMounts`, PersistentVolume with `hostPath`, PVC, Pod consuming the PVC. 9 extraMounts occurrences, 13 hostPath occurrences. Two-hop pattern documented. No regression. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/internal/doctor/check.go` | mountPathConfigurable interface and SetMountPaths function | VERIFIED | Lines 91–105. Interface unexported (internal), SetMountPaths exported. Iterates allChecks with type assertion. |
| `pkg/internal/doctor/hostmount.go` | setMountPaths method on both check types | VERIFIED | Line 48: `hostMountPathCheck.setMountPaths`. Line 125: `dockerDesktopFileSharingCheck.setMountPaths`. Both replace the getMountPaths closure. |
| `pkg/cmd/kind/doctor/doctor.go` | --config flag, extractMountPaths, runE wiring | VERIFIED | flagpole.Config field present. --config flag registered (line 53). extractMountPaths (lines 58–70) deduplicates by seen-map. runE wires config when non-empty (lines 80–88). |
| `pkg/cmd/kind/doctor/doctor_test.go` | Tests for extractMountPaths | VERIFIED | 5 test cases: no-nodes, no-extraMounts, single mount, multi-node deduplication, multi-node all unique. All pass. |
| `pkg/internal/doctor/hostmount_test.go` | Tests for setMountPaths on both checks + SetMountPaths | VERIFIED | TestHostMountPathCheck_SetMountPaths, TestDockerDesktopFileSharingCheck_SetMountPaths, TestSetMountPaths (global). All 3 new tests pass alongside 17 pre-existing tests. |
| `pkg/cluster/internal/create/create.go` | validateExtraMounts() and logMountPropagationPlatformWarning() | VERIFIED | Unchanged from initial verification. No regression. |
| `kinder-site/src/content/docs/guides/host-directory-mounting.md` | Complete guide for host directory mounting | VERIFIED | 254 lines, all YAML examples present, no regression. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `doctor.go` runE | `encoding.Load` | `encoding.Load(flags.Config)` at line 81 | WIRED | Guarded by `flags.Config != ""` — backward compat preserved. |
| `doctor.go` runE | `doctor.SetMountPaths` | `doctor.SetMountPaths(paths)` at line 86 | WIRED | Called only when paths non-empty after extractMountPaths. |
| `check.go` SetMountPaths | `hostmount.go` setMountPaths | `mountPathConfigurable` type assertion on allChecks (line 101) | WIRED | Both mount checks implement setMountPaths — type assertion succeeds for both. |
| `create.go` Cluster() | `validateExtraMounts()` | Direct call at line 161 | WIRED | Unchanged. |
| `create.go` Cluster() | `logMountPropagationPlatformWarning()` | Direct call at line 166 | WIRED | Unchanged. |
| `check.go` allChecks | `hostmount.go` constructors | newHostMountPathCheck() / newDockerDesktopFileSharingCheck() at lines 86–87 | WIRED | Unchanged. |
| `host-directory-mounting.md` | extraMounts + hostPath PV YAML examples | inline YAML in guide | WIRED | Unchanged. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `hostmount.go` hostMountPathCheck | `getMountPaths()` result | `setMountPaths` replaces closure with `func() []string { return paths }` where `paths` comes from `extractMountPaths(cfg)` | Yes — real ExtraMounts host paths from cluster config file | FLOWING (with --config); SKIP (without --config, by design) |
| `hostmount.go` dockerDesktopFileSharingCheck | `getMountPaths()` result | Same injection via `SetMountPaths` | Yes — same paths | FLOWING (with --config); SKIP (without --config, by design) |
| `create.go` validateExtraMounts | `cfg.Config.Nodes[].ExtraMounts` | Passed directly from ClusterOptions.Config | Yes — real cluster config at creation time | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| validateExtraMounts tests | `go test ./pkg/cluster/internal/create/... -run TestValidateExtraMounts -v` | 6/6 PASS | PASS |
| logMountPropagationPlatformWarning tests | `go test ./pkg/cluster/internal/create/... -run TestLogMountPropagation -v` | 5/5 PASS | PASS |
| doctor mount checks tests (pre-existing) | `go test ./pkg/internal/doctor/... -run TestHostMountPathCheck\|TestDockerDesktopFileSharingCheck\|TestIsPathCovered -v` | 17/17 PASS | PASS |
| SetMountPaths new tests | `go test ./pkg/internal/doctor/... -run TestSetMountPaths\|TestHostMountPathCheck_SetMountPaths\|TestDockerDesktopFileSharingCheck_SetMountPaths -v` | 3/3 PASS | PASS |
| extractMountPaths tests | `go test ./pkg/cmd/kind/doctor/... -v` | 5/5 PASS | PASS |
| Full doctor package | `go test ./pkg/internal/doctor/... -count=1` | All PASS | PASS |
| go vet | `go vet ./pkg/internal/doctor/... ./pkg/cmd/kind/doctor/...` | Clean | PASS |
| Full build | `go build ./...` | BUILD OK | PASS |
| Backward compat | `flags.Config == ""` branch in runE skips SetMountPaths entirely | Code inspection confirmed | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| MOUNT-01 | 45-01 | Host path existence validated before containers created | SATISFIED | validateExtraMounts() in create.go called before Provision(). Returns error with node/mount index and path. |
| MOUNT-02 | 45-01 | Platform propagation warning on macOS/Windows | SATISFIED | logMountPropagationPlatformWarning() warns once on darwin/windows for non-None propagation. |
| MOUNT-03 | 45-02 + 45-04 | kinder doctor checks mount path existence and Docker Desktop file sharing | SATISFIED | Check logic implemented and registered (45-02). Config injection via --config flag wired via mountPathConfigurable interface (45-04). kinder doctor --config cluster.yaml now runs both checks against real ExtraMounts paths. |
| MOUNT-04 | 45-03 | Website documents two-hop mount pattern | SATISFIED | host-directory-mounting.md guide with complete YAML (cluster config, PV, PVC, Pod), two-hop diagram, platform notes. |

### Anti-Patterns Found

None. All previously identified blocker stubs (getMountPaths hardcoded nil in production) are resolved by the mountPathConfigurable injection mechanism. No new TODO/FIXME/placeholder patterns found in modified files.

### Human Verification Required

None. All criteria are deterministically verifiable from code inspection and test results.

### Gaps Summary

No gaps remain. SC-3 gap closed by plan 45-04:

- `mountPathConfigurable` interface in `check.go` — unexported, implemented by both mount checks
- `SetMountPaths` exported function in `check.go` — iterates allChecks, type-asserts, injects paths
- `setMountPaths` methods on `hostMountPathCheck` and `dockerDesktopFileSharingCheck` in `hostmount.go`
- `--config` flag on `kinder doctor` in `doctor.go` — registered with descriptive usage string
- `extractMountPaths` helper in `doctor.go` — deduplicates host paths across all nodes via seen-map
- `runE` in `doctor.go` — loads config with `encoding.Load`, extracts paths, calls `doctor.SetMountPaths` before `RunAllChecks` when `--config` non-empty
- Without `--config`, `runE` skips `SetMountPaths` entirely — mount checks continue to skip (backward compat)
- 5 plan-04 commits verified (4d10a054, 8dea75a8, 6f4ce7b8, b1257689, 4953de3e)
- All tests pass: 20 doctor-package tests + 5 doctor-cmd tests + 11 create-package tests

Phase 45 goal is fully achieved.

---

_Verified: 2026-04-09T18:30:00Z_
_Verifier: Claude (gsd-verifier)_
