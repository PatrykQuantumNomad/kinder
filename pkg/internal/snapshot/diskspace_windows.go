//go:build windows

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

import "golang.org/x/sys/windows"

// freeBytes returns the number of bytes the calling user can use on the
// volume containing path, via GetDiskFreeSpaceExW. The first uint64 out-param
// (lpFreeBytesAvailableToCaller) is the user-quota-aware figure; the others
// are total volume size and total free bytes regardless of quota.
func freeBytes(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeAvailable, totalBytes, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeAvailable, &totalBytes, &totalFree); err != nil {
		return 0, err
	}
	return freeAvailable, nil
}
