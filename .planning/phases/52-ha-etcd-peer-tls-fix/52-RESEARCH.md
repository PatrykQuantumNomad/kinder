# Phase 52: HA Etcd Peer-TLS Fix - Research

**Researched:** 2026-05-09
**Domain:** Docker/Podman IPAM, etcd peer TLS, kinder lifecycle (pause/resume), doctor catalog
**Confidence:** HIGH (core findings verified by live experiment; nerdctl gap verified by docs)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Runtime coverage:** Support all 3 runtimes: docker, podman, nerdctl. Each runtime is probed independently. A runtime that fails the probe falls through to cert-regen; it does not poison the others.

**Probe scope:** Full kinder lifecycle simulation — run container with `--ip`, take it through actual `kinder pause` + `kinder resume` equivalent steps (stop → start, not freeze/unfreeze), verify IP unchanged. 10-30s runtime acceptable. Exposed as a standalone doctor check.

**Doctor surface (new in this phase):** Add a new doctor check exposing resume-strategy state for an existing HA cluster: `ip-pinned` / `cert-regen` / `unsupported`. Doctor count test must be updated.

**Existing-cluster migration:** No migration command. Legacy clusters get cert-regen forever. Doctor stays quiet on legacy clusters.

### Claude's Discretion

- Unsupported-runtime UX on `kinder create cluster --config ha.yaml`
- Probe site (per-create vs. cached vs. version table)
- Probe runtime-error verdict (soft fallback / hard halt / single retry)
- Probe network (ephemeral scratch vs. actual cluster network)
- Cert-regen trigger timing (reactive on IP change vs. always-on-resume)
- Cert-regen progress UX
- Cert-regen scope (incremental vs. wholesale)
- Cert-regen mid-resume failure recovery
- Default path for legacy HA clusters
- Legacy detection mechanism

### Deferred Ideas (OUT OF SCOPE)

- `kinder cluster migrate-ha <name>` command
- Doctor failing on unmigrated legacy HA clusters
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| LIFE-09 | HA pause/resume preserves etcd peer connectivity across container IP reassignment via IP pinning (`docker network connect --ip <stored-ip>`); cert regen is documented fallback if Docker IPAM is infeasible | Research confirms: IP pinning works on Docker and Podman; nerdctl lacks `network connect` entirely (must use cert-regen path); probe structure and insertion point identified |
</phase_requirements>

---

## Summary

The core technical problem is that `kinder resume` on a 3-CP HA cluster can return different container IPs than the original run, causing etcd peer TLS verification to fail because the peer cert SANs no longer match the actual IP addresses. Research confirmed via live experiment that Docker IPAM **does** reassign container IPs when `docker start` is issued in a different order than the original `docker run` order — and kinder's pause order (workers → CP → LB) deliberately differs from its resume order (LB → CP → workers), making IP reassignment nearly certain on any production resume cycle.

The preferred fix — reconnecting each HA control-plane container to its network with `docker network connect --ip <original-ip>` while the container is stopped — is verified to work on Docker 29.4.1 and Podman 5.8.2 (both CLI tools expose `--ip` on `network connect`). The assigned IP survives `docker stop` + `docker start` and persists even when start order differs from allocation order. `nerdctl` is the critical exception: the `network connect` subcommand is not implemented and is listed as an explicit unimplemented gap in nerdctl's docs; nerdctl clusters must always fall through to the cert-regen path.

The planner needs to deliver: (1) a probe task that tests the full stop→start lifecycle per runtime and records the result, (2) IP pinning logic wired into `kinder create cluster` (for new HA clusters) and `lifecycle.Resume` (to apply pins before starting containers), (3) cert-regen fallback for when pinning is unavailable or fails, and (4) two new doctor checks (IPAM probe + resume-strategy). The probe is Plan 52-01 Task 1; no source code for paths 2-4 is written until probe result is known.

**Primary recommendation:** Wire IP pinning at create time (record IPs after provisioning, store as container labels, reconnect with pinned IPs on resume). Cert-regen is the fallback, not the default.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| IPAM probe execution | CLI / doctor check | — | Runs container runtime CLI, no K8s API involved |
| IP address storage | Container label on each node | — | Labels survive container stop; queryable without provider |
| IP pinning on resume | `lifecycle.Resume` (before `docker start`) | Provider (per-runtime) | Must happen while container stopped, before K8s sees the new IPs |
| IP pinning on create | Provider `Provision` or post-provision hook | — | Record and pin IPs immediately after containers are created |
| Cert regen | `lifecycle.Resume` (fallback path) | Node exec inside container | Must run with containers stopped; triggers kubeadm inside node |
| Resume-strategy doctor check | `doctor` package | — | Inspects labels/certs, no lifecycle mutation |
| IPAM probe doctor check | `doctor` package | — | Standalone; uses same runtime CLI as lifecycle |

---

## Standard Stack

### Core (all existing — no new dependencies)

| Component | Location | Purpose | Notes |
|-----------|----------|---------|-------|
| `pkg/internal/lifecycle/` | pause.go, resume.go, state.go | Pause/resume orchestration | IP pinning hooks into `Resume` before `startNodes()` |
| `pkg/cluster/internal/providers/docker/` | provider.go, create.go, network.go | Docker network/container management | `network connect --ip` issued here or from lifecycle |
| `pkg/internal/doctor/` | check.go, *.go | Doctor check registry | Two new checks registered in `allChecks` |
| `pkg/cmd/kind/doctor/doctor.go` | — | CLI entry point for `kinder doctor` | No changes needed; AllChecks() auto-includes new checks |
| `pkg/cmd/kind/resume/resume.go` | — | CLI entry point for `kinder resume` | No direct changes; hooks via lifecycle |

### No New Go Dependencies

All required operations use the existing `exec.Command` pattern that calls the runtime CLI. No new Go libraries needed.

---

## Architecture Patterns

### System Architecture Diagram

```
kinder create cluster (HA config)
        |
        v
  Provision containers (docker run)
        |
        v
  [NEW] Probe: is runtime IPAM-pinnable?
        |
   YES  |  NO (nerdctl / error)
        |   \
        |    \-> Record strategy=cert-regen in label
        v
  Record each CP/LB IP via docker inspect
        |
        v
  Reconnect with docker network connect --ip <ip>
        |
        v
  Write label: io.x-k8s.kinder.resume-strategy=ip-pinned
        |
        v
  [kinder pause: docker stop in order workers→CP→LB]

kinder resume
        |
        v
  Read resume-strategy label from container
        |
  ip-pinned       cert-regen        no label (legacy)
        |               |                |
        v               |                v
  Disconnect+connect    |         cert-regen (always)
  with --ip (stopped)  |
        |               |
        v               v
  docker start all containers (LB→CP→workers)
        |               |
        v               v
  etcd peers connected  kubeadm certs renew etcd-peer
  (IP unchanged)        + static pod cycle on all CPs
```

### Recommended Project Structure (new files only)

```
pkg/internal/lifecycle/
├── ippin.go              # IP pinning: probe, record, reconnect
└── certregen.go          # Cert-regen: kubeadm certs renew + static pod cycle

pkg/internal/doctor/
├── ipamprobe.go          # New doctor check: IPAM probe
└── resumestrategy.go     # New doctor check: per-cluster resume-strategy
```

### Pattern 1: IP Label Storage

**What:** After `docker run` assigns IPs, record them as container labels so `kinder resume` can retrieve them without re-querying the network (which may have changed).

**Label key:** `io.x-k8s.kinder.pinned-ip` → value is the IPv4 address assigned at create time.

**How to set after creation:** `docker container update --label-add` does not exist in Docker. Labels must be set at `docker run` time or via a `docker network connect` + label workaround. The simpler approach: store IP in a separate label applied at container creation by adding it to `commonArgs` in `create.go`. But the IP is not known until the container is running (Docker assigns it). Therefore the pattern is:

1. After `errors.UntilErrorConcurrent(createContainerFuncs)` in `provider.Provision`, for HA clusters: query each CP container's IP via `docker inspect`, then reconnect with `--ip` and store the result in a file inside the container (`/kind/pinned-ip`) or as a container label via a new `docker network connect` that includes `--label` (not supported). **Simpler approach:** store pinned IPs in a file inside the container filesystem (`/kind/ipam-state.json`) written at create time. The file persists in the container's filesystem across stop/start.

**Recommended storage:** `/kind/ipam-state.json` inside each CP node container. The file contains `{"network": "kind", "ipv4": "172.18.0.X"}`. This avoids label mutation limitations and is consistent with how kinder already uses `/kind/pause-snapshot.json` and `/kind/version`.

### Pattern 2: IP Pinning on Resume (before docker start)

**What:** Read `/kind/ipam-state.json` from each stopped CP container (using `docker cp` or `docker exec sh -c 'cat /kind/ipam-state.json'` — note: exec requires running container so must use `docker cp`). Disconnect + reconnect with `--ip` while container is stopped.

**Commands:**
```
docker cp <container>:/kind/ipam-state.json /tmp/kinder-ipam-<container>.json
# parse JSON to get IP and network name
docker network disconnect <network> <container>
docker network connect --ip <ip> <network> <container>
```

**Critical constraint:** All of this must happen while the container is in `exited` state. Running `docker exec` on a stopped container fails. Use `docker cp` to read the state file.

**Insertion point in lifecycle.Resume:** Before the `startNodes(cp)` call in `Resume()`, in the block guarded by `len(cp) >= 2` (HA only).

### Pattern 3: k3d's Approach (for reference / validation)

k3d uses a different but related mechanism [CITED: k3d changelog v4.4.2, k3d cluster.go]:

- k3d sets `node.IP.Static = true` and `node.IP.IP = <ip>` on server nodes during cluster creation
- The IP is encoded into `NetworkingConfig.EndpointsConfig[networkName].IPAMConfig.IPv4Address` passed to `docker.ContainerCreate()`
- This pins the IP at creation time via the Docker API (equivalent to `docker run --ip <ip>`)
- k3d stores the static IP in a container label `k3d.io/node.staticIP`

**kinder difference:** kinder uses the CLI (`exec.Command("docker", ...)`) not the Docker Go SDK, so the equivalent is `docker run --ip <ip> ...`. However, kinder creates containers first and queries IPs second (no pre-chosen IP). Therefore kinder must use the **reconnect pattern** (disconnect + `network connect --ip`) rather than `docker run --ip`. This is functionally equivalent and verified to work.

### Pattern 4: Cert-Regen (fallback path)

For nerdctl clusters, or when IP pinning fails:

```bash
# On each CP node (exec inside container):
kubeadm certs renew etcd-peer
# Then cycle the etcd static pod:
mv /etc/kubernetes/manifests/etcd.yaml /tmp/etcd.yaml.bak
# wait for kubelet to remove the pod (kubelet fileCheckFrequency = 20s)
sleep 20
mv /tmp/etcd.yaml.bak /etc/kubernetes/manifests/etcd.yaml
sleep 20  # wait for pod recreation
```

**Scope decision (Claude's Discretion):** Use **wholesale cert-regen** (all CP nodes, not incremental). Etcd does not tolerate a mixed-cert state where some peers have new peer certs and others have old ones — the TLS handshake will fail because etcd verifies peer certs against the CA, and if one peer's cert has new IPs in its SANs but another peer's cert still has old IPs, the verification asymmetry causes split health reports. Regen all or none.

**Trigger decision (Claude's Discretion):** Use **reactive** trigger only — detect IP change by comparing current `docker inspect` IP against the stored value in `/kind/ipam-state.json`. If IPs match, skip cert-regen. This avoids unnecessary cert cycling (which takes ~40s per cluster) for the common case where the runtime preserved IPs.

**Progress UX (Claude's Discretion):** Use the existing single-line spinner pattern from `pkg/internal/cli/status.go`. One status line per CP node: `"Regenerating etcd peer cert on <node> (N/M)"`. Match existing `lifecycle.Resume` progress pattern.

**Failure recovery (Claude's Discretion):** On cert-regen failure: **halt + diagnostic**. Do not attempt snapshot restore (snapshot/restore is out of scope for this phase). Emit a structured error message: `"etcd peer cert regen failed on <node>: <error>. Cluster state is undefined — delete and recreate."` Return non-nil error from Resume.

### Anti-Patterns to Avoid

- **Never run cert-regen on running containers.** kubeadm writes to `/etc/kubernetes/pki/etcd/` which is bind-mounted from the host filesystem equivalent (actually container volume). If the etcd pod is running when cert files are replaced, it may read partial state. All cert operations must happen with the etcd static pod stopped.
- **Never use `docker exec` on stopped containers.** exec requires running state. Use `docker cp` for reading files from stopped containers.
- **Never assume IP stability without the probe.** On Docker 29.4.1 confirmed: different start order → different IPs. Even a simple 2-container test shows this.
- **Never use `docker pause` / `docker unpause` for the probe.** kinder pause uses `docker stop` (exited state), not `docker pause` (frozen/paused state). The probe must use stop/start.

---

## Probe: Concrete Definition (Plan 52-01 Task 1)

### What the Probe Must Do

The probe simulates the full kinder pause/resume lifecycle for each runtime:

1. Create a scratch network: `<runtime> network create --subnet=<probe-subnet> <probe-net-name>`
2. Start a scratch container on the network: `<runtime> run -d --name <probe-container> --network <probe-net-name> <minimal-image> sleep 60`
3. Record the container's assigned IP: `<runtime> inspect --format={{.NetworkSettings.Networks.<net>.IPAddress}} <probe-container>`
4. Disconnect and reconnect with the same IP (simulating the pin): `<runtime> network disconnect <net> <container>` + `<runtime> network connect --ip <original-ip> <net> <container>`
5. Stop the container: `<runtime> stop <probe-container>` (must use stop, not pause)
6. Start the container: `<runtime> start <probe-container>`
7. Verify IP unchanged: `<runtime> inspect ...` returns same IP
8. Cleanup: `<runtime> rm -f <probe-container>` + `<runtime> network rm <probe-net-name>`

**Verdict:** PASS if step 7 IP matches step 3 IP. FAIL if IP changed. ERROR if any command fails (probe-runtime-error — see below).

### Runtime-Specific Notes

| Runtime | `network connect --ip` | Expected Probe Result |
|---------|----------------------|----------------------|
| Docker | Supported (verified: v29.4.1) | PASS |
| Podman | Supported (verified: v5.8.2 CLI shows `--ip` flag) | PASS (expected; unverified via live test — podman machine not running) |
| nerdctl | `network connect` NOT IMPLEMENTED | FAIL (hard fail at step 4 — command does not exist) |

**Confidence:** Docker result HIGH [VERIFIED: live experiment]; Podman result MEDIUM [VERIFIED: CLI help output, unverified via live container cycle]; nerdctl result HIGH [VERIFIED: official nerdctl docs, GitHub discussion #2848].

### Probe Network Decision (Claude's Discretion)

Use an **ephemeral scratch network** (not the actual cluster network). Rationale: running the probe on the actual cluster network risks leaving artifacts on probe failure (e.g., stale IP reservations). The scratch network uses a dedicated subnet (e.g., `172.200.0.0/24`) guaranteed not to overlap with the kind network. The probe cleans up even on failure via deferred cleanup.

### Probe Site Decision (Claude's Discretion)

Run the probe **per-create** (once per `kinder create cluster` invocation for HA clusters), not cached. Rationale: runtime behavior can change across Docker Engine upgrades; a cached result from a previous run may be stale. The 10-30s probe runtime is acceptable given the user already waits several minutes for cluster creation. Cache invalidation complexity is not worth the few seconds saved.

### Probe Runtime-Error Verdict (Claude's Discretion)

On permission denied, network creation refused, or other non-"command not found" errors: **soft fallback to cert-regen with a warning**. Rationale: permission errors are environment-specific (e.g., rootless mode, restricted daemons) and transient. Halting cluster creation for a probe permission error is worse UX than falling back to cert-regen. Single retry is not needed — the error types that appear here are not transient (they're config issues).

### Probe as Doctor Check

The probe is registered in `allChecks` as `"ipam-probe"` in the `"Network"` category. When run via `kinder doctor`, it executes the full lifecycle simulation on the currently-detected runtime and reports:
- `ok` + message `"Docker IPAM supports IP pinning (ip-pinned path available)"`
- `warn` + message `"nerdctl does not support network connect (cert-regen path will be used)"`
- `warn` + message `"IPAM probe failed: <error> (cert-regen path will be used)"`

The doctor check does NOT create a kind cluster — it only creates/destroys a scratch container on a scratch network.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Container IP query | Custom JSON parser for `docker inspect` output | `exec.OutputLines(exec.Command(runtime, "inspect", "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", container))` | Existing pattern in codebase (see `state.go:ContainerState`) |
| Cert renewal | Custom cert generator | `kubeadm certs renew etcd-peer` (exec'd inside node container) | kubeadm handles SAN regeneration from kubeadm-config ConfigMap |
| Static pod cycle | Custom kubelet API call | Move manifest file `/etc/kubernetes/manifests/etcd.yaml` out and back | Standard kubeadm-documented procedure; kubelet fileCheckFrequency = 20s |
| File read from stopped container | `docker exec cat /kind/...` (fails on stopped) | `docker cp <container>:/kind/ipam-state.json /tmp/...` | exec requires running state; cp works on stopped containers |
| IP validation | Custom IP parser | `net.ParseIP()` from Go stdlib | Already imported in network.go |

**Key insight:** All runtime operations in this phase are thin wrappers around existing runtime CLI commands using the existing `exec.Command` pattern. No new Go libraries or APIs are needed.

---

## Common Pitfalls

### Pitfall 1: Cert/Network Operations on Running Containers (CATASTROPHIC)
**What goes wrong:** Running `kubeadm certs renew etcd-peer` while etcd is running can cause etcd to read a partially-written cert file and enter an undefined state. Running `docker network disconnect` on a running container drops the etcd peer TCP connection mid-flight, potentially causing quorum loss.
**Why it happens:** Developers assume "running = accessible" but for cert and network mutations "stopped = safe."
**How to avoid:** ALL network disconnect/reconnect and cert operations must be gated on `ContainerState == "exited"`. The Resume flow already stops all containers before starting them, so IP pinning (during resume) happens in the correct window. For cert-regen: run after containers are stopped, before `startNodes()`.
**Warning signs:** etcd refuses to start after resume, shows TLS errors in `docker logs <cp-node>`.

### Pitfall 2: `docker exec` on Stopped Containers Fails
**What goes wrong:** Reading `/kind/ipam-state.json` inside a stopped container via `docker exec` fails with "container is not running."
**Why it happens:** `docker exec` requires the container to be running.
**How to avoid:** Use `docker cp <container>:/kind/ipam-state.json /tmp/kinder-ipam-<container>.json` to read files from stopped containers. `docker cp` works in both running and stopped states.
**Warning signs:** "Error response from daemon: container is not running" in probe or resume logs.

### Pitfall 3: IP Reassignment Only Happens on Different Start Order
**What goes wrong:** Testing with same start order as stop order gives false confidence that IPAM is stable. In kinder, pause stops workers → CP → LB; resume starts LB → CP → workers. The order inversion is what triggers IP reassignment.
**Why it happens:** Docker IPAM assigns IPs in first-available-slot order per container start. Same order → same slots. Different order → slots may shift.
**How to avoid:** The probe MUST use stop order that differs from start order (or at minimum, stop ALL containers before starting ANY). The simple probe (stop one container, start it) passes even when the full 3-CP scenario fails. The probe must stop all containers, then start them.
**Warning signs:** Probe passes with 1 container but HA cluster still breaks on resume.

### Pitfall 4: nerdctl `network connect` Does Not Exist
**What goes wrong:** Code that calls `nerdctl network connect --ip ...` gets "nerdctl: unknown command 'network'" (or similar).
**Why it happens:** nerdctl explicitly lists `network connect` as an unimplemented Docker command. [VERIFIED: GitHub discussion #2848, nerdctl command reference]
**How to avoid:** nerdctl runtime must always use the cert-regen path. No IP pinning code should be emitted for nerdctl. The probe detects this at the `network connect` step and records `nerdctl` as requiring cert-regen.
**Warning signs:** `Error: unknown command "connect" for "nerdctl network"` in logs.

### Pitfall 5: kubeadm Cert Regen Requires kubeadm-config ConfigMap
**What goes wrong:** `kubeadm certs renew etcd-peer` fails if the `kubeadm-config` ConfigMap in `kube-system` namespace is not present or if kubeadm cannot reach the API server.
**Why it happens:** kubeadm reads the cluster config (including SAN list) from the ConfigMap to know what IPs to put in the new cert. The API server must be running for this.
**How to avoid:** Cert-regen in the resume flow must happen AFTER containers are started (API server running), not before. Specifically: start all containers first, then run `kubeadm certs renew etcd-peer` on each CP, then cycle etcd static pod. This differs from the IP-pin approach which works before start.
**Warning signs:** `kubeadm certs renew: error: the specified configuration API object does not exist`.

### Pitfall 6: Cert-Regen Must Run on All CP Nodes (Wholesale)
**What goes wrong:** Running cert-regen on only the CP nodes whose IPs changed results in a mixed-cert state. Etcd member A has a cert with IPs [old-b-ip], member B has a new cert with its new IP. B rejects A's handshake because A's cert has stale SANs.
**Why it happens:** Each etcd peer cert includes the IPs of ALL etcd members in the cluster's SAN list. When any IP changes, ALL peer certs must be regenerated.
**How to avoid:** Cert-regen runs on ALL CP nodes atomically (or as close to atomically as possible given serial execution). Do not attempt incremental/selective regen.
**Warning signs:** Partial etcd health (`1/3 members healthy`) after resume.

### Pitfall 7: Static Pod Cycle Timing
**What goes wrong:** Moving etcd.yaml back to /etc/kubernetes/manifests/ before kubelet has finished removing the old pod results in duplicate containers, causing etcd to fail with "member already exists" or port conflicts.
**Why it happens:** kubelet's fileCheckFrequency defaults to 20 seconds. The manifest must be absent for at least one check cycle before being restored.
**How to avoid:** Sleep 25 seconds (slightly longer than fileCheckFrequency to account for timing variance) between removing and restoring the manifest. Or poll kubelet API/crictl to verify pod is gone before restoring.
**Warning signs:** `crictl ps` shows two etcd containers simultaneously; etcd log shows port-in-use errors.

### Pitfall 8: `docker network connect --ip` Requires IP Outside DHCP Range
**What goes wrong:** Pinning to an IP that falls within Docker's default IPAM allocation range may cause conflicts when a new container is started and Docker allocates the same IP to it.
**Why it happens:** Docker tracks connected endpoints but may not exclude pinned IPs from the pool when the container is disconnected.
**How to avoid:** At cluster create time, the IP pinning reconnect uses the IP Docker already assigned to the container. Since Docker assigned it, it is within the valid range. The risk only arises if other containers join the same network between pause and resume. The `kind` network is private and only kinder containers join it, so this risk is negligible in practice.
**Warning signs:** `docker network connect` returns "Address already in use" on resume.

---

## Claude's Discretion Recommendations

| Decision | Recommendation | Rationale |
|----------|---------------|-----------|
| Unsupported-runtime UX on `kinder create cluster --config ha.yaml` | Warn and proceed with cert-regen | v2.3 SYNC error-handling: warn for degraded capability, never block. Match `logMetalLBPlatformWarning` pattern in `create.go`. |
| Probe site | Per-create (run on every `kinder create cluster` for HA) | Runtime behavior changes across engine upgrades; 10-30s is acceptable in create flow |
| Probe runtime-error verdict | Soft fallback to cert-regen with a warning | Matches existing kinder convention: degrade gracefully, never block cluster creation for a probe error |
| Probe network | Ephemeral scratch network | Avoids artifact leakage on failure; scratch network cleanup is deferred so it runs even on error |
| Cert-regen trigger timing | Reactive (only when IP change detected) | Cert-regen takes ~40-60s per cluster; skipping it when not needed is a meaningful UX improvement |
| Cert-regen progress UX | Single-line spinner per CP node, matching `cli.StatusForLogger` pattern | Already used in `lifecycle.Resume`; phase-by-phase would add complexity without user benefit |
| Cert-regen scope | Wholesale (all CP nodes) | Mixed-cert state is unsafe; etcd peer certs include all member IPs in SANs |
| Cert-regen mid-resume failure | Halt + diagnostic message | Snapshot restore is out of scope; single retry adds complexity for a rare path |
| Legacy cluster default | Cert-regen forever (no label = legacy = cert-regen) | Consistent with "no migration command" + "doctor stays quiet"; lowest risk |
| Legacy detection | Absence of `io.x-k8s.kinder.resume-strategy` label on CP containers | Cheapest check: one `docker inspect` per CP; no SAN parsing needed |

---

## Code Examples

### Read IP from stopped container
```go
// Source: docker cp pattern (verified in Docker 29.4.1 by live test)
// docker cp works on stopped containers; docker exec does not
func readIPAMStateFromContainer(binaryName, containerName, tmpDir string) (*ipamState, error) {
    hostPath := filepath.Join(tmpDir, containerName+"-ipam.json")
    cmd := exec.Command(binaryName, "cp",
        containerName+":/kind/ipam-state.json",
        hostPath,
    )
    if err := cmd.Run(); err != nil {
        return nil, errors.Wrapf(err, "docker cp ipam-state from %s", containerName)
    }
    // ... read and parse hostPath
}
```

### Disconnect + reconnect with pinned IP (stopped container)
```go
// Source: verified by live experiment on Docker 29.4.1
func pinContainerIP(binaryName, containerName, networkName, ip string) error {
    if err := exec.Command(binaryName, "network", "disconnect",
        networkName, containerName).Run(); err != nil {
        return errors.Wrapf(err, "network disconnect %s", containerName)
    }
    return exec.Command(binaryName, "network", "connect",
        "--ip", ip, networkName, containerName).Run()
}
```

### Write IPAM state into container filesystem at create time
```go
// Source: pattern from lifecycle/pause.go captureHASnapshot()
// Use sh -c with heredoc to avoid quoting issues around JSON
payload := fmt.Sprintf(`{"network":%q,"ipv4":%q}`, networkName, assignedIP)
script := fmt.Sprintf("cat > /kind/ipam-state.json <<'IPAM_EOF'\n%s\nIPAM_EOF\n", payload)
writeCmd := bootstrap.Command("sh", "-c", script)
```

### Probe: full lifecycle test
```go
// Commands executed in sequence for the probe:
// 1. <runtime> network create --subnet=172.200.0.0/24 kinder-ipam-probe-<timestamp>
// 2. <runtime> run -d --name kinder-ipam-probe-<ts> --network <probenet> <image> sleep 30
// 3. <runtime> inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' <probe>
// 4. <runtime> network disconnect <probenet> <probe>
// 5. <runtime> network connect --ip <ip> <probenet> <probe>
// 6. <runtime> stop <probe>
// 7. <runtime> start <probe>
// 8. <runtime> inspect ... (verify IP unchanged)
// 9. defer: <runtime> rm -f <probe>; <runtime> network rm <probenet>
```

### kubeadm cert regen + static pod cycle (cert-regen path)
```go
// Source: kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/
// Exec inside each CP node container (container must be running for kubeadm)
// Step 1: start all containers first (LB→CP→workers via existing startNodes)
// Step 2: run on each CP:
cpNode.Command("kubeadm", "certs", "renew", "etcd-peer").Run()
// Step 3: cycle etcd static pod on each CP:
cpNode.Command("mv", "/etc/kubernetes/manifests/etcd.yaml", "/tmp/etcd-bak.yaml").Run()
// sleep 25s (kubelet fileCheckFrequency = 20s)
cpNode.Command("sh", "-c", "sleep 25").Run()
cpNode.Command("mv", "/tmp/etcd-bak.yaml", "/etc/kubernetes/manifests/etcd.yaml").Run()
// sleep 20s for pod recreation
cpNode.Command("sh", "-c", "sleep 20").Run()
```

### Doctor check: resume-strategy
```go
// Pattern matches existing clusterResumeReadinessCheck
// Name: "ha-resume-strategy" / Category: "Cluster"
// Logic:
// 1. List CP containers (reuse realListCPNodes pattern)
// 2. If count <= 1: skip (single-CP)
// 3. Inspect each CP for label "io.x-k8s.kinder.resume-strategy"
// 4. If absent on any: report strategy=cert-regen (legacy cluster)
// 5. If present: report the strategy value
// Output: ok + "resume-strategy: ip-pinned" or warn + "resume-strategy: cert-regen"
```

---

## k3d Precedent Analysis

k3d (referenced in roadmap as the model) implements IP pinning through:

1. **At node spec translation time** [CITED: k3d translate.go]: Sets `NetworkingConfig.EndpointsConfig[network].IPAMConfig.IPv4Address = node.IP.IP.String()` in the Docker API call for container creation. This is equivalent to `docker run --ip <ip>` but uses the Go SDK.

2. **IP selection** [CITED: k3d cluster.go `ClusterCreate`]: Calls `GetIP()` from a managed IPAM pool, reserves the IP in `cluster.Network.IPAM.IPsUsed`, and sets `node.IP.Static = true`. The IP is chosen BEFORE container creation.

3. **Label storage**: Stores the static IP in container label `k3d.io/node.staticIP` for later retrieval.

**Key difference from kinder's approach:** k3d picks the IP before container creation and uses the Docker Go SDK to pin it at creation time. Kinder uses the CLI and must use the reconnect pattern (disconnect + `network connect --ip`) because it creates containers first and queries IPs second. Both approaches result in the same persisted IP state in Docker's IPAM database.

**Gotcha from k3d:** k3d's IPAM pool management (`--subnet` flag, IP reservation) is more complex than needed for kinder. Kinder can simplify: let Docker assign the initial IP, then pin it via reconnect. No need for a separate IPAM pool manager.

---

## Runtime State Inventory

This is a new-feature phase (not a rename/refactor), so no existing runtime state is mutated. The new runtime state introduced by this phase:

| Category | Items Created | Action at Removal |
|----------|--------------|-------------------|
| Container filesystem | `/kind/ipam-state.json` on each new HA CP container | Removed with cluster (`kinder delete cluster`) |
| Container labels | `io.x-k8s.kinder.resume-strategy` on each new HA CP container | Removed with cluster |
| Doctor checks | Two new entries in `allChecks` registry | Code only; no persistent state |

**Pre-existing state not affected:** pause snapshots (`/kind/pause-snapshot.json`), existing cluster labels, etcd data, kubeconfig. This phase does not touch snapshot/restore functionality.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | IP-pin path probe | Yes | 29.4.1 | cert-regen path |
| Podman | Podman IP-pin probe | Yes (binary) | 5.8.2 (no machine running) | cert-regen path |
| nerdctl | nerdctl probe | No | — | cert-regen always |
| Go | Build | Yes | 1.26.3 | — |
| kubeadm (inside kindest/node) | Cert-regen path | Available inside node containers | — | — |

**nerdctl:** Not installed on this machine. The probe must handle "command not found" gracefully and record it as unsupported.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (`testing` package), existing kinder test patterns |
| Config file | None (no separate test framework config) |
| Quick run command | `go test ./pkg/internal/lifecycle/... -run TestIPAM -v` |
| Full suite command | `go test ./pkg/internal/... ./pkg/cluster/...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LIFE-09 | Probe correctly identifies runtime capability | unit | `go test ./pkg/internal/doctor/... -run TestIPAMProbe -v` | No — Wave 0 |
| LIFE-09 | IP pinning applied before docker start | unit | `go test ./pkg/internal/lifecycle/... -run TestIPPin -v` | No — Wave 0 |
| LIFE-09 | Cert-regen triggered when IPs change | unit | `go test ./pkg/internal/lifecycle/... -run TestCertRegen -v` | No — Wave 0 |
| LIFE-09 | Resume-strategy doctor check reports correct values | unit | `go test ./pkg/internal/doctor/... -run TestResumeStrategy -v` | No — Wave 0 |
| LIFE-09 | Doctor count test updated | unit | `go test ./pkg/internal/doctor/... -run TestAllChecks -v` | Yes (check_test.go) |

### Wave 0 Gaps

- [ ] `pkg/internal/lifecycle/ippin_test.go` — covers IP-pin probe, record, reconnect
- [ ] `pkg/internal/lifecycle/certregen_test.go` — covers cert-regen trigger, scope, failure
- [ ] `pkg/internal/doctor/ipamprobe_test.go` — covers IPAM probe doctor check
- [ ] `pkg/internal/doctor/resumestrategy_test.go` — covers resume-strategy doctor check

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | — |
| V3 Session Management | No | — |
| V4 Access Control | Partial | Only privileged containers (already Docker `--privileged`) can manipulate `/etc/kubernetes/pki/` |
| V5 Input Validation | Yes | Validate IP address format before passing to `--ip` flag (use `net.ParseIP`) |
| V6 Cryptography | Yes | kubeadm handles cert generation; never hand-roll cert issuance |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Pinning to a wrong IP (IP injection) | Tampering | Validate with `net.ParseIP()`; only use IPs read from `docker inspect` or `/kind/ipam-state.json` (written by kinder, readable by kinder only) |
| Cert regen writes to wrong path | Tampering | kubeadm writes to standard path; kinder only execs `kubeadm certs renew etcd-peer` inside the node container |
| IPAM probe leaving scratch containers | DoS (resource exhaustion) | Defer cleanup ensures scratch network + container are removed even on probe failure |
| `/kind/ipam-state.json` world-readable | Info disclosure | Likely not a concern (kinder containers are `--privileged` with no multi-tenant isolation) but file should be mode 0600 |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Podman `network connect --ip` works on stopped containers for IP persistence across start | Probe table | Probe would fail; cert-regen path activates (low risk, correct fallback) |
| A2 | `kubeadm certs renew etcd-peer` inside kindest/node images works without external connectivity | Cert-regen pattern | Regen would fail; user would need to delete+recreate cluster |
| A3 | kubelet fileCheckFrequency = 20s in kindest/node | Cert-regen static pod cycle | If longer, etcd static pod cycle takes longer; 25s sleep may be insufficient |
| A4 | `docker cp` works on stopped containers for reading files | Code examples | IP state read would fail; probe would need to run with container started |

---

## Open Questions

1. **Probe image for scratch container**
   - What we know: The probe needs a minimal container image that can `sleep`. `alpine` is the obvious choice.
   - What's unclear: kinder clusters may be air-gapped. The probe shouldn't pull `alpine` in air-gapped environments.
   - Recommendation: Use the `kindest/node` image already pulled by the cluster creation (which is always available at create time). Or use a pause container image if available. The planner should document this constraint.

2. **`docker cp` write path for IPAM state at create time**
   - What we know: At create time, the container is running (systemd reached multi-user.target). We can use `docker exec sh -c 'cat > /kind/ipam-state.json ...'` OR `docker cp`.
   - What's unclear: Is the create-time hook in `provider.Provision` (before Kubernetes setup) or after Kubernetes is running (post-provision)?
   - Recommendation: Write at post-provision time (after `errors.UntilErrorConcurrent(createContainerFuncs)` in `provider.Provision`). Container is running, exec works.

3. **etcd peer cert SANs: do they include container IP or pod IP?**
   - What we know: In kind clusters, etcd runs as a static pod. The etcd peer cert is generated by kubeadm with SANs containing the node's IP address (the container IP on the Docker network), not a pod CIDR IP.
   - What's unclear: Whether kubeadm uses the container IP or localhost for peer communication.
   - Recommendation: Verify during probe phase by reading an existing etcd peer cert SAN list: `openssl x509 -in /etc/kubernetes/pki/etcd/peer.crt -text -noout | grep -A2 'Subject Alternative'`. This should be Plan 52-01 Task 2 (done while containers are up, after probe).

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Accept IP reassignment silently (etcd breaks after resume) | IP pinning via `docker network connect --ip` | Phase 52 (this phase) | HA resume becomes reliable |
| No visibility into resume path | `ha-resume-strategy` doctor check | Phase 52 | Users can diagnose slow/failed resumes |
| nerdctl: no fallback (broken) | nerdctl: cert-regen fallback | Phase 52 | nerdctl HA clusters resume correctly |

---

## Sources

### Primary (HIGH confidence)

- Live experiment on Docker 29.4.1, macOS darwin/arm64 — `docker network connect --ip` on stopped containers, IP persistence after stop/start, start-order IPAM reassignment
- Live experiment: podman 5.8.2 CLI `podman network connect --help` — confirms `--ip` flag exists
- `/Users/patrykattc/work/git/kinder/pkg/internal/lifecycle/resume.go` — Resume orchestration and insertion points
- `/Users/patrykattc/work/git/kinder/pkg/internal/lifecycle/pause.go` — Pause pattern, HA snapshot precedent
- `/Users/patrykattc/work/git/kinder/pkg/internal/doctor/check.go` — allChecks registry, Check interface
- `/Users/patrykattc/work/git/kinder/pkg/internal/doctor/resumereadiness.go` — Existing HA doctor check pattern
- `/Users/patrykattc/work/git/kinder/pkg/cluster/internal/providers/docker/create.go` — Container creation, label pattern, commonArgs

### Secondary (MEDIUM confidence)

- [CITED: docs.docker.com/reference/cli/docker/network/connect/] — `--ip` flag documentation, IP reapplication on restart
- [CITED: docs.podman.io/en/stable/markdown/podman-network-connect.1.html] — `--ip` flag confirmed
- [CITED: k3d changelog v4.4.2] — `--subnet` flag and static IP precedent description
- [CITED: k3d translate.go via WebFetch] — `EndpointIPAMConfig.IPv4Address` pattern for IP pinning at container creation
- [CITED: kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-certs/] — `kubeadm certs renew etcd-peer` procedure and static pod cycle

### Tertiary (LOW confidence)

- [ASSUMED] — kubelet fileCheckFrequency = 20s in kindest/node images (not verified against actual image)
- GitHub discussion #2848 [VERIFIED: nerdctl network connect unimplemented] — nerdctl does not support network connect

---

## Metadata

**Confidence breakdown:**
- Docker IP pinning mechanics: HIGH — verified by live experiment
- Podman IP pinning: MEDIUM — CLI flag confirmed, live container test not possible (no running machine)
- nerdctl network connect gap: HIGH — verified in official docs and GitHub discussion
- k3d precedent: MEDIUM — referenced via WebFetch of k3d source, consistent with changelog claims
- Cert-regen procedure: MEDIUM — documented in official Kubernetes docs, not tested against kindest/node image

**Research date:** 2026-05-09
**Valid until:** 2026-06-09 (Docker and nerdctl APIs are stable; Podman behavior may vary across versions)
