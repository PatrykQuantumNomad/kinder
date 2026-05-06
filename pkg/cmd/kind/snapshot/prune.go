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
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
	kinderrors "sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/kind/pkg/internal/lifecycle"
	snapshot "sigs.k8s.io/kind/pkg/internal/snapshot"
	"sigs.k8s.io/kind/pkg/log"

	"sigs.k8s.io/kind/pkg/internal/cli"
	"sigs.k8s.io/kind/pkg/internal/runtime"
)

// pruneFlagpole holds parsed flag values for `kinder snapshot prune`.
type pruneFlagpole struct {
	// KeepLast keeps the N most-recent snapshots; delete the rest.
	KeepLast int
	// OlderThan deletes snapshots older than this duration (e.g. 168h for 7 days).
	OlderThan time.Duration
	// MaxSize deletes oldest snapshots until total size <= SIZE (e.g. "10G", "500M").
	MaxSize string
	// Yes skips the y/N confirmation prompt.
	Yes bool
}

// prunestoreFn is the test-injection point for store operations.
// Returns (list function, delete function). Tests substitute fakes.
type pruneStoreFns struct {
	list   func(ctx context.Context) ([]snapshot.Info, error)
	delete func(ctx context.Context, name string) error
}

var pruneStoreFn = func(root, clusterName string) (*pruneStoreFns, error) {
	store, err := snapshot.NewStore(root, clusterName)
	if err != nil {
		return nil, err
	}
	return &pruneStoreFns{
		list:   store.List,
		delete: store.Delete,
	}, nil
}

// newPruneCommand returns the `kinder snapshot prune [cluster-name]` command.
func newPruneCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	flags := &pruneFlagpole{}
	c := &cobra.Command{
		Use:   "prune [cluster-name]",
		Short: "Delete snapshots according to a retention policy",
		Long: "Delete snapshots that match the specified retention policy.\n\n" +
			"At least one policy flag is REQUIRED — running prune with no flags is rejected\n" +
			"to prevent accidental data loss (CONTEXT.md locked decision).\n\n" +
			"  --keep-last N     keep N most-recent snapshots, delete older ones\n" +
			"  --older-than D    delete snapshots older than duration D (e.g. 168h for 7 days)\n" +
			"  --max-size SIZE   delete oldest snapshots until total size <= SIZE\n" +
			"                    SIZE supports K/KiB, M/MiB, G/GiB, T/TiB suffixes (base-2)\n\n" +
			"Multiple flags compose as a UNION (OR): a snapshot is deleted if any policy matches.\n\n" +
			"Unless --yes/-y is set, the deletion plan is printed and y/N confirmation is prompted.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.OverrideDefaultName(cmd.Flags())
			return runPrune(logger, streams, flags, args, cmd.Context())
		},
	}
	c.Flags().IntVar(&flags.KeepLast, "keep-last", 0, "keep N most-recent snapshots, delete older")
	c.Flags().DurationVar(&flags.OlderThan, "older-than", 0, "delete snapshots older than duration (e.g. 168h for 7d)")
	c.Flags().StringVar(&flags.MaxSize, "max-size", "", "delete oldest until total <= SIZE (e.g. 10G, 500M; base-2: 1K=1024)")
	c.Flags().BoolVarP(&flags.Yes, "yes", "y", false, "skip confirmation prompt")
	return c
}

func runPrune(logger log.Logger, streams cmd.IOStreams, flags *pruneFlagpole, args []string, ctx context.Context) error {
	// CONTEXT.md locked: NEVER delete on no-flag invocation.
	if flags.KeepLast == 0 && flags.OlderThan == 0 && flags.MaxSize == "" {
		return fmt.Errorf(
			"snapshot prune requires at least one policy flag:\n" +
				"  --keep-last N        keep N most-recent snapshots\n" +
				"  --older-than D       delete snapshots older than D (e.g. 168h for 7 days)\n" +
				"  --max-size SIZE      delete oldest until total <= SIZE (e.g. 10G, 500M)",
		)
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

	fns, err := pruneStoreFn("", name)
	if err != nil {
		return err
	}

	infos, err := fns.list(ctx)
	if err != nil {
		return err
	}

	maxBytes, err := parseSize(flags.MaxSize)
	if err != nil {
		return err
	}

	policy := snapshot.Policy{
		KeepLastN: flags.KeepLast,
		OlderThan: flags.OlderThan,
		MaxSize:   maxBytes,
	}

	victims := snapshot.PrunePlan(infos, policy, time.Now())
	if len(victims) == 0 {
		fmt.Fprintln(streams.Out, "No snapshots match the policy. Nothing to delete.")
		return nil
	}

	fmt.Fprintf(streams.Out, "The following %d snapshot(s) will be deleted:\n", len(victims))
	for _, v := range victims {
		fmt.Fprintf(streams.Out, "  - %s (%s, %s)\n", v.Name, humanizeBytes(v.Size), humanizeAge(time.Since(v.CreatedAt)))
	}

	if !flags.Yes {
		fmt.Fprint(streams.Out, "\nProceed? [y/N] ")
		line, _ := bufio.NewReader(streams.In).ReadString('\n')
		if strings.TrimSpace(strings.ToLower(line)) != "y" {
			return fmt.Errorf("aborted")
		}
	}

	var errs []error
	for _, v := range victims {
		if err := fns.delete(ctx, v.Name); err != nil {
			errs = append(errs, fmt.Errorf("delete %s: %w", v.Name, err))
		}
	}
	return kinderrors.NewAggregate(errs)
}
