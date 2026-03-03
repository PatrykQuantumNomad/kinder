# Phase 19: Bug Fixes - Context

**Gathered:** 2026-03-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix four correctness bugs (BUG-01 through BUG-04) across all three container runtime providers (docker, podman, nerdctl). No refactoring, no new features — pure bug fixes that must land before any v1.3 work begins.

</domain>

<decisions>
## Implementation Decisions

### Fix isolation
- One commit per bug — each fix independently testable and revertable
- Each bug gets its own unit test(s) proving the fix
- Fix order: tackle them in BUG-01 → BUG-04 sequence (port leak, tar truncation, empty cluster name, network sort)

### BUG-01: Port listener leak
- Port listeners acquired in `generatePortMappings` must be released at iteration end, not at function return
- Fix must handle early exit paths — no port leak under any code path
- Verify fix across all three providers since `generatePortMappings` exists in each

### BUG-02: Silent tar truncation
- Tar extraction must return an error on truncated files instead of silently stopping mid-archive
- Fail explicitly — no silent data loss
- Error message should indicate truncation specifically (not a generic I/O error)

### BUG-03: Empty cluster name resolution
- `ListInternalNodes` must resolve empty cluster name to the default cluster name consistently across all three providers
- Verify the default name resolution matches what `kind` itself uses
- All three providers must behave identically for this case

### BUG-04: Network sort comparator
- Network sort comparator must satisfy strict weak ordering — identical networks compare equal
- Sort must be deterministic across runs
- Fix the comparison logic, don't work around it with stable sort

### Claude's Discretion
- Exact test structure (table-driven vs individual test functions)
- Whether to add helper functions for test setup
- Specific error message wording for BUG-02

</decisions>

<specifics>
## Specific Ideas

No specific requirements — bugs are well-defined by success criteria in the roadmap. Standard Go testing patterns apply.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 19-bug-fixes*
*Context gathered: 2026-03-03*
