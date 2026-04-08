# Phase 42: Multi-Version Node Validation - Context

**Gathered:** 2026-04-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can configure per-node Kubernetes versions and kinder validates version-skew correctness at config parse time instead of surfacing cryptic kubeadm errors after provisioning begins. Per-node `image:` fields override the global `--image` flag. `kinder get nodes` displays version info, and `kinder doctor` reports skew on running clusters.

</domain>

<decisions>
## Implementation Decisions

### Validation strictness
- Hard fail on version-skew violations — reject config and exit before any containers are created
- No `--force` or override flag — invalid skew is a hard error
- Match upstream Kubernetes skew policy exactly: workers can be up to 3 minor versions behind the control-plane
- No warnings for configs within policy — if it passes validation, output is clean (no "you're near the boundary" noise)
- HA control-plane version matching: Claude's Discretion on whether to enforce same minor or same patch

### Error presentation
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

### Version display format
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

### Doctor check behavior
- Severity level: Claude's Discretion — pick based on existing kinder doctor check patterns
- Version source: Claude's Discretion — pick the most reliable approach (live kubelet query vs image tag inspection)
- Use the same table format as create-time validation errors for consistency
- Check BOTH upstream skew policy AND config drift — warn if running versions differ from what was originally configured

### Claude's Discretion
- HA control-plane version strictness (same minor vs same patch)
- Doctor check severity level (warning vs error)
- Doctor version detection method (kubelet exec vs image tag inspection)
- Exact spacing and column widths in table output

</decisions>

<specifics>
## Specific Ideas

- Table format for errors was explicitly chosen over bullet lists — user wants structured, scannable output
- SKEW column with `✓`/`✗` markers preferred for terminal compatibility
- Config drift detection in doctor (actual vs configured) is a deliberate choice — catches unexpected state changes

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 42-multi-version-node-validation*
*Context gathered: 2026-04-08*
