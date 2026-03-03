---
phase: 23-cert-manager-addon
verified: 2026-03-03T17:00:00Z
status: passed
score: 4/4 must-haves verified
re_verification: false
---

# Phase 23: cert-manager Addon Verification Report

**Phase Goal:** Users get a fully ready cert-manager installation with a self-signed ClusterIssuer so Certificate resources work immediately after cluster creation — no manual cert-manager setup required
**Verified:** 2026-03-03T17:00:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                                          | Status     | Evidence                                                                                        |
|----|----------------------------------------------------------------------------------------------------------------|------------|-------------------------------------------------------------------------------------------------|
| 1  | cert-manager manifest is embedded and applied with --server-side during cluster creation                       | VERIFIED   | `//go:embed manifests/cert-manager.yaml` at line 29; `"apply", "--server-side", "-f", "-"` at line 72 in certmanager.go |
| 2  | All three deployments (cert-manager, cert-manager-cainjector, cert-manager-webhook) waited on with 300s timeout | VERIFIED   | Loop over all three names with `--timeout=300s` at lines 80-95 in certmanager.go               |
| 3  | A self-signed ClusterIssuer named selfsigned-issuer is applied after webhook is Available                      | VERIFIED   | `selfSignedClusterIssuerYAML` const has `name: selfsigned-issuer`; applied only after wait loop completes (lines 99-105) |
| 4  | Setting addons.certManager: false skips cert-manager installation entirely                                     | VERIFIED   | `runAddon("Cert Manager", opts.Config.Addons.CertManager, ...)` in create.go line 219; runAddon skips when enabled=false; test in load_test.go lines 215-216 |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact                                                                              | Expected                                    | Status   | Details                                                                   |
|---------------------------------------------------------------------------------------|---------------------------------------------|----------|---------------------------------------------------------------------------|
| `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go`              | Action implementation with NewAction()/Execute(), server-side flag | VERIFIED | 109 lines; exports NewAction(); Execute() implements all 3 steps with --server-side, 300s wait loop, and ClusterIssuer apply |
| `pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml` | Embedded cert-manager manifest (min 100 lines) | VERIFIED | 13263 lines, 986843 bytes (986KB); 3 Deployment resources confirmed; namespace: cert-manager present |
| `pkg/cluster/internal/create/create.go`                                               | runAddon wiring for Cert Manager            | VERIFIED | Import at line 39; `runAddon("Cert Manager", opts.Config.Addons.CertManager, installcertmanager.NewAction())` at line 219 |

### Key Link Verification

| From                           | To                                        | Via                           | Status   | Details                                                                                  |
|--------------------------------|-------------------------------------------|-------------------------------|----------|------------------------------------------------------------------------------------------|
| `create.go`                    | `installcertmanager` package              | import + runAddon call        | WIRED    | Import at line 39; runAddon call at line 219 passing `opts.Config.Addons.CertManager`    |
| `certmanager.go`               | `manifests/cert-manager.yaml`             | go:embed directive            | WIRED    | `//go:embed manifests/cert-manager.yaml` at line 29; `var certManagerManifest string`    |
| `certmanager.go Execute()`     | kubectl apply --server-side               | node.Command for manifest apply | WIRED  | Lines 69-75: `node.Command("kubectl", ..., "apply", "--server-side", "-f", "-").SetStdin(strings.NewReader(certManagerManifest)).Run()` |
| `certmanager.go Execute()`     | kubectl apply -f - (ClusterIssuer)        | node.Command after webhook wait | WIRED  | Lines 99-105: applied only after all three deployments in wait loop complete             |

### Requirements Coverage

| Requirement | Source Plan | Description                                                        | Status    | Evidence                                                                                       |
|-------------|-------------|--------------------------------------------------------------------|-----------|------------------------------------------------------------------------------------------------|
| CERT-01     | 23-01       | Embed and apply cert-manager manifest via go:embed                 | SATISFIED | go:embed at certmanager.go:29; --server-side at certmanager.go:72; manifest is 986KB           |
| CERT-02     | 23-01       | Wait for all 3 components to reach Available status                | SATISFIED | Loop over ["cert-manager", "cert-manager-cainjector", "cert-manager-webhook"] with --timeout=300s |
| CERT-03     | 23-01       | Bootstrap self-signed ClusterIssuer so Certificate resources work  | SATISFIED | selfSignedClusterIssuerYAML const; applied after webhook Available gate                        |
| CERT-04     | 23-01       | Addon enabled by default, disableable via addons.certManager: false | SATISFIED | boolVal(in.Addons.CertManager) in convert_v1alpha4.go:57; nil pointer defaults to true; test confirms false works |

### Anti-Patterns Found

| File           | Line | Pattern | Severity | Impact |
|----------------|------|---------|----------|--------|
| None found     | -    | -       | -        | -      |

No TODO, FIXME, placeholder, empty implementation, or stub patterns detected in any of the three modified files.

### Import Order Note

The `installcertmanager` import in `create.go` appears at line 39 (after `installcorednstuning` at line 38), but strictly alphabetically "cert" should come before "cni" and "coredns". The plan stated it should be "between installdashboard and installenvoygw", but the actual placement is before both. This is a cosmetic style issue — `go build ./...` passes with zero errors and `go test ./...` passes with no regressions. No functional impact.

### Human Verification Required

The following items require live cluster testing and cannot be verified programmatically:

#### 1. End-to-end cert-manager installation with real cluster

**Test:** Create a kinder cluster with default config (addons.certManager defaults to true). After creation completes, run `kubectl get deployments -n cert-manager` and observe status.
**Expected:** All three deployments (cert-manager, cert-manager-cainjector, cert-manager-webhook) show READY=1/1 and AVAILABLE=1 before the kinder create command returns.
**Why human:** Requires a running Docker/containerd environment and actual cluster provisioning to verify the 300s wait loop halts cluster readiness reporting until all three are available.

#### 2. ClusterIssuer availability for Certificate resources

**Test:** After cluster creation, run `kubectl apply -f -` with a Certificate resource referencing `issuerRef.name: selfsigned-issuer`. Observe whether it is accepted without error.
**Expected:** The Certificate resource is created without "no endpoints available" or webhook admission errors; the cert-manager controller issues it successfully.
**Why human:** Requires a live cluster with the webhook pod running; webhook admission errors only manifest at runtime.

#### 3. addons.certManager: false skips installation

**Test:** Create a kinder cluster with a config file setting `addons: {certManager: false}`. Verify the "Skipping Cert Manager (disabled in config)" message appears and cluster creation time is not delayed by any cert-manager webhook wait.
**Expected:** No cert-manager namespace exists post-creation; creation completes in normal time without any cert-manager webhook readiness wait.
**Why human:** Requires cluster creation to observe actual timing and kubectl verification that the namespace is absent.

## Gaps Summary

No gaps found. All four observable truths are verified, all three required artifacts exist and are substantive and wired, all four key links are confirmed, and all four requirements (CERT-01 through CERT-04) are satisfied.

The implementation correctly:
- Uses `go:embed` for the cert-manager v1.16.3 manifest (986KB, 13263 lines — note: v1.17.6 does not exist; v1.16.3 is latest stable, documented as a known deviation in the SUMMARY)
- Applies with `--server-side` (mandatory at 986KB > 256KB annotation limit)
- Waits on all three deployments in sequence with 300s timeout each
- Applies the `selfsigned-issuer` ClusterIssuer only after the webhook is confirmed Available
- Wires into `create.go` as a disableable addon via the existing `runAddon` helper
- Has `go build ./...` and `go test ./...` both passing with zero errors

---

_Verified: 2026-03-03T17:00:00Z_
_Verifier: Claude (gsd-verifier)_
