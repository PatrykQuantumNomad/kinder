---
phase: 53-addon-version-audit-bumps-sync-05
plan: 01
type: execute
wave: 2
depends_on: ["53-00"]
files_modified:
  - pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go
  - pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go
  - pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml
  - kinder-site/src/content/docs/addons/local-path-provisioner.md
  - kinder-site/src/content/docs/changelog.md
autonomous: true
requirements: [ADDON-01]

must_haves:
  truths:
    - "Images slice in installlocalpath/localpathprovisioner.go pins docker.io/rancher/local-path-provisioner:v0.0.36"
    - "Embedded manifest pins busybox:1.37.0 in BOTH occurrences (helperPod template image + helper-image flag)"
    - "Embedded manifest StorageClass keeps storageclass.kubernetes.io/is-default-class: \"true\" annotation"
    - "TestImagesPinsV0036, TestManifestPinsBusybox, and TestStorageClassIsDefault pass"
    - "go test ./pkg/cluster/internal/create/actions/installlocalpath/... -race passes"
    - "Atomic CHANGELOG stub for ADDON-01 landed in same commit pair as GREEN"
  artifacts:
    - path: "pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go"
      provides: "Images slice pinning local-path-provisioner v0.0.36"
      contains: "docker.io/rancher/local-path-provisioner:v0.0.36"
    - path: "pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go"
      provides: "TestImagesPinsV0036, TestManifestPinsBusybox, TestStorageClassIsDefault assertions"
      contains: "TestImagesPinsV0036"
    - path: "pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml"
      provides: "Upstream v0.0.36 manifest with kinder overrides re-applied"
      contains: "rancher/local-path-provisioner:v0.0.36"
  key_links:
    - from: "Images slice in localpathprovisioner.go"
      to: "Image tag in manifests/local-path-storage.yaml"
      via: "go:embed string match — both must reference v0.0.36"
      pattern: "local-path-provisioner:v0\\.0\\.36"
    - from: "Vendored manifest"
      to: "kinder air-gap support"
      via: "busybox:1.37.0 pin in TWO places + is-default-class annotation"
      pattern: "busybox:1\\.37\\.0|is-default-class"
---

<objective>
Bump local-path-provisioner from v0.0.35 to v0.0.36 (closes CVE GHSA-7fxv-8wr2-mfc4 HelperPod Template Injection security fix). Preserve kinder's two vendored overrides: `busybox:1.37.0` pin and `storageclass.kubernetes.io/is-default-class: "true"` annotation.

Purpose: ADDON-01 — security fix bundled into v2.4 baseline. Pattern: TDD RED commit (3 new assertion tests) → GREEN commit (Images slice tag + manifest YAML replacement with overrides re-applied + addon doc + CHANGELOG stub).

Output: 2-commit pair (RED + GREEN) plus atomic CHANGELOG line under the v2.4 H2.
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

@pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go
@pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go
@pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml
@kinder-site/src/content/docs/addons/local-path-provisioner.md
@kinder-site/src/content/docs/changelog.md
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: RED — add three failing assertion tests for v0.0.36 bump + override preservation</name>
  <files>
    pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go
  </files>
  <behavior>
    - Test 1 (TestImagesPinsV0036): asserts that Images slice contains "docker.io/rancher/local-path-provisioner:v0.0.36"
    - Test 2 (TestManifestPinsBusybox): asserts that the embedded manifest string contains "busybox:1.37.0" in TWO occurrences (Pitfall LPP-01)
    - Test 3 (TestStorageClassIsDefault): asserts that the embedded manifest string contains `storageclass.kubernetes.io/is-default-class: "true"` (Pitfall LPP-02)
    - All three tests MUST fail at HEAD (Images still pins v0.0.35; manifest may already pass busybox + default-class but tests must be created so the regression net is in place going forward)
  </behavior>
  <action>
Open `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go`. Append the three new test functions (do not delete or modify existing tests). The tests reference the package-level `Images []string` and the unexported `localPathManifest string` (the go:embed target — confirm the variable name by reading the source).

```go
// TestImagesPinsV0036 pins the Images slice to local-path-provisioner v0.0.36
// (ADDON-01: CVE GHSA-7fxv-8wr2-mfc4 fix).
func TestImagesPinsV0036(t *testing.T) {
	t.Parallel()
	const want = "docker.io/rancher/local-path-provisioner:v0.0.36"
	for _, img := range Images {
		if img == want {
			return
		}
	}
	t.Errorf("Images = %v; want to contain %q", Images, want)
}

// TestManifestPinsBusybox guards Pitfall LPP-01: upstream v0.0.36 manifest uses
// unpinned busybox; kinder's vendored manifest MUST re-pin busybox:1.37.0 in
// BOTH occurrences (helperPod template image + helper-image flag).
func TestManifestPinsBusybox(t *testing.T) {
	t.Parallel()
	const tag = "busybox:1.37.0"
	count := strings.Count(localPathManifest, tag)
	if count < 2 {
		t.Errorf("localPathManifest contains %q %d time(s); want >= 2 (helperPod image + helper-image flag)", tag, count)
	}
}

// TestStorageClassIsDefault guards Pitfall LPP-02: upstream manifest does NOT
// set is-default-class; kinder's vendored manifest MUST add it so PVCs without
// storageClassName are bound automatically.
func TestStorageClassIsDefault(t *testing.T) {
	t.Parallel()
	const annotation = `storageclass.kubernetes.io/is-default-class: "true"`
	if !strings.Contains(localPathManifest, annotation) {
		t.Errorf("localPathManifest missing annotation %q (PVCs without explicit class will hang Pending)", annotation)
	}
}
```

Add `"strings"` import if not already present.

Run the tests — TestImagesPinsV0036 MUST fail (still v0.0.35); TestManifestPinsBusybox should pass (kinder already pins both); TestStorageClassIsDefault should pass (kinder already has the annotation). The two passing tests act as a forward-looking regression net — Task 2 must NOT regress them when copying upstream YAML.

```bash
go test ./pkg/cluster/internal/create/actions/installlocalpath/... -run "TestImagesPinsV0036|TestManifestPinsBusybox|TestStorageClassIsDefault" -v
```

Confirm TestImagesPinsV0036 fails (output should print the v0.0.35 entry). Confirm the other two pass at HEAD.

Commit (RED):
```bash
git add pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner_test.go
git commit -m "test(53-01): add failing test pinning local-path-provisioner v0.0.36 + override guards"
```
  </action>
  <verify>
<automated>go test ./pkg/cluster/internal/create/actions/installlocalpath/... -run "TestImagesPinsV0036|TestManifestPinsBusybox|TestStorageClassIsDefault" -v</automated>
  </verify>
  <done>RED commit landed. TestImagesPinsV0036 fails at HEAD; the two override-preservation tests pass at HEAD (forward regression net for Task 2).</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: GREEN — bump Images slice + replace manifest YAML with v0.0.36 + re-apply kinder overrides + addon doc + CHANGELOG stub</name>
  <files>
    pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go
    pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml
    kinder-site/src/content/docs/addons/local-path-provisioner.md
    kinder-site/src/content/docs/changelog.md
  </files>
  <behavior>
    - Images slice line containing "v0.0.35" → "v0.0.36"
    - manifests/local-path-storage.yaml contents: replace with upstream v0.0.36 from https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.36/deploy/local-path-storage.yaml, then re-apply kinder overrides (busybox:1.37.0 pin in both spots, is-default-class annotation on the StorageClass)
    - All three RED tests now pass
    - Existing tests (TestExecute, TestImages, etc.) continue to pass
    - Addon doc version line bumped; security-fix mention added
    - CHANGELOG stub appended under v2.4 H2
  </behavior>
  <action>
**STEP A — Update the Images slice in `localpathprovisioner.go`.**

Find the `var Images = []string{` block (RESEARCH confirms it currently contains `"docker.io/rancher/local-path-provisioner:v0.0.35"` and `"docker.io/library/busybox:1.37.0"`). Change ONLY the v0.0.35 line:

```go
var Images = []string{
	"docker.io/rancher/local-path-provisioner:v0.0.36",
	"docker.io/library/busybox:1.37.0",
}
```

**STEP B — Replace `manifests/local-path-storage.yaml` with upstream v0.0.36 + re-applied kinder overrides.**

1. Download upstream v0.0.36 manifest:
   ```bash
   curl -sL https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.36/deploy/local-path-storage.yaml \
     -o /tmp/upstream-v0.0.36.yaml
   ```

2. Inspect the diff between `pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` (current — kinder overrides applied to v0.0.35) and `/tmp/upstream-v0.0.36.yaml`. Expected differences:
   - Image tag: upstream uses `rancher/local-path-provisioner:v0.0.36`; current pins `v0.0.35`.
   - `busybox:1.37.0` lines: upstream uses unpinned `image: busybox` (in the helperPod template inside the ConfigMap) and `--helper-image busybox` (in the deployment args); current pins both to `busybox:1.37.0`.
   - StorageClass annotation `storageclass.kubernetes.io/is-default-class: "true"`: present in current; absent in upstream.
   - Any new RBAC/policy improvements upstream added — accept these.

3. Copy upstream YAML to the manifest path:
   ```bash
   cp /tmp/upstream-v0.0.36.yaml \
     pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml
   ```

4. **Re-apply kinder override #1: busybox pin (TWO places).**
   - Find the helperPod ConfigMap key (typically `helperPod.yaml: |-` block). Replace `image: busybox` (unpinned) → `image: busybox:1.37.0`.
   - Find the deployment args (typically inside `kind: Deployment` → `spec.template.spec.containers[].args`). Replace `--helper-image=busybox` (or whatever exact form upstream uses) → `--helper-image=busybox:1.37.0`. If the flag form uses two separate args (`- --helper-image`, `- busybox`) on adjacent lines, pin the second arg.
   - After both edits, run: `grep -nE "image:.*busybox|helper-image" pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` and verify both occurrences pin `busybox:1.37.0`.

5. **Re-apply kinder override #2: is-default-class annotation.**
   - Find the `kind: StorageClass` resource (typically named `local-path`).
   - Add to `metadata.annotations`:
     ```yaml
     metadata:
       name: local-path
       annotations:
         storageclass.kubernetes.io/is-default-class: "true"
     ```
   - If upstream already has an `annotations:` block, add the line; otherwise create the block.

6. **Run all three RED tests — they MUST all pass now:**
   ```bash
   go test ./pkg/cluster/internal/create/actions/installlocalpath/... -run "TestImagesPinsV0036|TestManifestPinsBusybox|TestStorageClassIsDefault" -v
   ```

7. **Run the full installlocalpath package test suite to confirm no regression:**
   ```bash
   go test ./pkg/cluster/internal/create/actions/installlocalpath/... -race
   go build ./...
   ```

**STEP C — Update addon doc.**

Edit `kinder-site/src/content/docs/addons/local-path-provisioner.md`:
- Bump version line (search for "v0.0.35" near the top of the page; replace with "v0.0.36").
- Add a brief mention of the security fix in the addon doc body (single sentence pointing to GHSA-7fxv-8wr2-mfc4).
- No `:::caution[Breaking change]` callout needed — the bump is a security patch, not a breaking change.

**STEP D — CHANGELOG stub.**

Append to `kinder-site/src/content/docs/changelog.md` under the `## v2.4 — Hardening (in progress)` H2 (create the H2 above `## v1.5 — Inner Loop` if it doesn't exist yet — note that 53-00 may have already created it):

```markdown
- **`local-path-provisioner` bumped to v0.0.36** — closes [GHSA-7fxv-8wr2-mfc4](https://github.com/rancher/local-path-provisioner/security/advisories/GHSA-7fxv-8wr2-mfc4) HelperPod Template Injection security advisory. Embedded `busybox:1.37.0` pin and `is-default-class` StorageClass annotation preserved. (ADDON-01)
```

**STEP E — Commit GREEN.**

```bash
git add pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go \
        pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml \
        kinder-site/src/content/docs/addons/local-path-provisioner.md \
        kinder-site/src/content/docs/changelog.md
git commit -m "feat(53-01): bump local-path-provisioner to v0.0.36 (CVE GHSA-7fxv-8wr2-mfc4)"
```
  </action>
  <verify>
<automated>go test ./pkg/cluster/internal/create/actions/installlocalpath/... -race && go build ./... && grep -c "busybox:1.37.0" pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml | awk '{ if ($1 < 2) exit 1 }' && grep -q "is-default-class" pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml</automated>
  </verify>
  <done>(1) Images slice pins v0.0.36. (2) Manifest YAML body is upstream v0.0.36 with both kinder overrides re-applied (busybox pin in two places + is-default-class annotation). (3) All three new tests pass; existing tests still pass; -race clean. (4) Addon doc version bumped; security-fix mention present. (5) CHANGELOG stub line for ADDON-01 landed atomically with GREEN commit. (6) `git log --oneline -2` shows RED `test(53-01): ...` then GREEN `feat(53-01): ...`.</done>
</task>

</tasks>

<verification>
- `git log --oneline -2` shows the RED commit precedes the GREEN commit.
- `grep -n "v0.0.36" pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` returns at least 2 matches (Images slice + manifest image:).
- `grep -c "busybox:1.37.0" pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` returns >= 2.
- `grep "is-default-class" pkg/cluster/internal/create/actions/installlocalpath/manifests/local-path-storage.yaml` returns at least one match.
- `go test ./pkg/cluster/internal/create/actions/installlocalpath/... -race` exits 0.
- `go build ./...` exits 0.
- `kinder-site/src/content/docs/changelog.md` contains the ADDON-01 line item under the v2.4 H2.
</verification>

<success_criteria>
SC1 (Phase 53): `kinder create cluster` installs local-path-provisioner v0.0.36 (GHSA-7fxv-8wr2-mfc4 security fix closed); `kinder doctor offline-readiness` does not warn on a fresh default cluster (the latter is wired in 53-07 when the offlinereadiness allAddonImages list updates).

ADDON-01: local-path-provisioner v0.0.36 with embedded busybox pin retained and is-default-class annotation preserved.
</success_criteria>

<output>
After completion, create `.planning/phases/53-addon-version-audit-bumps-sync-05/53-01-SUMMARY.md` with: RED + GREEN commit SHAs, the diff of the manifest YAML around the two override points (showing the re-applied busybox pin and is-default-class annotation), test output for all three new tests, and a note that offlinereadiness consolidation is deferred to 53-07.
</output>
