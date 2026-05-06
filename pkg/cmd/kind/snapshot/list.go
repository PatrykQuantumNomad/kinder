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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	sigsyaml "sigs.k8s.io/yaml"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	snapshot "sigs.k8s.io/kind/pkg/internal/snapshot"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
)

// listFlagpole holds parsed flag values for `kinder snapshot list`.
type listFlagpole struct {
	// Output is the output format: "" (table), "json", or "yaml".
	Output string
	// NoTrunc disables ADDONS column truncation.
	NoTrunc bool
}

// addonsTruncateThreshold is the max rune width of the ADDONS column before
// ellipsis-truncation kicks in (CONTEXT.md locked: wide-terminal first;
// --no-trunc bypasses). Tuned for a typical 120-col terminal where
// NAME+AGE+SIZE+K8S+STATUS reserve ~70 cols, leaving ~50 for ADDONS.
const addonsTruncateThreshold = 50

// listFn is the test-injection point for listing snapshots.
// Production code creates a SnapshotStore and calls List; tests substitute a fake.
var listFn = func(ctx context.Context, root, clusterName string) ([]snapshot.Info, error) {
	store, err := snapshot.NewStore(root, clusterName)
	if err != nil {
		return nil, err
	}
	return store.List(ctx)
}

// newListCommand returns the `kinder snapshot list [cluster-name]` command.
func newListCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &listFlagpole{}
	c := &cobra.Command{
		Use:   "list [cluster-name]",
		Short: "List snapshots for a cluster",
		Long: "List all snapshots for the named cluster (or auto-detected cluster).\n\n" +
			"Columns: NAME, AGE, SIZE, K8S, ADDONS, STATUS\n" +
			"  STATUS: ok | corrupt | unknown\n\n" +
			"The ADDONS column is truncated at " + fmt.Sprintf("%d", addonsTruncateThreshold) + " chars by default; use --no-trunc to disable.\n" +
			"Use --output=json or --output=yaml for machine-parseable output.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runList(logger, streams, flags, args, cmd.Context())
		},
	}
	c.Flags().StringVar(&flags.Output, "output", "", "output format: json | yaml (default: table)")
	c.Flags().BoolVar(&flags.NoTrunc, "no-trunc", false, "do not truncate any output column")
	return c
}

func runList(logger log.Logger, streams cmd.IOStreams, flags *listFlagpole, args []string, ctx context.Context) error {
	if flags.Output != "" && flags.Output != "json" && flags.Output != "yaml" {
		return fmt.Errorf("unsupported output format %q: supported values are \"\", \"json\", \"yaml\"", flags.Output)
	}

	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(logger),
		runtime.GetDefault(logger),
	)
	name, err := lifecycle.ResolveClusterName(args, provider)
	if err != nil {
		return err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	infos, err := listFn(ctx, "", name)
	if err != nil {
		return err
	}

	switch flags.Output {
	case "json":
		return json.NewEncoder(streams.Out).Encode(infos)
	case "yaml":
		data, err := sigsyaml.Marshal(infos)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		_, err = streams.Out.Write(data)
		return err
	default:
		return printTable(streams.Out, infos, flags.NoTrunc)
	}
}

// printTable writes a tabwriter-formatted table with columns
// NAME, AGE, SIZE, K8S, ADDONS, STATUS to w.
func printTable(w io.Writer, infos []snapshot.Info, noTrunc bool) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tAGE\tSIZE\tK8S\tADDONS\tSTATUS")
	for _, info := range infos {
		age := humanizeAge(time.Since(info.CreatedAt))
		size := humanizeBytes(info.Size)
		k8s := ""
		addons := ""
		if info.Metadata != nil {
			k8s = info.Metadata.K8sVersion
			addons = formatAddons(info.Metadata.AddonVersions)
		}
		// CONTEXT.md locked: truncate ADDONS column above threshold;
		// --no-trunc bypasses entirely.
		if !noTrunc && len([]rune(addons)) > addonsTruncateThreshold {
			addons = truncateString(addons, addonsTruncateThreshold)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			info.Name, age, size, k8s, addons, info.Status)
	}
	return tw.Flush()
}
