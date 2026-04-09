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

package installlocalpath

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
				{}, // Step 1: kubectl apply manifest
				{}, // Step 2: kubectl wait deployment
			},
			wantErr:   false,
			wantCalls: 2,
		},
		{
			name: "apply manifest fails",
			cmds: []*testutil.FakeCmd{
				{Err: errors.New("fail")}, // Step 1: kubectl apply manifest fails
			},
			wantErr:     true,
			errContains: "failed to apply local-path-provisioner manifest",
		},
		{
			name: "wait deployment fails",
			cmds: []*testutil.FakeCmd{
				{},                           // Step 1: kubectl apply manifest succeeds
				{Err: errors.New("timeout")}, // Step 2: kubectl wait deployment fails
			},
			wantErr:     true,
			errContains: "did not become available",
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

func TestImages(t *testing.T) {
	if len(Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(Images))
	}
	hasProvisioner := false
	hasBusybox := false
	for _, img := range Images {
		if strings.Contains(img, "local-path-provisioner") {
			hasProvisioner = true
		}
		if strings.Contains(img, "busybox") {
			hasBusybox = true
		}
	}
	if !hasProvisioner {
		t.Error("Images missing local-path-provisioner")
	}
	if !hasBusybox {
		t.Error("Images missing busybox")
	}
}
