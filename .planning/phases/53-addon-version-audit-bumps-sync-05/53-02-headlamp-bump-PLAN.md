---
phase: 53-addon-version-audit-bumps-sync-05
plan: 02
type: execute
wave: 3
depends_on: ["53-01"]
files_modified:
  - pkg/cluster/internal/create/actions/installdashboard/dashboard.go
  - pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go
  - pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
  - kinder-site/src/content/docs/addons/headlamp.md
  - kinder-site/src/content/docs/changelog.md
autonomous: false
requirements: [ADDON-02]

must_haves:
  truths:
    - "Images slice in installdashboard/dashboard.go pins ghcr.io/headlamp-k8s/headlamp:v0.42.0"
    - "Embedded manifests/headlamp.yaml references the v0.42.0 image"
    - "Embedded manifest preserves the existing kinder-dashboard-token Secret + ServiceAccount RBAC pattern (token-print flow intact)"
    - "Embedded manifest preserves -in-cluster arg and targetPort: 4466 (Pitfall HEAD-01)"
    - "TestImagesPinsHeadlampV042 passes; existing TestExecute (token-print smoke) continues to pass"
    - "Live UAT (kubectl auth can-i + curl UI) confirms token authenticates against the Headlamp UI on a fresh cluster"
    - "If live UAT fails, addon is held at v0.40.1 with reason documented in 53-02 commit + CHANGELOG"
    - "Atomic CHANGELOG stub for ADDON-02 (or hold) landed with the closing commit"
  artifacts:
    - path: "pkg/cluster/internal/create/actions/installdashboard/dashboard.go"
      provides: "Images slice pinning Headlamp v0.42.0"
      contains: "ghcr.io/headlamp-k8s/headlamp:v0.42.0"
    - path: "pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go"
      provides: "TestImagesPinsHeadlampV042 + retained TestExecute"
      contains: "TestImagesPinsHeadlampV042"
    - path: "pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml"
      provides: "Headlamp v0.42.0 manifest with kinder token-print wiring intact"
      contains: "headlamp-k8s/headlamp:v0.42.0"
  key_links:
    - from: "manifests/headlamp.yaml deployment args"
      to: "Token-print flow (dashboard.go reads kinder-dashboard-token Secret)"
      via: "-in-cluster arg + ServiceAccount + Secret of type kubernetes.io/service-account-token"
      pattern: "-in-cluster|kinder-dashboard-token"
    - from: "Live UAT result"
      to: "Hold-or-bump decision in checkpoint"
      via: "kubectl auth can-i + curl UI returning 200"
      pattern: "auth can-i|curl"
---

<objective>
Bump Headlamp from v0.40.1 to v0.42.0. Includes a MANDATORY live UAT (token auth smoke) gate before final commit. If the printed ServiceAccount token does not authenticate against the v0.42.0 UI, the addon is held at v0.40.1 with the failure reason recorded.

Purpose: ADDON-02 — bring the dashboard to current stable. The hold-trigger criteria (CONTEXT D-locks) are the safety net for any token-flow regression in v0.42.0; researcher confirmed v0.42.0 release notes show no breaking auth changes, but live verification is mandatory per roadmap.

Output: Either (a) RED+GREEN+post-UAT commit pair landing the bump and CHANGELOG stub, or (b) a single hold commit + CHANGELOG hold note keeping v0.40.1 in place. The plan finishes either way; the rest of the phase proceeds.
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

@pkg/cluster/internal/create/actions/installdashboard/dashboard.go
@pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go
@pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
@kinder-site/src/content/docs/addons/headlamp.md
@kinder-site/src/content/docs/changelog.md
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: RED + GREEN — bump Images slice and manifest to v0.42.0; preserve token-print wiring</name>
  <files>
    pkg/cluster/internal/create/actions/installdashboard/dashboard.go
    pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go
    pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
  </files>
  <behavior>
    - TestImagesPinsHeadlampV042 fails at HEAD (Images still v0.40.1) and passes after GREEN
    - TestExecute (existing token-print smoke) continues to pass after GREEN
    - Embedded manifest reference image = ghcr.io/headlamp-k8s/headlamp:v0.42.0
    - Embedded manifest preserves -in-cluster arg, targetPort: 4466, and the existing ServiceAccount/Secret/RBAC shape (Pitfall HEAD-01)
  </behavior>
  <action>
**STEP A — RED test.**

Open `pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go`. Append:

```go
// TestImagesPinsHeadlampV042 pins the dashboard Images slice to v0.42.0 (ADDON-02).
func TestImagesPinsHeadlampV042(t *testing.T) {
	t.Parallel()
	const want = "ghcr.io/headlamp-k8s/headlamp:v0.42.0"
	for _, img := range Images {
		if img == want {
			return
		}
	}
	t.Errorf("Images = %v; want to contain %q", Images, want)
}
```

Run:
```bash
go test ./pkg/cluster/internal/create/actions/installdashboard/... -run TestImagesPinsHeadlampV042 -v
```
MUST fail (Images still v0.40.1). Confirm and commit RED:
```bash
git add pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go
git commit -m "test(53-02): add failing test pinning Headlamp v0.42.0"
```

**STEP B — GREEN: bump Images slice + replace manifest YAML.**

1. Update `dashboard.go` Images slice (currently `"ghcr.io/headlamp-k8s/headlamp:v0.40.1"`):
   ```go
   var Images = []string{
   	"ghcr.io/headlamp-k8s/headlamp:v0.42.0",
   }
   ```

2. Replace the embedded manifest. Download upstream Headlamp v0.42.0 manifest (Headlamp ships YAML in its release / repo `kubernetes-headlamp.yaml`):
   ```bash
   curl -sL https://raw.githubusercontent.com/headlamp-k8s/headlamp/v0.42.0/kubernetes-headlamp.yaml \
     -o /tmp/upstream-headlamp-v0.42.0.yaml
   ```
   If that path 404s in v0.42.0, fall back to `https://github.com/headlamp-k8s/headlamp/releases/download/v0.42.0/kubernetes-headlamp.yaml` or whatever the v0.42.0 release lists. The exact path may differ; check `https://github.com/headlamp-k8s/headlamp/releases/tag/v0.42.0` for the manifest asset.

3. Diff against current `pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml`. Per RESEARCH §Pitfall HEAD-01, the kinder vendored manifest contains kinder-specific wiring:
   - **MUST PRESERVE** the kinder-specific `Secret` of type `kubernetes.io/service-account-token` annotated with the SA name (this is what `dashboard.go` reads to print the token).
   - **MUST PRESERVE** the `-in-cluster` arg in the deployment container args.
   - **MUST PRESERVE** `targetPort: 4466` on the Service (port-forward UX printed by the action depends on it).
   - The Headlamp upstream manifest uses a slightly different SA/Service shape; merge upstream's v0.42.0 RBAC/Deployment with kinder's vendored token Secret + service shape.

4. Replace the file body. The minimum change required is bumping the `image:` line to `ghcr.io/headlamp-k8s/headlamp:v0.42.0`. If upstream changed RBAC, container args, or the namespace beyond cosmetic, fold those in but do NOT regress the three preserved items above.

5. **Capture the actual SA + Secret names from the existing kinder-vendored manifest BEFORE you modify it** — do NOT hardcode. The current vendored manifest may use any SA/Secret name; the preservation invariant is "whatever the current names are, keep them." Run:
   ```bash
   git show HEAD:pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml | \
     awk '/^kind: ServiceAccount/{f="sa"; next} /^kind: Secret/{f="secret"; next} f && /^  name:/{print f": "$2; f=""}' \
     > /tmp/53-02-preserved-names.txt
   cat /tmp/53-02-preserved-names.txt
   ```
   Expected output is a two-line file like:
   ```
   sa: <captured-sa-name>
   secret: <captured-secret-name>
   ```
   These captured names are what step 6 must verify after the manifest swap.

6. Verify the preservation invariants by grep against the **captured** names from step 5 (NOT hardcoded strings):
   ```bash
   SA_NAME=$(awk -F': ' '$1=="sa"{print $2}' /tmp/53-02-preserved-names.txt)
   SECRET_NAME=$(awk -F': ' '$1=="secret"{print $2}' /tmp/53-02-preserved-names.txt)
   test -n "$SA_NAME" && test -n "$SECRET_NAME" || { echo "Step 5 capture failed; abort."; exit 1; }

   grep -n "$SA_NAME"     pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
   grep -n "$SECRET_NAME" pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
   grep -n -- "-in-cluster"     pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
   grep -n "targetPort: 4466"   pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
   grep -n "headlamp:v0.42.0"   pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
   ```
   All five MUST return at least one match. If `$SA_NAME` or `$SECRET_NAME` are not in the new manifest body (i.e., upstream renamed them), you must explicitly re-add the kinder-specific Secret/SA shape with the captured names — that's part of the "vendored wiring preservation" mandate.

7. Run tests:
   ```bash
   go test ./pkg/cluster/internal/create/actions/installdashboard/... -race
   go build ./...
   ```
   `TestImagesPinsHeadlampV042` and the existing `TestExecute` (token-print smoke via FakeNode) MUST both pass.

**Do NOT commit yet.** This is a working tree change held until Task 2 (live UAT) confirms or holds. If you want to commit-and-revert in case of hold, that's acceptable, but the cleaner path is: hold the working tree, run UAT, then commit GREEN with a confirmed v0.42.0 OR revert and commit a hold note.
  </action>
  <verify>
<automated>SA_NAME=$(awk -F': ' '$1=="sa"{print $2}' /tmp/53-02-preserved-names.txt 2>/dev/null) && SECRET_NAME=$(awk -F': ' '$1=="secret"{print $2}' /tmp/53-02-preserved-names.txt 2>/dev/null) && test -n "$SA_NAME" && test -n "$SECRET_NAME" && go test ./pkg/cluster/internal/create/actions/installdashboard/... -race && go build ./... && grep -q "$SA_NAME" pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml && grep -q "$SECRET_NAME" pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml && grep -q -- "-in-cluster" pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml && grep -q "targetPort: 4466" pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml && grep -q "headlamp:v0.42.0" pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml</automated>
  </verify>
  <done>RED commit landed; working tree contains v0.42.0 bump (Images + manifest); all unit tests pass; preservation invariants intact verified against captured (not hardcoded) SA/Secret names: `$SA_NAME`, `$SECRET_NAME`, `-in-cluster` arg, `targetPort: 4466`.</done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 2: Live UAT — token auth smoke against fresh cluster (HOLD GATE)</name>
  <what-built>Headlamp v0.42.0 bump in working tree (Images slice + embedded manifest). Unit tests pass; preservation invariants present. Now requires live verification before final commit.</what-built>
  <how-to-verify>
**This is the mandatory hold-trigger gate. Per CONTEXT.md hold criteria, ANY of the following triggers a hold at v0.40.1.**

1. Build the binary from the working tree:
   ```bash
   go build -o bin/kinder ./
   ```

2. Create a fresh cluster:
   ```bash
   ./bin/kinder create cluster --name uat-53-02
   ```
   The success message at the end of cluster create should print a SA token and a `kubectl port-forward` command.

3. Capture the printed token (call it `$TOKEN`).

4. **RBAC check** — token must authenticate against the API:
   ```bash
   kubectl auth can-i --token=$TOKEN get pods --all-namespaces
   ```
   Expected output: `yes`. ANY error (e.g. `error: You must be logged in to the server (Unauthorized)`) IS a HOLD trigger.

5. **UI check** — token must authenticate against the Headlamp UI:
   ```bash
   # In one terminal:
   kubectl port-forward -n kube-system svc/headlamp 4466:4466 &
   PF_PID=$!
   sleep 3

   # In another (or same) terminal:
   curl -sS -o /dev/null -w "%{http_code}\n" \
     -H "Authorization: Bearer $TOKEN" \
     http://localhost:4466/

   kill $PF_PID
   ```
   Expected output: `200`. A `401`, `302` redirect to `/login`, or any non-success IS a HOLD trigger.

6. **Manifest shape check** — confirm SA/Secret/Namespace did not silently break docs. Reuse the captured names from Task 1 step 5:
   ```bash
   SA_NAME=$(awk -F': ' '$1=="sa"{print $2}' /tmp/53-02-preserved-names.txt)
   SECRET_NAME=$(awk -F': ' '$1=="secret"{print $2}' /tmp/53-02-preserved-names.txt)
   kubectl -n kube-system get sa "$SA_NAME" 2>&1
   kubectl -n kube-system get secret "$SECRET_NAME" -o yaml 2>&1 | head -20
   ```
   Both must succeed. If the SA name, Secret name, or namespace changed in the v0.42.0 manifest in a way that breaks existing kinder docs, that IS a HOLD trigger.

7. Tear down the test cluster:
   ```bash
   ./bin/kinder delete cluster --name uat-53-02
   ```

**Decision tree:**

- **All three checks pass** → reply `bump` (and proceed to Task 3 GREEN+CHANGELOG commit).
- **ANY check fails** → reply `hold` with a paste of the failing output (and proceed to Task 3 hold path).
  </how-to-verify>
  <resume-signal>Reply `bump` if all three live checks pass; reply `hold` (with failure reason) to stay at v0.40.1.</resume-signal>
</task>

<task type="auto">
  <name>Task 3: Final commit — GREEN bump or HOLD note (depending on Task 2 outcome) + CHANGELOG stub + addon doc</name>
  <files>
    pkg/cluster/internal/create/actions/installdashboard/dashboard.go
    pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go
    pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
    kinder-site/src/content/docs/addons/headlamp.md
    kinder-site/src/content/docs/changelog.md
  </files>
  <action>
**Branch on Task 2 resume signal:**

---

**PATH A: bump (UAT passed) — commit GREEN + CHANGELOG stub + addon doc bump.**

1. Update addon doc `kinder-site/src/content/docs/addons/headlamp.md`:
   - Bump version mention near the top of the page (search for "v0.40.1" → "v0.42.0").
   - No `:::caution[Breaking change]` callout (UAT confirmed no breaking auth changes).

2. Append CHANGELOG stub to `kinder-site/src/content/docs/changelog.md` under the `## v2.4 — Hardening (in progress)` H2:
   ```markdown
   - **`Headlamp` bumped to v0.42.0** — token-print authentication flow re-verified live (`kubectl auth can-i` + UI curl with the printed SA token both succeed). Existing kinder-specific Secret + `-in-cluster` deployment arg pattern preserved. (ADDON-02)
   ```

3. Commit GREEN:
   ```bash
   git add pkg/cluster/internal/create/actions/installdashboard/dashboard.go \
           pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml \
           kinder-site/src/content/docs/addons/headlamp.md \
           kinder-site/src/content/docs/changelog.md
   git commit -m "feat(53-02): bump Headlamp to v0.42.0 (token UAT verified)"
   ```

---

**PATH B: hold (UAT failed) — revert working-tree changes and commit a hold note.**

1. Revert the bump in working tree:
   ```bash
   git checkout HEAD -- pkg/cluster/internal/create/actions/installdashboard/dashboard.go \
                        pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml
   ```

2. **The RED test (`TestImagesPinsHeadlampV042`) is still committed from Task 1.** It MUST be neutralized via `t.Skip` — do NOT delete the test. Deletion would discard the documented intent and break the re-runnability guarantee for the next bump attempt. The Skip path keeps the assertion in source as a ready-made entry point for retry.

   Edit `pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go` and modify the test body to call `t.Skip` as the first statement. The original assertion lines MUST be preserved verbatim below the Skip call (they are dead code under Skip but become live again when the Skip is removed on the next bump attempt):
   ```go
   func TestImagesPinsHeadlampV042(t *testing.T) {
   	t.Skip("HOLD per 53-02 SUMMARY: Headlamp v0.42.0 token UAT failed — re-enable when retried")
   	t.Parallel()
   	const want = "ghcr.io/headlamp-k8s/headlamp:v0.42.0"
   	for _, img := range Images {
   		if img == want {
   			return
   		}
   	}
   	t.Errorf("Images = %v; want to contain %q", Images, want)
   }
   ```

   Verify the Skip is in place and the body is preserved:
   ```bash
   grep -n "t.Skip(\"HOLD per 53-02 SUMMARY" pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go
   grep -n "ghcr.io/headlamp-k8s/headlamp:v0.42.0" pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go
   ```
   Both MUST return at least one match. Re-run `go test` to confirm the test reports `--- SKIP:` and the suite is green:
   ```bash
   go test ./pkg/cluster/internal/create/actions/installdashboard/... -run TestImagesPinsHeadlampV042 -v
   ```

3. Append CHANGELOG hold note to `kinder-site/src/content/docs/changelog.md` under the v2.4 H2:
   ```markdown
   - **`Headlamp` HELD at v0.40.1** — v0.42.0 live token UAT failed: <one-line reason from Task 2 paste>. The bump is documented and re-runnable (RED test preserved with `t.Skip`); see `.planning/phases/53-addon-version-audit-bumps-sync-05/53-02-SUMMARY.md`. (ADDON-02 — held)
   ```

4. Commit HOLD:
   ```bash
   git add pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go \
           kinder-site/src/content/docs/changelog.md
   git commit -m "docs(53-02): hold Headlamp at v0.40.1 — v0.42.0 token UAT failed"
   ```

5. Update the addon doc only if there's a user-facing reason — otherwise leave `kinder-site/src/content/docs/addons/headlamp.md` unchanged (still says v0.40.1).

---

In BOTH paths, write `.planning/phases/53-addon-version-audit-bumps-sync-05/53-02-SUMMARY.md` with the live UAT output (full curl/kubectl transcripts), the chosen path (A or B), the commit SHA, and a clear ADDON-02 status (delivered / held).
  </action>
  <verify>
<automated>git log --oneline -3 | head -3 && go test ./pkg/cluster/internal/create/actions/installdashboard/... -race && go build ./...</automated>
  </verify>
  <done>Either (Path A) v0.42.0 bumped, GREEN commit landed, addon doc + CHANGELOG stub updated; or (Path B) v0.40.1 held, RED test preserved with `t.Skip("HOLD per 53-02 SUMMARY: ...")` as the first statement (assertion body intact below), CHANGELOG hold note landed, working tree reverted. Either way: SUMMARY.md captures live UAT output; ADDON-02 has a definitive disposition.</done>
</task>

</tasks>

<verification>
- `go test ./pkg/cluster/internal/create/actions/installdashboard/... -race` exits 0.
- `go build ./...` exits 0.
- 53-02-SUMMARY.md contains the live UAT transcript (kubectl auth can-i output + curl HTTP code + SA/Secret existence checks).
- CHANGELOG has either an ADDON-02 bump line OR an ADDON-02 hold line under the v2.4 H2.
- If Path A: `grep "headlamp:v0.42.0" pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml` returns at least one match; `grep -- "-in-cluster" pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml` returns at least one match.
- If Path B: `grep "v0.40.1" pkg/cluster/internal/create/actions/installdashboard/dashboard.go` confirms held; `TestImagesPinsHeadlampV042` body retains both the `t.Skip("HOLD per 53-02 SUMMARY: ...")` first-statement and the original `for/want/Errorf` assertion below it (verified by grep returning matches for both).
</verification>

<success_criteria>
SC2 (Phase 53): `kinder create cluster` installs Headlamp v0.42.0; the printed ServiceAccount token authenticates successfully against the Headlamp UI (or a documented hold at v0.40.1 is in place with explanation). The live UAT in Task 2 IS the SC2 verification — it cannot be replaced by unit tests.

ADDON-02: Headlamp v0.42.0 with mandatory plan-time token-print flow verification.
</success_criteria>

<output>
`.planning/phases/53-addon-version-audit-bumps-sync-05/53-02-SUMMARY.md` records:
- Path taken (A: bump or B: hold)
- Live UAT transcript (kubectl auth can-i output, curl HTTP code, SA/Secret existence checks)
- Commits landed (RED + GREEN OR RED + HOLD)
- ADDON-02 status: delivered at v0.42.0 OR held at v0.40.1 with reason

If Path B, the SUMMARY explicitly notes that the RED test was preserved via `t.Skip("HOLD per 53-02 SUMMARY: ...")` as the first statement of the test body (assertion intact below) so the next bump attempt has a ready-made entry point — only removing the Skip call is needed to re-arm it.
</output>
