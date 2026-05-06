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
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"

	"sigs.k8s.io/kind/pkg/log"
)

// watcherEventBuffer is the buffer size of the channel returned by StartWatcher.
// 64 is large enough that a typical IDE-save burst (5–50 events) fits, while
// small enough that a runaway producer (e.g. a build tool writing thousands of
// files) trips the non-blocking send fast-path and falls through to the drop
// branch — the debouncer downstream collapses the burst regardless.
const watcherEventBuffer = 64

// StartWatcher launches an fsnotify-based file watcher rooted at watchDir.
// It recursively registers all existing subdirectories at startup and
// dynamically adds new subdirectories as Create events arrive. Each
// Write or Create event on a non-directory path produces a single send
// on the returned channel. Channel is closed when ctx is canceled.
//
// The output channel is buffered (cap=watcherEventBuffer = 64) so a burst
// of events does not block the watcher goroutine. Callers SHOULD pass the
// channel through Debounce() to coalesce rapid bursts into single cycle
// triggers.
//
// Rationale (RESEARCH §5): fsnotify.Watcher.Add is non-recursive in
// v1.10.1. We walk the tree at startup; new subdirs from Create events
// are added on the fly. fsnotify.ErrEventOverflow is handled by
// logging + emitting a synthetic event so the cycle runner notices
// there were many changes (matches recommendation in common pitfall 4).
//
// If watchDir does not exist or is unreadable, StartWatcher returns a
// non-nil error and no goroutine is leaked.
func StartWatcher(ctx context.Context, watchDir string, logger log.Logger) (<-chan struct{}, error) {
	if _, err := os.Stat(watchDir); err != nil {
		return nil, err
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := filepath.WalkDir(watchDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip unreadable subtrees — do not abort registration of the rest.
			return nil
		}
		if d.IsDir() {
			return w.Add(path)
		}
		return nil
	}); err != nil {
		_ = w.Close()
		return nil, err
	}

	out := make(chan struct{}, watcherEventBuffer)
	go func() {
		defer close(out)
		defer w.Close()
		for {
			select {
			case e, ok := <-w.Events:
				if !ok {
					return
				}
				if !(e.Has(fsnotify.Write) || e.Has(fsnotify.Create)) {
					continue
				}
				if e.Has(fsnotify.Create) {
					// Re-Stat in case the create is a new directory; ignore Stat
					// errors (file may have been removed in a race, or symlink
					// dangling — neither warrants killing the watcher).
					if info, sErr := os.Stat(e.Name); sErr == nil && info.IsDir() {
						_ = w.Add(e.Name)
					}
				}
				// Non-blocking send: if the buffer is full, drop. The downstream
				// debouncer collapses bursts to one trigger anyway, so a dropped
				// event under saturation does not lose user intent.
				select {
				case out <- struct{}{}:
				default:
				}
			case wErr, ok := <-w.Errors:
				if !ok {
					return
				}
				if errors.Is(wErr, fsnotify.ErrEventOverflow) {
					logger.Warn("dev watcher: event overflow — triggering cycle")
					select {
					case out <- struct{}{}:
					default:
					}
				} else {
					logger.Warn("dev watcher error: " + wErr.Error())
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
