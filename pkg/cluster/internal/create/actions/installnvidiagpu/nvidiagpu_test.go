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

package installnvidiagpu

import (
	"errors"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/testutil"
	"sigs.k8s.io/kind/pkg/cluster/internal/providers"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
)

func init() {
	// Override platform guard and pre-flight for unit tests
	// (no NVIDIA hardware in CI/dev machines).
	currentOS = "linux"
	checkPrerequisites = func() error { return nil }
}

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
				{}, // RuntimeClass apply
				{}, // DaemonSet apply
			},
			wantErr:   false,
			wantCalls: 2,
		},
		{
			name: "RuntimeClass apply fails",
			cmds: []*testutil.FakeCmd{
				{Err: errors.New("fail")},
			},
			wantErr:     true,
			errContains: "failed to apply NVIDIA RuntimeClass",
		},
		{
			name: "DaemonSet apply fails",
			cmds: []*testutil.FakeCmd{
				{},                        // RuntimeClass succeeds
				{Err: errors.New("fail")}, // DaemonSet fails
			},
			wantErr:     true,
			errContains: "failed to apply NVIDIA device plugin DaemonSet",
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

// TestExecute_NonLinuxSkips verifies that the addon skips with zero node calls on non-Linux platforms.
// NOT parallel — modifies the package-level currentOS var.
func TestExecute_NonLinuxSkips(t *testing.T) {
	saved := currentOS
	currentOS = "darwin"
	defer func() { currentOS = saved }()

	ctx, node := makeCtx(nil)
	err := NewAction().Execute(ctx)
	if err != nil {
		t.Errorf("Execute() on non-Linux should return nil, got: %v", err)
	}
	if len(node.Calls) != 0 {
		t.Errorf("Execute() on non-Linux should make 0 node calls, got %d", len(node.Calls))
	}
}

// TestExecute_PreflightFailure verifies that a pre-flight check failure prevents any node calls.
// NOT parallel — modifies the package-level checkPrerequisites var.
func TestExecute_PreflightFailure(t *testing.T) {
	saved := checkPrerequisites
	checkPrerequisites = func() error {
		return errors.New("nvidia-smi not found")
	}
	defer func() { checkPrerequisites = saved }()

	ctx, node := makeCtx(nil)
	err := NewAction().Execute(ctx)
	if err == nil {
		t.Error("Execute() should fail when pre-flight fails")
	}
	if len(node.Calls) != 0 {
		t.Errorf("Execute() should make 0 node calls on pre-flight failure, got %d", len(node.Calls))
	}
}
