---
phase: 46-kinder-load-images-command
verified: 2026-04-09T12:00:00Z
status: human_needed
score: 4/4
overrides_applied: 0
human_verification:
  - test: "Run kinder load images <image> against a docker cluster and observe all nodes receive the image"
    expected: "Image appears in crictl images on every node after the command completes"
    why_human: "Requires a running kind cluster with docker provider; cannot execute without live docker daemon"
  - test: "Run kinder load images <image> with KIND_EXPERIMENTAL_PROVIDER=podman and a podman cluster"
    expected: "podman save is invoked (not docker save); image loads on all nodes"
    why_human: "Requires podman installed and a running podman-backed cluster"
  - test: "Run kinder load images <image> with KIND_EXPERIMENTAL_PROVIDER=finch and a finch cluster"
    expected: "finch save is invoked; image loads on all nodes correctly"
    why_human: "Requires finch installed and a running finch-backed cluster"
  - test: "Run kinder load images <image> twice; second run should report image already present and complete without re-importing"
    expected: "Second run logs 'Image ... found to be already present on all nodes.' and returns 0"
    why_human: "Requires a running cluster; the smart-load path executes crictl on live nodes"
---

# Phase 46: kinder load images Command — Verification Report

**Phase Goal:** Users can load one or more local images into all nodes of a running cluster with a single command that works across all three providers and handles Docker Desktop 27+ containerd image store compatibility
**Verified:** 2026-04-09T12:00:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `kinder load images <image> [<image>...]` loads the specified images into every node of the target cluster using the correct provider's image save mechanism (not hardcoded to `docker save`) | VERIFIED | `save(binaryName, ...)` and `imageID(binaryName, ...)` both use the resolved `binaryName` parameter, never "docker". `providerBinaryName()` returns `provider.Name()` for docker/podman and reads `KIND_EXPERIMENTAL_PROVIDER` for nerdctl variants. Full binary builds clean. |
| 2 | The command works identically with docker, podman, and nerdctl providers, using each provider's native image export | VERIFIED | `providerBinaryName()` dispatches correctly: docker → "docker", podman → "podman", nerdctl/finch/nerdctl.lima → reads `KIND_EXPERIMENTAL_PROVIDER`. Code path is identical — only the binary name differs. |
| 3 | On Docker Desktop 27+ with the containerd image store enabled, `kinder load images` successfully imports multi-platform images without the `content digest: not found` error (fallback strategy applied automatically) | VERIFIED | `LoadImageArchiveWithFallback` in `pkg/cluster/nodeutils/util.go` (lines 135–163): attempt 1 uses `--all-platforms`; on `isContentDigestError` detection, attempt 2 retries without `--all-platforms`. `loadImage()` in `images.go` calls this with an `os.Open` factory so a fresh reader is available for attempt 2. `TestIsContentDigestError` covers all 5 detection cases (nil, RunError with/without content digest in Output, plain error with/without). |
| 4 | Re-running `kinder load images` with an image already present on all nodes completes without re-importing, reporting that the image was skipped as already present | VERIFIED | Smart-load logic at lines 150–179 of `images.go`: for each image, if `checkIfImageReTagRequired` finds the image present and correctly tagged on every candidate node, `selectedNodes` map stays empty and the function returns early. The skip message (`"Image: %q with ID %q found to be already present on all nodes."`) is emitted via `logger.V(0).Infof`. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/cluster/nodeutils/util.go` | LoadImageArchiveWithFallback + runImport | VERIFIED | All three functions present: `runImport` (line 97), `isContentDigestError` (line 113), `LoadImageArchiveWithFallback` (line 135). Existing `LoadImageArchive` unchanged. |
| `pkg/cluster/nodeutils/util_test.go` | Unit tests for fallback/error detection | VERIFIED* | `TestIsContentDigestError` present with 5 table-driven cases, all passing. `TestLoadImageArchiveWithFallback` intentionally not a test function — plan task explicitly documents this is not unit-testable without live cluster node infrastructure; comment note added instead. The PLAN frontmatter `contains: "TestLoadImageArchiveWithFallback"` is a plan authoring inconsistency, not an implementation gap. |
| `pkg/cmd/kind/load/images/images.go` | Full load images subcommand | VERIFIED | `NewCommand` with correct `Use: "images <IMAGE> [IMAGE...]"`, flags, provider abstraction, smart-load, fallback wiring. |
| `pkg/cmd/kind/load/load.go` | Registration of images subcommand | VERIFIED | `images.NewCommand(logger, streams)` registered as third subcommand; `kinder load --help` shows `images` alongside `docker-image` and `image-archive`. |

*Note on `TestLoadImageArchiveWithFallback`: The plan task body explicitly states this function cannot be unit-tested because `getSnapshotter` requires a live cluster node. The PLAN frontmatter `contains:` value conflicts with its own task text. The implementation correctly follows the task description.

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `pkg/cluster/nodeutils/util.go` | `pkg/exec/types.go` | `exec.RunError` for output text matching | VERIFIED | Line 118: `var runErr *exec.RunError` — gsd-tools regex issue with escaped dot; confirmed manually |
| `pkg/cluster/nodeutils/util.go` | `ctr images import` | `runImport` builds ctr args with/without `--all-platforms` | VERIFIED | Lines 97–108 of util.go confirmed |
| `pkg/cmd/kind/load/images/images.go` | `pkg/cluster/nodeutils/util.go` | `nodeutils.LoadImageArchiveWithFallback` | VERIFIED | Line 214 of images.go confirmed |
| `pkg/cmd/kind/load/images/images.go` | `pkg/cluster/provider.go` | `provider.ListInternalNodes` | VERIFIED | Line 121 of images.go confirmed |
| `pkg/cmd/kind/load/load.go` | `pkg/cmd/kind/load/images/images.go` | `images.NewCommand` registration | VERIFIED | Line 50 of load.go confirmed |

### Data-Flow Trace (Level 4)

Not applicable — all artifacts are CLI command handlers and utility functions, not components rendering dynamic data. The data flow is: host image → tar file → node ctr import. No state/rendering pipeline.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full binary builds without errors | `go build -o /dev/null ./` | No output (success) | PASS |
| `kinder load --help` shows `images` subcommand | `/tmp/kinder-test load --help` | "images: Loads images from host into nodes" | PASS |
| `kinder load images --help` shows correct usage | `/tmp/kinder-test load images --help` | `kinder load images <IMAGE> [IMAGE...]` | PASS |
| All nodeutils tests pass | `go test ./pkg/cluster/nodeutils/... -v` | TestParseSnapshotter PASS, TestIsContentDigestError PASS (5/5) | PASS |
| go vet clean on all modified packages | `go vet ./pkg/cluster/nodeutils/... ./pkg/cmd/kind/load/...` | No output (success) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| LOAD-01 | 46-02-PLAN.md | `kinder load images` subcommand loads one or more images into all cluster nodes | SATISFIED | `images.go` registered, iterates all candidate nodes, concurrent load via `UntilErrorConcurrent` |
| LOAD-02 | 46-02-PLAN.md | Provider-abstracted image saving (docker/podman/nerdctl, not hardcoded to docker save) | SATISFIED | `providerBinaryName()` + `save(binaryName, ...)` + `imageID(binaryName, ...)` |
| LOAD-03 | 46-01-PLAN.md | Fallback import strategy for Docker Desktop 27+ containerd image store compatibility | SATISFIED | `LoadImageArchiveWithFallback` in util.go with two-attempt factory-pattern fallback. Note: REQUIREMENTS.md checkbox `[ ]` and tracking table show "Pending" — this is a documentation tracking inconsistency; the code fully implements LOAD-03 |
| LOAD-04 | 46-02-PLAN.md | Smart-load skips images already present on nodes | SATISFIED | `checkIfImageReTagRequired` + `selectedNodes` map + early return when all nodes have image |

**LOAD-03 tracking note:** The REQUIREMENTS.md file shows `- [ ] **LOAD-03**` (unchecked) and `| LOAD-03 | Phase 46 | Pending |` in the tracker. The implementation is complete in code. No phase 46 commit updated REQUIREMENTS.md. This is a documentation gap — the checkbox and status table need updating.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `pkg/cluster/nodeutils/util.go` | 68 | `// TODO: experiment with streaming instead to avoid the copy` | Info | Pre-existing TODO in `CopyNodeToNode` function — unrelated to phase 46 work |

No blockers or warnings found in phase 46 code paths.

### Human Verification Required

#### 1. Docker provider end-to-end load

**Test:** On a machine with docker installed, create a kind cluster (`kinder create cluster`) and run `kinder load images nginx:latest`. Then exec into a node and run `crictl images | grep nginx`.
**Expected:** nginx:latest appears in crictl output on all cluster nodes.
**Why human:** Requires a running Docker daemon and cluster; cannot mock node-side ctr import.

#### 2. Podman provider end-to-end load

**Test:** Set `KIND_EXPERIMENTAL_PROVIDER=podman`, create a podman-backed kinder cluster, and run `kinder load images nginx:latest`. Observe that `podman save` (not `docker save`) is invoked.
**Expected:** podman save called; image loads on all nodes without error.
**Why human:** Requires podman installed; provider dispatch is correct in code but runtime behavior needs confirmation.

#### 3. Nerdctl/finch provider image export

**Test:** Set `KIND_EXPERIMENTAL_PROVIDER=finch`, run `kinder load images nginx:latest` on a finch-backed cluster.
**Expected:** finch save is called; image loads correctly on all nodes.
**Why human:** Requires finch runtime; binary name dispatch is code-verified but real invocation needs confirming.

#### 4. Docker Desktop 27+ containerd image store fallback

**Test:** On Docker Desktop 27+ with the containerd image store enabled in Docker Desktop settings, build or pull a multi-platform image, then run `kinder load images <image>`. Check that it completes without error.
**Expected:** First `ctr import --all-platforms` fails with "content digest: not found"; second attempt without `--all-platforms` succeeds. No error returned to user.
**Why human:** Requires Docker Desktop 27+ with specific setting; the fallback trigger is environment-dependent.

#### 5. Smart-load re-run skip behavior

**Test:** Run `kinder load images nginx:latest` twice in a row on the same cluster. Observe second run output.
**Expected:** Second run logs "Image: ... found to be already present on all nodes." and exits 0 without re-importing.
**Why human:** Requires live cluster; the skip logic calls crictl on real nodes to compare image IDs.

### Gaps Summary

No code gaps. All four success criteria are implemented and verified at the code level. The implementation exactly matches the plan specifications.

**REQUIREMENTS.md tracking inconsistency:** LOAD-03 checkbox and progress table are not updated. This is a documentation-only issue — REQUIREMENTS.md was not updated as part of phase 46 commits. Recommend updating `- [ ] **LOAD-03**` to `- [x] **LOAD-03**` and `| LOAD-03 | Phase 46 | Pending |` to `| LOAD-03 | Phase 46 | Complete |`.

---

_Verified: 2026-04-09T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
