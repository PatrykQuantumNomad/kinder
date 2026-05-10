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

package lifecycle

// IP-pin facade: this file re-exports the public API from pkg/internal/ippin
// so that Plan 52-03's resume path (and other lifecycle callers) can use
// RecordAndPinHAControlPlane / ReadIPAMState without a direct dependency on
// the ippin package.
//
// Import-cycle note: pkg/internal/lifecycle transitively imports
// sigs.k8s.io/kind/pkg/cluster (via pause.go PauseOptions.Provider field),
// which imports the docker/podman providers. To break this cycle the IP-pin
// implementation lives in the neutral sigs.k8s.io/kind/pkg/internal/ippin
// package, which has no dependency on lifecycle or cluster. This file is the
// thin facade. See SUMMARY 52-02 for the full rationale.
//
// Strategy constants (ResumeStrategyLabel, StrategyIPPinned, StrategyCertRegen)
// live in sigs.k8s.io/kind/pkg/cluster/constants so that both the providers
// and this lifecycle facade can consume them without a cycle.

import (
	kindippin "sigs.k8s.io/kind/pkg/internal/ippin"
	"sigs.k8s.io/kind/pkg/log"
)

// IPAMState is the JSON written to /kind/ipam-state.json on each pinned CP.
// This is a type alias for ippin.IPAMState so callers in the lifecycle package
// do not need a direct import of pkg/internal/ippin.
type IPAMState = kindippin.IPAMState

// ippinCmder and probeIPAMFn are package-level shims that delegate to the
// corresponding variables in pkg/internal/ippin. Tests in lifecycle/ippin_test.go
// swap ippin.IppinCmder and ippin.ProbeIPAMFn directly.
//
// These vars are intentionally unexported and kept here only so the test file
// (which is in the lifecycle package) can reference them through the ippin
// package. See ippin_test.go for usage.

// RecordAndPinHAControlPlane delegates to ippin.RecordAndPinHAControlPlane.
// See that function for full documentation.
func RecordAndPinHAControlPlane(
	binaryName string,
	networkName string,
	cpContainers []string,
	logger log.Logger,
) error {
	return kindippin.RecordAndPinHAControlPlane(binaryName, networkName, cpContainers, logger)
}

// ReadIPAMState delegates to ippin.ReadIPAMState.
// See that function for full documentation.
func ReadIPAMState(binaryName, container, tmpDir string) (*IPAMState, error) {
	return kindippin.ReadIPAMState(binaryName, container, tmpDir)
}
