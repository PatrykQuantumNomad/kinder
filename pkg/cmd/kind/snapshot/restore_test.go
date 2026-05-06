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
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"sigs.k8s.io/kind/pkg/cmd"
	snapshot "sigs.k8s.io/kind/pkg/internal/snapshot"
	"sigs.k8s.io/kind/pkg/log"
)

// withRestoreFn swaps the package-level restoreFn for the duration of t and
// restores it on cleanup. Tests use this to avoid spinning a real cluster.
func withRestoreFn(t *testing.T, fn func(opts snapshot.RestoreOptions) (*snapshot.RestoreResult, error)) {
	t.Helper()
	prev := restoreFn
	restoreFn = fn
	t.Cleanup(func() { restoreFn = prev })
}

// fakeRestoreResult returns a non-nil RestoreResult for use in tests.
func fakeRestoreResult(snapName string) *snapshot.RestoreResult {
	return &snapshot.RestoreResult{
		SnapName:  snapName,
		DurationS: 2.5,
	}
}

// newRestoreStreams returns IOStreams backed by bytes.Buffer for test inspection.
func newRestoreStreams() cmd.IOStreams {
	streams, _, _ := newTestStreams()
	return streams
}

// TestRestoreCommand_OneArg: 1 arg → snap-name set, ClusterName="" (auto-detect).
func TestRestoreCommand_OneArg(t *testing.T) {
	var captured snapshot.RestoreOptions
	withRestoreFn(t, func(opts snapshot.RestoreOptions) (*snapshot.RestoreResult, error) {
		captured = opts
		return fakeRestoreResult("my-snap"), nil
	})

	c := NewCommand(log.NoopLogger{}, newRestoreStreams())
	c.SetArgs([]string{"restore", "my-snap"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if captured.ClusterName != "" {
		t.Errorf("expected ClusterName empty (auto-detect), got %q", captured.ClusterName)
	}
	if captured.SnapName != "my-snap" {
		t.Errorf("expected SnapName=my-snap, got %q", captured.SnapName)
	}
}

// TestRestoreCommand_TwoArgs: 2 args → ClusterName=args[0], SnapName=args[1].
func TestRestoreCommand_TwoArgs(t *testing.T) {
	var captured snapshot.RestoreOptions
	withRestoreFn(t, func(opts snapshot.RestoreOptions) (*snapshot.RestoreResult, error) {
		captured = opts
		return fakeRestoreResult("my-snap"), nil
	})

	c := NewCommand(log.NoopLogger{}, newRestoreStreams())
	c.SetArgs([]string{"restore", "mycluster", "my-snap"})
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if captured.ClusterName != "mycluster" {
		t.Errorf("expected ClusterName=mycluster, got %q", captured.ClusterName)
	}
	if captured.SnapName != "my-snap" {
		t.Errorf("expected SnapName=my-snap, got %q", captured.SnapName)
	}
}

// TestRestoreCommand_RejectsNoArgs: 0 args → cobra.RangeArgs(1,2) rejects.
func TestRestoreCommand_RejectsNoArgs(t *testing.T) {
	withRestoreFn(t, func(opts snapshot.RestoreOptions) (*snapshot.RestoreResult, error) {
		return fakeRestoreResult("x"), nil
	})

	c := NewCommand(log.NoopLogger{}, newRestoreStreams())
	c.SetArgs([]string{"restore"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for 0 args (RangeArgs), got nil")
	}
}

// TestRestoreCommand_PropagatesErrCompatK8sMismatch: injected restoreFn returns
// ErrCompatK8sMismatch → command returns same error.
func TestRestoreCommand_PropagatesErrCompatK8sMismatch(t *testing.T) {
	withRestoreFn(t, func(opts snapshot.RestoreOptions) (*snapshot.RestoreResult, error) {
		return nil, snapshot.ErrCompatK8sMismatch
	})

	c := NewCommand(log.NoopLogger{}, newRestoreStreams())
	c.SetArgs([]string{"restore", "mycluster", "my-snap"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "k8s") && !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("expected compat mismatch in error, got: %v", err)
	}
}

// TestRestoreCommand_NoYesFlag: restore must NOT expose --yes flag.
// This is a CONTEXT.md locked decision: hard overwrite, no confirmation gate.
func TestRestoreCommand_NoYesFlag(t *testing.T) {
	// c is the kinder snapshot group command; its direct children include "restore".
	c := NewCommand(log.NoopLogger{}, newRestoreStreams())
	var restoreCmd *cobra.Command
	for _, sub := range c.Commands() {
		if sub.Name() == "restore" {
			restoreCmd = sub
			break
		}
	}
	if restoreCmd == nil {
		t.Fatal("could not find 'restore' subcommand under snapshot group")
	}
	if f := restoreCmd.Flags().Lookup("yes"); f != nil {
		t.Errorf("restore must NOT have a --yes flag (CONTEXT.md locked: hard overwrite by design)")
	}
}
