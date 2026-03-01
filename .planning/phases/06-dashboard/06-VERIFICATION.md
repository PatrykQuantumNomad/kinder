---
status: human_needed
phase: 06-dashboard
verified: 2026-03-01
score: 5/5 must-haves verified
human_verification:
  - test: "Run `kinder create cluster` and check output for token and port-forward command"
    expected: "Output includes a JWT token and the line `kubectl port-forward -n kube-system service/headlamp 8080:80`"
    why_human: "Cannot run a live Docker-based cluster in CI; only end-to-end execution proves the spinner/Logger output flows correctly"
  - test: "Copy the printed port-forward command, run it, open http://localhost:8080, paste the printed token"
    expected: "Headlamp UI loads, login succeeds, and cluster resources (pods, services, deployments, logs) are visible across namespaces"
    why_human: "UI rendering and Headlamp token authentication cannot be verified without a running cluster and a browser"
  - test: "Create a cluster config file with `addons: {dashboard: false}` and run `kinder create cluster --config <file>`"
    expected: "No headlamp pods or kinder-dashboard RBAC resources appear in kube-system; output shows `Skipping Dashboard (disabled in config)`"
    why_human: "Requires live cluster to confirm zero Kubernetes objects are created when addon is disabled"
---

# Phase 6: Dashboard — Verification

## Goal

Headlamp is installed and a developer can immediately open the Kubernetes dashboard using a printed token and port-forward command.

**Verified:** 2026-03-01
**Status:** human_needed (all automated checks passed; 3 items require live cluster testing)

---

## Must-Have Verification

### Truths

- [x] **Headlamp Deployment reaches Available condition in kube-system during cluster creation**

  Evidence: `dashboard.go` lines 71–81 call `kubectl wait --namespace=kube-system --for=condition=Available deployment/headlamp --timeout=120s` via `node.Command(...).Run()`. Error is wrapped and surfaced to the caller, so cluster creation fails cleanly if Headlamp does not become ready.

- [x] **kinder-dashboard ServiceAccount has cluster-admin access via ClusterRoleBinding**

  Evidence: `headlamp.yaml` lines 10–21 define a `ClusterRoleBinding` named `kinder-dashboard` that binds `ClusterRole/cluster-admin` to `ServiceAccount/kinder-dashboard` in `kube-system`. The subjects entry includes `namespace: kube-system` (required for ServiceAccount kind).

- [x] **A long-lived service account token is printed to the user after cluster creation**

  Evidence: `dashboard.go` lines 83–100 read `kinder-dashboard-token` Secret via `kubectl get secret ... -o jsonpath={.data.token}` into a `bytes.Buffer`, then call `base64.StdEncoding.DecodeString(strings.TrimSpace(...))`. The decoded JWT is printed via `ctx.Logger.V(0).Infof("  Token: %s", token)` (line 108). The Secret in `headlamp.yaml` is typed `kubernetes.io/service-account-token` — a long-lived token with no TTL.

- [x] **A port-forward command is printed so the user can access the dashboard immediately**

  Evidence: `dashboard.go` line 109 prints `"  Port-forward: kubectl port-forward -n kube-system service/headlamp 8080:80"` and line 110 prints `"  Then open: http://localhost:8080"` via `ctx.Logger.V(0).Info(...)`.

- [x] **Setting `addons.dashboard: false` skips all Headlamp resources (already wired in create.go)**

  Evidence: `create.go` line 203: `runAddon("Dashboard", opts.Config.Addons.Dashboard, installdashboard.NewAction())`. The `runAddon` closure (lines 179–191) checks `enabled` first; when false it logs `Skipping Dashboard (disabled in config)` and returns without calling Execute. The `Dashboard` field is declared in `pkg/apis/config/v1alpha4/types.go` line 109 and converted in `convert_v1alpha4.go` line 55. Default is `true` (v1alpha4 default.go line 90).

### Artifacts

- [x] **`pkg/cluster/internal/create/actions/installdashboard/manifests/headlamp.yaml`** — EXISTS, substantive (85 lines), WIRED

  Contains all 5 resources in dependency order:
  - `ServiceAccount/kinder-dashboard` in `kube-system`
  - `ClusterRoleBinding/kinder-dashboard` -> `cluster-admin` with `namespace: kube-system` in subjects
  - `Secret/kinder-dashboard-token` type `kubernetes.io/service-account-token`
  - `Service/headlamp` port 80 -> 4466, selector `k8s-app: headlamp`
  - `Deployment/headlamp` image `ghcr.io/headlamp-k8s/headlamp:v0.40.1`, replicas 1, probes on port 4466 (initialDelaySeconds 30, timeoutSeconds 30), no OpenTelemetry env vars

  Wired: embedded via `//go:embed manifests/headlamp.yaml` in `dashboard.go` (line 32).

- [x] **`pkg/cluster/internal/create/actions/installdashboard/dashboard.go`** — EXISTS, substantive (114 lines), WIRED

  Implements full Execute function:
  - Spinner start/end (Status.Start + Status.End)
  - Control plane node resolution via `nodeutils.ControlPlaneNodes`
  - Manifest apply via kubectl stdin
  - Deployment wait via `kubectl wait --for=condition=Available`
  - Token read into `bytes.Buffer` via SetStdout
  - Base64 decode via `base64.StdEncoding.DecodeString`
  - `ctx.Status.End(true)` before multi-line Logger output
  - Token + port-forward printed via `ctx.Logger.V(0).Info/Infof`

  Wired: imported and called by `create.go` line 203 via `installdashboard.NewAction()`.

### Key Links

- [x] **`dashboard.go` -> `manifests/headlamp.yaml` via `go:embed` directive**

  Evidence: `dashboard.go` line 32: `//go:embed manifests/headlamp.yaml` and line 33: `var headlampManifest string`. Build passes (`go build ./...`), confirming the embed resolves correctly.

- [x] **`dashboard.go` -> `kinder-dashboard-token` Secret via `SetStdout` + `base64.StdEncoding.DecodeString`**

  Evidence: `dashboard.go` lines 84–99. `SetStdout(&tokenBuf)` captures kubectl jsonpath output; `base64.StdEncoding.DecodeString(strings.TrimSpace(tokenBuf.String()))` decodes it in Go (avoiding cross-platform base64 flag differences). Import `"encoding/base64"` confirmed at line 23.

- [x] **`create.go` line 203 -> `dashboard.go` via `runAddon("Dashboard", opts.Config.Addons.Dashboard, installdashboard.NewAction())`**

  Evidence: `create.go` line 203 confirmed. Import `"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/installdashboard"` at line 39. The `runAddon` helper gates on `opts.Config.Addons.Dashboard`, enabling DASH-06.

---

## Success Criteria Assessment

| # | Success Criterion | Status | Evidence |
|---|-------------------|--------|----------|
| 1 | `kinder create cluster` output includes long-lived token and exact `kubectl port-forward` command | VERIFIED (automated) | `dashboard.go` lines 106–111 print token + port-forward string via `ctx.Logger.V(0)` |
| 2 | Following the port-forward command, Headlamp UI loads and accepts the printed token | NEEDS HUMAN | Requires live cluster + browser; cannot verify UI rendering or Headlamp token auth programmatically |
| 3 | In the Headlamp UI, user can view pods, services, deployments, and logs across namespaces | NEEDS HUMAN | RBAC is `cluster-admin` (verified in manifest), so access is theoretically complete; must confirm Headlamp UI navigation works with `-in-cluster` arg |
| 4 | `addons.dashboard: false` causes no Headlamp pods or RBAC resources to be installed | VERIFIED (automated) | `runAddon` in `create.go` short-circuits before calling `installdashboard.NewAction().Execute()`; field defaults and conversion chain verified |

---

## Requirements Coverage

| Requirement | Description | Status | Evidence |
|-------------|-------------|--------|----------|
| DASH-01 | Headlamp installed and running in `kube-system` by default | SATISFIED | Deployment in `headlamp.yaml` targets `kube-system`; `kubectl wait --for=condition=Available` enforces readiness |
| DASH-02 | `kinder-dashboard` SA with cluster-admin role | SATISFIED | `ServiceAccount` + `ClusterRoleBinding` -> `cluster-admin` in `headlamp.yaml`; `namespace: kube-system` in subjects |
| DASH-03 | Token printed at end of `kinder create cluster` | SATISFIED | `ctx.Logger.V(0).Infof("  Token: %s", token)` in `dashboard.go` line 108 |
| DASH-04 | Port-forward command printed | SATISFIED | `dashboard.go` line 109 prints exact `kubectl port-forward -n kube-system service/headlamp 8080:80` |
| DASH-05 | User can view pods, services, deployments, logs in Headlamp UI | NEEDS HUMAN | Cluster-admin RBAC + `-in-cluster` arg satisfies this architecturally; live UI test required |
| DASH-06 | Disable via `addons.dashboard: false` | SATISFIED | `runAddon("Dashboard", opts.Config.Addons.Dashboard, ...)` in `create.go` line 203; `Dashboard *bool` field in v1alpha4 API |

---

## Anti-Pattern Scan

No blockers or warnings found:

- No TODO/FIXME/PLACEHOLDER comments in `dashboard.go` or `headlamp.yaml`
- No empty implementations (`return {}`, `return []`, placeholder returns)
- No OpenTelemetry env vars in manifest (explicit plan requirement, confirmed absent)
- The only `return nil` in `dashboard.go` (line 113) is the legitimate success return at end of `Execute`
- Image is pinned to `ghcr.io/headlamp-k8s/headlamp:v0.40.1` (not `latest`)
- `go build ./...` passes
- `go vet ./pkg/cluster/internal/create/actions/installdashboard/...` passes

---

## Human Verification Required

### 1. Token and Port-Forward Appear in Cluster Creation Output

**Test:** Run `kinder create cluster` and inspect stdout.
**Expected:** Output includes a section like:
```
 * Installing Dashboard ... done

Dashboard:
  Token: eyJhbGciOiJSUzI1NiIsImtpZCI6Ii...
  Port-forward: kubectl port-forward -n kube-system service/headlamp 8080:80
  Then open: http://localhost:8080
```
**Why human:** Cannot run a Docker-based kind cluster in automated verification; only live execution proves the Logger output flows correctly through the spinner lifecycle.

### 2. Headlamp UI Loads and Accepts the Token

**Test:** Copy the printed port-forward command, run it in a terminal, open `http://localhost:8080` in a browser, and paste the printed token into the Headlamp sign-in screen.
**Expected:** Headlamp loads the cluster overview without errors; the token is accepted and the user is authenticated.
**Why human:** UI rendering and Headlamp token-based authentication require a browser and a live cluster.

### 3. Pods, Services, Deployments, and Logs Are Visible

**Test:** After signing in (test 2), navigate to Workloads -> Pods, Workloads -> Deployments, Network -> Services, and click any pod's log view.
**Expected:** Resources from all namespaces are listed; log streaming works.
**Why human:** Headlamp UI navigation and log streaming cannot be verified without a running cluster and browser session.

### 4. Dashboard Disabled by Config

**Test:** Create a config file:
```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
addons:
  dashboard: false
```
Run `kinder create cluster --config <file>`. After creation, run `kubectl get all -n kube-system`.
**Expected:** No `pod/headlamp-*` pods exist; no `serviceaccount/kinder-dashboard` or `clusterrolebinding/kinder-dashboard` exist; create output includes `Skipping Dashboard (disabled in config)`.
**Why human:** Requires live cluster to confirm zero Kubernetes objects are created; object absence cannot be confirmed without a running API server.

---

## Summary

All 5 automated must-haves are verified against the actual codebase. Both implementation files (`dashboard.go` and `manifests/headlamp.yaml`) exist, are substantive, and are fully wired into the cluster creation pipeline. The three key integration points (go:embed, base64 token decode, create.go addon wiring) are all confirmed. The build and vet pass cleanly. All 6 requirements (DASH-01 through DASH-06) have implementation evidence.

The phase goal is architecturally achieved. The 3 human verification items above confirm the live runtime behavior that programmatic verification cannot reach: the actual CLI output format, Headlamp UI authentication with the printed token, and absence of objects when the addon is disabled.

---

_Verified: 2026-03-01_
_Verifier: Claude (gsd-verifier)_
