---
phase: 53-addon-version-audit-bumps-sync-05
plan: 00
type: execute
wave: 1
depends_on: []
files_modified:
  - pkg/apis/config/defaults/image.go
  - pkg/apis/config/defaults/image_test.go
  - kinder-site/src/content/docs/guides/multi-version-clusters.md
  - kinder-site/src/content/docs/guides/tls-web-app.md
  - kinder-site/src/content/docs/changelog.md
autonomous: true
requirements: [SYNC-05]

must_haves:
  truths:
    - "Plan 53-00 returns a definitive Outcome A (image found, source bumped) or Outcome B (INCONCLUSIVE, no source change)"
    - "If Outcome A: pkg/apis/config/defaults/image.go const Image points to a verified-existing kindest/node:v1.36.x@sha256:<digest>"
    - "If Outcome A: a TDD RED→GREEN test pair pins the v1.36 prefix and digest format"
    - "If Outcome A: website default-tag references in multi-version-clusters.md and tls-web-app.md track the new default"
    - "If Outcome B: no Go source code changes are made; SUMMARY.md records INCONCLUSIVE with the probe output and the SYNC-05 deferral note"
    - "Sub-plans 53-01 through 53-07 are unblocked regardless of probe outcome"
  artifacts:
    - path: ".planning/phases/53-addon-version-audit-bumps-sync-05/53-00-SUMMARY.md"
      provides: "Probe outcome record (A or B) with captured TAG/DIGEST or INCONCLUSIVE rationale"
      contains: "Outcome"
    - path: "pkg/apis/config/defaults/image.go"
      provides: "Default kindest/node image constant — bumped only on Outcome A"
      contains: "kindest/node:v1."
    - path: "pkg/apis/config/defaults/image_test.go"
      provides: "TestDefaultImageIsKubernetes136 — created on Outcome A"
      contains: "TestDefaultImageIsKubernetes136"
  key_links:
    - from: "Plan 53-00 Task 1 (probe)"
      to: "Task 2 GREEN edit of image.go (gated)"
      via: "Captured TAG + DIGEST from Docker Hub probe"
      pattern: "kindest/node:v1\\.36\\."
---

<objective>
Run the SYNC-05 Docker Hub two-step probe for `kindest/node:v1.36.x`. If the image exists, bump the default node image constant via a TDD RED→GREEN cycle (mirror of plan 51-04 Task 2). If the image does not exist, write an INCONCLUSIVE summary and stop — sub-plans 53-01 through 53-07 proceed regardless.

Purpose: Address SYNC-05 (default `kindest/node` to K8s 1.36.x conditional on Docker Hub publication) without blocking the rest of the phase. Researcher pre-ran the probe today (2026-05-10) and got `count=0`, so Outcome B is the most likely path at execute time. This plan codifies the canonical probe pattern so SYNC-05 can be re-run cleanly later.

Output: Either a 2-commit pair (RED test + GREEN bump) plus updated website examples, or a single INCONCLUSIVE summary commit. Either way, the next plan (53-01) is unblocked.
</objective>

<execution_context>
@/Users/patrykattc/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/REQUIREMENTS.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-CONTEXT.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-RESEARCH.md
@.planning/phases/51-upstream-sync-k8s-1-36/51-04-default-node-image-bump-PLAN.md
@.planning/phases/51-upstream-sync-k8s-1-36/51-04-SUMMARY.md

@pkg/apis/config/defaults/image.go
@kinder-site/src/content/docs/guides/multi-version-clusters.md
@kinder-site/src/content/docs/guides/tls-web-app.md
@kinder-site/src/content/docs/changelog.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Docker Hub two-step probe — discover kindest/node:v1.36.x existence and capture digest</name>
  <files></files>
  <action>
**THIS IS A GATING PRE-FLIGHT.** Do not modify any source files in this task. The goal is to discover the latest available `kindest/node:v1.36.x` image on Docker Hub and capture both the tag and the sha256 digest for Task 2.

This is a verbatim mirror of plan 51-04 Task 1 (the canonical SYNC probe pattern).

**Step 0 — record raw probe output to a deterministic scratch path** (so the verify step can assert outcome):

```bash
mkdir -p /tmp/53-00-probe
PROBE_RAW=/tmp/53-00-probe/raw.json
PROBE_OUTCOME=/tmp/53-00-probe/outcome   # "A" | "B" | "C"
```

**Step 1 — existence query:**

```bash
curl -sS -w '\nHTTP %{http_code}\n' \
  'https://hub.docker.com/v2/repositories/kindest/node/tags/?page_size=100&name=v1.36' \
  -o "$PROBE_RAW" 2>"$PROBE_RAW.err" && PROBE_HTTP_OK=1 || PROBE_HTTP_OK=0

# If curl itself failed (network), record Outcome C:
if [ "$PROBE_HTTP_OK" -ne 1 ]; then
  echo "C" > "$PROBE_OUTCOME"
fi

# Otherwise, parse:
if [ "$PROBE_HTTP_OK" -eq 1 ]; then
  jq -r '.results[] | "\(.name)\t\(.digest // .images[0].digest // "no-digest")"' "$PROBE_RAW" 2>/dev/null | sort -V > /tmp/53-00-probe/tags.tsv

  if [ -s /tmp/53-00-probe/tags.tsv ]; then
    echo "A" > "$PROBE_OUTCOME"
  else
    echo "B" > "$PROBE_OUTCOME"
  fi
fi

cat "$PROBE_OUTCOME"   # for the executor's eyeball
```

Branch on `$PROBE_OUTCOME`:

**Outcome A — `cat /tmp/53-00-probe/outcome` prints `A`. At least one v1.36.x tag with a digest is returned.**
- Pick the highest patch version from `/tmp/53-00-probe/tags.tsv` (e.g. `v1.36.4` over `v1.36.0`).
- Capture the multi-arch digest. If the `digest` field is empty, query the manifest index:
  ```bash
  TAG=v1.36.X   # whatever the highest is
  curl -s -H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" \
    -H "Authorization: Bearer $(curl -s 'https://auth.docker.io/token?service=registry.docker.io&scope=repository:kindest/node:pull' | jq -r .token)" \
    "https://registry-1.docker.io/v2/kindest/node/manifests/$TAG" | sha256sum
  ```
  Or, if Docker is available locally:
  ```bash
  docker pull kindest/node:$TAG
  docker inspect kindest/node:$TAG --format '{{index .RepoDigests 0}}'
  ```
- Canonical kind format is `kindest/node:vX.Y.Z@sha256:<64-hex>` (multi-arch index digest).
- **Record `TAG` and `DIGEST` to the scratch path so the verify step can assert non-emptiness:**
  ```bash
  echo "$TAG" > /tmp/53-00-probe/tag
  echo "$DIGEST" > /tmp/53-00-probe/digest
  test -s /tmp/53-00-probe/tag && test -s /tmp/53-00-probe/digest \
    || { echo "FAIL: TAG or DIGEST missing for Outcome A"; exit 1; }
  ```
- **Proceed to Task 2 (Outcome A path).**

**Outcome B — `cat /tmp/53-00-probe/outcome` prints `B`. No v1.36.x tags returned (count=0, results=[]).**
- The plan halts cleanly. Write `.planning/phases/53-addon-version-audit-bumps-sync-05/53-00-SUMMARY.md` using the INCONCLUSIVE template (mirror of 51-04-SUMMARY.md):
  ```markdown
  ## INCONCLUSIVE

  Plan 53-00 cannot proceed past the gating probe: no `kindest/node:v1.36.x` image
  is published on Docker Hub as of <today's date>. Latest available is
  `kindest/node:v1.35.1` (kind v0.31.0 default). Re-run this plan after kind ships
  v0.32.0 (or whatever release publishes the v1.36 image), or after a manual
  `kind build node-image` upload.

  SC6 / SYNC-05 (default node image is K8s 1.36.x) remains DEFERRED.

  ### Probe output
  GET https://hub.docker.com/v2/repositories/kindest/node/tags/?page_size=100&name=v1.36
  HTTP/2 200
  {"count":0,"next":null,"previous":null,"results":[]}

  Phase 53 sub-plans 53-01 through 53-07 proceed normally; SYNC-05 does NOT block addon work.
  ```
- Commit ONLY the SUMMARY.md:
  ```bash
  git add .planning/phases/53-addon-version-audit-bumps-sync-05/53-00-SUMMARY.md
  git commit -m "docs(53-00): inconclusive — kindest/node:v1.36.x not yet on Docker Hub"
  ```
- **Stop. Do not proceed to Task 2.** Report INCONCLUSIVE to the orchestrator.

**Outcome C — `cat /tmp/53-00-probe/outcome` prints `C`. API error (rate limit, network failure).**
- Retry once with a 30-second backoff. Re-write the outcome file based on the retry result. If still failing, treat as INCONCLUSIVE and follow the Outcome B template, replacing the probe output section with the error message captured in `/tmp/53-00-probe/raw.json.err`.
  </action>
  <verify>
<automated>test -f /tmp/53-00-probe/outcome && OUT=$(cat /tmp/53-00-probe/outcome) && case "$OUT" in A) test -s /tmp/53-00-probe/tag && test -s /tmp/53-00-probe/digest && [ -n "$(git diff --cached pkg/apis/config/defaults/image.go 2>/dev/null)$(git diff pkg/apis/config/defaults/image.go 2>/dev/null)" ] || echo "Outcome A recorded but TAG/DIGEST missing or no image.go change pending — Task 2 will validate" ;; B|C) test -f .planning/phases/53-addon-version-audit-bumps-sync-05/53-00-SUMMARY.md && grep -q "INCONCLUSIVE" .planning/phases/53-addon-version-audit-bumps-sync-05/53-00-SUMMARY.md ;; *) echo "Outcome '$OUT' is not one of A/B/C — probe is in an ambiguous state"; exit 1 ;; esac</automated>
  </verify>
  <done>Either: (A) `cat /tmp/53-00-probe/outcome` is `A`; `/tmp/53-00-probe/tag` and `/tmp/53-00-probe/digest` are non-empty; proceed to Task 2. Or (B/C) `cat /tmp/53-00-probe/outcome` is `B` or `C`; `.planning/phases/53-addon-version-audit-bumps-sync-05/53-00-SUMMARY.md` exists, contains the literal string `INCONCLUSIVE`, and was committed; no other files modified; plan halts here.</done>
</task>

<task type="auto">
  <name>Task 2: TDD bump of default image constant + website tag references + CHANGELOG stub (Outcome A only)</name>
  <files>
    pkg/apis/config/defaults/image.go
    pkg/apis/config/defaults/image_test.go
    kinder-site/src/content/docs/guides/multi-version-clusters.md
    kinder-site/src/content/docs/guides/tls-web-app.md
    kinder-site/src/content/docs/changelog.md
  </files>
  <action>
**ONLY EXECUTE THIS TASK IF TASK 1 RETURNED OUTCOME A** (i.e., `cat /tmp/53-00-probe/outcome` prints `A`).

Strict TDD discipline (RED → GREEN), matching plan 51-04 Task 2. Two atomic commits.

---

**STEP A — RED: write failing test for the default-image constant.**

Create `pkg/apis/config/defaults/image_test.go` (file did not exist after Outcome B path of plan 51-04; if it now exists, extend it):

```go
package defaults

import (
	"strings"
	"testing"
)

// TestDefaultImageIsKubernetes136 pins the default kindest/node image to a
// v1.36.x patch release. SYNC-05 of phase 53 requires that `kinder create cluster`
// without an explicit image: field provisions a Kubernetes 1.36 node.
func TestDefaultImageIsKubernetes136(t *testing.T) {
	if !strings.HasPrefix(Image, "kindest/node:v1.36") {
		t.Fatalf("default Image = %q; want prefix %q (SYNC-05: default must be K8s 1.36.x)", Image, "kindest/node:v1.36")
	}
	// Canonical kind format requires an @sha256: digest pin to prevent tag drift.
	if !strings.Contains(Image, "@sha256:") {
		t.Fatalf("default Image = %q; missing @sha256:<digest> pin", Image)
	}
}
```

Run the test — it MUST fail because `Image` still points to `kindest/node:v1.35.1@sha256:...`:
```bash
go test ./pkg/apis/config/defaults/... -run TestDefaultImageIsKubernetes136 -v
```
Confirm the failure output names the v1.35 tag.

Commit (RED):
```bash
git add pkg/apis/config/defaults/image_test.go
git commit -m "test(53-00): add failing test pinning default image to kindest/node:v1.36.x"
```

---

**STEP B — GREEN: update the `Image` constant + website default-tag references + CHANGELOG stub.**

Read the captured probe values:
```bash
TAG=$(cat /tmp/53-00-probe/tag)
DIGEST=$(cat /tmp/53-00-probe/digest)
test -n "$TAG" && test -n "$DIGEST" || { echo "FAIL: TAG/DIGEST scratch values missing"; exit 1; }
```

Edit `pkg/apis/config/defaults/image.go` line 21 (currently `const Image = "kindest/node:v1.35.1@sha256:05d7bcdefbda08b4e038f644c4df690cdac3fba8b06f8289f30e10026720a1ab"`). Replace with the captured TAG and DIGEST:

```go
const Image = "kindest/node:<TAG>@sha256:<DIGEST>"
```

Update website example references that pin a specific older `kindest/node:vX.Y.Z` tag as the default:

1. `kinder-site/src/content/docs/guides/multi-version-clusters.md` — search for `kindest/node:v1.35.1` and update to the new TAG where it represents "the default". Leave references that explicitly demonstrate version-skew or older versions intact.
2. `kinder-site/src/content/docs/guides/tls-web-app.md` — search for any `kindest/node:vX.Y.Z` references that imply "the current default" and update to the new TAG. Leave example-of-old-version mentions alone.

**CHANGELOG stub** — append to `kinder-site/src/content/docs/changelog.md` under the `## v2.4 — Hardening (in progress)` H2 (create the H2 if absent, placing it ABOVE the `## v1.5 — Inner Loop` section):

```markdown
- **Default `kindest/node` bumped to `<TAG>` (Kubernetes 1.36.x)** — `kinder create cluster` without an explicit `image:` field now provisions a Kubernetes 1.36 node. Existing clusters are unaffected; recreate to pick up the new default. (SYNC-05)
```

Run sanity checks:
```bash
go test ./pkg/apis/config/defaults/... -run TestDefaultImageIsKubernetes136 -v
go build ./...
go test ./pkg/apis/config/... -race
```
All must pass. Then a broader grep:
```bash
grep -rn "kindest/node:v1\.35" pkg/ kinder-site/ 2>/dev/null
```
Acceptable matches: tests using a fixed older version intentionally (version-skew tests), historical changelog/decisions, OR website prose that explicitly says "in earlier versions, kinder defaulted to v1.35.1." Anything that still says "the default is v1.35.1" must be updated in this same task.

Commit (GREEN):
```bash
git add pkg/apis/config/defaults/image.go \
        kinder-site/src/content/docs/guides/multi-version-clusters.md \
        kinder-site/src/content/docs/guides/tls-web-app.md \
        kinder-site/src/content/docs/changelog.md
git commit -m "feat(53-00): bump default kindest/node to <TAG> (SYNC-05)"
```
(Replace `<TAG>` in the commit message with the actual tag picked.)

**Open Question 3 (RESEARCH §Open Questions)**: If the executor has time, also add `TestDefaultImageRejectsIPVS` covering "default image with IPVS proxy rejects at validate" — quick assertion in the existing `validate_test.go`. Not mandatory for SYNC-05 close; optional polish per RESEARCH recommendation.
  </action>
  <verify>
<automated>go test ./pkg/apis/config/defaults/... -run TestDefaultImageIsKubernetes136 -v && go build ./... && go test ./pkg/apis/config/... -race</automated>
  </verify>
  <done>Outcome A only: (1) RED commit `test(53-00): ...` precedes GREEN commit `feat(53-00): ...` in `git log --oneline -2`. (2) `grep -n "kindest/node:v1.36" pkg/apis/config/defaults/image.go` returns one match. (3) `grep -rn "kindest/node:v1\.35" pkg/apis/ kinder-site/src/content/docs/guides/` returns no matches that imply "default" — only intentional historical or version-skew references. (4) CHANGELOG stub line for SYNC-05 present under the v2.4 H2.</done>
</task>

</tasks>

<verification>
**If Task 1 returned Outcome A:**
- `pkg/apis/config/defaults/image.go` contains `kindest/node:v1.36` followed by an `@sha256:` digest.
- `go build ./... && go test ./pkg/apis/config/... -race` exit 0.
- 2 commits landed (RED `test(53-00): ...` then GREEN `feat(53-00): ...`).
- CHANGELOG stub line landed atomically with the GREEN commit.
- SUMMARY.md notes the picked TAG and DIGEST.

**If Task 1 returned Outcome B/C:**
- Only the INCONCLUSIVE SUMMARY.md is committed.
- `pkg/apis/config/defaults/image.go` is unchanged.
- Phase 53 SC6 (SYNC-05) is flagged as DEFERRED with re-runnable status preserved.
- Sub-plans 53-01 through 53-07 proceed normally.
</verification>

<success_criteria>
SC6 (Phase 53): If Docker Hub two-step probe (existence + manifest digest) confirms `kindest/node:v1.36.x` is published, the default image constant in `pkg/apis/config/defaults/image.go` is updated and `kinder create cluster` with no `--image` flag uses K8s 1.36; otherwise SYNC-05 halts INCONCLUSIVE with re-runnable status.

The probe outcome (A or B/C) is recorded in `53-00-SUMMARY.md` so the verifier and the next phase planner can read the exact state without re-running the probe.
</success_criteria>

<output>
After completion, ensure `.planning/phases/53-addon-version-audit-bumps-sync-05/53-00-SUMMARY.md` exists with:
- Outcome (A, B, or C) clearly marked.
- If A: picked TAG, DIGEST, commits landed (SHAs), `go test ./pkg/apis/config/...` summary, final-state grep listing of `kindest/node:v1\.` references across the repo, CHANGELOG stub diff.
- If B/C: probe output verbatim, error (if C), and explicit deferral note for SYNC-05.
</output>
