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
	"sigs.k8s.io/kind/pkg/internal/version"
)

// localPathCVECheck warns when local-path-provisioner in a running cluster
// is below v0.0.34 (CVE-2025-62878 fix threshold).
type localPathCVECheck struct {
	// getProvisionerVersion returns the local-path-provisioner image tag
	// from the running cluster. Returns ("", nil) if no cluster or no
	// provisioner is running. Injected for testing.
	getProvisionerVersion func() (string, error)
}

// newLocalPathCVECheck creates a localPathCVECheck that detects the running
// provisioner image tag via container exec kubectl and compares against the
// CVE-2025-62878 fix threshold v0.0.34.
func newLocalPathCVECheck() Check {
	return &localPathCVECheck{
		getProvisionerVersion: realGetProvisionerVersion,
	}
}

func (c *localPathCVECheck) Name() string       { return "local-path-cve" }
func (c *localPathCVECheck) Category() string    { return "Cluster" }
func (c *localPathCVECheck) Platforms() []string { return nil }

// realGetProvisionerVersion detects the container runtime, finds the kind
// control-plane container, then execs kubectl inside it to read the
// local-path-provisioner deployment image tag.
// Returns ("", nil) when no runtime, no cluster, or no provisioner is found.
func realGetProvisionerVersion() (string, error) {
	// Find container runtime binary.
	var binaryName string
	for _, rt := range []string{"docker", "podman", "nerdctl"} {
		if _, err := osexec.LookPath(rt); err == nil {
			binaryName = rt
			break
		}
	}
	if binaryName == "" {
		return "", nil // no runtime → skip
	}

	// Find kind control-plane container.
	lines, err := exec.OutputLines(exec.Command(
		binaryName, "ps",
		"--filter", "label=io.x-k8s.kind.role=control-plane",
		"--format", "{{.Names}}",
	))
	if err != nil || len(lines) == 0 {
		return "", nil // no cluster running → skip
	}
	cpName := strings.TrimSpace(lines[0])
	if cpName == "" {
		return "", nil
	}

	// Get provisioner deployment image via kubectl inside the container.
	imgLines, err := exec.OutputLines(exec.Command(
		binaryName, "exec", cpName,
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"get", "deployment", "local-path-provisioner",
		"-n", "local-path-storage",
		"-o", "jsonpath={.spec.template.spec.containers[0].image}",
	))
	if err != nil {
		return "", nil // provisioner not installed → skip
	}
	if len(imgLines) == 0 || strings.TrimSpace(imgLines[0]) == "" {
		return "", nil
	}

	// Extract version tag from image reference (e.g. "rancher/local-path-provisioner:v0.0.35").
	image := strings.TrimSpace(imgLines[0])
	idx := strings.LastIndex(image, ":")
	if idx < 0 {
		return "", fmt.Errorf("no tag in image %q", image)
	}
	return image[idx+1:], nil
}

// Run executes the CVE check:
//  1. Reads the provisioner image tag from the running cluster.
//  2. Skips when no cluster or provisioner is found.
//  3. Warns when the tag cannot be parsed as a semantic version.
//  4. Warns when the version is below v0.0.34 (CVE-2025-62878 threshold).
//  5. Returns ok when the version is v0.0.34 or later.
func (c *localPathCVECheck) Run() []Result {
	tag, err := c.getProvisionerVersion()
	if err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("could not determine local-path-provisioner version: %v", err),
		}}
	}
	if tag == "" {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "local-path-provisioner not detected in running cluster",
		}}
	}

	// Parse tag as semver. Add "v" prefix when absent (e.g. "0.0.35" → "v0.0.35").
	verStr := tag
	if !strings.HasPrefix(verStr, "v") {
		verStr = "v" + verStr
	}
	ver, parseErr := version.ParseSemantic(verStr)
	if parseErr != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("unparseable local-path-provisioner version %q", tag),
		}}
	}

	// CVE-2025-62878 threshold: v0.0.34.
	threshold, _ := version.ParseSemantic("v0.0.34")
	if ver.LessThan(threshold) {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("local-path-provisioner %s is below v0.0.34", tag),
			Reason:   "CVE-2025-62878: versions below v0.0.34 are vulnerable to path traversal",
			Fix:      "Upgrade local-path-provisioner to v0.0.34 or later. Recreate the cluster with kinder v2.2+.",
		}}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  fmt.Sprintf("local-path-provisioner %s (>= v0.0.34, CVE-2025-62878 patched)", tag),
	}}
}
