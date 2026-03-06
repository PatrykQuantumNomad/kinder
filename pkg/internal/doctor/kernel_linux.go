//go:build linux

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

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// minKernelMajor and minKernelMinor define the minimum kernel version
// required for cgroup namespace support (Linux 4.6+).
const (
	minKernelMajor = 4
	minKernelMinor = 6
)

// kernelVersionCheck verifies the Linux kernel version is >= 4.6
// for cgroup namespace support required by kind.
type kernelVersionCheck struct {
	uname func(buf *unix.Utsname) error
}

// newKernelVersionCheck creates a kernelVersionCheck with real system deps.
func newKernelVersionCheck() Check {
	return &kernelVersionCheck{
		uname: unix.Uname,
	}
}

func (c *kernelVersionCheck) Name() string       { return "kernel-version" }
func (c *kernelVersionCheck) Category() string    { return "Kernel" }
func (c *kernelVersionCheck) Platforms() []string { return []string{"linux"} }

func (c *kernelVersionCheck) Run() []Result {
	var buf unix.Utsname
	if err := c.uname(&buf); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("Could not determine kernel version: %v", err),
			Reason:   "Unable to call uname to check kernel version",
			Fix:      "uname -r",
		}}
	}

	release := unix.ByteSliceToString(buf.Release[:])
	major, minor, err := parseKernelVersion(release)
	if err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("Could not parse kernel version %q: %v", release, err),
			Reason:   "Kernel version string has unexpected format",
			Fix:      "uname -r",
		}}
	}

	if major < minKernelMajor || (major == minKernelMajor && minor < minKernelMinor) {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "fail",
			Message:  fmt.Sprintf("Kernel %d.%d is below minimum %d.%d", major, minor, minKernelMajor, minKernelMinor),
			Reason:   fmt.Sprintf("Kernel < %d.%d lacks cgroup namespace support required by kind", minKernelMajor, minKernelMinor),
			Fix:      fmt.Sprintf("Upgrade kernel to %d.%d or later", minKernelMajor, minKernelMinor),
		}}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  fmt.Sprintf("Kernel %d.%d meets minimum %d.%d", major, minor, minKernelMajor, minKernelMinor),
	}}
}

// parseKernelVersion extracts major and minor version numbers from a
// kernel release string like "5.15.0-91-generic" or "6.8.0-1025-azure".
func parseKernelVersion(release string) (major, minor int, err error) {
	if release == "" {
		return 0, 0, fmt.Errorf("empty release string")
	}

	parts := strings.SplitN(release, ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("expected at least major.minor, got %q", release)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}

	// Minor version may contain suffixes like "4" from "4-generic" or "6+"
	minorStr := parts[1]
	if idx := strings.IndexAny(minorStr, "-+"); idx >= 0 {
		minorStr = minorStr[:idx]
	}

	minor, err = strconv.Atoi(minorStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}

	return major, minor, nil
}
