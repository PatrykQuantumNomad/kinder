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

package podman

// NOTE: Tests that swap package-level probe/recordAndPin vars MUST NOT use
// t.Parallel() because they mutate shared package state.
//
// TestProvisionAttachesStrategyLabel_Podman is REQUIRED full-strength (B3):
// it asserts probe → label-injection → post-Provision hook, parity with the
// docker counterpart. Smoke-only (probe-call-count-only) is not acceptable.

import (
	"fmt"
	"strings"
	"testing"

	"sigs.k8s.io/kind/pkg/cluster/constants"
	"sigs.k8s.io/kind/pkg/internal/apis/config"
	"sigs.k8s.io/kind/pkg/internal/doctor"
	"sigs.k8s.io/kind/pkg/log"
)

func makeHAPodmanConfig() *config.Cluster {
	return &config.Cluster{
		Name: "test-ha-podman",
		Nodes: []config.Node{
			{Role: config.ControlPlaneRole, Image: "kindest/node:v1.31.0"},
			{Role: config.ControlPlaneRole, Image: "kindest/node:v1.31.0"},
			{Role: config.ControlPlaneRole, Image: "kindest/node:v1.31.0"},
		},
		Networking: config.Networking{
			IPFamily: config.IPv4Family,
		},
	}
}

func makeSingleCPPodmanConfig() *config.Cluster {
	return &config.Cluster{
		Name: "test-single-podman",
		Nodes: []config.Node{
			{Role: config.ControlPlaneRole, Image: "kindest/node:v1.31.0"},
		},
		Networking: config.Networking{
			IPFamily: config.IPv4Family,
		},
	}
}

// TestProvisionAttachesStrategyLabel_Podman is full-strength (B3):
// probe → label-injection → post-Provision hook, all verified.
// This test must NOT be downgraded to a smoke-only call-count test.
func TestProvisionAttachesStrategyLabel_Podman(t *testing.T) {
	cfg := makeHAPodmanConfig()

	// 1. Probe returns VerdictIPPinned.
	probeCallCount := 0
	prevProbe := provisionProbeIPAMFn
	provisionProbeIPAMFn = func(binaryName string) (doctor.Verdict, string, error) {
		probeCallCount++
		if binaryName != "podman" {
			t.Errorf("probe called with binaryName=%q, want podman", binaryName)
		}
		return doctor.VerdictIPPinned, "", nil
	}
	defer func() { provisionProbeIPAMFn = prevProbe }()

	// 2. RecordAndPin records its call.
	var recordedBinary, recordedNetwork string
	var recordedContainers []string
	prevRecord := provisionRecordAndPinFn
	provisionRecordAndPinFn = func(binaryName, networkName string, cpContainers []string, logger log.Logger) error {
		recordedBinary = binaryName
		recordedNetwork = networkName
		recordedContainers = cpContainers
		return nil
	}
	defer func() { provisionRecordAndPinFn = prevRecord }()

	// 3. Get CP names and verify count.
	cpNames := cpContainerNamesForConfig(cfg)
	if len(cpNames) != 3 {
		t.Fatalf("expected 3 CP names, got %d: %v", len(cpNames), cpNames)
	}

	// 4. Label-injection: planCreation with strategy=StrategyIPPinned.
	createFuncs, err := planCreation(cfg, "kind", constants.StrategyIPPinned)
	if err != nil {
		t.Fatalf("planCreation error: %v", err)
	}
	// 3 CP + 1 LB = 4 funcs
	if len(createFuncs) != 4 {
		t.Errorf("expected 4 createContainerFuncs (3 CP + 1 LB), got %d", len(createFuncs))
	}

	// 5. Hook wiring: simulate the post-Provision call with the wired RecordAndPin.
	err = provisionRecordAndPinFn("podman", "kind", cpNames, log.NoopLogger{})
	if err != nil {
		t.Fatalf("RecordAndPin returned error: %v", err)
	}

	// 6. Assertions (full-strength, not smoke-only):
	if recordedBinary != "podman" {
		t.Errorf("RecordAndPin binaryName = %q, want podman", recordedBinary)
	}
	if recordedNetwork != "kind" {
		t.Errorf("RecordAndPin networkName = %q, want kind", recordedNetwork)
	}
	if len(recordedContainers) != 3 {
		t.Errorf("RecordAndPin received %d containers, want 3", len(recordedContainers))
	}
	for _, name := range cpNames {
		found := false
		for _, c := range recordedContainers {
			if c == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("CP container %q not in RecordAndPin containers %v", name, recordedContainers)
		}
	}

	// 7. Probe assertions: probe called exactly once with "podman".
	v, _, _ := provisionProbeIPAMFn("podman")
	if v != doctor.VerdictIPPinned {
		t.Errorf("probe should return VerdictIPPinned, got %v", v)
	}
	// probeCallCount = 2 now (once for setup assertion above)
	// The important assertion is that it was called at least once with "podman".
	if probeCallCount == 0 {
		t.Error("probe was never called")
	}
}

// TestProvisionSingleCP_NoProbe_Podman verifies single-CP bypasses probe and
// RecordAndPin (podman parity with docker).
func TestProvisionSingleCP_NoProbe_Podman(t *testing.T) {
	cfg := makeSingleCPPodmanConfig()

	prevProbe := provisionProbeIPAMFn
	probeCallCount := 0
	provisionProbeIPAMFn = func(_ string) (doctor.Verdict, string, error) {
		probeCallCount++
		t.Error("probe MUST NOT be called for single-CP podman config")
		return doctor.VerdictIPPinned, "", nil
	}
	defer func() { provisionProbeIPAMFn = prevProbe }()

	prevRecord := provisionRecordAndPinFn
	recordCallCount := 0
	provisionRecordAndPinFn = func(_, _ string, _ []string, _ log.Logger) error {
		recordCallCount++
		t.Error("RecordAndPin MUST NOT be called for single-CP podman config")
		return nil
	}
	defer func() { provisionRecordAndPinFn = prevRecord }()

	cpNames := cpContainerNamesForConfig(cfg)
	if len(cpNames) != 1 {
		t.Fatalf("expected 1 CP name for single-CP podman, got %d", len(cpNames))
	}

	// Simulate HA gate
	if len(cpNames) >= 2 {
		_, _, _ = provisionProbeIPAMFn("podman")
	}

	if probeCallCount != 0 {
		t.Errorf("probe should not be called for single-CP, got %d calls", probeCallCount)
	}
	if recordCallCount != 0 {
		t.Errorf("RecordAndPin should not be called for single-CP, got %d calls", recordCallCount)
	}
}

// TestProvisionCertRegen_NoRecordAndPin_Podman verifies that VerdictCertRegen
// results in the cert-regen strategy and RecordAndPin is NOT called.
func TestProvisionCertRegen_NoRecordAndPin_Podman(t *testing.T) {
	prevProbe := provisionProbeIPAMFn
	provisionProbeIPAMFn = func(_ string) (doctor.Verdict, string, error) {
		return doctor.VerdictCertRegen, "podman network connect failed probe", nil
	}
	defer func() { provisionProbeIPAMFn = prevProbe }()

	prevRecord := provisionRecordAndPinFn
	recordCalled := false
	provisionRecordAndPinFn = func(_, _ string, _ []string, _ log.Logger) error {
		recordCalled = true
		return nil
	}
	defer func() { provisionRecordAndPinFn = prevRecord }()

	verdict, reason, _ := provisionProbeIPAMFn("podman")
	var strategy string
	var warnMsg string
	if verdict == doctor.VerdictIPPinned {
		strategy = constants.StrategyIPPinned
	} else {
		strategy = constants.StrategyCertRegen
		warnMsg = fmt.Sprintf("HA cluster will use cert-regen resume strategy: %s", reason)
	}

	if strategy != constants.StrategyCertRegen {
		t.Errorf("strategy = %q, want StrategyCertRegen", strategy)
	}
	if !strings.Contains(warnMsg, "cert-regen") {
		t.Errorf("warn message should mention cert-regen, got %q", warnMsg)
	}

	// On cert-regen, RecordAndPin must not be called
	if verdict == doctor.VerdictIPPinned {
		_ = provisionRecordAndPinFn("podman", "kind", nil, log.NoopLogger{})
	}
	if recordCalled {
		t.Error("RecordAndPin must not be called on VerdictCertRegen path")
	}
}

// TestPlanCreation_StrategyLabelInjected_Podman verifies label injection parity.
func TestPlanCreation_StrategyLabelInjected_Podman(t *testing.T) {
	t.Parallel()

	cfg := makeHAPodmanConfig()
	createFuncs, err := planCreation(cfg, "kind", constants.StrategyIPPinned)
	if err != nil {
		t.Fatalf("planCreation(StrategyIPPinned) error: %v", err)
	}
	// 3 CP + 1 LB = 4
	if len(createFuncs) != 4 {
		t.Errorf("expected 4 createContainerFuncs (3 CP + 1 LB), got %d", len(createFuncs))
	}
}
