---
phase: 53-addon-version-audit-bumps-sync-05
plan: 08
type: execute
wave: 1
depends_on: []
files_modified:
  - .planning/ROADMAP.md
  - .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md
autonomous: true
gap_closure: true

must_haves:
  truths:
    - "ROADMAP.md Phase 53 SC1 second clause references cluster-node containerd verification via crictl (the verified evidence path) instead of an unsatisfiable-by-design 'no warn on a fresh default cluster' assertion"
    - "ROADMAP.md Phase 53 SC3 third clause references upstream runAsNonRoot enforcement + distroless USER directive (functional UID 65532) instead of an explicit manifest runAsUser pin"
    - "ROADMAP.md Phase 53 SC wording stays consistent with REQUIREMENTS.md ADDON-01, ADDON-03, ADDON-05 locked scope (no requirement scope drift)"
    - "53-VERIFICATION.md frontmatter reflects gap closure: gaps_filed entries are moved to a gaps_closed structure with pointer to 53-08 plan; status transitions from gaps_found to verified"
  artifacts:
    - path: ".planning/ROADMAP.md"
      provides: "Phase 53 Success Criteria 1 and 3 with revised wording aligned to verified evidence (crictl on cluster node; runAsNonRoot + distroless USER directive)"
      contains: "crictl"
    - path: ".planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md"
      provides: "Updated frontmatter — gaps_closed[] array with closure_plan=53-08 references for both gaps; status=verified"
      contains: "gaps_closed"
  key_links:
    - from: ".planning/ROADMAP.md Phase 53 SC1 second clause"
      to: "53-07-SUMMARY.md DEVIATION recommendation (lines 151-152)"
      via: "wording mirror — 'cluster node containerd image store (verified via crictl)'"
      pattern: "crictl.*cluster node"
    - from: ".planning/ROADMAP.md Phase 53 SC3 third clause"
      to: "53-03-SUMMARY.md UID Deviation analysis (lines 136-153)"
      via: "wording mirror — 'runAsNonRoot: true + distroless image USER nonroot directive (functional UID 65532)'"
      pattern: "runAsNonRoot.*distroless"
    - from: ".planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md gaps_closed[]"
      to: "53-08-PLAN.md"
      via: "closure_plan reference in frontmatter"
      pattern: "53-08"
---

<objective>
Close the two SC-wording verification gaps filed in 53-VERIFICATION.md (gaps_filed[0] and gaps_filed[1]) by revising the affected SC clauses in `.planning/ROADMAP.md` so they describe what is actually verified by Phase 53 evidence — and update 53-VERIFICATION.md frontmatter to mark both gaps closed.

Both gaps are pure documentation/SC-wording fixes. NO source code, manifest, test, CHANGELOG, release-note, or addon-doc changes. The verifier's evidence stands; only ROADMAP prose changes.

Purpose: Eliminate the silent-override pattern. The developer (golysoft@gmail.com) explicitly declined to accept either gap as a `must_have` override on 2026-05-12, requesting that the SC wording be formally revised to match the verified architectural behavior. This plan executes that revision.

Output:
- `.planning/ROADMAP.md` — Phase 53 SC1 second clause and SC3 third clause re-worded per 53-07-SUMMARY.md and 53-03-SUMMARY.md recommendations
- `.planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md` — frontmatter `gaps_filed` → `gaps_closed`; `status: gaps_found` → `status: verified`
</objective>

<execution_context>
@/Users/patrykattc/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/REQUIREMENTS.md
@.planning/STATE.md

# Gap source — authoritative listing of both gaps with closure_path
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md

# Source of recommended SC1 second-clause wording (DEVIATION section, lines 135-160)
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-07-SUMMARY.md

# Source of recommended SC3 third-clause wording (UID Deviation section, lines 136-160)
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-03-SUMMARY.md

# D-locks (REQUIREMENTS.md ADDON-01/03/05 are the requirement scope SC revisions must stay aligned with)
@.planning/phases/53-addon-version-audit-bumps-sync-05/53-CONTEXT.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Revise Phase 53 SC1 second clause and SC3 third clause in ROADMAP.md</name>
  <files>.planning/ROADMAP.md</files>
  <action>
Edit `.planning/ROADMAP.md` Phase 53 Success Criteria block (currently at lines 140–146).

**Edit A — SC1 second clause (line 141):**

Replace the current line 141 in full:

OLD:
```
  1. `kinder create cluster` installs local-path-provisioner v0.0.36 (GHSA-7fxv-8wr2-mfc4 security fix closed); `kinder doctor offline-readiness` does not warn on a fresh default cluster
```

NEW:
```
  1. `kinder create cluster` installs local-path-provisioner v0.0.36 (GHSA-7fxv-8wr2-mfc4 security fix closed); all 14 bumped/held addon tags are present on the cluster node's containerd image store (verified via `crictl images` on the control-plane node after `kinder create cluster`). NOTE: `kinder doctor offline-readiness` measures HOST docker pre-pull readiness for `--air-gapped` mode (not cluster-node store), so warns on a fresh default cluster by design; the air-gapped semantics are documented in 53-07-SUMMARY.md.
```

Rationale (record in commit message, not file): `realInspectImage` in `pkg/internal/doctor/offlinereadiness.go` lines 158–169 inspects the HOST docker store, not the cluster node's containerd store. The check is correct for its purpose (air-gapped pre-pull gate). Live UAT-5 (uat-53-07) confirmed all 14 addon tags ARE on the cluster node via `crictl images`. The new wording mirrors 53-07-SUMMARY.md lines 151–152 (recommended re-wording option 1) and references the actually-verified evidence path.

**Edit B — SC3 third clause (line 143):**

Replace the current line 143 in full:

OLD:
```
  3. `kinder create cluster` installs cert-manager v1.20.2 with `--server-side` apply; `kubectl get crd certificates.cert-manager.io -o jsonpath='{.spec.versions[0].name}'` returns the v1.20.2 API version; self-signed ClusterIssuer issues a certificate with the new UID (65532)
```

NEW:
```
  3. `kinder create cluster` installs cert-manager v1.20.2 with `--server-side` apply; `kubectl get crd certificates.cert-manager.io -o jsonpath='{.spec.versions[0].name}'` returns the v1.20.2 API version; self-signed ClusterIssuer issues a certificate from pods enforced non-root via pod-level `runAsNonRoot: true` + distroless image USER nonroot directive (functional UID 65532; upstream v1.20.2 does not pin `runAsUser: 65532` in the manifest — kubelet `runAsNonRoot: true` enforcement is the actual security guarantee per 53-03-SUMMARY.md UID Deviation section).
```

Rationale (record in commit message, not file): Upstream cert-manager v1.20.2 manifest sets `runAsNonRoot: true` at pod level but does NOT set explicit `runAsUser: 65532` — the UID is enforced by the distroless image's `USER nonroot` directive embedded in image metadata. Same security outcome (non-root enforcement, Pitfall CERT-03 satisfied), different mechanism than a manifest UID pin. Live UAT-3 confirmed pods Running with 0 restarts (kubelet accepted non-root) and CN=uat-test Certificate issued. The new wording mirrors 53-03-SUMMARY.md lines 136–153 (UID Deviation analysis) and stays aligned with REQUIREMENTS.md ADDON-03 (which says "UID change (1000→65532)" without prescribing the enforcement mechanism).

**Do not modify** lines 142 (SC2), 144 (SC4), 145 (SC5), 146 (SC6) — those SCs verified cleanly without overrides.

**Do not modify** REQUIREMENTS.md — ADDON-01/03/05 wording is locked at the requirement level (the SC clauses are the implementation-level expression of those requirements; only the SC expression needs adjustment to match verified evidence).
  </action>
  <verify>
Run all of the following from the repo root; ALL must be true:

1. Both new clauses present (1 hit each):
```bash
grep -c "containerd image store (verified via .crictl images." .planning/ROADMAP.md   # expect 1
grep -c "distroless image USER nonroot directive (functional UID 65532" .planning/ROADMAP.md   # expect 1
```

2. Both old clauses fully removed (0 hits each):
```bash
grep -c "does not warn on a fresh default cluster" .planning/ROADMAP.md   # expect 0
grep -c "self-signed ClusterIssuer issues a certificate with the new UID (65532)$" .planning/ROADMAP.md   # expect 0 — old wording ended on this token
```

3. Phase 53 SC block intact — still 6 numbered SCs (lines unchanged in count, only re-worded):
```bash
sed -n '/^### Phase 53:/,/^### Phase 54:/p' .planning/ROADMAP.md | grep -cE '^  [1-6]\. '   # expect 6
```

4. No accidental changes outside Phase 53 SC block:
```bash
git diff --stat .planning/ROADMAP.md   # expect ONLY .planning/ROADMAP.md, 2 lines changed (1 deletion + 1 insertion per edit, or as unified diff shows)
```

5. Wording stays consistent with REQUIREMENTS.md (locked scope still reads as v0.0.36 / v1.20.2 / UID 65532):
```bash
grep -c "65532" .planning/ROADMAP.md   # expect ≥ 2 (REQUIREMENTS scope phrase + revised SC3)
grep -c "v0.0.36" .planning/ROADMAP.md   # expect ≥ 1 (SC1)
grep -c "v1.20.2" .planning/ROADMAP.md   # expect ≥ 1 (SC3)
```
  </verify>
  <done>
ROADMAP.md Phase 53 Success Criteria block contains the two new wordings (SC1 second clause references crictl on cluster node containerd store; SC3 third clause references runAsNonRoot + distroless USER directive with functional UID 65532). Both old wordings are removed. SC2/SC4/SC5/SC6 unchanged. REQUIREMENTS.md unchanged. Atomic commit:

```
docs(53-08): revise Phase 53 SC1 second + SC3 third clauses to match verified evidence

SC1 second clause: replace "does not warn on a fresh default cluster" (unsatisfiable
by design — offline-readiness checks HOST docker pre-pull readiness for --air-gapped
mode, not cluster-node containerd store) with the actually-verified evidence path:
all 14 bumped/held addon tags present on the cluster node's containerd image store
via crictl on the control-plane node. Live UAT-5 (uat-53-07) confirmed this. Mirrors
53-07-SUMMARY.md DEVIATION recommendation option 1.

SC3 third clause: replace "issues a certificate with the new UID (65532)" (implied
manifest-level runAsUser: 65532 pin) with the actual upstream enforcement mechanism:
runAsNonRoot: true at pod level + distroless image USER nonroot directive (functional
UID 65532; kubelet enforces non-root). Live UAT-3 confirmed pods Running + CN=uat-test
Certificate issued. Mirrors 53-03-SUMMARY.md UID Deviation analysis.

Closes both verification gaps filed 2026-05-12 by developer decision (no override
acceptance). Pure documentation fix — no code, test, manifest, CHANGELOG, release-note,
or addon-doc changes required. REQUIREMENTS.md ADDON-01/03/05 scope unchanged.
```
  </done>
</task>

<task type="auto">
  <name>Task 2: Close both gaps in 53-VERIFICATION.md frontmatter</name>
  <files>.planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md</files>
  <action>
Edit the frontmatter (lines 1–74) of `.planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md` to reflect gap closure landed by Task 1.

**Edit A — status field (line 5):**

OLD:
```
status: gaps_found
```

NEW:
```
status: verified
```

**Edit B — score field (line 6):**

OLD:
```
score: 4/6 must-haves fully verified (SC6 deferred; SC1 second clause + SC3 third clause filed as gap closures per developer decision 2026-05-12)
```

NEW:
```
score: 6/6 must-haves verified (SC6 deferred per Outcome B re-runnable status; SC1 second clause + SC3 third clause closed via 53-08 SC wording revision landed 2026-05-12)
```

**Edit C — replace `gaps_filed:` block (lines 8–23) with `gaps_closed:` block.**

OLD (preserve exact indentation when matching):
```
gaps_filed:
  - sc: "SC1 second clause"
    issue: >
      Wording 'kinder doctor offline-readiness does not warn on a fresh default cluster'
      is unsatisfiable by design — the check measures HOST docker pre-pull readiness for
      --air-gapped mode, not cluster node store. Recommend rewording to reference crictl
      verification on the cluster node, OR adding a pre-pull precondition.
    closure_path: "Documentation/ROADMAP.md SC revision (no code change)"
  - sc: "SC3 third clause"
    issue: >
      Wording 'self-signed ClusterIssuer issues a certificate with the new UID (65532)'
      assumes an explicit manifest runAsUser field. Upstream cert-manager v1.20.2 uses
      distroless USER nonroot + kubelet runAsNonRoot: true enforcement (same security
      outcome, different mechanism). Recommend rewording to reference the non-root
      enforcement mechanism, OR adding an ephemeral debug container UID assertion step.
    closure_path: "Documentation/ROADMAP.md SC revision (no code change)"
```

NEW:
```
gaps_closed:
  - sc: "SC1 second clause"
    closed_at: "2026-05-12T00:00:00Z"
    closure_plan: "53-08-sc-wording-revision-PLAN.md"
    resolution: >
      ROADMAP.md SC1 second clause re-worded from 'kinder doctor offline-readiness does
      not warn on a fresh default cluster' (unsatisfiable by design) to 'all 14 bumped/held
      addon tags are present on the cluster node containerd image store (verified via
      crictl images on the control-plane node)' — the actually-verified evidence path.
      Air-gapped semantics of offline-readiness check explicitly documented inline. See
      53-07-SUMMARY.md DEVIATION section for full architectural analysis.
  - sc: "SC3 third clause"
    closed_at: "2026-05-12T00:00:00Z"
    closure_plan: "53-08-sc-wording-revision-PLAN.md"
    resolution: >
      ROADMAP.md SC3 third clause re-worded from 'issues a certificate with the new UID
      (65532)' (implied manifest runAsUser pin — absent in upstream v1.20.2 by design) to
      'pods enforced non-root via pod-level runAsNonRoot: true + distroless image USER
      nonroot directive (functional UID 65532)'. Same security guarantee, accurate mechanism.
      Live UAT-3 evidence preserved (pods Running, CN=uat-test Certificate issued). See
      53-03-SUMMARY.md UID Deviation section for full upstream-design rationale.
```

**Edit D — remove `overrides_applied`, `overrides`, and `human_verification` blocks** (lines 24–37 and 53–73), since the gaps are closed by SC revision rather than override acceptance, and human verification is satisfied by the closure plan landing.

OLD (lines 24–37):
```
overrides_applied: 0
overrides:
  - must_have: "SC1 second clause: kinder doctor offline-readiness does not warn on a fresh default cluster"
    reason: >
      realInspectImage in offlinereadiness.go inspects the HOST docker store, not the cluster
      node's containerd store. This is correct behavior — the check measures air-gapped readiness
      (host must pre-pull images before --air-gapped). A default cluster that boots normally will
      ALWAYS trigger warn because addon images are only in containerd, not in host docker.
      Live UAT-5 (uat-53-07) confirmed all 14 bumped addon tags ARE present on the cluster node
      via crictl. SC1 second clause as written is unsatisfiable by design; the intent (images
      correctly tracked and delivered) is fully met. 53-07 SUMMARY documents this as an
      architectural finding, not a regression. See deferred section for wording fix tracking.
    accepted_by: "verifier (pending human confirmation — see human_verification section)"
    accepted_at: "2026-05-10T00:00:00Z"
```

NEW: remove entirely (no replacement — gap closed via plan, not override).

OLD (lines 53–73):
```
human_verification:
  - test: "Confirm SC1 second-clause override acceptance"
    expected: >
      Developer confirms that 'kinder doctor offline-readiness warns on a default cluster by design'
      is acceptable — it is an air-gapped pre-pull gate, not a cluster-health check.
      All 14 addon tags are on the cluster node containerd store (verified via crictl in UAT-5).
    why_human: >
      This is a success-criteria semantics decision: the verifier cannot autonomously accept
      a must-have override for an SC that was defined in ROADMAP.md. Developer confirmation
      required before marking SC1 second clause as PASSED (override).
  - test: "Confirm SC3 third clause UID 65532 deviation acceptance"
    expected: >
      Developer confirms that cert-manager v1.20.2 achieving UID 65532 via distroless image
      USER nonroot directive (kubelet runAsNonRoot: true enforcement) — rather than an explicit
      manifest runAsUser: 65532 field — is acceptable as the security intent is satisfied.
      Live UAT-3 confirmed pods ran without root; CN=uat-test Certificate issued successfully.
    why_human: >
      SC3 says 'self-signed ClusterIssuer issues a certificate with the new UID (65532)'.
      The manifest does not pin runAsUser: 65532 explicitly. The upstream design (distroless +
      kubelet enforcement) achieves the same security guarantee. This is a judgment call on
      whether the SC wording requires an explicit manifest field or accepts upstream enforcement.
```

NEW: remove entirely (human decision recorded — developer chose closure path 2026-05-12; satisfied by 53-08 landing).

**Edit E — preserve `deferred:` block** (the SC6 deferred-to-53-00-rerun entry must stay; the SC1 deferred entry on lines 46–52 referencing "Phase 54+ documentation fix" should be removed because the fix landed in 53-08, not Phase 54+).

OLD `deferred:` block:
```
deferred:
  - truth: "SC6: default image constant updated to kindest/node:v1.36.x"
    addressed_in: "Phase 53 sub-plan 53-00 re-run (when kind publishes v0.32.0)"
    evidence: >
      Plan 53-00 Outcome B executed correctly: Docker Hub API returned HTTP 200 with count=0 for
      v1.36 tags. SC6 defines Outcome B as the valid halting condition with re-runnable status.
      image.go correctly remains at kindest/node:v1.35.1. No code regression; re-run
      instructions documented in 53-00-SUMMARY.md.
  - truth: "SC1 second clause wording corrected to reflect air-gapped semantics"
    addressed_in: "Phase 54+ (documentation fix)"
    evidence: >
      53-07 SUMMARY explicitly calls out the wording fix as a follow-up item. The functional
      behavior is correct; only the ROADMAP SC wording needs updating to say 'all bumped addon
      tags are present on the cluster node's containerd store (verified via crictl after
      kinder create cluster)' instead of 'no warn on a fresh default cluster'.
```

NEW (keep only the SC6 entry):
```
deferred:
  - truth: "SC6: default image constant updated to kindest/node:v1.36.x"
    addressed_in: "Phase 53 sub-plan 53-00 re-run (when kind publishes v0.32.0)"
    evidence: >
      Plan 53-00 Outcome B executed correctly: Docker Hub API returned HTTP 200 with count=0 for
      v1.36 tags. SC6 defines Outcome B as the valid halting condition with re-runnable status.
      image.go correctly remains at kindest/node:v1.35.1. No code regression; re-run
      instructions documented in 53-00-SUMMARY.md.
```

**Edit F — leave the report body (everything after the closing `---` on line 74) UNCHANGED.** The report's "Goal Achievement", "Human Verification Required", etc. sections are a historical snapshot of the 2026-05-10 verification run and should not be retroactively edited. The frontmatter is the canonical machine-readable status.

**Do NOT** modify `human_resolved` (line 4) — it is the timestamp of the developer's decision to file the gaps for closure, which still happened on 2026-05-12.

**Do NOT** modify `human_decision` (line 7) — it documents the decision rationale and remains historically accurate.
  </action>
  <verify>
Run all of the following from the repo root; ALL must be true:

1. Status transitions to verified:
```bash
grep -c "^status: verified$" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 1
grep -c "^status: gaps_found$" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 0
```

2. `gaps_closed:` replaces `gaps_filed:`:
```bash
grep -c "^gaps_closed:" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 1
grep -c "^gaps_filed:" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 0
```

3. Both gap closures reference 53-08 plan:
```bash
grep -c "closure_plan: \"53-08-sc-wording-revision-PLAN.md\"" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 2
```

4. Override/human-verification blocks removed (since closure landed via plan, not override):
```bash
grep -c "^overrides_applied:" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 0
grep -c "^overrides:" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 0
grep -c "^human_verification:" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 0
```

5. SC6 deferred entry preserved; SC1-wording-deferred entry removed:
```bash
grep -c "SC6: default image constant updated to kindest/node:v1.36.x" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 1 (still deferred to 53-00 re-run)
grep -c "SC1 second clause wording corrected to reflect air-gapped semantics" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 0 (no longer deferred — closed by 53-08)
```

6. Report body (post-frontmatter) untouched — line count for lines 76+ unchanged from pre-edit (lines 1–74 are frontmatter; the report body starts at line 76):
```bash
# Pre-edit body line count is 164 lines (lines 75..238 — includes the blank line after closing ---). After the frontmatter shrinks the absolute line numbers shift but the body content must be byte-identical.
awk 'BEGIN{f=0} /^---$/{f++; next} f==2{print}' .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md | wc -l   # expect 164 (the report body lines after the closing frontmatter ---, including blank line 75)
```
  </verify>
  <done>
53-VERIFICATION.md frontmatter shows:
- `status: verified` (was `gaps_found`)
- `score:` updated to 6/6 with reference to 53-08 closure
- `gaps_closed:` array (was `gaps_filed:`) with both SCs and `closure_plan: 53-08-sc-wording-revision-PLAN.md`
- `overrides:` / `overrides_applied:` / `human_verification:` blocks removed
- `deferred:` reduced to the SC6-only entry
- Report body (post-frontmatter, lines after the closing `---`) byte-identical

Atomic commit:

```
docs(53-08): close gaps_filed[0,1] in 53-VERIFICATION.md frontmatter — SC wording landed in 53-08

Following Task 1's ROADMAP.md SC1 second-clause and SC3 third-clause re-wording,
move both gap entries from gaps_filed to gaps_closed (with closure_plan: 53-08).
Remove overrides_applied/overrides/human_verification blocks (decision path was
SC revision, not override acceptance — recorded in commit history via 53-08).
Remove the deferred SC1-wording entry (no longer deferred; landed in this plan).
Preserve the SC6 deferred entry (still re-runnable when kind publishes v0.32.0).
Status: gaps_found → verified; score: 4/6 → 6/6 (SC6 deferred per Outcome B).

Report body (post-frontmatter) preserved byte-identical — historical 2026-05-10
verification snapshot is not retroactively edited.
```
  </done>
</task>

</tasks>

<verification>

**Phase-level checks (run after both tasks complete):**

1. ROADMAP.md Phase 53 SC block contains the new wordings and no remnants of the old:
```bash
# Both new wordings present once
grep -c "crictl images.*control-plane node" .planning/ROADMAP.md   # expect 1
grep -c "runAsNonRoot.*distroless image USER nonroot" .planning/ROADMAP.md   # expect 1

# Both old wordings fully gone
grep -c "does not warn on a fresh default cluster" .planning/ROADMAP.md   # expect 0
grep "self-signed ClusterIssuer issues a certificate with the new UID (65532)$" .planning/ROADMAP.md   # expect 0 matches (string anchored to end-of-line)
```

2. VERIFICATION.md frontmatter shows closure:
```bash
grep -c "^status: verified$" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 1
grep -c "^gaps_closed:" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 1
grep -c "closure_plan: \"53-08-sc-wording-revision-PLAN.md\"" .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md   # expect 2
```

3. Atomic commits — exactly 2 new commits on the branch from this plan:
```bash
git log --oneline | head -5
# Expect top-two commits to be:
# - docs(53-08): close gaps_filed[0,1] in 53-VERIFICATION.md frontmatter ...
# - docs(53-08): revise Phase 53 SC1 second + SC3 third clauses ...
```

4. No code or test files modified by this plan (doc-only):
```bash
git diff --name-only HEAD~2..HEAD
# Expect EXACTLY these two paths:
# .planning/ROADMAP.md
# .planning/phases/53-addon-version-audit-bumps-sync-05/53-VERIFICATION.md
```

5. REQUIREMENTS.md untouched (locked requirement scope unchanged):
```bash
git diff HEAD~2..HEAD -- .planning/REQUIREMENTS.md   # expect empty output
```

6. Build/test sanity (the plan should not break anything, but verify since ROADMAP is parsed by some GSD tooling):
```bash
go build ./... 2>&1 | head -5   # expect no output (clean build — sanity only, this plan touches no Go code)
```

</verification>

<success_criteria>

- [ ] Both gap closure paths from 53-VERIFICATION.md `gaps_filed[]` are now reflected in ROADMAP.md SC wording
- [ ] SC1 second clause references crictl on cluster-node containerd store (the actually-verified evidence)
- [ ] SC3 third clause references runAsNonRoot + distroless USER directive (the actual upstream enforcement mechanism)
- [ ] 53-VERIFICATION.md frontmatter shows `status: verified` and `gaps_closed: [SC1, SC3]` with `closure_plan: 53-08-...`
- [ ] No code/test/manifest/CHANGELOG/release-note/addon-doc files modified
- [ ] REQUIREMENTS.md ADDON-01/03/05 wording unchanged (requirement-level scope intact)
- [ ] Exactly 2 atomic commits added by this plan
- [ ] `go build ./...` continues to pass (sanity — no Go code touched)
- [ ] Report body (post-frontmatter) of 53-VERIFICATION.md preserved byte-identical (historical snapshot integrity)

</success_criteria>

<output>
After completion, create `.planning/phases/53-addon-version-audit-bumps-sync-05/53-08-SUMMARY.md` recording:
- Both old SC clause texts (verbatim)
- Both new SC clause texts (verbatim)
- Pointer to 53-07-SUMMARY.md (SC1 source recommendation) and 53-03-SUMMARY.md (SC3 source recommendation)
- Confirmation that gaps_filed → gaps_closed transition landed in 53-VERIFICATION.md frontmatter
- Statement that REQUIREMENTS.md ADDON-01/03/05 scope is unchanged (only SC-level expression revised)
- Note that the cosmetic envoygw.go line 84 stale comment (Info severity, VERIFICATION anti-patterns table) was intentionally NOT bundled — it is unrelated to gap closure and is tracked separately in 53-VERIFICATION.md anti-patterns table
</output>
