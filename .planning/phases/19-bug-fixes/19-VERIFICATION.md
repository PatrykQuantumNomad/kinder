---
phase: 19-bug-fixes
verified: 2026-03-03T13:00:00Z
status: passed
score: 4/4 must-haves verified
gaps: []
human_verification: []
---

# Phase 19: Bug Fixes Verification Report

**Phase Goal:** All four correctness bugs are eliminated across every provider before any refactoring or new feature work begins
**Verified:** 2026-03-03T13:00:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #   | Truth                                                                                                        | Status     | Evidence                                                                                                    |
| --- | ------------------------------------------------------------------------------------------------------------ | ---------- | ----------------------------------------------------------------------------------------------------------- |
| 1   | Port listeners acquired in generatePortMappings are released at iteration end, not at function return        | VERIFIED   | All three providers call `releaseHostPortFn()` immediately after `args = append(...)`, zero `defer` usages  |
| 2   | Tar extraction returns an error on truncated files instead of silently stopping mid-archive                  | VERIFIED   | `tar.go` lines 78-79 check both `io.EOF` and `io.ErrUnexpectedEOF` and return a named truncation error      |
| 3   | ListInternalNodes resolves an empty cluster name to the default cluster name consistently across all three providers | VERIFIED   | `provider.go` line 231: `p.provider.ListNodes(defaultName(name))` — matches ListNodes and every other method |
| 4   | Network sort comparator satisfies strict weak ordering — identical networks compare equal, sort is deterministic | VERIFIED   | `network.go` lines 210-215: `iLen != jLen` guard; 100-run determinism test passes                           |

**Score:** 4/4 truths verified

---

### Required Artifacts

#### Plan 01 (BUG-01, BUG-02)

| Artifact                                                              | Expected                                              | Status   | Details                                                                                                                  |
| --------------------------------------------------------------------- | ----------------------------------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------ |
| `pkg/cluster/internal/providers/docker/provision.go`                  | Fixed generatePortMappings — immediate release        | VERIFIED | Lines 406-408: `if releaseHostPortFn != nil { releaseHostPortFn() }` after `args = append(...)`, no `defer`            |
| `pkg/cluster/internal/providers/nerdctl/provision.go`                 | Fixed generatePortMappings — immediate release        | VERIFIED | Lines 376-378: identical immediate-release pattern                                                                       |
| `pkg/cluster/internal/providers/podman/provision.go`                  | Fixed generatePortMappings — immediate release        | VERIFIED | Lines 421-423: identical immediate-release pattern (includes podman-specific `:0` stripping before release)             |
| `pkg/cluster/internal/providers/docker/provision_test.go`             | Test proving port release happens per-iteration       | VERIFIED | `multiple_port=0_mappings_all_acquire_distinct_ports_and_release_listeners` subtest re-binds ports after function return |
| `pkg/cluster/internal/providers/nerdctl/provision_test.go`            | Test proving port release happens per-iteration       | VERIFIED | Same per-iteration release subtest present                                                                               |
| `pkg/cluster/internal/providers/podman/provision_test.go`             | Test proving port release (created new file)          | VERIFIED | File created; per-iteration release subtest present                                                                      |
| `pkg/build/nodeimage/internal/kube/tar.go`                            | Fixed extractTarball — error on truncated archive     | VERIFIED | Lines 78-79: `err == io.EOF \|\| err == io.ErrUnexpectedEOF` returns `"archive truncated: ..."` error                   |
| `pkg/build/nodeimage/internal/kube/tar_test.go`                       | Test proving truncated tarball returns error           | VERIFIED | `TestExtractTarball_Truncated` with `writeTruncatedTarGz` helper; asserts `err != nil` and `strings.Contains("truncat")` |

#### Plan 02 (BUG-03, BUG-04)

| Artifact                                                              | Expected                                              | Status   | Details                                                                                                           |
| --------------------------------------------------------------------- | ----------------------------------------------------- | -------- | ----------------------------------------------------------------------------------------------------------------- |
| `pkg/cluster/provider.go`                                             | Fixed ListInternalNodes with defaultName() call       | VERIFIED | Line 231: `p.provider.ListNodes(defaultName(name))`                                                               |
| `pkg/cluster/provider_test.go`                                        | Test proving empty name resolves to default           | VERIFIED | `TestListInternalNodes_DefaultName`: mock records name passed to ListNodes; empty-string case expects `DefaultName` |
| `pkg/cluster/internal/providers/docker/network.go`                    | Fixed sortNetworkInspectEntries with strict weak ordering | VERIFIED | Lines 210-215: `iLen != jLen` guard before falling through to ID tiebreaker                                       |
| `pkg/cluster/internal/providers/docker/network_test.go`               | Test proving equal-container-count sorts by ID; more-containers wins | VERIFIED | "more containers wins over lower ID" case and `Test_sortNetworkInspectEntries_Deterministic` (100 runs)           |

---

### Key Link Verification

| From                                      | To                          | Via                                           | Status  | Details                                                                     |
| ----------------------------------------- | --------------------------- | --------------------------------------------- | ------- | --------------------------------------------------------------------------- |
| `docker/provision.go:generatePortMappings` | `common.PortOrGetFreePort`  | `releaseHostPortFn()` called after arg append | WIRED   | Immediate call at lines 406-408; `defer` keyword absent from loop           |
| `nerdctl/provision.go:generatePortMappings`| `common.PortOrGetFreePort`  | `releaseHostPortFn()` called after arg append | WIRED   | Immediate call at lines 376-378                                             |
| `podman/provision.go:generatePortMappings` | `common.PortOrGetFreePort`  | `releaseHostPortFn()` called after arg append | WIRED   | Immediate call at lines 421-423                                             |
| `tar.go:extractTarball`                   | `io.CopyN` return value     | EOF check returns error instead of break      | WIRED   | Lines 78-80: both `io.EOF` and `io.ErrUnexpectedEOF` produce named error   |
| `provider.go:ListInternalNodes`           | `defaultName()`             | wraps name before passing to p.provider.ListNodes | WIRED | Line 231: `p.provider.ListNodes(defaultName(name))`                       |
| `network.go:sortNetworkInspectEntries`    | `sort.Slice` comparator     | `iLen != jLen` guard with ID tiebreaker       | WIRED   | Lines 212-215: guard is present; ID fallthrough only when counts equal     |

---

### Requirements Coverage

| Requirement | Source Plan | Description                                                                        | Status    | Evidence                                                                           |
| ----------- | ----------- | ---------------------------------------------------------------------------------- | --------- | ---------------------------------------------------------------------------------- |
| BUG-01      | 19-01       | defer-in-loop port leak in generatePortMappings across all 3 providers             | SATISFIED | `defer` removed; immediate `releaseHostPortFn()` confirmed in all 3 provision.go   |
| BUG-02      | 19-01       | Silent tar truncation in extractTarball                                            | SATISFIED | `io.EOF`/`io.ErrUnexpectedEOF` both produce `"archive truncated: ..."` error       |
| BUG-03      | 19-02       | ListInternalNodes missing defaultName() call                                       | SATISFIED | `defaultName(name)` wrap present on line 231 of provider.go                        |
| BUG-04      | 19-02       | Network sort comparator violates strict weak ordering                              | SATISFIED | `iLen != jLen` guard present; 100-run determinism test passes                      |

---

### Anti-Patterns Found

No blocker or warning anti-patterns were found in the modified files. Grep for `defer releaseHostPortFn` across all three provision.go files produced zero matches (exit code 1 = no matches), confirming the defer was fully removed. No `TODO/FIXME/placeholder` patterns or stub implementations were found in any of the eight modified files.

---

### Human Verification Required

None. All four bug fixes are mechanically verifiable:

- BUG-01: Structural — `defer` keyword absent from loop; test re-binds released ports.
- BUG-02: Functional — test creates a genuinely truncated archive and asserts non-nil error with "truncat" in message.
- BUG-03: Structural — one-line change grep-confirmed; mock-based test asserts correct name propagation.
- BUG-04: Functional — sort test with adversarial input (higher-ID network listed first) confirms correct ordering; 100-run determinism test confirms stability.

---

### Test Results Summary

All targeted tests pass. Full `pkg/...` suite (23 packages) exits 0 with zero failures.

```
Test_generatePortMappings        docker   PASS  (6 subtests incl. port-release re-bind)
Test_generatePortMappings        nerdctl  PASS  (4 subtests incl. port-release re-bind)
Test_generatePortMappings        podman   PASS  (6 subtests incl. port-release re-bind)
TestExtractTarball_Truncated              PASS  (error contains "truncat")
TestExtractTarball_Normal                 PASS  (no regression)
TestListInternalNodes_DefaultName         PASS  (empty-name resolves to DefaultName)
Test_sortNetworkInspectEntries            PASS  (more-containers-wins-over-lower-ID case)
Test_sortNetworkInspectEntries_Deterministic PASS (100 identical runs)
go test ./pkg/... -short -tags=nointegration  EXIT 0  (23 packages, 0 failures)
```

---

### Structural Confirmation

**BUG-01 — No `defer releaseHostPortFn` anywhere in any provider:**

```
grep -n "defer releaseHostPortFn" pkg/cluster/internal/providers/*/provision.go
(no output — exit 1)
```

**BUG-02 — EOF branch returns error, not break:**

`tar.go` line 78: `if err == io.EOF || err == io.ErrUnexpectedEOF {`
`tar.go` line 79: `return fmt.Errorf("archive truncated: unexpected EOF while extracting %s", hdr.Name)`

**BUG-03 — defaultName wraps the argument:**

`provider.go` line 231: `n, err := p.provider.ListNodes(defaultName(name))`

**BUG-04 — iLen != jLen guard present:**

`network.go` lines 210-215:
```go
iLen := len(networks[i].Containers)
jLen := len(networks[j].Containers)
if iLen != jLen {
    return iLen > jLen
}
return networks[i].ID < networks[j].ID
```

---

*Verified: 2026-03-03T13:00:00Z*
*Verifier: Claude (gsd-verifier)*
