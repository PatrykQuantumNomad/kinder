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

func TestInotifyCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newInotifyCheck()
	if check.Name() != "inotify-limits" {
		t.Errorf("Name() = %q, want %q", check.Name(), "inotify-limits")
	}
	if check.Category() != "Kernel" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Kernel")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestInotifyCheck_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		files          map[string]string // path -> content for fakeReadFile
		fileErrors     map[string]error  // path -> error for fakeReadFile
		wantCount      int
		wantStatus     []string
		wantMsgContain []string
		wantFixContain []string
	}{
		{
			name: "both limits ok",
			files: map[string]string{
				"/proc/sys/fs/inotify/max_user_watches":   "524288\n",
				"/proc/sys/fs/inotify/max_user_instances": "512\n",
			},
			wantCount:      1,
			wantStatus:     []string{"ok"},
			wantMsgContain: []string{"524288"},
		},
		{
			name: "watches too low",
			files: map[string]string{
				"/proc/sys/fs/inotify/max_user_watches":   "8192\n",
				"/proc/sys/fs/inotify/max_user_instances": "512\n",
			},
			wantCount:      1,
			wantStatus:     []string{"warn"},
			wantMsgContain: []string{"8192"},
			wantFixContain: []string{"sysctl fs.inotify.max_user_watches"},
		},
		{
			name: "instances too low",
			files: map[string]string{
				"/proc/sys/fs/inotify/max_user_watches":   "524288\n",
				"/proc/sys/fs/inotify/max_user_instances": "128\n",
			},
			wantCount:      1,
			wantStatus:     []string{"warn"},
			wantMsgContain: []string{"128"},
			wantFixContain: []string{"sysctl fs.inotify.max_user_instances"},
		},
		{
			name: "both too low",
			files: map[string]string{
				"/proc/sys/fs/inotify/max_user_watches":   "8192\n",
				"/proc/sys/fs/inotify/max_user_instances": "128\n",
			},
			wantCount:      2,
			wantStatus:     []string{"warn", "warn"},
			wantMsgContain: []string{"8192", "128"},
			wantFixContain: []string{"sysctl fs.inotify.max_user_watches", "sysctl fs.inotify.max_user_instances"},
		},
		{
			name:  "both files unreadable",
			files: map[string]string{},
			fileErrors: map[string]error{
				"/proc/sys/fs/inotify/max_user_watches":   errors.New("no such file"),
				"/proc/sys/fs/inotify/max_user_instances": errors.New("no such file"),
			},
			wantCount:      1,
			wantStatus:     []string{"warn"},
			wantMsgContain: []string{"Could not read"},
		},
		{
			name: "watches unreadable but instances ok",
			files: map[string]string{
				"/proc/sys/fs/inotify/max_user_instances": "512\n",
			},
			fileErrors: map[string]error{
				"/proc/sys/fs/inotify/max_user_watches": errors.New("permission denied"),
			},
			wantCount:      1,
			wantStatus:     []string{"warn"},
			wantMsgContain: []string{"Could not read"},
		},
		{
			name: "values above threshold",
			files: map[string]string{
				"/proc/sys/fs/inotify/max_user_watches":   "1048576\n",
				"/proc/sys/fs/inotify/max_user_instances": "1024\n",
			},
			wantCount:      1,
			wantStatus:     []string{"ok"},
			wantMsgContain: []string{"1048576"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			check := &inotifyCheck{
				readFile: fakeReadFile(tt.files, tt.fileErrors),
			}
			results := check.Run()
			if len(results) != tt.wantCount {
				t.Fatalf("expected %d result(s), got %d: %+v", tt.wantCount, len(results), results)
			}
			for i, r := range results {
				if r.Name != "inotify-limits" {
					t.Errorf("result[%d].Name = %q, want %q", i, r.Name, "inotify-limits")
				}
				if r.Category != "Kernel" {
					t.Errorf("result[%d].Category = %q, want %q", i, r.Category, "Kernel")
				}
				if i < len(tt.wantStatus) && r.Status != tt.wantStatus[i] {
					t.Errorf("result[%d].Status = %q, want %q", i, r.Status, tt.wantStatus[i])
				}
				if i < len(tt.wantMsgContain) && tt.wantMsgContain[i] != "" && !strings.Contains(r.Message, tt.wantMsgContain[i]) {
					t.Errorf("result[%d].Message = %q, want to contain %q", i, r.Message, tt.wantMsgContain[i])
				}
				if i < len(tt.wantFixContain) && tt.wantFixContain[i] != "" && !strings.Contains(r.Fix, tt.wantFixContain[i]) {
					t.Errorf("result[%d].Fix = %q, want to contain %q", i, r.Fix, tt.wantFixContain[i])
				}
			}
		})
	}
}

// fakeReadFile creates a readFile function backed by maps for testing.
func fakeReadFile(files map[string]string, errs map[string]error) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		if errs != nil {
			if err, ok := errs[path]; ok {
				return nil, err
			}
		}
		if content, ok := files[path]; ok {
			return []byte(content), nil
		}
		return nil, errors.New("file not found: " + path)
	}
}
