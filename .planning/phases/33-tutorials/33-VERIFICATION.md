---
phase: 33-tutorials
verified: 2026-03-04T11:05:00Z
status: passed
score: 4/4 must-haves verified
re_verification: false
gaps: []
human_verification:
  - test: "Follow the TLS tutorial end-to-end on a real machine"
    expected: "HTTPS curl to the gateway IP returns the nginx welcome page"
    why_human: "Requires a running cluster with MetalLB IP allocation, cert-manager issuing a real certificate, and Envoy Gateway routing — cannot simulate programmatically"
  - test: "Follow the HPA tutorial and watch REPLICAS increase in kubectl get hpa --watch output"
    expected: "REPLICAS increases from 1 toward 5 as CPU climbs above 50%, then returns to 1 within 5 minutes after kubectl delete pod load-generator"
    why_human: "Requires a live cluster with Metrics Server scraping CPU, real load generation, and time-based observation"
  - test: "Follow the local dev workflow tutorial and verify curl output changes from 'Hello from v1!' to 'Hello from v2!'"
    expected: "After docker build/push v2 and kubectl set image, curl http://localhost:8080 returns Hello from v2!"
    why_human: "Requires Docker building a real Go image and a live cluster pulling from localhost:5001"
---

# Phase 33: Tutorials Verification Report

**Phase Goal:** Users can follow end-to-end tutorials that demonstrate real workflows combining multiple kinder features
**Verified:** 2026-03-04T11:05:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth                                                                                                                                                                          | Status     | Evidence                                                                                                                          |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------- | --------------------------------------------------------------------------------------------------------------------------------- |
| 1   | TLS tutorial guides user from cluster creation through pushing an image, exposing via Envoy Gateway with cert-manager TLS, and verifying HTTPS works                          | VERIFIED   | tls-web-app.md: 334 lines, Steps 1-8 cover full flow; ClusterIssuer x2, localhost:5001 x3, curl -k x2, gatewayClassName: eg x1  |
| 2   | HPA tutorial guides user through deploying a workload, configuring an HPA, generating load, and watching pods scale up in real time                                           | VERIFIED   | hpa-auto-scaling.md: 254 lines; HorizontalPodAutoscaler x1, kubectl run load-generator x7, resources.requests.cpu present, 5-min scale-down note present |
| 3   | Local dev workflow tutorial shows the full code-build-push-deploy-iterate loop using localhost:5001 with a concrete example application                                        | VERIFIED   | local-dev-workflow.md: 257 lines; localhost:5001 x16, kubectl set image x2, v1-to-v2 iteration with rollout status shown        |
| 4   | Each tutorial has clearly marked prerequisites, copy-pasteable commands, and expected output so a user can follow without guessing                                             | VERIFIED   | All three files have `## Prerequisites` section, shell commands in ```sh blocks, YAML in ```yaml blocks, "Expected output:" before every bare fenced block |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact                                                              | Expected                                           | Status     | Details                                                               |
| --------------------------------------------------------------------- | -------------------------------------------------- | ---------- | --------------------------------------------------------------------- |
| `kinder-site/src/content/docs/guides/tls-web-app.md`                 | Complete TLS web app tutorial (min 150 lines)      | VERIFIED   | 334 lines, substantive content, wired in sidebar slug                 |
| `kinder-site/src/content/docs/guides/hpa-auto-scaling.md`            | Complete HPA auto-scaling tutorial (min 120 lines) | VERIFIED   | 254 lines, substantive content, wired in sidebar slug                 |
| `kinder-site/src/content/docs/guides/local-dev-workflow.md`          | Complete local dev workflow tutorial (min 120 lines) | VERIFIED | 257 lines, substantive content, wired in sidebar slug                 |

### Key Link Verification

| From                    | To                        | Via                                    | Status     | Details                                                                |
| ----------------------- | ------------------------- | -------------------------------------- | ---------- | ---------------------------------------------------------------------- |
| tls-web-app.md          | cert-manager addon        | selfsigned-issuer ClusterIssuer ref    | WIRED      | `kind: ClusterIssuer` appears twice; caution callout explains why     |
| tls-web-app.md          | envoy-gateway addon       | gatewayClassName: eg                   | WIRED      | `gatewayClassName: eg` present in Gateway YAML                        |
| tls-web-app.md          | local-registry addon      | localhost:5001 push/pull               | WIRED      | `localhost:5001` appears 3 times (tag, push, deploy image ref)        |
| tls-web-app.md          | metallb addon             | curl -k https to gateway IP            | WIRED      | `curl -k https` appears twice (direct IP + port-forward macOS path)   |
| hpa-auto-scaling.md     | metrics-server addon      | resources.requests.cpu in deployment   | WIRED      | `requests:` + `cpu: "200m"` present; caution callout enforces it      |
| hpa-auto-scaling.md     | load generation           | kubectl run busybox loop               | WIRED      | `kubectl run load-generator` with full busybox wget loop command      |
| local-dev-workflow.md   | local-registry addon      | docker push to localhost:5001          | WIRED      | `docker push localhost:5001` present; localhost:5001 appears 16 times  |
| local-dev-workflow.md   | iteration loop            | kubectl set image for rolling update   | WIRED      | `kubectl set image deployment/myapp` appears twice (v1 setup + v2 update) |

### Requirements Coverage

| Requirement | Source Plan | Description                                              | Status    | Evidence                                                           |
| ----------- | ----------- | -------------------------------------------------------- | --------- | ------------------------------------------------------------------ |
| GUIDE-01    | 33-01-PLAN  | TLS web app tutorial (Local Registry + Envoy Gateway + cert-manager + MetalLB) | SATISFIED | tls-web-app.md: all four addons integrated in 8-step tutorial; commit 1e3cdcc6 |
| GUIDE-02    | 33-02-PLAN  | HPA auto-scaling tutorial (Metrics Server + load generation) | SATISFIED | hpa-auto-scaling.md: Metrics Server, busybox load generator, HPA YAML; commit 0a6c9091 |
| GUIDE-03    | 33-02-PLAN  | Local dev workflow tutorial (localhost:5001 code-build-push-deploy-iterate) | SATISFIED | local-dev-workflow.md: Go app, build/push v1, deploy, port-forward, iterate to v2; commit 67a2901f |

### Anti-Patterns Found

No anti-patterns found.

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| — | — | No TODO/FIXME/placeholder/Coming soon/stub patterns detected | — | — |

Explicit check: `grep -i "coming soon\|placeholder\|TODO\|FIXME" *.md` returned zero results across all three files.

### Human Verification Required

### 1. TLS Tutorial End-to-End

**Test:** Create a kinder cluster, follow Steps 1-8 of tls-web-app.md exactly as written
**Expected:** `curl -k https://$GATEWAY_IP` returns the nginx welcome page HTML (or port-forward path works on macOS/Windows)
**Why human:** Requires a live cluster with MetalLB assigning a real IP, cert-manager issuing a certificate from the selfsigned ClusterIssuer, and Envoy Gateway routing HTTPS traffic — none of which can be simulated with grep

### 2. HPA Scale-Up and Scale-Down Observation

**Test:** Follow the HPA tutorial, run the load-generator pod, watch `kubectl get hpa php-apache-hpa --watch` for several minutes, then delete the load-generator and wait for scale-down
**Expected:** REPLICAS climbs from 1 toward 5 as TARGETS exceeds 50%, then returns to 1 within approximately 5 minutes after stopping the load
**Why human:** Requires live cluster with Metrics Server scraping, real CPU load from the busybox wget loop, and time-based observation across multiple HPA reconciliation cycles

### 3. Local Dev v1-to-v2 Iteration

**Test:** Follow local-dev-workflow.md Steps 3-8 using a real Go environment and Docker
**Expected:** `curl http://localhost:8080` returns `Hello from v1!` after initial deploy, then returns `Hello from v2!` after the iteration loop (docker build v2, docker push, kubectl set image, rollout status)
**Why human:** Requires Docker building a real Go image from the Dockerfile, the cluster successfully pulling from localhost:5001, and `kubectl port-forward` proxying the request

### Sidebar Wiring

All three tutorials are wired in `astro.config.mjs` under the Guides group:

```
{ slug: 'guides/tls-web-app' },
{ slug: 'guides/hpa-auto-scaling' },
{ slug: 'guides/local-dev-workflow' },
```

The Astro build confirms all three rendered correctly:

```
/guides/tls-web-app/index.html
/guides/hpa-auto-scaling/index.html
/guides/local-dev-workflow/index.html
```

### Build Verification

`npm run build` completed with zero errors. Output confirmed `19 page(s) built in 1.66s` with all three guide pages present in the build output.

### Commit Verification

All commits documented in SUMMARY files exist in git history:

- `1e3cdcc6` — `docs(33-01): write TLS web app tutorial`
- `0a6c9091` — `docs(33-02): write HPA auto-scaling tutorial`
- `67a2901f` — `docs(33-02): write local dev workflow tutorial`

### Gaps Summary

No gaps. All four observable truths are verified, all three artifacts exist with substantive content well above minimum line thresholds, all eight key links are wired, all three requirements are satisfied, and the site builds with zero errors. Three human verification items are noted above for runtime behavior that cannot be verified programmatically.

---

_Verified: 2026-03-04T11:05:00Z_
_Verifier: Claude (gsd-verifier)_
