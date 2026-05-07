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
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/exec"
)

// ---------------------------------------------------------------------------
// Task 1 tests: dockerLogsFn / k8sEventsFn fn-var collectors
// ---------------------------------------------------------------------------

// TestDockerLogsFn_DefaultWiring verifies that the package-level dockerLogsFn
// var points to realDockerLogs by default.
func TestDockerLogsFn_DefaultWiring(t *testing.T) {
	got := reflect.ValueOf(dockerLogsFn).Pointer()
	want := reflect.ValueOf(realDockerLogs).Pointer()
	if got != want {
		t.Errorf("dockerLogsFn default pointer = %v; want realDockerLogs pointer = %v", got, want)
	}
}

// TestK8sEventsFn_DefaultWiring verifies that the package-level k8sEventsFn
// var points to realK8sEvents by default.
func TestK8sEventsFn_DefaultWiring(t *testing.T) {
	got := reflect.ValueOf(k8sEventsFn).Pointer()
	want := reflect.ValueOf(realK8sEvents).Pointer()
	if got != want {
		t.Errorf("k8sEventsFn default pointer = %v; want realK8sEvents pointer = %v", got, want)
	}
}

// TestRealDockerLogs_BuildsCorrectCmdLine verifies that realDockerLogs invokes
// execCommand with argv ["docker", "logs", "--since", "30m", "kind-control-plane"].
func TestRealDockerLogs_BuildsCorrectCmdLine(t *testing.T) {
	wantKey := "docker logs --since 30m kind-control-plane"
	called := false

	fakeExec := newFakeExecCmd(map[string]fakeExecResult{
		wantKey: {lines: "output line\n", err: nil},
	})
	origExecCmd := execCommand
	execCommand = func(name string, args ...string) exec.Cmd {
		key := name
		if len(args) > 0 {
			key = name + " " + strings.Join(args, " ")
		}
		if key == wantKey {
			called = true
		}
		return fakeExec(name, args...)
	}
	t.Cleanup(func() { execCommand = origExecCmd })

	_, err := realDockerLogs("docker", "kind-control-plane", "30m")
	if err != nil {
		t.Fatalf("realDockerLogs error: %v", err)
	}
	if !called {
		t.Errorf("realDockerLogs did not produce expected argv key %q", wantKey)
	}
}

// TestRealK8sEvents_DefaultFilter verifies that realK8sEvents with
// includeNormal=false appends "--field-selector type!=Normal" (locked decision #3).
func TestRealK8sEvents_DefaultFilter(t *testing.T) {
	wantKey := "docker exec kind-control-plane kubectl --kubeconfig=/etc/kubernetes/admin.conf get events --all-namespaces --sort-by=.lastTimestamp --field-selector type!=Normal"
	called := false

	fakeExec := newFakeExecCmd(map[string]fakeExecResult{
		wantKey: {lines: "LAST SEEN   TYPE      REASON         OBJECT   MESSAGE\n", err: nil},
	})
	origExecCmd := execCommand
	execCommand = func(name string, args ...string) exec.Cmd {
		key := name
		if len(args) > 0 {
			key = name + " " + strings.Join(args, " ")
		}
		if key == wantKey {
			called = true
		}
		return fakeExec(name, args...)
	}
	t.Cleanup(func() { execCommand = origExecCmd })

	_, err := realK8sEvents("docker", "kind-control-plane", "0s", false)
	if err != nil {
		t.Fatalf("realK8sEvents error: %v", err)
	}
	if !called {
		t.Errorf("realK8sEvents(includeNormal=false) did not use expected argv %q", wantKey)
	}
}

// TestRealK8sEvents_IncludeNormalOverride verifies that includeNormal=true
// omits the "--field-selector type!=Normal" pair (locked decision #3).
func TestRealK8sEvents_IncludeNormalOverride(t *testing.T) {
	wantKey := "docker exec kind-control-plane kubectl --kubeconfig=/etc/kubernetes/admin.conf get events --all-namespaces --sort-by=.lastTimestamp"
	wrongKey := wantKey + " --field-selector type!=Normal"
	gotKey := ""

	fakeExec := newFakeExecCmd(map[string]fakeExecResult{
		wantKey: {lines: "LAST SEEN   TYPE      REASON         OBJECT   MESSAGE\n", err: nil},
	})
	origExecCmd := execCommand
	execCommand = func(name string, args ...string) exec.Cmd {
		key := name
		if len(args) > 0 {
			key = name + " " + strings.Join(args, " ")
		}
		gotKey = key
		return fakeExec(name, args...)
	}
	t.Cleanup(func() { execCommand = origExecCmd })

	_, err := realK8sEvents("docker", "kind-control-plane", "0s", true)
	if err != nil {
		t.Fatalf("realK8sEvents error: %v", err)
	}
	if gotKey == wrongKey {
		t.Errorf("realK8sEvents(includeNormal=true) must NOT include --field-selector type!=Normal")
	}
	if gotKey != wantKey {
		t.Errorf("realK8sEvents(includeNormal=true) argv = %q; want %q", gotKey, wantKey)
	}
}

// TestRealK8sEvents_TimeWindowClientSideFilter verifies client-side filtering
// by LAST SEEN column per RESEARCH pitfall 4. No --since flag must be added to argv.
func TestRealK8sEvents_TimeWindowClientSideFilter(t *testing.T) {
	// Three event rows; header passes through always.
	eventOutput := strings.Join([]string{
		"LAST SEEN   TYPE      REASON         OBJECT             MESSAGE",
		"45m         Warning   FailedMount    pod/foo            some error",
		"5m          Warning   FailedPull     pod/bar            another error",
		"2s          Warning   BackOff        pod/baz            back off",
	}, "\n") + "\n"

	callKey := "docker exec kind-control-plane kubectl --kubeconfig=/etc/kubernetes/admin.conf get events --all-namespaces --sort-by=.lastTimestamp --field-selector type!=Normal"
	fakeExec := newFakeExecCmd(map[string]fakeExecResult{
		callKey: {lines: eventOutput, err: nil},
	})
	origExecCmd := execCommand
	execCommand = fakeExec
	t.Cleanup(func() { execCommand = origExecCmd })

	// With window=30m0s, the 45m row should be dropped; header + 5m + 2s kept.
	lines30m, err := realK8sEvents("docker", "kind-control-plane", "30m0s", false)
	if err != nil {
		t.Fatalf("realK8sEvents(30m0s) error: %v", err)
	}
	for _, l := range lines30m {
		if strings.Contains(l, "45m") && strings.Contains(l, "FailedMount") {
			t.Errorf("realK8sEvents(30m0s) should have filtered out the 45m row; got lines: %v", lines30m)
		}
	}
	found5m := false
	found2s := false
	for _, l := range lines30m {
		if strings.Contains(l, "FailedPull") {
			found5m = true
		}
		if strings.Contains(l, "BackOff") {
			found2s = true
		}
	}
	if !found5m || !found2s {
		t.Errorf("realK8sEvents(30m0s) should keep 5m and 2s rows; got: %v", lines30m)
	}

	// With window=0s, all rows pass through (no client-side filter).
	lines0, err := realK8sEvents("docker", "kind-control-plane", "0s", false)
	if err != nil {
		t.Fatalf("realK8sEvents(0s) error: %v", err)
	}
	found45m := false
	for _, l := range lines0 {
		if strings.Contains(l, "FailedMount") {
			found45m = true
		}
	}
	if !found45m {
		t.Errorf("realK8sEvents(0s) should keep all rows including 45m; got: %v", lines0)
	}
}

// ---------------------------------------------------------------------------
// Task 2 tests: RunDecode orchestrator
// ---------------------------------------------------------------------------

// TestRunDecode_HappyPath verifies RunDecode aggregates docker logs and k8s events
// from all nodes and returns a correct DecodeResult.
func TestRunDecode_HappyPath(t *testing.T) {
	// Fixture lines — include known Catalog matches to verify result.Matches.
	cpLines := []string{
		"too many open files in kind-control-plane",
		"no match line cp-1",
		"no match line cp-2",
	}
	workerLines := []string{
		"failed to pull image in kind-worker",
		"no match line worker-1",
	}
	eventLines := []string{
		"CrashLoopBackOff seen in k8s events",
		"no match events-1",
	}

	origDockerFn := dockerLogsFn
	origK8sFn := k8sEventsFn
	t.Cleanup(func() {
		dockerLogsFn = origDockerFn
		k8sEventsFn = origK8sFn
	})

	dockerLogsFn = func(binaryName, nodeName, since string) ([]string, error) {
		switch nodeName {
		case "kind-control-plane":
			return cpLines, nil
		case "kind-worker":
			return workerLines, nil
		}
		return nil, nil
	}
	k8sEventsFn = func(binaryName, cpNodeName, since string, includeNormal bool) ([]string, error) {
		return eventLines, nil
	}

	result, err := RunDecode(DecodeOptions{
		Cluster:             "kind",
		BinaryName:          "docker",
		CPNodeName:          "kind-control-plane",
		AllNodes:            []string{"kind-control-plane", "kind-worker"},
		Since:               30 * time.Minute,
		IncludeNormalEvents: false,
	})
	if err != nil {
		t.Fatalf("RunDecode returned unexpected error: %v", err)
	}
	if result.Cluster != "kind" {
		t.Errorf("result.Cluster = %q; want %q", result.Cluster, "kind")
	}

	// Verify sources are tagged correctly.
	sourcesSeen := map[string]bool{}
	for _, m := range result.Matches {
		sourcesSeen[m.Source] = true
	}
	for _, wantSrc := range []string{"docker-logs:kind-control-plane", "docker-logs:kind-worker", "k8s-events"} {
		if !sourcesSeen[wantSrc] {
			t.Errorf("expected match with source %q; got sources: %v", wantSrc, sourcesSeen)
		}
	}

	totalLines := len(cpLines) + len(workerLines) + len(eventLines)
	wantUnmatched := totalLines - len(result.Matches)
	if result.Unmatched != wantUnmatched {
		t.Errorf("result.Unmatched = %d; want %d (totalLines=%d, matches=%d)",
			result.Unmatched, wantUnmatched, totalLines, len(result.Matches))
	}
}

// TestRunDecode_PausedNodeSkipped verifies that an error from dockerLogsFn on one
// node does not abort RunDecode; other nodes still produce matches.
func TestRunDecode_PausedNodeSkipped(t *testing.T) {
	origDockerFn := dockerLogsFn
	origK8sFn := k8sEventsFn
	t.Cleanup(func() {
		dockerLogsFn = origDockerFn
		k8sEventsFn = origK8sFn
	})

	dockerLogsFn = func(binaryName, nodeName, since string) ([]string, error) {
		if nodeName == "kind-paused-node" {
			return nil, errors.New("container is stopped")
		}
		return []string{"too many open files on surviving node"}, nil
	}
	k8sEventsFn = func(binaryName, cpNodeName, since string, includeNormal bool) ([]string, error) {
		return []string{"no match event"}, nil
	}

	result, err := RunDecode(DecodeOptions{
		Cluster:    "kind",
		BinaryName: "docker",
		CPNodeName: "kind-control-plane",
		AllNodes:   []string{"kind-control-plane", "kind-paused-node"},
		Since:      30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("RunDecode unexpected error: %v", err)
	}
	// Should have matches from kind-control-plane's surviving lines.
	if len(result.Matches) == 0 {
		t.Error("RunDecode should have matches from surviving nodes but got none")
	}
	// Verify paused-node did NOT abort but also has no match from its source.
	for _, m := range result.Matches {
		if m.Source == "docker-logs:kind-paused-node" {
			t.Error("RunDecode should not have matches from the paused (errored) node")
		}
	}
}

// TestRunDecode_EventsFailureNonFatal verifies that an error from k8sEventsFn
// does not abort RunDecode; docker-logs matches are still returned.
func TestRunDecode_EventsFailureNonFatal(t *testing.T) {
	origDockerFn := dockerLogsFn
	origK8sFn := k8sEventsFn
	t.Cleanup(func() {
		dockerLogsFn = origDockerFn
		k8sEventsFn = origK8sFn
	})

	dockerLogsFn = func(binaryName, nodeName, since string) ([]string, error) {
		return []string{"too many open files from docker logs"}, nil
	}
	k8sEventsFn = func(binaryName, cpNodeName, since string, includeNormal bool) ([]string, error) {
		return nil, errors.New("cluster API not yet ready")
	}

	result, err := RunDecode(DecodeOptions{
		Cluster:    "kind",
		BinaryName: "docker",
		CPNodeName: "kind-control-plane",
		AllNodes:   []string{"kind-control-plane"},
		Since:      30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("RunDecode unexpected error: %v", err)
	}
	if len(result.Matches) == 0 {
		t.Error("RunDecode should have matches from docker-logs even when k8s events fail")
	}
	for _, m := range result.Matches {
		if m.Source == "k8s-events" {
			t.Error("RunDecode should not have k8s-events matches when k8sEventsFn fails")
		}
	}
}

// TestRunDecode_PassesThroughIncludeNormal verifies that opts.IncludeNormalEvents=true
// is threaded to the k8sEventsFn includeNormal parameter (locked decision #3).
func TestRunDecode_PassesThroughIncludeNormal(t *testing.T) {
	origDockerFn := dockerLogsFn
	origK8sFn := k8sEventsFn
	t.Cleanup(func() {
		dockerLogsFn = origDockerFn
		k8sEventsFn = origK8sFn
	})

	dockerLogsFn = func(binaryName, nodeName, since string) ([]string, error) {
		return []string{"no match"}, nil
	}

	var gotIncludeNormal bool
	k8sEventsFn = func(binaryName, cpNodeName, since string, includeNormal bool) ([]string, error) {
		gotIncludeNormal = includeNormal
		return []string{"no match event"}, nil
	}

	_, err := RunDecode(DecodeOptions{
		Cluster:             "kind",
		BinaryName:          "docker",
		CPNodeName:          "kind-control-plane",
		AllNodes:            []string{"kind-control-plane"},
		Since:               30 * time.Minute,
		IncludeNormalEvents: true,
	})
	if err != nil {
		t.Fatalf("RunDecode unexpected error: %v", err)
	}
	if !gotIncludeNormal {
		t.Error("RunDecode should pass IncludeNormalEvents=true to k8sEventsFn")
	}
}

// TestRunDecode_PassesThroughSinceAsString verifies that opts.Since=30*time.Minute
// reaches both dockerLogsFn and k8sEventsFn as "30m0s" (locked decision #2).
func TestRunDecode_PassesThroughSinceAsString(t *testing.T) {
	origDockerFn := dockerLogsFn
	origK8sFn := k8sEventsFn
	t.Cleanup(func() {
		dockerLogsFn = origDockerFn
		k8sEventsFn = origK8sFn
	})

	var gotDockerSince string
	var gotK8sSince string

	dockerLogsFn = func(binaryName, nodeName, since string) ([]string, error) {
		gotDockerSince = since
		return []string{"no match"}, nil
	}
	k8sEventsFn = func(binaryName, cpNodeName, since string, includeNormal bool) ([]string, error) {
		gotK8sSince = since
		return []string{"no match event"}, nil
	}

	_, err := RunDecode(DecodeOptions{
		Cluster:    "kind",
		BinaryName: "docker",
		CPNodeName: "kind-control-plane",
		AllNodes:   []string{"kind-control-plane"},
		Since:      30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("RunDecode unexpected error: %v", err)
	}
	wantSince := (30 * time.Minute).String() // "30m0s"
	if gotDockerSince != wantSince {
		t.Errorf("dockerLogsFn since = %q; want %q", gotDockerSince, wantSince)
	}
	if gotK8sSince != wantSince {
		t.Errorf("k8sEventsFn since = %q; want %q", gotK8sSince, wantSince)
	}
}

// TestRunDecode_EmptyAllNodesError verifies that RunDecode returns an error
// when both AllNodes and CPNodeName are empty (caller misuse).
func TestRunDecode_EmptyAllNodesError(t *testing.T) {
	_, err := RunDecode(DecodeOptions{
		Cluster:    "kind",
		BinaryName: "docker",
		// Both CPNodeName and AllNodes are zero values.
	})
	if err == nil {
		t.Error("RunDecode should return an error when AllNodes and CPNodeName are both empty")
	}
}
