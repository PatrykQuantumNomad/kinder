/*
Copyright 2019 The Kubernetes Authors.

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

package kubeconfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockUnlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filename := filepath.Join(dir, "kubeconfig")

	// Lock should succeed
	if err := lockFile(filename); err != nil {
		t.Fatalf("lockFile failed: %v", err)
	}

	// Verify lock file exists
	lockPath := lockName(filename)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatal("expected lock file to exist after lockFile")
	}

	// Second lock should fail (lock already held)
	if err := lockFile(filename); err == nil {
		t.Fatal("expected error when locking already-locked file")
	}

	// Unlock should succeed
	if err := unlockFile(filename); err != nil {
		t.Fatalf("unlockFile failed: %v", err)
	}

	// Verify lock file is removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("expected lock file to be removed after unlockFile")
	}

	// Lock should succeed again after unlock
	if err := lockFile(filename); err != nil {
		t.Fatalf("lockFile after unlock failed: %v", err)
	}
	// Clean up
	_ = unlockFile(filename)
}

func TestLockStaleLockDetection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filename := filepath.Join(dir, "kubeconfig")
	lockPath := lockName(filename)

	// Create a stale lock file manually
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0)
	if err != nil {
		t.Fatalf("failed to create stale lock file: %v", err)
	}
	_ = f.Close()

	// Set the lock file's modification time to 10 minutes ago
	staleTime := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatalf("failed to set lock file mtime: %v", err)
	}

	// lockFile should detect the stale lock, remove it, and succeed
	if err := lockFile(filename); err != nil {
		t.Fatalf("lockFile should have removed stale lock and succeeded, got: %v", err)
	}

	// Verify the lock file exists (newly created)
	info, err := os.Stat(lockPath)
	if os.IsNotExist(err) {
		t.Fatal("expected lock file to exist after stale lock removal")
	}
	if err != nil {
		t.Fatalf("unexpected error stating lock file: %v", err)
	}

	// The new lock file should have a recent modification time
	if time.Since(info.ModTime()) > 1*time.Minute {
		t.Error("new lock file should have a recent modification time")
	}

	// Clean up
	_ = unlockFile(filename)
}

func TestLockFreshLockNotRemoved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filename := filepath.Join(dir, "kubeconfig")
	lockPath := lockName(filename)

	// Create a fresh lock file (simulates another process holding the lock)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0)
	if err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}
	_ = f.Close()

	// lockFile should fail because the lock is fresh (not stale)
	if err := lockFile(filename); err == nil {
		t.Fatal("expected error when fresh lock file exists")
	}

	// Clean up
	_ = os.Remove(lockPath)
}

func TestLockCreatesDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Use a path where the parent directory does not exist yet
	filename := filepath.Join(dir, "subdir", "kubeconfig")

	if err := lockFile(filename); err != nil {
		t.Fatalf("lockFile should create parent directory, got: %v", err)
	}

	// Verify the lock file was created
	lockPath := lockName(filename)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatal("expected lock file to exist")
	}

	// Clean up
	_ = unlockFile(filename)
}
