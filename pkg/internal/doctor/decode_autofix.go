/*
Copyright 2019 The Kubernetes Authors.

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
	"os"
	"strconv"
	"strings"

	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// Auto-fix test injection points — production = real impls; tests swap via t.Cleanup.
// execCommand is declared by Plan 50-02 (decode_collectors.go) and reused here.
var (
	readSysctlFn       = realReadSysctl       // func(key string) (int, error)
	writeSysctlFn      = realWriteSysctl      // func(key, value string) error
	getCoreDNSStatusFn = realGetCoreDNSStatus // func(binaryName, cpNode string) (string, error)
	inspectStateAutoFn = realInspectStateAuto // func(binaryName, container string) (string, error)
	execCmdFn          = realExecCmd          // func(binaryName string, args ...string) error
	geteuidFn          = os.Geteuid           // func() int
)

// Inotify safe minimums per RESEARCH §"Auto-Fix Whitelist".
const (
	inotifyWatchesMin   = 524288
	inotifyInstancesMin = 512
)

// InotifyRaiseMitigation produces a SafeMitigation that raises the
// fs.inotify.max_user_watches and max_user_instances kernel limits to
// safe minimums (524288 / 512). NeedsFix returns true only when at least
// one of the two values is below its minimum (idempotent).
func InotifyRaiseMitigation() *SafeMitigation {
	return &SafeMitigation{
		Name: "inotify-raise",
		NeedsFix: func() bool {
			w, _ := readSysctlFn("fs.inotify.max_user_watches")
			i, _ := readSysctlFn("fs.inotify.max_user_instances")
			return w < inotifyWatchesMin || i < inotifyInstancesMin
		},
		Apply: func() error {
			if err := writeSysctlFn("fs.inotify.max_user_watches", strconv.Itoa(inotifyWatchesMin)); err != nil {
				return fmt.Errorf("inotify watches: %w", err)
			}
			if err := writeSysctlFn("fs.inotify.max_user_instances", strconv.Itoa(inotifyInstancesMin)); err != nil {
				return fmt.Errorf("inotify instances: %w", err)
			}
			return nil
		},
		NeedsRoot: true,
	}
}

// CoreDNSRestartMitigation rolls coredns when the deployment is stuck
// (Pending or non-Running). NeedsRoot=false; uses in-node kubectl with
// /etc/kubernetes/admin.conf.
func CoreDNSRestartMitigation(binaryName, cpNodeName string) *SafeMitigation {
	return &SafeMitigation{
		Name: "coredns-restart",
		NeedsFix: func() bool {
			status, err := getCoreDNSStatusFn(binaryName, cpNodeName)
			if err != nil {
				return false // unknown state → don't risk an action
			}
			return status != "" && status != "Running"
		},
		Apply: func() error {
			return execCmdFn(binaryName,
				"exec", cpNodeName,
				"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
				"rollout", "restart", "deployment/coredns",
				"-n", "kube-system",
			)
		},
		NeedsRoot: false,
	}
}

// NodeContainerRestartMitigation starts a node container that is not in
// "running" state. NeedsRoot=false (docker access already required).
func NodeContainerRestartMitigation(binaryName, nodeName string) *SafeMitigation {
	return &SafeMitigation{
		Name: "node-container-restart",
		NeedsFix: func() bool {
			state, err := inspectStateAutoFn(binaryName, nodeName)
			if err != nil {
				return false
			}
			return state != "" && state != "running"
		},
		Apply: func() error {
			return execCmdFn(binaryName, "start", nodeName)
		},
		NeedsRoot: false,
	}
}

// realReadSysctl reads /proc/sys/<key-with-dots-as-slashes>.
func realReadSysctl(key string) (int, error) {
	path := "/proc/sys/" + strings.ReplaceAll(key, ".", "/")
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

// realWriteSysctl writes to /proc/sys directly. EACCES (non-root) surfaces
// as an error so the orchestrator's NeedsRoot guard kicks in upstream.
func realWriteSysctl(key, value string) error {
	path := "/proc/sys/" + strings.ReplaceAll(key, ".", "/")
	return os.WriteFile(path, []byte(value), 0644)
}

// realGetCoreDNSStatus returns the first coredns pod's status.phase via
// in-node kubectl. Empty string + nil error means "could not determine".
func realGetCoreDNSStatus(binaryName, cpNodeName string) (string, error) {
	lines, err := exec.OutputLines(execCommand(
		binaryName, "exec", cpNodeName,
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"get", "pods", "-n", "kube-system",
		"-l", "k8s-app=kube-dns",
		"-o", "jsonpath={.items[0].status.phase}",
	))
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", nil
	}
	return strings.TrimSpace(lines[0]), nil
}

// realInspectStateAuto inlines lifecycle.ContainerState to avoid the
// doctor->lifecycle import cycle (lifecycle/resume.go imports doctor).
// This is intentionally identical to realInspectState in resumereadiness.go.
func realInspectStateAuto(binaryName, container string) (string, error) {
	lines, err := exec.OutputLines(execCommand(
		binaryName, "inspect", "--format", "{{.State.Status}}", container,
	))
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", nil
	}
	return strings.TrimSpace(lines[0]), nil
}

// realExecCmd runs the command and returns its error (output discarded).
func realExecCmd(binaryName string, args ...string) error {
	return execCommand(binaryName, args...).Run()
}

// ---------------------------------------------------------------------------
// Orchestrator: ApplyDecodeAutoFix + PreviewDecodeAutoFix
// ---------------------------------------------------------------------------

// DecodeAutoFixContext supplies the runtime parameters that parameterized
// SafeMitigations (CoreDNSRestartMitigation, NodeContainerRestartMitigation)
// need at apply time. Plan 50-03's CLI populates this from the resolved
// cluster: BinaryName via lifecycle.ProviderBinaryName(); CPNodeName via
// provider.ListNodes() filtered to the first control-plane.
type DecodeAutoFixContext struct {
	BinaryName string // "docker", "podman", "nerdctl"
	CPNodeName string // first control-plane node container name
}

// mitigationFor returns the SafeMitigation that should be applied for the
// given match. If the match's Pattern.AutoFix is non-nil it is used directly
// (e.g., InotifyRaiseMitigation). Otherwise the orchestrator constructs the
// mitigation by Pattern.ID using ctx for parameters. Returns nil when no
// mitigation is applicable (e.g., AutoFixable=false, or Source has no
// extractable node name for KUB-05).
func mitigationFor(m DecodeMatch, ctx DecodeAutoFixContext, logger log.Logger) *SafeMitigation {
	if !m.Pattern.AutoFixable {
		return nil
	}
	if m.Pattern.AutoFix != nil {
		return m.Pattern.AutoFix
	}
	switch m.Pattern.ID {
	case "KADM-02":
		return CoreDNSRestartMitigation(ctx.BinaryName, ctx.CPNodeName)
	case "KUB-05":
		const prefix = "docker-logs:"
		if !strings.HasPrefix(m.Source, prefix) {
			if logger != nil {
				logger.V(1).Infof("auto-fix: cannot infer node name for KUB-05 from source %q", m.Source)
			}
			return nil
		}
		nodeName := strings.TrimPrefix(m.Source, prefix)
		return NodeContainerRestartMitigation(ctx.BinaryName, nodeName)
	}
	return nil
}

// PreviewDecodeAutoFix returns one human-readable line per whitelisted,
// deduped mitigation. Side-effect free w.r.t. Apply (NeedsFix MAY be called).
func PreviewDecodeAutoFix(matches []DecodeMatch, ctx DecodeAutoFixContext) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range matches {
		sm := mitigationFor(m, ctx, nil)
		if sm == nil || seen[sm.Name] {
			continue
		}
		seen[sm.Name] = true
		note := ""
		if !sm.NeedsFix() {
			note = " (precondition not met — skip)"
		} else if sm.NeedsRoot && geteuidFn() != 0 {
			note = " (requires root — skip)"
		}
		out = append(out, fmt.Sprintf("would apply %s for %s%s", sm.Name, m.Pattern.ID, note))
	}
	return out
}

// ApplyDecodeAutoFix applies whitelisted SafeMitigations from the matches.
// Mitigations are deduped by Name. NeedsFix is honored (idempotency).
// NeedsRoot+non-root → skip with warn log. Apply errors are collected and
// returned but do not abort the loop.
func ApplyDecodeAutoFix(matches []DecodeMatch, ctx DecodeAutoFixContext, logger log.Logger) []error {
	var errs []error
	seen := map[string]bool{}
	for _, m := range matches {
		sm := mitigationFor(m, ctx, logger)
		if sm == nil || seen[sm.Name] {
			continue
		}
		seen[sm.Name] = true

		if !sm.NeedsFix() {
			if logger != nil {
				logger.V(1).Infof("auto-fix: skipping %s (precondition not met)", sm.Name)
			}
			continue
		}
		if sm.NeedsRoot && geteuidFn() != 0 {
			if logger != nil {
				logger.Warnf("auto-fix: skipping %s (requires root)", sm.Name)
			}
			continue
		}
		if err := sm.Apply(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", sm.Name, err))
			if logger != nil {
				logger.Errorf("auto-fix: %s failed: %v", sm.Name, err)
			}
			continue
		}
		if logger != nil {
			logger.V(0).Infof("auto-fix: applied %s", sm.Name)
		}
	}
	return errs
}
