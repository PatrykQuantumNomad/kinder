---
phase: 37-nvidia-gpu-addon
verified: 2026-03-04T19:35:00Z
status: human_needed
score: 12/12 must-haves verified
re_verification: false
human_verification:
  - test: "On a Linux host with NVIDIA GPU: create a cluster with addons.nvidiaGPU: true, apply a pod requesting nvidia.com/gpu: 1, confirm it schedules and runs"
    expected: "Pod reaches Running/Completed state, kubectl logs shows GPU computation output"
    why_human: "Requires physical NVIDIA GPU hardware, NVIDIA drivers, nvidia-container-toolkit, and Linux host — cannot simulate in CI or macOS dev environment"
  - test: "On macOS or Windows: run kinder create cluster --config gpu-cluster.yaml (with nvidiaGPU: true). Confirm cluster creates successfully with no error, and the log shows the Linux-only skip message"
    expected: "Cluster creation completes. Log contains '* NVIDIA GPU addon: skipping on darwin (Linux only)'. No error returned."
    why_human: "Requires an actual cluster creation flow end-to-end — automated grep verified the code path but not the full create flow on a non-Linux host"
  - test: "On Linux WITHOUT nvidia-ctk installed: run kinder create cluster with nvidiaGPU: true. Confirm it fails fast (before any cluster nodes are created, or immediately at pre-flight)"
    expected: "Process exits with an error message containing actionable install commands (apt-get install nvidia-container-toolkit). The failure should occur before 10 minutes of cluster creation."
    why_human: "Requires real cluster invocation on Linux without NVIDIA toolkit present — the code path is verified via unit tests but end-to-end timing and behavior need human confirmation"
  - test: "On Linux: run kinder doctor and confirm NVIDIA checks appear in output with correct format (driver version in ok result, actionable commands in warn results)"
    expected: "[ OK ] nvidia-driver: driver version <version>  OR  [WARN] nvidia-driver: nvidia-smi not found — ... on non-GPU Linux. macOS shows no NVIDIA checks at all."
    why_human: "Doctor command requires real binary lookups (nvidia-smi, nvidia-ctk, docker info) — output format is verified by code but runtime behavior needs human testing on actual hardware"
  - test: "Visit kinder.patrykgolabek.dev (or local site build) and navigate to the NVIDIA GPU addon page. Confirm it appears in site navigation and all content renders correctly."
    expected: "Page exists at /addons/nvidia-gpu/, shows Prerequisites, Configuration, Usage, Troubleshooting sections. Volume-mounts fix appears first in 0-GPUs troubleshooting."
    why_human: "Site builds cleanly (verified by npm run build), but rendering quality and navigation placement need visual confirmation"
---

# Phase 37: NVIDIA GPU Addon Verification Report

**Phase Goal:** Users on Linux with NVIDIA GPUs can create a kind cluster that exposes GPU resources to pods via a single config field, with actionable pre-flight error messages if prerequisites are missing
**Verified:** 2026-03-04T19:35:00Z
**Status:** human_needed (all automated checks passed; 5 items require runtime/hardware verification)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | NvidiaGPU field exists in v1alpha4 Addons as *bool with yaml tag nvidiaGPU | VERIFIED | `pkg/apis/config/v1alpha4/types.go` line 117-118: `NvidiaGPU *bool \`yaml:"nvidiaGPU,omitempty"\`` |
| 2 | NvidiaGPU nil defaults to false in conversion (opt-in, not opt-out) | VERIFIED | `pkg/internal/apis/config/convert_v1alpha4.go` lines 50-55: `boolValOptIn` helper (nil=false); `NvidiaGPU: boolValOptIn(in.Addons.NvidiaGPU)` at line 65 |
| 3 | NVIDIA GPU addon applies RuntimeClass then DaemonSet manifest via kubectl on Linux | VERIFIED | `nvidiagpu.go` lines 84-98: RuntimeClass applied first (line 85), DaemonSet second (line 93); unit tests confirm 2-call order |
| 4 | NVIDIA GPU addon returns nil with info log on non-Linux platforms | VERIFIED | `nvidiagpu.go` lines 57-60: `if currentOS != "linux" { ctx.Logger.V(0).Infof("...skipping on %s (Linux only)..."); return nil }`. TestExecute_NonLinuxSkips confirms 0 node calls |
| 5 | NVIDIA GPU addon fails fast with actionable error if nvidia-ctk not found | VERIFIED | `nvidiagpu.go` lines 112-118: LookPath("nvidia-ctk") + error with apt-get/configure/restart commands. TestExecute_PreflightFailure confirms 0 node calls on failure |
| 6 | NVIDIA GPU addon fails fast with actionable error if nvidia runtime not configured in Docker | VERIFIED | `nvidiagpu.go` lines 120-126: `nvidiaRuntimeInDocker()` check with actionable error message for `nvidia-ctk runtime configure --runtime=docker` |
| 7 | NVIDIA GPU addon is wired into create.go wave1 list | VERIFIED | `create.go` line 43: `installnvidiagpu` imported; line 256: `{"NVIDIA GPU", opts.Config.Addons.NvidiaGPU, installnvidiagpu.NewAction()}` in wave1 slice |
| 8 | checkPrerequisites and currentOS are package-level vars for test injection | VERIFIED | `nvidiagpu.go` lines 40-44: `var currentOS = runtime.GOOS` and `var checkPrerequisites = checkHostPrerequisites`. Used in `init()` of test file. |
| 9 | kinder doctor reports NVIDIA driver version, toolkit, and Docker runtime on Linux (warn-not-fail for missing) | VERIFIED | `doctor.go` lines 132-136: Linux-gated block; functions `checkNvidiaDriver()`, `checkNvidiaContainerToolkit()`, `checkNvidiaDockerRuntime()` all use "warn" status (lines 237, 263, 281) |
| 10 | kinder doctor NVIDIA checks are absent on macOS/Windows | VERIFIED | `doctor.go` line 132: `if runtime.GOOS == "linux"` gate — NVIDIA block only runs on Linux |
| 11 | Unit tests verify apply order, pre-flight guard, platform skip, and error propagation | VERIFIED | 5 tests in `nvidiagpu_test.go`: TestExecute (3 subtests: all-succeed/RuntimeClass-fail/DaemonSet-fail), TestExecute_NonLinuxSkips, TestExecute_PreflightFailure — all pass (`go test` output: PASS) |
| 12 | GPU documentation page covers prerequisites, config, usage, and troubleshooting with 0-GPUs fix | VERIFIED | `kinder-site/src/content/docs/addons/nvidia-gpu.md`: Prerequisites section (lines 12-41) includes volume-mounts command; Config section (lines 114-121); Usage with GPU test pod (lines 68-101); Troubleshooting (lines 138-189) with 3 Symptom/Cause/Fix scenarios |

**Score:** 12/12 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/apis/config/v1alpha4/types.go` | NvidiaGPU *bool field in Addons struct | VERIFIED | Line 117-118: field present with correct yaml tag `nvidiaGPU` |
| `pkg/internal/apis/config/types.go` | NvidiaGPU bool field in internal Addons struct | VERIFIED | Line 86: `NvidiaGPU bool` present |
| `pkg/internal/apis/config/convert_v1alpha4.go` | boolValOptIn helper and NvidiaGPU conversion | VERIFIED | Lines 50-55: `boolValOptIn` defined; line 65: `NvidiaGPU: boolValOptIn(in.Addons.NvidiaGPU)` |
| `pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu.go` | GPU addon action with pre-flight and platform guard | VERIFIED | 139 lines — full implementation: platform guard, pre-flight (3 checks), manifest application, testability vars |
| `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-device-plugin.yaml` | Embedded NVIDIA device plugin DaemonSet v0.17.1 | VERIFIED | Contains `nvidia-device-plugin-daemonset` name, image `nvcr.io/nvidia/k8s-device-plugin:v0.17.1`, FAIL_ON_INIT_ERROR=false |
| `pkg/cluster/internal/create/actions/installnvidiagpu/manifests/nvidia-runtimeclass.yaml` | Embedded RuntimeClass nvidia | VERIFIED | 5 lines: `kind: RuntimeClass`, `name: nvidia`, `handler: nvidia` |
| `pkg/cluster/internal/create/create.go` | NVIDIA GPU entry in wave1 addon list | VERIFIED | Line 43 import, line 256 wave1 entry `{"NVIDIA GPU", ...}` |
| `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` | NvidiaGPU *bool deepcopy block | VERIFIED | Lines 62-66: `if in.NvidiaGPU != nil { ... }` deepcopy block follows CertManager pattern |
| `pkg/cmd/kind/doctor/doctor.go` | Three NVIDIA check functions gated on Linux | VERIFIED | checkNvidiaDriver (line 234), checkNvidiaContainerToolkit (line 259), checkNvidiaDockerRuntime (line 274); gated at line 132 |
| `pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu_test.go` | Unit tests for GPU addon action | VERIFIED | 147 lines; 5 test cases using FakeNode/FakeCmd; all pass |
| `kinder-site/src/content/docs/addons/nvidia-gpu.md` | GPU addon documentation page | VERIFIED | 200 lines; site builds to 20 pages including `/addons/nvidia-gpu/index.html` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `convert_v1alpha4.go` | `v1alpha4/types.go` NvidiaGPU | `boolValOptIn(in.Addons.NvidiaGPU)` | WIRED | Line 65 in convert file; `boolValOptIn` defined lines 50-55 |
| `create.go` | `installnvidiagpu` package | import + wave1 AddonEntry | WIRED | Import line 43; wave1 entry line 256 |
| `nvidiagpu.go` | `manifests/*.yaml` | `//go:embed` directives | WIRED | Lines 33-37: both manifests embedded as string vars |
| `nvidiagpu.go` | `docker info` | `nvidiaRuntimeInDocker()` via kind exec | WIRED | Lines 132-138: `exec.Command("docker", "info", "--format", "{{range $k, $v := .Runtimes}}...")` |
| `doctor.go` | `nvidia-smi` binary | `osexec.LookPath` + `exec.Command` | WIRED | Lines 235-243: LookPath check + OutputLines for version query |
| `doctor.go` | `nvidia-ctk` binary | `osexec.LookPath` | WIRED | Line 260: `osexec.LookPath("nvidia-ctk")` |
| `doctor.go` | `docker info` | `exec.Command` + string search | WIRED | Lines 275-286: `docker info --format {{.Runtimes}}` + `strings.Contains(..., "nvidia")` |
| `nvidiagpu_test.go` | `testutil` package | import `testutil.FakeNode`, `testutil.FakeCmd` | WIRED | Lines 25-26: `testutil` imported; `FakeNode`, `FakeProvider`, `NewFakeControlPlane`, `NewTestContext` used |
| `nvidia-gpu.md` | GPU config field | yaml example with `nvidiaGPU: true` | WIRED | Line 57: `nvidiaGPU: true` in yaml config example |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| GPU-01 | 37-01, 37-03 | NVIDIA device plugin DaemonSet installed in cluster | SATISFIED | `devicePluginManifest` embedded and applied in `nvidiagpu.go`; manifest contains `nvidia-device-plugin-daemonset` name |
| GPU-02 | 37-01, 37-03 | nvidia RuntimeClass created | SATISFIED | `runtimeClassManifest` embedded and applied first before DaemonSet; manifest contains `kind: RuntimeClass, name: nvidia` |
| GPU-03 | 37-01 | NvidiaGPU field in v1alpha4 config API | SATISFIED | `NvidiaGPU *bool \`yaml:"nvidiaGPU,omitempty"\`` in v1alpha4 Addons struct |
| GPU-04 | 37-01, 37-03 | Platform guard: skip on non-Linux with informational message | SATISFIED | `if currentOS != "linux"` guard returns nil with Infof message; TestExecute_NonLinuxSkips confirms 0 node calls |
| GPU-05 | 37-02 | kinder doctor reports NVIDIA prerequisite status | SATISFIED | Three check functions in doctor.go all gated on `runtime.GOOS == "linux"` |
| GPU-06 | 37-01, 37-03 | Pre-flight check fails fast with actionable errors | SATISFIED | `checkHostPrerequisites()` checks nvidia-smi, nvidia-ctk, docker runtime; each with actionable error text. TestExecute_PreflightFailure confirms 0 node calls on failure |
| SITE-02 | 37-03 | GPU documentation page with prerequisites, config, usage, troubleshooting | SATISFIED | nvidia-gpu.md: Prerequisites (lines 12-41), Config (114-121), Usage with GPU test pod (51-101), Troubleshooting (138-189) with volume-mounts fix as primary step |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | — |

No TODO, FIXME, placeholder, or empty implementation patterns found in any phase-created files. All implementations are substantive.

**Note — one behavioral concern observed in create.go:** The `runAddonTimed` function (lines 98-110) uses warn-and-continue semantics (`actionsContext.Logger.Warnf(...)` and `return nil`). This means if `checkHostPrerequisites()` returns an error (pre-flight failure), the error is logged as a warning but cluster creation proceeds. This differs from the plan's stated goal of "fails fast." The pre-flight check will correctly return an error from `Execute()`, but `runAddonTimed` converts it to a warning. The cluster creation will complete but GPU addon will show "FAILED" in the summary. This is the design for all addons (warn-and-continue), not a bug introduced by this phase. Success criterion 4 ("fails fast") is partially met: the error occurs before 10 minutes (pre-flight runs before any kubectl applies), but the process does not hard-stop cluster creation. This is an architectural concern worth flagging but not a blocker since the pattern is consistent with all other addons.

### Human Verification Required

**5 items need human testing:**

#### 1. GPU Pod Scheduling on Real Hardware

**Test:** On a Linux host with a physical NVIDIA GPU and all prerequisites met (NVIDIA driver, nvidia-container-toolkit, Docker nvidia runtime, accept-nvidia-visible-devices-as-volume-mounts=true), create a cluster with `addons.nvidiaGPU: true` and apply a pod with `nvidia.com/gpu: 1` limit.

**Expected:** Pod reaches Running/Completed state. `kubectl logs gpu-test` shows Vector addition test output ending in "Test PASSED".

**Why human:** Requires physical NVIDIA GPU hardware. The code path is fully verified (platform guard bypassed by tests, manifests embedded and applied), but actual GPU resource exposure depends on the host hardware stack working end-to-end.

#### 2. macOS/Windows Non-Linux Skip Behavior (Full Cluster Flow)

**Test:** On macOS or Windows, run `kinder create cluster --config gpu-cluster.yaml` with a config file containing `addons.nvidiaGPU: true`. Let the full cluster creation complete.

**Expected:** Cluster creates successfully (exit 0). The addon log shows "* NVIDIA GPU addon: skipping on darwin (Linux only)". The addon summary shows "NVIDIA GPU: skipped (disabled)" or similar. No error is returned.

**Why human:** The code path is verified (unit test TestExecute_NonLinuxSkips confirms 0 calls and nil return). But the end-to-end cluster creation flow on a non-Linux host with this config field set needs confirmation that the addon log message surfaces correctly to the user.

#### 3. Pre-flight Failure Behavior (Missing nvidia-ctk on Linux)

**Test:** On a Linux machine without nvidia-container-toolkit installed, run `kinder create cluster` with `addons.nvidiaGPU: true`.

**Expected:** Process reports an error containing "nvidia-container-toolkit not found" with install commands. The failure should be early (not after 10 minutes of cluster creation). The cluster creation itself may still complete (due to warn-and-continue architecture) but the addon section should clearly show failure.

**Why human:** Unit tests (TestExecute_PreflightFailure) confirm the error is returned from Execute(), but the architectural warn-and-continue wrapping in runAddonTimed means cluster creation continues. Need human confirmation of the exact user-visible behavior and whether the timing meets the "not after 10 minutes" criterion from success criterion 4.

#### 4. kinder doctor Output on Linux (With and Without NVIDIA Hardware)

**Test (no GPU):** On a Linux machine without NVIDIA tools, run `kinder doctor`. Confirm NVIDIA checks appear as [WARN] with actionable messages. Confirm exit code is 2 (warn), not 1 (fail). Confirm macOS skips these checks entirely.

**Test (with GPU):** On a Linux machine with NVIDIA driver and toolkit installed, run `kinder doctor`. Confirm `[ OK ] nvidia-driver: driver version <version>` appears.

**Expected:** Output format matches plan spec. Exit code 2 on non-GPU Linux (not 1). Exit 0 on fully configured Linux GPU host.

**Why human:** Binary lookups (osexec.LookPath, exec.OutputLines) require real binary presence. Code logic verified, runtime behavior on actual hardware needs confirmation.

#### 5. Documentation Page Visual Rendering

**Test:** Run `npm run build` in kinder-site/ (verified clean at 20 pages) and open the built site or navigate to the deployed site at kinder.patrykgolabek.dev/addons/nvidia-gpu/.

**Expected:** Page renders correctly with all sections visible. Prerequisites section shows the volume-mounts command prominently. Troubleshooting section uses Symptom/Cause/Fix format. Page appears in site navigation under Addons.

**Why human:** npm build confirmed successful (20 pages, no errors), but visual rendering and navigation structure require human review.

---

## Summary

Phase 37 has achieved its goal at the code level. All 12 observable truths are verified against the actual codebase:

- The `installnvidiagpu` package is a complete, non-stub implementation with embedded manifests, platform guard, three-check pre-flight validation, and proper testability variables.
- The `NvidiaGPU` field flows correctly through the full config pipeline: v1alpha4 type → deepcopy → conversion (opt-in semantics via `boolValOptIn`) → internal type → `create.go` wave1 wiring.
- `kinder doctor` has three Linux-gated NVIDIA checks using warn-not-fail semantics.
- Unit tests pass (5/5) covering happy path, error propagation, platform skip, and pre-flight guard.
- The documentation page is substantive with all required sections including the kind-specific volume-mounts troubleshooting fix as the primary step for the 0-GPUs scenario.
- The site builds cleanly with the new page.

One architectural note: the warn-and-continue pattern in `runAddonTimed` means pre-flight failures warn rather than hard-stop cluster creation (consistent with all other addons). This affects success criterion 4 ("fails fast") — the error is surfaced but the process continues.

All 5 human verification items require real NVIDIA hardware, a Linux host, or end-to-end runtime testing that cannot be performed programmatically on the current macOS development environment.

---

_Verified: 2026-03-04T19:35:00Z_
_Verifier: Claude (gsd-verifier)_
