/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dev

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/kind/pkg/log"
)

// fileState captures the per-file attributes the poller compares between ticks.
// Two files are considered "the same" iff size and mtime are equal.
type fileState struct {
	size  int64
	mtime time.Time
}

// StartPoller launches a polling-based file watcher rooted at watchDir.
// It snapshots all file sizes and mtimes every `interval`, comparing the
// new snapshot to the previous one. If any file was added, removed, or
// changed in size or mtime, it emits a single send on the returned channel.
//
// StartPoller is the --poll fallback for environments where fsnotify
// events are unreliable: Docker Desktop volume mounts on macOS, NFS,
// SSHFS, etc. (RESEARCH §5). Output channel is buffered cap=1 — the
// polling cadence itself rate-limits emits, and the caller-side debouncer
// stacked on top is the canonical user-facing debounce.
//
// Errors during WalkDir are logged at warn but do not abort the poller —
// transient permission errors or files that vanish mid-walk should not
// kill the watch loop.
func StartPoller(ctx context.Context, watchDir string, interval time.Duration, logger log.Logger) (<-chan struct{}, error) {
	if _, err := os.Stat(watchDir); err != nil {
		return nil, err
	}
	prev := snapshotDir(watchDir, logger)

	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				curr := snapshotDir(watchDir, logger)
				if !fileStateMapsEqual(prev, curr) {
					select {
					case out <- struct{}{}:
					default:
					}
				}
				prev = curr
			}
		}
	}()
	return out, nil
}

// snapshotDir walks dir and captures size+mtime for every regular file.
// Per-file errors (permission denied on a sibling, file removed mid-walk,
// etc.) are logged at Warn and skipped — they must not abort the snapshot.
func snapshotDir(dir string, logger log.Logger) map[string]fileState {
	snap := map[string]fileState{}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Warn("dev poller: " + err.Error())
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			// File likely removed between WalkDir's readdir and Info() — ignore.
			return nil
		}
		snap[path] = fileState{size: info.Size(), mtime: info.ModTime()}
		return nil
	})
	return snap
}

// fileStateMapsEqual returns true iff a and b have identical key sets and
// every shared key maps to an equal fileState.
func fileStateMapsEqual(a, b map[string]fileState) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || va != vb {
			return false
		}
	}
	return true
}
