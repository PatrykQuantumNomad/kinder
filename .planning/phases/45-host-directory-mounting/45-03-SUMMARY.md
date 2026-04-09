---
phase: 45-host-directory-mounting
plan: "03"
subsystem: documentation
tags: [docs, guide, mounting, kubernetes, hostpath, persistentvolume]
requires:
  - 45-01-SUMMARY.md
  - 45-02-SUMMARY.md
provides:
  - MOUNT-04: Complete guide documenting two-hop host directory mounting pattern
affects:
  - kinder-site/src/content/docs/guides/
tech-stack:
  added: []
  patterns:
    - Astro Starlight admonition syntax (:::tip, :::note, :::caution)
    - Step-by-step guide structure matching existing guides
key-files:
  created:
    - kinder-site/src/content/docs/guides/host-directory-mounting.md
  modified: []
key-decisions:
  - Used absolute path in cluster config YAML example with comment to replace placeholder (tilde expansion is shell-only, not kind config aware)
  - Included Windows section to complete platform notes coverage even though plan only explicitly required macOS
  - Placed two-hop diagram immediately after verification step so the "How it works" section reads as a post-success explanation
metrics:
  duration: "1m22s"
  started: "2026-04-09T16:36:32Z"
  completed: "2026-04-09T16:37:54Z"
  tasks: 1
  files_created: 1
  files_modified: 0
---

# Phase 45 Plan 03: Host Directory Mounting Guide Summary

Complete Astro Starlight guide documenting the two-hop host-to-pod mount pattern (extraMounts -> hostPath PV) with copy-paste YAML examples, macOS/Windows/Linux platform notes, troubleshooting blocks, and a kinder doctor reference.

## Performance

| Metric | Value |
|--------|-------|
| Duration | 1m 22s |
| Started | 2026-04-09T16:36:32Z |
| Completed | 2026-04-09T16:37:54Z |
| Tasks completed | 1/1 |
| Files created | 1 |
| Files modified | 0 |

## Accomplishments

- Created `kinder-site/src/content/docs/guides/host-directory-mounting.md` covering the full two-hop mount workflow
- Opening paragraph explains why two hops are needed (kinder nodes are containers, not VMs)
- Complete runnable example: host directory creation, cluster config with extraMounts, PV+PVC YAML, pod YAML, kubectl logs verification
- Visual text-based flow diagram: `host machine -> node container -> pod`
- Platform notes section covering macOS Docker Desktop (file sharing + propagation), Windows (WSL 2 path format), and Linux (native, no VM)
- Troubleshooting section with three caution blocks: PVC stuck in Pending, empty /data directory, cluster creation path-not-found
- kinder doctor tip block explaining how to use diagnostics after cluster creation
- Cleanup section with sequential delete commands

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create host-directory-mounting.md guide | 70642ae1 | kinder-site/src/content/docs/guides/host-directory-mounting.md |

## Files Created

| File | Purpose |
|------|---------|
| kinder-site/src/content/docs/guides/host-directory-mounting.md | Guide: two-hop host directory mounting pattern |

## Files Modified

None.

## Decisions Made

1. **Absolute path in YAML example:** Used `/Users/you/shared-data` with a note to replace it, because tilde (`~`) is a shell expansion and kind config files do not expand it. This avoids user confusion from a config that looks correct but silently uses a literal `~` path.

2. **Added Windows section:** The plan required macOS platform notes; Windows was added as a brief section to make the guide useful for all supported platforms and prevent user confusion on Windows.

3. **"How it works" section placement:** Placed the two-hop explanation after verification (not before), so users can complete the tutorial before reading the theory. Follows the same pattern as other guides where conceptual explanation follows the working example.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None. The guide is a static documentation page with no data sources or dynamic content.

## Threat Flags

None. This plan produces a static documentation file with no new network endpoints, auth paths, or schema changes.

## Next Phase Readiness

Phase 45 (all three plans) is now complete:
- 45-01: Pre-flight host-path validation + propagation warning in `create.go`
- 45-02: `kinder doctor` host mount check (`hostmount.go`)
- 45-03: Documentation guide (this plan)

Ready for Phase 46 (load images addon).

## Self-Check: PASSED

- [x] `kinder-site/src/content/docs/guides/host-directory-mounting.md` exists on disk
- [x] Commit `70642ae1` exists in git log
- [x] `extraMounts` appears in guide (9 occurrences)
- [x] `PersistentVolume` appears in guide (9 occurrences)
- [x] `Docker Desktop` appears in guide (9 occurrences)
- [x] `kinder doctor` appears in guide (3 occurrences)
- [x] Two-hop pattern described in opening paragraph and How it works section
