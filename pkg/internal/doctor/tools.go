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

	"sigs.k8s.io/kind/pkg/exec"
)

// kubectlCheck verifies that kubectl is installed and responds to version --client.
type kubectlCheck struct {
	lookPath func(string) (string, error)
	execCmd  func(name string, args ...string) exec.Cmd
}

// newKubectlCheck creates a kubectlCheck with real system deps.
func newKubectlCheck() Check {
	return &kubectlCheck{
		lookPath: osexec.LookPath,
		execCmd:  exec.Command,
	}
}

func (c *kubectlCheck) Name() string       { return "kubectl" }
func (c *kubectlCheck) Category() string    { return "Tools" }
func (c *kubectlCheck) Platforms() []string { return nil }

func (c *kubectlCheck) Run() []Result {
	if _, err := c.lookPath("kubectl"); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "fail",
			Message:  "kubectl not found",
			Reason:   "kubectl is required to interact with the cluster",
			Fix:      "Install kubectl: https://kubernetes.io/docs/tasks/tools/",
		}}
	}
	// kubectl found -- check if version --client works.
	lines, err := exec.OutputLines(c.execCmd("kubectl", "version", "--client"))
	if err != nil || len(lines) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "kubectl found but version check failed",
			Reason:   "kubectl version --client did not produce output -- check your installation",
			Fix:      "Reinstall kubectl: https://kubernetes.io/docs/tasks/tools/",
		}}
	}
	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  "kubectl available",
	}}
}
