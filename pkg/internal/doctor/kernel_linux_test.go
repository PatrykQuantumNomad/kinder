//go:build linux

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

	"golang.org/x/sys/unix"
)

func TestKernelVersionCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newKernelVersionCheck()
	if check.Name() != "kernel-version" {
		t.Errorf("Name() = %q, want %q", check.Name(), "kernel-version")
	}
	if check.Category() != "Kernel" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Kernel")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

// makeUtsname creates a Utsname with the given release string.
func makeUtsname(release string) unix.Utsname {
	var u unix.Utsname
	for i, b := range []byte(release) {
		if i >= len(u.Release)-1 {
			break
		}
		u.Release[i] = int8(b)
	}
	return u
}

func TestKernelVersionCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		uname             func(buf *unix.Utsname) error
		wantStatus        string
		wantMsgContain    string
		wantReasonContain string
		wantFixContain    string
	}{
		{
			name: "kernel 5.15 - ok",
			uname: func(buf *unix.Utsname) error {
				*buf = makeUtsname("5.15.0-91-generic")
				return nil
			},
			wantStatus:     "ok",
			wantMsgContain: "5.15",
		},
		{
			name: "kernel 4.6 boundary - ok",
			uname: func(buf *unix.Utsname) error {
				*buf = makeUtsname("4.6.0")
				return nil
			},
			wantStatus:     "ok",
			wantMsgContain: "4.6",
		},
		{
			name: "kernel 4.5 - fail",
			uname: func(buf *unix.Utsname) error {
				*buf = makeUtsname("4.5.0")
				return nil
			},
			wantStatus:        "fail",
			wantReasonContain: "cgroup namespace",
		},
		{
			name: "kernel 3.10 RHEL 7 - fail",
			uname: func(buf *unix.Utsname) error {
				*buf = makeUtsname("3.10.0-1160.el7.x86_64")
				return nil
			},
			wantStatus:        "fail",
			wantReasonContain: "cgroup namespace",
		},
		{
			name: "uname fails - warn",
			uname: func(buf *unix.Utsname) error {
				return errors.New("uname syscall failed")
			},
			wantStatus:     "warn",
			wantFixContain: "uname -r",
		},
		{
			name: "garbled release string - warn",
			uname: func(buf *unix.Utsname) error {
				*buf = makeUtsname("not-a-version")
				return nil
			},
			wantStatus: "warn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &kernelVersionCheck{
				uname: tt.uname,
			}
			results := check.Run()
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
			}
			r := results[0]
			if r.Name != "kernel-version" {
				t.Errorf("Name = %q, want %q", r.Name, "kernel-version")
			}
			if r.Category != "Kernel" {
				t.Errorf("Category = %q, want %q", r.Category, "Kernel")
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

func TestParseKernelVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		release   string
		wantMajor int
		wantMinor int
		wantErr   bool
	}{
		{release: "5.15.0-91-generic", wantMajor: 5, wantMinor: 15},
		{release: "4.6.0", wantMajor: 4, wantMinor: 6},
		{release: "6.8.0-1025-azure", wantMajor: 6, wantMinor: 8},
		{release: "3.10.0-1160.el7.x86_64", wantMajor: 3, wantMinor: 10},
		{release: "5.4.72-microsoft-standard-WSL2", wantMajor: 5, wantMinor: 4},
		{release: "6.1.0+", wantMajor: 6, wantMinor: 1},
		{release: "", wantErr: true},
		{release: "not-a-version", wantErr: true},
		{release: "5", wantErr: true},
		{release: "abc.def.ghi", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.release, func(t *testing.T) {
			t.Parallel()
			major, minor, err := parseKernelVersion(tt.release)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseKernelVersion(%q) = (%d, %d, nil), want error", tt.release, major, minor)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseKernelVersion(%q) error: %v", tt.release, err)
			}
			if major != tt.wantMajor {
				t.Errorf("parseKernelVersion(%q) major = %d, want %d", tt.release, major, tt.wantMajor)
			}
			if minor != tt.wantMinor {
				t.Errorf("parseKernelVersion(%q) minor = %d, want %d", tt.release, minor, tt.wantMinor)
			}
		})
	}
}
