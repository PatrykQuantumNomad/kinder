# Pitfalls Research

**Domain:** Kubernetes-in-Docker (kinder v2.4 Hardening — brownfield maintenance milestone)
**Researched:** 2026-05-09
**Confidence:** HIGH (project-specific; grounded in v2.3 audit record, codebase review, and known landmines)

---

## Critical Pitfalls

### Pitfall 1: [ETCD-TLS] Regenerating peer certs while etcd is running causes quorum loss and possible data corruption

**What goes wrong:**
Any attempt to overwrite or rotate etcd peer TLS certificates while the etcd process is live and the cluster is in `Running` state will cause at least one etcd member to reject peer connections mid-flight. With a 3-node CP cluster this drops to 1 healthy peer, which is below quorum. etcd then refuses all writes. The API server begins returning 503s or 429s for write operations. If the data-dir is touched during this window (e.g., to atomically swap the cert), etcd may journal a partial write on resume.

**Why it happens:**
Developers see kubeadm's `alpha certs renew` path and assume it is safe to run live. It is safe for serving/client certs (kube-apiserver, kubelet) but NOT peer certs, because etcd reloads peer TLS on SIGHUP or restart only. There is no live-reload path for `peer.crt`/`peer.key` in etcd 3.5.

**How to avoid:**
1. Gate the entire cert-regen flow on ALL control-plane containers being `Stopped` (lifecycle.Pause idempotent call). Never proceed if `docker inspect` reports any CP node as `running`.
2. Regen order: stop LB → stop workers → stop all CP nodes → regen certs → start CP nodes → verify `etcdctl endpoint health` → start workers → start LB.
3. Regen must cover ALL CP nodes atomically in the same pass. A helper that iterates over `cluster.ListInternalNodes()` filtered to role `control-plane` is the correct source-of-truth. Never hard-code node count.
4. After regen, verify peer connectivity before starting any worker. Use `crictl exec <etcd-container> etcdctl endpoint health --cluster` exactly as the `cluster-resume-readiness` check does.
5. Assert that the CA cert is NOT being replaced. Only leaf peer certs rotate. Replacing the CA destroys all kubeconfigs and every controller's trust chain. Add an explicit test asserting CA serial number is unchanged before and after.

**Warning signs:**
- `etcdctl endpoint health` reports `unhealthy` for any member after cert regen.
- `docker logs <cp-node>` shows `x509: certificate is valid for <old-IP>, not <new-IP>` on peer port 2380.
- API server returns 503 with `etcdserver: request timed out` after cluster resume.
- `kinder doctor cluster-resume-readiness` fires `WARN` before the fix is complete.

**Phase to address:** Phase 52 (etcd peer-TLS regeneration). Must be the first phase of v2.4 or a standalone phase with zero other concurrent changes. Do not combine with addon bumps.

---

### Pitfall 2: [ETCD-TLS] Partial SAN coverage during IP transition creates split-brain

**What goes wrong:**
Each peer cert embeds the SANs it will accept connections on (both IP and DNS forms). If node A's cert is regenerated with new IPs before node B's cert, node B will reject node A's new cert (SAN mismatch on old cert, old IP now gone). Even a brief asymmetry causes split-brain during the transition window.

**Why it happens:**
Developers regen certs one node at a time to "be careful", not realising that etcd's peer TLS is mutual — both endpoints verify each other's cert against the cluster's peer CA, and asymmetry is worse than a full stop.

**How to avoid:**
Require all-stopped state before any cert write. Generate all peer certs for all CP nodes in a single atomic pass before starting any etcd. The implementation must loop over `ListInternalNodes(role=control-plane)`, generate all certs, write all certs, then start all CP nodes together. Never write one cert and test before writing the others.

For the transition window (old Docker IP → new Docker IP after bridge recreation), include both old and new IPs as SANs only if an atomic all-stopped swap is not possible. But prefer the atomic path — include only the new IPs to keep cert surface minimal.

**Warning signs:**
- Cluster hangs at `kinder resume` after cert regen with only some nodes coming up.
- `docker logs <cp-node-2>` shows `rejected connection from <cp-node-1>: remote error: tls: bad certificate`.
- `etcdctl member list` returns fewer members than expected.

**Phase to address:** Phase 52 (same as Pitfall 1). Must be handled in the same plan as SAN generation — not split across plans.

---

### Pitfall 3: [ETCD-TLS] CA private key may not be accessible at host-filesystem level

**What goes wrong:**
Peer cert regeneration requires the etcd CA private key (`ca.key`). Inside a kind/kinder node container, this file lives at `/etc/kubernetes/pki/etcd/ca.key`. If the container is stopped, the file is accessible only via `docker cp` from the stopped container's filesystem or via a temporary container. Developers forget that a stopped container's filesystem is still readable via `docker cp` and either fail the step or — worse — try to start the container to access the file, which re-starts etcd and invalidates the "all-stopped" precondition.

**How to avoid:**
Use `docker cp <stopped-container>:/etc/kubernetes/pki/etcd/ca.key /tmp/etcd-ca.key` to extract the CA key without starting the container. After regen, copy the new peer cert back with `docker cp /tmp/peer.crt <container>:/etc/kubernetes/pki/etcd/peer.crt`. Write a test that mocks this copy pattern with FakeCmd. Never start a container to access its filesystem.

**Warning signs:**
- Plan steps include `docker start <cp-node>` before cert regen — this is the failure mode.
- `docker cp` returning a non-zero exit code from a stopped container (rare, but check).

**Phase to address:** Phase 52.

---

### Pitfall 4: [ETCD-TLS] kindest/node version may not include kubeadm certs subcommand

**What goes wrong:**
`kubeadm alpha certs renew` was promoted from alpha in K8s 1.21 and exists in all supported versions. However, `kubeadm certs generate-csr` (needed if regenerating from a CSR flow) is only stable from 1.24+. Kinder supports nodes as old as the pinned default image. If a user runs `kinder etcd-regen` on an older cluster and the implementation shells into kubeadm, it may get a "unknown command" error.

**How to avoid:**
Do NOT rely on kubeadm for peer cert regen. Generate peer certs using `crypto/x509` and `crypto/tls` in Go directly, or shell to `openssl` inside the node container if Go cert generation is out of scope. Either approach is version-independent. Add a version probe (read `/kind/version` inside the node) and hard-fail if it cannot be determined. Document the minimum supported K8s version for this operation.

**Warning signs:**
- `kubeadm certs renew etcd-peer` exits non-zero with "unknown command" on K8s < 1.24 nodes.
- Integration test passes on 1.35.x but fails on 1.30.x clusters.

**Phase to address:** Phase 52.

---

### Pitfall 5: [ADDON] Bumping all 7 addons in one phase multiplies break modes

**What goes wrong:**
When cert-manager, Envoy Gateway, Headlamp, MetalLB, Metrics Server, local-path-provisioner, and the base node image are all bumped in the same commit wave, a single test failure becomes ambiguous — any one of 7 version changes could be the root cause. CI time increases because the cluster must be torn down and re-created for each attempt. Rollback is harder because you must bisect 7 independent version variables.

**How to avoid:**
Bump each addon in its own plan (one plan per addon), with its own TDD RED→GREEN cycle, its own isolated integration test, and its own offline-readiness check update. Sequence them from lowest-risk (local-path-provisioner, Metrics Server) to highest-risk (cert-manager, Envoy Gateway). Gate each bump on a passing `make test` before proceeding to the next. SYNC-02 (K8s node image bump) should be its own plan gated by the Docker Hub probe exactly as done in v2.3 plan 51-04.

**Warning signs:**
- A plan title contains more than one addon name.
- A PR diff touches more than two `addons.go` or `manifests/` subdirectories.

**Phase to address:** Phase 53 (addon version bumps). Use sub-plans 53-01 through 53-07 sequentially.

---

### Pitfall 6: [ADDON] cert-manager CRD annotation limit hit during manifest replace

**What goes wrong:**
cert-manager CRDs exceed 256 KB when applied with `kubectl apply` using the last-applied-configuration annotation. This is a known upstream issue from v1.16 onwards. The exact error is `metadata.annotations: Too long: must have at most 262144 bytes`. If the apply step in `installcertmanager/` does not use `--server-side`, the CRD apply will fail silently (returns error, addon skips with warn-and-continue) and cert-manager will appear installed but CRDs will be at the old version.

**How to avoid:**
The `--server-side` flag is already a locked decision from v2.3. Verify it is present in the `installcertmanager` action manifest-apply shell call. Add an integration test assertion that `kubectl get crd certificates.cert-manager.io -o jsonpath='{.spec.versions[0].name}'` returns the expected new version after a fresh `kinder create cluster`. Do NOT rely on the warn-and-continue path — assert success explicitly.

**Warning signs:**
- `kubectl get crd | grep cert-manager` shows old version after create.
- Docker logs for the cert-manager action show `Too long` annotation error.
- `kinder doctor offline-readiness` passes but cert-manager pods fail `CrashLoopBackOff` on startup with `v1alpha2: no kind is registered`.

**Phase to address:** Phase 53, plan 53-03 (cert-manager bump). Pre-flight: add an assertion test for `--server-side` flag presence before writing any new manifest version string.

---

### Pitfall 7: [ADDON] cert-manager webhook startup race after major version bump

**What goes wrong:**
cert-manager ships a ValidatingWebhookConfiguration that intercepts Certificate and ClusterIssuer creation. After a major manifest replace (v1.16 → v1.20), the webhook pod takes 10-30 seconds to become Ready. Any ClusterIssuer created by kinder's post-install step during this window will return a webhook admission error (`failed calling webhook "webhook.cert-manager.io"`), causing the addon install step to exit non-zero and the cluster to proceed without a working issuer.

**How to avoid:**
Add an explicit wait after manifest apply: `kubectl rollout status deployment/cert-manager-webhook -n cert-manager --timeout=120s`. This already exists in the current `installcertmanager` action (check the exact timeout). After the major bump, verify the timeout is still adequate — cert-manager v1.20 webhook startup may take longer than v1.16 due to new validation logic. If kinder creates any default ClusterIssuer, defer its creation to after the rollout-status wait.

**Warning signs:**
- `kinder create cluster` completes but `kubectl get clusterissuer` returns `Not Ready` or 404.
- Docker logs show `failed calling webhook` during addon wave 1.
- `cert-manager-webhook` pod shows `ContainerCreating` for more than 60 seconds.

**Phase to address:** Phase 53, plan 53-03.

---

### Pitfall 8: [ADDON] Envoy Gateway Gateway API CRD version locked to GW version

**What goes wrong:**
Envoy Gateway v1.3.x ships and requires Gateway API CRDs v1.0.0. Envoy Gateway v1.7.x requires Gateway API CRDs v1.2.x (HTTPRoute `v1` promotions, GRPCRoute additions). Bumping the Envoy Gateway manifest without bumping the Gateway API CRD manifest will cause `installenvoygw` to appear to succeed but all HTTPRoutes will fail admission with `no kind is registered for the type HTTPRoute in group gateway.networking.k8s.io`. The CRDs are installed separately from the Envoy Gateway manifests.

**How to avoid:**
For each Envoy Gateway version bump (1.3 → 1.5 → 1.7), look up the exact Gateway API CRD version in the Envoy Gateway release notes (`https://gateway.envoyproxy.io/releases/`). Bump both in the same plan. Add an integration assertion: after addon install, `kubectl get crd httproutes.gateway.networking.k8s.io -o jsonpath='{.spec.versions[*].name}'` must include `v1`. Never bump Envoy Gateway version without bumping the companion CRD version string.

**Warning signs:**
- `kubectl apply -f <envoy-gw-manifest>` returns errors about unknown resource types.
- `kubectl get httproute` returns `error: the server doesn't have a resource type "httproute"`.
- Envoy Gateway controller pod logs show `failed to list *v1.HTTPRoute: the server could not find the requested resource`.

**Phase to address:** Phase 53, plan 53-02 (Envoy Gateway bump). Use staged bumps: 1.3→1.5 first, validate, then 1.5→1.7.

---

### Pitfall 9: [ADDON] MetalLB speaker DaemonSet upgrade race + legacy CRD format

**What goes wrong:**
MetalLB >= v0.13 migrated from ConfigMap-based config to `IPAddressPool` and `L2Advertisement` CRDs. If any test fixture, integration test, or documentation still references the old `config` ConfigMap format, it will silently pass the install step (ConfigMap is created) but MetalLB speaker will ignore it (speakers only read CRDs in v0.13+). Additionally, during DaemonSet upgrades, the speaker pods are replaced node-by-node. On a single-node cluster (typical CI), there is a brief window where no speaker is running — any test that creates a LoadBalancer service during this window will see `<pending>` external-ip indefinitely.

**How to avoid:**
Audit every occurrence of MetalLB config in `pkg/cluster/internal/create/actions/installmetallb/` for ConfigMap references and remove them. The IPAddressPool CRD path must be the only config path. For DaemonSet upgrade race: after the manifest apply, add `kubectl rollout status daemonset/speaker -n metallb-system --timeout=120s` before any service-creation assertions in tests. Do not test LoadBalancer IP assignment until the rollout status returns 0.

**Warning signs:**
- `kubectl get ipaddresspool -n metallb-system` returns 404 (CRD not installed after bump).
- Services of type LoadBalancer stay `<pending>` after a bump but worked before.
- `kubectl get cm config -n metallb-system` exists (leftover ConfigMap — should not be used).

**Phase to address:** Phase 53, plan 53-04 (MetalLB bump). Add a legacy-config-format sweep as the first step of the plan.

---

### Pitfall 10: [ADDON] Headlamp token auth flow change breaks printed-token wiring

**What goes wrong:**
Headlamp v0.41 introduced an optional MCP server for AI assistants and changed the ServiceAccount token bootstrap flow. In v0.40.x, kinder's `installdashboard` action prints a token obtained via `kubectl create token`. In v0.41+, if the Headlamp chart switches to a `TokenRequest` API with a short-lived token or a UI-based login flow, the printed token will either be empty, wrong, or expire within 10 minutes, making the "Access your dashboard at..." message misleading.

**How to avoid:**
Before writing the v0.41 bump plan, check Headlamp's changelog and helm chart values for `serviceAccount.token.type` or similar. If the token flow changed, update `installdashboard.go`'s token-print step to match. Add a test that asserts the printed token is non-empty after install. If the token flow is now UI-only, remove the token print and update the success message.

**Warning signs:**
- `kinder create cluster` prints a token but `kubectl auth can-i --token=<printed> get pods` returns `no`.
- Headlamp pod logs show `invalid token` for requests using the printed value.
- Headlamp chart README mentions `--set auth.type=...` in v0.41 release notes.

**Phase to address:** Phase 53, plan 53-05 (Headlamp bump).

---

### Pitfall 11: [ADDON] Metrics Server `--kubelet-insecure-tls` flag deprecation

**What goes wrong:**
Metrics Server is moving toward dropping `--kubelet-insecure-tls` in favor of CA injection. If a Metrics Server bump moves to a version where `--kubelet-insecure-tls` is removed or renamed, the deployment will crash with `unknown flag: --kubelet-insecure-tls` in container args. In a kinder cluster (self-signed certs, no external CA), the flag is mandatory for HPA to work — removing it without a CA-injection alternative breaks HPA test fixtures silently (HPA creates but never scales).

**How to avoid:**
Check the Metrics Server release notes for the target version. If `--kubelet-insecure-tls` still exists, verify it is present in the manifest patch in `installmetricsserver/`. If deprecated, implement the CA injection alternative (inject the kubelet serving CA from the node's `/var/lib/kubelet/pki/kubelet.crt` via ConfigMap or direct cert-manager issuer). Add an integration test that creates an HPA and verifies `kubectl get hpa` shows `TARGETS` populated (not `<unknown>`).

**Warning signs:**
- Metrics Server pod `CrashLoopBackOff` with `unknown flag` in logs.
- `kubectl top nodes` returns `error: metrics not available yet` more than 3 minutes after cluster creation.
- HPA shows `TARGETS: <unknown>/50%` after the bump.

**Phase to address:** Phase 53, plan 53-06 (Metrics Server bump).

---

### Pitfall 12: [ADDON] local-path-provisioner busybox pin breaks in air-gap after manifest bump

**What goes wrong:**
The local-path-provisioner manifest references a `helperImage` (busybox). The version in the manifest may change between minor releases. In air-gap mode, kinder pre-pulls a specific set of images. If the manifest bump changes the busybox tag (e.g., `busybox:1.35` → `busybox:1.36` or adds a digest pin), the air-gap pre-pull list in `offlinereadiness.go` will be stale and busybox will fail to pull at PVC-provision time — not at cluster-creation time. The cluster appears healthy but PVCs never bind.

**How to avoid:**
After updating the `installlocalpath` manifest, grep the new manifest for every image reference (`image:`, `helperImage:`) and update `offlinereadiness.go` to match. Add an assertion in the `installlocalpath` integration test that extracts the `helperImage` from the live ConfigMap after install and compares it to the value in `offlinereadiness.go`. This is the same offline-readiness coupling documented for Envoy in v2.3 plan 51-01.

**Warning signs:**
- `kubectl get pvc` shows `Pending` after cluster creation when local-path is the storage class.
- `kubectl describe pod <local-path-provisioner>` shows `ErrImagePull` for the helper image.
- `kinder doctor offline-readiness` passes but PVCs fail (offline list is stale).

**Phase to address:** Phase 53, plan 53-07 (local-path-provisioner bump).

---

### Pitfall 13: [SYNC-02] Image bump probe passes at validate time but image is pulled mid-create

**What goes wrong:**
Plan 51-04's gating probe checks Docker Hub for `kindest/node:v1.36.x` existence before proceeding. The same probe strategy must be used in v2.4. The failure mode is: the probe succeeds (image exists on Hub), cluster creation begins, and the `provider.Provision()` step starts pulling the new image. If the image is partially uploaded to Docker Hub (or a CDN edge has stale data), the pull will fail mid-provision, leaving containers in a broken state. The `--retain` flag preserves the broken cluster but the image.go default is now pointing at a broken tag.

**How to avoid:**
After the probe confirms the image exists, perform a manifest digest verification: `docker manifest inspect kindest/node:v1.36.x --verbose | jq '.[0].SchemaV2Manifest.config.digest'` must return a non-empty SHA. This confirms the image is fully pushed, not just a tag pointing at an incomplete manifest. Only then write the new default in `image.go`. Gate the entire plan on this two-step probe (existence + digest).

**Warning signs:**
- `docker pull kindest/node:v1.36.x` exits 0 for the index but subsequent `docker run` fails with `image not found`.
- `docker manifest inspect` returns a manifest with zero layers.
- CI create-cluster step hangs at `Pulling image...` for more than 5 minutes.

**Phase to address:** Phase 53 (or re-run of plan 51-04 from v2.3) — SYNC-02 carry-forward.

---

### Pitfall 14: [SYNC-02] Stale version strings in offline-readiness, website guide, and IPVS guard

**What goes wrong:**
When `defaults/image.go` is changed from `v1.35.x` to `v1.36.x`, there are at least three other places that must be updated atomically:
1. `offlinereadiness.go` — the node image in the offline pre-pull list must match the new default.
2. `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` — the placeholder `kinder v0.X or later` version reference.
3. `validate.go` IPVS guard — the minimum K8s version for IPVS rejection is 1.36+; if the default image version is bumped to 1.36, verify the guard fires correctly on a default cluster config with `kubeProxyMode: ipvs`.

Missing any of these three causes documentation drift (users follow the website guide but get wrong output), CI regressions (offline-readiness check fires on a cluster created with the new default), or a silent guard bypass.

**How to avoid:**
Create a grep-based pre-submit test (or a plain `go test` that reads the version constants from `image.go` and asserts the same version string appears in `offlinereadiness.go`). For the website placeholder: convert it to a Go constant reference or a template variable during plan 51-04 re-run so the placeholder cannot persist past the next bump. For the IPVS guard: add a test case with `image: kindest/node:v1.36.x` + `kubeProxyMode: ipvs` to `TestIPVSDeprecationGuard`.

**Warning signs:**
- `kinder doctor offline-readiness` warns `image kindest/node:v1.35.1 not pre-pulled` on a fresh cluster (version mismatch).
- Website guide shows `kinder v0.X or later` after the release note is published.
- `TestIPVSDeprecationGuard` does not have a v1.36 test case.

**Phase to address:** Phase 53 (SYNC-02 plan), addressed in the same commit as `image.go` change.

---

### Pitfall 15: [SIGNING] Symbol-strip before ad-hoc signing invalidates the signature

**What goes wrong:**
GoReleaser's build step applies `-ldflags="-s -w"` (strip debug info and DWARF) as part of the build. If `codesign -s -` (ad-hoc sign) is applied in a `post_hooks` or `signs` step AFTER the binary is already built and stripped, this ordering is correct. However, if any post-sign manipulation occurs — such as a `upx` compression step, a binary copy that resets xattrs, or a subsequent `strip` call — the Mach-O signature block is invalidated and macOS will refuse to run the binary with `Killed: 9`.

**How to avoid:**
Ad-hoc sign must be the LAST operation on each binary before packaging. GoReleaser hook order: build (with `-s -w`) → sign (`codesign -s - --force --preserve-metadata=entitlements <binary>`) → archive (tar.gz). No UPX, no strip, no binary copy after sign. Verify by running `codesign -vvv <binary>` on the archived binary after extraction — it must return `satisfies its Designated Requirement`. Add this verification as a CI step.

**Warning signs:**
- `codesign -vvv kinder` after extraction returns `code object is not signed at all`.
- macOS shows `"kinder" is damaged and can't be opened` on first run.
- GoReleaser logs show sign step completing but `codesign --verify` fails in the next CI job.

**Phase to address:** Phase 54 (macOS ad-hoc signing).

---

### Pitfall 16: [SIGNING] GoReleaser fan-out: only one architecture signed, the other silently skipped

**What goes wrong:**
GoReleaser builds `darwin/amd64` and `darwin/arm64` as separate artifacts. If the `signs` block uses a glob that accidentally matches only one artifact (e.g., `artifacts: archive` applies to tar.gz but not the inner binary, or a path glob resolves to only arm64), the amd64 binary ships unsigned. Users on Intel Macs see `Killed: 9` or Gatekeeper quarantine errors that amd64 users falsely assume are a Gatekeeper bypass requirement.

**How to avoid:**
Use `artifacts: binary` in the GoReleaser `signs` block (not `archive`) to sign each architecture-specific binary before archiving. Verify by listing GoReleaser's signing artifacts: `goreleaser build --snapshot --clean` then check `dist/` for both `kinder_darwin_amd64_v1/kinder` and `kinder_darwin_arm64/kinder`, and run `codesign -vvv` on each. Add a CI matrix step that verifies signatures for both architectures separately.

**Warning signs:**
- `codesign -vvv dist/kinder_darwin_amd64_v1/kinder` returns `not signed` while arm64 is signed.
- Release notes only mention arm64 signing.
- GoReleaser `--debug` output shows sign command running once, not twice.

**Phase to address:** Phase 54.

---

### Pitfall 17: [SIGNING] Ad-hoc signature does NOT bypass Gatekeeper for quarantined binaries

**What goes wrong:**
Ad-hoc signing (`codesign -s -`) satisfies macOS's signature format requirement but does NOT add a Gatekeeper-approved certificate chain. Binaries downloaded via browser, `curl`, or direct tarball download will have the `com.apple.quarantine` extended attribute set by macOS. Gatekeeper will still block them on first run with `"kinder" cannot be opened because the developer cannot be verified`. Developers ship ad-hoc signing expecting it to resolve all user complaints about "permission denied" or quarantine blocks — it only resolves the `Killed: 9` (signature format) issue, not the Gatekeeper trust issue.

**How to avoid:**
Document this clearly in the release notes and install instructions. For direct tarball installs, provide the `xattr -d com.apple.quarantine kinder` one-liner prominently. For Homebrew Cask installs, note that brew does NOT set the quarantine bit on installed binaries, so ad-hoc signing is most useful for tarball/direct-download users. Do not claim ad-hoc signing "solves" Gatekeeper. This is a communication pitfall, not a code pitfall — address it in the v2.4 website copy.

**Warning signs:**
- Issue tracker receives reports of `cannot be opened because the developer cannot be verified` even after ad-hoc signing ships.
- Release notes say "signed binary" without distinguishing ad-hoc from Developer ID / Notarization.

**Phase to address:** Phase 54. Add a "Known Limitations" section to the signing PR description and release notes.

---

### Pitfall 18: [WINDOWS-CI] cgo transitive dependencies break Windows cross-compile

**What goes wrong:**
kind and kinder have historically been cgo-free on the main binary path. However, transitive dependencies introduced in recent versions (fsnotify v1.10 added in v2.3, potential go-libcontainer paths in provider code) may pull in cgo-required C libraries that are available on Linux/macOS but not on Windows cross-compile (`CGO_ENABLED=0`). The `go build` command for Windows (`GOOS=windows go build`) will succeed if the cgo path is never reached (conditional build tags exclude it), but fail with `cgo: C compiler "x86_64-w64-mingw32-gcc" not found` if any transitive import touches C code.

**How to avoid:**
Before adding the Windows CI job, run `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` locally and in CI. If it fails, audit `go build -v -x` output to identify the offending import chain. fsnotify v1.10 on Windows uses the Win32 ReadDirectoryChangesW API via `golang.org/x/sys/windows` — no cgo — so it should be safe. The risk is in provider code pulling in `opencontainers/runc` or `moby/sys/mountinfo` which do have cgo paths on Linux. Use `//go:build !windows` tags to exclude those paths.

**Warning signs:**
- `go build ./... 2>&1 | grep "cgo"` returns results under `GOOS=windows CGO_ENABLED=0`.
- Windows CI job fails at compile step with `C compiler not found`.
- `go list -f '{{.CgoFiles}}' ./...` returns non-empty results for any package.

**Phase to address:** Phase 55 (Windows CI). Run the cross-compile probe as the very first step of the plan before writing any CI YAML.

---

### Pitfall 19: [WINDOWS-CI] Build-only check masks runtime path separator failures

**What goes wrong:**
Windows uses `\` as path separator; Go's `filepath.Join` handles this correctly, but string concatenation (`path + "/" + file`) does not. Additionally, `exec.Command("docker", "run", ...)` on Windows resolves binaries differently (`.exe` suffix, PATH resolution). A `go build` check for Windows passes even if the binary would crash or produce wrong paths at runtime. Test fixtures that hardcode `/tmp/kinder-...` paths will compile on Windows but produce `C:\Windows\Temp\kinder-...` or fail silently.

**How to avoid:**
Run `go vet ./...` under `GOOS=windows` to catch obvious issues. Audit every string path construction in `pkg/cluster/internal/create/`, `pkg/internal/doctor/`, and `pkg/internal/snapshot/` for `/`-concatenation patterns. Replace with `filepath.Join`. For the CI job itself: add a `go vet` step for Windows explicitly, and document in the CI YAML that this is a compile+vet check only (no runtime). Mark the Windows CI job as informational (non-blocking) for v2.4 — block only on Linux/macOS.

**Warning signs:**
- `grep -r '"/' pkg/ | grep -v '_test.go'` returns many results (potential hardcoded Unix paths).
- `go vet ./...` under `GOOS=windows` shows `suspect use of filepath separator` lints.
- Any test using `os.TempDir()` and then constructing paths with `/` will fail on Windows.

**Phase to address:** Phase 55.

---

### Pitfall 20: [DEBT-04] Mutex held during check execution serializes doctor run

**What goes wrong:**
The DEBT-04 race is a mutation-under-parallel race on the `allChecks` global slice in `check_test.go` and `socket_test.go`. The naive fix is to add a `sync.RWMutex` around all `allChecks` access. If the lock is acquired with `Lock()` (write lock) in `RunAllChecks()` and held for the duration of all check executions, the entire doctor run becomes serialized and the lock is held for seconds. This defeats the purpose and introduces a risk of lock inversion if any check itself calls `RunAllChecks()` (not currently the case, but a future pitfall).

**How to avoid:**
The race is a test-time mutation problem, not a production-code mutation problem. The correct fix is narrower: in test code, replace direct `allChecks = append(...)` mutations with a test helper that saves and restores `allChecks` around each test (`t.Cleanup(func() { allChecks = saved })`). For production code, `allChecks` is initialized at package load and never mutated — it does not need a mutex at all. The `sync.OnceValues` precedent from v1.4 applies here: use it for lazy initialization if the registry needs to be lazily populated, but do NOT add a mutex to the read path. Verify the fix by running `go test -race ./pkg/internal/doctor/... -count=10`.

**Warning signs:**
- PR diff shows a `sync.RWMutex` added to the `allChecks` variable declaration (too broad).
- `go test -race ./pkg/internal/doctor/... -count=10` still reports races after the fix.
- `kinder doctor` takes noticeably longer after the fix (serialization regression).

**Phase to address:** Phase 56 (DEBT-04 race fix).

---

### Pitfall 21: [DOCTOR] cluster-node-skew filter skips LB nodes incorrectly

**What goes wrong:**
The `cluster-node-skew` doctor check uses `ListInternalNodes()` to find nodes. If the filter for the LB role excludes nodes before skew calculation, a setup where the LB container was accidentally created with a different kindest/node image version (possible after a partial upgrade or manual intervention) will pass the skew check silently. Conversely, if the filter is too broad and includes the LB node (which does not run kubelet), the skew check will try to read a kubelet version from an LB container and fail with a confusing error.

**How to avoid:**
Use `ListInternalNodes()` (not `ListNodes()`) to exclude the LB role from skew calculation — this is the existing correct pattern. Verify by inspecting the filter chain in the skew check implementation. Add a test case for the edge case where the cluster has 3 CP nodes + 1 LB node, and assert that the LB node does not appear in the skew calculation output. Do NOT add a separate role filter on top of `ListInternalNodes()` — it already excludes LB.

**Warning signs:**
- `kinder doctor cluster-node-skew` returns an error mentioning `external-load-balancer` node.
- The check output includes `lb` in the node list.
- After Envoy LB swap (v2.3), if any code references `kindest/haproxy` in the skew logic, it is stale.

**Phase to address:** Phase 57 (doctor cosmetic fixes).

---

### Pitfall 22: [DOCTOR] etcdctl JSON shape varies across versions — defensive parsing required

**What goes wrong:**
The `cluster-resume-readiness` check calls `etcdctl endpoint health --write-out=json` and parses the output. The etcd 3.4.x JSON shape is `[{"endpoint":"...","health":true,"took":"..."}]` (lowercase keys). The etcd 3.5.x shape adds an `error` field and may use capitalized key names in some builds. If the JSON parser uses a strict struct without `omitempty` or without lowercase JSON tags, a shape mismatch will cause the parser to silently return `health: false` for all endpoints, causing the check to always warn even on healthy clusters.

**How to avoid:**
Use a defensive struct with both capitalized and lowercase JSON alternatives via a custom `UnmarshalJSON`, OR parse into `[]map[string]interface{}` and check for `"health"` key case-insensitively. After the fix, add test cases for both the 3.4.x and 3.5.x JSON shapes (use string fixtures, no real etcd needed). The v2.3 audit note mentions this exact TODO at `pkg/internal/snapshot/etcd.go:24` — this is the same JSON parsing surface.

**Warning signs:**
- `kinder doctor cluster-resume-readiness` shows `WARN: etcd endpoint unhealthy` on a healthy cluster.
- Manually running `etcdctl endpoint health --write-out=json` in the node returns uppercase keys.
- After a kindest/node version bump, etcdctl version changes and the JSON shape shifts.

**Phase to address:** Phase 57 (doctor cosmetic fixes).

---

### Pitfall 23: [UAT-LIVE] Phase 47 + 51 UAT drift — recordings and smoke scripts go stale

**What goes wrong:**
The v2.3 audit deferred live UAT for phases 47 and 51. If these are carried into v2.4 and the live UAT includes asciinema recordings or bash smoke scripts, any subsequent change to CLI output format (e.g., the `cluster-resume-readiness` output wording changes in a cosmetic fix) will make the recording misleading. Worse, if the live HA smoke script is not run on the actual rebuilt binary (the audit noted `bin/kinder` and `/opt/homebrew/bin/kinder` were stale), it will pass against old code.

**How to avoid:**
Before running any live UAT for phases 47/51:
1. `make build` and confirm `bin/kinder version` shows the v2.4 build hash.
2. Run the smoke script against `./bin/kinder`, not `kinder` from PATH.
3. Do NOT record asciinema during the UAT itself — the recording is documentation, not a test gate. Record separately after UAT passes on clean output.
4. The live HA smoke for phase 47 requires 3 CP + 2 workers + 1 LB = 6 Docker containers minimum. On a 16 GB laptop, run `docker stats --no-stream` before creating the cluster to confirm >4 GB free RAM. If not available, use a CI runner with 32 GB.

**Warning signs:**
- `bin/kinder version` output does not match expected v2.4 build hash before UAT.
- `kinder pause` output format differs from what the smoke script `grep`s for.
- Docker Desktop shows `>90% memory pressure` during HA cluster creation.

**Phase to address:** Phase 58 (Phase 47 + 51 live UAT closure). This is a milestone-closure phase, not a code phase.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Bump all addons in one PR | Single CI run | Ambiguous failures, hard rollback | Never for v2.4 (7 addons = 7 plans) |
| Skip `--server-side` assertion test for cert-manager CRD | Saves one integration test | Silent CRD downgrade if flag regresses | Never — add the assertion once |
| Use `sync.Mutex` around entire `allChecks` read path | Trivially "fixes" the race | Serializes all doctor checks; lock inversion risk | Never — fix at test scope only |
| Skip offline-readiness update when bumping local-path | Faster per-addon plan | Air-gap users get `ErrImagePull` on PVC creation | Never — always update together |
| Ad-hoc sign only arm64 (skip amd64) | Less CI complexity | Intel Mac users see `Killed: 9` | Never |
| Build-only Windows CI (no vet) | Fast CI job | Misses path separator bugs | Acceptable for v2.4 first pass, escalate to vet+test in v2.5 |
| etcd peer cert regen with cluster running | No downtime | Quorum loss, possible data corruption | Never |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Envoy GW + Gateway API CRDs | Bump Envoy GW manifest without bumping companion CRD version | Always look up the required GW API CRD version in Envoy GW release notes and bump together |
| cert-manager CRD + `--server-side` | Apply without `--server-side`, silently fail on >256KB annotation | Assert `--server-side` flag in action code; add integration test verifying CRD version post-install |
| local-path busybox + offlinereadiness | Bump manifest, forget to update offline pre-pull list | Grep new manifest for all image references; update offlinereadiness.go in the same commit |
| etcd peer TLS + pause/resume lifecycle | Regen certs while any CP container is running | Gate regen on `lifecycle.Pause` completing; verify all CP containers stopped before writing any cert |
| GoReleaser signing + symbol strip | Strip symbols after signing | Build → strip (via `-ldflags="-s -w"`) → sign → archive; sign is always last operation |
| IPVS guard + SYNC-02 image default | Update `image.go` without updating IPVS guard test matrix | Add `v1.36.x` test case to `TestIPVSDeprecationGuard` in the same plan as `image.go` change |
| doctor `allChecks` + `t.Parallel()` | Add mutex to production read path | Fix at test scope only — save/restore `allChecks` around parallel tests |
| Phase 47/51 UAT + stale binary | Run smoke against PATH `kinder` (old build) | Always rebuild with `make build` and use `./bin/kinder` for UAT |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| HA smoke on a laptop during v2.4 UAT | Docker Desktop OOM kill mid-cluster-create | Verify >4 GB free RAM before HA smoke; use CI runner if unavailable | 6-container HA cluster on 8-16 GB machines |
| All 7 addon bumps in one test run | 30+ minute CI cycle for a single failing addon | One-plan-per-addon; each plan runs its own isolated create/test/delete | Every combined addon run |
| etcd cert regen with live cluster | API server 503s during cert write window | All-stopped gate; never regen live | Any attempt at live rotation |
| Doctor run serialized by over-broad mutex | `kinder doctor` takes 30s instead of 5s | Fix race at test scope; no mutex on read path | Immediately if mutex is added to RunAllChecks loop |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Exposing etcd CA private key on host filesystem during cert regen | CA key readable by any host process while temporarily extracted | Use `docker cp` to extract, write to `0600` temp file, delete immediately after cert generation; never write to `/tmp` without restricted mode |
| Shipping binary without ad-hoc signature as "signed" | macOS quarantine users see `Killed: 9`; false sense of security | Verify `codesign -vvv` on each artifact in CI; block release if any arch fails |
| Windows CI job using elevated permissions | Privilege escalation surface in CI | Run Windows CI as a standard user; do not add `--privileged` or `sudo` to CI steps |
| ad-hoc signing conflated with notarization in user docs | Users believe binary is Apple-vetted; security expectations mismatched | Explicitly document "ad-hoc only; not notarized" in install guide |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| `kinder etcd-regen` (or equivalent) with no warning about cluster downtime | Users trigger it on a running cluster thinking it is hot | Print `WARNING: all cluster containers will be stopped for certificate regeneration` and require `--confirm` flag |
| Printing a Headlamp token that has expired or changed format in v0.41 | Users paste the token and get `401 Unauthorized` | Either remove the token print if the flow changed, or print with an expiry warning |
| Ad-hoc signing release note says "signed" without qualification | Users expect Gatekeeper bypass; file bug when quarantine still blocks | Release note must say: "ad-hoc signed (not notarized); Homebrew install unaffected; direct download requires `xattr -d com.apple.quarantine`" |
| `cluster-resume-readiness` WARN fires on every single-CP cluster (if LB-role filter is wrong) | Warning fatigue; users ignore all doctor output | Verify LB-role filter at test time with a single-CP fixture; warn must NOT fire on healthy single-CP |

---

## "Looks Done But Isn't" Checklist

- [ ] **etcd peer-TLS regen**: Verify `codesign -vvv` — but more importantly verify `etcdctl endpoint health --cluster` returns all members healthy after cert regen AND cluster restart. A cluster that creates successfully but has one peer using old certs will fail on the next `kinder pause/resume` cycle.
- [ ] **cert-manager bump**: `kubectl get crd certificates.cert-manager.io -o jsonpath='{.spec.versions[0].name}'` must return the new API version, not the old one. CRD `kubectl apply --server-side` success does NOT guarantee the version was updated.
- [ ] **Envoy GW + Gateway API CRD**: `kubectl get crd httproutes.gateway.networking.k8s.io -o jsonpath='{.spec.versions[*].name}'` must include `v1` (not just `v1beta1`).
- [ ] **MetalLB bump**: `kubectl get ipaddresspool -n metallb-system` must return the pool kinder created (not 404, not old ConfigMap).
- [ ] **offline-readiness after any addon bump**: `kinder doctor offline-readiness` must NOT warn on a freshly created cluster with default settings.
- [ ] **SYNC-02**: `kinder create cluster` with no `--image` flag must pull the new v1.36.x image, not the old v1.35.x image.
- [ ] **Windows CI**: `go build` passing is not sufficient — also run `go vet ./...` under `GOOS=windows`.
- [ ] **ad-hoc signing**: `codesign -vvv ./bin/kinder` must return `valid on disk` AND `satisfies its Designated Requirement`, not just `code object is not signed` (which means the step was skipped).
- [ ] **DEBT-04 race fix**: `go test -race ./pkg/internal/doctor/... -count=100` must show zero races. A single clean run is insufficient — the race is timing-dependent.
- [ ] **Phase 47/51 live UAT**: Smoke must be run on the binary built from the v2.4 HEAD commit, confirmed by `./bin/kinder version` matching the expected build hash.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| etcd quorum lost during cert regen attempt | HIGH | Stop all CP containers; restore from pre-pause etcd snapshot (`/kind/pause-snapshot.json` from phase 47) to the data-dir; regen all peer certs atomically; restart |
| cert-manager CRD at wrong version after bump | MEDIUM | `kubectl delete crd --selector=app.kubernetes.io/name=cert-manager`; re-apply with `--server-side`; re-create any ClusterIssuers |
| Envoy GW HTTPRoutes broken after GW API CRD mismatch | MEDIUM | Apply correct GW API CRD version; `kubectl rollout restart deployment/envoy-gateway -n envoy-gateway-system` |
| MacOS binary ships without ad-hoc signature | LOW | Patch-release with signing; in interim, document `xattr -d com.apple.quarantine` workaround |
| DEBT-04 race fix causes doctor serialization regression | LOW | Revert mutex to test-scope-only fix; run `go test -race` to confirm |
| SYNC-02 default image points at incomplete Docker Hub push | MEDIUM | Revert `image.go` to v1.35.x; re-run plan 51-04 probe once Hub confirms complete manifest |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Etcd regen while running (quorum loss) | Phase 52 | `etcdctl endpoint health --cluster` returns all members healthy after regen + restart |
| Partial SAN coverage (split-brain) | Phase 52 | All CP nodes use certs from the same regen pass; `docker logs` shows no TLS peer errors |
| CA key inaccessible without starting container | Phase 52 | `docker cp` test with stopped container in plan pre-flight |
| kubeadm certs subcommand unavailable | Phase 52 | Implement cert regen without kubeadm; version probe as guard |
| Bumping all addons at once | Phase 53 | One plan per addon; plan naming is `53-01` through `53-07` |
| cert-manager annotation limit | Phase 53, plan 53-03 | Integration test asserts `--server-side` flag; CRD version verified post-install |
| cert-manager webhook race | Phase 53, plan 53-03 | `kubectl rollout status cert-manager-webhook` wait in action; ClusterIssuer deferred |
| Envoy GW CRD version mismatch | Phase 53, plan 53-02 | HTTPRoute CRD version assertion post-install |
| MetalLB legacy CRD format | Phase 53, plan 53-04 | IPAddressPool CRD present; ConfigMap config absent |
| Headlamp token flow change | Phase 53, plan 53-05 | Non-empty token printed; `kubectl auth can-i` validates token |
| Metrics Server flag deprecation | Phase 53, plan 53-06 | HPA `TARGETS` populated after cluster creation |
| local-path busybox air-gap mismatch | Phase 53, plan 53-07 | offlinereadiness.go matches manifest helperImage version |
| SYNC-02 incomplete Hub push | Phase 53 (SYNC-02) | Two-step probe: existence + manifest digest |
| SYNC-02 stale version strings | Phase 53 (SYNC-02) | grep-based test: image.go version == offlinereadiness.go version |
| Symbol strip invalidates signature | Phase 54 | `codesign -vvv` on both arch artifacts passes in CI |
| GoReleaser fan-out misses one arch | Phase 54 | CI verifies both `darwin/amd64` and `darwin/arm64` artifacts separately |
| Ad-hoc signing ≠ Gatekeeper bypass | Phase 54 | Release note explicitly states limitations; install guide has `xattr` one-liner |
| cgo transitive deps on Windows | Phase 55 | `CGO_ENABLED=0 GOOS=windows go build ./...` passes before CI YAML is written |
| Path separator failures on Windows | Phase 55 | `GOOS=windows go vet ./...` passes; grep audit for `/`-concatenation |
| Mutex held during check execution | Phase 56 | `go test -race ./pkg/internal/doctor/... -count=100` zero races; `kinder doctor` timing unchanged |
| LB node in skew calculation | Phase 57 | Test case with CP+LB cluster asserts LB node absent from skew output |
| etcdctl JSON shape variance | Phase 57 | Test fixtures for both 3.4.x and 3.5.x JSON shapes |
| UAT drift / stale binary | Phase 58 | `./bin/kinder version` matches v2.4 build hash before every UAT run |

---

## Sources

- v2.3 Milestone Audit: `.planning/milestones/v2.3-MILESTONE-AUDIT.md` (deferred items, tech debt log)
- v2.3 Requirements: `.planning/milestones/v2.3-REQUIREMENTS.md` (v2.4 carry-forwards, DEBT-01..04)
- Codebase Architecture: `.planning/codebase/ARCHITECTURE.md` (wave-based addon execution, doctor check registry, provider pattern)
- Codebase Concerns: `.planning/codebase/CONCERNS.md` (DEBT-04 race, provider duplication, fragile areas)
- etcd peer TLS: etcd docs on peer TLS (https://etcd.io/docs/v3.5/op-guide/security/) — confirmed peer certs are NOT live-reloaded; restart required
- cert-manager annotation limit: cert-manager upstream issue (annotation limit >256KB, `--server-side` required) — locked decision from v2.3
- Envoy Gateway compatibility matrix: https://gateway.envoyproxy.io/releases/ — GW API CRD version locked per GW release
- GoReleaser signing docs: https://goreleaser.com/customization/sign/ — `artifacts: binary` vs `artifacts: archive` distinction
- macOS ad-hoc signing: Apple developer docs — ad-hoc signing does not satisfy Gatekeeper; quarantine xattr persists

---
*Pitfalls research for: kinder v2.4 Hardening (brownfield, maintenance milestone)*
*Researched: 2026-05-09*
