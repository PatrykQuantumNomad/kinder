---
phase: 53-addon-version-audit-bumps-sync-05
plan: 03
type: execute
wave: 4
depends_on: ["53-02"]
files_modified:
  - pkg/cluster/internal/create/actions/installcertmanager/certmanager.go
  - pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go
  - pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
  - kinder-site/src/content/docs/addons/cert-manager.md
  - kinder-site/src/content/docs/changelog.md
autonomous: false
requirements: [ADDON-03]

must_haves:
  truths:
    - "Images slice in installcertmanager/certmanager.go pins cert-manager v1.20.2 across cainjector, controller, and webhook"
    - "Embedded manifests/cert-manager.yaml is the upstream v1.20.2 manifest (~992 KB)"
    - "Apply call in certmanager.go preserves the --server-side flag (Pitfall CERT-01 — manifest >256 KB requires server-side apply)"
    - "TestImagesPinsV1202 and TestExecuteUsesServerSideApply pass"
    - "Live UAT confirms self-signed ClusterIssuer issues a Certificate on a fresh cluster, AND running cert-manager pods report runAsUser=65532"
    - "If live UAT fails (ClusterIssuer or UID), addon held at v1.16.3 with reason documented"
    - "Addon doc has :::caution[Breaking change in v1.20] callout for UID 1000→65532 AND rotationPolicy: Always GA-mandatory flip"
    - "Atomic CHANGELOG stub for ADDON-03 (or hold) landed with the closing commit"
  artifacts:
    - path: "pkg/cluster/internal/create/actions/installcertmanager/certmanager.go"
      provides: "Images slice pinning cert-manager v1.20.2 (3 images); --server-side apply preserved"
      contains: "cert-manager-controller:v1.20.2"
    - path: "pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go"
      provides: "TestImagesPinsV1202 + TestExecuteUsesServerSideApply"
      contains: "TestExecuteUsesServerSideApply"
    - path: "pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml"
      provides: "Upstream cert-manager v1.20.2 manifest"
      contains: "cert-manager-controller:v1.20.2"
    - path: "kinder-site/src/content/docs/addons/cert-manager.md"
      provides: "Addon doc with :::caution[Breaking change in v1.20] callout"
      contains: "65532"
  key_links:
    - from: "certmanager.go Execute() apply call"
      to: "Manifest size 992 KB"
      via: "kubectl apply --server-side flag"
      pattern: "--server-side"
    - from: "Live UAT pod inspection"
      to: "UID 65532 confirmation"
      via: "kubectl get pod ... -o jsonpath='{.spec.containers[0].securityContext.runAsUser}'"
      pattern: "65532"
---

<objective>
Bump cert-manager from v1.16.3 to v1.20.2. Live UAT must confirm: (a) self-signed ClusterIssuer can issue a Certificate, AND (b) running cert-manager pods report `runAsUser=65532` (the upstream v1.20.0 default change). Failure of either triggers a hold at v1.16.3.

Purpose: ADDON-03 — cert-manager current stable. The 992 KB manifest mandates `--server-side` apply (already locked); the bump must NOT regress that flag (Pitfall CERT-01). UID change (1000→65532) and `rotationPolicy: Always` GA-mandatory flip require user-facing disclosure (CHANGELOG + addon doc :::caution callout).

Output: Either (a) RED+GREEN+post-UAT commit pair landing the bump and CHANGELOG stub, or (b) a single hold commit + CHANGELOG hold note keeping v1.16.3 in place.

**CRITICAL — UID is 65532 (not 65632):** RESEARCH §user_constraints flags that the CONTEXT.md decision bullet had a typo "65632"; REQUIREMENTS.md ADDON-03 and the upstream cert-manager v1.20.0 release notes both say **65532**. Use `65532` in plans, tests, addon doc, and CHANGELOG.
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

@pkg/cluster/internal/create/actions/installcertmanager/certmanager.go
@pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go
@pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
@kinder-site/src/content/docs/addons/cert-manager.md
@kinder-site/src/content/docs/changelog.md
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: RED + GREEN — pin v1.20.2 + assert --server-side preserved + replace 992 KB upstream manifest</name>
  <files>
    pkg/cluster/internal/create/actions/installcertmanager/certmanager.go
    pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go
    pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
  </files>
  <behavior>
    - TestImagesPinsV1202 fails at HEAD (Images still pins v1.16.3) and passes after GREEN
    - TestExecuteUsesServerSideApply (greps captured FakeCmd args for "--server-side") passes both before and after GREEN — guards Pitfall CERT-01 going forward
    - Embedded manifest body matches upstream cert-manager v1.20.2 (around 992 KB; 3 unique images: cainjector, controller, webhook)
  </behavior>
  <action>
**STEP A — RED tests.**

Open `pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go`. The file already has a `makeCtx(cmds []*testutil.FakeCmd) (*actions.ActionContext, *testutil.FakeNode)` helper and an existing `TestExecute` table-driven against `[]*testutil.FakeCmd`. The `*FakeNode` returned by `makeCtx` exposes `node.Calls [][]string` — every `node.Command(...)` and `node.CommandContext(...)` invocation appends `[command, args...]` to that slice (verified at `pkg/cluster/internal/create/actions/testutil/fake.go:124-128`). That is the arg-capture mechanism — no new mocking pattern needed.

Append to the file:

```go
// TestImagesPinsV1202 pins all three cert-manager images to v1.20.2 (ADDON-03).
func TestImagesPinsV1202(t *testing.T) {
	t.Parallel()
	wants := []string{
		"quay.io/jetstack/cert-manager-cainjector:v1.20.2",
		"quay.io/jetstack/cert-manager-controller:v1.20.2",
		"quay.io/jetstack/cert-manager-webhook:v1.20.2",
	}
	for _, want := range wants {
		found := false
		for _, img := range Images {
			if img == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Images = %v; want to contain %q", Images, want)
		}
	}
}

// TestExecuteUsesServerSideApply guards Pitfall CERT-01: cert-manager manifest
// is 992 KB (>256 KB annotation limit), so the apply call MUST use --server-side.
// We run Execute against a successful FakeCmd queue, then inspect the FakeNode's
// Calls slice (every Command/CommandContext invocation is recorded as
// [command, args...]) and assert that at least one kubectl-apply invocation
// carries the --server-side flag.
//
// The cert-manager Execute() runs:
//   Step 1: kubectl apply --server-side -f -          (cert-manager.yaml)
//   Step 2..4: kubectl wait deployment/<name>          (cainjector, controller, webhook)
//   Step 5: kubectl apply -f - (selfsigned ClusterIssuer)
// Six FakeCmds is enough headroom; trailing exhausted entries return a default
// success FakeCmd (testutil.fake.go:135).
func TestExecuteUsesServerSideApply(t *testing.T) {
	t.Parallel()
	cmds := []*testutil.FakeCmd{
		{}, // Step 1: kubectl apply --server-side cert-manager.yaml
		{}, // Step 2: kubectl wait deployment/cert-manager-cainjector
		{}, // Step 3: kubectl wait deployment/cert-manager
		{}, // Step 4: kubectl wait deployment/cert-manager-webhook
		{}, // Step 5: kubectl apply selfsigned ClusterIssuer
		{}, // headroom
	}
	ctx, node := makeCtx(cmds)
	if err := NewAction().Execute(ctx); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Find the kubectl-apply invocation for the cert-manager.yaml manifest. It is
	// the apply call that does NOT mention "ClusterIssuer" or "selfsigned" in its
	// argv; equivalently, it is the first apply invocation.
	var manifestApply []string
	for _, call := range node.Calls {
		// call = [command, args...] — Command "kubectl" with first arg "apply"
		if len(call) >= 2 && call[0] == "kubectl" && call[1] == "apply" {
			manifestApply = call
			break
		}
	}
	if manifestApply == nil {
		t.Fatalf("no kubectl apply invocation captured; Calls=%v", node.Calls)
	}

	// Assert --server-side appears somewhere in the captured argv.
	hasServerSide := false
	for _, a := range manifestApply {
		if a == "--server-side" {
			hasServerSide = true
			break
		}
	}
	if !hasServerSide {
		t.Errorf("kubectl apply argv missing --server-side flag (Pitfall CERT-01); got %v", manifestApply)
	}
}
```

If `TestExecute` invokes `kubectl` differently (e.g. it constructs argv with `Command("kubectl", "apply", "--server-side", "-f", "-")` vs `CommandContext(...)`), inspect the existing test cases at lines 41-130 of `certmanager_test.go` and adjust the `call[0] == "kubectl"` predicate accordingly — the structural shape (first-element command, remaining-elements args) is fixed by `testutil/fake.go:nextCmd`.

Run:
```bash
go test ./pkg/cluster/internal/create/actions/installcertmanager/... -run "TestImagesPinsV1202|TestExecuteUsesServerSideApply" -v
```
TestImagesPinsV1202 MUST fail (Images still v1.16.3). TestExecuteUsesServerSideApply MUST pass at HEAD (the existing apply call already uses `--server-side` per RESEARCH §Server-side apply for large manifests, verified at `certmanager.go:79`); the test acts as a forward regression guard.

**Regression-guard sanity check (proves the test actually fires):** After confirming TestExecuteUsesServerSideApply passes at HEAD, temporarily edit `certmanager.go` line 79 to remove the `"--server-side"` literal from the argv and re-run:
```bash
go test ./pkg/cluster/internal/create/actions/installcertmanager/... -run TestExecuteUsesServerSideApply -v
```
The test MUST now FAIL with the "missing --server-side flag" error message. **Revert** `certmanager.go` (`git checkout HEAD -- pkg/cluster/internal/create/actions/installcertmanager/certmanager.go`) and re-run to confirm green again. This is a one-shot proof that the regression guard fires; record the FAIL output line in the SUMMARY.

Commit RED:
```bash
git add pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go
git commit -m "test(53-03): add failing test pinning cert-manager v1.20.2 + --server-side guard"
```

**STEP B — GREEN: bump Images slice + replace manifest YAML.**

1. Update `certmanager.go` Images slice (currently three v1.16.3 entries):
   ```go
   var Images = []string{
   	"quay.io/jetstack/cert-manager-cainjector:v1.20.2",
   	"quay.io/jetstack/cert-manager-controller:v1.20.2",
   	"quay.io/jetstack/cert-manager-webhook:v1.20.2",
   }
   ```

2. **DO NOT** touch the apply call (the `--server-side` flag is already there at line 79; TestExecuteUsesServerSideApply is the regression net).

3. Download upstream cert-manager v1.20.2 manifest (RESEARCH confirms 992 KB):
   ```bash
   curl -sL https://github.com/cert-manager/cert-manager/releases/download/v1.20.2/cert-manager.yaml \
     -o pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
   ```

4. Verify the manifest body:
   ```bash
   ls -lh pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
   # Expected: ~992 KB
   grep -c "cert-manager-controller:v1.20.2" pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
   # Expected: at least 1 match
   grep -c "cert-manager-cainjector:v1.20.2" pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
   grep -c "cert-manager-webhook:v1.20.2" pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
   ```

5. Run tests + build:
   ```bash
   go test ./pkg/cluster/internal/create/actions/installcertmanager/... -race
   go build ./...
   ```
   TestImagesPinsV1202 MUST now pass. TestExecuteUsesServerSideApply MUST still pass. Existing TestExecute MUST still pass.

**Do NOT commit yet.** Hold the working tree for Task 2 (live UAT).
  </action>
  <verify>
<automated>go test ./pkg/cluster/internal/create/actions/installcertmanager/... -race && go build ./... && grep -q "cert-manager-controller:v1.20.2" pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml && grep -q -- "--server-side" pkg/cluster/internal/create/actions/installcertmanager/certmanager.go</automated>
  </verify>
  <done>RED commit landed; working tree contains v1.20.2 bump (Images slice + 992 KB manifest); --server-side flag intact in certmanager.go; all relevant tests pass; regression-guard sanity check (temporary --server-side removal → test FAIL → revert → test pass) recorded in SUMMARY draft.</done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 2: Live UAT — ClusterIssuer + UID 65532 verification (HOLD GATE)</name>
  <what-built>cert-manager v1.20.2 bump in working tree (Images + 992 KB manifest). --server-side apply preserved. Now requires live verification before final commit.</what-built>
  <how-to-verify>
**This is the mandatory hold-trigger gate. Per CONTEXT.md hold criteria, ANY of the following triggers a hold at v1.16.3.**

1. Build and create a fresh cluster:
   ```bash
   go build -o bin/kinder ./
   ./bin/kinder create cluster --name uat-53-03
   ```

2. **Verify cert-manager pods are running:**
   ```bash
   kubectl -n cert-manager get pods
   ```
   Expected: cainjector, controller, webhook all `Running`. If any pod is `CrashLoopBackOff`, `Pending`, or `ImagePullBackOff` past 5 minutes, that IS a HOLD trigger.

3. **UID 65532 verification (this is the critical Pitfall CERT-03 check):**
   ```bash
   for pod in $(kubectl -n cert-manager get pods -o jsonpath='{.items[*].metadata.name}'); do
     uid=$(kubectl -n cert-manager get pod $pod -o jsonpath='{.spec.containers[0].securityContext.runAsUser}')
     echo "$pod -> runAsUser=$uid"
   done
   ```
   Every line MUST report `runAsUser=65532`. ANY pod reporting a different UID (1000, empty, etc.) IS a HOLD trigger — note the upstream changed default may not have applied as expected.

4. **ClusterIssuer + Certificate smoke:**
   ```bash
   cat <<'EOF' | kubectl apply -f -
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: uat-selfsigned
   spec:
     selfSigned: {}
   ---
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: uat-test-cert
     namespace: default
   spec:
     secretName: uat-test-cert-secret
     duration: 24h
     issuerRef:
       name: uat-selfsigned
       kind: ClusterIssuer
     commonName: uat-test
     dnsNames:
       - uat-test.local
   EOF

   # Wait up to 60s for the Certificate to become Ready:
   kubectl wait --for=condition=Ready certificate/uat-test-cert --timeout=60s

   # Inspect:
   kubectl describe certificate uat-test-cert
   kubectl get secret uat-test-cert-secret -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -noout -subject
   ```
   `kubectl wait` MUST succeed within 60s. The decoded cert MUST show `subject=CN = uat-test`. ANY failure (timeout, no Secret, parse error) IS a HOLD trigger.

5. **Verify webhook startup did not regress (Pitfall CERT-02):** check `kubectl describe certificate` does not show repeated webhook timeouts.

6. Tear down:
   ```bash
   kubectl delete certificate uat-test-cert
   kubectl delete clusterissuer uat-selfsigned
   ./bin/kinder delete cluster --name uat-53-03
   ```

**Decision tree:**
- All four checks pass → reply `bump`.
- ANY check fails → reply `hold` with paste of the failing output.
  </how-to-verify>
  <resume-signal>Reply `bump` if all four live checks pass; reply `hold` (with failure reason) to stay at v1.16.3.</resume-signal>
</task>

<task type="auto">
  <name>Task 3: Final commit — GREEN bump + addon doc :::caution callout + CHANGELOG, OR hold note + CHANGELOG hold note</name>
  <files>
    pkg/cluster/internal/create/actions/installcertmanager/certmanager.go
    pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go
    pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
    kinder-site/src/content/docs/addons/cert-manager.md
    kinder-site/src/content/docs/changelog.md
  </files>
  <action>
**Branch on Task 2 resume signal:**

---

**PATH A: bump (UAT passed) — commit GREEN + addon doc + CHANGELOG stub.**

1. Update addon doc `kinder-site/src/content/docs/addons/cert-manager.md`:
   - Bump version mention near the top of the page (search for "v1.16.3" → "v1.20.2").
   - Add a `:::caution[Breaking change in v1.20]` Starlight aside (RESEARCH §Standard Stack confirms this is the existing pattern in addons/metallb.md):
     ```markdown
     :::caution[Breaking changes in v1.20]
     Two upstream-default changes affect users upgrading from earlier kinder versions:

     - **Container UID changed from 1000 to 65532.** cert-manager pods now run as the unprivileged UID `65532`. PVCs or mounted Secrets pre-populated with files owned by UID 1000 will be unreadable to the new pods. cert-manager itself does not use PVCs, but custom integrations that share volumes with cert-manager may break.
     - **`Certificate.spec.privateKey.rotationPolicy: Always` is now GA-mandatory.** v1.18.0 changed the default from `Never` to `Always`; v1.20 makes it required (cannot disable). Long-running Certificates without an explicit `rotationPolicy: Never` will see a NEW private key on next renewal. Set `rotationPolicy: Never` explicitly if you want the old behavior — kinder does not patch this for you.
     :::
     ```

2. Append CHANGELOG stub to `kinder-site/src/content/docs/changelog.md` under `## v2.4 — Hardening (in progress)`:
   ```markdown
   - **`cert-manager` bumped to v1.20.2** — `--server-side` apply preserved (manifest is 992 KB, exceeds 256 KB annotation limit). Live UAT verified self-signed ClusterIssuer issues a Certificate and pods run as UID `65532`. **Breaking changes:** container UID changed from `1000` to `65532` (Secret/PVC ownership impact); `Certificate.spec.privateKey.rotationPolicy: Always` is GA-mandatory (set `Never` explicitly to keep old behavior). See addon doc for details. (ADDON-03)
   ```

3. Commit GREEN:
   ```bash
   git add pkg/cluster/internal/create/actions/installcertmanager/certmanager.go \
           pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml \
           kinder-site/src/content/docs/addons/cert-manager.md \
           kinder-site/src/content/docs/changelog.md
   git commit -m "feat(53-03): bump cert-manager to v1.20.2 (UID 65532 + rotationPolicy disclosure)"
   ```

---

**PATH B: hold (UAT failed) — revert and document hold.**

1. Revert working tree:
   ```bash
   git checkout HEAD -- pkg/cluster/internal/create/actions/installcertmanager/certmanager.go \
                        pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml
   ```
2. Mark `TestImagesPinsV1202` as Skip with hold reason (preserves intent for next attempt) — keep the original assertion body intact below the `t.Skip` call:
   ```go
   func TestImagesPinsV1202(t *testing.T) {
   	t.Skip("HOLD per 53-03 SUMMARY: cert-manager v1.20.2 UAT failed — re-enable when retried")
   	t.Parallel()
   	wants := []string{
   		"quay.io/jetstack/cert-manager-cainjector:v1.20.2",
   		"quay.io/jetstack/cert-manager-controller:v1.20.2",
   		"quay.io/jetstack/cert-manager-webhook:v1.20.2",
   	}
   	// ... (rest of original assertion preserved verbatim)
   }
   ```
   Leave `TestExecuteUsesServerSideApply` as-is (it's a permanent guard regardless of cert-manager version — no Skip).
3. Append CHANGELOG hold note under v2.4 H2:
   ```markdown
   - **`cert-manager` HELD at v1.16.3** — v1.20.2 live UAT failed: <one-line reason from Task 2 paste>. `--server-side` apply pattern preserved. See `.planning/phases/53-addon-version-audit-bumps-sync-05/53-03-SUMMARY.md`. (ADDON-03 — held)
   ```
4. Commit HOLD:
   ```bash
   git add pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go \
           kinder-site/src/content/docs/changelog.md
   git commit -m "docs(53-03): hold cert-manager at v1.16.3 — v1.20.2 UAT failed"
   ```

---

In BOTH paths, write `.planning/phases/53-addon-version-audit-bumps-sync-05/53-03-SUMMARY.md` with the live UAT output (pod listing, UID jsonpath output, Certificate Ready event, openssl subject), the chosen path (A or B), commit SHAs, and a clear ADDON-03 status (delivered / held).
  </action>
  <verify>
<automated>git log --oneline -3 | head -3 && go test ./pkg/cluster/internal/create/actions/installcertmanager/... -race && go build ./... && grep -q -- "--server-side" pkg/cluster/internal/create/actions/installcertmanager/certmanager.go</automated>
  </verify>
  <done>Either (Path A) v1.20.2 bumped, GREEN commit landed, addon doc :::caution callout for UID 65532 + rotationPolicy added, CHANGELOG stub updated; or (Path B) v1.16.3 held, `TestImagesPinsV1202` neutralized via `t.Skip("HOLD per 53-03 SUMMARY: ...")` as the first statement (assertion body intact below), `TestExecuteUsesServerSideApply` left active, CHANGELOG hold note landed. Either way: --server-side flag preserved (TestExecuteUsesServerSideApply still passes); SUMMARY.md captures live UAT output; ADDON-03 has a definitive disposition.</done>
</task>

</tasks>

<verification>
- `go test ./pkg/cluster/internal/create/actions/installcertmanager/... -race` exits 0.
- `go build ./...` exits 0.
- `grep -- "--server-side" pkg/cluster/internal/create/actions/installcertmanager/certmanager.go` returns at least one match (regression guard).
- 53-03-SUMMARY.md contains the live UAT transcript: pod listing, UID jsonpath output (must show 65532), Certificate Ready confirmation, openssl subject decode, AND the regression-guard sanity-check FAIL line from Task 1 STEP A.
- CHANGELOG has either an ADDON-03 bump line OR an ADDON-03 hold line under the v2.4 H2.
- If Path A: addon doc has `:::caution[Breaking changes in v1.20]` block mentioning both UID 65532 and `rotationPolicy: Always`.
- If Path B: addon doc unchanged (still says v1.16.3); `TestImagesPinsV1202` body retains both the `t.Skip("HOLD per 53-03 SUMMARY: ...")` first-statement and the original `wants` assertion below it.
</verification>

<success_criteria>
SC3 (Phase 53): `kinder create cluster` installs cert-manager v1.20.2 with `--server-side` apply; `kubectl get crd certificates.cert-manager.io -o jsonpath='{.spec.versions[0].name}'` returns the v1.20.2 API version; self-signed ClusterIssuer issues a certificate with the new UID (65532).

ADDON-03: cert-manager v1.20.2 with `--server-side`; UID change (1000→65532); ClusterIssuer smoke verified live.
</success_criteria>

<output>
`.planning/phases/53-addon-version-audit-bumps-sync-05/53-03-SUMMARY.md` records:
- Path taken (A: bump or B: hold)
- Live UAT transcripts: cert-manager pod listing, runAsUser jsonpath output for each pod, Certificate apply + Ready event, openssl subject decode of the issued cert
- Task 1 STEP A regression-guard sanity-check transcript (one-shot proof that removing `--server-side` triggers test FAIL, then revert restores PASS)
- Commits landed
- ADDON-03 status: delivered at v1.20.2 OR held at v1.16.3 with reason
- Explicit note: "UID is 65532 (CONTEXT.md decision bullet had typo 65632; REQUIREMENTS.md and upstream release notes are authoritative at 65532)"
</output>
