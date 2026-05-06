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

// Package snapshot implements the `kinder snapshot` command group, which
// provides create, restore, list, show, and prune subcommands for managing
// cluster-state archives stored under ~/.kinder/snapshots/<cluster>/.
package snapshot

import (
	"github.com/spf13/cobra"

	"sigs.k8s.io/kind/pkg/cmd"
	"sigs.k8s.io/kind/pkg/log"
)

// NewCommand returns the `kinder snapshot` cobra group command with all five
// subcommands registered.
func NewCommand(logger log.Logger, streams cmd.IOStreams) *cobra.Command {
	c := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture and restore complete cluster snapshots",
		Long: "Snapshot manages cluster-state archives stored under ~/.kinder/snapshots/<cluster>/.\n" +
			"Each snapshot bundles etcd state, container images, local-path PV contents, and a\n" +
			"metadata manifest into a single .tar.gz file.\n\n" +
			"See `kinder snapshot create --help` to begin.",
	}
	c.AddCommand(newCreateCommand(logger, streams))
	c.AddCommand(newRestoreCommand(logger, streams))
	c.AddCommand(newListCommand(logger, streams))
	c.AddCommand(newShowCommand(logger, streams))
	c.AddCommand(newPruneCommand(logger, streams))
	return c
}
