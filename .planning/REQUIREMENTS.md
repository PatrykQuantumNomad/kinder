# Requirements: Kinder v2.4 Hardening

**Defined:** 2026-05-09
**Core Value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.

## v2.4 Requirements

Requirements for Hardening milestone. Each maps to roadmap phases. REQ-IDs continue numbering from v2.3 categories where applicable.

### Lifecycle

- [ ] **LIFE-09**: HA pause/resume preserves etcd peer connectivity across container IP reassignment via IP pinning (`docker network connect --ip <stored-ip>`); cert regen is documented fallback if Docker IPAM is infeasible

### Addons

- [ ] **ADDON-01**: `local-path-provisioner` bumped to v0.0.36 (CVE GHSA-7fxv-8wr2-mfc4 HelperPod Template Injection security fix); embedded busybox pin retained
- [ ] **ADDON-02**: `Headlamp` bumped to v0.42.0 with mandatory plan-time token-print flow verification against a real cluster (hold at v0.40.1 if broken)
- [ ] **ADDON-03**: `cert-manager` bumped to v1.20.2 with self-signed ClusterIssuer smoke covering UID change (1000→65532); `--server-side` apply pattern preserved (988 KB still > 256 KB annotation limit)
- [ ] **ADDON-04**: `Envoy Gateway` bumped to v1.7.2 (jumps two major lines) with dedicated HTTPRoute UAT covering readiness port, access log format, xDS snapshot behavior, and Gateway API CRD version lock
- [ ] **ADDON-05**: All bumped image references propagated to `pkg/internal/doctor/offlinereadiness.go` `allAddonImages` list; `TestAllAddonImages_CountMatchesExpected` updated; `RequiredAddonImages` returns the new image set

### Upstream Sync

- [ ] **SYNC-05**: Default `kindest/node` bumped to K8s 1.36.x conditional on Docker Hub publication (Plan probes Docker Hub at start; executes Plan 51-04 Task 2 atomically if image present; marks SKIPPED with documented re-runnable status if still gated)

### Distribution

- [ ] **DIST-01**: macOS GoReleaser artifacts ad-hoc signed via `codesign --force --sign -` post-hook running on `macos-latest` CI runner; signing happens AFTER `-ldflags="-s -w"` strip; release notes explicitly state ad-hoc signing does NOT bypass Gatekeeper quarantine
- [ ] **DIST-02**: PR CI gains a blocking `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` step on `ubuntu-latest`; failure fails the PR check

### Tech Debt

- [ ] **DEBT-04**: Pre-existing data race in `pkg/internal/doctor/check_test.go` and `pkg/internal/doctor/socket_test.go` (`allChecks` global mutated under `t.Parallel()`) eliminated; production read path remains lock-free; tests use scoped helper `runChecks(checks []Check)` rather than mutating package state

### Diagnostics

- [ ] **DIAG-05**: `cluster-node-skew` doctor check skips containers with role `external-load-balancer` (no `/kind/version` file present); ListInternalNodes role filter applied
- [ ] **DIAG-06**: `cluster-resume-readiness` doctor check parses `etcdctl endpoint health --write-out json` output (`[{endpoint,health,took}]` per member) and reports actionable reason text (e.g. "1/3 healthy, quorum at risk") instead of dumping raw etcdctl error

### User Acceptance

- [ ] **UAT-01**: Phase 47 live HA UAT closed — 3 CP + 2 worker + LB cluster smoke verifies pause/resume worker→CP→LB ordering, host CPU/RAM observation, cluster-state round-trip (pods/PVCs/services), and `cluster-resume-readiness` warn on quorum loss; rebuilt `./bin/kinder version` verified before run; evidence in `hack/uat-47-ha-smoke.sh` + log
- [ ] **UAT-02**: Phase 51 live UAT closed — `docker ps` confirms `envoyproxy/envoy` (not `kindest/haproxy`) on real HA cluster; `kinder create cluster --config <ipvs+1.36>` rejected at validate with migration URL in error; K8s 1.36 guide page renders with sidebar entry; rebuilt binary verified before run

## Documented Holds

These addons remain pinned at current versions. Documented in `.planning/research/STACK.md` with rationale.

| Addon | Current pin | Rationale |
|-------|-------------|-----------|
| MetalLB | v0.15.3 | Latest available; no v0.16 released |
| Metrics Server | v0.8.1 | Latest stable as of 2026-01-29; no v0.9.x |
| registry (local registry) | registry:2 (floating tag = 2.8.3) | v3 deprecated storage drivers; kind ecosystem on v2 |
| CoreDNS | (in-cluster) | Tuned at runtime; no version pin |

## v2.5+ Requirements

Deferred to future release. Acknowledged but not in v2.4 scope.

### Lifecycle (multi-provider parity)

- **LIFE-10**: `kinder pause`/`kinder resume` parity for podman provider
- **LIFE-11**: `kinder pause`/`kinder resume` parity for nerdctl provider
- **LIFE-12**: `kinder dev` parity for podman + nerdctl

### Snapshot

- **SNAP-01**: Snapshot remote-storage backend (S3/GCS/registry push/pull)

### Distribution

- **DIST-03**: Full macOS notarization (Apple Developer cert + `notarytool submit` + stapled artifact) for Gatekeeper-clean first-run

### Doctor

- **DIAG-07**: Etcd peer-TLS regen via `kubeadm certs renew etcd-peer` as fallback escape hatch (if v2.4 IP-pinning approach proves insufficient in field)

## Out of Scope

Explicitly excluded for v2.4. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| macOS notarization | Requires paid Apple Developer cert + notarytool secrets in CI; explicit scope cut |
| `kinder upgrade` | No in-place addon-version upgrade for existing clusters; users recreate |
| Helm-based addon manager | Existing decision (PROJECT.md) — go:embed + kubectl apply preserved |
| Unconditional etcd cert regen on every resume | FEATURES research flags as anti-feature (5–15s overhead, partial-failure risk); IP pinning chosen instead |
| Envoy Gateway downgrade path | One-way bump to v1.7.2; users on older clusters recreate |
| cert-manager v1.16.5 patch | Skipped in favor of v1.20.2 major bump |
| Strict-Windows runtime support | DIST-02 is build-only; running tests cross-compiled on Windows is not a v2.4 promise |
| Addon CRD migrations | If an upgrade requires CRD migration, the addon is held instead (e.g. MetalLB legacy ConfigMap format would block bump) |

## Traceability

Empty initially; populated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| LIFE-09 | TBD | Pending |
| ADDON-01 | TBD | Pending |
| ADDON-02 | TBD | Pending |
| ADDON-03 | TBD | Pending |
| ADDON-04 | TBD | Pending |
| ADDON-05 | TBD | Pending |
| SYNC-05 | TBD | Pending |
| DIST-01 | TBD | Pending |
| DIST-02 | TBD | Pending |
| DEBT-04 | TBD | Pending |
| DIAG-05 | TBD | Pending |
| DIAG-06 | TBD | Pending |
| UAT-01 | TBD | Pending |
| UAT-02 | TBD | Pending |

**Coverage:**
- v2.4 requirements: 14 total
- Mapped to phases: 0 (filled by roadmapper)
- Unmapped: 14 ⚠️

---
*Requirements defined: 2026-05-09*
*Last updated: 2026-05-09 after initial definition*
