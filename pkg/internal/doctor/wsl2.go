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
	"os"
	"strings"
)

// requiredCgroupControllers lists the cgroup v2 controllers that kind
// nodes need for resource management.
var requiredCgroupControllers = []string{"cpu", "memory", "pids"}

// wsl2Check detects whether the system is running under WSL2 using a
// multi-signal approach, then verifies cgroup v2 controller availability.
//
// Signal 1: /proc/version contains "microsoft" (case-insensitive).
// Signal 2 (one of):
//
//	a) WSL_DISTRO_NAME environment variable is set
//	b) /proc/sys/fs/binfmt_misc/WSLInterop file exists
//	c) /proc/sys/fs/binfmt_misc/WSLInterop-late file exists (WSL 4.1.4+)
//
// Both signals are required to avoid false positives on Azure VMs, which
// also have "microsoft" in /proc/version.
type wsl2Check struct {
	readFile func(string) ([]byte, error)
	getenv   func(string) string
	stat     func(string) (os.FileInfo, error)
}

// newWSL2Check creates a wsl2Check with real system deps.
func newWSL2Check() Check {
	return &wsl2Check{
		readFile: os.ReadFile,
		getenv:   os.Getenv,
		stat:     os.Stat,
	}
}

func (c *wsl2Check) Name() string       { return "wsl2-cgroup" }
func (c *wsl2Check) Category() string    { return "Platform" }
func (c *wsl2Check) Platforms() []string { return []string{"linux"} }

func (c *wsl2Check) Run() []Result {
	if !c.isWSL2() {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "Not running under WSL2",
		}}
	}

	// WSL2 confirmed -- check cgroup v2 controllers.
	data, err := c.readFile("/sys/fs/cgroup/cgroup.controllers")
	if err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "cgroup v2 controllers not available",
			Reason:   "WSL2 detected but cgroup v2 controllers cannot be read; kind nodes need cpu, memory, and pids controllers",
			Fix:      "Enable cgroup v2: https://github.com/spurin/wsl-cgroupsv2",
		}}
	}

	available := strings.Fields(strings.TrimSpace(string(data)))
	availableSet := make(map[string]bool, len(available))
	for _, ctrl := range available {
		availableSet[ctrl] = true
	}

	var missing []string
	for _, req := range requiredCgroupControllers {
		if !availableSet[req] {
			missing = append(missing, req)
		}
	}

	if len(missing) > 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("WSL2 cgroup v2 missing controllers: %s", strings.Join(missing, ", ")),
			Reason:   "kind nodes require cpu, memory, and pids cgroup controllers for resource management",
			Fix:      "Enable missing controllers: https://github.com/spurin/wsl-cgroupsv2",
		}}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  fmt.Sprintf("WSL2 cgroup v2 controllers available: %s", strings.Join(available, ", ")),
	}}
}

// isWSL2 implements multi-signal WSL2 detection.
// Requires /proc/version to contain "microsoft" (signal 1) AND at least
// one of: WSL_DISTRO_NAME set, WSLInterop file exists, or WSLInterop-late
// file exists (signal 2). This prevents false positives on Azure VMs.
func (c *wsl2Check) isWSL2() bool {
	// Signal 1: /proc/version must contain "microsoft" (case-insensitive).
	data, err := c.readFile("/proc/version")
	if err != nil {
		return false
	}
	if !strings.Contains(strings.ToLower(string(data)), "microsoft") {
		return false
	}

	// Signal 2a: WSL_DISTRO_NAME environment variable set.
	if c.getenv("WSL_DISTRO_NAME") != "" {
		return true
	}

	// Signal 2b: WSLInterop file exists.
	if _, err := c.stat("/proc/sys/fs/binfmt_misc/WSLInterop"); err == nil {
		return true
	}

	// Signal 2c: WSLInterop-late file exists (WSL 4.1.4+).
	if _, err := c.stat("/proc/sys/fs/binfmt_misc/WSLInterop-late"); err == nil {
		return true
	}

	// No second signal -- likely Azure VM or similar.
	return false
}
