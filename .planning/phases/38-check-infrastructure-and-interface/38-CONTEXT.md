# Phase 38: Check Infrastructure and Interface - Context

**Gathered:** 2026-03-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Unified Check interface for `kinder doctor` — category-grouped output, platform filtering, mitigation tier system (skeleton), and migration of 3 existing checks (container runtime, kubectl, NVIDIA GPU). New checks are added in Phases 39-40. Create-flow integration is Phase 41.

</domain>

<decisions>
## Implementation Decisions

### Doctor output style
- Bold section headers per category (e.g., `=== Runtime ===`) with checks listed below each
- Unicode status icons: ✓ ok, ✗ fail, ⚠ warn, ⊘ skip
- No ANSI colors — plain text only, works everywhere including piped output
- Summary line at the end: `13 checks: 9 ok, 1 warning, 0 failed, 3 skipped`

### Check result detail
- Passing checks show name + detected value: `✓ Container runtime detected (docker)`
- Warnings and failures show inline fix command: `→ sudo usermod -aG docker $USER`
- Warnings and failures include a one-liner explanation of WHY it matters before the fix
- JSON output uses structured fields: separate `message`, `reason`, and `fix` fields per check result (plus `name`, `category`, `status`)

### Platform skip behavior
- Skipped checks are visible in output with ⊘ icon and short platform tag: `⊘ inotify watches (linux only)`
- Categories with all skipped checks still show their header and skipped checks
- Skipped checks are included in the summary count as a separate "skipped" number
- Skip reasons use concise platform tags: `(linux only)`, `(requires nvidia-smi)` — not full sentences

### Mitigation messaging
- Two severity levels only: ⚠ warn (degraded but works) and ✗ fail (will break)
- Auto-fixable issues shown in a separate `=== Auto-mitigations ===` section after all check categories
- Auto-mitigations section only appears when there are fixable issues — no section when clean
- During `kinder create cluster`, print a brief line per applied mitigation: `Applied mitigation: override init=true`

### Claude's Discretion
- Exact category names and ordering for the three existing checks
- Section header formatting details (separator characters, spacing)
- JSON envelope structure (top-level fields beyond the checks array)

</decisions>

<specifics>
## Specific Ideas

- Output style reference: category headers like `=== Runtime ===`, checks indented below with 2-space indent
- Check detail format: status icon + name + value on first line, explanation on second line (indented), fix command on third line with arrow prefix (indented)
- Summary separator: `───` horizontal line before the count summary
- Auto-mitigations section uses bullet points: `• Docker init=true → override via config`

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 38-check-infrastructure-and-interface*
*Context gathered: 2026-03-06*
