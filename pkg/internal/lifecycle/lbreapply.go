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

// Package lifecycle — lbreapply.go provides Phase 1.25 of Resume(): re-
// apply the Envoy LB's dynamic xDS config (cds.yaml + lds.yaml) after the
// LB container starts. The LB entrypoint resets both files to "resources:
// []" on every container start (loadbalancer/config.go:167-174); without
// this reapply, post-resume the LB has zero clusters and host kubectl gets
// EOF — see Phase 57.1 ROADMAP entry.
//
// Architecture (per 57.1-CONTEXT.md D-locks + Phase 57.2 amendment):
//   - Fires at Phase 1.25 (after LB start, before ip-pin/CP start)
//   - No-op for single-CP clusters (lb == nil short-circuit)
//   - Retry 3x with 1s backoff between attempts (no sleep before attempt 1)
//   - Hard-fail after 3 attempts (Resume returns error; downstream phases
//     do not run)
//   - IPv6 detected via `loadbalancer.ClusterIPFamily(node)` which reads
//     the `io.x-k8s.kinder.ip-family` label stamped on the LB container at
//     create time (Phase 57.2; replaces the broken docker-network probe
//     from Phase 57.1 which read `docker network inspect EnableIPv6` and
//     mis-classified IPv4 clusters as IPv6 on macOS Docker Desktop's
//     dual-stack `kind` network — see hack/uat-47-ha-smoke.log.pre-57.2)
package lifecycle

import (
	"time"

	"sigs.k8s.io/kind/pkg/cluster/loadbalancer"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/log"
)

// defaultLBRetryBackoff is the pause between LB reapply attempts. Tests
// override to 0 (via the test helper in lbreapply_test.go) for fast
// retry-exhaustion tests (CONTEXT.md D-lock 2). Production value: 1 second.
var defaultLBRetryBackoff = 1 * time.Second

// reapplyLBConfig re-renders and atomically swaps the Envoy LB's cds.yaml
// and lds.yaml inside `lb`, using container names from `cps` as upstream
// backends. No-op when lb is nil (single-CP clusters per SC3). Retries
// 3x with defaultLBRetryBackoff between attempts; on exhaustion returns
// a wrapped error so Resume's downstream phases do not run.
//
// Phase 57.2: IPv6 mode is derived from the cluster-authoritative
// `io.x-k8s.kinder.ip-family` label on the LB container via
// loadbalancer.ClusterIPFamily. If the label is absent or unrecognized,
// the call fails loudly with "delete and recreate the cluster" guidance —
// there is no fallback to the (broken) docker-network probe.
func reapplyLBConfig(binaryName string, lb nodes.Node, cps []nodes.Node, logger log.Logger) error {
	if lb == nil {
		return nil
	}
	ipv6, err := loadbalancer.ClusterIPFamily(binaryName, lb)
	if err != nil {
		return errors.Wrapf(err,
			"reapplyLBConfig: cannot determine cluster IP family")
	}
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(defaultLBRetryBackoff) // RESEARCH PIT-2: sleep BETWEEN, not before first
		}
		err := loadbalancer.WriteDynamicConfig(lb, cps, ipv6)
		if err == nil {
			return nil
		}
		lastErr = err
		logger.V(1).Infof("LB reapply attempt %d/%d failed: %v", attempt, maxAttempts, err)
	}
	return errors.Wrapf(lastErr, "LB reapply failed after %d attempts; re-run kinder resume to retry", maxAttempts)
}
