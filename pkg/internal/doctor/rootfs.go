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
	osexec "os/exec"
	"strings"

	"sigs.k8s.io/kind/pkg/exec"
)

// rootfsDeviceCheck detects BTRFS as the Docker storage driver or backing
// filesystem, which can cause device node issues with kind containers.
type rootfsDeviceCheck struct {
	lookPath func(string) (string, error)
	execCmd  func(name string, args ...string) exec.Cmd
}

// newRootfsDeviceCheck creates a rootfsDeviceCheck with real system deps.
func newRootfsDeviceCheck() Check {
	return &rootfsDeviceCheck{
		lookPath: osexec.LookPath,
		execCmd:  exec.Command,
	}
}

func (c *rootfsDeviceCheck) Name() string       { return "rootfs-device" }
func (c *rootfsDeviceCheck) Category() string    { return "Platform" }
func (c *rootfsDeviceCheck) Platforms() []string { return []string{"linux"} }

func (c *rootfsDeviceCheck) Run() []Result {
	// Step 1: Check if docker is available.
	if _, err := c.lookPath("docker"); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "(docker not found)",
		}}
	}

	// Step 2: Query Docker storage driver.
	lines, err := exec.OutputLines(c.execCmd("docker", "info", "-f", "{{.Driver}}"))
	if err != nil || len(lines) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "(could not query Docker storage driver)",
		}}
	}

	driver := strings.TrimSpace(lines[0])

	// Step 3: Check if the storage driver itself is btrfs.
	if strings.EqualFold(driver, "btrfs") {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("Docker storage driver is %s", driver),
			Reason:   "BTRFS storage driver can cause device node and snapshot issues with kind/kubelet",
			Fix:      "Consider switching to overlay2: https://kind.sigs.k8s.io/docs/user/known-issues/#docker-on-btrfs",
		}}
	}

	// Step 4: Check DriverStatus for btrfs backing filesystem.
	statusLines, err := exec.OutputLines(c.execCmd("docker", "info", "-f", "{{json .DriverStatus}}"))
	if err == nil && len(statusLines) > 0 {
		statusOutput := strings.ToLower(strings.Join(statusLines, " "))
		if strings.Contains(statusOutput, "btrfs") {
			return []Result{{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "warn",
				Message:  fmt.Sprintf("Docker backing filesystem is btrfs (driver: %s)", driver),
				Reason:   "BTRFS backing filesystem can cause device node issues with kind containers",
				Fix:      "Consider using an ext4 or xfs partition for Docker storage: https://kind.sigs.k8s.io/docs/user/known-issues/#docker-on-btrfs",
			}}
		}
	}

	// Step 5: All clear.
	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  fmt.Sprintf("Docker storage driver: %s", driver),
	}}
}
