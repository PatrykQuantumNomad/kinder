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

// Package ippin implements the IP-pin recording and reading helpers used by
// the docker and podman providers at cluster create time and by Plan 52-03's
// resume path.
//
// This package is intentionally free of any dependency on
// sigs.k8s.io/kind/pkg/cluster or the provider packages — it only depends on
// exec, errors, log, and doctor — so that providers can import it without
// introducing an import cycle. See SUMMARY 52-02 for the full rationale.
//
// The sigs.k8s.io/kind/pkg/internal/lifecycle package re-exports the public
// API of this package so that callers using the lifecycle facade do not need
// to take a direct dependency on this internal package.
package ippin

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

// ipamStatePath is the in-container path where IP state is persisted.
const ipamStatePath = "/kind/ipam-state.json"

// IPAMState is the JSON written to ipamStatePath on each pinned CP container.
// Plan 52-03 reads this back via ReadIPAMState at resume time.
type IPAMState struct {
	Network string `json:"network"`
	IPv4    string `json:"ipv4"`
}

// Cmder is the function signature used to construct exec.Cmd objects.
// Tests can swap the package-level IppinCmder to inject fakes.
type Cmder func(name string, args ...string) exec.Cmd

// IppinCmder is the package-level command factory. Tests swap it via
// t.Cleanup to inject a fake. Tests that swap this var MUST NOT use
// t.Parallel() because it is a shared package-level global.
var IppinCmder Cmder = exec.Command

// ProbeIPAMFn is the package-level injection point for the IPAM probe.
// Tests swap it to control the probe verdict without running real container
// operations. Tests that swap this var MUST NOT use t.Parallel().
var ProbeIPAMFn = doctor.ProbeIPAM

// RecordAndPinHAControlPlane is called by the docker AND podman providers
// after their respective Provision succeeds. It is a no-op when
// len(cpContainers) <= 1 (single-CP, zero overhead per D-locked decision).
//
// For HA configs (len(cpContainers) >= 2):
//  1. Calls ProbeIPAMFn(binaryName) once.
//  2. If verdict == VerdictIPPinned: for each CP container inspects IP,
//     writes /kind/ipam-state.json (chmod 0600, T-52-02-03), disconnects +
//     reconnects with --ip to pin the address.
//  3. Otherwise: logs a Warn with the reason and returns nil (the strategy
//     label was already attached at run time as cert-regen by the provider).
//
// Error handling (W1 disposition):
//   - Inspect returns invalid IP → wrapped error.
//   - Disconnect fails → wrapped error; subsequent CPs NOT processed.
//   - connect --ip fails: best-effort recovery reconnect (no --ip).
//     If recovery succeeds → return wrapped error (original failure).
//     If recovery ALSO fails → return wrapped error containing BOTH causes.
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

	verdict, reason, _ := ProbeIPAMFn(binaryName)
	if verdict != doctor.VerdictIPPinned {
		logger.Warnf("IP pinning unavailable for %s (strategy=cert-regen): %s", binaryName, reason)
		return nil
	}

	for _, c := range cpContainers {
		if err := pinContainer(binaryName, networkName, c, logger); err != nil {
			return err
		}
	}

	logger.V(0).Infof("pinned %d HA control-plane IPs (strategy=ip-pinned)",
		len(cpContainers))
	return nil
}

// pinContainer performs the full pin sequence for a single CP container.
func pinContainer(binaryName, networkName, container string, logger log.Logger) error {
	// Step 1: Inspect IP (T-52-02-01: net.ParseIP validation before --ip reuse).
	lines, err := exec.OutputLines(IppinCmder(binaryName,
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

	// Step 2: Write /kind/ipam-state.json inside the running container.
	// Mirrors /kind/pause-snapshot.json pattern from pause.go (Phase 47).
	// T-52-02-03: chmod 0600 at write time.
	state := IPAMState{Network: networkName, IPv4: rawIP}
	payload, mErr := json.Marshal(state)
	if mErr != nil {
		return errors.Wrapf(mErr, "failed to marshal IPAM state for %s", container)
	}
	script := fmt.Sprintf(
		"cat > %s <<'IPAM_EOF'\n%s\nIPAM_EOF\nchmod 0600 %s\n",
		ipamStatePath, string(payload), ipamStatePath,
	)
	if err := IppinCmder(binaryName, "exec", container, "sh", "-c", script).Run(); err != nil {
		return errors.Wrapf(err, "failed to write %s on container %s", ipamStatePath, container)
	}

	// Step 3: Disconnect from the network (required before reconnect with --ip).
	if err := IppinCmder(binaryName, "network", "disconnect", networkName, container).Run(); err != nil {
		return errors.Wrapf(err, "failed to disconnect %s from network %s", container, networkName)
	}

	// Step 4: Reconnect with the pinned IP (W1 disposition).
	connectErr := IppinCmder(binaryName, "network", "connect",
		"--ip", rawIP, networkName, container,
	).Run()
	if connectErr != nil {
		logger.Warnf("failed to connect --ip %s for %s: %v; attempting recovery reconnect",
			rawIP, container, connectErr)
		recoveryErr := IppinCmder(binaryName, "network", "connect", networkName, container).Run()
		if recoveryErr != nil {
			return errors.Errorf(
				"failed to pin IP %s for %s: connect-ip error: %v; recovery-connect error: %v",
				rawIP, container, connectErr, recoveryErr,
			)
		}
		return errors.Wrapf(connectErr,
			"failed to pin IP %s for %s (recovery reconnect succeeded without --ip, but pin failed)",
			rawIP, container,
		)
	}

	return nil
}

// ReadIPAMState reads /kind/ipam-state.json from a container via
// `<binaryName> cp`. Works on stopped containers (RESEARCH PIT-2).
// Plan 52-03 calls this at resume time.
func ReadIPAMState(binaryName, container, tmpDir string) (*IPAMState, error) {
	hostPath := filepath.Join(tmpDir, container+"-ipam.json")
	if err := IppinCmder(binaryName, "cp",
		container+":"+ipamStatePath, hostPath,
	).Run(); err != nil {
		return nil, errors.Wrapf(err, "docker cp %s ipam-state", container)
	}
	data, err := os.ReadFile(hostPath)
	if err != nil {
		return nil, errors.Wrapf(err, "read ipam-state from %s", hostPath)
	}
	var st IPAMState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, errors.Wrapf(err, "parse ipam-state from %s", hostPath)
	}
	return &st, nil
}
