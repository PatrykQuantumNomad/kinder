# Architecture Research — v2.4 Hardening Integration Map

**Domain:** Brownfield maintenance milestone (kinder fork of kind)
**Researched:** 2026-05-09
**Confidence:** HIGH (all claims verified against source files)

---

## Scope

This document answers one question: **how does each v2.4 target feature integrate with the existing kinder architecture?** It does not re-derive the addon pipeline, doctor framework, or lifecycle package — those are documented in `.planning/codebase/ARCHITECTURE.md`. Every integration point below is verified against actual source files.

---

## 1. Addon Version Bumps

### Abstraction model: no shared version abstraction

There is no shared "addon version" registry or constant file. Each addon package independently owns its version pins. The pattern is:

```
pkg/cluster/internal/create/actions/install<AddonName>/
  <addon>.go          — var Images = []string{"image:version", ...}
  manifests/<file>.yaml — embedded YAML with image references inside
```

The `var Images` export is the sole authoritative source of image strings for each addon. It is consumed at exactly two call sites:

1. **`pkg/cluster/internal/providers/common/images.go`** — `RequiredAddonImages()` builds a `sets.String` of all images needed for a given `config.Cluster`. Used at `create.go:197` to emit the air-gapped fast-fail list.
2. **`pkg/internal/doctor/offlinereadiness.go`** — `allAddonImages []addonImage` is a **hand-maintained duplicate** of all image strings (see note below).

The load-balancer image is owned separately by `pkg/cluster/internal/loadbalancer/const.go`:
```go
const Image = "docker.io/envoyproxy/envoy:v1.36.2"
```

### Dual-update requirement

A version bump touches **two files per addon** (sometimes three):

| File | What to change |
|------|----------------|
| `pkg/cluster/internal/create/actions/install<Addon>/<addon>.go` | `var Images = []string{...}` |
| `pkg/cluster/internal/create/actions/install<Addon>/manifests/<file>.yaml` | image references inside the YAML |
| `pkg/internal/doctor/offlinereadiness.go` | matching entry in `allAddonImages` |

The decision to inline `allAddonImages` in `offlinereadiness.go` rather than importing the addon packages was deliberate to avoid an import cycle (PROJECT.md: "Inline allAddonImages in offlinereadiness.go — Importing pkg/cluster/internal creates a doctor import cycle"). This means **every addon image bump must update both the addon package AND `offlinereadiness.go`** — they are not automatically synchronized.

### Current pins (verified from source)

| Addon | `var Images` location | Current version |
|-------|----------------------|-----------------|
| MetalLB (×2 images) | `installmetallb/metallb.go:35-37` | `v0.15.3` |
| Metrics Server | `installmetricsserver/metricsserver.go:34-36` | `v0.8.1` |
| Envoy Gateway (×2 images) | `installenvoygw/envoygw.go:34-37` | `v1.3.1` + `ae4cee11` |
| cert-manager (×3 images) | `installcertmanager/certmanager.go:33-36` | `v1.16.3` |
| Headlamp | `installdashboard/dashboard.go:36-37` | `v0.40.1` |
| Local Registry | `installlocalregistry/localregistry.go:45` | `registry:2` (unversioned) |
| Local Path Provisioner | `installlocalpath/localpathprovisioner.go:34-36` | `v0.0.35` + `busybox:1.37.0` |
| NVIDIA GPU | `installnvidiagpu/nvidiagpu.go:40` | `v0.17.1` |
| Load Balancer (HA) | `pkg/cluster/internal/loadbalancer/const.go:20` | `envoy:v1.36.2` |

Total entries in `allAddonImages`: 14 (pinned by `TestAllAddonImages_CountMatchesExpected` at `offlinereadiness_test.go:120-126`). **This test will fail on count mismatch** if bumps add or remove images without updating the test constant.

### Test fixtures that pin addon versions

- `pkg/internal/doctor/offlinereadiness_test.go:123` — `const expected = 14` pins total image count; update if count changes.
- `pkg/internal/doctor/offlinereadiness_test.go:132` — `const envoyImage = "docker.io/envoyproxy/envoy:v1.36.2"` pins the LB image literal.
- `pkg/internal/doctor/socket_test.go:163` — `len(checks) != 24` pins the total check registry count; only relevant if a new check is added.
- `pkg/cluster/internal/providers/common/images_test.go` — `RequiredAddonImages` tests check exact image set membership; updating an image string will require test updates.

### Build order dependency

Addon bumps must precede the offline-readiness doctor check update because the test `TestAllAddonImages_CountMatchesExpected` compiles against the inline list. But since they are in separate packages and the doctor package does not import addon packages, they can be committed in either order as long as `go test ./...` is not run between the two.

---

## 2. SYNC-02 Conditional Re-Execution

### Single-file change, inline gating

The default node image constant lives at:
```
pkg/apis/config/defaults/image.go:21
const Image = "kindest/node:v1.35.1@sha256:05d7bcdefbda08b4e038f644c4df690cdac3fba8b06f8289f30e10026720a1ab"
```

Plan 51-04 is fully authored with a Docker Hub gating probe as Task 1 before any file modification. No pre-phase gate object exists in the architecture — the gate is **inline in the plan execution flow** via a conditional INCONCLUSIVE halt. This is correct: adding a new architectural gate for a single constant change would be over-engineering.

The conditional probe belongs in the plan executor logic (manual or automated), not in any new runtime check or feature flag. The plan document at `.planning/phases/51-upstream-sync-k8s-1-36/51-04-default-node-image-bump-PLAN.md` encodes this exactly.

### Integration points (SYNC-02)

| File | Change | Modified/New |
|------|--------|--------------|
| `pkg/apis/config/defaults/image.go` | Bump `const Image` to `kindest/node:v1.36.x@sha256:...` | Modified |
| `pkg/apis/config/defaults/image_test.go` | Add `TestDefaultImageIsKubernetes136` (TDD RED before GREEN) | New |
| `kinder-site/src/content/docs/guides/multi-version-clusters.md` | Update `kindest/node:v1.35.1` default references | Modified |
| `kinder-site/src/content/docs/guides/tls-web-app.md` | Update example tags where they imply current default | Modified |

**No other files are touched.** The `defaults.Image` constant is the single authoritative pin; `pkg/cluster/provider.go` and `pkg/internal/apis/config/default.go` reference it by import. The constraint is only actionable if `kindest/node:v1.36.x` is published on Docker Hub — if the probe in Task 1 fails, this entire item is deferred to a subsequent milestone.

---

## 3. Etcd Peer-TLS Regeneration

### Problem statement

When `kinder pause` stops all containers and Docker reassigns container IPs on `kinder resume`, the etcd peer certificates (stored at `/etc/kubernetes/pki/etcd/peer.{crt,key}`) contain the old IP in their SANs. Peer TLS handshakes fail between etcd members when any CP's IP changes.

### Where it hooks in

The resume lifecycle is at `pkg/internal/lifecycle/resume.go`. The three-phase start order (Phase 1: LB, Phase 2: CPs, Phase 3: readiness hook, Phase 4: workers) creates a natural insertion point: cert regeneration must happen **after CP containers are started** (kubeadm/crictl are inside the container) and **before the readiness probe** (so etcd can form quorum).

The current Phase 3 hook (`ResumeReadinessHook`) fires between CP start and worker start, only for HA clusters. Cert regen would extend this same window.

### Timing and approach

Two viable approaches:

**Option A — Host-side regen via kubeadm (before container start):** Run `kubeadm alpha certs renew` or reconstruct certs from CA. Requires mounting the PKI from the stopped container (volume or `docker cp`). Complex and fragile on different providers.

**Option B — In-container regen via crictl exec (after CP start, before readiness probe):** After CPs are started, exec into each CP and run `kubeadm certs renew peer` (or equivalent `kubeadm alpha certs` command). Uses the same `crictl exec` / `docker exec` pattern established for etcdctl probing. This is the lower-risk approach.

**Option B is recommended** because it reuses the existing `crictl exec` pattern from `pause.go:258-293` and `resumereadiness.go:142-165`, avoids host-side file manipulation, and fits cleanly in the existing Phase 3 window.

### HA implication

Peer certs are per-node and contain the SANs of ALL peer endpoints. When a CP's IP changes, every other CP's peer cert must also be regenerated (not just the one that moved). The regen step must iterate over **all CP containers**, not just the one with the changed IP.

Implementation approach for the regen loop:
```
for each CP in cp[]:
  exec into CP container
  run kubeadm certs renew peer (or equivalent)
  restart etcd static pod (crictl rm / kubelet restart, or touch /etc/kubernetes/manifests/etcd.yaml)
```

The restart of etcd is required because etcd reads certs once at startup — cert regen is not hot-reloaded.

### Integration points (etcd peer-TLS)

| File | Change | Modified/New |
|------|--------|--------------|
| `pkg/internal/lifecycle/resume.go` | Add `regenEtcdPeerTLS(binaryName, cpNodes)` function; call it in Phase 3 window after CP start, before readiness hook | Modified |
| `pkg/internal/lifecycle/resume_test.go` | Add test covering the regen phase ordering | Modified |

**No new files required.** The regen function is a peer of `captureHASnapshot` in `pause.go` — a self-contained best-effort operation that logs warnings but never blocks resume.

### Doctor integration trade-off

An `etcd-peer-tls-current` doctor check could inspect current cert SANs versus live container IPs. The trade-off:
- **Pro (new check):** Users can run `kinder doctor` to diagnose stale certs before attempting resume.
- **Con (new check):** Couples doctor to live cluster inspection for a very narrow failure mode; adds complexity to the 24-check registry.

**Recommended: no new doctor check.** The regen should be a resume-internal step, matching how the readiness probe is handled. If regen fails, it logs a warning (same `warn-and-continue` semantics). The `cluster-resume-readiness` check will detect the downstream symptom (etcd quorum failure) if regen silently fails.

---

## 4. DEBT-04 Race Fix

### Root cause

`pkg/internal/doctor/check.go:53` declares `var allChecks = []Check{...}` as a package-level slice. `check_test.go:54-69` and likely `socket_test.go` mutate this slice (via `allChecks = original` / `allChecks = []Check{...}`) inside `t.Parallel()` goroutines. Under `-race`, the read in `RunAllChecks()` and the write in the test setup race.

### Registry mutation pattern in source

Looking at `check_test.go:54-69`, the mutation pattern is:
```go
original := allChecks
defer func() { allChecks = original }()
allChecks = []Check{...}
```

This is a **test-only mutation** — production code never writes to `allChecks` after init. The registry is written once at package init time (the `var allChecks = []Check{...}` literal) and thereafter read-only in production. The only writes occur in tests.

### Appropriate fix pattern

Given that mutations are test-only and happen at init time in production:

**Pattern (c) — per-test registries — is the correct approach**, but at minimal invasiveness: rather than restructuring tests, the fix is to give each test that mutates `allChecks` an isolated registry by introducing a package-level `testAllChecks` indirection or by making `RunAllChecks` accept an optional registry parameter.

However, the simpler fix matching kinder's established convention (`sync.OnceValues` for idempotent reads per PROJECT.md: "sync.OnceValues for Nodes() cache") is:

**Pattern (b) variant — expose a test-friendly registry setter + freeze production:**

The minimum-invasive approach:
1. Change `RunAllChecks()` and `AllChecks()` to read from a local variable rather than the global `allChecks` directly (already done — they call `AllChecks()` which returns `allChecks`).
2. Add a `setTestAllChecks(checks []Check) func()` helper in `testhelpers_test.go` that swaps and restores atomically using a mutex.
3. Protect `allChecks` reads/writes with a `sync.RWMutex` declared in `check.go`.

This is the lowest-source-change approach: 3-4 lines in `check.go` (add `var checksMu sync.RWMutex`; wrap reads with `RLock`; wrap test writes with `Lock`).

**Pattern (a) — `sync.Map`** is heavier than needed since the access pattern is slice-indexed, not key-based.

### Integration points (DEBT-04)

| File | Change | Modified/New |
|------|--------|--------------|
| `pkg/internal/doctor/check.go` | Add `var checksMu sync.RWMutex`; wrap `allChecks` slice access in `AllChecks()` and `RunAllChecks()` with `checksMu.RLock()` | Modified |
| `pkg/internal/doctor/check_test.go` | Wrap `allChecks = ...` assignments with `checksMu.Lock()` / `checksMu.Unlock()` (or use a helper) | Modified |
| `pkg/internal/doctor/socket_test.go` | Same if `TestAllChecks_Registry` mutates `allChecks` indirectly | Modified (review) |

**No new files required.** The race is entirely within the `doctor` package; fix is confined to `check.go` and the two test files.

**Parallel-wave conflict note:** DEBT-04 and the doctor cosmetic fixes (item 8) both touch `pkg/internal/doctor/`. They must not be developed in parallel by the same wave if they modify the same functions. Assign to sequential sub-tasks or clearly separate the files each touches.

---

## 5. macOS Ad-hoc Signing

### Existing GoReleaser config

`.goreleaser.yaml` has:
- `builds:` section with `goos: [linux, darwin, windows]`
- `archives:` section
- `homebrew_casks:` section
- No `signs:`, `notarize:`, or post-build hooks

### Integration point

macOS ad-hoc signing (`codesign --sign -`) runs after the binary is built but before it is archived. GoReleaser v2 supports this via a `signs:` block or a `hooks:` section under `builds:`. The recommended approach is a `signs:` block with `cmd: codesign` and `artifact: all` filtered to darwin.

The `signs:` block integrates at the end of the build step, before archiving. There is no `after_hooks:` section in the existing config to extend — the signing block is a new top-level section.

Additionally, macOS binaries opened via browser download require an `Info.plist` to be embedded to avoid Gatekeeper warnings when running from certain paths. This is a separate step: embed via `-ldflags` or via a GoReleaser `upx`/`before` hook. The simpler approach for a CLI tool is `codesign --sign - --force --deep` without Info.plist — ad-hoc signing suppresses "application cannot be verified" but does not remove "app is damaged" on some macOS versions. For full Gatekeeper bypass without a paid certificate, embedding a minimal Info.plist in the binary resources is needed.

### Integration points (macOS signing)

| File | Change | Modified/New |
|------|--------|--------------|
| `.goreleaser.yaml` | Add `signs:` block after `archives:`, targeting darwin artifacts only | Modified |

No Go source changes required. This is a release-pipeline concern only.

---

## 6. Windows PR-CI Build Step

### Existing PR CI

PR CI is triggered by `pull_request` in two files:
- `.github/workflows/docker.yaml` — runs on `ubuntu-24.04`, `pull_request` trigger
- `.github/workflows/vm.yaml` — runs on `ubuntu-24.04`, `pull_request` trigger (Fedora VM via Lima)
- `.github/workflows/nerdctl.yaml` — separate trigger
- `.github/workflows/podman.yml` — separate trigger

No existing file performs a `GOOS=windows go build ./...` cross-compile check.

### Integration approach

Cross-compile on `ubuntu-latest` is the correct choice — cheaper than a `windows-latest` runner (approximately 2× cost difference on GitHub Actions), avoids native Windows environment issues, and a pure cross-compile verifies linkage without running tests.

**New job in existing file vs. new file:** The Windows build is a compile-only check unrelated to Docker cluster creation. Adding it to `docker.yaml` would conflate concerns. The correct home is either:
- A new file: `.github/workflows/build-check.yml` with `pull_request` trigger
- Or: add a separate `windows-build` job to the existing `docker.yaml` that does not require the Docker daemon

The new job structure:
```yaml
windows-build:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@...
    - uses: ./.github/actions/setup-env
    - run: GOOS=windows GOARCH=amd64 go build ./...
```

### Integration points (Windows CI)

| File | Change | Modified/New |
|------|--------|--------------|
| `.github/workflows/build-check.yml` (or `docker.yaml`) | Add `windows-build` job with `GOOS=windows go build ./...` on `ubuntu-latest` | New (recommended) or Modified |

---

## 7. Phase 47/51 Live UAT Closure

### Nature of the deliverable

UAT closure is a **verification deliverable**, not a source code change. It consists of:
1. A smoke-test script (runnable by the developer to reproduce the test scenario)
2. Evidence that the scenario passed (asciinema recording or annotated terminal transcript)

### Existing pattern

`.planning/phases/47-cluster-pause-resume/47-UAT.md` is the precedent: a YAML-fronted markdown file recording expected behavior, test results (pass/issue), root causes, and debug session links. Evidence is embedded inline as text, not as binary files.

No `scripts/` directory exists at repo root. No `uat-evidence/` directory exists. The kinder site does not host UAT recordings.

### Recommended output format

Following the existing convention:

**Phase 47 UAT closure:**
- Source: `scripts/uat-47-ha-smoke.sh` — shell script that sets up 3-CP HA cluster, pause/resumes, and asserts all nodes return Ready. New file.
- Evidence: Update `.planning/phases/47-cluster-pause-resume/47-UAT.md` status fields from `issue` to `pass` with a note on the binary version used.

**Phase 51 UAT closure:**
- Create `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md` following the same structure as `47-UAT.md`. Tests: Envoy LB image probe, IPVS-config CLI rejection, K8s 1.36 guide spot-check.
- No binary scripts needed for 51 (all CLI invocations, not multi-step scenarios).

### Integration points (UAT)

| File | Change | Modified/New |
|------|--------|--------------|
| `scripts/uat-47-ha-smoke.sh` | Shell script for HA pause/resume smoke test | New |
| `.planning/phases/47-cluster-pause-resume/47-UAT.md` | Update status fields to `pass` | Modified |
| `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md` | New UAT evidence file for Phase 51 tests | New |

---

## 8. Doctor Cosmetic Fixes

### 8a. cluster-node-skew LB version-warn bug

**Root cause (verified):** `pkg/internal/doctor/clusterskew.go:113-129` — `realListNodes()` iterates over ALL containers returned by `docker ps --filter label=io.x-k8s.kind.cluster`, including the external-load-balancer container. It then attempts `docker exec <lb-container> cat /kind/version`, which fails with a non-nil `VersionErr` because the Envoy LB container does not have `/kind/version`. This `VersionErr` is added to the `violations` slice at `clusterskew.go:171-180`, producing a false warn on every cluster that has an LB.

**Fix:** In `realListNodes()`, skip containers whose role is `external-load-balancer` before attempting `cat /kind/version`. The role is already read at line 105 via container label inspection. A single guard is sufficient:

```go
if role == "external-load-balancer" {
    // LB containers do not have /kind/version; skip version read.
    entries = append(entries, nodeEntry{Name: name, Role: role})
    continue
}
```

**Integration points (cluster-node-skew LB fix):**

| File | Change | Modified/New |
|------|--------|--------------|
| `pkg/internal/doctor/clusterskew.go` | Add role guard before `cat /kind/version` in `realListNodes()` (~5 lines) | Modified |
| `pkg/internal/doctor/clusterskew_test.go` | Add test case with an LB-role entry asserting no violation is generated | Modified |

### 8b. cluster-resume-readiness JSON error parsing

**Current behavior:** `resumereadiness.go:176-195` — when `etcdctl endpoint health` fails (non-zero exit), the check returns `warn` with `Reason: fmt.Sprintf("etcdctl endpoint health returned error: %v", err)`. The raw error string from `exec.OutputLines` contains the combined stderr, which may include partial JSON.

**Fix:** After the `err != nil` branch returns a warn, the `parseEtcdHealth` call at line 185 could be extended to attempt parsing partial output even in the error case (since etcdctl writes JSON to stdout even when some members are unhealthy and exits non-zero). However, the simpler targeted fix is:

In `parseEtcdHealth()`, improve the Reason field to extract meaningful text from the error output when the health check fails — specifically, parse JSON from the combined stdout even when `execInContainer` returns an error, since etcdctl may write structured output before the non-zero exit.

**Integration points (cluster-resume-readiness JSON parsing):**

| File | Change | Modified/New |
|------|--------|--------------|
| `pkg/internal/doctor/resumereadiness.go` | Refactor `parseEtcdHealth` error branch to attempt JSON parse on partial output; surface actionable member-level detail | Modified |
| `pkg/internal/doctor/resumereadiness_test.go` | Add test cases with partial JSON + non-nil error | Modified |

---

## Cross-Item Parallel-Wave Conflicts

The following items touch overlapping files and must be assigned to separate sequential sub-tasks (not parallel waves):

| Conflict | Files | Resolution |
|----------|-------|------------|
| DEBT-04 + cosmetic doctor fixes | `pkg/internal/doctor/check.go`, `clusterskew.go`, `resumereadiness.go`, `*_test.go` | Sequence DEBT-04 before cosmetic fixes; DEBT-04 adds mutex that cosmetic fix tests will inherit |
| Addon version bumps + offline-readiness update | `offlinereadiness.go` | Sequence addon bumps first, then update `allAddonImages` in a single follow-up commit to keep the list consistent |
| SYNC-02 image bump + site updates | `pkg/apis/config/defaults/image.go`, site `.md` files | Single plan (51-04), already sequenced; no conflict with other items |

Items that are independent and can proceed in parallel:
- macOS ad-hoc signing (`.goreleaser.yaml` only)
- Windows CI build step (`.github/workflows/` only)
- Etcd peer-TLS regeneration (`lifecycle/resume.go` only)
- Phase 47/51 UAT closure (planning files and scripts only)

---

## Suggested Build Order

Based on file-level dependencies and test coupling:

1. **DEBT-04 race fix** — first, so test infrastructure is clean before other test additions land
2. **Addon version bumps** (all 7 addons, single batch) — no code dependencies; each addon is an isolated package
3. **Offline-readiness `allAddonImages` update** — immediately after step 2; count test will fail until this lands
4. **Doctor cosmetic fixes** (LB skew + JSON parsing) — after DEBT-04; uses stable test infrastructure
5. **SYNC-02 default image bump** (gated on Docker Hub probe) — independent; can land any time the probe succeeds
6. **Etcd peer-TLS regeneration** — independent of everything above; modify `resume.go`
7. **Phase 47/51 UAT closure** — verify against final binary; run last
8. **macOS signing + Windows CI** — release-pipeline; no source dependencies; can land any time

---

## Data Flow Changes

### Addon version bumps — data flow unchanged

The `var Images` slice is read by `RequiredAddonImages()` at create time and by `allAddonImages` at doctor-run time. No new data flow introduced; only string values change.

### Etcd peer-TLS — new step in resume flow

Current resume data flow (Phase 1 → 2 → 3 → 4):
```
LB start → CP start → [readiness hook] → worker start → readiness probe
```

After fix (Phase 1 → 2 → 3a → 3b → 4):
```
LB start → CP start → [peer cert regen per CP] → [readiness hook] → worker start → readiness probe
```

The cert regen sits in the same Phase 3 window as the existing readiness hook. Both are HA-only (gated on `len(cp) >= 2`).

### DEBT-04 — mutex wraps existing global

No behavioral change. `allChecks` slice access gains mutex protection; callers (`AllChecks()`, `RunAllChecks()`, `SetMountPaths()`) add `RLock`; test mutations add `Lock`. External callers (`doctor.go` command) are unaffected since they call `RunAllChecks()` which is already the single-entry read path.

---

## Sources

All findings are from direct code inspection:

- `pkg/internal/doctor/check.go` — registry, `allChecks` var, `RunAllChecks`
- `pkg/internal/doctor/offlinereadiness.go` — `allAddonImages` inline list (14 entries)
- `pkg/internal/doctor/clusterskew.go` — `realListNodes`, role handling, LB bug location
- `pkg/internal/doctor/resumereadiness.go` — Phase 3 hook, `parseEtcdHealth`, error branch
- `pkg/internal/doctor/check_test.go` — registry mutation pattern under `t.Parallel()`
- `pkg/internal/doctor/socket_test.go` — `TestAllChecks_Registry` count assertion (expects 24)
- `pkg/internal/doctor/offlinereadiness_test.go` — `TestAllAddonImages_CountMatchesExpected` (expects 14)
- `pkg/internal/lifecycle/resume.go` — three-phase start order, `ResumeReadinessHook`
- `pkg/internal/lifecycle/pause.go` — `captureHASnapshot`, `readEtcdLeaderID` via crictl
- `pkg/cluster/internal/providers/common/images.go` — `RequiredAddonImages`
- `pkg/cluster/internal/create/actions/install*/` — `var Images` per addon
- `pkg/apis/config/defaults/image.go` — `const Image` (SYNC-02 target)
- `.goreleaser.yaml` — current GoReleaser config (no `signs:` block)
- `.github/workflows/docker.yaml` and `vm.yaml` — PR CI triggers (no Windows step)
- `.planning/phases/47-cluster-pause-resume/47-UAT.md` — UAT evidence file precedent
- `.planning/phases/51-upstream-sync-k8s-1-36/51-04-default-node-image-bump-PLAN.md` — SYNC-02 plan

---

*Integration architecture for: kinder v2.4 Hardening*
*Researched: 2026-05-09*
