# Phase 32: CLI Reference - Research

**Researched:** 2026-03-04
**Domain:** Starlight/Astro documentation authoring — CLI reference content for kinder commands
**Confidence:** HIGH

## Summary

Phase 32 is a pure documentation-writing phase. No new code, no new libraries, and no configuration changes to `astro.config.mjs` are needed. The sidebar for `cli-reference/` is already wired with three slugs (`profile-comparison`, `json-output`, `troubleshooting`), and all three pages exist as `:::note[Coming soon]` placeholders created in Phase 30. The work is to replace each placeholder with substantive reference content.

The Go source has been read directly, so all facts below — profile addon sets, JSON shapes, exit codes, check names — are verified from the implementation, not inferred. The writing environment is Astro 5.6.1 + Starlight 0.37.6 using `.md` files with the same callout and code-block conventions established in Phase 31.

Each page serves a distinct user intent. `profile-comparison.md` answers "which preset should I pick?". `json-output.md` answers "how do I script against kinder output?". `troubleshooting.md` answers "what does this exit code mean and what do I do about it?". The Phase 31 pattern of Symptom/Cause/Fix should be carried forward for the troubleshooting page. Profile and JSON reference pages use comparison tables + concrete command examples.

**Primary recommendation:** Replace all three placeholder `.md` files in-place. Each file replaces only its `:::note[Coming soon]` placeholder. No new files, no sidebar changes, no package changes.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CLI-01 | User can read a profile comparison guide showing what each --profile preset enables and when to use it | Profile presets verified from `pkg/cluster/createoption.go` — exact addon sets per profile documented below |
| CLI-02 | User can read a JSON output reference with --output json examples and jq filters for all 4 read commands | All 4 JSON-supporting commands verified from source — `get clusters`, `get nodes`, `env`, `doctor`; JSON shapes extracted from struct definitions |
| CLI-03 | User can read a troubleshooting guide for kinder env and kinder doctor with exit codes and solutions | Exit codes verified from `pkg/cmd/kind/doctor/doctor.go`; `kinder env` uses Cobra standard (0/1) |
</phase_requirements>

## Standard Stack

### Core (no changes needed)

| Tool | Version | Purpose | Note |
|------|---------|---------|------|
| Astro | 5.6.1 | Site framework | Already installed |
| @astrojs/starlight | 0.37.6 | Docs theme, callout syntax | Already installed |
| Markdown (.md) | — | Content format | All CLI reference pages are `.md` |

No `npm install` is required. No `astro.config.mjs` changes are needed.

### Content Conventions (established in Phase 31)

| Convention | Pattern | Source |
|------------|---------|--------|
| Callout types | `:::tip`, `:::note`, `:::caution`, `:::danger` | Phase 31 + all existing pages |
| Callout with title | `:::tip[Title here]` | All existing addon pages |
| Shell code blocks | ` ```sh ` | All existing addon pages |
| JSON code blocks | ` ```json ` | Phase 31 research |
| Output blocks | ` ``` ` (bare) | All existing addon pages |
| Section heading depth | `##` top-level, `###` sub | All existing pages |
| Troubleshooting sub-entry | `**Symptom:**, **Cause:**, **Fix:**` | MetalLB, Metrics Server, cert-manager pages |

## Architecture Patterns

### File Layout (no change needed)

```
kinder-site/src/content/docs/cli-reference/
├── profile-comparison.md   # placeholder → full content (CLI-01)
├── json-output.md          # placeholder → full content (CLI-02)
└── troubleshooting.md      # placeholder → full content (CLI-03)
```

### Pattern 1: Profile Comparison Page Structure

```markdown
---
title: Profile Comparison
description: Compare --profile presets...
---

[intro paragraph]

## Presets at a glance

[table with all 4 presets × all 7 addons]

## Preset details

### minimal
[prose + use case]

### full
[prose + use case]

### gateway
[prose + use case]

### ci
[prose + use case]

## How to use a profile

```sh
kinder create cluster --profile minimal
kinder create cluster --profile full
```

## Combining profiles with explicit config
[note about --config overriding profiles]
```

### Pattern 2: JSON Output Reference Page Structure

```markdown
---
title: JSON Output Reference
---

[intro: why --output json, scripting use cases]

## Commands with --output json

### kinder get clusters
[output shape + example output + jq recipes]

### kinder get nodes
[output shape + example output + jq recipes]

### kinder env
[output shape + example output + jq recipes]

### kinder doctor
[output shape + example output + jq recipes]

## Common jq patterns
[3+ recipes that work across commands]
```

### Pattern 3: Troubleshooting Page Structure

```markdown
---
title: Troubleshooting
---

[intro]

## kinder env

### Exit codes
[table: code → meaning]

### Common errors
[Symptom/Cause/Fix entries]

## kinder doctor

### Exit codes
[table: code → meaning]

### Check results
[table: check name → status → meaning → fix]

### Common scenarios
[walkthrough of fail/warn/ok patterns]
```

### Anti-Patterns to Avoid

- **Touching `astro.config.mjs`:** The sidebar is already wired. Do not modify it.
- **Creating new files:** All content goes into the existing 3 `.md` files.
- **Inventing addon behavior:** All profile membership is verified from Go source — use exact values.
- **Claiming kinder env has structured exit codes:** It does not. `kinder env` exits 0 on success and 1 on any error via Cobra. Only `kinder doctor` has the 3-code scheme (0/1/2).
- **Claiming kubeconfig supports --output json:** It does not. Only `get clusters`, `get nodes`, `env`, `doctor` support `--output json`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Callout boxes | Custom HTML | `:::tip/note/caution` | Already in Starlight theme |
| Comparison tables | Custom component | Standard Markdown table | Pattern established in existing pages |
| Troubleshooting entries | Custom component | Bold labels + prose | Pattern from MetalLB/Metrics Server/cert-manager pages |

## Verified Source Data

This section is the primary research output for Phase 32. All facts are read directly from Go source files.

### Profile Presets (from `pkg/cluster/createoption.go`)

| Addon field | minimal | full | gateway | ci |
|-------------|---------|------|---------|-----|
| MetalLB | false | true | true | false |
| EnvoyGateway | false | true | true | false |
| MetricsServer | false | true | false | true |
| CoreDNSTuning | false | true | false | false |
| Dashboard | false | true | false | false |
| LocalRegistry | false | true | false | false |
| CertManager | false | true | false | true |

**Use cases (inferred from composition):**
- `minimal` — bare kind cluster, no addons, smallest possible footprint. Use when you want to manage addons manually or don't need any extras.
- `full` — all 7 addons enabled. Use for demos, exploration, or when you want everything available without picking.
- `gateway` — MetalLB + Envoy Gateway only. Use for Gateway API routing work — real LoadBalancer IPs plus the eg GatewayClass, nothing else.
- `ci` — MetricsServer + CertManager only. Use in CI pipelines where you need HPA scaling and TLS but want minimal install time. No UI addons.

**Error case:** An unknown profile name returns: `unknown profile %q: valid values are minimal, full, gateway, ci`

### Commands Supporting `--output json` (from source files)

**`kinder get clusters`** (`pkg/cmd/kind/get/clusters/clusters.go`)
- Output: JSON array of strings (cluster names), `[]string`
- Empty: emits `[]` (never `null`)
- Example: `["kind","my-cluster"]`

**`kinder get nodes`** (`pkg/cmd/kind/get/nodes/nodes.go`)
- Output: JSON array of objects with `name` (string) and `role` (string) fields
- Role values: "control-plane", "worker", or "unknown" on error
- Example: `[{"name":"kind-control-plane","role":"control-plane"},{"name":"kind-worker","role":"worker"}]`
- Supports `--all-clusters` / `-A` flag alongside `--output json`
- Empty result: emits `[]`

**`kinder env`** (`pkg/cmd/kind/env/env.go`)
- Output: JSON object (single object, not array)
- Fields: `kinderProvider` (string), `clusterName` (string), `kubeconfig` (string)
- Example: `{"kinderProvider":"docker","clusterName":"kind","kubeconfig":"/Users/user/.kube/config"}`

**`kinder doctor`** (`pkg/cmd/kind/doctor/doctor.go`)
- Output: JSON array of check result objects
- Fields per object: `name` (string), `status` (string: "ok"/"warn"/"fail"), `message` (string, omitted if empty/ok)
- Check names: `"docker"` / `"podman"` / `"nerdctl"` (whichever container runtime was found or checked), `"container-runtime"` (if none found), `"kubectl"`
- Example all-ok: `[{"name":"docker","status":"ok"},{"name":"kubectl","status":"ok"}]`
- Example with fail: `[{"name":"container-runtime","status":"fail","message":"no container runtime found — install Docker..."},{"name":"kubectl","status":"ok"}]`

### Exit Codes

**`kinder doctor`** (explicit `os.Exit` calls in source):

| Code | Meaning | Condition |
|------|---------|-----------|
| 0 | All checks passed | All results are "ok" |
| 1 | One or more checks failed | At least one result has status "fail" |
| 2 | One or more checks warned | At least one "warn", no "fail" |

Exit code evaluation: hasFail takes priority over hasWarn. If both are true, exit 1 (not 2).

The same exit code logic applies regardless of `--output json` — JSON output is written first, then `os.Exit` is called.

**`kinder env`** (no explicit `os.Exit`; uses Cobra default):

| Code | Meaning | Condition |
|------|---------|-----------|
| 0 | Success | Provider detected, output written |
| 1 | Error | Unsupported `--output` format, or provider detection returns an unchecked error |

Note: `kinder env` does not fail if the cluster does not exist. It resolves environment variables without contacting the container runtime to check if the cluster is running.

### `kinder doctor` Check Logic

1. Tries `docker`, `podman`, `nerdctl` in that order. Stops at the first found+working binary.
2. If found but `version` / `-v` fails: status "warn" — daemon not responding.
3. If none found: status "fail" with install links for all three.
4. Always checks `kubectl` separately using `kubectl version --client`.
5. `kubectl` found but `version --client` fails: status "warn".

### `kinder env` Behavior

- `KINDER_PROVIDER` is resolved without contacting the container runtime if `KIND_EXPERIMENTAL_PROVIDER` env var is set to a valid value.
- If `KIND_EXPERIMENTAL_PROVIDER` is unset or unrecognized, auto-detection runs via `cluster.DetectNodeProvider()`.
- If auto-detection fails, falls back to `"docker"` with a logged warning (does not error).
- `KUBECONFIG` is the first non-empty entry from the `KUBECONFIG` env var (colon-separated list), or `$HOME/.kube/config` as fallback.
- `KIND_CLUSTER_NAME` is the `--name` flag value (default: `"kind"`).

## Common Pitfalls

### Pitfall 1: Claiming kinder env exits 2 on warnings

**What goes wrong:** Documentation writer assumes kinder env has the same 3-code exit scheme as kinder doctor.
**Why it happens:** kinder doctor is the only command with explicit `os.Exit(1)` / `os.Exit(2)` calls. kinder env uses standard Cobra error returns (exit 0 or 1).
**How to avoid:** Only document exit codes 0 and 1 for kinder env.

### Pitfall 2: Missing the "check name" distinction for container runtime

**What goes wrong:** Documentation says "the container-runtime check" but the actual check name is the runtime name itself (`"docker"`, `"podman"`, `"nerdctl"`) when found.
**Why it happens:** `"container-runtime"` is only used as the check name when no runtime is found at all. When a runtime is found but the daemon is not responding, the name is still the runtime name (e.g., `"docker"`).
**How to avoid:** Use conditional language — "the check name is the runtime that was found, or `container-runtime` if none was found".

### Pitfall 3: Confusing "no output json for kubeconfig"

**What goes wrong:** Documentation lists `kinder get kubeconfig` as a JSON-capable command.
**Why it happens:** `kubeconfig.go` does not import `encoding/json` and has no `--output` flag.
**How to avoid:** Only list the verified 4 commands: `get clusters`, `get nodes`, `env`, `doctor`.

### Pitfall 4: jq filters on single-object vs array

**What goes wrong:** Using `.[] | .name` on `kinder env` output (which is a single object, not an array).
**Why it happens:** `kinder env` emits a single JSON object; `kinder get clusters` and `kinder get nodes` emit arrays.
**How to avoid:** Show distinct jq recipes per command that match the actual output shape.

### Pitfall 5: Profile does not merge with existing config

**What goes wrong:** Documentation implies `--profile` and `--config` can be combined additively.
**Why it happens:** The source shows `--profile` sets `o.Config.Addons` completely — it overwrites the addons struct. If `--config` is also provided, whichever sets `Config` last wins based on option application order.
**How to avoid:** Recommend using either `--profile` or a manual `--config` file, not both, unless the user understands option ordering. Document this as a callout.

## Code Examples

Verified patterns from Go source:

### Profile usage

```sh
# Source: pkg/cluster/createoption.go — CreateWithAddonProfile
kinder create cluster --profile minimal
kinder create cluster --profile full
kinder create cluster --profile gateway
kinder create cluster --profile ci
kinder create cluster --name my-cluster --profile gateway
```

### JSON output — get clusters

```sh
# Source: pkg/cmd/kind/get/clusters/clusters.go
kinder get clusters --output json
```

Example output:
```json
["kind","my-cluster"]
```

jq recipes:
```sh
# Get first cluster name
kinder get clusters --output json | jq '.[0]'

# Count clusters
kinder get clusters --output json | jq 'length'

# Check if a specific cluster exists
kinder get clusters --output json | jq 'any(. == "kind")'
```

### JSON output — get nodes

```sh
# Source: pkg/cmd/kind/get/nodes/nodes.go
kinder get nodes --output json
kinder get nodes --all-clusters --output json
```

Example output:
```json
[{"name":"kind-control-plane","role":"control-plane"},{"name":"kind-worker","role":"worker"}]
```

jq recipes:
```sh
# List control-plane nodes only
kinder get nodes --output json | jq '[.[] | select(.role == "control-plane") | .name]'

# Count worker nodes
kinder get nodes --output json | jq '[.[] | select(.role == "worker")] | length'

# Get all node names as a space-separated list
kinder get nodes --output json | jq -r '.[].name' | tr '\n' ' '
```

### JSON output — env

```sh
# Source: pkg/cmd/kind/env/env.go
kinder env --output json
kinder env --name my-cluster --output json
```

Example output:
```json
{"kinderProvider":"docker","clusterName":"kind","kubeconfig":"/Users/user/.kube/config"}
```

jq recipes:
```sh
# Extract provider only
kinder env --output json | jq -r '.kinderProvider'

# Set KUBECONFIG from json output
export KUBECONFIG=$(kinder env --output json | jq -r '.kubeconfig')

# Use in shell scripting
PROVIDER=$(kinder env --output json | jq -r '.kinderProvider')
```

### JSON output — doctor

```sh
# Source: pkg/cmd/kind/doctor/doctor.go
kinder doctor --output json
```

Example output (all ok):
```json
[{"name":"docker","status":"ok"},{"name":"kubectl","status":"ok"}]
```

Example output (with fail):
```json
[{"name":"container-runtime","status":"fail","message":"no container runtime found — install Docker (https://docs.docker.com/get-docker/), Podman (https://podman.io/getting-started/installation), or nerdctl (https://github.com/containerd/nerdctl)"},{"name":"kubectl","status":"ok"}]
```

Example output (with warn):
```json
[{"name":"docker","status":"warn","message":"docker found but not responding — is the daemon running?"},{"name":"kubectl","status":"ok"}]
```

jq recipes:
```sh
# Check if any check failed
kinder doctor --output json | jq 'any(.[]; .status == "fail")'

# Get all failed checks and their messages
kinder doctor --output json | jq '[.[] | select(.status == "fail") | {name, message}]'

# CI-safe: run doctor and capture exit code
kinder doctor --output json; echo "exit: $?"
```

### kinder doctor human output (non-JSON)

```
[ OK ] docker
[ OK ] kubectl
```

```
[WARN] docker: docker found but not responding — is the daemon running?
[ OK ] kubectl
```

```
[FAIL] container-runtime: no container runtime found — install Docker...
[FAIL] kubectl: kubectl not found — install from https://kubernetes.io/docs/tasks/tools/
```

Note: human-readable output goes to stderr (`streams.ErrOut`); JSON output goes to stdout (`streams.Out`).

### kinder env human output (non-JSON)

```
KINDER_PROVIDER=docker
KIND_CLUSTER_NAME=kind
KUBECONFIG=/Users/user/.kube/config
```

Usage for shell eval:
```sh
eval $(kinder env)
```

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Callout boxes | Custom HTML/CSS | Starlight `:::tip/note/caution` syntax | Already in theme |
| Comparison tables | Custom Astro component | Standard Markdown table | Consistent with existing pages |
| jq filter testing | Reference to external tool | Inline examples | jq is widely available; no project-specific tool exists |

## State of the Art

| Area | Current State | What Changes in Phase 32 |
|------|---------------|--------------------------|
| profile-comparison.md | `:::note[Coming soon]` placeholder | Full profile table + per-preset detail + command examples |
| json-output.md | `:::note[Coming soon]` placeholder | All 4 command shapes + jq recipes |
| troubleshooting.md | `:::note[Coming soon]` placeholder | Exit code tables + doctor check reference + env error cases |

## Validation Architecture

The `workflow.nyquist_validation` key is not present in the planning config, so validation is treated as enabled.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | None (documentation phase — no automated tests) |
| Config file | N/A |
| Quick run command | `cd kinder-site && npm run build 2>&1 | tail -5` |
| Full suite command | `cd kinder-site && npm run build` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CLI-01 | profile-comparison.md contains table of all 4 presets | manual | `grep -c "minimal\|full\|gateway\|ci" kinder-site/src/content/docs/cli-reference/profile-comparison.md` | ✅ placeholder |
| CLI-02 | json-output.md has examples for all 4 commands | manual | `grep -c "get clusters\|get nodes\|kinder env\|kinder doctor" kinder-site/src/content/docs/cli-reference/json-output.md` | ✅ placeholder |
| CLI-03 | troubleshooting.md has exit code tables | manual | `grep -c "exit code\|Exit code\|os.Exit\|exit 0\|exit 1\|exit 2" kinder-site/src/content/docs/cli-reference/troubleshooting.md` | ✅ placeholder |

The build check (`npm run build`) is the primary automated gate — it catches frontmatter errors, broken internal links, and Starlight callout syntax errors.

### Sampling Rate

- **Per task commit:** `cd /Users/patrykattc/work/git/kinder/kinder-site && npm run build 2>&1 | tail -10`
- **Per wave merge:** `cd /Users/patrykattc/work/git/kinder/kinder-site && npm run build`
- **Phase gate:** Full build green before `/gsd:verify-work`

### Wave 0 Gaps

None — existing test infrastructure (Astro build) covers all phase requirements. No new test files needed.

## Open Questions

1. **Dashboard addon in profiles**
   - What we know: `full` profile enables `Dashboard: true`. The `Dashboard` addon refers to Headlamp (based on prior phases — the "dashboard" addons field is Headlamp).
   - What's unclear: The Go struct field is `Dashboard` but the addon page and user-facing config field may differ. The `configuration.md` should be checked to confirm the field name shown to users.
   - Recommendation: Cross-check `kinder-site/src/content/docs/configuration.md` before writing. Use whichever name appears in the config reference.

2. **Whether --profile + --config interaction is tested**
   - What we know: Source shows `--profile` sets `o.Config.Addons`, and `--config` sets `o.Config` via `internalencoding.Load`. If both are provided, the last option applied wins.
   - What's unclear: Option application order in Cobra (flag parsing order).
   - Recommendation: Note in a `:::caution` callout that using both `--profile` and `--config` is unsupported and results may be unpredictable. Recommend using one or the other.

## Sources

### Primary (HIGH confidence)

- `pkg/cluster/createoption.go` — Profile preset definitions, exact addon membership, error message text
- `pkg/cmd/kind/get/clusters/clusters.go` — JSON output shape for get clusters
- `pkg/cmd/kind/get/nodes/nodes.go` — JSON output shape for get nodes, nodeInfo struct
- `pkg/cmd/kind/env/env.go` — JSON output shape for env, exit behavior (no os.Exit)
- `pkg/cmd/kind/doctor/doctor.go` — JSON output shape for doctor, exit code logic (os.Exit 0/1/2), check names, check logic, message strings
- `kinder-site/astro.config.mjs` — Sidebar wiring (CLI Reference group, 3 slugs, already collapsed)
- `kinder-site/src/content/docs/cli-reference/*.md` — All 3 placeholder files confirmed
- `kinder-site/package.json` — Astro 5.6.1, Starlight 0.37.6

### Secondary (MEDIUM confidence)

- Phase 31 RESEARCH.md — Content conventions, callout syntax, troubleshooting pattern (Symptom/Cause/Fix)
- Phase 30 SUMMARY.md — Placeholder page creation, sidebar collapse decision
- Existing addon `.md` pages (metallb, metrics-server, cert-manager) — Troubleshooting section pattern

### Tertiary (LOW confidence)

- None — all critical facts verified from source code.

## Metadata

**Confidence breakdown:**
- Profile preset membership: HIGH — read directly from `createoption.go` switch statement
- JSON output shapes: HIGH — read directly from struct definitions and `json.NewEncoder` calls
- Exit codes: HIGH — read directly from `os.Exit()` calls and command Long description
- Content authoring conventions: HIGH — read from existing pages and confirmed in Phase 31 RESEARCH.md
- jq filter recipes: MEDIUM — jq syntax is stable but recipes were composed, not copy-pasted from an authoritative source

**Research date:** 2026-03-04
**Valid until:** 2026-04-04 (documentation phase; Go source is stable; jq syntax is stable)
