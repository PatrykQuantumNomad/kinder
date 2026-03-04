# Phase 34: Verification & Polish - Research

**Researched:** 2026-03-04
**Domain:** Static site audit, link verification, content accuracy, production build validation
**Confidence:** HIGH

## Summary

Phase 34 is an audit-and-fix pass over the completed kinder documentation site. The site is built with Astro 5.18.0 + Starlight 0.37.6 and contains 19 pages covering installation, quick-start, configuration, 7 addon pages, 3 guides, 3 CLI reference pages, and a changelog. All pages were added across phases 30-33 of the v1.5 milestone.

The production build (`npm run build`) already runs clean with zero errors and 19 pages built successfully. The primary verification work is: (1) confirming all sidebar slugs have corresponding built HTML files, (2) checking all internal markdown links resolve to existing pages, (3) validating that CLI commands in tutorials match the Go source code, and (4) fixing identified content inconsistencies. One content bug is already confirmed: `changelog.md` line 49 says `ci — MetalLB + Metrics Server` but the source code defines `ci` as `MetricsServer + CertManager` (no MetalLB).

The audit approach for this phase is entirely grep-and-read based — no external link checker tool is needed because the site has a small, known page set and all links are either absolute paths (verified by Astro build) or internal markdown relative links (which Astro resolves at build time). The build passing with zero errors or warnings already eliminates broken internal links at the Astro level. The residual work is prose accuracy: do command examples, flag names, output samples, and cross-references match what the CLI actually does.

**Primary recommendation:** Run a systematic grep-then-fix pass: check each page for CLI command accuracy against Go source, confirm sidebar covers all 19 pages, fix the confirmed `ci` profile bug in changelog.md, scan for stale placeholder text, then run `npm run build` as the final gate.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Astro | 5.18.0 | Static site generator | Already in use; produces 19-page HTML build |
| @astrojs/starlight | 0.37.6 | Documentation framework | Already in use; handles sidebar, search, MDX |
| Node.js (npm) | project version | Build runner | `npm run build` is the production build gate |

### Supporting
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| grep / Bash | system | Link and content pattern scanning | Scanning for placeholder text, URL patterns, command mismatches |
| `npm run build` | — | Production build verification | Final gate; Astro reports broken slug references |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| grep-based link check | `broken-link-checker` npm package | Package adds setup overhead; grep is sufficient for a 19-page site with known internal link patterns |
| Manual review | `lychee` or `htmlproofer` | External link checkers require network access and add flakiness; no external links need verification for this phase |

**No new installation required.** All tooling is already present.

## Architecture Patterns

### Recommended Audit Structure

The plan should follow this sequential order to avoid rework:

```
1. Sidebar completeness check   — astro.config.mjs slugs vs dist/ HTML
2. Internal link resolution     — grep markdown links, confirm targets built
3. CLI command accuracy         — grep docs vs Go source (cmd/, pkg/)
4. Content consistency          — changelog vs profile-comparison vs source
5. Placeholder scan             — grep for TODO/FIXME/Coming soon patterns
6. Production build gate        — npm run build, zero errors, 19 pages
```

### Pattern 1: Sidebar Slug Verification

**What:** Every slug in `astro.config.mjs` sidebar must have a corresponding `.md`/`.mdx` source file and appear in the build output.

**Verified state:** All 19 sidebar slugs confirmed present in build output:
- `/` (index.mdx)
- `/installation/`, `/quick-start/`, `/configuration/`, `/changelog/`
- `/addons/metallb/`, `/addons/envoy-gateway/`, `/addons/metrics-server/`, `/addons/coredns/`, `/addons/headlamp/`, `/addons/local-registry/`, `/addons/cert-manager/`
- `/guides/tls-web-app/`, `/guides/hpa-auto-scaling/`, `/guides/local-dev-workflow/`
- `/cli-reference/profile-comparison/`, `/cli-reference/json-output/`, `/cli-reference/troubleshooting/`

**Verification command:**
```bash
cd kinder-site && npm run build 2>&1 | grep "├─\|└─"
# Must list exactly 19 pages, no slug errors
```

### Pattern 2: Internal Link Scan

**What:** All markdown links `[text](/path)` in content files must resolve to pages that exist in the build.

**Known internal links (already verified in research):**

| File | Link | Target exists |
|------|------|---------------|
| index.mdx | `/addons/metallb/`, `/addons/metrics-server/`, `/addons/coredns/`, `/addons/envoy-gateway/`, `/addons/headlamp/`, `/addons/local-registry/`, `/addons/cert-manager/` | Yes |
| addons/envoy-gateway.md | `/configuration` | Yes |
| addons/headlamp.md | `/configuration` | Yes |
| addons/cert-manager.md | `/configuration` | Yes |
| addons/local-registry.md | `/configuration` | Yes |
| addons/coredns.md | `/configuration` | Yes |
| addons/metrics-server.md | `/configuration` | Yes |
| addons/metallb.md | `/configuration` | Yes |
| guides/hpa-auto-scaling.md | `/installation` | Yes |
| guides/tls-web-app.md | `/installation` | Yes |
| guides/local-dev-workflow.md | `/installation` | Yes |
| cli-reference/profile-comparison.md | `/configuration` (x2) | Yes |

**Scan command:**
```bash
grep -rn "\[.*\]([^h]" kinder-site/src/content/docs/
# All results must point to /installation, /configuration, or /addons/* pages
```

### Pattern 3: CLI Accuracy Check

**What:** CLI commands, flags, and output fields in docs must match Go source.

**Confirmed accurate (verified against source):**

| Claim | Source Location | Status |
|-------|----------------|--------|
| `--profile` values: `minimal`, `full`, `gateway`, `ci` | `pkg/cluster/createoption.go:132-158` | Correct |
| `ci` profile: MetricsServer + CertManager | `pkg/cluster/createoption.go:155` | Correct in profile-comparison.md and quick-start.md |
| `gateway` profile: MetalLB + EnvoyGateway | `pkg/cluster/createoption.go:153-154` | Correct |
| `kinder env` JSON fields: `kinderProvider`, `clusterName`, `kubeconfig` | `pkg/cmd/kind/env/env.go:76-84` | Correct |
| `kinder env` text output: `KINDER_PROVIDER`, `KIND_CLUSTER_NAME`, `KUBECONFIG` | `pkg/cmd/kind/env/env.go:88-90` | Correct |
| Supported `--output` values for `env`: `""` and `"json"` | `pkg/cmd/kind/env/env.go:68-73` | Correct |
| Default node image: `kindest/node:v1.35.1` | `pkg/apis/config/defaults/image.go:21` | Correct in quick-start.md |

**Confirmed inaccurate (must fix):**

| File | Line | Bug | Correct value | Source |
|------|------|-----|---------------|--------|
| `changelog.md` | 49 | `ci — MetalLB + Metrics Server` | `ci — Metrics Server + cert-manager` | `pkg/cluster/createoption.go:155` |

### Pattern 4: Content Consistency Cross-Check

**What:** Same feature described identically in all places.

**`ci` profile — inconsistency found:**
- `changelog.md` line 49: `ci — MetalLB + Metrics Server` — WRONG
- `quick-start.md` line 55: `--profile ci — Metrics Server + cert-manager` — correct
- `cli-reference/profile-comparison.md` table: Metrics Server + cert-manager checked — correct
- `pkg/cluster/createoption.go:155`: `MetricsServer: true, CertManager: true` — source of truth

Fix: Change changelog.md line 49 to read: `ci — Metrics Server + cert-manager (CI-optimized)`

### Pattern 5: Astro Build as Link Checker

**What:** Astro's static build is the authoritative link checker for this site. Any slug referenced in the sidebar that doesn't have a corresponding content file produces a build error. Any content collection file with bad frontmatter fails build.

**Starlight slug behavior:** Starlight derives page URLs from the file path under `src/content/docs/`. The sidebar uses `{ slug: 'path/to/page' }` which must match `src/content/docs/path/to/page.md`. A mismatch produces a build-time error — verified by running `npm run build`.

**Current build status:** Zero errors, 19 pages, complete.

### Anti-Patterns to Avoid

- **Running external link checkers:** External URLs (GitHub, kubernetes.io, metallb.universe.tf) do not need verification for this phase. The phase success criteria only covers internal links.
- **Checking for broken anchors in HTML:** Starlight generates heading anchors automatically and doesn't reference them in content. No anchor link verification needed.
- **Modifying sidebar order:** The sidebar structure was established in phase 30 and verified in phase 33. Don't reorder without specific reason.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Internal link verification | Custom HTML parser | `npm run build` | Astro's build already validates all slug references at build time |
| Checking all 19 pages render | Manual browser test | `npm run build` output list | Build output lists every page; zero-error build = all pages rendered |
| CLI flag accuracy | Re-implementing CLI | Direct grep of Go source files | Source code is authoritative; 2-step grep finds truth in seconds |

**Key insight:** For a 19-page static site with a known, small internal link set, `npm run build` + targeted grep is sufficient and faster than any external tooling.

## Common Pitfalls

### Pitfall 1: Trusting the Previous Build as Final
**What goes wrong:** Using the cached `dist/` from the last build rather than running a fresh `npm run build` after all fixes.
**Why it happens:** Build output persists between runs.
**How to avoid:** Run `npm run build` as the final step, after all content edits. Confirm the line count still reads `19 page(s) built` with zero errors.
**Warning signs:** Build output shows fewer pages or any `[warn]` lines.

### Pitfall 2: Missing the Changelog Inconsistency
**What goes wrong:** Fixing content pages but missing the `ci` profile bug in changelog.md because the changelog is rarely read during doc audits.
**Why it happens:** changelog.md is treated as historical; audits focus on current docs pages.
**How to avoid:** Explicitly include changelog.md in the grep pass for CLI flag descriptions.
**Warning signs:** `grep "ci —\|ci -" changelog.md` returns `MetalLB + Metrics Server`.

### Pitfall 3: Treating Expected-Output Blocks as Exact CLI Contract
**What goes wrong:** Flagging `tls-web-app.md` Expected output showing `kindest/node:v1.32.0` as a broken contract (current default is v1.35.1).
**Why it happens:** Expected output samples in tutorials are illustrative, not live-scraped from the binary.
**How to avoid:** Distinguish between: (a) command syntax and flag names — must be exact; (b) version numbers in expected output — illustrative only; (c) JSON field names — must be exact.
**Warning signs:** Attempting to update every version number in every Expected output block.

### Pitfall 4: Confusing Astro Build Errors with Link 404s
**What goes wrong:** Thinking `npm run build` passes = no 404s at runtime, but runtime 404s can still occur from hand-typed URLs.
**Why it happens:** Astro can't test runtime navigation from a user's browser.
**How to avoid:** Manually check that every sidebar item is reachable via the sidebar nav (sidebar wiring check). Since the build already succeeded with all slugs, this is a secondary check.
**Warning signs:** A sidebar slug exists in `astro.config.mjs` but the corresponding source file was deleted.

## Code Examples

Verified patterns from research:

### Running the Production Build
```bash
# From kinder-site/ directory
cd /path/to/kinder/kinder-site
npm run build
# Expected: "19 page(s) built" with "[build] Complete!" and no errors
```

### Grep for All Internal Markdown Links
```bash
# From repo root
grep -rn "\[.*\]([^h]" kinder-site/src/content/docs/
# All results should point to /installation, /configuration, /addons/*, /guides/*, /cli-reference/*
# No results should point to a path that lacks a corresponding .md source file
```

### Verify cli Profile in Source
```bash
grep -A 5 "case \"ci\":" kinder-site/../pkg/cluster/createoption.go
# Expected: MetricsServer: true, CertManager: true (NO MetalLB)
```

### Grep for Placeholder Content
```bash
grep -rni "coming soon\|placeholder\|TODO\|FIXME\|stub\|lorem ipsum" kinder-site/src/content/docs/
# Expected: zero results
```

### Fix: changelog.md ci Profile Description
The line at `changelog.md:49` reads:
```
  - `ci` — MetalLB + Metrics Server (CI-optimized)
```
Must be changed to:
```
  - `ci` — Metrics Server + cert-manager (CI-optimized)
```

## State of the Art

| Old State | Current State | When Established | Impact |
|-----------|---------------|-----------------|--------|
| 16 pages (pre-v1.5) | 19 pages (guides + CLI ref added) | Phases 32-33 | Three new guide pages and three CLI reference pages require audit |
| No guides section | Guides section with 3 tutorials | Phase 33 | Tutorial CLI commands need accuracy check against source |
| No CLI reference | CLI reference with 3 pages | Phase 32 | Profile/JSON output docs need cross-check with source |

**Known content state at phase 34 start:**
- `npm run build` passes with zero errors
- 19 pages built
- No placeholder text detected (grep confirmed zero results)
- One confirmed content bug: `changelog.md` line 49 `ci` profile description

## Open Questions

1. **Installation page Go version claim**
   - What we know: `installation.md` says "Go 1.25.7 or later"; `go.mod` requires `go 1.24.0`; `.go-version` says `1.25.7`; `go.mod` toolchain directive says `go1.26.0`
   - What's unclear: Is "Go 1.25.7 or later" the right user-facing message? The compiler toolchain (1.26.0) and .go-version (1.25.7) differ from the language minimum (1.24.0).
   - Recommendation: The .go-version file (1.25.7) is what `make install` will use if the user has that exact version. "Go 1.24 or later" is technically correct per go.mod. Safest fix is to say "Go 1.24 or later" matching the go.mod minimum requirement, since that's what actually gates compilation. Flag for the planner to make a decision, or keep "1.25.7" as a conservative user-friendly recommendation.

2. **Expected output version numbers in tutorials**
   - What we know: `tls-web-app.md` shows `kindest/node:v1.32.0` in expected output; current default is `v1.35.1`
   - What's unclear: Whether this counts as a "broken" expected output or is acceptable illustrative content
   - Recommendation: Do not update expected output version numbers in tutorials — they are illustrative samples, not live-scraped values. The tutorials explicitly use `kinder create cluster` without pinning a version, so whatever the current default is will be used at runtime. Version numbers in Expected output blocks are cosmetic.

## Validation Architecture

No test framework is applicable for a content audit phase. The verification mechanism is:

- **Per-task gate:** `npm run build` from `kinder-site/` — zero errors, 19 pages
- **Phase gate:** All four success criteria verified before closing the phase

### Phase Requirements Mapping

| Success Criterion | Verification Method |
|-------------------|---------------------|
| All internal links resolve without 404s | `npm run build` passes (Astro validates all slug references) + grep confirms no broken relative links in markdown |
| `npm run build` zero errors, zero broken link warnings | Direct: run `npm run build` in `kinder-site/` |
| All pages reachable from sidebar | Sidebar slug count in `astro.config.mjs` matches 19 built pages |
| Code blocks syntactically correct, commands match CLI | Grep Go source for flag definitions, compare to markdown command examples |

## Sources

### Primary (HIGH confidence)
- `kinder-site/astro.config.mjs` — sidebar slug definitions (all 19 slugs confirmed)
- `kinder-site/package.json` — Astro 5.18.0, Starlight 0.37.6 (installed, build confirmed)
- `pkg/cluster/createoption.go` — profile values and addon mappings (Go source, authoritative)
- `pkg/cmd/kind/env/env.go` — `kinder env` JSON field names and text output format (Go source, authoritative)
- `pkg/apis/config/defaults/image.go` — default node image `kindest/node:v1.35.1`
- `npm run build` output — 19 pages, zero errors, complete (run live during research)

### Secondary (MEDIUM confidence)
- `go.mod` — Go language minimum version 1.24.0
- `.go-version` — Go compiler version used for builds: 1.25.7
- Phase 33 VERIFICATION.md — confirmed no placeholder text across guide files, build passes

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Build state: HIGH — ran `npm run build` during research, zero errors confirmed
- Internal links: HIGH — all markdown links enumerated and targets confirmed present in build
- CLI accuracy: HIGH — cross-referenced Go source for every CLI claim in research scope
- Identified bugs: HIGH — `ci` profile bug confirmed by direct source code inspection

**Research date:** 2026-03-04
**Valid until:** Stable — content and source don't change without a commit; this research is tied to the current git state
