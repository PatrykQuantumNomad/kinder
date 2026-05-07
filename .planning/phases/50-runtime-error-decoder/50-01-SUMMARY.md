---
phase: 50-runtime-error-decoder
plan: 01
subsystem: doctor
tags: [go, pattern-matching, regexp, sync.Map, tdd, diagnostics]

requires:
  - phase: 49-kinder-dev
    provides: "Established TDD RED/GREEN commit cadence and zero-new-dep streak"
  - phase: 47-pause-resume
    provides: "SafeMitigation struct in pkg/internal/doctor/mitigations.go (AutoFix field type)"

provides:
  - "DecodeScope type with 5 const values (ScopeKubelet/Kubeadm/Containerd/Docker/Addon)"
  - "DecodePattern struct (8 fields: ID, Scope, Match, Explanation, Fix, DocLink, AutoFixable, AutoFix)"
  - "DecodeMatch struct (3 fields: Source, Line, Pattern)"
  - "DecodeResult struct (3 fields: Cluster, Matches, Unmatched)"
  - "matchLines() pure function (first-match-wins per line, no I/O)"
  - "Catalog var with 16 patterns across all 5 scopes (SC2 compliant)"

affects:
  - "50-02: RunDecode orchestrator — imports DecodePattern/DecodeMatch/DecodeResult/matchLines/Catalog"
  - "50-03: CLI wiring — imports all exported types for render"
  - "50-04: Auto-fix — sets AutoFixable=true and AutoFix=&SafeMitigation{} on KUB-01, KUB-02, KADM-02, KUB-05"

tech-stack:
  added: []
  patterns:
    - "sync.Map regex cache: unique regex strings compiled once, stored by pattern string key"
    - "matchesPattern dispatch: strings.Contains for plain Match, regexp.MustCompile for 'regex:' prefix"
    - "first-match-wins per line: inner loop breaks after first hit; prevents duplicate output (RESEARCH pitfall 3)"
    - "Non-nil empty slice return: matchLines returns []DecodeMatch{} not nil for range-safe callers"

key-files:
  created:
    - pkg/internal/doctor/decode.go
    - pkg/internal/doctor/decode_catalog.go
    - pkg/internal/doctor/decode_test.go
    - pkg/internal/doctor/decode_catalog_test.go
  modified: []

key-decisions:
  - "KUB-05 uses regex form ('regex:error adding pid \\d+ to cgroups') — matches digit-containing lines like 'error adding pid 1234 to cgroups'"
  - "KADM-02 uses single substring 'coredns' — research two-string combo (coredns+Pending) would require custom multi-match logic; simpler substring with explanation note works for v1"
  - "DOCK-02 uses 'docker.sock' as the single Match substring — more specific than 'permission denied' alone, sufficient to identify the Docker socket access pattern"
  - "AutoFixable=false and AutoFix=nil for all 16 entries — Plan 50-04 populates KUB-01, KUB-02, KADM-02, KUB-05 without touching decode.go struct definition"
  - "decode.go imports only strings, regexp, sync — zero project-internal imports; no lifecycle/cluster cycle risk"

patterns-established:
  - "Pattern: sync.Map regex cache keyed by pattern string — each unique regex compiles once, -race clean"
  - "Pattern: Test-local pattern slices in decode_test.go — decouples Task 1 tests from global Catalog churn"

duration: ~15min
completed: 2026-05-07
---

# Phase 50 Plan 01: Pattern Catalog + Matcher Engine Summary

**Pure matchLines engine with sync.Map regex cache and 16-entry seed Catalog spanning all 5 DecodeScope values, satisfying DIAG-02/SC2 and DIAG-03/SC3**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-05-07T10:20:00Z
- **Completed:** 2026-05-07T10:31:19Z
- **Tasks:** 2 (TDD: 4 commits — 2 RED + 2 GREEN)
- **Files modified:** 4

## Accomplishments

- Implemented `matchLines()` pure function with first-match-wins semantics and sync.Map-cached regex compilation for -race cleanliness
- Seeded `Catalog` with 16 HIGH-confidence entries: 5 kubelet, 3 kubeadm, 3 containerd, 3 docker, 2 addon — all five DecodeScope values covered (SC2 satisfied)
- Pre-declared `AutoFixable bool` and `AutoFix *SafeMitigation` fields as zero/nil on `DecodePattern` — Plan 50-04 can populate without touching the struct definition
- All 11 tests pass under -race; zero new module deps; no doctor→lifecycle import cycle

## Task Commits

Each task was committed atomically via TDD RED then GREEN:

1. **Task 1 RED: DecodePattern + matchLines tests** - `e30d2bcb` (test)
2. **Task 1 GREEN: DecodePattern + matchLines engine** - `08596ed8` (feat)
3. **Task 2 RED: Catalog tests** - `cca55ed9` (test)
4. **Task 2 GREEN: 16-entry Catalog** - `42ae6b4a` (feat)

**Plan metadata:** (docs commit follows)

_TDD gate compliance: RED commit e30d2bcb precedes GREEN 08596ed8; RED cca55ed9 precedes GREEN 42ae6b4a_

## Files Created/Modified

- `pkg/internal/doctor/decode.go` - DecodeScope (5 consts), DecodePattern (8 fields), DecodeMatch (3 fields), DecodeResult (3 fields), matchLines(), matchesPattern() with sync.Map regex cache
- `pkg/internal/doctor/decode_catalog.go` - var Catalog = []DecodePattern{...} with 16 entries (KUB-01..05, KADM-01..03, CTD-01..03, DOCK-01..03, ADDON-01..02)
- `pkg/internal/doctor/decode_test.go` - 6 TestMatchLines* tests (substring, no-match, regex, first-match-wins, field-preservation, empty-inputs)
- `pkg/internal/doctor/decode_catalog_test.go` - 5 TestCatalog* tests (count, all-scopes, fields-populated, IDs-unique, matches-known-lines)

## Catalog — IDs Shipped in v1

| Scope | IDs | Count |
|-------|-----|-------|
| ScopeKubelet | KUB-01, KUB-02, KUB-03, KUB-04, KUB-05 | 5 |
| ScopeKubeadm | KADM-01, KADM-02, KADM-03 | 3 |
| ScopeContainerd | CTD-01, CTD-02, CTD-03 | 3 |
| ScopeDocker | DOCK-01, DOCK-02, DOCK-03 | 3 |
| ScopeAddon | ADDON-01, ADDON-02 | 2 |
| **Total** | | **16** |

Auto-fix targets (Plan 50-04): KUB-01 (inotify-raise), KUB-02 (inotify-raise), KADM-02 (coredns-restart), KUB-05 (node-container-restart).

## Decisions Made

- KADM-02 uses single substring `"coredns"` — the research two-string combo (coredns+Pending) would require custom multi-match logic outside matchLines' design; simpler substring with explanation note is correct for v1
- DOCK-02 uses `"docker.sock"` — more specific than `"permission denied"` alone
- All 16 AutoFixable=false, AutoFix=nil — struct shape frozen here; Plan 50-04 only edits decode_catalog.go

## Deviations from Plan

None — plan executed exactly as written. All 4 TDD gate commits land in order (test → feat × 2).

## Issues Encountered

Pre-existing race condition in `TestSubnetClashCheck_Run` (not introduced by this plan). Confirmed by running test suite before adding any files; noted as out-of-scope per deviation rule boundary.

## Verification Checklist

- [x] `go test -race ./pkg/internal/doctor/... -run "TestMatchLines|TestCatalog" -count=1` → PASS (11 tests)
- [x] `go vet ./pkg/internal/doctor/...` → clean
- [x] `grep -c "DecodeScope" decode.go` → 8 (>= 6 required: 1 type def + 5 consts + 2 field refs)
- [x] 16 catalog entries (>= 15 SC2 floor)
- [x] `go build ./...` → PASS (no import cycle)
- [x] `decode.go` imports only `regexp`, `strings`, `sync` — no lifecycle/cluster/cmd imports
- [x] `go.mod` and `go.sum` unchanged (zero new deps)
- [x] AutoFixable bool + AutoFix *SafeMitigation declared, zero/nil for all entries

## Next Phase Readiness

- Plan 50-02 can import `DecodePattern`, `DecodeMatch`, `DecodeResult`, `matchLines`, and `Catalog` directly — stable contract
- Plan 50-04 needs only to edit `decode_catalog.go` to set AutoFixable=true and AutoFix=&SafeMitigation{} for 4 entries — no decode.go changes
- TDD gate compliance confirmed: RED commits precede GREEN commits for both tasks

---
*Phase: 50-runtime-error-decoder*
*Completed: 2026-05-07*
