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
)

// ErrInsufficientDiskSpace is returned by EnsureDiskSpace when the filesystem
// containing path does not have enough free bytes. Use errors.Is to check.
var ErrInsufficientDiskSpace = errors.New("insufficient disk space")

// EnsureDiskSpace returns nil if the filesystem containing path has at least
// required bytes free (available to non-root processes on Unix; equivalent
// for the calling user on Windows). On failure, wraps ErrInsufficientDiskSpace
// with the observed free bytes and the path.
//
// The platform-specific freeBytes implementation lives in diskspace_unix.go
// and diskspace_windows.go.
func EnsureDiskSpace(path string, required int64) error {
	free, err := freeBytes(path)
	if err != nil {
		return fmt.Errorf("statfs %s: %w", path, err)
	}
	return ensureFromBytes(int64(free), required, path)
}

// ensureFromBytes is the pure inner check used by tests so they do not depend
// on real filesystem state or platform-specific syscall types.
func ensureFromBytes(free, required int64, path string) error {
	if free < required {
		return fmt.Errorf("%w: need %d bytes, %d available at %s",
			ErrInsufficientDiskSpace, required, free, path)
	}
	return nil
}
