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

	"sigs.k8s.io/kind/pkg/exec"
)

// containerRuntimeCheck detects a working container runtime (docker, podman, or nerdctl).
type containerRuntimeCheck struct {
	lookPath func(string) (string, error)
	execCmd  func(name string, args ...string) exec.Cmd
}

// newContainerRuntimeCheck creates a containerRuntimeCheck with real system deps.
func newContainerRuntimeCheck() Check {
	return &containerRuntimeCheck{
		lookPath: osexec.LookPath,
		execCmd:  exec.Command,
	}
}

func (c *containerRuntimeCheck) Name() string       { return "container-runtime" }
func (c *containerRuntimeCheck) Category() string    { return "Runtime" }
func (c *containerRuntimeCheck) Platforms() []string { return nil }

func (c *containerRuntimeCheck) Run() []Result {
	runtimes := []string{"docker", "podman", "nerdctl"}
	for _, rt := range runtimes {
		if _, err := c.lookPath(rt); err != nil {
			continue
		}
		// Binary found -- check if it responds to "version".
		lines, err := exec.OutputLines(c.execCmd(rt, "version"))
		if err == nil && len(lines) > 0 {
			return []Result{{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "ok",
				Message:  fmt.Sprintf("Container runtime detected (%s)", rt),
			}}
		}
		// Fall back to "-v".
		lines, err = exec.OutputLines(c.execCmd(rt, "-v"))
		if err == nil && len(lines) > 0 {
			return []Result{{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "ok",
				Message:  fmt.Sprintf("Container runtime detected (%s)", rt),
			}}
		}
		// Binary present but not responding.
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  rt + " found but not responding",
			Reason:   "The container daemon must be running for cluster creation",
			Fix:      fmt.Sprintf("Start the %s daemon or check its status", rt),
		}}
	}
	// No runtime found at all.
	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "fail",
		Message:  "No container runtime found",
		Reason:   "A container runtime (Docker, Podman, or nerdctl) is required",
		Fix:      "Install Docker: https://docs.docker.com/get-docker/",
	}}
}
