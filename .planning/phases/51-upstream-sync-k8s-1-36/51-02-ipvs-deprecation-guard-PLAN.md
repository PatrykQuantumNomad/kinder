---
phase: 51-upstream-sync-k8s-1-36
plan: 02
type: tdd
wave: 1
depends_on: []
files_modified:
  - pkg/internal/apis/config/validate.go
  - pkg/internal/apis/config/validate_test.go
autonomous: true

must_haves:
  truths:
    - "A cluster config with kubeProxyMode: ipvs is rejected at validation time when any node image is K8s 1.36+"
    - "The error message uses 'deprecated, will be removed' framing (not 'removed') and includes a migration URL"
    - "Pre-1.36 nodes with ipvs continue to validate successfully (no regression)"
    - "Non-semver image tags (e.g. latest) skip the guard rather than producing spurious errors"
  artifacts:
    - path: "pkg/internal/apis/config/validate.go"
      provides: "IPVS-vs-1.36 guard inside Cluster.Validate()"
      contains: "ipvs is not supported with Kubernetes 1.36"
    - path: "pkg/internal/apis/config/validate_test.go"
      provides: "Test cases proving ipvs+1.36 fails, ipvs+1.35 passes, ipvs+latest skips guard"
  key_links:
    - from: "pkg/internal/apis/config/validate.go"
      to: "imageTagVersion + version.ParseSemantic (already in same file)"
      via: "reuse existing helpers, do not introduce new parsers"
      pattern: "imageTagVersion"
    - from: "validate.go IPVS guard error"
      to: "https://kubernetes.io/docs/reference/networking/virtual-ips/"
      via: "URL embedded in error message for user actionability"
      pattern: "kubernetes.io/docs/reference/networking/virtual-ips"
---

<objective>
Add a config-time guard in `Cluster.Validate()` that rejects `kubeProxyMode: ipvs` when any node image is Kubernetes 1.36 or higher, with an actionable error message pointing to the iptables/nftables migration path.

Purpose: Protect users from creating a 1.36+ cluster with IPVS that will become increasingly unmaintained as Kubernetes deprecates IPVS mode (deprecated in v1.35, removal scheduled for a future release). Delivers SYNC-03 and SC3.

Output: New validation rule in `pkg/internal/apis/config/validate.go`; test coverage in `validate_test.go` proving the guard fires on 1.36+, passes on 1.35 and below, and gracefully skips on non-semver tags.

Framing note: IPVS is **deprecated, not yet removed** in 1.36 (RESEARCH §3 + §Pitfall 6). The error message MUST say "deprecated and will be removed in a future release," NOT "removed in 1.36." A hard rejection at validation time is justified by SC3 itself ("rejected at validation time with a clear error message"), but the message must be technically accurate.
</objective>

<execution_context>
@/Users/patrykattc/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/51-upstream-sync-k8s-1-36/51-RESEARCH.md

@pkg/internal/apis/config/validate.go
@pkg/internal/apis/config/validate_test.go
@pkg/internal/apis/config/types.go
</context>

<feature>
  <name>IPVS-on-1.36 deprecation guard</name>
  <files>pkg/internal/apis/config/validate.go, pkg/internal/apis/config/validate_test.go</files>
  <behavior>
Given a `Cluster` config:
- If `Networking.KubeProxyMode == IPVSProxyMode` AND any node's parsed semver minor version >= 36 → `Validate()` returns an error containing the substring "kubeProxyMode: ipvs is not supported with Kubernetes 1.36" AND the substring "deprecated" AND the migration URL "https://kubernetes.io/docs/reference/networking/virtual-ips/".
- If `Networking.KubeProxyMode == IPVSProxyMode` AND all nodes are pre-1.36 → no error introduced by this guard (existing validations still apply).
- If `Networking.KubeProxyMode != IPVSProxyMode` → no error introduced by this guard regardless of node versions.
- If `imageTagVersion` returns an error for any node, OR `version.ParseSemantic` fails (e.g. tag is `latest`) → that node is skipped, the loop continues. If NO node parses successfully, no error is added.
- The guard breaks out of the loop on first 1.36+ match — only one error per Cluster.

Cases (in `validate_test.go`):

1. `ipvs_rejected_on_1_36_node` — Cluster with `kubeProxyMode: ipvs` and a single node image `kindest/node:v1.36.0` → expect error matching `/ipvs is not supported with Kubernetes 1\.36/`.
2. `ipvs_rejected_on_mixed_with_one_1_36_node` — Cluster with `kubeProxyMode: ipvs` and two nodes (one v1.35.1, one v1.36.0) → expect error (the 1.36 node trips the guard).
3. `ipvs_passes_on_1_35_only_cluster` — Cluster with `kubeProxyMode: ipvs` and all nodes v1.35.1 → expect NO ipvs-related error (other validation errors are fine, just not THIS one).
4. `iptables_passes_on_1_36_node` — Cluster with `kubeProxyMode: iptables` and a node v1.36.0 → expect NO ipvs-related error.
5. `ipvs_skipped_on_non_semver_tag` — Cluster with `kubeProxyMode: ipvs` and a node image `kindest/node:latest` → expect NO ipvs-related error (guard gracefully skips).
6. `ipvs_error_includes_migration_url` — assert the error string from case 1 contains `https://kubernetes.io/docs/reference/networking/virtual-ips/`.
7. `ipvs_error_uses_deprecated_framing` — assert the error string from case 1 contains the word "deprecated" and does NOT contain the word "removed in 1.36" (it MAY contain "removed in a future release").

Use the existing test pattern in `validate_test.go` — the table-driven `TestClusterValidate` style. Each new case adds a row; the harness uses `errors.As` / substring matching consistent with the file's existing approach.
  </behavior>
  <implementation>
Add the guard inside `Cluster.Validate()` AFTER the existing `KubeProxyMode` check (RESEARCH §3 says lines 71–75 in current code; verify exact insertion point at execution time — it's right after the existing check that the value is one of the valid modes).

Use the exact code shape from RESEARCH §Code Examples ("IPVS version guard"):

```go
// IPVS-on-1.36+ guard: kube-proxy IPVS mode was deprecated in v1.35 and will
// be removed in a future release. Reject up-front rather than letting users
// create a cluster that may break on future patch releases.
if c.Networking.KubeProxyMode == IPVSProxyMode {
    for _, n := range c.Nodes {
        tag, err := imageTagVersion(n.Image)
        if err != nil {
            continue // version unknown, skip guard for this node
        }
        v, err := version.ParseSemantic(tag)
        if err != nil {
            continue // non-semver tag (e.g. "latest"), skip guard
        }
        if v.Minor() >= 36 {
            errs = append(errs, errors.Errorf(
                "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+ "+
                    "(node image %q uses v1.%d); kube-proxy IPVS mode was deprecated in v1.35 "+
                    "and will be removed in a future release. Switch to iptables or nftables. "+
                    "Migration guide: https://kubernetes.io/docs/reference/networking/virtual-ips/",
                n.Image, v.Minor()))
            break // one error is enough
        }
    }
}
```

Reuse existing imports — `imageTagVersion` is defined in this same file (RESEARCH §3 says line 276), and `sigs.k8s.io/kind/pkg/internal/version` (`version.ParseSemantic`) is already imported. Do NOT add any new dependency or import.

The `IPVSProxyMode` constant is defined in `pkg/internal/apis/config/types.go` — verify the exact identifier (likely `IPVSProxyMode`) before referencing.

If `errs` is not the local error-slice variable name in this function, adapt — use whatever `Cluster.Validate` already accumulates errors into.
  </implementation>
</feature>

<tdd_cycle>

**RED phase:**
1. Open `pkg/internal/apis/config/validate_test.go`, find the `TestClusterValidate` table-driven test (or equivalent) — read the existing test cases to learn the harness shape.
2. Add the seven test cases listed in `<behavior>` above. Use the same table style (one row per case, expecting either a specific error substring or no error).
3. Run `go test ./pkg/internal/apis/config/... -run TestClusterValidate -race` — the new cases MUST fail (because the guard doesn't exist yet).
4. Commit: `test(51-02): add failing tests for IPVS-on-1.36+ deprecation guard`

**GREEN phase:**
1. Open `pkg/internal/apis/config/validate.go`. Locate `func (c *Cluster) Validate()`.
2. Insert the guard from `<implementation>` immediately AFTER the existing `KubeProxyMode` validity check.
3. Run `go test ./pkg/internal/apis/config/... -race` — all cases (new + existing) MUST pass.
4. Run `go build ./...` to confirm no callers broke.
5. Commit: `feat(51-02): reject kubeProxyMode ipvs on Kubernetes 1.36+ at validation time`

</tdd_cycle>

<verification>
- `go test ./pkg/internal/apis/config/... -race` exits 0 with all 7 new cases passing.
- `go build ./...` exits 0.
- `grep -n "kubeProxyMode: ipvs is not supported" pkg/internal/apis/config/validate.go` returns one match.
- `grep -n "kubernetes.io/docs/reference/networking/virtual-ips" pkg/internal/apis/config/validate.go` returns one match.
- 2 commits landed on main: 1 RED + 1 GREEN.
</verification>

<success_criteria>
SC3: A cluster config with `kubeProxyMode: ipvs` is rejected at validation time with a clear error message pointing to the iptables migration path when the node version is 1.36 or higher — satisfied by the guard added to `Cluster.Validate()` and proven by tests covering ipvs+1.36 (rejected), ipvs+1.35 (passes), iptables+1.36 (passes), and non-semver-tag fallthrough.
</success_criteria>

<output>
After completion, create `.planning/phases/51-upstream-sync-k8s-1-36/51-02-SUMMARY.md` with: tasks executed, commits landed (SHAs), final test counts (`go test ./pkg/internal/apis/config/... -race -v` summary), and the exact text of the new error message as it appears in validate.go.
</output>
