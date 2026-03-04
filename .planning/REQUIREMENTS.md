# Requirements: Kinder

**Defined:** 2026-03-04
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## v1.5 Requirements

Requirements for the website use cases & documentation milestone. Each maps to roadmap phases.

### Tutorials

- [ ] **GUIDE-01**: User can follow a step-by-step tutorial to deploy a web app with TLS using Local Registry, Envoy Gateway, cert-manager, and MetalLB
- [ ] **GUIDE-02**: User can follow a tutorial to set up HPA auto-scaling and watch pods scale under load using Metrics Server
- [ ] **GUIDE-03**: User can follow a local dev workflow tutorial: code, build, push to localhost:5001, deploy, iterate

### CLI Reference

- [ ] **CLI-01**: User can read a profile comparison guide showing what each --profile preset enables and when to use it
- [ ] **CLI-02**: User can read a JSON output reference with --output json examples and jq filters for all 4 read commands
- [ ] **CLI-03**: User can read a troubleshooting guide for kinder env and kinder doctor with exit codes and solutions

### Addon Enhancements

- [ ] **ADDON-01**: MetalLB page has practical examples (custom services, LB vs NodePort guidance) and troubleshooting
- [ ] **ADDON-02**: Envoy Gateway page has routing examples (path, header) and troubleshooting
- [ ] **ADDON-03**: Metrics Server page has kubectl top examples, basic HPA reference, and troubleshooting
- [ ] **ADDON-04**: CoreDNS page has DNS verification examples and troubleshooting
- [ ] **ADDON-05**: Headlamp page has dashboard navigation guide and troubleshooting
- [ ] **ADDON-06**: Local Registry page has multi-image workflow, cleanup, and troubleshooting
- [ ] **ADDON-07**: cert-manager page has additional certificate examples and troubleshooting

### Foundation Updates

- [ ] **FOUND-01**: Landing page Comparison component lists all 7 addons; description meta updated
- [ ] **FOUND-02**: Quick-start page verifies all 7 addons and mentions --profile flag
- [ ] **FOUND-03**: Configuration page documents all 7 addon fields including localRegistry and certManager
- [ ] **FOUND-04**: Sidebar has Guides and CLI Reference sections with all new pages

## Future Requirements

### Advanced Guides

- **GUIDE-04**: User can follow a CI pipeline tutorial using --profile ci
- **GUIDE-05**: User can follow a multi-service tutorial deploying frontend + backend + database

### Site Enhancements

- **SITE-01**: Animated terminal demo on landing page showing cluster creation
- **SITE-02**: Contributing guide for kinder development

## Out of Scope

| Feature | Reason |
|---------|--------|
| Video tutorials | Maintenance burden; text tutorials are more searchable and updatable |
| Interactive playground | Impossible with Docker dependency; fake demos break trust |
| API reference (Go docs) | Internal API; users interact via CLI only |
| Versioned documentation | No breaking changes yet; single version sufficient |
| Blog section | Overhead for a single-maintainer project; changelog covers releases |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FOUND-01 | Phase 30 | Pending |
| FOUND-02 | Phase 30 | Pending |
| FOUND-03 | Phase 30 | Pending |
| FOUND-04 | Phase 30 | Pending |
| ADDON-01 | Phase 31 | Pending |
| ADDON-02 | Phase 31 | Pending |
| ADDON-03 | Phase 31 | Pending |
| ADDON-04 | Phase 31 | Pending |
| ADDON-05 | Phase 31 | Pending |
| ADDON-06 | Phase 31 | Pending |
| ADDON-07 | Phase 31 | Pending |
| CLI-01 | Phase 32 | Pending |
| CLI-02 | Phase 32 | Pending |
| CLI-03 | Phase 32 | Pending |
| GUIDE-01 | Phase 33 | Pending |
| GUIDE-02 | Phase 33 | Pending |
| GUIDE-03 | Phase 33 | Pending |

**Coverage:**
- v1.5 requirements: 17 total
- Mapped to phases: 17
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-04*
*Last updated: 2026-03-04 after initial definition*
