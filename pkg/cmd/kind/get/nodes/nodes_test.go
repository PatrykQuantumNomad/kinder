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

package nodes

import (
	"testing"
)

func TestComputeSkew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cpMinor    uint
		nodeMinor  uint
		wantOK     bool
		wantContain string
	}{
		{
			name:        "same version returns checkmark",
			cpMinor:     31,
			nodeMinor:   31,
			wantOK:      true,
			wantContain: "\u2713", // checkmark ✓
		},
		{
			name:        "2 minors behind returns cross with -2",
			cpMinor:     31,
			nodeMinor:   29,
			wantOK:      true, // within 3-minor policy
			wantContain: "-2",
		},
		{
			name:        "4 minors behind returns cross with -4",
			cpMinor:     31,
			nodeMinor:   27,
			wantOK:      false,
			wantContain: "-4",
		},
		{
			name:        "1 minor behind is within policy",
			cpMinor:     31,
			nodeMinor:   30,
			wantOK:      true,
			wantContain: "-1",
		},
		{
			name:        "3 minors behind is at limit - ok",
			cpMinor:     31,
			nodeMinor:   28,
			wantOK:      true,
			wantContain: "-3",
		},
		{
			name:        "4 minors behind violates policy",
			cpMinor:     31,
			nodeMinor:   27,
			wantOK:      false,
			wantContain: "-4",
		},
		{
			name:        "node ahead of CP returns cross with positive offset",
			cpMinor:     31,
			nodeMinor:   33,
			wantOK:      false,
			wantContain: "+2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			display, ok := ComputeSkew(tt.cpMinor, tt.nodeMinor)
			if ok != tt.wantOK {
				t.Errorf("ComputeSkew(%d, %d) ok = %v, want %v; display = %q",
					tt.cpMinor, tt.nodeMinor, ok, tt.wantOK, display)
			}
			if tt.wantContain != "" {
				found := false
				for _, r := range display {
					_ = r
				}
				// Check substring
				if len(display) == 0 {
					t.Errorf("ComputeSkew(%d, %d) display is empty", tt.cpMinor, tt.nodeMinor)
				}
				found = containsStr(display, tt.wantContain)
				if !found {
					t.Errorf("ComputeSkew(%d, %d) display = %q, want to contain %q",
						tt.cpMinor, tt.nodeMinor, display, tt.wantContain)
				}
			}
		})
	}
}

// containsStr reports whether s contains substr.
func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
