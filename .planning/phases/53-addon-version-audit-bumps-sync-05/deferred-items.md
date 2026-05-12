# Phase 53 Deferred Items

## Pre-existing Race Condition in pkg/internal/doctor

**Detected during:** 53-07 Task 1 verification (`go test ./pkg/internal/doctor/... -race`)
**Status:** Pre-existing — confirmed present before any 53-07 changes
**Impact:** All tests in the doctor package fail under `-race` due to a concurrent
write/read of a shared variable in `TestRunAllChecks_NilPlatformsRunsOnAll` and
`TestAllChecks_Registry` (socket_test.go). The race is between goroutine 11 (check_test.go:89)
writing `allChecks` state and goroutine 174 (socket_test.go:162) reading it.
**In scope for:** A future plan targeting doctor package test infrastructure hardening.
**Not in scope for:** Phase 53 (addon version bumps; unrelated to offline-readiness functionality).
**Workaround:** Run targeted test subsets with `-run TestOfflineReadiness|TestAllAddonImages`
to exercise offline-readiness without triggering the race.
