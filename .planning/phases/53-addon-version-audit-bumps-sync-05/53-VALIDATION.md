---
phase: 53
phase_name: "Addon Version Audit, Bumps & SYNC-05"
nyquist_validation: enabled
source: "Extracted from 53-RESEARCH.md §Validation Architecture (lines 670–715) + per-plan must_haves"
generated: 2026-05-10
---

# Phase 53 — Validation Plan

> Test map for the Phase 53 addon bumps + SYNC-05 probe + offline-readiness consolidation. Listed test files annotated `❌ Wave 0` do not exist yet — each owning plan creates them as the RED commit of its TDD cycle. Manual UAT rows are CONTEXT-locked (see 53-CONTEXT.md "UAT depth per addon") and cannot be fully automated at plan time.

**Addon image count baseline (verified live in `pkg/internal/doctor/offlinereadiness.go`):**

| Stage | Count | Test name |
|-------|-------|-----------|
| Pre-Phase-53 | 14 | `TestAllAddonImages_CountMatchesExpected` (`expected = 14`) |
| After Plan 53-07 | 14 | `TestAllAddonImages_CountMatchesExpected` unchanged (only tags shift; Pitfall ALL-02) |

**SYNC-05 default node image:** `pkg/apis/config/defaults/image.go` is changed by 53-00 only on Outcome A (image published). The default is NOT in `allAddonImages`.

---

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `t.Parallel()` |
| Config file | none (Go convention) |
| Quick run command (per addon) | `go test ./pkg/cluster/internal/create/actions/installX/... -race -v` |
| Full suite command | `go test ./pkg/cluster/internal/create/actions/... ./pkg/internal/doctor/... ./pkg/apis/config/defaults/... -race` |
| Build gate | `go build ./...` |
| Static analysis | `go vet ./...` |
| Phase gate | Full suite green + `go build ./...` + `go vet ./...` before `/gsd-verify-work` |

---

## Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | Owning Plan | File Exists? |
|--------|----------|-----------|-------------------|-------------|--------------|
| SYNC-05 | (Outcome A only) Default image pinned to `kindest/node:v1.36.x` with `@sha256:` digest | unit | `go test ./pkg/apis/config/defaults/... -run TestDefaultImageIsKubernetes136 -v` | 53-00 | ❌ Wave 0 (created on Outcome A; mirrors plan 51-04 RED template) |
| ADDON-01 | local-path-provisioner v0.0.36 image pinned in `Images` slice | unit | `go test ./pkg/cluster/internal/create/actions/installlocalpath/... -run TestImagesPinsV0036 -v` | 53-01 | ❌ Wave 0 |
| ADDON-01 | `busybox:1.37.0` still pinned in vendored manifest | unit | `go test ./pkg/cluster/internal/create/actions/installlocalpath/... -run TestManifestPinsBusybox -v` | 53-01 | ❌ Wave 0 |
| ADDON-01 | local-path StorageClass has `is-default-class` annotation | unit | `go test ./pkg/cluster/internal/create/actions/installlocalpath/... -run TestStorageClassIsDefault -v` | 53-01 | ❌ Wave 0 |
| ADDON-01 | Live PVC dynamic-provisioning smoke (`kubectl create pvc` binds, pod mounts) | manual | UAT script in 53-01 SUMMARY | 53-01 | manual-only (CONTEXT-locked) |
| ADDON-02 | Headlamp v0.42.0 image pinned in `Images` slice | unit | `go test ./pkg/cluster/internal/create/actions/installdashboard/... -run TestImagesPinsHeadlampV042 -v` | 53-02 | ❌ Wave 0 |
| ADDON-02 | Token-print flow still works (FakeCmd queue test) | unit | `go test ./pkg/cluster/internal/create/actions/installdashboard/... -run TestExecute -v` | 53-02 | ✅ exists at `dashboard_test.go` |
| ADDON-02 | Live token smoke (`kubectl auth can-i` + curl UI returns 200) | manual | UAT in 53-02 Task 2 (HOLD GATE) | 53-02 | manual-only (CONTEXT-locked) |
| ADDON-03 | cert-manager v1.20.2 images pinned (cainjector, controller, webhook) | unit | `go test ./pkg/cluster/internal/create/actions/installcertmanager/... -run TestImagesPinsV1202 -v` | 53-03 | ❌ Wave 0 |
| ADDON-03 | `--server-side` flag preserved in apply call (Pitfall CERT-01 regression guard) | unit | `go test ./pkg/cluster/internal/create/actions/installcertmanager/... -run TestExecuteUsesServerSideApply -v` | 53-03 | ❌ Wave 0 (extension of existing TestExecute via FakeNode `Calls` capture) |
| ADDON-03 | Live UID 65532 verification on running pod | manual | `kubectl get pod ... -o jsonpath='{.spec.containers[0].securityContext.runAsUser}'` in 53-03 Task 2 | 53-03 | manual-only (CONTEXT-locked) |
| ADDON-03 | Live ClusterIssuer + Certificate smoke | manual | `kubectl wait --for=condition=Ready certificate/...` in 53-03 Task 2 | 53-03 | manual-only (CONTEXT-locked) |
| ADDON-04 | EG v1.7.2 + ratelimit:05c08d03 images pinned | unit | `go test ./pkg/cluster/internal/create/actions/installenvoygw/... -run TestImagesPinsEGV172 -v` | 53-04 | ❌ Wave 0 |
| ADDON-04 | certgen Job name still `eg-gateway-helm-certgen` in embedded YAML | unit | `go test ./pkg/cluster/internal/create/actions/installenvoygw/... -run TestManifestContainsCertgenJobName -v` | 53-04 | ❌ Wave 0 |
| ADDON-04 | Bundled Gateway API CRD bundle-version is v1.4.1 | unit | `go test ./pkg/cluster/internal/create/actions/installenvoygw/... -run TestManifestPinsGatewayAPIBundleV141 -v` | 53-04 | ❌ Wave 0 |
| ADDON-04 | Live HTTPRoute end-to-end smoke (curl through gateway returns 200) | manual | UAT in 53-04 Task 2 (HOLD GATE) | 53-04 | manual-only (CONTEXT-locked) |
| ADDON (MetalLB) | MetalLB pinned at v0.15.3; upstream-latest verification | mixed | `go test ./pkg/cluster/internal/create/actions/installmetallb/...` + curl GitHub releases API | 53-05 | source tests exist; upstream check is plan-time |
| ADDON (Metrics Server) | Metrics Server pinned at v0.8.1; upstream-latest verification | mixed | `go test ./pkg/cluster/internal/create/actions/installmetricsserver/...` + curl GitHub releases API | 53-06 | source tests exist; upstream check is plan-time |
| ADDON-05 | offlinereadiness `allAddonImages` reflects all delivered tags; count remains 14 | unit | `go test ./pkg/internal/doctor/... -run TestAllAddonImages` | 53-07 | ✅ exists at `offlinereadiness_test.go:118` (`expected = 14`) |
| SC1 (second clause) | `kinder doctor offline-readiness` does not warn on a fresh default cluster | manual | live `kinder doctor offline-readiness` against fresh cluster in 53-07 final task | 53-07 | manual-only (live cluster required) |

**Key links covered:**

- `installX/X.go::Images` ↔ `pkg/internal/doctor/offlinereadiness.go::allAddonImages` (verified by 53-07 cross-check grep + `TestAllAddonImages_CountMatchesExpected`).
- `installcertmanager/certmanager.go::Execute` → `kubectl apply --server-side` (verified by `TestExecuteUsesServerSideApply` capturing `FakeNode.Calls` and asserting `--server-side` is present in the kubectl-apply argv slice).
- `installdashboard/dashboard.go::Execute` → `kinder-dashboard-token` Secret read flow (verified by existing `TestExecute` cases and preservation grep in 53-02 Task 1 step 5).
- `installenvoygw/envoygw.go::certgenWait` → `Job/eg-gateway-helm-certgen` (verified by `TestManifestContainsCertgenJobName` greppping the embedded manifest body).

---

## Sampling Rate

- **Per task commit:** `go test ./pkg/cluster/internal/create/actions/installX/... -race` (the addon under edit) + `go build ./...`
- **Per wave merge (per sub-plan close):** `go test ./pkg/cluster/internal/create/actions/... ./pkg/internal/doctor/... -race`
- **Phase gate (before 53-07 merge):** Full suite + `go build ./...` + `go vet ./...`

---

## Wave 0 Gaps (test files / cases to be created during execution)

These are RED-commit responsibilities of each plan, not pre-Wave-0 work — they ship in the same commit-pair as their GREEN counterparts.

- [ ] `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go` — extend with `TestImagesPinsV0036`, `TestManifestPinsBusybox`, `TestStorageClassIsDefault` (Plan 53-01)
- [ ] `pkg/cluster/internal/create/actions/installdashboard/dashboard_test.go` — extend with `TestImagesPinsHeadlampV042` (Plan 53-02)
- [ ] `pkg/cluster/internal/create/actions/installcertmanager/certmanager_test.go` — extend with `TestImagesPinsV1202`, `TestExecuteUsesServerSideApply` (the latter inspects `FakeNode.Calls` for `--server-side` in the kubectl-apply argv) (Plan 53-03)
- [ ] `pkg/cluster/internal/create/actions/installenvoygw/envoygw_test.go` — extend with `TestImagesPinsEGV172`, `TestManifestContainsCertgenJobName`, `TestManifestPinsGatewayAPIBundleV141` (Plan 53-04)
- [ ] `pkg/apis/config/defaults/image_test.go` — created by Plan 53-00 GREEN if Outcome A, with `TestDefaultImageIsKubernetes136` per plan 51-04 RED template

---

## Manual UATs (Cannot Be Automated at Plan Time)

These UATs are CONTEXT-locked (see 53-CONTEXT.md "UAT depth per addon"). They are HOLD GATES — failure of any check triggers the addon-level hold path documented in each plan.

### UAT-1 — local-path-provisioner v0.0.36 PVC smoke (Plan 53-01)

**Why this is manual:** Dynamic-provisioning + host-path mount cannot be exercised by unit tests; requires a fresh kinder cluster with a real CSI controller.

**Steps:**

1. Build the binary: `go build -o bin/kinder ./`
2. Create a fresh cluster: `./bin/kinder create cluster --name uat-53-01`
3. Verify pods: `kubectl -n local-path-storage get pods` — all `Running`.
4. Verify image digest: `kubectl -n local-path-storage get pod -o jsonpath='{.items[*].spec.containers[*].image}'` mentions `local-path-provisioner:v0.0.36`.
5. Create a PVC + Pod that mounts it; wait for Bound:

   ```bash
   cat <<'EOF' | kubectl apply -f -
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata: { name: uat-pvc }
   spec:
     accessModes: [ReadWriteOnce]
     resources: { requests: { storage: 10Mi } }
   ---
   apiVersion: v1
   kind: Pod
   metadata: { name: uat-pod }
   spec:
     containers:
     - { name: w, image: busybox, command: ["sh","-c","echo ok > /data/x; sleep 3600"], volumeMounts: [{ name: d, mountPath: /data }] }
     volumes:
     - { name: d, persistentVolumeClaim: { claimName: uat-pvc } }
   EOF
   kubectl wait --for=condition=Ready pod/uat-pod --timeout=60s
   kubectl exec uat-pod -- cat /data/x   # expect: ok
   ```

6. Verify default StorageClass: `kubectl get sc -o jsonpath='{.items[?(@.metadata.annotations.storageclass\.kubernetes\.io/is-default-class=="true")].metadata.name}'` returns the local-path SC.
7. Tear down: `./bin/kinder delete cluster --name uat-53-01`.

**PASS:** PVC bound, pod reads `/data/x`, default-class annotation present.
**FAIL:** any step fails — capture output and trigger the 53-01 hold path.

### UAT-2 — Headlamp v0.42.0 token auth smoke (Plan 53-02 Task 2)

**Why this is manual:** Token-print → kubectl auth → UI HTTP 200 chain spans three live subsystems (kinder action, kube-apiserver authz, Headlamp pod). CONTEXT-locked.

**Steps:** Already detailed verbatim in 53-02 Task 2 (`<how-to-verify>` block). Summary:

1. `./bin/kinder create cluster --name uat-53-02` → captures printed token.
2. `kubectl auth can-i --token=$TOKEN get pods --all-namespaces` returns `yes`.
3. `kubectl port-forward -n kube-system svc/headlamp 4466:4466` + `curl -H "Authorization: Bearer $TOKEN" http://localhost:4466/` returns `200`.
4. Verify SA + Secret resources (`kinder-dashboard`, `kinder-dashboard-token` in `kube-system`) still resolve.
5. Tear down.

**PASS:** all three checks succeed → Path A bump.
**FAIL:** any check fails → Path B hold; CHANGELOG + RED test `t.Skip` per 53-02 Task 3.

### UAT-3 — cert-manager v1.20.2 ClusterIssuer + UID 65532 (Plan 53-03 Task 2)

**Why this is manual:** UID change (1000 → 65532) is a runtime-only property; ClusterIssuer issuance traverses controller, webhook, cainjector. CONTEXT-locked.

**Steps:** Already detailed verbatim in 53-03 Task 2 (`<how-to-verify>` block). Summary:

1. Fresh cluster.
2. `kubectl -n cert-manager get pods` → all `Running`.
3. UID assertion per pod: `kubectl get pod -o jsonpath='{.spec.containers[0].securityContext.runAsUser}'` returns `65532` for every cert-manager pod.
4. Apply self-signed ClusterIssuer + Certificate; `kubectl wait --for=condition=Ready certificate/uat-test-cert --timeout=60s` succeeds.
5. Decode issued cert: `openssl x509 -noout -subject` shows `CN = uat-test`.
6. Tear down.

**PASS:** all four checks succeed → Path A bump.
**FAIL:** any check fails → Path B hold; ADDON-03 disposition recorded as held.

### UAT-4 — Envoy Gateway v1.7.2 HTTPRoute end-to-end (Plan 53-04 Task 2)

**Why this is manual:** Gateway API HTTPRoute traffic flow requires a live cluster + EG controller + a backend pod. Roadmap-mandated.

**Steps:** Already detailed verbatim in 53-04 Task 2 (`<how-to-verify>` block). Summary:

1. Fresh cluster + EG installed.
2. EG pods Ready; Gateway resource `status.conditions[?(@.type=="Programmed")].status` == `True`.
3. Apply HTTPRoute + backend; `curl -H "Host: ..." http://<gateway-ip>/<path>` returns `200`.
4. Verify CRD bundle-version annotation matches v1.4.1.
5. Tear down.

**PASS:** all checks → Path A bump.
**FAIL:** any check → Path B hold; CHANGELOG hold note documents the staged 1.3→1.5→1.7 retreat as the next attempt.

### UAT-5 — `kinder doctor offline-readiness` no-warn on fresh cluster (Plan 53-07 final task — covers SC1 second clause)

**Why this is manual:** Verifies that the consolidated `allAddonImages` slice matches a real running cluster's pulled images — `go test` only checks string equality between source files, not what a live cluster actually pulls. SC1 explicitly requires `kinder doctor offline-readiness` to NOT warn on a fresh default cluster.

**Steps:**

1. Build: `go build -o bin/kinder ./`
2. Fresh cluster: `./bin/kinder create cluster --name uat-53-07`
3. Run the doctor: `./bin/kinder doctor offline-readiness 2>&1 | tee /tmp/53-07-doctor.out`
4. Assert no `warn` or `missing` lines:

   ```bash
   if grep -E '^(warn|missing)\b' /tmp/53-07-doctor.out; then
     echo "FAIL: offline-readiness reports warn/missing lines"; exit 1
   fi
   ```

5. Tear down: `./bin/kinder delete cluster --name uat-53-07`.

**PASS:** doctor exits 0 with no `warn`/`missing` lines → SC1 satisfied.
**FAIL:** doctor reports any addon image as missing/unpulled → revisit 53-07 Task 1 (a tag in `allAddonImages` does not match the live image actually pulled by the addon's `Execute`).

---

## Phase-Wide Acceptance Tests

| Acceptance Truth | Test Type | Harness | Automated Command |
|------------------|-----------|---------|-------------------|
| `go build ./...` succeeds with all eight plans landed | build | full module build | `go build ./...` |
| `go vet ./...` clean | static | vet | `go vet ./...` |
| Full unit suite passes with `-race` | unit | aggregate | `go test ./pkg/cluster/internal/create/actions/... ./pkg/internal/doctor/... ./pkg/apis/config/defaults/... -race` |
| `TestAllAddonImages_CountMatchesExpected` passes (count remains 14; Pitfall ALL-02) | unit | constant + slice cross-check | `go test ./pkg/internal/doctor/... -run TestAllAddonImages_CountMatchesExpected -v` |
| Each `installX/X.go::Images` tag mirrors `allAddonImages` (cross-check) | static | grep | manual grep audit per 53-07 Task 1 Step A |
| Three-tier disclosure shipped (CHANGELOG + addon docs + v2.4 release-notes draft) | review | file existence | `test -f kinder-site/src/content/docs/changelog.md && test -f .planning/release-notes-v2.4-draft.md` |
| Requirement coverage: every plan's `requirements:` field lists its ADDON/SYNC ID | static | frontmatter validation | `for f in .planning/phases/53-addon-version-audit-bumps-sync-05/53-0?-PLAN.md; do gsd-sdk query frontmatter.validate "$f" --schema plan; done` |

---

## Coverage Audit

Every truth in every plan's `must_haves` block has a row in this document. Every key link has at least one test row that exercises it. Every foundational assumption that cannot be unit-verified is listed under Manual UATs with explicit PASS/FAIL criteria and the corresponding hold-path reference.

If during execution any plan adds a truth or removes one, this VALIDATION.md MUST be updated in the same commit so the map stays in sync.
