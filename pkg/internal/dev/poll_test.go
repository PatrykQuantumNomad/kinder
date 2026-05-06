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

const pollTestInterval = 50 * time.Millisecond

func TestStartPoller_DetectsMtimeChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "m.txt", "stable-content")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := StartPoller(ctx, dir, pollTestInterval, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartPoller: %v", err)
	}

	// Allow at least one tick of stability — no emit should arrive yet because
	// the snapshot has not changed since startup.
	if waitForEvent(t, ch, 2*pollTestInterval) {
		t.Fatal("unexpected emit on stable directory")
	}

	// Bump mtime by 5s — guaranteed to differ regardless of FS resolution.
	future := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	if !waitForEvent(t, ch, 500*time.Millisecond) {
		t.Fatal("expected emit after mtime change, got timeout")
	}
}

func TestStartPoller_DetectsSizeChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "s.txt", "a")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := StartPoller(ctx, dir, pollTestInterval, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartPoller: %v", err)
	}

	writeFile(t, dir, "s.txt", "abc-much-longer")

	if !waitForEvent(t, ch, 500*time.Millisecond) {
		t.Fatal("expected emit after size change, got timeout")
	}
}

func TestStartPoller_DetectsNewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := StartPoller(ctx, dir, pollTestInterval, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartPoller: %v", err)
	}

	writeFile(t, dir, "n.txt", "fresh")

	if !waitForEvent(t, ch, 500*time.Millisecond) {
		t.Fatal("expected emit after new file, got timeout")
	}
}

func TestStartPoller_DetectsDeletedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeFile(t, dir, "d.txt", "doomed")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := StartPoller(ctx, dir, pollTestInterval, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartPoller: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if !waitForEvent(t, ch, 500*time.Millisecond) {
		t.Fatal("expected emit after file delete, got timeout")
	}
}

func TestStartPoller_NoSpuriousEmit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "stable.txt", "do not touch")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := StartPoller(ctx, dir, pollTestInterval, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartPoller: %v", err)
	}

	// 4 ticks worth — must observe ZERO emits.
	deadline := time.After(4 * pollTestInterval)
	for {
		select {
		case <-ch:
			t.Fatal("unexpected spurious emit on stable directory")
		case <-deadline:
			return
		}
	}
}

func TestStartPoller_CtxCancelClosesChannel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "c.txt", "data")

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := StartPoller(ctx, dir, pollTestInterval, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartPoller: %v", err)
	}

	cancel()

	if !waitForClose(t, ch, 500*time.Millisecond) {
		t.Fatal("expected channel to close within 500ms after ctx cancel, got timeout")
	}
}

func TestStartPoller_NonexistentDirReturnsError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := StartPoller(ctx, "/no/such/dir/that/should/not/exist", pollTestInterval, log.NoopLogger{})
	if err == nil {
		t.Fatal("expected non-nil error for nonexistent dir, got nil")
	}
}

// TestStartPoller_DetectsNestedFile validates that the poller walks subdirs.
func TestStartPoller_DetectsNestedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := StartPoller(ctx, dir, pollTestInterval, log.NoopLogger{})
	if err != nil {
		t.Fatalf("StartPoller: %v", err)
	}

	writeFile(t, nested, "nx.txt", "nested-content")

	if !waitForEvent(t, ch, 500*time.Millisecond) {
		t.Fatal("expected emit after creating nested file, got timeout")
	}
}
