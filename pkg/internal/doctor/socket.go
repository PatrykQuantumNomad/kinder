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
	osexec "os/exec"
	"strings"

	"sigs.k8s.io/kind/pkg/exec"
)

// dockerSocketCheck detects Docker socket permission denied errors on Linux
// and suggests the usermod fix.
type dockerSocketCheck struct {
	lookPath func(string) (string, error)
	execCmd  func(name string, args ...string) exec.Cmd
}

// newDockerSocketCheck creates a dockerSocketCheck with real system deps.
func newDockerSocketCheck() Check {
	return &dockerSocketCheck{
		lookPath: osexec.LookPath,
		execCmd:  exec.Command,
	}
}

func (c *dockerSocketCheck) Name() string       { return "docker-socket" }
func (c *dockerSocketCheck) Category() string    { return "Docker" }
func (c *dockerSocketCheck) Platforms() []string { return []string{"linux"} }

func (c *dockerSocketCheck) Run() []Result {
	if _, err := c.lookPath("docker"); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "(docker not found)",
		}}
	}

	lines, err := exec.CombinedOutputLines(c.execCmd("docker", "info"))
	if err != nil {
		output := strings.ToLower(strings.Join(lines, " "))
		if strings.Contains(output, "permission denied") {
			return []Result{{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "fail",
				Message:  "Docker socket permission denied",
				Reason:   "Your user does not have permission to access the Docker daemon",
				Fix:      "sudo usermod -aG docker $USER && newgrp docker",
			}}
		}
		// Not a permission issue -- the container-runtime check handles daemon-not-running.
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "Docker socket accessible (daemon may not be running)",
		}}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  "Docker socket accessible",
	}}
}
