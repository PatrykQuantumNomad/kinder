# Phase 58: Live UAT Closure for Phase 47 + 51 — Research

**Researched:** 2026-05-12
**Domain:** Live (shell-driven) UAT scripting against a rebuilt Go CLI on a developer macOS host, against Docker-backed Kubernetes HA clusters. Documentation hygiene (47-UAT.md status-field flip, 51-UAT.md authoring against existing schema). Pitfall-23 stale-binary gate.
**Confidence:** HIGH — every claim below is grounded in a file or commit already on disk; no third-party libraries are introduced; no version-volatile knowledge is involved.

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| UAT-01 | Phase 47 live HA UAT closed — 3 CP + 2 worker + LB cluster smoke verifies pause/resume worker→CP→LB ordering, host CPU/RAM observation, cluster-state round-trip (pods/PVCs/services), and `cluster-resume-readiness` warn on quorum loss; rebuilt `./bin/kinder version` verified before run; evidence in `hack/uat-47-ha-smoke.sh` + log | §"Plan 58-01 — Deliverable Surface"; §"Existing State of 47-UAT.md"; §"Pitfall 23 (stale binary) — Concrete Mechanics" |
| UAT-02 | Phase 51 live UAT closed — `docker ps` confirms `envoyproxy/envoy` (not `kindest/haproxy`) on real HA cluster; `kinder create cluster --config <ipvs+1.36>` rejected at validate with migration URL in error; K8s 1.36 guide page renders with sidebar entry; rebuilt binary verified before run | §"Plan 58-02 — Deliverable Surface"; §"Existing State of 51-UAT.md"; §"Phase 51 evidence harvest" |

---

## Summary

Phase 58 is a **milestone-closure phase**, not a code phase. No Go source, no test files, no manifest edits. The deliverables are: (a) two bash scripts under `hack/uat-47-ha-smoke.sh` and `hack/uat-51-envoy-ipvs-guide.sh`; (b) edits to two existing UAT documents (`47-UAT.md` status flips from `issue`→`pass`, `51-UAT.md` may keep its existing 3-test pass record but must be supplemented with a v2.4-binary attestation block); (c) optional captured log files committed alongside the scripts as evidence. Pitfall 23 (stale binary) is the singular blast-radius risk: every script MUST hard-fail before the first live operation if `./bin/kinder` does not match the v2.4 HEAD build.

The hardest research finding — **`./bin/kinder version` does NOT print the git commit hash** in the current build configuration (`versionPreRelease == ""` in `pkg/internal/kindversion/version.go:60`). The current output is the literal string `kind v1.5.0 go1.26.3 darwin/arm64`. This means a naive "diff `bin/kinder version` against expected hash" check is impossible. The scripts must instead use **`strings $(./bin/kinder version | …)`** introspection (the technique 47-VERIFICATION.md already prescribes) OR **mtime-vs-HEAD-commit-time** OR **rebuild-on-every-invocation** as the freshness gate. Recommended: rebuild on every invocation via `make build` inside the script (idempotent on a clean tree, ~10 s on Apple Silicon) and then perform `strings` introspection for specific v2.4-only string markers as a belt-and-suspenders check.

**Primary recommendation:** Two single-task plans. Each plan is one bash script under `hack/` plus the matching UAT.md edit. Each script begins with an idempotent `make build` invocation, then the strings-marker gate, then the live cluster work, and finally a "tear down or leave for inspection" coda. Plans are sequential (58-01 must land before 58-02) only because they share a host machine — both scripts are conceptually independent.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Build attestation (`./bin/kinder` reflects v2.4 HEAD) | Build / dev-environment | UAT script preamble | Pitfall 23 is solved at build time, not at runtime — but the script must hard-fail if the build didn't happen or didn't include HEAD. |
| 3-CP HA cluster lifecycle (create/pause/resume/delete) | `pkg/internal/lifecycle/{pause,resume}.go` (already shipped) | Phase 58 UAT script (invokes only via `./bin/kinder`) | Phase 47 source code is complete and verified at 4/4 truths; UAT consumes it via CLI. |
| `cluster-resume-readiness` quorum-loss probe | `pkg/internal/doctor/resumereadiness.go` (already shipped) | Phase 58 UAT script (invokes `./bin/kinder doctor`) | Phase 57 closed this; UAT is the field-confirmation step. |
| Envoy LB image at runtime | `pkg/cluster/internal/loadbalancer/const.go` (`Image = "docker.io/envoyproxy/envoy:v1.36.2"`) | Phase 58 UAT script (greps `docker ps`) | Static evidence is `const.go`; live evidence is the running container. |
| IPVS+1.36 validate rejection | `pkg/internal/apis/config/validate.go:80-100` (already shipped) | Phase 58 UAT script (asserts non-zero exit + error substring) | Validate guard is unit-tested with 7 cases; UAT confirms CLI path. |
| K8s 1.36 guide page rendering | `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` + `kinder-site/astro.config.mjs:83` | Phase 58 UAT step (visual confirmation in `astro dev` server) | Astro is the renderer; UAT confirms the dev server boots and the page is reachable. |
| Status-field bookkeeping in `*-UAT.md` | UAT-script authors (this phase) | — | The doc is the single source of truth for milestone closure; flipping `issue`→`pass` is the atomic act that closes UAT-01/UAT-02. |

---

## Existing State of 47-UAT.md (the thing the planner must edit)

**Path:** `.planning/phases/47-cluster-pause-resume/47-UAT.md`

**Frontmatter:**
```yaml
---
status: diagnosed       # ← will flip to: closed (or "complete" — see Plan 58-01 decision point below)
phase: 47-cluster-pause-resume
source: [47-01-SUMMARY.md, 47-02-SUMMARY.md, 47-03-SUMMARY.md, 47-04-SUMMARY.md, 47-05-SUMMARY.md]
started: 2026-05-05T14:07:44Z
updated: 2026-05-05T14:55:00Z   # ← will update to the live-UAT date
---
```

**Body shape:** A `## Tests` section with 14 numbered tests, each with `expected:` and `result:` keys. Today:
- total: 14
- passed: 9
- issues: 5 (tests 3, 9, 12, 13, 14)
- pending: 0

**The 5 `issue` rows the planner must change to `pass`:**

| # | Test | Current `result:` | Closure mechanism | Evidence required |
|---|------|-------------------|-------------------|-------------------|
| 3 | `kinder get nodes <name>` positional arg | issue (cobra.NoArgs blocked positional) | Source fixed in 47-06 commit `50aa742a` (verified at 47-VERIFICATION.md line 60). Run `./bin/kinder get nodes <cluster>` against live cluster. | One-line shell capture: command exits 0, prints node table. |
| 9 | `kinder resume <name> --wait 5m` parses | issue (IntVar rejected "5m") | Source fixed in 47-06 commit `7a4f722f` (DurationVar migration). | One-line shell capture: `./bin/kinder resume <cluster> --wait 5m` parses (no `strconv.ParseInt` error); exits 0 after readiness wait. |
| 12 | `cluster-resume-readiness: ok, 3/3 etcd members healthy` on healthy HA | issue (stale binary; clusterskew =kind pin) | Source fixed in 47-06 commit `ed85ecdf` (clusterFilter presence-only) AND 57-02 commit `c43bb599` (tolerant etcd JSON parse). Stale binary closed by `make build` preamble. | Capture `./bin/kinder doctor` output; look for line containing `cluster-resume-readiness` and `3/3 etcd members healthy`. |
| 13 | `cluster-resume-readiness: warn` after stopping 2 of 3 CPs | issue (HA gate counted RUNNING CPs only) | Source fixed in 47-06 commit `ed85ecdf` (`-a` flag in cpNodeFilter + inspectState bootstrap selection). | Stop 2 CPs with `docker stop`; capture `./bin/kinder doctor` output; line contains `cluster-resume-readiness` + `warn` + `1/3` (or `quorum at risk`). |
| 14 | Non-empty `leaderID` in `/kind/pause-snapshot.json` | issue (stale binary; pause.go was already correct) | Pure stale-binary issue. Closed by rebuild. | `./bin/kinder pause <cluster>`; then `docker exec <cluster>-control-plane cat /kind/pause-snapshot.json` shows non-empty `leaderID` field. |

**The 1 row that is **also** open implicitly** (host CPU/RAM observation): test 4 is currently `pass` in 47-UAT.md and test 7 is `pass` — but 47-VERIFICATION.md classifies SC1/SC2 as `? UNCERTAIN (code OK)` pending live host observation. The smoke script must include a `docker stats` capture before and after pause (one snapshot each) to convert SC1 from UNCERTAIN to VERIFIED. This is **above and beyond** the 5 explicit `issue` flips; it makes the script the canonical evidence document the v2.4 milestone audit will reference.

**Side observation already documented in 47-UAT.md line 60-63 (test 10 `note:`):** pause emitted `failed to capture etcd leader id ... exit status 127` using the legacy `docker exec etcdctl` path BEFORE 47-05 landed. After rebuild this MUST NOT appear in pause output. The script should grep stderr for the substring `failed to capture etcd leader id` and fail the run if it appears.

---

## Existing State of 51-UAT.md (the thing the planner must edit OR replace)

**Path:** `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md`

**Frontmatter:**
```yaml
---
status: complete        # ← already complete from 2026-05-07
phase: 51-upstream-sync-k8s-1-36
source: [51-01-SUMMARY.md, 51-02-SUMMARY.md, 51-03-SUMMARY.md]
started: 2026-05-07T15:00:00Z
updated: 2026-05-07T15:20:00Z
---
```

**Tests:** 3 tests, all `pass` with `evidence:` blocks. Total 3, passed 3, issues 0.

**The puzzle the planner must solve.** ROADMAP Success Criterion 3 says:

> Phase 51 UAT: `docker ps` confirms `envoyproxy/envoy` (not `kindest/haproxy`) as the LB container on the HA cluster; `kinder create cluster --config <ipvs+1.36-config>` is rejected at validate with migration URL in the error message; K8s 1.36 guide page renders with its sidebar entry; **`.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md` created with full evidence**

But `51-UAT.md` **already exists** with all 3 tests passing (against the May 7 binary, BEFORE phase 52-57 landed). The ROADMAP language ("created with full evidence") was written before that file existed; today it must be re-interpreted as **"refreshed against the final v2.4 binary so the evidence corresponds to the shipped release"**. Two acceptable paths exist:

**Option A — Augment, do not rewrite.** Add a new `## Re-verification against v2.4 binary` section to the existing 51-UAT.md, listing the same 3 tests with new `evidence:` blocks captured from the rebuilt binary. Update `updated:` timestamp. Frontmatter `status:` stays `complete`. This preserves the May 7 history.

**Option B — Replace.** Overwrite the body of 51-UAT.md with a fresh schema-equivalent doc whose 3 tests are dated 2026-05-12+ and reference `./bin/kinder` explicitly. The May 7 evidence is lost from the file but remains in git history.

**Recommendation: Option A.** It is non-destructive, preserves the milestone narrative ("we tested in May, re-tested at v2.4 close, both passed"), and matches the spirit of "full evidence against the final v2.4 binary" without erasing earlier evidence. The 58-02 plan should explicitly call out which option it chose so the verifier knows what to grade.

---

## Pitfall 23 (stale binary) — Concrete Mechanics

This is THE definitive gate of Phase 58. Source: `.planning/research/PITFALLS.md:399-417`.

**Reality check on the `./bin/kinder version` mechanism the ROADMAP SC1 invokes:**

`pkg/internal/kindversion/version.go` shows:

```go
const versionCore = "1.5.0"
var versionPreRelease = ""   // EMPTY
var gitCommitCount = ""      // injected by Makefile
var gitCommit = ""           // injected by Makefile

func version(core, preRelease, commit, commitCount string) string {
    v := core
    if preRelease != "" {     // ← gate
        v += "-" + preRelease
        if commitCount != "" { v += "." + commitCount }
        if commit != "" { v += "+" + truncate(commit, 14) }
    }
    return v
}
```

**Because `versionPreRelease == ""`, the commit hash and commit count are NEVER appended to the version string.** Running the current binary produces:

```
kind v1.5.0 go1.26.3 darwin/arm64
```

— with no hash. The Makefile **does** inject `-X=…gitCommit=$(COMMIT) -X=…gitCommitCount=$(COMMIT_COUNT)` at build time (Makefile:60-61), but the values are then suppressed by the `preRelease != ""` gate at output time.

**Implication:** Phase 58 scripts cannot satisfy ROADMAP SC1 ("`./bin/kinder version` confirms the v2.4 build hash") by parsing `version` output alone. The ROADMAP wording is aspirational; the scripts must adapt. Three viable approaches, in increasing order of robustness:

| Approach | Mechanism | Trust | Cost |
|----------|-----------|-------|------|
| **Mtime check** | `[ "$(stat -f %m bin/kinder)" -gt "$(git log -1 --format=%ct HEAD)" ]` | LOW — clock skew, manual touch can defeat | Free |
| **Strings introspection** | `strings bin/kinder \| grep -q 'all control-plane containers stopped'` (47-06 marker) AND `grep -q 'docker.io/envoyproxy/envoy:v1.36.2'` (51-01 marker) AND `grep -q '<DIAG-05/DIAG-06 marker>'` (57 marker) — and **NOT** match `grep 'failed to capture etcd leader id'` legacy path or `'label=io.x-k8s.kind.cluster=kind'` | HIGH — binary contents are authoritative | One-time author cost to pick stable markers |
| **Rebuild every invocation** | Script begins with `make build` unconditionally; treats the build as idempotent | HIGHEST — guaranteed-fresh, costs ~10 s | ~10 s per run |

**Recommendation: combine "Rebuild every invocation" + "Strings introspection".** Belt-and-suspenders. The rebuild guarantees fresh; the strings check catches the rare case where the rebuild silently no-op'd (e.g. dirty `bin/` cache, build flag drift). The 47-VERIFICATION.md document already authored the exact strings markers (see §1.4 below).

**Stable v2.4 marker strings to grep for in `strings bin/kinder`:**

| Marker | Source | Asserts |
|--------|--------|---------|
| `crictl ps --name etcd -q` | `pkg/internal/lifecycle/pause.go:259` (after 47-05) | 47-05 leader-ID crictl path present |
| `all control-plane containers stopped` | `pkg/internal/doctor/resumereadiness.go:135` (47-06) | 47-06 stopped-CPs warn path present |
| `docker.io/envoyproxy/envoy:v1.36.2` | `pkg/cluster/internal/loadbalancer/const.go:20` (51-01) | Envoy LB image baked in |
| `kubeProxyMode: ipvs is not supported with Kubernetes 1.36+` | `pkg/internal/apis/config/validate.go:92` (51-02) | IPVS-1.36 guard present |
| `quorum at risk` | DIAG-06 tolerant etcd JSON parse (57-02 — exact string per Pitfall 22 fix) | 57-02 wording landed |

**Stable NEGATIVE markers (must NOT appear in `strings bin/kinder`):**

| Marker | Means |
|--------|-------|
| `label=io.x-k8s.kind.cluster=kind` (value-pinned) | 47-06 clusterFilter not applied (would be a regression) |
| `/usr/local/bin/etcdctl` | pre-47-05 unreachable etcdctl path back somehow |
| `kindest/haproxy` | pre-51-01 LB image not removed |

**Note on macOS Homebrew Cask kinder symlink (the prior debug session):** 47-06-PLAN.md line 403-407 + 47-UAT.md test-14 root_cause document a real trap — `/opt/homebrew/bin/kinder` symlinks to a sealed Cask install (`/opt/homebrew/Caskroom/kinder/1.4/kinder`, dated April 11). If a Phase 58 script ever invokes bare `kinder` instead of `./bin/kinder`, it hits the Cask binary, which will fail every v2.4 check. ROADMAP SC4 explicitly requires `./bin/kinder` everywhere; the script must `set -u`, refuse `$PATH` resolution, and `command kinder` MUST NOT appear anywhere. (Belt: `which kinder` print + visual confirmation in script preamble.)

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| bash | 3.2+ (default macOS) | Driver shell for `uat-47-*.sh` and `uat-51-*.sh` | Already the language of every script under `hack/` (`verify-integration.sh`, `hack/ci/e2e.sh`, `hack/release/*`). No new dependency. |
| docker | 20.10+ (with Docker Desktop 4.x on macOS) | Container runtime for the kinder cluster | Already required by kinder. Validated by `kinder doctor`. |
| kubectl | match cluster K8s version (≥1.32) | Operate on the live cluster (apply Deployment/PVC/Service, get nodes, get pods) | Already user-installed for any kinder workflow. |
| make | GNU make (BSD make on macOS works for this Makefile) | Drive `make build` rebuild gate | Existing Makefile in repo root; `make build` is the documented entry point. |
| jq | 1.6+ | Parse `kinder pause --json` / `kinder resume --json` output and `kubectl get -o json` if needed | Already used by other hack scripts; user-installed on dev machines. Optional — pure `grep` / `awk` works if jq is unavailable. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| node + npm | Whatever `kinder-site/package.json` engines field requires | Boot `astro dev` server for the 1.36-guide rendering test | UAT-02 only. Already required for any kinder-site contributor. |
| curl | Anything modern | Probe `astro dev` HTTP server on `localhost:4321` | UAT-02 only. macOS ships it. |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Pure bash | go test / testscript | Tests in Go are already exhaustive at the unit level (47-VERIFICATION + 51-VERIFICATION both 100% green); Phase 58's job is to bridge from unit tests to live-Docker reality. Bash is the right tier for that bridge. |
| Recorded asciinema | Static text logs | Pitfall 23 §3 explicitly says "Do NOT record asciinema during the UAT itself — the recording is documentation, not a test gate." Plain `tee` to a `*.log` file is the canonical evidence form. |
| Sourcing kinder via PATH | Always `./bin/kinder` | SC4 mandates `./bin/kinder`. No tradeoff — non-negotiable. |

**Installation:** Nothing new to install. All tools are already present in any kinder developer environment.

### Version verification (no npm registry to query)

Not applicable — Phase 58 introduces no new package dependencies. The supporting tools are runtime-only (bash, docker, kubectl, jq), and their exact versions are not pinned by Phase 58.

---

## Architecture Patterns

### Recommended Project Structure

```
hack/
├── uat-47-ha-smoke.sh              # NEW (Plan 58-01)
├── uat-51-envoy-ipvs-guide.sh      # NEW (Plan 58-02)
├── ci/                              # existing — left untouched
├── build/                           # existing — left untouched
└── verify-integration.sh            # existing — left untouched

.planning/phases/47-cluster-pause-resume/
├── 47-UAT.md                        # EDITED (Plan 58-01 — flip 5 issue→pass; update timestamp; frontmatter status)
└── (other files unchanged)

.planning/phases/51-upstream-sync-k8s-1-36/
├── 51-UAT.md                        # EDITED (Plan 58-02 — append "Re-verification against v2.4 binary" section per Option A)
└── (other files unchanged)
```

**Rationale for `hack/` placement:** All existing kinder shell scripts live under `hack/` (per `hack/verify-integration.sh`, `hack/ci/e2e.sh`, etc.). REQUIREMENTS.md UAT-01/UAT-02 both literally say "evidence in `hack/uat-47-ha-smoke.sh` + log" — the path is locked. Do not create a new top-level `scripts/` directory; that would diverge from established convention.

**Rationale for ONE script per plan, ONE plan per UAT requirement:** Matches the 1:1:1 (Plan : Script : Requirement) shape. Easier verifier traceability. The two scripts are conceptually independent (different SCs, different runtime topology) — no shared library needed.

### System Architecture Diagram

```
                ┌─────────────────────────────────────────┐
                │ Developer terminal                       │
                │ $ bash hack/uat-47-ha-smoke.sh           │
                └─────────────────┬───────────────────────┘
                                  │
                                  ▼
                ┌─────────────────────────────────────────┐
                │ Preamble                                 │
                │ 1. set -euo pipefail                     │
                │ 2. cd to repo root                       │
                │ 3. make build                            │  ← rebuilds ./bin/kinder
                │ 4. strings ./bin/kinder | grep gates     │  ← positive markers present
                │ 5. strings ./bin/kinder | grep -v negs   │  ← negative markers absent
                │ 6. which kinder + printf './bin/kinder'  │  ← document path
                └─────────────────┬───────────────────────┘
                                  │ (preamble PASS)
                                  ▼
                ┌─────────────────────────────────────────┐
                │ Live phase                               │
                │ • ./bin/kinder create cluster --config   │
                │     <inline-HA-yaml> --name uat-58-01    │
                │ • docker stats --no-stream (baseline)    │
                │ • ./bin/kinder pause uat-58-01           │  ← orders: workers→CP→LB
                │ • docker stats --no-stream (post-pause)  │  ← SC1 host observation
                │ • ./bin/kinder doctor (test 12)          │
                │ • docker stop cp2 cp3                    │  ← quorum loss
                │ • ./bin/kinder doctor (test 13)          │
                │ • docker start cp2 cp3                   │  ← recover
                │ • ./bin/kinder resume uat-58-01 --wait 5m│  ← test 9 + ordering
                │ • kubectl apply -f <Dep+PVC+Svc>          │  ← state preservation
                │ • write sentinel into PVC, capture UIDs  │
                │ • ./bin/kinder pause + resume cycle      │
                │ • verify sentinel + UIDs unchanged       │  ← SC2
                │ • docker exec <cp> cat /kind/pause-      │  ← test 14
                │     snapshot.json | jq .leaderID         │
                │ • ./bin/kinder get nodes uat-58-01        │  ← test 3
                └─────────────────┬───────────────────────┘
                                  │
                                  ▼
                ┌─────────────────────────────────────────┐
                │ Capture phase                            │
                │ • tee everything to                       │
                │     hack/uat-47-ha-smoke.log              │
                │ • print "✓ PASS / ✗ FAIL <test#>"         │
                │ • leave cluster up by default; delete    │
                │   only on --teardown flag                 │
                └─────────────────────────────────────────┘
```

For Plan 58-02 the equivalent flow is shorter:

```
Preamble (same)
  ↓
Live phase (Envoy LB + IPVS guard):
  • ./bin/kinder create cluster --name uat-58-02 (HA: 2 CP + 1 worker)
  • docker ps --format '{{.Image}}\t{{.Names}}' | grep uat-58-02
  •    assert: envoyproxy/envoy present; kindest/haproxy absent
  • write /tmp/ipvs-1-36-test.yaml (ipvs + v1.36.0 image)
  • ./bin/kinder create cluster --config /tmp/ipvs-1-36-test.yaml || true
  •    assert: exit != 0; stderr contains the 4 required substrings
  • optional teardown
  ↓
Documentation phase (1.36 guide):
  • cd kinder-site && npm install (if needed)
  • npm run dev &   (background, capture PID)
  • wait for localhost:4321 to respond
  • curl -sf http://localhost:4321/guides/k8s-1-36-whats-new/
  •   | grep -q "User Namespaces" && grep -q "In-Place Pod Resize"
  • kill $DEV_PID
  ↓
Capture phase (same shape as 58-01)
```

### Pattern 1: Idempotent rebuild preamble

**What:** Every UAT script's first non-comment line after `set -euo pipefail` is `make build`. The Makefile target is idempotent — if `bin/kinder` is newer than every Go source file, the build is a no-op (~200 ms). If stale, the rebuild takes ~10 s.

**When to use:** ALL Phase 58 scripts. Non-negotiable.

**Example:**

```bash
#!/usr/bin/env bash
# Source: hack/uat-47-ha-smoke.sh — Phase 58 Plan 01

set -euo pipefail

# 1) Find repo root regardless of where the script is invoked from
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

# 2) Rebuild ./bin/kinder against current HEAD (idempotent on clean trees)
make build

# 3) Strings-marker gate — POSITIVE markers must all be present
KINDER_BIN="${REPO_ROOT}/bin/kinder"
REQUIRED_MARKERS=(
  "crictl ps --name etcd -q"
  "all control-plane containers stopped"
  "docker.io/envoyproxy/envoy:v1.36.2"
  "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+"
  "quorum at risk"
)
for m in "${REQUIRED_MARKERS[@]}"; do
  if ! strings "${KINDER_BIN}" | grep -qF "$m"; then
    echo "✗ STALE BINARY — required v2.4 marker not found: ${m}" >&2
    exit 1
  fi
done

# 4) NEGATIVE markers — must NOT appear
FORBIDDEN_MARKERS=(
  "label=io.x-k8s.kind.cluster=kind"   # 47-06 clusterFilter regression
  "/usr/local/bin/etcdctl"             # pre-47-05 path
  "kindest/haproxy"                    # pre-51-01 LB
)
for m in "${FORBIDDEN_MARKERS[@]}"; do
  if strings "${KINDER_BIN}" | grep -qF "$m"; then
    echo "✗ STALE BINARY — forbidden pre-v2.4 marker present: ${m}" >&2
    exit 1
  fi
done

# 5) Document version + path
"${KINDER_BIN}" version
echo "Using: ${KINDER_BIN}"
echo "HEAD:  $(git rev-parse HEAD)"
echo "Build: $(stat -f '%Sm' "${KINDER_BIN}")"
```

### Pattern 2: HA cluster config inline (no /tmp YAML dependency)

**What:** The 3-CP + 2-worker + 1-LB cluster config is inlined as a here-doc piped to `./bin/kinder create cluster --config -`. This avoids depending on a kept config file under `hack/` that could drift.

**Why:** 47-VERIFICATION.md §3 + REQUIREMENTS.md UAT-01 both require a 3 CP + 2 worker + 1 LB cluster. The `--config -` form is documented kinder usage (51-UAT.md test 1 uses the same pattern).

**Example:**

```bash
./bin/kinder create cluster --name uat-58-01 --config - <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: control-plane
  - role: control-plane
  - role: worker
  - role: worker
EOF
```

(The LB container is created automatically by kinder when nodes count(control-plane) >= 2 — no explicit `external-load-balancer` role is required in the config. The 51-UAT.md test 1 evidence confirms this with `docker ps` showing `ha-test-external-load-balancer` for a 2-CP config.)

### Pattern 3: docker stats observation as SC1 evidence

**What:** Take `docker stats --no-stream` snapshots at three points: (a) after cluster create but before pause; (b) immediately after pause completes; (c) immediately after resume completes. SC1 ("CPU and RAM drop to near-zero") is verified when the post-pause snapshot shows all `uat-58-01-*` containers absent OR with `CPU %` = 0.00% and very low `MEM %`.

**Why:** 47-VERIFICATION.md human_verification §1 says: "Docker container stop semantics and host CPU/RAM observation cannot be automated in unit tests; tests verify the code calls `docker stop` but cannot assert host-side resource reclamation." A 3-snapshot log is the lightweight automation that bridges this gap.

**Subtle point:** `docker stats` only shows RUNNING containers by default. After pause, the kinder containers are STOPPED, so they vanish from `docker stats` entirely — which is exactly the SC1 signal. The script should capture `docker ps -a --filter label=io.x-k8s.kind.cluster=uat-58-01 --format '{{.Names}}: {{.Status}}'` post-pause and assert every line contains `Exited`.

### Pattern 4: PVC sentinel for state-preservation evidence

**What:** Before the pause/resume cycle, deploy a Deployment + PVC + Service; write a known string ("UAT-58-01-SENTINEL-<timestamp>") into a file on the PVC mount; capture the pod UID and ClusterIP. After resume, read the same file, capture the same UID/ClusterIP — assert all three are byte-identical to pre-pause.

**Why:** 47-VERIFICATION.md human_verification §2 explicitly enumerates this as a human-required test. Automating it in bash closes the test that no Go unit test can ever close.

**Example:**

```bash
# Pre-pause: deploy a Deployment + PVC + Service
kubectl --context kind-uat-58-01 apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata: { name: uat-pvc }
spec:
  accessModes: [ReadWriteOnce]
  resources: { requests: { storage: 100Mi } }
---
apiVersion: apps/v1
kind: Deployment
metadata: { name: uat-deploy }
spec:
  replicas: 1
  selector: { matchLabels: { app: uat } }
  template:
    metadata: { labels: { app: uat } }
    spec:
      containers:
        - name: c
          image: busybox:1.37.0
          command: ["sleep", "infinity"]
          volumeMounts: [{ name: data, mountPath: /data }]
      volumes:
        - name: data
          persistentVolumeClaim: { claimName: uat-pvc }
---
apiVersion: v1
kind: Service
metadata: { name: uat-svc }
spec:
  selector: { app: uat }
  ports: [{ port: 80, targetPort: 80 }]
EOF

kubectl wait --for=condition=Ready pod -l app=uat --timeout=120s

SENTINEL="UAT-58-01-SENTINEL-$(date +%s)"
POD_NAME=$(kubectl get pod -l app=uat -o jsonpath='{.items[0].metadata.name}')
POD_UID=$(kubectl get pod "${POD_NAME}" -o jsonpath='{.metadata.uid}')
SVC_IP=$(kubectl get svc uat-svc -o jsonpath='{.spec.clusterIP}')

kubectl exec "${POD_NAME}" -- sh -c "echo '${SENTINEL}' > /data/sentinel.txt"

# Pause/resume cycle
./bin/kinder pause uat-58-01
./bin/kinder resume uat-58-01 --wait 5m

# Post-resume: read sentinel + recapture UID/IP
kubectl wait --for=condition=Ready pod -l app=uat --timeout=120s

POD_NAME_AFTER=$(kubectl get pod -l app=uat -o jsonpath='{.items[0].metadata.name}')
POD_UID_AFTER=$(kubectl get pod "${POD_NAME_AFTER}" -o jsonpath='{.metadata.uid}')
SVC_IP_AFTER=$(kubectl get svc uat-svc -o jsonpath='{.spec.clusterIP}')
SENTINEL_READBACK=$(kubectl exec "${POD_NAME_AFTER}" -- cat /data/sentinel.txt)

[[ "${POD_NAME}" == "${POD_NAME_AFTER}" ]] || { echo "✗ pod name changed: ${POD_NAME} → ${POD_NAME_AFTER}"; exit 1; }
[[ "${POD_UID}"  == "${POD_UID_AFTER}"  ]] || { echo "✗ pod UID changed: ${POD_UID} → ${POD_UID_AFTER}"; exit 1; }
[[ "${SVC_IP}"   == "${SVC_IP_AFTER}"   ]] || { echo "✗ ClusterIP changed: ${SVC_IP} → ${SVC_IP_AFTER}"; exit 1; }
[[ "${SENTINEL}" == "${SENTINEL_READBACK}" ]] || { echo "✗ PVC sentinel lost: ${SENTINEL} != ${SENTINEL_READBACK}"; exit 1; }
echo "✓ state preserved (pod UID, ClusterIP, PVC sentinel all identical)"
```

### Pattern 5: Astro dev server for guide rendering

**What:** Boot `npm run dev` (which is `astro dev`) in `kinder-site/`, poll `http://localhost:4321/guides/k8s-1-36-whats-new/` until it returns 200, then assert the body contains the two GA-feature headings.

**Why:** 51-UAT.md test 3 was confirmed `pass` against May 7 server. Re-verification against v2.4 (which post-dates Phase 51) is paranoia, but cheap.

**Subtlety:** `astro dev` listens on port 4321 by default (confirmed by `kinder-site/package.json` and Starlight defaults). The script must (a) `cd kinder-site`; (b) run `npm install` only if `node_modules` is missing (idempotent); (c) spawn `npm run dev` in background; (d) poll with `curl --max-time 30`; (e) kill the background process via `kill $!` or trap.

### Anti-Patterns to Avoid

- **Recording asciinema during the UAT.** Pitfall 23 §3 explicitly forbids this — recording is documentation, not a gate. Use `tee uat-47-ha-smoke.log` for plain-text capture; record separately if a demo video is needed.
- **Running smoke against `kinder` from `$PATH`.** SC4 mandates `./bin/kinder`. The macOS Homebrew Cask trap (47-UAT test 14 root_cause) is the reason. Hardcode the path; never use bare `kinder`.
- **Skipping `make build` "because the binary was built yesterday".** Even one missed rebuild against an intermediate commit produces misleading evidence. The build is idempotent; cost of paranoia is ~200 ms when current. Always rebuild.
- **Embedding the cluster name in expected output strings.** Phase 47's UAT test 12 was caught by this — `clusterskew.go` had `=kind` hardcoded. Use generic patterns; assert structurally (`grep -E 'cluster-resume-readiness.*ok.*3/3'`), not by exact substring match.
- **Deleting the cluster on script failure.** A failed UAT is a debugging artifact. The cluster should remain up unless an explicit `--teardown` flag is passed, so the developer can `kubectl get pods -A`, `docker logs`, etc.
- **Trusting `kinder pause --json` output schema as a stable spec.** It IS stable (47-CONTEXT.md decision: "JSON output supported on both commands, full parity with v1.4 phase-29 JSON output convention"), but the UAT should not be parsing fields beyond `state == "paused"` / `state == "running"`. Anything richer becomes test-spec drift the next time JSON keys are added.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| "Is the binary fresh?" check | A new `kinder version --commit` Go subcommand | `strings $(./bin/kinder)` + marker grep; `make build` always | Adding a new Go subcommand for a UAT gate is over-engineering. The strings technique is already prescribed by 47-VERIFICATION.md and is more honest (it inspects the artifact, not a string the artifact prints). |
| Wait-for-Ready polling | A custom kubectl poller in bash | `kubectl wait --for=condition=Ready` (already used in Phase 53-04 UAT exemplar) | `kubectl wait` is the upstream pattern; handles timeout, jsonpath, and condition matching natively. |
| HA cluster YAML | A generated YAML file under `hack/config/` | Inline here-doc piped to `--config -` | One file per UAT keeps blast radius low; here-docs are the kinder-documented input form (51-UAT.md test 1 uses it). |
| HTTP probe for astro dev | A node script | `curl --silent --fail --max-time 30 http://localhost:4321/...` retried in a loop | curl is universally available; jest/playwright is overkill for "is the page reachable?". |
| Process supervision (astro dev background lifecycle) | A `pgrep`/`pkill` matrix | `trap 'kill $DEV_PID 2>/dev/null' EXIT` after backgrounding | The trap pattern is idiomatic bash; survives script abort. |
| Pause-snapshot leaderID parsing | A jq schema check | `docker exec <cp> cat /kind/pause-snapshot.json \| jq -r .leaderID` + `[[ -n "$result" ]]` | The file shape is locked by 47-05; jq one-liner is fine. |
| Capturing docker stats over time | `docker events` stream | One-shot `docker stats --no-stream` at 3 timestamps | We only need 3 snapshots (pre/post-pause/post-resume). Streaming is overkill and harder to log. |

**Key insight:** Phase 58 is a thin shell-layer veneer over already-shipped Go behaviors. Every problem solved here has a 1-line idiomatic answer; resist the temptation to add abstractions, helper scripts, or libraries. The two scripts should each be ≤ 200 lines.

---

## Runtime State Inventory

> Phase 58 is a UAT/documentation phase — no source rename, refactor, or migration. This section is included to confirm "nothing applies."

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no databases or datastores involved. Phase 58 produces logs that are committed as text. | None |
| Live service config | None — the kinder clusters created by the scripts are ephemeral and torn down (or left for inspection, then deleted by the developer). No external service config (Datadog, Tailscale, etc.) is touched. | None |
| OS-registered state | None — no `launchctl`, no Windows Task Scheduler, no systemd units involved. | None |
| Secrets/env vars | None — the scripts do not read or write secrets. `KUBECONFIG` is read implicitly via `kubectl`'s usual resolution. | None |
| Build artifacts | `./bin/kinder` is rebuilt by every script invocation (Makefile target). No new artifacts created; nothing left stale. | None — the rebuild itself is the runtime-state inventory action |

**Verified by:** Reading the script-deliverable description; greping `.planning/` for any Phase-58 era mentions of databases, secrets, or OS-level state — zero matches.

---

## Common Pitfalls

### Pitfall A: HA cluster RAM budget exceeds host capacity

**What goes wrong:** A 3 CP + 2 worker + 1 LB cluster is 6 Docker containers. Each kindest/node container baseline is ~500-800 MB of resident RAM with addons. Cluster creation on a 16 GB MacBook can OOM Docker Desktop's VM if other apps are open.

**Why it happens:** Docker Desktop on macOS runs Linux inside a VM with a fixed RAM allocation (default 8 GB on Intel, 12 GB on Apple Silicon as of 2026). Six kindest/node containers + the host system can push the VM close to its ceiling, causing kubelet startup to be slow or to fail entirely.

**How to avoid:** Pitfall 23 §4 already prescribes this — before creating the HA cluster, run `docker stats --no-stream` and confirm at least 4 GB of free RAM in the Docker VM. The script should print Docker VM allocation (`docker info | grep -E 'Total Memory|CPUs'`) at preamble time so the developer can correlate failures to under-provisioning. **The script SHOULD NOT abort on low RAM** (developer judgment); it should warn loudly.

**Warning signs:**
- `./bin/kinder create cluster` hangs at "Waiting ≤ 2m0s for control-plane = Ready"
- `docker logs uat-58-01-control-plane` shows `kubelet: ... OOMKilled`
- Docker Desktop dashboard shows >90% memory pressure

### Pitfall B: macOS host cannot curl docker-bridge IPs

**What goes wrong:** 53-04-SUMMARY.md UAT-SCRIPT NOTE 2 documents this: "macOS hosts cannot curl docker-bridge IPs (curl HTTP 000); EG UAT scripts should use kubectl run uat-curl (in-cluster curl) or kubectl port-forward on macOS."

**Why it happens:** Docker Desktop's network model on macOS does NOT bridge the docker0 network to the host's network namespace. The host cannot reach `172.19.x.x` IPs directly.

**How to avoid:** Plan 58-01 uses `kubectl exec` to write the sentinel file from inside the pod, so no host-to-container HTTP is required. Plan 58-02 uses an in-cluster Service IP probe via `kubectl run uat-curl --image=curlimages/curl` if any curl-to-cluster is needed — but UAT-02's only HTTP probe is `localhost:4321` (the astro dev server), which IS reachable from the host. Net: this pitfall is in scope but neither script triggers it.

**Warning signs:**
- `curl` from the host to a ClusterIP returns `HTTP 000` or "Failed to connect"
- The same curl works inside a `kubectl run` pod

### Pitfall C: Astro dev server zombie

**What goes wrong:** If the UAT-02 script is killed mid-run (Ctrl-C, terminal close, error trap missing), `npm run dev` lingers as a zombie process holding port 4321. Next invocation fails with `EADDRINUSE`.

**How to avoid:** `trap 'kill -TERM $DEV_PID 2>/dev/null; wait $DEV_PID 2>/dev/null' EXIT INT TERM` at the top of the script. Also check `lsof -i :4321` at preamble time and fail with a clear message if the port is already bound.

**Warning signs:**
- Script aborts; subsequent run reports `address already in use` from astro
- `lsof -i :4321` shows a `node` PID owned by the developer

### Pitfall D: kubectl context drift after cluster delete

**What goes wrong:** After `./bin/kinder delete cluster --name <foo>`, the kubeconfig entry for `kind-<foo>` is removed, but if the developer's `KUBECONFIG` env points to a custom file with a current-context set to `kind-<foo>`, subsequent `kubectl` commands fail with "no current context" rather than a sensible error.

**How to avoid:** The script should explicitly set its kubeconfig context with `kubectl --context kind-uat-58-01 ...` (or `kubectl config use-context kind-uat-58-01` once at the top) and never rely on the developer's ambient `current-context`.

**Warning signs:**
- `kubectl get nodes` returns "context not found" inside the UAT script
- Developer's shell loses kubectl context after the script runs

### Pitfall E: `kinder pause` JSON schema parsing breakage

**What goes wrong:** 47-CONTEXT.md commits to a JSON schema for `--json` output (`{cluster, state, nodes[], durationSeconds, alreadyPaused?}`). If the UAT script greps for an exact string ("Cluster paused. Total time:") in text mode while the developer runs with `--json`, the assertion fails. Conversely, if the script uses `--json` and the schema changes in a v2.5 cosmetic fix, the script breaks silently.

**How to avoid:** Use text mode (no `--json` flag) for ordering observation (`grep -E 'worker|control-plane|external-load-balancer'` against stdout to verify ordering); use `--json` only where structural data is needed (e.g. capturing `nodes[].durationSeconds`). Document the schema dependency in a script-header comment so a future cosmetic-fix PR has a reason to update the UAT script in the same commit.

### Pitfall F: Stale Cask `kinder` shadowing `./bin/kinder`

**What goes wrong:** Same root cause as 47-UAT.md test 14 — `/opt/homebrew/bin/kinder` is a symlink into the Cask install. If the script ever invokes bare `kinder`, it hits that stale binary.

**How to avoid:** Hardcode the absolute path `${REPO_ROOT}/bin/kinder` everywhere. No `PATH` lookups. The preamble already does `which kinder` for documentation, but the script's actual invocations MUST use the absolute path. (`set -u` is necessary but not sufficient; it only catches unset variables, not PATH resolution drift.)

**Warning signs:**
- `which kinder` in preamble does NOT print the repo bin path
- The version-string output in the preamble does NOT match `${REPO_ROOT}/bin/kinder version`

### Pitfall G: Live UAT script committed without `.log` artifact

**What goes wrong:** The plan's "deliverable surface" should be the script + the log file. If only the script lands in git, the verifier has no evidence the script was ever run end-to-end. The script could be "looks right but never executed".

**How to avoid:** The plan's commit list should include `hack/uat-47-ha-smoke.log` (and `hack/uat-51-envoy-ipvs-guide.log`). These logs are produced by the script's `tee` invocation. They are not pristine — they contain real timestamps, real container IDs, real kubectl output — but that is exactly what makes them evidence. Add `.gitignore` notes: the .log files are intentionally NOT gitignored.

**Warning signs:**
- `git status` after the script run shows the .log file as untracked but the developer forgot to commit it
- The PR has a script but no proof of execution

---

## Code Examples

### Example A: Plan 58-01 — minimal viable shape (NOT the full script)

```bash
#!/usr/bin/env bash
# Source: hack/uat-47-ha-smoke.sh
# Closes: REQUIREMENTS.md UAT-01 + 47-UAT.md tests 3, 9, 12, 13, 14 (and SC1/SC2 host observation)
# Pitfall 23 gate: this script rebuilds ./bin/kinder against current HEAD on every invocation.

set -euo pipefail

CLUSTER="uat-58-01"
LOG="${LOG:-hack/uat-47-ha-smoke.log}"
TEARDOWN="${TEARDOWN:-no}"   # set to "yes" to delete cluster at end

main() {
  preamble                         # make build + strings gate + version print
  create_ha_cluster                # 3 CP + 2 workers + 1 LB
  observe_baseline_stats
  test_03_get_nodes_positional
  test_09_resume_wait_duration_string
  test_12_doctor_healthy_3of3
  test_13_doctor_warn_quorum_loss
  test_14_pause_snapshot_leaderid
  test_state_preservation          # SC1 + SC2 PVC/Service round-trip
  capture_pause_ordering
  capture_resume_ordering
  finalize
}

# (Each function implements one test, captures output to ${LOG}, asserts the
#  pass condition, and prints "✓ <test> PASS" or "✗ <test> FAIL". Failures
#  exit 1; the cluster is left up for debugging.)

trap 'echo "Cluster ${CLUSTER} left up for inspection. Run ./bin/kinder delete cluster --name ${CLUSTER} to clean up." >&2' ERR

main "$@" 2>&1 | tee "${LOG}"
```

### Example B: Plan 58-02 — minimal viable shape (NOT the full script)

```bash
#!/usr/bin/env bash
# Source: hack/uat-51-envoy-ipvs-guide.sh
# Closes: REQUIREMENTS.md UAT-02 + re-verifies 51-UAT.md tests 1, 2, 3 against v2.4 binary

set -euo pipefail

CLUSTER="uat-58-02"
LOG="${LOG:-hack/uat-51-envoy-ipvs-guide.log}"

main() {
  preamble                                    # same shape as 58-01
  test_01_envoy_lb_on_ha_cluster              # docker ps --filter | grep envoyproxy/envoy + assert NOT haproxy
  test_02_ipvs_1_36_rejected_at_validate      # exit != 0 + 4-substring assertion on stderr
  test_03_k8s_1_36_guide_renders              # boot astro dev, curl localhost:4321, assert page body
  finalize
}

trap 'cleanup_astro' EXIT INT TERM

main "$@" 2>&1 | tee "${LOG}"
```

### Example C: Test 02 (IPVS guard) — exact stderr assertion

```bash
test_02_ipvs_1_36_rejected_at_validate() {
  cat > /tmp/ipvs-1-36-test.yaml <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  kubeProxyMode: ipvs
nodes:
  - role: control-plane
    image: kindest/node:v1.36.0
EOF

  local stderr_capture
  set +e
  stderr_capture="$(./bin/kinder create cluster --config /tmp/ipvs-1-36-test.yaml --name should-not-exist 2>&1 >/dev/null)"
  local exit_code=$?
  set -e

  [[ $exit_code -ne 0 ]] || { echo "✗ test_02: kinder did NOT reject ipvs+1.36 config (exit 0)"; return 1; }

  # All four required substrings (per validate.go:92-95):
  local required=(
    "kubeProxyMode: ipvs is not supported with Kubernetes 1.36+"
    "kube-proxy IPVS mode was deprecated in v1.35"
    "Switch to iptables or nftables"
    "https://kubernetes.io/docs/reference/networking/virtual-ips/"
  )
  for s in "${required[@]}"; do
    grep -qF "$s" <<< "${stderr_capture}" || { echo "✗ test_02: error message missing substring: ${s}"; return 1; }
  done

  # Confirm no container was created:
  if docker ps -a --filter "name=should-not-exist" --format '{{.Names}}' | grep -q .; then
    echo "✗ test_02: kinder created a container despite validation rejection"
    return 1
  fi

  echo "✓ test_02 PASS — ipvs+1.36 rejected; all 4 substrings present; no container created"
}
```

### Example D: Test 01 (Envoy LB) — exact docker ps assertion

```bash
test_01_envoy_lb_on_ha_cluster() {
  ./bin/kinder create cluster --name "${CLUSTER}" --config - <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: control-plane
  - role: worker
EOF

  # Envoy LB image MUST be present:
  local lb_image
  lb_image="$(docker ps --filter "label=io.x-k8s.kind.cluster=${CLUSTER}" --filter "name=external-load-balancer" --format '{{.Image}}')"
  [[ "${lb_image}" == "docker.io/envoyproxy/envoy:v1.36.2" ]] \
    || { echo "✗ test_01: LB image = '${lb_image}', want 'docker.io/envoyproxy/envoy:v1.36.2'"; return 1; }

  # HAProxy MUST NOT be present anywhere:
  if docker ps -a --format '{{.Image}}' | grep -q 'kindest/haproxy'; then
    echo "✗ test_01: kindest/haproxy container is running (must be absent post-51-01)"
    return 1
  fi

  echo "✓ test_01 PASS — Envoy LB present; HAProxy absent"
}
```

### Example E: 47-UAT.md status-flip mechanic

The planner needs to edit the existing YAML-style block. The current row for test 3:

```yaml
### 3. kinder get nodes shows real container state
expected: ...
result: issue
reported: "kinder get nodes verify47 -> ERROR: unknown command..."
severity: major
```

Becomes:

```yaml
### 3. kinder get nodes shows real container state
expected: ...
result: pass
evidence: |
  Live UAT against ./bin/kinder built from HEAD <commit>.
  $ ./bin/kinder get nodes uat-58-01
  <captured node table here>
  Exit 0. Resolved via cobra.MaximumNArgs(1) + lifecycle.ResolveClusterName (47-06).
note: |
  Closed by 47-06 source fixes + Phase 58 live UAT. See hack/uat-47-ha-smoke.log for full transcript.
```

Note: `reported:` and `severity:` keys are dropped on flip (they are issue-only metadata); `evidence:` is added (matches the 51-UAT.md schema for pass rows). The `## Summary` block at the bottom of 47-UAT.md must update: `passed: 9 → passed: 14`, `issues: 5 → issues: 0`. The `## Gaps` section becomes empty or is removed entirely.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| HAProxy as LB image (`kindest/haproxy:vYYYYMMDD-…`) | Envoy as LB image (`docker.io/envoyproxy/envoy:v1.36.2`) with xDS LDS/CDS atomic mv-swap | Phase 51-01 (May 7, 2026) | All HA clusters created after May 7 ship with Envoy. Phase 58 must observe Envoy in `docker ps`. |
| IPVS kube-proxy on K8s 1.36+ allowed (silently broken) | Rejected at validate with migration URL | Phase 51-02 (May 7, 2026) | Phase 58 must observe non-zero exit + 4 required substrings in stderr. |
| `cluster-resume-readiness` LB-role false positive | Inline role guard in `realListNodes` (skips external-load-balancer + external-etcd) | Phase 57-01 (May 12, 2026) | Phase 58 test_12 should observe clean ok output with no LB-related noise. |
| `cluster-resume-readiness` raw etcdctl JSON dump on quorum loss | Tolerant JSON parse, prints "N/M etcd members healthy" + "quorum at risk" | Phase 57-02 (May 12, 2026) | Phase 58 test_13 should observe `quorum at risk` substring (NOT raw JSON). |
| `kinder resume --wait 5m` rejected (IntVar) | Accepted (DurationVar) | Phase 47-06 (May 5, 2026) | Phase 58 test_09 verifies. |
| `kinder get nodes <cluster>` rejected (cobra.NoArgs) | Accepted (cobra.MaximumNArgs(1) + ResolveClusterName) | Phase 47-06 (May 5, 2026) | Phase 58 test_03 verifies. |

**Deprecated/outdated:**

- `docker exec ... etcdctl` legacy probe path: replaced by `crictl ps --name etcd -q + crictl exec etcdctl` in 47-05. Phase 58 must grep stderr to confirm absence of "failed to capture etcd leader id" — its presence would mean the rebuild silently failed.
- `kindest/haproxy:*` image: removed in 51-01. Phase 58 strings-gate forbids it in `bin/kinder`; live-phase grep forbids it in `docker ps`.

---

## Open Questions

1. **How should we represent commit-hash-equality in the script when `kinder version` doesn't print the hash?**
   - What we know: `versionPreRelease == ""`, so `gitCommit` is injected at build time but never rendered. Strings introspection works (we proved this with `strings bin/kinder | grep ...`).
   - What's unclear: Should Phase 58 also propose changing `versionPreRelease` to e.g. "v2.4" in `kindversion/version.go` so the hash is visible in `version` output? That would make the SC1 wording ("`./bin/kinder version` confirms the v2.4 build hash") literally true.
   - Recommendation: **No.** Out of scope for Phase 58. Document the gap in 58-01 plan; defer to a v2.5+ "version output cosmetics" item. The strings-introspection technique is sufficient for milestone closure and is already prescribed by 47-VERIFICATION.md.

2. **Should the UAT scripts be wired into CI (a new GitHub Actions workflow) or run only manually by the developer at milestone close?**
   - What we know: Pitfall 23 §4 mentions "use a CI runner with 32 GB" as a fallback for under-provisioned laptops. But no existing Phase 58 reference says CI execution is required.
   - What's unclear: REQUIREMENTS.md UAT-01/UAT-02 only require "evidence in `hack/uat-*.sh` + log" — no CI invocation requirement.
   - Recommendation: **Manual only for v2.4.** The scripts must be runnable locally (their primary value). A future v2.5 "live-UAT in CI" phase can wire them into a self-hosted runner once the patterns are stable. Document this deferral in 58-01/58-02 plans.

3. **Should 51-UAT.md re-verification overwrite (Option B) or augment (Option A)?**
   - What we know: 51-UAT.md exists from May 7 with all 3 tests passing.
   - What's unclear: Spirit of ROADMAP SC3 ("created with full evidence") — does "created" mean "freshly created" or does the existing file satisfy it?
   - Recommendation: **Option A (augment).** Append a `## Re-verification against v2.4 binary` section dated 2026-05-12+ with three new evidence blocks. Preserves narrative; non-destructive.

4. **Should the cluster be torn down at end of script (default) or left up (default)?**
   - What we know: Failed UAT artifacts are debugging gold. Successful UAT clusters are clutter.
   - Recommendation: **Leave up by default; tear down only on explicit `--teardown` or `TEARDOWN=yes` env var.** Document the cleanup command in the script's final printed message so the developer can copy-paste.

5. **What is the upper-bound script runtime budget?**
   - What we know: HA cluster create on Apple Silicon ~3-5 min. Pause ~30 s. Resume ~60-90 s. Tests ~5 s each. Total: ~8-12 min for 58-01; ~5-7 min for 58-02.
   - Recommendation: Print a budget banner at script start ("ETA: ~10 min"). No hard timeout — fail-fast on individual test failures is enough.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker daemon | Both scripts | ✓ (assumed — kinder requires it) | Docker Desktop 4.x or Docker Engine 20.10+ | None — fail-fast with clear error if `docker version` exits non-zero |
| `make` | Both scripts (rebuild) | ✓ (macOS ships `bsdmake`; GNU make is in dev shell) | Anything modern | None — Makefile is bash-compatible |
| `kubectl` | Plan 58-01 (apply Deployment/PVC/Service); Plan 58-02 (optional) | ✓ (assumed — every kinder user has it) | Match cluster K8s version (≥1.32) | None |
| `jq` | Both scripts (parse JSON, parse pause-snapshot.json) | ✓ on dev machines; may be absent on minimal CI | 1.6+ | Fall back to `awk` / `grep` if jq missing; warn in preamble |
| `strings` | Both scripts (binary marker gate) | ✓ (macOS ships it; binutils-strings on Linux) | n/a | None — strings is fundamental |
| `npm` + `node` | Plan 58-02 only (astro dev) | ✓ (any kinder-site contributor) | Match `kinder-site/package.json` engines | Skip test 3 with a documented skip reason if npm is missing |
| `curl` | Plan 58-02 only (probe astro dev) | ✓ (macOS ships it) | Any | None |
| `tee`, `stat`, `lsof`, `trap` | Both scripts | ✓ (POSIX or near-POSIX, macOS-native) | n/a | None |

**Missing dependencies with no fallback:** None — every required tool is present on a kinder developer machine by construction (kinder requires Docker; UAT requires kinder).

**Missing dependencies with fallback:** `jq` (graceful degradation to grep/awk).

**Skip condition:** If npm/node is unavailable, Plan 58-02 SHOULD skip test 3 with `result: skipped` and `reason: "npm not installed; rendering test deferred to a machine with kinder-site toolchain"`. Document in script-header comment.

---

## Plan 58-01 — Deliverable Surface

**Scope:** Close UAT-01 (REQUIREMENTS.md). Flip 5 `issue` rows in `47-UAT.md` to `pass`. Capture host CPU/RAM observation evidence for SC1/SC2.

**Files modified (commit list, in this order):**

| # | File | Change |
|---|------|--------|
| 1 | `hack/uat-47-ha-smoke.sh` | NEW. ≤ 250 lines. Idempotent rebuild preamble + strings gate + create-3CP-2W-LB cluster + observe + pause + doctor-tests + resume + state-preservation roundtrip + ordering capture + finalize. |
| 2 | `hack/uat-47-ha-smoke.log` | NEW (committed evidence). Output of one successful run, captured via `tee`. Includes `./bin/kinder version` line at top, `git rev-parse HEAD` at top, `stat` of `./bin/kinder` at top, all 5+ test pass lines, final summary. |
| 3 | `.planning/phases/47-cluster-pause-resume/47-UAT.md` | EDITED. (a) frontmatter `status: diagnosed` → `status: closed`; `updated:` to current timestamp. (b) tests 3, 9, 12, 13, 14 `result: issue` → `result: pass`; drop `reported:` and `severity:` keys; add `evidence:` block. (c) summary block: `passed: 9` → `passed: 14`; `issues: 5` → `issues: 0`. (d) `## Gaps` section: replace contents with a single line "All UAT issues closed via Phase 58 Plan 01 (2026-05-12). See `hack/uat-47-ha-smoke.log`." or remove entirely. |
| 4 | `.planning/phases/58-live-uat-closure-for-phase-47-51/58-01-SUMMARY.md` | NEW. Standard Phase summary doc. |
| 5 | `.planning/REQUIREMENTS.md` | EDITED. UAT-01 row: `[ ]` → `[x]`. Traceability table: `UAT-01 | Phase 58 | Pending` → `UAT-01 | Phase 58 | Complete`. Bottom-of-file timestamp line. |
| 6 | `.planning/ROADMAP.md` | EDITED. Progress table row for Phase 58: increment plans complete. Phase 58 bullet list under "v2.4 Hardening (In Progress)" — Plan 58-01 row added. |
| 7 | `.planning/STATE.md` | EDITED. Update `stopped_at`, `last_activity`, performance metrics table row for 58-01, accumulated context decision line for 58-01. |

**Tasks (likely shape — planner can override):**

| Task | Type | Description |
|------|------|-------------|
| 1 | auto | Write `hack/uat-47-ha-smoke.sh` (no UAT run yet — pure authoring + lint with `shellcheck` if available + `bash -n` syntax check). Commit. |
| 2 | checkpoint:human-verify | Developer runs `bash hack/uat-47-ha-smoke.sh` on their machine. Reply `pass` with paste of summary lines, OR reply `fail` with paste of the failing test output. Hold gate. |
| 3 | auto | On `pass`: commit the `.log` file, flip 47-UAT.md status fields, write 58-01-SUMMARY.md, update REQUIREMENTS/ROADMAP/STATE. On `fail`: triage path (likely a stale Docker Desktop cache, not a kinder bug; defer back to Task 2 after the developer cleans up). |

**Acceptance criteria for the plan:**

- All 5 `issue` rows in 47-UAT.md flipped to `pass` with evidence text referencing the `.log` file
- `47-UAT.md` summary block reads `passed: 14`, `issues: 0`
- `hack/uat-47-ha-smoke.log` is committed and contains the verbatim output of one successful run
- REQUIREMENTS.md UAT-01 marked `[x]` and Traceability table row marked `Complete`
- ROADMAP and STATE updated per kinder convention

---

## Plan 58-02 — Deliverable Surface

**Scope:** Close UAT-02 (REQUIREMENTS.md). Re-verify 51-UAT.md's 3 tests against the v2.4 binary; augment 51-UAT.md (Option A) with a new evidence section.

**Files modified (commit list, in this order):**

| # | File | Change |
|---|------|--------|
| 1 | `hack/uat-51-envoy-ipvs-guide.sh` | NEW. ≤ 200 lines. Preamble + test_01 (Envoy LB on HA cluster) + test_02 (IPVS+1.36 reject with 4 substrings) + test_03 (astro dev guide rendering) + finalize. |
| 2 | `hack/uat-51-envoy-ipvs-guide.log` | NEW (committed evidence). |
| 3 | `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md` | EDITED (Option A — augment). Append a new section `## Re-verification against v2.4 binary (Phase 58)` containing three sub-blocks, one per test, each with: dated header, captured commands, captured output excerpts, `result: pass` (matching the existing schema). Update frontmatter `updated:` to current timestamp. Frontmatter `status:` stays `complete`. |
| 4 | `.planning/phases/58-live-uat-closure-for-phase-47-51/58-02-SUMMARY.md` | NEW. |
| 5 | `.planning/REQUIREMENTS.md` | EDITED. UAT-02 row `[ ]` → `[x]`. Traceability: `UAT-02 | Phase 58 | Pending` → `Complete`. |
| 6 | `.planning/ROADMAP.md` | EDITED. Phase 58 progress; plan list. |
| 7 | `.planning/STATE.md` | EDITED. |

**Tasks:**

| Task | Type | Description |
|------|------|-------------|
| 1 | auto | Write `hack/uat-51-envoy-ipvs-guide.sh`. Lint + `bash -n`. Commit. |
| 2 | checkpoint:human-verify | Developer runs the script. Reply `pass` with summary or `fail` with paste. |
| 3 | auto | On `pass`: commit log, append 51-UAT.md re-verification section, write 58-02-SUMMARY, update REQUIREMENTS/ROADMAP/STATE. |

**Acceptance criteria:**

- `hack/uat-51-envoy-ipvs-guide.log` shows: (a) Envoy image confirmed via `docker ps`; haproxy absent; (b) IPVS-1.36 config rejected with exit non-zero and all 4 required substrings; (c) astro dev server boots and serves the 1.36 guide with both GA feature headings rendered.
- 51-UAT.md gains a `## Re-verification against v2.4 binary` section.
- REQUIREMENTS.md UAT-02 marked `[x]`.

---

## Cross-Cutting Concerns

### Ordering of 58-01 vs 58-02

The two plans **could** run in parallel (independent test surfaces, independent docs). But they share a host machine — concurrent HA cluster creates will compete for Docker VM RAM. **Recommendation: sequential, 58-01 then 58-02.** Plan 58-01 creates a 6-container cluster (heavier); Plan 58-02 creates a 4-container cluster (lighter). Running them sequentially also produces a cleaner narrative ("Phase 47 closed at T+10min, Phase 51 closed at T+18min").

### Idempotency and re-runs

A failing UAT script does NOT delete the cluster. A second invocation of the script will:
- Hit `make build` — idempotent, fast.
- Pass the strings gate — same binary.
- Hit `./bin/kinder create cluster --name uat-58-01` — FAILS because cluster already exists.

The script needs a preamble step: detect existing cluster with the UAT name, prompt for delete (or auto-delete if `REUSE=no`), then proceed. The 53-04 plan does this implicitly by using a unique cluster name per run; we'll do the same.

### Commit cadence

Per kinder convention (gleaned from `git log` patterns in 47-06, 53-04, 57-01), one commit per logical step:
- `feat(58-01): add hack/uat-47-ha-smoke.sh`
- `chore(58-01): commit live UAT log evidence`
- `docs(58-01): flip 47-UAT.md issues to pass; mark UAT-01 complete`
- `docs(phase-58): mark Plan 58-01 complete; STATE+ROADMAP rolled`

Repeat for 58-02. Total: ~6-8 commits across both plans.

### Verifier expectations

The Phase 58 verifier (called by `/gsd-verify-work`) will look for:
- The two `.sh` files exist and are executable
- The two `.log` files exist
- 47-UAT.md has zero `result: issue` rows
- 51-UAT.md has a re-verification section dated post-v2.4
- REQUIREMENTS.md UAT-01 and UAT-02 are `[x]`
- ROADMAP Phase 58 row shows `2/2 Complete`
- STATE.md `progress.completed_phases` reflects 7 of 7

### What this phase explicitly does NOT change

- No Go source files
- No `pkg/` test files
- No manifests (cert-manager, envoy-gateway, metallb, etc.)
- No kinder-site content (only verifies it renders)
- No Makefile (only invokes existing `build` target)
- No GitHub Actions workflows (UAT remains manual for v2.4)
- No PROJECT.md (no architectural decision is logged by this phase)

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Docker Desktop on the developer's macOS host has ≥ 8 GB VM RAM available, sufficient for a 6-container HA cluster | Common Pitfalls A; Environment Availability | Plan 58-01 could fail at cluster create time on under-provisioned hosts. Mitigation: script prints `docker info` summary at preamble; developer can adjust Docker Desktop settings. |
| A2 | The K8s 1.36 guide page renders correctly because `astro dev` serves it; we are NOT also testing `astro build` (production build) | Plan 58-02 deliverable surface | If Astro production build differs from dev (e.g. due to Starlight v0.37 prerender quirk), production deploy could break despite UAT pass. Mitigation: kinder-site is deployed via Netlify which runs `astro build` on every commit; that's the canonical production-build gate, not Phase 58. |
| A3 | The 5 issue-row tests in 47-UAT.md are fully closeable by source fixes already on HEAD; no additional Go work is needed in Phase 58 | Existing State of 47-UAT.md table; Plan 58-01 acceptance criteria | If the developer runs the script and finds a regression (e.g. a test 12 that was supposed to be fixed at 47-06 is still broken), Phase 58 cannot proceed — the work returns to a code phase. Mitigation: 47-VERIFICATION.md is the authoritative pre-flight; it states 4/4 source-verified. Risk is LOW. |
| A4 | `make build` on a clean tree is fast enough (~10 s on Apple Silicon) that running it every script invocation is acceptable UX | Pattern 1; Pitfall 23 mechanics | If `make build` is unexpectedly slow (e.g. cold module cache, ~60 s), developers may skip it. Mitigation: Makefile target is incremental; second run is ~200 ms. Risk is LOW on warm caches. |
| A5 | 51-UAT.md Option A (augment) is preferred over Option B (replace) | Existing State of 51-UAT.md | If the verifier reads ROADMAP SC3 literally ("created") and demands a fresh file, Plan 58-02 verification could fail. Mitigation: Plan 58-02 explicitly documents the choice. The phase orchestrator can confirm with the user during planning. |
| A6 | macOS Homebrew Cask `/opt/homebrew/bin/kinder` symlink may still shadow `./bin/kinder` if the script ever uses bare `kinder`. Hardcoding `${REPO_ROOT}/bin/kinder` is the safe path. | Pitfall 23 mechanics; Pitfall F | If absent (Cask uninstalled), check is harmless. If present, hardcoding bypasses it. No risk. |
| A7 | The `hack/uat-*.log` files SHOULD be committed (not gitignored) as the canonical evidence artifact | Pitfall G; Plan 58-01/02 deliverable surface | If the verifier expects log files in `.planning/phases/58-…/` instead of `hack/`, the location is off. Mitigation: REQUIREMENTS.md UAT-01 says "evidence in `hack/uat-47-ha-smoke.sh` + log" — the path is locked. Risk is LOW. |
| A8 | The kindversion `versionPreRelease == ""` configuration is intentional and won't change in Phase 58 | Pitfall 23 mechanics | If a parallel phase changes versionPreRelease, the strings-marker technique still works but the rationale becomes obsolete. Risk is LOW (no other v2.4 work touches kindversion). |

**Mitigation summary:** Most assumptions have either explicit fallbacks or are LOW-risk by construction. The HIGHEST-risk item is A3 (source-fix completeness); the existing 47-VERIFICATION.md is the primary mitigation.

---

## Sources

### Primary (HIGH confidence)
- `.planning/phases/47-cluster-pause-resume/47-UAT.md` — current 14-test status, 5 issue rows, root causes
- `.planning/phases/47-cluster-pause-resume/47-VERIFICATION.md` — 5 human-verification items spelled out, including the strings-marker grep technique used here
- `.planning/phases/47-cluster-pause-resume/47-CONTEXT.md` — locked decisions on JSON schema, quorum-safe ordering, idempotency semantics
- `.planning/phases/51-upstream-sync-k8s-1-36/51-UAT.md` — current 3-test status, evidence blocks, side-observation log
- `.planning/phases/51-upstream-sync-k8s-1-36/51-VERIFICATION.md` — 2 human-verification items spelled out
- `.planning/ROADMAP.md` Phase 58 section — 4 success criteria
- `.planning/REQUIREMENTS.md` UAT-01 + UAT-02 — phrasing locks the script paths
- `.planning/STATE.md` — verified Phase 57 closed; Phase 58 is next; bin/kinder build mtime from May 12 08:32
- `.planning/research/PITFALLS.md` §Pitfall 23 — definitive stale-binary gate
- `pkg/internal/kindversion/version.go` — confirmed `versionPreRelease == ""` means commit hash is suppressed from version output
- `pkg/cluster/internal/loadbalancer/const.go:20` — confirmed `Image = "docker.io/envoyproxy/envoy:v1.36.2"`
- `pkg/internal/apis/config/validate.go:80-100` — confirmed IPVS-1.36 error message has 4 substrings
- `Makefile` — confirmed `build` target, `-ldflags` injection, idempotency
- `.planning/phases/53-addon-version-audit-bumps-sync-05/53-04-envoy-gateway-bump-PLAN.md` — exemplar for live UAT plan structure with checkpoint:human-verify gate
- `kinder-site/src/content/docs/guides/k8s-1-36-whats-new.md` — guide page exists; content frontmatter title "What's new in Kubernetes 1.36"
- `kinder-site/astro.config.mjs:83` — guide page registered in sidebar between multi-version-clusters and working-offline

### Secondary (MEDIUM confidence)
- 47-06-PLAN.md verification block — documents the macOS Homebrew Cask trap mechanism for the second time (cross-confirms the symlink pitfall)
- 53-04-SUMMARY.md UAT-SCRIPT NOTE 1 + NOTE 2 — empirical findings about hashicorp/http-echo CLI args and macOS curl-to-bridge-IP failures (informs Pitfall B)

### Tertiary (LOW confidence)
- None — all claims in this research are grounded in existing files on disk; nothing requires WebSearch or training-knowledge.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — every tool is already used by kinder developers; no new dependencies
- Architecture (script structure): HIGH — mirrors existing `hack/` script patterns and the 53-04 live-UAT exemplar
- Pitfalls: HIGH — Pitfall 23 is explicitly documented in `.planning/research/PITFALLS.md`; the binary-freshness mechanics were empirically verified in this research session (`bin/kinder version` output captured live)
- Doc-flip mechanics: HIGH — 47-UAT.md and 51-UAT.md schemas were read end-to-end

**Research date:** 2026-05-12
**Valid until:** 2026-06-12 (30 days) — the only sources of drift are upstream behavior changes (cert-manager / envoy-gateway / kind), but Phase 58 explicitly closes against the v2.4-frozen binary, so drift after v2.4 ship is not Phase 58's problem.

## RESEARCH COMPLETE
