# Project Milestones: Kinder

## v1.0 Batteries Included (Shipped: 2026-03-01)

**Delivered:** Forked kind into kinder with 5 default addons (MetalLB, Envoy Gateway, Metrics Server, CoreDNS tuning, Headlamp dashboard) that work out of the box and can be individually disabled via config.

**Phases completed:** 1-8 (12 plans total)

**Key accomplishments:**
- Binary renamed to `kinder` with backward-compatible v1alpha4 config schema extended with `addons` section
- MetalLB auto-detects Docker/Podman/Nerdctl subnet and assigns LoadBalancer IPs without user input
- Envoy Gateway installed with full wait chain for end-to-end Gateway API routing
- Metrics Server, CoreDNS tuning, and Headlamp dashboard all install automatically with printed access instructions
- Each addon individually disableable via `addons.<name>: false` in cluster config
- Integration test suite validates all 5 addons functional together

**Stats:**
- 65 files created/modified
- ~1,950 lines of Go (addon actions)
- 8 phases, 12 plans, 36 commits
- 1 day from start to ship

**Git range:** `feat(01-01)` → `fix(08-02)`

**What's next:** TBD — potential v1.1 with cert-manager, NodeLocal DNSCache, or Prometheus stack

---
