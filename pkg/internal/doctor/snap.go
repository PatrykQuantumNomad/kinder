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
	"path/filepath"
	"strings"
)

// dockerSnapCheck detects whether Docker is installed via snap, which
// causes TMPDIR issues with kind.
type dockerSnapCheck struct {
	lookPath     func(string) (string, error)
	evalSymlinks func(string) (string, error)
}

// newDockerSnapCheck creates a dockerSnapCheck with real system deps.
func newDockerSnapCheck() Check {
	return &dockerSnapCheck{
		lookPath:     osexec.LookPath,
		evalSymlinks: filepath.EvalSymlinks,
	}
}

func (c *dockerSnapCheck) Name() string       { return "docker-snap" }
func (c *dockerSnapCheck) Category() string    { return "Docker" }
func (c *dockerSnapCheck) Platforms() []string { return []string{"linux"} }

func (c *dockerSnapCheck) Run() []Result {
	dockerPath, err := c.lookPath("docker")
	if err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "(docker not found)",
		}}
	}

	// Resolve symlinks to find the actual binary location.
	resolved, err := c.evalSymlinks(dockerPath)
	if err != nil {
		// Fall back to the original path for the snap check.
		resolved = dockerPath
	}

	if strings.Contains(resolved, "/snap/") {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("Docker installed via snap (%s)", resolved),
			Reason:   "Snap Docker uses a confined TMPDIR which can break kind cluster creation",
			Fix:      "Set TMPDIR to a snap-accessible directory: export TMPDIR=$HOME/tmp && mkdir -p $TMPDIR",
		}}
	}

	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  "Docker not installed via snap",
	}}
}
