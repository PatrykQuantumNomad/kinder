---
phase: 51-upstream-sync-k8s-1-36
plan: 03
type: execute
wave: 1
depends_on: []
files_modified:
  - kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md
  - kinder-site/astro.config.mjs
autonomous: true

must_haves:
  truths:
    - "kinder website has a 'What's new in K8s 1.36' guide page reachable via the Guides sidebar"
    - "The page demonstrates User Namespaces (GA in 1.36) with a working hostUsers: false pod spec and a kubectl-based verification step"
    - "The page demonstrates In-Place Pod Resize (container-level, GA in 1.35, default-on in 1.36) with a working pod spec, a resize patch command, and a verification command"
    - "The page renders correctly in the Starlight Astro build (no broken frontmatter, no missing sidebar entry)"
  artifacts:
    - path: "kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md"
      provides: "Guide page with frontmatter + User Namespaces section + In-Place Pod Resize section"
      contains: "hostUsers: false"
      min_lines: 80
    - path: "kinder-site/astro.config.mjs"
      provides: "Sidebar registration of the new guide"
      contains: "guides/k8s-1-36-whats-new"
  key_links:
    - from: "kinder-site/astro.config.mjs Guides items array"
      to: "src/content/docs/guides/k8s-1-36-whats-new.md"
      via: "{ slug: 'guides/k8s-1-36-whats-new' } entry"
      pattern: "k8s-1-36-whats-new"
---

<objective>
Add a "What's new in Kubernetes 1.36" guide to the kinder website, demonstrating the two GA features called out by SC4: User Namespaces and In-Place Pod Resize, both runnable on a kinder cluster.

Purpose: Help users discover and exercise the headline 1.36 features without leaving the kinder docs site. Delivers SYNC-04 and SC4.

Output: A new MDX guide at `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` registered in the Starlight sidebar via `kinder-site/astro.config.mjs`.

Notes from RESEARCH §SYNC-04:
- User Namespaces: GA in 1.36, no feature gate needed (`hostUsers: false`), Linux kernel ≥5.12 required (default kindest/base satisfies this).
- In-Place Pod Resize: container-level (`InPlacePodVerticalScaling`) graduated to **GA in v1.35** (default-on in 1.36) — this is the simpler feature to demonstrate. Pod-level (`InPlacePodLevelResourcesVerticalScaling`) is Beta in 1.36 and out of scope for this page.
- kubeadm v1beta4 note: kind issue #3847 plans v1beta4 adoption for K8s 1.36+ clusters; mention briefly that users with `kubeadmConfigPatches` using v1beta3 `extraArgs` (map syntax) may need to update.
</objective>

<execution_context>
@/Users/patrykattc/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/phases/51-upstream-sync-k8s-1-36/51-RESEARCH.md

# Format reference — read these to mirror frontmatter, callouts, and code-fence style
@kinder-site/src/content/docs/guides/multi-version-clusters.md
@kinder-site/src/content/docs/guides/local-dev-workflow.md

# Sidebar registration target
@kinder-site/astro.config.mjs
</context>

<tasks>

<task type="auto">
  <name>Task 1: Author the K8s 1.36 guide page</name>
  <files>
    kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md
  </files>
  <action>
First, read at least one existing guide (e.g. `multi-version-clusters.md` and/or `local-dev-workflow.md`) to mirror the frontmatter shape, code-fence style, and Starlight callout (`:::tip`, `:::note`, `:::caution`) usage patterns already in use on this site. Match those conventions exactly — don't invent a new style.

Create `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` with the following structure:

**Frontmatter** (Starlight schema):
```yaml
---
title: What's new in Kubernetes 1.36
description: Demonstrate User Namespaces (GA) and In-Place Pod Resize (GA) on a kinder cluster running Kubernetes 1.36.
---
```

**Body sections** (in order):

1. **Intro** (1-2 short paragraphs) — what 1.36 brings, mention this guide focuses on two GA features that need no feature gates.

2. **Prerequisites** — bulleted list:
   - kinder ≥ v0.X (the version that ships SYNC-02 — write "v0.X" as a placeholder; the SYNC-02 plan or release process can fix the exact version).
   - A running kinder cluster on Kubernetes ≥1.36 (e.g. `kinder create cluster` with the new default).
   - `kubectl` configured for the cluster.
   - Linux host with kernel ≥5.12 (for User Namespaces; macOS via Docker Desktop's VM is fine — the VM kernel is recent).

3. **Section: User Namespaces (GA)**
   - One paragraph explaining what user namespaces do (map container UID 0 to a non-root host UID, contain breakouts).
   - Note: in 1.36 this is GA — no feature gate required. The feature gate `UserNamespacesSupport` is now permanently on.
   - YAML pod spec (use the exact one from RESEARCH §SYNC-04, `userns-demo`):
     ```yaml
     apiVersion: v1
     kind: Pod
     metadata:
       name: userns-demo
     spec:
       hostUsers: false
       containers:
       - name: app
         image: fedora:42
         securityContext:
           runAsUser: 0
         command: ["sh", "-c", "whoami && cat /proc/self/uid_map"]
     ```
   - Apply command: `kubectl apply -f userns-demo.yaml` (show as a fenced shell block).
   - Verification: `kubectl logs userns-demo` — explain that `whoami` prints `root` (in-container UID 0) but `/proc/self/uid_map` shows the host UID range starts at a high non-root number (e.g. `0 1000000 65536`), proving the namespacing.
   - `:::tip` callout: mention that on Docker Desktop the host-side UID range is allocated by the Docker VM's `/etc/subuid` config — usually fine out of the box, but if you see "permission denied" errors check the kindest/base requirements at https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/.

4. **Section: In-Place Pod Resize (GA, container-level)**
   - One paragraph: this lets you change CPU/memory `requests`/`limits` on a running container without restarting the pod, when `resizePolicy.restartPolicy` is `NotRequired`. Container-level resize was GA in 1.35 and is on by default in 1.36.
   - YAML pod spec (use exact one from RESEARCH §SYNC-04, `resize-demo`):
     ```yaml
     apiVersion: v1
     kind: Pod
     metadata:
       name: resize-demo
     spec:
       containers:
       - name: app
         image: registry.k8s.io/pause:3.10
         resources:
           requests:
             cpu: "100m"
             memory: "64Mi"
           limits:
             cpu: "200m"
             memory: "128Mi"
         resizePolicy:
         - resourceName: cpu
           restartPolicy: NotRequired
         - resourceName: memory
           restartPolicy: NotRequired
     ```
   - Apply: `kubectl apply -f resize-demo.yaml`
   - Inspect initial resources: `kubectl get pod resize-demo -o jsonpath='{.status.containerStatuses[0].resources}'`
   - Resize: `kubectl patch pod resize-demo --subresource resize --patch '{"spec":{"containers":[{"name":"app","resources":{"requests":{"cpu":"200m"}}}]}}'`
   - Verify: re-run the inspect command; the new requests should be reflected without a pod restart (`status.containerStatuses[0].restartCount` stays 0).
   - `:::note` callout: pod-level resource sharing (`InPlacePodLevelResourcesVerticalScaling`) is **Beta** in 1.36 and is a separate feature with its own pod spec; this guide covers only the GA container-level path.

5. **Section: kubeadm v1beta4 note**
   - One short paragraph explaining that kind plans to adopt kubeadm `v1beta4` for K8s 1.36+ clusters (kind issue #3847). If you have existing `kubeadmConfigPatches` using v1beta3's `extraArgs` map syntax, you may need to update them to v1beta4's list-of-`{name, value}` syntax. Link to the upstream kubeadm config reference: https://kubernetes.io/docs/reference/config-api/kubeadm-config.v1beta4/

6. **Cleanup** — `kubectl delete pod userns-demo resize-demo`

7. **References** — bullet list of links:
   - Kubernetes 1.36 release notes: https://kubernetes.io/blog/2026/04/22/kubernetes-v1-36-release/ (use the closest plausible URL; if the release date 2026-04-22 in the research is the published date, the canonical URL pattern is `kubernetes.io/blog/<year>/<month>/<day>/kubernetes-v<X>-<Y>-release/` — but pages may move; if unsure, link to https://kubernetes.io/releases/ instead and let it redirect).
   - User Namespaces GA blog: https://kubernetes.io/blog/2026/04/23/kubernetes-v1-36-userns-ga/
   - In-Place Pod Resize GA (v1.35) blog: https://kubernetes.io/blog/2025/12/19/kubernetes-v1-35-in-place-pod-resize-ga/
   - kubeadm v1beta4 reference: https://kubernetes.io/docs/reference/config-api/kubeadm-config.v1beta4/

**Style requirements:**
- Use the exact callout syntax from existing guides (read `multi-version-clusters.md` for `:::tip` / `:::note` examples).
- All code blocks fenced with language tags (`yaml`, `bash`).
- No emojis (project convention — see Phase 47–50 plans).
- No level-1 heading inside the body (Starlight uses frontmatter `title` for the H1).
- Use H2 (`##`) for the four major sections, H3 (`###`) for subsections within each.

Run a local sanity build of the website after writing:
```bash
cd kinder-site && npm run build 2>&1 | tail -40
```
The build must succeed. If `npm` isn't available or fails for unrelated reasons, fall back to syntactic check: `node -e "require('fs').readFileSync('src/content/docs/guides/k8s-1-36-whats-new.md', 'utf8')"` and visually verify the frontmatter is valid YAML (open and bracket-count manually).

This task can be a SINGLE commit (content-only, no TDD per planner_guidance #4):
```
git add kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md
git commit -m "docs(51-03): add 'What's new in K8s 1.36' guide with userns + in-place resize"
```
  </action>
  <verify>
File exists at `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md`. `wc -l` returns ≥80. `grep -c "hostUsers: false" kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` returns ≥1. `grep -c "resizePolicy" kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` returns ≥1. If `npm run build` was runnable, it exited 0.
  </verify>
  <done>Guide page committed with frontmatter, two feature sections, kubeadm v1beta4 note, cleanup, and references. Site build succeeds (or is verified syntactically valid).</done>
</task>

<task type="auto">
  <name>Task 2: Register the guide in the Starlight sidebar</name>
  <files>
    kinder-site/astro.config.mjs
  </files>
  <action>
Read `kinder-site/astro.config.mjs` and locate the `Guides` sidebar entry (around line 74 — `label: 'Guides'`). The `items` array under it lists all current guides as `{ slug: 'guides/<filename-without-extension>' }` entries.

Add a new entry for the new guide. Place it AFTER the existing `multi-version-clusters` entry (chronologically the most recent K8s-version-related guide) — RESEARCH §SYNC-04 suggests this position. Use whatever ordering convention the existing items follow (alphabetical by slug or by topic-grouping); if alphabetical, place it where `k8s-1-36-whats-new` sorts.

```js
{ slug: 'guides/k8s-1-36-whats-new' },
```

Verify that no other entries collide (the slug must be unique). Run a syntactic JS check:
```bash
node --check kinder-site/astro.config.mjs
```
If `npm` is available locally, also run `cd kinder-site && npm run build 2>&1 | tail -30` and confirm the build still succeeds AND the build log shows the new guide being included (Astro typically logs `Generating /guides/k8s-1-36-whats-new/`).

Commit:
```
git add kinder-site/astro.config.mjs
git commit -m "docs(51-03): register K8s 1.36 guide in Starlight sidebar"
```
  </action>
  <verify>
`grep -n "k8s-1-36-whats-new" kinder-site/astro.config.mjs` returns one match in the Guides items array. `node --check kinder-site/astro.config.mjs` exits 0. If runnable, `cd kinder-site && npm run build` exits 0.
  </verify>
  <done>Sidebar entry added; site build succeeds; the page is reachable from the Guides nav.</done>
</task>

</tasks>

<verification>
- `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` exists, ≥80 lines, contains User Namespaces pod spec + resize pod spec + resize patch command + kubeadm v1beta4 note + reference links.
- `kinder-site/astro.config.mjs` lists `guides/k8s-1-36-whats-new` in the Guides items array.
- If `npm run build` is available locally, it exits 0.
- 2 commits landed on main.
</verification>

<success_criteria>
SC4: The kinder website has a "What's new in K8s 1.36" recipe page with working examples demonstrating User Namespaces (GA) and In-Place Pod Resize (GA) on a kinder cluster — satisfied by:
- New `k8s-1-36-whats-new.md` guide with both feature sections
- Pod specs match exactly the RESEARCH §SYNC-04 templates (verified to be runnable on a kinder 1.36 cluster)
- Sidebar registration in astro.config.mjs makes the page reachable
- Site build succeeds with the new entry
</success_criteria>

<output>
After completion, create `.planning/phases/51-upstream-sync-k8s-1-36/51-03-SUMMARY.md` with: commits landed (SHAs), final guide line count, sidebar entry placement (which existing guide it sits next to in the items array), and `npm run build` exit code if runnable.
</output>
