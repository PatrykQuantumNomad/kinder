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

package create

import (
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/internal/apis/config"
)

// Note: testLogger is defined in create_addon_test.go (same package).
// Its lines field captures all Warn/Warnf/Error/Errorf/Info/Infof output.

func TestValidateExtraMounts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		cfg       *config.Cluster
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "no mounts",
			cfg:     &config.Cluster{Nodes: []config.Node{}},
			wantErr: false,
		},
		{
			name: "existing path",
			cfg: &config.Cluster{
				Nodes: []config.Node{
					{ExtraMounts: []config.Mount{{HostPath: t.TempDir()}}},
				},
			},
			wantErr: false,
		},
		{
			name: "non-existent path",
			cfg: &config.Cluster{
				Nodes: []config.Node{
					{ExtraMounts: []config.Mount{{HostPath: "/tmp/kinder-test-nonexistent-xyzzy123456"}}},
				},
			},
			wantErr:   true,
			errSubstr: "does not exist",
		},
		{
			name: "empty hostPath is skipped",
			cfg: &config.Cluster{
				Nodes: []config.Node{
					{ExtraMounts: []config.Mount{{HostPath: ""}}},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple nodes, second has bad path",
			cfg: &config.Cluster{
				Nodes: []config.Node{
					{ExtraMounts: []config.Mount{{HostPath: t.TempDir()}}},
					{ExtraMounts: []config.Mount{{HostPath: "/tmp/kinder-test-nonexistent-xyzzy123456"}}},
				},
			},
			wantErr:   true,
			errSubstr: "node[1]",
		},
		{
			name: "relative path that exists",
			cfg: &config.Cluster{
				Nodes: []config.Node{
					// "." resolves to the current working directory, which always exists.
					{ExtraMounts: []config.Mount{{HostPath: "."}}},
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateExtraMounts(tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.errSubstr)
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("expected error to contain %q, got: %v", tc.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestLogMountPropagationPlatformWarning(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  *config.Cluster
	}{
		{
			name: "no mounts",
			cfg:  &config.Cluster{Nodes: []config.Node{}},
		},
		{
			name: "None propagation",
			cfg: &config.Cluster{
				Nodes: []config.Node{
					{ExtraMounts: []config.Mount{{HostPath: "/foo", Propagation: config.MountPropagationNone}}},
				},
			},
		},
		{
			name: "HostToContainer propagation",
			cfg: &config.Cluster{
				Nodes: []config.Node{
					{ExtraMounts: []config.Mount{{HostPath: "/foo", Propagation: config.MountPropagationHostToContainer}}},
				},
			},
		},
		{
			name: "Bidirectional propagation",
			cfg: &config.Cluster{
				Nodes: []config.Node{
					{ExtraMounts: []config.Mount{{HostPath: "/foo", Propagation: config.MountPropagationBidirectional}}},
				},
			},
		},
		{
			name: "multiple mounts non-None — warn at most once",
			cfg: &config.Cluster{
				Nodes: []config.Node{
					{ExtraMounts: []config.Mount{
						{HostPath: "/a", Propagation: config.MountPropagationHostToContainer},
						{HostPath: "/b", Propagation: config.MountPropagationBidirectional},
					}},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := &testLogger{}
			// Must not panic on any platform.
			logMountPropagationPlatformWarning(l, tc.cfg)

			// Warn-once invariant: at most one warning emitted regardless of mount count.
			if len(l.lines) > 1 {
				t.Errorf("expected at most 1 warning (warn-once), got %d: %v", len(l.lines), l.lines)
			}
		})
	}
}
