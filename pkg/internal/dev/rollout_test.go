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

package dev

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// hasArgEq returns true if the args slice contains an element exactly equal
// to want. Mode "exact": no substring match; --kubeconfig=/tmp/x must match
// itself, not "--kubeconfig=/" plus suffix.
func hasArgEq(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// findCallStartingWith returns the first recorded call whose args start with
// the given prefix sequence (e.g. ["rollout", "restart"]). Returns nil if
// none found.
func findCallStartingWith(calls []recordedExecCall, prefix ...string) *recordedExecCall {
	for i := range calls {
		c := &calls[i]
		if len(c.args) < len(prefix) {
			continue
		}
		// Find the prefix sequence anywhere in args (kubectl flags may
		// precede). For our argv layout the verb pair is "rollout
		// restart" / "rollout status" — search for the contiguous pair.
		for j := 0; j+len(prefix) <= len(c.args); j++ {
			match := true
			for k, p := range prefix {
				if c.args[j+k] != p {
					match = false
					break
				}
			}
			if match {
				return c
			}
		}
	}
	return nil
}

func TestRolloutRestartAndWait_HappyPath(t *testing.T) {
	rec := &fakeExecCmder{lookup: func(_ string, _ []string) (string, error) {
		return "", nil
	}}
	withDevCmder(t, rec)

	if err := RolloutRestartAndWait(context.Background(),
		"/tmp/kinder-dev-abc.kubeconfig", "production", "myapp", 2*time.Minute,
	); err != nil {
		t.Fatalf("RolloutRestartAndWait returned error: %v", err)
	}

	calls := rec.snapshot()
	if len(calls) != 2 {
		t.Fatalf("expected exactly 2 kubectl calls (restart + status); got %d: %#v", len(calls), calls)
	}
	for i, c := range calls {
		if c.name != "kubectl" {
			t.Errorf("call %d: name = %q, want kubectl", i, c.name)
		}
		if !hasArgEq(c.args, "--kubeconfig=/tmp/kinder-dev-abc.kubeconfig") {
			t.Errorf("call %d args missing --kubeconfig=...; got %v", i, c.args)
		}
		if !hasArgEq(c.args, "--namespace=production") {
			t.Errorf("call %d args missing --namespace=production; got %v", i, c.args)
		}
		if !hasArgEq(c.args, "deployment/myapp") {
			t.Errorf("call %d args missing deployment/myapp; got %v", i, c.args)
		}
	}

	// First call: rollout restart.
	restart := findCallStartingWith(calls[:1], "rollout", "restart")
	if restart == nil {
		t.Errorf("expected first call to contain 'rollout restart'; got args %v", calls[0].args)
	}
	// Second call: rollout status with --timeout=2m0s (Go's
	// time.Duration.String() form for 2*time.Minute).
	status := findCallStartingWith(calls[1:], "rollout", "status")
	if status == nil {
		t.Errorf("expected second call to contain 'rollout status'; got args %v", calls[1].args)
	}
	if status != nil && !hasArgEq(status.args, "--timeout=2m0s") {
		t.Errorf("expected --timeout=2m0s in status call; got %v", status.args)
	}
}

func TestRolloutRestartAndWait_RestartFails(t *testing.T) {
	callCount := 0
	rec := &fakeExecCmder{lookup: func(_ string, _ []string) (string, error) {
		callCount++
		// Fail the first call (restart). Second (status) would also
		// reach this lookup if we let execution continue — assert below
		// that it does NOT.
		return "", errors.New("kubectl exec failed")
	}}
	withDevCmder(t, rec)

	err := RolloutRestartAndWait(context.Background(),
		"/tmp/kc", "default", "app", time.Minute,
	)
	if err == nil {
		t.Fatal("expected error from restart failure")
	}
	if !strings.Contains(err.Error(), "rollout restart") {
		t.Errorf("error %q should mention 'rollout restart'", err.Error())
	}
	if !strings.Contains(err.Error(), "deployment/app") && !strings.Contains(err.Error(), "app") {
		t.Errorf("error %q should mention the deployment name", err.Error())
	}
	if !strings.Contains(err.Error(), "default") {
		t.Errorf("error %q should mention the namespace", err.Error())
	}
	if got := len(rec.snapshot()); got != 1 {
		t.Errorf("expected exactly 1 kubectl call (status must NOT run after restart fails); got %d", got)
	}
}

func TestRolloutRestartAndWait_StatusFails(t *testing.T) {
	rec := &fakeExecCmder{lookup: func(_ string, args []string) (string, error) {
		// Only fail when this is the status call (contains "status").
		for _, a := range args {
			if a == "status" {
				return "", errors.New("status timed out")
			}
		}
		return "", nil
	}}
	withDevCmder(t, rec)

	err := RolloutRestartAndWait(context.Background(),
		"/tmp/kc", "ns1", "myapp", 90*time.Second,
	)
	if err == nil {
		t.Fatal("expected error from status failure")
	}
	if !strings.Contains(err.Error(), "rollout status") {
		t.Errorf("error %q should mention 'rollout status'", err.Error())
	}
	// Go's time.Duration.String() for 90s = "1m30s".
	if !strings.Contains(err.Error(), "1m30s") {
		t.Errorf("error %q should mention the timeout (1m30s); got: %s", err.Error(), err.Error())
	}
}

func TestRolloutRestartAndWait_RejectsEmptyKubeconfigPath(t *testing.T) {
	rec := &fakeExecCmder{}
	withDevCmder(t, rec)
	err := RolloutRestartAndWait(context.Background(), "", "ns", "app", time.Minute)
	if err == nil {
		t.Fatal("expected error for empty kubeconfigPath")
	}
	if !strings.Contains(err.Error(), "kubeconfigPath") {
		t.Errorf("error %q should mention kubeconfigPath", err.Error())
	}
	if got := len(rec.snapshot()); got != 0 {
		t.Errorf("no kubectl call should be made on validation failure; got %d", got)
	}
}

func TestRolloutRestartAndWait_RejectsEmptyNamespace(t *testing.T) {
	rec := &fakeExecCmder{}
	withDevCmder(t, rec)
	err := RolloutRestartAndWait(context.Background(), "/tmp/kc", "", "app", time.Minute)
	if err == nil {
		t.Fatal("expected error for empty namespace")
	}
	if !strings.Contains(err.Error(), "namespace") {
		t.Errorf("error %q should mention namespace", err.Error())
	}
	if got := len(rec.snapshot()); got != 0 {
		t.Errorf("no kubectl call should be made on validation failure; got %d", got)
	}
}

func TestRolloutRestartAndWait_RejectsEmptyDeployment(t *testing.T) {
	rec := &fakeExecCmder{}
	withDevCmder(t, rec)
	err := RolloutRestartAndWait(context.Background(), "/tmp/kc", "ns", "", time.Minute)
	if err == nil {
		t.Fatal("expected error for empty deployment")
	}
	if !strings.Contains(err.Error(), "deployment") {
		t.Errorf("error %q should mention deployment", err.Error())
	}
	if got := len(rec.snapshot()); got != 0 {
		t.Errorf("no kubectl call should be made on validation failure; got %d", got)
	}
}

func TestRolloutRestartAndWait_RejectsZeroTimeout(t *testing.T) {
	rec := &fakeExecCmder{}
	withDevCmder(t, rec)
	err := RolloutRestartAndWait(context.Background(), "/tmp/kc", "ns", "app", 0)
	if err == nil {
		t.Fatal("expected error for zero timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error %q should mention timeout", err.Error())
	}
}

func TestRolloutRestartAndWait_TimeoutPropagatesAsString(t *testing.T) {
	rec := &fakeExecCmder{lookup: func(_ string, _ []string) (string, error) {
		return "", nil
	}}
	withDevCmder(t, rec)

	if err := RolloutRestartAndWait(context.Background(),
		"/tmp/kc", "ns", "app", 90*time.Second,
	); err != nil {
		t.Fatalf("RolloutRestartAndWait returned error: %v", err)
	}
	calls := rec.snapshot()
	status := findCallStartingWith(calls, "rollout", "status")
	if status == nil {
		t.Fatalf("expected a 'rollout status' call; got %v", calls)
	}
	// 90s in Go's Duration.String() form = "1m30s".
	if !hasArgEq(status.args, "--timeout=1m30s") {
		t.Errorf("expected --timeout=1m30s in status args; got %v", status.args)
	}
}

func TestRolloutFn_WiredToRolloutRestartAndWait(t *testing.T) {
	if RolloutFn == nil {
		t.Fatal("RolloutFn must not be nil at package init")
	}
	rec := &fakeExecCmder{lookup: func(_ string, _ []string) (string, error) {
		return "", nil
	}}
	withDevCmder(t, rec)
	if err := RolloutFn(context.Background(), "/tmp/kc", "ns", "app", time.Second); err != nil {
		t.Fatalf("RolloutFn returned error: %v", err)
	}
	if got := len(rec.snapshot()); got != 2 {
		t.Errorf("RolloutFn should route through devCmder for both kubectl calls; got %d", got)
	}
}
