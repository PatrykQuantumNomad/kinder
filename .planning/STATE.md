---
gsd_state_version: 1.0
milestone: v2.3
milestone_name: Inner Loop
status: executing
stopped_at: Phase 50 Plan 02 complete — RunDecode orchestrator + live collectors shipped. Plan 50-03 (cobra command) is next.
last_updated: "2026-05-07T10:50:24.588Z"
last_activity: 2026-05-07
progress:
  total_phases: 5
  completed_phases: 3
  total_plans: 21
  completed_plans: 19
  percent: 90
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-05-03 for v2.3 milestone start)

**Core value:** A single command gives developers a local Kubernetes cluster where LoadBalancer services, Gateway API routing, metrics, and dashboards all work without any manual setup.
**Current focus:** v2.3 Inner Loop — Phase 50: Runtime Error Decoder (Plans 01+02 complete). Plans 03/04/05 remain.

## Current Position

Phase: 50 of 51 — IN PROGRESS
Plan: 3 of 5 — COMPLETE
Status: Ready to execute
Last activity: 2026-05-07

Progress: [█████████░] 90%

## Performance Metrics

**Velocity:**

- v1.0: 12 plans, 8 phases, 1 day
- v1.1: 8 plans, 6 phases, 2 days
- v1.2: 4 phases, 1 day
- v1.3: 8 plans, 6 phases, ~5 hours
- v1.4: 13 plans, 5 phases, 2 days
- v1.5: 7 plans, 5 phases, 1 day
- v2.0: 7 plans, 3 phases, 2 days
- v2.1: 10 plans, 4 phases, 1 day
- v2.2: 14 plans, 5 phases, ~2.5 days

**By Phase:**

| Phase | Plan | Duration | Tasks | Files | Notes                                                                                                       |
| ----- | ---- | -------- | ----- | ----- | ----------------------------------------------------------------------------------------------------------- |
| 47    | 01   | ~4h      | 3     | 11    | TDD cycles for tasks 1+2 (RED→GREEN); 2 auto-fix deviations (lifecycle path move, dead nodeLister cleanup). |
| 47    | 02   | ~7m      | 2     | 4     | TDD RED→GREEN for both tasks; 1 deviation (parallel-wave conflict with 47-03 — resume_test.go/resume.go redeclared shared symbols, parked aside during test runs). |
| 47    | 03   | ~25m     | 2     | 4     | TDD RED→GREEN for both tasks (4 commits); 2 auto-fix deviations (lifecycle path correction from plan frontmatter, removed redundant NodeResult/nodeFetcher declarations after 47-02 landed first). |
| 47    | 04   | ~30m     | 2     | 7     | TDD RED→GREEN for both tasks (4 commits); 2 auto-fix deviations (lifecycle path correction pre-flagged by orchestrator, both registry tests in gpu_test.go + socket_test.go updated for 23→24 check count). LIFE-04 delivered; Phase 47 complete. |
| 47    | 05   | ~25m     | 2     | 4     | TDD RED→GREEN for both tasks (4 commits); 1 auto-fix deviation (test lookup false substring match on "ps" inside "--endpoints=https://"). Gap closure: crictl exec probe replaces unreachable which-etcdctl path in doctor check and pause.go. |
| 47    | 06   | ~40m     | 3     | 10    | TDD RED→GREEN (6 commits: 3 tasks × 2). No deviations. 4 source gaps fixed: cluster discovery filter, -a flag + running-CP bootstrap, DurationVar flags, positional cluster arg. 16 test changes across 5 test files. |
| 48    | 01   | ~7m      | 3     | 9     | TDD RED→GREEN (6 commits: 3 tasks × 2). 17 tests pass -race. stdlib-only: metadata schema, bundle sha256, SnapshotStore 0700, prune policies. ArchiveDigest in sidecar only (not in tarred metadata.json). |
| 48    | 02   | ~8m      | 2     | 11    | TDD RED→GREEN (4 commits: 2 tasks × 2). 16 new tests pass -race. CaptureEtcd/Images/PVs/Topology/Addons/KindConfig. ClassifyFn injection avoids lifecycle import. No circular deps. |
| 48    | 03   | ~11m     | 2     | 6     | TDD RED→GREEN (4 commits: 2 tasks × 2). 9 new tests pass -race. RestoreEtcd (HA same-token, manifest bracket, atomic swap), RestoreImages (LoadImageArchiveWithFallback wrapper), RestorePVs (nested-tar dispatch). No circular deps. |
| 48    | 04   | ~35m     | 3     | 8     | TDD RED→GREEN (6 commits: 3 tasks × 2). 27 new tests pass -race. snapshot.Create (defer-Resume), snapshot.Restore (pre-flight gauntlet, no-rollback), CheckCompatibility (3 aggregated sentinels), EnsureDiskSpace (syscall.Statfs + ensureFromStatfs pure fn). 1 auto-fix (exec.OutputLines call). |
| 48    | 05   | ~9m      | 3     | 13    | All tasks pass -race. 5 CLI subcommands (create/restore/list/show/prune) wired via fn-injection. tabwriter table, JSON/YAML, no --yes on restore, prune no-flag refusal. 2 auto-fixes (test arg prefix, JSON key case). |
| 48    | 06   | ~48m     | 3     | 2     | 5 integration tests under //go:build integration: ConfigMap round-trip + LIFE-08 metadata, K8s/topology/addon refusals, STATUS=corrupt. Task 3 human-verify: live UAT approved 2026-05-06 (make integration + manual smoke all green). Phase 48 COMPLETE. |
| 49    | 01   | ~6m      | 3     | 9     | TDD RED→GREEN × 3 tasks (6 commits). 19 -race tests pass. fsnotify v1.10.1 added (first new dep since v2.0; STATE.md authorized). pkg/internal/dev/ pure (zero project-internal imports). Parallel-wave park-aside reused for 49-02 RED test collisions (per 47-02 precedent). 0 deviations. |
| 49    | 02   | ~21m     | 3     | 8     | TDD RED→GREEN × 3 tasks (6 commits). 32 -race tests pass. 4 cycle-step primitives: BuildImage (V5 mitigation), LoadImagesIntoCluster (replicates kinder load images core via public APIs — no pkg/cmd/kind/load import), RolloutRestartAndWait (host kubectl + external kubeconfig per RESEARCH §3), WriteKubeconfigTemp (0600 V4 mitigation). Zero new module deps. 2 deviations: Rule 3 timing-only (held for 49-01 GREEN race), Rule 1 imageInspectID switched to io.Pipe for -race cleanliness. |
| 49    | 03   | ~9m      | 2     | 4     | TDD RED→GREEN × 2 tasks (4 commits). 22 new -race tests (7 cycle + 15 Run); pkg/internal/dev now totals 72 -race tests in 3.7s. runOneCycle (build/load/rollout %.1fs per-step timing per Phase 47/48 convention) + Run orchestrator (signal.NotifyContext SIGINT/SIGTERM teardown, kubeconfig once-at-startup not-per-cycle, banner SC1, Debounce SC3, --poll SC4 dispatch, EventSource test-injection). Zero new module deps. 1 deviation: Rule 1 removed plan body's post-cycle drain that would silently drop edits-during-cycle (plan's own test asserted 2 cycles when 5 events arrive during in-flight cycle 1). Locked Options struct ready for Plan 04 cobra wiring. |
| 49    | 04   | ~6m      | 2     | 3     | TDD RED→GREEN for Task 1 (3 commits total: test + feat for Task 1, single feat for Task 2 root.go registration). 17 -race tests pass in 1.5s for pkg/cmd/kind/dev/. Phase 49 SC1 delivered (kinder dev is a registered top-level command). 2 Rule 1 deviations: (1) lifecycle.ProviderBinaryName signature is func() string not (logger) (string, error) as plan body draft showed; (2) TestDevCmd_HelpListsCriticalFlags needed c.SetOut(buf) for --help capture in isolated test. AddCommand inserted between delete-export (the alphabetical slot) not delete-doctor (those aren't adjacent in current root.go layout). Zero new module deps. |
| 50    | 01   | ~15m     | 2     | 4     | TDD RED→GREEN × 2 tasks (4 commits). 11 -race tests pass. matchLines pure engine (sync.Map regex cache, first-match-wins). 16-entry Catalog: KUB-01..05, KADM-01..03, CTD-01..03, DOCK-01..03, ADDON-01..02. All 5 DecodeScope values covered (SC2/DIAG-02). DIAG-03/SC3 fields: ID/Match/Explanation/Fix non-empty on every entry. AutoFixable+AutoFix pre-declared zero for Plan 50-04. Zero new module deps. |
| 50    | 02   | ~6m      | 2     | 2     | TDD RED→GREEN (2 commits: 1 RED + 1 GREEN; both task sets committed together — single-file boundary). 12 -race tests pass. dockerLogsFn / k8sEventsFn / execCommand injection points; realDockerLogs (--since argv); realK8sEvents (type!=Normal default filter + includeNormal override + client-side LAST SEEN age filter per RESEARCH pitfall 4); RunDecode orchestrator (best-effort scan; locked decisions #2+#3 end-to-end). Zero new module deps. 1 TDD gate deviation (4 expected commits, 2 delivered — same-file constraint). |
| 50    | 04   | ~4m      | 2     | 4     | TDD RED→GREEN × 2 tasks (4 commits). 20 new -race tests pass. Three SafeMitigation factories (inotify-raise/coredns-restart/node-container-restart) with NeedsFix preconditions and fn-var injection; ApplyDecodeAutoFix dedup-by-Name + NeedsFix/NeedsRoot orchestration; PreviewDecodeAutoFix side-effect-free. Catalog wired: KUB-01/02 AutoFix=InotifyRaiseMitigation(); KADM-02/KUB-05 AutoFixable=true+AutoFix=nil. Zero new module deps. 0 deviations. |

*Updated after each plan completion*

## Accumulated Context

### Decisions

- v1.0–v2.2: See PROJECT.md Key Decisions table (full log moved there at v2.2 milestone completion)
- 2026-05-03: v2.3 theme chosen as "Inner Loop" — strongest user-value-per-day signal; defers pure tech debt and pure differentiator features to v2.4
- 2026-05-03: Phase 49 (kinder dev) may introduce `github.com/fsnotify/fsnotify` as first new module dep since v2.0; poll-based stdlib alternative acceptable to keep zero-dep streak
- 2026-05-03 (47-01): Shared lifecycle helpers package located at `pkg/internal/lifecycle/` (not `pkg/cluster/internal/lifecycle/` as the plan specified) — Go's internal-package rule blocks `pkg/cmd/kind/...` consumers from `pkg/cluster/internal/`. Plans 47-02/03/04 must update their `files_modified` lists accordingly.
- 2026-05-03 (47-01): JSON schema for `kinder get clusters --output json` migrated from `[]string` to `[]{name,status}` — intentional breaking change accepted per CONTEXT.md.
- 2026-05-03 (47-01): Pause/resume command stubs return `errors.New("not yet implemented")` (non-zero exit) rather than success — clearer signal in dev/CI than a silent stub.
- 2026-05-03 (47-02): `lifecycle.PauseResult` / `lifecycle.NodeResult` struct json: tags ARE the `--json` wire schema — Go API and CLI contract share a single source of truth.
- 2026-05-03 (47-02): Snapshot capture for HA pause is best-effort — failures log a warning and write `{leaderID:"", pauseTime:...}` rather than aborting the pause. Plan 47-04 readiness check MUST tolerate empty `leaderID`.
- 2026-05-03 (47-02): Plans 47-02 and 47-03 share the lifecycle package and were scheduled in parallel — 47-03's untracked `resume.go` redeclared `nodeFetcher` and `NodeResult` from my pause.go. Worked around with filesystem park-aside (no commits, no modifications). 47-03 needs to rebase onto `c7952992` and reuse `lifecycle.NodeResult` instead of redeclaring it.
- 2026-05-03 (47-03): Resume's readiness probe queries ALL nodes (kubectl with no `--selector`); diverges from create's waitforready (which only watches control-plane because workers may not exist yet during create). For resume every container exists and every node must be Ready before the user can run kubectl.
- 2026-05-03 (47-03): K8s 1.24 selector fallback (`control-plane` ↔ `master` label) retained inside `WaitForNodesReady` for completeness even though Resume itself doesn't use a selector — keeps the helper reusable by any future caller (plan 47-04 doctor check) that wants to filter by role.
- 2026-05-03 (47-03): Skip readiness probe entirely if any container failed to start (no point waiting for a known-incomplete cluster). Aggregated start errors are returned directly. Idempotent fast-path also skips probe when `ClusterStatus="Running"`.
- 2026-05-03 (47-03): Resolved 47-02's blocker by reusing `NodeResult` and `nodeFetcher` from `pause.go` (same package); resume.go references them by name without redeclaration. Pattern for future Wave-2 shared-package plans: the plan that lands first owns the declaration; the second plan references by name.
- 2026-05-03 (47-04): Doctor checks can be invoked cross-package via exported per-check constructors. `doctor.NewClusterResumeReadinessCheck()` is the first such export — lifecycle.Resume calls it inline. Verified pkg/internal/doctor does not import pkg/internal/lifecycle or pkg/cluster (no cycle); pkg/cluster/internal/create already imports doctor for ApplySafeMitigations, proving this direction works in production.
- 2026-05-03 (47-04): Orchestration extension via package-level hook var (`lifecycle.ResumeReadinessHook`) keeps the public ResumeOptions surface stable while enabling inter-phase injection. Default impl wraps doctor.NewClusterResumeReadinessCheck. Tests swap via t.Cleanup like other lifecycle injection points.
- 2026-05-03 (47-04): Resume's start logic refactored from a single monolithic loop into three explicit phases (LB → CP → readiness hook → workers) using a closure-based startNodes helper. Hook is gated by HA-only AND no-prior-failures AND hook-installed (three guards). Single-CP clusters incur zero overhead.
- 2026-05-03 (47-04): cluster-resume-readiness check NEVER returns fail — only ok/warn/skip. Matches CONTEXT.md "warn and continue" semantics: warnings flow through opts.Logger, Resume's exit code is independent of hook output. defaultResumeReadinessHook still defensively handles a fail status (logs as warn) in case future code paths add it.
- 2026-05-05 (47-05): etcdctl must be reached via `crictl exec <id>` into the etcd static-pod container — NOT via direct invocation in kindest/node rootfs. etcdctl ships only inside registry.k8s.io/etcd:VERSION. crictl is available on kindest/node (used by container runtime). Cert paths are identical because kubelet bind-mounts /etc/kubernetes/pki/etcd/ into the etcd container.
- 2026-05-05 (47-05): Test lookup conditions must use args[0] (exact subcommand match) not joined-string substring when args may contain URLs. "--endpoints=https://..." contains "ps" as a substring ("https" → "tps") — substring match caused false match in test fakes.
- 2026-05-05 (47-06): Bare integer --wait=600/--timeout=30 intentionally rejected after DurationVar migration; no install base for Phase 47 CLI flags; use 600s/30s/5m syntax.
- 2026-05-05 (47-06): All-stopped HA cluster returns warn not skip from clusterResumeReadinessCheck — completely stopped HA cluster is real degradation with actionable advice, not "check not applicable".
- 2026-05-05 (47-06): realInspectState inlines lifecycle.ContainerState to avoid doctor→lifecycle import cycle; doctor must never import lifecycle (lifecycle/resume.go imports doctor).
- 2026-05-05 (47-06): listNodes nil-check injection in nodes.go: var is nil by default; production code nil-guards it and calls provider.ListNodes; test sets it to capture resolved name.
- 2026-05-06 (48-01): ArchiveDigest inside tarred metadata.json is intentionally left empty — including it would require knowing the archive digest before writing the archive (recursive). Sidecar .sha256 is the single source of truth for archive integrity; SnapshotStore.List reads sidecar for Status.
- 2026-05-06 (48-01): ErrMissingSidecar is a distinct sentinel from ErrCorruptArchive — missing sidecar is an operational error (interrupted write), not a data-integrity failure. Plan 05 CLI should surface both distinctly.
- 2026-05-06 (48-01): bundleReader is in-memory (all entries loaded on OpenBundle) — avoids seeking on non-seekable gzip streams; acceptable for restore because large entries extracted to temp files anyway.
- 2026-05-06 (48-01): PrunePlan union semantics — a snapshot is in the deletion set if ANY active policy marks it. Zero-value Policy fields are inactive (0 = no deletions for that field). CLI `kinder snapshot prune` must enforce at least one flag before calling PrunePlan.
- 2026-05-06 (48-01): SnapshotStore.List performs full VerifyBundle (re-hash) for accurate Status. Fast-path (Status='unknown' without re-hash) deferred to Plan 05 via StatusFast/StatusFull mode flag documented inline in store.go.
- 2026-05-06 (48-02): etcdctlAuthArgs duplicated inline in snapshot/etcd.go — no doctor package import avoids circular dependency; TODO references original location for future refactor.
- 2026-05-06 (48-02): ClassifyFn defined in snapshot package (not lifecycle) — Plan 04 injects lifecycle.ClassifyNodes; avoids circular import risk since lifecycle imports snapshot via Plan 04.
- 2026-05-06 (48-02): CapturePVs uses single outer tar with per-node nested entries (<nodeName>/local-path-provisioner.tar) — RESEARCH Q8 resolution: simpler layout, matches bundleReader expectations.
- 2026-05-06 (48-02): ReconstructKindConfig uses string builder (no v1alpha4 API types) — keeps snapshot pkg free of cluster API imports; output is documentation artifact, not for programmatic re-creation.
- 2026-05-06 (48-02): AddonRegistry omits localRegistry — it is a host-level Docker container not discoverable via kubectl get deployment; topology nodeImage covers it implicitly.
- 2026-05-06 (48-03): etcdctl invoked directly on kindest/node PATH (not via crictl exec) for snapshot restore — restore runs AFTER manifest removed so etcd container is gone; etcdctl must be on node PATH. Confirmed feasible per RESEARCH OQ-6.
- 2026-05-06 (48-03): etcdManifestSettleDelay is a package-level var (not EtcdRestoreOptions field) — test injection via assignment, keeps public API clean.
- 2026-05-06 (48-03): Manifest restore is a deferred call inside restoreSingleCP — guarantees rollback on ALL exit paths; runs after rm tmp snap in the success path.
- 2026-05-06 (48-04): Create defers Resume on all exit paths (read-only capture; no cluster mutation); Restore does NOT defer Resume (mutation path; CONTEXT.md no-rollback locked decision). Post-pause restore failures include recovery hint: "run `kinder resume <cluster>` to restart".
- 2026-05-06 (48-04): CheckCompatibility aggregates all three dimension violations (K8s+topology+addon) via kinderrors.NewAggregate — errors.Is() can drill into wrapped aggregate for each sentinel independently.
- 2026-05-06 (48-04): ErrClusterNotRunning added as new sentinel for etcd reachability pre-flight check in Restore — signals that the cluster must be running before restore can proceed.
- 2026-05-06 (48-04): Create disk threshold fixed at 8GiB — cannot estimate image size before listing (chicken-and-egg); lifecycle.ClassifyNodes and lifecycle.Pause/Resume are injected via nil-defaulted CreateOptions/RestoreOptions fields (matches Phase 47 test injection pattern).
- 2026-05-06 (48-05): restore has NO --yes flag — CONTEXT.md locked; hard overwrite is intentional signaling to the caller that this is destructive and non-interactive.
- 2026-05-06 (48-05): prune enforces at least one policy flag with error listing all 3 (--keep-last/--older-than/--max-size) — CONTEXT.md locked; never delete on naked invocation.
- 2026-05-06 (48-05): show uses vertical key/value layout — planner discretion for addon map readability; --output json/yaml available for scripted use.
- 2026-05-06 (48-05): ADDONS column truncation threshold = 50 runes for 120-col terminal; --no-trunc bypasses.
- 2026-05-06 (48-05): parseSize uses base-2 multipliers (1K=1024), case-insensitive, accepts KiB/MiB/GiB/TiB variants; no custom 'd' duration suffix (rely on Go ParseDuration h/m/s per Research OQ-4).
- 2026-05-06 (48-05): pruneStoreFns struct bundles list+delete fn injection together — cleaner test setup than two separate package vars.
- 2026-05-06 (49-01): fsnotify v1.10.1 added as direct dep — first new module dep since v2.0; pre-authorized by STATE.md 2026-05-03 decision. Transitive golang.org/x/sys at pre-existing v0.41.0 dominates fsnotify's v0.13.0 floor — go.sum gains 2 lines (fsnotify itself only).
- 2026-05-06 (49-01): Watcher channel cap=64, poller cap=1, debouncer cap=1. Watcher needs burst headroom (IDE atomic-save = 5–50 events/tick); poller's tick rate-limits emits intrinsically; debouncer enforces "boolean semantics" — consumer only needs to know "something changed since last drain."
- 2026-05-06 (49-01): Debouncer uses LEADING-trigger (first event arms timer, subsequent events absorbed) NOT trailing-trigger (last event resets timer). File-save bursts fire over <100ms — we want build-load-rollout cycle starting ASAP, not waiting for editor to finish swap-rename. Trailing-trigger is for fast-typing UIs.
- 2026-05-06 (49-01): fsnotify.ErrEventOverflow handler logs warn + emits a SYNTHETIC event into the output channel. Heavy builds writing thousands of files to _output/ commonly overflow inotify queue; silently dropping the trigger would be a UX disaster — synthesis ensures the cycle still fires.
- 2026-05-06 (49-01): Parallel-wave shared-package collision handled via filesystem park-aside (per 47-02 precedent). 49-02 dropped its RED test files into pkg/internal/dev/ during my Task 2 RED run, breaking my package compilation. Resolution: temporarily moved 49-02's untracked test files to /tmp during my test runs, restored after. NO 49-02 files were modified or committed by me.
- 2026-05-06 (49-02): LoadImagesIntoCluster replicates the `kinder load images` core via public APIs (RESEARCH §1 Option A) — does NOT import `pkg/cmd/kind/load`. The unexported `runE` takes an unexported flagpole and writes structured output to streams.Out; reusing it would require widening export surface (scope creep). Re-implementing the ~30-line pipeline against `nodeutils.ImageTags`/`ReTagImage`/`LoadImageArchiveWithFallback` + `errors.UntilErrorConcurrent` is the chosen path.
- 2026-05-06 (49-02): Single-image LoadOptions API (not multi-image []string like upstream runE). `kinder dev` only ever loads ONE image per cycle (the freshly-built one); multi-image surface adds complexity for no gain.
- 2026-05-06 (49-02): Rollout uses host `pkg/exec.CommandContext` against host kubectl with `--kubeconfig=<external>` (RESEARCH §3) — NOT `node.CommandContext` (the in-cluster system-addon pattern used by corednstuning). User Deployments are user-managed; rollout must run on the host where the user's existing kubectl context lives.
- 2026-05-06 (49-02): Function-var indirection over interface threading for hard-to-fake dependencies (`nodeLister`, `kubeconfigGetter`, `imageTagsFn`, `reTagFn`, `imageInspectID`, `devCmder`). Production paths default to the real implementation; tests swap via t.Cleanup. Threading interfaces through every signature would have widened the API for no test-injection gain — matches `pkg/internal/lifecycle/state.go` precedent.
- 2026-05-06 (49-02): Kubeconfig tempfile chmod to 0600 BEFORE writing (V4 mitigation). os.CreateTemp creates 0600 on Unix already, but explicit Chmod is defensive against unusual umask configurations. os.CreateTemp (not os.WriteFile) for unique-path concurrency-safety across multiple kinder dev invocations.
- 2026-05-06 (49-02): imageInspectID inlines a stripped-down OutputLines pipeline (io.Pipe + goroutine + manual line splitter) instead of calling exec.OutputLines directly — `-race` clean across the test fakes that script per-call stdout. Production behavior is identical (single-line stdout from `<binary> image inspect -f {{ .Id }} <ref>`).
- 2026-05-06 (49-02): Parallel-execution timing race (Rule 3 deviation, no code change): 49-01's RED commit `test(49-01): add failing tests for StartPoller` referenced an undefined `StartPoller` while my Task 1 GREEN gate was running. Held position; 49-01 advanced to GREEN within the same minute (`feat(49-01): implement stdlib StartPoller fallback`). The plan's concurrency note explicitly tolerates this hazard. Re-ran my GREEN gate → pass. Confirms the documented coordination model on the same `main` branch is workable but expects brief wait windows when one plan races ahead of another's GREEN.
- 2026-05-06 (49-03): Removed the post-cycle drain `select { case <-cycles: default: }` that the plan body suggested. The plan's own `TestRun_ConcurrentCyclesPrevented` test asserts that 5 edits arriving DURING an in-flight cycle should produce a follow-up cycle (build called EXACTLY twice). The drain would silently drop the queued event, defeating hot-reload UX. With Debounce(cap=1) + the serial outer for-select arms `<-ctx.Done()` and `<-cycles`, overlap is structurally impossible regardless of drain. RESEARCH common pitfall 3 is about overlap PREVENTION; the plan's drain reasoning misread it as event-dropping. Test (user-facing behavior) supersedes plan body (implementation guidance) when they conflict.
- 2026-05-06 (49-03): Three-tier injection layering for orchestrator tests: (1) `BuildImageFn` / `RolloutFn` from Plan 49-02 for shell-out primitives, (2) `loadImagesFn` (NEW in 49-03) wraps `LoadImagesIntoCluster` so cycle tests stub the full load step without setting up a fake `*cluster.Provider`, (3) `kubeconfigGetter` from Plan 49-02 for provider-side calls. Each tier is testable in isolation; combinators compose cleanly. `LoadOptions.ImageLoaderFn` (per-node injection) remains for `load_test.go` covering load internals.
- 2026-05-06 (49-03): `runOneCycle` substitutes `io.Discard` when `streams.Out` is nil (defensive guard). Plan 04's CLI wiring may produce partially-initialized Options; a panic on the first Fprintf would be a hostile failure mode for an inner-loop developer tool.
- 2026-05-06 (49-03): Default `ExitOnFirstError = false` (continue on cycle error, log to ErrOut). A flaky build mid-iteration should not auto-exit `kinder dev` — the user already sees the cycle error in their terminal. Eventual return value is the FIRST cycle error observed (when ctx-cancel terminates the loop). `ExitOnFirstError=true` remains internal for tests and future strict-mode flags.
- 2026-05-06 (49-03): EventSource `<-chan struct{}` test injection on Options struct lets `dev_test.go` drive the full watch loop deterministically without spinning a real fsnotify watcher. Production = nil (start the real watcher); tests = synthetic channel. Pattern composes with the per-cycle BuildImageFn / loadImagesFn / RolloutFn fakes for exhaustive orchestrator-level coverage that's still -race clean.
- 2026-05-06 (49-04): lifecycle.ProviderBinaryName signature is func() string (no logger arg, no error return) — adjusted resolveBinaryName var accordingly; runE checks binary == "" and surfaces clear error referencing kinder doctor. Plan body's pseudocode showed (logger) (string, error) which was incorrect. Pattern matches existing callers in get/clusters/clusters.go:85 and status/status.go:107.
- 2026-05-06 (49-04): kinder dev cobra command shipped with three injection points (runFn / resolveClusterName / resolveBinaryName), 17 unit tests, 10 LOCKED flags. resolveClusterName is (args, explicit) — explicit-aware closure (vs pause.go's args-only) because dev uses --name flag, not positional cluster arg. AddCommand inserted between delete and export (the alphabetical slot in root.go's existing layout, NOT strictly between delete-doctor as plan body said since those are not adjacent in current AddCommand layout).
- 2026-05-06 (49-04): --json flag is reserved/documented as 'currently unused' in --help rather than removed. Plan 49-03's Run does not emit JSON-structured cycle output; keeping the flag now avoids a future breaking change when JSON output is wired and mirrors pause.go's --json shape.
- 2026-05-07 (50-01): matchLines uses sync.Map regex cache keyed by pattern string — each unique regex compiles once across process lifetime; -race clean without mutex contention.
- 2026-05-07 (50-01): KADM-02 catalog entry uses single substring "coredns" rather than two-field compound match — multi-match logic is outside matchLines first-match-wins design; explanation note directs user to check for Pending state.
- 2026-05-07 (50-01): AutoFixable=false and AutoFix=nil for all 16 Catalog entries — Plan 50-04 populates KUB-01, KUB-02, KADM-02, KUB-05 auto-fix fields by editing only decode_catalog.go; decode.go struct definition is frozen.
- 2026-05-07 (50-02): execCommand var injection (var execCommand = exec.Command) enables test-time fakes without interface threading — same lifecycle.defaultCmder pattern used throughout project.
- 2026-05-07 (50-02): filterEventsByAge passes through rows with unparseable LAST SEEN (over-include vs silently drop) — conservative policy aligned with RESEARCH pitfall 4 recommendation.
- 2026-05-07 (50-02): RunDecode errors from individual sources are non-fatal — best-effort scan with V(1) logging; only returns error on caller misuse (empty inputs).
- 2026-05-07 (50-02): sinceStr = opts.Since.String() — Docker accepts "30m0s"; deterministic string for test assertions; NOT stripped to "30m" form.
- 2026-05-07 (50-02): TDD gate delivered as 2 commits instead of 4 — both task test groups in one file, both implementations in one file; RED gate satisfied (compile failure on undefined symbols for all 12 tests).
- 2026-05-07 (50-04): geteuidFn injected as fn-var for NeedsRoot test without requiring real OS privilege change — consistent with project's t.Cleanup-swap pattern for OS-level primitives.
- 2026-05-07 (50-04): realInspectStateAuto intentionally duplicates resumereadiness.go's realInspectState body to avoid doctor→lifecycle import cycle; no shared helper to avoid duplication because the cycle is a hard constraint (lifecycle imports doctor).
- 2026-05-07 (50-04): execCommand referenced from decode_collectors.go (50-02) without redeclaration — 50-02 was on disk before 50-04 ran; no filesystem park-aside needed. Single-file ownership pattern confirmed: Plan 50-02 owns execCommand.
- 2026-05-07 (50-04): KUB-05 node name extracted from match.Source "docker-logs:<node>" prefix — k8s-events source returns nil from mitigationFor (no node name extractable, skip is correct).
- 2026-05-07 (50-04): ApplyDecodeAutoFix dedupes by sm.Name so KUB-01+KUB-02 both embedding inotify-raise fires the mitigation exactly once per run (dedup before NeedsFix/NeedsRoot checks).

### Pending Todos

Three issues uncovered during phase 47 live UAT — all pre-existing or cosmetic, NOT 47 regressions; may be addressed in a future phase or as opportunistic fixes:

1. Etcd peer TLS certs are bound to original Docker container IPs; pause/resume can reassign IPs and break peer connectivity. Affects HA pause/resume usefulness in production. Candidate for phase 48 (snapshot/restore) consideration or a dedicated kinder fix.
2. `cluster-node-skew` doctor check tries to `docker exec <lb-container> cat /kind/version` and warns when the LB container doesn't have it — pre-existing skew-check bug, not 47-06 territory.
3. `cluster-resume-readiness` reason text dumps raw etcdctl error output when partial-failure JSON is available; could parse `[{"endpoint":...,"health":...}]` to produce "1/3 healthy, quorum at risk". Cosmetic — semantics (warn vs skip vs fail) are correct.

### Blockers/Concerns

None. Phase 47 fully delivers LIFE-01..LIFE-04. Phase 48 fully delivers snapshot/restore. Phase 49 source-level COMPLETE: Plan 04's kinder dev cobra command + root.go registration is landed. Phase 50 Plans 01+02+04 complete: matchLines engine + 16-entry Catalog (Plan 01); live collectors + RunDecode orchestrator (Plan 02); auto-fix whitelist + ApplyDecodeAutoFix/PreviewDecodeAutoFix (Plan 04). Plans 50-03 and 50-05 remain.

## Session Continuity

Last session: 2026-05-07T10:48:34Z
Stopped at: Phase 50 Plan 04 complete — auto-fix whitelist (inotify-raise/coredns-restart/node-container-restart) + ApplyDecodeAutoFix/PreviewDecodeAutoFix + Catalog wiring shipped. Plan 50-03 (cobra command) is next.
Resume file: None
