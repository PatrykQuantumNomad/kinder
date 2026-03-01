# Project Research Summary

**Project:** kinder — batteries-included kind fork with default addons
**Domain:** Kubernetes-in-Docker cluster tooling — addon integration into creation pipeline
**Researched:** 2026-03-01
**Confidence:** HIGH

## Executive Summary

Kinder is a fork of the upstream `sigs.k8s.io/kind` tool that adds five default addons to every cluster: MetalLB (LoadBalancer IPs), Envoy Gateway (Gateway API), Metrics Server (kubectl top / HPA), CoreDNS tuning, and a web dashboard. The research establishes that all five addons have well-understood installation patterns, version-pinned manifests, and a clear integration point in kind's existing sequential action pipeline — new addon actions slot in after `waitforready`, following the same `Action` interface every existing kind action uses. The key architectural principle is embed-over-fetch: all manifests are embedded at build time using `//go:embed`, ensuring offline-capable cluster creation without external tool dependencies, consistent with kind's existing CNI and storage manifest patterns.

The highest-risk area in the stack is the web dashboard choice. The official `kubernetes/dashboard` project was archived on January 21, 2026 with no further security patches. FEATURES.md recommends switching to **Headlamp** (the official Kubernetes SIG UI successor, v0.40.1), while STACK.md and ARCHITECTURE.md were written assuming Dashboard v3 (Helm chart 7.14.0) or Dashboard v2.7.0. This conflict must be resolved before implementation: Headlamp is the safe choice for a new project, but requires different embedded manifests. The decision affects one action package (`installdashboard`) and does not alter the overall architecture.

The critical operational risk is MetalLB's macOS and Windows incompatibility: MetalLB assigns external IPs successfully but those IPs are unreachable from the host on non-Linux platforms because Docker runs inside a VM. This is a known, documented limitation that affects the majority of developer laptops. Kinder must print a clear warning on non-Linux hosts and document that LoadBalancer IP reachability is Linux-only. A secondary risk is the strict install ordering required: MetalLB must be fully ready with a functioning IPAddressPool before any Envoy Gateway resources are created, and Gateway API CRDs must be Established before the Envoy Gateway controller starts. Violating either ordering produces confusing silent failures.

## Key Findings

### Recommended Stack

All addon versions are pinned to their latest stable releases as of March 2026. The install method for each is kubectl apply from embedded manifests (no Helm, no Kustomize, no external tool dependencies at runtime). Envoy Gateway is the only addon requiring `--server-side` apply due to manifest size. Dashboard v3 requires Helm at install time, making it incompatible with kinder's no-external-dependency constraint — this is the strongest argument for adopting Headlamp instead.

**Core technologies:**
- MetalLB v0.15.3 — LoadBalancer IP assignment via Layer 2 ARP — only viable LB option for kind's Docker bridge network; BGP mode impossible without a BGP router
- Envoy Gateway v1.7.0 — Gateway API implementation — CNCF-incubating reference implementation; bundles Gateway API CRDs v1.4.1; requires `--server-side` apply
- Metrics Server v0.8.1 — kubectl top / HPA metrics — requires `--kubelet-insecure-tls` patch for kind's self-signed kubelet certs; embed components.yaml pre-patched
- CoreDNS tuning — ConfigMap patch only (no new install) — increase cache TTL to 300s; add `health { lameduck 5s }`; patch-in-place, never replace full ConfigMap
- **Dashboard: Headlamp v0.40.1 (RECOMMENDED) over kubernetes/dashboard** — Headlamp is the official SIG UI successor; provides static manifests; kubernetes/dashboard archived January 2026 with no security updates; Dashboard v3 has hard Helm+cert-manager dependency incompatible with kinder's design constraints

**Mandatory install order:**
1. MetalLB manifests + wait for controller Running
2. MetalLB IPAddressPool + L2Advertisement CRs (must wait for webhook)
3. CoreDNS ConfigMap patch + rollout restart
4. Metrics Server manifest + insecure-TLS patch
5. Gateway API CRDs (server-side apply)
6. Envoy Gateway controller + wait for Available
7. Dashboard (Headlamp) + RBAC + print token

### Expected Features

See full feature analysis in `.planning/research/FEATURES.md`.

**Must have (table stakes):**
- LoadBalancer services get an EXTERNAL-IP within seconds — without MetalLB every LoadBalancer service hangs pending; this is the most visible failure
- kubectl top nodes/pods works immediately after cluster creation — requires `--kubelet-insecure-tls` pre-applied
- HTTPRoute traffic actually routable through Envoy Gateway — depends on MetalLB assigning gateway an IP; end-to-end smoke test required
- Cluster DNS has no regression after CoreDNS tuning — patch must preserve the `kubernetes` plugin and `forward` directive exactly
- Dashboard is reachable with a printed token — zero-friction first login; port-forward command printed at cluster creation end

**Should have (competitive differentiators):**
- Deterministic IP range auto-carved from Docker/Podman/Nerdctl network subnet — users never configure IP pools manually
- MetalLB supports all three providers (Docker, Podman, Nerdctl) — subnet inspection command differs per provider; implement `switch` on `ctx.Provider.String()`
- Envoy Gateway GatewayClass created automatically — users can deploy HTTPRoutes without understanding GatewayClass controller names
- CoreDNS cache TTL increased to 300s — reduces repeated upstream lookups in long dev sessions
- All addons opt-out via cluster config `addons.disableX: true` — power users can skip any addon without forking kinder

**Defer to v1.1+:**
- cert-manager integration for TLS — document manual cert path instead
- Headlamp HTTPRoute via Envoy Gateway for dashboard URL — port-forward covers MVP
- NodeLocal DNSCache — invasive node-level DaemonSet change
- Prometheus + Grafana monitoring stack — separate milestone
- CoreDNS `autopath @kubernetes` with `pods verified` — note: FEATURES.md recommends this; STACK.md recommends simpler `cache 300` only; start with simpler approach
- VPA (Vertical Pod Autoscaler)

### Architecture Approach

The new addon actions integrate into kind's existing sequential action pipeline by appending after `waitforready` in `pkg/cluster/internal/create/create.go`. Each addon is a separate Go package under `pkg/cluster/internal/create/actions/` implementing the `actions.Action` interface, identical to existing actions like `installstorage`. The `ActionContext` already provides everything needed: `ctx.Nodes()` for the control-plane node, `ctx.Config` for addon opt-out flags, `ctx.Provider` for Docker/Podman/Nerdctl branching, and `ctx.Status` for progress display.

**Major components:**
1. `installmetallb` — Embeds metallb-native.yaml; detects Docker/Podman/Nerdctl network subnet at runtime; generates IPAddressPool + L2Advertisement in-memory; waits for controller rollout before applying CRs
2. `installgatewayapi` — Embeds standard-install.yaml from Gateway API release; server-side apply; waits for CRD Established condition before returning
3. `installenvoygw` — Embeds Envoy Gateway install.yaml; server-side apply; waits for envoy-gateway deployment Available
4. `installmetricsserver` — Embeds pre-patched components.yaml (with `--kubelet-insecure-tls` baked in); waits for metrics-server deployment Available
5. `installcorednstuning` — No embedded manifest; patches existing CoreDNS ConfigMap in-place; rolling restart; waits for coredns rollout
6. `installdashboard` — Embeds Headlamp static manifests; creates kinder-dashboard ServiceAccount + cluster-admin ClusterRoleBinding; prints token and port-forward command to stdout

**Config schema extension:** Add `Addons` struct to both internal `pkg/internal/apis/config/types.go` and public `pkg/apis/config/v1alpha4/types.go`. All fields are bool with `omitempty`, defaulting to `false` (all addons enabled). Regenerate `zz_generated.deepcopy.go` for both. The backward-compatible design means existing kind cluster configs that omit `addons:` continue to work unchanged.

**Four implementation patterns (from ARCHITECTURE.md):**
- Pattern 1: Embedded manifest — `//go:embed manifests/*.yaml` piped to `kubectl apply -f -` (all addons except CoreDNS)
- Pattern 2: Dynamic manifest — generate YAML in Go after runtime subnet detection (MetalLB IPAddressPool only)
- Pattern 3: Rollout wait — `kubectl rollout status --timeout=120s` after every workload apply
- Pattern 4: Opt-out guard — `if !opts.Config.Addons.DisableX { actionsToRun = append(...) }` wrapping each action in `create.go`

### Critical Pitfalls

Full details in `.planning/research/PITFALLS.md`. Top issues that will cause build rework or user-visible failures if ignored:

1. **MetalLB IPs unreachable on macOS/Windows (C1)** — Docker's bridge network is inside a Linux VM on non-Linux hosts; MetalLB assigns IPs but they are unroutable from the host. Prevention: detect OS at startup, print warning on macOS/Windows, document Linux-only reachability prominently in README.

2. **MetalLB IPAddressPool uses wrong subnet (C3)** — Docker `kind` network subnet is dynamic and hash-derived; hardcoding `172.18.x.x` fails on many machines. Prevention: always detect subnet at runtime via `docker/podman/nerdctl network inspect kind`; allocate the upper /28 of the detected subnet (e.g., `.200-.250`).

3. **Envoy Gateway crash-loops when Gateway API CRDs are absent or not yet Established (C4)** — CRD must be Established (not just created) before the controller starts. Prevention: separate `installgatewayapi` action with `kubectl wait --for=condition=Established` before `installenvoygw` runs.

4. **Metrics Server CrashLoop due to self-signed kubelet TLS (C6)** — Default metrics-server fails TLS verification against kind's self-signed kubelet certs. Prevention: embed components.yaml with `--kubelet-insecure-tls` and `--kubelet-preferred-address-types=InternalIP` pre-applied; verify with `kubectl top nodes` post-install.

5. **CoreDNS forwarding loop on systemd-resolved hosts (C7)** — Ubuntu/Debian hosts expose `127.0.0.53` in `/etc/resolv.conf`; CoreDNS `forward . /etc/resolv.conf` loops back to itself. Prevention: use explicit upstream IPs (`8.8.8.8 1.1.1.1`) or `/run/systemd/resolve/resolv.conf` in the forward directive, or at minimum document and detect this condition.

6. **Dashboard choice: kubernetes/dashboard is archived (C8 + FEATURES.md finding)** — Dashboard v3 requires cert-manager and Helm at runtime; Dashboard v2.7.0 is end-of-life. Prevention: use Headlamp instead; static manifests available, no Helm dependency, actively maintained.

7. **CRD-before-CR timing race (M7)** — Applying CRDs and CRs in the same kubectl call races CRD propagation. Prevention: the kinder action pipeline serializes steps naturally; add explicit `kubectl wait --for=condition=Established` between CRD apply and any CR apply (particularly MetalLB: wait for webhook ready before IPAddressPool/L2Advertisement).

## Implications for Roadmap

Based on combined research, the natural implementation structure follows the addon dependency graph and the config schema change that must precede all action work.

### Phase 1: Config Schema and Action Scaffolding

**Rationale:** Every subsequent phase depends on the `Addons` struct existing in both the internal and public config types, and on the action pipeline entry points existing in `create.go`. This is pure scaffolding with no risk — it does not install anything. All eight modified files (types.go x2, convert.go, default.go, validate.go, deepcopy.go x2, create.go) must be updated before any action code can compile. This phase is the unblocking dependency for all other phases.

**Delivers:** Compilable project with `addons:` YAML key support; opt-out flags functional (no-op since no addon actions yet); deep-copy regenerated; backward-compatible with existing cluster configs.

**Addresses:** Config opt-out feature from FEATURES.md; ARCHITECTURE.md config schema spec.

**Avoids:** The anti-pattern of embedding addon logic in create.go directly rather than using the action pattern (ARCHITECTURE.md Anti-Pattern discussion).

**Research flag:** Standard patterns — no deeper research needed. Kind's existing code is the template.

### Phase 2: MetalLB Addon Action

**Rationale:** MetalLB is the load-balancer foundation that Envoy Gateway depends on. It must be implemented and verified before Envoy Gateway can be tested end-to-end. It also has the most complex runtime logic (subnet detection, IP range calculation, multi-provider branching) and the most pitfalls, making it the highest-risk single action — address it early when it is easiest to iterate.

**Delivers:** `installmetallb` package; embedded metallb-native.yaml; runtime Docker/Podman/Nerdctl subnet detection; IPAddressPool + L2Advertisement applied post-webhook-ready; `kubectl get svc` shows EXTERNAL-IP after cluster creation on Linux.

**Uses:** MetalLB v0.15.3 manifest; Docker/Podman/Nerdctl network inspect; IPAddressPool v1beta1 CRDs.

**Implements:** Pattern 1 (embedded manifest) + Pattern 2 (dynamic manifest for pool config) + Pattern 3 (rollout wait).

**Avoids:** C2 (single-node exclude label — verify kubeadminit removes it first), C3 (wrong subnet — runtime detection required), C1 (macOS warning printed to stderr).

**Research flag:** Standard patterns — MetalLB + kind is extremely well-documented. No additional research phase needed.

### Phase 3: Metrics Server Addon Action

**Rationale:** Metrics Server has zero dependencies on other addons, is the simplest action to implement (single manifest + one patch), and its verification (`kubectl top nodes`) is the fastest smoke test. Implementing it immediately after MetalLB gives a working end-to-end addon to validate the action scaffolding against before tackling the more complex Envoy Gateway.

**Delivers:** `installmetricsserver` package; pre-patched components.yaml embedded; `kubectl top nodes` and `kubectl top pods` functional within 60 seconds of cluster creation.

**Uses:** Metrics Server v0.8.1 with `--kubelet-insecure-tls` and `--kubelet-preferred-address-types=InternalIP` pre-applied.

**Implements:** Pattern 1 (embedded pre-patched manifest) + Pattern 3 (rollout wait).

**Avoids:** C6 (TLS crash-loop — insecure-tls must be baked in, not applied as a separate patch step); M5 (apiserver aggregation — do not modify kubeadm config).

**Research flag:** Standard patterns — the insecure-tls fix is universally documented. No additional research needed.

### Phase 4: CoreDNS Tuning Action

**Rationale:** CoreDNS tuning has no dependencies on other addons and affects all pods from the moment it is applied. Implementing it after Metrics Server but before Envoy Gateway means all subsequent addon pods benefit from improved cache settings. It is also the action with the most nuanced implementation (patch-in-place, not apply) and the highest risk of inadvertently breaking cluster DNS if done wrong — better to isolate it as its own phase with focused testing.

**Delivers:** `installcorednstuning` package; CoreDNS cache TTL increased to 300s; `lameduck 5s` added; ConfigMap patched in-place preserving all existing plugins; rollout restart confirmed.

**Uses:** ConfigMap patch (no embedded manifest); `kubectl patch` + `kubectl rollout restart`.

**Implements:** Pattern 3 (rollout wait); explicit read-modify-write to avoid full ConfigMap overwrite.

**Avoids:** C7 (systemd-resolved forwarding loop — audit forward directive before patching); M4 (full ConfigMap overwrite — use merge patch, always include complete Corefile preserving kubernetes plugin).

**Research flag:** Needs validation during implementation — the exact merge-patch strategy for the Corefile (which is a string blob, not structured YAML) requires testing against actual kind clusters with and without systemd-resolved. Consider making the upstream forwarder configurable.

### Phase 5: Envoy Gateway Addon Actions (Gateway API CRDs + Controller)

**Rationale:** Envoy Gateway comes after MetalLB (verified in Phase 2) because the Gateway proxy service requires a LoadBalancer IP from MetalLB. It is split into two actions (`installgatewayapi` then `installenvoygw`) matching the dependency: CRDs must be Established before the controller starts. This is the most architecturally complex addon because it involves large manifests (~3,000 lines), server-side apply, and a strict two-step ordering with a readiness wait in between.

**Delivers:** `installgatewayapi` + `installenvoygw` packages; Gateway API CRDs (experimental channel) installed; Envoy Gateway controller running; GatewayClass `eg` available; Gateway proxy service gets an EXTERNAL-IP from MetalLB; HTTPRoute traffic routable end-to-end.

**Uses:** Envoy Gateway v1.7.0 install.yaml (bundles Gateway API CRDs + controller); `kubectl apply --server-side`; `kubectl wait --for=condition=Established` on CRDs before controller deployment.

**Implements:** Pattern 1 (embedded manifest, server-side) + Pattern 3 (rollout wait) + Pattern 4 (opt-out guard with nested DisableGatewayAPI gating DisableEnvoyGateway).

**Avoids:** C4 (CRD-before-controller ordering — explicit Established wait between actions); C5 (Gateway stays Unknown — MetalLB must be ready first); N2 (direct_response 500s — verify backend services exist in smoke test).

**Research flag:** May benefit from a research-phase pass during planning. The experimental vs. standard Gateway API CRD channel distinction (PITFALLS.md C4) and the large embedded manifest size (binary size implications) are areas that need implementation-time decisions. Specifically: should kinder embed the full experimental-channel CRDs, or only standard-channel? Verify binary size impact of embedding ~3,000-line install.yaml.

### Phase 6: Dashboard Addon Action (Headlamp)

**Rationale:** Dashboard is cosmetic and has no hard dependencies on other addons (port-forward access works without Envoy Gateway). Placing it last allows the implementation to optionally create an HTTPRoute pointing to Headlamp if Envoy Gateway is also enabled, without making that a blocking dependency. It is also the addon with the sharpest research conflict (Headlamp vs kubernetes/dashboard) that requires a decision before implementation begins.

**Delivers:** `installdashboard` package; Headlamp deployed in `kube-system` (or `headlamp` namespace); `kinder-dashboard` ServiceAccount with cluster-admin ClusterRoleBinding; long-lived token printed to stdout; port-forward command printed; optional HTTPRoute created if Envoy Gateway is enabled.

**Uses:** Headlamp v0.40.1 static manifests (embed directly — no Helm required); inline RBAC YAML; `kubectl create token` or static Secret for persistent token.

**Implements:** Pattern 1 (embedded manifest) + Pattern 3 (rollout wait) + token/access instructions printed via `ctx.Logger`.

**Avoids:** C8 (kubernetes/dashboard v3 Kong+cert-manager requirement — use Headlamp instead); M6 (RBAC insufficient — create explicit cluster-admin SA, not default SA); N5 (1-hour token expiry — use `--duration=8760h` or static Secret).

**Research flag:** Needs focused research on Headlamp static manifests before implementation. The STACK.md and ARCHITECTURE.md were written assuming kubernetes/dashboard — Headlamp deployment manifests, service names, and access patterns are different and need to be verified against the Headlamp v0.40.1 release.

### Phase 7: Integration Testing and Smoke Tests

**Rationale:** PITFALLS.md documents a "Looks Done But Isn't" checklist for every addon — a pod Running does not mean the addon works. Each phase should include functional smoke tests, but the final phase consolidates end-to-end validation: a full `kinder create cluster` run that verifies all addons are functional before the cluster is reported ready.

**Delivers:** Functional verification for each addon embedded in the action's `Execute()` method (not just rollout wait); cross-addon integration tests (e.g., MetalLB → Envoy Gateway → HTTPRoute end-to-end); CI test additions; documentation.

**Implements:** Per-addon `kubectl wait --for=condition=Available`; post-install `kubectl top nodes` check; post-CoreDNS DNS resolution check from test pod; Gateway end-to-end curl test.

**Avoids:** TD2 (no health verification after installation — the most common source of "works on my machine" bugs across all addons).

**Research flag:** Standard patterns — verification commands are documented per-addon in PITFALLS.md. No additional research needed.

### Phase Ordering Rationale

- Config schema change is Phase 1 because it is a compile-time prerequisite for all action code; nothing can be written without it.
- MetalLB is Phase 2 because Envoy Gateway has a hard runtime dependency on MetalLB (LoadBalancer IP assignment), and testing the most risk-laden addon early allows iteration while the codebase is small.
- Metrics Server is Phase 3 because it is independent, simple, and provides an immediately verifiable outcome (`kubectl top nodes`) that validates the action scaffolding works correctly.
- CoreDNS is Phase 4 because it affects all subsequent pods and has a subtle implementation risk (ConfigMap merge vs. replace) that is easier to isolate in its own phase.
- Envoy Gateway is Phase 5, not Phase 3 or 4, because it requires MetalLB to be stable and tested first (Phase 2), and because its large manifest and two-step CRD ordering make it the most implementation-intensive action.
- Dashboard is last because it is cosmetic, opt-outable without losing core functionality, and requires a research decision (Headlamp vs. archived kubernetes/dashboard) that should not block other phases.
- Integration testing is its own phase because the per-addon smoke tests interact with the full cluster state and are best validated after all addons can coexist.

### Research Flags

Phases needing deeper research during planning:
- **Phase 4 (CoreDNS):** The merge-patch strategy for the Corefile blob (string value in a ConfigMap, not structured YAML) and the systemd-resolved detection/mitigation strategy need implementation-time research. Also reconcile the STACK.md recommendation (simple `cache 300`) against the FEATURES.md recommendation (`autopath @kubernetes` + `pods verified`) — these are different complexity levels.
- **Phase 5 (Envoy Gateway):** Confirm standard vs. experimental Gateway API CRD channel decision; measure embedded manifest binary size impact; verify `kubectl wait --for=condition=Established` exact syntax for multiple CRDs.
- **Phase 6 (Dashboard):** Headlamp v0.40.1 static manifest discovery — confirm deployment YAML is available without Helm; verify service names and port numbers differ from kubernetes/dashboard.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Config schema):** Kind's own types.go and zz_generated.deepcopy.go are the template; pattern is clear.
- **Phase 2 (MetalLB):** Extensively documented; subnet detection pattern validated by multiple community sources.
- **Phase 3 (Metrics Server):** The insecure-TLS fix is universally documented; no ambiguity.
- **Phase 7 (Integration testing):** Verification commands are fully specified in PITFALLS.md.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Versions verified against GitHub releases and official docs as of March 2026; one conflict (Dashboard choice) flagged and resolved in favor of Headlamp |
| Features | HIGH | Core features well-documented; FEATURES.md raises valid Headlamp recommendation backed by official archive notice |
| Architecture | HIGH | Kind codebase read directly at commit 89ff06bd; action pattern is clear and consistent; one inconsistency (Dashboard version) noted |
| Pitfalls | HIGH | 8 critical, 7 moderate, 5 minor pitfalls documented with sources; most are confirmed by official GitHub issues or docs |

**Overall confidence:** HIGH

### Gaps to Address

- **Dashboard implementation choice:** The three research files disagree — FEATURES.md recommends Headlamp, STACK.md recommends kubernetes/dashboard Helm chart 7.14.0, ARCHITECTURE.md references kubernetes/dashboard v2.7.0. This must be resolved before Phase 6 begins. Recommendation: adopt Headlamp (FEATURES.md position is most current and technically sound given the January 2026 archive). Requires a focused research pass on Headlamp v0.40.1 static manifests.

- **CoreDNS Corefile merge strategy:** ConfigMap values are string blobs — standard `kubectl patch --type=merge` merges ConfigMap `.data` keys but does not merge the string value within a key. The implementation must read the existing Corefile string, modify it programmatically in Go, and write it back. This is not documented in ARCHITECTURE.md and needs an implementation decision.

- **Binary size impact of embedded manifests:** Envoy Gateway install.yaml is ~3,000 lines; metallb-native.yaml is significant; the pre-rendered Dashboard manifests add more. Total binary size increase should be measured and, if excessive, the Envoy Gateway manifest should be compressed before embedding using `compress/gzip` + `//go:embed`.

- **macOS/Windows mitigation strategy:** Research confirms MetalLB IPs are unreachable on macOS/Windows but does not specify the exact warning mechanism in kinder. This should be a stderr warning printed by `installmetallb` when `runtime.GOOS != "linux"`, before attempting installation. Whether to skip MetalLB entirely on macOS/Windows (different user experience) is a product decision not resolved by the research.

- **Podman rootless MetalLB viability:** PITFALLS.md M2 flags that MetalLB L2 speaker may not work in rootless Podman due to host-network and raw socket restrictions. No definitive resolution is documented. This needs testing against the Podman provider during Phase 2.

## Sources

### Primary (HIGH confidence)
- Kind codebase at `/Users/patrykattc/work/git/kinder` (commit 89ff06bd) — action pipeline, provider interface, config types
- MetalLB official docs (metallb.universe.tf) — v0.15.3 installation, IPAddressPool, L2Advertisement, L2 mode concepts, troubleshooting
- Envoy Gateway official docs (gateway.envoyproxy.io) — v1.7.0 install YAML, compatibility matrix, quickstart
- Metrics Server GitHub (kubernetes-sigs/metrics-server) — v0.8.1 release, kind-specific TLS requirement
- Kubernetes official docs (kubernetes.io) — CoreDNS customization, resource metrics pipeline, DNS custom nameservers
- CoreDNS official docs (coredns.io) — loop plugin, autopath plugin
- Headlamp GitHub (kubernetes-sigs/headlamp) — v0.40.1 releases, official SIG UI successor status
- kubernetes-retired/dashboard GitHub — official archive confirmation, January 2026

### Secondary (MEDIUM confidence)
- michaelheap.com — MetalLB + kind subnet detection pattern (verified technique)
- MetalLB kind issue #3167 — Docker gateway IP conflict
- kind issue #3556 — macOS bridge network limitation confirmed
- CoreDNS loop issue #2354 — systemd-resolved root cause
- metrics-server kind gist (sanketsudake) — insecure-TLS patch pattern

### Tertiary (LOW confidence)
- Community blogs on Kubernetes Dashboard alternatives (2026) — useful ecosystem context; Headlamp recommendation independently verified against official sources
- Gateway API in 2026 dev.to post — ecosystem overview; Envoy Gateway adoption claim supported by CNCF incubation status

---
*Research completed: 2026-03-01*
*Ready for roadmap: yes*
