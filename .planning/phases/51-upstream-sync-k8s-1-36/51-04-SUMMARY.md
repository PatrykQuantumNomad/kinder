---
phase: 51-upstream-sync-k8s-1-36
plan: 04
subsystem: infra
tags: [kubernetes, kindest-node, docker-hub, default-image, k8s-1-36]

# Dependency graph
requires:
  - phase: 51-upstream-sync-k8s-1-36/51-01
    provides: Envoy LB migration (must land before SC2 default bump)
provides:
  - "INCONCLUSIVE: SC2 default node image bump gated on kindest/node:v1.36.x availability"
affects: [51-phase-verification, sc2-default-node-image]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified: []

key-decisions:
  - "Outcome B: Docker Hub probe returned HTTP 200 with count=0 — no kindest/node:v1.36.x images published as of 2026-05-07. SC2 deferred per plan INCONCLUSIVE protocol."

patterns-established: []

# Metrics
duration: ~5min
completed: 2026-05-07
---

# Phase 51 Plan 04: Default Node Image Bump Summary

**INCONCLUSIVE — kindest/node:v1.36.x not yet published on Docker Hub; SC2 deferred pending kind v0.32.0 release**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-05-07T12:43:00Z
- **Completed:** 2026-05-07T12:48:00Z
- **Tasks:** 1 of 2 (Task 1 complete — Outcome B; Task 2 skipped per gating protocol)
- **Files modified:** 0 (source files intentionally untouched)

## INCONCLUSIVE

Plan 51-04 cannot land: no `kindest/node:v1.36.x` image is published on Docker Hub
as of 2026-05-07. Latest available is `kindest/node:v1.35.1` (kind v0.31.0
default, published 2026-02-14). Re-run this plan after kind ships v0.32.0 (or
whatever release publishes the v1.36 image), or after a manual `kind build
node-image` is uploaded.

SC2 (default node image is K8s 1.36.x) remains **DEFERRED**.

### Probe output

```
GET https://hub.docker.com/v2/repositories/kindest/node/tags/?page_size=100&name=v1.36
HTTP/2 200
{"count":0,"next":null,"previous":null,"results":[]}
```

The API returned HTTP 200 with an empty results list — this is a clean Outcome B
(no images, no API error). Rate-limit headers confirmed: `x-ratelimit-remaining: 179`
(not throttled). Latest kindest/node tags are:

```
v1.35.0   2025-12-18T08:18:46Z
v1.35.1   2026-02-14T00:40:07Z   ← current default in kinder
```

No retry needed — the 200 response is authoritative.

## Accomplishments

- Task 1 (pre-flight gating probe) executed cleanly: Docker Hub API queried, 200 OK
  with zero results for `v1.36.x` tags — Outcome B confirmed
- No source code modified (no broken default committed)
- Pre-existing TDD-enhancement edits to the plan file folded into this docs commit
- Pre-existing research file edits folded into this docs commit

## Task Commits

1. **Task 1: Gating pre-flight — Outcome B** — no source commit (per plan protocol)
2. **Task 2: Skipped** — gated by Task 1 Outcome A; not reached

**Plan metadata / docs commit:** `5630c1ff` (docs: inconclusive — kindest/node:v1.36.x not yet on Docker Hub)

## Files Created/Modified

- `.planning/phases/51-upstream-sync-k8s-1-36/51-04-SUMMARY.md` — this file (INCONCLUSIVE marker)
- `.planning/phases/51-upstream-sync-k8s-1-36/51-04-default-node-image-bump-PLAN.md` — pre-existing TDD-enhancement edits folded in
- `.planning/phases/51-upstream-sync-k8s-1-36/51-RESEARCH.md` — pre-existing research update edits folded in

## Decisions Made

- **Outcome B confirmed:** Docker Hub probe returned `count: 0` for `?name=v1.36`. This
  is the authoritative signal that no v1.36.x image exists. No retry needed (non-error 200).
- **Zero source changes:** Per plan protocol for Outcome B, `pkg/apis/config/defaults/image.go`
  remains at `kindest/node:v1.35.1@sha256:...`. Committing a broken default (pointing to a
  non-existent image) would be worse than the current stale default.
- **SC2 treatment:** Phase 51 verifier should flag SC2 as DEFERRED with this SUMMARY as evidence.
  The plan is fully re-runnable once kind publishes a v1.36 image — Task 1's probe will
  return Outcome A and Task 2 (TDD RED→GREEN) will execute normally.

## Floor-Relaxation Note

Not applicable — no v1.36.x image of any patch level exists. The ≥1.36.4 floor from the
original ROADMAP is moot. Per RESEARCH §SYNC-02, that floor was a misattributed forward
projection from kind issue #4131 (which was actually a v1.35 StatefulSet regression, not a
v1.36 regression). Any v1.36.x image is acceptable when it becomes available.

## Deviations from Plan

None — plan executed exactly as specified for Outcome B. Task 1 ran the probe, found no
results, and halted cleanly. The pre-existing uncommitted edits to the plan file and
research file are folded into this docs commit per the execution context instructions.

## Re-run Instructions

When `kindest/node:v1.36.x` becomes available (expected with kind v0.32.0):

1. Re-run this plan: `gsd-execute-phase 51 04`
2. Task 1 will return Outcome A
3. Task 2 will execute TDD RED→GREEN:
   - RED commit: `pkg/apis/config/defaults/image_test.go` with `TestDefaultImageIsKubernetes136`
   - GREEN commit: update `pkg/apis/config/defaults/image.go` + website guide references
4. Check `go build ./... && go test ./pkg/apis/config/... -race`

The plan file already contains the full TDD implementation instructions for Task 2, ready
to execute once the image is available.

## Next Phase Readiness

- Phase 51 Wave 2 is complete (with SC2 deferred)
- Phase 51 overall: SC1 (Envoy LB), SC3 (IPVS guard), SC4 (K8s 1.36 recipe) all delivered
- SC2 (default node image = 1.36.x) is DEFERRED pending `kindest/node:v1.36.x` publication
- No other work blocked by this deferral

---
*Phase: 51-upstream-sync-k8s-1-36*
*Completed: 2026-05-07*
