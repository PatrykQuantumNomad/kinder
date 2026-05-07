---
phase: 51-upstream-sync-k8s-1-36
plan: 04
type: execute
wave: 2
depends_on: ["51-01"]
files_modified:
  - pkg/apis/config/defaults/image.go
  - kinder-site/src/content/docs/guides/multi-version-clusters.md
  - kinder-site/src/content/docs/guides/tls-web-app.md
autonomous: true

must_haves:
  truths:
    - "Running kinder create cluster without an explicit image: field provisions a Kubernetes 1.36.x node"
    - "The default image tag satisfies the >=1.36.4 floor declared in SC2"
    - "The default image is verified to exist on Docker Hub at execute time (no broken default)"
    - "Existing website examples that mention concrete kindest/node:vX.Y.Z tags are updated to the new default"
  artifacts:
    - path: "pkg/apis/config/defaults/image.go"
      provides: "Default kindest/node image constant pointing to 1.36.x"
      contains: "kindest/node:v1.36"
  key_links:
    - from: "pkg/apis/config/defaults/image.go Image constant"
      to: "Docker Hub kindest/node:v1.36.x@sha256:<digest>"
      via: "string literal with embedded sha256 digest"
      pattern: "kindest/node:v1\\.36\\."
---

<objective>
Bump the default `kindest/node` image to a Kubernetes 1.36.x patch release so that `kinder create cluster` (without an explicit `image:` field) provisions a 1.36 node.

Purpose: Deliver SC2 — the new default user experience is a 1.36 cluster. Sequenced AFTER 51-01 (Envoy LB migration) because the LB image change should land in the same milestone as the K8s default bump, so users adopting the new default also get the new LB.

**CRITICAL — SYNC-02 image-availability blocker (per planner_guidance #2):**

As of 2026-05-07 (research date), **no `kindest/node:v1.36.x` image is published on Docker Hub** (latest is `v1.35.1`, kind main branch still defaults to `v1.35.1`, kind v0.32.0 unreleased). The SC2 floor of `>=1.36.4` is forward-projected — even v1.36.4 may not exist when this plan executes.

**Resolution chosen (option a from planner_guidance):** Pin the default constant to the LATEST AVAILABLE `kindest/node:v1.36.x@sha256:<digest>` at execute time, with a hard pre-flight check. The plan returns INCONCLUSIVE if no 1.36.x image is published yet.

The executor's first task is a Docker Hub probe. If the probe fails, the plan halts cleanly and reports back — phase verification will treat SC2 as deferred and surface to the user. This avoids: (b) creating a feature flag (becomes tech debt) and (c) deferring to a follow-up phase (loses the SC2 milestone alignment).

If a v1.36.x image with patch ≥4 IS available, this plan proceeds. If only v1.36.0..v1.36.3 exist, the executor SHOULD still proceed using the highest available tag — and note in SUMMARY.md that the SC2 floor of "≥1.36.4" was relaxed to the highest available patch (the floor's "kind #4131 regression" rationale was misattributed in the original ROADMAP per RESEARCH §SYNC-02 — issue #4131 was actually about a 1.35 regression, not 1.36, so the ≥1.36.4 floor is not load-bearing). The verifier will accept any v1.36.x default with a SUMMARY.md note explaining the floor relaxation.

If NO v1.36.x image exists, the plan returns INCONCLUSIVE.
</objective>

<execution_context>
@/Users/patrykattc/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/51-upstream-sync-k8s-1-36/51-RESEARCH.md

@pkg/apis/config/defaults/image.go
@kinder-site/src/content/docs/guides/multi-version-clusters.md
@kinder-site/src/content/docs/guides/tls-web-app.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Verify a kindest/node:v1.36.x image exists on Docker Hub (gating pre-flight)</name>
  <files></files>
  <action>
**THIS IS A GATING PRE-FLIGHT.** Do not modify any files in this task. The goal is to discover the latest available `kindest/node:v1.36.x` image on Docker Hub and capture both the tag and the sha256 digest for the next task.

Run the Docker Hub tags API query:
```bash
curl -s 'https://hub.docker.com/v2/repositories/kindest/node/tags/?page_size=100&name=v1.36' | \
  jq -r '.results[] | "\(.name)\t\(.digest // .images[0].digest // "no-digest")"' | \
  sort -V
```

Expected outcomes:

**Outcome A: At least one v1.36.x tag with a digest is returned.**
- Pick the highest patch version (e.g. `v1.36.4` over `v1.36.3` over `v1.36.0`).
- Capture the multi-arch digest. If `digest` field is empty, query the manifest list:
  ```bash
  TAG=v1.36.4   # whatever the highest is
  curl -s -H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" \
    -H "Authorization: Bearer $(curl -s 'https://auth.docker.io/token?service=registry.docker.io&scope=repository:kindest/node:pull' | jq -r .token)" \
    "https://registry-1.docker.io/v2/kindest/node/manifests/$TAG" | sha256sum
  ```
  (Or simpler: `docker manifest inspect kindest/node:$TAG --verbose` and read `.descriptor.digest`.)
- Note: the canonical kind format is `kindest/node:vX.Y.Z@sha256:<64-hex>`. The digest is the multi-arch index digest, not a single-arch image digest.
- If `docker pull` is available, also `docker pull kindest/node:$TAG` and `docker inspect kindest/node:$TAG --format '{{index .RepoDigests 0}}'` to get the digest in the canonical `kindest/node@sha256:...` format.
- Record `TAG` and `DIGEST` in the working notes for Task 2 below.
- **Proceed to Task 2.**

**Outcome B: No v1.36.x tags returned (jq output is empty).**
- The plan CANNOT proceed. Halt and write a brief INCONCLUSIVE note to `.planning/phases/51-upstream-sync-k8s-1-36/51-04-SUMMARY.md`:
  ```
  ## INCONCLUSIVE
  
  Plan 51-04 cannot land: no `kindest/node:v1.36.x` image is published on Docker Hub
  as of <today's date>. Latest available is `kindest/node:v1.35.1` (kind v0.31.0
  default). Re-run this plan after kind ships v0.32.0 (or whatever release
  publishes the v1.36 image), or after a manual `kind build node-image` is uploaded.
  
  SC2 (default node image is K8s 1.36.x) remains DEFERRED.
  
  Probe output:
  <paste the curl/jq output>
  ```
- Commit only the SUMMARY.md:
  ```
  git add .planning/phases/51-upstream-sync-k8s-1-36/51-04-SUMMARY.md
  git commit -m "docs(51-04): inconclusive — kindest/node:v1.36.x not yet on Docker Hub"
  ```
- **Stop. Do not proceed to Task 2.** Report INCONCLUSIVE to the orchestrator.

**Outcome C: API error (rate limit, network failure).**
- Retry once with a 30-second backoff. If still failing, report INCONCLUSIVE with the error message rather than guessing a digest.
  </action>
  <verify>
If Outcome A: a `TAG` and `DIGEST` value are captured (note them in the SUMMARY draft so Task 2 can use them). If Outcome B/C: SUMMARY.md committed with INCONCLUSIVE marker; no other files modified.
  </verify>
  <done>Either: (A) confirmed v1.36.x image exists and digest captured → continue. Or (B/C) plan halts cleanly with INCONCLUSIVE summary committed.</done>
</task>

<task type="auto">
  <name>Task 2: Update default image constant + website tag references (gated by Task 1 Outcome A)</name>
  <files>
    pkg/apis/config/defaults/image.go
    kinder-site/src/content/docs/guides/multi-version-clusters.md
    kinder-site/src/content/docs/guides/tls-web-app.md
  </files>
  <action>
**ONLY EXECUTE THIS TASK IF TASK 1 RETURNED OUTCOME A.**

Update `pkg/apis/config/defaults/image.go`:
- Read the current file (RESEARCH §SYNC-02 says line 21 holds `const Image = "kindest/node:v1.35.1@sha256:05d7bcdefbda08b4e038f644c4df690cdac3fba8b06f8289f30e10026720a1ab"`).
- Replace the constant value with the canonical form using the captured TAG and DIGEST from Task 1:
  ```go
  const Image = "kindest/node:<TAG>@sha256:<DIGEST>"
  ```
  Example shape (do NOT use this digest — use the one captured in Task 1):
  ```go
  const Image = "kindest/node:v1.36.4@sha256:abcdef0123456789..."
  ```
- Preserve comments and surrounding code style.

Update website example references that pin a specific older `kindest/node:vX.Y.Z` tag (RESEARCH §SYNC-02 calls these out):

1. `kinder-site/src/content/docs/guides/tls-web-app.md` — line 28 area mentions `kindest/node:v1.32.0`. Update to the new TAG (digest not needed in docs prose). If the surrounding paragraph talks about "v1.32" specifically as an example of an old version, leave it alone — only update lines that imply "this is the current default".
2. `kinder-site/src/content/docs/guides/multi-version-clusters.md` — search for `kindest/node:v1.35.1` and update to the new tag where it represents "the default". Leave references that explicitly demonstrate version-skew or older versions intact.

Do NOT update the project-wide `STATE.md` or `PROJECT.md` — only the user-facing constants and website example tags.

Run sanity checks:
```bash
go build ./...
go test ./pkg/apis/config/... -race
```
Both must pass. Run the existing default-image regression test if any (look for test files in the same package). Then run a broader grep:
```bash
grep -rn "kindest/node:v1\.35" pkg/ kinder-site/ 2>/dev/null
```
Acceptable matches: tests that use a fixed older version intentionally (e.g. version-skew tests), historical changelog/decisions, OR website prose that explicitly says "in earlier versions, kinder defaulted to v1.35.1." Anything that still says "the default is v1.35.1" must be updated.

If the grep surfaces unexpected production matches, update those files in this same task. Capture the final list in the SUMMARY.

Commit:
```
git add pkg/apis/config/defaults/image.go kinder-site/src/content/docs/guides/multi-version-clusters.md kinder-site/src/content/docs/guides/tls-web-app.md
git commit -m "feat(51-04): bump default kindest/node to v1.36.x"
```
(Replace `v1.36.x` in the commit message with the actual tag picked.)
  </action>
  <verify>
`grep -n "kindest/node:v1.36" pkg/apis/config/defaults/image.go` returns one match. `go build ./... && go test ./pkg/apis/config/... -race` exit 0. `grep -rn "kindest/node:v1\.35" pkg/apis/ kinder-site/src/content/docs/guides/` returns no matches that imply "default" (only intentional historical or version-skew references).
  </verify>
  <done>Default image bumped to confirmed-available v1.36.x; build green; tests pass; website default-tag references updated.</done>
</task>

</tasks>

<verification>
**If Task 1 returned Outcome A:**
- `pkg/apis/config/defaults/image.go` contains `kindest/node:v1.36` followed by a sha256 digest.
- `go build ./... && go test ./pkg/apis/config/... -race` exit 0.
- Website `multi-version-clusters.md` and `tls-web-app.md` reference the new tag where they previously said "v1.35.1" as the default.
- 1 commit landed on main; SUMMARY.md notes the picked TAG and DIGEST and whether the ≥1.36.4 floor was met.

**If Task 1 returned Outcome B/C:**
- Only the INCONCLUSIVE SUMMARY.md is committed.
- `pkg/apis/config/defaults/image.go` is unchanged.
- Phase verification flags SC2 as DEFERRED with the digest-availability gating note.
</verification>

<success_criteria>
SC2: Running `kinder create cluster` without an explicit `image:` field provisions a K8s 1.36.x node — satisfied by the default constant in `pkg/apis/config/defaults/image.go` pointing to a verified-available `kindest/node:v1.36.x@sha256:<digest>`. The "≥1.36.4" floor in the original ROADMAP is treated as guidance per RESEARCH §SYNC-02 (the #4131 attribution was a misread of a 1.35-only regression); the verifier will accept any v1.36.x default with a SUMMARY note explaining the picked patch.

If the gating pre-flight (Task 1) returns no v1.36.x image: SC2 is DEFERRED and surfaced as INCONCLUSIVE — re-run this plan after kind publishes a v1.36 image.
</success_criteria>

<output>
After completion, create `.planning/phases/51-upstream-sync-k8s-1-36/51-04-SUMMARY.md` with: Task 1 outcome (A/B/C), the picked TAG and DIGEST (if A), commits landed (SHAs), `go test ./pkg/apis/config/...` summary, and a final-state grep `grep -rn "kindest/node:v1\." pkg/ kinder-site/` listing showing what references each known tag.

If the plan returned INCONCLUSIVE, the SUMMARY explains why, links to the Docker Hub probe output, and notes that re-running the plan once a v1.36 image publishes will close out SC2.
</output>
