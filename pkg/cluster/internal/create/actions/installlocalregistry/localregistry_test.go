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

package installlocalregistry

import (
	"errors"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions/testutil"
	"sigs.k8s.io/kind/pkg/cluster/internal/providers"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
)

// dockerAvailable returns true if a Docker daemon is reachable.
// This uses sigs.k8s.io/kind/pkg/exec (not os/exec).
func dockerAvailable() bool {
	return exec.Command("docker", "version").Run() == nil
}

// TestExecute_InfoError verifies that a failure from Provider.Info() is
// propagated as an error before any host-side exec.Command calls are made.
// This test does NOT require a Docker daemon and always runs.
func TestExecute_InfoError(t *testing.T) {
	t.Parallel()
	provider := &testutil.FakeProvider{
		InfoResp: nil,
		InfoErr:  errors.New("info failed"),
	}
	// FakeProvider does not implement fmt.Stringer, so binaryName defaults to
	// "docker" inside Execute(). The Info() call happens before any exec.Command
	// invocation, so no Docker daemon is needed.
	ctx := testutil.NewTestContext(provider)
	action := NewAction()
	err := action.Execute(ctx)
	if err == nil {
		t.Fatal("Execute() expected error from Info(), got nil")
	}
	if !strings.Contains(err.Error(), "failed to get provider info") {
		t.Errorf("Execute() error = %q, want error containing %q", err.Error(), "failed to get provider info")
	}
}

// TestExecute_FullPath tests the full Execute() flow including host-side Docker
// operations and node-side containerd patching.
//
// Host-side calls (docker inspect, docker run, docker network connect) cannot be
// intercepted via FakeNode — they invoke the real Docker CLI via exec.Command.
// This test is therefore SKIPPED when a Docker daemon is not available.
//
// When Docker IS available the test verifies that:
//   - Execute() returns nil (success)
//   - The FakeNode Calls queue contains at least 3 entries: mkdir, tee (for the
//     single control-plane node), and kubectl apply for the ConfigMap.
func TestExecute_FullPath(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("requires Docker daemon")
	}
	// Defer cleanup of the registry container so it does not pollute other tests.
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", registryName).Run()
	})

	// Provide 3 FakeCmds for the single control-plane node:
	//   1. mkdir -p /etc/containerd/certs.d/localhost:5001
	//   2. tee /etc/containerd/certs.d/localhost:5001/hosts.toml
	//   3. kubectl apply -f - (ConfigMap)
	cp := testutil.NewFakeControlPlane("cp1", []*testutil.FakeCmd{
		{}, // mkdir
		{}, // tee
		{}, // kubectl apply
	})
	provider := &testutil.FakeProvider{
		Nodes:    []nodes.Node{cp},
		InfoResp: &providers.ProviderInfo{Rootless: false},
	}
	ctx := testutil.NewTestContext(provider)

	action := NewAction()
	err := action.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if len(cp.Calls) < 3 {
		t.Errorf("Execute() node.Calls = %d, want at least 3 (mkdir, tee, kubectl apply)", len(cp.Calls))
	}
}

// TestExecute_NodePatchingErrors verifies that errors from node-side commands
// propagate with descriptive messages.
//
// Because the node-side patching (Step 3) only runs after the host-side Docker
// steps (Steps 1–2) succeed, this test also requires a Docker daemon.
func TestExecute_NodePatchingErrors(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("requires Docker daemon")
	}
	// Defer cleanup of the registry container once (shared across sub-tests).
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", registryName).Run()
	})

	tests := []struct {
		name        string
		cmds        []*testutil.FakeCmd
		errContains string
	}{
		{
			name: "mkdir fails on first node",
			cmds: []*testutil.FakeCmd{
				{Err: errors.New("mkdir failed")}, // mkdir
			},
			errContains: "failed to create certs.d dir",
		},
		{
			name: "tee fails on first node",
			cmds: []*testutil.FakeCmd{
				{},                                  // mkdir succeeds
				{Err: errors.New("tee failed")},     // tee fails
			},
			errContains: "failed to write hosts.toml",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Sub-tests share the Docker daemon; do not run in parallel to
			// avoid concurrent mutations of the kind-registry container.
			cp := testutil.NewFakeControlPlane("cp1", tc.cmds)
			provider := &testutil.FakeProvider{
				Nodes:    []nodes.Node{cp},
				InfoResp: &providers.ProviderInfo{Rootless: false},
			}
			// Each sub-test needs a fresh ActionContext to avoid cache reuse.
			ctx := testutil.NewTestContext(provider)

			action := NewAction()
			err := action.Execute(ctx)
			if err == nil {
				t.Errorf("Execute() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("Execute() error = %q, want error containing %q", err.Error(), tc.errContains)
			}
		})
	}
}
