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

// Package lifecycle – certregen.go provides the reactive drift detection and
// wholesale etcd peer cert regeneration helpers consumed by resume.go.
//
// Both functions live in the same package as resume.go so resume.go calls them
// as unqualified names (IPDriftDetected, RegenerateEtcdPeerCertsWholesale) —
// this is the W2 naming fix: zero `certregen.` package qualifiers.
package lifecycle

import (
	"net"
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// Package-level constants for the etcd static-pod cycle timing.
// Per RESEARCH: kubelet fileCheckFrequency=20s + 5s safety margin = 25s wait
// for kubelet to notice the removed manifest. Then 20s after restoration.
const (
	kubeletFileCheckFrequency  = 20 * time.Second
	staticPodCycleSafetyMargin = 5 * time.Second
	staticPodRecreationWait    = 20 * time.Second
	etcdManifestPath           = "/etc/kubernetes/manifests/etcd.yaml"
	etcdManifestBackup         = "/tmp/etcd-bak.yaml"
)

// certRegenSleeper is a package-level var so tests can swap it to a no-op,
// preventing real 25s+20s sleep blocks during unit tests.
// Production value: time.Sleep.
var certRegenSleeper = func(d time.Duration) { time.Sleep(d) }

// IPDriftDetected returns true iff the current docker-inspect IP for a CP
// differs from the value recorded in /kind/ipam-state.json (or no recording
// exists, i.e. legacy cluster).
//
// Parameters:
//   - binaryName: container runtime CLI ("docker", "podman").
//   - container: CP container name.
//   - tmpDir: host temp directory for the docker-cp file staging.
//
// Returns: (drifted, currentIP, recordedIP, err).
//
// Legacy cluster (no state file): recordedIP="", drifted=true, err=nil.
// If ReadIPAMState fails for a non-"no such file" reason, the error is returned.
// T-52-03-01: currentIP is validated via net.ParseIP before returning.
func IPDriftDetected(binaryName, container, tmpDir string) (drifted bool, currentIP string, recordedIP string, err error) {
	// Step 1: Read the recorded state from /kind/ipam-state.json.
	// ReadIPAMState is in the same package (ippin.go facade → pkg/internal/ippin).
	state, readErr := ReadIPAMState(binaryName, container, tmpDir)
	if readErr != nil {
		// Legacy detection: if the file is absent, treat the cluster as legacy
		// → always regen (cert-regen forever for legacy per CONTEXT.md).
		// We use a broad string match because the error is wrapped; the root cause
		// from docker cp typically contains "No such file" or "not found".
		errStr := strings.ToLower(readErr.Error())
		if strings.Contains(errStr, "no such file") ||
			strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "does not exist") {
			// Legacy path: recordedIP="", drifted=true.
			return true, "", "", nil
		}
		// Unexpected error — propagate.
		return false, "", "", readErr
	}
	recordedIP = state.IPv4

	// Step 2: Inspect the current container IP via the runtime CLI.
	// Uses defaultCmder (same injection point as resume.go / state.go).
	lines, inspectErr := exec.OutputLines(defaultCmder(binaryName,
		"inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		container,
	))
	if inspectErr != nil {
		return false, "", recordedIP, errors.Wrapf(inspectErr, "IPDriftDetected: failed to inspect %s", container)
	}
	rawIP := strings.TrimSpace(strings.Join(lines, ""))

	// T-52-03-01: Validate IP before use.
	if net.ParseIP(rawIP) == nil {
		return false, "", recordedIP, errors.Errorf("IPDriftDetected: invalid IP from inspect for %s: %q", container, rawIP)
	}
	currentIP = rawIP

	// Drift = IPs differ, or recorded was empty (shouldn't happen given state
	// file was read without error, but defensive).
	drifted = currentIP != recordedIP || recordedIP == ""
	return drifted, currentIP, recordedIP, nil
}

// RegenerateEtcdPeerCertsWholesale runs `kubeadm certs renew etcd-peer` on
// every CP node and cycles the etcd static pod. All CPs must be started before
// this call. Failure on any CP halts the operation and returns a structured
// diagnostic error directing the user to delete and recreate the cluster.
//
// The function is a no-op when len(cpNodes) <= 1 (defense in depth; callers
// already gate on HA, but safety is preserved here too).
//
// Timing: static pod cycle uses sleep-based approach (kubelet
// fileCheckFrequency=20s + 5s margin, then 20s after restoration). No crictl
// polling — adds a runtime dependency not already wired. See RESEARCH.
//
// Progress: emits a V(0).Infof per CP node: "Regenerating etcd peer cert on
// <node> (N/M)". Matches the existing lifecycle.Resume progress UX.
func RegenerateEtcdPeerCertsWholesale(cpNodes []nodes.Node, logger log.Logger) error {
	if len(cpNodes) <= 1 {
		return nil
	}

	total := len(cpNodes)
	for i, node := range cpNodes {
		logger.V(0).Infof("Regenerating etcd peer cert on %s (%d/%d)", node.String(), i+1, total)

		// Step 1: kubeadm certs renew etcd-peer.
		if err := node.Command("kubeadm", "certs", "renew", "etcd-peer").Run(); err != nil {
			return errors.Wrapf(err,
				"etcd peer cert regen failed on %s: kubeadm certs renew error. Cluster state is undefined — delete and recreate the cluster",
				node.String(),
			)
		}

		// Step 2: Remove etcd manifest so kubelet stops the pod.
		if err := node.Command("mv", etcdManifestPath, etcdManifestBackup).Run(); err != nil {
			return errors.Wrapf(err,
				"etcd peer cert regen failed on %s: failed to move etcd manifest out. Cluster state is undefined — delete and recreate the cluster",
				node.String(),
			)
		}

		// Step 3: Wait for kubelet to notice the manifest is gone.
		// kubelet fileCheckFrequency=20s; add 5s safety margin = 25s.
		certRegenSleeper(kubeletFileCheckFrequency + staticPodCycleSafetyMargin)

		// Step 4: Restore the manifest so kubelet recreates the pod.
		if err := node.Command("mv", etcdManifestBackup, etcdManifestPath).Run(); err != nil {
			// The manifest is at /tmp/etcd-bak.yaml on the node. Log a warning.
			logger.Warnf("etcd manifest restore failed on %s (manifest remains at %s): %v — delete and recreate the cluster",
				node.String(), etcdManifestBackup, err)
			return errors.Wrapf(err,
				"etcd peer cert regen failed on %s: failed to restore etcd manifest. Cluster state is undefined — delete and recreate the cluster",
				node.String(),
			)
		}

		// Step 5: Wait for kubelet to recreate the etcd pod.
		certRegenSleeper(staticPodRecreationWait)
	}

	return nil
}
