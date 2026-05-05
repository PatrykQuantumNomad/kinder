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

// Package pause implements the `kinder pause [cluster-name]` command, which
// stops every container in a kinder cluster in quorum-safe order so the host
// can reclaim CPU and RAM. Cluster state survives the pause and is restored
// by `kinder resume`.
package pause

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
)

// flagpole holds the parsed flag values for `kinder pause`.
type flagpole struct {
	// Timeout is the per-container graceful stop timeout before SIGKILL is sent.
	// Generous default leaves room for kubelet/etcd flush.
	Timeout time.Duration
	// JSON enables JSON output for scripted consumers.
	JSON bool
}

// pauseFn is the test-injection point for the orchestration call. Production
// code calls lifecycle.Pause; tests substitute a fake to avoid spinning a
// real cluster.
var pauseFn = lifecycle.Pause

// resolveClusterName is the test-injection point for cluster name resolution.
// Production code wires it to lifecycle.ResolveClusterName backed by a real
// *cluster.Provider; tests substitute a closure that returns a fixed name.
var resolveClusterName = func(args []string) (string, error) {
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(log.NoopLogger{}),
		runtime.GetDefault(log.NoopLogger{}),
	)
	return lifecycle.ResolveClusterName(args, provider)
}

// NewCommand returns a new cobra.Command for `kinder pause [cluster-name]`.
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	c := &cobra.Command{
		Args:  cobra.MaximumNArgs(1),
		Use:   "pause [cluster-name]",
		Short: "Pauses a running cluster, stopping all containers to reclaim CPU/RAM",
		Long: "Pauses a running kinder cluster by gracefully stopping every node container.\n" +
			"Workers stop before control-plane nodes to keep etcd quorum safe.\n" +
			"If no cluster name is given and exactly one cluster exists, it is auto-selected.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runE(logger, streams, flags, args)
		},
	}
	c.Flags().DurationVar(&flags.Timeout, "timeout", 30*time.Second, "graceful stop timeout before SIGKILL (e.g. 30s, 2m)")
	c.Flags().BoolVar(&flags.JSON, "json", false, "output JSON")
	return c
}

func runE(logger log.Logger, streams cmd.IOStreams, flags *flagpole, args []string) error {
	if flags.Timeout < 0 {
		return fmt.Errorf("invalid --timeout %v: must be >= 0", flags.Timeout)
	}

	name, err := resolveClusterName(args)
	if err != nil {
		return err
	}

	// Resolve a provider for the orchestration call. Production lifecycle.Pause
	// uses this to enumerate nodes; tests stub pauseFn entirely so the provider
	// it receives is never dereferenced.
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)

	result, pauseErr := pauseFn(lifecycle.PauseOptions{
		ClusterName: name,
		Timeout:     flags.Timeout,
		Logger:      logger,
		Provider:    provider,
	})

	// Render output regardless of pauseErr so partial-failure data is visible.
	if result != nil {
		if flags.JSON {
			if encErr := json.NewEncoder(streams.Out).Encode(result); encErr != nil {
				return encErr
			}
		} else if result.AlreadyPaused {
			logger.Warn(fmt.Sprintf("cluster %q is already paused; no action taken", result.Cluster))
		} else {
			fmt.Fprintf(streams.Out, "Cluster paused. Total time: %.1fs\n", result.Duration)
		}
	}
	return pauseErr
}
