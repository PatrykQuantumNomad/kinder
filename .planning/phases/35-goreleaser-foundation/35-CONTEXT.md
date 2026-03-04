# Phase 35: GoReleaser Foundation - Context

**Gathered:** 2026-03-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Replace cross.sh and softprops/action-gh-release with GoReleaser to produce pre-built kinder binaries for all platforms, published as GitHub Releases with an automated pipeline. Users can download platform-specific archives from the Releases page.

</domain>

<decisions>
## Implementation Decisions

### Release artifacts
- Archive naming: `kinder_VERSION_OS_ARCH` format (e.g., `kinder_2.0.0_linux_amd64.tar.gz`) — version in filename
- Archive contents: just the binary — no LICENSE, README, or extras bundled
- Windows archives use `.zip`; Linux and macOS use `.tar.gz`
- GitHub Release body includes install snippet (curl one-liner) for quick setup

### Version display
- Match kind's existing `kind version` output format for consistency
- Dev/snapshot builds should be clearly distinguishable from tagged releases

### Changelog style
- GoReleaser auto-generates changelog categorized by commit prefix (feat:, fix:, docs:, etc.)
- Commits grouped into sections like Features, Bug Fixes, Documentation, Other

### Cross.sh retirement
- Remove cross.sh in the same commit that adds GoReleaser — atomic, clean cut
- GoReleaser handles both building AND publishing — full replacement of cross.sh + softprops/action-gh-release
- Claude should check codebase for any other cross.sh references beyond the release workflow

### Claude's Discretion
- Exact changelog categories and whether to include author handles
- Whether pre-release/RC tags get draft releases
- Dev/snapshot version string format (GoReleaser snapshot defaults are fine)
- Whether `kinder version` also shows node image version (check what kind does)
- Whether `kinder version --json` is added (check existing CLI patterns from v1.4)
- Local developer build workflow (Makefile integration vs plain go build)

</decisions>

<specifics>
## Specific Ideas

- Version output should feel familiar to kind users — match kind's format
- Release page should be self-service: a user landing on the GitHub Releases page should be able to install kinder without visiting docs
- Atomic retirement of cross.sh — no transition period, no fallback

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 35-goreleaser-foundation*
*Context gathered: 2026-03-04*
