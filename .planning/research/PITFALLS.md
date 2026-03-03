# Domain Pitfalls: Kinder v1.3

**Domain:** Go Kubernetes tool — adding local registry addon, cert-manager addon, CLI diagnostic commands, and provider code deduplication to an existing kind fork
**Researched:** 2026-03-03
**Confidence:** HIGH (sourced from actual kinder codebase review, official kind/containerd/cert-manager docs, verified community issues)

---

## Critical Pitfalls

Mistakes that cause cluster breakage, data loss, or require rewrites.

---

### Pitfall C1: defer-in-Loop Port Release — Ports Held Until Function Returns

**What goes wrong:**
The `generatePortMappings` function in all three provider packages (docker, podman, nerdctl) calls `defer releaseHostPortFn()` inside a `for` loop over `portMappings`. Go's defer stack executes at function return, not at loop iteration end. The dummy TCP listener that holds the free port open is not closed until the entire function returns — after all ports have already been reserved. The result: if you have two port mappings that both request random ports (HostPort=0), the first port is still held when the second port probe runs, preventing collision. This is accidentally correct. However, if the cluster creation fails partway through and the function returns early, none of the port-holding listeners are released before the function exits — they leak until garbage collected. On high-node-count clusters with many port mappings, this manifests as transient "bind: address already in use" errors on the next creation attempt.

**Why it happens:**
The pattern `if releaseHostPortFn != nil { defer releaseHostPortFn() }` appears identically in all three providers (docker/provision.go:397, podman/provision.go:406, nerdctl/provision.go:363). This is copy-paste drift — the original code likely worked accidentally because it was written for a single port mapping. It was copied without understanding that defers accumulate on the stack.

**Actual code (identical in docker, podman, nerdctl):**
```go
hostPort, releaseHostPortFn, err := common.PortOrGetFreePort(pm.HostPort, pm.ListenAddress)
if err != nil {
    return nil, errors.Wrap(err, "failed to get random host port for port mapping")
}
if releaseHostPortFn != nil {
    defer releaseHostPortFn()  // BUG: runs at function return, not iteration end
}
```

**Consequences:**
- Port listeners accumulate in memory across all iterations before release
- On error return, listeners may be GC'd non-deterministically, causing race with the container runtime attempting to bind those ports
- When provider deduplication extracts this into shared code, the bug must be fixed before extraction — otherwise the shared function inherits it and it becomes harder to trace

**Prevention:**
Wrap the port acquisition and use in an immediately-invoked function literal:

```go
if err := func() error {
    hostPort, releaseHostPortFn, err := common.PortOrGetFreePort(pm.HostPort, pm.ListenAddress)
    if err != nil {
        return errors.Wrap(err, "failed to get random host port for port mapping")
    }
    if releaseHostPortFn != nil {
        defer releaseHostPortFn()  // now defers to the inner function, runs per-iteration
    }
    // ... use hostPort ...
    return nil
}(); err != nil {
    return nil, err
}
```

**Detection:**
- `go vet` with `loopclosure` linter
- `staticcheck` SA9003 / SA2001 warnings
- Manual grep: `defer.*releaseHostPort` inside any `for` loop

**Phase to address:** Phase 1 (bug fixes) — must be fixed in all three providers BEFORE provider deduplication begins; fixing it after extraction means the fix propagates automatically but finding it is harder

---

### Pitfall C2: tar Extraction Treating io.EOF as Success — Silent Data Truncation

**What goes wrong:**
In Go's `archive/tar` package, `tr.Next()` returns `io.EOF` at the end of a well-formed archive. Any other error from `tr.Next()` signals a malformed or truncated archive. The pattern `if err == io.EOF { break }` is correct. The silent failure bug is when code uses `if err != nil { break }` or doesn't check `err` from the reader at all — both treat a mid-archive truncation identically to a clean end-of-archive. The extracted output is silently incomplete.

**Why it matters here:**
The kinder CONCERNS.md notes "Entire tar archives processed sequentially and synchronously" (pkg/cluster/nodeutils/util.go line 67). Any tar extraction code that treats a non-EOF error as a break condition (rather than returning the error) will silently extract a partial tarball when the underlying stream is interrupted — for example, when extracting an image layer during cluster creation over a slow or dropped connection.

**Correct pattern:**
```go
for {
    header, err := tr.Next()
    if err == io.EOF {
        break  // clean end of archive
    }
    if err != nil {
        return fmt.Errorf("tar extraction error: %w", err)  // must NOT break silently
    }
    // process header...
}
```

**Consequences:**
- Partial image extraction produces containers that appear to start but fail at runtime with missing binaries or configs
- The failure is non-deterministic: only triggered when the source stream truncates
- Extremely hard to debug because the extracted filesystem looks superficially correct

**Prevention:**
- Audit every `archive/tar` usage site with `grep -n "tr.Next\|tar.Next" --include="*.go" -r .`
- Enforce the pattern: only `io.EOF` is a break condition; all other errors are returned
- Add a test that feeds a truncated tar stream and verifies an error is returned (not nil)

**Detection:**
- `errcheck` linter flags ignored errors from `tr.Next()`
- Manual code review of every tar read loop

**Phase to address:** Phase 1 (bug fixes) — before any new addon that ships files via tar extraction

---

### Pitfall C3: ListInternalNodes Missing defaultName() Call — Wrong Cluster Targeted

**What goes wrong:**
If `ListInternalNodes` (or the equivalent cluster lookup) does not call `defaultName()` to resolve an empty cluster name to the configured default, commands with no explicit `--name` flag silently target the wrong cluster or fail with a confusing "no cluster found" error. The user experience is: `kinder env` with no flags fails when multiple clusters exist, even though one is named "kind" (the default).

**Why it happens:**
The pattern in the existing codebase uses `defaultName()` to coerce an empty string to the configured default cluster name before any lookup. When new code paths are added (like `kinder env` or `kinder doctor`) that call internal node listing functions directly, missing the `defaultName()` normalization produces commands that only work when the cluster name is explicitly passed.

**Consequences:**
- `kinder env` and `kinder doctor` appear broken for the default cluster case
- Confusing error: "cluster not found" when the cluster clearly exists
- Intermittent: works when user passes `--name kind`, fails without it

**Prevention:**
In every new CLI command that accepts an optional `--name` flag:
```go
clusterName = provider.ByName(clusterName).defaultName()
```
Or use the existing helper pattern found in other commands:
```go
if clusterName == "" {
    clusterName = kind.DefaultClusterName
}
```
Apply the normalization as the very first step after flag parsing, before any provider interaction.

**Detection:**
- Integration test: run `kinder env` without `--name` on a cluster named "kind" — must succeed
- Unit test: assert that the name resolver is called before any provider lookup

**Phase to address:** Phase 3 (kinder env/doctor CLI commands) — build the normalization in from the start; do not assume the caller will always pass a name

---

### Pitfall C4: Network Sort Comparator — Incorrect Stable Sort Ordering

**What goes wrong:**
The `sortNetworkInspectEntries` function in docker/network.go uses a comparator that has a logical error. The intent is "networks with more containers are preferred (sorted first), with ID as tiebreaker." The current implementation:

```go
sort.Slice(networks, func(i, j int) bool {
    if len(networks[i].Containers) > len(networks[j].Containers) {
        return true
    }
    return networks[i].ID < networks[j].ID
})
```

This is not a strict weak ordering when `len(networks[i].Containers) < len(networks[j].Containers)` — the function returns `false` but then falls through to the ID comparison, which may return `true`. This means that for two networks where `i` has fewer containers than `j`, the comparator may incorrectly say `i < j` based on ID. The sort result is non-deterministic across Go versions (sort.Slice does not guarantee stability; the behavior depends on the internal sort algorithm's comparison calls).

**Correct comparator:**
```go
sort.Slice(networks, func(i, j int) bool {
    ci, cj := len(networks[i].Containers), len(networks[j].Containers)
    if ci != cj {
        return ci > cj  // more containers = higher priority (sort first)
    }
    return networks[i].ID < networks[j].ID  // stable tiebreaker
})
```

**Consequences:**
- Duplicate network cleanup may delete the wrong network (the one with active containers)
- Cluster nodes lose connectivity after the wrong network is deleted
- Non-deterministic: may work correctly 90% of the time and silently fail the rest

**Prevention:**
Always write sort comparators as strict weak orderings: for every pair (i, j), exactly one of `less(i,j)` or `less(j,i)` is true, or neither (equal). Test with at least 3 elements including ties.

**Detection:**
- Unit test with networks of [0, 1, 2] containers and verify the expected ordering
- `go test -race` on the network sort function with concurrent invocations

**Phase to address:** Phase 1 (bug fixes) — fix before any new code relies on network selection order

---

## Critical Integration Pitfall: Provider Deduplication Masking Provider-Specific Behavior

### Pitfall C5: Shared Code Hiding Divergent Provider Semantics

**What goes wrong:**
The three provider packages (docker, podman, nerdctl) are 70-80% identical by line count, but the ~20-30% that differs is not cosmetic — it encodes fundamental behavioral differences:

| Function | Docker | Podman | Nerdctl |
|----------|--------|--------|---------|
| `commonArgs` | Has `--restart=on-failure:1`, `--cgroupns=private`, `--init=false` | Has `--cgroupns=private`, `-e container=podman` (no init flag) | Has `--restart=on-failure:1`, `--init=false` (no cgroupns) |
| `runArgsForNode` | `--volume /var` (anonymous) | `--volume $varVolume:/var:suid,exec,dev` (named pre-created) | `--volume /var` (anonymous, same as docker) |
| Port format | `host:port/tcp` | Strips trailing `:0` → `host:/protocol` (Podman random port quirk) | `host:port/protocol` (lowercase, same as docker) |
| Image name | Used directly | Sanitized via `sanitizeImage()` | Used directly |
| Network env var | `KIND_EXPERIMENTAL_DOCKER_NETWORK` | `KIND_EXPERIMENTAL_PODMAN_NETWORK` | `KIND_EXPERIMENTAL_DOCKER_NETWORK` |
| Subnet inspection | Docker template format | JSON parsing of Podman v3/v4 structure | Docker template format |
| IP masquerade | Yes: `com.docker.network.bridge.enable_ip_masquerade=true` | N/A | No: comment says "Not supported in nerdctl yet" |

**Why it matters:**
When extracting shared code to a `common` package or a shared helper, it is tempting to unify all of the above. Doing so produces a generic implementation that works for Docker but silently breaks Podman (wrong volume flags, wrong port format) and nerdctl (wrong network options). The failure is not a compile error — it's a runtime failure when users with those runtimes create clusters.

**Prevention strategy:**
1. Extract shared code only for genuinely identical logic (the easy 70%)
2. For divergent logic, define a `ProviderBehavior` interface or strategy struct that each provider implements:

```go
type ProviderBehavior interface {
    ContainerArgs() []string              // provider-specific container flags
    VolumeArgs(volName string) []string   // provider-specific volume mount args
    FormatPortMapping(host, port, proto string) string
    NetworkEnvVar() string
}
```

3. Run all three providers' existing tests after each extraction step — before moving to the next
4. Never merge provider-specific env var names, volume semantics, or port formatting into shared code

**Detection:**
- Each provider package must retain its own `provision_test.go`
- Cross-provider integration test: create a cluster with each provider after deduplication and verify node connectivity
- Review diff of shared code: any reference to "docker", "podman", or "nerdctl" strings in shared code is a red flag

**Phase to address:** Phase 2 (provider deduplication) — define the interface before writing any shared code; do not start with "extract what looks the same"

---

## Critical Pitfall: Local Registry Addon

### Pitfall C6: Registry Container Network Connectivity — localhost Means Different Things

**What goes wrong:**
When a local registry is started on the host at `localhost:5000`, it is accessible from the host but NOT from inside kind node containers. Inside each kind node, `localhost` refers to the container's own loopback interface, not the host. Requests to `localhost:5000` from inside a node will fail with "connection refused."

The official kind local registry workaround requires:
1. The registry container must be on the same Docker network as the kind nodes (network named "kind")
2. The registry must be referenced by its container name (e.g., `kind-registry:5000`), not `localhost:5000`, from inside node containers
3. containerd inside each node must be configured to treat `kind-registry:5000` as a valid mirror host via `containerdConfigPatches`

**For Podman:**
Podman's rootless mode uses a different networking stack (pasta/slirp4netns). Joining a container to the "kind" network requires explicit `--network kind` at creation. The container DNS resolution behavior differs from Docker's embedded DNS.

**For nerdctl:**
nerdctl uses CNI networking, not Docker's networking stack. The "kind" network created by nerdctl uses CNI bridge plugins. Container name DNS resolution works differently and the `nerdctl network connect` equivalent may not work identically.

**Prevention:**
- Registry container must be created with `--network kind` (or equivalent for each provider)
- containerd config patch must be applied to every node after the registry is connected
- After applying the patch, containerd must be restarted inside each node: `systemctl restart containerd`
- The registry hostname used in the patch must match exactly the container name on the kind network

**Detection:**
- After registry setup: `docker exec <node-name> curl http://kind-registry:5000/v2/` should return `{}`
- After containerd restart: `docker exec <node-name> crictl pull kind-registry:5000/test:latest` should succeed

**Phase to address:** Phase 4 (local registry addon)

---

### Pitfall C7: containerd Config Patches Not Surviving Cluster Restart

**What goes wrong:**
The containerd config patch that configures the local registry as an insecure mirror is applied to `/etc/containerd/config.toml` inside each node. When the Docker host reboots, kind node containers restart (due to `--restart=on-failure:1`). However, the `/etc/containerd/config.toml` changes written during addon setup DO survive the restart IF they were written to the container filesystem (not a tmpfs mount).

The pitfall is that containerd itself may be restarted by systemd during node startup without re-reading the patched config if the service unit is mis-configured or if the config was applied at a path not monitored by containerd's config-drop-in system.

A subtler variant: if the registry container stops but the kind cluster nodes continue running, the containerd config still references a hostname (`kind-registry`) that no longer resolves on the kind network. Image pulls fail silently because the mirror is unavailable but containerd does not fallback to the original registry (depending on containerd version and mirror configuration).

**Prevention:**
- Write the registry config to `/etc/containerd/certs.d/<registry-host>/hosts.toml` (containerd v2 directory-based config) rather than patching `config.toml` — the directory-based config is the supported extension mechanism
- Use `containerd v2` config format when the cluster's containerd version supports it
- The registry addon's `Teardown()` method must remove the config drop-in, not just stop the registry container
- Add a health check in `kinder doctor` that verifies registry connectivity from each node

**Detection:**
- After host reboot: push a test image to the registry, then `kubectl run test --image=kind-registry:5000/test:latest` — must succeed without manual intervention
- Verify with: `docker exec <node> cat /etc/containerd/certs.d/kind-registry:5000/hosts.toml`

**Phase to address:** Phase 4 (local registry addon)

---

### Pitfall C8: Insecure Registry Configuration Diverges Between Docker and Podman Hosts

**What goes wrong:**
The registry runs as HTTP (no TLS) for local development. Each container runtime on the HOST needs to be configured to allow insecure communication with the registry:

- **Docker host:** requires `"insecure-registries": ["kind-registry:5000"]` in `/etc/docker/daemon.json` and a Docker daemon restart — OR the registry container must be on the `kind` bridge network (same network as the kind nodes) so Docker's own daemon does not validate TLS
- **Podman host:** uses `registries.conf` and the `[registries.insecure]` stanza, in `/etc/containers/registries.conf`
- **nerdctl host:** inherits containerd's configuration

The pitfall is writing an addon that only handles one of these cases. On a Podman host, the Docker-style insecure registry setup does nothing, and the registry addon appears to install successfully but image pushes from the HOST fail.

**Prevention:**
- The registry addon must detect the active provider and emit provider-appropriate host configuration instructions in its output
- The containerd-inside-node config (for image pulls BY PODS) is the same across all providers — only the host-side push configuration differs
- Document the host-side requirement explicitly; do not attempt to automate it (requires daemon restart and root access)

**Phase to address:** Phase 4 (local registry addon)

---

## Cert-Manager Addon Pitfalls

### Pitfall C9: cert-manager CRDs Must Install Before cert-manager Pods Are Ready

**What goes wrong:**
cert-manager requires its Custom Resource Definitions (CRDs) to be installed before the cert-manager controller, webhook, and cainjector pods can function. If the addon applies the cert-manager Helm chart or manifest without first ensuring CRDs are in `Established` state, the webhook server starts but cannot process `Certificate` objects. Attempts to create a `ClusterIssuer` immediately after manifest application fail with "no kind Certificate is registered."

**Prevention:**
- Apply CRDs first: `kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.x.y/cert-manager.crds.yaml`
- Wait for CRD establishment before applying the main manifest:
  ```bash
  kubectl wait --for condition=established --timeout=60s crd/certificates.cert-manager.io
  ```
- Only then apply cert-manager deployment manifests

**Phase to address:** Phase 5 (cert-manager addon)

---

### Pitfall C10: Envoy Gateway Addon Dependency — cert-manager Must Be Ready Before Gateway Provisions TLS

**What goes wrong:**
The cert-manager addon must be fully ready (all three pods: controller, webhook, cainjector) before the Envoy Gateway addon attempts to create `Certificate` resources. The Envoy Gateway integration uses cert-manager's mutating webhook to inject certificate data. If Envoy Gateway's `Gateway` resource is created before the cert-manager webhook is ready, the Gateway object is created without TLS, and cert-manager's reconciliation does not retroactively fix it — you must delete and recreate the Gateway.

**Specific failure mode:**
cert-manager's webhook requires a valid TLS certificate for itself (its own webhook). During initial cert-manager installation, there is a bootstrapping window (typically 30-90 seconds) where the webhook pod is running but its own certificate is not yet provisioned. Any cert-manager API request during this window returns a webhook timeout error.

**Prevention:**
- The cert-manager addon must wait for ALL THREE components: `deployment/cert-manager`, `deployment/cert-manager-webhook`, `deployment/cert-manager-cainjector`
- Wait with: `kubectl rollout status deployment cert-manager-webhook -n cert-manager --timeout=120s`
- Add an additional check: create a test `Certificate` resource and wait for it to reach `Ready=True` before declaring the addon healthy
- In `kinder doctor`, check cert-manager webhook readiness separately from pod running state

**Phase to address:** Phase 5 (cert-manager addon) — install ordering is critical

---

### Pitfall C11: cert-manager Installed Twice — Duplicate CRD Registration

**What goes wrong:**
If cert-manager is installed as a standalone addon AND also pulled in as a dependency of another addon (e.g., Envoy Gateway's Helm chart may bundle cert-manager), CRDs are applied twice. The second application of CRDs fails if the versions differ. The cert-manager pods then run in a state where the actual CRD version in the cluster does not match what the controller expects.

**Prevention:**
- The cert-manager addon must check for existing cert-manager CRDs before installing: `kubectl get crd certificates.cert-manager.io`
- If cert-manager is already installed, the addon must skip installation and log a warning
- Do not bundle cert-manager inside other addon Helm charts — declare it as a prerequisite instead

**Phase to address:** Phase 5 (cert-manager addon)

---

## Provider Deduplication Pitfalls

### Pitfall C12: Extracting Shared Code Without Tests Running First

**What goes wrong:**
The provider packages (docker, podman, nerdctl) have separate test files. The standard approach to deduplication is: (1) identify identical code, (2) move to shared location, (3) update callers. The pitfall is doing step 2 before verifying all existing tests pass. The existing tests are the only specification of what the provider must do — if they fail after extraction, the refactor broke something.

**Specifically for kinder:**
- `pkg/cluster/internal/providers/docker/provision_test.go` (exists)
- `pkg/cluster/internal/providers/podman/provision_test.go` (exists)
- `pkg/cluster/internal/providers/nerdctl/provision_test.go` (exists)
- `pkg/cluster/internal/providers/podman/images_test.go` (exists)
- `pkg/cluster/internal/providers/nerdctl/network_test.go` (exists)

These tests must all pass on main BEFORE extraction begins. If they don't currently pass, fix them first. If they pass before extraction and fail after, the extraction changed behavior.

**Prevention:**
```bash
# Run before starting any deduplication work
go test ./pkg/cluster/internal/providers/... -v
# Must exit 0; any failure must be resolved before touching shared code
```

Commit after each individual function extraction, not in batch. Use `git bisect` if a test starts failing.

**Phase to address:** Phase 2 (provider deduplication) — run tests first, commit atomically

---

### Pitfall C13: Podman Port Mapping Quirk — Trailing :0 Stripped for Random Ports

**What goes wrong:**
Podman requires that a random port mapping be expressed as `host::container/proto` (empty host port, not `host:0:container/proto`). The Docker and nerdctl providers use `host:0:container/proto` to request a random port. If the shared `generatePortMappings` function is extracted without preserving the Podman-specific stripping of `:0`, Podman clusters will fail to start with a parse error from the Podman CLI.

**Actual divergence in current code:**
```go
// docker/provision.go — does NOT strip :0
args = append(args, fmt.Sprintf("--publish=%s:%d/%s", hostPortBinding, pm.ContainerPort, protocol))

// podman/provision.go — strips trailing :0 for random ports
if strings.HasSuffix(hostPortBinding, ":0") {
    hostPortBinding = strings.TrimSuffix(hostPortBinding, "0")
}
args = append(args, fmt.Sprintf("--publish=%s:%d/%s", hostPortBinding, pm.ContainerPort, strings.ToLower(protocol)))
```

Note also: Podman uses `strings.ToLower(protocol)` but Docker does not. A unified function that omits either behavior is wrong for one of them.

**Prevention:**
This behavior must stay provider-specific. If extracting `generatePortMappings` to shared code, pass a `PortFormatter` function as a parameter:

```go
type PortFormatter func(hostPortBinding string, containerPort int32, protocol string) string
```

Each provider supplies its own formatter. Never merge the Podman `:0` stripping into the Docker or nerdctl path.

**Phase to address:** Phase 2 (provider deduplication)

---

### Pitfall C14: Podman Anonymous Volume Creation vs Docker Anonymous Volume

**What goes wrong:**
In `podman/provision.go`, the `/var` volume is created as a named anonymous volume BEFORE the container is created (`createAnonymousVolume(name)`), then mounted as `--volume $varVolume:/var:suid,exec,dev`. In Docker and nerdctl, the volume is declared inline as `--volume /var` (Docker manages the anonymous volume lifecycle).

The Podman approach uses `suid,exec,dev` mount options which are required for Podman's security model. The Docker approach does not need these because Docker uses different defaults for container capabilities.

If the shared `runArgsForNode` function is extracted and uses Docker's inline `--volume /var` syntax for Podman containers, the Podman nodes will fail to run workloads that require SUID binaries or device access inside `/var`.

**Prevention:**
- Do not extract `runArgsForNode` to shared code — it is the most divergent function
- If extraction is required, use a `NodeVolumeArgs(name string) []string` method on the provider behavior interface
- The Podman provider must always pre-create the volume and clean it up on node deletion

**Phase to address:** Phase 2 (provider deduplication)

---

## CLI Diagnostic Command Pitfalls

### Pitfall C15: kinder env/doctor Commands Assuming All Providers Are Available

**What goes wrong:**
`kinder doctor` checks environment health. A common mistake is implementing checks that assume Docker is available when the user might be running Podman or nerdctl. For example:

- Running `docker info` to check container runtime status — fails on Podman-only systems
- Checking `/var/run/docker.sock` for socket availability — irrelevant for nerdctl
- Hard-coding Docker-specific JSON output parsing in diagnostic output

**Prevention:**
- `kinder doctor` must detect the active provider FIRST (using the existing `DetectNodeProvider()` function in `pkg/cluster/provider.go`)
- All provider-specific checks must be gated on the detected provider
- Checks that apply to all providers (e.g., "can we list clusters?") must use the provider interface, not raw CLI calls

**Detection:**
- Run `kinder doctor` on a machine with only Podman installed — must not error with "docker: command not found"
- Run `kinder doctor` on a machine with only nerdctl — same requirement

**Phase to address:** Phase 3 (kinder env/doctor commands)

---

### Pitfall C16: kinder env Output Format — Machine-Readable vs Human-Readable Conflict

**What goes wrong:**
`kinder env` is typically used in shell scripts via `eval $(kinder env)` to set environment variables. If the output includes human-readable text (progress lines, warnings, header comments) mixed with the `export VAR=value` lines, `eval` will fail with a syntax error.

Conversely, if the output is ONLY machine-readable with no way to get human-readable output, the command is hard to debug interactively.

**Prevention:**
- By default, emit only `export VAR=value` lines — nothing else to stdout
- Warnings go to stderr only
- Add a `--shell` flag (default: auto-detect from `$SHELL`) and a `--no-export` flag for fish shell compatibility (fish uses `set -x VAR value`)
- Add a `--human` or `--verbose` flag that adds comments and headers for interactive use

**Detection:**
- Test: `eval $(kinder env)` in bash must succeed when a cluster exists
- Test: `kinder env` when no cluster exists must exit non-zero and emit a human-readable error to stderr (not stdout)

**Phase to address:** Phase 3 (kinder env/doctor commands)

---

## Bug Fix Regression Pitfalls

### Pitfall C17: Fixing One Provider's Bug Without Fixing All Three

**What goes wrong:**
All four bugs identified (defer-in-loop, tar extraction, ListInternalNodes, network sort) exist because of copy-paste replication across providers. The risk: fix the bug in `docker/provision.go` but forget `podman/provision.go` and `nerdctl/provision.go`. The fixed provider passes tests, but the other two remain broken.

**Prevention:**
- For each bug fix, search for the identical pattern in all three providers: `grep -rn "releaseHostPortFn\|defer.*release" pkg/cluster/internal/providers/`
- Write a test that covers the fix for each provider, not just docker
- Use the checklist: docker fixed? podman fixed? nerdctl fixed? each test added?

**Detection:**
- Run `go vet ./pkg/cluster/internal/providers/...` after each fix
- If the same test exists in all three providers' test files, all three must pass

**Phase to address:** Phase 1 (bug fixes)

---

### Pitfall C18: Bug Fixes That Change Provider External Behavior

**What goes wrong:**
The network sort comparator fix (Pitfall C4) changes which network is selected as the "primary" when duplicates exist. This could break existing clusters if they were relying on the old (incorrect) selection order. Similarly, fixing the defer-in-loop changes the timing of port release, which could theoretically affect port allocation behavior.

**Prevention:**
- For the network sort fix: the new sort produces a STABLE ordering (more containers first, then ID). The old sort was non-deterministic. The fix cannot produce a worse outcome than random selection.
- For the defer-in-loop fix: ports should be released AFTER the container is created, not BEFORE. The fix (IIFE pattern) releases ports at the end of each iteration — still before the next iteration's port probe. This is safe.
- Document the behavioral change in the commit message for each bug fix
- Add a regression test that exercises the fixed scenario

**Phase to address:** Phase 1 (bug fixes)

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Bug fixes (defer-in-loop) | Fix in docker but not podman/nerdctl | Grep-and-fix all three, add tests for all three |
| Bug fixes (tar extraction) | Treat non-EOF as break instead of error | Always `return err` on non-EOF; add truncated tar test |
| Bug fixes (ListInternalNodes) | Forget defaultName() in new commands | Normalize cluster name as first step in every command |
| Bug fixes (network sort) | Non-strict comparator re-introduced | Write sort test with 3-element array including ties |
| Provider deduplication | Merging Podman port stripping into shared path | Use PortFormatter interface; test Podman port format explicitly |
| Provider deduplication | Podman volume pre-creation removed | Keep Podman volume creation in provider-specific code |
| Provider deduplication | Shared commonArgs misses provider-specific flags | Define ProviderBehavior interface; test each provider's common args |
| Provider deduplication | Tests broken after extraction | Run all tests before each extraction step; commit atomically |
| kinder env command | Human text mixed with eval output | stdout = machine-readable only; stderr = human/warnings |
| kinder doctor command | Docker-specific checks on Podman system | Detect provider first; gate all checks on detected provider |
| Local registry addon | Registry unreachable from inside nodes | Registry must be on kind network; containerd must be patched and restarted |
| Local registry addon | Config not surviving cluster restart | Use containerd certs.d directory format; verify after simulated restart |
| Local registry addon | Host-side push fails on non-Docker system | Document provider-specific host config; detect and warn at addon install time |
| cert-manager addon | CRDs not established before controller starts | Wait for CRD establishment; then wait for webhook readiness |
| cert-manager addon | Envoy Gateway creates Gateway before cert-manager webhook ready | Install and health-check cert-manager before Envoy Gateway configuration |
| cert-manager addon | Double installation via dependency chain | Check for existing cert-manager CRDs before installing |

---

## Sources

- [kind local registry guide](https://kind.sigs.k8s.io/docs/user/local-registry/) — official containerd config patch pattern and registry-on-kind-network requirement (HIGH confidence)
- [containerd registry configuration](https://github.com/containerd/containerd/blob/main/docs/cri/registry.md) — certs.d directory-based config for containerd v2 (HIGH confidence)
- [cert-manager kind development guide](https://cert-manager.io/docs/contributing/kind/) — cert-manager + kind integration, webhook readiness bootstrapping (HIGH confidence)
- [cert-manager Envoy Gateway TLS guide](https://gateway.envoyproxy.io/docs/tasks/security/tls-cert-manager/) — dependency ordering between cert-manager and Envoy Gateway (HIGH confidence)
- [Go defer in loop — JetBrains Inspectopedia](https://www.jetbrains.com/help/inspectopedia/GoDeferInLoop.html) — canonical documentation of the defer accumulation problem (HIGH confidence)
- [Go archive/tar package](https://pkg.go.dev/archive/tar) — `io.EOF` return semantics for `Next()` (HIGH confidence)
- [kinder CONCERNS.md](planning/codebase/CONCERNS.md) — codebase analysis identifying the four bugs, provider duplication fragility, and test coverage gaps (HIGH confidence — primary source)
- [kinder docker/provision.go](pkg/cluster/internal/providers/docker/provision.go) — direct code evidence of defer-in-loop in generatePortMappings (HIGH confidence)
- [kinder podman/provision.go](pkg/cluster/internal/providers/podman/provision.go) — direct code evidence of Podman port format divergence and volume pre-creation (HIGH confidence)
- [kinder nerdctl/provision.go](pkg/cluster/internal/providers/nerdctl/provision.go) — direct code evidence of nerdctl network flag divergence (HIGH confidence)
- [kinder docker/network.go](pkg/cluster/internal/providers/docker/network.go) — direct code evidence of network sort comparator (HIGH confidence)
- [Interface pollution in Go — 100 Go Mistakes](https://100go.co/5-interface-pollution/) — guidance on when to define interfaces for behavior extraction (MEDIUM confidence)
- [kind issue: containerd race restarting with config patches](https://github.com/kubernetes-sigs/kind/issues/2262) — confirmed timing issue with containerd restart after config patching (HIGH confidence)

---

*Pitfalls research for: kinder v1.3 — local registry, cert-manager, CLI diagnostic tools, provider deduplication, and bug fixes*
*Researched: 2026-03-03*
