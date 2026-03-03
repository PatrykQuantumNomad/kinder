# Phase 23: cert-manager Addon - Research

**Researched:** 2026-03-03
**Domain:** cert-manager installation via static manifest + self-signed ClusterIssuer bootstrap, Go:embed pattern, kubectl wait for deployments
**Confidence:** HIGH for architecture and implementation pattern; MEDIUM for server-side apply requirement (flagged below)

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CERT-01 | Embed and apply cert-manager v1.17.6 manifest via go:embed | Confirmed: URL pattern `https://github.com/cert-manager/cert-manager/releases/download/v1.17.6/cert-manager.yaml`; same `//go:embed` pattern used by MetalLB and EnvoyGW addons; must use `--server-side` apply due to CRD size (see Pitfall 1) |
| CERT-02 | Wait for all 3 components (controller, webhook, cainjector) to reach Available status | Confirmed: exact deployment names are `cert-manager`, `cert-manager-cainjector`, `cert-manager-webhook` in `cert-manager` namespace; `kubectl wait --for=condition=Available deployment/...` pattern used by MetalLB and EnvoyGW addons |
| CERT-03 | Bootstrap self-signed ClusterIssuer so Certificate resources work immediately | Confirmed: ClusterIssuer YAML is trivial (`spec: { selfSigned: {} }`); MUST wait for webhook to be Available before applying ClusterIssuer or it fails; named `selfsigned-issuer` per success criteria |
| CERT-04 | Addon enabled by default, disableable via addons.certManager: false | Confirmed: `CertManager bool` already in internal `config.Addons` (Phase 21 complete); `boolPtrTrue(&obj.Addons.CertManager)` already in v1alpha4 `default.go`; `CertManager: boolVal(in.Addons.CertManager)` already in `convert_v1alpha4.go`; only `create.go` wiring (runAddon) and action package remain |
</phase_requirements>

## Summary

Phase 23 follows the exact same addon pattern established by Phases 22 (local registry) and the EnvoyGW addon, which is the most analogous precedent: apply a large static manifest, wait for components to become available, then bootstrap a post-install resource. The implementation is a new action package `installcertmanager` with an embedded `cert-manager.yaml` manifest and a self-signed ClusterIssuer inline YAML constant.

The key implementation subtlety is ordering: the cert-manager webhook must reach `Available` status before the ClusterIssuer can be applied. This is because the webhook validates cert-manager CRs — applying a ClusterIssuer while the webhook pod is still starting results in "no endpoints available for service cert-manager-webhook" errors. The solution is to wait for all three deployments to be Available before issuing the ClusterIssuer apply, which is the same sequential wait pattern used by EnvoyGW (wait for certgen job, then wait for controller, then apply GatewayClass).

A second important concern is `--server-side` apply. The cert-manager CRDs are large (the CRD YAML portion alone is ~150KB and total manifest is estimated 500KB+). The Kubernetes client-side apply limit is 256KB for `last-applied-configuration` annotations — cert-manager CRDs have exceeded this limit since v1.7+ (documented in cert-manager/cert-manager issue #4831 and #3933). The EnvoyGW manifest (2.5MB) already uses `--server-side` for exactly this reason. The cert-manager manifest must also use `--server-side` for a safe, idempotent first-time install.

**Primary recommendation:** Implement `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go` following the EnvoyGW pattern: `--server-side` apply for the manifest, three sequential `kubectl wait --for=condition=Available` calls for the three deployments with 300s timeout, then inline ClusterIssuer apply. Wire in `create.go` with `runAddon("Cert Manager", opts.Config.Addons.CertManager, installcertmanager.NewAction())` after the Dashboard addon.

## Standard Stack

### Core

| Component | Version/Source | Purpose | Why Standard |
|-----------|---------------|---------|--------------|
| cert-manager manifest | v1.17.6 (phase-locked) | CRDs + controller + webhook + cainjector | Official static manifest from GitHub releases; all resources in one file |
| `//go:embed` | Go stdlib | Embed cert-manager.yaml at build time | Same pattern as MetalLB, Dashboard, EnvoyGW addons |
| `node.Command("kubectl", "--server-side", ...)` | existing pkg | Apply large manifest safely | Required: CRDs exceed 256KB client-side apply annotation limit |
| `node.Command("kubectl", "wait", ...)` | existing pkg | Block until deployments ready | Same as MetalLB (controller wait) and EnvoyGW (certgen + controller wait) |
| Inline ClusterIssuer YAML const | — | Bootstrap self-signed issuer | Trivial YAML; no file needed; same inline-const pattern as MetalLB `crTemplate` |

### Supporting

| Component | Version/Source | Purpose | When to Use |
|-----------|---------------|---------|-------------|
| `nodeutils.ControlPlaneNodes` | existing pkg | Get control-plane node for kubectl calls | All manifest apply operations run on control-plane node |
| `strings.NewReader` | stdlib | Pass YAML as stdin to kubectl | Same as every other addon manifest apply |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `--server-side` apply | client-side `kubectl apply -f -` | Client-side fails for large CRDs (> 256KB annotation limit); v1.17 CRDs confirmed problematic since v1.7+; use `--server-side` |
| Inline ClusterIssuer YAML const | Embedded YAML file | File is 10 lines; inline const is simpler and matches MetalLB `crTemplate` pattern |
| Single wait for webhook | Wait for all 3 deployments | Webhook is sufficient for ClusterIssuer apply, but waiting for all 3 is more robust and matches the success criterion wording exactly |

**Installation:** No new Go dependencies. No `go get` required.

## Architecture Patterns

### Recommended Project Structure

```
pkg/cluster/internal/create/actions/installcertmanager/
├── certmanager.go        # Action implementation
└── manifests/
    └── cert-manager.yaml # Embedded v1.17.6 manifest (~500KB)
```

### Pattern 1: Large Manifest Apply with --server-side (CERT-01)

**What:** Apply the embedded cert-manager.yaml using `kubectl apply --server-side -f -` via the control-plane node. This avoids the 256KB `last-applied-configuration` annotation limit that breaks client-side apply for large CRDs.

**When to use:** CERT-01 — applied once during cluster creation.

**Source:** EnvoyGW addon (`installenvoygw/envoygw.go` line 69-75) uses identical pattern; cert-manager issue #4831 documents the CRD size problem.

```go
// Source: installenvoygw/envoygw.go pattern
if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "--server-side", "-f", "-",
).SetStdin(strings.NewReader(certManagerManifest)).Run(); err != nil {
    return errors.Wrap(err, "failed to apply cert-manager manifest")
}
```

### Pattern 2: Three-Deployment Availability Wait (CERT-02)

**What:** Wait for each of the three cert-manager deployments to reach `Available` condition before proceeding to ClusterIssuer bootstrap. Use 300s timeout — cert-manager can be slow on cold image pulls in kind.

**When to use:** CERT-02 — after manifest apply, before ClusterIssuer creation.

**Deployment names** (verified against official docs, HIGH confidence):
- `cert-manager` — main controller
- `cert-manager-cainjector` — CA injector
- `cert-manager-webhook` — validation/mutation webhook

All in namespace `cert-manager`.

**CRITICAL ordering:** ClusterIssuer apply MUST come AFTER the webhook is Available. The webhook validates cert-manager CRs. If applied too early, kubectl returns "no endpoints available for service cert-manager-webhook".

```go
// Source: cert-manager.io/docs/installation/kubectl/ — deployment names verified
// Pattern adapted from installmetallb/metallb.go wait pattern

for _, deployment := range []string{
    "cert-manager",
    "cert-manager-cainjector",
    "cert-manager-webhook",
} {
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "wait",
        "--namespace=cert-manager",
        "--for=condition=Available",
        "deployment/"+deployment,
        "--timeout=300s",
    ).Run(); err != nil {
        return errors.Wrapf(err, "cert-manager deployment %s did not become available", deployment)
    }
}
```

### Pattern 3: Self-Signed ClusterIssuer Bootstrap (CERT-03)

**What:** Apply a self-signed ClusterIssuer named `selfsigned-issuer` after cert-manager is fully available. The self-signed issuer requires no configuration — `spec.selfSigned: {}` is the complete spec.

**When to use:** CERT-03 — after all three deployments are Available.

**Source:** cert-manager.io/docs/configuration/selfsigned/ — SelfSigned issuer documented as ready "immediately" after apply.

```go
// Source: https://cert-manager.io/docs/configuration/selfsigned/
const selfSignedClusterIssuerYAML = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
`

if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "-f", "-",
).SetStdin(strings.NewReader(selfSignedClusterIssuerYAML)).Run(); err != nil {
    return errors.Wrap(err, "failed to apply selfsigned ClusterIssuer")
}
```

Note: ClusterIssuer uses standard client-side apply (NOT `--server-side`) — it is a small resource well within the 256KB limit.

### Pattern 4: create.go Wiring (CERT-04)

**What:** Register the cert-manager action in `create.go` via the `runAddon` helper, using `cfg.Addons.CertManager`.

**Ordering:** Cert Manager runs after Dashboard (last in the existing list). The cert-manager addon has no dependency on other addons, and no other addon depends on cert-manager. It should be placed last to not delay other addons with its 300s potential wait time.

**Note:** Unlike the Local Registry addon, cert-manager does NOT require a `ContainerdConfigPatches` injection before `p.Provision()`. No pre-provisioning step is needed.

```go
// In create.go addon section — add after Dashboard
runAddon("Dashboard", opts.Config.Addons.Dashboard, installdashboard.NewAction())
runAddon("Cert Manager", opts.Config.Addons.CertManager, installcertmanager.NewAction())
```

### Anti-Patterns to Avoid

- **Applying ClusterIssuer before webhook is Available:** The webhook validates ClusterIssuer resources. Applying before webhook readiness results in "no endpoints available for service cert-manager-webhook" error. Always wait for all three deployments first.
- **Using client-side `kubectl apply` for the cert-manager manifest:** The combined manifest + CRDs exceeds the 256KB client-side annotation limit. Use `--server-side` (same as EnvoyGW).
- **Using `--server-side` for the ClusterIssuer:** Small resources do not need `--server-side`. Use standard apply for the ClusterIssuer.
- **Very short wait timeouts:** On a fresh kind cluster with no cached images, cert-manager image pulls can take 60-120s. Use 300s (5 minutes) as the timeout — consistent with other addons using 120s for fast pulls but cert-manager pulling 3 images concurrently.
- **Naming the ClusterIssuer anything other than `selfsigned-issuer`:** The success criterion locks the name to `selfsigned-issuer`. Do not use `selfsigned-cluster-issuer` (the docs example uses this name, but the phase spec requires `selfsigned-issuer`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cert-manager installation | Custom CA/cert logic | cert-manager v1.17.6 static manifest | Full PKI automation including CRDs, RBAC, webhook, cainjector — 10,000+ lines of implementation |
| CRD application with large payloads | Chunked apply logic | `kubectl apply --server-side` | Server-side apply is built into kubectl; handles field ownership and large payloads natively |
| Webhook readiness detection | Health endpoint polling | `kubectl wait --for=condition=Available` | Standard Kubernetes condition check; handles pod restarts, backoffs, and leader election delays |
| PKI / certificate issuance | Custom Go TLS code | cert-manager ClusterIssuer | cert-manager handles certificate lifecycle, rotation, key management |

**Key insight:** cert-manager is a complete Kubernetes controller with its own CRDs, webhook, and RBAC. The only work for this phase is embedding its static manifest and bootstrapping a ClusterIssuer — no PKI logic belongs in kinder itself.

## Common Pitfalls

### Pitfall 1: CRD Annotation Size Limit — Silent Failure with Client-Side Apply

**What goes wrong:** `kubectl apply -f -` (client-side) stores the entire manifest in the `kubectl.kubernetes.io/last-applied-configuration` annotation. cert-manager CRDs have exceeded Kubernetes' 256KB annotation value limit since v1.7 (documented in cert-manager/cert-manager issue #4831). The apply may fail with "Request entity too large" or "annotations are too long" errors.

**Why it happens:** Kubernetes has a 1MB resource size limit and a 256KB annotation value limit. cert-manager CRDs include extensive OpenAPI v3 validation schemas that bloat the annotation.

**How to avoid:** Always use `kubectl apply --server-side -f -` for the cert-manager manifest. This uses server-side field management instead of annotations, bypassing the size limit. This is the same reason EnvoyGW uses `--server-side`.

**Warning signs:** `kubectl apply` exits with non-zero status and an error mentioning "too large" or "metadata.annotations: Too long". If using `--server-side` fails on a first-time install, check that the kubectl version inside the node image supports `--server-side` (requires kubectl 1.18+; kind node images use recent enough versions).

**Confidence:** MEDIUM — verified from cert-manager issue #4831 (v1.7 era) and confirmed as still applicable for v1.17 by the EnvoyGW precedent using the same workaround for the same reason.

### Pitfall 2: ClusterIssuer Applied Before Webhook Ready

**What goes wrong:** The action applies the cert-manager manifest and immediately applies the ClusterIssuer. The webhook pod has not reached the Available state yet. The ClusterIssuer apply fails with "Internal error occurred: failed calling webhook 'webhook.cert-manager.io': ... no endpoints available for service 'cert-manager-webhook'".

**Why it happens:** Kubernetes routes ClusterIssuer creation/update requests through the cert-manager webhook for validation. If the webhook pod is not yet running and its endpoint is not registered, the API server cannot forward the request.

**How to avoid:** Wait for `deployment/cert-manager-webhook` to reach `condition=Available` before applying the ClusterIssuer. The `kubectl wait` command in Pattern 2 (above) handles this.

**Warning signs:** Action exits with webhook error during ClusterIssuer apply but cert-manager pods are running. Check that the wait step ran before the ClusterIssuer apply.

### Pitfall 3: Wrong ClusterIssuer Name

**What goes wrong:** The phase success criterion specifies the ClusterIssuer be named `selfsigned-issuer`. The official cert-manager SelfSigned documentation uses `selfsigned-cluster-issuer` as its example name.

**Why it matters:** Users and downstream tools that reference the issuer by name (e.g., Certificate resources using `issuerRef.name: selfsigned-issuer`) will fail if the wrong name is used.

**How to avoid:** Use `name: selfsigned-issuer` — exactly as specified in CERT-03 success criterion.

**Warning signs:** After cluster creation, `kubectl get clusterissuer selfsigned-issuer` returns "not found" even though cert-manager installed successfully.

### Pitfall 4: Timeout Too Short for Cold Pulls

**What goes wrong:** The 120s timeout used by MetalLB is insufficient for cert-manager, which pulls 3 images (controller, webhook, cainjector) concurrently. On a cold Docker cache, each image pull can take 30-60s.

**Why it happens:** cert-manager images (especially the webhook, which uses a Go binary base) are larger than typical infrastructure images. In a CI environment or fresh machine, all three can be pulling simultaneously.

**How to avoid:** Use `--timeout=300s` (5 minutes) for all three `kubectl wait` calls. This is conservative but ensures the action does not fail on legitimate slow image pulls.

**Warning signs:** `kubectl wait` exits with "timed out waiting for the condition" even though the pod is actively pulling images (status: `ContainerCreating`).

### Pitfall 5: Missing Import in create.go

**What goes wrong:** The `installcertmanager` package is imported in `create.go` but the build fails because the import path does not match the package directory name or the alphabetical import ordering is wrong.

**Why it happens:** Go is strict about import paths. The kinder codebase orders local imports alphabetically within the local imports group.

**How to avoid:** Add import `"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installcertmanager"` after `installdashboard` and before `installenvoygw` (alphabetical order). The package directory name must be `installcertmanager` to match.

**Warning signs:** `go build ./...` fails with "could not import" or "package not found".

## Code Examples

Verified patterns from official sources and codebase:

### Complete Action Skeleton

```go
// Source: installenvoygw/envoygw.go (pattern), cert-manager.io/docs (names/YAML)
package installcertmanager

import (
    _ "embed"
    "strings"

    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/nodeutils"
    "sigs.k8s.io/kind/pkg/errors"
)

//go:embed manifests/cert-manager.yaml
var certManagerManifest string

const selfSignedClusterIssuerYAML = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
`

type action struct{}

// NewAction returns a new action for installing cert-manager.
func NewAction() actions.Action {
    return &action{}
}

// Execute runs the action.
func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Installing Cert Manager")
    defer ctx.Status.End(false)

    // Get control plane node
    allNodes, err := ctx.Nodes()
    if err != nil {
        return errors.Wrap(err, "failed to list cluster nodes")
    }
    controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)
    if err != nil {
        return errors.Wrap(err, "failed to find control plane nodes")
    }
    if len(controlPlanes) == 0 {
        return errors.New("no control plane nodes found")
    }
    node := controlPlanes[0]

    // Step 1 (CERT-01): Apply cert-manager manifest.
    // --server-side is REQUIRED: cert-manager CRDs exceed the 256KB
    // last-applied-configuration annotation limit for standard apply.
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "--server-side", "-f", "-",
    ).SetStdin(strings.NewReader(certManagerManifest)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply cert-manager manifest")
    }

    // Step 2 (CERT-02): Wait for all three components to be Available.
    // The webhook MUST be Available before the ClusterIssuer can be applied.
    // 300s timeout: cert-manager pulls 3 images; cold-pull can take 2-3 min total.
    for _, deployment := range []string{
        "cert-manager",
        "cert-manager-cainjector",
        "cert-manager-webhook",
    } {
        if err := node.Command(
            "kubectl",
            "--kubeconfig=/etc/kubernetes/admin.conf",
            "wait",
            "--namespace=cert-manager",
            "--for=condition=Available",
            "deployment/"+deployment,
            "--timeout=300s",
        ).Run(); err != nil {
            return errors.Wrapf(err, "cert-manager deployment %s did not become available", deployment)
        }
    }

    // Step 3 (CERT-03): Bootstrap self-signed ClusterIssuer.
    // Named "selfsigned-issuer" per phase success criteria.
    // Standard (client-side) apply is fine — ClusterIssuer is a small resource.
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(selfSignedClusterIssuerYAML)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply selfsigned ClusterIssuer")
    }

    ctx.Status.End(true)
    return nil
}
```

### create.go Wiring (CERT-04)

```go
// Source: create.go existing pattern — add after Dashboard
// Import to add (alphabetical after installdashboard, before installenvoygw):
// "sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installcertmanager"

// In addon section:
runAddon("Dashboard",    opts.Config.Addons.Dashboard,    installdashboard.NewAction())
runAddon("Cert Manager", opts.Config.Addons.CertManager,  installcertmanager.NewAction())
// NOTE: No ContainerdConfigPatches injection needed for cert-manager
// (unlike LocalRegistry which needs pre-provisioning config_path setup)
```

### Self-Signed ClusterIssuer YAML

```yaml
# Source: https://cert-manager.io/docs/configuration/selfsigned/
# Name "selfsigned-issuer" per CERT-03 success criterion
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-issuer
spec:
  selfSigned: {}
```

### Downloading the Manifest (build-time task)

```bash
# Download cert-manager v1.17.6 manifest for embedding
curl -Lo pkg/cluster/internal/create/actions/installcertmanager/manifests/cert-manager.yaml \
  https://github.com/cert-manager/cert-manager/releases/download/v1.17.6/cert-manager.yaml
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Client-side `kubectl apply` for CRDs | `kubectl apply --server-side` | cert-manager v1.7+ (CRD size exceeded 256KB) | Must use `--server-side` to avoid annotation limit errors |
| Wait for pods Running | Wait for deployment Available condition | General Kubernetes maturity | `condition=Available` is more robust than pod status checks |
| Manual cert-manager setup after cluster creation | Embedded addon, auto-installed during creation | kinder v1.3 Phase 23 | Batteries-included; Certificate resources work immediately after `kinder create cluster` |

**Deprecated/outdated:**
- Using `kubectl apply` without `--server-side` for cert-manager CRDs: broken since v1.7 due to CRD annotation size
- Waiting for `cert-manager-startupapicheck` Job: the job is ephemeral and only present at install time; `deployment Available` wait is more reliable and re-runnable

## Open Questions

1. **Exact cert-manager v1.17.6 manifest size and --server-side requirement**
   - What we know: cert-manager CRDs have exceeded the 256KB client-side annotation limit since v1.7 (documented issue #4831). The manifest for v1.17.2 begins with ~150KB of CRDs. EnvoyGW (2.5MB) uses `--server-side` for the same reason.
   - What's unclear: The exact total size of the v1.17.6 `cert-manager.yaml` was not confirmed (manifest content was truncated during WebFetch). The issue may be theoretically resolved if cert-manager reduced schema validation verbosity, but no evidence of this was found.
   - Recommendation: Use `--server-side` unconditionally. The worst case of using `--server-side` on a small manifest is harmless; the worst case of NOT using it on a large manifest is a hard failure. This is MEDIUM confidence — verify by checking the downloaded manifest file size.

2. **Optimal wait timeout for cert-manager deployments**
   - What we know: cert-manager pulls 3 container images. MetalLB uses 120s; EnvoyGW uses 120s for the certgen job and 120s for the controller. cert-manager documentation suggests `cmctl check api --wait=2m`.
   - What's unclear: Whether 300s is conservative enough for constrained CI environments or too long for normal use.
   - Recommendation: Use 300s. This is consistent with the phase success criterion that "all three components reach Available status before the cluster is reported ready" — a generous timeout prevents false failures.

3. **ClusterIssuer readiness verification**
   - What we know: The SelfSigned issuer "should be ready instantly" per official docs. There is no standard `kubectl wait` for ClusterIssuer conditions in older kubectl versions.
   - What's unclear: Whether a `kubectl wait --for=condition=Ready clusterissuer/selfsigned-issuer` is needed after the apply, or whether apply success implies readiness for SelfSigned issuers.
   - Recommendation: Do NOT add a wait for the ClusterIssuer. The SelfSigned issuer has no external dependencies and becomes Ready immediately. The phase success criterion says Certificate resources should work "without error" — this is satisfied by the webhook wait in Step 2 + ClusterIssuer apply in Step 3.

## Validation Architecture

No `nyquist_validation` config found — skipping structured test map.

Manual validation after implementation:
```bash
# 1. Verify all three deployments are Available
kubectl get deployments -n cert-manager
# Expected: cert-manager, cert-manager-cainjector, cert-manager-webhook all Available

# 2. Verify ClusterIssuer exists and is Ready
kubectl get clusterissuer selfsigned-issuer
# Expected: READY = True

# 3. Verify Certificate resource can be created
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: test-cert
  namespace: default
spec:
  secretName: test-cert-secret
  issuerRef:
    name: selfsigned-issuer
    kind: ClusterIssuer
  commonName: test.example.com
  dnsNames:
  - test.example.com
EOF
kubectl wait --for=condition=Ready certificate/test-cert --timeout=30s
# Expected: certificate/test-cert condition met

# 4. Verify cert-manager is skipped when disabled
# In cluster config: addons: { certManager: false }
# Expected: "Skipping Cert Manager (disabled in config)" in output
```

## Sources

### Primary (HIGH confidence)
- Direct codebase read: `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` — `--server-side` pattern, sequential wait pattern, GatewayClass apply after controller wait — direct template for this implementation
- Direct codebase read: `pkg/cluster/internal/create/create.go` — `runAddon` pattern, addon ordering, confirmed no ContainerdConfigPatches needed for non-registry addons
- Direct codebase read: `pkg/internal/apis/config/types.go` — `CertManager bool` confirmed in `Addons` struct
- Direct codebase read: `pkg/internal/apis/config/convert_v1alpha4.go` — `CertManager: boolVal(in.Addons.CertManager)` confirmed wired (Phase 21 complete)
- Direct codebase read: `pkg/apis/config/v1alpha4/default.go` — `boolPtrTrue(&obj.Addons.CertManager)` confirmed wired (Phase 21 complete)
- `https://cert-manager.io/docs/installation/kubectl/` — three deployment names confirmed: `cert-manager`, `cert-manager-cainjector`, `cert-manager-webhook`; all in `cert-manager` namespace
- `https://cert-manager.io/docs/configuration/selfsigned/` — SelfSigned ClusterIssuer YAML, "ready instantly" confirmation

### Secondary (MEDIUM confidence)
- `https://github.com/cert-manager/cert-manager/issues/4831` — CRD annotation size limit issue; confirms `--server-side` is required for CRD apply since v1.7+; applies to v1.17 by extension
- `https://github.com/cert-manager/cert-manager/discussions/4820` — confirms no reliable single `kubectl wait` for full readiness; recommends waiting for individual deployments + retry for CR apply
- `https://cert-manager.io/docs/releases/release-notes/release-notes-1.17/` — v1.17.6 existence confirmed (released December 17, 2025); v1.17 reached EOL October 7, 2025 (pinned version is used per phase spec)
- WebSearch result confirming v1.17.6 release date: December 17, 2025; Go version update for CVE fixes

### Tertiary (LOW confidence — flagged)
- Manifest size estimate (~150KB CRDs, estimated 500KB total) — derived from WebFetch of truncated manifest content; actual v1.17.6 size not confirmed; marked LOW — verify by downloading and checking size
- 300s timeout recommendation — derived from cert-manager docs' `--wait=2m` suggestion + conservative buffer; not benchmarked against kind specifically

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — `//go:embed`, `node.Command`, `nodeutils.ControlPlaneNodes` all verified from codebase; cert-manager deployment names verified from official docs
- Architecture: HIGH — directly derived from EnvoyGW addon pattern (verified from codebase); only new concern is `--server-side` flag (MEDIUM confidence, but safe to use unconditionally)
- Pitfalls: HIGH — webhook-before-ClusterIssuer pitfall is documented in official cert-manager troubleshooting; CRD size pitfall is documented in GitHub issues; both are preventable by the implementation patterns above
- Config wiring: HIGH — CertManager field verified present in all three config layers (v1alpha4 types, v1alpha4 default, internal convert) from direct codebase reads; Phase 21 confirmed complete

**Research date:** 2026-03-03
**Valid until:** 2026-04-03 (cert-manager v1.17.6 is pinned; patterns are stable; --server-side apply behavior is stable since kubectl 1.18)
