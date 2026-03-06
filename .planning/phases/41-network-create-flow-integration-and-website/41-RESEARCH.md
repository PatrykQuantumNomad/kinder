# Phase 41: Network, Create-Flow Integration, and Website - Research

**Researched:** 2026-03-06
**Domain:** Docker network subnet clash detection, create-flow mitigation wiring, Astro/Starlight documentation page
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PLAT-03 | Doctor detects Docker network subnet clashes with host routes | Architecture Patterns: subnet clash check using `net/netip.Prefix.Overlaps()`, cross-platform route parsing via `ip route` (Linux) and `netstat -rn` (macOS), injectable deps struct |
| INFRA-05 | ApplySafeMitigations() integrated into create flow before p.Provision() | Architecture Patterns: create.go insertion point after `validateProvider()` / `fixupOptions()` and before `p.Provision()`, non-fatal error handling |
| SITE-01 | Known Issues / Troubleshooting page documenting all checks and mitigations | Architecture Patterns: Starlight MDX page in `kinder-site/src/content/docs/`, sidebar config update in `astro.config.mjs`, content structure for all 18 checks |
</phase_requirements>

## Summary

Phase 41 completes the v2.1 milestone with three distinct work streams: (1) a Docker network subnet clash detection check (PLAT-03), (2) wiring the existing `ApplySafeMitigations()` skeleton into the `kinder create cluster` flow (INFRA-05), and (3) a comprehensive Known Issues / Troubleshooting page on the kinder website documenting all 18 diagnostic checks (SITE-01).

The subnet clash check is the final diagnostic check. It compares Docker network subnets (obtained via `docker network inspect`) against host routing table entries. On Linux, routes come from `ip route show`; on macOS, routes come from `netstat -rn`. The Go stdlib `net/netip.Prefix.Overlaps()` method (available since Go 1.18, kinder requires Go 1.24+) provides purpose-built CIDR overlap detection without manual computation. The macOS route table has a non-standard abbreviated format (e.g., `192.168.86` instead of `192.168.86.0/24`) that requires normalization before parsing.

The create-flow integration is a surgical insertion in `create.go`. The `ApplySafeMitigations()` function and its `SafeMitigation` struct already exist in `pkg/internal/doctor/mitigations.go` -- the function currently returns nil because `SafeMitigations()` returns an empty list. Phase 41 must: (a) populate `SafeMitigations()` with actual tier-1 mitigations (currently none of the 17 existing checks have auto-fixable mitigations -- see analysis below), (b) add a single call to `ApplySafeMitigations(logger)` in `create.go` after provider validation and before `p.Provision()`, and (c) handle the returned errors as informational warnings. The key constraint is that mitigations NEVER call sudo or modify system files -- only env vars and cluster config adjustments are allowed.

The website page is a Starlight/Astro MDX file documenting all 18 checks (17 existing + 1 new subnet clash) across 7 categories, explaining what each detects, why it matters, and how to fix it.

**Primary recommendation:** Implement the subnet clash check with `net/netip.Prefix.Overlaps()` and cross-platform route parsing, wire `ApplySafeMitigations()` into create.go as a single non-fatal call before provisioning, and create a comprehensive Known Issues page in `kinder-site/src/content/docs/` with all 18 checks documented.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net/netip` | Go 1.24+ | CIDR parsing and `Prefix.Overlaps()` for subnet clash detection | Purpose-built `Overlaps()` method; no manual computation; in stdlib since Go 1.18 |
| Go stdlib `os`, `os/exec`, `strings`, `fmt` | Go 1.24+ | Route parsing, command execution, string manipulation | Already used across all existing checks |
| `sigs.k8s.io/kind/pkg/exec` | (internal) | Command execution with `Cmd` interface for testability | Already used by all existing checks; enables FakeCmd injection |
| `sigs.k8s.io/kind/pkg/log` | (internal) | Logger interface for `ApplySafeMitigations()` | Already used in create.go and mitigations.go |
| Astro + Starlight | ^5.6.1 / ^0.37.6 | Website documentation page | Already used for all existing kinder-site pages |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `sigs.k8s.io/kind/pkg/internal/doctor` | (internal) | Check interface, Result type, allChecks registry | Adding subnet clash check and registering it |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `net/netip.Prefix.Overlaps()` | Manual `net.IPNet.Contains()` overlap check | `Overlaps()` is purpose-built and commutative; manual Contains requires checking both directions |
| Shell out to `ip route` / `netstat -rn` | `vishvananda/netlink` (Linux-only) | netlink is Linux-only, adds dependency, uses raw syscalls; shell-out is zero-dependency and cross-platform |
| Shell out to `netstat -rn` (macOS) | `golang.org/x/net/route` | x/net/route has known issues with IPv4 netmask parsing on macOS (golang/go#71578); shell-out is more reliable |

**Installation:**
```bash
# No new dependencies needed. go.mod unchanged. net/netip is stdlib.
# Website: npm install already done in kinder-site/.
```

## Architecture Patterns

### Recommended Project Structure
```
pkg/internal/doctor/
    subnet.go              # NEW: subnet clash check (PLAT-03)
    subnet_test.go         # NEW: tests for subnet clash check
    mitigations.go         # MODIFIED: populate SafeMitigations() with actual mitigations
    mitigations_test.go    # MODIFIED: test SafeMitigations returns populated list
    check.go               # MODIFIED: add newSubnetClashCheck() to allChecks registry

pkg/cluster/internal/create/
    create.go              # MODIFIED: add ApplySafeMitigations() call before p.Provision()

kinder-site/src/content/docs/
    known-issues.md        # NEW: Known Issues / Troubleshooting page (SITE-01)

kinder-site/
    astro.config.mjs       # MODIFIED: add known-issues to sidebar
```

### Pattern 1: Subnet Clash Check with Cross-Platform Route Parsing
**What:** A check that compares Docker network subnets against host routing table entries using `net/netip.Prefix.Overlaps()`.
**When to use:** For the PLAT-03 requirement.
**Example:**
```go
// Source: prior research (.planning/research/STACK.md) + net/netip official docs

package doctor

import (
    "fmt"
    "net/netip"
    osexec "os/exec"
    "runtime"
    "strings"

    "sigs.k8s.io/kind/pkg/exec"
)

type subnetClashCheck struct {
    lookPath   func(string) (string, error)
    execCmd    func(name string, args ...string) exec.Cmd
}

func newSubnetClashCheck() Check {
    return &subnetClashCheck{
        lookPath:   osexec.LookPath,
        execCmd:    exec.Command,
    }
}

func (c *subnetClashCheck) Name() string       { return "network-subnet" }
func (c *subnetClashCheck) Category() string    { return "Network" }
func (c *subnetClashCheck) Platforms() []string { return nil } // all platforms

func (c *subnetClashCheck) Run() []Result {
    // Step 1: Check docker is available.
    if _, err := c.lookPath("docker"); err != nil {
        return []Result{{
            Name: c.Name(), Category: c.Category(),
            Status: "skip", Message: "(docker not found)",
        }}
    }

    // Step 2: Get Docker network subnets (both "kind" and "bridge").
    kindSubnets := c.getDockerNetworkSubnets("kind")
    bridgeSubnets := c.getDockerNetworkSubnets("bridge")
    allSubnets := append(kindSubnets, bridgeSubnets...)
    if len(allSubnets) == 0 {
        return []Result{{
            Name: c.Name(), Category: c.Category(),
            Status: "ok", Message: "No Docker networks to check",
        }}
    }

    // Step 3: Get host routes.
    hostRoutes := c.getHostRoutes()
    if len(hostRoutes) == 0 {
        return []Result{{
            Name: c.Name(), Category: c.Category(),
            Status: "ok", Message: "Could not read host routes; skipping clash check",
        }}
    }

    // Step 4: Check for overlaps.
    for _, dockerPrefix := range allSubnets {
        for _, hostRoute := range hostRoutes {
            if dockerPrefix.Overlaps(hostRoute) {
                // Exclude self-referential matches (Docker's own routes).
                if dockerPrefix == hostRoute {
                    continue
                }
                return []Result{{
                    Name: c.Name(), Category: c.Category(),
                    Status:  "warn",
                    Message: fmt.Sprintf("Docker network %s overlaps host route %s", dockerPrefix, hostRoute),
                    Reason:  "Subnet clash may cause connectivity issues between cluster nodes and host services",
                    Fix:     "Configure custom Docker address pools in daemon.json: https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files",
                }}
            }
        }
    }

    return []Result{{
        Name: c.Name(), Category: c.Category(),
        Status:  "ok",
        Message: fmt.Sprintf("No subnet clashes detected (%d networks, %d host routes)", len(allSubnets), len(hostRoutes)),
    }}
}
```

### Pattern 2: macOS Route Parsing with Abbreviated CIDR Normalization
**What:** macOS `netstat -rn` uses abbreviated destination notation (e.g., `192.168.86` meaning `192.168.86.0/24`, `127` meaning `127.0.0.0/8`). These must be normalized to proper CIDR before `netip.ParsePrefix()`.
**When to use:** Parsing macOS routing table output.
**Example:**
```go
// Source: verified against actual macOS netstat -rn output on development machine

// normalizeAbbreviatedCIDR expands macOS abbreviated route destinations
// to full CIDR notation. Examples:
//   "127"           -> "127.0.0.0/8"
//   "169.254"       -> "169.254.0.0/16"
//   "192.168.86"    -> "192.168.86.0/24"
//   "192.168.86.1"  -> skip (host route, not a network)
//   "10.0.0.0/8"    -> "10.0.0.0/8" (already CIDR, pass through)
//   "default"       -> skip
func normalizeAbbreviatedCIDR(dest string) (netip.Prefix, bool) {
    // Already has CIDR notation.
    if strings.Contains(dest, "/") {
        prefix, err := netip.ParsePrefix(dest)
        return prefix, err == nil
    }

    // Skip non-numeric entries (default, link#N, etc.).
    if len(dest) == 0 || (dest[0] < '0' || dest[0] > '9') {
        return netip.Prefix{}, false
    }

    // Count octets to determine implied prefix length.
    octets := strings.Split(dest, ".")
    switch len(octets) {
    case 1:
        // e.g., "127" -> "127.0.0.0/8"
        prefix, err := netip.ParsePrefix(dest + ".0.0.0/8")
        return prefix, err == nil
    case 2:
        // e.g., "169.254" -> "169.254.0.0/16"
        prefix, err := netip.ParsePrefix(dest + ".0.0/16")
        return prefix, err == nil
    case 3:
        // e.g., "192.168.86" -> "192.168.86.0/24"
        prefix, err := netip.ParsePrefix(dest + ".0/24")
        return prefix, err == nil
    case 4:
        // Full IP without CIDR -- this is a host route, skip for subnet clash.
        return netip.Prefix{}, false
    default:
        return netip.Prefix{}, false
    }
}

func (c *subnetClashCheck) getHostRoutes() []netip.Prefix {
    var routeLines []string
    if runtime.GOOS == "linux" {
        routeLines, _ = exec.OutputLines(c.execCmd("ip", "route", "show"))
    } else {
        routeLines, _ = exec.OutputLines(c.execCmd("netstat", "-rn"))
    }

    var routes []netip.Prefix
    for _, line := range routeLines {
        fields := strings.Fields(line)
        if len(fields) == 0 {
            continue
        }
        dest := fields[0]

        if runtime.GOOS == "linux" {
            // Linux ip route: first field is CIDR like "172.17.0.0/16"
            if prefix, err := netip.ParsePrefix(dest); err == nil {
                routes = append(routes, prefix)
            }
        } else {
            // macOS netstat -rn: first field is abbreviated destination
            if prefix, ok := normalizeAbbreviatedCIDR(dest); ok {
                routes = append(routes, prefix)
            }
        }
    }
    return routes
}
```

### Pattern 3: Create-Flow Integration (Surgical Insertion)
**What:** A single `ApplySafeMitigations()` call in `create.go` after provider validation and before `p.Provision()`.
**When to use:** INFRA-05 requirement.
**Insertion point in create.go:**
```go
// In Cluster() function, AFTER validateProvider() + fixupOptions() + alreadyExists()
// and BEFORE p.Provision():

func Cluster(logger log.Logger, p providers.Provider, opts *ClusterOptions) error {
    // ... existing validation code ...

    // Apply safe mitigations before provisioning (Phase 41).
    // Mitigations are tier-1 only: env vars, cluster config adjustments.
    // Never calls sudo or modifies system files.
    // Errors are informational, not fatal -- log and continue.
    if errs := doctor.ApplySafeMitigations(logger); len(errs) > 0 {
        for _, err := range errs {
            logger.Warnf("Mitigation warning: %v", err)
        }
    }

    // Create node containers implementing defined config Nodes
    if err := p.Provision(status, opts.Config); err != nil {
        // ... existing error handling ...
    }
}
```

**Import needed in create.go:**
```go
import "sigs.k8s.io/kind/pkg/internal/doctor"
```

**Critical constraint:** The import path `sigs.k8s.io/kind/pkg/internal/doctor` works because `create.go` is in `pkg/cluster/internal/create/` which is within the same module. Go's `internal` package rule allows any package within the `sigs.k8s.io/kind` module to import `pkg/internal/doctor/`.

### Pattern 4: SafeMitigations Population
**What:** Populating the `SafeMitigations()` function with actual tier-1 mitigations.
**Current state analysis:**

Reviewing all 17 existing checks plus the new subnet clash check, here are the tier-1 mitigation candidates:

| Check | Auto-fixable? | Why/Why Not |
|-------|--------------|-------------|
| container-runtime | No | Cannot install Docker |
| disk-space | No | `docker system prune` is destructive |
| daemon-json | No | Modifying daemon.json is system-level |
| docker-snap | No | Cannot change install method |
| docker-socket | No | Requires root to add user to group |
| kubectl | No | Cannot install/upgrade kubectl |
| kubectl-version-skew | No | Cannot change kubectl version |
| nvidia-* (3 checks) | No | GPU driver install is user decision |
| inotify-limits | No | Requires sudo for sysctl; OUT OF SCOPE per constraints |
| kernel-version | No | Kernel upgrade is OS-level |
| apparmor | No | Security policy is user decision |
| selinux | No | Security policy is user decision |
| firewalld | No | Backend change affects all networking |
| wsl2-cgroup | No | .wslconfig change requires WSL restart |
| rootfs-device | No | Filesystem change is impractical |
| network-subnet | No | daemon.json change is system-level |

**Conclusion:** Currently, NONE of the 18 checks have tier-1 auto-fixable mitigations that meet the constraints (no sudo, no system file modification). The `SafeMitigations()` function will return an empty slice `[]SafeMitigation{}` (not nil). The infrastructure is wired and ready for future mitigations (e.g., setting env vars, adjusting cluster config) but no mitigations exist today.

**This is correct behavior.** The requirement says "applying only tier-1 mitigations (env vars, cluster config adjustments) automatically." Since none of the current checks have such mitigations, the function correctly does nothing. The wiring is the requirement -- having it called from the create flow before provisioning, ready for future mitigations.

### Pattern 5: Website Known Issues Page Structure
**What:** An Astro/Starlight MDX page documenting all 18 checks.
**When to use:** SITE-01 requirement.
**Structure:**
```markdown
---
title: Known Issues
description: Comprehensive guide to every diagnostic check kinder doctor performs, what it detects, why it matters, and how to fix it.
---

Brief intro explaining kinder doctor and the purpose of this page.

## How to run diagnostics

`kinder doctor` command, exit codes, JSON output.

## Runtime

### Container Runtime
What: ...
Why: ...
Fix: ...

## Docker

### Disk Space
...
### Docker Init Daemon Config
...
(etc for all 4 Docker checks)

## Tools

### kubectl
...
### kubectl Version Skew
...

## GPU

### NVIDIA Driver
...
(etc for all 3 GPU checks)

## Kernel

### Inotify Limits
...
### Kernel Version
...

## Security

### AppArmor
...
### SELinux
...

## Platform

### Firewalld
...
### WSL2 Cgroup
...
### Rootfs Device
...

## Network

### Subnet Clashes
...

## Automatic Mitigations

Explain that `kinder create cluster` runs ApplySafeMitigations() before provisioning.
Document what tier-1 means and the safety constraints.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All checks ok or skip |
| 1 | One or more failures |
| 2 | Warnings but no failures |
```

**Sidebar placement:** Add to the CLI Reference section, or as a top-level page. Given the comprehensiveness, a top-level sidebar entry is better for discoverability.

### Anti-Patterns to Avoid
- **Self-referential subnet match:** When checking Docker network subnets against host routes, Docker's own network creates a host route entry. The check must skip cases where `dockerPrefix == hostRoute` to avoid false positives (Docker always creates a host route for its own subnet).
- **Blocking create on mitigation failure:** `ApplySafeMitigations()` errors must be informational warnings, NEVER fatal. The function returns `[]error` and the caller logs them with `Warnf`, then continues to `p.Provision()`.
- **Importing `pkg/internal/doctor` from wrong scope:** The import works from `pkg/cluster/internal/create/` because both are within the `sigs.k8s.io/kind` module. Do NOT try to import from outside the module.
- **Parsing macOS routes as Linux CIDR:** macOS `netstat -rn` uses abbreviated notation (`192.168.86` not `192.168.86.0/24`). Passing raw macOS route destinations to `netip.ParsePrefix()` will fail.
- **Checking only the "kind" network:** Docker's default "bridge" network can also clash. Check both "kind" and "bridge" network subnets.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CIDR overlap detection | Manual `net.IPNet.Contains()` bidirectional check | `net/netip.Prefix.Overlaps()` | Overlaps() is commutative, purpose-built, handles IPv4/IPv6 families, in stdlib |
| Route table enumeration | `vishvananda/netlink` or `golang.org/x/net/route` | Shell out to `ip route` / `netstat -rn` | Zero new dependencies; netlink is Linux-only; x/net/route has macOS IPv4 netmask parsing bugs (golang/go#71578) |
| Docker network inspection | Parsing raw `docker network ls` text | `docker network inspect --format` Go template | Structured output via Go template is reliable; raw text parsing is fragile |
| Starlight page creation | Custom HTML/React components | Standard Starlight MDX with Astro components (notes, tips, tables) | Matches all existing kinder-site pages; Starlight handles dark theme, sidebar, search |

**Key insight:** This phase has zero new dependencies. `net/netip` is stdlib, route parsing uses existing shell-out patterns, and the website uses the existing Astro/Starlight stack. The primary work is integration and documentation, not greenfield implementation.

## Common Pitfalls

### Pitfall 1: Docker Network Self-Route False Positive
**What goes wrong:** Docker creates a host route for its own network (e.g., `172.19.0.0/16 dev br-xxx`). The subnet clash check reports Docker's own subnet as clashing with a host route -- which is actually Docker's own route.
**Why it happens:** The `Overlaps()` check correctly identifies that the Docker subnet and its own host route overlap, because they are identical.
**How to avoid:** Skip exact matches (`dockerPrefix == hostRoute`). Only report overlaps where the route is NOT the Docker network's own route.
**Warning signs:** `kinder doctor` always warns about subnet clash even on a clean machine with no VPN.

### Pitfall 2: macOS Abbreviated Route Parsing
**What goes wrong:** `netip.ParsePrefix("192.168.86")` returns an error because macOS abbreviates routes by dropping trailing zero octets and the CIDR suffix.
**Why it happens:** macOS `netstat -rn` uses a non-standard format where `192.168.86` means `192.168.86.0/24`, `127` means `127.0.0.0/8`, and `169.254` means `169.254.0.0/16`. This is not valid CIDR notation.
**How to avoid:** Write a `normalizeAbbreviatedCIDR()` helper that counts the number of dotted octets and infers the prefix length. Test with real macOS routing table samples (including entries like `default`, `link#N`, MAC addresses).
**Warning signs:** Subnet clash check always reports "no clashes" on macOS because route parsing silently fails for every entry.

### Pitfall 3: ApplySafeMitigations Import Cycle
**What goes wrong:** `create.go` in `pkg/cluster/internal/create/` imports `pkg/internal/doctor/` which (hypothetically) imports something from `pkg/cluster/`. This would create an import cycle.
**Why it happens:** Go does not allow circular imports.
**How to avoid:** `pkg/internal/doctor/` must NEVER import from `pkg/cluster/`. Verify that `mitigations.go` only uses stdlib and `sigs.k8s.io/kind/pkg/log`. The current implementation is already correct -- `mitigations.go` imports only `fmt`, `os`, `runtime`, and `sigs.k8s.io/kind/pkg/log`.
**Warning signs:** `go build` fails with "import cycle not allowed" after adding the import.

### Pitfall 4: SafeMitigations Returning nil vs Empty Slice
**What goes wrong:** If `SafeMitigations()` returns nil (current skeleton), the for-loop in `ApplySafeMitigations()` still works (range over nil is zero iterations). But the distinction matters for testing -- a test might assert `len(mitigations) == 0` which passes for both nil and empty slice, but `mitigations != nil` would differentiate.
**Why it happens:** The skeleton was designed as a placeholder returning nil.
**How to avoid:** Change `SafeMitigations()` to return `[]SafeMitigation{}` (empty non-nil slice) to match the `allChecks` non-nil guarantee pattern established in Phase 38.
**Warning signs:** Tests using `!= nil` check fail after mitigation population.

### Pitfall 5: Website Page Not in Sidebar
**What goes wrong:** Creating the MDX file but forgetting to add it to `astro.config.mjs` sidebar configuration. The page exists and is accessible via direct URL but users cannot navigate to it.
**Why it happens:** Starlight does not auto-discover pages for the sidebar; the sidebar config is explicit.
**How to avoid:** Add the slug to the sidebar array in `astro.config.mjs`. The existing CLI Reference section or a new top-level entry are both valid placements.
**Warning signs:** Page builds successfully but does not appear in the left sidebar navigation.

### Pitfall 6: Overlooking IPv6 Routes in Overlap Check
**What goes wrong:** The kind network has both IPv4 (`172.19.0.0/16`) and IPv6 (`fc00:f853:ccd:e793::/64`) subnets. If only IPv4 host routes are compared, IPv6 subnet clashes go undetected.
**Why it happens:** `ip route show` only shows IPv4 routes. IPv6 requires `ip -6 route show`.
**How to avoid:** Focus on IPv4 for the initial implementation. IPv6 subnet clashes in the `fc00::/8` ULA range are extremely rare in practice (VPNs and corporate networks use IPv4 subnets). Document this limitation. `Overlaps()` correctly handles different address families by returning false, so mixing IPv4 and IPv6 prefixes is safe -- it just never matches.
**Warning signs:** None expected. This is a deliberate scope limitation.

## Code Examples

Verified patterns from official sources and the existing codebase:

### Docker Network Inspection
```go
// Source: existing pattern in pkg/cluster/internal/providers/docker/network.go
// + verified against docker network inspect output on development machine

func (c *subnetClashCheck) getDockerNetworkSubnets(networkName string) []netip.Prefix {
    lines, err := exec.OutputLines(c.execCmd(
        "docker", "network", "inspect", networkName,
        "--format", "{{range .IPAM.Config}}{{.Subnet}} {{end}}",
    ))
    if err != nil || len(lines) == 0 {
        return nil
    }
    var prefixes []netip.Prefix
    for _, s := range strings.Fields(strings.TrimSpace(lines[0])) {
        if prefix, err := netip.ParsePrefix(s); err == nil {
            // Focus on IPv4 for subnet clash detection.
            if prefix.Addr().Is4() {
                prefixes = append(prefixes, prefix)
            }
        }
    }
    return prefixes
}
```

### Linux Route Parsing (ip route show)
```go
// Source: Linux ip route show format -- verified against multiple sources
// Example line: "172.17.0.0/16 dev docker0 proto kernel scope link src 172.17.0.1"
// First field is always CIDR notation.

// Linux parsing is straightforward: first field is CIDR.
// Skip "default" entries (not relevant for subnet clash).
for _, line := range routeLines {
    fields := strings.Fields(line)
    if len(fields) == 0 || fields[0] == "default" {
        continue
    }
    if prefix, err := netip.ParsePrefix(fields[0]); err == nil {
        if prefix.Addr().Is4() {
            routes = append(routes, prefix)
        }
    }
}
```

### macOS Route Parsing (netstat -rn)
```go
// Source: actual macOS netstat -rn output captured on development machine
// Example lines:
//   "default            192.168.86.1       UGScg                 en1"
//   "127                127.0.0.1          UCS                   lo0"
//   "169.254            link#23            UCS                   en1"
//   "192.168.64         link#33            UC              bridge100"
//   "192.168.86         link#23            UCS                   en1"
//   "192.168.86.1/32    link#23            UCS                   en1"
//
// Key observations:
// - Abbreviated destinations: "127" = 127.0.0.0/8, "192.168.86" = 192.168.86.0/24
// - Some entries have explicit CIDR: "192.168.86.1/32"
// - Skip: "default", MAC addresses, link#N when used as destination
// - First field is always the destination

for _, line := range routeLines {
    fields := strings.Fields(line)
    if len(fields) == 0 {
        continue
    }
    if prefix, ok := normalizeAbbreviatedCIDR(fields[0]); ok {
        if prefix.Addr().Is4() {
            routes = append(routes, prefix)
        }
    }
}
```

### Create-Flow Wiring
```go
// Source: existing create.go structure + mitigations.go API
// Insert location: after validation, before p.Provision()

import "sigs.k8s.io/kind/pkg/internal/doctor"

// In Cluster() function, after "we're going to start creating now" log line
// and before containerd config patches / p.Provision():

// Apply safe mitigations before provisioning.
if errs := doctor.ApplySafeMitigations(logger); len(errs) > 0 {
    for _, err := range errs {
        logger.Warnf("Mitigation warning: %v", err)
    }
}
```

### Test Pattern for Subnet Clash Check
```go
// Source: existing fakeCmd pattern from testhelpers_test.go

func TestSubnetClashCheck_NoDocker(t *testing.T) {
    t.Parallel()
    check := &subnetClashCheck{
        lookPath: func(name string) (string, error) {
            return "", fmt.Errorf("not found")
        },
        execCmd: newFakeExecCmd(nil),
    }
    results := check.Run()
    if len(results) != 1 || results[0].Status != "skip" {
        t.Errorf("expected skip when docker not found, got %+v", results)
    }
}

func TestSubnetClashCheck_OverlapDetected(t *testing.T) {
    t.Parallel()
    check := &subnetClashCheck{
        lookPath: func(name string) (string, error) {
            return "/usr/bin/" + name, nil
        },
        execCmd: newFakeExecCmd(map[string]fakeExecResult{
            "docker network inspect kind --format {{range .IPAM.Config}}{{.Subnet}} {{end}}": {
                lines: "172.19.0.0/16\n",
            },
            "docker network inspect bridge --format {{range .IPAM.Config}}{{.Subnet}} {{end}}": {
                lines: "172.17.0.0/16\n",
            },
            "ip route show": {
                lines: "default via 10.0.0.1 dev eth0\n172.19.0.0/16 dev br-xxx proto kernel scope link\n172.16.0.0/12 via 10.0.0.1 dev tun0\n",
            },
        }),
    }
    results := check.Run()
    // Should warn: 172.19.0.0/16 overlaps 172.16.0.0/12
    // But NOT warn about the self-referential 172.19.0.0/16 route
    if len(results) != 1 || results[0].Status != "warn" {
        t.Errorf("expected warn for VPN overlap, got %+v", results)
    }
}

func TestNormalizeAbbreviatedCIDR(t *testing.T) {
    t.Parallel()
    tests := []struct {
        input string
        want  string
        ok    bool
    }{
        {"127", "127.0.0.0/8", true},
        {"169.254", "169.254.0.0/16", true},
        {"192.168.86", "192.168.86.0/24", true},
        {"192.168.86.1/32", "192.168.86.1/32", true},
        {"192.168.86.1", "", false},  // host route
        {"default", "", false},
        {"link#23", "", false},
    }
    for _, tt := range tests {
        prefix, ok := normalizeAbbreviatedCIDR(tt.input)
        if ok != tt.ok {
            t.Errorf("normalizeAbbreviatedCIDR(%q): ok=%v, want %v", tt.input, ok, tt.ok)
        }
        if ok && prefix.String() != tt.want {
            t.Errorf("normalizeAbbreviatedCIDR(%q) = %s, want %s", tt.input, prefix, tt.want)
        }
    }
}
```

### Website Page Frontmatter
```markdown
---
title: Known Issues
description: Every diagnostic check kinder doctor runs, what it detects, why it matters, and how to fix it.
---
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No subnet clash detection | `net/netip.Prefix.Overlaps()` cross-platform check | Phase 41 (now) | VPN/corporate network users get advance warning |
| SafeMitigations() returns nil | SafeMitigations() returns empty slice, called from create flow | Phase 41 (now) | Create flow has mitigation hook ready for future use |
| Troubleshooting page with 3 checks | Known Issues page with all 18 checks documented | Phase 41 (now) | Comprehensive reference for all diagnostics |
| `net.IPNet.Contains()` for overlap | `net/netip.Prefix.Overlaps()` | Go 1.18+ (2022) | Purpose-built API, no manual bidirectional check |

**Deprecated/outdated:**
- The existing `cli-reference/troubleshooting.md` page covers only 3 checks. The new Known Issues page will supersede it as the comprehensive diagnostics reference. The existing page can remain for CLI-specific troubleshooting (exit codes, common errors) but should link to the Known Issues page for check details.

## Open Questions

1. **Should the subnet clash check inspect all Docker networks or just "kind" and "bridge"?**
   - What we know: Docker can have many user-defined networks. The "kind" network is created by kinder/kind. The "bridge" network is Docker's default.
   - What's unclear: Whether user-defined networks (e.g., from docker-compose) should also be checked.
   - Recommendation: Check only "kind" and "bridge" networks. User-defined networks are outside kinder's scope. If the "kind" network does not exist yet (first cluster creation), only check "bridge".

2. **Should the website Known Issues page replace or supplement the existing Troubleshooting page?**
   - What we know: `cli-reference/troubleshooting.md` currently documents exit codes, 3 checks, and common scenarios for `kinder env` and `kinder doctor`.
   - What's unclear: Whether to merge content or keep them separate.
   - Recommendation: Keep both. The existing Troubleshooting page covers CLI usage patterns (exit codes, JSON piping, CI integration). The new Known Issues page covers what each check detects and how to fix it. Add a cross-link from Troubleshooting to Known Issues.

3. **Where should Known Issues appear in the sidebar?**
   - What we know: Current sidebar structure is: Installation, Quick Start, Configuration, Addons (collapsed), Guides (collapsed), CLI Reference (collapsed), Changelog.
   - What's unclear: Top-level or nested under CLI Reference.
   - Recommendation: Add as a top-level entry before Changelog. It is a reference page that users will search for directly, not a CLI-specific topic. Placing it at the top level improves discoverability.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + Astro build verification |
| Config file | None needed for Go; `kinder-site/package.json` for Astro |
| Quick run command | `go test ./pkg/internal/doctor/... -count=1 -run TestSubnet` |
| Full suite command | `go test ./... -count=1 && cd kinder-site && npm run build` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLAT-03 | Subnet clash detection with cross-platform routes | unit | `go test ./pkg/internal/doctor/... -run TestSubnet -count=1` | No -- Wave 0 |
| PLAT-03 | macOS abbreviated CIDR normalization | unit | `go test ./pkg/internal/doctor/... -run TestNormalize -count=1` | No -- Wave 0 |
| INFRA-05 | ApplySafeMitigations called from create flow | integration | `go build ./...` (compilation check) + `go test ./pkg/internal/doctor/... -run TestMitigation -count=1` | Partial -- mitigations_test.go exists |
| SITE-01 | Known Issues page builds without errors | build | `cd kinder-site && npm run build` | No -- Wave 0 (page doesn't exist yet) |

### Sampling Rate
- **Per task commit:** `go test ./pkg/internal/doctor/... -count=1`
- **Per wave merge:** `go test ./... -count=1 && cd kinder-site && npm run build`
- **Phase gate:** Full suite green + site builds clean before verify-work

### Wave 0 Gaps
- [ ] `pkg/internal/doctor/subnet_test.go` -- covers PLAT-03 (subnet clash check, route parsing, CIDR normalization)
- [ ] `kinder-site/src/content/docs/known-issues.md` -- covers SITE-01 (page must exist for build to include it)

## Sources

### Primary (HIGH confidence)
- Direct codebase analysis: `pkg/internal/doctor/check.go` -- allChecks registry with 17 checks across 7 categories
- Direct codebase analysis: `pkg/internal/doctor/mitigations.go` -- SafeMitigation struct, ApplySafeMitigations() skeleton
- Direct codebase analysis: `pkg/cluster/internal/create/create.go` -- Cluster() function structure, insertion point before p.Provision()
- Direct codebase analysis: `pkg/internal/doctor/testhelpers_test.go` -- fakeCmd, fakeExecResult, newFakeExecCmd test infrastructure
- Direct codebase analysis: `kinder-site/astro.config.mjs` -- Starlight sidebar configuration, plugin setup
- Direct codebase analysis: `kinder-site/src/content/docs/cli-reference/troubleshooting.md` -- existing troubleshooting page
- Direct codebase analysis: `go.mod` -- Go 1.24 minimum, no new dependencies needed
- Go stdlib `net/netip` documentation: https://pkg.go.dev/net/netip -- `Prefix.Overlaps()` method confirmed
- Actual macOS routing table output captured on development machine -- abbreviated CIDR format verified
- Actual Docker network inspect output: kind network has both IPv4 (172.19.0.0/16) and IPv6 (fc00:f853:ccd:e793::/64) subnets
- Prior project research: `.planning/research/STACK.md` -- Check 13 specification, `net/netip.Overlaps()` recommendation
- Prior project research: `.planning/research/FEATURES.md` -- All 13 check specifications with detection methods
- Prior project research: `.planning/research/PITFALLS.md` -- Subnet clash cross-platform route parsing pitfall

### Secondary (MEDIUM confidence)
- Kind Known Issues page: https://kind.sigs.k8s.io/docs/user/known-issues/ -- "Local Subnet Clashes" section documenting the problem and daemon.json workaround
- TunnelsUp macOS routing guide: https://www.tunnelsup.com/how-to-see-the-routing-table-on-mac-osx/ -- macOS netstat -rn format

### Tertiary (LOW confidence)
- golang/go#71578: x/net/route IPv4 netmask parsing issue on macOS -- confirms shell-out is safer than using the route package

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- zero new dependencies, all patterns verified against existing codebase and stdlib docs
- Architecture: HIGH -- all three work streams use established codebase patterns (Check interface, create.go structure, Starlight page)
- Pitfalls: HIGH -- macOS route format verified on actual development machine, self-referential route false positive identified from real Docker network state
- Code examples: HIGH -- all examples derived from existing codebase patterns and verified against real tool outputs

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable domain -- Go stdlib, Docker CLI, Starlight MDX)
