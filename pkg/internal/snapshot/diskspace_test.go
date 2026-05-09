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
	"testing"
)

// ---------------------------------------------------------------------------
// EnsureDiskSpace tests
//
// These tests are platform-portable: they exercise EnsureDiskSpace via the
// real OS temp dir and the pure inner ensureFromBytes helper. They do not
// reference any syscall.Statfs_t / windows.GetDiskFreeSpaceEx symbols, so
// the same test file builds on Linux, macOS, and Windows.
// ---------------------------------------------------------------------------

func TestEnsureDiskSpaceSufficient(t *testing.T) {
	// Any modern dev box has at least 1 byte free in the temp dir.
	if err := EnsureDiskSpace(os.TempDir(), 1); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestEnsureDiskSpaceMissingPath(t *testing.T) {
	// Use os.PathSeparator so the test works on both Unix and Windows.
	bogus := string(os.PathSeparator) + "nonexistent" + string(os.PathSeparator) + "path-xyz-abc123"
	if err := EnsureDiskSpace(bogus, 1); err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}

// TestEnsureDiskSpaceInsufficient exercises the pure inner function with a
// synthetic small free-byte count to avoid relying on real disk state.
func TestEnsureDiskSpaceInsufficient(t *testing.T) {
	const free = int64(40960)   // 40 KiB free
	const required = int64(1) << 30 // require 1 GiB
	err := ensureFromBytes(free, required, "/tmp")
	if err == nil {
		t.Fatal("expected ErrInsufficientDiskSpace, got nil")
	}
	if !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Errorf("expected ErrInsufficientDiskSpace, got: %v", err)
	}
}

// TestErrInsufficientDiskSpaceSentinel verifies the sentinel via the pure
// helper.
func TestErrInsufficientDiskSpaceSentinel(t *testing.T) {
	if err := ensureFromBytes(512, 1024, "/tmp"); !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Errorf("expected ErrInsufficientDiskSpace sentinel, got: %v", err)
	}
}
