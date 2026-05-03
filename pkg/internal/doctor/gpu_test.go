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

func TestNvidiaDriverCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newNvidiaDriverCheck()
	if check.Name() != "nvidia-driver" {
		t.Errorf("Name() = %q, want %q", check.Name(), "nvidia-driver")
	}
	if check.Category() != "GPU" {
		t.Errorf("Category() = %q, want %q", check.Category(), "GPU")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestNvidiaDriverCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		lookPath       func(string) (string, error)
		execOutput     map[string]fakeExecResult
		wantStatus     string
		wantMsgContain string
		wantFix        string
	}{
		{
			name: "nvidia-smi found with driver version",
			lookPath: func(name string) (string, error) {
				if name == "nvidia-smi" {
					return "/usr/bin/nvidia-smi", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"nvidia-smi --query-gpu=driver_version --format=csv,noheader": {lines: "535.129.03\n"},
			},
			wantStatus:     "ok",
			wantMsgContain: "535.129.03",
		},
		{
			name: "nvidia-smi not found",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{},
			wantStatus: "warn",
			wantFix:    "https://www.nvidia.com/drivers",
		},
		{
			name: "nvidia-smi found but query fails",
			lookPath: func(name string) (string, error) {
				if name == "nvidia-smi" {
					return "/usr/bin/nvidia-smi", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"nvidia-smi --query-gpu=driver_version --format=csv,noheader": {err: errors.New("no GPU")},
			},
			wantStatus: "warn",
			wantFix:    "nvidia-smi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &nvidiaDriverCheck{
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
			if r.Category != "GPU" {
				t.Errorf("Category = %q, want %q", r.Category, "GPU")
			}
			if tt.wantMsgContain != "" && !strings.Contains(r.Message, tt.wantMsgContain) {
				t.Errorf("Message = %q, want to contain %q", r.Message, tt.wantMsgContain)
			}
			if tt.wantFix != "" && !strings.Contains(r.Fix, tt.wantFix) {
				t.Errorf("Fix = %q, want to contain %q", r.Fix, tt.wantFix)
			}
		})
	}
}

func TestNvidiaContainerToolkitCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newNvidiaContainerToolkitCheck()
	if check.Name() != "nvidia-container-toolkit" {
		t.Errorf("Name() = %q, want %q", check.Name(), "nvidia-container-toolkit")
	}
	if check.Category() != "GPU" {
		t.Errorf("Category() = %q, want %q", check.Category(), "GPU")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestNvidiaContainerToolkitCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		lookPath   func(string) (string, error)
		wantStatus string
		wantFix    string
	}{
		{
			name: "nvidia-ctk found",
			lookPath: func(name string) (string, error) {
				if name == "nvidia-ctk" {
					return "/usr/bin/nvidia-ctk", nil
				}
				return "", errors.New("not found")
			},
			wantStatus: "ok",
		},
		{
			name: "nvidia-ctk not found",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			wantStatus: "warn",
			wantFix:    "nvidia-container-toolkit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &nvidiaContainerToolkitCheck{
				lookPath: tt.lookPath,
			}
			results := check.Run()
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			r := results[0]
			if r.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", r.Status, tt.wantStatus)
			}
			if tt.wantFix != "" && !strings.Contains(r.Fix, tt.wantFix) {
				t.Errorf("Fix = %q, want to contain %q", r.Fix, tt.wantFix)
			}
		})
	}
}

func TestNvidiaDockerRuntimeCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newNvidiaDockerRuntimeCheck()
	if check.Name() != "nvidia-docker-runtime" {
		t.Errorf("Name() = %q, want %q", check.Name(), "nvidia-docker-runtime")
	}
	if check.Category() != "GPU" {
		t.Errorf("Category() = %q, want %q", check.Category(), "GPU")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestNvidiaDockerRuntimeCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		lookPath       func(string) (string, error)
		execOutput     map[string]fakeExecResult
		wantStatus     string
		wantMsgContain string
		wantFix        string
	}{
		{
			name: "nvidia runtime configured in Docker",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info --format {{.Runtimes}}": {lines: "io.containerd.runc.v2 nvidia runc\n"},
			},
			wantStatus:     "ok",
			wantMsgContain: "nvidia runtime configured",
		},
		{
			name: "nvidia runtime not configured in Docker",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info --format {{.Runtimes}}": {lines: "io.containerd.runc.v2 runc\n"},
			},
			wantStatus: "warn",
			wantFix:    "nvidia-ctk runtime configure",
		},
		{
			name: "docker not available - skip",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			execOutput:     map[string]fakeExecResult{},
			wantStatus:     "skip",
			wantMsgContain: "requires docker",
		},
		{
			name: "docker info fails",
			lookPath: func(name string) (string, error) {
				if name == "docker" {
					return "/usr/bin/docker", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"docker info --format {{.Runtimes}}": {err: errors.New("docker not running")},
			},
			wantStatus: "warn",
			wantFix:    "Docker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &nvidiaDockerRuntimeCheck{
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
			if tt.wantMsgContain != "" && !strings.Contains(r.Message, tt.wantMsgContain) {
				t.Errorf("Message = %q, want to contain %q", r.Message, tt.wantMsgContain)
			}
			if tt.wantFix != "" && !strings.Contains(r.Fix, tt.wantFix) {
				t.Errorf("Fix = %q, want to contain %q", r.Fix, tt.wantFix)
			}
		})
	}
}

func TestAllChecks_RegisteredOrder(t *testing.T) {
	t.Parallel()
	checks := AllChecks()
	if len(checks) != 24 {
		t.Fatalf("AllChecks() returned %d checks, want 24", len(checks))
	}
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
		{"local-path-cve", "Cluster"},
		{"cluster-resume-readiness", "Cluster"},
		{"offline-readiness", "Offline"},
		{"host-mount-path", "Mounts"},
		{"docker-desktop-file-sharing", "Mounts"},
	}
	for i, exp := range expected {
		if checks[i].Name() != exp.name {
			t.Errorf("checks[%d].Name() = %q, want %q", i, checks[i].Name(), exp.name)
		}
		if checks[i].Category() != exp.category {
			t.Errorf("checks[%d].Category() = %q, want %q", i, checks[i].Category(), exp.category)
		}
	}
}
