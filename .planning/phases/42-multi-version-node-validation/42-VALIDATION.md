# Phase 42: Multi-Version Node Validation - Validation

**Created:** 2026-04-08
**Source:** RESEARCH.md Validation Architecture (lines 496-526)

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing stdlib (no external framework) |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./pkg/internal/apis/config/... ./pkg/internal/doctor/... ./pkg/cmd/kind/get/nodes/... ./pkg/cluster/internal/create/...` |
| Full suite command | `go test ./...` |

## Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Test File | Automated Command | Plan |
|--------|----------|-----------|-----------|-------------------|------|
| MVER-01 | `--image` does not override explicit node images | unit | `pkg/cluster/internal/create/fixup_test.go` | `go test ./pkg/cluster/internal/create/... -run TestFixupOptions` | 42-01 Task 1 |
| MVER-01 | ExplicitImage propagated through V1Alpha4ToInternal | unit | `pkg/internal/apis/config/encoding/convert_test.go` | `go test ./pkg/internal/apis/config/encoding/... -run ExplicitImage` | 42-01 Task 1 |
| MVER-02 | Config validation rejects >3 minor skew | unit | `pkg/internal/apis/config/validate_test.go` | `go test ./pkg/internal/apis/config/... -run TestVersionSkew` | 42-01 Task 2 |
| MVER-02 | HA control-plane minor version mismatch rejected | unit | `pkg/internal/apis/config/validate_test.go` | `go test ./pkg/internal/apis/config/... -run TestVersionSkew` | 42-01 Task 2 |
| MVER-03 | Error table format has correct columns and hints | unit | `pkg/internal/apis/config/validate_test.go` | `go test ./pkg/internal/apis/config/... -run TestVersionSkewError` | 42-01 Task 2 |
| MVER-04 | Doctor reports skew violations on running cluster | unit | `pkg/internal/doctor/clusterskew_test.go` | `go test ./pkg/internal/doctor/... -run TestClusterNodeSkew` | 42-02 Task 1 |
| MVER-04 | Doctor detects config drift (image vs live version) | unit | `pkg/internal/doctor/clusterskew_test.go` | `go test ./pkg/internal/doctor/... -run TestClusterNodeSkew` | 42-02 Task 1 |
| MVER-05 | `get nodes` output includes VERSION/IMAGE/SKEW columns | unit | `pkg/cmd/kind/get/nodes/nodes_test.go` | `go test ./pkg/cmd/kind/get/nodes/... -run TestComputeSkew` | 42-02 Task 2 |

## Sampling Rate

- **Per task commit:** `go test ./pkg/internal/apis/config/... ./pkg/internal/doctor/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go test ./...` green before `/gsd-verify-work`

## Wave 0 Test Files (created by plans)

| File | Covers | Created By |
|------|--------|------------|
| `pkg/cluster/internal/create/fixup_test.go` | MVER-01 fixupOptions behavior | 42-01 Task 1 |
| `pkg/internal/apis/config/encoding/convert_test.go` | MVER-01 ExplicitImage propagation | 42-01 Task 1 |
| `pkg/internal/apis/config/validate_test.go` | MVER-02/03 skew validation (extend existing) | 42-01 Task 2 |
| `pkg/internal/doctor/clusterskew_test.go` | MVER-04 cluster skew + drift check | 42-02 Task 1 |
| `pkg/cmd/kind/get/nodes/nodes_test.go` | MVER-05 computeSkew helper | 42-02 Task 2 |

## Phase Gate Command

```bash
go test ./pkg/internal/apis/config/... ./pkg/internal/apis/config/encoding/... ./pkg/cluster/internal/create/... ./pkg/internal/doctor/... ./pkg/cmd/kind/get/nodes/... -v -count=1
```

All tests must pass before phase verification.
