# Phase 33: Tutorials - Research

**Researched:** 2026-03-04
**Domain:** Starlight/Astro documentation authoring — end-to-end Kubernetes workflow tutorials
**Confidence:** HIGH

## Summary

Phase 33 is a pure documentation-writing phase. Three tutorial stub files already exist in `kinder-site/src/content/docs/guides/` as `:::note[Coming soon]` placeholders (created in Phase 30). The sidebar is already wired in `astro.config.mjs` with three slugs under the `Guides` group (`guides/tls-web-app`, `guides/hpa-auto-scaling`, `guides/local-dev-workflow`). No new files, no sidebar changes, no `npm install`, and no `astro.config.mjs` edits are needed.

The tutorials are end-to-end workflow guides — not reference pages. They combine multiple addons and guide a user through a complete task from cluster creation to verified outcome. Each tutorial must have: (1) clearly marked prerequisites, (2) every command copy-pasteable in sequence, and (3) expected output after key commands so users can self-diagnose. The content pattern follows the Phase 31/32 conventions: `:::tip/note/caution` callouts, `sh` code blocks for commands, bare fenced blocks for expected output, YAML code blocks for manifests.

The three tutorials address different user goals. The TLS tutorial (GUIDE-01) demonstrates the integration of Local Registry + Envoy Gateway + cert-manager + MetalLB — the showpiece "batteries included" story. The HPA tutorial (GUIDE-02) demonstrates Metrics Server-driven auto-scaling. The local dev workflow tutorial (GUIDE-03) demonstrates the tight code-build-push-deploy-iterate loop enabled by `localhost:5001`. All technical facts needed to write these tutorials have been sourced from the existing addon pages and project documentation, not inferred from general Kubernetes knowledge.

**Primary recommendation:** Replace all three placeholder `.md` files in-place. Each file replaces only its `:::note[Coming soon]` placeholder. No new files, no sidebar changes, no package changes.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| GUIDE-01 | User can follow a step-by-step tutorial to deploy a web app with TLS using Local Registry, Envoy Gateway, cert-manager, and MetalLB | All four addons are documented in addon pages; TLS Gateway pattern verified in cert-manager.md; MetalLB IP assignment verified in metallb.md |
| GUIDE-02 | User can follow a tutorial to set up HPA auto-scaling and watch pods scale under load using Metrics Server | HPA manifest pattern verified in metrics-server.md; load generation approach clarified below (kubectl run with busybox loop is dependency-free) |
| GUIDE-03 | User can follow a local dev workflow tutorial: code, build, push to localhost:5001, deploy, iterate | Local registry push/pull pattern verified in local-registry.md; iteration loop (rebuild + re-push + rollout restart) verified from existing docs |
</phase_requirements>

## Standard Stack

### Core (no changes needed)

| Tool | Version | Purpose | Note |
|------|---------|---------|------|
| Astro | 5.6.1 | Site framework | Already installed |
| @astrojs/starlight | 0.37.6 | Docs theme, callout syntax | Already installed |
| Markdown (.md) | — | Content format | All guide files are `.md` |

No `npm install` is required. No `astro.config.mjs` changes are needed. The sidebar Guides group is already wired with all three slugs.

### Content Conventions (inherited from Phase 31 and 32)

| Convention | Pattern | Source |
|------------|---------|--------|
| Callout types | `:::tip`, `:::note`, `:::caution`, `:::danger` | All existing addon pages |
| Callout with title | `:::tip[Title here]` | All existing pages |
| Shell code blocks | ` ```sh ` | All existing pages |
| YAML code blocks | ` ```yaml ` | All existing addon pages |
| Output blocks | ` ``` ` (bare) | All existing pages |
| Section heading depth | `##` top-level, `###` sub | All existing pages |
| Troubleshooting entry | `**Symptom:** / **Cause:** / **Fix:**` | MetalLB, Metrics Server, cert-manager pages |

### Alternatives Considered for Load Generation (GUIDE-02)

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `hey` (external binary) | `kubectl run` busybox loop | `hey` is not installed in kinder; `kubectl run` needs no extra tools and works inside the cluster |
| `wrk` (external binary) | `kubectl run` busybox loop | Same reasoning — zero external dependency |
| `k6` (external binary) | `kubectl run` busybox loop | `k6` is excellent but requires installation that breaks the self-contained tutorial flow |

**Decision:** Use `kubectl run` with a busybox or Alpine image to generate load inside the cluster. The command is a `while true; do wget -qO- http://<service>; done` loop. This is dependency-free and consistent with the quick-start.md pattern already established in the project.

## Architecture Patterns

### Recommended File Layout (no change)

```
kinder-site/src/content/docs/guides/
├── tls-web-app.md         # placeholder → full content (GUIDE-01)
├── hpa-auto-scaling.md    # placeholder → full content (GUIDE-02)
└── local-dev-workflow.md  # placeholder → full content (GUIDE-03)
```

### Pattern 1: Tutorial Page Structure

Every tutorial page MUST follow this section order:

```markdown
---
title: [Tutorial Name]
description: [description]
---

[One paragraph overview — what the user will build and what they will learn]

## Prerequisites

[Bullet list of what must be true before starting]

## [Step 1: Create the cluster / Setup]

[Commands + expected output]

## [Step 2: First major action]

[Commands + manifests + expected output]

## [Step N: Verify the result]

[The "it works" moment — curl, kubectl get, etc.]

## Clean up

[Delete resources + optionally delete cluster]
```

**Rationale:** Prerequisites first prevents users from getting halfway through and failing. Clean up at the end follows standard Kubernetes docs convention. The "verify the result" step must be unambiguous — users should know exactly what success looks like.

### Pattern 2: Prerequisite Section

```markdown
## Prerequisites

- kinder installed — see [Installation](/installation)
- Docker (or Podman) installed and running
- `kubectl` installed and on PATH
- No existing cluster named `kind` (or use `--name` to avoid conflicts)
```

Adapt per tutorial: GUIDE-01 needs `curl` and ability to accept self-signed certs (`-k`). GUIDE-02 needs no extra tools. GUIDE-03 needs Docker and a text editor.

### Pattern 3: Expected Output Blocks

After every command that produces meaningful output, include an "Expected output:" label followed by a bare fenced block:

```markdown
```sh
kubectl get certificate my-cert
```

Expected output:

```
NAME      READY   SECRET        AGE
my-cert   True    my-cert-tls   10s
```
```

This pattern is used consistently in quick-start.md and all addon pages. Follow it exactly.

### Pattern 4: macOS/Windows Platform Note

MetalLB LoadBalancer IPs are not directly reachable on macOS or Windows. The quick-start.md already handles this:

```markdown
:::note[macOS / Windows]
On macOS and Windows, LoadBalancer IPs are not directly reachable from the host. Use port-forward to access the service:

```sh
kubectl port-forward svc/<service> 8080:443
```
:::
```

Include this in GUIDE-01 (TLS tutorial) wherever the user needs to reach the gateway external IP.

### Anti-Patterns to Avoid

- **Requiring external tools not in the prerequisites:** If a step needs `hey`, `jq`, or any tool not in the prereqs, either add it to prereqs or use a different approach. Prefer `kubectl run` for load generation.
- **Missing expected output:** Every `kubectl get` / `curl` / `docker push` in a tutorial must have a corresponding expected output block. Users follow tutorials to learn what success looks like.
- **YAML in sh blocks:** All Kubernetes manifests go in `yaml` blocks. Commands go in `sh` blocks. Never mix.
- **Touching astro.config.mjs:** The sidebar is already wired. Do not modify it.
- **Creating new guide files:** Only replace the three existing placeholders.
- **Using `kind: Issuer` instead of `kind: ClusterIssuer`:** The kinder-provisioned issuer is `selfsigned-issuer` of kind `ClusterIssuer`. Using `kind: Issuer` causes `READY: False`. This is documented as the most common cert-manager mistake.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Load generation | Custom load script | `kubectl run` busybox loop | Already works inside cluster; no extra dependencies |
| TLS termination | Manual cert creation | cert-manager Certificate resource | Already set up; `selfsigned-issuer` ClusterIssuer is ready |
| Image hosting | External registry | `localhost:5001` local registry | Already running; no auth, no push secrets needed |
| Gateway routing | Ingress resources | Envoy Gateway HTTPRoute | Envoy Gateway is the kinder-standard approach |

**Key insight:** Every tutorial component is pre-installed by `kinder create cluster`. The tutorials' value is showing how the pre-installed pieces connect, not installing them.

## Verified Tutorial Content

This section documents the exact technical content for each tutorial, sourced from existing addon pages.

### GUIDE-01: TLS Web App Tutorial

**Flow:** Create cluster → push nginx image to local registry → create cert-manager Certificate → create MetalLB-backed Gateway with TLS → create HTTPRoute → verify HTTPS with curl

**Cluster creation command:**
```sh
kinder create cluster
```
All required addons (MetalLB, Envoy Gateway, cert-manager, Local Registry) are enabled by default. No `--profile` flag is needed.

**Step: Push image to local registry** (from local-registry.md):
```sh
docker pull nginx:alpine
docker tag nginx:alpine localhost:5001/myapp:v1
docker push localhost:5001/myapp:v1
```

**Step: Deploy the application:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
        - name: myapp
          image: localhost:5001/myapp:v1
---
apiVersion: v1
kind: Service
metadata:
  name: myapp
spec:
  selector:
    app: myapp
  ports:
    - port: 80
      targetPort: 80
```

**Step: Create the TLS certificate** (from cert-manager.md):
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: myapp-tls
  namespace: default
spec:
  secretName: myapp-tls
  issuerRef:
    name: selfsigned-issuer
    kind: ClusterIssuer
  dnsNames:
    - myapp.local
```

Wait for `READY: True`:
```sh
kubectl get certificate myapp-tls
```

Expected output:
```
NAME        READY   SECRET     AGE
myapp-tls   True    myapp-tls  15s
```

**Step: Create the TLS Gateway** (from cert-manager.md, envoy-gateway.md):
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: myapp-gateway
spec:
  gatewayClassName: eg
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        mode: Terminate
        certificateRefs:
          - name: myapp-tls
```

**Step: Create HTTPRoute** (from envoy-gateway.md):
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: myapp-route
spec:
  parentRefs:
    - name: myapp-gateway
  rules:
    - backendRefs:
        - name: myapp
          port: 80
```

**Step: Get the gateway IP and verify:**
```sh
kubectl get gateway myapp-gateway
```

Expected output shows `PROGRAMMED: True` and an external IP from MetalLB.

```sh
GATEWAY_IP=$(kubectl get gateway myapp-gateway -o jsonpath='{.status.addresses[0].value}')
curl -k https://$GATEWAY_IP
```

Expected output: nginx welcome page HTML.

**Platform note:** On macOS/Windows, use port-forward instead:
```sh
kubectl port-forward svc/envoy-myapp-gateway-<hash> 8443:443 -n envoy-gateway-system
curl -k https://localhost:8443
```

**Important timing note (from cert-manager.md):** cert-manager webhook takes 30-60 seconds to become ready after cluster creation. If `kubectl apply` for the Certificate fails immediately, wait 60 seconds and retry.

### GUIDE-02: HPA Auto-Scaling Tutorial

**Flow:** Create cluster → deploy app with CPU requests → create HPA → verify HPA reading metrics → generate load with kubectl run → watch pods scale up → stop load → watch scale down

**Cluster creation command:**
```sh
kinder create cluster
```
MetalLB and Metrics Server are both enabled by default.

**Step: Deploy app with resource requests** (from metrics-server.md — CRITICAL: must have requests.cpu):
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: php-apache
spec:
  replicas: 1
  selector:
    matchLabels:
      app: php-apache
  template:
    metadata:
      labels:
        app: php-apache
    spec:
      containers:
        - name: php-apache
          image: registry.k8s.io/hpa-example
          resources:
            requests:
              cpu: "200m"
            limits:
              cpu: "500m"
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: php-apache
spec:
  selector:
    app: php-apache
  ports:
    - port: 80
```

**Note on image choice:** `registry.k8s.io/hpa-example` is the standard Kubernetes HPA tutorial image. It responds to HTTP requests with CPU-intensive PHP computation, making it ideal for demonstrating CPU-based HPA. Alternative: use nginx with a CPU-burning init or just use `nginx` with an aggressive `averageUtilization: 30` and a small `requests.cpu: 50m`.

**Step: Create the HPA** (from metrics-server.md):
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: php-apache-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: php-apache
  minReplicas: 1
  maxReplicas: 5
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 50
```

**Step: Verify HPA is reading metrics** (wait 60 seconds for Metrics Server):
```sh
kubectl get hpa php-apache-hpa
```

Expected output:
```
NAME             REFERENCE              TARGETS   MINPODS   MAXPODS   REPLICAS   AGE
php-apache-hpa   Deployment/php-apache  8%/50%    1         5         1          90s
```

If `TARGETS` shows `<unknown>/50%`, wait 60 seconds — Metrics Server needs time to collect data.

**Step: Generate load** (no external tools needed):
```sh
kubectl run load-generator --image=busybox:1.28 --restart=Never -- \
  /bin/sh -c "while true; do wget -qO- http://php-apache; done"
```

**Step: Watch scaling** (in a separate terminal):
```sh
kubectl get hpa php-apache-hpa --watch
```

Users will see `REPLICAS` increase from 1 toward the max as CPU utilization rises above 50%.

**Step: Stop load and watch scale down:**
```sh
kubectl delete pod load-generator
```

Scale-down takes ~5 minutes by default (Kubernetes cool-down period). Mention this explicitly so users don't think something is broken.

**Important note from metrics-server.md:** "The deployment MUST have `resources.requests.cpu` set. Without it, the HPA cannot calculate CPU utilization and will show `unknown/50%`."

### GUIDE-03: Local Dev Workflow Tutorial

**Flow:** Create cluster → write a simple Go (or any) app → build Docker image → push to localhost:5001 → deploy to cluster → access the app → make a code change → rebuild/re-push → rolling update → verify new version

**Cluster creation:**
```sh
kinder create cluster
```
Local Registry enabled by default.

**Example app choice:** Use a minimal Go HTTP server or nginx-based example. The simplest is a single-file Go server that responds with a version string. This keeps the tutorial self-contained without requiring a separate GitHub repo. Alternatively, use `nginx` with a custom `index.html` ConfigMap to simulate iteration — but a Go server with a version string is clearer.

**Recommended concrete example — minimal Go server:**

`main.go`:
```go
package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "Hello from v1!")
    })
    http.ListenAndServe(":8080", nil)
}
```

`Dockerfile`:
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go build -o server main.go

FROM alpine:latest
COPY --from=builder /app/server /server
EXPOSE 8080
CMD ["/server"]
```

**Step: Build and push v1:**
```sh
docker build -t localhost:5001/myapp:v1 .
docker push localhost:5001/myapp:v1
```

Expected output shows push digest:
```
v1: digest: sha256:... size: 1234
```

**Step: Deploy:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
        - name: myapp
          image: localhost:5001/myapp:v1
---
apiVersion: v1
kind: Service
metadata:
  name: myapp
spec:
  selector:
    app: myapp
  ports:
    - port: 80
      targetPort: 8080
  type: ClusterIP
```

**Step: Access the app** (use port-forward — works on all platforms):
```sh
kubectl port-forward svc/myapp 8080:80
curl http://localhost:8080
```

Expected output:
```
Hello from v1!
```

**Step: Make a change and iterate** — change the response to "Hello from v2!", rebuild, push, rollout:
```sh
# Edit main.go: change "v1" to "v2"
docker build -t localhost:5001/myapp:v2 .
docker push localhost:5001/myapp:v2
kubectl set image deployment/myapp myapp=localhost:5001/myapp:v2
kubectl rollout status deployment/myapp
```

Expected output:
```
deployment "myapp" successfully rolled out
```

**Step: Verify the new version:**
```sh
curl http://localhost:8080
```

Expected output:
```
Hello from v2!
```

**Tip on imagePullPolicy:** The tutorial should note that `localhost:5001` images use `imagePullPolicy: Always` implicitly when using a tag other than `:latest` — actually, Kubernetes defaults to `IfNotPresent` for non-`:latest` tags. When iterating with versioned tags (`:v1`, `:v2`), use a new tag each time OR set `imagePullPolicy: Always`. Using `:latest` with `imagePullPolicy: Always` also works but can cause accidental downgrades. The cleanest pattern: use versioned tags (`:v1`, `:v2`) and `kubectl set image` to update.

## Common Pitfalls

### Pitfall 1: Certificate Webhook Not Ready

**What goes wrong:** The `kubectl apply -f certificate.yaml` command fails with a webhook timeout error immediately after cluster creation.
**Why it happens:** cert-manager's webhook pod takes 30-60 seconds to become ready after cluster creation. Applying Certificate resources before the webhook is up causes validation rejection.
**How to avoid:** In the TLS tutorial, add a `kubectl get pods -n cert-manager` step with expected output showing all 3 pods `Running` BEFORE applying any Certificate resources. Alternatively, add an explicit "wait 60 seconds" instruction.
**Warning signs:** Error message mentions `webhook` or `connection refused` when applying a cert-manager resource.

### Pitfall 2: HPA Shows `<unknown>/50%`

**What goes wrong:** The HPA TARGETS column shows `<unknown>/50%` instead of a real percentage.
**Why it happens:** Either (a) the deployment has no `resources.requests.cpu`, or (b) Metrics Server hasn't had time to collect data yet (needs 60+ seconds).
**How to avoid:** In the HPA tutorial, the deployment manifest MUST have `resources.requests.cpu` set (verified from metrics-server.md). Include a "wait 60 seconds" instruction after creating the HPA before checking the TARGETS column.
**Warning signs:** `<unknown>/50%` in TARGETS column.

### Pitfall 3: LoadBalancer IP Not Reachable on macOS/Windows

**What goes wrong:** `curl https://$GATEWAY_IP` hangs or connection refused on macOS or Windows.
**Why it happens:** MetalLB assigns IPs inside the Docker VM network, which is not directly reachable from the host on macOS/Windows.
**How to avoid:** Always include a `:::note[macOS / Windows]` callout in the TLS tutorial (and anywhere an external IP is accessed) with the port-forward alternative.
**Warning signs:** External IP assigned but `curl` hangs.

### Pitfall 4: `kind: Issuer` Instead of `kind: ClusterIssuer`

**What goes wrong:** Certificate stays `READY: False` indefinitely.
**Why it happens:** `selfsigned-issuer` is a `ClusterIssuer`, not a namespace-scoped `Issuer`. This is the most common cert-manager mistake (documented in cert-manager.md).
**How to avoid:** All tutorial cert-manager YAML must use `kind: ClusterIssuer` in `issuerRef`. Include a `:::caution` noting that `kind: Issuer` will not work.
**Warning signs:** `kubectl describe certificate` shows "issuer not found" or similar.

### Pitfall 5: Image Pull Fails from localhost:5001

**What goes wrong:** Pod goes to `ImagePullBackOff` with `Failed to pull image "localhost:5001/myapp:v1"`.
**Why it happens:** The `kind-registry` container is not running — it may have been stopped or removed.
**How to avoid:** Include a registry verification step early in GUIDE-01 and GUIDE-03 tutorials: `docker ps --filter name=kind-registry`.
**Warning signs:** `ImagePullBackOff` in pod status.

### Pitfall 6: Scale-Down Looks Broken

**What goes wrong:** After stopping load in GUIDE-02, pods don't scale back down for 5+ minutes. Users think something failed.
**Why it happens:** Kubernetes has a 5-minute scale-down stabilization window by default to prevent flapping.
**How to avoid:** Explicitly mention in the tutorial that scale-down takes 5 minutes by default. This is expected behavior, not a bug.
**Warning signs:** Pod count stays elevated after load generator is deleted.

### Pitfall 7: Gateway Name Suffix for Port-Forward

**What goes wrong:** On macOS/Windows, users can't figure out the Envoy Gateway service name for port-forward.
**Why it happens:** Envoy Gateway creates services named `envoy-<namespace>-<gateway-name>-<hash>` in the `envoy-gateway-system` namespace. The hash suffix is generated and not predictable in advance.
**How to avoid:** In the TLS tutorial macOS/Windows note, show how to discover the service name: `kubectl get svc -n envoy-gateway-system`. Then use that service name for port-forward.
**Warning signs:** Port-forward command fails with "service not found".

## Code Examples

All examples sourced from existing project documentation.

### Cluster creation (all tutorials)

```sh
# Source: quick-start.md
kinder create cluster
```

### Verify all addons ready (GUIDE-01, GUIDE-03)

```sh
# Source: quick-start.md pattern
kubectl get pods -n cert-manager     # all 3 Running
kubectl get pods -n envoy-gateway-system  # Running
kubectl get pods -n metallb-system   # Running
docker ps --filter name=kind-registry  # registry:2 container
```

### Push to local registry (GUIDE-01, GUIDE-03)

```sh
# Source: local-registry.md
docker build -t localhost:5001/myapp:v1 .
docker push localhost:5001/myapp:v1
```

### cert-manager Certificate (GUIDE-01)

```yaml
# Source: cert-manager.md
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: myapp-tls
  namespace: default
spec:
  secretName: myapp-tls
  issuerRef:
    name: selfsigned-issuer
    kind: ClusterIssuer    # MUST be ClusterIssuer, not Issuer
  dnsNames:
    - myapp.local
```

### TLS Gateway (GUIDE-01)

```yaml
# Source: cert-manager.md (Use with Envoy Gateway section)
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: myapp-gateway
spec:
  gatewayClassName: eg
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        mode: Terminate
        certificateRefs:
          - name: myapp-tls
```

### HPA Deployment with CPU requests (GUIDE-02)

```yaml
# Source: metrics-server.md (Set up a Horizontal Pod Autoscaler section)
# CRITICAL: resources.requests.cpu MUST be set or HPA shows <unknown>/50%
resources:
  requests:
    cpu: "200m"
  limits:
    cpu: "500m"
```

### HPA resource (GUIDE-02)

```yaml
# Source: metrics-server.md
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
spec:
  minReplicas: 1
  maxReplicas: 5
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 50
```

### Load generation inside cluster (GUIDE-02)

```sh
# Dependency-free — uses only kubectl and busybox
kubectl run load-generator --image=busybox:1.28 --restart=Never -- \
  /bin/sh -c "while true; do wget -qO- http://php-apache; done"
```

### Rolling update (GUIDE-03)

```sh
kubectl set image deployment/myapp myapp=localhost:5001/myapp:v2
kubectl rollout status deployment/myapp
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Ingress resources | Envoy Gateway HTTPRoute | kinder v1.3+ | Gateway API is now the standard routing abstraction in kinder |
| External registry (Docker Hub) | `localhost:5001` local registry | kinder v1.3+ | No auth, no rate limits, faster iteration |
| Manual cert creation | cert-manager ClusterIssuer | kinder v1.3+ | TLS setup reduced to one Certificate resource |
| `autoscaling/v1` HPA | `autoscaling/v2` HPA | Kubernetes 1.23+ | `autoscaling/v2` supports multiple metrics types; use this |

## Open Questions

1. **Image for HPA tutorial — `registry.k8s.io/hpa-example` vs nginx**
   - What we know: `registry.k8s.io/hpa-example` is the standard k8s.io HPA example image that generates CPU load from HTTP requests. It requires pulling from `registry.k8s.io` (external, not the local registry). This is fine since the tutorial's goal is demonstrating HPA, not the local registry.
   - What's unclear: Whether `registry.k8s.io/hpa-example` is still maintained and pullable in 2026.
   - Recommendation: Use `registry.k8s.io/hpa-example` as primary choice (it's the standard k8s tutorial image for HPA). Mention nginx as an alternative with `averageUtilization: 30` and `requests.cpu: 50m` if the image is unavailable.

2. **Port-forward service name for GUIDE-01 macOS/Windows**
   - What we know: Envoy Gateway creates services in `envoy-gateway-system` with names like `envoy-default-myapp-gateway-<hash>`. The hash is generated at runtime.
   - What's unclear: Whether the hash is stable enough to show a fixed example, or whether we need to show the `kubectl get svc` discovery command.
   - Recommendation: Show `kubectl get svc -n envoy-gateway-system` and instruct the user to look for a service starting with `envoy-default-myapp-gateway-`. This is more robust than showing a fixed name.

3. **Go vs nginx for GUIDE-03 example app**
   - What we know: A Go server shows the "build from source" loop most clearly. However, it requires Go installed on the user's machine. Nginx with a ConfigMap avoids this but the "iteration" step (editing the ConfigMap and rolling out) is less intuitive than rebuilding an image.
   - Recommendation: Use nginx with a custom `index.html` mounted from a ConfigMap for the static "hello world" — then the iteration step is: edit the ConfigMap, trigger rollout restart (`kubectl rollout restart deployment/myapp`). This requires no external tools beyond Docker. Include the Go Dockerfile version as a code block with a note that users with Go installed can use it instead.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | None (documentation phase — no automated tests) |
| Config file | N/A |
| Quick run command | `cd /Users/patrykattc/work/git/kinder/kinder-site && npm run build 2>&1 \| tail -5` |
| Full suite command | `cd /Users/patrykattc/work/git/kinder/kinder-site && npm run build` |

### Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| GUIDE-01 | tls-web-app.md covers cluster creation through HTTPS verification | manual | `grep -c "cert-manager\|Gateway\|localhost:5001\|https" kinder-site/src/content/docs/guides/tls-web-app.md` | ✅ placeholder |
| GUIDE-02 | hpa-auto-scaling.md covers HPA creation, load generation, and watching scaling | manual | `grep -c "HorizontalPodAutoscaler\|load-generator\|REPLICAS\|scale" kinder-site/src/content/docs/guides/hpa-auto-scaling.md` | ✅ placeholder |
| GUIDE-03 | local-dev-workflow.md covers build-push-deploy-iterate loop | manual | `grep -c "localhost:5001\|docker build\|docker push\|kubectl set image" kinder-site/src/content/docs/guides/local-dev-workflow.md` | ✅ placeholder |

The build check (`npm run build`) is the primary automated gate — it catches frontmatter errors, broken internal links, and Starlight callout syntax errors.

### Sampling Rate

- **Per task commit:** `cd /Users/patrykattc/work/git/kinder/kinder-site && npm run build 2>&1 | tail -10`
- **Per wave merge:** `cd /Users/patrykattc/work/git/kinder/kinder-site && npm run build`
- **Phase gate:** Full build green before `/gsd:verify-work`

### Wave 0 Gaps

None — existing test infrastructure (Astro build) covers all phase requirements. No new test files needed.

## Sources

### Primary (HIGH confidence)

- `kinder-site/src/content/docs/addons/cert-manager.md` — TLS Certificate pattern, ClusterIssuer name (`selfsigned-issuer`), Gateway TLS listener example, cert-manager webhook timing (30-60s), most common mistake (kind: Issuer vs ClusterIssuer)
- `kinder-site/src/content/docs/addons/envoy-gateway.md` — GatewayClass name (`eg`), HTTPRoute pattern, Gateway `Programmed` status, server-side apply note
- `kinder-site/src/content/docs/addons/metallb.md` — IP range (`.200-.250`), macOS/Windows LoadBalancer limitation, port-forward alternative
- `kinder-site/src/content/docs/addons/metrics-server.md` — HPA manifest (autoscaling/v2), `resources.requests.cpu` requirement, `<unknown>/50%` pitfall, 60s metric collection delay
- `kinder-site/src/content/docs/addons/local-registry.md` — push/pull pattern, `localhost:5001`, `kind-registry` container, imagePullPolicy behavior
- `kinder-site/src/content/docs/quick-start.md` — macOS/Windows port-forward pattern, addon verification sequence
- `kinder-site/astro.config.mjs` — Sidebar Guides group already wired with all 3 slugs; no changes needed
- `kinder-site/src/content/docs/guides/*.md` — All 3 placeholder files confirmed (:::note[Coming soon] only)
- `kinder-site/package.json` — Astro 5.6.1, @astrojs/starlight 0.37.6

### Secondary (MEDIUM confidence)

- Phase 31 RESEARCH.md — Content conventions, callout syntax, Symptom/Cause/Fix troubleshooting pattern
- Phase 32 RESEARCH.md — Plan structure template, verified source data methodology
- Phase 32 PLAN.md — Task structure template for documentation replacement tasks

### Tertiary (LOW confidence)

- `registry.k8s.io/hpa-example` availability — this image is referenced in official Kubernetes HPA docs but its continued availability in 2026 has not been verified by pulling it. If unavailable, use nginx with reduced CPU thresholds.

## Metadata

**Confidence breakdown:**
- File structure and sidebar: HIGH — read directly from `astro.config.mjs` and confirmed all 3 placeholder files exist
- Content conventions: HIGH — read from all existing addon pages and Phase 31/32 research
- TLS tutorial technical content: HIGH — sourced from cert-manager.md, envoy-gateway.md, metallb.md, local-registry.md
- HPA tutorial technical content: HIGH — sourced from metrics-server.md; HPA manifest pattern and `<unknown>/50%` pitfall verified from project docs
- Local dev workflow content: HIGH — sourced from local-registry.md; kubectl set image pattern is standard Kubernetes
- `registry.k8s.io/hpa-example` availability: LOW — image referenced in official k8s docs but not verified pullable in current environment

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (documentation phase; all facts from stable project files; Astro/Starlight versions locked in package.json)
