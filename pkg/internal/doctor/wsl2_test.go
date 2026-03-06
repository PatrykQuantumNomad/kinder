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
	"os"
	"strings"
	"testing"
)

func TestWSL2Check_Metadata(t *testing.T) {
	t.Parallel()
	check := newWSL2Check()
	if check.Name() != "wsl2-cgroup" {
		t.Errorf("Name() = %q, want %q", check.Name(), "wsl2-cgroup")
	}
	if check.Category() != "Platform" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Platform")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestWSL2Check_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		files           map[string]string
		fileErrors      map[string]error
		env             map[string]string
		statResults     map[string]bool
		wantStatus      string
		wantMsgContain  string
		wantResultCount int
	}{
		{
			name: "WSL2 with WSL_DISTRO_NAME and cgroup controllers present",
			files: map[string]string{
				"/proc/version":                      "Linux version 5.15.146.1-microsoft-standard-WSL2",
				"/sys/fs/cgroup/cgroup.controllers":  "cpu memory pids io",
			},
			env: map[string]string{
				"WSL_DISTRO_NAME": "Ubuntu",
			},
			statResults:     map[string]bool{},
			wantStatus:      "ok",
			wantMsgContain:  "cpu",
			wantResultCount: 1,
		},
		{
			name: "WSL2 with Microsoft case variant and WSL_DISTRO_NAME",
			files: map[string]string{
				"/proc/version":                      "Linux version 5.15.146.1-Microsoft-standard-WSL2",
				"/sys/fs/cgroup/cgroup.controllers":  "cpu memory pids",
			},
			env: map[string]string{
				"WSL_DISTRO_NAME": "Ubuntu",
			},
			statResults:     map[string]bool{},
			wantStatus:      "ok",
			wantMsgContain:  "cpu",
			wantResultCount: 1,
		},
		{
			name: "WSL2 with WSLInterop file exists",
			files: map[string]string{
				"/proc/version":                      "Linux version 5.15.146.1-microsoft-standard-WSL2",
				"/sys/fs/cgroup/cgroup.controllers":  "cpu memory pids io",
			},
			env: map[string]string{},
			statResults: map[string]bool{
				"/proc/sys/fs/binfmt_misc/WSLInterop": true,
			},
			wantStatus:      "ok",
			wantMsgContain:  "cpu",
			wantResultCount: 1,
		},
		{
			name: "WSL2 with WSLInterop-late file exists (WSL 4.1.4+)",
			files: map[string]string{
				"/proc/version":                      "Linux version 5.15.146.1-microsoft-standard-WSL2",
				"/sys/fs/cgroup/cgroup.controllers":  "cpu memory pids io",
			},
			env: map[string]string{},
			statResults: map[string]bool{
				"/proc/sys/fs/binfmt_misc/WSLInterop-late": true,
			},
			wantStatus:      "ok",
			wantMsgContain:  "cpu",
			wantResultCount: 1,
		},
		{
			name: "Azure VM - microsoft in /proc/version but no second signal",
			files: map[string]string{
				"/proc/version": "Linux version 6.8.0-1025-azure (buildd@lcy02-amd64-115)",
			},
			env:             map[string]string{},
			statResults:     map[string]bool{},
			wantStatus:      "ok",
			wantMsgContain:  "Not running under WSL2",
			wantResultCount: 1,
		},
		{
			name: "Not WSL2 - no microsoft in /proc/version",
			files: map[string]string{
				"/proc/version": "Linux version 6.5.0-44-generic (buildd@lcy02-amd64-072)",
			},
			env:             map[string]string{},
			statResults:     map[string]bool{},
			wantStatus:      "ok",
			wantMsgContain:  "Not running under WSL2",
			wantResultCount: 1,
		},
		{
			name:            "/proc/version unreadable",
			files:           map[string]string{},
			fileErrors:      map[string]error{"/proc/version": errors.New("no such file")},
			env:             map[string]string{},
			statResults:     map[string]bool{},
			wantStatus:      "ok",
			wantMsgContain:  "Not running under WSL2",
			wantResultCount: 1,
		},
		{
			name: "WSL2 confirmed but cgroup.controllers unreadable",
			files: map[string]string{
				"/proc/version": "Linux version 5.15.146.1-microsoft-standard-WSL2",
			},
			fileErrors: map[string]error{
				"/sys/fs/cgroup/cgroup.controllers": errors.New("no such file"),
			},
			env: map[string]string{
				"WSL_DISTRO_NAME": "Ubuntu",
			},
			statResults:     map[string]bool{},
			wantStatus:      "warn",
			wantMsgContain:  "cgroup v2 controllers not available",
			wantResultCount: 1,
		},
		{
			name: "WSL2 confirmed but cgroup.controllers missing cpu",
			files: map[string]string{
				"/proc/version":                     "Linux version 5.15.146.1-microsoft-standard-WSL2",
				"/sys/fs/cgroup/cgroup.controllers":  "memory pids io",
			},
			env: map[string]string{
				"WSL_DISTRO_NAME": "Ubuntu",
			},
			statResults:     map[string]bool{},
			wantStatus:      "warn",
			wantMsgContain:  "cpu",
			wantResultCount: 1,
		},
		{
			name: "WSL2 confirmed but cgroup.controllers missing memory and pids",
			files: map[string]string{
				"/proc/version":                     "Linux version 5.15.146.1-microsoft-standard-WSL2",
				"/sys/fs/cgroup/cgroup.controllers":  "cpu io",
			},
			env: map[string]string{
				"WSL_DISTRO_NAME": "Ubuntu",
			},
			statResults:     map[string]bool{},
			wantStatus:      "warn",
			wantMsgContain:  "memory",
			wantResultCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			readFile := func(path string) ([]byte, error) {
				if tt.fileErrors != nil {
					if err, ok := tt.fileErrors[path]; ok {
						return nil, err
					}
				}
				if content, ok := tt.files[path]; ok {
					return []byte(content), nil
				}
				return nil, errors.New("file not found: " + path)
			}

			getenv := func(key string) string {
				if tt.env != nil {
					return tt.env[key]
				}
				return ""
			}

			stat := func(path string) (os.FileInfo, error) {
				if tt.statResults != nil {
					if exists, ok := tt.statResults[path]; ok && exists {
						return nil, nil // non-nil FileInfo not needed; only err==nil matters
					}
				}
				return nil, errors.New("stat: no such file: " + path)
			}

			check := &wsl2Check{
				readFile: readFile,
				getenv:   getenv,
				stat:     stat,
			}

			results := check.Run()
			if len(results) != tt.wantResultCount {
				t.Fatalf("expected %d result(s), got %d: %+v", tt.wantResultCount, len(results), results)
			}
			r := results[0]
			if r.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q (Message: %q)", r.Status, tt.wantStatus, r.Message)
			}
			if r.Category != "Platform" {
				t.Errorf("Category = %q, want %q", r.Category, "Platform")
			}
			if tt.wantMsgContain != "" && !strings.Contains(r.Message, tt.wantMsgContain) {
				t.Errorf("Message = %q, want to contain %q", r.Message, tt.wantMsgContain)
			}
		})
	}
}
