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

package installcorednstuning

import (
	"strings"
	"testing"
)

// standardCorefile is a realistic kind Corefile used as test input.
const standardCorefile = `.:53 {
    errors
    health {
       lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
       ttl 30
    }
    prometheus :9153
    forward . /etc/resolv.conf {
       max_concurrent 1000
    }
    cache 30
    loop
    reload
    loadbalance
}`

func TestPatchCorefile(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errContains string
		// checks run against the output when wantErr is false
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:    "standard corefile applies all three transforms",
			input:   standardCorefile,
			wantErr: false,
			mustContain: []string{
				"pods verified",
				"autopath @kubernetes\n    kubernetes cluster.local",
				"cache 60",
			},
			mustNotContain: []string{
				"pods insecure",
				"cache 30",
			},
		},
		{
			name:    "autopath inserted before kubernetes block",
			input:   standardCorefile,
			wantErr: false,
			mustContain: []string{
				"autopath @kubernetes",
				"kubernetes cluster.local",
			},
		},
		{
			name:        "missing pods insecure returns error",
			input:       strings.ReplaceAll(standardCorefile, "pods insecure", "pods verified"),
			wantErr:     true,
			errContains: "pods insecure",
		},
		{
			name:        "missing cache 30 returns error",
			input:       strings.ReplaceAll(standardCorefile, "cache 30", "cache 60"),
			wantErr:     true,
			errContains: "cache 30",
		},
		{
			name:        "missing kubernetes cluster.local returns error",
			input:       strings.ReplaceAll(standardCorefile, "kubernetes cluster.local", "kubernetes example.com"),
			wantErr:     true,
			errContains: "kubernetes cluster.local",
		},
		{
			name:        "empty input returns error",
			input:       "",
			wantErr:     true,
			errContains: "pods insecure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := patchCorefile(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("patchCorefile() expected error, got nil")
					return
				}
				if tc.errContains != "" {
					if !strings.Contains(err.Error(), tc.errContains) {
						t.Errorf("patchCorefile() error = %q, want error containing %q", err.Error(), tc.errContains)
					}
				}
				return
			}
			if err != nil {
				t.Errorf("patchCorefile() unexpected error: %v", err)
				return
			}
			for _, want := range tc.mustContain {
				if !strings.Contains(got, want) {
					t.Errorf("patchCorefile() output missing %q", want)
				}
			}
			for _, notWant := range tc.mustNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("patchCorefile() output should not contain %q", notWant)
				}
			}
		})
	}
}

func TestIndentCorefile(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "non-empty lines get 4-space indent",
			input: "line1\nline2\nline3",
			want:  "    line1\n    line2\n    line3",
		},
		{
			name:  "empty lines remain empty",
			input: "line1\n\nline3",
			want:  "    line1\n\n    line3",
		},
		{
			name:  "single line input",
			input: "only",
			want:  "    only",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := indentCorefile(tc.input)
			if got != tc.want {
				t.Errorf("indentCorefile() = %q, want %q", got, tc.want)
			}
		})
	}
}
