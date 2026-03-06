---
phase: 41-network-create-flow-integration-and-website
verified: 2026-03-06T18:30:00Z
status: passed
score: 3/3 must-haves verified
---

# Phase 41: Network Create Flow Integration and Website Verification Report

**Phase Goal:** Users get subnet clash detection before cluster creation, automatic safe mitigations during `kinder create cluster`, and a comprehensive Known Issues page on the website documenting all checks
**Verified:** 2026-03-06T18:30:00Z
**Status:** passed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `kinder doctor` detects when Docker network subnets overlap with host routing table entries and warns about potential connectivity issues, working on both Linux (`ip route`) and macOS (`netstat -rn`) | VERIFIED | `pkg/internal/doctor/subnet.go` implements `subnetClashCheck` with `getHostRoutesLinux()` (line 174: `exec.OutputLines(c.execCmd("ip", "route", "show"))`) and `getHostRoutesDarwin()` (line 203: `exec.OutputLines(c.execCmd("netstat", "-rn"))`). Returns `Status: "warn"` on overlap (line 113). `normalizeAbbreviatedCIDR` handles macOS abbreviated CIDR. 19 tests pass including VPN overlap, self-route skip, no-overlap, IPv6 filter, and 9 CIDR normalization cases. Registered as 18th check in `allChecks` at `check.go:79`. `go test ./pkg/internal/doctor/... -run TestSubnet` -- all pass. |
| 2 | `kinder create cluster` calls `ApplySafeMitigations()` after provider validation and before provisioning, applying only tier-1 mitigations (env vars, cluster config adjustments) automatically -- never calling sudo or modifying system files | VERIFIED | `create.go:175` calls `doctor.ApplySafeMitigations(logger)` after containerd config patches (line 163) and before `p.Provision()` (line 182). Import at `create.go:37`: `"sigs.k8s.io/kind/pkg/internal/doctor"`. `mitigations.go:49` early-returns on non-Linux. `mitigations.go:61` skips `NeedsRoot` mitigations when not root -- never escalates. `SafeMitigations()` returns `[]SafeMitigation{}` (non-nil empty slice, line 43). Errors logged as warnings (create.go:177), never fatal. `go build ./...` succeeds (no import cycle). Both mitigations tests pass. |
| 3 | The kinder website has a Known Issues / Troubleshooting page documenting every diagnostic check, what it detects, why it matters, and how to fix it | VERIFIED | `kinder-site/src/content/docs/known-issues.md` is 487 lines documenting all 18 checks across 8 categories (Runtime, Docker, Tools, GPU, Kernel, Security, Platform, Network) plus Automatic Mitigations section. Each check has What/Why/Platforms/Fix structure. Sidebar entry at `astro.config.mjs:89` (`{ slug: 'known-issues' }`) placed before changelog. Cross-link from `troubleshooting.md:49` to Known Issues page. |

**Score:** 3/3 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/internal/doctor/subnet.go` | Subnet clash check with cross-platform route parsing | VERIFIED | 293 lines. Contains `subnetClashCheck` struct, `newSubnetClashCheck()`, `getDockerNetworkSubnets()`, `getHostRoutesLinux()`, `getHostRoutesDarwin()`, `normalizeAbbreviatedCIDR()`. Uses `exec.OutputLines` for system commands, `netip.Prefix.Overlaps()` for CIDR comparison. |
| `pkg/internal/doctor/subnet_test.go` | Tests for subnet clash, CIDR normalization, route parsing | VERIFIED | 254 lines. 9 `TestNormalizeAbbreviatedCIDR` cases, 3 metadata tests, 7 `TestSubnetClashCheck_Run` cases (docker not found, no networks, no routes, VPN overlap, self-route skip, no overlap, IPv6 filter). All pass. |
| `pkg/internal/doctor/check.go` | Updated allChecks registry with network-subnet check | VERIFIED | `newSubnetClashCheck()` at line 79, 18th entry. Category comment "Network (Phase 41)" at line 78. |
| `pkg/internal/doctor/mitigations.go` | SafeMitigations returns empty non-nil slice, ApplySafeMitigations callable from create flow | VERIFIED | `SafeMitigations()` returns `[]SafeMitigation{}` (line 43). `ApplySafeMitigations` early-returns on non-Linux (line 49), skips NeedsRoot when not root (line 61). |
| `pkg/internal/doctor/mitigations_test.go` | Tests for SafeMitigations non-nil return and ApplySafeMitigations behavior | VERIFIED | `TestSafeMitigations_ReturnsEmptyNonNil` verifies non-nil and len==0. `TestApplySafeMitigations_EmptyReturnsNil` verifies nil errors return. Both pass. |
| `pkg/cluster/internal/create/create.go` | ApplySafeMitigations() call wired before p.Provision() | VERIFIED | Import at line 37. Call at line 175. Before `p.Provision()` at line 182. Errors logged as warnings (line 177). |
| `kinder-site/src/content/docs/known-issues.md` | Comprehensive Known Issues page for all 18 checks | VERIFIED | 487 lines, 8 category H2 sections, 18 check H3 sections, Automatic Mitigations section with tier-1 constraints documentation. |
| `kinder-site/astro.config.mjs` | Sidebar entry for known-issues page | VERIFIED | Line 89: `{ slug: 'known-issues' }` placed before `{ slug: 'changelog' }`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `check.go` | `subnet.go` | `newSubnetClashCheck()` in allChecks registry | WIRED | `check.go:79` calls `newSubnetClashCheck()` defined in `subnet.go:38` |
| `subnet.go` | `sigs.k8s.io/kind/pkg/exec` | `exec.OutputLines` for docker/route commands | WIRED | `subnet.go:136`, `174`, `203` call `exec.OutputLines()` |
| `create.go` | `mitigations.go` | `doctor.ApplySafeMitigations(logger)` | WIRED | `create.go:37` imports `sigs.k8s.io/kind/pkg/internal/doctor`, `create.go:175` calls `doctor.ApplySafeMitigations(logger)` |
| `astro.config.mjs` | `known-issues.md` | sidebar slug entry | WIRED | `astro.config.mjs:89` has `{ slug: 'known-issues' }`, resolves to `src/content/docs/known-issues.md` |
| `troubleshooting.md` | `known-issues.md` | cross-link | WIRED | `troubleshooting.md:49` links to `[Known Issues](/known-issues/)` |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| PLAT-03 | 41-01 | Doctor detects Docker network subnet clashes with host routes | SATISFIED | `subnet.go` implements full cross-platform detection. 19 tests pass. Registered in allChecks as check #18. |
| INFRA-05 | 41-02 | ApplySafeMitigations() integrated into create flow before p.Provision() | SATISFIED | `create.go:175` calls `doctor.ApplySafeMitigations(logger)` before `p.Provision()` at line 182. Non-nil empty slice from SafeMitigations(). Never calls sudo. |
| SITE-01 | 41-03 | Known Issues / Troubleshooting page documenting all checks and mitigations | SATISFIED | 487-line `known-issues.md` documents all 18 checks with What/Why/Platforms/Fix structure. Accessible via sidebar. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | - | - | - | No anti-patterns detected |

No TODO/FIXME/PLACEHOLDER/HACK comments found in phase files. No stub implementations. The `return []SafeMitigation{}` in mitigations.go is an intentional design choice (no tier-1 mitigations exist yet), not a stub -- the infrastructure is wired and the empty return is documented.

### Human Verification Required

### 1. Known Issues page renders correctly in browser

**Test:** Navigate to https://kinder.patrykgolabek.dev/known-issues/ after deployment
**Expected:** Page renders with all 18 checks across 8 categories, proper sidebar navigation, admonition blocks display correctly, code blocks have syntax highlighting
**Why human:** Visual rendering, CSS/layout correctness, and page navigation cannot be verified programmatically

### 2. Cross-link from Troubleshooting to Known Issues works

**Test:** Navigate to the Troubleshooting page, click the "Known Issues" link in the tip admonition
**Expected:** Navigates to /known-issues/ page successfully
**Why human:** Link resolution in deployed Astro site needs browser verification

### 3. Subnet clash detection on real host with VPN

**Test:** Run `kinder doctor` on a machine with an active VPN connection that uses 172.x.x.x ranges
**Expected:** Output includes a "warn" result for "network-subnet" showing the overlapping subnet
**Why human:** Requires real VPN environment and Docker daemon to exercise the actual system command execution paths

### Gaps Summary

No gaps found. All three observable truths are verified with full artifact existence, substantive implementation, and wiring confirmation. All three requirements (PLAT-03, INFRA-05, SITE-01) are satisfied. The Go build succeeds, all tests pass, and the known-issues documentation covers all 18 checks. Three items flagged for human verification relate to deployed website rendering and real-environment runtime behavior.

---

_Verified: 2026-03-06T18:30:00Z_
_Verifier: Claude (gsd-verifier)_
