---
phase: 50-runtime-error-decoder
plan: "04"
subsystem: doctor
tags: [go, doctor, decode, autofix, sysctl, coredns, kind]

requires:
  - phase: 50-01
    provides: DecodePattern/DecodeMatch/matchLines/Catalog (16-entry)
  - phase: 50-02
    provides: execCommand var, RunDecode orchestrator, DecodeOptions

provides:
  - InotifyRaiseMitigation() *SafeMitigation factory
  - CoreDNSRestartMitigation(binaryName, cpNodeName string) *SafeMitigation factory
  - NodeContainerRestartMitigation(binaryName, nodeName string) *SafeMitigation factory
  - DecodeAutoFixContext struct {BinaryName, CPNodeName string}
  - ApplyDecodeAutoFix(matches []DecodeMatch, ctx DecodeAutoFixContext, logger log.Logger) []error
  - PreviewDecodeAutoFix(matches []DecodeMatch, ctx DecodeAutoFixContext) []string
  - Catalog wired: KUB-01/02 AutoFixable=true+AutoFix=InotifyRaiseMitigation(); KADM-02/KUB-05 AutoFixable=true+AutoFix=nil

affects:
  - 50-03 (cobra command wires PreviewDecodeAutoFix + ApplyDecodeAutoFix via DecodeAutoFixContext)
  - 50-05 (integration / live UAT of --auto-fix flag)

tech-stack:
  added: []
  patterns:
    - "fn-var injection for auto-fix primitives (readSysctlFn, writeSysctlFn, getCoreDNSStatusFn, inspectStateAutoFn, execCmdFn, geteuidFn)"
    - "mitigationFor dispatch: AutoFix!=nil → use directly; AutoFix=nil → construct from Pattern.ID+ctx"
    - "dedup-by-Name across matches before Apply"

key-files:
  created:
    - pkg/internal/doctor/decode_autofix.go
    - pkg/internal/doctor/decode_autofix_test.go
  modified:
    - pkg/internal/doctor/decode_catalog.go
    - pkg/internal/doctor/decode_catalog_test.go

key-decisions:
  - "geteuidFn injected as fn-var for NeedsRoot test without requiring real OS privilege change"
  - "KUB-05 node name derived from match.Source 'docker-logs:<node>' prefix; k8s-events source returns nil (skip)"
  - "realInspectStateAuto intentionally duplicates resumereadiness.go's realInspectState to avoid doctor->lifecycle import cycle"
  - "execCommand in decode_autofix.go is a reference (not redeclaration) — owned by decode_collectors.go (50-02)"
  - "ApplyDecodeAutoFix dedupes by sm.Name so KUB-01+KUB-02 both embedding inotify-raise fires the mitigation exactly once"
  - "PreviewDecodeAutoFix MAY call NeedsFix for annotation but MUST NOT call Apply"

patterns-established:
  - "mitigationFor: AutoFix non-nil = use directly; AutoFix nil = construct from ID via ctx"
  - "Dedup loop: seen[sm.Name] before NeedsFix/NeedsRoot checks"

duration: ~4min
completed: 2026-05-07
---

# Phase 50 Plan 04: Auto-Fix Whitelist Summary

**Three whitelisted SafeMitigation factories (inotify-raise, coredns-restart, node-container-restart) with ApplyDecodeAutoFix+PreviewDecodeAutoFix orchestrator; Catalog wired with AutoFixable=true on KUB-01/02/KADM-02/KUB-05**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-05-07T10:44:31Z
- **Completed:** 2026-05-07T10:48:34Z
- **Tasks:** 2 (both TDD RED+GREEN)
- **Files modified:** 4

## Accomplishments

- Three SafeMitigation factories with idempotent NeedsFix preconditions and fn-var injectable Apply
- ApplyDecodeAutoFix orchestrator: dedup by Name, NeedsFix honor, NeedsRoot skip with warn log, error-but-continue collection
- PreviewDecodeAutoFix dry-run helper: side-effect free (Apply never called), annotates precondition/root skip
- Catalog wired: KUB-01/KUB-02 embed InotifyRaiseMitigation() pointer; KADM-02/KUB-05 set AutoFixable=true with nil AutoFix (constructed at runtime from DecodeAutoFixContext)
- All 20 new tests pass under -race; 0 new module deps; go.mod/go.sum unchanged

## Task Commits

Each task was committed atomically via TDD RED then GREEN:

1. **Task 1 RED: failing tests for auto-fix mitigations** - `4772c837` (test)
2. **Task 1 GREEN: three SafeMitigation factories** - `258f11ac` (feat)
3. **Task 2 RED: failing tests for orchestrator + catalog wiring** - `450e6641` (test)
4. **Task 2 GREEN: ApplyDecodeAutoFix + catalog AutoFix flags** - `f55236cb` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `pkg/internal/doctor/decode_autofix.go` — Three factory functions, mitigationFor dispatch, ApplyDecodeAutoFix, PreviewDecodeAutoFix, DecodeAutoFixContext; fn-var injection points; real impls for sysctl/coredns/container-state
- `pkg/internal/doctor/decode_autofix_test.go` — 20 unit tests covering all three factories, orchestrator dedup/NeedsFix/NeedsRoot/error-continue, preview side-effect-free check, catalog invariants
- `pkg/internal/doctor/decode_catalog.go` — KUB-01/KUB-02 AutoFixable=true+AutoFix=InotifyRaiseMitigation(); KADM-02/KUB-05 AutoFixable=true+AutoFix=nil; all other 12 entries unchanged
- `pkg/internal/doctor/decode_catalog_test.go` — TestCatalog_AutoFixableEntries (Test 20) added: verifies exact 4 AutoFixable=true entries and pointer/nil invariants

## Decisions Made

- `geteuidFn` injected as a fn-var (not threading os.Geteuid as parameter) — consistent with the project's t.Cleanup-swap pattern for OS-level primitives
- `realInspectStateAuto` intentionally duplicates `resumereadiness.go`'s `realInspectState` body — no doctor->lifecycle import; the cycle constraint (lifecycle imports doctor) is hard
- `execCommand` is referenced from `decode_collectors.go` (owned by Plan 50-02) without redeclaration — Plan 50-02 was confirmed on disk before this plan ran; no filesystem park-aside or parallel-wave coordination was needed
- KUB-05 node name extraction uses `"docker-logs:"` prefix convention — consistent with how RunDecode sets match.Source (`"docker-logs:" + nodeName`)
- ApplyDecodeAutoFix deduplication uses the seen-map-by-Name approach so KUB-01+KUB-02 both pointing to `inotify-raise` fire the mitigation exactly once when both patterns match in the same run

## Exported API (for Plan 50-03 wiring)

```go
// Orchestrator entry points — Plan 50-03 calls these directly.
func ApplyDecodeAutoFix(matches []DecodeMatch, ctx DecodeAutoFixContext, logger log.Logger) []error
func PreviewDecodeAutoFix(matches []DecodeMatch, ctx DecodeAutoFixContext) []string

// Context populated by Plan 50-03 CLI from the resolved cluster.
type DecodeAutoFixContext struct {
    BinaryName string // lifecycle.ProviderBinaryName()
    CPNodeName string // first control-plane container from provider.ListNodes()
}

// Factories — Plan 50-03 does NOT need to call these directly;
// they are embedded in Catalog (KUB-01/02) or constructed by mitigationFor.
func InotifyRaiseMitigation() *SafeMitigation
func CoreDNSRestartMitigation(binaryName, cpNodeName string) *SafeMitigation
func NodeContainerRestartMitigation(binaryName, nodeName string) *SafeMitigation
```

## Catalog AutoFixable Entries (exact set)

| ID      | AutoFixable | AutoFix pointer          | Notes                                          |
|---------|-------------|--------------------------|------------------------------------------------|
| KUB-01  | true        | InotifyRaiseMitigation() | parameterless; embedded in catalog             |
| KUB-02  | true        | InotifyRaiseMitigation() | same mitigation; deduped by Name at apply time |
| KADM-02 | true        | nil                      | constructed from ctx.BinaryName+CPNodeName     |
| KUB-05  | true        | nil                      | constructed from ctx.BinaryName+match.Source   |
| all other 12 | false  | nil                      | silently skipped by ApplyDecodeAutoFix         |

## Deviations from Plan

None — plan executed exactly as written.

The note about `grep -c "lifecycle\."` returning 0 applies to actual import statements; the two matches found are Go comments (`// realInspectStateAuto mirrors lifecycle.ContainerState` and `// cluster: BinaryName via lifecycle.ProviderBinaryName()`). No actual lifecycle import exists in decode_autofix.go — confirmed via `grep -n '".*lifecycle"'` returning empty.

## Verification Results

1. `go test -race ./pkg/internal/doctor/... -count=1` (decode-scope tests) — PASS (20 new tests)
2. `go vet ./pkg/internal/doctor/...` — CLEAN
3. `grep "lifecycle" import block in decode_autofix.go` — 0 (no import cycle)
4. `grep -E -c '^\s*AutoFixable:\s*true' decode_catalog.go` — 4 (exactly KUB-01/02/KADM-02/KUB-05)
5. `grep -c "var execCommand" decode_autofix.go` — 0 (owned by 50-02)
6. `grep -c "InotifyRaiseMitigation|CoreDNSRestartMitigation|NodeContainerRestartMitigation" decode_autofix.go` — 10 (3 defs + 3 internal refs + 4 catalog calls)
7. `git diff go.mod go.sum` — empty (zero new module deps)

## Plan 50-02 Wave Dependency Confirmation

Plan 50-02 (wave 2) was confirmed on disk and committed (`258f11ac` depends on `execCommand` from decode_collectors.go) before this plan ran. `execCommand` is simply referenced in realGetCoreDNSStatus/realInspectStateAuto/realExecCmd — no redeclaration, no filesystem park-aside needed.

## Known Stubs

None — all three mitigations have real production implementations (realReadSysctl, realWriteSysctl, realGetCoreDNSStatus, realInspectStateAuto, realExecCmd). No placeholder data flows to UI rendering.

## Threat Flags

None — decode_autofix.go does not introduce new network endpoints, auth paths, or file access patterns beyond the existing /proc/sys reads/writes (documented in the plan's RESEARCH §"Auto-Fix Whitelist" threat model). The NeedsRoot guard ensures sysctl writes are only attempted when the process has the necessary privilege.

## Self-Check: PASSED

All files verified present. All commits verified in git history:
- `4772c837` (Task 1 RED), `258f11ac` (Task 1 GREEN), `450e6641` (Task 2 RED), `f55236cb` (Task 2 GREEN), `0aefa56d` (docs)
- `go vet ./pkg/internal/doctor/...` clean
- `go test -race` decode-scope tests pass
- 4 AutoFixable=true entries confirmed
- 0 execCommand redeclarations in decode_autofix.go
- go.mod/go.sum unchanged (0 bytes diff)

## Next Phase Readiness

Plan 50-03 (cobra command for `kinder doctor decode`) can call:
- `doctor.PreviewDecodeAutoFix(result.Matches, doctor.DecodeAutoFixContext{BinaryName: binary, CPNodeName: cpNode})` for `--preview` mode
- `doctor.ApplyDecodeAutoFix(result.Matches, doctor.DecodeAutoFixContext{...}, logger)` for `--auto-fix` mode

No blockers. DIAG-04 / SC4 whitelist is complete.

---
*Phase: 50-runtime-error-decoder*
*Completed: 2026-05-07*
