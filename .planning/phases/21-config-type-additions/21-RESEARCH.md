# Phase 21: Config Type Additions - Research

**Researched:** 2026-03-03
**Domain:** Go config pipeline — v1alpha4 public API, internal types, conversion, defaults, deep copy, YAML test fixtures
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CFG-01 | Add LocalRegistry *bool to v1alpha4 Addons struct with default true | Identical pattern already used for MetalLB, EnvoyGateway, MetricsServer, CoreDNSTuning, Dashboard in the same struct |
| CFG-02 | Add CertManager *bool to v1alpha4 Addons struct with default true | Same pattern; both fields added together in a single struct edit |
| CFG-03 | Wire both fields through internal config types, conversion, and defaults (5 locations) | Five locations identified and mapped in Architecture Patterns section |
</phase_requirements>

## Summary

Phase 21 is a pure config-pipeline extension. The codebase already has five identical addon fields (`MetalLB`, `EnvoyGateway`, `MetricsServer`, `CoreDNSTuning`, `Dashboard`) added at v1.0 using exactly the same `*bool` pointer pattern. Adding `LocalRegistry` and `CertManager` requires touching the same five locations in exactly the same way, making this the lowest-risk phase in v1.3.

The five pipeline locations are: (1) `pkg/apis/config/v1alpha4/types.go` — public struct definition; (2) `pkg/apis/config/v1alpha4/default.go` — v1alpha4-layer pointer defaulting; (3) `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` — deepcopy for the public type; (4) `pkg/internal/apis/config/types.go` — internal struct definition (plain bool, no pointer); (5) `pkg/internal/apis/config/convert_v1alpha4.go` — boolVal conversion from *bool to bool. No changes are needed to `pkg/internal/apis/config/default.go` (internal defaults do not touch Addons fields — that defaulting is done at the v1alpha4 layer before conversion) and no changes are needed to `pkg/internal/apis/config/zz_generated.deepcopy.go` (internal Addons uses only plain bools, so `*out = *in` is sufficient and already correct). The blocker note about CertManager's default being true vs false is a product decision locked before this phase starts.

**Primary recommendation:** Follow the existing MetalLB/Dashboard pattern exactly — five edits, one new YAML test fixture (valid-addons-new-fields.yaml), two new test assertions in `TestAddonsDefaults`, and two new `boolPtrTrue` calls in `v1alpha4/default.go`.

## Standard Stack

### Core

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Go standard library | 1.21.0 (min) | All config types are plain structs | No external deps; project constraint |
| go.yaml.in/yaml/v3 | v3.0.4 | YAML parsing of cluster config files | Already in go.mod; strict-field decoding enforced |

No new dependencies are introduced by this phase.

## Architecture Patterns

### The Five-Location Config Pipeline

The project uses a Kubernetes-style API versioning pattern: public versioned types (v1alpha4) with pointer fields for optionality, converted to internal types with concrete values after defaulting. Every addon field passes through exactly these five locations:

```
pkg/apis/config/v1alpha4/
  types.go              ← Location 1: *bool field on public Addons struct
  default.go            ← Location 2: boolPtrTrue defaulting to true
  zz_generated.deepcopy.go ← Location 3: nil-guard pointer copy

pkg/internal/apis/config/
  types.go              ← Location 4: bool field on internal Addons struct (no pointer)
  convert_v1alpha4.go   ← Location 5: boolVal() dereference in out.Addons assignment
```

### Pattern 1: Public API — *bool with yaml/json tags (Location 1)

**What:** All optional addon fields are `*bool` in the public v1alpha4 struct, with `omitempty` serialization tags.
**When to use:** Any new field that has a sensible default and is user-overridable.

```go
// Source: pkg/apis/config/v1alpha4/types.go (existing pattern)
type Addons struct {
    MetalLB       *bool `yaml:"metalLB,omitempty" json:"metalLB,omitempty"`
    EnvoyGateway  *bool `yaml:"envoyGateway,omitempty" json:"envoyGateway,omitempty"`
    MetricsServer *bool `yaml:"metricsServer,omitempty" json:"metricsServer,omitempty"`
    CoreDNSTuning *bool `yaml:"coreDNSTuning,omitempty" json:"coreDNSTuning,omitempty"`
    Dashboard     *bool `yaml:"dashboard,omitempty" json:"dashboard,omitempty"`
    // NEW — add after Dashboard:
    LocalRegistry *bool `yaml:"localRegistry,omitempty" json:"localRegistry,omitempty"`
    CertManager   *bool `yaml:"certManager,omitempty" json:"certManager,omitempty"`
}
```

### Pattern 2: v1alpha4 Defaulting — boolPtrTrue helper (Location 2)

**What:** A local helper sets `*bool` fields to `true` when nil, implementing the "on by default" opt-out semantics.
**When to use:** Every new `*bool` addon field gets a `boolPtrTrue` call in `SetDefaultsCluster`.

```go
// Source: pkg/apis/config/v1alpha4/default.go (existing pattern)
boolPtrTrue := func(b **bool) {
    if *b == nil {
        t := true
        *b = &t
    }
}
boolPtrTrue(&obj.Addons.MetalLB)
boolPtrTrue(&obj.Addons.EnvoyGateway)
boolPtrTrue(&obj.Addons.MetricsServer)
boolPtrTrue(&obj.Addons.CoreDNSTuning)
boolPtrTrue(&obj.Addons.Dashboard)
// NEW — add after Dashboard:
boolPtrTrue(&obj.Addons.LocalRegistry)
boolPtrTrue(&obj.Addons.CertManager)
```

### Pattern 3: DeepCopy — nil-guard pointer allocation (Location 3)

**What:** `zz_generated.deepcopy.go` is hand-maintained in this project (not code-generated). New `*bool` fields need nil-guard copy blocks identical to the existing ones.
**When to use:** Every new `*bool` field on a public type that has a deepcopy function.

```go
// Source: pkg/apis/config/v1alpha4/zz_generated.deepcopy.go (existing pattern)
if in.LocalRegistry != nil {
    in, out := &in.LocalRegistry, &out.LocalRegistry
    *out = new(bool)
    **out = **in
}
if in.CertManager != nil {
    in, out := &in.CertManager, &out.CertManager
    *out = new(bool)
    **out = **in
}
```

### Pattern 4: Internal Type — plain bool (Location 4)

**What:** Internal types use plain `bool` (no pointer), since all defaults have already been applied before conversion.
**When to use:** Internal Addons struct mirrors every public field but as a non-pointer.

```go
// Source: pkg/internal/apis/config/types.go (existing pattern)
type Addons struct {
    MetalLB       bool
    EnvoyGateway  bool
    MetricsServer bool
    CoreDNSTuning bool
    Dashboard     bool
    // NEW — add after Dashboard:
    LocalRegistry bool
    CertManager   bool
}
```

**Note:** The internal `zz_generated.deepcopy.go` does NOT need changes. The `DeepCopyInto` for internal `Addons` uses `*out = *in` which copies all plain bool fields by value automatically. Adding new plain bool fields is covered by this existing line.

### Pattern 5: Conversion — boolVal dereference (Location 5)

**What:** `Convertv1alpha4` maps public `*bool` fields to internal `bool` fields using the `boolVal` helper that returns `true` when nil.
**When to use:** Every new field pair (public `*bool` → internal `bool`) gets a new `boolVal()` call in the `out.Addons` literal.

```go
// Source: pkg/internal/apis/config/convert_v1alpha4.go (existing pattern)
boolVal := func(b *bool) bool {
    if b == nil {
        return true
    }
    return *b
}

out.Addons = Addons{
    MetalLB:       boolVal(in.Addons.MetalLB),
    EnvoyGateway:  boolVal(in.Addons.EnvoyGateway),
    MetricsServer: boolVal(in.Addons.MetricsServer),
    CoreDNSTuning: boolVal(in.Addons.CoreDNSTuning),
    Dashboard:     boolVal(in.Addons.Dashboard),
    // NEW — add after Dashboard:
    LocalRegistry: boolVal(in.Addons.LocalRegistry),
    CertManager:   boolVal(in.Addons.CertManager),
}
```

### Recommended File Change Map

```
pkg/apis/config/v1alpha4/
  types.go                      ← add 2 fields to Addons struct
  default.go                    ← add 2 boolPtrTrue calls
  zz_generated.deepcopy.go      ← add 2 nil-guard copy blocks

pkg/internal/apis/config/
  types.go                      ← add 2 plain bool fields to Addons
  convert_v1alpha4.go           ← add 2 boolVal() entries to out.Addons literal
  zz_generated.deepcopy.go      ← NO CHANGE NEEDED (plain bool, covered by *out = *in)
  default.go                    ← NO CHANGE NEEDED (internal defaults do not touch Addons)

pkg/internal/apis/config/encoding/testdata/v1alpha4/
  valid-addons-new-fields.yaml  ← NEW: test fixture with localRegistry: false, certManager: false

pkg/internal/apis/config/encoding/
  load_test.go                  ← add 1 new TestLoadCurrent case + assertions in TestAddonsDefaults
```

Total: 5 files modified, 1 new test fixture, ~30 lines of code added.

### Anti-Patterns to Avoid

- **Forgetting zz_generated.deepcopy.go for public types:** The v1alpha4 deepcopy IS required for new `*bool` fields because `*out = *in` only copies the pointer value (alias), not the pointed-to bool. The nil-guard blocks are mandatory for correct isolation.
- **Adding pointer fields to internal types:** Internal `Addons` uses plain `bool` by design. Do not add `*bool` to the internal struct — conversion handles the nil→true translation at the boundary.
- **Changing internal default.go:** The internal `SetDefaultsCluster` does not set Addons fields. Addons defaulting is done by `v1alpha4.SetDefaultsCluster` (the v1alpha4-layer defaults) before conversion. Adding Addons defaulting to the internal layer would double-apply and could override user intent.
- **Forgetting the YAML strict decoder:** The encoding package uses `d.KnownFields(true)` which means any unrecognized YAML key returns an error. New YAML field names must exactly match the `yaml:"..."` struct tags. Use camelCase: `localRegistry`, `certManager`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| nil-safe bool dereferencing | Custom nil check everywhere | `boolVal` helper already in convert_v1alpha4.go | Already tested, consistent behavior |
| Pointer defaulting | Custom if-chains | `boolPtrTrue` helper already in v1alpha4/default.go | Already tested, consistent behavior |
| DeepCopy for *bool | Manual clone | Nil-guard pattern from existing zz_generated.deepcopy.go | Proven pattern, avoids aliasing bugs |

**Key insight:** All helpers needed for this phase already exist in the codebase. This phase is purely additive — zero new patterns to design.

## Common Pitfalls

### Pitfall 1: Missing deepcopy for public *bool fields
**What goes wrong:** If `zz_generated.deepcopy.go` (v1alpha4) is not updated, the `DeepCopy()` called at the start of `Convertv1alpha4` will alias the new `*bool` pointers. Mutations to the original struct will affect the converted copy.
**Why it happens:** `*out = *in` copies the pointer address, not the pointed-to value. Only primitive types (not pointers) are safe with value-copy semantics.
**How to avoid:** Add nil-guard blocks for LocalRegistry and CertManager to `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go`.
**Warning signs:** Test that mutates the original config after conversion sees the mutation reflected in the converted result.

### Pitfall 2: YAML field name mismatch with strict decoder
**What goes wrong:** `go.yaml.in/yaml/v3` with `KnownFields(true)` rejects any unrecognized key. If the YAML tag says `yaml:"localRegistry"` but a test fixture uses `local_registry:`, the test returns an error.
**Why it happens:** The strict decoder is intentional (catches config typos). camelCase is the project convention for YAML keys.
**How to avoid:** Use `localRegistry` and `certManager` as YAML keys (matching existing addon naming convention).
**Warning signs:** `TestLoadCurrent` with the new fixture returns "cannot unmarshal" error.

### Pitfall 3: CertManager default value mismatch with blocker
**What goes wrong:** STATE.md blocker says "Confirm true/false default before phase begins — research recommends false (opt-in)." If the phase implements `default true` but the product decision lands on `false`, Phase 23 will need to revisit defaults.
**Why it happens:** Requirements CFG-01 and CFG-02 both specify "default true" but the blocker suggests this may change for CertManager.
**How to avoid:** The planner should confirm with the user before implementing CertManager's default. The phase goal says "both defaulting to true when nil" — proceed with true for both unless explicitly overridden.
**Warning signs:** Success criterion 1 says "both defaulting to true when nil" — this is the locked spec for Phase 21.

### Pitfall 4: Updating internal default.go when it shouldn't be touched
**What goes wrong:** Adding Addons defaulting to `pkg/internal/apis/config/default.go` creates double-defaulting. The internal `SetDefaultsCluster` is called AFTER `V1Alpha4ToInternal` (which already applied v1alpha4 defaults and converted). Adding it again there would be redundant at best, silently wrong at worst.
**Why it happens:** The two-layer defaulting is non-obvious — v1alpha4 defaults first, then internal conversion, then internal defaults for non-Addons fields.
**How to avoid:** Do not touch `pkg/internal/apis/config/default.go`. Verify: the existing `TestSetDefaultsCluster_AddonFields` in that package tests that Addons fields remain unchanged by the internal defaulter.
**Warning signs:** `TestSetDefaultsCluster_AddonFields` test starts testing Addons fields being modified.

## Code Examples

### Complete change for Location 1 — types.go

```go
// Source: pkg/apis/config/v1alpha4/types.go
type Addons struct {
    MetalLB       *bool `yaml:"metalLB,omitempty" json:"metalLB,omitempty"`
    EnvoyGateway  *bool `yaml:"envoyGateway,omitempty" json:"envoyGateway,omitempty"`
    MetricsServer *bool `yaml:"metricsServer,omitempty" json:"metricsServer,omitempty"`
    CoreDNSTuning *bool `yaml:"coreDNSTuning,omitempty" json:"coreDNSTuning,omitempty"`
    Dashboard     *bool `yaml:"dashboard,omitempty" json:"dashboard,omitempty"`
    // LocalRegistry enables a local container registry at localhost:5001.
    // +optional (default: true)
    LocalRegistry *bool `yaml:"localRegistry,omitempty" json:"localRegistry,omitempty"`
    // CertManager enables cert-manager with a self-signed ClusterIssuer.
    // +optional (default: true)
    CertManager *bool `yaml:"certManager,omitempty" json:"certManager,omitempty"`
}
```

### New YAML test fixture

```yaml
# Source: testdata/v1alpha4/valid-addons-new-fields.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
addons:
  localRegistry: false
  certManager: false
```

### TestAddonsDefaults assertions to add

```go
// Source: pkg/internal/apis/config/encoding/load_test.go
// In TestAddonsDefaults, after existing assertions:

// Test with new fields absent — both default to true
if !cfg.Addons.LocalRegistry {
    t.Error("expected LocalRegistry to default to true when addons section absent")
}
if !cfg.Addons.CertManager {
    t.Error("expected CertManager to default to true when addons section absent")
}

// Test new fields can be set to false
cfg4, err := Load("./testdata/v1alpha4/valid-addons-new-fields.yaml")
if err != nil {
    t.Fatalf("failed to load config with new addon fields disabled: %v", err)
}
if cfg4.Addons.LocalRegistry {
    t.Error("expected LocalRegistry to be false when explicitly set to false")
}
if cfg4.Addons.CertManager {
    t.Error("expected CertManager to be false when explicitly set to false")
}
```

## State of the Art

| Old Approach | Current Approach | Notes |
|--------------|------------------|-------|
| N/A — new fields | *bool in v1alpha4, bool in internal | Established since v1.0 Addons struct |

**Key insight for Phase 22 and 23:** After Phase 21, `opts.Config.Addons.LocalRegistry` and `opts.Config.Addons.CertManager` are available as plain `bool` fields in `pkg/cluster/internal/create/create.go`. Phase 22 adds `runAddon("Local Registry", opts.Config.Addons.LocalRegistry, ...)` and Phase 23 adds `runAddon("cert-manager", opts.Config.Addons.CertManager, ...)` — same `runAddon` helper already in create.go.

## Open Questions

1. **CertManager default: true or false?**
   - What we know: Requirements say "default true" (CFG-02); roadmap success criteria confirm "both defaulting to true when nil"; STATE.md blocker says "research recommends false (opt-in) to keep cluster creation fast; this is a product decision"
   - What's unclear: Whether the product decision has been made before Phase 21 starts
   - Recommendation: Implement `default true` as specified in requirements (CFG-01/CFG-02) and roadmap success criteria. The blocker is flagged for Phase 23 specifically, not Phase 21. Phase 21 only adds the config field; Phase 23 implements the action. If the default changes, it's a one-line edit to `v1alpha4/default.go` before Phase 23.

2. **Should the new YAML fixture test `localRegistry: false, certManager: false` in the same file or separately?**
   - What we know: Existing fixtures use minimal, focused configs (one fixture per scenario)
   - Recommendation: One file (`valid-addons-new-fields.yaml`) with both fields disabled is sufficient for TestLoadCurrent. No error is expected.

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `pkg/apis/config/v1alpha4/types.go` — existing Addons struct with 5 *bool fields
- Direct code inspection: `pkg/apis/config/v1alpha4/default.go` — boolPtrTrue pattern
- Direct code inspection: `pkg/apis/config/v1alpha4/zz_generated.deepcopy.go` — nil-guard pointer copy pattern
- Direct code inspection: `pkg/internal/apis/config/types.go` — internal Addons with plain bool fields
- Direct code inspection: `pkg/internal/apis/config/convert_v1alpha4.go` — boolVal conversion pattern
- Direct code inspection: `pkg/internal/apis/config/encoding/convert.go` — V1Alpha4ToInternal flow
- Direct code inspection: `pkg/internal/apis/config/encoding/load_test.go` — TestAddonsDefaults and TestLoadCurrent patterns
- Direct code inspection: `pkg/cluster/internal/create/create.go` — runAddon helper, opts.Config.Addons usage

### Secondary (MEDIUM confidence)
- `.planning/REQUIREMENTS.md` — CFG-01, CFG-02, CFG-03 descriptions
- `.planning/ROADMAP.md` — Phase 21 success criteria
- `.planning/STATE.md` — CertManager default blocker note

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; pure Go stdlib
- Architecture (five locations): HIGH — verified by direct code reading of all five files
- Pitfalls: HIGH — derived from direct code reading of deepcopy mechanism, strict decoder, and internal/external defaulting split
- Open questions: LOW (CertManager default) — product decision not yet made; implementation spec says true

**Research date:** 2026-03-03
**Valid until:** 2026-04-03 (stable Go config pipeline; no external dependencies changing)
