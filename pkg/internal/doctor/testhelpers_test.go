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
	"bytes"
	"fmt"
	"io"
	"strings"

	"sigs.k8s.io/kind/pkg/exec"
)

// fakeExecResult holds the canned output for a fake command execution.
type fakeExecResult struct {
	lines string // newline-separated output lines
	err   error  // error to return from Run()
}

// fakeCmd implements exec.Cmd for testing. It captures stdout writer and
// writes canned output on Run().
type fakeCmd struct {
	output string
	err    error
	stdout io.Writer
}

func (f *fakeCmd) Run() error {
	if f.stdout != nil && f.output != "" {
		fmt.Fprint(f.stdout, f.output)
	}
	return f.err
}

func (f *fakeCmd) SetEnv(env ...string) exec.Cmd   { return f }
func (f *fakeCmd) SetStdin(r io.Reader) exec.Cmd    { return f }
func (f *fakeCmd) SetStdout(w io.Writer) exec.Cmd   { f.stdout = w; return f }
func (f *fakeCmd) SetStderr(w io.Writer) exec.Cmd   { return f }

// newFakeExecCmd returns an execCmd function that maps command strings to
// fakeExecResult values. The key is the command name + args joined by spaces.
func newFakeExecCmd(results map[string]fakeExecResult) func(name string, args ...string) exec.Cmd {
	return func(name string, args ...string) exec.Cmd {
		key := name
		if len(args) > 0 {
			key = name + " " + strings.Join(args, " ")
		}
		if result, ok := results[key]; ok {
			return &fakeCmd{
				output: result.lines,
				err:    result.err,
			}
		}
		// Unknown command returns empty output and no error by default.
		return &fakeCmd{}
	}
}

// Ensure fakeCmd satisfies exec.Cmd at compile time.
var _ exec.Cmd = &fakeCmd{}

// captureOutput is a helper for capturing formatted output.
func captureOutput() *bytes.Buffer {
	return &bytes.Buffer{}
}
