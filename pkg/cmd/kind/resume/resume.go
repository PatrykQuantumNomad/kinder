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

// Package resume implements the `kinder resume [cluster-name]` command, which
// starts every container in a paused kinder cluster in quorum-safe order and
// waits for all nodes to report Ready before exiting.
package resume

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

// flagpole holds the parsed flag values for `kinder resume`.
type flagpole struct {
	// Timeout is the per-container graceful start timeout, in seconds. Rarely
	// matters for `docker start` but threaded through for parity with `pause`.
	Timeout int
	// WaitSecs is the maximum number of seconds to wait for every node to
	// report Ready via the Kubernetes API after containers are running.
	WaitSecs int
	// JSON enables JSON output for scripted consumers.
	JSON bool
}

// resumeFn is the test-injection point for the orchestration call. Production
// code calls lifecycle.Resume; tests substitute a fake to avoid spinning a
// real cluster.
var resumeFn = lifecycle.Resume

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

// NewCommand returns a new cobra.Command for `kinder resume [cluster-name]`.
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &flagpole{}
	c := &cobra.Command{
		Args:  cobra.MaximumNArgs(1),
		Use:   "resume [cluster-name]",
		Short: "Resumes a paused cluster, starting all containers and waiting for nodes Ready",
		Long: "Resumes a paused kinder cluster by starting every node container in quorum-safe order\n" +
			"(external load balancer, then control-plane, then workers) and waiting for the\n" +
			"Kubernetes API to report all nodes Ready.\n" +
			"If no cluster name is given and exactly one cluster exists, it is auto-selected.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runE(logger, streams, flags, args)
		},
	}
	c.Flags().IntVar(&flags.Timeout, "timeout", 30, "graceful start timeout in seconds")
	c.Flags().IntVar(&flags.WaitSecs, "wait", 300, "max seconds to wait for all nodes Ready")
	c.Flags().BoolVar(&flags.JSON, "json", false, "output JSON")
	return c
}

func runE(logger log.Logger, streams cmd.IOStreams, flags *flagpole, args []string) error {
	if flags.WaitSecs < 0 {
		return fmt.Errorf("invalid --wait %d: must be >= 0", flags.WaitSecs)
	}
	if flags.Timeout < 0 {
		return fmt.Errorf("invalid --timeout %d: must be >= 0", flags.Timeout)
	}

	name, err := resolveClusterName(args)
	if err != nil {
		return err
	}

	// Resolve a provider for the orchestration call. Production lifecycle.Resume
	// uses this to enumerate nodes; tests stub resumeFn entirely so the provider
	// it receives is never dereferenced.
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)

	result, resumeErr := resumeFn(lifecycle.ResumeOptions{
		ClusterName:  name,
		StartTimeout: time.Duration(flags.Timeout) * time.Second,
		WaitTimeout:  time.Duration(flags.WaitSecs) * time.Second,
		Logger:       logger,
		Provider:     provider,
	})

	// Render output regardless of resumeErr so partial-failure data and
	// readiness-timeout context are visible.
	if result != nil {
		if flags.JSON {
			if encErr := json.NewEncoder(streams.Out).Encode(result); encErr != nil {
				return encErr
			}
		} else if result.AlreadyRunning {
			logger.Warn(fmt.Sprintf("cluster %q is already running; no action taken", result.Cluster))
		} else {
			fmt.Fprintf(streams.Out, "Cluster resumed. Total time: %.1fs\n", result.Duration)
		}
	}
	return resumeErr
}
