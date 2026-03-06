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
	"strings"

	"sigs.k8s.io/kind/pkg/exec"
)

// diskSpaceCheck verifies that sufficient disk space is available for
// Docker images and containers.
type diskSpaceCheck struct {
	readDiskFree func(path string) (uint64, error)
	execCmd      func(name string, args ...string) exec.Cmd
}

// newDiskSpaceCheck creates a diskSpaceCheck with real system deps.
func newDiskSpaceCheck() Check {
	return &diskSpaceCheck{
		readDiskFree: statfsFreeBytes,
		execCmd:      exec.Command,
	}
}

func (c *diskSpaceCheck) Name() string       { return "disk-space" }
func (c *diskSpaceCheck) Category() string    { return "Docker" }
func (c *diskSpaceCheck) Platforms() []string { return nil }

func (c *diskSpaceCheck) Run() []Result {
	// Determine the Docker data root directory.
	dataRoot := "/var/lib/docker"
	lines, err := exec.OutputLines(c.execCmd("docker", "info", "--format", "{{.DockerRootDir}}"))
	if err == nil && len(lines) > 0 {
		if trimmed := strings.TrimSpace(lines[0]); trimmed != "" {
			dataRoot = trimmed
		}
	}

	// Try to read free space on the data root, fall back to "/".
	checkedPath := dataRoot
	bytes, err := c.readDiskFree(dataRoot)
	if err != nil {
		checkedPath = "/"
		bytes, err = c.readDiskFree("/")
		if err != nil {
			return []Result{{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "warn",
				Message:  "Could not check disk space",
				Reason:   "Unable to determine free disk space",
				Fix:      "Check disk space manually: df -h",
			}}
		}
	}

	freeGB := float64(bytes) / (1024 * 1024 * 1024)

	if freeGB < 2.0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "fail",
			Message:  fmt.Sprintf("%.1f GB free on %s", freeGB, checkedPath),
			Reason:   "Insufficient disk space for Docker images and containers",
			Fix:      "Free disk space: docker system prune && docker image prune -a",
		}}
	}

	if freeGB < 5.0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("%.1f GB free on %s", freeGB, checkedPath),
			Reason:   "Low disk space may cause issues with Docker images and containers",
			Fix:      "Free disk space: docker system prune",
		}}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  fmt.Sprintf("%.1f GB free on %s", freeGB, checkedPath),
	}}
}
