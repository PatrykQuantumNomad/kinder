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
	"errors"
	"fmt"

	kinderrors "sigs.k8s.io/kind/pkg/errors"
)

// Sentinel errors for the three locked compatibility dimensions.
// Use errors.Is to check; errors are wrapped with context so callers
// see the specific mismatch values.
var (
	// ErrCompatK8sMismatch is returned when the snapshot and live cluster
	// have different Kubernetes versions (LIFE-06 / SC2 hard-fail).
	ErrCompatK8sMismatch = errors.New("snapshot k8s version mismatch")

	// ErrCompatTopologyMismatch is returned when the snapshot topology does not
	// match the live cluster (control-plane count, worker count, LB presence).
	ErrCompatTopologyMismatch = errors.New("snapshot topology mismatch")

	// ErrCompatAddonMismatch is returned when the snapshot addon versions do not
	// exactly match the live cluster (extra addons on either side, or version drift).
	ErrCompatAddonMismatch = errors.New("snapshot addon version mismatch")
)

// CheckCompatibility hard-fails on the three locked compatibility dimensions:
// K8s version, topology (CP count, worker count, LB presence), and addon
// versions. Returns ALL violations as an aggregated error so the user sees
// every reason in one shot rather than fix-rerun-discover loops.
//
// CONTEXT.md locked decisions: K8s version, topology, and addon versions all
// hard-fail; no warn-only path.
func CheckCompatibility(snap, live *Metadata) error {
	var errs []error

	// Dimension 1: Kubernetes version must match exactly.
	if snap.K8sVersion != live.K8sVersion {
		errs = append(errs, fmt.Errorf("%w: snapshot=%s cluster=%s",
			ErrCompatK8sMismatch, snap.K8sVersion, live.K8sVersion))
	}

	// Dimension 2: topology must match exactly (CP count, worker count, LB presence).
	if snap.Topology != live.Topology {
		errs = append(errs, fmt.Errorf("%w: snapshot=%+v cluster=%+v",
			ErrCompatTopologyMismatch, snap.Topology, live.Topology))
	}

	// Dimension 3: addon versions must match exactly — same key set AND identical
	// values for every key. Extras on either side and version drift both fail.
	if !addonsEqual(snap.AddonVersions, live.AddonVersions) {
		errs = append(errs, fmt.Errorf("%w: snapshot=%v cluster=%v (must match exactly — extras and version drifts both fail)",
			ErrCompatAddonMismatch, snap.AddonVersions, live.AddonVersions))
	}

	return kinderrors.NewAggregate(errs)
}

// addonsEqual returns true if snap and live addon maps have the same key set
// and identical values for every key. nil and empty map are treated as equal.
func addonsEqual(snap, live map[string]string) bool {
	// Normalize nil to empty for comparison.
	sLen := len(snap)
	lLen := len(live)
	if sLen != lLen {
		return false
	}
	// At this point both are empty (nil/empty map) or same length.
	for k, sv := range snap {
		lv, ok := live[k]
		if !ok || sv != lv {
			return false
		}
	}
	return true
}
