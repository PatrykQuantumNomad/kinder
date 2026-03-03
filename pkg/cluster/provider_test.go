/*
Copyright 2024 The Kubernetes Authors.

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

package cluster

import (
	"testing"

	internalproviders "sigs.k8s.io/kind/pkg/cluster/internal/providers"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/log"
)

// mockProvider is a minimal internalproviders.Provider that records the name
// argument passed to ListNodes. All other methods are stubs.
type mockProvider struct {
	lastListNodesName string
}

var _ internalproviders.Provider = (*mockProvider)(nil)

func (m *mockProvider) ListNodes(cluster string) ([]nodes.Node, error) {
	m.lastListNodesName = cluster
	return nil, nil
}

func (m *mockProvider) Provision(_ *cli.Status, _ *config.Cluster) error { return nil }
func (m *mockProvider) ListClusters() ([]string, error)                  { return nil, nil }
func (m *mockProvider) DeleteNodes(_ []nodes.Node) error                 { return nil }
func (m *mockProvider) GetAPIServerEndpoint(_ string) (string, error)    { return "", nil }
func (m *mockProvider) GetAPIServerInternalEndpoint(_ string) (string, error) {
	return "", nil
}
func (m *mockProvider) CollectLogs(_ string, _ []nodes.Node) error { return nil }
func (m *mockProvider) Info() (*internalproviders.ProviderInfo, error) {
	return &internalproviders.ProviderInfo{}, nil
}

// TestListInternalNodes_DefaultName verifies that ListInternalNodes resolves an
// empty name to the default cluster name before passing it to the internal
// provider — consistent with ListNodes and all other Provider methods.
func TestListInternalNodes_DefaultName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		inputName    string
		expectedName string
	}{
		{
			inputName:    "",
			expectedName: DefaultName,
		},
		{
			inputName:    "custom",
			expectedName: "custom",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("name="+tc.inputName, func(t *testing.T) {
			t.Parallel()
			mock := &mockProvider{}
			p := &Provider{
				provider: mock,
				logger:   log.NoopLogger{},
			}
			_, _ = p.ListInternalNodes(tc.inputName)
			if mock.lastListNodesName != tc.expectedName {
				t.Errorf(
					"ListInternalNodes(%q) called ListNodes with %q, want %q",
					tc.inputName, mock.lastListNodesName, tc.expectedName,
				)
			}
		})
	}
}
