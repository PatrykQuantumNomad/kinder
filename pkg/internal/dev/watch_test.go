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
	"os"
	"path/filepath"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/log"
)

// writeFile creates (or overwrites) a file under dir with the given name and content.
// It returns the absolute path to the file. fsync via Close is enough — fsnotify on
// linux/darwin observes Write events for unbuffered close-after-write.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile(%s): %v", path, err)
	}
	return path
}

// waitForEvent reads from ch with a timeout. Returns true if at least one
// event arrived within d, false on timeout.
func waitForEvent(t *testing.T, ch <-chan struct{}, d time.Duration) bool {
	t.Helper()
	select {
	case _, ok := <-ch:
		if !ok {
			t.Fatalf("channel closed unexpectedly while waiting for event")
		}
		return true
	case <-time.After(d):
		return false
	}
}

// waitForClose returns true if ch is closed within d, false on timeout.
func waitForClose(t *testing.T, ch <-chan struct{}, d time.Duration) bool {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return true
			}
			// drained an event — keep waiting for close
		case <-deadline:
			return false
		}
	}
}

func TestStartWatcher_DetectsWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create file BEFORE starting watcher so the initial Write triggers an event.
	writeFile(t, dir, "a.txt", "v1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := StartWatcher(ctx, dir, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartWatcher: %v", err)
	}

	// Give the watcher goroutine a moment to start its select loop.
	time.Sleep(50 * time.Millisecond)

	writeFile(t, dir, "a.txt", "v2-changed")

	if !waitForEvent(t, ch, 500*time.Millisecond) {
		t.Fatal("expected at least one event after rewriting file, got timeout")
	}
}

func TestStartWatcher_DetectsCreate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := StartWatcher(ctx, dir, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartWatcher: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	writeFile(t, dir, "b.txt", "fresh")

	if !waitForEvent(t, ch, 500*time.Millisecond) {
		t.Fatal("expected at least one event after creating file, got timeout")
	}
}

func TestStartWatcher_AddsNewSubdir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := StartWatcher(ctx, dir, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartWatcher: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	// Drain the Create-of-nested-dir event(s) before the in-subdir write.
	for i := 0; i < 4; i++ {
		select {
		case <-ch:
		case <-time.After(100 * time.Millisecond):
		}
	}

	// Give the watcher a moment to register the new subdir via w.Add.
	time.Sleep(100 * time.Millisecond)

	writeFile(t, nested, "c.txt", "deep")

	if !waitForEvent(t, ch, 1*time.Second) {
		t.Fatal("expected at least one event after writing into a newly created subdir, got timeout")
	}
}

func TestStartWatcher_CtxCancelClosesChannel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := StartWatcher(ctx, dir, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartWatcher: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	cancel()

	if !waitForClose(t, ch, 500*time.Millisecond) {
		t.Fatal("expected channel to close within 500ms after ctx cancel, got timeout")
	}
}

func TestStartWatcher_NonexistentDirReturnsError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := StartWatcher(ctx, "/no/such/dir/that/should/not/exist", log.NoopLogger{})
	if err == nil {
		t.Fatal("expected non-nil error for nonexistent dir, got nil")
	}
}
