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

func TestApparmorCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newApparmorCheck()
	if check.Name() != "apparmor" {
		t.Errorf("Name() = %q, want %q", check.Name(), "apparmor")
	}
	if check.Category() != "Security" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Security")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestApparmorCheck_Run(t *testing.T) {
	t.Parallel()

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
			name: "apparmor enabled - warn",
			files: map[string]string{
				"/sys/module/apparmor/parameters/enabled": "Y\n",
			},
			wantStatus:        "warn",
			wantMsgContain:    "enabled",
			wantReasonContain: "moby/moby#7512",
			wantFixContain:    "aa-remove-unknown",
		},
		{
			name: "apparmor disabled - ok",
			files: map[string]string{
				"/sys/module/apparmor/parameters/enabled": "N\n",
			},
			wantStatus:     "ok",
			wantMsgContain: "not enabled",
		},
		{
			name:           "apparmor module not loaded - ok",
			files:          map[string]string{},
			fileErrors:     map[string]error{},
			wantStatus:     "ok",
			wantMsgContain: "not enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &apparmorCheck{
				readFile: fakeReadFile(tt.files, tt.fileErrors),
			}
			results := check.Run()
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
			}
			r := results[0]
			if r.Name != "apparmor" {
				t.Errorf("Name = %q, want %q", r.Name, "apparmor")
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
