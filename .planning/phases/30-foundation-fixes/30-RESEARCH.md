# Phase 30: Foundation Fixes - Research

**Researched:** 2026-03-04
**Domain:** Astro + Starlight documentation site — content accuracy and navigation structure
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Addon presentation on landing page**
- Group addons as **core** (always-on: MetalLB, Metrics Server, CoreDNS) vs **optional** (opt-in: Envoy Gateway, Headlamp, Local Registry, cert-manager)
- Order by importance/usage within each group, most impactful first
- Each addon shows: name + one-liner description + concrete benefit (e.g., "MetalLB — LoadBalancer IPs → access services via localhost")
- Keep existing kind vs kinder side-by-side comparison layout, just update the addon list to include all 7

**Quick-start verification depth**
- Each addon gets a verification command + expected output so users can confirm success
- Group verification steps as core addons first, then optional — matching landing page grouping
- Default `kinder create` as the main path; introduce `--profile` flag in a tip/callout box (not up front)
- Add a "Something wrong?" section at the end pointing to `kinder doctor`

**Configuration page structure**
- Group addon fields by core vs optional — consistent with landing page and quick-start
- Each addon documented with YAML snippet + field table (field name, type, default, description)
- Full complete v1alpha4 config example at the top for copy-paste, then per-addon breakdowns below
- Prominent callout showing which addons are enabled by default vs require explicit opt-in

**Sidebar organization**
- New sections appear after Addons, before Changelog: Installation → Quick Start → Configuration → Addons → **Guides** → **CLI Reference** → Changelog
- All grouped sections (Addons, Guides, CLI Reference) are collapsible
- **Guides placeholders** (matching Phase 33): TLS Web App, HPA Auto-Scaling, Local Dev Workflow
- **CLI Reference placeholders** (matching Phase 32): Profile Comparison, JSON Output Reference, Troubleshooting (env/doctor)
- Placeholder pages contain just title + "coming soon" note — minimal content

### Claude's Discretion
- Exact wording of addon one-liners and benefit descriptions
- Visual styling of tip/callout boxes
- Ordering within importance-based sort (specific addon ranking)
- How to handle the full config example (collapsed vs expanded, annotations)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| FOUND-01 | Landing page Comparison component lists all 7 addons; description meta updated | Comparison.astro currently has 5 addons (missing Local Registry and cert-manager); index.mdx already has all 7 addon cards but meta description is stale |
| FOUND-02 | Quick-start page verifies all 7 addons and mentions --profile flag | quick-start.md currently covers 5 addons; missing Local Registry, cert-manager verification; missing --profile callout; missing "Something wrong?" section |
| FOUND-03 | Configuration page documents all 7 addon fields including localRegistry and certManager | configuration.md only documents 5 fields; missing `localRegistry` and `certManager`; Go types.go confirms both fields exist with `*bool` type, default true |
| FOUND-04 | Sidebar has Guides and CLI Reference sections with all new pages | astro.config.mjs sidebar has no Guides or CLI Reference sections; Starlight supports `collapsed: true` for collapsible groups |
</phase_requirements>

---

## Summary

This phase is a documentation accuracy and structure update for the kinder website (`kinder-site/`), which uses Astro v5 + @astrojs/starlight 0.37.6. The site already has all 7 addon pages in the addons directory, but three core pages (landing, quick-start, configuration) and the sidebar navigation are out of date with v1.3-v1.4 features.

The changes are isolated to four files plus six new placeholder files: `src/components/Comparison.astro`, `src/content/docs/quick-start.md`, `src/content/docs/configuration.md`, `astro.config.mjs` (sidebar), plus placeholder `.md` files in new `guides/` and `cli-reference/` content directories.

The locked decision to treat MetalLB, Metrics Server, and CoreDNS as "core" and Envoy Gateway, Headlamp, Local Registry, and cert-manager as "optional" is a **documentation grouping choice** — all 7 addons are enabled by default in the Go code (`default.go` calls `boolPtrTrue` on all 7 fields). This distinction should be surfaced as a conceptual "always-on" vs "easily disabled" framing.

**Primary recommendation:** Make targeted edits to 3 existing files and `astro.config.mjs`, then create 6 minimal placeholder content files. No framework changes, no component rewrites.

---

## Current State Audit

### What exists today vs what is needed

| File | Current State | Needed Change |
|------|---------------|---------------|
| `src/components/Comparison.astro` | 5 addons in kinder column (missing Local Registry, cert-manager) | Add 2 missing addons to kinder column; kind column needs 2 more "No X" items |
| `src/content/docs/index.mdx` | All 7 addon cards present; meta description stale | Update frontmatter `description` meta; re-organize cards into core/optional groups |
| `src/content/docs/quick-start.md` | Verifies 5 addons; missing Local Registry, cert-manager sections; no --profile mention; no "Something wrong?" | Add 2 verification sections; add --profile tip callout; add doctor section |
| `src/content/docs/configuration.md` | Documents 5 addon fields (no `localRegistry`, `certManager`) | Add both fields; restructure to core/optional grouping; add full YAML example at top |
| `astro.config.mjs` | No Guides or CLI Reference sidebar sections; Addons not collapsible | Add `collapsed: true` to Addons group; add Guides group; add CLI Reference group |
| `src/content/docs/guides/` | Directory does not exist | Create 3 placeholder `.md` files |
| `src/content/docs/cli-reference/` | Directory does not exist | Create 3 placeholder `.md` files |

---

## Standard Stack

### Core (confirmed from kinder-site/package.json and existing code)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Astro | ^5.6.1 | Site framework | Already in use; no change |
| @astrojs/starlight | ^0.37.6 | Documentation theme | Already in use; provides sidebar, MDX, callouts |

### Key Starlight features used in this phase

| Feature | Syntax | Confidence |
|---------|--------|------------|
| Collapsible sidebar group | `collapsed: true` on group object in `astro.config.mjs` | HIGH — verified with official Starlight docs |
| Callout/admonition | `:::tip`, `:::note`, `:::caution` in Markdown | HIGH — used throughout existing site |
| Frontmatter meta | `description:` field in `.md`/`.mdx` frontmatter | HIGH — confirmed in existing pages |
| MDX components | `import` + JSX in `.mdx` files | HIGH — `index.mdx` already uses this |

**No new packages needed.** `npm install` is not required for this phase.

---

## Architecture Patterns

### Recommended File Layout (end state)

```
kinder-site/src/
├── components/
│   └── Comparison.astro        # edit: add Local Registry, cert-manager rows
├── content/docs/
│   ├── index.mdx               # edit: update description meta, reorganize addon cards
│   ├── quick-start.md          # edit: add 2 addon verifications, --profile tip, doctor section
│   ├── configuration.md        # edit: add 2 addon fields, restructure, full YAML example
│   ├── addons/                 # existing — no changes in this phase
│   ├── guides/                 # new directory
│   │   ├── tls-web-app.md      # placeholder
│   │   ├── hpa-auto-scaling.md # placeholder
│   │   └── local-dev-workflow.md # placeholder
│   └── cli-reference/          # new directory
│       ├── profile-comparison.md   # placeholder
│       ├── json-output.md          # placeholder
│       └── troubleshooting.md      # placeholder
└── (other existing files unchanged)
```

### Pattern 1: Collapsible Sidebar Group

**What:** Groups of links with `collapsed: true` start folded and expand on click.
**When to use:** All three grouped sections (Addons, Guides, CLI Reference) per locked decisions.
**Example:**

```javascript
// astro.config.mjs — Source: Starlight official docs
sidebar: [
  { slug: 'installation' },
  { slug: 'quick-start' },
  { slug: 'configuration' },
  {
    label: 'Addons',
    collapsed: true,
    items: [
      { slug: 'addons/metallb' },
      { slug: 'addons/envoy-gateway' },
      { slug: 'addons/metrics-server' },
      { slug: 'addons/coredns' },
      { slug: 'addons/headlamp' },
      { slug: 'addons/local-registry' },
      { slug: 'addons/cert-manager' },
    ],
  },
  {
    label: 'Guides',
    collapsed: true,
    items: [
      { slug: 'guides/tls-web-app' },
      { slug: 'guides/hpa-auto-scaling' },
      { slug: 'guides/local-dev-workflow' },
    ],
  },
  {
    label: 'CLI Reference',
    collapsed: true,
    items: [
      { slug: 'cli-reference/profile-comparison' },
      { slug: 'cli-reference/json-output' },
      { slug: 'cli-reference/troubleshooting' },
    ],
  },
  { slug: 'changelog' },
],
```

### Pattern 2: Placeholder Content Page

**What:** Minimal `.md` file with frontmatter only + one-liner "coming soon" note.
**When to use:** All 6 new guide and CLI reference pages.
**Example:**

```markdown
---
title: TLS Web App
description: Step-by-step guide to deploying a TLS-secured web app with kinder.
---

:::note[Coming soon]
This guide is in progress. Check back after the next release.
:::
```

### Pattern 3: Starlight Callout/Admonition

**What:** Block-level callout for tips, notes, cautions.
**When to use:** --profile tip callout in quick-start; "enabled by default" callout in configuration.
**Available types (confirmed in existing site):** `:::tip`, `:::note`, `:::caution`
**Syntax with optional title:**

```markdown
:::tip[Power user: addon profiles]
`kinder create cluster --profile minimal` creates a plain kind cluster with no addons.
Available profiles: `minimal`, `full`, `gateway`, `ci`.
:::
```

### Pattern 4: Starlight meta description update

**What:** `description:` in frontmatter controls both sidebar subtitle and HTML `<meta name="description">`.
**When to use:** `index.mdx` description needs updating to mention all 7 addons.

Current (stale):
```yaml
description: kind with MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, and Headlamp pre-installed.
```

Should mention all 7 (Claude's discretion on exact wording).

### Anti-Patterns to Avoid

- **Renaming existing slug paths:** The existing addons slugs (`addons/metallb`, etc.) must not change — inbound links and the existing sidebar config depend on them.
- **Changing `.mdx` to `.md` or vice versa:** `index.mdx` must stay `.mdx` because it uses JSX component imports. New placeholder files should be plain `.md`.
- **Hardcoding sidebar labels that diverge from page titles:** Use `{ slug: 'guides/tls-web-app' }` (auto-derives label from frontmatter `title`) rather than specifying `label:` separately, to keep labels in sync.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Collapsible nav groups | Custom JavaScript/CSS toggle | Starlight `collapsed: true` in config | Built-in, keyboard accessible, no code needed |
| Callout boxes | Raw `<div>` with inline styles | Starlight `:::tip`, `:::note` directives | Consistent theming, screen-reader accessible |
| "Coming soon" page | Multi-paragraph placeholder with fake content | One `:::note` callout with two lines | Placeholder is temporary; minimal is correct |

**Key insight:** Every UI need in this phase is already solved by Starlight primitives. No new Astro components required.

---

## Common Pitfalls

### Pitfall 1: Comparison.astro parity — kind column must match kinder column

**What goes wrong:** Adding 2 new addons to the kinder column without adding corresponding "No X" entries to the kind column breaks the visual parity of the two-column layout. Users compare row-by-row.
**Why it happens:** The component is hand-authored HTML — there's no data binding enforcing column length equality.
**How to avoid:** For each new kinder addon added, add a corresponding "No X" or "No dashboard" line to the kind column in the same position.
**Warning signs:** The two `<ul>` elements have different `<li>` counts.

### Pitfall 2: Sidebar slug must match content file path exactly

**What goes wrong:** `{ slug: 'guides/tls-web-app' }` requires a file at `src/content/docs/guides/tls-web-app.md`. If the file is missing or the path differs (e.g., `tls-webapp.md`), `astro build` fails with a content collection error.
**Why it happens:** Starlight validates all slugs at build time against the content collection.
**How to avoid:** Create the content files before or at the same time as the sidebar config. Match filenames exactly to slugs.
**Warning signs:** `astro build` error: "Could not find entry for slug..."

### Pitfall 3: Configuration page — all 7 addons are enabled by default

**What goes wrong:** The current configuration.md says "All addons are enabled by default; set a field to `false` to skip installation." This is still true for all 7 addons. The "core vs optional" distinction the user wants is conceptual, not a default-enabled vs default-disabled distinction.
**Why it happens:** The CONTEXT.md decision uses "core/optional" language that could be misread as "default-on vs default-off."
**How to avoid:** The callout on the configuration page should say something like: "All 7 addons are enabled by default. The core group (MetalLB, Metrics Server, CoreDNS) is always useful for any workflow. The optional group (Envoy Gateway, Headlamp, Local Registry, cert-manager) is also enabled by default but commonly disabled for lightweight clusters."
**Warning signs:** If the config page implies optional addons are off by default, it contradicts the Go source code (`default.go` calls `boolPtrTrue` on all 7 fields).

### Pitfall 4: Quick-start cluster creation output is stale

**What goes wrong:** The quick-start page shows example cluster creation output that lists only 5 addons (MetalLB, Metrics Server, CoreDNS Tuning, Envoy Gateway, Dashboard). The actual output from v1.3+ includes Local Registry and cert-manager.
**Why it happens:** Output was written for the original 5-addon version and not updated.
**How to avoid:** Update the sample terminal output block to show all 7 addons in the creation summary.
**Warning signs:** Missing "Local Registry" and "cert-manager" lines in the output example.

### Pitfall 5: --profile flag ordering in quick-start

**What goes wrong:** Introducing `--profile` too early in the quick-start makes it feel like a required decision, not a power-user escape hatch.
**Why it happens:** It's tempting to put it at the top next to `kinder create cluster`.
**How to avoid:** Per the locked decision — introduce `--profile` in a `:::tip` callout after the main create flow, not before or alongside it.
**Warning signs:** `--profile` appears before the "Create a Cluster" section or in the main code block.

---

## Code Examples

Verified patterns from existing site and official sources:

### Updated Comparison.astro (kind vs kinder columns)

```html
<!-- src/components/Comparison.astro -->
<!-- kind column: 9 items (2 new "No X" entries) -->
<div class="column">
  <h3>kind</h3>
  <ul>
    <li>Bare Kubernetes cluster</li>
    <li>No LoadBalancer support</li>
    <li>No ingress controller</li>
    <li>No metrics (kubectl top fails)</li>
    <li>Default CoreDNS config</li>
    <li>No dashboard</li>
    <li>No local registry</li>
    <li>No TLS certificates</li>
  </ul>
</div>
<!-- kinder column: 9 items (2 new entries) -->
<div class="column kinder">
  <h3>kinder</h3>
  <ul>
    <li>Kubernetes cluster</li>
    <li>MetalLB — real LoadBalancer IPs</li>
    <li>Envoy Gateway — Gateway API routing</li>
    <li>Metrics Server — kubectl top and HPA</li>
    <li>CoreDNS — autopath, pods verified, 60s cache</li>
    <li>Headlamp — web dashboard</li>
    <li>Local Registry — push images to localhost:5001</li>
    <li>cert-manager — automatic TLS certificates</li>
  </ul>
</div>
```

### Full v1alpha4 config example (for configuration.md top section)

```yaml
# Full kinder v1alpha4 configuration — all fields shown
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
addons:
  # Core addons (always useful — enabled by default)
  metalLB: true
  metricsServer: true
  coreDNSTuning: true
  # Optional addons (enabled by default — disable for lightweight clusters)
  envoyGateway: true
  dashboard: true
  localRegistry: true
  certManager: true
```

### Configuration page — per-addon field table structure

```markdown
### MetalLB

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `metalLB` | `bool` | `true` | Install MetalLB for LoadBalancer IP assignment |

```yaml
addons:
  metalLB: true   # set to false to disable
```

### Local Registry

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `localRegistry` | `bool` | `true` | Run a private container registry at localhost:5001 |

```yaml
addons:
  localRegistry: true   # set to false to disable
```

### cert-manager

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `certManager` | `bool` | `true` | Install cert-manager with a self-signed ClusterIssuer |

```yaml
addons:
  certManager: true   # set to false to disable
```
```

### Quick-start Local Registry verification section

```markdown
### Local Registry

Confirm the registry container is running and accessible:

```sh
docker ps --filter name=kind-registry
```

Expected output:

```
CONTAINER ID   IMAGE        COMMAND                  PORTS                    NAMES
abc123def456   registry:2   "/entrypoint.sh /etc…"   0.0.0.0:5001->5000/tcp   kind-registry
```

Verify dev tool discovery ConfigMap is present:

```sh
kubectl get configmap local-registry-hosting -n kube-public
```

Expected output:

```
NAME                     DATA   AGE
local-registry-hosting   1      60s
```
```

### Quick-start cert-manager verification section

```markdown
### cert-manager

Check all three cert-manager components are running:

```sh
kubectl get pods -n cert-manager
```

Expected output:

```
NAME                                       READY   STATUS    RESTARTS   AGE
cert-manager-...                           1/1     Running   0          60s
cert-manager-cainjector-...                1/1     Running   0          60s
cert-manager-webhook-...                   1/1     Running   0          60s
```

Confirm the self-signed ClusterIssuer is ready:

```sh
kubectl get clusterissuer selfsigned-issuer
```

Expected output:

```
NAME                READY   AGE
selfsigned-issuer   True    60s
```
```

### Quick-start "Something wrong?" section

```markdown
## Something wrong?

Run `kinder doctor` to check prerequisites and identify issues:

```sh
kinder doctor
```

This checks that Docker (or Podman/nerdctl), kubectl, and other dependencies are installed and reachable.
```

### Quick-start --profile tip callout

```markdown
:::tip[Addon profiles]
`kinder create cluster` enables all 7 addons by default. Use `--profile` to select a preset:

- `--profile minimal` — no kinder addons (plain kind cluster)
- `--profile gateway` — MetalLB + Envoy Gateway only
- `--profile ci` — Metrics Server + cert-manager (CI-optimized)
- `--profile full` — all addons (same as default)
:::
```

---

## Exact YAML Field Names (from Go source — authoritative)

Source: `pkg/apis/config/v1alpha4/types.go`

| Go Field | YAML key | Default |
|----------|----------|---------|
| `MetalLB` | `metalLB` | `true` |
| `EnvoyGateway` | `envoyGateway` | `true` |
| `MetricsServer` | `metricsServer` | `true` |
| `CoreDNSTuning` | `coreDNSTuning` | `true` |
| `Dashboard` | `dashboard` | `true` |
| `LocalRegistry` | `localRegistry` | `true` |
| `CertManager` | `certManager` | `true` |

All 7 are `*bool` with `true` default (set by `boolPtrTrue` in `default.go`). The current configuration.md incorrectly omits `localRegistry` and `certManager`.

---

## Core vs Optional Grouping Rationale

The locked decision groups addons into core (MetalLB, Metrics Server, CoreDNS) and optional (Envoy Gateway, Headlamp, Local Registry, cert-manager). This is a **conceptual UX grouping**, not a technical default distinction — all 7 are default-enabled. The documentation should make this explicit.

Suggested framing: "Core addons are always useful regardless of your workload. Optional addons are powerful but commonly disabled for minimal or CI clusters."

This framing is consistent with the `ci` profile in `createoption.go` (which includes only MetricsServer + CertManager, suggesting cert-manager is sometimes categorized as CI-useful — but per the locked decision, cert-manager is in the "optional" group for the docs grouping).

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `astro build` (compile-time validation) |
| Config file | `kinder-site/astro.config.mjs` |
| Quick run command | `cd kinder-site && npm run build` |
| Full suite command | `cd kinder-site && npm run build` |

No unit test framework is installed in `kinder-site/`. The `package.json` has only `dev`, `start`, `build`, `preview`, and `astro` scripts — no `test` script. Validation is build-time: `astro build` catches broken slugs, missing files, and MDX syntax errors.

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FOUND-01 | Comparison component shows 7 addons | manual-only (visual) | `cd kinder-site && npm run build` (verifies no build error) | ✅ Comparison.astro exists |
| FOUND-01 | index.mdx description updated | manual-only (grep) | `grep "Local Registry\|cert-manager" kinder-site/src/content/docs/index.mdx` | ✅ |
| FOUND-02 | Quick-start covers 7 addon verifications | manual-only (review) | `cd kinder-site && npm run build` | ✅ quick-start.md exists |
| FOUND-02 | --profile flag mentioned | manual-only (grep) | `grep -- "--profile" kinder-site/src/content/docs/quick-start.md` | ✅ |
| FOUND-03 | Configuration page has localRegistry field | manual-only (grep) | `grep "localRegistry\|certManager" kinder-site/src/content/docs/configuration.md` | ✅ |
| FOUND-04 | Sidebar has Guides and CLI Reference | build validation | `cd kinder-site && npm run build` (fails on missing slugs) | ❌ Wave 0 needed |

### Wave 0 Gaps

- [ ] `kinder-site/src/content/docs/guides/tls-web-app.md` — required by sidebar slug `guides/tls-web-app`
- [ ] `kinder-site/src/content/docs/guides/hpa-auto-scaling.md` — required by sidebar slug `guides/hpa-auto-scaling`
- [ ] `kinder-site/src/content/docs/guides/local-dev-workflow.md` — required by sidebar slug `guides/local-dev-workflow`
- [ ] `kinder-site/src/content/docs/cli-reference/profile-comparison.md` — required by sidebar slug `cli-reference/profile-comparison`
- [ ] `kinder-site/src/content/docs/cli-reference/json-output.md` — required by sidebar slug `cli-reference/json-output`
- [ ] `kinder-site/src/content/docs/cli-reference/troubleshooting.md` — required by sidebar slug `cli-reference/troubleshooting`

These must be created before or alongside the `astro.config.mjs` sidebar update, or the build will fail.

---

## Sources

### Primary (HIGH confidence)

- Direct file inspection: `kinder-site/src/content/docs/` — current content state confirmed
- `pkg/apis/config/v1alpha4/types.go` — authoritative YAML field names for all 7 addons
- `pkg/apis/config/v1alpha4/default.go` — confirms all 7 addons default to `true`
- `pkg/cluster/createoption.go` — confirms 4 profile presets: minimal, full, gateway, ci
- `kinder-site/astro.config.mjs` — current sidebar structure confirmed
- `kinder-site/src/components/Comparison.astro` — confirmed 5-addon kinder column (missing 2)
- Starlight official docs (WebFetch) — `collapsed: true` sidebar syntax verified

### Secondary (MEDIUM confidence)

- `kinder-site/src/content/docs/addons/local-registry.md` — verification commands for local registry
- `kinder-site/src/content/docs/addons/cert-manager.md` — verification commands for cert-manager
- `kinder-site/src/content/docs/changelog.md` — confirmed v0.3.0-alpha added localRegistry and certManager

### Tertiary (LOW confidence)

None — all claims verified from source files.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — existing site dependencies confirmed from package.json
- Architecture: HIGH — all current file locations verified by direct inspection
- Pitfalls: HIGH — all pitfalls derived from actual code/content gaps found during research
- YAML field names: HIGH — sourced directly from Go types.go

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (stable content; Starlight API unlikely to change)
