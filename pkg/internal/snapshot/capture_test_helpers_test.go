/*
Copyright 2026 The Kubernetes Authors.

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

package snapshot

import (
	"context"
	"io"
	"os"

	"sigs.k8s.io/kind/pkg/exec"
)

// captureCallbackNode is a nodes.Node test double whose Command/CommandContext
// calls are routed through a per-call lookup function keyed by (name, args).
// It is the snapshot-package equivalent of lifecycle's commandCallbackNode.
type captureCallbackNode struct {
	name   string
	role   string
	lookup func(name string, args []string) (string, error)
}

func (n *captureCallbackNode) String() string { return n.name }
func (n *captureCallbackNode) Role() (string, error) {
	return n.role, nil
}
func (n *captureCallbackNode) IP() (string, string, error) { return "", "", nil }
func (n *captureCallbackNode) SerialLogs(_ io.Writer) error { return nil }

func (n *captureCallbackNode) Command(c string, a ...string) exec.Cmd {
	return n.CommandContext(context.Background(), c, a...)
}

func (n *captureCallbackNode) CommandContext(_ context.Context, c string, a ...string) exec.Cmd {
	stdout := ""
	var err error
	if n.lookup != nil {
		stdout, err = n.lookup(c, a)
	}
	return &captureTestCmd{stdout: stdout, err: err}
}

// captureTestCmd implements exec.Cmd for capture tests.
type captureTestCmd struct {
	stdout  string
	err     error
	stdoutW io.Writer
}

func (c *captureTestCmd) Run() error {
	if c.stdoutW != nil && c.stdout != "" {
		_, _ = c.stdoutW.Write([]byte(c.stdout))
	}
	return c.err
}
func (c *captureTestCmd) SetEnv(_ ...string) exec.Cmd    { return c }
func (c *captureTestCmd) SetStdin(_ io.Reader) exec.Cmd  { return c }
func (c *captureTestCmd) SetStdout(w io.Writer) exec.Cmd { c.stdoutW = w; return c }
func (c *captureTestCmd) SetStderr(_ io.Writer) exec.Cmd { return c }

// fakeExitError simulates a non-zero exit from a command.
type fakeExitError struct {
	msg string
}

func (e *fakeExitError) Error() string { return e.msg }

// multiNodeCapture implements nodes.Node for multi-node PV/topology tests.
type multiNodeCapture struct {
	captureCallbackNode
}

// openFileForRead is a test helper that opens a file for reading, failing the test if unable.
func openFileForRead(path string) (*os.File, error) {
	return os.Open(path)
}
