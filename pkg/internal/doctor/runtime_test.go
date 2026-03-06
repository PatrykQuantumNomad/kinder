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

func TestContainerRuntimeCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newContainerRuntimeCheck()
	if check.Name() != "container-runtime" {
		t.Errorf("Name() = %q, want %q", check.Name(), "container-runtime")
	}
	if check.Category() != "Runtime" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Runtime")
	}
	if check.Platforms() != nil {
		t.Errorf("Platforms() = %v, want nil (all platforms)", check.Platforms())
	}
}

func TestContainerRuntimeCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		lookPath       func(string) (string, error)
		execOutput     map[string]fakeExecResult
		wantCount      int
		wantStatus     string
		wantMsgContain string
		wantReason     string
		wantFix        string
	}{
		{
			name: "docker found and working",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker version": {lines: "Docker version 24.0.7\n"},
			},
			wantCount:      1,
			wantStatus:     "ok",
			wantMsgContain: "docker",
		},
		{
			name: "podman found and working when docker missing",
			lookPath: func(name string) (string, error) {
				if name == "podman" {
					return "/usr/bin/podman", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"podman version": {lines: "podman version 4.9.0\n"},
			},
			wantCount:      1,
			wantStatus:     "ok",
			wantMsgContain: "podman",
		},
		{
			name: "nerdctl found and working when docker and podman missing",
			lookPath: func(name string) (string, error) {
				if name == "nerdctl" {
					return "/usr/bin/nerdctl", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"nerdctl version": {lines: "nerdctl version 1.7.0\n"},
			},
			wantCount:      1,
			wantStatus:     "ok",
			wantMsgContain: "nerdctl",
		},
		{
			name: "docker found but not responding",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker version": {err: errors.New("daemon not responding")},
				"docker -v":      {err: errors.New("daemon not responding")},
			},
			wantCount:  1,
			wantStatus: "warn",
			wantReason: "The container daemon must be running for cluster creation",
			wantFix:    "docker",
		},
		{
			name: "no runtimes found",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{},
			wantCount:  1,
			wantStatus: "fail",
			wantReason: "A container runtime (Docker, Podman, or nerdctl) is required",
			wantFix:    "https://docs.docker.com/get-docker/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &containerRuntimeCheck{
				lookPath: tt.lookPath,
				execCmd:  newFakeExecCmd(tt.execOutput),
			}
			results := check.Run()
			if len(results) != tt.wantCount {
				t.Fatalf("expected %d result(s), got %d", tt.wantCount, len(results))
			}
			r := results[0]
			if r.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", r.Status, tt.wantStatus)
			}
			if r.Category != "Runtime" {
				t.Errorf("Category = %q, want %q", r.Category, "Runtime")
			}
			if r.Name != "container-runtime" {
				t.Errorf("Name = %q, want %q", r.Name, "container-runtime")
			}
			if tt.wantMsgContain != "" && !strings.Contains(r.Message, tt.wantMsgContain) {
				t.Errorf("Message = %q, want to contain %q", r.Message, tt.wantMsgContain)
			}
			if tt.wantReason != "" && !strings.Contains(r.Reason, tt.wantReason) {
				t.Errorf("Reason = %q, want to contain %q", r.Reason, tt.wantReason)
			}
			if tt.wantFix != "" && !strings.Contains(r.Fix, tt.wantFix) {
				t.Errorf("Fix = %q, want to contain %q", r.Fix, tt.wantFix)
			}
		})
	}
}
