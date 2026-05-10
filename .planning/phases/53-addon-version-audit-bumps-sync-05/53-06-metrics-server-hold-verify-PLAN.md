---
phase: 53-addon-version-audit-bumps-sync-05
plan: 06
type: execute
wave: 7
depends_on: ["53-05"]
files_modified:
  - kinder-site/src/content/docs/changelog.md
autonomous: true
requirements: [ADDON-05]

must_haves:
  truths:
    - "Upstream Metrics Server latest GitHub release tag is verified at execute time"
    - "If latest is still v0.8.1, hold is confirmed and no source change is made"
    - "If a newer release exists, the plan flags the divergence and stops — does NOT silently bump"
    - "53-06-SUMMARY.md records the verified upstream tag, the verification command output, and the hold disposition"
    - "Atomic CHANGELOG stub for the hold confirmation lands in this plan's commit"
  artifacts:
    - path: ".planning/phases/53-addon-version-audit-bumps-sync-05/53-06-SUMMARY.md"
      provides: "Hold-verify record for Metrics Server v0.8.1"
      contains: "v0.8.1"
    - path: "kinder-site/src/content/docs/changelog.md"
      provides: "CHANGELOG stub confirming Metrics Server hold"
      contains: "Metrics Server"
  key_links:
    - from: "GitHub releases API for kubernetes-sigs/metrics-server"
      to: "Pinned version v0.8.1 in installmetricsserver/metricsserver.go Images slice"
      via: "Version-string equality (latest tag == pinned tag)"
      pattern: "v0\\.8\\.1"
---

<objective>
Verify that Metrics Server's pinned version (v0.8.1) is still the latest upstream release. Same pattern as 53-05: upstream version-check only, no live cluster smoke (per Open Question 1 planner-discretion choice).

Purpose: Close the Metrics Server hold cleanly with verified evidence. Auto-bump is explicitly forbidden — a newer release halts the plan and surfaces the divergence so it can be planned with the same hold-trigger discipline as the bumped addons.

Output: A single commit landing 53-06-SUMMARY.md plus a CHANGELOG stub confirming the hold. No Go source change unless DIVERGENCE triggers (in which case the plan halts and reports back).
</objective>

<execution_context>
@/Users/patrykattc/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/REQUIREMENTS.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-CONTEXT.md
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-RESEARCH.md

@pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go
@kinder-site/src/content/docs/changelog.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Hold-verify probe + CHANGELOG stub</name>
  <files>
    kinder-site/src/content/docs/changelog.md
  </files>
  <action>
**STEP A — Upstream version probe.**

```bash
LATEST_TAG=$(curl -s 'https://api.github.com/repos/kubernetes-sigs/metrics-server/releases/latest' | jq -r '.tag_name')
echo "Latest Metrics Server upstream release: $LATEST_TAG"

curl -s 'https://api.github.com/repos/kubernetes-sigs/metrics-server/releases?per_page=5' | jq -r '.[] | "\(.tag_name)\t\(.published_at)\t\(.draft)\t\(.prerelease)"'
```

Capture the printed `LATEST_TAG` and the top-5 listing for the SUMMARY.

**STEP B — Compare against pinned version.**

The current pinned version in `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go` is `v0.8.1`. Compare:

- **If `LATEST_TAG == "v0.8.1"`:** hold confirmed. Proceed to Step C.
- **If `LATEST_TAG > v0.8.1` (e.g. `v0.8.2` or `v0.9.0` published since the research date):** halt with DIVERGENCE SUMMARY (mirror of 53-05 Step B):
  ```markdown
  ## DIVERGENCE — Metrics Server upstream has new release

  Pinned: v0.8.1 (in `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go`)
  Latest upstream: <LATEST_TAG>
  Published: <date>

  This plan does NOT auto-bump — a new Metrics Server version requires the same hold-trigger discipline as the bumped addons. Open a follow-up plan or surface in v2.4 retrospective for v2.5 scope.

  Phase 53 still ships with Metrics Server at v0.8.1; no source change in this commit.
  ```
  Commit only the SUMMARY:
  ```bash
  git add .planning/phases/53-addon-version-audit-bumps-sync-05/53-06-SUMMARY.md
  git commit -m "docs(53-06): metrics-server upstream divergence — new release detected; no auto-bump"
  ```
  Report DIVERGENCE and stop. (53-07 still proceeds.)

- **If the API probe fails:** retry once. If still failing, treat as DIVERGENCE-PROBE-ERROR.

**STEP C — (hold-confirmed path) — write SUMMARY + CHANGELOG stub.**

Write `.planning/phases/53-addon-version-audit-bumps-sync-05/53-06-SUMMARY.md`:
```markdown
## HOLD CONFIRMED — Metrics Server v0.8.1

Probed upstream `kubernetes-sigs/metrics-server` GitHub releases API at <today's date>:
- `tag_name` of latest release: v0.8.1
- Published: 2026-01-29 (per releases listing)
- Top 5 releases (no v0.9.x present): <paste the top-5 listing>

The pinned version in `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go`
is unchanged at v0.8.1. No source change.

The Metrics Server hold from REQUIREMENTS.md "Documented Holds" section is reaffirmed:
"Latest stable as of 2026-01-29; no v0.9.x."

This plan made no Go source changes. The Metrics Server image entry in
`pkg/internal/doctor/offlinereadiness.go` `allAddonImages` is consolidated
(unchanged) in 53-07.
```

Append CHANGELOG hold-confirmation stub to `kinder-site/src/content/docs/changelog.md` under `## v2.4 — Hardening (in progress)`:
```markdown
- **`Metrics Server` held at v0.8.1** — verified upstream `kubernetes-sigs/metrics-server` latest release is still v0.8.1 (published 2026-01-29). No newer release exists; the documented hold is unchanged.
```

Commit:
```bash
git add .planning/phases/53-addon-version-audit-bumps-sync-05/53-06-SUMMARY.md \
        kinder-site/src/content/docs/changelog.md
git commit -m "docs(53-06): confirm Metrics Server hold at v0.8.1 (upstream still latest)"
```
  </action>
  <verify>
<automated>test -f .planning/phases/53-addon-version-audit-bumps-sync-05/53-06-SUMMARY.md && grep -q "Metrics Server" kinder-site/src/content/docs/changelog.md</automated>
  </verify>
  <done>Either (hold-confirmed) SUMMARY.md records "HOLD CONFIRMED — Metrics Server v0.8.1" with the probe output, and CHANGELOG has the hold-confirmation stub; or (DIVERGENCE) SUMMARY.md records the new upstream version and the plan halts without source change. In both cases, no Go source files were modified.</done>
</task>

</tasks>

<verification>
- `git log --oneline -1` shows a single docs(53-06) commit.
- `pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go` is unchanged from HEAD before this plan ran.
- 53-06-SUMMARY.md contains either "HOLD CONFIRMED" or "DIVERGENCE" status.
- CHANGELOG has a Metrics-Server-related line under the v2.4 H2.
</verification>

<success_criteria>
ADDON-05 (Metrics Server portion): The Metrics Server hold from REQUIREMENTS.md is reaffirmed with documented upstream evidence, OR a divergence is surfaced for follow-up.

The Metrics Server image entry in `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` is unchanged (consolidated in 53-07 along with everything else, per CONTEXT.md "no per-plan offlinereadiness edits").
</success_criteria>

<output>
`.planning/phases/53-addon-version-audit-bumps-sync-05/53-06-SUMMARY.md` records:
- Status: HOLD CONFIRMED / DIVERGENCE / DIVERGENCE-PROBE-ERROR
- The exact GitHub releases API output
- The pinned version comparison
- Disposition for offlinereadiness consolidation in 53-07 (the Metrics Server image tag does not change)
</output>
