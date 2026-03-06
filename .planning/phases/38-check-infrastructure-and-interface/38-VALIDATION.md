# Phase 38: Check Infrastructure and Interface - Validation

**Generated:** 2026-03-06
**Source:** 38-RESEARCH.md Validation Architecture section

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None needed -- `go test ./...` works |
| Quick run command | `go test ./pkg/internal/doctor/... -count=1` |
| Full suite command | `go test ./... -count=1` |

## Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INFRA-01 | Check interface with Name/Category/Platforms/Run | unit | `go test ./pkg/internal/doctor/... -run TestCheckInterface -count=1` | No -- Wave 0 |
| INFRA-02 | Result type with ok/warn/fail/skip and category-grouped output | unit | `go test ./pkg/internal/doctor/... -run TestFormat -count=1` | No -- Wave 0 |
| INFRA-03 | AllChecks() registry with platform filtering | unit | `go test ./pkg/internal/doctor/... -run TestAllChecks -count=1` | No -- Wave 0 |
| INFRA-04 | SafeMitigation struct and ApplySafeMitigations skeleton | unit | `go test ./pkg/internal/doctor/... -run TestMitigation -count=1` | No -- Wave 0 |
| INFRA-06 | Existing checks migrated produce correct results | unit | `go test ./pkg/internal/doctor/... -run TestRuntime -count=1` | No -- Wave 0 |

## Sampling Rate

- **Per task commit:** `go test ./pkg/internal/doctor/... -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before verify-work

## Wave 0 Gaps

- [ ] `pkg/internal/doctor/check_test.go` -- covers INFRA-01, INFRA-03 (interface, registry, platform filter)
- [ ] `pkg/internal/doctor/format_test.go` -- covers INFRA-02 (output formatting, JSON envelope)
- [ ] `pkg/internal/doctor/runtime_test.go` -- covers INFRA-06 (container-runtime check migration)
- [ ] `pkg/internal/doctor/tools_test.go` -- covers INFRA-06 (kubectl check migration)
- [ ] `pkg/internal/doctor/gpu_test.go` -- covers INFRA-06 (NVIDIA checks migration)
- [ ] `pkg/internal/doctor/mitigations_test.go` -- covers INFRA-04 (SafeMitigation skeleton)

## Coverage Notes

All Wave 0 test files are created within the TDD tasks in Plans 38-01 and 38-02. No pre-existing test files need updating. All tasks have `<automated>` verify commands with `go test` invocations.
