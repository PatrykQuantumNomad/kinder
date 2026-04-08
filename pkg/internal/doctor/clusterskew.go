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
	"bytes"
	"fmt"
	osexec "os/exec"
	"strings"
	"text/tabwriter"

	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/version"
)

// nodeEntry holds per-node information collected from a running cluster.
type nodeEntry struct {
	Name       string
	Role       string
	Version    string // live version read from /kind/version, e.g. "v1.31.2"
	Image      string // container image, e.g. "kindest/node:v1.31.2"
	VersionErr error  // non-nil if the live version could not be read
}

// clusterNodeSkewCheck detects version-skew violations and config drift
// across nodes in a running kind cluster.
type clusterNodeSkewCheck struct {
	// listNodes is injected for testing; the real implementation calls
	// the cluster provider and container runtime.
	listNodes func() ([]nodeEntry, error)
}

// newClusterNodeSkewCheck creates a real clusterNodeSkewCheck that detects
// the active container runtime, lists cluster nodes, and reads their versions.
// This is the constructor used by the global allChecks registry.
func newClusterNodeSkewCheck() Check {
	return &clusterNodeSkewCheck{
		listNodes: realListNodes,
	}
}

// realListNodes discovers nodes from the default kind cluster and collects
// live version information using low-level container CLI commands.
// It avoids importing the cluster package (which would create an import cycle
// since cluster/internal/create imports doctor).
func realListNodes() ([]nodeEntry, error) {
	// Detect container runtime binary.
	var binaryName string
	for _, rt := range []string{"docker", "podman", "nerdctl"} {
		if _, err := osexec.LookPath(rt); err == nil {
			binaryName = rt
			break
		}
	}
	if binaryName == "" {
		return nil, nil // no runtime → skip
	}

	// List kind cluster containers by label.
	// The format outputs "name|role|image" per container.
	lines, err := exec.OutputLines(exec.Command(
		binaryName, "ps",
		"--filter", "label=io.x-k8s.kind.cluster=kind",
		"--format", `{{.Names}}`,
	))
	if err != nil || len(lines) == 0 {
		return nil, nil // no cluster running → skip
	}

	entries := make([]nodeEntry, 0, len(lines))
	for _, name := range lines {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// Get role and image via inspect.
		var role, image string
		inspectLines, inspectErr := exec.OutputLines(exec.Command(
			binaryName, "inspect",
			"--format", `{{ index .Config.Labels "io.x-k8s.kind.role" }}|{{.Config.Image}}`,
			name,
		))
		if inspectErr == nil && len(inspectLines) > 0 {
			parts := strings.SplitN(inspectLines[0], "|", 2)
			if len(parts) == 2 {
				role = parts[0]
				image = parts[1]
			}
		}

		// Get live Kubernetes version from /kind/version inside the container.
		var ver string
		var verErr error
		verLines, execErr := exec.OutputLines(exec.Command(
			binaryName, "exec", name, "cat", "/kind/version",
		))
		if execErr != nil {
			verErr = execErr
		} else if len(verLines) > 0 {
			ver = strings.TrimSpace(verLines[0])
		}

		entries = append(entries, nodeEntry{
			Name:       name,
			Role:       role,
			Version:    ver,
			Image:      image,
			VersionErr: verErr,
		})
	}
	return entries, nil
}

func (c *clusterNodeSkewCheck) Name() string       { return "cluster-node-skew" }
func (c *clusterNodeSkewCheck) Category() string    { return "Cluster" }
func (c *clusterNodeSkewCheck) Platforms() []string { return nil }

// skewViolation describes a single version-skew or drift problem found on a node.
type skewViolation struct {
	node   string
	detail string
}

// Run executes the check:
//  1. Lists cluster nodes; if none found → skip.
//  2. Checks that all control-plane nodes share the same minor version.
//  3. Checks that every worker node is within 3 minor versions of the CP.
//  4. Checks for config drift (image tag version ≠ live /kind/version output).
func (c *clusterNodeSkewCheck) Run() []Result {
	entries, err := c.listNodes()
	if err != nil {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "warn",
			Message:  fmt.Sprintf("Could not list cluster nodes: %v", err),
		}}
	}
	if len(entries) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "skip",
			Message:  "(no cluster found)",
		}}
	}

	var violations []skewViolation

	// Collect control-plane entries and read-errors.
	var cpEntries []nodeEntry
	for _, e := range entries {
		if e.VersionErr != nil {
			violations = append(violations, skewViolation{
				node:   e.Name,
				detail: fmt.Sprintf("could not read version: %v", e.VersionErr),
			})
			continue
		}
		if e.Role == "control-plane" {
			cpEntries = append(cpEntries, e)
		}
	}

	// Determine dominant CP minor version (most common, or first on tie).
	cpMinor, cpVersion := dominantCPMinor(cpEntries)

	// Validate HA control-plane consistency: all CP nodes should share the same minor.
	if len(cpEntries) > 1 {
		for _, e := range cpEntries {
			if e.VersionErr != nil {
				continue
			}
			v, parseErr := version.ParseSemantic(e.Version)
			if parseErr != nil {
				violations = append(violations, skewViolation{
					node:   e.Name,
					detail: fmt.Sprintf("unparseable version %q: %v", e.Version, parseErr),
				})
				continue
			}
			if v.Minor() != cpMinor {
				violations = append(violations, skewViolation{
					node:   e.Name,
					detail: fmt.Sprintf("control-plane minor mismatch: v1.%d (majority is v1.%d)", v.Minor(), cpMinor),
				})
			}
		}
	}

	// Validate workers: must be within 3 minor versions of CP.
	if cpVersion != "" {
		for _, e := range entries {
			if e.Role != "worker" || e.VersionErr != nil {
				continue
			}
			wv, parseErr := version.ParseSemantic(e.Version)
			if parseErr != nil {
				violations = append(violations, skewViolation{
					node:   e.Name,
					detail: fmt.Sprintf("unparseable version %q: %v", e.Version, parseErr),
				})
				continue
			}
			diff := int(cpMinor) - int(wv.Minor())
			if diff < 0 {
				diff = -diff
			}
			if diff > 3 {
				violations = append(violations, skewViolation{
					node:   e.Name,
					detail: fmt.Sprintf("worker skew %d minors (worker v1.%d, cp v1.%d) exceeds allowed 3", diff, wv.Minor(), cpMinor),
				})
			}
		}
	}

	// Config drift detection: image tag version vs live version.
	for _, e := range entries {
		if e.VersionErr != nil || e.Version == "" || e.Image == "" {
			continue
		}
		tagVer, tagErr := imageTagVersion(e.Image)
		if tagErr != nil {
			// Can't determine image tag version — not a drift violation.
			continue
		}
		tv, parseErr := version.ParseSemantic(tagVer)
		if parseErr != nil {
			continue
		}
		lv, parseErr := version.ParseSemantic(e.Version)
		if parseErr != nil {
			continue
		}
		if tv.Minor() != lv.Minor() || tv.Major() != lv.Major() {
			violations = append(violations, skewViolation{
				node:   e.Name,
				detail: fmt.Sprintf("config drift: image tag %s but live version %s", tagVer, e.Version),
			})
		}
	}

	if len(violations) == 0 {
		return []Result{{
			Name:     c.Name(),
			Category: c.Category(),
			Status:   "ok",
			Message:  "All cluster nodes within version-skew policy",
		}}
	}

	// Format violations as a table.
	msg := formatSkewTable(violations)
	return []Result{{
		Name:     c.Name(),
		Category: c.Category(),
		Status:   "warn",
		Message:  msg,
		Reason:   "Cluster version skew or config drift detected",
		Fix:      "Review node versions and upgrade nodes to bring cluster within skew policy",
	}}
}

// dominantCPMinor returns the minor version shared by the most control-plane
// nodes and the corresponding version string.  If cpEntries is empty, returns (0, "").
func dominantCPMinor(cpEntries []nodeEntry) (uint, string) {
	counts := make(map[uint]int)
	repr := make(map[uint]string)
	for _, e := range cpEntries {
		if e.VersionErr != nil {
			continue
		}
		v, err := version.ParseSemantic(e.Version)
		if err != nil {
			continue
		}
		m := v.Minor()
		counts[m]++
		if _, seen := repr[m]; !seen {
			repr[m] = e.Version
		}
	}
	var dominant uint
	var best int
	var dominantVer string
	for m, cnt := range counts {
		if cnt > best || (cnt == best && m > dominant) {
			best = cnt
			dominant = m
			dominantVer = repr[m]
		}
	}
	return dominant, dominantVer
}

// formatSkewTable renders violations as a tab-aligned table suitable for the
// Result.Message field. Uses the same tabwriter style as create-time checks.
func formatSkewTable(violations []skewViolation) string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NODE\tDETAIL")
	fmt.Fprintln(w, "----\t------")
	for _, v := range violations {
		fmt.Fprintf(w, "%s\t%s\n", v.node, v.detail)
	}
	w.Flush() //nolint:errcheck
	return strings.TrimRight(buf.String(), "\n")
}

// imageTagVersion extracts the Kubernetes version from a container image reference.
// It recognises the common patterns:
//
//	kindest/node:v1.31.2
//	registry.k8s.io/kindest/node:v1.31.2@sha256:...
//
// Returns an error if no valid semver-like tag (starting with "v1.") is found.
func imageTagVersion(image string) (string, error) {
	if image == "" {
		return "", fmt.Errorf("empty image string")
	}
	// Strip digest suffix (e.g. @sha256:...).
	if idx := strings.Index(image, "@"); idx >= 0 {
		image = image[:idx]
	}
	// Find tag after last ":".
	idx := strings.LastIndex(image, ":")
	if idx < 0 {
		return "", fmt.Errorf("no tag found in image %q", image)
	}
	tag := image[idx+1:]
	if !strings.HasPrefix(tag, "v") {
		return "", fmt.Errorf("tag %q does not look like a Kubernetes version", tag)
	}
	// Validate it parses as semver.
	if _, err := version.ParseSemantic(tag); err != nil {
		return "", fmt.Errorf("tag %q is not a valid semantic version: %w", tag, err)
	}
	return tag, nil
}
