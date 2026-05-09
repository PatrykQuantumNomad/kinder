//go:build unix

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

import "golang.org/x/sys/unix"

// freeBytes returns the number of free bytes available to non-root processes
// on the filesystem containing path. Uses statfs(2) under the hood.
//
// The int64 cast on Bsize handles the type difference between Linux (int64)
// and macOS (uint32). The same pattern is used in pkg/internal/doctor.
func freeBytes(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(int64(st.Bsize)), nil
}
