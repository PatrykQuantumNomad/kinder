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

	"sigs.k8s.io/kind/pkg/exec"
)

func TestFirewalldCheck_Metadata(t *testing.T) {
	t.Parallel()
	check := newFirewalldCheck()
	if check.Name() != "firewalld-backend" {
		t.Errorf("Name() = %q, want %q", check.Name(), "firewalld-backend")
	}
	if check.Category() != "Platform" {
		t.Errorf("Category() = %q, want %q", check.Category(), "Platform")
	}
	platforms := check.Platforms()
	if len(platforms) != 1 || platforms[0] != "linux" {
		t.Errorf("Platforms() = %v, want [linux]", platforms)
	}
}

func TestFirewalldCheck_Run(t *testing.T) {
	t.Parallel()

	nftablesConf := "# firewalld config file\n\n# NOTE: FirewallBackend defaults\nFirewallBackend=nftables\n"
	iptablesConf := "# firewalld config file\nFirewallBackend=iptables\n"
	noBackendConf := "# firewalld config file\nDefaultZone=public\n"

	tests := []struct {
		name              string
		lookPathErr       error                    // nil = firewall-cmd found
		cmdResults        map[string]fakeExecResult // for newFakeExecCmd
		files             map[string]string         // for fakeReadFile
		fileErrors        map[string]error          // for fakeReadFile
		wantStatus        string
		wantMsgContain    string
		wantReasonContain string
		wantFixContain    string
	}{
		{
			name:           "firewall-cmd not found - ok",
			lookPathErr:    errors.New("not found"),
			wantStatus:     "ok",
			wantMsgContain: "not installed",
		},
		{
			name: "firewalld not running - ok",
			cmdResults: map[string]fakeExecResult{
				"firewall-cmd --state": {lines: "not running\n", err: errors.New("exit 252")},
			},
			wantStatus:     "ok",
			wantMsgContain: "not running",
		},
		{
			name: "firewalld running with nftables backend - warn",
			cmdResults: map[string]fakeExecResult{
				"firewall-cmd --state": {lines: "running\n"},
			},
			files: map[string]string{
				"/etc/firewalld/firewalld.conf": nftablesConf,
			},
			wantStatus:     "warn",
			wantMsgContain: "nftables",
			wantFixContain: "iptables",
		},
		{
			name: "firewalld running with iptables backend - ok",
			cmdResults: map[string]fakeExecResult{
				"firewall-cmd --state": {lines: "running\n"},
			},
			files: map[string]string{
				"/etc/firewalld/firewalld.conf": iptablesConf,
			},
			wantStatus:     "ok",
			wantMsgContain: "iptables",
		},
		{
			name: "firewalld running but config unreadable - ok",
			cmdResults: map[string]fakeExecResult{
				"firewall-cmd --state": {lines: "running\n"},
			},
			files:          map[string]string{},
			fileErrors:     map[string]error{},
			wantStatus:     "ok",
			wantMsgContain: "config not readable",
		},
		{
			name: "firewalld running with no FirewallBackend line - warn (defaults to nftables)",
			cmdResults: map[string]fakeExecResult{
				"firewall-cmd --state": {lines: "running\n"},
			},
			files: map[string]string{
				"/etc/firewalld/firewalld.conf": noBackendConf,
			},
			wantStatus:     "warn",
			wantMsgContain: "nftables",
			wantFixContain: "iptables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Build lookPath function.
			lookPath := func(name string) (string, error) {
				if tt.lookPathErr != nil {
					return "", tt.lookPathErr
				}
				return "/usr/bin/" + name, nil
			}

			// Build execCmd function.
			var execCmd func(string, ...string) exec.Cmd
			if tt.cmdResults != nil {
				execCmd = newFakeExecCmd(tt.cmdResults)
			} else {
				execCmd = newFakeExecCmd(map[string]fakeExecResult{})
			}

			check := &firewalldCheck{
				readFile: fakeReadFile(tt.files, tt.fileErrors),
				lookPath: lookPath,
				execCmd:  execCmd,
			}
			results := check.Run()
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
			}
			r := results[0]
			if r.Name != "firewalld-backend" {
				t.Errorf("Name = %q, want %q", r.Name, "firewalld-backend")
			}
			if r.Category != "Platform" {
				t.Errorf("Category = %q, want %q", r.Category, "Platform")
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
