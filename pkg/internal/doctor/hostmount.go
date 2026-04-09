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
	"strings"
)

// hostMountPathCheck verifies that configured host mount paths exist on disk.
// It follows the localPathCVECheck pattern with injected dependencies for testing.
type hostMountPathCheck struct {
	getMountPaths func() []string
	statPath      func(string) (os.FileInfo, error)
}

// newHostMountPathCheck creates a hostMountPathCheck with real system dependencies.
func newHostMountPathCheck() Check {
	return &hostMountPathCheck{
		getMountPaths: func() []string { return nil },
		statPath:      os.Stat,
	}
}

func (c *hostMountPathCheck) Name() string       { return "host-mount-path" }
func (c *hostMountPathCheck) Category() string    { return "Mounts" }
func (c *hostMountPathCheck) Platforms() []string { return nil }

// Run checks each configured mount path for existence and accessibility.
// Returns a skip result when no mount paths are configured.
func (c *hostMountPathCheck) Run() []Result {
	paths := c.getMountPaths()
	if len(paths) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "no host mount paths configured",
		}}
	}

	var results []Result
	for _, path := range paths {
		info, err := c.statPath(path)
		if err == nil {
			_ = info
			results = append(results, Result{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "ok",
				Message:  fmt.Sprintf("host mount path exists: %s", path),
			})
			continue
		}
		if os.IsNotExist(err) {
			results = append(results, Result{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "fail",
				Message:  fmt.Sprintf("host mount path does not exist: %s", path),
				Reason:   "The directory must exist on the host before it can be mounted into a kind cluster node.",
				Fix:      fmt.Sprintf("mkdir -p %s", path),
			})
		} else {
			results = append(results, Result{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "warn",
				Message:  fmt.Sprintf("host mount path inaccessible: %s (%v)", path, err),
				Reason:   "The path exists but cannot be accessed; the mount may fail at cluster creation.",
			})
		}
	}
	return results
}

// dockerDesktopFileSharingCheck verifies that configured host mount paths are
// covered by Docker Desktop file-sharing directories on macOS.
// It reads ~/Library/Group Containers/group.com.docker/settings-store.json.
type dockerDesktopFileSharingCheck struct {
	readFile      func(string) ([]byte, error)
	homeDir       func() (string, error)
	getMountPaths func() []string
}

// newDockerDesktopFileSharingCheck creates a dockerDesktopFileSharingCheck with
// real system dependencies.
func newDockerDesktopFileSharingCheck() Check {
	return &dockerDesktopFileSharingCheck{
		readFile:      os.ReadFile,
		homeDir:       os.UserHomeDir,
		getMountPaths: func() []string { return nil },
	}
}

func (c *dockerDesktopFileSharingCheck) Name() string       { return "docker-desktop-file-sharing" }
func (c *dockerDesktopFileSharingCheck) Category() string    { return "Mounts" }
func (c *dockerDesktopFileSharingCheck) Platforms() []string { return []string{"darwin"} }

// defaultFileSharingDirs are the directories Docker Desktop shares by default.
var defaultFileSharingDirs = []string{
	"/Users",
	"/Volumes",
	"/private",
	"/tmp",
	"/var/folders",
}

// dockerDesktopSettings is the JSON structure of settings-store.json.
type dockerDesktopSettings struct {
	FilesharingDirectories []string `json:"filesharingDirectories"`
}

// Run checks that each configured mount path is covered by Docker Desktop
// file-sharing directories. Returns a skip result when no mount paths are
// configured.
func (c *dockerDesktopFileSharingCheck) Run() []Result {
	paths := c.getMountPaths()
	if len(paths) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "no host mount paths configured",
		}}
	}

	sharedDirs := c.readSharedDirs()

	var results []Result
	for _, path := range paths {
		if isPathCovered(path, sharedDirs) {
			results = append(results, Result{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "ok",
				Message:  fmt.Sprintf("host mount path covered by Docker Desktop file sharing: %s", path),
			})
		} else {
			results = append(results, Result{
				Name:     c.Name(),
				Category: c.Category(),
				Status:   "warn",
				Message:  fmt.Sprintf("host mount path not covered by Docker Desktop file sharing: %s", path),
				Reason:   "Docker Desktop only mounts paths listed under Preferences > Resources > File Sharing.",
				Fix:      fmt.Sprintf("Add %s to Docker Desktop file sharing: Preferences > Resources > File Sharing", path),
			})
		}
	}
	return results
}

// readSharedDirs reads the Docker Desktop file-sharing directories from
// settings-store.json. Falls back to defaultFileSharingDirs if the file
// cannot be read or does not contain the key.
func (c *dockerDesktopFileSharingCheck) readSharedDirs() []string {
	home, err := c.homeDir()
	if err != nil || home == "" {
		return defaultFileSharingDirs
	}

	settingsPath := filepath.Join(
		home,
		"Library",
		"Group Containers",
		"group.com.docker",
		"settings-store.json",
	)

	data, err := c.readFile(settingsPath)
	if err != nil {
		return defaultFileSharingDirs
	}

	var settings dockerDesktopSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaultFileSharingDirs
	}

	if len(settings.FilesharingDirectories) == 0 {
		return defaultFileSharingDirs
	}

	return settings.FilesharingDirectories
}

// isPathCovered reports whether path is equal to or a subdirectory of any
// directory in sharedDirs.
func isPathCovered(path string, sharedDirs []string) bool {
	for _, dir := range sharedDirs {
		if path == dir || strings.HasPrefix(path, dir+"/") {
			return true
		}
	}
	return false
}
