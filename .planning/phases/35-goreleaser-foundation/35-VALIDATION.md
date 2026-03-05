# Phase 35: GoReleaser Foundation - Validation

**Created:** 2026-03-04
**Source:** 35-RESEARCH.md Validation Architecture section

## Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` package (`go test ./...`) |
| Config file | none (standard Go test runner) |
| Quick run command | `go test ./pkg/internal/kindversion/... -v` |
| Full suite command | `go test ./... -count=1` |

## Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REL-01 | GoReleaser produces 5 platform binaries | smoke | `goreleaser build --snapshot --clean && ls dist/` | Wave 1 (Makefile target) |
| REL-02 | SHA-256 checksums file generated | smoke | `goreleaser build --snapshot --clean && test -f dist/checksums.txt` | Wave 1 (Makefile target) |
| REL-03 | Changelog generated | smoke | `goreleaser release --skip=publish --snapshot --clean 2>&1 \| grep -i changelog` | Wave 1 (Makefile target) |
| REL-04 | `kinder version` shows real commit hash | unit + smoke | `go test ./pkg/internal/kindversion/... -v` (existing) + `goreleaser build --snapshot --clean && ./dist/kinder_darwin_arm64_v1/kinder version` | Existing test |
| REL-05 | release.yml uses goreleaser-action | manual | Review `.github/workflows/release.yml` content | File exists (modified in Wave 2) |
| REL-06 | `goreleaser check` passes zero errors | smoke | `goreleaser check` | Wave 1 (Makefile target) |

## Sampling Rate

- **Per task commit:** `go test ./pkg/internal/kindversion/... -v && goreleaser check`
- **Per wave merge:** `goreleaser build --snapshot --clean` (verifies all 5 platform binaries)
- **Phase gate:** `goreleaser check` exits 0 with zero deprecation warnings; `goreleaser build --snapshot --clean` produces 5 archives in `dist/`; native binary `kinder version` shows non-empty commit hash

## Wave 0 Gaps

None — `.goreleaser.yaml` and Makefile targets are created in Plan 35-01 (Wave 1), which is the first plan.

*(REL-04's unit test exists at `pkg/internal/kindversion/version_test.go` — no new test file needed)*
