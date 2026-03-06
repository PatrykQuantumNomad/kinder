//go:build linux || darwin

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

package doctor

import "golang.org/x/sys/unix"

// statfsFreeBytes returns the number of free bytes available to unprivileged
// users on the filesystem containing path.
func statfsFreeBytes(path string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// int64 cast handles macOS uint32 Bsize vs Linux int64 Bsize.
	return stat.Bavail * uint64(int64(stat.Bsize)), nil
}
