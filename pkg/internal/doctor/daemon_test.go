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
	"os"
	"strings"
	"testing"
)

func TestDaemonJSONCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newDaemonJSONCheck()
	if check.Name() != "daemon-json-init" {
		t.Errorf("Name() = %q, want %q", check.Name(), "daemon-json-init")
	}
	if check.Category() != "Docker" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Docker")
	}
	if check.Platforms() != nil {
		t.Errorf("Platforms() = %v, want nil (all platforms)", check.Platforms())
	}
}

func TestDaemonJSONCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		goos              string
		files             map[string]string // path -> content (absent means os.ErrNotExist)
		wantStatus        string
		wantMsgContain    string
		wantReasonContain string
		wantFixContain    string
	}{
		{
			name:           "no daemon.json found - ok",
			goos:           "linux",
			files:          map[string]string{},
			wantStatus:     "ok",
			wantMsgContain: "No daemon.json found",
		},
		{
			name: "daemon.json with init true - warn",
			goos: "linux",
			files: map[string]string{
				"/etc/docker/daemon.json": `{"init": true}`,
			},
			wantStatus:        "warn",
			wantMsgContain:    "init",
			wantReasonContain: "init=true",
			wantFixContain:    "Remove",
		},
		{
			name: "daemon.json with init false - ok",
			goos: "linux",
			files: map[string]string{
				"/etc/docker/daemon.json": `{"init": false}`,
			},
			wantStatus:     "ok",
			wantMsgContain: "daemon.json checked",
		},
		{
			name: "daemon.json with storage-driver no init - ok",
			goos: "linux",
			files: map[string]string{
				"/etc/docker/daemon.json": `{"storage-driver": "overlay2"}`,
			},
			wantStatus:     "ok",
			wantMsgContain: "daemon.json checked",
		},
		{
			name: "malformed JSON - warn",
			goos: "linux",
			files: map[string]string{
				"/etc/docker/daemon.json": `{bad json`,
			},
			wantStatus:     "warn",
			wantMsgContain: "not valid JSON",
		},
		{
			name: "first candidate unreadable second has init true",
			goos: "linux",
			files: map[string]string{
				// /etc/docker/daemon.json is absent (not in map)
				"/home/testuser/.docker/daemon.json": `{"init": true}`,
			},
			wantStatus:        "warn",
			wantMsgContain:    "init",
			wantReasonContain: "init=true",
		},
		{
			name: "windows path included when goos is windows",
			goos: "windows",
			files: map[string]string{
				`C:\ProgramData\docker\config\daemon.json`: `{"init": true}`,
			},
			wantStatus:     "warn",
			wantMsgContain: "init",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &daemonJSONCheck{
				readFile: func(path string) ([]byte, error) {
					if content, ok := tt.files[path]; ok {
						return []byte(content), nil
					}
					return nil, os.ErrNotExist
				},
				homeDir: func() (string, error) {
					return "/home/testuser", nil
				},
				goos: tt.goos,
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
			if r.Name != "daemon-json-init" {
				t.Errorf("Name = %q, want %q", r.Name, "daemon-json-init")
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
