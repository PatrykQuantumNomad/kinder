# Research Summary: v2.3 Inner Loop

**Project:** kinder — v2.3 Inner Loop (cluster lifecycle, snapshot/restore, hot reload, runtime diagnostics, upstream sync)
**Researched:** 2026-05-03
**Discovery method:** Two parallel research streams — ecosystem feature-gap analysis (competitor matrix, kind issue tracker, HN/Reddit pain points) and upstream/addon update audit (kind main branch, K8s 1.35-1.36 release notes, addon changelogs).
**Overall confidence:** HIGH for ecosystem signals (sourced from issue reactions, recent release notes, comparable tools), HIGH for upstream sync data (verified against kind PRs, K8s release blogs, addon GitHub releases).

## TL;DR

Kinder v2.2 made cluster *creation* powerful (multi-version, air-gap, host-mount, image-load). v2.3 makes daily *iteration* fast: pause/resume to reclaim laptop resources, snapshot/restore to reset cluster state in seconds, `kinder dev` for inner-loop hot reload, and runtime error decoding that extends v2.1's doctor framework. A deliberate sync phase keeps kinder current with kind upstream's HAProxy→Envoy LB transition and K8s 1.36's default-image bump (avoiding silent IPVS-removal breakage).

Every feature builds on existing kinder infrastructure (v2.0 GoReleaser/GPU, v2.1 doctor framework, v2.2 image-load + air-gap). **Zero new module dependencies expected** — fsnotify watching for `kinder dev` is the only new candidate dep, and stdlib `inotify`/`kqueue` wrappers exist if we want to keep the zero-dep streak.

---

## Top Feature Candidates (selected for v2.3)

### 1. `kinder pause` / `kinder resume` (LIFE category)

**Pitch:** Stop all node containers without losing state; resume in seconds. Reclaims RAM/CPU on laptops without killing the cluster.

**Evidence:** kind issue [#2715](https://github.com/kubernetes-sigs/kind/issues/2715) — 75 reactions, 70 thumbs-up, third-most-reacted enhancement. Currently kind users do `docker stop` manually with no guarantee HA reboot doesn't break.

**Complexity:** LOW. `docker stop`/`start` orchestrated in correct order (control-plane first/last), kubelet/etcd validation post-resume.

**Dependencies:** None new. Reuses `pkg/cluster/internal/providers/{docker,podman,nerdctl}/provider.go` for stop/start primitives. Optional: doctor pre-flight check for HA reboot risk (kind issue [#1689](https://github.com/kubernetes-sigs/kind/issues/1689)).

**Risk:** HA cluster restart order; etcd quorum must form. Mitigation: explicit pre-flight check in v2.1 doctor framework.

**Synergy:** Doctor checks (v2.1) gate the resume; pairs with snapshot (#2) for "freeze a known-good state."

### 2. `kinder snapshot` / `kinder restore` (LIFE category)

**Pitch:** Snapshot a cluster's full state (etcd + loaded images + PV contents) to a named local archive; restore it in seconds when integration tests pollute state.

**Evidence:** kind issue [#3508](https://github.com/kubernetes-sigs/kind/issues/3508) — open since Feb 2024 with no native solution. Reset-time is the dominant inner-loop cost for integration testers; v2.2's `kinder load images` cuts cold-start; snapshots cut warm-start to near zero.

**Complexity:** MEDIUM. etcd `etcdctl snapshot save/restore` is well-documented (kubeadm uses it). Need to capture loaded images + local-path-provisioner PV contents.

**Dependencies:** Uses existing local-path-provisioner addon (v2.2), existing etcd in node containers. Zero new Go module deps.

**Risk:** Snapshot becomes stale across kube versions — must enforce version-matching on restore. Disk usage — gate behind `kinder snapshot prune` and quota warnings.

**Synergy:** Compounds v2.2's image-loading and air-gap story — a snapshot can include the offline image bundle, making "offline reproducible cluster fixtures" a kinder superpower.

### 3. `kinder dev` — inner-loop hot reload (DEV category)

**Pitch:** Watch a directory, build an image, import via existing `kinder load images`, and roll the target Deployment — Tilt/Skaffold ergonomics with zero YAML/DSL.

**Evidence:** Inner-loop tooling is the #1 friction area for K8s dev ([Northflank Tilt-alternatives](https://northflank.com/blog/tilt-alternatives), [DevSpace's "95% iteration-time reduction"](https://www.vcluster.com/blog/skaffold-vs-tilt-vs-devspace)). None of kind/k3d/minikube ship this. DevSpace/Skaffold/Tilt all require YAML/DSL setup; the kinder pitch is "single command, no config."

**Complexity:** MEDIUM-HIGH. fsnotify watcher, debounced builds, image digest tracking, `kubectl rollout restart` shim.

**Dependencies:** Reuses `kinder load images` (v2.2). The fsnotify dep (`github.com/fsnotify/fsnotify`) is a candidate new dep — small, ubiquitous, MIT-licensed. Alternative: poll-based watcher in stdlib (lower fidelity).

**Risk:** Easy to overscope into a Tilt clone. Keep deliberately minimal: single-deployment, single watch-dir, no Tiltfile DSL, no helm-chart awareness.

**Synergy:** Makes v2.2's image-loading work pay rent on every code save, not just on cluster create. Differentiates kinder from kind hard.

### 4. `kinder doctor decode` — runtime error decoder (DIAG category)

**Pitch:** A built-in dictionary that maps cryptic Docker/kubeadm/kubelet/containerd errors to plain-English fixes, with `--auto-fix` for known-safe remediations. Wraps `docker logs` and `kubectl get events` into one curated stream.

**Evidence:** kind issue [#3795](https://github.com/kubernetes-sigs/kind/issues/3795) (containerd image loading) has 64 comments of users debugging the same handful of errors. v2.1 already has 23 doctor checks — extending to runtime/post-create errors is the obvious next step. Reddit 2026 pain point: "diagnosing Kubernetes is like drinking from an angry firehose."

**Complexity:** MEDIUM. Error-pattern catalog (regex → remediation), event watcher, formatter.

**Dependencies:** None new. Direct extension of v2.1 doctor framework.

**Risk:** Catalog needs upkeep. Mitigation: source patterns from v2.1 checks, integration-test failures, and existing kind issue triage. Start narrow (top 10-15 patterns), expand based on user reports.

**Synergy:** Direct extension of v2.1 doctor framework; turns one-shot pre-flight into continuous diagnostics.

### 5. Upstream Sync — Envoy LB + K8s 1.36 default + IPVS warn (SYNC category)

**Pitch:** Adopt kind's HAProxy→Envoy load-balancer transition, bump default node image to K8s 1.36 ("Haru"), reject `kubeProxyMode: ipvs` on 1.36+ (silent breakage otherwise), and ship a recipe for User Namespaces (GA in 1.36) + In-Place Pod Resize (GA in 1.36).

**Evidence:**
- [kind PR #4127](https://github.com/kubernetes-sigs/kind/pull/4127) merged 2026-04-03 — replaces HAProxy with Envoy in providers + loadbalancer; +400/-276 lines well-isolated. Future kinder→kind syncs will conflict if not adopted.
- [K8s 1.36 release blog](https://kubernetes.io/blog/2026/04/22/kubernetes-v1-36-release/) (2026-04-22): User Namespaces GA, In-Place Pod Resize GA, IPVS removed from kube-proxy, `gitRepo` volume removed.
- [kind issue #4131](https://github.com/kubernetes-sigs/kind/issues/4131): K8s 1.35.0/1.35.1 ship with `MaxUnavailableStatefulSet` regression — fix in 1.35.4.

**Complexity:** LOW-MEDIUM. Envoy LB sync is a focused port of #4127. Image bump is hours; recipes a couple of days.

**Dependencies:** None new. Drops `kindest/haproxy` image dependency.

**Risk:** Envoy LB config differs subtly from HAProxy — must regression-test HA cluster create flow. K8s 1.36 may surface kubeadm v1beta3 deprecation warnings (not yet blocking; v1beta4 generator deferred to v2.4).

**Synergy:** Avoids accumulating drift; positions kinder for next year's K8s releases without forced migrations.

---

## Out of scope for v2.3 (explicitly deferred)

| Feature | Why deferred |
|---|---|
| `kinder fleet` multi-cluster | High value but vind/k3d coverage exists; doesn't fit "inner loop" theme |
| `kinder observe` LGTM stack preset | Fits "Observability" theme (Option C); resource-heavy on dev laptops |
| `kinder gpu ollama` | Fits "AI" theme (Option C); narrower audience than inner loop |
| `kinder gitops` Argo/Flux preset | Niche; defer until requested by users |
| `kinder runner` GitHub Actions | Niche; defer until requested by users |
| Provider de-duplication refactor | Pure tech debt; v2.4 milestone candidate |
| context.Context cancellation through Create | Pure tech debt; v2.4 milestone candidate |
| kubeadm v1beta4 generator | Not yet blocking; v2.4 milestone candidate |
| cert-manager v1.16.3 → v1.20.x | UID change requires audit; defer to v2.4 |
| Envoy Gateway v1.3.1 → v1.7 | 4-major bump needs staged testing; defer to v2.4 |
| Cluster pause for podman/nerdctl providers | Start with docker; expand based on demand |

## Anti-features (won't pursue)

| Anti-feature | Reason |
|---|---|
| Cilium-by-default CNI swap | Kernel/Docker Desktop pain; blast radius too large; opt-in only |
| Tiltfile-style DSL for `kinder dev` | Compete-with-Tilt = lose; stay imperative-CLI |
| Velero bundle for backup/restore | Overkill for laptops; lightweight `kinder snapshot` covers actual dev pain |
| Web UI for cluster management | Headlamp covers it; UX treadmill |
| Apple Containerization runtime support | Unstable surface; revisit in 6-12 months |

---

## Stack additions

**No new module dependencies expected** for pause/resume, snapshot/restore, doctor decode, or upstream sync. Only candidate: `github.com/fsnotify/fsnotify` for `kinder dev` file watching — small, MIT, ubiquitous. Alternative: poll-based watcher in stdlib (acceptable trade-off if zero-dep is preserved).

**Image dependencies dropped:** `kindest/haproxy:v20260131-7181c60a` (replaced by Envoy LB image already used elsewhere in kinder).

**Image dependencies bumped:** `kindest/node:v1.35.x` → `kindest/node:v1.36.x` (default; older versions still selectable via `image:` config).

## Watch out for

1. **HA reboot fragility (kind #1689):** Pause/resume on HA clusters can leave etcd in a bad state if quorum doesn't reform. Mitigation: doctor pre-flight check before resume.
2. **Snapshot version drift:** Restoring a snapshot with mismatched Kubernetes versions silently corrupts state. Mitigation: store version in snapshot metadata; refuse restore on mismatch.
3. **fsnotify on macOS Docker Desktop:** Volume-mount fsnotify events on Docker Desktop for Mac are flaky. Mitigation: poll-fallback flag + clear docs.
4. **Envoy LB regression on HA:** Subtle differences from HAProxy may surface as healthcheck timing changes. Mitigation: HA integration test in CI before ship.
5. **K8s 1.36 IPVS removal:** Anyone with `kubeProxyMode: ipvs` in their config gets silent broken nodes on 1.36. Mitigation: validation rejects ipvs on 1.36+; clear error message points to iptables migration.
6. **`gitRepo` volume removal:** Audit all kinder docs/examples; remove any gitRepo references.
7. **Doctor decode catalog upkeep:** Pattern catalog rots without active triage. Mitigation: source patterns from v2.1 checks, CI failures, kind issue tracker.

---

## Source URLs

- [kind issue #2715 — start/stop](https://github.com/kubernetes-sigs/kind/issues/2715)
- [kind issue #3508 — snapshot/restore](https://github.com/kubernetes-sigs/kind/issues/3508)
- [kind issue #3795 — containerd image loading errors](https://github.com/kubernetes-sigs/kind/issues/3795)
- [kind issue #1689 — HA reboot](https://github.com/kubernetes-sigs/kind/issues/1689)
- [kind PR #4127 — Envoy LB](https://github.com/kubernetes-sigs/kind/pull/4127)
- [Kubernetes 1.36 release blog](https://kubernetes.io/blog/2026/04/22/kubernetes-v1-36-release/)
- [palark 1.36 deep-dive](https://palark.com/blog/kubernetes-1-36-release-features/)
- [Northflank Tilt alternatives 2026](https://northflank.com/blog/tilt-alternatives)
- [vCluster Skaffold vs Tilt vs DevSpace](https://www.vcluster.com/blog/skaffold-vs-tilt-vs-devspace)
- [DevSpace](https://www.devspace.sh/)

---

*Research synthesized: 2026-05-03 from two discovery streams (ecosystem gap analysis + upstream/addon audit).*
