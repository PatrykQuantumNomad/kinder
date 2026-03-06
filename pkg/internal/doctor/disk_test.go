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
	"strings"
	"testing"
)

func TestDiskSpaceCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newDiskSpaceCheck()
	if check.Name() != "disk-space" {
		t.Errorf("Name() = %q, want %q", check.Name(), "disk-space")
	}
	if check.Category() != "Docker" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Docker")
	}
	if check.Platforms() != nil {
		t.Errorf("Platforms() = %v, want nil (all platforms)", check.Platforms())
	}
}

func TestDiskSpaceCheck_Run(t *testing.T) {
	t.Parallel()

	const (
		gb = 1024 * 1024 * 1024
	)

	tests := []struct {
		name              string
		readDiskFree      func(string) (uint64, error)
		execOutput        map[string]fakeExecResult
		wantStatus        string
		wantMsgContain    string
		wantReasonContain string
		wantFixContain    string
	}{
		{
			name: "10GB free - ok",
			readDiskFree: func(path string) (uint64, error) {
				return 10 * gb, nil
			},
			execOutput: map[string]fakeExecResult{
				"docker info --format {{.DockerRootDir}}": {lines: "/var/lib/docker\n"},
			},
			wantStatus:     "ok",
			wantMsgContain: "GB free",
		},
		{
			name: "4GB free - warn",
			readDiskFree: func(path string) (uint64, error) {
				return 4 * gb, nil
			},
			execOutput: map[string]fakeExecResult{
				"docker info --format {{.DockerRootDir}}": {lines: "/var/lib/docker\n"},
			},
			wantStatus:        "warn",
			wantMsgContain:    "4.0 GB free",
			wantReasonContain: "Low disk space",
			wantFixContain:    "docker system prune",
		},
		{
			name: "1GB free - fail",
			readDiskFree: func(path string) (uint64, error) {
				return 1 * gb, nil
			},
			execOutput: map[string]fakeExecResult{
				"docker info --format {{.DockerRootDir}}": {lines: "/var/lib/docker\n"},
			},
			wantStatus:        "fail",
			wantMsgContain:    "1.0 GB free",
			wantReasonContain: "Insufficient",
			wantFixContain:    "docker image prune",
		},
		{
			name: "readDiskFree errors on data root and fallback - warn",
			readDiskFree: func(path string) (uint64, error) {
				return 0, errors.New("statfs failed")
			},
			execOutput: map[string]fakeExecResult{
				"docker info --format {{.DockerRootDir}}": {lines: "/var/lib/docker\n"},
			},
			wantStatus:     "warn",
			wantMsgContain: "Could not check disk space",
		},
		{
			name: "docker info returns custom data root path",
			readDiskFree: func(path string) (uint64, error) {
				if path == "/mnt/data/docker" {
					return 10 * gb, nil
				}
				return 0, errors.New("wrong path")
			},
			execOutput: map[string]fakeExecResult{
				"docker info --format {{.DockerRootDir}}": {lines: "/mnt/data/docker\n"},
			},
			wantStatus:     "ok",
			wantMsgContain: "/mnt/data/docker",
		},
		{
			name: "docker info fails - falls back to default path",
			readDiskFree: func(path string) (uint64, error) {
				if path == "/var/lib/docker" {
					return 10 * gb, nil
				}
				return 0, errors.New("wrong path")
			},
			execOutput: map[string]fakeExecResult{
				"docker info --format {{.DockerRootDir}}": {err: errors.New("docker not running")},
			},
			wantStatus:     "ok",
			wantMsgContain: "/var/lib/docker",
		},
		{
			name: "readDiskFree errors on data root but succeeds on /",
			readDiskFree: func(path string) (uint64, error) {
				if path == "/" {
					return 10 * gb, nil
				}
				return 0, errors.New("path not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info --format {{.DockerRootDir}}": {lines: "/var/lib/docker\n"},
			},
			wantStatus:     "ok",
			wantMsgContain: "GB free",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &diskSpaceCheck{
				readDiskFree: tt.readDiskFree,
				execCmd:      newFakeExecCmd(tt.execOutput),
			}
			results := check.Run()
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			r := results[0]
			if r.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", r.Status, tt.wantStatus)
			}
			if r.Category != "Docker" {
				t.Errorf("Category = %q, want %q", r.Category, "Docker")
			}
			if r.Name != "disk-space" {
				t.Errorf("Name = %q, want %q", r.Name, "disk-space")
			}
			if tt.wantMsgContain != "" && !strings.Contains(r.Message, tt.wantMsgContain) {
				t.Errorf("Message = %q, want to contain %q", r.Message, tt.wantMsgContain)
			}
			if tt.wantReasonContain != "" && !strings.Contains(r.Reason, tt.wantReasonContain) {
				t.Errorf("Reason = %q, want to contain %q", r.Reason, tt.wantReasonContain)
			}
			if tt.wantFixContain != "" && !strings.Contains(r.Fix, tt.wantFixContain) {
				t.Errorf("Fix = %q, want to contain %q", r.Fix, tt.wantFixContain)
			}
		})
	}
}
