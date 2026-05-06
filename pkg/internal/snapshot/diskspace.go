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
	"fmt"
	"syscall"
)

// ErrInsufficientDiskSpace is returned by EnsureDiskSpace when the filesystem
// containing path does not have enough free bytes. Use errors.Is to check.
var ErrInsufficientDiskSpace = errors.New("insufficient disk space")

// EnsureDiskSpace returns nil if the filesystem containing path has at least
// required bytes free (available to non-root processes). On failure, wraps
// ErrInsufficientDiskSpace with the observed free bytes and the path.
func EnsureDiskSpace(path string, required int64) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return fmt.Errorf("statfs %s: %w", path, err)
	}
	return ensureFromStatfs(st, required, path)
}

// ensureFromStatfs is the pure inner function that performs the space check
// against a pre-populated Statfs_t. This is exported for testing without
// relying on real filesystem state.
//
// Bsize is the fundamental block size; Bavail is blocks available to non-root.
func ensureFromStatfs(st syscall.Statfs_t, required int64, path string) error {
	free := int64(st.Bavail) * int64(st.Bsize)
	if free < required {
		return fmt.Errorf("%w: need %d bytes, %d available at %s",
			ErrInsufficientDiskSpace, required, free, path)
	}
	return nil
}
