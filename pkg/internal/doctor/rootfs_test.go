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

func TestRootfsDeviceCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newRootfsDeviceCheck()
	if check.Name() != "rootfs-device" {
		t.Errorf("Name() = %q, want %q", check.Name(), "rootfs-device")
	}
	if check.Category() != "Platform" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Platform")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestRootfsDeviceCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		lookPath       func(string) (string, error)
		execOutput     map[string]fakeExecResult
		wantStatus     string
		wantMsgContain string
	}{
		{
			name: "docker not found",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{},
			wantStatus: "skip",
		},
		{
			name: "docker info driver query fails (daemon not running)",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info -f {{.Driver}}": {err: errors.New("docker not running")},
			},
			wantStatus: "skip",
		},
		{
			name: "docker info returns overlay2 driver with clean DriverStatus",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info -f {{.Driver}}":       {lines: "overlay2\n"},
				"docker info -f {{json .DriverStatus}}": {lines: "[[\"Backing Filesystem\",\"xfs\"],[\"Supports d_type\",\"true\"]]\n"},
			},
			wantStatus:     "ok",
			wantMsgContain: "overlay2",
		},
		{
			name: "docker info returns btrfs driver",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info -f {{.Driver}}": {lines: "btrfs\n"},
			},
			wantStatus:     "warn",
			wantMsgContain: "btrfs",
		},
		{
			name: "overlay2 driver but btrfs in DriverStatus backing filesystem",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info -f {{.Driver}}":       {lines: "overlay2\n"},
				"docker info -f {{json .DriverStatus}}": {lines: "[[\"Backing Filesystem\",\"btrfs\"],[\"Supports d_type\",\"true\"]]\n"},
			},
			wantStatus:     "warn",
			wantMsgContain: "btrfs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &rootfsDeviceCheck{
				lookPath: tt.lookPath,
				execCmd:  newFakeExecCmd(tt.execOutput),
			}
			results := check.Run()
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
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
