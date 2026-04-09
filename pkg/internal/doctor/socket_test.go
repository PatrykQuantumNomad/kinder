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

func TestDockerSocketCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newDockerSocketCheck()
	if check.Name() != "docker-socket" {
		t.Errorf("Name() = %q, want %q", check.Name(), "docker-socket")
	}
	if check.Category() != "Docker" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Docker")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestDockerSocketCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		lookPath       func(string) (string, error)
		execOutput     map[string]fakeExecResult
		wantStatus     string
		wantMsgContain string
		wantFixContain string
	}{
		{
			name: "docker not found",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			execOutput:     map[string]fakeExecResult{},
			wantStatus:     "skip",
			wantMsgContain: "(docker not found)",
		},
		{
			name: "docker info succeeds",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info": {lines: "Containers: 3\nImages: 10\n"},
			},
			wantStatus:     "ok",
			wantMsgContain: "Docker socket accessible",
		},
		{
			name: "docker info fails with permission denied",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info": {
					lines: "Got permission denied while trying to connect to the Docker daemon socket",
					err:   errors.New("exit status 1"),
				},
			},
			wantStatus:     "fail",
			wantMsgContain: "permission denied",
			wantFixContain: "usermod -aG docker",
		},
		{
			name: "docker info fails without permission denied",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info": {
					lines: "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
					err:   errors.New("exit status 1"),
				},
			},
			wantStatus:     "ok",
			wantMsgContain: "daemon may not be running",
		},
		{
			name: "docker info fails with Permission Denied uppercase",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info": {
					lines: "Error: Permission Denied accessing /var/run/docker.sock",
					err:   errors.New("exit status 1"),
				},
			},
			wantStatus:     "fail",
			wantMsgContain: "permission denied",
			wantFixContain: "usermod -aG docker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &dockerSocketCheck{
				lookPath: tt.lookPath,
				execCmd:  newFakeExecCmd(tt.execOutput),
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
			if r.Name != "docker-socket" {
				t.Errorf("Name = %q, want %q", r.Name, "docker-socket")
			}
			if tt.wantMsgContain != "" && !strings.Contains(r.Message, tt.wantMsgContain) {
				t.Errorf("Message = %q, want to contain %q", r.Message, tt.wantMsgContain)
			}
			if tt.wantFixContain != "" && !strings.Contains(r.Fix, tt.wantFixContain) {
				t.Errorf("Fix = %q, want to contain %q", r.Fix, tt.wantFixContain)
			}
		})
	}
}

func TestAllChecks_Registry(t *testing.T) {
	t.Parallel()
	checks := AllChecks()
	if len(checks) != 20 {
		t.Fatalf("AllChecks() has %d entries, want 20", len(checks))
	}

	// Expected order: Runtime(1), Docker(4), Tools(2), GPU(3), Kernel(2), Security(2), Platform(3), Network(1), Cluster(1), Offline(1)
	expected := []struct {
		name     string
		category string
	}{
		{"container-runtime", "Runtime"},
		{"disk-space", "Docker"},
		{"daemon-json-init", "Docker"},
		{"docker-snap", "Docker"},
		{"docker-socket", "Docker"},
		{"kubectl", "Tools"},
		{"kubectl-version-skew", "Tools"},
		{"nvidia-driver", "GPU"},
		{"nvidia-container-toolkit", "GPU"},
		{"nvidia-docker-runtime", "GPU"},
		{"inotify-limits", "Kernel"},
		{"kernel-version", "Kernel"},
		{"apparmor", "Security"},
		{"selinux", "Security"},
		{"firewalld-backend", "Platform"},
		{"wsl2-cgroup", "Platform"},
		{"rootfs-device", "Platform"},
		{"network-subnet", "Network"},
		{"cluster-node-skew", "Cluster"},
		{"offline-readiness", "Offline"},
	}

	for i, exp := range expected {
		if checks[i].Name() != exp.name {
			t.Errorf("AllChecks()[%d].Name() = %q, want %q", i, checks[i].Name(), exp.name)
		}
		if checks[i].Category() != exp.category {
			t.Errorf("AllChecks()[%d].Category() = %q, want %q", i, checks[i].Category(), exp.category)
		}
	}
}
