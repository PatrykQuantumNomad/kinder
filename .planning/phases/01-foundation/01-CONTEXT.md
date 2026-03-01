# Phase 1: Foundation - Context

**Gathered:** 2026-03-01
**Status:** Ready for planning

<domain>
## Phase Boundary

Config schema extensions and action pipeline scaffolding enabling addon opt-out. The `kinder` binary coexists with `kind`, extends the v1alpha4 config with an `addons` section, and provides the hook points that addon phases (2-6) will plug into. No addons are actually installed in this phase — only the infrastructure to enable/disable and invoke them.

</domain>

<decisions>
## Implementation Decisions

### Addon defaults & opt-out model
- All addons enabled by default — zero-config "batteries-included" experience
- Disabling an addon prints a one-line note: e.g. "Skipping MetalLB (disabled in config)"
- Dependency conflicts (e.g. MetalLB disabled + Envoy Gateway enabled) produce a warning and continue — install the dependent addon anyway but warn that it won't fully work
- Config supports booleans only for v1 — no per-addon nested settings yet

### Config schema feel
- Addons section uses flat boolean map: `addons.metalLB: true`
- Field names follow camelCase to match kind's existing v1alpha4 conventions
- Unrecognized addon names in config produce a strict error listing valid addon names — catches typos early
- Stay at v1alpha4 — the addons section is purely additive, no version bump needed

### CLI output during creation
- Match kind's existing output style (emoji + step lines) for addon installation
- Wait for pods to be ready and report health status before the command returns
- If an addon fails to install, warn and continue — cluster is usable, just missing that addon
- Print an addon summary block at the end listing installed addons and their status

### Platform warnings
- macOS/Windows MetalLB warning appears after MetalLB installs, alongside its success status
- Tone is factual + actionable: state the limitation and provide a workaround (e.g. `kubectl port-forward`)
- Warning appears every time — no suppression mechanism, no hidden state
- Visual style matches kind's existing warning/notice formatting

### Claude's Discretion
- Exact wording and formatting of warning messages
- Internal action pipeline architecture and hook mechanism
- Error message formatting details
- How addon readiness checks are implemented (polling interval, timeout)

</decisions>

<specifics>
## Specific Ideas

- The output should feel like a natural extension of kind's existing CLI — a developer familiar with kind shouldn't notice the seams
- Addon summary at the end should be scannable at a glance — which addons are installed, are they healthy, and any key info (like dashboard token)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-03-01*
