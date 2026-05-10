---
phase: 53-addon-version-audit-bumps-sync-05
plan: 04
type: execute
wave: 5
depends_on: ["53-03"]
files_modified:
  - pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
  - pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go
  - pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
  - kinder-site/src/content/docs/addons/envoy-gateway.md
  - kinder-site/src/content/docs/changelog.md
autonomous: false
requirements: [ADDON-04]

must_haves:
  truths:
    - "Images slice in installenvoygw/envoygw.go pins envoyproxy/gateway:v1.7.2 AND docker.io/envoyproxy/ratelimit:05c08d03"
    - "Embedded manifests/install.yaml is upstream EG v1.7.2 install.yaml (~3.3 MB)"
    - "Embedded manifest contains job name eg-gateway-helm-certgen (verified unchanged in v1.7.2)"
    - "Embedded manifest carries Gateway API CRDs at bundle-version v1.4.1 (was v1.2.1 in EG v1.3.1)"
    - "TestImagesPinsEGV172, TestManifestContainsCertgenJobName, TestManifestPinsGatewayAPIBundleV141 all pass"
    - "Live UAT confirms HTTPRoute end-to-end traffic returns 200 on a fresh cluster"
    - "If live UAT fails (HTTPRoute non-200, certgen job rename, CRD breaking change), addon held at v1.3.1"
    - "Addon doc has :::caution[Major upgrade] callout disclosing v1.3→v1.7 jump and Gateway API CRD bump v1.2.1→v1.4.1"
    - "Atomic CHANGELOG stub for ADDON-04 (or hold) landed with the closing commit"
  artifacts:
    - path: "pkg/cluster/internal/create/actions/installenvoygw/envoygw.go"
      provides: "Images slice pinning EG v1.7.2 + ratelimit:05c08d03"
      contains: "envoyproxy/gateway:v1.7.2"
    - path: "pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go"
      provides: "TestImagesPinsEGV172, TestManifestContainsCertgenJobName, TestManifestPinsGatewayAPIBundleV141"
      contains: "TestManifestPinsGatewayAPIBundleV141"
    - path: "pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml"
      provides: "Upstream EG v1.7.2 install.yaml with bundled Gateway API v1.4.1 CRDs"
      contains: "envoyproxy/gateway:v1.7.2"
  key_links:
    - from: "envoygw.go waitForCertgenJob() call"
      to: "Job named eg-gateway-helm-certgen in embedded manifest"
      via: "kubectl wait --for=condition=Complete job/eg-gateway-helm-certgen"
      pattern: "eg-gateway-helm-certgen"
    - from: "Live HTTPRoute UAT"
      to: "Gateway API CRD bundle v1.4.1"
      via: "GatewayClass + Gateway + HTTPRoute admission + curl 200"
      pattern: "gateway.networking.k8s.io/bundle-version"
---

<objective>
Bump Envoy Gateway from v1.3.1 to v1.7.2 (single-jump per CONTEXT D-lock; user overrode the staged v1.3→v1.5→v1.7 recommendation). The bundled Gateway API CRDs jump v1.2.1 → v1.4.1 in-band (no separate CRD pin — replace the whole `install.yaml` as a unit per Pitfall EG-01).

Live UAT must confirm an HTTPRoute admits and routes end-to-end traffic. The hold criteria (HTTPRoute non-200, certgen job rename, CRD breaking change) are the safety net for the single-jump strategy.

Purpose: ADDON-04 — Envoy Gateway current stable. The certgen Job name `eg-gateway-helm-certgen` is verified unchanged in v1.7.2 (RESEARCH §Pitfall EG-02 line 52747); the kinder action's `kubectl wait` pattern requires no source change beyond Images.

Output: Either (a) RED+GREEN+post-UAT commit pair landing the bump, addon :::caution callout, and CHANGELOG stub, or (b) a single hold commit + CHANGELOG hold note keeping v1.3.1 in place.
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

@pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
@pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go
@pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
@kinder-site/src/content/docs/addons/envoy-gateway.md
@kinder-site/src/content/docs/changelog.md
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: RED + GREEN — pin EG v1.7.2 + ratelimit + assert certgen + Gateway API CRD bundle v1.4.1</name>
  <files>
    pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
    pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go
    pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
  </files>
  <behavior>
    - TestImagesPinsEGV172: Images slice contains "envoyproxy/gateway:v1.7.2" AND "docker.io/envoyproxy/ratelimit:05c08d03"
    - TestManifestContainsCertgenJobName: embedded manifest body contains "name: eg-gateway-helm-certgen"
    - TestManifestPinsGatewayAPIBundleV141: embedded manifest body contains the bundle-version annotation pointing at v1.4.1
    - All three fail at HEAD (Images is v1.3.1 + ratelimit:ae4cee11; manifest still has bundle-version v1.2.1)
  </behavior>
  <action>
**STEP A — RED tests.**

Open `pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go`. Append:

```go
// TestImagesPinsEGV172 pins both EG images to the v1.7.2 set (ADDON-04).
func TestImagesPinsEGV172(t *testing.T) {
	t.Parallel()
	wants := []string{
		"envoyproxy/gateway:v1.7.2",
		"docker.io/envoyproxy/ratelimit:05c08d03",
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

// TestManifestContainsCertgenJobName guards Pitfall EG-02: the kinder action
// waits on Job/eg-gateway-helm-certgen by name. If upstream renames it, the
// wait times out. Researcher confirmed v1.7.2 still ships this job name
// (line 52747 of upstream install.yaml); this test is the forward regression net.
func TestManifestContainsCertgenJobName(t *testing.T) {
	t.Parallel()
	const want = "name: eg-gateway-helm-certgen"
	if !strings.Contains(envoyGatewayManifest, want) {
		t.Errorf("envoyGatewayManifest missing %q (kinder action waits on this Job by hardcoded name)", want)
	}
}

// TestManifestPinsGatewayAPIBundleV141 pins the bundled Gateway API CRD
// version to v1.4.1 (was v1.2.1 in EG v1.3.1). The bundle-version annotation
// is set on the Gateway API CRDs themselves, distributed inside EG's install.yaml.
func TestManifestPinsGatewayAPIBundleV141(t *testing.T) {
	t.Parallel()
	const want = "gateway.networking.k8s.io/bundle-version: v1.4.1"
	if !strings.Contains(envoyGatewayManifest, want) {
		t.Errorf("envoyGatewayManifest missing %q (Gateway API CRDs must bundle v1.4.1 in EG v1.7.2)", want)
	}
}
```

(Confirm the exact embedded-manifest variable name by reading `envoygw.go` — it may be `envoyGatewayManifest`, `installManifest`, or similar. Adjust the test variable references to match.)

Add `"strings"` import if missing.

Run:
```bash
go test ./pkg/cluster/internal/create/actions/installenvoygw/... -run "TestImagesPinsEGV172|TestManifestContainsCertgenJobName|TestManifestPinsGatewayAPIBundleV141" -v
```
All three MUST fail at HEAD (Images is v1.3.1 + ratelimit:ae4cee11; manifest is v1.3.1 with bundle-version v1.2.1).

Commit RED:
```bash
git add pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go
git commit -m "test(53-04): add failing tests pinning Envoy Gateway v1.7.2 + Gateway API v1.4.1"
```

**STEP B — GREEN: bump Images slice + replace manifest as a unit.**

1. Update `envoygw.go` Images slice (currently `"docker.io/envoyproxy/ratelimit:ae4cee11"` and `"envoyproxy/gateway:v1.3.1"`):
   ```go
   var Images = []string{
   	"docker.io/envoyproxy/ratelimit:05c08d03",
   	"envoyproxy/gateway:v1.7.2",
   }
   ```

2. Replace the embedded manifest as a UNIT (Pitfall EG-01 — do NOT split CRDs from the rest):
   ```bash
   curl -sL https://github.com/envoyproxy/gateway/releases/download/v1.7.2/install.yaml \
     -o pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
   ```

3. Verify the manifest body:
   ```bash
   ls -lh pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
   # Expected: ~3.3 MB
   grep -c "envoyproxy/gateway:v1.7.2" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
   # Expected: at least 1
   grep -c "envoyproxy/ratelimit:05c08d03" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
   # Expected: at least 1
   grep -c "name: eg-gateway-helm-certgen" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
   # Expected: at least 1 (Pitfall EG-02 verification)
   grep "gateway.networking.k8s.io/bundle-version" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml | sort -u
   # Expected: gateway.networking.k8s.io/bundle-version: v1.4.1
   ```

4. Run tests + build:
   ```bash
   go test ./pkg/cluster/internal/create/actions/installenvoygw/... -race
   go build ./...
   ```
   All three new tests MUST now pass. Existing tests MUST still pass.

**Do NOT commit yet.** Hold the working tree for Task 2 (live HTTPRoute UAT).
  </action>
  <verify>
<automated>go test ./pkg/cluster/internal/create/actions/installenvoygw/... -race && go build ./... && grep -q "envoyproxy/gateway:v1.7.2" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml && grep -q "name: eg-gateway-helm-certgen" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml && grep -q "bundle-version: v1.4.1" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml</automated>
  </verify>
  <done>RED commit landed; working tree contains v1.7.2 bump (Images + 3.3 MB install.yaml); certgen job name unchanged; bundled Gateway API CRDs at v1.4.1; all unit tests pass.</done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 2: Live UAT — HTTPRoute end-to-end smoke (HOLD GATE)</name>
  <what-built>Envoy Gateway v1.7.2 bump in working tree (Images + 3.3 MB install.yaml). certgen Job name unchanged; Gateway API CRDs bundled at v1.4.1. Now requires live HTTPRoute end-to-end verification before final commit.</what-built>
  <how-to-verify>
**This is the mandatory hold-trigger gate (the safety net for the single-jump strategy). Per CONTEXT.md hold criteria, ANY of the following triggers a hold at v1.3.1.**

1. Build + create cluster:
   ```bash
   go build -o bin/kinder ./
   ./bin/kinder create cluster --name uat-53-04
   ```

2. **EG pods Ready check:**
   ```bash
   kubectl -n envoy-gateway-system get pods
   ```
   All EG controller + ratelimit + certgen Job pods must be `Running`/`Completed`. ANY pod stuck `CrashLoopBackOff`, `ImagePullBackOff`, or the certgen Job stuck `Pending` past 5 minutes IS a HOLD trigger (the latter would indicate Pitfall EG-02 — certgen Job rename — though research verified the name is unchanged).

3. **GatewayClass + Gateway acceptance:**
   ```bash
   kubectl get gatewayclass
   ```
   Look for the EG class status `Accepted=True`. Then create a Gateway:
   ```bash
   cat <<'EOF' | kubectl apply -f -
   apiVersion: gateway.networking.k8s.io/v1
   kind: Gateway
   metadata:
     name: uat-gw
     namespace: default
   spec:
     gatewayClassName: eg
     listeners:
       - name: http
         protocol: HTTP
         port: 80
   EOF

   kubectl wait --for=condition=Programmed gateway/uat-gw --timeout=60s
   ```
   The wait MUST succeed within 60s. Failure IS a HOLD trigger.

4. **HTTPRoute end-to-end traffic (THE SC4 acceptance test):**
   ```bash
   # Backend deployment + service:
   kubectl create deployment uat-backend --image=hashicorp/http-echo --port=5678 -- -text="UAT-OK"
   kubectl expose deployment uat-backend --port=5678

   # HTTPRoute:
   cat <<'EOF' | kubectl apply -f -
   apiVersion: gateway.networking.k8s.io/v1
   kind: HTTPRoute
   metadata:
     name: uat-route
   spec:
     parentRefs:
       - name: uat-gw
     rules:
       - matches:
           - path:
               type: PathPrefix
               value: /
         backendRefs:
           - name: uat-backend
             port: 5678
   EOF

   kubectl wait --for=condition=Accepted httproute/uat-route --timeout=30s
   ```
   The wait MUST succeed. Then send curl traffic through the gateway. Get the EG service address:
   ```bash
   GW_ADDR=$(kubectl get svc -n envoy-gateway-system -l gateway.envoyproxy.io/owning-gateway-name=uat-gw -o jsonpath='{.items[0].status.loadBalancer.ingress[0].ip}')
   curl -sS -o /dev/null -w "HTTP %{http_code}\nbody=%{size_download}\n" http://$GW_ADDR/
   curl -sS http://$GW_ADDR/
   ```
   Expected: `HTTP 200` AND response body `UAT-OK`. ANY non-200 or timeout IS a HOLD trigger.

5. Tear down:
   ```bash
   kubectl delete httproute uat-route
   kubectl delete gateway uat-gw
   kubectl delete deployment uat-backend
   kubectl delete svc uat-backend
   ./bin/kinder delete cluster --name uat-53-04
   ```

**Decision tree:**
- All four checks pass → reply `bump`.
- ANY check fails → reply `hold` with paste of the failing output.
  </how-to-verify>
  <resume-signal>Reply `bump` if all four live checks pass; reply `hold` (with failure reason) to stay at v1.3.1.</resume-signal>
</task>

<task type="auto">
  <name>Task 3: Final commit — GREEN bump + addon doc :::caution callout + CHANGELOG, OR hold note + CHANGELOG hold note</name>
  <files>
    pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
    pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go
    pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
    kinder-site/src/content/docs/addons/envoy-gateway.md
    kinder-site/src/content/docs/changelog.md
  </files>
  <action>
**Branch on Task 2 resume signal:**

---

**PATH A: bump (UAT passed) — commit GREEN + addon doc + CHANGELOG stub.**

1. Update addon doc `kinder-site/src/content/docs/addons/envoy-gateway.md`:
   - Bump version mention near the top of the page (search for "v1.3.1" → "v1.7.2").
   - Add `:::caution[Major upgrade]` callout:
     ```markdown
     :::caution[Major upgrade in v2.4 — Envoy Gateway v1.3 → v1.7]
     This is a two-major-version jump. Bundled Gateway API CRDs jump from `v1.2.1` to `v1.4.1`.

     - **Existing kinder clusters:** unaffected — addon versions are pinned at cluster-create time. Recreate the cluster to pick up v1.7.2.
     - **HTTPRoute compatibility:** the v1.4.1 Gateway API CRD bundle is backwards compatible with v1.4.x and most v1.2.x HTTPRoute manifests. Custom resources using deprecated v1alpha2 fields may need updates — see [upstream Gateway API release notes](https://gateway-api.sigs.k8s.io/release-notes/).
     - **No Helm migration needed:** kinder uses upstream's `install.yaml` directly via `kubectl apply`; the change is fully manifest-driven.
     :::
     ```

2. Append CHANGELOG stub to `kinder-site/src/content/docs/changelog.md` under `## v2.4 — Hardening (in progress)`:
   ```markdown
   - **`Envoy Gateway` bumped to v1.7.2** (single-jump from v1.3.1). Bundled Gateway API CRDs upgrade from `v1.2.1` to `v1.4.1` in-band. Live HTTPRoute end-to-end UAT verified traffic returns 200 through the gateway. `eg-gateway-helm-certgen` Job name unchanged (verified in upstream install.yaml). Ratelimit image bumped from `ae4cee11` to `05c08d03`. (ADDON-04)
   ```

3. Commit GREEN:
   ```bash
   git add pkg/cluster/internal/create/actions/installenvoygw/envoygw.go \
           pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml \
           kinder-site/src/content/docs/addons/envoy-gateway.md \
           kinder-site/src/content/docs/changelog.md
   git commit -m "feat(53-04): bump Envoy Gateway to v1.7.2 (Gateway API CRDs v1.4.1; HTTPRoute UAT verified)"
   ```

---

**PATH B: hold (UAT failed) — revert and document hold.**

> **Atomic-revert reminder (53-04 sits at the upper risk edge: 3.3 MB manifest swap + UAT in a single plan).** The revert in step 1 MUST atomically restore BOTH the Images slice in `envoygw.go` AND the 3.3 MB embedded `install.yaml` in the SAME `git checkout` invocation. Reverting one without the other leaves the working tree in a wedged state where the Images slice references EG v1.7.2 binaries but the embedded manifest still ships v1.3.1 (or vice versa) — a class of failure unique to this plan because the manifest is a vendored binary blob and not a code change. The single-command form below ensures atomicity.

1. Revert working tree (single atomic checkout — restores Images slice AND 3.3 MB manifest in one operation):
   ```bash
   git checkout HEAD -- pkg/cluster/internal/create/actions/installenvoygw/envoygw.go \
                        pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
   ```
   Verify both files are at HEAD before continuing:
   ```bash
   git diff --stat HEAD pkg/cluster/internal/create/actions/installenvoygw/envoygw.go \
                        pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml
   # Expected: empty output (no diff = both files at HEAD)
   grep -q "envoyproxy/gateway:v1.3.1" pkg/cluster/internal/create/actions/installenvoygw/envoygw.go \
     || { echo "FAIL: envoygw.go did not revert to v1.3.1"; exit 1; }
   grep -q "bundle-version: v1.2.1" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml \
     || { echo "FAIL: install.yaml did not revert to bundle-version v1.2.1"; exit 1; }
   ```
2. Mark all three RED tests as Skip with hold reason — preserve the assertion bodies verbatim below the `t.Skip` call (the Skip-with-body pattern keeps the tests as ready-made entry points for the next bump attempt; per the 53-02 / 53-03 precedent, never delete a held test):
   ```go
   func TestImagesPinsEGV172(t *testing.T) {
   	t.Skip("HOLD per 53-04 SUMMARY: Envoy Gateway v1.7.2 UAT failed — re-enable when retried")
   	t.Parallel()
   	wants := []string{
   		"envoyproxy/gateway:v1.7.2",
   		"docker.io/envoyproxy/ratelimit:05c08d03",
   	}
   	// ... (rest of original assertion body preserved verbatim)
   }
   func TestManifestContainsCertgenJobName(t *testing.T) {
   	t.Skip("HOLD per 53-04 SUMMARY: Envoy Gateway v1.7.2 UAT failed — re-enable when retried")
   	// ... (original assertion body preserved verbatim)
   }
   func TestManifestPinsGatewayAPIBundleV141(t *testing.T) {
   	t.Skip("HOLD per 53-04 SUMMARY: Envoy Gateway v1.7.2 UAT failed — re-enable when retried")
   	// ... (original assertion body preserved verbatim)
   }
   ```
3. Append CHANGELOG hold note:
   ```markdown
   - **`Envoy Gateway` HELD at v1.3.1** — v1.7.2 live UAT failed: <one-line reason from Task 2 paste>. Single-jump strategy retreats; staged 1.3→1.5→1.7 (deferred-idea fallback per 53-CONTEXT.md) is the documented next attempt. See `.planning/phases/53-addon-version-audit-bumps-sync-05/53-04-SUMMARY.md`. (ADDON-04 — held)
   ```
4. Commit HOLD:
   ```bash
   git add pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go \
           kinder-site/src/content/docs/changelog.md
   git commit -m "docs(53-04): hold Envoy Gateway at v1.3.1 — v1.7.2 UAT failed"
   ```

---

In BOTH paths, write `.planning/phases/53-addon-version-audit-bumps-sync-05/53-04-SUMMARY.md` with the live UAT output (pod listing, Gateway Programmed event, HTTPRoute Accepted event, curl HTTP code + body), the chosen path, commit SHAs, and ADDON-04 disposition.
  </action>
  <verify>
<automated>git log --oneline -3 | head -3 && go test ./pkg/cluster/internal/create/actions/installenvoygw/... -race && go build ./...</automated>
  </verify>
  <done>Either (Path A) v1.7.2 bumped, GREEN commit landed, addon doc :::caution callout for v1.3→v1.7 jump + Gateway API v1.2.1→v1.4.1 added, CHANGELOG stub updated; or (Path B) v1.3.1 held — single atomic `git checkout HEAD --` restored BOTH `envoygw.go` Images slice AND the 3.3 MB `install.yaml`, all three RED tests neutralized via `t.Skip("HOLD per 53-04 SUMMARY: ...")` as the first statement (assertion bodies intact below), CHANGELOG hold note landed (mentioning the deferred staged-bump fallback). Either way: SUMMARY.md captures live HTTPRoute UAT output; ADDON-04 has a definitive disposition.</done>
</task>

</tasks>

<verification>
- `go test ./pkg/cluster/internal/create/actions/installenvoygw/... -race` exits 0.
- `go build ./...` exits 0.
- 53-04-SUMMARY.md contains the live UAT transcript: EG pod listing, certgen Job Complete event, GatewayClass status, Gateway Programmed event, HTTPRoute Accepted event, curl HTTP code + body.
- CHANGELOG has either an ADDON-04 bump line OR an ADDON-04 hold line under the v2.4 H2.
- If Path A: addon doc has `:::caution[Major upgrade ...]` block mentioning both the EG version jump and the Gateway API v1.2.1→v1.4.1 CRD bump; `grep "bundle-version: v1.4.1" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml` returns at least one match; `grep "name: eg-gateway-helm-certgen" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml` returns at least one match.
- If Path B: addon doc unchanged; all three RED tests retain `t.Skip("HOLD per 53-04 SUMMARY: ...")` as the first statement with the assertion body intact below; `grep "envoyproxy/gateway:v1.3.1" pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` AND `grep "bundle-version: v1.2.1" pkg/cluster/internal/create/actions/installenvoygw/manifests/install.yaml` BOTH return at least one match (atomic-revert verification).
</verification>

<success_criteria>
SC4 (Phase 53): `kinder create cluster` installs Envoy Gateway v1.7.2; an HTTPRoute routes traffic end-to-end; the `eg-gateway-helm-certgen` job name is verified in the v1.7.2 install.yaml before commit.

ADDON-04: Envoy Gateway v1.7.2 (jumps two majors); HTTPRoute UAT; certgen job name verification; Gateway API CRD v1.4.1 lock.
</success_criteria>

<output>
`.planning/phases/53-addon-version-audit-bumps-sync-05/53-04-SUMMARY.md` records:
- Path taken (A: bump or B: hold)
- Live UAT transcripts: EG pod listing, certgen Job Complete event, GatewayClass acceptance, Gateway Programmed event, HTTPRoute Accepted event, curl HTTP code + body
- Bundled Gateway API CRD bundle-version observed (must be v1.4.1 if Path A)
- Commits landed
- ADDON-04 status: delivered at v1.7.2 OR held at v1.3.1 with reason
- If Path B: explicit confirmation that the atomic revert restored BOTH `envoygw.go` and the 3.3 MB `install.yaml` in one `git checkout` invocation (paste `git diff --stat HEAD <those two paths>` showing empty diff after revert) AND the staged 1.3→1.5→1.7 deferred-idea fallback (CONTEXT.md `## Deferred Ideas`) is referenced as the next attempt
</output>
