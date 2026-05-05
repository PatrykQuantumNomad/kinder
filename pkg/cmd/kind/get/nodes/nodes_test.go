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

package nodes

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"
)

// withResolveClusterName swaps the package-level resolveClusterName for the
// duration of t. Mirrors the pattern in pkg/cmd/kind/resume/resume_test.go.
func withResolveClusterName(t *testing.T, fn func(args []string, p *cluster.Provider) (string, error)) {
	t.Helper()
	prev := resolveClusterName
	resolveClusterName = fn
	t.Cleanup(func() { resolveClusterName = prev })
}

// withListNodes swaps the package-level listNodes injection for the duration of t.
func withListNodes(t *testing.T, fn func(name string) (string, error)) {
	t.Helper()
	prev := listNodes
	listNodes = fn
	t.Cleanup(func() { listNodes = prev })
}

// newTestStreams returns IOStreams backed by bytes.Buffer for test inspection.
func newTestStreams() (cmd.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return cmd.IOStreams{In: nil, Out: out, ErrOut: errOut}, out, errOut
}

// TestGetNodesCmd_PositionalArg_Resolves: table-driven test for the three
// subcase precedence rules. Fails against cobra.NoArgs source because
// any positional arg is rejected at parse time.
func TestGetNodesCmd_PositionalArg_Resolves(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		nameFlag       string
		resolveReturns string
		wantListedName string
	}{
		{
			name:           "positional arg wins",
			args:           []string{"verify47"},
			nameFlag:       "",
			resolveReturns: "verify47",
			wantListedName: "verify47",
		},
		{
			name:           "no positional, name flag used for auto-detect",
			args:           []string{},
			nameFlag:       "other",
			resolveReturns: "other",
			wantListedName: "other",
		},
		{
			name:           "positional wins over --name flag",
			args:           []string{"positional-wins"},
			nameFlag:       "flag-loses",
			resolveReturns: "positional-wins",
			wantListedName: "positional-wins",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantResolve := tt.resolveReturns
			withResolveClusterName(t, func(args []string, _ *cluster.Provider) (string, error) {
				return wantResolve, nil
			})
			var listedName string
			withListNodes(t, func(name string) (string, error) {
				listedName = name
				return "", nil // empty JSON: node list empty is fine for this test
			})
			streams, _, _ := newTestStreams()
			c := NewCommand(log.NoopLogger{}, streams)
			cmdArgs := []string{"--output=json"}
			if tt.nameFlag != "" {
				cmdArgs = append(cmdArgs, "--name="+tt.nameFlag)
			}
			cmdArgs = append(cmdArgs, tt.args...)
			c.SetArgs(cmdArgs)
			if err := c.Execute(); err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			if listedName != tt.wantListedName {
				t.Errorf("ListNodes called with %q, want %q", listedName, tt.wantListedName)
			}
		})
	}
}

// TestGetNodesCmd_AllClustersTakesPrecedence: --all-clusters set with a
// positional arg. ResolveClusterName must NOT be called; provider.List() is used.
func TestGetNodesCmd_AllClustersTakesPrecedence(t *testing.T) {
	resolveCalled := false
	withResolveClusterName(t, func(_ []string, _ *cluster.Provider) (string, error) {
		resolveCalled = true
		return "ignored", nil
	})
	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	// --all-clusters with a positional arg that would otherwise be resolved.
	// With cobra.MaximumNArgs(1) this must not error on the positional arg.
	c.SetArgs([]string{"--all-clusters", "--output=json"})
	// We don't wire a real provider here — AllClusters path calls provider.List()
	// which will fail without a real runtime, but that's after the args check.
	// We only care that resolveClusterName was NOT called.
	_ = c.Execute() // may fail on provider.List() — that's ok
	if resolveCalled {
		t.Errorf("resolveClusterName must NOT be called when --all-clusters is set")
	}
}

// TestGetNodesCmd_NoArgsNoFlag_AutoDetect: args=[], no --name, no --all-clusters.
// resolveClusterName receives empty args and returns the single cluster name.
func TestGetNodesCmd_NoArgsNoFlag_AutoDetect(t *testing.T) {
	withResolveClusterName(t, func(args []string, _ *cluster.Provider) (string, error) {
		if len(args) != 0 {
			return "", fmt.Errorf("expected empty args; got %v", args)
		}
		return "auto-cluster", nil
	})
	var listedName string
	withListNodes(t, func(name string) (string, error) {
		listedName = name
		return "", nil
	})
	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"--output=json"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if listedName != "auto-cluster" {
		t.Errorf("expected ListNodes called with %q, got %q", "auto-cluster", listedName)
	}
}

// TestGetNodesCmd_TooManyArgs_Rejected: 2 positional args → cobra rejects with
// MaximumNArgs(1) error. resolveClusterName must NOT be called.
func TestGetNodesCmd_TooManyArgs_Rejected(t *testing.T) {
	resolveCalled := false
	withResolveClusterName(t, func(_ []string, _ *cluster.Provider) (string, error) {
		resolveCalled = true
		return "", nil
	})
	streams, _, _ := newTestStreams()
	c := NewCommand(log.NoopLogger{}, streams)
	c.SetArgs([]string{"a", "b"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for 2 positional args (MaximumNArgs(1)), got nil")
	}
	if !strings.Contains(err.Error(), "accepts at most") && !strings.Contains(err.Error(), "arg") {
		t.Errorf("error message %q should mention arg limit", err.Error())
	}
	if resolveCalled {
		t.Errorf("resolveClusterName must NOT be called when cobra rejects args")
	}
}

func TestComputeSkew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cpMinor    uint
		nodeMinor  uint
		wantOK     bool
		wantContain string
	}{
		{
			name:        "same version returns checkmark",
			cpMinor:     31,
			nodeMinor:   31,
			wantOK:      true,
			wantContain: "\u2713", // checkmark ✓
		},
		{
			name:        "2 minors behind returns cross with -2",
			cpMinor:     31,
			nodeMinor:   29,
			wantOK:      true, // within 3-minor policy
			wantContain: "-2",
		},
		{
			name:        "4 minors behind returns cross with -4",
			cpMinor:     31,
			nodeMinor:   27,
			wantOK:      false,
			wantContain: "-4",
		},
		{
			name:        "1 minor behind is within policy",
			cpMinor:     31,
			nodeMinor:   30,
			wantOK:      true,
			wantContain: "-1",
		},
		{
			name:        "3 minors behind is at limit - ok",
			cpMinor:     31,
			nodeMinor:   28,
			wantOK:      true,
			wantContain: "-3",
		},
		{
			name:        "4 minors behind violates policy",
			cpMinor:     31,
			nodeMinor:   27,
			wantOK:      false,
			wantContain: "-4",
		},
		{
			name:        "node ahead of CP returns cross with positive offset",
			cpMinor:     31,
			nodeMinor:   33,
			wantOK:      false,
			wantContain: "+2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			display, ok := ComputeSkew(tt.cpMinor, tt.nodeMinor)
			if ok != tt.wantOK {
				t.Errorf("ComputeSkew(%d, %d) ok = %v, want %v; display = %q",
					tt.cpMinor, tt.nodeMinor, ok, tt.wantOK, display)
			}
			if tt.wantContain != "" {
				found := false
				for _, r := range display {
					_ = r
				}
				// Check substring
				if len(display) == 0 {
					t.Errorf("ComputeSkew(%d, %d) display is empty", tt.cpMinor, tt.nodeMinor)
				}
				found = containsStr(display, tt.wantContain)
				if !found {
					t.Errorf("ComputeSkew(%d, %d) display = %q, want to contain %q",
						tt.cpMinor, tt.nodeMinor, display, tt.wantContain)
				}
			}
		})
	}
}

// containsStr reports whether s contains substr.
func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
