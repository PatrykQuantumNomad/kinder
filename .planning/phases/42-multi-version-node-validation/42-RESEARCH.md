# Phase 42: Multi-Version Node Validation - Research

**Researched:** 2026-04-08
**Domain:** Go CLI, config validation, Kubernetes version-skew policy, tabular terminal output
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Validation strictness:**
- Hard fail on version-skew violations — reject config and exit before any containers are created
- No `--force` or override flag — invalid skew is a hard error
- Match upstream Kubernetes skew policy exactly: workers can be up to 3 minor versions behind the control-plane
- No warnings for configs within policy — if it passes validation, output is clean (no "you're near the boundary" noise)
- HA control-plane version matching: Claude's Discretion on whether to enforce same minor or same patch

**Error presentation:**
- Report ALL violations at once — user fixes everything in one pass
- Include remediation hints with each violation (e.g., "update worker-2 to v1.28+")
- Use table format for validation errors:
  ```
  Error: version-skew policy violated

    Node        Role           Version  Delta
    worker-2    worker         v1.27.1  -4
    worker-3    worker         v1.26.0  -5

  Control-plane version: v1.31.2
  Max allowed skew: 3 minor versions

  Hint: update worker-2 to v1.28+ and worker-3 to v1.28+
  ```
- HA control-plane mismatch errors include a brief reason explaining WHY mixing versions is rejected (e.g., etcd consistency)

**Version display format:**
- `kinder get nodes` shows both VERSION and IMAGE columns
- Add a SKEW column with checkmark/cross markers and delta (e.g., `✗ (-2)`)
- SKEW column is always present, even when all nodes are the same version (consistent output for scripting)
- `kinder get nodes --json` includes version, image, and skew fields per node
- Example output:
  ```
  NAME                  ROLE            STATUS   VERSION   IMAGE                    SKEW
  cluster-control-plane control-plane   Ready    v1.31.2   kindest/node:v1.31.2     ✓
  cluster-worker        worker          Ready    v1.31.2   kindest/node:v1.31.2     ✓
  cluster-worker2       worker          Ready    v1.29.0   kindest/node:v1.29.0     ✗ (-2)
  ```

**Doctor check behavior:**
- Severity level: Claude's Discretion — pick based on existing kinder doctor check patterns
- Version source: Claude's Discretion — pick the most reliable approach (live kubelet query vs image tag inspection)
- Use the same table format as create-time validation errors for consistency
- Check BOTH upstream skew policy AND config drift — warn if running versions differ from what was originally configured

**Claude's Discretion:**
- HA control-plane version strictness (same minor vs same patch)
- Doctor check severity level (warning vs error)
- Doctor version detection method (kubelet exec vs image tag inspection)
- Exact spacing and column widths in table output

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MVER-01 | `--image` flag only overrides nodes without an explicit image in config (fix fixupOptions bug) | Bug is in `fixupOptions` in `pkg/cluster/internal/create/create.go:379-385` — currently overwrites all nodes unconditionally |
| MVER-02 | Config validation rejects invalid version-skew combinations before provisioning | Add to `Cluster.Validate()` in `pkg/internal/apis/config/validate.go`; version parsing via existing `pkg/internal/version` package |
| MVER-03 | Clear error messages surface version-skew violations at config parse time | Table-format multi-error via `errors.NewAggregate` with structured violation data in `validate.go` |
| MVER-04 | `kinder doctor` detects version-skew issues in running clusters | New Check implementation in `pkg/internal/doctor/`; uses `nodeutils.KubeVersion()` + `nodes.Node.Command()` |
| MVER-05 | `kinder get nodes` output includes per-node Kubernetes version column | Extend `nodeInfo` struct and output logic in `pkg/cmd/kind/get/nodes/nodes.go` |
</phase_requirements>

---

## Summary

Phase 42 is a pure Go codebase change with five distinct but interrelated requirements. All five touch different layers of the kinder stack: CLI options, config validation, error presentation, doctor checks, and terminal output formatting. No new dependencies are introduced — the entire feature set is buildable from packages already in go.mod.

The most critical insight is that **version parsing infrastructure already exists** at `pkg/internal/version` (MIT-licensed Kubernetes upstream code with `ParseSemantic`, `Minor()`, `AtLeast()`, etc.) and **Kubernetes version reading from running nodes already exists** at `pkg/cluster/nodeutils/util.go:KubeVersion()` which reads `/kind/version` from the node container. Neither needs to be built.

The fix for MVER-01 is surgical: `fixupOptions` currently overwrites all node images unconditionally when `NodeImage != ""`. The fix is to only overwrite nodes where `Image` was not explicitly set in the config (i.e., nodes whose image matches the default or is empty before defaulting). The safest approach is to track which nodes had explicit images before defaulting — done by checking if the node's Image field in the *original* YAML config was empty (i.e., whether the image was set by `SetDefaultsNode` vs. by the user). A practical implementation: before `SetDefaultsNode` is called, the `v1alpha4.Node.Image` field is what the user wrote; after encoding/conversion to internal config, nodes that had `""` in the YAML still have the default image set. The cleanest fix is to store an `ExplicitImage bool` sentinel on the internal `Node` type, set during YAML decode when `image:` is present, and skip override in `fixupOptions` when `ExplicitImage` is true.

The table-format error output (MVER-02/03) must use Go's `fmt` package with manual column alignment — no external tabwriter dependency is needed because the `text/tabwriter` package is in the Go standard library and already available without a go.mod entry. [VERIFIED: codebase grep — `text/tabwriter` is stdlib]

For the doctor check (MVER-04), severity should be `warn` (not `fail`) because a running multi-version cluster that's within policy is fine; only violations warrant surfacing. The existing `kubectlVersionSkewCheck` in `pkg/internal/doctor/versionskew.go` uses `warn` for skew issues, and the new cluster-node-skew check should match that convention. Version detection should use `nodeutils.KubeVersion()` (reads `/kind/version`) rather than image tag parsing, because image tags with digests (`kindest/node:v1.31.2@sha256:...`) require stripping logic, while `/kind/version` always contains a clean semver string. The doctor check is a live-cluster check so it has access to node exec.

**Primary recommendation:** Implement in task order — MVER-01 (fixupOptions), MVER-02+03 (validation with table errors), MVER-05 (get nodes columns), MVER-04 (doctor check) — each task touching a discrete, independently testable module.

---

## Standard Stack

### Core (all already in go.mod)

| Package | Location | Purpose | Why Standard |
|---------|----------|---------|--------------|
| `sigs.k8s.io/kind/pkg/internal/version` | `pkg/internal/version/` | Semver parsing and comparison | Already used in `versionskew.go`; has `ParseSemantic`, `Minor()`, `AtLeast()` |
| `sigs.k8s.io/kind/pkg/cluster/nodeutils` | `pkg/cluster/nodeutils/util.go` | Reading K8s version from running nodes | `KubeVersion()` reads `/kind/version` via container exec |
| `sigs.k8s.io/kind/pkg/errors` | `pkg/errors/` | Aggregate error composition | `errors.NewAggregate()` used throughout validation |
| `text/tabwriter` | Go stdlib | Column-aligned terminal output | No dependency needed; stdlib |
| `encoding/json` | Go stdlib | JSON output for `--json` flag | Already imported in `nodes.go` |

### No New Dependencies Required

The v2.2 locked decision "Zero new Go module dependencies" is fully compatible with this phase. Every capability needed is either in the existing codebase or Go stdlib.

---

## Architecture Patterns

### Recommended File Changes

```
pkg/
├── cluster/internal/create/
│   └── create.go              # MVER-01: fix fixupOptions
├── internal/apis/config/
│   ├── types.go               # MVER-01: add ExplicitImage bool to internal Node
│   ├── validate.go            # MVER-02/03: add validateVersionSkew()
│   └── validate_test.go       # MVER-02/03: tests for skew validation
├── internal/apis/config/encoding/
│   └── convert_v1alpha4.go    # MVER-01: set ExplicitImage flag during conversion
├── internal/doctor/
│   ├── clusterskew.go         # MVER-04: new cluster node skew check
│   ├── clusterskew_test.go    # MVER-04: tests
│   └── check.go               # MVER-04: register new check in allChecks
└── cmd/kind/get/nodes/
    └── nodes.go               # MVER-05: extend nodeInfo, add VERSION+IMAGE+SKEW columns
```

### Pattern 1: ExplicitImage Sentinel (MVER-01)

**What:** Add `ExplicitImage bool` to the internal `Node` type to track whether the image was user-provided in YAML or was defaulted.

**When to use:** During YAML-to-internal conversion, set `ExplicitImage = true` when the v1alpha4 `Node.Image` field was non-empty. In `fixupOptions`, skip override when `ExplicitImage` is true.

**Example:**
```go
// Source: [pkg/internal/apis/config/types.go] — PROPOSED ADDITION
type Node struct {
    Role          NodeRole
    Image         string
    ExplicitImage bool   // true when image was set by user in config, false when defaulted
    Labels        map[string]string
    // ... other fields
}
```

```go
// Source: [pkg/cluster/internal/create/create.go] — fixupOptions fix
if opts.NodeImage != "" {
    for i := range opts.Config.Nodes {
        if !opts.Config.Nodes[i].ExplicitImage {
            opts.Config.Nodes[i].Image = opts.NodeImage
        }
    }
}
```

```go
// Source: [pkg/internal/apis/config/encoding/convert_v1alpha4.go] — during conversion
func V1Alpha4ToInternal(cfg *v1alpha4.Cluster) *config.Cluster {
    // ...
    for i, n := range cfg.Nodes {
        internalNode.ExplicitImage = n.Image != ""
        // ...
    }
}
```

### Pattern 2: Config-time Version Skew Validation (MVER-02/03)

**What:** Add `validateVersionSkew()` called from `Cluster.Validate()`. Extract minor version from image tag. Build a list of violations, then render a table-format error string.

**When to use:** During `Cluster.Validate()` — called in `create.go` immediately after `fixupOptions`.

**Version extraction from image tag:** Image tags follow the `kindest/node:vX.Y.Z` convention. Extract version with `strings.Split(image, ":")[1]` then parse the tag prefix before `@sha256:` if a digest is present. Use `version.ParseSemantic()` on the extracted string.

**HA control-plane strictness recommendation:** Enforce same **minor** version (not same patch). Rationale: kubeadm HA setup requires etcd cluster consistency; mixing minor versions risks API incompatibility in the etcd cluster. Patch version differences are acceptable (e.g., `v1.31.1` and `v1.31.2` can coexist). [ASSUMED — based on kubeadm HA documentation patterns; enforcing same-minor is the conservative safe choice]

**Example structure:**
```go
// Source: [pkg/internal/apis/config/validate.go] — PROPOSED
type skewViolation struct {
    NodeName string // e.g., "worker-2" or index-based "node[2]"
    Role     string
    Version  string // from image tag, e.g., "v1.27.1"
    Delta    int    // negative = behind, e.g., -4
}

func validateVersionSkew(nodes []Node) error {
    // 1. Find control-plane minor version(s)
    // 2. Validate HA control-planes share same minor
    // 3. Validate workers are within 3 minor of control-plane
    // 4. Collect all violations, render table
}
```

**Table rendering without external deps:**
```go
// Source: [Go stdlib text/tabwriter]
import "text/tabwriter"

func renderSkewViolationTable(violations []skewViolation, cpVersion string) string {
    var buf bytes.Buffer
    w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "  Node\tRole\tVersion\tDelta")
    for _, v := range violations {
        fmt.Fprintf(w, "  %s\t%s\t%s\t%d\n", v.NodeName, v.Role, v.Version, v.Delta)
    }
    w.Flush()
    // append summary + hint lines
    return buf.String()
}
```

**Version extraction from image tag:**
```go
// Source: [VERIFIED: pkg/cluster/internal/providers/nerdctl/images.go:88 pattern]
func imageTagVersion(image string) (string, error) {
    // strip digest: "kindest/node:v1.31.2@sha256:abc..." -> "kindest/node:v1.31.2"
    base := strings.Split(image, "@sha256:")[0]
    // extract tag: "kindest/node:v1.31.2" -> "v1.31.2"
    parts := strings.SplitN(base, ":", 2)
    if len(parts) != 2 {
        return "", fmt.Errorf("image %q has no tag", image)
    }
    return parts[1], nil
}
```

### Pattern 3: Doctor Cluster Skew Check (MVER-04)

**What:** New `Check` implementation that lists running cluster nodes, reads their `/kind/version`, computes skew, and reports violations.

**Severity:** `warn` — consistent with `kubectlVersionSkewCheck`. A running cluster with skew is degraded but not necessarily broken. [RECOMMENDED based on existing doctor patterns]

**Version detection:** `nodeutils.KubeVersion(n)` which runs `cat /kind/version` inside the container. This is the most reliable approach: it's already used in the codebase, avoids image tag parsing complexity, and works for all providers. [VERIFIED: `pkg/cluster/nodeutils/util.go:35-46`]

**Challenge:** The doctor check currently has no cluster context. The existing `kinder doctor` only checks host tools/config, not running clusters. The new check needs a way to discover running clusters and their nodes. Looking at the architecture:
- The doctor check is currently stateless and has no `Provider` reference
- To add a cluster-aware check, the check needs either: (a) accept a provider as a constructor arg, or (b) detect the cluster at runtime via the provider
- The cleanest approach: new check constructor accepts optional `provider + clusterName` args; if nil, skip with a "no cluster context" skip result

**Config drift detection:** The check should compare running version (from `/kind/version`) to the version encoded in the image name on the running container. Use `docker inspect --format "{{.Config.Image}}"` to retrieve the container image. The `common.Node.Command()` method provides exec access; image name retrieval requires a provider inspect call (same pattern as `Role()` in `pkg/cluster/internal/providers/common/node.go`).

**Registration:** Add to `allChecks` in `pkg/internal/doctor/check.go` under a new "Cluster" category.

### Pattern 4: get nodes Column Extension (MVER-05)

**What:** Extend the tabular output of `kinder get nodes` to include VERSION, IMAGE, and SKEW columns. Extend the JSON `nodeInfo` struct.

**STATUS column:** The spec shows STATUS (Ready/NotReady) but the current `nodes.Node` interface has no `Status()` method. Reading status requires running `kubectl get node <name> -o jsonpath=...` which is heavy. For a simpler approach: use `Ready` as a constant for now (or omit STATUS from the display and only add VERSION/IMAGE/SKEW). [ASSUMED — needs checking against success criteria]

**Re-reading success criteria:** "kinder get nodes output includes a column showing the Kubernetes version installed on each node" — SUCCESS CRITERIA 5 only requires VERSION column. The CONTEXT.md example table also shows STATUS, which requires docker inspect for the running state or kubelet health. The safest approach is to implement STATUS using the node's Role result (always returning "Ready" for discovered running nodes, as listing nodes implies they're up) or to use `docker inspect --format "{{.State.Running}}"` to get a boolean.

**Version source for `get nodes`:** `nodeutils.KubeVersion(n)` reads `/kind/version` via container exec — authoritative. May be slow for large clusters (N exec calls). For the STATUS column, `docker inspect` (one call per node) already happens for `Role()`.

**Example extended nodeInfo:**
```go
// Source: [pkg/cmd/kind/get/nodes/nodes.go] — PROPOSED EXTENSION
type nodeInfo struct {
    Name    string `json:"name"`
    Role    string `json:"role"`
    Version string `json:"version"`
    Image   string `json:"image"`
    Skew    string `json:"skew"`  // e.g., "ok" or "-2"
    SkewOK  bool   `json:"skewOk"`
}
```

**Getting image name of running container:** Use provider inspect (same mechanism as Role). The `common.Node` struct has `BinaryName` and `Name`; run `docker inspect --format "{{.Config.Image}}" <name>`. However `nodes.Node` interface only exposes `Command()` (exec inside container), `Role()`, `IP()`, `String()` — there is no `Image()` method. To get the container image from outside, a separate docker/podman inspect call is needed. This is **not** available through the `nodes.Node` interface and requires either: (a) extending the interface, or (b) calling the runtime binary directly in the `nodes` command.

**Recommendation for image column:** Introduce a helper function in `nodeutils` or directly in the `nodes.go` command that runs `docker inspect --format "{{.Config.Image}}" <containerName>` using `exec.Command`. This is consistent with the existing pattern in `common/node.go` for Role. [ASSUMED — no existing `Image()` method on Node interface verified]

### Anti-Patterns to Avoid

- **Don't parse versions from image tags during doctor check:** Use `nodeutils.KubeVersion()` (reads `/kind/version`) for running-node version detection. Image tag parsing is error-prone with digest suffixes.
- **Don't return an error from `Cluster.Validate()` for valid configs:** The no-warnings-for-valid-configs constraint means `validateVersionSkew()` returns `nil` when all nodes are within policy.
- **Don't use `os.Exit()` directly in validate path:** The validation error propagates naturally through `Cluster.Validate()` → `create.go:150` → caller.
- **Don't extend the `nodes.Node` interface** to add `Image()`: Adding a method breaks all mock implementations. Instead, add a standalone helper function that takes the node name and calls `docker inspect` directly.
- **Don't call `tabwriter.NewWriter` for the validation error message** if the output will be embedded in a `fmt.Errorf()` chain — build the string first, then wrap with `errors.New(str)`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Semver parsing | Custom regex version parser | `pkg/internal/version.ParseSemantic()` | Already handles leading `v`, pre-release, build metadata |
| Minor version comparison | Manual string splitting | `version.Minor()` method | Type-safe, handles edge cases |
| Version from running node | Image tag string parsing | `nodeutils.KubeVersion()` → `/kind/version` | Authoritative, no digest stripping needed |
| Column-aligned output | Manual space padding | `text/tabwriter` (stdlib) | Handles variable-width strings correctly |
| Multi-error collection | Returning first error | `errors.NewAggregate(errs)` | Already used in `validate.go` for collecting all errors |
| Container exec | Direct subprocess calls | `nodes.Node.Command()` / `exec.Command()` | Provider-abstracted, testable with fakeCmd |

**Key insight:** The version infrastructure (parsing, comparison, node version reading) is 100% pre-built. The entire feature is assembly of existing primitives, not new capability construction.

---

## Common Pitfalls

### Pitfall 1: fixupOptions Override Ordering

**What goes wrong:** `fixupOptions` runs before `Validate()`. If the fix to `fixupOptions` is incorrect (e.g., checking for empty image vs. checking `ExplicitImage`), `Validate()` will still see the wrong image and either silently accept a wrong version or produce confusing errors.

**Why it happens:** The `--image` flag is applied during `fixupOptions` which modifies the config in-place. If a node had `image: kindest/node:v1.29.0` in YAML and `--image kindest/node:v1.31.2` was passed, the bug overwrites the explicit node image before validation even runs.

**How to avoid:** Add `ExplicitImage bool` during YAML conversion (before defaulting), not by comparing Image strings post-defaulting. The conversion layer is `V1Alpha4ToInternal` in `pkg/internal/apis/config/encoding/convert_v1alpha4.go`.

**Warning signs:** Test that explicitly-imaged node is not overridden by `--image` flag.

### Pitfall 2: Version Extraction from Image Tag Edge Cases

**What goes wrong:** Image tags can be `kindest/node:v1.31.2`, `kindest/node:v1.31.2@sha256:abc123`, or even `registry.example.com:5000/kindest/node:v1.31.2`. The tag extraction logic must handle all forms.

**Why it happens:** `strings.Split(image, ":")[1]` will incorrectly split on the registry port.

**How to avoid:** Strip the digest first (`strings.SplitN(image, "@sha256:", 2)[0]`), then extract the last colon-separated segment using `strings.LastIndex(base, ":")`.

**Warning signs:** Unit test with `registry:5000/image:v1.2.3` and `image:v1.2.3@sha256:abc`.

### Pitfall 3: Node Name in Validation Error Table

**What goes wrong:** The internal `config.Node` type has no `Name` field — nodes are anonymous in the config (named by their role + index at provisioning time). The error table column "Node" cannot show a container name at config-parse time.

**Why it happens:** Container names like `cluster-worker2` are assigned during provisioning by `MakeNodeNamer`. At validation time, only role and index are known.

**How to avoid:** In the validation error table, use a synthesized name like the role + index: `worker[0]`, `worker[1]`, `control-plane[0]`. The user's error message example in the context uses `worker-2` which appears to be illustrative, not a real container name. The actual provisioned name format is `<cluster>-worker`, `<cluster>-worker2` etc.

**Warning signs:** Verify the error format against the user's exact expected output from CONTEXT.md.

### Pitfall 4: Doctor Check with No Cluster Running

**What goes wrong:** If `kinder doctor` is run with no cluster provisioned, the cluster-skew check will fail trying to list nodes.

**Why it happens:** The check needs a Provider + cluster name to list nodes. When no cluster is running, `ListNodes()` returns empty slice.

**How to avoid:** Return `skip` result when no cluster context is available. Use `cluster.NewProvider().ListNodes(clusterName)` — if empty list, emit `skip` with message `"(no cluster running)"`.

**Warning signs:** Test with no cluster present.

### Pitfall 5: STATUS Column for `get nodes` — Node Interface Mismatch

**What goes wrong:** The desired output table shows a STATUS column (Ready/NotReady), but the `nodes.Node` interface has no `Status()` method. Checking running state requires either a docker inspect call (outside the interface) or calling kubelet status via exec.

**Why it happens:** The `nodes.Node` interface was designed for narrow purposes (exec, role, IP). Display concerns were not part of its contract.

**How to avoid:** For the STATUS column, use a simple heuristic: if `ListNodes()` returned the node, it is a running container. Use a direct container runtime inspect for accurate status. Alternatively, check success criteria — only VERSION column is strictly required (MVER-05). If STATUS is needed per the output example in CONTEXT.md, implement it via a direct `docker/podman/nerdctl inspect --format "{{.State.Running}}"` call using `exec.Command` in the nodes command — consistent with how `Role()` works in `common/node.go`.

---

## Code Examples

Verified patterns from existing codebase:

### Existing Version Parsing (use this)
```go
// Source: pkg/internal/version/version.go + pkg/internal/doctor/versionskew.go
import "sigs.k8s.io/kind/pkg/internal/version"

v, err := version.ParseSemantic("v1.29.0")
if err != nil { ... }
minor := v.Minor() // uint: 29
```

### Existing KubeVersion from Running Node (use this for MVER-04)
```go
// Source: pkg/cluster/nodeutils/util.go:35-46
// Already used internally; reads /kind/version from container
ver, err := nodeutils.KubeVersion(n) // returns "v1.29.0"
```

### Existing Aggregate Error Pattern (use this for MVER-02/03)
```go
// Source: pkg/internal/apis/config/validate.go
errs := []error{}
// ... collect errors ...
if len(errs) > 0 {
    return errors.NewAggregate(errs)
}
return nil
```

### Existing fakeExecCmd Test Helper (use this for MVER-04 tests)
```go
// Source: pkg/internal/doctor/testhelpers_test.go
check := &myCheck{
    execCmd: newFakeExecCmd(map[string]fakeExecResult{
        "kubectl version --client -o json": {lines: `{...}`},
    }),
}
```

### Image Tag Stripping Pattern (for config-time version extraction)
```go
// Source: pkg/cluster/internal/providers/nerdctl/images.go:88 (pattern)
func imageTagVersion(image string) (string, error) {
    base := strings.Split(image, "@sha256:")[0]
    idx := strings.LastIndex(base, ":")
    if idx == -1 {
        return "", fmt.Errorf("image %q has no tag", image)
    }
    return base[idx+1:], nil
}
```

### Tabwriter Column Alignment (stdlib, no import needed in go.mod)
```go
// Source: Go stdlib text/tabwriter documentation
import (
    "bytes"
    "fmt"
    "text/tabwriter"
)

var buf bytes.Buffer
w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
fmt.Fprintln(w, "  Node\tRole\tVersion\tDelta")
fmt.Fprintf(w, "  %s\t%s\t%s\t%d\n", nodeName, role, ver, delta)
w.Flush()
```

### Doctor Check Registration Pattern (follow this for MVER-04)
```go
// Source: pkg/internal/doctor/check.go:53-80
var allChecks = []Check{
    // ...
    // Category: Cluster
    newClusterVersionSkewCheck(provider, clusterName), // MVER-04
}
```

---

## Claude's Discretion Recommendations

### HA Control-Plane Version Strictness
**Recommendation: same minor version** (not same patch).

Rationale: kubeadm HA setup uses etcd as a distributed consensus store. Mixed minor versions in the control plane can cause API server version drift that breaks etcd raft consensus during upgrades. The Kubernetes documentation states HA control plane nodes must run the same version of kube-apiserver, kube-controller-manager, and kube-scheduler. Patch-level differences (v1.31.1 vs v1.31.2) are fine (same API surface). [ASSUMED — kubeadm HA documentation states matching versions; enforcing same-minor is consistent with upstream requirement]

Error message for HA mismatch:
```
Error: HA control-plane version mismatch

  Node                     Role           Version
  cluster-control-plane    control-plane  v1.31.2
  cluster-control-plane2   control-plane  v1.30.1

Reason: HA control-plane nodes must run the same minor version.
Mixed minor versions risk etcd cluster consistency and API compatibility
during rollover. All control-plane nodes must use the same minor version.
```

### Doctor Check Severity
**Recommendation: `warn`** (not `fail`).

Rationale: A running cluster with version skew might be intentional (e.g., in-progress rolling upgrade). The check should surface the condition without blocking doctor from exiting 0 if this is the only issue. The existing `kubectlVersionSkewCheck` uses `warn` for the same class of problem. Exit code 2 (warn-only) is appropriate.

### Doctor Version Detection Method
**Recommendation: `nodeutils.KubeVersion()` (reads `/kind/version`)** not image tag inspection.

Rationale:
1. `KubeVersion()` already exists and is tested
2. `/kind/version` contains a clean semver string with no digest stripping needed
3. Image tag inspection is fragile with custom registries and digest-pinned images
4. This approach is used by the existing integration test infrastructure

One caveat: the doctor check needs cluster+provider context to call `KubeVersion()`. The check must either accept these as constructor args or use a provider auto-detected at runtime. Recommend constructor injection for testability.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `--image` overrides all nodes | `--image` only overrides nodes without explicit images | Phase 42 (this phase) | Enables per-node version configuration |
| No version validation at config time | Reject invalid skew before provisioning | Phase 42 (this phase) | Eliminates cryptic kubeadm join failures |
| `kinder get nodes` shows only name | `kinder get nodes` shows VERSION/IMAGE/SKEW columns | Phase 42 (this phase) | First-class multi-version cluster visibility |

---

## Environment Availability

Step 2.6: SKIPPED — this phase is a pure Go code/config change. All required packages are in the existing codebase. No external tools, services, or CLI utilities beyond the project's own code.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing stdlib (no external framework) |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./pkg/internal/apis/config/... ./pkg/internal/doctor/... ./pkg/cmd/kind/get/nodes/... ./pkg/cluster/internal/create/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MVER-01 | `--image` does not override explicit node images | unit | `go test ./pkg/cluster/internal/create/... -run TestFixupOptions` | ❌ Wave 0 |
| MVER-02 | Config validation rejects >3 minor skew | unit | `go test ./pkg/internal/apis/config/... -run TestClusterValidate` | ✅ (extend existing) |
| MVER-03 | Error table format has correct columns and hints | unit | `go test ./pkg/internal/apis/config/... -run TestVersionSkewError` | ❌ Wave 0 |
| MVER-04 | Doctor reports skew violations on running cluster | unit | `go test ./pkg/internal/doctor/... -run TestClusterVersionSkewCheck` | ❌ Wave 0 |
| MVER-05 | `get nodes` JSON output includes version/image/skew | unit | `go test ./pkg/cmd/kind/get/nodes/... -run TestGetNodesJSON` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./pkg/internal/apis/config/... ./pkg/internal/doctor/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go test ./...` green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `pkg/cluster/internal/create/fixup_test.go` — covers MVER-01 fixupOptions behavior
- [ ] `pkg/internal/apis/config/versionskew_test.go` — covers MVER-02/03 skew validation
- [ ] `pkg/internal/doctor/clusterskew_test.go` — covers MVER-04 cluster skew check
- [ ] Test helpers for `nodeutils.KubeVersion` with mock node — needed for MVER-04/05

---

## Security Domain

This phase contains no authentication, session management, access control, or cryptography concerns. All changes are:
- Local config parsing and validation
- Local container exec (existing pattern)
- Terminal output formatting

ASVS categories V2-V6 do not apply. No new attack surface is introduced.

---

## Open Questions (RESOLVED)

1. **Node naming in validation errors vs. provisioned names**
   - What we know: Internal `config.Node` has no Name field; names are assigned at provisioning time by `MakeNodeNamer`
   - RESOLVED: Use role+index for config-time errors (e.g., `worker[1]`). At config-parse time, the cluster name may not be final. The user's CONTEXT.md example shows `worker-2` which is illustrative. For doctor checks on running clusters, use the actual container name from `node.String()`.

2. **STATUS column implementation for `get nodes`**
   - What we know: `nodes.Node` interface has no `Status()` method; STATUS requires either container inspect or kubelet API call
   - RESOLVED: Include STATUS column since CONTEXT.md example output shows it. Implement using a simple heuristic: if `KubeVersion()` succeeds (node responds to exec), status is "Ready"; if it fails, status is "NotReady". This avoids adding a new `Status()` method to the Node interface.

3. **Doctor check cluster discovery**
   - What we know: Doctor checks are currently stateless (no provider context); the check needs to discover and list running clusters
   - RESOLVED: Auto-discover the default cluster (`kind` cluster name) using `cluster.NewProvider().ListNodes(cluster.DefaultName)`. If no default cluster exists, return skip result. Use dependency injection in the check struct for testability.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | HA control-plane should enforce same minor version (not same patch) | Architecture Patterns / HA strictness | If kubeadm actually allows minor version mixing in HA, we're being too strict — but this is conservative and correct |
| A2 | Doctor cluster-skew check should auto-discover default cluster (`kind`) | Open Questions | If user has multiple clusters, only one gets checked — acceptable for v1 |
| A3 | STATUS column for `get nodes` requires separate container inspect call (no Node.Status() method exists) | Architecture Patterns / Pattern 4 | If a `Status()` method was added to the interface in an unreached file, we'd duplicate logic |
| A4 | `/kind/version` always contains a clean semver string in all kindest/node images | Code Examples | If older node images lack this file, `KubeVersion()` will error — acceptable, same behavior as existing usage |

---

## Sources

### Primary (HIGH confidence)
- `pkg/internal/version/version.go` — full semver parser; `ParseSemantic()`, `Minor()`, `AtLeast()` verified in codebase
- `pkg/cluster/nodeutils/util.go:35-46` — `KubeVersion()` function verified; reads `/kind/version`
- `pkg/cluster/internal/create/create.go:363-388` — `fixupOptions()` bug confirmed; overwrites all nodes unconditionally
- `pkg/internal/apis/config/validate.go` — `Cluster.Validate()` entry point confirmed; skew validation will be added here
- `pkg/internal/doctor/check.go` — Check interface and `allChecks` registry confirmed
- `pkg/internal/doctor/versionskew.go` — existing kubectlVersionSkewCheck pattern confirmed; uses `warn` status
- `pkg/cluster/internal/providers/common/node.go` — `Role()` inspect pattern confirmed; image inspect follows same pattern
- `pkg/cmd/kind/get/nodes/nodes.go` — nodeInfo struct and output logic confirmed

### Secondary (MEDIUM confidence)
- [Kubernetes Version Skew Policy](https://kubernetes.io/releases/version-skew-policy/) — kubelet up to 3 minor versions behind kube-apiserver is supported (verified via WebSearch, official k8s.io source)
- Go stdlib `text/tabwriter` — column alignment without dependencies

### Tertiary (LOW confidence)
- A1 (HA same-minor enforcement): Based on kubeadm documentation patterns; not directly verified via tool call

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified in codebase via direct file reads
- Architecture: HIGH — all touch points verified; `fixupOptions` bug confirmed by reading source
- Pitfalls: HIGH — node-naming pitfall and interface-gap pitfall verified by reading `types.go` and `nodes/types.go`
- HA strictness recommendation: MEDIUM — conservative choice based on kubeadm HA documentation

**Research date:** 2026-04-08
**Valid until:** 2026-05-08 (stable stdlib + internal packages; no external deps to drift)
