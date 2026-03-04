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

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/runtime"
)

type flagpole struct {
	Output string
}

// NewCommand returns a new cobra.Command for getting the list of clusters
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	c := &cobra.Command{
		Args: cobra.NoArgs,
		// TODO(bentheelder): more detailed usage
		Use:   "clusters",
		Short: "Lists existing kind clusters by their name",
		Long:  "Lists existing kind clusters by their name",
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
	clusters, err := provider.List()
	if err != nil {
		return err
	}

	if flags.Output == "json" {
		if clusters == nil {
			clusters = []string{} // avoid JSON null, emit []
		}
		return json.NewEncoder(streams.Out).Encode(clusters)
	}

	if len(clusters) == 0 {
		logger.V(0).Info("No kind clusters found.")
		return nil
	}
	for _, cluster := range clusters {
		fmt.Fprintln(streams.Out, cluster) //nolint:errcheck
	}
	return nil
}
