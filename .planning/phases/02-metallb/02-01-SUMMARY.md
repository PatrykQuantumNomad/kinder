---
phase: "02-metallb"
plan: "01"
subsystem: "installmetallb"
tags: ["metallb", "subnet", "networking", "tdd", "ippool"]
dependency_graph:
  requires: []
  provides: ["subnet.go with detectSubnet, parseSubnetFromJSON, carvePoolFromSubnet"]
  affects: ["pkg/cluster/internal/create/actions/installmetallb"]
tech_stack:
  added: []
  patterns:
    - "Broadcast-address-based IP pool carving for MetalLB L2 pools"
    - "Provider-branching JSON parsing (Docker vs Podman schema)"
    - "TDD RED-GREEN cycle with zero external dependencies"
key_files:
  created:
    - "pkg/cluster/internal/create/actions/installmetallb/subnet.go"
    - "pkg/cluster/internal/create/actions/installmetallb/subnet_test.go"
  modified: []
decisions:
  - id: "02-01-A"
    summary: "carvePoolFromSubnet uses broadcast-address arithmetic — computes network broadcast, then sets last octet to .200-.250, making /20 produce 10.0.15.200-10.0.15.250 automatically"
  - id: "02-01-B"
    summary: "parseSubnetFromJSON branches on providerName=='podman' for schema selection — Docker and Nerdctl share identical IPAM.Config schema so no third branch needed"
metrics:
  duration: "2 minutes"
  completed: "2026-03-01"
  tasks_completed: 2
  files_created: 2
  files_modified: 0
---

# Phase 2 Plan 1: Subnet Detection and IP Pool Carving Summary

**One-liner:** IPv4 subnet auto-detection from Docker/Podman/Nerdctl network inspect JSON, with broadcast-address-based IP pool carving for MetalLB L2 pools (.200-.250 range).

## What Was Built

Two files were created:

**`subnet.go`** — Pure functions for MetalLB subnet detection:
- `parseSubnetFromJSON(output []byte, providerName string) (string, error)` — Dispatches to Docker or Podman parser based on provider name. Docker/Nerdctl uses `IPAM.Config[].Subnet`, Podman uses `subnets[].subnet`. Filters for IPv4 using `ip.To4() != nil`.
- `carvePoolFromSubnet(cidr string) (string, error)` — Computes broadcast address via bitwise OR of network address and inverted mask, then pins the pool at bytes .200-.250 of the last octet. Works correctly for /16, /24, and intermediate prefix lengths like /20.
- `detectSubnet(providerName string) (string, error)` — Runs `<provider> network inspect kind` via `exec.Command`/`exec.Output`, reads `KIND_EXPERIMENTAL_DOCKER_NETWORK` env override, delegates parsing to `parseSubnetFromJSON`.

**`subnet_test.go`** — 15 unit tests covering:
- `TestParseSubnetFromJSON`: Docker IPv4+IPv6, Nerdctl IPv4+IPv6, Docker IPv6-only (error), Podman subnets array, Podman empty subnets (error), empty JSON array (error), malformed JSON (error), Podman malformed JSON (error)
- `TestCarvePoolFromSubnet`: /16 (172.18), /24 (10.89), /24 (192.168.49), /16 (172.19), /20 (10.0), invalid CIDR (error), IPv6 CIDR (error)

## TDD Cycle

| Step | Commit | Status |
|------|--------|--------|
| RED - Failing tests | `9f5fb379` | Build failed: undefined `parseSubnetFromJSON`, `carvePoolFromSubnet` |
| GREEN - Implementation | `999466b4` | All 15 tests pass |
| REFACTOR | N/A | Code already clean, no changes needed |

## Decisions Made

**02-01-A: Broadcast-address arithmetic for carvePoolFromSubnet**

The pool range is derived by computing `broadcast = networkAddr | ^mask` for each byte, then fixing the last octet to 200 (start) and 250 (end). This naturally handles all prefix lengths:
- /16: broadcast is X.X.255.255 → pool is X.X.255.200-X.X.255.250
- /24: broadcast is X.X.X.255 → pool is X.X.X.200-X.X.X.250
- /20: broadcast is X.X.15.255 → pool is X.X.15.200-X.X.15.250

**02-01-B: Single branch on providerName == "podman"**

Docker and Nerdctl share identical JSON schemas (`IPAM.Config[].Subnet`), so only Podman requires a separate code path. Nerdctl falls through to the Docker parser with no changes needed.

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check

Files created:
- `pkg/cluster/internal/create/actions/installmetallb/subnet.go` - FOUND
- `pkg/cluster/internal/create/actions/installmetallb/subnet_test.go` - FOUND

Commits:
- `9f5fb379` (RED - failing tests) - FOUND
- `999466b4` (GREEN - implementation) - FOUND

Test result: PASS (15/15 tests)

## Self-Check: PASSED
