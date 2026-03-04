# Phase 31: Addon Page Depth - Research

**Researched:** 2026-03-04
**Domain:** Starlight/Astro documentation authoring — Kubernetes addon content enrichment
**Confidence:** HIGH

## Summary

Phase 31 is a pure documentation-writing phase. There is no new code, no new libraries, and no configuration changes. The work is to enrich seven existing addon pages in `kinder-site/src/content/docs/addons/` with practical examples and troubleshooting sections. Each page already has a solid "what gets installed / how to verify / configuration / how to disable" skeleton from earlier phases. The gap is that no page has a troubleshooting section and most pages lack concrete "do something useful" examples beyond the minimum verify flow.

The content authoring environment is Astro 5.6 + Starlight 0.37.6, with Markdown files and a fixed set of callout types (`:::tip`, `:::note`, `:::caution`, `:::danger`). All content lives in `kinder-site/src/content/docs/addons/`. The sidebar is already wired in `astro.config.mjs` — no sidebar changes are needed for this phase.

Content quality for this phase means: every page should answer "does this work?" (verification), "how do I actually use it?" (practical example), and "why doesn't it work?" (troubleshooting). The success criteria are specific about minimum content — one troubleshooting entry per page covering the most common failure, and concrete examples matching each requirement.

**Primary recommendation:** Enrich each of the 7 addon `.md` files in-place. Add a `## Practical Examples` section and a `## Troubleshooting` section to each page. Follow the callout and code-block patterns already established in the codebase. No new files, no config changes.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| ADDON-01 | MetalLB page has practical examples (custom services, LB vs NodePort guidance) and troubleshooting | MetalLB page currently has verify flow only; needs CreateService with custom IP + NodePort comparison + troubleshooting section |
| ADDON-02 | Envoy Gateway page has routing examples (path, header) and troubleshooting | Page has a single path-prefix route; needs path-based + header-based HTTPRoute examples + troubleshooting |
| ADDON-03 | Metrics Server page has kubectl top examples, basic HPA reference, and troubleshooting | Page has `kubectl top nodes` verify; needs `kubectl top pods` example, HPA manifest, and troubleshooting |
| ADDON-04 | CoreDNS page has DNS verification examples and troubleshooting | Page has ConfigMap inspect only; needs DNS lookup commands (`nslookup`/`dig` from a pod) + troubleshooting |
| ADDON-05 | Headlamp page has dashboard navigation guide and troubleshooting | Page has port-forward + token steps; needs navigation guide (what to look at) + troubleshooting |
| ADDON-06 | Local Registry page has multi-image workflow, cleanup, and troubleshooting | Page has basic push/pull; needs tag-multiple-images + cleanup + troubleshooting |
| ADDON-07 | cert-manager page has additional certificate examples and troubleshooting | Page has one Certificate + Gateway combo; needs CA issuer or wildcard example + troubleshooting |
</phase_requirements>

## Standard Stack

### Core (no changes needed)

| Tool | Version | Purpose | Note |
|------|---------|---------|------|
| Astro | 5.6.1 | Site framework | Already installed, no change |
| @astrojs/starlight | 0.37.6 | Docs theme + callouts | Already installed, no change |
| Markdown (.md) | — | Content format | All addon pages are `.md` |

No package installation is required for this phase. The entire work is editing seven `.md` files.

### Content Conventions (established in codebase)

| Convention | Pattern | Source |
|------------|---------|--------|
| Callout types | `:::tip`, `:::note`, `:::caution`, `:::danger` | Starlight 0.37.6 |
| Callout with title | `:::tip[Title goes here]` | All existing addon pages |
| Shell code blocks | ` ```sh ` | All existing addon pages |
| YAML code blocks | ` ```yaml ` | All existing addon pages |
| Output blocks | ` ``` ` (bare) | All existing addon pages |
| Section heading depth | `##` for top-level, `###` for sub | All existing pages |

## Architecture Patterns

### Recommended Content Structure Per Page

Each addon page should follow this section order after this phase:

```
---
title: [Addon Name]
description: [description]
---

[Intro paragraph — what it does, version installed]

## What gets installed
[Table]

## How to use              ← practical examples go here (rename from "How to verify" if needed)
[Practical examples section — specific to each addon]

## How to verify           ← keep existing verify steps
[Existing verify commands]

## Configuration
[Existing config YAML]

## How to disable
[Existing disable YAML]

## Troubleshooting          ← NEW section for every page
[Table or subsections with symptom → cause → fix]

## Platform notes / Technical notes   ← keep if exists
[Existing platform caveats]
```

**Note:** The "Practical Examples" content may be placed as a new `## Practical Examples` section OR woven into an enriched `## How to use` section — whichever reads more naturally for each addon. The troubleshooting section MUST be a dedicated `## Troubleshooting` heading.

### Pattern 1: Troubleshooting Section Structure

Use a consistent subsection pattern for troubleshooting:

```markdown
## Troubleshooting

### [Symptom description]

**Symptom:** [what the user sees — error message, command output]
**Cause:** [why this happens]
**Fix:** [concrete command or config to resolve it]

```sh
[command to diagnose or fix]
```
```

If there is only one troubleshooting entry (the minimum requirement), a flat structure without `###` subheadings is also acceptable:

```markdown
## Troubleshooting

**Symptom:** ...
**Cause:** ...
**Fix:** ...
```

### Pattern 2: Practical Example with Context

Every practical example should have:
1. A sentence explaining *when* or *why* to do this
2. The YAML or commands
3. Expected output (use bare ` ``` ` blocks)
4. A cleanup command if resources were created

```markdown
### Create a LoadBalancer service with a custom IP

To request a specific IP from the MetalLB pool, use the `metallb.universe.tf/loadBalancerIPs` annotation:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    metallb.universe.tf/loadBalancerIPs: 172.20.255.210
spec:
  type: LoadBalancer
  selector:
    app: my-app
  ports:
    - port: 80
```

```sh
kubectl apply -f service.yaml
kubectl get svc my-service
```

Expected output:

```
NAME         TYPE           CLUSTER-IP    EXTERNAL-IP      PORT(S)   AGE
my-service   LoadBalancer   10.96.4.12    172.20.255.210   80/TCP    5s
```
```

### Anti-Patterns to Avoid

- **Adding new sidebar entries:** The sidebar in `astro.config.mjs` already covers all 7 addon slugs. Do not touch `astro.config.mjs`.
- **Creating new files:** All content goes into the existing 7 `.md` files. No new pages.
- **Stub troubleshooting entries:** Every troubleshooting entry must have a concrete symptom, cause, and fix — not placeholder text.
- **Inventing non-existent behavior:** All commands and YAML examples must reflect actual kinder/addon behavior. Cross-check against the existing "What gets installed" tables.
- **Duplicate verify steps:** Practical examples can overlap with verify steps but should extend them, not copy them verbatim.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Callout boxes | Custom HTML/CSS | Starlight `:::tip/note/caution/danger` syntax | Already in the theme, renders correctly in dark mode |
| Code tabs | Custom components | Separate `###` subsections | No tabs component established in project; separate sections are simpler and already the pattern |
| Troubleshooting tables | Symptom/Cause/Fix as a table | Prose subsections with bold labels | Tables render poorly for multi-line content; existing pages use prose |

## Common Pitfalls

### Pitfall 1: MetalLB annotation name varies by version

**What goes wrong:** The annotation for requesting a specific IP changed between MetalLB versions. Older docs use `metallb.universe.tf/address-pool`, newer versions use `metallb.universe.tf/loadBalancerIPs`.

**Why it happens:** MetalLB v0.13+ deprecated `address-pool` in favor of `loadBalancerIPs`. kinder installs v0.15.3, so `loadBalancerIPs` is correct.

**How to avoid:** Use `metallb.universe.tf/loadBalancerIPs` for the annotation in examples. Do not reference `address-pool`.

**Warning signs:** If the service does not receive the requested IP, check the annotation name.

### Pitfall 2: Envoy Gateway HTTPRoute path matching has two types

**What goes wrong:** HTTPRoute `path.type` can be `PathPrefix` or `Exact`. Using `Exact` for a prefix match silently fails (returns 404 for sub-paths).

**Why it happens:** Gateway API path matching is strict. `PathPrefix: /api` matches `/api/users` but `Exact: /api` does not.

**How to avoid:** Show both types in examples with a clear label on which is which.

**Warning signs:** Routes that worked at root path fail for nested paths.

### Pitfall 3: HPA requires a resource request to function

**What goes wrong:** An HPA targeting CPU utilization does nothing if the deployment's containers do not have `resources.requests.cpu` set.

**Why it happens:** Metrics Server calculates utilization as `current usage / requested`. Without a request, the denominator is undefined.

**How to avoid:** The HPA example MUST include a deployment with `resources.requests.cpu` and `resources.limits.cpu` set.

**Warning signs:** `kubectl describe hpa` shows `unknown/50%` for the metric.

### Pitfall 4: Headlamp token field confusion

**What goes wrong:** Users paste the raw base64-encoded token (the `.data.token` field) instead of the decoded value.

**Why it happens:** `kubectl get secret -o jsonpath='{.data.token}'` returns base64. The `| base64 --decode` step is easy to miss.

**How to avoid:** The troubleshooting entry for Headlamp should explicitly state "ensure you decoded the token with `| base64 --decode`".

**Warning signs:** Headlamp shows "Invalid token" or "Unauthorized" after pasting.

### Pitfall 5: Local Registry images not found in pods

**What goes wrong:** `kubectl run` with `image: localhost:5001/myapp:latest` produces `ImagePullBackOff`.

**Why it happens:** The containerd `hosts.toml` is written to nodes during cluster creation. If the registry container was stopped and restarted (different container ID), nodes may not resolve it correctly.

**How to avoid:** Troubleshooting entry should include `docker ps --filter name=kind-registry` to confirm the registry is running.

**Warning signs:** `kubectl describe pod` shows `Failed to pull image "localhost:5001/..."`.

### Pitfall 6: cert-manager Certificate stays NotReady

**What goes wrong:** `kubectl get certificate` shows `READY: False` indefinitely.

**Why it happens:** Most common causes: (a) webhook not yet ready (wait 30–60s after cluster creation), (b) `issuerRef` references a ClusterIssuer but `kind: Issuer` is used in the spec.

**How to avoid:** Troubleshooting entry should cover both the timing issue and the Issuer vs ClusterIssuer distinction. The kinder-installed issuer is `ClusterIssuer` named `selfsigned-issuer`.

**Warning signs:** `kubectl describe certificate my-cert` shows `Waiting for CertificateRequest`.

### Pitfall 7: CoreDNS DNS lookup from pods requires a running pod

**What goes wrong:** Running `nslookup kubernetes.default` from the host returns nothing useful (host DNS, not cluster DNS).

**Why it happens:** Cluster DNS is only accessible from inside pods. Host `nslookup` hits the host resolver, not CoreDNS.

**How to avoid:** DNS verification examples must use `kubectl run` with a busybox/dnsutils image to exec into a pod. Show the `kubectl run --rm -it` pattern.

**Warning signs:** User runs nslookup from host and gets unexpected results.

## Code Examples

Verified patterns from kinder codebase and official addon docs:

### MetalLB: LoadBalancer service with specific IP

```yaml
# Source: MetalLB v0.15 docs — metallb.universe.tf/loadBalancerIPs annotation
apiVersion: v1
kind: Service
metadata:
  name: web
  annotations:
    metallb.universe.tf/loadBalancerIPs: 172.20.255.210
spec:
  type: LoadBalancer
  selector:
    app: web
  ports:
    - port: 80
      targetPort: 8080
```

### MetalLB: NodePort as alternative (when to use)

```markdown
### When to use NodePort instead

Use `NodePort` instead of `LoadBalancer` when:
- Running on rootless Podman (ARP advertisement doesn't work)
- You only need local host access (port-forward is simpler)
- You're testing on macOS/Windows where LB IPs are not directly reachable

```sh
kubectl expose deployment my-app --type=NodePort --port=80
kubectl get svc my-app
# Access via: http://localhost:<node-port>
```
```

### Envoy Gateway: Path-based routing

```yaml
# Source: Gateway API spec — gateway.networking.k8s.io/v1
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: path-routing
spec:
  parentRefs:
    - name: my-gateway
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api
      backendRefs:
        - name: api-service
          port: 8080
    - matches:
        - path:
            type: PathPrefix
            value: /web
      backendRefs:
        - name: web-service
          port: 80
```

### Envoy Gateway: Header-based routing

```yaml
# Source: Gateway API spec — HTTPHeaderMatch
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: header-routing
spec:
  parentRefs:
    - name: my-gateway
  rules:
    - matches:
        - headers:
            - name: X-Environment
              value: canary
      backendRefs:
        - name: canary-service
          port: 80
    - backendRefs:
        - name: stable-service
          port: 80
```

### Metrics Server: kubectl top pods with namespace

```sh
# All pods in all namespaces
kubectl top pods -A

# Pods in a specific namespace, sorted by CPU
kubectl top pods -n default --sort-by=cpu
```

### Metrics Server: Basic HPA manifest (requires resource requests)

```yaml
# Deployment MUST have resources.requests.cpu set for HPA to work
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: my-app
          image: nginx
          resources:
            requests:
              cpu: "100m"
            limits:
              cpu: "200m"
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-app-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
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

### CoreDNS: DNS verification from inside a pod

```sh
# Run a one-shot pod with nslookup available
kubectl run dns-test --image=busybox:1.36 --rm -it --restart=Never -- nslookup kubernetes.default
```

Expected output:

```
Server:    10.96.0.10
Address 1: 10.96.0.10 kube-dns.kube-system.svc.cluster.local

Name:      kubernetes.default
Address 1: 10.96.0.1 kubernetes.default.svc.cluster.local
```

### Local Registry: Multi-image workflow

```sh
# Build and push multiple images
docker build -t localhost:5001/frontend:v1 ./frontend
docker push localhost:5001/frontend:v1

docker build -t localhost:5001/backend:v1 ./backend
docker push localhost:5001/backend:v1

# Verify images are in the registry
curl http://localhost:5001/v2/_catalog
```

Expected output:
```json
{"repositories":["frontend","backend"]}
```

### Local Registry: Cleanup

```sh
# Remove a specific image from the registry
curl -X DELETE http://localhost:5001/v2/myapp/manifests/<digest>

# Or simply re-push with the same tag to overwrite
docker push localhost:5001/myapp:latest
```

### cert-manager: CA-signed Certificate (using kinder's selfsigned-issuer)

```yaml
# Source: cert-manager.io/v1 Certificate API
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: wildcard-cert
  namespace: default
spec:
  secretName: wildcard-cert-tls
  issuerRef:
    name: selfsigned-issuer
    kind: ClusterIssuer
  commonName: "*.example.local"
  dnsNames:
    - "*.example.local"
    - "example.local"
  duration: 2160h    # 90 days
  renewBefore: 360h  # 15 days before expiry
```

### cert-manager: Troubleshooting certificate not ready

```sh
# Check certificate status and events
kubectl describe certificate my-cert -n default

# Check the CertificateRequest created by cert-manager
kubectl get certificaterequest -n default

# Check cert-manager controller logs for errors
kubectl logs -n cert-manager deployment/cert-manager | tail -20
```

## What Each Page Needs

Gap analysis — what is missing from each page vs. requirements:

### metallb.md (ADDON-01)

**Has:** What gets installed, basic verify flow (nginx deploy + expose), configuration, disable, Podman caution.
**Needs:**
- `## Practical Examples` section with: (a) service with custom IP annotation, (b) explicit NodePort guidance with when-to-use explanation
- `## Troubleshooting` section with at minimum: IP stays `<pending>` — symptom, cause (`IPAddressPool` exhausted or MetalLB not running), fix

### envoy-gateway.md (ADDON-02)

**Has:** Basic Gateway + HTTPRoute (path prefix to one service), verify steps, configuration, disable, server-side apply note.
**Needs:**
- `## Practical Examples` section with: (a) path-based routing (two services on different paths), (b) header-based routing
- `## Troubleshooting` section with at minimum: Gateway stays `Pending` or HTTPRoute not matching — cause and fix

### metrics-server.md (ADDON-03)

**Has:** `kubectl top nodes` verify, configuration, disable.
**Needs:**
- `## Practical Examples` section with: (a) `kubectl top pods -A`, (b) complete HPA manifest (with deployment that has resource requests)
- `## Troubleshooting` section with at minimum: HPA shows `unknown/50%` — cause (missing resource requests) and fix

### coredns.md (ADDON-04)

**Has:** ConfigMap before/after comparison, ConfigMap inspect verify, configuration, disable.
**Needs:**
- `## Practical Examples` section with: (a) DNS lookup from inside a pod using `busybox` (b) short-name resolution test showing autopath in action
- `## Troubleshooting` section with at minimum: DNS resolution failure from inside pod — cause and fix

### headlamp.md (ADDON-05)

**Has:** Port-forward command, token retrieval, pod verify, secret verify, configuration, disable.
**Needs:**
- `## Navigation Guide` (or `## What you can do`) section describing main areas: workloads, events, logs, config maps — 3-5 items users will actually use
- `## Troubleshooting` section with at minimum: "Invalid token" error — cause (raw base64 not decoded) and fix

### local-registry.md (ADDON-06)

**Has:** How it works, basic push/pull cycle, verify (docker ps + regtest pod), configuration, disable, dev tool integration, Podman caution.
**Needs:**
- `## Multi-image workflow` subsection showing building and pushing multiple images + `curl /v2/_catalog` to verify
- Cleanup commands (how to remove or overwrite images)
- `## Troubleshooting` section with at minimum: `ImagePullBackOff` from localhost:5001 — cause (registry stopped) and fix (`docker start kind-registry`)

### cert-manager.md (ADDON-07)

**Has:** Self-signed certificate example, Gateway TLS integration, verify, configuration, disable, server-side apply note.
**Needs:**
- Additional certificate example: wildcard cert or multi-SAN cert to show flexibility beyond single hostname
- `## Troubleshooting` section with at minimum: `READY: False` on certificate — two causes (webhook timing, Issuer vs ClusterIssuer kind) and fix

## State of the Art

| Area | Current State | What This Phase Changes |
|------|---------------|------------------------|
| Addon pages | Basic verify/config/disable skeleton | Add practical examples + troubleshooting per requirement |
| Starlight callouts | `:::tip`, `:::note`, `:::caution` in use | Continue using same callout types |
| HPA API version | `autoscaling/v2` (stable since k8s 1.26) | Use `autoscaling/v2` — not the deprecated `v2beta2` |
| Gateway API version | `gateway.networking.k8s.io/v1` (stable since 1.0.0) | Use `v1` — not `v1beta1` |
| MetalLB annotation | `metallb.universe.tf/loadBalancerIPs` (v0.13+) | Use current annotation name |

## Open Questions

1. **Local Registry cleanup mechanics**
   - What we know: The `registry:2` image supports the Docker Registry HTTP API v2 with `DELETE /v2/<name>/manifests/<digest>`
   - What's unclear: Whether garbage collection is enabled by default in the kinder registry setup (no explicit registry config is mounted)
   - Recommendation: Show the re-push-to-overwrite pattern as primary cleanup; mention DELETE as advanced. Do not promise garbage collection runs automatically.

2. **Headlamp navigation guide depth**
   - What we know: ADDON-05 requires a "dashboard navigation guide"
   - What's unclear: Whether this should describe specific Headlamp screens/views by name or just list categories
   - Recommendation: Keep it high-level (3-5 bullets on main areas: Workloads, Events log, Pod logs, ConfigMaps/Secrets). A full UI walkthrough is out of scope for a reference doc page.

3. **CoreDNS autopath observable behavior**
   - What we know: `autopath @kubernetes` reduces search-domain retries for short names
   - What's unclear: Whether there is a simple `nslookup` test that visibly demonstrates the autopath improvement vs. not having it
   - Recommendation: Show standard `nslookup kubernetes.default` as the verify example. Note that autopath eliminates the NXDOMAIN retry chain but this is not easily observable in output. Focus on the ConfigMap presence check as primary verification.

## Sources

### Primary (HIGH confidence)
- Existing `kinder-site/src/content/docs/addons/*.md` files — current content baseline
- `kinder-site/astro.config.mjs` — sidebar structure, no changes needed
- `kinder-site/package.json` — Starlight 0.37.6, Astro 5.6.1

### Secondary (MEDIUM confidence)
- MetalLB v0.15 docs (metallb.universe.tf) — annotation names, IPAddressPool behavior
- Gateway API spec (gateway.networking.k8s.io) — HTTPRoute path/header match types
- cert-manager.io/v1 Certificate API — duration, renewBefore fields
- Kubernetes HPA docs — `autoscaling/v2` as stable API, resource requests requirement

### Tertiary (LOW confidence)
- Local Registry garbage collection behavior — not confirmed from official docs; treat re-push as the safe cleanup path

## Metadata

**Confidence breakdown:**
- Content gaps (what each page needs): HIGH — based on direct reading of all 7 current pages vs. requirements
- Starlight authoring conventions: HIGH — based on direct reading of existing pages and astro.config.mjs
- Kubernetes/addon technical accuracy: MEDIUM — based on training knowledge cross-checked with existing page content; the most critical details (annotation names, API versions) flagged explicitly
- Local Registry cleanup: LOW — behavior of default registry:2 GC not confirmed from official docs

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (documentation phase, no fast-moving dependencies)
