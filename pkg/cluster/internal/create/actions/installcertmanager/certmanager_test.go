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

package installcertmanager

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
				{}, // Step 1: kubectl apply --server-side (cert-manager manifest)
				{}, // Step 2: kubectl wait deployment/cert-manager
				{}, // Step 3: kubectl wait deployment/cert-manager-cainjector
				{}, // Step 4: kubectl wait deployment/cert-manager-webhook
				{}, // Step 5: kubectl apply (ClusterIssuer)
			},
			wantErr:   false,
			wantCalls: 5,
		},
		{
			name: "apply manifest fails",
			cmds: []*testutil.FakeCmd{
				{Err: errors.New("fail")}, // Step 1: kubectl apply --server-side fails
			},
			wantErr:     true,
			errContains: "failed to apply cert-manager manifest",
		},
		{
			name: "wait cert-manager fails",
			cmds: []*testutil.FakeCmd{
				{},                           // Step 1: kubectl apply --server-side succeeds
				{Err: errors.New("timeout")}, // Step 2: kubectl wait deployment/cert-manager fails
			},
			wantErr:     true,
			errContains: "cert-manager",
		},
		{
			name: "wait cainjector fails",
			cmds: []*testutil.FakeCmd{
				{},                           // Step 1: kubectl apply --server-side succeeds
				{},                           // Step 2: kubectl wait deployment/cert-manager succeeds
				{Err: errors.New("timeout")}, // Step 3: kubectl wait deployment/cert-manager-cainjector fails
			},
			wantErr:     true,
			errContains: "cert-manager-cainjector",
		},
		{
			name: "wait webhook fails",
			cmds: []*testutil.FakeCmd{
				{},                           // Step 1: kubectl apply --server-side succeeds
				{},                           // Step 2: kubectl wait deployment/cert-manager succeeds
				{},                           // Step 3: kubectl wait deployment/cert-manager-cainjector succeeds
				{Err: errors.New("timeout")}, // Step 4: kubectl wait deployment/cert-manager-webhook fails
			},
			wantErr:     true,
			errContains: "cert-manager-webhook",
		},
		{
			name: "apply ClusterIssuer fails",
			cmds: []*testutil.FakeCmd{
				{},                        // Step 1: kubectl apply --server-side succeeds
				{},                        // Step 2: kubectl wait deployment/cert-manager succeeds
				{},                        // Step 3: kubectl wait deployment/cert-manager-cainjector succeeds
				{},                        // Step 4: kubectl wait deployment/cert-manager-webhook succeeds
				{Err: errors.New("fail")}, // Step 5: kubectl apply ClusterIssuer fails
			},
			wantErr:     true,
			errContains: "failed to apply selfsigned ClusterIssuer",
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

// TestImagesPinsV1202 pins all three cert-manager images to v1.20.2 (ADDON-03).
func TestImagesPinsV1202(t *testing.T) {
	t.Parallel()
	wants := []string{
		"quay.io/jetstack/cert-manager-cainjector:v1.20.2",
		"quay.io/jetstack/cert-manager-controller:v1.20.2",
		"quay.io/jetstack/cert-manager-webhook:v1.20.2",
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

// containsStr returns true if s appears anywhere in the slice.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// TestExecuteUsesServerSideApply guards Pitfall CERT-01: cert-manager manifest
// is ~992 KB (>256 KB annotation limit), so the apply call MUST use --server-side.
// We run Execute against a successful FakeCmd queue, then inspect the FakeNode's
// Calls slice (every Command/CommandContext invocation is recorded as
// [command, args...]) and assert that at least one kubectl-apply invocation
// carries the --server-side flag.
//
// The cert-manager Execute() runs:
//
//	Step 1: kubectl --kubeconfig=... apply --server-side -f -    (cert-manager.yaml)
//	Step 2: kubectl --kubeconfig=... wait deployment/cert-manager
//	Step 3: kubectl --kubeconfig=... wait deployment/cert-manager-cainjector
//	Step 4: kubectl --kubeconfig=... wait deployment/cert-manager-webhook
//	Step 5: kubectl --kubeconfig=... apply -f -                  (selfsigned ClusterIssuer)
func TestExecuteUsesServerSideApply(t *testing.T) {
	t.Parallel()
	cmds := []*testutil.FakeCmd{
		{}, // Step 1: kubectl apply --server-side cert-manager.yaml
		{}, // Step 2: kubectl wait deployment/cert-manager
		{}, // Step 3: kubectl wait deployment/cert-manager-cainjector
		{}, // Step 4: kubectl wait deployment/cert-manager-webhook
		{}, // Step 5: kubectl apply selfsigned ClusterIssuer
		{}, // headroom
	}
	ctx, node := makeCtx(cmds)
	if err := NewAction().Execute(ctx); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Find the first kubectl-apply invocation (the manifest apply, not the ClusterIssuer apply).
	// call = [command, args...] where command == "kubectl".
	// The actual argv includes --kubeconfig before "apply", so we search for "apply"
	// anywhere in the slice rather than at a fixed index.
	var manifestApply []string
	for _, call := range node.Calls {
		if len(call) >= 1 && call[0] == "kubectl" && containsStr(call, "apply") {
			manifestApply = call
			break
		}
	}
	if manifestApply == nil {
		t.Fatalf("no kubectl apply invocation captured; Calls=%v", node.Calls)
	}

	// Assert --server-side appears somewhere in the captured argv.
	if !containsStr(manifestApply, "--server-side") {
		t.Errorf("kubectl apply argv missing --server-side flag (Pitfall CERT-01); got %v", manifestApply)
	}
}
