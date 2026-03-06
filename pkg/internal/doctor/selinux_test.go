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
	"strings"
	"testing"
)

func TestSELinuxCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newSELinuxCheck()
	if check.Name() != "selinux" {
		t.Errorf("Name() = %q, want %q", check.Name(), "selinux")
	}
	if check.Category() != "Security" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Security")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestSELinuxCheck_Run(t *testing.T) {
	t.Parallel()

	fedoraRelease := "NAME=\"Fedora Linux\"\nVERSION=\"39 (Workstation Edition)\"\nID=fedora\nVERSION_ID=39\n"
	ubuntuRelease := "NAME=\"Ubuntu\"\nVERSION=\"22.04.3 LTS (Jammy Jellyfish)\"\nID=ubuntu\nVERSION_ID=\"22.04\"\n"

	tests := []struct {
		name              string
		files             map[string]string
		fileErrors        map[string]error
		wantStatus        string
		wantMsgContain    string
		wantReasonContain string
		wantFixContain    string
	}{
		{
			name: "enforcing on Fedora - warn",
			files: map[string]string{
				"/sys/fs/selinux/enforce": "1",
				"/etc/os-release":         fedoraRelease,
			},
			wantStatus:        "warn",
			wantMsgContain:    "enforcing",
			wantReasonContain: "/dev/dma_heap",
			wantFixContain:    "setenforce",
		},
		{
			name: "enforcing on Ubuntu - ok",
			files: map[string]string{
				"/sys/fs/selinux/enforce": "1",
				"/etc/os-release":         ubuntuRelease,
			},
			wantStatus:     "ok",
			wantMsgContain: "no known kind issues",
		},
		{
			name: "permissive - ok",
			files: map[string]string{
				"/sys/fs/selinux/enforce": "0",
			},
			wantStatus:     "ok",
			wantMsgContain: "permissive or disabled",
		},
		{
			name:           "selinux not available - ok",
			files:          map[string]string{},
			fileErrors:     map[string]error{},
			wantStatus:     "ok",
			wantMsgContain: "not available",
		},
		{
			name: "enforcing but os-release unreadable - ok",
			files: map[string]string{
				"/sys/fs/selinux/enforce": "1",
			},
			// /etc/os-release not in files map => fakeReadFile returns error
			wantStatus:     "ok",
			wantMsgContain: "no known kind issues",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &selinuxCheck{
				readFile: fakeReadFile(tt.files, tt.fileErrors),
			}
			results := check.Run()
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
			}
			r := results[0]
			if r.Name != "selinux" {
				t.Errorf("Name = %q, want %q", r.Name, "selinux")
			}
			if r.Category != "Security" {
				t.Errorf("Category = %q, want %q", r.Category, "Security")
			}
			if r.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", r.Status, tt.wantStatus)
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
