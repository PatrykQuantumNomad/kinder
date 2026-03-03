# Project Research Summary

**Project:** kinder — v1.3 Harden & Extend Milestone
**Domain:** Go CLI Kubernetes local development tool (kind fork)
**Researched:** 2026-03-03
**Confidence:** HIGH overall; MEDIUM for cert-manager version selection and Podman rootless registry networking

## Executive Summary

Kinder v1.3 is a hardening and extension milestone for a Go CLI tool that wraps kind (Kubernetes IN Docker). The milestone adds four features — local registry addon, cert-manager addon, `kinder env` command, `kinder doctor` command — plus provider code deduplication and four bug fixes. Critically, zero new external Go dependencies are required: all work builds on the standard library, existing go.mod packages, and manifest embedding via `//go:embed`. The only mandatory go.mod change is bumping the minimum Go version from 1.17 to 1.21.0 (and adding `toolchain go1.26.0`), which is a paperwork alignment with the toolchain already in use — not a toolchain change.

The recommended build order is strictly sequenced by compile-time dependencies: bug fixes first (correctness, isolated, must be fixed before deduplication so bugs don't propagate into shared code), then provider deduplication (which de-risks the registry addon by making the container runtime binary uniformly accessible), then config type additions (required before action packages can reference `cfg.Addons.*`), then the two new addons (registry before cert-manager per installation ordering), and finally the two CLI commands. This order also respects behavioral safety: per-provider tests must pass before and after each deduplication extraction step.

The primary implementation risks are in provider deduplication (the 70-80% code similarity across Docker/Podman/nerdctl providers masks critical 20-30% behavioral differences in volume semantics, port formatting, and network env vars that must not be merged) and in the local registry addon (the `localhost` networking boundary between host and kind node containers requires the registry to join the `kind` Docker/nerdctl/podman network and be referenced by container name). cert-manager's webhook bootstrapping window (30-90 seconds) is a timing risk that requires explicit readiness waits for all three cert-manager components — not just the controller deployment.

---

## Key Findings

### Recommended Stack

No new external Go dependencies are needed. The entire milestone reuses `exec.Command` patterns for container runtime interaction, `//go:embed` for manifest embedding, Cobra for the two new commands, and the existing `Provider` interface and `ActionContext` for addon wiring. The `registry:2` (Docker Distribution v2.8.x) image is the canonical local registry choice — `registry:3` (v3.0.0, released April 2025) deprecated storage drivers and is not yet adopted by the kind ecosystem. cert-manager v1.17.6 is the recommended pin: it covers k8s 1.28–1.31 and overlaps the widest range of default kind node images without risking compatibility breaks from the newer v1.19.x series.

**Core technologies:**
- `registry:2` (Docker Distribution v2.8.x): local registry container image — 25MB, zero config, universal kind ecosystem adoption; do not use v3
- cert-manager v1.17.6: TLS certificate management — best k8s version compatibility range for default kind node images; re-evaluate when project pins to k8s 1.31+ node images
- Go 1.21.0 minimum directive + `toolchain go1.26.0`: unlocks `slices`/`maps` packages and auto-seeded global rand; aligns go.mod with actual build toolchain
- `//go:embed` + `node.Command("kubectl", "apply", "-f", "-")`: manifest embedding and apply — established pattern for all five existing addons; no new mechanism needed
- Cobra: CLI framework — already in go.mod; `env` and `doctor` follow the existing `get/` and `create/` command patterns exactly

**Explicit exclusions (do not add):**
- `helm/helm` Go library — cert-manager uses static embedded manifest; Helm is not needed
- `k8s.io/client-go` — all kubectl operations use `node.Command("kubectl", ...)` inside node containers; this is intentional architecture
- `github.com/docker/docker` SDK — container runtimes are invoked as external binaries; must not change
- `viper` — `kinder env` output is simple key=value; Viper would add an unwanted dependency

### Expected Features

**Must have (table stakes):**
- Local registry at `localhost:5001` with containerd config patched on every node — every "kind + local dev" tutorial uses this; absence makes v1.3 feel incomplete
- `kube-public/local-registry-hosting` ConfigMap — Tilt, Skaffold, and other dev tools look for this to auto-configure; omitting it breaks third-party integrations
- cert-manager CRDs and webhook fully ready before cluster is reported done — webhook-not-ready errors are silent and produce confusing admission rejections
- Self-signed `ClusterIssuer` bootstrapped post-install — cert-manager alone is useless without an issuer; `kubectl apply -f my-certificate.yaml` must work immediately
- `kinder env` shows provider, cluster name, config path in machine-readable key=value format — primary debugging artifact for developers
- `kinder doctor` checks binary prerequisites with actionable exit codes — failure messages must name the fix, not just the symptom
- Provider code deduplication without behavior change — prerequisite for registry addon's uniform binary access; also eliminates 3-way drift risk going forward
- Four bug fixes (defer-in-loop port leak, tar extraction truncation, ListInternalNodes default name, network sort comparator) — correctness issues, not optional; each must be fixed in all three providers simultaneously

**Should have (differentiators):**
- `kinder env` shows enabled/disabled addon state — "did cert-manager actually install?" is the first debugging question
- `kinder doctor` checks resource minimums (4GB RAM, 10GB disk) with platform-specific syscalls
- `kinder doctor` exit code distinguishes warnings from errors (exit codes 0/1/2)
- `kinder env` `--shell` flag for fish shell compatibility (fish uses `set -x VAR value`, not `export`)
- Explicit provider detection in `kinder doctor` before any provider-specific check — a Podman-only system must not error with "docker: command not found"

**Defer to v1.4+:**
- Pull-through cache (Docker Hub mirror) for local registry — significant added complexity
- cert-manager trust-manager addon — edge case for local dev
- `kinder doctor --fix` auto-remediation — side effects without user consent; document fix instructions instead
- ACME/Let's Encrypt issuer — requires internet reachability; incompatible with offline local clusters
- Registry UI — `curl localhost:5001/v2/_catalog` is sufficient

### Architecture Integration Points

The existing addon pipeline is clean and well-established: `create.go` calls `runAddon(name, enabled, action)` in sequence; each addon action embeds its YAML, applies it via `node.Command("kubectl", "apply", "-f", "-")`, and waits for deployment readiness. New addons plug into this pipeline with no changes to the pipeline itself. Provider deduplication introduces `common/node.go` and `common/provision.go` that replace per-provider duplicated files; the `Provider` interface remains unchanged throughout.

**New files:**

| File | Purpose |
|------|---------|
| `pkg/cluster/internal/providers/common/node.go` | Shared `Node` struct with `binaryName string`; replaces docker/nerdctl/podman `node.go` |
| `pkg/cluster/internal/providers/common/provision.go` | Shared `generateMountBindings`, `generatePortMappings`, `createContainer`; replaces docker/nerdctl `provision.go` |
| `pkg/cluster/internal/create/actions/installlocalregistry/localregistry.go` | Local registry addon action |
| `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go` | cert-manager addon action + embedded manifest |
| `pkg/cmd/kind/env/env.go` | `kinder env` Cobra command |
| `pkg/cmd/kind/doctor/doctor.go` | `kinder doctor` Cobra command |

**Modified files (key changes):**
- `pkg/apis/config/v1alpha4/types.go` — add `LocalRegistry *bool`, `CertManager *bool` to `Addons` struct
- `pkg/internal/apis/config/types.go` + `convert_v1alpha4.go` + `default.go` — wire new fields through 5-location config pipeline
- `pkg/cluster/internal/create/create.go` — add `runAddon` calls for LocalRegistry (before CertManager) and CertManager
- `pkg/cmd/kind/root.go` — register `env` and `doctor` commands
- All three `provider.go` files — add/use `binaryName string` field, delegate to `common.Node` in `node()` factory

**Deleted files:** `docker/node.go`, `nerdctl/node.go`, `podman/node.go`, `docker/provision.go`, `nerdctl/provision.go` (podman/provision.go is modified, not deleted — its `runArgsForNode` has unique anonymous volume creation)

### Critical Pitfalls

1. **Provider semantics masquerading as duplication** — The 70-80% code overlap hides critical divergence: Podman pre-creates named volumes with `suid,exec,dev` mount options (Docker uses inline anonymous volumes), strips `:0` from port mappings (Docker does not), and uses a different network env var. Merging these into shared code breaks Podman clusters silently at runtime, not at compile time. Prevention: define a `PortFormatter` function parameter for port mapping generation; keep `runArgsForNode` per-provider; run all three providers' test suites after each extraction step.

2. **Registry localhost networking boundary** — Inside kind node containers, `localhost` is the container's loopback, not the host. A registry on `host:5001` is unreachable from within nodes. Prevention: create registry container with `--network kind`; reference it by container name (`kind-registry:5000`) in containerd's `hosts.toml`; restart containerd inside each node after patching the config directory; verify with `curl http://kind-registry:5000/v2/` from inside a node.

3. **cert-manager webhook bootstrapping window** — cert-manager has a 30-90 second window after pod start where the webhook is running but its own TLS certificate is not yet provisioned. Any cert-manager API request during this window returns a webhook timeout error. Prevention: wait for ALL THREE components (`cert-manager`, `cert-manager-webhook`, `cert-manager-cainjector` deployments) to reach `Available=True`; the webhook wait is the binding constraint and must not be skipped.

4. **defer-in-loop port leak** — All three providers have `defer releaseHostPortFn()` inside a `for` loop in `generatePortMappings`. Go defers run at function return, causing port listeners to accumulate across loop iterations and leak on early return. This bug must be fixed before provider deduplication begins, or the shared function inherits it. Prevention: wrap each port acquisition in an IIFE so `defer` runs per-iteration; run `go vet ./pkg/cluster/internal/providers/...` to confirm.

5. **Missing defaultName() normalization in new commands** — New CLI commands accepting an optional `--name` flag must normalize an empty name to `kind.DefaultClusterName` as the very first step after flag parsing, before any provider interaction. Omitting this makes `kinder env` and `kinder doctor` fail with "cluster not found" when no `--name` is passed even when the default cluster exists. Prevention: apply normalization unconditionally; add an integration test for the no-flag case.

---

## Implications for Roadmap

The research is unanimous on build order. Dependencies flow in one direction: bugs must be clean before refactoring; refactoring enables registry addon; config types must exist before action code compiles; addons install in network-dependency order; CLI commands are independent. There is no research gap that requires re-ordering.

### Phase 1: Bug Fixes

**Rationale:** Four correctness bugs exist that are independent of all new features, required to fix before deduplication (bugs must not propagate into shared code), and blocking quality for production use. Fix atomically, each bug in all three providers simultaneously.
**Delivers:** Correct port release semantics (defer-in-loop fixed), safe tar extraction (non-EOF treated as error), reliable cluster name resolution (defaultName() called consistently), deterministic network selection (strict weak ordering comparator).
**Addresses:** C1 (defer-in-loop), C2 (tar truncation), C3 (ListInternalNodes default name), C4 (network sort comparator).
**Must avoid:** Fixing only one provider per bug (C17) — grep for each pattern across docker, podman, nerdctl and fix all three; add tests for all three.
**Research flag:** SKIP — all four bugs are fully documented with exact fix patterns in PITFALLS.md. No ambiguity.

### Phase 2: Provider Code Deduplication

**Rationale:** Must come after bug fixes (bugs must not be extracted into shared code) and before registry addon (registry needs uniform `binaryName` access). Run all three providers' existing test suites before starting any extraction; commit after each individual function extraction.
**Delivers:** `common/node.go` (shared `Node` struct with `binaryName`), `common/provision.go` (shared `generateMountBindings`, `generatePortMappings`, `createContainer`); deletion of 5 per-provider duplicated files; uniform `binaryName` field on all three provider structs.
**Must avoid:** Merging Podman port formatter into shared path (C13), removing Podman anonymous volume pre-creation (C14), unifying provider-specific commonArgs flags into shared code (C5).
**Constraint:** Do not delete per-provider files until `common/node.go` satisfies `nodes.Node` interface and the full build passes; run `go build ./...` and `go test ./...` after each extraction step before proceeding.
**Research flag:** May benefit from a planning spike on `ProviderBehavior` interface design (how to parameterize divergent port/volume/network behavior). ARCHITECTURE.md gives two options (interface vs callback); choose before writing shared code.

### Phase 3: Config Type Additions

**Rationale:** Both new addon actions reference `cfg.Addons.LocalRegistry` and `cfg.Addons.CertManager` — these fields must compile before any action package can be written. Config changes are low-risk and self-contained. A separate phase also produces a clear, reviewable diff.
**Delivers:** `LocalRegistry *bool` and `CertManager *bool` in v1alpha4 public API; corresponding `bool` fields in internal config; conversion and defaults wired through all five config locations.
**Addresses:** v1alpha4/types.go, internal/config/types.go, convert_v1alpha4.go, default.go, create.go.
**Research flag:** SKIP — identical to how all five existing addon flags were added. Pure mechanical work.

### Phase 4: Local Registry Addon

**Rationale:** Registry addon comes before cert-manager because it is simpler (no CRD bootstrapping complexity), establishes the containerd config patching pattern, and the registry container runs on the kind network as a prerequisite that cert-manager could optionally use in the future.
**Delivers:** `addons.localRegistry: true` (default: enabled) — `registry:2` container on kind network, containerd config patched on all nodes via `/etc/containerd/certs.d/localhost:5001/hosts.toml`, `kube-public/local-registry-hosting` ConfigMap applied, containerd restarted on each node post-patch.
**Must avoid:** localhost networking confusion (C6), containerd config not surviving cluster restart (C7 — use certs.d directory-based config, not config.toml patching), host-side insecure registry configuration divergence across providers (C8 — document per-provider host config; do not attempt to automate daemon restarts).
**Research flag:** Verify that `--network kind` and container name DNS resolution work identically in rootless Podman before committing to the registry-on-kind-network implementation. If Podman rootless is a first-class target, this needs validation.

### Phase 5: cert-manager Addon

**Rationale:** Requires a running cluster (waitforready must complete first); ordered after registry to maintain consistent network-dependency sequence (registry sets up the container runtime environment that cert-manager's webhook implicitly relies on being stable). cert-manager's webhook bootstrapping complexity is isolated in this phase.
**Delivers:** `addons.certManager: true` — embedded v1.17.6 manifest applied, all three cert-manager components waited for `Available` status, self-signed `ClusterIssuer` bootstrapped so `Certificate` resources work immediately after cluster creation.
**Must avoid:** CRD not established before controller starts (C9 — the single-file cert-manager.yaml applies CRDs and deployments in order; wait for `crd/certificates.cert-manager.io` established before proceeding), Envoy Gateway or other addons creating cert-manager resources before webhook ready (C10), double installation if cert-manager is already present (C11 — check for existing CRDs before applying).
**Design decision required:** Should `certManager` default to `true` or `false`? It adds ~50MB embedded manifest and 30-90s of webhook readiness wait. STACK.md explicitly flags this as a design decision. FEATURES.md lists cert-manager as a differentiator. **Recommend defaulting to `false` (opt-in)** to preserve fast cluster creation for the majority of users who don't need TLS management. Confirm before implementation.
**Research flag:** SKIP implementation pattern — identical to MetalLB/Dashboard addon pattern. FLAG the true/false default decision as a product call before the phase begins.

### Phase 6: CLI Commands (env + doctor)

**Rationale:** Both commands are read-only, low-risk, and follow established Cobra patterns. Placed last to avoid scope creep during the more complex refactoring work. Independent of the addon pipeline — could be built in parallel with Phases 3-5 if desired.
**Delivers:** `kinder env` (prints KUBECONFIG path, cluster name, active provider, addon states; machine-readable stdout, warnings to stderr only) and `kinder doctor` (checks binary availability, daemon running, resource minimums; exit codes 0=ok, 1=fail, 2=warn).
**Must avoid:** Provider assumption in doctor (C15 — detect active provider first with `DetectNodeProvider()`; gate all provider-specific checks on the detected provider), human text mixed with machine output in env (C16 — stdout must be eval-safe; `eval $(kinder env)` must succeed in bash), missing defaultName() normalization (C3 — normalize cluster name before any provider call).
**Research flag:** SKIP — Cobra command structure follows `pkg/cmd/kind/get/get.go` exactly. No new infrastructure. Only design decision is the field set for `kinder env` output (provider, name, config path, addon states are confirmed; exact format is a minor decision).

### Phase Ordering Rationale

- **Bugs before deduplication:** Buggy shared code is harder to trace and fix than per-file bugs. Fixing first ensures the extracted shared code is correct by construction.
- **Deduplication before registry:** Registry needs uniform provider binary access. Doing it after would force the registry addon to implement its own binary detection — duplicating what deduplication is trying to eliminate.
- **Config before actions:** Go compile-time dependency. Action packages reference `cfg.Addons.LocalRegistry`; that field must exist before the package compiles.
- **Registry before cert-manager:** Simpler addon establishes the `certs.d` patching pattern. cert-manager's CRD bootstrapping complexity is isolated in its own phase.
- **CLI commands last:** Read-only, independent, low-risk. Doing them last keeps the team focused on the riskier refactoring work first.

### Research Flags

Phases that may benefit from deeper research during planning:
- **Phase 2 (Provider Deduplication):** The `ProviderBehavior` interface design (how to parameterize port formatting, volume args, and network env vars) should be decided before writing any shared code. ARCHITECTURE.md provides two patterns; pick one in planning.
- **Phase 4 (Local Registry — Podman rootless):** If Podman rootless is a first-class target, verify the `--network kind` + container name DNS pattern works in rootless mode before committing to the implementation approach.
- **Phase 5 (cert-manager default):** Confirm true/false default before Phase 5 begins. This is a product decision, not a technical one.

Phases with standard patterns (skip research-phase during planning):
- **Phase 1 (Bug Fixes):** Exact fix patterns documented in PITFALLS.md with code examples.
- **Phase 3 (Config Types):** Mechanical five-location change, identical to existing addon additions.
- **Phase 6 (CLI Commands):** Follows `pkg/cmd/kind/get/get.go` pattern with no new mechanisms.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Zero new dependencies confirmed by direct codebase analysis; registry:2 and cert-manager versions verified against official release pages; go.mod change is paperwork only |
| Features | HIGH | Based on official kind docs, official cert-manager docs, and direct codebase analysis; MVP scope is clearly defined with explicit out-of-scope items |
| Architecture | HIGH | All analysis from direct source file reads of all three provider packages, all existing addon actions, and config pipeline; no inference; build order validated against Go compile-time dependency graph |
| Pitfalls | HIGH | Bugs confirmed with file:line citations in actual source code; provider behavioral differences confirmed by reading all three provider files side-by-side; external pitfalls sourced to official docs and confirmed GitHub issues |

**Overall confidence:** HIGH

### Gaps to Address

- **cert-manager version pin:** v1.17.6 is "MEDIUM confidence" because the exact default kind node image k8s version should be verified before shipping. If kinder ships targeting k8s 1.32+ node images, upgrade to cert-manager v1.19.x (which covers k8s 1.31–1.35).
- **Podman rootless registry networking:** The registry-on-kind-network pattern is documented and confirmed for Docker. Podman rootless uses pasta/slirp4netns and has different DNS resolution behavior. Verify `--network kind` + container name resolution before Phase 4 implementation if Podman rootless support is required.
- **cert-manager default (true vs false):** Research identifies this as a design decision. Recommend `false` (opt-in) to keep cluster creation fast for users who don't need TLS, but this requires explicit confirmation before Phase 5 begins.
- **`kinder env` addon state persistence:** Showing "which addons are enabled" requires reading the config used at cluster creation time. The mechanism for persisting this (cluster label, config snapshot file, or on-demand node inspection) is not decided in the research. Design this before Phase 6 begins.

---

## Sources

### Primary (HIGH confidence)
- kind local registry guide: https://kind.sigs.k8s.io/docs/user/local-registry/ — canonical registry-on-kind-network pattern and containerd config approach
- cert-manager installation docs: https://cert-manager.io/docs/installation/kubectl/ — single-manifest install approach
- cert-manager supported releases + k8s compatibility: https://cert-manager.io/docs/releases/ — version compatibility matrix used for v1.17.6 selection
- cert-manager kind development guide: https://cert-manager.io/docs/contributing/kind/ — webhook bootstrapping timing issue
- containerd registry config docs: https://github.com/containerd/containerd/blob/main/docs/cri/registry.md — certs.d directory-based config for containerd v2
- Go toolchain directive semantics: https://go.dev/doc/toolchain — go directive vs toolchain directive distinction
- Go 1.21 release notes: https://tip.golang.org/doc/go1.21 — mandatory minimum enforcement change
- Go archive/tar package: https://pkg.go.dev/archive/tar — `io.EOF` return semantics
- Direct kinder codebase analysis: all provider, action, config, and command files — behavioral differences confirmed line-by-line

### Secondary (MEDIUM confidence)
- Docker Hub registry tags (registry:2 = v2.8.x, registry:3 = v3.0.0): https://hub.docker.com/_/registry/tags — confirmed via search; page CSS-blocked on direct fetch
- cert-manager v1.17.6 release date (2025-12-17): cert-manager release index search result
- Envoy Gateway TLS + cert-manager ordering: https://gateway.envoyproxy.io/docs/tasks/security/tls-cert-manager/ — dependency ordering between addons
- kind issue on containerd restart race: https://github.com/kubernetes-sigs/kind/issues/2262 — confirms timing issue with containerd restart after config patching
- Go defer-in-loop documentation: https://www.jetbrains.com/help/inspectopedia/GoDeferInLoop.html — documents accumulation behavior

---
*Research completed: 2026-03-03*
*Ready for roadmap: yes*
