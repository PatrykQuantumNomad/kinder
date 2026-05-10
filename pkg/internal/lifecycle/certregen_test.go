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

// NOTE: Tests in this file that swap package-level globals (certRegenSleeper,
// kindippin.IppinCmder, or defaultCmder) MUST NOT use t.Parallel() because
// they mutate shared package state. Pure data tests are exempt.
package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	kindippin "sigs.k8s.io/kind/pkg/internal/ippin"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/log"
)

// ---- captureLogger ----------------------------------------------------------

// captureLogger records log lines for assertion.
type captureLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *captureLogger) Warn(msg string)                  { l.mu.Lock(); l.lines = append(l.lines, msg); l.mu.Unlock() }
func (l *captureLogger) Warnf(f string, a ...interface{}) { l.mu.Lock(); l.lines = append(l.lines, fmt.Sprintf(f, a...)); l.mu.Unlock() }
func (l *captureLogger) Error(string)                     {}
func (l *captureLogger) Errorf(string, ...interface{})    {}
func (l *captureLogger) V(_ log.Level) log.InfoLogger     { return &captureInfoLogger{parent: l} }

type captureInfoLogger struct{ parent *captureLogger }

func (il *captureInfoLogger) Info(msg string) {
	il.parent.mu.Lock()
	il.parent.lines = append(il.parent.lines, msg)
	il.parent.mu.Unlock()
}
func (il *captureInfoLogger) Infof(f string, a ...interface{}) {
	il.parent.mu.Lock()
	il.parent.lines = append(il.parent.lines, fmt.Sprintf(f, a...))
	il.parent.mu.Unlock()
}
func (il *captureInfoLogger) Enabled() bool { return true }

var _ log.Logger = (*captureLogger)(nil)

// ---- swapCertRegenSleeper --------------------------------------------------

// swapCertRegenSleeper replaces the package-level certRegenSleeper for the
// duration of the test. MUST NOT be used with t.Parallel().
func swapCertRegenSleeper(t *testing.T, fn func(time.Duration)) {
	t.Helper()
	prev := certRegenSleeper
	certRegenSleeper = fn
	t.Cleanup(func() { certRegenSleeper = prev })
}

// noopSleeper returns a sleep function that does nothing (avoids 45s+ test waits).
func noopSleeper() func(time.Duration) {
	return func(time.Duration) {}
}

// recordingSleeper records sleep durations.
type recordingSleeper struct {
	mu     sync.Mutex
	sleeps []time.Duration
}

func (r *recordingSleeper) sleep(d time.Duration) {
	r.mu.Lock()
	r.sleeps = append(r.sleeps, d)
	r.mu.Unlock()
}

// ---- IPAM state helpers ----------------------------------------------------

// writeFakeIPAMState writes a fake ipam-state.json for the given container
// into tmpDir so ReadIPAMState (called by IPDriftDetected) can find it.
func writeFakeIPAMState(t *testing.T, tmpDir, container, network, ipv4 string) {
	t.Helper()
	hostPath := filepath.Join(tmpDir, container+"-ipam.json")
	state := kindippin.IPAMState{Network: network, IPv4: ipv4}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("writeFakeIPAMState: marshal: %v", err)
	}
	if err := os.WriteFile(hostPath, data, 0o600); err != nil {
		t.Fatalf("writeFakeIPAMState: write: %v", err)
	}
}

// ---- helper to join calls into a readable string slice ---------------------

func joinCalls(calls []recordedCall) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = strings.Join(append([]string{c.name}, c.args...), " ")
	}
	return out
}

// ============================================================================
// IPDriftDetected tests
// ============================================================================

// TestIPDriftDetected_NoDrift: recorded IP and current IP are the same.
// Expect drifted=false, no error.
func TestIPDriftDetected_NoDrift(t *testing.T) {
	// IPDriftDetected uses certRegenIppinCmder (a package-level var in
	// certregen.go) for docker-cp + inspect. We swap kindippin.IppinCmder
	// since certRegenIppinCmder delegates to it (via ReadIPAMState facade).
	// ReadIPAMState uses kindippin.IppinCmder for the cp call and then reads
	// the copied file from tmpDir. For the inspect call, IPDriftDetected uses
	// defaultCmder. Pre-write the state file so the cp call succeeds.
	tmpDir := t.TempDir()
	writeFakeIPAMState(t, tmpDir, "cp1", "kind", "172.18.0.5")

	// IppinCmder: call 0 = docker cp → succeeds (file pre-written in tmpDir).
	fc := &ippinCmderFake{
		responses: []ippinCmdResp{
			{}, // docker cp succeeds; file already in tmpDir
		},
	}
	swapIPPinCmder(t, fc)

	// defaultCmder: inspect returns same IP.
	withCmder(t, fakeCmderByName(map[string]*fakeCmd{
		"cp1": {stdout: "172.18.0.5\n"},
	}))

	drifted, currentIP, recordedIP, err := IPDriftDetected("docker", "cp1", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if drifted {
		t.Errorf("expected drifted=false, got true (currentIP=%q recordedIP=%q)", currentIP, recordedIP)
	}
	if currentIP != "172.18.0.5" {
		t.Errorf("currentIP=%q, want 172.18.0.5", currentIP)
	}
	if recordedIP != "172.18.0.5" {
		t.Errorf("recordedIP=%q, want 172.18.0.5", recordedIP)
	}
}

// TestIPDriftDetected_Drift: recorded IP differs from current IP.
// Expect drifted=true, currentIP="172.18.0.7", recordedIP="172.18.0.5".
func TestIPDriftDetected_Drift(t *testing.T) {
	tmpDir := t.TempDir()
	writeFakeIPAMState(t, tmpDir, "cp1", "kind", "172.18.0.5")

	fc := &ippinCmderFake{
		responses: []ippinCmdResp{
			{}, // docker cp succeeds
		},
	}
	swapIPPinCmder(t, fc)

	withCmder(t, fakeCmderByName(map[string]*fakeCmd{
		"cp1": {stdout: "172.18.0.7\n"}, // different IP
	}))

	drifted, currentIP, recordedIP, err := IPDriftDetected("docker", "cp1", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !drifted {
		t.Errorf("expected drifted=true")
	}
	if currentIP != "172.18.0.7" {
		t.Errorf("currentIP=%q, want 172.18.0.7", currentIP)
	}
	if recordedIP != "172.18.0.5" {
		t.Errorf("recordedIP=%q, want 172.18.0.5", recordedIP)
	}
}

// TestIPDriftDetected_LegacyNoFile: docker cp returns "No such file" error.
// Expect drifted=true, recordedIP="", no error.
func TestIPDriftDetected_LegacyNoFile(t *testing.T) {
	tmpDir := t.TempDir()

	fc := &ippinCmderFake{
		responses: []ippinCmdResp{
			{err: fmt.Errorf("No such file or directory")}, // docker cp fails
		},
	}
	swapIPPinCmder(t, fc)

	// inspect may or may not be called (legacy branch may skip it)
	withCmder(t, fakeCmderByName(map[string]*fakeCmd{
		"cp1": {stdout: "172.18.0.5\n"},
	}))

	drifted, _, recordedIP, err := IPDriftDetected("docker", "cp1", tmpDir)
	if err != nil {
		t.Fatalf("legacy no-file must not return error, got: %v", err)
	}
	if !drifted {
		t.Errorf("expected drifted=true for legacy cluster (no ipam-state.json)")
	}
	if recordedIP != "" {
		t.Errorf("recordedIP should be empty for legacy cluster, got %q", recordedIP)
	}
}

// ============================================================================
// RegenerateEtcdPeerCertsWholesale tests
// ============================================================================

// All fakeNode.Command() calls route through defaultCmder (see state_test.go),
// so withCmder captures all node.Command() calls in these tests.

// makeHACPNodes creates N fakeNode instances with control-plane role.
func makeHACPNodes(names ...string) []nodes.Node {
	out := make([]nodes.Node, len(names))
	for i, n := range names {
		out[i] = &fakeNode{name: n, role: "control-plane"}
	}
	return out
}

// TestRegenerateEtcdPeerCertsWholesale_HappyPath: 3 CP nodes.
// Asserts 3 commands per CP (kubeadm renew, mv out, mv back) × 3 = 9 total.
// Serial ordering: full cycle on cp1 before cp2, etc.
func TestRegenerateEtcdPeerCertsWholesale_HappyPath(t *testing.T) {
	swapCertRegenSleeper(t, noopSleeper())

	rec := &recordingCmder{
		lookup: func(_ string, _ []string) (string, error) { return "", nil },
	}
	withCmder(t, rec.cmder())

	cpNodes := makeHACPNodes("cp1", "cp2", "cp3")
	clog := &captureLogger{}

	err := RegenerateEtcdPeerCertsWholesale(cpNodes, clog)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := rec.snapshot()
	// 3 commands per CP × 3 nodes = 9
	if len(calls) != 9 {
		t.Fatalf("expected 9 calls (3 per CP × 3 nodes), got %d; calls=%v", len(calls), joinCalls(calls))
	}

	for i, nodeName := range []string{"cp1", "cp2", "cp3"} {
		base := i * 3

		// Step 1: kubeadm certs renew etcd-peer
		c0 := strings.Join(append([]string{calls[base].name}, calls[base].args...), " ")
		if !strings.Contains(c0, "kubeadm") || !strings.Contains(c0, "certs") || !strings.Contains(c0, "etcd-peer") {
			t.Errorf("node %s step1: want 'kubeadm certs renew etcd-peer', got %q", nodeName, c0)
		}

		// Step 2: mv etcd manifest out
		c1 := strings.Join(append([]string{calls[base+1].name}, calls[base+1].args...), " ")
		if !strings.Contains(c1, "mv") || !strings.Contains(c1, etcdManifestPath) {
			t.Errorf("node %s step2: want 'mv %s ...', got %q", nodeName, etcdManifestPath, c1)
		}
		if !strings.Contains(c1, etcdManifestBackup) {
			t.Errorf("node %s step2: backup path %q missing in %q", nodeName, etcdManifestBackup, c1)
		}

		// Step 3: mv manifest back
		c2 := strings.Join(append([]string{calls[base+2].name}, calls[base+2].args...), " ")
		if !strings.Contains(c2, "mv") || !strings.Contains(c2, etcdManifestBackup) {
			t.Errorf("node %s step3: want 'mv %s ...', got %q", nodeName, etcdManifestBackup, c2)
		}
		if !strings.Contains(c2, etcdManifestPath) {
			t.Errorf("node %s step3: manifest path %q missing in %q", nodeName, etcdManifestPath, c2)
		}
	}
}

// TestRegenerateEtcdPeerCertsWholesale_RenewFailureHalts: cp2 kubeadm renew fails.
// Returns wrapped error containing "etcd peer cert regen failed on cp2".
// cp3 must NOT be touched.
func TestRegenerateEtcdPeerCertsWholesale_RenewFailureHalts(t *testing.T) {
	swapCertRegenSleeper(t, noopSleeper())

	callIdx := 0
	rec := &recordingCmder{
		lookup: func(_ string, args []string) (string, error) {
			idx := callIdx
			callIdx++
			// Call 3 (0-indexed) is cp2's first command (kubeadm certs renew)
			if idx == 3 && len(args) > 0 && args[0] == "certs" {
				return "", fmt.Errorf("kubeadm: cert renew failed on cp2")
			}
			return "", nil
		},
	}
	withCmder(t, rec.cmder())

	cpNodes := makeHACPNodes("cp1", "cp2", "cp3")
	err := RegenerateEtcdPeerCertsWholesale(cpNodes, log.NoopLogger{})
	if err == nil {
		t.Fatal("expected error for cp2 renew failure, got nil")
	}
	if !strings.Contains(err.Error(), "etcd peer cert regen failed") {
		t.Errorf("error should contain 'etcd peer cert regen failed': %v", err)
	}
	if !strings.Contains(err.Error(), "cp2") {
		t.Errorf("error should mention cp2: %v", err)
	}

	// cp3 must NOT be touched. After cp1 (3 calls) + cp2 kubeadm fails (1 call) = 4 total.
	calls := rec.snapshot()
	if len(calls) > 4 {
		t.Errorf("expected ≤4 calls after cp2 failure, got %d: %v", len(calls), joinCalls(calls))
	}
}

// TestRegenerateEtcdPeerCertsWholesale_StaticPodCycleFailureHalts: mv out fails on cp1.
// Returns wrapped error. cp2/cp3 must NOT be touched.
func TestRegenerateEtcdPeerCertsWholesale_StaticPodCycleFailureHalts(t *testing.T) {
	swapCertRegenSleeper(t, noopSleeper())

	callIdx := 0
	rec := &recordingCmder{
		lookup: func(_ string, args []string) (string, error) {
			idx := callIdx
			callIdx++
			// Call 1 (0-indexed) is cp1's mv out
			if idx == 1 && len(args) > 0 && args[0] == etcdManifestPath {
				return "", fmt.Errorf("mv: permission denied")
			}
			return "", nil
		},
	}
	withCmder(t, rec.cmder())

	cpNodes := makeHACPNodes("cp1", "cp2", "cp3")
	err := RegenerateEtcdPeerCertsWholesale(cpNodes, log.NoopLogger{})
	if err == nil {
		t.Fatal("expected error for mv failure on cp1, got nil")
	}
	if !strings.Contains(err.Error(), "etcd peer cert regen failed") {
		t.Errorf("error should contain 'etcd peer cert regen failed': %v", err)
	}
	if !strings.Contains(err.Error(), "cp1") {
		t.Errorf("error should mention cp1: %v", err)
	}

	// cp2/cp3 untouched: at most 2 calls (kubeadm renew + failed mv)
	calls := rec.snapshot()
	if len(calls) > 2 {
		t.Errorf("expected ≤2 calls after cp1 mv failure, got %d: %v", len(calls), joinCalls(calls))
	}
}

// TestRegenerateEtcdPeerCertsWholesale_SingleCPSkip: len(cpNodes)==1 →
// returns nil immediately, zero commands (defense in depth).
func TestRegenerateEtcdPeerCertsWholesale_SingleCPSkip(t *testing.T) {
	swapCertRegenSleeper(t, noopSleeper())

	rec := &recordingCmder{
		lookup: func(_ string, _ []string) (string, error) { return "", nil },
	}
	withCmder(t, rec.cmder())

	cpNodes := makeHACPNodes("cp1")
	err := RegenerateEtcdPeerCertsWholesale(cpNodes, log.NoopLogger{})
	if err != nil {
		t.Fatalf("expected nil for single CP, got %v", err)
	}
	calls := rec.snapshot()
	if len(calls) != 0 {
		t.Errorf("expected 0 commands for single CP, got %d: %v", len(calls), joinCalls(calls))
	}
}

// TestRegenerateEtcdPeerCertsWholesale_LoggerProgress: a V(0).Infof per-CP
// line containing "Regenerating etcd peer cert on <node> (N/M)".
func TestRegenerateEtcdPeerCertsWholesale_LoggerProgress(t *testing.T) {
	swapCertRegenSleeper(t, noopSleeper())

	rec := &recordingCmder{
		lookup: func(_ string, _ []string) (string, error) { return "", nil },
	}
	withCmder(t, rec.cmder())

	cpNodes := makeHACPNodes("cp1", "cp2", "cp3")
	clog := &captureLogger{}

	if err := RegenerateEtcdPeerCertsWholesale(cpNodes, clog); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clog.mu.Lock()
	lines := clog.lines
	clog.mu.Unlock()

	if len(lines) < 3 {
		t.Errorf("expected ≥3 progress log lines, got %d: %v", len(lines), lines)
	}
	for _, nodeName := range []string{"cp1", "cp2", "cp3"} {
		found := false
		for _, l := range lines {
			if strings.Contains(l, nodeName) && strings.Contains(l, "Regenerating etcd peer cert") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected progress line for %q; got lines: %v", nodeName, lines)
		}
	}
}

// TestRegenerateEtcdPeerCertsWholesale_SleepIsInjectable: certRegenSleeper
// is package-level and injectable. Asserts: (a) function does NOT block 45s,
// (b) correct sleep durations are passed (25s + 20s per CP).
func TestRegenerateEtcdPeerCertsWholesale_SleepIsInjectable(t *testing.T) {
	rs := &recordingSleeper{}
	swapCertRegenSleeper(t, rs.sleep)

	rec := &recordingCmder{
		lookup: func(_ string, _ []string) (string, error) { return "", nil },
	}
	withCmder(t, rec.cmder())

	cpNodes := makeHACPNodes("cp1", "cp2")

	start := time.Now()
	if err := RegenerateEtcdPeerCertsWholesale(cpNodes, log.NoopLogger{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Real sleeps would take 45s × 2 = 90s; with injection it should be fast.
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("function took %v — sleeper injection is not working", elapsed)
	}

	rs.mu.Lock()
	sleeps := rs.sleeps
	rs.mu.Unlock()

	// 2 CPs × 2 sleeps = 4 sleep calls
	if len(sleeps) != 4 {
		t.Errorf("expected 4 sleep calls (2 per CP × 2), got %d: %v", len(sleeps), sleeps)
	}
	expectedFirst := kubeletFileCheckFrequency + staticPodCycleSafetyMargin
	if sleeps[0] != expectedFirst {
		t.Errorf("sleep[0] = %v, want %v (kubelet+margin)", sleeps[0], expectedFirst)
	}
	if sleeps[1] != staticPodRecreationWait {
		t.Errorf("sleep[1] = %v, want %v (recreation wait)", sleeps[1], staticPodRecreationWait)
	}
	if sleeps[2] != expectedFirst {
		t.Errorf("sleep[2] = %v, want %v", sleeps[2], expectedFirst)
	}
	if sleeps[3] != staticPodRecreationWait {
		t.Errorf("sleep[3] = %v, want %v", sleeps[3], staticPodRecreationWait)
	}
}
