# Phase 52: HA Etcd Peer-TLS Fix - Context

**Gathered:** 2026-05-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Make HA cluster `kinder pause` + `kinder resume` survive Docker IPAM IP reassignment so etcd peer TLS connectivity does not break on resume. Single-CP clusters incur zero overhead. The roadmap locks: IP pinning is the preferred path (k3d precedent), cert-regen is the documented fallback, and the Docker IPAM feasibility probe is Plan 52-01 Task 1 (no source code is written until probe result is known).

Out of scope: new lifecycle commands, snapshot/restore changes, doctor cosmetics (Phase 57 owns those), and any addon-related work (Phase 53 owns that).

</domain>

<decisions>
## Implementation Decisions

### Runtime coverage
- **Support all 3 runtimes**: docker, podman, nerdctl. The fix targets parity with v2.3 SYNC LB work which already crosses these runtimes.
- podman exposes `podman network connect --ip` and nerdctl is Docker-CLI-compatible — both are expected to support static IP assignment, but the probe is what proves it for each.
- Each runtime is probed independently. A runtime that fails the probe falls through to cert-regen on its clusters; it does not poison the others.

### Probe (lifecycle scope and surfacing)
- **Probe scope: full kinder lifecycle simulation.** The probe must run a container with `--ip`, take it through the actual `kinder pause` + `kinder resume` equivalent steps (or the runtime's closest analogue), and verify the IP is unchanged after resume. A simple `docker pause`/`unpause` round-trip is insufficient — the probe must catch every reassignment trigger that production hits.
- A 10-30s probe runtime is acceptable.
- **Probe is exposed as a standalone doctor check** (e.g., `kinder doctor ipam-probe` or named within the existing doctor catalog). Users can validate HA capability without creating a cluster.

### Doctor surface (new in this phase)
- **Add a new doctor check that exposes the resume-strategy state for an existing HA cluster** — values: `ip-pinned` / `cert-regen` / `unsupported`. Helps users diagnose slow resumes and understand which path their cluster is on.
- This new check is added in Phase 52 (not deferred to Phase 57). Phase 57 owns separate cosmetic fixes.
- The doctor count test (`TestAllAddonImages_CountMatchesExpected`-style coverage in the doctor package) must be updated for the new check.

### Existing-cluster migration
- **No migration command.** If a user wants the IP-pin path on a pre-v2.4 HA cluster, the only path is delete and recreate. Zero new top-level command surface for migration.
- **Doctor stays quiet on legacy clusters.** No warning, no failed check on legacy HA clusters that haven't migrated. The new resume-strategy doctor check (above) will reflect their `cert-regen` state if the user runs it, but doctor itself does not nag.

### Claude's Discretion
The user explicitly delegated these decisions:

- **Unsupported-runtime UX on `kinder create cluster --config ha.yaml`** — auto-fall to cert-regen vs. warn-and-proceed vs. block HA. Pick the option most consistent with v2.3 SYNC error-handling conventions.
- **Probe site** — per-create vs. cached in user state vs. hardcoded version table. Pick what minimizes time-to-first-cluster while staying correct across runtime upgrades.
- **Probe runtime-error verdict** (permission denied, network creation refused, etc.) — soft fallback, hard halt, or single retry. Pick what matches existing kinder error-handling.
- **Probe network** — ephemeral scratch network vs. the actual cluster network. Pick whatever is most representative without leaving artifacts on probe failure.
- **Cert-regen trigger timing** — reactive (only on detected IP change) vs. always-on-resume. Pick based on etcd peer cert verification semantics.
- **Cert-regen progress UX** — one-line spinner vs. phase-by-phase vs. silent-unless-slow. Match existing kinder spinner conventions.
- **Cert-regen scope** — incremental (only changed nodes) vs. wholesale. Pick based on etcd's tolerance for mixed-cert states during regen.
- **Cert-regen mid-resume failure recovery** — halt + diagnostic vs. auto-restore from v2.3 snapshot vs. single retry. Pick based on snapshot/restore integration cost in this phase.
- **Default path for legacy (pre-v2.4) HA clusters** — cert-regen forever vs. auto-migrate on first resume vs. block-with-opt-in. Combined with the locked "no migration command" + "doctor stays quiet" decisions, the lowest-risk path is implied (cert-regen forever) but Claude validates against migration-risk profile.
- **Legacy detection mechanism** — cluster metadata version field vs. inspecting etcd peer cert SANs. Pick whichever is cheapest given what kinder already records per cluster.

</decisions>

<specifics>
## Specific Ideas

- **k3d precedent**: roadmap explicitly cites k3d's IP-pinning approach as the model. Researcher should investigate exactly how k3d does it (network creation, IP allocation, container start ordering) and identify any gotchas in their implementation.
- **`docker network connect --ip` is the canonical syntax** for the Docker path; podman/nerdctl analogues must be confirmed via the probe.
- **Pitfall 1 (catastrophic)**: cert/network operations on running containers can cause quorum loss / data corruption — the planner must ensure all operations are gated on container-stopped state where required.
- **Standalone diagnostic surface**: the user wants the probe and the resume-strategy state both reachable through `kinder doctor`, not buried inside `kinder create cluster` flow only.

</specifics>

<deferred>
## Deferred Ideas

- **Explicit `kinder cluster migrate-ha <name>` command** — discussed and rejected for v2.4. If user demand emerges, revisit as its own phase in a later milestone.
- **Doctor failing on unmigrated legacy HA clusters** — discussed and rejected. The resume-strategy check provides the signal; nagging is undesirable noise.

</deferred>

---

*Phase: 52-ha-etcd-peer-tls-fix*
*Context gathered: 2026-05-09*
