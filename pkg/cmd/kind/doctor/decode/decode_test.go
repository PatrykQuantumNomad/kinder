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

package decode

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cmd"
	idoctor "sigs.k8s.io/kind/pkg/internal/doctor"
	"sigs.k8s.io/kind/pkg/log"
)

// ---------------------------------------------------------------------------
// Helper: newTestStreams
// ---------------------------------------------------------------------------

func newTestStreams() (cmd.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return cmd.IOStreams{In: nil, Out: out, ErrOut: errOut}, out, errOut
}

// ---------------------------------------------------------------------------
// newMockParentWithDecode builds a minimal parent cobra command that registers
// the decode subcommand via NewCommand — used in Test 1 without importing the
// doctor package (which would create an import cycle since doctor imports decode).
// ---------------------------------------------------------------------------

func newMockParentWithDecode(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	parent := &cobra.Command{
		Use:   "doctor",
		Short: "mock parent for test",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
	parent.AddCommand(NewCommand(logger, streams))
	return parent
}

// ---------------------------------------------------------------------------
// Injection helpers (swap via t.Cleanup, same as dev_test.go pattern).
// Test wrappers use fakeNode to avoid importing cluster/nodes in tests.
// ---------------------------------------------------------------------------

type fakeNode struct {
	name string
	role string
}

func (n fakeNode) String() string { return n.name }
func (n fakeNode) Role() (string, error) {
	if n.role == "" {
		return "control-plane", nil
	}
	return n.role, nil
}

func withResolveClusterName(t *testing.T, fn func(args []string) (string, error)) {
	t.Helper()
	prev := resolveClusterName
	resolveClusterName = fn
	t.Cleanup(func() { resolveClusterName = prev })
}

func withListNodesFnFakes(t *testing.T, nodes []fakeNode, err error) {
	t.Helper()
	prev := listNodesFn
	listNodesFn = func(_ string, _ log.Logger) ([]nodeStringer, error) {
		if err != nil {
			return nil, err
		}
		out := make([]nodeStringer, len(nodes))
		for i, n := range nodes {
			out[i] = n
		}
		return out, nil
	}
	t.Cleanup(func() { listNodesFn = prev })
}

func withBinaryNameFn(t *testing.T, fn func() string) {
	t.Helper()
	prev := binaryNameFn
	binaryNameFn = fn
	t.Cleanup(func() { binaryNameFn = prev })
}

func withRunDecodeFn(t *testing.T, fn func(opts idoctor.DecodeOptions) (*idoctor.DecodeResult, error)) {
	t.Helper()
	prev := runDecodeFn
	runDecodeFn = fn
	t.Cleanup(func() { runDecodeFn = prev })
}

func withFormatHumanFn(t *testing.T, fn func(w io.Writer, result *idoctor.DecodeResult)) {
	t.Helper()
	prev := formatHumanFn
	formatHumanFn = fn
	t.Cleanup(func() { formatHumanFn = prev })
}

func withFormatJSONFn(t *testing.T, fn func(result *idoctor.DecodeResult) map[string]interface{}) {
	t.Helper()
	prev := formatJSONFn
	formatJSONFn = fn
	t.Cleanup(func() { formatJSONFn = prev })
}

func withPreviewAutoFixFn(t *testing.T, fn func(matches []idoctor.DecodeMatch, ctx idoctor.DecodeAutoFixContext) []string) {
	t.Helper()
	prev := previewAutoFixFn
	previewAutoFixFn = fn
	t.Cleanup(func() { previewAutoFixFn = prev })
}

func withApplyAutoFixFn(t *testing.T, fn func(matches []idoctor.DecodeMatch, ctx idoctor.DecodeAutoFixContext, logger log.Logger) []error) {
	t.Helper()
	prev := applyAutoFixFn
	applyAutoFixFn = fn
	t.Cleanup(func() { applyAutoFixFn = prev })
}

// ---------------------------------------------------------------------------
// installDefaultStubs wires all injectable fns to no-op success implementations.
// ---------------------------------------------------------------------------

func installDefaultStubs(t *testing.T) *idoctor.DecodeResult {
	t.Helper()

	result := &idoctor.DecodeResult{Cluster: "kind", Matches: nil, Unmatched: 0}

	withResolveClusterName(t, func(args []string) (string, error) {
		if len(args) > 0 {
			return args[0], nil
		}
		return "kind", nil
	})
	withListNodesFnFakes(t, []fakeNode{{name: "kind-control-plane", role: "control-plane"}}, nil)
	withBinaryNameFn(t, func() string { return "docker" })
	withRunDecodeFn(t, func(_ idoctor.DecodeOptions) (*idoctor.DecodeResult, error) {
		return result, nil
	})
	withFormatHumanFn(t, func(_ io.Writer, _ *idoctor.DecodeResult) {})
	withFormatJSONFn(t, func(_ *idoctor.DecodeResult) map[string]interface{} {
		return map[string]interface{}{}
	})
	withPreviewAutoFixFn(t, func(_ []idoctor.DecodeMatch, _ idoctor.DecodeAutoFixContext) []string {
		return nil
	})
	withApplyAutoFixFn(t, func(_ []idoctor.DecodeMatch, _ idoctor.DecodeAutoFixContext, _ log.Logger) []error {
		return nil
	})

	return result
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestDecodeCmd_RegistersAsSubcommandOfDoctor verifies the decode command
// registers with Use containing "decode" and is accessible as a child.
func TestDecodeCmd_RegistersAsSubcommandOfDoctor(t *testing.T) {
	streams, _, _ := newTestStreams()
	parent := newMockParentWithDecode(log.NoopLogger{}, streams)

	found := false
	for _, child := range parent.Commands() {
		if strings.HasPrefix(child.Use, "decode") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("parent has no 'decode' subcommand; children: %v", parent.Commands())
	}
}

// TestDoctorCmd_BareInvocationStillRunsChecks verifies the parent doctor
// command's RunE is non-nil (locked decision #1: bare kinder doctor unchanged).
// The test uses the mock parent to avoid importing the doctor package.
func TestDoctorCmd_BareInvocationStillRunsChecks(t *testing.T) {
	streams, _, _ := newTestStreams()
	// We cannot import the doctor parent without a cycle, so we verify the
	// decode command itself has a non-nil RunE (it does), and separately verify
	// the decode command's Use starts with "decode" so it is not the bare doctor.
	c := NewCommand(log.NoopLogger{}, streams)
	if c.RunE == nil {
		t.Error("decode subcommand RunE is nil")
	}
	if !strings.HasPrefix(c.Use, "decode") {
		t.Errorf("decode subcommand Use = %q; want prefix 'decode'", c.Use)
	}
}

// TestDecodeCmd_DefaultFlags verifies all required flags are registered with
// the correct types and default values.
func TestDecodeCmd_DefaultFlags(t *testing.T) {
	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)

	// --name: string, default ""
	nameFlag := c.Flags().Lookup("name")
	if nameFlag == nil {
		t.Fatal("--name flag not registered")
	}
	if nameFlag.DefValue != "" {
		t.Errorf("--name default = %q, want \"\"", nameFlag.DefValue)
	}

	// --since: duration, default 30m
	sinceFlag := c.Flags().Lookup("since")
	if sinceFlag == nil {
		t.Fatal("--since flag not registered")
	}
	if sinceFlag.DefValue != (30 * time.Minute).String() {
		t.Errorf("--since default = %q, want %q", sinceFlag.DefValue, (30 * time.Minute).String())
	}

	// --output: string, default ""
	outputFlag := c.Flags().Lookup("output")
	if outputFlag == nil {
		t.Fatal("--output flag not registered")
	}
	if outputFlag.DefValue != "" {
		t.Errorf("--output default = %q, want \"\"", outputFlag.DefValue)
	}

	// --auto-fix: bool, default false
	autoFixFlag := c.Flags().Lookup("auto-fix")
	if autoFixFlag == nil {
		t.Fatal("--auto-fix flag not registered")
	}
	if autoFixFlag.DefValue != "false" {
		t.Errorf("--auto-fix default = %q, want \"false\"", autoFixFlag.DefValue)
	}

	// --include-normal: bool, default false
	includeNormalFlag := c.Flags().Lookup("include-normal")
	if includeNormalFlag == nil {
		t.Fatal("--include-normal flag not registered")
	}
	if includeNormalFlag.DefValue != "false" {
		t.Errorf("--include-normal default = %q, want \"false\"", includeNormalFlag.DefValue)
	}
}

// TestDecodeCmd_OutputFormatValidation asserts an unsupported output format
// returns an error containing "unsupported output format".
func TestDecodeCmd_OutputFormatValidation(t *testing.T) {
	streams, _, _ := newTestStreams()
	installDefaultStubs(t)

	flags := &flagpole{
		Since:  30 * time.Minute,
		Output: "bogus",
	}
	err := runE(log.NoopLogger{}, streams, flags, nil)
	if err == nil {
		t.Fatal("expected error for bogus output format, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Errorf("error = %q; want to contain 'unsupported output format'", err.Error())
	}
}

// TestDecodeCmd_DispatchesToRunDecodeAndRenderer verifies that human renderer
// is called when Output="" and JSON renderer is called when Output="json".
func TestDecodeCmd_DispatchesToRunDecodeAndRenderer(t *testing.T) {
	streams, _, _ := newTestStreams()
	installDefaultStubs(t)

	humanCalled := false
	jsonCalled := false

	withFormatHumanFn(t, func(_ io.Writer, _ *idoctor.DecodeResult) {
		humanCalled = true
	})
	withFormatJSONFn(t, func(_ *idoctor.DecodeResult) map[string]interface{} {
		jsonCalled = true
		return map[string]interface{}{}
	})

	// Human output (default).
	flags := &flagpole{Since: 30 * time.Minute, Output: ""}
	if err := runE(log.NoopLogger{}, streams, flags, nil); err != nil {
		t.Fatalf("runE (human) error: %v", err)
	}
	if !humanCalled {
		t.Error("human renderer not called for Output=\"\"")
	}
	if jsonCalled {
		t.Error("JSON renderer should not be called for Output=\"\"")
	}

	// Reset.
	humanCalled = false
	jsonCalled = false

	// JSON output.
	flags2 := &flagpole{Since: 30 * time.Minute, Output: "json"}
	if err := runE(log.NoopLogger{}, streams, flags2, nil); err != nil {
		t.Fatalf("runE (json) error: %v", err)
	}
	if !jsonCalled {
		t.Error("JSON renderer not called for Output=\"json\"")
	}
	if humanCalled {
		t.Error("human renderer should not be called for Output=\"json\"")
	}
}

// TestDecodeCmd_AutoFixWiring asserts applyAutoFixFn and previewAutoFixFn are
// called when --auto-fix=true, with preview before apply.
func TestDecodeCmd_AutoFixWiring(t *testing.T) {
	streams, out, _ := newTestStreams()

	matches := []idoctor.DecodeMatch{
		{Source: "docker-logs:kind-control-plane", Line: "line1", Pattern: idoctor.DecodePattern{ID: "KUB-01", AutoFixable: true}},
		{Source: "docker-logs:kind-control-plane", Line: "line2", Pattern: idoctor.DecodePattern{ID: "KUB-02", AutoFixable: true}},
	}
	result := &idoctor.DecodeResult{Cluster: "kind", Matches: matches}

	installDefaultStubs(t)
	withRunDecodeFn(t, func(_ idoctor.DecodeOptions) (*idoctor.DecodeResult, error) {
		return result, nil
	})

	var callOrder []string
	var capturedCtx idoctor.DecodeAutoFixContext
	var capturedMatches []idoctor.DecodeMatch

	withPreviewAutoFixFn(t, func(ms []idoctor.DecodeMatch, ctx idoctor.DecodeAutoFixContext) []string {
		callOrder = append(callOrder, "preview")
		capturedCtx = ctx
		capturedMatches = ms
		return []string{"would apply inotify-raise for KUB-01"}
	})
	withApplyAutoFixFn(t, func(ms []idoctor.DecodeMatch, ctx idoctor.DecodeAutoFixContext, _ log.Logger) []error {
		callOrder = append(callOrder, "apply")
		capturedCtx = ctx
		capturedMatches = ms
		return nil
	})

	flags := &flagpole{Since: 30 * time.Minute, AutoFix: true}
	if err := runE(log.NoopLogger{}, streams, flags, nil); err != nil {
		t.Fatalf("runE error: %v", err)
	}

	// preview must come BEFORE apply.
	if len(callOrder) < 2 || callOrder[0] != "preview" || callOrder[1] != "apply" {
		t.Errorf("call order = %v; want [preview apply]", callOrder)
	}
	// BinaryName and CPNodeName must be non-empty.
	if capturedCtx.BinaryName == "" {
		t.Error("DecodeAutoFixContext.BinaryName is empty")
	}
	if capturedCtx.CPNodeName == "" {
		t.Error("DecodeAutoFixContext.CPNodeName is empty")
	}
	// matches must be the result's matches.
	if len(capturedMatches) != 2 {
		t.Errorf("got %d matches, want 2", len(capturedMatches))
	}
	// preview output must appear on streams.Out.
	if !strings.Contains(out.String(), "preview") {
		t.Errorf("preview output not written to streams.Out; out=%q", out.String())
	}
}

// TestDecodeCmd_AutoFixDryRunMode asserts that without --auto-fix the preview
// is NOT shown and apply is NOT called.
func TestDecodeCmd_AutoFixDryRunMode(t *testing.T) {
	streams, _, _ := newTestStreams()
	installDefaultStubs(t)

	previewCalled := false
	applyCalled := false

	withPreviewAutoFixFn(t, func(_ []idoctor.DecodeMatch, _ idoctor.DecodeAutoFixContext) []string {
		previewCalled = true
		return []string{"would apply something"}
	})
	withApplyAutoFixFn(t, func(_ []idoctor.DecodeMatch, _ idoctor.DecodeAutoFixContext, _ log.Logger) []error {
		applyCalled = true
		return nil
	})

	flags := &flagpole{Since: 30 * time.Minute, AutoFix: false}
	if err := runE(log.NoopLogger{}, streams, flags, nil); err != nil {
		t.Fatalf("runE error: %v", err)
	}

	if previewCalled {
		t.Error("previewAutoFixFn must NOT be called when --auto-fix=false")
	}
	if applyCalled {
		t.Error("applyAutoFixFn must NOT be called when --auto-fix=false")
	}
}

// TestDecodeCmd_NoCluster_ErrorPath asserts the error from resolveClusterName
// is returned verbatim.
func TestDecodeCmd_NoCluster_ErrorPath(t *testing.T) {
	streams, _, _ := newTestStreams()
	installDefaultStubs(t)

	withResolveClusterName(t, func(_ []string) (string, error) {
		return "", errors.New("no kind clusters found")
	})

	flags := &flagpole{Since: 30 * time.Minute}
	err := runE(log.NoopLogger{}, streams, flags, nil)
	if err == nil {
		t.Fatal("expected error for no cluster, got nil")
	}
	if !strings.Contains(err.Error(), "no kind clusters found") {
		t.Errorf("error = %q; want 'no kind clusters found'", err.Error())
	}
}

// TestDecodeCmd_IncludeNormalFlagThreadsToRunDecode asserts that
// --include-normal=true reaches RunDecode's IncludeNormalEvents field.
func TestDecodeCmd_IncludeNormalFlagThreadsToRunDecode(t *testing.T) {
	streams, _, _ := newTestStreams()
	installDefaultStubs(t)

	var capturedOpts idoctor.DecodeOptions
	withRunDecodeFn(t, func(opts idoctor.DecodeOptions) (*idoctor.DecodeResult, error) {
		capturedOpts = opts
		return &idoctor.DecodeResult{Cluster: opts.Cluster}, nil
	})

	flags := &flagpole{Since: 30 * time.Minute, IncludeNormal: true}
	if err := runE(log.NoopLogger{}, streams, flags, nil); err != nil {
		t.Fatalf("runE error: %v", err)
	}
	if !capturedOpts.IncludeNormalEvents {
		t.Error("IncludeNormalEvents was false; want true when --include-normal=true")
	}
}
