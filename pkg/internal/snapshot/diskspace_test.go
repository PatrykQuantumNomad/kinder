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

package snapshot

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

// ---------------------------------------------------------------------------
// EnsureDiskSpace tests
// ---------------------------------------------------------------------------

func TestEnsureDiskSpaceSufficient(t *testing.T) {
	// Any modern dev box has at least 1 byte free in /tmp.
	if err := EnsureDiskSpace(os.TempDir(), 1); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestEnsureDiskSpaceMissingPath(t *testing.T) {
	err := EnsureDiskSpace("/nonexistent/path/xyz/abc123", 1)
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}

// TestEnsureDiskSpaceInsufficient tests via the pure inner function with a
// synthetic small-filesystem result to avoid relying on real disk state.
func TestEnsureDiskSpaceInsufficient(t *testing.T) {
	// Construct a Statfs_t that reports a very small free space.
	st := syscall.Statfs_t{
		Bavail: 10,   // 10 available blocks
		Bsize:  4096, // 4KB block size → 40960 bytes free
	}
	err := ensureFromStatfs(st, 1<<30, "/tmp") // require 1GB
	if err == nil {
		t.Fatal("expected ErrInsufficientDiskSpace, got nil")
	}
	if !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Errorf("expected ErrInsufficientDiskSpace, got: %v", err)
	}
}

// TestErrInsufficientDiskSpaceSentinel verifies the sentinel via ensureFromStatfs.
func TestErrInsufficientDiskSpaceSentinel(t *testing.T) {
	st := syscall.Statfs_t{
		Bavail: 1,
		Bsize:  512, // 512 bytes free
	}
	err := ensureFromStatfs(st, 1024, "/tmp") // require 1KB
	if !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Errorf("expected ErrInsufficientDiskSpace sentinel, got: %v", err)
	}
}
