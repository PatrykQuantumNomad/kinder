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

// Package nodes implements the `nodes` command
package nodes

import (
	"encoding/json"
	"fmt"
	"os"
	osexec "os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cluster/nodeutils"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
)

type flagpole struct {
	Name        string
	AllClusters bool
	Output      string
}

// nodeInfo holds per-node display data for both table and JSON output.
type nodeInfo struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
	Version string `json:"version"`
	Image  string `json:"image"`
	Skew   string `json:"skew"`
	SkewOK bool   `json:"skewOk"`
}

// resolveClusterName is the test-injection point for cluster name resolution.
// Production code wires it to lifecycle.ResolveClusterName backed by a real
// *cluster.Provider; tests substitute a closure that returns a fixed name.
var resolveClusterName = func(args []string, p *cluster.Provider) (string, error) {
	return lifecycle.ResolveClusterName(args, p)
}

// listNodes is the test-injection point for provider.ListNodes. When non-nil,
// it is called instead of provider.ListNodes(name) — tests use this to capture
// which cluster name was resolved without spinning a real cluster.
// When nil (default), production code calls provider.ListNodes(name) directly.
var listNodes func(name string) ([]nodes.Node, error)

// NewCommand returns a new cobra.Command for getting the list of nodes for a given cluster
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	cmd := &cobra.Command{
		Args:  cobra.MaximumNArgs(1),
		Use:   "nodes [cluster-name]",
		Short: "Lists existing kind nodes by their name",
		Long: "Lists existing kind nodes by their name. If no cluster name is given and exactly\n" +
			"one cluster exists, it is auto-selected (matches kinder pause/resume convention).",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runE(logger, streams, flags, args)
		},
	}
	cmd.Flags().StringVarP(
		&flags.Name,
		"name",
		"n",
		cluster.DefaultName,
		"the cluster context name",
	)
	cmd.Flags().BoolVarP(
		&flags.AllClusters,
		"all-clusters",
		"A",
		false,
		"If present, list all the available nodes across all cluster contexts. Current context is ignored even if specified with --name.",
	)
	cmd.Flags().StringVar(
		&flags.Output,
		"output",
		"",
		`output format; supported values: "", "json"`,
	)
	return cmd
}

func runE(logger log.Logger, streams cmd.IOStreams, flags *flagpole, args []string) error {
	// Validate output format
	if flags.Output != "" && flags.Output != "json" {
		return fmt.Errorf("unsupported output format %q: supported values are \"\", \"json\"", flags.Output)
	}

	// List nodes by cluster context name
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)

	var allNodes []nodes.Node
	var err error
	if flags.AllClusters {
		// --all-clusters takes precedence over both positional arg and --name.
		clusters, err := provider.List()
		if err != nil {
			return err
		}
		for _, clusterName := range clusters {
			clusterNodes, err := provider.ListNodes(clusterName)
			if err != nil {
				return err
			}
			allNodes = append(allNodes, clusterNodes...)
		}
	} else {
		// Resolve the target cluster: positional arg > --name > auto-detect.
		targetName, resolveErr := resolveClusterName(args, provider)
		if resolveErr != nil {
			return resolveErr
		}
		if listNodes != nil {
			// Test injection path: captures which name was resolved.
			allNodes, err = listNodes(targetName)
		} else {
			allNodes, err = provider.ListNodes(targetName)
		}
		if err != nil {
			return err
		}
	}

	// Collect nodeInfo for all nodes.
	infos := collectNodeInfos(allNodes)

	// JSON output branch — before human-readable empty-node checks
	if flags.Output == "json" {
		return json.NewEncoder(streams.Out).Encode(infos)
	}

	// Human-readable output with empty-node messages
	if flags.AllClusters {
		if len(allNodes) == 0 {
			logger.V(0).Infof("No kind nodes for any cluster.")
			return nil
		}
	} else {
		if len(allNodes) == 0 {
			logger.V(0).Infof("No kind nodes found for cluster %q.", flags.Name)
			return nil
		}
	}

	// Tabwriter output: NAME, ROLE, STATUS, VERSION, IMAGE, SKEW
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tROLE\tSTATUS\tVERSION\tIMAGE\tSKEW") //nolint:errcheck
	for _, info := range infos {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", //nolint:errcheck
			info.Name, info.Role, info.Status, info.Version, info.Image, info.Skew)
	}
	w.Flush() //nolint:errcheck
	return nil
}

// collectNodeInfos gathers extended information for every node.
// It determines the control-plane minor version first so the SKEW column
// can be computed relative to CP.
func collectNodeInfos(allNodes []nodes.Node) []nodeInfo {
	if len(allNodes) == 0 {
		return []nodeInfo{}
	}

	// First pass: collect roles, versions, container states.
	type raw struct {
		name    string
		role    string
		version string
		image   string
		state   string // container runtime state (e.g. "running", "exited")
	}
	// Detect container runtime for image inspect.
	var binaryName string
	for _, rt := range []string{"docker", "podman", "nerdctl"} {
		if _, err := osexec.LookPath(rt); err == nil {
			binaryName = rt
			break
		}
	}

	raws := make([]raw, 0, len(allNodes))
	for _, n := range allNodes {
		role, err := n.Role()
		if err != nil {
			role = "unknown"
		}
		ver, err := nodeutils.KubeVersion(n)
		if err != nil {
			ver = "unknown"
		}
		// Get container image via runtime inspect.
		var image string
		if binaryName != "" {
			lines, inspectErr := exec.OutputLines(exec.Command(
				binaryName, "inspect",
				"--format", "{{.Config.Image}}",
				n.String(),
			))
			if inspectErr == nil && len(lines) > 0 {
				image = strings.TrimSpace(lines[0])
			}
		}
		// Get container runtime state via shared lifecycle helper.
		// Replaces the previous hardcoded `status := "Ready"` literal.
		var state string
		if binaryName != "" {
			if s, err := lifecycle.ContainerState(binaryName, n.String()); err == nil {
				state = s
			}
		}
		raws = append(raws, raw{
			name:    n.String(),
			role:    role,
			version: ver,
			image:   image,
			state:   state,
		})
	}

	// Determine CP minor version (use first CP node found).
	var cpMinor uint
	cpMinorSet := false
	for _, r := range raws {
		if r.role == "control-plane" && r.version != "unknown" {
			m, ok := parseMinor(r.version)
			if ok {
				cpMinor = m
				cpMinorSet = true
				break
			}
		}
	}

	// Second pass: build nodeInfo with skew.
	infos := make([]nodeInfo, 0, len(raws))
	for _, r := range raws {
		// Map container runtime state to a user-facing Status value.
		// "running" → "Ready" (the container is up; existing column semantics)
		// "exited"/"created" → "Stopped" (kinder pause exits containers)
		// "paused" → "Paused" (docker freeze; rare but distinct)
		// anything else (or empty after error) → "Unknown"
		var status string
		switch r.state {
		case "running":
			status = "Ready"
		case "exited", "created":
			status = "Stopped"
		case "paused":
			status = "Paused"
		default:
			status = "Unknown"
		}
		skewDisplay := ""
		skewOK := true

		if cpMinorSet && r.version != "unknown" {
			nodeMinor, ok := parseMinor(r.version)
			if ok {
				skewDisplay, skewOK = ComputeSkew(cpMinor, nodeMinor)
			}
		}
		if skewDisplay == "" {
			skewDisplay = "n/a"
		}

		infos = append(infos, nodeInfo{
			Name:    r.name,
			Role:    r.role,
			Status:  status,
			Version: r.version,
			Image:   r.image,
			Skew:    skewDisplay,
			SkewOK:  skewOK,
		})
	}
	return infos
}

// ComputeSkew returns a display string and whether the skew is within policy.
// Policy: nodes may be at most 3 minor versions behind the control-plane.
// Nodes ahead of the CP are always a policy violation.
//
// cpMinor is the control-plane minor version; nodeMinor is the node's minor version.
// Returns:
//   - ("✓", true)  when nodeMinor == cpMinor (exact match)
//   - ("✗ (-N)", true)  when nodeMinor is N minors behind and N <= 3 (within policy)
//   - ("✗ (-N)", false) when nodeMinor is N minors behind and N > 3 (policy violation)
//   - ("✗ (+N)", false) when nodeMinor is N minors ahead (always a violation)
func ComputeSkew(cpMinor uint, nodeMinor uint) (string, bool) {
	if nodeMinor == cpMinor {
		return "\u2713", true // ✓
	}
	diff := int(nodeMinor) - int(cpMinor)
	if diff < 0 {
		// Node is behind CP.
		behind := -diff
		ok := behind <= 3
		return fmt.Sprintf("\u2717 (-%d)", behind), ok // ✗ (-N)
	}
	// Node is ahead of CP — always a violation.
	return fmt.Sprintf("\u2717 (+%d)", diff), false // ✗ (+N)
}

// parseMinor extracts the minor version component from a version string like "v1.31.2".
// Returns (minor, true) on success, (0, false) on parse failure.
func parseMinor(ver string) (uint, bool) {
	// Strip leading "v"
	s := ver
	if len(s) > 0 && s[0] == 'v' {
		s = s[1:]
	}
	// Find first dot
	dot1 := -1
	for i, c := range s {
		if c == '.' {
			dot1 = i
			break
		}
	}
	if dot1 < 0 {
		return 0, false
	}
	// Find second dot
	rest := s[dot1+1:]
	dot2 := -1
	for i, c := range rest {
		if c == '.' {
			dot2 = i
			break
		}
	}
	var minorStr string
	if dot2 < 0 {
		minorStr = rest
	} else {
		minorStr = rest[:dot2]
	}
	var minor uint
	for _, c := range minorStr {
		if c < '0' || c > '9' {
			return 0, false
		}
		minor = minor*10 + uint(c-'0')
	}
	return minor, len(minorStr) > 0
}
