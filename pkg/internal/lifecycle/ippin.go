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

// Package lifecycle — IP-pin module.
//
// This file implements the recording side of the IP-pin contract for HA
// clusters (≥2 control-plane nodes). At cluster create time, after all
// containers are running, RecordAndPinHAControlPlane:
//   1. Calls the IPAM probe to determine the appropriate strategy.
//   2. On VerdictIPPinned: for each CP container inspects its assigned IPv4,
//      writes /kind/ipam-state.json inside the container, then disconnect +
//      reconnect with --ip to pin the address for future stop/start cycles.
//   3. On any other verdict (VerdictCertRegen / VerdictUnsupported): logs a
//      warning and returns nil — the label already attached at run time by the
//      provider will cause Plan 52-03 to choose the cert-regen path.
//
// Plan 52-03 owns the consuming side (resume-time orchestration).
// Plan 52-02 Task 2 wires the run-time strategy label at docker/podman run.
//
// Legacy detection: absence of the resume-strategy label on the bootstrap CP
// container signals the legacy (pre-52-02) path — see CONTEXT.md "legacy
// detection mechanism".

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/doctor"
	"sigs.k8s.io/kind/pkg/log"
)

// ResumeStrategyLabel is the Docker/Podman label key written at container
// creation time to record the resume strategy for this cluster.
// Absence of the label (legacy clusters) → cert-regen branch at resume time.
const ResumeStrategyLabel = "io.x-k8s.kinder.resume-strategy"

// StrategyIPPinned is the label value indicating that each CP container's IP
// has been pinned and /kind/ipam-state.json records the assigned address.
const StrategyIPPinned = "ip-pinned"

// StrategyCertRegen is the label value indicating that cert-regen will be
// used at resume time (IP pinning unavailable or probe returned non-pinned).
const StrategyCertRegen = "cert-regen"

// ipamStatePath is the in-container path where IP state is persisted.
const ipamStatePath = "/kind/ipam-state.json"

// IPAMState is the JSON written to ipamStatePath on each pinned CP container.
// Plan 52-03 reads this back via ReadIPAMState at resume time.
type IPAMState struct {
	Network string `json:"network"`
	IPv4    string `json:"ipv4"`
}

// probeIPAMFn is the package-private injection point for the doctor probe.
// Tests swap this var to avoid real container operations (mirrors
// defaultReadinessProber in resume.go).
var probeIPAMFn = doctor.ProbeIPAM

// ippinCmder is the package-private command factory used by all ippin
// operations. Tests swap it via t.Cleanup to inject a fake. Do NOT use
// t.Parallel() in tests that mutate this var.
var ippinCmder Cmder = exec.Command

// RecordAndPinHAControlPlane is called by the docker AND podman providers
// after their respective Provision succeeds. It is a no-op when
// len(cpContainers) <= 1 (single-CP, zero overhead per D-locked decision).
//
// For HA configs (len(cpContainers) >= 2):
//  1. Calls probeIPAMFn(binaryName) once.
//  2. If verdict == VerdictIPPinned: for each CP container, inspects IP,
//     writes /kind/ipam-state.json (chmod 0600, T-52-02-03 mitigation), then
//     disconnects + reconnects with --ip to pin the address.
//  3. Otherwise: logs a Warn with the reason and returns nil (the strategy
//     label was already attached at run time as cert-regen by Task 2).
//
// Error handling (W1 disposition):
//   - Inspect returns invalid IP → wrapped error.
//   - Disconnect fails → wrapped error; subsequent CPs NOT processed.
//   - connect --ip fails: best-effort recovery reconnect (no --ip).
//     If recovery succeeds → return wrapped error containing original failure.
//     If recovery ALSO fails → return wrapped error containing BOTH causes.
//   Provision surfaces these as a fatal cluster-create failure.
func RecordAndPinHAControlPlane(
	binaryName string,
	networkName string,
	cpContainers []string,
	logger log.Logger,
) error {
	// D-locked: single-CP incurs zero overhead.
	if len(cpContainers) <= 1 {
		return nil
	}

	verdict, reason, _ := probeIPAMFn(binaryName)
	if verdict != doctor.VerdictIPPinned {
		logger.Warnf("IP pinning unavailable for %s (strategy=cert-regen): %s", binaryName, reason)
		return nil
	}

	for _, c := range cpContainers {
		if err := pinContainer(binaryName, networkName, c, logger); err != nil {
			return err
		}
	}

	logger.V(0).Infof("pinned %d HA control-plane IPs (strategy=%s)",
		len(cpContainers), StrategyIPPinned)
	return nil
}

// pinContainer performs the full pin sequence for a single CP container:
// inspect IP → write state file → disconnect → reconnect with --ip.
func pinContainer(binaryName, networkName, container string, logger log.Logger) error {
	// Step 1: Inspect the container's current IP on the kind network.
	// T-52-02-01: validate via net.ParseIP before reuse in --ip flag.
	lines, err := exec.OutputLines(ippinCmder(binaryName,
		"inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		container,
	))
	if err != nil {
		return errors.Wrapf(err, "failed to inspect IP for container %s", container)
	}
	rawIP := strings.TrimSpace(strings.Join(lines, ""))
	if net.ParseIP(rawIP) == nil {
		return errors.Errorf("invalid IP from docker inspect for %s: %q", container, rawIP)
	}

	// Step 2: Marshal and write /kind/ipam-state.json inside the running container.
	// Mirrors /kind/pause-snapshot.json pattern from pause.go (Phase 47).
	// T-52-02-03: chmod 0600 at write time.
	state := IPAMState{Network: networkName, IPv4: rawIP}
	payload, err := json.Marshal(state)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal IPAM state for %s", container)
	}
	script := fmt.Sprintf(
		"cat > %s <<'IPAM_EOF'\n%s\nIPAM_EOF\nchmod 0600 %s\n",
		ipamStatePath, string(payload), ipamStatePath,
	)
	if err := ippinCmder(binaryName, "exec", container, "sh", "-c", script).Run(); err != nil {
		return errors.Wrapf(err, "failed to write %s on container %s", ipamStatePath, container)
	}

	// Step 3: Disconnect from the network (required before reconnect with --ip).
	if err := ippinCmder(binaryName, "network", "disconnect", networkName, container).Run(); err != nil {
		return errors.Wrapf(err, "failed to disconnect %s from network %s", container, networkName)
	}

	// Step 4: Reconnect with the pinned IP.
	// W1 disposition: connect failure → best-effort recovery → hard error.
	connectErr := ippinCmder(binaryName, "network", "connect",
		"--ip", rawIP, networkName, container,
	).Run()
	if connectErr != nil {
		logger.Warnf("failed to connect --ip %s for %s: %v; attempting recovery reconnect",
			rawIP, container, connectErr)
		// Best-effort recovery: reconnect without --ip to restore network access.
		recoveryErr := ippinCmder(binaryName, "network", "connect", networkName, container).Run()
		if recoveryErr != nil {
			// Both connect attempts failed. Container is left disconnected.
			// Return wrapped error with both causes for operator diagnosis.
			return errors.Errorf(
				"failed to pin IP %s for %s: connect-ip error: %v; recovery-connect error: %v",
				rawIP, container, connectErr, recoveryErr,
			)
		}
		// Recovery succeeded but the IP is not pinned — Provision must still fail.
		return errors.Wrapf(connectErr,
			"failed to pin IP %s for %s (recovery reconnect succeeded without --ip, but pin failed)",
			rawIP, container,
		)
	}

	return nil
}

// ReadIPAMState reads /kind/ipam-state.json from a container (stopped or
// running) by copying it to the host via `<binaryName> cp`. This is the
// consume-side helper used by Plan 52-03 at resume time.
//
// tmpDir is a caller-provided temporary directory; the extracted file is
// named "<container>-ipam.json" inside it.
func ReadIPAMState(binaryName, container, tmpDir string) (*IPAMState, error) {
	hostPath := filepath.Join(tmpDir, container+"-ipam.json")
	if err := ippinCmder(binaryName, "cp",
		container+":"+ipamStatePath, hostPath,
	).Run(); err != nil {
		return nil, errors.Wrapf(err, "docker cp %s ipam-state", container)
	}
	data, err := os.ReadFile(hostPath)
	if err != nil {
		return nil, errors.Wrapf(err, "read ipam-state from %s", hostPath)
	}
	var state IPAMState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, errors.Wrapf(err, "parse ipam-state from %s", hostPath)
	}
	return &state, nil
}
