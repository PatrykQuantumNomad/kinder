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

// Package env implements the `env` command
package env

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/log"
)

type flagpole struct {
	Name string
}

// NewCommand returns a new cobra.Command for the env subcommand
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "env",
		Short: "Prints cluster environment variables in eval-safe key=value format",
		Long:  "Prints KINDER_PROVIDER, KIND_CLUSTER_NAME, and KUBECONFIG as key=value lines suitable for eval",
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
	return cmd
}

func runE(logger log.Logger, streams cmd.IOStreams, flags *flagpole) error {
	providerName := activeProviderName(logger)
	clusterName := flags.Name
	kubeconfigPath := activeKubeconfigPath()

	fmt.Fprintf(streams.Out, "KINDER_PROVIDER=%s\n", providerName)  //nolint:errcheck
	fmt.Fprintf(streams.Out, "KIND_CLUSTER_NAME=%s\n", clusterName) //nolint:errcheck
	fmt.Fprintf(streams.Out, "KUBECONFIG=%s\n", kubeconfigPath)     //nolint:errcheck
	return nil
}

// activeProviderName resolves the active container runtime provider name
// without contacting the container runtime.
func activeProviderName(logger log.Logger) string {
	p := os.Getenv("KIND_EXPERIMENTAL_PROVIDER")
	switch p {
	case "docker", "podman", "nerdctl", "finch", "nerdctl.lima":
		return p
	case "":
		// fall through to auto-detect
	default:
		logger.Warnf("unknown KIND_EXPERIMENTAL_PROVIDER value %q", p)
		// fall through to auto-detect
	}

	opt, err := cluster.DetectNodeProvider()
	if err != nil {
		logger.Warn("could not detect node provider; defaulting to docker")
		return "docker"
	}
	provider := cluster.NewProvider(cluster.ProviderWithLogger(log.NoopLogger{}), opt)
	return provider.Name()
}

// activeKubeconfigPath resolves the effective KUBECONFIG path without
// contacting the container runtime or requiring a cluster to exist.
func activeKubeconfigPath() string {
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		// KUBECONFIG may be a colon-separated list; use the first entry
		parts := filepath.SplitList(kc)
		for _, part := range parts {
			if part != "" {
				return part
			}
		}
	}
	return filepath.Join(os.Getenv("HOME"), ".kube", "config")
}
