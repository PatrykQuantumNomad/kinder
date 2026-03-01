# Phase 4: CoreDNS Tuning - Research

**Researched:** 2026-03-01
**Domain:** CoreDNS ConfigMap read-modify-write in Go, autopath plugin, pods verified, cache TTL
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DNS-01 | CoreDNS Corefile is patched (not replaced) with `autopath @kubernetes` plugin | Read Corefile from ConfigMap via kubectl into bytes.Buffer, insert `autopath @kubernetes` line before the `kubernetes` block, write back via `kubectl apply -f -`; rollout restart ensures CoreDNS picks up new config |
| DNS-02 | CoreDNS `pods insecure` is changed to `pods verified` (required for autopath) | strings.ReplaceAll on the Corefile string: `pods insecure` → `pods verified`; autopath @kubernetes requires pods verified per official CoreDNS docs (HIGH confidence) |
| DNS-03 | Cache TTL is increased from 30s to 60s for external queries | strings.ReplaceAll: `cache 30` → `cache 60`; the default Corefile uses `cache 30` as a top-level directive; replacing it with `cache 60` sets the max TTL for all cached records |
| DNS-04 | Existing in-cluster DNS resolution continues to work after patching | The patch is surgical: only `pods insecure` → `pods verified`, `cache 30` → `cache 60`, and insertion of `autopath @kubernetes`; all other Corefile content preserved verbatim; rollout restart does rolling update with zero downtime |
| DNS-05 | User can disable CoreDNS tuning via `addons.coreDNSTuning: false` in cluster config | Already wired in `create.go`: `runAddon("CoreDNS Tuning", opts.Config.Addons.CoreDNSTuning, installcorednstuning.NewAction())` — the stub is registered; only Execute body needs implementation |
</phase_requirements>

## Summary

Phase 4 replaces the `installcorednstuning` stub action with a complete implementation that patches the CoreDNS Corefile in-place. Unlike other addon actions (MetalLB, Metrics Server), CoreDNS tuning does NOT apply a new manifest — it reads the existing `coredns` ConfigMap in the `kube-system` namespace, surgically modifies the Corefile string in Go, writes the updated ConfigMap back, then triggers a rolling restart of the CoreDNS deployment so the new Corefile takes effect.

The three modifications required are: (1) change `pods insecure` to `pods verified` (required prerequisite for autopath), (2) insert `autopath @kubernetes` as a new line in the server block, and (3) change `cache 30` to `cache 60`. These three changes are applied as sequential `strings.ReplaceAll` calls on the raw Corefile string — no YAML or Corefile parser is needed because the default kind Corefile has a consistent, predictable format. The modified Corefile is piped back via `kubectl apply -f -` using a ConfigMap YAML envelope.

After applying the ConfigMap update, a `kubectl rollout restart deployment/coredns -n kube-system` must be issued and the action must wait for the rollout to complete. The CoreDNS `reload` plugin does detect Corefile changes on disk, but ConfigMap volume propagation has a 60-90 second kubelet sync delay — forcing a rollout restart is faster (typically 10-20 seconds) and deterministic. DNS-04 is satisfied because a rolling restart replaces one pod at a time, maintaining at least one live CoreDNS pod throughout.

DNS-05 is already wired in `create.go` at the `runAddon` call level. Phase 4 only needs to replace the stub's Execute body — no changes to `create.go` or config types are required.

**Primary recommendation:** Read Corefile via `kubectl get configmap coredns -n kube-system -o jsonpath='{.data.Corefile}'` into a bytes.Buffer, apply three `strings.ReplaceAll` transforms in Go, wrap in a ConfigMap YAML, write back via `kubectl apply -f -`, then run `kubectl rollout restart deployment/coredns -n kube-system` and wait for rollout completion.

## Standard Stack

### Core
| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| `bytes.Buffer` (stdlib) | - | Capture kubectl get output into Go string | Same pattern as `installcni` (reads files via node.Command + SetStdout) |
| `strings.ReplaceAll` (stdlib) | - | Surgical Corefile string transforms | Three well-defined substitutions on predictable text |
| `fmt.Sprintf` (stdlib) | - | Construct ConfigMap YAML envelope for write-back | Build the patch document from the modified Corefile string |
| `kubectl get configmap` (inside node) | - | Read current Corefile from kube-system ConfigMap | Established pattern; control plane node has admin.conf access |
| `kubectl apply -f -` (inside node) | - | Write updated ConfigMap back to cluster | Idempotent; same as MetalLB/Metrics Server apply pattern |
| `kubectl rollout restart` (inside node) | - | Force CoreDNS pods to reload new Corefile | ConfigMap volume propagation delay is 60-90s; rollout restart is 10-20s and deterministic |
| `kubectl rollout status` (inside node) | - | Wait for rolling restart to complete | Blocks until all pods have picked up new config |

### Supporting
| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| `strings.Contains` (stdlib) | - | Guard: verify Corefile has expected tokens before patching | Detect if Corefile is in expected format; fail early if not |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `strings.ReplaceAll` on raw Corefile | CoreDNS Corefile parser (external library) | Parser handles any Corefile format; strings.ReplaceAll works reliably for the predictable kubeadm default; no external dependency needed |
| `kubectl apply -f -` for write-back | `kubectl patch configmap coredns --type merge` with JSON patch | JSON patch of a multi-line string requires JSON escaping of newlines/special chars in Go; kubectl apply with YAML is cleaner and the established project pattern |
| `kubectl rollout restart` + wait | Wait 90s for ConfigMap volume propagation | Propagation is non-deterministic (60-90s+); rollout restart is 10-20s and deterministic |
| `kubectl rollout restart` | `kubectl delete pod -n kube-system -l k8s-app=kube-dns` | Pod delete is less controlled; rollout restart respects PodDisruptionBudgets and rolling strategy |

**Installation:** No new Go module dependencies required. All patterns already exist in the codebase.

## Architecture Patterns

### Recommended Project Structure

```
pkg/cluster/internal/create/actions/installcorednstuning/
└── corednstuning.go          # Action implementation (Execute func) — NO manifests/ dir needed
```

This differs from MetalLB and Metrics Server: there is no embedded manifest. CoreDNS tuning reads and modifies the existing cluster-installed ConfigMap. No `manifests/` subdirectory is created.

### Pattern 1: Read ConfigMap via kubectl into bytes.Buffer

**What:** Run `kubectl get configmap coredns -n kube-system -o jsonpath='{.data.Corefile}'` inside the control plane node, capture stdout into a `bytes.Buffer`, convert to string for manipulation.
**When to use:** Any time the action needs to read existing cluster state rather than apply a new manifest.
**Example:**
```go
// Source: derived from installcni/cni.go pattern (var raw bytes.Buffer + node.Command(...).SetStdout(&raw).Run())
var corefile bytes.Buffer
if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "get", "configmap", "coredns",
    "--namespace=kube-system",
    "-o", "jsonpath={.data.Corefile}",
).SetStdout(&corefile).Run(); err != nil {
    return errors.Wrap(err, "failed to read CoreDNS Corefile from ConfigMap")
}
corefileStr := corefile.String()
```

### Pattern 2: String-Level Corefile Modifications

**What:** Apply three `strings.ReplaceAll` transformations on the raw Corefile string. Guard each replacement with a `strings.Contains` check so the action fails clearly if the Corefile format is unexpected.
**When to use:** When the config to modify is a predictable-format string blob (not structured YAML/JSON that can be parsed).
**Example:**
```go
// Guard: verify expected tokens exist before patching
if !strings.Contains(corefileStr, "pods insecure") {
    return errors.New("CoreDNS Corefile does not contain expected 'pods insecure' directive; cannot patch safely")
}
if !strings.Contains(corefileStr, "cache 30") {
    return errors.New("CoreDNS Corefile does not contain expected 'cache 30' directive; cannot patch safely")
}

// Transform 1: pods insecure -> pods verified (required for autopath)
corefileStr = strings.ReplaceAll(corefileStr, "pods insecure", "pods verified")

// Transform 2: Insert autopath @kubernetes before the kubernetes block
// The kubernetes plugin block starts with "kubernetes cluster.local"
corefileStr = strings.ReplaceAll(
    corefileStr,
    "    kubernetes cluster.local",
    "    autopath @kubernetes\n    kubernetes cluster.local",
)

// Transform 3: Increase cache TTL from 30s to 60s
corefileStr = strings.ReplaceAll(corefileStr, "cache 30", "cache 60")
```

### Pattern 3: Write-Back via ConfigMap YAML Envelope + kubectl apply

**What:** Wrap the modified Corefile string in a ConfigMap YAML document and pipe it to `kubectl apply -f -` on stdin. Use `fmt.Sprintf` with a YAML template string.
**When to use:** When writing back a modified ConfigMap with a multi-line string value (the pipe-to-stdin pattern avoids shell escaping issues).
**Example:**
```go
// Source: pattern from installmetallb (crYAML + kubectl apply -f - with SetStdin)
const configMapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
%s
`

// Indent each Corefile line with 4 spaces (required for YAML block scalar under "  Corefile: |")
indentedCorefile := indentCorefile(corefileStr)
configMapYAML := fmt.Sprintf(configMapTemplate, indentedCorefile)

if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "apply", "-f", "-",
).SetStdin(strings.NewReader(configMapYAML)).Run(); err != nil {
    return errors.Wrap(err, "failed to apply updated CoreDNS ConfigMap")
}
```

**Helper for indentation:**
```go
// indentCorefile adds 4 spaces to each line of the Corefile for YAML block scalar indentation
func indentCorefile(corefile string) string {
    lines := strings.Split(corefile, "\n")
    for i, line := range lines {
        if line != "" {
            lines[i] = "    " + line
        }
    }
    return strings.Join(lines, "\n")
}
```

### Pattern 4: Rollout Restart + Wait for Completion

**What:** Trigger a rolling restart of the CoreDNS deployment, then wait for it to complete before the action returns. This ensures DNS is fully functional with the new config before `kinder create cluster` finishes.
**When to use:** After any ConfigMap change that CoreDNS must pick up. The `reload` plugin has up to 45s jitter delay; rollout restart is deterministic.
**Example:**
```go
// Source: standard kubectl pattern; same node.Command style as MetalLB webhook wait
if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "rollout", "restart",
    "--namespace=kube-system",
    "deployment/coredns",
).Run(); err != nil {
    return errors.Wrap(err, "failed to restart CoreDNS deployment")
}

if err := node.Command(
    "kubectl",
    "--kubeconfig=/etc/kubernetes/admin.conf",
    "rollout", "status",
    "--namespace=kube-system",
    "deployment/coredns",
    "--timeout=60s",
).Run(); err != nil {
    return errors.Wrap(err, "CoreDNS rollout did not complete after config update")
}
```

### Anti-Patterns to Avoid

- **Using `kubectl patch configmap --type json` with inline Corefile:** Requires JSON-escaping all newlines and special characters; brittle and hard to read. The YAML envelope + `kubectl apply -f -` is cleaner.
- **Replacing the entire Corefile with a hardcoded string:** Violates DNS-01 ("not replaced"). If the kind image upgrades its Corefile format, a hardcoded replacement breaks. Only touch the three specific directives.
- **Relying on the reload plugin to propagate ConfigMap changes:** ConfigMap volume mount propagation takes 60-90 seconds (kubelet sync). The reload plugin does detect file changes, but the pod filesystem update is delayed. Forcing a rollout restart is faster and deterministic.
- **Not guarding with `strings.Contains` before `strings.ReplaceAll`:** If a future kind version changes `pods insecure` to `pods disabled` or removes `cache 30`, a silent no-op replace would leave CoreDNS unconfigured with no error.
- **Waiting for the rollout before applying the ConfigMap change:** Apply ConfigMap first, then restart. The restart without the ConfigMap change is a no-op.
- **Using `kubectl create` instead of `kubectl apply` for the ConfigMap:** `kubectl create` fails if the ConfigMap already exists. Always use `apply` for idempotency.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Corefile syntax parsing | Custom Go Corefile parser | `strings.ReplaceAll` on predictable default format | The default kubeadm/kind Corefile has a consistent, well-known format; a full parser adds complexity without benefit for this use case |
| ConfigMap YAML serialization | Custom Go ConfigMap struct marshaling | `fmt.Sprintf` with a YAML template string | The ConfigMap structure is trivially simple (one string key); no need for k8s client-go or YAML marshal |
| DNS resolution verification | Custom DNS query in Go | `kubectl rollout status` wait | The rollout status covers the readiness probe which verifies CoreDNS is running; DNS resolution testing adds complexity and timing sensitivity |
| ConfigMap write-back escaping | Manual JSON escaping of multi-line Corefile | YAML block scalar via pipe-to-stdin pattern | YAML handles multi-line strings cleanly; JSON requires `\n` escaping throughout the Corefile |

**Key insight:** The entire complexity of this phase is in three string substitutions plus the kubectl plumbing. The correct approach is minimal and surgical — do not introduce parsers, client-go dependencies, or complex YAML manipulation.

## Common Pitfalls

### Pitfall 1: autopath @kubernetes Position in Corefile
**What goes wrong:** `autopath @kubernetes` is added inside the `kubernetes` plugin block instead of as a sibling directive in the server block. CoreDNS fails to start with a parse error.
**Why it happens:** The keyword `autopath` looks like it belongs inside the kubernetes block, but it is a separate plugin directive at the same indentation level as `kubernetes`.
**How to avoid:** Insert `autopath @kubernetes` as a new line immediately BEFORE `kubernetes cluster.local`, not inside the kubernetes braces. The correct order in the server block is:
```
    autopath @kubernetes
    kubernetes cluster.local in-addr.arpa ip6.arpa {
        pods verified
        ...
    }
```
**Warning signs:** CoreDNS pod enters CrashLoopBackOff; logs show "Corefile parse error" or "plugin not found: autopath".

### Pitfall 2: pods insecure Not Changed Before autopath
**What goes wrong:** autopath @kubernetes is added but `pods insecure` is still present. CoreDNS starts but autopath silently fails — DNS works but search path optimization is not active.
**Why it happens:** The official CoreDNS docs state "pods must be set to verified for autopath to function properly." This is a strict requirement.
**How to avoid:** Always apply Transform 1 (pods insecure → pods verified) before or at the same time as inserting autopath @kubernetes.
**Warning signs:** autopath is present but DNS lookups still show 5+ queries per hostname (SERVFAIL → retry cycle not eliminated).

### Pitfall 3: Corefile YAML Indentation Errors on Write-Back
**What goes wrong:** The modified Corefile is written back to the ConfigMap without proper YAML indentation. kubectl reports a YAML parse error, or the ConfigMap applies but CoreDNS gets a malformed Corefile.
**Why it happens:** YAML block scalars (using `|`) require the content to be consistently indented. The Corefile lines must each be indented relative to the `Corefile:` key in the ConfigMap YAML.
**How to avoid:** Use the `indentCorefile` helper to prepend 4 spaces to every non-empty line before inserting into the `fmt.Sprintf` template. Verify the result manually during development.
**Warning signs:** `kubectl apply -f -` exits with "yaml: line N: mapping values are not allowed here" or CoreDNS pods start with empty Corefile.

### Pitfall 4: ConfigMap Change Without Rollout Restart
**What goes wrong:** ConfigMap is updated successfully but the action returns immediately. The old Corefile remains active for 60-90 seconds until kubelet syncs the volume mount to the pod filesystem and the reload plugin picks it up.
**Why it happens:** Kubernetes ConfigMap volume mounts have a kubelet sync period (default 60s) before updated data appears in the pod's mounted file. The CoreDNS `reload` plugin then adds another 30-45s check cycle.
**How to avoid:** Always follow the ConfigMap apply with `kubectl rollout restart deployment/coredns -n kube-system` and wait for `kubectl rollout status`.
**Warning signs:** DNS-04 verification passes immediately after cluster creation, then breaks for ~90 seconds before recovering.

### Pitfall 5: Trailing Newline Causes Duplicate Empty Line in Corefile
**What goes wrong:** The `indentCorefile` helper adds a space to the trailing newline of the Corefile, causing a spurious `    ` line at the end. CoreDNS may log a warning about unexpected empty directives.
**Why it happens:** `strings.Split(corefile, "\n")` on a file ending with `\n` produces a final empty string element.
**How to avoid:** In the `indentCorefile` helper, only add indentation to non-empty lines: `if line != "" { lines[i] = "    " + line }`. Leave empty lines as empty strings.
**Warning signs:** CoreDNS logs show unexpected parse warnings; the ConfigMap Corefile has trailing whitespace lines.

### Pitfall 6: Multiple Occurrences of "cache 30" in Corefile
**What goes wrong:** If there are multiple `cache 30` occurrences (e.g., in a custom stub zone block), all are replaced, potentially changing cache behavior for non-external zones.
**Why it happens:** `strings.ReplaceAll` replaces all occurrences, not just the top-level one.
**How to avoid:** In the default kind Corefile, there is only one `cache 30` — the top-level directive. Add a guard that verifies `strings.Count(corefileStr, "cache 30") == 1` and fail if there are multiple occurrences. For local dev clusters, this is extremely unlikely.
**Warning signs:** More than one `cache 60` appears in the resulting Corefile.

## Code Examples

Verified patterns from codebase and official sources:

### Default kind Corefile (confirmed via official Kubernetes docs)
```
# Source: https://v1-32.docs.kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/
.:53 {
    errors
    health { lameduck 5s }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
        pods insecure
        fallthrough in-addr.arpa ip6.arpa
        ttl 30
    }
    prometheus :9153
    forward . /etc/resolv.conf
    cache 30
    loop
    reload
    loadbalance
}
```

### Target Corefile After Tuning
```
.:53 {
    errors
    health { lameduck 5s }
    ready
    autopath @kubernetes
    kubernetes cluster.local in-addr.arpa ip6.arpa {
        pods verified
        fallthrough in-addr.arpa ip6.arpa
        ttl 30
    }
    prometheus :9153
    forward . /etc/resolv.conf
    cache 60
    loop
    reload
    loadbalance
}
```

### Complete Execute Function Skeleton
```go
// Source: pattern derived from installcni/cni.go (read + modify + apply) + installmetricsserver/metricsserver.go (wait)
package installcorednstuning

import (
    "bytes"
    "fmt"
    "strings"

    "sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
    "sigs.k8s.io/kind/pkg/cluster/nodeutils"
    "sigs.k8s.io/kind/pkg/errors"
)

type action struct{}

// NewAction returns a new action for tuning CoreDNS
func NewAction() actions.Action {
    return &action{}
}

// Execute runs the action
func (a *action) Execute(ctx *actions.ActionContext) error {
    ctx.Status.Start("Tuning CoreDNS")
    defer ctx.Status.End(false)

    allNodes, err := ctx.Nodes()
    if err != nil {
        return errors.Wrap(err, "failed to list cluster nodes")
    }
    controlPlanes, err := nodeutils.ControlPlaneNodes(allNodes)
    if err != nil {
        return errors.Wrap(err, "failed to find control plane nodes")
    }
    if len(controlPlanes) == 0 {
        return errors.New("no control plane nodes found")
    }
    node := controlPlanes[0]

    // Step 1: Read current Corefile from ConfigMap
    var corefile bytes.Buffer
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "get", "configmap", "coredns",
        "--namespace=kube-system",
        "-o", "jsonpath={.data.Corefile}",
    ).SetStdout(&corefile).Run(); err != nil {
        return errors.Wrap(err, "failed to read CoreDNS Corefile from ConfigMap")
    }
    corefileStr := corefile.String()

    // Step 2: Guard - verify expected directives exist
    if !strings.Contains(corefileStr, "pods insecure") {
        return errors.New("CoreDNS Corefile does not contain 'pods insecure'; cannot patch safely")
    }
    if !strings.Contains(corefileStr, "cache 30") {
        return errors.New("CoreDNS Corefile does not contain 'cache 30'; cannot patch safely")
    }
    if !strings.Contains(corefileStr, "kubernetes cluster.local") {
        return errors.New("CoreDNS Corefile does not contain 'kubernetes cluster.local'; cannot patch safely")
    }

    // Step 3: Apply string transforms
    // Transform 1: pods insecure -> pods verified (prerequisite for autopath)
    corefileStr = strings.ReplaceAll(corefileStr, "pods insecure", "pods verified")
    // Transform 2: Insert autopath @kubernetes before kubernetes block
    corefileStr = strings.ReplaceAll(
        corefileStr,
        "    kubernetes cluster.local",
        "    autopath @kubernetes\n    kubernetes cluster.local",
    )
    // Transform 3: Increase external cache TTL from 30s to 60s
    corefileStr = strings.ReplaceAll(corefileStr, "cache 30", "cache 60")

    // Step 4: Write back updated ConfigMap via kubectl apply
    configMapYAML := fmt.Sprintf(configMapTemplate, indentCorefile(corefileStr))
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "apply", "-f", "-",
    ).SetStdin(strings.NewReader(configMapYAML)).Run(); err != nil {
        return errors.Wrap(err, "failed to apply updated CoreDNS ConfigMap")
    }

    // Step 5: Rolling restart so new Corefile takes effect immediately
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "rollout", "restart",
        "--namespace=kube-system",
        "deployment/coredns",
    ).Run(); err != nil {
        return errors.Wrap(err, "failed to restart CoreDNS deployment")
    }

    // Step 6: Wait for rollout to complete
    if err := node.Command(
        "kubectl",
        "--kubeconfig=/etc/kubernetes/admin.conf",
        "rollout", "status",
        "--namespace=kube-system",
        "deployment/coredns",
        "--timeout=60s",
    ).Run(); err != nil {
        return errors.Wrap(err, "CoreDNS rollout did not complete after config update")
    }

    ctx.Status.End(true)
    return nil
}

const configMapTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
%s
`

// indentCorefile adds 4 spaces to each non-empty Corefile line for YAML block scalar indentation.
func indentCorefile(corefile string) string {
    lines := strings.Split(corefile, "\n")
    for i, line := range lines {
        if line != "" {
            lines[i] = "    " + line
        }
    }
    return strings.Join(lines, "\n")
}
```

### autopath Plugin Syntax (Official CoreDNS Docs)
```
# Source: https://coredns.io/plugins/autopath/
# autopath syntax: autopath [ZONE...] RESOLV-CONF
# For Kubernetes integration: autopath @kubernetes
# @kubernetes means "use the kubernetes plugin's pod namespace data"

autopath @kubernetes
```

### cache Plugin TTL Syntax
```
# Source: https://coredns.io/plugins/cache/
# cache [TTL] [ZONES...]
# TTL is the maximum TTL in seconds for cached records (default: 3600 for NOERROR, 1800 for NXDOMAIN)
# Setting cache 60 means max 60s TTL for external queries (the default kind Corefile uses cache 30)

cache 60
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `pods insecure` (kube-dns compat mode) | `pods verified` (secure pod lookup) | Available since CoreDNS 1.x | Verified mode tracks actual pod IPs; insecure mode is provided only for backward compat |
| Manual `kubectl edit` to tune Corefile | Programmatic read-modify-write in Go | N/A (kind-specific pattern) | Enables automation without kubectl interactive mode |
| Wait for ConfigMap volume propagation (60-90s) | `kubectl rollout restart` (10-20s) | N/A (operational best practice) | Deterministic rollout vs. non-deterministic kubelet sync |
| Replace entire Corefile | Surgical string substitution | N/A | Safer; preserves site-specific customizations; satisfies DNS-01 "not replaced" requirement |

**Deprecated/outdated:**
- `pods insecure`: Maintained for backward compat with kube-dns. For new installs, `pods verified` is correct.
- Relying on `reload` plugin for ConfigMap changes: Works eventually (30-45s after kubelet syncs), but rollout restart is faster and more predictable for automated flows.

## Open Questions

1. **autopath @kubernetes interaction with cache plugin**
   - What we know: Community reports that autopath without a properly sized cache can cause a "huge upstream traffic increase" (GitHub issue #3765). The default `cache 30` (being changed to `cache 60`) provides basic caching. For a local dev cluster with low traffic, this is unlikely to be a problem.
   - What's unclear: Whether autopath + cache 60 combination causes any observable behavior change vs. no-autopath.
   - Recommendation: For a local dev cluster, the traffic impact is negligible. Proceed with the implementation as planned. Document in user-facing output that autopath reduces DNS lookup latency.

2. **Windows node autopath incompatibility**
   - What we know: Official CoreDNS autopath docs note the plugin "is incompatible with Windows node pods."
   - What's unclear: Whether kind clusters run Windows node containers (they do not in standard usage).
   - Recommendation: Not applicable to kind clusters (Linux containers only). No special handling needed.

3. **CoreDNS Corefile format variation across kind versions**
   - What we know: The official kubeadm/kind default Corefile format is stable and consistently uses the exact tokens we're patching (`pods insecure`, `cache 30`, `kubernetes cluster.local`).
   - What's unclear: Whether future kind node images might change the Corefile indentation or token format.
   - Recommendation: The guard conditions (`strings.Contains` checks) will catch format changes and fail with a clear error message rather than silently producing a broken Corefile.

4. **DNS-04 verification approach**
   - What we know: `kubectl rollout status` waits until all CoreDNS pods are ready (readiness probe passes). This is a necessary but not sufficient condition for DNS resolution to work.
   - What's unclear: Whether DNS resolution could fail immediately after rollout completes due to any transient state from the `pods verified` change (which requires CoreDNS to build a pod cache).
   - Recommendation: Wait for rollout status. For local dev clusters, the pod cache builds within seconds of CoreDNS starting. The `pods verified` mode adds memory overhead but not significant startup delay.

## Sources

### Primary (HIGH confidence)
- `https://coredns.io/plugins/autopath/` — autopath plugin docs: syntax `autopath @kubernetes`, requires pods verified, Windows incompatibility
- `https://coredns.io/plugins/kubernetes/` — kubernetes plugin docs: pods insecure/verified/disabled modes, autopath @kubernetes requirement
- `https://coredns.io/plugins/cache/` — cache plugin docs: `cache [TTL]` syntax, max TTL semantics
- `https://coredns.io/plugins/reload/` — reload plugin docs: 30s default interval, 15s jitter, graceful in-place reload
- `https://v1-32.docs.kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/` — Official Kubernetes docs: default kind/kubeadm Corefile format (confirmed `pods insecure`, `cache 30`, all plugin list)
- Kinder codebase: `pkg/cluster/internal/create/actions/installcni/cni.go` — bytes.Buffer + SetStdout pattern for reading from node command
- Kinder codebase: `pkg/cluster/internal/create/actions/installmetallb/metallb.go` — strings.NewReader + SetStdin for apply -f -
- Kinder codebase: `pkg/cluster/internal/create/actions/installcorednstuning/corednstuning.go` — stub confirmed; package, NewAction pattern
- Kinder codebase: `pkg/cluster/internal/create/create.go:201` — `runAddon("CoreDNS Tuning", opts.Config.Addons.CoreDNSTuning, ...)` wiring confirmed

### Secondary (MEDIUM confidence)
- `https://sleeplessbeastie.eu/2024/01/15/how-to-alter-coredns-configuration/` (Jan 2024) — Get/modify/apply + rollout restart pattern confirmed as the correct operational approach
- `https://github.com/coredns/coredns/issues/3765` — autopath + cache interaction: warns about traffic increase without cache; confirmed cache should accompany autopath
- `https://github.com/coredns/coredns/issues/1752` — Community confirmation that `pods verified` is required for autopath @kubernetes
- `https://kubernetes.io/docs/concepts/configuration/configmap/` — ConfigMap volume propagation: 60-90s delay with default kubelet sync period

### Tertiary (LOW confidence)
- Community sources confirming `kubectl rollout restart` is the standard pattern after CoreDNS ConfigMap patching — consistent across multiple cloud provider docs (EKS, AKS, DigitalOcean) but procedure varies slightly per provider

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all kubectl commands and Go stdlib patterns verified against codebase and official docs; no external dependencies
- Architecture: HIGH — read-modify-write pattern directly mirrors installcni (SetStdout/bytes.Buffer) and installmetallb (SetStdin/strings.NewReader); the only novel element is rollout restart
- Corefile format: HIGH — default format confirmed via official Kubernetes docs (v1-32)
- Pitfalls: HIGH for autopath position, pods verified requirement, YAML indentation (all from official CoreDNS docs); MEDIUM for rollout restart timing (operational experience, not doc-verified)
- ConfigMap propagation: HIGH — official Kubernetes docs confirm 60-90s kubelet sync delay; rollout restart as workaround is widely confirmed

**Research date:** 2026-03-01
**Valid until:** 2026-04-01 (CoreDNS Corefile format is very stable; Go patterns are stable)
