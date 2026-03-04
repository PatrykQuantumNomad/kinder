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

// Package testutil provides shared fake implementations of nodes.Node,
// exec.Cmd, and providers.Provider for use in addon action unit tests.
package testutil

import (
	"context"
	"io"

	"sigs.k8s.io/kind/pkg/cluster/constants"
	"sigs.k8s.io/kind/pkg/cluster/internal/create/actions"
	"sigs.k8s.io/kind/pkg/cluster/internal/providers"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/log"
)

// FakeCmd implements exec.Cmd for testing. It records the configured error and
// output, writing Output to the configured stdout writer on Run().
type FakeCmd struct {
	// Err is the error returned by Run.
	Err error
	// Output is written to the stdout writer on Run() if both are non-nil.
	Output []byte
	stdout io.Writer
}

var _ exec.Cmd = (*FakeCmd)(nil)

// Run writes Output to stdout (if configured) and returns Err.
func (c *FakeCmd) Run() error {
	if c.stdout != nil && len(c.Output) > 0 {
		_, _ = c.stdout.Write(c.Output)
	}
	return c.Err
}

// SetEnv is a no-op for FakeCmd; it returns the receiver for chaining.
func (c *FakeCmd) SetEnv(_ ...string) exec.Cmd {
	return c
}

// SetStdin is a no-op for FakeCmd; it returns the receiver for chaining.
func (c *FakeCmd) SetStdin(_ io.Reader) exec.Cmd {
	return c
}

// SetStdout stores the writer and returns the receiver for chaining.
func (c *FakeCmd) SetStdout(w io.Writer) exec.Cmd {
	c.stdout = w
	return c
}

// SetStderr is a no-op for FakeCmd; it returns the receiver for chaining.
func (c *FakeCmd) SetStderr(_ io.Writer) exec.Cmd {
	return c
}

// FakeNode implements nodes.Node for testing. It returns pre-configured
// FakeCmd values from a queue in order, recording every CommandContext call.
type FakeNode struct {
	name string
	role string

	// Cmds is the queue of FakeCmd values returned by CommandContext/Command.
	Cmds []*FakeCmd
	// Calls records every [command, args...] invocation.
	Calls [][]string
	idx   int
}

var _ nodes.Node = (*FakeNode)(nil)

// NewFakeControlPlane returns a FakeNode configured as a control-plane node.
func NewFakeControlPlane(name string, cmds []*FakeCmd) *FakeNode {
	return &FakeNode{
		name: name,
		role: constants.ControlPlaneNodeRoleValue,
		Cmds: cmds,
	}
}

// String returns the node name.
func (n *FakeNode) String() string { return n.name }

// Role returns the configured node role.
func (n *FakeNode) Role() (string, error) { return n.role, nil }

// IP returns a static placeholder IP address.
func (n *FakeNode) IP() (string, string, error) { return "10.0.0.1", "", nil }

// SerialLogs is a no-op for FakeNode.
func (n *FakeNode) SerialLogs(_ io.Writer) error { return nil }

// Command records the call and returns the next FakeCmd from the queue.
func (n *FakeNode) Command(command string, args ...string) exec.Cmd {
	return n.nextCmd(command, args...)
}

// CommandContext records the call and returns the next FakeCmd from the queue.
func (n *FakeNode) CommandContext(_ context.Context, command string, args ...string) exec.Cmd {
	return n.nextCmd(command, args...)
}

// nextCmd appends [command, args...] to Calls and returns the next FakeCmd.
// If the queue is exhausted a default success FakeCmd is returned.
func (n *FakeNode) nextCmd(command string, args ...string) exec.Cmd {
	entry := make([]string, 0, 1+len(args))
	entry = append(entry, command)
	entry = append(entry, args...)
	n.Calls = append(n.Calls, entry)

	if n.idx < len(n.Cmds) {
		cmd := n.Cmds[n.idx]
		n.idx++
		return cmd
	}
	return &FakeCmd{}
}

// FakeProvider implements providers.Provider for testing.
type FakeProvider struct {
	// Nodes is returned by ListNodes.
	Nodes []nodes.Node
	// InfoResp is returned by Info.
	InfoResp *providers.ProviderInfo
	// InfoErr is returned as the error from Info.
	InfoErr error
}

var _ providers.Provider = (*FakeProvider)(nil)

// Provision is a no-op.
func (p *FakeProvider) Provision(_ *cli.Status, _ *config.Cluster) error { return nil }

// ListClusters returns nil.
func (p *FakeProvider) ListClusters() ([]string, error) { return nil, nil }

// ListNodes returns the configured Nodes slice.
func (p *FakeProvider) ListNodes(_ string) ([]nodes.Node, error) { return p.Nodes, nil }

// DeleteNodes is a no-op.
func (p *FakeProvider) DeleteNodes(_ []nodes.Node) error { return nil }

// GetAPIServerEndpoint returns an empty string.
func (p *FakeProvider) GetAPIServerEndpoint(_ string) (string, error) { return "", nil }

// GetAPIServerInternalEndpoint returns an empty string.
func (p *FakeProvider) GetAPIServerInternalEndpoint(_ string) (string, error) { return "", nil }

// CollectLogs is a no-op.
func (p *FakeProvider) CollectLogs(_ string, _ []nodes.Node) error { return nil }

// Info returns the configured InfoResp and InfoErr.
func (p *FakeProvider) Info() (*providers.ProviderInfo, error) {
	return p.InfoResp, p.InfoErr
}

// NewTestContext constructs an ActionContext with a background context,
// noop logger, and minimal cluster config suitable for unit tests.
func NewTestContext(provider providers.Provider) *actions.ActionContext {
	logger := log.NoopLogger{}
	return actions.NewActionContext(
		context.Background(),
		logger,
		cli.StatusForLogger(logger),
		provider,
		&config.Cluster{Name: "test"},
	)
}
