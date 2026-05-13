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
// Architecture (per 57.1-CONTEXT.md D-locks):
//   - Fires at Phase 1.25 (after LB start, before ip-pin/CP start)
//   - No-op for single-CP clusters (lb == nil short-circuit)
//   - Retry 3x with 1s backoff between attempts (no sleep before attempt 1)
//   - Hard-fail after 3 attempts (Resume returns error; downstream phases
//     do not run)
//   - IPv6 detected via docker network inspect; failure defaults to IPv4
//     + V(1) log line (graceful fallback per CONTEXT.md Discretion)
package lifecycle

import (
	"encoding/json"
	"strings"
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

// discoverLBIPv6 returns true iff the docker network the LB is attached to
// has EnableIPv6=true. On any docker-inspect error, returns false and
// logs a V(1) line — sensible default per CONTEXT.md Claude's Discretion.
//
// Call shape (RESEARCH Finding 8):
//  1. <binary> inspect --format '{{json .NetworkSettings.Networks}}' <lb>
//     → JSON object whose first key is the network name
//  2. <binary> network inspect <network> --format '{{.EnableIPv6}}'
//     → "true\n" or "false\n"
func discoverLBIPv6(binaryName string, lb nodes.Node, logger log.Logger) bool {
	var out strings.Builder
	cmd := defaultCmder(binaryName, "inspect", "--format",
		"{{json .NetworkSettings.Networks}}", lb.String())
	cmd.SetStdout(&out)
	if err := cmd.Run(); err != nil {
		logger.V(1).Infof("LB IPv6 discovery: docker inspect failed (%v); defaulting to IPv4", err)
		return false
	}
	var networks map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &networks); err != nil {
		logger.V(1).Infof("LB IPv6 discovery: networks JSON parse failed (%v); defaulting to IPv4", err)
		return false
	}
	var networkName string
	for k := range networks {
		networkName = k
		break // RESEARCH PIT-3: take first key; kind LB is on its dedicated network
	}
	if networkName == "" {
		logger.V(1).Infof("LB IPv6 discovery: no network attachments; defaulting to IPv4")
		return false
	}
	var out2 strings.Builder
	cmd2 := defaultCmder(binaryName, "network", "inspect", networkName, "--format", "{{.EnableIPv6}}")
	cmd2.SetStdout(&out2)
	if err := cmd2.Run(); err != nil {
		logger.V(1).Infof("LB IPv6 discovery: docker network inspect %q failed (%v); defaulting to IPv4", networkName, err)
		return false
	}
	return strings.TrimSpace(out2.String()) == "true"
}

// reapplyLBConfig re-renders and atomically swaps the Envoy LB's cds.yaml
// and lds.yaml inside `lb`, using container names from `cps` as upstream
// backends. No-op when lb is nil (single-CP clusters per SC3). Retries
// 3x with defaultLBRetryBackoff between attempts; on exhaustion returns
// a wrapped error so Resume's downstream phases do not run.
func reapplyLBConfig(binaryName string, lb nodes.Node, cps []nodes.Node, logger log.Logger) error {
	if lb == nil {
		return nil
	}
	ipv6 := discoverLBIPv6(binaryName, lb, logger)
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
