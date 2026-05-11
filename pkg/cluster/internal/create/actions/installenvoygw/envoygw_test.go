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

package installenvoygw

import (
	"errors"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/testutil"
	"sigs.k8s.io/kind/pkg/cluster/internal/providers"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

// makeCtx creates an ActionContext and FakeNode wired to the given cmd queue.
func makeCtx(cmds []*testutil.FakeCmd) (*actions.ActionContext, *testutil.FakeNode) {
	node := testutil.NewFakeControlPlane("cp1", cmds)
	provider := &testutil.FakeProvider{
		Nodes:    []nodes.Node{node},
		InfoResp: &providers.ProviderInfo{},
	}
	return testutil.NewTestContext(provider), node
}

func TestExecute(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		cmds        []*testutil.FakeCmd
		wantErr     bool
		errContains string
		wantCalls   int
	}{
		{
			name: "all steps succeed",
			cmds: []*testutil.FakeCmd{
				{}, // Step 1: kubectl apply Envoy Gateway manifest (server-side)
				{}, // Step 2: kubectl wait certgen job complete
				{}, // Step 3: kubectl wait envoy-gateway deployment available
				{}, // Step 4: kubectl apply GatewayClass manifest
				{}, // Step 5: kubectl wait GatewayClass accepted
			},
			wantErr:   false,
			wantCalls: 5,
		},
		{
			name: "apply manifest fails",
			cmds: []*testutil.FakeCmd{
				{Err: errors.New("fail")}, // Step 1 fails
			},
			wantErr:     true,
			errContains: "failed to apply Envoy Gateway manifest",
		},
		{
			name: "wait certgen fails",
			cmds: []*testutil.FakeCmd{
				{},                        // Step 1 succeeds
				{Err: errors.New("timeout")}, // Step 2 fails
			},
			wantErr:     true,
			errContains: "certgen job did not complete",
		},
		{
			name: "wait deployment fails",
			cmds: []*testutil.FakeCmd{
				{}, // Step 1 succeeds
				{}, // Step 2 succeeds
				{Err: errors.New("timeout")}, // Step 3 fails
			},
			wantErr:     true,
			errContains: "did not become available",
		},
		{
			name: "apply GatewayClass fails",
			cmds: []*testutil.FakeCmd{
				{}, // Step 1 succeeds
				{}, // Step 2 succeeds
				{}, // Step 3 succeeds
				{Err: errors.New("fail")}, // Step 4 fails
			},
			wantErr:     true,
			errContains: "failed to apply Envoy Gateway GatewayClass",
		},
		{
			name: "wait GatewayClass fails",
			cmds: []*testutil.FakeCmd{
				{}, // Step 1 succeeds
				{}, // Step 2 succeeds
				{}, // Step 3 succeeds
				{}, // Step 4 succeeds
				{Err: errors.New("timeout")}, // Step 5 fails
			},
			wantErr:     true,
			errContains: "was not accepted",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, node := makeCtx(tc.cmds)
			action := NewAction()
			err := action.Execute(ctx)
			if tc.wantErr {
				if err == nil {
					t.Errorf("Execute() expected error, got nil")
					return
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("Execute() error = %q, want error containing %q", err.Error(), tc.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("Execute() unexpected error: %v", err)
				return
			}
			if tc.wantCalls > 0 && len(node.Calls) != tc.wantCalls {
				t.Errorf("Execute() node.Calls = %d, want %d", len(node.Calls), tc.wantCalls)
			}
		})
	}
}

// TestImagesPinsEGV172 pins both EG images to the v1.7.2 set (ADDON-04).
func TestImagesPinsEGV172(t *testing.T) {
	t.Parallel()
	wants := []string{
		"envoyproxy/gateway:v1.7.2",
		"docker.io/envoyproxy/ratelimit:05c08d03",
	}
	for _, want := range wants {
		found := false
		for _, img := range Images {
			if img == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Images = %v; want to contain %q", Images, want)
		}
	}
}

// TestManifestContainsCertgenJobName guards Pitfall EG-02: the kinder action
// waits on Job/eg-gateway-helm-certgen by name. If upstream renames it, the
// wait times out. Researcher confirmed v1.7.2 still ships this job name
// (line 52747 of upstream install.yaml); this test is the forward regression net.
func TestManifestContainsCertgenJobName(t *testing.T) {
	t.Parallel()
	const want = "name: eg-gateway-helm-certgen"
	if !strings.Contains(envoyGWManifest, want) {
		t.Errorf("envoyGWManifest missing %q (kinder action waits on this Job by hardcoded name)", want)
	}
}

// TestManifestPinsGatewayAPIBundleV141 pins the bundled Gateway API CRD
// version to v1.4.1 (was v1.2.1 in EG v1.3.1). The bundle-version annotation
// is set on the Gateway API CRDs themselves, distributed inside EG's install.yaml.
func TestManifestPinsGatewayAPIBundleV141(t *testing.T) {
	t.Parallel()
	const want = "gateway.networking.k8s.io/bundle-version: v1.4.1"
	if !strings.Contains(envoyGWManifest, want) {
		t.Errorf("envoyGWManifest missing %q (Gateway API CRDs must bundle v1.4.1 in EG v1.7.2)", want)
	}
}
