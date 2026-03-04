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

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cluster/nodes"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
)

type flagpole struct {
	Name        string
	AllClusters bool
	Output      string
}

type nodeInfo struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// NewCommand returns a new cobra.Command for getting the list of nodes for a given cluster
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "nodes",
		Short: "Lists existing kind nodes by their name",
		Long:  "Lists existing kind nodes by their name",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runE(logger, streams, flags)
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

func runE(logger log.Logger, streams cmd.IOStreams, flags *flagpole) error {
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
		allNodes, err = provider.ListNodes(flags.Name)
		if err != nil {
			return err
		}
	}

	// JSON output branch — before human-readable empty-node checks
	if flags.Output == "json" {
		infos := make([]nodeInfo, 0, len(allNodes))
		for _, n := range allNodes {
			role, err := n.Role()
			if err != nil {
				role = "unknown"
			}
			infos = append(infos, nodeInfo{Name: n.String(), Role: role})
		}
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

	for _, node := range allNodes {
		fmt.Fprintln(streams.Out, node.String()) //nolint:errcheck
	}
	return nil
}
