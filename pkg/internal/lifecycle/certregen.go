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

import (
	"time"

	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/log"
)

// Package-level constants for the etcd static-pod cycle timing.
// Per RESEARCH: kubelet fileCheckFrequency=20s + 5s safety margin = 25s wait
// for kubelet to notice the removed manifest. Then 20s after restoration.
const (
	kubeletFileCheckFrequency    = 20 * time.Second
	staticPodCycleSafetyMargin   = 5 * time.Second
	staticPodRecreationWait      = 20 * time.Second
	etcdManifestPath             = "/etc/kubernetes/manifests/etcd.yaml"
	etcdManifestBackup           = "/tmp/etcd-bak.yaml"
)

// certRegenSleeper is a package-level var so tests can swap it to a no-op,
// preventing real 25s+20s sleep blocks during unit tests.
var certRegenSleeper = func(d time.Duration) { time.Sleep(d) }

// IPDriftDetected returns true iff the current docker-inspect IP for a CP
// differs from the value recorded in /kind/ipam-state.json, or the recording
// is absent (legacy cluster = always regen).
//
// Parameters:
//   - binaryName: container runtime CLI ("docker", "podman").
//   - container: CP container name.
//   - tmpDir: host temp directory for the docker-cp file staging.
//
// Returns: (drifted, currentIP, recordedIP, err).
// Legacy (no state file) → drifted=true, recordedIP="", err=nil.
func IPDriftDetected(binaryName, container, tmpDir string) (drifted bool, currentIP string, recordedIP string, err error) {
	// TODO: implement
	return false, "", "", errors.New("not implemented")
}

// RegenerateEtcdPeerCertsWholesale runs `kubeadm certs renew etcd-peer` on
// every CP node and cycles the etcd static pod. All CPs must be started before
// this call. Failure of any CP halts the operation and returns a structured
// diagnostic error.
//
// The function is a no-op when len(cpNodes) <= 1 (defense in depth; callers
// gate on HA, but safety is preserved here too).
func RegenerateEtcdPeerCertsWholesale(cpNodes []nodes.Node, logger log.Logger) error {
	// TODO: implement
	return errors.New("not implemented")
}
