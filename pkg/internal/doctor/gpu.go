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

// ---------- nvidia-driver check ----------

// nvidiaDriverCheck verifies the NVIDIA driver is installed via nvidia-smi.
type nvidiaDriverCheck struct {
	lookPath func(string) (string, error)
	execCmd  func(name string, args ...string) exec.Cmd
}

func newNvidiaDriverCheck() Check {
	return &nvidiaDriverCheck{
		lookPath: osexec.LookPath,
		execCmd:  exec.Command,
	}
}

func (c *nvidiaDriverCheck) Name() string       { return "nvidia-driver" }
func (c *nvidiaDriverCheck) Category() string    { return "GPU" }
func (c *nvidiaDriverCheck) Platforms() []string { return []string{"linux"} }

func (c *nvidiaDriverCheck) Run() []Result {
	if _, err := c.lookPath("nvidia-smi"); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "nvidia-smi not found",
			Reason:   "NVIDIA GPU addon requires the NVIDIA driver to be installed",
			Fix:      "Install drivers: https://www.nvidia.com/drivers",
		}}
	}
	lines, err := exec.OutputLines(c.execCmd(
		"nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader",
	))
	if err != nil || len(lines) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "nvidia-smi found but could not query driver version",
			Reason:   "The GPU may not be accessible to the current user",
			Fix:      "Check GPU access: nvidia-smi",
		}}
	}
	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  fmt.Sprintf("NVIDIA driver %s", strings.TrimSpace(lines[0])),
	}}
}

// ---------- nvidia-container-toolkit check ----------

// nvidiaContainerToolkitCheck verifies nvidia-ctk is installed.
type nvidiaContainerToolkitCheck struct {
	lookPath func(string) (string, error)
}

func newNvidiaContainerToolkitCheck() Check {
	return &nvidiaContainerToolkitCheck{
		lookPath: osexec.LookPath,
	}
}

func (c *nvidiaContainerToolkitCheck) Name() string       { return "nvidia-container-toolkit" }
func (c *nvidiaContainerToolkitCheck) Category() string    { return "GPU" }
func (c *nvidiaContainerToolkitCheck) Platforms() []string { return []string{"linux"} }

func (c *nvidiaContainerToolkitCheck) Run() []Result {
	if _, err := c.lookPath("nvidia-ctk"); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "nvidia-ctk not found",
			Reason:   "NVIDIA container toolkit is required for GPU containers",
			Fix:      "Install: sudo apt-get install -y nvidia-container-toolkit",
		}}
	}
	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  "nvidia-container-toolkit available",
	}}
}

// ---------- nvidia-docker-runtime check ----------

// nvidiaDockerRuntimeCheck verifies the nvidia runtime is configured in Docker.
// If Docker is not the active runtime, returns skip with "(requires docker)".
type nvidiaDockerRuntimeCheck struct {
	lookPath func(string) (string, error)
	execCmd  func(name string, args ...string) exec.Cmd
}

func newNvidiaDockerRuntimeCheck() Check {
	return &nvidiaDockerRuntimeCheck{
		lookPath: osexec.LookPath,
		execCmd:  exec.Command,
	}
}

func (c *nvidiaDockerRuntimeCheck) Name() string       { return "nvidia-docker-runtime" }
func (c *nvidiaDockerRuntimeCheck) Category() string    { return "GPU" }
func (c *nvidiaDockerRuntimeCheck) Platforms() []string { return []string{"linux"} }

func (c *nvidiaDockerRuntimeCheck) Run() []Result {
	// If Docker is not available, skip this check entirely.
	if _, err := c.lookPath("docker"); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "(requires docker)",
		}}
	}

	lines, err := exec.OutputLines(c.execCmd("docker", "info", "--format", "{{.Runtimes}}"))
	if err != nil || len(lines) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "Could not query Docker runtimes",
			Reason:   "Docker may not be running or accessible",
			Fix:      "Ensure Docker is running: sudo systemctl start docker",
		}}
	}

	output := strings.Join(lines, " ")
	if !strings.Contains(output, "nvidia") {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "nvidia runtime not configured in Docker",
			Reason:   "The NVIDIA runtime must be registered with Docker for GPU containers",
			Fix:      "Configure: sudo nvidia-ctk runtime configure --runtime=docker && sudo systemctl restart docker",
		}}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  "nvidia runtime configured in Docker",
	}}
}
