# Stack Research — kinder v2.4 Hardening

**Domain:** Brownfield maintenance milestone (Go CLI tool, Kubernetes addon manager)
**Researched:** 2026-05-09
**Confidence:** HIGH (all addon versions fetched from upstream GitHub releases; GoReleaser docs fetched from goreleaser.com; kubeadm docs fetched from kubernetes.io)

---

## 1. Addon Version Audit

All versions verified against upstream GitHub releases pages as of 2026-05-09.

### cert-manager

| Line | Current pin | Target | Release date | Manifest size |
|------|-------------|--------|--------------|---------------|
| v1.16.x | v1.16.3 | **v1.16.5** | 2025-04-24 | 986 KB |
| Latest stable | — | **v1.20.2** | 2025-04-11 | 988 KB |

**Support status:** v1.16 reached EOL 2025-06-10 per cert-manager.io/docs/releases. v1.20 is the current stable. v1.19 is the prior supported release.

**Recommendation: Bump to v1.16.5 now; target v1.20.2 if schedule permits.**

v1.16.5 is a pure CVE/security-dependency patch (Go 1.23.8, golang-jwt, golang.org/x/net). No API changes. Safe drop-in for kinder.

v1.20.2 is a major version jump from v1.16. Breaking changes accumulated across v1.17–v1.20 are minor for kinder's usage:
- v1.17: RSA 3072/4096-bit keys switch to SHA-384/SHA-512 (kinder uses self-signed ClusterIssuer; irrelevant). Log message format changed (kinder does not parse cert-manager logs). `ValidateCAA` feature gate deprecated.
- v1.18–v1.19: No breaking changes affecting kinder's embedded ClusterIssuer YAML.
- v1.20: Default container UID/GID changed from 1000/0 to 65532/65532. `DefaultPrivateKeyRotationPolicyAlways` promoted to GA (cannot be disabled). API defaults revert for issuer reference kind/group (fixes a bug, no kinder impact).

**Server-side apply required:** YES. Both v1.16.5 (986 KB) and v1.20.2 (988 KB) exceed the 256 KB client-side annotation limit. Kinder's existing `--server-side` flag is correct and must be preserved for all cert-manager versions.

**kinder landing zone:** `pkg/cluster/internal/create/actions/installcertmanager/certmanager.go` — update `Images` slice (3 images: cainjector, controller, webhook) and the manifest URL constant.

**Manifest URL pattern:** `https://github.com/cert-manager/cert-manager/releases/download/v{VERSION}/cert-manager.yaml`

Sources: [cert-manager releases](https://github.com/cert-manager/cert-manager/releases), [cert-manager supported releases](https://cert-manager.io/docs/releases/), [cert-manager kubectl install](https://cert-manager.io/docs/installation/kubectl/), [v1.16→v1.17 upgrade notes](https://cert-manager.io/docs/installation/upgrading/upgrading-1.16-1.17/)

---

### Envoy Gateway

| Channel | Current pin | Target | Release date | K8s compat |
|---------|-------------|--------|--------------|------------|
| Latest stable | v1.3.1 | **v1.7.2** | 2026-04-17 | v1.32–v1.35 |
| LTS-prior | — | **v1.6.7** | 2026-04-27 | v1.30–v1.33 |

**Note on v1.3.1:** This is now two major versions behind. v1.4 EOL'd 2025-11-13. v1.5 EOL'd 2026-02-13. v1.6 EOLs 2026-05-13 (one week from now). v1.7 is the current maintained line.

**Recommendation: Bump to v1.7.2.** v1.6.7 is imminently EOL.

**Breaking changes in v1.7 that affect kinder:**

The install process now explicitly recommends `kubectl apply --server-side -f https://github.com/envoyproxy/gateway/releases/download/v1.7.2/install.yaml` — which matches kinder's existing pattern. No change needed there.

There is one operational caveat: v1.7 upgrading from v1.6 requires CRDs to be applied before the gateway controller. For kinder's fresh-install flow (not upgrade), this is irrelevant since addon apply is always from scratch on a new cluster.

v1.7 behavioral changes in the runtime (OAuth2 metrics prefix, HTTP filter ordering, stats_tags defaults) do not affect kinder's `GatewayClass` + `HTTPRoute` YAML stanzas — those are user-space resources, not the gateway controller config.

**K8s 1.32 minimum:** v1.7 requires Kubernetes ≥ 1.32. kinder's default image target is K8s 1.35.x (and 1.36.x once kind v0.32.0 ships). COMPATIBLE.

**v1.3.1 had a specific job name comment** in `envoygw.go` line 84: `// Job name "eg-gateway-helm-certgen" verified for v1.3.1; re-check on upgrade`. The `eg-gateway-helm-certgen` job name should be re-verified against v1.7.2's install.yaml before the bump.

**kinder landing zone:** `pkg/cluster/internal/create/actions/installenvoygw/envoygw.go` — update `Images` slice (1 image: `envoyproxy/gateway:v1.7.2`) and manifest URL.

**Manifest URL pattern:** `https://github.com/envoyproxy/gateway/releases/download/v{VERSION}/install.yaml`

Sources: [Envoy Gateway releases](https://github.com/envoyproxy/gateway/releases), [Envoy Gateway compatibility matrix](https://gateway.envoyproxy.io/news/releases/matrix/), [Envoy Gateway v1.7 announcement](https://gateway.envoyproxy.io/news/releases/v1.7/), [install-yaml docs](https://gateway.envoyproxy.io/docs/install/install-yaml/)

---

### Headlamp

| Current pin | Target | Release date |
|-------------|--------|--------------|
| v0.40.1 | **v0.42.0** | 2026-05-07 |

v0.41.0 released 2026-03-23. v0.42.0 released 2026-05-07 (2 days before this research).

v0.42.0 includes: deep-link pod terminals/logs, label-based resource search, form-based resource creation, log severity filtering, node resource allocation metrics, GRPCRoute details. Bug fixes: WebSocket deadlock, cache desync, React hooks violations, backend panics in port-forwarding and Helm handling.

**Breaking changes affecting kinder:** None. The Helm chart gains additive fields (`service.extraServicePorts`) but kinder does not use the Helm chart — it applies the raw deployment manifest. The image reference is unchanged: `ghcr.io/headlamp-k8s/headlamp:v0.42.0`.

**kinder landing zone:** `pkg/cluster/internal/create/actions/installdashboard/dashboard.go` — update `Images` slice (1 image).

Sources: [Headlamp releases](https://github.com/headlamp-k8s/headlamp/releases), [v0.42.0 release notes](https://github.com/headlamp-k8s/headlamp/releases/tag/v0.42.0)

---

### MetalLB

| Current pin | Target | Release date | Notes |
|-------------|--------|--------------|-------|
| v0.15.3 | **v0.15.3** (HOLD) | 2024-12-04 | Latest stable; no v0.16 exists |

**v0.15.3 is the latest MetalLB release.** The v0.15.x line has no newer patch as of 2026-05-09. There is no v0.16.x or v0.17.x. 67 commits exist on `main` since v0.15.3 but none have been tagged as a release.

**Recommendation: Hold at v0.15.3. No action required.**

Sources: [MetalLB releases](https://github.com/metallb/metallb/releases), [MetalLB release notes](https://metallb.universe.tf/release-notes/)

---

### Metrics Server

| Current pin | Target | Release date |
|-------------|--------|--------------|
| v0.8.1 | **v0.8.1** (HOLD) | 2026-01-29 |

v0.8.1 is the latest stable release. No v0.9.x exists. The release bumped Go to 1.24.12 and Kubernetes dependencies to v0.33.7.

**Recommendation: Hold at v0.8.1. No action required.**

Sources: [metrics-server releases](https://github.com/kubernetes-sigs/metrics-server/releases)

---

### local-path-provisioner

| Current pin | Target | Release date | CVE fix |
|-------------|--------|--------------|---------|
| v0.0.35 | **v0.0.36** | 2025-05-08 | GHSA-7fxv-8wr2-mfc4 (HelperPod Template Injection) |

v0.0.36 fixes a HIGH severity vulnerability: HelperPod Template Injection (GHSA-7fxv-8wr2-mfc4). The fix adds HelperPod template validation rejecting privileged containers, hostPath volumes, and dangerous pod security settings. This is distinct from CVE-2025-62878 which was fixed in v0.0.34.

**Recommendation: Bump to v0.0.36 immediately.** Security fix.

**Breaking changes affecting kinder:** None. The StorageClass configuration (named `local-path`) is unchanged. The busybox:1.37.0 pin for air-gap PVC operations is unaffected — v0.0.36 does not change the helper image default.

**kinder landing zone:** `pkg/cluster/internal/create/actions/installlocalpath/localpathprovisioner.go` — update `Images` slice (1 image: `docker.io/rancher/local-path-provisioner:v0.0.36`). Also update the CVE threshold in `pkg/internal/doctor/localpath.go` — the existing check warns on versions strictly less than v0.0.34; consider updating the threshold comment to reference GHSA-7fxv-8wr2-mfc4 fix at v0.0.36.

Sources: [local-path-provisioner releases](https://github.com/rancher/local-path-provisioner/releases), [v0.0.36 release notes](https://github.com/rancher/local-path-provisioner/releases/tag/v0.0.36)

---

### Local Registry (registry:2)

| Current pin | Status | Notes |
|-------------|--------|-------|
| `registry:2` (floating tag) | **No change recommended** | registry:2 = 2.8.3; registry:3 is out of scope |

Docker Hub shows `registry:2` resolves to `2.8.3` (pushed ~1 year ago). The `registry:3` tag is at `3.1.1` (pushed 5 days ago) but is explicitly OUT OF SCOPE per PROJECT.md: "registry:3 deprecated storage drivers; kind ecosystem on v2".

The floating `registry:2` tag is acceptable for the local development use case. Pin to `registry:2.8.3` only if air-gap reproducibility is a concern (snapshot metadata already records the image digest, so this is low priority).

**Recommendation: Hold at `registry:2` floating tag. No action required.**

Sources: [Docker Hub registry tags](https://hub.docker.com/_/registry/tags)

---

## 2. macOS Ad-Hoc Signing in GoReleaser

### What ad-hoc signing fixes

Ad-hoc signing (`codesign --sign -`) resolves the `AMFI: has no CMS blob?` kernel error that kills Go binaries on Apple Silicon on first run. This is a macOS security requirement: all executables must be code-signed, including with an ad-hoc (no-certificate) signature. Standard `go build` on a developer's local machine automatically ad-hoc-signs the output, but cross-compiled binaries built on Linux CI do not carry any signature, causing macOS to kill them via AMFI on the first dyld load.

**What it does NOT fix:** Gatekeeper quarantine (`spctl`). Ad-hoc signatures are accepted by the macOS kernel for execution but are NOT validated by Gatekeeper/notarization policy. Users who download the binary via curl/browser will still need to run `xattr -d com.apple.quarantine ./kinder` or right-click → Open the first time. This is the expected behavior for open-source CLI tools distributed without an Apple Developer certificate. It is not a regression — it is unchanged from the current situation, and is explicitly the correct scope for this milestone (notarization is OUT OF SCOPE).

### GoReleaser hook approach

The GoReleaser builds documentation (fetched 2026-05-09) shows that build hooks support `cmd`, `dir`, `output`, and `env` fields but do NOT document a native `if`/condition field for platform filtering. However, the hook `cmd` field supports full Go template syntax, including `.Target` (e.g., `darwin_amd64`) and `.Os` template variables.

The correct approach is to use a wrapper shell script that no-ops on non-darwin targets, or to split the build into separate darwin and non-darwin build entries in `.goreleaser.yaml`. The split-build approach is cleaner and more maintainable:

```yaml
builds:
  - id: kinder-unix
    main: .
    binary: kinder
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -buildid=
      - -w
      - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .FullCommit }}
    mod_timestamp: "{{ .CommitTimestamp }}"

  - id: kinder-darwin
    main: .
    binary: kinder
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -buildid=
      - -w
      - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .FullCommit }}
    mod_timestamp: "{{ .CommitTimestamp }}"
    hooks:
      post:
        - cmd: codesign --force --sign - "{{ .Path }}"
          output: true

  - id: kinder-windows
    main: .
    binary: kinder
    env:
      - CGO_ENABLED=0
    goos:
      - windows
    goarch:
      - amd64
    flags:
      - -trimpath
    ldflags:
      - -buildid=
      - -w
      - -X=sigs.k8s.io/kind/pkg/internal/kindversion.gitCommit={{ .FullCommit }}
    mod_timestamp: "{{ .CommitTimestamp }}"
```

The `codesign` binary must be present on the CI runner. The release workflow currently uses `runs-on: ubuntu-latest`, which does NOT have `codesign`. **Three options:**

**Option A (Recommended): macOS runner for release.** Change `release.yml` to `runs-on: macos-latest`. GoReleaser can still cross-compile for linux/windows from macOS. `codesign` is available natively. Cost: macOS runners are ~10x more expensive per minute than Linux on GitHub Actions (10 min * $0.08/min = $0.80/release).

**Option B:** Use `goreleaser/goreleaser-action` with a macOS runner only for the darwin builds, keeping linux/windows on ubuntu. Requires splitting the release job into platform-specific jobs. More complex but cost-efficient.

**Option C:** Accept the limitation and document that macOS users need to run `codesign --sign - ./kinder` after download. Add to the release footer in `.goreleaser.yaml`. Zero CI cost, matches what Homebrew does automatically (Homebrew re-signs on install).

**GoReleaser version compatibility:** The current `.goreleaser.yaml` uses `version: 2` and `goreleaser-action@v7` with `version: "~> v2"`. The GoReleaser docs show v2.15 is current. The split-build approach works with GoReleaser v2.x — no version bump required.

### Info.plist — NOT required for ad-hoc signing

Embedding an `Info.plist` via linker flags (`-extldflags "-sectcreate __TEXT __info_plist ..."`) is required only for macOS applications distributed as `.app` bundles. For a CLI binary, `codesign --sign -` without an Info.plist is valid and sufficient. The `codesign` tool will auto-generate a derived identifier from the binary path when no explicit `-i` identifier is provided.

Sources: [golang/go#42684](https://github.com/golang/go/issues/42684), [GoReleaser build hooks](https://goreleaser.com/customization/builds/hooks/), [macOS codesign ad-hoc](https://stories.miln.eu/graham/2024-06-25-ad-hoc-code-signing-a-mac-app/)

---

## 3. Windows CI Build Step

### Decision: ubuntu-latest with cross-compile (CGO_ENABLED=0)

kinder uses `CGO_ENABLED=0` throughout (confirmed in `.goreleaser.yaml` and Go build flags). Cross-compiling for Windows from `ubuntu-latest` is 100% equivalent to a native `windows-latest` build when CGO is disabled. There are no CGO dependencies in kinder (no C bindings, no SQLite, no OS-specific C libraries).

`windows-latest` runners cost ~2x more per minute than `ubuntu-latest` on GitHub Actions and take longer to spin up. For a pure compile-check job, `ubuntu-latest` is the right choice.

**Exact GHA step to add to the existing PR CI workflow:**

Add a new job (or step) to the existing `docker.yaml` / `podman.yml` / `nerdctl.yaml` workflows, or create a dedicated `build.yaml` that runs on all PRs:

```yaml
  build-windows:
    name: Build (Windows cross-compile)
    runs-on: ubuntu-24.04
    steps:
      - name: Check out code
        uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2

      - uses: ./.github/actions/setup-env

      - name: Cross-compile for Windows
        env:
          GOOS: windows
          GOARCH: amd64
          CGO_ENABLED: "0"
        run: go build ./...
```

This matches the pattern already established in `setup-env/action.yaml` (reads `.go-version`, uses `actions/setup-go`).

### Known Windows compile caveats for kinder

1. **File path separators:** kinder uses `filepath.Join` throughout (confirmed from kind upstream patterns). No hardcoded `/` path separators in business logic. Low risk.

2. **`syscall` and platform-specific build tags:** kinder has build-tagged files (`disk_unix.go`, `kernel_linux.go`). These correctly use `//go:build` constraints and will not compile on Windows — which is the correct behavior. `go build ./...` on Windows will skip Linux/Unix-only files and compile only the platform-neutral subset. The compile check validates that the Windows-compiled subset has no syntax errors or missing imports.

3. **`fsnotify` v1.10.1 (added in v2.3):** fsnotify supports Windows natively. No issue.

4. **Doctor checks with OS-specific stubs:** The `Platforms()` method on checks gates runtime behavior, not compile-time. All check structs compile on Windows; only `Run()` behavior is platform-filtered. No issue.

5. **`exec.Command` and socket paths:** Some doctor checks reference `/var/run/docker.sock` as a string literal (used in error messages, not as a dial address on Windows). These are in non-Windows-specific files. As long as the string is not fed to `net.Dial` directly in Windows code paths, this is fine. Review `dockersocket.go` if it lacks a `//go:build !windows` constraint.

Sources: [Go cross-compilation wiki](https://go.dev/wiki/WindowsCrossCompiling), [fsnotify cross-platform support](https://github.com/fsnotify/fsnotify)

---

## 4. Etcd Peer-TLS Regeneration for HA Pause/Resume

### The problem

When `kinder pause` stops HA control-plane containers and `kinder resume` restarts them, Docker may reassign container IP addresses from its IPAM pool. The etcd peer certificates contain IP SANs for the original container IPs. If any control-plane node gets a different IP, etcd peer connections are rejected with TLS verification failures (peer IP not in SAN list), breaking HA quorum permanently until certs are regenerated.

### Approach A: kubeadm cert regeneration (RECOMMENDED)

`kubeadm certs renew etcd-peer` is the official subcommand for regenerating etcd peer certificates inside a running node. It is available inside `kindest/node` images (which include `kubeadm`).

**Exact command sequence (execute inside each control-plane container via `docker exec`):**

```bash
# 1. Regenerate etcd-peer cert with current node IP in SAN
docker exec <cp-container> kubeadm certs renew etcd-peer

# 2. Restart etcd to pick up new cert
docker exec <cp-container> crictl stop $(crictl ps -q --name=etcd)
# etcd is a static Pod — kubelet will restart it automatically after crictl stop
```

**Sequencing:** Must run on ALL control-plane nodes before resuming the etcd cluster. Run serially to avoid quorum loss (stop one etcd, regen cert, wait for it to rejoin, proceed to next).

**kubeadm availability in kindest/node:** kubeadm is always present in `kindest/node` images — it is used during `kinder create cluster` to bootstrap the cluster. HIGH confidence this subcommand is available.

**kubeadm certs renew etcd-peer options:**
- `--cert-dir string` — default `/etc/kubernetes/pki`
- `--config string` — kubeadm config file
- Returns error if the CA key is not present (it IS present in kind nodes since the cluster was bootstrapped locally)

### Approach B: Docker static IP assignment (ALTERNATIVE)

Docker user-defined networks support static IP assignment via `docker network connect --ip <addr> <container>`. When a container is stopped and restarted, Docker reapplies the assigned IP if the `--ip` was specified at `docker run` time or via `docker network connect`. However, this only works if the IP was reserved via `--ip-range` exclusion when the network was created — otherwise, IPAM may have handed the IP to another container during the pause window.

**Feasibility for kinder:** Kind creates its cluster network (`kind`) using a standard bridge with IPAM. The IP assignments are not guaranteed-static by default. Retrofitting IP pinning would require:
1. Capturing each node's current IP at `kinder pause` time.
2. Calling `docker network disconnect / connect --ip <original-ip>` at `kinder resume` time before starting the container.
3. Verifying the IP is still available in the pool.

This is more fragile than cert regeneration and is not the recommended path.

### Recommended implementation in kinder

The `resume` action in kinder should:
1. Before restarting etcd containers: call `docker exec <node> kubeadm certs renew etcd-peer` on each control-plane node.
2. After restart: wait for `etcdctl endpoint health` (already done by `cluster-resume-readiness` doctor check) before returning success.

The `cluster-resume-readiness` check already uses `crictl exec <etcd-id> etcdctl endpoint health --cluster` — the kubeadm cert regen fits the same execution model.

Sources: [kubeadm certs renew docs](https://kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm-certs/), [kubeadm certificate management](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/), [docker network connect](https://docs.docker.com/reference/cli/docker/network/connect/)

---

## 5. DEBT-04 Race Fix — allChecks Global Under t.Parallel()

### The race

`pkg/internal/doctor/check_test.go` has multiple tests that:
1. Call `t.Parallel()`.
2. Save and restore the `allChecks` package-level `var allChecks = []Check{...}` via `original := allChecks; defer func() { allChecks = original }()`.
3. Mutate `allChecks = []Check{...}` to inject mock checks.

When two such tests run concurrently (which `t.Parallel()` allows), both are reading and writing `allChecks` without synchronization. Go's race detector (`-race`) flags this as a data race.

### The fix pattern

Two options:

**Option A (Recommended): Eliminate shared global mutation — inject checks via parameter.**

Refactor `RunAllChecks()` and `AllChecks()` to accept an optional override slice, eliminating the need for tests to mutate the global:

```go
// check.go — production code unchanged
func RunAllChecks() []Result {
    return runChecks(allChecks)
}

func runChecks(checks []Check) []Result {
    // existing loop logic
}

// check_test.go — tests call the internal function directly
func TestRunAllChecks_PlatformSkip(t *testing.T) {
    t.Parallel()
    checks := []Check{&mockCheck{...}}
    results := runChecks(checks) // no global mutation
    // assertions...
}
```

This is the idiomatic Go pattern: exported functions use globals, internal functions are parameterized and testable. No mutex needed. Zero shared state between parallel tests.

**Option B: sync.Mutex on allChecks access.**

Protect all reads and writes with a package-level mutex. More mechanical but adds locking overhead and is harder to reason about:

```go
var (
    allChecksMu sync.Mutex
    allChecks   = []Check{...}
)

func setAllChecks(checks []Check) { // test helper
    allChecksMu.Lock()
    allChecks = checks
    allChecksMu.Unlock()
}
```

This is valid but is the second-choice pattern for this case. The race is in test code only, so eliminating the shared state (Option A) is cleaner.

**Existing precedent in kinder:** `sync.OnceValues` is used in `action.go` for the `nodesOnce` cache — single-initialization pattern. The ethos is "eliminate shared state via construction, not via locking." Option A matches this ethos.

**Landing zone:** `pkg/internal/doctor/check.go` (add unexported `runChecks` function), `pkg/internal/doctor/check_test.go` (replace global mutation with `runChecks` calls), and `pkg/internal/doctor/socket_test.go` (same pattern if it has analogous races).

Sources: [Go race detector](https://go.dev/doc/articles/race_detector), [t.Parallel patterns](https://brandur.org/t-parallel)

---

## 6. What NOT to Add

| Technology / Pattern | Why Not | Correct Alternative |
|---------------------|---------|---------------------|
| Apple Developer certificate notarization | Requires paid Apple Developer Program membership ($99/yr); notarization requires internet access at signing time; out of scope per PROJECT.md | Ad-hoc signing only (`codesign --sign -`) |
| `goreleaser/goreleaser-pro` notarization features | Pro version; not available to open source; requires Apple cert | Use build hooks on macOS runner with `codesign --sign -` |
| Quill (anchore/quill) cross-platform signing | Requires Apple Developer cert for real signing; ad-hoc via quill is still just `codesign -s -` with extra indirection | Direct `codesign --sign -` in build hook is simpler |
| `windows-latest` runner for compile check | 2x cost, longer startup; pure CGO_ENABLED=0 build is identical on linux | `ubuntu-latest` with `GOOS=windows go build ./...` |
| `goreleaser-cross` Docker image | Needed only for CGO cross-compilation; kinder has CGO_ENABLED=0 everywhere | Standard GoReleaser with GOOS env var |
| Helm for addon management | Explicitly out of scope; adds Helm as a runtime dependency; violates go:embed static manifest approach | Continue with runtime `kubectl apply` |
| DAG-based addon orchestration | 7 addons with shallow deps; adds 200+ LOC for zero benefit | Existing wave-based parallel execution |
| registry:3 | v3 deprecated storage drivers; kind ecosystem on v2 | Keep `registry:2` |
| ACME issuers for cert-manager | Requires internet; breaks offline/air-gap clusters | Self-signed ClusterIssuer (current approach) |
| sync.RWMutex for allChecks race | Over-engineered for test-only shared state | Eliminate shared state via `runChecks(checks []Check)` parameter injection |

---

## Version Compatibility Summary

| Component | From | To | Action | Breaking? |
|-----------|------|----|--------|-----------|
| cert-manager | v1.16.3 | v1.16.5 | Bump | No |
| cert-manager | v1.16.3 | v1.20.2 | Optional bump | Minor (UID change, log format) |
| Envoy Gateway | v1.3.1 | v1.7.2 | Bump (v1.3 2+ versions EOL) | Verify job name in install.yaml |
| Headlamp | v0.40.1 | v0.42.0 | Bump | No |
| MetalLB | v0.15.3 | v0.15.3 | Hold (latest) | N/A |
| Metrics Server | v0.8.1 | v0.8.1 | Hold (latest) | N/A |
| local-path-provisioner | v0.0.35 | v0.0.36 | Bump (security fix) | No |
| registry:2 | floating | floating | Hold | N/A |
| GoReleaser | v2.x | v2.x (≤v2.15) | No change needed | N/A |

---

## Sources

- [cert-manager GitHub releases](https://github.com/cert-manager/cert-manager/releases) — v1.16.5 date (2025-04-24), v1.20.2 date (2025-04-11), EOL status. HIGH confidence.
- [cert-manager supported releases page](https://cert-manager.io/docs/releases/) — v1.16 EOL 2025-06-10, v1.20 current stable. HIGH confidence.
- [cert-manager kubectl install docs](https://cert-manager.io/docs/installation/kubectl/) — server-side apply not required by docs but manifest is 986 KB (v1.16.5) / 988 KB (v1.20.2) — both exceed 256 KB annotation limit. Verified by downloading manifests. HIGH confidence.
- [Envoy Gateway releases](https://github.com/envoyproxy/gateway/releases) — v1.7.2 date (2026-04-17), v1.6.7 date (2026-04-27). HIGH confidence.
- [Envoy Gateway compatibility matrix](https://gateway.envoyproxy.io/news/releases/matrix/) — v1.7 supports K8s 1.32–1.35. HIGH confidence.
- [Envoy Gateway v1.7 announcement](https://gateway.envoyproxy.io/news/releases/v1.7/) — 7 breaking changes listed; none affect kinder's GatewayClass/HTTPRoute YAML. HIGH confidence.
- [Headlamp releases](https://github.com/headlamp-k8s/headlamp/releases) — v0.42.0 date (2026-05-07). HIGH confidence.
- [MetalLB release notes](https://metallb.universe.tf/release-notes/) — v0.15.3 is latest; no v0.16. HIGH confidence.
- [metrics-server releases](https://github.com/kubernetes-sigs/metrics-server/releases) — v0.8.1 latest (2026-01-29). HIGH confidence.
- [local-path-provisioner releases](https://github.com/rancher/local-path-provisioner/releases) — v0.0.36 (2025-05-08) security fix GHSA-7fxv-8wr2-mfc4. HIGH confidence.
- [Docker Hub registry tags](https://hub.docker.com/_/registry/tags) — registry:2 = 2.8.3. HIGH confidence.
- [GoReleaser build hooks docs](https://goreleaser.com/customization/builds/hooks/) — hook schema (cmd, dir, output, env; no native `if` field; template vars: .Name, .Ext, .Path, .Target). HIGH confidence.
- [golang/go#42684](https://github.com/golang/go/issues/42684) — `codesign -s -` fixes arm64 macOS AMFI kill. HIGH confidence.
- [kubeadm certs renew docs](https://kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm-certs/) — `kubeadm certs renew etcd-peer` subcommand confirmed. HIGH confidence.
- [docker network connect docs](https://docs.docker.com/reference/cli/docker/network/connect/) — `--ip` persists across stop/start. MEDIUM confidence (IP pinning feasibility for kinder's existing network setup requires testing).
- [Go race detector](https://go.dev/doc/articles/race_detector) — t.Parallel + global mutation pattern. HIGH confidence.

---
*Stack research for: kinder v2.4 Hardening (brownfield maintenance)*
*Researched: 2026-05-09*
