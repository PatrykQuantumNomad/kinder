---
phase: 29-cli-features
verified: 2026-03-04T12:00:00Z
status: human_needed
score: 11/11 must-haves verified
human_verification:
  - test: "Run kinder env --output json | jq empty"
    expected: "Exit 0; stderr may contain provider-detection warnings but stdout is clean JSON with kinderProvider, clusterName, kubeconfig keys"
    why_human: "Requires a container runtime to be present for full provider detection; logger output path to stderr cannot be confirmed without execution"
  - test: "Run kinder doctor --output json | jq empty"
    expected: "Exit 0 (or 1/2 for fail/warn); stdout is a JSON array of objects with name, status, message fields; no human-readable text on stdout"
    why_human: "Requires container runtime and kubectl present/absent to produce real check results; exit code semantics require execution"
  - test: "Run kinder get clusters --output json | jq empty"
    expected: "Exit 0; stdout is [] when no clusters exist, or [\"name1\",...] when clusters exist; no log output on stdout"
    why_human: "Requires a container runtime; actual cluster listing requires Docker/Podman"
  - test: "Run kinder get nodes --output json | jq empty"
    expected: "Exit 0; stdout is [] when no nodes for default cluster, or [{\"name\":\"...\",\"role\":\"...\"},...] when nodes exist"
    why_human: "Requires a running cluster with nodes"
  - test: "Run kinder create cluster --profile minimal and verify no kinder addons are installed"
    expected: "Cluster created; no MetalLB, EnvoyGateway, MetricsServer, CoreDNSTuning, Dashboard, LocalRegistry, CertManager installed"
    why_human: "Requires Docker and a node image; addon installation is a runtime side-effect"
  - test: "Run kinder create cluster --profile full and verify all addons are installed"
    expected: "Cluster created; all seven addons installed"
    why_human: "Requires Docker and a node image; addon installation is a runtime side-effect"
  - test: "Run kinder create cluster --profile gateway and verify only MetalLB + EnvoyGateway are installed"
    expected: "Cluster created; only MetalLB and EnvoyGateway installed, others absent"
    why_human: "Requires Docker and a node image"
  - test: "Run kinder create cluster --profile ci and verify only MetricsServer + CertManager are installed"
    expected: "Cluster created; only MetricsServer and CertManager installed, others absent"
    why_human: "Requires Docker and a node image"
  - test: "Run kinder create cluster (no --profile) and confirm behavior matches current default"
    expected: "Cluster behaves identically to before phase 29; all addons enabled via v1alpha4 boolPtrTrue defaults"
    why_human: "Requires Docker and a node image; comparing addon state to a baseline"
---

# Phase 29: CLI Features Verification Report

**Phase Goal:** Every kinder read command accepts `--output json` and produces clean, jq-parseable JSON on stdout; `kinder create cluster --profile <name>` selects a named addon preset without requiring a YAML config file

**Verified:** 2026-03-04T12:00:00Z
**Status:** human_needed (all automated checks passed; 9 runtime integration tests need human verification)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (Plan 01)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `kinder env --output json` prints valid JSON to stdout with kinderProvider, clusterName, kubeconfig fields | VERIFIED | `env.go:75-86` — JSON branch uses anonymous struct with json tags, encoded via `json.NewEncoder(streams.Out).Encode(out)` |
| 2 | `kinder doctor --output json` prints a JSON array of check results to stdout with name, status, message fields | VERIFIED | `doctor.go:141-161` — `checkResult` struct has exported JSON-tagged fields; `json.NewEncoder(streams.Out).Encode(out)` writes to stdout |
| 3 | `kinder get clusters --output json` prints a JSON string array to stdout (empty array `[]` when no clusters) | VERIFIED | `clusters.go:71-76` — nil guard `clusters = []string{}` before `json.NewEncoder(streams.Out).Encode(clusters)` |
| 4 | Logger and warning output stays on stderr in all JSON modes | VERIFIED | `doctor.go:167-172` — human-readable output uses `streams.ErrOut`; JSON branch writes to `streams.Out`; `env.go` logger warnings go through `log.Logger` (stderr-routed by design) |

### Observable Truths (Plan 02)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 5 | `kinder get nodes --output json` prints a JSON array of objects with name and role fields to stdout | VERIFIED | `nodes.go:116-126` — `make([]nodeInfo, 0)` ensures non-null; encodes via `json.NewEncoder(streams.Out).Encode(infos)` |
| 6 | `kinder get nodes --output json` prints `[]` when no nodes exist | VERIFIED | `nodes.go:117` — `make([]nodeInfo, 0, len(allNodes))` produces empty slice (not nil); JSON encodes as `[]` |
| 7 | `kinder create cluster --profile minimal` creates a cluster with all addons disabled | VERIFIED | `createoption.go:147` — `o.Config.Addons = internalconfig.Addons{}` (zero value = all false) |
| 8 | `kinder create cluster --profile full` creates a cluster with all addons enabled | VERIFIED | `createoption.go:148-153` — all 7 `Addons` fields set to `true` |
| 9 | `kinder create cluster --profile gateway` enables only MetalLB + EnvoyGateway | VERIFIED | `createoption.go:154` — `Addons{MetalLB: true, EnvoyGateway: true}` |
| 10 | `kinder create cluster --profile ci` enables only MetricsServer + CertManager | VERIFIED | `createoption.go:156` — `Addons{MetricsServer: true, CertManager: true}` |
| 11 | `kinder create cluster` without `--profile` behaves identically to current default | VERIFIED | `createoption.go:135-137` — empty profile returns nil immediately (strict no-op); `createcluster.go:120` — Profile flag default is `""` |

**Score:** 11/11 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `pkg/cmd/kind/env/env.go` | --output json flag and JSON branch with json.NewEncoder | VERIFIED | `encoding/json` imported; `Output string` in flagpole; `json.NewEncoder(streams.Out)` at line 85 |
| `pkg/cmd/kind/doctor/doctor.go` | checkResult struct + JSON branch | VERIFIED | `checkResult` struct at lines 41-45 with exported Name/Status/Message fields and json tags; `json.NewEncoder(streams.Out)` at line 150 |
| `pkg/cmd/kind/get/clusters/clusters.go` | flagpole with Output, JSON branch | VERIFIED | `flagpole.Output` at line 34; `json.NewEncoder(streams.Out)` at line 75 |
| `pkg/cmd/kind/get/nodes/nodes.go` | nodeInfo struct + Output flag + JSON branch | VERIFIED | `nodeInfo` at lines 41-44; `Output string` in flagpole line 38; `json.NewEncoder(streams.Out)` at line 125 |
| `pkg/cluster/createoption.go` | CreateWithAddonProfile function | VERIFIED | Function at lines 130-162; nil Config guard at line 138; 4 profile cases + unknown error |
| `pkg/cmd/kind/create/cluster/createcluster.go` | Profile flag + CreateWithAddonProfile wired | VERIFIED | `Profile string` in flagpole line 42; `--profile` flag registered at lines 95-100; `cluster.CreateWithAddonProfile(flags.Profile)` at line 120 after `withConfig` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `pkg/cmd/kind/env/env.go` | `streams.Out` | `json.NewEncoder(streams.Out).Encode()` | WIRED | Line 85: `return json.NewEncoder(streams.Out).Encode(out)` |
| `pkg/cmd/kind/doctor/doctor.go` | `streams.Out` | `json.NewEncoder(streams.Out).Encode()` | WIRED | Line 150: `json.NewEncoder(streams.Out).Encode(out)` |
| `pkg/cmd/kind/get/clusters/clusters.go` | `streams.Out` | `json.NewEncoder(streams.Out).Encode()` | WIRED | Line 75: `return json.NewEncoder(streams.Out).Encode(clusters)` |
| `pkg/cmd/kind/get/nodes/nodes.go` | `streams.Out` | `json.NewEncoder(streams.Out).Encode()` | WIRED | Line 125: `return json.NewEncoder(streams.Out).Encode(infos)` |
| `pkg/cmd/kind/create/cluster/createcluster.go` | `pkg/cluster/createoption.go` | `cluster.CreateWithAddonProfile(flags.Profile)` | WIRED | Line 120 in `provider.Create()` call; placed after `withConfig` so profile overrides config-file addon settings |
| `pkg/cluster/createoption.go` | `pkg/internal/apis/config/types.go` | `o.Config.Addons = internalconfig.Addons{...}` | WIRED | Lines 147, 148-153, 154, 156 — all 4 profile cases assign to `o.Config.Addons` using `internalconfig.Addons` struct |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CLI-01 | 29-01 | `kinder env --output json` | SATISFIED | env.go JSON branch + streams.Out |
| CLI-02 | 29-01 | `kinder doctor --output json` | SATISFIED | doctor.go checkResult + JSON branch |
| CLI-03 | 29-01 | `kinder get clusters --output json` | SATISFIED | clusters.go nil-safe JSON branch |
| CLI-04 | 29-02 | `kinder get nodes --output json` | SATISFIED | nodes.go nodeInfo + JSON branch |
| CLI-05 | 29-02 | `kinder create cluster --profile` | SATISFIED | createoption.go + createcluster.go wiring |

### Build and Vet Verification

`go build ./...` — PASSED (no errors)

`go vet ./pkg/cmd/kind/env/ ./pkg/cmd/kind/doctor/ ./pkg/cmd/kind/get/clusters/ ./pkg/cmd/kind/get/nodes/ ./pkg/cluster/ ./pkg/cmd/kind/create/cluster/` — PASSED (no errors)

All 5 feature commits exist in git history:
- `d7d3f4ed` — feat(29-01): add --output json to kinder env command
- `8ab81bc9` — feat(29-01): add --output json to kinder doctor command
- `d5644875` — feat(29-01): add --output json to kinder get clusters command
- `090fabc0` — feat(29-02): add --output json flag to kinder get nodes
- `1741621e` — feat(29-02): add --profile flag to kinder create cluster

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `pkg/cmd/kind/get/clusters/clusters.go` | 42 | `// TODO(bentheelder): more detailed usage` | Info | Pre-existing comment unrelated to phase 29 work; no functional impact |

No blockers. No stubs. No placeholder implementations.

### Notable Implementation Details

**doctor.go — JSON null avoidance:** The JSON branch in `doctor.go` uses `var out []checkResult` (nil slice), then appends. This means if `results` is empty, `out` would be `null` in JSON, not `[]`. However, in practice `results` always has at least the container-runtime check and kubectl check, so this is not a functional gap. It is a stylistic difference from the `make([]T, 0)` pattern used in `nodes.go` and the nil guard used in `clusters.go`.

**doctor.go — hasFail/hasWarn before JSON branch:** Correctly computed in a pre-pass loop (lines 130-139) before the JSON/human branch. Exit codes are consistent regardless of output format.

**createoption.go — nil Config guard:** `CreateWithAddonProfile` loads default config via `internalencoding.Load("")` when `o.Config` is nil, preventing nil-pointer panics when `--profile` is used without `--config`. This is the correct pattern documented in `fixupOptions`.

### Human Verification Required

The phase goal requires end-to-end execution that depends on a container runtime (Docker/Podman), running clusters, and a node image. All automated static checks pass. The following integration tests need a live environment:

**1. kinder env --output json | jq empty**

Test: Run `kinder env --output json | jq empty`
Expected: Exit 0; any provider-detection warnings appear on stderr only; stdout is valid JSON with `kinderProvider`, `clusterName`, `kubeconfig` keys
Why human: Requires container runtime detection; logger stderr routing cannot be confirmed without execution

**2. kinder doctor --output json | jq empty**

Test: Run `kinder doctor --output json | jq empty`
Expected: Exit 0 (or 1 if container runtime absent); stdout is a JSON array like `[{"name":"docker","status":"ok"},{"name":"kubectl","status":"ok"}]`; no `[ OK ]`/`[FAIL]`/`[WARN]` text on stdout
Why human: Binary check results depend on host environment; exit code semantics require execution

**3. kinder get clusters --output json | jq empty**

Test: Run `kinder get clusters --output json | jq empty`
Expected: Exit 0; stdout is `[]` with no clusters, or `["kind"]` when a cluster named `kind` exists
Why human: Requires container runtime to list clusters

**4. kinder get nodes --output json | jq empty**

Test: Run `kinder get nodes --output json | jq empty`
Expected: Exit 0; stdout is `[]` with no nodes, or `[{"name":"kind-control-plane","role":"control-plane"}]` with a cluster
Why human: Requires a running cluster to list nodes with roles

**5. kinder create cluster --profile minimal (no addons)**

Test: `kinder create cluster --profile minimal --name test-minimal`; inspect what addons are installed
Expected: Cluster created; no MetalLB, EnvoyGateway, MetricsServer, CoreDNSTuning, Dashboard, LocalRegistry, CertManager
Why human: Addon installation is a runtime side-effect verified by inspecting cluster state

**6-9. --profile full, gateway, ci, and default behavior**

Same rationale — all require Docker + node image + inspection of cluster state post-creation.

---

_Verified: 2026-03-04T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
