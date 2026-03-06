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

func TestKubectlCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newKubectlCheck()
	if check.Name() != "kubectl" {
		t.Errorf("Name() = %q, want %q", check.Name(), "kubectl")
	}
	if check.Category() != "Tools" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Tools")
	}
	if check.Platforms() != nil {
		t.Errorf("Platforms() = %v, want nil (all platforms)", check.Platforms())
	}
}

func TestKubectlCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		lookPath       func(string) (string, error)
		execOutput     map[string]fakeExecResult
		wantStatus     string
		wantMsgContain string
		wantReason     string
		wantFix        string
	}{
		{
			name: "kubectl found and working",
			lookPath: func(name string) (string, error) {
				if name == "kubectl" {
					return "/usr/local/bin/kubectl", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"kubectl version --client": {lines: "Client Version: v1.31.0\n"},
			},
			wantStatus:     "ok",
			wantMsgContain: "kubectl available",
		},
		{
			name: "kubectl not found",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{},
			wantStatus: "fail",
			wantFix:    "https://kubernetes.io/docs/tasks/tools/",
		},
		{
			name: "kubectl found but version fails",
			lookPath: func(name string) (string, error) {
				if name == "kubectl" {
					return "/usr/local/bin/kubectl", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"kubectl version --client": {err: errors.New("version failed")},
			},
			wantStatus: "warn",
			wantReason: "kubectl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &kubectlCheck{
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
			if r.Category != "Tools" {
				t.Errorf("Category = %q, want %q", r.Category, "Tools")
			}
			if r.Name != "kubectl" {
				t.Errorf("Name = %q, want %q", r.Name, "kubectl")
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
