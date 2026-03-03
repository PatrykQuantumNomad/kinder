# Phase 25: Foundation - Context

**Gathered:** 2026-03-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Clean up the codebase baseline for v1.4. Bump go.mod to 1.23, update dependencies, migrate to golangci-lint v2, fix the layer violation (version package), and apply code quality fixes (SHA-256, permissions, log levels, naming, dead code). All changes are internal — no user-facing behavior changes except the version package relocation which requires linker flag updates.

</domain>

<decisions>
## Implementation Decisions

### Go version and dependencies
- Bump go.mod minimum from 1.17/1.21 to 1.23
- Update golang.org/x/sys from v0.6.0 to v0.41.0
- Remove rand.NewSource dead code that was comment-gated for pre-1.23

### golangci-lint v2 migration
- Upgrade from v1.62.2 to v2.10.1
- Migrate existing config to v2 format
- Fix all violations so `golangci-lint run ./...` passes clean

### Layer violation fix
- Move version package from `pkg/cmd/kind/version/` to `pkg/internal/kindversion/`
- Update all import paths
- Update linker -X flags to reference new package path
- Verify `kinder version` prints correct string after move

### Code quality fixes
- SHA-1 → SHA-256 for subnet generation in all three providers
- os.ModePerm (0777) → 0755 for log directory permissions
- Dashboard token log level V(0) → V(1)
- Rename NoNodeProviderDetectedError → ErrNoNodeProviderDetected
- Remove redundant contains helper from subnet_test.go

### Claude's Discretion
- golangci-lint v2 config strictness — carry over existing rules or tighten
- Change sequencing and plan granularity
- Whether to fix similar naming patterns beyond FOUND-09 if encountered
- How to handle any cascading lint violations from v2 migration
- Backward compatibility for subnet hash changes (SHA-256 produces different subnets)

</decisions>

<specifics>
## Specific Ideas

No specific requirements — the 10 FOUND-xx requirements fully specify the changes. Implementation follows standard Go project practices.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 25-foundation*
*Context gathered: 2026-03-03*
