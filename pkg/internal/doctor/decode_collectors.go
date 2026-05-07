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
	"strings"
	"time"

	"sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"
)

// execCommand is the injectable command factory.  Production code uses
// exec.Command (the project's wrapper); tests swap this to a fake.
// This mirrors the lifecycle.defaultCmder pattern used throughout the project.
var execCommand = exec.Command

// dockerLogsFn is the injectable collector for docker logs.
// Default points to realDockerLogs; tests swap via t.Cleanup.
var dockerLogsFn = realDockerLogs

// k8sEventsFn is the injectable collector for kubectl get events.
// Default points to realK8sEvents; tests swap via t.Cleanup.
var k8sEventsFn = realK8sEvents

// realDockerLogs invokes `<binaryName> logs --since <since> <nodeName>` and
// returns stdout split into lines.
func realDockerLogs(binaryName, nodeName, since string) ([]string, error) {
	return exec.OutputLines(execCommand(binaryName, "logs", "--since", since, nodeName))
}

// realK8sEvents invokes kubectl get events on a control-plane node via
// `docker exec <cpNodeName> kubectl ...` and returns stdout split into lines.
//
// Locked decision #3: default filter is type!=Normal (Warnings only).
// Set includeNormal=true to omit the --field-selector pair.
//
// Locked decision #2 (RESEARCH pitfall 4): kubectl does NOT support --since.
// When since is non-zero and non-"0s", returned lines are post-filtered
// client-side by filterEventsByAge so only events within the window are kept.
func realK8sEvents(binaryName, cpNodeName, since string, includeNormal bool) ([]string, error) {
	args := []string{
		"exec", cpNodeName,
		"kubectl", "--kubeconfig=/etc/kubernetes/admin.conf",
		"get", "events", "--all-namespaces",
		"--sort-by=.lastTimestamp",
	}
	if !includeNormal {
		args = append(args, "--field-selector", "type!=Normal")
	}
	lines, err := exec.OutputLines(execCommand(binaryName, args...))
	if err != nil {
		return nil, err
	}
	if since == "" || since == "0s" {
		return lines, nil
	}
	return filterEventsByAge(lines, since), nil
}

// filterEventsByAge keeps the header row plus any data row whose LAST SEEN
// column parses as a duration <= window.  Rows with unparseable LAST SEEN
// values pass through unchanged (better to over-include than silently drop).
//
// Header row is detected by strings.HasPrefix(line, "LAST SEEN") (kubectl
// tabular default output).
func filterEventsByAge(lines []string, since string) []string {
	window, err := time.ParseDuration(since)
	if err != nil || window <= 0 {
		// Cannot parse the window — return all lines unchanged.
		return lines
	}
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "LAST SEEN") {
			// Header row always passes through.
			filtered = append(filtered, line)
			continue
		}
		// The LAST SEEN column is the first whitespace-separated field.
		fields := strings.Fields(line)
		if len(fields) == 0 {
			filtered = append(filtered, line)
			continue
		}
		age, parseErr := time.ParseDuration(fields[0])
		if parseErr != nil {
			// Unparseable LAST SEEN — pass through.
			filtered = append(filtered, line)
			continue
		}
		if age <= window {
			filtered = append(filtered, line)
		}
		// If age > window the row is too old — silently drop it.
	}
	return filtered
}

// DecodeOptions configures a RunDecode invocation.
type DecodeOptions struct {
	// Cluster is the resolved cluster name (display only).
	Cluster string
	// BinaryName is the container runtime binary ("docker" / "podman" / "nerdctl").
	BinaryName string
	// CPNodeName is the first control-plane node for kubectl exec.
	CPNodeName string
	// AllNodes contains all node container names for the docker logs scan.
	AllNodes []string
	// Since is the time window applied to BOTH docker logs (via --since) and
	// kubectl events (via client-side LAST SEEN filter). Locked decision #2.
	// The CLI layer (Plan 50-03) should default this to 30 * time.Minute.
	Since time.Duration
	// IncludeNormalEvents flips the kubectl events filter off when true.
	// Default false → Warnings only (type!=Normal). Locked decision #3.
	IncludeNormalEvents bool
	// Logger is optional; nil-safe. V(1) messages are emitted for per-node errors.
	Logger log.Logger
}

// RunDecode collects docker logs from every node and kubectl events from the
// CP node, matches both streams against Catalog, and returns the aggregated
// DecodeResult.
//
// Failures from individual sources (e.g. stopped containers, unavailable API)
// are logged at V(1) and skipped; the scan continues on all remaining sources.
// RunDecode only returns an error for caller misuse (empty inputs).
func RunDecode(opts DecodeOptions) (*DecodeResult, error) {
	if len(opts.AllNodes) == 0 && opts.CPNodeName == "" {
		return nil, errors.New("RunDecode: AllNodes and CPNodeName both empty; nothing to scan")
	}

	sinceStr := opts.Since.String() // "30m0s", "1h0m0s", "0s" when zero

	result := &DecodeResult{Cluster: opts.Cluster}
	var totalLines int

	// Per-node docker logs.
	for _, node := range opts.AllNodes {
		lines, err := dockerLogsFn(opts.BinaryName, node, sinceStr)
		if err != nil {
			if opts.Logger != nil {
				opts.Logger.V(1).Infof("decode: skipping docker logs for %q: %v", node, err)
			}
			continue
		}
		totalLines += len(lines)
		result.Matches = append(result.Matches,
			matchLines(lines, Catalog, "docker-logs:"+node)...)
	}

	// kubectl events from the control-plane node.
	if opts.CPNodeName != "" {
		lines, err := k8sEventsFn(opts.BinaryName, opts.CPNodeName, sinceStr, opts.IncludeNormalEvents)
		if err != nil {
			if opts.Logger != nil {
				opts.Logger.V(1).Infof("decode: skipping kubectl events: %v", err)
			}
		} else {
			totalLines += len(lines)
			result.Matches = append(result.Matches,
				matchLines(lines, Catalog, "k8s-events")...)
		}
	}

	result.Unmatched = totalLines - len(result.Matches)
	if result.Unmatched < 0 {
		result.Unmatched = 0 // defensive: matchLines cannot produce more matches than lines
	}

	return result, nil
}
