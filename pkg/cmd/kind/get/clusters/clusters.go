/*
Copyright 2018 The Kubernetes Authors.

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

// Package clusters implements the `clusters` command
package clusters

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/runtime"
)

type flagpole struct {
	Output string
}

// clusterInfo holds per-cluster display data for both tabwriter and JSON output.
//
// NOTE: This struct replaces the previous bare []string JSON shape for
// `kinder get clusters --output json`. The schema migration is intentional and
// is documented in .planning/phases/47-cluster-pause-resume/47-CONTEXT.md
// (Cluster list integration: add Status column).
type clusterInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// NewCommand returns a new cobra.Command for getting the list of clusters
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	c := &cobra.Command{
		Args: cobra.NoArgs,
		// TODO(bentheelder): more detailed usage
		Use:   "clusters",
		Short: "Lists existing kind clusters by their name",
		Long:  "Lists existing kind clusters by their name and current status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runE(logger, streams, flags)
		},
	}
	c.Flags().StringVar(&flags.Output, "output", "", "output format; supported values: \"\", \"json\"")
	return c
}

func runE(logger log.Logger, streams cmd.IOStreams, flags *flagpole) error {
	switch flags.Output {
	case "", "json":
		// valid
	default:
		return fmt.Errorf("unsupported output format %q; supported values: \"\", \"json\"", flags.Output)
	}

	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)
	names, err := provider.List()
	if err != nil {
		return err
	}

	binaryName := lifecycle.ProviderBinaryName()
	infos := collectClusterInfos(provider, binaryName, names)

	if flags.Output == "json" {
		return renderJSON(streams.Out, infos)
	}

	if len(infos) == 0 {
		logger.V(0).Info("No kind clusters found.")
		return nil
	}
	return renderText(streams.Out, infos)
}

// collectClusterInfos turns a list of cluster names into clusterInfo entries
// by querying each cluster's container states. If the runtime binary is not
// available the status defaults to "Error" so the user is alerted; this matches
// the lifecycle.ClusterStatus error semantics.
func collectClusterInfos(provider *cluster.Provider, binaryName string, names []string) []clusterInfo {
	infos := make([]clusterInfo, 0, len(names))
	for _, name := range names {
		status := "Error"
		if binaryName != "" {
			nodes, err := provider.ListNodes(name)
			if err == nil {
				status = lifecycle.ClusterStatus(binaryName, nodes)
			}
		}
		infos = append(infos, clusterInfo{Name: name, Status: status})
	}
	return infos
}

// renderJSON encodes the cluster info slice as a JSON array of {name, status}
// objects. Always emits `[]` instead of `null` for empty input to match the
// pre-migration "no nulls" convention.
func renderJSON(w io.Writer, infos []clusterInfo) error {
	if infos == nil {
		infos = []clusterInfo{}
	}
	return json.NewEncoder(w).Encode(infos)
}

// renderText writes a tabwriter table with NAME and STATUS columns to w.
func renderText(w io.Writer, infos []clusterInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tSTATUS"); err != nil {
		return err
	}
	for _, c := range infos {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", c.Name, c.Status); err != nil {
			return err
		}
	}
	return tw.Flush()
}
