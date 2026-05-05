---
status: diagnosed
trigger: "When 2 of 3 CPs are stopped on an HA kinder cluster, kinder doctor's cluster-resume-readiness check reports skip 'single-control-plane cluster; HA check not applicable'. The HA gate counts running CPs (1 alive), not the cluster's declared topology (3 CPs). In the exact failure mode where quorum-loss detection should matter most, the check decides the cluster isn't HA and skips."
created: 2026-05-05
updated: 2026-05-05
---

## Current Focus

hypothesis: realListCPNodes in pkg/internal/doctor/resumereadiness.go enumerates control-plane containers via plain `docker ps` (running only). When the user stops 2-of-3 CPs to simulate quorum loss, only 1 CP container is "running" so listClusterNodes returns `["verify47-control-plane"]`. The check's `len(cpNodeNames) <= 1` gate then triggers the single-CP skip — even though the cluster's declared topology has 3 CPs. CONFIRMED.
test: code inspection of pkg/internal/doctor/resumereadiness.go:291-295 vs. kind's own provider.ListNodes at pkg/cluster/internal/providers/docker/provider.go:114-134
expecting: confirm the `docker ps` invocation lacks `-a` and confirm kind has a precedent of using `docker ps -a` for cluster topology enumeration
next_action: report ROOT CAUSE FOUND with specialist_hint=go and minimal-fix shape

## Symptoms

expected: After `docker stop verify47-control-plane2 verify47-control-plane3` (forcing 2-of-3 CP loss on HA cluster verify47), `kinder doctor` reports `cluster-resume-readiness` with status `warn` and a non-empty `reason` mentioning quorum/unhealthy members. Check NEVER returns fail (warn-and-continue).
actual: Same setup yields `⊘ cluster-resume-readiness single-control-plane cluster; HA check not applicable`. The HA gate counts only running CPs (1) so the check thinks it's not an HA cluster and skips.
errors: None — graceful skip; wrong disposition.
reproduction: Test 13 in .planning/phases/47-cluster-pause-resume/47-UAT.md. On 3-CP HA cluster: `docker stop <cluster>-control-plane2 <cluster>-control-plane3 && kinder doctor`.
started: Discovered during UAT 2026-05-05 (Phase 47 SC4 reverse direction).

## Eliminated

(none — the first hypothesis was the right one)

## Evidence

- timestamp: 2026-05-05
  checked: pkg/internal/doctor/resumereadiness.go:279-312 (realListCPNodes)
  found: |
    Lines 291-295:
        lines, err := exec.OutputLines(exec.Command(
            binaryName, "ps",
            "--filter", "label=io.x-k8s.kind.cluster",
            "--format", `{{.Names}}|{{.Label "io.x-k8s.kind.role"}}`,
        ))
    The command is `docker ps` (NOT `docker ps -a`). Plain `docker ps` returns
    ONLY running containers. Stopped (exited) containers are silently omitted.
  implication: When 2-of-3 CPs are stopped via `docker stop`, only the 1 surviving CP appears in `cpNodeNames`. Combined with the gate at line 104 (`if len(cpNodeNames) <= 1 { skip }`), the check skips with "single-control-plane cluster" — exactly the observed symptom. This is the ROOT CAUSE.

- timestamp: 2026-05-05
  checked: pkg/internal/doctor/resumereadiness.go:104-111 (HA gate)
  found: |
    if len(cpNodeNames) <= 1 {
        return []Result{{
            Name:     c.Name(),
            Category: c.Category(),
            Status:   "skip",
            Message:  "single-control-plane cluster; HA check not applicable",
        }}
    }
    The skip message string "single-control-plane cluster; HA check not applicable" is a verbatim match for the symptom output. The gate compares len of the input slice — so the bug is upstream in what feeds the gate, not in the gate condition itself.
  implication: The gate's intent (skip true single-CP clusters) is correct; its INPUT is wrong. Fix must change the input — make listClusterNodes return ALL declared CPs, not only running ones.

- timestamp: 2026-05-05
  checked: pkg/cluster/internal/providers/docker/provider.go:114-134 (kind upstream ListNodes)
  found: |
    func (p *provider) ListNodes(cluster string) ([]nodes.Node, error) {
        cmd := exec.Command("docker",
            "ps",
            "-a", // show stopped nodes
            "--filter", fmt.Sprintf("label=%s=%s", clusterLabelKey, cluster),
            "--format", `{{.Names}}`,
        )
        ...
    }
    Kind's own ListNodes EXPLICITLY uses `-a` with the comment "show stopped nodes". This is the canonical pattern for enumerating a cluster's declared topology in kind/kinder.
  implication: Precedent exists. The fix is well-defined: add `-a` to the docker ps invocation in realListCPNodes (and the analogous podman/nerdctl forms when those binaries are detected). This matches what `kinder get nodes`, `kinder status`, `lifecycle.Pause`, and `lifecycle.Resume` all do (they call provider.ListNodes which uses `docker ps -a` under the hood).

- timestamp: 2026-05-05
  checked: pkg/internal/doctor/clusterskew.go:75-79 (precedent the 47-04 plan referenced)
  found: |
    Plan 47-04 line 275 said: "use existing pattern from clusterskew.go that lists all kind containers." clusterskew.go also uses plain `docker ps` (no `-a`). However, clusterskew's purpose is fundamentally different: it execs into nodes to read /kind/version — which only works on RUNNING nodes. So clusterskew's "running only" enumeration is correct FOR ITS PURPOSE.
  implication: The 47-04 plan inherited a "running-only" enumeration pattern from a context where running-only was correct, and applied it to a context (resume readiness, where the explicit goal is detecting quorum loss caused by stopped CPs) where running-only is wrong. This is a context-mismatch defect introduced during 47-04 implementation.

- timestamp: 2026-05-05
  checked: tests at pkg/internal/doctor/resumereadiness_test.go
  found: |
    All tests inject `cpNodeNames` directly via fakeReadinessOpts and never exercise realListCPNodes. The single-CP skip test (TestClusterResumeReadiness_SingleCP_Skip) passes `[]string{"kind-control-plane"}` — a TRULY single-CP cluster. There is NO test that simulates "HA cluster with stopped CPs" — i.e., no test where realListCPNodes (production code) is exercised against a topology that contains stopped containers.
  implication: The bug escaped because realListCPNodes has no integration test against `docker ps -a` semantics. The test suite stubs the listing function, masking the production-only defect.

## Resolution

root_cause: |
  pkg/internal/doctor/resumereadiness.go:291-295 (realListCPNodes) enumerates control-plane
  containers using plain `docker ps`, which lists ONLY running containers. When CPs are
  stopped (e.g. via `docker stop` to simulate quorum loss, or after `kinder pause` if pause
  uses `docker stop`), they are omitted from cpNodeNames. The HA gate at line 104
  (`if len(cpNodeNames) <= 1`) then treats the cluster as single-CP and skips with
  "single-control-plane cluster; HA check not applicable".

  This means the readiness check's "HA detection" reflects RUNTIME state (how many CPs are
  alive right now), not DECLARED TOPOLOGY (how many CPs the cluster was created with). In
  the exact scenario the check exists to detect — quorum loss from stopped CPs on an HA
  cluster — the bug causes the check to silently bail out before running etcdctl probes.

  Kind's own pkg/cluster/internal/providers/docker/provider.go:114-134 ListNodes uses
  `docker ps -a` (explicit comment: "show stopped nodes") and is the canonical way to
  enumerate a cluster's declared topology. realListCPNodes should follow that pattern.

fix: |
  Minimal fix: add `-a` to the docker ps invocation in realListCPNodes
  (pkg/internal/doctor/resumereadiness.go:291-295). Same change applies to podman/nerdctl
  forms (both honor `-a` identically).

  Diff shape:
      lines, err := exec.OutputLines(exec.Command(
          binaryName, "ps",
  +       "-a", // include stopped CPs so HA topology is detected even after CP loss
          "--filter", "label=io.x-k8s.kind.cluster",
          "--format", `{{.Names}}|{{.Label "io.x-k8s.kind.role"}}`,
      ))

  After the fix, on the verify47 reproduction (3 CPs declared, 2 stopped, 1 running):
    - cpNodeNames = ["verify47-control-plane", "verify47-control-plane2", "verify47-control-plane3"]
    - len > 1 → HA gate passes
    - bootstrap = cpNodeNames[0] = "verify47-control-plane" (the running one, alphabetically first; sort.Strings preserves this)
    - crictl/etcdctl probes run on the surviving CP and report 1/3 healthy → warn with "1/3 etcd members healthy" and "2 unhealthy etcd member(s) — quorum at risk"

  Caveat: bootstrap selection currently takes cpNodeNames[0] after sort, which could pick
  a STOPPED container if the stopped CP sorts first alphabetically. Recommended companion
  refinement: pick the first cpNode whose ContainerState is "running" (using
  lifecycle.ContainerState). Otherwise crictl exec would fail on a stopped container and
  the check would skip with "crictl unavailable". A skip with that message is still wrong
  for the user's case but is at worst a missed-warning; not a regression.

  Test gap to close: add a test that wires listClusterNodes to return e.g.
  ["cp1","cp2","cp3"] but exec into "cp1" (the only "running" one in the scenario) returns
  unhealthy etcd members → assert warn with "quorum at risk". This locks in the correct
  behavior end-to-end. Also add an integration-style test for realListCPNodes that asserts
  the docker ps command line includes "-a" (string assertion on the exec.Cmd args), since
  that's the actual production defect.

verification: (deferred — diagnosis only per goal: find_root_cause_only)
files_changed: []
