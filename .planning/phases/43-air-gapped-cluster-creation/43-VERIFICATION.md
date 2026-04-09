---
phase: 43-air-gapped-cluster-creation
verified: 2026-04-09T13:00:00Z
status: human_needed
score: 5/5 must-haves verified
overrides_applied: 0
human_verification:
  - test: "Run `kinder create cluster --air-gapped` on a machine where all required images are pre-loaded with docker"
    expected: "Cluster creates successfully with no image pull attempts; the status spinner shows 'Checking local images (air-gapped mode)' and completes with a checkmark"
    why_human: "Requires a running Docker daemon with pre-loaded images; cannot simulate runtime exec in a static grep check"
  - test: "Run `kinder create cluster --air-gapped` on a machine where at least one required image is absent"
    expected: "kinder exits immediately with an error message beginning 'air-gapped mode: the following required images are not present locally:' listing every missing image with docker pre-load instructions"
    why_human: "Requires a running Docker daemon to test actual exec.Command behavior end-to-end"
  - test: "Run `kinder create cluster` (without --air-gapped) with at least one addon enabled in config"
    expected: "Output includes a NOTE block listing each addon image before Provision begins, with guidance to use --air-gapped to skip pulls"
    why_human: "Requires cluster creation to reach the warning block; depends on a live provider"
  - test: "Run `kinder doctor` on a machine missing some addon images"
    expected: "Output shows an 'Offline' category with a warn result displaying a tab-aligned table of MISSING IMAGE / REQUIRED BY columns"
    why_human: "Requires a container runtime to be present; the check's runtime detection (docker/podman/nerdctl) must be tested live"
---

# Phase 43: Air-Gapped Cluster Creation Verification Report

**Phase Goal:** Users can create a fully functional cluster without internet access by passing `--air-gapped`, and kinder fails immediately with a complete list of missing images rather than hanging on failed pulls
**Verified:** 2026-04-09T13:00:00Z
**Status:** human_needed
**Re-verification:** No - initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `kinder create cluster --air-gapped` on a machine with pre-loaded images creates the cluster without any network calls for image pulls across all three providers (docker, podman, nerdctl) | VERIFIED | `ensureNodeImages` in docker/podman/nerdctl all branch on `cfg.AirGapped` at the top, calling `checkAllImagesPresent` which uses `inspect --type=image` (no pull) and returns nil on success. Automated tests confirm nil error when all images present (`TestCheckAllImagesPresent_AllPresent`). |
| 2 | `kinder create cluster --air-gapped` on a machine with missing images exits immediately with a human-readable list of every image that must be pre-loaded | VERIFIED | `checkAllImagesPresent` accumulates ALL missing images into a slice before returning `formatMissingImagesError`. Error begins with "air-gapped mode: the following required images are not present locally:" and includes runtime-specific pull/save/load instructions. `TestCheckAllImagesPresent_AllMissing` and `TestFormatMissingImagesError` confirm the accumulate-all pattern. |
| 3 | Running `kinder create cluster` (without `--air-gapped`) prints a warning listing all addon images that will be pulled | VERIFIED | `create.go` lines 184-196: `if !opts.Config.AirGapped { addonImages := common.RequiredAddonImages(opts.Config); if addonImages.Len() > 0 { ... NOTE block logged ... } }`. Wired to real `RequiredAddonImages` which returns non-empty set for any enabled addon. |
| 4 | `kinder doctor` run before cluster creation lists which required images are absent from the local image store | VERIFIED | `offlinereadiness.go` implements `offlineReadinessCheck` registered in `allChecks` at `pkg/internal/doctor/check.go:83`. `Run()` returns warn result with tabwriter table of MISSING IMAGE / REQUIRED BY columns. `TestOfflineReadiness_SomeAbsent` verifies MetalLB label appears in warn output. |
| 5 | The two-mode offline workflow (pre-create image baking vs. post-create load via `kinder load docker-image`) is documented and reachable from the website | VERIFIED | `site/content/docs/user/working-offline.md` contains "Addon images" section with two-mode workflow: Mode 1 (pre-create baking: kinder doctor + docker pull/save/load + kinder create cluster --air-gapped) and Mode 2 (post-create: kinder load docker-image). Link references preserved. Note: SC5 wording in ROADMAP says "privileged container commit" but the doc describes the equivalent docker pull/save/load workflow — same intent, accurate implementation. |

**Score:** 5/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/internal/apis/config/types.go` | AirGapped bool field on Cluster struct | VERIFIED | Line 76-77: `AirGapped bool` with comment |
| `pkg/apis/config/v1alpha4/types.go` | AirGapped bool with yaml/json tags | VERIFIED | Line 91-92: `AirGapped bool \`yaml:"airGapped,omitempty" json:"airGapped,omitempty"\`` |
| `pkg/internal/apis/config/convert_v1alpha4.go` | AirGapped propagated in conversion | VERIFIED | Line 68: `out.AirGapped = in.AirGapped` |
| `pkg/cluster/createoption.go` | CreateWithAirGapped option function | VERIFIED | Lines 130-135: function wires `o.AirGapped = airGapped` |
| `pkg/cmd/kind/create/cluster/createcluster.go` | --air-gapped CLI flag wired | VERIFIED | Lines 103-104: `BoolVar` registers flag; line 128: `cluster.CreateWithAirGapped(flags.AirGapped)` |
| `pkg/cluster/internal/create/create.go` | ClusterOptions.AirGapped + fixupOptions propagation + addon warning | VERIFIED | Line 81: struct field; lines 409-411: fixupOptions sets `opts.Config.AirGapped = true`; lines 184-196: addon image NOTE block |
| `pkg/cluster/internal/providers/common/images.go` | RequiredAddonImages and RequiredAllImages | VERIFIED | Lines 43-82: both functions implemented, wired to real addon Images vars |
| `pkg/cluster/internal/providers/common/images_test.go` | 6 test functions | VERIFIED | TestRequiredNodeImages, TestRequiredAddonImages_AllDisabled, TestRequiredAddonImages_LBOnlyWithMultiCP, TestRequiredAddonImages_MetalLBOnly, TestRequiredAddonImages_AllEnabled, TestRequiredAllImages |
| `pkg/cluster/internal/providers/docker/images.go` | Air-gapped branch in ensureNodeImages | VERIFIED | Lines 36-38: early return to `checkAllImagesPresent`; lines 52-93: check and error formatter with injectable `inspectImageFunc` |
| `pkg/cluster/internal/providers/docker/images_test.go` | Unit tests for fast-fail | VERIFIED | 4 tests: AllPresent, SomeMissing, AllMissing, FormatMissingImagesError |
| `pkg/cluster/internal/providers/podman/images.go` | Air-gapped branch in ensureNodeImages | VERIFIED | Lines 36-38: early return; lines 52-89: check + error formatter using "podman" binary |
| `pkg/cluster/internal/providers/nerdctl/images.go` | Air-gapped branch in ensureNodeImages | VERIFIED | Lines 36-37: early return with binaryName; lines 52-89: check + error formatter using binaryName |
| `pkg/cluster/internal/create/actions/installmetallb/metallb.go` | var Images exported | VERIFIED | `var Images = []string{...}` with 2 metallb image refs |
| `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go` | var Images exported | VERIFIED | `var Images = []string{...}` with metrics-server ref |
| `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go` | var Images exported | VERIFIED | `var Images = []string{...}` with 3 jetstack refs |
| `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` | var Images exported | VERIFIED | `var Images = []string{...}` with ratelimit + gateway refs |
| `pkg/cluster/internal/create/actions/installdashboard/dashboard.go` | var Images exported | VERIFIED | `var Images = []string{...}` with headlamp ref |
| `pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go` | var Images exported | VERIFIED | `var Images = []string{registryImage}` |
| `pkg/cluster/internal/create/actions/installnvidiagpu/nvidiagpu.go` | var Images exported | VERIFIED | `var Images = []string{...}` with k8s-device-plugin ref |
| `pkg/internal/doctor/offlinereadiness.go` | offline-readiness check with injectable deps | VERIFIED | 165 lines; `offlineReadinessCheck` struct with `inspectImage` and `lookPath` injection; 12-entry `allAddonImages`; tabwriter table output |
| `pkg/internal/doctor/offlinereadiness_test.go` | 5 test functions | VERIFIED | AllPresent, SomeAbsent, AllAbsent, NoRuntime, CountMatchesExpected |
| `pkg/internal/doctor/check.go` | newOfflineReadinessCheck registered in allChecks | VERIFIED | Line 83: `newOfflineReadinessCheck()` under "Category: Offline (Phase 43)" comment |
| `site/content/docs/user/working-offline.md` | Two-mode offline workflow with air-gapped and kinder doctor | VERIFIED | "Addon images" section (lines 160-242) with doctor reference, pre-load workflow, `--air-gapped` command, Mode 1/Mode 2 two-mode workflow |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `createcluster.go` | `createoption.go` | `CreateWithAirGapped(flags.AirGapped)` | WIRED | Line 128 calls CreateWithAirGapped; option sets `o.AirGapped` |
| `createoption.go` | `create.go ClusterOptions` | `o.AirGapped = airGapped` in option adapter | WIRED | ClusterOptions.AirGapped set; fixupOptions propagates to Config.AirGapped |
| `docker/images.go` | `common/images.go` | `common.RequiredAllImages(cfg)` in checkAllImagesPresent | WIRED | Line 63: `allRequired := common.RequiredAllImages(cfg)` |
| `podman/images.go` | `common/images.go` | `common.RequiredAllImages(cfg)` in checkAllImagesPresent | WIRED | Line 57: same pattern |
| `nerdctl/images.go` | `common/images.go` | `common.RequiredAllImages(cfg)` in checkAllImagesPresent | WIRED | Line 57: same pattern |
| `create.go` | `common/images.go` | `common.RequiredAddonImages(opts.Config)` in warning block | WIRED | Line 186: `addonImages := common.RequiredAddonImages(opts.Config)` |
| `common/images.go` | `installmetallb` package | `installmetallb.Images...` | WIRED | Import + line 55 insert |
| `offlinereadiness.go` | `check.go` | `newOfflineReadinessCheck()` in allChecks | WIRED | check.go line 83 |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `docker/images.go checkAllImagesPresent` | `allRequired` (sets.String) | `common.RequiredAllImages(cfg)` → addon `Images` vars (populated from embedded manifest constants) | Yes — string slices from real image refs in YAML manifests | FLOWING |
| `create.go warning block` | `addonImages` (sets.String) | `common.RequiredAddonImages(opts.Config)` → same addon `Images` vars | Yes | FLOWING |
| `offlinereadiness.go Run()` | `allAddonImages` (static slice) | Inline const — 12 image refs hardcoded from manifest/const files; verified by `TestAllAddonImages_CountMatchesExpected` | Yes — real image refs, not placeholders | FLOWING |

### Behavioral Spot-Checks

Step 7b SKIPPED for live cluster creation tests — requires running Docker daemon.

The following static behavioral checks were run:

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `go build ./...` compiles without errors | `go build ./...` | Exit 0, no output | PASS |
| `go test ./pkg/cluster/internal/providers/common/...` | test run | `ok sigs.k8s.io/kind/pkg/cluster/internal/providers/common` | PASS |
| `go test ./pkg/cluster/internal/providers/docker/...` | test run | `ok sigs.k8s.io/kind/pkg/cluster/internal/providers/docker 0.971s` | PASS |
| `go test ./pkg/internal/doctor/...` | test run | `ok sigs.k8s.io/kind/pkg/internal/doctor` | PASS |
| All other packages in `go test ./...` | test run | 0 FAIL results across all packages | PASS |
| All 7 phase 43 commits present in git log | git log | 7feb7044, c3310735, 7e455484, bc0b31cc, 4b7a6a7c, f2e84747, ed996183 all confirmed | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| AIRGAP-01 | 43-01, 43-02 | Air-gapped flag plumbing from CLI to config | SATISFIED | AirGapped bool flows CLI -> CreateWithAirGapped -> ClusterOptions -> config.Cluster |
| AIRGAP-02 | 43-02 | Provider fast-fail with complete missing image list | SATISFIED | All 3 providers: `ensureNodeImages` branches on AirGapped; `checkAllImagesPresent` accumulates all missing; `formatMissingImagesError` formats complete list |
| AIRGAP-03 | 43-01, 43-02 | Addon image warning when not air-gapped | SATISFIED | `create.go` lines 184-196: NOTE block with `RequiredAddonImages` output |
| AIRGAP-04 | 43-01, 43-02 | Doctor offline-readiness check | SATISFIED | `offlineReadinessCheck` registered in `allChecks`; reports missing images |
| AIRGAP-05 | 43-03 | Doctor offline-readiness check reports missing images | SATISFIED | tabwriter table with MISSING IMAGE + REQUIRED BY columns; addon labels from `allAddonImages` struct |
| AIRGAP-06 | 43-03 | Working-offline.md two-mode documentation | SATISFIED | "Addon images" section in working-offline.md with Mode 1 (pre-create baking) and Mode 2 (post-create load) |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `create.go` | 247, 400 | TODO comments | Info | Pre-existing TODOs unrelated to phase 43 — confirmed present before phase 43 commits via `git show 2baec8c6` |

No blockers or warnings found. All phase 43 code is substantive with no stubs, placeholders, or hardcoded empty values.

### Human Verification Required

The automated checks verify code structure, test passing, and static wiring. Four behavioral scenarios require a running container runtime to fully confirm:

### 1. Air-Gapped Success Path (Docker)

**Test:** Pre-load all required images (`kindest/node`, addon images, LB image) into Docker, then run `kinder create cluster --air-gapped --image kindest/node:v1.31.0` with a config that enables at least one addon.
**Expected:** Cluster creates successfully. Status spinner shows "Checking local images (air-gapped mode)" followed by checkmark. No `docker pull` invocations observed in Docker daemon logs.
**Why human:** Requires live Docker daemon with pre-loaded images. Cannot simulate exec.Command results end-to-end without a daemon.

### 2. Air-Gapped Fail-Fast Path (Docker)

**Test:** Ensure at least one required image (e.g., a MetalLB image) is absent from Docker's local store. Run `kinder create cluster --air-gapped --config <config-with-metallb>`.
**Expected:** kinder exits immediately (within seconds) with error message: "air-gapped mode: the following required images are not present locally:" followed by a list of every absent image and docker pull/save/load pre-load instructions. No timeout, no partial cluster state.
**Why human:** Requires live Docker daemon and controlled image state. The `inspectImageFunc` injection in tests validates the logic but not the real exec path.

### 3. Non-Air-Gapped Addon Warning

**Test:** Run `kinder create cluster` (no `--air-gapped`) with a config enabling MetalLB and Cert Manager.
**Expected:** Before cluster creation begins, output includes a NOTE block listing MetalLB and Cert Manager images with guidance to pre-load and use `--air-gapped`.
**Why human:** Requires cluster creation to progress past fixupOptions to the warning block. Provider presence needed.

### 4. Doctor Offline-Readiness Check Live Run

**Test:** On a machine with Docker installed but missing 3-4 addon images, run `kinder doctor`.
**Expected:** Output shows an "Offline" category section with an "offline-readiness" check in warn status displaying a tab-aligned table with MISSING IMAGE and REQUIRED BY columns. Check should skip gracefully if Docker is replaced with a test that has no runtime.
**Why human:** Requires a live container runtime to test `realInspectImage` via the real LookPath + exec.Command path. Test injection covers the logic but not runtime integration.

### Gaps Summary

No gaps found. All 5 success criteria are verified at the code level. The 4 human verification items are behavioral/integration checks that require a live container runtime — they do not indicate missing implementation but rather confirm the end-to-end path that tests already cover at the unit level.

---

_Verified: 2026-04-09T13:00:00Z_
_Verifier: Claude (gsd-verifier)_
