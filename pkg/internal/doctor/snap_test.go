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

func TestDockerSnapCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newDockerSnapCheck()
	if check.Name() != "docker-snap" {
		t.Errorf("Name() = %q, want %q", check.Name(), "docker-snap")
	}
	if check.Category() != "Docker" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Docker")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestDockerSnapCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		lookPath          func(string) (string, error)
		evalSymlinks      func(string) (string, error)
		wantStatus        string
		wantMsgContain    string
		wantReasonContain string
		wantFixContain    string
	}{
		{
			name: "docker not found - skip",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			evalSymlinks: func(path string) (string, error) {
				return path, nil
			},
			wantStatus:     "skip",
			wantMsgContain: "docker not found",
		},
		{
			name: "docker installed via snap - warn",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/snap/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			evalSymlinks: func(path string) (string, error) {
				return "/snap/docker/current/bin/docker", nil
			},
			wantStatus:     "warn",
			wantMsgContain: "snap",
			wantFixContain: "TMPDIR",
		},
		{
			name: "docker not installed via snap - ok",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			evalSymlinks: func(path string) (string, error) {
				return "/usr/bin/docker", nil
			},
			wantStatus:     "ok",
			wantMsgContain: "not installed via snap",
		},
		{
			name: "evalSymlinks fails - falls back to lookPath result",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/snap/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			evalSymlinks: func(path string) (string, error) {
				return "", errors.New("symlink error")
			},
			wantStatus:     "warn",
			wantMsgContain: "snap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &dockerSnapCheck{
				lookPath:     tt.lookPath,
				evalSymlinks: tt.evalSymlinks,
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
			if r.Name != "docker-snap" {
				t.Errorf("Name = %q, want %q", r.Name, "docker-snap")
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
