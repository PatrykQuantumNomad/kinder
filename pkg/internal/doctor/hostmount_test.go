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
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestHostMountPathCheck covers the hostMountPathCheck.Run() logic.
func TestHostMountPathCheck(t *testing.T) {
	t.Parallel()

	realStat := os.Stat

	tests := []struct {
		name           string
		getMountPaths  func() []string
		statPath       func(string) (os.FileInfo, error)
		wantLen        int
		wantStatuses   []string
		wantMsgContain []string
		wantFixContain []string
	}{
		{
			name:          "skip when nil paths",
			getMountPaths: func() []string { return nil },
			statPath:      realStat,
			wantLen:       1,
			wantStatuses:  []string{"skip"},
			wantMsgContain: []string{"no host mount paths configured"},
		},
		{
			name:          "skip when empty paths",
			getMountPaths: func() []string { return []string{} },
			statPath:      realStat,
			wantLen:       1,
			wantStatuses:  []string{"skip"},
			wantMsgContain: []string{"no host mount paths configured"},
		},
		{
			name:          "ok when path exists",
			getMountPaths: func() []string { return []string{t.TempDir()} },
			statPath:      realStat,
			wantLen:       1,
			wantStatuses:  []string{"ok"},
			wantMsgContain: []string{"host mount path exists"},
		},
		{
			name:          "fail when path missing",
			getMountPaths: func() []string { return []string{"/nonexistent/kinder/test/path"} },
			statPath:      realStat,
			wantLen:       1,
			wantStatuses:  []string{"fail"},
			wantMsgContain: []string{"does not exist"},
			wantFixContain: []string{"mkdir -p"},
		},
		{
			name:          "warn when inaccessible",
			getMountPaths: func() []string { return []string{"/some/path"} },
			statPath: func(p string) (os.FileInfo, error) {
				return nil, fmt.Errorf("permission denied")
			},
			wantLen:       1,
			wantStatuses:  []string{"warn"},
			wantMsgContain: []string{"inaccessible"},
		},
		{
			name: "mixed results existing and missing",
			getMountPaths: func() []string {
				return []string{t.TempDir(), "/nonexistent/kinder/test/missing"}
			},
			statPath:      realStat,
			wantLen:       2,
			wantStatuses:  []string{"ok", "fail"},
			wantMsgContain: []string{"host mount path exists", "does not exist"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &hostMountPathCheck{
				getMountPaths: tt.getMountPaths,
				statPath:      tt.statPath,
			}
			results := check.Run()

			if len(results) != tt.wantLen {
				t.Fatalf("len(results) = %d, want %d", len(results), tt.wantLen)
			}
			for i, r := range results {
				if r.Name != "host-mount-path" {
					t.Errorf("results[%d].Name = %q, want %q", i, r.Name, "host-mount-path")
				}
				if r.Category != "Mounts" {
					t.Errorf("results[%d].Category = %q, want %q", i, r.Category, "Mounts")
				}
				if i < len(tt.wantStatuses) && r.Status != tt.wantStatuses[i] {
					t.Errorf("results[%d].Status = %q, want %q", i, r.Status, tt.wantStatuses[i])
				}
				if i < len(tt.wantMsgContain) && tt.wantMsgContain[i] != "" {
					if !strings.Contains(r.Message, tt.wantMsgContain[i]) {
						t.Errorf("results[%d].Message = %q, want to contain %q", i, r.Message, tt.wantMsgContain[i])
					}
				}
				if i < len(tt.wantFixContain) && tt.wantFixContain[i] != "" {
					if !strings.Contains(r.Fix, tt.wantFixContain[i]) {
						t.Errorf("results[%d].Fix = %q, want to contain %q", i, r.Fix, tt.wantFixContain[i])
					}
				}
			}
		})
	}
}

// TestDockerDesktopFileSharingCheck covers the dockerDesktopFileSharingCheck.Run() logic.
func TestDockerDesktopFileSharingCheck(t *testing.T) {
	t.Parallel()

	settingsWithDirs := func(dirs []string) []byte {
		data := `{"filesharingDirectories":[`
		for i, d := range dirs {
			if i > 0 {
				data += ","
			}
			data += fmt.Sprintf("%q", d)
		}
		data += `]}`
		return []byte(data)
	}

	tests := []struct {
		name           string
		getMountPaths  func() []string
		readFile       func(string) ([]byte, error)
		homeDir        func() (string, error)
		wantLen        int
		wantStatuses   []string
		wantMsgContain []string
	}{
		{
			name:          "skip when nil paths",
			getMountPaths: func() []string { return nil },
			readFile:      os.ReadFile,
			homeDir:       os.UserHomeDir,
			wantLen:       1,
			wantStatuses:  []string{"skip"},
			wantMsgContain: []string{"no host mount paths configured"},
		},
		{
			name:          "ok when path covered",
			getMountPaths: func() []string { return []string{"/Users/dev/project"} },
			readFile: func(string) ([]byte, error) {
				return settingsWithDirs([]string{"/Users", "/Volumes"}), nil
			},
			homeDir: func() (string, error) { return "/Users/dev", nil },
			wantLen: 1,
			wantStatuses:  []string{"ok"},
			wantMsgContain: []string{"covered by Docker Desktop file sharing"},
		},
		{
			name:          "warn when not covered",
			getMountPaths: func() []string { return []string{"/opt/data"} },
			readFile: func(string) ([]byte, error) {
				return settingsWithDirs([]string{"/Users", "/Volumes"}), nil
			},
			homeDir: func() (string, error) { return "/Users/dev", nil },
			wantLen: 1,
			wantStatuses:  []string{"warn"},
			wantMsgContain: []string{"not covered by Docker Desktop file sharing"},
		},
		{
			name:          "defaults when key missing from settings",
			getMountPaths: func() []string { return []string{"/tmp/kinder-test"} },
			readFile: func(string) ([]byte, error) {
				return []byte(`{"otherKey": "value"}`), nil
			},
			homeDir: func() (string, error) { return "/Users/dev", nil },
			wantLen: 1,
			// /tmp is in defaultFileSharingDirs so this should be ok
			wantStatuses:  []string{"ok"},
			wantMsgContain: []string{"covered"},
		},
		{
			name:          "warn when settings unreadable",
			getMountPaths: func() []string { return []string{"/opt/data"} },
			readFile: func(string) ([]byte, error) {
				return nil, errors.New("file not found")
			},
			homeDir: func() (string, error) { return "/Users/dev", nil },
			wantLen: 1,
			// /opt/data is not in defaultFileSharingDirs
			wantStatuses:  []string{"warn"},
			wantMsgContain: []string{"not covered"},
		},
		{
			name:          "prefix false positive guard /Userspace vs /Users",
			getMountPaths: func() []string { return []string{"/Userspace/data"} },
			readFile: func(string) ([]byte, error) {
				return settingsWithDirs([]string{"/Users", "/Volumes"}), nil
			},
			homeDir: func() (string, error) { return "/Users/dev", nil },
			wantLen: 1,
			// /Userspace/data should NOT be covered by /Users
			wantStatuses:  []string{"warn"},
			wantMsgContain: []string{"not covered"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &dockerDesktopFileSharingCheck{
				readFile:      tt.readFile,
				homeDir:       tt.homeDir,
				getMountPaths: tt.getMountPaths,
			}
			results := check.Run()

			if len(results) != tt.wantLen {
				t.Fatalf("len(results) = %d, want %d", len(results), tt.wantLen)
			}
			for i, r := range results {
				if r.Name != "docker-desktop-file-sharing" {
					t.Errorf("results[%d].Name = %q, want %q", i, r.Name, "docker-desktop-file-sharing")
				}
				if r.Category != "Mounts" {
					t.Errorf("results[%d].Category = %q, want %q", i, r.Category, "Mounts")
				}
				if i < len(tt.wantStatuses) && r.Status != tt.wantStatuses[i] {
					t.Errorf("results[%d].Status = %q, want %q", i, r.Status, tt.wantStatuses[i])
				}
				if i < len(tt.wantMsgContain) && tt.wantMsgContain[i] != "" {
					if !strings.Contains(r.Message, tt.wantMsgContain[i]) {
						t.Errorf("results[%d].Message = %q, want to contain %q", i, r.Message, tt.wantMsgContain[i])
					}
				}
			}
		})
	}
}

// TestHostMountPathCheck_SetMountPaths tests the setMountPaths method on
// hostMountPathCheck directly.
func TestHostMountPathCheck_SetMountPaths(t *testing.T) {
	t.Parallel()

	// Create a check with a fake statPath that returns not-exist for everything.
	check := &hostMountPathCheck{
		getMountPaths: func() []string { return nil },
		statPath: func(p string) (os.FileInfo, error) {
			return nil, &os.PathError{Op: "stat", Path: p, Err: os.ErrNotExist}
		},
	}

	// Before setMountPaths: Run returns skip.
	results := check.Run()
	if len(results) != 1 || results[0].Status != "skip" {
		t.Fatalf("before setMountPaths: want 1 skip result, got %v", results)
	}

	// After setMountPaths: Run returns non-skip results.
	check.setMountPaths([]string{"/data/test"})
	results = check.Run()
	if len(results) == 0 {
		t.Fatal("after setMountPaths: expected non-empty results")
	}
	if results[0].Status == "skip" {
		t.Errorf("after setMountPaths: expected non-skip status, got skip")
	}

	// Calling setMountPaths(nil) restores skip behavior.
	check.setMountPaths(nil)
	results = check.Run()
	if len(results) != 1 || results[0].Status != "skip" {
		t.Errorf("after setMountPaths(nil): want skip, got %v", results)
	}
}

// TestDockerDesktopFileSharingCheck_SetMountPaths tests the setMountPaths method on
// dockerDesktopFileSharingCheck directly.
func TestDockerDesktopFileSharingCheck_SetMountPaths(t *testing.T) {
	t.Parallel()

	check := &dockerDesktopFileSharingCheck{
		getMountPaths: func() []string { return nil },
		readFile: func(string) ([]byte, error) {
			return []byte(`{"filesharingDirectories":["/Users","/Volumes","/private","/tmp"]}`), nil
		},
		homeDir: func() (string, error) { return "/Users/dev", nil },
	}

	// Before setMountPaths: Run returns skip.
	results := check.Run()
	if len(results) != 1 || results[0].Status != "skip" {
		t.Fatalf("before setMountPaths: want 1 skip result, got %v", results)
	}

	// After setMountPaths with a covered path: ok result.
	check.setMountPaths([]string{"/Users/dev/project"})
	results = check.Run()
	if len(results) == 0 {
		t.Fatal("after setMountPaths: expected non-empty results")
	}
	if results[0].Status == "skip" {
		t.Errorf("after setMountPaths: expected non-skip status, got skip")
	}

	// Calling setMountPaths(nil) restores skip behavior.
	check.setMountPaths(nil)
	results = check.Run()
	if len(results) != 1 || results[0].Status != "skip" {
		t.Errorf("after setMountPaths(nil): want skip, got %v", results)
	}
}

// TestSetMountPaths tests the exported SetMountPaths function that injects paths
// into all mountPathConfigurable checks via the global allChecks registry.
func TestSetMountPaths(t *testing.T) {
	// Not parallel: manipulates global allChecks.

	// Save and restore allChecks.
	original := allChecks
	defer func() { allChecks = original }()

	// Create fresh mount checks with injected dependencies so the test
	// doesn't rely on the filesystem.
	hostCheck := &hostMountPathCheck{
		getMountPaths: func() []string { return nil },
		statPath: func(p string) (os.FileInfo, error) {
			return nil, &os.PathError{Op: "stat", Path: p, Err: os.ErrNotExist}
		},
	}
	ddCheck := &dockerDesktopFileSharingCheck{
		getMountPaths: func() []string { return nil },
		readFile: func(string) ([]byte, error) {
			// Return dirs that don't include /data/test so we get a warn.
			return []byte(`{"filesharingDirectories":["/Users","/Volumes"]}`), nil
		},
		homeDir: func() (string, error) { return "/Users/dev", nil },
	}

	// Replace allChecks with just the mount checks for isolation.
	allChecks = []Check{hostCheck, ddCheck}

	// Before SetMountPaths: both checks should skip.
	for _, c := range allChecks {
		results := c.Run()
		if len(results) != 1 || results[0].Status != "skip" {
			t.Errorf("before SetMountPaths: check %s: want skip, got %v", c.Name(), results)
		}
	}

	// After SetMountPaths: both checks should produce non-skip results.
	SetMountPaths([]string{"/data/test"})
	for _, c := range allChecks {
		results := c.Run()
		if len(results) == 0 {
			t.Errorf("after SetMountPaths: check %s: expected non-empty results", c.Name())
			continue
		}
		if results[0].Status == "skip" {
			t.Errorf("after SetMountPaths: check %s: expected non-skip, got skip", c.Name())
		}
	}

	// SetMountPaths(nil) restores skip behavior.
	SetMountPaths(nil)
	for _, c := range allChecks {
		results := c.Run()
		if len(results) != 1 || results[0].Status != "skip" {
			t.Errorf("after SetMountPaths(nil): check %s: want skip, got %v", c.Name(), results)
		}
	}
}

// TestIsPathCovered covers the isPathCovered helper.
func TestIsPathCovered(t *testing.T) {
	t.Parallel()

	sharedDirs := []string{"/Users", "/Volumes", "/private"}

	tests := []struct {
		path string
		want bool
	}{
		{"/Users/dev/project", true},        // subdirectory match
		{"/Users", true},                    // exact match
		{"/Volumes/data", true},             // subdirectory of second entry
		{"/opt/data", false},                // not covered
		{"/Userspace/data", false},          // prefix false positive guard
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := isPathCovered(tt.path, sharedDirs)
			if got != tt.want {
				t.Errorf("isPathCovered(%q, ...) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
