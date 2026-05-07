---
phase: 51-upstream-sync-k8s-1-36
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - pkg/cluster/internal/loadbalancer/const.go
  - pkg/cluster/internal/loadbalancer/config.go
  - pkg/cluster/internal/loadbalancer/config_test.go
  - pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go
  - pkg/cluster/internal/providers/docker/create.go
  - pkg/cluster/internal/providers/docker/create_test.go
  - pkg/cluster/internal/providers/podman/provision.go
  - pkg/cluster/internal/providers/podman/provision_test.go
  - pkg/cluster/internal/providers/nerdctl/create.go
  - pkg/cluster/internal/providers/nerdctl/create_test.go
  - pkg/internal/doctor/offlinereadiness.go
  - pkg/internal/doctor/offlinereadiness_test.go
autonomous: true

must_haves:
  truths:
    - "HA clusters created with kinder use Envoy as the load-balancer container instead of HAProxy"
    - "kindest/haproxy is no longer pulled or referenced in production code"
    - "loadbalancer reload uses atomic file swap (mv), not kill -s HUP 1"
    - "All three providers (docker/podman/nerdctl) append GenerateBootstrapCommand after the LB image in their docker run args"
  artifacts:
    - path: "pkg/cluster/internal/loadbalancer/const.go"
      provides: "Envoy image constant + four ProxyConfigPath* constants"
      contains: "envoyproxy/envoy"
    - path: "pkg/cluster/internal/loadbalancer/config.go"
      provides: "DynamicFilesystemConfigTemplate + ProxyLDSConfigTemplate + ProxyCDSConfigTemplate + Config(data, template) + GenerateBootstrapCommand"
      exports: ["Config", "GenerateBootstrapCommand", "DynamicFilesystemConfigTemplate", "ProxyLDSConfigTemplate", "ProxyCDSConfigTemplate"]
    - path: "pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go"
      provides: "atomic-swap LDS+CDS write logic"
      contains: "ProxyConfigPathLDS"
    - path: "pkg/internal/doctor/offlinereadiness.go"
      provides: "Envoy image listed for offline readiness"
      contains: "envoyproxy/envoy"
  key_links:
    - from: "pkg/cluster/internal/providers/docker/create.go"
      to: "pkg/cluster/internal/loadbalancer.GenerateBootstrapCommand"
      via: "args = append(args, loadbalancer.Image); args = append(args, loadbalancer.GenerateBootstrapCommand(cfg.Name, name)...)"
      pattern: "GenerateBootstrapCommand"
    - from: "pkg/cluster/internal/providers/podman/provision.go"
      to: "pkg/cluster/internal/loadbalancer.GenerateBootstrapCommand"
      via: "appended to runArgs after loadbalancer.Image"
      pattern: "GenerateBootstrapCommand"
    - from: "pkg/cluster/internal/providers/nerdctl/create.go"
      to: "pkg/cluster/internal/loadbalancer.GenerateBootstrapCommand"
      via: "appended to args after loadbalancer.Image"
      pattern: "GenerateBootstrapCommand"
    - from: "pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go"
      to: "pkg/cluster/internal/loadbalancer.{Config, ProxyLDSConfigTemplate, ProxyCDSConfigTemplate, ProxyConfigPathLDS, ProxyConfigPathCDS}"
      via: "two Config() calls, two WriteFile calls, sh -c \"chmod && mv && mv\""
      pattern: "ProxyConfigPathLDS"
---

<objective>
Adopt upstream kind PR #4127 (merged 2026-04-02) — replace the HAProxy load-balancer with Envoy across kinder. The migration touches the loadbalancer package internals (const + config + reload action), all three provider create/provision files (which need to append `GenerateBootstrapCommand` after the LB image in docker run args), and the doctor offline-readiness image list.

Purpose: Sync with upstream kind so kinder HA clusters benefit from Envoy's xDS reload semantics (atomic file swap instead of SIGHUP), drop the `kindest/haproxy` image dependency, and stay aligned with kind's LB roadmap. Delivers SYNC-01 and SC1.

Output: Envoy is the LB container for HA clusters; `kindest/haproxy` is not referenced in any production code path; provider docker-run args include the bootstrap bash command after the image; LB reload uses `mv` not SIGHUP; offlinereadiness check lists the Envoy image instead of HAProxy.
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

# Current LB code that this plan rewrites
@pkg/cluster/internal/loadbalancer/const.go
@pkg/cluster/internal/loadbalancer/config.go
@pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go

# Provider files (each appends GenerateBootstrapCommand)
@pkg/cluster/internal/providers/docker/create.go
@pkg/cluster/internal/providers/podman/provision.go
@pkg/cluster/internal/providers/nerdctl/create.go

# Doctor image list to update
@pkg/internal/doctor/offlinereadiness.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: TDD RED — failing tests for Envoy const + config rewrite</name>
  <files>
    pkg/cluster/internal/loadbalancer/config_test.go
    pkg/internal/doctor/offlinereadiness_test.go
  </files>
  <action>
Write failing tests in `pkg/cluster/internal/loadbalancer/config_test.go` (or extend if exists):

1. `TestImageConstantIsEnvoy` — asserts `loadbalancer.Image == "docker.io/envoyproxy/envoy:v1.36.2"` (exact string per RESEARCH §SYNC-01).
2. `TestProxyConfigPathConstants` — asserts the four new constants exist and equal:
   - `ProxyConfigPath == "/home/envoy/envoy.yaml"`
   - `ProxyConfigPathCDS == "/home/envoy/cds.yaml"`
   - `ProxyConfigPathLDS == "/home/envoy/lds.yaml"`
   - `ProxyConfigDir == "/home/envoy"`
3. `TestConfigLDSRendersControlPlanePort` — calls `Config(&ConfigData{ControlPlanePort: 6443, IPv6: false}, ProxyLDSConfigTemplate)`, asserts the rendered string contains `port_value: 6443` AND `address: "0.0.0.0"`.
4. `TestConfigLDSIPv6` — same but with `IPv6: true`, asserts the rendered string contains `address: "::"`.
5. `TestConfigCDSRendersBackendServers` — calls `Config(&ConfigData{BackendServers: map[string]string{"cp-1": "172.18.0.4:6443", "cp-2": "172.18.0.5:6443"}}, ProxyCDSConfigTemplate)`, asserts both `172.18.0.4` and `172.18.0.5` appear in output AND that `health_checks` block is present.
6. `TestGenerateBootstrapCommandShape` — calls `GenerateBootstrapCommand("my-cluster", "my-cluster-external-load-balancer")`, asserts: returned slice has length 3, [0]=="bash", [1]=="-c", [2] contains all four strings: `mkdir -p /home/envoy`, `/home/envoy/envoy.yaml`, `/home/envoy/cds.yaml`, `/home/envoy/lds.yaml`, AND contains `while true; do envoy -c`.

In `pkg/internal/doctor/offlinereadiness_test.go` (or extend):

7. `TestOfflineReadinessIncludesEnvoyImage` — locates the offlinereadiness check's image list and asserts it contains `"docker.io/envoyproxy/envoy:v1.36.2"` AND does NOT contain any string matching `kindest/haproxy`.

Run `go test ./pkg/cluster/internal/loadbalancer/... ./pkg/internal/doctor/... -run 'TestImageConstantIsEnvoy|TestProxyConfigPathConstants|TestConfigLDSRendersControlPlanePort|TestConfigLDSIPv6|TestConfigCDSRendersBackendServers|TestGenerateBootstrapCommandShape|TestOfflineReadinessIncludesEnvoyImage' -race`. All seven MUST fail (compile errors are acceptable for the new symbols).

Commit RED:
```
git add pkg/cluster/internal/loadbalancer/config_test.go pkg/internal/doctor/offlinereadiness_test.go
git commit -m "test(51-01): add failing tests for Envoy LB constants and bootstrap command"
```
  </action>
  <verify>
`go test ./pkg/cluster/internal/loadbalancer/... -run 'TestImageConstantIsEnvoy|TestProxyConfigPath|TestConfigLDS|TestConfigCDS|TestGenerateBootstrap' -race 2>&1 | grep -E '(FAIL|undefined|cannot use)'` — must show failures or compile errors. Same for offlinereadiness test.
  </verify>
  <done>RED commit landed; running the new tests against current code fails (or fails to compile) on every assertion.</done>
</task>

<task type="auto">
  <name>Task 2: TDD GREEN — port Envoy const + config from upstream kind</name>
  <files>
    pkg/cluster/internal/loadbalancer/const.go
    pkg/cluster/internal/loadbalancer/config.go
    pkg/internal/doctor/offlinereadiness.go
  </files>
  <action>
Replace the contents of `pkg/cluster/internal/loadbalancer/const.go` with the upstream-equivalent constants (RESEARCH §SYNC-01 / Code Examples):
```go
package loadbalancer

const Image = "docker.io/envoyproxy/envoy:v1.36.2"

const (
    ProxyConfigPathCDS = "/home/envoy/cds.yaml"
    ProxyConfigPathLDS = "/home/envoy/lds.yaml"
    ProxyConfigPath    = "/home/envoy/envoy.yaml"
    ProxyConfigDir     = "/home/envoy"
)
```
Delete the old `ConfigPath` constant. If anything else in the package referenced `ConfigPath` (other than `actions/loadbalancer/loadbalancer.go`, which Task 3 fixes), grep and fix in this task.

Rewrite `pkg/cluster/internal/loadbalancer/config.go` to mirror upstream kind's structure:
1. Keep `ConfigData` struct unchanged: `ControlPlanePort int`, `BackendServers map[string]string`, `IPv6 bool`.
2. Replace `DefaultConfigTemplate` with three new exported template constants:
   - `DynamicFilesystemConfigTemplate` (a `fmt.Sprintf` format string for the bootstrap YAML; do NOT use Go template — upstream uses Sprintf with %s placeholders for cluster name, container name, CDS path, LDS path).
   - `ProxyLDSConfigTemplate` (Go template — listener config; uses `{{ if .IPv6 }}"::"{{ else }}"0.0.0.0"{{ end }}` and `{{ .ControlPlanePort }}`). Use the exact YAML from RESEARCH §Code Examples.
   - `ProxyCDSConfigTemplate` (Go template — cluster config with health checks and `{{ range $server, $address := .BackendServers }}` block; uses `hostPort` FuncMap to split `host:port` strings). For full template body refer to upstream kind's `pkg/cluster/internal/loadbalancer/config.go` at the kubernetes-sigs/kind main branch (PR #4127). MUST include `dns_lookup_family: AUTO` (per RESEARCH pitfall 4 — IPv6 handling).
3. Change `Config` signature from `Config(data *ConfigData) (string, error)` to `Config(data *ConfigData, configTemplate string) (string, error)`. Add `hostPort` FuncMap (a one-liner that splits a `host:port` string into a map; used by CDS template). The function uses Go's `text/template` to render `configTemplate` against `data`.
4. Add `GenerateBootstrapCommand(clusterName, containerName string) []string` exactly per RESEARCH §Code Examples block — formats the dynamic config with Sprintf, returns `[]string{"bash", "-c", "mkdir -p ... && echo -en ... > ... && while true; do envoy -c %s && break; sleep 1; done"}`.

Update `pkg/internal/doctor/offlinereadiness.go` line 51 (RESEARCH §SYNC-01):
- OLD: `{"docker.io/kindest/haproxy:v20260131-7181c60a", "Load Balancer (HA)"}`
- NEW: `{"docker.io/envoyproxy/envoy:v1.36.2", "Load Balancer (HA)"}`

Before committing, run a sanity grep:
```bash
grep -rn "kindest/haproxy" pkg/ kinder-site/ 2>/dev/null | grep -v _test.go
```
The only acceptable output is matches inside `_test.go` (negative-assertion test fixtures) or comments. If production code outside the listed files matches, fix in this task.

Run `go test ./pkg/cluster/internal/loadbalancer/... ./pkg/internal/doctor/... -race`. All Task-1 RED tests must now pass. The package's pre-existing tests (if any) must still pass.

Commit GREEN:
```
git add pkg/cluster/internal/loadbalancer/const.go pkg/cluster/internal/loadbalancer/config.go pkg/internal/doctor/offlinereadiness.go
git commit -m "feat(51-01): port Envoy LB constants, templates, and bootstrap command from kind PR #4127"
```
  </action>
  <verify>
`go test ./pkg/cluster/internal/loadbalancer/... ./pkg/internal/doctor/... -race` exits 0. `grep -rn "kindest/haproxy" pkg/ | grep -v _test.go` returns no production-code matches.
  </verify>
  <done>Envoy image constant, four path constants, three templates, two-arg `Config`, and `GenerateBootstrapCommand` exist; offlinereadiness lists Envoy image. RED tests now GREEN. Pre-existing tests still pass.</done>
</task>

<task type="auto">
  <name>Task 3: TDD RED+GREEN — atomic-swap LB action + provider bootstrap wiring</name>
  <files>
    pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go
    pkg/cluster/internal/providers/docker/create.go
    pkg/cluster/internal/providers/docker/create_test.go
    pkg/cluster/internal/providers/podman/provision.go
    pkg/cluster/internal/providers/podman/provision_test.go
    pkg/cluster/internal/providers/nerdctl/create.go
    pkg/cluster/internal/providers/nerdctl/create_test.go
  </files>
  <action>
**RED phase** — for each provider with an existing test file, add a failing test asserting that the LB run-args slice contains `bash`, `-c`, AND a string starting with `mkdir -p /home/envoy` somewhere AFTER the LB image position. Specifically:

- `docker/create_test.go`: extend or add `TestRunArgsForLoadBalancerAppendsBootstrap` — call the same internal helper that builds LB args (use the existing test pattern in this file; if the helper isn't directly testable, write a higher-level test that exercises `commonArgs` / `runArgsForNode` for a control-plane=loadbalancer scenario). Assert: index of `loadbalancer.Image` in returned args < index of `"bash"` < index of `"-c"`, and one of the args contains `mkdir -p /home/envoy`.
- `podman/provision_test.go`: same shape adapted to podman's helper signature.
- `nerdctl/create_test.go`: same shape adapted to nerdctl's helper signature.

If any provider lacks a unit-testable LB-args helper today, fall back to a single `TestGenerateBootstrapCommandIsAppendedAfterImage` style assertion that exercises whatever LB-args function the provider exposes. Run the new tests; they MUST fail.

Commit RED:
```
git add pkg/cluster/internal/providers/docker/create_test.go pkg/cluster/internal/providers/podman/provision_test.go pkg/cluster/internal/providers/nerdctl/create_test.go
git commit -m "test(51-01): add failing tests for provider LB bootstrap wiring"
```

**GREEN phase** —

1. **`pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go`** — rewrite the reload section per RESEARCH §SYNC-01 / Code Examples:
   - Generate two configs:
     ```go
     ldsConfig, err := loadbalancer.Config(data, loadbalancer.ProxyLDSConfigTemplate)
     if err != nil { return err }
     cdsConfig, err := loadbalancer.Config(data, loadbalancer.ProxyCDSConfigTemplate)
     if err != nil { return err }
     ```
   - Write each to a tmp file under `/home/envoy/` (e.g. `tmpLDS = ProxyConfigPathLDS + ".tmp"`, `tmpCDS = ProxyConfigPathCDS + ".tmp"`):
     ```go
     if err := nodeutils.WriteFile(loadBalancerNode, tmpLDS, ldsConfig); err != nil { return err }
     if err := nodeutils.WriteFile(loadBalancerNode, tmpCDS, cdsConfig); err != nil { return err }
     ```
   - Replace the `kill -s HUP 1` invocation with an atomic swap:
     ```go
     cmd := fmt.Sprintf("chmod 666 %s %s && mv %s %s && mv %s %s",
         tmpLDS, tmpCDS,
         tmpLDS, loadbalancer.ProxyConfigPathLDS,
         tmpCDS, loadbalancer.ProxyConfigPathCDS)
     if err := loadBalancerNode.Command("sh", "-c", cmd).Run(); err != nil {
         return errors.Wrap(err, "failed to reload Envoy load balancer config")
     }
     ```
   - Remove all references to `loadbalancer.ConfigPath` and `Config(data)` (single-arg) in this file.

2. **`pkg/cluster/internal/providers/docker/create.go`** — find the LB run-args function (RESEARCH says line 284 in upstream). The pattern is:
   ```go
   // OLD:
   return append(args, loadbalancer.Image), nil
   // NEW:
   args = append(args, loadbalancer.Image)
   args = append(args, loadbalancer.GenerateBootstrapCommand(cfg.Name, name)...)
   return args, nil
   ```
   `cfg.Name` is the cluster name; `name` is the container name (typically `<cluster>-external-load-balancer`). Use whatever local variables already hold those values in the existing function.

3. **`pkg/cluster/internal/providers/podman/provision.go`** — same pattern.

4. **`pkg/cluster/internal/providers/nerdctl/create.go`** — same pattern.

Run the full test sweep:
```bash
go test ./pkg/cluster/internal/loadbalancer/... \
  ./pkg/cluster/internal/create/actions/loadbalancer/... \
  ./pkg/cluster/internal/providers/... \
  ./pkg/internal/doctor/... \
  -race
```
All tests must pass. Then run `go build ./...` to confirm no callers were missed.

Commit GREEN:
```
git add pkg/cluster/internal/create/actions/loadbalancer/loadbalancer.go pkg/cluster/internal/providers/docker/create.go pkg/cluster/internal/providers/podman/provision.go pkg/cluster/internal/providers/nerdctl/create.go
git commit -m "feat(51-01): wire Envoy bootstrap into providers and atomic-swap LB reload"
```

**Final sanity check** (no commit, just verification):
```bash
grep -rn "kindest/haproxy" pkg/ kinder-site/ 2>/dev/null
grep -rn "kill.*HUP.*1" pkg/cluster/ 2>/dev/null
grep -rn "ConfigPath[^a-zA-Z]" pkg/cluster/internal/ 2>/dev/null | grep -v ProxyConfigPath
```
The first should match only `_test.go` (negative assertions) or comments; the second should not match the LB action; the third should not show the deleted `ConfigPath` constant being used.
  </action>
  <verify>
`go test ./pkg/cluster/... ./pkg/internal/... -race` exits 0. `go build ./...` exits 0. `grep -rn "kindest/haproxy" pkg/ | grep -v _test.go` returns no matches in production code. `grep -rn "kill.*HUP" pkg/cluster/internal/create/actions/loadbalancer/` returns no matches.
  </verify>
  <done>LB action writes LDS+CDS to tmp files and atomic-swaps via `mv`. All three providers append `GenerateBootstrapCommand(cfg.Name, name)...` after `loadbalancer.Image`. Provider RED tests now GREEN. No production reference to `kindest/haproxy` or `kill -s HUP 1` remains.</done>
</task>

</tasks>

<verification>
- All tests in `pkg/cluster/internal/loadbalancer/`, `pkg/cluster/internal/create/actions/loadbalancer/`, `pkg/cluster/internal/providers/{docker,podman,nerdctl}/`, and `pkg/internal/doctor/` pass with `-race`.
- `go build ./...` succeeds.
- `grep -rn "kindest/haproxy" pkg/ | grep -v _test.go` returns no production matches.
- `grep -rn "envoyproxy/envoy" pkg/cluster/internal/loadbalancer/const.go` returns the new image constant.
- `grep -n "GenerateBootstrapCommand" pkg/cluster/internal/providers/{docker/create.go,podman/provision.go,nerdctl/create.go}` returns one match per file (in the LB run-args function).
- 4 commits landed on main: 2 RED + 2 GREEN (Task 1 = RED-only; Task 2 = GREEN against Task 1's RED; Task 3 = its own RED+GREEN).
</verification>

<success_criteria>
SC1: HA clusters use Envoy as LB; `kindest/haproxy` is no longer pulled — satisfied by:
- `loadbalancer.Image` set to `docker.io/envoyproxy/envoy:v1.36.2`
- All three providers append `GenerateBootstrapCommand` so Envoy starts correctly
- Doctor offlinereadiness lists Envoy (so air-gap verification covers the new image)
- No production code references `kindest/haproxy`
</success_criteria>

<output>
After completion, create `.planning/phases/51-upstream-sync-k8s-1-36/51-01-SUMMARY.md` with: tasks executed, commits landed (SHAs), test counts before/after, list of files changed, any deviations from the plan body, and a `grep -rn "kindest/haproxy" pkg/` final-state report.
</output>
