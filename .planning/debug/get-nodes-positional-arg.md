---
status: diagnosed
trigger: "kinder get nodes verify47 returns 'unknown command verify47' — Phase 47 SUMMARY described positional-arg form, but cobra rejects it"
created: 2026-05-05
updated: 2026-05-05
---

## Current Focus

hypothesis: `kinder get nodes` only accepts `--name <cluster>` (flag form). Positional argument was never added. UAT description (and likely 47-01-SUMMARY) used the wrong invocation form, conflating it with `kinder pause/resume` which DO accept positional args. The "real container state" feature itself works — it just renders via `kinder get nodes --name verify47`.
test: Read pkg/cmd/kind/get/nodes/nodes.go, pkg/cmd/kind/get/clusters/clusters.go, pkg/cmd/kind/pause/pause.go, pkg/cmd/kind/resume/resume.go and compare cobra `Args:` directives. Check git log on nodes.go for any 47-era change to argument handling.
expecting: nodes.go uses `cobra.NoArgs` and only `--name` flag; pause/resume use `cobra.MaximumNArgs(1)`. If so, this is a UX/docs gap, not a regression.
next_action: Diagnosis-only mode — return ROOT CAUSE FOUND to caller.

## Symptoms

expected: `kinder get nodes <cluster-name>` accepts the cluster name as a positional argument and prints per-node status with STATUS reflecting real container state (Ready/Stopped/Paused/Unknown). Phase 47-01's fix replaced hardcoded "Ready".
actual: `kinder get nodes verify47` returns `ERROR: unknown command "verify47" for "kinder get nodes"`. `kinder get clusters` works and shows the new Status column. User did not test `kinder get nodes --name verify47`.
errors: ERROR: unknown command "verify47" for "kinder get nodes"
reproduction: Test 3 in .planning/phases/47-cluster-pause-resume/47-UAT.md. Run `kinder get nodes verify47` against any existing kinder cluster.
started: Discovered during UAT 2026-05-05. Predates Phase 47 — `cobra.NoArgs` constraint has been in nodes.go for years.

## Eliminated

- hypothesis: Phase 47-01 introduced/regressed the positional-arg path
  evidence: git log shows the only Phase 47 commit on this file is 2b69b481, which only replaced the hardcoded "Ready" status with `lifecycle.ContainerState`. It did NOT touch the cobra `Args:` declaration. The `cobra.NoArgs` constraint is from the original kind upstream; positional args were never supported on `get nodes`.
  timestamp: 2026-05-05

- hypothesis: There is a hidden subcommand named after the cluster
  evidence: cobra's "unknown command 'X' for parent" error message is what cobra produces when `Args: cobra.NoArgs` is set and an extra arg is supplied (cobra interprets unexpected positional args as subcommand candidates). nodes.go has no subcommands registered. So the message is misleading — it is really "no positional args allowed."
  timestamp: 2026-05-05

## Evidence

- timestamp: 2026-05-05
  checked: pkg/cmd/kind/get/nodes/nodes.go lines 60-93 (NewCommand)
  found: Line 63 `Args:  cobra.NoArgs`. Line 72-78 declares `--name`/`-n` flag with default `cluster.DefaultName`. Line 79-85 declares `--all-clusters`/`-A`. There is NO positional argument handling. RunE ignores the `args []string` parameter entirely (line 67: `RunE: func(cmd *cobra.Command, args []string) error { ... return runE(logger, streams, flags) }` — args never read).
  implication: The command only accepts cluster name via `-n verify47` or `--name verify47`. A bare positional `verify47` is rejected by cobra at parse time.

- timestamp: 2026-05-05
  checked: pkg/cmd/kind/get/clusters/clusters.go lines 52-66 (NewCommand)
  found: Line 55 `Args: cobra.NoArgs`. Only `--output` flag declared. No `--name` (lists ALL clusters by definition). Convention matches `get nodes` for the no-positional-args part.
  implication: `get clusters` works without args because it inherently lists all. `get nodes` defaults to `cluster.DefaultName` ("kind") when no `-n` is provided — that's why `kinder get nodes` (no args) doesn't error but shows nodes for the default-named cluster only.

- timestamp: 2026-05-05
  checked: pkg/cmd/kind/pause/pause.go lines 65-89 and pkg/cmd/kind/resume/resume.go lines 67-96
  found: Both use `Args: cobra.MaximumNArgs(1)` and `Use: "pause [cluster-name]"` / `Use: "resume [cluster-name]"`. They resolve cluster name via `lifecycle.ResolveClusterName(args, provider)` — explicit positional-arg support. Phase 47 commands diverge from the `get` convention.
  implication: Within Phase 47's own surface area, `kinder pause verify47` and `kinder resume verify47` work. So users (and the SUMMARY author) reasonably expect `kinder get nodes verify47` to also work. It doesn't. This is a UX inconsistency, not a regression.

- timestamp: 2026-05-05
  checked: git log --all -- pkg/cmd/kind/get/nodes/nodes.go
  found: Only Phase 47 commit is 2b69b481 ("feat(47-01): implement kinder status command, add Status column to get clusters, replace hardcoded Ready in get nodes"). Pre-47 commits all relate to upstream kind/kinder: column additions (VERSION/IMAGE/SKEW in 42-02), JSON output (29-02), `-n` short flag, KIND_CLUSTER_NAME support, etc. No commit ever added positional-arg support.
  implication: Positional arg was never supported. The "regression" framing is wrong.

- timestamp: 2026-05-05
  checked: .planning/phases/47-cluster-pause-resume/47-UAT.md lines 23-26
  found: Test 3 explicitly states `expected: kinder get nodes <cluster-name>` — the UAT description itself is wrong. The user followed the documented (incorrect) form.
  implication: The bug is in the documentation/UAT description, not the binary. The actual underlying feature (real container state in Status column) is intact and reachable via `kinder get nodes --name verify47` or `kinder get nodes -n verify47` or `kinder get nodes --all-clusters`.

## Resolution

root_cause: |
  `kinder get nodes` is declared with `cobra.NoArgs` (pkg/cmd/kind/get/nodes/nodes.go:63) and accepts cluster name only via the `--name`/`-n` flag (default "kind"). Positional argument support was never implemented on this command — the `cobra.NoArgs` constraint has been in place since long before Phase 47. Cobra produces the misleading `unknown command "verify47"` error because it interprets unexpected positional args as subcommand candidates.

  Phase 47-01's commit (2b69b481) only changed the Status column derivation from a hardcoded "Ready" literal to `lifecycle.ContainerState(...)` mapping. It did not modify the command's argument handling. Therefore:

  - This is NOT a regression. Positional arg never worked.
  - The Status-column feature itself works correctly — it just has to be invoked as `kinder get nodes --name verify47` or `kinder get nodes -n verify47` (or `--all-clusters` to see every cluster's nodes).
  - The 47-UAT.md description (and presumably 47-01-SUMMARY.md) wrote `kinder get nodes <cluster-name>` because Phase 47's own new commands (`kinder pause [cluster-name]`, `kinder resume [cluster-name]`) DO accept positional args via `cobra.MaximumNArgs(1)` + `lifecycle.ResolveClusterName`. The author conflated the conventions.

  Classification: documentation/UX-gap bug. Two valid framings:
    (a) **Docs error** — UAT/SUMMARY should describe the actual flag-form invocation.
    (b) **Legitimate UX gap** — `get nodes` should match the `pause/resume` convention and accept a positional cluster name (in addition to `--name` for backward compat). This would be a small, isolated change: relax `cobra.NoArgs` to `cobra.MaximumNArgs(1)`, and in RunE prefer `args[0]` over `flags.Name` when both/either are supplied (mirroring `lifecycle.ResolveClusterName`).
fix: (not applied — diagnose-only mode)
verification: (not applicable — no fix applied)
files_changed: []
