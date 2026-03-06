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
	"os"
	"path/filepath"
	"runtime"
)

// daemonJSONCheck searches for Docker daemon.json files and warns if
// "init": true is set, which breaks kind nodes.
type daemonJSONCheck struct {
	readFile func(string) ([]byte, error)
	homeDir  func() (string, error)
	goos     string
}

// newDaemonJSONCheck creates a daemonJSONCheck with real system deps.
func newDaemonJSONCheck() Check {
	return &daemonJSONCheck{
		readFile: os.ReadFile,
		homeDir:  os.UserHomeDir,
		goos:     runtime.GOOS,
	}
}

func (c *daemonJSONCheck) Name() string       { return "daemon-json-init" }
func (c *daemonJSONCheck) Category() string    { return "Docker" }
func (c *daemonJSONCheck) Platforms() []string { return nil }

// daemonJSONCandidates returns the prioritized list of daemon.json paths.
func (c *daemonJSONCheck) daemonJSONCandidates() []string {
	home, _ := c.homeDir()
	if home == "" {
		home = "/root"
	}

	candidates := []string{
		"/etc/docker/daemon.json",
		filepath.Join(home, ".docker", "daemon.json"),
	}

	// XDG_CONFIG_HOME or default ~/.config
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	candidates = append(candidates, filepath.Join(xdgConfig, "docker", "daemon.json"))

	candidates = append(candidates,
		"/var/snap/docker/current/config/daemon.json",
		filepath.Join(home, ".rd", "docker", "daemon.json"),
	)

	if c.goos == "windows" {
		candidates = append(candidates, `C:\ProgramData\docker\config\daemon.json`)
	}

	return candidates
}

func (c *daemonJSONCheck) Run() []Result {
	candidates := c.daemonJSONCandidates()

	for _, path := range candidates {
		data, err := c.readFile(path)
		if err != nil {
			continue // file doesn't exist or is unreadable, try next
		}

		// File found -- parse it.
		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			return []Result{{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "warn",
				Message:  fmt.Sprintf("daemon.json at %s is not valid JSON", path),
				Reason:   "Docker may not start with invalid configuration",
				Fix:      fmt.Sprintf("Fix JSON syntax in %s", path),
			}}
		}

		// Check for "init": true.
		if initVal, ok := config["init"]; ok {
			if boolVal, isBool := initVal.(bool); isBool && boolVal {
				return []Result{{
					Name:     c.Name(),
					Category: c.Category(),
					Status:   "warn",
					Message:  fmt.Sprintf("daemon.json at %s has init=true", path),
					Reason:   `Docker init=true breaks kind nodes with: Couldn't find an alternative telinit implementation to spawn`,
					Fix:      fmt.Sprintf(`Remove "init": true from %s and restart Docker`, path),
				}}
			}
		}

		// File parsed, no init:true found.
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  fmt.Sprintf("daemon.json checked (%s)", path),
		}}
	}

	// No candidate was readable.
	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "ok",
		Message:  "No daemon.json found (Docker using defaults)",
	}}
}
