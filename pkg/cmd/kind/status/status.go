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

// Package status implements the `kinder status [cluster-name]` command, which
// reports per-node container state, role and Kubernetes version for a single
// kinder cluster. It is read-only and shares its container-state queries with
// pause/resume via pkg/cluster/internal/lifecycle.
package status

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/exec"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
)

type flagpole struct {
	Output string
}

// nodeStatusInfo holds per-node display data for both tabwriter and JSON output.
type nodeStatusInfo struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	State   string `json:"state"`
	Version string `json:"version"`
}

// statusOutput is the top-level JSON shape: cluster name plus an array of nodes.
type statusOutput struct {
	Cluster string           `json:"cluster"`
	Nodes   []nodeStatusInfo `json:"nodes"`
}

// NewCommand returns a new cobra.Command for `kinder status [cluster-name]`.
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	c := &cobra.Command{
		Args:  cobra.MaximumNArgs(1),
		Use:   "status [cluster-name]",
		Short: "Reports per-node container state for a kinder cluster",
		Long: "Reports per-node container state, role and Kubernetes version for a kinder cluster.\n" +
			"If no cluster name is given and exactly one cluster exists, it is auto-selected.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runE(logger, streams, flags, args)
		},
	}
	c.Flags().StringVar(
		&flags.Output,
		"output",
		"",
		`output format; supported values: "", "json"`,
	)
	return c
}

func runE(logger log.Logger, streams cmd.IOStreams, flags *flagpole, args []string) error {
	if flags.Output != "" && flags.Output != "json" {
		return fmt.Errorf("unsupported output format %q: supported values are \"\", \"json\"", flags.Output)
	}

	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)

	clusterName, err := lifecycle.ResolveClusterName(args, provider)
	if err != nil {
		return err
	}

	allNodes, err := provider.ListNodes(clusterName)
	if err != nil {
		return err
	}
	if len(allNodes) == 0 {
		return fmt.Errorf("no nodes found for cluster %q", clusterName)
	}

	binaryName := lifecycle.ProviderBinaryName()
	infos := collectNodeStatus(binaryName, allNodes)

	if flags.Output == "json" {
		return renderJSON(streams.Out, clusterName, infos)
	}
	return renderText(streams.Out, infos)
}

// collectNodeStatus reads role, container state, and live Kubernetes version
// for every node. Best-effort: failures leave individual fields empty rather
// than aborting the whole command. This matches the existing `kinder get nodes`
// behavior for unknown role/version values.
func collectNodeStatus(binaryName string, allNodes []nodes.Node) []nodeStatusInfo {
	infos := make([]nodeStatusInfo, 0, len(allNodes))
	for _, n := range allNodes {
		role, err := n.Role()
		if err != nil || role == "" {
			role = "unknown"
		}

		state := ""
		if binaryName != "" {
			s, sErr := lifecycle.ContainerState(binaryName, n.String())
			if sErr == nil {
				state = s
			}
		}

		version := ""
		if binaryName != "" {
			// Mirror pkg/internal/doctor/clusterskew.go realListNodes: read
			// /kind/version inside the container. Tolerate failures (paused
			// nodes will fail this exec and show empty version).
			lines, vErr := exec.OutputLines(exec.Command(
				binaryName, "exec", n.String(), "cat", "/kind/version",
			))
			if vErr == nil && len(lines) > 0 {
				version = strings.TrimSpace(lines[0])
			}
		}

		infos = append(infos, nodeStatusInfo{
			Name:    n.String(),
			Role:    role,
			State:   state,
			Version: version,
		})
	}
	return infos
}

// renderJSON encodes a statusOutput object to w.
func renderJSON(w io.Writer, clusterName string, infos []nodeStatusInfo) error {
	if infos == nil {
		infos = []nodeStatusInfo{}
	}
	return json.NewEncoder(w).Encode(statusOutput{
		Cluster: clusterName,
		Nodes:   infos,
	})
}

// renderText writes a tabwriter table with NAME, ROLE, STATE, K8S-VERSION columns to w.
func renderText(w io.Writer, infos []nodeStatusInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tROLE\tSTATE\tK8S-VERSION"); err != nil {
		return err
	}
	for _, n := range infos {
		state := n.State
		if state == "" {
			state = "unknown"
		}
		version := n.Version
		if version == "" {
			version = "-"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", n.Name, n.Role, state, version); err != nil {
			return err
		}
	}
	return tw.Flush()
}
