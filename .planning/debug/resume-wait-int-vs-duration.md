---
status: diagnosed
trigger: "kinder resume verify47 --wait 5m fails with strconv.ParseInt error; --wait flag registered as int (seconds) but Go users expect time.Duration syntax"
created: 2026-05-05T00:00:00Z
updated: 2026-05-05T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED — --wait and --timeout flags are registered as IntVar (seconds) in resume.go and pause.go. The closest precedent in this codebase, `kinder create cluster --wait`, uses DurationVar. The 47-RESEARCH.md Open Question #1 explicitly recommended matching that precedent (default "5m"). The plan (47-01-PLAN.md line 311, 47-03-PLAN.md lines 102-105) drifted to IntVar without rationale, contradicting the research recommendation. Internal lifecycle.ResumeOptions/PauseOptions already use time.Duration, so the int→Duration conversion is purely a CLI-layer artifact.
test: complete
expecting: complete
next_action: return diagnosis (goal: find_root_cause_only)

## Symptoms

expected: `kinder resume <name> --wait 5m` waits up to 5 minutes for all nodes Ready. Same for `--timeout`.
actual: `kinder resume verify47 --wait 5m` → `ERROR: invalid argument "5m" for "--wait" flag: strconv.ParseInt: parsing "5m": invalid syntax`
errors: ERROR: invalid argument "5m" for "--wait" flag: strconv.ParseInt: parsing "5m": invalid syntax
reproduction: Test 9 in .planning/phases/47-cluster-pause-resume/47-UAT.md. Run `kinder resume <existing-cluster> --wait 5m`.
started: Discovered 2026-05-05 during UAT testing of Phase 47 deliverables.

## Eliminated

(none yet)

## Evidence

- timestamp: 2026-05-05
  checked: pkg/cmd/kind/resume/resume.go lines 82-83
  found: `c.Flags().IntVar(&flags.Timeout, "timeout", 30, "graceful start timeout in seconds")` and `c.Flags().IntVar(&flags.WaitSecs, "wait", 300, "max seconds to wait for all nodes Ready")`. flagpole struct fields are `Timeout int` and `WaitSecs int`. Conversion to Duration happens at lines 111-112: `time.Duration(flags.Timeout) * time.Second`.
  implication: pflag's IntVar uses strconv.ParseInt — exactly the error in the symptom. "5m" is rejected because it's not parseable as int. Direct cause confirmed.

- timestamp: 2026-05-05
  checked: pkg/cmd/kind/pause/pause.go line 79
  found: `c.Flags().IntVar(&flags.Timeout, "timeout", 30, "graceful stop timeout in seconds before SIGKILL")`. Same pattern as resume.
  implication: pause has the same defect (--timeout would also reject "5m"), even though no symptom for pause was reported in this UAT.

- timestamp: 2026-05-05
  checked: pkg/cmd/kind/create/cluster/createcluster.go lines 84-89
  found: `cmd.Flags().DurationVar(&flags.Wait, "wait", time.Duration(0), "wait for control plane node to be ready (default 0s)")`. Uses Go duration string parsing.
  implication: The closest in-repo precedent for a `--wait` flag uses DurationVar. resume/pause diverged from this convention without justification.

- timestamp: 2026-05-05
  checked: pkg/internal/lifecycle/resume.go lines 40-43 and pkg/internal/lifecycle/pause.go line 40
  found: Internal options struct fields are `StartTimeout time.Duration`, `WaitTimeout time.Duration`, `Timeout time.Duration`. Defaults set inside lifecycle as `30 * time.Second` and `5 * time.Minute`.
  implication: Lifecycle layer is correct and idiomatic. The CLI is the only layer doing IntVar→Duration translation. Switching to DurationVar at CLI removes the conversion entirely (binding directly to time.Duration fields).

- timestamp: 2026-05-05
  checked: .planning/phases/47-cluster-pause-resume/47-RESEARCH.md Open Question #1 (lines 588-593)
  found: Research explicitly recommended `--wait` "matching the `--wait` flag precedent from `kinder create cluster`" with "default `5m`". The "5m" string in research = Go duration syntax = DurationVar.
  implication: The research design intent was DurationVar with duration-string defaults. The plan files (47-01-PLAN.md line 311, 47-03-PLAN.md lines 102-105) downgraded this to IntVar without recorded rationale. This is a planning regression, not an intentional UX choice.

- timestamp: 2026-05-05
  checked: .planning/phases/47-cluster-pause-resume/47-CONTEXT.md (decisions block)
  found: No decision recorded preferring int-seconds over duration strings. CONTEXT line 26 just says "30s default" — agnostic on flag type.
  implication: No design decision exists to defend the IntVar choice. It is an implementation accident, not "working as designed."

- timestamp: 2026-05-05
  checked: pkg/cmd/kind/resume/resume_test.go lines 226-282
  found: Existing tests use `c.SetArgs([]string{"--wait=600"})` and `--wait=-1`. They lock in the int form by example but do not assert int-vs-duration policy.
  implication: Tests will need updating when fixing — current test values "600" and "-1" become "600s" / "10m" / "-1s". Also: negative-value rejection logic in runE (lines 89-94) needs to switch from `< 0` int comparison to `< 0` Duration comparison (still valid since time.Duration is int64 nanoseconds), or use `< time.Duration(0)`.

## Resolution

root_cause: |
  pkg/cmd/kind/resume/resume.go (lines 82-83) and pkg/cmd/kind/pause/pause.go (line 79) register `--wait` and `--timeout` with cobra/pflag's `IntVar`, which parses values via `strconv.ParseInt`. When a user passes `5m` (a Go time.Duration string), pflag rejects it with the exact error in the symptom: `invalid argument "5m" for "--wait" flag: strconv.ParseInt: parsing "5m": invalid syntax`. This is a CLI-layer ergonomics regression: (1) the closest in-repo precedent — `kinder create cluster --wait` — uses `DurationVar`; (2) Phase 47's own RESEARCH.md (Open Question #1) explicitly recommended matching that precedent with default `5m`; (3) the internal `lifecycle.ResumeOptions`/`PauseOptions` already use `time.Duration` fields, so IntVar adds an unnecessary `time.Duration(flags.X) * time.Second` conversion at the CLI boundary. The IntVar choice was introduced in 47-01-PLAN.md without documented rationale, contradicting the research recommendation. This is a UX bug, not working-as-designed.
fix: |
  Switch `--wait` and `--timeout` flags to `DurationVar` in both pkg/cmd/kind/resume/resume.go and pkg/cmd/kind/pause/pause.go. Bind them directly to `time.Duration` fields on flagpole (eliminating WaitSecs/Timeout int + manual `* time.Second` conversion). Defaults become `30 * time.Second` and `5 * time.Minute` (or `300 * time.Second` to preserve current default exactly). Update negative-value validation to compare against `time.Duration(0)`. Update resume_test.go args from `"--wait=600"` to `"--wait=600s"` (or `"10m"`) and `"--wait=-1"` to `"--wait=-1s"`. Backward compatibility note: bare integers (e.g. `--wait 300`) will NO LONGER work with DurationVar — Go duration parsing requires a unit suffix. If we want to accept both, we'd need a custom flag type. Recommendation: just adopt DurationVar (consistency with `kinder create cluster` is more important than accepting bare ints, and the flag is brand-new in Phase 47 so there's no install base to break).
verification: pending (diagnose-only mode)
files_changed: []
