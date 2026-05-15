---
phase: 58-live-uat-closure-for-phase-47-51
plan: 02
type: execute
wave: 2
depends_on: ["58-01"]
files_modified:
  - hack/uat-51-envoy-ipvs-guide.sh
  - hack/uat-51-envoy-ipvs-guide.log
  - .planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md
  - .planning/REQUIREMENTS.md
  - .planning/ROADMAP.md
  - .planning/STATE.md
autonomous: false
requirements:
  - UAT-02

planner_decisions:
  - id: "a) 51-UAT.md augment vs replace"
    decision: "augment (Option A) — append `## Re-verification against v2.4 binary` section dated 2026-05-12+; preserve the May 7 narrative; frontmatter `status:` stays `complete`"
    rationale: "Non-destructive; preserves milestone narrative (\"we tested in May, re-tested at v2.4 close, both passed\"); matches RESEARCH §'Existing State of 51-UAT.md' recommendation; aligns with the spirit of ROADMAP SC3 'created with full evidence against the final v2.4 binary' without erasing earlier evidence."
  - id: "e) 58-01 vs 58-02 ordering"
    decision: "sequential — 58-02 depends on 58-01 (wave 2 after wave 1)"
    rationale: "Conceptually independent, but share host Docker VM RAM; concurrent HA cluster creates would compete. 58-01 creates 6-container cluster (heavier), 58-02 creates 4-container cluster (lighter); sequential ordering produces clean evidence narrative."
  - id: "c) manual vs CI execution"
    decision: "manual-only for v2.4"
    rationale: "Same as 58-01. RESEARCH Open Question 2."
  - id: "d) cluster teardown default"
    decision: "leave-up by default; explicit --teardown flag or TEARDOWN=yes env var"
    rationale: "Same as 58-01. RESEARCH Open Question 4."
  - id: "b) versionPreRelease source change scope"
    decision: "out of scope for Phase 58; deferred to v2.5"
    rationale: "Same as 58-01. The strings-marker freshness gate (RESEARCH Pattern 1) is sufficient."

must_haves:
  truths:
    - "hack/uat-51-envoy-ipvs-guide.sh exists, is executable, passes `bash -n` and (if available) `shellcheck`, and starts with `set -euo pipefail`"
    - "Preamble matches 58-01 — `make build` + 5 POSITIVE + 3 NEGATIVE strings markers + absolute `${REPO_ROOT}/bin/kinder` invocations + Docker capacity print + re-entrancy guard for cluster `uat-58-02`"
    - "Test 01 (Envoy LB): creates a 2-CP + 1-worker cluster `uat-58-02` (4 containers including auto-created LB), asserts `docker ps --filter label=io.x-k8s.kind.cluster=uat-58-02 --filter name=external-load-balancer --format '{{.Image}}'` equals `docker.io/envoyproxy/envoy:v1.36.2`, AND asserts NO container anywhere has image matching `kindest/haproxy`"
    - "Test 02 (IPVS+1.36 reject): writes a config with `kubeProxyMode: ipvs` + `image: kindest/node:v1.36.0` to a script-local temp path, invokes `./bin/kinder create cluster --config <path> --name should-not-exist`, asserts (a) exit != 0, (b) stderr contains all 4 required substrings from validate.go:80-100, (c) `docker ps -a --filter name=should-not-exist` returns zero rows (no container created)"
    - "Test 02 required-substring assertions: `kubeProxyMode: ipvs is not supported with Kubernetes 1.36+`, `kube-proxy IPVS mode was deprecated in v1.35`, `Switch to iptables or nftables`, `https://kubernetes.io/docs/reference/networking/virtual-ips/`"
    - "Test 03 (1.36 guide renders): `cd kinder-site && (test -d node_modules || npm install) && npm run dev &`; waits for `curl -sf http://localhost:4321/guides/k8s-1-36-whats-new/` to return 200 within 60s; greps response body for `User Namespaces` AND `In-Place Pod Resize` substrings; kills the dev-server PID on success or failure via EXIT trap"
    - "Test 03 graceful-skip path: if `npm` is not on PATH, prints `[SKIP] test_03 — npm not installed; rendering deferred to a machine with kinder-site toolchain` and continues (no fail)"
    - "Astro dev-server zombie guard (RESEARCH Pitfall C): preamble checks `lsof -i :4321` and fails fast with a clear message if port 4321 is already bound; EXIT/INT/TERM trap kills `$DEV_PID` with TERM then waits"
    - "Re-entrancy: detects existing `uat-58-02` cluster; honors REUSE=no for auto-delete; default = print cleanup command and exit 1"
    - "All script output is `tee`'d to `hack/uat-51-envoy-ipvs-guide.log` (path overridable via $LOG)"
    - "51-UAT.md gains a new section titled `## Re-verification against v2.4 binary (Phase 58)` BELOW the existing `## Notes` section (preserves the May 7 narrative above; no existing line is deleted or rewritten); the new section contains three sub-blocks (test 01, test 02, test 03) each with `dated:` header (UTC), captured commands, captured output excerpts, `result: pass`"
    - "51-UAT.md frontmatter `updated:` timestamp is refreshed to the current UTC ISO-8601 stamp; frontmatter `status:` STAYS `complete`; `source:` MAY append the Phase-58 plan ID"
    - "REQUIREMENTS.md UAT-02 checkbox transitions `[ ]` -> `[x]`; Traceability table row `UAT-02 | Phase 58 | Pending` transitions to `Complete`; bottom-of-file `Last updated:` line refreshed (this is the phase-close edit, gated on BOTH 58-01 and 58-02 passing — described in Task 3)"
    - "REQUIREMENTS.md UAT-01 checkbox ALSO transitions `[ ]` -> `[x]` and its Traceability row to `Complete` — Phase 58 closes BOTH requirements together in the same commit (this is the phase-close commit owned by 58-02 since it is the terminal plan)"
    - "ROADMAP.md Phase 58 row in the Progress table transitions to `Complete` with `2/2` plans; the Phase 58 entry in `### v2.4 Hardening (In Progress)` checkbox flips `[ ]` -> `[x]` with completion-narrative bullet matching the style of Phase 52-57 rows"
    - "STATE.md frontmatter `milestone_status`, `progress` block, and `stopped_at` updated to reflect Phase 58 closed; Performance Metrics table gains rows for 58-01 and 58-02; Decisions section gains an entry recording the 5 planner-decisions outcomes"
    - "hack/uat-51-envoy-ipvs-guide.log is committed (NOT gitignored) and contains the verbatim output of one successful run"
  artifacts:
    - path: "hack/uat-51-envoy-ipvs-guide.sh"
      provides: "Executable bash script with preamble + test_01 (Envoy LB) + test_02 (IPVS reject) + test_03 (astro dev guide) + finalize"
      contains: "envoyproxy/envoy:v1.36.2"
    - path: "hack/uat-51-envoy-ipvs-guide.sh"
      provides: "IPVS-1.36 4-substring assertion"
      contains: "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+"
    - path: "hack/uat-51-envoy-ipvs-guide.sh"
      provides: "Astro dev-server background lifecycle with EXIT trap"
      contains: "kill -TERM"
    - path: "hack/uat-51-envoy-ipvs-guide.log"
      provides: "Verbatim log of one successful run (canonical evidence)"
      contains: "[OK] test_01"
    - path: ".planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md"
      provides: "Augmented re-verification section (Option A)"
      contains: "Re-verification against v2.4 binary"
    - path: ".planning/REQUIREMENTS.md"
      provides: "UAT-01 + UAT-02 transitioned to Complete in checkboxes and Traceability table"
      contains: "UAT-02 | Phase 58 | Complete"
    - path: ".planning/ROADMAP.md"
      provides: "Phase 58 marked Complete with 2/2 plans"
      contains: "58. Live UAT Closure for Phase 47 + 51"
    - path: ".planning/STATE.md"
      provides: "Phase 58 close-out: milestone progress, performance metrics, decisions record"
      contains: "Phase 58"
  key_links:
    - from: "hack/uat-51-envoy-ipvs-guide.sh test_01"
      to: "loadbalancer/const.go (51-01 Envoy image constant)"
      via: "docker ps --filter --format '{{.Image}}' exact equality"
      pattern: "docker.io/envoyproxy/envoy:v1.36.2"
    - from: "hack/uat-51-envoy-ipvs-guide.sh test_02"
      to: "pkg/internal/apis/config/validate.go:80-100 (51-02 IPVS guard)"
      via: "4-substring grep on captured stderr after non-zero exit"
      pattern: "kubeProxyMode: ipvs is not supported"
    - from: "hack/uat-51-envoy-ipvs-guide.sh test_03"
      to: "kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md (51-03 guide)"
      via: "curl http://localhost:4321/guides/k8s-1-36-whats-new/ + body grep"
      pattern: "User Namespaces|In-Place Pod Resize"
    - from: "51-UAT.md `## Re-verification against v2.4 binary` section"
      to: "hack/uat-51-envoy-ipvs-guide.log"
      via: "per-test sub-block citing the captured command + output excerpt"
      pattern: "result: pass"
---

<objective>
Close UAT-02 (REQUIREMENTS.md) by delivering a self-contained bash script that re-verifies the Phase 51 success criteria (Envoy LB image, IPVS-on-1.36 validate rejection, K8s 1.36 guide page renders) against the rebuilt v2.4 `./bin/kinder`, captures live evidence into a committed log, augments `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md` with a new `## Re-verification against v2.4 binary` section (Option A — preserve May 7 narrative), and — as the terminal plan of Phase 58 — flips REQUIREMENTS.md UAT-01 + UAT-02 to Complete, marks ROADMAP Phase 58 complete with 2/2 plans, and rolls STATE.md.

Purpose: ROADMAP Phase 58 SC3 + SC4 ("Phase 51 UAT: docker ps confirms `envoyproxy/envoy` (not `kindest/haproxy`) as the LB container on the HA cluster; `kinder create cluster --config <ipvs+1.36-config>` is rejected at validate with migration URL in the error message; K8s 1.36 guide page renders with its sidebar entry; `51-UAT.md` created with full evidence"). The Pitfall 23 (stale-PATH-binary) gate is honored via the same `make build` + strings-marker technique as 58-01 (researcher Pattern 1).

Output:
  - `hack/uat-51-envoy-ipvs-guide.sh` — NEW (≤ 200 lines). Preamble + test_01 (Envoy LB on 2-CP cluster `uat-58-02`) + test_02 (IPVS+1.36 reject, no container created) + test_03 (astro dev server boots, guide renders with both GA-feature headings) + finalize. `set -euo pipefail`; absolute `${REPO_ROOT}/bin/kinder`; EXIT trap for dev-server cleanup; leave-up-on-failure.
  - `hack/uat-51-envoy-ipvs-guide.log` — NEW (committed). Verbatim `tee` output.
  - `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md` — EDITED (Option A augment). New section `## Re-verification against v2.4 binary (Phase 58)` with 3 sub-blocks; frontmatter `updated:` refreshed; `status:` stays `complete`.
  - `.planning/REQUIREMENTS.md` — EDITED. UAT-01 + UAT-02 -> `[x]`; Traceability rows -> `Complete`; bottom-of-file `Last updated:` line.
  - `.planning/ROADMAP.md` — EDITED. Phase 58 entry checkbox flipped; Progress table updated; sub-plan list under Phase 58 finalized.
  - `.planning/STATE.md` — EDITED. Progress, Performance Metrics, Decisions sections.
</objective>

<execution_context>
@/Users/patrykattc/.claude/get-shit-done/workflows/execute-plan.md
@/Users/patrykattc/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/REQUIREMENTS.md
@.planning/research/PITFALLS.md
@.planning/phases/58-live-uat-closure-for-phase-47-51/58-RESEARCH.md
@.planning/phases/58-live-uat-closure-for-phase-47-51/58-01-ha-smoke-PLAN.md
@.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md
@.planning/phases/51-upstream-sync-k8s-1-36/51-VERIFICATION.md
@pkg/cluster/internal/loadbalancer/const.go
@pkg/internal/apis/config/validate.go
@kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md
@kinder-site/astro.config.mjs

<interfaces>
<!-- All verified at HEAD on 2026-05-12. -->

**Locked script path:** `hack/uat-51-envoy-ipvs-guide.sh` — descriptive slug; the matching log path is `hack/uat-51-envoy-ipvs-guide.log`. REQUIREMENTS.md UAT-02 wording does not lock a specific filename for the Phase 51 script (unlike UAT-01 which names `hack/uat-47-ha-smoke.sh`). This plan adopts the name from 58-RESEARCH.md "Recommended Project Structure" diagram. The 1:1:1 Plan:Script:Requirement shape mirrors 58-01.

**Cluster name:** `uat-58-02` — distinct from 58-01's `uat-58-01` to allow re-runs without name collision.

**Cluster topology for test 01:** 2 CP + 1 worker (4 containers including auto-created LB). 51-UAT.md test 1 used this exact shape — preserve continuity.

**Envoy LB image (51-01 deliverable, locked at `pkg/cluster/internal/loadbalancer/const.go:20`):**

```go
const Image = "docker.io/envoyproxy/envoy:v1.36.2"
```

**IPVS-1.36 4-substring assertion (51-02 deliverable, from `pkg/internal/apis/config/validate.go:80-100`):**

1. `kubeProxyMode: ipvs is not supported with Kubernetes 1.36+`
2. `kube-proxy IPVS mode was deprecated in v1.35`
3. `Switch to iptables or nftables`
4. `https://kubernetes.io/docs/reference/networking/virtual-ips/`

51-UAT.md test 2 confirms all 4 are present in stderr verbatim.

**K8s 1.36 guide page (51-03 deliverable):**
- `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` — 168 lines; contains `hostUsers: false` (3 occurrences) AND `resizePolicy` (2 occurrences); registered in sidebar at `kinder-site/astro.config.mjs:83`
- Test 03 must boot `npm run dev` (which is `astro dev`) and assert `curl http://localhost:4321/guides/k8s-1-36-whats-new/` returns HTTP 200 with body containing the strings `User Namespaces` AND `In-Place Pod Resize`

**Astro dev-server port:** 4321 (Starlight default; confirmed by `kinder-site/package.json` engines + 58-RESEARCH.md §"Pattern 5"). Pitfall C (RESEARCH) warns about zombie processes; mitigated via preamble lsof check + EXIT/INT/TERM trap.

**51-UAT.md augment shape (Option A — locked planner decision (a)):**

Current 51-UAT.md frontmatter:
```yaml
---
status: complete       # ← stays `complete`
phase: 51-upstream-sync-k8s-1-36
source:
  - 51-01-SUMMARY.md
  - 51-02-SUMMARY.md
  - 51-03-SUMMARY.md
started: 2026-05-07T15:00:00Z
updated: 2026-05-07T15:20:00Z   # ← refreshed to current UTC
---
```

After (Option A — augment, do not replace):
```yaml
---
status: complete
phase: 51-upstream-sync-k8s-1-36
source:
  - 51-01-SUMMARY.md
  - 51-02-SUMMARY.md
  - 51-03-SUMMARY.md
  - 58-02 (Phase 58 live UAT re-verification)
started: 2026-05-07T15:00:00Z
updated: <current UTC ISO-8601>
---
```

The existing `## Tests`, `## Summary`, `## Gaps`, `## Notes` sections are PRESERVED INTACT. A new section is APPENDED at the bottom of the file:

```markdown
## Re-verification against v2.4 binary (Phase 58)

Date: <current UTC>
Binary: ./bin/kinder built from HEAD <40-char-commit-hash-from-log>
Script: hack/uat-51-envoy-ipvs-guide.sh
Log: hack/uat-51-envoy-ipvs-guide.log

### 1. HA cluster uses Envoy as load balancer (no HAProxy) — re-verified against v2.4
result: pass
evidence: |
  $ ./bin/kinder create cluster --name uat-58-02 (2 CP + 1 worker)
  $ docker ps --filter label=io.x-k8s.kind.cluster=uat-58-02 \
      --filter name=external-load-balancer --format '{{.Image}}'
  docker.io/envoyproxy/envoy:v1.36.2
  $ docker ps -a --format '{{.Image}}' | grep -c kindest/haproxy
  0

### 2. IPVS + K8s 1.36 config rejected at validation — re-verified against v2.4
result: pass
evidence: |
  $ ./bin/kinder create cluster --config /tmp/ipvs-1-36-test.yaml --name should-not-exist
  Exit 1. Stderr contains all 4 required substrings:
    - "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+"
    - "kube-proxy IPVS mode was deprecated in v1.35"
    - "Switch to iptables or nftables"
    - "https://kubernetes.io/docs/reference/networking/virtual-ips/"
  $ docker ps -a --filter name=should-not-exist
  (zero containers — validation rejected before any image pull)

### 3. K8s 1.36 website guide renders with both GA demos — re-verified against v2.4
result: pass
evidence: |
  $ cd kinder-site && npm run dev &
  $ curl -sf http://localhost:4321/guides/k8s-1-36-whats-new/ | head -50
  <captured response excerpt containing both `User Namespaces` and `In-Place Pod Resize` headings>
  HTTP 200; sidebar entry between Multi-Version Clusters and Working Offline confirmed.
```

(The exact bytes for evidence blocks come from the log; the script's tee output is the authoritative source.)

**Re-verification against v2.4 binary — what's actually being attested:**

The May 7 51-UAT.md tests passed against a binary that PRE-DATES phases 52-57. Phase 58's Option A augment is the v2.4-binary attestation. The three sub-blocks are the same three tests, run again against the rebuilt binary; the evidence proves that 51-01/51-02/51-03 deliverables still hold AFTER LIFE-09 (Phase 52), addon bumps (Phase 53), macOS signing (Phase 54), Windows CI (Phase 55), DEBT-04 (Phase 56), and DIAG-05/DIAG-06 (Phase 57) all landed. No code was expected to regress these properties, and the re-verification confirms it.

**REQUIREMENTS.md edits (gated on BOTH 58-01 and 58-02 passing):**

UAT-01 row in `### User Acceptance` section:
- `[ ] **UAT-01**: ...` -> `[x] **UAT-01**: ...`

UAT-02 row:
- `[ ] **UAT-02**: ...` -> `[x] **UAT-02**: ...`

Traceability table:
- `| UAT-01 | Phase 58 | Pending |` -> `| UAT-01 | Phase 58 | Complete |`
- `| UAT-02 | Phase 58 | Pending |` -> `| UAT-02 | Phase 58 | Complete |`

Bottom `Last updated:` line: append `; UAT-01 + UAT-02 marked complete after Phase 58 live UAT closure — hack/uat-47-ha-smoke.log + hack/uat-51-envoy-ipvs-guide.log committed as evidence`.

**ROADMAP.md edits:**

Under `### v2.4 Hardening (In Progress)`:
- `- [ ] **Phase 58: Live UAT Closure for Phase 47 + 51** - Run and record live smoke tests against rebuilt v2.4 binary for both deferred UAT items` -> `- [x] **Phase 58: Live UAT Closure for Phase 47 + 51** - ...` with a completion-narrative pre-line mirroring Phases 52-57 (e.g. `(completed <date>; hack/uat-47-ha-smoke.sh + hack/uat-51-envoy-ipvs-guide.sh; 47-UAT.md 14/14 pass; 51-UAT.md augmented with v2.4 re-verification; SC1-SC4 all green)`)

Progress table row:
- `| 58. Live UAT Closure for Phase 47 + 51 | v2.4 | 0/TBD | Not started | - |` -> `| 58. Live UAT Closure for Phase 47 + 51 | v2.4 | 2/2 | Complete | <date> |`

Phase 58 detail block under `### Phase 58: Live UAT Closure for Phase 47 + 51`:
- `**Plans**: TBD (2 plans: 58-01 Phase 47 HA smoke; 58-02 Phase 51 Envoy LB + IPVS + guide)` -> `**Plans**: 2 plans` plus the standard `Plans:` list with `- [x] 58-01-ha-smoke-PLAN.md — ...` and `- [x] 58-02-envoy-ipvs-guide-PLAN.md — ...`

**STATE.md edits:**

- Frontmatter: `stopped_at`, `last_updated`, `last_activity` refreshed; `progress.completed_phases` 6 -> 7; `progress.percent` (recompute against `total_phases: 7`)
- `## Current Position` block: update phase to `Phase 58 of 58 — CLOSED`
- `## Performance Metrics` -> `**By Phase:**` table: append two rows for 58-01 and 58-02 (Tasks N × ~M min)
- `## Accumulated Context` -> `### Decisions`: append a 2026-05-12+ decision entry summarizing the 5 planner-decisions outcomes (a, b, c, d, e)
- `## Session Continuity` -> `Last session:` and `Stopped at:` refreshed
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Author hack/uat-51-envoy-ipvs-guide.sh + syntax-validate</name>
  <files>hack/uat-51-envoy-ipvs-guide.sh</files>
  <action>
Create `hack/uat-51-envoy-ipvs-guide.sh` (new file). Mark executable. Implement the following structure verbatim — function names, marker strings, and the 4-substring IPVS assertion bytes are load-bearing:

```bash
#!/usr/bin/env bash
# Source: hack/uat-51-envoy-ipvs-guide.sh — Phase 58 Plan 02
# Closes: REQUIREMENTS.md UAT-02 + re-verifies 51-UAT.md tests 1/2/3 against the v2.4 binary
# Pitfall 23 gate: rebuilds ./bin/kinder against current HEAD on every invocation.
# Hard contract: ALWAYS uses absolute ${REPO_ROOT}/bin/kinder; NEVER bare `kinder` from $PATH.

set -euo pipefail

CLUSTER="uat-58-02"
LOG="${LOG:-hack/uat-51-envoy-ipvs-guide.log}"
TEARDOWN="${TEARDOWN:-no}"
REUSE="${REUSE:-prompt}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
KINDER_BIN="${REPO_ROOT}/bin/kinder"

DEV_PID=""
TMP_IPVS_CONFIG=""

cleanup() {
  if [[ -n "${DEV_PID}" ]] && kill -0 "${DEV_PID}" 2>/dev/null; then
    kill -TERM "${DEV_PID}" 2>/dev/null || true
    wait "${DEV_PID}" 2>/dev/null || true
  fi
  if [[ -n "${TMP_IPVS_CONFIG}" && -f "${TMP_IPVS_CONFIG}" ]]; then
    rm -f "${TMP_IPVS_CONFIG}"
  fi
}
trap cleanup EXIT INT TERM

# ---------- preamble ----------

preamble() {
  cd "${REPO_ROOT}"
  echo "=== Phase 58 Plan 02 — Phase 51 Envoy LB + IPVS guard + 1.36 guide live UAT ==="
  echo "ETA ~7 min; cluster ${CLUSTER}; log ${LOG}"
  echo "HEAD:  $(git rev-parse HEAD)"
  echo "Date:  $(date -u +%Y-%m-%dT%H:%M:%SZ)"

  # 1) Rebuild (idempotent)
  echo "--- make build ---"
  make build

  # 2) POSITIVE marker gate (same set as 58-01)
  local positive=(
    "crictl ps --name etcd -q"
    "all control-plane containers stopped"
    "docker.io/envoyproxy/envoy:v1.36.2"
    "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+"
    "quorum at risk"
  )
  for m in "${positive[@]}"; do
    if ! strings "${KINDER_BIN}" | grep -qF "${m}"; then
      echo "[FAIL] STALE BINARY — required v2.4 marker absent: ${m}" >&2
      exit 1
    fi
  done

  # 3) NEGATIVE marker gate
  local negative=(
    "label=io.x-k8s.kind.cluster=kind"
    "/usr/local/bin/etcdctl"
    "kindest/haproxy"
  )
  for m in "${negative[@]}"; do
    if strings "${KINDER_BIN}" | grep -qF "${m}"; then
      echo "[FAIL] STALE BINARY — forbidden pre-v2.4 marker present: ${m}" >&2
      exit 1
    fi
  done

  # 4) Astro dev-server port preflight (Pitfall C)
  if command -v lsof >/dev/null 2>&1 && lsof -i :4321 >/dev/null 2>&1; then
    echo "[FAIL] Port 4321 is already bound — astro dev server cannot start. Kill the existing process and re-run." >&2
    lsof -i :4321 >&2 || true
    exit 1
  fi

  # 5) Document version + path + Docker capacity
  "${KINDER_BIN}" version
  echo "Using: ${KINDER_BIN}"
  echo "Build: $(stat -f '%Sm' "${KINDER_BIN}" 2>/dev/null || stat -c '%y' "${KINDER_BIN}")"
  echo "--- docker info (RAM headroom) ---"
  docker info 2>/dev/null | grep -E 'Total Memory|CPUs' || true

  # 6) Re-entrancy
  if "${KINDER_BIN}" get clusters --output json 2>/dev/null | grep -q "\"name\":\"${CLUSTER}\""; then
    if [[ "${REUSE}" == "no" ]]; then
      "${KINDER_BIN}" delete cluster --name "${CLUSTER}"
    else
      echo "[FAIL] Existing cluster ${CLUSTER} detected. Run:" >&2
      echo "  ${KINDER_BIN} delete cluster --name ${CLUSTER}" >&2
      exit 1
    fi
  fi
}

# ---------- tests ----------

test_01_envoy_lb_on_ha_cluster() {
  echo "--- test_01: HA cluster LB is Envoy (no HAProxy) ---"
  "${KINDER_BIN}" create cluster --name "${CLUSTER}" --config - <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: control-plane
  - role: worker
EOF
  kubectl --context "kind-${CLUSTER}" wait --for=condition=Ready node --all --timeout=180s

  local lb_image
  lb_image="$(docker ps \
    --filter "label=io.x-k8s.kind.cluster=${CLUSTER}" \
    --filter "name=external-load-balancer" \
    --format '{{.Image}}')"
  if [[ "${lb_image}" != "docker.io/envoyproxy/envoy:v1.36.2" ]]; then
    echo "[FAIL] test_01 — LB image = '${lb_image}', want 'docker.io/envoyproxy/envoy:v1.36.2'"
    return 1
  fi
  if docker ps -a --format '{{.Image}}' | grep -q 'kindest/haproxy'; then
    echo "[FAIL] test_01 — kindest/haproxy container present (must be absent post-51-01)"
    docker ps -a --format '{{.Image}}' | grep 'kindest/haproxy'
    return 1
  fi
  echo "[OK] test_01 — Envoy LB (${lb_image}) present; HAProxy absent"
}

test_02_ipvs_1_36_rejected_at_validate() {
  echo "--- test_02: kubeProxyMode:ipvs + 1.36 image rejected at validate ---"
  TMP_IPVS_CONFIG="$(mktemp -t ipvs-1-36-test.XXXXXX.yaml)"
  cat > "${TMP_IPVS_CONFIG}" <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  kubeProxyMode: ipvs
nodes:
  - role: control-plane
    image: kindest/node:v1.36.0
EOF

  local stderr_capture exit_code
  set +e
  stderr_capture="$("${KINDER_BIN}" create cluster --config "${TMP_IPVS_CONFIG}" --name should-not-exist 2>&1 >/dev/null)"
  exit_code=$?
  set -e

  if [[ ${exit_code} -eq 0 ]]; then
    echo "[FAIL] test_02 — kinder did NOT reject ipvs+1.36 config (exit 0)"
    return 1
  fi

  local required=(
    "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+"
    "kube-proxy IPVS mode was deprecated in v1.35"
    "Switch to iptables or nftables"
    "https://kubernetes.io/docs/reference/networking/virtual-ips/"
  )
  for s in "${required[@]}"; do
    if ! grep -qF "${s}" <<< "${stderr_capture}"; then
      echo "[FAIL] test_02 — stderr missing required substring: ${s}"
      echo "Captured stderr:"
      echo "${stderr_capture}"
      return 1
    fi
  done

  if docker ps -a --filter "name=should-not-exist" --format '{{.Names}}' | grep -q .; then
    echo "[FAIL] test_02 — kinder created a container despite validation rejection"
    return 1
  fi

  echo "[OK] test_02 — ipvs+1.36 rejected (exit ${exit_code}); all 4 required substrings present; no container created"
}

test_03_k8s_1_36_guide_renders() {
  echo "--- test_03: kinder-site k8s-1-36-whats-new guide page renders ---"
  if ! command -v npm >/dev/null 2>&1; then
    echo "[SKIP] test_03 — npm not installed; rendering deferred to a machine with kinder-site toolchain"
    return 0
  fi

  pushd "${REPO_ROOT}/kinder-site" >/dev/null
  if [[ ! -d node_modules ]]; then
    echo "Installing kinder-site dependencies..."
    npm install
  fi

  # Start astro dev in background; capture PID for cleanup trap.
  npm run dev >/tmp/uat-58-02-astro.log 2>&1 &
  DEV_PID=$!

  # Poll for HTTP 200
  local i success=0
  for i in {1..30}; do
    if curl -sf -o /dev/null --max-time 2 http://localhost:4321/guides/k8s-1-36-whats-new/; then
      success=1
      break
    fi
    sleep 2
  done
  if [[ ${success} -eq 0 ]]; then
    echo "[FAIL] test_03 — astro dev server did not serve /guides/k8s-1-36-whats-new/ within 60s"
    cat /tmp/uat-58-02-astro.log || true
    popd >/dev/null
    return 1
  fi

  local body
  body="$(curl -sf http://localhost:4321/guides/k8s-1-36-whats-new/)"
  popd >/dev/null

  if ! grep -qF "User Namespaces" <<< "${body}"; then
    echo "[FAIL] test_03 — response body missing 'User Namespaces'"
    return 1
  fi
  if ! grep -qF "In-Place Pod Resize" <<< "${body}"; then
    echo "[FAIL] test_03 — response body missing 'In-Place Pod Resize'"
    return 1
  fi

  echo "[OK] test_03 — guide page renders; both GA-feature headings present; HTTP 200"
  # Kill dev server proactively (trap will also handle it).
  kill -TERM "${DEV_PID}" 2>/dev/null || true
  wait "${DEV_PID}" 2>/dev/null || true
  DEV_PID=""
}

finalize() {
  echo "=== ALL TESTS PASSED ==="
  if [[ "${TEARDOWN}" == "yes" ]] || [[ "${1:-}" == "--teardown" ]]; then
    "${KINDER_BIN}" delete cluster --name "${CLUSTER}"
    echo "Cluster ${CLUSTER} torn down."
  else
    echo "Cluster ${CLUSTER} left up. Clean up with:"
    echo "  ${KINDER_BIN} delete cluster --name ${CLUSTER}"
  fi
}

trap '[[ $? -ne 0 ]] && echo "[FAIL] Cluster ${CLUSTER} may be left up for inspection. Run: ${KINDER_BIN} delete cluster --name ${CLUSTER}" >&2' ERR

main() {
  preamble
  test_01_envoy_lb_on_ha_cluster
  test_02_ipvs_1_36_rejected_at_validate
  test_03_k8s_1_36_guide_renders
  finalize "$@"
}

main "$@" 2>&1 | tee "${LOG}"
```

After writing, run `chmod +x hack/uat-51-envoy-ipvs-guide.sh` and `bash -n hack/uat-51-envoy-ipvs-guide.sh`. Run `shellcheck` if available (informational only).

Do NOT execute the script in this task. Live work is gated on the Task 2 checkpoint.

Commit message (atomic): `feat(58-02): add hack/uat-51-envoy-ipvs-guide.sh` — files: `hack/uat-51-envoy-ipvs-guide.sh`.
  </action>
  <verify>
    <automated>
cd /Users/patrykattc/work/git/kinder && \
  test -x hack/uat-51-envoy-ipvs-guide.sh && \
  bash -n hack/uat-51-envoy-ipvs-guide.sh && \
  grep -qE '^set -euo pipefail' hack/uat-51-envoy-ipvs-guide.sh && \
  grep -qE '\$\{REPO_ROOT\}/bin/kinder' hack/uat-51-envoy-ipvs-guide.sh && \
  ! grep -qE '^[[:space:]]*kinder[[:space:]]' hack/uat-51-envoy-ipvs-guide.sh && \
  grep -qF 'docker.io/envoyproxy/envoy:v1.36.2' hack/uat-51-envoy-ipvs-guide.sh && \
  grep -qF 'kubeProxyMode: ipvs is not supported with Kubernetes 1.36+' hack/uat-51-envoy-ipvs-guide.sh && \
  grep -qF 'kube-proxy IPVS mode was deprecated in v1.35' hack/uat-51-envoy-ipvs-guide.sh && \
  grep -qF 'Switch to iptables or nftables' hack/uat-51-envoy-ipvs-guide.sh && \
  grep -qF 'https://kubernetes.io/docs/reference/networking/virtual-ips/' hack/uat-51-envoy-ipvs-guide.sh && \
  grep -qE 'test_01_envoy_lb_on_ha_cluster|test_02_ipvs_1_36_rejected_at_validate|test_03_k8s_1_36_guide_renders' hack/uat-51-envoy-ipvs-guide.sh && \
  grep -qF 'kill -TERM' hack/uat-51-envoy-ipvs-guide.sh
    </automated>
  </verify>
  <done>
    File `hack/uat-51-envoy-ipvs-guide.sh` exists, is executable, parses cleanly, contains all 4 required IPVS substrings, references `${REPO_ROOT}/bin/kinder` (never bare `kinder`), defines the 3 test functions, and includes the EXIT-trap kill -TERM dev-server cleanup.
  </done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 2: Developer runs the script on their machine; reports pass/fail</name>
  <what-built>
    Task 1 wrote `hack/uat-51-envoy-ipvs-guide.sh` — a self-contained bash script that rebuilds `./bin/kinder`, runs the strings-marker gate, creates a 2 CP + 1 worker HA cluster (`uat-58-02`), asserts Envoy LB / no HAProxy (test 01), creates a /tmp YAML with `kubeProxyMode: ipvs` + 1.36 image and asserts validate rejection with all 4 required substrings (test 02), boots `astro dev` and asserts the K8s 1.36 guide renders with both GA-feature headings (test 03), and `tee`'s everything to `hack/uat-51-envoy-ipvs-guide.log`.

    Live cluster + astro work cannot run inside Claude. This checkpoint blocks Task 3 until evidence of one successful run is in hand.
  </what-built>
  <how-to-verify>
    On the developer's macOS host with Docker Desktop running, Node/npm available, and port 4321 free:

    1. `cd /Users/patrykattc/work/git/kinder`
    2. Confirm Plan 58-01's cluster has been torn down (or use a name that doesn't collide):
       ```
       ./bin/kinder delete cluster --name uat-58-01 || true
       ```
       Reason: planner decision (e) — Docker VM RAM contention.
    3. Run the script:
       ```
       bash hack/uat-51-envoy-ipvs-guide.sh
       ```
       Expected wall-clock: ~7 min. Expected exit code: 0.
    4. Inspect `hack/uat-51-envoy-ipvs-guide.log` — should contain:
       - `kind v1.5.0 go1.26.3 ...` from `./bin/kinder version`
       - `HEAD:  <40-char hash>`
       - `[OK] test_01 — Envoy LB (docker.io/envoyproxy/envoy:v1.36.2) present; HAProxy absent`
       - `[OK] test_02 — ipvs+1.36 rejected (exit <N>); all 4 required substrings present; no container created`
       - `[OK] test_03 — guide page renders; both GA-feature headings present; HTTP 200`
         (or `[SKIP] test_03 — npm not installed` if running on a machine without Node — acceptable per planner spec)
       - `=== ALL TESTS PASSED ===`

    Reply with:
    - `pass` + paste the final 3 `[OK]` lines and the `=== ALL TESTS PASSED ===` line
    - OR `fail` + paste the `[FAIL] test_<N>` line + ~10 lines of surrounding context

    Common failure modes:
    - `[FAIL] test_01 — LB image = 'kindest/haproxy:...'` -> 51-01 regression somewhere; check `pkg/cluster/internal/loadbalancer/const.go`.
    - `[FAIL] test_02 — stderr missing required substring: <X>` -> the IPVS guard error message wording changed; either the validate.go wording was edited (regression) or one of the 4 strings has a typo in this script.
    - `[FAIL] test_03 — astro dev server did not serve ...` -> usually port 4321 conflict or npm install failure; the cleanup trap handles dev-server kill, but the log will show the astro startup error.
    - `[FAIL] STALE BINARY` -> dirty `bin/`; try `rm -rf bin/ && bash hack/uat-51-envoy-ipvs-guide.sh`.
  </how-to-verify>
  <resume-signal>Reply `pass` + log excerpt OR `fail` + log excerpt</resume-signal>
</task>

<task type="auto">
  <name>Task 3: Commit log + augment 51-UAT.md + close Phase 58 (REQUIREMENTS + ROADMAP + STATE)</name>
  <files>
    hack/uat-51-envoy-ipvs-guide.log
    .planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md
    .planning/REQUIREMENTS.md
    .planning/ROADMAP.md
    .planning/STATE.md
  </files>
  <action>
**Step A — commit the log.** After Task 2 returns `pass`, `git add hack/uat-51-envoy-ipvs-guide.log`. Verbatim run transcript; do NOT edit. Commit (atomic): `chore(58-02): commit live UAT log evidence` — files: `hack/uat-51-envoy-ipvs-guide.log`.

**Step B — augment 51-UAT.md (Option A).** Edit `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md`:

1. Frontmatter:
   - `updated: 2026-05-07T15:20:00Z` -> `updated: <current UTC ISO-8601>`
   - `status: complete` stays as-is
   - Append `- 58-02 (Phase 58 live UAT re-verification)` to the `source:` list

2. Preserve all existing sections (`## Current Test`, `## Tests`, `## Summary`, `## Gaps`, `## Notes`) byte-for-byte. Do NOT edit any existing test row or evidence block.

3. Append a new section AT THE END of the file (after the existing `## Notes` block):

```markdown

## Re-verification against v2.4 binary (Phase 58)

Date: <current UTC ISO-8601>
Binary: ./bin/kinder built from HEAD <40-char-commit-hash-extracted-from-log>
Script: hack/uat-51-envoy-ipvs-guide.sh
Log: hack/uat-51-envoy-ipvs-guide.log

This re-verification confirms the Phase 51 success criteria still hold against the final v2.4 binary (after Phases 52-57 landed: LIFE-09 IP-pinning, addon bumps, macOS ad-hoc signing, Windows PR-CI, DEBT-04 race fix, DIAG-05+DIAG-06 doctor cosmetics). No code was expected to regress these properties; the script's strings-marker gate (`make build` + 5 POSITIVE + 3 NEGATIVE markers) attests the binary inspected here is the v2.4 build.

### 1. HA cluster uses Envoy as load balancer (no HAProxy) — re-verified
result: pass
evidence: |
  $ ./bin/kinder create cluster --name uat-58-02 --config - (2 CP + 1 worker)
  $ docker ps --filter label=io.x-k8s.kind.cluster=uat-58-02 \
      --filter name=external-load-balancer --format '{{.Image}}'
  docker.io/envoyproxy/envoy:v1.36.2
  $ docker ps -a --format '{{.Image}}' | grep -c kindest/haproxy
  0
note: |
  Live UAT against rebuilt ./bin/kinder. Phase 51-01 deliverable (envoyproxy/envoy:v1.36.2) confirmed unchanged at v2.4 close.

### 2. IPVS + K8s 1.36 config rejected at validation — re-verified
result: pass
evidence: |
  $ ./bin/kinder create cluster --config <tmpfile> --name should-not-exist
  Exit non-zero (validation rejection before any container created).
  All 4 required substrings present in stderr:
    - "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+"
    - "kube-proxy IPVS mode was deprecated in v1.35"
    - "Switch to iptables or nftables"
    - "https://kubernetes.io/docs/reference/networking/virtual-ips/"
  $ docker ps -a --filter name=should-not-exist
  (zero rows — no container leaked through)
note: |
  Live UAT against rebuilt ./bin/kinder. Phase 51-02 deliverable (validate.go:80-100 guard + 4-substring error message + migration URL) confirmed unchanged at v2.4 close.

### 3. K8s 1.36 website guide renders with both GA demos — re-verified
result: pass
evidence: |
  $ cd kinder-site && npm run dev  (background)
  $ curl -sf http://localhost:4321/guides/k8s-1-36-whats-new/
  HTTP 200. Body contains both "User Namespaces" and "In-Place Pod Resize" substrings.
  Sidebar entry (registered at astro.config.mjs:83) between Multi-Version Clusters and Working Offline confirmed.
note: |
  Live UAT against rebuilt ./bin/kinder. Phase 51-03 deliverable (guides/k8s-1-36-whats-new.md + sidebar registration) confirmed unchanged at v2.4 close.
```

(Use the actual commit hash + UTC stamp extracted from `hack/uat-51-envoy-ipvs-guide.log`. The `evidence:` blocks summarize what the log shows; the log itself is the verbatim source.)

Commit (atomic): `docs(58-02): augment 51-UAT.md with v2.4 re-verification evidence` — files: `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md`.

**Step C — phase-close edits (REQUIREMENTS.md + ROADMAP.md + STATE.md).** These three files transition Phase 58 from `Not started`/`Pending` to `Complete` in one commit (terminal-plan responsibility since 58-02 is the last plan in the phase):

**`.planning/REQUIREMENTS.md`:**

1. `### User Acceptance` section:
   - `- [ ] **UAT-01**:` -> `- [x] **UAT-01**:`
   - `- [ ] **UAT-02**:` -> `- [x] **UAT-02**:`

2. `## Traceability` table:
   - `| UAT-01 | Phase 58 | Pending |` -> `| UAT-01 | Phase 58 | Complete |`
   - `| UAT-02 | Phase 58 | Pending |` -> `| UAT-02 | Phase 58 | Complete |`

3. Bottom-of-file `*Last updated:* ...` line: append `; UAT-01 + UAT-02 marked complete after Phase 58 live UAT closure — hack/uat-47-ha-smoke.log + hack/uat-51-envoy-ipvs-guide.log committed as evidence; 47-UAT.md flipped 5 issue rows to pass (passed: 14/14); 51-UAT.md augmented with v2.4 re-verification section (Option A)`.

**`.planning/ROADMAP.md`:**

1. Under `### v2.4 Hardening (In Progress)` — Phase 58 line:
   - `- [ ] **Phase 58: Live UAT Closure for Phase 47 + 51** - Run and record live smoke tests against rebuilt v2.4 binary for both deferred UAT items`
   - ->
   - `- [x] **Phase 58: Live UAT Closure for Phase 47 + 51** - Run and record live smoke tests against rebuilt v2.4 binary for both deferred UAT items (completed <YYYY-MM-DD>; hack/uat-47-ha-smoke.sh + hack/uat-51-envoy-ipvs-guide.sh authored; live runs captured to hack/uat-47-ha-smoke.log + hack/uat-51-envoy-ipvs-guide.log; 47-UAT.md tests 3/9/12/13/14 flipped to pass (passed: 14/14); 51-UAT.md augmented with v2.4 re-verification section (Option A); make build + 5-POSITIVE + 3-NEGATIVE strings-marker gate honored Pitfall 23; SC1-SC4 all green)`

2. `## Phase Details` -> `### Phase 58: Live UAT Closure for Phase 47 + 51`:
   - `**Plans**: TBD (2 plans: 58-01 Phase 47 HA smoke; 58-02 Phase 51 Envoy LB + IPVS + guide)`
   - ->
   - `**Plans**: 2 plans`
   - Then a `Plans:` block:
     ```
     Plans:
     - [x] 58-01-ha-smoke-PLAN.md — Phase 47 HA pause/resume live UAT against rebuilt v2.4 binary; flips 47-UAT.md tests 3/9/12/13/14 from issue to pass
     - [x] 58-02-envoy-ipvs-guide-PLAN.md — Phase 51 Envoy LB + IPVS-1.36 reject + K8s 1.36 guide re-verification against rebuilt v2.4 binary; augments 51-UAT.md with v2.4 evidence section
     ```

3. `## Progress` table — Phase 58 row:
   - `| 58. Live UAT Closure for Phase 47 + 51 | v2.4 | 0/TBD | Not started | - |`
   - ->
   - `| 58. Live UAT Closure for Phase 47 + 51 | v2.4 | 2/2 | Complete | <YYYY-MM-DD> |`

**`.planning/STATE.md`:**

1. Frontmatter:
   - `stopped_at:` -> a short description of Phase 58 close (e.g. `"Phase 58 CLOSED — UAT-01 + UAT-02 closed via live UAT against rebuilt v2.4 binary; hack/uat-47-ha-smoke.sh + hack/uat-51-envoy-ipvs-guide.sh + matching .log files committed; 47-UAT.md tests 3/9/12/13/14 flipped to pass; 51-UAT.md augmented with v2.4 re-verification section (Option A — planner decision (a)); REQUIREMENTS.md UAT-01 + UAT-02 marked Complete; ROADMAP Phase 58 marked Complete (2/2); v2.4 Hardening milestone is feature-complete pending milestone audit + ship."`)
   - `last_updated:` and `last_activity:` -> current UTC ISO-8601 stamp
   - `progress.completed_phases` -> 7
   - `progress.percent` -> 100 (already 100 — STATE.md is currently 100% for 6/7, recompute against `total_phases: 7`)

2. `## Current Position`:
   - Phase: `Phase 58 of 58 — CLOSED`
   - Plan: short text noting both 58-01 + 58-02 done

3. `## Performance Metrics` -> `**By Phase:**` table: append two rows:
   - `| 58-01 | 3 tasks (script + checkpoint + log+UAT.md flip) | ~<actual> |`
   - `| 58-02 | 3 tasks (script + checkpoint + log+UAT.md augment + REQ/ROADMAP/STATE close) | ~<actual> |`

4. `## Accumulated Context` -> `### Decisions`: append a 2026-05-12+ entry summarizing the 5 planner-decisions outcomes recorded in this plan's frontmatter (a, b, c, d, e). One paragraph, in the style of prior entries.

5. `## Session Continuity` -> refresh `Last session:` and `Stopped at:`.

Commit (atomic): `docs(phase-58): mark complete — UAT-01 + UAT-02 closed; REQUIREMENTS+ROADMAP+STATE rolled` — files: `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, `.planning/STATE.md`.

Do NOT write 58-01-SUMMARY.md or 58-02-SUMMARY.md in this task — those are owned by the verifier / `/gsd:verify-work` step. The plan body's must_haves frontmatter and this <output> block document what landed.
  </action>
  <verify>
    <automated>
cd /Users/patrykattc/work/git/kinder && \
  test -f hack/uat-51-envoy-ipvs-guide.log && \
  git ls-files --error-unmatch hack/uat-51-envoy-ipvs-guide.log && \
  grep -qE 'Re-verification against v2.4 binary' .planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md && \
  grep -qE '^status: complete' .planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md && \
  grep -qE '\- \[x\] \*\*UAT-01\*\*' .planning/REQUIREMENTS.md && \
  grep -qE '\- \[x\] \*\*UAT-02\*\*' .planning/REQUIREMENTS.md && \
  grep -qE 'UAT-01 \| Phase 58 \| Complete' .planning/REQUIREMENTS.md && \
  grep -qE 'UAT-02 \| Phase 58 \| Complete' .planning/REQUIREMENTS.md && \
  grep -qE '\- \[x\] \*\*Phase 58:' .planning/ROADMAP.md && \
  grep -qE '58\. Live UAT Closure for Phase 47 \+ 51 \| v2\.4 \| 2/2 \| Complete' .planning/ROADMAP.md && \
  grep -qE 'completed_phases: 7' .planning/STATE.md
    </automated>
  </verify>
  <done>
    `hack/uat-51-envoy-ipvs-guide.log` is tracked. `51-UAT.md` contains the new `## Re-verification against v2.4 binary (Phase 58)` section (status still `complete`). REQUIREMENTS.md UAT-01 + UAT-02 are `[x]` and Traceability rows are `Complete`. ROADMAP Phase 58 checkbox is `[x]`; Progress table row reads `2/2 Complete`. STATE.md `completed_phases: 7`. v2.4 Hardening milestone is feature-complete in the planning state.
  </done>
</task>

</tasks>

<verification>
Phase 58 SC3 + SC4 + phase-close invariants must hold after Task 3 completes.

**SC3 — 51-UAT.md augmented (Option A; planner decision (a)):**
```bash
grep -A2 -E '^## Re-verification against v2.4 binary' .planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md
# Expected: section present with the dated header and Binary/Script/Log lines.
grep -cE '^### [123]\. ' .planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md
# Expected: at least 3 (original) + 3 (re-verification) = 6 section-3-level headings, or whatever count preserves the existing 3 plus adds 3 new ones; the key invariant is the existing 3 are untouched.
grep -cE '^result: pass' .planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md
# Expected: >= 6 (3 original pass + 3 new pass).
```

**SC4 — script uses absolute path:**
```bash
grep -nE '^[[:space:]]*kinder[[:space:]]+(get|pause|resume|create|delete|doctor|version)' hack/uat-51-envoy-ipvs-guide.sh
# Expected: zero matches.
```

**Strings-marker gate (researcher Pattern 1):**
```bash
cd /Users/patrykattc/work/git/kinder && make build && \
  strings ./bin/kinder | grep -qF 'docker.io/envoyproxy/envoy:v1.36.2' && \
  strings ./bin/kinder | grep -qF 'kubeProxyMode: ipvs is not supported with Kubernetes 1.36+' && \
  ! strings ./bin/kinder | grep -qF 'kindest/haproxy' && \
  echo OK
```

**Phase-close invariants (terminal-plan responsibility):**
```bash
# REQUIREMENTS: both UAT rows complete
grep -E '^\- \[x\] \*\*UAT-0[12]\*\*' .planning/REQUIREMENTS.md
# Expected: 2 lines.

# ROADMAP: Phase 58 marked complete in Progress table
grep -E '58\. Live UAT Closure for Phase 47 \+ 51 \| v2\.4 \| 2/2 \| Complete' .planning/ROADMAP.md
# Expected: 1 line.

# STATE: completed_phases = 7
grep -E 'completed_phases: 7' .planning/STATE.md
# Expected: 1 line.

# Log evidence committed
git ls-files --error-unmatch hack/uat-47-ha-smoke.log hack/uat-51-envoy-ipvs-guide.log
# Expected: both files tracked.
```

**Script lint:**
```bash
bash -n hack/uat-51-envoy-ipvs-guide.sh
# Expected: exit 0.
```
</verification>

<success_criteria>
- [ ] **ROADMAP SC3 (Phase 51 UAT against v2.4 binary)**: `docker ps` evidence of `envoyproxy/envoy` (not `kindest/haproxy`) for `uat-58-02`; IPVS+1.36 config rejected at validate with all 4 required substrings in stderr and zero `should-not-exist` containers created; K8s 1.36 guide page renders with both `User Namespaces` and `In-Place Pod Resize` substrings in the response body (or `[SKIP]` graceful-degradation if npm absent — documented in script-header).
- [ ] **ROADMAP SC4 (./bin/kinder everywhere)**: zero bare `kinder` invocations in the script; every kinder call uses `${KINDER_BIN}` or `${REPO_ROOT}/bin/kinder`.
- [ ] **51-UAT.md augment (Option A, planner decision (a))**: new section `## Re-verification against v2.4 binary (Phase 58)` appended below the existing `## Notes` block; existing `## Tests` / `## Summary` / `## Gaps` / `## Notes` sections preserved byte-for-byte; frontmatter `status:` STAYS `complete`; `updated:` refreshed.
- [ ] **Evidence artifact committed**: `hack/uat-51-envoy-ipvs-guide.log` is tracked by git and contains the verbatim transcript with all `[OK] test_<N>` (or `[SKIP] test_03`) lines and the `=== ALL TESTS PASSED ===` footer.
- [ ] **REQUIREMENTS.md phase-close**: UAT-01 + UAT-02 checkboxes are `[x]`; Traceability rows show `Complete` for both; bottom `Last updated:` line is appended.
- [ ] **ROADMAP.md phase-close**: Phase 58 checkbox is `[x]` with completion narrative; Progress table row reads `2/2 Complete`; sub-plan list under Phase 58 detail block names both plans with `[x]`.
- [ ] **STATE.md phase-close**: `completed_phases: 7`; current position notes Phase 58 closed; Performance Metrics table gains 58-01 + 58-02 rows; Decisions section records the 5 planner-decisions outcomes.
- [ ] **No Go source / no manifest / no Makefile / no CI workflow changes**: this plan touches only `hack/` + `.planning/`.
- [ ] **Planner decisions recorded** in frontmatter `planner_decisions:` block — all 5 (a, b, c, d, e) documented with rationale.
</success_criteria>

<output>
Phase 58 Plan 02 produces three commits at execute time:
- `feat(58-02): add hack/uat-51-envoy-ipvs-guide.sh` (Task 1)
- `chore(58-02): commit live UAT log evidence` (Task 3 Step A)
- `docs(58-02): augment 51-UAT.md with v2.4 re-verification evidence` (Task 3 Step B)
- `docs(phase-58): mark complete — UAT-01 + UAT-02 closed; REQUIREMENTS+ROADMAP+STATE rolled` (Task 3 Step C — the terminal-plan phase-close commit)

After these commits land, v2.4 Hardening is feature-complete pending milestone audit. The verifier (`/gsd:verify-work` Phase 58) should grade against:
- Both `.sh` files executable
- Both `.log` files committed and non-empty
- 47-UAT.md zero `result: issue` rows
- 51-UAT.md has the new section + status stays `complete`
- REQUIREMENTS.md UAT-01 + UAT-02 = Complete
- ROADMAP Phase 58 = 2/2 Complete
- STATE.md = 7/7 completed_phases

SUMMARY.md files (58-01-SUMMARY.md, 58-02-SUMMARY.md) are owned by the close-out step, not by this plan.
</output>
