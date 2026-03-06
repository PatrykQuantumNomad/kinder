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
	"os"
	"strings"
)

// selinuxCheck detects SELinux enforcing mode, which on Fedora causes
// /dev/dma_heap permission denials that break kind node containers.
type selinuxCheck struct {
	readFile func(string) ([]byte, error)
}

// newSELinuxCheck creates a selinuxCheck with real system deps.
func newSELinuxCheck() Check {
	return &selinuxCheck{
		readFile: os.ReadFile,
	}
}

func (c *selinuxCheck) Name() string       { return "selinux" }
func (c *selinuxCheck) Category() string    { return "Security" }
func (c *selinuxCheck) Platforms() []string { return []string{"linux"} }

func (c *selinuxCheck) Run() []Result {
	data, err := c.readFile("/sys/fs/selinux/enforce")
	if err != nil {
		// SELinux not available on this system.
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "SELinux not available",
		}}
	}

	enforce := strings.TrimSpace(string(data))
	if enforce == "0" {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "SELinux permissive or disabled",
		}}
	}

	// SELinux is enforcing -- only warn on Fedora where /dev/dma_heap
	// denials are a known issue.
	if c.isFedora() {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "SELinux is enforcing on Fedora",
			Reason:   "Fedora SELinux policy denies /dev/dma_heap access, breaking kind node containers",
			Fix:      "Run: sudo setenforce 0 (temporary) or set SELINUX=permissive in /etc/selinux/config (persistent)",
		}}
	}

	// Enforcing on non-Fedora distro -- no known kind issues.
	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  "SELinux enforcing, no known kind issues on this distro",
	}}
}

// isFedora checks /etc/os-release for ID=fedora (case-insensitive).
// Returns false on any error (cannot confirm Fedora, err on safe side).
func (c *selinuxCheck) isFedora() bool {
	data, err := c.readFile("/etc/os-release")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.EqualFold(line, "id=fedora") || strings.EqualFold(line, `id="fedora"`) {
			return true
		}
	}
	return false
}
