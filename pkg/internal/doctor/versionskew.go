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
	"encoding/json"
	"fmt"
	osexec "os/exec"
	"strings"

	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/version"
)

// referenceK8sMinor is the Kubernetes minor version that the default kind
// node image ships. Update this when bumping the default kind node image version.
const referenceK8sMinor uint = 31

// kubectlVersionOutput models the JSON output of "kubectl version --client -o json".
type kubectlVersionOutput struct {
	ClientVersion struct {
		GitVersion string `json:"gitVersion"`
	} `json:"clientVersion"`
}

// kubectlVersionSkewCheck warns when the installed kubectl is more than
// one minor version away from the reference Kubernetes version.
type kubectlVersionSkewCheck struct {
	lookPath func(string) (string, error)
	execCmd  func(name string, args ...string) exec.Cmd
}

// newKubectlVersionSkewCheck creates a kubectlVersionSkewCheck with real system deps.
func newKubectlVersionSkewCheck() Check {
	return &kubectlVersionSkewCheck{
		lookPath: osexec.LookPath,
		execCmd:  exec.Command,
	}
}

func (c *kubectlVersionSkewCheck) Name() string       { return "kubectl-version-skew" }
func (c *kubectlVersionSkewCheck) Category() string    { return "Tools" }
func (c *kubectlVersionSkewCheck) Platforms() []string { return nil }

func (c *kubectlVersionSkewCheck) Run() []Result {
	if _, err := c.lookPath("kubectl"); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "(kubectl not found)",
		}}
	}

	lines, err := exec.CombinedOutputLines(c.execCmd("kubectl", "version", "--client", "-o", "json"))
	if err != nil || len(lines) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "Could not determine kubectl version",
			Reason:   "kubectl version --client -o json failed or produced no output",
			Fix:      "Reinstall kubectl: https://kubernetes.io/docs/tasks/tools/",
		}}
	}

	var out kubectlVersionOutput
	if err := json.Unmarshal([]byte(strings.Join(lines, "\n")), &out); err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "Could not parse kubectl version JSON",
			Reason:   fmt.Sprintf("JSON parse error: %v", err),
			Fix:      "Reinstall kubectl: https://kubernetes.io/docs/tasks/tools/",
		}}
	}

	gitVersion := out.ClientVersion.GitVersion
	clientVer, err := version.ParseSemantic(gitVersion)
	if err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("Could not parse kubectl version %q", gitVersion),
			Reason:   fmt.Sprintf("Version parse error: %v", err),
			Fix:      "Reinstall kubectl: https://kubernetes.io/docs/tasks/tools/",
		}}
	}

	clientMinor := clientVer.Minor()
	diff := int(clientMinor) - int(referenceK8sMinor)
	if diff < 0 {
		diff = -diff
	}

	if diff > 1 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("kubectl %s (client minor %d, reference %d)", gitVersion, clientMinor, referenceK8sMinor),
			Reason:   fmt.Sprintf("kubectl version skew exceeds +/-1 minor version from Kubernetes 1.%d", referenceK8sMinor),
			Fix:      "Update kubectl: https://kubernetes.io/docs/tasks/tools/",
		}}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  fmt.Sprintf("kubectl version compatible (%s)", gitVersion),
	}}
}
