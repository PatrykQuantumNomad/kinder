# Phase 30: Foundation Fixes - Context

**Gathered:** 2026-03-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Update existing website pages (landing, quick-start, configuration) to accurately reflect all v1.3-v1.4 features including all 7 addons. Add Guides and CLI Reference sidebar sections with placeholder entries for future phases. No new tutorial content — just structure and accuracy.

</domain>

<decisions>
## Implementation Decisions

### Addon presentation on landing page
- Group addons as **core** (always-on: MetalLB, Metrics Server, CoreDNS) vs **optional** (opt-in: Envoy Gateway, Headlamp, Local Registry, cert-manager)
- Order by importance/usage within each group, most impactful first
- Each addon shows: name + one-liner description + concrete benefit (e.g., "MetalLB — LoadBalancer IPs → access services via localhost")
- Keep existing kind vs kinder side-by-side comparison layout, just update the addon list to include all 7

### Quick-start verification depth
- Each addon gets a verification command + expected output so users can confirm success
- Group verification steps as core addons first, then optional — matching landing page grouping
- Default `kinder create` as the main path; introduce `--profile` flag in a tip/callout box (not up front)
- Add a "Something wrong?" section at the end pointing to `kinder doctor`

### Configuration page structure
- Group addon fields by core vs optional — consistent with landing page and quick-start
- Each addon documented with YAML snippet + field table (field name, type, default, description)
- Full complete v1alpha4 config example at the top for copy-paste, then per-addon breakdowns below
- Prominent callout showing which addons are enabled by default vs require explicit opt-in

### Sidebar organization
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

</decisions>

<specifics>
## Specific Ideas

- Consistent "core vs optional" grouping across all three pages (landing, quick-start, config) for a unified mental model
- Verification steps should show what the output actually looks like — not just "check if running"
- The `--profile` flag introduction should feel like a power-user tip, not a required decision point

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 30-foundation-fixes*
*Context gathered: 2026-03-04*
