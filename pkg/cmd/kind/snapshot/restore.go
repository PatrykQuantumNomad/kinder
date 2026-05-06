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

package snapshot

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	snapshot "sigs.k8s.io/kind/pkg/internal/snapshot"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
)

// restoreFlagpole holds parsed flag values for `kinder snapshot restore`.
type restoreFlagpole struct {
	// JSON enables JSON output for scripted consumers.
	JSON bool
}

// restoreFn is the test-injection point for the Restore orchestration call.
// Production code calls snapshot.Restore; tests substitute a fake.
var restoreFn = snapshot.Restore

// newRestoreCommand returns the `kinder snapshot restore [cluster] <snap-name>` command.
//
// IMPORTANT: restore does NOT have a --yes flag. Per CONTEXT.md locked decision,
// restore is a hard overwrite by design; the absence of --yes is intentional and
// signals to the caller that this is a destructive, non-interactive operation.
func newRestoreCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &restoreFlagpole{}
	c := &cobra.Command{
		Use:   "restore [cluster-name] <snap-name>",
		Short: "Restore a cluster from a named snapshot (HARD OVERWRITE — no --yes flag by design)",
		Long: "Restore overwrites the running cluster state with the named snapshot. This is a\n" +
			"destructive operation with NO automatic rollback on failure.\n\n" +
			"Pre-flight checks (k8s version, topology, addon versions) must pass or the command\n" +
			"exits non-zero listing each violation before any mutation occurs.\n\n" +
			"On failure AFTER the cluster is paused, the cluster is left in a paused/inconsistent\n" +
			"state. The error message includes a recovery hint: run `kinder resume <cluster>`.\n\n" +
			"Arg disambiguation: with 2 args, args[0]=cluster, args[1]=snap-name.\n" +
			"With 1 arg, arg[0]=snap-name (cluster is auto-detected when exactly one exists).",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runRestore(logger, streams, flags, args)
		},
	}
	c.Flags().BoolVar(&flags.JSON, "json", false, "output JSON")
	return c
}

func runRestore(logger log.Logger, streams cmd.IOStreams, flags *restoreFlagpole, args []string) error {
	var clusterName, snapName string
	if len(args) == 1 {
		// Single arg = snap-name; auto-detect cluster.
		snapName = args[0]
	} else {
		// Two args: cluster then snap-name.
		clusterName = args[0]
		snapName = args[1]
	}

	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)

	res, err := restoreFn(snapshot.RestoreOptions{
		ClusterName: clusterName,
		SnapName:    snapName,
		Logger:      logger,
		Provider:    provider,
	})
	if err != nil {
		return err
	}

	if flags.JSON {
		return json.NewEncoder(streams.Out).Encode(res)
	}
	fmt.Fprintf(streams.Out, "Restored snapshot %s in %.1fs\n", res.SnapName, res.DurationS)
	return nil
}
