# Phase 11: Documentation Content - Research

**Researched:** 2026-03-01
**Domain:** Starlight MDX content authoring, Astro sidebar configuration, Pagefind search, kinder addon technical details
**Confidence:** HIGH

## Summary

Phase 11 writes all eight documentation pages for kinder. The site already runs Astro 5.6.1 with Starlight 0.37.6 and the dark terminal theme. All that is missing is content: markdown files in `kinder-site/src/content/docs/` that cover installation, quick start, configuration reference, and five addon pages.

The work is almost entirely content authoring in Markdown/MDX with no new npm packages, no new build configuration, and no new components. The one structural change is configuring the `sidebar` array in `astro.config.mjs` so the eight pages appear in the correct order. Pagefind search indexes all content at build time automatically — no extra configuration is needed for search to work once pages exist.

The critical blocker from STATE.md must be resolved before writing DOCS-01 (installation guide): the binary distribution method is unconfirmed. The installation guide cannot be accurate without knowing whether users run `go install sigs.k8s.io/kind@<tag>`, download a binary from GitHub Releases, or use a Homebrew tap. All three methods are technically available (the module path is `sigs.k8s.io/kind`, `make build` produces a `kinder` binary, and the Makefile uses `KIND_BINARY_NAME?=kinder`), but none is confirmed as the official distribution method. The planner must make this decision before DOCS-01 can be executed.

**Primary recommendation:** Write the eight pages as plain Markdown (`.md`) files under a `docs/` folder structure, configure the sidebar explicitly in `astro.config.mjs`, and derive all technical details (config fields, defaults, addon behaviors) directly from the Go source already read in this research.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DOCS-01 | Installation guide page covering `go install` and binary download | Go module path `sigs.k8s.io/kind`, Makefile confirms binary name `kinder`, `.go-version` is 1.25.7 — content ready pending distribution method decision |
| DOCS-02 | Quick start page walking through `kinder create cluster` and verifying addons | Addon output strings (`ctx.Status.Start(...)`) read from source; verification commands derivable from addon code |
| DOCS-03 | Configuration reference page documenting `v1alpha4` addons schema and all config options | All `Addons` struct fields with comments and defaults read from `types.go` and `default.go` |
| DOCS-04 | MetalLB addon documentation page | `metallb.go` and `subnet.go` read in full; IP pool logic, rootless warning, CRs documented |
| DOCS-05 | Envoy Gateway addon documentation page | `envoygw.go` read in full; GatewayClass YAML, server-side apply requirement, wait steps documented |
| DOCS-06 | Metrics Server addon documentation page | `metricsserver.go` read in full; deployment name, namespace documented |
| DOCS-07 | CoreDNS tuning addon documentation page | `corednstuning.go` read in full; exact Corefile transforms (autopath, pods verified, cache 60) documented |
| DOCS-08 | Headlamp dashboard addon documentation page | `dashboard.go` read in full; token secret name, port-forward command, access URL printed by kinder documented |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| @astrojs/starlight | 0.37.6 (installed) | Documentation framework with built-in sidebar, search, TOC | Already installed; this phase uses its content layer |
| astro | 5.6.1 (installed) | Build system | Already installed; no change |
| Markdown (.md) | N/A | Content format for all 8 pages | Simpler than MDX for text-heavy docs; MDX only needed for interactive components |

### Supporting
| Tool | Purpose | When to Use |
|------|---------|-------------|
| Starlight `Aside` (MDX import) | Notes, warnings, tips callouts | When a page needs a `:::note`, `:::tip`, or `:::warning` callout — available in .md via `:::` syntax without MDX |
| Expressive Code (built-in) | Code blocks with syntax highlighting, terminal frames, titles | Every page has code blocks; uses fenced code blocks with language identifiers |
| Pagefind (built-in) | Full-text search index | Built at `npm run build` time; no config needed for basic search |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Plain Markdown (.md) | MDX (.mdx) | MDX needed only if importing React/Astro components inline; these pages need no custom components — use .md |
| Explicit sidebar config | `autogenerate: { directory: '...' }` | Autogenerate sorts alphabetically; explicit config gives precise ordering required by the UX |
| Separate section directories | Flat `docs/` folder | Section directories (guides/, reference/, addons/) match Starlight's recommended structure and allow autogenerate for addon sub-section |

**No new npm packages required.** All eight pages are Markdown + sidebar configuration.

## Architecture Patterns

### Recommended Project Structure
```
kinder-site/src/content/docs/
├── index.mdx                    # Existing: placeholder (NOT modified this phase)
├── installation.md              # DOCS-01: go install + binary download
├── quick-start.md               # DOCS-02: kinder create cluster walkthrough
├── configuration.md             # DOCS-03: v1alpha4 addons schema reference
└── addons/
    ├── metallb.md               # DOCS-04: MetalLB addon
    ├── envoy-gateway.md         # DOCS-05: Envoy Gateway addon
    ├── metrics-server.md        # DOCS-06: Metrics Server addon
    ├── coredns.md               # DOCS-07: CoreDNS tuning addon
    └── headlamp.md              # DOCS-08: Headlamp dashboard addon
```

### Pattern 1: Sidebar Configuration in astro.config.mjs

**What:** Explicitly define the sidebar order so pages appear in a logical reading order rather than alphabetical.

**When to use:** Always when page ordering matters (installation before quick start before reference).

**Example:**
```javascript
// Source: https://starlight.astro.build/reference/configuration/
// kinder-site/astro.config.mjs
starlight({
  title: 'kinder',
  sidebar: [
    { slug: 'installation', label: 'Installation' },
    { slug: 'quick-start', label: 'Quick Start' },
    { slug: 'configuration', label: 'Configuration Reference' },
    {
      label: 'Addons',
      autogenerate: { directory: 'addons' },
    },
  ],
  // ... rest of config
})
```

### Pattern 2: Starlight Markdown Page Frontmatter

**What:** Every page requires `title`. Add `description` for search engine summaries and Pagefind metadata.

**Example:**
```markdown
---
title: Installation
description: How to install kinder using go install or a pre-built binary.
sidebar:
  order: 1
---
```

### Pattern 3: Code Blocks with Expressive Code

**What:** Fenced code blocks with language identifiers get syntax highlighting and terminal frames automatically.

**Example:**
```markdown
<!-- Shell commands get terminal frame styling automatically -->
```bash
kinder create cluster
```

<!-- YAML with title -->
```yaml title="kinder-config.yaml"
kind: Cluster
apiVersion: kinder.dev/v1alpha4
```

<!-- Terminal output (no language = plain) or use console -->
```console
Creating cluster "kind" ...
 ✓ Ensuring node image (kindest/node:v1.32.2) 🖼
 ✓ Preparing nodes 📦
 ✓ Installing MetalLB
 ✓ Installing Envoy Gateway
```
```

### Pattern 4: Asides for Notes and Warnings

**What:** Triple-colon syntax for callout boxes. Available in plain `.md` files in Starlight.

**When to use:** Platform caveats, important notes, tips for verifying setup.

**Example:**
```markdown
:::note
MetalLB's L2 speaker cannot send ARP in rootless Podman mode.
LoadBalancer services may not receive an external IP in that environment.
:::

:::tip
Run `kubectl get nodes` to verify the cluster is ready before proceeding.
:::

:::caution
Do not run `kinder create cluster` inside a Docker container — MetalLB requires
access to the host network bridge.
:::
```

### Pattern 5: Page Content Structure (Diataxis)

**What:** Apply Diataxis framework roles per page type for consistent quality.

| Page | Diataxis Type | Structure |
|------|--------------|-----------|
| Installation (DOCS-01) | How-to guide | Prerequisites → Method 1: go install → Method 2: binary download → Verify |
| Quick Start (DOCS-02) | Tutorial | Step 1 → Step 2 → ... → Step N → What you get |
| Configuration Reference (DOCS-03) | Reference | Schema table → Field descriptions → Example YAML |
| Each addon page (DOCS-04–08) | Explanation + Reference | What it does → What you get → Config field → How to disable → Platform notes |

### Anti-Patterns to Avoid
- **Duplicate content in index.mdx:** Do NOT replace or heavily edit the existing `index.mdx` — that page is reserved for Phase 12's landing page; this phase adds new pages alongside it.
- **Putting all docs in a flat structure:** Group addon pages under `addons/` directory to allow autogenerate and clean sidebar nesting.
- **MDX for pages with no components:** MDX adds parsing overhead; use `.md` unless a page needs to import an Astro/React component.
- **Missing `description` frontmatter:** Pagefind and social previews use `description`; every page must have it.
- **Sidebar ordering without explicit config:** Starlight's `autogenerate` sorts alphabetically. Top-level pages (installation, quick-start, configuration) must be in the `sidebar` array explicitly to control order.
- **Writing the install command before deciding distribution method:** Placeholder text in DOCS-01 blocks verification. Either decide the distribution method first, or write the guide with `go install` as the only confirmed method and note binary releases as "coming soon."

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Search index | Custom search JSON, Lunr, Fuse.js | Pagefind (built into Starlight build) | Pagefind indexes at build time, shipped as static files, zero config needed |
| Table of contents | Custom TOC component | Starlight's built-in TOC (automatic) | Starlight generates TOC from `##` headings automatically on every doc page |
| Sidebar navigation | Custom nav component | `sidebar` config in `astro.config.mjs` | Starlight renders sidebar from config; supports groups, autogenerate, badges |
| Code highlighting | Prism.js, highlight.js | Expressive Code (built into Starlight) | Already installed; zero config for syntax highlighting + terminal frames |
| Callout boxes | Custom CSS + div | `:::note`, `:::tip`, `:::warning` | Starlight's Aside component, accessible via `:::` syntax in plain Markdown |

**Key insight:** Starlight provides everything needed for documentation. This phase is a pure content-writing exercise — no engineering work beyond sidebar configuration and file creation.

## Common Pitfalls

### Pitfall 1: Using go install with Wrong Module Path
**What goes wrong:** The installation guide instructs users to run `go install sigs.k8s.io/kind@...` which installs `kind`, not `kinder`. The kinder module path is `sigs.k8s.io/kind` (inherited from the fork), but the binary is renamed to `kinder` by the Makefile via `KIND_BINARY_NAME?=kinder`. `go install` installs based on the binary name in the package's `main.go`, which is `cmd/kind/main.go` — Go will install it as `kind` (the directory name), not `kinder`.
**Why it happens:** The fork renamed the binary in the Makefile but `go install` uses the package directory name (`kind`) not the `KIND_BINARY_NAME` variable.
**How to avoid:** Verify whether `go install sigs.k8s.io/kind@latest` produces a `kind` or `kinder` binary before writing DOCS-01. If `go install` produces `kind`, either: (a) document binary download from GitHub Releases as the primary install method, (b) document building from source (`make install`), or (c) use a `cmd/kinder/main.go` wrapper. This is the blocker noted in STATE.md.
**Warning signs:** Users follow the installation guide and end up with a `kind` binary instead of `kinder`.

### Pitfall 2: Sidebar Pages Not Appearing
**What goes wrong:** Pages exist in `src/content/docs/` but don't appear in the sidebar.
**Why it happens:** Without a `sidebar` config, Starlight defaults to autogenerating from all pages. But once you add a `sidebar` config, only explicitly listed pages or explicitly autogenerated directories appear. Pages outside configured entries are hidden.
**How to avoid:** When adding a `sidebar` config, either list every page explicitly or use `autogenerate` for directories that should be included. Verify by building: `npm run build` will warn about orphaned pages in some versions.
**Warning signs:** New pages exist as files but are missing from the sidebar after adding sidebar config.

### Pitfall 3: Pagefind Not Indexing New Pages
**What goes wrong:** Search returns no results for content on new pages.
**Why it happens:** Pagefind only indexes the built output. If you test search against `npm run dev` (development server), Pagefind is not active — it only runs after `npm run build`. Testing search requires serving the `dist/` directory.
**How to avoid:** Verify search only against the built output: `npm run build && npm run preview`. Do NOT test Pagefind against the dev server.
**Warning signs:** Search bar doesn't find content from new pages during `npm run dev` — this is expected and not a bug.

### Pitfall 4: MetalLB Platform Caveat Omitted
**What goes wrong:** The MetalLB docs make no mention of the rootless Podman limitation, leaving users to discover failures on their own.
**Why it happens:** The warning exists in the Go source (`ctx.Logger.Warn("MetalLB L2 speaker cannot send ARP in rootless mode...")`) but is easy to overlook when writing docs.
**How to avoid:** DOCS-04 must include a `:::caution` or `:::note` documenting the rootless Podman behavior: MetalLB L2 speaker cannot send ARP in rootless mode; LoadBalancer services may not receive EXTERNAL-IP.
**Warning signs:** No platform note exists on the MetalLB page; users on rootless Podman report broken LoadBalancer services.

### Pitfall 5: Envoy Gateway --server-side Apply Not Explained
**What goes wrong:** Users who attempt to manually apply the Envoy Gateway manifest fail because `kubectl apply -f` (without `--server-side`) fails for manifests > 256 KB. The docs don't explain why kinder uses server-side apply internally.
**Why it happens:** This detail is internal to kinder's implementation and users don't see it. But if they ever try to re-apply the manifest manually, they hit the error.
**How to avoid:** DOCS-05 should include a note explaining that kinder uses `kubectl apply --server-side` for Envoy Gateway because the HTTPRoutes CRD exceeds the 256 KB annotation limit for standard apply. This is a real user gotcha.
**Warning signs:** No mention of server-side apply on the Envoy Gateway page.

### Pitfall 6: Headlamp Token Regeneration Not Documented
**What goes wrong:** Users close the terminal without saving the token kinder prints, then can't access the dashboard.
**Why it happens:** kinder prints the token once to stdout during `kinder create cluster`. If missed, users don't know how to retrieve it.
**How to avoid:** DOCS-08 must explain that kinder prints the token once and how to retrieve it manually: `kubectl get secret kinder-dashboard-token -n kube-system -o jsonpath='{.data.token}' | base64 -d`.
**Warning signs:** DOCS-08 only shows the port-forward command but not how to get the token if the user didn't save it from cluster creation output.

## Code Examples

Verified patterns from the actual Go source:

### Addon Config YAML (from types.go and default.go)
```yaml
# Source: pkg/apis/config/v1alpha4/types.go
# All addons default to true; set false to disable
kind: Cluster
apiVersion: kinder.dev/v1alpha4
addons:
  metalLB: false          # Disable MetalLB
  envoyGateway: false     # Disable Envoy Gateway
  metricsServer: false    # Disable Metrics Server
  coreDNSTuning: false    # Disable CoreDNS tuning
  dashboard: false        # Disable Headlamp dashboard
```

### MetalLB IP Pool (from metallb.go crTemplate)
```yaml
# Source: pkg/cluster/internal/create/actions/installmetallb/metallb.go
# Applied automatically — shows what kinder creates for LoadBalancer IPs
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kind-pool
  namespace: metallb-system
spec:
  addresses:
  - 172.18.255.200-172.18.255.250  # Carved from host's kind network subnet
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: kind-l2advert
  namespace: metallb-system
spec:
  ipAddressPools:
  - kind-pool
```

### Envoy Gateway GatewayClass (from envoygw.go)
```yaml
# Source: pkg/cluster/internal/create/actions/installenvoygw/envoygw.go
# Applied automatically — users reference this GatewayClass in their Gateway resources
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
```

### CoreDNS Corefile Changes (from corednstuning.go patchCorefile)
```
# Source: pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go
# Three transforms applied to the default Corefile:
# DNS-01: inserts "autopath @kubernetes" before kubernetes block
# DNS-02: "pods insecure" -> "pods verified"
# DNS-03: "cache 30" -> "cache 60"

# Before (kind default):
.:53 {
    kubernetes cluster.local ...
    pods insecure
    cache 30
    ...
}

# After (kinder):
.:53 {
    autopath @kubernetes
    kubernetes cluster.local ...
    pods verified
    cache 60
    ...
}
```

### Headlamp Access (from dashboard.go)
```bash
# Source: pkg/cluster/internal/create/actions/installdashboard/dashboard.go
# kinder prints this during cluster creation:
# Dashboard:
#   Token: <long-lived-token-value>
#   Port-forward: kubectl port-forward -n kube-system service/headlamp 8080:80
#   Then open: http://localhost:8080

# To retrieve token manually after creation:
kubectl get secret kinder-dashboard-token -n kube-system \
  -o jsonpath='{.data.token}' | base64 -d
```

### Sidebar Configuration Pattern
```javascript
// Source: https://starlight.astro.build/reference/configuration/
// astro.config.mjs — full sidebar config for Phase 11
starlight({
  title: 'kinder',
  sidebar: [
    { slug: 'installation' },
    { slug: 'quick-start' },
    { slug: 'configuration' },
    {
      label: 'Addons',
      items: [
        { slug: 'addons/metallb' },
        { slug: 'addons/envoy-gateway' },
        { slug: 'addons/metrics-server' },
        { slug: 'addons/coredns' },
        { slug: 'addons/headlamp' },
      ],
    },
  ],
})
```

## Content Specification

Each page's required content, derived from the Go source. Planner should use these as task inputs.

### DOCS-01: Installation (installation.md)

**Required sections:**
1. Prerequisites: Go 1.25.7+, Docker/Podman/Nerdctl
2. Method 1: `go install` — **BLOCKED: see Open Questions #1**
3. Method 2: Build from source — `git clone`, `make install`
4. Verify: `kinder version` or `kinder --help`

**Key fact:** Binary name is `kinder`, module path is `sigs.k8s.io/kind` (inherited from fork).

### DOCS-02: Quick Start (quick-start.md)

**Required 5 steps (maps to success criterion "5 steps"):**
1. Create cluster: `kinder create cluster`
2. Verify nodes: `kubectl get nodes`
3. Verify MetalLB: `kubectl get pods -n metallb-system`
4. Verify Envoy Gateway + GatewayClass: `kubectl get gatewayclass eg`
5. Open dashboard: `kubectl port-forward -n kube-system service/headlamp 8080:80` → open http://localhost:8080

**What kinder prints during creation (from Go source `ctx.Status.Start` messages):**
```
Creating cluster "kind" ...
 ✓ Ensuring node image
 ✓ Preparing nodes
 ✓ Installing MetalLB
 ✓ Installing Envoy Gateway
 ✓ Installing Metrics Server
 ✓ Tuning CoreDNS
 ✓ Installing Dashboard
```

### DOCS-03: Configuration Reference (configuration.md)

**Required content — all from types.go and default.go:**

| Field | YAML Key | Type | Default | Description |
|-------|----------|------|---------|-------------|
| MetalLB | `metalLB` | bool | true | Enables MetalLB for LoadBalancer services |
| Envoy Gateway | `envoyGateway` | bool | true | Enables Envoy Gateway + Gateway API CRDs |
| Metrics Server | `metricsServer` | bool | true | Enables Kubernetes Metrics Server |
| CoreDNS Tuning | `coreDNSTuning` | bool | true | Enables CoreDNS autopath + cache tuning |
| Dashboard | `dashboard` | bool | true | Enables Headlamp dashboard |

**API version:** `kinder.dev/v1alpha4` (NOTE: verify actual apiVersion value — types.go doesn't show it, check yaml.go)

### DOCS-04: MetalLB (addons/metallb.md)

**What it installs:** MetalLB v0.15.3 (from PROJECT.md), deployed to `metallb-system` namespace
**What users get:** Any Service of type LoadBalancer gets an external IP from the auto-detected pool
**IP pool:** Carved from the last .200-.250 of the `kind` Docker/Podman network subnet (e.g., 172.18.255.200-172.18.255.250)
**Config field:** `addons.metalLB: false` to disable
**Platform note:** L2 speaker cannot send ARP in rootless Podman mode — LoadBalancer EXTERNAL-IP may not be assigned

### DOCS-05: Envoy Gateway (addons/envoy-gateway.md)

**What it installs:** Envoy Gateway v1.3.1 (from PROJECT.md), deployed to `envoy-gateway-system` namespace
**What users get:** Gateway API CRDs, `GatewayClass` named `eg` with controllerName `gateway.envoyproxy.io/gatewayclass-controller`
**Config field:** `addons.envoyGateway: false` to disable
**Technical note:** Uses `kubectl apply --server-side` because the HTTPRoutes CRD is 372 KB (> 256 KB standard apply limit)
**Usage example:** Reference `eg` GatewayClass in Gateway resources to create HTTP/HTTPS routing

### DOCS-06: Metrics Server (addons/metrics-server.md)

**What it installs:** Metrics Server v0.8.1 (from PROJECT.md), deployed as `deployment/metrics-server` in `kube-system`
**What users get:** `kubectl top nodes`, `kubectl top pods`, HPA (Horizontal Pod Autoscaler) support
**Config field:** `addons.metricsServer: false` to disable
**Note:** Deployed with `--kubelet-insecure-tls` (confirmed in PROJECT.md) for compatibility with kind's self-signed kubelet certs

### DOCS-07: CoreDNS Tuning (addons/coredns.md)

**What it changes:** Three Corefile patches applied to the existing CoreDNS deployment in `kube-system`
**Changes (from corednstuning.go):**
1. `autopath @kubernetes` inserted before `kubernetes cluster.local` block — enables short DNS names without namespace qualification
2. `pods insecure` → `pods verified` — enables pod DNS record security validation
3. `cache 30` → `cache 60` — doubles DNS cache TTL for better performance
**Config field:** `addons.coreDNSTuning: false` to disable

### DOCS-08: Headlamp Dashboard (addons/headlamp.md)

**What it installs:** Headlamp v0.40.1 (from PROJECT.md), deployed as `deployment/headlamp` in `kube-system`
**What users get:** Web-based Kubernetes dashboard accessible via port-forward
**Access:** `kubectl port-forward -n kube-system service/headlamp 8080:80` then http://localhost:8080
**Token:** kinder prints a long-lived token during `kinder create cluster` from secret `kinder-dashboard-token` in `kube-system`
**Manual token retrieval:** `kubectl get secret kinder-dashboard-token -n kube-system -o jsonpath='{.data.token}' | base64 -d`
**Config field:** `addons.dashboard: false` to disable

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hand-written search index (JSON) | Pagefind (auto-indexed at build time) | Pagefind release ~2022; Starlight added it at launch | Zero config; run `npm run build` and search works |
| Separate docs framework (Docusaurus, Gitbook) | Starlight (already installed) | This project: Phase 9 | No migration needed; all tools already available |
| `autogenerate` sidebar (alphabetical) | Explicit `sidebar` config | N/A — both supported always | Explicit gives correct reading order for tutorial-style docs |
| MDX for all pages | Plain `.md` for content-only pages | Astro 2+ MDX is opt-in | Simpler, lighter, correct for pages with no component imports |

**Deprecated/outdated:**
- `guides/` and `reference/` directory structure from Starlight's default scaffold: The `src/content/docs/guides/example.md` and `src/content/docs/reference/example.md` placeholder files from the default scaffold should be deleted. They have no content and show as stubs in the sidebar.

## Open Questions

1. **Binary distribution method for DOCS-01 (BLOCKER)**
   - What we know: The Makefile renames the binary to `kinder` via `KIND_BINARY_NAME?=kinder`. The Go module path is `sigs.k8s.io/kind`. Running `go install sigs.k8s.io/kind@latest` installs based on the `cmd/kind/main.go` package directory — Go installs the binary as `kind`, not `kinder`. The only confirmed way to get a binary named `kinder` is via `make install` (build from source) or by downloading a pre-built binary from GitHub Releases.
   - What's unclear: Whether there are published GitHub Releases for `patrykattc/kinder` with pre-built `kinder` binaries, and whether `go install` actually produces `kind` or `kinder`.
   - Recommendation: Before planning DOCS-01, verify: (a) check `https://github.com/patrykattc/kinder/releases` for published releases with binaries, and (b) test `go install sigs.k8s.io/kind@latest` and check binary name. If `go install` produces `kind`, document `make install` (build from source) as the primary install path and note GitHub Releases as an alternative. The planner should add a checkpoint task for the user to confirm the install command before DOCS-01 content is finalized.

2. **Correct `apiVersion` value for kinder's v1alpha4 schema**
   - What we know: `types.go` defines the `Cluster` struct with `TypeMeta`. The kind equivalent uses `kind.x-k8s.io/v1alpha4`. Kinder's yaml.go may register a different group.
   - What's unclear: Whether kinder uses `kind.x-k8s.io/v1alpha4`, `kinder.dev/v1alpha4`, or something else. PROJECT.md mentions "kinder.dev/v1alpha4" but this may be aspirational.
   - Recommendation: Read `pkg/apis/config/v1alpha4/yaml.go` before writing DOCS-03. That file will show the registered group/version string. The planner should add a "verify apiVersion" step to the DOCS-03 task.

3. **Whether to delete the placeholder `guides/example.md` and `reference/example.md` files**
   - What we know: The default Starlight scaffold created these files in Phase 9. They appear in the sidebar with stub content.
   - What's unclear: Whether the sidebar config in Phase 11 will hide them (if they're not listed) or whether they need explicit deletion.
   - Recommendation: Delete them in this phase. Once an explicit `sidebar` config is added, unlisted pages won't appear in navigation — but they'll still exist in the build and be indexed by Pagefind, polluting search results. Delete the placeholder files explicitly.

## Sources

### Primary (HIGH confidence)
- `/Users/patrykattc/work/git/kinder/pkg/apis/config/v1alpha4/types.go` — All addon struct fields, field names, YAML keys, and comments
- `/Users/patrykattc/work/git/kinder/pkg/apis/config/v1alpha4/default.go` — All default values (all addons default to true via `boolPtrTrue`)
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/actions/installmetallb/metallb.go` — MetalLB install logic, IP pool CRs, rootless warning
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/actions/installmetallb/subnet.go` — IP pool range algorithm (`.200-.250`)
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` — Envoy Gateway install, GatewayClass YAML, server-side apply
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/actions/installmetricsserver/metricsserver.go` — Metrics Server install
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go` — CoreDNS transforms (exact strings)
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/create/actions/installdashboard/dashboard.go` — Headlamp install, token secret name, port-forward command
- `/Users/patrykattc/work/git/kinder/kinder-site/package.json` — Astro 5.6.1, Starlight 0.37.6 (verified installed versions)
- `/Users/patrykattc/work/git/kinder/.planning/PROJECT.md` — Addon versions: MetalLB v0.15.3, Envoy Gateway v1.3.1, Metrics Server v0.8.1, Headlamp v0.40.1
- https://starlight.astro.build/reference/configuration/ — Sidebar config format, `autogenerate`, explicit items (verified 2026-03-01)
- https://starlight.astro.build/reference/frontmatter/ — Frontmatter fields: title, description, sidebar, pagefind (verified 2026-03-01)
- https://starlight.astro.build/guides/authoring-content/ — Asides syntax, code blocks, Expressive Code (verified 2026-03-01)

### Secondary (MEDIUM confidence)
- https://starlight.astro.build/guides/site-search/ — Pagefind behavior: indexes at build time, `pagefind: false` frontmatter, `data-pagefind-ignore` attribute (verified via WebFetch 2026-03-01)
- `/Users/patrykattc/work/git/kinder/Makefile` — `KIND_BINARY_NAME?=kinder`, binary naming via Makefile variable (verified directly)
- `/Users/patrykattc/work/git/kinder/.go-version` — Go 1.25.7 compiler version (verified directly)

### Tertiary (LOW confidence)
- Phase 11 apiVersion value "kinder.dev/v1alpha4" from PROJECT.md — may be aspirational; actual value requires reading `yaml.go` (flagged as Open Question #2)
- `go install sigs.k8s.io/kind@latest` behavior for binary naming — unverified; flagged as Open Question #1 (blocker)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — Starlight 0.37.6 installed; all APIs verified against official docs fetched 2026-03-01
- Architecture (file structure, sidebar config): HIGH — Starlight docs confirmed, pattern derived from installed version
- Content accuracy (addon details): HIGH — Read directly from Go source code; these are facts, not assumptions
- Pitfalls: HIGH — Derived from actual source code analysis (rootless Podman warning from metallb.go, server-side apply from envoygw.go, token from dashboard.go)
- Distribution method for DOCS-01: LOW — Unverified; known blocker; flagged as Open Question #1

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (Starlight pre-1.0; content is stable Go source — no expiry on content accuracy)
