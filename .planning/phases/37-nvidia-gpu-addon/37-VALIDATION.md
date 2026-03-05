# Phase 37: NVIDIA GPU Addon - Validation

**Derived from:** 37-RESEARCH.md Validation Architecture section
**Phase:** 37-nvidia-gpu-addon

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none (go test ./...) |
| Quick run command | `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/... ./pkg/cmd/kind/doctor/... ./pkg/internal/apis/config/... ./pkg/apis/config/v1alpha4/...` |
| Full suite command | `go test ./...` |

## Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| GPU-01 | DaemonSet applied via kubectl apply | unit (FakeNode queue) | `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/...` | Wave 0 |
| GPU-02 | RuntimeClass applied before DaemonSet | unit (FakeNode call order) | `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/...` | Wave 0 |
| GPU-03 | NvidiaGPU *bool field, nil=false in conversion | unit | `go test ./pkg/internal/apis/config/...` | Wave 0 |
| GPU-04 | Non-Linux platforms return nil, log info message | unit (currentOS var override) | `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/...` | Wave 0 |
| GPU-05 | Doctor checks: driver, toolkit, docker runtime | unit (mock LookPath via interface OR skip on non-Linux CI) | `go test ./pkg/cmd/kind/doctor/...` | Wave 0 |
| GPU-06 | Pre-flight error before kubectl apply | unit (FakeNode expects 0 calls when pre-flight fails) | `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/...` | Wave 0 |
| SITE-02 | Documentation page exists with required sections | manual | N/A | Wave 0 |

**Note on GPU-04 testing:** `runtime.GOOS` cannot be mocked at test time. The `currentOS` package-level variable in nvidiagpu.go (created by Plan 37-01 Task 2) allows tests to override the platform guard. Tests set `currentOS = "darwin"` to verify the non-Linux skip path and `currentOS = "linux"` (via init()) to test the Linux execution path on any platform.

## Sampling Rate

- **Per task commit:** `go test ./pkg/cluster/internal/create/actions/installnvidiagpu/... ./pkg/internal/apis/config/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go test ./...` green before `/gsd:verify-work`

## Wave 0 Gaps

- [ ] `pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu_test.go` -- covers GPU-01, GPU-02, GPU-04, GPU-06
- [ ] `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-device-plugin.yaml` -- embedded manifest (GPU-01)
- [ ] `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-runtimeclass.yaml` -- embedded manifest (GPU-02)
- [ ] Doctor test additions for GPU-05 checks -- add to existing `pkg/cmd/kind/doctor/doctor_test.go` (may not exist yet; create)
- [ ] `pkg/internal/apis/config/default_test.go` addition -- test NvidiaGPU=false when nil (GPU-03)
