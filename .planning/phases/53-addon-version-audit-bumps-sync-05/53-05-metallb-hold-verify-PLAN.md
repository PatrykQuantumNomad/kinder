---
phase: 53-addon-version-audit-bumps-sync-05
plan: 05
type: execute
wave: 6
depends_on: ["53-04"]
files_modified:
  - kinder-site/src/content/docs/changelog.md
autonomous: true
requirements: [ADDON-05]

must_haves:
  truths:
    - "Upstream MetalLB latest GitHub release tag is verified at execute time"
    - "If latest is still v0.15.3, hold is confirmed and no source change is made"
    - "If a newer release exists, the plan flags the divergence and stops — does NOT silently bump"
    - "53-05-SUMMARY.md records the verified upstream tag, the verification command output, and the hold disposition"
    - "Atomic CHANGELOG stub for the hold confirmation lands in this plan's commit"
  artifacts:
    - path: ".planning/phases/53-addon-version-audit-bumps-sync-05/53-05-SUMMARY.md"
      provides: "Hold-verify record for MetalLB v0.15.3"
      contains: "v0.15.3"
    - path: "kinder-site/src/content/docs/changelog.md"
      provides: "CHANGELOG stub confirming MetalLB hold"
      contains: "MetalLB"
  key_links:
    - from: "GitHub releases API for metallb/metallb"
      to: "Pinned version v0.15.3 in installmetallb/metallb.go Images slice"
      via: "Version-string equality (latest tag == pinned tag)"
      pattern: "v0\\.15\\.3"
---

<objective>
Verify that MetalLB's pinned version (v0.15.3) is still the latest upstream release. Per CONTEXT.md ("Claude's Discretion") and RESEARCH §Open Question 1, the planner has chosen the **upstream version-check only** path — no live cluster smoke. The MetalLB action and addon docs already exercise the v0.15.3 path through every cluster create, so an unchanged upstream version means nothing to do.

Purpose: Close the MetalLB hold cleanly with verified evidence. If a newer release shipped between research date (2026-05-10) and execute time, the plan halts and surfaces the divergence — it does NOT silently auto-bump (that would lose the hold criteria + UAT review that an actual bump deserves).

Output: A single commit landing 53-05-SUMMARY.md plus a CHANGELOG stub confirming the hold. No Go source change unless the divergence path triggers (in which case the plan halts and reports back).
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

@pkg/cluster/internal/create/actions/installmetallb/metallb.go
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

Run the GitHub releases probe:
```bash
LATEST_TAG=$(curl -s 'https://api.github.com/repos/metallb/metallb/releases/latest' | jq -r '.tag_name')
echo "Latest MetalLB upstream release: $LATEST_TAG"

# Also list the top 5 published releases (in case a v0.16.x exists but is not marked "latest"):
curl -s 'https://api.github.com/repos/metallb/metallb/releases?per_page=5' | jq -r '.[] | "\(.tag_name)\t\(.published_at)\t\(.draft)\t\(.prerelease)"'
```

Capture the printed `LATEST_TAG` and the top-5 listing for the SUMMARY.

**STEP B — Compare against pinned version.**

The current pinned version in `pkg/cluster/internal/create/actions/installmetallb/metallb.go` is `v0.15.3` (both controller and speaker images). Compare:

- **If `LATEST_TAG == "v0.15.3"`:** hold confirmed. Proceed to Step C.
- **If `LATEST_TAG > v0.15.3` (e.g. `v0.15.4` or `v0.16.0` published since the research date):** halt. Do NOT silently bump. Write `.planning/phases/53-addon-version-audit-bumps-sync-05/53-05-SUMMARY.md` with status `DIVERGENCE`:
  ```markdown
  ## DIVERGENCE — MetalLB upstream has new release

  Pinned: v0.15.3 (in `pkg/cluster/internal/create/actions/installmetallb/metallb.go`)
  Latest upstream: <LATEST_TAG>
  Published: <date from releases listing>

  This plan does NOT auto-bump — a new MetalLB version requires the same hold-trigger discipline as the bumped addons (53-01 through 53-04). Open a follow-up plan or surface this in the v2.4 retrospective for v2.5 scope.

  Phase 53 still ships with MetalLB at v0.15.3 (existing kinder behavior preserved); no source change in this commit.
  ```
  Commit only the SUMMARY:
  ```bash
  git add .planning/phases/53-addon-version-audit-bumps-sync-05/53-05-SUMMARY.md
  git commit -m "docs(53-05): metalLB upstream divergence — new release detected; no auto-bump"
  ```
  Report DIVERGENCE to the orchestrator and stop. (53-06 and 53-07 still proceed; the MetalLB addon stays pinned at v0.15.3.)

- **If the API probe fails** (rate limit, network): retry once. If still failing, treat as DIVERGENCE-PROBE-ERROR and halt with a SUMMARY noting the probe error and the unchanged pin.

**STEP C — (hold-confirmed path) — write SUMMARY + CHANGELOG stub.**

Write `.planning/phases/53-addon-version-audit-bumps-sync-05/53-05-SUMMARY.md`:
```markdown
## HOLD CONFIRMED — MetalLB v0.15.3

Probed upstream `metallb/metallb` GitHub releases API at <today's date>:
- `tag_name` of latest release: v0.15.3
- Published: 2024-12-04 (per releases listing)
- Top 5 releases (no v0.16.x present): <paste the top-5 listing>

The pinned version in `pkg/cluster/internal/create/actions/installmetallb/metallb.go`
is unchanged at v0.15.3 (both controller and speaker images). No source change.

The MetalLB hold from REQUIREMENTS.md "Documented Holds" section is reaffirmed:
"Latest available; no v0.16 released."

This plan made no Go source changes. The CHANGELOG stub for ADDON-05 (offline-readiness consolidation, including MetalLB image entries) is added here atomically and consolidated in 53-07.
```

Append CHANGELOG hold-confirmation stub to `kinder-site/src/content/docs/changelog.md` under `## v2.4 — Hardening (in progress)`:
```markdown
- **`MetalLB` held at v0.15.3** — verified upstream `metallb/metallb` latest release is still v0.15.3 (published 2024-12-04). No newer release exists; the documented hold is unchanged.
```

Commit:
```bash
git add .planning/phases/53-addon-version-audit-bumps-sync-05/53-05-SUMMARY.md \
        kinder-site/src/content/docs/changelog.md
git commit -m "docs(53-05): confirm MetalLB hold at v0.15.3 (upstream still latest)"
```
  </action>
  <verify>
<automated>test -f .planning/phases/53-addon-version-audit-bumps-sync-05/53-05-SUMMARY.md && grep -q "MetalLB" kinder-site/src/content/docs/changelog.md</automated>
  </verify>
  <done>Either (hold-confirmed) SUMMARY.md records "HOLD CONFIRMED — MetalLB v0.15.3" with the probe output, and CHANGELOG has the hold-confirmation stub; or (DIVERGENCE) SUMMARY.md records the new upstream version and the plan halts without source change. In both cases, no Go source files were modified.</done>
</task>

</tasks>

<verification>
- `git log --oneline -1` shows a single docs(53-05) commit.
- `pkg/cluster/internal/create/actions/installmetallb/metallb.go` is unchanged from HEAD before this plan ran.
- 53-05-SUMMARY.md contains either "HOLD CONFIRMED" or "DIVERGENCE" status.
- CHANGELOG has a MetalLB-related line under the v2.4 H2.
</verification>

<success_criteria>
ADDON-05 (MetalLB portion): The MetalLB hold from REQUIREMENTS.md is reaffirmed with documented upstream evidence, OR a divergence is surfaced for follow-up.

The MetalLB image entries in `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` are consolidated in 53-07 (no per-plan offlinereadiness edits per CONTEXT.md decision).
</success_criteria>

<output>
`.planning/phases/53-addon-version-audit-bumps-sync-05/53-05-SUMMARY.md` records:
- Status: HOLD CONFIRMED / DIVERGENCE / DIVERGENCE-PROBE-ERROR
- The exact GitHub releases API output (tag_name + top-5 listing)
- The pinned version comparison
- Disposition for offlinereadiness consolidation in 53-07 (the MetalLB image tags do not change)
</output>
