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

package doctor

import (
	"fmt"
	"net"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"time"

	kindexec "sigs.k8s.io/kind/pkg/exec"

	"sigs.k8s.io/kind/pkg/apis/config/defaults"
	kinderrors "sigs.k8s.io/kind/pkg/errors"
)

// probeSubnet is the scratch network subnet used by the IPAM probe.
// It is intentionally non-overlapping with kind's default network range
// (172.18.0.0/16) as documented in RESEARCH.md "Probe Network Decision".
const probeSubnet = "172.200.0.0/24"

// Verdict is the IPAM probe outcome consumed by the doctor check and downstream
// lifecycle plans (52-02 ip-pin path, 52-03 cert-regen path).
type Verdict string

const (
	// VerdictIPPinned means the runtime supports `network connect --ip` and the
	// IP survived stop/start — the ip-pin lifecycle path is available.
	VerdictIPPinned Verdict = "ip-pinned"

	// VerdictCertRegen means IP pinning is unavailable or failed for this runtime;
	// the cert-regen fallback path will be used.
	VerdictCertRegen Verdict = "cert-regen"

	// VerdictUnsupported means the runtime does not implement `network connect`
	// at all (nerdctl per RESEARCH PIT-4); the cert-regen path is mandatory.
	VerdictUnsupported Verdict = "unsupported"
)

// ipamProbeCmder is the package-level command factory used by ProbeIPAM.
// Tests swap this via setIPAMProbeCmder to avoid real container operations.
var ipamProbeCmder func(name string, args ...string) kindexec.Cmd = func(name string, args ...string) kindexec.Cmd {
	return kindexec.Command(name, args...)
}

// ProbeIPAM runs the full stop→start lifecycle simulation against the given
// runtime CLI (docker / podman / nerdctl) and returns:
//   - verdict: the capability determination
//   - reason: human-readable explanation (empty on VerdictIPPinned)
//   - err: always nil — runtime errors produce VerdictCertRegen with a non-empty
//     reason (soft fallback per CONTEXT.md "Probe runtime-error verdict").
//
// Cleanup is deferred and runs best-effort on every code path, including early
// failures. Unique timestamped names prevent stale-resource collisions when
// concurrent invocations occur (RESEARCH "Probe Network Decision").
//
// nerdctl: short-circuits to VerdictUnsupported BEFORE any container ops
// because nerdctl network connect is not implemented (RESEARCH PIT-4).
func ProbeIPAM(binaryName string) (Verdict, string, error) {
	// nerdctl short-circuit: network connect is not implemented (RESEARCH PIT-4).
	if filepath.Base(binaryName) == "nerdctl" {
		return VerdictUnsupported, "nerdctl network connect is not implemented", nil
	}

	// Unique suffix ensures concurrent probe invocations do not collide.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	probeName := "kinder-ipam-probe-" + suffix
	probeNet := "kinder-ipam-probe-" + suffix

	// Defer best-effort cleanup. Each cleanup step is independent so failure of
	// one does not prevent the other (T-52-01-02 DoS mitigation).
	defer func() {
		_ = ipamProbeCmder(binaryName, "rm", "-f", probeName).Run()
		_ = ipamProbeCmder(binaryName, "network", "rm", probeNet).Run()
	}()

	// Step 1: create scratch network on a non-overlapping subnet.
	if err := ipamProbeCmder(binaryName, "network", "create",
		"--subnet="+probeSubnet, probeNet).Run(); err != nil {
		return VerdictCertRegen,
			fmt.Sprintf("probe runtime error: network create failed: %v", err),
			nil
	}

	// Step 2: start a scratch container. kindest/node is used because it is
	// always pulled by kinder, making the probe air-gap friendly (RESEARCH OQ-1).
	if err := ipamProbeCmder(binaryName, "run", "-d",
		"--name", probeName,
		"--network", probeNet,
		defaults.Image,
		"sleep", "30",
	).Run(); err != nil {
		return VerdictCertRegen,
			fmt.Sprintf("probe runtime error: container run failed: %v", err),
			nil
	}

	// Step 3: capture the container's assigned IP.
	// Use the same `{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}`
	// format as realInspectState uses the State.Status format — consistent pattern.
	inspectLines, err := kindexec.OutputLines(ipamProbeCmder(binaryName,
		"inspect", "--format",
		"{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		probeName,
	))
	if err != nil {
		return VerdictCertRegen,
			fmt.Sprintf("probe runtime error: inspect (pre-stop) failed: %v", err),
			nil
	}
	originalIP := strings.TrimSpace(strings.Join(inspectLines, ""))
	// T-52-01-01: validate the captured IP before passing it back to --ip.
	if net.ParseIP(originalIP) == nil {
		return VerdictCertRegen,
			fmt.Sprintf("probe runtime error: inspect returned invalid IP %q", originalIP),
			nil
	}

	// Step 4: disconnect from the probe network.
	_ = ipamProbeCmder(binaryName, "network", "disconnect", probeNet, probeName).Run()

	// Step 5: reconnect with the pinned IP (simulates the kinder resume ip-pin step).
	if err := ipamProbeCmder(binaryName, "network", "connect",
		"--ip", originalIP, probeNet, probeName,
	).Run(); err != nil {
		return VerdictCertRegen,
			fmt.Sprintf("network connect --ip unsupported or rejected: %v",
				kinderrors.WithStack(err)),
			nil
	}

	// Step 6: stop the container (RESEARCH PIT-3: must use stop, not pause).
	_ = ipamProbeCmder(binaryName, "stop", probeName).Run()

	// Step 7: start the container (order-inversion scenario exercised by single container).
	_ = ipamProbeCmder(binaryName, "start", probeName).Run()

	// Step 8: re-inspect IP — verify it survived stop/start.
	postLines, err := kindexec.OutputLines(ipamProbeCmder(binaryName,
		"inspect", "--format",
		"{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		probeName,
	))
	if err != nil {
		return VerdictCertRegen,
			fmt.Sprintf("probe runtime error: inspect (post-start) failed: %v", err),
			nil
	}
	postIP := strings.TrimSpace(strings.Join(postLines, ""))

	if postIP != originalIP {
		return VerdictCertRegen,
			fmt.Sprintf("IP reassigned from %s to %s across stop/start", originalIP, postIP),
			nil
	}

	return VerdictIPPinned, "", nil
}

// ipamProbeCheck is the doctor Check wrapper that executes ProbeIPAM and
// translates the verdict into a Result. Registered in allChecks as "ipam-probe"
// in the Network category (D-01 locked decision).
type ipamProbeCheck struct {
	// probeFunc is the probe function; injectable for tests.
	probeFunc func(binaryName string) (Verdict, string, error)
	// detectRuntime returns the binary name of the first detected runtime,
	// or "" if no runtime is found. Injectable for tests.
	detectRuntime func() string
}

// newIPAMProbeCheck returns the production Check wired to the real ProbeIPAM
// function and the system runtime detector.
func newIPAMProbeCheck() Check {
	return &ipamProbeCheck{
		probeFunc:     ProbeIPAM,
		detectRuntime: detectRuntimeBinary,
	}
}

func (c *ipamProbeCheck) Name() string       { return "ipam-probe" }
func (c *ipamProbeCheck) Category() string    { return "Network" }
func (c *ipamProbeCheck) Platforms() []string { return nil }

// Run executes the IPAM probe and returns a single Result. Verdicts map to:
//   - VerdictIPPinned   → ok  (ip-pin lifecycle path available)
//   - VerdictCertRegen  → warn (cert-regen path will be used)
//   - VerdictUnsupported → warn (runtime lacks network connect)
func (c *ipamProbeCheck) Run() []Result {
	binary := c.detectRuntime()
	if binary == "" {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "no container runtime detected",
		}}
	}

	verdict, reason, _ := c.probeFunc(binary)
	switch verdict {
	case VerdictIPPinned:
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "IPAM supports IP pinning (ip-pinned path available)",
		}}
	case VerdictUnsupported:
		msg := fmt.Sprintf("%s does not support network connect (cert-regen path will be used; unsupported)", binary)
		if reason != "" {
			msg = reason + " (cert-regen path will be used; unsupported)"
		}
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  msg,
			Reason:   "nerdctl network connect is not implemented; cert-regen fallback is mandatory",
			Fix:      "No action required: kinder will use cert-regen on resume for nerdctl clusters",
		}}
	default: // VerdictCertRegen
		msg := "IPAM probe: cert-regen path will be used"
		if reason != "" {
			msg = fmt.Sprintf("IPAM probe: cert-regen path will be used (%s)", reason)
		}
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  msg,
			Reason:   "IP pinning is unavailable for this runtime; cert-regen fallback will be applied on HA resume",
			Fix:      "No action required: kinder handles this automatically on resume",
		}}
	}
}

// detectRuntimeBinary returns the first available container runtime binary name
// ("docker", "podman", "nerdctl") or "" if none is found on PATH.
// Mirrors the discovery pattern in realListCPNodes (resumereadiness.go).
func detectRuntimeBinary() string {
	for _, rt := range []string{"docker", "podman", "nerdctl"} {
		if _, err := osexec.LookPath(rt); err == nil {
			return rt
		}
	}
	return ""
}
