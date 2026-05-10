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
	osexec "os/exec"
	"sort"
	"strings"

	"sigs.k8s.io/kind/pkg/exec"

	// Strategy label constants live in pkg/cluster/constants (Plan 52-02).
	// pkg/cluster/constants has zero imports so there is no import cycle.
	// Canonical definitions: constants.ResumeStrategyLabel, constants.StrategyIPPinned,
	// constants.StrategyCertRegen.
	"sigs.k8s.io/kind/pkg/cluster/constants"
)

// haResumeStrategyCheck (LIFE-09, phase 52 plan 04) inspects the resume-strategy
// label on every control-plane container in the kinder cluster and reports a
// diagnostic verdict. The check is HA-only — single-CP clusters skip silently.
//
// Verdicts:
//
//	ok         — all CPs labeled "ip-pinned"
//	warn       — all CPs labeled "cert-regen" (slower resume, but consistent)
//	warn       — all CPs have absent/empty label (legacy cluster pre-52)
//	fail       — CPs have mixed labels (ip-pinned on some, cert-regen on others) — corruption
//	skip       — single-CP cluster or no cluster detected
//	warn       — inspect on any CP fails (transient daemon issue)
//	warn       — multiple kinder clusters detected (ambiguous context)
type haResumeStrategyCheck struct {
	// lister returns a map[clusterName][]containerName for kinder CP containers
	// and the detected runtime binary name. Tests inject a fake.
	lister func(binaryName string) (clusters map[string][]string, binary string, err error)
	// inspector reads the resume-strategy label from a single container.
	// Returns (value, present, error). Tests inject a fake.
	inspector func(binaryName, container, labelKey string) (value string, present bool, err error)
}

// newHAResumeStrategyCheck returns the production Check wired to real container
// CLI calls.
func newHAResumeStrategyCheck() Check {
	c := &haResumeStrategyCheck{}
	c.lister = listKinderCPContainersByCluster
	c.inspector = inspectContainerLabel
	return c
}

func (c *haResumeStrategyCheck) Name() string       { return "ha-resume-strategy" }
func (c *haResumeStrategyCheck) Category() string    { return "Cluster" }
func (c *haResumeStrategyCheck) Platforms() []string { return nil }

// Run inspects the resume-strategy label on every CP and returns one Result.
// It deliberately does NOT call ProbeIPAM — this check is pure inspection of
// recorded state, not a runtime probe (DoesNotRunProbe invariant).
func (c *haResumeStrategyCheck) Run() []Result {
	// Detect runtime binary for container CLI calls.
	binary := detectRuntimeBinary()
	if binary == "" {
		// No runtime → no cluster.
		return c.skipResult("no kinder cluster detected")
	}

	clusters, _, err := c.lister(binary)
	if err != nil {
		return c.skipResult("no kinder cluster detected")
	}

	// Verdict 6: skip — no cluster.
	if len(clusters) == 0 {
		return c.skipResult("no kinder cluster detected")
	}

	// Verdict 8: warn — multiple clusters (ambiguous).
	if len(clusters) > 1 {
		clusterNames := sortedKeys(clusters)
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("multiple kinder clusters present (%s)", strings.Join(clusterNames, ", ")),
			Reason:   "check kubectl context to select the correct cluster before running doctor",
		}}
	}

	// Single cluster path.
	var clusterName string
	var cps []string
	for k, v := range clusters {
		clusterName = k
		cps = v
	}
	sort.Strings(cps)

	// Verdict 5: skip — single CP (not HA).
	if len(cps) < 2 {
		return c.skipResult(fmt.Sprintf("non-HA cluster (%d control-plane node)", len(cps)))
	}

	_ = clusterName // used above for display; not needed below

	// Inspect the resume-strategy label on each CP.
	var inspectErrors []string
	// labelValues maps container name → label value ("ip-pinned", "cert-regen", "").
	labelValues := make(map[string]string, len(cps))
	for _, cp := range cps {
		val, _, inspErr := c.inspector(binary, cp, constants.ResumeStrategyLabel)
		if inspErr != nil {
			inspectErrors = append(inspectErrors, fmt.Sprintf("%s: %v", cp, inspErr))
			continue
		}
		labelValues[cp] = val
	}

	// Verdict 7: warn — indeterminate (any inspect error).
	if len(inspectErrors) > 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "resume-strategy: indeterminate",
			Reason: fmt.Sprintf("Could not read resume-strategy label from one or more control-plane nodes: %s. "+
				"Re-run after the runtime daemon recovers.", strings.Join(inspectErrors, "; ")),
		}}
	}

	// Aggregate distinct label values across all CPs.
	distinctValues := map[string][]string{} // value → list of CPs with that value
	for _, cp := range cps {
		v := labelValues[cp]
		distinctValues[v] = append(distinctValues[v], cp)
	}

	// Verdict 4: fail — mixed labels (genuine corruption).
	if len(distinctValues) > 1 {
		parts := make([]string, 0, len(distinctValues))
		for v, nodes := range distinctValues {
			label := v
			if label == "" {
				label = "(absent)"
			}
			parts = append(parts, fmt.Sprintf("%s=%s", strings.Join(nodes, "+"), label))
		}
		sort.Strings(parts)
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "fail",
			Message:  "resume-strategy: mixed",
			Reason: fmt.Sprintf("Control-plane nodes disagree on resume-strategy (%s). "+
				"This indicates manual label tampering or partial migration; cluster state may be inconsistent. "+
				"Delete and recreate.", strings.Join(parts, ", ")),
		}}
	}

	// All CPs have the same label value — determine which verdict applies.
	var uniformValue string
	for v := range distinctValues {
		uniformValue = v
	}

	switch uniformValue {
	case constants.StrategyIPPinned:
		// Verdict 1: ok — all ip-pinned.
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "resume-strategy: ip-pinned",
		}}

	case constants.StrategyCertRegen:
		// Verdict 2: warn — all cert-regen (consistent but slower).
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "resume-strategy: cert-regen",
			Reason: "Cluster will regenerate etcd peer certs on each resume " +
				"(~40-60s overhead). Switch to ip-pinned by deleting and " +
				"recreating the cluster on a runtime that supports IP pinning.",
		}}

	default:
		// Verdict 3: warn — legacy (absent/empty label, cluster predates v2.4).
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  "resume-strategy: cert-regen (legacy)",
			Reason: "Cluster was created before v2.4 IP-pinning " +
				"was available. Delete and recreate to opt into the faster " +
				"ip-pinned resume path.",
		}}
	}
}

// skipResult returns a single skip Result with the given message.
func (c *haResumeStrategyCheck) skipResult(msg string) []Result {
	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "skip",
		Message:  msg,
	}}
}

// listKinderCPContainersByCluster discovers all kinder control-plane containers
// grouped by cluster name. It uses `<binaryName> ps -a` filtered to the kinder
// role label, grouping by the cluster label.
//
// Returns (clusters map[clusterName][]containerName, binaryName, error).
// An empty map with nil error means no kinder cluster was found (skip sentinel).
//
// This is a separate helper from realListCPNodes (resumereadiness.go) because
// haResumeStrategyCheck needs the cluster-grouping dimension to detect the
// multi-cluster case (Verdict 8). realListCPNodes flattens all CPs into a single
// slice and assumes a single cluster.
func listKinderCPContainersByCluster(binaryName string) (map[string][]string, string, error) {
	if binaryName == "" {
		for _, rt := range []string{"docker", "podman", "nerdctl"} {
			if _, err := osexec.LookPath(rt); err == nil {
				binaryName = rt
				break
			}
		}
	}
	if binaryName == "" {
		return map[string][]string{}, "", nil
	}

	// List ALL kind cluster containers (including stopped), filtering CP role.
	// Format: "name\tclusterLabel\troleLabel"
	lines, err := exec.OutputLines(exec.Command(
		binaryName,
		"ps", "-a",
		"--filter", "label=io.x-k8s.kind.role=control-plane",
		"--format", `{{.Names}}	{{.Label "io.x-k8s.kind.cluster"}}`,
	))
	if err != nil {
		// Treat runtime errors as "no cluster found" (best-effort).
		return map[string][]string{}, binaryName, nil
	}

	clusters := map[string][]string{}
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), "\t", 2)
		if len(parts) != 2 {
			continue
		}
		name, cluster := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if name == "" || cluster == "" {
			continue
		}
		clusters[cluster] = append(clusters[cluster], name)
	}
	return clusters, binaryName, nil
}

// inspectContainerLabel reads the value of a single Docker/Podman label from
// the named container. Returns (value, present, error).
//
// present=false means the label key does not exist on the container (distinct
// from present=true with value=""). For resume-strategy, absent label = legacy
// cluster (Verdict 3).
func inspectContainerLabel(binaryName, container, labelKey string) (string, bool, error) {
	// Use two separate inspect calls: one for value, one for presence check.
	// A single-pass format using conditional templates is fragile across
	// docker/podman/nerdctl versions.
	lines, err := exec.OutputLines(exec.Command(
		binaryName,
		"inspect",
		"--format", fmt.Sprintf(`{{index .Config.Labels %q}}`, labelKey),
		container,
	))
	if err != nil {
		return "", false, err
	}
	value := strings.TrimSpace(strings.Join(lines, ""))

	// Check label presence: inspect the full label map and see if the key exists.
	// We use a separate format that outputs "__ABSENT__" only when the key is missing.
	presenceLines, err := exec.OutputLines(exec.Command(
		binaryName,
		"inspect",
		"--format", fmt.Sprintf(`{{if (index .Config.Labels %q)}}PRESENT{{else}}ABSENT{{end}}`, labelKey),
		container,
	))
	if err != nil {
		return value, value != "", nil
	}
	presence := strings.TrimSpace(strings.Join(presenceLines, ""))
	return value, presence == "PRESENT", nil
}

// sortedKeys returns the keys of a string map in sorted order.
func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
