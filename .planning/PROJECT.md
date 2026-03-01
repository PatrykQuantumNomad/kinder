# Kinder

## What This Is

Kinder is a fork of kind (Kubernetes IN Docker) that provides a batteries-included local Kubernetes development experience. Where kind gives you a bare cluster, kinder comes with LoadBalancer support (MetalLB), Gateway API ingress (Envoy Gateway), metrics (`kubectl top` / HPA), tuned DNS (CoreDNS autopath + cache), and a dashboard (Headlamp) — all working out of the box. Users run `kinder create cluster` and get a fully functional development environment with zero manual setup.

## Core Value

A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## Requirements

### Validated

- ✓ Binary renamed to `kinder`, coexists with `kind` — v1.0
- ✓ Config schema extends v1alpha4 with `addons` section — v1.0
- ✓ Existing kind configs work unchanged (backward compatible) — v1.0
- ✓ Each addon checks enable flag before executing — v1.0
- ✓ Platform warning for MetalLB on macOS/Windows — v1.0
- ✓ MetalLB installed by default with auto-detected subnet — v1.0
- ✓ Envoy Gateway installed by default with Gateway API CRDs — v1.0
- ✓ Metrics Server installed by default with `--kubelet-insecure-tls` — v1.0
- ✓ CoreDNS tuning applied by default (autopath, pods verified, cache 60) — v1.0
- ✓ Headlamp dashboard installed with printed token and port-forward command — v1.0
- ✓ Each addon individually disableable via cluster config — v1.0
- ✓ Addons wait for readiness before cluster is reported ready — v1.0

### Active

- [ ] Kinder website with landing page and documentation, built with Astro
- [ ] Modern dark developer-tool aesthetic (terminal-style, code snippets, badges)
- [ ] Landing page showcasing features, install instructions, and addon highlights
- [ ] Documentation pages: installation guide, configuration reference, addon docs
- [ ] Custom domain: kinder.patrykgolabek.dev via GitHub Pages
- [ ] Source lives in `kinder-site/` directory at repo root

## Current Milestone: v1.1 Kinder Website

**Goal:** Create a standalone website for kinder with a marketing landing page and full documentation, hosted on GitHub Pages at kinder.patrykgolabek.dev.

**Target features:**
- Astro-based site in `kinder-site/` with modern dark developer aesthetic
- Landing page with feature highlights, install quickstart, and addon showcase
- Documentation pages covering installation, cluster config, and each addon
- Custom domain configuration for GitHub Pages

### Out of Scope

- OAuth/OIDC integration for dashboard — too complex for v1; token-based auth is sufficient for local dev
- Multi-cluster networking — single cluster focus
- Helm as addon manager — avoid Helm dependency; use static manifests/go:embed
- Custom addon plugin system — hardcoded addons for v1, extensibility later
- Service mesh (Istio, Linkerd) — conflicts with Envoy Gateway; separate concern
- BGP mode for MetalLB — requires external router; impossible in kind without extra containers

## Context

- Kinder is a fork of sigs.k8s.io/kind at commit 89ff06bd
- Shipped v1.0 with ~1,950 LOC Go across 5 addon action packages
- Tech stack: Go, same build system as kind, go:embed for all addon manifests
- All addons applied at runtime via kubectl (not baked into node image)
- Addon versions pinned: MetalLB v0.15.3, Metrics Server v0.8.1, Envoy Gateway v1.3.1, Headlamp v0.40.1

## Constraints

- **Tech stack (core)**: Go, same build system as kind — no new languages or build tools
- **Tech stack (website)**: Astro framework, TypeScript/JS — standard static site tooling
- **Compatibility**: Must work with Docker, Podman, and Nerdctl (all existing providers)
- **Config format**: Extend kind's `v1alpha4` config API with addon fields, don't break existing configs
- **Image size**: Addon manifests applied at runtime, not baked into node image
- **Hosting**: GitHub Pages with custom domain kinder.patrykgolabek.dev

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Fork kind, don't wrap it | Full control over action pipeline, single binary | ✓ Good |
| Addons as creation actions | Follows existing kind pattern (installcni, installstorage) | ✓ Good |
| On by default, opt-out | Target audience wants batteries included; power users can disable | ✓ Good |
| Runtime manifest apply | Apply addon manifests via kubectl during creation | ✓ Good |
| *bool for v1alpha4 Addons | nil means not-set, defaults to true; avoids Go zero-value ambiguity | ✓ Good |
| go:embed for all manifests | Offline-capable, no external tools at cluster creation | ✓ Good |
| Headlamp over kubernetes/dashboard | kubernetes/dashboard archived Jan 2026, Helm dependency | ✓ Good |
| Server-side apply for Envoy Gateway | httproutes CRD exceeds 256 KB annotation limit | ✓ Good |
| CoreDNS read-modify-write in Go | Corefile is a string blob, not structured YAML | ✓ Good |
| Astro for website | Static site generator, fast, great for docs + landing pages | — Pending |
| Separate kinder-site/ directory | Clean separation from Go codebase, no reuse of kind's site | — Pending |
| No kind website code reuse | Fresh design for kinder identity, avoids inheriting kind's patterns | — Pending |

---
*Last updated: 2026-03-01 after v1.1 milestone start*
