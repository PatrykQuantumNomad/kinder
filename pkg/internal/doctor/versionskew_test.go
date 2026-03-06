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

func TestKubectlVersionSkewCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newKubectlVersionSkewCheck()
	if check.Name() != "kubectl-version-skew" {
		t.Errorf("Name() = %q, want %q", check.Name(), "kubectl-version-skew")
	}
	if check.Category() != "Tools" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Tools")
	}
	if check.Platforms() != nil {
		t.Errorf("Platforms() = %v, want nil (all platforms)", check.Platforms())
	}
}

func TestKubectlVersionSkewCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		lookPath          func(string) (string, error)
		execOutput        map[string]fakeExecResult
		wantStatus        string
		wantMsgContain    string
		wantMsgNotContain string
		wantReasonContain string
		wantFixContain    string
	}{
		{
			name: "kubectl not found",
			lookPath: func(name string) (string, error) {
				return "", errors.New("not found")
			},
			execOutput:     map[string]fakeExecResult{},
			wantStatus:     "skip",
			wantMsgContain: "(kubectl not found)",
		},
		{
			name: "exact match v1.31.0",
			lookPath: func(name string) (string, error) {
				if name == "kubectl" {
					return "/usr/local/bin/kubectl", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"kubectl version --client -o json": {
					lines: `{"clientVersion":{"gitVersion":"v1.31.0"}}`,
				},
			},
			wantStatus:     "ok",
			wantMsgContain: "v1.31.0",
		},
		{
			name: "1 minor behind v1.30.0 is ok",
			lookPath: func(name string) (string, error) {
				if name == "kubectl" {
					return "/usr/local/bin/kubectl", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"kubectl version --client -o json": {
					lines: `{"clientVersion":{"gitVersion":"v1.30.0"}}`,
				},
			},
			wantStatus:     "ok",
			wantMsgContain: "v1.30.0",
		},
		{
			name: "3 minor behind v1.28.4 warns",
			lookPath: func(name string) (string, error) {
				if name == "kubectl" {
					return "/usr/local/bin/kubectl", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"kubectl version --client -o json": {
					lines: `{"clientVersion":{"gitVersion":"v1.28.4"}}`,
				},
			},
			wantStatus:        "warn",
			wantMsgContain:    "v1.28.4",
			wantReasonContain: "skew",
		},
		{
			name: "1 minor ahead v1.32.0 is ok",
			lookPath: func(name string) (string, error) {
				if name == "kubectl" {
					return "/usr/local/bin/kubectl", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"kubectl version --client -o json": {
					lines: `{"clientVersion":{"gitVersion":"v1.32.0"}}`,
				},
			},
			wantStatus:     "ok",
			wantMsgContain: "v1.32.0",
		},
		{
			name: "3 minor ahead v1.34.0 warns",
			lookPath: func(name string) (string, error) {
				if name == "kubectl" {
					return "/usr/local/bin/kubectl", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"kubectl version --client -o json": {
					lines: `{"clientVersion":{"gitVersion":"v1.34.0"}}`,
				},
			},
			wantStatus:        "warn",
			wantMsgContain:    "v1.34.0",
			wantReasonContain: "skew",
		},
		{
			name: "unparseable version output",
			lookPath: func(name string) (string, error) {
				if name == "kubectl" {
					return "/usr/local/bin/kubectl", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"kubectl version --client -o json": {
					lines: `{"clientVersion":{"gitVersion":"not-a-version"}}`,
				},
			},
			wantStatus:     "warn",
			wantMsgContain: "Could not parse",
		},
		{
			name: "kubectl version command fails",
			lookPath: func(name string) (string, error) {
				if name == "kubectl" {
					return "/usr/local/bin/kubectl", nil
				}
				return "", errors.New("not found")
			},
			execOutput: map[string]fakeExecResult{
				"kubectl version --client -o json": {
					err: errors.New("command failed"),
				},
			},
			wantStatus:     "warn",
			wantMsgContain: "Could not determine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &kubectlVersionSkewCheck{
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
			if r.Name != "kubectl-version-skew" {
				t.Errorf("Name = %q, want %q", r.Name, "kubectl-version-skew")
			}
			if tt.wantMsgContain != "" && !strings.Contains(r.Message, tt.wantMsgContain) {
				t.Errorf("Message = %q, want to contain %q", r.Message, tt.wantMsgContain)
			}
			if tt.wantMsgNotContain != "" && strings.Contains(r.Message, tt.wantMsgNotContain) {
				t.Errorf("Message = %q, want NOT to contain %q", r.Message, tt.wantMsgNotContain)
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
