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
	"time"

	"github.com/spf13/cobra"
	sigsyaml "sigs.k8s.io/yaml"

	"sigs.k8s.io/kind/pkg/cmd"
	snapshot "sigs.k8s.io/kind/pkg/internal/snapshot"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
)

// showFlagpole holds parsed flag values for `kinder snapshot show`.
type showFlagpole struct {
	// Output is the output format: "" (vertical key/value), "json", or "yaml".
	Output string
}

// showFn is the test-injection point for opening a snapshot bundle.
// Returns (*Metadata, Info.Size, error). Tests substitute a fake.
type showResult struct {
	Meta *snapshot.Metadata
	Size int64
}

var showFn = func(ctx context.Context, root, clusterName, snapName string) (*showResult, error) {
	store, err := snapshot.NewStore(root, clusterName)
	if err != nil {
		return nil, err
	}
	br, info, err := store.Open(ctx, snapName)
	if err != nil {
		return nil, err
	}
	defer br.Close()
	m := br.Metadata()
	if m == nil {
		return nil, fmt.Errorf("snapshot %q has no readable metadata", snapName)
	}
	return &showResult{Meta: m, Size: info.Size}, nil
}

// newShowCommand returns the `kinder snapshot show [cluster-name] <snap-name>` command.
func newShowCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &showFlagpole{}
	c := &cobra.Command{
		Use:   "show [cluster-name] <snap-name>",
		Short: "Show full metadata for a snapshot in vertical key/value layout",
		Long: "Show detailed metadata for the named snapshot.\n\n" +
			"Arg disambiguation: with 2 args, args[0]=cluster, args[1]=snap-name.\n" +
			"With 1 arg, arg[0]=snap-name (cluster is auto-detected when exactly one exists).\n\n" +
			"Default output is a vertical key/value layout (planner discretion: chosen for\n" +
			"readability of the addon map). Use --output=json or --output=yaml for scripted use.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runShow(streams, flags, args, cmd.Context())
		},
	}
	c.Flags().StringVar(&flags.Output, "output", "", "output format: json | yaml (default: vertical key/value)")
	return c
}

func runShow(streams cmd.IOStreams, flags *showFlagpole, args []string, ctx context.Context) error {
	if flags.Output != "" && flags.Output != "json" && flags.Output != "yaml" {
		return fmt.Errorf("unsupported output format %q: supported values are \"\", \"json\", \"yaml\"", flags.Output)
	}

	var clusterName, snapName string
	if len(args) == 1 {
		snapName = args[0]
	} else {
		clusterName = args[0]
		snapName = args[1]
	}

	if ctx == nil {
		ctx = context.Background()
	}
	res, err := showFn(ctx, "", clusterName, snapName)
	if err != nil {
		return err
	}
	m := res.Meta

	switch flags.Output {
	case "json":
		return json.NewEncoder(streams.Out).Encode(m)
	case "yaml":
		data, err := sigsyaml.Marshal(m)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		_, err = streams.Out.Write(data)
		return err
	default:
		return printVertical(streams, m, res.Size)
	}
}

// printVertical writes the vertical key/value layout for a snapshot's metadata.
// Planner discretion: vertical layout chosen for readability of the addon map.
func printVertical(streams cmd.IOStreams, m *snapshot.Metadata, size int64) error {
	fmt.Fprintf(streams.Out, "Name:           %s\n", m.Name)
	fmt.Fprintf(streams.Out, "Cluster:        %s\n", m.ClusterName)
	fmt.Fprintf(streams.Out, "Created:        %s\n", m.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(streams.Out, "Size:           %s\n", humanizeBytes(size))
	fmt.Fprintf(streams.Out, "K8s Version:    %s\n", m.K8sVersion)
	fmt.Fprintf(streams.Out, "Node Image:     %s\n", m.NodeImage)
	fmt.Fprintf(streams.Out, "Topology:       %d CP / %d worker / lb=%v\n",
		m.Topology.ControlPlaneCount, m.Topology.WorkerCount, m.Topology.HasLoadBalancer)
	fmt.Fprintf(streams.Out, "Addons:\n")
	for _, name := range sortedKeys(m.AddonVersions) {
		fmt.Fprintf(streams.Out, "  - %s: %s\n", name, m.AddonVersions[name])
	}
	fmt.Fprintf(streams.Out, "Digests:\n")
	fmt.Fprintf(streams.Out, "  archive: %s\n", m.ArchiveDigest)
	fmt.Fprintf(streams.Out, "  etcd:    %s\n", m.EtcdDigest)
	fmt.Fprintf(streams.Out, "  images:  %s\n", m.ImagesDigest)
	fmt.Fprintf(streams.Out, "  pvs:     %s\n", m.PVsDigest)
	return nil
}
