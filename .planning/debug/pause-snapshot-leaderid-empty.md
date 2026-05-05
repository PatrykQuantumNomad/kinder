---
status: investigating
trigger: "pause-snapshot-leaderid-empty-legacy-etcdctl-path"
created: 2026-05-05T00:00:00Z
updated: 2026-05-05T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED — stale Homebrew Cask binary. `/opt/homebrew/Caskroom/kinder/1.4/kinder` was built Apr 11 2026; commit 0c612a54 (47-05 crictl fix) landed May 5 2026 09:54. The on-disk source already uses crictl (verified lines 257-294 of pause.go). The running binary literally cannot contain the fix.
test: (done) — checked pause.go source on disk, git history of readEtcdLeaderID, and mtimes of every `kinder` binary on $PATH and in repo.
expecting: User must rebuild and reinstall kinder from current HEAD before re-running UAT 14 (and likely 12/13).
next_action: return ROOT CAUSE FOUND

## Symptoms

expected: After `kinder pause` on a 3-CP HA cluster, `/kind/pause-snapshot.json` contains `{"leaderID": "<non-empty number>", "pauseTime": "<RFC3339>"}`. 47-05 used `crictl ps --name etcd -q` then `crictl exec <id> etcdctl ...`.
actual: leaderID is empty; stderr shows: `failed to capture etcd leader id for HA pause snapshot (continuing): command "docker exec --privileged verify47-control-plane etcdctl --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/peer.crt --key=/etc/kubernetes/pki/etcd/peer.key --endpoints=https://127.0.0.1:2379 endpoint status --cluster --write-out=json" failed with error: exit status 127`
errors: exit status 127 (= command not found in shell). etcdctl IS NOT in kindest/node rootfs.
reproduction: Test 14 in 47-UAT.md. `kinder pause <ha-cluster>` then docker start CP and cat /kind/pause-snapshot.json
started: Discovered during UAT 2026-05-05. 47-05 was committed (a54de41c, 0c612a54) before this test.

## Eliminated

(none yet)

## Evidence

- timestamp: 2026-05-05
  checked: pkg/internal/lifecycle/pause.go on disk (HEAD)
  found: readEtcdLeaderID at lines 257-294 uses crictl path: `crictl ps --name etcd -q` then `crictl exec <id> etcdctl ...`. NO `docker exec ... etcdctl` direct invocation exists in source.
  implication: Source code is already correct. The bug must be in what's running, not what's checked in.

- timestamp: 2026-05-05
  checked: `grep -rn 'docker exec --privileged.*etcdctl\|/usr/local/bin/etcdctl' pkg/ cmd/`
  found: zero hits
  implication: Confirmed — legacy etcdctl path no longer exists anywhere in source.

- timestamp: 2026-05-05
  checked: `git log --oneline -- pkg/internal/lifecycle/pause.go`
  found: 0c612a54 (47-05, May 5 09:54) explicitly replaces the legacy path. Commit ac8c1a16 (47-02) introduced the legacy `candidates := []string{"/usr/local/bin/etcdctl", "etcdctl"}` loop that produced the exact stderr the user is seeing.
  implication: The user's running binary contains pre-47-05 source — i.e., the 47-02 candidate-loop.

- timestamp: 2026-05-05
  checked: `which kinder`, `stat`, `file` on the binary on $PATH
  found: `/opt/homebrew/bin/kinder` is a symlink to `/opt/homebrew/Caskroom/kinder/1.4/kinder`. Cask binary mtime: **Apr 11 2026 07:21:58** (Mach-O arm64). 47-05 commit landed **May 5 2026 09:54:47** — 24 days *after* the binary was built.
  implication: SMOKING GUN — the installed kinder physically cannot contain the crictl fix.

- timestamp: 2026-05-05
  checked: `ls /Users/patrykattc/work/git/kinder/bin/kinder` (repo-local build)
  found: `/Users/patrykattc/work/git/kinder/bin/kinder` mtime **May 5 2026 08:40:53** — also pre-47-05 (47-05 landed at 09:54). The repo-local binary is also stale.
  implication: User has *no* binary on disk that contains the 47-05 fix. They must rebuild before re-testing.

- timestamp: 2026-05-05
  checked: stderr text reported by user — `command "docker exec --privileged verify47-control-plane etcdctl --cacert=... endpoint status --cluster --write-out=json"`
  found: This is exactly the wrapped form of `bootstrap.Command("etcdctl", "--cacert=...", "endpoint", "status", ...)` in the 47-02 candidate loop. If the new crictl source were running, the wrapped command would be `docker exec --privileged verify47-control-plane crictl exec <container-id> etcdctl ...`. It isn't.
  implication: Triple-confirmed — running binary is pre-47-05.

- timestamp: 2026-05-05
  checked: 47-UAT.md tests 12 (resume readiness check), 13 (HA gate), 14 (this one)
  found: All three tests verify behavior added/fixed in 47-04 and 47-05. Test 12 reads the same /kind/pause-snapshot.json that test 14 inspects; if leaderID is "" the resume readiness check has nothing to compare against. Test 13 gates HA snapshot on len(cp)>=2, also added recently.
  implication: All three test failures are the same root cause: stale binary missing 47-04 and 47-05 changes. One rebuild fixes all of them.

## Resolution

root_cause: |
  STALE BINARY. The `kinder` on the user's $PATH (`/opt/homebrew/bin/kinder` → `/opt/homebrew/Caskroom/kinder/1.4/kinder`) was built **Apr 11 2026**, ~24 days BEFORE commit 0c612a54 (47-05 crictl fix, May 5 2026 09:54). The on-disk source at pkg/internal/lifecycle/pause.go:257-294 already uses the correct crictl path. The repo-local `bin/kinder` is also stale (May 5 08:40, ~74 minutes before 47-05). The user is running a pre-47-05 binary that still contains the 47-02 `[]string{"/usr/local/bin/etcdctl","etcdctl"}` candidate loop, which produces exactly the `docker exec --privileged ... etcdctl ... exit status 127` stderr they reported. This is NOT a code regression — it is a build/install gap.

fix: |
  Rebuild and reinstall kinder from current HEAD, then re-run UAT 14 (and 12/13). The Homebrew Cask version (1.4, locked to a fixed binary) must be replaced with a freshly-built binary from the local repo. From repo root:
    make build           # or: go build -o bin/kinder ./cmd/kinder
    install bin/kinder /opt/homebrew/bin/kinder    # overrides the cask symlink
  Verify: `kinder --version` should show a HEAD-matching commit, and `strings $(which kinder) | grep -E 'crictl ps --name etcd|crictl exec'` should match (the new code path) while `strings $(which kinder) | grep '/usr/local/bin/etcdctl'` should NOT match (legacy path absent).

verification: (pending — user action: rebuild + rerun UAT 14)
files_changed: []  # no source changes needed; source is already correct
